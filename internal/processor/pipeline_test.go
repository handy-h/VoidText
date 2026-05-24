package processor

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"voidtext/internal/config"
	"voidtext/internal/database"
	"voidtext/internal/processor/preprocess"
)

func initPipelineTestConfig() {
	config.AppConfigInstance = config.AppConfig{
		EnableBasicCleaning:       true,
		EnableVectorDetection:     true,
		EnableModelRepair:         true,
		TraditionalToSimple:       false,
		VectorSimilarityThreshold: 0.95,
		VectorModelType:           "local",
		VectorModelName:           "test-model",
		RepairModelType:           "local",
		RepairModelName:           "test-model",
		DataDir:                   "/tmp/voidtext_test",
	}
}

func TestGetNextStep_ShouldReturnNextStep(t *testing.T) {
	tests := []struct {
		current string
		want    string
	}{
		{StepCleaning, StepIndexing},
		{StepIndexing, StepLlmFix},
		{StepLlmFix, StepReview},
		{StepReview, StepFinalizing},
		{StepFinalizing, ""},
	}

	for _, tt := range tests {
		result := GetNextStep(tt.current)
		if result != tt.want {
			t.Errorf("GetNextStep(%s) = %s, want %s", tt.current, result, tt.want)
		}
	}
}

func TestGetNextStep_ShouldReturnFirstStepForUnknown(t *testing.T) {
	result := GetNextStep("unknown_step")
	if result != StepCleaning {
		t.Errorf("GetNextStep(unknown) = %s, want %s", result, StepCleaning)
	}
}

func TestGetStepIndex_ShouldReturnCorrectIndex(t *testing.T) {
	tests := []struct {
		step string
		want int
	}{
		{StepCleaning, 0},
		{StepIndexing, 1},
		{StepLlmFix, 2},
		{StepReview, 3},
		{StepFinalizing, 4},
	}

	for _, tt := range tests {
		result := GetStepIndex(tt.step)
		if result != tt.want {
			t.Errorf("GetStepIndex(%s) = %d, want %d", tt.step, result, tt.want)
		}
	}
}

func TestCalculateProgress_ShouldReturnCorrectPercentage(t *testing.T) {
	tests := []struct {
		step         string
		stepProgress int
		wantMin      int
		wantMax      int
	}{
		{StepCleaning, 0, 0, 20},
		{StepCleaning, 100, 20, 20},
		{StepIndexing, 0, 20, 40},
		{StepLlmFix, 50, 40, 60},
		{StepReview, 0, 60, 80},
		{StepFinalizing, 0, 80, 100},
	}

	for _, tt := range tests {
		result := CalculateProgress(tt.step, tt.stepProgress)
		if result < tt.wantMin || result > tt.wantMax {
			t.Errorf("CalculateProgress(%s, %d) = %d, want between %d and %d",
				tt.step, tt.stepProgress, result, tt.wantMin, tt.wantMax)
		}
	}
}

func TestDefaultRulesConfig_ShouldReturnConfig(t *testing.T) {
	config.AppConfigInstance = config.AppConfig{
		EnableBasicCleaning:       true,
		TraditionalToSimple:       false,
		EnableVectorDetection:     true,
		VectorSimilarityThreshold: 0.95,
		EnableModelRepair:         true,
	}

	cfg := DefaultRulesConfig()
	if !cfg.EnableBasicCleaning {
		t.Errorf("DefaultRulesConfig() EnableBasicCleaning should be true")
	}
	if cfg.SimilarityThreshold != 0.95 {
		t.Errorf("DefaultRulesConfig() SimilarityThreshold = %f, want 0.95", cfg.SimilarityThreshold)
	}
}

func TestParseRulesConfig_ShouldParseValidJSON(t *testing.T) {
	jsonStr := `{"enableBasicCleaning":true,"enableVectorDetection":false}`
	cfg := ParseRulesConfig(jsonStr)

	if !cfg.EnableBasicCleaning {
		t.Errorf("ParseRulesConfig() EnableBasicCleaning should be true")
	}
	if cfg.EnableVectorDetection {
		t.Errorf("ParseRulesConfig() EnableVectorDetection should be false")
	}
}

func TestParseRulesConfig_ShouldReturnDefaultForEmpty(t *testing.T) {
	config.AppConfigInstance = config.AppConfig{
		EnableBasicCleaning:       true,
		TraditionalToSimple:       false,
		EnableVectorDetection:     true,
		VectorSimilarityThreshold: 0.95,
		EnableModelRepair:         true,
	}

	cfg := ParseRulesConfig("")
	if !cfg.EnableBasicCleaning {
		t.Errorf("ParseRulesConfig('') should return default config")
	}
}

