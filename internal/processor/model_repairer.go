package processor

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"
	"voidtext/internal/config"
	"voidtext/internal/external"
	"voidtext/internal/processor/preprocess"
)

// ModelRepairer 模型修复器
type ModelRepairer struct {
	RepairModelType string
	RepairModelName string
	api             *external.API
	ollamaClient    *external.OllamaClient
}

// ModelRepairResult 模型修复结果
type ModelRepairResult struct {
	Content  string              `json:"content"`
	Original string              `json:"original"`
	Changes  []preprocess.Change `json:"changes"`
	Stats    map[string]int      `json:"stats"`
}

// NewModelRepairer 创建模型修复器
func NewModelRepairer(modelType, modelName string) *ModelRepairer {
	mr := &ModelRepairer{
		RepairModelType: modelType,
		RepairModelName: modelName,
		api:             external.NewAPI(),
	}
	if config.AppConfigInstance.EnableLocalModel {
		timeout := time.Duration(config.AppConfigInstance.LocalModelTimeout) * time.Second
		mr.ollamaClient = external.NewOllamaClient(
			config.AppConfigInstance.LocalModelURL,
			config.AppConfigInstance.LocalModelName,
			timeout,
		)
	}
	return mr
}

// RepairText 修复文本中的错别字和语法错误
func (mr *ModelRepairer) RepairText(content string) ModelRepairResult {
	result := ModelRepairResult{
		Content:  content,
		Original: content,
		Changes:  []preprocess.Change{},
		Stats:    make(map[string]int),
	}

	// 如果模型修复被禁用，直接返回
	if !config.AppConfigInstance.EnableModelRepair {
		return result
	}

	// 按段落分割文本
	paragraphs := mr.SplitIntoParagraphs(content)

	if len(paragraphs) == 0 {
		return result
	}

	// 修复每个段落
	repairedParagraphs := []string{}

	for _, paragraph := range paragraphs {
		repaired, changes := mr.RepairParagraph(paragraph)
		repairedParagraphs = append(repairedParagraphs, repaired)
		result.Changes = append(result.Changes, changes...)
	}

	// 重新组合文本
	result.Content = strings.Join(repairedParagraphs, "\n")
	result.Stats["paragraphs_repaired"] = len(paragraphs)
	result.Stats["total_changes"] = len(result.Changes)

	return result
}

