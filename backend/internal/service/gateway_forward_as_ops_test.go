package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type queuedGatewayOpsAttempt struct {
	statusCode int
	requestID  string
	body       string
}

type queuedGatewayOpsUpstream struct {
	attempts []queuedGatewayOpsAttempt
	delay    time.Duration
	calls    int
}

func (u *queuedGatewayOpsUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *queuedGatewayOpsUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.calls++
	if u.delay > 0 {
		time.Sleep(u.delay)
	}
	idx := u.calls - 1
	if idx >= len(u.attempts) {
		idx = len(u.attempts) - 1
	}
	attempt := u.attempts[idx]
	return &http.Response{
		StatusCode: attempt.statusCode,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{attempt.requestID},
		},
		Body: io.NopCloser(strings.NewReader(attempt.body)),
	}, nil
}

func TestGatewayService_ForwardAsChatCompletions_RecordsOpsContextAndLatency(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-Id": []string{"rid-forward-cc"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"claude model not found","type":"invalid_request_error"}}`)),
		},
	}

	svc := &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				LogUpstreamErrorBody:         true,
				LogUpstreamErrorBodyMaxBytes: 1024,
			},
		},
		httpUpstream: upstream,
	}

	account := newAnthropicAPIKeyAccountForTest()

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(`{"model":"claude-3-7-sonnet-20250219","messages":[{"role":"user","content":"hello"}],"stream":false}`), &ParsedRequest{
		Model:  "claude-3-7-sonnet-20250219",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	latencyValue, ok := c.Get(OpsUpstreamLatencyMsKey)
	require.True(t, ok)
	require.IsType(t, int64(0), latencyValue)

	rawBody, ok := c.Get(OpsUpstreamRequestBodyKey)
	require.True(t, ok)
	bodyBytes, ok := rawBody.([]byte)
	require.True(t, ok)
	require.Contains(t, string(bodyBytes), `"claude-3-7-sonnet-20250219"`)

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "http_error", events[0].Kind)
	require.Equal(t, http.StatusBadRequest, events[0].UpstreamStatusCode)
	require.Equal(t, "rid-forward-cc", events[0].UpstreamRequestID)
	require.Contains(t, events[0].UpstreamURL, "/v1/messages")
	require.Contains(t, events[0].Message, "model not found")
}

func TestGatewayService_ForwardAsResponses_RecordsOpsContextAndLatency(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-Id": []string{"rid-forward-responses"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"responses request invalid","type":"invalid_request_error"}}`)),
		},
	}

	svc := &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				LogUpstreamErrorBody:         true,
				LogUpstreamErrorBodyMaxBytes: 1024,
			},
		},
		httpUpstream: upstream,
	}

	account := newAnthropicAPIKeyAccountForTest()

	result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"claude-3-7-sonnet-20250219","input":"hello","stream":false}`), &ParsedRequest{
		Model:  "claude-3-7-sonnet-20250219",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	latencyValue, ok := c.Get(OpsUpstreamLatencyMsKey)
	require.True(t, ok)
	require.IsType(t, int64(0), latencyValue)

	rawBody, ok := c.Get(OpsUpstreamRequestBodyKey)
	require.True(t, ok)
	bodyBytes, ok := rawBody.([]byte)
	require.True(t, ok)
	require.Contains(t, string(bodyBytes), `"claude-3-7-sonnet-20250219"`)

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "http_error", events[0].Kind)
	require.Equal(t, http.StatusBadRequest, events[0].UpstreamStatusCode)
	require.Equal(t, "rid-forward-responses", events[0].UpstreamRequestID)
	require.Contains(t, events[0].UpstreamURL, "/v1/messages")
	require.Contains(t, events[0].Message, "responses request invalid")
}

func TestGatewayService_Forward_AccumulatesOpsUpstreamLatencyAcrossRetryAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &queuedGatewayOpsUpstream{
		delay: 5 * time.Millisecond,
		attempts: []queuedGatewayOpsAttempt{
			{
				statusCode: http.StatusServiceUnavailable,
				requestID:  "rid-forward-1",
				body:       `{"error":{"message":"temporary overload","type":"overloaded_error"}}`,
			},
			{
				statusCode: http.StatusBadRequest,
				requestID:  "rid-forward-2",
				body:       `{"error":{"message":"final invalid request","type":"invalid_request_error"}}`,
			},
		},
	}

	svc := &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				MaxLineSize:                  defaultMaxLineSize,
				LogUpstreamErrorBody:         true,
				LogUpstreamErrorBodyMaxBytes: 1024,
			},
		},
		httpUpstream:     upstream,
		rateLimitService: &RateLimitService{},
	}

	account := &Account{
		ID:          401,
		Name:        "anthropic-forward-retry",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":                    "upstream-anthropic-key",
			"base_url":                   "https://api.anthropic.com",
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(http.StatusBadRequest)},
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
		Body:   []byte(`{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hello"}],"stream":false}`),
		Model:  "claude-3-5-sonnet-20241022",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, 2, upstream.calls, "should include one retry attempt")

	latencyValue, ok := c.Get(OpsUpstreamLatencyMsKey)
	require.True(t, ok)
	latencyMs, ok := latencyValue.(int64)
	require.True(t, ok)
	require.GreaterOrEqual(t, latencyMs, int64(8), "latency should accumulate multiple upstream attempts")

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 2)
	require.Equal(t, "retry", events[0].Kind)
	require.Equal(t, http.StatusServiceUnavailable, events[0].UpstreamStatusCode)
	require.Equal(t, "http_error", events[1].Kind)
	require.Equal(t, http.StatusBadRequest, events[1].UpstreamStatusCode)
}

