package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

func TestKiroGatewayService_ResolveTLSProfile_UsesKiroResolver(t *testing.T) {
	svc := &KiroGatewayService{
		tlsFPProfileSvc: &TLSFingerprintProfileService{
			localCache: map[int64]*model.TLSFingerprintProfile{
				7: {ID: 7, Name: "Kiro Gateway Profile"},
			},
		},
	}

	profile := svc.resolveTLSProfile(&Account{
		ID:       88,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": int64(7),
		},
	})

	require.NotNil(t, profile)
	require.Equal(t, "Kiro Gateway Profile", profile.Name)
}

func TestKiroGatewayService_BuildRequest_DoesNotForceConnectionClose(t *testing.T) {
	svc := &KiroGatewayService{}

	req, err := svc.buildRequest(context.Background(), &Account{
		ID:       90,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
		},
	}, []byte(`{}`), "access-token", nil)

	require.NoError(t, err)
	require.Empty(t, req.Header.Values("Connection"))
}

func TestKiroGatewayService_BuildRequest_UsesRuntimeSettings(t *testing.T) {
	svc := &KiroGatewayService{}

	req, err := svc.buildRequest(context.Background(), &Account{
		ID:       90,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"refresh_token": "refresh-token",
			"machine_id":    "machine-id",
		},
	}, []byte(`{}`), "access-token", &KiroRuntimeSettings{
		KiroVersion:   "0.11.0",
		KiroCommit:    "commit-123",
		SystemVersion: "linux#6.8.0",
		NodeVersion:   "22.22.0",
	})

	require.NoError(t, err)
	require.Contains(t, req.Header.Get("x-amz-user-agent"), "KiroIDE-0.11.0-")
	require.Contains(t, req.Header.Get("User-Agent"), "os/linux#6.8.0")
	require.Contains(t, req.Header.Get("User-Agent"), "md/nodejs#22.22.0")
	require.Equal(t, "commit-123", req.Header.Get("x-amzn-kiro-commit"))
}

func TestKiroGatewayService_ResolveAccessToken_UsesAPIKeyForAPIKeyAccounts(t *testing.T) {
	svc := &KiroGatewayService{}

	token, err := svc.resolveAccessToken(context.Background(), &Account{
		ID:       91,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "kiro-api-key", token)
}

func TestKiroGatewayService_RefreshFakeCacheStrategyFlushesExistingEntries(t *testing.T) {
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}
	plan := &kiropkg.FakeCachePlan{
		IndependentKey:             "kiro:test:strategy:independent",
		IndependentCacheableTokens: 64,
	}
	initial := &KiroRuntimeSettings{
		CacheHitRateScale:       95,
		CacheMinBlockTokens:     0,
		CacheIndependentTTLSecs: 3600,
		CachePrefixTTLSecs:      300,
	}

	svc.refreshFakeCacheStrategy(initial)
	plan.CacheStrategy = svc.fakeCacheStrategy
	plan.CacheStrategyGeneration = svc.fakeCacheGen
	svc.commitFakeCachePlan(plan, initial)
	_, found := svc.fakeCache.Get(plan.IndependentKey)
	require.True(t, found)

	svc.refreshFakeCacheStrategy(&KiroRuntimeSettings{
		CacheHitRateScale:       50,
		CacheMinBlockTokens:     0,
		CacheIndependentTTLSecs: 3600,
		CachePrefixTTLSecs:      300,
	})
	_, found = svc.fakeCache.Get(plan.IndependentKey)
	require.False(t, found)
}

func TestKiroGatewayService_CommitFakeCachePlanSkipsStaleStrategyGeneration(t *testing.T) {
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}
	oldSettings := &KiroRuntimeSettings{
		CacheHitRateScale:       95,
		CacheMinBlockTokens:     0,
		CacheIndependentTTLSecs: 3600,
		CachePrefixTTLSecs:      300,
	}
	newSettings := &KiroRuntimeSettings{
		CacheHitRateScale:       50,
		CacheMinBlockTokens:     0,
		CacheIndependentTTLSecs: 3600,
		CachePrefixTTLSecs:      300,
	}
	plan := &kiropkg.FakeCachePlan{
		CurrentKey:              "kiro:test:stale-generation",
		CurrentCacheableTokens:  64,
		CacheStrategy:           kiroFakeCacheStrategy(oldSettings),
		CacheStrategyGeneration: 1,
	}

	svc.refreshFakeCacheStrategy(oldSettings)
	svc.refreshFakeCacheStrategy(newSettings)
	svc.commitFakeCachePlan(plan, oldSettings)

	_, found := svc.fakeCache.Get(plan.CurrentKey)
	require.False(t, found, "old in-flight requests must not repopulate cache after strategy changes")
}

