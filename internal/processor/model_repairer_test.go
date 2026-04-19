package processor

import (
	"strings"
	"testing"
	"txt-cleaning/internal/config"
)

func setupModelRepairerConfig(enableModelRepair bool) {
	config.AppConfigInstance.EnableModelRepair = enableModelRepair
}

func TestNewModelRepairer(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	if repairer.RepairModelType != "local" {
		t.Errorf("Expected RepairModelType 'local', got '%s'", repairer.RepairModelType)
	}
	if repairer.RepairModelName != "test-model" {
		t.Errorf("Expected RepairModelName 'test-model', got '%s'", repairer.RepairModelName)
	}
}

func TestRepairText_Disabled(t *testing.T) {
	setupModelRepairerConfig(false)
	repairer := NewModelRepairer("local", "test-model")

	result := repairer.RepairText("test content")

	if result.Content != "test content" {
		t.Errorf("Expected content unchanged, got '%s'", result.Content)
	}
	if len(result.Changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(result.Changes))
	}
}

func TestRepairText_EmptyContent(t *testing.T) {
	setupModelRepairerConfig(true)
	repairer := NewModelRepairer("local", "test-model")

	result := repairer.RepairText("")

	if result.Content != "" {
		t.Errorf("Expected empty content, got '%s'", result.Content)
	}
	if len(result.Changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(result.Changes))
	}
}

func TestRepairText_ShortContent(t *testing.T) {
	setupModelRepairerConfig(true)
	repairer := NewModelRepairer("local", "test-model")

	content := "短文本"
	result := repairer.RepairText(content)

	if result.Content != content {
		t.Errorf("Expected content unchanged, got '%s'", result.Content)
	}
	if len(result.Changes) != 0 {
		t.Errorf("Expected no changes for short content, got %d", len(result.Changes))
	}
}

func TestRepairText_NoTypos(t *testing.T) {
	setupModelRepairerConfig(true)
	repairer := NewModelRepairer("local", "test-model")

	content := "这是一个正确的段落文本，没有任何错别字。"
	result := repairer.RepairText(content)

	if len(result.Changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(result.Changes))
	}
}

func TestRepairText_WithTypos(t *testing.T) {
	setupModelRepairerConfig(true)
	repairer := NewModelRepairer("local", "test-model")

	content := "她高兴及了，跑过去抱住他。"
	result := repairer.RepairText(content)

	if len(result.Changes) == 0 {
		t.Error("Expected changes to be recorded for typos")
	}

	if result.Content != "她高兴极了，跑过去抱住他。" {
		t.Errorf("Expected corrected content, got '%s'", result.Content)
	}
}

func TestRepairText_MultipleParagraphs(t *testing.T) {
	setupModelRepairerConfig(true)
	repairer := NewModelRepairer("local", "test-model")

	content := "她高兴及了，跑过去。\n他在次见到了老朋友。"
	result := repairer.RepairText(content)

	if len(result.Changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(result.Changes))
	}

	if !strings.Contains(result.Content, "极了") {
		t.Error("Expected '极了' in repaired content")
	}
	if !strings.Contains(result.Content, "再次") {
		t.Error("Expected '再次' in repaired content")
	}
}

func TestRepairText_StatsVerification(t *testing.T) {
	setupModelRepairerConfig(true)
	repairer := NewModelRepairer("local", "test-model")

	content := "她高兴及了。\n他在次见面。"
	result := repairer.RepairText(content)

	if result.Stats["paragraphs_repaired"] != 2 {
		t.Errorf("Expected 2 paragraphs repaired, got %d", result.Stats["paragraphs_repaired"])
	}

	if result.Stats["total_changes"] != 2 {
		t.Errorf("Expected 2 total changes, got %d", result.Stats["total_changes"])
	}
}

func TestModelRepairerSplitIntoParagraphs_Normal(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	content := "段落1\n段落2\n段落3"
	paragraphs := repairer.SplitIntoParagraphs(content)

	if len(paragraphs) != 3 {
		t.Errorf("Expected 3 paragraphs, got %d", len(paragraphs))
	}
	if paragraphs[0] != "段落1" {
		t.Errorf("Expected '段落1', got '%s'", paragraphs[0])
	}
}

