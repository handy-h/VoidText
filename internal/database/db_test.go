package database

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	if err := Init(tmpDir); err != nil {
		t.Fatalf("Failed to init test database: %v", err)
	}
}

func teardownTestDB(t *testing.T) {
	t.Helper()
	if err := Close(); err != nil {
		t.Fatalf("Failed to close test database: %v", err)
	}
}

func TestInit_ShouldCreateDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer Close()

	dbPath := filepath.Join(tmpDir, "cleaning.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file should exist at %s", dbPath)
	}
}

func TestInit_ShouldCreateDataDir(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "nested", "dir")
	err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer Close()

	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Errorf("Data directory should exist at %s", tmpDir)
	}
}

func TestCreateFile_ShouldInsertRecord(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	record := &FileRecord{
		Md5:      "test_md5_001",
		Author:   "测试作者",
		Title:    "测试标题",
		FileName: "测试作者~测试标题.txt",
		FileSize: 1024,
		FilePath: "/tmp/test.txt",
		Status:   "pending",
	}

	err := CreateFile(record)
	if err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if record.ID == 0 {
		t.Errorf("CreateFile() should set record.ID")
	}
}

func TestGetFileByMd5_ShouldReturnRecord(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	record := &FileRecord{
		Md5:      "test_md5_002",
		Author:   "查询作者",
		Title:    "查询标题",
		FileName: "查询作者~查询标题.txt",
		FileSize: 2048,
		FilePath: "/tmp/query.txt",
		Status:   "pending",
	}
	CreateFile(record)

	found, err := GetFileByMd5("test_md5_002")
	if err != nil {
		t.Fatalf("GetFileByMd5() error = %v", err)
	}
	if found == nil {
		t.Fatalf("GetFileByMd5() should return record")
	}
	if found.Author != "查询作者" {
		t.Errorf("GetFileByMd5() Author = %s, want 查询作者", found.Author)
	}
}

func TestGetFileByMd5_ShouldReturnNilForNonExistent(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	found, err := GetFileByMd5("nonexistent_md5")
	if err != nil {
		t.Fatalf("GetFileByMd5() error = %v", err)
	}
	if found != nil {
		t.Errorf("GetFileByMd5() should return nil for non-existent MD5")
	}
}

func TestUpdateFileStatus_ShouldUpdateStatus(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	record := &FileRecord{
		Md5:    "test_md5_003",
		Status: "pending",
	}
	CreateFile(record)

	err := UpdateFileStatus("test_md5_003", "processing", "cleaning", 20, "")
	if err != nil {
		t.Fatalf("UpdateFileStatus() error = %v", err)
	}

	found, _ := GetFileByMd5("test_md5_003")
	if found.Status != "processing" {
		t.Errorf("UpdateFileStatus() Status = %s, want processing", found.Status)
	}
	if found.CurrentStep != "cleaning" {
		t.Errorf("UpdateFileStatus() CurrentStep = %s, want cleaning", found.CurrentStep)
	}
}

func TestUpdateFileRules_ShouldUpdateRules(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	record := &FileRecord{
		Md5:    "test_md5_004",
		Status: "pending",
	}
	CreateFile(record)

	rulesJSON := `{"enableBasicCleaning":true}`
	err := UpdateFileRules("test_md5_004", rulesJSON)
	if err != nil {
		t.Fatalf("UpdateFileRules() error = %v", err)
	}

	found, _ := GetFileByMd5("test_md5_004")
	if found.RulesConfig != rulesJSON {
		t.Errorf("UpdateFileRules() RulesConfig = %s, want %s", found.RulesConfig, rulesJSON)
	}
}

func TestListAllFiles_ShouldReturnAllRecords(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateFile(&FileRecord{Md5: "list_001", Status: "pending"})
	CreateFile(&FileRecord{Md5: "list_002", Status: "completed"})

	records, err := ListAllFiles()
	if err != nil {
		t.Fatalf("ListAllFiles() error = %v", err)
	}
	if len(records) != 2 {
		t.Errorf("ListAllFiles() length = %d, want 2", len(records))
	}
}

func TestListPendingFiles_ShouldReturnOnlyPending(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateFile(&FileRecord{Md5: "pending_001", Status: "pending"})
	CreateFile(&FileRecord{Md5: "pending_002", Status: "completed"})
	CreateFile(&FileRecord{Md5: "pending_003", Status: "processing"})

	records, err := ListPendingFiles()
	if err != nil {
		t.Fatalf("ListPendingFiles() error = %v", err)
	}
	for _, r := range records {
		if r.Status == "completed" {
			t.Errorf("ListPendingFiles() should not return completed files")
		}
	}
}

func TestDeleteFile_ShouldRemoveRecord(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateFile(&FileRecord{Md5: "delete_001", Status: "pending"})

	err := DeleteFile("delete_001")
	if err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}

	found, _ := GetFileByMd5("delete_001")
	if found != nil {
		t.Errorf("DeleteFile() should remove record")
	}
}

