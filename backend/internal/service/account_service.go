package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrAccountNotFound      = infraerrors.NotFound("ACCOUNT_NOT_FOUND", "account not found")
	ErrAccountNilInput      = infraerrors.BadRequest("ACCOUNT_NIL_INPUT", "account input cannot be nil")
	ErrAccountNotInFallback = infraerrors.BadRequest("ACCOUNT_NOT_IN_FALLBACK", "account is not in proxy fallback state")
)

const AccountListGroupUngrouped int64 = -1
const AccountPrivacyModeUnsetFilter = "__unset__"

type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, id int64) (*Account, error)
	// GetByIDs fetches accounts by IDs in a single query.
	// It should return all accounts found (missing IDs are ignored).
	GetByIDs(ctx context.Context, ids []int64) ([]*Account, error)
	// ExistsByID 检查账号是否存在，仅返回布尔值，用于删除前的轻量级存在性检查
	ExistsByID(ctx context.Context, id int64) (bool, error)
	// GetByCRSAccountID finds an account previously synced from CRS.
	// Returns (nil, nil) if not found.
	GetByCRSAccountID(ctx context.Context, crsAccountID string) (*Account, error)
	// FindByExtraField 根据 extra 字段中的键值对查找账号
	FindByExtraField(ctx context.Context, key string, value any) ([]Account, error)
	// ListCRSAccountIDs returns a map of crs_account_id -> local account ID
	// for all accounts that have been synced from CRS.
	ListCRSAccountIDs(ctx context.Context) (map[string]int64, error)
	Update(ctx context.Context, account *Account) error
	Delete(ctx context.Context, id int64) error

	List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error)
	ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error)
	ListByGroup(ctx context.Context, groupID int64) ([]Account, error)
	ListActive(ctx context.Context) ([]Account, error)
	ListOAuthRefreshCandidates(ctx context.Context) ([]Account, error)
	ListByPlatform(ctx context.Context, platform string) ([]Account, error)

	UpdateLastUsed(ctx context.Context, id int64) error
	BatchUpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error
	SetError(ctx context.Context, id int64, errorMsg string) error
	ClearError(ctx context.Context, id int64) error
	SetSchedulable(ctx context.Context, id int64, schedulable bool) error
	AutoPauseExpiredAccounts(ctx context.Context, now time.Time) (int64, error)
	BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error

	ListSchedulable(ctx context.Context) ([]Account, error)
	ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error)
	ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error)
	ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error)
	ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error)
	ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error)
	ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error)
	ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error)

	SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error
	SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time, reason ...string) error
	SetOverloaded(ctx context.Context, id int64, until time.Time) error
	SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error
	ClearTempUnschedulable(ctx context.Context, id int64) error
	ClearRateLimit(ctx context.Context, id int64) error
	ClearAntigravityQuotaScopes(ctx context.Context, id int64) error
	ClearModelRateLimits(ctx context.Context, id int64) error
	UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error
	// UpdateSessionWindowEnd 仅更新 5h 窗口的结束时间，不动 start / status。
	// 用于 active poll 拿到新 ResetsAt 后回写，避免覆盖请求路径上记录的 status。
	UpdateSessionWindowEnd(ctx context.Context, id int64, end time.Time) error
	UpdateExtra(ctx context.Context, id int64, updates map[string]any) error
	BulkUpdate(ctx context.Context, ids []int64, updates AccountBulkUpdate) (int64, error)
	// IncrementQuotaUsed 原子递增 API Key 账号的配额用量（总/日/周）
	IncrementQuotaUsed(ctx context.Context, id int64, amount float64) error
	// ResetQuotaUsed 重置 API Key 账号所有维度的配额用量为 0
	ResetQuotaUsed(ctx context.Context, id int64) error
	// RevertProxyFallback 将账号的 proxy_id 切回 proxy_fallback_origin_id，并清空 origin 字段。
	// 仅当 proxy_fallback_origin_id IS NOT NULL 时更新，否则视为账号不存在（返回 ErrAccountNotFound）。
	RevertProxyFallback(ctx context.Context, accountID int64) error
}

// AccountBulkUpdate describes the fields that can be updated in a bulk operation.
// Nil pointers mean "do not change".
type AccountBulkUpdate struct {
	Name           *string
	ProxyID        *int64
	Concurrency    *int
	Priority       *int
	RateMultiplier *float64
	LoadFactor     *int
	Status         *string
	Schedulable    *bool
	Credentials    map[string]any
	Extra          map[string]any
}

