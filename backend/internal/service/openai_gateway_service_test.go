package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/cespare/xxhash/v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 编译期接口断言
var _ AccountRepository = (*stubOpenAIAccountRepo)(nil)
var _ GatewayCache = (*stubGatewayCache)(nil)

type stubOpenAIAccountRepo struct {
	AccountRepository
	accounts []Account
}

type snapshotUpdateAccountRepo struct {
	stubOpenAIAccountRepo
	updateExtraCalls chan map[string]any
}

type ttftCooldownCall struct {
	id     int64
	until  time.Time
	reason string
}

type ttftCooldownAccountRepoStub struct {
	AccountRepository
	calls []ttftCooldownCall
}

type openAISettingRepoStub struct {
	values map[string]string
}

func (s *openAISettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	if s == nil || s.values == nil {
		return nil, ErrSettingNotFound
	}
	value, ok := s.values[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (s *openAISettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s == nil || s.values == nil {
		return "", nil
	}
	return s.values[key], nil
}

func (s *openAISettingRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	s.values[key] = value
	return nil
}

func (s *openAISettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *openAISettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *openAISettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *openAISettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func (r *snapshotUpdateAccountRepo) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	if r.updateExtraCalls != nil {
		copied := make(map[string]any, len(updates))
		for k, v := range updates {
			copied[k] = v
		}
		r.updateExtraCalls <- copied
	}
	return nil
}

func (r *ttftCooldownAccountRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.calls = append(r.calls, ttftCooldownCall{id: id, until: until, reason: reason})
	return nil
}

func (r stubOpenAIAccountRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	for i := range r.accounts {
		if r.accounts[i].ID == id {
			return &r.accounts[i], nil
		}
	}
	return nil, errors.New("account not found")
}

func (r stubOpenAIAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r stubOpenAIAccountRepo) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r stubOpenAIAccountRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return r.ListSchedulableByPlatform(ctx, platform)
}

func (r stubOpenAIAccountRepo) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	result := make([]Account, 0, len(r.accounts))
	result = append(result, r.accounts...)
	return result, nil
}

type groupAwareStubOpenAIAccountRepo struct {
	stubOpenAIAccountRepo
}

func (r groupAwareStubOpenAIAccountRepo) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform && openAIStickyAccountMatchesGroup(&acc, &groupID) {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r groupAwareStubOpenAIAccountRepo) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform && openAIStickyAccountMatchesGroup(&acc, nil) {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r groupAwareStubOpenAIAccountRepo) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if openAIStickyAccountMatchesGroup(&acc, &groupID) {
			result = append(result, acc)
		}
	}
	return result, nil
}

func (r stubOpenAIAccountRepo) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	var result []Account
	for _, acc := range r.accounts {
		if acc.Platform == platform {
			result = append(result, acc)
		}
	}
	return result, nil
}

type stubConcurrencyCache struct {
	ConcurrencyCache
	loadBatchErr    error
	loadMap         map[int64]*AccountLoadInfo
	acquireResults  map[int64]bool
	acquireMax      map[int64]int
	waitCounts      map[int64]int
	skipDefaultLoad bool
}

type cancelReadCloser struct{}

func (c cancelReadCloser) Read(p []byte) (int, error) { return 0, context.Canceled }
func (c cancelReadCloser) Close() error               { return nil }

type errReadCloser struct {
	err error
}

func (r errReadCloser) Read([]byte) (int, error) { return 0, r.err }
func (r errReadCloser) Close() error             { return nil }

type failingGinWriter struct {
	gin.ResponseWriter
	failAfter int
	writes    int
}

func (w *failingGinWriter) Write(p []byte) (int, error) {
	if w.writes >= w.failAfter {
		return 0, errors.New("write failed")
	}
	w.writes++
	return w.ResponseWriter.Write(p)
}

func (c stubConcurrencyCache) AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
	if c.acquireMax != nil {
		c.acquireMax[accountID] = maxConcurrency
	}
	if c.acquireResults != nil {
		if result, ok := c.acquireResults[accountID]; ok {
			return result, nil
		}
	}
	return true, nil
}

func (c stubConcurrencyCache) ReleaseAccountSlot(ctx context.Context, accountID int64, requestID string) error {
	return nil
}

func (c stubConcurrencyCache) GetAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	if c.loadBatchErr != nil {
		return nil, c.loadBatchErr
	}
	out := make(map[int64]*AccountLoadInfo, len(accounts))
	if c.skipDefaultLoad && c.loadMap != nil {
		for _, acc := range accounts {
			if load, ok := c.loadMap[acc.ID]; ok {
				out[acc.ID] = load
			}
		}
		return out, nil
	}
	for _, acc := range accounts {
		if c.loadMap != nil {
			if load, ok := c.loadMap[acc.ID]; ok {
				out[acc.ID] = load
				continue
			}
		}
		out[acc.ID] = &AccountLoadInfo{AccountID: acc.ID, LoadRate: 0}
	}
	return out, nil
}

func TestOpenAIGatewayService_GenerateSessionHash_Priority(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)

	svc := &OpenAIGatewayService{}

	bodyWithKey := []byte(`{"prompt_cache_key":"ses_aaa"}`)

	// 1) session_id header wins
	c.Request.Header.Set("session_id", "sess-123")
	c.Request.Header.Set("conversation_id", "conv-456")
	h1 := svc.GenerateSessionHash(c, bodyWithKey)
	if h1 == "" {
		t.Fatalf("expected non-empty hash")
	}

	// 2) conversation_id used when session_id absent
	c.Request.Header.Del("session_id")
	h2 := svc.GenerateSessionHash(c, bodyWithKey)
	if h2 == "" {
		t.Fatalf("expected non-empty hash")
	}
	if h1 == h2 {
		t.Fatalf("expected different hashes for different keys")
	}

	// 3) prompt_cache_key used when both headers absent
	c.Request.Header.Del("conversation_id")
	h3 := svc.GenerateSessionHash(c, bodyWithKey)
	if h3 == "" {
		t.Fatalf("expected non-empty hash")
	}
	if h2 == h3 {
		t.Fatalf("expected different hashes for different keys")
	}

	// 4) empty when no signals
	h4 := svc.GenerateSessionHash(c, []byte(`{}`))
	if h4 != "" {
		t.Fatalf("expected empty hash when no signals")
	}
}

func TestOpenAIGatewayService_GenerateSessionHash_UsesXXHash64(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)

	c.Request.Header.Set("session_id", "sess-fixed-value")
	svc := &OpenAIGatewayService{}

	got := svc.GenerateSessionHash(c, nil)
	want := fmt.Sprintf("%016x", xxhash.Sum64String("sess-fixed-value"))
	require.Equal(t, want, got)
}

func TestOpenAIGatewayService_GenerateSessionHash_AttachesLegacyHashToContext(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)

	c.Request.Header.Set("session_id", "sess-legacy-check")
	svc := &OpenAIGatewayService{}

	sessionHash := svc.GenerateSessionHash(c, nil)
	require.NotEmpty(t, sessionHash)
	require.NotNil(t, c.Request)
	require.NotNil(t, c.Request.Context())
	require.NotEmpty(t, openAILegacySessionHashFromContext(c.Request.Context()))
}

func TestOpenAIGatewayService_GenerateSessionHash_IsolatesExplicitSessionByAPIKey(t *testing.T) {
	setGinTestMode()
	svc := &OpenAIGatewayService{}

	newContext := func(apiKeyID int64, sessionID string) *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("session_id", sessionID)
		c.Set("api_key", &APIKey{ID: apiKeyID})
		return c
	}

	sameKeySessionA := svc.GenerateSessionHash(newContext(11, "session-a"), nil)
	sameKeySessionB := svc.GenerateSessionHash(newContext(11, "session-b"), nil)
	require.NotEmpty(t, sameKeySessionA)
	require.NotEmpty(t, sameKeySessionB)
	require.NotEqual(t, sameKeySessionA, sameKeySessionB, "same API key must isolate different explicit sessions")

	keyOneHash := svc.GenerateSessionHash(newContext(11, "shared-session"), nil)
	keyTwoHash := svc.GenerateSessionHash(newContext(12, "shared-session"), nil)
	require.NotEmpty(t, keyOneHash)
	require.NotEmpty(t, keyTwoHash)
	require.NotEqual(t, keyOneHash, keyTwoHash, "different API keys must not share the same raw session namespace")

	keyOneHashAgain := svc.GenerateSessionHash(newContext(11, "shared-session"), nil)
	require.Equal(t, keyOneHash, keyOneHashAgain, "same API key and same explicit session should stay sticky")
}

func TestOpenAIGatewayService_GenerateSessionHash_NoExplicitSignalDoesNotCreateSharedSession(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)
	c.Set("api_key", &APIKey{ID: 11})

	svc := &OpenAIGatewayService{}
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"Hello"}]}`)

	require.Empty(t, svc.GenerateSessionHash(c, body), "content alone is not a stable client session boundary")
}

func TestOpenAIGatewayService_GenerateExplicitSessionHash_SkipsContentFallback(t *testing.T) {
	setGinTestMode()
	svc := &OpenAIGatewayService{}
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat"}`)

	t.Run("stateless image body stays unstuck", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

		require.Empty(t, svc.GenerateExplicitSessionHash(c, body))
		require.Empty(t, openAILegacySessionHashFromContext(c.Request.Context()))
	})

	t.Run("prompt_cache_key is explicit", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

		got := svc.GenerateExplicitSessionHash(c, []byte(`{"model":"gpt-image-2","prompt_cache_key":"image-session"}`))
		require.Equal(t, fmt.Sprintf("%016x", xxhash.Sum64String("image-session")), got)
		require.NotEmpty(t, openAILegacySessionHashFromContext(c.Request.Context()))
	})

	t.Run("header overrides body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
		c.Request.Header.Set("session_id", "header-session")

		got := svc.GenerateExplicitSessionHash(c, []byte(`{"prompt_cache_key":"body-session"}`))
		require.Equal(t, fmt.Sprintf("%016x", xxhash.Sum64String("header-session")), got)
	})
}

func TestOpenAIGatewayService_GenerateSessionHashWithFallback(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)

	svc := &OpenAIGatewayService{}
	seed := "openai_ws_ingress:9:100:200"

	got := svc.GenerateSessionHashWithFallback(c, []byte(`{}`), seed)
	want := fmt.Sprintf("%016x", xxhash.Sum64String(seed))
	require.Equal(t, want, got)
	require.NotEmpty(t, openAILegacySessionHashFromContext(c.Request.Context()))

	empty := svc.GenerateSessionHashWithFallback(c, []byte(`{}`), "   ")
	require.Equal(t, "", empty)
}

func TestOpenAIGatewayService_GenerateSessionHashWithFallback_UsesFallbackWhenNoExplicitSignal(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Set("api_key", &APIKey{ID: 11})

	svc := &OpenAIGatewayService{}
	seed := "openai_ws_ingress_conn:local-request-1"
	body := []byte(`{"model":"gpt-5.4","input":[{"type":"input_text","text":"hello"}]}`)

	got := svc.GenerateSessionHashWithFallback(c, body, seed)
	want := fmt.Sprintf("%016x", xxhash.Sum64String(seed))
	require.Equal(t, want, got, "WS ingress fallback must be connection/request scoped, not content scoped")
}

func TestOpenAIGatewayService_ResolveOpenAICompactSessionID_ContentFallbackIsDeterministic(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)

	body := []byte(`{"model":"gpt-5.5","instructions":"compact-test","input":[{"type":"message","role":"user","content":"hello"}]}`)
	first := resolveOpenAICompactSessionID(c, body)
	second := resolveOpenAICompactSessionID(c, body)
	other := resolveOpenAICompactSessionID(c, []byte(`{"model":"gpt-5.5","instructions":"compact-test","input":[{"type":"message","role":"user","content":"different"}]}`))

	require.NotEmpty(t, first)
	require.Equal(t, first, second)
	require.True(t, strings.HasPrefix(first, contentSessionSeedPrefix))
	require.NotEqual(t, first, other)
	require.LessOrEqual(t, len(first), 64)
	require.NotContains(t, first, "compact-test")
	require.NotContains(t, first, "hello")
	require.NotContains(t, first, "different")
}

func TestOpenAIGatewayService_GenerateSessionHash_ContentDoesNotCreateStickySession(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)

	svc := &OpenAIGatewayService{}

	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"Hello"}]}`)

	hash := svc.GenerateSessionHash(c, body)
	require.Empty(t, hash, "content alone is not a reliable client session boundary")
}

func TestClassifyOpenAICodexCompatFallback(t *testing.T) {
	t.Run("call_id schema mismatch", func(t *testing.T) {
		reason := classifyOpenAICodexCompatFallback(
			http.StatusBadRequest,
			"",
			"Invalid schema for field input[1].call_id",
			nil,
		)
		require.Equal(t, "call_id", reason)
	})

	t.Run("function call output missing matching tool call", func(t *testing.T) {
		reason := classifyOpenAICodexCompatFallback(
			http.StatusBadRequest,
			"",
			"No tool call found for function call output with call_id call_m5e8pq2TZm6njNl9TYxWq7lZ.",
			nil,
		)
		require.Equal(t, "call_id", reason)
	})

	t.Run("item_reference schema mismatch", func(t *testing.T) {
		reason := classifyOpenAICodexCompatFallback(
			http.StatusBadRequest,
			"",
			"Invalid schema for field input[0].item_reference",
			nil,
		)
		require.Equal(t, "item_reference", reason)
	})

	t.Run("invalid encrypted content excluded", func(t *testing.T) {
		reason := classifyOpenAICodexCompatFallback(
			http.StatusBadRequest,
			"invalid_encrypted_content",
			"invalid encrypted content",
			nil,
		)
		require.Empty(t, reason)
	})

	t.Run("generic bad request not retried", func(t *testing.T) {
		reason := classifyOpenAICodexCompatFallback(
			http.StatusBadRequest,
			"",
			"Bad request",
			nil,
		)
		require.Empty(t, reason)
	})
}

func TestOpenAIGatewayService_Forward_RetriesCodexCompatFallbackOnce(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex-tui/0.125.0")
	c.Request.Header.Set("Accept", "text/event-stream")

	upstream := &httpUpstreamSequenceRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Invalid schema for field input[1].call_id"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-codex-fallback"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_codex_fallback\",\"model\":\"gpt-5.4\",\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"input_tokens_details\":{\"cached_tokens\":1}}}}\n\n",
				)),
			},
		},
	}

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Gateway.OpenAIWS.Enabled = false
	svc := &OpenAIGatewayService{
		cfg:          cfg,
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          1,
		Name:        "oauth-codex",
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

	body := []byte(`{"model":"gpt-5.4","stream":true,"input":[{"type":"item_reference","id":"call_1"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, upstream.callCount)
	require.Equal(t, "call_1", gjson.GetBytes(upstream.bodies[0], "input.0.id").String())
	require.Equal(t, "call_1", gjson.GetBytes(upstream.bodies[0], "input.1.call_id").String())
	require.Equal(t, "fc_1", gjson.GetBytes(upstream.bodies[1], "input.0.id").String())
	require.Equal(t, "fc_1", gjson.GetBytes(upstream.bodies[1], "input.1.call_id").String())
	require.True(t, c.GetBool(openAICodexCompatFallbackKey))
	require.Equal(t, "call_id", c.GetString(openAICodexCompatFallbackReasonKey))
}

func TestOpenAIGatewayService_Forward_RetriesCodexCompatFallbackOnceOnMissingToolCall(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex-tui/0.125.0")
	c.Request.Header.Set("Accept", "text/event-stream")

	upstream := &httpUpstreamSequenceRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"error":{"message":"No tool call found for function call output with call_id call_1.","type":"invalid_request_error","param":"input"}}`,
				)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-codex-fallback-tool-call"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_codex_fallback_tool_call\",\"model\":\"gpt-5.4\",\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"input_tokens_details\":{\"cached_tokens\":1}}}}\n\n",
				)),
			},
		},
	}

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Gateway.OpenAIWS.Enabled = false
	svc := &OpenAIGatewayService{
		cfg:          cfg,
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          11,
		Name:        "oauth-codex-tool-call",
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

	body := []byte(`{"model":"gpt-5.4","stream":true,"input":[{"type":"item_reference","id":"call_1"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, upstream.callCount)
	require.Equal(t, "call_1", gjson.GetBytes(upstream.bodies[0], "input.0.id").String())
	require.Equal(t, "call_1", gjson.GetBytes(upstream.bodies[0], "input.1.call_id").String())
	require.Equal(t, "fc_1", gjson.GetBytes(upstream.bodies[1], "input.0.id").String())
	require.Equal(t, "fc_1", gjson.GetBytes(upstream.bodies[1], "input.1.call_id").String())
	require.True(t, c.GetBool(openAICodexCompatFallbackKey))
	require.Equal(t, "call_id", c.GetString(openAICodexCompatFallbackReasonKey))
}

func TestApplyOpenAIWSCodexCompatFallback_RewritesToolContinuationIDs(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": []any{
			map[string]any{"type": "item_reference", "id": "call_1"},
			map[string]any{"type": "function_call_output", "call_id": "call_1", "output": "ok"},
		},
		"stream": true,
		"store":  false,
	}

	result := applyOpenAIWSCodexCompatFallback(reqBody, c, false, false, "call_id")
	require.True(t, result.Modified)
	require.True(t, c.GetBool(openAICodexCompatFallbackKey))
	require.Equal(t, "call_id", c.GetString(openAICodexCompatFallbackReasonKey))

	input, ok := reqBody["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 2)

	first, ok := input[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fc_1", first["id"])

	second, ok := input[1].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "fc_1", second["call_id"])
}

func TestOpenAIGatewayService_Forward_CodexCompatFallbackStopsAfterOneRetry(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex-tui/0.125.0")
	c.Request.Header.Set("Accept", "text/event-stream")

	upstream := &httpUpstreamSequenceRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Invalid schema for field input[1].call_id"}}`)),
			},
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Invalid schema for field input[1].call_id"}}`)),
			},
		},
	}

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Gateway.OpenAIWS.Enabled = false
	svc := &OpenAIGatewayService{
		cfg:          cfg,
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          2,
		Name:        "oauth-codex-once",
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

	body := []byte(`{"model":"gpt-5.4","stream":true,"input":[{"type":"item_reference","id":"call_1"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, 2, upstream.callCount)
	require.Equal(t, "call_1", gjson.GetBytes(upstream.bodies[0], "input.1.call_id").String())
	require.Equal(t, "fc_1", gjson.GetBytes(upstream.bodies[1], "input.1.call_id").String())
}

func TestOpenAIGatewayService_Forward_DowngradesOrphanToolRoleWithoutContinuationContext(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex-tui/0.125.0")
	c.Request.Header.Set("Accept", "text/event-stream")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"stop after capture"}}`)),
	}}

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Gateway.OpenAIWS.Enabled = false
	svc := &OpenAIGatewayService{
		cfg:          cfg,
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          21,
		Name:        "oauth-codex-orphan-tool-role",
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

	body := []byte(`{"model":"gpt-5.4","stream":true,"input":[{"type":"message","role":"tool","tool_call_id":"call_1","content":"ok"}]}`)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, "message", gjson.GetBytes(upstream.lastBody, "input.0.type").String())
	require.Equal(t, "user", gjson.GetBytes(upstream.lastBody, "input.0.role").String())
	require.Equal(t, "ok", gjson.GetBytes(upstream.lastBody, "input.0.content").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input.0.call_id").Exists())
}

