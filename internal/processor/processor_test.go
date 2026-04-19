package processor

import (
	"testing"
	"txt-cleaning/internal/config"
	"txt-cleaning/internal/processor/preprocess"
)

func setupProcessorConfig() {
	config.AppConfigInstance.EnableBasicCleaning = true
	config.AppConfigInstance.EnableVectorDetection = true
	config.AppConfigInstance.EnableModelRepair = true
	config.AppConfigInstance.TraditionalToSimple = false
	config.AppConfigInstance.VectorModelType = "local"
	config.AppConfigInstance.VectorModelName = "test-model"
	config.AppConfigInstance.VectorSimilarityThreshold = 0.95
	config.AppConfigInstance.RepairModelType = "local"
	config.AppConfigInstance.RepairModelName = "test-model"
}

func TestProcess_EmptyContent(t *testing.T) {
	setupProcessorConfig()

	result := Process("")

	if result.Content != "" {
		t.Errorf("Expected empty content, got '%s'", result.Content)
	}
	if result.Original != "" {
		t.Errorf("Expected empty original, got '%s'", result.Original)
	}
}

func TestProcess_NormalContent(t *testing.T) {
	setupProcessorConfig()

	content := "这是一段正常的文本。"
	result := Process(content)

	if result.Original != content {
		t.Errorf("Expected original to be set, got '%s'", result.Original)
	}
}

func TestProcess_ProcessingStages(t *testing.T) {
	setupProcessorConfig()

	content := "这是一段正常的文本。"
	result := Process(content)

	if len(result.ProcessingStages) == 0 {
		t.Error("Expected processing stages to be recorded")
	}
}

func TestProcess_StatsMerged(t *testing.T) {
	setupProcessorConfig()

	content := "这是一段正常的文本。"
	result := Process(content)

	if len(result.Stats) == 0 {
		t.Error("Expected stats to be recorded")
	}
}

func TestProcessWithReview(t *testing.T) {
	setupProcessorConfig()

	content := "这是一段正常的文本。"
	result, err := ProcessWithReview(content, "test_file", "test_process")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.ReviewSessionID == "" {
		t.Error("Expected review session ID to be set")
	}
}

func TestGetSuggestions(t *testing.T) {
	setupProcessorConfig()

	content := "这是一段正常的文本。"
	suggestions := GetSuggestions(content)

	if suggestions == nil {
		t.Error("Expected suggestions to be returned")
	}
}

func TestApplySuggestion_PositionMatch(t *testing.T) {
	content := "Hello World"
	suggestion := preprocess.Change{
		Type:        "typo",
		Original:    "World",
		Replacement: "世界",
		Position:    6,
	}

	result := ApplySuggestion(content, suggestion)

	if result != "Hello 世界" {
		t.Errorf("Expected 'Hello 世界', got '%s'", result)
	}
}

func TestApplySuggestion_EmptyOriginal(t *testing.T) {
	content := "Hello World"
	suggestion := preprocess.Change{
		Type:        "typo",
		Original:    "",
		Replacement: "世界",
		Position:    0,
	}

	result := ApplySuggestion(content, suggestion)

	if result != content {
		t.Errorf("Expected content unchanged for empty original, got '%s'", result)
	}
}

func TestApplySuggestion_SingleReplacement(t *testing.T) {
	content := "Hello World World"
	suggestion := preprocess.Change{
		Type:        "typo",
		Original:    "World",
		Replacement: "世界",
		Position:    -1,
	}

	result := ApplySuggestion(content, suggestion)

	if result != "Hello 世界 World" {
		t.Errorf("Expected 'Hello 世界 World', got '%s'", result)
	}
}

func TestApplySuggestion_DuplicateParagraph(t *testing.T) {
	content := "段落1\n\n段落2\n\n段落1"
	suggestion := preprocess.Change{
		Type:        "duplicate_paragraph",
		Original:    "段落1",
		Replacement: "",
		Position:    -1,
	}

	result := ApplySuggestion(content, suggestion)

	if result == content {
		t.Error("Expected duplicate paragraph to be removed")
	}
}

func TestApplyAllSuggestions(t *testing.T) {
	content := "Hello World"
	suggestions := []preprocess.Change{
		{
			Type:        "typo",
			Original:    "Hello",
			Replacement: "你好",
			Position:    0,
		},
		{
			Type:        "typo",
			Original:    "World",
			Replacement: "世界",
			Position:    6,
		},
	}

	result := ApplyAllSuggestions(content, suggestions)

	if result != "你好 世界" {
		t.Errorf("Expected '你好 世界', got '%s'", result)
	}
}

func TestApplyAllSuggestions_SortedByPosition(t *testing.T) {
	content := "A B C"
	suggestions := []preprocess.Change{
		{
			Type:        "typo",
			Original:    "A",
			Replacement: "1",
			Position:    0,
		},
		{
			Type:        "typo",
			Original:    "C",
			Replacement: "3",
			Position:    4,
		},
		{
			Type:        "typo",
			Original:    "B",
			Replacement: "2",
			Position:    2,
		},
	}

	result := ApplyAllSuggestions(content, suggestions)

	if result != "1 2 3" {
		t.Errorf("Expected '1 2 3', got '%s'", result)
	}
}

func TestMergeStats(t *testing.T) {
	target := map[string]int{"a": 1, "b": 2}
	source := map[string]int{"b": 3, "c": 4}

	mergeStats(target, source)

	if target["a"] != 1 {
		t.Errorf("Expected target['a'] to be 1, got %d", target["a"])
	}
	if target["b"] != 5 {
		t.Errorf("Expected target['b'] to be 5, got %d", target["b"])
	}
	if target["c"] != 4 {
		t.Errorf("Expected target['c'] to be 4, got %d", target["c"])
	}
}

func TestGetRuleManager(t *testing.T) {
	manager := GetRuleManager()

	if manager == nil {
		t.Error("Expected rule manager to be returned")
	}
}

func TestGetReviewManager(t *testing.T) {
	manager := GetReviewManager()

	if manager == nil {
		t.Error("Expected review manager to be returned")
	}
}
