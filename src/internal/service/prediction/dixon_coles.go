package prediction

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"math"
)

type DixonColesService struct {
	elo       *EloService
	apiSports *APISportsService
	rhoOffset float64
}

func NewDixonColesService(elo *EloService, apiSports *APISportsService) *DixonColesService {
	s := &DixonColesService{
		elo:       elo,
		apiSports: apiSports,
		rhoOffset: 0.0,
	}
	s.RecalculateRhoOffset()
	return s
}

// CalculateParams 根据两队当前 Elo 计算 Dixon-Coles 初始期望参数（包含自适应偏移）
func (s *DixonColesService) CalculateParams(homeTeam, awayTeam string) models.DixonColesParams {
	p := s.CalculateParamsWithoutOffset(homeTeam, awayTeam)
	p.Rho += s.rhoOffset
	// 强约束 rho 的范围在 [-0.15, -0.01] 之间
	if p.Rho > -0.01 {
		p.Rho = -0.01
	}
	if p.Rho < -0.15 {
		p.Rho = -0.15
	}
	return p
}

func (s *DixonColesService) CalculateParamsWithoutOffset(homeTeam, awayTeam string) models.DixonColesParams {
	eloH := s.elo.GetElo(homeTeam)
	eloA := s.elo.GetElo(awayTeam)
	diff := (eloH - eloA) / 400.0

	// 整合历史场均进失球特征 (交战攻防底蕴)
	featH := s.elo.GetFeature(homeTeam)
	featA := s.elo.GetFeature(awayTeam)

	// 计算基础进球概率倾向 (几何平均数以平衡两队攻防，除以 1.35 归一化以维持传统比分数学中位数期望)
	baseH := math.Sqrt(featH.AvgGoalsScored * featA.AvgGoalsConceded) / 1.35
	baseA := math.Sqrt(featA.AvgGoalsScored * featH.AvgGoalsConceded) / 1.35

	// 融入 api-football 历史直接交锋记录 (H2H) 场均进球数并与模型大盘进球率进行自适应加权混合
	var h2hDiff float64
	var hasH2H bool
	var drawRate float64
	var totalH2HMatches int
	if s.apiSports != nil {
		h2h, err := s.apiSports.GetH2HRecord(homeTeam, awayTeam)
		if err == nil && h2h.TotalMatches > 0 {
			hasH2H = true
			totalH2HMatches = h2h.TotalMatches
			// 自适应 H2H 权重：交手次数越多，克制权重越高，上限为 30%
			h2hWeight := math.Min(0.30, float64(h2h.TotalMatches)*0.08)
			baseH = (1.0-h2hWeight)*baseH + h2hWeight*h2h.AvgHomeGoals
			baseA = (1.0-h2hWeight)*baseA + h2hWeight*h2h.AvgAwayGoals
			// 计算双方历史交手胜率差值倾向 (值范围 -1.0 到 1.0)
			h2hDiff = (float64(h2h.HomeWins) - float64(h2h.AwayWins)) / float64(h2h.TotalMatches)
			drawRate = float64(h2h.Draws) / float64(h2h.TotalMatches)
		}
	}

	// 安全上限与下限约束，防止异常边界值导致数学溢出
	if baseH <= 0.2 {
		baseH = 0.2
	}
	if baseA <= 0.2 {
		baseA = 0.2
	}

	// 最终复合进球期望 Lambda：将基础攻防与实时 Elo 实力差的指数级加权调节相叠加
	lambdaH := baseH * math.Exp(0.12 + 0.35*diff)
	lambdaA := baseA * math.Exp(-0.12 - 0.35*diff)

	// 如果存在历史交战统计，继续叠加上 15% 上下浮动的胜率克制修正系数
	if hasH2H {
		lambdaH = lambdaH * (1.0 + 0.15*h2hDiff)
		lambdaA = lambdaA * (1.0 - 0.15*h2hDiff)
	}

	// 经典平局相关系数初始值
	rho := -0.08
	if hasH2H && totalH2HMatches >= 3 {
		if drawRate >= 0.35 {
			// 平局倾向强：负向加深 rho，强化 0-0/1-1 概率倾向
			rho -= 0.10 * drawRate
		} else if drawRate == 0 {
			// 无平局倾向：削弱平局因子
			rho += 0.04
		}
	}

	return models.DixonColesParams{
		LambdaHome: lambdaH,
		LambdaAway: lambdaA,
		Rho:        rho,
	}
}

