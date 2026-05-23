package processor

import (
	"math"
	"testing"

	"voidtext/internal/config"
)

func initTestConfig() {
	config.AppConfigInstance = config.AppConfig{
		EnableVectorDetection:     true,
		VectorSimilarityThreshold: 0.95,
		VectorModelType:           "local",
		VectorModelName:           "test-model",
	}
}

func TestNewVectorDetector_ShouldCreateDetector(t *testing.T) {
	vd := NewVectorDetector(0.95, "local", "test-model")
	if vd == nil {
		t.Fatalf("NewVectorDetector() should return non-nil")
	}
	if vd.SimilarityThreshold != 0.95 {
		t.Errorf("SimilarityThreshold = %f, want 0.95", vd.SimilarityThreshold)
	}
}

func TestSplitIntoParagraphs_ShouldSplitByNewline(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := vd.splitIntoParagraphs("第一段\n第二段\n第三段")
	if len(paragraphs) != 3 {
		t.Errorf("splitIntoParagraphs() length = %d, want 3", len(paragraphs))
	}
}

func TestSplitIntoParagraphs_ShouldFilterEmpty(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := vd.splitIntoParagraphs("第一段\n\n\n第二段")
	if len(paragraphs) != 2 {
		t.Errorf("splitIntoParagraphs() length = %d, want 2", len(paragraphs))
	}
}

func TestSplitIntoParagraphs_ShouldHandleEmptyInput(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := vd.splitIntoParagraphs("")
	if len(paragraphs) != 0 {
		t.Errorf("splitIntoParagraphs() length = %d, want 0", len(paragraphs))
	}
}

func TestCalculateCosineSimilarity_ShouldReturnOneForIdentical(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	vec := []float64{1.0, 2.0, 3.0}
	result := vd.calculateCosineSimilarity(vec, vec)

	if math.Abs(result-1.0) > 0.001 {
		t.Errorf("calculateCosineSimilarity() = %f, want 1.0", result)
	}
}

func TestCalculateCosineSimilarity_ShouldReturnZeroForOrthogonal(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	vec1 := []float64{1.0, 0.0}
	vec2 := []float64{0.0, 1.0}
	result := vd.calculateCosineSimilarity(vec1, vec2)

	if math.Abs(result) > 0.001 {
		t.Errorf("calculateCosineSimilarity() = %f, want 0.0", result)
	}
}

func TestCalculateCosineSimilarity_ShouldReturnZeroForDifferentLengths(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	vec1 := []float64{1.0, 2.0}
	vec2 := []float64{1.0}
	result := vd.calculateCosineSimilarity(vec1, vec2)

	if result != 0.0 {
		t.Errorf("calculateCosineSimilarity() = %f, want 0.0 for different lengths", result)
	}
}

func TestCalculateCosineSimilarity_ShouldReturnZeroForZeroVectors(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	vec1 := []float64{0.0, 0.0}
	vec2 := []float64{1.0, 2.0}
	result := vd.calculateCosineSimilarity(vec1, vec2)

	if result != 0.0 {
		t.Errorf("calculateCosineSimilarity() = %f, want 0.0 for zero vector", result)
	}
}

func TestNormalizeParagraph_ShouldRemovePunctuation(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	result := vd.normalizeParagraph("你好，世界！")
	if result != "你好世界" {
		t.Errorf("normalizeParagraph() = %s, want 你好世界", result)
	}
}

func TestNormalizeParagraph_ShouldRemoveWhitespace(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	result := vd.normalizeParagraph("你 好 世 界")
	if result != "你好世界" {
		t.Errorf("normalizeParagraph() = %s, want 你好世界", result)
	}
}

func TestDetectDuplicates_ShouldDetectExactDuplicates(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	content := "第一段内容\n第二段内容\n第一段内容"
	result, err := vd.DetectDuplicates(content)
	if err != nil {
		t.Fatalf("DetectDuplicates() unexpected error: %v", err)
	}

	if result.Stats["duplicate_paragraphs_removed"] != 1 {
		t.Errorf("DetectDuplicates() should detect 1 duplicate, got %d", result.Stats["duplicate_paragraphs_removed"])
	}
}

func TestDetectDuplicates_ShouldNotDetectNonDuplicates(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	content := "第一段内容\n第二段内容\n第三段内容"
	result, err := vd.DetectDuplicates(content)
	if err != nil {
		t.Fatalf("DetectDuplicates() unexpected error: %v", err)
	}

	if result.Stats["duplicate_paragraphs_removed"] != 0 {
		t.Errorf("DetectDuplicates() should not detect duplicates for different paragraphs")
	}
}

func TestDetectDuplicates_ShouldHandleSingleParagraph(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	content := "只有一段内容"
	result, err := vd.DetectDuplicates(content)
	if err != nil {
		t.Fatalf("DetectDuplicates() unexpected error: %v", err)
	}

	if len(result.Changes) != 0 {
		t.Errorf("DetectDuplicates() should not detect duplicates for single paragraph")
	}
}

func TestGenerateVectors_ShouldReturnCorrectLength(t *testing.T) {
	initTestConfig()
	vd := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := []string{"段落一", "段落二", "段落三"}
	vectors, err := vd.generateVectors(paragraphs)
	if err != nil {
		t.Fatalf("generateVectors() unexpected error: %v", err)
	}

	if len(vectors) != 3 {
		t.Errorf("generateVectors() length = %d, want 3", len(vectors))
	}
	for _, vec := range vectors {
		if len(vec) != 7 {
			t.Errorf("generateVectors() vector length = %d, want 7", len(vec))
		}
	}
}
