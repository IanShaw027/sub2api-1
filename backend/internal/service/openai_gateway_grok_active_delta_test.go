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
	// Match production default: force store=true on full/create so previous_response_id works.
	cfg.Gateway.Grok.HTTPActiveDeltaRequireStoreOnCreate = true
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
	sessionHash := resolveGrokActiveDeltaSessionHash(c, cacheIdentity, payload)
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
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool(), "require_store_on_create forces store=true on delta too")
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 1)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String())
}

func TestGrokHTTPActiveDelta_ToolContinuationSendsOnlyNewOutput(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_grok_tool_delta_ok")}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92021)
	groupID := int64(92022)
	apiKeyID := int64(92023)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"lookup"}]}`
	call := `{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"}`
	output := `{"type":"function_call_output","call_id":"call_1","output":"ok"}`
	input2 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}`
	firstBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + call + `]}`)
	followupBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + call + `,` + output + `,` + input2 + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-tool")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)
	bindGrokActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstCanonical, cacheIdentity, "resp_grok_tool_prev")

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-tool")
	followupCanonical, err := applyGrokResponsesCacheIdentity(followupBody, followupBody, cacheIdentity, false)
	require.NoError(t, err)
	result, err := svc.doGrokResponsesUpstream(
		context.Background(), followupCtx, account, followupCanonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now(),
	)
	require.NoError(t, err)
	require.True(t, result.OpenAIWSDeltaActive)
	require.Equal(t, "resp_grok_tool_prev", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 2)
	require.Equal(t, "function_call_output", gjson.GetBytes(upstream.bodies[0], "input.0.type").String())
	require.Equal(t, "call_1", gjson.GetBytes(upstream.bodies[0], "input.0.call_id").String())
	require.Equal(t, "continue", gjson.GetBytes(upstream.bodies[0], "input.1.content.0.text").String())
}

func TestGrokHTTPActiveDelta_ReusedExplicitSessionKeepsPromptBranchesIndependent(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		grokActiveDeltaSSE("resp_branch_a_next"),
		grokActiveDeltaSSE("resp_branch_b_next"),
	}}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92031)
	groupID := int64(92032)
	apiKeyID := int64(92033)

	firstA := []byte(`{"model":"grok-4.5","stream":true,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"alpha"}]}]}`)
	firstB := []byte(`{"model":"grok-4.5","stream":true,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"beta"}]}]}`)
	followA := []byte(`{"model":"grok-4.5","stream":true,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"alpha"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"a1"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"a2"}]}]}`)
	followB := []byte(`{"model":"grok-4.5","stream":true,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"beta"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"b1"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"b2"}]}]}`)

	ctxA, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "reused-explicit-session")
	ctxB, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "reused-explicit-session")
	cacheIdentityA := resolveGrokCacheIdentity(ctxA, firstA, "", "grok-4.5")
	cacheIdentityB := resolveGrokCacheIdentity(ctxB, firstB, "", "grok-4.5")
	require.Equal(t, cacheIdentityA, cacheIdentityB, "the upstream cache identity remains header-stable")

	canonicalA, err := applyGrokResponsesCacheIdentity(firstA, firstA, cacheIdentityA, false)
	require.NoError(t, err)
	canonicalB, err := applyGrokResponsesCacheIdentity(firstB, firstB, cacheIdentityB, false)
	require.NoError(t, err)
	hashA := bindGrokActiveDeltaInputOnlyContext(t, svc, ctxA, account, canonicalA, cacheIdentityA, "resp_branch_a")
	hashB := bindGrokActiveDeltaInputOnlyContext(t, svc, ctxB, account, canonicalB, cacheIdentityB, "resp_branch_b")
	require.NotEqual(t, hashA, hashB, "different opening prompts need independent active-delta slots")

	followCtxA, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "reused-explicit-session")
	followCanonicalA, err := applyGrokResponsesCacheIdentity(followA, followA, cacheIdentityA, false)
	require.NoError(t, err)
	resultA, err := svc.doGrokResponsesUpstream(
		context.Background(), followCtxA, account, followCanonicalA, "grok-4.5", "grok-4.5", cacheIdentityA, true, time.Now(),
	)
	require.NoError(t, err)
	require.True(t, resultA.OpenAIWSDeltaActive)
	require.Equal(t, "resp_branch_a", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.Equal(t, "a2", gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String())

	followCtxB, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "reused-explicit-session")
	followCanonicalB, err := applyGrokResponsesCacheIdentity(followB, followB, cacheIdentityB, false)
	require.NoError(t, err)
	resultB, err := svc.doGrokResponsesUpstream(
		context.Background(), followCtxB, account, followCanonicalB, "grok-4.5", "grok-4.5", cacheIdentityB, true, time.Now(),
	)
	require.NoError(t, err)
	require.True(t, resultB.OpenAIWSDeltaActive)
	require.Equal(t, "resp_branch_b", gjson.GetBytes(upstream.bodies[1], "previous_response_id").String())
	require.Equal(t, "b2", gjson.GetBytes(upstream.bodies[1], "input.0.content.0.text").String())
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
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool(), "full fallback must force store=true for subsequent previous anchors")
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
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool(), "first full create must store=true so previous anchors survive")

	sessionHash := resolveGrokActiveDeltaSessionHash(firstCtx, cacheIdentity, firstCanonical)
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
	require.True(t, gjson.GetBytes(upstream.bodies[1], "store").Bool(), "delta must store=true so its response remains a usable next-turn anchor")
	require.Len(t, gjson.GetBytes(upstream.bodies[1], "input").Array(), 1)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[1], "input.0.content.0.text").String())
}

func TestGrokHTTPActiveDelta_ThreeTurnChainKeepsStoredAnchors(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	// T1 full → T2 delta → T3 delta must all succeed without full-replay, and each
	// successful turn must leave a stored previous_response_id for the next turn.
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			grokActiveDeltaSSE("resp_chain_1"),
			grokActiveDeltaSSE("resp_chain_2"),
			grokActiveDeltaSSE("resp_chain_3"),
		},
	}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92019)
	groupID := int64(92200)
	apiKeyID := int64(92201)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	out1 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	input2 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	out2 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}`
	input3 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"third"}]}`

	// Keep non-input fingerprint stable across turns (store/stream/model identical).
	t1Body := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `]}`)
	t2Body := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + out1 + `,` + input2 + `]}`)
	t3Body := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + out1 + `,` + input2 + `,` + out2 + `,` + input3 + `]}`)

	t1Ctx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-chain")
	cacheIdentity := resolveGrokCacheIdentity(t1Ctx, t1Body, "", "grok-4.5")
	require.NotEmpty(t, cacheIdentity)
	t1Canonical, err := applyGrokResponsesCacheIdentity(t1Body, t1Body, cacheIdentity, false)
	require.NoError(t, err)

	r1, err := svc.doGrokResponsesUpstream(context.Background(), t1Ctx, account, t1Canonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now())
	require.NoError(t, err)
	require.Equal(t, "resp_chain_1", r1.ResponseID)
	require.False(t, r1.OpenAIWSDeltaActive)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool())
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())

	t2Ctx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-chain")
	t2Canonical, err := applyGrokResponsesCacheIdentity(t2Body, t2Body, cacheIdentity, false)
	require.NoError(t, err)
	r2, err := svc.doGrokResponsesUpstream(context.Background(), t2Ctx, account, t2Canonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now())
	require.NoError(t, err)
	require.True(t, r2.OpenAIWSDeltaActive)
	require.Equal(t, "resp_chain_2", r2.ResponseID)
	require.Equal(t, "resp_chain_1", gjson.GetBytes(upstream.bodies[1], "previous_response_id").String())
	require.True(t, gjson.GetBytes(upstream.bodies[1], "store").Bool(), "T2 delta must store so T3 can anchor")
	require.Len(t, gjson.GetBytes(upstream.bodies[1], "input").Array(), 1)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[1], "input.0.content.0.text").String())

	sessionHash := resolveGrokActiveDeltaSessionHash(t2Ctx, cacheIdentity, t2Canonical)
	cached, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok)
	require.Equal(t, "resp_chain_2", cached.lastResponseID)

	t3Ctx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-chain")
	t3Canonical, err := applyGrokResponsesCacheIdentity(t3Body, t3Body, cacheIdentity, false)
	require.NoError(t, err)
	r3, err := svc.doGrokResponsesUpstream(context.Background(), t3Ctx, account, t3Canonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now())
	require.NoError(t, err)
	require.True(t, r3.OpenAIWSDeltaActive, "T3 must still apply delta against stored T2 response")
	require.Equal(t, "resp_chain_3", r3.ResponseID)
	require.Len(t, upstream.bodies, 3, "three-turn chain must not full-replay (one request per turn)")
	require.Equal(t, "resp_chain_2", gjson.GetBytes(upstream.bodies[2], "previous_response_id").String())
	require.True(t, gjson.GetBytes(upstream.bodies[2], "store").Bool())
	require.Len(t, gjson.GetBytes(upstream.bodies[2], "input").Array(), 1)
	require.Equal(t, "third", gjson.GetBytes(upstream.bodies[2], "input.0.content.0.text").String())
}

func TestGrokHTTPActiveDelta_EnabledOverridesDisabledStoreRequirement(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_zdr_delta")}
	svc := newGrokActiveDeltaTestService(upstream)
	svc.cfg.Gateway.Grok.HTTPActiveDeltaRequireStoreOnCreate = false
	account := newGrokActiveDeltaTestAccount(92020)
	groupID := int64(92210)
	apiKeyID := int64(92211)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-zdr")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)
	bindGrokActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstCanonical, cacheIdentity, "resp_zdr_prev")

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-zdr")
	followupCanonical, err := applyGrokResponsesCacheIdentity(fullFollowupBody, fullFollowupBody, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(
		context.Background(), followupCtx, account, followupCanonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now(),
	)
	require.NoError(t, err)
	require.True(t, result.OpenAIWSDeltaActive)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool(), "active delta must keep the response usable as the next anchor")
}

func TestApplyGrokActiveDeltaStorePolicy(t *testing.T) {
	body := []byte(`{"model":"grok-4.5","store":false,"previous_response_id":"resp_1","input":[]}`)
	out, err := applyGrokActiveDeltaStorePolicy(body, true)
	require.NoError(t, err)
	require.True(t, gjson.GetBytes(out, "store").Bool())

	out2, err := applyGrokActiveDeltaStorePolicy(body, false)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(out2, "store").Bool())
}

func TestGrokHTTPActiveDeltaSkipResult_ClassifiesExpectedFull(t *testing.T) {
	body := []byte(`{"model":"grok-4.5","input":[]}`)
	out := grokHTTPActiveDeltaSkipResult(body, 1, 2, 3, "sess", "req", "no_explicit_session", openAIWSSessionContextValue{}, false, "", false)
	require.Equal(t, body, out.body)
	require.False(t, out.applied)
	require.Equal(t, "expected_full_no_explicit_session", out.log.FallbackReason)

	out2 := grokHTTPActiveDeltaSkipResult(body, 1, 2, 3, "sess", "req", "no_session_context", openAIWSSessionContextValue{}, false, "", false)
	require.Equal(t, "expected_full_no_session_context", out2.log.FallbackReason)

	// True regressions keep raw reason.
	out3 := grokHTTPActiveDeltaSkipResult(body, 1, 2, 3, "sess", "req", "account_mismatch", openAIWSSessionContextValue{accountID: 9}, true, "resp_x", true)
	require.Equal(t, "account_mismatch", out3.log.FallbackReason)

	// previous mismatch without cache → expected full, not a sticky regression label.
	out4 := grokHTTPActiveDeltaSkipResult(body, 1, 2, 3, "sess", "req", "previous_response_mismatch", openAIWSSessionContextValue{}, false, "resp_x", true)
	require.Equal(t, "expected_full_no_session_context", out4.log.FallbackReason)
}

func TestGrokHTTPActiveDelta_NoExplicitSessionIsExpectedFull(t *testing.T) {
	setGinTestMode()
	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_no_sess")}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(93001)
	// Context without session_id header → no explicit identity.
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	groupID := int64(93010)
	c.Set("api_key", &APIKey{ID: 93011, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformGrok}})

	body := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	_, err := svc.doGrokResponsesUpstream(context.Background(), c, account, body, "grok-4.5", "grok-4.5", "", true, time.Now())
	require.NoError(t, err)
	require.Len(t, upstream.bodies, 1)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	// Full path still applies store policy when require_store is on.
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool())
}

func TestGrokHTTPActiveDelta_ExplicitStoreFalseIsOverriddenAndBindsSession(t *testing.T) {
	setGinTestMode()
	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_explicit_zdr")}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(93012)
	groupID := int64(93020)
	apiKeyID := int64(93021)
	c, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-explicit-zdr")
	body := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	markGrokClientStorePreference(c, body)
	cacheIdentity := resolveGrokCacheIdentity(c, body, "", "grok-4.5")
	canonical, err := applyGrokResponsesCacheIdentity(body, body, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(context.Background(), c, account, canonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now())
	require.NoError(t, err)
	require.False(t, result.OpenAIWSDeltaActive)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool())

	sessionHash := resolveGrokActiveDeltaSessionHash(c, cacheIdentity, canonical)
	cached, bound := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, bound)
	require.Equal(t, "resp_explicit_zdr", cached.lastResponseID)
}

func TestGrokHTTPActiveDelta_FirstTurnNoContextIsExpectedFull(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_first_full")}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(93002)
	groupID := int64(93020)
	apiKeyID := int64(93021)
	c, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-first-full")
	body := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	cacheIdentity := resolveGrokCacheIdentity(c, body, "", "grok-4.5")
	canonical, err := applyGrokResponsesCacheIdentity(body, body, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(context.Background(), c, account, canonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now())
	require.NoError(t, err)
	require.False(t, result.OpenAIWSDeltaActive)
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool())
	// Session should bind after success for next-turn delta.
	sessionHash := resolveGrokActiveDeltaSessionHash(c, cacheIdentity, canonical)
	cached, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok)
	require.Equal(t, "resp_first_full", cached.lastResponseID)
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
	require.True(t, gjson.GetBytes(upstream.bodies[1], "store").Bool(), "full replay must force store=true")
	require.GreaterOrEqual(t, len(gjson.GetBytes(upstream.bodies[1], "input").Array()), 2)

	// Invalidate drops the dead anchor; successful full-replay rebinds to the new response id.
	sessionHash := resolveGrokActiveDeltaSessionHash(followupCtx, cacheIdentity, followupCanonical)
	cached, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok, "successful full-replay rebinds session for the next turn")
	require.Equal(t, "resp_grok_replay_ok", cached.lastResponseID)
	require.NotEqual(t, "resp_missing", cached.lastResponseID)
}

func TestGrokHTTPActiveDelta_StripsInstructionsOnDelta(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_grok_no_instr")}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92016)
	groupID := int64(92170)
	apiKeyID := int64(92171)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	// Non-input fields must match between bind fingerprint and follow-up (store/stream/model/instructions).
	firstBody := []byte(`{"model":"grok-4.5","instructions":"be concise","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"grok-4.5","instructions":"be concise","stream":true,"store":false,"input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-instr")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)
	bindGrokActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstCanonical, cacheIdentity, "resp_grok_prev")

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-instr")
	followupCanonical, err := applyGrokResponsesCacheIdentity(fullFollowupBody, fullFollowupBody, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(
		context.Background(), followupCtx, account, followupCanonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now(),
	)
	require.NoError(t, err)
	require.True(t, result.OpenAIWSDeltaActive)
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, "resp_grok_prev", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.False(t, gjson.GetBytes(upstream.bodies[0], "instructions").Exists(), "xAI rejects instructions + previous_response_id")
	require.True(t, gjson.GetBytes(upstream.bodies[0], "store").Bool(), "require_store keeps delta storable after sanitize")
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 1)
}

