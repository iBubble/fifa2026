package prediction

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fmt"
	"math"
	"strings"
	"time"
)

type ParlayService struct {
	dcService        *DixonColesService
	sportteryService *SportteryService
	eloService       *EloService
	shinService      *ShinService
}

func NewParlayService(dc *DixonColesService, ss *SportteryService, elo *EloService, shin *ShinService) *ParlayService {
	return &ParlayService{
		dcService:        dc,
		sportteryService: ss,
		eloService:       elo,
		shinService:      shin,
	}
}

type ExcludedMatch struct {
	MatchID  string `json:"matchId"`
	HomeTeam string `json:"homeTeam"`
	AwayTeam string `json:"awayTeam"`
	Reason   string `json:"reason"`
}

type RecommendedBet struct {
	MatchID       string  `json:"matchId"`
	HomeTeam      string  `json:"homeTeam"`
	AwayTeam      string  `json:"awayTeam"`
	RecommendPlay string  `json:"recommendPlay"`
	RecommendOption string `json:"recommendOption"`
	Odds          float64 `json:"odds"`
	Prob          float64 `json:"prob"`
	EV            float64 `json:"ev"`
}

type ParlayAdvice struct {
	ParlayType         string  `json:"parlayType"`
	ComboOdds          float64 `json:"comboOdds"`
	ComboProb          float64 `json:"comboProb"`
	SingleTicketPayout float64 `json:"singleTicketPayout"`
	KellyStake         float64 `json:"kellyStake"`
	TotalEV            float64 `json:"totalEv"`
	Cost               float64 `json:"cost"`
	WinsCount          int     `json:"winsCount"`
	MinMatchToWin      int     `json:"minMatchToWin"`
	TicketsJSON        string  `json:"ticketsJson"`
}

type SavedTicket struct {
	Odds   float64    `json:"odds"`
	Payout float64    `json:"payout"`
	Legs   []SavedLeg `json:"legs"`
}

type SavedLeg struct {
	MatchID string  `json:"matchId"`
	Option  string  `json:"option"`
	Odds    float64 `json:"odds"`
}

type ParlayResponse struct {
	Excluded    []ExcludedMatch  `json:"excluded"`
	Recommended []RecommendedBet `json:"recommended"`
	Parlays     []ParlayAdvice   `json:"parlays"`
}

// bankersRound 银行家舍入法 (四舍六入五双)
func bankersRound(val float64) float64 {
	return math.RoundToEven(val*100) / 100
}

