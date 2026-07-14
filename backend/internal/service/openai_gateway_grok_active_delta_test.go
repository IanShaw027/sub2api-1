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