func TestParseRulesConfig_ShouldReturnDefaultForInvalidJSON(t *testing.T) {
	config.AppConfigInstance = config.AppConfig{
		EnableBasicCleaning:       true,
		TraditionalToSimple:       false,
		EnableVectorDetection:     true,
		VectorSimilarityThreshold: 0.95,
		EnableModelRepair:         true,
	}

	cfg := ParseRulesConfig("invalid json")
	if !cfg.EnableBasicCleaning {
		t.Errorf("ParseRulesConfig() should return default for invalid JSON")
	}
}

func TestApplySuggestion_ShouldApplyAtPosition(t *testing.T) {
	content := "她高兴及了"
	suggestion := preprocess.Change{
		Original:    "及了",
		Replacement: "极了",
		Position:    6,
	}

	result := ApplySuggestion(content, suggestion)
	if result != "她高兴极了" {
		t.Errorf("ApplySuggestion() = %s, want 她高兴极了", result)
	}
}

func TestApplySuggestion_ShouldFallbackToStringSearch(t *testing.T) {
	content := "她高兴及了"
	suggestion := preprocess.Change{
		Original:    "及了",
		Replacement: "极了",
		Position:    -1,
	}

	result := ApplySuggestion(content, suggestion)
	if result != "她高兴极了" {
		t.Errorf("ApplySuggestion() with fallback = %s, want 她高兴极了", result)
	}
}

func TestApplySuggestion_ShouldHandleEmptyOriginal(t *testing.T) {
	content := "测试内容"
	suggestion := preprocess.Change{
		Original:    "",
		Replacement: "",
	}

	result := ApplySuggestion(content, suggestion)
	if result != content {
		t.Errorf("ApplySuggestion() with empty original should return unchanged content")
	}
}

func TestApplyAllSuggestions_ShouldApplyInReverseOrder(t *testing.T) {
	content := "ABCDEF"
	suggestions := []preprocess.Change{
		{Original: "AB", Replacement: "ab", Position: 0},
		{Original: "EF", Replacement: "ef", Position: 4},
	}

	result := ApplyAllSuggestions(content, suggestions)
	if result != "abCDef" {
		t.Errorf("ApplyAllSuggestions() = %s, want abCDef", result)
	}
}

func TestMergeStats_ShouldCombineStats(t *testing.T) {
	target := map[string]int{"a": 1, "b": 2}
	source := map[string]int{"a": 3, "c": 4}

	mergeStats(target, source)

	if target["a"] != 4 {
		t.Errorf("mergeStats() a = %d, want 4", target["a"])
	}
	if target["b"] != 2 {
		t.Errorf("mergeStats() b = %d, want 2", target["b"])
	}
	if target["c"] != 4 {
		t.Errorf("mergeStats() c = %d, want 4", target["c"])
	}
}

func TestProcessLlmFixStep_ShouldCompleteWithConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	if err := database.Init(tmpDir); err != nil {
		t.Fatalf("database.Init() error = %v", err)
	}
	defer database.Close()

	uploadsDir := filepath.Join(tmpDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}

	fileMd5 := "llm-fix-concurrency-md5"
	inputPath := filepath.Join(uploadsDir, fileMd5+".txt")
	content := "这是第一段内容，长度足够触发本地修复逻辑。\n这是第二段内容，长度也足够触发本地修复逻辑。\n这是第三段内容，继续用于测试并发完成。"
	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	config.AppConfigInstance = config.AppConfig{
		EnableModelRepair: true,
		RepairModelType:   "local",
		RepairModelName:   "test-model",
		LLMConcurrency:    2,
		DataDir:           tmpDir,
	}

	record := &database.FileRecord{
		Md5:      fileMd5,
		FileName: "test.txt",
		FilePath: inputPath,
		Status:   "processing",
	}
	if err := database.CreateFile(record); err != nil {
		t.Fatalf("database.CreateFile() error = %v", err)
	}

	resultCh := make(chan *PipelineResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := processLlmFixStep(fileMd5, content, RulesConfig{EnableModelRepair: true}, record)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case err := <-errCh:
		t.Fatalf("processLlmFixStep() error = %v", err)
	case result := <-resultCh:
		if result.NextStep != StepReview {
			t.Fatalf("NextStep = %s, want %s", result.NextStep, StepReview)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("processLlmFixStep() timed out, possible worker channel deadlock")
	}
}
