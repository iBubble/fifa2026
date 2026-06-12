package models

import "time"

// ValueBet 代表一个具有正期望价值的投资注单
type ValueBet struct {
	MatchID    string  `json:"matchId"`
	HomeTeam   string  `json:"homeTeam"`
	AwayTeam   string  `json:"awayTeam"`
	Market     string  `json:"market"`     // 盘口: "1X2", "OverUnder2.5", "DNB"
	Outcome    string  `json:"outcome"`    // 投注项: "Home", "Away", "Draw", "Over 2.5", "Under 2.5"
	SystemProb float64 `json:"systemProb"` // 模型测算概率
	ConsensusP float64 `json:"consensusP"` // 博彩市场 Shin 氏去抽水共识胜率
	Odds       float64 `json:"odds"`       // 庄家开盘赔率
	EV         float64 `json:"ev"`         // 期望价值 (p * odds - 1)
	KellyStake float64 `json:"kellyStake"` // 建议投注金额占比 (基于总本金比例)
}

// KellyParams 凯利公式输入参数
type KellyParams struct {
	WinProb  float64 `json:"winProb"`  // 系统评估胜率
	Odds     float64 `json:"odds"`     // 平台小数赔率
	Bankroll float64 `json:"bankroll"` // 总可用投注本金
	Fraction float64 `json:"fraction"` // 凯利比例收缩系数: 1.0 (全凯利), 0.5 (半凯利), 0.25 (1/4凯利)
}

// KellyResult 凯利资金分配计算结果
type KellyResult struct {
	RawFraction      float64 `json:"rawFraction"`      // 原始凯利值
	AdjustedFraction float64 `json:"adjustedFraction"` // 偏置收缩后比例
	SuggestedStake   float64 `json:"suggestedStake"`   // 最终推荐下注金额
	ExpectedValue    float64 `json:"expectedValue"`    // EV
}

// ArbitrageLeg 套利单腿明细
type ArbitrageLeg struct {
	Bookmaker string  `json:"bookmaker"`
	Outcome   string  `json:"outcome"`
	Odds      float64 `json:"odds"`
	StakeAmt  float64 `json:"stakeAmt"` // 此平台需下注本金
}

// ArbitrageOpportunity 扫描到的无风险套利空间
type ArbitrageOpportunity struct {
	MatchID    string         `json:"matchId"`
	HomeTeam   string         `json:"homeTeam"`
	AwayTeam   string         `json:"awayTeam"`
	Market     string         `json:"market"`
	LValue     float64        `json:"lValue"` // 套利系数 L = ∑(1/odds)，L < 1 代表存在套利
	ROI        float64        `json:"roi"`    // 预期无风险收益率
	Legs       []ArbitrageLeg `json:"legs"`
	DetectedAt time.Time      `json:"detectedAt"`
}

// Bet 投注历史账本
type Bet struct {
	ID            int64     `json:"id"`
	TournamentID  string    `json:"tournamentId"` // 多赛季隔离
	MatchID       string    `json:"matchId"`
	HomeTeam      string    `json:"homeTeam"`
	AwayTeam      string    `json:"awayTeam"`
	Bookmaker     string    `json:"bookmaker"`
	Market        string    `json:"market"`
	Outcome       string    `json:"outcome"`
	Odds          float64   `json:"odds"`
	Stake         float64   `json:"stake"`
	Result        string    `json:"result"` // "PENDING", "WIN", "LOSS", "VOID", "HALF_WIN", "HALF_LOSS"
	PnL           float64   `json:"pnl"`    // 实际盈亏
	KellyFraction float64   `json:"kellyFraction"`
	ConsensusProb float64   `json:"consensusProb"` // 投注时的市场共识概率
	ExpectedValue float64   `json:"expectedValue"` // 投注时的期望价值
	PlacedAt      time.Time `json:"placedAt"`
}

// BetSummary 投注账本可视化统计汇总
type BetSummary struct {
	TotalBets   int     `json:"totalBets"`
	WinCount    int     `json:"winCount"`
	LossCount   int     `json:"lossCount"`
	WinRate     float64 `json:"winRate"`
	TotalStake  float64 `json:"totalStake"`
	TotalPnL    float64 `json:"totalPnl"`
	ROI         float64 `json:"roi"`
	MaxDrawdown float64 `json:"maxDrawdown"`
}

// OddsRecord 赔率快照记录
type OddsRecord struct {
	ID         int64     `json:"id"`
	MatchID    string    `json:"matchId"`
	Bookmaker  string    `json:"bookmaker"`
	HomeOdds   float64   `json:"homeOdds"`
	DrawOdds   float64   `json:"drawOdds"`
	AwayOdds   float64   `json:"awayOdds"`
	CapturedAt time.Time `json:"capturedAt"`
}

// LotteryPlan 代表已保存的投注精算方案
type LotteryPlan struct {
	ID                 int64     `json:"id"`
	PlanType           string    `json:"planType"` // "single" 或 "parlay"
	MatchIDs           string    `json:"matchIds"` // 逗号分隔的比赛ID
	RiskLevel          string    `json:"riskLevel,omitempty"`
	OddsH              float64   `json:"oddsH,omitempty"`
	OddsD              float64   `json:"oddsD,omitempty"`
	OddsA              float64   `json:"oddsA,omitempty"`
	PrimaryBet         string    `json:"primaryBet,omitempty"`
	PrimaryOdds        float64   `json:"primaryOdds,omitempty"`
	PrimaryAmt         float64   `json:"primaryAmt,omitempty"`
	HedgeBet           string    `json:"hedgeBet,omitempty"`
	HedgeOdds          float64   `json:"hedgeOdds,omitempty"`
	HedgeAmt           float64   `json:"hedgeAmt,omitempty"`
	ParlayType         string    `json:"parlayType,omitempty"`
	ParlayMode         string    `json:"parlayMode,omitempty"`
	ParlayOptions      string    `json:"parlayOptions,omitempty"`
	DescStr            string    `json:"descStr"`
	WinsCount          int       `json:"winsCount,omitempty"`
	Cost               float64   `json:"cost,omitempty"`
	SingleTicketPayout float64   `json:"singleTicketPayout,omitempty"`
	ComboOdds          float64   `json:"comboOdds,omitempty"`
	ComboProb          float64   `json:"comboProb,omitempty"`
	TotalEV            float64   `json:"totalEv,omitempty"`
	KellyStake         float64   `json:"kellyStake,omitempty"`
	TicketsJSON        string    `json:"ticketsJson,omitempty"`
	IsSettled          int       `json:"isSettled"` // 0: 待结算, 1: 已结算
	SafeProfit         float64   `json:"safeProfit"`
	SafeReturn         float64   `json:"safeReturn"`
	AggProfit          float64   `json:"aggProfit"`
	AggReturn          float64   `json:"aggReturn"`
	CreatedAt          time.Time `json:"createdAt"`
}

