package openai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/redissession"
	"github.com/redis/go-redis/v9"
)

// OpenAI OAuth Constants (from CRS project - Codex CLI client)
const (
	// OAuth Client ID for OpenAI (Codex CLI official)
	ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

	// OAuth endpoints
	AuthorizeURL = "https://auth.openai.com/oauth/authorize"
	TokenURL     = "https://auth.openai.com/oauth/token"

	// Default redirect URI (can be customized)
	DefaultRedirectURI = "http://localhost:1455/auth/callback"

	// Scopes
	DefaultScopes = "openid profile email offline_access"
	// RefreshScopes - scope for token refresh (without offline_access, aligned with CRS project)
	RefreshScopes = "openid profile email"

	// Session TTL
	SessionTTL = 30 * time.Minute
)

const (
	// OAuthPlatformOpenAI uses OpenAI Codex-compatible OAuth client.
	OAuthPlatformOpenAI = "openai"
)

// OAuthSession stores OAuth flow state for OpenAI
type OAuthSession struct {
	State        string    `json:"state"`
	CodeVerifier string    `json:"code_verifier"`
	ClientID     string    `json:"client_id,omitempty"`
	ProxyURL     string    `json:"proxy_url,omitempty"`
	RedirectURI  string    `json:"redirect_uri"`
	CreatedAt    time.Time `json:"created_at"`
}

// SessionStore manages OAuth sessions (memory + optional Redis for multi-instance).
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*OAuthSession
	stopOnce sync.Once
	stopCh   chan struct{}
	remote   *redissession.Store
}

type oauthSessionDTO struct {
	State        string    `json:"state"`
	CodeVerifier string    `json:"code_verifier"`
	ClientID     string    `json:"client_id,omitempty"`
	ProxyURL     string    `json:"proxy_url,omitempty"`
	RedirectURI  string    `json:"redirect_uri"`
	CreatedAt    time.Time `json:"created_at"`
}

// NewSessionStore creates a new session store
func NewSessionStore() *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*OAuthSession),
		stopCh:   make(chan struct{}),
	}
	go store.cleanup()
	return store
}

// NewRedisSessionStore returns a multi-instance session store backed by Redis.
func NewRedisSessionStore(rdb *redis.Client) *SessionStore {
	store := NewSessionStore()
	if rdb != nil {
		store.remote = redissession.New(rdb, "oauth:session:openai", SessionTTL)
	}
	return store
}

