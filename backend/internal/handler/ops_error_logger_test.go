package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsErrorLoggerCancelKey struct{}

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

func TestOpsErrorLogConfig_CapsWorstCaseQueueFootprint(t *testing.T) {
	prev := runtime.GOMAXPROCS(64)
	t.Cleanup(func() { runtime.GOMAXPROCS(prev) })

	workerCount, queueSize := opsErrorLogConfig()

	require.Equal(t, opsErrorLogMaxWorkerCount, workerCount)
	require.Equal(t, 1024, queueSize)
}

func TestOpsCaptureWriterPool_ResetOnRelease(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	writer := acquireOpsCaptureWriter(c.Writer)
	require.NotNil(t, writer)
	c.Writer.WriteHeader(http.StatusInternalServerError)
	_, err := writer.WriteString("temp-error-body")
	require.NoError(t, err)
	require.NotEmpty(t, writer.capturedBytes())

	releaseOpsCaptureWriter(writer)

	reused := acquireOpsCaptureWriter(c.Writer)
	defer releaseOpsCaptureWriter(reused)

	require.Empty(t, reused.capturedBytes(), "writer should be reset before reuse")
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

func TestOpsErrorLoggerMiddleware_SkipsRecoveredUpstreamErrorOnSuccessfulRequest(t *testing.T) {
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
	require.Equal(t, int64(0), OpsErrorLogEnqueuedTotal())
	require.Equal(t, int64(0), OpsErrorLogQueueLength())
}

func TestOpsErrorLoggerMiddleware_ResponsesFailedSSEUsesFailureStatus(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
	gin.SetMode(gin.TestMode)

	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.POST("/v1/responses", OpsErrorLoggerMiddleware(ops), func(c *gin.Context) {
		body := []byte(`{"model":"gpt-5.5","input":"hi","stream":true}`)
		setOpsRequestContext(c, "gpt-5.5", true, body)
		setOpsEndpointContext(c, "gpt-5.5", int16(service.RequestTypeSync))

		h := &OpenAIGatewayHandler{}
		h.handleStreamingAwareError(c, http.StatusTooManyRequests, "rate_limit_error", "Too many pending requests, please retry later", true)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "streaming response already started, recorder keeps HTTP 200")
	require.Equal(t, int64(1), OpsErrorLogEnqueuedTotal())
	require.Equal(t, int64(1), OpsErrorLogQueueLength())

	select {
	case job := <-opsErrorLogQueue:
		opsErrorLogQueueLen.Add(-1)
		require.NotNil(t, job.entry)
		require.Equal(t, http.StatusTooManyRequests, job.entry.StatusCode, "client-visible stream failure must not be persisted as 200")
		require.NotEqual(t, http.StatusOK, job.entry.StatusCode)
		require.Contains(t, job.entry.ErrorBody, `"type":"response.failed"`)
		require.Contains(t, job.entry.ErrorMessage, "Too many pending requests")
	default:
		t.Fatal("expected response.failed streaming error to be enqueued")
	}
}

func TestOpsErrorLoggerMiddleware_StreamFailurePrefersParsedMessageForUpstreamContext(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
	gin.SetMode(gin.TestMode)

	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.POST("/v1/responses", OpsErrorLoggerMiddleware(ops), func(c *gin.Context) {
		body := []byte(`{"model":"gpt-5.5","input":"hi","stream":true}`)
		setOpsRequestContext(c, "gpt-5.5", true, body)
		setOpsEndpointContext(c, "gpt-5.5", int16(service.RequestTypeSync))
		service.SetOpsUpstreamErrorWithType(c, "invalid_request_error", 0, "Upstream transport error", `upstream response failed: Your input exceeds the context window of this model. Please adjust your input and try again.`)
		c.Status(http.StatusOK)
		_, _ = c.Writer.WriteString(
			"event: error\n" +
				`data: {"type":"error","error":{"type":"invalid_request_error","code":"context_length_exceeded","message":"Your input exceeds the context window of this model. Please adjust your input and try again."}}` +
				"\n\n",
		)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(1), OpsErrorLogEnqueuedTotal())
	require.Equal(t, int64(1), OpsErrorLogQueueLength())

	select {
	case job := <-opsErrorLogQueue:
		opsErrorLogQueueLen.Add(-1)
		require.NotNil(t, job.entry)
		require.Equal(t, http.StatusBadRequest, job.entry.StatusCode)
		require.NotNil(t, job.entry.UpstreamErrorMessage)
		require.Equal(t, "Your input exceeds the context window of this model. Please adjust your input and try again.", *job.entry.UpstreamErrorMessage)
		require.NotNil(t, job.entry.UpstreamStatusCode)
		require.Equal(t, http.StatusBadRequest, *job.entry.UpstreamStatusCode)
	default:
		t.Fatal("expected parsed stream failure log to be enqueued")
	}
}

func TestOpsErrorLoggerMiddleware_SkipsRecoveredUpstreamCanceledOnSuccessfulRequest(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
	gin.SetMode(gin.TestMode)

	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.POST("/v1/responses", OpsErrorLoggerMiddleware(ops), func(c *gin.Context) {
		body := []byte(`{"model":"gpt-5.5","input":"hi","stream":true}`)
		setOpsRequestContext(c, "gpt-5.5", true, body)
		setOpsEndpointContext(c, "gpt-5.5", int16(service.RequestTypeSync))
		c.Set(service.OpsUpstreamErrorTypeKey, "upstream_canceled_error")
		c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{
			Platform:  service.PlatformOpenAI,
			AccountID: 67124,
			Kind:      "transport_error",
			Message:   "Upstream request was canceled",
			Detail:    "failed to get reader: context canceled",
		}})
		c.JSON(http.StatusOK, gin.H{"id": "resp_123"})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(0), OpsErrorLogEnqueuedTotal())
	require.Equal(t, int64(0), OpsErrorLogQueueLength())
}

func TestOpsErrorLoggerMiddleware_SkipsRecoveredContextCanceledWhenRequestContextCanceled(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
	gin.SetMode(gin.TestMode)

	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.POST("/v1/images/generations", OpsErrorLoggerMiddleware(ops), func(c *gin.Context) {
		body := []byte(`{"model":"gpt-image-2","prompt":"draw"}`)
		setOpsRequestContext(c, "gpt-image-2", false, body)
		setOpsEndpointContext(c, "gpt-image-2", int16(service.RequestTypeImage))
		c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{
			Platform:           service.PlatformOpenAI,
			AccountID:          74209,
			Kind:               "stream_error",
			Message:            "context canceled",
			UpstreamStatusCode: http.StatusOK,
			Detail:             "context canceled",
		}})
		if cancel, ok := c.Request.Context().Value(opsErrorLoggerCancelKey{}).(context.CancelFunc); ok {
			cancel()
		}
		c.JSON(http.StatusOK, gin.H{"id": "img_123"})
	})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil).WithContext(context.WithValue(ctx, opsErrorLoggerCancelKey{}, context.CancelFunc(cancel)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(0), OpsErrorLogEnqueuedTotal())
	require.Equal(t, int64(0), OpsErrorLogQueueLength())
}

