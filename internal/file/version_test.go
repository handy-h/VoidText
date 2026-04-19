package file

import (
	"os"
	"testing"
	"txt-cleaning/internal/config"
)

func setupVersionTest(t *testing.T) (*VersionManager, func()) {
	tempDir, err := os.MkdirTemp("", "version_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	config.AppConfigInstance.DataDir = tempDir

	manager := NewVersionManager()

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return manager, cleanup
}

func TestNewVersionManager(t *testing.T) {
	manager := NewVersionManager()

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
}

func TestCreateVersion(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	version, err := manager.CreateVersion("file_1", "content 1", "initial version")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	if version.FileID != "file_1" {
		t.Errorf("Expected file ID 'file_1', got '%s'", version.FileID)
	}
	if version.Version != "1" {
		t.Errorf("Expected version '1', got '%s'", version.Version)
	}
	if version.Content != "content 1" {
		t.Errorf("Expected content 'content 1', got '%s'", version.Content)
	}
	if version.Size != int64(len("content 1")) {
		t.Errorf("Expected size %d, got %d", len("content 1"), version.Size)
	}
	if version.Note != "initial version" {
		t.Errorf("Expected note 'initial version', got '%s'", version.Note)
	}
}

func TestCreateVersion_Multiple(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.CreateVersion("file_2", "content 1", "version 1")
	if err != nil {
		t.Fatalf("Failed to create version 1: %v", err)
	}

	version2, err := manager.CreateVersion("file_2", "content 2", "version 2")
	if err != nil {
		t.Fatalf("Failed to create version 2: %v", err)
	}

	if version2.Version != "2" {
		t.Errorf("Expected version '2', got '%s'", version2.Version)
	}
}

func TestGetVersions(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.CreateVersion("file_3", "content 1", "version 1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}
	_, err = manager.CreateVersion("file_3", "content 2", "version 2")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	versions, err := manager.GetVersions("file_3")
	if err != nil {
		t.Fatalf("Failed to get versions: %v", err)
	}

	if len(versions) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(versions))
	}
}

func TestGetVersion_Existing(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.CreateVersion("file_4", "content 1", "version 1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	version, err := manager.GetVersion("file_4", "1")
	if err != nil {
		t.Fatalf("Failed to get version: %v", err)
	}

	if version.Version != "1" {
		t.Errorf("Expected version '1', got '%s'", version.Version)
	}
}

func TestGetVersion_NonExisting(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.CreateVersion("file_5", "content 1", "version 1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	_, err = manager.GetVersion("file_5", "99")
	if err == nil {
		t.Error("Expected error for non-existing version")
	}
}

func TestRestoreVersion(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.CreateVersion("file_6", "original content", "version 1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	content, err := manager.RestoreVersion("file_6", "1")
	if err != nil {
		t.Fatalf("Failed to restore version: %v", err)
	}

	if content != "original content" {
		t.Errorf("Expected restored content 'original content', got '%s'", content)
	}

	versions, err := manager.GetVersions("file_6")
	if err != nil {
		t.Fatalf("Failed to get versions: %v", err)
	}

	if len(versions) != 2 {
		t.Errorf("Expected 2 versions after restore, got %d", len(versions))
	}
}

func TestRestoreVersion_NonExisting(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.RestoreVersion("file_7", "99")
	if err == nil {
		t.Error("Expected error for restoring non-existing version")
	}
}

func TestDeleteVersion(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.CreateVersion("file_8", "content 1", "version 1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	err = manager.DeleteVersion("file_8", "1")
	if err != nil {
		t.Fatalf("Failed to delete version: %v", err)
	}

	versions, err := manager.GetVersions("file_8")
	if err != nil {
		t.Fatalf("Failed to get versions: %v", err)
	}

	if len(versions) != 0 {
		t.Errorf("Expected 0 versions after deletion, got %d", len(versions))
	}
}

func TestDeleteVersion_NonExisting(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.CreateVersion("file_9", "content 1", "version 1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	err = manager.DeleteVersion("file_9", "99")
	if err != nil {
		t.Errorf("Expected no error for deleting non-existing version, got %v", err)
	}

	versions, err := manager.GetVersions("file_9")
	if err != nil {
		t.Fatalf("Failed to get versions: %v", err)
	}

	if len(versions) != 1 {
		t.Errorf("Expected 1 version to remain, got %d", len(versions))
	}
}

func TestCleanupOldVersions(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.CreateVersion("file_10", "content 1", "version 1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	err = manager.CleanupOldVersions("file_10", 0)
	if err != nil {
		t.Fatalf("Failed to cleanup old versions: %v", err)
	}

	versions, err := manager.GetVersions("file_10")
	if err != nil {
		t.Fatalf("Failed to get versions: %v", err)
	}

	if len(versions) != 0 {
		t.Errorf("Expected 0 versions after cleanup, got %d", len(versions))
	}
}

func TestCleanupOldVersions_KeepRecent(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.CreateVersion("file_11", "content 1", "version 1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	err = manager.CleanupOldVersions("file_11", 30)
	if err != nil {
		t.Fatalf("Failed to cleanup old versions: %v", err)
	}

	versions, err := manager.GetVersions("file_11")
	if err != nil {
		t.Fatalf("Failed to get versions: %v", err)
	}

	if len(versions) != 1 {
		t.Errorf("Expected 1 version after cleanup, got %d", len(versions))
	}
}

func TestSaveVersion(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	version := Version{
		ID:      "file_12_v1",
		FileID:  "file_12",
		Version: "1",
		Content: "test content",
		Size:    12,
		Note:    "test note",
	}

	err := manager.saveVersion(version)
	if err != nil {
		t.Fatalf("Failed to save version: %v", err)
	}

	versions, err := manager.GetVersions("file_12")
	if err != nil {
		t.Fatalf("Failed to get versions: %v", err)
	}

	if len(versions) != 1 {
		t.Errorf("Expected 1 version saved, got %d", len(versions))
	}

	if versions[0].ID != "file_12_v1" {
		t.Errorf("Expected version ID 'file_12_v1', got '%s'", versions[0].ID)
	}
}

func TestLoadVersions(t *testing.T) {
	manager, cleanup := setupVersionTest(t)
	defer cleanup()

	_, err := manager.CreateVersion("file_13", "content 1", "version 1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	versions, err := manager.loadVersions("file_13")
	if err != nil {
		t.Fatalf("Failed to load versions: %v", err)
	}

	if len(versions) != 1 {
		t.Errorf("Expected 1 version loaded, got %d", len(versions))
	}
}

func TestVersion_StructFields(t *testing.T) {
	version := Version{
		ID:      "test_v1",
		FileID:  "test_file",
		Version: "1",
		Content: "test content",
		Size:    12,
		Note:    "test note",
	}

	if version.ID != "test_v1" {
		t.Errorf("Expected ID 'test_v1', got '%s'", version.ID)
	}
	if version.FileID != "test_file" {
		t.Errorf("Expected FileID 'test_file', got '%s'", version.FileID)
	}
	if version.Version != "1" {
		t.Errorf("Expected Version '1', got '%s'", version.Version)
	}
	if version.Content != "test content" {
		t.Errorf("Expected Content 'test content', got '%s'", version.Content)
	}
	if version.Size != 12 {
		t.Errorf("Expected Size 12, got %d", version.Size)
	}
	if version.Note != "test note" {
		t.Errorf("Expected Note 'test note', got '%s'", version.Note)
	}
}
