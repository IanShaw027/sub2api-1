package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func f64p(v float64) *float64 { return &v }

type httpUpstreamRecorder struct {
	lastReq      *http.Request
	lastBody     []byte
	lastProxyURL string
	requests     []*http.Request
	bodies       [][]byte

	resp      *http.Response
	responses []*http.Response
	err       error

	tlsCalled      bool
	lastTLSProfile *tlsfingerprint.Profile
}

func (u *httpUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	u.lastReq = req
	u.lastProxyURL = proxyURL
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		u.lastBody = b
		u.bodies = append(u.bodies, append([]byte(nil), b...))
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	u.requests = append(u.requests, req)
	if u.err != nil {
		return nil, u.err
	}
	if len(u.responses) > 0 {
		resp := u.responses[0]
		u.responses = u.responses[1:]
		return resp, nil
	}
	return u.resp, nil
}

func (u *httpUpstreamRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.tlsCalled = true
	u.lastTLSProfile = profile
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestOpenAIGatewayService_ResponsesUnknownModelDoesNotFallbackToGPT54(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	originalBody := []byte(`{"model":"gpt6","stream":false,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_unknown_model"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"model not found"}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          123,
		Name:        "acc",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.Error(t, err)
	require.Nil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://chatgpt.com/backend-api/codex/responses", upstream.lastReq.URL.String())
	require.Equal(t, "gpt6", gjson.GetBytes(upstream.lastBody, "model").String())
	require.NotEqual(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
	require.True(t, rec.Code >= http.StatusBadRequest)
}

func TestOpenAIGatewayService_OAuthResponsesModelMappingFeedsNormalization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	originalBody := []byte(`{"model":"gpt-5","store":false,"stream":true,"instructions":"local-test-instructions","input":[{"type":"message","role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_mapped_model"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop after capture"}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          123,
		Name:        "mapped-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
			"model_mapping": map[string]any{
				"gpt-5": "custom-codex-model",
			},
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.Error(t, err)
	require.Nil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "custom-codex-model", gjson.GetBytes(upstream.lastBody, "model").String())
	require.NotEqual(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestOpenAIGatewayService_ResponsesUnknownModelRetriesConfiguredFallbackOnce(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	originalBody := []byte(`{"model":"gpt6","stream":false,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_unknown_model_first"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"unknown model"}}`)),
			},
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_unknown_model_second"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop after fallback capture"}}`)),
			},
		},
	}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
		settingService: NewSettingService(&antigravityFallbackSettingRepoStub{values: map[string]string{
			SettingKeyEnableModelFallback: "true",
			SettingKeyFallbackModelOpenAI: "gpt-5.4",
		}}, &config.Config{}),
	}
	account := &Account{
		ID:          123,
		Name:        "acc",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.Error(t, err)
	require.Nil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "gpt6", gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.bodies[1], "model").String())

	rawBody, ok := c.Get(OpsUpstreamRequestBodyKey)
	require.True(t, ok)
	opsBody, ok := rawBody.([]byte)
	require.True(t, ok)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(opsBody, "model").String())
}

func TestOpenAIGatewayService_ResponsesUnknownModelFailoverReplaysFallbackBody(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	originalBody := []byte(`{"model":"deepseek-v4-flash","stream":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_unknown_model_first"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"The 'deepseek-v4-flash' model is not supported when using Codex with a ChatGPT account."}}`)),
			},
			{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_fallback_rate_limit"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"usage_limit_reached","message":"rate limited"}}`)),
			},
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_after_failover"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop after failover capture"}}`)),
			},
		},
	}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
		settingService: NewSettingService(&antigravityFallbackSettingRepoStub{values: map[string]string{
			SettingKeyEnableModelFallback: "true",
			SettingKeyFallbackModelOpenAI: "gpt-5.4",
		}}, &config.Config{}),
	}
	account1 := &Account{
		ID:          123,
		Name:        "acc-1",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token-1",
			"chatgpt_account_id": "chatgpt-acc-1",
		},
		Extra:       map[string]any{"openai_passthrough": true},
		Status:      StatusActive,
		Schedulable: true,
	}
	account2 := &Account{
		ID:          124,
		Name:        "acc-2",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token-2",
			"chatgpt_account_id": "chatgpt-acc-2",
		},
		Extra:       map[string]any{"openai_passthrough": true},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account1, originalBody)
	require.Error(t, err)
	require.Nil(t, result)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)

	result, err = svc.Forward(context.Background(), c, account2, originalBody)
	require.Error(t, err)
	require.Nil(t, result)

	require.Len(t, upstream.bodies, 3)
	require.Equal(t, "deepseek-v4-flash", gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.bodies[1], "model").String())
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.bodies[2], "model").String())
}

func TestOpenAIGatewayService_OAuthResponsesNormalizesCodexMiniLatestForGenericClient(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	originalBody := []byte(`{"model":"codex-mini-latest","stream":true,"instructions":"local-test-instructions","input":[{"type":"message","role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "generic-openai-client/1.0")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_codex_mini_latest"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop after capture"}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          123,
		Name:        "acc",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.Error(t, err)
	require.Nil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://chatgpt.com/backend-api/codex/responses", upstream.lastReq.URL.String())
	require.Equal(t, "gpt-5.3-codex", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestOpenAIGatewayService_OAuthMessagesBridgeInjectsDefaultInstructions(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	originalBody := []byte(`{"model":"gpt-5.5","stream":true,"prompt_cache_key":"anthropic-metadata-session-1","input":[{"type":"message","role":"developer","content":[{"type":"input_text","text":"<sub2api-claude-code-todo-guard>"}]},{"type":"message","role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_bridge"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"bridge stop"}}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          123,
		Name:        "acc",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.Error(t, err)
	require.Nil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.lastBody, "instructions").String()))
	require.False(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_key").Exists())
	require.NotEmpty(t, upstream.lastReq.Header.Get("Session_Id"))
	require.Empty(t, upstream.lastReq.Header.Get("Conversation_Id"))
	require.Empty(t, upstream.lastReq.Header.Get("OpenAI-Beta"))
	require.Empty(t, upstream.lastReq.Header.Get("originator"))
}

func TestOpenAIGatewayService_OAuthMessagesBridgeRetryKeepsFinalizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	originalBody := []byte(`{"model":"gpt-5.5","stream":false,"prompt_cache_key":"anthropic-metadata-session-1","input":[{"type":"reasoning","encrypted_content":"bad-ciphertext","summary":[{"type":"summary_text","text":"drop with encrypted item"}]},{"type":"message","role":"developer","content":[{"type":"input_text","text":"<sub2api-claude-code-todo-guard>"}]},{"type":"message","role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_bridge_retry_bad"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","code":"invalid_encrypted_content","message":"The encrypted content bad-ciphertext could not be verified."}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_bridge_retry_ok"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_bridge_retry_ok","usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}`)),
		},
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          123,
		Name:        "acc",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.bodies[0], "instructions").String()))
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.bodies[1], "instructions").String()))
	require.False(t, gjson.GetBytes(upstream.bodies[0], "prompt_cache_key").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "prompt_cache_key").Exists())
	require.True(t, gjson.GetBytes(upstream.bodies[0], "input.0.encrypted_content").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "input.0.encrypted_content").Exists())
	require.Equal(t, "message", gjson.GetBytes(upstream.bodies[1], "input.0.type").String())
}

func TestOpenAIGatewayService_OAuthRetryThenCodexCompatRetryDecodesCurrentBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	originalBody := []byte(`{"model":"gpt-5.5","stream":false,"input":[{"type":"reasoning","encrypted_content":"bad-ciphertext","summary":[{"type":"summary_text","text":"drop with encrypted item"}]},{"type":"item_reference","id":"call_1"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_retry_encrypted_bad"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","code":"invalid_encrypted_content","message":"The encrypted content bad-ciphertext could not be verified."}}`)),
		},
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_retry_call_id_bad"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"Invalid schema for field input[2].call_id"}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_retry_call_id_ok"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_retry_call_id_ok","usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}`)),
		},
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          124,
		Name:        "acc",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 3)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "input.0.encrypted_content").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "input.0.encrypted_content").Exists())
	require.Equal(t, "fc_1", gjson.GetBytes(upstream.bodies[2], "input.0.id").String())
	require.Equal(t, "fc_1", gjson.GetBytes(upstream.bodies[2], "input.1.call_id").String())
}

type openAIPassthroughFailoverRepo struct {
	stubOpenAIAccountRepo
	rateLimitCalls []time.Time
	overloadCalls  []time.Time
	setErrorCalls  []string
}

func (r *openAIPassthroughFailoverRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	r.rateLimitCalls = append(r.rateLimitCalls, resetAt)
	return nil
}

func (r *openAIPassthroughFailoverRepo) SetOverloaded(_ context.Context, _ int64, until time.Time) error {
	r.overloadCalls = append(r.overloadCalls, until)
	return nil
}

func (r *openAIPassthroughFailoverRepo) SetError(_ context.Context, _ int64, errorMsg string) error {
	r.setErrorCalls = append(r.setErrorCalls, errorMsg)
	return nil
}

var structuredLogCaptureMu sync.Mutex

type inMemoryLogSink struct {
	mu     sync.Mutex
	events []*logger.LogEvent
}

func (s *inMemoryLogSink) WriteLogEvent(event *logger.LogEvent) {
	if event == nil {
		return
	}
	cloned := *event
	if event.Fields != nil {
		cloned.Fields = make(map[string]any, len(event.Fields))
		for k, v := range event.Fields {
			cloned.Fields[k] = v
		}
	}
	s.mu.Lock()
	s.events = append(s.events, &cloned)
	s.mu.Unlock()
}

func (s *inMemoryLogSink) ContainsMessage(substr string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ev := range s.events {
		if ev != nil && strings.Contains(ev.Message, substr) {
			return true
		}
	}
	return false
}

func (s *inMemoryLogSink) ContainsMessageAtLevel(substr, level string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	wantLevel := strings.ToLower(strings.TrimSpace(level))
	for _, ev := range s.events {
		if ev == nil {
			continue
		}
		if strings.Contains(ev.Message, substr) && strings.ToLower(strings.TrimSpace(ev.Level)) == wantLevel {
			return true
		}
	}
	return false
}

func (s *inMemoryLogSink) ContainsFieldValue(field, substr string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ev := range s.events {
		if ev == nil || ev.Fields == nil {
			continue
		}
		if v, ok := ev.Fields[field]; ok && strings.Contains(fmt.Sprint(v), substr) {
			return true
		}
	}
	return false
}

func (s *inMemoryLogSink) ContainsField(field string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ev := range s.events {
		if ev == nil || ev.Fields == nil {
			continue
		}
		if _, ok := ev.Fields[field]; ok {
			return true
		}
	}
	return false
}

func captureStructuredLog(t *testing.T) (*inMemoryLogSink, func()) {
	t.Helper()
	structuredLogCaptureMu.Lock()

	err := logger.Init(logger.InitOptions{
		Level:       "debug",
		Format:      "json",
		ServiceName: "sub2api",
		Environment: "test",
		Output: logger.OutputOptions{
			ToStdout: true,
			ToFile:   false,
		},
		Sampling: logger.SamplingOptions{Enabled: false},
	})
	require.NoError(t, err)

	sink := &inMemoryLogSink{}
	logger.SetSink(sink)
	return sink, func() {
		logger.SetSink(nil)
		structuredLogCaptureMu.Unlock()
	}
}

func TestOpenAIGatewayService_OAuthPassthrough_StreamKeepsToolNameAndBodyNormalized(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("Authorization", "Bearer inbound-should-not-forward")
	c.Request.Header.Set("Cookie", "secret=1")
	c.Request.Header.Set("X-Api-Key", "sk-inbound")
	c.Request.Header.Set("X-Goog-Api-Key", "goog-inbound")
	c.Request.Header.Set("Accept-Encoding", "gzip")
	c.Request.Header.Set("Proxy-Authorization", "Basic abc")
	c.Request.Header.Set("X-Test", "keep")
	c.Request.Header.Set("x-codex-beta-features", "remote_compaction_v2")

	originalBody := []byte(`{"model":"gpt-5.2","stream":true,"store":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)

	upstreamSSE := strings.Join([]string{
		`data: {"type":"response.output_item.added","item":{"type":"tool_call","tool_calls":[{"function":{"name":"apply_patch"}}]}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid"}},
		Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
		openAITokenProvider: &OpenAITokenProvider{ // minimal: will be bypassed by nil cache/service, but GetAccessToken uses provider only if non-nil
			accountRepo: nil,
		},
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	// Use the gateway method that reads token from credentials when provider is nil.
	svc.openAITokenProvider = nil

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)

	// 1) 透传 OAuth 请求体与旧链路关键行为保持一致：store=false + stream=true。
	require.Equal(t, false, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.Equal(t, true, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "local-test-instructions", strings.TrimSpace(gjson.GetBytes(upstream.lastBody, "instructions").String()))
	// 其余关键字段保持原值。
	require.Equal(t, "gpt-5.2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "hi", gjson.GetBytes(upstream.lastBody, "input.0.text").String())

	// 2) only auth is replaced; inbound auth/cookie are not forwarded
	require.Equal(t, "Bearer oauth-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "codex_cli_rs/0.1.0", upstream.lastReq.Header.Get("User-Agent"))
	require.Empty(t, upstream.lastReq.Header.Get("Cookie"))
	require.Empty(t, upstream.lastReq.Header.Get("X-Api-Key"))
	require.Empty(t, upstream.lastReq.Header.Get("X-Goog-Api-Key"))
	require.Empty(t, upstream.lastReq.Header.Get("Accept-Encoding"))
	require.Empty(t, upstream.lastReq.Header.Get("Proxy-Authorization"))
	require.Empty(t, upstream.lastReq.Header.Get("X-Test"))
	require.Equal(t, "remote_compaction_v2", upstream.lastReq.Header.Get("x-codex-beta-features"))

	// 3) required OAuth headers are present
	require.Equal(t, "chatgpt.com", upstream.lastReq.Host)
	require.Equal(t, "chatgpt-acc", upstream.lastReq.Header.Get("chatgpt-account-id"))

	// 4) downstream SSE keeps tool name (no toolCorrector)
	body := rec.Body.String()
	require.Contains(t, body, "apply_patch")
	require.NotContains(t, body, "\"name\":\"edit\"")
}

func TestOpenAIGatewayService_OAuthPassthrough_CompactUsesJSONAndKeepsNonStreaming(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(nil))
	c.Request.Header.Set("Accept", "*/*")
	c.Request.Header.Set("User-Agent", "codex_exec/0.144.1")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Session_Id", "cache-123")
	c.Request.Header.Set("X-Codex-Installation-Id", "inst-123")
	c.Request.Header.Set("X-Codex-Window-Id", "cache-123:0")
	c.Request.Header.Set("Originator", "codex_exec")

	originalBody := []byte(`{
		"model":"gpt-5.1-codex",
		"stream":true,
		"store":true,
		"instructions":"local-test-instructions",
		"input":[{"type":"text","text":"compact me"}],
		"tools":[{"type":"function","function":{"name":"apply_patch"}}],
		"parallel_tool_calls":true,
		"reasoning":{"effort":"high"},
		"text":{"verbosity":"low"},
		"prompt_cache_key":"cache-123",
		"metadata":{"source":"test"},
		"max_output_tokens":64
	}`)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-compact"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"cmp_123","usage":{"input_tokens":11,"output_tokens":22}}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{
			ForceCodexCLI:      false,
			OpenAICompactModel: "gpt-5.4",
		}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)

	require.False(t, gjson.GetBytes(upstream.lastBody, "store").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Exists())
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "compact me", gjson.GetBytes(upstream.lastBody, "input.0.text").String())
	require.Equal(t, "local-test-instructions", strings.TrimSpace(gjson.GetBytes(upstream.lastBody, "instructions").String()))
	require.Equal(t, "apply_patch", gjson.GetBytes(upstream.lastBody, "tools.0.function.name").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "parallel_tool_calls").Bool())
	require.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "reasoning.effort").String())
	require.Equal(t, "low", gjson.GetBytes(upstream.lastBody, "text.verbosity").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_key").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "metadata").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "max_output_tokens").Exists())
	require.Equal(t, "*/*", upstream.lastReq.Header.Get("Accept"))
	require.Empty(t, upstream.lastReq.Header.Get("Version"))
	require.Empty(t, upstream.lastReq.Header.Get("OpenAI-Beta"))
	require.Equal(t, "cache-123", upstream.lastReq.Header.Get("Session_Id"))
	require.Empty(t, upstream.lastReq.Header.Get("Conversation_Id"))
	require.Equal(t, "inst-123", upstream.lastReq.Header.Get("X-Codex-Installation-Id"))
	require.Equal(t, "cache-123:0", upstream.lastReq.Header.Get("X-Codex-Window-Id"))
	require.Equal(t, "codex_exec", upstream.lastReq.Header.Get("Originator"))
	require.Equal(t, "chatgpt.com", upstream.lastReq.Host)
	require.Equal(t, "chatgpt-acc", upstream.lastReq.Header.Get("chatgpt-account-id"))
	require.Contains(t, rec.Body.String(), `"id":"cmp_123"`)
}