func TestOpsErrorLoggerMiddleware_RecordsNonImageRecoveredContextCanceledWhenRequestContextCanceled(t *testing.T) {
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
			Platform:           service.PlatformKiro,
			AccountID:          99,
			Kind:               "transport_error",
			Message:            "context canceled",
			UpstreamStatusCode: http.StatusOK,
			Detail:             "context canceled",
		}})
		if cancel, ok := c.Request.Context().Value(opsErrorLoggerCancelKey{}).(context.CancelFunc); ok {
			cancel()
		}
		c.JSON(http.StatusOK, gin.H{"type": "message"})
	})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(context.WithValue(ctx, opsErrorLoggerCancelKey{}, context.CancelFunc(cancel)))
	rec := httptest.NewRecorder()
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
		require.Contains(t, job.entry.ErrorMessage, "context canceled")
	default:
		t.Fatal("expected non-image recovered context canceled telemetry to be enqueued")
	}
}

func TestOpsErrorLoggerMiddleware_RecordsImageRecoveredContextCanceledWhenRequestDeadlineExceeded(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
	gin.SetMode(gin.TestMode)

	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.POST("/v1/images/generations", OpsErrorLoggerMiddleware(ops), func(c *gin.Context) {
		body := []byte(`{"model":"gpt-image-2","prompt":"draw"}`)
		setOpsRequestContext(c, "gpt-image-2", false, body)
		setOpsEndpointContext(c, "gpt-image-2", int16(service.RequestTypeImage))
		c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{
			Platform:           service.PlatformOpenAI,
			AccountID:          74209,
			Kind:               "stream_error",
			Message:            "context canceled",
			UpstreamStatusCode: http.StatusOK,
			Detail:             "context canceled",
		}})
		c.JSON(http.StatusOK, gin.H{"id": "img_123"})
	})

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
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
		require.Contains(t, job.entry.ErrorMessage, "context canceled")
	default:
		t.Fatal("expected image recovered context canceled telemetry to be enqueued for deadline exceeded requests")
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

