package db

import (
	"database/sql"
	"fifa2026/src/internal/models"
	"time"
)

// SaveLotteryPlan 插入或更新方案，确保同类未结算方案不重复
func SaveLotteryPlan(p models.LotteryPlan) error {
	// 首先删除同类待结算的方案，保证幂等性
	if p.PlanType == "single" {
		_, _ = DB.Exec("DELETE FROM lottery_plans WHERE plan_type = 'single' AND match_ids = ? AND is_settled = 0", p.MatchIDs)
	} else {
		_, _ = DB.Exec("DELETE FROM lottery_plans WHERE plan_type = 'parlay' AND match_ids = ? AND parlay_type = ? AND is_settled = 0", p.MatchIDs, p.ParlayType)
	}

	query := `INSERT INTO lottery_plans (
		plan_type, match_ids, risk_level, odds_h, odds_d, odds_a,
		primary_bet, primary_odds, primary_amt, hedge_bet, hedge_odds, hedge_amt,
		parlay_type, parlay_mode, parlay_options, desc_str, wins_count, cost,
		single_ticket_payout, combo_odds, combo_prob, total_ev, kelly_stake, tickets_json,
		is_settled, safe_profit, safe_return, agg_profit, agg_return
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := DB.Exec(query,
		p.PlanType, p.MatchIDs, p.RiskLevel, p.OddsH, p.OddsD, p.OddsA,
		p.PrimaryBet, p.PrimaryOdds, p.PrimaryAmt, p.HedgeBet, p.HedgeOdds, p.HedgeAmt,
		p.ParlayType, p.ParlayMode, p.ParlayOptions, p.DescStr, p.WinsCount, p.Cost,
		p.SingleTicketPayout, p.ComboOdds, p.ComboProb, p.TotalEV, p.KellyStake, p.TicketsJSON,
		p.IsSettled, p.SafeProfit, p.SafeReturn, p.AggProfit, p.AggReturn,
	)
	return err
}

// GetUnsettledLotteryPlans 获取所有未结算的方案
func GetUnsettledLotteryPlans() ([]models.LotteryPlan, error) {
	query := `SELECT id, plan_type, match_ids, risk_level, odds_h, odds_d, odds_a,
		primary_bet, primary_odds, primary_amt, hedge_bet, hedge_odds, hedge_amt,
		parlay_type, parlay_mode, parlay_options, desc_str, wins_count, cost,
		single_ticket_payout, combo_odds, combo_prob, total_ev, kelly_stake, tickets_json,
		is_settled, safe_profit, safe_return, agg_profit, agg_return, created_at
		FROM lottery_plans WHERE is_settled = 0`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLotteryPlans(rows)
}

// GetSettledLotteryPlans 获取已结算的方案
func GetSettledLotteryPlans() ([]models.LotteryPlan, error) {
	query := `SELECT id, plan_type, match_ids, risk_level, odds_h, odds_d, odds_a,
		primary_bet, primary_odds, primary_amt, hedge_bet, hedge_odds, hedge_amt,
		parlay_type, parlay_mode, parlay_options, desc_str, wins_count, cost,
		single_ticket_payout, combo_odds, combo_prob, total_ev, kelly_stake, tickets_json,
		is_settled, safe_profit, safe_return, agg_profit, agg_return, created_at
		FROM lottery_plans WHERE is_settled = 1 ORDER BY created_at DESC`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLotteryPlans(rows)
}

// UpdateLotteryPlanSettlement 保存方案的财务复盘结算数据
func UpdateLotteryPlanSettlement(id int64, safeReturn, safeProfit, aggReturn, aggProfit float64) error {
	query := `UPDATE lottery_plans SET is_settled = 1, safe_return = ?, safe_profit = ?, agg_return = ?, agg_profit = ? WHERE id = ?`
	_, err := DB.Exec(query, safeReturn, safeProfit, aggReturn, aggProfit, id)
	return err
}

func scanLotteryPlans(rows *sql.Rows) ([]models.LotteryPlan, error) {
	var plans []models.LotteryPlan
	for rows.Next() {
		var p models.LotteryPlan
		var createdStr string
		err := rows.Scan(
			&p.ID, &p.PlanType, &p.MatchIDs, &p.RiskLevel, &p.OddsH, &p.OddsD, &p.OddsA,
			&p.PrimaryBet, &p.PrimaryOdds, &p.PrimaryAmt, &p.HedgeBet, &p.HedgeOdds, &p.HedgeAmt,
			&p.ParlayType, &p.ParlayMode, &p.ParlayOptions, &p.DescStr, &p.WinsCount, &p.Cost,
			&p.SingleTicketPayout, &p.ComboOdds, &p.ComboProb, &p.TotalEV, &p.KellyStake, &p.TicketsJSON,
			&p.IsSettled, &p.SafeProfit, &p.SafeReturn, &p.AggProfit, &p.AggReturn, &createdStr,
		)
		if err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05-07:00", createdStr)
		if p.CreatedAt.IsZero() {
			p.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		}
		plans = append(plans, p)
	}
	return plans, nil
}

// GetSavedLotteryPlans 获取所有已保存的方案（无论是否结算）
func GetSavedLotteryPlans() ([]models.LotteryPlan, error) {
	query := `SELECT id, plan_type, match_ids, risk_level, odds_h, odds_d, odds_a,
		primary_bet, primary_odds, primary_amt, hedge_bet, hedge_odds, hedge_amt,
		parlay_type, parlay_mode, parlay_options, desc_str, wins_count, cost,
		single_ticket_payout, combo_odds, combo_prob, total_ev, kelly_stake, tickets_json,
		is_settled, safe_profit, safe_return, agg_profit, agg_return, created_at
		FROM lottery_plans ORDER BY id DESC`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanLotteryPlans(rows)
}

// DeleteLotteryPlans 根据 ID 列表删除已保存的方案
func DeleteLotteryPlans(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	query := "DELETE FROM lottery_plans WHERE id IN ("
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	_, err := DB.Exec(query, args...)
	return err
}

