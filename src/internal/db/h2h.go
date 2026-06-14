package db

import (
	"database/sql"
	"strings"
)

// GetSortedTeamKey 按照字母顺序对主客队英文名进行排序，并生成唯一规范的 teamKey
func GetSortedTeamKey(team1, team2 string) (teamKey string, teamA string, teamB string) {
	t1 := strings.TrimSpace(team1)
	t2 := strings.TrimSpace(team2)
	if t1 < t2 {
		return t1 + ":" + t2, t1, t2
	}
	return t2 + ":" + t1, t2, t1
}

// SaveTeamApiMapping 保存球队名称到 api-football ID 的映射记录
func SaveTeamApiMapping(teamName string, apiTeamID int) error {
	query := `INSERT OR REPLACE INTO team_api_mappings (team_name, api_team_id, created_at)
              VALUES (?, ?, CURRENT_TIMESTAMP)`
	_, err := DB.Exec(query, strings.TrimSpace(teamName), apiTeamID)
	return err
}

// GetTeamApiMapping 获取球队对应的 api-football ID，如不存在返回 0
func GetTeamApiMapping(teamName string) (int, error) {
	query := `SELECT api_team_id FROM team_api_mappings WHERE team_name = ?`
	var apiTeamID int
	err := DB.QueryRow(query, strings.TrimSpace(teamName)).Scan(&apiTeamID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return apiTeamID, err
}

// SaveH2HRecord 缓存两队的历史交锋统计数据
func SaveH2HRecord(team1, team2 string, totalMatches, teamAWins, draws, teamBWins int, avgAGoals, avgBGoals float64, matchesJson string) error {
	teamKey, _, _ := GetSortedTeamKey(team1, team2)
	query := `INSERT OR REPLACE INTO h2h_records 
              (team_key, total_matches, team_a_wins, draws, team_b_wins, avg_a_goals, avg_b_goals, matches_json, last_updated)
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`
	_, err := DB.Exec(query, teamKey, totalMatches, teamAWins, draws, teamBWins, avgAGoals, avgBGoals, matchesJson)
	return err
}

// GetH2HRecord 读取本地两队的历史交手统计数据
func GetH2HRecord(team1, team2 string) (totalMatches, teamAWins, draws, teamBWins int, avgAGoals, avgBGoals float64, matchesJson string, found bool, err error) {
	teamKey, _, _ := GetSortedTeamKey(team1, team2)
	query := `SELECT total_matches, team_a_wins, draws, team_b_wins, avg_a_goals, avg_b_goals, matches_json 
              FROM h2h_records WHERE team_key = ?`
	err = DB.QueryRow(query, teamKey).Scan(&totalMatches, &teamAWins, &draws, &teamBWins, &avgAGoals, &avgBGoals, &matchesJson)
	if err == sql.ErrNoRows {
		return 0, 0, 0, 0, 0, 0, "[]", false, nil
	}
	if err != nil {
		return 0, 0, 0, 0, 0, 0, "", false, err
	}
	return totalMatches, teamAWins, draws, teamBWins, avgAGoals, avgBGoals, matchesJson, true, nil
}

// GetCachedH2HRecordsCount 获取当前本地已经缓存的交手组合总数 (可用于调试)
func GetCachedH2HRecordsCount() (int, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM h2h_records").Scan(&count)
	return count, err
}