// setupOpsErrorLogTestQueue 阻止 enqueueOpsErrorLog 启动真实 worker，改用可检查的测试队列。
func setupOpsErrorLogTestQueue(t *testing.T, size int) {
	t.Helper()
	resetOpsErrorLoggerStateForTest(t)
	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, size)
	opsErrorLogMu.Unlock()
}

// 就地(in-band) SSE 错误挂在已固化的 HTTP 200 流上：wire 状态码为 200，
// 常规 status>=400 采集路径不会触发。logOpsStreamError 必须据 MarkOpsStreamError
// 补记一条错误日志，且用 IntendedStatus(429) 分级、StatusCode 仍记 wire 的 200。
func TestLogOpsStreamError_RecordsInBandConcurrencyLimit(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 4)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set(opsModelKey, "test-model")

	service.MarkOpsStreamError(c, "rate_limit_error",
		"Concurrency limit exceeded for account, please retry later", http.StatusTooManyRequests)

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	logOpsStreamError(c, ops, http.StatusOK)

	require.Equal(t, int64(1), OpsErrorLogEnqueuedTotal())
	require.Equal(t, int64(1), OpsErrorLogQueueLength())

	job := <-opsErrorLogQueue
	require.NotNil(t, job.entry)
	require.Equal(t, "rate_limit_error", job.entry.ErrorType)
	require.Equal(t, "request", job.entry.ErrorPhase)
	require.True(t, job.entry.IsBusinessLimited)
	require.True(t, job.entry.Stream)
	require.Equal(t, http.StatusOK, job.entry.StatusCode) // wire 状态码保持 200
	require.Equal(t, "P1", job.entry.Severity)            // 用 IntendedStatus 429 分级
	require.Equal(t, "test-model", job.entry.Model)
	require.Equal(t, "Concurrency limit exceeded for account, please retry later", job.entry.ErrorMessage)
}

// 未标记流内错误时 logOpsStreamError 必须是 no-op（不误记正常的 200 流）。
func TestLogOpsStreamError_NoopWhenNotMarked(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 4)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	logOpsStreamError(c, ops, http.StatusOK)

	require.Equal(t, int64(0), OpsErrorLogEnqueuedTotal())
}

// 命中 skip_monitoring=true 透传规则时不落库，与其它采集分支一致。
func TestLogOpsStreamError_SkipWhenPassthroughSkipMonitoring(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 4)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	service.MarkOpsStreamError(c, "upstream_error", "Upstream request failed", http.StatusBadGateway)
	c.Set(service.OpsSkipPassthroughKey, true)

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	logOpsStreamError(c, ops, http.StatusOK)

	require.Equal(t, int64(0), OpsErrorLogEnqueuedTotal())
}

// MarkOpsStreamError 采用「首个标记生效」：后续的通用兜底帧不得覆盖根因错误。
func TestMarkOpsStreamError_FirstWins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	service.MarkOpsStreamError(c, "rate_limit_error", "Concurrency limit exceeded for account", http.StatusTooManyRequests)
	service.MarkOpsStreamError(c, "upstream_error", "Upstream request failed", http.StatusBadGateway)

	se, ok := service.GetOpsStreamError(c)
	require.True(t, ok)
	require.Equal(t, "rate_limit_error", se.ErrType)
	require.Equal(t, "Concurrency limit exceeded for account", se.Message)
	require.Equal(t, http.StatusTooManyRequests, se.IntendedStatus)
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

