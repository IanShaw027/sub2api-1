package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSnapshotOpenAICompatRuntimeMetrics(t *testing.T) {
	before := SnapshotOpenAICompatRuntimeMetrics()

	recordOpenAICompatStrippedField("temperature")
	recordOpenAICompatStrippedField("top_p")
	recordOpenAICompatDroppedFields(&apicompat.ResponsesRequest{DroppedCompatibilityFields: []string{"cache_control"}})
	recordOpenAICompatToolContinuationDetected()
	recordOpenAICompatPromptCacheInjected()
	recordOpenAICompatUpstreamStatus("gpt-5.4", http.StatusTooManyRequests)
	recordOpenAICompatUpstreamStatus("gpt-5.4", http.StatusBadGateway)

	after := SnapshotOpenAICompatRuntimeMetrics()
	require.GreaterOrEqual(t, after.StrippedTemperatureTotal, before.StrippedTemperatureTotal+1)
	require.GreaterOrEqual(t, after.StrippedTopPTotal, before.StrippedTopPTotal+1)
	require.GreaterOrEqual(t, after.StrippedCacheControlTotal, before.StrippedCacheControlTotal+1)
	require.GreaterOrEqual(t, after.ToolContinuationDetectedTotal, before.ToolContinuationDetectedTotal+1)
	require.GreaterOrEqual(t, after.PromptCacheInjectedTotal, before.PromptCacheInjectedTotal+1)
	require.GreaterOrEqual(t, after.Upstream4xxByModel["gpt-5.4"], before.Upstream4xxByModel["gpt-5.4"]+1)
	require.GreaterOrEqual(t, after.Upstream5xxByModel["gpt-5.4"], before.Upstream5xxByModel["gpt-5.4"]+1)
}

func TestApplyCodexOAuthTransform_RecordsCompatRuntimeMetrics(t *testing.T) {
	before := SnapshotOpenAICompatRuntimeMetrics()

	reqBody := map[string]any{
		"model":                "gpt-5.1-codex",
		"temperature":          0.2,
		"top_p":                0.8,
		"previous_response_id": "resp_1",
		"input": []any{
			map[string]any{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  "ok",
			},
		},
	}

	result := applyCodexOAuthTransform(reqBody, false, false)
	require.True(t, result.Modified)

	after := SnapshotOpenAICompatRuntimeMetrics()
	require.GreaterOrEqual(t, after.StrippedTemperatureTotal, before.StrippedTemperatureTotal+1)
	require.GreaterOrEqual(t, after.StrippedTopPTotal, before.StrippedTopPTotal+1)
	require.GreaterOrEqual(t, after.ToolContinuationDetectedTotal, before.ToolContinuationDetectedTotal+1)
}

func TestForwardAsAnthropic_RecordsCompatPromptCacheInjectionMetric(t *testing.T) {
	setGinTestMode()

	before := SnapshotOpenAICompatRuntimeMetrics()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_obs_1","object":"response","model":"gpt-5.4","status":"completed","output":[{"type":"message","id":"msg_obs_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_obs_prompt_cache"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
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
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "session-observed", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "session-observed", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())

	after := SnapshotOpenAICompatRuntimeMetrics()
	require.GreaterOrEqual(t, after.PromptCacheInjectedTotal, before.PromptCacheInjectedTotal+1)
}

func TestForwardAsAnthropic_RecordsCompatUpstream4xxMetricByModel(t *testing.T) {
	setGinTestMode()

	before := SnapshotOpenAICompatRuntimeMetrics()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","max_tokens":16,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_obs_429"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
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

	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "gpt-5.4")
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)

	after := SnapshotOpenAICompatRuntimeMetrics()
	require.GreaterOrEqual(t, after.Upstream4xxByModel["gpt-5.4"], before.Upstream4xxByModel["gpt-5.4"]+1)
}
