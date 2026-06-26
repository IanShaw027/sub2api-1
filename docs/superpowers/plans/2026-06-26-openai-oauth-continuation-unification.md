# OpenAI OAuth Continuation Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Unify OpenAI OAuth continuation so WS hot continuation, HTTP cold continuation, and full rebuild fallback share one planner, preserve sticky session/account behavior, and maximize safe incremental sending.

**Architecture:** Introduce a dedicated continuation planner and session rebuild window, then migrate transport-specific code to consume planner outputs instead of ad-hoc `previous_response_id` rules. Roll out in three phases behind feature flags so HTTP continuation correctness lands before sticky semantics and full rebuild fallback.

**Tech Stack:** Go, Gin, existing OpenAI WS state store, existing Responses ingress normalization, existing OpenAI WS/HTTP protocol tests.

---

### Task 1: Planner Red Tests

**Files:**
- Create: `backend/internal/service/openai_responses_continuation_planner.go`
- Create: `backend/internal/service/openai_responses_continuation_planner_test.go`

- [ ] **Step 1: Write the failing planner action tests**

```go
func TestOpenAIResponsesContinuationPlanner_PrefersHotWSIncremental(t *testing.T) {
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
	require.True(t, decision.PreservePreviousResponseID)
}

func TestOpenAIResponsesContinuationPlanner_UsesColdHTTPIncrementalWithoutLiveWS(t *testing.T) {
	planner := openAIResponsesContinuationPlanner{}
	decision := planner.Plan(openAIResponsesContinuationInput{
		AccountType:            AccountTypeOAuth,
		StoreDisabled:          true,
		PreviousResponseID:     "resp_cold_1",
		LiveWSAvailable:        false,
		StickyAccountHit:       true,
		SessionWindowAvailable: true,
	})
	require.Equal(t, openAIResponsesContinuationActionColdHTTPIncremental, decision.Action)
}

func TestOpenAIResponsesContinuationPlanner_RejectsUnsafeToolContinuation(t *testing.T) {
	planner := openAIResponsesContinuationPlanner{}
	decision := planner.Plan(openAIResponsesContinuationInput{
		AccountType:             AccountTypeOAuth,
		StoreDisabled:           true,
		PreviousResponseID:      "resp_tool_1",
		HasFunctionCallOutput:   true,
		HasSafeToolReplayWindow: false,
	})
	require.Equal(t, openAIResponsesContinuationActionRejectUnsafeContinuation, decision.Action)
}
```

- [ ] **Step 2: Run planner tests to verify they fail**

Run: `go test ./internal/service -run 'TestOpenAIResponsesContinuationPlanner' -count=1`
Expected: FAIL with undefined planner types/functions.

- [ ] **Step 3: Add minimal planner types and action enum**

```go
type openAIResponsesContinuationAction string

const (
	openAIResponsesContinuationActionHotWSIncremental      openAIResponsesContinuationAction = "hot_ws_incremental"
	openAIResponsesContinuationActionColdHTTPIncremental   openAIResponsesContinuationAction = "cold_http_incremental"
	openAIResponsesContinuationActionFullRebuild           openAIResponsesContinuationAction = "full_rebuild"
	openAIResponsesContinuationActionRejectUnsafeContinuation openAIResponsesContinuationAction = "reject_unsafe"
)

type openAIResponsesContinuationInput struct {
	AccountType             string
	StoreDisabled           bool
	PreviousResponseID      string
	LiveWSAvailable         bool
	StickyAccountHit        bool
	StickyConnHit           bool
	SessionWindowAvailable  bool
	HasFunctionCallOutput   bool
	HasSafeToolReplayWindow bool
}

type openAIResponsesContinuationDecision struct {
	Action                    openAIResponsesContinuationAction
	PreservePreviousResponseID bool
}
```

- [ ] **Step 4: Implement the minimal planner logic to make tests pass**

```go
func (openAIResponsesContinuationPlanner) Plan(in openAIResponsesContinuationInput) openAIResponsesContinuationDecision {
	if in.HasFunctionCallOutput && !in.HasSafeToolReplayWindow {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionRejectUnsafeContinuation}
	}
	if strings.TrimSpace(in.PreviousResponseID) == "" {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionFullRebuild}
	}
	if in.LiveWSAvailable && in.StickyAccountHit && in.StickyConnHit {
		return openAIResponsesContinuationDecision{
			Action:                     openAIResponsesContinuationActionHotWSIncremental,
			PreservePreviousResponseID: true,
		}
	}
	if in.StickyAccountHit && in.SessionWindowAvailable {
		return openAIResponsesContinuationDecision{
			Action:                     openAIResponsesContinuationActionColdHTTPIncremental,
			PreservePreviousResponseID: true,
		}
	}
	return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionFullRebuild}
}
```