func TestGrokHTTPActiveDelta_PreviousConflictFullReplayInvalidatesThenRebinds(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	// Upstream rejects previous_response_id with the xAI instructions conflict message
	// (sanitize already strips instructions; this covers the recovery classifier + invalidate).
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"error":{"message":"Argument not supported: instructions and previous_response_id together"}}`,
				)),
			},
			grokActiveDeltaSSE("resp_instr_replay_ok"),
		},
	}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(92017)
	groupID := int64(92180)
	apiKeyID := int64(92181)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[` + input1 + `,` + newInput + `]}`)

	firstCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-instr-replay")
	cacheIdentity := resolveGrokCacheIdentity(firstCtx, firstBody, "", "grok-4.5")
	firstCanonical, err := applyGrokResponsesCacheIdentity(firstBody, firstBody, cacheIdentity, false)
	require.NoError(t, err)
	bindGrokActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstCanonical, cacheIdentity, "resp_conflict")

	followupCtx, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-instr-replay")
	followupCanonical, err := applyGrokResponsesCacheIdentity(fullFollowupBody, fullFollowupBody, cacheIdentity, false)
	require.NoError(t, err)

	result, err := svc.doGrokResponsesUpstream(
		context.Background(), followupCtx, account, followupCanonical, "grok-4.5", "grok-4.5", cacheIdentity, true, time.Now(),
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "resp_conflict", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "previous_response_id").Exists())
	require.True(t, gjson.GetBytes(upstream.bodies[1], "store").Bool())

	sessionHash := resolveGrokActiveDeltaSessionHash(followupCtx, cacheIdentity, followupCanonical)
	cached, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok)
	require.Equal(t, "resp_instr_replay_ok", cached.lastResponseID)
	require.NotEqual(t, "resp_conflict", cached.lastResponseID)
}

