// Package db 负责 SQLite 数据库的初始化、迁移和连接管理
// 使用 modernc.org/sqlite（纯 Go 实现，无需 CGO，macOS 开箱即用）
package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // 注册 sqlite 驱动
)

// DB 包级别的数据库连接（单例）
var DB *sql.DB

// Init 初始化 SQLite 数据库，创建数据目录并执行自动迁移
// dataDir: 数据库文件存放目录（通常为 ~/.football 或应用数据目录）
func Init(dataDir string) error {
	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, "football.db")
	log.Printf("[DB] 打开数据库: %s", dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	// 开启 WAL 模式提升并发读性能
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("开启 WAL 模式失败: %w", err)
	}

	// 开启外键约束
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("开启外键约束失败: %w", err)
	}

	// 设置连接池参数（SQLite 建议保持单写连接）
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	DB = db

	// 执行数据库迁移
	return migrate(db)
}

// Close 关闭数据库连接（应在应用退出时调用）
func Close() {
	if DB != nil {
		log.Println("[DB] 关闭数据库连接")
		DB.Close()
	}
}

// migrate 执行所有数据库建表迁移（幂等操作，使用 IF NOT EXISTS）
func migrate(db *sql.DB) error {
	log.Println("[DB] 执行数据库迁移...")

	migrations := []struct {
		name string
		sql  string
	}{
		{
			name: "create_matches",
			sql: `CREATE TABLE IF NOT EXISTS matches (
				id TEXT PRIMARY KEY,
				home_team TEXT NOT NULL,
				away_team TEXT NOT NULL,
				league TEXT NOT NULL DEFAULT '',
				country TEXT NOT NULL DEFAULT '',
				scheduled_at DATETIME NOT NULL,
				status TEXT NOT NULL DEFAULT 'NS',
				home_score INTEGER NOT NULL DEFAULT 0,
				away_score INTEGER NOT NULL DEFAULT 0,
				minute INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
		},
		{
			name: "create_odds_history",
			sql: `CREATE TABLE IF NOT EXISTS odds_history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				match_id TEXT NOT NULL,
				bookmaker TEXT NOT NULL,
				market TEXT NOT NULL,
				outcome TEXT NOT NULL,
				odds REAL NOT NULL,
				captured_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (match_id) REFERENCES matches(id)
			)`,
		},
		{
			name: "create_bets",
			sql: `CREATE TABLE IF NOT EXISTS bets (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				match_id TEXT NOT NULL,
				home_team TEXT NOT NULL DEFAULT '',
				away_team TEXT NOT NULL DEFAULT '',
				bookmaker TEXT NOT NULL,
				market TEXT NOT NULL,
				outcome TEXT NOT NULL,
				odds REAL NOT NULL,
				stake REAL NOT NULL,
				result TEXT NOT NULL DEFAULT 'PENDING',
				pnl REAL NOT NULL DEFAULT 0,
				kelly_fraction REAL NOT NULL DEFAULT 0,
				placed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				settled_at DATETIME,
				notes TEXT DEFAULT ''
			)`,
		},
		{
			name: "create_arbitrage_log",
			sql: `CREATE TABLE IF NOT EXISTS arbitrage_log (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				match_id TEXT NOT NULL,
				market TEXT NOT NULL DEFAULT '',
				l_value REAL NOT NULL,
				roi REAL NOT NULL,
				details TEXT NOT NULL DEFAULT '{}',
				detected_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
		},
		{
			name: "create_idx_odds_match",
			sql:  `CREATE INDEX IF NOT EXISTS idx_odds_match ON odds_history(match_id, captured_at DESC)`,
		},
		{
			name: "create_idx_bets_match",
			sql:  `CREATE INDEX IF NOT EXISTS idx_bets_match ON bets(match_id)`,
		},
	}

	for _, m := range migrations {
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("迁移 [%s] 失败: %w", m.name, err)
		}
		log.Printf("[DB] ✓ %s", m.name)
	}

	log.Println("[DB] 数据库迁移完成")
	return nil
}
