package v1

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// PredictMatch 接口，提供 Dixon-Coles 模型与 Ollama AI 参数纠偏混合预测
func (ctrl *APIController) PredictMatch(c *gin.Context) {
	var req struct {
		MatchID string `json:"matchId"`
		Info    string `json:"info"`
		UseLLM  bool   `json:"useLLM"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	match, err := db.GetMatch(req.MatchID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到指定比赛"})
		return
	}

	params := ctrl.DCService.CalculateParamsWithVenue(match.HomeTeam, match.AwayTeam, match.Venue, match.ScheduledAt)
	refined := params
	llmRefined := false
	var tactics, poster string
	var proponentOpinion, critiqueAnalysis, consensusReason string

	if req.UseLLM {
		log.Printf("[Predict Debug] 🔍 开始大模型纠偏, MatchID=%s", req.MatchID)
		feedbackStr := ""
		reps, errReps := db.GetBacktestReports()
		if errReps == nil && len(reps) > 0 {
			limit := 3
			if len(reps) < limit {
				limit = len(reps)
			}
			var sbFeedback strings.Builder
			sbFeedback.WriteString("【最近完赛模型预测误差校准反馈】:\n")
			for idx := len(reps) - 1; idx >= len(reps)-limit; idx-- {
				r := reps[idx]
				mInfo, errM := db.GetMatch(r.MatchID)
				if errM == nil {
					sbFeedback.WriteString(fmt.Sprintf("- 比赛: %s vs %s, 赛果: %d:%d, 预测 Brier Score: %.4f, 反思: %s\n",
						mInfo.HomeTeam, mInfo.AwayTeam, mInfo.HomeScore, mInfo.AwayScore, r.BrierScore, r.TacticsReview))
				}
			}
			feedbackStr = sbFeedback.String()
		}

		newsSummary := ctrl.NewsService.BuildPredictInfoForMatch(match.HomeTeam, match.AwayTeam)
		weatherSummary := ctrl.WeatherService.BuildWeatherSummary(match.HomeTeam, match.AwayTeam, match.Venue, match.ScheduledAt)
		qualitativeInfo := fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", req.Info, newsSummary, weatherSummary, feedbackStr)

		diff := ctrl.EloService.GetElo(match.HomeTeam) - ctrl.EloService.GetElo(match.AwayTeam)
		log.Printf("[Predict Debug] 🚀 准备调用 RefineParams 进行参数微调...")
		offsets, err := ctrl.OllamaService.RefineParams(match, diff, params, qualitativeInfo)
		log.Printf("[Predict Debug] 📥 RefineParams 调用返回, err=%v", err)
		if err == nil {
			// 名气惩罚因子 (Reputation Decay Factor) 优化机制
			if offsets.LambdaHomeOffset > 0 {
				feat := ctrl.EloService.GetFeature(match.HomeTeam)
				if feat.InitialElo > 1650 || feat.Titles > 0 {
					var ftCount, winCount int
					_ = db.DB.QueryRow("SELECT COUNT(*), SUM(CASE WHEN (home_team = ? AND home_score > away_score) OR (away_team = ? AND away_score > home_score) THEN 1 ELSE 0 END) FROM matches WHERE status = 'FT' AND (home_team = ? OR away_team = ?)",
						match.HomeTeam, match.HomeTeam, match.HomeTeam, match.HomeTeam).Scan(&ftCount, &winCount)
					if ftCount > 0 && winCount == 0 {
						offsets.LambdaHomeOffset *= 0.70
					}
				}
			}
			if offsets.LambdaAwayOffset > 0 {
				feat := ctrl.EloService.GetFeature(match.AwayTeam)
				if feat.InitialElo > 1650 || feat.Titles > 0 {
					var ftCount, winCount int
					_ = db.DB.QueryRow("SELECT COUNT(*), SUM(CASE WHEN (home_team = ? AND home_score > away_score) OR (away_team = ? AND away_score > home_score) THEN 1 ELSE 0 END) FROM matches WHERE status = 'FT' AND (home_team = ? OR away_team = ?)",
						match.AwayTeam, match.AwayTeam, match.AwayTeam, match.AwayTeam).Scan(&ftCount, &winCount)
					if ftCount > 0 && winCount == 0 {
						offsets.LambdaAwayOffset *= 0.70
					}
				}
			}

			refined.LambdaHome = params.LambdaHome + offsets.LambdaHomeOffset
			refined.LambdaAway = params.LambdaAway + offsets.LambdaAwayOffset
			refined.Rho = params.Rho + offsets.RhoOffset
			llmRefined = true
			tactics = offsets.TacticsAnalysis
			poster = offsets.PosterPrompt
			proponentOpinion = offsets.ProponentOpinion
			critiqueAnalysis = offsets.CritiqueAnalysis
			consensusReason = offsets.ConsensusReason
		} else {
			log.Printf("[Predict] ⚠️ Ollama 大模型微调参数降级: %v", err)
		}
	}

	matrix, over25, under25 := ctrl.DCService.GenerateProbabilityMatrixWithTeams(refined, match.HomeTeam, match.AwayTeam)
	odds := ctrl.SportteryService.GetMatchOdds(match.HomeTeam, match.AwayTeam, match.ScheduledAt)
	homeRank := ctrl.EloService.GetEloRank(match.HomeTeam)
	awayRank := ctrl.EloService.GetEloRank(match.AwayTeam)

	var h2hRecord *models.H2HRecord
	if ctrl.APISportsService != nil {
		h2h, err := ctrl.APISportsService.GetH2HRecord(match.HomeTeam, match.AwayTeam)
		if err == nil {
			h2hRecord = &h2h
		}
	}

	// 辛氏概率去水校准与平滑融合
	matrix, over25, under25 = ctrl.calibrateMatrixWithOdds(matrix, odds)

	origMatrix, origOver25, origUnder25 := ctrl.DCService.GenerateProbabilityMatrixWithTeams(params, match.HomeTeam, match.AwayTeam)

	report := models.PredictionReport{
		MatchID:              req.MatchID,
		OriginalParams:       params,
		RefinedParams:        refined,
		LLMRefined:           llmRefined,
		ScoreMatrix:          matrix,
		Over2_5Prob:          over25,
		Under2_5Prob:         under25,
		TacticsAnalysis:      tactics,
		PosterPrompt:         poster,
		ProponentOpinion:     proponentOpinion,
		CritiqueAnalysis:     critiqueAnalysis,
		ConsensusReason:      consensusReason,
		OriginalScoreMatrix:  origMatrix,
		OriginalOver2_5Prob:  origOver25,
		OriginalUnder2_5Prob: origUnder25,
		H2H:                  h2hRecord,
		HomeRank:             homeRank,
		AwayRank:             awayRank,
	}
	_ = db.SavePredictionReport(report)

	c.JSON(http.StatusOK, report)
}
