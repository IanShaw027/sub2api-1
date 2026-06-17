package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEmitOpenAICacheProbeEvent_LogsCompactNormalizationSignals(t *testing.T) {
	setGinTestMode()
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	c.Set("api_key", &APIKey{ID: 42})
	c.Set(openAICompactSessionSeedKey, "compact-seed-1")

	originalBody := []byte(`{"model":"gpt-5.5","stream":true,"store":true,"prompt_cache_key":"pc-123","previous_response_id":"resp_prev","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]}`)
	finalBody := []byte(`{"model":"gpt-5.5","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_text","text":"world"}]}]}`)
	result := &OpenAIForwardResult{
		RequestID:     "resp_123",
		Model:         "gpt-5.5",
		UpstreamModel: "gpt-5.5",
		Usage: OpenAIUsage{
			InputTokens:          124000,
			CacheReadInputTokens: 121000,
			OutputTokens:         200,
		},
	}

	emitOpenAICacheProbeEvent(context.Background(), c, &Account{
		ID:       7805,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}, originalBody, finalBody, result, "pc-123", true)

	require.True(t, logSink.ContainsMessageAtLevel("OpenAI cache probe", "info"))
	require.True(t, logSink.ContainsFieldValue("component", "audit.openai_cache_probe"))
	require.True(t, logSink.ContainsFieldValue("request_id", "resp_123"))
	require.True(t, logSink.ContainsFieldValue("api_key_id", "42"))
	require.True(t, logSink.ContainsFieldValue("request_prompt_cache_key_sha256", hashSensitiveValueForLog("pc-123")))
	require.True(t, logSink.ContainsFieldValue("routing_prompt_cache_key_sha256", hashSensitiveValueForLog("pc-123")))
	require.True(t, logSink.ContainsFieldValue("upstream_session_source", "prompt_cache_key"))
	require.True(t, logSink.ContainsFieldValue("prompt_cache_key_dropped", "true"))
	require.True(t, logSink.ContainsFieldValue("previous_response_id_dropped", "true"))
	require.True(t, logSink.ContainsFieldValue("usage_cache_read_tokens", "121000"))
	require.True(t, logSink.ContainsField("upstream_body_prefix_4k_sha256"))
	require.True(t, logSink.ContainsField("upstream_input_prefix_4k_sha256"))
}

func TestEmitOpenAICacheProbeEvent_LogsCompactContentSeedSource(t *testing.T) {
	setGinTestMode()
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	c.Set("api_key", &APIKey{ID: 43})

	originalBody := []byte(`{"model":"gpt-5.5","instructions":"compact-test","input":[{"type":"text","text":"hello"}]}`)
	finalBody := []byte(`{"model":"gpt-5.5","instructions":"compact-test","input":[{"type":"text","text":"hello"}]}`)
	result := &OpenAIForwardResult{
		RequestID:     "resp_124",
		Model:         "gpt-5.5",
		UpstreamModel: "gpt-5.5",
		Usage: OpenAIUsage{
			InputTokens:          50,
			CacheReadInputTokens: 10,
			OutputTokens:         5,
		},
	}

	emitOpenAICacheProbeEvent(context.Background(), c, &Account{
		ID:       7806,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}, originalBody, finalBody, result, "", true)

	require.True(t, logSink.ContainsMessageAtLevel("OpenAI cache probe", "info"))
	require.True(t, logSink.ContainsFieldValue("upstream_session_source", "compact_content_seed"))
	require.True(t, logSink.ContainsField("upstream_session_id_sha256"))
}

func TestEmitOpenAICacheProbeEvent_SkipsNonCacheRequests(t *testing.T) {
	setGinTestMode()
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	emitOpenAICacheProbeEvent(context.Background(), c, &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}, []byte(`{"model":"gpt-5.4","input":[{"type":"input_text","text":"hello"}]}`), []byte(`{"model":"gpt-5.4","input":[{"type":"input_text","text":"hello"}]}`), &OpenAIForwardResult{
		RequestID:     "resp_no_cache",
		Model:         "gpt-5.4",
		UpstreamModel: "gpt-5.4",
		Usage:         OpenAIUsage{InputTokens: 10, OutputTokens: 1},
	}, "", false)

	require.False(t, logSink.ContainsMessage("OpenAI cache probe"))
}

func TestEmitOpenAICodexCompatFallbackEvent_LogsWithoutCacheProbe(t *testing.T) {
	setGinTestMode()
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set("api_key", &APIKey{ID: 7})

	emitOpenAICodexCompatFallbackEvent(
		context.Background(),
		c,
		&Account{
			ID:       9,
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
		},
		[]byte(`{"model":"gpt-5.4","input":[{"type":"message","id":"msg_123","role":"user","content":"hello"}]}`),
		[]byte(`{"model":"gpt-5.4","input":[{"type":"message","role":"user","content":"hello"}]}`),
		openAICodexCompatFallbackState{
			Triggered:    true,
			Reason:       "input_schema",
			BodyModified: true,
		},
		"success",
		http.StatusOK,
		"",
		"",
		&OpenAIForwardResult{
			RequestID: "resp_fallback_1",
			Model:     "gpt-5.4",
			Usage: OpenAIUsage{
				InputTokens:  3,
				OutputTokens: 2,
			},
		},
	)

	require.True(t, logSink.ContainsMessageAtLevel("OpenAI Codex compat fallback", "info"))
	require.True(t, logSink.ContainsFieldValue("component", "audit.openai_codex_compat_fallback"))
	require.True(t, logSink.ContainsFieldValue("request_id", "resp_fallback_1"))
	require.True(t, logSink.ContainsFieldValue("api_key_id", "7"))
	require.True(t, logSink.ContainsFieldValue("fallback_reason", "input_schema"))
	require.True(t, logSink.ContainsFieldValue("fallback_outcome", "success"))
	require.True(t, logSink.ContainsFieldValue("fallback_body_modified", "true"))
}
