package service

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func requiresOAuthOnlyAccount(platform string) bool {
	switch platform {
	case PlatformOpenAI, PlatformAntigravity, PlatformAnthropic, PlatformGemini, PlatformKiro, PlatformGrok:
		return true
	default:
		return false
	}
}

func validateAccountPlatform(platform string) error {
	switch strings.TrimSpace(platform) {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformKiro, PlatformGrok:
		return nil
	default:
		return infraerrors.BadRequest("UNSUPPORTED_PLATFORM", fmt.Sprintf("unsupported platform: %s", platform))
	}
}

func validatePlatformAccountType(platform, accountType string) error {
	if err := validateAccountPlatform(platform); err != nil {
		return err
	}
	if platform == PlatformKiro && accountType != AccountTypeOAuth && accountType != AccountTypeAPIKey {
		return infraerrors.BadRequest("UNSUPPORTED_ACCOUNT_TYPE", "kiro accounts only support oauth or apikey type")
	}
	if platform == PlatformGrok && accountType != AccountTypeOAuth && accountType != AccountTypeAPIKey {
		return infraerrors.BadRequest("UNSUPPORTED_ACCOUNT_TYPE", "grok accounts only support oauth or apikey type")
	}
	return nil
}

func validateKiroCredentials(credentials map[string]any) error {
	credentials = NormalizeKiroOAuthCredentialShape(credentials)
	refreshToken := strings.TrimSpace(stringCredential(credentials, "refresh_token"))
	if err := ValidateKiroRefreshTokenHealth(refreshToken); err != nil {
		return err
	}
	authMethod := NormalizeKiroAuthMethod(credentials)
	if KiroAuthMethodUsesIDCRefresh(authMethod) {
		if strings.TrimSpace(stringCredential(credentials, "client_id")) == "" || strings.TrimSpace(stringCredential(credentials, "client_secret")) == "" {
			return infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro idc client_id and client_secret are required")
		}
	}
	if authMethod == "external_idp" {
		if strings.TrimSpace(stringCredential(credentials, "client_id")) == "" {
			return infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro external_idp client_id is required")
		}
		if strings.TrimSpace(resolveKiroExternalIDPTokenEndpoint(credentials)) == "" {
			return infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro external_idp token_endpoint or Microsoft issuer_url is required")
		}
	}
	if rawExpiresAt, ok := credentials["expires_at"]; ok {
		if _, err := parseKiroExpiresAt(rawExpiresAt); err != nil {
			return infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro expires_at is invalid")
		}
	}
	return nil
}

const kiroRefreshTokenMinLength = 10

func ValidateKiroRefreshTokenHealth(refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	switch {
	case refreshToken == "":
		return infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro refresh_token is required")
	case len(refreshToken) < kiroRefreshTokenMinLength:
		return infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro refresh_token is too short")
	case looksLikeTruncatedKiroRefreshToken(refreshToken):
		return infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro refresh_token appears truncated")
	default:
		return nil
	}
}

func looksLikeTruncatedKiroRefreshToken(refreshToken string) bool {
	token := strings.TrimSpace(strings.ToLower(refreshToken))
	return strings.HasSuffix(token, "...") ||
		strings.HasSuffix(token, "…") ||
		strings.Contains(token, "(truncated)") ||
		strings.Contains(token, "[truncated]")
}

func validateCredentialsBaseURL(platform, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s base_url is invalid", platform)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s base_url is invalid", platform)
	}
	return nil
}

func validateAPIKeyCredentials(account *Account) error {
	if account == nil {
		return errors.New("account is required")
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return fmt.Errorf("%s api_key is required", account.Platform)
	}
	return validateCredentialsBaseURL(account.Platform, account.GetCredential("base_url"))
}

func validateOAuthCredentials(account *Account) error {
	if account == nil {
		return errors.New("account is required")
	}
	if strings.TrimSpace(account.GetCredential("access_token")) == "" && strings.TrimSpace(account.GetCredential("refresh_token")) == "" {
		return fmt.Errorf("%s access_token or refresh_token is required", account.Platform)
	}
	return nil
}

func validateAccountTestCredentials(account *Account) error {
	if account == nil {
		return errors.New("account is required")
	}
	switch account.Type {
	case AccountTypeAPIKey:
		return validateAPIKeyCredentials(account)
	case AccountTypeOAuth, AccountTypeSetupToken:
		return validateOAuthCredentials(account)
	default:
		return fmt.Errorf("unsupported account type for %s: %s", account.Platform, account.Type)
	}
}

