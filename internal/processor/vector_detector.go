package processor

import (
	"fmt"
	"math"
	"strings"
	"time"
	"voidtext/internal/config"
	"voidtext/internal/external"
	"voidtext/internal/processor/preprocess"
)

// VectorDetector 向量检测器
type VectorDetector struct {
	SimilarityThreshold float64
	VectorModelType     string
	VectorModelName     string
	apiClient           *external.API
	ollamaClient        *external.OllamaClient
}

// VectorDetectionResult 向量检测结果
type VectorDetectionResult struct {
	Content  string              `json:"content"`
	Original string              `json:"original"`
	Changes  []preprocess.Change `json:"changes"`
	Stats    map[string]int      `json:"stats"`
}

// NewVectorDetector 创建向量检测器
func NewVectorDetector(similarityThreshold float64, modelType, modelName string) *VectorDetector {
	vd := &VectorDetector{
		SimilarityThreshold: similarityThreshold,
		VectorModelType:     modelType,
		VectorModelName:     modelName,
	}
	if modelType == "api" {
		vd.apiClient = external.NewEmbeddingAPI()
	} else if modelType == "ollama" {
		timeout := time.Duration(config.AppConfigInstance.LocalModelTimeout) * time.Second
		vd.ollamaClient = external.NewOllamaClient(
			config.AppConfigInstance.LocalModelURL,
			modelName,
			timeout,
		)
	}
	return vd
}

// DetectDuplicates 检测重复内容
func (vd *VectorDetector) DetectDuplicates(content string) (VectorDetectionResult, error) {
	result := VectorDetectionResult{
		Content:  content,
		Original: content,
		Changes:  []preprocess.Change{},
		Stats:    make(map[string]int),
	}

	// 如果向量检测被禁用，直接返回
	if !config.AppConfigInstance.EnableVectorDetection {
		return result, nil
	}

	// 按段落分割文本
	paragraphs := vd.splitIntoParagraphs(content)

	if len(paragraphs) <= 1 {
		return result, nil
	}

	// 生成向量表示
	vectors, err := vd.generateVectors(paragraphs)
	if err != nil {
		return result, err
	}

	// 检测重复段落
	duplicateIndices := vd.findDuplicateIndices(vectors, paragraphs)

	// 移除重复段落
	if len(duplicateIndices) > 0 {
		result = vd.removeDuplicateParagraphs(result, paragraphs, duplicateIndices)
	}

	return result, nil
}

// splitIntoParagraphs 将文本分割为段落
func (vd *VectorDetector) splitIntoParagraphs(content string) []string {
	// 按换行符分割
	paragraphs := strings.Split(content, "\n")

	// 过滤空段落
	filtered := []string{}
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if len(p) > 0 {
			filtered = append(filtered, p)
		}
	}

	return filtered
}

// generateVectors 生成段落向量
// api 模式：调用外部 Embedding 服务；ollama 模式：调用本地 Ollama embedding；local 模式：7 维混合特征
func (vd *VectorDetector) generateVectors(paragraphs []string) ([][]float64, error) {
	if vd.VectorModelType == "api" && vd.apiClient != nil {
		resp, err := vd.apiClient.GenerateEmbedding(paragraphs)
		if err != nil {
			return nil, fmt.Errorf("embedding API 调用失败: %w", err)
		}
		if resp == nil || len(resp.Data) != len(paragraphs) {
			return nil, fmt.Errorf("embedding API 返回数量不匹配: 期望 %d, 实际 %d", len(paragraphs), len(resp.Data))
		}
		vectors := make([][]float64, len(paragraphs))
		for _, d := range resp.Data {
			vectors[d.Index] = d.Embedding
		}
		return vectors, nil
	}

	if vd.VectorModelType == "ollama" && vd.ollamaClient != nil {
		resp, err := vd.ollamaClient.GenerateEmbedding(paragraphs)
		if err != nil {
			return nil, fmt.Errorf("ollama embedding 调用失败: %w", err)
		}
		if resp == nil || len(resp.Embeddings) != len(paragraphs) {
			return nil, fmt.Errorf("ollama embedding 返回数量不匹配: 期望 %d, 实际 %d", len(paragraphs), len(resp.Embeddings))
		}
		vectors := make([][]float64, len(paragraphs))
		copy(vectors, resp.Embeddings)
		return vectors, nil
	}

	vectors := make([][]float64, len(paragraphs))
	for i, paragraph := range paragraphs {
		runes := []rune(paragraph)
		runeLen := float64(len(runes))

		vector := make([]float64, 7)

		// 语义特征（均归一化到 [0,1]）
		vector[0] = math.Min(runeLen/500.0, 1.0)
		if runeLen > 0 {
			vector[1] = float64(strings.Count(paragraph, "。")) / runeLen
			vector[2] = float64(strings.Count(paragraph, "，")) / runeLen
		}

		// 判别特征：FNV-1a 哈希分片为 4 个 uint16 → [0,1]
		normalized := vd.normalizeParagraph(paragraph)
		h := fnv1a64(normalized)
		vector[3] = float64(h&0xFFFF) / 65536.0
		vector[4] = float64((h>>16)&0xFFFF) / 65536.0
		vector[5] = float64((h>>32)&0xFFFF) / 65536.0
		vector[6] = float64((h>>48)&0xFFFF) / 65536.0

		vectors[i] = vector
	}

	return vectors, nil
}

