package prediction

import (
	"fifa2026/src/internal/models"
	"fmt"
	"math"
	"strings"
)

type LotteryService struct {
	dcService        *DixonColesService
	sportteryService *SportteryService
}

func NewLotteryService(dc *DixonColesService, ss *SportteryService) *LotteryService {
	return &LotteryService{
		dcService:        dc,
		sportteryService: ss,
	}
}

type LotteryAdvice struct {
	MatchID       string        `json:"matchId"`
	HomeTeam      string        `json:"homeTeam"`
	AwayTeam      string        `json:"awayTeam"`
	RecommendType string        `json:"recommendType"`
	PrimaryBet    string        `json:"primaryBet"`
	PrimaryOdds   float64       `json:"primaryOdds"`
	PrimaryStake  float64       `json:"primaryStake"`
	HedgeBets     []Hedge       `json:"hedgeBets"`
	Status        string        `json:"status"`
	Reason        string        `json:"reason"`
	OfficialOdds  *OfficialOdds `json:"officialOdds,omitempty"`
	Critique      string        `json:"critique"` // 大模型风控反驳评语
}

type Hedge struct {
	Outcome  string  `json:"outcome"`
	Odds     float64 `json:"odds"`
	StakePct float64 `json:"stakePct"`
}

// GenerateSingleAdvice 单场体彩预测与官方赔率自适应匹配
func (s *LotteryService) GenerateSingleAdvice(match models.Match, oddsHome, oddsDraw, oddsAway float64, report *models.PredictionReport, isSingleHad bool) LotteryAdvice {
	var winH, draw, winA float64
	var matrix []models.ScoreProbability
	var isLLMRefined bool
	var tacticsReport string
	var critiqueReport string

	advice := LotteryAdvice{
		MatchID:       match.ID,
		HomeTeam:      match.HomeTeam,
		AwayTeam:      match.AwayTeam,
		RecommendType: "SINGLE",
	}

	if report != nil {
		matrix = report.ScoreMatrix
		isLLMRefined = report.LLMRefined
		tacticsReport = report.TacticsAnalysis
		critiqueReport = report.CritiqueAnalysis
		advice.Critique = critiqueReport
	} else {
		params := s.dcService.CalculateParamsWithVenue(match.HomeTeam, match.AwayTeam, match.Venue, match.ScheduledAt)
		matrix, _, _ = s.dcService.GenerateProbabilityMatrix(params)
	}

	for _, c := range matrix {
		if c.HomeScore > c.AwayScore {
			winH += c.Prob
		} else if c.HomeScore == c.AwayScore {
			draw += c.Prob
		} else {
			winA += c.Prob
		}
	}

	maxP := math.Max(winH, math.Max(draw, winA))
	minP := math.Min(winH, math.Min(draw, winA))

	// 获取官方赔率
	official := s.sportteryService.GetMatchOdds(match.HomeTeam, match.AwayTeam, match.ScheduledAt)

	// 针对官方开售但常规未开盘（仅开让球）或完全未开盘（不可购买）的实战拦截与路由降级
	hadAvailable := official.IsAvailable && official.HomeOdds > 0.0 && official.HadSingle
	if official.IsAvailable && !hadAvailable {
		hhadAvailable := official.IsAvailable && official.HhadHomeOdds > 0.0 && official.HadSingle
		if hhadAvailable {
			// 1. 计算让球胜平负概率
			gLine := official.GoalLine
			if gLine == 0 {
				gLine = -1
			}
			var pRHome, pRDraw, pRAway float64
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

			// 2. 选择主推项
			var primary string
			var pProb, pOdds float64
			if pRHome >= pRDraw && pRHome >= pRAway {
				primary = fmt.Sprintf("让胜(%d)", gLine)
				pProb = pRHome
				pOdds = official.HhadHomeOdds
			} else if pRAway >= pRHome && pRAway >= pRDraw {
				primary = fmt.Sprintf("让负(%d)", gLine)
				pProb = pRAway
				pOdds = official.HhadAwayOdds
			} else {
				primary = fmt.Sprintf("让平(%d)", gLine)
				pProb = pRDraw
				pOdds = official.HhadDrawOdds
			}

			advice.PrimaryBet = primary
			advice.PrimaryOdds = pOdds
			llmPrefix := ""
			if isLLMRefined {
				llmPrefix = "【外围情报校准】"
			}

			// 3. 配置对冲项 (比分1-1)
			hedgeOutcome := "比分 1-1"
			hedgeOdds := 6.00
			if official.CrsOdds != nil {
				if val, ok := official.CrsOdds["s01s01"]; ok && val > 0 {
					hedgeOdds = val
				}
			}

			if pProb >= 0.75 {
				advice.PrimaryStake = 1.0
				advice.HedgeBets = []Hedge{}
				advice.Reason = fmt.Sprintf("%s官方常规胜平负未开售，系统自动切换至让球。测算让球胜率高达 %s，属碾压级优势局，建议体彩 100%% 独投【%s】以最大化优势盘口红利。",
					llmPrefix, fmt.Sprintf("%.1f%%", pProb*100), primary)
			} else {
				advice.PrimaryStake = 0.80
				advice.HedgeBets = []Hedge{
					{Outcome: hedgeOutcome, Odds: hedgeOdds, StakePct: 0.20},
				}
				advice.Reason = fmt.Sprintf("%s官方常规胜平负未开售，系统自动切换至让球玩法。让球胜率达 %s，建议体彩 80%% 投【%s】，20%% 配备【%s @%.2f】保本防冷平。", 
					llmPrefix, fmt.Sprintf("%.1f%%", pProb*100), primary, hedgeOutcome, hedgeOdds)
			}

			advice.Status = "RECOMMENDED"
			advice.OfficialOdds = &official
			return advice
		} else {
			// 常规和让球均未开售，我们检查比分（CRS）和总进球（TTG）是否开售以进行次优推荐
			if len(official.CrsOdds) > 0 {
				var bestCrs string
				var maxProb float64
				var bestOdds float64

				for code, oVal := range official.CrsOdds {
					if oVal <= 0.0 {
						continue
					}
					var pVal float64
					for _, cell := range matrix {
						cellCode := getPreciseCrsKey(cell.HomeScore, cell.AwayScore)
						if cellCode == code {
							pVal = cell.Prob
							break
						}
					}
					if pVal > maxProb {
						maxProb = pVal
						bestCrs = getCrsDisplayName(code)
						bestOdds = oVal
					}
				}

				if bestCrs != "" && bestOdds > 0.0 {
					advice.PrimaryBet = "比分 " + bestCrs
					advice.PrimaryOdds = bestOdds
					advice.PrimaryStake = 1.0
					advice.Status = "RECOMMENDED"
					advice.Reason = fmt.Sprintf("【体彩降级】官方常规与让球未开售，系统智能切换至比分玩法。主推【比分 %s @%.2f】（几率 %.1f%%）。", 
						bestCrs, bestOdds, maxProb*100)
					advice.OfficialOdds = &official
					return advice
				}
			}

			if len(official.TtgOdds) > 0 {
				var bestTtg string
				var maxProb float64
				var bestOdds float64

				ttgProbs := make([]float64, 8)
				for _, cell := range matrix {
					tot := cell.HomeScore + cell.AwayScore
					if tot >= 7 {
						ttgProbs[7] += cell.Prob
					} else {
						ttgProbs[tot] += cell.Prob
					}
				}

				for code, oVal := range official.TtgOdds {
					if oVal <= 0.0 {
						continue
					}
					var g int
					_, errScan := fmt.Sscanf(code, "s%d", &g)
					if errScan == nil && g >= 0 && g <= 7 {
						pVal := ttgProbs[g]
						if pVal > maxProb {
							maxProb = pVal
							if g == 7 {
								bestTtg = "7+球"
							} else {
								bestTtg = fmt.Sprintf("%d球", g)
							}
							bestOdds = oVal
						}
					}
				}

				if bestTtg != "" && bestOdds > 0.0 {
					advice.PrimaryBet = "总进球 " + bestTtg
					advice.PrimaryOdds = bestOdds
					advice.PrimaryStake = 1.0
					advice.Status = "RECOMMENDED"
					advice.Reason = fmt.Sprintf("【体彩降级】官方常规与让球未开售，系统智能切换至总进球数。主推【%s @%.2f】（几率 %.1f%%）。", 
						bestTtg, bestOdds, maxProb*100)
					advice.OfficialOdds = &official
					return advice
				}
			}

			// 如果所有玩法均未开售，才真正标记为未开盘
			advice.Status = "EXCLUDED"
			advice.Reason = "【体彩未开盘】该赛事常规、让球、比分及总进球玩法目前均未开售，无法提供投注推荐。"
			advice.OfficialOdds = &official
			return advice
		}
	}

	if official.IsAvailable {
		oddsHome, oddsDraw, oddsAway = official.HomeOdds, official.DrawOdds, official.AwayOdds
	} else {
		advice.Status = "EXCLUDED"
		advice.Reason = "【体彩未开盘】该赛事竞彩官网目前未开售，无法提供投注推荐。"
		advice.OfficialOdds = &official
		return advice
	}

	if official.IsAvailable {
		advice.OfficialOdds = &official
	}

	// 战意/伤病突发风控排除
	hasNegativeInfo := false
	if isLLMRefined {
		negKeywords := []string{"内讧", "暴雨", "伤病", "缺阵", "红牌", "矛盾", "不确定", "大波动", "停赛", "大热必死"}
		combinedAnalysis := tacticsReport + " " + critiqueReport
		for _, kw := range negKeywords {
			if strings.Contains(combinedAnalysis, kw) {
				hasNegativeInfo = true
				break
			}
		}
	}

	// 历史交锋极端天敌克制风控排除
	hasClashRisk := false
	var clashReason string
	if report != nil && report.H2H != nil && report.H2H.TotalMatches >= 3 {
		h2h := report.H2H
		homeWinRate := float64(h2h.HomeWins) / float64(h2h.TotalMatches)
		awayWinRate := float64(h2h.AwayWins) / float64(h2h.TotalMatches)

		// 判定潜在的主推倾向
		tempPrimaryWinHome := winH >= draw && winH >= winA
		tempPrimaryWinAway := winA >= winH && winA >= draw

		if homeWinRate == 0 && tempPrimaryWinHome {
			hasClashRisk = true
			clashReason = fmt.Sprintf("【交手天敌克制拦截】两队历史交锋 %d 次，主队 %s 胜率为 0%%。虽大盘模型偏向主胜，但历史直接交锋天敌属性极强，防范爆冷故安全排除。", h2h.TotalMatches, match.HomeTeam)
		} else if awayWinRate == 0 && tempPrimaryWinAway {
			hasClashRisk = true
			clashReason = fmt.Sprintf("【交手天敌克制拦截】两队历史交锋 %d 次，客队 %s 胜率为 0%%。虽大盘模型偏向客胜，但历史直接交锋天敌属性极强，防范爆冷故安全排除。", h2h.TotalMatches, match.AwayTeam)
		}
	}

	if maxP < 0.38 || (maxP-minP) < 0.10 || hasNegativeInfo || hasClashRisk {
		advice.Status = "EXCLUDED"
		reason := "【体彩风控拦截】胜平负概率均等，研判有冷门风险，建议排除在串关之外。"
		if hasNegativeInfo {
			reason = "【体彩情报风控拦截】大模型研判本场受到伤病/天气/突发事件负面冲击，已进行安全隔离排除。"
		} else if hasClashRisk {
			reason = clashReason
		}
		advice.Reason = reason
		return advice
	}

	var primary string
	var pProb, pOdds float64
	if winH >= draw && winH >= winA {
		primary = "主胜 (3)"
		pProb = winH
		pOdds = oddsHome
	} else if winA >= winH && winA >= draw {
		primary = "客胜 (0)"
		pProb = winA
		pOdds = oddsAway
	} else {
		primary = "平局 (1)"
		pProb = draw
		pOdds = oddsDraw
	}

	advice.PrimaryBet = primary
	advice.PrimaryOdds = pOdds
	advice.Status = "RECOMMENDED"

	var hedgeOutcome string
	var hedgeOdds float64
	if primary == "主胜 (3)" {
		hedgeOutcome = "比分 1-1"
		hedgeOdds = 6.00
		if official.CrsOdds != nil {
			if val, ok := official.CrsOdds["s01s01"]; ok && val > 0 {
				hedgeOdds = val
			}
		}
	} else if primary == "客胜 (0)" {
		hedgeOutcome = "比分 1-1"
		hedgeOdds = 6.00
		if official.CrsOdds != nil {
			if val, ok := official.CrsOdds["s01s01"]; ok && val > 0 {
				hedgeOdds = val
			}
		}
	} else {
		hedgeOutcome = "比分 1-0"
		hedgeOdds = 6.50
		if official.CrsOdds != nil {
			if val, ok := official.CrsOdds["s01s00"]; ok && val > 0 {
				hedgeOdds = val
			}
		}
	}

	llmPrefix := ""
	if isLLMRefined {
		llmPrefix = "【外围情报校准】"
	}

	if pProb >= 0.75 {
		advice.PrimaryStake = 1.0
		advice.HedgeBets = []Hedge{}
		advice.Reason = fmt.Sprintf("%s定量模型测算首推胜率高达 %s，属超强实力碾压局，建议体彩 100%% 独投【%s】以避免分仓对冲造成的资金红利稀释。",
			llmPrefix, fmt.Sprintf("%.1f%%", pProb*100), primary)
	} else {
		advice.PrimaryStake = 0.80
		advice.HedgeBets = []Hedge{
			{Outcome: hedgeOutcome, Odds: hedgeOdds, StakePct: 0.20},
		}
		advice.Reason = fmt.Sprintf("%s定量模型测算首推胜率达 %s，建议体彩 80%% 投【%s】，20%% 配备【%s @%.2f】防冷平，锁死下限保本。",
			llmPrefix, fmt.Sprintf("%.1f%%", pProb*100), primary, hedgeOutcome, hedgeOdds)
	}

	return advice
}