func TestOpenAIGatewayService_OAuthPassthrough_UpstreamRequestIgnoresClientCancel(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	reqCtx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil)).WithContext(reqCtx)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	cancel()

	originalBody := []byte(`{"model":"gpt-5.2","stream":true,"store":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_passthrough_ctx"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.completed","response":{"usage":{"input_tokens":2,"output_tokens":1}}}`,
			"",
			"data: [DONE]",
			"",
		}, "\n"))),
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true, "openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(reqCtx, c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.NoError(t, upstream.lastReq.Context().Err())
}

func TestOpenAIGatewayService_OAuthPassthrough_CodexMissingInstructionsInjectsBeforeUpstream(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?trace=1", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0 (Windows 10.0.19045; x86_64) unknown")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("OpenAI-Beta", "responses=experimental")

	// Codex 模型且缺少 instructions，应补入默认值后继续透传。
	originalBody := []byte(`{"model":"gpt-5.1-codex-max","stream":false,"store":true,"input":[{"type":"text","text":"hi"}]}`)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_text.delta","delta":"h"}`,
				"",
				`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
				"",
				"data: [DONE]",
				"",
			}, "\n"))),
		},
	}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, upstream.lastReq)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.lastBody, "instructions").String()))
}

func TestOpenAIGatewayService_OAuthPassthrough_RetriesInstructionsRequiredOnce(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.133 (external, cli)")
	c.Request.Header.Set("Content-Type", "application/json")

	originalBody := []byte(`{"model":"gpt-5.4-mini","stream":true,"input":[{"type":"text","text":"hi"}]}`)

	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_missing_instructions"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"Instructions are required"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_after_retry"}},
				Body: io.NopCloser(strings.NewReader(strings.Join([]string{
					`data: {"type":"response.output_text.delta","delta":"h"}`,
					"",
					`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
					"",
					"data: [DONE]",
					"",
				}, "\n"))),
			},
		},
	}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.bodies[0], "instructions").String()))
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.bodies[1], "instructions").String()))
}

