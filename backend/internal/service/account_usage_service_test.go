package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

type accountUsageCodexProbeRepo struct {
	stubOpenAIAccountRepo
	updateExtraCh chan map[string]any
	rateLimitCh   chan time.Time
}

func (r *accountUsageCodexProbeRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.updateExtraCh != nil {
		copied := make(map[string]any, len(updates))
		for k, v := range updates {
			copied[k] = v
		}
		r.updateExtraCh <- copied
	}
	return nil
}

func (r *accountUsageCodexProbeRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	if r.rateLimitCh != nil {
		r.rateLimitCh <- resetAt
	}
	return nil
}

type grokSevenDayUsageRepoStub struct {
	geminiUsageLogRepoStub
	requestedAccountID int64
	requestedStart     time.Time
	requestedStarts    []time.Time
	windowStats        *usagestats.AccountStats
	windowStatsByCall  []*usagestats.AccountStats
}

func (r *grokSevenDayUsageRepoStub) GetAccountWindowStats(_ context.Context, accountID int64, startTime time.Time) (*usagestats.AccountStats, error) {
	r.requestedAccountID = accountID
	r.requestedStart = startTime
	r.requestedStarts = append(r.requestedStarts, startTime)
	if index := len(r.requestedStarts) - 1; index < len(r.windowStatsByCall) {
		return r.windowStatsByCall[index], nil
	}
	return r.windowStats, nil
}

func TestAccountUsageService_GetGrokUsageShowsSevenDayRatioAndBillingStats(t *testing.T) {
	t.Parallel()

	requestLimit := int64(100)
	requestRemaining := int64(60)
	tokenLimit := int64(1000)
	tokenRemaining := int64(100)
	resetUnix := time.Now().Add(7 * 24 * time.Hour).Unix()
	account := &Account{
		ID:       7042,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			grokQuotaSnapshotExtraKey: &xai.QuotaSnapshot{
				Requests:  &xai.QuotaWindow{Limit: &requestLimit, Remaining: &requestRemaining},
				Tokens:    &xai.QuotaWindow{Limit: &tokenLimit, Remaining: &tokenRemaining, ResetUnix: &resetUnix},
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}
	repo := &grokSevenDayUsageRepoStub{
		windowStats: &usagestats.AccountStats{
			Requests:     7,
			Tokens:       700,
			Cost:         1.23,
			StandardCost: 1.11,
			UserCost:     2.34,
		},
	}
	svc := &AccountUsageService{usageLogRepo: repo, grokQuotaFetcher: NewGrokQuotaFetcher(), cache: NewUsageCache()}

	usage, err := svc.getGrokUsage(context.Background(), account, false)

	if err != nil {
		t.Fatalf("getGrokUsage() error = %v", err)
	}
	if usage.SevenDay == nil {
		t.Fatal("expected seven_day usage progress")
	}
	if usage.SevenDay.Utilization != 90 {
		t.Fatalf("seven_day utilization = %v, want 90", usage.SevenDay.Utilization)
	}
	if usage.SevenDay.ResetsAt == nil || usage.SevenDay.ResetsAt.Unix() != resetUnix {
		t.Fatalf("seven_day resets_at = %v, want unix %d", usage.SevenDay.ResetsAt, resetUnix)
	}
	if usage.SevenDay.WindowStats == nil {
		t.Fatal("expected seven_day window_stats")
	}
	if usage.SevenDay.WindowStats.Requests != 7 || usage.SevenDay.WindowStats.Tokens != 700 {
		t.Fatalf("seven_day window stats = %+v, want requests=7 tokens=700", usage.SevenDay.WindowStats)
	}
	if usage.SevenDay.WindowStats.Cost != 1.23 || usage.SevenDay.WindowStats.UserCost != 2.34 {
		t.Fatalf("seven_day billing stats = %+v, want account=1.23 user=2.34", usage.SevenDay.WindowStats)
	}
	if usage.GrokLocalUsage == nil || usage.GrokLocalUsage.Requests != 7 {
		t.Fatalf("grok_local_usage = %+v, want requests=7 aligned to weekly window", usage.GrokLocalUsage)
	}
	if repo.requestedAccountID != account.ID {
		t.Fatalf("GetAccountWindowStats account id = %d, want %d", repo.requestedAccountID, account.ID)
	}
	if time.Since(repo.requestedStart) < 6*24*time.Hour {
		t.Fatalf("GetAccountWindowStats start = %v, expected roughly 7d window", repo.requestedStart)
	}
}

func TestAccountUsageService_GetGrokUsageWithoutQuotaSnapshotDoesNotPanic(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       7043,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{},
	}
	repo := &grokSevenDayUsageRepoStub{
		windowStats: &usagestats.AccountStats{Requests: 3, Tokens: 300},
	}
	svc := &AccountUsageService{usageLogRepo: repo, grokQuotaFetcher: NewGrokQuotaFetcher(), cache: NewUsageCache()}

	usage, err := svc.getGrokUsage(context.Background(), account, false)

	if err != nil {
		t.Fatalf("getGrokUsage() error = %v", err)
	}
	if usage == nil {
		t.Fatal("expected usage")
	}
	if usage.SevenDay == nil || usage.SevenDay.WindowStats == nil {
		t.Fatalf("expected seven_day billing window stats without quota snapshot, got %+v", usage.SevenDay)
	}
	if usage.SevenDay.WindowStats.Requests != 3 || usage.SevenDay.WindowStats.Tokens != 300 {
		t.Fatalf("seven_day window stats = %+v, want requests=3 tokens=300", usage.SevenDay.WindowStats)
	}
}

