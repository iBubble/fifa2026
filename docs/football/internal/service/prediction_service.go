// Package service - prediction_service.go
// 实现核心量化算法：凯利公式（Kelly Criterion）、套利检测（Arbitrage）和权重预测
package service

import (
	"fmt"
	"math"
	"time"

	"football/internal/models"
)

// PredictionService 提供量化预测相关的计算能力
type PredictionService struct{}

// NewPredictionService 构造函数
func NewPredictionService() *PredictionService {
	return &PredictionService{}
}

// ─────────────────────────────────────────────────────────────
// 凯利公式 (Kelly Criterion)
// ─────────────────────────────────────────────────────────────

// CalculateKelly 计算凯利公式建议投注分数
//
// 凯利公式: f* = (b*p - q) / b
//   - b = odds - 1 (净赔率, 即每押1元赢得的纯利润)
//   - p = 估计胜率 (win probability)
//   - q = 1 - p (败率)
//   - f* = 最优押注资金占比
//
// params.Fraction 用于控制"分数凯利"（通常取 0.25~0.5 以降低波动风险）
func (s *PredictionService) CalculateKelly(params models.KellyParams) (models.KellyResult, error) {
	// 参数校验
	if params.Odds <= 1.0 {
		return models.KellyResult{}, fmt.Errorf("赔率必须大于 1.0，当前值: %.2f", params.Odds)
	}
	if params.WinProb <= 0 || params.WinProb >= 1 {
		return models.KellyResult{}, fmt.Errorf("胜率必须在 (0, 1) 范围内，当前值: %.4f", params.WinProb)
	}
	if params.Bankroll <= 0 {
		return models.KellyResult{}, fmt.Errorf("资金额必须大于 0，当前值: %.2f", params.Bankroll)
	}
	if params.Fraction <= 0 || params.Fraction > 1 {
		params.Fraction = 0.25 // 默认 1/4 凯利
	}

	b := params.Odds - 1.0 // 净赔率
	p := params.WinProb
	q := 1.0 - p

	// 核心凯利公式
	kellyF := (b*p - q) / b

	// 期望值 EV = p * odds - 1
	ev := p*params.Odds - 1.0

	// 赔率隐含概率 vs 估计胜率 的边际
	impliedProb := 1.0 / params.Odds
	edge := (p - impliedProb) / impliedProb * 100

	// 调整后凯利（分数凯利，降低方差）
	adjustedF := kellyF * params.Fraction

	// 若期望值为负或凯利为负，不建议下注
	if kellyF <= 0 || ev <= 0 {
		return models.KellyResult{
			KellyFraction:    kellyF,
			AdjustedFraction: 0,
			SuggestedStake:   0,
			ExpectedValue:    ev,
			EdgePct:          edge,
		}, nil
	}

	// 建议投注金额（向下取整到2位小数）
	suggestedStake := math.Floor(params.Bankroll*adjustedF*100) / 100

	return models.KellyResult{
		KellyFraction:    kellyF,
		AdjustedFraction: adjustedF,
		SuggestedStake:   suggestedStake,
		ExpectedValue:    ev,
		EdgePct:          edge,
	}, nil
}

// ─────────────────────────────────────────────────────────────
// 套利检测 (Arbitrage Scanning)
// ─────────────────────────────────────────────────────────────

// CheckArbitrage 检测赔率集合中是否存在套利机会
//
// 套利公式: L = Σ(1/odds_i)
//   - L < 1: 存在无风险套利（各选项对应赔率之和的倒数之和小于1）
//   - ROI = (1/L - 1) * 100%（理论最大收益率）
//
// 最优资金分配（使各腿盈亏对等）:
//   - stake_i = (1/odds_i / L) * total_bankroll
//
// legs: 每条腿的赔率信息（{bookmaker, outcome, odds}）
// bankroll: 总可投注资金
func (s *PredictionService) CheckArbitrage(
	matchID string,
	match models.Match,
	market models.MarketType,
	legs []models.ArbLeg,
	bankroll float64,
) (models.ArbitrageOpportunity, bool) {

	if len(legs) < 2 {
		return models.ArbitrageOpportunity{}, false
	}

	// 计算 L = Σ(1/odds_i)
	lValue := 0.0
	for _, leg := range legs {
		if leg.Odds <= 0 {
			return models.ArbitrageOpportunity{}, false
		}
		lValue += 1.0 / leg.Odds
	}

	// L >= 1：无套利空间
	if lValue >= 1.0 {
		return models.ArbitrageOpportunity{}, false
	}

	// 计算收益率
	roi := (1.0/lValue - 1.0) * 100

	// 计算每条腿的最优资金分配
	resultLegs := make([]models.ArbLeg, len(legs))
	for i, leg := range legs {
		stakePct := (1.0 / leg.Odds) / lValue // 资金占比
		stakeAmt := math.Floor(stakePct*bankroll*100) / 100
		resultLegs[i] = models.ArbLeg{
			Bookmaker: leg.Bookmaker,
			Outcome:   leg.Outcome,
			Odds:      leg.Odds,
			StakePct:  stakePct * 100, // 转换为百分比
			StakeAmt:  stakeAmt,
		}
	}

	return models.ArbitrageOpportunity{
		MatchID:    matchID,
		Match:      match,
		Market:     market,
		LValue:     lValue,
		ROI:        roi,
		Legs:       resultLegs,
		DetectedAt: time.Now(),
	}, true
}

