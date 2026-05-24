package processor

import "testing"

func TestProcessingProgress_ShouldKeepRealTotalChunksWhenOverSampleLimit(t *testing.T) {
	progress := NewProcessingProgress("test-md5", 1083)

	for i := 0; i < 1083; i++ {
		progress.RecordChunkComplete(true, 10, "remote")
	}

	info := progress.GetProgress()
	if info.TotalChunks != 1083 {
		t.Fatalf("TotalChunks = %d, want 1083", info.TotalChunks)
	}
	if info.ProcessedChunks != 1083 {
		t.Fatalf("ProcessedChunks = %d, want 1083", info.ProcessedChunks)
	}
	if info.RemainingChunks != 0 {
		t.Fatalf("RemainingChunks = %d, want 0", info.RemainingChunks)
	}
	if info.Progress != 100 {
		t.Fatalf("Progress = %d, want 100", info.Progress)
	}
	if len(progress.ChunkTimes) != maxTrackedChunkTimes {
		t.Fatalf("len(ChunkTimes) = %d, want %d", len(progress.ChunkTimes), maxTrackedChunkTimes)
	}
}

func TestProcessingProgress_ShouldClampProgressWhenProcessedExceedsTotal(t *testing.T) {
	progress := NewProcessingProgress("test-md5", 2)

	for i := 0; i < 3; i++ {
		progress.RecordChunkComplete(true, 10, "remote")
	}

	info := progress.GetProgress()
	if info.ProcessedChunks != 2 {
		t.Fatalf("ProcessedChunks = %d, want 2", info.ProcessedChunks)
	}
	if info.RemainingChunks != 0 {
		t.Fatalf("RemainingChunks = %d, want 0", info.RemainingChunks)
	}
	if info.Progress != 100 {
		t.Fatalf("Progress = %d, want 100", info.Progress)
	}
}
