package v1

import (
	"fifa2026/src/internal/db"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GetBetMatches 获取当前参与投注建议的在售未开赛比赛列表
func (ctrl *APIController) GetBetMatches(c *gin.Context) {
	startDateStr := c.Query("startDate")
	if startDateStr == "" {
		startDateStr = c.Query("date")
	}
	if startDateStr == "" {
		startDateStr = time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	}
	endDateStr := c.Query("endDate")
	if endDateStr == "" {
		endDateStr = startDateStr
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	tStart, errS := time.ParseInLocation("2006-01-02", startDateStr, loc)
	if errS != nil {
		tStart = time.Now().AddDate(0, 0, 1)
	}
	tEnd, errE := time.ParseInLocation("2006-01-02", endDateStr, loc)
	if errE != nil {
		tEnd = tStart
	}

	startOfDay := time.Date(tStart.Year(), tStart.Month(), tStart.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(tEnd.Year(), tEnd.Month(), tEnd.Day(), 23, 59, 59, 0, loc)

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