func TestCreateReviewItems_ShouldInsertItems(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	items := []ReviewItemRecord{
		{
			FileMd5:          "review_md5_001",
			OriginalText:     "图书管",
			SuggestedText:    "图书馆",
			ModificationType: "typo_correction",
			Confidence:       0.95,
			Status:           "pending",
		},
		{
			FileMd5:          "review_md5_001",
			OriginalText:     "高兴及了",
			SuggestedText:    "高兴极了",
			ModificationType: "typo_correction",
			Confidence:       0.90,
			Status:           "pending",
		},
	}

	err := CreateReviewItems(items)
	if err != nil {
		t.Fatalf("CreateReviewItems() error = %v", err)
	}
}

func TestCreateReviewItems_ShouldHandleEmptyList(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	err := CreateReviewItems([]ReviewItemRecord{})
	if err != nil {
		t.Fatalf("CreateReviewItems() with empty list should not error: %v", err)
	}
}

func TestGetReviewItemsByFileMd5_ShouldReturnItems(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateReviewItems([]ReviewItemRecord{
		{FileMd5: "review_md5_002", OriginalText: "错字", SuggestedText: "正字", Status: "pending"},
		{FileMd5: "review_md5_002", OriginalText: "广告", SuggestedText: "", Status: "approved"},
	})

	items, err := GetReviewItemsByFileMd5("review_md5_002", "")
	if err != nil {
		t.Fatalf("GetReviewItemsByFileMd5() error = %v", err)
	}
	if len(items) != 2 {
		t.Errorf("GetReviewItemsByFileMd5() length = %d, want 2", len(items))
	}
}

func TestGetReviewItemsByFileMd5_ShouldFilterByStatus(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateReviewItems([]ReviewItemRecord{
		{FileMd5: "review_md5_003", OriginalText: "待审核", Status: "pending"},
		{FileMd5: "review_md5_003", OriginalText: "已通过", Status: "approved"},
	})

	items, err := GetReviewItemsByFileMd5("review_md5_003", "pending")
	if err != nil {
		t.Fatalf("GetReviewItemsByFileMd5() error = %v", err)
	}
	if len(items) != 1 {
		t.Errorf("GetReviewItemsByFileMd5() with filter length = %d, want 1", len(items))
	}
}

func TestUpdateReviewItemStatus_ShouldUpdateStatus(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateReviewItems([]ReviewItemRecord{
		{FileMd5: "review_md5_004", OriginalText: "测试", Status: "pending"},
	})

	items, _ := GetReviewItemsByFileMd5("review_md5_004", "")
	itemID := items[0].ID

	err := UpdateReviewItemStatus(itemID, "approved", "")
	if err != nil {
		t.Fatalf("UpdateReviewItemStatus() error = %v", err)
	}

	item, _ := GetReviewItemByID(itemID)
	if item.Status != "approved" {
		t.Errorf("UpdateReviewItemStatus() Status = %s, want approved", item.Status)
	}
}

func TestUpdateReviewItemStatus_ShouldClearResolvedAtOnRestore(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateReviewItems([]ReviewItemRecord{
		{FileMd5: "review_md5_005", OriginalText: "恢复测试", Status: "pending"},
	})

	items, _ := GetReviewItemsByFileMd5("review_md5_005", "")
	itemID := items[0].ID

	UpdateReviewItemStatus(itemID, "approved", "")
	UpdateReviewItemStatus(itemID, "pending", "")

	item, _ := GetReviewItemByID(itemID)
	if item.ResolvedAt != nil {
		t.Errorf("UpdateReviewItemStatus() restore should clear resolvedAt")
	}
}

func TestBatchUpdateReviewItemStatus_ShouldUpdateMultiple(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateReviewItems([]ReviewItemRecord{
		{FileMd5: "review_md5_006", OriginalText: "批量1", Status: "pending"},
		{FileMd5: "review_md5_006", OriginalText: "批量2", Status: "pending"},
	})

	items, _ := GetReviewItemsByFileMd5("review_md5_006", "")
	ids := []int64{items[0].ID, items[1].ID}

	err := BatchUpdateReviewItemStatus(ids, "approved")
	if err != nil {
		t.Fatalf("BatchUpdateReviewItemStatus() error = %v", err)
	}

	updatedItems, _ := GetReviewItemsByFileMd5("review_md5_006", "approved")
	if len(updatedItems) != 2 {
		t.Errorf("BatchUpdateReviewItemStatus() should update both items")
	}
}

func TestGetReviewProgress_ShouldReturnCorrectCounts(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateReviewItems([]ReviewItemRecord{
		{FileMd5: "review_md5_007", OriginalText: "待审核1", Status: "pending"},
		{FileMd5: "review_md5_007", OriginalText: "已通过1", Status: "approved"},
		{FileMd5: "review_md5_007", OriginalText: "已拒绝1", Status: "rejected"},
	})

	total, resolved, err := GetReviewProgress("review_md5_007")
	if err != nil {
		t.Fatalf("GetReviewProgress() error = %v", err)
	}
	if total != 3 {
		t.Errorf("GetReviewProgress() total = %d, want 3", total)
	}
	if resolved != 2 {
		t.Errorf("GetReviewProgress() resolved = %d, want 2", resolved)
	}
}

