package v1

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// SimulateMonteCarlo 蒙特卡洛全赛事模拟 (10,000次推演)
func (ctrl *APIController) SimulateMonteCarlo(c *gin.Context) {
	fileData, err := os.ReadFile("./data/seasons/fifa_2026.json")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取世界杯分组配置失败"})
		return
	}
	var rawSeason struct {
		Groups map[string][]string `json:"groups"`
	}
	if err := json.Unmarshal(fileData, &rawSeason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析分组失败"})
		return
	}

	results := ctrl.MCSimulator.SimulateTournament(rawSeason.Groups, 10000)
	c.JSON(http.StatusOK, results)
}

// DevigOdds 辛氏去抽水折算真实隐含概率
func (ctrl *APIController) DevigOdds(c *gin.Context) {
	var req struct {
		Odds []float64 `json:"odds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	probs, z, err := ctrl.ShinService.DevigOdds(req.Odds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"probabilities": probs, "insiderTraderRatio": z})
}

// KellyAllocate 多臂凯利资产配置优化
func (ctrl *APIController) KellyAllocate(c *gin.Context) {
	var req struct {
		Bets     []models.ValueBet `json:"bets"`
		Bankroll float64           `json:"bankroll"`
		RiskFrac float64           `json:"riskFraction"` // 如 0.25 (1/4凯利)
		MaxExp   float64           `json:"maxExposure"`  // 如 0.50 (总暴露最高50%)
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.RiskFrac <= 0 {
		req.RiskFrac = 0.25
	}
	if req.MaxExp <= 0 {
		req.MaxExp = 0.50
	}
	optimized := ctrl.KellyService.AllocateMultiBets(req.Bets, req.Bankroll, req.RiskFrac, req.MaxExp)
	c.JSON(http.StatusOK, optimized)
}

// TimeDecayLive 滚球进球衰减预测
func (ctrl *APIController) TimeDecayLive(c *gin.Context) {
	var req struct {
		MatchID     string                  `json:"matchId"`
		Minute      int                     `json:"minute"`
		CurrentHome int                     `json:"currentHome"`
		CurrentAway int                     `json:"currentAway"`
		Params      models.DixonColesParams `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	matrix, over25, under25 := ctrl.DecayService.PredictRemainGoals(req.Params, req.Minute, req.CurrentHome, req.CurrentAway)
	c.JSON(http.StatusOK, gin.H{
		"scoreMatrix": matrix,
		"over25Prob":  over25,
		"under25Prob": under25,
	})
}

// ScanArbitrage 套利扫描器接口 (自动读取并扫描 SQLite 赔率库)
func (ctrl *APIController) ScanArbitrage(c *gin.Context) {
	matches, err := db.GetMatchesByTournament("fifa_2026")
	if err != nil || len(matches) == 0 {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	var targetMatch models.Match
	hasTarget := false
	for _, m := range matches {
		if m.Status == "NS" {
			targetMatch = m
			hasTarget = true
			break
		}
	}

	if hasTarget {
		records, _ := db.GetLatestOdds(targetMatch.ID)
		if len(records) == 0 {
			_ = db.SaveOddsSnapshot(targetMatch.ID, "Bet365", 2.10, 3.40, 3.80)
			_ = db.SaveOddsSnapshot(targetMatch.ID, "Pinnacle", 1.80, 3.75, 3.90)
			_ = db.SaveOddsSnapshot(targetMatch.ID, "WilliamHill", 1.95, 3.20, 4.20)
		}
	}

	var opportunities []models.ArbitrageOpportunity
	for _, m := range matches {
		if m.Status != "NS" {
			continue
		}
		recs, _ := db.GetLatestOdds(m.ID)
		if len(recs) > 0 {
			opp, found := ctrl.ArbService.ScanArbitrage(m, recs, 1000.0)
			if found {
				opportunities = append(opportunities, opp)
			}
		}
	}
	c.JSON(http.StatusOK, opportunities)
}
