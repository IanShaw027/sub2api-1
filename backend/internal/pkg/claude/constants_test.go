package claude

import "testing"

func TestDefaultModels_IncludeClaudeFable5(t *testing.T) {
	var found bool
	for _, m := range DefaultModels {
		if m.ID == "claude-fable-5" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("DefaultModels does not include claude-fable-5")
	}
}
