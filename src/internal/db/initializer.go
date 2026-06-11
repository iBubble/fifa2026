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

	// 3. 读取各国家队历史特征，并导入初始 Elo
	// 提示: 我们在这个初创阶段，将各队的特征导入，后续预测引擎可根据此处初始 Elo 计算进球 lambda
	// 这一部分可以直接由 Go 的 Elo 服务进行内存化/数据库化加载

	// 4. 将静态赛程表的比赛写入 SQLite
	for _, m := range rawSeason.Matches {
		// 检查数据库中是否已存在该比赛，避免覆盖已完赛 (FT) 或进行中 (Live) 的数据
		var exists int
		err := DB.QueryRow("SELECT 1 FROM matches WHERE id = ?", m.ID).Scan(&exists)
		if err == nil && exists == 1 {
			continue
		}

		// 真正的导入使用 types.go 中的 Match 结构
		var match models.Match
		match.ID = m.ID
		match.TournamentID = "fifa_2026"
		match.HomeTeam = m.HomeTeam
		match.AwayTeam = m.AwayTeam
		match.Group = m.Group
		match.Status = m.Status
		match.Venue = m.Venue

		// 我们使用 time.Parse 来解析
		// json 格式为 "2026-06-11T18:00:00Z" (RFC3339)
		var tErr error
		match.ScheduledAt, tErr = time.Parse("2006-01-02T15:04:05Z", m.ScheduledAt)
		if tErr != nil {
			match.ScheduledAt = time.Now() // 备用
		}

		if err := SaveMatch(match); err != nil {
			return fmt.Errorf("导入比赛 %s 失败: %w", match.ID, err)
		}
	}

	return nil
}
