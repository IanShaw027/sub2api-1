package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// IPSecurityBlock enforces active IP bans at the outermost HTTP ingress.
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
