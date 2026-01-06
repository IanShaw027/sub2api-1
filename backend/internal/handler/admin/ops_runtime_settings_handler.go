package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GetAlertRuntimeSettings returns Ops alert evaluator runtime settings (DB-backed).
// GET /api/admin/ops/runtime/alert
func (h *OpsHandler) GetAlertRuntimeSettings(c *gin.Context) {
	cfg, err := h.opsService.GetOpsAlertRuntimeSettings(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get alert runtime settings")
		return
	}
	response.Success(c, cfg)
}

// UpdateAlertRuntimeSettings updates Ops alert evaluator runtime settings (DB-backed).
// PUT /api/admin/ops/runtime/alert
func (h *OpsHandler) UpdateAlertRuntimeSettings(c *gin.Context) {
	var req service.OpsAlertRuntimeSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	updated, err := h.opsService.UpdateOpsAlertRuntimeSettings(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, updated)
}

// GetGroupAvailabilityRuntimeSettings returns group availability monitor runtime settings (DB-backed).
// GET /api/admin/ops/runtime/group-availability
func (h *OpsHandler) GetGroupAvailabilityRuntimeSettings(c *gin.Context) {
	cfg, err := h.opsService.GetOpsGroupAvailabilityRuntimeSettings(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get group availability runtime settings")
		return
	}
	response.Success(c, cfg)
}

// UpdateGroupAvailabilityRuntimeSettings updates group availability monitor runtime settings (DB-backed).
// PUT /api/admin/ops/runtime/group-availability
func (h *OpsHandler) UpdateGroupAvailabilityRuntimeSettings(c *gin.Context) {
	var req service.OpsGroupAvailabilityRuntimeSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	updated, err := h.opsService.UpdateOpsGroupAvailabilityRuntimeSettings(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, updated)
}