func TestGatewayService_Forward_NativeMessagesHTTPErrorEventIncludesAccountAndUpstreamURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &queuedGatewayOpsUpstream{
		attempts: []queuedGatewayOpsAttempt{
			{
				statusCode: http.StatusBadRequest,
				requestID:  "rid-forward-http-error",
				body:       `{"error":{"message":"invalid request from upstream","type":"invalid_request_error"}}`,
			},
		},
	}

	svc := &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				MaxLineSize:                  defaultMaxLineSize,
				LogUpstreamErrorBody:         true,
				LogUpstreamErrorBodyMaxBytes: 1024,
			},
		},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:          402,
		Name:        "anthropic-forward-http-error",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "upstream-anthropic-key",
			"base_url": "https://api.anthropic.com",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
		Body:   []byte(`{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hello"}],"stream":false}`),
		Model:  "claude-3-5-sonnet-20241022",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "http_error", events[0].Kind)
	require.Equal(t, account.Name, events[0].AccountName)
	require.Contains(t, events[0].UpstreamURL, "/v1/messages")
}

func TestGatewayService_Forward_NativeMessagesFailoverEventIncludesAccountAndUpstreamURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &queuedGatewayOpsUpstream{
		attempts: []queuedGatewayOpsAttempt{
			{
				statusCode: http.StatusServiceUnavailable,
				requestID:  "rid-forward-failover",
				body:       `{"error":{"message":"upstream unavailable","type":"overloaded_error"}}`,
			},
		},
	}

	svc := &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				MaxLineSize:                  defaultMaxLineSize,
				LogUpstreamErrorBody:         true,
				LogUpstreamErrorBodyMaxBytes: 1024,
			},
		},
		httpUpstream:     upstream,
		rateLimitService: &RateLimitService{},
	}

	account := &Account{
		ID:          403,
		Name:        "anthropic-forward-failover",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "upstream-anthropic-key",
			"base_url": "https://api.anthropic.com",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
		Body:   []byte(`{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hello"}],"stream":false}`),
		Model:  "claude-3-5-sonnet-20241022",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "failover", events[0].Kind)
	require.Equal(t, account.Name, events[0].AccountName)
	require.Contains(t, events[0].UpstreamURL, "/v1/messages")
}

func TestGatewayService_Forward_AnthropicPassthroughRecordsLatencyAndFailoverFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &queuedGatewayOpsUpstream{
		delay: 5 * time.Millisecond,
		attempts: []queuedGatewayOpsAttempt{
			{
				statusCode: http.StatusServiceUnavailable,
				requestID:  "rid-pass-failover",
				body:       `{"error":{"message":"upstream unavailable","type":"overloaded_error"}}`,
			},
		},
	}

	svc := &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				MaxLineSize:                  defaultMaxLineSize,
				LogUpstreamErrorBody:         true,
				LogUpstreamErrorBodyMaxBytes: 1024,
			},
		},
		httpUpstream:     upstream,
		rateLimitService: &RateLimitService{},
	}

	account := newAnthropicAPIKeyAccountForTest()
	account.ID = 404
	account.Name = "anthropic-passthrough-failover"

	result, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
		Body:   []byte(`{"model":"claude-3-7-sonnet-20250219","messages":[{"role":"user","content":"hello"}],"stream":false}`),
		Model:  "claude-3-7-sonnet-20250219",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)

	latencyValue, ok := c.Get(OpsUpstreamLatencyMsKey)
	require.True(t, ok)
	latencyMs, ok := latencyValue.(int64)
	require.True(t, ok)
	require.GreaterOrEqual(t, latencyMs, int64(4))

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "failover", events[0].Kind)
	require.Equal(t, account.Name, events[0].AccountName)
	require.Contains(t, events[0].UpstreamURL, "/v1/messages")
}

func TestGatewayService_Forward_BedrockRecordsLatencyAndFailoverFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &queuedGatewayOpsUpstream{
		delay: 5 * time.Millisecond,
		attempts: []queuedGatewayOpsAttempt{
			{
				statusCode: http.StatusServiceUnavailable,
				requestID:  "rid-bedrock-failover",
				body:       `{"error":{"message":"bedrock unavailable","type":"overloaded_error"}}`,
			},
		},
	}

	svc := &GatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				MaxLineSize:                  defaultMaxLineSize,
				LogUpstreamErrorBody:         true,
				LogUpstreamErrorBodyMaxBytes: 1024,
			},
		},
		httpUpstream:     upstream,
		rateLimitService: &RateLimitService{},
	}

	account := &Account{
		ID:          405,
		Name:        "bedrock-failover",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeBedrock,
		Concurrency: 1,
		Credentials: map[string]any{
			"auth_mode":  "apikey",
			"api_key":    "bedrock-api-key",
			"aws_region": "us-east-1",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
		Body:   []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"stream":false}`),
		Model:  "claude-sonnet-4-5",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)

	latencyValue, ok := c.Get(OpsUpstreamLatencyMsKey)
	require.True(t, ok)
	latencyMs, ok := latencyValue.(int64)
	require.True(t, ok)
	require.GreaterOrEqual(t, latencyMs, int64(4))

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "failover", events[0].Kind)
	require.Equal(t, account.Name, events[0].AccountName)
	require.Contains(t, events[0].UpstreamURL, "bedrock-runtime")
	require.Contains(t, events[0].UpstreamURL, "/invoke")
}
