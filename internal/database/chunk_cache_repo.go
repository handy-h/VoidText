package database

import (
	"database/sql"
	"fmt"
	"time"
)

// Table names
const (
	ChunkRepairCacheTable = "chunk_repair_cache"
)

// Time windows for statistics
const (
	statsTimeWindow = "24 hours" // 24小时统计窗口
)

// Default retention days
const (
	defaultCacheRetentionDays = 30 // 默认缓存保留30天
)

// ChunkRepairCacheRecord 块修复缓存记录（混合架构版）
type ChunkRepairCacheRecord struct {
	ID               int64     `json:"id"`
	FileMd5          string    `json:"file_md5"`
	ChunkID          int       `json:"chunk_id"`
	ChunkHash        string    `json:"chunk_hash"`
	OriginalText     string    `json:"original_text"`
	RepairedText     string    `json:"repaired_text"`
	PromptVersion    string    `json:"prompt_version"`
	APIModel         string    `json:"api_model"`
	TokenUsage       int       `json:"token_usage"`
	ProcessingTimeMs int       `json:"processing_time_ms"`
	Confidence       float64   `json:"confidence"` // 处理结果置信度
	Source           string    `json:"source"`     // 处理来源：local/remote/cache
	CreatedAt        time.Time `json:"created_at"`
}