- [ ] **Step 5: Re-run planner tests and verify they pass**

Run: `go test ./internal/service -run 'TestOpenAIResponsesContinuationPlanner' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_responses_continuation_planner.go backend/internal/service/openai_responses_continuation_planner_test.go
git commit -m "feat(openai): add continuation planner skeleton"
```

### Task 2: HTTP Continuation Correctness

**Files:**
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_ws_forwarder.go`
- Modify: `backend/internal/service/openai_ws_protocol_forward_test.go`
- Modify: `backend/internal/service/openai_ws_account_sticky_test.go`

- [ ] **Step 1: Write a failing HTTP continuation transport test**

```go
func TestOpenAIGatewayService_Forward_HTTPContinuationPreservesPreviousResponseID(t *testing.T) {
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

	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	body := []byte(`{"model":"gpt-5.1","store":false,"previous_response_id":"resp_http_prev","input":[{"type":"input_text","text":"hello"}]}`)
	_, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.Equal(t, "resp_http_prev", gjson.GetBytes(upstream.lastBody, "previous_response_id").String())
}
```

- [ ] **Step 2: Write a failing sticky-selection test for force-http**

```go
func TestOpenAIGatewayService_SelectAccountByPreviousResponseID_ForceHTTPStillUsesSticky(t *testing.T) {
	ctx := context.Background()
	groupID := int64(23)
	account := Account{
		ID:       11,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
		Schedulable: true,
		Extra: map[string]any{
			"openai_ws_force_http": true,
		},
	}
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	svc := &OpenAIGatewayService{
		accountRepo:        stubOpenAIAccountRepo{accounts: []Account{account}},
		cache:              cache,
		cfg:                newOpenAIWSV2TestConfig(),
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{}),
		openaiWSStateStore: store,
	}
	require.NoError(t, store.BindResponseAccount(ctx, groupID, 0, "resp_force_http_prev", account.ID, time.Hour))

	selection, err := svc.SelectAccountByPreviousResponseID(ctx, &groupID, 0, "resp_force_http_prev", "gpt-5.1", nil, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, account.ID, selection.Account.ID)
}
```

- [ ] **Step 3: Run targeted tests and confirm they fail**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_Forward_HTTPContinuationPreservesPreviousResponseID|TestOpenAIGatewayService_SelectAccountByPreviousResponseID_ForceHTTPStillUsesSticky' -count=1`
Expected: FAIL because HTTP path still strips `previous_response_id` and force-http still ignores sticky selection.

- [ ] **Step 4: Remove unconditional HTTP previous_response_id stripping behind a new flag**

```go
if wsDecision.Transport != OpenAIUpstreamTransportResponsesWebsocketV2 &&
	gjson.GetBytes(body, "previous_response_id").Exists() &&
	!s.openAIHTTPIncrementalContinuationEnabled() {
	markPatchDelete("previous_response_id")
}
```

- [ ] **Step 5: Allow sticky account selection for forced HTTP continuation**

```go
transport := s.getOpenAIWSProtocolResolver().Resolve(account).Transport
if transport != OpenAIUpstreamTransportResponsesWebsocketV2 &&
	!(transport == OpenAIUpstreamTransportHTTPSSE && s.openAIHTTPIncrementalStickyEnabled()) {
	return nil, nil
}
```

