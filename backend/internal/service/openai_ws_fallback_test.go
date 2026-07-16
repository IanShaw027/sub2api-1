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
	require.False(t, matched)
	require.Empty(t, msg)

	msg, matched = classifyOpenAIWSSoftRateLimitAdvisory([]byte(`{"type":"codex.rate_limits","rate_limits":{"allowed":true,"limit_reached":false,"primary":null,"secondary":{"used_percent":95,"window_minutes":10080,"reset_at":1700000000}}}`))
	require.False(t, matched)
	require.Empty(t, msg)

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

	responseFailed := &openAIWSResponseFailedError{code: "server_error", errType: "server_error", message: "temporary", retryable: true}
	reason, retryable = classifyOpenAIWSReconnectReason(wrapOpenAIWSFallback("prewarm_response_failed", responseFailed))
	require.Equal(t, "prewarm_response_failed", reason)
	require.True(t, retryable)
}

func TestShouldFallbackOpenAIWSToHTTP_AuthFailed(t *testing.T) {
	err := wrapOpenAIWSFallback("auth_failed", &openAIWSDialError{
		StatusCode: http.StatusUnauthorized,
		Err:        errors.New("unauthorized websocket handshake"),
	})

	require.True(t, shouldFallbackOpenAIWSToHTTP(err))
}

func TestOpenAIWSPaymentRequiredEventFallsBackWith402(t *testing.T) {
	reason, recoverable := classifyOpenAIWSErrorEvent([]byte(`{"type":"error","error":{"code":"deactivated_workspace","message":"workspace deactivated"}}`))
	require.True(t, recoverable)
	require.Equal(t, "payment_required", reason)

	err := wrapOpenAIWSFallback(reason, errors.New("workspace deactivated"))
	require.True(t, shouldFallbackOpenAIWSToHTTP(err))
	statusCode, _, _, _, ok := resolveOpenAIWSFallbackErrorResponse(err)
	require.True(t, ok)
	require.Equal(t, http.StatusPaymentRequired, statusCode)
}

