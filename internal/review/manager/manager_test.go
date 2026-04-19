package manager

import (
	"os"
	"testing"
	"txt-cleaning/internal/config"
	"txt-cleaning/internal/processor/preprocess"
)

func setupManagerTest(t *testing.T) (*Manager, func()) {
	tempDir, err := os.MkdirTemp("", "review_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	config.AppConfigInstance.DataDir = tempDir

	manager := NewManager()

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return manager, cleanup
}

func TestNewManager(t *testing.T) {
	manager := NewManager()

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}
}

func TestCreateSession(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	suggestions := []preprocess.Change{
		{Type: "typo", Original: "及了", Replacement: "极了", Position: 0},
		{Type: "typo", Original: "在次", Replacement: "再次", Position: 10},
	}

	session, err := manager.CreateSession("session_1", "file_1", "process_1", suggestions)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session.ID != "session_1" {
		t.Errorf("Expected session ID 'session_1', got '%s'", session.ID)
	}
	if session.FileID != "file_1" {
		t.Errorf("Expected file ID 'file_1', got '%s'", session.FileID)
	}
	if session.ProcessID != "process_1" {
		t.Errorf("Expected process ID 'process_1', got '%s'", session.ProcessID)
	}
	if len(session.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(session.Items))
	}
	if session.Completed {
		t.Error("Expected session not to be completed")
	}
}

func TestCreateSession_EmptySuggestions(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	session, err := manager.CreateSession("session_2", "file_2", "process_2", []preprocess.Change{})
	if err != nil {
		t.Fatalf("Failed to create session with empty suggestions: %v", err)
	}

	if len(session.Items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(session.Items))
	}
}

func TestGetSession_Existing(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	suggestions := []preprocess.Change{
		{Type: "typo", Original: "及了", Replacement: "极了", Position: 0},
	}

	_, err := manager.CreateSession("session_3", "file_3", "process_3", suggestions)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	session, err := manager.GetSession("session_3")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if session.ID != "session_3" {
		t.Errorf("Expected session ID 'session_3', got '%s'", session.ID)
	}
}

func TestGetSession_NonExisting(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	_, err := manager.GetSession("non_existing")
	if err == nil {
		t.Error("Expected error for non-existing session")
	}
}

func TestUpdateItemStatus_Approved(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	suggestions := []preprocess.Change{
		{Type: "typo", Original: "及了", Replacement: "极了", Position: 0},
	}

	_, err := manager.CreateSession("session_4", "file_4", "process_4", suggestions)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	err = manager.UpdateItemStatus("session_4", "1", StatusApproved, "已批准")
	if err != nil {
		t.Fatalf("Failed to update item status: %v", err)
	}

	session, err := manager.GetSession("session_4")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if session.Items[0].Status != StatusApproved {
		t.Errorf("Expected status 'approved', got '%s'", session.Items[0].Status)
	}
	if session.Items[0].ReviewerNote != "已批准" {
		t.Errorf("Expected reviewer note '已批准', got '%s'", session.Items[0].ReviewerNote)
	}
	if session.Completed != true {
		t.Error("Expected session to be completed after all items approved")
	}
}

func TestUpdateItemStatus_Rejected(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	suggestions := []preprocess.Change{
		{Type: "typo", Original: "及了", Replacement: "极了", Position: 0},
	}

	_, err := manager.CreateSession("session_5", "file_5", "process_5", suggestions)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	err = manager.UpdateItemStatus("session_5", "1", StatusRejected, "已拒绝")
	if err != nil {
		t.Fatalf("Failed to update item status: %v", err)
	}

	session, err := manager.GetSession("session_5")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if session.Items[0].Status != StatusRejected {
		t.Errorf("Expected status 'rejected', got '%s'", session.Items[0].Status)
	}
}

