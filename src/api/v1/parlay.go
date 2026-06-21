package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ParlayRecommend 提供混合过关智能体彩推荐
func (ctrl *APIController) ParlayRecommend(c *gin.Context) {
	var req struct {
		MatchIDs      []string `json:"matchIds"`
		ParlayMode    string   `json:"parlayMode"`
		ParlayOptions []string `json:"parlayOptions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.MatchIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少选择两场比赛。"})
		return
	}
	resp, err := ctrl.ParlayService.RecommendParlay(req.MatchIDs, req.ParlayMode, req.ParlayOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
