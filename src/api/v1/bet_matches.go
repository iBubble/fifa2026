package v1

import (
	"fifa2026/src/internal/db"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GetBetMatches 获取当前参与投注建议的在售未开赛比赛列表
func (ctrl *APIController) GetBetMatches(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		dateStr = time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	t, errT := time.ParseInLocation("2006-01-02", dateStr, loc)
	if errT != nil {
		t = time.Now().AddDate(0, 0, 1)
	}
	startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.AddDate(0, 0, 1).Add(-time.Second)

	allMatches, err := db.GetMatchesByTournament("fifa_2026")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var list []gin.H
	for _, m := range allMatches {
		if m.Status != "NS" {
			continue
		}
		mLocal := m.ScheduledAt.In(loc)
		if mLocal.Before(startOfDay) || mLocal.After(endOfDay) {
			continue
		}
		odds := ctrl.SportteryService.GetMatchOdds(m.HomeTeam, m.AwayTeam, m.ScheduledAt)
		if !odds.IsAvailable {
			continue
		}
		homeCn := m.HomeTeam
		tHome, errH := db.GetTeamTranslation(m.HomeTeam)
		if errH == nil && tHome.CnName != "" {
			homeCn = tHome.CnName
		}
		awayCn := m.AwayTeam
		tAway, errA := db.GetTeamTranslation(m.AwayTeam)
		if errA == nil && tAway.CnName != "" {
			awayCn = tAway.CnName
		}
		list = append(list, gin.H{
			"id":        m.ID,
			"homeTeam":  m.HomeTeam,
			"awayTeam":  m.AwayTeam,
			"homeCn":    homeCn,
			"awayCn":    awayCn,
			"matchTime": m.ScheduledAt.Format("06-01-02 15:04"),
		})
	}
	c.JSON(http.StatusOK, list)
}
