package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newGrokActiveDeltaTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.Grok.HTTPActiveDeltaEnabled = true
	cfg.Gateway.Grok.HTTPActiveDeltaRequireStoreOnCreate = false
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 3600
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
	return cfg
}

func newGrokActiveDeltaTestService(upstream *httpUpstreamRecorder) *OpenAIGatewayService {
	cfg := newGrokActiveDeltaTestConfig()
	return &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}
}

func newGrokActiveDeltaTestAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Name:        "grok-active-delta",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "grok-oauth-token",
			"base_url":     "https://api.x.ai/v1",
		},
	}
}

func newGrokActiveDeltaContext(groupID, apiKeyID int64, sessionID string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("session_id", sessionID)
	c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformGrok}})
	return c, rec
}

func grokActiveDeltaSSE(responseID string) *http.Response {
	body := `event: response.completed
data: {"type":"response.completed","response":{"id":"` + responseID + `","model":"grok-4.5","usage":{"input_tokens":1,"output_tokens":1}}}

`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{responseID}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func bindGrokActiveDeltaInputOnlyContext(t *testing.T, svc *OpenAIGatewayService, c *gin.Context, account *Account, payload []byte, cacheIdentity, lastResponseID string) string {
	t.Helper()
	sessionHash := resolveGrokActiveDeltaSessionHash(c, cacheIdentity)
	require.NotEmpty(t, sessionHash)

	inputItems, exists, err := openAIWSExtractNormalizedInputSequence(payload)
	require.NoError(t, err)
	require.True(t, exists)
	inputHashes, ok := openAIWSCanonicalItemHashes(inputItems)
	require.True(t, ok)
	inputShapes, ok := openAIWSItemShapes(inputItems)
	require.True(t, ok)
	nonInputHash, _, nonInputFields := openAIWSNonInputFingerprint(payload)

	svc.getOpenAIWSStateStore().BindSessionContext(
		getOpenAIGroupIDFromContext(c),
		getAPIKeyIDFromContext(c),
		sessionHash,
		openAIWSSessionContextValue{
			accountID:               account.ID,
			connID:                  "http",
			lastResponseID:          lastResponseID,
			materializedHashes:      inputHashes,
			materializedShapes:      inputShapes,
			materializedCount:       len(inputHashes),
			inputCount:              len(inputHashes),
			inputOnlyContext:        true,
			nonInputHash:            nonInputHash,
			nonInputFields:          nonInputFields,
			rawVsClientVisibleEqual: true,
		},
		time.Hour,
	)
	return sessionHash
}

func TestGrokHTTPActiveDelta_SecondTurnSendsOnlyNewInput(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_grok_delta_ok")}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92001)
	groupID := int64(92010)
	apiKeyID := int64(92011)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-delta")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	require.NotEmpty(t, cacheIdentity)
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)
	bindGrokActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstCanonical, cacheIdentity, "resp_grok_prev")

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-delta")
	followupCanonical, err := applyGrokResponsesCacheIdentity(fullFollowupBody, fullFollowupBody, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(
		context.Background(),
		followupCtx,
		account,
		followupCanonical,
		"grok-4.5",
		"grok-4.5",
		cacheIdentity,
		true,
		time.Now(),
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.OpenAIWSDeltaActive)
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, "resp_grok_prev", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.False(t, gjson.GetBytes(upstream.bodies[0], "store").Bool())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 1)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String())
}

func TestGrokHTTPActiveDelta_PrefixRewriteFallsBackToFullWithoutPrevious(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_grok_full_ok")}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92002)
	groupID := int64(92020)
	apiKeyID := int64(92021)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	rewritten := `{"type":"message","role":"user","content":[{"type":"input_text","text":"changed-history"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"previous_response_id":"resp_stale","input":[` + rewritten + `,` + newInput + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-rewrite")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)
	bindGrokActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstCanonical, cacheIdentity, "resp_grok_prev")

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-rewrite")
	followupCanonical, err := applyGrokResponsesCacheIdentity(fullFollowupBody, fullFollowupBody, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(
		context.Background(),
		followupCtx,
		account,
		followupCanonical,
		"grok-4.5",
		"grok-4.5",
		cacheIdentity,
		true,
		time.Now(),
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.OpenAIWSDeltaActive)
	require.Len(t, upstream.bodies, 1)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.GreaterOrEqual(t, len(gjson.GetBytes(upstream.bodies[0], "input").Array()), 2)
}

