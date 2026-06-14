package db

import (
	"database/sql"
	"encoding/json"
	"strings"
)

// TeamTranslation 单支参赛队的中英文映射与特征数据
type TeamTranslation struct {
	EnName           string   `json:"enName"`
	CnName           string   `json:"cnName"`
	FlagCode         string   `json:"flagCode"`
	InitialElo       float64  `json:"initialElo"`
	AvgGoalsScored   float64  `json:"avgGoalsScored"`
	AvgGoalsConceded float64  `json:"avgGoalsConceded"`
	CleanSheetRate   float64  `json:"cleanSheetRate"`
	ApiTeamID        int      `json:"apiTeamId"`
	Aliases          []string `json:"aliases"`
	FifaRanking      int      `json:"fifaRanking"`
}

// SaveTeamTranslation 保存或更新单支队伍的翻译映射
func SaveTeamTranslation(t TeamTranslation) error {
	aliasJSON, _ := json.Marshal(t.Aliases)
	query := `INSERT OR REPLACE INTO team_translations 
		(en_name, cn_name, flag_code, initial_elo, avg_goals_scored, avg_goals_conceded, clean_sheet_rate, api_team_id, aliases, fifa_ranking)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := DB.Exec(query, t.EnName, t.CnName, t.FlagCode,
		t.InitialElo, t.AvgGoalsScored, t.AvgGoalsConceded, t.CleanSheetRate,
		t.ApiTeamID, string(aliasJSON), t.FifaRanking)
	return err
}

// GetTeamTranslation 按英文标准名查询翻译
func GetTeamTranslation(enName string) (TeamTranslation, error) {
	query := `SELECT en_name, cn_name, flag_code, initial_elo, avg_goals_scored, avg_goals_conceded, clean_sheet_rate, api_team_id, aliases, fifa_ranking
		FROM team_translations WHERE en_name = ?`
	var t TeamTranslation
	var aliasStr string
	err := DB.QueryRow(query, strings.TrimSpace(enName)).Scan(
		&t.EnName, &t.CnName, &t.FlagCode,
		&t.InitialElo, &t.AvgGoalsScored, &t.AvgGoalsConceded, &t.CleanSheetRate,
		&t.ApiTeamID, &aliasStr, &t.FifaRanking)
	if err != nil {
		return t, err
	}
	_ = json.Unmarshal([]byte(aliasStr), &t.Aliases)
	return t, nil
}

// GetTeamByAlias 通过别名（中文名/缩写等）反查标准英文名
func GetTeamByAlias(alias string) (string, error) {
	clean := strings.TrimSpace(alias)
	// 1. 直接匹配英文名
	var enName string
	err := DB.QueryRow("SELECT en_name FROM team_translations WHERE en_name = ?", clean).Scan(&enName)
	if err == nil {
		return enName, nil
	}
	// 2. 匹配中文名
	err = DB.QueryRow("SELECT en_name FROM team_translations WHERE cn_name = ?", clean).Scan(&enName)
	if err == nil {
		return enName, nil
	}
	// 3. 在 aliases JSON 数组中模糊搜索
	rows, err := DB.Query("SELECT en_name, aliases FROM team_translations")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var en, aliasStr string
		if err := rows.Scan(&en, &aliasStr); err != nil {
			continue
		}
		var aliases []string
		_ = json.Unmarshal([]byte(aliasStr), &aliases)
		for _, a := range aliases {
			if strings.EqualFold(a, clean) {
				return en, nil
			}
		}
	}
	return "", sql.ErrNoRows
}

// GetAllTeamTranslations 获取全部 48 支队的翻译数据
func GetAllTeamTranslations() ([]TeamTranslation, error) {
	rows, err := DB.Query(`SELECT en_name, cn_name, flag_code, initial_elo, 
		avg_goals_scored, avg_goals_conceded, clean_sheet_rate, api_team_id, aliases, fifa_ranking
		FROM team_translations ORDER BY en_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TeamTranslation
	for rows.Next() {
		var t TeamTranslation
		var aliasStr string
		if err := rows.Scan(&t.EnName, &t.CnName, &t.FlagCode,
			&t.InitialElo, &t.AvgGoalsScored, &t.AvgGoalsConceded, &t.CleanSheetRate,
			&t.ApiTeamID, &aliasStr, &t.FifaRanking); err != nil {
			continue
		}
		_ = json.Unmarshal([]byte(aliasStr), &t.Aliases)
		results = append(results, t)
	}
	return results, nil
}
