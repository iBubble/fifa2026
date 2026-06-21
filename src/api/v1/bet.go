package v1

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GetBets 获取当前的投注历史流水列表
func (ctrl *APIController) GetBets(c *gin.Context) {
	bets, err := db.GetBets("fifa_2026")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bets)
}

// GetBetSummary 获取账本的综合 ROI 等统计汇总
func (ctrl *APIController) GetBetSummary(c *gin.Context) {
	summary, err := db.GetBetSummary("fifa_2026")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// CreateBet 新增一条投注流水记录
func (ctrl *APIController) CreateBet(c *gin.Context) {
	var bet models.Bet
	if err := c.ShouldBindJSON(&bet); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bet.TournamentID = "fifa_2026"
	bet.PlacedAt = time.Now()
	bet.Result = "PENDING"
	id, err := db.AddBet(bet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// SettleBet 结算单条投注流水
func (ctrl *APIController) SettleBet(c *gin.Context) {
	var req struct {
		ID     int64   `json:"id"`
		Result string  `json:"result"` // "WIN", "LOSS", "VOID"
		PnL    float64 `json:"pnl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := db.UpdateBetResult(req.ID, req.Result, req.PnL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
