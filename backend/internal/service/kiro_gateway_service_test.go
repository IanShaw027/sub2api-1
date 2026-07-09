package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
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
	"github.com/Wei-Shaw/sub2api/internal/pkg/webfetch"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"github.com/gin-gonic/gin"
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

func TestKiroFakeCachePlanBodyPrefersForwardBody(t *testing.T) {
	parsed := &ParsedRequest{Body: NewRequestBodyRef([]byte(`{"metadata":{"user_id":"original"}}`))}
	meta := &kiroPreparedRequestMeta{ForwardBody: []byte(`{"metadata":{"user_id":"compacted"}}`)}

	require.Equal(t, string(meta.ForwardBody), string(kiroFakeCachePlanBody(parsed, meta)))
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

func TestBuildKiroToolUseBlock_RepairsAskUserQuestionMissingQuestion(t *testing.T) {
	state := &kiroToolState{
		ToolUseID: "toolu_ask",
		Name:      "AskUserQuestion",
	}
	_, _ = state.InputBuilder.WriteString(`{"questions":[{"header":"QQ 数据清理范围选哪个？","id":"qq_cleanup","options":[{"label":"连聊天记录清 13.6G","description":"清理 QQ 聊天记录"}]}]}`)

	block, ok := buildKiroToolUseBlock(state, nil)
	require.True(t, ok)

	input, ok := block["input"].(map[string]any)
	require.True(t, ok, "input has type %T", block["input"])
	questions, ok := input["questions"].([]any)
	require.True(t, ok, "questions has type %T", input["questions"])
	require.Len(t, questions, 1)
	question, ok := questions[0].(map[string]any)
	require.True(t, ok, "questions[0] has type %T", questions[0])
	require.Equal(t, "QQ 数据清理范围选哪个？", question["question"])
	require.Equal(t, "QQ 数据清理范围选哪个？", question["header"])
}

func TestBuildKiroToolUseBlock_InvalidJSONInputDoesNotSilentlyFallbackToEmptyObject(t *testing.T) {
	state := &kiroToolState{
		ToolUseID: "toolu_bad",
		Name:      "Bash",
		Started:   true,
	}
	_, _ = state.InputBuilder.WriteString(`{"command":`)

	block, ok := buildKiroToolUseBlock(state, nil)

	require.False(t, ok)
	require.Nil(t, block)
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

func TestKiroGatewayService_ResolveAccessToken_RejectsNilAccount(t *testing.T) {
	svc := &KiroGatewayService{}

	token, err := svc.resolveAccessToken(context.Background(), nil)

	require.Empty(t, token)
	require.EqualError(t, err, "account is required")
}

func TestKiroGatewayService_RefreshFakeCacheStrategyFlushesExistingEntries(t *testing.T) {
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
	svc.fakeCache.wait()
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
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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

	svc.fakeCache.wait()
	_, found := svc.fakeCache.Get(plan.CurrentKey)
	require.False(t, found, "old in-flight requests must not repopulate cache after strategy changes")
}

func TestKiroGatewayService_CommitFakeCachePlanSkipsConcurrentStaleGenerationAfterFlush(t *testing.T) {
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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

	svc.fakeCache.wait()
	_, found := svc.fakeCache.Get(plan.CurrentKey)
	require.False(t, found, "old in-flight requests must not repopulate cache after a concurrent flush")
}

func TestKiroGatewayService_CommitFakeCachePlanPersistsEffectiveProgress(t *testing.T) {
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}
	settings := &KiroRuntimeSettings{
		CacheHitRateScale:       95,
		CacheMinBlockTokens:     0,
		CacheIndependentTTLSecs: 3600,
		CachePrefixTTLSecs:      300,
	}
	plan := &kiropkg.FakeCachePlan{
		SessionProgressKey:           "kiro:test:session-progress",
		CurrentPrefixKey:             "kiro:test:prefix:current",
		CurrentCacheableTokens:       1200,
		CurrentPrefixCacheableTokens: 1200,
	}

	svc.refreshFakeCacheStrategy(settings)
	plan.CacheStrategy = svc.fakeCacheStrategy
	plan.CacheStrategyGeneration = svc.fakeCacheGen
	usage := resolveKiroFakeCacheUsage(plan, kiropkg.FakeCacheHitState{}, 1200, settings)
	require.Equal(t, 0, usage.CacheReadInputTokens)
	require.Equal(t, 1200, usage.CacheCreationInputTokens)
	require.Equal(t, 0, usage.InputTokens)

	svc.commitFakeCachePlan(plan, settings)
	svc.fakeCache.wait()
	progress, found := svc.fakeCache.Get(plan.SessionProgressKey)
	require.True(t, found)
	require.Equal(t, usage.CacheReadInputTokens+usage.CacheCreationInputTokens, progress)
}

func TestKiroGatewayService_FakeCacheSessionProgressCarriesCheckpointAcrossTurns(t *testing.T) {
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}
	account := &Account{ID: 77, Platform: PlatformKiro, Type: AccountTypeOAuth}
	settings := &KiroRuntimeSettings{
		CacheHitRateScale:       100,
		CacheMinBlockTokens:     0,
		CacheIndependentTTLSecs: 3600,
		CachePrefixTTLSecs:      300,
	}
	sessionID := "123e4567-e89b-12d3-a456-426614174000"
	firstBody := []byte(fmt.Sprintf(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_%s"},
		"system":"Use concise answers.",
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"first prompt"},
				{"type":"text","text":"cache checkpoint one","cache_control":{"type":"ephemeral"}}
			]}
		]
	}`, sessionID))
	secondBody := []byte(fmt.Sprintf(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_%s"},
		"system":"Use concise answers.",
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"first prompt"},
				{"type":"text","text":"cache checkpoint one"}
			]},
			{"role":"assistant","content":[
				{"type":"tool_use","id":"toolu-different","name":"Bash","input":{"command":"printf ok"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu-different","content":"ok","cache_control":{"type":"ephemeral"}}
			]}
		]
	}`, sessionID))

	firstPlan, firstHit := svc.prepareFakeCachePlan(account, &ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef(firstBody), UserID: 1, APIKeyID: 2}, nil, settings)
	require.NotNil(t, firstPlan)
	require.Zero(t, firstHit.CheckpointTokens)
	firstCurrent := firstPlan.CurrentCheckpointTokens()
	firstCacheable := firstPlan.CurrentCacheableTokens
	require.Positive(t, firstCurrent)
	require.GreaterOrEqual(t, firstCacheable, firstCurrent)

	svc.commitFakeCachePlan(firstPlan, settings)
	svc.fakeCache.wait()

	secondPlan, secondHit := svc.prepareFakeCachePlan(account, &ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef(secondBody), UserID: 1, APIKeyID: 2}, nil, settings)
	require.NotNil(t, secondPlan)
	require.Greater(t, secondPlan.CurrentCheckpointTokens(), firstCurrent)
	// Turn 1 committed its effective cache progress to SessionProgress; turn 2
	// also finds a shorter checkpoint. SessionProgress should supplement that
	// shorter hit basis used for cache_read.
	require.Equal(t, firstCacheable, secondHit.EffectiveCachedTokens)

	totalInputTokens := secondPlan.CurrentCheckpointTokens() + 10
	usage := resolveKiroFakeCacheUsage(secondPlan, secondHit, totalInputTokens, settings)
	require.Greater(t, secondHit.CheckpointTokens, 0)
	require.Greater(t, secondHit.EffectiveCachedTokens, secondHit.CheckpointTokens)
	idealRead := secondHit.EffectiveCachedTokens
	require.Equal(t, idealRead, usage.CacheReadInputTokens)
	require.Equal(t, totalInputTokens, usage.InputTokens+usage.CacheCreationInputTokens+usage.CacheReadInputTokens)
	require.LessOrEqual(t, usage.InputTokens, 10)
}

func TestKiroGatewayService_ForwardSnapshotsFakeCacheHitBeforeUpstreamRequest(t *testing.T) {
	setGinTestMode()

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
	plan, err := kiropkg.BuildFakeCachePlan(body, kiropkg.FakeCacheScope{AccountID: 77, UserID: 1, APIKeyID: 2}, "claude-sonnet-4-5-20250929")
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.NotEmpty(t, plan.PreviousPrefixKey)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
		Model:    "claude-sonnet-4-5-20250929",
		Body:     NewRequestBodyRef(body),
		UserID:   1,
		APIKeyID: 2,
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, upstream.calls)
	require.Zero(t, result.Usage.CacheReadInputTokens)
	require.Greater(t, result.Usage.CacheCreationInputTokens, 0)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestKiroGatewayService_PrepareFakeCachePlanReusesAcrossAccountsForSameUserAndAPIKey(t *testing.T) {
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}
	settings := &KiroRuntimeSettings{
		CacheHitRateScale:       100,
		CacheMinBlockTokens:     0,
		CacheIndependentTTLSecs: 3600,
		CachePrefixTTLSecs:      300,
	}
	firstBody := []byte(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174099"},
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"},
			{"role":"user","content":"please continue"}
		]
	}`)
	secondBody := []byte(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174099"},
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"},
			{"role":"user","content":"please continue"},
			{"role":"user","content":"one more step"}
		]
	}`)

	firstPlan, firstHit := svc.prepareFakeCachePlan(&Account{ID: 42, Platform: PlatformKiro, Type: AccountTypeOAuth}, &ParsedRequest{
		Model:    "claude-sonnet-4",
		Body:     NewRequestBodyRef(firstBody),
		UserID:   1,
		APIKeyID: 2,
	}, nil, settings)
	require.NotNil(t, firstPlan)
	require.False(t, firstHit.Prefix)
	svc.commitFakeCachePlan(firstPlan, settings)
	svc.fakeCache.wait()

	secondPlan, secondHit := svc.prepareFakeCachePlan(&Account{ID: 99, Platform: PlatformKiro, Type: AccountTypeOAuth}, &ParsedRequest{
		Model:    "claude-sonnet-4",
		Body:     NewRequestBodyRef(secondBody),
		UserID:   1,
		APIKeyID: 2,
	}, nil, settings)
	require.NotNil(t, secondPlan)
	require.Equal(t, firstPlan.CurrentPrefixKey, secondPlan.PreviousPrefixKey)
	require.True(t, secondHit.Prefix)
}