// SplitIntoParagraphs 将文本分割为段落
func (mr *ModelRepairer) SplitIntoParagraphs(content string) []string {
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

// RepairParagraph 修复单个段落
func (mr *ModelRepairer) RepairParagraph(paragraph string) (string, []preprocess.Change) {
	if len(paragraph) < 10 {
		return paragraph, []preprocess.Change{}
	}

	// 本地 Ollama 优先（若已配置且当前健康）
	if mr.ollamaClient != nil && GetHealthManager().ShouldUseLocalModel() {
		return mr.repairWithOllama(paragraph)
	}

	// 远程 API
	if mr.RepairModelType == "api" {
		return mr.repairWithAPI(paragraph)
	}

	return mr.repairLocally(paragraph)
}

// repairWithOllama 使用本地 Ollama 模型修复文本，失败时降级到远程 API 或本地字典
func (mr *ModelRepairer) repairWithOllama(paragraph string) (string, []preprocess.Change) {
	systemPrompt := "你是一个专业的中文小说校对编辑。请修正以下段落中的错别字和语法错误，保持原文风格不变。只输出修正后的文本，无需解释。"
	userPrompt := "输入：她高兴及了，跑过去抱住他。\n输出：她高兴极了，跑过去抱住他。\n\n当前任务：\n输入：" + paragraph + "\n输出："

	repairedText, err := mr.ollamaClient.Generate(userPrompt, systemPrompt)
	if err != nil {
		log.Printf("[Ollama修复] 调用失败，降级处理: 段落长度=%d, 错误=%v", len(paragraph), err)
		if mr.RepairModelType == "api" {
			return mr.repairWithAPI(paragraph)
		}
		return mr.repairLocally(paragraph)
	}

	repairedText = strings.TrimSpace(repairedText)
	if repairedText == "" || repairedText == paragraph {
		return paragraph, []preprocess.Change{}
	}
	if !mr.validateRepair(paragraph, repairedText) {
		return paragraph, []preprocess.Change{}
	}

	changes := mr.compareTexts(paragraph, repairedText)
	log.Printf("[Ollama修复] 成功: 输入长度=%d, 输出长度=%d, 修改数=%d", len(paragraph), len(repairedText), len(changes))
	return repairedText, changes
}

// repairWithAPI 使用外部API修复文本，带重试机制
func (mr *ModelRepairer) repairWithAPI(paragraph string) (string, []preprocess.Change) {
	systemPrompt := "你是一个专业的中文小说校对编辑。请修正以下段落中的错别字和语法错误，保持原文风格不变。只输出修正后的文本，无需解释。"
	userPrompt := "输入：她高兴及了，跑过去抱住他。\n输出：她高兴极了，跑过去抱住他。\n\n当前任务：\n输入：" + paragraph + "\n输出："

	maxRetries := 3
	baseDelay := 2 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := mr.api.GenerateChatCompletion(systemPrompt, userPrompt, 0, -1)
		if err == nil && resp != nil && len(resp.Choices) > 0 {
			// 成功响应
			repairedText := strings.TrimSpace(resp.Choices[0].Message.Content)
			if repairedText == "" || repairedText == paragraph {
				return paragraph, []preprocess.Change{}
			}
			if !mr.validateRepair(paragraph, repairedText) {
				return paragraph, []preprocess.Change{}
			}
			changes := mr.compareTexts(paragraph, repairedText)
			log.Printf("[LLM修复] 成功: 输入长度=%d, 输出长度=%d, 修改数=%d (第%d次尝试)", len(paragraph), len(repairedText), len(changes), attempt+1)
			return repairedText, changes
		}

		// 判断错误是否可重试
		if !isRetryableError(err) || attempt == maxRetries-1 {
			log.Printf("[LLM修复] API调用失败，回退到本地修复: 段落长度=%d, 错误=%v, 尝试=%d", len(paragraph), err, attempt+1)
			return mr.repairLocally(paragraph)
		}

		// 指数退避
		delay := time.Duration(math.Pow(2, float64(attempt))) * baseDelay
		log.Printf("[LLM修复] API可重试错误，第%d次重试，等待%v: %v", attempt+1, delay, err)
		time.Sleep(delay)
	}

	// 所有重试均失败，回退本地修复
	log.Printf("[LLM修复] %d次重试全部失败，回退到本地修复: 段落长度=%d", maxRetries, len(paragraph))
	return mr.repairLocally(paragraph)
}

// isRetryableError 判断API错误是否可重试
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// 可重试的HTTP状态码
	retryableCodes := []string{"429", "500", "502", "503", "504", "timeout", "connection reset", "EOF", "broken pipe"}
	for _, code := range retryableCodes {
		if strings.Contains(errStr, code) {
			return true
		}
	}
	return false
}

// repairLocally 本地修复（简化实现）
func (mr *ModelRepairer) repairLocally(paragraph string) (string, []preprocess.Change) {
	changes := []preprocess.Change{}

	// 常见错别字映射表
	typoMap := map[string]string{
		"图书管": "图书馆",
		"及了":  "极了",
		"在次":  "再次",
		"哪么":  "那么",
		"因该":  "应该",
		"以经":  "已经",
		"好象":  "好像",
		"做车":  "坐车",
		"的士":  "的士", // 保留
		"他":   "他",  // 保留
		"她":   "她",  // 保留
	}

	// 应用错别字修正
	repaired := paragraph
	for typo, correct := range typoMap {
		if strings.Contains(repaired, typo) && typo != correct {
			// 记录变更
			position := strings.Index(repaired, typo)
			changes = append(changes, preprocess.Change{
				Type:        "typo_correction",
				Original:    typo,
				Replacement: correct,
				Position:    position,
			})

			// 应用修正
			repaired = strings.Replace(repaired, typo, correct, 1)
		}
	}

	return repaired, changes
}

