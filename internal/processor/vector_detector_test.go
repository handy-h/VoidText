package processor

import (
	"testing"
)

func Test_normalizeParagraph_should_preserve_spaces(t *testing.T) {
	vd := &VectorDetector{}

	// 空格应保留，避免 "★ ★ ★" 和 "★    ★" 碰撞
	input1 := "★ ★ ★ ★ ★"
	input2 := "★    ★    ★    ★    ★"

	result1 := vd.normalizeParagraph(input1)
	result2 := vd.normalizeParagraph(input2)

	if result1 == result2 {
		t.Errorf("不同空格格式不应归一化为相同结果: %q vs %q", result1, result2)
	}
}

func Test_normalizeParagraph_should_remove_punctuation(t *testing.T) {
	vd := &VectorDetector{}

	input := "你好，世界！这是一个测试。"
	expected := "你好世界这是一个测试"

	result := vd.normalizeParagraph(input)
	if result != expected {
		t.Errorf("标点移除失败: 期望 %q, 实际 %q", expected, result)
	}
}

func Test_normalizeParagraph_should_preserve_parentheses(t *testing.T) {
	vd := &VectorDetector{}

	// 括号应保留，避免 "（3）" 和 "3" 碰撞
	input1 := "（3）"
	input2 := "3"

	result1 := vd.normalizeParagraph(input1)
	result2 := vd.normalizeParagraph(input2)

	if result1 == result2 {
		t.Errorf("括号内容不应与纯数字碰撞: %q vs %q", result1, result2)
	}
}

func Test_normalizeParagraph_should_trim_whitespace(t *testing.T) {
	vd := &VectorDetector{}

	input := "  你好世界  "
	expected := "你好世界"

	result := vd.normalizeParagraph(input)
	if result != expected {
		t.Errorf("空白修剪失败: 期望 %q, 实际 %q", expected, result)
	}
}

func Test_findDuplicateIndices_should_skip_short_paragraphs(t *testing.T) {
	vd := &VectorDetector{
		SimilarityThreshold: 0.95,
	}

	// 短段落（< 5 字符）不应参与精确匹配
	paragraphs := []string{
		"（1）",    // 3 字符（含括号）
		"（2）",    // 3 字符
		"（3）",    // 3 字符
		"这是一个测试段落",
		"这是另一个测试段落",
	}

	// 为每个段落生成向量（这里用简单的占位向量）
	vectors := make([][]float64, len(paragraphs))
	for i := range vectors {
		vectors[i] = []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7}
	}

	duplicates := vd.findDuplicateIndices(vectors, paragraphs)

	// 短段落不应被标记为重复
	for _, idx := range duplicates {
		if idx < 3 {
			t.Errorf("短段落 (index=%d) 不应被标记为重复", idx)
		}
	}
}

func Test_findDuplicateIndices_should_detect_long_duplicate_paragraphs(t *testing.T) {
	vd := &VectorDetector{
		SimilarityThreshold: 0.95,
	}

	// 长段落（>= 5 字符）应参与精确匹配
	paragraphs := []string{
		"这是一个足够长的测试段落",
		"这是另一个不同的段落",
		"这是一个足够长的测试段落", // 重复
	}

	// 为每个段落生成向量
	vectors := make([][]float64, len(paragraphs))
	for i := range vectors {
		vectors[i] = []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7}
	}

	duplicates := vd.findDuplicateIndices(vectors, paragraphs)

	// 第三个段落应被标记为重复
	found := false
	for _, idx := range duplicates {
		if idx == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("重复的长段落应被标记为重复")
	}
}

func Test_findDuplicateIndices_should_not_match_structural_elements(t *testing.T) {
	vd := &VectorDetector{
		SimilarityThreshold: 0.95,
	}

	// 结构元素（章节标题、编号等）不应被误判为重复
	paragraphs := []string{
		"暗黑英雄传（序章）", // 章节标题
		"暗黑英雄传（序章）", // 重复的章节标题
		"第1章 1",           // 章节编号
		"第2章 2",           // 不同的章节编号
		"这是正文内容，足够长以通过阈值检查",
	}

	// 使用差异较大的向量避免余弦相似度误判
	vectors := [][]float64{
		{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7}, // 段落 0
		{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7}, // 段落 1（与 0 相同，测试精确匹配）
		{0.9, 0.1, 0.8, 0.2, 0.7, 0.3, 0.6}, // 段落 2（差异大的向量）
		{0.1, 0.9, 0.2, 0.8, 0.3, 0.7, 0.4}, // 段落 3（差异大的向量）
		{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5}, // 段落 4
	}

	duplicates := vd.findDuplicateIndices(vectors, paragraphs)

	// 检查 "第1章 1" 和 "第2章 2" 不应被标记为重复
	for _, idx := range duplicates {
		if idx == 2 || idx == 3 {
			t.Errorf("章节编号 (index=%d) 不应被标记为重复", idx)
		}
	}

	// 检查重复的章节标题应被标记为重复
	found := false
	for _, idx := range duplicates {
		if idx == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("重复的章节标题应被标记为重复")
	}
}