func TestOpenAIGatewayService_OAuthPassthrough_RetriesInvalidEncryptedContentWithSanitizedBody(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
	c.Request.Header.Set("Content-Type", "application/json")

	originalBody := []byte(`{"model":"gpt-5.5","stream":true,"previous_response_id":"resp_stale","input":[{"type":"reasoning","encrypted_content":"gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]}`)

	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_invalid_encrypted"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","code":"invalid_encrypted_content","message":"The encrypted content could not be verified. Reason: Encrypted content could not be decrypted or parsed."}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_after_sanitize"}},
				Body: io.NopCloser(strings.NewReader(strings.Join([]string{
					`data: {"type":"response.output_text.delta","delta":"o"}`,
					"",
					`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
					"",
					"data: [DONE]",
					"",
				}, "\n"))),
			},
		},
	}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true, "openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.True(t, gjson.GetBytes(upstream.bodies[0], "input.0.encrypted_content").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "previous_response_id").Exists())
	require.NotContains(t, string(upstream.bodies[1]), "encrypted_content")
	require.Equal(t, "continue", gjson.GetBytes(upstream.bodies[1], "input.0.content.0.text").String())
}

func TestOpenAIGatewayService_OAuthPassthrough_RetriesUnsupportedPreviousResponseIDWithDroppedField(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
	c.Request.Header.Set("Content-Type", "application/json")

	originalBody := []byte(`{"model":"gpt-5.5","stream":true,"previous_response_id":"resp_stale","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]}`)
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_previous_response_unsupported"}},
				Body:       io.NopCloser(strings.NewReader(`{"detail":"Unsupported parameter: previous_response_id"}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_previous_response_retry"}},
				Body: io.NopCloser(strings.NewReader(strings.Join([]string{
					`data: {"type":"response.output_text.delta","delta":"o"}`,
					"",
					`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
					"",
					"data: [DONE]",
					"",
				}, "\n"))),
			},
		},
	}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true, "openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "previous_response_id").Exists())
}

