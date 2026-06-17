package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type goldenFixtureExpectation struct {
	EqualsPaths       map[string]any `json:"equals_paths"`
	AbsentPaths       []string       `json:"absent_paths"`
	OrderedSubstrings []string       `json:"ordered_substrings"`
}

func runForwardAsChatCompletionsGoldenFixture(
	t *testing.T,
	fixtureDir string,
	promptCacheKey string,
	requestModel string,
	accountType string,
) {
	t.Helper()

	setGinTestMode()

	baseDir := filepath.Join("testdata", "openai_claude_compat", fixtureDir)
	body := loadGoldenFixtureBytes(t, filepath.Join(baseDir, "chat_request.json"))
	upstreamSSE := string(loadGoldenFixtureBytes(t, filepath.Join(baseDir, "upstream_sse.txt")))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_fixture"}},
		Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled: false,
				},
			},
		},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "openai-compat",
		Platform:    PlatformOpenAI,
		Type:        accountType,
		Concurrency: 1,
	}
	switch accountType {
	case AccountTypeOAuth:
		account.Credentials = map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		}
	case AccountTypeAPIKey:
		account.Credentials = map[string]any{
			"api_key": "sk-test",
		}
	}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body, promptCacheKey, requestModel, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	requireGoldenFixtureMatch(t, upstream.lastBody, filepath.Join(baseDir, "expected_upstream_request.json"))
	requireGoldenFixtureMatch(t, rec.Body.Bytes(), filepath.Join(baseDir, "expected_downstream_response.json"))
}

func runForwardAsAnthropicGoldenFixture(t *testing.T, fixtureDir, promptCacheKey, requestModel string) {
	t.Helper()

	setGinTestMode()

	baseDir := filepath.Join("testdata", "openai_claude_compat", fixtureDir)
	body := loadGoldenFixtureBytes(t, filepath.Join(baseDir, "anthropic_request.json"))
	upstreamSSE := string(loadGoldenFixtureBytes(t, filepath.Join(baseDir, "upstream_sse.txt")))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_fixture"}},
		Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, promptCacheKey, requestModel)
	require.NoError(t, err)
	require.NotNil(t, result)

	requireGoldenFixtureMatch(t, upstream.lastBody, filepath.Join(baseDir, "expected_upstream_request.json"))
	requireGoldenFixtureMatch(t, rec.Body.Bytes(), filepath.Join(baseDir, "expected_downstream_response.json"))
}

func loadGoldenFixtureBytes(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func requireGoldenFixtureMatch(t *testing.T, actual []byte, path string) {
	t.Helper()

	var expected goldenFixtureExpectation
	require.NoError(t, json.Unmarshal(loadGoldenFixtureBytes(t, path), &expected))

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
