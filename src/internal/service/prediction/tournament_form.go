package prediction

import (
	"fifa2026/src/internal/db"
	"fmt"
	"math"
	"time"
)

// TeamForm 球队在本届赛事中的实时表现
type TeamForm struct {
	Team          string   `json:"team"`
	Played        int      `json:"played"`
	Won           int      `json:"won"`
	Drawn         int      `json:"drawn"`
	Lost          int      `json:"lost"`
	GoalsFor      int      `json:"goalsFor"`
	GoalsAgainst  int      `json:"goalsAgainst"`
	RecentResults []string `json:"recentResults"` // ["W4-1 vs Paraguay", "D1-1 vs Bosnia"]
	FormString    string   `json:"formString"`    // "WDL" 最近3场
}

// GetTeamForm 从已完赛 matches 表计算指定球队在本届世界杯的实时表现
func GetTeamForm(teamName string) TeamForm {
	form := TeamForm{Team: teamName}

	matches, err := db.GetMatchesByTeam("fifa_2026", teamName)
	if err != nil || len(matches) == 0 {
		return form
	}

	for _, m := range matches {
		if m.Status != "FT" {
			continue
		}
		form.Played++

		isHome := m.HomeTeam == teamName
		var goalsFor, goalsAgainst int
		var opponent string
		if isHome {
			goalsFor = m.HomeScore
			goalsAgainst = m.AwayScore
			opponent = m.AwayTeam
		} else {
			goalsFor = m.AwayScore
			goalsAgainst = m.HomeScore
			opponent = m.HomeTeam
		}

		form.GoalsFor += goalsFor
		form.GoalsAgainst += goalsAgainst

		var result string
		if goalsFor > goalsAgainst {
			form.Won++
			result = fmt.Sprintf("W%d-%d vs %s", goalsFor, goalsAgainst, opponent)
		} else if goalsFor == goalsAgainst {
			form.Drawn++
			result = fmt.Sprintf("D%d-%d vs %s", goalsFor, goalsAgainst, opponent)
		} else {
			form.Lost++
			result = fmt.Sprintf("L%d-%d vs %s", goalsFor, goalsAgainst, opponent)
		}
		form.RecentResults = append(form.RecentResults, result)
	}

	// 生成最近 3 场 FormString
	n := len(form.RecentResults)
	start := 0
	if n > 3 {
		start = n - 3
	}
	for i := start; i < n; i++ {
		form.FormString += string(form.RecentResults[i][0]) // W/D/L
	}

	return form
}

// GetFormLambdaOffset 根据本届赛事表现计算 λ 偏移
func GetFormLambdaOffset(teamName string) float64 {
	form := GetTeamForm(teamName)
	if form.Played == 0 {
		return 0.0
	}

	// 连胜加成 / 连败惩罚
	winRate := float64(form.Won) / float64(form.Played)
	if winRate >= 1.0 && form.Played >= 2 {
		return 0.04 // 全胜状态
	}
	if winRate == 0 && form.Lost >= 2 {
		return -0.04 // 全败状态
	}
	return 0.0
}

// GetRestDaysOffset 计算休息天数 λ 偏移
func GetRestDaysOffset(teamName string, matchDate time.Time) float64 {
	matches, err := db.GetMatchesByTeam("fifa_2026", teamName)
	if err != nil {
		return 0.0
	}

	var lastFinished time.Time
	for _, m := range matches {
		if m.Status == "FT" && m.ScheduledAt.Before(matchDate) {
			if m.ScheduledAt.After(lastFinished) {
				lastFinished = m.ScheduledAt
			}
		}
	}

	if lastFinished.IsZero() {
		return 0.0 // 首场比赛，无疲劳
	}

	restDays := math.Floor(matchDate.Sub(lastFinished).Hours() / 24)
	if restDays <= 2 {
		return -0.06 // 严重疲劳
	} else if restDays == 3 {
		return -0.02 // 轻度疲劳
	}
	return 0.0
}

// DetectMotivation 小组赛末轮战意检测
func DetectMotivation(teamName, group string, matchday int) float64 {
	if matchday < 3 {
		return 0.0 // 仅末轮检测
	}

	// 获取该组所有已完赛比赛，计算当前积分榜
	standings := calcGroupStandings(group)
	if len(standings) == 0 {
		return 0.0
	}

	teamPts := 0
	teamRank := 0
	for i, s := range standings {
		if s.team == teamName {
			teamPts = s.points
			teamRank = i + 1
			break
		}
	}

	// 简化判定：
	// 前2名且积分领先第3名3+分 → 已确保出线
	if teamRank <= 2 && len(standings) >= 3 {
		if teamPts-standings[2].points >= 3 {
			return -0.04 // 已确保出线，可能轮换
		}
	}

	// 第4名且落后第3名3+分 → 已无出线可能
	if teamRank == 4 && len(standings) >= 3 {
		if standings[2].points-teamPts >= 4 {
			return -0.08 // 已淘汰
		}
	}

	// 第3/4名且差距小 → 背水一战
	if teamRank >= 3 {
		return 0.03
	}

	return 0.0
}

type standingEntry struct {
	team   string
	points int
}

func calcGroupStandings(group string) []standingEntry {
	matches, err := db.GetMatchesByGroup("fifa_2026", group)
	if err != nil {
		return nil
	}

	pts := make(map[string]int)
	for _, m := range matches {
		if m.Status != "FT" {
			continue
		}
		if m.HomeScore > m.AwayScore {
			pts[m.HomeTeam] += 3
		} else if m.HomeScore == m.AwayScore {
			pts[m.HomeTeam] += 1
			pts[m.AwayTeam] += 1
		} else {
			pts[m.AwayTeam] += 3
		}
	}

	var standings []standingEntry
	for team, p := range pts {
		standings = append(standings, standingEntry{team: team, points: p})
	}

	// 按积分降序排序
	for i := 0; i < len(standings); i++ {
		for j := i + 1; j < len(standings); j++ {
			if standings[j].points > standings[i].points {
				standings[i], standings[j] = standings[j], standings[i]
			}
		}
	}

	return standings
}
