package service

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

type kiroUsageCache struct {
	usageInfo *UsageInfo
	timestamp time.Time
}

type KiroQuotaBreakdown struct {
	CurrentUsage float64    `json:"current_usage"`
	UsageLimit   float64    `json:"usage_limit"`
	Remaining    float64    `json:"remaining"`
	Utilization  float64    `json:"utilization"`
	ResetsAt     *time.Time `json:"resets_at,omitempty"`
}

// NewKiroUsageService returns a Kiro usage client with the same transport,
// fingerprint, proxy and runtime-setting dependencies as the live gateway.
func (s *AccountUsageService) NewKiroUsageService() *KiroUsageService {
	if s == nil {
		return NewKiroUsageService()
	}
	return NewKiroUsageService().
		WithTransport(s.kiroHTTPUpstream(), s.tlsFPProfileService).
		WithSettingService(s.settingService).
		WithProxyRepo(s.proxyRepo)
}
func (s *AccountUsageService) getKiroUsage(ctx context.Context, account *Account) (*UsageInfo, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	if s.cache != nil {
		if cached, ok := s.cache.kiroCache.Load(account.ID); ok {
			if cache, ok := cached.(*kiroUsageCache); ok {
				ttl := kiroUsageCacheTTL(cache.usageInfo)
				if time.Since(cache.timestamp) < ttl {
					usage := cache.usageInfo
					recalcKiroRemainingSeconds(usage)
					return usage, nil
				}
			}
		}
	}
	var loadUsage func(fetchCtx context.Context, forceRefresh bool) (*UsageInfo, error)
	loadUsage = func(fetchCtx context.Context, forceRefresh bool) (*UsageInfo, error) {
		kiroHTTPUpstream := s.kiroHTTPUpstream()
		accessToken := ""
		if forceRefresh {
			// 走带分布式锁+DB 重读+invalid_grant 竞争恢复+credentials-only 持久化的统一刷新入口，
			// 不再裸调 refresher.Refresh + 整行 accountRepo.Update（会与网关侧带锁刷新并发 lost-update
			// 掉 token、把账号推上分叉的 refresh_token 链最终变砖）。RefreshAccount 用 kiroAdminRefreshSkew
			// (100 年)使 NeedsRefresh 恒真，保留“强制换新 token”语义。
			if err := s.invalidateKiroTokenCache(fetchCtx, account); err != nil {
				log.Printf("warning: failed to invalidate Kiro token cache before usage refresh for account %d: %v", account.ID, err)
			}
			if s.kiroTokenProvider == nil {
				return buildKiroDegradedUsage(fmt.Errorf("kiro token provider is not configured")), nil
			}
			refreshedAccount, err := s.kiroTokenProvider.RefreshAccount(fetchCtx, account)
			if err != nil {
				if shouldRetryKiroUsageWithForcedRefresh(err) {
					return buildKiroDegradedUsage(fmt.Errorf("kiro usage upstream returned 401: %w", err)), nil
				}
				return buildKiroDegradedUsage(fmt.Errorf("kiro usage refresh failed after auth retry: %w", err)), nil
			}
			copyKiroAccountRuntimeState(account, refreshedAccount)
			s.InvalidateKiroUsageCache(account.ID)
			accessToken = account.GetCredential("access_token")
		} else if s.kiroTokenProvider != nil {
			var err error
			accessToken, err = s.kiroTokenProvider.GetAccessToken(fetchCtx, account)
			if err != nil {
				return buildKiroDegradedUsage(err), nil
			}
		} else {
			accessToken = account.GetCredential("access_token")
		}
		if accessToken == "" {
			if s.kiroTokenProvider == nil {
				return buildKiroDegradedUsage(fmt.Errorf("kiro token provider is not configured")), nil
			}
			refreshedAccount, err := s.kiroTokenProvider.RefreshAccount(fetchCtx, account)
			if err != nil {
				return buildKiroDegradedUsage(err), nil
			}
			copyKiroAccountRuntimeState(account, refreshedAccount)
			s.InvalidateKiroUsageCache(account.ID)
			accessToken = account.GetCredential("access_token")
		}

		usageService := s.NewKiroUsageService().WithTransport(kiroHTTPUpstream, s.tlsFPProfileService)
		limits, err := usageService.FetchUsageLimits(fetchCtx, account, accessToken)
		if err != nil {
			if !forceRefresh && shouldRetryKiroUsageWithForcedRefresh(err) {
				s.markKiroAccessTokenRejected(fetchCtx, account)
				if s.kiroTokenProvider == nil {
					// 首次请求拿旧 access token 命中 401 时，会尝试“强制刷新后重试”来恢复。
					// 但若当前实例根本未配置 refresh 能力（测试/裁剪部署），不能把原始 401
					// 降级语义改写成“provider 未配置”的网络错误，否则会丢掉 NeedsReauth/401
					// 信号并错误清洗掉账号的可恢复认证态。
					return buildKiroDegradedUsage(fmt.Errorf("kiro usage upstream returned 401: %w", err)), nil
				}
				return loadUsage(fetchCtx, true)
			}
			return buildKiroDegradedUsage(err), nil
		}

		currentUsage := limits.CurrentUsage()
		usageLimit := limits.UsageLimit()
		remaining := limits.Remaining()
		utilization := 0.0
		if usageLimit > 0 {
			utilization = (currentUsage / usageLimit) * 100
		}
		var resetAt *time.Time
		if t := limits.ResetAt(); t != nil {
			resetAt = t
		}
		now := time.Now()

		monthlyQuota := buildKiroQuotaBreakdown(limits.MonthlyCurrentUsage(), limits.MonthlyUsageLimit(), resetAt)
		bonusQuota := buildKiroQuotaBreakdown(limits.BonusCurrentUsage(), limits.BonusUsageLimit(), nil)
		freeTrialQuota := buildKiroQuotaBreakdown(limits.FreeTrialCurrentUsage(), limits.FreeTrialUsageLimit(), nil)
		totalQuota := buildKiroQuotaBreakdown(currentUsage, usageLimit, resetAt)
		windowStats := &WindowStats{}
		if s.usageLogRepo != nil {
			startTime := timezone.StartOfMonth(now)
			if stats, err := s.usageLogRepo.GetAccountWindowStats(fetchCtx, account.ID, startTime); err == nil && stats != nil {
				windowStats = windowStatsFromAccountStats(stats)
			} else if err != nil {
				log.Printf("Failed to get Kiro window stats for account %d: %v", account.ID, err)
			}
		}
		info := &UsageInfo{
			KiroSubscriptionTitle: limits.SubscriptionTitle(),
			KiroCurrentUsage:      currentUsage,
			KiroUsageLimit:        usageLimit,
			KiroRemaining:         remaining,
			KiroEmail:             limits.Email(),
			KiroOverageCapability: limits.OverageCapability(),
			KiroOverageEnabled:    limits.OverageEnabled(),
			KiroProfileID:         firstNonEmptyKiroString(account.GetCredential("profile_id"), account.GetExtraString("profile_id")),
			KiroLoginProvider:     firstNonEmptyKiroString(account.GetCredential("login_provider"), account.GetExtraString("login_provider")),
			KiroStatusReason:      firstNonEmptyKiroString(account.GetCredential("status_reason"), account.GetCredential("kiro_status_reason"), account.GetExtraString("status_reason"), account.GetExtraString("kiro_status_reason")),
			KiroMonthlyQuota:      monthlyQuota,
			KiroBonusQuota:        bonusQuota,
			KiroFreeTrialQuota:    freeTrialQuota,
			KiroTotalQuota:        totalQuota,
			KiroQuota: &UsageProgress{
				Utilization: utilization,
				ResetsAt:    resetAt,
				WindowStats: windowStats,
			},
		}
		info.UpdatedAt = &now
		recalcKiroRemainingSeconds(info)
		s.persistKiroSchedulerSnapshot(fetchCtx, account, info)
		return info, nil
	}

	if s.cache == nil {
		return loadUsage(ctx, false)
	}

	flightKey := fmt.Sprintf("kiro-usage:%d", account.ID)
	result, flightErr, _ := s.cache.kiroFlight.Do(flightKey, func() (any, error) {
		if cached, ok := s.cache.kiroCache.Load(account.ID); ok {
			if cache, ok := cached.(*kiroUsageCache); ok {
				ttl := kiroUsageCacheTTL(cache.usageInfo)
				if time.Since(cache.timestamp) < ttl {
					usage := cache.usageInfo
					recalcKiroRemainingSeconds(usage)
					return usage, nil
				}
			}
		}

		fetchCtx, fetchCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer fetchCancel()

		usage, err := loadUsage(fetchCtx, false)
		if err != nil {
			return nil, err
		}
		s.cache.kiroCache.Store(account.ID, &kiroUsageCache{
			usageInfo: usage,
			timestamp: time.Now(),
		})
		return usage, nil
	})
	if flightErr != nil {
		return nil, flightErr
	}
	usage, ok := result.(*UsageInfo)
	if !ok || usage == nil {
		now := time.Now()
		return &UsageInfo{UpdatedAt: &now}, nil
	}
	return usage, nil
}