func TestKiroGatewayService_ForwardCountTokens_RejectsUnsupportedModel(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{}

	err := svc.ForwardCountTokens(context.Background(), c, &Account{
		ID:       101,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, &ParsedRequest{
		Model: "claude-unknown-9-9",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-unknown-9-9",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
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
	require.Equal(t, "claude-sonnet-4.6", mapKiroModel(account, "claude-sonnet-4-6-1m"))
	require.Equal(t, "claude-opus-4.7", mapKiroModel(account, "claude-opus-4-7-1m"))
	require.Equal(t, "claude-opus-4.7", mapKiroModel(account, "claude-opus-4.7[1m]"))

	account.Credentials["model_mapping"] = map[string]any{
		"claude-opus-*":   "claude-opus-4.6",
		"claude-opus-4.7": "claude-opus-4.7",
	}
	require.Equal(t, "claude-opus-4.7", mapKiroModel(account, "claude-opus-4-7"))
	require.Equal(t, "claude-opus-4.6", mapKiroModel(account, "claude-opus-4-6"))
}

func TestMapKiroModel_RejectsCrossFamilyFallbackMappings(t *testing.T) {
	account := &Account{
		ID:       103,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-opus-4-6": "claude-sonnet-4.6",
			},
		},
	}

	require.Empty(t, mapKiroModel(account, "claude-opus-4-6"))
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

func TestResolveKiroRequestedModelWithRouting_UsesPlatformDefaultConfig(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformDefaultAccountModelConfig: `{
				"kiro": {
					"model_mapping": {"claude-sonnet-4-5": "claude-sonnet-4.6"}
				}
			}`,
		},
	}, &config.Config{})
	account := &Account{ID: 110, Platform: PlatformKiro, Type: AccountTypeOAuth}

	model, err := resolveKiroRequestedModelWithRouting(context.Background(), svc, account, "claude-sonnet-4-5")

	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4.6", model)
}

func TestResolveKiroRequestedModelForRequest_PreservesOneMillionMappedModel(t *testing.T) {
	account := &Account{ID: 105, Platform: PlatformKiro, Type: AccountTypeOAuth}

	model, err := resolveKiroRequestedModelForRequest(account, &ParsedRequest{
		Model:           "claude-sonnet-4-5-20250929-1m",
		ThinkingEnabled: true,
		OutputEffort:    "xhigh",
	}, nil)

	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4-5-20250929", model)
	require.Equal(t, "claude-sonnet-4.5", kiropkg.MapModel(model))
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

func TestResolveKiroRequestedModelForRequest_RejectsThinkingSuffixModel(t *testing.T) {
	account := &Account{ID: 108, Platform: PlatformKiro, Type: AccountTypeOAuth}

	_, err := resolveKiroRequestedModelForRequest(account, &ParsedRequest{
		Model:           "claude-sonnet-4-6-thinking",
		ThinkingEnabled: true,
		OutputEffort:    "high",
	}, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported kiro model")
}

func TestPrepareKiroConvertedRequest_PromotesLargeContextToOneMillionModelWithoutDroppingBillingBaseline(t *testing.T) {
	account := &Account{ID: 110, Platform: PlatformKiro, Type: AccountTypeOAuth}
	body := []byte(fmt.Sprintf(`{
		"model":"claude-sonnet-4-6",
		"messages":[{"role":"user","content":"%s"}]
	}`, strings.Repeat("x ", 181500)))

	converted, billedInputTokens, err := prepareKiroConvertedRequest(account, &ParsedRequest{
		Model: "claude-sonnet-4-6",
		Body:  NewRequestBodyRef(body),
	}, nil)
	require.NoError(t, err)
	require.Greater(t, billedInputTokens, kiroStandardContextBudgetTokens)
	require.Equal(t, "claude-sonnet-4.6", converted.Model)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(converted.Body, &payload))
	state, ok := payload["conversationState"].(map[string]any)
	require.True(t, ok)
	currentMessage, ok := state["currentMessage"].(map[string]any)
	require.True(t, ok)
	current, ok := currentMessage["userInputMessage"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "claude-sonnet-4.6", current["modelId"])
}

func TestBuildKiroGenerateAssistantRequest_ExternalIDPSetsTokenTypeHeader(t *testing.T) {
	account := &Account{
		ID:       112,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"auth_method": "ExternalIdp",
		},
	}

	req, err := buildKiroGenerateAssistantRequest(context.Background(), account, []byte(`{}`), "access-token", DefaultKiroRuntimeSettings())

	require.NoError(t, err)
	require.Equal(t, "EXTERNAL_IDP", req.Header.Get("TokenType"))
}

func TestPrepareKiroConvertedRequest_IncludesAccountProfileARN(t *testing.T) {
	account := &Account{
		ID:       111,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"profile_arn": "arn:aws:codewhisperer:us-east-1:123456789012:profile/test",
		},
	}
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}`)
	injected := injectKiroProfileARNIntoAnthropicBody(body, account)
	var injectedPayload map[string]any
	require.NoError(t, json.Unmarshal(injected, &injectedPayload))
	require.Equal(t, "arn:aws:codewhisperer:us-east-1:123456789012:profile/test", account.GetCredential("profile_arn"))
	require.Equal(t, "arn:aws:codewhisperer:us-east-1:123456789012:profile/test", injectedPayload["profile_arn"])

	converted, _, err := prepareKiroConvertedRequest(account, &ParsedRequest{
		Model: "claude-sonnet-4-6",
		Body:  NewRequestBodyRef(body),
	}, nil)

	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(converted.Body, &payload))
	require.Equal(t, "arn:aws:codewhisperer:us-east-1:123456789012:profile/test", payload["profileArn"])
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

func TestKiroGatewayService_Forward_FreeAccountThinkingRequestUsesSingleUpstreamCall(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{}
	upstream.doFunc = func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
		switch upstream.calls {
		case 1:
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
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"thinking":{"type":"enabled","budget_tokens":5000},
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, upstream.calls, "free accounts should not use the removed thinking preflight")
	require.NotContains(t, rec.Body.String(), `"type":"thinking"`)
	require.NotContains(t, rec.Body.String(), `preflight reasoning`)
	require.Contains(t, rec.Body.String(), `"text":"final answer"`)
	require.NotContains(t, rec.Body.String(), `Thinking through the request with high effort`)
}

func TestKiroGatewayService_Forward_LegacyThinkingSettingsDoNotFallbackToSimulatedThinking(t *testing.T) {
	setGinTestMode()

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
			"kiro_thinking_mode":                "model_and_simulate",
			"kiro_thinking_effort_threshold":    "medium",
			"kiro_thinking_simulation_template": "fallback {effort} {model} {upstream_model}",
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
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"thinking":{"type":"enabled","budget_tokens":5000},
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, upstream.calls, "legacy model_and_simulate setting should not try the removed free thinking preflight")
	require.NotContains(t, rec.Body.String(), `"type":"thinking"`)
	require.NotContains(t, rec.Body.String(), `fallback high claude-sonnet-4-5-20250929 claude-sonnet-4.5`)
	require.Contains(t, rec.Body.String(), `"text":"final answer"`)
}

func TestKiroGatewayService_Forward_NativeThinkingBlocksUseUpstreamContent(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "assistantResponseEvent",
			}, map[string]any{"content": "<thinking>\nreal native reasoning</thinking>\n\nfinal answer"}))),
		},
	}
	svc := &KiroGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:       113,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}
	parsed := &ParsedRequest{
		Model:           "claude-opus-4-6",
		ThinkingEnabled: true,
		OutputEffort:    "high",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-opus-4-6",
			"thinking":{"type":"enabled","budget_tokens":5000},
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, upstream.calls)
	require.Contains(t, rec.Body.String(), `"thinking":"real native reasoning","type":"thinking"`)
	require.Contains(t, rec.Body.String(), `"text":"final answer","type":"text"`)
	require.NotContains(t, rec.Body.String(), `Thinking through the request with high effort`)
}

func TestKiroGatewayService_ForwardStream_NativeThinkingBlocksUseUpstreamContent(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "<thinking>\nreal"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": " native reasoning</thinking>\n\nfinal"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": " answer"}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 114, Platform: PlatformKiro, Type: AccountTypeAPIKey},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-opus-4-6", Stream: true, ThinkingEnabled: true},
		&kiropkg.ConvertResult{Model: "claude-opus-4.6"},
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
	require.Contains(t, output, `"content_block":{"signature":"","thinking":"","type":"thinking"}`)
	require.Contains(t, output, `"delta":{"thinking":"real native reasoning","type":"thinking_delta"}`)
	require.Contains(t, output, `"content_block":{"text":"","type":"text"}`)
	require.Contains(t, output, `"delta":{"text":"final","type":"text_delta"}`)
	require.Contains(t, output, `"delta":{"text":" answer","type":"text_delta"}`)
	require.NotContains(t, output, `Thinking through the request with high effort`)
}

func TestKiroGatewayService_ForwardStream_ReasoningContentEventEmitsThinkingBlock(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "reasoningContentEvent",
		}, map[string]any{"text": "let me think"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "reasoningContentEvent",
		}, map[string]any{"text": " step by step"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "reasoningContentEvent",
		}, map[string]any{"signature": "sig-abc"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "final answer"}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 201, Platform: PlatformKiro, Type: AccountTypeAPIKey},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-opus-4-7", Stream: true, ThinkingEnabled: true},
		&kiropkg.ConvertResult{Model: "claude-opus-4.7"},
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
	require.Contains(t, output, `"content_block":{"signature":"","thinking":"","type":"thinking"}`)
	require.Contains(t, output, `"delta":{"thinking":"let me think","type":"thinking_delta"}`)
	require.Contains(t, output, `"delta":{"thinking":" step by step","type":"thinking_delta"}`)
	require.Contains(t, output, `"delta":{"signature":"sig-abc","type":"signature_delta"}`)
	require.Contains(t, output, `"delta":{"text":"final answer","type":"text_delta"}`)
	signatureAt := strings.Index(output, `"delta":{"signature":"sig-abc","type":"signature_delta"}`)
	require.NotEqual(t, -1, signatureAt)
	afterSignature := output[signatureAt:]
	stopAfterSignature := strings.Index(afterSignature, `"type":"content_block_stop"`)
	require.NotEqual(t, -1, stopAfterSignature)
	require.NotContains(t, afterSignature[:stopAfterSignature], `"type":"thinking_delta"`)
	thinkingStart := strings.Index(output, `"content_block":{"signature":"","thinking":"","type":"thinking"}`)
	textStart := strings.Index(output, `"content_block":{"text":"","type":"text"}`)
	require.Greater(t, textStart, thinkingStart, "thinking block must precede text block")
	thinkingStop := strings.Index(output[thinkingStart:], `"content_block_stop"`)
	require.GreaterOrEqual(t, thinkingStop, 0, "thinking block should be closed before text block opens")
}

func TestKiroGatewayService_ForwardNonStream_ReasoningContentEventEmitsThinkingBlock(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader(bytes.Join([][]byte{
				buildKiroTestFrame(t, map[string]string{
					":message-type": "event",
					":event-type":   "reasoningContentEvent",
				}, map[string]any{"text": "let me think step by step"}),
				buildKiroTestFrame(t, map[string]string{
					":message-type": "event",
					":event-type":   "reasoningContentEvent",
				}, map[string]any{"signature": "sig-xyz"}),
				buildKiroTestFrame(t, map[string]string{
					":message-type": "event",
					":event-type":   "assistantResponseEvent",
				}, map[string]any{"content": "final answer"}),
			}, nil))),
		},
	}
	svc := &KiroGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:       202,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}
	parsed := &ParsedRequest{
		Model:           "claude-opus-4-7",
		ThinkingEnabled: true,
		OutputEffort:    "high",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-opus-4-7",
			"thinking":{"type":"enabled","budget_tokens":5000},
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, upstream.calls)
	body := rec.Body.String()
	require.Contains(t, body, `"thinking":"let me think step by step"`)
	require.Contains(t, body, `"signature":"sig-xyz"`)
	require.Contains(t, body, `"text":"final answer","type":"text"`)
	require.NotContains(t, body, `Thinking through the request with high effort`)
}

func TestNormalizeKiroShadowToolHistory_PreservesNativeWebSearchHistory(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4",
		"messages":[
			{
				"role":"assistant",
				"content":[
					{"type":"server_tool_use","id":"toolu_search_1","name":"web_search","input":{"query":"golang"}},
					{"type":"text","text":"search completed"}
				]
			},
			{
				"role":"user",
				"content":[
					{"type":"web_search_tool_result","tool_use_id":"toolu_search_1","content":[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]},
					{"type":"text","text":"Summarize the result"}
				]
			}
		]
	}`)

	normalized := normalizeKiroShadowToolHistory(input)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(normalized, &payload))
	messages, ok := payload["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)

	assistant, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assistantContent, ok := assistant["content"].([]any)
	require.True(t, ok)
	require.Len(t, assistantContent, 2)
	firstAssistantBlock, ok := assistantContent[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "server_tool_use", firstAssistantBlock["type"])
	require.Equal(t, "toolu_search_1", firstAssistantBlock["id"])
	require.Equal(t, "web_search", firstAssistantBlock["name"])
	firstAssistantInput, _ := firstAssistantBlock["input"].(map[string]any)
	require.Equal(t, "golang", firstAssistantInput["query"])

	user, ok := messages[1].(map[string]any)
	require.True(t, ok)
	userContent, ok := user["content"].([]any)
	require.True(t, ok)
	require.Len(t, userContent, 2)
	firstUserBlock, ok := userContent[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "web_search_tool_result", firstUserBlock["type"])
	require.Equal(t, "toolu_search_1", firstUserBlock["tool_use_id"])
	results, ok := firstUserBlock["content"].([]any)
	require.True(t, ok)
	require.Len(t, results, 1)
	firstResult, ok := results[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "url", firstResult["type"])
	require.Equal(t, "https://go.dev", firstResult["url"])
}

