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
		Model: "claude-opus-4-7",
		Body: []byte(`{
			"model":"claude-opus-4-7",
			"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
		}`),
	})

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "unsupported kiro model")
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
	require.JSONEq(t, `{"input_tokens":6}`, rec.Body.String())
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
	)

	require.Error(t, err)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Body.String())
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
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "event: message_start")
	require.Contains(t, rec.Body.String(), "event: error")
	require.NotContains(t, rec.Body.String(), "event: message_delta")
	require.NotContains(t, rec.Body.String(), "event: message_stop")
	_, found := svc.fakeCache.Get(fakeCachePlan.CurrentKey)
	require.False(t, found, "exception streams must not commit fake cache")
}

func TestKiroGatewayService_ForwardStream_PreStartExceptionReturnsJSONError(t *testing.T) {
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
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "upstream failed before stream")
	require.NotContains(t, rec.Body.String(), "event: message_start")
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")
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
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
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
	}, map[string]any{"contextUsagePercentage": 30})

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
	)

	require.Error(t, err)
	require.Nil(t, result)
	require.NotContains(t, rec.Body.String(), "event: message_start")
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
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, rec.Body.String(), `"index":0`)
	require.Contains(t, rec.Body.String(), `"index":1`)
	require.NotContains(t, rec.Body.String(), `"index":-1`)
	require.Less(t, strings.Index(rec.Body.String(), `"index":0`), strings.Index(rec.Body.String(), `"index":1`))
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