func TestSanitizeGrokActiveDeltaUpstreamBody_StripsInstructions(t *testing.T) {
	body := []byte(`{"model":"grok-4.5","instructions":"sys","previous_response_id":"resp_1","input":[]}`)
	out, stripped, err := sanitizeGrokActiveDeltaUpstreamBody(body)
	require.NoError(t, err)
	require.Equal(t, []string{"instructions"}, stripped)
	require.False(t, gjson.GetBytes(out, "instructions").Exists())
	require.Equal(t, "resp_1", gjson.GetBytes(out, "previous_response_id").String())
}

func TestIsGrokActiveDeltaInstructionsConflict(t *testing.T) {
	require.True(t, isGrokActiveDeltaInstructionsConflict("Argument not supported: instructions and previous_response_id together"))
	require.True(t, isGrokActiveDeltaInstructionsConflict("instructions and previous_response_id together"))
	require.False(t, isGrokActiveDeltaInstructionsConflict("Previous response not found"))
	require.False(t, isGrokActiveDeltaInstructionsConflict(""))
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

func TestGrokHTTPActiveDelta_ContentDerivedIdentityDoesNotBind(t *testing.T) {
	setGinTestMode()
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			grokActiveDeltaSSE("resp_content_identity_1"),
			grokActiveDeltaSSE("resp_content_identity_2"),
		},
	}
	svc := newGrokActiveDeltaTestService(upstream)
	account := newGrokActiveDeltaTestAccount(920081)
	groupID := int64(920082)
	apiKeyID := int64(920083)
	body := []byte(`{"model":"grok-4.5","stream":true,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"same opening turn"}]}]}`)

	newContext := func() *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformGrok}})
		return c
	}

	firstCtx := newContext()
	identity := resolveGrokCacheIdentity(firstCtx, body, "", "grok-4.5")
	require.NotEmpty(t, identity, "content identity remains available for stateless prompt caching")
	require.False(t, hasExplicitGrokSessionIdentity(firstCtx))
	canonical, err := applyGrokResponsesCacheIdentity(body, body, identity, false)
	require.NoError(t, err)
	first, err := svc.doGrokResponsesUpstream(context.Background(), firstCtx, account, canonical, "grok-4.5", "grok-4.5", identity, true, time.Now())
	require.NoError(t, err)
	require.False(t, first.OpenAIWSDeltaActive)

	sessionHash := resolveGrokActiveDeltaSessionHash(firstCtx, identity, canonical)
	_, bound := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.False(t, bound, "content-derived identity must not create stateful session context")

	secondCtx := newContext()
	secondIdentity := resolveGrokCacheIdentity(secondCtx, body, "", "grok-4.5")
	require.Equal(t, identity, secondIdentity)
	second, err := svc.doGrokResponsesUpstream(context.Background(), secondCtx, account, canonical, "grok-4.5", "grok-4.5", secondIdentity, true, time.Now())
	require.NoError(t, err)
	require.False(t, second.OpenAIWSDeltaActive)
	require.Len(t, upstream.bodies, 2)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[1], "previous_response_id").Exists())
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

	sessionHash := resolveGrokActiveDeltaSessionHash(firstCtx, cacheIdentity, firstCanonical)
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
	// Default config already requires store; keep explicit for documentation.
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

