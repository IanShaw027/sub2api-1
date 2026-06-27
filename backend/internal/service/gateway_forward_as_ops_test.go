package service

import (
	"context"
	"encoding/json"
	"errors"
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
	err        error
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
	if attempt.err != nil {
		return nil, attempt.err
	}
	return &http.Response{
		StatusCode: attempt.statusCode,
		Request:    req,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{attempt.requestID},
		},
		Body: io.NopCloser(strings.NewReader(attempt.body)),
	}, nil
}

func TestGatewayService_ForwardAsChatCompletions_RecordsOpsContextAndLatency(t *testing.T) {
	setGinTestMode()

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
	setGinTestMode()

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

func TestGatewayService_ForwardAsChatCompletions_OAuthIgnoresCredentialModelMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-Id": []string{"rid-forward-cc-oauth"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"stop after recording request","type":"invalid_request_error"}}`)),
		},
	}
	svc := &GatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := newAnthropicOAuthAccountWithCredentialModelMappingForTest()

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(`{"model":"claude-3-7-sonnet-20250219","messages":[{"role":"user","content":"hello"}],"stream":false}`), &ParsedRequest{
		Model:  "claude-3-7-sonnet-20250219",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "claude-3-7-sonnet-20250219", decodeRecordedAnthropicModel(t, upstream.lastBody))
}

func TestGatewayService_ForwardAsResponses_OAuthIgnoresCredentialModelMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-Id": []string{"rid-forward-responses-oauth"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"stop after recording request","type":"invalid_request_error"}}`)),
		},
	}
	svc := &GatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := newAnthropicOAuthAccountWithCredentialModelMappingForTest()

	result, err := svc.ForwardAsResponses(context.Background(), c, account, []byte(`{"model":"claude-3-7-sonnet-20250219","input":"hello","stream":false}`), &ParsedRequest{
		Model:  "claude-3-7-sonnet-20250219",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "claude-3-7-sonnet-20250219", decodeRecordedAnthropicModel(t, upstream.lastBody))
}

func newAnthropicOAuthAccountWithCredentialModelMappingForTest() *Account {
	return &Account{
		ID:          202,
		Name:        "anthropic-oauth-forward",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":    "oauth-token",
			"model_mapping":   map[string]any{"claude-3-7-sonnet-20250219": "claude-3-haiku-20240307"},
			"refresh_token":   "refresh-token",
			"expires_at":      "2999-01-01T00:00:00Z",
			"subscription":    "max",
			"organization_id": "org-test",
		},
		Status:      StatusActive,
		Schedulable: true,
	}
}

func decodeRecordedAnthropicModel(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Model string `json:"model"`
	}
	require.NoError(t, json.Unmarshal(body, &payload))
	return payload.Model
}

