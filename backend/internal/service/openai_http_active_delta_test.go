package service

import (
	"bytes"
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
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newOpenAIHTTPActiveDeltaTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	cfg.Gateway.OpenAIWS.HTTPIncrementalContinuationEnabled = true
	cfg.Gateway.OpenAIWS.HTTPIncrementalStickyEnabled = true
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 3600
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
	return cfg
}

func newOpenAIHTTPActiveDeltaTestService(upstream *httpUpstreamRecorder) *OpenAIGatewayService {
	cfg := newOpenAIHTTPActiveDeltaTestConfig()
	return &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
	}
}

func newOpenAIHTTPActiveDeltaTestAccount(id int64) *Account {
	return &Account{
		ID:          id,
		Name:        "openai-http-active-delta",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token-http-active-delta",
			"chatgpt_account_id": "chatgpt-acc-http-active-delta",
		},
	}
}

func enableOpenAIHTTPPreviousResponseIDForTest(account *Account) {
	if account == nil {
		return
	}
	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	account.Extra["openai_http_previous_response_id_supported"] = true
}

func newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID int64, sessionID string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("User-Agent", "custom-http-client/1.0")
	c.Request.Header.Set("session_id", sessionID)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
	return c, rec
}

func openAIHTTPActiveDeltaSSE(responseID string) *http.Response {
	body := `event: response.completed
data: {"type":"response.completed","response":{"id":"` + responseID + `","model":"gpt-5.4","usage":{"input_tokens":1,"output_tokens":1}}}

`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{responseID}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func bindOpenAIHTTPActiveDeltaInputOnlyContext(t *testing.T, svc *OpenAIGatewayService, c *gin.Context, account *Account, payload []byte, lastResponseID string) string {
	t.Helper()

	sessionHash := svc.GenerateSessionHash(c, payload)
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

func TestOpenAIGatewayService_Forward_HTTPActiveDeltaUsesServerHistoryWithoutClientPreviousResponseID(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: openAIHTTPActiveDeltaSSE("resp_http_delta_ok")}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91001)
	enableOpenAIHTTPPreviousResponseIDForTest(account)
	groupID := int64(91010)
	apiKeyID := int64(91011)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"service_tier":"auto","input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"service_tier":"auto","input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-history")
	bindOpenAIHTTPActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstBody, "resp_http_prev")

	followupCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-history")
	result, err := svc.Forward(context.Background(), followupCtx, account, fullFollowupBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, "resp_http_prev", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.False(t, gjson.GetBytes(upstream.bodies[0], "store").Bool())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 1)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String())
}

func TestOpenAIGatewayService_Forward_HTTPActiveDeltaBindsOriginalBodyAfterFingerprintNormalization(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: openAIHTTPActiveDeltaSSE("resp_http_original_body_ok")}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	svc.fingerprintNormalizer = NewFingerprintNormalizer(nil, nil, nil, &CanonicalFingerprintConfig{
		Enabled:           true,
		AntiBanEnabled:    true,
		EnabledByPlatform: map[string]bool{"openai": true},
	}, nil)

	account := newOpenAIHTTPActiveDeltaTestAccount(91014)
	enableOpenAIHTTPPreviousResponseIDForTest(account)
	groupID := int64(91140)
	apiKeyID := int64(91141)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-original-body")
	bindOpenAIHTTPActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstBody, "resp_http_prev")

	followupCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-original-body")
	result, err := svc.Forward(context.Background(), followupCtx, account, fullFollowupBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 1)

	originalHash, _, _ := openAIWSNonInputFingerprint(fullFollowupBody)
	_, normalizedFollowupBody, normErr := svc.fingerprintNormalizer.ApplyToRequest(nil, fullFollowupBody, svc.fingerprintNormalizer.ResolveCanonical(context.Background(), account, "custom-http-client/1.0"))
	require.NoError(t, normErr)
	_, _, _ = openAIWSNonInputFingerprint(normalizedFollowupBody)

	sessionHash := svc.GenerateSessionHash(followupCtx, fullFollowupBody)
	cached, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok)
	require.Equal(t, originalHash, cached.nonInputHash)
}

func TestOpenAIGatewayService_Forward_HTTPActiveDeltaDisabledByDefaultForOAuthHTTP(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: openAIHTTPActiveDeltaSSE("resp_http_delta_default_disabled")}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91009)
	groupID := int64(91090)
	apiKeyID := int64(91091)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-disabled-default")
	bindOpenAIHTTPActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstBody, "resp_http_prev")

	followupCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-disabled-default")
	result, err := svc.Forward(context.Background(), followupCtx, account, fullFollowupBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 1)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 3)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[0], "input.2.content.0.text").String())
}