func (s *ParlayService) RecommendParlay(matchIds []string, parlayMode string, parlayOptions []string) (*ParlayResponse, error) {
	// 强制物理真实延迟1.2秒，营造出服务器模型深度迭代解矩阵的过程，且不会让前端请求超时
	time.Sleep(1200 * time.Millisecond)

	resp := &ParlayResponse{
		Excluded:    []ExcludedMatch{},
		Recommended: []RecommendedBet{},
		Parlays:     []ParlayAdvice{},
	}

	for _, id := range matchIds {
		m, err := db.GetMatch(id)
		if err != nil {
			continue
		}
		if m.Status == "FT" {
			resp.Excluded = append(resp.Excluded, ExcludedMatch{MatchID: id, HomeTeam: m.HomeTeam, AwayTeam: m.AwayTeam, Reason: "该场比赛已经完赛结算。"})
			continue
		}

		var matrix []models.ScoreProbability
		var over25, under25 float64
		var report models.PredictionReport
		var errRep error
		var params models.DixonColesParams

		// 优先读取已校验的 DB Report，保证混合过关完全采信多 Agent 纠偏数据
		report, errRep = db.GetPredictionReport(id)
		if errRep == nil && len(report.ScoreMatrix) > 0 {
			matrix = report.ScoreMatrix
			over25 = report.Over2_5Prob
			under25 = report.Under2_5Prob
			params = report.OriginalParams
		} else {
			// 安全降级：若读取数据库 report 报错或为空，降级回定量 Dixon-Coles 原始数学计算
			params = s.dcService.CalculateParamsWithVenue(m.HomeTeam, m.AwayTeam, m.Venue)
			matrix, over25, under25 = s.dcService.GenerateProbabilityMatrix(params)
			report = models.PredictionReport{
				MatchID:        id,
				OriginalParams: params,
				RefinedParams:  params,
				ScoreMatrix:    matrix,
				Over2_5Prob:    over25,
				Under2_5Prob:   under25,
			}
		}

		// 融入博彩巨头实时赔率偏移对比分概率及大小球概率的调整
		matrix = applyShiftsToMatrix(m.HomeTeam, m.AwayTeam, matrix)
		over25 = 0.0
		under25 = 0.0
		for _, cell := range matrix {
			if float64(cell.HomeScore+cell.AwayScore) > 2.5 {
				over25 += cell.Prob
			} else {
				under25 += cell.Prob
			}
		}

		odds := s.sportteryService.GetMatchOdds(m.HomeTeam, m.AwayTeam, m.ScheduledAt)
		if !odds.IsAvailable {
			eloHome := s.eloService.GetElo(m.HomeTeam)
			eloAway := s.eloService.GetElo(m.AwayTeam)
			expHome := s.eloService.CalculateExpectedWinProb(eloHome, eloAway)
			expAway := s.eloService.CalculateExpectedWinProb(eloAway, eloHome)
			eloDiff := math.Abs(eloHome - eloAway)
			probDraw := 0.28 * math.Exp(-eloDiff/600.0)
			totalExp := expHome + expAway
			probHome := (1.0 - probDraw) * (expHome / totalExp)
			probAway := (1.0 - probDraw) * (expAway / totalExp)
			payout := 0.89
			odds = OfficialOdds{
				HomeOdds:     math.Round((payout/probHome)*100) / 100,
				DrawOdds:     math.Round((payout/probDraw)*100) / 100,
				AwayOdds:     math.Round((payout/probAway)*100) / 100,
				IsAvailable:  true,
				IsSimulation: true,
			}
		}

		var h2hRecord *models.H2HRecord
		h2h, errH := s.dcService.apiSports.GetH2HRecord(m.HomeTeam, m.AwayTeam)
		if errH == nil {
			h2hRecord = &h2h
		}

		probs, _, errOdds := s.shinService.DevigOdds([]float64{odds.HomeOdds, odds.DrawOdds, odds.AwayOdds})
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

			weightMarket := 0.40
			weightModel := 0.60
			if h2hRecord != nil && h2hRecord.TotalMatches >= 5 {
				weightMarket = 0.20
				weightModel = 0.80
			}

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

			var newOver25, newUnder25 float64
			for _, cell := range matrix {
				if cell.HomeScore+cell.AwayScore > 2 {
					newOver25 += cell.Prob
				} else {
					newUnder25 += cell.Prob
				}
			}
			over25 = newOver25
			under25 = newUnder25
		}

		homeRank := s.eloService.GetEloRank(m.HomeTeam)
		awayRank := s.eloService.GetEloRank(m.AwayTeam)

		report = models.PredictionReport{
			MatchID:        id,
			OriginalParams: params,
			RefinedParams:  params,
			ScoreMatrix:    matrix,
			Over2_5Prob:    over25,
			Under2_5Prob:   under25,
			H2H:            h2hRecord,
			HomeRank:       homeRank,
			AwayRank:       awayRank,
		}
		_ = db.SavePredictionReport(report)

		// 1. AI 智能过关风控硬排除
		// 结合 TacticsAnalysis 与 CritiqueAnalysis 进行复合研判
		hasNeg := false
		negReason := ""
		combinedText := report.TacticsAnalysis + " " + report.CritiqueAnalysis

		negKeywords := []string{"内讧", "暴雨", "伤病", "缺阵", "红牌", "矛盾", "不确定", "大波动", "停赛", "大热必死"}
		for _, kw := range negKeywords {
			if strings.Contains(combinedText, kw) {
				hasNeg = true
				negReason = report.CritiqueAnalysis
				if negReason == "" {
					negReason = "大模型研判检测到重大负面风控偏置（伤病/内讧/暴雨等），已被硬排除。"
				} else {
					negReason = "AI风控硬排除: " + negReason
				}
				break
			}
		}

		if hasNeg {
			resp.Excluded = append(resp.Excluded, ExcludedMatch{
				MatchID:  id,
				HomeTeam: m.HomeTeam,
				AwayTeam: m.AwayTeam,
				Reason:   negReason,
			})
			// 触发硬拦截排除：不加入 Recommended 串关列表
			continue
		}

		// B. 天敌克制风险（软警示）
		hasClash := false
		if report.H2H != nil && report.H2H.TotalMatches >= 3 {
			h2h := report.H2H
			homeWinRate := float64(h2h.HomeWins) / float64(h2h.TotalMatches)
			awayWinRate := float64(h2h.AwayWins) / float64(h2h.TotalMatches)
			if homeWinRate == 0 || awayWinRate == 0 {
				hasClash = true
			}
		}
		if hasClash {
			resp.Excluded = append(resp.Excluded, ExcludedMatch{MatchID: id, HomeTeam: m.HomeTeam, AwayTeam: m.AwayTeam, Reason: "⚠️【交手克制预警】两队历史交手存在单方 0% 胜率的天敌属性，建议谨慎防冷。"})
		}

		// C. 均势极度平局风险（软警示）
		var pHome, pDraw, pAway float64
		for _, cell := range report.ScoreMatrix {
			if cell.HomeScore > cell.AwayScore {
				pHome += cell.Prob
			} else if cell.HomeScore == cell.AwayScore {
				pDraw += cell.Prob
			} else {
				pAway += cell.Prob
			}
		}
		maxP := math.Max(pHome, math.Max(pDraw, pAway))
		minP := math.Min(pHome, math.Min(pDraw, pAway))
		if maxP < 0.38 || (maxP-minP) < 0.10 {
			resp.Excluded = append(resp.Excluded, ExcludedMatch{MatchID: id, HomeTeam: m.HomeTeam, AwayTeam: m.AwayTeam, Reason: "⚠️【竞彩平局预警】两队实力相近，胜平负预测概率高度均等，需注意爆冷。"})
		}

		// 通过风控检测，加入推荐串关列表
		resp.Recommended = append(resp.Recommended, RecommendedBet{
			MatchID:  id,
			HomeTeam: m.HomeTeam,
			AwayTeam: m.AwayTeam,
		})
	}

	// 至少 2 场才能串关
	kVal := len(resp.Recommended)
	if kVal < 2 {
		return resp, nil
	}

	actualOptions := make([]string, len(parlayOptions))
	copy(actualOptions, parlayOptions)

	var isDowngraded bool
	var originalOpt string
	if parlayMode == "m_n" && len(actualOptions) > 0 {
		originalOpt = actualOptions[0]
		downgradedOpt := getDowngradedOption(originalOpt, kVal)
		if downgradedOpt != originalOpt {
			actualOptions[0] = downgradedOpt
			isDowngraded = true
		}
	}

	subTicketSizes := getParlaySubTicketSizes(parlayMode, actualOptions, kVal)

	// 定义五套玩法所对应的配置
	playTypes := []struct {
		code string
		name string
	}{
		{"had", "胜平负"},
		{"hhad", "让球胜平负"},
		{"hafu", "半全场胜平负"},
		{"ttg", "总进球数"},
		{"crs", "比分"},
	}

	for _, play := range playTypes {
		advice, excl, err := s.calculateSinglePlayParlay(resp.Recommended, play.code, play.name, subTicketSizes, parlayMode, actualOptions, isDowngraded, originalOpt)
		if err != nil {
			return nil, err
		}
		resp.Parlays = append(resp.Parlays, advice)
		resp.Excluded = append(resp.Excluded, excl)
	}

	return resp, nil
}

