package prediction

import (
	"fifa2026/src/internal/db"
	"math"
)

// CalcBrierScore 计算三项概率预测的 Brier Score
func CalcBrierScore(pWin, pDraw, pLoss float64, homeScore, awayScore int) float64 {
	var oWin, oDraw, oLoss float64
	if homeScore > awayScore {
		oWin = 1.0
	} else if homeScore == awayScore {
		oDraw = 1.0
	} else {
		oLoss = 1.0
	}
	return (pWin-oWin)*(pWin-oWin) + (pDraw-oDraw)*(pDraw-oDraw) + (pLoss-oLoss)*(pLoss-oLoss)
}

// GetOptaSimulatedProbs 根据两队 FIFA 排名差，利用逻辑回归映射模拟 Opta 的胜平负概率
func GetOptaSimulatedProbs(homeTeam, awayTeam string) (float64, float64, float64) {
	rankHome := 50 // 默认值
	rankAway := 50

	tHome, errH := db.GetTeamTranslation(homeTeam)
	if errH == nil && tHome.FifaRanking > 0 {
		rankHome = tHome.FifaRanking
	}
	tAway, errA := db.GetTeamTranslation(awayTeam)
	if errA == nil && tAway.FifaRanking > 0 {
		rankAway = tAway.FifaRanking
	}

	// 排名越小实力越强，所以 rankAway - rankHome 为正代表主队排名更靠前（更强）
	diff := float64(rankAway - rankHome)

	// 排名差逻辑回归公式
	pWinRaw := math.Exp(0.02 * diff)
	pLossRaw := math.Exp(-0.02 * diff)
	pDrawRaw := 0.60

	total := pWinRaw + pDrawRaw + pLossRaw
	return pWinRaw / total, pDrawRaw / total, pLossRaw / total
}

// GetConsensusProbs 从 odds_history 提取该比赛的最新赔率并执行 Shin 氏去抽水，还原真实共识概率
func GetConsensusProbs(matchID string, shin *ShinService) (float64, float64, float64, bool) {
	query := `SELECT home_odds, draw_odds, away_odds FROM odds_history 
		WHERE match_id = ? ORDER BY captured_at DESC LIMIT 1`
	row := db.DB.QueryRow(query, matchID)
	var h, d, a float64
	err := row.Scan(&h, &d, &a)
	if err != nil {
		return 0, 0, 0, false
	}

	probs, _, errShin := shin.DevigOdds([]float64{h, d, a})
	if errShin != nil || len(probs) < 3 {
		return 1.0 / h, 1.0 / d, 1.0 / a, true // 退化为带抽水的原始隐含概率
	}
	return probs[0], probs[1], probs[2], true
}
