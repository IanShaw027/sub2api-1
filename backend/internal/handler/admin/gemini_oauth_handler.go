package admin

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type GeminiOAuthHandler struct {
	geminiOAuthService *service.GeminiOAuthService
}

func NewGeminiOAuthHandler(geminiOAuthService *service.GeminiOAuthService) *GeminiOAuthHandler {
	return &GeminiOAuthHandler{geminiOAuthService: geminiOAuthService}
}

// GetCapabilities returns the Gemini OAuth configuration capabilities.
// GET /api/v1/admin/gemini/oauth/capabilities
func (h *GeminiOAuthHandler) GetCapabilities(c *gin.Context) {
	cfg := h.geminiOAuthService.GetOAuthConfig()
	response.Success(c, cfg)
}

type GeminiGenerateAuthURLRequest struct {
	ProxyID       *int64 `json:"proxy_id"`
	ProjectID     string `json:"project_id"`
	ProjectIDHint string `json:"project_id_hint"`
	// OAuth 类型: "code_assist" 或 "google_one"
	OAuthType string `json:"oauth_type"`
	// TierID is an optional legacy fallback tier.
	// Current frontend flow no longer sends tier_id before auth, but the backend still accepts it
	// and stores it in the OAuth session for compatibility with older clients.
	TierID string `json:"tier_id"`
}

// GenerateAuthURL generates Google OAuth authorization URL for Gemini.
// POST /api/v1/admin/gemini/oauth/auth-url
func (h *GeminiOAuthHandler) GenerateAuthURL(c *gin.Context) {
	var req GeminiGenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	oauthType := normalizeGeminiOAuthTypeAlias(req.OAuthType)
	if oauthType == "" {
		oauthType = "code_assist"
	}
	if oauthType != "code_assist" && oauthType != "google_one" {
		response.BadRequest(c, "Invalid oauth_type: must be 'code_assist' or 'google_one'")
		return
	}

	// Always pass the "hosted" callback URI; the OAuth service may override it depending on
	// oauth_type and whether the built-in Gemini CLI OAuth client is used.
	redirectURI := deriveGeminiRedirectURI(c)
	projectIDHint := strings.TrimSpace(req.ProjectIDHint)
	if projectIDHint == "" {
		projectIDHint = strings.TrimSpace(req.ProjectID)
	}

	result, err := h.geminiOAuthService.GenerateAuthURL(c.Request.Context(), req.ProxyID, redirectURI, projectIDHint, oauthType, req.TierID)
	if err != nil {
		msg := err.Error()
		// Treat missing/invalid OAuth client configuration as a user/config error.
		if strings.Contains(msg, "OAuth client not configured") ||
			strings.Contains(msg, "requires your own OAuth Client") ||
			strings.Contains(msg, "requires a custom OAuth Client") ||
			strings.Contains(msg, "GEMINI_CLI_OAUTH_CLIENT_SECRET_MISSING") ||
			strings.Contains(msg, "built-in Gemini CLI OAuth client_secret is not configured") {
			response.BadRequest(c, "Failed to generate auth URL: "+msg)
			return
		}
		response.InternalError(c, "Failed to generate auth URL: "+msg)
		return
	}

	response.Success(c, result)
}

type GeminiExchangeCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	State     string `json:"state" binding:"required"`
	Code      string `json:"code" binding:"required"`
	ProxyID   *int64 `json:"proxy_id"`
	// OAuth 类型: "code_assist" 或 "google_one"，需要与 GenerateAuthURL 时的类型一致
	OAuthType string `json:"oauth_type"`
	// TierID is an optional legacy fallback tier.
	// Current frontend flow omits tier_id here; when absent, the backend reuses the tier stored in
	// the OAuth session by GenerateAuthURL, while still accepting explicit tier_id from older clients.
	TierID string `json:"tier_id"`
}

// ExchangeCode exchanges authorization code for tokens.
// POST /api/v1/admin/gemini/oauth/exchange-code
func (h *GeminiOAuthHandler) ExchangeCode(c *gin.Context) {
	var req GeminiExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	oauthType := normalizeGeminiOAuthTypeAlias(req.OAuthType)
	if oauthType == "" {
		oauthType = "code_assist"
	}
	if oauthType != "code_assist" && oauthType != "google_one" {
		response.BadRequest(c, "Invalid oauth_type: must be 'code_assist' or 'google_one'")
		return
	}

	tokenInfo, err := h.geminiOAuthService.ExchangeCode(c.Request.Context(), &service.GeminiExchangeCodeInput{
		SessionID: req.SessionID,
		State:     req.State,
		Code:      req.Code,
		ProxyID:   req.ProxyID,
		OAuthType: oauthType,
		TierID:    req.TierID,
	})
	if err != nil {
		response.BadRequest(c, "Failed to exchange code: "+err.Error())
		return
	}

	response.Success(c, tokenInfo)
}

func normalizeGeminiOAuthTypeAlias(oauthType string) string {
	switch strings.ToLower(strings.TrimSpace(oauthType)) {
	case "":
		return ""
	case "ai_studio":
		return "google_one"
	case "code_assist", "google_one":
		return strings.ToLower(strings.TrimSpace(oauthType))
	default:
		return ""
	}
}

func deriveGeminiRedirectURI(c *gin.Context) string {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin != "" {
		return strings.TrimRight(origin, "/") + "/auth/callback"
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xfProto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); xfProto != "" {
		scheme = strings.TrimSpace(strings.Split(xfProto, ",")[0])
	}

	host := strings.TrimSpace(c.Request.Host)
	if xfHost := strings.TrimSpace(c.GetHeader("X-Forwarded-Host")); xfHost != "" {
		host = strings.TrimSpace(strings.Split(xfHost, ",")[0])
	}

	return fmt.Sprintf("%s://%s/auth/callback", scheme, host)
}
