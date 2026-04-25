package apicompat

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type goldenFixtureExpectation struct {
	EqualsPaths       map[string]any `json:"equals_paths"`
	AbsentPaths       []string       `json:"absent_paths"`
	OrderedSubstrings []string       `json:"ordered_substrings"`
}

func loadGoldenFixtureJSON(t *testing.T, path string, target any) {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, target))
}

func requireGoldenFixtureMatch(t *testing.T, actual []byte, path string) {
	t.Helper()

	var expected goldenFixtureExpectation
	loadGoldenFixtureJSON(t, path, &expected)

	for jsonPath, want := range expected.EqualsPaths {
		result := gjson.GetBytes(actual, jsonPath)
		require.Truef(t, result.Exists(), "expected JSON path %q to exist", jsonPath)
		requireGoldenPathValue(t, jsonPath, result, want)
	}

	for _, jsonPath := range expected.AbsentPaths {
		require.Falsef(t, gjson.GetBytes(actual, jsonPath).Exists(), "expected JSON path %q to be absent", jsonPath)
	}

	raw := string(actual)
	prev := -1
	for _, substr := range expected.OrderedSubstrings {
		pos := strings.Index(raw, substr)
		require.NotEqualf(t, -1, pos, "expected substring %q in JSON", substr)
		require.Greaterf(t, pos, prev, "expected substring %q to appear after previous substring", substr)
		prev = pos
	}
}

func requireGoldenPathValue(t *testing.T, jsonPath string, got gjson.Result, want any) {
	t.Helper()

	switch v := want.(type) {
	case string:
		require.Equalf(t, v, got.String(), "unexpected value at %q", jsonPath)
	case bool:
		require.Equalf(t, v, got.Bool(), "unexpected value at %q", jsonPath)
	case float64:
		require.Equalf(t, v, got.Float(), "unexpected value at %q", jsonPath)
	default:
		wantJSON, err := json.Marshal(v)
		require.NoError(t, err)
		require.JSONEqf(t, string(wantJSON), got.Raw, "unexpected JSON value at %q", jsonPath)
	}
}