// CreateAccountRequest 创建账号请求
type CreateAccountRequest struct {
	Name               string         `json:"name"`
	Notes              *string        `json:"notes"`
	Platform           string         `json:"platform"`
	Type               string         `json:"type"`
	Credentials        map[string]any `json:"credentials"`
	Extra              map[string]any `json:"extra"`
	ProxyID            *int64         `json:"proxy_id"`
	Concurrency        int            `json:"concurrency"`
	Priority           int            `json:"priority"`
	GroupIDs           []int64        `json:"group_ids"`
	ExpiresAt          *time.Time     `json:"expires_at"`
	AutoPauseOnExpired *bool          `json:"auto_pause_on_expired"`
}

// UpdateAccountRequest 更新账号请求
type UpdateAccountRequest struct {
	Name               *string         `json:"name"`
	Notes              *string         `json:"notes"`
	Type               *string         `json:"type"`
	Credentials        *map[string]any `json:"credentials"`
	Extra              *map[string]any `json:"extra"`
	ProxyID            *int64          `json:"proxy_id"`
	Concurrency        *int            `json:"concurrency"`
	Priority           *int            `json:"priority"`
	Status             *string         `json:"status"`
	GroupIDs           *[]int64        `json:"group_ids"`
	ExpiresAt          *time.Time      `json:"expires_at"`
	AutoPauseOnExpired *bool           `json:"auto_pause_on_expired"`
}

// AccountService 账号管理服务
type AccountService struct {
	accountRepo AccountRepository
	groupRepo   GroupRepository
}

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
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformSora, PlatformKiro, PlatformGrok:
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
		return errors.New("account is nil")
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return fmt.Errorf("%s api_key is required", account.Platform)
	}
	return validateCredentialsBaseURL(account.Platform, account.GetCredential("base_url"))
}

func validateOAuthCredentials(account *Account) error {
	if account == nil {
		return errors.New("account is nil")
	}
	if strings.TrimSpace(account.GetCredential("access_token")) == "" && strings.TrimSpace(account.GetCredential("refresh_token")) == "" {
		return fmt.Errorf("%s access_token or refresh_token is required", account.Platform)
	}
	return nil
}

