package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClassifyOpenAIWSAcquireError(t *testing.T) {
	t.Run("dial_426_upgrade_required", func(t *testing.T) {
		err := &openAIWSDialError{StatusCode: 426, Err: errors.New("upgrade required")}
		require.Equal(t, "upgrade_required", classifyOpenAIWSAcquireError(err))
	})

	t.Run("queue_full", func(t *testing.T) {
		require.Equal(t, "conn_queue_full", classifyOpenAIWSAcquireError(errOpenAIWSConnQueueFull))
	})

	t.Run("preferred_conn_unavailable", func(t *testing.T) {
		require.Equal(t, "preferred_conn_unavailable", classifyOpenAIWSAcquireError(errOpenAIWSPreferredConnUnavailable))
	})

	t.Run("acquire_timeout", func(t *testing.T) {
		require.Equal(t, "acquire_timeout", classifyOpenAIWSAcquireError(context.DeadlineExceeded))
	})

	t.Run("auth_failed_401", func(t *testing.T) {
		err := &openAIWSDialError{StatusCode: 401, Err: errors.New("unauthorized")}
		require.Equal(t, "auth_failed", classifyOpenAIWSAcquireError(err))
	})

	t.Run("upstream_rate_limited", func(t *testing.T) {
		err := &openAIWSDialError{StatusCode: 429, Err: errors.New("rate limited")}
		require.Equal(t, "upstream_rate_limited", classifyOpenAIWSAcquireError(err))
	})

	t.Run("upstream_5xx", func(t *testing.T) {
		err := &openAIWSDialError{StatusCode: 502, Err: errors.New("bad gateway")}
		require.Equal(t, "upstream_5xx", classifyOpenAIWSAcquireError(err))
	})

	t.Run("dial_failed_other_status", func(t *testing.T) {
		err := &openAIWSDialError{StatusCode: 418, Err: errors.New("teapot")}
		require.Equal(t, "dial_failed", classifyOpenAIWSAcquireError(err))
	})

	t.Run("other", func(t *testing.T) {
		require.Equal(t, "acquire_conn", classifyOpenAIWSAcquireError(errors.New("x")))
	})

	t.Run("nil", func(t *testing.T) {
		require.Equal(t, "acquire_conn", classifyOpenAIWSAcquireError(nil))
	})
}

func TestClassifyOpenAIWSDialError(t *testing.T) {
	t.Run("handshake_not_finished", func(t *testing.T) {
		err := &openAIWSDialError{
			StatusCode: http.StatusBadGateway,
			Err:        errors.New("WebSocket protocol error: Handshake not finished"),
		}
		require.Equal(t, "handshake_not_finished", classifyOpenAIWSDialError(err))
	})

	t.Run("context_deadline", func(t *testing.T) {
		err := &openAIWSDialError{
			StatusCode: 0,
			Err:        context.DeadlineExceeded,
		}
		require.Equal(t, "ctx_deadline_exceeded", classifyOpenAIWSDialError(err))
	})
}

func TestSummarizeOpenAIWSDialError(t *testing.T) {
	err := &openAIWSDialError{
		StatusCode: http.StatusBadGateway,
		ResponseHeaders: http.Header{
			"Server":       []string{"cloudflare"},
			"Via":          []string{"1.1 example"},
			"Cf-Ray":       []string{"abcd1234"},
			"X-Request-Id": []string{"req_123"},
		},
		Err: errors.New("WebSocket protocol error: Handshake not finished"),
	}

	status, class, closeStatus, closeReason, server, via, cfRay, reqID := summarizeOpenAIWSDialError(err)
	require.Equal(t, http.StatusBadGateway, status)
	require.Equal(t, "handshake_not_finished", class)
	require.Equal(t, "-", closeStatus)
	require.Equal(t, "-", closeReason)
	require.Equal(t, "cloudflare", server)
	require.Equal(t, "1.1 example", via)
	require.Equal(t, "abcd1234", cfRay)
	require.Equal(t, "req_123", reqID)
}

