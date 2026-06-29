//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 目标：严格验证“antigravity 账号通过 /v1/messages 提供 Claude 服务时”，
// 当账号 credentials.intercept_warmup_requests=true 且请求为 Warmup 时，
// 后端会在转发上游前直接拦截并返回 mock 响应（不依赖上游）。

type fakeSchedulerCache struct {
	accounts []*service.Account
}

func (f *fakeSchedulerCache) GetSnapshot(_ context.Context, _ service.SchedulerBucket) ([]*service.Account, bool, error) {
	return f.accounts, true, nil
}
func (f *fakeSchedulerCache) SetSnapshot(_ context.Context, _ service.SchedulerBucket, _ []service.Account) error {
	return nil
}
func (f *fakeSchedulerCache) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	for _, account := range f.accounts {
		if account != nil && account.ID == id {
			return account, nil
		}
	}
	return nil, nil
}
func (f *fakeSchedulerCache) SetAccount(_ context.Context, _ *service.Account) error { return nil }
func (f *fakeSchedulerCache) DeleteAccount(_ context.Context, _ int64) error         { return nil }
func (f *fakeSchedulerCache) UpdateLastUsed(_ context.Context, _ map[int64]time.Time) error {
	return nil
}
func (f *fakeSchedulerCache) TryLockBucket(_ context.Context, _ service.SchedulerBucket, _ time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeSchedulerCache) UnlockBucket(_ context.Context, _ service.SchedulerBucket) error {
	return nil
}
func (f *fakeSchedulerCache) ListBuckets(_ context.Context) ([]service.SchedulerBucket, error) {
	return nil, nil
}
func (f *fakeSchedulerCache) GetOutboxWatermark(_ context.Context) (int64, error) { return 0, nil }
func (f *fakeSchedulerCache) SetOutboxWatermark(_ context.Context, _ int64) error { return nil }

type fakeGroupRepo struct {
	group *service.Group
}

func (f *fakeGroupRepo) Create(context.Context, *service.Group) error { return nil }
func (f *fakeGroupRepo) GetByID(context.Context, int64) (*service.Group, error) {
	return f.group, nil
}
func (f *fakeGroupRepo) GetByIDLite(context.Context, int64) (*service.Group, error) {
	return f.group, nil
}
func (f *fakeGroupRepo) Update(context.Context, *service.Group) error          { return nil }
func (f *fakeGroupRepo) Delete(context.Context, int64) error                   { return nil }
func (f *fakeGroupRepo) DeleteCascade(context.Context, int64) ([]int64, error) { return nil, nil }
func (f *fakeGroupRepo) List(context.Context, pagination.PaginationParams) ([]service.Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (f *fakeGroupRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]service.Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (f *fakeGroupRepo) ListActive(context.Context) ([]service.Group, error) { return nil, nil }
func (f *fakeGroupRepo) ListActiveByPlatform(context.Context, string) ([]service.Group, error) {
	return nil, nil
}
func (f *fakeGroupRepo) ExistsByName(context.Context, string) (bool, error) { return false, nil }
func (f *fakeGroupRepo) GetAccountCount(context.Context, int64) (int64, int64, error) {
	return 0, 0, nil
}
func (f *fakeGroupRepo) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (f *fakeGroupRepo) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (f *fakeGroupRepo) BindAccountsToGroup(context.Context, int64, []int64) error { return nil }
func (f *fakeGroupRepo) UpdateSortOrders(context.Context, []service.GroupSortOrderUpdate) error {
	return nil
}

type fakeConcurrencyCache struct{}

func (f *fakeConcurrencyCache) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}
func (f *fakeConcurrencyCache) ReleaseAccountSlot(context.Context, int64, string) error { return nil }
func (f *fakeConcurrencyCache) GetAccountConcurrency(context.Context, int64) (int, error) {
	return 0, nil
}
func (f *fakeConcurrencyCache) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	return true, nil
}
func (f *fakeConcurrencyCache) DecrementAccountWaitCount(context.Context, int64) error { return nil }
func (f *fakeConcurrencyCache) GetAccountWaitingCount(context.Context, int64) (int, error) {
	return 0, nil
}
func (f *fakeConcurrencyCache) AcquireUserSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}
func (f *fakeConcurrencyCache) ReleaseUserSlot(context.Context, int64, string) error   { return nil }
func (f *fakeConcurrencyCache) GetUserConcurrency(context.Context, int64) (int, error) { return 0, nil }
func (f *fakeConcurrencyCache) IncrementWaitCount(context.Context, int64, int) (bool, error) {
	return true, nil
}
func (f *fakeConcurrencyCache) DecrementWaitCount(context.Context, int64) error { return nil }
func (f *fakeConcurrencyCache) GetAccountsLoadBatch(context.Context, []service.AccountWithConcurrency) (map[int64]*service.AccountLoadInfo, error) {
	return map[int64]*service.AccountLoadInfo{}, nil
}
func (f *fakeConcurrencyCache) GetUsersLoadBatch(context.Context, []service.UserWithConcurrency) (map[int64]*service.UserLoadInfo, error) {
	return map[int64]*service.UserLoadInfo{}, nil
}
func (f *fakeConcurrencyCache) GetAccountConcurrencyBatch(_ context.Context, accountIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int, len(accountIDs))
	for _, id := range accountIDs {
		result[id] = 0
	}
	return result, nil
}
func (f *fakeConcurrencyCache) CleanupExpiredAccountSlots(context.Context, int64) error { return nil }
func (f *fakeConcurrencyCache) CleanupStaleProcessSlots(context.Context, string) error  { return nil }