func shouldRetryKiroUsageWithForcedRefresh(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "kiro usage upstream returned 401") ||
		strings.Contains(errStr, "kiro refresh token is invalid") ||
		strings.Contains(errStr, "kiro_refresh_token_invalid") ||
		strings.Contains(errStr, "invalid_grant") ||
		strings.Contains(errStr, "invalid token") ||
		strings.Contains(errStr, "invalid_token") ||
		strings.Contains(errStr, "expired token") ||
		strings.Contains(errStr, "token expired") ||
		strings.Contains(errStr, "unauthorized")
}

func buildKiroDegradedUsage(err error) *UsageInfo {
	now := time.Now()
	info := &UsageInfo{
		UpdatedAt: &now,
		Error:     "Failed to fetch Kiro usage: " + err.Error(),
	}

	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "returned 401"),
		strings.Contains(errStr, "invalid_grant"),
		strings.Contains(errStr, "unauthenticated"),
		strings.Contains(errStr, "access_token not found"):
		info.ErrorCode = errorCodeUnauthenticated
		info.NeedsReauth = true
	case strings.Contains(errStr, "returned 403"),
		strings.Contains(errStr, "forbidden"):
		info.ErrorCode = errorCodeForbidden
		info.IsForbidden = true
		info.ForbiddenType = forbiddenTypeForbidden
		info.ForbiddenReason = err.Error()
	case strings.Contains(errStr, "returned 429"):
		info.ErrorCode = errorCodeRateLimited
	default:
		info.ErrorCode = errorCodeNetworkError
	}

	return info
}

