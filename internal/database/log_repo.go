package database

import (
	"database/sql"
	"fmt"
	"time"
)

// ProcessingLogRecord 处理日志记录
type ProcessingLogRecord struct {
	ID        int64  `json:"id"`
	FileMd5   string `json:"fileMd5"`
	Step      string `json:"step"`
	Action    string `json:"action"`
	Details   string `json:"details"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// CreateProcessingLog 创建处理日志
func CreateProcessingLog(record *ProcessingLogRecord) error {
	now := time.Now().Format(time.RFC3339)
	result, err := db.Exec(`
		INSERT INTO processing_logs (file_md5, step, action, details, status, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)`,
		record.FileMd5, record.Step, record.Action, record.Details, record.Status, now)
	if err != nil {
		return fmt.Errorf("创建处理日志失败: %w", err)
	}
	id, _ := result.LastInsertId()
	record.ID = id
	record.Timestamp = now
	return nil
}

// GetProcessingLogsByFileMd5 查询文件的处理日志
func GetProcessingLogsByFileMd5(fileMd5 string) ([]ProcessingLogRecord, error) {
	rows, err := db.Query(`
		SELECT id, file_md5, step, action, details, status, timestamp
		FROM processing_logs WHERE file_md5 = ? ORDER BY timestamp ASC`, fileMd5)
	if err != nil {
		return nil, fmt.Errorf("查询处理日志失败: %w", err)
	}
	defer rows.Close()

	var records []ProcessingLogRecord
	for rows.Next() {
		var record ProcessingLogRecord
		err := rows.Scan(
			&record.ID, &record.FileMd5, &record.Step, &record.Action,
			&record.Details, &record.Status, &record.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描处理日志失败: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("行迭代错误: %w", err)
	}
	return records, nil
}

// GetLatestProcessingLog 获取文件最新的处理日志
func GetLatestProcessingLog(fileMd5 string) (*ProcessingLogRecord, error) {
	row := db.QueryRow(`
		SELECT id, file_md5, step, action, details, status, timestamp
		FROM processing_logs WHERE file_md5 = ? ORDER BY id DESC LIMIT 1`, fileMd5)

	var record ProcessingLogRecord
	err := row.Scan(
		&record.ID, &record.FileMd5, &record.Step, &record.Action,
		&record.Details, &record.Status, &record.Timestamp,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询最新处理日志失败: %w", err)
	}
	return &record, nil
}

// GetRecentProcessingLogs 获取文件最近N条处理日志（按时间倒序）
func GetRecentProcessingLogs(fileMd5 string, limit int) ([]ProcessingLogRecord, error) {
	rows, err := db.Query(`
		SELECT id, file_md5, step, action, details, status, timestamp
		FROM processing_logs WHERE file_md5 = ? ORDER BY timestamp DESC LIMIT ?`, fileMd5, limit)
	if err != nil {
		return nil, fmt.Errorf("查询最近处理日志失败: %w", err)
	}
	defer rows.Close()

	var records []ProcessingLogRecord
	for rows.Next() {
		var record ProcessingLogRecord
		err := rows.Scan(
			&record.ID, &record.FileMd5, &record.Step, &record.Action,
			&record.Details, &record.Status, &record.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描处理日志失败: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("行迭代错误: %w", err)
	}
	return records, nil
}
