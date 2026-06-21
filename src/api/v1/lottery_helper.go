package v1

import (
	"fifa2026/src/internal/db"
	"fmt"
	"strings"
)

// getPreciseCrsKey 将具体的比分映射为体彩的官方分类键名 (例如胜其它、负其它)
func getPreciseCrsKey(homeScore, awayScore int) string {
	if homeScore > 5 || awayScore > 5 {
		if homeScore > awayScore {
			return "s1sh" // 胜其它
		} else if homeScore == awayScore {
			return "s1sd" // 平其它
		} else {
			return "s1sa" // 负其它
		}
	}
	isAvailable := false
	if homeScore > awayScore {
		if (homeScore == 1 && awayScore == 0) ||
			(homeScore == 2 && (awayScore == 0 || awayScore == 1)) ||
			(homeScore == 3 && (awayScore == 0 || awayScore == 1 || awayScore == 2)) ||
			(homeScore == 4 && (awayScore == 0 || awayScore == 1 || awayScore == 2)) ||
			(homeScore == 5 && (awayScore == 0 || awayScore == 1 || awayScore == 2)) {
			isAvailable = true
		}
	} else if homeScore == awayScore {
		if homeScore >= 0 && homeScore <= 3 {
			isAvailable = true
		}
	} else {
		if (awayScore == 1 && homeScore == 0) ||
			(awayScore == 2 && (homeScore == 0 || homeScore == 1)) ||
			(awayScore == 3 && (homeScore == 0 || homeScore == 1 || homeScore == 2)) ||
			(awayScore == 4 && (homeScore == 0 || homeScore == 1 || homeScore == 2)) ||
			(awayScore == 5 && (homeScore == 0 || homeScore == 1 || homeScore == 2)) {
			isAvailable = true
		}
	}
	if isAvailable {
		return fmt.Sprintf("s%02ds%02d", homeScore, awayScore)
	}
	if homeScore > awayScore {
		return "s1sh"
	} else if homeScore == awayScore {
		return "s1sd"
	}
	return "s1sa"
}

// checkLegHit 判定具体的某个投注选项在当前比赛比分下是否命中
func checkLegHit(matchID string, option string) bool {
	m, err := db.GetMatch(matchID)
	if err != nil {
		return false
	}
	h, a := m.HomeScore, m.AwayScore

	// 1. 胜平负 (had)
	if option == "主胜" || option == "主胜 (3)" {
		return h > a
	}
	if option == "平局" || option == "平局 (1)" {
		return h == a
	}
	if option == "客胜" || option == "客胜 (0)" {
		return h < a
	}

	// 2. 让球 (hhad)
	if strings.Contains(option, "让胜") {
		var gLine int
		fmt.Sscanf(option, "让胜(%d)", &gLine)
		if gLine == 0 {
			gLine = -1
		}
		return h - a + gLine > 0
	}
	if strings.Contains(option, "让平") {
		var gLine int
		fmt.Sscanf(option, "让平(%d)", &gLine)
		if gLine == 0 {
			gLine = -1
		}
		return h - a + gLine == 0
	}
	if strings.Contains(option, "让负") {
		var gLine int
		fmt.Sscanf(option, "让负(%d)", &gLine)
		if gLine == 0 {
			gLine = -1
		}
		return h - a + gLine < 0
	}

	// 3. 总进球数 (ttg)
	if strings.HasSuffix(option, "球") {
		if option == "7+球" {
			return h+a >= 7
		}
		var goals int
		_, err := fmt.Sscanf(option, "%d球", &goals)
		if err == nil {
			return h+a == goals
		}
	}

	// 4. 比分 (crs)
	if strings.Contains(option, ":") {
		var hGoal, aGoal int
		_, err := fmt.Sscanf(option, "%d:%d", &hGoal, &aGoal)
		if err == nil {
			return h == hGoal && a == aGoal
		}
	}
	if option == "胜其它" || option == "s1sh" {
		return getPreciseCrsKey(h, a) == "s1sh"
	}
	if option == "平其它" || option == "s1sd" {
		return getPreciseCrsKey(h, a) == "s1sd"
	}
	if option == "负其它" || option == "s1sa" {
		return getPreciseCrsKey(h, a) == "s1sa"
	}

	return false
}

// LegWithResult 带有比分及命中结果的单关明细
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

// TicketWithResult 带有串关赔率、预计返还及各场结果的子单票
type TicketWithResult struct {
	Odds   float64         `json:"odds"`
	Payout float64         `json:"payout"`
	Legs   []LegWithResult `json:"legs"`
}
