package processor

import (
	"testing"

	"voidtext/internal/config"
)

func initRepairerTestConfig() {
	config.AppConfigInstance = config.AppConfig{
		EnableModelRepair:   true,
		RepairModelType:     "local",
		RepairModelName:     "test-model",
		LLMApiURL:           "",
		LLMApiKey:           "",
		CompletionModelName: "test-model",
	}
}

func TestNewModelRepairer_ShouldCreateRepairer(t *testing.T) {
	mr := NewModelRepairer("local", "test-model")
	if mr == nil {
		t.Fatalf("NewModelRepairer() should return non-nil")
	}
}

func TestModelRepairerSplitIntoParagraphs_ShouldSplitByNewline(t *testing.T) {
	initRepairerTestConfig()
	mr := NewModelRepairer("local", "test-model")

	paragraphs := mr.SplitIntoParagraphs("第一段\n第二段\n第三段")
	if len(paragraphs) != 3 {
		t.Errorf("SplitIntoParagraphs() length = %d, want 3", len(paragraphs))
	}
}

func TestModelRepairerSplitIntoParagraphs_ShouldFilterEmpty(t *testing.T) {
	initRepairerTestConfig()
	mr := NewModelRepairer("local", "test-model")

	paragraphs := mr.SplitIntoParagraphs("第一段\n\n\n第二段")
	if len(paragraphs) != 2 {
		t.Errorf("SplitIntoParagraphs() length = %d, want 2", len(paragraphs))
	}
}

func TestModelRepairerSplitIntoParagraphs_ShouldHandleEmptyInput(t *testing.T) {
	initRepairerTestConfig()
	mr := NewModelRepairer("local", "test-model")

	paragraphs := mr.SplitIntoParagraphs("")
	if len(paragraphs) != 0 {
		t.Errorf("SplitIntoParagraphs() length = %d, want 0", len(paragraphs))
	}
}

func TestRepairParagraph_ShouldSkipShortParagraph(t *testing.T) {
	initRepairerTestConfig()
	mr := NewModelRepairer("local", "test-model")

	result := mr.RepairParagraph("短")
	if result != "短" {
		t.Errorf("RepairParagraph() should skip short paragraphs")
	}
}

func TestRepairLocally_ShouldFixTypos(t *testing.T) {
	initRepairerTestConfig()
	mr := NewModelRepairer("local", "test-model")

	_, changes := mr.repairLocally("她高兴及了，跑过去抱住他。因该是这样的。")
	if len(changes) == 0 {
		t.Errorf("repairLocally() should detect typos")
	}

	foundJiLe := false
	foundYinGai := false
	for _, change := range changes {
		if change.Original == "及了" && change.Replacement == "极了" {
			foundJiLe = true
		}
		if change.Original == "因该" && change.Replacement == "应该" {
			foundYinGai = true
		}
	}
	if !foundJiLe {
		t.Errorf("repairLocally() should detect 及了 -> 极了")
	}
	if !foundYinGai {
		t.Errorf("repairLocally() should detect 因该 -> 应该")
	}
}

func TestRepairLocally_ShouldFixTuShuGuan(t *testing.T) {
	initRepairerTestConfig()
	mr := NewModelRepairer("local", "test-model")

	_, changes := mr.repairLocally("他每天都去图书管看书。")
	if len(changes) == 0 {
		t.Errorf("repairLocally() should detect 图书管 typo")
	}

	found := false
	for _, change := range changes {
		if change.Original == "图书管" && change.Replacement == "图书馆" {
			found = true
		}
	}
	if !found {
		t.Errorf("repairLocally() should detect 图书管 -> 图书馆")
	}
}

func TestRepairText_ShouldProcessAllParagraphs(t *testing.T) {
	initRepairerTestConfig()
	mr := NewModelRepairer("local", "test-model")

	content := "她高兴及了，跑过去抱住他。\n他每天都去图书管看书。"
	result := mr.RepairText(content)

	if result.Stats["paragraphs_repaired"] != 2 {
		t.Errorf("RepairText() paragraphs_repaired = %d, want 2", result.Stats["paragraphs_repaired"])
	}
}

func TestRepairParagraph_ShouldReturnChangesWithRelativePositions(t *testing.T) {
	initRepairerTestConfig()
	mr := NewModelRepairer("local", "test-model")

	_, changes := mr.repairLocally("她高兴及了，因该这样。")
	if len(changes) == 0 {
		t.Fatalf("repairLocally() should produce changes")
	}
	for _, change := range changes {
		if change.Position < 0 {
			t.Errorf("change.Position should be non-negative, got %d", change.Position)
		}
	}
}

func TestRepairText_ShouldReturnOriginalWhenDisabled(t *testing.T) {
	config.AppConfigInstance.EnableModelRepair = false
	mr := NewModelRepairer("local", "test-model")

	content := "她高兴及了"
	result := mr.RepairText(content)

	if result.Content != content {
		t.Errorf("RepairText() should return original when disabled")
	}
}