func TestGatewayService_Forward_AccumulatesOpsUpstreamLatencyAcrossRetryAttempts(t *testing.T) {
	setGinTestMode()

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
		Body:   NewRequestBodyRef([]byte(`{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hello"}],"stream":false}`)),
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
	setGinTestMode()

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
		Body:   NewRequestBodyRef([]byte(`{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hello"}],"stream":false}`)),
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
	setGinTestMode()

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
		Body:   NewRequestBodyRef([]byte(`{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hello"}],"stream":false}`)),
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
	setGinTestMode()

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
		Body:   NewRequestBodyRef([]byte(`{"model":"claude-3-7-sonnet-20250219","messages":[{"role":"user","content":"hello"}],"stream":false}`)),
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
	setGinTestMode()

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
		Body:   NewRequestBodyRef([]byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"stream":false}`)),
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

func TestGatewayService_Forward_NativeMessagesSignatureRetryFinalHTTPErrorUsesFilteredBody(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &queuedGatewayOpsUpstream{
		attempts: []queuedGatewayOpsAttempt{
			{
				statusCode: http.StatusBadRequest,
				requestID:  "rid-signature-original",
				body:       `{"error":{"message":"Invalid \u0060signature\u0060 in \u0060thinking\u0060 block","type":"invalid_request_error"}}`,
			},
			{
				statusCode: http.StatusBadRequest,
				requestID:  "rid-signature-filtered",
				body:       `{"error":{"message":"filtered retry invalid request","type":"invalid_request_error"}}`,
			},
		},
	}

	rectifierJSON, err := json.Marshal(&RectifierSettings{
		Enabled:                true,
		APIKeySignatureEnabled: true,
	})
	require.NoError(t, err)

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
		settingService: &SettingService{settingRepo: &countTokensSettingRepoStub{
			values: map[string]string{
				SettingKeyRectifierSettings: string(rectifierJSON),
			},
		}},
	}

	account := newAnthropicAPIKeyAccountForTest()
	account.ID = 406
	account.Name = "anthropic-native-signature-retry-final"
	account.Extra = nil

	originalBody := []byte(`{"model":"claude-3-5-sonnet","thinking":{"type":"enabled","budget_tokens":1200},"messages":[{"role":"user","content":[{"type":"thinking","thinking":"chain","signature":"bad-sig"},{"type":"text","text":"hello"}]}],"stream":false}`)
	result, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
		Body:   NewRequestBodyRef(originalBody),
		Model:  "claude-3-5-sonnet",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, 2, upstream.calls, "signature rectifier should perform one retry attempt")

	filteredBody := strings.TrimSpace(string(FilterThinkingBlocksForRetry(originalBody)))

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 2)
	require.Equal(t, "signature_error", events[0].Kind)
	require.Equal(t, strings.TrimSpace(string(originalBody)), events[0].UpstreamRequestBody)
	require.Equal(t, "http_error", events[1].Kind)
	require.Equal(t, "rid-signature-filtered", events[1].UpstreamRequestID)
	require.Equal(t, filteredBody, events[1].UpstreamRequestBody)

	rawBody, ok := c.Get(OpsUpstreamRequestBodyKey)
	require.True(t, ok)
	bodyBytes, ok := rawBody.([]byte)
	require.True(t, ok)
	require.Equal(t, filteredBody, strings.TrimSpace(string(bodyBytes)))
}

func TestGatewayService_Forward_NativeMessagesSignatureRetryRequestErrorRecordsFilteredBody(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &queuedGatewayOpsUpstream{
		attempts: []queuedGatewayOpsAttempt{
			{
				statusCode: http.StatusBadRequest,
				requestID:  "rid-signature-original",
				body:       `{"error":{"message":"Invalid \u0060signature\u0060 in \u0060thinking\u0060 block","type":"invalid_request_error"}}`,
			},
			{
				err: errors.New("dial tcp: thinking retry failed"),
			},
		},
	}

	rectifierJSON, err := json.Marshal(&RectifierSettings{
		Enabled:                true,
		APIKeySignatureEnabled: true,
	})
	require.NoError(t, err)

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
		settingService: &SettingService{settingRepo: &countTokensSettingRepoStub{
			values: map[string]string{
				SettingKeyRectifierSettings: string(rectifierJSON),
			},
		}},
	}

	account := newAnthropicAPIKeyAccountForTest()
	account.ID = 408
	account.Name = "anthropic-native-signature-retry-request-error"
	account.Extra = nil

	originalBody := []byte(`{"model":"claude-3-5-sonnet","thinking":{"type":"enabled","budget_tokens":1200},"messages":[{"role":"user","content":[{"type":"thinking","thinking":"chain","signature":"bad-sig"},{"type":"text","text":"hello"}]}],"stream":false}`)
	result, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
		Body:   NewRequestBodyRef(originalBody),
		Model:  "claude-3-5-sonnet",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, 2, upstream.calls, "signature rectifier should perform one retry attempt")

	filteredBody := strings.TrimSpace(string(FilterThinkingBlocksForRetry(originalBody)))

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 3)
	require.Equal(t, "signature_error", events[0].Kind)
	require.Equal(t, strings.TrimSpace(string(originalBody)), events[0].UpstreamRequestBody)
	require.Equal(t, "signature_retry_request_error", events[1].Kind)
	require.Equal(t, filteredBody, events[1].UpstreamRequestBody)
	require.Contains(t, events[1].Message, "thinking retry failed")
	require.Equal(t, "http_error", events[2].Kind)
	require.Equal(t, filteredBody, events[2].UpstreamRequestBody)

	rawBody, ok := c.Get(OpsUpstreamRequestBodyKey)
	require.True(t, ok)
	bodyBytes, ok := rawBody.([]byte)
	require.True(t, ok)
	require.Equal(t, filteredBody, strings.TrimSpace(string(bodyBytes)))
}

func TestGatewayService_Forward_NativeMessagesBudgetRetryFinalHTTPErrorUsesRectifiedBody(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &queuedGatewayOpsUpstream{
		attempts: []queuedGatewayOpsAttempt{
			{
				statusCode: http.StatusBadRequest,
				requestID:  "rid-budget-original",
				body:       `{"error":{"message":"thinking.budget_tokens input should be greater than or equal to 1024","type":"invalid_request_error"}}`,
			},
			{
				statusCode: http.StatusBadRequest,
				requestID:  "rid-budget-rectified",
				body:       `{"error":{"message":"rectified budget retry still invalid","type":"invalid_request_error"}}`,
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
		settingService:   &SettingService{settingRepo: &countTokensSettingRepoStub{}},
	}

	account := newAnthropicAPIKeyAccountForTest()
	account.ID = 409
	account.Name = "anthropic-native-budget-retry-final"
	account.Extra = nil

	originalBody := []byte(`{"model":"claude-3-5-sonnet","thinking":{"type":"enabled","budget_tokens":1200},"max_tokens":1000,"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"stream":false}`)
	result, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
		Body:   NewRequestBodyRef(originalBody),
		Model:  "claude-3-5-sonnet",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, 2, upstream.calls, "budget rectifier should perform one retry attempt")

	rectifiedBody, applied := RectifyThinkingBudget(originalBody)
	require.True(t, applied)
	rectifiedBodyString := strings.TrimSpace(string(rectifiedBody))

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 2)
	require.Equal(t, "budget_constraint_error", events[0].Kind)
	require.Equal(t, strings.TrimSpace(string(originalBody)), events[0].UpstreamRequestBody)
	require.Equal(t, "http_error", events[1].Kind)
	require.Equal(t, "rid-budget-rectified", events[1].UpstreamRequestID)
	require.Equal(t, rectifiedBodyString, events[1].UpstreamRequestBody)

	rawBody, ok := c.Get(OpsUpstreamRequestBodyKey)
	require.True(t, ok)
	bodyBytes, ok := rawBody.([]byte)
	require.True(t, ok)
	require.Equal(t, rectifiedBodyString, strings.TrimSpace(string(bodyBytes)))
}