func TestOpenAIGatewayService_Forward_CodexCompatFallbackDropsOrdinaryIDsForInputSchema(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex-tui/0.125.0")
	c.Request.Header.Set("Accept", "text/event-stream")

	upstream := &httpUpstreamSequenceRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Invalid schema for field input[0].id"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-codex-id-fallback"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_codex_id_fallback\",\"model\":\"gpt-5.4\",\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"input_tokens_details\":{\"cached_tokens\":0}}}}\n\n",
				)),
			},
		},
	}

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Gateway.OpenAIWS.Enabled = false
	svc := &OpenAIGatewayService{
		cfg:          cfg,
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          3,
		Name:        "oauth-codex-input-schema",
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

	body := []byte(`{"model":"gpt-5.4","stream":true,"input":[{"type":"message","id":"msg_123","role":"user","content":"hello"}]}`)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, upstream.callCount)
	require.Equal(t, "msg_123", gjson.GetBytes(upstream.bodies[0], "input.0.id").String())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "input.0.id").Exists())
	require.True(t, c.GetBool(openAICodexCompatFallbackKey))
	require.Equal(t, "input_schema", c.GetString(openAICodexCompatFallbackReasonKey))
}

func TestOpenAIGatewayService_Forward_OAuthOrderedMarshalPreservesPromptCacheFriendlyPrefix(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	originalBody := []byte(`{
		"model":"gpt-5.2",
		"stream":false,
		"store":true,
		"prompt_cache_key":"cache-ordered-123",
		"instructions":"local-test-instructions",
		"input":"hello ordered world"
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "text/event-stream")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-ordered"}},
			Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
		},
	}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          321,
		Name:        "oauth-ordered",
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
	require.NotNil(t, upstream.lastReq)
	requireOrderedJSONKeys(t, upstream.lastBody, "model", "instructions", "prompt_cache_key", "input")
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "store").Bool())
	require.Equal(t, "cache-ordered-123", gjson.GetBytes(upstream.lastBody, "prompt_cache_key").String())
	require.Equal(t, "local-test-instructions", gjson.GetBytes(upstream.lastBody, "instructions").String())
	require.Equal(t, "hello ordered world", gjson.GetBytes(upstream.lastBody, "input.0.content").String())
}

func TestOpenAIGatewayService_GenerateSessionHash_ExplicitSignalWinsOverContent(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)

	svc := &OpenAIGatewayService{}
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"Hello"}]}`)

	contentHash := svc.GenerateSessionHash(c, body)
	require.Empty(t, contentHash)

	c.Request.Header.Set("session_id", "explicit-session")
	explicitHash := svc.GenerateSessionHash(c, body)
	require.NotEmpty(t, explicitHash)
}

