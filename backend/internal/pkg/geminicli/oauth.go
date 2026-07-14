package geminicli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/redissession"
	"github.com/redis/go-redis/v9"
)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       string
}

type OAuthSession struct {
	State         string `json:"state"`
	CodeVerifier  string `json:"code_verifier"`
	ProxyURL      string `json:"proxy_url,omitempty"`
	RedirectURI   string `json:"redirect_uri"`
	ProjectIDHint string `json:"project_id_hint,omitempty"`
	// TierID is a user-selected fallback tier.
	// For oauth types that support auto detection (google_one/code_assist), the server will prefer
	// the detected tier and fall back to TierID when detection fails.
	TierID    string    `json:"tier_id,omitempty"`
	OAuthType string    `json:"oauth_type"` // "code_assist" 或 "ai_studio"
	CreatedAt time.Time `json:"created_at"`
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*OAuthSession
	// localOnly prevents a remote miss from reviving a stale local session.
	localOnly map[string]struct{}
	stopCh    chan struct{}
	stopOnce  sync.Once
	remote    *redissession.Store
}

func NewRedisSessionStore(rdb *redis.Client) *SessionStore {
	store := NewSessionStore()
	if rdb != nil {
		store.remote = redissession.New(rdb, "oauth:session:gemini", SessionTTL)
	}
	return store
}

func NewSessionStore() *SessionStore {
	store := &SessionStore{
		sessions:  make(map[string]*OAuthSession),
		localOnly: make(map[string]struct{}),
		stopCh:    make(chan struct{}),
	}
	go store.cleanup()
	return store
}

func (s *SessionStore) Set(sessionID string, session *OAuthSession) error {
	if session == nil {
		return nil
	}
	var remoteErr error
	if s != nil && s.remote != nil {
		remoteErr = s.remote.Set(context.Background(), sessionID, session)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
	if remoteErr != nil {
		s.localOnly[sessionID] = struct{}{}
		log.Printf("gemini oauth session Redis write failed; using process-local fallback: %v", remoteErr)
	} else {
		delete(s.localOnly, sessionID)
	}
	return remoteErr
}

func (s *SessionStore) Get(sessionID string) (*OAuthSession, bool) {
	if s.isLocalOnly(sessionID) {
		return s.getMemory(sessionID)
	}
	if s != nil && s.remote != nil {
		var session OAuthSession
		ok, err := s.remote.Get(context.Background(), sessionID, &session)
		if err != nil || !ok {
			return nil, false
		}
		if time.Since(session.CreatedAt) > SessionTTL {
			_ = s.remote.Delete(context.Background(), sessionID)
			return nil, false
		}
		s.mu.Lock()
		s.sessions[sessionID] = &session
		s.mu.Unlock()
		return &session, true
	}
	return s.getMemory(sessionID)
}

func (s *SessionStore) getMemory(sessionID string) (*OAuthSession, bool) {
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
	delete(s.localOnly, sessionID)
}

func (s *SessionStore) TryConsumeSession(sessionID string) bool {
	if s == nil {
		return false
	}
	if s.isLocalOnly(sessionID) {
		return s.tryConsumeMemory(sessionID)
	}
	if s.remote != nil {
		ok, err := s.remote.TryConsume(context.Background(), sessionID)
		return err == nil && ok
	}
	return s.tryConsumeMemory(sessionID)
}

func (s *SessionStore) isLocalOnly(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.localOnly[sessionID]
	return ok
}

func (s *SessionStore) tryConsumeMemory(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return false
	}
	if time.Since(session.CreatedAt) > SessionTTL {
		delete(s.sessions, sessionID)
		delete(s.localOnly, sessionID)
		return false
	}
	delete(s.sessions, sessionID)
	return true
}

