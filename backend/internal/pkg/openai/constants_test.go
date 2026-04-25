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
