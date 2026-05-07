package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	// Canonical tier IDs used by sub2api (2026-aligned).
	GeminiTierGoogleOneFree    = "google_one_free"
	GeminiTierGoogleAIPro      = "google_ai_pro"
	GeminiTierGoogleAIUltra    = "google_ai_ultra"
	GeminiTierGCPStandard      = "gcp_standard"
	GeminiTierGCPEnterprise    = "gcp_enterprise"
	GeminiTierAIStudioFree     = "aistudio_free"
	GeminiTierAIStudioPaid     = "aistudio_paid"
	GeminiTierGoogleOneUnknown = "google_one_unknown"

	// Legacy/compat tier IDs that may exist in historical data or upstream responses.
	legacyTierAIPremium          = "AI_PREMIUM"
	legacyTierGoogleOneStandard  = "GOOGLE_ONE_STANDARD"
	legacyTierGoogleOneBasic     = "GOOGLE_ONE_BASIC"
	legacyTierFree               = "FREE"
	legacyTierGoogleOneUnknown   = "GOOGLE_ONE_UNKNOWN"
	legacyTierGoogleOneUnlimited = "GOOGLE_ONE_UNLIMITED"
)

const (
	googleOAuthUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

	GB = 1024 * 1024 * 1024
	TB = 1024 * GB

	StorageTierUnlimited = 100 * TB // 100TB
	StorageTierAIPremium = 2 * TB   // 2TB
	StorageTierStandard  = 200 * GB // 200GB
	StorageTierBasic     = 100 * GB // 100GB
	StorageTierFree      = 15 * GB  // 15GB
)

type GeminiOAuthService struct {
	sessionStore *geminicli.SessionStore
	proxyRepo    ProxyRepository
	oauthClient  GeminiOAuthClient
	codeAssist   GeminiCliCodeAssistClient
	driveClient  geminicli.DriveClient
	cfg          *config.Config
}

type GeminiOAuthCapabilities struct {
	AIStudioOAuthEnabled bool     `json:"ai_studio_oauth_enabled"`
	RequiredRedirectURIs []string `json:"required_redirect_uris"`
}

func NewGeminiOAuthService(
	proxyRepo ProxyRepository,
	oauthClient GeminiOAuthClient,
	codeAssist GeminiCliCodeAssistClient,
	driveClient geminicli.DriveClient,
	cfg *config.Config,
) *GeminiOAuthService {
	return &GeminiOAuthService{
		sessionStore: geminicli.NewSessionStore(),
		proxyRepo:    proxyRepo,
		oauthClient:  oauthClient,
		codeAssist:   codeAssist,
		driveClient:  driveClient,
		cfg:          cfg,
	}
}

func (s *GeminiOAuthService) GetOAuthConfig() *GeminiOAuthCapabilities {
	return &GeminiOAuthCapabilities{
		AIStudioOAuthEnabled: false,
		RequiredRedirectURIs: []string{geminicli.AIStudioOAuthRedirectURI},
	}
}

type GeminiAuthURLResult struct {
	AuthURL   string `json:"auth_url"`
	SessionID string `json:"session_id"`
	State     string `json:"state"`
}

