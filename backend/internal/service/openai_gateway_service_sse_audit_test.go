package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewaySSEAuditInvokeResult struct {
	usage       *OpenAIUsage
	contentType string
	body        string
}

func invokeGatewaySSEAuditHandler(t *testing.T, body string, passthrough bool) (*gatewaySSEAuditInvokeResult, error) {
	t.Helper()

	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	account := &Account{ID: 99, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	var usage *OpenAIUsage
	var err error
	if passthrough {
		var result *openaiNonStreamingResultPassthrough
		result, err = svc.handlePassthroughSSEToJSON(resp, c, account, []byte(body), "gpt-4o", "gpt-4o")
		if result != nil {
			usage = result.usage
		}
	} else {
		var result *openaiNonStreamingResult
		result, err = svc.handleSSEToJSON(resp, c, account, []byte(body), "gpt-4o", "gpt-4o")
		if result != nil {
			usage = result.usage
		}
	}

	return &gatewaySSEAuditInvokeResult{
		usage:       usage,
		contentType: rec.Header().Get("Content-Type"),
		body:        rec.Body.String(),
	}, err
}

func TestGatewaySSEAudit_TerminalEnvelopeEventsReturnJSON(t *testing.T) {
	invokers := []struct {
		name        string
		passthrough bool
	}{
		{name: "standard", passthrough: false},
		{name: "passthrough", passthrough: true},
	}
	eventTypes := []string{"response.incomplete", "response.cancelled", "response.canceled"}

	for _, invoker := range invokers {
		invoker := invoker
		for _, eventType := range eventTypes {
			eventType := eventType
			t.Run(invoker.name+"_"+strings.ReplaceAll(eventType, ".", "_"), func(t *testing.T) {
				body := strings.Join([]string{
					`data: {"type":"response.in_progress","response":{"id":"resp_terminal"}}`,
					`data: {"type":"` + eventType + `","response":{"id":"resp_terminal","model":"gpt-4o","output":[{"type":"message","content":[{"type":"output_text","text":"terminal text"}]}],"usage":{"input_tokens":7,"output_tokens":9,"input_tokens_details":{"cached_tokens":2}}}}`,
					`data: [DONE]`,
				}, "\n")

				result, err := invokeGatewaySSEAuditHandler(t, body, invoker.passthrough)
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotNil(t, result.usage)
				require.Equal(t, 7, result.usage.InputTokens)
				require.Equal(t, 9, result.usage.OutputTokens)
				require.Equal(t, 2, result.usage.CacheReadInputTokens)
				require.Contains(t, result.contentType, "application/json")
				require.Contains(t, result.body, `"id":"resp_terminal"`)
				require.NotContains(t, result.body, `data: {"type":"`+eventType+`"`)
			})
		}
	}
}

func TestGatewaySSEAudit_LastNonEmptyTerminalUsageWins(t *testing.T) {
	invokers := []struct {
		name        string
		passthrough bool
	}{
		{name: "standard", passthrough: false},
		{name: "passthrough", passthrough: true},
	}

	body := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_usage_fallback","model":"gpt-4o","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}}`,
		`data: {"type":"response.incomplete","response":{"id":"resp_usage_fallback","usage":{"input_tokens":4,"output_tokens":6,"input_tokens_details":{"cached_tokens":1}}}}`,
		`data: {"type":"response.done","response":{"id":"resp_usage_fallback"}}`,
		`data: [DONE]`,
	}, "\n")

	for _, invoker := range invokers {
		invoker := invoker
		t.Run(invoker.name, func(t *testing.T) {
			result, err := invokeGatewaySSEAuditHandler(t, body, invoker.passthrough)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.usage)
			require.Equal(t, 4, result.usage.InputTokens)
			require.Equal(t, 6, result.usage.OutputTokens)
			require.Equal(t, 1, result.usage.CacheReadInputTokens)
			require.Contains(t, result.body, `"id":"resp_usage_fallback"`)
		})
	}
}

func TestGatewaySSEAudit_UsageObjectThatNeedsFallbackDoesNotSuppressIt(t *testing.T) {
	invokers := []struct {
		name        string
		passthrough bool
	}{
		{name: "standard", passthrough: false},
		{name: "passthrough", passthrough: true},
	}
	cases := []struct {
		name          string
		embeddedUsage string
	}{
		{name: "empty_usage_object", embeddedUsage: `{}`},
		{name: "partial_usage_object", embeddedUsage: `{"input_tokens":2}`},
	}

	for _, invoker := range invokers {
		invoker := invoker
		for _, tc := range cases {
			tc := tc
			t.Run(invoker.name+"_"+tc.name, func(t *testing.T) {
				body := strings.Join([]string{
					`data: {"type":"response.completed","response":{"id":"resp_partial_usage","model":"gpt-4o","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}],"usage":` + tc.embeddedUsage + `}}`,
					`data: {"type":"response.done","response":{"id":"resp_partial_usage","usage":{"input_tokens":11,"output_tokens":13,"input_tokens_details":{"cached_tokens":4}}}}`,
					`data: [DONE]`,
				}, "\n")

				result, err := invokeGatewaySSEAuditHandler(t, body, invoker.passthrough)
				require.NoError(t, err)
				require.NotNil(t, result)
				require.NotNil(t, result.usage)
				require.Equal(t, 11, result.usage.InputTokens)
				require.Equal(t, 13, result.usage.OutputTokens)
				require.Equal(t, 4, result.usage.CacheReadInputTokens)
				require.Contains(t, result.body, `"id":"resp_partial_usage"`)
			})
		}
	}
}

