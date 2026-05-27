package database

import (
	"testing"
)

func TestCreateReviewParagraphs_ShouldInsertAndQuery(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	records := []ReviewParagraphRecord{
		{FileMd5: "abc", ParagraphIndex: 0, OriginalText: "原段A", SuggestedText: "修复A", ModificationType: "llm_repair"},
		{FileMd5: "abc", ParagraphIndex: 5, OriginalText: "原段B", SuggestedText: "修复B", ModificationType: "llm_repair"},
		{FileMd5: "abc", ParagraphIndex: 2, OriginalText: "重复段", SuggestedText: "", ModificationType: "duplicate_paragraph"},
	}
	if err := CreateReviewParagraphs(records); err != nil {
		t.Fatalf("CreateReviewParagraphs() error = %v", err)
	}

	got, err := GetReviewParagraphsByFileMd5("abc", "")
	if err != nil {
		t.Fatalf("GetReviewParagraphsByFileMd5() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 records, got %d", len(got))
	}
	// 应按 paragraph_index 升序
	if got[0].ParagraphIndex != 0 || got[1].ParagraphIndex != 2 || got[2].ParagraphIndex != 5 {
		t.Errorf("records not sorted by paragraph_index: %+v", got)
	}
	if got[0].Status != "pending" {
		t.Errorf("default status should be 'pending', got %q", got[0].Status)
	}
}

func TestCreateReviewParagraphs_ShouldRejectDuplicateIndex(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	records := []ReviewParagraphRecord{
		{FileMd5: "abc", ParagraphIndex: 0, OriginalText: "x", SuggestedText: "y", ModificationType: "llm_repair"},
		{FileMd5: "abc", ParagraphIndex: 0, OriginalText: "x", SuggestedText: "z", ModificationType: "llm_repair"},
	}
	if err := CreateReviewParagraphs(records); err == nil {
		t.Errorf("expected UNIQUE constraint violation")
	}
}

func TestUpdateReviewParagraphStatus_ShouldSetResolvedAt(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	records := []ReviewParagraphRecord{
		{FileMd5: "f", ParagraphIndex: 0, OriginalText: "a", SuggestedText: "b", ModificationType: "llm_repair"},
	}
	if err := CreateReviewParagraphs(records); err != nil {
		t.Fatal(err)
	}
	got, _ := GetReviewParagraphsByFileMd5("f", "")
	id := got[0].ID

	if err := UpdateReviewParagraphStatus(id, "approved", ""); err != nil {
		t.Fatalf("UpdateReviewParagraphStatus() error = %v", err)
	}
	r, _ := GetReviewParagraphByID(id)
	if r.Status != "approved" {
		t.Errorf("status = %q, want approved", r.Status)
	}
	if r.ResolvedAt == nil {
		t.Errorf("resolvedAt should be set after approval")
	}

	// 切回 pending 应清空 resolvedAt
	if err := UpdateReviewParagraphStatus(id, "pending", ""); err != nil {
		t.Fatal(err)
	}
	r, _ = GetReviewParagraphByID(id)
	if r.ResolvedAt != nil {
		t.Errorf("resolvedAt should be nil after revert to pending, got %v", *r.ResolvedAt)
	}
}

func TestBatchUpdateReviewParagraphStatus_ShouldOnlyAffectPending(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	records := []ReviewParagraphRecord{
		{FileMd5: "f", ParagraphIndex: 0, OriginalText: "a", SuggestedText: "b", ModificationType: "llm_repair"},
		{FileMd5: "f", ParagraphIndex: 1, OriginalText: "c", SuggestedText: "d", ModificationType: "llm_repair"},
		{FileMd5: "f", ParagraphIndex: 2, OriginalText: "e", SuggestedText: "f", ModificationType: "llm_repair"},
	}
	if err := CreateReviewParagraphs(records); err != nil {
		t.Fatal(err)
	}
	got, _ := GetReviewParagraphsByFileMd5("f", "")
	if err := UpdateReviewParagraphStatus(got[0].ID, "rejected", ""); err != nil {
		t.Fatal(err)
	}

	if err := BatchUpdateReviewParagraphStatus("f", "approved"); err != nil {
		t.Fatalf("BatchUpdateReviewParagraphStatus() error = %v", err)
	}

	all, _ := GetReviewParagraphsByFileMd5("f", "")
	if all[0].Status != "rejected" {
		t.Errorf("idx=0 status should remain rejected, got %q", all[0].Status)
	}
	if all[1].Status != "approved" || all[2].Status != "approved" {
		t.Errorf("pending records should be approved, got %q %q", all[1].Status, all[2].Status)
	}
}

func TestGetReviewParagraphProgress(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	records := []ReviewParagraphRecord{
		{FileMd5: "f", ParagraphIndex: 0, OriginalText: "a", SuggestedText: "b", ModificationType: "llm_repair"},
		{FileMd5: "f", ParagraphIndex: 1, OriginalText: "c", SuggestedText: "d", ModificationType: "llm_repair"},
		{FileMd5: "f", ParagraphIndex: 2, OriginalText: "e", SuggestedText: "f", ModificationType: "llm_repair"},
	}
	if err := CreateReviewParagraphs(records); err != nil {
		t.Fatal(err)
	}
	got, _ := GetReviewParagraphsByFileMd5("f", "")
	UpdateReviewParagraphStatus(got[0].ID, "approved", "")
	UpdateReviewParagraphStatus(got[1].ID, "rejected", "")

	total, resolved, err := GetReviewParagraphProgress("f")
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || resolved != 2 {
		t.Errorf("progress = (%d, %d), want (3, 2)", total, resolved)
	}
}

func TestDeleteReviewParagraphsByFileMd5(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	records := []ReviewParagraphRecord{
		{FileMd5: "f", ParagraphIndex: 0, OriginalText: "a", SuggestedText: "b", ModificationType: "llm_repair"},
	}
	if err := CreateReviewParagraphs(records); err != nil {
		t.Fatal(err)
	}
	if err := DeleteReviewParagraphsByFileMd5("f"); err != nil {
		t.Fatal(err)
	}
	got, _ := GetReviewParagraphsByFileMd5("f", "")
	if len(got) != 0 {
		t.Errorf("expected 0 records after delete, got %d", len(got))
	}
}
