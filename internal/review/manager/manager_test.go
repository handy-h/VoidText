package manager

import (
	"os"
	"testing"

	"voidtext/internal/config"
	"voidtext/internal/processor/preprocess"
)

func initTestConfig(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	config.AppConfigInstance = config.AppConfig{
		DataDir: tmpDir,
	}
}

func TestNewManager_ShouldCreateManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatalf("NewManager() should return non-nil")
	}
}

func TestCreateSession_ShouldCreateSession(t *testing.T) {
	initTestConfig(t)
	m := NewManager()

	suggestions := []preprocess.Change{
		{Original: "及了", Replacement: "极了", Type: "typo_correction"},
	}

	session, err := m.CreateSession("session_001", "file_001", "process_001", suggestions)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session.ID != "session_001" {
		t.Errorf("CreateSession() ID = %s, want session_001", session.ID)
	}
	if len(session.Items) != 1 {
		t.Errorf("CreateSession() items length = %d, want 1", len(session.Items))
	}
	if session.Completed {
		t.Errorf("CreateSession() Completed should be false")
	}
}

func TestGetSession_ShouldReturnSession(t *testing.T) {
	initTestConfig(t)
	m := NewManager()

	suggestions := []preprocess.Change{
		{Original: "测试", Replacement: "替换", Type: "test"},
	}
	m.CreateSession("session_002", "file_002", "process_002", suggestions)

	session, err := m.GetSession("session_002")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.ID != "session_002" {
		t.Errorf("GetSession() ID = %s, want session_002", session.ID)
	}
}

func TestGetSession_ShouldLoadFromDisk(t *testing.T) {
	initTestConfig(t)
	m1 := NewManager()

	suggestions := []preprocess.Change{
		{Original: "磁盘测试", Replacement: "替换", Type: "test"},
	}
	m1.CreateSession("session_003", "file_003", "process_003", suggestions)

	m2 := NewManager()
	session, err := m2.GetSession("session_003")
	if err != nil {
		t.Fatalf("GetSession() from disk error = %v", err)
	}
	if session.ID != "session_003" {
		t.Errorf("GetSession() from disk ID = %s, want session_003", session.ID)
	}
}

func TestUpdateItemStatus_ShouldApproveItem(t *testing.T) {
	initTestConfig(t)
	m := NewManager()

	suggestions := []preprocess.Change{
		{Original: "审核测试", Replacement: "替换", Type: "test"},
	}
	m.CreateSession("session_004", "file_004", "process_004", suggestions)

	err := m.UpdateItemStatus("session_004", "1", StatusApproved, "")
	if err != nil {
		t.Fatalf("UpdateItemStatus() error = %v", err)
	}

	session, _ := m.GetSession("session_004")
	if session.Items[0].Status != StatusApproved {
		t.Errorf("UpdateItemStatus() Status = %s, want %s", session.Items[0].Status, StatusApproved)
	}
}

func TestUpdateItemStatus_ShouldRejectItem(t *testing.T) {
	initTestConfig(t)
	m := NewManager()

	suggestions := []preprocess.Change{
		{Original: "拒绝测试", Replacement: "替换", Type: "test"},
	}
	m.CreateSession("session_005", "file_005", "process_005", suggestions)

	m.UpdateItemStatus("session_005", "1", StatusRejected, "不需要修改")

	session, _ := m.GetSession("session_005")
	if session.Items[0].Status != StatusRejected {
		t.Errorf("UpdateItemStatus() Status = %s, want %s", session.Items[0].Status, StatusRejected)
	}
	if session.Items[0].ReviewerNote != "不需要修改" {
		t.Errorf("UpdateItemStatus() ReviewerNote = %s, want 不需要修改", session.Items[0].ReviewerNote)
	}
}

func TestUpdateItemStatus_ShouldMarkCompletedWhenAllReviewed(t *testing.T) {
	initTestConfig(t)
	m := NewManager()

	suggestions := []preprocess.Change{
		{Original: "项目1", Replacement: "替换1", Type: "test"},
		{Original: "项目2", Replacement: "替换2", Type: "test"},
	}
	m.CreateSession("session_006", "file_006", "process_006", suggestions)

	m.UpdateItemStatus("session_006", "1", StatusApproved, "")
	m.UpdateItemStatus("session_006", "2", StatusRejected, "")

	session, _ := m.GetSession("session_006")
	if !session.Completed {
		t.Errorf("UpdateItemStatus() should mark session as completed when all items reviewed")
	}
}

func TestGetPendingItems_ShouldReturnOnlyPending(t *testing.T) {
	initTestConfig(t)
	m := NewManager()

	suggestions := []preprocess.Change{
		{Original: "待审核1", Replacement: "替换1", Type: "test"},
		{Original: "待审核2", Replacement: "替换2", Type: "test"},
	}
	m.CreateSession("session_007", "file_007", "process_007", suggestions)
	m.UpdateItemStatus("session_007", "1", StatusApproved, "")

	pending, err := m.GetPendingItems("session_007")
	if err != nil {
		t.Fatalf("GetPendingItems() error = %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("GetPendingItems() length = %d, want 1", len(pending))
	}
}

func TestGetApprovedSuggestions_ShouldReturnOnlyApproved(t *testing.T) {
	initTestConfig(t)
	m := NewManager()

	suggestions := []preprocess.Change{
		{Original: "通过1", Replacement: "替换1", Type: "test"},
		{Original: "拒绝1", Replacement: "替换2", Type: "test"},
	}
	m.CreateSession("session_008", "file_008", "process_008", suggestions)
	m.UpdateItemStatus("session_008", "1", StatusApproved, "")
	m.UpdateItemStatus("session_008", "2", StatusRejected, "")

	approved, err := m.GetApprovedSuggestions("session_008")
	if err != nil {
		t.Fatalf("GetApprovedSuggestions() error = %v", err)
	}
	if len(approved) != 1 {
		t.Errorf("GetApprovedSuggestions() length = %d, want 1", len(approved))
	}
}

func TestSaveProgress_ShouldPersistSession(t *testing.T) {
	initTestConfig(t)
	m := NewManager()

	suggestions := []preprocess.Change{
		{Original: "保存测试", Replacement: "替换", Type: "test"},
	}
	m.CreateSession("session_009", "file_009", "process_009", suggestions)

	err := m.SaveProgress("session_009", "测试备注")
	if err != nil {
		t.Fatalf("SaveProgress() error = %v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