func TestUpdateItemStatus_PartialCompletion(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	suggestions := []preprocess.Change{
		{Type: "typo", Original: "及了", Replacement: "极了", Position: 0},
		{Type: "typo", Original: "在次", Replacement: "再次", Position: 10},
	}

	_, err := manager.CreateSession("session_6", "file_6", "process_6", suggestions)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	err = manager.UpdateItemStatus("session_6", "1", StatusApproved, "")
	if err != nil {
		t.Fatalf("Failed to update item status: %v", err)
	}

	session, err := manager.GetSession("session_6")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if session.Completed {
		t.Error("Expected session not to be completed with partial review")
	}
}

func TestUpdateItemStatus_NonExistingSession(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	err := manager.UpdateItemStatus("non_existing", "1", StatusApproved, "")
	if err == nil {
		t.Error("Expected error for non-existing session")
	}
}

func TestSaveProgress(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	suggestions := []preprocess.Change{
		{Type: "typo", Original: "及了", Replacement: "极了", Position: 0},
	}

	_, err := manager.CreateSession("session_7", "file_7", "process_7", suggestions)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	err = manager.SaveProgress("session_7", "保存进度")
	if err != nil {
		t.Fatalf("Failed to save progress: %v", err)
	}

	session, err := manager.GetSession("session_7")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if session.LastModified.IsZero() {
		t.Error("Expected last modified time to be set")
	}
}

func TestGetPendingItems(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	suggestions := []preprocess.Change{
		{Type: "typo", Original: "及了", Replacement: "极了", Position: 0},
		{Type: "typo", Original: "在次", Replacement: "再次", Position: 10},
	}

	_, err := manager.CreateSession("session_8", "file_8", "process_8", suggestions)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	pending, err := manager.GetPendingItems("session_8")
	if err != nil {
		t.Fatalf("Failed to get pending items: %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("Expected 2 pending items, got %d", len(pending))
	}
}

func TestGetPendingItems_AfterApproval(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	suggestions := []preprocess.Change{
		{Type: "typo", Original: "及了", Replacement: "极了", Position: 0},
		{Type: "typo", Original: "在次", Replacement: "再次", Position: 10},
	}

	_, err := manager.CreateSession("session_9", "file_9", "process_9", suggestions)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	err = manager.UpdateItemStatus("session_9", "1", StatusApproved, "")
	if err != nil {
		t.Fatalf("Failed to update item status: %v", err)
	}

	pending, err := manager.GetPendingItems("session_9")
	if err != nil {
		t.Fatalf("Failed to get pending items: %v", err)
	}

	if len(pending) != 1 {
		t.Errorf("Expected 1 pending item, got %d", len(pending))
	}
}

func TestGetApprovedSuggestions(t *testing.T) {
	manager, cleanup := setupManagerTest(t)
	defer cleanup()

	suggestions := []preprocess.Change{
		{Type: "typo", Original: "及了", Replacement: "极了", Position: 0},
		{Type: "typo", Original: "在次", Replacement: "再次", Position: 10},
	}

	_, err := manager.CreateSession("session_10", "file_10", "process_10", suggestions)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	err = manager.UpdateItemStatus("session_10", "1", StatusApproved, "")
	if err != nil {
		t.Fatalf("Failed to update item status: %v", err)
	}

	approved, err := manager.GetApprovedSuggestions("session_10")
	if err != nil {
		t.Fatalf("Failed to get approved suggestions: %v", err)
	}

	if len(approved) != 1 {
		t.Errorf("Expected 1 approved suggestion, got %d", len(approved))
	}

	if approved[0].Original != "及了" {
		t.Errorf("Expected approved suggestion original '及了', got '%s'", approved[0].Original)
	}
}

func TestReviewStatus_Constants(t *testing.T) {
	if StatusPending != "pending" {
		t.Errorf("Expected StatusPending 'pending', got '%s'", StatusPending)
	}
	if StatusApproved != "approved" {
		t.Errorf("Expected StatusApproved 'approved', got '%s'", StatusApproved)
	}
	if StatusRejected != "rejected" {
		t.Errorf("Expected StatusRejected 'rejected', got '%s'", StatusRejected)
	}
	if StatusSkipped != "skipped" {
		t.Errorf("Expected StatusSkipped 'skipped', got '%s'", StatusSkipped)
	}
}
