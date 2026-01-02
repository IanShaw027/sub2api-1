package handler

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	opsModelKey  = "ops_model"
	opsStreamKey = "ops_stream"
)

const (
	opsErrorLogTimeout      = 5 * time.Second
	opsErrorLogDrainTimeout = 10 * time.Second

	opsErrorLogMinWorkerCount = 4
	opsErrorLogMaxWorkerCount = 32

	opsErrorLogQueueSizePerWorker = 128
	opsErrorLogMinQueueSize       = 256
	opsErrorLogMaxQueueSize       = 8192
)

type opsErrorLogJob struct {
	ops   *service.OpsService
	entry *service.OpsErrorLog
}

var (
	opsErrorLogOnce  sync.Once
	opsErrorLogQueue chan opsErrorLogJob

	opsErrorLogStopOnce  sync.Once
	opsErrorLogWorkersWg sync.WaitGroup
	opsErrorLogMu        sync.RWMutex
	opsErrorLogStopping  bool
	opsErrorLogQueueLen  atomic.Int64

	opsErrorLogShutdownCh   = make(chan struct{})
	opsErrorLogShutdownOnce sync.Once
	opsErrorLogDrained      atomic.Bool
)

func startOpsErrorLogWorkers() {
	opsErrorLogMu.Lock()
	defer opsErrorLogMu.Unlock()

	if opsErrorLogStopping {
		return
	}

	workerCount, queueSize := opsErrorLogConfig()
	opsErrorLogQueue = make(chan opsErrorLogJob, queueSize)
	opsErrorLogQueueLen.Store(0)

	opsErrorLogWorkersWg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer opsErrorLogWorkersWg.Done()
			for job := range opsErrorLogQueue {
				opsErrorLogQueueLen.Add(-1)
				if job.ops == nil || job.entry == nil {
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), opsErrorLogTimeout)
				_ = job.ops.RecordError(ctx, job.entry)
				cancel()
			}
		}()
	}
}

func enqueueOpsErrorLog(ops *service.OpsService, entry *service.OpsErrorLog) {
	if ops == nil || entry == nil {
		return
	}
	select {
	case <-opsErrorLogShutdownCh:
		return
	default:
	}

	opsErrorLogMu.RLock()
	stopping := opsErrorLogStopping
	opsErrorLogMu.RUnlock()
	if stopping {
		return
	}

	opsErrorLogOnce.Do(startOpsErrorLogWorkers)

	opsErrorLogMu.RLock()
	defer opsErrorLogMu.RUnlock()
	if opsErrorLogStopping || opsErrorLogQueue == nil {
		return
	}

	select {
	case opsErrorLogQueue <- opsErrorLogJob{ops: ops, entry: entry}:
		opsErrorLogQueueLen.Add(1)
	default:
		// Queue is full; drop to avoid blocking request handling.
	}
}

func StopOpsErrorLogWorkers() bool {
	opsErrorLogStopOnce.Do(func() {
		opsErrorLogShutdownOnce.Do(func() {
			close(opsErrorLogShutdownCh)
		})
		opsErrorLogDrained.Store(stopOpsErrorLogWorkers())
	})
	return opsErrorLogDrained.Load()
}

func stopOpsErrorLogWorkers() bool {
	opsErrorLogMu.Lock()
	opsErrorLogStopping = true
	ch := opsErrorLogQueue
	if ch != nil {
		close(ch)
	}
	opsErrorLogQueue = nil
	opsErrorLogMu.Unlock()

	if ch == nil {
		opsErrorLogQueueLen.Store(0)
		return true
	}

	done := make(chan struct{})
	go func() {
		opsErrorLogWorkersWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		opsErrorLogQueueLen.Store(0)
		return true
	case <-time.After(opsErrorLogDrainTimeout):
		return false
	}
}

func OpsErrorLogQueueLength() int64 {
	return opsErrorLogQueueLen.Load()
}

func opsErrorLogConfig() (workerCount int, queueSize int) {
	workerCount = runtime.GOMAXPROCS(0) * 2
	if workerCount < opsErrorLogMinWorkerCount {
		workerCount = opsErrorLogMinWorkerCount
	}
	if workerCount > opsErrorLogMaxWorkerCount {
		workerCount = opsErrorLogMaxWorkerCount
	}

	queueSize = workerCount * opsErrorLogQueueSizePerWorker
	if queueSize < opsErrorLogMinQueueSize {
		queueSize = opsErrorLogMinQueueSize
	}
	if queueSize > opsErrorLogMaxQueueSize {
		queueSize = opsErrorLogMaxQueueSize
	}

	return workerCount, queueSize
}