func TestOpenAIGatewayService_OAuthPassthrough_RetriesUnknownReasoningEnabledWithDroppedField(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
	c.Request.Header.Set("Content-Type", "application/json")

	originalBody := []byte(`{"model":"gpt-5.4","stream":true,"reasoning":{"effort":"high","enabled":true},"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]}`)
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_reasoning_enabled_unknown"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Unknown parameter: 'reasoning.enabled'.","type":"invalid_request_error","param":"reasoning.enabled","code":"unknown_parameter"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_reasoning_enabled_retry"}},
				Body: io.NopCloser(strings.NewReader(strings.Join([]string{
					`data: {"type":"response.output_text.delta","delta":"o"}`,
					"",
					`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
					"",
					"data: [DONE]",
					"",
				}, "\n"))),
			},
		},
	}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true, "openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "reasoning.enabled").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "reasoning.enabled").Exists())
	require.Equal(t, "high", gjson.GetBytes(upstream.bodies[1], "reasoning.effort").String())
}

func TestOpenAIGatewayService_OAuthPassthrough_AppliesErrorPassthroughRule(t *testing.T) {
	setGinTestMode()

	responseCode := http.StatusRequestEntityTooLarge
	customMessage := "Request body is too large for the selected upstream. Reduce attachments/context and retry."
	for _, statusCode := range []int{http.StatusRequestEntityTooLarge, http.StatusInternalServerError} {
		t.Run(fmt.Sprintf("status_%d", statusCode), func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
			c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
			c.Request.Header.Set("Content-Type", "application/json")

			ruleSvc := &ErrorPassthroughService{}
			ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{
				{
					ID:              1,
					Name:            "openai-large-request",
					Enabled:         true,
					Priority:        1,
					ErrorCodes:      []int{http.StatusRequestEntityTooLarge, http.StatusInternalServerError},
					Keywords:        []string{"message too big"},
					MatchMode:       model.MatchModeAll,
					Platforms:       []string{model.PlatformOpenAI},
					PassthroughCode: false,
					ResponseCode:    &responseCode,
					PassthroughBody: false,
					CustomMessage:   &customMessage,
				},
			})
			BindErrorPassthroughService(c, ruleSvc)

			originalBody := []byte(`{"model":"gpt-5.4","stream":false,"input":[{"type":"message","role":"user","content":"large"}]}`)
			upstream := &httpUpstreamRecorder{
				resp: &http.Response{
					StatusCode: statusCode,
					Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_large_request"}},
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"message too big","type":"server_error"}}`)),
				},
			}
			svc := &OpenAIGatewayService{
				cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
				httpUpstream: upstream,
			}
			account := &Account{
				ID:             123,
				Name:           "acc",
				Platform:       PlatformOpenAI,
				Type:           AccountTypeOAuth,
				Concurrency:    1,
				Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
				Extra:          map[string]any{"openai_passthrough": true, "openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff},
				Status:         StatusActive,
				Schedulable:    true,
				RateMultiplier: f64p(1),
			}

			result, err := svc.Forward(context.Background(), c, account, originalBody)
			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
			require.Contains(t, rec.Body.String(), customMessage)
		})
	}
}

func TestOpenAIGatewayService_OpenAIPassthrough_ErrorPassthroughRuleSkipsAccountPolicy(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
	c.Request.Header.Set("Content-Type", "application/json")

	responseCode := http.StatusRequestEntityTooLarge
	customMessage := "Request body is too large for the selected upstream. Reduce attachments/context and retry."
	ruleSvc := &ErrorPassthroughService{}
	ruleSvc.setLocalCache([]*model.ErrorPassthroughRule{
		{
			ID:              1,
			Name:            "openai-large-request",
			Enabled:         true,
			Priority:        1,
			ErrorCodes:      []int{http.StatusInternalServerError},
			Keywords:        []string{"message too big"},
			MatchMode:       model.MatchModeAll,
			Platforms:       []string{model.PlatformOpenAI},
			PassthroughCode: false,
			ResponseCode:    &responseCode,
			PassthroughBody: false,
			CustomMessage:   &customMessage,
		},
	})
	BindErrorPassthroughService(c, ruleSvc)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_large_request"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"message too big","type":"server_error"}}`)),
		},
	}
	repo := &openAIPassthroughFailoverRepo{}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
		rateLimitService: &RateLimitService{
			accountRepo: repo,
			cfg:         &config.Config{},
		},
	}
	account := &Account{
		ID:          123,
		Name:        "acc",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":                    "sk-test",
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(http.StatusInternalServerError)},
		},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.4","stream":false,"input":[{"type":"message","role":"user","content":"large"}]}`))
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.Contains(t, rec.Body.String(), customMessage)
	require.Empty(t, repo.setErrorCalls)
}

func TestOpenAIGatewayService_OAuthPassthrough_StripsImageToolCapabilityForDisabledGroup(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")
	groupID := int64(4242)
	c.Set("api_key", &APIKey{
		ID:      2424,
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			AllowImageGeneration: false,
			RateMultiplier:       1,
			ImageRateMultiplier:  1,
		},
	})

	originalBody := []byte(`{"model":"gpt-5.4","stream":false,"input":"write code","tools":[{"type":"image_generation"},{"type":"function","name":"shell"}]}`)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_strip_image_tool"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_text.delta","delta":"o"}`,
				"",
				`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
				"",
				"data: [DONE]",
				"",
			}, "\n"))),
		},
	}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             126,
		Name:           "acc-pass-image-tool-strip",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.False(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
	require.True(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="function")`).Exists())
}

func TestOpenAIGatewayService_APIKeyPassthrough_StripsSamplingControls(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")

	originalBody := []byte(`{"model":"gpt-5.3-codex","stream":true,"temperature":0.2,"top_p":0.8,"input":"hi"}`)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_text.delta","delta":"h"}`,
				"",
				`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
				"",
				"data: [DONE]",
				"",
			}, "\n"))),
		},
	}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:          125,
		Name:        "acc-apikey-pass",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test"},
		Extra:       map[string]any{"openai_passthrough": true},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.False(t, gjson.GetBytes(upstream.lastBody, "instructions").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "temperature").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "top_p").Exists())
	require.Equal(t, "hi", gjson.GetBytes(upstream.lastBody, "input.0.content").String())
}

func TestOpenAIGatewayService_APIKeyPassthrough_OfficialClientMissingInstructionsDoesNotInjectOrReject(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses?trace=1", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.98.0 (Windows 10.0.19045; x86_64) unknown")
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("OpenAI-Beta", "responses=experimental")

	originalBody := []byte(`{"model":"gpt-5.3-codex","stream":true,"input":"hi"}`)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-apikey-official"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_text.delta","delta":"h"}`,
				"",
				`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
				"",
				"data: [DONE]",
				"",
			}, "\n"))),
		},
	}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:          126,
		Name:        "acc-apikey-official",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test"},
		Extra:       map[string]any{"openai_passthrough": true},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, upstream.lastReq)
	require.False(t, gjson.GetBytes(upstream.lastBody, "instructions").Exists())
}

func TestOpenAIGatewayService_APIKeyPassthrough_CustomBaseURLPreservesTopP(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")

	originalBody := []byte(`{"model":"gpt-5.3-codex","stream":true,"top_p":0.8,"input":"hi"}`)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-custom-baseurl"}},
			Body: io.NopCloser(strings.NewReader(strings.Join([]string{
				`data: {"type":"response.output_text.delta","delta":"h"}`,
				"",
				`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
				"",
				"data: [DONE]",
				"",
			}, "\n"))),
		},
	}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:          127,
		Name:        "acc-apikey-custom-baseurl",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://third-party.example/api",
		},
		Extra:       map[string]any{"openai_passthrough": true},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, 0.8, gjson.GetBytes(upstream.lastBody, "top_p").Float())
}