func TestAccountUsageService_GetGrokFreeUsageUsesRolling24HourWindow(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       7045,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			grokBillingSnapshotExtraKey: &xai.BillingSnapshot{
				UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
				SubscriptionTier: "free",
			},
		},
	}
	repo := &grokSevenDayUsageRepoStub{
		windowStats: &usagestats.AccountStats{Requests: 4, Tokens: 800000},
	}
	svc := &AccountUsageService{usageLogRepo: repo, grokQuotaFetcher: NewGrokQuotaFetcher(), cache: NewUsageCache()}

	before := time.Now().UTC().Add(-24 * time.Hour)
	usage, err := svc.getGrokUsage(context.Background(), account, false)
	after := time.Now().UTC().Add(-24 * time.Hour)

	if err != nil {
		t.Fatalf("getGrokUsage() error = %v", err)
	}
	if usage.GrokLocalUsage24h == nil || usage.GrokLocalUsage24h.Tokens != 800000 {
		t.Fatalf("grok_local_usage_24h = %+v, want tokens=800000", usage.GrokLocalUsage24h)
	}
	if usage.GrokFreeQuotaUsage == nil || usage.GrokFreeQuotaUsage.Tokens != 800000 {
		t.Fatalf("grok_free_quota_usage = %+v, want tokens=800000", usage.GrokFreeQuotaUsage)
	}
	if usage.GrokFreeQuotaPolicy == nil {
		t.Fatal("expected Grok free quota policy")
	}
	if usage.GrokFreeQuotaPolicy.TokenLimit != 2_000_000 ||
		usage.GrokFreeQuotaPolicy.SoftGatePercent != 95 ||
		usage.GrokFreeQuotaPolicy.SoftGateTokens != 1_900_000 ||
		usage.GrokFreeQuotaPolicy.WindowHours != 24 {
		t.Fatalf("grok_free_quota_policy = %+v, want default policy", usage.GrokFreeQuotaPolicy)
	}
	if usage.GrokLocalUsage != nil || usage.SevenDay != nil || usage.ThirtyDay != nil {
		t.Fatalf("free usage must not synthesize paid windows: %+v", usage)
	}
	if repo.requestedStart.Before(before.Add(-time.Second)) || repo.requestedStart.After(after.Add(time.Second)) {
		t.Fatalf("rolling window start = %v, want between %v and %v", repo.requestedStart, before, after)
	}
}