func validateKiroAPIKeyCredentials(credentials map[string]any) error {
	apiKey := strings.TrimSpace(stringCredential(credentials, "api_key"))
	if apiKey == "" {
		return infraerrors.BadRequest("INVALID_KIRO_CREDENTIALS", "kiro api_key is required")
	}
	return nil
}

func validateKiroAccountCredentials(accountType string, credentials map[string]any) error {
	if accountType == AccountTypeOAuth {
		credentials = NormalizeKiroOAuthCredentialShape(credentials)
	}
	switch accountType {
	case AccountTypeOAuth:
		return validateKiroCredentials(credentials)
	case AccountTypeAPIKey:
		return validateKiroAPIKeyCredentials(credentials)
	default:
		return nil
	}
}

var (
	kiroOAuthCredentialKeysToDropForTypeSwitch = map[string]struct{}{
		"access_token":  {},
		"refresh_token": {},
		"expires_at":    {},
		"client_id":     {},
		"client_secret": {},
		"auth_method":   {},
	}
	kiroAPIKeyCredentialKeysToDropForTypeSwitch = map[string]struct{}{
		"api_key": {},
	}
)

func mergeKiroCredentialsForAccountUpdate(accountType string, existing, incoming map[string]any, allowSensitiveCredentials bool) map[string]any {
	merged := cloneCredentials(existing)
	dropStaleKiroCredentialsForType(merged, accountType, incoming, allowSensitiveCredentials)
	for key, value := range incoming {
		if shouldIgnoreKiroCredentialPatch(accountType, key, allowSensitiveCredentials) {
			continue
		}
		if shouldDeleteKiroCredentialOnUpdate(key, value, allowSensitiveCredentials) {
			delete(merged, key)
			continue
		}
		merged[key] = value
	}
	return merged
}

func shouldIgnoreKiroCredentialPatch(accountType, key string, allowSensitiveCredentials bool) bool {
	if allowSensitiveCredentials {
		return false
	}
	if accountType != AccountTypeOAuth {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "access_token", "refresh_token", "expires_at", "client_id", "client_secret", "auth_method":
		return true
	default:
		return false
	}
}

func mergeAccountCredentialsForAccountUpdate(platform, accountType string, existing, incoming map[string]any, allowSensitiveCredentials bool) map[string]any {
	if platform == PlatformKiro {
		return mergeKiroCredentialsForAccountUpdate(accountType, existing, incoming, allowSensitiveCredentials)
	}

	merged := cloneCredentials(existing)
	if allowSensitiveCredentials {
		for key := range merged {
			if isSensitiveCredentialKey(key) {
				if _, exists := incoming[key]; !exists {
					delete(merged, key)
				}
			}
		}
	}
	for key, value := range incoming {
		if allowSensitiveCredentials && isSensitiveCredentialKey(key) {
			if credentialPatchValueIsEmpty(value) {
				delete(merged, key)
				continue
			}
			merged[key] = value
			continue
		}
		if shouldIgnoreSensitiveCredentialPatch(key, value) {
			continue
		}
		if shouldDeleteCredentialOnUpdate(key, value) {
			delete(merged, key)
			continue
		}
		merged[key] = value
	}
	return merged
}

func shouldIgnoreSensitiveCredentialPatch(key string, value any) bool {
	if !isSensitiveCredentialKey(key) {
		return false
	}
	return credentialPatchValueIsEmpty(value)
}

func shouldDeleteCredentialOnUpdate(key string, value any) bool {
	if isSensitiveCredentialKey(key) {
		return false
	}
	return value == nil
}

func isSensitiveCredentialKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if IsSensitiveCredentialKey(normalized) {
		return true
	}
	switch normalized {
	case "client_id", "session_token", "password":
		return true
	default:
		return false
	}
}