func newTestGatewayHandler(t *testing.T, group *service.Group, accounts []*service.Account) (*GatewayHandler, func()) {
	return newTestGatewayHandlerWithUpstream(t, group, accounts, nil)
}

func newTestGatewayHandlerWithUpstream(t *testing.T, group *service.Group, accounts []*service.Account, upstream service.HTTPUpstream) (*GatewayHandler, func()) {
	t.Helper()

	schedulerCache := &fakeSchedulerCache{accounts: accounts}
	schedulerSnapshot := service.NewSchedulerSnapshotService(schedulerCache, nil, nil, nil, nil)
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.Security.URLAllowlist.Enabled = true
	cfg.Security.URLAllowlist.UpstreamHosts = []string{"api.anthropic.com"}
	cfg.Security.URLAllowlist.AllowPrivateHosts = true

	gwSvc := service.NewGatewayService(
		nil, // accountRepo (not used: scheduler snapshot hit)
		&fakeGroupRepo{group: group},
		nil, // usageLogRepo
		nil, // usageBillingRepo
		nil, // userRepo
		nil, // userSubRepo
		nil, // userGroupRateRepo
		nil, // cache (disable sticky)
		cfg,
		schedulerSnapshot,
		nil, // concurrencyService (disable load-aware; tryAcquire always acquired)
		nil, // billingService
		nil, // rateLimitService
		nil, // billingCacheService
		nil, // identityService
		upstream,
		nil, // deferredService
		nil, // claudeTokenProvider
		nil, // sessionLimitCache
		nil, // rpmCache
		nil, // digestStore
		nil, // settingService
		nil, // tlsFPProfileService
		nil, // channelService
		nil, // resolver
		nil, // balanceNotifyService
		nil, // userPlatformQuotaRepo
		nil, // fingerprintNormalizer
	)

	// RunModeSimple：跳过计费检查，避免引入 repo/cache 依赖。
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)

	concurrencySvc := service.NewConcurrencyService(&fakeConcurrencyCache{})
	concurrencyHelper := NewConcurrencyHelper(concurrencySvc, SSEPingFormatClaude, 0)

	h := &GatewayHandler{
		gatewayService:      gwSvc,
		billingCacheService: billingCacheSvc,
		concurrencyHelper:   concurrencyHelper,
		// 这些字段对本测试不敏感，保持较小即可
		maxAccountSwitches:       1,
		maxAccountSwitchesGemini: 1,
	}

	cleanup := func() {
		billingCacheSvc.Stop()
	}
	return h, cleanup
}

type fakeGatewayHTTPUpstream struct {
	calls int
}

