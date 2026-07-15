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

const googleOAuthUserInfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"

var geminiTierIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_/-]+$`)

type GeminiOAuthService struct {
	sessionStore *geminicli.SessionStore
	proxyRepo    ProxyRepository
	oauthClient  GeminiOAuthClient
	codeAssist   GeminiCliCodeAssistClient
	cfg          *config.Config
}

type GeminiOAuthCapabilities struct {
	AIStudioOAuthEnabled bool     `json:"ai_studio_oauth_enabled"`
	RequiredRedirectURIs []string `json:"required_redirect_uris"`
}

func (s *GeminiOAuthService) WithSessionStore(store *geminicli.SessionStore) *GeminiOAuthService {
	if s == nil || store == nil {
		return s
	}
	if s.sessionStore != nil {
		s.sessionStore.Stop()
	}
	s.sessionStore = store
	return s
}

func NewGeminiOAuthService(
	proxyRepo ProxyRepository,
	oauthClient GeminiOAuthClient,
	codeAssist GeminiCliCodeAssistClient,
	cfg *config.Config,
) *GeminiOAuthService {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return &GeminiOAuthService{
		sessionStore: geminicli.NewSessionStore(),
		proxyRepo:    proxyRepo,
		oauthClient:  oauthClient,
		codeAssist:   codeAssist,
		cfg:          cfg,
	}
}

func (s *GeminiOAuthService) resolveProxyURL(ctx context.Context, proxyID *int64) (string, error) {
	if proxyID == nil {
		return "", nil
	}
	if s == nil || s.proxyRepo == nil {
		return "", fmt.Errorf("proxy repository is unavailable")
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

func (s *GeminiOAuthService) requireOAuthClient() (GeminiOAuthClient, error) {
	if s == nil || s.oauthClient == nil {
		return nil, fmt.Errorf("oauth client is not configured")
	}
	return s.oauthClient, nil
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

func (s *GeminiOAuthService) GenerateAuthURL(ctx context.Context, proxyID *int64, redirectURI, projectIDHint, oauthType, tierID string) (*GeminiAuthURLResult, error) {
	oauthType = resolveGeminiOAuthType(oauthType, tierID, projectIDHint, "")
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

	proxyURL, err := s.resolveProxyURL(ctx, proxyID)
	if err != nil {
		return nil, err
	}

	effectiveCfg, err := geminicli.EffectiveOAuthConfig(geminicli.OAuthConfig{
		ClientID:     s.cfg.Gemini.OAuth.ClientID,
		ClientSecret: s.cfg.Gemini.OAuth.ClientSecret,
		Scopes:       s.cfg.Gemini.OAuth.Scopes,
	}, oauthType)
	if err != nil {
		return nil, err
	}
	redirectURI = resolveGeminiOAuthRedirectURI(effectiveCfg, redirectURI)

	session := &geminicli.OAuthSession{
		State:         state,
		CodeVerifier:  codeVerifier,
		ProxyURL:      proxyURL,
		RedirectURI:   redirectURI,
		ProjectIDHint: strings.TrimSpace(projectIDHint),
		TierID:        canonicalGeminiTierIDForOAuthType(oauthType, tierID),
		OAuthType:     oauthType,
		CreatedAt:     time.Now(),
	}
	s.sessionStore.Set(sessionID, session)

	authURL, err := geminicli.BuildAuthorizationURL(effectiveCfg, state, codeChallenge, session.RedirectURI, session.ProjectIDHint, oauthType)
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
	Extra        map[string]any `json:"extra,omitempty"`      // Supplemental Gemini metadata
	Status       string         `json:"status,omitempty"`
	StatusReason string         `json:"status_reason,omitempty"`
	UsageRaw     map[string]any `json:"usage_raw,omitempty"`
	// ProjectIDMissing marks a temporary post-auth metadata gap. Tokens are valid, and project_id
	// backfill can be retried later without forcing the whole OAuth flow to fail.
	ProjectIDMissing bool `json:"-"`
	// ProjectMetadataConfirmed marks that this refresh/auth cycle explicitly confirmed Gemini
	// project metadata or companion-project validity via upstream metadata APIs.
	ProjectMetadataConfirmed bool `json:"-"`
}

type geminiCodeAssistSnapshot struct {
	ProjectID       string
	TierID          string
	TierRegistered  bool // true if tier comes from currentTier/paidTier (registered user), false if from allowedTiers (new user)
	TierUserDefined *bool
	HasOnboarded    *bool
	IneligibleTiers []geminicli.IneligibleTier
	Extra           map[string]any
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
	if !geminiTierIDPattern.MatchString(tierID) {
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

func firstGeminiValidationRequiredTier(ineligibleTiers []geminicli.IneligibleTier) *geminicli.IneligibleTier {
	for i := range ineligibleTiers {
		tier := &ineligibleTiers[i]
		if tier.ReasonCode == geminicli.IneligibleTierReasonCodeValidationRequired {
			return tier
		}
	}
	return nil
}

func buildGeminiCodeAssistValidationError(ineligibleTiers []geminicli.IneligibleTier) error {
	tier := firstGeminiValidationRequiredTier(ineligibleTiers)
	if tier == nil {
		return nil
	}

	message := strings.TrimSpace(tier.ValidationErrorMessage)
	if message == "" {
		message = strings.TrimSpace(tier.ReasonMessage)
	}
	if message == "" {
		message = "Gemini Code Assist account validation required"
	}

	validationURL := strings.TrimSpace(tier.ValidationURL)
	if validationURL != "" {
		return fmt.Errorf("validation_required: %s (validation_url=%s)", message, validationURL)
	}
	return fmt.Errorf("validation_required: %s", message)
}

func joinGeminiIneligibleTierReasonCodes(ineligibleTiers []geminicli.IneligibleTier) string {
	codes := make([]string, 0, len(ineligibleTiers))
	seen := make(map[string]struct{}, len(ineligibleTiers))
	for _, tier := range ineligibleTiers {
		code := strings.TrimSpace(string(tier.ReasonCode))
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	return strings.Join(codes, ",")
}

func summarizeGeminiIneligibleTiers(ineligibleTiers []geminicli.IneligibleTier) string {
	if len(ineligibleTiers) == 0 {
		return ""
	}

	parts := make([]string, 0, len(ineligibleTiers))
	for _, tier := range ineligibleTiers {
		values := make([]string, 0, 4)
		if code := strings.TrimSpace(string(tier.ReasonCode)); code != "" {
			values = append(values, fmt.Sprintf("reason_code=%s", code))
		}
		if tierID := strings.TrimSpace(tier.TierID); tierID != "" {
			values = append(values, fmt.Sprintf("tier_id=%s", tierID))
		}
		if message := strings.TrimSpace(tier.ReasonMessage); message != "" {
			values = append(values, fmt.Sprintf("reason_message=%q", message))
		}
		if message := strings.TrimSpace(tier.ValidationErrorMessage); message != "" {
			values = append(values, fmt.Sprintf("validation_error=%q", message))
		}
		parts = append(parts, "{"+strings.Join(values, ", ")+"}")
	}
	return strings.Join(parts, "; ")
}

func buildGeminiCodeAssistIneligibleError(ineligibleTiers []geminicli.IneligibleTier) error {
	if len(ineligibleTiers) == 0 {
		return nil
	}
	if err := buildGeminiCodeAssistValidationError(ineligibleTiers); err != nil {
		return err
	}

	reasons := make([]string, 0, len(ineligibleTiers))
	for _, tier := range ineligibleTiers {
		reason := strings.TrimSpace(tier.ReasonMessage)
		if reason == "" {
			reason = strings.TrimSpace(tier.ValidationErrorMessage)
		}
		if reason == "" && tier.ReasonCode != "" {
			reason = string(tier.ReasonCode)
		}
		if reason == "" {
			reason = "Gemini Code Assist tier is not eligible"
		}
		reasons = append(reasons, reason)
	}

	if reasonCodes := joinGeminiIneligibleTierReasonCodes(ineligibleTiers); reasonCodes != "" {
		return fmt.Errorf("ineligible_tier[%s]: %s", reasonCodes, strings.Join(reasons, "; "))
	}
	return fmt.Errorf("ineligible_tier: %s", strings.Join(reasons, "; "))
}

func buildGeminiCodeAssistProjectIDRequiredError(registeredTierID string, userDefinedProject *bool) error {
	if userDefinedProject != nil && *userDefinedProject {
		if tierID := strings.TrimSpace(registeredTierID); tierID != "" {
			return fmt.Errorf("user_defined_project_required: upstream tier %s requires a user-defined cloudaicompanionProject, so Google did not auto-return a companion project for this account. If you do not want to provide your own Project ID, use a different eligible account/tier that supports auto-assigned companion projects", tierID)
		}
		return fmt.Errorf("user_defined_project_required: this upstream tier requires a user-defined cloudaicompanionProject, so Google did not auto-return a companion project for this account")
	}
	if tierID := strings.TrimSpace(registeredTierID); tierID != "" {
		return fmt.Errorf("registered_tier_missing_companion_project: user is registered (tier: %s) but upstream returned no cloudaicompanionProject. This usually means upstream account state is incomplete or inconsistent for this Gemini tier", tierID)
	}
	return fmt.Errorf("project_id required for Gemini Code Assist authorization. The upstream response did not provide a usable cloudaicompanionProject")
}

func IsGeminiProjectConfigurationError(err error) bool {
	if err == nil {
		return false
	}
	return IsGeminiProjectConfigurationErrorMessage(err.Error())
}

func IsGeminiProjectConfigurationErrorMessage(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "missing_project_id") ||
		strings.Contains(normalized, "missing project_id for code assist oauth") ||
		strings.Contains(normalized, "google one accounts require a project_id") ||
		strings.Contains(normalized, "no project_id available") ||
		strings.Contains(normalized, "user_defined_project_required:") ||
		strings.Contains(normalized, "registered_tier_missing_companion_project:") ||
		strings.Contains(normalized, "project_id required for gemini code assist authorization")
}

func isGeminiEligibilityError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "validation_required:") ||
		strings.Contains(message, "ineligible_tier") ||
		IsGeminiProjectConfigurationErrorMessage(message)
}

func isGeminiValidationOrIneligibleError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "validation_required:") ||
		strings.Contains(message, "ineligible_tier")
}

func (s *GeminiOAuthService) ExchangeCode(ctx context.Context, input *GeminiExchangeCodeInput) (*GeminiTokenInfo, error) {
	if input == nil {
		return nil, fmt.Errorf("oauth input is required")
	}
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
	if !s.sessionStore.TryConsumeSession(input.SessionID) {
		return nil, fmt.Errorf("oauth session has already been used")
	}
	defer s.sessionStore.Delete(input.SessionID)

	proxyURL := session.ProxyURL
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ProxyURL: %s", proxyURL)

	// Resolve oauth_type early using the session first, then the request fallback.
	rawOAuthType := session.OAuthType
	if strings.TrimSpace(rawOAuthType) == "" {
		rawOAuthType = input.OAuthType
	}
	oauthType := resolveGeminiOAuthType(rawOAuthType, input.TierID, session.ProjectIDHint, "")
	if oauthType == "" {
		return nil, fmt.Errorf("missing oauth_type: must be 'code_assist' or 'google_one'")
	}
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] OAuth Type: %s", oauthType)
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Project ID hint from session: %s", session.ProjectIDHint)

	_, err := geminicli.EffectiveOAuthConfig(geminicli.OAuthConfig{
		ClientID:     s.cfg.Gemini.OAuth.ClientID,
		ClientSecret: s.cfg.Gemini.OAuth.ClientSecret,
		Scopes:       s.cfg.Gemini.OAuth.Scopes,
	}, oauthType)
	if err != nil {
		return nil, err
	}
	redirectURI := strings.TrimSpace(session.RedirectURI)
	if redirectURI == "" {
		redirectURI = geminicli.AIStudioOAuthRedirectURI
	}
	oauthClient, err := s.requireOAuthClient()
	if err != nil {
		return nil, err
	}

	tokenResp, err := oauthClient.ExchangeCode(ctx, oauthType, input.Code, session.CodeVerifier, redirectURI, proxyURL)
	if err != nil {
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ERROR: Failed to exchange code: %v", err)
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Token exchange successful")
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Token scope: %s", tokenResp.Scope)
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Token expires_in: %d seconds", tokenResp.ExpiresIn)

	sessionProjectIDHint := strings.TrimSpace(session.ProjectIDHint)
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

	projectID := ""
	projectIDHint := sessionProjectIDHint
	var tierID string
	tokenExtra := map[string]any{}
	fallbackTierID := canonicalGeminiTierIDForOAuthType(oauthType, input.TierID)
	if fallbackTierID == "" {
		fallbackTierID = canonicalGeminiTierIDForOAuthType(oauthType, session.TierID)
	}

	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ========== Account Type Detection START ==========")
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] OAuth Type: %s", oauthType)

	switch oauthType {
	case "code_assist":
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Processing code_assist OAuth type")
		projectIDMissing := false
		if projectIDHint == "" {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] No project_id provided, attempting to fetch from LoadCodeAssist API...")
		} else {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] User provided project_id hint: %s, fetching companion project and tier...", projectIDHint)
		}
		snapshot, detectErr := s.fetchProjectID(ctx, tokenResp.AccessToken, proxyURL, projectIDHint)
		if snapshot != nil {
			projectID = snapshot.ProjectID
			tierID = snapshot.TierID
			tokenExtra = mergeGeminiCredentialExtra(tokenExtra, snapshot.Extra)
		}
		if detectErr != nil {
			if isGeminiEligibilityError(detectErr) {
				return nil, detectErr
			}
			// 记录警告但不阻断流程，允许后续补充 project_id
			fmt.Printf("[GeminiOAuth] Warning: Failed to fetch project_id during token exchange: %v\n", detectErr)
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] WARNING: Failed to fetch project_id: %v", detectErr)
			projectIDMissing = strings.TrimSpace(projectID) == ""
		} else {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Successfully fetched project_id: %s, tier_id: %s", projectID, tierID)
		}
		if strings.TrimSpace(projectID) == "" {
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ERROR: Missing project_id for Code Assist OAuth")
			return nil, fmt.Errorf("missing project_id for Code Assist OAuth: please fill Project ID (optional field) and regenerate the auth URL, or ensure your Google account has an ACTIVE GCP project")
		}
		// Prefer auto-detected tier; fall back to user-selected tier.
		tierID = strings.TrimSpace(tierID)
		if tierID == "" {
			if fallbackTierID != "" {
				tierID = fallbackTierID
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Using fallback tier_id from user/session: %s", tierID)
			} else {
				tierID = "STANDARD"
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
		detectedTierID := ""
		snapshot, detectErr := s.fetchProjectID(ctx, tokenResp.AccessToken, proxyURL, projectIDHint)
		if snapshot != nil {
			projectID = snapshot.ProjectID
			detectedTierID = canonicalGeminiTierIDForOAuthType(oauthType, snapshot.TierID)
			tokenExtra = mergeGeminiCredentialExtra(tokenExtra, snapshot.Extra)
		}
		if detectErr != nil {
			if isGeminiValidationOrIneligibleError(detectErr) {
				return nil, detectErr
			}
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] WARNING: Failed to fetch Google One metadata: %v", detectErr)
			tokenExtra = mergeGeminiCredentialExtra(tokenExtra, map[string]any{
				"auto_detect_project_id": "true",
			})
		}
		switch {
		case detectedTierID != "":
			tierID = detectedTierID
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Using detected tier_id from account metadata: %s", tierID)
		case fallbackTierID != "":
			tierID = fallbackTierID
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Using fallback tier_id from user/session: %s", tierID)
		default:
			tierID = GeminiTierGoogleOneFree
			logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Using default tier_id: %s", tierID)
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
		PlanName:     geminiPlanNameForToken(tierID, tokenExtra),
		TierID:       tierID,
		OAuthType:    oauthType,
		Extra:        tokenExtra,
	}
	if oauthType == "code_assist" && strings.TrimSpace(projectID) == "" {
		result.ProjectIDMissing = true
	}
	if oauthType == "google_one" && strings.TrimSpace(projectID) == "" {
		result.ProjectIDMissing = true
	}
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Final result - OAuth Type: %s, Project ID: %s, Tier ID: %s", result.OAuthType, result.ProjectID, result.TierID)
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] ========== ExchangeCode END ==========")
	return result, nil
}

func resolveGeminiOAuthRedirectURI(cfg geminicli.OAuthConfig, requestedRedirectURI string) string {
	if strings.TrimSpace(cfg.ClientID) == geminicli.GeminiCLIOAuthClientID {
		return geminicli.GeminiCLIRedirectURI
	}
	if trimmed := strings.TrimSpace(requestedRedirectURI); trimmed != "" {
		return trimmed
	}
	return geminicli.AIStudioOAuthRedirectURI
}

func (s *GeminiOAuthService) RefreshToken(ctx context.Context, oauthType, refreshToken, proxyURL string) (*GeminiTokenInfo, error) {
	oauthClient, err := s.requireOAuthClient()
	if err != nil {
		return nil, err
	}
	var lastErr error

	for attempt := 0; attempt <= 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		tokenResp, err := oauthClient.RefreshToken(ctx, oauthType, refreshToken, proxyURL)
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
	if account == nil || account.Platform != PlatformGemini || account.Type != AccountTypeOAuth {
		return nil, fmt.Errorf("account is not a Gemini OAuth account")
	}

	refreshToken := account.GetCredential("refresh_token")
	if strings.TrimSpace(refreshToken) == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	rawOAuthType := account.GetCredential("oauth_type")
	tierIDHint := account.GetCredential("tier_id")
	projectIDHint := strings.TrimSpace(account.GetCredential("project_id"))
	planNameHint := account.GetCredential("plan_name")
	oauthType := resolveGeminiOAuthType(rawOAuthType, tierIDHint, projectIDHint, planNameHint)
	if oauthType == "" {
		return nil, fmt.Errorf("missing oauth_type and unable to infer a supported Gemini OAuth flow from stored credentials; please re-authorize this account")
	}

	proxyURL, err := s.resolveProxyURL(ctx, account.ProxyID)
	if err != nil {
		return nil, err
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
	tokenInfo.PlanName = geminiPlanNameForToken(tokenInfo.TierID, tokenInfo.Extra)

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

	// For Google login / Code Assist, project metadata comes from Code Assist APIs.
	switch oauthType {
	case "code_assist":
		// 先设置默认值或保留旧值，确保 tier_id 始终有值
		if existingTierID != "" {
			tokenInfo.TierID = strings.TrimSpace(existingTierID)
		}
		if tokenInfo.TierID == "" {
			tokenInfo.TierID = "STANDARD"
		}

		// 尝试自动探测 project_id 和 tier_id
		needDetect := strings.TrimSpace(tokenInfo.ProjectID) == "" || tokenInfo.TierID == ""
		if needDetect {
			snapshot, detectErr := s.fetchProjectID(ctx, tokenInfo.AccessToken, proxyURL, existingProjectID)
			projectID := ""
			tierID := ""
			if snapshot != nil {
				projectID = snapshot.ProjectID
				tierID = snapshot.TierID
				tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, snapshot.Extra)
			}
			err := detectErr
			if err != nil {
				if isGeminiEligibilityError(err) {
					return nil, err
				}
				fmt.Printf("[GeminiOAuth] Warning: failed to auto-detect project/tier: %v\n", err)
			} else {
				if strings.TrimSpace(existingProjectID) != "" || strings.TrimSpace(projectID) != "" {
					tokenInfo.ProjectMetadataConfirmed = true
				}
				if strings.TrimSpace(tokenInfo.ProjectID) == "" && projectID != "" {
					tokenInfo.ProjectID = projectID
				}
				if strings.TrimSpace(tierID) != "" {
					tokenInfo.TierID = strings.TrimSpace(tierID)
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
		if s.codeAssist == nil {
			canonicalExistingTier := strings.TrimSpace(existingTierID)
			switch {
			case canonicalExistingTier != "":
				tokenInfo.TierID = canonicalExistingTier
			default:
				tokenInfo.TierID = GeminiTierGoogleOneFree
			}
			break
		}
		detectedTierID := ""
		if existingProjectID != "" {
			if snapshot, err := s.fetchCodeAssistSnapshot(ctx, tokenInfo.AccessToken, proxyURL, existingProjectID); err == nil {
				tokenInfo.ProjectMetadataConfirmed = true
				if strings.TrimSpace(snapshot.ProjectID) != "" {
					tokenInfo.ProjectID = strings.TrimSpace(snapshot.ProjectID)
				}
				detectedTierID = canonicalGeminiTierIDForOAuthType(oauthType, snapshot.TierID)
				tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, snapshot.Extra)
			} else {
				if isGeminiEligibilityError(err) {
					return nil, err
				}
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] WARNING: Failed to refresh Google login metadata: %v", err)
			}
		} else {
			snapshot, err := s.fetchProjectID(ctx, tokenInfo.AccessToken, proxyURL, "")
			if snapshot != nil {
				if strings.TrimSpace(snapshot.ProjectID) != "" {
					tokenInfo.ProjectMetadataConfirmed = true
					tokenInfo.ProjectID = strings.TrimSpace(snapshot.ProjectID)
				}
				detectedTierID = canonicalGeminiTierIDForOAuthType(oauthType, snapshot.TierID)
				tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, snapshot.Extra)
			}
			if err != nil {
				if isGeminiValidationOrIneligibleError(err) {
					return nil, err
				}
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] WARNING: Failed to auto-detect Google One metadata during refresh: %v", err)
			}
			if strings.TrimSpace(tokenInfo.ProjectID) == "" {
				tokenInfo.ProjectIDMissing = true
				tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, map[string]any{
					"auto_detect_project_id": "true",
				})
			}
		}
		canonicalExistingTier := strings.TrimSpace(existingTierID)
		switch {
		case detectedTierID != "":
			tokenInfo.TierID = detectedTierID
		case canonicalExistingTier != "":
			tokenInfo.TierID = canonicalExistingTier
		default:
			tokenInfo.TierID = GeminiTierGoogleOneFree
		}
	}
	tokenInfo.PlanName = geminiPlanNameForToken(tokenInfo.TierID, tokenInfo.Extra)

	if oauthType == "code_assist" && strings.TrimSpace(tokenInfo.ProjectID) != "" {
		quotaResp, quotaErr := s.retrieveGeminiUserQuota(ctx, tokenInfo.AccessToken, proxyURL, tokenInfo.ProjectID)
		switch quotaErr {
		case nil:
			tokenInfo.UsageRaw = quotaResp
			tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, buildGeminiQuotaExtra(quotaResp), clearGeminiQuotaErrorExtra())
			tokenInfo.Status = ""
			tokenInfo.StatusReason = ""
		default:
			tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, buildGeminiQuotaErrorExtra(quotaErr))
			if isGeminiForbiddenQuotaError(quotaErr) {
				tokenInfo.UsageRaw = nil
				tokenInfo.Extra = mergeGeminiCredentialExtra(tokenInfo.Extra, map[string]any{
					"gemini_usage_raw": nil,
					"usage_updated_at": "",
				})
				tokenInfo.Status = "forbidden"
				tokenInfo.StatusReason = quotaErr.Error()
			}
		}
	}

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
	} else {
		creds["gemini_status"] = ""
	}
	if tokenInfo.StatusReason != "" {
		creds["gemini_status_reason"] = tokenInfo.StatusReason
	} else {
		creds["gemini_status_reason"] = ""
	}
	if tokenInfo.ProjectMetadataConfirmed {
		creds["gemini_project_metadata_confirmed_at"] = time.Now().Format(time.RFC3339Nano)
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
		"gemini_current_tier_id",
		"gemini_current_tier_name",
		"gemini_current_tier_user_defined_project",
		"gemini_paid_tier_id",
		"gemini_paid_tier_name",
		"gemini_paid_tier_user_defined_project",
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
		if loadResp.CurrentTier.UserDefinedCloudAICompanionProject != nil {
			extra["gemini_current_tier_user_defined_project"] = *loadResp.CurrentTier.UserDefinedCloudAICompanionProject
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
		if loadResp.PaidTier.UserDefinedCloudAICompanionProject != nil {
			extra["gemini_paid_tier_user_defined_project"] = *loadResp.PaidTier.UserDefinedCloudAICompanionProject
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

func isGeminiForbiddenQuotaError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *geminicli.CodeAssistHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == 403
	}
	// Fallback for wrapped errors that lost the type.
	return strings.Contains(strings.ToLower(err.Error()), "status 403")
}

func clearGeminiQuotaErrorExtra() map[string]any {
	return map[string]any{
		"quota_query_last_error":    "",
		"quota_query_last_error_at": "",
	}
}

func (s *GeminiOAuthService) retrieveGeminiUserQuota(ctx context.Context, accessToken, proxyURL, projectID string) (map[string]any, error) {
	if s == nil || s.codeAssist == nil {
		return nil, errors.New("code assist client not configured")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("project_id is empty")
	}

	resp, err := s.codeAssist.RetrieveUserQuota(ctx, accessToken, proxyURL, &geminicli.RetrieveUserQuotaRequest{
		Project:   projectID,
		UserAgent: geminicli.GeminiCLIUserAgent,
	})
	if err != nil {
		return nil, err
	}

	// Marshal→Unmarshal round-trip converts the typed struct into a map[string]any
	// for storage in the credentials JSONB column. The typed fields in
	// RetrieveUserQuotaBucket (string, float64) survive this conversion faithfully.
	raw, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal retrieveUserQuota response failed: %w", err)
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("normalize retrieveUserQuota response failed: %w", err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func buildGeminiLoadCodeAssistRequest(projectID string) *geminicli.LoadCodeAssistRequest {
	trimmedProjectID := strings.TrimSpace(projectID)
	return &geminicli.LoadCodeAssistRequest{
		CloudAICompanionProject: trimmedProjectID,
		Metadata: geminicli.LoadCodeAssistMetadata{
			IDEType:       "IDE_UNSPECIFIED",
			Platform:      "PLATFORM_UNSPECIFIED",
			PluginType:    "GEMINI",
			DuetProject:   trimmedProjectID,
			UpdateChannel: "PREVIEW", // 启用 Preview Release Channel 以访问预览版模型
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

func geminiPlanNameForToken(tierID string, extra map[string]any) string {
	if extra != nil {
		if value, ok := extra["gemini_paid_tier_name"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
		if value, ok := extra["gemini_current_tier_name"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	if strings.TrimSpace(tierID) != "" {
		return strings.TrimSpace(tierID)
	}
	return ""
}

func (s *GeminiOAuthService) fetchCodeAssistSnapshot(ctx context.Context, accessToken, proxyURL, projectID string) (*geminiCodeAssistSnapshot, error) {
	if s.codeAssist == nil {
		return nil, errors.New("code assist client not configured")
	}

	loadResp, err := s.codeAssist.LoadCodeAssist(ctx, accessToken, proxyURL, buildGeminiLoadCodeAssistRequest(projectID))
	snapshot := &geminiCodeAssistSnapshot{Extra: buildGeminiCodeAssistExtra(loadResp)}
	if loadResp != nil {
		snapshot.ProjectID = strings.TrimSpace(loadResp.CloudAICompanionProject)
		snapshot.IneligibleTiers = append(snapshot.IneligibleTiers, loadResp.IneligibleTiers...)
		if loadResp.PaidTier != nil && strings.TrimSpace(loadResp.PaidTier.ID) != "" {
			snapshot.TierID = strings.TrimSpace(loadResp.PaidTier.ID)
			snapshot.TierRegistered = true
			snapshot.TierUserDefined = loadResp.PaidTier.UserDefinedCloudAICompanionProject
			snapshot.HasOnboarded = loadResp.PaidTier.HasOnboardedPreviously
		} else if loadResp.CurrentTier != nil && strings.TrimSpace(loadResp.CurrentTier.ID) != "" {
			snapshot.TierID = strings.TrimSpace(loadResp.CurrentTier.ID)
			snapshot.TierRegistered = true
			snapshot.TierUserDefined = loadResp.CurrentTier.UserDefinedCloudAICompanionProject
			snapshot.HasOnboarded = loadResp.CurrentTier.HasOnboardedPreviously
		} else {
			snapshot.TierID = extractTierIDFromAllowedTiers(loadResp.AllowedTiers)
		}
	}
	if err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func (s *GeminiOAuthService) fetchProjectID(ctx context.Context, accessToken, proxyURL string, projectIDHint string) (*geminiCodeAssistSnapshot, error) {
	trimmedProjectIDHint := strings.TrimSpace(projectIDHint)
	snapshot, loadErr := s.fetchCodeAssistSnapshot(ctx, accessToken, proxyURL, trimmedProjectIDHint)
	if snapshot == nil {
		snapshot = &geminiCodeAssistSnapshot{}
	}
	if len(snapshot.IneligibleTiers) > 0 {
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] Ineligible tiers returned by LoadCodeAssist: %s", summarizeGeminiIneligibleTiers(snapshot.IneligibleTiers))
	}
	if loadErr == nil {
		if validationErr := buildGeminiCodeAssistValidationError(snapshot.IneligibleTiers); validationErr != nil {
			return snapshot, validationErr
		}
	}
	if s.codeAssist == nil {
		if loadErr != nil {
			return snapshot, loadErr
		}
		return snapshot, errors.New("code assist client not configured")
	}

	// If LoadCodeAssist returned a project, use it
	if loadErr == nil && strings.TrimSpace(snapshot.ProjectID) != "" {
		return snapshot, nil
	}

	// 对齐 Gemini CLI：
	// 当 LoadCodeAssist 返回了 currentTier / paidTier（表示账号已注册）但没有返回 cloudaicompanionProject 时，
	// 如果用户已提供 project hint，则继续使用该 project；否则才要求用户手动提供。
	// 注意：仅 TierRegistered=true（来自 currentTier/paidTier）的才算已注册用户，
	// AllowedTiers-only 的新用户应继续走 onboardUser 流程。
	if loadErr == nil && snapshot.TierRegistered {
		registeredTierID := strings.TrimSpace(snapshot.TierID)
		if registeredTierID != "" {
			if trimmedProjectIDHint != "" {
				snapshot.ProjectID = trimmedProjectIDHint
				logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] User has tier (%s) but no cloudaicompanionProject; falling back to provided project_id hint: %s", registeredTierID, trimmedProjectIDHint)
				return snapshot, nil
			}
			logger.LegacyPrintf(
				"service.gemini_oauth",
				"[GeminiOAuth] User has tier (%s) but no cloudaicompanionProject; user_defined_project=%v has_onboarded_previously=%v",
				registeredTierID,
				boolPtrString(snapshot.TierUserDefined),
				boolPtrString(snapshot.HasOnboarded),
			)
			if ineligibleErr := buildGeminiCodeAssistIneligibleError(snapshot.IneligibleTiers); ineligibleErr != nil {
				return snapshot, ineligibleErr
			}
			return snapshot, buildGeminiCodeAssistProjectIDRequiredError(registeredTierID, snapshot.TierUserDefined)
		}
		if ineligibleErr := buildGeminiCodeAssistIneligibleError(snapshot.IneligibleTiers); ineligibleErr != nil {
			return snapshot, ineligibleErr
		}
	}

	// AllowedTiers-only 新用户也需要检查 ineligible tiers
	if loadErr == nil {
		if ineligibleErr := buildGeminiCodeAssistIneligibleError(snapshot.IneligibleTiers); ineligibleErr != nil {
			return snapshot, ineligibleErr
		}
	}

	// 未检测到 currentTier/paidTier，视为新用户，继续调用 onboardUser
	if strings.TrimSpace(snapshot.TierID) == "" {
		snapshot.TierID = "LEGACY"
	}
	logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] No currentTier/paidTier found, proceeding with onboardUser (tierID: %s)", snapshot.TierID)

	req := &geminicli.OnboardUserRequest{
		TierID:                  snapshot.TierID,
		CloudAICompanionProject: trimmedProjectIDHint,
		Metadata:                buildGeminiLoadCodeAssistRequest(trimmedProjectIDHint).Metadata,
	}

	resp, err := s.codeAssist.OnboardUser(ctx, accessToken, proxyURL, req)
	if err != nil {
		return snapshot, err
	}
	if projectID := extractGeminiCompanionProjectID(resp); projectID != "" {
		snapshot.ProjectID = projectID
		return snapshot, nil
	}
	if trimmedProjectIDHint != "" {
		snapshot.ProjectID = trimmedProjectIDHint
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] onboardUser returned no cloudaicompanionProject; falling back to provided project_id hint: %s", trimmedProjectIDHint)
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
	if trimmedProjectIDHint != "" {
		snapshot.ProjectID = trimmedProjectIDHint
		logger.LegacyPrintf("service.gemini_oauth", "[GeminiOAuth] onboardUser completed without cloudaicompanionProject; falling back to provided project_id hint: %s", trimmedProjectIDHint)
		return snapshot, nil
	}
	if resp != nil && resp.Done {
		return snapshot, buildGeminiCodeAssistProjectIDRequiredError(snapshot.TierID, snapshot.TierUserDefined)
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

func boolPtrString(value *bool) string {
	if value == nil {
		return "unknown"
	}
	if *value {
		return "true"
	}
	return "false"
}