// compareTexts 比较两个文本，生成变更记录
func (mr *ModelRepairer) compareTexts(original, repaired string) []preprocess.Change {
	changes := []preprocess.Change{}

	if original == repaired {
		return changes
	}

	origRunes := []rune(original)
	repRunes := []rune(repaired)

	if len(origRunes) == len(repRunes) {
		byteOffset := 0
		for i := 0; i < len(origRunes); i++ {
			if origRunes[i] != repRunes[i] {
				origStr := string(origRunes[i])
				repStr := string(repRunes[i])

				j := i + 1
				for j < len(origRunes) && origRunes[j] != repRunes[j] {
					origStr += string(origRunes[j])
					repStr += string(repRunes[j])
					j++
				}

				changes = append(changes, preprocess.Change{
					Type:        "character_correction",
					Original:    origStr,
					Replacement: repStr,
					Position:    byteOffset,
				})
				i = j - 1
			}
			byteOffset += len(string(origRunes[i]))
		}
	} else {
		minLen := len(origRunes)
		if len(repRunes) < minLen {
			minLen = len(repRunes)
		}

		byteOffset := 0
		i := 0
		for i < minLen {
			if origRunes[i] != repRunes[i] {
				origChunk := string(origRunes[i])
				repChunk := string(repRunes[i])

				j := i + 1
				oob := j < len(origRunes)
				rob := j < len(repRunes)
				for oob && rob && origRunes[j] != repRunes[j] {
					origChunk += string(origRunes[j])
					repChunk += string(repRunes[j])
					j++
					oob = j < len(origRunes)
					rob = j < len(repRunes)
				}

				changes = append(changes, preprocess.Change{
					Type:        "character_correction",
					Original:    origChunk,
					Replacement: repChunk,
					Position:    byteOffset,
				})
				byteOffset += len(string(origRunes[i]))
				i = j
			} else {
				byteOffset += len(string(origRunes[i]))
				i++
			}
		}

		if len(origRunes) > minLen {
			remaining := ""
			for k := minLen; k < len(origRunes); k++ {
				remaining += string(origRunes[k])
			}
			if remaining != "" {
				changes = append(changes, preprocess.Change{
					Type:        "text_deletion",
					Original:    remaining,
					Replacement: "",
					Position:    byteOffset,
				})
			}
		} else if len(repRunes) > minLen {
			remaining := ""
			for k := minLen; k < len(repRunes); k++ {
				remaining += string(repRunes[k])
			}
			if remaining != "" {
				changes = append(changes, preprocess.Change{
					Type:        "text_insertion",
					Original:    "",
					Replacement: remaining,
					Position:    byteOffset,
				})
			}
		}
	}

	return changes
}

// paragraphReconstructPrompt 段落重组系统提示词（通用版，不包含任何特定书目例子）
const paragraphReconstructPrompt = `你是一个专业的中文小说排版助手。请智能重组以下文本的段落格式。

核心任务：
识别文本中错误的换行并修正，同时为语义段落插入正确的分隔。

处理原则（按优先级）：
1. 合并被硬切断的行：如果相邻行属于同一自然段，将它们连接为一行
2. 分离段落：在话题转换、时间推移、场景切换处插入空行作为段落分隔
3. 保持原样：已经是正确格式的段落不做任何修改

判断"应合并"的信号（任意一条满足即可）：
- 上一行以不完整短语结尾，下一行开头是其自然延续
- 上一行以逗号、顿号、分号结尾，且两行属于同一句子
- 合并后能形成通顺完整的中文句子

判断"应分段"的信号（任意一条满足即可）：
- 出现明确的时间状语（如"第二天"、"几个月后"、"那年"）
- 话题从描述转换为对话，或从对话转换为描述
- 出现章节标记（如"第X章"、"◇"、分隔线"——"）
- 叙事视角或场景发生明显变化

对话处理：
- 同一人物的连续话语保持连贯
- 对话与叙述之间适当留空行
- 引号内的内容不要拆分或重组

绝对约束：
- 不改动任何文字内容（包括标点符号）
- 只调整换行符（\n）的位置和数量
- 不添加或删除任何字符
- 直接输出重组后的完整文本，不要任何说明或标记`