func credentialPatchValueIsEmpty(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func dropStaleKiroCredentialsForType(credentials map[string]any, accountType string, incoming map[string]any, allowSensitiveCredentials bool) {
	switch accountType {
	case AccountTypeOAuth:
		if _, hasOAuthSecret := incoming["refresh_token"]; hasOAuthSecret {
			for key := range kiroAPIKeyCredentialKeysToDropForTypeSwitch {
				delete(credentials, key)
			}
		}
		if allowSensitiveCredentials {
			if method, ok := incoming["auth_method"].(string); ok {
				switch normalizedMethod := NormalizeKiroAuthMethod(map[string]any{"auth_method": method}); {
				case KiroAuthMethodUsesIDCRefresh(normalizedMethod):
				case normalizedMethod == "external_idp":
					for _, key := range []string{"client_secret", "idc_region"} {
						delete(credentials, key)
					}
				default:
					for _, key := range []string{"client_id", "client_secret", "issuer_url", "idc_region", "scopes", "login_hint", "token_endpoint"} {
						delete(credentials, key)
					}
				}
			}
		}
	case AccountTypeAPIKey:
		if _, hasAPIKey := incoming["api_key"]; hasAPIKey {
			for key := range kiroOAuthCredentialKeysToDropForTypeSwitch {
				delete(credentials, key)
			}
		}
	}
}

func shouldDeleteKiroCredentialOnUpdate(key string, value any, allowSensitiveCredentials bool) bool {
	if allowSensitiveCredentials && value == nil {
		return true
	}
	switch key {
	case "expires_at":
		if value == nil {
			return true
		}
		typed, ok := value.(string)
		return ok && strings.TrimSpace(typed) == ""
	case "model_whitelist":
		return value == nil
	default:
		return shouldDeleteCredentialOnUpdate(key, value)
	}
}

// KiroAuthMethodUsesIDCRefresh mirrors KiroTokenRefresher.Refresh's IDC/OIDC
// token refresh branch.
func KiroAuthMethodUsesIDCRefresh(authMethod string) bool {
	return normalizeKiroAuthMethodValue(authMethod) == "idc"
}

func NormalizeKiroAuthMethod(credentials map[string]any) string {
	authMethod := normalizeKiroAuthMethodValue(stringCredential(credentials, "auth_method"))
	if authMethod != "" {
		return authMethod
	}
	if strings.TrimSpace(stringCredential(credentials, "client_id")) != "" && strings.TrimSpace(stringCredential(credentials, "client_secret")) != "" {
		return "idc"
	}
	for _, key := range []string{"provider", "login_provider", "login_option"} {
		if authMethod := normalizeKiroAuthMethodValue(stringCredential(credentials, key)); authMethod != "" {
			return authMethod
		}
	}
	return "social"
}

func normalizeKiroAuthMethodValue(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	switch normalized {
	case "":
		return ""
	case "external-idp", "externalidp":
		return "external_idp"
	case "idc", "builderid", "builder-id", "awsidc", "aws-idc", "iam", "internal", "enterprise":
		return "idc"
	default:
		return "social"
	}
}

func NormalizeKiroOAuthCredentialShape(credentials map[string]any) map[string]any {
	if credentials == nil {
		return nil
	}
	normalized := cloneCredentials(credentials)
	copyKiroCredentialStringIfEmpty(normalized, normalized, "access_token", "accessToken", "access_token")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "refresh_token", "refreshToken", "refresh_token")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "token_type", "tokenType", "token_type")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "expires_at", "expiresAt", "expires_at")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "auth_method", "authMethod", "auth_method")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "client_id", "clientId", "client_id")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "client_secret", "clientSecret", "client_secret")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "issuer_url", "issuerUrl", "issuer_url")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "scopes", "scopes", "scope")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "token_endpoint", "tokenEndpoint", "token_endpoint")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "login_provider", "provider", "loginProvider", "login_provider")
	copyKiroCredentialStringIfEmpty(normalized, normalized, "login_hint", "loginHint", "login_hint")

	if rawToken, ok := kiroCredentialNestedMap(normalized, "kiro_auth_token_raw"); ok {
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "access_token", "accessToken", "access_token")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "refresh_token", "refreshToken", "refresh_token")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "token_type", "tokenType", "token_type")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "expires_at", "expiresAt", "expires_at")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "auth_method", "authMethod", "auth_method")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "client_id", "clientId", "client_id")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "client_secret", "clientSecret", "client_secret")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "issuer_url", "issuerUrl", "issuer_url")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "scopes", "scopes", "scope")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "token_endpoint", "tokenEndpoint", "token_endpoint")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "login_provider", "provider", "loginProvider", "login_provider")
		copyKiroCredentialStringIfEmpty(normalized, rawToken, "login_hint", "loginHint", "login_hint")
	}
	if rawProfile, ok := kiroCredentialNestedMap(normalized, "kiro_profile_raw"); ok {
		copyKiroCredentialStringIfEmpty(normalized, rawProfile, "profile_arn", "profileArn", "profile_arn", "arn")
		copyKiroCredentialStringIfEmpty(normalized, rawProfile, "profile_name", "name")
	}
	copyKiroCredentialStringIfEmpty(normalized, normalized, "profile_arn", "profileArn", "profile_arn", "arn")

	if NormalizeKiroAuthMethod(normalized) == "external_idp" {
		normalized["auth_method"] = "external_idp"
		delete(normalized, "tokenEndpoint")
	}
	if endpoint := resolveKiroExternalIDPTokenEndpoint(normalized); endpoint != "" {
		normalized["token_endpoint"] = endpoint
	} else if NormalizeKiroAuthMethod(normalized) == "external_idp" {
		delete(normalized, "token_endpoint")
	}
	if profileARN := strings.TrimSpace(stringCredential(normalized, "profile_arn")); profileARN != "" && strings.TrimSpace(stringCredential(normalized, "profile_id")) == "" {
		normalized["profile_id"] = profileARNProfileID(profileARN)
	}
	return normalized
}