func TestOpenAIGatewayService_Forward_StripsUnsupportedFieldsConsistently(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{
		"model":"gpt-5.1-codex",
		"input":"hello",
		"stream":false,
		"temperature":0.2,
		"prompt_cache_retention":"24h",
		"reasoningSummary":"auto",
		"safety_identifier":"safe-id",
		"metadata":{"k":"v"},
		"stream_options":{"include_usage":true}
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-strip-unsupported"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_strip_1","usage":{"input_tokens":1,"output_tokens":1}}`)),
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          321,
		Name:        "oauth-strip",
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

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.lastBody, "instructions").String()))
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "temperature").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "prompt_cache_retention").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "reasoningSummary").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "safety_identifier").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "metadata").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream_options").Exists())
	require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "input.0.content").String())
	require.False(t, result.Stream)
}

func TestOpenAIGatewayService_Forward_CodexCLIStripsMessageNoiseFields(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{
		"model":"gpt-5.2",
		"store":false,
		"stream":false,
		"input":[{
			"id":null,
			"cwd":null,
			"name":null,
			"role":"user",
			"type":"message",
			"input":null,
			"action":null,
			"output":null,
			"stderr":null,
			"stdout":null,
			"call_id":null,
			"changes":null,
			"command":null,
			"content":"health check",
			"thought":null,
			"arguments":null,
			"encrypted_content":null,
			"reasoning_content":null,
			"thought_signature":"[REDACTED]",
			"working_directory":null
		}]
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.125.0")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-codex-message-clean"}},
			Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          654,
		Name:        "oauth-codex-message-clean",
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

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "health check", gjson.GetBytes(upstream.lastBody, "input.0.content").String())
	require.Equal(t, "message", gjson.GetBytes(upstream.lastBody, "input.0.type").String())
	require.Equal(t, "user", gjson.GetBytes(upstream.lastBody, "input.0.role").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input.0.action").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input.0.cwd").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input.0.command").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input.0.stdout").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input.0.thought_signature").Exists())
}

func TestOpenAIGatewayService_OAuthLegacy_ClaudeCLIResponsesInjectsInstructionsAndNormalizesStrictToolSchema(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{
		"model":"gpt-5.4",
		"stream":true,
		"text":{"verbosity":"medium"},
		"input":[{"role":"developer","type":"message","content":[{"type":"input_text","text":"You are Claude Code."}]},{"role":"user","type":"message","content":[{"type":"input_text","text":"hi"}]}],
		"tools":[{"name":"read_file","type":"function","strict":true,"parameters":{"type":"object","$schema":"http://json-schema.org/draft-07/schema#","required":["filePath"],"properties":{"limit":{"type":"number"},"offset":{"type":"number"},"filePath":{"type":"string"}},"additionalProperties":false}}]
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "claude-cli/2.1.92 (external, cli)")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid-claude-cli"}},
			Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          38056,
		Name:        "oauth-claude-cli",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Extra: map[string]any{
			"openai_passthrough": false,
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, strings.TrimSpace(gjson.GetBytes(upstream.lastBody, "instructions").String()))
	require.Equal(t, "filePath", gjson.GetBytes(upstream.lastBody, "tools.0.parameters.required.0").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "tools.0.parameters.required.1").Exists())
}

func TestOpenAIGatewayService_GenerateSessionHash_EmptyBodyStillEmpty(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", nil)

	svc := &OpenAIGatewayService{}
	require.Empty(t, svc.GenerateSessionHash(c, []byte(`{}`)))
	require.Empty(t, svc.GenerateSessionHash(c, nil))
}

func (c stubConcurrencyCache) GetAccountWaitingCount(ctx context.Context, accountID int64) (int, error) {
	if c.waitCounts != nil {
		if count, ok := c.waitCounts[accountID]; ok {
			return count, nil
		}
	}
	return 0, nil
}

type stubGatewayCache struct {
	sessionBindings map[string]int64
	deletedSessions map[string]int
}

func (c *stubGatewayCache) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	if id, ok := c.sessionBindings[sessionHash]; ok {
		return id, nil
	}
	return 0, errors.New("not found")
}

func (c *stubGatewayCache) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	if c.sessionBindings == nil {
		c.sessionBindings = make(map[string]int64)
	}
	c.sessionBindings[sessionHash] = accountID
	return nil
}

func (c *stubGatewayCache) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	return nil
}

func (c *stubGatewayCache) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	if c.sessionBindings == nil {
		return nil
	}
	if c.deletedSessions == nil {
		c.deletedSessions = make(map[string]int)
	}
	c.deletedSessions[sessionHash]++
	delete(c.sessionBindings, sessionHash)
	return nil
}

func TestOpenAISelectAccountWithLoadAwareness_FiltersUnschedulable(t *testing.T) {
	now := time.Now()
	resetAt := now.Add(10 * time.Minute)
	groupID := int64(1)

	rateLimited := Account{
		ID:               1,
		Platform:         PlatformOpenAI,
		Type:             AccountTypeAPIKey,
		Status:           StatusActive,
		Schedulable:      true,
		Concurrency:      1,
		Priority:         0,
		RateLimitResetAt: &resetAt,
	}
	available := Account{
		ID:          2,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    1,
	}

	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{rateLimited, available}},
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-5.2", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil {
		t.Fatalf("expected selection with account")
	}
	if selection.Account.ID != available.ID {
		t.Fatalf("expected account %d, got %d", available.ID, selection.Account.ID)
	}
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAISelectAccountWithLoadAwareness_ImageRateLimitSkipsOnlyImageRequests(t *testing.T) {
	future := time.Now().Add(10 * time.Minute).Format(time.RFC3339)
	groupID := int64(1)

	imageLimited := Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    0,
		Extra: map[string]any{
			modelRateLimitsKey: map[string]any{
				openAIImageGenerationRateLimitKey: map[string]any{
					"rate_limit_reset_at": future,
				},
			},
		},
	}
	available := Account{
		ID:          2,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    1,
	}
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{imageLimited, available}},
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
	}

	imageSelection, err := svc.SelectAccountWithLoadAwareness(WithOpenAIImageGenerationIntent(context.Background()), &groupID, "", "gpt-5.4", nil)
	require.NoError(t, err)
	require.NotNil(t, imageSelection)
	require.Equal(t, available.ID, imageSelection.Account.ID)
	if imageSelection.ReleaseFunc != nil {
		imageSelection.ReleaseFunc()
	}

	textSelection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-5.4", nil)
	require.NoError(t, err)
	require.NotNil(t, textSelection)
	require.Equal(t, imageLimited.ID, textSelection.Account.ID)
	if textSelection.ReleaseFunc != nil {
		textSelection.ReleaseFunc()
	}
}

func TestOpenAISelectAccountWithLoadAwareness_FiltersUnschedulableWhenNoConcurrencyService(t *testing.T) {
	now := time.Now()
	resetAt := now.Add(10 * time.Minute)
	groupID := int64(1)

	rateLimited := Account{
		ID:               1,
		Platform:         PlatformOpenAI,
		Type:             AccountTypeAPIKey,
		Status:           StatusActive,
		Schedulable:      true,
		Concurrency:      1,
		Priority:         0,
		RateLimitResetAt: &resetAt,
	}
	available := Account{
		ID:          2,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Priority:    1,
	}

	svc := &OpenAIGatewayService{
		accountRepo: stubOpenAIAccountRepo{accounts: []Account{rateLimited, available}},
		// concurrencyService is nil, forcing the non-load-batch selection path.
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-5.2", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil {
		t.Fatalf("expected selection with account")
	}
	if selection.Account.ID != available.ID {
		t.Fatalf("expected account %d, got %d", available.ID, selection.Account.ID)
	}
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAISelectAccountForModelWithExclusions_StickyUnschedulableClearsSession(t *testing.T) {
	sessionHash := "session-1"
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusDisabled, Schedulable: true, Concurrency: 1},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1},
		},
	}
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{"openai:" + sessionHash: 1},
	}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       cache,
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, sessionHash, "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountForModelWithExclusions error: %v", err)
	}
	if acc == nil || acc.ID != 2 {
		t.Fatalf("expected account 2, got %+v", acc)
	}
	if cache.deletedSessions["openai:"+sessionHash] != 1 {
		t.Fatalf("expected sticky session to be deleted")
	}
	if cache.sessionBindings["openai:"+sessionHash] != 2 {
		t.Fatalf("expected sticky session to bind to account 2")
	}
}

func TestOpenAISelectAccountForModelWithExclusions_StickyOutsideGroupClearsSession(t *testing.T) {
	sessionHash := "session-outside-group"
	groupID := int64(1001)
	repo := groupAwareStubOpenAIAccountRepo{
		stubOpenAIAccountRepo{
			accounts: []Account{
				{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1},
				{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, AccountGroups: []AccountGroup{{GroupID: groupID}}},
			},
		},
	}
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{"openai:" + sessionHash: 1},
	}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       cache,
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), &groupID, sessionHash, "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountForModelWithExclusions error: %v", err)
	}
	if acc == nil || acc.ID != 2 {
		t.Fatalf("expected account 2, got %+v", acc)
	}
	if cache.deletedSessions["openai:"+sessionHash] != 1 {
		t.Fatalf("expected sticky session to be deleted")
	}
	if cache.sessionBindings["openai:"+sessionHash] != 2 {
		t.Fatalf("expected sticky session to bind to account 2")
	}
}

func TestOpenAISelectAccountWithLoadAwareness_StickyUnschedulableClearsSession(t *testing.T) {
	sessionHash := "session-2"
	groupID := int64(1)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusDisabled, Schedulable: true, Concurrency: 1},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1},
		},
	}
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{"openai:" + sessionHash: 1},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, sessionHash, "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil || selection.Account.ID != 2 {
		t.Fatalf("expected account 2, got %+v", selection)
	}
	if cache.deletedSessions["openai:"+sessionHash] != 1 {
		t.Fatalf("expected sticky session to be deleted")
	}
	if cache.sessionBindings["openai:"+sessionHash] != 2 {
		t.Fatalf("expected sticky session to bind to account 2")
	}
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAISelectAccountWithLoadAwareness_StickyOutsideGroupClearsSession(t *testing.T) {
	sessionHash := "session-load-outside-group"
	groupID := int64(1002)
	repo := groupAwareStubOpenAIAccountRepo{
		stubOpenAIAccountRepo{
			accounts: []Account{
				{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1},
				{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, AccountGroups: []AccountGroup{{GroupID: groupID}}},
			},
		},
	}
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{"openai:" + sessionHash: 1},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, sessionHash, "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil || selection.Account.ID != 2 {
		t.Fatalf("expected account 2, got %+v", selection)
	}
	if cache.deletedSessions["openai:"+sessionHash] != 1 {
		t.Fatalf("expected sticky session to be deleted")
	}
	if cache.sessionBindings["openai:"+sessionHash] != 2 {
		t.Fatalf("expected sticky session to bind to account 2")
	}
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAISelectAccountForModelWithExclusions_NoModelSupport(t *testing.T) {
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{
				ID:          1,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"}},
			},
		},
	}
	cache := &stubGatewayCache{}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       cache,
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, "", "gpt-4", nil)
	if err == nil {
		t.Fatalf("expected error for unsupported model")
	}
	if acc != nil {
		t.Fatalf("expected nil account for unsupported model")
	}
	var modelErr *GroupModelUnsupportedError
	require.ErrorAs(t, err, &modelErr)
	require.Equal(t, PlatformOpenAI, modelErr.Platform)
	require.Equal(t, "gpt-4", modelErr.RequestedModel)
	require.Contains(t, modelErr.AvailableModels, "gpt-3.5-turbo")
}

func TestOpenAISelectAccountForModelWithExclusions_ModelSupportedButUnavailableKeepsGenericError(t *testing.T) {
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{
				ID:          1,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"}},
			},
			{
				ID:          2,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-4": "gpt-4"}},
				Extra: map[string]any{
					modelRateLimitsKey: map[string]any{
						"gpt-4": map[string]any{
							"rate_limit_reset_at": time.Now().Add(10 * time.Minute).Format(time.RFC3339),
						},
					},
				},
			},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       &stubGatewayCache{},
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, "", "gpt-4", nil)
	require.Error(t, err)
	require.Nil(t, acc)
	var modelErr *GroupModelUnsupportedError
	require.NotErrorAs(t, err, &modelErr)
}

func TestOpenAISelectAccountForModelWithExclusions_ExcludedUnsupportedKeepsGenericError(t *testing.T) {
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{
				ID:          1,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"}},
			},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       &stubGatewayCache{},
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, "", "gpt-4", map[int64]struct{}{1: {}})
	require.Error(t, err)
	require.Nil(t, acc)
	var modelErr *GroupModelUnsupportedError
	require.NotErrorAs(t, err, &modelErr)
}

func TestOpenAISelectAccountForModelWithExclusions_RuntimeBlockedUnsupportedKeepsGenericError(t *testing.T) {
	account := Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"}},
	}
	repo := stubOpenAIAccountRepo{
		accounts: []Account{account},
	}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       &stubGatewayCache{},
	}
	svc.BlockAccountScheduling(&account, time.Now().Add(10*time.Minute), "test")

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, "", "gpt-4", nil)
	require.Error(t, err)
	require.Nil(t, acc)
	var modelErr *GroupModelUnsupportedError
	require.NotErrorAs(t, err, &modelErr)
}

func TestOpenAISelectAccountForModelWithExclusions_StaleSnapshotUnavailableUnsupportedKeepsGenericError(t *testing.T) {
	stale := &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"}},
	}
	latest := Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Status:      StatusError,
		Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"}},
	}
	cache := &snapshotHydrationCache{
		snapshot: []*Account{stale},
		accounts: map[int64]*Account{1: stale},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{latest}}
	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              &stubGatewayCache{},
		schedulerSnapshot:  NewSchedulerSnapshotService(cache, nil, nil, nil, nil),
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, "", "gpt-4", nil)
	require.Error(t, err)
	require.Nil(t, acc)
	var modelErr *GroupModelUnsupportedError
	require.NotErrorAs(t, err, &modelErr)
}

func TestOpenAISelectAccountForModelWithExclusions_ExcludedModelSupportedKeepsGenericError(t *testing.T) {
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{
				ID:          1,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"}},
			},
			{
				ID:          2,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-4": "gpt-4"}},
			},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       &stubGatewayCache{},
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, "", "gpt-4", map[int64]struct{}{2: {}})
	require.Error(t, err)
	require.Nil(t, acc)
	var modelErr *GroupModelUnsupportedError
	require.NotErrorAs(t, err, &modelErr)
}

func TestOpenAISelectAccountForModelWithExclusions_RuntimeBlockedModelSupportedKeepsGenericError(t *testing.T) {
	supported := Account{
		ID:          2,
		Platform:    PlatformOpenAI,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-4": "gpt-4"}},
	}
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{
				ID:          1,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"}},
			},
			supported,
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       &stubGatewayCache{},
	}
	svc.BlockAccountScheduling(&supported, time.Now().Add(10*time.Minute), "test")

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, "", "gpt-4", nil)
	require.Error(t, err)
	require.Nil(t, acc)
	var modelErr *GroupModelUnsupportedError
	require.NotErrorAs(t, err, &modelErr)
}

func TestOpenAISelectAccountWithLoadAwarenessForImageRoute_RouteIncompatibleUnsupportedKeepsGenericError(t *testing.T) {
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{
				ID:          3,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				Credentials: map[string]any{
					"model_mapping": map[string]any{"gpt-3.5-turbo": "gpt-3.5-turbo"},
					"plan_type":     "free",
				},
			},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       &stubGatewayCache{},
	}

	selection, err := svc.selectAccountWithLoadAwarenessForImageRoute(context.Background(), nil, "", "gpt-4", nil, false, GroupImageGenerationRouteCodex, false)
	require.Error(t, err)
	require.Nil(t, selection)
	var modelErr *GroupModelUnsupportedError
	require.NotErrorAs(t, err, &modelErr)
}

func TestOpenAISelectAccountWithLoadAwareness_LoadBatchErrorFallback(t *testing.T) {
	groupID := int64(1)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 2},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
		},
	}
	cache := &stubGatewayCache{}
	concurrencyCache := stubConcurrencyCache{
		loadBatchErr: errors.New("load batch failed"),
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "fallback", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil {
		t.Fatalf("expected selection")
	}
	if selection.Account.ID != 2 {
		t.Fatalf("expected account 2, got %d", selection.Account.ID)
	}
	if cache.sessionBindings["openai:fallback"] != 2 {
		t.Fatalf("expected sticky session updated")
	}
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestOpenAISelectAccountWithLoadAwareness_NoSlotFallbackWait(t *testing.T) {
	groupID := int64(1)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
		},
	}
	cache := &stubGatewayCache{}
	concurrencyCache := stubConcurrencyCache{
		acquireResults: map[int64]bool{1: false},
		loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, LoadRate: 10},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.WaitPlan == nil {
		t.Fatalf("expected wait plan fallback")
	}
	if selection.Account == nil || selection.Account.ID != 1 {
		t.Fatalf("expected account 1")
	}
}

func TestOpenAISelectAccountForModelWithExclusions_SetsStickyBinding(t *testing.T) {
	sessionHash := "bind"
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
		},
	}
	cache := &stubGatewayCache{}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       cache,
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, sessionHash, "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountForModelWithExclusions error: %v", err)
	}
	if acc == nil || acc.ID != 1 {
		t.Fatalf("expected account 1")
	}
	if cache.sessionBindings["openai:"+sessionHash] != 1 {
		t.Fatalf("expected sticky session binding")
	}
}

func TestOpenAISelectAccountWithLoadAwareness_StickyWaitPlan(t *testing.T) {
	sessionHash := "sticky-wait"
	groupID := int64(1)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
		},
	}
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{"openai:" + sessionHash: 1},
	}
	concurrencyCache := stubConcurrencyCache{
		acquireResults: map[int64]bool{1: false},
		waitCounts:     map[int64]int{1: 0},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, sessionHash, "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.WaitPlan == nil {
		t.Fatalf("expected sticky wait plan")
	}
	if selection.Account == nil || selection.Account.ID != 1 {
		t.Fatalf("expected account 1")
	}
}

func TestOpenAISelectAccountWithLoadAwareness_StickyTempUnschedulableWithinWaitBudgetKeepsSticky(t *testing.T) {
	resetOpenAIStickyWaitTimeoutSettingCacheForTest()
	defer resetOpenAIStickyWaitTimeoutSettingCacheForTest()

	sessionHash := "sticky-temp-wait"
	groupID := int64(1)
	waitUntil := time.Now().Add(20 * time.Second)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{
				ID:                     1,
				Platform:               PlatformOpenAI,
				Status:                 StatusActive,
				Schedulable:            true,
				Concurrency:            1,
				Priority:               1,
				TempUnschedulableUntil: &waitUntil,
			},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 9},
		},
	}
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{"openai:" + sessionHash: 1},
	}
	settingSvc := NewSettingService(&openAISettingRepoStub{
		values: map[string]string{SettingKeyOpenAIStickyWaitTimeoutSeconds: "30"},
	}, &config.Config{})

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		settingService:     settingSvc,
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, sessionHash, "gpt-4", nil)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(1), selection.Account.ID)
	require.False(t, selection.Acquired)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, int64(1), selection.WaitPlan.AccountID)
	require.NotNil(t, selection.WaitPlan.NotBefore)
	require.WithinDuration(t, waitUntil, *selection.WaitPlan.NotBefore, time.Second)
	require.Equal(t, 30*time.Second, selection.WaitPlan.Timeout)
}

func TestOpenAISelectAccountWithLoadAwareness_StickyTempUnschedulableBeyondWaitBudgetSwitchesAndRebinds(t *testing.T) {
	resetOpenAIStickyWaitTimeoutSettingCacheForTest()
	defer resetOpenAIStickyWaitTimeoutSettingCacheForTest()

	sessionHash := "sticky-temp-switch"
	groupID := int64(1)
	waitUntil := time.Now().Add(45 * time.Second)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{
				ID:                     1,
				Platform:               PlatformOpenAI,
				Status:                 StatusActive,
				Schedulable:            true,
				Concurrency:            1,
				Priority:               1,
				TempUnschedulableUntil: &waitUntil,
			},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 9},
		},
	}
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{"openai:" + sessionHash: 1},
	}
	settingSvc := NewSettingService(&openAISettingRepoStub{
		values: map[string]string{SettingKeyOpenAIStickyWaitTimeoutSeconds: "30"},
	}, &config.Config{})

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{acquireResults: map[int64]bool{2: true}}),
		settingService:     settingSvc,
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, sessionHash, "gpt-4", nil)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(2), selection.Account.ID)
	require.True(t, selection.Acquired)
	require.Nil(t, selection.WaitPlan)
	require.Equal(t, 1, cache.deletedSessions["openai:"+sessionHash])
	require.Equal(t, int64(2), cache.sessionBindings["openai:"+sessionHash])
}

func TestOpenAISelectAccountWithLoadAwareness_PrefersLowerLoad(t *testing.T) {
	groupID := int64(1)
	lastUsed := time.Now().Add(-1 * time.Hour)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 10, Priority: 1, LastUsedAt: &lastUsed},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 10, Priority: 1, LastUsedAt: &lastUsed},
		},
	}
	cache := &stubGatewayCache{}
	concurrencyCache := stubConcurrencyCache{
		loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, CurrentConcurrency: 8, LoadRate: 80},
			2: {AccountID: 2, CurrentConcurrency: 1, LoadRate: 10},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "load", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil || selection.Account.ID != 2 {
		t.Fatalf("expected account 2")
	}
	if cache.sessionBindings["openai:load"] != 2 {
		t.Fatalf("expected sticky session updated")
	}
}

func TestOpenAISelectAccountForModelWithExclusions_StickyExcludedFallback(t *testing.T) {
	sessionHash := "excluded"
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 2},
		},
	}
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{"openai:" + sessionHash: 1},
	}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       cache,
	}

	excluded := map[int64]struct{}{1: {}}
	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, sessionHash, "gpt-4", excluded)
	if err != nil {
		t.Fatalf("SelectAccountForModelWithExclusions error: %v", err)
	}
	if acc == nil || acc.ID != 2 {
		t.Fatalf("expected account 2")
	}
}

func TestOpenAISelectAccountForModelWithExclusions_StickyNonOpenAI(t *testing.T) {
	sessionHash := "non-openai"
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformAnthropic, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 2},
		},
	}
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{"openai:" + sessionHash: 1},
	}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       cache,
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, sessionHash, "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountForModelWithExclusions error: %v", err)
	}
	if acc == nil || acc.ID != 2 {
		t.Fatalf("expected account 2")
	}
}

func TestOpenAISelectAccountForModelWithExclusions_NoAccounts(t *testing.T) {
	repo := stubOpenAIAccountRepo{accounts: []Account{}}
	cache := &stubGatewayCache{}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       cache,
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, "", "", nil)
	if err == nil {
		t.Fatalf("expected error for no accounts")
	}
	if acc != nil {
		t.Fatalf("expected nil account")
	}
	if !strings.Contains(err.Error(), "no available OpenAI accounts") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAISelectAccountWithLoadAwareness_NoCandidates(t *testing.T) {
	groupID := int64(1)
	resetAt := time.Now().Add(1 * time.Hour)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, RateLimitResetAt: &resetAt},
		},
	}
	cache := &stubGatewayCache{}
	concurrencyCache := stubConcurrencyCache{}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err == nil {
		t.Fatalf("expected error for no candidates")
	}
	if selection != nil {
		t.Fatalf("expected nil selection")
	}
}

func TestOpenAISelectAccountWithLoadAwareness_AllFullWaitPlan(t *testing.T) {
	groupID := int64(1)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
		},
	}
	cache := &stubGatewayCache{}
	concurrencyCache := stubConcurrencyCache{
		loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, CurrentConcurrency: 1, LoadRate: 100},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.WaitPlan == nil {
		t.Fatalf("expected wait plan")
	}
}

func TestOpenAISelectAccountWithLoadAwareness_LoadBatchErrorNoAcquire(t *testing.T) {
	groupID := int64(1)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
		},
	}
	cache := &stubGatewayCache{}
	concurrencyCache := stubConcurrencyCache{
		loadBatchErr:   errors.New("load batch failed"),
		acquireResults: map[int64]bool{1: false},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.WaitPlan == nil {
		t.Fatalf("expected wait plan")
	}
}

func TestOpenAISelectAccountWithLoadAwareness_MissingLoadInfo(t *testing.T) {
	groupID := int64(1)
	lastUsed := time.Now().Add(-1 * time.Hour)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, LastUsedAt: &lastUsed},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, LastUsedAt: &lastUsed},
		},
	}
	cache := &stubGatewayCache{}
	concurrencyCache := stubConcurrencyCache{
		loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, CurrentConcurrency: 1, LoadRate: 100},
		},
		skipDefaultLoad: true,
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil || selection.Account.ID != 2 {
		t.Fatalf("expected account 2")
	}
}

func TestOpenAISelectAccountForModelWithExclusions_LeastRecentlyUsed(t *testing.T) {
	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Priority: 1, LastUsedAt: &newTime},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Priority: 1, LastUsedAt: &oldTime},
		},
	}
	cache := &stubGatewayCache{}

	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       cache,
	}

	acc, err := svc.SelectAccountForModelWithExclusions(context.Background(), nil, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountForModelWithExclusions error: %v", err)
	}
	if acc == nil || acc.ID != 2 {
		t.Fatalf("expected account 2")
	}
}

func TestOpenAISelectAccountWithLoadAwareness_PreferNeverUsed(t *testing.T) {
	groupID := int64(1)
	lastUsed := time.Now().Add(-1 * time.Hour)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1, LastUsedAt: &lastUsed},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
		},
	}
	cache := &stubGatewayCache{}
	concurrencyCache := stubConcurrencyCache{
		loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, LoadRate: 10},
			2: {AccountID: 2, LoadRate: 10},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil || selection.Account.ID != 2 {
		t.Fatalf("expected account 2")
	}
}

func TestOpenAISelectAccountWithLoadAwareness_PrefersLeastRecentlyUsedBeforeConcurrencyRatio(t *testing.T) {
	groupID := int64(1)
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 10, Priority: 1, LastUsedAt: &older},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 10, Priority: 1, LastUsedAt: &newer},
		},
	}
	cache := &stubGatewayCache{}
	concurrencyCache := stubConcurrencyCache{
		loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, CurrentConcurrency: 4, LoadRate: 40},
			2: {AccountID: 2, CurrentConcurrency: 0, LoadRate: 0},
		},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil || selection.Account.ID != 1 {
		t.Fatalf("expected account 1")
	}
}

func TestOpenAISelectAccountWithLoadAwareness_StickyReservePercentLimitsNewSessions(t *testing.T) {
	resetOpenAIStickyReservePercentSettingCacheForTest()
	defer resetOpenAIStickyReservePercentSettingCacheForTest()

	groupID := int64(1)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 10, Priority: 1},
		},
	}
	cache := &stubGatewayCache{}
	concurrencyCache := stubConcurrencyCache{
		loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, CurrentConcurrency: 8, LoadRate: 80},
		},
	}
	settingRepo := &openAISettingRepoStub{
		values: map[string]string{SettingKeyOpenAIStickyReservePercent: "20"},
	}

	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
		settingService:     NewSettingService(settingRepo, &config.Config{}),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.WaitPlan == nil {
		t.Fatalf("expected wait plan")
	}
	if selection.WaitPlan.MaxConcurrency != 8 {
		t.Fatalf("expected fresh-session wait concurrency 8, got %d", selection.WaitPlan.MaxConcurrency)
	}
}

func TestOpenAISelectionResult_HotUpdatesSchedulerSnapshotLastUsed(t *testing.T) {
	groupID := int64(1)
	account := &Account{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1}
	snapshotCache := &openAISnapshotCacheStub{
		snapshotAccounts: []*Account{account},
		accountsByID:     map[int64]*Account{1: account},
	}
	svc := &OpenAIGatewayService{
		schedulerSnapshot: &SchedulerSnapshotService{cache: snapshotCache},
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{
			loadMap: map[int64]*AccountLoadInfo{
				1: {AccountID: 1, CurrentConcurrency: 0, LoadRate: 0},
			},
		}),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, "", "gpt-4", nil)
	if err != nil {
		t.Fatalf("SelectAccountWithLoadAwareness error: %v", err)
	}
	if selection == nil || selection.Account == nil {
		t.Fatalf("expected account selection")
	}
	if selection.Account.LastUsedAt == nil {
		t.Fatalf("expected selection last_used_at hot-updated")
	}
	if snapshotCache.accountsByID[1] == nil || snapshotCache.accountsByID[1].LastUsedAt == nil {
		t.Fatalf("expected snapshot cache last_used_at hot-updated")
	}
}

func TestSchedulerSnapshotService_ListSchedulableAccounts_DBFallbackOverlaysCachedLastUsed(t *testing.T) {
	oldLastUsed := time.Now().Add(-4 * time.Hour)
	newLastUsed := time.Now().Add(-30 * time.Minute)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{
				ID:          47001,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				LastUsedAt:  &oldLastUsed,
			},
		},
	}
	cache := &openAISnapshotCacheStub{
		accountsByID: map[int64]*Account{
			47001: {
				ID:         47001,
				LastUsedAt: &newLastUsed,
			},
		},
	}
	svc := NewSchedulerSnapshotService(cache, nil, repo, nil, nil)

	accounts, _, err := svc.ListSchedulableAccounts(context.Background(), nil, PlatformOpenAI, false)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.NotNil(t, accounts[0].LastUsedAt)
	require.True(t, accounts[0].LastUsedAt.Equal(newLastUsed))
}

func TestOpenAIGatewayService_ListOpenAIImageCandidateAccounts_OverlaysCachedLastUsed(t *testing.T) {
	oldLastUsed := time.Now().Add(-5 * time.Hour)
	newLastUsed := time.Now().Add(-10 * time.Minute)
	account := Account{
		ID:          47011,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		LastUsedAt:  &oldLastUsed,
	}
	cache := &openAISnapshotCacheStub{
		accountsByID: map[int64]*Account{
			account.ID: {
				ID:         account.ID,
				LastUsedAt: &newLastUsed,
			},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:       stubOpenAIAccountRepo{accounts: []Account{account}},
		schedulerSnapshot: &SchedulerSnapshotService{cache: cache},
		cfg:               &config.Config{},
	}

	accounts, err := svc.listOpenAIImageCandidateAccounts(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.NotNil(t, accounts[0].LastUsedAt)
	require.True(t, accounts[0].LastUsedAt.Equal(newLastUsed))
}

func TestOpenAIGatewayService_SelectOpenAIImageCodexRouteLimitedAccountWaits(t *testing.T) {
	resetAt := time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)
	oldLastUsed := time.Now().Add(-5 * time.Hour)
	newLastUsed := time.Now().Add(-1 * time.Hour)
	accounts := []Account{
		{
			ID:          47021,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 4,
			Priority:    1,
			LastUsedAt:  &oldLastUsed,
			Credentials: map[string]any{"plan_type": "plus"},
			Extra: map[string]any{
				"openai_image_codex_rate_limit_reset_at": resetAt.Format(time.RFC3339),
			},
		},
		{
			ID:          47022,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    1,
			LastUsedAt:  &newLastUsed,
			Credentials: map[string]any{"plan_type": "plus"},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: accounts},
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{acquireResults: map[int64]bool{47021: true, 47022: true}}),
		cfg:                &config.Config{Gateway: config.GatewayConfig{Scheduling: config.GatewaySchedulingConfig{LoadBatchEnabled: true, FallbackWaitTimeout: 30 * time.Second, FallbackMaxWaiting: 10}}},
	}

	selection, err := svc.selectAccountWithLoadAwarenessForImageRoute(context.Background(), nil, "", "gpt-image-1", nil, false, GroupImageGenerationRouteCodex, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(47021), selection.Account.ID)
	require.False(t, selection.Acquired)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, int64(47021), selection.WaitPlan.AccountID)
	require.Equal(t, 2, selection.WaitPlan.MaxConcurrency)
	require.NotNil(t, selection.WaitPlan.NotBefore)
	require.True(t, selection.WaitPlan.NotBefore.Equal(resetAt))
}

func TestOpenAIGatewayService_SelectAccountWithLoadAwarenessForImageRoute_CodexSharesAccountConcurrency(t *testing.T) {
	accounts := []Account{
		{
			ID:          47041,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 6,
			Priority:    1,
			Credentials: map[string]any{"plan_type": "plus"},
		},
	}
	acquireMax := map[int64]int{}
	svc := &OpenAIGatewayService{
		accountRepo: stubOpenAIAccountRepo{accounts: accounts},
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{
			acquireResults: map[int64]bool{47041: true},
			acquireMax:     acquireMax,
		}),
		cfg: &config.Config{},
	}

	selection, err := svc.selectAccountWithLoadAwarenessForImageRoute(context.Background(), nil, "", "gpt-image-1", nil, false, GroupImageGenerationRouteCodex, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(47041), selection.Account.ID)
	require.True(t, selection.Acquired)
	require.Equal(t, 4, acquireMax[47041])
}

func TestOpenAIGatewayService_SelectOpenAIImageCodexSkipsAccountRateLimited(t *testing.T) {
	accountResetAt := time.Now().Add(10 * time.Minute).UTC().Truncate(time.Second)
	imageResetAt := time.Now().Add(20 * time.Second).UTC().Truncate(time.Second)
	oldLastUsed := time.Now().Add(-5 * time.Hour)
	newLastUsed := time.Now().Add(-1 * time.Hour)
	webProfile := map[string]any{
		"user_agent":                 "Mozilla/5.0 (Macintosh; Intel Mac OS X 12_6_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.6950.102 Safari/537.36",
		"accept_language":            "en;q=0.9",
		"sec_ch_ua":                  `"Not.A/Brand";v="8", "Chromium";v="145", "Google Chrome";v="145"`,
		"sec_ch_ua_mobile":           "?0",
		"sec_ch_ua_platform":         `"macOS"`,
		"sec_ch_ua_arch":             `"arm"`,
		"sec_ch_ua_bitness":          `"64"`,
		"sec_ch_ua_full_version":     `"145.0.0.0"`,
		"sec_ch_ua_platform_version": `"15.4.0"`,
		"oai_device_id":              "device-1",
		"oai_session_id":             "session-1",
		"cookies": []any{
			map[string]any{"name": "__Secure-next-auth.session-token", "value": "cookie-value", "domain": ".chatgpt.com", "path": "/"},
		},
	}
	accounts := []Account{
		{
			ID:               47031,
			Platform:         PlatformOpenAI,
			Type:             AccountTypeOAuth,
			Status:           StatusActive,
			Schedulable:      true,
			Concurrency:      1,
			Priority:         1,
			LastUsedAt:       &oldLastUsed,
			RateLimitResetAt: &accountResetAt,
			Credentials:      map[string]any{"plan_type": "plus"},
			Extra: map[string]any{
				"web_profile": map[string]any{
					"user_agent":                 webProfile["user_agent"],
					"accept_language":            webProfile["accept_language"],
					"sec_ch_ua":                  webProfile["sec_ch_ua"],
					"sec_ch_ua_mobile":           webProfile["sec_ch_ua_mobile"],
					"sec_ch_ua_platform":         webProfile["sec_ch_ua_platform"],
					"sec_ch_ua_arch":             webProfile["sec_ch_ua_arch"],
					"sec_ch_ua_bitness":          webProfile["sec_ch_ua_bitness"],
					"sec_ch_ua_full_version":     webProfile["sec_ch_ua_full_version"],
					"sec_ch_ua_platform_version": webProfile["sec_ch_ua_platform_version"],
					"oai_device_id":              webProfile["oai_device_id"],
					"oai_session_id":             webProfile["oai_session_id"],
					"cookies":                    webProfile["cookies"],
				},
				"openai_image_web2api_rate_limit_reset_at": imageResetAt.Format(time.RFC3339),
			},
		},
		{
			ID:          47032,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Priority:    1,
			LastUsedAt:  &newLastUsed,
			Credentials: map[string]any{"plan_type": "plus"},
			Extra: map[string]any{
				"web_profile": map[string]any{
					"user_agent":                 webProfile["user_agent"],
					"accept_language":            webProfile["accept_language"],
					"sec_ch_ua":                  webProfile["sec_ch_ua"],
					"sec_ch_ua_mobile":           webProfile["sec_ch_ua_mobile"],
					"sec_ch_ua_platform":         webProfile["sec_ch_ua_platform"],
					"sec_ch_ua_arch":             webProfile["sec_ch_ua_arch"],
					"sec_ch_ua_bitness":          webProfile["sec_ch_ua_bitness"],
					"sec_ch_ua_full_version":     webProfile["sec_ch_ua_full_version"],
					"sec_ch_ua_platform_version": webProfile["sec_ch_ua_platform_version"],
					"oai_device_id":              "device-2",
					"oai_session_id":             "session-2",
					"cookies": []any{
						map[string]any{"name": "__Secure-next-auth.session-token", "value": "cookie-value", "domain": ".chatgpt.com", "path": "/"},
					},
				},
			},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: accounts},
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{acquireResults: map[int64]bool{47031: true, 47032: true}}),
		cfg:                &config.Config{Gateway: config.GatewayConfig{Scheduling: config.GatewaySchedulingConfig{LoadBatchEnabled: true, FallbackWaitTimeout: 30 * time.Second, FallbackMaxWaiting: 10}}},
	}

	codexSelection, err := svc.selectAccountWithLoadAwarenessForImageRoute(context.Background(), nil, "", "gpt-image-1", nil, false, GroupImageGenerationRouteCodex, false)
	require.NoError(t, err)
	require.NotNil(t, codexSelection)
	require.Equal(t, int64(47032), codexSelection.Account.ID)
	require.True(t, codexSelection.Acquired)

}

func TestOpenAIStreamingTimeout(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 1,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{},
	}

	start := time.Now()
	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1}, start, "model", "model")
	_ = pw.Close()
	_ = pr.Close()

	if err == nil || !strings.Contains(err.Error(), "stream data interval timeout") {
		t.Fatalf("expected stream timeout error, got %v", err)
	}
	if !strings.Contains(rec.Body.String(), "\"type\":\"error\"") || !strings.Contains(rec.Body.String(), "stream_timeout") {
		t.Fatalf("expected OpenAI-compatible error SSE event, got %q", rec.Body.String())
	}
}

func TestOpenAIStreamingContextCanceledReturnsIncompleteErrorWithoutInjectingErrorEvent(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       cancelReadCloser{},
		Header:     http.Header{},
	}

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1}, time.Now(), "model", "model")
	if err == nil || !strings.Contains(err.Error(), "stream usage incomplete") {
		t.Fatalf("expected incomplete stream error, got %v", err)
	}
	if strings.Contains(rec.Body.String(), "event: error") || strings.Contains(rec.Body.String(), "stream_read_error") {
		t.Fatalf("expected no injected SSE error event, got %q", rec.Body.String())
	}
}

func TestOpenAIStreamingReadErrorBeforeOutputReturnsFailover(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       errReadCloser{err: io.ErrUnexpectedEOF},
		Header:     http.Header{"X-Request-Id": []string{"rid-disconnect"}},
	}

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "model", "model")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestOpenAIStreamingResponseFailedBeforeOutputReturnsFailover(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_1"}}`,
			"",
			"event: response.in_progress",
			`data: {"type":"response.in_progress","response":{"id":"resp_1"}}`,
			"",
			"event: response.failed",
			`data: {"type":"response.failed","error":{"message":"An error occurred while processing your request."}}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-failed"}},
	}

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "model", "model")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "An error occurred while processing your request")
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestOpenAIStreamingResponseFailedCapacityBeforeOutputReturnsRetryableFailover(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_1"}}`,
			"",
			"event: response.in_progress",
			`data: {"type":"response.in_progress","response":{"id":"resp_1"}}`,
			"",
			"event: response.failed",
			`data: {"type":"response.failed","error":{"message":"Selected model is at capacity. Please try a different model.","type":"invalid_request_error"}}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-capacity-failed"}},
	}

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "model", "model")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestOpenAIStreamingResponseFailedCapacityAfterOutputReturnsRetryableFailover(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_1"}}`,
			"",
			"event: response.output_text.delta",
			`data: {"type":"response.output_text.delta","delta":"hello"}`,
			"",
			"event: error",
			`data: {"type":"error","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later.","param":null},"sequence_number":2}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-capacity-after-output"}},
	}

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "model", "model")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.True(t, c.Writer.Written())
	require.Contains(t, rec.Body.String(), `"delta":"hello"`)
	require.NotContains(t, rec.Body.String(), "server_is_overloaded")
}

func TestOpenAIStreamingSoftRateLimitAdvisoryBeforeOutputReturnsFailover(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_1"}}`,
			"",
			"event: codex.rate_limits",
			`data: {"type":"codex.rate_limits","metered_limit_name":"codex","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":91,"window_minutes":300,"reset_at":1700000000},"secondary":null},"credits":{"has_credits":false,"unlimited":false,"balance":null}}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-soft-limit"}},
	}

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "model", "model")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestOpenAIStreamingPreambleOnlyMissingTerminalReturnsFailover(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_1"}}`,
			"",
			"event: response.in_progress",
			`data: {"type":"response.in_progress","response":{"id":"resp_1"}}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-missing-terminal"}},
	}

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "model", "model")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestOpenAIStreamingPreambleKeepaliveUsesDownstreamIdle(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   1,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{},
	}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\"}}\n\n"))
		for i := 0; i < 6; i++ {
			time.Sleep(250 * time.Millisecond)
			_, _ = pw.Write([]byte("data: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_1\"}}\n\n"))
		}
		_, _ = pw.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}}\n\n"))
	}()

	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "model", "model")
	_ = pr.Close()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), ":\n\n")
	require.Contains(t, rec.Body.String(), "response.completed")
}

func TestOpenAIStreamingNormalizesResponseDoneEventForHTTPClients(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.done",
			`data: {"type":"response.done","response":{"id":"resp_done","usage":{"input_tokens":2,"output_tokens":3,"input_tokens_details":{"cached_tokens":1}}}}`,
			"",
		}, "\n"))),
		Header: http.Header{},
	}

	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "model", "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 2, result.usage.InputTokens)
	require.Contains(t, rec.Body.String(), "event: response.completed")
	require.Contains(t, rec.Body.String(), `"type":"response.completed"`)
	require.NotContains(t, rec.Body.String(), "response.done")
}

func TestOpenAIStreamingPolicyResponseFailedBeforeOutputPassesThrough(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_1"}}`,
			"",
			"event: response.failed",
			`data: {"type":"response.failed","error":{"type":"safety_error","message":"This request has been flagged for potentially high-risk cyber activity."}}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-policy-failed"}},
	}

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "model", "model")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.True(t, c.Writer.Written())
	require.Contains(t, rec.Body.String(), "response.failed")
	require.Contains(t, rec.Body.String(), "high-risk cyber activity")
}

func TestOpenAIStreamingClientDisconnectDrainsUpstreamUsage(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Writer = &failingGinWriter{ResponseWriter: c.Writer, failAfter: 0}

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{},
	}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"response.in_progress\",\"response\":{}}\n\n"))
		_, _ = pw.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":3,\"output_tokens\":5,\"input_tokens_details\":{\"cached_tokens\":1}}}}\n\n"))
	}()

	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1}, time.Now(), "model", "model")
	_ = pr.Close()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result == nil || result.usage == nil {
		t.Fatalf("expected usage result")
	}
	if result.usage.InputTokens != 3 || result.usage.OutputTokens != 5 || result.usage.CacheReadInputTokens != 1 {
		t.Fatalf("unexpected usage: %+v", *result.usage)
	}
	if strings.Contains(rec.Body.String(), "event: error") || strings.Contains(rec.Body.String(), "write_failed") {
		t.Fatalf("expected no injected SSE error event, got %q", rec.Body.String())
	}
}

func TestOpenAIStreamingMissingTerminalEventReturnsIncompleteError(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{},
	}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"message\"},\"output_index\":0}\n\n"))
	}()

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1}, time.Now(), "model", "model")
	_ = pr.Close()
	if err == nil || !strings.Contains(err.Error(), "missing terminal event") {
		t.Fatalf("expected missing terminal event error, got %v", err)
	}
}

func TestOpenAIStreamingPassthroughMissingTerminalEventReturnsIncompleteError(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{},
	}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"message\"},\"output_index\":0}\n\n"))
	}()

	_, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1}, time.Now(), "", "")
	_ = pr.Close()
	if err == nil || !strings.Contains(err.Error(), "missing terminal event") {
		t.Fatalf("expected missing terminal event error, got %v", err)
	}
}

func TestOpenAIStreamingPassthroughResponseFailedBeforeOutputReturnsFailover(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_1"}}`,
			"",
			"event: response.failed",
			`data: {"type":"response.failed","error":{"message":"upstream processing failed"}}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-passthrough-failed"}},
	}

	_, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "", "")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "upstream processing failed")
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestOpenAIStreamingPassthroughErrorEventServerOverloadedBeforeOutputReturnsRetryableFailover(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_1"}}`,
			"",
			"event: error",
			`data: {"type":"error","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later.","param":null},"sequence_number":2}`,
			"",
			"event: response.failed",
			`data: {"type":"response.failed","response":{"id":"resp_1","status":"failed","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later."}}}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-overloaded-passthrough"}},
	}

	_, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "", "")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestOpenAIStreamingPassthroughErrorEventServerOverloadedAfterOutputReturnsRetryableFailover(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_1"}}`,
			"",
			"event: response.output_text.delta",
			`data: {"type":"response.output_text.delta","delta":"hello"}`,
			"",
			"event: error",
			`data: {"type":"error","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later.","param":null},"sequence_number":2}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-overloaded-after-output"}},
	}

	_, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "", "")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.True(t, c.Writer.Written())
	require.Contains(t, rec.Body.String(), `"delta":"hello"`)
	require.NotContains(t, rec.Body.String(), "server_is_overloaded")
}

func TestOpenAIStreamingRetryAfterPartialOverloadDedupesRetriedPrefix(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}

	firstResp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_first"}}`,
			"",
			"event: response.output_text.delta",
			`data: {"type":"response.output_text.delta","delta":"hello"}`,
			"",
			"event: error",
			`data: {"type":"error","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later.","param":null},"sequence_number":2}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-first"}},
	}

	_, err := svc.handleStreamingResponse(c.Request.Context(), firstResp, c, account, time.Now(), "model", "model")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.True(t, failoverErr.RetryableOnSameAccount)

	secondResp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_retry"}}`,
			"",
			"event: response.output_text.delta",
			`data: {"type":"response.output_text.delta","delta":"hello"}`,
			"",
			"event: response.output_text.delta",
			`data: {"type":"response.output_text.delta","delta":" world"}`,
			"",
			"event: response.completed",
			`data: {"type":"response.completed","response":{"id":"resp_retry","usage":{"input_tokens":4,"output_tokens":2}}}`,
			"",
			`data: [DONE]`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-retry"}},
	}

	result, err := svc.handleStreamingResponse(c.Request.Context(), secondResp, c, account, time.Now(), "model", "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 4, result.usage.InputTokens)
	require.Equal(t, 2, result.usage.OutputTokens)

	body := rec.Body.String()
	require.Equal(t, 1, strings.Count(body, `"type":"response.created"`))
	require.Equal(t, 1, strings.Count(body, `"delta":"hello"`))
	require.Equal(t, 1, strings.Count(body, `"delta":" world"`))
	require.NotContains(t, body, "server_is_overloaded")
}

