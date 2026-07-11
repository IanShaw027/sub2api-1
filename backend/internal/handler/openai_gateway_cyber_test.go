package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newTestGinContext builds a bare gin.Context backed by an httptest recorder.
func newTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c
}

// TestRecordCyberPolicyIfMarked_NoMark verifies that when no cyber mark is set,
// the function returns immediately and does NOT set the recorded flag.
func TestRecordCyberPolicyIfMarked_NoMark(t *testing.T) {
	c := newTestGinContext()
	h := &OpenAIGatewayHandler{}

	h.recordCyberPolicyIfMarked(c, nil, nil, nil, "gpt-5", true, "", service.ChannelUsageFields{}, "", "", nil)

	// Flag must NOT be set when there was no mark.
	require.False(t, c.GetBool(cyberPolicyRecordedKey),
		"cyberPolicyRecordedKey must remain false when no cyber mark is present")
}

// TestRecordCyberPolicyIfMarked_WithMark verifies that:
//  1. When a cyber mark is present, the recorded flag is set (guard activated).
//  2. A second call is a no-op (idempotent guard).
//  3. Nil services do not panic.
func TestRecordCyberPolicyIfMarked_WithMark(t *testing.T) {
	c := newTestGinContext()
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "flagged",
		Body:           `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus: 400,
	})

	h := &OpenAIGatewayHandler{} // nil services — must not panic

	// First call: should set the flag.
	require.NotPanics(t, func() {
		h.recordCyberPolicyIfMarked(c, nil, nil, nil, "gpt-5", true, "", service.ChannelUsageFields{}, "", "", nil)
	})
	require.True(t, c.GetBool(cyberPolicyRecordedKey),
		"cyberPolicyRecordedKey must be true after first call with a mark")

	// Second call: flag already set — must be a no-op (idempotent).
	require.NotPanics(t, func() {
		h.recordCyberPolicyIfMarked(c, nil, nil, nil, "gpt-5", false, "", service.ChannelUsageFields{}, "", "", nil)
	})
	// Flag should still be true (not toggled or cleared).
	require.True(t, c.GetBool(cyberPolicyRecordedKey),
		"cyberPolicyRecordedKey must remain true after second call (guard)")
}

// TestRecordCyberPolicyIfMarked_ForwardSuccessSkipsUsageLog verifies the semantic:
// when forwardErrored=false the function still sets the guard flag (mark present),
// but the cyber usage row is NOT requested (only RecordCyberPolicyEvent fires).
// Since services are nil here we only verify the guard flag and no panic.
func TestRecordCyberPolicyIfMarked_ForwardSuccessSkipsUsageLog(t *testing.T) {
	c := newTestGinContext()
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "flagged",
		UpstreamStatus: 200,
	})

	h := &OpenAIGatewayHandler{}

	require.NotPanics(t, func() {
		h.recordCyberPolicyIfMarked(c, nil, nil, nil, "gpt-5", false /* forwardErrored=false */, "", service.ChannelUsageFields{}, "", "", nil)
	})
	require.True(t, c.GetBool(cyberPolicyRecordedKey))
}

// TestClearCyberPolicyTurnState verifies F1 at the handler level: after a turn
// is finalized, both the mark and the recorded guard are reset so the next WS
// turn detects/records independently.
func TestClearCyberPolicyTurnState(t *testing.T) {
	c := newTestGinContext()
	h := &OpenAIGatewayHandler{}

	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{Message: "turn1", UpstreamStatus: 200})
	h.recordCyberPolicyIfMarked(c, nil, nil, nil, "gpt-5", false, "", service.ChannelUsageFields{}, "", "", nil)
	require.True(t, c.GetBool(cyberPolicyRecordedKey))

	clearCyberPolicyTurnState(c)
	require.Nil(t, service.GetOpsCyberPolicy(c))
	require.False(t, c.GetBool(cyberPolicyRecordedKey))

	// turn2: a fresh cyber hit must be recordable again.
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{Message: "turn2", UpstreamStatus: 200})
	h.recordCyberPolicyIfMarked(c, nil, nil, nil, "gpt-5", false, "", service.ChannelUsageFields{}, "", "", nil)
	require.True(t, c.GetBool(cyberPolicyRecordedKey))
	require.Equal(t, "turn2", service.GetOpsCyberPolicy(c).Message)
}

// TestBuildCyberSessionBlockedOpsEntry verifies the locally-rejected request is
// auditable: 403 / phase=request / type=cyber_policy_session_blocked — distinct
// from upstream cyber_policy hits, and it must NOT touch moderation/violation.
func TestBuildCyberSessionBlockedOpsEntry(t *testing.T) {
	entry := buildCyberSessionBlockedOpsEntry(cyberPolicyOpsErrorMeta{
		RequestID: "req-9", Model: "gpt-5", RequestPath: "/openai/v1/responses",
	})
	require.Equal(t, 403, entry.StatusCode)
	require.Equal(t, "cyber_policy_session_blocked", entry.ErrorType)
	require.Equal(t, "request", entry.ErrorPhase)
	require.True(t, entry.IsBusinessLimited)
	require.Equal(t, "gateway_local", entry.ErrorSource)
	require.Equal(t, "platform", entry.ErrorOwner)
	require.Empty(t, entry.ErrorBody, "no session block key → ErrorBody must be empty")

	entryWithKey := buildCyberSessionBlockedOpsEntry(cyberPolicyOpsErrorMeta{
		RequestID: "req-9", Model: "gpt-5", RequestPath: "/openai/v1/responses",
		SessionBlockKey: "abc123",
	})
	require.Equal(t, "session_block_key=abc123", entryWithKey.ErrorBody)
}

// TestRejectIfCyberSessionBlocked_FailOpen verifies fail-open paths: nil handler
// services, no explicit session signal, and (implicitly) disabled switch all
// pass the request through.
func TestRejectIfCyberSessionBlocked_FailOpen(t *testing.T) {
	c := newTestGinContext()
	c.Request = httptest.NewRequest("POST", "/openai/v1/responses", strings.NewReader(`{}`))

	h := &OpenAIGatewayHandler{}
	require.False(t, h.rejectIfCyberSessionBlocked(c, nil, []byte(`{}`), "gpt-5", cyberBlockFormatResponses), "nil apiKey → pass")

	h2 := &OpenAIGatewayHandler{gatewayService: nil}
	key := &service.APIKey{ID: 1}
	require.False(t, h2.rejectIfCyberSessionBlocked(c, key, []byte(`{}`), "gpt-5", cyberBlockFormatResponses), "nil gateway service → pass")
}

func TestRejectIfCyberSessionBlocked_CompactHeartbeatTerminatesSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5","prompt_cache_key":"blocked-compact-session","input":[{"type":"compaction_trigger"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(string(body)))

	cache := &handlerCyberCacheStoreStub{}
	settingSvc := service.NewSettingService(
		&handlerCyberSettingRepoStub{vals: map[string]string{
			service.SettingKeyCyberSessionBlockEnabled:    "true",
			service.SettingKeyCyberSessionBlockTTLSeconds: "60",
		}},
		&config.Config{},
	)
	gatewaySvc := service.NewOpenAIGatewayService(
		nil, nil, nil, nil, nil, nil,
		cache,
		&config.Config{},
		nil, nil, nil, nil,
		&service.BillingCacheService{},
		nil, &service.DeferredService{},
		nil, nil, nil, nil, nil, nil,
		settingSvc,
		nil, nil,
	)
	apiKey := &service.APIKey{ID: 22, Key: "sk-compact-test", User: &service.User{ID: 33}}
	key := service.CyberSessionBlockKey(apiKey.ID, c, body)
	require.NotEmpty(t, key)
	cache.blocked = map[string]bool{key: true}

	service.MarkOpenAICompactClientStream(c)
	stop := service.StartOpenAICompactSSEKeepalive(c, time.Millisecond)
	defer stop()
	waitForCompactKeepaliveCommit(t, c)

	blocked := (&OpenAIGatewayHandler{gatewayService: gatewaySvc}).rejectIfCyberSessionBlocked(
		c, apiKey, body, "gpt-5", cyberBlockFormatResponses,
	)

	require.True(t, blocked)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "event: response.failed\n")
	require.Contains(t, rec.Body.String(), cyberSessionBlockedClientMsg)
	streamErr, ok := service.GetOpsStreamError(c)
	require.True(t, ok)
	require.Equal(t, "permission_error", streamErr.ErrType)
	require.Equal(t, http.StatusForbidden, streamErr.IntendedStatus)
}

// TestRecordCyberPolicyIfMarked_BlockKeyPlumbed verifies the 6th param is
// accepted and a non-empty key with nil gateway service does not panic
// (write-side guards live in the service layer).
func TestRecordCyberPolicyIfMarked_BlockKeyPlumbed(t *testing.T) {
	c := newTestGinContext()
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{Message: "x", UpstreamStatus: 400})
	h := &OpenAIGatewayHandler{}
	require.NotPanics(t, func() {
		h.recordCyberPolicyIfMarked(c, nil, nil, nil, "gpt-5", true, "deadbeef", service.ChannelUsageFields{}, "", "", nil)
	})
}

func TestRecordCyberPolicyIfMarked_RecordsFlaggedHashesBeforeAsyncAudit(t *testing.T) {
	c := newTestGinContext()
	c.Request = httptest.NewRequest("POST", "/openai/v1/chat/completions", strings.NewReader(`{}`))
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "blocked",
		Body:           `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus: 400,
	})

	releaseAudit := make(chan struct{})
	defer close(releaseAudit)
	repo := &blockingCyberPolicyModerationRepo{
		started: make(chan struct{}),
		release: releaseAudit,
	}
	hashCache := &recordingCyberPolicyHashCache{}
	moderationSvc := service.NewContentModerationService(
		&contentModerationHandlerSettingRepo{values: map[string]string{
			service.SettingKeyRiskControlEnabled: "true",
		}},
		repo,
		hashCache,
		nil,
		nil,
		nil,
		nil,
	)
	h := &OpenAIGatewayHandler{contentModerationService: moderationSvc}
	body := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"repeat this cyber input"}]}`)

	h.recordCyberPolicyIfMarked(c, nil, nil, nil, "gpt-5", false, "", service.ChannelUsageFields{}, "", service.ContentModerationProtocolOpenAIChat, body)

	require.Eventually(t, func() bool {
		return repo.createStarted()
	}, time.Second, 10*time.Millisecond, "async audit should be blocked before it reaches hash recording")
	require.NotEmpty(t, hashCache.snapshot(), "flagged hashes must be recorded synchronously before asynchronous audit work continues")
}

func TestGatewayRecordCyberPolicyIfMarked_RecordsFlaggedHashesBeforeAsyncAudit(t *testing.T) {
	c := newTestGinContext()
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{}`))
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "blocked",
		Body:           `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus: 400,
	})

	releaseAudit := make(chan struct{})
	defer close(releaseAudit)
	repo := &blockingCyberPolicyModerationRepo{
		started: make(chan struct{}),
		release: releaseAudit,
	}
	hashCache := &recordingCyberPolicyHashCache{}
	moderationSvc := service.NewContentModerationService(
		&contentModerationHandlerSettingRepo{values: map[string]string{
			service.SettingKeyRiskControlEnabled: "true",
		}},
		repo,
		hashCache,
		nil,
		nil,
		nil,
		nil,
	)
	h := &GatewayHandler{contentModerationService: moderationSvc}
	body := []byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"repeat this compat cyber input"}]}`)

	h.recordGatewayCyberPolicyIfMarked(c, nil, nil, nil, "glm-5.2", false, "", service.ChannelUsageFields{}, service.ContentModerationProtocolOpenAIChat, body)

	require.Eventually(t, func() bool {
		return repo.createStarted()
	}, time.Second, 10*time.Millisecond, "gateway async audit should be blocked before it reaches hash recording")
	require.NotEmpty(t, hashCache.snapshot(), "gateway flagged hashes must be recorded synchronously before asynchronous audit work continues")
}

