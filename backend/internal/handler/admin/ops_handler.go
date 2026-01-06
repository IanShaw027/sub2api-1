package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OpsHandler handles ops dashboard endpoints.
type OpsHandler struct {
	opsService *service.OpsService
}

func (h *OpsHandler) RequireOpsEnabled(c *gin.Context) {
	if h == nil || h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not initialized")
		c.Abort()
		return
	}
	if !h.opsService.IsOpsMonitoringEnabled(c.Request.Context()) {
		response.Error(c, http.StatusNotFound, "Ops monitoring is disabled")
		c.Abort()
		return
	}
	c.Next()
}

// NewOpsHandler creates a new OpsHandler.
func NewOpsHandler(opsService *service.OpsService) *OpsHandler {
	return &OpsHandler{opsService: opsService}
}
