package v1

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/prediction"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SaveSingleLottery 手动保存单场投注建议方案
func (ctrl *APIController) SaveSingleLottery(c *gin.Context) {
	var req struct {
		MatchID     string  `json:"matchId"`
		OddsH       float64 `json:"oddsH"`
		OddsD       float64 `json:"oddsD"`
		OddsA       float64 `json:"oddsA"`
		PrimaryBet  string  `json:"primaryBet"`
		PrimaryOdds float64 `json:"primaryOdds"`
		HedgeBet    string  `json:"hedgeBet"`
		HedgeOdds   float64 `json:"hedgeOdds"`
		HedgeAmt    float64 `json:"hedgeAmt"`
		Reason      string  `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	plan := models.LotteryPlan{
		PlanType:    "single",
		MatchIDs:    req.MatchID,
		OddsH:       req.OddsH,
		OddsD:       req.OddsD,
		OddsA:       req.OddsA,
		PrimaryBet:  req.PrimaryBet,
		PrimaryOdds: req.PrimaryOdds,
		PrimaryAmt:  80.0,
		HedgeBet:    req.HedgeBet,
		HedgeOdds:   req.HedgeOdds,
		HedgeAmt:    req.HedgeAmt,
		DescStr:     req.Reason,
		IsSettled:   0,
	}
	err := db.SaveLotteryPlan(plan)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// SaveParlayLottery 手动保存多场串关方案
func (ctrl *APIController) SaveParlayLottery(c *gin.Context) {
	var req struct {
		MatchIDs      string                     `json:"matchIds"`
		ParlayMode    string                     `json:"parlayMode"`
		ParlayOptions string                     `json:"parlayOptions"`
		Parlays       []prediction.ParlayAdvice  `json:"parlays"`
		Excluded      []prediction.ExcludedMatch `json:"excluded"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	savedCount := 0
	for _, p := range req.Parlays {
		var descStr string
		for _, ex := range req.Excluded {
			if ex.HomeTeam == p.ParlayType {
				descStr = ex.AwayTeam
				break
			}
		}
		plan := models.LotteryPlan{
			PlanType:           "parlay",
			MatchIDs:           req.MatchIDs,
			ParlayType:         p.ParlayType,
			ParlayMode:         req.ParlayMode,
			ParlayOptions:      req.ParlayOptions,
			DescStr:            descStr,
			WinsCount:          p.WinsCount,
			Cost:               p.Cost,
			SingleTicketPayout: p.SingleTicketPayout,
			ComboOdds:          p.ComboOdds,
			ComboProb:          p.ComboProb,
			TotalEV:            p.TotalEV,
			KellyStake:         p.KellyStake,
			TicketsJSON:        p.TicketsJSON,
			IsSettled:          0,
		}
		if err := db.SaveLotteryPlan(plan); err == nil {
			savedCount++
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "saved": savedCount})
}

// GetOfficialLottery 获取指定赛事的官方体彩赔率数据 (如果未开盘则利用 Elo 算法仿真)
func (ctrl *APIController) GetOfficialLottery(c *gin.Context) {
	matchID := c.Query("matchId")
	if matchID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 matchId"})
		return
	}
	match, err := db.GetMatch(matchID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "比赛未找到"})
		return
	}
	odds := ctrl.SportteryService.GetMatchOdds(match.HomeTeam, match.AwayTeam, match.ScheduledAt)
	c.JSON(http.StatusOK, odds)
}

// DeleteLotteryPlans 物理删除保存方案
func (ctrl *APIController) DeleteLotteryPlans(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数解析失败: " + err.Error()})
		return
	}
	if err := db.DeleteLotteryPlans(req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "物理删除方案失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetSavedLotteryPlans 获取所有已保存的方案（单场和过关，包括未结算）
func (ctrl *APIController) GetSavedLotteryPlans(c *gin.Context) {
	plans, err := db.GetSavedLotteryPlans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var savedList []gin.H
	for _, p := range plans {
		if p.PlanType == "single" {
			savedList = append(savedList, buildSingleSavedItem(p))
		} else {
			savedList = append(savedList, buildParlaySavedItem(p))
		}
	}
	c.JSON(http.StatusOK, savedList)
}

func buildSingleSavedItem(p models.LotteryPlan) gin.H {
	m, err := db.GetMatch(p.MatchIDs)
	homeTeam, awayTeam := "未知主队", "未知客队"
	homeScore, awayScore := 0, 0
	status := "NS"
	if err == nil {
		homeTeam, awayTeam = m.HomeTeam, m.AwayTeam
		homeScore, awayScore = m.HomeScore, m.AwayScore
		status = m.Status
	}

	primaryHit := checkLegHit(p.MatchIDs, p.PrimaryBet)
	hedgeHit := false
	if p.HedgeBet != "" {
		hedgeHit = checkLegHit(p.MatchIDs, p.HedgeBet)
	}

	return gin.H{
		"id":          p.ID,
		"planType":    p.PlanType,
		"matchId":     p.MatchIDs,
		"homeTeam":    homeTeam,
		"awayTeam":    awayTeam,
		"homeScore":   homeScore,
		"awayScore":   awayScore,
		"status":      status,
		"isSettled":   p.IsSettled,
		"primaryBet":  p.PrimaryBet,
		"primaryOdds": p.PrimaryOdds,
		"primaryHit":  primaryHit,
		"hedgeBet":    p.HedgeBet,
		"hedgeOdds":   p.HedgeOdds,
		"hedgeHit":    hedgeHit,
		"createdAt":   p.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

func buildParlaySavedItem(p models.LotteryPlan) gin.H {
	mNames := "多场混合过关精算"
	mIDs := strings.Split(p.MatchIDs, ",")
	if len(mIDs) > 0 {
		if m, err := db.GetMatch(mIDs[0]); err == nil {
			mNames = fmt.Sprintf("%s等%d场串关", m.HomeTeam, len(mIDs))
		}
	}

	var tickets []prediction.SavedTicket
	if p.TicketsJSON != "" {
		_ = json.Unmarshal([]byte(p.TicketsJSON), &tickets)
	}

	var ticketsWithResult []TicketWithResult
	for _, tk := range tickets {
		var legsWithRes []LegWithResult
		for _, leg := range tk.Legs {
			m, err := db.GetMatch(leg.MatchID)
			hScore, aScore := 0, 0
			hTeam, aTeam := "", ""
			mStatus := "NS"
			if err == nil {
				hScore, aScore = m.HomeScore, m.AwayScore
				hTeam, aTeam = m.HomeTeam, m.AwayTeam
				mStatus = m.Status
			}
			hit := checkLegHit(leg.MatchID, leg.Option)
			legsWithRes = append(legsWithRes, LegWithResult{
				MatchID:   leg.MatchID,
				Option:    leg.Option,
				Odds:      leg.Odds,
				HomeTeam:  hTeam,
				AwayTeam:  aTeam,
				HomeScore: hScore,
				AwayScore: aScore,
				Status:    mStatus,
				Hit:       hit,
			})
		}
		ticketsWithResult = append(ticketsWithResult, TicketWithResult{
			Odds:   tk.Odds,
			Payout: tk.Payout,
			Legs:   legsWithRes,
		})
	}

	return gin.H{
		"id":          p.ID,
		"planType":    p.PlanType,
		"matchId":     p.MatchIDs,
		"homeTeam":    mNames,
		"awayTeam":    p.ParlayType,
		"homeScore":   0,
		"awayScore":   0,
		"isSettled":   p.IsSettled,
		"primaryHit":  p.SafeProfit > 0,
		"tickets":     ticketsWithResult,
		"createdAt":   p.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}
