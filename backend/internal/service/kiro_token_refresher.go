package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
)

const kiroRefreshTokenInvalidReason = "KIRO_REFRESH_TOKEN_INVALID"

type KiroTokenRefresher struct {
	httpUpstream        HTTPUpstream
	tlsFPProfileService *TLSFingerprintProfileService
	settingService      *SettingService
	proxyRepo           ProxyRepository
}

func NewKiroTokenRefresher() *KiroTokenRefresher {
	return &KiroTokenRefresher{}
}

func (r *KiroTokenRefresher) WithTransport(httpUpstream HTTPUpstream, tlsFPProfileService *TLSFingerprintProfileService) *KiroTokenRefresher {
	if r == nil {
		return nil
	}
	r.httpUpstream = httpUpstream
	r.tlsFPProfileService = tlsFPProfileService
	return r
}

func (r *KiroTokenRefresher) WithSettingService(settingService *SettingService) *KiroTokenRefresher {
	if r == nil {
		return nil
	}
	r.settingService = settingService
	return r
}

func (r *KiroTokenRefresher) WithProxyRepo(proxyRepo ProxyRepository) *KiroTokenRefresher {
	if r == nil {
		return nil
	}
	r.proxyRepo = proxyRepo
	return r
}

func (r *KiroTokenRefresher) UsageService() *KiroUsageService {
	return NewKiroUsageService().
		WithTransport(r.httpUpstream, r.tlsFPProfileService).
		WithSettingService(r.settingService).
		WithProxyRepo(r.proxyRepo)
}

func (r *KiroTokenRefresher) CacheKey(account *Account) string {
	return KiroTokenCacheKey(account)
}

func (r *KiroTokenRefresher) CanRefresh(account *Account) bool {
	return account != nil && account.Platform == PlatformKiro && account.Type == AccountTypeOAuth
}

func (r *KiroTokenRefresher) NeedsRefresh(account *Account, refreshWindow time.Duration) bool {
	expiresAt := account.GetCredentialAsTime("expires_at")
	if expiresAt == nil {
		return true
	}
	return time.Until(*expiresAt) < refreshWindow
}

func (r *KiroTokenRefresher) Refresh(ctx context.Context, account *Account) (map[string]any, error) {
	account = r.prepareAccount(ctx, account)
	runtimeSettings := DefaultKiroRuntimeSettings()
	if r != nil && r.settingService != nil {
		runtimeSettings = r.settingService.GetKiroRuntimeSettings(ctx)
	}
	runtimeSettings, machineID, err := applyKiroSidecarIdentity(ctx, account, runtimeSettings)
	if err != nil {
		return nil, err
	}
	ctx = withKiroSidecarIdentity(ctx, runtimeSettings, machineID)
	var (
		accessToken  string
		refreshToken string
		expiresAt    string
		profileARN   string
	)

	switch authMethod := NormalizeKiroAuthMethod(account.Credentials); {
	case KiroAuthMethodUsesIDCRefresh(authMethod):
		accessToken, refreshToken, expiresAt, err = r.refreshKiroIDCToken(ctx, account, machineID)
	case authMethod == "external_idp":
		accessToken, refreshToken, expiresAt, err = r.refreshKiroExternalIDPToken(ctx, account)
	default:
		accessToken, refreshToken, expiresAt, profileARN, err = r.refreshKiroSocialToken(ctx, account, machineID)
	}
	if err != nil {
		return nil, err
	}

	newCreds := map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_at":    expiresAt,
	}
	if strings.TrimSpace(account.GetCredential("machine_id")) == "" && machineID != "" {
		newCreds["machine_id"] = machineID
	}
	if profileARN != "" {
		newCreds["profile_arn"] = profileARN
	}
	if NormalizeKiroAuthMethod(account.Credentials) == "external_idp" {
		newCreds["auth_method"] = "external_idp"
		if endpoint := resolveKiroExternalIDPTokenEndpoint(account.Credentials); endpoint != "" {
			newCreds["token_endpoint"] = endpoint
		}
	}
	return MergeCredentials(account.Credentials, newCreds), nil
}

func (r *KiroTokenRefresher) prepareAccount(ctx context.Context, account *Account) *Account {
	if account == nil || account.Proxy != nil || account.ProxyID == nil || r == nil || r.proxyRepo == nil {
		return account
	}
	if ctx == nil {
		ctx = context.Background()
	}
	proxy, err := r.proxyRepo.GetByID(ctx, *account.ProxyID)
	if err != nil || proxy == nil {
		return account
	}
	cloned := *account
	cloned.Proxy = proxy
	return &cloned
}