func validateAccountTestCredentials(account *Account) error {
	if account == nil {
		return errors.New("account is nil")
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
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "access_token", "refresh_token", "api_key", "client_secret", "client_id", "session_token", "password", "cookie":
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
	if typed, ok := raw.(map[string]interface{}); ok {
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

type groupExistenceBatchChecker interface {
	ExistsByIDs(ctx context.Context, ids []int64) (map[int64]bool, error)
}

// NewAccountService 创建账号服务实例
func NewAccountService(accountRepo AccountRepository, groupRepo GroupRepository) *AccountService {
	return &AccountService{
		accountRepo: accountRepo,
		groupRepo:   groupRepo,
	}
}

// Create 创建账号
func (s *AccountService) Create(ctx context.Context, req CreateAccountRequest) (*Account, error) {
	if err := validatePlatformAccountType(req.Platform, req.Type); err != nil {
		return nil, err
	}
	if req.Platform == PlatformKiro && req.Type == AccountTypeOAuth {
		req.Credentials = NormalizeKiroOAuthCredentialShape(req.Credentials)
	}
	if req.Platform == PlatformKiro {
		if err := validateKiroAccountCredentials(req.Type, req.Credentials); err != nil {
			return nil, err
		}
	}

	// 验证分组是否存在（如果指定了分组）
	if len(req.GroupIDs) > 0 {
		if err := s.validateGroupIDsExist(ctx, req.GroupIDs); err != nil {
			return nil, err
		}
	}
	if err := s.validateOAuthOnlyGroups(ctx, req.Type, req.GroupIDs); err != nil {
		return nil, err
	}

	// 创建账号
	account := &Account{
		Name:        req.Name,
		Notes:       normalizeAccountNotes(req.Notes),
		Platform:    req.Platform,
		Type:        req.Type,
		Credentials: req.Credentials,
		Extra:       req.Extra,
		ProxyID:     req.ProxyID,
		Concurrency: req.Concurrency,
		Priority:    req.Priority,
		Status:      StatusActive,
		ExpiresAt:   req.ExpiresAt,
	}
	if req.AutoPauseOnExpired != nil {
		account.AutoPauseOnExpired = *req.AutoPauseOnExpired
	} else {
		account.AutoPauseOnExpired = true
	}

	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	// require_oauth_only 检查：apikey 类型账号不可加入限制分组
	if account.Type == AccountTypeAPIKey && len(req.GroupIDs) > 0 {
		for _, gid := range req.GroupIDs {
			g, err := s.groupRepo.GetByID(ctx, gid)
			if err != nil {
				return nil, err
			}
			if g.RequireOAuthOnly && (g.Platform == PlatformOpenAI || g.Platform == PlatformAntigravity || g.Platform == PlatformAnthropic || g.Platform == PlatformGemini || g.Platform == PlatformGrok) {
				return nil, fmt.Errorf("分组 [%s] 仅允许 OAuth 账号，apikey 类型账号无法加入", g.Name)
			}
		}
	}

	// 绑定分组
	if len(req.GroupIDs) > 0 {
		if err := s.accountRepo.BindGroups(ctx, account.ID, req.GroupIDs); err != nil {
			return nil, fmt.Errorf("bind groups: %w", err)
		}
	}

	return account, nil
}

// GetByID 根据ID获取账号
func (s *AccountService) GetByID(ctx context.Context, id int64) (*Account, error) {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	return account, nil
}

// List 获取账号列表
func (s *AccountService) List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	accounts, pagination, err := s.accountRepo.List(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("list accounts: %w", err)
	}
	return accounts, pagination, nil
}

// ListByPlatform 根据平台获取账号列表
func (s *AccountService) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	accounts, err := s.accountRepo.ListByPlatform(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("list accounts by platform: %w", err)
	}
	return accounts, nil
}

// ListByGroup 根据分组获取账号列表
func (s *AccountService) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	accounts, err := s.accountRepo.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("list accounts by group: %w", err)
	}
	return accounts, nil
}

// Update 更新账号
func (s *AccountService) Update(ctx context.Context, id int64, req UpdateAccountRequest) (*Account, error) {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}

	// 更新字段
	if req.Name != nil {
		account.Name = *req.Name
	}
	if req.Notes != nil {
		account.Notes = normalizeAccountNotes(req.Notes)
	}
	if req.Type != nil {
		account.Type = *req.Type
	}
	if err := validatePlatformAccountType(account.Platform, account.Type); err != nil {
		return nil, err
	}

	if req.Credentials != nil {
		nextCredentials := *req.Credentials
		if account.Platform == PlatformKiro && account.Type == AccountTypeOAuth {
			nextCredentials = NormalizeKiroOAuthCredentialShape(nextCredentials)
		}
		account.Credentials = mergeAccountCredentialsForAccountUpdate(account.Platform, account.Type, account.Credentials, nextCredentials, false)
	}

	if req.Extra != nil {
		account.Extra = *req.Extra
	}

	if req.ProxyID != nil {
		account.ProxyID = req.ProxyID
	}

	if req.Concurrency != nil {
		account.Concurrency = *req.Concurrency
	}

	if req.Priority != nil {
		account.Priority = *req.Priority
	}

	if req.Status != nil {
		account.Status = *req.Status
	}
	if req.ExpiresAt != nil {
		account.ExpiresAt = req.ExpiresAt
	}
	if req.AutoPauseOnExpired != nil {
		account.AutoPauseOnExpired = *req.AutoPauseOnExpired
	}

	// 先验证分组是否存在（在任何写操作之前）
	if req.GroupIDs != nil {
		if err := s.validateGroupIDsExist(ctx, *req.GroupIDs); err != nil {
			return nil, err
		}
		if err := s.validateOAuthOnlyGroups(ctx, account.Type, *req.GroupIDs); err != nil {
			return nil, err
		}
	}
	if account.Platform == PlatformKiro {
		if err := validateKiroAccountCredentials(account.Type, account.Credentials); err != nil {
			return nil, err
		}
	}

	// 执行更新
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, fmt.Errorf("update account: %w", err)
	}

	// require_oauth_only 检查
	if account.Type == AccountTypeAPIKey && req.GroupIDs != nil {
		for _, gid := range *req.GroupIDs {
			g, err := s.groupRepo.GetByID(ctx, gid)
			if err != nil {
				return nil, err
			}
			if g.RequireOAuthOnly && (g.Platform == PlatformOpenAI || g.Platform == PlatformAntigravity || g.Platform == PlatformAnthropic || g.Platform == PlatformGemini || g.Platform == PlatformGrok) {
				return nil, fmt.Errorf("分组 [%s] 仅允许 OAuth 账号，apikey 类型账号无法加入", g.Name)
			}
		}
	}

	// 绑定分组
	if req.GroupIDs != nil {
		if err := s.accountRepo.BindGroups(ctx, account.ID, *req.GroupIDs); err != nil {
			return nil, fmt.Errorf("bind groups: %w", err)
		}
	}

	return account, nil
}