func TestAccountUsageService_GrokFreeQuotaPolicyUsesRuntimeConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Gateway.Grok.FreeQuotaSoftGateEnabled = true
	cfg.Gateway.Grok.FreeQuotaTokenLimit = 4_000_000
	cfg.Gateway.Grok.FreeQuotaSoftGatePercent = 80
	cfg.Gateway.Grok.FreeQuotaWindowHours = 12
	account := &Account{
		ID:       7046,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			grokBillingSnapshotExtraKey: &xai.BillingSnapshot{
				UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
				SubscriptionTier: "free",
			},
		},
	}
	repo := &grokSevenDayUsageRepoStub{
		windowStats: &usagestats.AccountStats{Requests: 5, Tokens: 3_200_000},
	}
	svc := &AccountUsageService{
		usageLogRepo:   repo,
		settingService: &SettingService{cfg: cfg},
	}

	before := time.Now().UTC().Add(-12 * time.Hour)
	usage := svc.buildGrokUsageInfo(context.Background(), account, false)
	after := time.Now().UTC().Add(-12 * time.Hour)
	policy := usage.GrokFreeQuotaPolicy
	if policy == nil || !policy.Enabled || policy.TokenLimit != 4_000_000 ||
		policy.SoftGatePercent != 80 || policy.SoftGateTokens != 3_200_000 || policy.WindowHours != 12 {
		t.Fatalf("grok free quota policy = %+v", policy)
	}
	if usage.GrokFreeQuotaUsage == nil || usage.GrokFreeQuotaUsage.Tokens != 3_200_000 {
		t.Fatalf("grok free quota usage = %+v, want tokens=3200000", usage.GrokFreeQuotaUsage)
	}
	if repo.requestedStart.Before(before.Add(-time.Second)) || repo.requestedStart.After(after.Add(time.Second)) {
		t.Fatalf("configured rolling window start = %v, want between %v and %v", repo.requestedStart, before, after)
	}
}

func TestAccountUsageService_GetGrokUsageAppliesOfficialBillingSnapshot(t *testing.T) {
	t.Parallel()

	weekStart := time.Now().UTC().Add(-3 * 24 * time.Hour).Truncate(time.Second)
	weekEnd := weekStart.Add(7 * 24 * time.Hour)
	monthEnd := time.Date(weekStart.Year(), weekStart.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	account := &Account{
		ID:       7044,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			grokBillingSnapshotExtraKey: &xai.BillingSnapshot{
				UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
				SubscriptionTier: "GrokPro",
				Credits: &xai.CreditsBillingConfig{
					CurrentPeriod: &xai.UsagePeriod{
						Type:  "USAGE_PERIOD_TYPE_WEEKLY",
						Start: weekStart.Format(time.RFC3339Nano),
						End:   weekEnd.Format(time.RFC3339Nano),
					},
					CreditUsagePercent:   55,
					PrepaidBalance:       &xai.MoneyVal{Val: 12},
					OnDemandCap:          &xai.MoneyVal{Val: 100},
					OnDemandUsed:         &xai.MoneyVal{Val: 5},
					IsUnifiedBillingUser: true,
				},
				Monthly: &xai.MonthlyBillingConfig{
					MonthlyLimit:       &xai.MoneyVal{Val: 15000},
					Used:               &xai.MoneyVal{Val: 2192},
					BillingPeriodEnd:   monthEnd.Format(time.RFC3339),
					BillingPeriodStart: time.Date(weekStart.Year(), weekStart.Month(), 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
				},
			},
		},
	}
	repo := &grokSevenDayUsageRepoStub{
		windowStatsByCall: []*usagestats.AccountStats{
			{Requests: 9, Tokens: 900, Cost: 1.1, StandardCost: 1.0, UserCost: 1.5},
			{Requests: 30, Tokens: 3000, Cost: 3.1, StandardCost: 3.0, UserCost: 3.5},
		},
	}
	svc := &AccountUsageService{usageLogRepo: repo, grokQuotaFetcher: NewGrokQuotaFetcher(), cache: NewUsageCache()}

	usage, err := svc.getGrokUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getGrokUsage() error = %v", err)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 55 {
		t.Fatalf("seven_day = %+v, want utilization 55", usage.SevenDay)
	}
	if usage.SevenDay.ResetsAt == nil || !usage.SevenDay.ResetsAt.Equal(weekEnd) {
		t.Fatalf("seven_day resets_at = %v, want %v", usage.SevenDay.ResetsAt, weekEnd)
	}
	if usage.ThirtyDay == nil || usage.ThirtyDay.Utilization < 14 || usage.ThirtyDay.Utilization > 15 {
		t.Fatalf("thirty_day = %+v, want ~14.6%%", usage.ThirtyDay)
	}
	if usage.ThirtyDay.WindowStats == nil || usage.ThirtyDay.WindowStats.Requests != 30 || usage.ThirtyDay.WindowStats.Tokens != 3000 {
		t.Fatalf("thirty_day window stats = %+v, want requests=30 tokens=3000", usage.ThirtyDay.WindowStats)
	}
	if usage.ThirtyDay.WindowStats.Cost != 3.1 || usage.ThirtyDay.WindowStats.UserCost != 3.5 {
		t.Fatalf("thirty_day billing stats = %+v, want account=3.1 user=3.5", usage.ThirtyDay.WindowStats)
	}
	if usage.GrokBilling == nil || usage.GrokBilling.MonthlyUsed != 2192 || usage.GrokBilling.MonthlyLimit != 15000 {
		t.Fatalf("grok_billing = %+v", usage.GrokBilling)
	}
	if usage.GrokBilling.PrepaidBalance != 12 || usage.GrokBilling.OnDemandCap != 100 {
		t.Fatalf("grok_billing balance fields = %+v", usage.GrokBilling)
	}
	if usage.SubscriptionTier != "GrokPro" {
		t.Fatalf("subscription_tier = %q, want GrokPro", usage.SubscriptionTier)
	}
	// Local stats window must align to official weekly period start.
	if len(repo.requestedStarts) != 2 {
		t.Fatalf("window stats query count = %d, want weekly and monthly", len(repo.requestedStarts))
	}
	if !repo.requestedStarts[0].Equal(weekStart) && repo.requestedStarts[0].Sub(weekStart).Abs() > time.Second {
		t.Fatalf("weekly window start = %v, want %v", repo.requestedStarts[0], weekStart)
	}
	monthStart := time.Date(weekStart.Year(), weekStart.Month(), 1, 0, 0, 0, 0, time.UTC)
	if !repo.requestedStarts[1].Equal(monthStart) {
		t.Fatalf("monthly window start = %v, want %v", repo.requestedStarts[1], monthStart)
	}
	if usage.GrokLocalUsage == nil || usage.GrokLocalUsage.Requests != 9 {
		t.Fatalf("grok_local_usage = %+v, want requests=9", usage.GrokLocalUsage)
	}
}

