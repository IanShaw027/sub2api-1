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
	clientIP := ip.GetTrustedClientIP(c)
	blocked, deniedIP, err := security.EnforcePinnedUserIP(c.Request.Context(), userID, clientIP)
	if err != nil {
		AbortWithError(c, 503, "IP_SECURITY_UNAVAILABLE", "IP security policy is temporarily unavailable")
		return
	}
	if blocked {
		msg := "Access denied. IP is not in this user's allowed exit list"
		if deniedIP != "" {
			msg = msg + " (" + deniedIP + ")"
		}
		AbortWithError(c, 403, "IP_NOT_ALLOWED", msg)
		return
	}
	security.Observe(c.Request.Context(), service.IPSecurityActivity{
		IPAddress:    clientIP,
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
