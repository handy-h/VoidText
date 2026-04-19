package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComputeContentMd5_ShouldReturnConsistentHash(t *testing.T) {
	content := "hello world"
	result := ComputeContentMd5(content)
	expected := "5eb63bbbe01eeed093cb22bb8f5acdc3"
	if result != expected {
		t.Errorf("ComputeContentMd5() = %s, want %s", result, expected)
	}
}

func TestComputeContentMd5_ShouldReturnSameHashForSameContent(t *testing.T) {
	content := "测试中文内容"
	result1 := ComputeContentMd5(content)
	result2 := ComputeContentMd5(content)
	if result1 != result2 {
		t.Errorf("ComputeContentMd5() inconsistent: %s != %s", result1, result2)
	}
}

func TestComputeContentMd5_ShouldReturnDifferentHashForDifferentContent(t *testing.T) {
	result1 := ComputeContentMd5("content A")
	result2 := ComputeContentMd5("content B")
	if result1 == result2 {
		t.Errorf("ComputeContentMd5() should return different hashes for different content")
	}
}

func TestComputeContentMd5_ShouldHandleEmptyString(t *testing.T) {
	result := ComputeContentMd5("")
	if result == "" {
		t.Errorf("ComputeContentMd5() should return non-empty hash for empty string")
	}
}

func TestComputeFileMd5_ShouldCalculateFileHash(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := ComputeFileMd5(testFile)
	if err != nil {
		t.Fatalf("ComputeFileMd5() error = %v", err)
	}
	expected := "5eb63bbbe01eeed093cb22bb8f5acdc3"
	if result != expected {
		t.Errorf("ComputeFileMd5() = %s, want %s", result, expected)
	}
}

func TestComputeFileMd5_ShouldReturnErrorForNonExistentFile(t *testing.T) {
	_, err := ComputeFileMd5("/nonexistent/file.txt")
	if err == nil {
		t.Errorf("ComputeFileMd5() should return error for non-existent file")
	}
}

func TestComputeFileMd5_ShouldReturnSameHashForSameFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("same content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result1, err1 := ComputeFileMd5(testFile)
	result2, err2 := ComputeFileMd5(testFile)
	if err1 != nil || err2 != nil {
		t.Fatalf("ComputeFileMd5() unexpected error: %v, %v", err1, err2)
	}
	if result1 != result2 {
		t.Errorf("ComputeFileMd5() inconsistent: %s != %s", result1, result2)
	}
}
