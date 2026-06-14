package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TEMP_DIAG(openai_ws_delta_shadow) remove_after_debug=true — tests for shadow helpers.

func mustItemHash(t *testing.T, raw string) [32]byte {
	t.Helper()
	h, ok := openAIWSCanonicalItemHash([]byte(raw))
	require.True(t, ok, "canonical hash failed for %s", raw)
	return h
}

func TestOpenAIWSCanonicalItemHash_VolatileEnvelopeStrippedButContentPreserved(t *testing.T) {
	// message: top-level id/status are volatile envelope -> stripped -> same hash.
	base := `{"type":"message","role":"user","content":"hi"}`
	withEnvelope := `{"type":"message","id":"msg_123","status":"completed","role":"user","content":"hi"}`
	require.Equal(t, mustItemHash(t, base), mustItemHash(t, withEnvelope),
		"volatile id/status must not affect message hash")

	// item_reference: id is semantic -> never stripped -> different ids hash differently.
	ref1 := `{"type":"item_reference","id":"resp_1"}`
	ref2 := `{"type":"item_reference","id":"resp_2"}`
	require.NotEqual(t, mustItemHash(t, ref1), mustItemHash(t, ref2),
		"item_reference id is semantic and must change the hash (false-match guard)")

	// key order is canonicalized -> same hash regardless of field order.
	reordered := `{"content":"hi","role":"user","type":"message"}`
	require.Equal(t, mustItemHash(t, base), mustItemHash(t, reordered))
}

func TestOpenAIWSCanonicalItemHash_ReplayEquivalentEnvelopeFields(t *testing.T) {
	rawReasoning := `{"type":"reasoning","id":"rs_1","status":"completed","metadata":{"turn_id":"turn_1"},"content":[],"summary":[],"encrypted_content":"enc"}`
	replayedReasoning := `{"type":"reasoning","summary":[],"encrypted_content":"enc"}`
	require.Equal(t, mustItemHash(t, rawReasoning), mustItemHash(t, replayedReasoning))

	rawMessage := `{"type":"message","id":"msg_1","status":"completed","metadata":{"turn_id":"turn_1"},"role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[],"logprobs":[]}]}`
	replayedMessage := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	require.Equal(t, mustItemHash(t, rawMessage), mustItemHash(t, replayedMessage))

	rawFunctionCall := `{"type":"function_call","id":"fc_1","status":"completed","metadata":{"turn_id":"turn_1"},"call_id":"call_1","name":"run","arguments":"{}"}`
	replayedFunctionCall := `{"type":"function_call","call_id":"call_1","name":"run","arguments":"{}"}`
	require.Equal(t, mustItemHash(t, rawFunctionCall), mustItemHash(t, replayedFunctionCall))

	rawCustomToolCall := `{"type":"custom_tool_call","id":"ctc_1","status":"completed","metadata":{"turn_id":"turn_1"},"call_id":"call_1","name":"apply_patch","input":"diff"}`
	replayedCustomToolCall := `{"type":"custom_tool_call","status":"completed","call_id":"call_1","name":"apply_patch","input":"diff"}`
	require.Equal(t, mustItemHash(t, rawCustomToolCall), mustItemHash(t, replayedCustomToolCall))
}

func TestOpenAIWSCanonicalItemHash_DoesNotDropSemanticEnvelopeLookalikes(t *testing.T) {
	messageWithAnnotation := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation"}]}]}`
	messageWithoutAnnotation := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	require.NotEqual(t, mustItemHash(t, messageWithAnnotation), mustItemHash(t, messageWithoutAnnotation))

	messageWithSemanticMetadata := `{"type":"message","role":"assistant","metadata":{"turn_id":"turn_1","semantic":"keep"},"content":[{"type":"output_text","text":"hello"}]}`
	messageWithoutMetadata := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	require.NotEqual(t, mustItemHash(t, messageWithSemanticMetadata), mustItemHash(t, messageWithoutMetadata))
}

func TestOpenAIWSCanonicalItemHash_WhitespaceInContentBreaksHash(t *testing.T) {
	oneSpace := `{"type":"message","role":"user","content":"a b"}`
	twoSpace := `{"type":"message","role":"user","content":"a  b"}`
	require.NotEqual(t, mustItemHash(t, oneSpace), mustItemHash(t, twoSpace),
		"whitespace inside text content must change the hash")
}