func (r *KiroTokenRefresher) refreshKiroSocialToken(ctx context.Context, account *Account, machineID string) (accessToken, refreshToken, expiresAt, profileARN string, err error) {
	refreshToken = strings.TrimSpace(account.GetCredential("refresh_token"))
	if err = ValidateKiroRefreshTokenHealth(refreshToken); err != nil {
		return "", "", "", "", err
	}
	payload := map[string]any{
		"refreshToken": refreshToken,
	}
	url := fmt.Sprintf("https://prod.%s.auth.desktop.kiro.dev/refreshToken", KiroAuthRegion(account))
	host := fmt.Sprintf("prod.%s.auth.desktop.kiro.dev", KiroAuthRegion(account))
	var out kiroRefreshResponse
	if err = r.doKiroJSONRequest(ctx, account, url, host, payload, &out, machineID); err != nil {
		return "", "", "", "", err
	}
	if err = validateKiroRefreshResponse(out); err != nil {
		return "", "", "", "", err
	}
	if ValidateKiroRefreshTokenHealth(out.RefreshToken) == nil {
		refreshToken = out.RefreshToken
	}
	expiresIn := defaultKiroRefreshExpiresIn(out.ExpiresIn, "social")
	expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339)
	return out.AccessToken, refreshToken, expiresAt, out.ProfileARN, nil
}

func (r *KiroTokenRefresher) refreshKiroIDCToken(ctx context.Context, account *Account, machineID string) (accessToken, refreshToken, expiresAt string, err error) {
	refreshToken = strings.TrimSpace(account.GetCredential("refresh_token"))
	if err = ValidateKiroRefreshTokenHealth(refreshToken); err != nil {
		return "", "", "", err
	}
	payload := map[string]any{
		"clientId":     account.GetCredential("client_id"),
		"clientSecret": account.GetCredential("client_secret"),
		"refreshToken": refreshToken,
		"grantType":    "refresh_token",
	}
	url := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", KiroAuthRegion(account))
	host := fmt.Sprintf("oidc.%s.amazonaws.com", KiroAuthRegion(account))
	var out kiroRefreshResponse
	if err = r.doKiroJSONRequest(ctx, account, url, host, payload, &out, machineID); err != nil {
		return "", "", "", fmt.Errorf("kiro idc refresh failed: %w", err)
	}
	if err = validateKiroRefreshResponse(out); err != nil {
		return "", "", "", err
	}
	if ValidateKiroRefreshTokenHealth(out.RefreshToken) == nil {
		refreshToken = out.RefreshToken
	}
	expiresIn := defaultKiroRefreshExpiresIn(out.ExpiresIn, "idc")
	expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339)
	return out.AccessToken, refreshToken, expiresAt, nil
}

