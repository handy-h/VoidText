package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// Init 初始化数据库连接和表结构
func Init(dataDir string) error {
	dbPath := filepath.Join(dataDir, "cleaning.db")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	if err := createTables(); err != nil {
		return fmt.Errorf("创建表结构失败: %w", err)
	}

	return nil
}

// GetDB 获取数据库连接
func GetDB() *sql.DB {
	return db
}

// Close 关闭数据库连接
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

func createTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			md5 TEXT UNIQUE NOT NULL,
			original_md5 TEXT,
			author TEXT,
			title TEXT,
			file_name TEXT,
			file_size INTEGER,
			file_path TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			current_step TEXT,
			progress INTEGER NOT NULL DEFAULT 0,
			rules_config TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			error_msg TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			original_md5 TEXT NOT NULL,
			version_md5 TEXT UNIQUE NOT NULL,
			parent_md5 TEXT,
			version_type TEXT NOT NULL,
			file_path TEXT NOT NULL,
			step TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS review_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_md5 TEXT NOT NULL,
			original_text TEXT,
			suggested_text TEXT,
			modification_type TEXT,
			confidence REAL,
			position_start INTEGER,
			position_end INTEGER,
			status TEXT NOT NULL DEFAULT 'pending',
			edited_text TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			resolved_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS processing_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_md5 TEXT,
			step TEXT,
			action TEXT,
			details TEXT,
			status TEXT,
			timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_files_md5 ON files(md5)`,
		`CREATE INDEX IF NOT EXISTS idx_files_original_md5 ON files(original_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_files_status ON files(status)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_version_md5 ON versions(version_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_original_md5 ON versions(original_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_review_items_file_md5 ON review_items(file_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_review_items_status ON review_items(file_md5, status)`,
		`CREATE INDEX IF NOT EXISTS idx_processing_logs_file_md5 ON processing_logs(file_md5)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("执行建表语句失败 [%s]: %w", stmt[:50], err)
		}
	}

	return nil
}
