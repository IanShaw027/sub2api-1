package service

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

const (
	opsUpstreamFailureQueueSize  = 8192
	opsUpstreamFailureBatchSize  = 200
	opsUpstreamFailureFlushEvery = 500 * time.Millisecond
)

func bindOpsUpstreamFailureSink(
	sink OpsUpstreamFailureSink,
	gateway *GatewayService,
	openAI *OpenAIGatewayService,
	gemini *GeminiMessagesCompatService,
	antigravity *AntigravityGatewayService,
) {
	if sink == nil {
		return
	}
	bindHTTP := func(upstream HTTPUpstream) {
		if upstream == nil {
			return
		}
		if setter, ok := upstream.(HTTPUpstreamFailureSinkSetter); ok {
			setter.SetOpsUpstreamFailureSink(sink)
		}
	}
	if gateway != nil {
		bindHTTP(gateway.httpUpstream)
	}
	if openAI != nil {
		bindHTTP(openAI.httpUpstream)
		openAI.SetOpsUpstreamFailureSink(sink)
	}
	if gemini != nil {
		bindHTTP(gemini.httpUpstream)
	}
	if antigravity != nil {
		bindHTTP(antigravity.httpUpstream)
	}
}

// SetOpsUpstreamFailureSink wires both request-bound WS attempts and the
// background WS pool into the same per-attempt error stream as HTTP upstreams.
func (s *OpenAIGatewayService) SetOpsUpstreamFailureSink(sink OpsUpstreamFailureSink) {
	if s == nil {
		return
	}
	s.opsUpstreamFailureSinkMu.Lock()
	s.opsUpstreamFailureSink = sink
	s.opsUpstreamFailureSinkMu.Unlock()
	s.openaiWSPoolMu.RLock()
	pool := s.openaiWSPool
	s.openaiWSPoolMu.RUnlock()
	if pool != nil {
		pool.setOpsUpstreamFailureSink(sink)
	}
}

func (s *OpenAIGatewayService) reportOpsUpstreamFailure(ctx context.Context, failure OpsUpstreamFailure) {
	if s == nil {
		return
	}
	s.opsUpstreamFailureSinkMu.RLock()
	sink := s.opsUpstreamFailureSink
	s.opsUpstreamFailureSinkMu.RUnlock()
	if sink != nil {
		sink.EnqueueOpsUpstreamFailure(ctx, failure)
	}
}

func (s *OpsService) EnqueueOpsUpstreamFailure(ctx context.Context, failure OpsUpstreamFailure) {
	if s == nil {
		return
	}
	entry := buildOpsUpstreamFailureEntry(ctx, failure)
	if entry == nil {
		return
	}
	s.upstreamFailureQueueOnce.Do(func() {
		s.upstreamFailureQueue = make(chan *OpsInsertErrorLogInput, opsUpstreamFailureQueueSize)
		go s.runOpsUpstreamFailureQueue()
	})
	select {
	case s.upstreamFailureQueue <- entry:
	default:
		// Preserve the every-attempt contract under a burst. This path is only
		// reached after 8192 queued failures, so a bounded synchronous fallback
		// is preferable to silently losing the diagnostic event.
		writeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.RecordError(writeCtx, entry, nil)
	}
}

