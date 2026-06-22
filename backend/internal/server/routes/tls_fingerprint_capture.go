package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"

	"github.com/gin-gonic/gin"
)

// RegisterTLSFingerprintCaptureRoutes is intentionally empty.
//
// Raw ClientHello capture runs on the dedicated TLSFingerprintNativeCaptureListener
// so the gateway API no longer exposes a JSON submit endpoint.
func RegisterTLSFingerprintCaptureRoutes(v1 *gin.RouterGroup, h *handler.Handlers) {
	_, _ = v1, h
}