func (f *fakeGatewayHTTPUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	f.calls++
	return &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"unexpected upstream call"}}`))),
	}, nil
}

func (f *fakeGatewayHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return f.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestGatewayHandlerMessages_InterceptWarmup_AntigravityAccount_MixedSchedulingV1(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(2001)
	accountID := int64(1001)

	group := &service.Group{
		ID:       groupID,
		Hydrated: true,
		Platform: service.PlatformAnthropic, // /v1/messages（Claude兼容）入口
		Status:   service.StatusActive,
	}

	account := &service.Account{
		ID:       accountID,
		Name:     "ag-1",
		Platform: service.PlatformAntigravity,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":              "tok_xxx",
			"intercept_warmup_requests": true,
		},
		Extra: map[string]any{
			"mixed_scheduling": true, // 关键：允许被 anthropic 分组混合调度选中
		},
		Concurrency:   1,
		Priority:      1,
		Status:        service.StatusActive,
		Schedulable:   true,
		AccountGroups: []service.AccountGroup{{AccountID: accountID, GroupID: groupID}},
	}

	h, cleanup := newTestGatewayHandler(t, group, []*service.Account{account})
	defer cleanup()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	body := []byte(`{
		"model": "claude-sonnet-4-5",
		"max_tokens": 256,
		"messages": [{"role":"user","content":[{"type":"text","text":"Warmup"}]}]
	}`)
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.Group, group))
	c.Request = req

	apiKey := &service.APIKey{
		ID:      3001,
		UserID:  4001,
		GroupID: &groupID,
		Status:  service.StatusActive,
		User: &service.User{
			ID:          4001,
			Concurrency: 10,
			Balance:     100,
		},
		Group: group,
	}

	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})

	h.Messages(c)

	require.Equal(t, 200, rec.Code)

	// 断言：确实选中了 antigravity 账号（不是纯函数测试，而是从 Handler 里验证调度结果）
	selected, ok := c.Get(opsAccountIDKey)
	require.True(t, ok)
	require.Equal(t, accountID, selected)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "msg_mock_warmup", resp["id"])
	require.Equal(t, "claude-sonnet-4-5", resp["model"])

	content, ok := resp["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	first, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "New Conversation", first["text"])
}

func TestGatewayHandlerMessages_InterceptWarmup_AntigravityAccount_ForcePlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(2002)
	accountID := int64(1002)

	group := &service.Group{
		ID:       groupID,
		Hydrated: true,
		Platform: service.PlatformAntigravity,
		Status:   service.StatusActive,
	}

	account := &service.Account{
		ID:       accountID,
		Name:     "ag-2",
		Platform: service.PlatformAntigravity,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":              "tok_xxx",
			"intercept_warmup_requests": true,
		},
		Concurrency:   1,
		Priority:      1,
		Status:        service.StatusActive,
		Schedulable:   true,
		AccountGroups: []service.AccountGroup{{AccountID: accountID, GroupID: groupID}},
	}

	h, cleanup := newTestGatewayHandler(t, group, []*service.Account{account})
	defer cleanup()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	body := []byte(`{
		"model": "claude-sonnet-4-5",
		"max_tokens": 256,
		"messages": [{"role":"user","content":[{"type":"text","text":"Warmup"}]}]
	}`)
	req := httptest.NewRequest("POST", "/antigravity/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// 模拟 routes/gateway.go 里的 ForcePlatform 中间件效果：
	// - 写入 request.Context（Service读取）
	// - 写入 gin.Context（Handler快速读取）
	ctx := context.WithValue(req.Context(), ctxkey.Group, group)
	ctx = context.WithValue(ctx, ctxkey.ForcePlatform, service.PlatformAntigravity)
	req = req.WithContext(ctx)
	c.Request = req
	c.Set(string(middleware.ContextKeyForcePlatform), service.PlatformAntigravity)

	apiKey := &service.APIKey{
		ID:      3002,
		UserID:  4002,
		GroupID: &groupID,
		Status:  service.StatusActive,
		User: &service.User{
			ID:          4002,
			Concurrency: 10,
			Balance:     100,
		},
		Group: group,
	}

	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})

	h.Messages(c)

	require.Equal(t, 200, rec.Code)

	selected, ok := c.Get(opsAccountIDKey)
	require.True(t, ok)
	require.Equal(t, accountID, selected)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "msg_mock_warmup", resp["id"])
	require.Equal(t, "claude-sonnet-4-5", resp["model"])
}

func TestGatewayHandlerMessages_ChannelMonitorProbe_ReturnsLocalChallengeAnswerWithoutForward(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(2003)
	accountID := int64(1003)

	group := &service.Group{
		ID:       groupID,
		Hydrated: true,
		Platform: service.PlatformAnthropic,
		Status:   service.StatusActive,
	}

	account := &service.Account{
		ID:          accountID,
		Name:        "anthropic-probe",
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "upstream-key", "base_url": "https://api.anthropic.com"},
		Concurrency: 1,
		Priority:    1,
		Status:      service.StatusActive,
		Schedulable: true,
		AccountGroups: []service.AccountGroup{
			{AccountID: accountID, GroupID: groupID},
		},
	}

	upstream := &fakeGatewayHTTPUpstream{}
	h, cleanup := newTestGatewayHandlerWithUpstream(t, group, []*service.Account{account}, upstream)
	defer cleanup()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	body := []byte(`{
		"model": "claude-haiku-4-5",
		"max_tokens": 50,
		"messages": [{"role":"user","content":"Calculate and respond with ONLY the number, nothing else.\n\nQ: 3 + 5 = ?\nA: 8\n\nQ: 12 - 7 = ?\nA: 5\n\nQ: 16 + 5 = ?\nA:"}]
	}`)
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(service.ChannelMonitorProbeHeaderName, service.ChannelMonitorProbeHeaderValue)
	ctx := context.WithValue(req.Context(), ctxkey.Group, group)
	ctx = service.WithChannelMonitorProbeContext(ctx)
	req = req.WithContext(ctx)
	c.Request = req

	apiKey := &service.APIKey{
		ID:      3003,
		UserID:  4003,
		GroupID: &groupID,
		Status:  service.StatusActive,
		User: &service.User{
			ID:          4003,
			Concurrency: 10,
			Balance:     100,
		},
		Group: group,
	}

	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})

	h.Messages(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Zero(t, upstream.calls, "channel monitor probes should not reach external upstream")

	selected, ok := c.Get(opsAccountIDKey)
	require.True(t, ok)
	require.Equal(t, accountID, selected)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "claude-haiku-4-5", resp["model"])
	content, ok := resp["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 1)
	first, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "21", first["text"])
}

func TestGatewayHandlerMessages_ExternalChannelMonitorProbeHeaderDoesNotBypassUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(2004)
	accountID := int64(1004)

	group := &service.Group{
		ID:       groupID,
		Hydrated: true,
		Platform: service.PlatformAnthropic,
		Status:   service.StatusActive,
	}
	account := &service.Account{
		ID:          accountID,
		Name:        "anthropic-external-probe",
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "upstream-key", "base_url": "https://api.anthropic.com"},
		Concurrency: 1,
		Priority:    1,
		Status:      service.StatusActive,
		Schedulable: true,
		AccountGroups: []service.AccountGroup{
			{AccountID: accountID, GroupID: groupID},
		},
	}
	upstream := &fakeGatewayHTTPUpstream{}
	h, cleanup := newTestGatewayHandlerWithUpstream(t, group, []*service.Account{account}, upstream)
	defer cleanup()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{
		"model": "claude-haiku-4-5",
		"max_tokens": 50,
		"messages": [{"role":"user","content":"Calculate and respond with ONLY the number, nothing else.\n\nQ: 3 + 5 = ?\nA: 8\n\nQ: 12 - 7 = ?\nA: 5\n\nQ: 16 + 5 = ?\nA:"}]
	}`)
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(service.ChannelMonitorProbeHeaderName, service.ChannelMonitorProbeHeaderValue)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.Group, group))
	c.Request = req

	apiKey := &service.APIKey{
		ID:      3004,
		UserID:  4004,
		GroupID: &groupID,
		Status:  service.StatusActive,
		User: &service.User{
			ID:          4004,
			Concurrency: 10,
			Balance:     100,
		},
		Group: group,
	}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})

	h.Messages(c)

	require.NotEqual(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, upstream.calls, "external headers alone must not trigger local channel monitor answer")
	require.NotContains(t, rec.Body.String(), "msg_channel_monitor_probe")
}