func (s *SessionStore) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

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
					delete(s.localOnly, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func GenerateState() (string, error) {
	bytes, err := GenerateRandomBytes(32)
	if err != nil {
		return "", err
	}
	return base64URLEncode(bytes), nil
}

func GenerateSessionID() (string, error) {
	bytes, err := GenerateRandomBytes(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateCodeVerifier returns an RFC 7636 compatible code verifier (43+ chars).
func GenerateCodeVerifier() (string, error) {
	bytes, err := GenerateRandomBytes(32)
	if err != nil {
		return "", err
	}
	return base64URLEncode(bytes), nil
}

func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64URLEncode(hash[:])
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

// EffectiveOAuthConfig returns the effective OAuth configuration.
// oauthType: "code_assist" or "ai_studio" (defaults to "code_assist" if empty).
//
// If ClientID/ClientSecret is not provided, this falls back to the built-in Gemini CLI OAuth client.
//
// Note: The built-in Gemini CLI OAuth client is restricted and may reject some scopes (e.g.
// https://www.googleapis.com/auth/generative-language), which will surface as
// "restricted_client" / "Unregistered scope(s)" errors during browser authorization.
func EffectiveOAuthConfig(cfg OAuthConfig, oauthType string) (OAuthConfig, error) {
	effective := OAuthConfig{
		ClientID:     strings.TrimSpace(cfg.ClientID),
		ClientSecret: strings.TrimSpace(cfg.ClientSecret),
		Scopes:       strings.TrimSpace(cfg.Scopes),
	}

	// Match official Gemini CLI behavior for personal Google login:
	// always use the built-in Gemini CLI OAuth client and Code Assist scopes.
	if oauthType == "google_one" {
		effective.ClientID = ""
		effective.ClientSecret = ""
		effective.Scopes = ""
	}

	// Normalize scopes: allow comma-separated input but send space-delimited scopes to Google.
	if effective.Scopes != "" {
		effective.Scopes = strings.Join(strings.Fields(strings.ReplaceAll(effective.Scopes, ",", " ")), " ")
	}

	// Fall back to built-in Gemini CLI OAuth client when not configured.
	// SECURITY: This repo does not embed the built-in client secret; it must be provided via env.
	if effective.ClientID == "" && effective.ClientSecret == "" {
		secret := strings.TrimSpace(GeminiCLIOAuthClientSecret)
		if secret == "" {
			if v, ok := os.LookupEnv(GeminiCLIOAuthClientSecretEnv); ok {
				secret = strings.TrimSpace(v)
			}
		}
		if secret == "" {
			return OAuthConfig{}, infraerrors.Newf(http.StatusBadRequest, "GEMINI_CLI_OAUTH_CLIENT_SECRET_MISSING", "built-in Gemini CLI OAuth client_secret is not configured; set %s or provide a custom OAuth client", GeminiCLIOAuthClientSecretEnv)
		}
		effective.ClientID = GeminiCLIOAuthClientID
		effective.ClientSecret = secret
	} else if effective.ClientID == "" || effective.ClientSecret == "" {
		return OAuthConfig{}, infraerrors.New(http.StatusBadRequest, "GEMINI_OAUTH_CLIENT_NOT_CONFIGURED", "OAuth client not configured: please set both client_id and client_secret (or leave both empty to use the built-in Gemini CLI client)")
	}

	isBuiltinClient := effective.ClientID == GeminiCLIOAuthClientID

	if effective.Scopes == "" {
		// Use different default scopes based on OAuth type
		switch oauthType {
		case "ai_studio":
			// Built-in client can't request some AI Studio scopes (notably generative-language).
			if isBuiltinClient {
				effective.Scopes = DefaultCodeAssistScopes
			} else {
				effective.Scopes = DefaultAIStudioScopes
			}
		case "google_one":
			// Google One matches official Gemini CLI personal Google login:
			// built-in OAuth client + Code Assist scopes.
			effective.Scopes = DefaultCodeAssistScopes
		default:
			// Default to Code Assist scopes
			effective.Scopes = DefaultCodeAssistScopes
		}
	} else if (oauthType == "ai_studio" || oauthType == "google_one") && isBuiltinClient {
		// If user overrides scopes while still using the built-in client, strip restricted scopes.
		parts := strings.Fields(effective.Scopes)
		filtered := make([]string, 0, len(parts))
		for _, s := range parts {
			if hasRestrictedScope(s) {
				continue
			}
			filtered = append(filtered, s)
		}
		if len(filtered) == 0 {
			effective.Scopes = DefaultCodeAssistScopes
		} else {
			effective.Scopes = strings.Join(filtered, " ")
		}
	}

	// Backward compatibility: normalize older AI Studio scope to the currently documented one.
	if oauthType == "ai_studio" && effective.Scopes != "" {
		parts := strings.Fields(effective.Scopes)
		for i := range parts {
			if parts[i] == "https://www.googleapis.com/auth/generative-language" {
				parts[i] = "https://www.googleapis.com/auth/generative-language.retriever"
			}
		}
		effective.Scopes = strings.Join(parts, " ")
	}

	return effective, nil
}

func hasRestrictedScope(scope string) bool {
	return strings.HasPrefix(scope, "https://www.googleapis.com/auth/generative-language") ||
		strings.HasPrefix(scope, "https://www.googleapis.com/auth/drive")
}

func BuildAuthorizationURL(cfg OAuthConfig, state, codeChallenge, redirectURI, projectID, oauthType string) (string, error) {
	effectiveCfg, err := EffectiveOAuthConfig(cfg, oauthType)
	if err != nil {
		return "", err
	}
	redirectURI = strings.TrimSpace(redirectURI)
	if redirectURI == "" {
		return "", fmt.Errorf("redirect_uri is required")
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", effectiveCfg.ClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", effectiveCfg.Scopes)
	params.Set("state", state)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("include_granted_scopes", "true")
	if oauthType == "code_assist" && strings.TrimSpace(projectID) != "" {
		params.Set("project_id", strings.TrimSpace(projectID))
	}

	return fmt.Sprintf("%s?%s", AuthorizeURL, params.Encode()), nil
}
