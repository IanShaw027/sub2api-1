package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

const tlsFingerprintCaptureSubmitMaxBodySize = 256 * 1024

// RegisterTLSFingerprintCaptureRoutes registers token-protected collector submit endpoints.
func RegisterTLSFingerprintCaptureRoutes(v1 *gin.RouterGroup, h *handler.Handlers) {
	captures := v1.Group("/tls-fingerprint-captures", middleware.RequestBodyLimit(tlsFingerprintCaptureSubmitMaxBodySize))
	{
		captures.POST("/submit", h.Admin.TLSFingerprintProfile.SubmitCapture)
	}
}