// ─────────────────────────────────────────────────────────────
// 权重预测 (Weighted Prediction)
// ─────────────────────────────────────────────────────────────

// TeamFormData 球队近期表现数据（近5场）
type TeamFormData struct {
	WinRate  float64 // 近5场胜率
	GoalsFor float64 // 近5场场均进球
	GoalsAga float64 // 近5场场均失球
	XGFor    float64 // 近5场场均 xG 进攻
	XGAga    float64 // 近5场场均 xG 防守
}

// H2HData 历史对阵数据
type H2HData struct {
	HomeWinRate float64 // 主队历史交锋胜率
	TotalGoals  float64 // 历史场均总进球
}

// PredictionInput 预测模型输入
type PredictionInput struct {
	HomeForm TeamFormData
	AwayForm TeamFormData
	H2H      H2HData
	// 赔率隐含概率（从市场赔率反推）
	OddsImpliedHomeWin float64
	OddsImpliedDraw    float64
	OddsImpliedAwayWin float64
	// 权重配置
	Weights models.PredictionWeights
}

// PredictionOutput 预测模型输出
type PredictionOutput struct {
	HomeWinProb float64 `json:"homeWinProb"` // 主队胜概率
	DrawProb    float64 `json:"drawProb"`    // 平局概率
	AwayWinProb float64 `json:"awayWinProb"` // 客队胜概率
	Confidence  float64 `json:"confidence"`  // 预测置信度（0~1）
}

// WeightedPredict 基于多维度权重的胜平负概率预测
//
// 模型逻辑：
// 1. 近期形态信号 → 主队胜概率基础值
// 2. 历史对阵信号 → 修正系数
// 3. 赔率隐含概率 → 市场智慧参考
// 4. xG 数据      → 进攻/防守能力评估
// 5. 加权融合，归一化后输出概率
func (s *PredictionService) WeightedPredict(input PredictionInput) PredictionOutput {
	w := input.Weights

	// ── 信号1: 近期状态信号（主队相对优势）
	formHomeAdv := (input.HomeForm.WinRate - input.AwayForm.WinRate) * 0.5
	// 归一化到 [0.2, 0.8]
	formHomeProb := clamp(0.40+formHomeAdv, 0.20, 0.75)

	// ── 信号2: 历史对阵信号
	h2hHomeProb := clamp(input.H2H.HomeWinRate, 0.20, 0.75)

	// ── 信号3: 赔率隐含概率（市场已去除博彩商利润）
	oddsSum := input.OddsImpliedHomeWin + input.OddsImpliedDraw + input.OddsImpliedAwayWin
	var oddsHomeProb float64
	if oddsSum > 0 {
		oddsHomeProb = input.OddsImpliedHomeWin / oddsSum
	} else {
		oddsHomeProb = 0.45
	}

	// ── 信号4: xG 信号（进攻动能对比）
	homeXGAdv := (input.HomeForm.XGFor - input.AwayForm.XGFor) * 0.1
	homeXGDef := (input.AwayForm.XGAga - input.HomeForm.XGAga) * 0.05
	xgHomeProb := clamp(0.40+homeXGAdv+homeXGDef, 0.20, 0.75)

	// ── 加权融合（权重归一化）
	totalW := w.FormWeight + w.H2HWeight + w.OddsWeight + w.XGWeight
	if totalW == 0 {
		totalW = 1
	}
	rawHomeProb := (w.FormWeight*formHomeProb +
		w.H2HWeight*h2hHomeProb +
		w.OddsWeight*oddsHomeProb +
		w.XGWeight*xgHomeProb) / totalW

	// 简单三分法（主平客）
	// 客队胜率 ≈ (1 - rawHomeProb) * awayFormRatio
	awayFormRatio := clamp(input.AwayForm.WinRate/(input.HomeForm.WinRate+0.01), 0.3, 1.5)
	rawAwayProb := (1 - rawHomeProb) * awayFormRatio * 0.5
	rawDrawProb := 1 - rawHomeProb - rawAwayProb

	// 归一化确保三者之和为 1
	total := rawHomeProb + rawDrawProb + rawAwayProb
	homeWin := rawHomeProb / total
	draw := rawDrawProb / total
	awayWin := rawAwayProb / total

	// 置信度：各信号之间的方差越小，置信度越高
	variance := computeVariance([]float64{formHomeProb, h2hHomeProb, oddsHomeProb, xgHomeProb})
	confidence := clamp(1-variance*5, 0.3, 0.95)

	return PredictionOutput{
		HomeWinProb: math.Round(homeWin*10000) / 10000,
		DrawProb:    math.Round(draw*10000) / 10000,
		AwayWinProb: math.Round(awayWin*10000) / 10000,
		Confidence:  math.Round(confidence*10000) / 10000,
	}
}

// ─────────────────────────────────────────────────────────────
// 辅助函数
// ─────────────────────────────────────────────────────────────

// clamp 将值约束在 [min, max] 范围内
func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// computeVariance 计算一组数的方差（衡量信号一致性）
func computeVariance(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	vari := 0.0
	for _, v := range vals {
		d := v - mean
		vari += d * d
	}
	return vari / float64(len(vals))
}
