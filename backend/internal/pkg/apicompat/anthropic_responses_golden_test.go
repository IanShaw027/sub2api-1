package apicompat

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestResponsesToAnthropic_GoldenFixtures(t *testing.T) {
	t.Parallel()

	fixtureDir := filepath.Join("testdata", "openai_claude_compat", "tool_name_round_trip")

	var resp ResponsesResponse
	loadGoldenFixtureJSON(t, filepath.Join(fixtureDir, "responses.json"), &resp)

	var tools []ResponsesTool
	loadGoldenFixtureJSON(t, filepath.Join(fixtureDir, "tools.json"), &tools)

	got := ResponsesToAnthropic(&resp, "claude-opus-4-6", ClaudeToolNameMapFromTools(tools))
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal anthropic response: %v", err)
	}

	requireGoldenFixtureMatch(t, gotJSON, filepath.Join(fixtureDir, "expected_anthropic.json"))
}