func (s *GeminiOAuthService) GenerateAuthURL(ctx context.Context, proxyID *int64, redirectURI, projectID, oauthType, tierID string) (*GeminiAuthURLResult, error) {
	oauthType = resolveGeminiOAuthType(oauthType, tierID, projectID, "")
	if oauthType == "" {
		return nil, fmt.Errorf("missing oauth_type: must be 'code_assist' or 'google_one'")
	}

	state, err := geminicli.GenerateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	codeVerifier, err := geminicli.GenerateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeChallenge := geminicli.GenerateCodeChallenge(codeVerifier)
	sessionID, err := geminicli.GenerateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	var proxyURL string
	if proxyID != nil {
		if s.proxyRepo == nil {
			return nil, fmt.Errorf("proxy repository is unavailable")
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

	oauthCfg := geminicli.OAuthConfig{
		ClientID:     s.cfg.Gemini.OAuth.ClientID,
		ClientSecret: s.cfg.Gemini.OAuth.ClientSecret,
		Scopes:       s.cfg.Gemini.OAuth.Scopes,
	}
	oauthCfg.ClientID = ""
	oauthCfg.ClientSecret = ""

	session := &geminicli.OAuthSession{
		State:        state,
		CodeVerifier: codeVerifier,
		ProxyURL:     proxyURL,
		RedirectURI:  redirectURI,
		ProjectID:    strings.TrimSpace(projectID),
		TierID:       canonicalGeminiTierIDForOAuthType(oauthType, tierID),
		OAuthType:    oauthType,
		CreatedAt:    time.Now(),
	}
	s.sessionStore.Set(sessionID, session)

	effectiveCfg, err := geminicli.EffectiveOAuthConfig(oauthCfg, oauthType)
	if err != nil {
		return nil, err
	}

	redirectURI = geminicli.GeminiCLIRedirectURI
	session.RedirectURI = redirectURI
	s.sessionStore.Set(sessionID, session)

	authURL, err := geminicli.BuildAuthorizationURL(effectiveCfg, state, codeChallenge, redirectURI, session.ProjectID, oauthType)
	if err != nil {
		return nil, err
	}

	return &GeminiAuthURLResult{
		AuthURL:   authURL,
		SessionID: sessionID,
		State:     state,
	}, nil
}

type GeminiExchangeCodeInput struct {
	SessionID string
	State     string
	Code      string
	ProxyID   *int64
	OAuthType string // "code_assist" or "google_one"
	// TierID is a user-selected tier to be used when auto detection is unavailable or fails.
	// If empty, the service will fall back to the tier stored in the OAuth session (if any).
	TierID string
}

type GeminiTokenInfo struct {
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	IDToken      string         `json:"id_token,omitempty"`
	ExpiresIn    int64          `json:"expires_in"`
	ExpiresAt    int64          `json:"expires_at"`
	TokenType    string         `json:"token_type"`
	Scope        string         `json:"scope,omitempty"`
	ProjectID    string         `json:"project_id,omitempty"`
	Email        string         `json:"email,omitempty"`
	AuthID       string         `json:"auth_id,omitempty"`
	Name         string         `json:"name,omitempty"`
	PlanName     string         `json:"plan_name,omitempty"`
	OAuthType    string         `json:"oauth_type,omitempty"` // "code_assist" or "google_one"
	TierID       string         `json:"tier_id,omitempty"`    // Canonical tier id (e.g. google_one_free, gcp_standard)
	Extra        map[string]any `json:"extra,omitempty"`      // Drive metadata
	Status       string         `json:"status,omitempty"`
	StatusReason string         `json:"status_reason,omitempty"`
	UsageRaw     map[string]any `json:"usage_raw,omitempty"`
	// ProjectIDMissing marks a temporary post-auth metadata gap. Tokens are valid, and project_id
	// backfill can be retried later without forcing the whole OAuth flow to fail.
	ProjectIDMissing bool `json:"-"`
}

type geminiCodeAssistSnapshot struct {
	ProjectID string
	TierID    string
	Extra     map[string]any
}

type geminiOAuthProfile struct {
	Email   string
	Subject string
	Name    string
}

// validateTierID validates tier_id format and length
func validateTierID(tierID string) error {
	if tierID == "" {
		return nil // Empty is allowed
	}
	if len(tierID) > 64 {
		return fmt.Errorf("tier_id exceeds maximum length of 64 characters")
	}
	// Allow alphanumeric, underscore, hyphen, and slash (for tier paths)
	if !regexp.MustCompile(`^[a-zA-Z0-9_/-]+$`).MatchString(tierID) {
		return fmt.Errorf("tier_id contains invalid characters")
	}
	return nil
}

func canonicalGeminiTierID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	lower := strings.ToLower(raw)
	switch lower {
	case GeminiTierGoogleOneFree,
		GeminiTierGoogleAIPro,
		GeminiTierGoogleAIUltra,
		GeminiTierGCPStandard,
		GeminiTierGCPEnterprise,
		GeminiTierAIStudioFree,
		GeminiTierAIStudioPaid,
		GeminiTierGoogleOneUnknown:
		return lower
	}

	upper := strings.ToUpper(raw)
	switch upper {
	// Google One legacy tiers
	case legacyTierAIPremium:
		return GeminiTierGoogleAIPro
	case legacyTierGoogleOneUnlimited:
		return GeminiTierGoogleAIUltra
	case legacyTierFree, legacyTierGoogleOneBasic, legacyTierGoogleOneStandard:
		return GeminiTierGoogleOneFree
	case legacyTierGoogleOneUnknown:
		return GeminiTierGoogleOneUnknown

	// Code Assist legacy tiers
	case "STANDARD", "PRO", "LEGACY":
		return GeminiTierGCPStandard
	case "ENTERPRISE", "ULTRA":
		return GeminiTierGCPEnterprise
	}

	// Some Code Assist responses use kebab-case tier identifiers.
	switch lower {
	case "free-tier":
		return GeminiTierGoogleOneFree
	case "g1-pro-tier":
		return GeminiTierGoogleAIPro
	case "g1-ultra-tier":
		return GeminiTierGoogleAIUltra
	case "standard-tier", "pro-tier":
		return GeminiTierGCPStandard
	case "ultra-tier":
		return GeminiTierGCPEnterprise
	}

	return ""
}

func canonicalGeminiTierIDForOAuthType(oauthType, tierID string) string {
	oauthType = strings.ToLower(strings.TrimSpace(oauthType))
	canonical := canonicalGeminiTierID(tierID)
	if canonical == "" {
		return ""
	}

	switch oauthType {
	case "google_one":
		switch canonical {
		case GeminiTierGoogleOneFree, GeminiTierGoogleAIPro, GeminiTierGoogleAIUltra:
			return canonical
		default:
			return ""
		}
	case "code_assist":
		switch canonical {
		case GeminiTierGCPStandard, GeminiTierGCPEnterprise:
			return canonical
		default:
			return ""
		}
	default:
		// Unknown oauth type: accept canonical tier.
		return canonical
	}
}

func normalizeGeminiOAuthType(oauthType string) string {
	switch strings.ToLower(strings.TrimSpace(oauthType)) {
	case "code_assist", "google_one":
		return strings.ToLower(strings.TrimSpace(oauthType))
	default:
		return ""
	}
}

func resolveGeminiOAuthType(rawOAuthType, tierID, _ string, planName string) string {
	if normalized := normalizeGeminiOAuthType(rawOAuthType); normalized != "" {
		return normalized
	}

	if canonicalGeminiTierIDForOAuthType("google_one", tierID) != "" {
		return "google_one"
	}
	if canonicalGeminiTierIDForOAuthType("code_assist", tierID) != "" {
		return "code_assist"
	}

	normalizedPlan := strings.ToLower(strings.TrimSpace(planName))
	switch {
	case strings.Contains(normalizedPlan, "google one"):
		return "google_one"
	case strings.Contains(normalizedPlan, "code assist"):
		return "code_assist"
	}

	return ""
}

// extractTierIDFromAllowedTiers extracts tierID from LoadCodeAssist response
// Prioritizes IsDefault tier, falls back to first non-empty tier
func extractTierIDFromAllowedTiers(allowedTiers []geminicli.AllowedTier) string {
	tierID := "LEGACY"
	// First pass: look for default tier
	for _, tier := range allowedTiers {
		if tier.IsDefault && strings.TrimSpace(tier.ID) != "" {
			tierID = strings.TrimSpace(tier.ID)
			break
		}
	}
	// Second pass: if still LEGACY, take first non-empty tier
	if tierID == "LEGACY" {
		for _, tier := range allowedTiers {
			if strings.TrimSpace(tier.ID) != "" {
				tierID = strings.TrimSpace(tier.ID)
				break
			}
		}
	}
	return tierID
}

// inferGoogleOneTier infers Google One tier from Drive storage limit
func inferGoogleOneTier(storageBytes int64) string {
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] inferGoogleOneTier - input: %d bytes (%.2f TB)", storageBytes, float64(storageBytes)/float64(TB))

	if storageBytes <= 0 {
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] inferGoogleOneTier - storageBytes <= 0, returning UNKNOWN")
		return GeminiTierGoogleOneUnknown
	}

	if storageBytes > StorageTierUnlimited {
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] inferGoogleOneTier - > %d bytes (100TB), returning UNLIMITED", StorageTierUnlimited)
		return GeminiTierGoogleAIUltra
	}
	if storageBytes >= StorageTierAIPremium {
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] inferGoogleOneTier - >= %d bytes (2TB), returning google_ai_pro", StorageTierAIPremium)
		return GeminiTierGoogleAIPro
	}
	if storageBytes >= StorageTierFree {
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] inferGoogleOneTier - >= %d bytes (15GB), returning FREE", StorageTierFree)
		return GeminiTierGoogleOneFree
	}

	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] inferGoogleOneTier - < %d bytes (15GB), returning UNKNOWN", StorageTierFree)
	return GeminiTierGoogleOneUnknown
}