func TestNormalizeKiroShadowToolHistory_PreservesNativeWebFetchHistory(t *testing.T) {
	input := []byte(`{
		"model":"claude-sonnet-4",
		"messages":[
			{
				"role":"assistant",
				"content":[
					{
						"type":"server_tool_use",
						"id":"toolu_fetch_1",
						"name":"web_fetch",
						"input":{"url":"https://example.com/fetch"},
						"allowed_domains":["example.com","docs.example.com"],
						"blocked_domains":["blocked.example.com"],
						"max_uses":2,
						"max_content_tokens":4096
					}
				]
			},
			{
				"role":"user",
				"content":[
					{
						"type":"web_fetch_tool_result",
						"tool_use_id":"toolu_fetch_1",
						"content":{
							"type":"web_fetch_tool_error",
							"url":"https://example.com/fetch",
							"error_code":"url_not_accessible",
							"text":"dial tcp: i/o timeout"
						}
					}
				]
			}
		]
	}`)

	normalized := normalizeKiroShadowToolHistory(input)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(normalized, &payload))
	messages, _ := payload["messages"].([]any)

	assistant, _ := messages[0].(map[string]any)
	assistantContent, _ := assistant["content"].([]any)
	firstAssistantBlock, _ := assistantContent[0].(map[string]any)
	require.Equal(t, "server_tool_use", firstAssistantBlock["type"])
	require.Equal(t, "web_fetch", firstAssistantBlock["name"])
	require.Equal(t, float64(2), firstAssistantBlock["max_uses"])
	require.Equal(t, float64(4096), firstAssistantBlock["max_content_tokens"])
	require.Equal(t, []any{"example.com", "docs.example.com"}, firstAssistantBlock["allowed_domains"])
	require.Equal(t, []any{"blocked.example.com"}, firstAssistantBlock["blocked_domains"])

	user, _ := messages[1].(map[string]any)
	userContent, _ := user["content"].([]any)
	firstUserBlock, _ := userContent[0].(map[string]any)
	require.Equal(t, "web_fetch_tool_result", firstUserBlock["type"])
	errorContent, _ := firstUserBlock["content"].(map[string]any)
	require.Equal(t, "web_fetch_tool_error", errorContent["type"])
	require.Equal(t, "url_not_accessible", errorContent["error_code"])
}

func TestNormalizeKiroShadowToolHistory_RestoresLegacyGoogleSearchAlias(t *testing.T) {
	recovered := normalizeKiroShadowToolHistory([]byte(`{
		"model":"claude-sonnet-4",
		"messages":[
			{
				"role":"assistant",
				"content":[
					{"type":"server_tool_use","id":"toolu_search_1","name":"google_search","input":{"query":"golang"}}
				]
			},
			{
				"role":"user",
				"content":[
					{"type":"web_search_tool_result","tool_use_id":"toolu_search_1","content":[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]}
				]
			}
		]
	}`))
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recovered, &payload))
	messages, ok := payload["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)
	assistant, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assistantContent, ok := assistant["content"].([]any)
	require.True(t, ok)
	require.Len(t, assistantContent, 1)
	firstAssistantBlock, ok := assistantContent[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_use", firstAssistantBlock["type"])
	require.Equal(t, kiropkg.ShadowToolWebSearch, firstAssistantBlock["name"])
	bridge, _ := firstAssistantBlock["_shadow_bridge"].(map[string]any)
	require.Equal(t, "google_search", bridge["anthropic_name"])
	user, ok := messages[1].(map[string]any)
	require.True(t, ok)
	userContent, ok := user["content"].([]any)
	require.True(t, ok)
	require.Len(t, userContent, 1)
	firstUserBlock, ok := userContent[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "tool_result", firstUserBlock["type"])
}

func TestKiroGatewayService_ForwardStream_DoesNotSplitUTF8WhenBufferingThinkingMarkers(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	chinese := "中文中文中文"
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "assistantResponseEvent",
	}, map[string]any{"content": chinese})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 115, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-opus-4-6", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-opus-4.6"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	var reconstructed strings.Builder
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
		if deltaType != "text_delta" {
			continue
		}
		chunk, _ := delta["text"].(string)
		_, _ = reconstructed.WriteString(chunk)
	}

	require.Equal(t, chinese, reconstructed.String())
	require.NotContains(t, rec.Body.String(), "�")
}

func TestExtractKiroNativeThinkingText_UsesSameParserAsNativeNonStreamPath(t *testing.T) {
	thinking := extractKiroNativeThinkingText("prefix<thinking>\nfirst pass</thinking>\n\nmiddle<thinking>second pass</thinking>\n\nsuffix")

	require.Equal(t, "first pass\n\nsecond pass", thinking)
}

func TestKiroResponseTelemetry_OmitsSimulatedThinkingFields(t *testing.T) {
	telemetry := &kiroResponseTelemetry{
		FramesSeen:          1,
		AssistantChars:      12,
		NativeThinkingChars: 5,
	}

	require.NotContains(t, telemetry.completionKinds(), "simulated_thinking")
	require.NotContains(t, telemetry.anomalyKinds("end_turn"), "fallback_thinking_only")
	require.NotContains(t, telemetry.opsDetail("end_turn", 8, false, nil), "simulated_thinking")
}

func TestKiroGatewayService_ForwardCountTokens_RejectsInvalidConversationShape(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{}

	err := svc.ForwardCountTokens(context.Background(), c, &Account{
		ID:       102,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, &ParsedRequest{
		Model: "claude-sonnet-4-6",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-6",
			"messages":[{"role":"assistant","content":[{"type":"text","text":"hello"}]}]
		}`)),
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "empty messages")
}

func TestKiroGatewayService_ForwardCountTokens_UsesForwardValidationWithLocalEstimate(t *testing.T) {
	setGinTestMode()

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
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello from count tokens"}]}]
		}`)),
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	// count_tokens shares the calibrated EstimateInputTokens contract so the
	// reported value matches what usage reporting bills (raw tiktoken count
	// would be 4; calibration adds base + content scaling + per-message overhead).
	expectedCalibrated := kiropkg.EstimateInputTokens([]byte(`{
			"model":"claude-sonnet-4-5-20250929",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello from count tokens"}]}]
		}`))
	require.Equal(t, 4, expectedInputTokens, "raw tiktoken count should remain stable as calibration baseline")
	require.Greater(t, expectedCalibrated, expectedInputTokens, "calibrated estimate must exceed raw count")
	require.JSONEq(t, fmt.Sprintf(`{"input_tokens":%d}`, expectedCalibrated), rec.Body.String())
}

func TestKiroGatewayService_ForwardNonStream_ExceptionDoesNotCommitFakeCache(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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

func TestKiroGatewayService_ForwardStream_DropsRepeatedWordHoldbackOnExceptionAndUsesLongCooldown(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	repo := &kiroPreStartAccountRepoStub{}
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
		rateLimitService: &RateLimitService{
			accountRepo: repo,
		},
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "partial output"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "\n\ncourt"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "\n\ncourt"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "\n\ncourt"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "\n\ncourt"}),
		buildKiroTestFrame(t, map[string]string{
			":event-type":     "exception",
			":exception-type": "RuntimeException",
		}, map[string]any{"message": "Encountered an unexpected error when processing the request, please try again."}),
	}, nil)

	before := time.Now()
	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 75521, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-opus-4-8", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-opus-4.8"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Error(t, err)
	require.Nil(t, result)
	output := rec.Body.String()
	require.Contains(t, output, `"partial output"`)
	require.NotContains(t, output, "court", "repeated upstream degeneration words should be withheld and dropped when the stream ends with exception")
	require.Contains(t, output, "Kiro upstream generation failed after stream started")
	require.NotContains(t, output, "kiro upstream returned exception frame")

	require.Len(t, repo.calls, 1)
	require.Equal(t, int64(75521), repo.calls[0].accountID)
	require.True(t, repo.calls[0].until.After(before.Add(kiroPostStartGenerationFailureCooldown-time.Second)))
	require.Contains(t, repo.calls[0].reason, kiroPostStartGenerationFailureReasonKeyword)
}

func TestKiroGatewayService_ForwardStream_FlushesRepeatedWordHoldbackOnNormalCompletion(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: gocache.New(time.Minute, time.Minute),
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "court"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "\n\ncourt"}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 75521, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-opus-4-8", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-opus-4.8"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), "court")
}

