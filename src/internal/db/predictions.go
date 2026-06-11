package db

import (
	"database/sql"
	"encoding/json"
	"fifa2026/src/internal/models"
)

// SavePredictionReport 插入或更新单场分析预测报告
func SavePredictionReport(r models.PredictionReport) error {
	matrixJSON, err := json.Marshal(r.ScoreMatrix)
	if err != nil {
		return err
	}

	query := `INSERT INTO prediction_reports (
			match_id, lambda_home, lambda_away, rho,
			original_lambda_home, original_lambda_away, original_rho,
			over_2_5_prob, under_2_5_prob, tactics_analysis, poster_prompt, score_matrix_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(match_id) DO UPDATE SET
			lambda_home = excluded.lambda_home,
			lambda_away = excluded.lambda_away,
			rho = excluded.rho,
			original_lambda_home = excluded.original_lambda_home,
			original_lambda_away = excluded.original_lambda_away,
			original_rho = excluded.original_rho,
			over_2_5_prob = excluded.over_2_5_prob,
			under_2_5_prob = excluded.under_2_5_prob,
			tactics_analysis = excluded.tactics_analysis,
			poster_prompt = excluded.poster_prompt,
			score_matrix_json = excluded.score_matrix_json`
	_, err = DB.Exec(query,
		r.MatchID,
		r.RefinedParams.LambdaHome,
		r.RefinedParams.LambdaAway,
		r.RefinedParams.Rho,
		r.OriginalParams.LambdaHome,
		r.OriginalParams.LambdaAway,
		r.OriginalParams.Rho,
		r.Over2_5Prob,
		r.Under2_5Prob,
		r.TacticsAnalysis,
		r.PosterPrompt,
		string(matrixJSON),
	)
	return err
}

// GetPredictionReport 获取单场比赛的分析预测报告
func GetPredictionReport(matchID string) (models.PredictionReport, error) {
	query := `SELECT 
			match_id, lambda_home, lambda_away, rho,
			original_lambda_home, original_lambda_away, original_rho,
			over_2_5_prob, under_2_5_prob, tactics_analysis, poster_prompt, score_matrix_json
		FROM prediction_reports WHERE match_id = ?`
	row := DB.QueryRow(query, matchID)

	var r models.PredictionReport
	var matrixStr string
	err := row.Scan(
		&r.MatchID,
		&r.RefinedParams.LambdaHome,
		&r.RefinedParams.LambdaAway,
		&r.RefinedParams.Rho,
		&r.OriginalParams.LambdaHome,
		&r.OriginalParams.LambdaAway,
		&r.OriginalParams.Rho,
		&r.Over2_5Prob,
		&r.Under2_5Prob,
		&r.TacticsAnalysis,
		&r.PosterPrompt,
		&matrixStr,
	)
	if err == sql.ErrNoRows {
		return models.PredictionReport{}, err
	} else if err != nil {
		return models.PredictionReport{}, err
	}

	r.LLMRefined = r.RefinedParams.LambdaHome != r.OriginalParams.LambdaHome || r.RefinedParams.LambdaAway != r.OriginalParams.LambdaAway

	var scoreMatrix []models.ScoreProbability
	if err := json.Unmarshal([]byte(matrixStr), &scoreMatrix); err != nil {
		return models.PredictionReport{}, err
	}
	r.ScoreMatrix = scoreMatrix

	return r, nil
}
