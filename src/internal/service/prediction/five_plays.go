package prediction

import (
	"fifa2026/src/internal/models"
	"fmt"
	"sort"
	"strings"
)

type PlayAdvice struct {
	PlayCode        string       `json:"playCode"`
	PlayName        string       `json:"playName"`
	Safe            []PlayOption `json:"safe"`
	Aggressive      []PlayOption `json:"aggressive"`
	SingleAvailable bool         `json:"singleAvailable"`
	SingleTip       string       `json:"singleTip"`
}

type PlayOption struct {
	Option string  `json:"option"`
	Odds   float64 `json:"odds"`
	Prob   float64 `json:"prob"`
	EV     float64 `json:"ev"`
}

func (s *LotteryService) GenerateFivePlaysAdvice(match models.Match, report *models.PredictionReport, isSingleHad bool) []PlayAdvice {
	var matrix []models.ScoreProbability
	var lh, la float64
	if report != nil {
		matrix = report.ScoreMatrix
		lh = report.RefinedParams.LambdaHome
		la = report.RefinedParams.LambdaAway
	} else {
		params := s.dcService.CalculateParamsWithVenue(match.HomeTeam, match.AwayTeam, match.Venue)
		matrix, _, _ = s.dcService.GenerateProbabilityMatrix(params)
		lh = params.LambdaHome
		la = params.LambdaAway
	}

	// 融入博彩巨头实时赔率偏移对比分概率矩阵的调整
	matrix = applyShiftsToMatrix(match.HomeTeam, match.AwayTeam, matrix)

	// 基础 Dixon-Coles 概率与参数，用于生成更具波动且逼真的仿真官方赔率（模拟在资讯偏差之前的初始赔率）
	baseParams := s.dcService.CalculateParamsWithVenue(match.HomeTeam, match.AwayTeam, match.Venue)
	baseMatrix, _, _ := s.dcService.GenerateProbabilityMatrix(baseParams)

	odds := s.sportteryService.GetMatchOdds(match.HomeTeam, match.AwayTeam, match.ScheduledAt)
	if !odds.IsAvailable {
		var advices []PlayAdvice
		advices = append(advices, PlayAdvice{"had", "胜平负", []PlayOption{{"不可售", 0.0, 0.0, 0.0}}, []PlayOption{{"不可售", 0.0, 0.0, 0.0}}, false, "玩法当前未开售"})
		advices = append(advices, PlayAdvice{"hhad", "让球胜平负", []PlayOption{{"不可售", 0.0, 0.0, 0.0}}, []PlayOption{{"不可售", 0.0, 0.0, 0.0}}, false, "玩法当前未开售"})
		advices = append(advices, PlayAdvice{"crs", "比分", []PlayOption{{"不可售", 0.0, 0.0, 0.0}}, []PlayOption{{"不可售", 0.0, 0.0, 0.0}}, false, "玩法当前未开售"})
		advices = append(advices, PlayAdvice{"ttg", "总进球数", []PlayOption{{"不可售", 0.0, 0.0, 0.0}}, []PlayOption{{"不可售", 0.0, 0.0, 0.0}}, false, "玩法当前未开售"})
		advices = append(advices, PlayAdvice{"hafu", "半全场胜平负", []PlayOption{{"不可售", 0.0, 0.0, 0.0}}, []PlayOption{{"不可售", 0.0, 0.0, 0.0}}, false, "玩法当前未开售"})
		return advices
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

		var safeOpts []PlayOption
		var aggOpts []PlayOption

		// 校验：如果官方已开售其他玩法，但当前常规胜平负未售，标记为不可售并将赔率归零，防止用户购买
		if odds.IsAvailable && oH <= 0.0 {
			safeOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
			aggOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
		} else {
			opts := []PlayOption{
				{"主胜", oH, pHome, evH},
				{"平局", oD, pDraw, evD},
				{"客胜", oA, pAway, evA},
			}
			optsSafe := make([]PlayOption, len(opts))
			copy(optsSafe, opts)
			safeOpts = getTop3Safe(optsSafe)

			optsAgg := make([]PlayOption, len(opts))
			copy(optsAgg, opts)
			aggOpts = getTop3Aggressive(optsAgg)
		}

		var singleTip string
		isSingleAvailable := odds.HadSingle
		if len(safeOpts) > 0 && safeOpts[0].Option == "不可售" {
			isSingleAvailable = false
			singleTip = "玩法当前未开售"
		} else if odds.HadSingle {
			singleTip = "支持单场购买（已开售单关）"
		} else {
			singleTip = "仅限过关（未开售单关，无法单场购买）"
		}
		advices = append(advices, PlayAdvice{"had", "胜平负", safeOpts, aggOpts, isSingleAvailable, singleTip})
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
		var safeOpts []PlayOption
		var aggOpts []PlayOption

		// 校验：如果官方已开售其他玩法，但当前让球胜平负未开售（赔率为0或负数），直接标记为不可售
		if odds.IsAvailable && oRH <= 0.0 {
			safeOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
			aggOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
		} else {
			evRH := pRHome*oRH - 1.0
			evRD := pRDraw*oRD - 1.0
			evRA := pRAway*oRA - 1.0

			opts := []PlayOption{
				{fmt.Sprintf("让胜(%d)", gLine), oRH, pRHome, evRH},
				{fmt.Sprintf("让平(%d)", gLine), oRD, pRDraw, evRD},
				{fmt.Sprintf("让负(%d)", gLine), oRA, pRAway, evRA},
			}

			optsSafe := make([]PlayOption, len(opts))
			copy(optsSafe, opts)
			safeOpts = getTop3Safe(optsSafe)

			optsAgg := make([]PlayOption, len(opts))
			copy(optsAgg, opts)
			aggOpts = getTop3Aggressive(optsAgg)
		}
		var singleTip string
		isSingleAvailable := odds.HadSingle
		if len(safeOpts) > 0 && safeOpts[0].Option == "不可售" {
			isSingleAvailable = false
			singleTip = "玩法当前未开售"
		} else if odds.HadSingle {
			singleTip = "支持单场购买（已开售单关）"
		} else {
			singleTip = "仅限过关（未开售单关，无法单场购买）"
		}
		advices = append(advices, PlayAdvice{"hhad", "让球胜平负", safeOpts, aggOpts, isSingleAvailable, singleTip})
	}
	// 3. 比分 (crs)
	{
		aggProbs := make(map[string]float64)     // key: preciseKey
		baseAggProbs := make(map[string]float64) // key: preciseKey

		for _, cell := range matrix {
			code := getPreciseCrsKey(cell.HomeScore, cell.AwayScore)
			aggProbs[code] += cell.Prob
		}

		for _, cellB := range baseMatrix {
			code := getPreciseCrsKey(cellB.HomeScore, cellB.AwayScore)
			baseAggProbs[code] += cellB.Prob
		}

		var safeOpts []PlayOption
		var aggOpts []PlayOption

		// 校验：如果官方已开售其他玩法，但当前比分未开售，直接标记为不可售
		if odds.IsAvailable && len(odds.CrsOdds) == 0 {
			safeOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
			aggOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
		} else {
			// 竞彩官方 31 个比分代码
			officialCrsCodes := []string{
				"s01s00", "s02s00", "s02s01", "s03s00", "s03s01", "s03s02",
				"s04s00", "s04s01", "s04s02", "s05s00", "s05s01", "s05s02", "s1sh",
				"s00s00", "s01s01", "s02s02", "s03s03", "s1sd",
				"s00s01", "s00s02", "s01s02", "s00s03", "s01s03", "s02s03",
				"s00s04", "s01s04", "s02s04", "s00s05", "s01s05", "s02s05", "s1sa",
			}

			var opts []PlayOption
			for _, code := range officialCrsCodes {
				prob := aggProbs[code]
				oVal := odds.CrsOdds[code]
				if oVal <= 0.0 {
					continue
				}
				ev := prob*oVal - 1.0
				key := getCrsDisplayName(code)
				opts = append(opts, PlayOption{key, oVal, prob, ev})
			}

			if len(opts) == 0 {
				safeOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
				aggOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
			} else {
				optsSafe := make([]PlayOption, len(opts))
				copy(optsSafe, opts)
				safeOpts = getTop3Safe(optsSafe)

				optsAgg := make([]PlayOption, len(opts))
				copy(optsAgg, opts)
				aggOpts = getTop3Aggressive(optsAgg)
			}
		}
		var singleTip string
		isSingleAvailable := true
		if len(safeOpts) > 0 && safeOpts[0].Option == "不可售" {
			isSingleAvailable = false
			singleTip = "玩法当前未开售"
		} else {
			singleTip = "支持单场购买（默认开售单关）"
		}
		advices = append(advices, PlayAdvice{"crs", "比分", safeOpts, aggOpts, isSingleAvailable, singleTip})
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

		var safeOpts []PlayOption
		var aggOpts []PlayOption

		// 校验：如果官方已开售其他玩法，但当前总进球数未开售，直接标记为不可售
		if odds.IsAvailable && len(odds.TtgOdds) == 0 {
			safeOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
			aggOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
		} else {
			var opts []PlayOption
			for g := 0; g <= 7; g++ {
				prob := ttgProbs[g]
				apiCode := fmt.Sprintf("s%d", g)
				oVal := odds.TtgOdds[apiCode]
				if oVal <= 0.0 {
					continue
				}
				ev := prob*oVal - 1.0
				name := fmt.Sprintf("%d球", g)
				if g == 7 {
					name = "7+球"
				}
				opts = append(opts, PlayOption{name, oVal, prob, ev})
			}

			if len(opts) == 0 {
				safeOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
				aggOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
			} else {
				optsSafe := make([]PlayOption, len(opts))
				copy(optsSafe, opts)
				safeOpts = getTop3Safe(optsSafe)

				optsAgg := make([]PlayOption, len(opts))
				copy(optsAgg, opts)
				aggOpts = getTop3Aggressive(optsAgg)
			}
		}
		var singleTip string
		isSingleAvailable := true
		if len(safeOpts) > 0 && safeOpts[0].Option == "不可售" {
			isSingleAvailable = false
			singleTip = "玩法当前未开售"
		} else {
			singleTip = "支持单场购买（默认开售单关）"
		}
		advices = append(advices, PlayAdvice{"ttg", "总进球数", safeOpts, aggOpts, isSingleAvailable, singleTip})
	}

	// 5. 半全场胜平负 (hafu)
	{
		hafuProbs := CalculateRefinedHafuProbs(lh, la, match, odds, s.dcService)

		hafuKeys := map[string]string{
			"胜胜": "hh", "胜平": "hd", "胜负": "ha",
			"平胜": "dh", "平平": "dd", "平负": "da",
			"负胜": "ah", "负平": "ad", "负负": "aa",
		}

		var safeOpts []PlayOption
		var aggOpts []PlayOption

		// 校验：如果官方已开售其他玩法，但当前半全场胜平负未开售，直接标记为不可售
		if odds.IsAvailable && len(odds.HafuOdds) == 0 {
			safeOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
			aggOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
		} else {
			var opts []PlayOption
			for op, prob := range hafuProbs {
				apiCode := hafuKeys[op]
				oVal := odds.HafuOdds[apiCode]
				if oVal <= 0.0 {
					continue
				}
				ev := prob*oVal - 1.0
				opts = append(opts, PlayOption{op, oVal, prob, ev})
			}

			if len(opts) == 0 {
				safeOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
				aggOpts = []PlayOption{{"不可售", 0.0, 0.0, 0.0}}
			} else {
				optsSafe := make([]PlayOption, len(opts))
				copy(optsSafe, opts)
				safeOpts = getTop3Safe(optsSafe)

				optsAgg := make([]PlayOption, len(opts))
				copy(optsAgg, opts)
				aggOpts = getTop3Aggressive(optsAgg)
			}
		}
		var singleTip string
		isSingleAvailable := true
		if len(safeOpts) > 0 && safeOpts[0].Option == "不可售" {
			isSingleAvailable = false
			singleTip = "玩法当前未开售"
		} else {
			singleTip = "支持单场购买（默认开售单关）"
		}
		advices = append(advices, PlayAdvice{"hafu", "半全场胜平负", safeOpts, aggOpts, isSingleAvailable, singleTip})
	}

	return advices
}