- [ ] **Step 6: Re-run targeted tests and verify they pass**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_Forward_HTTPContinuationPreservesPreviousResponseID|TestOpenAIGatewayService_SelectAccountByPreviousResponseID_ForceHTTPStillUsesSticky' -count=1`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add backend/internal/service/openai_gateway_service.go backend/internal/service/openai_ws_forwarder.go backend/internal/service/openai_ws_protocol_forward_test.go backend/internal/service/openai_ws_account_sticky_test.go
git commit -m "feat(openai): enable gated HTTP continuation"
```

### Task 3: Session Rebuild Window

**Files:**
- Create: `backend/internal/service/openai_responses_session_window.go`
- Create: `backend/internal/service/openai_responses_session_window_test.go`
- Modify: `backend/internal/service/openai_ws_state_store.go`
- Modify: `backend/internal/service/openai_ws_state_store_test.go`

- [ ] **Step 1: Write a failing session-window persistence test**

```go
func TestOpenAIResponsesSessionWindowStore_BindAndGet(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	window := openAIResponsesSessionWindow{
		LatestResponseID: "resp_latest_1",
		PromptCacheKey:   "pcache_1",
		ReplayInputRaw:   []byte(`[{"type":"input_text","text":"hello"}]`),
	}
	require.NoError(t, store.BindSessionWindow(7, 11, "session_hash_1", window, time.Hour))

	got, ok := store.GetSessionWindow(7, 11, "session_hash_1")
	require.True(t, ok)
	require.Equal(t, "resp_latest_1", got.LatestResponseID)
	require.Equal(t, "pcache_1", got.PromptCacheKey)
}
```

- [ ] **Step 2: Run the store tests to verify they fail**

Run: `go test ./internal/service -run 'TestOpenAIResponsesSessionWindowStore_BindAndGet' -count=1`
Expected: FAIL with undefined session-window store methods/types.

- [ ] **Step 3: Add the session window type and store methods**

```go
type openAIResponsesSessionWindow struct {
	LatestResponseID string
	PromptCacheKey   string
	ReplayInputRaw   []byte
	Compacted        bool
}

type OpenAIWSStateStore interface {
	// existing methods...
	BindSessionWindow(groupID int64, apiKeyID int64, sessionHash string, window openAIResponsesSessionWindow, ttl time.Duration) error
	GetSessionWindow(groupID int64, apiKeyID int64, sessionHash string) (openAIResponsesSessionWindow, bool)
	DeleteSessionWindow(groupID int64, apiKeyID int64, sessionHash string)
}
```

- [ ] **Step 4: Implement minimal in-memory storage**

```go
type openAIResponsesSessionWindowBinding struct {
	window    openAIResponsesSessionWindow
	expiresAt time.Time
}

func (s *defaultOpenAIWSStateStore) BindSessionWindow(groupID int64, apiKeyID int64, sessionHash string, window openAIResponsesSessionWindow, ttl time.Duration) error {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash) + ":window"
	s.sessionWindowMu.Lock()
	s.sessionWindow[key] = openAIResponsesSessionWindowBinding{window: window, expiresAt: time.Now().Add(normalizeOpenAIWSTTL(ttl))}
	s.sessionWindowMu.Unlock()
	return nil
}
```

- [ ] **Step 5: Re-run session-window tests and verify they pass**

Run: `go test ./internal/service -run 'TestOpenAIResponsesSessionWindowStore_BindAndGet' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_responses_session_window.go backend/internal/service/openai_responses_session_window_test.go backend/internal/service/openai_ws_state_store.go backend/internal/service/openai_ws_state_store_test.go
git commit -m "feat(openai): add session rebuild window store"
```

### Task 4: previous_response_not_found Full-Rebuild Ladder

**Files:**
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_ws_forwarder.go`
- Modify: `backend/internal/service/openai_ws_protocol_forward_test.go`

- [ ] **Step 1: Write a failing recovery test that expects full rebuild**

```go
func TestOpenAIGatewayService_Forward_HTTPContinuationPreviousResponseNotFoundRebuildsFromSessionWindow(t *testing.T) {
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
	require.NoError(t, store.BindSessionWindow(7, 11, "session_hash_1", openAIResponsesSessionWindow{
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

- [ ] **Step 2: Run the recovery test and confirm it fails**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_Forward_HTTPContinuationPreviousResponseNotFoundRebuildsFromSessionWindow' -count=1`
Expected: FAIL because HTTP recovery still lacks rebuild-window handling.

- [ ] **Step 3: Replace delete-anchor retry with planner-driven rebuild**

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