func TestOpenAIStreamingPassthroughRetryAfterPartialOverloadDedupesRetriedPrefix(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}

	firstResp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_first"}}`,
			"",
			"event: response.output_text.delta",
			`data: {"type":"response.output_text.delta","delta":"hello"}`,
			"",
			"event: error",
			`data: {"type":"error","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later.","param":null},"sequence_number":2}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-first-pass"}},
	}

	_, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), firstResp, c, account, time.Now(), "model", "model")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.True(t, failoverErr.RetryableOnSameAccount)

	secondResp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_retry"}}`,
			"",
			"event: response.output_text.delta",
			`data: {"type":"response.output_text.delta","delta":"hello"}`,
			"",
			"event: response.output_text.delta",
			`data: {"type":"response.output_text.delta","delta":" world"}`,
			"",
			"event: response.completed",
			`data: {"type":"response.completed","response":{"id":"resp_retry","usage":{"input_tokens":4,"output_tokens":2}}}`,
			"",
			`data: [DONE]`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-retry-pass"}},
	}

	result, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), secondResp, c, account, time.Now(), "model", "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 4, result.usage.InputTokens)
	require.Equal(t, 2, result.usage.OutputTokens)

	body := rec.Body.String()
	require.Equal(t, 1, strings.Count(body, `"type":"response.created"`))
	require.Equal(t, 1, strings.Count(body, `"delta":"hello"`))
	require.Equal(t, 1, strings.Count(body, `"delta":" world"`))
	require.NotContains(t, body, "server_is_overloaded")
}