func TestKiroGatewayService_CommitFakeCachePlanSkipsConcurrentStaleGenerationAfterFlush(t *testing.T) {
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}
	oldSettings := &KiroRuntimeSettings{
		CacheHitRateScale:       95,
		CacheMinBlockTokens:     0,
		CacheIndependentTTLSecs: 3600,
		CachePrefixTTLSecs:      300,
	}
	newSettings := &KiroRuntimeSettings{
		CacheHitRateScale:       50,
		CacheMinBlockTokens:     0,
		CacheIndependentTTLSecs: 3600,
		CachePrefixTTLSecs:      300,
	}
	plan := &kiropkg.FakeCachePlan{
		CurrentKey:             "kiro:test:concurrent-stale-generation",
		CurrentCacheableTokens: 64,
	}

	svc.refreshFakeCacheStrategy(oldSettings)
	plan.CacheStrategy = svc.fakeCacheStrategy
	plan.CacheStrategyGeneration = svc.fakeCacheGen

	startCommit := make(chan struct{})
	commitDone := make(chan struct{})
	go func() {
		defer close(commitDone)
		<-startCommit
		svc.commitFakeCachePlan(plan, oldSettings)
	}()

	svc.refreshFakeCacheStrategy(newSettings)
	close(startCommit)
	<-commitDone

	_, found := svc.fakeCache.Get(plan.CurrentKey)
	require.False(t, found, "old in-flight requests must not repopulate cache after a concurrent flush")
}

func TestKiroGatewayService_ForwardSnapshotsFakeCacheHitBeforeUpstreamRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sessionID := "123e4567-e89b-12d3-a456-426614174000"
	longFirstPrompt := strings.Repeat("first prompt token ", 1200)
	longSecondPrompt := strings.Repeat("second prompt token ", 1200)
	body := []byte(fmt.Sprintf(`{
		"model":"claude-sonnet-4-5-20250929",
		"metadata":{"user_id":"user_x_account__session_%s"},
		"messages":[
			{"role":"user","content":%q},
			{"role":"assistant","content":"ok"},
			{"role":"user","content":%q}
		],
		"max_tokens":128
	}`, sessionID, longFirstPrompt, longSecondPrompt))
	plan, err := kiropkg.BuildFakeCachePlan(body, 77, "claude-sonnet-4-5-20250929")
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.NotEmpty(t, plan.PreviousPrefixKey)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}
	upstream := &kiroMutatingHTTPUpstream{
		beforeReturn: func() {
			// If Forward calculated fake-cache hits after DoWithTLS, this request would
			// incorrectly count as a cache read.
			svc.fakeCache.Set(plan.PreviousPrefixKey, struct{}{}, time.Minute)
		},
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "assistantResponseEvent",
			}, map[string]any{"content": "hello from kiro"}))),
		},
	}
	svc.httpUpstream = upstream
	account := &Account{
		ID:       77,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}
	parsed := &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body:  body,
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, upstream.calls)
	require.Zero(t, result.Usage.CacheReadInputTokens)
	require.Greater(t, result.Usage.CacheCreationInputTokens, 0)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestKiroGatewayService_ForwardCountTokens_RejectsUnsupportedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{}

	err := svc.ForwardCountTokens(context.Background(), c, &Account{
		ID:       101,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, &ParsedRequest{
		Model: "claude-unknown-9-9",
		Body: []byte(`{
			"model":"claude-unknown-9-9",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`),
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "unsupported kiro model")
}

func TestMapKiroModel_MatchesClaudeCodeAliasesAgainstConfiguredKiroModels(t *testing.T) {
	account := &Account{
		ID:       103,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-sonnet-4.6":    "claude-sonnet-4.6",
				"claude-sonnet-4.6-1m": "claude-sonnet-4.6-1m",
				"claude-opus-4.7":      "claude-opus-4.7",
				"claude-opus-4.7-1m":   "claude-opus-4.7-1m",
			},
		},
	}

	require.Equal(t, "claude-sonnet-4.6", mapKiroModel(account, "claude-sonnet-4-6"))
	require.Equal(t, "claude-sonnet-4.6-1m", mapKiroModel(account, "claude-sonnet-4-6-1m"))
	require.Equal(t, "claude-opus-4.7-1m", mapKiroModel(account, "claude-opus-4-7-1m"))
}