func TestKiroGatewayService_ForwardNonStream_UsageMatchesAnthropicCacheShape(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}
	fakeCachePlan := &kiropkg.FakeCachePlan{
		CurrentKey:             "kiro:test:usage-shape",
		CurrentCacheableTokens: 17,
	}
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "assistantResponseEvent",
	}, map[string]any{"content": "hello"})
	headers := http.Header{"X-Amzn-Requestid": []string{"kiro-request-usage"}}

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 1, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: headers},
		&ParsedRequest{Model: "claude-sonnet-4-6"},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.6"},
		32,
		time.Now(),
		fakeCachePlan,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	usage, ok := payload["usage"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, usage, "cache_creation_input_tokens")
	require.Contains(t, usage, "cache_read_input_tokens")
	require.Contains(t, usage, "output_tokens_details")
	require.NotContains(t, usage, "service_tier")
	require.NotContains(t, usage, "inference_geo")

	// Verify context_management and stop_details are present
	require.Contains(t, payload, "context_management")
	require.Contains(t, payload, "stop_details")

	require.Equal(t, result.Usage.CacheCreationInputTokens, result.Usage.CacheCreation5mTokens)
	require.Zero(t, result.Usage.CacheCreation1hTokens)
}

func TestKiroGatewayService_ForwardStream_PopulatesCacheCreationTTLBreakdown(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}
	fakeCachePlan := &kiropkg.FakeCachePlan{
		CurrentKey:             "kiro:test:stream-cache-ttl",
		CurrentCacheableTokens: 17,
	}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "assistantResponseEvent",
	}, map[string]any{"content": "hello"})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 1, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4-6", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.6"},
		32,
		time.Now(),
		fakeCachePlan,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, result.Usage.CacheCreationInputTokens, result.Usage.CacheCreation5mTokens)
	require.Zero(t, result.Usage.CacheCreation1hTokens)
}

func TestKiroGatewayService_Forward_HTTPErrorRecordsOpsContext(t *testing.T) {
	setGinTestMode()

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
			Body: NewRequestBodyRef([]byte(`{
				"model":"claude-sonnet-4-5-20250929",
				"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
			}`)),
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

func TestKiroGatewayService_Forward_Kiro429MarksSameAccountRetry(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header: http.Header{
				"X-Amzn-Requestid": []string{"kiro-request-429"},
			},
			Body: io.NopCloser(strings.NewReader(`{"message":"Too many requests, please wait before trying again.","reason":null}`)),
		},
	}
	svc := &KiroGatewayService{
		httpUpstream: upstream,
	}

	result, err := svc.Forward(
		context.Background(),
		c,
		&Account{
			ID:       8,
			Name:     "Kiro Gateway",
			Platform: PlatformKiro,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key": "kiro-api-key",
			},
		},
		&ParsedRequest{
			Model: "claude-sonnet-4-6",
			Body: NewRequestBodyRef([]byte(`{
				"model":"claude-sonnet-4-6",
				"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
			}`)),
		},
	)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Equal(t, 3*time.Second, failoverErr.SameAccountRetryDelay)
	require.Equal(t, 3, failoverErr.SameAccountRetryMax)
	require.Equal(t, time.Minute, failoverErr.RetryExhaustedCooldown)
	require.Equal(t, "kiro_429_retry_exhausted", failoverErr.RetryExhaustedReason)
	require.Equal(t, "kiro-request-429", failoverErr.ResponseHeaders.Get("X-Amzn-Requestid"))
}

func TestKiroGatewayService_Forward_Kiro429SuspiciousActivityDoesNotMarkSameAccountRetry(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header: http.Header{
				"X-Amzn-Requestid": []string{"kiro-request-429-suspicious"},
			},
			Body: io.NopCloser(strings.NewReader(`{"message":"Due to suspicious activity, we are imposing temporary limits on how frequently your account can send a request to Kiro while we investigate."}`)),
		},
	}
	svc := &KiroGatewayService{
		httpUpstream: upstream,
	}

	result, err := svc.Forward(
		context.Background(),
		c,
		&Account{
			ID:       9,
			Name:     "Kiro Gateway",
			Platform: PlatformKiro,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key": "kiro-api-key",
			},
		},
		&ParsedRequest{
			Model: "claude-sonnet-4-6",
			Body: NewRequestBodyRef([]byte(`{
				"model":"claude-sonnet-4-6",
				"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
			}`)),
		},
	)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Zero(t, failoverErr.SameAccountRetryDelay)
	require.Zero(t, failoverErr.SameAccountRetryMax)
	require.Equal(t, "kiro-request-429-suspicious", failoverErr.ResponseHeaders.Get("X-Amzn-Requestid"))
}

func TestKiroGatewayService_ForwardStream_PreStartExceptionReturnsFailover(t *testing.T) {
	setGinTestMode()

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
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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

func TestKiroMarkerPrefixHoldbackBytes(t *testing.T) {
	require.Equal(t, 0, kiroMarkerPrefixHoldbackBytes("plain text", "<thinking>"))
	require.Equal(t, len("<thin"), kiroMarkerPrefixHoldbackBytes("answer <thin", "<thinking>"))
	require.Equal(t, len("<thinking"), kiroMarkerPrefixHoldbackBytes("answer <thinking", "<thinking>"))
	require.Equal(t, 0, kiroMarkerPrefixHoldbackBytes("answer <thinking>", "<thinking>"))
}

func TestKiroGatewayService_ForwardNonStream_EmptyBodyFails(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "event: error")
	require.NotContains(t, rec.Body.String(), "event: message_start")
	require.NotContains(t, rec.Body.String(), "event: message_delta")
	require.NotContains(t, rec.Body.String(), "event: message_stop")
}

func TestKiroGatewayService_ForwardStream_ContextOnlyBodyFailsWithoutStartingStream(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "event: error")
	require.NotContains(t, rec.Body.String(), "context usage reached 98%")
	require.NotContains(t, rec.Body.String(), "event: message_start")
}

func TestKiroGatewayService_ForwardNonStream_ContextOnlyBodyReturnsAnomaly(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "contextUsageEvent",
	}, map[string]any{"contextUsagePercentage": 42})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 5, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4-6", ThinkingEnabled: true, OutputEffort: "high"},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.6"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.ErrorContains(t, err, "kiro response contained no assistant output")
	require.Nil(t, result)
	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "request_error", events[0].Kind)
	require.Contains(t, events[0].Message, "kiro response contained no assistant output")
}

func TestKiroGatewayService_ForwardStream_ContextOnlyBodyFailsWithoutThinkingFallback(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "contextUsageEvent",
	}, map[string]any{"contextUsagePercentage": 42})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 6, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4-6", Stream: true, ThinkingEnabled: true, OutputEffort: "high"},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.6"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.ErrorContains(t, err, "upstream error")
	require.Nil(t, result)
	require.Empty(t, rec.Body.String())
}

func TestKiroGatewayService_ForwardStream_PlaceholderOnlyBodyFailsWithoutThinkingFallback(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "assistantResponseEvent",
	}, map[string]any{"content": "I will call the requested tools."})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 6, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4-6", Stream: true, ThinkingEnabled: true, OutputEffort: "high"},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.6"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.ErrorContains(t, err, "upstream error")
	require.Nil(t, result)
	require.Empty(t, rec.Body.String())
}

func TestKiroGatewayService_ForwardStream_PlaceholderOnlyBodyWithoutThinkingFailsBeforeStreamStart(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	repo := &kiroPreStartAccountRepoStub{}
	svc := &KiroGatewayService{
		rateLimitService: &RateLimitService{
			accountRepo: repo,
		},
	}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "assistantResponseEvent",
	}, map[string]any{"content": "I will call the requested tools."})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 6, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4-6", Stream: true},
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
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Empty(t, rec.Body.String())
	require.Len(t, repo.calls, 1)
}

func TestKiroGatewayService_ForwardNonStream_IncompleteToolUseReturnsRecoverableFailure(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{
		"toolUseId": "tool-1",
		"name":      "search",
		"input":     `{"q":"a"`,
	})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 7, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: false},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.6"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Contains(t, string(failoverErr.ResponseBody), "incomplete tool_use output")
	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "response_anomaly", events[0].Kind)
	require.Contains(t, events[0].Message, "incomplete_tool_use_completed")
	require.Contains(t, events[0].Detail, `"partial_tool_use_count":1`)
}

func TestKiroGatewayService_ForwardNonStream_ValidToolUseWithoutStopSucceeds(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "I'll call a tool now."}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{
			"toolUseId": "tool-1",
			"name":      "search",
			"input":     `{"q":"a"}`,
		}),
	}, nil)

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 7, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: false},
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
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"stop_reason":"tool_use"`)
	require.Contains(t, rec.Body.String(), `"type":"tool_use"`)
	require.Contains(t, rec.Body.String(), `"name":"search"`)
	require.NotContains(t, rec.Body.String(), "incomplete tool_use output")
	_, ok := c.Get(OpsUpstreamErrorsKey)
	require.False(t, ok)
}

func TestKiroGatewayService_ForwardNonStream_OfficialBashMapsBackToAnthropicName(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{
		"toolUseId": "tool-1",
		"name":      "Bash",
		"input":     `{"command":"pwd"}`,
		"stop":      true,
	})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 7, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: false},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.6",
			ToolMetadata: &kiropkg.ToolMetadata{ResponseTools: map[string]kiropkg.ResponseToolBridge{
				"Bash": {AnthropicType: "bash_20250124", AnthropicName: "bash", Family: "anthropic_bash"},
			}},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"name":"bash"`)
	require.NotContains(t, rec.Body.String(), `"name":"Bash"`)
}

func TestKiroGatewayService_ForwardNonStream_CustomBashStaysCustom(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{
		"toolUseId": "tool-1",
		"name":      "Bash",
		"input":     `{"script":"custom"}`,
		"stop":      true,
	})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 7, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: false},
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
	require.Contains(t, rec.Body.String(), `"name":"Bash"`)
	require.Contains(t, rec.Body.String(), `"script":"custom"`)
}