// RetryQueueRecord 重试队列记录
type RetryQueueRecord struct {
	ID            int64     `json:"id"`
	FileMd5       string    `json:"file_md5"`
	ChunkID       int       `json:"chunk_id"`
	OriginalText  string    `json:"original_text"`
	FailureReason string    `json:"failure_reason"`
	ErrorType     string    `json:"error_type"`
	ErrorContext  string    `json:"error_context"`
	PromptVersion string    `json:"prompt_version"`
	RetryCount    int       `json:"retry_count"`
	MaxRetries    int       `json:"max_retries"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	NextRetryAt   time.Time `json:"next_retry_at"`
}

// PromptVersionRecord 提示词版本记录
type PromptVersionRecord struct {
	ID             int64     `json:"id"`
	PromptName     string    `json:"prompt_name"`
	PromptVersion  string    `json:"prompt_version"`
	PromptContent  string    `json:"prompt_content"`
	Source         string    `json:"source"`
	ErrorPattern   string    `json:"error_pattern"`
	SuccessRate    float64   `json:"success_rate"`
	TotalUses      int       `json:"total_uses"`
	SuccessfulUses int       `json:"successful_uses"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ChunkCacheRepo 块缓存仓库
type ChunkCacheRepo struct {
	db *sql.DB
}

// NewChunkCacheRepo 创建块缓存仓库
func NewChunkCacheRepo() *ChunkCacheRepo {
	return &ChunkCacheRepo{db: GetDB()}
}

// SaveChunkRepair 保存块修复结果到缓存
func (r *ChunkCacheRepo) SaveChunkRepair(record *ChunkRepairCacheRecord) error {
	stmt := fmt.Sprintf(`INSERT OR REPLACE INTO %s
		(file_md5, chunk_id, chunk_hash, original_text, repaired_text, prompt_version, api_model, token_usage, processing_time_ms, confidence, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, ChunkRepairCacheTable)

	_, err := r.db.Exec(stmt,
		record.FileMd5,
		record.ChunkID,
		record.ChunkHash,
		record.OriginalText,
		record.RepairedText,
		record.PromptVersion,
		record.APIModel,
		record.TokenUsage,
		record.ProcessingTimeMs,
		record.Confidence,
		record.Source,
	)
	if err != nil {
		return fmt.Errorf("插入块修复缓存失败: %w", err)
	}
	return nil
}

// GetChunkRepair 根据块哈希获取缓存修复结果
// 使用查询缓存提高性能，减少重复API调用
func (r *ChunkCacheRepo) GetChunkRepair(fileMd5, chunkHash string) (*ChunkRepairCacheRecord, error) {
	stmt := fmt.Sprintf(`SELECT id, file_md5, chunk_id, chunk_hash, original_text, repaired_text, 
		prompt_version, api_model, token_usage, processing_time_ms, confidence, source, created_at
		FROM %s WHERE file_md5 = ? AND chunk_hash = ?`, ChunkRepairCacheTable)

	row := r.db.QueryRow(stmt, fileMd5, chunkHash)

	var record ChunkRepairCacheRecord
	err := row.Scan(
		&record.ID,
		&record.FileMd5,
		&record.ChunkID,
		&record.ChunkHash,
		&record.OriginalText,
		&record.RepairedText,
		&record.PromptVersion,
		&record.APIModel,
		&record.TokenUsage,
		&record.ProcessingTimeMs,
		&record.Confidence,
		&record.Source,
		&record.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询块修复缓存失败: %w", err)
	}

	return &record, nil
}

// GetUnfinishedChunks 获取未完成的块（用于断点续传）
// 通过比较已缓存块和总块数，识别需要继续处理的块
func (r *ChunkCacheRepo) GetUnfinishedChunks(fileMd5 string, totalChunks int) ([]int, error) {
	stmt := fmt.Sprintf(`SELECT chunk_id FROM %s WHERE file_md5 = ? ORDER BY chunk_id`, ChunkRepairCacheTable)
	rows, err := r.db.Query(stmt, fileMd5)
	if err != nil {
		return nil, fmt.Errorf("查询已缓存块失败: %w", err)
	}
	defer rows.Close()

	cachedChunks := make(map[int]bool)
	for rows.Next() {
		var chunkID int
		if err := rows.Scan(&chunkID); err != nil {
			return nil, fmt.Errorf("扫描块ID失败: %w", err)
		}
		cachedChunks[chunkID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("行迭代错误: %w", err)
	}

	// 找出未完成的块
	unfinished := []int{}
	for i := 0; i < totalChunks; i++ {
		if !cachedChunks[i] {
			unfinished = append(unfinished, i)
		}
	}

	return unfinished, nil
}

// AddToRetryQueue 添加失败块到重试队列
// 支持指数退避重试策略，避免频繁重试
func (r *ChunkCacheRepo) AddToRetryQueue(record *RetryQueueRecord) error {
	// 设置下次重试时间（指数退避：2^retryCount 分钟）
	backoffMinutes := 1 << record.RetryCount
	if backoffMinutes > 60 {
		backoffMinutes = 60 // 最大退避1小时
	}
	nextRetryAt := time.Now().Add(time.Duration(backoffMinutes) * time.Minute)

	stmt := `INSERT INTO retry_queue 
		(file_md5, chunk_id, original_text, failure_reason, error_type, error_context, 
		prompt_version, retry_count, max_retries, status, next_retry_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`

	_, err := r.db.Exec(stmt,
		record.FileMd5,
		record.ChunkID,
		record.OriginalText,
		record.FailureReason,
		record.ErrorType,
		record.ErrorContext,
		record.PromptVersion,
		record.RetryCount,
		record.MaxRetries,
		nextRetryAt,
	)

	if err != nil {
		return fmt.Errorf("添加到重试队列失败: %w", err)
	}

	return nil
}

// GetPendingRetries 获取待处理的重试任务
// 只获取已到重试时间的任务，避免过早重试
func (r *ChunkCacheRepo) GetPendingRetries(limit int) ([]RetryQueueRecord, error) {
	stmt := `SELECT id, file_md5, chunk_id, original_text, failure_reason, error_type, 
		error_context, prompt_version, retry_count, max_retries, status, created_at, 
		updated_at, next_retry_at
		FROM retry_queue 
		WHERE status = 'pending' AND next_retry_at <= ?
		ORDER BY next_retry_at ASC
		LIMIT ?`

	rows, err := r.db.Query(stmt, time.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("查询待处理重试失败: %w", err)
	}
	defer rows.Close()

	var records []RetryQueueRecord
	for rows.Next() {
		var record RetryQueueRecord
		err := rows.Scan(
			&record.ID,
			&record.FileMd5,
			&record.ChunkID,
			&record.OriginalText,
			&record.FailureReason,
			&record.ErrorType,
			&record.ErrorContext,
			&record.PromptVersion,
			&record.RetryCount,
			&record.MaxRetries,
			&record.Status,
			&record.CreatedAt,
			&record.UpdatedAt,
			&record.NextRetryAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描重试记录失败: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("行迭代错误: %w", err)
	}

	return records, nil
}

// UpdateRetryStatus 更新重试任务状态
func (r *ChunkCacheRepo) UpdateRetryStatus(id int64, status string, retryCount int) error {
	_, err := r.db.Exec(
		`UPDATE retry_queue SET status = ?, retry_count = ?, updated_at = ? WHERE id = ?`,
		status, retryCount, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("更新重试状态失败: %w", err)
	}
	return nil
}

// SavePromptVersion 保存提示词版本
// 用于Evolver自进化，记录不同版本的提示词效果
func (r *ChunkCacheRepo) SavePromptVersion(record *PromptVersionRecord) error {
	stmt := `INSERT OR REPLACE INTO prompt_versions 
		(prompt_name, prompt_version, prompt_content, source, error_pattern, 
		success_rate, total_uses, successful_uses)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(stmt,
		record.PromptName,
		record.PromptVersion,
		record.PromptContent,
		record.Source,
		record.ErrorPattern,
		record.SuccessRate,
		record.TotalUses,
		record.SuccessfulUses,
	)

	if err != nil {
		return fmt.Errorf("保存提示词版本失败: %w", err)
	}

	return nil
}

// GetLatestPromptVersion 获取最新的提示词版本
// 优先返回成功率高、使用次数多的版本
func (r *ChunkCacheRepo) GetLatestPromptVersion(promptName string) (*PromptVersionRecord, error) {
	stmt := `SELECT id, prompt_name, prompt_version, prompt_content, source, error_pattern,
		success_rate, total_uses, successful_uses, created_at, updated_at
		FROM prompt_versions 
		WHERE prompt_name = ?
		ORDER BY 
			CASE WHEN total_uses > 0 THEN successful_uses * 1.0 / total_uses ELSE 0 END DESC,
			total_uses DESC,
			updated_at DESC
		LIMIT 1`

	row := r.db.QueryRow(stmt, promptName)

	var record PromptVersionRecord
	err := row.Scan(
		&record.ID,
		&record.PromptName,
		&record.PromptVersion,
		&record.PromptContent,
		&record.Source,
		&record.ErrorPattern,
		&record.SuccessRate,
		&record.TotalUses,
		&record.SuccessfulUses,
		&record.CreatedAt,
		&record.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询提示词版本失败: %w", err)
	}

	return &record, nil
}

// UpdatePromptUsage 更新提示词使用统计
// 用 UPSERT 原子更新，避免先读后写导致 error_pattern 等字段被 INSERT OR REPLACE 清空
func (r *ChunkCacheRepo) UpdatePromptUsage(promptName, promptVersion string, success bool) error {
	successIncrement := 0
	if success {
		successIncrement = 1
	}
	now := time.Now()

	// ON CONFLICT DO UPDATE 只修改计数字段，保留 error_pattern、created_at 等其余字段不变
	stmt := `
		INSERT INTO prompt_versions
			(prompt_name, prompt_version, prompt_content, source,
			 total_uses, successful_uses, success_rate, updated_at)
		VALUES (?, ?, '', 'file', 1, ?, ?, ?)
		ON CONFLICT(prompt_name, prompt_version) DO UPDATE SET
			total_uses      = total_uses + 1,
			successful_uses = successful_uses + excluded.successful_uses,
			success_rate    = CAST(successful_uses + excluded.successful_uses AS REAL)
			                  / (total_uses + 1),
			updated_at      = excluded.updated_at`

	_, err := r.db.Exec(stmt,
		promptName, promptVersion,
		successIncrement,
		float64(successIncrement),
		now,
	)
	if err != nil {
		return fmt.Errorf("更新提示词使用统计失败: %w", err)
	}
	return nil
}

// GetCacheHitRate 获取缓存命中率（百分比）

// DeleteOldCache 删除旧的缓存记录（清理策略）
// 保留最近30天的缓存，避免数据库无限增长
func (r *ChunkCacheRepo) DeleteOldCache(daysToKeep int) error {
	cutoffTime := time.Now().AddDate(0, 0, -daysToKeep)

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	// 删除旧的块修复缓存
	_, err = tx.Exec(fmt.Sprintf(`DELETE FROM %s WHERE created_at < ?`, ChunkRepairCacheTable), cutoffTime)
	if err != nil {
		return fmt.Errorf("删除旧块修复缓存失败: %w", err)
	}

	// 删除已完成的重试记录
	_, err = tx.Exec(`DELETE FROM retry_queue WHERE status IN ('completed', 'failed') AND updated_at < ?`, cutoffTime)
	if err != nil {
		return fmt.Errorf("删除旧重试记录失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

// GetCacheHitRate 获取缓存命中率（百分比）
// 计算最近24小时内的缓存命中率：缓存命中次数 / 总查询次数
// 简化实现：假设有统计表，这里返回一个估计值
func (r *ChunkCacheRepo) GetCacheHitRate() (float64, error) {
	// 查询最近24小时内的缓存命中次数
	var hitCount int
	row := r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s 
		WHERE created_at >= datetime('now', '-%s')`, ChunkRepairCacheTable, statsTimeWindow))
	if err := row.Scan(&hitCount); err != nil {
		return 0.0, fmt.Errorf("查询缓存命中次数失败: %w", err)
	}

	// 查询最近24小时内的总查询次数（估算）
	// 这里我们使用缓存记录数 + 重试记录数作为总查询次数的近似值
	var retryCount int
	row = r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM retry_queue 
		WHERE created_at >= datetime('now', '-%s')`, statsTimeWindow))
	if err := row.Scan(&retryCount); err != nil {
		return 0.0, fmt.Errorf("查询重试次数失败: %w", err)
	}

	totalQueries := hitCount + retryCount
	if totalQueries == 0 {
		return 0.0, nil // 没有查询，命中率为0
	}

	hitRate := float64(hitCount) / float64(totalQueries) * 100.0
	return hitRate, nil
}

// GetErrorRate 获取API错误率（百分比）
// 计算最近24小时内的错误率：错误次数 / 总请求次数
func (r *ChunkCacheRepo) GetErrorRate() (float64, error) {
	// 查询最近24小时内的错误次数（重试队列中的记录）
	var errorCount int
	row := r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM retry_queue 
		WHERE created_at >= datetime('now', '-%s')`, statsTimeWindow))
	if err := row.Scan(&errorCount); err != nil {
		return 0.0, fmt.Errorf("查询错误次数失败: %w", err)
	}

	// 查询最近24小时内的总请求次数（缓存命中 + 错误）
	var hitCount int
	row = r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s
		WHERE created_at >= datetime('now', '-%s')`, ChunkRepairCacheTable, statsTimeWindow))
	if err := row.Scan(&hitCount); err != nil {
		return 0.0, fmt.Errorf("查询缓存命中次数失败: %w", err)
	}

	totalRequests := hitCount + errorCount
	if totalRequests == 0 {
		return 0.0, nil // 没有请求，错误率为0
	}

	errorRate := float64(errorCount) / float64(totalRequests) * 100.0
	return errorRate, nil
}