func TestGrokSnapshotUtilizationNilSnapshot(t *testing.T) {
	t.Parallel()

	utilization, resetAt, ok := grokSnapshotUtilization(nil)

	if ok {
		t.Fatalf("grokSnapshotUtilization(nil) ok = true, want false with utilization %v reset %v", utilization, resetAt)
	}
	if utilization != 0 || resetAt != nil {
		t.Fatalf("grokSnapshotUtilization(nil) = (%v, %v), want (0, nil)", utilization, resetAt)
	}
}

func TestShouldRefreshOpenAICodexSnapshot(t *testing.T) {
	t.Parallel()

	rateLimitedUntil := time.Now().Add(5 * time.Minute)
	now := time.Now()
	usage := &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 0},
		SevenDay: &UsageProgress{Utilization: 0},
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{RateLimitResetAt: &rateLimitedUntil}, usage, now) {
		t.Fatal("expected rate-limited account to force codex snapshot refresh")
	}

	if shouldRefreshOpenAICodexSnapshot(&Account{}, usage, now) {
		t.Fatal("expected complete non-rate-limited usage to skip codex snapshot refresh")
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{}, &UsageInfo{FiveHour: nil, SevenDay: &UsageProgress{}}, now) {
		t.Fatal("expected missing 5h snapshot to require refresh")
	}

	staleAt := now.Add(-(openAIProbeCacheTTL + time.Minute)).Format(time.RFC3339)
	if !shouldRefreshOpenAICodexSnapshot(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"codex_usage_updated_at":                       staleAt,
		},
	}, usage, now) {
		t.Fatal("expected stale ws snapshot to trigger refresh")
	}
}