func TestDeleteReviewItemsByFileMd5_ShouldRemoveAllItems(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateReviewItems([]ReviewItemRecord{
		{FileMd5: "review_md5_008", OriginalText: "删除1", Status: "pending"},
		{FileMd5: "review_md5_008", OriginalText: "删除2", Status: "pending"},
	})

	err := DeleteReviewItemsByFileMd5("review_md5_008")
	if err != nil {
		t.Fatalf("DeleteReviewItemsByFileMd5() error = %v", err)
	}

	items, _ := GetReviewItemsByFileMd5("review_md5_008", "")
	if len(items) != 0 {
		t.Errorf("DeleteReviewItemsByFileMd5() should remove all items")
	}
}

func TestCreateVersion_ShouldInsertRecord(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	record := &VersionRecord{
		OriginalMd5: "orig_md5_001",
		VersionMd5:  "ver_md5_001",
		ParentMd5:   "orig_md5_001",
		VersionType: "original",
		FilePath:    "/tmp/original.txt",
		Step:        "cleaning",
	}

	err := CreateVersion(record)
	if err != nil {
		t.Fatalf("CreateVersion() error = %v", err)
	}
	if record.ID == 0 {
		t.Errorf("CreateVersion() should set record.ID")
	}
}

func TestGetVersionByMd5_ShouldReturnRecord(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateVersion(&VersionRecord{
		OriginalMd5: "orig_md5_002",
		VersionMd5:  "ver_md5_002",
		VersionType: "intermediate",
		FilePath:    "/tmp/intermediate.txt",
	})

	found, err := GetVersionByMd5("ver_md5_002")
	if err != nil {
		t.Fatalf("GetVersionByMd5() error = %v", err)
	}
	if found == nil {
		t.Fatalf("GetVersionByMd5() should return record")
	}
	if found.VersionType != "intermediate" {
		t.Errorf("GetVersionByMd5() VersionType = %s, want intermediate", found.VersionType)
	}
}

func TestGetVersionsByOriginalMd5_ShouldReturnAllVersions(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateVersion(&VersionRecord{OriginalMd5: "orig_md5_003", VersionMd5: "v1", VersionType: "original", FilePath: "/tmp/v1.txt"})
	CreateVersion(&VersionRecord{OriginalMd5: "orig_md5_003", VersionMd5: "v2", VersionType: "intermediate", FilePath: "/tmp/v2.txt"})

	versions, err := GetVersionsByOriginalMd5("orig_md5_003")
	if err != nil {
		t.Fatalf("GetVersionsByOriginalMd5() error = %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("GetVersionsByOriginalMd5() length = %d, want 2", len(versions))
	}
}

func TestGetLatestVersion_ShouldReturnLatest(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateVersion(&VersionRecord{OriginalMd5: "orig_md5_004", VersionMd5: "v1_old", ParentMd5: "orig_md5_004", VersionType: "original", FilePath: "/tmp/v1.txt"})

	_, err := db.Exec(`INSERT INTO versions (original_md5, version_md5, parent_md5, version_type, file_path, step, created_at) VALUES (?, ?, ?, ?, ?, ?, datetime('now', '+1 second'))`,
		"orig_md5_004", "v2_new", "v1_old", "final", "/tmp/v2.txt", "finalizing")
	if err != nil {
		t.Fatalf("insert second version error = %v", err)
	}

	latest, err := GetLatestVersion("orig_md5_004")
	if err != nil {
		t.Fatalf("GetLatestVersion() error = %v", err)
	}
	if latest.VersionMd5 != "v2_new" {
		t.Errorf("GetLatestVersion() VersionMd5 = %s, want v2_new", latest.VersionMd5)
	}
}

func TestCreateProcessingLog_ShouldInsertLog(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	record := &ProcessingLogRecord{
		FileMd5: "log_md5_001",
		Step:    "cleaning",
		Action:  "start",
		Status:  "running",
	}

	err := CreateProcessingLog(record)
	if err != nil {
		t.Fatalf("CreateProcessingLog() error = %v", err)
	}
	if record.ID == 0 {
		t.Errorf("CreateProcessingLog() should set record.ID")
	}
}

func TestGetProcessingLogsByFileMd5_ShouldReturnLogs(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	CreateProcessingLog(&ProcessingLogRecord{FileMd5: "log_md5_002", Step: "cleaning", Action: "start", Status: "running"})
	CreateProcessingLog(&ProcessingLogRecord{FileMd5: "log_md5_002", Step: "cleaning", Action: "complete", Status: "success"})

	logs, err := GetProcessingLogsByFileMd5("log_md5_002")
	if err != nil {
		t.Fatalf("GetProcessingLogsByFileMd5() error = %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("GetProcessingLogsByFileMd5() length = %d, want 2", len(logs))
	}
}