func TestClassifyOpenAIWSErrorEvent(t *testing.T) {
	reason, recoverable := classifyOpenAIWSErrorEvent([]byte(`{"type":"error","error":{"code":"upgrade_required","message":"Upgrade required"}}`))
	require.Equal(t, "upgrade_required", reason)
	require.True(t, recoverable)

	reason, recoverable = classifyOpenAIWSErrorEvent([]byte(`{"type":"error","error":{"code":"previous_response_not_found","message":"not found"}}`))
	require.Equal(t, "previous_response_not_found", reason)
	require.True(t, recoverable)

	reason, recoverable = classifyOpenAIWSErrorEvent([]byte(`{"type":"error","error":{"type":"usage_limit_reached","code":"billing_hard_limit","message":"You've hit your usage limit. Upgrade to Plus to continue using Codex, or try again after 1:00 PM."}}`))
	require.Equal(t, "upstream_rate_limited", reason)
	require.True(t, recoverable)

	reason, recoverable = classifyOpenAIWSErrorEvent([]byte(`{"type":"error","error":{"type":"invalid_request_error","code":"invalid_request","message":"No tool call found for function call output with call_id call_1."}}`))
	require.Equal(t, "call_id", reason)
	require.True(t, recoverable)

	reason, recoverable = classifyOpenAIWSErrorEvent([]byte(`{"type":"error","error":{"type":"invalid_request_error","code":"invalid_request","message":"The 'gpt-4o-mini' model is not supported when using Codex with a ChatGPT account."}}`))
	require.Equal(t, "model_unavailable", reason)
	require.True(t, recoverable)

	reason, recoverable = classifyOpenAIWSErrorEvent([]byte(`{"type":"error","error":{"type":"invalid_request_error","code":"invalid_request","message":"The field model.metadata does not exist on this request."}}`))
	require.Equal(t, "event_error", reason)
	require.False(t, recoverable)
}

func TestClassifyOpenAIWSSoftRateLimitAdvisory(t *testing.T) {
	msg, matched := classifyOpenAIWSSoftRateLimitAdvisory([]byte(`{"type":"codex.rate_limits","metered_limit_name":"codex","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":91,"window_minutes":300,"reset_at":1700000000},"secondary":null},"credits":{"has_credits":false,"unlimited":false,"balance":null}}`))
	require.True(t, matched)
	require.Contains(t, msg, "Approaching upstream rate limits")

	msg, matched = classifyOpenAIWSSoftRateLimitAdvisory([]byte(`{"type":"codex.rate_limits","rate_limits":{"allowed":true,"limit_reached":false,"primary":null,"secondary":{"used_percent":95,"window_minutes":10080,"reset_at":1700000000}}}`))
	require.True(t, matched)
	require.Contains(t, msg, "Approaching upstream rate limits")

	msg, matched = classifyOpenAIWSSoftRateLimitAdvisory([]byte(`{"type":"response.output_text.delta","delta":"Approaching rate limits\nSwitch to gpt-5.4-mini for lower credit usage?\n1. Switch to gpt-5.4-mini\n2. Keep current model\n3. Keep current model (never show again)"}`))
	require.False(t, matched)
	require.Empty(t, msg)

	msg, matched = classifyOpenAIWSSoftRateLimitAdvisory([]byte(`{"type":"response.completed","response":{"output":[{"type":"message","content":[{"type":"output_text","text":"Approaching rate limits\nSwitch to gpt-5.4-mini for lower credit usage?\n1. Switch to gpt-5.4-mini\n2. Keep current model\n3. Keep current model (never show again)"}]}]}}`))
	require.False(t, matched)
	require.Empty(t, msg)

	msg, matched = classifyOpenAIWSSoftRateLimitAdvisory([]byte(`{"type":"codex.rate_limits","metered_limit_name":"codex_other","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":95}}}`))
	require.False(t, matched)
	require.Empty(t, msg)

	msg, matched = classifyOpenAIWSSoftRateLimitAdvisory([]byte(`{"type":"codex.rate_limits","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":95}},"credits":{"has_credits":true,"unlimited":false}}`))
	require.False(t, matched)
	require.Empty(t, msg)

	msg, matched = classifyOpenAIWSSoftRateLimitAdvisory([]byte(`{"type":"codex.rate_limits","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":89.9}}}`))
	require.False(t, matched)
	require.Empty(t, msg)
}