// fnv1a64 计算字符串的 FNV-1a 64 位哈希值
func fnv1a64(s string) uint64 {
	var h uint64 = 14695981039346656037 // FNV offset basis
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211 // FNV prime
	}
	return h
}

// findDuplicateIndices 查找重复段落的索引
// 两阶段检测：(1) 精确匹配（快速路径），(2) 向量余弦相似度（近义重复）
func (vd *VectorDetector) findDuplicateIndices(vectors [][]float64, paragraphs []string) []int {
	duplicateIndices := []int{}
	duplicateSet := make(map[int]bool)
	seen := make(map[string]bool) // normalized -> seen

	for i, paragraph := range paragraphs {
		normalized := vd.normalizeParagraph(paragraph)

		// 阶段 1：精确匹配（去标点后完全相同）
		if _, exists := seen[normalized]; exists {
			// 仅标记当前为重复，保留 firstIdx 以允许后续余弦比较
			duplicateIndices = append(duplicateIndices, i)
			duplicateSet[i] = true
			continue
		}
		seen[normalized] = true

		// 阶段 2：向量余弦相似度（检测近义重复）
		for j := 0; j < i; j++ {
			if duplicateSet[j] {
				continue // j 已经是重复段落，跳过
			}
			similarity := vd.calculateCosineSimilarity(vectors[i], vectors[j])
			if similarity >= vd.SimilarityThreshold {
				duplicateIndices = append(duplicateIndices, i)
				duplicateSet[i] = true
				break
			}
		}
	}

	return duplicateIndices
}

// normalizeParagraph 规范化段落文本
func (vd *VectorDetector) normalizeParagraph(paragraph string) string {
	// 移除标点符号和空白字符进行简化匹配
	replacer := strings.NewReplacer(
		"，", "", "。", "", "！", "", "？", "",
		"；", "", "：", "", "（", "", "）", "",
		"【", "", "】", "", "《", "", "》", "",
		" ", "", "\t", "", "\n", "", "\r", "",
	)

	return replacer.Replace(paragraph)
}

// removeDuplicateParagraphs 移除重复段落
func (vd *VectorDetector) removeDuplicateParagraphs(result VectorDetectionResult, paragraphs []string, duplicateIndices []int) VectorDetectionResult {
	filteredParagraphs := []string{}
	dupSet := make(map[int]bool)
	for _, idx := range duplicateIndices {
		dupSet[idx] = true
	}

	currentPos := 0
	for i, paragraph := range paragraphs {
		isDuplicate := dupSet[i]

		if !isDuplicate {
			filteredParagraphs = append(filteredParagraphs, paragraph)
		} else {
			result.Changes = append(result.Changes, preprocess.Change{
				Type:        "duplicate_paragraph",
				Original:    paragraph,
				Replacement: "",
				Position:    currentPos,
			})
		}

		currentPos += len(paragraph) + 1
	}

	result.Content = strings.Join(filteredParagraphs, "\n")
	result.Stats["duplicate_paragraphs_removed"] = len(duplicateIndices)

	return result
}

// calculateCosineSimilarity 计算余弦相似度（简化实现）
func (vd *VectorDetector) calculateCosineSimilarity(vec1, vec2 []float64) float64 {
	if len(vec1) != len(vec2) {
		return 0.0
	}

	dotProduct := 0.0
	magnitude1 := 0.0
	magnitude2 := 0.0

	for i := range len(vec1) {
		dotProduct += vec1[i] * vec2[i]
		magnitude1 += vec1[i] * vec1[i]
		magnitude2 += vec2[i] * vec2[i]
	}

	if magnitude1 == 0 || magnitude2 == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(magnitude1) * math.Sqrt(magnitude2))
}
