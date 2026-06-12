package prediction

import (
	"fifa2026/src/internal/models"
	"fmt"
	"math"
)

type PlayAdvice struct {
	PlayCode   string     `json:"playCode"`
	PlayName   string     `json:"playName"`
	Safe       PlayOption `json:"safe"`
	Aggressive PlayOption `json:"aggressive"`
}

type PlayOption struct {
	Option string  `json:"option"`
	Odds   float64 `json:"odds"`
	Prob   float64 `json:"prob"`
	EV     float64 `json:"ev"`
}

func (s *LotteryService) GenerateFivePlaysAdvice(match models.Match, report *models.PredictionReport) []PlayAdvice {
	var matrix []models.ScoreProbability
	var lh, la float64
	if report != nil {
		matrix = report.ScoreMatrix
		lh = report.RefinedParams.LambdaHome
		la = report.RefinedParams.LambdaAway
	} else {
		params := s.dcService.CalculateParams(match.HomeTeam, match.AwayTeam)
		matrix, _, _ = s.dcService.GenerateProbabilityMatrix(params)
		lh = params.LambdaHome
		la = params.LambdaAway
	}

	// 基础 Dixon-Coles 概率与参数，用于生成更具波动且逼真的仿真官方赔率（模拟在资讯偏差之前的初始赔率）
	baseParams := s.dcService.CalculateParams(match.HomeTeam, match.AwayTeam)
	baseMatrix, _, _ := s.dcService.GenerateProbabilityMatrix(baseParams)

	odds := s.sportteryService.GetMatchOdds(match.HomeTeam, match.AwayTeam)
	if !odds.IsAvailable {
		pHomeBase, pDrawBase, pAwayBase := 0.0, 0.0, 0.0
		for _, cell := range baseMatrix {
			if cell.HomeScore > cell.AwayScore {
				pHomeBase += cell.Prob
			} else if cell.HomeScore == cell.AwayScore {
				pDrawBase += cell.Prob
			} else {
				pAwayBase += cell.Prob
			}
		}
		odds.HomeOdds = math.Min(100.0, 0.89/math.Max(0.001, pHomeBase))
		odds.DrawOdds = math.Min(100.0, 0.89/math.Max(0.001, pDrawBase))
		odds.AwayOdds = math.Min(100.0, 0.89/math.Max(0.001, pAwayBase))
		odds.GoalLine = -1
		odds.IsAvailable = true
	}

	var advices []PlayAdvice

	// 1. 胜平负 (had)
	{
		pHome, pDraw, pAway := 0.0, 0.0, 0.0
		for _, cell := range matrix {
			if cell.HomeScore > cell.AwayScore {
				pHome += cell.Prob
			} else if cell.HomeScore == cell.AwayScore {
				pDraw += cell.Prob
			} else {
				pAway += cell.Prob
			}
		}
		oH, oD, oA := odds.HomeOdds, odds.DrawOdds, odds.AwayOdds
		evH := pHome*oH - 1.0
		evD := pDraw*oD - 1.0
		evA := pAway*oA - 1.0

		var safeOpt PlayOption
		if pHome >= pDraw && pHome >= pAway {
			safeOpt = PlayOption{"主胜", oH, pHome, evH}
		} else if pDraw >= pHome && pDraw >= pAway {
			safeOpt = PlayOption{"平局", oD, pDraw, evD}
		} else {
			safeOpt = PlayOption{"客胜", oA, pAway, evA}
		}

		var aggOpt PlayOption
		if evH >= evD && evH >= evA {
			aggOpt = PlayOption{"主胜", oH, pHome, evH}
		} else if evD >= evH && evD >= evA {
			aggOpt = PlayOption{"平局", oD, pDraw, evD}
		} else {
			aggOpt = PlayOption{"客胜", oA, pAway, evA}
		}

		advices = append(advices, PlayAdvice{"had", "胜平负", safeOpt, aggOpt})
	}

	// 2. 让球胜平负 (hhad)
	{
		gLine := odds.GoalLine
		if gLine == 0 {
			gLine = -1
		}
		pRHome, pRDraw, pRAway := 0.0, 0.0, 0.0
		for _, cell := range matrix {
			diff := cell.HomeScore - cell.AwayScore + gLine
			if diff > 0 {
				pRHome += cell.Prob
			} else if diff == 0 {
				pRDraw += cell.Prob
			} else {
				pRAway += cell.Prob
			}
		}

		oRH, oRD, oRA := odds.HhadHomeOdds, odds.HhadDrawOdds, odds.HhadAwayOdds
		if oRH <= 0 {
			pRHomeBase, pRDrawBase, pRAwayBase := 0.0, 0.0, 0.0
			for _, cell := range baseMatrix {
				diff := cell.HomeScore - cell.AwayScore + gLine
				if diff > 0 {
					pRHomeBase += cell.Prob
				} else if diff == 0 {
					pRDrawBase += cell.Prob
				} else {
					pRAwayBase += cell.Prob
				}
			}
			oRH = math.Min(100.0, 0.89/math.Max(0.001, pRHomeBase))
			oRD = math.Min(100.0, 0.89/math.Max(0.001, pRDrawBase))
			oRA = math.Min(100.0, 0.89/math.Max(0.001, pRAwayBase))
		}
		evRH := pRHome*oRH - 1.0
		evRD := pRDraw*oRD - 1.0
		evRA := pRAway*oRA - 1.0

		var safeOpt PlayOption
		if pRHome >= pRDraw && pRHome >= pRAway {
			safeOpt = PlayOption{fmt.Sprintf("让胜(%d)", gLine), oRH, pRHome, evRH}
		} else if pRDraw >= pRHome && pRDraw >= pRAway {
			safeOpt = PlayOption{fmt.Sprintf("让平(%d)", gLine), oRD, pRDraw, evRD}
		} else {
			safeOpt = PlayOption{fmt.Sprintf("让负(%d)", gLine), oRA, pRAway, evRA}
		}

		var aggOpt PlayOption
		if evRH >= evRD && evRH >= evRA {
			aggOpt = PlayOption{fmt.Sprintf("让胜(%d)", gLine), oRH, pRHome, evRH}
		} else if evRD >= evRH && evRD >= evRA {
			aggOpt = PlayOption{fmt.Sprintf("让平(%d)", gLine), oRD, pRDraw, evRD}
		} else {
			aggOpt = PlayOption{fmt.Sprintf("让负(%d)", gLine), oRA, pRAway, evRA}
		}
		advices = append(advices, PlayAdvice{"hhad", "让球胜平负", safeOpt, aggOpt})
	}
	// 3. 比分 (crs)
	{
		var safeOpt PlayOption
		var aggOpt PlayOption
		var maxProb, maxEV float64
		first := true

		for _, cell := range matrix {
			key := fmt.Sprintf("%d:%d", cell.HomeScore, cell.AwayScore)
			preciseKey := getPreciseCrsKey(cell.HomeScore, cell.AwayScore)
			oVal := odds.CrsOdds[preciseKey]
			if oVal <= 0 {
				oValBase := 0.0
				for _, cellB := range baseMatrix {
					if cellB.HomeScore == cell.HomeScore && cellB.AwayScore == cell.AwayScore {
						oValBase = 0.89 / math.Max(0.001, cellB.Prob)
						break
					}
				}
				if oValBase <= 0 {
					oValBase = 0.89 / math.Max(0.001, cell.Prob)
				}
				oVal = math.Min(100.0, oValBase)
			}
			ev := cell.Prob*oVal - 1.0

			if cell.Prob > maxProb || first {
				maxProb = cell.Prob
				safeOpt = PlayOption{key, oVal, cell.Prob, ev}
			}
			if ev > maxEV || first {
				maxEV = ev
				aggOpt = PlayOption{key, oVal, cell.Prob, ev}
			}
			first = false
		}
		advices = append(advices, PlayAdvice{"crs", "比分", safeOpt, aggOpt})
	}

	// 4. 总进球数 (ttg)
	{
		ttgProbs := make([]float64, 8)
		for _, cell := range matrix {
			tot := cell.HomeScore + cell.AwayScore
			if tot >= 7 {
				ttgProbs[7] += cell.Prob
			} else {
				ttgProbs[tot] += cell.Prob
			}
		}

		baseTtgProbs := make([]float64, 8)
		for _, cell := range baseMatrix {
			tot := cell.HomeScore + cell.AwayScore
			if tot >= 7 {
				baseTtgProbs[7] += cell.Prob
			} else {
				baseTtgProbs[tot] += cell.Prob
			}
		}

		var safeOpt PlayOption
		var aggOpt PlayOption
		var maxProb, maxEV float64
		first := true

		for g := 0; g <= 7; g++ {
			prob := ttgProbs[g]
			apiCode := fmt.Sprintf("s%d", g)
			oVal := odds.TtgOdds[apiCode]
			if oVal <= 0 {
				oVal = math.Min(100.0, 0.89/math.Max(0.001, baseTtgProbs[g]))
			}
			ev := prob*oVal - 1.0
			name := fmt.Sprintf("%d球", g)
			if g == 7 {
				name = "7+球"
			}

			if prob > maxProb || first {
				maxProb = prob
				safeOpt = PlayOption{name, oVal, prob, ev}
			}
			if ev > maxEV || first {
				maxEV = ev
				aggOpt = PlayOption{name, oVal, prob, ev}
			}
			first = false
		}
		advices = append(advices, PlayAdvice{"ttg", "总进球数", safeOpt, aggOpt})
	}

	// 5. 半全场胜平负 (hafu)
	{
		hafuProbs := make(map[string]float64)
		options := []string{"胜胜", "胜平", "胜负", "平胜", "平平", "平负", "负胜", "负平", "负负"}
		for _, op := range options {
			hafuProbs[op] = 0.0
		}
		lhHalf := lh * 0.5
		laHalf := la * 0.5
		lhSecond := lh * 0.5
		laSecond := la * 0.5

		for hHome := 0; hHome <= 4; hHome++ {
			for hAway := 0; hAway <= 4; hAway++ {
				pHalf := s.dcService.ComputePoissonProb(lhHalf, hHome) * s.dcService.ComputePoissonProb(laHalf, hAway)
				for sHome := 0; sHome <= 4; sHome++ {
					for sAway := 0; sAway <= 4; sAway++ {
						pSec := s.dcService.ComputePoissonProb(lhSecond, sHome) * s.dcService.ComputePoissonProb(laSecond, sAway)
						pJoint := pHalf * pSec
						var halfState string
						if hHome > hAway {
							halfState = "胜"
						} else if hHome == hAway {
							halfState = "平"
						} else {
							halfState = "负"
						}
						fHome := hHome + sHome
						fAway := hAway + sAway
						var fullState string
						if fHome > fAway {
							fullState = "胜"
						} else if fHome == fAway {
							fullState = "平"
						} else {
							fullState = "负"
						}
						hafuProbs[halfState+fullState] += pJoint
					}
				}
			}
		}

		baseHafuProbs := make(map[string]float64)
		for _, op := range options {
			baseHafuProbs[op] = 0.0
		}
		lhHalfBase := baseParams.LambdaHome * 0.5
		laHalfBase := baseParams.LambdaAway * 0.5
		lhSecondBase := baseParams.LambdaHome * 0.5
		laSecondBase := baseParams.LambdaAway * 0.5

		for hHome := 0; hHome <= 4; hHome++ {
			for hAway := 0; hAway <= 4; hAway++ {
				pHalf := s.dcService.ComputePoissonProb(lhHalfBase, hHome) * s.dcService.ComputePoissonProb(laHalfBase, hAway)
				for sHome := 0; sHome <= 4; sHome++ {
					for sAway := 0; sAway <= 4; sAway++ {
						pSec := s.dcService.ComputePoissonProb(lhSecondBase, sHome) * s.dcService.ComputePoissonProb(laSecondBase, sAway)
						pJoint := pHalf * pSec
						var halfState string
						if hHome > hAway {
							halfState = "胜"
						} else if hHome == hAway {
							halfState = "平"
						} else {
							halfState = "负"
						}
						fHome := hHome + sHome
						fAway := hAway + sAway
						var fullState string
						if fHome > fAway {
							fullState = "胜"
						} else if fHome == fAway {
							fullState = "平"
						} else {
							fullState = "负"
						}
						baseHafuProbs[halfState+fullState] += pJoint
					}
				}
			}
		}

		hafuKeys := map[string]string{
			"胜胜": "hh", "胜平": "hd", "胜负": "ha",
			"平胜": "dh", "平平": "dd", "平负": "da",
			"负胜": "ah", "负平": "ad", "负负": "aa",
		}

		var safeOpt PlayOption
		var aggOpt PlayOption
		var maxProb, maxEV float64
		first := true

		for op, prob := range hafuProbs {
			apiCode := hafuKeys[op]
			oVal := odds.HafuOdds[apiCode]
			if oVal <= 0 {
				oVal = math.Min(100.0, 0.89/math.Max(0.001, baseHafuProbs[op]))
			}
			ev := prob*oVal - 1.0

			if prob > maxProb || first {
				maxProb = prob
				safeOpt = PlayOption{op, oVal, prob, ev}
			}
			if ev > maxEV || first {
				maxEV = ev
				aggOpt = PlayOption{op, oVal, prob, ev}
			}
			first = false
		}
		advices = append(advices, PlayAdvice{"hafu", "半全场胜平负", safeOpt, aggOpt})
	}

	return advices
}