func TestClassifyOpenAIWSReconnectReason(t *testing.T) {
	reason, retryable := classifyOpenAIWSReconnectReason(wrapOpenAIWSFallback("policy_violation", errors.New("policy")))
	require.Equal(t, "policy_violation", reason)
	require.False(t, retryable)

	reason, retryable = classifyOpenAIWSReconnectReason(wrapOpenAIWSFallback("read_event", errors.New("io")))
	require.Equal(t, "read_event", reason)
	require.True(t, retryable)

	reason, retryable = classifyOpenAIWSReconnectReason(wrapOpenAIWSFallback("model_unavailable", errors.New("unsupported model")))
	require.Equal(t, "model_unavailable", reason)
	require.False(t, retryable)

	reason, retryable = classifyOpenAIWSReconnectReason(wrapOpenAIWSFallback("session_preempted", errOpenAIWSSessionPreempted))
	require.Equal(t, "session_preempted", reason)
	require.False(t, retryable)
}

func TestShouldFallbackOpenAIWSToHTTP_AuthFailed(t *testing.T) {
	err := wrapOpenAIWSFallback("auth_failed", &openAIWSDialError{
		StatusCode: http.StatusUnauthorized,
		Err:        errors.New("unauthorized websocket handshake"),
	})

	require.True(t, shouldFallbackOpenAIWSToHTTP(err))
}

func TestShouldFallbackOpenAIWSToHTTP_SessionPreempted(t *testing.T) {
	require.False(t, shouldFallbackOpenAIWSToHTTP(wrapOpenAIWSFallback("session_preempted", errOpenAIWSSessionPreempted)))
}

func TestIsOpenAIWSSessionPreemptedError(t *testing.T) {
	require.True(t, IsOpenAIWSSessionPreemptedError(wrapOpenAIWSFallback("session_preempted", errOpenAIWSSessionPreempted)))
	require.True(t, IsOpenAIWSSessionPreemptedError(fmt.Errorf("wrapped: %w", errOpenAIWSSessionPreempted)))
	require.False(t, IsOpenAIWSSessionPreemptedError(wrapOpenAIWSFallback("acquire_timeout", context.DeadlineExceeded)))
	require.False(t, IsOpenAIWSSessionPreemptedError(context.Canceled))
}

func TestOpenAIWSErrorHTTPStatus(t *testing.T) {
	require.Equal(t, http.StatusBadRequest, openAIWSErrorHTTPStatus([]byte(`{"type":"error","error":{"type":"invalid_request_error","code":"invalid_request","message":"invalid input"}}`)))
	require.Equal(t, http.StatusUnauthorized, openAIWSErrorHTTPStatus([]byte(`{"type":"error","error":{"type":"authentication_error","code":"invalid_api_key","message":"auth failed"}}`)))
	require.Equal(t, http.StatusForbidden, openAIWSErrorHTTPStatus([]byte(`{"type":"error","error":{"type":"permission_error","code":"forbidden","message":"forbidden"}}`)))
	require.Equal(t, http.StatusTooManyRequests, openAIWSErrorHTTPStatus([]byte(`{"type":"error","error":{"type":"rate_limit_error","code":"rate_limit_exceeded","message":"rate limited"}}`)))
	require.Equal(t, http.StatusBadGateway, openAIWSErrorHTTPStatus([]byte(`{"type":"error","error":{"type":"server_error","code":"server_error","message":"server"}}`)))
}