func getCrsDisplayName(code string) string {
	if code == "s1sh" {
		return "胜其它"
	}
	if code == "s1sd" {
		return "平其它"
	}
	if code == "s1sa" {
		return "负其它"
	}
	var h, a int
	if _, err := fmt.Sscanf(code, "s%02ds%02d", &h, &a); err == nil {
		return fmt.Sprintf("%d:%d", h, a)
	}
	return code
}

// CalculateRefinedHafuProbs 核心重构算法：结合巨头偏移、第三方预测和体彩隐含概率来精细测算半全场概率
func CalculateRefinedHafuProbs(lh, la float64, match models.Match, odds OfficialOdds, dcService *DixonColesService) map[string]float64 {
	hafuProbs := make(map[string]float64)
	options := []string{"胜胜", "胜平", "胜负", "平胜", "平平", "平负", "负胜", "负平", "负负"}
	for _, op := range options {
		hafuProbs[op] = 0.0
	}
	lhHalf := lh * 0.5
	laHalf := la * 0.5
	lhSecond := lh * 0.5
	laSecond := la * 0.5

	// 1. 经典独立泊松分布计算半全场初始几率
	for hHome := 0; hHome <= 4; hHome++ {
		for hAway := 0; hAway <= 4; hAway++ {
			pHalf := dcService.ComputePoissonProb(lhHalf, hHome) * dcService.ComputePoissonProb(laHalf, hAway)
			for sHome := 0; sHome <= 4; sHome++ {
				for sAway := 0; sAway <= 4; sAway++ {
					pSec := dcService.ComputePoissonProb(lhSecond, sHome) * dcService.ComputePoissonProb(laSecond, sAway)
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

	// 2. 引入第三方预测共识，并在全场维度上进行高比重校正
	extService := NewExternalPredictionService()
	extPreds := extService.GetExternalConsensus(match.HomeTeam, match.AwayTeam)
	if len(extPreds) > 0 {
		var extH, extD, extA float64
		var extCount float64
		for _, p := range extPreds {
			extH += p.HomeWin
			extD += p.Draw
			extA += p.AwayWin
			extCount++
		}
		if extCount > 0 {
			extH /= extCount
			extD /= extCount
			extA /= extCount

			// 计算本地泊松模型的全场胜平负概率
			var localH, localD, localA float64
			for op, prob := range hafuProbs {
				if strings.HasSuffix(op, "胜") {
					localH += prob
				} else if strings.HasSuffix(op, "平") {
					localD += prob
				} else {
					localA += prob
				}
			}

			// 进行加权平均融合，将第三方预测的比重设为 0.6
			mergedH := 0.4*localH + 0.6*extH
			mergedD := 0.4*localD + 0.6*extD
			mergedA := 0.4*localA + 0.6*extA

			// 用融合后的概率缩放 9 种半全场概率
			for op, prob := range hafuProbs {
				if strings.HasSuffix(op, "胜") && localH > 0 {
					hafuProbs[op] = prob * (mergedH / localH)
				} else if strings.HasSuffix(op, "平") && localD > 0 {
					hafuProbs[op] = prob * (mergedD / localD)
				} else if strings.HasSuffix(op, "负") && localA > 0 {
					hafuProbs[op] = prob * (mergedA / localA)
				}
			}
		}
	}

	// 3. 引入博彩巨头赔率偏移数据，在全场维度上对半全场进行大比重修正
	tracker := &OddsTrackerService{}
	shifts := tracker.GetOddsShifts(match.HomeTeam, match.AwayTeam)
	var shiftH, shiftD, shiftA float64
	for _, s := range shifts {
		if s.Bookmaker == "Bet365" && s.Outcome == "主胜" {
			shiftH = s.ShiftPct
		} else if s.Bookmaker == "Pinnacle (平博)" && s.Outcome == "平局" {
			shiftD = s.ShiftPct
		} else if s.Bookmaker == "William Hill" && s.Outcome == "客胜" {
			shiftA = s.ShiftPct
		}
	}

	for op, prob := range hafuProbs {
		var shift float64
		if strings.HasSuffix(op, "胜") {
			shift = shiftH
		} else if strings.HasSuffix(op, "平") {
			shift = shiftD
		} else {
			shift = shiftA
		}
		// 大比重（0.02）修正：降水概率上升，升水概率下降
		hafuProbs[op] = prob * (1.0 - 0.02*shift)
	}

	// 归一化
	sum := 0.0
	for _, prob := range hafuProbs {
		sum += prob
	}
	if sum > 0 {
		for op := range hafuProbs {
			hafuProbs[op] /= sum
		}
	}

	// 4. 融合官方已售半全场赔率的隐含概率 (若可用)
	if odds.IsAvailable && len(odds.HafuOdds) > 0 {
		hafuKeys := map[string]string{
			"胜胜": "hh", "胜平": "hd", "胜负": "ha",
			"平胜": "dh", "平平": "dd", "平负": "da",
			"负胜": "ah", "负平": "ad", "负负": "aa",
		}

		var invSum float64
		for _, code := range hafuKeys {
			if oVal, ok := odds.HafuOdds[code]; ok && oVal > 0 {
				invSum += 1.0 / oVal
			}
		}

		if invSum > 0 {
			oddsProbs := make(map[string]float64)
			for op, code := range hafuKeys {
				if oVal, ok := odds.HafuOdds[code]; ok && oVal > 0 {
					oddsProbs[op] = (1.0 / oVal) / invSum
				} else {
					oddsProbs[op] = 0.0
				}
			}

			// 对官方赔率反推得到的隐含概率，也注入赔率偏移修正
			for op, prob := range oddsProbs {
				var shift float64
				if strings.HasSuffix(op, "胜") {
					shift = shiftH
				} else if strings.HasSuffix(op, "平") {
					shift = shiftD
				} else {
					shift = shiftA
				}
				oddsProbs[op] = prob * (1.0 - 0.02*shift)
			}

			sumOdds := 0.0
			for _, prob := range oddsProbs {
				sumOdds += prob
			}
			if sumOdds > 0 {
				for op := range oddsProbs {
					oddsProbs[op] /= sumOdds
				}
			}

			// 将博彩模型概率与官方赔率隐含概率以 0.5 : 0.5 权重进行融合
			for op := range hafuProbs {
				hafuProbs[op] = 0.5*hafuProbs[op] + 0.5*oddsProbs[op]
			}

			// 最终归一化
			sumFinal := 0.0
			for _, prob := range hafuProbs {
				sumFinal += prob
			}
			if sumFinal > 0 {
				for op := range hafuProbs {
					hafuProbs[op] /= sumFinal
				}
			}
		}
	}

	return hafuProbs
}

func getTop3Safe(options []PlayOption) []PlayOption {
	sort.Slice(options, func(i, j int) bool {
		return options[i].Prob > options[j].Prob
	})
	if len(options) > 3 {
		return options[:3]
	}
	return options
}

func getTop3Aggressive(options []PlayOption) []PlayOption {
	sort.Slice(options, func(i, j int) bool {
		return options[i].EV > options[j].EV
	})
	if len(options) > 3 {
		return options[:3]
	}
	return options
}