func (s *OpsService) runOpsUpstreamFailureQueue() {
	ticker := time.NewTicker(opsUpstreamFailureFlushEvery)
	defer ticker.Stop()
	batch := make([]*OpsInsertErrorLogInput, 0, opsUpstreamFailureBatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.RecordErrorBatch(writeCtx, batch)
		cancel()
		batch = batch[:0]
	}
	for {
		select {
		case entry := <-s.upstreamFailureQueue:
			if entry != nil {
				batch = append(batch, entry)
			}
			if len(batch) >= opsUpstreamFailureBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func buildOpsUpstreamFailureEntry(ctx context.Context, failure OpsUpstreamFailure) *OpsInsertErrorLogInput {
	if failure.Err == nil && failure.StatusCode < http.StatusBadRequest {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	platform := strings.TrimSpace(failure.Platform)
	if platform == "" {
		platform = inferOpsPlatformFromUpstreamURL(failure.URL)
	}
	method := strings.ToUpper(strings.TrimSpace(failure.Method))
	upstreamURL := safeUpstreamURL(failure.URL)
	upstreamEndpoint := upstreamPath(failure.URL)

	errorType := "upstream_error"
	message := http.StatusText(failure.StatusCode)
	detailText := ""
	statusCode := failure.StatusCode
	if failure.Err != nil {
		detail := classifyUpstreamTransportError(failure.Err)
		errorType = detail.ErrorType
		message = detail.Message
		detailText = detail.Detail
		statusCode = http.StatusBadGateway
	} else {
		switch failure.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			errorType = "authentication_error"
		case http.StatusTooManyRequests:
			errorType = "rate_limit_error"
		default:
			if failure.StatusCode >= http.StatusInternalServerError {
				errorType = "upstream_error"
			} else {
				errorType = "invalid_request_error"
			}
		}
	}
	if strings.TrimSpace(message) == "" {
		message = "Upstream request failed"
	}
	if method != "" && upstreamEndpoint != "" {
		message = method + " " + upstreamEndpoint + ": " + message
	}

	var accountID *int64
	if failure.AccountID > 0 {
		id := failure.AccountID
		accountID = &id
	}
	var upstreamStatus *int
	if failure.StatusCode >= http.StatusBadRequest {
		code := failure.StatusCode
		upstreamStatus = &code
	}
	upstreamMessage := strings.TrimSpace(message)
	upstreamDetail := strings.TrimSpace(detailText)
	upstreamResponseBody := strings.TrimSpace(failure.ResponseBody)
	if upstreamDetail == "" {
		upstreamDetail = upstreamResponseBody
	}
	event := &OpsUpstreamErrorEvent{
		AtUnixMs:             time.Now().UnixMilli(),
		Platform:             platform,
		AccountID:            failure.AccountID,
		UpstreamStatusCode:   failure.StatusCode,
		UpstreamURL:          upstreamURL,
		Kind:                 strings.TrimSpace(failure.Kind),
		Message:              upstreamMessage,
		Detail:               upstreamDetail,
		UpstreamResponseBody: upstreamResponseBody,
	}
	entry := &OpsInsertErrorLogInput{
		RequestID:            stringContextValue(ctx, ctxkey.RequestID),
		ClientRequestID:      stringContextValue(ctx, ctxkey.ClientRequestID),
		AccountID:            accountID,
		Platform:             platform,
		RequestPath:          upstreamEndpoint,
		UpstreamEndpoint:     upstreamEndpoint,
		ErrorPhase:           "upstream",
		ErrorType:            errorType,
		Severity:             "P2",
		StatusCode:           statusCode,
		IsRetryable:          failure.Err != nil || failure.StatusCode == http.StatusTooManyRequests || failure.StatusCode >= http.StatusInternalServerError,
		ErrorMessage:         upstreamMessage,
		ErrorBody:            upstreamResponseBody,
		ErrorSource:          "upstream_http",
		ErrorOwner:           "provider",
		UpstreamStatusCode:   upstreamStatus,
		UpstreamErrorMessage: &upstreamMessage,
		UpstreamErrors:       []*OpsUpstreamErrorEvent{event},
		CreatedAt:            time.Now(),
	}
	if upstreamDetail != "" {
		entry.UpstreamErrorDetail = &upstreamDetail
	}
	if failure.Err != nil {
		entry.ErrorSource = "gateway"
		if errors.Is(failure.Err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
			entry.ErrorOwner = "client"
		}
	}
	return entry
}

func stringContextValue(ctx context.Context, key ctxkey.Key) string {
	value, _ := ctx.Value(key).(string)
	return strings.TrimSpace(value)
}

func upstreamPath(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return ""
	}
	return strings.TrimSpace(parsed.Path)
}

func inferOpsPlatformFromUpstreamURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case strings.Contains(host, "openai.com"), strings.Contains(host, "chatgpt.com"):
		return PlatformOpenAI
	case strings.Contains(host, "x.ai"), strings.Contains(host, "grok.com"):
		return PlatformGrok
	case strings.Contains(host, "cloudcode-pa"):
		return PlatformAntigravity
	case strings.Contains(host, "googleapis.com"), strings.Contains(host, "google.com"):
		return PlatformGemini
	case strings.Contains(host, "anthropic.com"):
		return PlatformAnthropic
	case strings.HasPrefix(host, "q.") && strings.HasSuffix(host, ".amazonaws.com"):
		return PlatformKiro
	case strings.Contains(host, "bedrock-runtime") && strings.HasSuffix(host, ".amazonaws.com"):
		return PlatformAnthropic
	default:
		return ""
	}
}
