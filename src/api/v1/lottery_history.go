package v1

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/service/prediction"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetLotteryHistory 获取历史体彩建议收益结算列表
func (ctrl *APIController) GetLotteryHistory(c *gin.Context) {
	plans, err := db.GetSettledLotteryPlans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var historyList []gin.H
	var totalSafeCost, totalSafeReturn float64
	var totalAggCost, totalAggReturn float64

	for _, p := range plans {
		if p.PlanType == "single" {
			totalSafeCost += 100.0
			totalSafeReturn += p.SafeReturn
			totalAggCost += 100.0
			totalAggReturn += p.AggReturn

			m, _ := db.GetMatch(p.MatchIDs)
			primaryHit := checkLegHit(p.MatchIDs, p.PrimaryBet)
			hedgeHit := false
			if p.HedgeBet != "" {
				hedgeHit = checkLegHit(p.MatchIDs, p.HedgeBet)
			}

			historyList = append(historyList, gin.H{
				"id":          p.ID,
				"planType":    p.PlanType,
				"matchId":     p.MatchIDs,
				"homeTeam":    m.HomeTeam,
				"awayTeam":    m.AwayTeam,
				"homeScore":   m.HomeScore,
				"awayScore":   m.AwayScore,
				"primaryBet":  p.PrimaryBet,
				"primaryOdds": p.PrimaryOdds,
				"primaryHit":  primaryHit,
				"hedgeBet":    p.HedgeBet,
				"hedgeOdds":   p.HedgeOdds,
				"hedgeHit":    hedgeHit,
				"safeReturn":  math.Round(p.SafeReturn*100) / 100,
				"safeProfit":  math.Round(p.SafeProfit*100) / 100,
				"aggReturn":   math.Round(p.AggReturn*100) / 100,
				"aggProfit":   math.Round(p.AggProfit*100) / 100,
			})
		} else {
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

			type LegWithResult struct {
				MatchID   string  `json:"matchId"`
				Option    string  `json:"option"`
				Odds      float64 `json:"odds"`
				HomeTeam  string  `json:"homeTeam"`
				AwayTeam  string  `json:"awayTeam"`
				HomeScore int     `json:"homeScore"`
				AwayScore int     `json:"awayScore"`
				Status    string  `json:"status"`
				Hit       bool    `json:"hit"`
			}
			type TicketWithResult struct {
				Odds   float64         `json:"odds"`
				Payout float64         `json:"payout"`
				Legs   []LegWithResult `json:"legs"`
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

			historyList = append(historyList, gin.H{
				"id":          p.ID,
				"planType":    p.PlanType,
				"matchId":     p.MatchIDs,
				"homeTeam":    mNames,
				"awayTeam":    p.ParlayType,
				"homeScore":   0,
				"awayScore":   0,
				"primaryBet":  fmt.Sprintf("过关:%s", p.ParlayOptions),
				"primaryOdds": p.ComboOdds,
				"primaryHit":  p.SafeProfit > 0,
				"hedgeBet":    "",
				"hedgeOdds":   0.0,
				"hedgeHit":    false,
				"safeReturn":  math.Round(p.SafeReturn*100) / 100,
				"safeProfit":  math.Round(p.SafeProfit*100) / 100,
				"aggReturn":   math.Round(p.SafeReturn*100) / 100,
				"aggProfit":   math.Round(p.SafeProfit*100) / 100,
				"tickets":     ticketsWithResult,
			})
		}
	}

	safeProfit := totalSafeReturn - totalSafeCost
	aggProfit := totalAggReturn - totalAggCost

	safeRoi, aggRoi := 0.0, 0.0
	if totalSafeCost > 0 {
		safeRoi = (safeProfit / totalSafeCost) * 100.0
	}
	if totalAggCost > 0 {
		aggRoi = (aggProfit / totalAggCost) * 100.0
	}

	c.JSON(http.StatusOK, gin.H{
		"history": historyList,
		"summary": gin.H{
			"totalSafeCost":   totalSafeCost,
			"totalSafeReturn": math.Round(totalSafeReturn*100) / 100,
			"totalSafeProfit": math.Round(safeProfit*100) / 100,
			"safeRoi":         math.Round(safeRoi*100) / 100,
			"totalAggCost":    totalAggCost,
			"totalAggReturn":  math.Round(totalAggReturn*100) / 100,
			"totalAggProfit":  math.Round(aggProfit*100) / 100,
			"aggRoi":          math.Round(aggRoi*100) / 100,
		},
	})
}