func TestRecordCyberPolicyIfMarked_BlocksSessionSynchronously(t *testing.T) {
	c := newTestGinContext()
	body := []byte(`{"model":"gpt-5","prompt_cache_key":"openai-session","messages":[{"role":"user","content":"cyber input"}]}`)
	c.Request = httptest.NewRequest("POST", "/openai/v1/chat/completions", strings.NewReader(string(body)))
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "blocked by upstream policy",
		Body:           `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus: http.StatusBadRequest,
	})

	releaseAudit := make(chan struct{})
	defer close(releaseAudit)
	repo := &blockingCyberPolicyModerationRepo{
		started: make(chan struct{}),
		release: releaseAudit,
	}
	moderationSvc := service.NewContentModerationService(
		&contentModerationHandlerSettingRepo{values: map[string]string{
			service.SettingKeyRiskControlEnabled: "true",
		}},
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	cache := &handlerCyberCacheStoreStub{}
	settingSvc := service.NewSettingService(
		&handlerCyberSettingRepoStub{
			vals: map[string]string{
				service.SettingKeyCyberSessionBlockEnabled:    "true",
				service.SettingKeyCyberSessionBlockTTLSeconds: "60",
			},
		},
		&config.Config{},
	)
	cfg := &config.Config{}
	gatewaySvc := service.NewOpenAIGatewayService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cache,
		cfg,
		nil,
		nil,
		nil,
		nil,
		&service.BillingCacheService{},
		nil,
		&service.DeferredService{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		settingSvc,
		nil,
		nil,
	)
	apiKey := &service.APIKey{ID: 22, Name: "openai key", Key: "sk-test-openai-key"}
	key := service.CyberSessionBlockKey(apiKey.ID, c, body)
	require.NotEmpty(t, key)
	h := &OpenAIGatewayHandler{gatewayService: gatewaySvc, contentModerationService: moderationSvc}

	h.recordCyberPolicyIfMarked(c, apiKey, nil, nil, "gpt-5", false, key, service.ChannelUsageFields{}, "", service.ContentModerationProtocolOpenAIChat, body)

	require.Eventually(t, func() bool {
		return repo.createStarted()
	}, time.Second, 10*time.Millisecond, "async audit should be blocked before old async session-block writes")
	require.True(t, gatewaySvc.IsCyberSessionBlocked(context.Background(), key), "session block must be visible when recordCyberPolicyIfMarked returns")
}

func TestGatewayRecordCyberPolicyIfMarked_BlocksSessionSynchronously(t *testing.T) {
	c := newTestGinContext()
	body := []byte(`{"model":"glm-5.2","prompt_cache_key":"compat-session","messages":[{"role":"user","content":"cyber input"}]}`)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(string(body)))
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "blocked by upstream policy",
		Body:           `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus: http.StatusBadRequest,
	})

	releaseAudit := make(chan struct{})
	defer close(releaseAudit)
	repo := &blockingCyberPolicyModerationRepo{
		started: make(chan struct{}),
		release: releaseAudit,
	}
	moderationSvc := service.NewContentModerationService(
		&contentModerationHandlerSettingRepo{values: map[string]string{
			service.SettingKeyRiskControlEnabled: "true",
		}},
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	cache := &handlerCyberCacheStoreStub{}
	settingSvc := service.NewSettingService(
		&handlerCyberSettingRepoStub{
			vals: map[string]string{
				service.SettingKeyCyberSessionBlockEnabled:    "true",
				service.SettingKeyCyberSessionBlockTTLSeconds: "60",
			},
		},
		&config.Config{},
	)
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	gatewaySvc := service.NewGatewayService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cache,
		cfg,
		nil,
		nil,
		service.NewBillingService(cfg, nil),
		nil,
		&service.BillingCacheService{},
		nil,
		nil,
		&service.DeferredService{},
		nil,
		nil,
		nil,
		nil,
		settingSvc,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	apiKey := &service.APIKey{ID: 22, Name: "compat key", Key: "sk-test-compat-key"}
	key := service.CyberSessionBlockKey(apiKey.ID, c, body)
	require.NotEmpty(t, key)
	h := &GatewayHandler{gatewayService: gatewaySvc, contentModerationService: moderationSvc}

	h.recordGatewayCyberPolicyIfMarked(c, apiKey, nil, nil, "glm-5.2", false, key, service.ChannelUsageFields{}, service.ContentModerationProtocolOpenAIChat, body)

	require.Eventually(t, func() bool {
		return repo.createStarted()
	}, time.Second, 10*time.Millisecond, "gateway async audit should be blocked before old async session-block writes")
	require.True(t, gatewaySvc.IsCyberSessionBlocked(context.Background(), key), "session block must be visible when recordGatewayCyberPolicyIfMarked returns")
}

