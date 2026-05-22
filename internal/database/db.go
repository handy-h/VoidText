package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Table names (exported for use in migrations and tests)

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

	// SQLite并发优化：启用WAL模式（Write-Ahead Logging）
	// WAL模式允许多个读和单个写并发，避免写入时锁定整个数据库
	// 配合SetMaxOpenConns(1)可确保写操作顺序执行，避免SQLite的并发限制
	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		return fmt.Errorf("启用WAL模式失败: %w", err)
	}

	// 设置WAL自动检查点（每1000页或100MB）
	if _, err := db.Exec("PRAGMA wal_autocheckpoint = 1000;"); err != nil {
		return fmt.Errorf("设置WAL自动检查点失败: %w", err)
	}

	// 设置同步模式为NORMAL（在WAL模式下更安全）
	if _, err := db.Exec("PRAGMA synchronous = NORMAL;"); err != nil {
		return fmt.Errorf("设置同步模式失败: %w", err)
	}

	// 设置缓存大小（页数）提高性能
	if _, err := db.Exec("PRAGMA cache_size = -10000;"); err != nil {
		return fmt.Errorf("设置缓存大小失败: %w", err)
	}

	// 启用外键约束（SQLite 默认关闭）
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return fmt.Errorf("启用外键约束失败: %w", err)
	}

	// 写锁等待超时：5秒内重试，避免并发写入时立即报 "database is locked"
	if _, err := db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		return fmt.Errorf("设置 busy_timeout 失败: %w", err)
	}

	// SQLite并发限制：单个连接可以安全处理多个goroutine的并发操作
	// 但写入操作必须序列化，所以设置MaxOpenConns=1确保写入顺序执行
	// 读取操作可以并发，但SQLite驱动本身支持并发读取
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// 设置连接最大存活时间，防止长时间连接导致的资源泄露
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	if err := createTables(); err != nil {
		return fmt.Errorf("创建表结构失败: %w", err)
	}

	if err := migrateTables(); err != nil {
		return fmt.Errorf("迁移表结构失败: %w", err)
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
		// 关闭前执行检查点，确保WAL日志写入主数据库
		if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
			// 仅记录错误，不中断关闭流程
			fmt.Fprintf(os.Stderr, "执行WAL检查点失败: %v\n", err)
		}
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
		// 新增：块级修复缓存表，支持断点续传和幂等性（混合架构版）
		`CREATE TABLE IF NOT EXISTS chunk_repair_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_md5 TEXT NOT NULL,
			chunk_id INTEGER NOT NULL,
			chunk_hash TEXT NOT NULL,
			original_text TEXT NOT NULL,
			repaired_text TEXT NOT NULL,
			prompt_version TEXT NOT NULL,
			api_model TEXT,
			token_usage INTEGER,
			processing_time_ms INTEGER,
			confidence REAL DEFAULT 1.0, -- 处理结果置信度
			source TEXT DEFAULT 'remote', -- 处理来源：local/remote/cache
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(file_md5, chunk_hash)
		)`,
		// 新增：重试队列表，用于Evolver自进化
		`CREATE TABLE IF NOT EXISTS retry_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_md5 TEXT NOT NULL,
			chunk_id INTEGER NOT NULL,
			original_text TEXT NOT NULL,
			failure_reason TEXT NOT NULL,
			error_type TEXT NOT NULL,
			error_context TEXT,
			prompt_version TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0,
			max_retries INTEGER NOT NULL DEFAULT 3,
			status TEXT NOT NULL DEFAULT 'pending', -- pending/processing/completed/failed
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			next_retry_at TIMESTAMP
		)`,
		// 新增：提示词版本表，用于Evolver自进化
		`CREATE TABLE IF NOT EXISTS prompt_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt_name TEXT NOT NULL,
			prompt_version TEXT NOT NULL,
			prompt_content TEXT NOT NULL,
			source TEXT NOT NULL, -- file/database/evolver
			error_pattern TEXT, -- 触发此版本的错误模式
			success_rate REAL,
			total_uses INTEGER NOT NULL DEFAULT 0,
			successful_uses INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(prompt_name, prompt_version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_files_original_md5 ON files(original_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_files_status ON files(status)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_original_md5 ON versions(original_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_review_items_file_md5 ON review_items(file_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_review_items_status ON review_items(file_md5, status)`,
		`CREATE INDEX IF NOT EXISTS idx_processing_logs_file_id ON processing_logs(file_md5, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_processing_logs_file_ts ON processing_logs(file_md5, timestamp DESC)`,
		// 新增索引
fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_chunk_repair_cache_file_md5 ON %s(file_md5)`, ChunkRepairCacheTable),
fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_chunk_repair_cache_chunk_hash ON %s(chunk_hash)`, ChunkRepairCacheTable),
		`CREATE INDEX IF NOT EXISTS idx_retry_queue_status ON retry_queue(status)`,
		`CREATE INDEX IF NOT EXISTS idx_retry_queue_next_retry ON retry_queue(next_retry_at) WHERE status = 'pending'`,
		`CREATE INDEX IF NOT EXISTS idx_prompt_versions_name_version ON prompt_versions(prompt_name, prompt_version)`,
	}

	// 使用事务批量执行建表语句，提高效率并确保原子性
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("执行建表语句失败 [%s]: %w", stmt[:50], err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// migrateTables 迁移表结构，为旧数据库添加缺失的列
func migrateTables() error {
	migrations := []string{
		`ALTER TABLE chunk_repair_cache ADD COLUMN confidence REAL DEFAULT 1.0`,
		`ALTER TABLE chunk_repair_cache ADD COLUMN source TEXT DEFAULT 'remote'`,
		// 删除被 UNIQUE 约束自动覆盖的冗余索引
		`DROP INDEX IF EXISTS idx_files_md5`,
		`DROP INDEX IF EXISTS idx_versions_version_md5`,
		// 用复合索引替换 processing_logs 单列索引
		`DROP INDEX IF EXISTS idx_processing_logs_file_md5`,
		`CREATE INDEX IF NOT EXISTS idx_processing_logs_file_id ON processing_logs(file_md5, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_processing_logs_file_ts ON processing_logs(file_md5, timestamp DESC)`,
	}

	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			if !isColumnExistsError(err) {
				return fmt.Errorf("执行迁移失败 [%s]: %w", stmt[:60], err)
			}
		}
	}

	return nil
}

// isColumnExistsError 判断是否是列已存在的错误
func isColumnExistsError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "duplicate column") ||
		contains(errStr, "column already exists") ||
		contains(errStr, "SQL logic error")
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
