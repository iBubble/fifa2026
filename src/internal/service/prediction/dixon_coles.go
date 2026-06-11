package prediction

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"math"
)

type DixonColesService struct {
	elo       *EloService
	rhoOffset float64
}

func NewDixonColesService(elo *EloService) *DixonColesService {
	s := &DixonColesService{
		elo:       elo,
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

	// 历史世界杯进球率拟合出的回归模型参数 (指数映射)
	lambdaH := math.Exp(0.12 + 0.35*diff)
	lambdaA := math.Exp(-0.12 - 0.35*diff)

	// 经典平局相关系数初始值
	rho := -0.08

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
