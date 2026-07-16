package service

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildOpenAIContextCompactionTriggerBodyPreservesHistoryAndDropsAnchor(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-terra","stream":true,"store":true,"previous_response_id":"resp_old","input":[{"type":"message","role":"user","content":"old"},{"type":"message","role":"assistant","content":"answer"}]}`)

	got, err := buildOpenAIContextCompactionTriggerBody(body)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(got, "stream").Bool())
	require.False(t, gjson.GetBytes(got, "store").Exists())
	require.False(t, gjson.GetBytes(got, "previous_response_id").Exists())
	require.Equal(t, "old", gjson.GetBytes(got, "input.0.content").String())
	require.Equal(t, "compaction_trigger", gjson.GetBytes(got, "input.2.type").String())
}

func TestBuildOpenAIContextCompactionTriggerBodyRejectsUnsafeInput(t *testing.T) {
	for name, body := range map[string][]byte{
		"function output": []byte(`{"input":[{"type":"function_call_output","call_id":"c","output":"ok"}]}`),
		"image":           []byte(`{"input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"https://example.test/a.png"}]}]}`),
		"tool call":       []byte(`{"input":[{"type":"function_call","name":"f"}]}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := buildOpenAIContextCompactionTriggerBody(body)
			require.Error(t, err)
		})
	}
}

func TestIsOpenAIContextRemoteCompactionV2RequestRequiresStreamingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"stream":true}`)))
	c.Request.Header.Set("x-codex-beta-features", "responses_websockets_v2, remote_compaction_v2")
	require.True(t, isOpenAIContextRemoteCompactionV2Request(c, []byte(`{"stream":true}`)))
	require.False(t, isOpenAIContextRemoteCompactionV2Request(c, []byte(`{"stream":false}`)))
}

func TestOpenAIContextCompactionPathOverrideKeepsInboundURLAndFailoverBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	compactBody := []byte(`{"model":"gpt-5.6-terra","input":[{"type":"compaction_trigger"}]}`)

	setOpenAIResponsesUpstreamPathSuffixOverride(c, "/compact")
	setOpenAIFailoverRequestBody(c, compactBody)

	require.Equal(t, "/v1/responses", c.Request.URL.Path)
	require.Equal(t, "/compact", openAIResponsesRequestPathSuffix(c))
	got, ok := getOpenAIFailoverRequestBody(c, nil)
	require.True(t, ok)
	require.Equal(t, compactBody, got)
}

func TestClearOpenAICompactFailoverStateIfUnsupported(t *testing.T) {
	t.Run("clears compact override for unsupported account", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		setOpenAIResponsesUpstreamPathSuffixOverride(c, "/compact")
		setOpenAIFailoverRequestBody(c, []byte(`{"input":"compacted"}`))
		MarkOpenAICompactClientStream(c)

		clearOpenAICompactFailoverStateIfUnsupported(c, &Account{
			Platform: PlatformOpenAI,
			Extra:    map[string]any{"openai_compact_supported": false},
		})

		require.Empty(t, openAIResponsesRequestPathSuffix(c))
		_, ok := getOpenAIFailoverRequestBody(c, nil)
		require.False(t, ok)
		require.False(t, openAICompactClientWantsStream(c))
	})

	t.Run("preserves ordinary retry body", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		retryBody := []byte(`{"input":"retry"}`)
		setOpenAIFailoverRequestBody(c, retryBody)

		clearOpenAICompactFailoverStateIfUnsupported(c, &Account{
			Platform: PlatformOpenAI,
			Extra:    map[string]any{"openai_compact_supported": false},
		})

		got, ok := getOpenAIFailoverRequestBody(c, nil)
		require.True(t, ok)
		require.JSONEq(t, string(retryBody), string(got))
	})
}