func classifyOpsErrorLog(c *gin.Context, errType, message, code string, status int) (string, bool, string, string) {
	phase := classifyOpsPhaseForContext(c, errType, message, code)
	isBusinessLimited := classifyOpsIsBusinessLimitedForContext(c, errType, phase, code, status, message)
	errorOwner := classifyOpsErrorOwnerForContext(c, phase, errType, message, code)
	errorSource := classifyOpsErrorSourceForContext(c, phase, errType, message, code)
	return phase, isBusinessLimited, errorOwner, errorSource
}

func TestClassifyOpsErrorOwner_AccountScoped(t *testing.T) {
	require.Equal(t, "account", classifyOpsErrorOwner("request", "billing_error", "Insufficient account balance", "INSUFFICIENT_BALANCE"))
	require.Equal(t, "account", classifyOpsErrorOwner("request", "subscription_error", "User account is not active", "USER_INACTIVE"))
	require.Equal(t, "account", classifyOpsErrorOwner("auth", "authentication_error", "Failed to get upstream access token", ""))
	require.Equal(t, "client", classifyOpsErrorOwner("auth", "authentication_error", "Invalid API key", ""))
}

func TestClassifyOpsNoAvailableAccountsExcludedFromSLA(t *testing.T) {
	const message = "No available accounts"
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	markOpsRoutingCapacityLimited(c)

	errType := normalizeOpsErrorType("api_error", "")
	phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(c, errType, message, "", http.StatusServiceUnavailable)

	require.Equal(t, "api_error", errType)
	require.Equal(t, "routing", phase)
	require.True(t, isBusinessLimited)
	require.Equal(t, "platform", errorOwner)
	require.Equal(t, "gateway", errorSource)
}

func TestClassifyOpsRoutingCapacityMarkerExcludesMaskedSelectionFailureFromSLA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	markOpsRoutingCapacityLimited(c)

	phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(
		c,
		"api_error",
		"Service temporarily unavailable",
		"",
		http.StatusServiceUnavailable,
	)

	require.Equal(t, "routing", phase)
	require.True(t, isBusinessLimited)
	require.Equal(t, "platform", errorOwner)
	require.Equal(t, "gateway", errorSource)
}

func TestClassifyOpsAuthClientErrorsExcludedFromSLA(t *testing.T) {
	tests := []struct {
		name    string
		errType string
		message string
		code    string
		status  int
	}{
		{
			name:    "standard invalid API key",
			errType: "api_error",
			message: "Invalid API key",
			code:    "INVALID_API_KEY",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "standard missing API key",
			errType: "api_error",
			message: "API key is required in Authorization header (Bearer scheme), x-api-key header, or x-goog-api-key header",
			code:    "API_KEY_REQUIRED",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "expired local API key",
			errType: "api_error",
			message: "API key 已过期",
			code:    "API_KEY_EXPIRED",
			status:  http.StatusForbidden,
		},
		{
			name:    "disabled local API key",
			errType: "api_error",
			message: "API key is disabled",
			code:    "API_KEY_DISABLED",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "local API key user missing",
			errType: "api_error",
			message: "User associated with API key not found",
			code:    "USER_NOT_FOUND",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "inactive local API key user",
			errType: "api_error",
			message: "User account is not active",
			code:    "USER_INACTIVE",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "deleted local API key group",
			errType: "api_error",
			message: "API Key 所属分组已删除",
			code:    "GROUP_DELETED",
			status:  http.StatusForbidden,
		},
		{
			name:    "disabled local API key group",
			errType: "api_error",
			message: "API Key 所属分组已停用",
			code:    "GROUP_DISABLED",
			status:  http.StatusForbidden,
		},
		{
			name:    "google deleted API key group message without semantic code",
			errType: "api_error",
			message: "API Key 所属分组已删除",
			code:    "403",
			status:  http.StatusForbidden,
		},
		{
			name:    "anthropic unassigned API key group",
			errType: "permission_error",
			message: "API Key is not assigned to any group and cannot be used. Please contact the administrator to assign it to a group.",
			code:    "",
			status:  http.StatusForbidden,
		},
		{
			name:    "google invalid API key",
			errType: "api_error",
			message: "Invalid API key",
			code:    "401",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "google missing API key",
			errType: "api_error",
			message: "API key is required",
			code:    "401",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "google disabled API key",
			errType: "api_error",
			message: "API key is disabled",
			code:    "401",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "google local API key user missing",
			errType: "api_error",
			message: "User associated with API key not found",
			code:    "401",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "google inactive local API key user",
			errType: "api_error",
			message: "User account is not active",
			code:    "401",
			status:  http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)

			errType := normalizeOpsErrorType(tt.errType, tt.code)
			phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(c, errType, tt.message, tt.code, tt.status)

			require.Equal(t, "api_error", errType)
			require.Equal(t, "auth", phase)
			require.True(t, isBusinessLimited)
			require.Equal(t, "client", errorOwner)
			require.Equal(t, "client_request", errorSource)
		})
	}
}

