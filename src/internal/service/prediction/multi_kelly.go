package prediction

import (
	"fifa2026/src/internal/models"
	"math"
)

type MultiKellyService struct{}

func NewMultiKellyService() *MultiKellyService {
	return &MultiKellyService{}
}

// CalculateSingleKelly 计算单项凯利值
func (s *MultiKellyService) CalculateSingleKelly(p models.KellyParams) models.KellyResult {
	if p.Odds <= 1.0 || p.WinProb <= 0.0 {
		return models.KellyResult{}
	}

	b := p.Odds - 1.0 // 净赔率
	q := 1.0 - p.WinProb

	// f* = (b*p - q) / b = p - q/b
	rawF := (b*p.WinProb - q) / b
	ev := p.WinProb*p.Odds - 1.0

	adjF := rawF * p.Fraction
	if adjF < 0 {
		adjF = 0 // 屏蔽负期望
	}

	return models.KellyResult{
		RawFraction:      rawF,
		AdjustedFraction: adjF,
		SuggestedStake:   adjF * p.Bankroll,
		ExpectedValue:    ev,
	}
}

// AllocateMultiBets 并行投注资金分配。输入多个候选投注项及总本金、风险系数，输出优化的下注方案
// 对正 EV 注单执行杠杆暴露上限约束优化 (Leverage-Constrained Multi-bet Kelly)
func (s *MultiKellyService) AllocateMultiBets(bets []models.ValueBet, bankroll float64, riskFraction float64, maxTotalExposure float64) []models.ValueBet {
	if len(bets) == 0 || bankroll <= 0 {
		return nil
	}

	// 1. 先计算每个注单的单体凯利值
	var activeBets []models.ValueBet
	sumKelly := 0.0

	for _, b := range bets {
		if b.EV <= 0 {
			continue // 过滤掉负期望注单
		}

		netOdds := b.Odds - 1.0
		// 基础凯利比例
		fStar := (netOdds*b.SystemProb - (1.0 - b.SystemProb)) / netOdds

		// 结合风险系数折算 (如 1/4 凯利)
		fStarAdj := fStar * riskFraction
		if fStarAdj > 0 {
			b.KellyStake = fStarAdj
			sumKelly += fStarAdj
			activeBets = append(activeBets, b)
		}
	}

	if len(activeBets) == 0 {
		return nil
	}

	// 2. 杠杆暴露控制。如果所有注单的建议资金比例和超过了最大暴露额 (如 50%)，等比例收缩
	scale := 1.0
	if sumKelly > maxTotalExposure {
		scale = maxTotalExposure / sumKelly
	}

	for i := range activeBets {
		activeBets[i].KellyStake = math.Min(activeBets[i].KellyStake*scale, 0.20) // 单笔注单最高暴露 20%
	}

	return activeBets
}
