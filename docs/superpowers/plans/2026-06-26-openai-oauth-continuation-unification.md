# OpenAI OAuth Continuation Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the best-practice OpenAI OAuth continuation architecture: `store=false` traffic prefers hot WS incremental continuation, falls back to full rebuild instead of unsafe cold HTTP continuation, preserves sticky session/account behavior, and adds a separate durable continuation lane only behind explicit flags.

**Architecture:** Introduce a shared continuation planner plus a shared rebuild-window store, route OAuth `store=false` traffic through `HotWSIncremental -> FullRebuild -> RejectUnsafeContinuation`, and defer HTTP continuation to an explicit durable lane after the fallback surface is correct. Add a dedicated performance phase for compaction, WS warm-up, and `service_tier=priority` policy instead of assuming continuation refactoring alone maximizes TTFT or throughput.

**Tech Stack:** Go, Gin, existing OpenAI WS state store and GatewayCache, existing Responses ingress normalization, existing OpenAI WS/HTTP protocol tests.

---

### Task 1: Planner Contracts For The Primary OAuth Lane

**Files:**
- Create: `backend/internal/service/openai_responses_continuation_planner.go`
- Create: `backend/internal/service/openai_responses_continuation_planner_test.go`

- [ ] **Step 1: Write the failing planner tests for the primary lane**

```go
func TestOpenAIResponsesContinuationPlanner_OAuthStoreFalsePrefersHotWS(t *testing.T) {
	planner := openAIResponsesContinuationPlanner{}
	decision := planner.Plan(openAIResponsesContinuationInput{
		AccountType:        AccountTypeOAuth,
		StoreDisabled:      true,
		PreviousResponseID: "resp_hot_1",
		LiveWSAvailable:    true,
		StickyAccountHit:   true,
		StickyConnHit:      true,
	})
	require.Equal(t, openAIResponsesContinuationActionHotWSIncremental, decision.Action)
}

func TestOpenAIResponsesContinuationPlanner_OAuthStoreFalseWithoutLiveWSRebuilds(t *testing.T) {
	planner := openAIResponsesContinuationPlanner{}
	decision := planner.Plan(openAIResponsesContinuationInput{
		AccountType:            AccountTypeOAuth,
		StoreDisabled:          true,
		PreviousResponseID:     "resp_cold_1",
		LiveWSAvailable:        false,
		StickyAccountHit:       true,
		SessionWindowAvailable: true,
	})
	require.Equal(t, openAIResponsesContinuationActionFullRebuild, decision.Action)
}

func TestOpenAIResponsesContinuationPlanner_DurableLaneAllowsColdHTTP(t *testing.T) {
	planner := openAIResponsesContinuationPlanner{}
	decision := planner.Plan(openAIResponsesContinuationInput{
		AccountType:                 AccountTypeOAuth,
		StoreDisabled:               false,
		PreviousResponseID:          "resp_durable_1",
		LiveWSAvailable:             false,
		StickyAccountHit:            true,
		SessionWindowAvailable:      true,
		DurableContinuationAllowed:  true,
		PersistedContinuationAvailable: true,
	})
	require.Equal(t, openAIResponsesContinuationActionColdHTTPIncremental, decision.Action)
}
```

- [ ] **Step 2: Run the planner tests and verify they fail**

Run: `go test ./internal/service -run 'TestOpenAIResponsesContinuationPlanner' -count=1`
Expected: FAIL with undefined planner types/functions.

- [ ] **Step 3: Add planner action types and input contract**

```go
type openAIResponsesContinuationAction string

const (
	openAIResponsesContinuationActionHotWSIncremental        openAIResponsesContinuationAction = "hot_ws_incremental"
	openAIResponsesContinuationActionColdHTTPIncremental     openAIResponsesContinuationAction = "cold_http_incremental"
	openAIResponsesContinuationActionFullRebuild             openAIResponsesContinuationAction = "full_rebuild"
	openAIResponsesContinuationActionRejectUnsafeContinuation openAIResponsesContinuationAction = "reject_unsafe"
)

type openAIResponsesContinuationInput struct {
	AccountType                  string
	StoreDisabled                bool
	PreviousResponseID           string
	LiveWSAvailable              bool
	StickyAccountHit             bool
	StickyConnHit                bool
	SessionWindowAvailable       bool
	HasFunctionCallOutput        bool
	HasSafeToolReplayWindow      bool
	DurableContinuationAllowed   bool
	PersistedContinuationAvailable bool
}
```

- [ ] **Step 4: Implement the minimal primary-lane planner**