func TestOpenAIStreamingCompletedBeforeTrailingOverloadTreatsStreamAsSuccess(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_done"}}`,
			"",
			"event: response.output_text.delta",
			`data: {"type":"response.output_text.delta","delta":"hello"}`,
			"",
			"event: response.completed",
			`data: {"type":"response.completed","response":{"id":"resp_done","usage":{"input_tokens":6,"output_tokens":3}}}`,
			"",
			"event: error",
			`data: {"type":"error","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later.","param":null},"sequence_number":9}`,
			"",
			`data: [DONE]`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-trailing-overload"}},
	}

	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "model", "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 6, result.usage.InputTokens)
	require.Equal(t, 3, result.usage.OutputTokens)
	require.Contains(t, rec.Body.String(), `"delta":"hello"`)
	require.Contains(t, rec.Body.String(), `"type":"response.completed"`)
	require.NotContains(t, rec.Body.String(), "server_is_overloaded")
}

func TestOpenAIStreamingPassthroughSoftRateLimitAdvisoryBeforeOutputReturnsFailover(t *testing.T) {
	setGinTestMode()
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.created",
			`data: {"type":"response.created","response":{"id":"resp_1"}}`,
			"",
			"event: codex.rate_limits",
			`data: {"type":"codex.rate_limits","metered_limit_name":"codex","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":95,"window_minutes":300,"reset_at":1700000000},"secondary":null},"credits":{"has_credits":false,"unlimited":false,"balance":null}}`,
			"",
		}, "\n"))),
		Header: http.Header{"X-Request-Id": []string{"rid-soft-limit-passthrough"}},
	}

	_, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, time.Now(), "", "")
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestOpenAIStreamingTTFTWatchdogReturnsFailoverAndCooldown(t *testing.T) {
	setGinTestMode()
	prevTimeout := openAITTFTWatchdogTimeout
	prevCooldown := openAITTFTCooldownDuration
	openAITTFTWatchdogTimeout = 20 * time.Millisecond
	openAITTFTCooldownDuration = time.Minute
	defer func() {
		openAITTFTWatchdogTimeout = prevTimeout
		openAITTFTCooldownDuration = prevCooldown
	}()

	repo := &ttftCooldownAccountRepoStub{}
	svc := &OpenAIGatewayService{
		cfg:         &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}},
		accountRepo: repo,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{"X-Request-Id": []string{"rid-ttft-watchdog"}},
	}
	go func() {
		time.Sleep(60 * time.Millisecond)
		_ = pw.Close()
	}()

	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Name: "oauth-42"}
	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "gpt-5.5", "gpt-5.5")
	require.Error(t, err)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusGatewayTimeout, failoverErr.StatusCode)
	require.False(t, c.Writer.Written())
	require.Len(t, repo.calls, 1)
	require.Equal(t, int64(42), repo.calls[0].id)
	require.Contains(t, repo.calls[0].reason, "ttft_timeout")
	require.WithinDuration(t, time.Now().Add(time.Minute), repo.calls[0].until, 2*time.Second)
}

func TestOpenAIStreamingPassthroughTTFTWatchdogReturnsFailoverAndCooldown(t *testing.T) {
	setGinTestMode()
	prevTimeout := openAITTFTWatchdogTimeout
	prevCooldown := openAITTFTCooldownDuration
	openAITTFTWatchdogTimeout = 20 * time.Millisecond
	openAITTFTCooldownDuration = time.Minute
	defer func() {
		openAITTFTWatchdogTimeout = prevTimeout
		openAITTFTCooldownDuration = prevCooldown
	}()

	repo := &ttftCooldownAccountRepoStub{}
	svc := &OpenAIGatewayService{
		cfg:         &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}},
		accountRepo: repo,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{"X-Request-Id": []string{"rid-ttft-watchdog-pass"}},
	}
	go func() {
		time.Sleep(60 * time.Millisecond)
		_ = pw.Close()
	}()

	account := &Account{ID: 84, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Name: "oauth-84"}
	_, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, time.Now(), "gpt-5.5", "gpt-5.5")
	require.Error(t, err)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusGatewayTimeout, failoverErr.StatusCode)
	require.False(t, c.Writer.Written())
	require.Len(t, repo.calls, 1)
	require.Equal(t, int64(84), repo.calls[0].id)
	require.Contains(t, repo.calls[0].reason, "ttft_timeout")
	require.WithinDuration(t, time.Now().Add(time.Minute), repo.calls[0].until, 2*time.Second)
}

func TestOpenAITTFTWatchdogDisabledForResponsesImageOnlyRequest(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Name: "oauth-42"}
	body := []byte(`{"model":"gpt-image-2","size":"1024x1024"}`)

	_, _, err := finalizeOpenAIResponsesOAuthUpstreamBody(c, account, "gpt-image-2", body)
	require.NoError(t, err)

	svc := &OpenAIGatewayService{}
	require.False(t, svc.shouldEnableOpenAITTFTWatchdog(c, account))
}

func TestOpenAITTFTWatchdogDisabledForResponsesExplicitImageToolChoice(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	account := &Account{ID: 84, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Name: "oauth-84"}
	body := []byte(`{"model":"gpt-5.4","tool_choice":{"type":"image_generation"},"tools":[{"type":"image_generation","output_format":"png"}],"input":"draw a cat"}`)

	_, _, err := finalizeOpenAIResponsesOAuthUpstreamBody(c, account, "gpt-5.4", body)
	require.NoError(t, err)

	svc := &OpenAIGatewayService{}
	require.False(t, svc.shouldEnableOpenAITTFTWatchdog(c, account))
}

func TestOpenAINonStreamingSoftRateLimitAdvisoryDoesNotFailover(t *testing.T) {
	setGinTestMode()
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_soft_limit","output":[{"type":"message","content":[{"type":"output_text","text":"Approaching rate limits\nSwitch to o4-mini for lower credit usage?\n1. Switch to o4-mini\n2. Keep current model\n3. Keep current model (never show again)"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"rid-soft-limit-json"}},
	}

	result, err := svc.handleNonStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, "model", "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 1, result.usage.InputTokens)
	require.Equal(t, 1, result.usage.OutputTokens)
	require.Contains(t, rec.Body.String(), "Approaching rate limits")
}

func TestOpenAINonStreamingPassthroughSoftRateLimitAdvisoryDoesNotFailover(t *testing.T) {
	setGinTestMode()
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_soft_limit_pt","output":[{"type":"message","content":[{"type":"output_text","text":"Approaching rate limits\nSwitch to o4-mini for lower credit usage?\n1. Switch to o4-mini\n2. Keep current model\n3. Keep current model (never show again)"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"rid-soft-limit-pt"}},
	}

	result, err := svc.handleNonStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1, Platform: PlatformOpenAI, Name: "acc"}, "model", "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 1, result.usage.InputTokens)
	require.Equal(t, 1, result.usage.OutputTokens)
	require.Contains(t, rec.Body.String(), "Approaching rate limits")
}

func TestExtractOpenAIWSSoftRateLimitAdvisoryFromSSEBody(t *testing.T) {
	body := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_1"}}`,
		"",
		"event: codex.rate_limits",
		`data: {"type":"codex.rate_limits","metered_limit_name":"codex","rate_limits":{"allowed":true,"limit_reached":false,"primary":{"used_percent":95}},"credits":{"has_credits":false,"unlimited":false}}`,
		"",
	}, "\n")

	msg, matched := extractOpenAIWSSoftRateLimitAdvisoryFromSSEBody(body)
	require.True(t, matched)
	require.Contains(t, msg, "Approaching upstream rate limits")

	body = strings.Join([]string{
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_2","output":[{"type":"message","content":[{"type":"output_text","text":"Approaching rate limits\nSwitch to o4-mini for lower credit usage?\n1. Switch to o4-mini\n2. Keep current model\n3. Keep current model (never show again)"}]}]}}`,
		"",
	}, "\n")

	msg, matched = extractOpenAIWSSoftRateLimitAdvisoryFromSSEBody(body)
	require.False(t, matched)
	require.Empty(t, msg)
}

func TestOpenAIStreamingPassthroughResponseDoneWithoutDoneMarkerStillSucceeds(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{},
	}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"response.done\",\"response\":{\"usage\":{\"input_tokens\":2,\"output_tokens\":3,\"input_tokens_details\":{\"cached_tokens\":1}}}}\n\n"))
	}()

	result, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1}, time.Now(), "", "")
	_ = pr.Close()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 2, result.usage.InputTokens)
	require.Equal(t, 3, result.usage.OutputTokens)
	require.Equal(t, 1, result.usage.CacheReadInputTokens)
	require.Contains(t, rec.Body.String(), "response.completed")
	require.NotContains(t, rec.Body.String(), "response.done")
}

func TestOpenAIStreamingPassthroughCountsImageOutputs(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"ig_passthrough_1\",\"type\":\"image_generation_call\",\"result\":\"final-image\"}}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_passthrough_image\",\"output\":[{\"id\":\"ig_passthrough_1\",\"type\":\"image_generation_call\",\"result\":\"final-image\"}],\"usage\":{\"input_tokens\":2,\"output_tokens\":3,\"output_tokens_details\":{\"image_tokens\":1}}}}\n\n",
		)),
		Header: http.Header{},
	}

	result, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1}, time.Now(), "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 1, result.imageCount)
	require.Equal(t, 1, result.usage.ImageOutputTokens)
}

func TestOpenAIStreamingPassthroughResponseIncompleteWithoutDoneMarkerStillSucceeds(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{},
	}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"response.incomplete\",\"response\":{\"usage\":{\"input_tokens\":2,\"output_tokens\":3,\"input_tokens_details\":{\"cached_tokens\":1}}}}\n\n"))
	}()

	result, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1}, time.Now(), "", "")
	_ = pr.Close()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 2, result.usage.InputTokens)
	require.Equal(t, 3, result.usage.OutputTokens)
	require.Equal(t, 1, result.usage.CacheReadInputTokens)
}

