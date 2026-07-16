package handler

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	refreshTokenCookieName = "sub2api_refresh_token"
	refreshRequestHeader   = "X-Sub2API-Refresh"
)

func (h *AuthHandler) refreshCookieSameSite() http.SameSite {
	if h != nil && h.cfg != nil {
		switch strings.ToLower(strings.TrimSpace(h.cfg.JWT.RefreshCookieSameSite)) {
		case "strict":
			return http.SameSiteStrictMode
		case "none":
			return http.SameSiteNoneMode
		}
	}
	return http.SameSiteLaxMode
}

func (h *AuthHandler) refreshCookiePath() string {
	if h != nil && h.cfg != nil {
		if path := strings.TrimSpace(h.cfg.JWT.RefreshCookiePath); strings.HasPrefix(path, "/") {
			return path
		}
	}
	return "/api/v1/auth"
}

func (h *AuthHandler) setRefreshTokenCookie(c *gin.Context, refreshToken string) {
	if c == nil || strings.TrimSpace(refreshToken) == "" {
		return
	}
	maxAge := 30 * 24 * 60 * 60
	if h != nil && h.cfg != nil && h.cfg.JWT.RefreshTokenExpireDays > 0 {
		maxAge = h.cfg.JWT.RefreshTokenExpireDays * 24 * 60 * 60
	}
	sameSite := h.refreshCookieSameSite()
	secure := isRequestHTTPS(c) || sameSite == http.SameSiteNoneMode
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    refreshToken,
		Path:     h.refreshCookiePath(),
		MaxAge:   maxAge,
		Expires:  time.Now().UTC().Add(time.Duration(maxAge) * time.Second),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func (h *AuthHandler) clearRefreshTokenCookie(c *gin.Context) {
	if c == nil {
		return
	}
	sameSite := h.refreshCookieSameSite()
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		Path:     h.refreshCookiePath(),
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		HttpOnly: true,
		Secure:   isRequestHTTPS(c) || sameSite == http.SameSiteNoneMode,
		SameSite: sameSite,
	})
}

func readRefreshTokenCookie(c *gin.Context) string {
	if c == nil {
		return ""
	}
	cookie, err := c.Request.Cookie(refreshTokenCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func usesRefreshCookieMode(c *gin.Context) bool {
	return c != nil && strings.TrimSpace(c.GetHeader(refreshRequestHeader)) == "1"
}

func (h *AuthHandler) validateRefreshCookieRequest(c *gin.Context) bool {
	if !usesRefreshCookieMode(c) {
		return false
	}
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		// The non-simple custom header is the CSRF barrier for same-origin and
		// non-browser clients that omit Origin.
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	normalizedOrigin := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
	if requestOrigin := strings.ToLower(strings.TrimSpace(requestBaseURL(c))); requestOrigin != "" && normalizedOrigin == requestOrigin {
		return true
	}
	if h == nil || h.cfg == nil || len(h.cfg.CORS.AllowedOrigins) == 0 {
		// Preserve compatibility for deployments that do not configure CORS
		// here (for example, when an upstream proxy owns CORS enforcement).
		// The custom refresh header remains the CSRF barrier.
		return true
	}
	for _, allowed := range h.cfg.CORS.AllowedOrigins {
		if strings.EqualFold(strings.TrimSpace(allowed), normalizedOrigin) {
			return true
		}
	}
	return false
}
