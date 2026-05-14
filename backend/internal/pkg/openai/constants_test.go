package openai

import "testing"

func TestDefaultModelsIncludeGPT55(t *testing.T) {
	found := false
	for _, model := range DefaultModels {
		if model.ID == "gpt-5.5" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("DefaultModels should include gpt-5.5")
	}
}

func TestDefaultModelsIncludeCodexAutoReview(t *testing.T) {
	found := false
	for _, model := range DefaultModels {
		if model.ID == "codex-auto-review" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("DefaultModels should include codex-auto-review")
	}
}