func TestResolveOpenAIWSFallbackErrorResponse(t *testing.T) {
	t.Run("previous_response_not_found", func(t *testing.T) {
		statusCode, errType, clientMessage, upstreamMessage, ok := resolveOpenAIWSFallbackErrorResponse(
			wrapOpenAIWSFallback("previous_response_not_found", errors.New("previous response not found")),
		)
		require.True(t, ok)
		require.Equal(t, http.StatusBadRequest, statusCode)
		require.Equal(t, "invalid_request_error", errType)
		require.Equal(t, "previous response not found", clientMessage)
		require.Equal(t, "previous response not found", upstreamMessage)
	})

	t.Run("auth_failed_uses_dial_status", func(t *testing.T) {
		statusCode, errType, clientMessage, upstreamMessage, ok := resolveOpenAIWSFallbackErrorResponse(
			wrapOpenAIWSFallback("auth_failed", &openAIWSDialError{
				StatusCode: http.StatusForbidden,
				Err:        errors.New("forbidden"),
			}),
		)
		require.True(t, ok)
		require.Equal(t, http.StatusForbidden, statusCode)
		require.Equal(t, "upstream_error", errType)
		require.Equal(t, "forbidden", clientMessage)
		require.Equal(t, "forbidden", upstreamMessage)
	})

	t.Run("call_id_uses_invalid_request_error", func(t *testing.T) {
		statusCode, errType, clientMessage, upstreamMessage, ok := resolveOpenAIWSFallbackErrorResponse(
			wrapOpenAIWSFallback("call_id", errors.New("No tool call found for function call output with call_id call_1.")),
		)
		require.True(t, ok)
		require.Equal(t, http.StatusBadRequest, statusCode)
		require.Equal(t, "invalid_request_error", errType)
		require.Equal(t, "No tool call found for function call output with call_id call_1.", clientMessage)
		require.Equal(t, "No tool call found for function call output with call_id call_1.", upstreamMessage)
	})

	t.Run("session_preempted_uses_client_canceled", func(t *testing.T) {
		statusCode, errType, clientMessage, upstreamMessage, ok := resolveOpenAIWSFallbackErrorResponse(
			wrapOpenAIWSFallback("session_preempted", errOpenAIWSSessionPreempted),
		)
		require.True(t, ok)
		require.Equal(t, 499, statusCode)
		require.Equal(t, "request_canceled", errType)
		require.Equal(t, "Superseded by a newer request in the same session", clientMessage)
		require.Equal(t, "Superseded by a newer request in the same session", upstreamMessage)
	})

	t.Run("non_fallback_error_not_resolved", func(t *testing.T) {
		_, _, _, _, ok := resolveOpenAIWSFallbackErrorResponse(errors.New("plain error"))
		require.False(t, ok)
	})
}

func TestWriteOpenAIWSFallbackErrorResponse_SessionPreemptedDoesNotSetOpsUpstreamError(t *testing.T) {
	setGinTestMode()
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	wrote := svc.writeOpenAIWSFallbackErrorResponse(c, &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Name:     "oauth",
	}, wrapOpenAIWSFallback("session_preempted", errOpenAIWSSessionPreempted))

	require.True(t, wrote)
	require.Equal(t, 499, rec.Code)
	_, hasType := c.Get(OpsUpstreamErrorTypeKey)
	require.False(t, hasType)
	_, hasMessage := c.Get(OpsUpstreamErrorMessageKey)
	require.False(t, hasMessage)
	_, hasEvents := c.Get(OpsUpstreamErrorsKey)
	require.False(t, hasEvents)
}

func TestNewOpenAIWSFailoverError(t *testing.T) {
	setGinTestMode()

	t.Run("rate_limited_promotes_to_failover", func(t *testing.T) {
		svc := &OpenAIGatewayService{cfg: &config.Config{}}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

		failoverErr := svc.newOpenAIWSFailoverError(
			c,
			&Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"},
			wrapOpenAIWSFallback("upstream_rate_limited", errors.New("You've hit your usage limit. Upgrade to Plus to continue using Codex, or try again after 1:00 PM.")),
		)

		require.NotNil(t, failoverErr)
		require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
		require.Contains(t, string(failoverErr.ResponseBody), `"type":"rate_limit_error"`)
		require.Contains(t, string(failoverErr.ResponseBody), "usage limit")
		require.False(t, c.Writer.Written())
	})

	t.Run("non_rate_limited_keeps_client_response_path", func(t *testing.T) {
		svc := &OpenAIGatewayService{cfg: &config.Config{}}
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

		failoverErr := svc.newOpenAIWSFailoverError(
			c,
			&Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"},
			wrapOpenAIWSFallback("upgrade_required", errors.New("upgrade required")),
		)

		require.Nil(t, failoverErr)
	})
}