func TestOpenAIGatewayService_OAuthPassthrough_NonCodexResponsesWithoutInstructionsReachUpstream(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "curl/8.0")
	c.Request.Header.Set("Content-Type", "application/json")

	originalBody := []byte(`{"model":"gpt-5.4-mini","stream":true,"store":true,"max_output_tokens":1200,"input":[{"type":"text","text":"hi"}]}`)

	upstreamSSE := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"h"}`,
		"",
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid"}},
			Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
		},
	}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             124,
		Name:           "acc-generic",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.lastBody, "instructions").String()))
	require.True(t, gjson.GetBytes(upstream.lastBody, "store").Exists())
	require.Equal(t, false, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "max_output_tokens").Exists())
}

func TestEnsureOpenAIPassthroughInstructions_ChatCompletionsPathInjectsDefaultInstructions(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))

	originalBody := []byte(`{"model":"gpt-5.1-codex-max","stream":false,"store":true,"input":[{"type":"text","text":"hi"}]}`)
	updatedBody, changed, err := ensureOpenAIPassthroughInstructions(c, "gpt-5.1-codex-max", originalBody)
	require.NoError(t, err)
	require.True(t, changed)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(updatedBody, "instructions").String()))
}

func TestEnsureOpenAIPassthroughInstructions_ChatCompletionsPathInjectsDefaultInstructionsForGenericResponsesModel(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))

	originalBody := []byte(`{"model":"gpt-5.4","stream":false,"store":true,"input":[{"type":"text","text":"hi"}]}`)
	updatedBody, changed, err := ensureOpenAIPassthroughInstructions(c, "gpt-5.4", originalBody)
	require.NoError(t, err)
	require.True(t, changed)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(updatedBody, "instructions").String()))
}

func TestEnsureOpenAIPassthroughInstructions_ResponsesPathInjectsDefaultInstructions(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))

	originalBody := []byte(`{"model":"gpt-5.1-codex-max","stream":false,"store":true,"input":[{"type":"text","text":"hi"}]}`)
	updatedBody, changed, err := ensureOpenAIPassthroughInstructions(c, "gpt-5.1-codex-max", originalBody)
	require.NoError(t, err)
	require.True(t, changed)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(updatedBody, "instructions").String()))
}

func TestEnsureOpenAIPassthroughInstructions_RawResponsesPathInjectsDefaultInstructions(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/responses", bytes.NewReader(nil))

	originalBody := []byte(`{"model":"gpt-5.1-codex-max","stream":false,"store":true,"input":[{"type":"text","text":"hi"}]}`)
	updatedBody, changed, err := ensureOpenAIPassthroughInstructions(c, "gpt-5.1-codex-max", originalBody)
	require.NoError(t, err)
	require.True(t, changed)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(updatedBody, "instructions").String()))
}

func TestOpenAIGatewayService_OAuthPassthrough_DisabledUsesLegacyTransform(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")

	// store=true + stream=false should be forced to store=false + stream=true by applyCodexOAuthTransform (OAuth legacy path)
	inputBody := []byte(`{"model":"gpt-5.2","stream":false,"store":true,"input":[{"type":"text","text":"hi"}]}`)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid"}},
		Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": false},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, inputBody)
	require.NoError(t, err)

	// legacy path rewrites request body (not byte-equal)
	require.NotEqual(t, inputBody, upstream.lastBody)
	require.Contains(t, string(upstream.lastBody), `"store":false`)
	require.Contains(t, string(upstream.lastBody), `"stream":true`)
}

func TestOpenAIGatewayService_OAuthLegacy_GenericResponsesForcesStoreFalseAndStripsMaxOutputTokens(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "Go-http-client/2.0")

	inputBody := []byte(`{"model":"gpt-5.4-mini","stream":false,"max_output_tokens":1200,"max_completion_tokens":800,"input":[{"type":"text","text":"hi"}]}`)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-generic-legacy"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_ok","usage":{"input_tokens":1,"output_tokens":1}}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": false, "openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, inputBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, gjson.GetBytes(upstream.lastBody, "store").Exists())
	require.Equal(t, false, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.Equal(t, true, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.lastBody, "instructions").String()))
	require.False(t, gjson.GetBytes(upstream.lastBody, "max_output_tokens").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "max_completion_tokens").Exists())
	require.False(t, result.Stream)
}

func TestOpenAIGatewayService_APIKeyLegacy_GenericResponsesWithoutInstructionsDoesNotInjectDefaultInstructions(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "Go-http-client/2.0")

	inputBody := []byte(`{"model":"gpt-5.4-mini","stream":false,"input":[{"type":"text","text":"hi"}]}`)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-apikey-legacy"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_ok","usage":{"input_tokens":1,"output_tokens":1}}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:          223,
		Name:        "acc-apikey-legacy",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, inputBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, gjson.GetBytes(upstream.lastBody, "instructions").Exists())
}

func TestOpenAIGatewayService_OAuthLegacy_UpstreamRequestIgnoresClientCancel(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	reqCtx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil)).WithContext(reqCtx)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	cancel()

	originalBody := []byte(`{"model":"gpt-5.2","stream":false,"store":true,"input":[{"type":"text","text":"hi"}]}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_legacy_ctx"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1}}}`,
			"",
			"data: [DONE]",
			"",
		}, "\n"))),
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": false, "openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(reqCtx, c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.NoError(t, upstream.lastReq.Context().Err())
}

func TestOpenAIGatewayService_OAuthLegacy_CompositeCodexUAUsesCodexOriginator(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	// 复合 UA（前缀不是 codex_cli_rs），历史实现会误判为非 Codex 并走 opencode。
	c.Request.Header.Set("User-Agent", "Mozilla/5.0 codex_cli_rs/0.1.0")

	inputBody := []byte(`{"model":"gpt-5.2","stream":true,"store":false,"input":[{"type":"text","text":"hi"}]}`)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid"}},
		Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": false},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, inputBody)
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	// Browser-shaped composite identities use the configured/default codex-tui
	// anti-challenge profile, then pair originator to that final User-Agent.
	require.Equal(t, "codex-tui", upstream.lastReq.Header.Get("originator"))
	require.NotEqual(t, "opencode", upstream.lastReq.Header.Get("originator"))
}

