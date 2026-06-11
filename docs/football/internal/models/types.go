// Package models 定义所有请求、响应及数据库表的 Go Struct
// JSON 标签全部使用驼峰命名（camelCase），与前端 TypeScript 类型保持一致
package models

import "time"

// ─────────────────────────────────────────────────────────────
// 联赛相关
// ─────────────────────────────────────────────────────────────

// LeagueType 联赛类型枚举
type LeagueType string

const (
	LeagueDomestic LeagueType = "domestic" // 联赛制（英超、西甲等，有积分榜）
	LeagueCup      LeagueType = "cup"      // 杯赛/锦标赛制（世界杯、欧冠等，有分组+淘汰赛）
)

// League 联赛元数据（内置注册表条目）
type League struct {
	SportKey       string     `json:"sportKey"`       // The Odds API sport key，如 "soccer_epl"
	Name           string     `json:"name"`           // 联赛中文名，如 "英超"
	FullName       string     `json:"fullName"`       // 联赛全名，如 "英格兰足球超级联赛"
	Country        string     `json:"country"`        // 国家/地区
	Emoji          string     `json:"emoji"`          // 联赛图标 emoji
	APIFootballID  int        `json:"apiFootballId"`  // API-Football 中的联赛 ID
	Season         int        `json:"season"`         // 当前赛季年份
	Type           LeagueType `json:"type"`           // 联赛类型：domestic / cup
}

// ─────────────────────────────────────────────────────────────
// 比赛相关
// ─────────────────────────────────────────────────────────────

// Match 代表一场足球比赛的基础信息（来自 API-Football 或本地数据库）
type Match struct {
	ID          string    `json:"id"`
	HomeTeam    string    `json:"homeTeam"`
	AwayTeam    string    `json:"awayTeam"`
	League      string    `json:"league"`
	Country     string    `json:"country"`
	ScheduledAt time.Time `json:"scheduledAt"`
	Status      string    `json:"status"`  // "NS"=未开赛 "1H"=上半场 "HT"=中场 "2H"=下半场 "FT"=结束
	HomeScore   int       `json:"homeScore"`
	AwayScore   int       `json:"awayScore"`
	Minute      int       `json:"minute"` // 当前比赛时间（分钟）
}

// LiveScore 实时比分推送（通过 Wails 事件发送，高频更新）
type LiveScore struct {
	MatchID   string `json:"matchId"`
	HomeScore int    `json:"homeScore"`
	AwayScore int    `json:"awayScore"`
	Minute    int    `json:"minute"`
	Status    string `json:"status"`
}

// XGDataPoint xG（期望进球）时间序列中的单个数据点
type XGDataPoint struct {
	Minute  int     `json:"minute"`
	HomeXG  float64 `json:"homeXG"`
	AwayXG  float64 `json:"awayXG"`
	// 累积 xG（用于 ECharts 折线图）
	HomeCumXG float64 `json:"homeCumXG"`
	AwayCumXG float64 `json:"awayCumXG"`
}

// XGUpdate 包含某场比赛的完整 xG 时间序列（事件推送载荷）
type XGUpdate struct {
	MatchID string        `json:"matchId"`
	Points  []XGDataPoint `json:"points"`
}

// ─────────────────────────────────────────────────────────────
// 赔率相关
// ─────────────────────────────────────────────────────────────

// MarketType 盘口类型枚举
type MarketType string

const (
	Market1X2       MarketType = "h2h"         // 胜平负
	MarketHandicap  MarketType = "spreads"      // 让球盘
	MarketOverUnder MarketType = "totals"       // 大小球
)

// OddsOutcome 单个赔率结果（某平台某盘口某选项的赔率）
type OddsOutcome struct {
	Name  string  `json:"name"`  // "Home" / "Draw" / "Away" / "Over 2.5" 等
	Price float64 `json:"price"` // 欧洲赔率（小数格式，如 2.50）
}