func buildKiroQuotaBreakdown(currentUsage, usageLimit float64, resetAt *time.Time) *KiroQuotaBreakdown {
	if currentUsage <= 0 && usageLimit <= 0 {
		return nil
	}
	remaining := usageLimit - currentUsage
	if remaining < 0 {
		remaining = 0
	}
	utilization := 0.0
	if usageLimit > 0 {
		utilization = (currentUsage / usageLimit) * 100
	}
	return &KiroQuotaBreakdown{
		CurrentUsage: currentUsage,
		UsageLimit:   usageLimit,
		Remaining:    remaining,
		Utilization:  utilization,
		ResetsAt:     resetAt,
	}
}

func recalcKiroRemainingSeconds(info *UsageInfo) {
	if info == nil || info.KiroQuota == nil || info.KiroQuota.ResetsAt == nil {
		return
	}
	remaining := int(time.Until(*info.KiroQuota.ResetsAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}
	info.KiroQuota.RemainingSeconds = remaining
}

func kiroUsageCacheTTL(info *UsageInfo) time.Duration {
	if info == nil {
		return apiErrorCacheTTL
	}
	if info.ErrorCode != "" || info.Error != "" || info.NeedsReauth || info.IsForbidden {
		return apiErrorCacheTTL
	}
	return apiCacheTTL
}

func (s *AccountUsageService) InvalidateKiroUsageCache(accountID int64) {
	if s == nil || s.cache == nil || accountID <= 0 {
		return
	}
	s.cache.kiroCache.Delete(accountID)
	s.cache.kiroFlight.Forget(fmt.Sprintf("kiro-usage:%d", accountID))
}

func (s *AccountUsageService) kiroHTTPUpstream() HTTPUpstream {
	if s == nil || s.usageFetcher == nil {
		return nil
	}
	type httpUpstreamProvider interface {
		HTTPUpstream() HTTPUpstream
	}
	if provider, ok := s.usageFetcher.(httpUpstreamProvider); ok {
		return provider.HTTPUpstream()
	}
	return nil
}

func (s *AccountUsageService) invalidateKiroTokenCache(ctx context.Context, account *Account) error {
	if s.tokenCacheInvalidator != nil {
		if err := s.tokenCacheInvalidator.InvalidateToken(ctx, account); err != nil {
			log.Printf("warning: failed to invalidate Kiro token cache after usage refresh for account %d: %v", account.ID, err)
			return err
		}
	}
	return nil
}

func (s *AccountUsageService) markKiroAccessTokenRejected(ctx context.Context, account *Account) {
	if s == nil || account == nil {
		return
	}
	credentials := cloneCredentials(account.Credentials)
	if credentials == nil {
		credentials = make(map[string]any, 1)
	}
	credentials["expires_at"] = time.Now().Add(-kiroTokenRefreshSkew).UTC().Format(time.RFC3339)
	account.Credentials = credentials

	updater, ok := any(s.accountRepo).(accountCredentialsUpdater)
	if !ok || updater == nil {
		return
	}
	if err := updater.UpdateCredentials(ctx, account.ID, credentials); err != nil {
		log.Printf("warning: failed to persist rejected Kiro access token state for account %d: %v", account.ID, err)
	}
}

// persistRefreshedKiroCredentials keeps unit tests and legacy refresh paths on the
// same semantics after Kiro usage refresh was consolidated around RefreshAccount.
// It persists the refreshed credentials, updates the caller's in-memory account,
// invalidates the cached usage snapshot, and best-effort invalidates token cache.
func (s *AccountUsageService) persistRefreshedKiroCredentials(ctx context.Context, account *Account, newCreds map[string]any) error {
	if s == nil || account == nil {
		return nil
	}
	updated := *account
	updated.Credentials = cloneCredentials(newCreds)
	if err := s.accountRepo.Update(ctx, &updated); err != nil {
		return err
	}
	account.Credentials = cloneCredentials(newCreds)
	s.InvalidateKiroUsageCache(account.ID)
	_ = s.invalidateKiroTokenCache(ctx, account)
	return nil
}

func buildKiroSchedulerSnapshotExtraUpdates(info *UsageInfo) map[string]any {
	if info == nil || info.KiroQuota == nil {
		return nil
	}
	if info.Error != "" || info.ErrorCode != "" || info.IsForbidden || info.NeedsReauth {
		return nil
	}

	updatedAt := time.Now().UTC()
	if info.UpdatedAt != nil {
		updatedAt = info.UpdatedAt.UTC()
	}

	updates := map[string]any{
		"kiro_sched_utilization":      info.KiroQuota.Utilization,
		"kiro_sched_usage_updated_at": updatedAt.Format(time.RFC3339),
	}
	if info.KiroQuota.ResetsAt != nil {
		updates["kiro_sched_reset_at"] = info.KiroQuota.ResetsAt.UTC().Format(time.RFC3339)
	}

	return updates
}

func (s *AccountUsageService) persistKiroSchedulerSnapshot(ctx context.Context, account *Account, info *UsageInfo) {
	s.persistSchedulerSnapshotExtra(ctx, account, buildKiroSchedulerSnapshotExtraUpdates(info), "kiro")
}

func (s *AccountUsageService) persistAntigravitySchedulerSnapshot(ctx context.Context, account *Account, info *UsageInfo) {
	s.persistSchedulerSnapshotExtra(ctx, account, buildAntigravitySchedulerSnapshotExtraUpdates(info), "antigravity")
}

func (s *AccountUsageService) persistSchedulerSnapshotExtra(ctx context.Context, account *Account, updates map[string]any, platform string) {
	if s == nil || s.accountRepo == nil || account == nil || account.ID <= 0 || len(updates) == 0 {
		return
	}
	if err := s.accountRepo.UpdateExtra(ctx, account.ID, updates); err != nil {
		slog.Warn("persist_scheduler_snapshot_failed", "platform", platform, "account_id", account.ID, "error", err)
		return
	}
	mergeAccountExtra(account, updates)
}