// TestShouldRefreshOpenAICodexSnapshot_SparkShadowIgnoresWSv2 外审第9轮 P1:spark 影子用量走
// QueryUsage(/wham/usage,与 WSv2 无关),staleness 不得被 WSv2 门控,否则首刷后窗口永久冻结。
func TestShouldRefreshOpenAICodexSnapshot_SparkShadowIgnoresWSv2(t *testing.T) {
	t.Parallel()

	now := time.Now()
	usage := &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 0},
		SevenDay: &UsageProgress{Utilization: 0},
	}
	staleAt := now.Add(-(openAIProbeCacheTTL + time.Minute)).Format(time.RFC3339)
	freshAt := now.Add(-time.Minute).Format(time.RFC3339)
	parentID := int64(7001)

	// 影子无 WSv2,但首刷后窗口已存在;过期 codex_usage_updated_at 必须触发再刷新。
	shadowStale := &Account{
		Platform:        PlatformOpenAI,
		Type:            AccountTypeOAuth,
		ParentAccountID: &parentID,
		QuotaDimension:  QuotaDimensionSpark,
		Extra:           map[string]any{"codex_usage_updated_at": staleAt},
	}
	if !shouldRefreshOpenAICodexSnapshot(shadowStale, usage, now) {
		t.Fatal("expected stale spark shadow (no WSv2) to trigger refresh")
	}

	// 影子时间戳仍新鲜→不刷(TTL 生效)。
	shadowFresh := &Account{
		Platform:        PlatformOpenAI,
		Type:            AccountTypeOAuth,
		ParentAccountID: &parentID,
		QuotaDimension:  QuotaDimensionSpark,
		Extra:           map[string]any{"codex_usage_updated_at": freshAt},
	}
	if shouldRefreshOpenAICodexSnapshot(shadowFresh, usage, now) {
		t.Fatal("expected fresh spark shadow to skip refresh (TTL not elapsed)")
	}

	// 反向对照:普通账号无 WSv2 + 过期时间戳→仍不刷(WSv2 门控普通账号的 probe 刷新)。
	normalNoWS := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{"codex_usage_updated_at": staleAt},
	}
	if shouldRefreshOpenAICodexSnapshot(normalNoWS, usage, now) {
		t.Fatal("expected non-WSv2 normal account to skip codex probe refresh")
	}
}

func TestExtractOpenAICodexProbeUpdatesAccepts429WithCodexHeaders(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "100")
	headers.Set("x-codex-secondary-reset-after-seconds", "18000")
	headers.Set("x-codex-secondary-window-minutes", "300")

	updates, err := extractOpenAICodexProbeUpdates(&http.Response{StatusCode: http.StatusTooManyRequests, Header: headers})
	if err != nil {
		t.Fatalf("extractOpenAICodexProbeUpdates() error = %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("expected codex probe updates from 429 headers")
	}
	if got := updates["codex_5h_used_percent"]; got != 100.0 {
		t.Fatalf("codex_5h_used_percent = %v, want 100", got)
	}
	if got := updates["codex_7d_used_percent"]; got != 100.0 {
		t.Fatalf("codex_7d_used_percent = %v, want 100", got)
	}
}

