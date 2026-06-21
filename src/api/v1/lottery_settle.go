package v1

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/prediction"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// SettleLottery 触发赛事实战一键财务复盘结算
func (ctrl *APIController) SettleLottery(c *gin.Context) {
	plans, err := db.GetUnsettledLotteryPlans()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	settledCount := 0
	for _, p := range plans {
		mIDs := strings.Split(p.MatchIDs, ",")
		allFinished := true
		var finishedMatches []models.Match
		for _, mid := range mIDs {
			m, err := db.GetMatch(mid)
			if err != nil || m.Status != "FT" {
				allFinished = false
				break
			}
			finishedMatches = append(finishedMatches, m)
		}

		if !allFinished || len(finishedMatches) == 0 {
			continue
		}

		if p.PlanType == "single" {
			m := finishedMatches[0]
			primaryHit := checkLegHit(m.ID, p.PrimaryBet)

			hedgeHit := false
			if p.HedgeBet != "" {
				hedgeHit = checkLegHit(m.ID, p.HedgeBet)
			}

			safeReturn := 0.0
			if primaryHit {
				safeReturn += p.PrimaryAmt * p.PrimaryOdds
			}
			if hedgeHit {
				safeReturn += p.HedgeAmt * p.HedgeOdds
			}
			safeProfit := safeReturn - 100.0

			aggReturn := 0.0
			if primaryHit {
				aggReturn += 100.0 * p.PrimaryOdds
			}
			aggProfit := aggReturn - 100.0

			_ = db.UpdateLotteryPlanSettlement(p.ID, safeReturn, safeProfit, aggReturn, aggProfit)
			settledCount++
		} else {
			var savedTickets []prediction.SavedTicket
			if err := json.Unmarshal([]byte(p.TicketsJSON), &savedTickets); err != nil {
				continue
			}

			totalReturn := 0.0
			for _, t := range savedTickets {
				ticketWon := true
				for _, leg := range t.Legs {
					if !checkLegHit(leg.MatchID, leg.Option) {
						ticketWon = false
						break
					}
				}
				if ticketWon {
					totalReturn += t.Payout
				}
			}
			totalProfit := totalReturn - p.Cost
			_ = db.UpdateLotteryPlanSettlement(p.ID, totalReturn, totalProfit, totalReturn, totalProfit)
			settledCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"settled": settledCount,
	})
}
