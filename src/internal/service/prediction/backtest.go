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

// ReviewMatch 赛后复盘单场预测精度并自适应进化
func (s *BacktestService) ReviewMatch(match models.Match, report *models.PredictionReport) (db.DbBacktestReport, error) {
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

	// 触发自适应 Dixon-Coles 参数的平局修正系数进化
	if s.dcService != nil {
		s.dcService.RecalculateRhoOffset()
	}

	return dbReport, nil
}