func TestResolveKiroRequestedModelForRequest_DefaultSimulationKeepsMappedModel(t *testing.T) {
	account := &Account{ID: 104, Platform: PlatformKiro, Type: AccountTypeOAuth}

	model, err := resolveKiroRequestedModelForRequest(account, &ParsedRequest{
		Model:           "claude-sonnet-4-5",
		ThinkingEnabled: true,
		OutputEffort:    "high",
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4-5", model)
	require.Equal(t, "claude-sonnet-4.5", kiropkg.MapModel(model))
}

func TestResolveKiroRequestedModelForRequest_PreservesOneMillionMappedModel(t *testing.T) {
	account := &Account{ID: 105, Platform: PlatformKiro, Type: AccountTypeOAuth}

	model, err := resolveKiroRequestedModelForRequest(account, &ParsedRequest{
		Model:           "claude-sonnet-4-5-20250929-1m",
		ThinkingEnabled: true,
		OutputEffort:    "xhigh",
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4-5-20250929-1m", model)
	require.Equal(t, "claude-sonnet-4.5-1m", kiropkg.MapModel(model))
}

func TestResolveKiroRequestedModelForRequest_LowEffortDoesNotUseThinkingVariant(t *testing.T) {
	account := &Account{ID: 106, Platform: PlatformKiro, Type: AccountTypeOAuth}

	for _, effort := range []string{"low", "minimal"} {
		t.Run(effort, func(t *testing.T) {
			model, err := resolveKiroRequestedModelForRequest(account, &ParsedRequest{
				Model:           "claude-sonnet-4-5",
				ThinkingEnabled: true,
				OutputEffort:    effort,
			}, nil)

			require.NoError(t, err)
			require.Equal(t, "claude-sonnet-4-5", model)
		})
	}
}

func TestResolveKiroRequestedModelForRequest_KeepsAccountMappedModelForThinkingRequests(t *testing.T) {
	account := &Account{
		ID:       107,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-sonnet-4-5": "claude-sonnet-4.5",
			},
		},
	}

	model, err := resolveKiroRequestedModelForRequest(account, &ParsedRequest{
		Model:           "claude-sonnet-4-5",
		ThinkingEnabled: true,
		OutputEffort:    "medium",
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4.5", model)
	require.Equal(t, "claude-sonnet-4.5", kiropkg.MapModel(model))
}

func TestResolveKiroRequestedModelForRequest_DoesNotRequireThinkingVariantInAccountMapping(t *testing.T) {
	account := &Account{
		ID:       109,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-sonnet-4-5": "claude-sonnet-4.5",
			},
		},
	}

	model, err := resolveKiroRequestedModelForRequest(account, &ParsedRequest{
		Model:           "claude-sonnet-4-5",
		ThinkingEnabled: true,
		OutputEffort:    "medium",
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4.5", model)
}

func TestResolveKiroRequestedModelForRequest_SimulateModeDoesNotUseThinkingVariant(t *testing.T) {
	account := &Account{ID: 108, Platform: PlatformKiro, Type: AccountTypeOAuth}

	model, err := resolveKiroRequestedModelForRequest(account, &ParsedRequest{
		Model:           "claude-sonnet-4-5",
		ThinkingEnabled: true,
		OutputEffort:    "high",
	}, &KiroRuntimeSettings{ThinkingMode: KiroThinkingModeSimulate, ThinkingEffortThreshold: "medium"})

	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4-5", model)
}

func TestShouldUseKiroFreeThinkingPath_ThinkingModes(t *testing.T) {
	account := &Account{
		ID:       109,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"subscription_type": "free",
		},
	}
	parsed := &ParsedRequest{ThinkingEnabled: true}

	tests := []struct {
		name     string
		mode     string
		expected bool
	}{
		{name: "simulate", mode: KiroThinkingModeSimulate, expected: false},
		{name: "model", mode: KiroThinkingModeModel, expected: false},
		{name: "model_and_simulate", mode: KiroThinkingModeModelAndSimulate, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, shouldUseKiroFreeThinkingPath(account, parsed, &KiroRuntimeSettings{ThinkingMode: tt.mode}))
		})
	}
}

func TestPrepareKiroConvertedRequest_PromotesLargeContextToOneMillionModelWithoutDroppingBillingBaseline(t *testing.T) {
	account := &Account{ID: 110, Platform: PlatformKiro, Type: AccountTypeOAuth}
	body := []byte(fmt.Sprintf(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"%s"}]
	}`, strings.Repeat("x ", 181500)))

	converted, billedInputTokens, err := prepareKiroConvertedRequest(account, &ParsedRequest{
		Model: "claude-sonnet-4-6",
		Body:  body,
	}, nil)
	require.NoError(t, err)
	require.Greater(t, billedInputTokens, kiroStandardContextBudgetTokens)
	require.Equal(t, "claude-sonnet-4.6-1m", converted.Model)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(converted.Body, &payload))
	state := payload["conversationState"].(map[string]any)
	current := state["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	require.Equal(t, "claude-sonnet-4.6-1m", current["modelId"])
}

func TestRenderKiroThinkingSimulation_UsesConfiguredTemplateAndEffortThreshold(t *testing.T) {
	settings := &KiroRuntimeSettings{
		ThinkingMode:               KiroThinkingModeSimulate,
		ThinkingEffortThreshold:    "high",
		ThinkingSimulationTemplate: "think {effort} {model} {upstream_model} {detail}",
	}

	low := renderKiroThinkingSimulation(&ParsedRequest{
		Model:           "claude-sonnet-4-5",
		ThinkingEnabled: true,
		OutputEffort:    "medium",
	}, &kiropkg.ConvertResult{Model: "claude-sonnet-4.5"}, settings)
	high := renderKiroThinkingSimulation(&ParsedRequest{
		Model:           "claude-sonnet-4-5",
		ThinkingEnabled: true,
		OutputEffort:    "high",
	}, &kiropkg.ConvertResult{Model: "claude-sonnet-4.5"}, settings)

	require.Empty(t, low)
	require.Contains(t, high, "think high claude-sonnet-4-5 claude-sonnet-4.5")
	require.Contains(t, high, "failure modes")
}

func TestKiroGatewayService_Forward_FreeSimulateModeKeepsSingleRequestAndSimulatedThinking(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "assistantResponseEvent",
			}, map[string]any{"content": "final answer"}))),
		},
	}
	svc := &KiroGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:       111,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":           "kiro-api-key",
			"subscription_type": "free",
		},
	}
	parsed := &ParsedRequest{
		Model:           "claude-sonnet-4-5-20250929",
		ThinkingEnabled: true,
		OutputEffort:    "high",
		Body: []byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"thinking":{"type":"enabled","budget_tokens":5000},
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, upstream.calls, "simulate mode should not take the free two-request path")
	require.Contains(t, rec.Body.String(), `"type":"thinking"`)
	require.Contains(t, rec.Body.String(), `Using Kiro simulated thinking with high effort for claude-sonnet-4-5-20250929`)
	require.Contains(t, rec.Body.String(), `"text":"final answer"`)
}

func TestKiroGatewayService_Forward_ModelAndSimulateFreeThinkingFallsBackToSimulatedThinking(t *testing.T) {
	gin.SetMode(gin.TestMode)

	kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
	kiroRuntimeSettingsSF.Forget("kiro_runtime")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{}
	upstream.doFunc = func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
		if accountID == 0 {
			t.Fatalf("unexpected zero account id")
		}
		if req == nil {
			t.Fatalf("expected upstream request")
		}
		switch upstream.calls {
		case 1:
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"message":"thinking step failed"}`)),
			}, nil
		case 2:
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
					":message-type": "event",
					":event-type":   "assistantResponseEvent",
				}, map[string]any{"content": "final answer"}))),
			}, nil
		default:
			t.Fatalf("unexpected upstream call %d", upstream.calls)
			return nil, nil
		}
	}
	settingSvc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyKiroThinkingMode:               KiroThinkingModeModelAndSimulate,
			SettingKeyKiroThinkingEffortThreshold:    "medium",
			SettingKeyKiroThinkingSimulationTemplate: "fallback {effort} {model} {upstream_model}",
		},
	}, &config.Config{})
	svc := &KiroGatewayService{
		httpUpstream:   upstream,
		settingService: settingSvc,
	}
	account := &Account{
		ID:       112,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":           "kiro-api-key",
			"subscription_type": "free",
		},
	}
	parsed := &ParsedRequest{
		Model:           "claude-sonnet-4-5-20250929",
		ThinkingEnabled: true,
		OutputEffort:    "high",
		Body: []byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"thinking":{"type":"enabled","budget_tokens":5000},
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, upstream.calls, "model_and_simulate mode should still try the free thinking preflight")
	require.Contains(t, rec.Body.String(), `"type":"thinking"`)
	require.Contains(t, rec.Body.String(), `fallback high claude-sonnet-4-5-20250929 claude-sonnet-4.5`)
	require.Contains(t, rec.Body.String(), `"text":"final answer"`)
}

