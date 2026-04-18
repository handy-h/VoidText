package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"txt-cleaning/internal/config"
)

// Version 版本信息
type Version struct {
	ID        string    `json:"id"`
	FileID    string    `json:"fileId"`
	Version   string    `json:"version"`
	Content   string    `json:"content"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
	Note      string    `json:"note"`
}

// VersionManager 版本管理器
type VersionManager struct {
	versions map[string][]Version
}

// NewVersionManager 创建新的版本管理器
func NewVersionManager() *VersionManager {
	return &VersionManager{
		versions: make(map[string][]Version),
	}
}

// CreateVersion 创建新版本
func (vm *VersionManager) CreateVersion(fileID, content, note string) (Version, error) {
	// 获取文件当前版本号
	currentVersions, err := vm.GetVersions(fileID)
	if err != nil {
		currentVersions = []Version{}
	}

	// 生成新版本号
	version := string(len(currentVersions) + 1)

	// 创建版本
	newVersion := Version{
		ID:        fileID + "_v" + version,
		FileID:    fileID,
		Version:   version,
		Content:   content,
		Size:      int64(len(content)),
		CreatedAt: time.Now(),
		Note:      note,
	}

	// 保存版本
	err = vm.saveVersion(newVersion)
	if err != nil {
		return Version{}, err
	}

	// 更新内存中的版本列表
	currentVersions = append(currentVersions, newVersion)
	vm.versions[fileID] = currentVersions

	return newVersion, nil
}

// GetVersions 获取文件的所有版本
func (vm *VersionManager) GetVersions(fileID string) ([]Version, error) {
	// 从内存中获取
	if versions, exists := vm.versions[fileID]; exists {
		return versions, nil
	}

	// 从磁盘加载
	versions, err := vm.loadVersions(fileID)
	if err != nil {
		return nil, err
	}

	// 保存到内存
	vm.versions[fileID] = versions

	return versions, nil
}

// GetVersion 获取特定版本
func (vm *VersionManager) GetVersion(fileID, version string) (Version, error) {
	// 获取所有版本
	versions, err := vm.GetVersions(fileID)
	if err != nil {
		return Version{}, err
	}

	// 查找特定版本
	for _, v := range versions {
		if v.Version == version {
			return v, nil
		}
	}

	return Version{}, os.ErrNotExist
}

// RestoreVersion 恢复到特定版本
func (vm *VersionManager) RestoreVersion(fileID, version string) (string, error) {
	// 获取特定版本
	targetVersion, err := vm.GetVersion(fileID, version)
	if err != nil {
		return "", err
	}

	// 保存恢复操作
	_, err = vm.CreateVersion(fileID, targetVersion.Content, "恢复到版本 " + version)
	if err != nil {
		return "", err
	}

	return targetVersion.Content, nil
}

// DeleteVersion 删除特定版本
func (vm *VersionManager) DeleteVersion(fileID, version string) error {
	// 获取所有版本
	versions, err := vm.GetVersions(fileID)
	if err != nil {
		return err
	}

	// 查找并删除版本
	var newVersions []Version
	for _, v := range versions {
		if v.Version != version {
			newVersions = append(newVersions, v)
		} else {
			// 删除版本文件
			versionPath := filepath.Join(config.AppConfig.DataDir, "backups", v.ID+'.json')
			os.Remove(versionPath)
		}
	}

	// 更新内存中的版本列表
	vm.versions[fileID] = newVersions

	return nil
}

// CleanupOldVersions 清理旧版本
func (vm *VersionManager) CleanupOldVersions(fileID string, keepDays int) error {
	// 获取所有版本
	versions, err := vm.GetVersions(fileID)
	if err != nil {
		return err
	}

	// 计算截止日期
	cutoff := time.Now().AddDate(0, 0, -keepDays)

	// 清理旧版本
	var newVersions []Version
	for _, v := range versions {
		if v.CreatedAt.After(cutoff) {
			newVersions = append(newVersions, v)
		} else {
			// 删除版本文件
			versionPath := filepath.Join(config.AppConfig.DataDir, "backups", v.ID+'.json')
			os.Remove(versionPath)
		}
	}

	// 更新内存中的版本列表
	vm.versions[fileID] = newVersions

	return nil
}

// saveVersion 保存版本到磁盘
func (vm *VersionManager) saveVersion(version Version) error {
	versionPath := filepath.Join(config.AppConfig.DataDir, "backups", version.ID+'.json')

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(versionPath), 0755); err != nil {
		return err
	}

	// 序列化
	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(versionPath, data, 0644)
}

// loadVersions 从磁盘加载版本列表
func (vm *VersionManager) loadVersions(fileID string) ([]Version, error) {
	backupsDir := filepath.Join(config.AppConfig.DataDir, "backups")

	// 确保目录存在
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		return nil, err
	}

	// 读取目录
	files, err := os.ReadDir(backupsDir)
	if err != nil {
		return nil, err
	}

	// 过滤文件的版本
	var versions []Version
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			// 读取版本文件
			versionPath := filepath.Join(backupsDir, file.Name())
			data, err := os.ReadFile(versionPath)
			if err != nil {
				continue
			}

			// 反序列化
			var version Version
			if err := json.Unmarshal(data, &version); err != nil {
				continue
			}

			// 检查是否是目标文件的版本
			if version.FileID == fileID {
				versions = append(versions, version)
			}
		}
	}

	return versions, nil
}