package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	kiroOAuthPortalURL        = "https://app.kiro.dev/signin"
	kiroOAuthTokenEndpoint    = "https://prod.us-east-1.auth.desktop.kiro.dev/oauth/token"
	kiroOAuthCallbackBaseURL  = "http://localhost:3128"
	kiroOAuthSessionTTL       = 30 * time.Minute
	kiroOAuthDefaultUserAgent = "sub2api Kiro OAuth"
	kiroIDCDefaultStartURL    = "https://view.awsapps.com/start"
	kiroIDCDefaultRegion      = "us-east-1"
	kiroIDCDeviceGrantType    = "urn:ietf:params:oauth:grant-type:device_code"
)

var kiroIDCDefaultScopes = []string{"codewhisperer:conversations"}

var kiroCodeExchangeFunc = exchangeKiroCodeForToken
var kiroFindCallbackBaseURLFunc = findKiroCallbackBaseURL
var kiroIDCRegisterClientFunc = registerKiroIDCClient
var kiroIDCStartDeviceAuthorizationFunc = startKiroIDCDeviceAuthorization
var kiroIDCCompleteDeviceAuthorizationFunc = completeKiroIDCDeviceAuthorization

type KiroOAuthSession struct {
	State           string
	CodeVerifier    string
	RedirectURI     string
	CallbackBaseURL string
	ProxyURL        string
	CreatedAt       time.Time
	IDCContinuation *KiroIDCContinuationSession

	// mu guards the mutable parts of a session that may be touched by
	// concurrent callback / continuation requests sharing the same session
	// pointer (the store hands out the same *KiroOAuthSession to every
	// caller). It must not be copied.
	mu sync.Mutex
	// consumed marks a session whose single-use authorization code has
	// already been (or is being) exchanged. Replaying the same session is
	// rejected so a leaked callback URL cannot be redeemed twice.
	consumed bool
}

// tryConsume atomically marks the session as consumed. It returns true only
// for the first caller; every subsequent caller (concurrent or sequential)
// gets false and must be rejected.
func (s *KiroOAuthSession) tryConsume() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.consumed {
		return false
	}
	s.consumed = true
	return true
}

type KiroOAuthSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*KiroOAuthSession
	stopOnce sync.Once
	stopCh   chan struct{}
}

func NewKiroOAuthSessionStore() *KiroOAuthSessionStore {
	store := &KiroOAuthSessionStore{
		sessions: make(map[string]*KiroOAuthSession),
		stopCh:   make(chan struct{}),
	}
	go store.cleanup()
	return store
}

func (s *KiroOAuthSessionStore) Set(sessionID string, session *KiroOAuthSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
}

func (s *KiroOAuthSessionStore) Get(sessionID string) (*KiroOAuthSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if time.Since(session.CreatedAt) > kiroOAuthSessionTTL {
		return nil, false
	}
	return session, true
}

func (s *KiroOAuthSessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *KiroOAuthSessionStore) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

