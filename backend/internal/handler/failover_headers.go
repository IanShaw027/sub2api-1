package handler

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func copyFailoverResponseHeaders(c *gin.Context, failoverErr *service.UpstreamFailoverError) {
	if c == nil || failoverErr == nil || c.Writer.Written() {
		return
	}
	if failoverErr.ResponseHeaders != nil {
		for _, key := range []string{
			"Retry-After",
			"Www-Authenticate",
			"x-request-id",
			"x-amzn-requestid",
			"x-amzn-request-id",
			"x-amzn-trace-id",
			"x-amzn-errortype",
			"x-amz-cf-id",
			"cf-ray",
			"cf-mitigated",
		} {
			if value := strings.TrimSpace(failoverErr.ResponseHeaders.Get(key)); value != "" {
				c.Header(key, value)
			}
		}
		if c.Writer.Header().Get("x-request-id") == "" {
			if requestID := strings.TrimSpace(failoverErr.ResponseHeaders.Get("x-amzn-requestid")); requestID != "" {
				c.Header("x-request-id", requestID)
			}
		}
	}
	if failoverErr.StatusCode == http.StatusTooManyRequests && c.Writer.Header().Get("Retry-After") == "" {
		c.Header("Retry-After", "5")
	}
}