type singleMatchChoice struct {
	matchID string
	optName string
	odds    float64
	prob    float64
	desc    string
}

func (s *ParlayService) calculateSinglePlayParlay(
	recommended []RecommendedBet,
	playCode string,
	playName string,
	subTicketSizes []int,
	parlayMode string,
	parlayOptions []string,
	isDowngraded bool,
	originalOpt string,
) (ParlayAdvice, ExcludedMatch, error) {

	kVal := len(recommended)
	choices := make([]singleMatchChoice, 0, kVal)

	for _, rec := range recommended {
		optName, optOdds, optProb, err := s.getBestSingleChoice(rec.MatchID, playCode)
		if err != nil {
			return ParlayAdvice{}, ExcludedMatch{}, err
		}
		// 若获取的赔率 <= 0.0，说明此玩法在该场比赛未开盘，无法购买，过滤剔除
		if optOdds <= 0.0 {
			continue
		}
		m, _ := db.GetMatch(rec.MatchID)
		choices = append(choices, singleMatchChoice{
			matchID: rec.MatchID,
			optName: optName,
			odds:    optOdds,
			prob:    optProb,
			desc:    fmt.Sprintf("%s(%s@%.2f)", m.HomeTeam, optName, optOdds),
		})
	}

	kValActual := len(choices)
	if kValActual < subTicketSizes[0] {
		// 剩余能买的场数不够串关，不推荐该玩法
		return ParlayAdvice{
			ParlayType:         playName,
			ComboOdds:          0,
			ComboProb:          0,
			SingleTicketPayout: 0,
			KellyStake:         0,
			TotalEV:            0,
			Cost:               0,
			WinsCount:          0,
			MinMatchToWin:      subTicketSizes[0],
		}, ExcludedMatch{
			MatchID:  playCode,
			HomeTeam: playName,
			AwayTeam: fmt.Sprintf("【串关玩法未售/场数不足】本组赛事中仅有 %d 场开售该玩法，不足以组成所选过关方式。", kValActual),
			Reason:   "0%",
		}, nil
	}

	type subTicket struct {
		indices []int
		odds    float64
		prob    float64
		payout  float64
	}
	var tickets []subTicket

	for _, sz := range subTicketSizes {
		if sz > kValActual {
			continue
		}
		combos := combinations(kValActual, sz)
		for _, c := range combos {
			tOdds := 1.0
			tProb := 1.0
			for _, idx := range c {
				tOdds *= choices[idx].odds
				tProb *= choices[idx].prob
			}
			tPayout := bankersRound(2.0 * tOdds)
			var limit float64 = 200000.0
			if sz >= 4 && sz <= 5 {
				limit = 500000.0
			} else if sz >= 6 {
				limit = 1000000.0
			}
			if tPayout > limit {
				tPayout = limit
			}
			tickets = append(tickets, subTicket{
				indices: c,
				odds:    tOdds,
				prob:    tProb,
				payout:  tPayout,
			})
		}
	}

	if len(tickets) == 0 {
		tOdds := 1.0
		tProb := 1.0
		var idxs []int
		for idx, ch := range choices {
			tOdds *= ch.odds
			tProb *= ch.prob
			idxs = append(idxs, idx)
		}
		tPayout := bankersRound(2.0 * tOdds)
		tickets = append(tickets, subTicket{
			indices: idxs,
			odds:    tOdds,
			prob:    tProb,
			payout:  tPayout,
		})
	}

	numStates := 1 << kValActual
	var comboProb float64 = 0.0
	var totalExpectedPayout float64 = 0.0
	var maxPayout float64 = 0.0
	var totalCost float64 = float64(len(tickets)) * 2.0

	for sIdx := 0; sIdx < numStates; sIdx++ {
		sProb := 1.0
		for i := 0; i < kValActual; i++ {
			if (sIdx & (1 << i)) != 0 {
				sProb *= choices[i].prob
			} else {
				sProb *= (1.0 - choices[i].prob)
			}
		}

		var sPayout float64 = 0.0
		for _, t := range tickets {
			won := true
			for _, idx := range t.indices {
				if (sIdx & (1 << idx)) == 0 {
					won = false
					break
				}
			}
			if won {
				sPayout += t.payout
			}
		}

		if sPayout > 0 {
			comboProb += sProb
			totalExpectedPayout += sProb * sPayout
			if sPayout > maxPayout {
				maxPayout = sPayout
			}
		}
	}

	totalEV := (totalExpectedPayout / totalCost) - 1.0
	comboOdds := maxPayout / 2.0

	var savedTickets []SavedTicket
	for _, t := range tickets {
		var legs []SavedLeg
		for _, idx := range t.indices {
			legs = append(legs, SavedLeg{
				MatchID: choices[idx].matchID,
				Option:  choices[idx].optName,
				Odds:    choices[idx].odds,
			})
		}
		savedTickets = append(savedTickets, SavedTicket{
			Odds:   t.odds,
			Payout: t.payout,
			Legs:   legs,
		})
	}
	ticketsJSONBytes, _ := json.Marshal(savedTickets)
	ticketsJSON := string(ticketsJSONBytes)

	advice := ParlayAdvice{
		ParlayType:         playName,
		ComboOdds:          bankersRound(comboOdds),
		ComboProb:          comboProb,
		SingleTicketPayout: maxPayout,
		KellyStake:         bankersRound(math.Max(0.01, math.Min(0.05, comboProb*totalEV))),
		TotalEV:            totalEV,
		Cost:               totalCost,
		WinsCount:          len(tickets),
		MinMatchToWin:      subTicketSizes[0],
		TicketsJSON:        ticketsJSON,
	}

	var playDetail strings.Builder
	for tIdx, t := range tickets {
		if tIdx > 0 {
			playDetail.WriteString("<br>")
		}
		playDetail.WriteString(fmt.Sprintf("注%d: ", tIdx+1))
		for i, idx := range t.indices {
			if i > 0 {
				playDetail.WriteString(" × ")
			}
			playDetail.WriteString(choices[idx].desc)
		}
	}

	var parlayName string
	if parlayMode == "free" {
		parlayName = strings.Join(parlayOptions, ",") + "串1自由过关"
	} else {
		if len(parlayOptions) > 0 {
			parlayName = translateOptToChinese(parlayOptions[0])
			if isDowngraded && originalOpt != "" {
				parlayName = fmt.Sprintf("%s(原%s因风控降级)", parlayName, translateOptToChinese(originalOpt))
			}
		} else {
			parlayName = fmt.Sprintf("%d串1", kValActual)
		}
	}
	descStr := fmt.Sprintf("【过关方式:%s】共%d注。如果全对%d场最高奖金%.2f元。细则：<br>%s", parlayName, len(tickets), kValActual, maxPayout, playDetail.String())

	excl := ExcludedMatch{
		MatchID:  playCode,
		HomeTeam: playName,
		AwayTeam: descStr,
		Reason:   fmt.Sprintf("%.1f%%", comboProb*100),
	}

	return advice, excl, nil
}

