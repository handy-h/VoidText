package database

import (
	"database/sql"
	"fmt"
	"time"
)

// FileRecord 文件记录
type FileRecord struct {
	ID                   int64  `json:"id"`
	Md5                  string `json:"md5"`
	OriginalMd5          string `json:"originalMd5"`
	Author               string `json:"author"`
	Title                string `json:"title"`
	FileName             string `json:"fileName"`
	FileSize             int64  `json:"fileSize"`
	FilePath             string `json:"filePath"`
	Status               string `json:"status"`
	CurrentStep          string `json:"currentStep"`
	Progress             int    `json:"progress"`
	RulesConfig          string `json:"rulesConfig"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
	ErrorMsg             string `json:"errorMsg"`
	LlmProgressParagraph int    `json:"llmProgressParagraph"`
	LlmProgressCheckpoint string `json:"llmProgressCheckpoint"`
	CancelFlag           int    `json:"cancelFlag"`
}

// CreateFile 创建文件记录
func CreateFile(record *FileRecord) error {
	now := time.Now().Format(time.RFC3339)
	result, err := db.Exec(`
		INSERT INTO files (md5, original_md5, author, title, file_name, file_size, file_path, status, current_step, progress, rules_config, created_at, updated_at, error_msg, llm_progress_paragraph, llm_progress_checkpoint, cancel_flag)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Md5, record.OriginalMd5, record.Author, record.Title,
		record.FileName, record.FileSize, record.FilePath,
		record.Status, record.CurrentStep, record.Progress,
		record.RulesConfig, now, now, record.ErrorMsg,
		record.LlmProgressParagraph, record.LlmProgressCheckpoint, record.CancelFlag,
	)
	if err != nil {
		return fmt.Errorf("创建文件记录失败: %w", err)
	}
	id, _ := result.LastInsertId()
	record.ID = id
	record.CreatedAt = now
	record.UpdatedAt = now
	return nil
}

// GetFileByMd5 根据MD5查询文件记录
func GetFileByMd5(md5 string) (*FileRecord, error) {
	row := db.QueryRow(`
		SELECT id, md5, original_md5, author, title, file_name, file_size, file_path, status, current_step, progress, rules_config, created_at, updated_at, error_msg, llm_progress_paragraph, llm_progress_checkpoint, cancel_flag
		FROM files WHERE md5 = ?`, md5)

	record := &FileRecord{}
	err := row.Scan(
		&record.ID, &record.Md5, &record.OriginalMd5, &record.Author, &record.Title,
		&record.FileName, &record.FileSize, &record.FilePath, &record.Status,
		&record.CurrentStep, &record.Progress, &record.RulesConfig,
		&record.CreatedAt, &record.UpdatedAt, &record.ErrorMsg,
		&record.LlmProgressParagraph, &record.LlmProgressCheckpoint, &record.CancelFlag,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询文件记录失败: %w", err)
	}
	return record, nil
}

// GetFileByID 根据ID查询文件记录
func GetFileByID(id int64) (*FileRecord, error) {
	row := db.QueryRow(`
		SELECT id, md5, original_md5, author, title, file_name, file_size, file_path, status, current_step, progress, rules_config, created_at, updated_at, error_msg, llm_progress_paragraph, llm_progress_checkpoint, cancel_flag
		FROM files WHERE id = ?`, id)

	record := &FileRecord{}
	err := row.Scan(
		&record.ID, &record.Md5, &record.OriginalMd5, &record.Author, &record.Title,
		&record.FileName, &record.FileSize, &record.FilePath, &record.Status,
		&record.CurrentStep, &record.Progress, &record.RulesConfig,
		&record.CreatedAt, &record.UpdatedAt, &record.ErrorMsg,
		&record.LlmProgressParagraph, &record.LlmProgressCheckpoint, &record.CancelFlag,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询文件记录失败: %w", err)
	}
	return record, nil
}

// UpdateFileStatus 更新文件状态
func UpdateFileStatus(md5, status, currentStep string, progress int, errorMsg string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(`
		UPDATE files SET status = ?, current_step = ?, progress = ?, error_msg = ?, updated_at = ? WHERE md5 = ?`,
		status, currentStep, progress, errorMsg, now, md5)
	if err != nil {
		return fmt.Errorf("更新文件状态失败: %w", err)
	}
	return nil
}

// UpdateFileRules 更新文件规则配置
func UpdateFileRules(md5, rulesConfig string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(`
		UPDATE files SET rules_config = ?, updated_at = ? WHERE md5 = ?`,
		rulesConfig, now, md5)
	if err != nil {
		return fmt.Errorf("更新文件规则失败: %w", err)
	}
	return nil
}

// ListPendingFiles 列出所有未完成的文件
func ListPendingFiles() ([]FileRecord, error) {
	rows, err := db.Query(`
		SELECT id, md5, original_md5, author, title, file_name, file_size, file_path, status, current_step, progress, rules_config, created_at, updated_at, error_msg, llm_progress_paragraph, llm_progress_checkpoint, cancel_flag
		FROM files WHERE status != 'completed' ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询待处理文件失败: %w", err)
	}
	defer rows.Close()

	return scanFileRows(rows)
}

// ListAllFiles 列出所有文件
func ListAllFiles() ([]FileRecord, error) {
	rows, err := db.Query(`
		SELECT id, md5, original_md5, author, title, file_name, file_size, file_path, status, current_step, progress, rules_config, created_at, updated_at, error_msg, llm_progress_paragraph, llm_progress_checkpoint, cancel_flag
		FROM files ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询文件列表失败: %w", err)
	}
	defer rows.Close()

	return scanFileRows(rows)
}

// DeleteFile 删除文件记录
func DeleteFile(md5 string) error {
	_, err := db.Exec(`DELETE FROM files WHERE md5 = ?`, md5)
	if err != nil {
		return fmt.Errorf("删除文件记录失败: %w", err)
	}
	return nil
}

func scanFileRows(rows *sql.Rows) ([]FileRecord, error) {
	var records []FileRecord
	for rows.Next() {
		var record FileRecord
		err := rows.Scan(
			&record.ID, &record.Md5, &record.OriginalMd5, &record.Author, &record.Title,
			&record.FileName, &record.FileSize, &record.FilePath, &record.Status,
			&record.CurrentStep, &record.Progress, &record.RulesConfig,
			&record.CreatedAt, &record.UpdatedAt, &record.ErrorMsg,
			&record.LlmProgressParagraph, &record.LlmProgressCheckpoint, &record.CancelFlag,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描文件记录失败: %w", err)
		}
		records = append(records, record)
	}
	return records, nil
}

// UpdateLlmProgress 更新LLM修复进度（断点恢复用）
func UpdateLlmProgress(md5 string, paragraphIndex int, checkpoint string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(`
		UPDATE files SET llm_progress_paragraph = ?, llm_progress_checkpoint = ?, updated_at = ? WHERE md5 = ?`,
		paragraphIndex, checkpoint, now, md5)
	if err != nil {
		return fmt.Errorf("更新LLM进度失败: %w", err)
	}
	return nil
}

// SetCancelFlag 设置文件取消标志
func SetCancelFlag(md5 string, flag int) error {
	_, err := db.Exec(`UPDATE files SET cancel_flag = ? WHERE md5 = ?`, flag, md5)
	return err
}

// IsFileCancelled 检查文件是否已被取消
func IsFileCancelled(md5 string) (bool, error) {
	var flag int
	err := db.QueryRow(`SELECT cancel_flag FROM files WHERE md5 = ?`, md5).Scan(&flag)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return flag == 1, nil
}
