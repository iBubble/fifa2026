package models

import "time"

// Tournament 代表一个杯赛或赛季，如 "fifa_2026"
type Tournament struct {
	ID        string    `json:"id"`     // 唯一ID，如 "fifa_2026"
	Name      string    `json:"name"`   // 赛事名称
	Year      int       `json:"year"`   // 年份
	Status    string    `json:"status"` // 状态: PENDING/ACTIVE/FINISHED
	CreatedAt time.Time `json:"createdAt"`
}

// Team 代表一支参赛国家队
type Team struct {
	Name             string  `json:"name"`             // 英文名作唯一标识，如 "Brazil"
	ChineseName      string  `json:"chineseName"`      // 中文名，如 "巴西"
	InitialElo       float64 `json:"initialElo"`       // 初始 Elo
	CurrentElo       float64 `json:"currentElo"`       // 实时演变 Elo
	Titles           int     `json:"titles"`           // 夺冠次数
	AvgGoalsScored   float64 `json:"avgGoalsScored"`   // 历史世界杯场均进球
	AvgGoalsConceded float64 `json:"avgGoalsConceded"` // 历史世界杯场均失球
	CleanSheetRate   float64 `json:"cleanSheetRate"`   // 零封率
	Description      string  `json:"description"`      // 球队特征描述
}

// Match 代表一场具体的世界杯比赛
type Match struct {
	ID           string    `json:"id"`
	TournamentID string    `json:"tournamentId"` // 关联的赛事ID，进行多赛季隔离
	HomeTeam     string    `json:"homeTeam"`     // 主队名 (与 Team.Name 关联)
	AwayTeam     string    `json:"awayTeam"`     // 客队名
	Group        string    `json:"group"`        // 分组，如 "A"
	ScheduledAt  time.Time `json:"scheduledAt"`  // 开赛时间
	Status       string    `json:"status"`       // "NS"=未开赛 "FT"=结束 "1H"=上半场 "2H"=下半场
	HomeScore    int       `json:"homeScore"`
	AwayScore    int       `json:"awayScore"`
	Venue        string    `json:"venue"` // 比赛场馆
}
