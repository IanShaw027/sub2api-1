package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type captureOpsUpstreamFailureSink struct {
	failures []OpsUpstreamFailure
}

func (s *captureOpsUpstreamFailureSink) EnqueueOpsUpstreamFailure(_ context.Context, failure OpsUpstreamFailure) {
	s.failures = append(s.failures, failure)
}

func TestBuildOpsUpstreamFailureEntry_TransportTimeout(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxkey.RequestID, "req-timeout")
	ctx = context.WithValue(ctx, ctxkey.ClientRequestID, "client-timeout")

	entry := buildOpsUpstreamFailureEntry(ctx, OpsUpstreamFailure{
		AccountID: 42,
		Method:    http.MethodPost,
		URL:       "https://chatgpt.com/backend-api/codex/responses?token=secret",
		Kind:      "http_attempt",
		Err:       context.DeadlineExceeded,
	})

	require.NotNil(t, entry)
	require.Equal(t, "req-timeout", entry.RequestID)
	require.Equal(t, "client-timeout", entry.ClientRequestID)
	require.Equal(t, PlatformOpenAI, entry.Platform)
	require.NotNil(t, entry.AccountID)
	require.Equal(t, int64(42), *entry.AccountID)
	require.Equal(t, "upstream_timeout_error", entry.ErrorType)
	require.Equal(t, http.StatusBadGateway, entry.StatusCode)
	require.Nil(t, entry.UpstreamStatusCode)
	require.Len(t, entry.UpstreamErrors, 1)
	require.Equal(t, "https://chatgpt.com/backend-api/codex/responses", entry.UpstreamErrors[0].UpstreamURL)
	require.NotContains(t, entry.UpstreamErrors[0].UpstreamURL, "secret")
}

func TestBuildOpsUpstreamFailureEntry_HTTPRateLimit(t *testing.T) {
	entry := buildOpsUpstreamFailureEntry(context.Background(), OpsUpstreamFailure{
		AccountID:    77,
		Method:       http.MethodPost,
		URL:          "https://api.x.ai/v1/responses",
		Kind:         "http_attempt",
		StatusCode:   http.StatusTooManyRequests,
		ResponseBody: `{"error":{"message":"slow down"}}`,
	})

	require.NotNil(t, entry)
	require.Equal(t, PlatformGrok, entry.Platform)
	require.Equal(t, "rate_limit_error", entry.ErrorType)
	require.Equal(t, http.StatusTooManyRequests, entry.StatusCode)
	require.NotNil(t, entry.UpstreamStatusCode)
	require.Equal(t, http.StatusTooManyRequests, *entry.UpstreamStatusCode)
	require.True(t, entry.IsRetryable)
	require.JSONEq(t, `{"error":{"message":"slow down"}}`, entry.ErrorBody)
	require.JSONEq(t, `{"error":{"message":"slow down"}}`, entry.UpstreamErrors[0].UpstreamResponseBody)
}

func TestBuildOpsUpstreamFailureEntry_IgnoresSuccess(t *testing.T) {
	require.Nil(t, buildOpsUpstreamFailureEntry(context.Background(), OpsUpstreamFailure{
		StatusCode: http.StatusOK,
	}))
}

func TestInferOpsPlatformFromUpstreamURL(t *testing.T) {
	tests := map[string]string{
		"https://chatgpt.com/backend-api/codex/responses":                PlatformOpenAI,
		"https://api.x.ai/v1/responses":                                  PlatformGrok,
		"https://generativelanguage.googleapis.com/v1beta/models":        PlatformGemini,
		"https://cloudcode-pa.googleapis.com/v1internal:generateContent": PlatformAntigravity,
		"https://q.us-east-1.amazonaws.com/generateAssistantResponse":    PlatformKiro,
		"https://bedrock-runtime.us-east-1.amazonaws.com/model/a/invoke": PlatformAnthropic,
		"https://api.anthropic.com/v1/messages":                          PlatformAnthropic,
	}
	for rawURL, want := range tests {
		require.Equal(t, want, inferOpsPlatformFromUpstreamURL(rawURL), rawURL)
	}
}