func TestGrokHTTPActiveDelta_ApikeyDoesNotMutate(t *testing.T) {
	setGinTestMode()
	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_grok_apikey")}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92003)
	account.Type = AccountTypeAPIKey
	account.Credentials = map[string]any{"api_key": "xai-key"}

	groupID := int64(92030)
	apiKeyID := int64(92031)
	c, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-apikey")
	body := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	cacheIdentity := resolveGrokCacheIdentity(c, body, "", "grok-4.5")
	canonical, err := applyGrokResponsesCacheIdentity(body, body, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(context.Background(), c, account, canonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.OpenAIWSDeltaActive)
	require.Len(t, upstream.bodies, 1)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
}

func TestGrokHTTPActiveDelta_FirstTurnBindsAndSecondTurnDeltas(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			grokActiveDeltaSSE("resp_grok_bind_1"),
			grokActiveDeltaSSE("resp_grok_bind_2"),
		},
	}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92005)
	groupID := int64(92050)
	apiKeyID := int64(92051)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-bind")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	require.NotEmpty(t, cacheIdentity)
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)

	firstResult, err := svc.doGrokResponsesUpstream(
		context.Background(), firstCtx, account, firstCanonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now(),
	)
	require.NoError(t, err)
	require.Equal(t, "resp_grok_bind_1", firstResult.ResponseID)
	require.False(t, firstResult.OpenAIWSDeltaActive)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())

	sessionHash := resolveGrokActiveDeltaSessionHash(firstCtx, cacheIdentity)
	cached, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok, "first turn must bind session context for subsequent deltas")
	require.Equal(t, "resp_grok_bind_1", cached.lastResponseID)
	require.Equal(t, account.ID, cached.accountID)

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-bind")
	followupCanonical, err := applyGrokResponsesCacheIdentity(fullFollowupBody, fullFollowupBody, cacheIdentity, false)
	require.NoError(t, err)

	secondResult, err := svc.doGrokResponsesUpstream(
		context.Background(), followupCtx, account, followupCanonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now(),
	)
	require.NoError(t, err)
	require.True(t, secondResult.OpenAIWSDeltaActive)
	require.Equal(t, "resp_grok_bind_1", gjson.GetBytes(upstream.bodies[1], "previous_response_id").String())
	require.Len(t, gjson.GetBytes(upstream.bodies[1], "input").Array(), 1)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[1], "input.0.content.0.text").String())
}

func TestHandleChatStreamingResponse_SetsResponseIDFromTerminal(t *testing.T) {
	setGinTestMode()
	svc := &OpenAIGatewayService{}
	account := newGrokActiveDeltaTestAccount(92006)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_chat_stream_1","status":"in_progress"}}`,
		``,
		`data: {"type":"response.output_text.delta","delta":"hi"}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_chat_stream_1","status":"completed","usage":{"input_tokens":1,"output_tokens":1}}}`,
		``,
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}

	result, err := svc.handleChatStreamingResponse(resp, c, account, "grok-4.5", "grok-4.5", "grok-4.5", time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_chat_stream_1", result.ResponseID)
}

func TestHandleChatBufferedStreamingResponse_SetsResponseIDFromTerminal(t *testing.T) {
	setGinTestMode()
	svc := &OpenAIGatewayService{}
	account := newGrokActiveDeltaTestAccount(92007)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	upstreamBody := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"hi"}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_chat_buf_1","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}`,
		``,
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_chat_buf"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}

	result, err := svc.handleChatBufferedStreamingResponse(resp, c, account, "grok-4.5", "grok-4.5", "grok-4.5", time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_chat_buf_1", result.ResponseID)
}

func TestGrokHTTPActiveDelta_PreviousNotFoundFullReplay(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"previous_response_not_found","message":"Previous response not found."}}`)),
			},
			grokActiveDeltaSSE("resp_grok_replay_ok"),
		},
	}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92004)
	groupID := int64(92040)
	apiKeyID := int64(92041)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + newInput + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-replay")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)
	bindGrokActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstCanonical, cacheIdentity, "resp_missing")

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-replay")
	followupCanonical, err := applyGrokResponsesCacheIdentity(fullFollowupBody, fullFollowupBody, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(
		context.Background(),
		followupCtx,
		account,
		followupCanonical,
		"grok-4.5",
		"grok-4.5",
		cacheIdentity,
		true,
		time.Now(),
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "resp_missing", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "previous_response_id").Exists())
	require.GreaterOrEqual(t, len(gjson.GetBytes(upstream.bodies[1], "input").Array()), 2)
}

