package prediction

import (
	"fifa2026/src/internal/models"
	"time"
)

type ArbitrageService struct{}

func NewArbitrageService() *ArbitrageService {
	return &ArbitrageService{}
}

// ScanArbitrage 扫描单场比赛多平台的胜平负赔率快照，识别无风险套利机会
func (s *ArbitrageService) ScanArbitrage(match models.Match, records []models.OddsRecord, totalBankroll float64) (models.ArbitrageOpportunity, bool) {
	if len(records) < 2 {
		return models.ArbitrageOpportunity{}, false
	}

	// 找出各大盘口的最优赔率
	bestHome := 0.0
	bestHomeBook := ""
	bestDraw := 0.0
	bestDrawBook := ""
	bestAway := 0.0
	bestAwayBook := ""

	for _, r := range records {
		if r.HomeOdds > bestHome {
			bestHome = r.HomeOdds
			bestHomeBook = r.Bookmaker
		}
		if r.DrawOdds > bestDraw {
			bestDraw = r.DrawOdds
			bestDrawBook = r.Bookmaker
		}
		if r.AwayOdds > bestAway {
			bestAway = r.AwayOdds
			bestAwayBook = r.Bookmaker
		}
	}

	if bestHome <= 1.0 || bestDraw <= 1.0 || bestAway <= 1.0 {
		return models.ArbitrageOpportunity{}, false
	}

	// 计算套利系数 L
	lValue := (1.0 / bestHome) + (1.0 / bestDraw) + (1.0 / bestAway)

	// 当且仅当 L < 1.0 时存在套利空间
	if lValue >= 1.0 {
		return models.ArbitrageOpportunity{
			MatchID:  match.ID,
			HomeTeam: match.HomeTeam,
			AwayTeam: match.AwayTeam,
			Market:   "1X2",
			LValue:   lValue,
			ROI:      0.0,
		}, false
	}

	// 计算无风险收益率 ROI
	roi := (1.0/lValue - 1.0) * 100.0

	// 资金配比对冲计算 (各平台应投注金额)
	stakeHome := (totalBankroll / bestHome) / lValue
	stakeDraw := (totalBankroll / bestDraw) / lValue
	stakeAway := (totalBankroll / bestAway) / lValue

	opp := models.ArbitrageOpportunity{
		MatchID:  match.ID,
		HomeTeam: match.HomeTeam,
		AwayTeam: match.AwayTeam,
		Market:   "1X2",
		LValue:   lValue,
		ROI:      roi,
		Legs: []models.ArbitrageLeg{
			{Bookmaker: bestHomeBook, Outcome: "Home", Odds: bestHome, StakeAmt: stakeHome},
			{Bookmaker: bestDrawBook, Outcome: "Draw", Odds: bestDraw, StakeAmt: stakeDraw},
			{Bookmaker: bestAwayBook, Outcome: "Away", Odds: bestAway, StakeAmt: stakeAway},
		},
		DetectedAt: time.Now(),
	}

	return opp, true
}