- [ ] **Step 4: Re-run the recovery test and verify it passes**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_Forward_HTTPContinuationPreviousResponseNotFoundRebuildsFromSessionWindow' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/openai_gateway_service.go backend/internal/service/openai_ws_forwarder.go backend/internal/service/openai_ws_protocol_forward_test.go
git commit -m "feat(openai): rebuild continuation after missing anchor"
```

### Task 5: Tool/Reasoning Safety Gates

**Files:**
- Modify: `backend/internal/service/openai_tool_continuation.go`
- Modify: `backend/internal/service/openai_gateway_service.go`
- Modify: `backend/internal/service/openai_codex_transform.go`
- Modify: `backend/internal/service/openai_oauth_passthrough_test.go`

- [ ] **Step 1: Write failing tests for unsafe tool continuation rejection**

```go
func TestOpenAIGatewayService_HTTPContinuationRejectsUnsafeFunctionCallOutputWithoutReplayWindow(t *testing.T) {
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
func TestApplyCodexOAuthTransform_PreservesReasoningEncryptedContentForContinuationLane(t *testing.T) {
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

- [ ] **Step 3: Run the safety tests and confirm they fail**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_HTTPContinuationRejectsUnsafeFunctionCallOutputWithoutReplayWindow|TestApplyCodexOAuthTransform_PreservesReasoningEncryptedContentForContinuationLane' -count=1`
Expected: FAIL because HTTP lane safety gating is incomplete.

- [ ] **Step 4: Implement the safety gates**

```go
signals := AnalyzeToolContinuationSignals(reqBody)
if signals.HasFunctionCallOutput && !hasSafeReplayWindow(reqBody, sessionWindow) {
	return nil, newUnsafeToolContinuationHTTPError()
}
ensureCodexReasoningInclude(reqBody)
```

- [ ] **Step 5: Re-run the safety tests and verify they pass**

Run: `go test ./internal/service -run 'TestOpenAIGatewayService_HTTPContinuationRejectsUnsafeFunctionCallOutputWithoutReplayWindow|TestApplyCodexOAuthTransform_PreservesReasoningEncryptedContentForContinuationLane' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/service/openai_tool_continuation.go backend/internal/service/openai_gateway_service.go backend/internal/service/openai_codex_transform.go backend/internal/service/openai_oauth_passthrough_test.go
git commit -m "feat(openai): add continuation safety gates"
```

### Task 6: Flags, Metrics, and Verification

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/internal/service/openai_ws_forwarder.go`
- Modify: `backend/internal/service/openai_gateway_service.go`

- [ ] **Step 1: Add failing config tests for new flags**

```go
func TestConfig_OpenAIWSContinuationFlagsDefaultOff(t *testing.T) {
	cfg := DefaultConfig()
	require.False(t, cfg.Gateway.OpenAIWS.HTTPIncrementalContinuationEnabled)
	require.False(t, cfg.Gateway.OpenAIWS.HTTPIncrementalStickyEnabled)
	require.False(t, cfg.Gateway.OpenAIWS.RebuildFallbackEnabled)
}
```

- [ ] **Step 2: Run config tests and confirm they fail**

Run: `go test ./internal/config -run 'TestConfig_OpenAIWSContinuationFlagsDefaultOff' -count=1`
Expected: FAIL with missing config fields/defaults.

- [ ] **Step 3: Add config fields and observability labels**

```go
type OpenAIWSConfig struct {
	HTTPIncrementalContinuationEnabled bool `mapstructure:"http_incremental_continuation_enabled"`
	HTTPIncrementalStickyEnabled       bool `mapstructure:"http_incremental_sticky_enabled"`
	RebuildFallbackEnabled             bool `mapstructure:"rebuild_fallback_enabled"`
}

logOpenAIWSModeInfo(
	"continuation_decision account_id=%d action=%s reason=%s sticky_account_hit=%v sticky_conn_hit=%v session_window_hit=%v",
	account.ID,
	decision.Action,
	decision.Reason,
	decision.StickyAccountHit,
	decision.StickyConnHit,
	decision.SessionWindowHit,
)
```

- [ ] **Step 4: Run the focused verification bundle**

Run: `go test ./internal/service -run 'TestOpenAIResponsesContinuationPlanner|TestOpenAIGatewayService_Forward_HTTPContinuationPreservesPreviousResponseID|TestOpenAIGatewayService_SelectAccountByPreviousResponseID_ForceHTTPStillUsesSticky|TestOpenAIGatewayService_Forward_HTTPContinuationPreviousResponseNotFoundRebuildsFromSessionWindow|TestOpenAIGatewayService_HTTPContinuationRejectsUnsafeFunctionCallOutputWithoutReplayWindow' -count=1`
Expected: PASS

- [ ] **Step 5: Run config tests and diff hygiene**

Run: `go test ./internal/config -run 'TestConfig_OpenAIWSContinuationFlagsDefaultOff' -count=1`
Expected: PASS

Run: `git diff --check`
Expected: no output

- [ ] **Step 6: Commit**

```bash
git add backend/internal/config/config.go backend/internal/config/config_test.go backend/internal/service/openai_ws_forwarder.go backend/internal/service/openai_gateway_service.go
git commit -m "feat(openai): add continuation rollout flags and telemetry"
```
