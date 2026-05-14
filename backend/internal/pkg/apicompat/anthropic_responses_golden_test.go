package apicompat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestResponsesToAnthropic_GoldenFixtures(t *testing.T) {
	t.Parallel()

	fixtures := []string{
		"tool_name_round_trip",
		"refusal_envelope",
	}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			fixtureDir := filepath.Join("testdata", "openai_claude_compat", name)

			var resp ResponsesResponse
			loadGoldenFixtureJSON(t, filepath.Join(fixtureDir, "responses.json"), &resp)

			var toolNameMap map[string]string
			toolsPath := filepath.Join(fixtureDir, "tools.json")
			if _, err := os.Stat(toolsPath); err == nil {
				var tools []ResponsesTool
				loadGoldenFixtureJSON(t, toolsPath, &tools)
				toolNameMap = ClaudeToolNameMapFromTools(tools)
			}

			got := ResponsesToAnthropic(&resp, "claude-opus-4-6", toolNameMap)
			gotJSON, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal anthropic response: %v", err)
			}

			requireGoldenFixtureMatch(t, gotJSON, filepath.Join(fixtureDir, "expected_anthropic.json"))
		})
	}
}
