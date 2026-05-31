package processor

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"
	"voidtext/internal/config"
	"voidtext/internal/external"
	"voidtext/internal/processor/preprocess"
)

// 精确匹配最小字符数阈值，避免结构元素（如 "（1）"、"★ ★ ★"）被误判为重复
const minExactMatchRunes = 5

// 本地模型向量维度（8 语义特征 + 24 哈希特征 = 32 维）
const localModelDims = 32

// 滑动窗口大小：只比较最近 N 个段落（重复段落通常距离不远）
// 设置为 0 则比较所有段落（原始行为）
const defaultWindowSize = 300

// VectorDetector 向量检测器
type VectorDetector struct {
	SimilarityThreshold float64
	VectorModelType     string
	VectorModelName     string
	WindowSize          int // 滑动窗口大小，0 表示比较所有段落
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
		WindowSize:          defaultWindowSize, // 默认使用滑动窗口优化
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

	startTime := time.Now()

	// 按段落分割文本
	paragraphs := vd.splitIntoParagraphs(content)

	if len(paragraphs) <= 1 {
		return result, nil
	}

	log.Printf("[向量检测] 开始处理 %d 个段落", len(paragraphs))

	// 生成向量表示
	vectors, err := vd.generateVectors(paragraphs)
	if err != nil {
		return result, err
	}

	// 检测重复段落（优化版）
	duplicateIndices := vd.findDuplicateIndices(vectors, paragraphs)

	// 移除重复段落
	if len(duplicateIndices) > 0 {
		result = vd.removeDuplicateParagraphs(result, paragraphs, duplicateIndices)
	}

	elapsed := time.Since(startTime)
	log.Printf("[向量检测] 完成，耗时 %.2f 秒，移除 %d 个重复段落",
		elapsed.Seconds(), len(duplicateIndices))

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
// api 模式：调用外部 Embedding 服务；ollama 模式：调用本地 Ollama embedding；local 模式：32 维混合特征
func (vd *VectorDetector) generateVectors(paragraphs []string) ([][]float64, error) {
	if vd.VectorModelType == "api" && vd.apiClient != nil {
		resp, err := vd.apiClient.GenerateEmbedding(paragraphs)
		if err != nil {
			return nil, fmt.Errorf("embedding API 调用失败: %w", err)
		}
		if resp == nil {
			return nil, fmt.Errorf("embedding API 返回空响应")
		}
		if len(resp.Data) != len(paragraphs) {
			return nil, fmt.Errorf("embedding API 返回数量不匹配: 期望 %d, 实际 %d", len(paragraphs), len(resp.Data))
		}
		vectors := make([][]float64, len(paragraphs))
		for _, d := range resp.Data {
			vectors[d.Index] = d.Embedding
		}
		return vectors, nil
	}

	if vd.VectorModelType == "ollama" && vd.ollamaClient != nil {
		// 分批处理，避免一次性发送过多段落导致超时
		const batchSize = 50
		total := len(paragraphs)
		vectors := make([][]float64, total)

		for start := 0; start < total; start += batchSize {
			end := start + batchSize
			if end > total {
				end = total
			}
			batch := paragraphs[start:end]

			log.Printf("[向量检测] Ollama embedding 进度: %d/%d 段落", start+len(batch), total)
			resp, err := vd.ollamaClient.GenerateEmbedding(batch)
			if err != nil {
				return nil, fmt.Errorf("ollama embedding 调用失败 (batch %d-%d): %w", start, end, err)
			}
			if resp == nil {
				return nil, fmt.Errorf("ollama embedding 返回空响应 (batch %d-%d)", start, end)
			}
			if len(resp.Embeddings) != len(batch) {
				return nil, fmt.Errorf("ollama embedding 返回数量不匹配: 期望 %d, 实际 %d", len(batch), len(resp.Embeddings))
			}
			copy(vectors[start:end], resp.Embeddings)
		}
		return vectors, nil
	}

	vectors := make([][]float64, len(paragraphs))
	for i, paragraph := range paragraphs {
		runes := []rune(paragraph)
		runeLen := float64(len(runes))

		vector := make([]float64, localModelDims)

		// 语义特征（均归一化到 [0,1]）
		vector[0] = math.Min(runeLen/500.0, 1.0)
		if runeLen > 0 {
			vector[1] = float64(strings.Count(paragraph, "。")) / runeLen
			vector[2] = float64(strings.Count(paragraph, "，")) / runeLen
			vector[3] = float64(strings.Count(paragraph, "！")) / runeLen
			vector[4] = float64(strings.Count(paragraph, "？")) / runeLen
			vector[5] = float64(strings.Count(paragraph, "：")) / runeLen
			vector[6] = float64(strings.Count(paragraph, "；")) / runeLen
			// 平均字符值（归一化）
			var runeSum float64
			for _, r := range runes {
				runeSum += float64(r)
			}
			vector[7] = math.Min(runeSum/(runeLen*65536.0), 1.0)
		}

		// 判别特征：多个哈希函数分片，提升区分力
		normalized := vd.normalizeParagraph(paragraph)
		// FNV-1a 哈希 → 8 个 uint8 特征
		h1 := fnv1a64(normalized)
		for j := 0; j < 8; j++ {
			vector[8+j] = float64((h1>>(uint(j)*8))&0xFF) / 256.0
		}
		// 第二哈希（不同质数）→ 8 个 uint8 特征
		h2 := fnv1a64WithSeed(normalized, 0x9e3779b97f4a7c15)
		for j := 0; j < 8; j++ {
			vector[16+j] = float64((h2>>(uint(j)*8))&0xFF) / 256.0
		}
		// 第三哈希（不同质数）→ 8 个 uint8 特征
		h3 := fnv1a64WithSeed(normalized, 0xbf58476d1ce4e5b9)
		for j := 0; j < 8; j++ {
			vector[24+j] = float64((h3>>(uint(j)*8))&0xFF) / 256.0
		}

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

// fnv1a64WithSeed 使用自定义偏移量计算 FNV-1a 哈希（生成独立分布的哈希值）
func fnv1a64WithSeed(s string, seed uint64) uint64 {
	h := seed
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// vectorMagnitude 预计算的向量及其模长
type vectorMagnitude struct {
	vector    []float64
	magnitude float64
}

// findDuplicateIndices 查找重复段落的索引
// 优化策略：
// 1. 滑动窗口：只比较最近 WindowSize 个段落（重复段落通常距离不远）
// 2. 预计算模长：避免重复计算 sqrt
// 3. 并行计算：使用 goroutine 并行处理相似度计算
func (vd *VectorDetector) findDuplicateIndices(vectors [][]float64, paragraphs []string) []int {
	n := len(paragraphs)
	duplicateSet := make(map[int]bool)
	seen := make(map[string]int) // normalized -> first index

	// 预计算所有向量的模长
	vecMags := make([]vectorMagnitude, n)
	for i, vec := range vectors {
		mag := 0.0
		for _, v := range vec {
			mag += v * v
		}
		vecMags[i] = vectorMagnitude{
			vector:    vec,
			magnitude: math.Sqrt(mag),
		}
	}

	// 阶段 1：精确匹配（快速路径）
	for i, paragraph := range paragraphs {
		normalized := vd.normalizeParagraph(paragraph)

		// 跳过归一化后过短的段落
		runeLen := len([]rune(normalized))
		if runeLen < minExactMatchRunes {
			continue
		}

		// 精确匹配
		if firstIdx, exists := seen[normalized]; exists {
			duplicateSet[i] = true
			log.Printf("[向量检测] 精确匹配重复: 段落 %d 与 %d 相同", i, firstIdx)
			continue
		}
		seen[normalized] = i
	}

	// 阶段 2：向量余弦相似度（滑动窗口 + 并行计算）
	type compareTask struct {
		i int
		j int
	}

	type compareResult struct {
		i          int
		j          int
		similarity float64
	}

	// 确定并行度
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // 限制最大并行度
	}

	taskChan := make(chan compareTask, 1000)
	resultChan := make(chan compareResult, 1000)
	var wg sync.WaitGroup

	// 启动 worker goroutines
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range taskChan {
				i, j := task.i, task.j
				similarity := vd.calculateCosineSimilarityOptimized(
					vecMags[i], vecMags[j],
				)
				if similarity >= vd.SimilarityThreshold {
					resultChan <- compareResult{i: i, j: j, similarity: similarity}
				}
			}
		}()
	}

	// 生成比较任务（滑动窗口）
	go func() {
		for i := 0; i < n; i++ {
			if duplicateSet[i] {
				continue // 已经是重复段落，跳过
			}

			// 滑动窗口：只比较最近 WindowSize 个段落
			// WindowSize = 0 表示比较所有段落（原始行为）
			start := 0
			if vd.WindowSize > 0 {
				start = i - vd.WindowSize
				if start < 0 {
					start = 0
				}
			}

			for j := start; j < i; j++ {
				if duplicateSet[j] {
					continue // j 已经是重复段落，跳过
				}
				taskChan <- compareTask{i: i, j: j}
			}
		}
		close(taskChan)
	}()

	// 收集结果
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 处理结果
	for result := range resultChan {
		if !duplicateSet[result.i] {
			duplicateSet[result.i] = true
			log.Printf("[向量检测] 向量相似重复: 段落 %d 与 %d 相似度 %.4f",
				result.i, result.j, result.similarity)
		}
	}

	// 转换为索引列表
	duplicateIndices := []int{}
	for i := 0; i < n; i++ {
		if duplicateSet[i] {
			duplicateIndices = append(duplicateIndices, i)
		}
	}

	return duplicateIndices
}

// normalizeParagraph 规范化段落文本
// 仅移除中文标点符号，保留空格和括号等有意义的格式字符
func (vd *VectorDetector) normalizeParagraph(paragraph string) string {
	// 移除标点符号进行简化匹配，但保留空格（避免 "★ ★ ★" 和 "★    ★" 碰撞）
	replacer := strings.NewReplacer(
		"，", "", "。", "", "！", "", "？", "",
		"；", "", "：", "", "【", "", "】", "",
		"《", "", "》", "",
	)

	return strings.TrimSpace(replacer.Replace(paragraph))
}

// removeDuplicateParagraphs 移除重复段落
func (vd *VectorDetector) removeDuplicateParagraphs(result VectorDetectionResult, paragraphs []string, duplicateIndices []int) VectorDetectionResult {
	filteredParagraphs := []string{}
	dupSet := make(map[int]bool)
	for _, idx := range duplicateIndices {
		dupSet[idx] = true
	}

	charsRemoved := 0
	currentPos := 0
	for i, paragraph := range paragraphs {
		isDuplicate := dupSet[i]

		if !isDuplicate {
			filteredParagraphs = append(filteredParagraphs, paragraph)
		} else {
			charsRemoved += len([]rune(paragraph))
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
	result.Stats["duplicate_chars_removed"] = charsRemoved

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

// calculateCosineSimilarityOptimized 计算余弦相似度（优化版，使用预计算的模长）
func (vd *VectorDetector) calculateCosineSimilarityOptimized(vm1, vm2 vectorMagnitude) float64 {
	if len(vm1.vector) != len(vm2.vector) {
		return 0.0
	}

	if vm1.magnitude == 0 || vm2.magnitude == 0 {
		return 0.0
	}

	dotProduct := 0.0
	for i := range len(vm1.vector) {
		dotProduct += vm1.vector[i] * vm2.vector[i]
	}

	return dotProduct / (vm1.magnitude * vm2.magnitude)
}