func TestOpenAIGatewayService_Forward_HTTPDefaultDoesNotBindContinuationState(t *testing.T) {
	setGinTestMode()

	upstream := &httpUpstreamRecorder{resp: openAIHTTPActiveDeltaSSE("resp_http_default_no_bind")}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91010)
	groupID := int64(91100)
	apiKeyID := int64(91101)
	body := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	c, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-default-no-bind")
	sessionHash := svc.GenerateSessionHash(c, body)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)

	_, contextFound := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.False(t, contextFound)
	accountID, accountErr := svc.getOpenAIWSStateStore().GetResponseAccount(context.Background(), groupID, apiKeyID, "resp_http_default_no_bind")
	require.NoError(t, accountErr)
	require.Zero(t, accountID)
}

func TestOpenAIGatewayService_Forward_HTTPOAuthWithoutPreviousResponseSupportKeepsDefaultDisabled(t *testing.T) {
	setGinTestMode()

	upstream := &httpUpstreamRecorder{resp: openAIHTTPActiveDeltaSSE("resp_http_no_support")}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91015)
	groupID := int64(91150)
	apiKeyID := int64(91151)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-no-support")
	bindOpenAIHTTPActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstBody, "resp_http_prev")

	followupCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-no-support")
	result, err := svc.Forward(context.Background(), followupCtx, account, fullFollowupBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 1)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 3)
}

func TestOpenAIGatewayService_Forward_HTTPActiveDeltaSkipsCachedWSContext(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: openAIHTTPActiveDeltaSSE("resp_http_delta_ws_context_skipped")}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91012)
	enableOpenAIHTTPPreviousResponseIDForTest(account)
	groupID := int64(91120)
	apiKeyID := int64(91121)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-ws-context")
	sessionHash := bindOpenAIHTTPActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstBody, "resp_ws_prev")
	cached, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok)
	cached.connID = "oa_ws_91012_1"
	svc.getOpenAIWSStateStore().BindSessionContext(groupID, apiKeyID, sessionHash, cached, time.Hour)

	followupCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-ws-context")
	result, err := svc.Forward(context.Background(), followupCtx, account, fullFollowupBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 1)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 3)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[0], "input.2.content.0.text").String())
}

func TestOpenAIGatewayService_Forward_HTTPIngressOAuthPassthroughModePrefersHTTPActiveDelta(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: openAIHTTPActiveDeltaSSE("resp_http_passthrough_mode_delta_ok")}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	svc.cfg.Gateway.OpenAIWS.Enabled = true
	svc.cfg.Gateway.OpenAIWS.OAuthEnabled = true
	svc.cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	svc.cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	svc.cfg.Gateway.OpenAIWS.IngressModeDefault = OpenAIWSIngressModeCtxPool
	svc.cfg.Gateway.OpenAIWS.HttpIngressUpstreamWSEnabled = true
	svc.openaiWSURLBuilder = func(*Account) (string, error) {
		return "", errors.New("test ws should not be selected")
	}

	account := newOpenAIHTTPActiveDeltaTestAccount(91007)
	enableOpenAIHTTPPreviousResponseIDForTest(account)
	account.Extra = map[string]any{
		"openai_http_previous_response_id_supported":   true,
		"openai_passthrough":                           false,
		"openai_oauth_responses_websockets_v2_mode":    OpenAIWSIngressModePassthrough,
		"openai_oauth_responses_websockets_v2_enabled": true,
	}
	groupID := int64(91070)
	apiKeyID := int64(91071)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-passthrough-mode")
	bindOpenAIHTTPActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstBody, "resp_http_prev")

	followupCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-passthrough-mode")
	result, err := svc.Forward(context.Background(), followupCtx, account, fullFollowupBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, "resp_http_prev", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 1)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String())

	decision, exists := followupCtx.Get("openai_ws_transport_decision")
	require.True(t, exists)
	require.Equal(t, string(OpenAIUpstreamTransportHTTPSSE), decision)
	reason, exists := followupCtx.Get("openai_ws_transport_reason")
	require.True(t, exists)
	require.Equal(t, "http_incremental_preferred_non_ctx_pool", reason)
}