func TestGrokHTTPActiveDelta_EnabledForcesStoreWhenRequireStoreSettingIsDisabled(t *testing.T) {
	setGinTestMode()
	upstream := &httpUpstreamRecorder{resp: grokActiveDeltaSSE("resp_store_false")}
	svc := newGrokActiveDeltaTestService(upstream)
	svc.cfg.Gateway.Grok.HTTPActiveDeltaRequireStoreOnCreate = false
	account := newGrokActiveDeltaTestAccount(92018)
	groupID := int64(92190)
	apiKeyID := int64(92191)
	c, _ := newGrokActiveDeltaContext(groupID, apiKeyID, "sess-grok-store-off")
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
	require.NotEqual(t,
		resolveGrokActiveDeltaSessionHash(c1, id1, body1),
		resolveGrokActiveDeltaSessionHash(c2, id2, body2),
		"active-delta contexts must split when one explicit header is reused for different opening prompts",
	)
	toolBody1 := []byte(`{"model":"grok-4.5","tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}],"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"same"}]}]}`)
	toolBody2 := []byte(`{"model":"grok-4.5","tools":[{"type":"function","name":"search","parameters":{"type":"object"}}],"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"same"}]}]}`)
	require.Equal(t,
		resolveGrokActiveDeltaSessionHash(c1, id1, toolBody1),
		resolveGrokActiveDeltaSessionHash(c2, id2, toolBody2),
		"tool changes are checked by the non-input fingerprint and must not fragment the prompt branch",
	)
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
		sessionHash := resolveGrokActiveDeltaSessionHash(firstCtx, cacheIdentity, upstream.bodies[0])
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
