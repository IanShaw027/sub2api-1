package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsLoggerSettingRepoStub struct {
	values map[string]string
}

func (s *opsLoggerSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}
func (s *opsLoggerSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}
func (s *opsLoggerSettingRepoStub) Set(context.Context, string, string) error { return nil }
func (s *opsLoggerSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (s *opsLoggerSettingRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }
func (s *opsLoggerSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (s *opsLoggerSettingRepoStub) Delete(context.Context, string) error { return nil }

func resetOpsErrorLoggerStateForTest(t *testing.T) {
	t.Helper()

	opsErrorLogMu.Lock()
	ch := opsErrorLogQueue
	opsErrorLogQueue = nil
	opsErrorLogStopping = true
	opsErrorLogMu.Unlock()

	if ch != nil {
		close(ch)
	}
	opsErrorLogWorkersWg.Wait()

	opsErrorLogOnce = sync.Once{}
	opsErrorLogStopOnce = sync.Once{}
	opsErrorLogWorkersWg = sync.WaitGroup{}
	opsErrorLogMu = sync.RWMutex{}
	opsErrorLogStopping = false

	opsErrorLogQueueLen.Store(0)
	opsErrorLogEnqueued.Store(0)
	opsErrorLogDropped.Store(0)
	opsErrorLogProcessed.Store(0)
	opsErrorLogSanitized.Store(0)
	opsErrorLogLastDropLogAt.Store(0)

	opsErrorLogShutdownCh = make(chan struct{})
	opsErrorLogShutdownOnce = sync.Once{}
	opsErrorLogDrained.Store(false)
}

func TestAttachOpsRequestBodyToEntry_SanitizeAndTrim(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	raw := []byte(`{"access_token":"secret-token","messages":[{"role":"user","content":"hello"}]}`)
	setOpsRequestContext(c, "claude-3", false, raw)

	entry := &service.OpsInsertErrorLogInput{}
	attachOpsRequestBodyToEntry(c, entry)

	require.NotNil(t, entry.RequestBodyBytes)
	require.Equal(t, len(raw), *entry.RequestBodyBytes)
	require.NotNil(t, entry.RequestBodyJSON)
	require.NotContains(t, *entry.RequestBodyJSON, "secret-token")
	require.Contains(t, *entry.RequestBodyJSON, "[REDACTED]")
	require.Equal(t, int64(1), OpsErrorLogSanitizedTotal())
}

func TestAttachOpsRequestBodyToEntry_InvalidJSONKeepsSize(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	raw := []byte("not-json")
	setOpsRequestContext(c, "claude-3", false, raw)

	entry := &service.OpsInsertErrorLogInput{}
	attachOpsRequestBodyToEntry(c, entry)

	require.Nil(t, entry.RequestBodyJSON)
	require.NotNil(t, entry.RequestBodyBytes)
	require.Equal(t, len(raw), *entry.RequestBodyBytes)
	require.False(t, entry.RequestBodyTruncated)
	require.Equal(t, int64(1), OpsErrorLogSanitizedTotal())
}

func TestEnqueueOpsErrorLog_QueueFullDrop(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)

	// 禁止 enqueueOpsErrorLog 触发 workers，使用测试队列验证满队列降级。
	opsErrorLogOnce.Do(func() {})

	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	entry := &service.OpsInsertErrorLogInput{ErrorPhase: "upstream", ErrorType: "upstream_error"}

	enqueueOpsErrorLog(ops, entry)
	enqueueOpsErrorLog(ops, entry)

	require.Equal(t, int64(1), OpsErrorLogEnqueuedTotal())
	require.Equal(t, int64(1), OpsErrorLogDroppedTotal())
	require.Equal(t, int64(1), OpsErrorLogQueueLength())
}

func TestAttachOpsRequestBodyToEntry_EarlyReturnBranches(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	gin.SetMode(gin.TestMode)

	entry := &service.OpsInsertErrorLogInput{}
	attachOpsRequestBodyToEntry(nil, entry)
	attachOpsRequestBodyToEntry(&gin.Context{}, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	// 无请求体 key
	attachOpsRequestBodyToEntry(c, entry)
	require.Nil(t, entry.RequestBodyJSON)
	require.Nil(t, entry.RequestBodyBytes)
	require.False(t, entry.RequestBodyTruncated)

	// 错误类型
	c.Set(opsRequestBodyKey, "not-bytes")
	attachOpsRequestBodyToEntry(c, entry)
	require.Nil(t, entry.RequestBodyJSON)
	require.Nil(t, entry.RequestBodyBytes)

	// 空 bytes
	c.Set(opsRequestBodyKey, []byte{})
	attachOpsRequestBodyToEntry(c, entry)
	require.Nil(t, entry.RequestBodyJSON)
	require.Nil(t, entry.RequestBodyBytes)

	require.Equal(t, int64(0), OpsErrorLogSanitizedTotal())
}

func TestEnqueueOpsErrorLog_EarlyReturnBranches(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	entry := &service.OpsInsertErrorLogInput{ErrorPhase: "upstream", ErrorType: "upstream_error"}

	// nil 入参分支
	enqueueOpsErrorLog(nil, entry)
	enqueueOpsErrorLog(ops, nil)
	require.Equal(t, int64(0), OpsErrorLogEnqueuedTotal())

	// shutdown 分支
	close(opsErrorLogShutdownCh)
	enqueueOpsErrorLog(ops, entry)
	require.Equal(t, int64(0), OpsErrorLogEnqueuedTotal())

	// stopping 分支
	resetOpsErrorLoggerStateForTest(t)
	opsErrorLogMu.Lock()
	opsErrorLogStopping = true
	opsErrorLogMu.Unlock()
	enqueueOpsErrorLog(ops, entry)
	require.Equal(t, int64(0), OpsErrorLogEnqueuedTotal())

	// queue nil 分支（防止启动 worker 干扰）
	resetOpsErrorLoggerStateForTest(t)
	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = nil
	opsErrorLogMu.Unlock()
	enqueueOpsErrorLog(ops, entry)
	require.Equal(t, int64(0), OpsErrorLogEnqueuedTotal())
}

func TestOpsCaptureWriterPool_ResetOnRelease(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	writer := acquireOpsCaptureWriter(c.Writer)
	require.NotNil(t, writer)
	_, err := writer.buf.WriteString("temp-error-body")
	require.NoError(t, err)

	releaseOpsCaptureWriter(writer)

	reused := acquireOpsCaptureWriter(c.Writer)
	defer releaseOpsCaptureWriter(reused)

	require.Zero(t, reused.buf.Len(), "writer should be reset before reuse")
}

func TestOpsErrorLoggerMiddleware_RecordsNoAvailableAccounts(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
	gin.SetMode(gin.TestMode)

	// Keep the worker pool disabled so the test can inspect the enqueued job
	// deterministically instead of racing the async batch writer.
	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.POST("/backend-api/codex/responses", OpsErrorLoggerMiddleware(ops), func(c *gin.Context) {
		body := []byte(`{"model":"claude-sonnet-4-5-20250929","input":"hi"}`)
		setOpsRequestContext(c, "claude-sonnet-4-5-20250929", false, body)
		setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "No available accounts: no available accounts",
				"type":    "api_error",
			},
			"type": "error",
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/backend-api/codex/responses", nil)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, int64(1), OpsErrorLogEnqueuedTotal())
	require.Equal(t, int64(1), OpsErrorLogQueueLength())

	select {
	case job := <-opsErrorLogQueue:
		opsErrorLogQueueLen.Add(-1)
		require.NotNil(t, job.entry)
		require.Equal(t, "routing", job.entry.ErrorPhase)
		require.Equal(t, "api_error", job.entry.ErrorType)
		require.Equal(t, http.StatusServiceUnavailable, job.entry.StatusCode)
		require.Contains(t, job.entry.ErrorMessage, "No available accounts")
		require.Equal(t, "gateway", job.entry.ErrorSource)
		require.Equal(t, "platform", job.entry.ErrorOwner)
	default:
		t.Fatal("expected no-available-accounts error to be enqueued")
	}
}

func TestOpsErrorLoggerMiddleware_RecordsRecoveredUpstreamErrorOnSuccessfulRequest(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
	gin.SetMode(gin.TestMode)

	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.POST("/v1/messages", OpsErrorLoggerMiddleware(ops), func(c *gin.Context) {
		body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hi"}]}`)
		setOpsRequestContext(c, "claude-sonnet-4-5", true, body)
		setOpsEndpointContext(c, "claude-sonnet-4.5", int16(service.RequestTypeSync))
		c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{
			Platform:          service.PlatformKiro,
			AccountID:         99,
			AccountName:       "kiro-test",
			UpstreamRequestID: "kiro-upstream-1",
			Kind:              "response_anomaly",
			Message:           "kiro completed with anomalies: incomplete_tool_use_completed",
			Detail:            `{"anomaly_kinds":["incomplete_tool_use_completed"]}`,
		}})
		c.JSON(http.StatusOK, gin.H{"type": "message"})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(1), OpsErrorLogEnqueuedTotal())
	require.Equal(t, int64(1), OpsErrorLogQueueLength())

	select {
	case job := <-opsErrorLogQueue:
		opsErrorLogQueueLen.Add(-1)
		require.NotNil(t, job.entry)
		require.Equal(t, http.StatusOK, job.entry.StatusCode)
		require.Equal(t, "upstream", job.entry.ErrorPhase)
		require.Equal(t, "upstream_error", job.entry.ErrorType)
		require.Contains(t, job.entry.ErrorMessage, "Recovered upstream error")
		require.Contains(t, job.entry.ErrorMessage, "incomplete_tool_use_completed")
		require.Len(t, job.entry.UpstreamErrors, 1)
		require.Equal(t, "response_anomaly", job.entry.UpstreamErrors[0].Kind)
	default:
		t.Fatal("expected recovered upstream anomaly to be enqueued")
	}
}

func TestShouldSkipOpsErrorLog_NewAdvancedFilters(t *testing.T) {
	repo := &opsLoggerSettingRepoStub{values: map[string]string{}}
	raw, err := json.Marshal(map[string]any{
		"ignore_credential_401_errors":    true,
		"ignore_rate_limit_429_errors":    true,
		"ignore_account_not_found_errors": true,
	})
	require.NoError(t, err)
	repo.values[service.SettingKeyOpsAdvancedSettings] = string(raw)

	ops := service.NewOpsService(nil, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	require.True(t, shouldSkipOpsErrorLog(context.Background(), ops, "Authentication failed (401): invalid or expired credentials", "", "/v1/chat/completions", 401))
	require.True(t, shouldSkipOpsErrorLog(context.Background(), ops, "too many requests", "", "/v1/chat/completions", 429))
	require.True(t, shouldSkipOpsErrorLog(context.Background(), ops, "Account not found", "", "/internal/scheduled-tests/accounts/203/test", 500))
	require.False(t, shouldSkipOpsErrorLog(context.Background(), ops, "internal server error", "", "/v1/chat/completions", 500))
}

func TestOpsErrorLoggerMiddleware_DoesNotBreakOuterMiddlewares(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(middleware2.Recovery())
	r.Use(middleware2.RequestLogger())
	r.Use(middleware2.Logger())
	r.GET("/v1/messages", OpsErrorLoggerMiddleware(nil), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/messages", nil)

	require.NotPanics(t, func() {
		r.ServeHTTP(rec, req)
	})
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestIsKnownOpsErrorType(t *testing.T) {
	known := []string{
		"invalid_request_error",
		"authentication_error",
		"rate_limit_error",
		"billing_error",
		"subscription_error",
		"upstream_error",
		"upstream_http2_internal_error",
		"upstream_timeout_error",
		"overloaded_error",
		"api_error",
		"not_found_error",
		"forbidden_error",
	}
	for _, k := range known {
		require.True(t, isKnownOpsErrorType(k), "expected known: %s", k)
	}

	unknown := []string{"<nil>", "null", "", "random_error", "some_new_type", "<nil>\u003e"}
	for _, u := range unknown {
		require.False(t, isKnownOpsErrorType(u), "expected unknown: %q", u)
	}
}

func TestNormalizeOpsErrorType(t *testing.T) {
	tests := []struct {
		name    string
		errType string
		code    string
		want    string
	}{
		// Known types pass through.
		{"known invalid_request_error", "invalid_request_error", "", "invalid_request_error"},
		{"known rate_limit_error", "rate_limit_error", "", "rate_limit_error"},
		{"known upstream_error", "upstream_error", "", "upstream_error"},
		{"known detailed upstream error", "upstream_http2_internal_error", "", "upstream_http2_internal_error"},

		// Unknown/garbage types are rejected and fall through to code-based or default.
		{"nil literal from upstream", "<nil>", "", "api_error"},
		{"null string", "null", "", "api_error"},
		{"random string", "something_weird", "", "api_error"},

		// Unknown type but known code still maps correctly.
		{"nil with INSUFFICIENT_BALANCE code", "<nil>", "INSUFFICIENT_BALANCE", "billing_error"},
		{"nil with USAGE_LIMIT_EXCEEDED code", "<nil>", "USAGE_LIMIT_EXCEEDED", "subscription_error"},
		{"nil with USER_INACTIVE code", "<nil>", "USER_INACTIVE", "subscription_error"},

		// Empty type falls through to code-based mapping.
		{"empty type with balance code", "", "INSUFFICIENT_BALANCE", "billing_error"},
		{"empty type with subscription code", "", "SUBSCRIPTION_NOT_FOUND", "subscription_error"},
		{"empty type no code", "", "", "api_error"},

		// Known type overrides conflicting code-based mapping.
		{"known type overrides conflicting code", "rate_limit_error", "INSUFFICIENT_BALANCE", "rate_limit_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeOpsErrorType(tt.errType, tt.code)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestClassifyOpsErrorOwner_AccountScoped(t *testing.T) {
	require.Equal(t, "account", classifyOpsErrorOwner("request", "billing_error", "Insufficient account balance", "INSUFFICIENT_BALANCE"))
	require.Equal(t, "account", classifyOpsErrorOwner("request", "subscription_error", "User account is not active", "USER_INACTIVE"))
	require.Equal(t, "account", classifyOpsErrorOwner("auth", "authentication_error", "Failed to get upstream access token", ""))
	require.Equal(t, "client", classifyOpsErrorOwner("auth", "authentication_error", "Invalid API key", ""))
}

func TestSetOpsEndpointContext_SetsContextKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	setOpsEndpointContext(c, "claude-3-5-sonnet-20241022", int16(2)) // stream

	v, ok := c.Get(opsUpstreamModelKey)
	require.True(t, ok)
	vStr, ok := v.(string)
	require.True(t, ok)
	require.Equal(t, "claude-3-5-sonnet-20241022", vStr)

	rt, ok := c.Get(opsRequestTypeKey)
	require.True(t, ok)
	rtVal, ok := rt.(int16)
	require.True(t, ok)
	require.Equal(t, int16(2), rtVal)
}

func TestSetOpsEndpointContext_EmptyModelNotStored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	setOpsEndpointContext(c, "", int16(1))

	_, ok := c.Get(opsUpstreamModelKey)
	require.False(t, ok, "empty upstream model should not be stored")

	rt, ok := c.Get(opsRequestTypeKey)
	require.True(t, ok)
	rtVal, ok := rt.(int16)
	require.True(t, ok)
	require.Equal(t, int16(1), rtVal)
}

func TestSetOpsEndpointContext_NilContext(t *testing.T) {
	require.NotPanics(t, func() {
		setOpsEndpointContext(nil, "model", int16(1))
	})
}
