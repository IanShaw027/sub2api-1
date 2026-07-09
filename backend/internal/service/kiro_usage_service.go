package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"go.uber.org/zap"
)

type KiroUsageLimits struct {
	NextDateReset    *float64              `json:"nextDateReset"`
	SubscriptionInfo *KiroSubscriptionInfo `json:"subscriptionInfo"`
	UsageBreakdowns  []KiroUsageBreakdown  `json:"usageBreakdownList"`
}

type KiroSubscriptionInfo struct {
	SubscriptionTitle *string `json:"subscriptionTitle"`
}

type KiroUsageBreakdown struct {
	CurrentUsageWithPrecision float64          `json:"currentUsageWithPrecision"`
	UsageLimitWithPrecision   float64          `json:"usageLimitWithPrecision"`
	Bonuses                   []KiroUsageBonus `json:"bonuses"`
	FreeTrialInfo             *KiroFreeTrial   `json:"freeTrialInfo"`
	NextDateReset             *float64         `json:"nextDateReset"`
}

type KiroUsageBonus struct {
	CurrentUsage float64 `json:"currentUsage"`
	UsageLimit   float64 `json:"usageLimit"`
	Status       *string `json:"status"`
}

type KiroFreeTrial struct {
	CurrentUsageWithPrecision float64 `json:"currentUsageWithPrecision"`
	UsageLimitWithPrecision   float64 `json:"usageLimitWithPrecision"`
	FreeTrialStatus           *string `json:"freeTrialStatus"`
}

type KiroUsageService struct {
	httpUpstream        HTTPUpstream
	tlsFPProfileService *TLSFingerprintProfileService
	settingService      *SettingService
	proxyRepo           ProxyRepository
}

func NewKiroUsageService() *KiroUsageService {
	return &KiroUsageService{}
}

func (s *KiroUsageService) WithTransport(httpUpstream HTTPUpstream, tlsFPProfileService *TLSFingerprintProfileService) *KiroUsageService {
	if s == nil {
		return nil
	}
	s.httpUpstream = httpUpstream
	s.tlsFPProfileService = tlsFPProfileService
	return s
}

func (s *KiroUsageService) WithSettingService(settingService *SettingService) *KiroUsageService {
	if s == nil {
		return nil
	}
	s.settingService = settingService
	return s
}

func (s *KiroUsageService) WithProxyRepo(proxyRepo ProxyRepository) *KiroUsageService {
	if s == nil {
		return nil
	}
	s.proxyRepo = proxyRepo
	return s
}

func (s *KiroUsageService) FetchUsageLimits(ctx context.Context, account *Account, accessToken string) (*KiroUsageLimits, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	account = s.prepareAccount(ctx, account)
	host := fmt.Sprintf("q.%s.amazonaws.com", KiroRegion(account))
	params := url.Values{
		"origin":       {"AI_EDITOR"},
		"resourceType": {"AGENTIC_REQUEST"},
	}
	if profileARN := account.GetCredential("profile_arn"); profileARN != "" {
		params.Set("profileArn", profileARN)
	}
	url := fmt.Sprintf("https://%s/getUsageLimits?%s", host, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	runtimeSettings := DefaultKiroRuntimeSettings()
	if s != nil && s.settingService != nil {
		runtimeSettings = s.settingService.GetKiroRuntimeSettings(ctx)
	}
	machineID := kiro.GenerateMachineID(account.GetCredential("machine_id"), account.GetCredential("refresh_token"))
	kiroVersion := runtimeSettings.KiroVersion
	req.Header.Set("Authorization", "Bearer "+accessToken)
	if isKiroExternalIDPAccount(account) {
		req.Header.Set("TokenType", "EXTERNAL_IDP")
	}
	req.Header.Set("host", host)
	req.Header.Set("amz-sdk-invocation-id", generateRequestID())
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	xAmzUserAgent, userAgent := kiro.BuildCodeWhispererRuntimeUserAgents(kiroVersion, machineID, runtimeSettings.SystemVersion, runtimeSettings.NodeVersion)
	req.Header.Set("x-amz-user-agent", xAmzUserAgent)
	req.Header.Set("User-Agent", userAgent)
	if runtimeSettings.KiroCommit != "" {
		req.Header.Set("x-amzn-kiro-commit", runtimeSettings.KiroCommit)
	}

	resp, err := doKiroSidecarRequest(req, account, s.httpUpstream, s.tlsFPProfileService, 60*time.Second)
	if err != nil {
		kiroLogger(ctx, account).Warn("kiro.usage_limits_failed", zap.Error(err))
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := buildKiroUsageUpstreamError(resp)
		kiroLogger(ctx, account).Warn("kiro.usage_limits_failed", zap.Int("status_code", resp.StatusCode), zap.Error(err))
		return nil, err
	}

	var out KiroUsageLimits
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		kiroLogger(ctx, account).Warn("kiro.usage_limits_failed", zap.Error(err))
		return nil, err
	}
	fields := []zap.Field{
		zap.Float64("current_usage", out.CurrentUsage()),
		zap.Float64("usage_limit", out.UsageLimit()),
		zap.Float64("remaining_usage", out.Remaining()),
		zap.String("subscription_title", out.SubscriptionTitle()),
	}
	if resetAt := out.ResetAt(); resetAt != nil {
		fields = append(fields, zap.Time("reset_at", *resetAt))
	}
	kiroLogger(ctx, account).Info("kiro.usage_limits_fetched", fields...)
	return &out, nil
}

