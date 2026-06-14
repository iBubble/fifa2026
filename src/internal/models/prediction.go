package models

// DixonColesParams 保存 Dixon-Coles 算法的核心进球率及平局相关系数
type DixonColesParams struct {
	LambdaHome float64 `json:"lambdaHome"` // 主队期望进球率
	LambdaAway float64 `json:"lambdaAway"` // 客队期望进球率
	Rho        float64 `json:"rho"`        // 平局修正因子 (Dixon-Coles)
}

// LLMRefineOffsets 保存大模型分析后输出的参数定性修正偏移量
type LLMRefineOffsets struct {
	LambdaHomeOffset float64 `json:"lambdaHomeOffset"` // 主队λ偏移
	LambdaAwayOffset float64 `json:"lambdaAwayOffset"` // 客队λ偏移
	RhoOffset        float64 `json:"rhoOffset"`        // 平局因子偏移
	TacticsAnalysis  string  `json:"tacticsAnalysis"`  // 战术定性分析报告
	PosterPrompt     string  `json:"posterPrompt"`     // 生成的焦点战海报 Prompt (SD/MJ 英文提示词)
	ProponentOpinion string  `json:"proponentOpinion"` // 常规立论 (主张正面微调的理由)
	CritiqueAnalysis  string  `json:"critiqueAnalysis"`  // 魔鬼反驳 (指出冷门、逆向EV或高压降级的反面理由)
	ConsensusReason  string  `json:"consensusReason"`  // 理性决策共识 (折中折中裁判结论)
}

// ScoreProbability 代表单比分联合概率
type ScoreProbability struct {
	HomeScore int     `json:"homeScore"`
	AwayScore int     `json:"awayScore"`
	Prob      float64 `json:"prob"` // 联合概率 (0~1)
}

// PredictionReport 汇总单场对决的完整量化报告数据
type PredictionReport struct {
	MatchID              string             `json:"matchId"`
	OriginalParams       DixonColesParams   `json:"originalParams"`
	RefinedParams        DixonColesParams   `json:"refinedParams"`
	LLMRefined           bool               `json:"llmRefined"`  // 是否经过了LLM偏置修正
	ScoreMatrix          []ScoreProbability `json:"scoreMatrix"` // 纠偏校准后的比分概率矩阵
	Over2_5Prob          float64            `json:"over25Prob"`  // 纠偏校准后的大球 (Over 2.5) 概率
	Under2_5Prob         float64            `json:"under25Prob"` // 纠偏校准后的小球 (Under 2.5) 概率
	TacticsAnalysis      string             `json:"tacticsAnalysis"`
	PosterPrompt         string             `json:"posterPrompt"`
	ProponentOpinion     string             `json:"proponentOpinion"` // 常规立论
	CritiqueAnalysis     string             `json:"critiqueAnalysis"`  // 魔鬼反驳
	ConsensusReason      string             `json:"consensusReason"`  // 理性共识
	OriginalScoreMatrix  []ScoreProbability `json:"originalScoreMatrix"`  // 纯定量模型原始概率矩阵
	OriginalOver2_5Prob  float64            `json:"originalOver2_5Prob"`   // 纯定量模型原始大球概率
	OriginalUnder2_5Prob float64            `json:"originalUnder2_5Prob"`  // 纯定量模型原始小球概率
	H2H                  *H2HRecord         `json:"h2h"`      // 历史直接交锋记录
	HomeRank             int                `json:"homeRank"` // 主队实力排名
	AwayRank             int                `json:"awayRank"` // 客队实力排名
}

// SimulationResult 世界杯蒙特卡洛 10,000 次推演出的晋级/夺冠期望
type SimulationResult struct {
	TeamName     string  `json:"teamName"`
	GroupOutProb float64 `json:"groupOutProb"` // 小组出线概率
	Round16Prob  float64 `json:"round16Prob"`  // 16强概率
	QuarterProb  float64 `json:"quarterProb"`  // 8强概率
	SemiProb     float64 `json:"semiProb"`     // 4强概率
	FinalProb    float64 `json:"finalProb"`    // 决赛概率
	WinnerProb   float64 `json:"winnerProb"`   // 夺冠概率
}

// H2HRecord 代表两队之间的历史直接交手统计数据
type H2HRecord struct {
	TotalMatches int     `json:"totalMatches"`
	HomeWins     int     `json:"homeWins"`
	Draws        int     `json:"draws"`
	AwayWins     int     `json:"awayWins"`
	AvgHomeGoals float64 `json:"avgHomeGoals"`
	AvgAwayGoals float64 `json:"avgAwayGoals"`
}