func TestAccountUsageService_PersistOpenAICodexProbeSnapshotOnlyUpdatesExtra(t *testing.T) {
	t.Parallel()

	repo := &accountUsageCodexProbeRepo{
		updateExtraCh: make(chan map[string]any, 1),
		rateLimitCh:   make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	svc.persistOpenAICodexProbeSnapshot(321, map[string]any{
		"codex_7d_used_percent": 100.0,
		"codex_7d_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
	})

	select {
	case updates := <-repo.updateExtraCh:
		if got := updates["codex_7d_used_percent"]; got != 100.0 {
			t.Fatalf("codex_7d_used_percent = %v, want 100", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 探测快照写入 extra 超时")
	}

	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("不应将探测快照写入运行时限流状态: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestAccountUsageService_GetOpenAIUsage_DoesNotPromoteCodexExtraToRateLimit(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(6 * 24 * time.Hour).UTC().Truncate(time.Second)
	repo := &accountUsageCodexProbeRepo{
		rateLimitCh: make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent": 1.0,
			"codex_5h_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     resetAt.Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 100.0 {
		t.Fatalf("预期 7 天用量仍然可见，实际为 %#v", usage.SevenDay)
	}
	if account.RateLimitResetAt != nil {
		t.Fatalf("不应让已耗尽的 codex extra 改写运行时限流状态: %v", account.RateLimitResetAt)
	}
	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("不应将已耗尽的 codex extra 持久化为运行时限流状态: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestAccountUsageService_GetOpenAIUsage_IgnoresMismatchedCodexSnapshotIdentity(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(6 * 24 * time.Hour).UTC().Truncate(time.Second)
	svc := &AccountUsageService{}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"email":              "CageLeen9208@outlook.com",
			"chatgpt_account_id": "1f945aa7-d9a9-4369-9542-0c702ff4adb0",
			"workspace_id":       "org-nU4goUxMmureroyswT5oYPv4",
		},
		Extra: map[string]any{
			"email":                 "MasonDobies01@outlook.com",
			"name":                  "Paul Clark",
			"workspace_id":          "org-avRk1G4qdXg7qph3cRIraNKf",
			"codex_5h_used_percent": 4.0,
			"codex_5h_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     resetAt.Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account, false)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.FiveHour != nil {
		t.Fatalf("expected mismatched 5h codex snapshot to be ignored, got %#v", usage.FiveHour)
	}
	if usage.SevenDay != nil {
		t.Fatalf("expected mismatched 7d codex snapshot to be ignored, got %#v", usage.SevenDay)
	}
}

func TestBuildCodexUsageProgressFromExtra_ZerosExpiredWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	t.Run("expired 5h window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     "2026-03-16T10:00:00Z", // 2h ago
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired window, got %v", progress.Utilization)
		}
		if progress.RemainingSeconds != 0 {
			t.Fatalf("expected RemainingSeconds=0, got %v", progress.RemainingSeconds)
		}
	})

	t.Run("active 5h window keeps utilization", func(t *testing.T) {
		resetAt := now.Add(2 * time.Hour).Format(time.RFC3339)
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     resetAt,
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 42.0 {
			t.Fatalf("expected Utilization=42, got %v", progress.Utilization)
		}
	})

	t.Run("expired 7d window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_7d_used_percent": 88.0,
			"codex_7d_reset_at":     "2026-03-15T00:00:00Z", // yesterday
		}
		progress := buildCodexUsageProgressFromExtra(extra, "7d", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired 7d window, got %v", progress.Utilization)
		}
	})
}

func TestAccountUsageService_GetUsage_APIKeyReturnsBadRequest(t *testing.T) {
	t.Parallel()

	repo := &stubOpenAIAccountRepo{
		accounts: []Account{{
			ID:       38070,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
		}},
	}
	svc := &AccountUsageService{
		accountRepo: repo,
		cache:       NewUsageCache(),
	}

	usage, err := svc.GetUsage(context.Background(), 38070)
	if usage != nil {
		t.Fatalf("expected nil usage, got %#v", usage)
	}
	if !infraerrors.IsBadRequest(err) {
		t.Fatalf("expected bad request error, got %v", err)
	}
}

type geminiUsageLogRepoStub struct {
	UsageLogRepository
	modelStats []usagestats.ModelStat
}

var _ UsageLogRepository = (*geminiUsageLogRepoStub)(nil)

func (r *geminiUsageLogRepoStub) Create(_ context.Context, _ *UsageLog) (bool, error) {
	return true, nil
}

func (r *geminiUsageLogRepoStub) GetByID(_ context.Context, _ int64) (*UsageLog, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) Delete(_ context.Context, _ int64) error {
	return nil
}