func (s *SessionStore) Set(sessionID string, session *OAuthSession) {
	if session == nil {
		return
	}
	if s != nil && s.remote != nil {
		_ = s.remote.Set(context.Background(), sessionID, oauthSessionDTO{
			State: session.State,
			CodeVerifier: session.CodeVerifier,
			ClientID: session.ClientID,
			ProxyURL: session.ProxyURL,
			RedirectURI: session.RedirectURI,
			CreatedAt: session.CreatedAt,
		})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
}

func (s *SessionStore) Get(sessionID string) (*OAuthSession, bool) {
	if s != nil && s.remote != nil {
		var dto oauthSessionDTO
		ok, err := s.remote.Get(context.Background(), sessionID, &dto)
		if err != nil || !ok {
			return nil, false
		}
		if time.Since(dto.CreatedAt) > SessionTTL {
			_ = s.remote.Delete(context.Background(), sessionID)
			return nil, false
		}
		session := &OAuthSession{
			State: dto.State,
			CodeVerifier: dto.CodeVerifier,
			ClientID: dto.ClientID,
			ProxyURL: dto.ProxyURL,
			RedirectURI: dto.RedirectURI,
			CreatedAt: dto.CreatedAt,
		}
		s.mu.Lock()
		s.sessions[sessionID] = session
		s.mu.Unlock()
		return session, true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if time.Since(session.CreatedAt) > SessionTTL {
		return nil, false
	}
	return session, true
}

func (s *SessionStore) Delete(sessionID string) {
	if s != nil && s.remote != nil {
		_ = s.remote.Delete(context.Background(), sessionID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

// TryConsumeSession marks the session used once (Redis SET NX when configured).
func (s *SessionStore) TryConsumeSession(sessionID string) bool {
	if s == nil {
		return false
	}
	if s.remote != nil {
		ok, err := s.remote.TryConsume(context.Background(), sessionID)
		return err == nil && ok
	}
	// Memory-only stores without per-session consume: treat delete as consume no-op success.
	_, ok := s.Get(sessionID)
	return ok
}


// Stop stops the cleanup goroutine
func (s *SessionStore) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

// cleanup removes expired sessions periodically
func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, session := range s.sessions {
				if time.Since(session.CreatedAt) > SessionTTL {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

// GenerateRandomBytes generates cryptographically secure random bytes
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateState generates a random state string for OAuth
func GenerateState() (string, error) {
	bytes, err := GenerateRandomBytes(32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateSessionID generates a unique session ID
func GenerateSessionID() (string, error) {
	bytes, err := GenerateRandomBytes(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateCodeVerifier generates a PKCE code verifier (64 bytes -> hex for OpenAI)
// OpenAI uses hex encoding instead of base64url
func GenerateCodeVerifier() (string, error) {
	bytes, err := GenerateRandomBytes(64)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateCodeChallenge generates a PKCE code challenge using S256 method
// Uses base64url encoding as per RFC 7636
func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64URLEncode(hash[:])
}

// base64URLEncode encodes bytes to base64url without padding
func base64URLEncode(data []byte) string {
	encoded := base64.URLEncoding.EncodeToString(data)
	// Remove padding
	return strings.TrimRight(encoded, "=")
}

// BuildAuthorizationURL builds the OpenAI OAuth authorization URL
func BuildAuthorizationURL(state, codeChallenge, redirectURI string) string {
	return BuildAuthorizationURLForPlatform(state, codeChallenge, redirectURI, OAuthPlatformOpenAI)
}

// BuildAuthorizationURLForPlatform builds authorization URL by platform.
func BuildAuthorizationURLForPlatform(state, codeChallenge, redirectURI, platform string) string {
	if redirectURI == "" {
		redirectURI = DefaultRedirectURI
	}

	clientID, codexFlow := OAuthClientConfigByPlatform(platform)

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", DefaultScopes)
	params.Set("state", state)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	// OpenAI specific parameters
	params.Set("id_token_add_organizations", "true")
	if codexFlow {
		params.Set("codex_cli_simplified_flow", "true")
	}

	return fmt.Sprintf("%s?%s", AuthorizeURL, params.Encode())
}

// OAuthClientConfigByPlatform returns oauth client_id and whether codex simplified flow should be enabled.
func OAuthClientConfigByPlatform(platform string) (clientID string, codexFlow bool) {
	return ClientID, true
}

// TokenRequest represents the token exchange request body
type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
}

// TokenResponse represents the token response from OpenAI OAuth
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// RefreshTokenRequest represents the refresh token request
type RefreshTokenRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
	Scope        string `json:"scope"`
}

// IDTokenClaims represents the claims from OpenAI ID Token
type IDTokenClaims struct {
	// Standard claims
	Sub           string   `json:"sub"`
	Email         string   `json:"email"`
	Name          string   `json:"name"`
	EmailVerified bool     `json:"email_verified"`
	Iss           string   `json:"iss"`
	Aud           []string `json:"aud"` // OpenAI returns aud as an array
	Exp           int64    `json:"exp"`
	Iat           int64    `json:"iat"`

	// OpenAI specific claims (nested under https://api.openai.com/auth)
	OpenAIAuth *OpenAIAuthClaims `json:"https://api.openai.com/auth,omitempty"`
}

// OpenAIAuthClaims represents the OpenAI specific auth claims
type OpenAIAuthClaims struct {
	ChatGPTAccountID string              `json:"chatgpt_account_id"`
	ChatGPTUserID    string              `json:"chatgpt_user_id"`
	ChatGPTPlanType  string              `json:"chatgpt_plan_type"`
	UserID           string              `json:"user_id"`
	POID             string              `json:"poid"` // organization ID in access_token JWT
	Organizations    []OrganizationClaim `json:"organizations"`
}

// OrganizationClaim represents an organization in the ID Token
type OrganizationClaim struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Title     string `json:"title"`
	IsDefault bool   `json:"is_default"`
}

// BuildTokenRequest creates a token exchange request for OpenAI
func BuildTokenRequest(code, codeVerifier, redirectURI string) *TokenRequest {
	if redirectURI == "" {
		redirectURI = DefaultRedirectURI
	}
	return &TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     ClientID,
		Code:         code,
		RedirectURI:  redirectURI,
		CodeVerifier: codeVerifier,
	}
}

// BuildRefreshTokenRequest creates a refresh token request for OpenAI
func BuildRefreshTokenRequest(refreshToken string) *RefreshTokenRequest {
	return &RefreshTokenRequest{
		GrantType:    "refresh_token",
		RefreshToken: refreshToken,
		ClientID:     ClientID,
		Scope:        RefreshScopes,
	}
}

// ToFormData converts TokenRequest to URL-encoded form data
func (r *TokenRequest) ToFormData() string {
	params := url.Values{}
	params.Set("grant_type", r.GrantType)
	params.Set("client_id", r.ClientID)
	params.Set("code", r.Code)
	params.Set("redirect_uri", r.RedirectURI)
	params.Set("code_verifier", r.CodeVerifier)
	return params.Encode()
}

// ToFormData converts RefreshTokenRequest to URL-encoded form data
func (r *RefreshTokenRequest) ToFormData() string {
	params := url.Values{}
	params.Set("grant_type", r.GrantType)
	params.Set("client_id", r.ClientID)
	params.Set("refresh_token", r.RefreshToken)
	params.Set("scope", r.Scope)
	return params.Encode()
}

// DecodeIDToken decodes the ID Token JWT payload without validating expiration.
// Use this for best-effort extraction (e.g., during data import) where the token may be expired.
func DecodeIDToken(idToken string) (*IDTokenClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	// Decode payload (second part)
	payload := parts[1]
	// Add padding if necessary
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		// Try standard encoding
		decoded, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
		}
	}

	var claims IDTokenClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return &claims, nil
}

// ParseIDToken parses the ID Token JWT and extracts claims.
// 注意：当前仅解码 payload 并校验 exp，未验证 JWT 签名。
// 生产环境如需用 ID Token 做授权决策，应通过 OpenAI 的 JWKS 端点验证签名：
//
//	https://auth.openai.com/.well-known/jwks.json
func ParseIDToken(idToken string) (*IDTokenClaims, error) {
	claims, err := DecodeIDToken(idToken)
	if err != nil {
		return nil, err
	}

	// 校验 ID Token 是否已过期（允许 2 分钟时钟偏差，防止因服务器时钟略有差异误判刚颁发的令牌）
	const clockSkewTolerance = 120 // 秒
	now := time.Now().Unix()
	if claims.Exp > 0 && now > claims.Exp+clockSkewTolerance {
		return nil, fmt.Errorf("id_token has expired (exp: %d, now: %d, skew_tolerance: %ds)", claims.Exp, now, clockSkewTolerance)
	}

	return claims, nil
}

// UserInfo represents user information extracted from ID Token claims.
type UserInfo struct {
	Name              string
	Email             string
	ChatGPTAccountID  string
	ChatGPTUserID     string
	PlanType          string
	UserID            string
	OrganizationID    string
	OrganizationTitle string
	OrganizationRole  string
	Organizations     []OrganizationClaim
}

// GetUserInfo extracts user info from ID Token claims
func (c *IDTokenClaims) GetUserInfo() *UserInfo {
	info := &UserInfo{
		Name:  c.Name,
		Email: c.Email,
	}

	if c.OpenAIAuth != nil {
		info.ChatGPTAccountID = c.OpenAIAuth.ChatGPTAccountID
		info.ChatGPTUserID = c.OpenAIAuth.ChatGPTUserID
		info.PlanType = c.OpenAIAuth.ChatGPTPlanType
		info.UserID = c.OpenAIAuth.UserID
		info.Organizations = c.OpenAIAuth.Organizations

		// Prefer the exact workspace from access_token `poid` when present.
		// For team plans without a usable `poid`, fall back to the default org
		// before using role/title heuristics.
		var defaultOrg *OrganizationClaim
		var poidOrg *OrganizationClaim
		poid := strings.TrimSpace(c.OpenAIAuth.POID)
		for i := range c.OpenAIAuth.Organizations {
			org := &c.OpenAIAuth.Organizations[i]
			if defaultOrg == nil && org.IsDefault {
				defaultOrg = org
			}
			if poid != "" && org.ID == poid {
				poidOrg = org
			}
		}

		var chosen *OrganizationClaim
		if poidOrg != nil {
			chosen = poidOrg
		}
		if chosen == nil && info.PlanType == "team" && len(c.OpenAIAuth.Organizations) > 1 {
			if defaultOrg != nil && defaultOrg.Role != "" && defaultOrg.Role != "owner" {
				chosen = defaultOrg
			}
			if chosen == nil && defaultOrg != nil && defaultOrg.Title != "Personal" {
				chosen = defaultOrg
			}
			// Try non-"owner" role org first (clearly the team workspace for members)
			if chosen == nil {
				for i := range c.OpenAIAuth.Organizations {
					org := &c.OpenAIAuth.Organizations[i]
					if org.Role != "" && org.Role != "owner" {
						chosen = org
						break
					}
				}
			}
			// For team owners: both orgs may be "owner", pick non-"Personal" title.
			if chosen == nil {
				for i := range c.OpenAIAuth.Organizations {
					org := &c.OpenAIAuth.Organizations[i]
					if org.Title != "Personal" {
						chosen = org
						break
					}
				}
			}
		}
		if chosen == nil {
			chosen = defaultOrg
		}
		if chosen == nil && len(c.OpenAIAuth.Organizations) > 0 {
			chosen = &c.OpenAIAuth.Organizations[0]
		}
		if chosen != nil {
			info.OrganizationID = chosen.ID
			info.OrganizationTitle = chosen.Title
			info.OrganizationRole = chosen.Role
		}
	}

	return info
}
