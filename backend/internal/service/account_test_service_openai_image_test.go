package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAIImageTestMode(t *testing.T) {
	require.Equal(t, "codex", NormalizeOpenAIImageTestMode("codex"))
	require.Equal(t, "codex", NormalizeOpenAIImageTestMode("image_api"))
	require.Equal(t, "web2api", NormalizeOpenAIImageTestMode("web2api"))
	require.Equal(t, "", NormalizeOpenAIImageTestMode("responses-tool"))
}

func TestResolveOpenAIImageExecutionMode_DefaultsToCodex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	require.Equal(t, "codex", resolveOpenAIImageExecutionMode(c))

	c.Set(AccountTestContextRequestedModeKey, "web2api")
	require.Equal(t, "web2api", resolveOpenAIImageExecutionMode(c))
}

func TestAccountTestService_OpenAIImageOAuthDefaultCallsImagesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"ig_123\",\"type\":\"image_generation_call\",\"result\":\"aGVsbG8=\",\"revised_prompt\":\"draw a cat\",\"output_format\":\"png\"}}\n\n" +
					"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000006,\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:       53,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	err := svc.testOpenAIAccountConnection(c, account, "gpt-image-2", "draw a cat")
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, chatgptCodexURL, upstream.lastReq.URL.String())
	require.Equal(t, "Bearer token-123", upstream.lastReq.Header.Get("Authorization"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(upstream.lastBody, &body))
	input := body["input"].([]any)
	message := input[0].(map[string]any)
	content := message["content"].([]any)
	require.Equal(t, "draw a cat", content[0].(map[string]any)["text"])
	require.Equal(t, false, body["store"])
	require.Equal(t, "gpt-image-2", body["tools"].([]any)[0].(map[string]any)["model"])
	require.Contains(t, rec.Body.String(), "data:image/png;base64,aGVsbG8=")
	require.Contains(t, rec.Body.String(), "\"success\":true")
}
