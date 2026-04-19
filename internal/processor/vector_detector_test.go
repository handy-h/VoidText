package processor

import (
	"math"
	"testing"
	"txt-cleaning/internal/config"
	"txt-cleaning/internal/processor/preprocess"
)

func setupTestConfig(enableVectorDetection bool) {
	config.AppConfigInstance.EnableVectorDetection = enableVectorDetection
}

func TestNewVectorDetector(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	if detector.SimilarityThreshold != 0.95 {
		t.Errorf("Expected SimilarityThreshold 0.95, got %f", detector.SimilarityThreshold)
	}
	if detector.VectorModelType != "local" {
		t.Errorf("Expected VectorModelType 'local', got '%s'", detector.VectorModelType)
	}
	if detector.VectorModelName != "test-model" {
		t.Errorf("Expected VectorModelName 'test-model', got '%s'", detector.VectorModelName)
	}
}

func TestDetectDuplicates_Disabled(t *testing.T) {
	setupTestConfig(false)
	detector := NewVectorDetector(0.95, "local", "test-model")

	result := detector.DetectDuplicates("test content")

	if result.Content != "test content" {
		t.Errorf("Expected content unchanged, got '%s'", result.Content)
	}
	if len(result.Changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(result.Changes))
	}
}

func TestDetectDuplicates_EmptyContent(t *testing.T) {
	setupTestConfig(true)
	detector := NewVectorDetector(0.95, "local", "test-model")

	result := detector.DetectDuplicates("")

	if result.Content != "" {
		t.Errorf("Expected empty content, got '%s'", result.Content)
	}
	if len(result.Changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(result.Changes))
	}
}

func TestDetectDuplicates_SingleParagraph(t *testing.T) {
	setupTestConfig(true)
	detector := NewVectorDetector(0.95, "local", "test-model")

	content := "这是一个单独的段落。"
	result := detector.DetectDuplicates(content)

	if result.Content != content {
		t.Errorf("Expected content unchanged, got '%s'", result.Content)
	}
	if len(result.Changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(result.Changes))
	}
}

func TestDetectDuplicates_NoDuplicates(t *testing.T) {
	setupTestConfig(true)
	detector := NewVectorDetector(0.95, "local", "test-model")

	content := "第一段内容。\n第二段不同的内容。\n第三段完全不一样的文字。"
	result := detector.DetectDuplicates(content)

	if len(result.Changes) != 0 {
		t.Errorf("Expected no changes, got %d", len(result.Changes))
	}
}

func TestDetectDuplicates_WithDuplicates(t *testing.T) {
	setupTestConfig(true)
	detector := NewVectorDetector(0.95, "local", "test-model")

	content := "第一段内容。\n第二段内容。\n第一段内容。"
	result := detector.DetectDuplicates(content)

	if len(result.Changes) == 0 {
		t.Error("Expected changes to be recorded for duplicate paragraphs")
	}

	if result.Stats["duplicate_paragraphs_removed"] != 1 {
		t.Errorf("Expected 1 duplicate removed, got %d", result.Stats["duplicate_paragraphs_removed"])
	}
}

func TestDetectDuplicates_MultipleDuplicates(t *testing.T) {
	setupTestConfig(true)
	detector := NewVectorDetector(0.95, "local", "test-model")

	content := "段落A。\n段落B。\n段落A。\n段落C。\n段落B。"
	result := detector.DetectDuplicates(content)

	if result.Stats["duplicate_paragraphs_removed"] != 2 {
		t.Errorf("Expected 2 duplicates removed, got %d", result.Stats["duplicate_paragraphs_removed"])
	}
}

func TestSplitIntoParagraphs_Normal(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	content := "段落1\n段落2\n段落3"
	paragraphs := detector.splitIntoParagraphs(content)

	if len(paragraphs) != 3 {
		t.Errorf("Expected 3 paragraphs, got %d", len(paragraphs))
	}
	if paragraphs[0] != "段落1" {
		t.Errorf("Expected '段落1', got '%s'", paragraphs[0])
	}
}