```go
func (openAIResponsesContinuationPlanner) Plan(in openAIResponsesContinuationInput) openAIResponsesContinuationDecision {
	if in.HasFunctionCallOutput && !in.HasSafeToolReplayWindow {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionRejectUnsafeContinuation}
	}
	if strings.TrimSpace(in.PreviousResponseID) == "" {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionFullRebuild}
	}
	if in.LiveWSAvailable && in.StickyAccountHit && in.StickyConnHit {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionHotWSIncremental}
	}
	if in.StoreDisabled {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionFullRebuild}
	}
	if in.DurableContinuationAllowed && in.PersistedContinuationAvailable && in.StickyAccountHit && in.SessionWindowAvailable {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionColdHTTPIncremental}
	}
	return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionFullRebuild}
}
```

- [ ] **Step 5: Re-run the planner tests and verify they pass**

Run: `go test ./internal/service -run 'TestOpenAIResponsesContinuationPlanner' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_responses_continuation_planner.go backend/internal/service/openai_responses_continuation_planner_test.go
git commit -m "feat(openai): add continuation planner contracts"
```

### Task 2: Shared Rebuild Window Before Any HTTP Continuation Expansion

**Files:**
- Create: `backend/internal/service/openai_responses_session_window.go`
- Create: `backend/internal/service/openai_responses_session_window_test.go`
- Modify: `backend/internal/service/openai_ws_state_store.go`
- Modify: `backend/internal/service/openai_ws_state_store_test.go`
- Modify: `backend/internal/service/gateway_service.go`
- Modify: `backend/internal/repository/gateway_cache.go`
- Modify: `backend/internal/repository/gateway_cache_integration_test.go`

- [ ] **Step 1: Write failing tests for shared rebuild-window persistence**

```go
func TestOpenAIResponsesSessionWindowStore_BindAndGetViaSharedCache(t *testing.T) {
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	window := openAIResponsesSessionWindow{
		LatestResponseID: "resp_latest_1",
		PromptCacheKey:   "pcache_1",
		ReplayInputRaw:   []byte(`[{"type":"input_text","text":"history"}]`),
	}
	require.NoError(t, store.BindSessionWindow(context.Background(), 7, 11, "session_hash_1", window, time.Hour))

	got, ok := store.GetSessionWindow(context.Background(), 7, 11, "session_hash_1")
	require.True(t, ok)
	require.Equal(t, "resp_latest_1", got.LatestResponseID)
	require.Equal(t, "pcache_1", got.PromptCacheKey)
}
```

- [ ] **Step 2: Run the rebuild-window tests and verify they fail**

Run: `go test ./internal/service -run 'TestOpenAIResponsesSessionWindowStore_BindAndGetViaSharedCache' -count=1`
Expected: FAIL with undefined rebuild-window methods/types.

- [ ] **Step 3: Extend cache/store contracts for a shared rebuild window**

```go
type openAIResponsesSessionWindow struct {
	LatestResponseID string
	PromptCacheKey   string
	ReplayInputRaw   []byte
	Compacted        bool
}

type GatewayCache interface {
	// existing methods...
	GetOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string) ([]byte, error)
	SetOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string, payload []byte, ttl time.Duration) error
	DeleteOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string) error
}
```

- [ ] **Step 4: Implement local hot cache plus shared cache persistence**

```go
func (s *defaultOpenAIWSStateStore) BindSessionWindow(ctx context.Context, groupID int64, apiKeyID int64, sessionHash string, window openAIResponsesSessionWindow, ttl time.Duration) error {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash) + ":window"
	encoded, err := json.Marshal(window)
	if err != nil {
		return err
	}
	s.sessionWindowMu.Lock()
	s.sessionWindow[key] = openAIResponsesSessionWindowBinding{window: window, expiresAt: time.Now().Add(normalizeOpenAIWSTTL(ttl))}
	s.sessionWindowMu.Unlock()
	if s.cache == nil {
		return nil
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	return s.cache.SetOpenAIResponsesSessionWindow(cacheCtx, groupID, key, encoded, ttl)
}
```

- [ ] **Step 5: Re-run rebuild-window tests and verify they pass**

Run: `go test ./internal/service -run 'TestOpenAIResponsesSessionWindowStore_BindAndGetViaSharedCache' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_responses_session_window.go backend/internal/service/openai_responses_session_window_test.go backend/internal/service/openai_ws_state_store.go backend/internal/service/openai_ws_state_store_test.go backend/internal/service/gateway_service.go backend/internal/repository/gateway_cache.go backend/internal/repository/gateway_cache_integration_test.go
git commit -m "feat(openai): add shared rebuild window store"
```

### Task 3: Convert previous_response_not_found To Full Rebuild On The Primary Lane

