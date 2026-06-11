package db

import (
	"fifa2026/src/internal/models"
	"time"
)

// SaveOddsSnapshot 保存赔率快照
func SaveOddsSnapshot(matchID string, bookmaker string, home, draw, away float64) error {
	query := `INSERT INTO odds_history (match_id, bookmaker, home_odds, draw_odds, away_odds)
		VALUES (?, ?, ?, ?, ?)`
	_, err := DB.Exec(query, matchID, bookmaker, home, draw, away)
	return err
}

// GetLatestOdds 获取某场比赛各平台的最新的赔率报价
func GetLatestOdds(matchID string) ([]models.OddsRecord, error) {
	query := `SELECT h.id, h.match_id, h.bookmaker, h.home_odds, h.draw_odds, h.away_odds, h.captured_at
		FROM odds_history h
		INNER JOIN (
			SELECT bookmaker, MAX(captured_at) as max_cap 
			FROM odds_history 
			WHERE match_id = ? 
			GROUP BY bookmaker
		) latest ON h.bookmaker = latest.bookmaker AND h.captured_at = latest.max_cap
		WHERE h.match_id = ?`
	rows, err := DB.Query(query, matchID, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.OddsRecord
	for rows.Next() {
		var r models.OddsRecord
		var capStr string
		err := rows.Scan(&r.ID, &r.MatchID, &r.Bookmaker, &r.HomeOdds, &r.DrawOdds, &r.AwayOdds, &capStr)
		if err != nil {
			return nil, err
		}
		r.CapturedAt, _ = time.Parse("2006-01-02 15:04:05-07:00", capStr)
		if r.CapturedAt.IsZero() {
			r.CapturedAt, _ = time.Parse(time.RFC3339, capStr)
		}
		records = append(records, r)
	}
	return records, nil
}