func TestModelRepairerSplitIntoParagraphs_FilterEmptyLines(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	content := "段落1\n\n\n段落2\n\n段落3"
	paragraphs := repairer.SplitIntoParagraphs(content)

	if len(paragraphs) != 3 {
		t.Errorf("Expected 3 paragraphs after filtering empty lines, got %d", len(paragraphs))
	}
}

func TestModelRepairerSplitIntoParagraphs_FilterWhitespace(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	content := "段落1\n   \n\t\t\n段落2"
	paragraphs := repairer.SplitIntoParagraphs(content)

	if len(paragraphs) != 2 {
		t.Errorf("Expected 2 paragraphs after filtering whitespace, got %d", len(paragraphs))
	}
}

func TestRepairParagraph_Short(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	paragraph := "短文本"
	repaired, changes := repairer.RepairParagraph(paragraph)

	if repaired != paragraph {
		t.Errorf("Expected paragraph unchanged, got '%s'", repaired)
	}
	if len(changes) != 0 {
		t.Errorf("Expected no changes for short paragraph, got %d", len(changes))
	}
}

func TestRepairParagraph_LocalMode(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	paragraph := "她高兴及了，跑过去。"
	repaired, changes := repairer.RepairParagraph(paragraph)

	if len(changes) == 0 {
		t.Error("Expected changes to be recorded")
	}
	if repaired != "她高兴极了，跑过去。" {
		t.Errorf("Expected corrected paragraph, got '%s'", repaired)
	}
}

func TestRepairLocally_NoTypos(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	text := "这是一个正确的段落文本。"
	repaired, changes := repairer.repairLocally(text)

	if repaired != text {
		t.Errorf("Expected text unchanged, got '%s'", repaired)
	}
	if len(changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(changes))
	}
}

func TestRepairLocally_SingleTypo(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	text := "她高兴及了，跑过去。"
	repaired, changes := repairer.repairLocally(text)

	if len(changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(changes))
	}
	if repaired != "她高兴极了，跑过去。" {
		t.Errorf("Expected '她高兴极了，跑过去。', got '%s'", repaired)
	}
	if changes[0].Original != "及了" {
		t.Errorf("Expected original '及了', got '%s'", changes[0].Original)
	}
	if changes[0].Replacement != "极了" {
		t.Errorf("Expected replacement '极了', got '%s'", changes[0].Replacement)
	}
}

func TestRepairLocally_MultipleTypos(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	text := "她高兴及了，在次见面。"
	repaired, changes := repairer.repairLocally(text)

	if len(changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(changes))
	}
	if !strings.Contains(repaired, "极了") {
		t.Error("Expected '极了' in repaired text")
	}
	if !strings.Contains(repaired, "再次") {
		t.Error("Expected '再次' in repaired text")
	}
}

func TestRepairLocally_TypoMapCoverage(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	testCases := []struct {
		input    string
		expected string
	}{
		{"我去图书管看书", "我去图书馆看书"},
		{"她高兴及了", "她高兴极了"},
		{"我们在次见面", "我们再次见面"},
		{"哪么好的事情", "那么好的事情"},
		{"我因该去", "我应该去"},
		{"任务以经完成", "任务已经完成"},
		{"他好象是", "他好像是"},
		{"我做车去", "我坐车去"},
	}

	for _, tc := range testCases {
		repaired, _ := repairer.repairLocally(tc.input)
		if repaired != tc.expected {
			t.Errorf("For input '%s', expected '%s', got '%s'", tc.input, tc.expected, repaired)
		}
	}
}

func TestRepairLocally_PreservedWords(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	text := "他坐的士去见她。"
	repaired, changes := repairer.repairLocally(text)

	if repaired != text {
		t.Errorf("Expected preserved words unchanged, got '%s'", repaired)
	}
	if len(changes) != 0 {
		t.Errorf("Expected no changes for preserved words, got %d", len(changes))
	}
}

func TestCompareTexts_Identical(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	text := "相同的文本内容"
	changes := repairer.compareTexts(text, text)

	if len(changes) != 0 {
		t.Errorf("Expected no changes for identical texts, got %d", len(changes))
	}
}