func TestOpenAIWSNonInputHash_DenylistIgnoresVolatileFields(t *testing.T) {
	p1 := []byte(`{"model":"gpt","tools":[{"type":"x"}],"input":[{"a":1}],"previous_response_id":"resp_1","store":false,"client_metadata":{"x-client-request-id":"a"},"turn_metadata":"t1"}`)
	p2 := []byte(`{"model":"gpt","tools":[{"type":"x"}],"input":[{"b":2},{"c":3}],"previous_response_id":"resp_9","store":true,"client_metadata":{"x-client-request-id":"z"},"turn_metadata":"t2"}`)
	h1, ig1 := openAIWSNonInputHash(p1)
	h2, _ := openAIWSNonInputHash(p2)
	require.Equal(t, h1, h2, "denylist fields must not affect the non-input fingerprint")
	require.Subset(t, ig1, []string{"input", "previous_response_id", "store", "client_metadata", "turn_metadata"})

	// a semantic change (model) must change the fingerprint.
	p3 := []byte(`{"model":"gpt-other","tools":[{"type":"x"}],"input":[{"a":1}]}`)
	h3, _ := openAIWSNonInputHash(p3)
	require.NotEqual(t, h1, h3)

	// an unknown future field is included by default.
	p4 := []byte(`{"model":"gpt","tools":[{"type":"x"}],"future_param":true}`)
	h4, _ := openAIWSNonInputHash(p4)
	require.NotEqual(t, h1, h4, "unknown fields must be included so future params can't false-match")
}

func TestOpenAIWSExtractResponseOutputItems(t *testing.T) {
	items, ok := openAIWSExtractResponseOutputItems([]byte(`{"id":"resp_1","output":[{"type":"message","role":"assistant","content":"hi"}]}`))
	require.True(t, ok)
	require.Len(t, items, 1)

	_, ok = openAIWSExtractResponseOutputItems([]byte(`{"id":"resp_1"}`))
	require.False(t, ok, "missing output array must report not-captured")

	_, ok = openAIWSExtractResponseOutputItems([]byte(`{"id":"resp_1","output":[]}`))
	require.False(t, ok, "empty output array must report not-captured")
}