func TestClassifyOpsLocalBusinessLimitErrorsExcludedFromSLA(t *testing.T) {
	tests := []struct {
		name        string
		errType     string
		message     string
		code        string
		status      int
		wantErrType string
		wantPhase   string
	}{
		{
			name:        "standard API key quota exhausted",
			errType:     "api_error",
			message:     "API key 额度已用完",
			code:        "API_KEY_QUOTA_EXHAUSTED",
			status:      http.StatusTooManyRequests,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "standard query API key deprecated",
			errType:     "api_error",
			message:     "API key in query parameter is deprecated. Please use Authorization header instead.",
			code:        "api_key_in_query_deprecated",
			status:      http.StatusBadRequest,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "google query API key deprecated",
			errType:     "api_error",
			message:     "Query parameter api_key is deprecated. Use Authorization header or key instead.",
			code:        "400",
			status:      http.StatusBadRequest,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "google no active subscription",
			errType:     "api_error",
			message:     "No active subscription found for this group",
			code:        "403",
			status:      http.StatusForbidden,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "gateway subscription invalid cache recheck",
			errType:     "billing_error",
			message:     "subscription is invalid or expired",
			code:        "billing_error",
			status:      http.StatusForbidden,
			wantErrType: "billing_error",
			wantPhase:   "request",
		},
		{
			name:        "google insufficient account balance",
			errType:     "api_error",
			message:     "Insufficient account balance",
			code:        "403",
			status:      http.StatusForbidden,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "gateway billing cache insufficient balance",
			errType:     "billing_error",
			message:     "insufficient balance",
			code:        "",
			status:      http.StatusForbidden,
			wantErrType: "billing_error",
			wantPhase:   "request",
		},
		{
			name:        "gemini group platform mismatch",
			errType:     "api_error",
			message:     "API key group platform is not gemini",
			code:        "400",
			status:      http.StatusBadRequest,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "gateway API key 5h rate limit",
			errType:     "api_error",
			message:     "api key 5小时限额已用完",
			code:        "rate_limit_exceeded",
			status:      http.StatusTooManyRequests,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "gateway group RPM limit",
			errType:     "api_error",
			message:     "group requests-per-minute limit exceeded",
			code:        "rate_limit_exceeded",
			status:      http.StatusTooManyRequests,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "google subscription daily limit",
			errType:     "api_error",
			message:     "daily usage limit exceeded",
			code:        "429",
			status:      http.StatusTooManyRequests,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "user platform daily quota exhausted",
			errType:     "api_error",
			message:     "Daily usage quota exhausted for this platform.",
			code:        "rate_limit_exceeded",
			status:      http.StatusTooManyRequests,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "local pending queue limit",
			errType:     "rate_limit_error",
			message:     "Too many pending requests, please retry later",
			code:        "",
			status:      http.StatusTooManyRequests,
			wantErrType: "rate_limit_error",
			wantPhase:   "request",
		},
		{
			name:        "local concurrency limit",
			errType:     "rate_limit_error",
			message:     "Concurrency limit exceeded for user, please retry later",
			code:        "",
			status:      http.StatusTooManyRequests,
			wantErrType: "rate_limit_error",
			wantPhase:   "request",
		},
		{
			name:        "group claude code only feature gate",
			errType:     "permission_error",
			message:     "This group is restricted to Claude Code clients (/v1/messages only)",
			code:        "",
			status:      http.StatusForbidden,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "group image generation feature gate",
			errType:     "permission_error",
			message:     "Image generation is not enabled for this group",
			code:        "",
			status:      http.StatusForbidden,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "route token counting platform unsupported",
			errType:     "not_found_error",
			message:     "Token counting is not supported for this platform",
			code:        "",
			status:      http.StatusNotFound,
			wantErrType: "not_found_error",
			wantPhase:   "request",
		},
		{
			name:        "route images API platform unsupported",
			errType:     "not_found_error",
			message:     "Images API is not supported for this platform",
			code:        "",
			status:      http.StatusNotFound,
			wantErrType: "not_found_error",
			wantPhase:   "request",
		},
		{
			name:        "antigravity model whitelist feature gate",
			errType:     "permission_error",
			message:     "model claude-3-5-sonnet not in whitelist",
			code:        "",
			status:      http.StatusForbidden,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "google antigravity model whitelist feature gate",
			errType:     "api_error",
			message:     "model gemini-2.5-pro not in whitelist",
			code:        "403",
			status:      http.StatusForbidden,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "claude beta policy block",
			errType:     "invalid_request_error",
			message:     "beta feature interleaved-thinking-2025-05-14 is not allowed",
			code:        "",
			status:      http.StatusBadRequest,
			wantErrType: "invalid_request_error",
			wantPhase:   "request",
		},
		{
			name:        "openai fast policy block",
			errType:     "permission_error",
			message:     "openai service_tier=priority is not allowed for model gpt-5.5",
			code:        "",
			status:      http.StatusForbidden,
			wantErrType: "api_error",
			wantPhase:   "request",
		},
		{
			name:        "codex official client policy block",
			errType:     "forbidden_error",
			message:     "This account only allows Codex official clients",
			code:        "",
			status:      http.StatusForbidden,
			wantErrType: "forbidden_error",
			wantPhase:   "request",
		},
		{
			name:        "openai wsv1 unsupported feature gate",
			errType:     "invalid_request_error",
			message:     "OpenAI WSv1 is temporarily unsupported. Please enable responses_websockets_v2.",
			code:        "",
			status:      http.StatusBadRequest,
			wantErrType: "invalid_request_error",
			wantPhase:   "request",
		},
		{
			name:        "openai passthrough instructions policy block",
			errType:     "forbidden_error",
			message:     "OpenAI codex passthrough requires a non-empty instructions field",
			code:        "",
			status:      http.StatusForbidden,
			wantErrType: "forbidden_error",
			wantPhase:   "request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)

			errType := normalizeOpsErrorType(tt.errType, tt.code)
			phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(c, errType, tt.message, tt.code, tt.status)

			require.Equal(t, tt.wantErrType, errType)
			require.Equal(t, tt.wantPhase, phase)
			require.True(t, isBusinessLimited)
			require.Equal(t, "client", errorOwner)
			require.Equal(t, "client_request", errorSource)
		})
	}
}