func TestOpenAIGatewayService_OAuthPassthrough_ResponseHeadersAllowXCodex(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")

	originalBody := []byte(`{"model":"gpt-5.2","stream":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("x-request-id", "rid")
	headers.Set("x-codex-primary-used-percent", "12")
	headers.Set("x-codex-secondary-used-percent", "34")
	headers.Set("x-codex-primary-window-minutes", "300")
	headers.Set("x-codex-secondary-window-minutes", "10080")
	headers.Set("x-codex-primary-reset-after-seconds", "1")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     headers,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"h"}`,
			"",
			`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
			"",
			"data: [DONE]",
			"",
		}, "\n"))),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)

	require.Equal(t, "12", rec.Header().Get("x-codex-primary-used-percent"))
	require.Equal(t, "34", rec.Header().Get("x-codex-secondary-used-percent"))
}

func TestOpenAIGatewayService_OAuthPassthrough_UpstreamErrorIncludesPassthroughFlag(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")

	originalBody := []byte(`{"model":"gpt-5.2","stream":false,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad"}}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, originalBody)
	require.Error(t, err)
	require.True(t, c.Writer.Written(), "非 429/529 的 passthrough 错误应继续原样写回客户端")
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// should append an upstream error event with passthrough=true
	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	arr, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.NotEmpty(t, arr)
	require.True(t, arr[len(arr)-1].Passthrough)
	require.Equal(t, "http_error", arr[len(arr)-1].Kind)
}

func TestOpenAIGatewayService_OpenAIPassthrough_429And529TriggerFailover(t *testing.T) {
	setGinTestMode()
	originalBody := []byte(`{"model":"gpt-5.2","stream":false,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)

	newAccount := func(accountType string) *Account {
		account := &Account{
			ID:             123,
			Name:           "acc",
			Platform:       PlatformOpenAI,
			Type:           accountType,
			Concurrency:    1,
			Extra:          map[string]any{"openai_passthrough": true},
			Status:         StatusActive,
			Schedulable:    true,
			RateMultiplier: f64p(1),
		}
		switch accountType {
		case AccountTypeOAuth:
			account.Credentials = map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"}
		case AccountTypeAPIKey:
			account.Credentials = map[string]any{"api_key": "sk-test"}
		}
		return account
	}

	testCases := []struct {
		name           string
		accountType    string
		statusCode     int
		body           string
		expectFailover bool
		assertRepo     func(t *testing.T, repo *openAIPassthroughFailoverRepo, start time.Time)
	}{
		{
			name:        "oauth_429_rate_limit",
			accountType: AccountTypeOAuth,
			statusCode:  http.StatusTooManyRequests,
			body: func() string {
				resetAt := time.Now().Add(7 * 24 * time.Hour).Unix()
				return fmt.Sprintf(`{"error":{"message":"The usage limit has been reached","type":"usage_limit_reached","resets_at":%d}}`, resetAt)
			}(),
			expectFailover: true,
			assertRepo: func(t *testing.T, repo *openAIPassthroughFailoverRepo, _ time.Time) {
				require.Len(t, repo.rateLimitCalls, 1)
				require.Empty(t, repo.overloadCalls)
				require.True(t, time.Until(repo.rateLimitCalls[0]) > 24*time.Hour)
			},
		},
		{
			name:           "oauth_529_overload",
			accountType:    AccountTypeOAuth,
			statusCode:     529,
			body:           `{"error":{"message":"server overloaded","type":"server_error"}}`,
			expectFailover: true,
			assertRepo: func(t *testing.T, repo *openAIPassthroughFailoverRepo, start time.Time) {
				require.Empty(t, repo.rateLimitCalls)
				require.Len(t, repo.overloadCalls, 1)
				require.WithinDuration(t, start.Add(10*time.Minute), repo.overloadCalls[0], 5*time.Second)
			},
		},
		{
			name:           "oauth_502_bad_gateway",
			accountType:    AccountTypeOAuth,
			statusCode:     http.StatusBadGateway,
			body:           `{"error":{"message":"bad gateway","type":"server_error"}}`,
			expectFailover: false,
			assertRepo: func(t *testing.T, repo *openAIPassthroughFailoverRepo, _ time.Time) {
				require.Empty(t, repo.rateLimitCalls)
				require.Empty(t, repo.overloadCalls)
			},
		},
		{
			name:           "oauth_503_unavailable",
			accountType:    AccountTypeOAuth,
			statusCode:     http.StatusServiceUnavailable,
			body:           `{"error":{"message":"service unavailable","type":"server_error"}}`,
			expectFailover: false,
			assertRepo: func(t *testing.T, repo *openAIPassthroughFailoverRepo, _ time.Time) {
				require.Empty(t, repo.rateLimitCalls)
				require.Empty(t, repo.overloadCalls)
			},
		},
		{
			name:           "oauth_504_gateway_timeout",
			accountType:    AccountTypeOAuth,
			statusCode:     http.StatusGatewayTimeout,
			body:           `{"error":{"message":"gateway timeout","type":"server_error"}}`,
			expectFailover: false,
			assertRepo: func(t *testing.T, repo *openAIPassthroughFailoverRepo, _ time.Time) {
				require.Empty(t, repo.rateLimitCalls)
				require.Empty(t, repo.overloadCalls)
			},
		},
		{
			name:        "apikey_429_rate_limit",
			accountType: AccountTypeAPIKey,
			statusCode:  http.StatusTooManyRequests,
			body: func() string {
				resetAt := time.Now().Add(7 * 24 * time.Hour).Unix()
				return fmt.Sprintf(`{"error":{"message":"The usage limit has been reached","type":"usage_limit_reached","resets_at":%d}}`, resetAt)
			}(),
			expectFailover: true,
			assertRepo: func(t *testing.T, repo *openAIPassthroughFailoverRepo, _ time.Time) {
				require.Len(t, repo.rateLimitCalls, 1)
				require.Empty(t, repo.overloadCalls)
				require.True(t, time.Until(repo.rateLimitCalls[0]) > 24*time.Hour)
			},
		},
		{
			name:           "apikey_529_overload",
			accountType:    AccountTypeAPIKey,
			statusCode:     529,
			body:           `{"error":{"message":"server overloaded","type":"server_error"}}`,
			expectFailover: true,
			assertRepo: func(t *testing.T, repo *openAIPassthroughFailoverRepo, start time.Time) {
				require.Empty(t, repo.rateLimitCalls)
				require.Len(t, repo.overloadCalls, 1)
				require.WithinDuration(t, start.Add(10*time.Minute), repo.overloadCalls[0], 5*time.Second)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
			c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")

			resp := &http.Response{
				StatusCode: tc.statusCode,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"x-request-id": []string{"rid-failover"},
				},
				Body: io.NopCloser(strings.NewReader(tc.body)),
			}
			upstream := &httpUpstreamRecorder{resp: resp}
			repo := &openAIPassthroughFailoverRepo{}
			rateSvc := &RateLimitService{
				accountRepo: repo,
				cfg: &config.Config{
					RateLimit: config.RateLimitConfig{OverloadCooldownMinutes: 10},
				},
			}

			svc := &OpenAIGatewayService{
				cfg:              &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
				httpUpstream:     upstream,
				rateLimitService: rateSvc,
			}

			account := newAccount(tc.accountType)
			start := time.Now()
			_, err := svc.Forward(context.Background(), c, account, originalBody)
			require.Error(t, err)

			var failoverErr *UpstreamFailoverError
			if tc.expectFailover {
				require.ErrorAs(t, err, &failoverErr)
				require.Equal(t, tc.statusCode, failoverErr.StatusCode)
				require.False(t, c.Writer.Written(), "retryable passthrough 错误应返回 failover 错误给上层换号，而不是直接向客户端写响应")
			} else {
				require.False(t, errors.As(err, &failoverErr))
				require.True(t, c.Writer.Written(), "非 failover 的 passthrough http 错误应直接写回客户端")
				require.Equal(t, tc.statusCode, rec.Code)
			}

			v, ok := c.Get(OpsUpstreamErrorsKey)
			require.True(t, ok)
			arr, ok := v.([]*OpsUpstreamErrorEvent)
			require.True(t, ok)
			require.NotEmpty(t, arr)
			require.True(t, arr[len(arr)-1].Passthrough)
			if tc.expectFailover {
				require.Equal(t, "failover", arr[len(arr)-1].Kind)
			} else {
				require.Equal(t, "http_error", arr[len(arr)-1].Kind)
			}
			require.Equal(t, tc.statusCode, arr[len(arr)-1].UpstreamStatusCode)

			tc.assertRepo(t, repo, start)
		})
	}
}

func TestOpenAIGatewayService_OpenAIPassthrough_Transient5xxTriggerFailoverWithoutSameAccountRetry(t *testing.T) {
	setGinTestMode()
	originalBody := []byte(`{"model":"gpt-5.2","stream":false,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)

	testCases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "cloudflare_520", statusCode: 520, body: `{"error":{"message":"unknown cloudflare error","type":"server_error"}}`},
		{name: "bad_gateway_502", statusCode: http.StatusBadGateway, body: `{"error":{"message":"bad gateway","type":"server_error"}}`},
		{name: "service_unavailable_503", statusCode: http.StatusServiceUnavailable, body: `{"error":{"message":"service temporarily unavailable","type":"server_error"}}`},
		{name: "cloudflare_524", statusCode: 524, body: `{"error":{"message":"a timeout occurred","type":"server_error"}}`},
		{name: "internal_500", statusCode: http.StatusInternalServerError, body: `{"error":{"message":"internal server error","type":"server_error"}}`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
			c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")

			upstream := &httpUpstreamRecorder{
				resp: &http.Response{
					StatusCode: tc.statusCode,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
						"x-request-id": []string{"rid-transient"},
					},
					Body: io.NopCloser(strings.NewReader(tc.body)),
				},
			}
			svc := &OpenAIGatewayService{
				cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
				httpUpstream: upstream,
			}
			account := &Account{
				ID:             123,
				Name:           "acc",
				Platform:       PlatformOpenAI,
				Type:           AccountTypeAPIKey,
				Concurrency:    1,
				Credentials:    map[string]any{"api_key": "sk-test", "pool_mode": true},
				Extra:          map[string]any{"openai_passthrough": true},
				Status:         StatusActive,
				Schedulable:    true,
				RateMultiplier: f64p(1),
			}

			_, err := svc.Forward(context.Background(), c, account, originalBody)
			require.Error(t, err)

			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, tc.statusCode, failoverErr.StatusCode)
			require.False(t, failoverErr.RetryableOnSameAccount)
			require.False(t, c.Writer.Written(), "transient passthrough 5xx should fail over instead of writing directly")
		})
	}
}