// ReconstructParagraphs 使用LLM智能重组段落
// 对全文进行分块处理后，逐块调用LLM识别段落边界，输出格式良好、段落结构清晰的文本
func (mr *ModelRepairer) ReconstructParagraphs(content string) (string, error) {
	chunkSize := config.AppConfigInstance.ParagraphChunkSize
	if chunkSize <= 0 {
		chunkSize = 8000
	}
	overlap := 200

	// 智能分块：优先按双换行边界，降级到句子边界，再降级到硬分块
	chunks := mr.smartChunk(content, chunkSize, overlap)
	log.Printf("[段落重组] 分块完成: 总块数=%d, 分块大小=%d, 重叠=%d", len(chunks), chunkSize, overlap)

	reconstructedChunks := make([]string, 0, len(chunks))

	for i, chunk := range chunks {
		log.Printf("[段落重组] 正在处理第 %d/%d 块 (长度=%d字符)", i+1, len(chunks), len([]rune(chunk)))

		resp, err := mr.api.GenerateChatCompletion(paragraphReconstructPrompt, chunk, 0, -1)
		if err != nil {
			return content, fmt.Errorf("第%d块LLM调用失败: %w", i+1, err)
		}
		if resp == nil || len(resp.Choices) == 0 {
			return content, fmt.Errorf("第%d块LLM返回空结果", i+1)
		}

		repaired := strings.TrimSpace(resp.Choices[0].Message.Content)
		if repaired == "" {
			// 降级：使用原块
			repaired = chunk
		}
		reconstructedChunks = append(reconstructedChunks, repaired)
		log.Printf("[段落重组] 第 %d/%d 块完成 (输出长度=%d字符)", i+1, len(chunks), len([]rune(repaired)))
	}

	// 合并各块（去除重叠区域）
	result := mr.mergeChunks(reconstructedChunks, overlap)
	log.Printf("[段落重组] 全部完成: 原始长度=%d字符, 重组后长度=%d字符",
		len([]rune(content)), len([]rune(result)))

	return result, nil
}

// ReconstructParagraphsWithCheckpoint 带断点保存的段落重组
// fileMd5 用于断点恢复，checkpointFn 为进度回调（参数：已完成块数, 总块数）
func (mr *ModelRepairer) ReconstructParagraphsWithCheckpoint(content, fileMd5 string, checkpointFn func(done, total int)) (string, error) {
	chunkSize := config.AppConfigInstance.ParagraphChunkSize
	if chunkSize <= 0 {
		chunkSize = 8000
	}
	overlap := 200

	chunks := mr.smartChunk(content, chunkSize, overlap)
	totalChunks := len(chunks)
	log.Printf("[段落重组] 分块完成: 总块数=%d, 分块大小=%d, 重叠=%d", totalChunks, chunkSize, overlap)

	reconstructedChunks := make([]string, 0, totalChunks)

	for i, chunk := range chunks {
		log.Printf("[段落重组] 正在处理第 %d/%d 块 (长度=%d字符)", i+1, totalChunks, len([]rune(chunk)))

		// 段落重组使用远程 API（避免本地模型加载导致 OOM）
		resp, err := mr.api.GenerateChatCompletion(paragraphReconstructPrompt, chunk, 0, -1)
		if err != nil {
			// 保存当前进度
			if len(reconstructedChunks) > 0 && fileMd5 != "" {
				partial := mr.mergeChunks(reconstructedChunks, overlap)
				log.Printf("[段落重组] 第%d块失败，已保存 %d/%d 块的部分结果（%d字符）", i+1, i, totalChunks, len([]rune(partial)))
			}
			return content, fmt.Errorf("第%d块LLM调用失败: %w", i+1, err)
		}
		if resp == nil || len(resp.Choices) == 0 {
			return content, fmt.Errorf("第%d块LLM返回空结果", i+1)
		}

		repaired := strings.TrimSpace(resp.Choices[0].Message.Content)
		if repaired == "" {
			repaired = chunk // 降级：使用原块
		}
		reconstructedChunks = append(reconstructedChunks, repaired)
		log.Printf("[段落重组] 第 %d/%d 块完成 (输出长度=%d字符)", i+1, totalChunks, len([]rune(repaired)))

		// 每处理完一块就保存断点
		if fileMd5 != "" && checkpointFn != nil {
			checkpointFn(i+1, totalChunks)
		}
	}

	result := mr.mergeChunks(reconstructedChunks, overlap)
	log.Printf("[段落重组] 全部完成: 原始长度=%d字符, 重组后长度=%d字符",
		len([]rune(content)), len([]rune(result)))

	return result, nil
}