func buildKiroUsageUpstreamError(resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("kiro usage upstream returned invalid response")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("kiro usage upstream returned %d", resp.StatusCode)
	}
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("kiro usage upstream returned %d", resp.StatusCode)
	}

	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		parts := []string{
			firstNonEmptyStringValue(payload, "error"),
			firstNonEmptyStringValue(payload, "error_description", "errorDescription", "message"),
		}
		if joined := strings.TrimSpace(strings.Join(filterEmptyStrings(parts), ": ")); joined != "" {
			detail = joined
		}
	}

	return fmt.Errorf("kiro usage upstream returned %d: %s", resp.StatusCode, detail)
}

func (s *KiroUsageService) prepareAccount(ctx context.Context, account *Account) *Account {
	if account == nil || account.Proxy != nil || account.ProxyID == nil || s == nil || s.proxyRepo == nil {
		return account
	}
	if ctx == nil {
		ctx = context.Background()
	}
	proxy, err := s.proxyRepo.GetByID(ctx, *account.ProxyID)
	if err != nil || proxy == nil {
		return account
	}
	cloned := *account
	cloned.Proxy = proxy
	return &cloned
}

func doKiroSidecarRequest(
	req *http.Request,
	account *Account,
	httpUpstream HTTPUpstream,
	tlsFPProfileService *TLSFingerprintProfileService,
	timeout time.Duration,
) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	if httpUpstream != nil {
		return httpUpstream.DoWithTLS(
			req,
			accountProxyURL(account),
			account.ID,
			account.Concurrency,
			resolveKiroTLSProfile(account, tlsFPProfileService),
		)
	}

	client, err := newKiroSidecarHTTPClient(account, tlsFPProfileService, timeout)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func normalizeKiroTransportConcurrency(concurrency int) int {
	if concurrency > 0 {
		return concurrency
	}
	return 1
}

func (k *KiroUsageLimits) SubscriptionTitle() string {
	if k == nil || k.SubscriptionInfo == nil || k.SubscriptionInfo.SubscriptionTitle == nil {
		return ""
	}
	return *k.SubscriptionInfo.SubscriptionTitle
}

func (k *KiroUsageLimits) CurrentUsage() float64 {
	return k.MonthlyCurrentUsage() + k.FreeTrialCurrentUsage() + k.BonusCurrentUsage()
}

func (k *KiroUsageLimits) UsageLimit() float64 {
	return k.MonthlyUsageLimit() + k.FreeTrialUsageLimit() + k.BonusUsageLimit()
}

func (k *KiroUsageLimits) MonthlyCurrentUsage() float64 {
	breakdown := k.primaryBreakdown()
	if breakdown == nil {
		return 0
	}
	return breakdown.CurrentUsageWithPrecision
}

func (k *KiroUsageLimits) MonthlyUsageLimit() float64 {
	breakdown := k.primaryBreakdown()
	if breakdown == nil {
		return 0
	}
	return breakdown.UsageLimitWithPrecision
}

func (k *KiroUsageLimits) BonusCurrentUsage() float64 {
	breakdown := k.primaryBreakdown()
	if breakdown == nil {
		return 0
	}
	total := 0.0
	for _, bonus := range breakdown.Bonuses {
		if bonus.isActive() {
			total += bonus.CurrentUsage
		}
	}
	return total
}

func (k *KiroUsageLimits) BonusUsageLimit() float64 {
	breakdown := k.primaryBreakdown()
	if breakdown == nil {
		return 0
	}
	total := 0.0
	for _, bonus := range breakdown.Bonuses {
		if bonus.isActive() {
			total += bonus.UsageLimit
		}
	}
	return total
}

func (k *KiroUsageLimits) FreeTrialCurrentUsage() float64 {
	breakdown := k.primaryBreakdown()
	if breakdown == nil {
		return 0
	}
	if breakdown.FreeTrialInfo != nil && breakdown.FreeTrialInfo.isActive() {
		return breakdown.FreeTrialInfo.CurrentUsageWithPrecision
	}
	return 0
}

func (k *KiroUsageLimits) FreeTrialUsageLimit() float64 {
	breakdown := k.primaryBreakdown()
	if breakdown == nil {
		return 0
	}
	if breakdown.FreeTrialInfo != nil && breakdown.FreeTrialInfo.isActive() {
		return breakdown.FreeTrialInfo.UsageLimitWithPrecision
	}
	return 0
}

func (k *KiroUsageLimits) Remaining() float64 {
	remaining := k.UsageLimit() - k.CurrentUsage()
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (k *KiroUsageLimits) ResetAt() *time.Time {
	if k == nil {
		return nil
	}
	if breakdown := k.primaryBreakdown(); breakdown != nil && breakdown.NextDateReset != nil {
		t := unixFloatToTime(*breakdown.NextDateReset)
		return &t
	}
	if k.NextDateReset != nil {
		t := unixFloatToTime(*k.NextDateReset)
		return &t
	}
	return nil
}

func (k *KiroUsageLimits) primaryBreakdown() *KiroUsageBreakdown {
	if k == nil || len(k.UsageBreakdowns) == 0 {
		return nil
	}
	return &k.UsageBreakdowns[0]
}

func (b KiroUsageBonus) isActive() bool {
	return b.Status != nil && *b.Status == "ACTIVE"
}

func (f *KiroFreeTrial) isActive() bool {
	return f != nil && f.FreeTrialStatus != nil && *f.FreeTrialStatus == "ACTIVE"
}

func unixFloatToTime(v float64) time.Time {
	if v >= 1e12 {
		v = v / 1000
	}
	sec := int64(v)
	nsec := int64((v - float64(sec)) * float64(time.Second))
	return time.Unix(sec, nsec).UTC()
}
