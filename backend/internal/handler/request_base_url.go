package handler

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// requestBaseURL derives the inbound request's base URL (scheme://host) from
// the request object without trusting client-supplied forwarding headers. It is
// used to build backend-proxied signed download links that point back at the
// host the caller actually reached, rather than a statically configured
// object-storage domain.
func requestBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}

	host := strings.TrimSpace(c.Request.Host)

	if !isForwardedHostSafe(host) {
		return ""
	}
	return scheme + "://" + host
}

func contextWithRequestBaseURL(ctx context.Context, c *gin.Context) context.Context {
	return service.WithRequestBaseURL(ctx, requestBaseURL(c))
}

func firstForwardedValue(raw string) string {
	if raw == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(raw, ",")[0])
}

func isHTTPForwardedScheme(scheme string) bool {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "http", "https":
		return true
	default:
		return false
	}
}

func isForwardedHostSafe(host string) bool {
	host = strings.TrimSpace(host)
	return host != "" && !strings.ContainsAny(host, " \t\r\n/\\")
}
