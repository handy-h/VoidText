package database

import (
	"database/sql"
	"fmt"
)

// VersionRecord 版本记录
type VersionRecord struct {
	ID          int64  `json:"id"`
	OriginalMd5 string `json:"originalMd5"`
	VersionMd5  string `json:"versionMd5"`
	ParentMd5   string `json:"parentMd5"`
	VersionType string `json:"versionType"`
	FilePath    string `json:"filePath"`
	Step        string `json:"step"`
	CreatedAt   string `json:"createdAt"`
}

// CreateVersion 创建版本记录
func CreateVersion(record *VersionRecord) error {
	result, err := db.Exec(`
		INSERT INTO versions (original_md5, version_md5, parent_md5, version_type, file_path, step)
		VALUES (?, ?, ?, ?, ?, ?)`,
		record.OriginalMd5, record.VersionMd5, record.ParentMd5,
		record.VersionType, record.FilePath, record.Step)
	if err != nil {
		return fmt.Errorf("创建版本记录失败: %w", err)
	}
	id, _ := result.LastInsertId()
	record.ID = id
	return nil
}

// GetVersionByMd5 根据版本MD5查询版本记录
func GetVersionByMd5(versionMd5 string) (*VersionRecord, error) {
	row := db.QueryRow(`
		SELECT id, original_md5, version_md5, parent_md5, version_type, file_path, step, created_at
		FROM versions WHERE version_md5 = ?`, versionMd5)

	record := &VersionRecord{}
	err := row.Scan(
		&record.ID, &record.OriginalMd5, &record.VersionMd5, &record.ParentMd5,
		&record.VersionType, &record.FilePath, &record.Step, &record.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询版本记录失败: %w", err)
	}
	return record, nil
}

// GetVersionsByOriginalMd5 查询原始文件的所有版本
func GetVersionsByOriginalMd5(originalMd5 string) ([]VersionRecord, error) {
	rows, err := db.Query(`
		SELECT id, original_md5, version_md5, parent_md5, version_type, file_path, step, created_at
		FROM versions WHERE original_md5 = ? ORDER BY created_at ASC`, originalMd5)
	if err != nil {
		return nil, fmt.Errorf("查询版本链失败: %w", err)
	}
	defer rows.Close()

	return scanVersionRows(rows)
}

// GetLatestVersion 获取原始文件的最新版本
func GetLatestVersion(originalMd5 string) (*VersionRecord, error) {
	row := db.QueryRow(`
		SELECT id, original_md5, version_md5, parent_md5, version_type, file_path, step, created_at
		FROM versions WHERE original_md5 = ? ORDER BY created_at DESC LIMIT 1`, originalMd5)

	record := &VersionRecord{}
	err := row.Scan(
		&record.ID, &record.OriginalMd5, &record.VersionMd5, &record.ParentMd5,
		&record.VersionType, &record.FilePath, &record.Step, &record.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询最新版本失败: %w", err)
	}
	return record, nil
}

// TraceOriginalMd5 通过版本链追溯到原始文件MD5
func TraceOriginalMd5(versionMd5 string) (string, error) {
	version, err := GetVersionByMd5(versionMd5)
	if err != nil {
		return "", err
	}
	if version == nil {
		return "", nil
	}
	return version.OriginalMd5, nil
}

func scanVersionRows(rows *sql.Rows) ([]VersionRecord, error) {
	var records []VersionRecord
	for rows.Next() {
		var record VersionRecord
		err := rows.Scan(
			&record.ID, &record.OriginalMd5, &record.VersionMd5, &record.ParentMd5,
			&record.VersionType, &record.FilePath, &record.Step, &record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描版本记录失败: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("行迭代错误: %w", err)
	}
	return records, nil
}
