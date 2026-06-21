package prediction

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/ai"
	"time"
)

type BacktestService struct {
	eloService    *EloService
	ollamaService *ai.OllamaService
	dcService     *DixonColesService
}

func NewBacktestService(elo *EloService, ollama *ai.OllamaService, dc *DixonColesService) *BacktestService {
	return &BacktestService{
		eloService:    elo,
		ollamaService: ollama,
		dcService:     dc,
	}
}

var backtestSemaphore = make(chan struct{}, 1)

// ReviewMatch 赛后复盘单场预测精度并自适应进化
func (s *BacktestService) ReviewMatch(match models.Match, report *models.PredictionReport) (db.DbBacktestReport, error) {
	// 复盘串行化限流，防止并发把 Ollama 推理队列堵塞
	backtestSemaphore <- struct{}{}
	defer func() { <-backtestSemaphore }()

	// 1. 计算 Brier Score
	var pWin, pDraw, pLoss float64
	for _, cell := range report.ScoreMatrix {
		if cell.HomeScore > cell.AwayScore {
			pWin += cell.Prob
		} else if cell.HomeScore == cell.AwayScore {
			pDraw += cell.Prob
		} else {
			pLoss += cell.Prob
		}
	}

	var oWin, oDraw, oLoss float64
	if match.HomeScore > match.AwayScore {
		oWin = 1.0
	} else if match.HomeScore == match.AwayScore {
		oDraw = 1.0
	} else {
		oLoss = 1.0
	}

	brierScore := (pWin-oWin)*(pWin-oWin) + (pDraw-oDraw)*(pDraw-oDraw) + (pLoss-oLoss)*(pLoss-oLoss)

	// 2. 在线修正两队 Elo 基础评级
	oldEloHome := s.eloService.GetElo(match.HomeTeam)
	oldEloAway := s.eloService.GetElo(match.AwayTeam)

	s.eloService.UpdateElos(match.HomeTeam, match.AwayTeam, match.HomeScore, match.AwayScore)

	newEloHome := s.eloService.GetElo(match.HomeTeam)
	newEloAway := s.eloService.GetElo(match.AwayTeam)

	homeEloDiff := newEloHome - oldEloHome
	awayEloDiff := newEloAway - oldEloAway

	// 3. 触发 Ollama 赛后定性分析报告
	reviewText, err := s.ollamaService.ReviewPrediction(match, brierScore, report.TacticsAnalysis, match.HomeScore, match.AwayScore)
	if err != nil {
		reviewText = "量化反思: Ollama 解析异常，模型已利用定量 Brier Score 完成 Elo 自校准修正。"
	}

	// 4. 持久化到数据库
	dbReport := db.DbBacktestReport{
		MatchID:       match.ID,
		BrierScore:    brierScore,
		HomeEloDiff:   homeEloDiff,
		AwayEloDiff:   awayEloDiff,
		TacticsReview: reviewText,
		ReviewedAt:    time.Now(),
	}

	if err := db.SaveBacktestReport(dbReport); err != nil {
		return dbReport, err
	}

	// 同时也持久化保存本场预测分析的完整报告 (Dixon-Coles参数、矩阵概率、定性分析等)
	_ = db.SavePredictionReport(*report)

	// 触发自适应 Dixon-Coles 参数的平局修正系数与进球期望偏差自适应进化
	if s.dcService != nil {
		s.dcService.RecalculateRhoOffset()
		s.dcService.RecalculateLambdaOffset()
	}

	return dbReport, nil
}

// RebuildAllFinishedMatchesBacktest 重置状态并在时序上重新跑所有已完赛比赛的回测，返回计算前后的 Brier Score
func (s *BacktestService) RebuildAllFinishedMatchesBacktest() (float64, float64, error) {
	// 1. 获取重置前的平均 Brier Score
	var oldBrier float64
	reports, err := db.GetBacktestReports()
	if err == nil && len(reports) > 0 {
		for _, r := range reports {
			oldBrier += r.BrierScore
		}
		oldBrier /= float64(len(reports))
	}

	// 2. 清空数据库中的旧预测和回测报告数据
	_, _ = db.DB.Exec("DELETE FROM prediction_reports;")
	_, _ = db.DB.Exec("DELETE FROM backtest_reports;")

	// 3. 重置内存中的 Elo 评级为冷启动初始值
	s.eloService.ResetToInitialElos()

	// 4. 重置 Dixon-Coles 的自适应偏移
	if s.dcService != nil {
		s.dcService.rhoOffset = 0.0
		s.dcService.lambdaHomeOffset = 0.0
		s.dcService.lambdaAwayOffset = 0.0
	}

	// 5. 按 ScheduledAt 升序排列查询所有已完赛比赛 (避免时序颠倒)
	rows, err := db.DB.Query(`SELECT id, tournament_id, home_team, away_team, match_group, scheduled_at, status, home_score, away_score, venue 
		FROM matches 
		WHERE status = 'FT' 
		ORDER BY scheduled_at ASC`)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var schedStr string
		err := rows.Scan(&m.ID, &m.TournamentID, &m.HomeTeam, &m.AwayTeam, &m.Group, &schedStr, &m.Status, &m.HomeScore, &m.AwayScore, &m.Venue)
		if err == nil {
			m.ScheduledAt, _ = time.Parse(time.RFC3339, schedStr)
			if m.ScheduledAt.IsZero() {
				m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05", schedStr)
			}
			matches = append(matches, m)
		}
	}

	// 6. 顺次进行时序重演推理与复盘
	for _, m := range matches {
		params := s.dcService.CalculateParamsWithVenue(m.HomeTeam, m.AwayTeam, m.Venue, m.ScheduledAt)
		matrix, over25, under25 := s.dcService.GenerateProbabilityMatrixWithTeams(params, m.HomeTeam, m.AwayTeam)

		report := models.PredictionReport{
			MatchID:              m.ID,
			OriginalParams:       params,
			RefinedParams:        params,
			ScoreMatrix:          matrix,
			Over2_5Prob:          over25,
			Under2_5Prob:         under25,
			OriginalScoreMatrix:  matrix,
			OriginalOver2_5Prob:  over25,
			OriginalUnder2_5Prob: under25,
		}

		// 触发赛后分析与自校准
		_, _ = s.ReviewMatch(m, &report)
	}

	// 7. 重演完毕后自动运行网格搜索进行参数的最优调优
	var newBrier float64
	if s.dcService != nil {
		_, _, _, _, bs, errOpt := s.dcService.OptimizeParameters()
		if errOpt == nil {
			newBrier = bs
		}
	}

	if newBrier == 0 {
		newReports, _ := db.GetBacktestReports()
		if len(newReports) > 0 {
			for _, r := range newReports {
				newBrier += r.BrierScore
			}
			newBrier /= float64(len(newReports))
		}
	}

	return oldBrier, newBrier, nil
}
