package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OpsGroupAvailabilityHandler handles group availability monitoring endpoints.
type OpsGroupAvailabilityHandler struct {
	opsService   *service.OpsService
	groupService *service.GroupService
}

// NewOpsGroupAvailabilityHandler creates a new OpsGroupAvailabilityHandler.
func NewOpsGroupAvailabilityHandler(
	opsService *service.OpsService,
	groupService *service.GroupService,
) *OpsGroupAvailabilityHandler {
	return &OpsGroupAvailabilityHandler{
		opsService:   opsService,
		groupService: groupService,
	}
}

// GroupAvailabilityConfigRequest represents the request body for creating/updating config.
type GroupAvailabilityConfigRequest struct {
	Enabled              bool   `json:"enabled"`
	MinAvailableAccounts int    `json:"min_available_accounts" binding:"required,min=1"`
	NotifyEmail          bool   `json:"notify_email"`
	Severity             string `json:"severity" binding:"required,oneof=critical warning info"`
	CooldownMinutes      int    `json:"cooldown_minutes" binding:"min=0"`
}

// GroupAvailabilityConfigResponse represents the response with group info.
type GroupAvailabilityConfigResponse struct {
	service.OpsGroupAvailabilityConfig
	Group *service.Group `json:"group"`
}

type groupAvailabilityStatusResponse struct {
	service.OpsGroupAvailabilityStatus
	Config *service.OpsGroupAvailabilityConfig `json:"config,omitempty"`
}

// ListConfigs returns all group availability monitoring configs.
// GET /api/admin/ops/group-availability/configs
func (h *OpsGroupAvailabilityHandler) ListConfigs(c *gin.Context) {
	configs, err := h.opsService.ListGroupAvailabilityConfigs(c.Request.Context(), false)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list configs")
		return
	}

	result := make([]GroupAvailabilityConfigResponse, 0, len(configs))
	for i := range configs {
		group, err := h.groupService.GetByID(c.Request.Context(), configs[i].GroupID)
		if err != nil {
			continue
		}
		result = append(result, GroupAvailabilityConfigResponse{
			OpsGroupAvailabilityConfig: configs[i],
			Group:                      group,
		})
	}

	response.Success(c, result)
}

// GetConfig returns a single group's monitoring config.
// GET /api/admin/ops/group-availability/configs/:groupId
func (h *OpsGroupAvailabilityHandler) GetConfig(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("groupId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	group, err := h.groupService.GetByID(c.Request.Context(), groupID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Group not found")
		return
	}

	config, err := h.opsService.GetGroupAvailabilityConfig(c.Request.Context(), groupID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.Error(c, http.StatusNotFound, "Config not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to get config")
		}
		return
	}
	if config == nil {
		response.Error(c, http.StatusNotFound, "Config not found")
		return
	}

	response.Success(c, GroupAvailabilityConfigResponse{
		OpsGroupAvailabilityConfig: *config,
		Group:                      group,
	})
}

// UpsertConfig creates or updates a group's monitoring config.
// PUT /api/admin/ops/group-availability/configs/:groupId
func (h *OpsGroupAvailabilityHandler) UpsertConfig(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("groupId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	var req GroupAvailabilityConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if _, err := h.groupService.GetByID(c.Request.Context(), groupID); err != nil {
		response.Error(c, http.StatusNotFound, "Group not found")
		return
	}

	existing, _ := h.opsService.GetGroupAvailabilityConfig(c.Request.Context(), groupID)

	config := &service.OpsGroupAvailabilityConfig{
		GroupID:              groupID,
		Enabled:              req.Enabled,
		MinAvailableAccounts: req.MinAvailableAccounts,
		NotifyEmail:          req.NotifyEmail,
		Severity:             req.Severity,
		CooldownMinutes:      req.CooldownMinutes,
	}

	if existing != nil {
		config.ID = existing.ID
		if err := h.opsService.UpdateGroupAvailabilityConfig(c.Request.Context(), config); err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to update config")
			return
		}
	} else {
		if err := h.opsService.CreateGroupAvailabilityConfig(c.Request.Context(), config); err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to create config")
			return
		}
	}

	response.Success(c, config)
}

// DeleteConfig deletes a group's monitoring config.
// DELETE /api/admin/ops/group-availability/configs/:groupId
func (h *OpsGroupAvailabilityHandler) DeleteConfig(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("groupId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	if err := h.opsService.DeleteGroupAvailabilityConfig(c.Request.Context(), groupID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.Error(c, http.StatusNotFound, "Config not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to delete config")
		}
		return
	}

	response.Success(c, nil)
}

