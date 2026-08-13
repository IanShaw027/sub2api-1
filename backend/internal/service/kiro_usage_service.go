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
	NextDateReset        *float64                  `json:"nextDateReset"`
	SubscriptionInfo     *KiroSubscriptionInfo     `json:"subscriptionInfo"`
	UsageBreakdowns      []KiroUsageBreakdown      `json:"usageBreakdownList"`
	OverageConfiguration *KiroOverageConfiguration `json:"overageConfiguration"`
	UserInfo             *KiroUserInfo             `json:"userInfo"`
}

type KiroSubscriptionInfo struct {
	SubscriptionTitle *string `json:"subscriptionTitle"`
	OverageCapability *string `json:"overageCapability"`
}

type KiroOverageConfiguration struct {
	Enabled *bool `json:"enabled"`
}

type KiroUserInfo struct {
	Email *string `json:"email"`
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

type KiroAvailableProfiles struct {
	Profiles []KiroAvailableProfile `json:"profiles"`
}

type KiroAvailableProfile struct {
	ARN         string `json:"arn"`
	ProfileARN  string `json:"profileArn"`
	ProfileName string `json:"profileName"`
}

type KiroAvailableModel struct {
	ModelID string `json:"modelId"`
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
	var firstErr error
	for _, region := range kiroRESTRegions(account) {
		out, err := s.fetchUsageLimitsInRegion(ctx, account, accessToken, region)
		if err == nil {
			if account != nil && account.Credentials != nil {
				account.Credentials["api_region"] = region
				account.Credentials["region"] = region
			}
			fields := []zap.Field{
				zap.String("region", region),
				zap.Float64("current_usage", out.CurrentUsage()),
				zap.Float64("usage_limit", out.UsageLimit()),
				zap.Float64("remaining_usage", out.Remaining()),
				zap.String("subscription_title", out.SubscriptionTitle()),
			}
			if resetAt := out.ResetAt(); resetAt != nil {
				fields = append(fields, zap.Time("reset_at", *resetAt))
			}
			kiroLogger(ctx, account).Info("kiro.usage_limits_fetched", fields...)
			return out, nil
		}
		if firstErr == nil {
			firstErr = err
		}
		kiroLogger(ctx, account).Warn("kiro.usage_limits_failed", zap.String("region", region), zap.Error(err))
		if shouldStopKiroUsageRegionFallback(err) {
			return nil, err
		}
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("kiro usage region candidates exhausted")
	}
	return nil, firstErr
}

func (s *KiroUsageService) FetchAvailableModels(ctx context.Context, account *Account, accessToken string) ([]KiroAvailableModel, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	account = s.prepareAccount(ctx, account)
	var firstErr error
	for _, region := range kiroRESTRegions(account) {
		models, err := s.fetchAvailableModelsInRegion(ctx, account, accessToken, region)
		if err == nil && len(models) > 0 {
			return models, nil
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, fmt.Errorf("kiro available models returned empty result")
}

func (s *KiroUsageService) SetOveragePreference(ctx context.Context, account *Account, accessToken string, enabled bool) error {
	if account == nil {
		return fmt.Errorf("account is required")
	}
	account = s.prepareAccount(ctx, account)
	status := "DISABLED"
	if enabled {
		status = "ENABLED"
	}
	body := map[string]any{
		"overageConfiguration": map[string]any{
			"overageStatus": status,
		},
	}
	if profileARN := strings.TrimSpace(account.GetCredential("profile_arn")); profileARN != "" {
		body["profileArn"] = profileARN
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	var firstErr error
	for _, region := range kiroRESTRegions(account) {
		host := fmt.Sprintf("q.%s.amazonaws.com", region)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host+"/setUserPreference", strings.NewReader(string(payload)))
		if err != nil {
			return err
		}
		runtimeSettings := DefaultKiroRuntimeSettings()
		if s != nil && s.settingService != nil {
			runtimeSettings = s.settingService.GetKiroRuntimeSettings(ctx)
		}
		machineID := kiro.GenerateMachineID(account.GetCredential("machine_id"), account.GetCredential("refresh_token"))
		kiroVersion := runtimeSettings.KiroVersion
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+accessToken)
		if isKiroExternalIDPAccount(account) {
			req.Header.Set("TokenType", "EXTERNAL_IDP")
		} else if account.Type == AccountTypeAPIKey {
			req.Header.Set("TokenType", "API_KEY")
		}
		req.Header.Set("host", host)
		req.Header.Set("amz-sdk-invocation-id", generateRequestID())
		req.Header.Set("amz-sdk-request", "attempt=1; max=1")
		xAmzUserAgent, userAgent := kiro.BuildCodeWhispererRuntimeUserAgents(kiroVersion, machineID, runtimeSettings.SystemVersion, runtimeSettings.NodeVersion)
		req.Header.Set("x-amz-user-agent", xAmzUserAgent)
		req.Header.Set("User-Agent", userAgent)
		resp, err := doKiroSidecarRequest(req, account, s.httpUpstream, s.tlsFPProfileService, 60*time.Second)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		err = func() error {
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			return buildKiroUsageUpstreamError(resp)
		}()
		if err == nil {
			return nil
		}
		if firstErr == nil {
			firstErr = err
		}
		if resp.StatusCode == http.StatusForbidden {
			continue
		}
		return err
	}
	if firstErr == nil {
		firstErr = fmt.Errorf("kiro setUserPreference failed")
	}
	return firstErr
}

func (s *KiroUsageService) ResolveBestProfileARN(ctx context.Context, account *Account, accessToken string) (string, *KiroUsageLimits, error) {
	if account == nil {
		return "", nil, fmt.Errorf("account is required")
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return "", nil, fmt.Errorf("access_token is required")
	}
	account = s.prepareAccount(ctx, account)

	profiles := make([]string, 0, 2)
	seen := make(map[string]bool)
	for _, region := range kiroProfileDiscoveryRegions(account) {
		out, err := s.fetchAvailableProfilesInRegion(ctx, account, accessToken, region)
		if err != nil {
			kiroLogger(ctx, account).Warn("kiro.available_profiles_failed", zap.String("region", region), zap.Error(err))
			continue
		}
		for _, profile := range out.Profiles {
			arn := strings.TrimSpace(firstNonEmptyKiroString(profile.ARN, profile.ProfileARN))
			if arn == "" || seen[arn] {
				continue
			}
			seen[arn] = true
			profiles = append(profiles, arn)
		}
	}
	if len(profiles) == 0 {
		return "", nil, fmt.Errorf("kiro available profiles returned no profileArn")
	}

	var (
		firstErr   error
		firstOKARN string
		firstOK    *KiroUsageLimits
		preferred  string
		preferredU *KiroUsageLimits
	)
	for _, arn := range profiles {
		candidate := cloneAccountForKiroProfile(account, arn)
		usage, err := s.FetchUsageLimits(ctx, candidate, accessToken)
		if err == nil {
			if firstOKARN == "" {
				firstOKARN = arn
				firstOK = usage
			}
			if profileARNRegion(arn) != "us-east-1" {
				preferred = arn
				preferredU = usage
				break
			}
			continue
		}
		if firstErr == nil {
			firstErr = err
		}
		kiroLogger(ctx, account).Warn("kiro.profile_usage_probe_failed", zap.String("profile_arn", arn), zap.Error(err))
	}
	if preferred != "" {
		return preferred, preferredU, nil
	}
	if firstOKARN != "" {
		return firstOKARN, firstOK, nil
	}
	if firstErr != nil {
		kiroLogger(ctx, account).Warn("kiro.profile_usage_probe_all_failed", zap.Error(firstErr))
	}
	return profiles[0], nil, nil
}

func (s *KiroUsageService) ResolveAvailableProfiles(ctx context.Context, account *Account, accessToken string) ([]KiroAvailableProfile, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, fmt.Errorf("access_token is required")
	}
	account = s.prepareAccount(ctx, account)
	seen := map[string]bool{}
	profiles := make([]KiroAvailableProfile, 0, 4)
	for _, region := range kiroProfileDiscoveryRegions(account) {
		out, err := s.fetchAvailableProfilesInRegion(ctx, account, accessToken, region)
		if err != nil {
			continue
		}
		for _, profile := range out.Profiles {
			arn := strings.TrimSpace(firstNonEmptyKiroString(profile.ARN, profile.ProfileARN))
			if arn == "" || seen[arn] {
				continue
			}
			seen[arn] = true
			profiles = append(profiles, profile)
		}
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("kiro available profiles returned no profiles")
	}
	return profiles, nil
}

func (s *KiroUsageService) fetchUsageLimitsInRegion(ctx context.Context, account *Account, accessToken, region string) (*KiroUsageLimits, error) {
	host := fmt.Sprintf("q.%s.amazonaws.com", region)
	params := url.Values{
		"origin":       {"AI_EDITOR"},
		"resourceType": {"AGENTIC_REQUEST"},
	}
	if profileARN := account.GetCredential("profile_arn"); profileARN != "" {
		params.Set("profileArn", profileARN)
	}
	reqURL := fmt.Sprintf("https://%s/getUsageLimits?%s", host, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, buildKiroUsageUpstreamError(resp)
	}
	var out KiroUsageLimits
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *KiroUsageService) fetchAvailableProfilesInRegion(ctx context.Context, account *Account, accessToken, region string) (*KiroAvailableProfiles, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	region = strings.TrimSpace(region)
	if region == "" {
		region = "us-east-1"
	}
	host := fmt.Sprintf("q.%s.amazonaws.com", region)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+host+"/", strings.NewReader(`{"maxResults":10}`))
	if err != nil {
		return nil, err
	}
	runtimeSettings := DefaultKiroRuntimeSettings()
	if s != nil && s.settingService != nil {
		runtimeSettings = s.settingService.GetKiroRuntimeSettings(ctx)
	}
	machineID := kiro.GenerateMachineID(account.GetCredential("machine_id"), account.GetCredential("refresh_token"))
	kiroVersion := runtimeSettings.KiroVersion
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	req.Header.Set("x-amz-target", "AmazonCodeWhispererService.ListAvailableProfiles")
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

	resp, err := doKiroSidecarRequest(req, account, s.httpUpstream, s.tlsFPProfileService, 60*time.Second)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, buildKiroUsageUpstreamError(resp)
	}
	var out KiroAvailableProfiles
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *KiroUsageService) fetchAvailableModelsInRegion(ctx context.Context, account *Account, accessToken, region string) ([]KiroAvailableModel, error) {
	host := fmt.Sprintf("q.%s.amazonaws.com", region)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/listAvailableModels?origin=AI_EDITOR", nil)
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
	resp, err := doKiroSidecarRequest(req, account, s.httpUpstream, s.tlsFPProfileService, 60*time.Second)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, buildKiroUsageUpstreamError(resp)
	}
	var out struct {
		Models []KiroAvailableModel `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

func kiroProfileDiscoveryRegions(account *Account) []string {
	out := []string{}
	add := func(region string) {
		region = strings.TrimSpace(region)
		if region == "" {
			return
		}
		for _, existing := range out {
			if existing == region {
				return
			}
		}
		out = append(out, region)
	}
	add(profileARNRegion(accountCredential(account, "profile_arn")))
	add(accountCredential(account, "api_region"))
	add(accountCredential(account, "auth_region"))
	add(accountCredential(account, "region"))
	add("us-east-1")
	add("eu-central-1")
	return out
}

func kiroRESTRegions(account *Account) []string {
	out := []string{}
	add := func(region string) {
		region = strings.TrimSpace(region)
		if region == "" {
			return
		}
		for _, existing := range out {
			if existing == region {
				return
			}
		}
		out = append(out, region)
	}
	add(profileARNRegion(accountCredential(account, "profile_arn")))
	add(accountCredential(account, "api_region"))
	add(accountCredential(account, "auth_region"))
	add(accountCredential(account, "region"))
	add("us-east-1")
	add("eu-central-1")
	return out
}

func cloneAccountForKiroProfile(account *Account, profileARN string) *Account {
	if account == nil {
		return nil
	}
	cloned := *account
	cloned.Credentials = cloneCredentials(account.Credentials)
	if cloned.Credentials == nil {
		cloned.Credentials = map[string]any{}
	}
	profileARN = strings.TrimSpace(profileARN)
	if profileARN != "" {
		cloned.Credentials["profile_arn"] = profileARN
		if profileID := profileARNProfileID(profileARN); profileID != "" {
			cloned.Credentials["profile_id"] = profileID
		}
		if region := profileARNRegion(profileARN); region != "" {
			cloned.Credentials["api_region"] = region
			cloned.Credentials["region"] = region
		}
	}
	return &cloned
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

func shouldStopKiroUsageRegionFallback(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "kiro usage upstream returned 401") ||
		strings.Contains(errStr, "invalid token") ||
		strings.Contains(errStr, "invalid_token") ||
		strings.Contains(errStr, "expired token") ||
		strings.Contains(errStr, "token expired") ||
		strings.Contains(errStr, "unauthorized")
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
	return strings.TrimSpace(*k.SubscriptionInfo.SubscriptionTitle)
}

func (k *KiroUsageLimits) OverageEnabled() *bool {
	if k == nil || k.OverageConfiguration == nil {
		return nil
	}
	return k.OverageConfiguration.Enabled
}

func (k *KiroUsageLimits) OverageCapability() string {
	if k == nil || k.SubscriptionInfo == nil || k.SubscriptionInfo.OverageCapability == nil {
		return ""
	}
	return strings.TrimSpace(*k.SubscriptionInfo.OverageCapability)
}

func (k *KiroUsageLimits) Email() string {
	if k == nil || k.UserInfo == nil || k.UserInfo.Email == nil {
		return ""
	}
	return strings.TrimSpace(*k.UserInfo.Email)
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