func TestKiroThinkingBodyBuilders_RemoveThinkingDependentContextStrategies(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4-5-20250929",
		"thinking":{"type":"enabled","budget_tokens":5000},
		"context_management":{
			"edits":[
				{"type":"clear_thinking_20251015","keep":"all"},
				{"type":"keep_recent_messages_20251015","count":3}
			]
		},
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
	}`)

	tests := []struct {
		name  string
		build func([]byte) []byte
	}{
		{
			name: "free_thinking_body",
			build: func(in []byte) []byte {
				return buildKiroFreeThinkingBody(in, "free prompt")
			},
		},
		{
			name:  "strip_thinking_field",
			build: stripKiroThinkingField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.build(body)
			require.NotNil(t, out)

			var payload map[string]any
			require.NoError(t, json.Unmarshal(out, &payload))
			_, hasThinking := payload["thinking"]
			require.False(t, hasThinking)

			contextManagement, ok := payload["context_management"].(map[string]any)
			require.True(t, ok)
			edits, ok := contextManagement["edits"].([]any)
			require.True(t, ok)
			require.Len(t, edits, 1)

			edit, ok := edits[0].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "keep_recent_messages_20251015", edit["type"])
		})
	}
}

func TestKiroGatewayService_ForwardCountTokens_RejectsInvalidConversationShape(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{}

	err := svc.ForwardCountTokens(context.Background(), c, &Account{
		ID:       102,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, &ParsedRequest{
		Model: "claude-sonnet-4-6",
		Body: []byte(`{
			"model":"claude-sonnet-4-6",
			"messages":[{"role":"assistant","content":[{"type":"text","text":"hello"}]}]
		}`),
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "empty messages")
}

func TestKiroGatewayService_ForwardCountTokens_UsesForwardValidationWithLocalEstimate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{}
	expectedInputTokens := kiropkg.AccurateTokenCount("hello from count tokens")

	err := svc.ForwardCountTokens(context.Background(), c, &Account{
		ID:       103,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body: []byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello from count tokens"}]}]
		}`),
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 4, expectedInputTokens, "fixture should lock the tiktoken-backed contract, not the legacy heuristic")
	require.JSONEq(t, fmt.Sprintf(`{"input_tokens":%d}`, expectedInputTokens), rec.Body.String())
}

