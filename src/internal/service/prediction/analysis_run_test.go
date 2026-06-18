package prediction

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fmt"
	"math"
	"testing"
)

type MatchAnalysisResult struct {
	MatchID      string    `json:"matchId"`
	HomeTeam     string    `json:"homeTeam"`
	AwayTeam     string    `json:"awayTeam"`
	HomeScore    int       `json:"homeScore"`
	AwayScore    int       `json:"awayScore"`
	Result       string    `json:"result"` // W / D / L
	IsOver2_5    bool      `json:"isOver2_5"`
	Original     PredModel `json:"original"`
	Refined      PredModel `json:"refined"`
	Consensus    PredModel `json:"consensus"`
	OptaSim      PredModel `json:"optaSim"`
}

type PredModel struct {
	PWin      float64 `json:"pWin"`
	PDraw     float64 `json:"pDraw"`
	PLoss     float64 `json:"pLoss"`
	POver2_5  float64 `json:"pOver2_5"`
	BrierWDW  float64 `json:"brierWdw"`
	HitWDW    bool    `json:"hitWdw"`
	HitOver2_5 bool   `json:"hitOver2_5"`
}

func TestRunQuantAnalysis(t *testing.T) {
	dataDir := "../../../../data/db"
	err := db.Init(dataDir)
	if err != nil {
		t.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()
	db.InitTeamTranslations()

	elo, err := NewEloService("../../../../data/seasons/history_features.json")
	if err != nil {
		t.Fatalf("初始化 Elo 失败: %v", err)
	}
	dc := NewDixonColesService(elo, nil)
	shin := NewShinService()

	rows, err := db.DB.Query(`
		SELECT m.id, m.home_team, m.away_team, m.home_score, m.away_score,
		       p.original_lambda_home, p.original_lambda_away, p.original_rho,
		       p.lambda_home, p.lambda_away, p.rho
		FROM matches m
		JOIN prediction_reports p ON m.id = p.match_id
		WHERE m.status = 'FT'
		ORDER BY m.scheduled_at ASC
	`)
	if err != nil {
		t.Fatalf("查询已完赛比赛失败: %v", err)
	}
	defer rows.Close()

	var results []MatchAnalysisResult
	for rows.Next() {
		var r MatchAnalysisResult
		var origLH, origLA, origRho, refLH, refLA, refRho float64
		err := rows.Scan(
			&r.MatchID, &r.HomeTeam, &r.AwayTeam, &r.HomeScore, &r.AwayScore,
			&origLH, &origLA, &origRho, &refLH, &refLA, &refRho,
		)
		if err != nil {
			t.Fatalf("读取比赛及预测数据失败: %v", err)
		}

		// 判定赛果
		if r.HomeScore > r.AwayScore {
			r.Result = "W"
		} else if r.HomeScore == r.AwayScore {
			r.Result = "D"
		} else {
			r.Result = "L"
		}
		r.IsOver2_5 = (r.HomeScore + r.AwayScore) > 2

		// 1. 原始模型
		origParams := models.DixonColesParams{LambdaHome: origLH, LambdaAway: origLA, Rho: origRho}
		matrixOrig, overOrig, _ := dc.GenerateProbabilityMatrix(origParams)
		pWOrig, pDOrig, pLOrig := getWDWProbs(matrixOrig)
		r.Original = buildPredModel(pWOrig, pDOrig, pLOrig, overOrig, r.Result, r.IsOver2_5, r.HomeScore, r.AwayScore)

		// 2. 修正后模型
		refParams := models.DixonColesParams{LambdaHome: refLH, LambdaAway: refLA, Rho: refRho}
		matrixRef, overRef, _ := dc.GenerateProbabilityMatrix(refParams)
		pWRef, pDRef, pLRef := getWDWProbs(matrixRef)
		r.Refined = buildPredModel(pWRef, pDRef, pLRef, overRef, r.Result, r.IsOver2_5, r.HomeScore, r.AwayScore)

		// 3. 博彩共识
		pWCon, pDCon, pLCon, hasCon := GetConsensusProbs(r.MatchID, shin)
		if hasCon {
			r.Consensus = buildPredModel(pWCon, pDCon, pLCon, 0.5, r.Result, r.IsOver2_5, r.HomeScore, r.AwayScore) // 大小球设 0.5 作为占位
		}

		// 4. Opta 模拟
		pWOpta, pDOpta, pLOpta := GetOptaSimulatedProbs(r.HomeTeam, r.AwayTeam)
		r.OptaSim = buildPredModel(pWOpta, pDOpta, pLOpta, 0.5, r.Result, r.IsOver2_5, r.HomeScore, r.AwayScore)

		results = append(results, r)
	}

	// 打印与保存数据
	printQuantSummary(results)
	printBettingSummary()
}

func printQuantSummary(results []MatchAnalysisResult) {
	n := float64(len(results))
	var origHitWDW, origHitOver, refHitWDW, refHitOver, conHitWDW, optaHitWDW float64
	var origBrier, refBrier, conBrier, optaBrier float64
	var conCount, optaCount float64

	for _, r := range results {
		origBrier += r.Original.BrierWDW
		refBrier += r.Refined.BrierWDW
		if r.Original.HitWDW { origHitWDW++ }
		if r.Original.HitOver2_5 { origHitOver++ }
		if r.Refined.HitWDW { refHitWDW++ }
		if r.Refined.HitOver2_5 { refHitOver++ }

		// Consensus
		if r.Consensus.PWin > 0 {
			conCount++
			conBrier += r.Consensus.BrierWDW
			if r.Consensus.HitWDW { conHitWDW++ }
		}
		// Opta
		if r.OptaSim.PWin > 0 {
			optaCount++
			optaBrier += r.OptaSim.BrierWDW
			if r.OptaSim.HitWDW { optaHitWDW++ }
		}
	}

	fmt.Printf("\n==================== 模型精度复盘汇总 (共 %d 场) ====================\n", len(results))
	fmt.Printf("1. 基础 Dixon-Coles 原始模型：\n   - 胜平负准确率: %.2f%% (%.0f/%d)\n   - 平均 Brier Score: %.4f\n   - 大小球准确率: %.2f%% (%.0f/%d)\n",
		origHitWDW/n*100, origHitWDW, len(results), origBrier/n, origHitOver/n*100, origHitOver, len(results))
	fmt.Printf("2. 双 Agent 辩论大模型修正后：\n   - 胜平负准确率: %.2f%% (%.0f/%d)\n   - 平均 Brier Score: %.4f\n   - 大小球准确率: %.2f%% (%.0f/%d)\n",
		refHitWDW/n*100, refHitWDW, len(results), refBrier/n, refHitOver/n*100, refHitOver, len(results))
	if conCount > 0 {
		fmt.Printf("3. 主流博彩去抽水共识 (Consensus/Shin)：\n   - 胜平负准确率: %.2f%% (%.0f/%.0f)\n   - 平均 Brier Score: %.4f\n",
			conHitWDW/conCount*100, conHitWDW, conCount, conBrier/conCount)
	}
	if optaCount > 0 {
		fmt.Printf("4. Opta 排名差逻辑回归模拟器 (Opta-Sim)：\n   - 胜平负准确率: %.2f%% (%.0f/%.0f)\n   - 平均 Brier Score: %.4f\n",
			optaHitWDW/optaCount*100, optaHitWDW, optaCount, optaBrier/optaCount)
	}
	fmt.Printf("=====================================================================\n")
}

func printBettingSummary() {
	rows, err := db.DB.Query(`
		SELECT plan_type, cost, safe_profit, safe_return, agg_profit, agg_return
		FROM lottery_plans WHERE is_settled = 1
	`)
	if err != nil {
		fmt.Printf("查询投注计划失败: %v\n", err)
		return
	}
	defer rows.Close()

	var singleCount, parlayCount int
	var singleSafeProfit, singleAggProfit float64
	var parlayCost, parlaySafeProfit, parlayAggProfit float64
	var parlayNormalizedSafeProfit, parlayNormalizedAggProfit float64

	for rows.Next() {
		var pType string
		var cost, sProfit, sReturn, aProfit, aReturn float64
		if err := rows.Scan(&pType, &cost, &sProfit, &sReturn, &aProfit, &aReturn); err != nil {
			continue
		}

		if pType == "single" {
			singleCount++
			// 单场成本默认是100
			singleSafeProfit += sProfit
			singleAggProfit += aProfit
		} else if pType == "parlay" {
			parlayCount++
			parlayCost += cost
			parlaySafeProfit += sProfit
			parlayAggProfit += aProfit

			// 百元下注等比例折算
			if cost > 0 {
				parlayNormalizedSafeProfit += (sProfit / cost) * 100
				parlayNormalizedAggProfit += (aProfit / cost) * 100
			}
		}
	}

	fmt.Printf("\n==================== 投资策略收益复盘 (百元模拟) ====================\n")
	if singleCount > 0 {
		singleTotalCost := float64(singleCount) * 100
		fmt.Printf("1. 量化投注单场 (Single) [共 %d 单，总本金 %.0f 元]：\n", singleCount, singleTotalCost)
		fmt.Printf("   - 稳妥策略利润: %.2f 元 | ROI: %.2f%%\n", singleSafeProfit, (singleSafeProfit/singleTotalCost)*100)
		fmt.Printf("   - 激进策略利润: %.2f 元 | ROI: %.2f%%\n", singleAggProfit, (singleAggProfit/singleTotalCost)*100)
	}
	if parlayCount > 0 {
		parlayTotalCost := float64(parlayCount) * 100
		fmt.Printf("2. 混合过关串关 (Parlay) [共 %d 单，归一化总本金 %.0f 元]：\n", parlayCount, parlayTotalCost)
		fmt.Printf("   - 稳妥策略利润: %.2f 元 | ROI: %.2f%%\n", parlayNormalizedSafeProfit, (parlayNormalizedSafeProfit/parlayTotalCost)*100)
		fmt.Printf("   - 激进策略利润: %.2f 元 | ROI: %.2f%%\n", parlayNormalizedAggProfit, (parlayNormalizedAggProfit/parlayTotalCost)*100)
		fmt.Printf("   [实际串关累计原始数据：本金 %.2f 元，稳妥利润 %.2f 元，激进利润 %.2f 元]\n",
			parlayCost, parlaySafeProfit, parlayAggProfit)
	}
	fmt.Printf("=====================================================================\n")
}


func getWDWProbs(matrix []models.ScoreProbability) (float64, float64, float64) {
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
	return w, d, l
}

func buildPredModel(pW, pD, pL, pOver float64, actResult string, actOver bool, hScore, aScore int) PredModel {
	brier := CalcBrierScore(pW, pD, pL, hScore, aScore)
	
	maxP := pW
	predResult := "W"
	if pD > maxP {
		maxP = pD
		predResult = "D"
	}
	if pL > maxP {
		predResult = "L"
	}
	hitWDW := predResult == actResult

	predOver := pOver > 0.5
	hitOver := predOver == actOver

	return PredModel{
		PWin:      math.Round(pW*1000) / 1000,
		PDraw:     math.Round(pD*1000) / 1000,
		PLoss:     math.Round(pL*1000) / 1000,
		POver2_5:  math.Round(pOver*1000) / 1000,
		BrierWDW:  math.Round(brier*1000) / 1000,
		HitWDW:    hitWDW,
		HitOver2_5: hitOver,
	}
}

type TeamStats struct {
	Name      string `json:"name"`
	CnName    string `json:"cnName"`
	Group     string `json:"group"`
	Rank      int    `json:"rank"`
	Elo       float64 `json:"elo"`
	Opponent  string `json:"opponent"`
	ScoreSelf int    `json:"scoreSelf"`
	ScoreOpp  int    `json:"scoreOpp"`
	Result    string `json:"result"` // W / D / L
}

func TestExportTeamProfiles(t *testing.T) {
	dataDir := "../../../../data/db"
	_ = db.Init(dataDir)
	defer db.Close()

	// 1. 获取所有球队的中英文对照和基本数据
	rows, err := db.DB.Query("SELECT en_name, cn_name, initial_elo, fifa_ranking FROM team_translations")
	if err != nil {
		t.Fatalf("查询球队失败: %v", err)
	}
	defer rows.Close()

	teamMap := make(map[string]*TeamStats)
	for rows.Next() {
		var en, cn string
		var elo float64
		var rk int
		if err := rows.Scan(&en, &cn, &elo, &rk); err == nil {
			teamMap[en] = &TeamStats{
				Name:   en,
				CnName: cn,
				Elo:    elo,
				Rank:   rk,
			}
		}
	}

	// 2. 扫描首轮比赛关联结果
	mRows, err := db.DB.Query("SELECT home_team, away_team, match_group, home_score, away_score FROM matches WHERE status='FT'")
	if err != nil {
		t.Fatalf("查询比赛失败: %v", err)
	}
	defer mRows.Close()

	for mRows.Next() {
		var h, a, gp string
		var hs, as int
		if err := mRows.Scan(&h, &a, &gp, &hs, &as); err == nil {
			if th, ok := teamMap[h]; ok {
				th.Group = gp
				th.Opponent = teamMap[a].CnName
				th.ScoreSelf = hs
				th.ScoreOpp = as
				if hs > as {
					th.Result = "W"
				} else if hs == as {
					th.Result = "D"
				} else {
					th.Result = "L"
				}
			}
			if ta, ok := teamMap[a]; ok {
				ta.Group = gp
				ta.Opponent = teamMap[h].CnName
				ta.ScoreSelf = as
				ta.ScoreOpp = hs
				if as > hs {
					ta.Result = "W"
				} else if as == hs {
					ta.Result = "D"
				} else {
					ta.Result = "L"
				}
			}
		}
	}

	// 3. 打印 JSON 格式供我们生成画像
	var list []*TeamStats
	for _, stats := range teamMap {
		if stats.Group != "" { // 只统计参与了第一轮的队伍
			list = append(list, stats)
		}
	}
	jsonData, _ := json.MarshalIndent(list, "", "  ")
	fmt.Printf("\n=== TEAM_STATS_START ===\n%s\n=== TEAM_STATS_END ===\n", string(jsonData))
}

func TestRunOptimizeParameters(t *testing.T) {
	dataDir := "../../../../data/db"
	_ = db.Init(dataDir)
	defer db.Close()

	elo, err := NewEloService("../../../../data/seasons/history_features.json")
	if err != nil {
		t.Fatalf("Elo 失败: %v", err)
	}
	dc := NewDixonColesService(elo, nil)

	bestNormDiv, bestDiffMult, bestH2hMult, bestRho, bestBrier, err := dc.OptimizeParameters()
	if err != nil {
		t.Fatalf("网格搜索优化参数失败: %v", err)
	}

	fmt.Printf("\n==================== 系统定量参数自动纠偏优化 (网格搜索) ====================\n")
	fmt.Printf("- 优化后最佳参数配置：\n")
	fmt.Printf("  * 归一化除数 NormDivulator: %.2f\n", bestNormDiv)
	fmt.Printf("  * 实力差权重 DiffMultiplier: %.2f\n", bestDiffMult)
	fmt.Printf("  * 交锋克制比重 H2hMultiplier: %.2f\n", bestH2hMult)
	fmt.Printf("  * 初始平局相关系数 InitialRho: %.2f\n", bestRho)
	fmt.Printf("- 优化后模型在首轮 24 场比赛上的平均 Brier Score 降低至: %.4f\n", bestBrier)
	fmt.Printf("=============================================================================\n")
}