// FetchGoogleOneTier fetches Google One tier from Drive API.
// Note: LoadCodeAssist API is NOT called for Google One accounts because:
// 1. It's designed for GCP IAM (enterprise), not personal Google accounts
// 2. Personal accounts will get 403/404 from cloudaicompanion.googleapis.com
// 3. Google consumer (Google One) and enterprise (GCP) systems are physically isolated
func (s *GeminiOAuthService) FetchGoogleOneTier(ctx context.Context, accessToken, proxyURL string) (string, *geminicli.DriveStorageInfo, error) {
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Starting FetchGoogleOneTier (Google One personal account)")

	// Use Drive API to infer tier from storage quota (requires drive.readonly scope)
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Calling Drive API for storage quota...")

	storageInfo, err := s.driveClient.GetStorageQuota(ctx, accessToken, proxyURL)
	if err != nil {
		// Check if it's a 403 (scope not granted)
		if strings.Contains(err.Error(), "status 403") {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Drive API scope not available (403): %v", err)
			return GeminiTierGoogleOneUnknown, nil, err
		}
		// Other errors
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Failed to fetch Drive storage: %v", err)
		return GeminiTierGoogleOneUnknown, nil, err
	}

	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Drive API response - Limit: %d bytes (%.2f TB), Usage: %d bytes (%.2f GB)",
		storageInfo.Limit, float64(storageInfo.Limit)/float64(TB),
		storageInfo.Usage, float64(storageInfo.Usage)/float64(GB))

	tierID := inferGoogleOneTier(storageInfo.Limit)
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Inferred tier from storage: %s", tierID)

	return tierID, storageInfo, nil
}

// RefreshAccountGoogleOneTier 刷新单个账号的 Google One Tier
func (s *GeminiOAuthService) RefreshAccountGoogleOneTier(
	ctx context.Context,
	account *Account,
) (tierID string, extra map[string]any, credentials map[string]any, err error) {
	if account == nil {
		return "", nil, nil, fmt.Errorf("account is nil")
	}

	// 验证账号类型
	oauthType, ok := account.Credentials["oauth_type"].(string)
	if !ok || oauthType != "google_one" {
		return "", nil, nil, fmt.Errorf("not a google_one OAuth account")
	}

	// 获取 access_token
	accessToken, ok := account.Credentials["access_token"].(string)
	if !ok || accessToken == "" {
		return "", nil, nil, fmt.Errorf("missing access_token")
	}

	// 获取 proxy URL
	var proxyURL string
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	// 调用 Drive API
	tierID, storageInfo, err := s.FetchGoogleOneTier(ctx, accessToken, proxyURL)
	if err != nil {
		return "", nil, nil, err
	}

	// 构建 extra 数据（保留原有 extra 字段）
	extra = make(map[string]any)
	for k, v := range account.Extra {
		extra[k] = v
	}
	if storageInfo != nil {
		extra["drive_storage_limit"] = storageInfo.Limit
		extra["drive_storage_usage"] = storageInfo.Usage
		extra["drive_tier_updated_at"] = time.Now().Format(time.RFC3339)
	}

	// 构建 credentials 数据
	credentials = make(map[string]any)
	for k, v := range account.Credentials {
		credentials[k] = v
	}
	credentials["tier_id"] = tierID

	return tierID, extra, credentials, nil
}