func TestKiroGatewayService_ForwardNonStream_ExceptionDoesNotCommitFakeCache(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}
	fakeCachePlan := &kiropkg.FakeCachePlan{
		CurrentKey:             "kiro:test:nonstream",
		CurrentCacheableTokens: 12,
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "partial output"}),
		buildKiroTestFrame(t, map[string]string{
			":event-type":     "exception",
			":exception-type": "RuntimeException",
		}, map[string]any{"message": "upstream failed"}),
	}, nil)

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 1, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4"},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		fakeCachePlan,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Error(t, err)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Body.String())
	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "http_error", events[0].Kind)
	require.Equal(t, http.StatusBadGateway, events[0].UpstreamStatusCode)
	require.Contains(t, events[0].Message, "kiro upstream returned exception frame")
	_, found := svc.fakeCache.Get(fakeCachePlan.CurrentKey)
	require.False(t, found, "exception responses must not commit fake cache")
}

func TestKiroGatewayService_ForwardStream_ExceptionDoesNotCommitFakeCacheOrEmitFinalStop(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}
	fakeCachePlan := &kiropkg.FakeCachePlan{
		CurrentKey:             "kiro:test:stream",
		CurrentCacheableTokens: 12,
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "partial output"}),
		buildKiroTestFrame(t, map[string]string{
			":event-type":     "exception",
			":exception-type": "RuntimeException",
		}, map[string]any{"message": "upstream failed"}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 1, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		fakeCachePlan,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "event: message_start")
	require.Contains(t, rec.Body.String(), "event: error")
	require.NotContains(t, rec.Body.String(), "event: message_delta")
	require.NotContains(t, rec.Body.String(), "event: message_stop")
	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "http_error", events[0].Kind)
	require.Equal(t, http.StatusBadGateway, events[0].UpstreamStatusCode)
	require.Contains(t, events[0].Message, "kiro upstream returned exception frame")
	_, found := svc.fakeCache.Get(fakeCachePlan.CurrentKey)
	require.False(t, found, "exception streams must not commit fake cache")
}

