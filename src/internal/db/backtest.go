package db

import (
	"database/sql"
	"time"
)

type DbBacktestReport struct {
	MatchID       string    `json:"matchId"`
	BrierScore    float64   `json:"brierScore"`
	HomeEloDiff   float64   `json:"homeEloDiff"`
	AwayEloDiff   float64   `json:"awayEloDiff"`
	TacticsReview string    `json:"tacticsReview"`
	ReviewedAt    time.Time `json:"reviewedAt"`
}

// SaveBacktestReport 保存或更新赛后复盘报告
func SaveBacktestReport(r DbBacktestReport) error {
	query := `INSERT INTO backtest_reports (match_id, brier_score, home_elo_diff, away_elo_diff, tactics_review)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(match_id) DO UPDATE SET
			brier_score = excluded.brier_score,
			home_elo_diff = excluded.home_elo_diff,
			away_elo_diff = excluded.away_elo_diff,
			tactics_review = excluded.tactics_review`
	_, err := DB.Exec(query, r.MatchID, r.BrierScore, r.HomeEloDiff, r.AwayEloDiff, r.TacticsReview)
	return err
}

// GetBacktestReports 获取全部已结算的赛后复盘报告
func GetBacktestReports() ([]DbBacktestReport, error) {
	query := `SELECT match_id, brier_score, home_elo_diff, away_elo_diff, tactics_review, reviewed_at
		FROM backtest_reports ORDER BY reviewed_at ASC`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []DbBacktestReport
	for rows.Next() {
		var r DbBacktestReport
		var reviewedStr string
		err := rows.Scan(&r.MatchID, &r.BrierScore, &r.HomeEloDiff, &r.AwayEloDiff, &r.TacticsReview, &reviewedStr)
		if err != nil {
			return nil, err
		}
		// 容错解析不同格式的时间戳
		r.ReviewedAt, err = time.Parse("2006-01-02 15:04:05", reviewedStr)
		if err != nil {
			r.ReviewedAt, _ = time.Parse(time.RFC3339, reviewedStr)
		}
		reports = append(reports, r)
	}
	return reports, nil
}

// GetBacktestReport 获取单场比赛的复盘报告
func GetBacktestReport(matchID string) (DbBacktestReport, error) {
	query := `SELECT match_id, brier_score, home_elo_diff, away_elo_diff, tactics_review, reviewed_at
		FROM backtest_reports WHERE match_id = ?`
	row := DB.QueryRow(query, matchID)
	var r DbBacktestReport
	var reviewedStr string
	err := row.Scan(&r.MatchID, &r.BrierScore, &r.HomeEloDiff, &r.AwayEloDiff, &r.TacticsReview, &reviewedStr)
	if err == sql.ErrNoRows {
		return DbBacktestReport{}, err
	} else if err != nil {
		return DbBacktestReport{}, err
	}
	r.ReviewedAt, err = time.Parse("2006-01-02 15:04:05", reviewedStr)
	if err != nil {
		r.ReviewedAt, _ = time.Parse(time.RFC3339, reviewedStr)
	}
	return r, nil
}