func TestKiroGatewayService_ForwardNonStream_OfficialTextEditorMapsBackToAnthropicInput(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{
		"toolUseId": "tool-1",
		"name":      "Edit",
		"input":     `{"file_path":"/tmp/a.txt","old_string":"old","new_string":"new"}`,
		"stop":      true,
	})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 7, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: false},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.6",
			ToolMetadata: &kiropkg.ToolMetadata{ResponseTools: map[string]kiropkg.ResponseToolBridge{
				"Read":  {AnthropicType: "text_editor_20250728", AnthropicName: "str_replace_based_edit_tool", Family: "anthropic_text_editor"},
				"Write": {AnthropicType: "text_editor_20250728", AnthropicName: "str_replace_based_edit_tool", Family: "anthropic_text_editor"},
				"Edit":  {AnthropicType: "text_editor_20250728", AnthropicName: "str_replace_based_edit_tool", Family: "anthropic_text_editor"},
			}},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"name":"str_replace_based_edit_tool"`)
	require.Contains(t, rec.Body.String(), `"command":"str_replace"`)
	require.Contains(t, rec.Body.String(), `"path":"/tmp/a.txt"`)
	require.Contains(t, rec.Body.String(), `"old_str":"old"`)
	require.Contains(t, rec.Body.String(), `"new_str":"new"`)
}

func TestKiroGatewayService_ForwardNonStream_NativeWebSearchMapsToServerToolUse(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		require.Equal(t, "golang", query)
		return &websearch.SearchResponse{
			Query: query,
			Results: []websearch.SearchResult{
				{URL: "https://go.dev", Title: "The Go Programming Language", Snippet: "Official site"},
			},
		}, "stub", nil
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{
		"toolUseId": "tool-search-1",
		"name":      "web_search",
		"input":     `{"query":"golang"}`,
		"stop":      true,
	})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 7, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: false},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.6",
			ToolMetadata: &kiropkg.ToolMetadata{ResponseTools: map[string]kiropkg.ResponseToolBridge{
				"web_search": {AnthropicType: "web_search_20250305", AnthropicName: "web_search", Family: "anthropic_web_search"},
			}},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"type":"server_tool_use"`)
	require.Contains(t, rec.Body.String(), `"name":"web_search"`)
	require.Contains(t, rec.Body.String(), `"query":"golang"`)
	require.Contains(t, rec.Body.String(), `"type":"web_search_tool_result"`)
	require.Contains(t, rec.Body.String(), `"url":"https://go.dev"`)
	require.NotContains(t, rec.Body.String(), `"type":"tool_use"`)
}

func TestKiroGatewayService_ForwardNonStream_NativeWebFetchMapsToServerToolUse(t *testing.T) {
	setGinTestMode()

	previousFetchExecutor := kiroShadowWebFetchExecutor
	kiroShadowWebFetchExecutor = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
		require.Equal(t, "https://example.com/doc", req.URL)
		return &webfetch.FetchResult{
			RequestedURL: req.URL,
			FinalURL:     req.URL,
			StatusCode:   http.StatusOK,
			ContentType:  "text/html",
			Title:        "Fetch Title",
			Text:         "Fetch body",
		}
	}
	t.Cleanup(func() { kiroShadowWebFetchExecutor = previousFetchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{
		"toolUseId": "tool-fetch-1",
		"name":      "web_fetch",
		"input":     `{"url":"https://example.com/doc"}`,
		"stop":      true,
	})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 7, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: false},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.6",
			ToolMetadata: &kiropkg.ToolMetadata{ResponseTools: map[string]kiropkg.ResponseToolBridge{
				"web_fetch": {AnthropicType: "web_fetch_20260318", AnthropicName: "web_fetch", Family: "anthropic_web_fetch"},
			}},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"type":"server_tool_use"`)
	require.Contains(t, rec.Body.String(), `"name":"web_fetch"`)
	require.Contains(t, rec.Body.String(), `"url":"https://example.com/doc"`)
	require.Contains(t, rec.Body.String(), `"type":"web_fetch_tool_result"`)
	require.Contains(t, rec.Body.String(), `"Fetch body"`)
	require.NotContains(t, rec.Body.String(), `"type":"tool_use"`)
}

func TestKiroGatewayService_ForwardNonStream_SuppressesTrailingPlaceholderFragmentBeforeToolUse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "Some real answer. "}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "call"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{
			"toolUseId": "tool-1",
			"name":      "search",
			"input":     `{"q":"a"}`,
			"stop":      true,
		}),
	}, nil)

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 7, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: false},
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
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"text":"Some real answer. "`)
	require.NotContains(t, rec.Body.String(), `"text":"Some real answer. call"`)
	require.Contains(t, rec.Body.String(), `"type":"tool_use"`)
	require.Contains(t, rec.Body.String(), `"name":"search"`)
}

func TestKiroGatewayService_ForwardStream_ContextWindowExceededUsesStopReason(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "contextUsageEvent",
		}, map[string]any{"contextUsagePercentage": 100}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "done"}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 8, Platform: PlatformKiro, Type: AccountTypeOAuth},
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
	require.Contains(t, rec.Body.String(), `"stop_reason":"model_context_window_exceeded"`)
	require.Contains(t, rec.Body.String(), "event: message_stop")
	require.NotContains(t, rec.Body.String(), "event: error")
	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "response_anomaly", events[0].Kind)
	require.Contains(t, events[0].Message, "context_window_exceeded")
	require.Contains(t, events[0].Detail, `"stop_reason":"model_context_window_exceeded"`)
}

func TestKiroGatewayService_ForwardStream_DoesNotBillContextUsagePercentageAsInputTokens(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
	require.NotContains(t, rec.Body.String(), "service_tier")
	require.NotContains(t, rec.Body.String(), "inference_geo")
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
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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

func TestKiroGatewayService_ForwardStream_IncompleteToolUseEOFReturnsRecoverableFailure(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{
		"toolUseId": "tool-1",
		"name":      "search",
		"input":     `{"q":"a"`,
	})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 9, Platform: PlatformKiro, Type: AccountTypeOAuth},
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

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, rec.Body.String(), `"id":"tool-1"`)
	require.Contains(t, rec.Body.String(), `"partial_json":"{\"q\":\"a\""`)
	require.Contains(t, rec.Body.String(), "event: error")
	require.Contains(t, rec.Body.String(), "incomplete tool_use output")
	require.NotContains(t, rec.Body.String(), "event: message_stop")
	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "response_anomaly", events[0].Kind)
	require.Contains(t, events[0].Message, "incomplete_tool_use_completed")
	require.Contains(t, events[0].Detail, `"partial_tool_use_count":1`)
}

func TestKiroGatewayService_ForwardStream_ValidToolUseWithoutStopCompletesAtEOF(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "assistantResponseEvent",
		}, map[string]any{"content": "I'll call a tool now."}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{
			"toolUseId": "tool-1",
			"name":      "search",
			"input":     `{"q":"a"}`,
		}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 9, Platform: PlatformKiro, Type: AccountTypeOAuth},
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
	require.Contains(t, rec.Body.String(), `"id":"tool-1"`)
	require.Contains(t, rec.Body.String(), `"partial_json":"{\"q\":\"a\"}"`)
	require.Contains(t, rec.Body.String(), `"stop_reason":"tool_use"`)
	require.Contains(t, rec.Body.String(), `event: message_stop`)
	require.NotContains(t, rec.Body.String(), `event: error`)
	_, ok := c.Get(OpsUpstreamErrorsKey)
	require.False(t, ok)
}

func TestKiroGatewayService_ForwardStream_TextToolTextClosesBlocksInOrder(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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

func TestKiroGatewayService_ForwardStream_SuppressesBufferedPlaceholderFragmentBeforeToolUse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
		}, map[string]any{"content": "call"}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-2", "name": "lookup", "input": `{"q":"b"}`, "stop": true}),
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
	require.Contains(t, output, `"delta":{"text":"hello","type":"text_delta"}`)
	require.NotContains(t, output, `"delta":{"text":"call","type":"text_delta"}`)
	require.Contains(t, output, `"id":"tool-1"`)
	require.Contains(t, output, `"id":"tool-2"`)
	require.Contains(t, output, `"stop_reason":"tool_use"`)
}

func TestKiroGatewayService_ForwardStream_PreservesWhitespaceInContentAndInputDeltas(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{
		fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
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
			chunk, _ := delta["text"].(string)
			textDelta += chunk
		case "input_json_delta":
			inputDelta, _ = delta["partial_json"].(string)
		}
	}

	require.Equal(t, "  keep text delta spaces  ", textDelta)
	require.Equal(t, " {\"q\":\" value with spaces \"} ", inputDelta)
	require.Contains(t, rec.Body.String(), `"id":"tool-1"`, "tool_use id should still be normalized as a control field")
	require.Contains(t, rec.Body.String(), `"name":"search"`, "tool_use name should still be normalized as a control field")
}