func (r *geminiUsageLogRepoStub) ListByUser(_ context.Context, _ int64, _ pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *geminiUsageLogRepoStub) ListByAPIKey(_ context.Context, _ int64, _ pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *geminiUsageLogRepoStub) ListByAccount(_ context.Context, _ int64, _ pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *geminiUsageLogRepoStub) ListByUserAndTimeRange(_ context.Context, _ int64, _, _ time.Time) ([]UsageLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *geminiUsageLogRepoStub) ListByAPIKeyAndTimeRange(_ context.Context, _ int64, _, _ time.Time) ([]UsageLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *geminiUsageLogRepoStub) ListByAccountAndTimeRange(_ context.Context, _ int64, _, _ time.Time) ([]UsageLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *geminiUsageLogRepoStub) ListByModelAndTimeRange(_ context.Context, _ string, _, _ time.Time) ([]UsageLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *geminiUsageLogRepoStub) GetAccountWindowStats(_ context.Context, _ int64, _ time.Time) (*usagestats.AccountStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetAccountTodayStats(_ context.Context, _ int64) (*usagestats.AccountStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetDashboardStats(_ context.Context) (*usagestats.DashboardStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetUsageTrendWithFilters(_ context.Context, _, _ time.Time, _ string, _, _, _, _ int64, _ string, _ *int16, _ *bool, _ *int8, _ string) ([]usagestats.TrendDataPoint, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetModelStatsWithFilters(ctx context.Context, startTime, endTime time.Time, userID, apiKeyID, accountID, groupID int64, requestType *int16, stream *bool, billingType *int8, billingMode string) ([]usagestats.ModelStat, error) {
	return r.modelStats, nil
}

func (r *geminiUsageLogRepoStub) GetEndpointStatsWithFilters(_ context.Context, _, _ time.Time, _, _, _, _ int64, _ string, _ *int16, _ *bool, _ *int8) ([]usagestats.EndpointStat, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetUpstreamEndpointStatsWithFilters(_ context.Context, _, _ time.Time, _, _, _, _ int64, _ string, _ *int16, _ *bool, _ *int8) ([]usagestats.EndpointStat, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetGroupStatsWithFilters(_ context.Context, _, _ time.Time, _, _, _, _ int64, _ *int16, _ *bool, _ *int8, _ string) ([]usagestats.GroupStat, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetUserBreakdownStats(_ context.Context, _, _ time.Time, _ usagestats.UserBreakdownDimension, _ int) ([]usagestats.UserBreakdownItem, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetAllGroupUsageSummary(_ context.Context, _ time.Time) ([]usagestats.GroupUsageSummary, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetAPIKeyUsageTrend(_ context.Context, _, _ time.Time, _ string, _ int) ([]usagestats.APIKeyUsageTrendPoint, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetUserUsageTrend(_ context.Context, _, _ time.Time, _ string, _ int) ([]usagestats.UserUsageTrendPoint, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetUserSpendingRanking(_ context.Context, _, _ time.Time, _ int) (*usagestats.UserSpendingRankingResponse, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetBatchUserUsageStats(_ context.Context, _ []int64, _, _ time.Time) (map[int64]*usagestats.BatchUserUsageStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetBatchAPIKeyUsageStats(_ context.Context, _ []int64, _, _ time.Time) (map[int64]*usagestats.BatchAPIKeyUsageStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetUserDashboardStats(_ context.Context, _ int64) (*usagestats.UserDashboardStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetAPIKeyDashboardStats(_ context.Context, _ int64) (*usagestats.UserDashboardStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetUserUsageTrendByUserID(_ context.Context, _ int64, _, _ time.Time, _ string) ([]usagestats.TrendDataPoint, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetUserModelStats(_ context.Context, _ int64, _, _ time.Time) ([]usagestats.ModelStat, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) ListWithFilters(_ context.Context, _ pagination.PaginationParams, _ usagestats.UsageLogFilters) ([]UsageLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *geminiUsageLogRepoStub) GetGlobalStats(_ context.Context, _, _ time.Time) (*usagestats.UsageStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetStatsWithFilters(_ context.Context, _ usagestats.UsageLogFilters) (*usagestats.UsageStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetAccountUsageStats(_ context.Context, _ int64, _, _ time.Time) (*usagestats.AccountUsageStatsResponse, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetUserStatsAggregated(_ context.Context, _ int64, _, _ time.Time) (*usagestats.UsageStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetAPIKeyStatsAggregated(_ context.Context, _ int64, _, _ time.Time) (*usagestats.UsageStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetAccountStatsAggregated(_ context.Context, _ int64, _, _ time.Time) (*usagestats.UsageStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetModelStatsAggregated(_ context.Context, _ string, _, _ time.Time) (*usagestats.UsageStats, error) {
	return nil, nil
}

func (r *geminiUsageLogRepoStub) GetDailyStatsAggregated(_ context.Context, _ int64, _, _ time.Time) ([]map[string]any, error) {
	return nil, nil
}

func TestAccountUsageService_GetGeminiUsage_PrefersQuotaSnapshotUtilization(t *testing.T) {
	t.Parallel()

	repo := &geminiUsageLogRepoStub{
		modelStats: []usagestats.ModelStat{
			{Model: "gemini-2.5-pro", Requests: 12, TotalTokens: 1200, ActualCost: 0.12},
			{Model: "gemini-2.5-flash", Requests: 30, TotalTokens: 3000, ActualCost: 0.03},
		},
	}

	svc := &AccountUsageService{
		usageLogRepo:       repo,
		geminiQuotaService: NewGeminiQuotaService(nil, nil),
	}

	account := &Account{
		ID:       9001,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"oauth_type": "code_assist",
			"tier_id":    "STANDARD",
			"gemini_usage_raw": map[string]any{
				"buckets": []any{
					map[string]any{
						"modelId":           "gemini-2.5-pro",
						"remainingFraction": 0.25,
						"remainingAmount":   "25",
						"resetTime":         time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339),
					},
					map[string]any{
						"modelId":           "gemini-2.5-flash",
						"remainingFraction": 0.60,
						"remainingAmount":   "60",
						"resetTime":         time.Now().Add(90 * time.Minute).UTC().Format(time.RFC3339),
					},
				},
			},
		},
	}

	usage, err := svc.getGeminiUsage(context.Background(), account)
	if err != nil {
		t.Fatalf("getGeminiUsage() error = %v", err)
	}
	if usage.GeminiProDaily == nil || usage.GeminiFlashDaily == nil {
		t.Fatalf("expected gemini daily windows, got %#v", usage)
	}
	if usage.GeminiProDaily.Utilization != 75 {
		t.Fatalf("GeminiProDaily utilization = %v, want 75", usage.GeminiProDaily.Utilization)
	}
	if usage.GeminiFlashDaily.Utilization != 40 {
		t.Fatalf("GeminiFlashDaily utilization = %v, want 40", usage.GeminiFlashDaily.Utilization)
	}
	if usage.GeminiProDaily.WindowStats == nil || usage.GeminiProDaily.WindowStats.Requests != 12 {
		t.Fatalf("GeminiProDaily window stats should retain local usage logs: %#v", usage.GeminiProDaily.WindowStats)
	}
	if usage.GeminiFlashDaily.WindowStats == nil || usage.GeminiFlashDaily.WindowStats.Requests != 30 {
		t.Fatalf("GeminiFlashDaily window stats should retain local usage logs: %#v", usage.GeminiFlashDaily.WindowStats)
	}
	if usage.GeminiSharedDaily == nil || usage.GeminiSharedDaily.Utilization != 75 {
		t.Fatalf("GeminiSharedDaily utilization = %#v, want 75", usage.GeminiSharedDaily)
	}
}

func TestAccountUsageService_GetGeminiUsage_UsesStoredForbiddenStatus(t *testing.T) {
	t.Parallel()

	repo := &geminiUsageLogRepoStub{}
	svc := &AccountUsageService{
		usageLogRepo:       repo,
		geminiQuotaService: NewGeminiQuotaService(nil, nil),
	}

	account := &Account{
		ID:       9002,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"oauth_type":             "code_assist",
			"tier_id":                "STANDARD",
			"gemini_status":          "forbidden",
			"gemini_status_reason":   "retrieveUserQuota failed: status 403, body: forbidden",
			"quota_query_last_error": "retrieveUserQuota failed: status 403, body: forbidden",
		},
	}

	usage, err := svc.getGeminiUsage(context.Background(), account)
	if err != nil {
		t.Fatalf("getGeminiUsage() error = %v", err)
	}
	if !usage.IsForbidden {
		t.Fatal("expected Gemini usage to expose stored forbidden status")
	}
	if usage.ErrorCode != errorCodeForbidden {
		t.Fatalf("ErrorCode = %q, want %q", usage.ErrorCode, errorCodeForbidden)
	}
	if usage.ForbiddenReason == "" {
		t.Fatal("expected forbidden reason to be preserved")
	}
	if usage.Error == "" {
		t.Fatal("expected quota_query_last_error to surface as usage error")
	}
}