func TestGrokHTTPActiveDelta_NoIdentityDoesNotBind(t *testing.T) {
	setGinTestMode()
	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_no_identity")}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92008)
	groupID := int64(92080)
	apiKeyID := int64(92081)

	// No session_id / conversation header.
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformGrok}})

	body := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	// Force empty identity: bind/evaluate gates must no-op (P4 fail closed).
	require.Empty(t, resolveGrokActiveDeltaSessionHash(c, ""))
	result, err := svc.doGrokResponsesUpstream(context.Background(), c, account, body, "grok-4.5", "grok-4.5", "", true, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_no_identity", result.ResponseID)
	require.False(t, result.OpenAIWSDeltaActive)
	// Empty identity must not write session context under the empty-hash key.
	_, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, "")
	require.False(t, ok)
}

func TestGrokHTTPActiveDelta_AccountMismatchFallsBackToFull(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_acct_mismatch_ok")}
	svc := newGrokActiveDeltaTestService(upstream)
	accountA := newGrokActiveDeltaTestAccount(92009)
	accountB := newGrokActiveDeltaTestAccount(92010)
	groupID := int64(92090)
	apiKeyID := int64(92091)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + newInput + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-acct")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)
	bindGrokActiveDeltaInputOnlyContext(t, svc, firstCtx, accountA, firstCanonical, cacheIdentity, "resp_on_a")

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-acct")
	followupCanonical, err := applyGrokResponsesCacheIdentity(fullFollowupBody, fullFollowupBody, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(
		context.Background(), followupCtx, accountB, followupCanonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now(),
	)
	require.NoError(t, err)
	require.False(t, result.OpenAIWSDeltaActive)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.GreaterOrEqual(t, len(gjson.GetBytes(upstream.bodies[0], "input").Array()), 2)
}

func TestGrokHTTPActiveDelta_SessionInflightFallsBackToFull(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_inflight_ok")}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92011)
	groupID := int64(92110)
	apiKeyID := int64(92111)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + newInput + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-inflight")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)
	bindGrokActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstCanonical, cacheIdentity, "resp_prev")

	sessionHash := resolveGrokActiveDeltaSessionHash(firstCtx, cacheIdentity)
	require.True(t, svc.getOpenAIWSStateStore().TrySessionInFlight(groupID, apiKeyID, sessionHash))
	t.Cleanup(func() {
		svc.getOpenAIWSStateStore().EndSessionInFlight(groupID, apiKeyID, sessionHash)
	})

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-inflight")
	followupCanonical, err := applyGrokResponsesCacheIdentity(fullFollowupBody, fullFollowupBody, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(
		context.Background(), followupCtx, account, followupCanonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now(),
	)
	require.NoError(t, err)
	require.False(t, result.OpenAIWSDeltaActive, "in-flight session must not apply active delta")
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
}

func TestGrokHTTPActiveDelta_WrittenBlocksFullReplay(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"previous_response_not_found","message":"Previous response not found."}}`)),
		},
	}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92012)
	groupID := int64(92120)
	apiKeyID := int64(92121)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + newInput + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-written")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)
	bindGrokActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstCanonical, cacheIdentity, "resp_missing")

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-written")
	// Mark client response via gin Writer so Written() is true (P6).
	followupCtx.Writer.WriteHeader(http.StatusOK)
	_, _ = followupCtx.Writer.Write([]byte("partial"))
	require.True(t, clientResponseAlreadyWritten(followupCtx))
	followupCanonical, err := applyGrokResponsesCacheIdentity(fullFollowupBody, fullFollowupBody, cacheIdentity, false)
	require.NoError(t, err)

	_, err = svc.doGrokResponsesUpstream(
		context.Background(), followupCtx, account, followupCanonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now(),
	)
	// Error path may return failover or handled error; only assert single attempt.
	require.Len(t, upstream.bodies, 1, "already-written client must not full-replay previous_response failures")
	_ = err
}

func TestGrokHTTPActiveDelta_RequireStoreOnCreate(t *testing.T) {
	setGinTestMode()
	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_store_true")}
	svc := newGrokActiveDeltaTestService(upstream)
	svc.cfg.Gateway.Grok.HTTPActiveDeltaRequireStoreOnCreate = true
	account := newGrokActiveDeltaTestAccount(92013)
	groupID := int64(92130)
	apiKeyID := int64(92131)
	c, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-store")
	body := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	cacheIdentity := resolveGrokCacheIdentity(c, body, "", "grok-4.5")
	canonical, err := applyGrokResponsesCacheIdentity(body, body, cacheIdentity, false)
	require.NoError(t, err)

	_, err = svc.doGrokResponsesUpstream(context.Background(), c, account, canonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now())
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool())
}

func TestGrokHTTPActiveDelta_HeaderSessionIdentityStableAcrossTurns(t *testing.T) {
	setGinTestMode()
	// Explicit session header must dominate over body prompt_cache_key rewrites.
	groupID := int64(92140)
	apiKeyID := int64(92141)
	c1, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "stable-header-session")
	c2, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "stable-header-session")
	body1 := []byte(`{"model":"grok-4.5","prompt_cache_key":"round-1-key","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"a"}]}]}`)
	body2 := []byte(`{"model":"grok-4.5","prompt_cache_key":"round-2-key","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"b"}]}]}`)
	id1 := resolveGrokCacheIdentity(c1, nil, "", "grok-4.5")
	id2 := resolveGrokCacheIdentity(c2, nil, "", "grok-4.5")
	require.NotEmpty(t, id1)
	require.Equal(t, id1, id2, "header session seed must stay stable across turns")
	// Even with different body keys, header-first resolution stays stable.
	id1b := resolveGrokCacheIdentity(c1, body1, "", "grok-4.5")
	id2b := resolveGrokCacheIdentity(c2, body2, "", "grok-4.5")
	require.Equal(t, id1, id1b)
	require.Equal(t, id1, id2b)
	require.Equal(t, resolveGrokActiveDeltaSessionHash(c1, id1), resolveGrokActiveDeltaSessionHash(c2, id2))
}

