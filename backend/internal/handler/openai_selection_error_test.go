package handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestClassifySelectionFailureError_RateLimitedPool(t *testing.T) {
	fallback := noAccountErrorClassification{Status: http.StatusServiceUnavailable, ErrType: "api_error", Message: "Service temporarily unavailable"}
	got := classifySelectionFailureError(
		fmt.Errorf("no available accounts supporting model: gpt-5.6-sol (total=3 eligible=0 model_rate_limited=3)"),
		fallback,
	)
	require.Equal(t, http.StatusTooManyRequests, got.Status)
	require.Equal(t, "rate_limit_error", got.ErrType)
	require.Contains(t, got.Message, "rate-limited")
}

func TestClassifySelectionFailureError_PrefersPositiveRateLimitedAfterZeroModelCount(t *testing.T) {
	fallback := noAccountErrorClassification{Status: http.StatusServiceUnavailable, ErrType: "api_error", Message: "Service temporarily unavailable"}
	got := classifySelectionFailureError(
		fmt.Errorf("no available accounts (model_rate_limited=0 rate_limited=3)"),
		fallback,
	)
	require.Equal(t, http.StatusTooManyRequests, got.Status)
	require.Equal(t, "rate_limit_error", got.ErrType)
}

func TestClassifySelectionFailureError_ZeroCountersKeepFallback(t *testing.T) {
	fallback := noAccountErrorClassification{Status: http.StatusServiceUnavailable, ErrType: "api_error", Message: "Service temporarily unavailable"}
	require.Equal(t, fallback, classifySelectionFailureError(
		fmt.Errorf("no available accounts (model_rate_limited=0 rate_limited=0)"),
		fallback,
	))
}

func TestClassifySelectionFailureError_UnknownKeepsFallback(t *testing.T) {
	fallback := noAccountErrorClassification{Status: http.StatusServiceUnavailable, ErrType: "api_error", Message: "Service temporarily unavailable"}
	require.Equal(t, fallback, classifySelectionFailureError(errors.New("no available accounts"), fallback))
}

func TestBuildOpenAISelectionFailureMessage_StripsInternalCause(t *testing.T) {
	err := fmt.Errorf("no available OpenAI accounts supporting model: foo: no available accounts")
	got := buildOpenAISelectionFailureMessage(err, "Service temporarily unavailable")
	require.Equal(t, "No available accounts supporting model: foo", got)
}

func TestBuildOpenAISelectionFailureMessage_NoAvailableAccounts(t *testing.T) {
	got := buildOpenAISelectionFailureMessage(service.ErrNoAvailableAccounts, "Service temporarily unavailable")
	require.Equal(t, "No available accounts", got)
}

func TestBuildOpenAISelectionFailureMessage_NilUsesFallback(t *testing.T) {
	got := buildOpenAISelectionFailureMessage(nil, "Service temporarily unavailable")
	require.Equal(t, "Service temporarily unavailable", got)
}

func TestBuildOpenAISelectionFailureMessage_CompactUsesFallback(t *testing.T) {
	got := buildOpenAISelectionFailureMessage(service.ErrNoAvailableCompactAccounts, "No available accounts support /responses/compact")
	require.Equal(t, "No available accounts support /responses/compact", got)
}

func TestBuildOpenAISelectionFailureMessage_UnknownUsesFallback(t *testing.T) {
	got := buildOpenAISelectionFailureMessage(errors.New("scheduler exploded"), "Service temporarily unavailable")
	require.Equal(t, "Service temporarily unavailable", got)
}

func TestBuildOpenAISelectionExhaustedMessage_LocalExclusionsKeepFallback(t *testing.T) {
	err := fmt.Errorf("no available accounts supporting model: foo")
	got := buildOpenAISelectionExhaustedMessage(err, "Service temporarily unavailable", true)
	require.Equal(t, "Service temporarily unavailable", got)
}