func TestClassifyOpsIPRestrictionAccessDeniedExcludedFromSLA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonIPRestriction)

	errType := normalizeOpsErrorType("api_error", "ACCESS_DENIED")
	phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(c, errType, "Access denied", "ACCESS_DENIED", http.StatusForbidden)

	require.Equal(t, "api_error", errType)
	require.Equal(t, "auth", phase)
	require.True(t, isBusinessLimited)
	require.Equal(t, "client", errorOwner)
	require.Equal(t, "client_request", errorSource)
}

func TestClassifyOpsClientBusinessLimitedMarkerExcludesCustomPolicyDenialFromSLA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)

	errType := normalizeOpsErrorType("invalid_request_error", "")
	phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(c, errType, "custom admin policy message", "", http.StatusBadRequest)

	require.Equal(t, "invalid_request_error", errType)
	require.Equal(t, "auth", phase)
	require.True(t, isBusinessLimited)
	require.Equal(t, "client", errorOwner)
	require.Equal(t, "client_request", errorSource)
}

func TestClassifyOpsOtherErrorsStillCountForSLA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	errType := normalizeOpsErrorType("api_error", "INTERNAL_ERROR")
	phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(c, errType, "Failed to validate API key", "INTERNAL_ERROR", http.StatusInternalServerError)

	require.Equal(t, "api_error", errType)
	require.Equal(t, "internal", phase)
	require.False(t, isBusinessLimited)
	require.Equal(t, "platform", errorOwner)
	require.Equal(t, "gateway", errorSource)
}