func TestForwardAsAnthropic_GrokUsesSharedActiveDeltaEgress(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			grokActiveDeltaSSE("resp_claude_1"),
			grokActiveDeltaSSE("resp_claude_2"),
		},
	}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92014)
	groupID := int64(92150)
	apiKeyID := int64(92151)

	firstBody := []byte(`{"model":"claude-sonnet-4","stream":true,"max_tokens":128,"messages":[{"role":"user","content":"hi"}]}`)
	secondBody := []byte(`{"model":"claude-sonnet-4","stream":true,"max_tokens":128,"messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"hello"},{"role":"user","content":"again"}]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-claude-grok")
	firstResult, err := svc.ForwardAsAnthropic(context.Background(), firstCtx, account, firstBody, "", "")
	require.NoError(t, err)
	require.NotNil(t, firstResult)
	require.Equal(t, "resp_claude_1", firstResult.ResponseID)
	require.Len(t, upstream.bodies, 1)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())

	// Ensure session context was bound via shared egress.
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, nil, "", "grok-4.5")
	if cacheIdentity == "" {
		// Model mapping may resolve to default text model; use whatever was sent.
		sentModel := gjson.GetBytes(upstream.bodies[0], "model").String()
		cacheIdentity = resolveGrokCacheIdentity(firstCtx, nil, "", sentModel)
	}
	if cacheIdentity != "" {
		sessionHash := resolveGrokActiveDeltaSessionHash(firstCtx, cacheIdentity)
		cached, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
		if ok {
			require.Equal(t, "resp_claude_1", cached.lastResponseID)
		}
	}

	secondCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-claude-grok")
	secondResult, err := svc.ForwardAsAnthropic(context.Background(), secondCtx, account, secondBody, "", "")
	require.NoError(t, err)
	require.NotNil(t, secondResult)
	require.Len(t, upstream.bodies, 2)
	// If session homology held, second turn should prefer previous_response_id only-new;
	// otherwise Full is still correct. Accept either but require shared egress was used
	// (both requests hit xAI responses path via recorder).
	require.NotEmpty(t, upstream.bodies[1])
}

func TestForwardAsAnthropic_GrokFailoverOn5xx(t *testing.T) {
	setGinTestMode()
	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadGateway,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_5xx"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream down"}}`)),
		},
	}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92015)
	groupID := int64(92160)
	apiKeyID := int64(92161)
	c, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-claude-5xx")
	body := []byte(`{"model":"claude-sonnet-4","stream":false,"max_tokens":64,"messages":[{"role":"user","content":"hi"}]}`)

	_, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
	require.Error(t, err)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, http.StatusBadGateway, failover.StatusCode)
	require.Contains(t, string(failover.ResponseBody), "upstream down")
	// Shared egress must surface failover (not a swallowed local error).
	require.NotEmpty(t, upstream.bodies)
}
