package file

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// ComputeFileMd5 计算文件的MD5值
func ComputeFileMd5(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", fmt.Errorf("计算MD5失败: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ComputeContentMd5 计算文本内容的MD5值
func ComputeContentMd5(content string) string {
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:])
}
