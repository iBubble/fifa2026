// Package db - repository.go 提供所有数据库的 CRUD 操作
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"football/internal/models"
)

// parseTimeRobust 强健的时间解析器，自动适应 SQLite 各种写入格式及 Go time.Time 的默认 String 输出
func parseTimeRobust(s string) time.Time {
	s = strings.TrimSpace(s)
	// 如果包含 Go 的单调时间信息 m=+...，先将其截断
	if idx := strings.Index(s, " m="); idx != -1 {
		s = s[:idx]
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05.999999999 -07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ─────────────────────────────────────────────────────────────
// Match CRUD
// ─────────────────────────────────────────────────────────────

// UpsertMatch 插入或更新比赛记录（INSERT OR REPLACE）
func UpsertMatch(m models.Match) error {
	_, err := DB.Exec(`
		INSERT INTO matches (id, home_team, away_team, league, country, scheduled_at, status, home_score, away_score, minute, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			home_team=excluded.home_team,
			away_team=excluded.away_team,
			league=excluded.league,
			country=excluded.country,
			scheduled_at=excluded.scheduled_at,
			status=excluded.status,
			home_score=excluded.home_score,
			away_score=excluded.away_score,
			minute=excluded.minute,
			updated_at=CURRENT_TIMESTAMP
	`, m.ID, m.HomeTeam, m.AwayTeam, m.League, m.Country,
		m.ScheduledAt, m.Status, m.HomeScore, m.AwayScore, m.Minute)
	return err
}

// DeleteMockMatches 清理数据库中所有的模拟比赛数据（当拉取到真实数据时调用）
func DeleteMockMatches() error {
	_, err := DB.Exec(`DELETE FROM matches WHERE id LIKE 'wc2026-%'`)
	return err
}

// GetMatches 查询所有比赛，按开赛时间升序排列
func GetMatches() ([]models.Match, error) {
	rows, err := DB.Query(`
		SELECT id, home_team, away_team, league, country, scheduled_at, status, home_score, away_score, minute
		FROM matches ORDER BY scheduled_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var scheduledAt string
		err := rows.Scan(&m.ID, &m.HomeTeam, &m.AwayTeam, &m.League, &m.Country,
			&scheduledAt, &m.Status, &m.HomeScore, &m.AwayScore, &m.Minute)
		if err != nil {
			return nil, err
		}
		m.ScheduledAt = parseTimeRobust(scheduledAt)
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

// GetMatchesByLeague 按联赛名称过滤查询比赛
func GetMatchesByLeague(league string) ([]models.Match, error) {
	rows, err := DB.Query(`
		SELECT id, home_team, away_team, league, country, scheduled_at, status, home_score, away_score, minute
		FROM matches WHERE league = ? ORDER BY scheduled_at ASC
	`, league)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var scheduledAt string
		err := rows.Scan(&m.ID, &m.HomeTeam, &m.AwayTeam, &m.League, &m.Country,
			&scheduledAt, &m.Status, &m.HomeScore, &m.AwayScore, &m.Minute)
		if err != nil {
			return nil, err
		}
		m.ScheduledAt = parseTimeRobust(scheduledAt)
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

// ─────────────────────────────────────────────────────────────
// OddsHistory CRUD
// ─────────────────────────────────────────────────────────────

// SaveOddsSnapshot 保存一次赔率快照（批量写入所有平台所有盘口）
func SaveOddsSnapshot(snapshot models.OddsSnapshot) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO odds_history (match_id, bookmaker, market, outcome, odds)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, bk := range snapshot.Bookmakers {
		for _, outcome := range bk.Outcomes {
			if _, err := stmt.Exec(snapshot.MatchID, bk.Bookmaker, string(bk.Market), outcome.Name, outcome.Price); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// GetOddsHistory 查询某场比赛的历史赔率（最近 N 条）
func GetOddsHistory(matchID string, limit int) ([]models.OddsSnapshot, error) {
	rows, err := DB.Query(`
		SELECT bookmaker, market, outcome, odds, captured_at
		FROM odds_history
		WHERE match_id = ?
		ORDER BY captured_at DESC
		LIMIT ?
	`, matchID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 按时间聚合成 snapshot
	snapshotMap := make(map[string]*models.OddsSnapshot)
	for rows.Next() {
		var (
			bookmaker, market, outcome string
			odds                       float64
			capturedAt                 string
		)
		if err := rows.Scan(&bookmaker, &market, &outcome, &odds, &capturedAt); err != nil {
			return nil, err
		}
		t := parseTimeRobust(capturedAt)

		key := fmt.Sprintf("%s|%s|%s", bookmaker, market, capturedAt)
		if _, ok := snapshotMap[key]; !ok {
			snapshotMap[key] = &models.OddsSnapshot{
				MatchID:    matchID,
				CapturedAt: t,
			}
		}
		// 简化处理：将赔率数据附加到 bookmaker
		found := false
		for i := range snapshotMap[key].Bookmakers {
			if snapshotMap[key].Bookmakers[i].Bookmaker == bookmaker {
				snapshotMap[key].Bookmakers[i].Outcomes = append(
					snapshotMap[key].Bookmakers[i].Outcomes,
					models.OddsOutcome{Name: outcome, Price: odds},
				)
				found = true
				break
			}
		}
		if !found {
			snapshotMap[key].Bookmakers = append(snapshotMap[key].Bookmakers, models.BookmakerOdds{
				Bookmaker: bookmaker,
				Market:    models.MarketType(market),
				Outcomes:  []models.OddsOutcome{{Name: outcome, Price: odds}},
				UpdatedAt: t,
			})
		}
	}

	var result []models.OddsSnapshot
	for _, s := range snapshotMap {
		result = append(result, *s)
	}
	return result, rows.Err()
}

// ─────────────────────────────────────────────────────────────
// Bet CRUD（投注账本）
// ─────────────────────────────────────────────────────────────

// AddBet 新增一条投注记录
func AddBet(bet models.Bet) (int64, error) {
	res, err := DB.Exec(`
		INSERT INTO bets (match_id, home_team, away_team, bookmaker, market, outcome, odds, stake, result, pnl, kelly_fraction, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, bet.MatchID, bet.HomeTeam, bet.AwayTeam, bet.Bookmaker, bet.Market,
		bet.Outcome, bet.Odds, bet.Stake, string(bet.Result), bet.PnL, bet.KellyFraction, bet.Notes)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateBetResult 更新投注结果（结算时调用）
func UpdateBetResult(betID int64, result models.BetResult, pnl float64) error {
	_, err := DB.Exec(`
		UPDATE bets SET result=?, pnl=?, settled_at=CURRENT_TIMESTAMP WHERE id=?
	`, string(result), pnl, betID)
	return err
}

// GetBets 查询所有投注记录（按下注时间降序）
func GetBets() ([]models.Bet, error) {
	rows, err := DB.Query(`
		SELECT id, match_id, home_team, away_team, bookmaker, market, outcome,
		       odds, stake, result, pnl, kelly_fraction, placed_at, settled_at, notes
		FROM bets ORDER BY placed_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bets []models.Bet
	for rows.Next() {
		var (
			b            models.Bet
			placedAt     string
			settledAt    sql.NullString
			resultStr    string
		)
		err := rows.Scan(&b.ID, &b.MatchID, &b.HomeTeam, &b.AwayTeam,
			&b.Bookmaker, &b.Market, &b.Outcome,
			&b.Odds, &b.Stake, &resultStr, &b.PnL, &b.KellyFraction,
			&placedAt, &settledAt, &b.Notes)
		if err != nil {
			return nil, err
		}
		b.Result = models.BetResult(resultStr)
		b.PlacedAt = parseTimeRobust(placedAt)
		if settledAt.Valid {
			t := parseTimeRobust(settledAt.String)
			b.SettledAt = &t
		}
		bets = append(bets, b)
	}
	return bets, rows.Err()
}

// GetBetSummary 计算账本汇总统计
func GetBetSummary() (models.BetSummary, error) {
	row := DB.QueryRow(`
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN result='WIN' THEN 1 ELSE 0 END) as wins,
			SUM(CASE WHEN result='LOSS' THEN 1 ELSE 0 END) as losses,
			COALESCE(SUM(stake), 0) as total_stake,
			COALESCE(SUM(pnl), 0) as total_pnl,
			COALESCE(AVG(odds), 0) as avg_odds
		FROM bets WHERE result != 'PENDING'
	`)

	var s models.BetSummary
	err := row.Scan(&s.TotalBets, &s.WinCount, &s.LossCount,
		&s.TotalStake, &s.TotalPnL, &s.AvgOdds)
	if err != nil {
		return s, err
	}
	if s.TotalBets > 0 {
		s.WinRate = float64(s.WinCount) / float64(s.TotalBets) * 100
	}
	if s.TotalStake > 0 {
		s.ROI = s.TotalPnL / s.TotalStake * 100
	}
	return s, nil
}

// ─────────────────────────────────────────────────────────────
// ArbitrageLog CRUD
// ─────────────────────────────────────────────────────────────

// SaveArbitrageOpportunity 记录套利机会到数据库
func SaveArbitrageOpportunity(opp models.ArbitrageOpportunity) error {
	detailsJSON, err := json.Marshal(opp.Legs)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`
		INSERT INTO arbitrage_log (match_id, market, l_value, roi, details)
		VALUES (?, ?, ?, ?, ?)
	`, opp.MatchID, string(opp.Market), opp.LValue, opp.ROI, string(detailsJSON))
	return err
}
