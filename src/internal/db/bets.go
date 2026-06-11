package db

import (
	"fifa2026/src/internal/models"
	"time"
)

// AddBet 新增投注记录
func AddBet(b models.Bet) (int64, error) {
	query := `INSERT INTO bets (tournament_id, match_id, home_team, away_team, bookmaker, market, outcome, odds, stake, result, pnl, kelly_fraction, consensus_prob, expected_value)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := DB.Exec(query, b.TournamentID, b.MatchID, b.HomeTeam, b.AwayTeam, b.Bookmaker, b.Market, b.Outcome, b.Odds, b.Stake, b.Result, b.PnL, b.KellyFraction, b.ConsensusProb, b.ExpectedValue)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateBetResult 结算投注
func UpdateBetResult(betID int64, result string, pnl float64) error {
	query := `UPDATE bets SET result = ?, pnl = ? WHERE id = ?`
	_, err := DB.Exec(query, result, pnl, betID)
	return err
}

// GetBets 获取指定赛事的所有投注流水
func GetBets(tournamentID string) ([]models.Bet, error) {
	query := `SELECT id, tournament_id, match_id, home_team, away_team, bookmaker, market, outcome, odds, stake, result, pnl, kelly_fraction, consensus_prob, expected_value, placed_at
		FROM bets WHERE tournament_id = ? ORDER BY placed_at DESC`
	rows, err := DB.Query(query, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bets []models.Bet
	for rows.Next() {
		var b models.Bet
		var placedStr string
		err := rows.Scan(&b.ID, &b.TournamentID, &b.MatchID, &b.HomeTeam, &b.AwayTeam, &b.Bookmaker, &b.Market, &b.Outcome, &b.Odds, &b.Stake, &b.Result, &b.PnL, &b.KellyFraction, &b.ConsensusProb, &b.ExpectedValue, &placedStr)
		if err != nil {
			return nil, err
		}
		b.PlacedAt, _ = time.Parse("2006-01-02 15:04:05-07:00", placedStr)
		if b.PlacedAt.IsZero() {
			b.PlacedAt, _ = time.Parse(time.RFC3339, placedStr)
		}
		bets = append(bets, b)
	}
	return bets, nil
}

// GetBetSummary 获取指定赛事的账本汇总数据
func GetBetSummary(tournamentID string) (models.BetSummary, error) {
	query := `SELECT COUNT(id), 
		COALESCE(SUM(CASE WHEN result = 'WIN' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN result = 'LOSS' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(stake), 0.0), 
		COALESCE(SUM(pnl), 0.0)
		FROM bets WHERE tournament_id = ?`
	row := DB.QueryRow(query, tournamentID)

	var summary models.BetSummary
	err := row.Scan(&summary.TotalBets, &summary.WinCount, &summary.LossCount, &summary.TotalStake, &summary.TotalPnL)
	if err != nil {
		return models.BetSummary{}, err
	}

	if summary.TotalBets > 0 {
		summary.WinRate = float64(summary.WinCount) / float64(summary.TotalBets) * 100
		if summary.TotalStake > 0 {
			summary.ROI = (summary.TotalPnL / summary.TotalStake) * 100
		}
	}

	// 计算最大回撤 (简化版：资金水线最大滑落)
	summary.MaxDrawdown = calculateMaxDrawdown(tournamentID)
	return summary, nil
}

// calculateMaxDrawdown 计算历史最大回撤
func calculateMaxDrawdown(tournamentID string) float64 {
	query := `SELECT pnl FROM bets WHERE tournament_id = ? AND result != 'PENDING' ORDER BY placed_at ASC`
	rows, err := DB.Query(query, tournamentID)
	if err != nil {
		return 0.0
	}
	defer rows.Close()

	peak := 0.0
	currentBalance := 0.0
	maxDd := 0.0

	for rows.Next() {
		var pnl float64
		if err := rows.Scan(&pnl); err != nil {
			continue
		}
		currentBalance += pnl
		if currentBalance > peak {
			peak = currentBalance
		}
		dd := peak - currentBalance
		if dd > maxDd {
			maxDd = dd
		}
	}
	return maxDd
}