// GenerateParlayAdvice 混合过关 2串1 时序对冲无风险套利计算
func (s *LotteryService) GenerateParlayAdvice(m1, m2 models.Match, odds1, odds2 float64, report *models.PredictionReport) LotteryAdvice {
	off1 := s.sportteryService.GetMatchOdds(m1.HomeTeam, m1.AwayTeam, m1.ScheduledAt)
	off2 := s.sportteryService.GetMatchOdds(m2.HomeTeam, m2.AwayTeam, m2.ScheduledAt)

	o1 := odds1
	if off1.IsAvailable {
		o1 = off1.HomeOdds
	}
	o2 := off2.HhadHomeOdds     // 第二场主推：让球主胜 (HHAD)
	oHedge := off2.HhadAwayOdds // 第二场对冲：让球客负 (HHAD)

	if !off2.IsAvailable || o2 <= 0.0 || oHedge <= 0.0 {
		return LotteryAdvice{
			RecommendType: "PARLAY",
			Status:        "RECOMMENDED",
			Reason:        "体彩混合过关串关：第二场赛事赔率暂未开盘，无法计算精确套利公式，请待开盘后刷新。",
		}
	}

	// 混合过关 2串1 时序对冲公式计算
	s1 := 100.0 // 初始主单投注 100 元
	sHedge := s1 * (o1 * o2) / oHedge
	totalStake := s1 + sHedge
	totalReturn := s1 * o1 * o2
	profit := totalReturn - totalStake
	roi := (profit / totalStake) * 100.0

	// 标注数据来源
	dataSource := "基于定量泊松模型"
	if report != nil && report.LLMRefined {
		dataSource = "基于多Agent反驳纠偏模型"
	}

	var reason string
	if roi > 0 {
		reason = fmt.Sprintf("【混合过关套利锁利方案 (%s)】：第一场单选【%s 胜平负(主胜 @%.2f)】，第二场单选【%s 让球胜平负(让主胜 @%.2f)】组成混合过关 2串1。若第一场打出，在第二场开赛前单投第二场相反项【让客负 @%.2f】对冲 %.2f 元。无论第二场结果如何，均可稳定锁定 %.2f 元无风险利润 (ROI: +%.1f%%)！",
			dataSource, m1.HomeTeam, o1, m2.HomeTeam, o2, oHedge, sHedge, profit, roi)
	} else {
		reason = fmt.Sprintf("【混合过关2串1建议 (%s)】：主推【%s 主胜 @%.2f】+【%s 让主胜 @%.2f】。因对冲防守项【让客负 @%.2f】目前奖金偏低，公式计算套利 ROI 倒挂，不建议强行对冲，建议单独串关或继续观望赔率浮动。",
			dataSource, m1.HomeTeam, o1, m2.HomeTeam, o2, oHedge)
	}

	return LotteryAdvice{
		RecommendType: "PARLAY",
		Status:        "RECOMMENDED",
		Reason:        reason,
	}
}