func TestClassifyOpsUnsupportedModelExcludedFromSLA(t *testing.T) {
	tests := []string{
		"No available accounts: no available accounts supporting model: made-up-model",
		"No available accounts: no available OpenAI accounts supporting model: made-up-model",
		"No available Gemini accounts: no available Gemini accounts supporting model: made-up-model",
		"No available accounts: no available accounts supporting model: made-up-model (channel pricing restriction)",
	}

	for _, message := range tests {
		t.Run(message, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			markOpsRoutingCapacityLimited(c)

			errType := normalizeOpsErrorType("api_error", "")
			phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(c, errType, message, "", http.StatusServiceUnavailable)

			require.Equal(t, "api_error", errType)
			require.Equal(t, "routing", phase)
			require.True(t, isBusinessLimited)
			require.Equal(t, "platform", errorOwner)
			require.Equal(t, "gateway", errorSource)
		})
	}
}

func TestClassifyOpsUnmarkedNoAvailableTextStillCountsForSLA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(
		c,
		"api_error",
		"No available accounts",
		"",
		http.StatusServiceUnavailable,
	)

	require.Equal(t, "routing", phase)
	require.False(t, isBusinessLimited)
	require.Equal(t, "platform", errorOwner)
	require.Equal(t, "gateway", errorSource)
}

func TestClassifyOpsUpstreamAuthTextStillCountsForSLA(t *testing.T) {
	tests := []struct {
		name    string
		message string
		code    string
		status  int
	}{
		{
			name:    "invalid API key",
			message: "Invalid API key",
			code:    "401",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "disabled API key",
			message: "API key is disabled",
			code:    "API_KEY_DISABLED",
			status:  http.StatusUnauthorized,
		},
		{
			name:    "gemini group platform mismatch",
			message: "API key group platform is not gemini",
			code:    "400",
			status:  http.StatusBadRequest,
		},
		{
			name:    "provider balance error",
			message: "Insufficient account balance",
			code:    "INSUFFICIENT_BALANCE",
			status:  http.StatusForbidden,
		},
		{
			name:    "provider subscription error",
			message: "No active subscription found for this group",
			code:    "SUBSCRIPTION_NOT_FOUND",
			status:  http.StatusForbidden,
		},
		{
			name:    "provider quota error",
			message: "api key 额度已用完",
			code:    "API_KEY_QUOTA_EXHAUSTED",
			status:  http.StatusTooManyRequests,
		},
		{
			name:    "provider deleted group shaped error",
			message: "API Key 所属分组已删除",
			code:    "GROUP_DELETED",
			status:  http.StatusForbidden,
		},
		{
			name:    "provider unassigned group shaped error",
			message: "API Key is not assigned to any group and cannot be used. Please contact the administrator to assign it to a group.",
			code:    "403",
			status:  http.StatusForbidden,
		},
		{
			name:    "provider local quota shaped error",
			message: "Daily usage quota exhausted for this platform.",
			code:    "rate_limit_exceeded",
			status:  http.StatusTooManyRequests,
		},
		{
			name:    "provider feature gate shaped error",
			message: "Image generation is not enabled for this group",
			code:    "403",
			status:  http.StatusForbidden,
		},
		{
			name:    "provider token counting unsupported shaped error",
			message: "Token counting is not supported for this platform",
			code:    "404",
			status:  http.StatusNotFound,
		},
		{
			name:    "provider image API unsupported shaped error",
			message: "Images API is not supported for this platform",
			code:    "404",
			status:  http.StatusNotFound,
		},
		{
			name:    "provider antigravity whitelist shaped error",
			message: "model claude-3-5-sonnet not in whitelist",
			code:    "403",
			status:  http.StatusForbidden,
		},
		{
			name:    "provider beta policy shaped error",
			message: "beta feature interleaved-thinking-2025-05-14 is not allowed",
			code:    "400",
			status:  http.StatusBadRequest,
		},
		{
			name:    "provider openai fast policy shaped error",
			message: "openai service_tier=priority is not allowed for model gpt-5.5",
			code:    "403",
			status:  http.StatusForbidden,
		},
		{
			name:    "provider codex client policy shaped error",
			message: "This account only allows Codex official clients",
			code:    "403",
			status:  http.StatusForbidden,
		},
		{
			name:    "provider wsv1 unsupported shaped error",
			message: "OpenAI WSv1 is temporarily unsupported. Please enable responses_websockets_v2.",
			code:    "400",
			status:  http.StatusBadRequest,
		},
		{
			name:    "provider passthrough instructions shaped error",
			message: "OpenAI codex passthrough requires a non-empty instructions field",
			code:    "403",
			status:  http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			service.SetOpsUpstreamError(c, tt.status, tt.message, "")

			phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(
				c,
				"api_error",
				tt.message,
				tt.code,
				tt.status,
			)

			require.Equal(t, "upstream", phase)
			require.False(t, isBusinessLimited)
			require.Equal(t, "provider", errorOwner)
			require.Equal(t, "upstream_http", errorSource)
		})
	}
}