func (s *GeminiOAuthService) ExchangeCode(ctx context.Context, input *GeminiExchangeCodeInput) (*GeminiTokenInfo, error) {
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ========== ExchangeCode START ==========")
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] SessionID: %s", input.SessionID)

	session, ok := s.sessionStore.Get(input.SessionID)
	if !ok {
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ERROR: Session not found or expired")
		return nil, fmt.Errorf("session not found or expired")
	}
	if strings.TrimSpace(input.State) == "" || input.State != session.State {
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ERROR: Invalid state")
		return nil, fmt.Errorf("invalid state")
	}

	proxyURL := session.ProxyURL
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ProxyURL: %s", proxyURL)

	// Resolve oauth_type early using the session first, then the request fallback.
	rawOAuthType := session.OAuthType
	if strings.TrimSpace(rawOAuthType) == "" {
		rawOAuthType = input.OAuthType
	}
	oauthType := resolveGeminiOAuthType(rawOAuthType, input.TierID, session.ProjectID, "")
	if oauthType == "" {
		return nil, fmt.Errorf("missing oauth_type: must be 'code_assist' or 'google_one'")
	}
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] OAuth Type: %s", oauthType)
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Project ID from session: %s", session.ProjectID)

	_, err := geminicli.EffectiveOAuthConfig(geminicli.OAuthConfig{
		ClientID:     s.cfg.Gemini.OAuth.ClientID,
		ClientSecret: s.cfg.Gemini.OAuth.ClientSecret,
		Scopes:       s.cfg.Gemini.OAuth.Scopes,
	}, oauthType)
	if err != nil {
		return nil, err
	}
	redirectURI := geminicli.GeminiCLIRedirectURI

	tokenResp, err := s.oauthClient.ExchangeCode(ctx, oauthType, input.Code, session.CodeVerifier, redirectURI, proxyURL)
	if err != nil {
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ERROR: Failed to exchange code: %v", err)
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Token exchange successful")
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Token scope: %s", tokenResp.Scope)
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Token expires_in: %d seconds", tokenResp.ExpiresIn)

	sessionProjectID := strings.TrimSpace(session.ProjectID)
	s.sessionStore.Delete(input.SessionID)

	profile := s.extractProfileFromTokenResponse(ctx, tokenResp, proxyURL)
	email := profile.Email

	// 计算过期时间：减去 5 分钟安全时间窗口（考虑网络延迟和时钟偏差）
	// 同时设置下界保护，防止 expires_in 过小导致过去时间（引发刷新风暴）
	const safetyWindow = 300 // 5 minutes
	const minTTL = 30        // minimum 30 seconds
	expiresAt := time.Now().Unix() + tokenResp.ExpiresIn - safetyWindow
	minExpiresAt := time.Now().Unix() + minTTL
	if expiresAt < minExpiresAt {
		expiresAt = minExpiresAt
	}

	projectID := sessionProjectID
	var tierID string
	tokenExtra := map[string]any{}
	fallbackTierID := canonicalGeminiTierIDForOAuthType(oauthType, input.TierID)
	if fallbackTierID == "" {
		fallbackTierID = canonicalGeminiTierIDForOAuthType(oauthType, session.TierID)
	}

	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ========== Account Type Detection START ==========")
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] OAuth Type: %s", oauthType)

	// code_assist/google_one both rely on Code Assist metadata and companion-project assignment.
	switch oauthType {
	case "code_assist":
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Processing code_assist OAuth type")
		projectIDMissing := false
		if projectID == "" {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] No project_id provided, attempting to fetch from LoadCodeAssist API...")
			snapshot, detectErr := s.fetchProjectID(ctx, tokenResp.AccessToken, proxyURL)
			if snapshot != nil {
				projectID = snapshot.ProjectID
				tierID = snapshot.TierID
				tokenExtra = mergeGeminiCredentialExtra(tokenExtra, snapshot.Extra)
			}
			if detectErr != nil {
				// 记录警告但不阻断流程，允许后续补充 project_id
				fmt.Printf("[GeminiOAuth] Warning: Failed to fetch project_id during token exchange: %v\n", detectErr)
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] WARNING: Failed to fetch project_id: %v", detectErr)
				projectIDMissing = strings.TrimSpace(projectID) == ""
			} else {
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Successfully fetched project_id: %s, tier_id: %s", projectID, tierID)
			}
		} else {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] User provided project_id: %s, fetching tier_id...", projectID)
			// 用户手动填了 project_id，仍需调用 LoadCodeAssist 获取 tierID 和账户元数据
			snapshot, err := s.fetchCodeAssistSnapshot(ctx, tokenResp.AccessToken, proxyURL, projectID)
			if err != nil {
				fmt.Printf("[GeminiOAuth] Warning: Failed to fetch tierID: %v\n", err)
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] WARNING: Failed to fetch tier_id: %v", err)
			} else {
				tierID = snapshot.TierID
				tokenExtra = mergeGeminiCredentialExtra(tokenExtra, snapshot.Extra)
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Successfully fetched tier_id: %s", tierID)
			}
		}
		if strings.TrimSpace(projectID) == "" {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ERROR: Missing project_id for Code Assist OAuth")
			return nil, fmt.Errorf("missing project_id for Code Assist OAuth: please fill Project ID (optional field) and regenerate the auth URL, or ensure your Google account has an ACTIVE GCP project")
		}
		// Prefer auto-detected tier; fall back to user-selected tier.
		tierID = canonicalGeminiTierIDForOAuthType(oauthType, tierID)
		if tierID == "" {
			if fallbackTierID != "" {
				tierID = fallbackTierID
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Using fallback tier_id from user/session: %s", tierID)
			} else {
				tierID = GeminiTierGCPStandard
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Using default tier_id: %s", tierID)
			}
		}
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Final code_assist result - project_id: %s, tier_id: %s", projectID, tierID)
		if projectIDMissing {
			tokenExtra = mergeGeminiCredentialExtra(tokenExtra, map[string]any{
				"auto_detect_project_id": "true",
			})
		}

	case "google_one":
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Processing google_one OAuth type")

		// Google One accounts use cloudaicompanion API, which requires a project_id.
		// For personal accounts, Google auto-assigns a project_id via the LoadCodeAssist API.
		if projectID == "" {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] No project_id provided, attempting to fetch from LoadCodeAssist API...")
			snapshot, err := s.fetchProjectID(ctx, tokenResp.AccessToken, proxyURL)
			if snapshot != nil {
				projectID = snapshot.ProjectID
				tokenExtra = mergeGeminiCredentialExtra(tokenExtra, snapshot.Extra)
			}
			if err != nil {
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ERROR: Failed to fetch project_id: %v", err)
				return nil, fmt.Errorf("google One accounts require a project_id, failed to auto-detect: %w", err)
			}
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Successfully fetched project_id: %s", projectID)
		} else {
			snapshot, err := s.fetchCodeAssistSnapshot(ctx, tokenResp.AccessToken, proxyURL, projectID)
			if err != nil {
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] WARNING: Failed to fetch account metadata: %v", err)
			} else {
				tokenExtra = mergeGeminiCredentialExtra(tokenExtra, snapshot.Extra)
			}
		}

		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Attempting to fetch Google One tier from Drive API...")
		// Attempt to fetch Drive storage tier
		var storageInfo *geminicli.DriveStorageInfo
		var err error
		tierID, storageInfo, err = s.FetchGoogleOneTier(ctx, tokenResp.AccessToken, proxyURL)
		if err != nil {
			// Log warning but don't block - use fallback
			fmt.Printf("[GeminiOAuth] Warning: Failed to fetch Drive tier: %v\n", err)
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] WARNING: Failed to fetch Drive tier: %v", err)
			tierID = ""
		} else {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Successfully fetched Drive tier: %s", tierID)
			if storageInfo != nil {
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Drive storage - Limit: %d bytes (%.2f TB), Usage: %d bytes (%.2f GB)",
					storageInfo.Limit, float64(storageInfo.Limit)/float64(TB),
					storageInfo.Usage, float64(storageInfo.Usage)/float64(GB))
			}
		}
		tierID = canonicalGeminiTierIDForOAuthType(oauthType, tierID)
		if tierID == "" || tierID == GeminiTierGoogleOneUnknown {
			if fallbackTierID != "" {
				tierID = fallbackTierID
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Using fallback tier_id from user/session: %s", tierID)
			} else {
				tierID = GeminiTierGoogleOneFree
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Using default tier_id: %s", tierID)
			}
		}
		fmt.Printf("[GeminiOAuth] Google One tierID after normalization: %s\n", tierID)

		// Store Drive info in extra field for caching
		if storageInfo != nil {
			tokenExtra = mergeGeminiCredentialExtra(tokenExtra, map[string]any{
				"drive_storage_limit":   storageInfo.Limit,
				"drive_storage_usage":   storageInfo.Usage,
				"drive_tier_updated_at": time.Now().Format(time.RFC3339),
			})
		}

	default:
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Processing %s OAuth type (no tier detection)", oauthType)
	}

	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ========== Account Type Detection END ==========")

	result := &GeminiTokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		IDToken:      tokenResp.IDToken,
		TokenType:    tokenResp.TokenType,
		ExpiresIn:    tokenResp.ExpiresIn,
		ExpiresAt:    expiresAt,
		Scope:        tokenResp.Scope,
		ProjectID:    projectID,
		Email:        email,
		AuthID:       profile.Subject,
		Name:         profile.Name,
		PlanName:     geminiPlanNameForTier(tierID),
		TierID:       tierID,
		OAuthType:    oauthType,
		Extra:        tokenExtra,
	}
	if oauthType == "code_assist" && strings.TrimSpace(projectID) == "" {
		result.ProjectIDMissing = true
	}
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Final result - OAuth Type: %s, Project ID: %s, Tier ID: %s", result.OAuthType, result.ProjectID, result.TierID)
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ========== ExchangeCode END ==========")
	return result, nil
}