**Files:**
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_ws_forwarder.go`
- Modify: `backend/internal/service/openai_ws_protocol_forward_test.go`
- Modify: `backend/internal/service/openai_ws_forwarder_ingress_session_test.go`

- [ ] **Step 1: Write failing tests that require rebuild instead of delete-anchor retry**

```go
func TestOpenAIGatewayService_OAuthStoreFalsePreviousResponseNotFoundUsesRebuildWindow(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"previous_response_not_found","message":"missing anchor"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"resp_rebuilt_ok","usage":{"input_tokens":1,"output_tokens":1}}`)),
			},
		},
	}

	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	store := svc.getOpenAIWSStateStore()
	require.NoError(t, store.BindSessionWindow(context.Background(), 7, 11, "session_hash_1", openAIResponsesSessionWindow{
		LatestResponseID: "resp_missing_prev",
		ReplayInputRaw:   []byte(`[{"type":"input_text","text":"history"},{"type":"input_text","text":"followup"}]`),
	}, time.Hour))

	body := []byte(`{"model":"gpt-5.1","store":false,"previous_response_id":"resp_missing_prev","input":[{"type":"input_text","text":"followup"}]}`)
	_, err := svc.Forward(context.Background(), c, &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, body)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(upstream.bodies[1], "previous_response_id").Exists())
	require.Equal(t, "history", gjson.GetBytes(upstream.bodies[1], "input.0.text").String())
}
```

- [ ] **Step 2: Run the recovery tests and verify they fail**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_OAuthStoreFalsePreviousResponseNotFoundUsesRebuildWindow' -count=1`
Expected: FAIL because current recovery still centers on dropping the anchor and retrying the current turn.

- [ ] **Step 3: Replace delete-anchor retry with planner-driven rebuild on the primary lane**

```go
decision := planner.Plan(openAIResponsesContinuationInput{
	AccountType:            account.Type,
	StoreDisabled:          true,
	PreviousResponseID:     previousResponseID,
	SessionWindowAvailable: sessionWindowExists,
	RecoveryReason:         "previous_response_not_found",
})
if decision.Action == openAIResponsesContinuationActionFullRebuild {
	delete(reqBody, "previous_response_id")
	reqBody["store"] = false
	setReplayInputFromSessionWindow(reqBody, sessionWindow)
}
```