func TestGatewayService_Forward_NativeMessagesToolDowngradeRetryRequestErrorUsesToolFilteredBody(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	upstream := &queuedGatewayOpsUpstream{
		attempts: []queuedGatewayOpsAttempt{
			{
				statusCode: http.StatusBadRequest,
				requestID:  "rid-signature-original",
				body:       `{"error":{"message":"Invalid \u0060signature\u0060 in \u0060thinking\u0060 block","type":"invalid_request_error"}}`,
			},
			{
				statusCode: http.StatusBadRequest,
				requestID:  "rid-signature-thinking-retry",
				body:       `{"error":{"message":"Invalid signature in tool_use block","type":"invalid_request_error"}}`,
			},
			{
				err: errors.New("dial tcp: tool downgrade retry failed"),
			},
		},
	}

	rectifierJSON, err := json.Marshal(&RectifierSettings{
		Enabled:                true,
		APIKeySignatureEnabled: true,
	})
	require.NoError(t, err)

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
		settingService: &SettingService{settingRepo: &countTokensSettingRepoStub{
			values: map[string]string{
				SettingKeyRectifierSettings: string(rectifierJSON),
			},
		}},
	}

	account := newAnthropicAPIKeyAccountForTest()
	account.ID = 407
	account.Name = "anthropic-native-signature-tool-retry"
	account.Extra = nil

	originalBody := []byte(`{"model":"claude-3-5-sonnet","thinking":{"type":"enabled","budget_tokens":1200},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"chain","signature":"bad-sig"},{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"hello"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"world"}]}],"stream":false}`)
	result, err := svc.Forward(context.Background(), c, account, &ParsedRequest{
		Body:   NewRequestBodyRef(originalBody),
		Model:  "claude-3-5-sonnet",
		Stream: false,
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, 3, upstream.calls, "tool-downgrade signature rectifier should perform a second-stage retry")

	filteredThinkingBody := strings.TrimSpace(string(FilterThinkingBlocksForRetry(originalBody)))
	filteredToolsBody := strings.TrimSpace(string(FilterSignatureSensitiveBlocksForRetry(originalBody, "claude-3-5-sonnet")))

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 4)
	require.Equal(t, "signature_error", events[0].Kind)
	require.Equal(t, strings.TrimSpace(string(originalBody)), events[0].UpstreamRequestBody)
	require.Equal(t, "signature_retry_thinking", events[1].Kind)
	require.Equal(t, filteredThinkingBody, events[1].UpstreamRequestBody)
	require.Equal(t, "signature_retry_tools_request_error", events[2].Kind)
	require.Equal(t, filteredToolsBody, events[2].UpstreamRequestBody)
	require.Contains(t, events[2].Message, "tool downgrade retry failed")
	require.Equal(t, "http_error", events[3].Kind)
	require.Equal(t, "rid-signature-thinking-retry", events[3].UpstreamRequestID)
	require.Equal(t, filteredThinkingBody, events[3].UpstreamRequestBody)

	rawBody, ok := c.Get(OpsUpstreamRequestBodyKey)
	require.True(t, ok)
	bodyBytes, ok := rawBody.([]byte)
	require.True(t, ok)
	require.Equal(t, filteredThinkingBody, strings.TrimSpace(string(bodyBytes)))
}
