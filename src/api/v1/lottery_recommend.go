package v1

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/prediction"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RecommendLottery 接口，中国体彩竞猜量化套利建议
func (ctrl *APIController) RecommendLottery(c *gin.Context) {
	var req struct {
		MatchIDs      []string                 `json:"matchIds"`
		Odds          []float64                `json:"odds"`
		PredictReport *models.PredictionReport `json:"predictReport"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.MatchIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择至少一场比赛进行分析"})
		return
	}

	m1, err := db.GetMatch(req.MatchIDs[0])
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到首场比赛"})
		return
	}

	oddsH, oddsD, oddsA := 1.95, 3.20, 3.80
	if len(req.Odds) >= 3 {
		oddsH, oddsD, oddsA = req.Odds[0], req.Odds[1], req.Odds[2]
	}

	odds := ctrl.SportteryService.GetMatchOdds(m1.HomeTeam, m1.AwayTeam, m1.ScheduledAt)
	isSingleHad := odds.HadSingle

	singleAdvice := ctrl.LotteryService.GenerateSingleAdvice(m1, oddsH, oddsD, oddsA, req.PredictReport, isSingleHad)

	var parlayAdvice *prediction.LotteryAdvice
	if len(req.MatchIDs) >= 2 {
		m2, err := db.GetMatch(req.MatchIDs[1])
		if err == nil {
			pAdv := ctrl.LotteryService.GenerateParlayAdvice(m1, m2, oddsH, oddsA, req.PredictReport)
			parlayAdvice = &pAdv
		}
	}

	fivePlays := ctrl.LotteryService.GenerateFivePlaysAdvice(m1, req.PredictReport, isSingleHad)

	c.JSON(http.StatusOK, gin.H{
		"single":    singleAdvice,
		"parlay":    parlayAdvice,
		"fivePlays": fivePlays,
	})
}