func TestGatewaySSEAudit_ExtractOpenAIUsageFromJSONBytes_RejectsFallbackOnlyUsageObjects(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "empty_usage_object", body: `{"usage":{}}`},
		{name: "partial_usage_object", body: `{"usage":{"input_tokens":2}}`},
		{name: "usage_details_only", body: `{"usage":{"input_tokens_details":{"cached_tokens":3}}}`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			usage, ok := extractOpenAIUsageFromJSONBytes([]byte(tc.body))
			require.False(t, ok)
			require.Equal(t, OpenAIUsage{}, usage)
		})
	}
}

func TestGatewaySSEAudit_PassthroughFailedTerminalReturnsProtocolError(t *testing.T) {
	body := strings.Join([]string{
		`data: {"type":"response.failed","error":{"message":"upstream rejected request"}}`,
		`data: [DONE]`,
	}, "\n")

	result, err := invokeGatewaySSEAuditHandler(t, body, true)
	require.Error(t, err)
	require.Nil(t, result.usage)
	require.Contains(t, result.contentType, "application/json")
	require.Contains(t, result.body, `"type":"upstream_error"`)
	require.Contains(t, result.body, `upstream rejected request`)
}

func TestGatewaySSEAudit_ResponseFailedTerminalEnvelopeReturnsJSON(t *testing.T) {
	invokers := []struct {
		name        string
		passthrough bool
	}{
		{name: "standard", passthrough: false},
		{name: "passthrough", passthrough: true},
	}

	body := strings.Join([]string{
		`data: {"type":"response.in_progress","response":{"id":"resp_failed_envelope"}}`,
		`data: {"type":"response.failed","response":{"id":"resp_failed_envelope","status":"failed","error":{"message":"upstream rejected request"},"usage":{"input_tokens":11,"output_tokens":13,"input_tokens_details":{"cached_tokens":4}}}}`,
		`data: [DONE]`,
	}, "\n")

	for _, invoker := range invokers {
		invoker := invoker
		t.Run(invoker.name, func(t *testing.T) {
			result, err := invokeGatewaySSEAuditHandler(t, body, invoker.passthrough)
			require.Error(t, err)
			var failoverErr *UpstreamFailoverError
			require.False(t, errors.As(err, &failoverErr))
			require.NotNil(t, result)
			require.Nil(t, result.usage)
			require.Contains(t, result.contentType, "application/json")
			require.Contains(t, result.body, `"type":"upstream_error"`)
			require.Contains(t, result.body, `upstream rejected request`)
			require.NotContains(t, result.body, `"id":"resp_failed_envelope"`)
		})
	}
}

func TestGatewaySSEAudit_ResponseFailedTerminalEnvelopeDominatesLaterDone(t *testing.T) {
	invokers := []struct {
		name        string
		passthrough bool
	}{
		{name: "standard", passthrough: false},
		{name: "passthrough", passthrough: true},
	}

	body := strings.Join([]string{
		`data: {"type":"response.in_progress","response":{"id":"resp_failed_then_done"}}`,
		`data: {"type":"response.failed","response":{"id":"resp_failed_then_done","status":"failed","error":{"message":"upstream rejected request"}}}`,
		`data: {"type":"response.done","response":{"id":"resp_failed_then_done","model":"gpt-4o","output":[{"type":"message","content":[{"type":"output_text","text":"should not win"}]}],"usage":{"input_tokens":11,"output_tokens":13,"input_tokens_details":{"cached_tokens":4}}}}`,
		`data: [DONE]`,
	}, "\n")

	for _, invoker := range invokers {
		invoker := invoker
		t.Run(invoker.name, func(t *testing.T) {
			result, err := invokeGatewaySSEAuditHandler(t, body, invoker.passthrough)
			require.Error(t, err)
			var failoverErr *UpstreamFailoverError
			require.False(t, errors.As(err, &failoverErr))
			require.NotNil(t, result)
			require.Nil(t, result.usage)
			require.Contains(t, result.contentType, "application/json")
			require.Contains(t, result.body, `"type":"upstream_error"`)
			require.Contains(t, result.body, `upstream rejected request`)
			require.NotContains(t, result.body, `should not win`)
		})
	}
}

func TestGatewaySSEAudit_ResponseFailedTerminalEnvelopeWithCodexRateLimitAdvisoryDoesNotReturn429(t *testing.T) {
	invokers := []struct {
		name        string
		passthrough bool
	}{
		{name: "standard", passthrough: false},
		{name: "passthrough", passthrough: true},
	}

	body := strings.Join([]string{
		`data: {"type":"codex.rate_limits","metered_limit_name":"codex","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":95,"window_minutes":300,"reset_at":1700000000},"secondary":null},"credits":{"has_credits":false,"unlimited":false,"balance":null}}`,
		`data: {"type":"response.failed","response":{"id":"resp_failed_retryable","status":"failed","error":{"message":"temporary upstream failure"}}}`,
		`data: [DONE]`,
	}, "\n")

	for _, invoker := range invokers {
		invoker := invoker
		t.Run(invoker.name, func(t *testing.T) {
			result, err := invokeGatewaySSEAuditHandler(t, body, invoker.passthrough)
			require.Error(t, err)
			var failoverErr *UpstreamFailoverError
			require.False(t, errors.As(err, &failoverErr))
			require.NotNil(t, result)
			require.Nil(t, result.usage)
			require.Contains(t, result.contentType, "application/json")
			require.Contains(t, result.body, `"type":"upstream_error"`)
			require.Contains(t, result.body, `temporary upstream failure`)
		})
	}
}