// BookmakerOdds 某博彩平台对某场比赛某盘口的报价
type BookmakerOdds struct {
	Bookmaker string        `json:"bookmaker"` // "Bet365" / "Pinnacle" / "William Hill"
	Market    MarketType    `json:"market"`
	Outcomes  []OddsOutcome `json:"outcomes"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

// OddsSnapshot 某场比赛在某时刻的全网赔率快照（Wails 事件推送载荷）
type OddsSnapshot struct {
	MatchID    string          `json:"matchId"`
	Match      Match           `json:"match"`
	Bookmakers []BookmakerOdds `json:"bookmakers"`
	CapturedAt time.Time       `json:"capturedAt"`
}

// ─────────────────────────────────────────────────────────────
// 套利相关
// ─────────────────────────────────────────────────────────────

// ArbLeg 套利组合中的单腿（在某平台投注某选项）
type ArbLeg struct {
	Bookmaker string  `json:"bookmaker"`
	Outcome   string  `json:"outcome"`
	Odds      float64 `json:"odds"`
	// 建议投注比例（资金分配百分比）
	StakePct  float64 `json:"stakePct"`
	// 建议投注金额（基于 bankroll 计算）
	StakeAmt  float64 `json:"stakeAmt"`
}

// ArbitrageOpportunity 一次套利机会（Wails 事件推送载荷）
// L = Σ(1/odds)，L < 1 表示无风险套利存在
type ArbitrageOpportunity struct {
	MatchID   string    `json:"matchId"`
	Match     Match     `json:"match"`
	Market    MarketType `json:"market"`
	LValue    float64   `json:"lValue"`  // 套利系数，越小机会越好
	ROI       float64   `json:"roi"`     // 预期收益率，= (1/L - 1) * 100%
	Legs      []ArbLeg  `json:"legs"`    // 各腿详情
	DetectedAt time.Time `json:"detectedAt"`
}

// ─────────────────────────────────────────────────────────────
// 凯利公式相关
// ─────────────────────────────────────────────────────────────

// PredictionWeights AI 预测时各维度权重（前端 Slider 沙盒控制）
type PredictionWeights struct {
	FormWeight     float64 `json:"formWeight"`     // 近期状态权重 0~1
	H2HWeight      float64 `json:"h2hWeight"`      // 历史交锋权重 0~1
	OddsWeight     float64 `json:"oddsWeight"`     // 赔率隐含概率权重 0~1
	XGWeight       float64 `json:"xgWeight"`       // xG 数据权重 0~1
	InjuryWeight   float64 `json:"injuryWeight"`   // 伤病/阵容权重 0~1
}

// KellyParams 凯利公式前端传入参数
type KellyParams struct {
	Odds      float64           `json:"odds"`      // 博彩平台赔率（小数格式）
	WinProb   float64           `json:"winProb"`   // 估计胜率（0~1）
	Bankroll  float64           `json:"bankroll"`  // 当前总资金
	Fraction  float64           `json:"fraction"`  // 凯利分数（通常 0.25~0.5，避免过激）
	Weights   PredictionWeights `json:"weights"`
}

// KellyResult 凯利公式计算结果
type KellyResult struct {
	// f* = (b*p - q) / b，其中 b = odds-1, p = winProb, q = 1-winProb
	KellyFraction    float64 `json:"kellyFraction"`    // 原始凯利分数
	AdjustedFraction float64 `json:"adjustedFraction"` // 调整后（乘以 fraction 参数）
	SuggestedStake   float64 `json:"suggestedStake"`   // 建议投注金额
	ExpectedValue    float64 `json:"expectedValue"`    // 期望值 = p*odds - 1
	EdgePct          float64 `json:"edgePct"`          // 赔率边际 %
}

// ─────────────────────────────────────────────────────────────
// 账本相关
// ─────────────────────────────────────────────────────────────

// BetResult 投注结果枚举
type BetResult string

const (
	BetWin    BetResult = "WIN"
	BetLoss   BetResult = "LOSS"
	BetDraw   BetResult = "DRAW"
	BetVoid   BetResult = "VOID"
	BetPending BetResult = "PENDING"
)

// Bet 单条投注记录（账本条目）
type Bet struct {
	ID            int64     `json:"id"`
	MatchID       string    `json:"matchId"`
	HomeTeam      string    `json:"homeTeam"`
	AwayTeam      string    `json:"awayTeam"`
	Bookmaker     string    `json:"bookmaker"`
	Market        string    `json:"market"`
	Outcome       string    `json:"outcome"`
	Odds          float64   `json:"odds"`
	Stake         float64   `json:"stake"`
	Result        BetResult `json:"result"`
	PnL           float64   `json:"pnl"`           // 盈亏金额
	KellyFraction float64   `json:"kellyFraction"` // 下注时的凯利分数
	PlacedAt      time.Time `json:"placedAt"`
	SettledAt     *time.Time `json:"settledAt,omitempty"`
	Notes         string    `json:"notes"`
}

// BetSummary 账本汇总统计（Reports 页面展示）
type BetSummary struct {
	TotalBets    int     `json:"totalBets"`
	WinCount     int     `json:"winCount"`
	LossCount    int     `json:"lossCount"`
	WinRate      float64 `json:"winRate"`
	TotalStake   float64 `json:"totalStake"`
	TotalPnL     float64 `json:"totalPnl"`
	ROI          float64 `json:"roi"`
	AvgOdds      float64 `json:"avgOdds"`
	MaxDrawdown  float64 `json:"maxDrawdown"`
}

// ─────────────────────────────────────────────────────────────
// 日志相关（Terminal 页面）
// ─────────────────────────────────────────────────────────────

// LogLevel 日志级别
type LogLevel string

const (
	LogDebug   LogLevel = "DEBUG"
	LogInfo    LogLevel = "INFO"
	LogWarning LogLevel = "WARN"
	LogError   LogLevel = "ERROR"
	LogSuccess LogLevel = "SUCCESS"
)

// LogEntry 单条后端日志（通过 Wails 事件实时推送到 Terminal 页面）
type LogEntry struct {
	Level     LogLevel  `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`    // "OddsService" / "MatchService" 等
	Timestamp time.Time `json:"timestamp"`
}

// ─────────────────────────────────────────────────────────────
// API 响应结构体（用于解析外部 API 响应）
// ─────────────────────────────────────────────────────────────

// TheOddsAPIResponse The Odds API 响应结构
type TheOddsAPIResponse struct {
	ID           string                  `json:"id"`
	SportKey     string                  `json:"sport_key"`
	SportTitle   string                  `json:"sport_title"`
	CommenceTime time.Time               `json:"commence_time"`
	HomeTeam     string                  `json:"home_team"`
	AwayTeam     string                  `json:"away_team"`
	Bookmakers   []TheOddsAPIBookmaker   `json:"bookmakers"`
}

type TheOddsAPIScore struct {
	Name  string `json:"name"`
	Score string `json:"score"`
}

type TheOddsAPIScoresResponse struct {
	ID           string            `json:"id"`
	SportKey     string            `json:"sport_key"`
	SportTitle   string            `json:"sport_title"`
	CommenceTime time.Time         `json:"commence_time"`
	Completed    bool              `json:"completed"`
	HomeTeam     string            `json:"home_team"`
	AwayTeam     string            `json:"away_team"`
	Scores       []TheOddsAPIScore `json:"scores"`
	LastUpdate   time.Time         `json:"last_update"`
}


type TheOddsAPIBookmaker struct {
	Key        string               `json:"key"`
	Title      string               `json:"title"`
	LastUpdate time.Time            `json:"last_update"`
	Markets    []TheOddsAPIMarket   `json:"markets"`
}

type TheOddsAPIMarket struct {
	Key      string               `json:"key"`
	LastUpdate time.Time          `json:"last_update"`
	Outcomes []TheOddsAPIOutcome  `json:"outcomes"`
}

type TheOddsAPIOutcome struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Point float64 `json:"point,omitempty"` // 用于让球/大小球盘口
}

// APIFootballFixture API-Football 比赛数据结构（简化版）
type APIFootballFixture struct {
	Fixture struct {
		ID     int    `json:"id"`
		Status struct {
			Short   string `json:"short"`
			Elapsed int    `json:"elapsed"`
		} `json:"status"`
		Date string `json:"date"`
	} `json:"fixture"`
	League struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"league"`
	Teams struct {
		Home struct{ Name string `json:"name"` } `json:"home"`
		Away struct{ Name string `json:"name"` } `json:"away"`
	} `json:"teams"`
	Goals struct {
		Home *int `json:"home"`
		Away *int `json:"away"`
	} `json:"goals"`
	Statistics []APIFootballStatistic `json:"statistics"`
}

type APIFootballStatistic struct {
	Team  struct{ Name string `json:"name"` } `json:"team"`
	Stats []struct {
		Type  string      `json:"type"`
		Value interface{} `json:"value"` // 可能是 string/int/float
	} `json:"statistics"`
}