func (r *KiroTokenRefresher) refreshKiroExternalIDPToken(ctx context.Context, account *Account) (accessToken, refreshToken, expiresAt string, err error) {
	refreshToken = strings.TrimSpace(account.GetCredential("refresh_token"))
	if err = ValidateKiroRefreshTokenHealth(refreshToken); err != nil {
		return "", "", "", err
	}
	clientID := strings.TrimSpace(account.GetCredential("client_id"))
	if clientID == "" {
		return "", "", "", infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro external_idp client_id is required")
	}
	tokenEndpoint := resolveKiroExternalIDPTokenEndpoint(account.Credentials)
	if tokenEndpoint == "" {
		return "", "", "", infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro external_idp token_endpoint or Microsoft issuer_url is required")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", clientID)
	form.Set("refresh_token", refreshToken)
	if scopes := strings.TrimSpace(stringCredential(account.Credentials, "scopes")); scopes != "" {
		form.Set("scope", scopes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := doKiroSidecarRequest(req, account, r.httpUpstream, r.tlsFPProfileService, 60*time.Second)
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", "", "", readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", "", buildKiroRefreshUpstreamError(resp.StatusCode, body)
	}

	var out kiroRefreshResponse
	if err := decodeKiroRefreshResponse(body, &out); err != nil {
		return "", "", "", err
	}
	if err := validateKiroRefreshResponse(out); err != nil {
		return "", "", "", err
	}
	if ValidateKiroRefreshTokenHealth(out.RefreshToken) == nil {
		refreshToken = out.RefreshToken
	}
	expiresIn := defaultKiroRefreshExpiresIn(out.ExpiresIn, "external_idp")
	expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format(time.RFC3339)
	return out.AccessToken, refreshToken, expiresAt, nil
}

func defaultKiroRefreshExpiresIn(expiresIn int64, authMethod string) int64 {
	if expiresIn > 0 {
		return expiresIn
	}
	slog.Warn("kiro_refresh_missing_expires_in_defaulted",
		"auth_method", authMethod,
		"expires_in", expiresIn,
		"default_expires_in", int64(3600),
	)
	return 3600
}

type kiroRefreshResponse struct {
	AccessToken  string
	RefreshToken string
	ProfileARN   string
	ExpiresIn    int64
}

func validateKiroRefreshResponse(out kiroRefreshResponse) error {
	if strings.TrimSpace(out.AccessToken) == "" {
		return infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro refresh response missing access_token")
	}
	return nil
}

func (r *KiroTokenRefresher) doKiroJSONRequest(ctx context.Context, account *Account, url, host string, payload any, out any, machineID string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return r.doKiroRequest(req, account, host, out, machineID)
}

func (r *KiroTokenRefresher) doKiroRequest(req *http.Request, account *Account, host string, out any, machineID string) error {
	runtimeSettings := DefaultKiroRuntimeSettings()
	if r != nil && r.settingService != nil {
		runtimeSettings = r.settingService.GetKiroRuntimeSettings(req.Context())
	}
	runtimeSettings, profileMachineID, err := applyKiroSidecarIdentity(req.Context(), account, runtimeSettings)
	if err != nil {
		return err
	}
	if strings.TrimSpace(machineID) == "" {
		machineID = profileMachineID
	}
	kiroVersion := runtimeSettings.KiroVersion
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("host", host)
	req.Header.Set("User-Agent", fmt.Sprintf("KiroIDE-%s-%s", kiroVersion, machineID))
	if runtimeSettings.KiroCommit != "" {
		req.Header.Set("x-amzn-kiro-commit", runtimeSettings.KiroCommit)
	}
	if req.URL != nil && strings.Contains(req.URL.Host, "oidc.") {
		xAmzUserAgent, userAgent := kiropkg.BuildSSOOIDCUserAgents(kiroVersion, machineID, runtimeSettings.SystemVersion, runtimeSettings.NodeVersion)
		req.Header.Set("x-amz-user-agent", xAmzUserAgent)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "*")
		req.Header.Set("sec-fetch-mode", "cors")
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := doKiroSidecarRequest(req, account, r.httpUpstream, r.tlsFPProfileService, 60*time.Second)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("kiro oauth refresh upstream returned %d", resp.StatusCode)
		}
		return buildKiroRefreshUpstreamError(resp.StatusCode, body)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := decodeKiroRefreshResponse(body, out); err != nil {
		return err
	}
	return nil
}

func decodeKiroRefreshResponse(body []byte, out any) error {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return err
	}
	if nested, ok := payload["data"].(map[string]any); ok && len(nested) > 0 {
		payload = nested
	}
	switch typed := out.(type) {
	case *kiroRefreshResponse:
		typed.AccessToken = firstNonEmptyStringValue(payload, "accessToken", "access_token")
		typed.RefreshToken = firstNonEmptyStringValue(payload, "refreshToken", "refresh_token")
		typed.ProfileARN = firstNonEmptyStringValue(payload, "profileArn", "profile_arn", "arn")
		typed.ExpiresIn = firstNonZeroInt64Value(payload, "expiresIn", "expires_in")
		return nil
	default:
		return json.Unmarshal(body, out)
	}
}

func buildKiroRefreshUpstreamError(statusCode int, body []byte) error {
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("kiro oauth refresh upstream returned %d", statusCode)
	}
	errorCode := ""
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		errorCode = firstNonEmptyStringValue(payload, "error")
		parts := []string{
			errorCode,
			firstNonEmptyStringValue(payload, "error_description", "errorDescription", "message"),
		}
		joined := strings.TrimSpace(strings.Join(filterEmptyStrings(parts), ": "))
		if joined != "" {
			detail = joined
		}
	}
	if statusCode == http.StatusBadRequest && strings.EqualFold(strings.TrimSpace(errorCode), "invalid_grant") {
		return infraerrors.Unauthorized(kiroRefreshTokenInvalidReason, fmt.Sprintf("kiro refresh token is invalid: %s", detail)).
			WithMetadata(map[string]string{
				"upstream_status": fmt.Sprintf("%d", statusCode),
				"upstream_error":  "invalid_grant",
			})
	}
	return fmt.Errorf("kiro oauth refresh upstream returned %d: %s", statusCode, detail)
}

func firstNonEmptyStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func firstNonZeroInt64Value(values map[string]any, keys ...string) int64 {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			if typed != 0 {
				return int64(typed)
			}
		case int64:
			if typed != 0 {
				return typed
			}
		case int:
			if typed != 0 {
				return int64(typed)
			}
		case json.Number:
			if parsed, err := typed.Int64(); err == nil && parsed != 0 {
				return parsed
			}
		case string:
			if parsed, err := json.Number(strings.TrimSpace(typed)).Int64(); err == nil && parsed != 0 {
				return parsed
			}
		}
	}
	return 0
}

func filterEmptyStrings(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