func TestOpenAIGatewayService_ShouldPreferHTTPIncrementalForHTTPIngressModes(t *testing.T) {
	cfg := newOpenAIHTTPActiveDeltaTestConfig()
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.IngressModeDefault = OpenAIWSIngressModeCtxPool
	svc := &OpenAIGatewayService{cfg: cfg}

	newAccount := func(mode string, passthrough bool) *Account {
		account := newOpenAIHTTPActiveDeltaTestAccount(91008)
		account.Extra = map[string]any{
			"openai_http_previous_response_id_supported": true,
			"openai_passthrough":                         passthrough,
			"openai_oauth_responses_websockets_v2_mode":  mode,
		}
		return account
	}

	require.True(t, svc.shouldPreferOpenAIHTTPIncrementalForHTTPIngress(newAccount(OpenAIWSIngressModePassthrough, false), OpenAIClientTransportHTTP))
	require.False(t, svc.shouldPreferOpenAIHTTPIncrementalForHTTPIngress(newAccount(OpenAIWSIngressModeCtxPool, false), OpenAIClientTransportHTTP))
	require.False(t, svc.shouldPreferOpenAIHTTPIncrementalForHTTPIngress(newAccount(OpenAIWSIngressModeShared, false), OpenAIClientTransportHTTP))
	require.False(t, svc.shouldPreferOpenAIHTTPIncrementalForHTTPIngress(newAccount(OpenAIWSIngressModeDedicated, false), OpenAIClientTransportHTTP))
	require.False(t, svc.shouldPreferOpenAIHTTPIncrementalForHTTPIngress(newAccount(OpenAIWSIngressModeOff, false), OpenAIClientTransportHTTP))
	require.False(t, svc.shouldPreferOpenAIHTTPIncrementalForHTTPIngress(newAccount(OpenAIWSIngressModePassthrough, true), OpenAIClientTransportHTTP))
	require.False(t, svc.shouldPreferOpenAIHTTPIncrementalForHTTPIngress(newAccount(OpenAIWSIngressModePassthrough, false), OpenAIClientTransportWS))
}

func TestOpenAIGatewayService_Forward_HTTPActiveDeltaUsesExplicitClientPreviousResponseID(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: openAIHTTPActiveDeltaSSE("resp_http_explicit_ok")}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91002)
	enableOpenAIHTTPPreviousResponseIDForTest(account)
	groupID := int64(91020)
	apiKeyID := int64(91021)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"previous_response_id":"resp_http_prev","input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-explicit")
	bindOpenAIHTTPActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstBody, "resp_http_prev")

	followupCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-explicit")
	result, err := svc.Forward(context.Background(), followupCtx, account, fullFollowupBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, "resp_http_prev", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.False(t, gjson.GetBytes(upstream.bodies[0], "store").Bool())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 1)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String())
}

func TestOpenAIGatewayService_Forward_HTTPSuccessBindsActiveDeltaSessionContext(t *testing.T) {
	setGinTestMode()

	upstream := &httpUpstreamRecorder{resp: openAIHTTPActiveDeltaSSE("resp_http_bind_1")}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91003)
	enableOpenAIHTTPPreviousResponseIDForTest(account)
	groupID := int64(91030)
	apiKeyID := int64(91031)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	body := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `]}`)
	c, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-bind")
	sessionHash := svc.GenerateSessionHash(c, body)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)

	cached, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok, "successful HTTP responses should bind input-only session context for later active delta")
	require.Equal(t, account.ID, cached.accountID)
	require.Equal(t, "resp_http_bind_1", cached.lastResponseID)
	require.Equal(t, 1, cached.materializedCount)
	require.Equal(t, 1, cached.inputCount)
	require.True(t, cached.inputOnlyContext)
	require.True(t, cached.rawVsClientVisibleEqual)
}

func TestOpenAIGatewayService_Forward_HTTPActiveDeltaUnsupportedPreviousResponseIDRetriesFullBody(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-prev-unsupported"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"unsupported_parameter","message":"Unsupported parameter: previous_response_id"}}`)),
		},
		openAIHTTPActiveDeltaSSE("resp_http_retry_full_ok"),
	}}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91004)
	enableOpenAIHTTPPreviousResponseIDForTest(account)
	groupID := int64(91040)
	apiKeyID := int64(91041)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-retry")
	bindOpenAIHTTPActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstBody, "resp_http_prev")

	followupCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-retry")
	result, err := svc.Forward(context.Background(), followupCtx, account, fullFollowupBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "resp_http_prev", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 1)
	require.False(t, gjson.GetBytes(upstream.bodies[1], "previous_response_id").Exists(), "retry must remove the continuation anchor")
	require.Len(t, gjson.GetBytes(upstream.bodies[1], "input").Array(), 3, "retry must restore the original full request body, not delta-only input")
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[1], "input.2.content.0.text").String())
}

