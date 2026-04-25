package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	var (
		accessToken  string
		refreshToken string
		expiresAt    string
		profileARN   string
		err          error
	)

	switch authMethod := NormalizeKiroAuthMethod(account.Credentials); {
	case KiroAuthMethodUsesIDCRefresh(authMethod):
		accessToken, refreshToken, expiresAt, err = r.refreshKiroIDCToken(ctx, account)
	default:
		accessToken, refreshToken, expiresAt, profileARN, err = r.refreshKiroSocialToken(ctx, account)
	}
	if err != nil {
		return nil, err
	}

	newCreds := map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_at":    expiresAt,
	}
	if profileARN != "" {
		newCreds["profile_arn"] = profileARN
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

func (r *KiroTokenRefresher) refreshKiroSocialToken(ctx context.Context, account *Account) (accessToken, refreshToken, expiresAt, profileARN string, err error) {
	payload := map[string]any{
		"refreshToken": account.GetCredential("refresh_token"),
	}
	url := fmt.Sprintf("https://prod.%s.auth.desktop.kiro.dev/refreshToken", KiroAuthRegion(account))
	host := fmt.Sprintf("prod.%s.auth.desktop.kiro.dev", KiroAuthRegion(account))
	var out kiroRefreshResponse
	if err = r.doKiroJSONRequest(ctx, account, url, host, payload, &out); err != nil {
		return "", "", "", "", err
	}
	refreshToken = out.RefreshToken
	if refreshToken == "" {
		refreshToken = account.GetCredential("refresh_token")
	}
	expiresAt = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	return out.AccessToken, refreshToken, expiresAt, out.ProfileARN, nil
}

func (r *KiroTokenRefresher) refreshKiroIDCToken(ctx context.Context, account *Account) (accessToken, refreshToken, expiresAt string, err error) {
	payload := url.Values{
		"client_id":     []string{account.GetCredential("client_id")},
		"client_secret": []string{account.GetCredential("client_secret")},
		"refresh_token": []string{account.GetCredential("refresh_token")},
		"grant_type":    []string{"refresh_token"},
	}
	url := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", KiroAuthRegion(account))
	host := fmt.Sprintf("oidc.%s.amazonaws.com", KiroAuthRegion(account))
	var out kiroRefreshResponse
	if err = r.doKiroFormRequest(ctx, account, url, host, payload, &out); err != nil {
		return "", "", "", fmt.Errorf("kiro idc refresh failed: %w", err)
	}
	refreshToken = out.RefreshToken
	if refreshToken == "" {
		refreshToken = account.GetCredential("refresh_token")
	}
	expiresAt = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	return out.AccessToken, refreshToken, expiresAt, nil
}

type kiroRefreshResponse struct {
	AccessToken  string
	RefreshToken string
	ProfileARN   string
	ExpiresIn    int64
}

func (r *KiroTokenRefresher) doKiroJSONRequest(ctx context.Context, account *Account, url, host string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return r.doKiroRequest(req, account, host, out)
}

func (r *KiroTokenRefresher) doKiroFormRequest(ctx context.Context, account *Account, url, host string, payload url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(payload.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r.doKiroRequest(req, account, host, out)
}

func (r *KiroTokenRefresher) doKiroRequest(req *http.Request, account *Account, host string, out any) error {
	runtimeSettings := DefaultKiroRuntimeSettings()
	if r != nil && r.settingService != nil {
		runtimeSettings = r.settingService.GetKiroRuntimeSettings(req.Context())
	}
	runtimeSettings = normalizeKiroRuntimeSettings(runtimeSettings)
	machineID := kiropkg.GenerateMachineID(account.GetCredential("machine_id"), "", account.GetCredential("refresh_token"))
	kiroVersion := runtimeSettings.KiroVersion
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("host", host)
	req.Header.Set("User-Agent", fmt.Sprintf("KiroIDE-%s-%s", kiroVersion, machineID))
	if runtimeSettings.KiroCommit != "" {
		req.Header.Set("x-amzn-kiro-commit", runtimeSettings.KiroCommit)
	}
	if req.URL != nil && strings.Contains(req.URL.Host, "oidc.") {
		req.Header.Set("x-amz-user-agent", fmt.Sprintf("aws-sdk-js/3.738.0 KiroIDE-%s-%s", kiroVersion, machineID))
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "*")
		req.Header.Set("sec-fetch-mode", "cors")
		req.Header.Set("User-Agent", fmt.Sprintf("aws-sdk-js/3.738.0 ua/2.1 os/%s lang/js md/nodejs#%s api/sso-oidc#3.738.0 m/E KiroIDE-%s-%s", runtimeSettings.SystemVersion, runtimeSettings.NodeVersion, kiroVersion, machineID))
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
