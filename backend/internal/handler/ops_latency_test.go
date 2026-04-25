package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecordOpsForwardLatencies_UsesUpstreamLatencyAndTTFT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	service.SetOpsLatencyMs(c, service.OpsUpstreamLatencyMsKey, 80)
	firstTokenMs := 45

	recordOpsForwardLatencies(c, 140, &service.ForwardResult{
		FirstTokenMs: &firstTokenMs,
	}, nil)

	responseLatencyMs, ok := getContextInt64(c, service.OpsResponseLatencyMsKey)
	require.True(t, ok)
	require.EqualValues(t, 60, responseLatencyMs)

	ttftMs, ok := getContextInt64(c, service.OpsTimeToFirstTokenMsKey)
	require.True(t, ok)
	require.EqualValues(t, firstTokenMs, ttftMs)
}

func TestRecordOpsForwardLatencies_ErrorSkipsTTFT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	service.SetOpsLatencyMs(c, service.OpsUpstreamLatencyMsKey, 200)
	firstTokenMs := 30

	recordOpsForwardLatencies(c, 140, &service.ForwardResult{
		FirstTokenMs: &firstTokenMs,
	}, errors.New("boom"))

	responseLatencyMs, ok := getContextInt64(c, service.OpsResponseLatencyMsKey)
	require.True(t, ok)
	require.EqualValues(t, 140, responseLatencyMs)

	_, ok = getContextInt64(c, service.OpsTimeToFirstTokenMsKey)
	require.False(t, ok)
}

func TestRecordOpsRoutingLatency_UsesAttemptStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	firstAttemptStart := time.Now().Add(-120 * time.Millisecond)
	recordOpsRoutingLatency(c, firstAttemptStart)
	firstRoutingMs, ok := getContextInt64(c, service.OpsRoutingLatencyMsKey)
	require.True(t, ok)
	require.GreaterOrEqual(t, firstRoutingMs, int64(80))

	secondAttemptStart := time.Now().Add(-20 * time.Millisecond)
	recordOpsRoutingLatency(c, secondAttemptStart)
	secondRoutingMs, ok := getContextInt64(c, service.OpsRoutingLatencyMsKey)
	require.True(t, ok)
	require.Less(t, secondRoutingMs, firstRoutingMs)
	require.Less(t, secondRoutingMs, int64(80))
}

func TestRecordOpsForwardLatencies_MultipleAttemptsUseLatestForwardDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	// First (failed) attempt
	service.SetOpsLatencyMs(c, service.OpsUpstreamLatencyMsKey, 30)
	recordOpsForwardLatencies(c, 200, nil, errors.New("failover"))

	// Final attempt should overwrite with its own forward latency math.
	service.SetOpsLatencyMs(c, service.OpsUpstreamLatencyMsKey, 90)
	firstTokenMs := 55
	recordOpsForwardLatencies(c, 130, &service.ForwardResult{
		FirstTokenMs: &firstTokenMs,
	}, nil)

	responseLatencyMs, ok := getContextInt64(c, service.OpsResponseLatencyMsKey)
	require.True(t, ok)
	require.EqualValues(t, 40, responseLatencyMs)

	ttftMs, ok := getContextInt64(c, service.OpsTimeToFirstTokenMsKey)
	require.True(t, ok)
	require.EqualValues(t, firstTokenMs, ttftMs)
}