func TestRecordCyberPolicyIfMarked_RecordsFlaggedHashesAfterRequestContextCanceled(t *testing.T) {
	reqCtx, cancelReq := context.WithCancel(context.Background())
	cancelReq()
	c := newTestGinContext()
	c.Request = httptest.NewRequest("POST", "/openai/v1/chat/completions", strings.NewReader(`{}`)).WithContext(reqCtx)
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "blocked",
		Body:           `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus: 400,
	})

	hashCache := &recordingCyberPolicyHashCache{respectContext: true}
	moderationSvc := service.NewContentModerationService(
		&contentModerationHandlerSettingRepo{values: map[string]string{
			service.SettingKeyRiskControlEnabled: "true",
		}},
		nil,
		hashCache,
		nil,
		nil,
		nil,
		nil,
	)
	h := &OpenAIGatewayHandler{contentModerationService: moderationSvc}
	body := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"request canceled cyber input"}]}`)

	h.recordCyberPolicyIfMarked(c, nil, nil, nil, "gpt-5", false, "", service.ChannelUsageFields{}, "", service.ContentModerationProtocolOpenAIChat, body)

	require.NotEmpty(t, hashCache.snapshot(), "flagged hashes must be recorded even if the client request context has already been canceled")
}

func TestGatewayRecordCyberPolicyIfMarked_RecordsFlaggedHashesAfterRequestContextCanceled(t *testing.T) {
	reqCtx, cancelReq := context.WithCancel(context.Background())
	cancelReq()
	c := newTestGinContext()
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{}`)).WithContext(reqCtx)
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "blocked",
		Body:           `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus: 400,
	})

	hashCache := &recordingCyberPolicyHashCache{respectContext: true}
	moderationSvc := service.NewContentModerationService(
		&contentModerationHandlerSettingRepo{values: map[string]string{
			service.SettingKeyRiskControlEnabled: "true",
		}},
		nil,
		hashCache,
		nil,
		nil,
		nil,
		nil,
	)
	h := &GatewayHandler{contentModerationService: moderationSvc}
	body := []byte(`{"model":"glm-5.2","messages":[{"role":"user","content":"request canceled compat cyber input"}]}`)

	h.recordGatewayCyberPolicyIfMarked(c, nil, nil, nil, "glm-5.2", false, "", service.ChannelUsageFields{}, service.ContentModerationProtocolOpenAIChat, body)

	require.NotEmpty(t, hashCache.snapshot(), "gateway flagged hashes must be recorded even if the client request context has already been canceled")
}

