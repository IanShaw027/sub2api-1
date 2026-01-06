package infraerror

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	PhaseInternal  = "internal"
	SeverityP1     = "P1"
	TypeDBError    = "database_error"
	TypeRedisError = "redis_error"
	TypeOtherError = "internal_error"

	queueSize     = 256
	workerCount   = 2
	dedupeWindow  = 30 * time.Second
	maxErrMsgSize = 8 * 1024
)

type InfraError struct {
	Phase     string
	Type      string
	Severity  string
	Component string
	Operation string
	Err       error
}

func (e *InfraError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Type
	}
	return e.Err.Error()
}

func (e *InfraError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type entry struct {
	Timestamp string `json:"timestamp"`

	Phase     string `json:"phase"`
	ErrorType string `json:"error_type"`
	Severity  string `json:"severity"`

	Component string `json:"component"`
	Operation string `json:"operation"`
	Kind      string `json:"kind"`

	Message string `json:"message"`
}

var (
	startOnce sync.Once
	queue     chan entry

	lastLoggedMu sync.Mutex
	lastLogged   = make(map[string]time.Time)
)

type recordingDisabledKey struct{}

func WithRecordingDisabled(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Value(recordingDisabledKey{}) != nil {
		return ctx
	}
	return context.WithValue(ctx, recordingDisabledKey{}, true)
}

func RecordingDisabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	return ctx.Value(recordingDisabledKey{}) != nil
}

func startWorkers() {
	queue = make(chan entry, queueSize)
	for i := 0; i < workerCount; i++ {
		go func() {
			for e := range queue {
				b, err := json.Marshal(e)
				if err != nil {
					log.Printf("[InfraError] phase=%s type=%s component=%s operation=%s kind=%s message=%q",
						e.Phase, e.ErrorType, e.Component, e.Operation, e.Kind, e.Message,
					)
					continue
				}
				log.Printf("[InfraError] %s", string(b))
			}
		}()
	}
}

func shouldLog(key string, now time.Time) bool {
	lastLoggedMu.Lock()
	defer lastLoggedMu.Unlock()

	if t, ok := lastLogged[key]; ok && now.Sub(t) < dedupeWindow {
		return false
	}
	lastLogged[key] = now
	return true
}

func normalizeErrorType(component string) string {
	switch strings.ToLower(strings.TrimSpace(component)) {
	case "db", "database", "postgres", "postgresql", "mysql":
		return TypeDBError
	case "redis":
		return TypeRedisError
	default:
		return TypeOtherError
	}
}

func RecordInfrastructureError(ctx context.Context, component, operation string, err error) {
	if RecordingDisabled(ctx) {
		return
	}
	if !IsCriticalInfrastructureError(err) {
		return
	}

	startOnce.Do(startWorkers)

	now := time.Now().UTC()
	kind := classify(err)
	errorType := normalizeErrorType(component)

	key := errorType + "|" + strings.TrimSpace(component) + "|" + strings.TrimSpace(operation) + "|" + kind
	if !shouldLog(key, now) {
		return
	}

	msg := err.Error()
	if len(msg) > maxErrMsgSize {
		msg = msg[:maxErrMsgSize] + "...(truncated)"
	}

	item := entry{
		Timestamp: now.Format(time.RFC3339Nano),
		Phase:     PhaseInternal,
		ErrorType: errorType,
		Severity:  SeverityP1,
		Component: strings.TrimSpace(component),
		Operation: strings.TrimSpace(operation),
		Kind:      kind,
		Message:   msg,
	}

	select {
	case queue <- item:
	default:
		// Drop to avoid blocking hot paths.
	}

	_ = ctx // reserved for future correlation hooks (request_id, trace_id, etc.)
}

func IsCriticalInfrastructureError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, redis.Nil) {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, driver.ErrBadConn) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() || netErr.Temporary() {
			return true
		}
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "bad connection") ||
		strings.Contains(msg, "too many connections") ||
		strings.Contains(msg, "connection terminated") ||
		strings.Contains(msg, "server closed the connection") ||
		strings.Contains(msg, "eof")
}

func classify(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "too many connections") {
		return "conn_exhausted"
	}
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host") {
		return "conn_failed"
	}
	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, driver.ErrBadConn) {
		return "conn_bad"
	}
	return "internal"
}