func (s *GeminiOAuthService) RefreshToken(ctx context.Context, oauthType, refreshToken, proxyURL string) (*GeminiTokenInfo, error) {
	var lastErr error

	for attempt := 0; attempt <= 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			time.Sleep(backoff)
		}

		tokenResp, err := s.oauthClient.RefreshToken(ctx, oauthType, refreshToken, proxyURL)
		if err == nil {
			profile := s.extractProfileFromTokenResponse(ctx, tokenResp, proxyURL)
			// 计算过期时间：减去 5 分钟安全时间窗口（考虑网络延迟和时钟偏差）
			// 同时设置下界保护，防止 expires_in 过小导致过去时间（引发刷新风暴）
			const safetyWindow = 300 // 5 minutes
			const minTTL = 30        // minimum 30 seconds
			expiresAt := time.Now().Unix() + tokenResp.ExpiresIn - safetyWindow
			minExpiresAt := time.Now().Unix() + minTTL
			if expiresAt < minExpiresAt {
				expiresAt = minExpiresAt
			}
			return &GeminiTokenInfo{
				AccessToken:  tokenResp.AccessToken,
				RefreshToken: tokenResp.RefreshToken,
				IDToken:      tokenResp.IDToken,
				TokenType:    tokenResp.TokenType,
				ExpiresIn:    tokenResp.ExpiresIn,
				ExpiresAt:    expiresAt,
				Scope:        tokenResp.Scope,
				Email:        profile.Email,
				AuthID:       profile.Subject,
				Name:         profile.Name,
			}, nil
		}

		if isNonRetryableGeminiOAuthError(err) {
			return nil, err
		}
		lastErr = err
	}

	return nil, fmt.Errorf("token refresh failed after retries: %w", lastErr)
}

func isNonRetryableGeminiOAuthError(err error) bool {
	msg := err.Error()
	nonRetryable := []string{
		"invalid_grant",
		"invalid_client",
		"unauthorized_client",
		"access_denied",
	}
	for _, needle := range nonRetryable {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}

func (s *GeminiOAuthService) RefreshAccountToken(ctx context.Context, account *Account) (*GeminiTokenInfo, error) {
	if account.Platform != PlatformGemini || account.Type != AccountTypeOAuth {
		return nil, fmt.Errorf("account is not a Gemini OAuth account")
	}

	refreshToken := account.GetCredential("refresh_token")
	if strings.TrimSpace(refreshToken) == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	rawOAuthType := account.GetCredential("oauth_type")
	tierIDHint := account.GetCredential("tier_id")
	projectIDHint := account.GetCredential("project_id")
	planNameHint := account.GetCredential("plan_name")
	oauthType := resolveGeminiOAuthType(rawOAuthType, tierIDHint, projectIDHint, planNameHint)
	if oauthType == "" {
		return nil, fmt.Errorf("missing oauth_type and unable to infer a supported Gemini OAuth flow from stored credentials; please re-authorize this account")
	}

	var proxyURL string
	if account.ProxyID != nil {
		proxy, err := s.proxyRepo.GetByID(ctx, *account.ProxyID)
		if err == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	tokenInfo, err := s.RefreshToken(ctx, oauthType, refreshToken, proxyURL)
	if err != nil {
		// Provide a more actionable error for common OAuth client mismatch issues.
		if strings.Contains(err.Error(), "unauthorized_client") {
			return nil, fmt.Errorf("%w (OAuth client mismatch: the refresh_token is bound to the OAuth client used during authorization; please re-authorize this account or restore the original GEMINI_OAUTH_CLIENT_ID/SECRET)", err)
		}
		return nil, err
	}

	tokenInfo.OAuthType = oauthType
	tokenInfo.PlanName = geminiPlanNameForTier(tokenInfo.TierID)

	if existingEmail := strings.TrimSpace(account.GetCredential("email")); existingEmail != "" && tokenInfo.Email == "" {
		tokenInfo.Email = existingEmail
	}
	if existingAuthID := strings.TrimSpace(account.GetCredential("auth_id")); existingAuthID != "" && tokenInfo.AuthID == "" {
		tokenInfo.AuthID = existingAuthID
	}
	if existingName := strings.TrimSpace(account.GetCredential("name")); existingName != "" && tokenInfo.Name == "" {
		tokenInfo.Name = existingName
	}
	tokenInfo.Extra = extractGeminiPersistedExtra(account.Credentials)

	// Preserve account's project_id when present.
	existingProjectID := strings.TrimSpace(account.GetCredential("project_id"))
	if existingProjectID != "" {
		tokenInfo.ProjectID = existingProjectID
	}

	// 尝试从账号凭证获取 tierID（向后兼容）
	existingTierID := strings.TrimSpace(account.GetCredential("tier_id"))

	// For Code Assist, project_id is required. Auto-detect if missing.
	switch oauthType {
	case "code_assist":
		// 先设置默认值或保留旧值，确保 tier_id 始终有值
		if existingTierID != "" {
			tokenInfo.TierID = canonicalGeminiTierIDForOAuthType(oauthType, existingTierID)
		}
		if tokenInfo.TierID == "" {
			tokenInfo.TierID = GeminiTierGCPStandard
		}

		// 尝试自动探测 project_id 和 tier_id
		needDetect := strings.TrimSpace(tokenInfo.ProjectID) == "" || tokenInfo.TierID == ""
		if needDetect {
			snapshot, detectErr := s.fetchProjectID(ctx, tokenInfo.AccessToken, proxyURL)
			projectID := ""
			tierID := ""
			if snapshot != nil {
				projectID = snapshot.ProjectID
				tierID = snapshot.TierID
				tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, snapshot.Extra)
			}
			err := detectErr
			if err != nil {
				fmt.Printf("[GeminiOAuth] Warning: failed to auto-detect project/tier: %v\n", err)
			} else {
				if strings.TrimSpace(tokenInfo.ProjectID) == "" && projectID != "" {
					tokenInfo.ProjectID = projectID
				}
				if tierID != "" {
					if canonical := canonicalGeminiTierIDForOAuthType(oauthType, tierID); canonical != "" {
						tokenInfo.TierID = canonical
					}
				}
			}
		}

		if strings.TrimSpace(tokenInfo.ProjectID) == "" {
			tokenInfo.ProjectIDMissing = true
			tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, map[string]any{
				"auto_detect_project_id": "true",
			})
		}
	case "google_one":
		if existingProjectID != "" {
			if snapshot, err := s.fetchCodeAssistSnapshot(ctx, tokenInfo.AccessToken, proxyURL, existingProjectID); err == nil {
				tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, snapshot.Extra)
			} else {
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] WARNING: Failed to refresh account metadata: %v", err)
			}
		}
		canonicalExistingTier := canonicalGeminiTierIDForOAuthType(oauthType, existingTierID)
		// Check if tier cache is stale (> 24 hours)
		needsRefresh := true
		if updatedAt, ok := geminiDriveTierUpdatedAt(account); ok && time.Since(updatedAt) <= 24*time.Hour {
			needsRefresh = false
			// Use cached tier persisted via BuildAccountCredentials; fall back to legacy account.Extra.
			tokenInfo.TierID = canonicalExistingTier
		}

		if tokenInfo.TierID == "" {
			tokenInfo.TierID = canonicalExistingTier
		}

		if needsRefresh {
			tierID, storageInfo, err := s.FetchGoogleOneTier(ctx, tokenInfo.AccessToken, proxyURL)
			if err == nil {
				if canonical := canonicalGeminiTierIDForOAuthType(oauthType, tierID); canonical != "" && canonical != GeminiTierGoogleOneUnknown {
					tokenInfo.TierID = canonical
				}
				if storageInfo != nil {
					tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, map[string]any{
						"drive_storage_limit":   storageInfo.Limit,
						"drive_storage_usage":   storageInfo.Usage,
						"drive_tier_updated_at": time.Now().Format(time.RFC3339),
					})
				}
			}
		}

		if tokenInfo.TierID == "" || tokenInfo.TierID == GeminiTierGoogleOneUnknown {
			if canonicalExistingTier != "" {
				tokenInfo.TierID = canonicalExistingTier
			} else {
				tokenInfo.TierID = GeminiTierGoogleOneFree
			}
		}
	}
	tokenInfo.PlanName = geminiPlanNameForTier(tokenInfo.TierID)

	return tokenInfo, nil
}

