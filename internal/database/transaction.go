package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Transaction 事务接口
type Transaction interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Prepare(query string) (*sql.Stmt, error)
}

// TxWrapper 事务包装器
type TxWrapper struct {
	tx *sql.Tx
}

// Exec 执行SQL语句
func (t *TxWrapper) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.Exec(query, args...)
}

// Query 执行查询
func (t *TxWrapper) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.Query(query, args...)
}

// QueryRow 执行单行查询
func (t *TxWrapper) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRow(query, args...)
}

// Prepare 准备语句
func (t *TxWrapper) Prepare(query string) (*sql.Stmt, error) {
	return t.tx.Prepare(query)
}

// WithTransaction 在事务中执行函数
func WithTransaction(ctx context.Context, fn func(tx Transaction) error) error {
	// 开始事务
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}

	// 创建事务包装器
	txWrapper := &TxWrapper{tx: tx}

	// 执行函数
	err = fn(txWrapper)
	if err != nil {
		// 回滚事务
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Printf("回滚事务失败: %v", rbErr)
		}
		return err
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// WithTransactionSimple 简化版事务（无上下文）
func WithTransactionSimple(fn func(tx Transaction) error) error {
	return WithTransaction(context.Background(), fn)
}

// CreateFileWithVersion 创建文件记录和版本记录（原子操作）
func CreateFileWithVersion(fileRecord *FileRecord, versionRecord *VersionRecord) error {
	return WithTransactionSimple(func(tx Transaction) error {
		// 创建文件记录
		now := fileRecord.CreatedAt
		if now == "" {
			now = fileRecord.UpdatedAt
		}

		result, err := tx.Exec(`
      INSERT INTO files (md5, original_md5, author, title, file_name, original_file_name, file_size, file_path, review_baseline_path, status, current_step, progress, rules_config, created_at, updated_at, error_msg)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fileRecord.Md5, fileRecord.OriginalMd5, fileRecord.Author, fileRecord.Title,
			fileRecord.FileName, fileRecord.OriginalFileName, fileRecord.FileSize, fileRecord.FilePath,
			fileRecord.ReviewBaselinePath,
			fileRecord.Status, fileRecord.CurrentStep, fileRecord.Progress,
			fileRecord.RulesConfig, now, now, fileRecord.ErrorMsg,
		)
		if err != nil {
			return fmt.Errorf("创建文件记录失败: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("获取文件记录ID失败: %w", err)
		}
		fileRecord.ID = id
		fileRecord.CreatedAt = now
		fileRecord.UpdatedAt = now

		// 创建版本记录
		if versionRecord != nil {
			if versionRecord.CreatedAt == "" {
				versionRecord.CreatedAt = now
			}

			_, err = tx.Exec(`
        INSERT INTO versions (original_md5, version_md5, parent_md5, version_type, file_path, step, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
				versionRecord.OriginalMd5, versionRecord.VersionMd5, versionRecord.ParentMd5,
				versionRecord.VersionType, versionRecord.FilePath, versionRecord.Step,
				versionRecord.CreatedAt,
			)
			if err != nil {
				return fmt.Errorf("创建版本记录失败: %w", err)
			}
		}

		return nil
	})
}

// UpdateFileStatusWithLog 更新文件状态并记录日志（原子操作）
func UpdateFileStatusWithLog(fileMd5, status, currentStep string, progress int, errorMsg string, logDetails map[string]interface{}) error {
	return WithTransactionSimple(func(tx Transaction) error {
		// 更新文件状态
		now := time.Now().Format(time.RFC3339)
		_, err := tx.Exec(`
      UPDATE files SET status = ?, current_step = ?, progress = ?, error_msg = ?, updated_at = ? WHERE md5 = ?`,
			status, currentStep, progress, errorMsg, now, fileMd5)
		if err != nil {
			return fmt.Errorf("更新文件状态失败: %w", err)
		}

		// 记录处理日志
		if logDetails != nil {
			detailsJSON, err := json.Marshal(logDetails)
			if err != nil {
				return fmt.Errorf("序列化日志详情失败: %w", err)
			}

			_, err = tx.Exec(`
        INSERT INTO processing_logs (file_md5, step, action, details, status, timestamp)
        VALUES (?, ?, ?, ?, ?, ?)`,
				fileMd5, currentStep, "status_update", string(detailsJSON), status, now,
			)
			if err != nil {
				return fmt.Errorf("记录处理日志失败: %w", err)
			}
		}

		return nil
	})
}

// DeleteFileWithRelatedData 删除文件及相关数据（原子操作）
func DeleteFileWithRelatedData(fileMd5 string) error {
	return WithTransactionSimple(func(tx Transaction) error {
		// 删除段级审核记录
		_, err := tx.Exec("DELETE FROM review_paragraphs WHERE file_md5 = ?", fileMd5)
		if err != nil {
			return fmt.Errorf("删除审核项失败: %w", err)
		}

		// 删除版本记录
		_, err = tx.Exec("DELETE FROM versions WHERE original_md5 = ? OR version_md5 = ?", fileMd5, fileMd5)
		if err != nil {
			return fmt.Errorf("删除版本记录失败: %w", err)
		}

		// 删除处理日志
		_, err = tx.Exec("DELETE FROM processing_logs WHERE file_md5 = ?", fileMd5)
		if err != nil {
			return fmt.Errorf("删除处理日志失败: %w", err)
		}

		// 删除块修复缓存
		_, err = tx.Exec("DELETE FROM chunk_repair_cache WHERE file_md5 = ?", fileMd5)
		if err != nil {
			return fmt.Errorf("删除块修复缓存失败: %w", err)
		}

		// 删除重试队列
		_, err = tx.Exec("DELETE FROM retry_queue WHERE file_md5 = ?", fileMd5)
		if err != nil {
			return fmt.Errorf("删除重试队列失败: %w", err)
		}

		// 最后删除文件记录
		_, err = tx.Exec("DELETE FROM files WHERE md5 = ?", fileMd5)
		if err != nil {
			return fmt.Errorf("删除文件记录失败: %w", err)
		}

		return nil
	})
}