func TestGatewayRecordCyberPolicyIfMarked_EnqueuesOpsErrorLog(t *testing.T) {
	resetOpsErrorLoggerStateForTest(t)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })

	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, 1)
	opsErrorLogMu.Unlock()

	c := newTestGinContext()
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{}`))
	c.Writer.Header().Set("X-Request-Id", "req-gateway-cyber")
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "blocked by upstream policy",
		Body:           `{"error":{"code":"cyber_policy","message":"blocked by upstream policy"}}`,
		UpstreamStatus: http.StatusBadRequest,
		UpstreamInTok:  11,
		UpstreamOutTok: 7,
	})

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h := &GatewayHandler{opsService: ops}

	h.recordGatewayCyberPolicyIfMarked(c, nil, nil, nil, "glm-5.2", false, "", service.ChannelUsageFields{}, service.ContentModerationProtocolOpenAIChat, nil)

	require.Eventually(t, func() bool {
		return OpsErrorLogEnqueuedTotal() == 1
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, int64(1), OpsErrorLogQueueLength())

	select {
	case job := <-opsErrorLogQueue:
		opsErrorLogQueueLen.Add(-1)
		require.NotNil(t, job.entry)
		require.Equal(t, "cyber_policy", job.entry.ErrorType)
		require.Equal(t, "request", job.entry.ErrorPhase)
		require.Equal(t, http.StatusBadRequest, job.entry.StatusCode)
		require.Equal(t, "glm-5.2", job.entry.Model)
		require.Equal(t, "/v1/chat/completions", job.entry.RequestPath)
		require.Contains(t, job.entry.ErrorMessage, "blocked by upstream policy")
		require.Contains(t, job.entry.ErrorBody, `"code":"cyber_policy"`)
	default:
		t.Fatal("expected gateway cyber policy ops error to be enqueued")
	}
}

func TestGatewayRecordCyberPolicyIfMarked_ForwardErrorRecordsUsageAndBlocksSession(t *testing.T) {
	c := newTestGinContext()
	body := []byte(`{"model":"glm-5.2","prompt_cache_key":"compat-session","messages":[{"role":"user","content":"cyber input"}]}`)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(string(body)))
	c.Writer.Header().Set("X-Request-Id", "req-gateway-cyber-usage")
	service.MarkOpsCyberPolicy(c, service.CyberPolicyMark{
		Message:        "blocked by upstream policy",
		Body:           `{"error":{"code":"cyber_policy","message":"blocked by upstream policy"}}`,
		UpstreamStatus: http.StatusBadRequest,
		UpstreamInTok:  17,
		UpstreamOutTok: 3,
	})

	usageRepo := &handlerCyberUsageLogRepoStub{inserted: true}
	cache := &handlerCyberCacheStoreStub{}
	settingSvc := service.NewSettingService(
		&handlerCyberSettingRepoStub{
			vals: map[string]string{
				service.SettingKeyCyberSessionBlockEnabled:    "true",
				service.SettingKeyCyberSessionBlockTTLSeconds: "60",
			},
		},
		&config.Config{},
	)
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	gatewaySvc := service.NewGatewayService(
		nil,                                 // accountRepo
		nil,                                 // groupRepo
		usageRepo,                           // usageLogRepo
		nil,                                 // usageBillingRepo
		&handlerCyberUserRepoStub{},         // userRepo
		&handlerCyberSubRepoStub{},          // userSubRepo
		nil,                                 // userGroupRateRepo
		cache,                               // cache
		cfg,                                 // cfg
		nil,                                 // schedulerSnapshot
		nil,                                 // concurrencyService
		service.NewBillingService(cfg, nil), // billingService
		nil,                                 // rateLimitService
		&service.BillingCacheService{},
		nil, // identityService
		nil, // httpUpstream
		&service.DeferredService{},
		nil,        // claudeTokenProvider
		nil,        // sessionLimitCache
		nil,        // rpmCache
		nil,        // digestStore
		settingSvc, // settingService
		nil,        // tlsFPProfileService
		nil,        // channelService
		nil,        // resolver
		nil,        // balanceNotifyService
		nil,        // userPlatformQuotaRepo
		nil,        // fingerprintNormalizer
	)
	key := service.CyberSessionBlockKey(22, c, body)
	require.NotEmpty(t, key)

	apiKey := &service.APIKey{
		ID:    22,
		Name:  "compat key",
		Key:   "sk-test-compat-key",
		User:  &service.User{ID: 33, Email: "u@example.com"},
		Group: &service.Group{ID: 44, Platform: service.PlatformAnthropic, RateMultiplier: 1},
	}
	groupID := int64(44)
	apiKey.GroupID = &groupID
	account := &service.Account{ID: 55, Platform: service.PlatformAnthropic}
	h := &GatewayHandler{gatewayService: gatewaySvc}

	h.recordGatewayCyberPolicyIfMarked(c, apiKey, account, nil, "glm-5.2", true, key, service.ChannelUsageFields{}, service.ContentModerationProtocolOpenAIChat, body)

	require.Eventually(t, func() bool {
		return usageRepo.calls == 1 && gatewaySvc.IsCyberSessionBlocked(context.Background(), key)
	}, time.Second, 10*time.Millisecond)
	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, service.RequestTypeCyberBlocked, usageRepo.lastLog.RequestType)
	require.Equal(t, 17, usageRepo.lastLog.InputTokens)
	require.Equal(t, 3, usageRepo.lastLog.OutputTokens)
	require.Equal(t, "glm-5.2", usageRepo.lastLog.Model)
}

func TestGatewayRejectIfCyberSessionBlocked_WritesCompatError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := []byte(`{"model":"glm-5.2","prompt_cache_key":"compat-session","messages":[{"role":"user","content":"blocked followup"}]}`)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(string(body)))

	cache := &handlerCyberCacheStoreStub{}
	settingSvc := service.NewSettingService(
		&handlerCyberSettingRepoStub{
			vals: map[string]string{
				service.SettingKeyCyberSessionBlockEnabled:    "true",
				service.SettingKeyCyberSessionBlockTTLSeconds: "60",
			},
		},
		&config.Config{},
	)
	cfg := &config.Config{}
	gatewaySvc := service.NewGatewayService(
		nil, nil, nil, nil, nil, nil, nil,
		cache,
		cfg,
		nil, nil, service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, nil, nil, &service.DeferredService{},
		nil, nil, nil, nil,
		settingSvc,
		nil, nil, nil, nil, nil, nil,
	)
	apiKey := &service.APIKey{ID: 22, Name: "compat key", Key: "sk-test-compat-key", Group: &service.Group{ID: 44, Platform: service.PlatformAnthropic}}
	groupID := int64(44)
	apiKey.GroupID = &groupID
	key := service.CyberSessionBlockKey(apiKey.ID, c, body)
	require.NotEmpty(t, key)
	cache.blocked = map[string]bool{key: true}
	h := &GatewayHandler{gatewayService: gatewaySvc}

	blocked := h.rejectIfCyberSessionBlocked(c, apiKey, body, "glm-5.2", cyberBlockFormatChat)

	require.True(t, blocked)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "session_blocked_by_cyber_policy")
}

// TestBuildCyberPolicyOpsErrorEntry_StatusCode verifies F6: the ops error log
// records the status the codex client actually received (400 non-stream / 200 stream),
// not a hardcoded 403.
func TestBuildCyberPolicyOpsErrorEntry_StatusCode(t *testing.T) {
	for _, tc := range []struct {
		name           string
		upstreamStatus int
	}{
		{"non_stream_400", 400},
		{"stream_200", 200},
		{"zero_value", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mark := &service.CyberPolicyMark{
				Code:           "cyber_policy",
				Message:        "blocked",
				UpstreamStatus: tc.upstreamStatus,
			}
			entry := buildCyberPolicyOpsErrorEntry(cyberPolicyOpsErrorMeta{
				RequestID: "req-1", Model: "gpt-5", RequestPath: "/openai/v1/responses",
			}, mark)
			require.Equal(t, tc.upstreamStatus, entry.StatusCode)
			require.Equal(t, "cyber_policy", entry.ErrorType)
			require.Equal(t, "request", entry.ErrorPhase)
		})
	}
}

type blockingCyberPolicyModerationRepo struct {
	contentModerationHandlerTestRepo
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (r *blockingCyberPolicyModerationRepo) CreateLog(ctx context.Context, log *service.ContentModerationLog) error {
	r.once.Do(func() {
		close(r.started)
	})
	<-r.release
	return r.contentModerationHandlerTestRepo.CreateLog(ctx, log)
}

func (r *blockingCyberPolicyModerationRepo) createStarted() bool {
	select {
	case <-r.started:
		return true
	default:
		return false
	}
}

type recordingCyberPolicyHashCache struct {
	mu             sync.Mutex
	recorded       []string
	checked        []string
	matched        map[string]struct{}
	respectContext bool
}

func (c *recordingCyberPolicyHashCache) RecordFlaggedInputHash(ctx context.Context, inputHash string, meta service.ContentModerationHashMeta) error {
	if c.respectContext {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recorded = append(c.recorded, inputHash)
	return nil
}

func (c *recordingCyberPolicyHashCache) HasFlaggedInputHash(ctx context.Context, inputHash string) (bool, error) {
	if c.respectContext {
		if err := ctx.Err(); err != nil {
			return false, err
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checked = append(c.checked, inputHash)
	_, ok := c.matched[inputHash]
	return ok, nil
}

func (c *recordingCyberPolicyHashCache) ListFlaggedInputHashes(ctx context.Context, filter service.ContentModerationHashListFilter) ([]service.ContentModerationHashItem, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{Page: 1, PageSize: 20, Pages: 1}, nil
}

func (c *recordingCyberPolicyHashCache) DeleteFlaggedInputHash(ctx context.Context, inputHash string) (bool, error) {
	return false, nil
}

func (c *recordingCyberPolicyHashCache) DeleteFlaggedInputHashes(ctx context.Context, inputHashes []string) (int64, error) {
	return 0, nil
}

func (c *recordingCyberPolicyHashCache) ClearFlaggedInputHashes(ctx context.Context) (int64, error) {
	return 0, nil
}

func (c *recordingCyberPolicyHashCache) CountFlaggedInputHashes(ctx context.Context) (int64, error) {
	return 0, nil
}

func (c *recordingCyberPolicyHashCache) AdmitModerationAPIKeyQuota(ctx context.Context, keyHash string, rpmLimit int, rpdLimit int, tpmLimit int, tokenEstimate int) (*service.ContentModerationAPIKeyQuotaState, bool, error) {
	return &service.ContentModerationAPIKeyQuotaState{}, true, nil
}

func (c *recordingCyberPolicyHashCache) GetModerationAPIKeyQuotaState(ctx context.Context, keyHash string) (*service.ContentModerationAPIKeyQuotaState, error) {
	return &service.ContentModerationAPIKeyQuotaState{}, nil
}

func (c *recordingCyberPolicyHashCache) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.recorded...)
}

func (c *recordingCyberPolicyHashCache) snapshotChecked() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.checked...)
}

type handlerCyberUsageLogRepoStub struct {
	service.UsageLogRepository

	inserted bool
	calls    int
	lastLog  *service.UsageLog
}

func (s *handlerCyberUsageLogRepoStub) Create(ctx context.Context, log *service.UsageLog) (bool, error) {
	s.calls++
	s.lastLog = log
	return s.inserted, nil
}

type handlerCyberUserRepoStub struct {
	service.UserRepository
}

func (s *handlerCyberUserRepoStub) DeductBalance(ctx context.Context, id int64, amount float64) error {
	return nil
}

type handlerCyberSubRepoStub struct {
	service.UserSubscriptionRepository
}

func (s *handlerCyberSubRepoStub) IncrementUsage(ctx context.Context, id int64, costUSD float64) error {
	return nil
}

type handlerCyberSettingRepoStub struct {
	vals map[string]string
}

func (r *handlerCyberSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	v, ok := r.vals[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return v, nil
}

func (r *handlerCyberSettingRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	panic("handlerCyberSettingRepoStub.Get not implemented")
}

func (r *handlerCyberSettingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("handlerCyberSettingRepoStub.Set not implemented")
}

func (r *handlerCyberSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("handlerCyberSettingRepoStub.GetMultiple not implemented")
}

func (r *handlerCyberSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("handlerCyberSettingRepoStub.SetMultiple not implemented")
}

func (r *handlerCyberSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("handlerCyberSettingRepoStub.GetAll not implemented")
}

func (r *handlerCyberSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("handlerCyberSettingRepoStub.Delete not implemented")
}

type handlerCyberCacheStoreStub struct {
	mu      sync.Mutex
	blocked map[string]bool
}

func (c *handlerCyberCacheStoreStub) GetSessionAccountID(ctx context.Context, groupID int64, sessionHash string) (int64, error) {
	return 0, nil
}

func (c *handlerCyberCacheStoreStub) SetSessionAccountID(ctx context.Context, groupID int64, sessionHash string, accountID int64, ttl time.Duration) error {
	return nil
}

func (c *handlerCyberCacheStoreStub) RefreshSessionTTL(ctx context.Context, groupID int64, sessionHash string, ttl time.Duration) error {
	return nil
}

func (c *handlerCyberCacheStoreStub) DeleteSessionAccountID(ctx context.Context, groupID int64, sessionHash string) error {
	return nil
}

func (c *handlerCyberCacheStoreStub) GetOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string) ([]byte, error) {
	return nil, nil
}

func (c *handlerCyberCacheStoreStub) SetOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string, payload []byte, ttl time.Duration) error {
	return nil
}

func (c *handlerCyberCacheStoreStub) DeleteOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string) error {
	return nil
}

func (c *handlerCyberCacheStoreStub) SetCyberSessionBlocked(ctx context.Context, key string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.blocked == nil {
		c.blocked = map[string]bool{}
	}
	c.blocked[key] = true
	return nil
}

func (c *handlerCyberCacheStoreStub) IsCyberSessionBlocked(ctx context.Context, key string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.blocked[key], nil
}