func TestKiroGatewayService_Forward_HTTPErrorRecordsOpsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"X-Amzn-Requestid": []string{"kiro-request-123"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":"invalid_request","message":"selected model is not available for this account"}`)),
		},
	}
	svc := &KiroGatewayService{
		httpUpstream: upstream,
	}

	result, err := svc.Forward(
		context.Background(),
		c,
		&Account{
			ID:       7,
			Name:     "Kiro Gateway",
			Platform: PlatformKiro,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key": "kiro-api-key",
			},
		},
		&ParsedRequest{
			Model: "claude-sonnet-4-5-20250929",
			Body: []byte(`{
				"model":"claude-sonnet-4-5-20250929",
				"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
			}`),
		},
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "Kiro upstream returned 400: invalid_request: selected model is not available for this account")
	require.ErrorContains(t, err, "Kiro upstream returned 400: invalid_request: selected model is not available for this account")
	statusCodeValue, ok := c.Get(OpsUpstreamStatusCodeKey)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, statusCodeValue)
	messageValue, ok := c.Get(OpsUpstreamErrorMessageKey)
	require.True(t, ok)
	require.Equal(t, "Kiro upstream returned 400: invalid_request: selected model is not available for this account", messageValue)
	detailValue, ok := c.Get(OpsUpstreamErrorDetailKey)
	require.True(t, ok)
	require.Equal(t, "invalid_request: selected model is not available for this account", detailValue)
	upstreamModelValue, ok := c.Get(OpsUpstreamModelKey)
	require.True(t, ok)
	require.Equal(t, "claude-sonnet-4.5", upstreamModelValue)
	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "http_error", events[0].Kind)
	require.Equal(t, http.StatusBadRequest, events[0].UpstreamStatusCode)
	require.Equal(t, "kiro-request-123", events[0].UpstreamRequestID)
	require.Equal(t, "https://q.us-east-1.amazonaws.com/generateAssistantResponse", events[0].UpstreamURL)
	require.Equal(t, "Kiro upstream returned 400: invalid_request: selected model is not available for this account", events[0].Message)
	require.Equal(t, "invalid_request: selected model is not available for this account", events[0].Detail)
	sentBody, readErr := io.ReadAll(upstream.req.Body)
	require.NoError(t, readErr)
	require.Equal(t, string(sentBody), events[0].UpstreamRequestBody)
	require.Contains(t, events[0].UpstreamRequestBody, `"modelId":"claude-sonnet-4.5"`)
	require.NotContains(t, events[0].UpstreamRequestBody, `"model":"claude-sonnet-4-5-20250929"`)
}

func TestKiroGatewayService_ForwardStream_PreStartExceptionReturnsFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{}

	body := buildKiroTestFrame(t, map[string]string{
		":event-type":     "exception",
		":exception-type": "RuntimeException",
	}, map[string]any{"message": "upstream failed before stream"})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 11, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Error(t, err)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "event: message_start")
}

func TestKiroGatewayService_ForwardNonStream_IncompleteFrameDoesNotCommitFakeCache(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}
	fakeCachePlan := &kiropkg.FakeCachePlan{
		CurrentKey:             "kiro:test:nonstream:truncated",
		CurrentCacheableTokens: 12,
	}

	frame := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "assistantResponseEvent",
	}, map[string]any{"content": "partial output"})
	body := frame[:len(frame)-3]

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 2, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4"},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		fakeCachePlan,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	_, found := svc.fakeCache.Get(fakeCachePlan.CurrentKey)
	require.False(t, found, "truncated non-stream responses must not commit fake cache")
}