func TestOpenAIWSPoolReportsBackgroundFailure(t *testing.T) {
	sink := &captureOpsUpstreamFailureSink{}
	pool := &openAIWSConnPool{}
	pool.setOpsUpstreamFailureSink(sink)
	wantErr := errors.New("context deadline exceeded")

	pool.reportUpstreamFailure(context.Background(), OpsUpstreamFailure{
		Platform:  PlatformOpenAI,
		AccountID: 76182,
		Method:    "WS PING",
		Kind:      "background_ping",
		Err:       wantErr,
	})

	require.Len(t, sink.failures, 1)
	require.Equal(t, int64(76182), sink.failures[0].AccountID)
	require.Equal(t, "background_ping", sink.failures[0].Kind)
	require.ErrorIs(t, sink.failures[0].Err, wantErr)
}

func TestOpenAIServiceReportsEachFailedWSAttempt(t *testing.T) {
	sink := &captureOpsUpstreamFailureSink{}
	svc := &OpenAIGatewayService{}
	svc.SetOpsUpstreamFailureSink(sink)

	svc.reportOpenAIWSAttemptFailure(context.Background(), &Account{
		ID:   76182,
		Type: AccountTypeOAuth,
	}, wrapOpenAIWSFallback("response_failed", errors.New("overloaded")))

	require.Len(t, sink.failures, 1)
	require.Equal(t, PlatformOpenAI, sink.failures[0].Platform)
	require.Equal(t, int64(76182), sink.failures[0].AccountID)
	require.Equal(t, "ws_attempt_response_failed", sink.failures[0].Kind)
	require.Error(t, sink.failures[0].Err)
}

func TestOpenAIServiceDoesNotReportSessionPreemptionAsUpstreamFailure(t *testing.T) {
	sink := &captureOpsUpstreamFailureSink{}
	svc := &OpenAIGatewayService{}
	svc.SetOpsUpstreamFailureSink(sink)

	svc.reportOpenAIWSAttemptFailure(context.Background(), &Account{
		ID:   76182,
		Type: AccountTypeOAuth,
	}, wrapOpenAIWSFallback("session_preempted", errOpenAIWSSessionPreempted))

	require.Empty(t, sink.failures)
}

func TestSanitizeOpsUpstreamErrorsRedactsCapturedResponseBody(t *testing.T) {
	entry := &OpsInsertErrorLogInput{
		UpstreamErrors: []*OpsUpstreamErrorEvent{{
			UpstreamStatusCode:   http.StatusUnauthorized,
			Message:              "Unauthorized",
			UpstreamResponseBody: `{"error":"unauthorized","access_token":"secret-token"}`,
		}},
	}

	require.NoError(t, sanitizeOpsUpstreamErrors(entry))
	require.Nil(t, entry.UpstreamErrors)
	require.NotEmpty(t, entry.UpstreamErrorsJSON)

	var events []*OpsUpstreamErrorEvent
	require.NoError(t, json.Unmarshal([]byte(*entry.UpstreamErrorsJSON), &events))
	require.Len(t, events, 1)
	require.NotContains(t, events[0].UpstreamResponseBody, "secret-token")
	require.Contains(t, events[0].UpstreamResponseBody, "[REDACTED]")
}

func TestOpsServicePersistsQueuedUpstreamFailure(t *testing.T) {
	inserted := make(chan *OpsInsertErrorLogInput, 1)
	svc := NewOpsService(&opsRepoMock{
		InsertErrorLogFn: func(_ context.Context, input *OpsInsertErrorLogInput) (int64, error) {
			inserted <- input
			return 1, nil
		},
	}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx := context.WithValue(context.Background(), ctxkey.RequestID, "req-queued")

	svc.EnqueueOpsUpstreamFailure(ctx, OpsUpstreamFailure{
		Platform:   PlatformOpenAI,
		AccountID:  42,
		Method:     http.MethodPost,
		URL:        "https://api.openai.com/v1/responses",
		Kind:       "http_attempt",
		StatusCode: http.StatusBadGateway,
	})

	select {
	case entry := <-inserted:
		require.Equal(t, "req-queued", entry.RequestID)
		require.Equal(t, "upstream", entry.ErrorPhase)
		require.NotNil(t, entry.AccountID)
		require.Equal(t, int64(42), *entry.AccountID)
	case <-time.After(2 * time.Second):
		t.Fatal("queued upstream failure was not persisted")
	}
}
