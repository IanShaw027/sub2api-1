package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// IPSecurityBlock enforces active IP bans after forwarded-IP settings are snapshotted.
func IPSecurityBlock() gin.HandlerFunc {
	return func(c *gin.Context) {
		security := service.GlobalIPSecurityService()
		if security != nil && security.IsBlocked(c.Request.Context(), ip.GetTrustedClientIP(c)) {
			AbortWithError(c, 403, "IP_BLOCKED", "IP address blocked by security policy")
			return
		}
		c.Next()
	}
}

func observeIPSecurity(c *gin.Context, userID int64, source string, apiKeyID int64) {
	security := service.GlobalIPSecurityService()
	if security == nil || userID <= 0 {
		return
	}
	security.Observe(c.Request.Context(), service.IPSecurityActivity{
		IPAddress:    ip.GetTrustedClientIP(c),
		PeerIP:       ip.GetPeerIP(c),
		ForwardedFor: c.GetHeader("X-Forwarded-For"),
		UserID:       userID,
		Source:       source,
		APIKeyID:     apiKeyID,
		Method:       c.Request.Method,
		Path:         c.Request.URL.Path,
		RequestID:    c.GetString("request_id"),
	})
}
