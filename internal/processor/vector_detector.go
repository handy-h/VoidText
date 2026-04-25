package processor

import (
	"log"
	"math"
	"strings"
	"voidtext/internal/config"
	"voidtext/internal/external"
	"voidtext/internal/processor/preprocess"
)

// VectorDetector 向量检测器
type VectorDetector struct {
	SimilarityThreshold float64
	VectorModelType     string
	VectorModelName     string
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
	return &VectorDetector{
		SimilarityThreshold: similarityThreshold,
		VectorModelType:     modelType,
		VectorModelName:     modelName,
	}
}

// DetectDuplicates 检测重复内容
func (vd *VectorDetector) DetectDuplicates(content string) VectorDetectionResult {
	result := VectorDetectionResult{
		Content:  content,
		Original: content,
		Changes:  []preprocess.Change{},
		Stats:    make(map[string]int),
	}

	// 如果向量检测被禁用，直接返回
	if !config.AppConfigInstance.EnableVectorDetection {
		return result
	}

	// 按段落分割文本
	paragraphs := vd.splitIntoParagraphs(content)

	if len(paragraphs) <= 1 {
		return result
	}

	// 生成向量表示
	vectors := vd.generateVectors(paragraphs)

	// 检测重复段落
	duplicateIndices := vd.findDuplicateIndices(vectors, paragraphs)

	// 移除重复段落
	if len(duplicateIndices) > 0 {
		result = vd.removeDuplicateParagraphs(result, paragraphs, duplicateIndices)
	}

	return result
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

// generateVectors 生成段落向量（简化实现）
func (vd *VectorDetector) generateVectors(paragraphs []string) [][]float64 {
	vectors := make([][]float64, len(paragraphs))

	// 简化实现：使用段落长度作为向量特征
	// 实际项目中应该使用真正的向量模型
	for i, paragraph := range paragraphs {
		// 使用段落长度和字符分布作为简化特征
		vector := make([]float64, 3)
		vector[0] = float64(len(paragraph))
		vector[1] = float64(strings.Count(paragraph, "。"))
		vector[2] = float64(strings.Count(paragraph, "，"))

		// 归一化
		if vector[0] > 0 {
			vector[1] = vector[1] / vector[0]
			vector[2] = vector[2] / vector[0]
		}

		vectors[i] = vector
	}

	return vectors
}

// findDuplicateIndices 查找重复段落的索引
func (vd *VectorDetector) findDuplicateIndices(_ [][]float64, paragraphs []string) []int {
	duplicateIndices := []int{}
	seen := make(map[string]bool)

	for i, paragraph := range paragraphs {
		// 使用精确匹配作为简化实现
		normalized := vd.normalizeParagraph(paragraph)

		if seen[normalized] {
			duplicateIndices = append(duplicateIndices, i)
		} else {
			seen[normalized] = true
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

// generateEmbeddings 使用外部API生成嵌入向量
func (vd *VectorDetector) generateEmbeddings(texts []string) ([][]float64, error) {
	if vd.VectorModelType == "api" {
		api := external.NewAPI()
		resp, err := api.GenerateEmbedding(texts)
		if err != nil {
			log.Printf("[向量检测] API调用失败，降级为本地向量: 错误=%v", err)
			return vd.generateVectors(texts), nil
		}

		if resp != nil && len(resp.Data) > 0 {
			embeddings := make([][]float64, len(resp.Data))
			for i, data := range resp.Data {
				embeddings[i] = data.Embedding
			}
			return embeddings, nil
		}

		log.Printf("[向量检测] API返回空数据，降级为本地向量")
	}

	// 如果API调用失败或使用本地模型，返回简化向量
	return vd.generateVectors(texts), nil
}
