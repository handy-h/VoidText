package database

import (
	"database/sql"
	"fmt"
	"time"
)

// ReviewParagraphRecord 段级审核记录
// 替代 ReviewItemRecord，使用 paragraph_index 替代字节偏移定位
type ReviewParagraphRecord struct {
	ID               int64   `json:"id"`
	FileMd5          string  `json:"fileMd5"`
	ParagraphIndex   int     `json:"paragraphIndex"`
	OriginalText     string  `json:"originalText"`
	SuggestedText    string  `json:"suggestedText"`
	ModificationType string  `json:"modificationType"`
	Status           string  `json:"status"`
	EditedText       string  `json:"editedText"`
	CreatedAt        string  `json:"createdAt"`
	ResolvedAt       *string `json:"resolvedAt"`
}

// CreateReviewParagraphs 批量创建段级审核记录
func CreateReviewParagraphs(records []ReviewParagraphRecord) error {
	if len(records) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO review_paragraphs
		(file_md5, paragraph_index, original_text, suggested_text, modification_type, status, edited_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("准备插入语句失败: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Format(time.RFC3339)
	for _, r := range records {
		status := r.Status
		if status == "" {
			status = "pending"
		}
		_, err := stmt.Exec(
			r.FileMd5, r.ParagraphIndex, r.OriginalText, r.SuggestedText,
			r.ModificationType, status, r.EditedText, now,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("插入段级审核记录失败: %w", err)
		}
	}

	return tx.Commit()
}

// GetReviewParagraphsByFileMd5 按文件 MD5 查询段级审核记录，按 paragraph_index 升序
func GetReviewParagraphsByFileMd5(fileMd5, statusFilter string) ([]ReviewParagraphRecord, error) {
	var rows *sql.Rows
	var err error

	if statusFilter != "" && statusFilter != "all" {
		rows, err = db.Query(`
			SELECT id, file_md5, paragraph_index, original_text, suggested_text, modification_type, status, edited_text, created_at, resolved_at
			FROM review_paragraphs WHERE file_md5 = ? AND status = ? ORDER BY paragraph_index ASC`, fileMd5, statusFilter)
	} else {
		rows, err = db.Query(`
			SELECT id, file_md5, paragraph_index, original_text, suggested_text, modification_type, status, edited_text, created_at, resolved_at
			FROM review_paragraphs WHERE file_md5 = ? ORDER BY paragraph_index ASC`, fileMd5)
	}

	if err != nil {
		return nil, fmt.Errorf("查询段级审核记录失败: %w", err)
	}
	defer rows.Close()

	return scanReviewParagraphRows(rows)
}

// GetReviewParagraphByID 按 ID 查询段级审核记录
func GetReviewParagraphByID(id int64) (*ReviewParagraphRecord, error) {
	row := db.QueryRow(`
		SELECT id, file_md5, paragraph_index, original_text, suggested_text, modification_type, status, edited_text, created_at, resolved_at
		FROM review_paragraphs WHERE id = ?`, id)

	r := &ReviewParagraphRecord{}
	err := row.Scan(
		&r.ID, &r.FileMd5, &r.ParagraphIndex, &r.OriginalText, &r.SuggestedText,
		&r.ModificationType, &r.Status, &r.EditedText, &r.CreatedAt, &r.ResolvedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询段级审核记录失败: %w", err)
	}
	return r, nil
}

// UpdateReviewParagraphStatus 更新段级审核记录的状态与编辑文本
func UpdateReviewParagraphStatus(id int64, status, editedText string) error {
	now := time.Now().Format(time.RFC3339)
	if status == "approved" || status == "rejected" || status == "edited" {
		_, err := db.Exec(`
			UPDATE review_paragraphs SET status = ?, edited_text = ?, resolved_at = ? WHERE id = ?`,
			status, editedText, now, id)
		if err != nil {
			return fmt.Errorf("更新段级审核记录失败: %w", err)
		}
		return nil
	}
	_, err := db.Exec(`
		UPDATE review_paragraphs SET status = ?, edited_text = ?, resolved_at = NULL WHERE id = ?`,
		status, editedText, id)
	if err != nil {
		return fmt.Errorf("更新段级审核记录失败: %w", err)
	}
	return nil
}

// BatchUpdateReviewParagraphStatus 把指定文件下所有 pending 段一次性置为给定状态
func BatchUpdateReviewParagraphStatus(fileMd5, status string) error {
	now := time.Now().Format(time.RFC3339)
	if status == "approved" || status == "rejected" {
		_, err := db.Exec(`
			UPDATE review_paragraphs SET status = ?, resolved_at = ?
			WHERE file_md5 = ? AND status = 'pending'`, status, now, fileMd5)
		if err != nil {
			return fmt.Errorf("批量更新段级审核记录失败: %w", err)
		}
		return nil
	}
	_, err := db.Exec(`
		UPDATE review_paragraphs SET status = ?, resolved_at = NULL
		WHERE file_md5 = ? AND status = 'pending'`, status, fileMd5)
	if err != nil {
		return fmt.Errorf("批量更新段级审核记录失败: %w", err)
	}
	return nil
}

// GetReviewParagraphProgress 返回 (总数, 已解决数)
func GetReviewParagraphProgress(fileMd5 string) (int, int, error) {
	var total, resolved int
	err := db.QueryRow(`SELECT COUNT(*) FROM review_paragraphs WHERE file_md5 = ?`, fileMd5).Scan(&total)
	if err != nil {
		return 0, 0, fmt.Errorf("查询总数失败: %w", err)
	}
	err = db.QueryRow(`
		SELECT COUNT(*) FROM review_paragraphs
		WHERE file_md5 = ? AND status IN ('approved','rejected','edited')`, fileMd5).Scan(&resolved)
	if err != nil {
		return 0, 0, fmt.Errorf("查询已解决数失败: %w", err)
	}
	return total, resolved, nil
}

// DeleteReviewParagraphsByFileMd5 删除指定文件的全部段级审核记录
func DeleteReviewParagraphsByFileMd5(fileMd5 string) error {
	_, err := db.Exec(`DELETE FROM review_paragraphs WHERE file_md5 = ?`, fileMd5)
	if err != nil {
		return fmt.Errorf("删除段级审核记录失败: %w", err)
	}
	return nil
}

func scanReviewParagraphRows(rows *sql.Rows) ([]ReviewParagraphRecord, error) {
	var records []ReviewParagraphRecord
	for rows.Next() {
		r := ReviewParagraphRecord{}
		err := rows.Scan(
			&r.ID, &r.FileMd5, &r.ParagraphIndex, &r.OriginalText, &r.SuggestedText,
			&r.ModificationType, &r.Status, &r.EditedText, &r.CreatedAt, &r.ResolvedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描段级审核记录失败: %w", err)
		}
		records = append(records, r)
	}
	return records, nil
}