// Delete 删除账号
// 优化：使用 ExistsByID 替代 GetByID 进行存在性检查，
// 避免加载完整账号对象及其关联数据，提升删除操作的性能
func (s *AccountService) Delete(ctx context.Context, id int64) error {
	// 使用轻量级的存在性检查，而非加载完整账号对象
	exists, err := s.accountRepo.ExistsByID(ctx, id)
	if err != nil {
		return fmt.Errorf("check account: %w", err)
	}
	// 明确返回账号不存在错误，便于调用方区分错误类型
	if !exists {
		return ErrAccountNotFound
	}

	if err := s.accountRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}

	return nil
}

func (s *AccountService) validateOAuthOnlyGroups(ctx context.Context, accountType string, groupIDs []int64) error {
	if accountType != AccountTypeAPIKey || len(groupIDs) == 0 {
		return nil
	}
	for _, gid := range groupIDs {
		g, err := s.groupRepo.GetByID(ctx, gid)
		if err != nil {
			return err
		}
		if g.RequireOAuthOnly && requiresOAuthOnlyAccount(g.Platform) {
			return fmt.Errorf("分组 [%s] 仅允许 OAuth 账号，apikey 类型账号无法加入", g.Name)
		}
	}
	return nil
}

func (s *AccountService) validateGroupIDsExist(ctx context.Context, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	if s.groupRepo == nil {
		return fmt.Errorf("group repository not configured")
	}

	if batchChecker, ok := s.groupRepo.(groupExistenceBatchChecker); ok {
		existsByID, err := batchChecker.ExistsByIDs(ctx, groupIDs)
		if err != nil {
			return fmt.Errorf("check groups exists: %w", err)
		}
		for _, groupID := range groupIDs {
			if groupID <= 0 {
				return fmt.Errorf("get group: %w", ErrGroupNotFound)
			}
			if !existsByID[groupID] {
				return fmt.Errorf("get group: %w", ErrGroupNotFound)
			}
		}
		return nil
	}

	for _, groupID := range groupIDs {
		_, err := s.groupRepo.GetByID(ctx, groupID)
		if err != nil {
			return fmt.Errorf("get group: %w", err)
		}
	}
	return nil
}

// UpdateStatus 更新账号状态
func (s *AccountService) UpdateStatus(ctx context.Context, id int64, status string, errorMessage string) error {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get account: %w", err)
	}

	account.Status = status
	account.ErrorMessage = errorMessage

	if err := s.accountRepo.Update(ctx, account); err != nil {
		return fmt.Errorf("update account: %w", err)
	}

	return nil
}

// UpdateLastUsed 更新最后使用时间
func (s *AccountService) UpdateLastUsed(ctx context.Context, id int64) error {
	if err := s.accountRepo.UpdateLastUsed(ctx, id); err != nil {
		return fmt.Errorf("update last used: %w", err)
	}
	return nil
}

// GetCredential 获取账号凭证（安全访问）
func (s *AccountService) GetCredential(ctx context.Context, id int64, key string) (string, error) {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get account: %w", err)
	}

	return account.GetCredential(key), nil
}

// TestCredentials 测试账号凭证是否有效（需要实现具体平台的测试逻辑）
func (s *AccountService) TestCredentials(ctx context.Context, id int64) error {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get account: %w", err)
	}

	switch account.Platform {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini:
		return validateAccountTestCredentials(account)
	case PlatformGrok:
		// Grok OAuth credentials are validated via token exchange/refresh and request-path probes.
		return nil
	default:
		return fmt.Errorf("unsupported platform: %s", account.Platform)
	}
}