// smartChunk 智能分块
// 优先按双换行边界分块，找不到则按句子结尾标点分块，再找不到则硬分块
func (mr *ModelRepairer) smartChunk(content string, chunkSize, overlap int) []string {
	runes := []rune(content)
	totalLen := len(runes)

	// 文本长度小于分块大小，无需分块
	if totalLen <= chunkSize {
		return []string{content}
	}

	chunks := make([]string, 0)
	start := 0

	for start < totalLen {
		end := start + chunkSize
		if end > totalLen {
			end = totalLen
		}

		// 如果不是最后一块，尝试找最佳切割点
		if end < totalLen {
			end = mr.findBestSplitPoint(runes, start, end, chunkSize)
		}

		chunk := string(runes[start:end])
		chunks = append(chunks, chunk)

		// 下一块起点 = 当前结束 - 重叠
		start = end - overlap
		if start <= 0 || start >= totalLen {
			break
		}
		// 防止无限循环
		if start >= end {
			start = end
		}
	}

	return chunks
}

// findBestSplitPoint 在指定范围内找到最佳切割点
// 优先级：双换行 > 句子结尾标点 > 原始切点
func (mr *ModelRepairer) findBestSplitPoint(runes []rune, start, maxEnd, chunkSize int) int {
	// 搜索范围：从 maxEnd-400 到 maxEnd+200
	searchStart := maxEnd - 400
	if searchStart < start {
		searchStart = start
	}
	searchEnd := maxEnd + 200
	if searchEnd > len(runes) {
		searchEnd = len(runes)
	}

	// 优先找双换行
	for i := maxEnd - 1; i >= searchStart && i < searchEnd; i-- {
		if i+1 < len(runes) && runes[i] == '\n' && runes[i+1] == '\n' {
			return i + 2
		}
		if i > 0 && runes[i] == '\n' && runes[i-1] == '\n' {
			return i + 1
		}
	}

	// 次优先找句子结尾标点（。！？）
	sentenceEnds := map[rune]bool{'。': true, '！': true, '？': true, '"': true, '」': true}
	for i := maxEnd - 1; i >= searchStart; i-- {
		if sentenceEnds[runes[i]] {
			return i + 1
		}
	}

	// 降级到换行符
	for i := maxEnd - 1; i >= searchStart; i-- {
		if runes[i] == '\n' {
			return i + 1
		}
	}

	return maxEnd
}

// mergeChunks 合并分块处理后的结果，处理重叠区域去重
func (mr *ModelRepairer) mergeChunks(chunks []string, overlap int) string {
	if len(chunks) == 0 {
		return ""
	}
	if len(chunks) == 1 {
		return chunks[0]
	}

	var result strings.Builder
	result.WriteString(chunks[0])

	for i := 1; i < len(chunks); i++ {
		prev := []rune(chunks[i-1])
		curr := []rune(chunks[i])

		if len(prev) < overlap || len(curr) < overlap {
			// 块太短，直接拼接
			result.WriteString("\n")
			result.WriteString(chunks[i])
			continue
		}

		// 找到前一块末尾与后一块开头的最长公共序列，确定去重边界
		cutIndex := mr.findMergePoint(curr, prev)
		if cutIndex < len(curr) {
			result.WriteString(string(curr[cutIndex:]))
		}
	}

	return result.String()
}