func TestOpenAIGatewayService_OAuthPassthrough_NonCodexUAFallsBackWithoutForceCodexCLI(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	// Non-Codex UA
	c.Request.Header.Set("User-Agent", "curl/8.0")

	inputBody := []byte(`{"model":"gpt-5.2","stream":false,"store":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid"}},
		Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, inputBody)
	require.NoError(t, err)
	require.Equal(t, false, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.Equal(t, true, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, codexCLIUserAgent, upstream.lastReq.Header.Get("User-Agent"))
	require.Empty(t, upstream.lastReq.Header.Get("Originator"))
}

func TestOpenAIGatewayService_CodexCLIOnly_RejectsNonCodexClient(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "curl/8.0")

	inputBody := []byte(`{"model":"gpt-5.2","stream":false,"store":true,"input":[{"type":"text","text":"hi"}]}`)

	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true, "codex_cli_only": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, inputBody)
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "Codex official clients")
}

func TestOpenAIGatewayService_CodexCLIOnly_AllowOfficialClientFamilies(t *testing.T) {
	setGinTestMode()

	tests := []struct {
		name       string
		ua         string
		originator string
	}{
		{name: "codex_cli_rs", ua: "codex_cli_rs/0.99.0", originator: ""},
		{name: "codex_vscode", ua: "codex_vscode/1.0.0", originator: ""},
		{name: "codex_app", ua: "codex_app/2.1.0", originator: ""},
		// req②：codex_cli_only 下 UA 须能解析出引擎版本；originator 命中路径用可解析的非官方前缀 UA。
		{name: "originator_codex_chatgpt_desktop", ua: "myterm/0.141.0", originator: "codex_chatgpt_desktop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
			c.Request.Header.Set("User-Agent", tt.ua)
			if tt.originator != "" {
				c.Request.Header.Set("originator", tt.originator)
			}
			// 引擎指纹头：真实官方客户端必带。本测试用 nil settingService 构造 gateway，
			// detectCodexClientRestriction 会兜底默认种子指纹信号（只勾 x-codex-），与生产默认策略一致，
			// 故官方家族也须携带 x-codex-* 才能过门（对齐 TestDetect_EngineFingerprintSignals）。
			c.Request.Header.Set("x-codex-window-id", "1")

			inputBody := []byte(`{"model":"gpt-5.2","stream":false,"store":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)

			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid"}},
				Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
			}
			upstream := &httpUpstreamRecorder{resp: resp}

			svc := &OpenAIGatewayService{
				cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
				httpUpstream: upstream,
			}

			account := &Account{
				ID:             123,
				Name:           "acc",
				Platform:       PlatformOpenAI,
				Type:           AccountTypeOAuth,
				Concurrency:    1,
				Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
				Extra:          map[string]any{"openai_passthrough": true, "codex_cli_only": true},
				Status:         StatusActive,
				Schedulable:    true,
				RateMultiplier: f64p(1),
			}

			_, err := svc.Forward(context.Background(), c, account, inputBody)
			require.NoError(t, err)
			require.NotNil(t, upstream.lastReq)
		})
	}
}

func TestOpenAIGatewayService_OAuthPassthrough_StreamingSetsFirstTokenMs(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")

	originalBody := []byte(`{"model":"gpt-5.2","stream":true,"service_tier":"fast","instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)

	upstreamSSE := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"h"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid"}},
		Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	start := time.Now()
	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	// sanity: duration after start
	require.GreaterOrEqual(t, time.Since(start), time.Duration(0))
	require.NotNil(t, result.FirstTokenMs)
	require.GreaterOrEqual(t, *result.FirstTokenMs, 0)
	require.NotNil(t, result.ServiceTier)
	require.Equal(t, "priority", *result.ServiceTier)
}

func TestOpenAIGatewayService_OAuthPassthrough_StreamClientDisconnectStillCollectsUsage(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	// 首次写入成功，后续写入失败，模拟客户端中途断开。
	c.Writer = &failingGinWriter{ResponseWriter: c.Writer, failAfter: 1}

	originalBody := []byte(`{"model":"gpt-5.2","stream":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)

	upstreamSSE := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"h"}`,
		"",
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":11,"output_tokens":7,"input_tokens_details":{"cached_tokens":3}}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid"}},
		Body:       io.NopCloser(strings.NewReader(upstreamSSE)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             123,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.NotNil(t, result.FirstTokenMs)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, 3, result.Usage.CacheReadInputTokens)
}

func TestOpenAIGatewayService_APIKeyPassthrough_PreservesBodyAndUsesResponsesEndpoint(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "curl/8.0")
	c.Request.Header.Set("X-Test", "keep")
	c.Request.Header.Set("x-codex-beta-features", "remote_compaction_v2")

	originalBody := []byte(`{"model":"gpt-5.2","stream":false,"service_tier":"flex","max_output_tokens":128,"reasoning":{"mode":"pro"},"input":[{"type":"text","text":"hi"}]}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid"}},
		Body:       io.NopCloser(strings.NewReader(`{"output":[],"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}`)),
	}
	upstream := &httpUpstreamRecorder{resp: resp}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}

	account := &Account{
		ID:             456,
		Name:           "apikey-acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeAPIKey,
		Concurrency:    1,
		Credentials:    map[string]any{"api_key": "sk-api-key", "base_url": "https://api.openai.com"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ServiceTier)
	require.Equal(t, "flex", *result.ServiceTier)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "gpt-5.2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "flex", gjson.GetBytes(upstream.lastBody, "service_tier").String())
	require.Equal(t, int64(128), gjson.GetBytes(upstream.lastBody, "max_output_tokens").Int())
	require.Equal(t, "pro", gjson.GetBytes(upstream.lastBody, "reasoning.mode").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "reasoning.effort").Exists())
	require.Equal(t, "hi", gjson.GetBytes(upstream.lastBody, "input.0.text").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "instructions").Exists())
	require.Equal(t, "https://api.openai.com/v1/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-api-key", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "curl/8.0", upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "remote_compaction_v2", upstream.lastReq.Header.Get("x-codex-beta-features"))
	require.Empty(t, upstream.lastReq.Header.Get("X-Test"))
}