func TestOpenAIGatewayService_Forward_HTTPActiveDeltaUnsupportedPreviousResponseIDRetriesFullBodyWithFunctionCallOutput(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-prev-unsupported-tool"}},
			Body:       io.NopCloser(strings.NewReader(`{"detail":"Unsupported parameter: previous_response_id"}`)),
		},
		openAIHTTPActiveDeltaSSE("resp_http_retry_full_tool_ok"),
	}}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91013)
	enableOpenAIHTTPPreviousResponseIDForTest(account)
	groupID := int64(91130)
	apiKeyID := int64(91131)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"run"}]}`
	toolCall := `{"type":"function_call","call_id":"call_1","name":"shell","arguments":"{}"}`
	toolOutput := `{"type":"function_call_output","call_id":"call_1","output":"ok"}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}`
	firstBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `,` + toolCall + `,` + toolOutput + `,` + newInput + `]}`)

	firstCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-retry-tool")
	bindOpenAIHTTPActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstBody, "resp_http_prev_tool")

	followupCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-retry-tool")
	result, err := svc.Forward(context.Background(), followupCtx, account, fullFollowupBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "resp_http_prev_tool", gjson.GetBytes(upstream.bodies[0], "previous_response_id").String())
	require.True(t, HasToolContinuationOutputInRawPayload(upstream.bodies[0]))
	require.False(t, gjson.GetBytes(upstream.bodies[1], "previous_response_id").Exists(), "retry must remove the gateway-injected continuation anchor")
	require.Len(t, gjson.GetBytes(upstream.bodies[1], "input").Array(), 4, "retry must restore the original full request body")
	require.Equal(t, "function_call", gjson.GetBytes(upstream.bodies[1], "input.1.type").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(upstream.bodies[1], "input.2.type").String())
	require.Equal(t, "continue", gjson.GetBytes(upstream.bodies[1], "input.3.content.0.text").String())
}

func TestOpenAIGatewayService_Forward_HTTPActiveDeltaRejectsPreviousResponseAccountMismatch(t *testing.T) {
	setGinTestMode()
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	upstream := &httpUpstreamRecorder{resp: openAIHTTPActiveDeltaSSE("resp_http_account_mismatch_ok")}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91006)
	groupID := int64(91060)
	apiKeyID := int64(91061)

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	firstBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"input":[` + input1 + `]}`)
	fullFollowupBody := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"previous_response_id":"resp_http_prev","input":[` + input1 + `,` + replayedOutput + `,` + newInput + `]}`)

	firstCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-account-mismatch")
	bindOpenAIHTTPActiveDeltaInputOnlyContext(t, svc, firstCtx, account, firstBody, "resp_http_prev")
	require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(context.Background(), groupID, apiKeyID, "resp_http_prev", account.ID+1000, time.Hour))

	followupCtx, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-delta-account-mismatch")
	result, err := svc.Forward(context.Background(), followupCtx, account, fullFollowupBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 1)
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.Len(t, gjson.GetBytes(upstream.bodies[0], "input").Array(), 3)
	require.Equal(t, "again", gjson.GetBytes(upstream.bodies[0], "input.2.content.0.text").String())
}

func TestOpenAIGatewayService_Forward_HTTPNonStreamingBindsResponseID(t *testing.T) {
	setGinTestMode()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-http-nonstream"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_http_nonstream_1","usage":{"input_tokens":1,"output_tokens":1},"output":[]}`)),
	}}
	svc := newOpenAIHTTPActiveDeltaTestService(upstream)
	account := newOpenAIHTTPActiveDeltaTestAccount(91005)
	enableOpenAIHTTPPreviousResponseIDForTest(account)
	groupID := int64(91050)
	apiKeyID := int64(91051)

	body := []byte(`{"model":"gpt-5.4","instructions":"test","stream":false,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	c, _ := newOpenAIHTTPActiveDeltaContext(groupID, apiKeyID, "sess-http-nonstream-bind")
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_http_nonstream_1", result.ResponseID)

	accountID, err := svc.getOpenAIWSStateStore().GetResponseAccount(context.Background(), groupID, apiKeyID, "resp_http_nonstream_1")
	require.NoError(t, err)
	require.Equal(t, account.ID, accountID)
}

func TestOpenAIHTTPActiveDeltaResponsePayloadFixtureIsValidJSON(t *testing.T) {
	resp := openAIHTTPActiveDeltaSSE("resp_fixture")
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	_, payload, ok := extractOpenAISSETerminalEvent(string(body))
	require.True(t, ok)
	require.True(t, json.Valid(payload))
}

func TestRestoreOpenAIHTTPActiveDeltaFullReplayBodyRefusesFunctionCallOutput(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"store":false,"previous_response_id":"resp_prev","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)
	restored, ok, err := restoreOpenAIHTTPActiveDeltaFullReplayBody(body)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, restored)
}
func TestRestoreOpenAIHTTPActiveDeltaFullReplayBodyKeepsFunctionCallOutput(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","stream":true,"store":false,"previous_response_id":"resp_prev","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)
	restored, ok, err := restoreOpenAIHTTPActiveDeltaFullReplayBody(body)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, restored)
}