func TestCompareTexts_EqualLength(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	original := "ABC"
	repaired := "AXC"
	changes := repairer.compareTexts(original, repaired)

	if len(changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "character_correction" {
		t.Errorf("Expected type 'character_correction', got '%s'", changes[0].Type)
	}
	if changes[0].Position != 1 {
		t.Errorf("Expected position 1, got %d", changes[0].Position)
	}
	if changes[0].Original != "B" {
		t.Errorf("Expected original 'B', got '%s'", changes[0].Original)
	}
	if changes[0].Replacement != "X" {
		t.Errorf("Expected replacement 'X', got '%s'", changes[0].Replacement)
	}
}

func TestCompareTexts_DifferentLength(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	original := "短文本"
	repaired := "这是一个更长的文本"
	changes := repairer.compareTexts(original, repaired)

	if len(changes) < 1 {
		t.Errorf("Expected at least 1 change, got %d", len(changes))
	}

	hasCorrection := false
	for _, c := range changes {
		if c.Type == "character_correction" || c.Type == "text_insertion" || c.Type == "text_deletion" {
			hasCorrection = true
		}
	}
	if !hasCorrection {
		t.Errorf("Expected correction type change, got types: %v", changes)
	}
}

func TestCompareTexts_ChangeTypeVerification(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	original := "ABC"
	repaired := "AXC"
	changes := repairer.compareTexts(original, repaired)

	if len(changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(changes))
	}
	if changes[0].Type != "character_correction" {
		t.Errorf("Expected type 'character_correction', got '%s'", changes[0].Type)
	}
	if changes[0].Original != "B" {
		t.Errorf("Expected original 'B', got '%s'", changes[0].Original)
	}
	if changes[0].Replacement != "X" {
		t.Errorf("Expected replacement 'X', got '%s'", changes[0].Replacement)
	}
}

func TestCompareTexts_DifferentLengthChangeType(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	original := "原文本"
	repaired := "修改后的文本"
	changes := repairer.compareTexts(original, repaired)

	if len(changes) < 1 {
		t.Errorf("Expected at least 1 change, got %d", len(changes))
	}

	hasCorrection := false
	for _, c := range changes {
		if c.Type == "character_correction" || c.Type == "text_insertion" || c.Type == "text_deletion" {
			hasCorrection = true
		}
	}
	if !hasCorrection {
		t.Errorf("Expected correction type change, got types: %v", changes)
	}
}

func TestDetectCommonTypos_NoTypos(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	text := "正确的文本没有任何错别字"
	changes := repairer.detectCommonTypos(text)

	if len(changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(changes))
	}
}

func TestDetectCommonTypos_WithTypos(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	text := "她去图书管看书"
	changes := repairer.detectCommonTypos(text)

	if len(changes) == 0 {
		t.Error("Expected changes to be detected")
	}

	found := false
	for _, change := range changes {
		if change.Original == "管" && change.Replacement == "馆" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find '管' -> '馆' correction")
	}
}

func TestDetectCommonTypos_MultipleTypos(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	text := "她在次说哪么好"
	changes := repairer.detectCommonTypos(text)

	if len(changes) < 2 {
		t.Errorf("Expected at least 2 changes, got %d", len(changes))
	}
}

func TestBuildRepairPrompt(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	testText := "测试文本内容"
	prompt := repairer.buildRepairPrompt(testText)

	if !strings.Contains(prompt, "测试文本内容") {
		t.Error("Expected prompt to contain input text")
	}
	if !strings.Contains(prompt, "你是一个专业的中文小说校对编辑") {
		t.Error("Expected prompt to contain role description")
	}
	if !strings.Contains(prompt, "她高兴及了") {
		t.Error("Expected prompt to contain example")
	}
}

func TestBuildRepairPrompt_TextEmbedding(t *testing.T) {
	repairer := NewModelRepairer("local", "test-model")

	testText := "这是一个需要修复的段落"
	prompt := repairer.buildRepairPrompt(testText)

	if !strings.Contains(prompt, "输入：这是一个需要修复的段落") {
		t.Error("Expected prompt to contain input text with label")
	}
	if !strings.Contains(prompt, "输出：") {
		t.Error("Expected prompt to contain output label")
	}
}

func TestRepairText_NewlinesOnly(t *testing.T) {
	setupModelRepairerConfig(true)
	repairer := NewModelRepairer("local", "test-model")

	content := "\n\n\n"
	result := repairer.RepairText(content)

	if result.Content != content {
		t.Errorf("Expected content unchanged for newlines-only input, got '%s'", result.Content)
	}
	if len(result.Changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(result.Changes))
	}
}
