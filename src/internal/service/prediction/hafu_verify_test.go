package prediction

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func TestHafuVerify(t *testing.T) {
	// 1. 初始化数据库
	dataDir := "../../../../data/db"
	err := db.Init(dataDir)
	if err != nil {
		t.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()
	db.InitTeamTranslations()

	// 2. 初始化服务实例
	eloService, errElo := NewEloService("../../../../data/seasons/history_features.json")
	if errElo != nil {
		t.Fatalf("初始化 EloService 失败: %v", errElo)
	}
	// APISportsService 传 nil，DixonColes 会自动处理，避免网络请求干扰
	dcService := NewDixonColesService(eloService, nil)
	sportteryService := NewSportteryService()

	// 3. 获取前 10 场比赛
	matches, err := db.GetMatchesByTournament("fifa_2026")
	if err != nil {
		t.Fatalf("获取比赛列表失败: %v", err)
	}

	limit := 10
	if len(matches) < limit {
		limit = len(matches)
	}
	if limit == 0 {
		t.Fatalf("没有找到任何比赛数据")
	}

	options := []string{"胜胜", "胜平", "胜负", "平胜", "平平", "平负", "负胜", "负平", "负负"}
	
	// 汇总各选项的概率
	sumProbs := make(map[string]float64)
	for _, op := range options {
		sumProbs[op] = 0.0
	}

	fmt.Printf("\n### 🚀 连续 10 场比赛半全场 9 种概率计算明细\n\n")
	fmt.Printf("| 比赛对阵 (名义主 vs 名义客) | 胜胜 | 胜平 | 胜负 | 平胜 | 平平 | 平负 | 负胜 | 负平 | 负负 |\n")
	fmt.Printf("|---|---|---|---|---|---|---|---|---|---|\n")

	for i := 0; i < limit; i++ {
		m := matches[i]
		params := dcService.CalculateParamsWithVenue(m.HomeTeam, m.AwayTeam, m.Venue)
		odds := sportteryService.GetMatchOdds(m.HomeTeam, m.AwayTeam, m.ScheduledAt)
		
		probs := CalculateRefinedHafuProbs(params.LambdaHome, params.LambdaAway, m, odds, dcService)
		
		for _, op := range options {
			sumProbs[op] += probs[op]
		}

		fmt.Sprintf("%s vs %s", m.HomeTeam, m.AwayTeam)
		fmt.Printf("| %s vs %s | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.2f%% | %.2f%% |\n",
			translateTeam(m.HomeTeam), translateTeam(m.AwayTeam),
			probs["胜胜"]*100, probs["胜平"]*100, probs["胜负"]*100,
			probs["平胜"]*100, probs["平平"]*100, probs["平负"]*100,
			probs["负胜"]*100, probs["负平"]*100, probs["负负"]*100,
		)
	}

	fmt.Printf("\n### 📊 9 种可能结果的平均概率统计汇总\n\n")
	fmt.Printf("| 半全场结果 | 平均概率 (10场样本) |\n")
	fmt.Printf("|---|---|\n")
	for _, op := range options {
		avg := sumProbs[op] / float64(limit)
		fmt.Printf("| **%s** | **%.2f%%** |\n", op, avg*100)
	}
	fmt.Println()
}

func TestBacktestVerify(t *testing.T) {
	// 1. 初始化数据库
	dataDir := "../../../../data/db"
	err := db.Init(dataDir)
	if err != nil {
		t.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()
	db.InitTeamTranslations()

	// 2. 初始化服务
	eloService, _ := NewEloService("../../../../data/seasons/history_features.json")
	dcService := NewDixonColesService(eloService, nil)
	sportteryService := NewSportteryService()
	lotteryService := NewLotteryService(dcService, sportteryService)

	// 3. 获取所有已完赛的比赛
	rows, err := db.DB.Query("SELECT id, home_team, away_team, home_score, away_score, venue, scheduled_at FROM matches WHERE status = 'FT' ORDER BY scheduled_at ASC")
	if err != nil {
		t.Fatalf("获取已完赛比赛失败: %v", err)
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var schedStr string
		err := rows.Scan(&m.ID, &m.HomeTeam, &m.AwayTeam, &m.HomeScore, &m.AwayScore, &m.Venue, &schedStr)
		if err == nil {
			m.Status = "FT"
			m.ScheduledAt, _ = time.Parse(time.RFC3339, schedStr)
			if m.ScheduledAt.IsZero() {
				m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05", schedStr)
			}
			matches = append(matches, m)
		}
	}

	fmt.Printf("\n### 🔍 完赛 %d 场比赛预测与投注回测明细表\n\n", len(matches))
	fmt.Printf("| 比赛对阵 | 实际比分 | 原始DC胜平负概率 | 稳妥主推 | 对冲建议 | 稳妥主推判定 | 激进主推 | 激进概率 | 实际盈亏(假设每单投注100元) |\n")
	fmt.Printf("|---|---|---|---|---|---|---|---|---|\n")

	var totalSafeHits, totalAggHits, totalRecommends int
	var totalSafePnl, totalAggPnl float64
	var totalBrier float64

	for _, m := range matches {
		params := dcService.CalculateParamsWithVenue(m.HomeTeam, m.AwayTeam, m.Venue)
		matrix, _, _ := dcService.GenerateProbabilityMatrix(params)
		
		var pH, pD, pA float64
		for _, cell := range matrix {
			if cell.HomeScore > cell.AwayScore {
				pH += cell.Prob
			} else if cell.HomeScore == cell.AwayScore {
				pD += cell.Prob
			} else {
				pA += cell.Prob
			}
		}

		var oH, oD, oA float64
		if m.HomeScore > m.AwayScore {
			oH = 1.0
		} else if m.HomeScore == m.AwayScore {
			oD = 1.0
		} else {
			oA = 1.0
		}
		bs := math.Pow(pH-oH, 2) + math.Pow(pD-oD, 2) + math.Pow(pA-oA, 2)
		totalBrier += bs

		// 智能获取/模拟体彩赔率 (若未匹配到官方真实赔率则采用返还率 90% 的数学期望返还赔率)
		odds := sportteryService.GetMatchOdds(m.HomeTeam, m.AwayTeam, m.ScheduledAt)
		if !odds.IsAvailable || odds.HomeOdds <= 0.0 {
			odds = OfficialOdds{
				HomeOdds: math.Round(0.90/pH*100) / 100.0,
				DrawOdds: math.Round(0.90/pD*100) / 100.0,
				AwayOdds: math.Round(0.90/pA*100) / 100.0,
				IsAvailable: true,
			}
			odds.CrsOdds = make(map[string]float64)
			// 从比分矩阵模拟比分赔率
			for _, cell := range matrix {
				code := fmt.Sprintf("s%02ds%02d", cell.HomeScore, cell.AwayScore)
				if cell.Prob > 0 {
					odds.CrsOdds[code] = math.Round(0.85/cell.Prob*100) / 100.0 // 比分抽水通常在 15% 左右
				}
			}
		}

		// 注册赔率缓存，确保系统服务可以读取
		sportteryService.mu.Lock()
		if sportteryService.cachedOdds == nil {
			sportteryService.cachedOdds = make(map[string]OfficialOdds)
		}
		odds.MatchTime = m.ScheduledAt
		sportteryService.cachedOdds[m.HomeTeam+"_"+m.AwayTeam] = odds
		sportteryService.mu.Unlock()

		// 调用真实系统服务生成推荐
		advice := lotteryService.GenerateSingleAdvice(m, 0, 0, 0, nil, false)

		isRecommended := advice.Status == "RECOMMENDED"
		primaryBet := advice.PrimaryBet
		primaryOdds := advice.PrimaryOdds

		hedgeOutcome := "无对冲"
		hedgeOdds := 0.0
		if len(advice.HedgeBets) > 0 {
			hedgeOutcome = advice.HedgeBets[0].Outcome
			hedgeOdds = advice.HedgeBets[0].Odds
		}

		safeHit := false
		safePnl := 0.0
		if isRecommended {
			totalRecommends++
			safeHit = checkHit(m.HomeScore, m.AwayScore, advice.PrimaryBet)
			if safeHit {
				totalSafeHits++
				safePnl = (100.0 * advice.PrimaryStake * advice.PrimaryOdds) - 100.0
			} else {
				// 检查对冲项是否命中
				hedgeHit := false
				if len(advice.HedgeBets) > 0 {
					hedgeHit = checkHit(m.HomeScore, m.AwayScore, advice.HedgeBets[0].Outcome)
					if hedgeHit {
						safePnl = (100.0 * advice.HedgeBets[0].StakePct * advice.HedgeBets[0].Odds) - 100.0
					}
				}
				if !hedgeHit {
					safePnl = -100.0
				}
			}
			totalSafePnl += safePnl
		}

		// 激进单场策略 (HAD 玩法中期望价值 EV 最大的选项)
		evH := pH*odds.HomeOdds - 1.0
		evD := pD*odds.DrawOdds - 1.0
		evA := pA*odds.AwayOdds - 1.0

		aggBet := "主胜 (3)"
		aggOdds := odds.HomeOdds
		maxEV := evH
		if evD > maxEV {
			aggBet = "平局 (1)"
			aggOdds = odds.DrawOdds
			maxEV = evD
		}
		if evA > maxEV {
			aggBet = "客胜 (0)"
			aggOdds = odds.AwayOdds
		}

		aggHit := checkHit(m.HomeScore, m.AwayScore, aggBet)
		aggPnl := 0.0
		if aggHit {
			totalAggHits++
			aggPnl = (100.0 * aggOdds) - 100.0
		} else {
			aggPnl = -100.0
		}
		totalAggPnl += aggPnl

		safeHitStr := "❌ 未中"
		if isRecommended {
			if safeHit {
				safeHitStr = "✅ 命中"
			}
		} else {
			safeHitStr = "隔离/不推荐"
		}
		aggHitStr := "❌ 未中"
		if aggHit {
			aggHitStr = "✅ 命中"
		}

		fmt.Printf("| %s vs %s | %d:%d | 主胜:%.1f%% 平:%.1f%% 客胜:%.1f%% | %s(@%.2f) | %s(@%.2f) | %s | %s(@%.2f) | %s | 稳妥:%.1f元 / 激进:%.1f元 |\n",
			translateTeam(m.HomeTeam), translateTeam(m.AwayTeam), m.HomeScore, m.AwayScore,
			pH*100, pD*100, pA*100,
			primaryBet, primaryOdds, hedgeOutcome, hedgeOdds, safeHitStr,
			aggBet, aggOdds, aggHitStr, safePnl, aggPnl,
		)
	}

	fmt.Printf("\n### 📊 完赛数据回测整体统计\n\n")
	fmt.Printf("- **总完赛样本量**: %d 场\n", len(matches))
	fmt.Printf("- **模型平均 Brier Score (越低越准)**: %.4f\n", totalBrier/float64(len(matches)))
	fmt.Printf("- **稳妥主推命中率 (推荐场次)**: %.2f%% (%d/%d)\n", float64(totalSafeHits)/float64(totalRecommends)*100.0, totalSafeHits, totalRecommends)
	fmt.Printf("- **稳妥投资累计盈亏 (单注100元)**: %.2f 元\n", totalSafePnl)
	fmt.Printf("- **激进投资累计盈亏 (单注100元)**: %.2f 元\n", totalAggPnl)
}

func checkHit(h, a int, option string) bool {
	if option == "主胜" || option == "主胜 (3)" {
		return h > a
	}
	if option == "平局" || option == "平局 (1)" {
		return h == a
	}
	if option == "客胜" || option == "客胜 (0)" {
		return h < a
	}

	if strings.Contains(option, "让胜") {
		var gLine int
		fmt.Sscanf(option, "让胜(%d)", &gLine)
		if gLine == 0 {
			gLine = -1
		}
		return h-a+gLine > 0
	}
	if strings.Contains(option, "让平") {
		var gLine int
		fmt.Sscanf(option, "让平(%d)", &gLine)
		if gLine == 0 {
			gLine = -1
		}
		return h-a+gLine == 0
	}
	if strings.Contains(option, "让负") {
		var gLine int
		fmt.Sscanf(option, "让负(%d)", &gLine)
		if gLine == 0 {
			gLine = -1
		}
		return h-a+gLine < 0
	}

	if strings.HasSuffix(option, "球") {
		if option == "7+球" {
			return h+a >= 7
		}
		var goals int
		_, err := fmt.Sscanf(option, "%d球", &goals)
		if err == nil {
			return h+a == goals
		}
	}

	if strings.Contains(option, ":") {
		var hGoal, aGoal int
		_, err := fmt.Sscanf(option, "%d:%d", &hGoal, &aGoal)
		if err == nil {
			return h == hGoal && a == aGoal
		}
	}
	if option == "比分 1-1" || option == "1-1" {
		return h == 1 && a == 1
	}
	if option == "比分 1-0" || option == "1-0" {
		return h == 1 && a == 0
	}
	if option == "比分 0-1" || option == "0-1" {
		return h == 0 && a == 1
	}

	return false
}

func TestOptimizeParams(t *testing.T) {
	dataDir := "../../../../data/db"
	_ = db.Init(dataDir)
	defer db.Close()
	db.InitTeamTranslations()
	eloService, _ := NewEloService("../../../../data/seasons/history_features.json")
	sportteryService := NewSportteryService()
	rows, _ := db.DB.Query("SELECT id, home_team, away_team, home_score, away_score, venue, scheduled_at FROM matches WHERE status = 'FT' ORDER BY scheduled_at ASC")
	defer rows.Close()
	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var schedStr string
		_ = rows.Scan(&m.ID, &m.HomeTeam, &m.AwayTeam, &m.HomeScore, &m.AwayScore, &m.Venue, &schedStr)
		m.Status = "FT"
		m.ScheduledAt, _ = time.Parse(time.RFC3339, schedStr)
		if m.ScheduledAt.IsZero() {
			m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05", schedStr)
		}
		matches = append(matches, m)
	}
	bestBrier := 999.0
	var bestNormDiv, bestDiffMult, bestH2hMult, bestRho, bestSafePnl, bestAggPnl, bestSafeHitRate float64
	normDivs := []float64{0.95, 1.00, 1.05, 1.10}
	diffMults := []float64{0.20, 0.25, 0.30, 0.35, 0.40, 0.45}
	h2hMults := []float64{0.05, 0.10, 0.15, 0.20}
	rhos := []float64{-0.03, -0.05, -0.08, -0.10, -0.12}
	for _, nd := range normDivs {
		for _, dm := range diffMults {
			for _, hm := range h2hMults {
				for _, r := range rhos {
					dcService := NewDixonColesService(eloService, nil)
					dcService.NormDivulator, dcService.DiffMultiplier, dcService.H2hMultiplier, dcService.InitialRho = nd, dm, hm, r
					dcService.RecalculateRhoOffset()
					dcService.RecalculateLambdaOffset()
					lotteryService := NewLotteryService(dcService, sportteryService)
					var totalBrier, totalSafePnl, totalAggPnl float64
					var totalSafeHits, totalRecommends int
					for _, m := range matches {
						params := dcService.CalculateParamsWithVenue(m.HomeTeam, m.AwayTeam, m.Venue)
						matrix, _, _ := dcService.GenerateProbabilityMatrix(params)
						var pH, pD, pA float64
						for _, cell := range matrix {
							if cell.HomeScore > cell.AwayScore { pH += cell.Prob } else if cell.HomeScore == cell.AwayScore { pD += cell.Prob } else { pA += cell.Prob }
						}
						var oH, oD, oA float64
						if m.HomeScore > m.AwayScore { oH = 1.0 } else if m.HomeScore == m.AwayScore { oD = 1.0 } else { oA = 1.0 }
						totalBrier += math.Pow(pH-oH, 2) + math.Pow(pD-oD, 2) + math.Pow(pA-oA, 2)
						odds := sportteryService.GetMatchOdds(m.HomeTeam, m.AwayTeam, m.ScheduledAt)
						if !odds.IsAvailable || odds.HomeOdds <= 0.0 {
							odds = OfficialOdds{HomeOdds: math.Round(0.90/pH*100)/100.0, DrawOdds: math.Round(0.90/pD*100)/100.0, AwayOdds: math.Round(0.90/pA*100)/100.0, IsAvailable: true}
						}
						sportteryService.mu.Lock()
						if sportteryService.cachedOdds == nil { sportteryService.cachedOdds = make(map[string]OfficialOdds) }
						odds.MatchTime = m.ScheduledAt
						sportteryService.cachedOdds[m.HomeTeam+"_"+m.AwayTeam] = odds
						sportteryService.mu.Unlock()
						advice := lotteryService.GenerateSingleAdvice(m, 0, 0, 0, nil, false)
						if advice.Status == "RECOMMENDED" {
							totalRecommends++
							safeHit := checkHit(m.HomeScore, m.AwayScore, advice.PrimaryBet)
							safePnl := 0.0
							if safeHit {
								totalSafeHits++
								safePnl = (100.0 * advice.PrimaryStake * advice.PrimaryOdds) - 100.0
							} else {
								hedgeHit := false
								if len(advice.HedgeBets) > 0 {
									hedgeHit = checkHit(m.HomeScore, m.AwayScore, advice.HedgeBets[0].Outcome)
									if hedgeHit { safePnl = (100.0 * advice.HedgeBets[0].StakePct * advice.HedgeBets[0].Odds) - 100.0 }
								}
								if !hedgeHit { safePnl = -100.0 }
							}
							totalSafePnl += safePnl
						}
						evH, evD, evA := pH*odds.HomeOdds-1.0, pD*odds.DrawOdds-1.0, pA*odds.AwayOdds-1.0
						aggBet, aggOdds, maxEV := "主胜 (3)", odds.HomeOdds, evH
						if evD > maxEV { aggBet, aggOdds, maxEV = "平局 (1)", odds.DrawOdds, evD }
						if evA > maxEV { aggBet, aggOdds = "客胜 (0)", odds.AwayOdds }
						aggPnl := -100.0
						if checkHit(m.HomeScore, m.AwayScore, aggBet) { aggPnl = (100.0 * aggOdds) - 100.0 }
						totalAggPnl += aggPnl
					}
					avgBrier := totalBrier / float64(len(matches))
					if avgBrier < bestBrier {
						bestBrier, bestNormDiv, bestDiffMult, bestH2hMult, bestRho, bestSafePnl, bestAggPnl = avgBrier, nd, dm, hm, r, totalSafePnl, totalAggPnl
						if totalRecommends > 0 { bestSafeHitRate = float64(totalSafeHits) / float64(totalRecommends) }
					}
				}
			}
		}
	}
	fmt.Printf("\n🏆 【最优预测参数网格搜索结果】\n- **最小平均 Brier Score**: %.6f\n- **最优归一化分母 (NormDivulator)**: %.2f\n- **最优实力差系数 (DiffMultiplier)**: %.2f\n- **最优交锋修正系数 (H2hMultiplier)**: %.2f\n- **最优初始平局系数 (InitialRho)**: %.2f\n- **对应稳妥主推命中率**: %.2f%%\n- **对应稳妥累计收益**: %.2f 元\n- **对应激进累计收益**: %.2f 元\n", bestBrier, bestNormDiv, bestDiffMult, bestH2hMult, bestRho, bestSafeHitRate*100, bestSafePnl, bestAggPnl)
}

