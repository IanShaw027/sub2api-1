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

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type countTokensSettingRepoStub struct {
	values map[string]string
}

func (s *countTokensSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *countTokensSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.values != nil {
		if value, ok := s.values[key]; ok {
			return value, nil
		}
	}
	return "", ErrSettingNotFound
}

func (s *countTokensSettingRepoStub) Set(ctx context.Context, key, value string) error {
	return nil
}

func (s *countTokensSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *countTokensSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	return nil
}

func (s *countTokensSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *countTokensSettingRepoStub) Delete(ctx context.Context, key string) error {
	return nil
}

type countTokensQueuedAttempt struct {
	resp *http.Response
	err  error
}

type countTokensQueuedUpstream struct {
	attempts []countTokensQueuedAttempt
	calls    int
}

func (u *countTokensQueuedUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *countTokensQueuedUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.calls++
	idx := u.calls - 1
	if idx >= len(u.attempts) {
		idx = len(u.attempts) - 1
	}
	attempt := u.attempts[idx]
	return attempt.resp, attempt.err
}

func TestGatewayService_ForwardCountTokens_RecordsOpsContextAndLatency(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)

	upstream := &anthropicHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-Id": []string{"rid-count-tokens"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"model not found: claude-missing","type":"invalid_request_error"}}`)),
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

	account := &Account{
		ID:          301,
		Name:        "anthropic-count-tokens",
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

	err := svc.ForwardCountTokens(context.Background(), c, account, &ParsedRequest{
		Body:  []byte(`{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`),
		Model: "claude-3-5-sonnet",
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	latencyValue, ok := c.Get(OpsUpstreamLatencyMsKey)
	require.True(t, ok)
	require.IsType(t, int64(0), latencyValue)

	rawBody, ok := c.Get(OpsUpstreamRequestBodyKey)
	require.True(t, ok)
	bodyBytes, ok := rawBody.([]byte)
	require.True(t, ok)
	require.Contains(t, string(bodyBytes), `"claude-3-5-sonnet"`)

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "http_error", events[0].Kind)
	require.Equal(t, http.StatusBadRequest, events[0].UpstreamStatusCode)
	require.Equal(t, "rid-count-tokens", events[0].UpstreamRequestID)
	require.Contains(t, events[0].UpstreamURL, "/v1/messages/count_tokens")
	require.Contains(t, events[0].Message, "model not found")
}

func TestGatewayService_ForwardCountTokens_SignatureRetryTransportFailureKeepsBodyContext(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", nil)

	upstream := &countTokensQueuedUpstream{
		attempts: []countTokensQueuedAttempt{
			{
				resp: &http.Response{
					StatusCode: http.StatusBadRequest,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
						"X-Request-Id": []string{"rid-signature-1"},
					},
					Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Invalid \u0060signature\u0060 in \u0060thinking\u0060 block","type":"invalid_request_error"}}`)),
				},
			},
			{
				err: errors.New("dial tcp: retry transport failed"),
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

	account := &Account{
		ID:          302,
		Name:        "anthropic-count-tokens-signature-retry",
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

	originalBody := []byte(`{"model":"claude-3-5-sonnet","thinking":{"type":"enabled","budget_tokens":1200},"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	err = svc.ForwardCountTokens(context.Background(), c, account, &ParsedRequest{
		Body:  originalBody,
		Model: "claude-3-5-sonnet",
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, 2, upstream.calls, "count_tokens signature rectifier should perform one retry attempt")

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 2)

	require.Equal(t, "signature_retry_request_error", events[0].Kind)
	require.Equal(t, 0, events[0].UpstreamStatusCode)
	require.Contains(t, events[0].Message, "retry transport failed")
	require.Contains(t, events[0].UpstreamURL, "/v1/messages/count_tokens")

	require.Equal(t, "http_error", events[1].Kind)
	require.Equal(t, http.StatusBadRequest, events[1].UpstreamStatusCode)
	require.Equal(t, "rid-signature-1", events[1].UpstreamRequestID)
	require.Contains(t, events[1].Message, "signature")
	require.Contains(t, events[1].UpstreamRequestBody, `"thinking":{"type":"enabled","budget_tokens":1200}`)

	rawBody, ok := c.Get(OpsUpstreamRequestBodyKey)
	require.True(t, ok)
	bodyBytes, ok := rawBody.([]byte)
	require.True(t, ok)
	require.Contains(t, string(bodyBytes), `"thinking":{"type":"enabled","budget_tokens":1200}`)
}
