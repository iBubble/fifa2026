package db

import (
	"encoding/json"
	"fifa2026/src/internal/models"
	"fmt"
	"os"
	"time"
)

// ImportInitialData 从本地 JSON 静态文件将 2026 世界杯基础数据录入 SQLite 数据库
func ImportInitialData(seasonsFilePath, featuresFilePath string) error {
	// 1. 初始化 2026 世界杯赛季元数据
	t := models.Tournament{
		ID:     "fifa_2026",
		Name:   "2026 FIFA World Cup",
		Year:   2026,
		Status: "PENDING",
	}
	if err := SaveTournament(t); err != nil {
		return fmt.Errorf("导入世界杯赛季失败: %w", err)
	}

	// 2. 读取世界杯球队分组与赛程表
	seasonsData, err := os.ReadFile(seasonsFilePath)
	if err != nil {
		return fmt.Errorf("读取赛季JSON文件 %s 失败: %w", seasonsFilePath, err)
	}

	var rawSeason struct {
		Matches []struct {
			ID          string `json:"id"`
			HomeTeam    string `json:"homeTeam"`
			AwayTeam    string `json:"awayTeam"`
			Group       string `json:"group"`
			ScheduledAt string `json:"scheduledAt"`
			Status      string `json:"status"`
			Venue       string `json:"venue"`
		} `json:"matches"`
	}

	if err := json.Unmarshal(seasonsData, &rawSeason); err != nil {
		return fmt.Errorf("解析赛季JSON失败: %w", err)
	}

	// 3. 读取各国家队历史特征并进行数据预加载（本系统由 Elo 服务实现）

	// 4. 清理残留的旧静态导入赛程数据和竞彩生成的重复数据（注意：保留官方 wc2026_ 下已完赛的 FT 赛事）
	if _, err := DB.Exec("DELETE FROM matches WHERE id LIKE 'sporttery_%'"); err != nil {
		return fmt.Errorf("清理旧竞彩赛程失败: %w", err)
	}
	if _, err := DB.Exec("DELETE FROM matches WHERE id LIKE 'wc2026_%' AND status != 'FT'"); err != nil {
		return fmt.Errorf("清理旧静态赛程失败: %w", err)
	}
	if _, err := DB.Exec("DELETE FROM matches WHERE tournament_id = 'fifa_2026' AND id NOT LIKE 'wc2026_%'"); err != nil {
		return fmt.Errorf("清理非世界杯脏数据失败: %w", err)
	}

	// 5. 导入 JSON 文件中的所有比赛，但在导入前检查并保护已经完赛的 FT 赛事
	for _, m := range rawSeason.Matches {
		var existingStatus string
		var existingHomeScore, existingAwayScore int
		err := DB.QueryRow("SELECT status, home_score, away_score FROM matches WHERE id = ?", m.ID).Scan(&existingStatus, &existingHomeScore, &existingAwayScore)
		
		status := m.Status
		homeScore := 0
		awayScore := 0
		
		if err == nil && existingStatus == "FT" {
			status = "FT"
			homeScore = existingHomeScore
			awayScore = existingAwayScore
		}

		scheduledTime, err := time.Parse(time.RFC3339, m.ScheduledAt)
		if err != nil {
			scheduledTime = time.Now().Add(24 * time.Hour) // 容错兜底
		}

		matchObj := models.Match{
			ID:           m.ID,
			TournamentID: "fifa_2026",
			HomeTeam:     m.HomeTeam,
			AwayTeam:     m.AwayTeam,
			Group:        m.Group,
			ScheduledAt:  scheduledTime,
			Status:       status,
			HomeScore:    homeScore,
			AwayScore:    awayScore,
			Venue:        m.Venue,
		}
		if err := SaveMatch(matchObj); err != nil {
			return fmt.Errorf("保存静态赛事 %s 失败: %w", m.ID, err)
		}
	}

	// 6. 初始化 48 支参赛队的中英文权威映射表
	InitTeamTranslations()

	return nil
}
