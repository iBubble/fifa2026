package v1

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/service/ai"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// OptimizeBacktest 手动触发已完赛复盘参数网格搜索与热更新优化接口
func (ctrl *APIController) OptimizeBacktest(c *gin.Context) {
	rebuild := c.Query("rebuild")
	var oldBs, newBs float64
	var rebuilt bool

	if rebuild == "true" {
		var errRb error
		oldBs, newBs, errRb = ctrl.BacktestService.RebuildAllFinishedMatchesBacktest()
		if errRb != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("时序重演错误: %v", errRb)})
			return
		}
		rebuilt = true
	}

	nd, dm, hm, r, bs, err := ctrl.DCService.OptimizeParameters()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"status":         "success",
		"NormDivulator":  nd,
		"DiffMultiplier": dm,
		"H2hMultiplier":  hm,
		"InitialRho":     r,
		"BrierScore":     bs,
		"rebuilt":        rebuilt,
	}
	if rebuilt {
		response["oldBrierScore"] = oldBs
		response["newBrierScore"] = newBs
	}

	c.JSON(http.StatusOK, response)
}

// GetBacktestHistory 拉取所有已完赛复盘报告历史数据，供大屏绘制 Brier Score 曲线
func (ctrl *APIController) GetBacktestHistory(c *gin.Context) {
	reports, err := db.GetBacktestReports()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var responseList []gin.H
	for _, r := range reports {
		m, err := db.GetMatch(r.MatchID)
		homeTeam := "未知主队"
		awayTeam := "未知客队"
		homeScore := 0
		awayScore := 0
		if err == nil {
			homeTeam = m.HomeTeam
			awayTeam = m.AwayTeam
			homeScore = m.HomeScore
			awayScore = m.AwayScore
		}

		reviewText := r.TacticsReview
		if strings.Contains(reviewText, "无法获取赛后反思文本") || strings.Contains(reviewText, "超时降级") || reviewText == "" {
			reviewText = ai.GenerateFallbackReview(homeTeam, awayTeam, homeScore, awayScore, r.BrierScore)
		}

		responseList = append(responseList, gin.H{
			"matchId":       r.MatchID,
			"brierScore":    r.BrierScore,
			"homeEloDiff":   r.HomeEloDiff,
			"awayEloDiff":   r.AwayEloDiff,
			"tacticsReview": reviewText,
			"reviewedAt":    r.ReviewedAt,
			"homeTeam":      homeTeam,
			"awayTeam":      awayTeam,
			"homeScore":     homeScore,
			"awayScore":     awayScore,
		})
	}
	c.JSON(http.StatusOK, responseList)
}

// GetNews 真实外围情报获取接口
func (ctrl *APIController) GetNews(c *gin.Context) {
	matchID := c.Query("matchId")
	var homeTeam, awayTeam string
	if matchID != "" {
		if match, err := db.GetMatch(matchID); err == nil {
			homeTeam = match.HomeTeam
			awayTeam = match.AwayTeam
		}
	}
	articles, err := ctrl.NewsService.FetchRealNewsForMatch(homeTeam, awayTeam)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, articles)
}

// GetOddsShifts 全球博彩公司赔率与偏移监测接口
func (ctrl *APIController) GetOddsShifts(c *gin.Context) {
	matchID := c.Query("matchId")
	var homeTeam, awayTeam string
	if matchID != "" {
		if match, err := db.GetMatch(matchID); err == nil {
			homeTeam = match.HomeTeam
			awayTeam = match.AwayTeam
		}
	}
	shifts := ctrl.OddsTrackerService.GetOddsShifts(homeTeam, awayTeam)
	c.JSON(http.StatusOK, shifts)
}