// findMergePoint 找到后一块中去重后的起始位置
// 从前一块末尾取 overlap 字符，与后一块开头匹配找最长公共前缀
func (mr *ModelRepairer) findMergePoint(curr, prev []rune) int {
	if len(prev) == 0 {
		return 0
	}

	// 取前一块末尾的搜索窗口（最多3倍overlap）
	searchStart := len(prev) - 600
	if searchStart < 0 {
		searchStart = 0
	}
	tail := prev[searchStart:]

	// 在后一块中查找与 tail 开头匹配的位置
	bestOffset := 0
	bestMatch := 0

	// 逐字符滑动匹配
	for offset := 0; offset < len(curr) && offset < 600; offset++ {
		matchLen := 0
		for k := 0; k < len(tail) && offset+k < len(curr); k++ {
			if tail[k] == curr[offset+k] {
				matchLen++
			} else {
				break
			}
		}
		if matchLen > bestMatch && matchLen >= 10 {
			bestMatch = matchLen
			bestOffset = offset
		}
	}

	if bestMatch >= 20 {
		// 找到可靠的重叠点，跳过重叠部分
		return bestOffset + bestMatch
	}

	// 未找到可靠重叠，保守处理：直接拼接
	return 0
}

// validateRepair 校验LLM修复结果的质量
// 如果修复结果与原始文本差异过大，返回 false 表示拒绝该修复
// 这可以防止LLM产生不合理的输出（如"他是。"→"他是是"）被误接受
func (mr *ModelRepairer) validateRepair(original, repaired string) bool {
	origLen := len([]rune(original))
	repLen := len([]rune(repaired))

	if origLen == 0 || repLen == 0 {
		return false
	}

	// 短文本（< 15 字符）校验更严格：只要长度不同就拒绝
	if origLen < 15 || repLen < 15 {
		if origLen != repLen {
			log.Printf("[修复校验] 拒绝修复（短文本长度变化）: 原始=%d字符 修复=%d字符", origLen, repLen)
			return false
		}
	}

	// 长度差比例超过 20% 且长度差 > 2，拒绝
	lenDiff := float64(abs(origLen - repLen))
	maxLen := float64(max(origLen, repLen))
	if lenDiff > 2 && lenDiff/maxLen > 0.20 {
		log.Printf("[修复校验] 拒绝修复（长度变化过大）: 原始=%d字符 修复=%d字符", origLen, repLen)
		return false
	}

	// 编辑距离校验：仅对短段落（≤ 500 字符）计算编辑距离
	// 长段落的内存和时间开销过大，只用长度差判断
	if origLen <= 500 && repLen <= 500 {
		origRunes := []rune(original)
		repRunes := []rune(repaired)
		editDist := levenshteinDistanceRunes(origRunes, repRunes)
		if float64(editDist)/maxLen > 0.25 {
			log.Printf("[修复校验] 拒绝修复（编辑距离过大）: 原文=%d字符 编辑距离=%d 比例=%.2f",
				origLen, editDist, float64(editDist)/maxLen)
			return false
		}
	}

	return true
}

// levenshteinDistanceRunes 计算两个 rune 切片的莱文斯坦编辑距离，使用O(min(m,n))空间的优化实现
func levenshteinDistanceRunes(a, b []rune) int {
	m, n := len(a), len(b)

	// 确保 a 是较短的，以节省空间
	if m > n {
		a, b = b, a
		m, n = n, m
	}

	// 当前行和前一行
	prev := make([]int, m+1)
	curr := make([]int, m+1)

	for i := 0; i <= m; i++ {
		prev[i] = i
	}

	for j := 1; j <= n; j++ {
		curr[0] = j
		for i := 1; i <= m; i++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[i] = min3(
				prev[i]+1,      // 删除
				curr[i-1]+1,    // 插入
				prev[i-1]+cost, // 替换
			)
		}
		prev, curr = curr, prev
	}

	return prev[m]
}

// abs 返回整数的绝对值
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// min3 返回三个整数的最小值
func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