// ListStatus returns availability status for all groups with monitoring enabled.
// GET /api/admin/ops/group-availability/status
func (h *OpsGroupAvailabilityHandler) ListStatus(c *gin.Context) {
	configs, err := h.opsService.ListGroupAvailabilityConfigs(c.Request.Context(), true)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list configs")
		return
	}

	result := make([]groupAvailabilityStatusResponse, 0, len(configs))
	for i := range configs {
		status, err := h.computeStatus(c.Request.Context(), &configs[i])
		if err != nil {
			continue
		}
		result = append(result, groupAvailabilityStatusResponse{
			OpsGroupAvailabilityStatus: *status,
			Config:                     &configs[i],
		})
	}

	response.Success(c, result)
}

// GetStatus returns availability status for a single group.
// GET /api/admin/ops/group-availability/status/:groupId
func (h *OpsGroupAvailabilityHandler) GetStatus(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("groupId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid group ID")
		return
	}

	config, err := h.opsService.GetGroupAvailabilityConfig(c.Request.Context(), groupID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.Error(c, http.StatusNotFound, "Config not found")
		} else {
			response.Error(c, http.StatusInternalServerError, "Failed to get config")
		}
		return
	}
	if config == nil {
		response.Error(c, http.StatusNotFound, "Config not found")
		return
	}

	status, err := h.computeStatus(c.Request.Context(), config)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to compute status")
		return
	}

	response.Success(c, groupAvailabilityStatusResponse{
		OpsGroupAvailabilityStatus: *status,
		Config:                     config,
	})
}

// ListEvents returns alert event history.
// GET /api/admin/ops/group-availability/events
func (h *OpsGroupAvailabilityHandler) ListEvents(c *gin.Context) {
	limit := 50
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	status := c.DefaultQuery("status", "all")
	if status != "all" && status != "firing" && status != "resolved" {
		response.BadRequest(c, "Invalid status (must be: firing, resolved, all)")
		return
	}

	statusFilter := status
	if status == "all" {
		statusFilter = ""
	}
	events, err := h.opsService.ListGroupAvailabilityEvents(c.Request.Context(), limit, statusFilter)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list events")
		return
	}

	response.Success(c, events)
}

// computeStatus calculates the current availability status for a group.
func (h *OpsGroupAvailabilityHandler) computeStatus(ctx context.Context, config *service.OpsGroupAvailabilityConfig) (*service.OpsGroupAvailabilityStatus, error) {
	group, err := h.groupService.GetByID(ctx, config.GroupID)
	if err != nil {
		return nil, err
	}

	available, total, err := h.opsService.CountAvailableAccountsByGroup(ctx, config.GroupID)
	if err != nil {
		return nil, err
	}

	disabled := 0
	errorAccounts := 0
	overload := 0

	isHealthy := available >= config.MinAvailableAccounts
	alertStatus := "ok"
	if !isHealthy {
		alertStatus = "firing"
	}

	event, _ := h.opsService.GetLatestGroupAvailabilityEvent(ctx, config.ID)

	status := &service.OpsGroupAvailabilityStatus{
		GroupID:   config.GroupID,
		GroupName: group.Name,
		Platform:  group.Platform,

		TotalAccounts:     total,
		AvailableAccounts: available,
		DisabledAccounts:  disabled,
		ErrorAccounts:     errorAccounts,
		OverloadAccounts:  overload,

		MonitoringEnabled:    config.Enabled,
		MinAvailableAccounts: config.MinAvailableAccounts,

		IsHealthy:   isHealthy,
		AlertStatus: alertStatus,
	}

	if event != nil {
		status.LastAlertAt = &event.CreatedAt
	}

	return status, nil
}

// Router registration (add to backend/internal/server/router.go):
//
// groupAvailHandler := admin.NewOpsGroupAvailabilityHandler(opsService, groupService)
// opsGroup := adminGroup.Group("/ops/group-availability")
// {
//     opsGroup.GET("/configs", groupAvailHandler.ListConfigs)
//     opsGroup.GET("/configs/:groupId", groupAvailHandler.GetConfig)
//     opsGroup.PUT("/configs/:groupId", groupAvailHandler.UpsertConfig)
//     opsGroup.DELETE("/configs/:groupId", groupAvailHandler.DeleteConfig)
//     opsGroup.GET("/status", groupAvailHandler.ListStatus)
//     opsGroup.GET("/status/:groupId", groupAvailHandler.GetStatus)
//     opsGroup.GET("/events", groupAvailHandler.ListEvents)
// }