func (s *GeminiOAuthService) BuildAccountCredentials(tokenInfo *GeminiTokenInfo) map[string]any {
	creds := map[string]any{
		"access_token": tokenInfo.AccessToken,
		"expires_at":   strconv.FormatInt(tokenInfo.ExpiresAt, 10),
	}
	if tokenInfo.RefreshToken != "" {
		creds["refresh_token"] = tokenInfo.RefreshToken
	}
	if tokenInfo.IDToken != "" {
		creds["id_token"] = tokenInfo.IDToken
	}
	if tokenInfo.TokenType != "" {
		creds["token_type"] = tokenInfo.TokenType
	}
	if tokenInfo.Scope != "" {
		creds["scope"] = tokenInfo.Scope
	}
	if tokenInfo.ProjectID != "" {
		creds["project_id"] = tokenInfo.ProjectID
	}
	if tokenInfo.Email != "" {
		creds["email"] = tokenInfo.Email
	}
	if tokenInfo.AuthID != "" {
		creds["auth_id"] = tokenInfo.AuthID
		creds["subject"] = tokenInfo.AuthID
	}
	if tokenInfo.Name != "" {
		creds["name"] = tokenInfo.Name
	}
	if tokenInfo.PlanName != "" {
		creds["plan_name"] = tokenInfo.PlanName
	}
	if tokenInfo.TierID != "" {
		// Validate tier_id before storing
		if err := validateTierID(tokenInfo.TierID); err == nil {
			creds["tier_id"] = tokenInfo.TierID
			fmt.Printf("[GeminiOAuth] Storing tier_id: %s\n", tokenInfo.TierID)
		} else {
			fmt.Printf("[GeminiOAuth] Invalid tier_id %s: %v\n", tokenInfo.TierID, err)
		}
		// Silently skip invalid tier_id (don't block account creation)
	}
	if tokenInfo.OAuthType != "" {
		creds["oauth_type"] = tokenInfo.OAuthType
	}
	if tokenInfo.Status != "" {
		creds["gemini_status"] = tokenInfo.Status
	}
	if tokenInfo.StatusReason != "" {
		creds["gemini_status_reason"] = tokenInfo.StatusReason
	}
	if len(tokenInfo.UsageRaw) > 0 {
		creds["gemini_usage_raw"] = tokenInfo.UsageRaw
		creds["usage_updated_at"] = time.Now().Format(time.RFC3339)
	}
	// Store extra metadata (Drive info) if present
	if len(tokenInfo.Extra) > 0 {
		for k, v := range tokenInfo.Extra {
			creds[k] = v
		}
	}
	return creds
}

