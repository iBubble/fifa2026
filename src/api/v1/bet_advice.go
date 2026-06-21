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

// SaveBetAdvice 持久化保存大模型生成的智能投注方案
func (ctrl *APIController) SaveBetAdvice(c *gin.Context) {
	var req struct {
		PlanType    string `json:"planType"`
		TotalAmount float64 `json:"totalAmount"`
		Items       []struct {
			MatchName  string  `json:"matchName"`
			Market     string  `json:"market"`
			Selection  string  `json:"selection"`
			Odds       float64 `json:"odds"`
			Stake      float64 `json:"stake"`
			BetType    string  `json:"betType"`
		} `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有可保存的投注项"})
		return
	}

	allMatches, err := db.GetMatchesByTournament("fifa_2026")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	teamToMatchID := make(map[string]string)
	for _, m := range allMatches {
		teamToMatchID[strings.ToLower(strings.TrimSpace(m.HomeTeam))] = m.ID
		teamToMatchID[strings.ToLower(strings.TrimSpace(m.AwayTeam))] = m.ID
		tHome, _ := db.GetTeamTranslation(m.HomeTeam)
		if tHome.CnName != "" {
			teamToMatchID[strings.ToLower(strings.TrimSpace(tHome.CnName))] = m.ID
		}
		tAway, _ := db.GetTeamTranslation(m.AwayTeam)
		if tAway.CnName != "" {
			teamToMatchID[strings.ToLower(strings.TrimSpace(tAway.CnName))] = m.ID
		}
	}

	findMatchID := func(matchName string) string {
		matchName = strings.ReplaceAll(matchName, " ", "")
		subParts := strings.Split(matchName, "VS")
		if len(subParts) == 2 {
			h := strings.ToLower(strings.TrimSpace(subParts[0]))
			if id, ok := teamToMatchID[h]; ok {
				return id
			}
			a := strings.ToLower(strings.TrimSpace(subParts[1]))
			if id, ok := teamToMatchID[a]; ok {
				return id
			}
		}
		for name, id := range teamToMatchID {
			if strings.Contains(strings.ToLower(matchName), name) {
				return id
			}
		}
		return ""
	}

	var savedTickets []prediction.SavedTicket
	matchIDSet := make(map[string]bool)

	for _, item := range req.Items {
		var ticket prediction.SavedTicket
		ticket.Odds = item.Odds
		ticket.Payout = item.Stake * item.Odds

		if strings.Contains(item.MatchName, "&") && strings.Contains(item.Selection, "&") {
			mNames := strings.Split(item.MatchName, "&")
			sels := strings.Split(item.Selection, "&")
			for idx, mn := range mNames {
				mn = strings.TrimSpace(mn)
				sel := "主胜"
				if idx < len(sels) {
					sel = strings.TrimSpace(sels[idx])
				}
				mid := findMatchID(mn)
				if mid != "" {
					matchIDSet[mid] = true
					ticket.Legs = append(ticket.Legs, prediction.SavedLeg{
						MatchID: mid,
						Option:  sel,
						Odds:    1.0,
					})
				}
			}
		} else {
			mid := findMatchID(item.MatchName)
			if mid != "" {
				matchIDSet[mid] = true
				ticket.Legs = append(ticket.Legs, prediction.SavedLeg{
					MatchID: mid,
					Option:  strings.TrimSpace(item.Selection),
					Odds:    item.Odds,
				})
			}
		}
		if len(ticket.Legs) > 0 {
			savedTickets = append(savedTickets, ticket)
		}
	}

	if len(savedTickets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未能成功匹配到任何有效赛事的 ID"})
		return
	}

	var mIDList []string
	for mid := range matchIDSet {
		mIDList = append(mIDList, mid)
	}

	ticketsBytes, _ := json.Marshal(savedTickets)
	plan := models.LotteryPlan{
		PlanType:    "parlay",
		MatchIDs:    strings.Join(mIDList, ","),
		RiskLevel:   req.PlanType,
		Cost:        req.TotalAmount,
		TicketsJSON: string(ticketsBytes),
		DescStr:     "智能多Agent推荐方案(" + req.PlanType + ")",
		IsSettled:   0,
	}

	errSave := db.SaveLotteryPlan(plan)
	if errSave != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errSave.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
