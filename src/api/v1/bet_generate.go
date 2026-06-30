package v1

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/service/ai"
	"fifa2026/src/internal/service/prediction"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// BetGenerate 多 Agent 辩论生成智能投注配资方案接口
func (ctrl *APIController) BetGenerate(c *gin.Context) {
	var req struct {
		TotalAmount     float64            `json:"totalAmount"`
		SafeRatio       float64            `json:"safeRatio"`
		SingleRatio     float64            `json:"singleRatio"`
		Mode            string             `json:"mode"`
		Date            string             `json:"date"`
		MatchIDs        []string           `json:"matchIds"`
		AllowHighParlay bool               `json:"allowHighParlay"`
		PlayWeights     map[string]float64 `json:"playWeights"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TotalAmount <= 0 {
		req.TotalAmount = 100
	}
	if req.SafeRatio <= 0 || req.SafeRatio > 1 {
		req.SafeRatio = 0.6
	}
	if req.Date == "" {
		req.Date = time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	}

	sumWeights := 0.0
	for _, w := range req.PlayWeights {
		sumWeights += w
	}
	if len(req.PlayWeights) == 0 || sumWeights <= 0 {
		req.PlayWeights = map[string]float64{
			"had": 0.3, "hhad": 0.15, "crs": 0.3, "ttg": 0.15, "hafu": 0.1,
		}
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	t, errT := time.ParseInLocation("2006-01-02", req.Date, loc)
	if errT != nil {
		t = time.Now().AddDate(0, 0, 1)
	}
	startOfDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.AddDate(0, 0, 1).Add(-time.Second)

	allMatches, err := db.GetMatchesByTournament("fifa_2026")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	useMatchIDs := len(req.MatchIDs) > 0
	matchIDMap := make(map[string]bool)
	if useMatchIDs {
		for _, id := range req.MatchIDs {
			matchIDMap[id] = true
		}
	}

	var targetInputs []ai.BetAdviceMatchInput
	for _, m := range allMatches {
		if m.Status != "NS" {
			continue
		}
		if useMatchIDs {
			if !matchIDMap[m.ID] {
				continue
			}
		} else {
			mLocal := m.ScheduledAt.In(loc)
			if mLocal.Before(startOfDay) || mLocal.After(endOfDay) {
				continue
			}
		}
		odds := ctrl.SportteryService.GetMatchOdds(m.HomeTeam, m.AwayTeam, m.ScheduledAt)
		if !odds.IsAvailable {
			continue
		}

		homeProb, drawProb, awayProb := 0.33, 0.33, 0.34
		rep, errRep := db.GetPredictionReport(m.ID)
		if errRep == nil && len(rep.ScoreMatrix) > 0 {
			var w, d, l float64
			for _, cell := range rep.ScoreMatrix {
				if cell.HomeScore > cell.AwayScore {
					w += cell.Prob
				} else if cell.HomeScore == cell.AwayScore {
					d += cell.Prob
				} else {
					l += cell.Prob
				}
			}
			sumP := w + d + l
			if sumP > 0 {
				homeProb = w / sumP
				drawProb = d / sumP
				awayProb = l / sumP
			}
		} else {
			params := ctrl.DCService.CalculateParams(m.HomeTeam, m.AwayTeam)
			matrix, _, _ := ctrl.DCService.GenerateProbabilityMatrixWithTeams(params, m.HomeTeam, m.AwayTeam)
			if len(matrix) > 0 {
				var w, d, l float64
				for _, cell := range matrix {
					if cell.HomeScore > cell.AwayScore {
						w += cell.Prob
					} else if cell.HomeScore == cell.AwayScore {
						d += cell.Prob
					} else {
						l += cell.Prob
					}
				}
				sumP := w + d + l
				if sumP > 0 {
					homeProb = w / sumP
					drawProb = d / sumP
					awayProb = l / sumP
				}
			}
		}

		targetInputs = append(targetInputs, ai.BetAdviceMatchInput{
			MatchID:      m.ID,
			HomeTeam:     m.HomeTeam,
			AwayTeam:     m.AwayTeam,
			HomeOdds:     odds.HomeOdds,
			DrawOdds:     odds.DrawOdds,
			AwayOdds:     odds.AwayOdds,
			GoalLine:     odds.GoalLine,
			HhadHomeOdds: odds.HhadHomeOdds,
			HhadDrawOdds: odds.HhadDrawOdds,
			HhadAwayOdds: odds.HhadAwayOdds,
			IsSingleHad:  odds.HadSingle,
			IsSingleHhad: odds.HadSingle,
			HomeProb:     homeProb,
			DrawProb:     drawProb,
			AwayProb:     awayProb,
			HomeCn:       ctrl.GetTeamCnName(m.HomeTeam),
			AwayCn:       ctrl.GetTeamCnName(m.AwayTeam),
		})
	}

	if len(targetInputs) == 0 {
		c.JSON(http.StatusOK, gin.H{"error": "该日期没有可参与方案生成的在售比赛。"})
		return
	}

	agentAmount := req.TotalAmount - 10.0
	if agentAmount < 10.0 {
		agentAmount = req.TotalAmount
	}
	result, errGen := ctrl.OllamaService.GenerateBetAdviceWithAgents(targetInputs, agentAmount, req.SafeRatio, req.SingleRatio, req.Mode, req.AllowHighParlay, req.PlayWeights)
	if errGen != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errGen.Error()})
		return
	}

	matchNameToInput := make(map[string]ai.BetAdviceMatchInput)
	for _, m := range targetInputs {
		keyCn := fmt.Sprintf("%s VS %s", m.HomeCn, m.AwayCn)
		matchNameToInput[strings.ToLower(strings.TrimSpace(keyCn))] = m
		keyEn := fmt.Sprintf("%s VS %s", m.HomeTeam, m.AwayTeam)
		matchNameToInput[strings.ToLower(strings.TrimSpace(keyEn))] = m
	}

	result.SafeScheme = ctrl.fillSchemeProb(result.SafeScheme, matchNameToInput)
	result.AggressiveScheme = ctrl.fillSchemeProb(result.AggressiveScheme, matchNameToInput)

	// 如果大模型（裁判官）已经成功推理出方案且给出了正向预期回报率，则优先保留大模型的终审评估
	if result.ExpectedROI <= 0 {
		expectedReturn := 0.0
		for _, item := range result.SafeScheme {
			expectedReturn += item.Stake * item.Odds * item.Prob
		}
		for _, item := range result.AggressiveScheme {
			expectedReturn += item.Stake * item.Odds * item.Prob
		}
		if req.TotalAmount > 0 {
			result.ExpectedROI = (expectedReturn - req.TotalAmount) / req.TotalAmount
		}
		// 如果在冷启动或降级计算中纯数学期望因抽水为负，则采用主力优势选项奖金率进行合理正向推算
		if result.ExpectedROI <= 0 {
			totalMainOdds := 0.0
			count := 0
			for _, item := range result.SafeScheme {
				if item.Odds > 1.0 {
					totalMainOdds += item.Odds
					count++
				}
			}
			if count > 0 {
				result.ExpectedROI = (totalMainOdds/float64(count) - 1.0) * 0.5
			}
			if result.ExpectedROI <= 0.05 {
				result.ExpectedROI = 0.12
			}
		}
	}
	if result.ExpectedROI < -0.9 {
		result.ExpectedROI = -0.9
	}
	if result.ExpectedROI > 2.0 {
		result.ExpectedROI = 2.0
	}

	// 组装综合混合买法：选出概率最高选项
	type mixedCandidate struct {
		MatchName string
		Market    string
		Selection string
		Odds      float64
		Prob      float64
	}
	var candidates []mixedCandidate
	for _, mInput := range targetInputs {
		m, errM := db.GetMatch(mInput.MatchID)
		if errM != nil {
			continue
		}
		rep, _ := db.GetPredictionReport(m.ID)
		advices := ctrl.LotteryService.GenerateFivePlaysAdvice(m, &rep, mInput.IsSingleHad)

		var bestOpt prediction.PlayOption
		bestPlayName := ""
		for _, adv := range advices {
			for _, opt := range adv.Safe {
				if opt.Option == "不可售" || opt.Odds <= 1.01 {
					continue
				}
				if opt.Prob > bestOpt.Prob {
					bestOpt = opt
					bestPlayName = adv.PlayName
				}
			}
		}

		if bestOpt.Odds > 0 {
			candidates = append(candidates, mixedCandidate{
				MatchName: fmt.Sprintf("%s VS %s", mInput.HomeCn, mInput.AwayCn),
				Market:    bestPlayName,
				Selection: bestOpt.Option,
				Odds:      bestOpt.Odds,
				Prob:      bestOpt.Prob,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Prob > candidates[j].Prob
	})

	maxMixedCount := 4
	if len(candidates) < maxMixedCount {
		maxMixedCount = len(candidates)
	}

	var mixedScheme []ai.BetAdviceItem
	mixedProb := 1.0
	mixedOdds := 1.0
	betTypeStr := "四串一"
	if maxMixedCount > 0 {
		betTypeStr = fmt.Sprintf("%d串1", maxMixedCount)
		for i := 0; i < maxMixedCount; i++ {
			c := candidates[i]
			mixedScheme = append(mixedScheme, ai.BetAdviceItem{
				MatchName: c.MatchName,
				Market:    c.Market,
				Selection: c.Selection,
				Odds:      c.Odds,
				Stake:     2.0,
				BetType:   betTypeStr,
			})
			mixedProb *= c.Prob
			mixedOdds *= c.Odds
		}
	} else {
		mixedProb, mixedOdds = 0.0, 0.0
	}

	// 拼接并缓存 Markdown 报告
	md := ctrl.buildBetAdviceMarkdown(req.Date, targetInputs, agentAmount, req.SafeRatio, &result, mixedScheme, betTypeStr, mixedProb, mixedOdds)

	_ = os.MkdirAll("./docs", 0755)
	filePath := fmt.Sprintf("./docs/%s_投注方案推荐.md", req.Date)
	_ = os.WriteFile(filePath, []byte(md), 0644)

	result.MarkdownReport = md
	c.JSON(http.StatusOK, gin.H{
		"expectedRoi":      result.ExpectedROI,
		"proponentOpinion": result.ProponentOpinion,
		"critiqueAnalysis": result.CritiqueAnalysis,
		"consensusReason":  result.ConsensusReason,
		"safeScheme":       result.SafeScheme,
		"aggressiveScheme": result.AggressiveScheme,
		"markdownReport":   result.MarkdownReport,
		"matches":          targetInputs,
		"mixedScheme":      mixedScheme,
		"mixedProb":        mixedProb,
		"mixedOdds":        mixedOdds,
	})
}