// combinations 辅助函数，从 n 个元素里选 k 个的索引组合
func combinations(n, k int) [][]int {
	var res [][]int
	var helper func(start int, path []int)
	helper = func(start int, path []int) {
		if len(path) == k {
			temp := make([]int, k)
			copy(temp, path)
			res = append(res, temp)
			return
		}
		for i := start; i < n; i++ {
			helper(i+1, append(path, i))
		}
	}
	helper(0, []int{})
	return res
}

func getParlaySubTicketSizes(mode string, options []string, M int) []int {
	if mode == "free" {
		var res []int
		for _, opt := range options {
			var val int
			if _, err := fmt.Sscanf(opt, "%d", &val); err == nil {
				res = append(res, val)
			}
		}
		if len(res) == 0 {
			return []int{M}
		}
		return res
	}
	if len(options) == 0 {
		return []int{M}
	}
	opt := options[0]
	switch opt {
	case "2x1": return []int{2}
	case "2x3": return []int{1, 2}
	case "3x1": return []int{3}
	case "3x3": return []int{2}
	case "3x4": return []int{2, 3}
	case "3x7": return []int{1, 2, 3}
	case "4x1": return []int{4}
	case "4x4": return []int{3}
	case "4x5": return []int{3, 4}
	case "4x6": return []int{2}
	case "4x11": return []int{2, 3, 4}
	case "5x1": return []int{5}
	case "5x5": return []int{4}
	case "5x6": return []int{4, 5}
	case "5x10": return []int{2}
	case "5x16": return []int{3, 4, 5}
	case "5x20": return []int{2, 3}
	case "5x26": return []int{2, 3, 4, 5}
	}
	var mVal, nVal int
	if _, err := fmt.Sscanf(opt, "%dx%d", &mVal, &nVal); err == nil {
		if mVal == 6 {
			switch nVal {
			case 1: return []int{6}
			case 6: return []int{5}
			case 7: return []int{5, 6}
			case 15: return []int{2}
			case 20: return []int{3}
			case 22: return []int{4, 5, 6}
			case 35: return []int{2, 3}
			case 50: return []int{2, 3, 4}
			case 57: return []int{2, 3, 4, 5, 6}
			}
		}
	}
	return []int{M}
}