func TestOpenAIGatewayService_ContextCompactionFailoverKeepsCompactBodyAndInboundPath(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("x-codex-beta-features", "remote_compaction_v2")
	originalBody := []byte(`{"model":"gpt-5.6-terra","stream":true,"instructions":"compact safely","input":[{"type":"message","role":"user","content":"long context"}]}`)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"context_length_exceeded","message":"maximum context length exceeded"}}`)),
		},
		{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"rate_limit_exceeded","message":"retry another account"}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-compact-ok"}},
			Body: io.NopCloser(strings.NewReader(
				`{"id":"resp_compact","output":[{"type":"compaction","encrypted_content":"summary"}],"usage":{"input_tokens":2,"output_tokens":1}}`,
			)),
		},
	}}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{
			ForceCodexCLI:      false,
			OpenAICompactModel: "gpt-5.4",
		}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          789,
		Name:        "compact-account",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:       map[string]any{"openai_passthrough": true, "openai_compact_supported": true},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, "/v1/responses", c.Request.URL.Path)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "/backend-api/codex/responses", upstream.requests[0].URL.Path)
	require.Equal(t, "/backend-api/codex/responses/compact", upstream.requests[1].URL.Path)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.bodies[1], "model").String())
	require.Equal(t, "compaction_trigger", gjson.GetBytes(upstream.bodies[1], "input.1.type").String())

	result, err = svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "/v1/responses", c.Request.URL.Path)
	require.Len(t, upstream.requests, 3)
	require.Equal(t, "/backend-api/codex/responses/compact", upstream.requests[2].URL.Path)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.bodies[2], "model").String())
	require.Equal(t, "compaction_trigger", gjson.GetBytes(upstream.bodies[2], "input.1.type").String())
}

func TestOpenAIGatewayService_ContextCompactionSecondOverflowStopsAndLogs(t *testing.T) {
	setGinTestMode()
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("x-codex-beta-features", "remote_compaction_v2")
	originalBody := []byte(`{"model":"gpt-5.6-terra","stream":true,"instructions":"compact safely","input":[{"type":"message","role":"user","content":"long context"}]}`)
	contextOverflow := func(message string) *http.Response {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"code":"context_length_exceeded","message":"` + message + `"}}`,
			)),
		}
	}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		contextOverflow("normal request overflow"),
		contextOverflow("compact request overflow"),
	}}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{
			OpenAICompactModel: "gpt-5.4",
		}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          790,
		Name:        "compact-account",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:       map[string]any{"openai_passthrough": true, "openai_compact_supported": true},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)

	require.Nil(t, result)
	require.Error(t, err)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "/backend-api/codex/responses/compact", upstream.requests[1].URL.Path)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.bodies[1], "model").String())
	require.True(t, logSink.ContainsMessage("openai.context_overflow_remote_compaction_failed"))
	require.True(t, logSink.ContainsFieldValue("compact_model", "gpt-5.4"))
	require.True(t, logSink.ContainsFieldValue("upstream_code", "context_length_exceeded"))
}

func TestOpenAIGatewayService_OAuthPassthrough_WarnOnTimeoutHeadersForStream(t *testing.T) {
	setGinTestMode()
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("x-stainless-timeout", "10000")

	originalBody := []byte(`{"model":"gpt-5.2","stream":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"rid-timeout"}},
		Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
	}
	upstream := &httpUpstreamRecorder{resp: resp}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             321,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.True(t, logSink.ContainsMessage("检测到超时相关请求头，将按配置过滤以降低断流风险"))
	require.True(t, logSink.ContainsFieldValue("timeout_headers", "x-stainless-timeout=10000"))
}

func TestOpenAIGatewayService_OAuthPassthrough_InfoWhenStreamEndsWithoutDone(t *testing.T) {
	setGinTestMode()
	logSink, restore := captureStructuredLog(t)
	defer restore()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")

	originalBody := []byte(`{"model":"gpt-5.2","stream":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)
	// 注意：刻意不发送 [DONE]，模拟上游中途断流。
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"rid-truncate"}},
		Body:       io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"h\"}\n\n")),
	}
	upstream := &httpUpstreamRecorder{resp: resp}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             654,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, originalBody)
	require.EqualError(t, err, "stream usage incomplete: missing terminal event")
	require.True(t, logSink.ContainsMessage("上游流在未收到 [DONE] 时结束，疑似断流"))
	require.True(t, logSink.ContainsMessageAtLevel("上游流在未收到 [DONE] 时结束，疑似断流", "info"))
	require.True(t, logSink.ContainsFieldValue("upstream_request_id", "rid-truncate"))
}

func TestOpenAIGatewayService_OAuthPassthrough_DefaultFiltersTimeoutHeaders(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("x-stainless-timeout", "120000")
	c.Request.Header.Set("X-Test", "keep")

	originalBody := []byte(`{"model":"gpt-5.2","stream":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"rid-filter-default"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
			"",
			"data: [DONE]",
			"",
		}, "\n"))),
	}
	upstream := &httpUpstreamRecorder{resp: resp}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             111,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Empty(t, upstream.lastReq.Header.Get("x-stainless-timeout"))
	require.Empty(t, upstream.lastReq.Header.Get("X-Test"))
}

func TestOpenAIGatewayService_OAuthPassthrough_AllowTimeoutHeadersWhenConfigured(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("x-stainless-timeout", "120000")
	c.Request.Header.Set("X-Test", "keep")

	originalBody := []byte(`{"model":"gpt-5.2","stream":true,"instructions":"local-test-instructions","input":[{"type":"text","text":"hi"}]}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "X-Request-Id": []string{"rid-filter-allow"}},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}}`,
			"",
			"data: [DONE]",
			"",
		}, "\n"))),
	}
	upstream := &httpUpstreamRecorder{resp: resp}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{Gateway: config.GatewayConfig{
			ForceCodexCLI:                        false,
			OpenAIPassthroughAllowTimeoutHeaders: true,
		}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             222,
		Name:           "acc",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeOAuth,
		Concurrency:    1,
		Credentials:    map[string]any{"access_token": "oauth-token", "chatgpt_account_id": "chatgpt-acc"},
		Extra:          map[string]any{"openai_passthrough": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "120000", upstream.lastReq.Header.Get("x-stainless-timeout"))
	require.Empty(t, upstream.lastReq.Header.Get("X-Test"))
}
