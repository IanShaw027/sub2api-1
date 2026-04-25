package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	kiroOAuthDefaultCallback  = "http://localhost:1455"
	kiroOAuthSessionTTL       = 30 * time.Minute
	kiroOAuthDefaultUserAgent = "sub2api Kiro OAuth"
)

var kiroCodeExchangeFunc = exchangeKiroCodeForToken

type KiroOAuthSession struct {
	State           string
	CodeVerifier    string
	CallbackBaseURL string
	ProxyURL        string
	CreatedAt       time.Time
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
) *KiroOAuthService {
	return &KiroOAuthService{
		sessionStore: NewKiroOAuthSessionStore(),
		proxyRepo:    proxyRepo,
		usageService: NewKiroUsageService().WithTransport(httpUpstream, tlsFPProfileService),
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

func (s *KiroOAuthService) GenerateAuthURL(ctx context.Context, proxyID *int64) (*KiroAuthURLResult, error) {
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
		proxy, err := s.proxyRepo.GetByID(ctx, *proxyID)
		if err == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	callbackBaseURL := kiroOAuthDefaultCallback
	s.sessionStore.Set(sessionID, &KiroOAuthSession{
		State:           state,
		CodeVerifier:    codeVerifier,
		CallbackBaseURL: callbackBaseURL,
		ProxyURL:        proxyURL,
		CreatedAt:       time.Now(),
	})

	params := url.Values{}
	params.Set("state", state)
	params.Set("code_challenge", generateKiroCodeChallenge(codeVerifier))
	params.Set("code_challenge_method", "S256")
	params.Set("redirect_uri", callbackBaseURL)
	params.Set("redirect_from", "KiroIDE")

	return &KiroAuthURLResult{
		AuthURL:     kiroOAuthPortalURL + "?" + params.Encode(),
		SessionID:   sessionID,
		CallbackURL: callbackBaseURL,
	}, nil
}

func (s *KiroOAuthService) ExchangeCallback(ctx context.Context, input *KiroExchangeCallbackInput) (*KiroTokenInfo, error) {
	session, ok := s.sessionStore.Get(input.SessionID)
	if !ok {
		return nil, fmt.Errorf("kiro oauth session not found or expired")
	}

	parsedURL, err := parseKiroCallbackURL(input.CallbackURL, session.CallbackBaseURL)
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
		return nil, fmt.Errorf("invalid kiro oauth state")
	}

	loginOption := strings.ToLower(strings.TrimSpace(firstNonEmptyKiroString(
		query.Get("login_option"),
		query.Get("loginOption"),
	)))
	code := strings.TrimSpace(query.Get("code"))
	if code == "" {
		switch loginOption {
		case "builderid", "awsidc", "internal":
			return nil, fmt.Errorf("the current kiro login mode requires additional IDC material and cannot be imported automatically")
		case "external_idp":
			return nil, fmt.Errorf("the current kiro external IdP callback does not provide an authorization code")
		default:
			return nil, fmt.Errorf("kiro callback is missing authorization code")
		}
	}

	proxyURL := session.ProxyURL
	if input.ProxyID != nil {
		proxy, err := s.proxyRepo.GetByID(ctx, *input.ProxyID)
		if err == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	tokenPayload, err := kiroCodeExchangeFunc(ctx, code, session.CodeVerifier, session.CallbackBaseURL, proxyURL)
	if err != nil {
		return nil, err
	}
	s.sessionStore.Delete(input.SessionID)

	tokenInfo := buildKiroTokenInfo(tokenPayload, query, loginOption)
	s.enrichTokenInfo(ctx, tokenInfo)
	return tokenInfo, nil
}

func (s *KiroOAuthService) enrichTokenInfo(ctx context.Context, tokenInfo *KiroTokenInfo) {
	if tokenInfo == nil || tokenInfo.AccessToken == "" || tokenInfo.ProfileARN == "" || s.usageService == nil {
		return
	}

	account := &Account{
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  tokenInfo.AccessToken,
			"refresh_token": tokenInfo.RefreshToken,
			"profile_arn":   tokenInfo.ProfileARN,
			"region":        firstNonEmptyKiroString(tokenInfo.Region, "us-east-1"),
			"auth_region":   tokenInfo.AuthRegion,
			"api_region":    tokenInfo.APIRegion,
		},
		Extra: map[string]any{
			"kiro_version":   "0.10.0",
			"system_version": "darwin#24.6.0",
			"node_version":   "22.21.1",
		},
		Concurrency: 1,
	}

	usage, err := s.usageService.FetchUsageLimits(ctx, account, tokenInfo.AccessToken)
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

func (s *KiroOAuthService) Stop() {
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

	parsed, err := url.Parse(base + "/oauth/callback?" + strings.TrimLeft(trimmed, "?"))
	if err != nil {
		return nil, fmt.Errorf("invalid kiro callback URL: %w", err)
	}
	return parsed, nil
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
	if transport, err := buildKiroOAuthTransport(proxyURL); err == nil {
		client.Transport = transport
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kiro oauth/token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var raw any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode kiro oauth token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("kiro oauth/token upstream returned %d", resp.StatusCode)
	}

	payload := unwrapKiroOAuthResponse(raw)
	obj, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid kiro oauth token response shape")
	}
	return obj, nil
}

func buildKiroOAuthTransport(proxyURL string) (*http.Transport, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
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
		profileARN,
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
	raw := strings.ToLower(strings.TrimSpace(firstNonEmptyKiroString(
		pickKiroString(payload, []string{"authMethod"}, []string{"auth_method"}),
		pickKiroString(payload, []string{"provider"}, []string{"loginProvider"}, []string{"login_option"}),
		loginOption,
	)))
	switch raw {
	case "idc", "builderid", "awsidc", "internal", "enterprise", "external_idp":
		return "idc"
	}
	if pickKiroString(payload, []string{"client_secret"}, []string{"clientSecret"}) != "" &&
		pickKiroString(payload, []string{"client_id"}, []string{"clientId"}) != "" {
		return "idc"
	}
	return "social"
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

func profileARNRegion(profileARN string) string {
	parts := strings.Split(profileARN, ":")
	if len(parts) < 4 {
		return ""
	}
	return strings.TrimSpace(parts[3])
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