func TestKiroGatewayService_ForwardStream_IncompleteFrameDoesNotCommitFakeCacheOrEmitFinalStop(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}
	fakeCachePlan := &kiropkg.FakeCachePlan{
		CurrentKey:             "kiro:test:stream:truncated",
		CurrentCacheableTokens: 12,
	}

	frame := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "assistantResponseEvent",
	}, map[string]any{"content": "partial output"})
	body := frame[:len(frame)-5]

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 2, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		fakeCachePlan,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "event: message_start")
	require.NotContains(t, rec.Body.String(), "event: message_stop")
	_, found := svc.fakeCache.Get(fakeCachePlan.CurrentKey)
	require.False(t, found, "truncated stream responses must not commit fake cache")
}

func TestKiroGatewayService_ForwardNonStream_EmptyBodyFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 3, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(nil)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4"},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "Failed to decode Kiro response")
}

func TestKiroGatewayService_ForwardStream_EmptyBodyFailsWithoutFinalEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 4, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(nil)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "event: error")
	require.NotContains(t, rec.Body.String(), "event: message_start")
	require.NotContains(t, rec.Body.String(), "event: message_delta")
	require.NotContains(t, rec.Body.String(), "event: message_stop")
}

func TestKiroGatewayService_ForwardStream_ContextOnlyBodyFailsWithoutStartingStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "contextUsageEvent",
	}, map[string]any{"contextUsagePercentage": 98})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 4, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.6"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, rec.Body.String(), "event: error")
	require.Contains(t, rec.Body.String(), "context usage reached 98%")
	require.NotContains(t, rec.Body.String(), "event: message_start")
}

func TestKiroGatewayService_ForwardStream_DoesNotBillContextUsagePercentageAsInputTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}

	body := append(
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "contextUsageEvent",
		}, map[string]any{"contextUsagePercentage": 30}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "ok"})...,
	)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 4, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.6"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 32, result.Usage.InputTokens)
	require.Contains(t, rec.Body.String(), `"input_tokens":32`)
	require.NotContains(t, rec.Body.String(), `"input_tokens":300000`)
}

func TestParseKiroFrame_RejectsOversizedHeaderLength(t *testing.T) {
	frame := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "assistantResponseEvent",
	}, map[string]any{"content": "ok"})
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(frame)))
	binary.BigEndian.PutUint32(frame[8:12], crc32.ChecksumIEEE(frame[:8]))

	parsed, consumed, ok, err := parseKiroFrame(frame)
	require.Error(t, err)
	require.Nil(t, parsed)
	require.Zero(t, consumed)
	require.False(t, ok)
}

func TestParseKiroFrame_RejectsOversizedFrameLength(t *testing.T) {
	frame := make([]byte, kiroPreludeSize)
	binary.BigEndian.PutUint32(frame[0:4], uint32(kiroMaxBodySize+1))
	binary.BigEndian.PutUint32(frame[4:8], 0)
	binary.BigEndian.PutUint32(frame[8:12], crc32.ChecksumIEEE(frame[:8]))

	parsed, consumed, ok, err := parseKiroFrame(frame)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kiro frame exceeded limit")
	require.Nil(t, parsed)
	require.Zero(t, consumed)
	require.False(t, ok)
}

func TestKiroGatewayService_ForwardStream_ToolFirstUsesMonotonicBlockIndexes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-1", "name": "search", "input": `{"q":"a"}`, "stop": true}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "done"}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 5, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"index":0`)
	require.Contains(t, rec.Body.String(), `"index":1`)
	require.NotContains(t, rec.Body.String(), `"index":-1`)
	require.Less(t, strings.Index(rec.Body.String(), `"index":0`), strings.Index(rec.Body.String(), `"index":1`))
}

func TestKiroGatewayService_ForwardStream_ToolOnlyCountsOutputTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-1", "name": "search", "input": `{"q":"a"}`, "stop": true})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 5, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Greater(t, result.Usage.OutputTokens, 0)
	require.Contains(t, rec.Body.String(), fmt.Sprintf(`"output_tokens":%d`, result.Usage.OutputTokens))
}

