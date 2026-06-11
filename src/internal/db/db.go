package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// Init 初始化数据库，自动创建数据目录并建立多赛季隔离表
func Init(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, "fifa2026.db")
	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("打开SQLite数据库失败: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("数据库连接Ping失败: %w", err)
	}

	if err := createTables(); err != nil {
		return fmt.Errorf("数据库建表失败: %w", err)
	}

	return nil
}

// Close 关闭数据库连接
func Close() {
	if DB != nil {
		DB.Close()
	}
}

// createTables 创建多赛季隔离表
func createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tournaments (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			year INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'PENDING',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS matches (
			id TEXT PRIMARY KEY,
			tournament_id TEXT NOT NULL,
			home_team TEXT NOT NULL,
			away_team TEXT NOT NULL,
			match_group TEXT NOT NULL,
			scheduled_at DATETIME NOT NULL,
			status TEXT NOT NULL DEFAULT 'NS',
			home_score INTEGER DEFAULT 0,
			away_score INTEGER DEFAULT 0,
			venue TEXT NOT NULL,
			FOREIGN KEY (tournament_id) REFERENCES tournaments(id)
		);`,

		`CREATE TABLE IF NOT EXISTS bets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tournament_id TEXT NOT NULL,
			match_id TEXT NOT NULL,
			home_team TEXT NOT NULL,
			away_team TEXT NOT NULL,
			bookmaker TEXT NOT NULL,
			market TEXT NOT NULL,
			outcome TEXT NOT NULL,
			odds REAL NOT NULL,
			stake REAL NOT NULL,
			result TEXT NOT NULL DEFAULT 'PENDING',
			pnl REAL DEFAULT 0.0,
			kelly_fraction REAL DEFAULT 0.0,
			consensus_prob REAL DEFAULT 0.0,
			expected_value REAL DEFAULT 0.0,
			placed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (tournament_id) REFERENCES tournaments(id)
		);`,

		`CREATE TABLE IF NOT EXISTS odds_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			match_id TEXT NOT NULL,
			bookmaker TEXT NOT NULL,
			home_odds REAL NOT NULL,
			draw_odds REAL NOT NULL,
			away_odds REAL NOT NULL,
			captured_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS backtest_reports (
			match_id TEXT PRIMARY KEY,
			brier_score REAL NOT NULL,
			home_elo_diff REAL NOT NULL,
			away_elo_diff REAL NOT NULL,
			tactics_review TEXT NOT NULL,
			reviewed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS news_articles (
			source_url TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			summary TEXT NOT NULL,
			publish_time DATETIME NOT NULL,
			source_site TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS prediction_reports (
			match_id TEXT PRIMARY KEY,
			lambda_home REAL NOT NULL,
			lambda_away REAL NOT NULL,
			rho REAL NOT NULL,
			original_lambda_home REAL NOT NULL,
			original_lambda_away REAL NOT NULL,
			original_rho REAL NOT NULL,
			over_2_5_prob REAL NOT NULL,
			under_2_5_prob REAL NOT NULL,
			tactics_analysis TEXT NOT NULL,
			poster_prompt TEXT NOT NULL,
			score_matrix_json TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (match_id) REFERENCES matches(id)
		);`,
	}

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			return err
		}
	}
	return nil
}