func TestOpenAIStreamingPassthroughNormalizesResponseDoneEventForHTTPClients(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			MaxLineSize: defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: response.done",
			`data: {"type":"response.done","response":{"id":"resp_done","usage":{"input_tokens":2,"output_tokens":3,"input_tokens_details":{"cached_tokens":1}}}}`,
			"",
		}, "\n"))),
		Header: http.Header{},
	}

	result, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, &Account{ID: 1}, time.Now(), "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 2, result.usage.InputTokens)
	require.Contains(t, rec.Body.String(), "event: response.completed")
	require.Contains(t, rec.Body.String(), `"type":"response.completed"`)
	require.NotContains(t, rec.Body.String(), "response.done")
}

func TestOpenAIStreamingTooLong(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               64 * 1024,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{},
	}

	go func() {
		defer func() { _ = pw.Close() }()
		// 写入超过 MaxLineSize 的单行数据，触发 ErrTooLong
		payload := "data: " + strings.Repeat("a", 128*1024) + "\n"
		_, _ = pw.Write([]byte(payload))
	}()

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 2}, time.Now(), "model", "model")
	_ = pr.Close()

	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("expected ErrTooLong, got %v", err)
	}
	if !strings.Contains(rec.Body.String(), "\"type\":\"error\"") || !strings.Contains(rec.Body.String(), "response_too_large") {
		t.Fatalf("expected OpenAI-compatible error SSE event, got %q", rec.Body.String())
	}
}

func TestOpenAINonStreamingContentTypePassThrough(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Security: config.SecurityConfig{
			ResponseHeaders: config.ResponseHeaderConfig{Enabled: false},
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	body := []byte(`{"usage":{"input_tokens":1,"output_tokens":2,"input_tokens_details":{"cached_tokens":0}}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/vnd.test+json"}},
	}

	_, err := svc.handleNonStreamingResponse(c.Request.Context(), resp, c, &Account{}, "model", "model")
	if err != nil {
		t.Fatalf("handleNonStreamingResponse error: %v", err)
	}

	if !strings.Contains(rec.Header().Get("Content-Type"), "application/vnd.test+json") {
		t.Fatalf("expected Content-Type passthrough, got %q", rec.Header().Get("Content-Type"))
	}
}

func TestOpenAINonStreamingContentTypeDefault(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Security: config.SecurityConfig{
			ResponseHeaders: config.ResponseHeaderConfig{Enabled: false},
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	body := []byte(`{"usage":{"input_tokens":1,"output_tokens":2,"input_tokens_details":{"cached_tokens":0}}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     http.Header{},
	}

	_, err := svc.handleNonStreamingResponse(c.Request.Context(), resp, c, &Account{}, "model", "model")
	if err != nil {
		t.Fatalf("handleNonStreamingResponse error: %v", err)
	}

	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("expected default Content-Type, got %q", rec.Header().Get("Content-Type"))
	}
}

func TestOpenAIStreamingHeadersOverride(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Security: config.SecurityConfig{
			ResponseHeaders: config.ResponseHeaderConfig{Enabled: false},
		},
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header: http.Header{
			"Cache-Control": []string{"upstream"},
			"X-Request-Id":  []string{"req-123"},
			"Content-Type":  []string{"application/custom"},
		},
	}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{}}\n\n"))
	}()

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1}, time.Now(), "model", "model")
	_ = pr.Close()
	if err != nil {
		t.Fatalf("handleStreamingResponse error: %v", err)
	}

	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("expected Cache-Control override, got %q", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected Content-Type override, got %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Request-Id") != "req-123" {
		t.Fatalf("expected X-Request-Id passthrough, got %q", rec.Header().Get("X-Request-Id"))
	}
}

func TestOpenAIStreamingReuseScannerBufferAndStillWorks(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			StreamDataIntervalTimeout: 0,
			StreamKeepaliveInterval:   0,
			MaxLineSize:               defaultMaxLineSize,
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
		Header:     http.Header{},
	}

	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":2,\"input_tokens_details\":{\"cached_tokens\":3}}}}\n\n"))
	}()

	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 1}, time.Now(), "model", "model")
	_ = pr.Close()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 1, result.usage.InputTokens)
	require.Equal(t, 2, result.usage.OutputTokens)
	require.Equal(t, 3, result.usage.CacheReadInputTokens)
}

func TestOpenAIInvalidBaseURLWhenAllowlistDisabled(t *testing.T) {
	setGinTestMode()
	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "://invalid-url"},
	}

	_, err := svc.buildUpstreamRequest(c.Request.Context(), c, account, []byte("{}"), "token", false, "", false)
	if err == nil {
		t.Fatalf("expected error for invalid base_url when allowlist disabled")
	}
}

func TestOpenAIValidateUpstreamBaseURLDisabledRequiresHTTPS(t *testing.T) {
	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	if _, err := svc.validateUpstreamBaseURL("http://not-https.example.com"); err == nil {
		t.Fatalf("expected http to be rejected when allow_insecure_http is false")
	}
	normalized, err := svc.validateUpstreamBaseURL("https://example.com")
	if err != nil {
		t.Fatalf("expected https to be allowed when allowlist disabled, got %v", err)
	}
	if normalized != "https://example.com" {
		t.Fatalf("expected raw url passthrough, got %q", normalized)
	}
}

func TestOpenAIValidateUpstreamBaseURLDisabledAllowsHTTP(t *testing.T) {
	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled:           false,
				AllowInsecureHTTP: true,
			},
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	normalized, err := svc.validateUpstreamBaseURL("http://not-https.example.com")
	if err != nil {
		t.Fatalf("expected http allowed when allow_insecure_http is true, got %v", err)
	}
	if normalized != "http://not-https.example.com" {
		t.Fatalf("expected raw url passthrough, got %q", normalized)
	}
}

func TestOpenAIValidateUpstreamBaseURLEnabledEnforcesAllowlist(t *testing.T) {
	cfg := &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled:       true,
				UpstreamHosts: []string{"example.com"},
			},
		},
	}
	svc := &OpenAIGatewayService{cfg: cfg}

	if _, err := svc.validateUpstreamBaseURL("https://example.com"); err != nil {
		t.Fatalf("expected allowlisted host to pass, got %v", err)
	}
	if _, err := svc.validateUpstreamBaseURL("https://evil.com"); err == nil {
		t.Fatalf("expected non-allowlisted host to fail")
	}
}

func TestOpenAIUpdateCodexUsageSnapshotFromHeaders(t *testing.T) {
	repo := &snapshotUpdateAccountRepo{updateExtraCalls: make(chan map[string]any, 1)}
	svc := &OpenAIGatewayService{accountRepo: repo}
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "12")
	headers.Set("x-codex-secondary-used-percent", "34")
	headers.Set("x-codex-primary-window-minutes", "300")
	headers.Set("x-codex-secondary-window-minutes", "10080")
	headers.Set("x-codex-primary-reset-after-seconds", "600")
	headers.Set("x-codex-secondary-reset-after-seconds", "86400")

	svc.UpdateCodexUsageSnapshotFromHeaders(context.Background(), 123, headers)

	select {
	case updates := <-repo.updateExtraCalls:
		require.Equal(t, 12.0, updates["codex_5h_used_percent"])
		require.Equal(t, 34.0, updates["codex_7d_used_percent"])
		require.Equal(t, 600, updates["codex_5h_reset_after_seconds"])
		require.Equal(t, 86400, updates["codex_7d_reset_after_seconds"])
	case <-time.After(2 * time.Second):
		t.Fatal("expected UpdateExtra to be called")
	}
}

func TestOpenAIResponsesRequestPathSuffix(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "exact v1 responses", path: "/v1/responses", want: ""},
		{name: "compact v1 responses", path: "/v1/responses/compact", want: "/compact"},
		{name: "compact alias responses", path: "/responses/compact/", want: "/compact"},
		{name: "nested suffix", path: "/openai/v1/responses/compact/detail", want: "/compact/detail"},
		{name: "unrelated path", path: "/v1/chat/completions", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, nil)
			require.Equal(t, tt.want, openAIResponsesRequestPathSuffix(c))
		})
	}
}

func TestOpenAIGatewayService_Forward_BlocksResponsesFastPolicyBeforeUpstream(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.5","stream":false,"service_tier":"priority","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	settings := &OpenAIFastPolicySettings{
		Rules: []OpenAIFastPolicyRule{{
			ServiceTier:    OpenAIFastTierPriority,
			Action:         BetaPolicyActionBlock,
			Scope:          BetaPolicyScopeAll,
			ErrorMessage:   "fast mode blocked for responses",
			ModelWhitelist: []string{"gpt-5.5"},
			FallbackAction: BetaPolicyActionPass,
		}},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_unused"}`)),
	}}
	svc := newOpenAIGatewayServiceWithSettings(t, settings)
	svc.cfg = &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled: false,
			},
		},
	}
	svc.httpUpstream = upstream
	account := &Account{
		ID:          1,
		Name:        "openai-responses-fast-block",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://upstream.example",
		},
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq, "blocked requests must not reach upstream")
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"permission_error"`)
	require.Contains(t, rec.Body.String(), "fast mode blocked for responses")
}

func TestNormalizeOpenAICompactRequestBodyPreservesCurrentCodexPayloadFields(t *testing.T) {
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"message","role":"user","content":"compact me"}],"instructions":"compact-test","tools":[{"type":"function","name":"shell"}],"parallel_tool_calls":true,"reasoning":{"effort":"high"},"text":{"verbosity":"low"},"previous_response_id":"resp_123","store":true,"stream":true,"prompt_cache_key":"cache_123"}`)

	normalized, changed, err := normalizeOpenAICompactRequestBody(body)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "gpt-5.5", gjson.GetBytes(normalized, "model").String())
	require.True(t, gjson.GetBytes(normalized, "tools").Exists())
	require.True(t, gjson.GetBytes(normalized, "parallel_tool_calls").Bool())
	require.Equal(t, "high", gjson.GetBytes(normalized, "reasoning.effort").String())
	require.Equal(t, "low", gjson.GetBytes(normalized, "text.verbosity").String())
	require.Equal(t, "resp_123", gjson.GetBytes(normalized, "previous_response_id").String())
	require.False(t, gjson.GetBytes(normalized, "store").Exists())
	require.False(t, gjson.GetBytes(normalized, "stream").Exists())
	require.False(t, gjson.GetBytes(normalized, "prompt_cache_key").Exists())
}

func TestOpenAIBuildUpstreamRequestOpenAIPassthroughPreservesCompactPath(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader([]byte(`{"model":"gpt-5"}`)))
	c.Request.Header.Set("Accept", "*/*")
	c.Request.Header.Set("Session_Id", "compact-cache")
	c.Request.Header.Set("X-Codex-Installation-Id", "inst-123")
	c.Request.Header.Set("X-Codex-Window-Id", "compact-cache:0")

	svc := &OpenAIGatewayService{}
	account := &Account{Type: AccountTypeOAuth}

	req, err := svc.buildUpstreamRequestOpenAIPassthrough(c.Request.Context(), c, account, []byte(`{"model":"gpt-5"}`), "token", "compact-cache")
	require.NoError(t, err)
	require.Equal(t, chatgptCodexURL+"/compact", req.URL.String())
	require.Equal(t, "*/*", req.Header.Get("Accept"))
	require.Empty(t, req.Header.Get("Version"))
	require.Empty(t, req.Header.Get("OpenAI-Beta"))
	require.Equal(t, "compact-cache", req.Header.Get("Session_Id"))
	require.Empty(t, req.Header.Get("Conversation_Id"))
	require.Equal(t, "inst-123", req.Header.Get("X-Codex-Installation-Id"))
	require.Equal(t, "compact-cache:0", req.Header.Get("X-Codex-Window-Id"))
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(req.Context()))
}

func TestOpenAIBuildUpstreamRequestCompactPreservesOfficialOAuthHeaders(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader([]byte(`{"model":"gpt-5"}`)))
	c.Request.Header.Set("Accept", "*/*")
	c.Request.Header.Set("Session_Id", "sess-123")
	c.Request.Header.Set("X-Codex-Installation-Id", "inst-123")
	c.Request.Header.Set("X-Codex-Window-Id", "sess-123:0")
	c.Request.Header.Set("Originator", "codex_exec")

	svc := &OpenAIGatewayService{}
	account := &Account{
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"chatgpt_account_id": "chatgpt-acc"},
	}

	req, err := svc.buildUpstreamRequest(c.Request.Context(), c, account, []byte(`{"model":"gpt-5"}`), "token", false, "", true)
	require.NoError(t, err)
	require.Equal(t, chatgptCodexURL+"/compact", req.URL.String())
	require.Equal(t, "*/*", req.Header.Get("Accept"))
	require.Empty(t, req.Header.Get("Version"))
	require.Empty(t, req.Header.Get("OpenAI-Beta"))
	require.Equal(t, "sess-123", req.Header.Get("Session_Id"))
	require.Empty(t, req.Header.Get("Conversation_Id"))
	require.Equal(t, "inst-123", req.Header.Get("X-Codex-Installation-Id"))
	require.Equal(t, "sess-123:0", req.Header.Get("X-Codex-Window-Id"))
	require.Equal(t, "codex_exec", req.Header.Get("Originator"))
}

func TestOpenAIBuildUpstreamRequestResponsesPreservesOfficialOAuthHeaders(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"gpt-5"}`)))
	c.Request.Header.Set("Accept", "text/event-stream")
	c.Request.Header.Set("Session_Id", "sess-123")
	c.Request.Header.Set("Conversation_Id", "conv-should-not-forward")
	c.Request.Header.Set("X-Codex-Beta-Features", "memories,prevent_idle_sleep")
	c.Request.Header.Set("X-Client-Request-Id", "req-123")
	c.Request.Header.Set("X-Codex-Window-Id", "sess-123:0")
	c.Request.Header.Set("Originator", "codex_exec")
	c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")

	svc := &OpenAIGatewayService{}
	account := &Account{
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"chatgpt_account_id": "chatgpt-acc"},
	}

	req, err := svc.buildUpstreamRequest(c.Request.Context(), c, account, []byte(`{"model":"gpt-5"}`), "token", true, "pcache-123", true)
	require.NoError(t, err)
	require.Equal(t, chatgptCodexURL, req.URL.String())
	require.Equal(t, "text/event-stream", req.Header.Get("Accept"))
	require.Empty(t, req.Header.Get("OpenAI-Beta"))
	require.Equal(t, "sess-123", req.Header.Get("Session_Id"))
	require.Empty(t, req.Header.Get("Conversation_Id"))
	require.Equal(t, "memories,prevent_idle_sleep", req.Header.Get("X-Codex-Beta-Features"))
	require.Equal(t, "req-123", req.Header.Get("X-Client-Request-Id"))
	require.Equal(t, "sess-123:0", req.Header.Get("X-Codex-Window-Id"))
	require.Equal(t, "codex_exec", req.Header.Get("Originator"))
	require.Equal(t, "codex_exec/0.124.0", req.Header.Get("User-Agent"))
}

func TestOpenAIBuildUpstreamRequestOpenAIPassthroughResponsesPreservesOfficialOAuthHeaders(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"gpt-5"}`)))
	c.Request.Header.Set("Accept", "text/event-stream")
	c.Request.Header.Set("Session_Id", "sess-123")
	c.Request.Header.Set("Conversation_Id", "conv-should-not-forward")
	c.Request.Header.Set("X-Codex-Beta-Features", "memories,prevent_idle_sleep")
	c.Request.Header.Set("X-Client-Request-Id", "req-123")
	c.Request.Header.Set("X-Codex-Window-Id", "sess-123:0")
	c.Request.Header.Set("Originator", "codex_exec")
	c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")

	svc := &OpenAIGatewayService{}
	account := &Account{Type: AccountTypeOAuth}

	req, err := svc.buildUpstreamRequestOpenAIPassthrough(c.Request.Context(), c, account, []byte(`{"model":"gpt-5"}`), "token", "pcache-123")
	require.NoError(t, err)
	require.Equal(t, chatgptCodexURL, req.URL.String())
	require.Equal(t, "text/event-stream", req.Header.Get("Accept"))
	require.Empty(t, req.Header.Get("OpenAI-Beta"))
	require.Equal(t, "sess-123", req.Header.Get("Session_Id"))
	require.Empty(t, req.Header.Get("Conversation_Id"))
	require.Equal(t, "memories,prevent_idle_sleep", req.Header.Get("X-Codex-Beta-Features"))
	require.Equal(t, "req-123", req.Header.Get("X-Client-Request-Id"))
	require.Equal(t, "sess-123:0", req.Header.Get("X-Codex-Window-Id"))
	require.Equal(t, "codex_exec", req.Header.Get("Originator"))
	require.Equal(t, "codex_exec/0.124.0", req.Header.Get("User-Agent"))
}