// ComputePoissonProb 经典泊松分布概率质量计算
func (s *DixonColesService) ComputePoissonProb(lambda float64, k int) float64 {
	if k < 0 {
		return 0
	}
	return (math.Pow(lambda, float64(k)) * math.Exp(-lambda)) / float64(factorial(k))
}

// ComputeJointProbability Dixon-Coles 修正后的主客队联合比分概率
func (s *DixonColesService) ComputeJointProbability(x, y int, p models.DixonColesParams) float64 {
	pHome := s.ComputePoissonProb(p.LambdaHome, x)
	pAway := s.ComputePoissonProb(p.LambdaAway, y)
	rawJoint := pHome * pAway

	// Dixon-Coles 修正因子 tau
	tau := 1.0
	if x == 0 && y == 0 {
		tau = 1.0 - p.LambdaHome*p.LambdaAway*p.Rho
	} else if x == 1 && y == 0 {
		tau = 1.0 + p.LambdaAway*p.Rho
	} else if x == 0 && y == 1 {
		tau = 1.0 + p.LambdaHome*p.Rho
	} else if x == 1 && y == 1 {
		tau = 1.0 - p.Rho
	}

	return math.Max(0, rawJoint*tau) // 确保概率非负
}

// GenerateProbabilityMatrix 生成 6x6 精确比分概率矩阵，并推算大小球概率
func (s *DixonColesService) GenerateProbabilityMatrix(p models.DixonColesParams) ([]models.ScoreProbability, float64, float64) {
	var matrix []models.ScoreProbability
	var over2_5, under2_5 float64

	for x := 0; x <= 5; x++ {
		for y := 0; y <= 5; y++ {
			prob := s.ComputeJointProbability(x, y, p)
			matrix = append(matrix, models.ScoreProbability{
				HomeScore: x,
				AwayScore: y,
				Prob:      prob,
			})
			if x+y > 2 {
				over2_5 += prob
			} else {
				under2_5 += prob
			}
		}
	}

	// 进行归一化，确保总体概率和等于 1 (针对 6x6 以外被截断的微小尾部概率)
	total := 0.0
	for _, cell := range matrix {
		total += cell.Prob
	}
	if total > 0 {
		for i := range matrix {
			matrix[i].Prob /= total
		}
		over2_5 /= total
		under2_5 /= total
	}

	return matrix, over2_5, under2_5
}

func factorial(n int) int {
	if n <= 0 {
		return 1
	}
	f := 1
	for i := 1; i <= n; i++ {
		f *= f
	}
	// 修复原先可能死循环或精度丢失：阶乘应为 f *= i
	f = 1
	for i := 2; i <= n; i++ {
		f *= i
	}
	return f
}

// RecalculateRhoOffset 根据所有已结算比赛的历史 Brier Score 纠偏自适应平局系数
func (s *DixonColesService) RecalculateRhoOffset() {
	reports, err := db.GetBacktestReports()
	if err != nil || len(reports) == 0 {
		s.rhoOffset = 0.0
		return
	}

	offset := 0.0
	learningRate := 0.05

	for _, r := range reports {
		m, err := db.GetMatch(r.MatchID)
		if err != nil {
			continue
		}

		pBase := s.CalculateParamsWithoutOffset(m.HomeTeam, m.AwayTeam)
		p := pBase
		p.Rho += offset
		if p.Rho > -0.01 {
			p.Rho = -0.01
		}
		if p.Rho < -0.15 {
			p.Rho = -0.15
		}

		matrix, _, _ := s.GenerateProbabilityMatrix(p)
		pDraw := 0.0
		for _, cell := range matrix {
			if cell.HomeScore == cell.AwayScore {
				pDraw += cell.Prob
			}
		}

		oDraw := 0.0
		if m.HomeScore == m.AwayScore {
			oDraw = 1.0
		}

		// 反馈自校准：如果实际平局而预测偏低，需要 rho 变得更负（即 offset 变小/负），以提高平局修正作用
		delta := learningRate * (oDraw - pDraw) * r.BrierScore
		offset -= delta

		if offset > 0.07 {
			offset = 0.07
		}
		if offset < -0.07 {
			offset = -0.07
		}
	}
	s.rhoOffset = offset
}
