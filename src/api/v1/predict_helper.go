package v1

import (
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/prediction"
)

// calibrateMatrixWithOdds 调用辛氏去抽水折算真实市场概率，并与 Dixon-Coles 矩阵进行共识加权校准
func (ctrl *APIController) calibrateMatrixWithOdds(matrix []models.ScoreProbability, odds prediction.OfficialOdds) ([]models.ScoreProbability, float64, float64) {
	var over25, under25 float64

	// 胜平负 (HAD) 辛氏去抽水融合校准
	probs, _, errOdds := ctrl.ShinService.DevigOdds([]float64{odds.HomeOdds, odds.DrawOdds, odds.AwayOdds})
	if errOdds == nil && len(probs) >= 3 {
		var sumDCHome, sumDCDraw, sumDCAway float64
		for _, cell := range matrix {
			if cell.HomeScore > cell.AwayScore {
				sumDCHome += cell.Prob
			} else if cell.HomeScore == cell.AwayScore {
				sumDCDraw += cell.Prob
			} else {
				sumDCAway += cell.Prob
			}
		}

		weightMarket := 0.15
		weightModel := 0.85

		finalHome := weightMarket*probs[0] + weightModel*sumDCHome
		finalDraw := weightMarket*probs[1] + weightModel*sumDCDraw
		finalAway := weightMarket*probs[2] + weightModel*sumDCAway

		for i := range matrix {
			if matrix[i].HomeScore > matrix[i].AwayScore {
				if sumDCHome > 0 {
					matrix[i].Prob = matrix[i].Prob * (finalHome / sumDCHome)
				}
			} else if matrix[i].HomeScore == matrix[i].AwayScore {
				if sumDCDraw > 0 {
					matrix[i].Prob = matrix[i].Prob * (finalDraw / sumDCDraw)
				}
			} else {
				if sumDCAway > 0 {
					matrix[i].Prob = matrix[i].Prob * (finalAway / sumDCAway)
				}
			}
		}
	}

	// 让球胜平负 (HHAD) 辛氏去抽水融合校准
	if odds.IsAvailable && odds.HhadHomeOdds > 0.0 && odds.HhadDrawOdds > 0.0 && odds.HhadAwayOdds > 0.0 {
		hhadProbs, _, errHhad := ctrl.ShinService.DevigOdds([]float64{odds.HhadHomeOdds, odds.HhadDrawOdds, odds.HhadAwayOdds})
		if errHhad == nil && len(hhadProbs) >= 3 {
			var sumDCHhadHome, sumDCHhadDraw, sumDCHhadAway float64
			for _, cell := range matrix {
				diff := cell.HomeScore - cell.AwayScore + odds.GoalLine
				if diff > 0 {
					sumDCHhadHome += cell.Prob
				} else if diff == 0 {
					sumDCHhadDraw += cell.Prob
				} else {
					sumDCHhadAway += cell.Prob
				}
			}

			weightMarket := 0.15
			weightModel := 0.85

			finalHhadHome := weightMarket*hhadProbs[0] + weightModel*sumDCHhadHome
			finalHhadDraw := weightMarket*hhadProbs[1] + weightModel*sumDCHhadDraw
			finalHhadAway := weightMarket*hhadProbs[2] + weightModel*sumDCHhadAway

			for i := range matrix {
				diff := matrix[i].HomeScore - matrix[i].AwayScore + odds.GoalLine
				if diff > 0 {
					if sumDCHhadHome > 0 {
						matrix[i].Prob = matrix[i].Prob * (finalHhadHome / sumDCHhadHome)
					}
				} else if diff == 0 {
					if sumDCHhadDraw > 0 {
						matrix[i].Prob = matrix[i].Prob * (finalHhadDraw / sumDCHhadDraw)
					}
				} else {
					if sumDCHhadAway > 0 {
						matrix[i].Prob = matrix[i].Prob * (finalHhadAway / sumDCHhadAway)
					}
				}
			}

			// 归一化以保障比分总体概率和为 1
			total := 0.0
			for _, cell := range matrix {
				total += cell.Prob
			}
			if total > 0 {
				for i := range matrix {
					matrix[i].Prob /= total
				}
			}
		}
	}

	// 最终重新计算校准后的大小球概率
	for _, cell := range matrix {
		if cell.HomeScore+cell.AwayScore > 2 {
			over25 += cell.Prob
		} else {
			under25 += cell.Prob
		}
	}

	return matrix, over25, under25
}