func kiroCredentialNestedMap(credentials map[string]any, key string) (map[string]any, bool) {
	raw, ok := credentials[key]
	if !ok || raw == nil {
		return nil, false
	}
	if typed, ok := raw.(map[string]any); ok {
		return typed, true
	}
	if typed, ok := raw.(map[string]any); ok {
		return map[string]any(typed), true
	}
	return nil, false
}

func copyKiroCredentialStringIfEmpty(dst, src map[string]any, target string, keys ...string) {
	if strings.TrimSpace(stringCredential(dst, target)) != "" {
		return
	}
	for _, key := range keys {
		if value := strings.TrimSpace(stringCredential(src, key)); value != "" {
			dst[target] = value
			return
		}
	}
}

func resolveKiroExternalIDPTokenEndpoint(credentials map[string]any) string {
	if credentials == nil {
		return ""
	}
	if endpoint := strings.TrimSpace(stringCredential(credentials, "token_endpoint")); endpoint != "" {
		return normalizeKiroMicrosoftTokenEndpoint(endpoint)
	}
	if endpoint := strings.TrimSpace(stringCredential(credentials, "tokenEndpoint")); endpoint != "" {
		return normalizeKiroMicrosoftTokenEndpoint(endpoint)
	}
	issuerURL := strings.TrimSpace(stringCredential(credentials, "issuer_url"))
	if issuerURL == "" {
		issuerURL = strings.TrimSpace(stringCredential(credentials, "issuerUrl"))
	}
	return deriveMicrosoftTokenEndpointFromIssuer(issuerURL)
}

func deriveMicrosoftTokenEndpointFromIssuer(issuerURL string) string {
	issuerURL = strings.TrimSpace(issuerURL)
	if issuerURL == "" {
		return ""
	}
	parsed, err := url.Parse(issuerURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	if strings.ToLower(parsed.Scheme) != "https" || parsed.Port() != "" || strings.ToLower(parsed.Hostname()) != "login.microsoftonline.com" {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return ""
	}
	tenant := parts[0]
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenant)
}

func normalizeKiroMicrosoftTokenEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	if strings.ToLower(parsed.Scheme) != "https" || parsed.Port() != "" || strings.ToLower(parsed.Hostname()) != "login.microsoftonline.com" {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || strings.TrimSpace(parts[0]) == "" || parts[1] != "oauth2" || parts[2] != "v2.0" || parts[3] != "token" {
		return ""
	}
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", parts[0])
}

func stringCredential(credentials map[string]any, key string) string {
	if credentials == nil {
		return ""
	}
	value, ok := credentials[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		return strings.Join(typed, " ")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			part := strings.TrimSpace(fmt.Sprint(item))
			if part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, " ")
	}
	return fmt.Sprintf("%v", value)
}

func parseKiroExpiresAt(value any) (time.Time, error) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return time.Time{}, fmt.Errorf("empty expires_at")
		}
		if ts, err := time.Parse(time.RFC3339, trimmed); err == nil {
			return ts, nil
		}
		seconds, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
		return time.Unix(seconds, 0).UTC(), nil
	case int64:
		return time.Unix(typed, 0).UTC(), nil
	case int:
		return time.Unix(int64(typed), 0).UTC(), nil
	case float64:
		return time.Unix(int64(typed), 0).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported expires_at type")
	}
}