func TestSplitIntoParagraphs_FilterEmptyLines(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	content := "段落1\n\n\n段落2\n\n段落3"
	paragraphs := detector.splitIntoParagraphs(content)

	if len(paragraphs) != 3 {
		t.Errorf("Expected 3 paragraphs after filtering empty lines, got %d", len(paragraphs))
	}
}

func TestSplitIntoParagraphs_FilterWhitespace(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	content := "段落1\n   \n\t\t\n段落2"
	paragraphs := detector.splitIntoParagraphs(content)

	if len(paragraphs) != 2 {
		t.Errorf("Expected 2 paragraphs after filtering whitespace, got %d", len(paragraphs))
	}
}

func TestGenerateVectors_EmptyArray(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	vectors := detector.generateVectors([]string{})

	if len(vectors) != 0 {
		t.Errorf("Expected empty vectors, got %d", len(vectors))
	}
}

func TestGenerateVectors_Dimension(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := []string{"测试段落内容。"}
	vectors := detector.generateVectors(paragraphs)

	if len(vectors) != 1 {
		t.Errorf("Expected 1 vector, got %d", len(vectors))
	}
	if len(vectors[0]) != 3 {
		t.Errorf("Expected vector dimension 3, got %d", len(vectors[0]))
	}
}

func TestGenerateVectors_Normalization(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := []string{"这是一个测试段落。它包含句号。"}
	vectors := detector.generateVectors(paragraphs)

	vector := vectors[0]
	if vector[0] <= 0 {
		t.Error("Expected positive length value")
	}

	expectedNormalized := float64(2) / vector[0]
	if math.Abs(vector[1]-expectedNormalized) > 0.0001 {
		t.Errorf("Expected normalized value %f, got %f", expectedNormalized, vector[1])
	}
}

func TestFindDuplicateIndices_NoDuplicates(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := []string{"段落A", "段落B", "段落C"}
	vectors := detector.generateVectors(paragraphs)

	indices := detector.findDuplicateIndices(vectors, paragraphs)

	if len(indices) != 0 {
		t.Errorf("Expected no duplicate indices, got %d", len(indices))
	}
}

func TestFindDuplicateIndices_WithDuplicates(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := []string{"段落A", "段落B", "段落A", "段落C"}
	vectors := detector.generateVectors(paragraphs)

	indices := detector.findDuplicateIndices(vectors, paragraphs)

	if len(indices) != 1 {
		t.Errorf("Expected 1 duplicate index, got %d", len(indices))
	}
	if indices[0] != 2 {
		t.Errorf("Expected duplicate at index 2, got %d", indices[0])
	}
}

func TestFindDuplicateIndices_PunctuationDifference(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := []string{"段落内容", "段落内容。"}
	vectors := detector.generateVectors(paragraphs)

	indices := detector.findDuplicateIndices(vectors, paragraphs)

	if len(indices) != 1 {
		t.Errorf("Expected 1 duplicate (punctuation difference), got %d", len(indices))
	}
}

func TestNormalizeParagraph_RemovePunctuation(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	input := "测试，内容。！"
	result := detector.normalizeParagraph(input)

	if result != "测试内容" {
		t.Errorf("Expected '测试内容', got '%s'", result)
	}
}

func TestNormalizeParagraph_RemoveWhitespace(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	input := "测 试\t内\n容\r"
	result := detector.normalizeParagraph(input)

	if result != "测试内容" {
		t.Errorf("Expected '测试内容', got '%s'", result)
	}
}

