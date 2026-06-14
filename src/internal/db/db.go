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
			proponent_opinion TEXT NOT NULL DEFAULT '',
			critique_analysis TEXT NOT NULL DEFAULT '',
			consensus_reason TEXT NOT NULL DEFAULT '',
			original_score_matrix_json TEXT NOT NULL DEFAULT '[]',
			original_over_2_5_prob REAL NOT NULL DEFAULT 0.0,
			original_under_2_5_prob REAL NOT NULL DEFAULT 0.0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (match_id) REFERENCES matches(id)
		);`,

		`CREATE TABLE IF NOT EXISTS team_api_mappings (
			team_name TEXT PRIMARY KEY,
			api_team_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS h2h_records (
			team_key TEXT PRIMARY KEY,
			total_matches INTEGER NOT NULL,
			team_a_wins INTEGER NOT NULL,
			draws INTEGER NOT NULL,
			team_b_wins INTEGER NOT NULL,
			avg_a_goals REAL NOT NULL,
			avg_b_goals REAL NOT NULL,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		`CREATE TABLE IF NOT EXISTS lottery_plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plan_type TEXT NOT NULL,
			match_ids TEXT NOT NULL,
			risk_level TEXT,
			odds_h REAL,
			odds_d REAL,
			odds_a REAL,
			primary_bet TEXT,
			primary_odds REAL,
			primary_amt REAL,
			hedge_bet TEXT,
			hedge_odds REAL,
			hedge_amt REAL,
			parlay_type TEXT,
			parlay_mode TEXT,
			parlay_options TEXT,
			desc_str TEXT,
			wins_count INTEGER,
			cost REAL,
			single_ticket_payout REAL,
			combo_odds REAL,
			combo_prob REAL,
			total_ev REAL,
			kelly_stake REAL,
			tickets_json TEXT,
			is_settled INTEGER DEFAULT 0,
			safe_profit REAL DEFAULT 0.0,
			safe_return REAL DEFAULT 0.0,
			agg_profit REAL DEFAULT 0.0,
			agg_return REAL DEFAULT 0.0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// 48 支参赛队中英文权威映射表
		`CREATE TABLE IF NOT EXISTS team_translations (
			en_name TEXT PRIMARY KEY,
			cn_name TEXT NOT NULL,
			flag_code TEXT NOT NULL DEFAULT '',
			initial_elo REAL NOT NULL DEFAULT 1500.0,
			avg_goals_scored REAL NOT NULL DEFAULT 1.35,
			avg_goals_conceded REAL NOT NULL DEFAULT 1.20,
			clean_sheet_rate REAL NOT NULL DEFAULT 0.25,
			api_team_id INTEGER NOT NULL DEFAULT 0,
			aliases TEXT NOT NULL DEFAULT '[]',
			fifa_ranking INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			return err
		}
	}

	// 平滑升级：若已存在的表没有 fifa_ranking 字段，自动添加
	_, _ = DB.Exec("ALTER TABLE team_translations ADD COLUMN fifa_ranking INTEGER NOT NULL DEFAULT 0")

	// 平滑升级：若已存在的表没有双 Agent 辩论及双矩阵比对字段，自动添加
	_, _ = DB.Exec("ALTER TABLE prediction_reports ADD COLUMN proponent_opinion TEXT NOT NULL DEFAULT ''")
	_, _ = DB.Exec("ALTER TABLE prediction_reports ADD COLUMN critique_analysis TEXT NOT NULL DEFAULT ''")
	_, _ = DB.Exec("ALTER TABLE prediction_reports ADD COLUMN consensus_reason TEXT NOT NULL DEFAULT ''")
	_, _ = DB.Exec("ALTER TABLE prediction_reports ADD COLUMN original_score_matrix_json TEXT NOT NULL DEFAULT '[]'")
	_, _ = DB.Exec("ALTER TABLE prediction_reports ADD COLUMN original_over_2_5_prob REAL NOT NULL DEFAULT 0.0")
	_, _ = DB.Exec("ALTER TABLE prediction_reports ADD COLUMN original_under_2_5_prob REAL NOT NULL DEFAULT 0.0")

	return nil
}