func mergeGeminiCredentialExtra(base map[string]any, extras ...map[string]any) map[string]any {
	merged := make(map[string]any)
	for k, v := range base {
		merged[k] = v
	}
	for _, extra := range extras {
		for k, v := range extra {
			merged[k] = v
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func extractGeminiPersistedExtra(credentials map[string]any) map[string]any {
	keys := []string{
		"drive_storage_limit",
		"drive_storage_usage",
		"drive_tier_updated_at",
		"gemini_current_tier_id",
		"gemini_current_tier_name",
		"gemini_paid_tier_id",
		"gemini_paid_tier_name",
		"gemini_has_onboarded_previously",
		"gemini_available_credits",
		"gemini_code_assist_updated_at",
		"gemini_usage_raw",
		"gemini_status",
		"gemini_status_reason",
		"quota_query_last_error",
		"quota_query_last_error_at",
		"usage_updated_at",
	}
	extra := make(map[string]any)
	for _, key := range keys {
		if value, ok := credentials[key]; ok {
			extra[key] = value
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return extra
}

func geminiDriveTierUpdatedAt(account *Account) (time.Time, bool) {
	if account == nil {
		return time.Time{}, false
	}

	if updatedAt, ok := parseGeminiUpdatedAt(account.Credentials["drive_tier_updated_at"]); ok {
		return updatedAt, true
	}

	if updatedAt, ok := parseGeminiUpdatedAt(account.Extra["drive_tier_updated_at"]); ok {
		return updatedAt, true
	}

	return time.Time{}, false
}

func parseGeminiUpdatedAt(raw any) (time.Time, bool) {
	updatedAtStr, ok := raw.(string)
	if !ok || strings.TrimSpace(updatedAtStr) == "" {
		return time.Time{}, false
	}

	updatedAt, err := time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return time.Time{}, false
	}

	return updatedAt, true
}

func buildGeminiCodeAssistExtra(loadResp *geminicli.LoadCodeAssistResponse) map[string]any {
	if loadResp == nil {
		return nil
	}
	extra := map[string]any{
		"gemini_code_assist_updated_at": time.Now().Format(time.RFC3339),
	}
	if loadResp.CurrentTier != nil {
		if value := strings.TrimSpace(loadResp.CurrentTier.ID); value != "" {
			extra["gemini_current_tier_id"] = value
		}
		if value := strings.TrimSpace(loadResp.CurrentTier.Name); value != "" {
			extra["gemini_current_tier_name"] = value
		}
		if loadResp.CurrentTier.HasOnboardedPreviously != nil {
			extra["gemini_has_onboarded_previously"] = *loadResp.CurrentTier.HasOnboardedPreviously
		}
		if len(loadResp.CurrentTier.AvailableCredits) > 0 {
			extra["gemini_available_credits"] = loadResp.CurrentTier.AvailableCredits
		}
	}
	if loadResp.PaidTier != nil {
		if value := strings.TrimSpace(loadResp.PaidTier.ID); value != "" {
			extra["gemini_paid_tier_id"] = value
		}
		if value := strings.TrimSpace(loadResp.PaidTier.Name); value != "" {
			extra["gemini_paid_tier_name"] = value
		}
		if len(loadResp.PaidTier.AvailableCredits) > 0 {
			extra["gemini_available_credits"] = loadResp.PaidTier.AvailableCredits
		}
	}
	return extra
}

func buildGeminiQuotaExtra(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	return map[string]any{
		"gemini_usage_raw": raw,
		"usage_updated_at": time.Now().Format(time.RFC3339),
	}
}

func buildGeminiQuotaErrorExtra(err error) map[string]any {
	if err == nil {
		return nil
	}
	return map[string]any{
		"quota_query_last_error":    err.Error(),
		"quota_query_last_error_at": time.Now().Format(time.RFC3339),
	}
}

func clearGeminiQuotaErrorExtra() map[string]any {
	return map[string]any{
		"quota_query_last_error":    "",
		"quota_query_last_error_at": "",
	}
}

func buildGeminiLoadCodeAssistRequest(projectID string) *geminicli.LoadCodeAssistRequest {
	trimmedProjectID := strings.TrimSpace(projectID)
	return &geminicli.LoadCodeAssistRequest{
		CloudAICompanionProject: trimmedProjectID,
		Metadata: geminicli.LoadCodeAssistMetadata{
			IDEType:     "ANTIGRAVITY",
			Platform:    "PLATFORM_UNSPECIFIED",
			PluginType:  "GEMINI",
			DuetProject: trimmedProjectID,
		},
	}
}

func (s *GeminiOAuthService) Stop() {
	s.sessionStore.Stop()
}

func (s *GeminiOAuthService) extractProfileFromTokenResponse(ctx context.Context, tokenResp *geminicli.TokenResponse, proxyURL string) geminiOAuthProfile {
	if tokenResp == nil {
		return geminiOAuthProfile{}
	}
	profile := extractGeminiProfileFromIDToken(tokenResp.IDToken)
	if !geminiTokenScopeHasUserInfo(tokenResp.Scope) {
		return profile
	}
	userInfo, err := fetchGeminiUserInfo(ctx, tokenResp.AccessToken, proxyURL)
	if err != nil {
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] WARNING: Failed to fetch userinfo profile: %v", err)
		return profile
	}
	if profile.Email == "" {
		profile.Email = userInfo.Email
	}
	if profile.Subject == "" {
		profile.Subject = userInfo.Subject
	}
	if profile.Name == "" {
		profile.Name = userInfo.Name
	}
	return profile
}

func geminiTokenScopeHasUserInfo(scope string) bool {
	for _, part := range strings.Fields(scope) {
		if part == "email" ||
			part == "profile" ||
			part == "https://www.googleapis.com/auth/userinfo.email" ||
			part == "https://www.googleapis.com/auth/userinfo.profile" {
			return true
		}
	}
	return false
}

func extractGeminiProfileFromIDToken(idToken string) geminiOAuthProfile {
	idToken = strings.TrimSpace(idToken)
	if idToken == "" {
		return geminiOAuthProfile{}
	}
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return geminiOAuthProfile{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return geminiOAuthProfile{}
	}
	var claims struct {
		Email   string `json:"email"`
		Subject string `json:"sub"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return geminiOAuthProfile{}
	}
	return geminiOAuthProfile{
		Email:   strings.TrimSpace(claims.Email),
		Subject: strings.TrimSpace(claims.Subject),
		Name:    strings.TrimSpace(claims.Name),
	}
}

func fetchGeminiUserInfo(ctx context.Context, accessToken, proxyURL string) (geminiOAuthProfile, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return geminiOAuthProfile{}, errors.New("access token is empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleOAuthUserInfoURL, nil)
	if err != nil {
		return geminiOAuthProfile{}, fmt.Errorf("failed to create userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", geminicli.GeminiCLIUserAgent)

	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL:           strings.TrimSpace(proxyURL),
		Timeout:            10 * time.Second,
		ValidateResolvedIP: true,
	})
	if err != nil {
		return geminiOAuthProfile{}, fmt.Errorf("create http client failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return geminiOAuthProfile{}, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return geminiOAuthProfile{}, fmt.Errorf("failed to read userinfo response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return geminiOAuthProfile{}, fmt.Errorf("userinfo HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var info struct {
		Email string `json:"email"`
		Sub   string `json:"sub"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(bodyBytes, &info); err != nil {
		return geminiOAuthProfile{}, fmt.Errorf("failed to parse userinfo response: %w", err)
	}
	profile := geminiOAuthProfile{
		Email:   strings.TrimSpace(info.Email),
		Subject: strings.TrimSpace(info.Sub),
		Name:    strings.TrimSpace(info.Name),
	}
	if profile.Email == "" && profile.Subject == "" && profile.Name == "" {
		return geminiOAuthProfile{}, errors.New("userinfo response missing profile fields")
	}
	return profile, nil
}

func geminiPlanNameForTier(tierID string) string {
	switch canonicalGeminiTierID(tierID) {
	case GeminiTierGoogleAIPro:
		return "Gemini Code Assist in Google One AI Pro"
	case GeminiTierGoogleAIUltra:
		return "Gemini Code Assist in Google One AI Ultra"
	case GeminiTierGoogleOneFree:
		return "Gemini Code Assist in Google One Free"
	case GeminiTierGCPEnterprise:
		return "Gemini Code Assist Enterprise"
	case GeminiTierGCPStandard:
		return "Gemini Code Assist Standard"
	case GeminiTierAIStudioPaid:
		return "Google AI Studio Pay-as-you-go"
	case GeminiTierAIStudioFree:
		return "Google AI Studio Free"
	default:
		return ""
	}
}

func (s *GeminiOAuthService) fetchCodeAssistSnapshot(ctx context.Context, accessToken, proxyURL, projectID string) (*geminiCodeAssistSnapshot, error) {
	if s.codeAssist == nil {
		return nil, errors.New("code assist client not configured")
	}

	loadResp, err := s.codeAssist.LoadCodeAssist(ctx, accessToken, proxyURL, buildGeminiLoadCodeAssistRequest(projectID))
	snapshot := &geminiCodeAssistSnapshot{Extra: buildGeminiCodeAssistExtra(loadResp)}
	if loadResp != nil {
		snapshot.ProjectID = strings.TrimSpace(loadResp.CloudAICompanionProject)
		if tier := loadResp.GetTier(); tier != "" {
			snapshot.TierID = tier
		} else {
			snapshot.TierID = extractTierIDFromAllowedTiers(loadResp.AllowedTiers)
		}
	}
	if err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (s *GeminiOAuthService) fetchProjectID(ctx context.Context, accessToken, proxyURL string) (*geminiCodeAssistSnapshot, error) {
	snapshot, loadErr := s.fetchCodeAssistSnapshot(ctx, accessToken, proxyURL, "")
	if snapshot == nil {
		snapshot = &geminiCodeAssistSnapshot{}
	}

	// If LoadCodeAssist returned a project, use it
	if loadErr == nil && strings.TrimSpace(snapshot.ProjectID) != "" {
		return snapshot, nil
	}

	// 对齐 Gemini CLI：
	// 当 LoadCodeAssist 返回了 currentTier / paidTier（表示账号已注册）但没有返回 cloudaicompanionProject 时，
	// 不再本地枚举用户的 GCP project；直接要求用户手动提供 Project ID。
	if loadErr == nil {
		registeredTierID := strings.TrimSpace(snapshot.TierID)
		if registeredTierID != "" {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] User has tier (%s) but no cloudaicompanionProject; manual project_id required", registeredTierID)
			return snapshot, fmt.Errorf("user is registered (tier: %s) but no project_id available. Please provide Project ID manually in the authorization form, or create a project at https://console.cloud.google.com", registeredTierID)
		}
	}

	// 未检测到 currentTier/paidTier，视为新用户，继续调用 onboardUser
	if strings.TrimSpace(snapshot.TierID) == "" {
		snapshot.TierID = "LEGACY"
	}
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] No currentTier/paidTier found, proceeding with onboardUser (tierID: %s)", snapshot.TierID)

	req := &geminicli.OnboardUserRequest{
		TierID:                  snapshot.TierID,
		CloudAICompanionProject: "",
		Metadata:                buildGeminiLoadCodeAssistRequest("").Metadata,
	}

	resp, err := s.codeAssist.OnboardUser(ctx, accessToken, proxyURL, req)
	if err != nil {
		return snapshot, err
	}
	if projectID := extractGeminiCompanionProjectID(resp); projectID != "" {
		snapshot.ProjectID = projectID
		return snapshot, nil
	}

	operationName := strings.TrimSpace(resp.Name)
	maxAttempts := 5
	for attempt := 1; attempt <= maxAttempts && operationName != ""; attempt++ {
		if resp.Done {
			break
		}
		time.Sleep(2 * time.Second)
		resp, err = s.codeAssist.GetOperation(ctx, accessToken, proxyURL, operationName)
		if err != nil {
			return snapshot, err
		}
		if projectID := extractGeminiCompanionProjectID(resp); projectID != "" {
			snapshot.ProjectID = projectID
			return snapshot, nil
		}
	}
	if resp != nil && resp.Done {
		return snapshot, errors.New("onboardUser completed but no project_id returned")
	}
	if loadErr != nil {
		return snapshot, fmt.Errorf("loadCodeAssist failed (%v) and onboardUser timeout after %d attempts", loadErr, maxAttempts)
	}
	return snapshot, fmt.Errorf("onboardUser timeout after %d attempts", maxAttempts)
}

func extractGeminiCompanionProjectID(resp *geminicli.OnboardUserResponse) string {
	if resp == nil || resp.Response == nil || resp.Response.CloudAICompanionProject == nil {
		return ""
	}
	switch v := resp.Response.CloudAICompanionProject.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		if id, ok := v["id"].(string); ok {
			return strings.TrimSpace(id)
		}
		if id, ok := v["projectId"].(string); ok {
			return strings.TrimSpace(id)
		}
	}
	return ""
}