func TestNormalizeParagraph_EmptyString(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	result := detector.normalizeParagraph("")

	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestRemoveDuplicateParagraphs_SingleDuplicate(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := []string{"段落A", "段落B", "段落A"}
	duplicateIndices := []int{2}

	result := VectorDetectionResult{
		Content:  "",
		Original: "",
		Changes:  []preprocess.Change{},
		Stats:    make(map[string]int),
	}

	result = detector.removeDuplicateParagraphs(result, paragraphs, duplicateIndices)

	if result.Stats["duplicate_paragraphs_removed"] != 1 {
		t.Errorf("Expected 1 duplicate removed, got %d", result.Stats["duplicate_paragraphs_removed"])
	}

	if len(result.Changes) != 1 {
		t.Errorf("Expected 1 change recorded, got %d", len(result.Changes))
	}

	if result.Changes[0].Type != "duplicate_paragraph" {
		t.Errorf("Expected change type 'duplicate_paragraph', got '%s'", result.Changes[0].Type)
	}
}

func TestRemoveDuplicateParagraphs_MultipleDuplicates(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	paragraphs := []string{"段落A", "段落B", "段落A", "段落C", "段落B"}
	duplicateIndices := []int{2, 4}

	result := VectorDetectionResult{
		Content:  "",
		Original: "",
		Changes:  []preprocess.Change{},
		Stats:    make(map[string]int),
	}

	result = detector.removeDuplicateParagraphs(result, paragraphs, duplicateIndices)

	if result.Stats["duplicate_paragraphs_removed"] != 2 {
		t.Errorf("Expected 2 duplicates removed, got %d", result.Stats["duplicate_paragraphs_removed"])
	}

	if len(result.Changes) != 2 {
		t.Errorf("Expected 2 changes recorded, got %d", len(result.Changes))
	}
}

func TestCalculateCosineSimilarity_IdenticalVectors(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	vec1 := []float64{1.0, 2.0, 3.0}
	vec2 := []float64{1.0, 2.0, 3.0}

	similarity := detector.calculateCosineSimilarity(vec1, vec2)

	if math.Abs(similarity-1.0) > 0.0001 {
		t.Errorf("Expected similarity 1.0, got %f", similarity)
	}
}

func TestCalculateCosineSimilarity_OrthogonalVectors(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	vec1 := []float64{1.0, 0.0, 0.0}
	vec2 := []float64{0.0, 1.0, 0.0}

	similarity := detector.calculateCosineSimilarity(vec1, vec2)

	if math.Abs(similarity-0.0) > 0.0001 {
		t.Errorf("Expected similarity 0.0, got %f", similarity)
	}
}

func TestCalculateCosineSimilarity_DifferentDimensions(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	vec1 := []float64{1.0, 2.0}
	vec2 := []float64{1.0, 2.0, 3.0}

	similarity := detector.calculateCosineSimilarity(vec1, vec2)

	if similarity != 0.0 {
		t.Errorf("Expected similarity 0.0 for different dimensions, got %f", similarity)
	}
}

func TestCalculateCosineSimilarity_ZeroVector(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	vec1 := []float64{0.0, 0.0, 0.0}
	vec2 := []float64{1.0, 2.0, 3.0}

	similarity := detector.calculateCosineSimilarity(vec1, vec2)

	if similarity != 0.0 {
		t.Errorf("Expected similarity 0.0 for zero vector, got %f", similarity)
	}
}

func TestGenerateEmbeddings_LocalMode(t *testing.T) {
	detector := NewVectorDetector(0.95, "local", "test-model")

	texts := []string{"测试文本1", "测试文本2"}
	vectors, err := detector.generateEmbeddings(texts)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(vectors) != 2 {
		t.Errorf("Expected 2 vectors, got %d", len(vectors))
	}

	if len(vectors[0]) != 3 {
		t.Errorf("Expected vector dimension 3, got %d", len(vectors[0]))
	}
}

func TestGenerateEmbeddings_APIMode_FallbackToLocal(t *testing.T) {
	detector := NewVectorDetector(0.95, "api", "test-model")

	texts := []string{"测试文本"}
	vectors, err := detector.generateEmbeddings(texts)

	if err != nil {
		t.Errorf("Expected no error (fallback to local), got %v", err)
	}

	if len(vectors) != 1 {
		t.Errorf("Expected 1 vector, got %d", len(vectors))
	}
}

func TestDetectDuplicates_StatsVerification(t *testing.T) {
	setupTestConfig(true)
	detector := NewVectorDetector(0.95, "local", "test-model")

	content := "重复段落。\n不同段落。\n重复段落。\n另一个重复段落。\n另一个重复段落。"
	result := detector.DetectDuplicates(content)

	if _, exists := result.Stats["duplicate_paragraphs_removed"]; !exists {
		t.Error("Expected 'duplicate_paragraphs_removed' in stats")
	}

	if result.Stats["duplicate_paragraphs_removed"] != 2 {
		t.Errorf("Expected 2 duplicates removed, got %d", result.Stats["duplicate_paragraphs_removed"])
	}
}