func TestOpenAIWSHTTPFallbackBlockerRequiresReplayablePayloadAndNoOutput(t *testing.T) {
	safePayload := []byte(`{"model":"gpt-5.5","input":[{"type":"message","role":"user","content":"hello"}]}`)
	unsafeToolPayload := []byte(`{"model":"gpt-5.5","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)
	err := wrapOpenAIWSFallback("message_too_big", coderws.CloseError{Code: coderws.StatusMessageTooBig})

	require.Equal(t, "response_writer_missing", openAIWSHTTPFallbackBlocker(nil, err, safePayload, safePayload))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	require.Empty(t, openAIWSHTTPFallbackBlocker(c, err, safePayload, safePayload))
	require.Equal(t, "ws_payload_not_replayable", openAIWSHTTPFallbackBlocker(c, err, unsafeToolPayload, safePayload))
	require.Equal(t, "original_payload_not_replayable", openAIWSHTTPFallbackBlocker(c, err, safePayload, unsafeToolPayload))

	_, writeErr := c.Writer.Write([]byte("started"))
	require.NoError(t, writeErr)
	require.Equal(t, "client_output_written", openAIWSHTTPFallbackBlocker(c, err, safePayload, safePayload))
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
	require.Equal(t, http.StatusUnauthorized, openAIWSErrorHTTPStatus([]byte(`{"type":"error","error":{"code":"token_invalidated","message":"token invalidated"}}`)))
	require.Equal(t, http.StatusUnauthorized, openAIWSErrorHTTPStatus([]byte(`{"type":"error","error":{"code":"token_revoked","message":"token revoked"}}`)))
	require.Equal(t, http.StatusPaymentRequired, openAIWSErrorHTTPStatus([]byte(`{"type":"error","error":{"code":"deactivated_workspace","message":"workspace deactivated"}}`)))
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
		require.Equal(t, "OpenAI websocket continuation expired; retry the request without previous_response_id.", clientMessage)
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

	t.Run("response_failed_usage_limit_is_sanitized", func(t *testing.T) {
		statusCode, errType, clientMessage, upstreamMessage, ok := resolveOpenAIWSFallbackErrorResponse(
			wrapOpenAIWSFallback("response_failed", &openAIWSResponseFailedError{
				code:      "billing_hard_limit",
				errType:   "usage_limit_reached",
				message:   "You've hit your usage limit. Upgrade to Plus to continue using Codex, or try again after 1:00 PM.",
				retryable: false,
			}),
		)
		require.True(t, ok)
		require.Equal(t, http.StatusTooManyRequests, statusCode)
		require.Equal(t, "rate_limit_error", errType)
		require.Equal(t, "Upstream billing or quota limit reached, please retry later", clientMessage)
		require.Equal(t, "You've hit your usage limit. Upgrade to Plus to continue using Codex, or try again after 1:00 PM.", upstreamMessage)
	})

	t.Run("session_preempted_not_client_visible", func(t *testing.T) {
		_, _, _, _, ok := resolveOpenAIWSFallbackErrorResponse(
			wrapOpenAIWSFallback("session_preempted", errOpenAIWSSessionPreempted),
		)
		require.False(t, ok)
	})

	t.Run("non_fallback_error_not_resolved", func(t *testing.T) {
		_, _, _, _, ok := resolveOpenAIWSFallbackErrorResponse(errors.New("plain error"))
		require.False(t, ok)
	})
}

func TestWriteOpenAIWSFallbackErrorResponse_SessionPreemptedIsSuppressed(t *testing.T) {
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

	// 会话抢占必须被完全静默：不向客户端写任何响应(避免 499 错误体)，
	// 也不设置 ops 上游错误键(避免被 ops 错误日志记录)。
	require.False(t, wrote)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
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
		require.Contains(t, string(failoverErr.ResponseBody), "Upstream billing or quota limit reached, please retry later")
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
	require.Equal(t, "normal_close", classifyOpenAIWSReadFallbackReason(coderws.CloseError{Code: coderws.StatusNormalClosure}))
	require.Equal(t, "read_event", classifyOpenAIWSReadFallbackReason(errors.New("io")))
}

func TestClassifyOpenAIWSErrorEventMessageTooBig(t *testing.T) {
	reason, canFallback := classifyOpenAIWSErrorEventFromRaw("message_too_big", "invalid_request_error", "frame rejected")
	require.Equal(t, "message_too_big", reason)
	require.True(t, canFallback)

	reason, canFallback = classifyOpenAIWSErrorEventFromRaw("server_error", "server_error", "request payload too large")
	require.Equal(t, "message_too_big", reason)
	require.True(t, canFallback)
}

func TestOpenAIWSNormalCloseIsRetryableWithNewConnection(t *testing.T) {
	reason, retryable := classifyOpenAIWSReconnectReason(wrapOpenAIWSFallback("normal_close", coderws.CloseError{Code: coderws.StatusNormalClosure}))
	require.Equal(t, "normal_close", reason)
	require.True(t, retryable)
	require.True(t, shouldForceNewConnOnHTTPIngressWSOneShotRetry(reason))
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

func TestShouldUseOpenAIHTTPIngressWSOneShotRespectsBodyStickySeeds(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.HttpIngressUpstreamWSEnabled = true
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	// Truly cold HTTP body → one-shot OK.
	require.True(t, svc.shouldUseOpenAIHTTPIngressWSOneShot(c, map[string]any{
		"model": "gpt-5.1",
		"input": []any{},
	}, "", ""))

	// Body prompt_cache_key should disable one-shot (multi-turn sticky possible).
	require.False(t, svc.shouldUseOpenAIHTTPIngressWSOneShot(c, map[string]any{
		"model":            "gpt-5.1",
		"prompt_cache_key": "sess-body",
		"input":            []any{},
	}, "", ""))

	// Client previous disables one-shot.
	require.False(t, svc.shouldUseOpenAIHTTPIngressWSOneShot(c, map[string]any{
		"model": "gpt-5.1",
		"input": []any{},
	}, "resp_prev", ""))
}

func TestOpenAIWSClientPreviousMatchesSessionContext(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	store.BindSessionContext(7, 11, "sess", openAIWSSessionContextValue{
		accountID:      101,
		connID:         "oa_ws_1",
		lastResponseID: "resp_a",
	}, time.Minute)

	require.True(t, openAIWSClientPreviousMatchesSessionContext(store, 7, 11, "sess", 101, ""))
	require.True(t, openAIWSClientPreviousMatchesSessionContext(store, 7, 11, "sess", 101, "resp_a"))
	require.False(t, openAIWSClientPreviousMatchesSessionContext(store, 7, 11, "sess", 101, "resp_other"))
	require.False(t, openAIWSClientPreviousMatchesSessionContext(store, 7, 11, "sess", 202, "resp_a"))
}

func TestOpenAIWSV2PassthroughFailoverStatus(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		payload   string
		code      string
		errType   string
		wantCode  int
		want      bool
	}{
		{name: "revoked token", eventType: "error", code: "token_revoked", wantCode: http.StatusUnauthorized, want: true},
		{name: "deactivated workspace", eventType: "response.failed", code: "deactivated_workspace", wantCode: http.StatusPaymentRequired, want: true},
		{name: "transient failed", eventType: "response.failed", payload: `{"type":"response.failed","response":{"error":{"code":"server_error","message":"temporary upstream failure"}}}`, code: "server_error", errType: "server_error", wantCode: http.StatusBadGateway, want: true},
		{name: "invalid request", eventType: "response.failed", payload: `{"type":"response.failed","response":{"error":{"code":"invalid_request_error","message":"bad input"}}}`, code: "invalid_request_error", errType: "invalid_request_error", wantCode: http.StatusBadRequest, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusCode, got := openAIWSV2PassthroughFailoverStatus(tt.eventType, []byte(tt.payload), tt.code, tt.errType, "temporary upstream failure")
			require.Equal(t, tt.wantCode, statusCode)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestShouldForceOpenAIWSPreferredConnEvenWhenLeased(t *testing.T) {
	account := &Account{Type: AccountTypeOAuth}
	snap := openAIWSConnAcquireSnapshot{
		Exists:         true,
		MatchesAcquire: true,
		Leased:         true,
		Waiters:        2,
	}
	require.True(t, shouldForceOpenAIWSPreferredConn(
		account, "oa_ws_1", snap, openAIWSConnProfileSessionBound, false, false,
	), "busy preferred conn must still be forced so acquire queues instead of dialing a new session_bound")
	require.False(t, shouldForceOpenAIWSPreferredConn(
		account, "oa_ws_1", snap, openAIWSConnProfileSessionBound, false, true,
	), "one-shot must not force preferred")
}

func TestOpenAIWSLastFailureAllowsDeltaConnReanchor(t *testing.T) {
	require.True(t, openAIWSLastFailureAllowsDeltaConnReanchor(""))
	require.True(t, openAIWSLastFailureAllowsDeltaConnReanchor("write_request"))
	require.True(t, openAIWSLastFailureAllowsDeltaConnReanchor("prewrite_ping_fail"))
	require.True(t, openAIWSLastFailureAllowsDeltaConnReanchor("preferred_conn_unavailable"))
	require.False(t, openAIWSLastFailureAllowsDeltaConnReanchor("previous_response_not_found"))
	require.False(t, openAIWSLastFailureAllowsDeltaConnReanchor("policy_violation"))
}

func TestOpenAIWSShouldSkipActiveDeltaForExpectedFull(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)

	skip, reason := openAIWSShouldSkipActiveDeltaForExpectedFull("previous_response_not_found", false, false, "sess", store)
	require.True(t, skip)
	require.Equal(t, "expected_full_after_prev_miss", reason)

	skip, reason = openAIWSShouldSkipActiveDeltaForExpectedFull("prewarm_previous_response_not_found", false, false, "sess", store)
	require.True(t, skip)
	require.Equal(t, "expected_full_after_prev_miss", reason)

	skip, reason = openAIWSShouldSkipActiveDeltaForExpectedFull("", true, false, "sess", store)
	require.True(t, skip)
	require.Equal(t, "expected_full_session_preempted", reason)

	skip, reason = openAIWSShouldSkipActiveDeltaForExpectedFull("", false, true, "sess", store)
	require.True(t, skip)
	require.Equal(t, "expected_full_http_one_shot", reason)

	skip, reason = openAIWSShouldSkipActiveDeltaForExpectedFull("", false, false, "", store)
	require.True(t, skip)
	require.Equal(t, "expected_full_missing_session_hash", reason)

	// Clean multi-turn attempt must still evaluate delta.
	skip, reason = openAIWSShouldSkipActiveDeltaForExpectedFull("", false, false, "sess", store)
	require.False(t, skip)
	require.Empty(t, reason)

	// Transport write failure is retriable with delta still allowed.
	skip, _ = openAIWSShouldSkipActiveDeltaForExpectedFull("write_request", false, false, "sess", store)
	require.False(t, skip)
}

func TestOpenAIWSClassifyExpectedFullDeltaReason(t *testing.T) {
	require.Equal(t, "expected_full_no_session_context",
		openAIWSClassifyExpectedFullDeltaReason("no_session_context", false, "", "missing_previous_response_id"))
	require.Equal(t, "expected_full_first_turn",
		openAIWSClassifyExpectedFullDeltaReason("not_candidate", false, "", "missing_previous_response_id"))
	require.Equal(t, "expected_full_session_oversized",
		openAIWSClassifyExpectedFullDeltaReason("session_context_oversized", true, "resp_x", ""))
	// True regressions keep original reason.
	require.Equal(t, "conn_mismatch",
		openAIWSClassifyExpectedFullDeltaReason("conn_mismatch", true, "resp_x", "active_delta"))
	require.Equal(t, "prefix_break_output",
		openAIWSClassifyExpectedFullDeltaReason("prefix_break_output", true, "", ""))
	require.Equal(t, "expected_full_http_one_shot",
		openAIWSClassifyExpectedFullDeltaReason("expected_full_http_one_shot", false, "", ""))
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

func TestShouldBypassOpenAIWSSessionConnForColdToolReplay(t *testing.T) {
	account := &Account{Type: AccountTypeOAuth}
	fullToolReplay := map[string]any{
		"store": false,
		"input": []any{
			map[string]any{"type": "function_call", "call_id": "call_1", "name": "shell", "arguments": "{}"},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok"},
			map[string]any{"type": "message", "role": "user", "content": "continue"},
		},
	}

	require.True(t, shouldBypassOpenAIWSSessionConnForColdToolReplay(
		account,
		false,
		true,
		"",
		"session-hash",
		"",
		"",
		fullToolReplay,
	), "full tool-context replay should use a neutral/fresh connection instead of a stale session-bound ctx_pool conn")

	legacyRoleToolReplay := map[string]any{
		"store": false,
		"input": []any{
			map[string]any{"type": "function_call", "call_id": "call_legacy", "name": "shell", "arguments": "{}"},
			map[string]any{"role": "tool", "tool_call_id": "call_legacy", "content": "ok"},
			map[string]any{"type": "message", "role": "user", "content": "continue"},
		},
	}
	require.True(t, shouldBypassOpenAIWSSessionConnForColdToolReplay(
		account,
		false,
		true,
		"",
		"session-hash",
		"",
		"",
		legacyRoleToolReplay,
	), "legacy role=tool full replay should not reuse a stale session-bound ctx_pool conn")

	require.False(t, shouldBypassOpenAIWSSessionConnForColdToolReplay(
		account,
		false,
		true,
		"resp_prev",
		"session-hash",
		"",
		"",
		fullToolReplay,
	), "explicit previous_response_id continuations still need their bound session connection")

	require.False(t, shouldBypassOpenAIWSSessionConnForColdToolReplay(
		account,
		false,
		true,
		"",
		"session-hash",
		"",
		"",
		map[string]any{
			"input": []any{
				map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok"},
			},
		},
	), "orphan tool output without replayed tool-call context cannot be treated as a cold full replay")
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
		AccountID:                 42,
		AccountType:               AccountTypeOAuth,
		ConnID:                    "oa_ws_42_1",
		PreviousResponseID:        "resp_prev_123",
		PreviousResponseIDKind:    OpenAIPreviousResponseIDKindResponseID,
		PreviousResponseIDSource:  "client",
		OriginalPreviousIDPresent: true,
		PreferredConnID:           "oa_ws_42_1",
		ConnReused:                true,
		StoreDisabled:             false,
		StoreMode:                 openAIWSStoreModeIncremental,
		StoreEnabled:              true,
		StickyAccountID:           42,
		StickyAccountHit:          true,
		StickyAccountMismatch:     false,
		ConnAffinityHit:           true,
		FallbackReason:            "",
		PayloadBytes:              3210,
		ConnPickMs:                3,
		QueueWaitMs:               0,
		ConnAgeMs:                 81000,
		ConnIdleMs:                79000,
		ConnLeaseCount:            4,
		SessionHash:               "abcdef1234567890",
		HeaderSessionID:           "session-header",
		HeaderConversationID:      "conversation-header",
		SessionIDSource:           "header",
		ConversationIDSource:      "header",
		HasTurnState:              true,
		TurnStateLen:              17,
		HasPromptCacheKey:         true,
	})

	require.Contains(t, msg, "continuation_probe")
	require.Contains(t, msg, "previous_response_id_source=client")
	require.Contains(t, msg, "original_previous_response_id_present=true")
	require.Contains(t, msg, "store_mode=incremental")
	require.Contains(t, msg, "store_enabled=true")
	require.Contains(t, msg, "sticky_account_id=42")
	require.Contains(t, msg, "sticky_account_hit=true")
	require.Contains(t, msg, "sticky_account_conflict=false")
	require.NotContains(t, msg, "sticky_account_mismatch")
	require.Contains(t, msg, "conn_affinity_hit=true")
	require.Contains(t, msg, "fallback_reason=-")
	require.Contains(t, msg, "payload_bytes=3210")
	require.Contains(t, msg, "conn_pick_ms=3")
	require.Contains(t, msg, "queue_wait_ms=0")
	require.Contains(t, msg, "conn_age_ms=81000")
	require.Contains(t, msg, "conn_idle_ms=79000")
	require.Contains(t, msg, "conn_lease_count=4")
}

func TestOpenAIWSReadFailLogMessageIncludesRequestAndConnectionContext(t *testing.T) {
	msg := openAIWSReadFailLogMessage(openAIWSReadFailLog{
		RequestID:                "req-read-fail",
		ClientRequestID:          "client-read-fail",
		AccountID:                44,
		AccountType:              AccountTypeOAuth,
		Model:                    "gpt-5.4",
		UpstreamModel:            "gpt-5.4-codex",
		ConnProfile:              openAIWSConnProfileSessionBound,
		ConnID:                   "oa_ws_44_2",
		ConnReused:               true,
		Transport:                string(OpenAIUpstreamTransportResponsesWebsocketV2),
		Attempt:                  2,
		ConnPickMs:               5,
		QueueWaitMs:              7,
		ConnAgeMs:                91000,
		ConnIdleMs:               88000,
		ConnLeaseCount:           2,
		PayloadBytes:             1234,
		PreviousResponseID:       "resp_prev_read_fail",
		PreviousResponseIDKind:   OpenAIPreviousResponseIDKindResponseID,
		PreviousResponseIDSource: "session_context",
		StoreMode:                openAIWSStoreModeIncremental,
		StoreEnabled:             true,
		StoreDisabled:            false,
		StickyAccountID:          44,
		StickyAccountHit:         true,
		ConnAffinityHit:          true,
		PreferredConnID:          "oa_ws_44_2",
		StoreFallbackReason:      "active_delta",
		SessionHash:              "abcdef1234567890",
		HasPromptCacheKey:        true,
		HasTurnState:             true,
		TurnStateLen:             22,
		ProxyEnabled:             true,
		ProxyID:                  57,
		WroteDownstream:          true,
		CloseStatus:              "1006",
		CloseReason:              "abnormal closure",
		Cause:                    "websocket: close 1006",
		Events:                   198,
		TokenEvents:              117,
		TerminalEvents:           0,
		FirstEventMs:             121,
		FirstTokenMs:             2048,
		FirstTokenEvent:          "response.output_text.delta",
		BufferedPending:          0,
		BufferedFlushed:          3,
		FirstEvent:               "response.created",
		LastEvent:                "response.output_text.delta",
	})

	require.Contains(t, msg, "read_fail")
	require.Contains(t, msg, "request_id=req-read-fail")
	require.Contains(t, msg, "client_request_id=client-read-fail")
	require.Contains(t, msg, "account_id=44")
	require.Contains(t, msg, "account_type=oauth")
	require.Contains(t, msg, "model=gpt-5.4")
	require.Contains(t, msg, "upstream_model=gpt-5.4-codex")
	require.Contains(t, msg, "conn_profile=session_bound")
	require.Contains(t, msg, "conn_id=oa_ws_44_2")
	require.Contains(t, msg, "conn_reused=true")
	require.Contains(t, msg, "transport=responses_websockets_v2")
	require.Contains(t, msg, "attempt=2")
	require.Contains(t, msg, "conn_pick_ms=5")
	require.Contains(t, msg, "queue_wait_ms=7")
	require.Contains(t, msg, "conn_age_ms=91000")
	require.Contains(t, msg, "conn_idle_ms=88000")
	require.Contains(t, msg, "conn_lease_count=2")
	require.Contains(t, msg, "payload_bytes=1234")
	require.Contains(t, msg, "previous_response_id=resp_prev_read_fail")
	require.Contains(t, msg, "previous_response_id_kind=response_id")
	require.Contains(t, msg, "previous_response_id_source=session_context")
	require.Contains(t, msg, "store_mode=incremental")
	require.Contains(t, msg, "store_enabled=true")
	require.Contains(t, msg, "store_disabled=false")
	require.Contains(t, msg, "sticky_account_id=44")
	require.Contains(t, msg, "sticky_account_hit=true")
	require.Contains(t, msg, "conn_affinity_hit=true")
	require.Contains(t, msg, "preferred_conn_id=oa_ws_44_2")
	require.Contains(t, msg, "store_fallback_reason=active_delta")
	require.Contains(t, msg, "session_hash=abcdef123456")
	require.Contains(t, msg, "has_prompt_cache_key=true")
	require.Contains(t, msg, "has_turn_state=true")
	require.Contains(t, msg, "turn_state_len=22")
	require.Contains(t, msg, "proxy_enabled=true")
	require.Contains(t, msg, "proxy_id=57")
	require.Contains(t, msg, "wrote_downstream=true")
	require.Contains(t, msg, "close_status=1006")
	require.Contains(t, msg, "close_reason=abnormal_closure")
	require.Contains(t, msg, "events=198")
	require.Contains(t, msg, "token_events=117")
	require.Contains(t, msg, "terminal_events=0")
	require.Contains(t, msg, "first_event_ms=121")
	require.Contains(t, msg, "first_token_ms=2048")
	require.Contains(t, msg, "first_token_event=response.output_text.delta")
	require.Contains(t, msg, "last_event=response.output_text.delta")
}

func TestOpenAIWSErrorEventLogMessageIncludesRequestBindingAndUpstreamFields(t *testing.T) {
	msg := openAIWSErrorEventLogMessage(openAIWSErrorEventLog{
		RequestID:                 "req-upstream-event",
		ClientRequestID:           "client-upstream-event",
		AccountID:                 46,
		AccountType:               AccountTypeOAuth,
		ConnProfile:               openAIWSConnProfileSessionBound,
		ConnID:                    "oa_ws_46_4",
		ConnReused:                true,
		Transport:                 string(OpenAIUpstreamTransportResponsesWebsocketV2),
		Attempt:                   1,
		ConnAgeMs:                 55000,
		ConnIdleMs:                77000,
		ConnLeaseCount:            9,
		PayloadBytes:              7654,
		ResponseID:                "resp_event_context",
		PreviousResponseID:        "resp_prev_upstream_event",
		PreviousResponseIDKind:    OpenAIPreviousResponseIDKindResponseID,
		PreviousResponseIDSource:  "session_context",
		OriginalPreviousIDPresent: false,
		StoreMode:                 openAIWSStoreModeIncremental,
		StoreDisabled:             true,
		StickyAccountID:           46,
		StickyAccountHit:          true,
		ConnAffinityHit:           true,
		PreferredConnID:           "oa_ws_46_4",
		StoreFallbackReason:       "active_delta",
		CleanupReason:             "error_event",
		ActiveDelta:               true,
		DeltaItems:                2,
		FullItems:                 99,
		EventIndex:                4,
		FallbackReason:            "previous_response_not_found",
		CanFallback:               true,
		ErrorCode:                 "previous_response_not_found",
		ErrorType:                 "invalid_request_error",
		ErrorMessage:              "previous response not found",
		SessionHash:               "abcdef1234567890",
		HeaderSessionID:           "session-header",
		HeaderConversationID:      "conversation-header",
		SessionIDSource:           "prompt_cache_key",
		ConversationIDSource:      "header",
		HTTPIngressWSOneShot:      false,
		HasPromptCacheKey:         true,
		HasTurnState:              false,
		TurnStateLen:              0,
	})

	require.Contains(t, msg, "error_event")
	require.Contains(t, msg, "request_id=req-upstream-event")
	require.Contains(t, msg, "client_request_id=client-upstream-event")
	require.Contains(t, msg, "account_id=46")
	require.Contains(t, msg, "conn_profile=session_bound")
	require.Contains(t, msg, "conn_reused=true")
	require.Contains(t, msg, "response_id=resp_event_context")
	require.Contains(t, msg, "previous_response_id=resp_prev_upstream_event")
	require.Contains(t, msg, "previous_response_id_source=session_context")
	require.Contains(t, msg, "original_previous_response_id_present=false")
	require.Contains(t, msg, "sticky_account_id=46")
	require.Contains(t, msg, "sticky_account_hit=true")
	require.Contains(t, msg, "conn_affinity_hit=true")
	require.Contains(t, msg, "preferred_conn_id=oa_ws_46_4")
	require.Contains(t, msg, "store_fallback_reason=active_delta")
	require.Contains(t, msg, "cleanup_reason=error_event")
	require.Contains(t, msg, "active_delta=true")
	require.Contains(t, msg, "fallback_reason=previous_response_not_found")
	require.Contains(t, msg, "can_fallback=true")
	require.Contains(t, msg, "err_code=previous_response_not_found")
	require.Contains(t, msg, "session_hash=abcdef123456")
	require.Contains(t, msg, "header_session_id=session-header")
	require.Contains(t, msg, "header_conversation_id=conversation-header")
	require.Contains(t, msg, "session_id_source=prompt_cache_key")
	require.Contains(t, msg, "conversation_id_source=header")
	require.Contains(t, msg, "http_ingress_ws_one_shot=false")
}

func TestClassifyOpenAIWSErrorEventFromRawRecognizesMismatchForFallback(t *testing.T) {
	reason, canFallback := classifyOpenAIWSErrorEventFromRaw("conn_mismatch", "invalid_request_error", "connection context mismatch")
	require.Equal(t, "conn_mismatch", reason)
	require.True(t, canFallback)

	reason, canFallback = classifyOpenAIWSErrorEventFromRaw("account_mismatch", "invalid_request_error", "account context mismatch")
	require.Equal(t, "account_mismatch", reason)
	require.True(t, canFallback)
}

func TestOpenAIWSWriteRequestFailLogMessageIncludesRequestAndReanchorContext(t *testing.T) {
	msg := openAIWSWriteRequestFailLogMessage(openAIWSWriteRequestFailLog{
		RequestID:                "req-write-fail",
		ClientRequestID:          "client-write-fail",
		AccountID:                45,
		AccountType:              AccountTypeOAuth,
		Model:                    "gpt-5.5",
		UpstreamModel:            "gpt-5.5",
		ConnProfile:              openAIWSConnProfileSessionBound,
		ConnID:                   "oa_ws_45_3",
		ConnReused:               true,
		Transport:                string(OpenAIUpstreamTransportResponsesWebsocketV2),
		Attempt:                  1,
		ConnPickMs:               4,
		QueueWaitMs:              6,
		ConnAgeMs:                120000,
		ConnIdleMs:               87000,
		ConnLeaseCount:           3,
		SessionIdleTTLMS:         80000,
		PrewritePingThresholdMS:  60000,
		PrewritePingAttempted:    true,
		PrewritePingResult:       "ok",
		PayloadBytes:             4321,
		PreviousResponseID:       "resp_prev_write_fail",
		PreviousResponseIDKind:   OpenAIPreviousResponseIDKindResponseID,
		PreviousResponseIDSource: "session_context",
		StoreMode:                openAIWSStoreModeIncremental,
		StoreEnabled:             false,
		StoreDisabled:            true,
		StickyAccountID:          45,
		StickyAccountHit:         true,
		ConnAffinityHit:          true,
		PreferredConnID:          "oa_ws_45_3",
		StoreFallbackReason:      "active_delta",
		CleanupReason:            "write_request_fail_no_reanchor",
		AllowDeltaConnReanchor:   false,
		ConnReanchorBlockers:     "has_function_call_output",
		SessionHash:              "abcdef1234567890",
		HasPromptCacheKey:        true,
		HasTurnState:             false,
		TurnStateLen:             0,
		HTTPIngressWSOneShot:     false,
		ForceNewConn:             false,
		AffinityOnlyReuse:        false,
		HasFunctionCallOutput:    true,
		ActiveDelta:              true,
		DeltaItems:               1,
		DeltaBytes:               799,
		FullItems:                305,
		FullBytes:                834681,
		ProxyEnabled:             true,
		ProxyID:                  58,
		Cause:                    "fail to write frame: broken pipe",
	})

	require.Contains(t, msg, "write_request_fail")
	require.Contains(t, msg, "request_id=req-write-fail")
	require.Contains(t, msg, "client_request_id=client-write-fail")
	require.Contains(t, msg, "account_id=45")
	require.Contains(t, msg, "account_type=oauth")
	require.Contains(t, msg, "model=gpt-5.5")
	require.Contains(t, msg, "conn_profile=session_bound")
	require.Contains(t, msg, "conn_id=oa_ws_45_3")
	require.Contains(t, msg, "conn_reused=true")
	require.Contains(t, msg, "transport=responses_websockets_v2")
	require.Contains(t, msg, "attempt=1")
	require.Contains(t, msg, "conn_pick_ms=4")
	require.Contains(t, msg, "queue_wait_ms=6")
	require.Contains(t, msg, "conn_age_ms=120000")
	require.Contains(t, msg, "conn_idle_ms=87000")
	require.Contains(t, msg, "conn_lease_count=3")
	require.Contains(t, msg, "session_idle_ttl_ms=80000")
	require.Contains(t, msg, "prewrite_ping_threshold_ms=60000")
	require.Contains(t, msg, "prewrite_ping_attempted=true")
	require.Contains(t, msg, "prewrite_ping_result=ok")
	require.Contains(t, msg, "payload_bytes=4321")
	require.Contains(t, msg, "previous_response_id=resp_prev_write_fail")
	require.Contains(t, msg, "previous_response_id_source=session_context")
	require.Contains(t, msg, "store_mode=incremental")
	require.Contains(t, msg, "sticky_account_id=45")
	require.Contains(t, msg, "sticky_account_hit=true")
	require.Contains(t, msg, "conn_affinity_hit=true")
	require.Contains(t, msg, "preferred_conn_id=oa_ws_45_3")
	require.Contains(t, msg, "store_fallback_reason=active_delta")
	require.Contains(t, msg, "cleanup_reason=write_request_fail_no_reanchor")
	require.Contains(t, msg, "allow_delta_conn_reanchor=false")
	require.Contains(t, msg, "conn_reanchor_blockers=has_function_call_output")
	require.Contains(t, msg, "session_hash=abcdef123456")
	require.Contains(t, msg, "has_prompt_cache_key=true")
	require.Contains(t, msg, "http_ingress_ws_one_shot=false")
	require.Contains(t, msg, "force_new_conn=false")
	require.Contains(t, msg, "has_function_call_output=true")
	require.Contains(t, msg, "active_delta=true")
	require.Contains(t, msg, "delta_items=1")
	require.Contains(t, msg, "delta_bytes=799")
	require.Contains(t, msg, "full_items=305")
	require.Contains(t, msg, "full_bytes=834681")
	require.Contains(t, msg, "proxy_enabled=true")
	require.Contains(t, msg, "proxy_id=58")
	require.Contains(t, msg, "cause=fail_to_write_frame:_broken_pipe")
}

func TestOpenAIWSSessionPrewritePingIdleThresholdUsesSessionTTL(t *testing.T) {
	// Clamped to [20s, 30s]: production data needed earlier dead-conn detection.
	require.Equal(t, 20*time.Second, openAIWSSessionPrewritePingIdleThreshold(0))
	require.Equal(t, 20*time.Second, openAIWSSessionPrewritePingIdleThreshold(60*time.Second))  // 6s → min 20s
	require.Equal(t, 30*time.Second, openAIWSSessionPrewritePingIdleThreshold(600*time.Second)) // 60s → max 30s
	require.Equal(t, 30*time.Second, openAIWSSessionPrewritePingIdleThreshold(1000*time.Second))
	require.Equal(t, 25*time.Second, openAIWSSessionPrewritePingIdleThreshold(250*time.Second)) // 25s in band
}

func TestShouldOpenAIWSSessionPrewritePingRequiresIdlePingCapability(t *testing.T) {
	conn := newOpenAIWSConn("coder-no-idle-reader", 1, &coderOpenAIWSClientConn{}, nil)
	conn.lastUsedNano.Store(time.Now().Add(-2 * time.Minute).UnixNano())
	lease := &openAIWSConnLease{conn: conn, reused: true}
	account := &Account{Type: AccountTypeOAuth}

	// coder conn does not support idle ping without a reader → still skip.
	require.False(t, shouldOpenAIWSSessionPrewritePing(
		account,
		openAIWSConnProfileSessionBound,
		lease,
		false,
		time.Minute,
	))
}

// idlePingCapableFakeConn is an openAIWSClientConn that also opts into idle ping.
type idlePingCapableFakeConn struct {
	openAIWSFakeConn
}

func (*idlePingCapableFakeConn) SupportsIdlePingWithoutReader() bool { return true }

func TestShouldOpenAIWSSessionPrewritePingCoversNeutralAndShortIdle(t *testing.T) {
	conn := newOpenAIWSConnWithProfile("oa_ws_neutral_ping", &idlePingCapableFakeConn{}, nil, openAIWSConnProfileNeutral)
	conn.lastUsedNano.Store(time.Now().Add(-25 * time.Second).UnixNano())
	lease := &openAIWSConnLease{conn: conn, reused: true}
	account := &Account{Type: AccountTypeOAuth}

	require.True(t, shouldOpenAIWSSessionPrewritePing(
		account,
		openAIWSConnProfileNeutral,
		lease,
		false,
		20*time.Second,
	), "neutral reused conns idle past threshold must prewrite-ping")

	require.False(t, shouldOpenAIWSSessionPrewritePing(
		account,
		openAIWSConnProfileNeutral,
		lease,
		true, // one-shot
		20*time.Second,
	), "http one-shot skips prewrite ping")
}

func TestOpenAIWSConnEvictReasonPrewritePingDropsSessionContext(t *testing.T) {
	require.True(t, openAIWSConnEvictReasonInvalidatesSessionContext("prewrite_ping_fail"))
	require.True(t, openAIWSConnEvictReasonDeletesDurableSessionContext("prewrite_ping_fail"))
	require.True(t, openAIWSConnEvictReasonInvalidatesSessionContext("background_ping_fail"))
	require.True(t, openAIWSConnEvictReasonDeletesDurableSessionContext("keepalive_ping_timeout"))
}

func TestOpenAIWSPrewritePingLogMessageIncludesReuseRisk(t *testing.T) {
	msg := openAIWSPrewritePingLogMessage(openAIWSPrewritePingLog{
		RequestID:                "req-ping",
		ClientRequestID:          "client-ping",
		AccountID:                45,
		AccountType:              AccountTypeOAuth,
		ConnProfile:              openAIWSConnProfileSessionBound,
		ConnID:                   "oa_ws_45_3",
		ConnReused:               true,
		ConnAgeMS:                120000,
		ConnIdleMS:               73000,
		ConnLeaseCount:           4,
		SessionIdleTTLMS:         80000,
		PrewritePingThresholdMS:  60000,
		PreviousResponseID:       "resp_prev_ping",
		PreviousResponseIDSource: "session_context",
		StoreMode:                openAIWSStoreModeIncremental,
		StoreFallbackReason:      "active_delta",
		ActiveDelta:              true,
		HasFunctionCallOutput:    true,
		SessionHash:              "abcdef1234567890",
		StickyAccountID:          45,
		StickyAccountHit:         true,
		ConnAffinityHit:          true,
		Result:                   "fail",
		Cause:                    "websocket closed",
	})

	require.Contains(t, msg, "prewrite_ping")
	require.Contains(t, msg, "request_id=req-ping")
	require.Contains(t, msg, "client_request_id=client-ping")
	require.Contains(t, msg, "account_id=45")
	require.Contains(t, msg, "conn_profile=session_bound")
	require.Contains(t, msg, "conn_id=oa_ws_45_3")
	require.Contains(t, msg, "conn_reused=true")
	require.Contains(t, msg, "conn_idle_ms=73000")
	require.Contains(t, msg, "session_idle_ttl_ms=80000")
	require.Contains(t, msg, "prewrite_ping_threshold_ms=60000")
	require.Contains(t, msg, "previous_response_id=resp_prev_ping")
	require.Contains(t, msg, "previous_response_id_source=session_context")
	require.Contains(t, msg, "store_mode=incremental")
	require.Contains(t, msg, "store_fallback_reason=active_delta")
	require.Contains(t, msg, "active_delta=true")
	require.Contains(t, msg, "has_function_call_output=true")
	require.Contains(t, msg, "sticky_account_id=45")
	require.Contains(t, msg, "sticky_account_hit=true")
	require.Contains(t, msg, "conn_affinity_hit=true")
	require.Contains(t, msg, "result=fail")
	require.Contains(t, msg, "cause=websocket_closed")
}

func TestOpenAIWSTransportPathFromStartClassifiesReuseAndDelta(t *testing.T) {
	require.Equal(t, "session_bound_ws_reused_incremental_delta", openAIWSTransportPathFromStart(openAIWSDiagnosticStartLog{
		ConnProfile: openAIWSConnProfileSessionBound,
		ConnReused:  true,
		DeltaActive: true,
	}))
	require.Equal(t, "neutral_ws_new_full_or_non_delta", openAIWSTransportPathFromStart(openAIWSDiagnosticStartLog{
		ConnProfile: openAIWSConnProfileNeutral,
		ConnReused:  false,
		DeltaActive: false,
	}))
}