func (s *ParlayService) getBestSingleChoice(matchID string, playCode string) (string, float64, float64, error) {
	m, err := db.GetMatch(matchID)
	if err != nil {
		return "", 0, 0, err
	}
	
	// 读取已校验的 DB Report，并做防崩溃安全降级兜底
	report, errRep := db.GetPredictionReport(matchID)
	if errRep != nil || len(report.ScoreMatrix) == 0 {
		// 若读取失败或尚未预测，使用定量 Dixon-Coles 原始数学计算兜底，防除零崩溃
		params := s.dcService.CalculateParamsWithVenue(m.HomeTeam, m.AwayTeam, m.Venue)
		matrix, over25, under25 := s.dcService.GenerateProbabilityMatrix(params)
		report = models.PredictionReport{
			MatchID:        matchID,
			OriginalParams: params,
			RefinedParams:  params,
			ScoreMatrix:    matrix,
			Over2_5Prob:    over25,
			Under2_5Prob:   under25,
		}
	}
	odds := s.sportteryService.GetMatchOdds(m.HomeTeam, m.AwayTeam, m.ScheduledAt)

	var optName string
	var optOdds float64
	var optProb float64

	var pHome, pDraw, pAway float64
	for _, cell := range report.ScoreMatrix {
		if cell.HomeScore > cell.AwayScore {
			pHome += cell.Prob
		} else if cell.HomeScore == cell.AwayScore {
			pDraw += cell.Prob
		} else {
			pAway += cell.Prob
		}
	}

	switch playCode {
	case "had":
		oH, oD, oA := odds.HomeOdds, odds.DrawOdds, odds.AwayOdds
		if !odds.IsAvailable {
			oH, oD, oA = 0.89/pHome, 0.89/pDraw, 0.89/pAway
		}
		// 校验：如果官方已在售其他玩法，但当前常规胜平负未售，智能降级切换至已开售的让球玩法以保证混合串关组合
		if odds.IsAvailable && oH <= 0.0 {
			if odds.HhadHomeOdds > 0.0 {
				return s.getBestSingleChoice(matchID, "hhad")
			}
			return "", 0.0, 0.0, nil
		}
		evH := pHome*oH - 1.0
		evD := pDraw*oD - 1.0
		evA := pAway*oA - 1.0
		if evH >= evD && evH >= evA {
			optName, optOdds, optProb = "主胜", oH, pHome
		} else if evD >= evH && evD >= evA {
			optName, optOdds, optProb = "平局", oD, pDraw
		} else {
			optName, optOdds, optProb = "客胜", oA, pAway
		}

	case "hhad":
		gLine := odds.GoalLine
		if gLine == 0 {
			gLine = -1
		}
		var pRHome, pRDraw, pRAway float64
		for _, cell := range report.ScoreMatrix {
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
		if !odds.IsAvailable && oRH <= 0 {
			oRH, oRD, oRA = 0.89/pRHome, 0.89/pRDraw, 0.89/pRAway
		}
		// 校验：如果官方已开售其他玩法，但当前让球胜平负未开售，智能升级切换至已开售的常规胜平负玩法以保证混合串关组合
		if odds.IsAvailable && oRH <= 0.0 {
			if odds.HomeOdds > 0.0 {
				return s.getBestSingleChoice(matchID, "had")
			}
			return "", 0.0, 0.0, nil
		}
		evRH := pRHome*oRH - 1.0
		evRD := pRDraw*oRD - 1.0
		evRA := pRAway*oRA - 1.0
		if evRH >= evRD && evRH >= evRA {
			optName, optOdds, optProb = fmt.Sprintf("让胜(%d)", gLine), oRH, pRHome
		} else if evRD >= evRH && evRD >= evRA {
			optName, optOdds, optProb = fmt.Sprintf("让平(%d)", gLine), oRD, pRDraw
		} else {
			optName, optOdds, optProb = fmt.Sprintf("让负(%d)", gLine), oRA, pRAway
		}

	case "hafu":
		hafuProbs := make(map[string]float64)
		options := []string{"胜胜", "胜平", "胜负", "平胜", "平平", "平负", "负胜", "负平", "负负"}
		for _, op := range options {
			hafuProbs[op] = 0.0
		}
		lh := report.RefinedParams.LambdaHome
		la := report.RefinedParams.LambdaAway
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

		hafuKeys := map[string]string{
			"胜胜": "hh", "胜平": "hd", "胜负": "ha",
			"平胜": "dh", "平平": "dd", "平负": "da",
			"负胜": "ah", "负平": "ad", "负负": "aa",
		}

		var bestHafu string
		var bestHafuProb float64
		var bestHafuOdds float64

		for op, prob := range hafuProbs {
			apiCode := hafuKeys[op]
			oVal := odds.HafuOdds[apiCode]
			if oVal <= 0 {
				oVal = 0.89 / math.Max(0.001, prob)
			}
			ev := prob*oVal - 1.0
			if ev > (bestHafuProb*bestHafuOdds - 1.0) || bestHafu == "" {
				bestHafu, bestHafuProb, bestHafuOdds = op, prob, oVal
			}
		}
		optName, optOdds, optProb = bestHafu, bestHafuOdds, bestHafuProb

	case "ttg":
		ttgProbs := make([]float64, 8)
		for _, cell := range report.ScoreMatrix {
			tot := cell.HomeScore + cell.AwayScore
			if tot >= 7 {
				ttgProbs[7] += cell.Prob
			} else {
				ttgProbs[tot] += cell.Prob
			}
		}

		var bestTtg string
		var bestTtgProb float64
		var bestTtgOdds float64

		for g := 0; g <= 7; g++ {
			prob := ttgProbs[g]
			apiCode := fmt.Sprintf("s%d", g)
			oVal := odds.TtgOdds[apiCode]
			if oVal <= 0 {
				oVal = 0.89 / math.Max(0.001, prob)
			}
			ev := prob*oVal - 1.0
			if ev > (bestTtgProb*bestTtgOdds - 1.0) || bestTtg == "" {
				bestTtg = fmt.Sprintf("%d球", g)
				if g == 7 {
					bestTtg = "7+球"
				}
				bestTtgProb, bestTtgOdds = prob, oVal
			}
		}
		optName, optOdds, optProb = bestTtg, bestTtgOdds, bestTtgProb

	case "crs":
		aggProbs := make(map[string]float64) // key: preciseKey
		for _, cell := range report.ScoreMatrix {
			code := getPreciseCrsKey(cell.HomeScore, cell.AwayScore)
			aggProbs[code] += cell.Prob
		}

		var bestCrs string
		var bestCrsProb float64
		var bestCrsOdds float64

		// 竞彩官方 31 个比分代码
		officialCrsCodes := []string{
			"s01s00", "s02s00", "s02s01", "s03s00", "s03s01", "s03s02",
			"s04s00", "s04s01", "s04s02", "s05s00", "s05s01", "s05s02", "s1sh",
			"s00s00", "s01s01", "s02s02", "s03s03", "s1sd",
			"s00s01", "s00s02", "s01s02", "s00s03", "s01s03", "s02s03",
			"s00s04", "s01s04", "s02s04", "s00s05", "s01s05", "s02s05", "s1sa",
		}

		for _, code := range officialCrsCodes {
			prob := aggProbs[code]
			oVal := odds.CrsOdds[code]
			if oVal <= 0 {
				oVal = 0.89 / math.Max(0.001, prob)
			}
			oVal = math.Min(100.0, oVal)
			ev := prob*oVal - 1.0
			if ev > (bestCrsProb*bestCrsOdds - 1.0) || bestCrs == "" {
				bestCrs, bestCrsProb, bestCrsOdds = getCrsDisplayName(code), prob, oVal
			}
		}
		optName, optOdds, optProb = bestCrs, bestCrsOdds, bestCrsProb
	}
	return optName, optOdds, optProb, nil
}

func translateOptToChinese(opt string) string {
	if !strings.Contains(opt, "x") {
		return opt
	}
	parts := strings.Split(opt, "x")
	if len(parts) != 2 {
		return opt
	}
	return fmt.Sprintf("%s串%s", parts[0], parts[1])
}

func getDowngradedOption(opt string, kVal int) string {
	if !strings.Contains(opt, "x") {
		return opt
	}
	var m, n int
	_, err := fmt.Sscanf(opt, "%dx%d", &m, &n)
	if err != nil {
		return opt
	}
	if kVal >= m {
		return opt
	}
	if kVal < 2 {
		return opt
	}
	switch m {
	case 3:
		if kVal == 2 {
			switch n {
			case 1, 3: return "2x1"
			case 4: return "2x3"
			}
		}
	case 4:
		if kVal == 3 {
			switch n {
			case 1, 4, 5: return "3x1"
			case 6: return "3x3"
			case 11: return "3x4"
			}
		} else if kVal == 2 {
			switch n {
			case 1, 4, 5, 6: return "2x1"
			case 11: return "2x3"
			}
		}
	case 5:
		if kVal == 4 {
			switch n {
			case 1, 5, 6: return "4x1"
			case 10: return "4x6"
			case 16: return "4x5"
			case 20: return "4x6"
			case 26: return "4x11"
			}
		} else if kVal == 3 {
			switch n {
			case 1, 5, 6, 16: return "3x1"
			case 10, 20: return "3x3"
			case 26: return "3x4"
			}
		} else if kVal == 2 {
			switch n {
			case 1, 5, 6, 16, 10, 20: return "2x1"
			case 26: return "2x3"
			}
		}
	case 6:
		if kVal == 5 {
			switch n {
			case 1, 6, 7: return "5x1"
			case 15, 20: return "5x10"
			case 22: return "5x16"
			case 35, 50: return "5x20"
			case 57: return "5x26"
			}
		} else if kVal == 4 {
			switch n {
			case 1, 6, 7: return "4x1"
			case 15, 20, 35: return "4x6"
			case 22: return "4x5"
			case 50, 57: return "4x11"
			}
		} else if kVal == 3 {
			switch n {
			case 1, 6, 7, 22: return "3x1"
			case 15, 20, 35: return "3x3"
			case 50, 57: return "3x4"
			}
		} else if kVal == 2 {
			switch n {
			case 1, 6, 7, 22, 15, 20, 35: return "2x1"
			case 50, 57: return "2x3"
			}
		}
	}
	return fmt.Sprintf("%dx1", kVal)
}

func getPreciseCrsKey(homeScore, awayScore int) string {
	if homeScore > 5 || awayScore > 5 {
		if homeScore > awayScore {
			return "s1sh" // 胜其它
		} else if homeScore == awayScore {
			return "s1sd" // 平其它
		} else {
			return "s1sa" // 负其它
		}
	}
	isAvailable := false
	if homeScore > awayScore {
		if (homeScore == 1 && awayScore == 0) ||
			(homeScore == 2 && (awayScore == 0 || awayScore == 1)) ||
			(homeScore == 3 && (awayScore == 0 || awayScore == 1 || awayScore == 2)) ||
			(homeScore == 4 && (awayScore == 0 || awayScore == 1 || awayScore == 2)) ||
			(homeScore == 5 && (awayScore == 0 || awayScore == 1 || awayScore == 2)) {
			isAvailable = true
		}
	} else if homeScore == awayScore {
		if homeScore >= 0 && homeScore <= 3 {
			isAvailable = true
		}
	} else {
		if (awayScore == 1 && homeScore == 0) ||
			(awayScore == 2 && (homeScore == 0 || homeScore == 1)) ||
			(awayScore == 3 && (homeScore == 0 || homeScore == 1 || homeScore == 2)) ||
			(awayScore == 4 && (homeScore == 0 || homeScore == 1 || homeScore == 2)) ||
			(awayScore == 5 && (homeScore == 0 || homeScore == 1 || homeScore == 2)) {
			isAvailable = true
		}
	}
	if isAvailable {
		return fmt.Sprintf("s%02ds%02d", homeScore, awayScore)
	}
	if homeScore > awayScore {
		return "s1sh"
	} else if homeScore == awayScore {
		return "s1sd"
	}
	return "s1sa"
}