func TestKiroGatewayService_ForwardStream_TextToolTextClosesBlocksInOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "hello"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-1", "name": "search", "input": `{"q":"a"}`, "stop": true}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "world"}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 6, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	output := rec.Body.String()
	textStart0 := strings.Index(output, `event: content_block_start`+"\n"+`data: {"content_block":{"text":"","type":"text"},"index":0,"type":"content_block_start"}`)
	textStop0 := strings.Index(output, `data: {"index":0,"type":"content_block_stop"}`)
	toolStart1 := strings.Index(output, `data: {"content_block":{"id":"tool-1","input":{},"name":"search","type":"tool_use"},"index":1,"type":"content_block_start"}`)
	toolStop1 := strings.Index(output, `data: {"index":1,"type":"content_block_stop"}`)
	textStart2 := strings.Index(output, `data: {"content_block":{"text":"","type":"text"},"index":2,"type":"content_block_start"}`)
	textStop2 := strings.LastIndex(output, `data: {"index":2,"type":"content_block_stop"}`)

	require.NotEqual(t, -1, textStart0)
	require.NotEqual(t, -1, textStop0)
	require.NotEqual(t, -1, toolStart1)
	require.NotEqual(t, -1, toolStop1)
	require.NotEqual(t, -1, textStart2)
	require.NotEqual(t, -1, textStop2)
	require.Less(t, textStart0, textStop0)
	require.Less(t, textStop0, toolStart1)
	require.Less(t, toolStart1, toolStop1)
	require.Less(t, toolStop1, textStart2)
	require.Less(t, textStart2, textStop2)
}

func TestKiroGatewayService_ForwardStream_PreservesWhitespaceInContentAndInputDeltas(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "  keep text delta spaces  "}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{
			"toolUseId": " tool-1 ",
			"name":      " search ",
			"input":     " {\"q\":\" value with spaces \"} ",
			"stop":      true,
		}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 6, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	var textDelta, inputDelta string
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &payload); err != nil {
			continue
		}
		delta, _ := payload["delta"].(map[string]any)
		deltaType, _ := delta["type"].(string)
		switch deltaType {
		case "text_delta":
			textDelta, _ = delta["text"].(string)
		case "input_json_delta":
			inputDelta, _ = delta["partial_json"].(string)
		}
	}

	require.Equal(t, "  keep text delta spaces  ", textDelta)
	require.Equal(t, " {\"q\":\" value with spaces \"} ", inputDelta)
	require.Contains(t, rec.Body.String(), `"id":"tool-1"`, "tool_use id should still be normalized as a control field")
	require.Contains(t, rec.Body.String(), `"name":"search"`, "tool_use name should still be normalized as a control field")
}

func buildKiroTestFrame(t *testing.T, headers map[string]string, payload map[string]any) []byte {
	t.Helper()

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	headerNames := make([]string, 0, len(headers))
	for name := range headers {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)

	headerBytes := make([]byte, 0, len(headers)*16)
	for _, name := range headerNames {
		value := headers[name]
		headerBytes = append(headerBytes, byte(len(name)))
		headerBytes = append(headerBytes, name...)
		headerBytes = append(headerBytes, 7)

		var valueLen [2]byte
		binary.BigEndian.PutUint16(valueLen[:], uint16(len(value)))
		headerBytes = append(headerBytes, valueLen[:]...)
		headerBytes = append(headerBytes, value...)
	}

	totalLength := kiroPreludeSize + len(headerBytes) + len(payloadBytes) + 4
	frame := make([]byte, totalLength)
	binary.BigEndian.PutUint32(frame[0:4], uint32(totalLength))
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(headerBytes)))
	binary.BigEndian.PutUint32(frame[8:12], crc32.ChecksumIEEE(frame[:8]))
	copy(frame[kiroPreludeSize:], headerBytes)
	copy(frame[kiroPreludeSize+len(headerBytes):], payloadBytes)
	binary.BigEndian.PutUint32(frame[totalLength-4:], crc32.ChecksumIEEE(frame[:totalLength-4]))
	return frame
}

type kiroMutatingHTTPUpstream struct {
	calls        int
	beforeReturn func()
	resp         *http.Response
	err          error
}

func (u *kiroMutatingHTTPUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *kiroMutatingHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.calls++
	if u.beforeReturn != nil {
		u.beforeReturn()
	}
	return u.resp, u.err
}