func TestOpenAIBuildUpstreamRequest_ForceCodexCLIOnlyOverridesUserAgent(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"gpt-5"}`)))
	c.Request.Header.Set("Session_Id", "sess-nonofficial")
	c.Request.Header.Set("User-Agent", "custom-client/1.0")
	c.Set("api_key", &APIKey{ID: 42})

	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{ForceCodexCLI: true},
	}}
	account := &Account{
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"chatgpt_account_id": "chatgpt-acc"},
	}

	req, err := svc.buildUpstreamRequest(c.Request.Context(), c, account, []byte(`{"model":"gpt-5"}`), "token", true, "", false)
	require.NoError(t, err)
	require.Equal(t, codexCLIUserAgent, req.Header.Get("User-Agent"))
	require.Equal(t, "opencode", req.Header.Get("Originator"))
	require.Equal(t, "responses=experimental", req.Header.Get("OpenAI-Beta"))
	require.Equal(t, isolateOpenAISessionID(42, "sess-nonofficial"), req.Header.Get("Session_Id"))
	require.Equal(t, isolateOpenAISessionID(42, "sess-nonofficial"), req.Header.Get("Conversation_Id"))
}

func TestOpenAIBuildUpstreamRequestOpenAIPassthrough_NonOfficialOAuthPreservesUserAgentWithoutForceCodexCLI(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"gpt-5"}`)))
	c.Request.Header.Set("Session_Id", "sess-nonofficial")
	c.Request.Header.Set("User-Agent", "custom-client/1.0")

	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{ForceCodexCLI: false},
	}}
	account := &Account{
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"chatgpt_account_id": "chatgpt-acc"},
	}

	req, err := svc.buildUpstreamRequestOpenAIPassthrough(c.Request.Context(), c, account, []byte(`{"model":"gpt-5"}`), "token", "")
	require.NoError(t, err)
	require.Equal(t, "custom-client/1.0", req.Header.Get("User-Agent"))
	require.Equal(t, "opencode", req.Header.Get("Originator"))
	require.Equal(t, "responses=experimental", req.Header.Get("OpenAI-Beta"))
}

func TestOpenAIBuildUpstreamRequest_APIKeySkipsOAuthOnlyCodexHeaders(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"gpt-5"}`)))
	c.Request.Header.Set("X-Codex-Beta-Features", "memories,prevent_idle_sleep")
	c.Request.Header.Set("X-Client-Request-Id", "req-123")
	c.Request.Header.Set("X-Codex-Window-Id", "sess-123:0")
	c.Request.Header.Set("User-Agent", "custom-client/1.0")

	svc := &OpenAIGatewayService{}
	account := &Account{
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
	}

	req, err := svc.buildUpstreamRequest(c.Request.Context(), c, account, []byte(`{"model":"gpt-5"}`), "token", true, "", false)
	require.NoError(t, err)
	require.Empty(t, req.Header.Get("X-Codex-Beta-Features"))
	require.Empty(t, req.Header.Get("X-Client-Request-Id"))
	require.Empty(t, req.Header.Get("X-Codex-Window-Id"))

	passthroughReq, err := svc.buildUpstreamRequestOpenAIPassthrough(c.Request.Context(), c, account, []byte(`{"model":"gpt-5"}`), "token", "")
	require.NoError(t, err)
	require.Empty(t, passthroughReq.Header.Get("X-Codex-Beta-Features"))
	require.Empty(t, passthroughReq.Header.Get("X-Client-Request-Id"))
	require.Empty(t, passthroughReq.Header.Get("X-Codex-Window-Id"))
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(req.Context()))
}

func TestOpenAIBuildUpstreamRequestOAuthMessagesBridgeUsesSessionOnly(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.5","prompt_cache_key":"anthropic-metadata-session-1","input":[{"type":"message","role":"developer","content":[{"type":"input_text","text":"<sub2api-claude-code-todo-guard>"}]},{"type":"message","role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("OpenAI-Beta", "responses=experimental")
	c.Request.Header.Set("originator", "codex_cli_rs")

	svc := &OpenAIGatewayService{}
	account := &Account{
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"chatgpt_account_id": "chatgpt-acc"},
	}

	req, err := svc.buildUpstreamRequest(c.Request.Context(), c, account, body, "token", true, "anthropic-metadata-session-1", false)
	require.NoError(t, err)
	require.NotEmpty(t, req.Header.Get("Session_Id"))
	require.Empty(t, req.Header.Get("Conversation_Id"))
	require.Empty(t, req.Header.Get("OpenAI-Beta"))
	require.Empty(t, req.Header.Get("originator"))
}

func TestOpenAIBuildUpstreamRequestPreservesCompactPathForAPIKeyBaseURL(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/responses/compact", bytes.NewReader([]byte(`{"model":"gpt-5"}`)))

	svc := &OpenAIGatewayService{cfg: &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		},
	}}
	account := &Account{
		Type:        AccountTypeAPIKey,
		Platform:    PlatformOpenAI,
		Credentials: map[string]any{"base_url": "https://example.com/v1"},
	}

	req, err := svc.buildUpstreamRequest(c.Request.Context(), c, account, []byte(`{"model":"gpt-5"}`), "token", false, "", false)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/v1/responses/compact", req.URL.String())
}

func TestOpenAIBuildUpstreamRequestOAuthOfficialClientOriginatorCompatibility(t *testing.T) {
	setGinTestMode()

	tests := []struct {
		name           string
		userAgent      string
		originator     string
		wantOriginator string
	}{
		{name: "desktop originator preserved", originator: "Codex Desktop", wantOriginator: "Codex Desktop"},
		{name: "vscode originator preserved", originator: "codex_vscode", wantOriginator: "codex_vscode"},
		{name: "official ua fallback to codex_cli_rs", userAgent: "Codex Desktop/1.2.3", wantOriginator: "codex_cli_rs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{"model":"gpt-5"}`)))
			if tt.userAgent != "" {
				c.Request.Header.Set("User-Agent", tt.userAgent)
			}
			if tt.originator != "" {
				c.Request.Header.Set("originator", tt.originator)
			}

			svc := &OpenAIGatewayService{}
			account := &Account{
				Type:        AccountTypeOAuth,
				Credentials: map[string]any{"chatgpt_account_id": "chatgpt-acc"},
			}

			isCodexCLI := openai.IsCodexOfficialClientByHeaders(c.GetHeader("User-Agent"), c.GetHeader("originator"))
			req, err := svc.buildUpstreamRequest(c.Request.Context(), c, account, []byte(`{"model":"gpt-5"}`), "token", false, "", isCodexCLI)
			require.NoError(t, err)
			require.Equal(t, tt.wantOriginator, req.Header.Get("originator"))
		})
	}
}

// ==================== P1-08 修复：model 替换性能优化测试 ====================

// ==================== P1-08 修复：model 替换性能优化测试 =============
func TestReplaceModelInSSELine(t *testing.T) {
	svc := &OpenAIGatewayService{}

	tests := []struct {
		name     string
		line     string
		from     string
		to       string
		expected string
	}{
		{
			name:     "顶层 model 字段替换",
			line:     `data: {"id":"chatcmpl-123","model":"gpt-4o","choices":[]}`,
			from:     "gpt-4o",
			to:       "my-custom-model",
			expected: `data: {"id":"chatcmpl-123","model":"my-custom-model","choices":[]}`,
		},
		{
			name:     "嵌套 response.model 替换",
			line:     `data: {"type":"response","response":{"id":"resp-1","model":"gpt-4o","output":[]}}`,
			from:     "gpt-4o",
			to:       "my-model",
			expected: `data: {"type":"response","response":{"id":"resp-1","model":"my-model","output":[]}}`,
		},
		{
			name:     "model 不匹配时不替换",
			line:     `data: {"id":"chatcmpl-123","model":"gpt-3.5-turbo","choices":[]}`,
			from:     "gpt-4o",
			to:       "my-model",
			expected: `data: {"id":"chatcmpl-123","model":"gpt-3.5-turbo","choices":[]}`,
		},
		{
			name:     "无 model 字段时不替换",
			line:     `data: {"id":"chatcmpl-123","choices":[]}`,
			from:     "gpt-4o",
			to:       "my-model",
			expected: `data: {"id":"chatcmpl-123","choices":[]}`,
		},
		{
			name:     "空 data 行",
			line:     `data: `,
			from:     "gpt-4o",
			to:       "my-model",
			expected: `data: `,
		},
		{
			name:     "[DONE] 行",
			line:     `data: [DONE]`,
			from:     "gpt-4o",
			to:       "my-model",
			expected: `data: [DONE]`,
		},
		{
			name:     "非 data: 前缀行",
			line:     `event: message`,
			from:     "gpt-4o",
			to:       "my-model",
			expected: `event: message`,
		},
		{
			name:     "非法 JSON 不替换",
			line:     `data: {invalid json}`,
			from:     "gpt-4o",
			to:       "my-model",
			expected: `data: {invalid json}`,
		},
		{
			name:     "无空格 data: 格式",
			line:     `data:{"id":"x","model":"gpt-4o"}`,
			from:     "gpt-4o",
			to:       "my-model",
			expected: `data: {"id":"x","model":"my-model"}`,
		},
		{
			name:     "model 名含特殊字符",
			line:     `data: {"model":"org/model-v2.1-beta"}`,
			from:     "org/model-v2.1-beta",
			to:       "custom/alias",
			expected: `data: {"model":"custom/alias"}`,
		},
		{
			name:     "空行",
			line:     "",
			from:     "gpt-4o",
			to:       "my-model",
			expected: "",
		},
		{
			name:     "保持其他字段不变",
			line:     `data: {"id":"abc","object":"chat.completion.chunk","model":"gpt-4o","created":1234567890,"choices":[{"index":0,"delta":{"content":"hi"}}]}`,
			from:     "gpt-4o",
			to:       "alias",
			expected: `data: {"id":"abc","object":"chat.completion.chunk","model":"alias","created":1234567890,"choices":[{"index":0,"delta":{"content":"hi"}}]}`,
		},
		{
			name:     "顶层优先于嵌套：同时存在两个 model",
			line:     `data: {"model":"gpt-4o","response":{"model":"gpt-4o"}}`,
			from:     "gpt-4o",
			to:       "replaced",
			expected: `data: {"model":"replaced","response":{"model":"gpt-4o"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.replaceModelInSSELine(tt.line, tt.from, tt.to)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestReplaceModelInSSEBody(t *testing.T) {
	svc := &OpenAIGatewayService{}

	tests := []struct {
		name     string
		body     string
		from     string
		to       string
		expected string
	}{
		{
			name:     "多行 SSE body 替换",
			body:     "data: {\"model\":\"gpt-4o\",\"choices\":[]}\n\ndata: {\"model\":\"gpt-4o\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n",
			from:     "gpt-4o",
			to:       "alias",
			expected: "data: {\"model\":\"alias\",\"choices\":[]}\n\ndata: {\"model\":\"alias\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n",
		},
		{
			name:     "无需替换的 body",
			body:     "data: {\"model\":\"gpt-3.5-turbo\"}\n\ndata: [DONE]\n",
			from:     "gpt-4o",
			to:       "alias",
			expected: "data: {\"model\":\"gpt-3.5-turbo\"}\n\ndata: [DONE]\n",
		},
		{
			name:     "混合 event 和 data 行",
			body:     "event: message\ndata: {\"model\":\"gpt-4o\"}\n\n",
			from:     "gpt-4o",
			to:       "alias",
			expected: "event: message\ndata: {\"model\":\"alias\"}\n\n",
		},
		{
			name:     "空 body",
			body:     "",
			from:     "gpt-4o",
			to:       "alias",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.replaceModelInSSEBody(tt.body, tt.from, tt.to)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestReplaceModelInResponseBody(t *testing.T) {
	svc := &OpenAIGatewayService{}

	tests := []struct {
		name     string
		body     string
		from     string
		to       string
		expected string
	}{
		{
			name:     "替换顶层 model",
			body:     `{"id":"chatcmpl-123","model":"gpt-4o","choices":[]}`,
			from:     "gpt-4o",
			to:       "alias",
			expected: `{"id":"chatcmpl-123","model":"alias","choices":[]}`,
		},
		{
			name:     "model 不匹配不替换",
			body:     `{"id":"chatcmpl-123","model":"gpt-3.5-turbo","choices":[]}`,
			from:     "gpt-4o",
			to:       "alias",
			expected: `{"id":"chatcmpl-123","model":"gpt-3.5-turbo","choices":[]}`,
		},
		{
			name:     "无 model 字段不替换",
			body:     `{"id":"chatcmpl-123","choices":[]}`,
			from:     "gpt-4o",
			to:       "alias",
			expected: `{"id":"chatcmpl-123","choices":[]}`,
		},
		{
			name:     "非法 JSON 返回原值",
			body:     `not json`,
			from:     "gpt-4o",
			to:       "alias",
			expected: `not json`,
		},
		{
			name:     "空 body 返回原值",
			body:     ``,
			from:     "gpt-4o",
			to:       "alias",
			expected: ``,
		},
		{
			name:     "保持嵌套结构不变",
			body:     `{"model":"gpt-4o","usage":{"prompt_tokens":10,"completion_tokens":20},"choices":[{"message":{"role":"assistant","content":"hello"}}]}`,
			from:     "gpt-4o",
			to:       "alias",
			expected: `{"model":"alias","usage":{"prompt_tokens":10,"completion_tokens":20},"choices":[{"message":{"role":"assistant","content":"hello"}}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.replaceModelInResponseBody([]byte(tt.body), tt.from, tt.to)
			require.Equal(t, tt.expected, string(got))
		})
	}
}

func TestExtractOpenAISSEDataLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantData string
		wantOK   bool
	}{
		{name: "标准格式", line: `data: {"type":"x"}`, wantData: `{"type":"x"}`, wantOK: true},
		{name: "无空格格式", line: `data:{"type":"x"}`, wantData: `{"type":"x"}`, wantOK: true},
		{name: "纯空数据", line: `data:   `, wantData: ``, wantOK: true},
		{name: "非 data 行", line: `event: message`, wantData: ``, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractOpenAISSEDataLine(tt.line)
			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.wantData, got)
		})
	}
}

func TestParseSSEUsage_SelectiveParsing(t *testing.T) {
	svc := &OpenAIGatewayService{}
	usage := &OpenAIUsage{InputTokens: 9, OutputTokens: 8, CacheReadInputTokens: 7}

	// 非 completed 事件，不应覆盖 usage
	svc.parseSSEUsage(`{"type":"response.in_progress","response":{"usage":{"input_tokens":1,"output_tokens":2}}}`, usage)
	require.Equal(t, 9, usage.InputTokens)
	require.Equal(t, 8, usage.OutputTokens)
	require.Equal(t, 7, usage.CacheReadInputTokens)

	// completed 事件，应提取 usage
	svc.parseSSEUsage(`{"type":"response.completed","response":{"usage":{"input_tokens":3,"output_tokens":5,"input_tokens_details":{"cached_tokens":2}}}}`, usage)
	require.Equal(t, 3, usage.InputTokens)
	require.Equal(t, 5, usage.OutputTokens)
	require.Equal(t, 2, usage.CacheReadInputTokens)

	// done 事件同样可能携带最终 usage
	svc.parseSSEUsage(`{"type":"response.done","response":{"usage":{"input_tokens":13,"output_tokens":15,"input_tokens_details":{"cached_tokens":4}}}}`, usage)
	require.Equal(t, 13, usage.InputTokens)
	require.Equal(t, 15, usage.OutputTokens)
	require.Equal(t, 4, usage.CacheReadInputTokens)

	// 后续终止事件若不带 usage，不应覆盖之前已恢复出的 usage
	svc.parseSSEUsage(`{"type":"response.canceled"}`, usage)
	require.Equal(t, 13, usage.InputTokens)
	require.Equal(t, 15, usage.OutputTokens)
	require.Equal(t, 4, usage.CacheReadInputTokens)
}