func TestKiroGatewayService_ForwardNonStream_ShadowWebSearchReturnsServerToolPauseTurn(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		require.Equal(t, int64(10), account.ID)
		require.Equal(t, "golang", query)
		return &websearch.SearchResponse{
			Query: query,
			Results: []websearch.SearchResult{
				{URL: "https://example.com/golang", Title: "Go", Snippet: "The Go programming language"},
			},
		}, "stub", nil
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-1", "name": "cc_srv_web_search", "input": `{"query":"golang"}`, "stop": true})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{
			ID:       10,
			Platform: PlatformKiro,
			Type:     AccountTypeOAuth,
		},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_search": {AnthropicType: "web_search_20250305", AnthropicName: "web_search"},
				},
			},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"type":"server_tool_use"`)
	require.Contains(t, rec.Body.String(), `"type":"web_search_tool_result"`)
	require.Contains(t, rec.Body.String(), `"stop_reason":"pause_turn"`)
}

func TestKiroGatewayService_ForwardStream_ShadowWebSearchEmitsInputJSONDeltaAndPauseTurn(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		require.Equal(t, int64(1010), account.ID)
		require.Equal(t, "golang", query)
		return &websearch.SearchResponse{
			Query: query,
			Results: []websearch.SearchResult{
				{URL: "https://go.dev", Title: "The Go Programming Language", Snippet: "Official site"},
			},
		}, "stub", nil
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-shadow", "name": "cc_srv_web_search", "input": `{"query":"golang"}`, "stop": true})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 1010, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_search": {AnthropicType: "web_search_20250305", AnthropicName: "web_search"},
				},
			},
		},
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
	require.Contains(t, output, `"content_block":{"id":"tool-shadow","input":{},"name":"web_search","type":"server_tool_use"}`)
	require.Contains(t, output, `"delta":{"partial_json":"{\"query\":\"golang\"}","type":"input_json_delta"}`)
	require.Contains(t, output, `"type":"web_search_tool_result"`)
	require.Contains(t, output, `"stop_reason":"pause_turn"`)
}

func TestKiroGatewayService_ForwardStream_NativeServerToolsEmitServerToolUse(t *testing.T) {
	setGinTestMode()

	cases := []struct {
		name          string
		kiroName      string
		anthropicType string
		family        string
		input         string
		resultType    string
	}{
		{
			name:          "web_search",
			kiroName:      "web_search",
			anthropicType: "web_search_20250305",
			family:        "anthropic_web_search",
			input:         `{"query":"golang"}`,
			resultType:    "web_search_tool_result",
		},
		{
			name:          "web_fetch",
			kiroName:      "web_fetch",
			anthropicType: "web_fetch_20260318",
			family:        "anthropic_web_fetch",
			input:         `{"url":"https://example.com/doc"}`,
			resultType:    "web_fetch_tool_result",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			previousSearchExecutor := kiroShadowWebSearchExecutor
			previousFetchExecutor := kiroShadowWebFetchExecutor
			kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
				require.Equal(t, "golang", query)
				return &websearch.SearchResponse{
					Query: query,
					Results: []websearch.SearchResult{
						{URL: "https://go.dev", Title: "The Go Programming Language", Snippet: "Official site"},
					},
				}, "stub", nil
			}
			kiroShadowWebFetchExecutor = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
				require.Equal(t, "https://example.com/doc", req.URL)
				return &webfetch.FetchResult{
					RequestedURL: req.URL,
					FinalURL:     req.URL,
					StatusCode:   http.StatusOK,
					ContentType:  "text/html",
					Title:        "Fetch Title",
					Text:         "Fetch body",
				}
			}
			t.Cleanup(func() {
				kiroShadowWebSearchExecutor = previousSearchExecutor
				kiroShadowWebFetchExecutor = previousFetchExecutor
			})

			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

			body := buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "toolUseEvent",
			}, map[string]any{"toolUseId": "tool-native", "name": tc.kiroName, "input": tc.input, "stop": true})

			result, err := svc.forwardStream(
				context.Background(),
				c,
				&Account{ID: 1011, Platform: PlatformKiro, Type: AccountTypeOAuth},
				&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
				&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
				&kiropkg.ConvertResult{
					Model: "claude-sonnet-4.6",
					ToolMetadata: &kiropkg.ToolMetadata{ResponseTools: map[string]kiropkg.ResponseToolBridge{
						tc.kiroName: {AnthropicType: tc.anthropicType, AnthropicName: tc.name, Family: tc.family},
					}},
				},
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
			require.Contains(t, output, fmt.Sprintf(`"content_block":{"id":"tool-native","input":{},"name":"%s","type":"server_tool_use"}`, tc.name))
			require.Contains(t, output, fmt.Sprintf(`"delta":{"partial_json":%q,"type":"input_json_delta"}`, tc.input))
			require.Contains(t, output, fmt.Sprintf(`"type":"%s"`, tc.resultType))
			require.Contains(t, output, `"stop_reason":"pause_turn"`)
		})
	}
}

func TestKiroGatewayService_ForwardStream_NativeWebSearchLocalFallbackFailureEmitsProperResultBlock(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		require.Equal(t, "golang", query)
		return nil, "", errors.New("search backend unavailable")
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-native-search", "name": "web_search", "input": `{"query":"golang"}`, "stop": true})

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 1016, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.6",
			ToolMetadata: &kiropkg.ToolMetadata{ResponseTools: map[string]kiropkg.ResponseToolBridge{
				"web_search": {AnthropicType: "web_search_20250305", AnthropicName: "web_search", Family: "anthropic_web_search"},
			}},
		},
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
	require.Contains(t, output, `"type":"server_tool_use"`)
	require.Contains(t, output, `"name":"web_search"`)
	require.Contains(t, output, `"type":"web_search_tool_result"`)
	require.Contains(t, output, "No search results found (emulation unavailable)")
	require.Contains(t, output, `"stop_reason":"tool_use"`)
	require.NotContains(t, output, `"cc_srv_web_search"`)
	require.NotContains(t, output, `_shadow_bridge`)
}

func TestKiroLegacyShadowToolNameForBridgeOnlyAllowsKnownWebTools(t *testing.T) {
	require.Equal(t, kiropkg.ShadowToolWebSearch, kiroLegacyShadowToolNameForBridge(nil, kiropkg.ShadowToolBridge{AnthropicType: "web_search_20250305"}))
	require.Equal(t, kiropkg.ShadowToolWebFetch, kiroLegacyShadowToolNameForBridge(nil, kiropkg.ShadowToolBridge{AnthropicName: "web_fetch"}))
	require.Empty(t, kiroLegacyShadowToolNameForBridge(nil, kiropkg.ShadowToolBridge{}))
	require.Empty(t, kiroLegacyShadowToolNameForBridge(&kiroToolState{Name: "Read"}, kiropkg.ShadowToolBridge{AnthropicName: "Read"}))
}

func TestKiroGatewayService_ForwardNonStream_NativeWebSearchToolUseExecutesLocalFallback(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		require.Equal(t, int64(1012), account.ID)
		require.Equal(t, "golang", query)
		return &websearch.SearchResponse{
			Query: query,
			Results: []websearch.SearchResult{
				{URL: "https://go.dev", Title: "The Go Programming Language", Snippet: "Official site"},
			},
		}, "stub", nil
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-native-search", "name": "web_search", "input": `{"query":"golang"}`, "stop": true})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 1012, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.6",
			ToolMetadata: &kiropkg.ToolMetadata{ResponseTools: map[string]kiropkg.ResponseToolBridge{
				"web_search": {AnthropicType: "web_search_20250305", AnthropicName: "web_search", Family: "anthropic_web_search"},
			}},
		},
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
	require.Contains(t, output, `"type":"server_tool_use"`)
	require.Contains(t, output, `"name":"web_search"`)
	require.Contains(t, output, `"type":"web_search_tool_result"`)
	require.Contains(t, output, `"url":"https://go.dev"`)
	require.Contains(t, output, `"stop_reason":"pause_turn"`)
	require.NotContains(t, output, `"cc_srv_web_search"`)
}

func TestKiroGatewayService_ForwardNonStream_NativeWebSearchLocalFallbackFailureEmitsProperResultBlock(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		require.Equal(t, "golang", query)
		return nil, "", errors.New("search backend unavailable")
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-native-search", "name": "web_search", "input": `{"query":"golang"}`, "stop": true})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 1013, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.6",
			ToolMetadata: &kiropkg.ToolMetadata{ResponseTools: map[string]kiropkg.ResponseToolBridge{
				"web_search": {AnthropicType: "web_search_20250305", AnthropicName: "web_search", Family: "anthropic_web_search"},
			}},
		},
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
	require.Contains(t, output, `"type":"server_tool_use"`)
	require.Contains(t, output, `"name":"web_search"`)
	require.Contains(t, output, `"type":"web_search_tool_result"`)
	require.Contains(t, output, "No search results found (emulation unavailable)")
	require.Contains(t, output, `"stop_reason":"tool_use"`)
	require.NotContains(t, output, `"cc_srv_web_search"`)
	require.NotContains(t, output, `_shadow_bridge`)
}

func TestKiroGatewayService_ForwardNonStream_NativeWebFetchToolUseExecutesLocalFallback(t *testing.T) {
	setGinTestMode()

	previousFetchExecutor := kiroShadowWebFetchExecutor
	kiroShadowWebFetchExecutor = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
		require.Equal(t, int64(1014), account.ID)
		require.Equal(t, "https://example.com/doc", req.URL)
		return &webfetch.FetchResult{
			RequestedURL: req.URL,
			FinalURL:     req.URL,
			StatusCode:   http.StatusOK,
			ContentType:  "text/html",
			Title:        "Fetch Title",
			Text:         "Hello from native fetch fallback",
		}
	}
	t.Cleanup(func() { kiroShadowWebFetchExecutor = previousFetchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	svc := &KiroGatewayService{
		fakeCache:      newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
		settingService: NewSettingService(nil, cfg),
	}
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-native-fetch", "name": "web_fetch", "input": `{"url":"https://example.com/doc"}`, "stop": true})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 1014, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.6",
			ToolMetadata: &kiropkg.ToolMetadata{ResponseTools: map[string]kiropkg.ResponseToolBridge{
				"web_fetch": {AnthropicType: "web_fetch_20260318", AnthropicName: "web_fetch", Family: "anthropic_web_fetch"},
			}},
		},
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
	require.Contains(t, output, `"type":"server_tool_use"`)
	require.Contains(t, output, `"name":"web_fetch"`)
	require.Contains(t, output, `"type":"web_fetch_tool_result"`)
	require.Contains(t, output, `"Hello from native fetch fallback"`)
	require.Contains(t, output, `"stop_reason":"pause_turn"`)
	require.NotContains(t, output, `"cc_srv_web_fetch"`)
}

func TestKiroGatewayService_ForwardNonStream_NativeWebFetchLocalFallbackFailureUsesLegacyShadowTool(t *testing.T) {
	setGinTestMode()

	previousFetchExecutor := kiroShadowWebFetchExecutor
	kiroShadowWebFetchExecutor = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
		require.Equal(t, "https://example.com/doc", req.URL)
		return nil
	}
	t.Cleanup(func() { kiroShadowWebFetchExecutor = previousFetchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}
	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-native-fetch", "name": "web_fetch", "input": `{"url":"https://example.com/doc"}`, "stop": true})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 1015, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.6",
			ToolMetadata: &kiropkg.ToolMetadata{ResponseTools: map[string]kiropkg.ResponseToolBridge{
				"web_fetch": {AnthropicType: "web_fetch_20260318", AnthropicName: "web_fetch", Family: "anthropic_web_fetch"},
			}},
		},
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
	require.Contains(t, output, `"type":"tool_use"`)
	require.Contains(t, output, `"name":"cc_srv_web_fetch"`)
	require.Contains(t, output, `"_shadow_bridge"`)
	require.NotContains(t, output, `"type":"web_fetch_tool_result"`)
	require.Contains(t, output, `"stop_reason":"tool_use"`)
}

func TestKiroGatewayService_ForwardNonStream_ShadowWebFetchReturnsServerToolPauseTurn(t *testing.T) {
	setGinTestMode()

	previousFetchExecutor := kiroShadowWebFetchExecutor
	kiroShadowWebFetchExecutor = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
		require.Equal(t, int64(11), account.ID)
		require.Equal(t, "https://example.com/fetch", req.URL)
		require.Equal(t, []string{"127.0.0.1", "localhost", "example.com"}, req.AllowedHosts)
		require.Equal(t, []string{"blocked.local"}, req.BlockedHosts)
		require.True(t, req.AllowPrivate)
		require.False(t, req.AllowInsecureHTTP)
		return &webfetch.FetchResult{
			RequestedURL: req.URL,
			FinalURL:     req.URL,
			StatusCode:   http.StatusOK,
			ContentType:  "text/html",
			Title:        "Fetch Title",
			Text:         "Hello from fetch",
		}
	}
	t.Cleanup(func() { kiroShadowWebFetchExecutor = previousFetchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	svc := &KiroGatewayService{
		fakeCache:      newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
		settingService: NewSettingService(nil, cfg),
	}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-2", "name": "cc_srv_web_fetch", "input": `{"url":"https://example.com/fetch"}`, "stop": true})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 11, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_fetch": {
						AnthropicType:  "web_fetch_20250305",
						AnthropicName:  "web_fetch",
						AllowedDomains: []string{"127.0.0.1", "localhost", "example.com"},
						BlockedDomains: []string{"blocked.local"},
					},
				},
			},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"type":"server_tool_use"`)
	require.Contains(t, rec.Body.String(), `"type":"web_fetch_tool_result"`)
	require.Contains(t, rec.Body.String(), `"type":"web_fetch_result"`)
	require.Contains(t, rec.Body.String(), `"title":"Fetch Title"`)
	require.Contains(t, rec.Body.String(), `"stop_reason":"pause_turn"`)
}