func TestOpenAIWSFallbackCooling(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	svc.cfg.Gateway.OpenAIWS.FallbackCooldownSeconds = 1

	require.False(t, svc.isOpenAIWSFallbackCooling(1))
	svc.markOpenAIWSFallbackCooling(1, "upgrade_required")
	require.True(t, svc.isOpenAIWSFallbackCooling(1))

	svc.clearOpenAIWSFallbackCooling(1)
	require.False(t, svc.isOpenAIWSFallbackCooling(1))

	svc.markOpenAIWSFallbackCooling(2, "x")
	time.Sleep(1200 * time.Millisecond)
	require.False(t, svc.isOpenAIWSFallbackCooling(2))
}

func TestOpenAIWSRetryBackoff(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	svc.cfg.Gateway.OpenAIWS.RetryBackoffInitialMS = 100
	svc.cfg.Gateway.OpenAIWS.RetryBackoffMaxMS = 400
	svc.cfg.Gateway.OpenAIWS.RetryJitterRatio = 0

	require.Equal(t, time.Duration(100)*time.Millisecond, svc.openAIWSRetryBackoff(1))
	require.Equal(t, time.Duration(200)*time.Millisecond, svc.openAIWSRetryBackoff(2))
	require.Equal(t, time.Duration(400)*time.Millisecond, svc.openAIWSRetryBackoff(3))
	require.Equal(t, time.Duration(400)*time.Millisecond, svc.openAIWSRetryBackoff(4))
}

func TestOpenAIWSRetryTotalBudget(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	svc.cfg.Gateway.OpenAIWS.RetryTotalBudgetMS = 1200
	require.Equal(t, 1200*time.Millisecond, svc.openAIWSRetryTotalBudget())

	svc.cfg.Gateway.OpenAIWS.RetryTotalBudgetMS = 0
	require.Equal(t, time.Duration(0), svc.openAIWSRetryTotalBudget())
}

func TestClassifyOpenAIWSReadFallbackReason(t *testing.T) {
	require.Equal(t, "policy_violation", classifyOpenAIWSReadFallbackReason(coderws.CloseError{Code: coderws.StatusPolicyViolation}))
	require.Equal(t, "message_too_big", classifyOpenAIWSReadFallbackReason(coderws.CloseError{Code: coderws.StatusMessageTooBig}))
	require.Equal(t, "read_event", classifyOpenAIWSReadFallbackReason(errors.New("io")))
}

func TestOpenAIWSStoreDisabledConnMode(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	svc.cfg.Gateway.OpenAIWS.StoreDisabledForceNewConn = true
	require.Equal(t, openAIWSStoreDisabledConnModeStrict, svc.openAIWSStoreDisabledConnMode())

	svc.cfg.Gateway.OpenAIWS.StoreDisabledConnMode = "adaptive"
	require.Equal(t, openAIWSStoreDisabledConnModeAdaptive, svc.openAIWSStoreDisabledConnMode())

	svc.cfg.Gateway.OpenAIWS.StoreDisabledConnMode = ""
	svc.cfg.Gateway.OpenAIWS.StoreDisabledForceNewConn = false
	require.Equal(t, openAIWSStoreDisabledConnModeOff, svc.openAIWSStoreDisabledConnMode())
}

