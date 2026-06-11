package prediction

import (
	"fifa2026/src/internal/models"
	"math"
)

type TimeDecayService struct {
	dc *DixonColesService
}

func NewTimeDecayService(dc *DixonColesService) *TimeDecayService {
	return &TimeDecayService{dc: dc}
}

// CalculateRemainLambda 计算在比赛第 minute 分钟时，剩余时间内的进球期望值
// 采用指数时间衰减模型 lambda_remain = lambda_0 * ((90-t)/90) * exp(-gamma * t/90)
func (s *TimeDecayService) CalculateRemainLambda(lambdaZero float64, minute int) float64 {
	if minute >= 90 {
		return 0.0
	}
	t := float64(minute)
	remainRatio := (90.0 - t) / 90.0

	// 时间衰减系数 gamma (体能下降与战术收缩的指数影响)
	gamma := 0.22
	decay := math.Exp(-gamma * (t / 90.0))

	return lambdaZero * remainRatio * decay
}

// PredictRemainGoals 预测从第 minute 分钟起，剩余时间内的进球数概率分布
// currentHome, currentAway 分别为当前比分
func (s *TimeDecayService) PredictRemainGoals(p models.DixonColesParams, minute int, currentHome, currentAway int) ([]models.ScoreProbability, float64, float64) {
	// 1. 计算主客队剩余时间进球期望 λ'
	lambdaHomeRemain := s.CalculateRemainLambda(p.LambdaHome, minute)
	lambdaAwayRemain := s.CalculateRemainLambda(p.LambdaAway, minute)

	remainParams := models.DixonColesParams{
		LambdaHome: lambdaHomeRemain,
		LambdaAway: lambdaAwayRemain,
		Rho:        p.Rho * ((90.0 - float64(minute)) / 90.0), // 平局相关因子随着时间衰减而向 0 收敛
	}

	// 2. 利用 Dixon-Coles 算法计算剩余进球的联合概率，并叠加当前比分
	var matrix []models.ScoreProbability
	var over2_5, under2_5 float64

	// 计算剩余时间内最多进 5 个球的概率
	for x := 0; x <= 5; x++ {
		for y := 0; y <= 5; y++ {
			prob := s.dc.ComputeJointProbability(x, y, remainParams)
			finalHome := currentHome + x
			finalAway := currentAway + y

			matrix = append(matrix, models.ScoreProbability{
				HomeScore: finalHome,
				AwayScore: finalAway,
				Prob:      prob,
			})

			// 总体大小球判定 (基于全场总比分是否大于 2.5)
			if finalHome+finalAway > 2 {
				over2_5 += prob
			} else {
				under2_5 += prob
			}
		}
	}

	// 归一化
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