func (s *KiroOAuthSessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, session := range s.sessions {
				if time.Since(session.CreatedAt) > kiroOAuthSessionTTL {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

type KiroOAuthService struct {
	sessionStore *KiroOAuthSessionStore
	proxyRepo    ProxyRepository
	usageService *KiroUsageService
}

func NewKiroOAuthService(
	proxyRepo ProxyRepository,
	httpUpstream HTTPUpstream,
	tlsFPProfileService *TLSFingerprintProfileService,
	settingService *SettingService,
) *KiroOAuthService {
	return &KiroOAuthService{
		sessionStore: NewKiroOAuthSessionStore(),
		proxyRepo:    proxyRepo,
		usageService: NewKiroUsageService().WithTransport(httpUpstream, tlsFPProfileService).WithSettingService(settingService),
	}
}

type KiroAuthURLResult struct {
	AuthURL     string `json:"auth_url"`
	SessionID   string `json:"session_id"`
	CallbackURL string `json:"callback_url"`
}

type KiroExchangeCallbackInput struct {
	SessionID   string
	CallbackURL string
	ProxyID     *int64
	StartURL    string
	IssuerURL   string
	IDCRegion   string
	Scopes      []string
	LoginHint   string
	ClientName  string
}

type KiroDeviceCompleteInput struct {
	SessionID string
	ProxyID   *int64
}

type KiroOAuthProgressResult struct {
	TokenInfo    *KiroTokenInfo           `json:"token_info,omitempty"`
	Continuation *KiroIDCContinuationInfo `json:"continuation,omitempty"`
}

func (s *KiroOAuthService) resolveProxyURL(ctx context.Context, proxyID *int64, fallbackProxyURL string) (string, error) {
	if proxyID == nil {
		return fallbackProxyURL, nil
	}
	if s == nil || s.proxyRepo == nil {
		return "", fmt.Errorf("kiro proxy repository is unavailable")
	}
	proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
	if err != nil {
		return "", err
	}
	if proxy == nil {
		return "", ErrProxyNotFound
	}
	return proxy.URL(), nil
}

type KiroTokenInfo struct {
	AccessToken      string `json:"access_token,omitempty"`
	RefreshToken     string `json:"refresh_token,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	AuthMethod       string `json:"auth_method,omitempty"`
	ClientID         string `json:"client_id,omitempty"`
	ClientSecret     string `json:"client_secret,omitempty"`
	Region           string `json:"region,omitempty"`
	AuthRegion       string `json:"auth_region,omitempty"`
	APIRegion        string `json:"api_region,omitempty"`
	ProfileARN       string `json:"profile_arn,omitempty"`
	ProfileID        string `json:"profile_id,omitempty"`
	Email            string `json:"email,omitempty"`
	Name             string `json:"name,omitempty"`
	UserID           string `json:"user_id,omitempty"`
	LoginProvider    string `json:"login_provider,omitempty"`
	PlanName         string `json:"plan_name,omitempty"`
	PlanTier         string `json:"plan_tier,omitempty"`
	UsageResetAt     string `json:"usage_reset_at,omitempty"`
	Status           string `json:"status,omitempty"`
	StatusReason     string `json:"status_reason,omitempty"`
	IssuerURL        string `json:"issuer_url,omitempty"`
	IDCRegion        string `json:"idc_region,omitempty"`
	Scopes           string `json:"scopes,omitempty"`
	LoginHint        string `json:"login_hint,omitempty"`
	SubscriptionType string `json:"subscription_type,omitempty"`
}

type KiroIDCContinuationSession struct {
	LoginOption             string
	ClientID                string
	ClientSecret            string
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	StartURL                string
	IssuerURL               string
	Region                  string
	Scopes                  []string
	LoginHint               string
	IntervalSeconds         int64
	ExpiresAt               time.Time
	CreatedAt               time.Time
}

type KiroIDCContinuationInfo struct {
	SessionID               string   `json:"session_id"`
	Status                  string   `json:"status"`
	AuthMethod              string   `json:"auth_method"`
	LoginOption             string   `json:"login_option,omitempty"`
	StartURL                string   `json:"start_url,omitempty"`
	IssuerURL               string   `json:"issuer_url,omitempty"`
	IDCRegion               string   `json:"idc_region,omitempty"`
	Scopes                  []string `json:"scopes,omitempty"`
	LoginHint               string   `json:"login_hint,omitempty"`
	UserCode                string   `json:"user_code,omitempty"`
	VerificationURI         string   `json:"verification_uri,omitempty"`
	VerificationURIComplete string   `json:"verification_uri_complete,omitempty"`
	IntervalSeconds         int64    `json:"interval_seconds,omitempty"`
	ExpiresAt               string   `json:"expires_at,omitempty"`
	Message                 string   `json:"message,omitempty"`
}

type kiroIDCRegisterClientInput struct {
	ProxyURL     string
	ClientName   string
	RedirectURIs []string
	Scopes       []string
	IssuerURL    string
	GrantTypes   []string
	Region       string
}

type kiroIDCRegisterClientResult struct {
	ClientID     string
	ClientSecret string
}

type kiroIDCStartDeviceAuthorizationInput struct {
	ProxyURL     string
	ClientID     string
	ClientSecret string
	StartURL     string
	Region       string
}

type kiroIDCStartDeviceAuthorizationResult struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresIn               int64
	Interval                int64
}

type kiroIDCCompleteDeviceAuthorizationInput struct {
	ProxyURL     string
	ClientID     string
	ClientSecret string
	DeviceCode   string
	Region       string
}

type kiroIDCAPIError struct {
	Operation        string
	StatusCode       int
	ErrorCode        string
	ErrorDescription string
}

func (e *kiroIDCAPIError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{
		strings.TrimSpace(e.Operation),
		strings.TrimSpace(e.ErrorCode),
		strings.TrimSpace(e.ErrorDescription),
	}
	text := strings.TrimSpace(strings.Join(filterEmptyKiroStrings(parts), ": "))
	if text == "" {
		return "kiro idc request failed"
	}
	return text
}

func (s *KiroOAuthService) GenerateAuthURL(ctx context.Context, proxyID *int64) (*KiroAuthURLResult, error) {
	if s == nil || s.sessionStore == nil {
		return nil, fmt.Errorf("kiro oauth service is unavailable")
	}

	state, err := generateKiroOAuthToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate kiro oauth state: %w", err)
	}
	codeVerifier, err := generateKiroOAuthToken(48)
	if err != nil {
		return nil, fmt.Errorf("failed to generate kiro oauth code verifier: %w", err)
	}
	sessionID, err := generateKiroOAuthToken(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate kiro oauth session id: %w", err)
	}

	var proxyURL string
	if proxyID != nil {
		if s.proxyRepo == nil {
			return nil, fmt.Errorf("kiro proxy repository is unavailable")
		}
		proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
		if err != nil {
			return nil, err
		}
		if proxy == nil {
			return nil, ErrProxyNotFound
		}
		proxyURL = proxy.URL()
	}

	callbackBaseURL, err := kiroFindCallbackBaseURLFunc()
	if err != nil {
		return nil, err
	}
	redirectURI := strings.TrimSpace(callbackBaseURL)
	s.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    codeVerifier,
		RedirectURI:     redirectURI,
		CallbackBaseURL: redirectURI,
		ProxyURL:        proxyURL,
		CreatedAt:       time.Now(),
	})

	params := url.Values{}
	params.Set("state", state)
	params.Set("code_challenge", generateKiroCodeChallenge(codeVerifier))
	params.Set("code_challenge_method", "S256")
	params.Set("redirect_uri", redirectURI)
	params.Set("redirect_from", "KiroIDE")

	return &KiroAuthURLResult{
		AuthURL:     kiroOAuthPortalURL + "?" + params.Encode(),
		SessionID:   sessionID,
		CallbackURL: redirectURI,
	}, nil
}

func (s *KiroOAuthService) ExchangeCallback(ctx context.Context, input *KiroExchangeCallbackInput) (*KiroTokenInfo, error) {
	result, err := s.exchangeCallbackProgress(ctx, input, false)
	if err != nil {
		return nil, err
	}
	if result == nil || result.TokenInfo == nil {
		return nil, fmt.Errorf("kiro callback did not return token info")
	}
	return result.TokenInfo, nil
}

func (s *KiroOAuthService) ExchangeCallbackOrStartContinuation(ctx context.Context, input *KiroExchangeCallbackInput) (*KiroOAuthProgressResult, error) {
	return s.exchangeCallbackProgress(ctx, input, true)
}

func (s *KiroOAuthService) CompleteDeviceAuthorization(ctx context.Context, input *KiroDeviceCompleteInput) (*KiroOAuthProgressResult, error) {
	if s == nil || s.sessionStore == nil {
		return nil, fmt.Errorf("kiro oauth service is unavailable")
	}
	if input == nil {
		return nil, fmt.Errorf("kiro device completion input is required")
	}
	if strings.TrimSpace(input.SessionID) == "" {
		return nil, fmt.Errorf("kiro device completion session id is required")
	}

	session, ok := s.sessionStore.Get(input.SessionID)
	if !ok {
		return nil, fmt.Errorf("kiro 授权会话不存在或已过期。请重新生成授权链接，并在当前弹窗同一轮流程中于 30 分钟内完成授权后，再粘贴最新地址栏中的完整回调 URL")
	}
	if session.IDCContinuation == nil {
		return nil, fmt.Errorf("kiro device authorization continuation was not started for this session")
	}
	if time.Now().After(session.IDCContinuation.ExpiresAt) {
		s.sessionStore.Delete(input.SessionID)
		return nil, fmt.Errorf("kiro device authorization session expired; please restart the Kiro OAuth flow")
	}

	proxyURL, err := s.resolveProxyURL(ctx, input.ProxyID, session.ProxyURL)
	if err != nil {
		return nil, err
	}

	tokenPayload, err := kiroIDCCompleteDeviceAuthorizationFunc(ctx, kiroIDCCompleteDeviceAuthorizationInput{
		ProxyURL:     proxyURL,
		ClientID:     session.IDCContinuation.ClientID,
		ClientSecret: session.IDCContinuation.ClientSecret,
		DeviceCode:   session.IDCContinuation.DeviceCode,
		Region:       firstNonEmptyKiroString(session.IDCContinuation.Region, kiroIDCDefaultRegion),
	})
	if err != nil {
		var apiErr *kiroIDCAPIError
		if errors.As(err, &apiErr) {
			switch strings.ToLower(strings.TrimSpace(apiErr.ErrorCode)) {
			case "authorization_pending":
				return &KiroOAuthProgressResult{
					Continuation: buildKiroIDCContinuationInfo(input.SessionID, session.IDCContinuation, "authorization_pending", "complete the browser/device authorization, then call device-complete again"),
				}, nil
			case "slow_down":
				if session.IDCContinuation.IntervalSeconds <= 0 {
					session.IDCContinuation.IntervalSeconds = 5
				}
				session.IDCContinuation.IntervalSeconds += 5
				s.sessionStore.Set(input.SessionID, session)
				return &KiroOAuthProgressResult{
					Continuation: buildKiroIDCContinuationInfo(input.SessionID, session.IDCContinuation, "authorization_pending", "authorization is still pending; poll more slowly before calling device-complete again"),
				}, nil
			case "expired_token", "access_denied":
				s.sessionStore.Delete(input.SessionID)
			}
		}
		return nil, err
	}

	enrichedPayload := map[string]any{
		"auth_method":   "idc",
		"client_id":     session.IDCContinuation.ClientID,
		"client_secret": session.IDCContinuation.ClientSecret,
		"issuer_url":    session.IDCContinuation.IssuerURL,
		"idc_region":    session.IDCContinuation.Region,
		"auth_region":   session.IDCContinuation.Region,
		"region":        session.IDCContinuation.Region,
		"login_hint":    session.IDCContinuation.LoginHint,
		"login_option":  session.IDCContinuation.LoginOption,
	}
	if len(session.IDCContinuation.Scopes) > 0 {
		enrichedPayload["scope"] = strings.Join(session.IDCContinuation.Scopes, " ")
	}
	for key, value := range tokenPayload {
		enrichedPayload[key] = value
	}

	query := url.Values{}
	if session.IDCContinuation.LoginHint != "" {
		query.Set("login_hint", session.IDCContinuation.LoginHint)
	}
	if len(session.IDCContinuation.Scopes) > 0 {
		query.Set("scope", strings.Join(session.IDCContinuation.Scopes, " "))
	}
	tokenInfo := buildKiroTokenInfo(enrichedPayload, query, session.IDCContinuation.LoginOption)
	tokenInfo.AuthMethod = "idc"
	tokenInfo.ClientID = firstNonEmptyKiroString(tokenInfo.ClientID, session.IDCContinuation.ClientID)
	tokenInfo.ClientSecret = firstNonEmptyKiroString(tokenInfo.ClientSecret, session.IDCContinuation.ClientSecret)
	tokenInfo.AuthRegion = firstNonEmptyKiroString(tokenInfo.AuthRegion, session.IDCContinuation.Region)
	tokenInfo.IDCRegion = firstNonEmptyKiroString(tokenInfo.IDCRegion, session.IDCContinuation.Region)
	tokenInfo.Region = firstNonEmptyKiroString(tokenInfo.Region, session.IDCContinuation.Region)
	tokenInfo.IssuerURL = firstNonEmptyKiroString(tokenInfo.IssuerURL, session.IDCContinuation.IssuerURL)
	tokenInfo.LoginHint = firstNonEmptyKiroString(tokenInfo.LoginHint, session.IDCContinuation.LoginHint)
	s.enrichTokenInfo(ctx, tokenInfo)
	s.sessionStore.Delete(input.SessionID)

	return &KiroOAuthProgressResult{TokenInfo: tokenInfo}, nil
}

func (s *KiroOAuthService) exchangeCallbackProgress(ctx context.Context, input *KiroExchangeCallbackInput, allowIDCContinuation bool) (*KiroOAuthProgressResult, error) {
	if s == nil || s.sessionStore == nil {
		return nil, fmt.Errorf("kiro oauth service is unavailable")
	}
	if input == nil {
		return nil, fmt.Errorf("kiro callback input is required")
	}
	if strings.TrimSpace(input.SessionID) == "" {
		return nil, fmt.Errorf("kiro callback session id is required")
	}
	if strings.TrimSpace(input.CallbackURL) == "" {
		return nil, fmt.Errorf("kiro callback URL is required")
	}

	session, ok := s.sessionStore.Get(input.SessionID)
	if !ok {
		return nil, fmt.Errorf("kiro 授权会话不存在或已过期。请重新生成授权链接，并在当前弹窗同一轮流程中于 30 分钟内完成授权后，再粘贴最新地址栏中的完整回调 URL")
	}

	redirectURI := kiroOAuthSessionRedirectURI(session)
	callbackBaseURL := strings.TrimSpace(session.CallbackBaseURL)
	if callbackBaseURL == "" {
		callbackBaseURL = redirectURI
	}
	parsedURL, err := parseKiroCallbackURL(input.CallbackURL, callbackBaseURL)
	if err != nil {
		return nil, err
	}

	if path := parsedURL.Path; path != "/oauth/callback" && path != "/signin/callback" {
		return nil, fmt.Errorf("invalid kiro callback path: %s", path)
	}

	query := parsedURL.Query()
	if errCode := strings.TrimSpace(query.Get("error")); errCode != "" {
		description := strings.TrimSpace(query.Get("error_description"))
		if description == "" {
			return nil, fmt.Errorf("kiro authorization failed: %s", errCode)
		}
		return nil, fmt.Errorf("kiro authorization failed: %s (%s)", errCode, description)
	}

	if state := strings.TrimSpace(query.Get("state")); state == "" || state != session.State {
		return nil, fmt.Errorf("kiro 授权状态不匹配：你粘贴的回调地址不属于当前这次授权流程。请重新生成授权链接，并在同一标签页完成授权后，把最新地址栏中的完整回调 URL 粘贴回来；不要刷新页面、不要再次点击生成，且需在 30 分钟内完成")
	}

	loginOption := strings.ToLower(strings.TrimSpace(firstNonEmptyKiroString(
		query.Get("login_option"),
		query.Get("loginOption"),
	)))
	code := strings.TrimSpace(query.Get("code"))
	if code == "" {
		switch loginOption {
		case "builderid", "awsidc", "internal":
			if allowIDCContinuation {
				return s.startOrResumeIDCContinuation(ctx, input, session, query, loginOption)
			}
			return nil, fmt.Errorf("the current kiro login mode requires additional IDC material and cannot be imported automatically")
		case "external_idp":
			return nil, fmt.Errorf("the current kiro external IdP callback does not provide an authorization code")
		default:
			return nil, fmt.Errorf("kiro callback is missing authorization code")
		}
	}

	proxyURL, err := s.resolveProxyURL(ctx, input.ProxyID, session.ProxyURL)
	if err != nil {
		return nil, err
	}

	// Atomically claim the session before redeeming the single-use
	// authorization code. Concurrent or replayed callbacks for the same
	// session pointer lose the race and are rejected, so a leaked callback
	// URL cannot be exchanged twice.
	if !session.tryConsume() {
		return nil, fmt.Errorf("kiro 授权会话已被使用，无法重复兑换。请重新生成授权链接并在同一轮流程中完成授权")
	}

	tokenExchangeRedirectURI := buildKiroTokenExchangeRedirectURI(redirectURI, parsedURL, loginOption)
	tokenPayload, err := kiroCodeExchangeFunc(ctx, code, session.CodeVerifier, tokenExchangeRedirectURI, proxyURL)
	if err != nil {
		// The code may be single-use upstream; once consumed the session
		// cannot be retried, so drop it and force a fresh authorization link.
		s.sessionStore.Delete(input.SessionID)
		return nil, err
	}
	s.sessionStore.Delete(input.SessionID)

	tokenInfo := buildKiroTokenInfo(tokenPayload, query, loginOption)
	s.enrichTokenInfo(ctx, tokenInfo)
	return &KiroOAuthProgressResult{TokenInfo: tokenInfo}, nil
}

func (s *KiroOAuthService) enrichTokenInfo(ctx context.Context, tokenInfo *KiroTokenInfo) {
	s.enrichTokenInfoForAccount(ctx, nil, tokenInfo)
}

func (s *KiroOAuthService) enrichTokenInfoForAccount(ctx context.Context, baseAccount *Account, tokenInfo *KiroTokenInfo) {
	if s == nil || tokenInfo == nil || tokenInfo.AccessToken == "" || tokenInfo.ProfileARN == "" || s.usageService == nil {
		return
	}

	credentials := map[string]any{
		"access_token":  tokenInfo.AccessToken,
		"refresh_token": tokenInfo.RefreshToken,
		"profile_arn":   tokenInfo.ProfileARN,
		"region":        firstNonEmptyKiroString(tokenInfo.Region, "us-east-1"),
		"auth_region":   tokenInfo.AuthRegion,
		"api_region":    tokenInfo.APIRegion,
	}
	account := baseAccount
	if account == nil {
		account = &Account{
			Platform:    PlatformKiro,
			Type:        AccountTypeOAuth,
			Credentials: credentials,
			Concurrency: 1,
		}
	} else {
		cloned := *account
		cloned.Credentials = MergeCredentials(cloneCredentials(account.Credentials), credentials)
		if cloned.Concurrency <= 0 {
			cloned.Concurrency = 1
		}
		account = &cloned
	}

	usageService := s.usageService.WithProxyRepo(s.proxyRepo)
	usage, err := usageService.FetchUsageLimits(ctx, account, tokenInfo.AccessToken)
	if err != nil {
		return
	}

	subscriptionTitle := strings.TrimSpace(usage.SubscriptionTitle())
	if subscriptionTitle != "" {
		tokenInfo.PlanName = subscriptionTitle
		tokenInfo.SubscriptionType = subscriptionTitle
	}
	if resetAt := usage.ResetAt(); resetAt != nil {
		tokenInfo.UsageResetAt = resetAt.Format(time.RFC3339)
	}
}

func (s *KiroOAuthService) EnrichRefreshedCredentials(ctx context.Context, credentials map[string]any) map[string]any {
	return s.EnrichRefreshedCredentialsForAccount(ctx, nil, credentials)
}

func (s *KiroOAuthService) EnrichRefreshedCredentialsForAccount(ctx context.Context, account *Account, credentials map[string]any) map[string]any {
	if credentials == nil {
		return nil
	}
	tokenInfo := buildKiroTokenInfo(credentials, url.Values{}, NormalizeKiroAuthMethod(credentials))
	if s != nil {
		s.enrichTokenInfoForAccount(ctx, account, tokenInfo)
	}
	return MergeCredentials(credentials, kiroTokenInfoMap(tokenInfo))
}

func kiroTokenInfoMap(tokenInfo *KiroTokenInfo) map[string]any {
	if tokenInfo == nil {
		return nil
	}
	values := map[string]any{
		"access_token":      tokenInfo.AccessToken,
		"refresh_token":     tokenInfo.RefreshToken,
		"token_type":        tokenInfo.TokenType,
		"expires_at":        tokenInfo.ExpiresAt,
		"auth_method":       tokenInfo.AuthMethod,
		"client_id":         tokenInfo.ClientID,
		"client_secret":     tokenInfo.ClientSecret,
		"region":            tokenInfo.Region,
		"auth_region":       tokenInfo.AuthRegion,
		"api_region":        tokenInfo.APIRegion,
		"profile_arn":       tokenInfo.ProfileARN,
		"profile_id":        tokenInfo.ProfileID,
		"email":             tokenInfo.Email,
		"name":              tokenInfo.Name,
		"user_id":           tokenInfo.UserID,
		"login_provider":    tokenInfo.LoginProvider,
		"plan_name":         tokenInfo.PlanName,
		"plan_tier":         tokenInfo.PlanTier,
		"usage_reset_at":    tokenInfo.UsageResetAt,
		"status":            tokenInfo.Status,
		"status_reason":     tokenInfo.StatusReason,
		"issuer_url":        tokenInfo.IssuerURL,
		"idc_region":        tokenInfo.IDCRegion,
		"scopes":            tokenInfo.Scopes,
		"login_hint":        tokenInfo.LoginHint,
		"subscription_type": tokenInfo.SubscriptionType,
	}
	for key, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			delete(values, key)
		}
	}
	return values
}

func (s *KiroOAuthService) Stop() {
	if s == nil || s.sessionStore == nil {
		return
	}
	s.sessionStore.Stop()
}

func generateKiroOAuthToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func generateKiroCodeChallenge(codeVerifier string) string {
	hash := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func findKiroCallbackBaseURL() (string, error) {
	return kiroOAuthCallbackBaseURL, nil
}

func kiroOAuthSessionRedirectURI(session *KiroOAuthSession) string {
	if session == nil {
		return ""
	}
	if redirectURI := strings.TrimSpace(session.RedirectURI); redirectURI != "" {
		return redirectURI
	}
	return strings.TrimSpace(session.CallbackBaseURL)
}

func parseKiroCallbackURL(rawValue, callbackBaseURL string) (*url.URL, error) {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return nil, fmt.Errorf("kiro callback URL is required")
	}

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid kiro callback URL: %w", err)
		}
		base, err := url.Parse(strings.TrimRight(strings.TrimSpace(callbackBaseURL), "/"))
		if err != nil || base == nil || base.Scheme == "" || base.Host == "" {
			return nil, fmt.Errorf("invalid kiro callback base URL")
		}
		if !sameKiroCallbackOrigin(parsed, base) {
			return nil, fmt.Errorf("kiro callback origin does not match this authorization session")
		}
		return parsed, nil
	}

	base := strings.TrimRight(strings.TrimSpace(callbackBaseURL), "/")
	if strings.HasPrefix(trimmed, "/") {
		parsed, err := url.Parse(base + trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid kiro callback URL: %w", err)
		}
		return parsed, nil
	}

	callbackPath := defaultKiroCallbackPathFromInput(trimmed)
	parsed, err := url.Parse(base + "/oauth/callback?" + strings.TrimLeft(trimmed, "?"))
	if err != nil {
		return nil, fmt.Errorf("invalid kiro callback URL: %w", err)
	}
	parsed.Path = callbackPath
	return parsed, nil
}

func sameKiroCallbackOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func defaultKiroCallbackPathFromInput(rawValue string) string {
	queryText := strings.TrimLeft(strings.TrimSpace(rawValue), "?")
	values, err := url.ParseQuery(queryText)
	if err != nil {
		return "/oauth/callback"
	}
	loginOption := strings.ToLower(strings.TrimSpace(firstNonEmptyKiroString(
		values.Get("login_option"),
		values.Get("loginOption"),
	)))
	if loginOption == "" || loginOption == "social" {
		return "/oauth/callback"
	}
	return "/signin/callback"
}

func buildKiroTokenExchangeRedirectURI(callbackBaseURL string, parsedCallbackURL *url.URL, loginOption string) string {
	base := strings.TrimRight(strings.TrimSpace(callbackBaseURL), "/")
	if base == "" {
		if parsedCallbackURL != nil {
			return strings.TrimSpace(parsedCallbackURL.String())
		}
		return ""
	}

	callbackPath := "/oauth/callback"
	if parsedCallbackURL != nil {
		if path := strings.TrimSpace(parsedCallbackURL.Path); path != "" {
			callbackPath = path
		}
	}
	if !strings.HasPrefix(callbackPath, "/") {
		callbackPath = "/" + callbackPath
	}

	redirectURI := base + callbackPath
	if trimmedLoginOption := strings.TrimSpace(loginOption); trimmedLoginOption != "" {
		redirectURI += "?login_option=" + url.QueryEscape(trimmedLoginOption)
	}
	return redirectURI
}

func (s *KiroOAuthService) startOrResumeIDCContinuation(
	ctx context.Context,
	input *KiroExchangeCallbackInput,
	session *KiroOAuthSession,
	query url.Values,
	loginOption string,
) (*KiroOAuthProgressResult, error) {
	if session == nil {
		return nil, fmt.Errorf("kiro oauth session is required")
	}

	// Hold the per-session lock for the entire register -> device-authorize
	// -> store sequence. Two concurrent callbacks sharing this session pointer
	// would otherwise each register a fresh IDC client and the later Set would
	// clobber the earlier one (lost update), orphaning a client + leaking its
	// client_secret. Serializing here lets the second caller observe the
	// pending continuation the first one stored and reuse it.
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.IDCContinuation != nil {
		if time.Now().Before(session.IDCContinuation.ExpiresAt) && strings.EqualFold(strings.TrimSpace(session.IDCContinuation.LoginOption), strings.TrimSpace(loginOption)) {
			return &KiroOAuthProgressResult{
				Continuation: buildKiroIDCContinuationInfo(input.SessionID, session.IDCContinuation, "authorization_pending", "device authorization is already pending for this session"),
			}, nil
		}
		session.IDCContinuation = nil
	}

	proxyURL, err := s.resolveProxyURL(ctx, input.ProxyID, session.ProxyURL)
	if err != nil {
		return nil, err
	}
	session.ProxyURL = proxyURL

	cfg, err := resolveKiroIDCContinuationConfig(input, query, loginOption, session)
	if err != nil {
		return nil, err
	}
	registerResult, err := kiroIDCRegisterClientFunc(ctx, kiroIDCRegisterClientInput{
		ProxyURL:   proxyURL,
		ClientName: cfg.ClientName,
		Scopes:     cfg.Scopes,
		IssuerURL:  cfg.IssuerURL,
		GrantTypes: []string{kiroIDCDeviceGrantType, "refresh_token"},
		Region:     cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	deviceResult, err := kiroIDCStartDeviceAuthorizationFunc(ctx, kiroIDCStartDeviceAuthorizationInput{
		ProxyURL:     proxyURL,
		ClientID:     registerResult.ClientID,
		ClientSecret: registerResult.ClientSecret,
		StartURL:     cfg.StartURL,
		Region:       cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	continuation := &KiroIDCContinuationSession{
		LoginOption:             loginOption,
		ClientID:                registerResult.ClientID,
		ClientSecret:            registerResult.ClientSecret,
		DeviceCode:              deviceResult.DeviceCode,
		UserCode:                deviceResult.UserCode,
		VerificationURI:         deviceResult.VerificationURI,
		VerificationURIComplete: firstNonEmptyKiroString(deviceResult.VerificationURIComplete, deviceResult.VerificationURI),
		StartURL:                cfg.StartURL,
		IssuerURL:               cfg.IssuerURL,
		Region:                  cfg.Region,
		Scopes:                  cfg.Scopes,
		LoginHint:               cfg.LoginHint,
		IntervalSeconds:         firstNonZeroKiroInt(deviceResult.Interval, 5),
		ExpiresAt:               time.Now().Add(time.Duration(firstNonZeroKiroInt(deviceResult.ExpiresIn, 600)) * time.Second),
		CreatedAt:               time.Now(),
	}
	session.IDCContinuation = continuation
	s.sessionStore.Set(input.SessionID, session)

	return &KiroOAuthProgressResult{
		Continuation: buildKiroIDCContinuationInfo(input.SessionID, continuation, "authorization_pending", "complete the device authorization in your browser, then call device-complete to retrieve the final Kiro IDC tokens"),
	}, nil
}

type kiroIDCContinuationConfig struct {
	ClientName   string
	StartURL     string
	IssuerURL    string
	Region       string
	Scopes       []string
	LoginHint    string
	RedirectURIs []string
}

func resolveKiroIDCContinuationConfig(input *KiroExchangeCallbackInput, query url.Values, loginOption string, session *KiroOAuthSession) (kiroIDCContinuationConfig, error) {
	redirectURI := strings.TrimSpace(kiroOAuthSessionRedirectURI(session))
	issuerURL := firstNonEmptyKiroString(
		strings.TrimSpace(query.Get("issuer_url")),
		strings.TrimSpace(query.Get("issuerUrl")),
		strings.TrimSpace(input.IssuerURL),
	)
	startURL := firstNonEmptyKiroString(
		strings.TrimSpace(query.Get("start_url")),
		strings.TrimSpace(query.Get("startUrl")),
		strings.TrimSpace(input.StartURL),
		kiroIDCDefaultStartURL,
	)
	region := firstNonEmptyKiroString(
		deriveKiroIDCRegionFromIssuerURL(issuerURL),
		strings.TrimSpace(query.Get("idc_region")),
		strings.TrimSpace(query.Get("idcRegion")),
		strings.TrimSpace(query.Get("auth_region")),
		strings.TrimSpace(query.Get("authRegion")),
		strings.TrimSpace(query.Get("region")),
		strings.TrimSpace(input.IDCRegion),
		kiroIDCDefaultRegion,
	)

	// Security: issuer_url / start_url / region originate from the (untrusted)
	// OAuth callback query or request body. Without an allowlist an attacker
	// could point the IDC client registration / device authorization at a
	// forged OIDC endpoint and harvest the client_secret we mint. Only AWS
	// official domains are accepted.
	if err := validateKiroIDCRegion(region); err != nil {
		return kiroIDCContinuationConfig{}, err
	}
	if err := validateKiroIDCIssuerURL(issuerURL); err != nil {
		return kiroIDCContinuationConfig{}, err
	}
	if err := validateKiroIDCStartURL(startURL); err != nil {
		return kiroIDCContinuationConfig{}, err
	}

	clientName := firstNonEmptyKiroString(
		strings.TrimSpace(input.ClientName),
		"sub2api Kiro "+strings.ToUpper(strings.TrimSpace(loginOption)),
	)
	return kiroIDCContinuationConfig{
		ClientName: clientName,
		StartURL:   startURL,
		IssuerURL:  issuerURL,
		Region:     region,
		Scopes:     normalizeKiroScopes(input.Scopes, query),
		LoginHint: firstNonEmptyKiroString(
			strings.TrimSpace(input.LoginHint),
			strings.TrimSpace(query.Get("login_hint")),
			strings.TrimSpace(query.Get("loginHint")),
		),
		RedirectURIs: filterEmptyKiroStrings([]string{redirectURI}),
	}, nil
}

// kiroIDCAllowedRegions is the allowlist of AWS regions that may be used for
// Kiro IDC client registration and device authorization. Region values flow
// into the oidc.<region>.amazonaws.com endpoint hostname, so an unvalidated
// value could redirect requests to an attacker-controlled host.
var kiroIDCAllowedRegions = map[string]struct{}{
	"us-east-1":      {},
	"us-east-2":      {},
	"us-west-1":      {},
	"us-west-2":      {},
	"af-south-1":     {},
	"ap-east-1":      {},
	"ap-south-1":     {},
	"ap-south-2":     {},
	"ap-northeast-1": {},
	"ap-northeast-2": {},
	"ap-northeast-3": {},
	"ap-southeast-1": {},
	"ap-southeast-2": {},
	"ap-southeast-3": {},
	"ap-southeast-4": {},
	"ca-central-1":   {},
	"eu-central-1":   {},
	"eu-central-2":   {},
	"eu-west-1":      {},
	"eu-west-2":      {},
	"eu-west-3":      {},
	"eu-north-1":     {},
	"eu-south-1":     {},
	"eu-south-2":     {},
	"il-central-1":   {},
	"me-south-1":     {},
	"me-central-1":   {},
	"sa-east-1":      {},
}

func validateKiroIDCRegion(region string) error {
	trimmed := strings.ToLower(strings.TrimSpace(region))
	if trimmed == "" {
		return fmt.Errorf("kiro idc region is required")
	}
	if _, ok := kiroIDCAllowedRegions[trimmed]; !ok {
		return fmt.Errorf("kiro idc region %q is not an allowed AWS region", region)
	}
	return nil
}

// validateKiroIDCIssuerURL accepts only AWS IAM Identity Center OIDC issuers of
// the form https://oidc.<region>.amazonaws.com (no path/userinfo/port). An empty
// issuer is permitted because the start_url + region are sufficient to drive the
// flow; only a present-but-forged issuer is rejected.
func validateKiroIDCIssuerURL(issuerURL string) error {
	trimmed := strings.TrimSpace(issuerURL)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid kiro idc issuer_url: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("kiro idc issuer_url must use https: %q", issuerURL)
	}
	if parsed.User != nil || parsed.Port() != "" {
		return fmt.Errorf("kiro idc issuer_url has an unexpected host component: %q", issuerURL)
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if !strings.HasPrefix(host, "oidc.") || !strings.HasSuffix(host, ".amazonaws.com") {
		return fmt.Errorf("kiro idc issuer_url host %q is not an AWS OIDC endpoint", host)
	}
	region := strings.TrimSuffix(strings.TrimPrefix(host, "oidc."), ".amazonaws.com")
	// Reject extra labels (e.g. oidc.evil.example.amazonaws.com) by requiring
	// the middle segment to be exactly a single allowed region label.
	if strings.Contains(region, ".") {
		return fmt.Errorf("kiro idc issuer_url host %q is not an AWS OIDC endpoint", host)
	}
	if err := validateKiroIDCRegion(region); err != nil {
		return err
	}
	return nil
}

// validateKiroIDCStartURL accepts only AWS IAM Identity Center access-portal
// start URLs hosted under *.awsapps.com (e.g. https://view.awsapps.com/start).
func validateKiroIDCStartURL(startURL string) error {
	trimmed := strings.TrimSpace(startURL)
	if trimmed == "" {
		return fmt.Errorf("kiro idc start_url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid kiro idc start_url: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("kiro idc start_url must use https: %q", startURL)
	}
	if parsed.User != nil || parsed.Port() != "" {
		return fmt.Errorf("kiro idc start_url has an unexpected host component: %q", startURL)
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	// Require a non-empty subdomain label in front of awsapps.com so a bare
	// "awsapps.com" or an attacker domain like "evil-awsapps.com" is rejected.
	if host == "awsapps.com" || !strings.HasSuffix(host, ".awsapps.com") {
		return fmt.Errorf("kiro idc start_url host %q is not an AWS access portal", host)
	}
	label := strings.TrimSuffix(host, ".awsapps.com")
	if label == "" || strings.HasPrefix(label, ".") {
		return fmt.Errorf("kiro idc start_url host %q is not an AWS access portal", host)
	}
	return nil
}

func buildKiroIDCContinuationInfo(sessionID string, continuation *KiroIDCContinuationSession, status, message string) *KiroIDCContinuationInfo {
	if continuation == nil {
		return nil
	}
	info := &KiroIDCContinuationInfo{
		SessionID:               strings.TrimSpace(sessionID),
		Status:                  firstNonEmptyKiroString(status, "authorization_pending"),
		AuthMethod:              "idc",
		LoginOption:             continuation.LoginOption,
		StartURL:                continuation.StartURL,
		IssuerURL:               continuation.IssuerURL,
		IDCRegion:               continuation.Region,
		Scopes:                  continuation.Scopes,
		LoginHint:               continuation.LoginHint,
		UserCode:                continuation.UserCode,
		VerificationURI:         continuation.VerificationURI,
		VerificationURIComplete: continuation.VerificationURIComplete,
		IntervalSeconds:         continuation.IntervalSeconds,
		Message:                 strings.TrimSpace(message),
	}
	if !continuation.ExpiresAt.IsZero() {
		info.ExpiresAt = continuation.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return info
}

func exchangeKiroCodeForToken(
	ctx context.Context,
	code string,
	codeVerifier string,
	redirectURI string,
	proxyURL string,
) (map[string]any, error) {
	body, err := json.Marshal(map[string]any{
		"code":          code,
		"code_verifier": codeVerifier,
		"redirect_uri":  redirectURI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode kiro oauth token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kiroOAuthTokenEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create kiro oauth token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", kiroOAuthDefaultUserAgent)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	transport, err := buildKiroOAuthTransport(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("kiro oauth transport: %w", err)
	}
	client.Transport = transport

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kiro oauth/token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read kiro oauth token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, buildKiroOAuthTokenExchangeError(resp.StatusCode, bodyBytes)
	}

	var raw any
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode kiro oauth token response: %w", err)
	}

	payload := unwrapKiroOAuthResponse(raw)
	obj, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid kiro oauth token response shape")
	}
	return obj, nil
}

func registerKiroIDCClient(ctx context.Context, input kiroIDCRegisterClientInput) (*kiroIDCRegisterClientResult, error) {
	payload := map[string]any{
		"clientName": input.ClientName,
		"clientType": "public",
		"grantTypes": input.GrantTypes,
	}
	if len(input.RedirectURIs) > 0 {
		payload["redirectUris"] = input.RedirectURIs
	}
	if len(input.Scopes) > 0 {
		payload["scopes"] = input.Scopes
	}
	if strings.TrimSpace(input.IssuerURL) != "" {
		payload["issuerUrl"] = strings.TrimSpace(input.IssuerURL)
	}
	endpoint := fmt.Sprintf("https://oidc.%s.amazonaws.com/client/register", firstNonEmptyKiroString(input.Region, kiroIDCDefaultRegion))
	var out struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	}
	if err := doKiroIDCJSONRequest(ctx, input.ProxyURL, endpoint, payload, &out, "kiro idc register client"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.ClientID) == "" || strings.TrimSpace(out.ClientSecret) == "" {
		return nil, fmt.Errorf("kiro idc register client returned empty client credentials")
	}
	return &kiroIDCRegisterClientResult{
		ClientID:     strings.TrimSpace(out.ClientID),
		ClientSecret: strings.TrimSpace(out.ClientSecret),
	}, nil
}

func startKiroIDCDeviceAuthorization(ctx context.Context, input kiroIDCStartDeviceAuthorizationInput) (*kiroIDCStartDeviceAuthorizationResult, error) {
	payload := map[string]any{
		"clientId":     input.ClientID,
		"clientSecret": input.ClientSecret,
		"startUrl":     input.StartURL,
	}
	endpoint := fmt.Sprintf("https://oidc.%s.amazonaws.com/device_authorization", firstNonEmptyKiroString(input.Region, kiroIDCDefaultRegion))
	var out struct {
		DeviceCode              string `json:"deviceCode"`
		UserCode                string `json:"userCode"`
		VerificationURI         string `json:"verificationUri"`
		VerificationURIComplete string `json:"verificationUriComplete"`
		ExpiresIn               int64  `json:"expiresIn"`
		Interval                int64  `json:"interval"`
	}
	if err := doKiroIDCJSONRequest(ctx, input.ProxyURL, endpoint, payload, &out, "kiro idc start device authorization"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.DeviceCode) == "" {
		return nil, fmt.Errorf("kiro idc start device authorization returned empty device code")
	}
	return &kiroIDCStartDeviceAuthorizationResult{
		DeviceCode:              strings.TrimSpace(out.DeviceCode),
		UserCode:                strings.TrimSpace(out.UserCode),
		VerificationURI:         strings.TrimSpace(out.VerificationURI),
		VerificationURIComplete: strings.TrimSpace(out.VerificationURIComplete),
		ExpiresIn:               out.ExpiresIn,
		Interval:                out.Interval,
	}, nil
}

func completeKiroIDCDeviceAuthorization(ctx context.Context, input kiroIDCCompleteDeviceAuthorizationInput) (map[string]any, error) {
	payload := map[string]any{
		"clientId":     input.ClientID,
		"clientSecret": input.ClientSecret,
		"grantType":    kiroIDCDeviceGrantType,
		"deviceCode":   input.DeviceCode,
	}
	endpoint := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", firstNonEmptyKiroString(input.Region, kiroIDCDefaultRegion))
	var out map[string]any
	if err := doKiroIDCJSONRequest(ctx, input.ProxyURL, endpoint, payload, &out, "kiro idc create token"); err != nil {
		return nil, err
	}
	return out, nil
}

func doKiroIDCJSONRequest(ctx context.Context, proxyURL string, endpoint string, payload any, out any, operation string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", kiroOAuthDefaultUserAgent)

	client := &http.Client{Timeout: 60 * time.Second}
	transport, err := buildKiroOAuthTransport(proxyURL)
	if err != nil {
		return fmt.Errorf("%s: kiro oauth transport: %w", operation, err)
	}
	client.Transport = transport

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: failed to read response: %w", operation, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return buildKiroIDCAPIError(operation, resp.StatusCode, bodyBytes)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return fmt.Errorf("%s: failed to decode response: %w", operation, err)
	}
	return nil
}

func buildKiroIDCAPIError(operation string, statusCode int, body []byte) error {
	errCode := ""
	errDesc := strings.TrimSpace(string(body))
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		errCode = firstNonEmptyKiroString(
			pickKiroString(payload, []string{"error"}),
			pickKiroString(payload, []string{"Error", "Code"}),
		)
		errDesc = firstNonEmptyKiroString(
			pickKiroString(payload, []string{"error_description"}),
			pickKiroString(payload, []string{"message"}),
			pickKiroString(payload, []string{"Error", "Message"}),
			errDesc,
		)
	}
	return &kiroIDCAPIError{
		Operation:        strings.TrimSpace(operation),
		StatusCode:       statusCode,
		ErrorCode:        errCode,
		ErrorDescription: errDesc,
	}
}

func buildKiroOAuthTransport(proxyURL string) (*http.Transport, error) {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default HTTP transport has type %T, want *http.Transport", http.DefaultTransport)
	}
	transport := defaultTransport.Clone()
	transport.Proxy = nil
	if strings.TrimSpace(proxyURL) == "" {
		return transport, nil
	}
	parsed, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil {
		return nil, err
	}
	transport.Proxy = http.ProxyURL(parsed)
	transport.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	return transport, nil
}

func unwrapKiroOAuthResponse(raw any) any {
	root, ok := raw.(map[string]any)
	if !ok {
		return raw
	}
	if data, ok := root["data"]; ok {
		if obj, ok := data.(map[string]any); ok {
			return obj
		}
	}
	return root
}

func buildKiroTokenInfo(payload map[string]any, query url.Values, loginOption string) *KiroTokenInfo {
	accessToken := pickKiroString(payload, []string{"accessToken"}, []string{"access_token"}, []string{"token"}, []string{"idToken"}, []string{"id_token"}, []string{"accessTokenJwt"})
	refreshToken := pickKiroString(payload, []string{"refreshToken"}, []string{"refresh_token"}, []string{"refreshTokenJwt"})
	idToken := pickKiroString(payload, []string{"idToken"}, []string{"id_token"}, []string{"idTokenJwt"})
	accessClaims := decodeKiroJWTClaims(accessToken)
	idClaims := decodeKiroJWTClaims(idToken)

	expiresAt := normalizeKiroExpiresAt(payload)
	profileARN := firstNonEmptyKiroString(
		pickKiroString(payload, []string{"profileArn"}, []string{"profile_arn"}, []string{"arn"}),
	)
	authRegion := firstNonEmptyKiroString(
		pickKiroString(payload, []string{"idc_region"}, []string{"idcRegion"}, []string{"region"}),
	)
	apiRegion := profileARNRegion(profileARN)
	region := firstNonEmptyKiroString(apiRegion, authRegion, "us-east-1")
	profileID := firstNonEmptyKiroString(
		pickKiroString(payload, []string{"profileId"}, []string{"profile_id"}),
		profileARNProfileID(profileARN),
	)

	email := firstNonEmptyKiroString(
		pickKiroString(payload, []string{"email"}, []string{"userEmail"}),
		pickKiroStringFromMap(idClaims, []string{"email"}, []string{"upn"}, []string{"preferred_username"}),
		pickKiroStringFromMap(accessClaims, []string{"email"}, []string{"upn"}, []string{"preferred_username"}),
		strings.TrimSpace(query.Get("login_hint")),
	)

	name := firstNonEmptyKiroString(
		pickKiroString(payload, []string{"name"}, []string{"displayName"}, []string{"profileName"}),
	)
	if name == "" {
		name = email
	}

	userID := firstNonEmptyKiroString(
		pickKiroString(payload, []string{"userId"}, []string{"user_id"}, []string{"sub"}, []string{"accountId"}),
		pickKiroStringFromMap(idClaims, []string{"sub"}, []string{"user_id"}, []string{"uid"}),
		pickKiroStringFromMap(accessClaims, []string{"sub"}, []string{"user_id"}, []string{"uid"}),
		profileID,
	)

	loginProvider := normalizeKiroProvider(firstNonEmptyKiroString(
		pickKiroString(payload, []string{"provider"}, []string{"loginProvider"}, []string{"login_option"}),
		loginOption,
	))

	authMethod := deriveKiroAuthMethod(loginOption, payload)

	tokenInfo := &KiroTokenInfo{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		TokenType:        firstNonEmptyKiroString(pickKiroString(payload, []string{"tokenType"}, []string{"token_type"}, []string{"authType"}), "Bearer"),
		ExpiresAt:        expiresAt,
		AuthMethod:       authMethod,
		ClientID:         pickKiroString(payload, []string{"client_id"}, []string{"clientId"}, []string{"clientRegistration", "clientId"}, []string{"registration", "clientId"}, []string{"oidcClient", "clientId"}),
		ClientSecret:     pickKiroString(payload, []string{"client_secret"}, []string{"clientSecret"}, []string{"clientRegistration", "clientSecret"}, []string{"clientRegistration", "client_secret"}, []string{"registration", "clientSecret"}, []string{"oidcClient", "clientSecret"}),
		Region:           region,
		AuthRegion:       firstNonEmptyKiroString(authRegion, region),
		APIRegion:        firstNonEmptyKiroString(apiRegion, region),
		ProfileARN:       profileARN,
		ProfileID:        profileID,
		Email:            email,
		Name:             name,
		UserID:           userID,
		LoginProvider:    loginProvider,
		IssuerURL:        pickKiroString(payload, []string{"issuer_url"}, []string{"issuerUrl"}, []string{"issuer"}),
		IDCRegion:        authRegion,
		Scopes:           firstNonEmptyKiroString(pickKiroString(payload, []string{"scopes"}, []string{"scope"}), strings.TrimSpace(query.Get("scopes")), strings.TrimSpace(query.Get("scope"))),
		LoginHint:        firstNonEmptyKiroString(pickKiroString(payload, []string{"login_hint"}, []string{"loginHint"}), email),
		Status:           "normal",
		SubscriptionType: "",
	}

	return tokenInfo
}

func normalizeKiroExpiresAt(payload map[string]any) string {
	if ts := pickKiroTime(payload, []string{"expiresAt"}, []string{"expires_at"}, []string{"expiry"}, []string{"expiration"}); !ts.IsZero() {
		return ts.Format(time.RFC3339)
	}
	if seconds := pickKiroInt(payload, []string{"expiresIn"}, []string{"expires_in"}); seconds > 0 {
		return time.Now().Add(time.Duration(seconds) * time.Second).UTC().Format(time.RFC3339)
	}
	return ""
}

func deriveKiroAuthMethod(loginOption string, payload map[string]any) string {
	return NormalizeKiroAuthMethod(map[string]any{
		"auth_method":    pickKiroString(payload, []string{"authMethod"}, []string{"auth_method"}),
		"provider":       pickKiroString(payload, []string{"provider"}),
		"login_provider": pickKiroString(payload, []string{"loginProvider"}),
		"login_option":   firstNonEmptyKiroString(pickKiroString(payload, []string{"login_option"}), loginOption),
		"client_id":      pickKiroString(payload, []string{"client_id"}, []string{"clientId"}),
		"client_secret":  pickKiroString(payload, []string{"client_secret"}, []string{"clientSecret"}),
	})
}

func normalizeKiroProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "google":
		return "Google"
	case "github":
		return "Github"
	default:
		return strings.TrimSpace(value)
	}
}

func normalizeKiroScopes(explicit []string, query url.Values) []string {
	if len(explicit) > 0 {
		filtered := filterEmptyKiroStrings(explicit)
		if len(filtered) > 0 {
			return filtered
		}
	}
	scopeText := firstNonEmptyKiroString(query.Get("scopes"), query.Get("scope"))
	if scopeText != "" {
		parts := strings.FieldsFunc(scopeText, func(r rune) bool {
			return r == ' ' || r == ','
		})
		filtered := filterEmptyKiroStrings(parts)
		if len(filtered) > 0 {
			return filtered
		}
	}
	return append([]string(nil), kiroIDCDefaultScopes...)
}

func deriveKiroIDCRegionFromIssuerURL(issuerURL string) string {
	trimmed := strings.TrimSpace(issuerURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(parsed.Host)
	if !strings.HasPrefix(host, "oidc.") || !strings.HasSuffix(host, ".amazonaws.com") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(host, "oidc."), ".amazonaws.com")
}

func firstNonZeroKiroInt(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func filterEmptyKiroStrings(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return filtered
}

func profileARNRegion(profileARN string) string {
	parts := strings.Split(profileARN, ":")
	if len(parts) < 4 {
		return ""
	}
	return strings.TrimSpace(parts[3])
}

func profileARNProfileID(profileARN string) string {
	profileARN = strings.TrimSpace(profileARN)
	if profileARN == "" {
		return ""
	}
	if idx := strings.LastIndex(profileARN, "/"); idx >= 0 && idx+1 < len(profileARN) {
		return strings.TrimSpace(profileARN[idx+1:])
	}
	parts := strings.Split(profileARN, ":")
	if len(parts) > 0 {
		resource := strings.TrimSpace(parts[len(parts)-1])
		if idx := strings.LastIndex(resource, "/"); idx >= 0 && idx+1 < len(resource) {
			return strings.TrimSpace(resource[idx+1:])
		}
		return resource
	}
	return ""
}

func decodeKiroJWTClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil
	}
	return claims
}

func pickKiroString(root map[string]any, paths ...[]string) string {
	return pickKiroStringFromMap(root, paths...)
}

func pickKiroStringFromMap(root map[string]any, paths ...[]string) string {
	for _, path := range paths {
		if value, ok := walkKiroValue(root, path); ok {
			switch typed := value.(type) {
			case string:
				if trimmed := strings.TrimSpace(typed); trimmed != "" {
					return trimmed
				}
			case json.Number:
				return typed.String()
			case float64:
				if typed == float64(int64(typed)) {
					return fmt.Sprintf("%.0f", typed)
				}
				return fmt.Sprintf("%v", typed)
			}
		}
	}
	return ""
}

func pickKiroInt(root map[string]any, paths ...[]string) int64 {
	for _, path := range paths {
		if value, ok := walkKiroValue(root, path); ok {
			switch typed := value.(type) {
			case int64:
				return typed
			case int:
				return int64(typed)
			case float64:
				return int64(typed)
			case json.Number:
				if parsed, err := typed.Int64(); err == nil {
					return parsed
				}
			case string:
				if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(typed)); err == nil {
					return parsed.Unix()
				}
				if parsed, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(typed)); err == nil {
					return parsed.Unix()
				}
			}
		}
	}
	return 0
}

func pickKiroTime(root map[string]any, paths ...[]string) time.Time {
	for _, path := range paths {
		if value, ok := walkKiroValue(root, path); ok {
			switch typed := value.(type) {
			case string:
				text := strings.TrimSpace(typed)
				if text == "" {
					continue
				}
				if parsed, err := time.Parse(time.RFC3339, text); err == nil {
					return parsed.UTC()
				}
			case float64:
				return normalizeKiroTimestamp(int64(typed))
			case int64:
				return normalizeKiroTimestamp(typed)
			case json.Number:
				if parsed, err := typed.Int64(); err == nil {
					return normalizeKiroTimestamp(parsed)
				}
			}
		}
	}
	return time.Time{}
}

func normalizeKiroTimestamp(raw int64) time.Time {
	if raw <= 0 {
		return time.Time{}
	}
	if raw > 10_000_000_000 {
		raw /= 1000
	}
	return time.Unix(raw, 0).UTC()
}

func walkKiroValue(root map[string]any, path []string) (any, bool) {
	if len(path) == 0 {
		return nil, false
	}
	var current any = root
	for _, segment := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = next[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func firstNonEmptyKiroString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