- [ ] **Step 4: Re-run the recovery tests and verify they pass**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_OAuthStoreFalsePreviousResponseNotFoundUsesRebuildWindow' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/openai_gateway_service.go backend/internal/service/openai_ws_forwarder.go backend/internal/service/openai_ws_protocol_forward_test.go backend/internal/service/openai_ws_forwarder_ingress_session_test.go
git commit -m "feat(openai): rebuild after missing continuation anchor"
```

### Task 4: Safety Gates Before Any Durable Lane Work

**Files:**
- Modify: `backend/internal/service/openai_tool_continuation.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_codex_transform.go`
- Modify: `backend/internal/service/openai_oauth_passthrough_test.go`

- [ ] **Step 1: Write failing tests for unsafe tool continuation rejection**

```go
func TestOpenAIGatewayService_OAuthStoreFalseRejectsUnsafeFunctionCallOutputWithoutReplayWindow(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &httpUpstreamRecorder{}}
	body := []byte(`{"model":"gpt-5.1","store":false,"previous_response_id":"resp_tool_prev","input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)
	result, err := svc.Forward(context.Background(), c, &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, body)
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusConflict, rec.Code)
}
```

- [ ] **Step 2: Write failing tests for reasoning include preservation**

```go
func TestApplyCodexOAuthTransform_PreservesReasoningEncryptedContentForPrimaryLane(t *testing.T) {
	reqBody := map[string]any{
		"model": "gpt-5.1",
		"store": false,
		"reasoning": map[string]any{"effort": "medium"},
	}
	applyCodexOAuthTransform(reqBody, false, false)
	include, _ := reqBody["include"].([]any)
	require.Contains(t, include, "reasoning.encrypted_content")
}
```

- [ ] **Step 3: Run the safety tests and verify they fail**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_OAuthStoreFalseRejectsUnsafeFunctionCallOutputWithoutReplayWindow|TestApplyCodexOAuthTransform_PreservesReasoningEncryptedContentForPrimaryLane' -count=1`
Expected: FAIL because the primary-lane safety gates are not yet unified.

- [ ] **Step 4: Implement the safety gates**

```go
signals := AnalyzeToolContinuationSignals(reqBody)
if signals.HasFunctionCallOutput && !hasSafeReplayWindow(reqBody, sessionWindow) {
	return nil, newUnsafeToolContinuationHTTPError()
}
ensureCodexReasoningInclude(reqBody)
```

- [ ] **Step 5: Re-run the safety tests and verify they pass**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_OAuthStoreFalseRejectsUnsafeFunctionCallOutputWithoutReplayWindow|TestApplyCodexOAuthTransform_PreservesReasoningEncryptedContentForPrimaryLane' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_tool_continuation.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_codex_transform.go backend/internal/service/openai_oauth_passthrough_test.go
git commit -m "feat(openai): add primary-lane continuation safety gates"
```

### Task 5: Optional Durable Continuation Lane

**Files:**
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_ws_forwarder.go`
- Modify: `backend/internal/service/openai_ws_protocol_forward_test.go`
- Modify: `backend/internal/service/openai_ws_account_sticky_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for the explicit durable lane**

```go
func TestOpenAIGatewayService_Forward_DurableHTTPContinuationPreservesPreviousResponseID(t *testing.T) {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"resp_http_ok","usage":{"input_tokens":1,"output_tokens":1}}`)),
		},
	}

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.HTTPIncrementalContinuationEnabled = true
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	body := []byte(`{"model":"gpt-5.1","store":true,"previous_response_id":"resp_http_prev","input":[{"type":"input_text","text":"hello"}]}`)
	_, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.Equal(t, "resp_http_prev", gjson.GetBytes(upstream.lastBody, "previous_response_id").String())
}
```

- [ ] **Step 2: Run the durable-lane tests and verify they fail**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_Forward_DurableHTTPContinuationPreservesPreviousResponseID' -count=1`
Expected: FAIL because durable HTTP continuation is not yet wired.

- [ ] **Step 3: Gate HTTP continuation and sticky behind explicit durable-lane flags**

```go
if wsDecision.Transport != OpenAIUpstreamTransportResponsesWebsocketV2 &&
	gjson.GetBytes(body, "previous_response_id").Exists() &&
	!s.openAIHTTPIncrementalContinuationEnabled() {
	markPatchDelete("previous_response_id")
}
```

```go
transport := s.getOpenAIWSProtocolResolver().Resolve(account).Transport
if transport != OpenAIUpstreamTransportResponsesWebsocketV2 &&
	!(transport == OpenAIUpstreamTransportHTTPSSE && s.openAIHTTPIncrementalStickyEnabled()) {
	return nil, nil
}
```

- [ ] **Step 4: Re-run durable-lane tests and verify they pass**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_Forward_DurableHTTPContinuationPreservesPreviousResponseID' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/openai_gateway_service.go backend/internal/service/openai_ws_forwarder.go backend/internal/service/openai_ws_protocol_forward_test.go backend/internal/service/openai_ws_account_sticky_test.go backend/internal/config/config.go backend/internal/config/config_test.go
git commit -m "feat(openai): gate durable HTTP continuation"
```

### Task 6: Performance Phase

**Files:**
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_ws_forwarder.go`
- Modify: `backend/internal/service/openai_codex_transform.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for the performance levers**

```go
func TestApplyCodexOAuthTransform_PreservesPromptCacheKeyAndReasoningInclude(t *testing.T) {
	reqBody := map[string]any{
		"model":            "gpt-5.1",
		"store":            false,
		"prompt_cache_key": "pcache_1",
		"reasoning":        map[string]any{"effort": "medium"},
	}
	applyCodexOAuthTransform(reqBody, false, false)
	require.Equal(t, "pcache_1", reqBody["prompt_cache_key"])
}
```

- [ ] **Step 2: Add and verify config for performance policy**

Run: `go test ./internal/config -run 'TestConfig_OpenAIWSContinuationFlagsDefaultOff' -count=1`
Expected: FAIL until config fields/defaults exist for:
- `rebuild_fallback_enabled`
- `http_incremental_continuation_enabled`
- `http_incremental_sticky_enabled`

- [ ] **Step 3: Wire the performance levers**

```go
// planner/transport path:
// - preserve prompt_cache_key
// - support priority service_tier policy
// - keep WS warm-up / prewarm knobs in the continuation lane
// - surface TTFT / throughput telemetry fields
```

- [ ] **Step 4: Run the focused verification bundle**

Run: `go test ./internal/service -run 'TestOpenAIResponsesContinuationPlanner|TestOpenAIResponsesSessionWindowStore_BindAndGetViaSharedCache|TestOpenAIGatewayService_OAuthStoreFalsePreviousResponseNotFoundUsesRebuildWindow|TestOpenAIGatewayService_OAuthStoreFalseRejectsUnsafeFunctionCallOutputWithoutReplayWindow|TestOpenAIGatewayService_Forward_DurableHTTPContinuationPreservesPreviousResponseID' -count=1`
Expected: PASS

- [ ] **Step 5: Run config tests and diff hygiene**

Run: `go test ./internal/config -count=1`
Expected: PASS

Run: `git diff --check`
Expected: no output

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_gateway_service.go backend/internal/service/openai_ws_forwarder.go backend/internal/service/openai_codex_transform.go backend/internal/config/config.go backend/internal/config/config_test.go
git commit -m "feat(openai): add continuation performance policy"
```