func TestClassifyOpsUpstreamNoAvailableTextStillCountsForSLA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	service.SetOpsUpstreamError(c, http.StatusServiceUnavailable, "No available accounts", "")

	phase, isBusinessLimited, errorOwner, errorSource := classifyOpsErrorLog(
		c,
		"api_error",
		"No available accounts",
		"",
		http.StatusServiceUnavailable,
	)

	require.Equal(t, "upstream", phase)
	require.False(t, isBusinessLimited)
	require.Equal(t, "provider", errorOwner)
	require.Equal(t, "upstream_http", errorSource)
}

func TestParseOpsErrorResponsePreservesNestedStringCode(t *testing.T) {
	parsed := parseOpsErrorResponse([]byte(`{"error":{"type":"permission_error","code":"GROUP_DELETED","message":"API Key 所属分组已删除"}}`))

	require.Equal(t, "permission_error", parsed.ErrorType)
	require.Equal(t, "GROUP_DELETED", parsed.Code)
	require.Equal(t, "API Key 所属分组已删除", parsed.Message)
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

func TestOpsErrorLoggerMiddleware_UsesBusinessLimitedMarkerForClassification(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
	gin.SetMode(gin.TestMode)

	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.POST("/v1/chat/completions", OpsErrorLoggerMiddleware(ops), func(c *gin.Context) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonIPRestriction)
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "ACCESS_DENIED",
			"message": "Access denied",
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)

	select {
	case job := <-opsErrorLogQueue:
		opsErrorLogQueueLen.Add(-1)
		require.NotNil(t, job.entry)
		require.True(t, job.entry.IsBusinessLimited)
		require.Equal(t, "auth", job.entry.ErrorPhase)
		require.Equal(t, "client", job.entry.ErrorOwner)
		require.Equal(t, "client_request", job.entry.ErrorSource)
	default:
		t.Fatal("expected business-limited error to be enqueued")
	}
}

func TestOpsErrorLoggerMiddleware_UsesRoutingCapacityMarkerForClassification(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
	gin.SetMode(gin.TestMode)

	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.POST("/v1beta/models/gemini-2.5-pro:generateContent", OpsErrorLoggerMiddleware(ops), func(c *gin.Context) {
		markOpsRoutingCapacityLimited(c)
		googleError(c, http.StatusServiceUnavailable, "No available Gemini accounts")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent", nil)
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	select {
	case job := <-opsErrorLogQueue:
		opsErrorLogQueueLen.Add(-1)
		require.NotNil(t, job.entry)
		require.Equal(t, "routing", job.entry.ErrorPhase)
		require.Equal(t, "platform", job.entry.ErrorOwner)
		require.Equal(t, "gateway", job.entry.ErrorSource)
		require.True(t, job.entry.IsBusinessLimited)
	default:
		t.Fatal("expected routing-capacity error to be enqueued")
	}
}
