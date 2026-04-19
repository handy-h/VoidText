package file

import (
	"path/filepath"
	"strings"

	"txt-cleaning/internal/config"
)

// ParsedFileName 解析后的文件名信息
type ParsedFileName struct {
	Author string `json:"author"`
	Title  string `json:"title"`
}

// ParseFileName 从文件名中提取作者和标题
func ParseFileName(fileName string) ParsedFileName {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	base = strings.TrimSpace(base)

	separators := config.AppConfigInstance.GetNameSeparators()

	for _, sep := range separators {
		idx := strings.Index(base, sep)
		if idx > 0 && idx < len(base)-len(sep) {
			part1 := strings.TrimSpace(base[:idx])
			part2 := strings.TrimSpace(base[idx+len(sep):])

			if isLikelyAuthor(part1) {
				return ParsedFileName{Author: part1, Title: part2}
			}
			if isLikelyAuthor(part2) {
				return ParsedFileName{Author: part2, Title: part1}
			}

			return ParsedFileName{Author: part1, Title: part2}
		}
	}

	return ParsedFileName{Author: "", Title: base}
}

// isLikelyAuthor 判断字符串是否像作者名
func isLikelyAuthor(s string) bool {
	runeCount := len([]rune(s))
	return runeCount >= 1 && runeCount <= 10
}
