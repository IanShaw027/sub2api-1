package admin

import (
	"errors"
	"io"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type KiroOAuthHandler struct {
	oauthService *service.KiroOAuthService
	refresher    *service.KiroTokenRefresher
}

func NewKiroOAuthHandler(oauthService *service.KiroOAuthService, refresher *service.KiroTokenRefresher) *KiroOAuthHandler {
	return &KiroOAuthHandler{
		oauthService: oauthService,
		refresher:    refresher,
	}
}

type KiroGenerateAuthURLRequest struct {
	ProxyID *int64 `json:"proxy_id"`
}

func (h *KiroOAuthHandler) GenerateAuthURL(c *gin.Context) {
	var req KiroGenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if !errors.Is(err, io.EOF) {
			response.BadRequest(c, "请求无效: "+err.Error())
			return
		}
	}
	if h == nil || h.oauthService == nil {
		response.InternalError(c, "Kiro OAuth service is not configured")
		return
	}

	result, err := h.oauthService.GenerateAuthURL(c.Request.Context(), req.ProxyID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

type KiroExchangeCallbackRequest struct {
	SessionID   string   `json:"session_id" binding:"required"`
	CallbackURL string   `json:"callback_url" binding:"required"`
	ProxyID     *int64   `json:"proxy_id"`
	StartURL    string   `json:"start_url"`
	IssuerURL   string   `json:"issuer_url"`
	IDCRegion   string   `json:"idc_region"`
	Scopes      []string `json:"scopes"`
	LoginHint   string   `json:"login_hint"`
	ClientName  string   `json:"client_name"`
}

func (h *KiroOAuthHandler) ExchangeCallback(c *gin.Context) {
	var req KiroExchangeCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}
	if h == nil || h.oauthService == nil {
		response.InternalError(c, "Kiro OAuth service is not configured")
		return
	}

	result, err := h.oauthService.ExchangeCallbackOrStartContinuation(c.Request.Context(), &service.KiroExchangeCallbackInput{
		SessionID:   strings.TrimSpace(req.SessionID),
		CallbackURL: strings.TrimSpace(req.CallbackURL),
		ProxyID:     req.ProxyID,
		StartURL:    strings.TrimSpace(req.StartURL),
		IssuerURL:   strings.TrimSpace(req.IssuerURL),
		IDCRegion:   strings.TrimSpace(req.IDCRegion),
		Scopes:      req.Scopes,
		LoginHint:   strings.TrimSpace(req.LoginHint),
		ClientName:  strings.TrimSpace(req.ClientName),
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, kiroExchangeCallbackResponseData(result))
}

type KiroDeviceCompleteRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	ProxyID   *int64 `json:"proxy_id"`
}

func (h *KiroOAuthHandler) DeviceComplete(c *gin.Context) {
	var req KiroDeviceCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}
	if h == nil || h.oauthService == nil {
		response.InternalError(c, "Kiro OAuth service is not configured")
		return
	}

	result, err := h.oauthService.CompleteDeviceAuthorization(c.Request.Context(), &service.KiroDeviceCompleteInput{
		SessionID: strings.TrimSpace(req.SessionID),
		ProxyID:   req.ProxyID,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, result)
}

type KiroRefreshTokenRequest struct {
	Credentials map[string]any `json:"credentials" binding:"required"`
	Extra       map[string]any `json:"extra"`
	ProxyID     *int64         `json:"proxy_id"`
}

// RefreshToken validates Kiro credentials through the upstream refresh path and
// returns the refreshed credential set that should be written back.
// POST /api/v1/admin/kiro/oauth/refresh-token
func (h *KiroOAuthHandler) RefreshToken(c *gin.Context) {
	var req KiroRefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求无效: "+err.Error())
		return
	}
	if h == nil || h.refresher == nil {
		response.InternalError(c, "Kiro OAuth refresh service is not configured")
		return
	}

	req.Credentials = service.NormalizeKiroOAuthCredentialShape(req.Credentials)
	refreshToken := strings.TrimSpace(stringCredentialValue(req.Credentials, "refresh_token"))
	if err := service.ValidateKiroRefreshTokenHealth(refreshToken); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	authMethod := service.NormalizeKiroAuthMethod(req.Credentials)
	if service.KiroAuthMethodUsesIDCRefresh(authMethod) {
		clientID := strings.TrimSpace(stringCredentialValue(req.Credentials, "client_id"))
		clientSecret := strings.TrimSpace(stringCredentialValue(req.Credentials, "client_secret"))
		if clientID == "" || clientSecret == "" {
			response.BadRequest(c, "kiro idc client_id and client_secret are required")
			return
		}
	}

	account := &service.Account{
		Platform:    service.PlatformKiro,
		Type:        service.AccountTypeOAuth,
		Credentials: req.Credentials,
		Extra:       req.Extra,
		ProxyID:     req.ProxyID,
		Concurrency: 1,
	}

	credentials, err := h.refresher.Refresh(c.Request.Context(), account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.oauthService != nil {
		account.Credentials = credentials
		credentials, err = h.oauthService.ValidateAndEnrichRefreshedCredentialsForAccount(c.Request.Context(), account, credentials)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	response.Success(c, credentials)
}

func stringCredentialValue(credentials map[string]any, key string) string {
	if credentials == nil {
		return ""
	}
	value, ok := credentials[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func kiroExchangeCallbackResponseData(result *service.KiroOAuthProgressResult) any {
	if result != nil && result.TokenInfo != nil && result.Continuation == nil {
		return result.TokenInfo
	}
	return result
}