func TestShouldForceNewConnOnStoreDisabled(t *testing.T) {
	require.True(t, shouldForceNewConnOnStoreDisabled(openAIWSStoreDisabledConnModeStrict, ""))
	require.False(t, shouldForceNewConnOnStoreDisabled(openAIWSStoreDisabledConnModeOff, "policy_violation"))

	require.True(t, shouldForceNewConnOnStoreDisabled(openAIWSStoreDisabledConnModeAdaptive, "policy_violation"))
	require.True(t, shouldForceNewConnOnStoreDisabled(openAIWSStoreDisabledConnModeAdaptive, "prewarm_message_too_big"))
	require.False(t, shouldForceNewConnOnStoreDisabled(openAIWSStoreDisabledConnModeAdaptive, "read_event"))
}

func TestShouldForceNewConnOnHTTPIngressWSOneShotRetry(t *testing.T) {
	require.False(t, shouldForceNewConnOnHTTPIngressWSOneShotRetry(""))
	require.False(t, shouldForceNewConnOnHTTPIngressWSOneShotRetry("policy_violation"))
	require.False(t, shouldForceNewConnOnHTTPIngressWSOneShotRetry("prewarm_read_event"))

	require.True(t, shouldForceNewConnOnHTTPIngressWSOneShotRetry("read_event"))
	require.True(t, shouldForceNewConnOnHTTPIngressWSOneShotRetry("write_request"))
	require.True(t, shouldForceNewConnOnHTTPIngressWSOneShotRetry("write"))
}

func TestShouldUseOpenAIWSNeutralForColdSessionToolContinuationContext(t *testing.T) {
	account := &Account{Type: AccountTypeOAuth}

	for _, tc := range []struct {
		name    string
		payload map[string]any
		want    bool
	}{
		{
			name: "tool_output_with_full_context",
			payload: map[string]any{
				"input": []any{
					map[string]any{"type": "function_call", "call_id": "call_1", "name": "shell", "arguments": "{}"},
					map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok"},
					map[string]any{"type": "message", "role": "user", "content": "continue"},
				},
			},
			want: true,
		},
		{
			name: "tool_output_with_item_reference_context_stays_session_bound",
			payload: map[string]any{
				"input": []any{
					map[string]any{"type": "item_reference", "id": "call_1"},
					map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok"},
				},
			},
			want: false,
		},
		{
			name: "orphan_tool_output_stays_session_bound",
			payload: map[string]any{
				"input": []any{
					map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok"},
				},
			},
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldUseOpenAIWSNeutralForColdSession(
				account,
				false,
				true,
				"",
				"session-hash",
				"",
				"",
				tc.payload,
			))
		})
	}
}

func TestOpenAIWSRetryMetricsSnapshot(t *testing.T) {
	svc := &OpenAIGatewayService{}
	svc.recordOpenAIWSRetryAttempt(150 * time.Millisecond)
	svc.recordOpenAIWSRetryAttempt(0)
	svc.recordOpenAIWSRetryExhausted()
	svc.recordOpenAIWSNonRetryableFastFallback()

	snapshot := svc.SnapshotOpenAIWSRetryMetrics()
	require.Equal(t, int64(2), snapshot.RetryAttemptsTotal)
	require.Equal(t, int64(150), snapshot.RetryBackoffMsTotal)
	require.Equal(t, int64(1), snapshot.RetryExhaustedTotal)
	require.Equal(t, int64(1), snapshot.NonRetryableFastFallbackTotal)
}

func TestShouldLogOpenAIWSPayloadSchema(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	svc.cfg.Gateway.OpenAIWS.PayloadLogSampleRate = 0
	require.True(t, svc.shouldLogOpenAIWSPayloadSchema(1), "首次尝试应始终记录 payload_schema")
	require.False(t, svc.shouldLogOpenAIWSPayloadSchema(2))

	svc.cfg.Gateway.OpenAIWS.PayloadLogSampleRate = 1
	require.True(t, svc.shouldLogOpenAIWSPayloadSchema(2))
}