func TestOpenAIWSExtractOutputItemDoneItem(t *testing.T) {
	idx, item, ok := openAIWSExtractOutputItemDoneItem([]byte(`{"type":"response.output_item.done","output_index":2,"item":{"type":"message","role":"assistant","content":"hi"}}`))
	require.True(t, ok)
	require.Equal(t, 2, idx)
	require.JSONEq(t, `{"type":"message","role":"assistant","content":"hi"}`, string(item))

	_, _, ok = openAIWSExtractOutputItemDoneItem([]byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"message"}}`))
	require.False(t, ok)

	ordered := openAIWSOrderedOutputDoneItems(map[int]json.RawMessage{
		2: json.RawMessage(`{"type":"message","content":"two"}`),
		0: json.RawMessage(`{"type":"message","content":"zero"}`),
	})
	require.Len(t, ordered, 2)
	require.JSONEq(t, `{"type":"message","content":"zero"}`, string(ordered[0]))
	require.JSONEq(t, `{"type":"message","content":"two"}`, string(ordered[1]))
}

func TestOpenAIWSHashPrefixBreak(t *testing.T) {
	a := mustItemHash(t, `{"type":"message","content":"1"}`)
	b := mustItemHash(t, `{"type":"message","content":"2"}`)
	c := mustItemHash(t, `{"type":"message","content":"3"}`)

	matched, idx := openAIWSHashPrefixBreak([][32]byte{a, b, c}, [][32]byte{a, b})
	require.True(t, matched)
	require.Equal(t, -1, idx)

	matched, idx = openAIWSHashPrefixBreak([][32]byte{a, c, c}, [][32]byte{a, b})
	require.False(t, matched)
	require.Equal(t, 1, idx)

	// prefix longer than full -> break at len(full).
	matched, idx = openAIWSHashPrefixBreak([][32]byte{a}, [][32]byte{a, b})
	require.False(t, matched)
	require.Equal(t, 1, idx)
}

func buildShadowCandidateFixture(t *testing.T) (payload []byte, cached openAIWSSessionContextValue) {
	t.Helper()
	msg1 := `{"type":"message","role":"user","content":"hi"}`
	out1 := `{"type":"message","role":"assistant","content":"hello"}`
	newMsg := `{"type":"message","role":"user","content":"again"}`
	payload = []byte(`{"model":"gpt","input":[` + msg1 + `,` + out1 + `,` + newMsg + `],"store":false,"previous_response_id":"resp_1"}`)

	h1 := mustItemHash(t, msg1)
	h2 := mustItemHash(t, out1)
	nonInput, _ := openAIWSNonInputHash(payload)
	cached = openAIWSSessionContextValue{
		accountID:               5,
		connID:                  "conn_a",
		lastResponseID:          "resp_1",
		materializedHashes:      [][32]byte{h1, h2},
		materializedCount:       2,
		inputCount:              1,
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}
	return payload, cached
}

func TestEvaluateDeltaShadowCandidate_Happy(t *testing.T) {
	payload, cached := buildShadowCandidateFixture(t)
	log := evaluateOpenAIWSDeltaShadowCandidate(openAIWSDeltaShadowInput{
		RequestID:                "req1",
		AccountID:                5,
		LeaseConnID:              "conn_a",
		ConnMostRecentResponseID: "resp_1",
		CurrentPayload:           payload,
		HasFunctionCallOutput:    false,
		Cached:                   cached,
		CachedFound:              true,
	})
	require.True(t, log.Candidate)
	require.True(t, log.PrefixMatch)
	require.Equal(t, 1, log.DeltaItems, "only the trailing new item should be the delta")
	require.Equal(t, 3, log.FullItems)
	require.Less(t, log.DeltaBytes, log.FullBytes)
}

func TestEvaluateDeltaShadowCandidate_FallbackReasons(t *testing.T) {
	payload, cached := buildShadowCandidateFixture(t)
	base := openAIWSDeltaShadowInput{
		RequestID: "r", AccountID: 5, LeaseConnID: "conn_a",
		ConnMostRecentResponseID: "resp_1", CurrentPayload: payload,
		Cached: cached, CachedFound: true,
	}

	// no session context
	noCtx := base
	noCtx.CachedFound = false
	require.Equal(t, "no_session_context", evaluateOpenAIWSDeltaShadowCandidate(noCtx).FallbackReason)

	// conn mismatch
	connMis := base
	connMis.LeaseConnID = "conn_b"
	r := evaluateOpenAIWSDeltaShadowCandidate(connMis)
	require.False(t, r.Candidate)
	require.Equal(t, "conn_mismatch", r.FallbackReason)

	// not most recent
	notRecent := base
	notRecent.ConnMostRecentResponseID = "resp_other"
	require.Equal(t, "not_most_recent", evaluateOpenAIWSDeltaShadowCandidate(notRecent).FallbackReason)

	// raw vs client divergent
	divergent := base
	divCached := cached
	divCached.rawVsClientVisibleEqual = false
	divergent.Cached = divCached
	require.Equal(t, "raw_client_divergent", evaluateOpenAIWSDeltaShadowCandidate(divergent).FallbackReason)

	// tool continuation
	tool := base
	tool.HasFunctionCallOutput = true
	require.Equal(t, "tool_continuation", evaluateOpenAIWSDeltaShadowCandidate(tool).FallbackReason)
}

func TestEvaluateDeltaShadowCandidate_OutputBoundaryBreak(t *testing.T) {
	// current input echoes the previous OUTPUT item with a changed content -> break at output boundary.
	msg1 := `{"type":"message","role":"user","content":"hi"}`
	out1 := `{"type":"message","role":"assistant","content":"hello"}`
	out1Changed := `{"type":"message","role":"assistant","content":"HELLO-changed"}`
	newMsg := `{"type":"message","role":"user","content":"again"}`
	payload := []byte(`{"model":"gpt","input":[` + msg1 + `,` + out1Changed + `,` + newMsg + `]}`)

	nonInput, _ := openAIWSNonInputHash(payload)
	cached := openAIWSSessionContextValue{
		accountID: 5, connID: "conn_a", lastResponseID: "resp_1",
		materializedHashes:      [][32]byte{mustItemHash(t, msg1), mustItemHash(t, out1)},
		materializedCount:       2,
		inputCount:              1, // index 0 is input; index 1 is output boundary
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}
	log := evaluateOpenAIWSDeltaShadowCandidate(openAIWSDeltaShadowInput{
		RequestID: "r", AccountID: 5, LeaseConnID: "conn_a",
		ConnMostRecentResponseID: "resp_1", CurrentPayload: payload,
		Cached: cached, CachedFound: true,
	})
	require.False(t, log.Candidate)
	require.False(t, log.PrefixMatch)
	require.Equal(t, "output", log.BreakBoundary)
	require.Equal(t, "prefix_break_output", log.FallbackReason)
}

func TestEvaluateDeltaShadowCandidate_OutputBoundaryBreakIncludesShapeDiagnostics(t *testing.T) {
	msg1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	rawReasoning := `{"type":"reasoning","id":"rs_1","status":"completed","summary":[{"type":"summary_text","text":"hidden raw reasoning"}]}`
	replayedReasoning := `{"type":"reasoning","content":[{"type":"output_text","text":"hidden raw reasoning"}]}`
	newMsg := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	payload := []byte(`{"model":"gpt","input":[` + msg1 + `,` + replayedReasoning + `,` + newMsg + `]}`)

	materializedItems := []json.RawMessage{
		json.RawMessage(msg1),
		json.RawMessage(rawReasoning),
	}
	materializedShapes, ok := openAIWSItemShapes(materializedItems)
	require.True(t, ok)
	nonInput, _ := openAIWSNonInputHash(payload)
	cached := openAIWSSessionContextValue{
		accountID: 5, connID: "conn_a", lastResponseID: "resp_1",
		materializedHashes:      [][32]byte{mustItemHash(t, msg1), mustItemHash(t, rawReasoning)},
		materializedShapes:      materializedShapes,
		materializedCount:       2,
		inputCount:              1,
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}

	log := evaluateOpenAIWSDeltaShadowCandidate(openAIWSDeltaShadowInput{
		RequestID: "r", AccountID: 5, LeaseConnID: "conn_a",
		ConnMostRecentResponseID: "resp_1", CurrentPayload: payload,
		Cached: cached, CachedFound: true,
	})
	require.False(t, log.Candidate)
	require.Equal(t, "output", log.BreakBoundary)
	require.Equal(t, "reasoning", log.BreakItemType)
	require.Equal(t, "reasoning", log.BreakCachedItemType)
	require.Contains(t, log.BreakCachedShape, "summary[]")
	require.Contains(t, log.BreakCurrentShape, "content[]")
	require.NotContains(t, log.BreakCachedShape, "hidden raw reasoning")
	require.NotContains(t, log.BreakCurrentShape, "hidden raw reasoning")
}

func TestEvaluateDeltaShadowCandidate_ItemCountDecreaseBreaks(t *testing.T) {
	msg1 := `{"type":"message","role":"user","content":"hi"}`
	payload := []byte(`{"model":"gpt","input":[` + msg1 + `]}`) // shorter than materialized prefix
	nonInput, _ := openAIWSNonInputHash(payload)
	cached := openAIWSSessionContextValue{
		accountID: 5, connID: "conn_a", lastResponseID: "resp_1",
		materializedHashes:      [][32]byte{mustItemHash(t, msg1), mustItemHash(t, `{"type":"message","content":"x"}`)},
		materializedCount:       2,
		inputCount:              1,
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}
	log := evaluateOpenAIWSDeltaShadowCandidate(openAIWSDeltaShadowInput{
		RequestID: "r", AccountID: 5, LeaseConnID: "conn_a",
		ConnMostRecentResponseID: "resp_1", CurrentPayload: payload,
		Cached: cached, CachedFound: true,
	})
	require.False(t, log.Candidate)
	require.False(t, log.PrefixMatch)
}

func TestOpenAIWSStateStore_SessionContextRoundTrip(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	val := openAIWSSessionContextValue{
		accountID:          5,
		connID:             "conn_a",
		lastResponseID:     "resp_1",
		materializedHashes: [][32]byte{{1}, {2}},
		materializedCount:  2,
		inputCount:         1,
		nonInputHash:       [32]byte{9},
	}
	store.BindSessionContext(7, 11, "sess", val, time.Minute)

	got, ok := store.GetSessionContext(7, 11, "sess")
	require.True(t, ok)
	require.Equal(t, val, got)

	// api-key isolation
	_, ok = store.GetSessionContext(7, 12, "sess")
	require.False(t, ok)

	store.DeleteSessionContext(7, 11, "sess")
	_, ok = store.GetSessionContext(7, 11, "sess")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_ConnLastResponseAndInFlight(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)

	store.BindConnLastResponse("conn_a", "resp_1", time.Minute)
	got, ok := store.GetConnLastResponse("conn_a")
	require.True(t, ok)
	require.Equal(t, "resp_1", got)

	store.DeleteConnLastResponse("conn_a")
	_, ok = store.GetConnLastResponse("conn_a")
	require.False(t, ok)

	// in-flight is atomic: first TryBegin owns; second fails until End.
	require.True(t, store.TrySessionInFlight(7, 11, "sess"))
	require.False(t, store.TrySessionInFlight(7, 11, "sess"))
	store.EndSessionInFlight(7, 11, "sess")
	require.True(t, store.TrySessionInFlight(7, 11, "sess"))
	store.EndSessionInFlight(7, 11, "sess")
}

func TestOpenAIWSConnEvictHookInvalidatesConnLastResponse(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	store.BindConnLastResponse("conn_x", "resp_1", time.Minute)

	RegisterOpenAIWSConnEvictHook(func(connID string) { store.DeleteConnLastResponse(connID) })
	t.Cleanup(func() { RegisterOpenAIWSConnEvictHook(nil) })

	ap := &openAIWSAccountPool{conns: map[string]*openAIWSConn{"conn_x": {}}}
	deleteOpenAIWSAccountConnLocked(ap, "conn_x")

	_, present := ap.conns["conn_x"]
	require.False(t, present, "conn must be removed from pool")
	_, ok := store.GetConnLastResponse("conn_x")
	require.False(t, ok, "evict hook must invalidate conn_id -> last_response_id")
}

func TestOpenAIWSDeltaShadow_ForwardWSV2SessionBoundWritesContextAndStaysInert(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 3600
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	output1 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	output2 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}`

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.output_item.done","response_id":"resp_delta_forward_1","output_index":0,"item":` + output1 + `}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_delta_forward_1","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":2}}}`),
			[]byte(`{"type":"response.output_item.done","response_id":"resp_delta_forward_2","output_index":0,"item":` + output2 + `}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_delta_forward_2","model":"gpt-5.1","usage":{"input_tokens":5,"output_tokens":2}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          78001,
		Name:        "openai-delta-shadow-forward",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token-delta-forward"},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}

	groupID := int64(78010)
	apiKeyID := int64(78011)
	newContext := func() *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
		c.Request.Header.Set("originator", "codex_exec")
		c.Request.Header.Set("session_id", "sess-delta-shadow-forward")
		c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
		return c
	}

	firstCtx := newContext()
	sessionHash := svc.GenerateSessionHash(firstCtx, nil)
	require.NotEmpty(t, sessionHash)
	firstBody := []byte(`{"model":"gpt-5.1","stream":true,"input":[` + input1 + `]}`)
	firstResult, err := svc.Forward(context.Background(), firstCtx, account, firstBody)
	require.NoError(t, err)
	require.Equal(t, "resp_delta_forward_1", firstResult.RequestID)

	stateStore := svc.getOpenAIWSStateStore()
	cached, ok := stateStore.GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok, "session-bound forward WS path must write delta-shadow session context")
	require.Equal(t, account.ID, cached.accountID)
	require.Equal(t, "resp_delta_forward_1", cached.lastResponseID)
	require.Equal(t, 2, cached.materializedCount, "context must be input_N + raw output_N")
	require.Equal(t, 1, cached.inputCount)
	require.True(t, cached.rawVsClientVisibleEqual)

	connLast, ok := stateStore.GetConnLastResponse(cached.connID)
	require.True(t, ok)
	require.Equal(t, "resp_delta_forward_1", connLast)

	secondCtx := newContext()
	secondBody := []byte(`{"model":"gpt-5.1","stream":true,"input":[` + input1 + `,` + output1 + `,` + newInput + `]}`)
	secondResult, err := svc.Forward(context.Background(), secondCtx, account, secondBody)
	require.NoError(t, err)
	require.Equal(t, "resp_delta_forward_2", secondResult.RequestID)

	captureConn.mu.Lock()
	writes := append([]map[string]any(nil), captureConn.writes...)
	captureConn.mu.Unlock()
	require.Len(t, writes, 2)
	secondWrite := requestToJSONString(writes[1])
	require.False(t, gjson.Get(secondWrite, "previous_response_id").Exists(), "shadow phase must not mutate upstream payload")
	require.Len(t, gjson.Get(secondWrite, "input").Array(), 3, "shadow phase must leave full replay input intact")

	cached, ok = stateStore.GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok)
	require.Equal(t, "resp_delta_forward_2", cached.lastResponseID)
}
