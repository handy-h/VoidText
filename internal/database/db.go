package database

import (
	"database/sql"
	"fmt"
	"log"
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

	// WAL 模式下读操作不阻塞写操作，设置合理连接数
	// SQLite 仅支持单写入者，MaxOpenConns 不宜过大，避免 SQLITE_BUSY
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	// 启用 WAL 模式，允许并发读写，改善 LLM 修复期间的 HTTP 响应速度
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return fmt.Errorf("启用WAL模式失败: %w", err)
	}
	if _, err := db.Exec(`PRAGMA synchronous=NORMAL`); err != nil {
		return fmt.Errorf("设置同步模式失败: %w", err)
	}
	// 设置繁忙超时，避免 SQLITE_BUSY 立即失败（等待最多 5 秒）
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		return fmt.Errorf("设置繁忙超时失败: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 迁移：为已有库添加断点恢复所需的列
	migrateSchema(db)

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
			review_baseline_path TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			current_step TEXT,
			progress INTEGER NOT NULL DEFAULT 0,
			rules_config TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			error_msg TEXT,
			llm_progress_paragraph INTEGER NOT NULL DEFAULT 0,
			llm_progress_checkpoint TEXT,
			cancel_flag INTEGER NOT NULL DEFAULT 0
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
		`CREATE TABLE IF NOT EXISTS review_paragraphs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_md5 TEXT NOT NULL,
			paragraph_index INTEGER NOT NULL,
			original_text TEXT NOT NULL,
			suggested_text TEXT NOT NULL,
			modification_type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			edited_text TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			resolved_at TIMESTAMP,
			UNIQUE(file_md5, paragraph_index)
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
		`CREATE TABLE IF NOT EXISTS chunk_repair_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_md5 TEXT NOT NULL,
			chunk_id INTEGER NOT NULL,
			chunk_hash TEXT NOT NULL,
			original_text TEXT,
			repaired_text TEXT,
			prompt_version TEXT,
			api_model TEXT,
			token_usage INTEGER NOT NULL DEFAULT 0,
			processing_time_ms INTEGER NOT NULL DEFAULT 0,
			confidence REAL NOT NULL DEFAULT 0,
			source TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(file_md5, chunk_hash)
		)`,
		`CREATE TABLE IF NOT EXISTS retry_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_md5 TEXT NOT NULL,
			chunk_id INTEGER NOT NULL,
			original_text TEXT,
			failure_reason TEXT,
			error_type TEXT,
			error_context TEXT,
			prompt_version TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0,
			max_retries INTEGER NOT NULL DEFAULT 3,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			next_retry_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS prompt_versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			prompt_name TEXT NOT NULL,
			prompt_version TEXT NOT NULL,
			prompt_content TEXT,
			source TEXT,
			error_pattern TEXT,
			success_rate REAL NOT NULL DEFAULT 0,
			total_uses INTEGER NOT NULL DEFAULT 0,
			successful_uses INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(prompt_name, prompt_version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_files_md5 ON files(md5)`,
		`CREATE INDEX IF NOT EXISTS idx_files_original_md5 ON files(original_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_files_status ON files(status)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_version_md5 ON versions(version_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_original_md5 ON versions(original_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_review_paragraphs_file_md5 ON review_paragraphs(file_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_review_paragraphs_status ON review_paragraphs(file_md5, status)`,
		`CREATE INDEX IF NOT EXISTS idx_processing_logs_file_md5 ON processing_logs(file_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_chunk_cache_file_md5 ON chunk_repair_cache(file_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_retry_queue_file_md5 ON retry_queue(file_md5)`,
		`CREATE INDEX IF NOT EXISTS idx_retry_queue_pending ON retry_queue(status, next_retry_at)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			preview := stmt
			if len(stmt) > 50 {
				preview = stmt[:50] + "..."
			}
			return fmt.Errorf("执行建表语句失败 [%s]: %w", preview, err)
		}
	}

	return nil
}

// columnExists 通过 PRAGMA table_info 检查列是否已存在
func columnExists(db *sql.DB, table, column string) bool {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err == nil {
			if name == column {
				return true
			}
		}
	}
	return false
}

// migrateSchema 执行数据库迁移（先查询列是否存在，避免依赖错误消息文本）
func migrateSchema(db *sql.DB) {
	migrations := []struct {
		stmt   string
		column string
	}{
		{`ALTER TABLE files ADD COLUMN llm_progress_paragraph INTEGER NOT NULL DEFAULT 0`, "llm_progress_paragraph"},
		{`ALTER TABLE files ADD COLUMN llm_progress_checkpoint TEXT`, "llm_progress_checkpoint"},
		{`ALTER TABLE files ADD COLUMN cancel_flag INTEGER NOT NULL DEFAULT 0`, "cancel_flag"},
		{`ALTER TABLE files ADD COLUMN review_baseline_path TEXT`, "review_baseline_path"},
	}
	for _, m := range migrations {
		if columnExists(db, "files", m.column) {
			continue
		}
		_, err := db.Exec(m.stmt)
		if err != nil {
			log.Printf("[数据库迁移] 警告：执行迁移语句失败: %v (语句: %s)", err, m.stmt)
		}
	}
}