func TestOpenAIWSContinuationProbeLogMessageIncludesStoreAndTimingFields(t *testing.T) {
	msg := openAIWSContinuationProbeLogMessage(openAIWSContinuationProbeLog{
		AccountID:              42,
		AccountType:            AccountTypeOAuth,
		ConnID:                 "oa_ws_42_1",
		PreviousResponseID:     "resp_prev_123",
		PreviousResponseIDKind: OpenAIPreviousResponseIDKindResponseID,
		PreferredConnID:        "oa_ws_42_1",
		ConnReused:             true,
		StoreDisabled:          false,
		StoreMode:              openAIWSStoreModeIncremental,
		StoreEnabled:           true,
		StickyAccountHit:       true,
		ConnAffinityHit:        true,
		FallbackReason:         "",
		PayloadBytes:           3210,
		ConnPickMs:             3,
		QueueWaitMs:            0,
		SessionHash:            "abcdef1234567890",
		HeaderSessionID:        "session-header",
		HeaderConversationID:   "conversation-header",
		SessionIDSource:        "header",
		ConversationIDSource:   "header",
		HasTurnState:           true,
		TurnStateLen:           17,
		HasPromptCacheKey:      true,
	})

	require.Contains(t, msg, "continuation_probe")
	require.Contains(t, msg, "store_mode=incremental")
	require.Contains(t, msg, "store_enabled=true")
	require.Contains(t, msg, "sticky_account_hit=true")
	require.Contains(t, msg, "conn_affinity_hit=true")
	require.Contains(t, msg, "fallback_reason=-")
	require.Contains(t, msg, "payload_bytes=3210")
	require.Contains(t, msg, "conn_pick_ms=3")
	require.Contains(t, msg, "queue_wait_ms=0")
}

func TestOpenAIWSDiagnosticStartLogMessageIncludesTemporaryStoreAndConnFields(t *testing.T) {
	msg := openAIWSDiagnosticStartLogMessage(openAIWSDiagnosticStartLog{
		RequestID:                 "req-local-1",
		ClientRequestID:           "client-req-1",
		AccountID:                 42,
		AccountType:               AccountTypeOAuth,
		ConnProfile:               openAIWSConnProfileSessionBound,
		ConnID:                    "oa_ws_42_1",
		ConnReused:                true,
		Transport:                 "websocket",
		Model:                     "gpt-5.5",
		Stream:                    "true",
		PayloadEventType:          "response.create",
		PayloadBytes:              4567,
		PayloadKeys:               "input,model,previous_response_id,store",
		InputSummary:              "items=1",
		PreviousResponseID:        "resp_prev_123",
		PreviousResponseIDKind:    OpenAIPreviousResponseIDKindResponseID,
		PreferredConnID:           "oa_ws_42_1",
		StoreMode:                 openAIWSStoreModeIncremental,
		StoreEnabled:              true,
		StoreDisabled:             false,
		StickyAccountHit:          true,
		ConnAffinityHit:           true,
		FallbackReason:            "",
		DroppedPreviousResponseID: false,
		UnsafeToolContinuation:    false,
		ConnPickMs:                2,
		QueueWaitMs:               0,
		SessionHash:               "abcdef1234567890",
		HeaderSessionID:           "session-header",
		HeaderConversationID:      "conversation-header",
		SessionIDSource:           "header",
		ConversationIDSource:      "header",
		HasTurnState:              true,
		TurnStateLen:              12,
		HasPromptCacheKey:         true,
		HasTools:                  true,
		HTTPIngressWSOneShot:      false,
		ForceNewConn:              false,
		AffinityOnlyReuse:         true,
		StoreDisabledConnMode:     openAIWSStoreDisabledConnModeStrict,
		ProxyEnabled:              true,
	})

	require.Contains(t, msg, "openai_ws_diag_start")
	require.Contains(t, msg, "temporary_diag=ctx_pool_store_true")
	require.Contains(t, msg, "remove_after_debug=true")
	require.Contains(t, msg, "request_id=req-local-1")
	require.Contains(t, msg, "client_request_id=client-req-1")
	require.Contains(t, msg, "account_id=42")
	require.Contains(t, msg, "conn_profile=session_bound")
	require.Contains(t, msg, "conn_reused=true")
	require.Contains(t, msg, "transport=websocket")
	require.Contains(t, msg, "model=gpt-5.5")
	require.Contains(t, msg, "payload_event=response.create")
	require.Contains(t, msg, "payload_bytes=4567")
	require.Contains(t, msg, "payload_keys=input,model,previous_response_id,store")
	require.Contains(t, msg, "previous_response_id=resp_prev_123")
	require.Contains(t, msg, "store_mode=incremental")
	require.Contains(t, msg, "store_enabled=true")
	require.Contains(t, msg, "sticky_account_hit=true")
	require.Contains(t, msg, "conn_affinity_hit=true")
	require.Contains(t, msg, "conn_pick_ms=2")
	require.Contains(t, msg, "queue_wait_ms=0")
	require.Contains(t, msg, "has_prompt_cache_key=true")
	require.Contains(t, msg, "has_tools=true")
	require.Contains(t, msg, "affinity_only_reuse=true")
	require.Contains(t, msg, "proxy_enabled=true")
}