func TestKiroGatewayService_ForwardNonStream_ShadowWebFetchFailureReturnsStructuredToolError(t *testing.T) {
	setGinTestMode()

	previousFetchExecutor := kiroShadowWebFetchExecutor
	kiroShadowWebFetchExecutor = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
		require.Equal(t, int64(111), account.ID)
		require.Equal(t, "https://example.com/fetch", req.URL)
		return &webfetch.FetchResult{
			RequestedURL: req.URL,
			Error: &webfetch.FetchError{
				Code:    webfetch.ErrorCodeRequestFailed,
				Message: "dial tcp: i/o timeout",
			},
		}
	}
	t.Cleanup(func() { kiroShadowWebFetchExecutor = previousFetchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-err", "name": "cc_srv_web_fetch", "input": `{"url":"https://example.com/fetch"}`, "stop": true})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 111, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_fetch": {
						AnthropicType: "web_fetch_20250305",
						AnthropicName: "web_fetch",
					},
				},
			},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"type":"web_fetch_tool_error"`)
	require.Contains(t, rec.Body.String(), `"error_code":"url_not_accessible"`)
	require.Contains(t, rec.Body.String(), `"stop_reason":"pause_turn"`)
}

func TestKiroGatewayService_ForwardNonStream_ShadowWebFetchTruncatesToMaxContentTokens(t *testing.T) {
	setGinTestMode()

	previousFetchExecutor := kiroShadowWebFetchExecutor
	kiroShadowWebFetchExecutor = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
		return &webfetch.FetchResult{
			RequestedURL: req.URL,
			FinalURL:     req.URL,
			StatusCode:   http.StatusOK,
			ContentType:  "text/html",
			Title:        "Fetch Title",
			Text:         "alpha beta gamma delta epsilon zeta eta theta",
		}
	}
	t.Cleanup(func() { kiroShadowWebFetchExecutor = previousFetchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := buildKiroTestFrame(t, map[string]string{
		":message-type": "event",
		":event-type":   "toolUseEvent",
	}, map[string]any{"toolUseId": "tool-trunc", "name": "cc_srv_web_fetch", "input": `{"url":"https://example.com/fetch"}`, "stop": true})

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 112, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_fetch": {
						AnthropicType:    "web_fetch_20250305",
						AnthropicName:    "web_fetch",
						MaxContentTokens: 4,
					},
				},
			},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"type":"web_fetch_result"`)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	content, _ := payload["content"].([]any)
	resultBlock, _ := content[1].(map[string]any)
	fetchContent, _ := resultBlock["content"].(map[string]any)
	text, _ := fetchContent["text"].(string)
	require.LessOrEqual(t, kiropkg.AccurateTokenCount(text), 4)
	document, _ := fetchContent["document"].(map[string]any)
	source, _ := document["source"].(map[string]any)
	require.Equal(t, text, source["data"])
}

func TestKiroGatewayService_ForwardNonStream_ShadowWebToolsExceedMaxUsesReturnsError(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	searchCalls := 0
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		searchCalls++
		return &websearch.SearchResponse{
			Query:   query,
			Results: []websearch.SearchResult{{URL: "https://go.dev", Title: "Go"}},
		}, "stub", nil
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-shadow-1", "name": "cc_srv_web_search", "input": `{"query":"golang"}`, "stop": true}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-shadow-2", "name": "cc_srv_web_search", "input": `{"query":"kiro"}`, "stop": true}),
	}, nil)

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 113, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_search": {
						AnthropicType: "web_search_20250305",
						AnthropicName: "web_search",
						MaxUses:       1,
					},
				},
			},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "max_uses exceeded")
	require.Equal(t, 1, searchCalls)
}

func TestKiroGatewayService_ForwardNonStream_ShadowWebFetchPreservesContextWindowStopReason(t *testing.T) {
	setGinTestMode()

	previousFetchExecutor := kiroShadowWebFetchExecutor
	kiroShadowWebFetchExecutor = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
		return &webfetch.FetchResult{
			RequestedURL: req.URL,
			FinalURL:     req.URL,
			StatusCode:   http.StatusOK,
			ContentType:  "text/html",
			Title:        "Fetch Title",
			Text:         "Hello from fetch",
		}
	}
	t.Cleanup(func() { kiroShadowWebFetchExecutor = previousFetchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "contextUsageEvent",
		}, map[string]any{"contextUsagePercentage": 100}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-ctx-fetch", "name": "cc_srv_web_fetch", "input": `{"url":"https://example.com/fetch"}`, "stop": true}),
	}, nil)

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 211, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_fetch": {AnthropicType: "web_fetch_20250305", AnthropicName: "web_fetch"},
				},
			},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"stop_reason":"model_context_window_exceeded"`)
	require.NotContains(t, rec.Body.String(), `"stop_reason":"pause_turn"`)
}

func TestKiroGatewayService_ForwardNonStream_NormalToolBeforeShadowToolKeepsToolUseStopPath(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-normal", "name": "search", "input": `{"q":"normal"}`, "stop": true}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-shadow", "name": "cc_srv_web_search", "input": `{"query":"golang"}`, "stop": true}),
	}, nil)

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 12, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_search": {AnthropicType: "web_search_20250305", AnthropicName: "web_search"},
				},
			},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"stop_reason":"tool_use"`)
	require.Contains(t, rec.Body.String(), `"name":"search"`)
	require.NotContains(t, rec.Body.String(), `"type":"server_tool_use"`)
}

func TestKiroGatewayService_ForwardNonStream_ShadowToolFollowedByNormalToolReturnsConflict(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-shadow", "name": "cc_srv_web_search", "input": `{"query":"golang"}`, "stop": true}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-normal", "name": "search", "input": `{"q":"normal"}`, "stop": true}),
	}, nil)

	result, err := svc.forwardNonStream(
		context.Background(),
		c,
		&Account{ID: 13, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Body: NewRequestBodyRef([]byte(`{}`))},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_search": {AnthropicType: "web_search_20250305", AnthropicName: "web_search"},
				},
			},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "shadow web tool conflict")
}

func TestKiroGatewayService_ForwardStream_NormalToolBeforeShadowToolKeepsToolUseStopPath(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-normal", "name": "search", "input": `{"q":"normal"}`, "stop": true}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-shadow", "name": "cc_srv_web_search", "input": `{"query":"golang"}`, "stop": true}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 14, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_search": {AnthropicType: "web_search_20250305", AnthropicName: "web_search"},
				},
			},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"type":"tool_use"`)
	require.Contains(t, rec.Body.String(), `"name":"search"`)
	require.Contains(t, rec.Body.String(), `"stop_reason":"tool_use"`)
	require.NotContains(t, rec.Body.String(), `"type":"server_tool_use"`)
}

func TestKiroGatewayService_ForwardStream_ShadowToolFollowedByNormalToolReturnsConflict(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		require.Equal(t, int64(15), account.ID)
		require.Equal(t, "golang", query)
		return &websearch.SearchResponse{
			Query: query,
			Results: []websearch.SearchResult{
				{URL: "https://go.dev", Title: "The Go Programming Language", Snippet: "Official site"},
			},
		}, "stub", nil
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &KiroGatewayService{fakeCache: newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries)}

	body := bytes.Join([][]byte{
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-shadow", "name": "cc_srv_web_search", "input": `{"query":"golang"}`, "stop": true}),
		buildKiroTestFrame(t, map[string]string{
			":message-type": "event",
			":event-type":   "toolUseEvent",
		}, map[string]any{"toolUseId": "tool-normal", "name": "search", "input": `{"q":"normal"}`, "stop": true}),
	}, nil)

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 15, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{
			Model: "claude-sonnet-4.5",
			BridgeMetadata: &kiropkg.BridgeMetadata{
				ShadowTools: map[string]kiropkg.ShadowToolBridge{
					"cc_srv_web_search": {AnthropicType: "web_search_20250305", AnthropicName: "web_search"},
				},
			},
		},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, err.Error(), "shadow web tool conflict")
	require.Contains(t, rec.Body.String(), `"type":"server_tool_use"`)
	require.Contains(t, rec.Body.String(), "event: error")
	require.NotContains(t, rec.Body.String(), "event: message_stop")
}

func TestGatewayForwardAsResponses_KiroWebSearchPauseTurnReturnsResponsesCall(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		require.Equal(t, "golang", query)
		return &websearch.SearchResponse{
			Query: query,
			Results: []websearch.SearchResult{
				{URL: "https://go.dev", Title: "The Go Programming Language", Snippet: "Official site"},
			},
		}, "stub", nil
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"claude-sonnet-4.5","stream":false,"input":"Search for Go","tools":[{"type":"web_search"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "toolUseEvent",
			}, map[string]any{"toolUseId": "srvtoolu_search_3", "name": "web_search", "input": `{"query":"golang"}`, "stop": true}))),
		},
	}

	svc := &GatewayService{
		kiroGatewayService: &KiroGatewayService{
			httpUpstream: upstream,
			fakeCache:    newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
		},
	}
	account := &Account{
		ID:       12,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}

	result, err := svc.ForwardAsResponses(context.Background(), c, account, body, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.req)
	sentBody, err := io.ReadAll(upstream.req.Body)
	require.NoError(t, err)
	require.Contains(t, string(sentBody), `"name":"web_search"`)
	require.NotContains(t, string(sentBody), `"name":"cc_srv_web_search"`)
	require.Contains(t, rec.Body.String(), `"type":"web_search_call"`)
	require.Contains(t, rec.Body.String(), `"query":"golang"`)
	require.Contains(t, rec.Body.String(), `"url":"https://go.dev"`)
	require.NotContains(t, rec.Body.String(), `"type":"function_call"`)
}

func TestKiroGatewayService_Forward_ContinuationReplaySendsNativeWebSearchHistoryToKiro(t *testing.T) {
	setGinTestMode()

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
		ID:       301,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}
	parsed := &ParsedRequest{
		Model: "claude-sonnet-4",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4",
			"tools":[{"type":"web_search_20250305","name":"web_search"}],
			"messages":[
				{"role":"user","content":[{"type":"text","text":"Search for Go"}]},
				{"role":"assistant","content":[{"type":"server_tool_use","id":"toolu_search_1","name":"web_search","input":{"query":"golang"}}]},
				{"role":"user","content":[
					{"type":"web_search_tool_result","tool_use_id":"toolu_search_1","content":[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]},
					{"type":"text","text":"Summarize the result"}
				]}
			]
		}`)),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, upstream.calls)
	sentBody, readErr := io.ReadAll(upstream.req.Body)
	require.NoError(t, readErr)
	body := string(sentBody)
	require.Contains(t, body, `"name":"web_search"`)
	require.NotContains(t, body, `"name":"cc_srv_web_search"`)
	require.Contains(t, body, `"toolUseId":"toolu_search_1"`)
	require.Contains(t, body, `"toolResults"`)
	require.NotContains(t, body, `"server_tool_use"`)
	require.NotContains(t, body, `"web_search_tool_result"`)
	require.Contains(t, rec.Body.String(), `"text":"final answer","type":"text"`)
}