func setOpsRequestContext(c *gin.Context, model string, stream bool) {
	c.Set(opsModelKey, model)
	c.Set(opsStreamKey, stream)
}

func recordOpsError(c *gin.Context, ops *service.OpsService, status int, errType, message, fallbackPlatform string, streamInterrupted bool) {
	if ops == nil || c == nil {
		return
	}

	model, _ := c.Get(opsModelKey)
	stream, _ := c.Get(opsStreamKey)

	var modelName string
	if m, ok := model.(string); ok {
		modelName = m
	}
	streaming, _ := stream.(bool)

	apiKey, _ := middleware2.GetAPIKeyFromContext(c)

	logEntry := &service.OpsErrorLog{
		Phase:      classifyOpsPhase(errType, message),
		Type:       errType,
		Severity:   classifyOpsSeverity(errType, status),
		StatusCode: status,
		Platform:   resolveOpsPlatform(apiKey, fallbackPlatform),
		Model:      modelName,
		RequestID:  c.Writer.Header().Get("x-request-id"),
		Message:    message,
		ClientIP:   c.ClientIP(),
		RequestPath: func() string {
			if c.Request != nil && c.Request.URL != nil {
				return c.Request.URL.Path
			}
			return ""
		}(),
		Stream: streaming,
	}
	if c.Request != nil {
		logEntry.RetryCount = getOpsRetryCountFromContext(c.Request.Context())
	}
	logEntry.IsRetryable = classifyOpsIsRetryable(errType, status)
	logEntry.CompletionStatus = classifyOpsCompletionStatus(status, streaming, streamInterrupted)

	if apiKey != nil {
		logEntry.APIKeyID = &apiKey.ID
		if apiKey.User != nil {
			logEntry.UserID = &apiKey.User.ID
		}
		if apiKey.GroupID != nil {
			logEntry.GroupID = apiKey.GroupID
		}
	}

	enqueueOpsErrorLog(ops, logEntry)
}

func resolveOpsPlatform(apiKey *service.APIKey, fallback string) string {
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.Platform != "" {
		return apiKey.Group.Platform
	}
	return fallback
}

func classifyOpsPhase(errType, message string) string {
	msg := strings.ToLower(message)
	switch errType {
	case "authentication_error":
		return "auth"
	case "billing_error", "subscription_error":
		return "billing"
	case "rate_limit_error":
		if strings.Contains(msg, "concurrency") || strings.Contains(msg, "pending") {
			return "concurrency"
		}
		return "upstream"
	case "invalid_request_error":
		return "response"
	case "upstream_error", "overloaded_error":
		return "upstream"
	case "api_error":
		if strings.Contains(msg, "no available accounts") {
			return "scheduling"
		}
		return "internal"
	default:
		return "internal"
	}
}

func classifyOpsSeverity(errType string, status int) string {
	switch errType {
	case "invalid_request_error", "authentication_error", "billing_error", "subscription_error":
		return "P3"
	}
	if status >= 500 {
		return "P1"
	}
	if status == 429 {
		return "P1"
	}
	if status >= 400 {
		return "P2"
	}
	return "P3"
}

func getOpsRetryCountFromContext(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	v := ctx.Value(ctxkey.RetryCount)
	switch n := v.(type) {
	case int:
		if n < 0 {
			return 0
		}
		return n
	case int64:
		if n < 0 {
			return 0
		}
		return int(n)
	default:
		return 0
	}
}

func classifyOpsIsRetryable(errType string, statusCode int) bool {
	switch errType {
	case "authentication_error", "invalid_request_error":
		return false
	case "timeout_error", "rate_limit_error":
		return true
	case "upstream_error":
		return statusCode >= 500
	default:
		return statusCode >= 500
	}
}

func classifyOpsCompletionStatus(statusCode int, streaming bool, streamInterrupted bool) string {
	if statusCode < 400 {
		return "success"
	}
	if streaming && streamInterrupted {
		return "partial"
	}
	return "failed"
}