func TestOpenAIWSDiagnosticCompletedLogMessageIncludesTemporaryTTFTAndUsageFields(t *testing.T) {
	msg := openAIWSDiagnosticCompletedLogMessage(openAIWSDiagnosticCompletedLog{
		RequestID:              "req-local-2",
		ClientRequestID:        "client-req-2",
		AccountID:              43,
		AccountType:            AccountTypeOAuth,
		ConnProfile:            openAIWSConnProfileNeutral,
		ConnID:                 "oa_ws_43_1",
		ConnReused:             false,
		ResponseID:             "resp_done_123",
		Model:                  "gpt-5.4",
		UpstreamModel:          "gpt-5.4",
		Stream:                 true,
		StoreMode:              openAIWSStoreModeFull,
		StoreEnabled:           false,
		StoreDisabled:          true,
		HasPreviousResponseID:  false,
		PreviousResponseIDKind: OpenAIPreviousResponseIDKindEmpty,
		PayloadBytes:           98765,
		DurationMs:             12345,
		FirstTokenMs:           2345,
		Events:                 12,
		TokenEvents:            4,
		TerminalEvents:         1,
		BufferedEvents:         3,
		BufferedFlushed:        3,
		FirstEvent:             "response.created",
		LastEvent:              "response.completed",
		WroteDownstream:        true,
		ClientDisconnected:     false,
		HTTPIngressWSOneShot:   true,
		InputTokens:            1200,
		CacheReadTokens:        800,
		CacheCreationTokens:    16,
		OutputTokens:           321,
	})

	require.Contains(t, msg, "openai_ws_diag_completed")
	require.Contains(t, msg, "temporary_diag=ctx_pool_store_true")
	require.Contains(t, msg, "remove_after_debug=true")
	require.Contains(t, msg, "request_id=req-local-2")
	require.Contains(t, msg, "client_request_id=client-req-2")
	require.Contains(t, msg, "account_id=43")
	require.Contains(t, msg, "conn_profile=neutral")
	require.Contains(t, msg, "conn_reused=false")
	require.Contains(t, msg, "response_id=resp_done_123")
	require.Contains(t, msg, "store_mode=full")
	require.Contains(t, msg, "store_enabled=false")
	require.Contains(t, msg, "store_disabled=true")
	require.Contains(t, msg, "has_previous_response_id=false")
	require.Contains(t, msg, "payload_bytes=98765")
	require.Contains(t, msg, "duration_ms=12345")
	require.Contains(t, msg, "first_token_ms=2345")
	require.Contains(t, msg, "events=12")
	require.Contains(t, msg, "token_events=4")
	require.Contains(t, msg, "terminal_events=1")
	require.Contains(t, msg, "first_event=response.created")
	require.Contains(t, msg, "last_event=response.completed")
	require.Contains(t, msg, "http_ingress_ws_one_shot=true")
	require.Contains(t, msg, "input_tokens=1200")
	require.Contains(t, msg, "cache_read_tokens=800")
	require.Contains(t, msg, "cache_creation_tokens=16")
	require.Contains(t, msg, "output_tokens=321")
}