func TestKiroGatewayService_Forward_RejectsUnsupportedServerToolFamilies(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{}
	svc := &KiroGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:       302,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}
	parsed := &ParsedRequest{
		Model: "claude-sonnet-4",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4",
			"tools":[{"type":"computer_20250124","name":"computer","display_width_px":1024,"display_height_px":768}],
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`)),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"invalid_request_error"`)
	require.Contains(t, rec.Body.String(), "unsupported server-side tool family")
	require.Zero(t, upstream.calls)
}

func TestKiroGatewayService_Forward_ContinuationWithoutToolsRestoresNativeWebSearch(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		require.Equal(t, "golang 1.23", query)
		return &websearch.SearchResponse{
			Query: query,
			Results: []websearch.SearchResult{
				{URL: "https://go.dev/doc/go1.23", Title: "Go 1.23 Release Notes", Snippet: "Release notes"},
			},
		}, "stub", nil
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "toolUseEvent",
			}, map[string]any{"toolUseId": "toolu_search_2", "name": "web_search", "input": `{"query":"golang 1.23"}`, "stop": true}))),
		},
	}
	svc := &KiroGatewayService{
		httpUpstream: upstream,
		fakeCache:    newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}
	account := &Account{
		ID:       303,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}
	parsed := &ParsedRequest{
		Model: "claude-sonnet-4",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4",
			"messages":[
				{"role":"user","content":[{"type":"text","text":"Search for Go"}]},
				{"role":"assistant","content":[{"type":"server_tool_use","id":"toolu_search_1","name":"web_search","input":{"query":"golang"}}]},
				{"role":"user","content":[
					{"type":"web_search_tool_result","tool_use_id":"toolu_search_1","content":[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]},
					{"type":"text","text":"Search Go 1.23 next"}
				]}
			]
		}`)),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	sentBody, readErr := io.ReadAll(upstream.req.Body)
	require.NoError(t, readErr)
	require.Contains(t, string(sentBody), `"name":"web_search"`)
	require.NotContains(t, string(sentBody), `"name":"cc_srv_web_search"`)
	require.Contains(t, rec.Body.String(), `"type":"server_tool_use"`)
	require.Contains(t, rec.Body.String(), `"type":"web_search_tool_result"`)
	require.Contains(t, rec.Body.String(), `"url":"https://go.dev/doc/go1.23"`)
	require.Contains(t, rec.Body.String(), `"stop_reason":"pause_turn"`)
}

func TestKiroGatewayService_Forward_ContinuationWithoutToolsRestoresNativeWebFetch(t *testing.T) {
	setGinTestMode()

	previousFetchExecutor := kiroShadowWebFetchExecutor
	kiroShadowWebFetchExecutor = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
		require.Equal(t, "https://docs.example.com/next", req.URL)
		return &webfetch.FetchResult{
			RequestedURL: req.URL,
			FinalURL:     req.URL,
			StatusCode:   http.StatusOK,
			ContentType:  "text/html",
			Title:        "Next Docs",
			Text:         "next docs body",
		}
	}
	t.Cleanup(func() { kiroShadowWebFetchExecutor = previousFetchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "toolUseEvent",
			}, map[string]any{"toolUseId": "toolu_fetch_2", "name": "web_fetch", "input": `{"url":"https://docs.example.com/next"}`, "stop": true}))),
		},
	}
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	svc := &KiroGatewayService{
		httpUpstream:   upstream,
		fakeCache:      newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
		settingService: NewSettingService(nil, cfg),
	}
	account := &Account{
		ID:       304,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}
	parsed := &ParsedRequest{
		Model: "claude-sonnet-4",
		Body: NewRequestBodyRef([]byte(`{
			"model":"claude-sonnet-4",
			"messages":[
				{"role":"user","content":[{"type":"text","text":"Fetch docs"}]},
				{"role":"assistant","content":[
					{
						"type":"server_tool_use",
						"id":"toolu_fetch_1",
						"name":"web_fetch",
						"input":{"url":"https://example.com/start"},
						"allowed_domains":["example.com","docs.example.com"],
						"blocked_domains":["blocked.example.com"],
						"max_uses":2,
						"max_content_tokens":4096
					}
				]},
				{"role":"user","content":[
					{
						"type":"web_fetch_tool_result",
						"tool_use_id":"toolu_fetch_1",
						"content":{
							"type":"web_fetch_result",
							"url":"https://example.com/start",
							"text":"start body"
						}
					},
					{"type":"text","text":"Fetch the docs page next"}
				]}
			]
		}`)),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	sentBody, readErr := io.ReadAll(upstream.req.Body)
	require.NoError(t, readErr)
	require.Contains(t, string(sentBody), `"name":"web_fetch"`)
	require.NotContains(t, string(sentBody), `"name":"cc_srv_web_fetch"`)
	require.Contains(t, string(sentBody), `"toolResults"`)
	require.Contains(t, rec.Body.String(), `"type":"server_tool_use"`)
	require.Contains(t, rec.Body.String(), `"type":"web_fetch_tool_result"`)
	require.Contains(t, rec.Body.String(), `"next docs body"`)
	require.Contains(t, rec.Body.String(), `"stop_reason":"pause_turn"`)
}

func TestKiroGatewayService_ForwardStream_NativeWebSearchContinuesToFinalAnswer(t *testing.T) {
	setGinTestMode()

	previousSearchExecutor := kiroShadowWebSearchExecutor
	kiroShadowWebSearchExecutor = func(ctx context.Context, account *Account, query string) (*websearch.SearchResponse, string, error) {
		require.Equal(t, "golang", query)
		return &websearch.SearchResponse{
			Query: query,
			Results: []websearch.SearchResult{
				{URL: "https://go.dev", Title: "The Go Programming Language", Snippet: "Official Go site"},
			},
		}, "stub", nil
	}
	t.Cleanup(func() { kiroShadowWebSearchExecutor = previousSearchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{
		"model":"claude-sonnet-4",
		"stream":true,
		"max_tokens":1024,
		"tool_choice":{"type":"tool","name":"web_search"},
		"tools":[{"type":"web_search_20250305","name":"web_search"}],
		"messages":[{"role":"user","content":"Search Go"}]
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	var continuationBody string
	upstream := &kiroHTTPUpstreamRecorder{}
	upstream.doFunc = func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		if upstream.calls == 1 {
			require.Contains(t, string(raw), `"name":"web_search"`)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"x-amzn-requestid": []string{"first"}},
				Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
					":message-type": "event",
					":event-type":   "toolUseEvent",
				}, map[string]any{"toolUseId": "toolu_search_1", "name": "web_search", "input": `{"query":"golang"}`, "stop": true}))),
			}, nil
		}
		require.Equal(t, 2, upstream.calls, "native web_search should be replayed once as a continuation")
		continuationBody = string(raw)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"x-amzn-requestid": []string{"second"}},
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "assistantResponseEvent",
			}, map[string]any{"content": "Go's official site is https://go.dev."}))),
		}, nil
	}

	svc := &KiroGatewayService{
		httpUpstream: upstream,
		fakeCache:    newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}
	account := &Account{
		ID:       305,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}
	parsed := &ParsedRequest{
		Model:  "claude-sonnet-4",
		Stream: true,
		Body:   NewRequestBodyRef(body),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, upstream.calls)
	require.Contains(t, continuationBody, `"toolUses"`)
	require.Contains(t, continuationBody, `"toolResults"`)
	require.NotContains(t, continuationBody, `"tool_choice"`)
	output := rec.Body.String()
	require.Contains(t, output, `"type":"server_tool_use"`)
	require.Contains(t, output, `"type":"web_search_tool_result"`)
	require.Contains(t, output, `Go's official site is https`)
	require.Contains(t, output, `://go.dev.`)
	require.Contains(t, output, `"stop_reason":"end_turn"`)
	require.NotContains(t, output, `"stop_reason":"pause_turn"`)
}

func TestKiroGatewayService_ForwardStream_NativeWebFetchContinuesToFinalAnswer(t *testing.T) {
	setGinTestMode()

	previousFetchExecutor := kiroShadowWebFetchExecutor
	kiroShadowWebFetchExecutor = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
		require.Equal(t, "https://example.com/doc", req.URL)
		return &webfetch.FetchResult{
			RequestedURL: req.URL,
			FinalURL:     req.URL,
			StatusCode:   http.StatusOK,
			ContentType:  "text/html",
			Title:        "Example Docs",
			Text:         "Example docs body",
		}
	}
	t.Cleanup(func() { kiroShadowWebFetchExecutor = previousFetchExecutor })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{
		"model":"claude-sonnet-4",
		"stream":true,
		"max_tokens":1024,
		"tool_choice":{"type":"tool","name":"web_fetch"},
		"tools":[{"type":"web_fetch_20250910","name":"web_fetch"}],
		"messages":[{"role":"user","content":"Fetch the docs page"}]
	}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	var continuationBody string
	upstream := &kiroHTTPUpstreamRecorder{}
	upstream.doFunc = func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		if upstream.calls == 1 {
			require.Contains(t, string(raw), `"name":"web_fetch"`)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"x-amzn-requestid": []string{"first-fetch"}},
				Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
					":message-type": "event",
					":event-type":   "toolUseEvent",
				}, map[string]any{"toolUseId": "toolu_fetch_1", "name": "web_fetch", "input": `{"url":"https://example.com/doc"}`, "stop": true}))),
			}, nil
		}
		require.Equal(t, 2, upstream.calls, "native web_fetch should be replayed once as a continuation")
		continuationBody = string(raw)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"x-amzn-requestid": []string{"second-fetch"}},
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "assistantResponseEvent",
			}, map[string]any{"content": "The fetched page is Example Docs."}))),
		}, nil
	}

	svc := &KiroGatewayService{
		httpUpstream: upstream,
		fakeCache:    newKiroFakeCache(kiropkg.DefaultFakeCacheMaxEntries),
	}
	account := &Account{
		ID:       306,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-api-key",
		},
	}
	parsed := &ParsedRequest{
		Model:  "claude-sonnet-4",
		Stream: true,
		Body:   NewRequestBodyRef(body),
	}

	result, err := svc.Forward(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, upstream.calls)
	require.Contains(t, continuationBody, `"toolUses"`)
	require.Contains(t, continuationBody, `"toolResults"`)
	require.NotContains(t, continuationBody, `"tool_choice"`)
	output := rec.Body.String()
	require.Contains(t, output, `"type":"server_tool_use"`)
	require.Contains(t, output, `"type":"web_fetch_tool_result"`)
	require.Contains(t, output, `The fetched page is`)
	require.Contains(t, output, `mple Docs.`)
	require.Contains(t, output, `"stop_reason":"end_turn"`)
	require.NotContains(t, output, `"stop_reason":"pause_turn"`)
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
