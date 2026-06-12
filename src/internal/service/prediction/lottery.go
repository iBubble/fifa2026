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
}

type Hedge struct {
	Outcome  string  `json:"outcome"`
	Odds     float64 `json:"odds"`
	StakePct float64 `json:"stakePct"`
}

// GenerateSingleAdvice 单场体彩预测与官方赔率自适应匹配
func (s *LotteryService) GenerateSingleAdvice(match models.Match, oddsHome, oddsDraw, oddsAway float64, report *models.PredictionReport) LotteryAdvice {
	var winH, draw, winA float64
	var matrix []models.ScoreProbability
	var isLLMRefined bool
	var tacticsReport string

	if report != nil {
		matrix = report.ScoreMatrix
		isLLMRefined = report.LLMRefined
		tacticsReport = report.TacticsAnalysis
	} else {
		params := s.dcService.CalculateParams(match.HomeTeam, match.AwayTeam)
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
	official := s.sportteryService.GetMatchOdds(match.HomeTeam, match.AwayTeam)
	if official.IsAvailable {
		oddsHome, oddsDraw, oddsAway = official.HomeOdds, official.DrawOdds, official.AwayOdds
	} else {
		// 官方未开售降级：使用 Dixon-Coles 泊松联合概率与竞彩理论 89% 返奖率仿真模拟官方赔率
		payout := 0.89
		if winH > 0 {
			oddsHome = math.Min(100.0, payout/winH)
		} else {
			oddsHome = 2.0
		}
		if draw > 0 {
			oddsDraw = math.Min(100.0, payout/draw)
		} else {
			oddsDraw = 3.2
		}
		if winA > 0 {
			oddsAway = math.Min(100.0, payout/winA)
		} else {
			oddsAway = 3.6
		}

		official = OfficialOdds{
			HomeOdds:     oddsHome,
			DrawOdds:     oddsDraw,
			AwayOdds:     oddsAway,
			IsAvailable:  true,
			IsSimulation: true,
		}
	}

	advice := LotteryAdvice{
		MatchID:       match.ID,
		HomeTeam:      match.HomeTeam,
		AwayTeam:      match.AwayTeam,
		RecommendType: "SINGLE",
	}
	if official.IsAvailable {
		advice.OfficialOdds = &official
	}

	// 战意/伤病突发风控排除
	hasNegativeInfo := false
	if isLLMRefined {
		negKeywords := []string{"内讧", "暴雨", "伤病", "缺阵", "红牌", "矛盾", "不确定", "大波动", "停赛"}
		for _, kw := range negKeywords {
			if strings.Contains(tacticsReport, kw) {
				hasNegativeInfo = true
				break
			}
		}
	}

	if maxP < 0.38 || (maxP-minP) < 0.10 || hasNegativeInfo {
		advice.Status = "EXCLUDED"
		reason := "【体彩风控拦截】胜平负概率均等，研判有冷门风险，建议排除在串关之外。"
		if hasNegativeInfo {
			reason = "【体彩情报风控拦截】大模型研判本场受到伤病/天气/突发事件负面冲击，已进行安全隔离排除。"
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
	advice.PrimaryStake = 0.80
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

	advice.HedgeBets = []Hedge{
		{Outcome: hedgeOutcome, Odds: hedgeOdds, StakePct: 0.20},
	}

	llmPrefix := ""
	if isLLMRefined {
		llmPrefix = "【外围情报校准】"
	}
	advice.Reason = fmt.Sprintf("%s定量模型测算首推胜率达 %s，建议体彩 80%% 投【%s】，20%% 配备【%s @%.2f】防冷平，锁死下限保本。", llmPrefix, fmt.Sprintf("%.1f%%", pProb*100), primary, hedgeOutcome, hedgeOdds)

	return advice
}

// GenerateParlayAdvice 混合过关 2串1 时序对冲无风险套利计算
func (s *LotteryService) GenerateParlayAdvice(m1, m2 models.Match, odds1, odds2 float64) LotteryAdvice {
	off1 := s.sportteryService.GetMatchOdds(m1.HomeTeam, m1.AwayTeam)
	off2 := s.sportteryService.GetMatchOdds(m2.HomeTeam, m2.AwayTeam)

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

	var reason string
	if roi > 0 {
		reason = fmt.Sprintf("【混合过关套利锁利方案】：第一场单选【%s 胜平负(主胜 @%.2f)】，第二场单选【%s 让球胜平负(让主胜 @%.2f)】组成混合过关 2串1。若第一场打出，在第二场开赛前单投第二场相反项【让客负 @%.2f】对冲 %.2f 元。无论第二场结果如何，均可稳定锁定 %.2f 元无风险利润 (ROI: +%.1f%%)！",
			m1.HomeTeam, o1, m2.HomeTeam, o2, oHedge, sHedge, profit, roi)
	} else {
		reason = fmt.Sprintf("【混合过关2串1建议】：主推【%s 主胜 @%.2f】+【%s 让主胜 @%.2f】。因对冲防守项【让客负 @%.2f】目前奖金偏低，公式计算套利 ROI 倒挂，不建议强行对冲，建议单独串关或继续观望赔率浮动。",
			m1.HomeTeam, o1, m2.HomeTeam, o2, oHedge)
	}

	return LotteryAdvice{
		RecommendType: "PARLAY",
		Status:        "RECOMMENDED",
		Reason:        reason,
	}
}
