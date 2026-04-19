package database

import (
	"database/sql"
	"fmt"
	"time"
)

// ReviewItemRecord 审核项记录
type ReviewItemRecord struct {
	ID               int64   `json:"id"`
	FileMd5          string  `json:"fileMd5"`
	OriginalText     string  `json:"originalText"`
	SuggestedText    string  `json:"suggestedText"`
	ModificationType string  `json:"modificationType"`
	Confidence       float64 `json:"confidence"`
	PositionStart    int     `json:"positionStart"`
	PositionEnd      int     `json:"positionEnd"`
	Status           string  `json:"status"`
	EditedText       string  `json:"editedText"`
	CreatedAt        string  `json:"createdAt"`
	ResolvedAt       *string `json:"resolvedAt"`
}

// CreateReviewItems 批量创建审核项
func CreateReviewItems(items []ReviewItemRecord) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO review_items (file_md5, original_text, suggested_text, modification_type, confidence, position_start, position_end, status, edited_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("准备插入语句失败: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Format(time.RFC3339)
	for _, item := range items {
		_, err := stmt.Exec(
			item.FileMd5, item.OriginalText, item.SuggestedText,
			item.ModificationType, item.Confidence, item.PositionStart,
			item.PositionEnd, item.Status, item.EditedText, now,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("插入审核项失败: %w", err)
		}
	}

	return tx.Commit()
}

// GetReviewItemsByFileMd5 根据文件MD5查询审核项
func GetReviewItemsByFileMd5(fileMd5 string, statusFilter string) ([]ReviewItemRecord, error) {
	var rows *sql.Rows
	var err error

	if statusFilter != "" && statusFilter != "all" {
		rows, err = db.Query(`
			SELECT id, file_md5, original_text, suggested_text, modification_type, confidence, position_start, position_end, status, edited_text, created_at, resolved_at
			FROM review_items WHERE file_md5 = ? AND status = ? ORDER BY position_start ASC`, fileMd5, statusFilter)
	} else {
		rows, err = db.Query(`
			SELECT id, file_md5, original_text, suggested_text, modification_type, confidence, position_start, position_end, status, edited_text, created_at, resolved_at
			FROM review_items WHERE file_md5 = ? ORDER BY position_start ASC`, fileMd5)
	}

	if err != nil {
		return nil, fmt.Errorf("查询审核项失败: %w", err)
	}
	defer rows.Close()

	return scanReviewItemRows(rows)
}

// GetReviewItemByID 根据ID查询审核项
func GetReviewItemByID(id int64) (*ReviewItemRecord, error) {
	row := db.QueryRow(`
		SELECT id, file_md5, original_text, suggested_text, modification_type, confidence, position_start, position_end, status, edited_text, created_at, resolved_at
		FROM review_items WHERE id = ?`, id)

	record := &ReviewItemRecord{}
	err := row.Scan(
		&record.ID, &record.FileMd5, &record.OriginalText, &record.SuggestedText,
		&record.ModificationType, &record.Confidence, &record.PositionStart,
		&record.PositionEnd, &record.Status, &record.EditedText,
		&record.CreatedAt, &record.ResolvedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询审核项失败: %w", err)
	}
	return record, nil
}

// UpdateReviewItemStatus 更新审核项状态
func UpdateReviewItemStatus(id int64, status string, editedText string) error {
	now := time.Now().Format(time.RFC3339)

	if status == "approved" || status == "rejected" || status == "edited" {
		_, err := db.Exec(`
			UPDATE review_items SET status = ?, edited_text = ?, resolved_at = ? WHERE id = ?`,
			status, editedText, now, id)
		if err != nil {
			return fmt.Errorf("更新审核项状态失败: %w", err)
		}
	} else {
		_, err := db.Exec(`
			UPDATE review_items SET status = ?, edited_text = ?, resolved_at = NULL WHERE id = ?`,
			status, editedText, id)
		if err != nil {
			return fmt.Errorf("更新审核项状态失败: %w", err)
		}
	}
	return nil
}

// BatchUpdateReviewItemStatus 批量更新审核项状态
func BatchUpdateReviewItemStatus(ids []int64, status string) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	now := time.Now().Format(time.RFC3339)
	stmt, err := tx.Prepare(`
		UPDATE review_items SET status = ?, resolved_at = ? WHERE id = ?`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("准备更新语句失败: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		if status == "approved" || status == "rejected" {
			_, err = stmt.Exec(status, now, id)
		} else {
			_, err = stmt.Exec(status, nil, id)
		}
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("批量更新审核项失败: %w", err)
		}
	}

	return tx.Commit()
}

// GetReviewProgress 获取审核进度
func GetReviewProgress(fileMd5 string) (total int, resolved int, err error) {
	row := db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN status IN ('approved', 'rejected', 'edited') THEN 1 ELSE 0 END), 0)
		FROM review_items WHERE file_md5 = ?`, fileMd5)

	err = row.Scan(&total, &resolved)
	if err != nil {
		return 0, 0, fmt.Errorf("查询审核进度失败: %w", err)
	}
	return total, resolved, nil
}

// DeleteReviewItemsByFileMd5 删除文件的所有审核项
func DeleteReviewItemsByFileMd5(fileMd5 string) error {
	_, err := db.Exec(`DELETE FROM review_items WHERE file_md5 = ?`, fileMd5)
	if err != nil {
		return fmt.Errorf("删除审核项失败: %w", err)
	}
	return nil
}

func scanReviewItemRows(rows *sql.Rows) ([]ReviewItemRecord, error) {
	var records []ReviewItemRecord
	for rows.Next() {
		var record ReviewItemRecord
		err := rows.Scan(
			&record.ID, &record.FileMd5, &record.OriginalText, &record.SuggestedText,
			&record.ModificationType, &record.Confidence, &record.PositionStart,
			&record.PositionEnd, &record.Status, &record.EditedText,
			&record.CreatedAt, &record.ResolvedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描审核项记录失败: %w", err)
		}
		records = append(records, record)
	}
	return records, nil
}
