package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
)

type KiroTokenRefresher struct {
	httpUpstream        HTTPUpstream
	tlsFPProfileService *TLSFingerprintProfileService
	settingService      *SettingService
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

func (r *KiroTokenRefresher) refreshKiroSocialToken(ctx context.Context, account *Account) (accessToken, refreshToken, expiresAt, profileARN string, err error) {
	payload := map[string]any{
		"refreshToken": account.GetCredential("refresh_token"),
	}
	url := fmt.Sprintf("https://prod.%s.auth.desktop.kiro.dev/refreshToken", KiroAuthRegion(account))
	host := fmt.Sprintf("prod.%s.auth.desktop.kiro.dev", KiroAuthRegion(account))
	var out struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ProfileARN   string `json:"profileArn"`
		ExpiresIn    int64  `json:"expiresIn"`
	}
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
	payload := map[string]any{
		"clientId":     account.GetCredential("client_id"),
		"clientSecret": account.GetCredential("client_secret"),
		"refreshToken": account.GetCredential("refresh_token"),
		"grantType":    "refresh_token",
	}
	url := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", KiroAuthRegion(account))
	host := fmt.Sprintf("oidc.%s.amazonaws.com", KiroAuthRegion(account))
	var out struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int64  `json:"expiresIn"`
	}
	if err = r.doKiroJSONRequest(ctx, account, url, host, payload, &out); err != nil {
		return "", "", "", err
	}
	refreshToken = out.RefreshToken
	if refreshToken == "" {
		refreshToken = account.GetCredential("refresh_token")
	}
	expiresAt = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	return out.AccessToken, refreshToken, expiresAt, nil
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

	runtimeSettings := DefaultKiroRuntimeSettings()
	if r != nil && r.settingService != nil {
		runtimeSettings = r.settingService.GetKiroRuntimeSettings(ctx)
	}
	runtimeSettings = normalizeKiroRuntimeSettings(runtimeSettings)
	machineID := kiropkg.GenerateMachineID(account.GetCredential("machine_id"), "", account.GetCredential("refresh_token"))
	kiroVersion := runtimeSettings.KiroVersion
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("host", host)
	req.Header.Set("User-Agent", fmt.Sprintf("KiroIDE-%s-%s", kiroVersion, machineID))
	if runtimeSettings.KiroCommit != "" {
		req.Header.Set("x-amzn-kiro-commit", runtimeSettings.KiroCommit)
	}
	if strings.Contains(url, "oidc.") {
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
		return fmt.Errorf("kiro oauth refresh upstream returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