func TestExtractCodexFinalResponse_SampleReplay(t *testing.T) {
	body := strings.Join([]string{
		`event: message`,
		`data: {"type":"response.in_progress","response":{"id":"resp_1"}}`,
		`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-4o","usage":{"input_tokens":11,"output_tokens":22,"input_tokens_details":{"cached_tokens":3}}}}`,
		`data: [DONE]`,
	}, "\n")

	finalResp, ok := extractCodexFinalResponse(body)
	require.True(t, ok)
	require.Contains(t, string(finalResp), `"id":"resp_1"`)
	require.Contains(t, string(finalResp), `"input_tokens":11`)
}

func TestExtractCodexFinalResponse_PrefersRicherEnvelopeOverLaterUsageOnlyTerminal(t *testing.T) {
	body := strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_rich","model":"gpt-4o","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}}`,
		`data: {"type":"response.done","response":{"id":"resp_rich","usage":{"input_tokens":4,"output_tokens":6,"input_tokens_details":{"cached_tokens":2}}}}`,
		`data: [DONE]`,
	}, "\n")

	finalResp, ok := extractCodexFinalResponse(body)
	require.True(t, ok)
	require.Contains(t, string(finalResp), `"id":"resp_rich"`)
	require.Contains(t, string(finalResp), `"model":"gpt-4o"`)
	require.Contains(t, string(finalResp), `"text":"hello"`)
}

func TestHandleSSEToJSON_TreatsTerminalResponseEnvelopesAsFinalJSON(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		respID    string
	}{
		{name: "incomplete", eventType: "response.incomplete", respID: "resp_incomplete"},
		{name: "cancelled", eventType: "response.cancelled", respID: "resp_cancelled"},
		{name: "canceled", eventType: "response.canceled", respID: "resp_canceled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setGinTestMode()
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			account := &Account{ID: 11, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

			svc := &OpenAIGatewayService{cfg: &config.Config{}}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			}
			body := []byte(strings.Join([]string{
				`data: {"type":"response.in_progress","response":{"id":"` + tt.respID + `"}}`,
				`data: {"type":"` + tt.eventType + `","response":{"id":"` + tt.respID + `","model":"gpt-4o","output":[{"type":"message","content":[{"type":"output_text","text":"terminal"}]}],"usage":{"input_tokens":7,"output_tokens":9,"input_tokens_details":{"cached_tokens":1}}}}`,
				`data: [DONE]`,
			}, "\n"))

			result, err := svc.handleSSEToJSON(resp, c, account, body, "gpt-4o", "gpt-4o")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.usage)
			require.Equal(t, 7, result.usage.InputTokens)
			require.Equal(t, 9, result.usage.OutputTokens)
			require.Equal(t, 1, result.usage.CacheReadInputTokens)
			require.Contains(t, rec.Body.String(), `"id":"`+tt.respID+`"`)
			require.NotContains(t, rec.Body.String(), "data:")
		})
	}
}

func TestHandleSSEToJSON_CompletedEventReturnsJSON(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	body := []byte(strings.Join([]string{
		`data: {"type":"response.in_progress","response":{"id":"resp_2"}}`,
		`data: {"type":"response.completed","response":{"id":"resp_2","model":"gpt-4o","usage":{"input_tokens":7,"output_tokens":9,"input_tokens_details":{"cached_tokens":1}}}}`,
		`data: [DONE]`,
	}, "\n"))

	result, err := svc.handleSSEToJSON(resp, c, account, body, "gpt-4o", "gpt-4o")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 7, result.usage.InputTokens)
	require.Equal(t, 9, result.usage.OutputTokens)
	require.Equal(t, 1, result.usage.CacheReadInputTokens)
	// Header 可能由上游 Content-Type 透传；关键是 body 已转换为最终 JSON 响应。
	require.NotContains(t, rec.Body.String(), "event:")
	require.Contains(t, rec.Body.String(), `"id":"resp_2"`)
	require.NotContains(t, rec.Body.String(), "data:")
}

func TestExtractOpenAIUsageFromJSONBytes_RequiresUsageObject(t *testing.T) {
	usage, ok := extractOpenAIUsageFromJSONBytes([]byte(`{"id":"resp_without_usage","output":[]}`))
	require.False(t, ok)
	require.Equal(t, OpenAIUsage{}, usage)

	usage, ok = extractOpenAIUsageFromJSONBytes([]byte(`{"usage":{"input_tokens":0,"output_tokens":0}}`))
	require.True(t, ok)
	require.Equal(t, 0, usage.InputTokens)
	require.Equal(t, 0, usage.OutputTokens)

	usage, ok = extractOpenAIUsageFromJSONBytes([]byte(`{"input_tokens":3,"output_tokens":4,"output_tokens_details":{"image_tokens":2}}`))
	require.True(t, ok)
	require.Equal(t, 3, usage.InputTokens)
	require.Equal(t, 4, usage.OutputTokens)
	require.Equal(t, 2, usage.ImageOutputTokens)
}

func TestHandleNonStreamingResponse_ValidJSONWithoutUsageStillSucceeds(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_without_usage","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}`,
		)),
		Header: http.Header{"Content-Type": []string{"application/json"}},
	}

	result, err := svc.handleNonStreamingResponse(c.Request.Context(), resp, c, &Account{}, "gpt-4o", "gpt-4o")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Zero(t, result.usage.InputTokens)
	require.Contains(t, rec.Body.String(), `"id":"resp_without_usage"`)
}

func TestHandleSSEToJSON_FallsBackToFullSSEUsageWhenFinalResponseLacksUsage(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 5, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	body := []byte(strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_fallback_usage","model":"gpt-4o","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}}`,
		`data: {"type":"response.done","response":{"id":"resp_fallback_usage","usage":{"input_tokens":4,"output_tokens":6,"input_tokens_details":{"cached_tokens":2}}}}`,
		`data: [DONE]`,
	}, "\n"))

	result, err := svc.handleSSEToJSON(resp, c, account, body, "gpt-4o", "gpt-4o")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 4, result.usage.InputTokens)
	require.Equal(t, 6, result.usage.OutputTokens)
	require.Equal(t, 2, result.usage.CacheReadInputTokens)
	require.Contains(t, rec.Body.String(), `"id":"resp_fallback_usage"`)
	require.Contains(t, rec.Body.String(), `"model":"gpt-4o"`)
	require.Contains(t, rec.Body.String(), `"text":"hello"`)
}

func TestHandleSSEToJSON_PreservesRecoveredUsageWhenLaterTerminalEventsOmitUsage(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	body := []byte(strings.Join([]string{
		`data: {"type":"response.done","response":{"id":"resp_preserve_usage","usage":{"input_tokens":4,"output_tokens":6,"input_tokens_details":{"cached_tokens":2}}}}`,
		`data: {"type":"response.incomplete","response":{"id":"resp_preserve_usage","model":"gpt-4o","output":[{"type":"message","content":[{"type":"output_text","text":"partial"}]}]}}`,
		`data: {"type":"response.canceled"}`,
		`data: [DONE]`,
	}, "\n"))

	result, err := svc.handleSSEToJSON(resp, c, account, body, "gpt-4o", "gpt-4o")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 4, result.usage.InputTokens)
	require.Equal(t, 6, result.usage.OutputTokens)
	require.Equal(t, 2, result.usage.CacheReadInputTokens)
	require.Contains(t, rec.Body.String(), `"id":"resp_preserve_usage"`)
	require.Contains(t, rec.Body.String(), `"text":"partial"`)
	require.NotContains(t, rec.Body.String(), "data:")
}

func TestHandlePassthroughSSEToJSON_FallsBackToFullSSEUsageWhenFinalResponseLacksUsage(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 6, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	body := []byte(strings.Join([]string{
		`data: {"type":"response.completed","response":{"id":"resp_passthrough_usage","model":"gpt-4o","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}}`,
		`data: {"type":"response.done","response":{"id":"resp_passthrough_usage","usage":{"input_tokens":5,"output_tokens":7,"input_tokens_details":{"cached_tokens":3}}}}`,
		`data: [DONE]`,
	}, "\n"))

	result, err := svc.handlePassthroughSSEToJSON(resp, c, account, body, "gpt-4o", "gpt-4o")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 5, result.usage.InputTokens)
	require.Equal(t, 7, result.usage.OutputTokens)
	require.Equal(t, 3, result.usage.CacheReadInputTokens)
	require.Contains(t, rec.Body.String(), `"id":"resp_passthrough_usage"`)
	require.Contains(t, rec.Body.String(), `"model":"gpt-4o"`)
	require.Contains(t, rec.Body.String(), `"text":"hello"`)
}

func TestHandleSSEToJSON_ReconstructsImageGenerationOutputItemDone(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	body := []byte(strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"id":"ig_123","type":"image_generation_call","result":"aGVsbG8=","revised_prompt":"draw a cat","output_format":"png"}}`,
		`data: {"type":"response.completed","response":{"id":"resp_img","model":"gpt-5.4","output":[],"usage":{"input_tokens":7,"output_tokens":9,"output_tokens_details":{"image_tokens":4}}}}`,
		`data: [DONE]`,
	}, "\n"))

	result, err := svc.handleSSEToJSON(resp, c, account, body, "gpt-5.4", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 4, result.usage.ImageOutputTokens)
	require.Equal(t, 1, result.imageCount)
	require.NotContains(t, rec.Body.String(), "data:")
	require.Equal(t, "image_generation_call", gjson.Get(rec.Body.String(), "output.0.type").String())
	require.Equal(t, "aGVsbG8=", gjson.Get(rec.Body.String(), "output.0.result").String())
	require.Equal(t, "draw a cat", gjson.Get(rec.Body.String(), "output.0.revised_prompt").String())
}

func TestHandleSSEToJSON_EventNamedCompletedReturnsJSON(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 22, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	body := []byte(strings.Join([]string{
		`event: response.output_text.delta`,
		`data: {"delta":"ok"}`,
		``,
		`event: response.completed`,
		`data: {"response":{"id":"resp_event_json","model":"gpt-5.4","output":[{"type":"message","id":"msg_1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":7,"output_tokens":3,"input_tokens_details":{"cached_tokens":2}}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n"))

	result, err := svc.handleSSEToJSON(resp, c, account, body, "gpt-5.4", "gpt-5.4")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 7, result.usage.InputTokens)
	require.Equal(t, 3, result.usage.OutputTokens)
	require.Equal(t, 2, result.usage.CacheReadInputTokens)
	require.NotContains(t, rec.Body.String(), "data:")
	require.Equal(t, "resp_event_json", gjson.Get(rec.Body.String(), "id").String())
	require.Equal(t, "ok", gjson.Get(rec.Body.String(), "output.0.content.0.text").String())
}

func TestHandleSSEToJSON_NoFinalResponseKeepsSSEBody(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	body := []byte(strings.Join([]string{
		`data: {"type":"response.in_progress","response":{"id":"resp_3"}}`,
		`data: [DONE]`,
	}, "\n"))

	result, err := svc.handleSSEToJSON(resp, c, account, body, "gpt-4o", "gpt-4o")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.usage)
	require.Equal(t, 0, result.usage.InputTokens)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, rec.Body.String(), `data: {"type":"response.in_progress"`)
}

func TestHandleSSEToJSON_ResponseFailedReturnsProtocolError(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	body := []byte(strings.Join([]string{
		`data: {"type":"response.failed","error":{"message":"upstream rejected request"}}`,
		`data: [DONE]`,
	}, "\n"))

	result, err := svc.handleSSEToJSON(resp, c, account, body, "gpt-4o", "gpt-4o")
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "upstream rejected request")
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestHandleSSEToJSON_ResponseFailedServerOverloadedReturnsRetryableFailover(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	account := &Account{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	body := []byte(strings.Join([]string{
		`data: {"type":"response.failed","error":{"type":"service_unavailable_error","code":"server_is_overloaded","message":"Our servers are currently overloaded. Please try again later."}}`,
		`data: [DONE]`,
	}, "\n"))

	result, err := svc.handleSSEToJSON(resp, c, account, body, "gpt-4o", "gpt-4o")
	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusServiceUnavailable, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
}

func TestOpenAICompatSSEFrameParserResetsEventTypeAtFrameBoundary(t *testing.T) {
	var parser openAICompatSSEFrameParser

	frame, ok := parser.AddLine("event: response.created")
	require.False(t, ok)
	require.Empty(t, frame)

	frame, ok = parser.AddLine(`data: {"response":{"id":"resp_1"}}`)
	require.False(t, ok)
	require.Empty(t, frame)

	frame, ok = parser.AddLine("")
	require.True(t, ok)
	require.Equal(t, "response.created", frame.EventType)
	require.JSONEq(t, `{"response":{"id":"resp_1"}}`, frame.Data)

	frame, ok = parser.AddLine(`data: {"delta":"ok"}`)
	require.False(t, ok)
	require.Empty(t, frame.EventType)

	frame, ok = parser.AddLine("")
	require.True(t, ok)
	require.Empty(t, frame.EventType)
	require.JSONEq(t, `{"delta":"ok"}`, frame.Data)
}

func TestOpenAICompatSSEFrameParserHandlesCRLFFrameBoundary(t *testing.T) {
	var parser openAICompatSSEFrameParser

	_, ok := parser.AddLine("event: response.completed\r")
	require.False(t, ok)
	_, ok = parser.AddLine(`data: {"response":{"id":"resp_crlf"}}` + "\r")
	require.False(t, ok)

	frame, ok := parser.AddLine("\r")
	require.True(t, ok)
	require.Equal(t, "response.completed", frame.EventType)
	require.JSONEq(t, `{"response":{"id":"resp_crlf"}}`, frame.Data)
}

func TestOpenAICompatSSEFrameParserJoinsMultilineDataUntilBoundary(t *testing.T) {
	var parser openAICompatSSEFrameParser

	_, ok := parser.AddLine("event: response.completed")
	require.False(t, ok)
	_, ok = parser.AddLine(`data: {`)
	require.False(t, ok)
	_, ok = parser.AddLine(`data: "response":{"id":"resp_multiline"}`)
	require.False(t, ok)
	_, ok = parser.AddLine(`data: }`)
	require.False(t, ok)

	frame, ok := parser.AddLine("")
	require.True(t, ok)
	require.Equal(t, "response.completed", frame.EventType)
	require.JSONEq(t, `{"response":{"id":"resp_multiline"}}`, frame.Data)
}

func TestOpenAICompatSSEFrameParserDispatchesPendingFrameBeforeDoneSentinel(t *testing.T) {
	var parser openAICompatSSEFrameParser

	_, ok := parser.AddLine(`data: {"type":"response.completed","response":{"id":"resp_done"}}`)
	require.False(t, ok)

	frame, ok := parser.AddLine("data: [DONE]")
	require.True(t, ok)
	require.JSONEq(t, `{"type":"response.completed","response":{"id":"resp_done"}}`, frame.Data)

	frame, ok = parser.Finish()
	require.True(t, ok)
	require.Equal(t, "[DONE]", strings.TrimSpace(frame.Data))
}

func TestOpenAICompatSSEFrameParserDispatchesPendingFrameBeforeNextEvent(t *testing.T) {
	var parser openAICompatSSEFrameParser

	_, ok := parser.AddLine("event: response.created")
	require.False(t, ok)
	_, ok = parser.AddLine(`data: {"response":{"id":"resp_created"}}`)
	require.False(t, ok)

	frame, ok := parser.AddLine("event: response.output_text.delta")
	require.True(t, ok)
	require.Equal(t, "response.created", frame.EventType)
	require.JSONEq(t, `{"response":{"id":"resp_created"}}`, frame.Data)

	_, ok = parser.AddLine(`data: {"delta":"ok"}`)
	require.False(t, ok)
	frame, ok = parser.AddLine("")
	require.True(t, ok)
	require.Equal(t, "response.output_text.delta", frame.EventType)
	require.JSONEq(t, `{"delta":"ok"}`, frame.Data)
}
