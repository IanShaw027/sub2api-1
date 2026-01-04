package admin

import (
	"context"
	"math"
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
	Enabled                *bool    `json:"enabled"`
	ThresholdMode          *string  `json:"threshold_mode"`
	MinAvailableAccounts   *int     `json:"min_available_accounts"`
	MinAvailablePercentage *float64 `json:"min_available_percentage"`
	NotifyEmail            *bool    `json:"notify_email"`
	Severity               *string  `json:"severity"`
	CooldownMinutes        *int     `json:"cooldown_minutes"`
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

	// Base config (defaults)
	base := &service.OpsGroupAvailabilityConfig{
		GroupID:                groupID,
		Enabled:                false,
		MinAvailableAccounts:   1,
		ThresholdMode:          "count",
		MinAvailablePercentage: 0,
		NotifyEmail:            true,
		Severity:               "warning",
		CooldownMinutes:        30,
	}
	if existing != nil {
		*base = *existing
	}

	// Apply patch fields
	if req.Enabled != nil {
		base.Enabled = *req.Enabled
	}
	if req.NotifyEmail != nil {
		base.NotifyEmail = *req.NotifyEmail
	}
	if req.Severity != nil {
		base.Severity = strings.TrimSpace(*req.Severity)
	}
	if req.CooldownMinutes != nil {
		base.CooldownMinutes = *req.CooldownMinutes
	}
	if req.MinAvailableAccounts != nil {
		base.MinAvailableAccounts = *req.MinAvailableAccounts
	}
	if req.MinAvailablePercentage != nil {
		base.MinAvailablePercentage = *req.MinAvailablePercentage
	}
	modeProvided := req.ThresholdMode != nil
	if req.ThresholdMode != nil {
		base.ThresholdMode = strings.ToLower(strings.TrimSpace(*req.ThresholdMode))
	}

	// Normalize & validate
	mode := strings.ToLower(strings.TrimSpace(base.ThresholdMode))
	if !modeProvided {
		// Allow "set one or both thresholds" even without explicitly providing threshold_mode:
		// infer mode from which thresholds are set.
		if base.MinAvailableAccounts > 0 && base.MinAvailablePercentage > 0 {
			mode = "both"
		} else if base.MinAvailablePercentage > 0 {
			mode = "percentage"
		} else {
			mode = "count"
		}
	}
	if mode == "" {
		mode = "count"
	}
	if mode != "count" && mode != "percentage" && mode != "both" {
		response.BadRequest(c, "Invalid threshold_mode (must be: count, percentage, both)")
		return
	}

	if base.CooldownMinutes < 0 {
		response.BadRequest(c, "cooldown_minutes must be >= 0")
		return
	}
	if base.MinAvailableAccounts < 0 {
		response.BadRequest(c, "min_available_accounts must be >= 0")
		return
	}
	if base.MinAvailablePercentage < 0 || base.MinAvailablePercentage > 100 {
		response.BadRequest(c, "min_available_percentage must be between 0 and 100")
		return
	}

	if base.Severity != "critical" && base.Severity != "warning" && base.Severity != "info" {
		response.BadRequest(c, "Invalid severity (must be: critical, warning, info)")
		return
	}

	// Enforce mode semantics
	switch mode {
	case "count":
		base.ThresholdMode = "count"
		base.MinAvailablePercentage = 0
		if base.MinAvailableAccounts < 1 {
			response.BadRequest(c, "min_available_accounts must be >= 1 for threshold_mode=count")
			return
		}
	case "percentage":
		base.ThresholdMode = "percentage"
		base.MinAvailableAccounts = 0
		if base.MinAvailablePercentage <= 0 {
			response.BadRequest(c, "min_available_percentage must be > 0 for threshold_mode=percentage")
			return
		}
	case "both":
		base.ThresholdMode = "both"
		if base.MinAvailableAccounts < 1 {
			response.BadRequest(c, "min_available_accounts must be >= 1 for threshold_mode=both")
			return
		}
		if base.MinAvailablePercentage <= 0 {
			response.BadRequest(c, "min_available_percentage must be > 0 for threshold_mode=both")
			return
		}
	}

	if existing != nil {
		base.ID = existing.ID
		if err := h.opsService.UpdateGroupAvailabilityConfig(c.Request.Context(), base); err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to update config")
			return
		}
	} else {
		if err := h.opsService.CreateGroupAvailabilityConfig(c.Request.Context(), base); err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to create config")
			return
		}
	}

	response.Success(c, base)
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

// ListStatus returns availability status for all active groups.
// GET /api/admin/ops/group-availability/status?search=xxx&monitoring=enabled&alert=firing&page=1&page_size=20
func (h *OpsGroupAvailabilityHandler) ListStatus(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	search := strings.TrimSpace(c.Query("search"))
	monitoringFilter := strings.ToLower(strings.TrimSpace(c.Query("monitoring")))
	alertFilter := strings.ToLower(strings.TrimSpace(c.Query("alert")))

	page := 1
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	pageSize := 20
	if ps := c.Query("page_size"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
			pageSize = parsed
		}
	}

	groups, err := h.groupService.ListActive(ctx)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list groups")
		return
	}

	configs, err := h.opsService.ListGroupAvailabilityConfigs(ctx, false)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list configs")
		return
	}

	configByGroupID := make(map[int64]*service.OpsGroupAvailabilityConfig, len(configs))
	for i := range configs {
		configByGroupID[configs[i].GroupID] = &configs[i]
	}

	// Compute all statuses
	allStatuses := make([]groupAvailabilityStatusResponse, 0, len(groups))
	for i := range groups {
		group := groups[i]
		cfg := configByGroupID[group.ID]

		status, err := h.computeStatusWithGroup(ctx, &group, cfg)
		if err != nil {
			continue
		}
		allStatuses = append(allStatuses, groupAvailabilityStatusResponse{
			OpsGroupAvailabilityStatus: *status,
			Config:                     cfg,
		})
	}

	// Apply filters
	filtered := make([]groupAvailabilityStatusResponse, 0, len(allStatuses))
	for _, status := range allStatuses {
		// Search filter (fuzzy match on group name)
		if search != "" {
			if !strings.Contains(strings.ToLower(status.GroupName), strings.ToLower(search)) {
				continue
			}
		}

		// Monitoring status filter
		if monitoringFilter != "" && monitoringFilter != "all" {
			if monitoringFilter == "enabled" && !status.MonitoringEnabled {
				continue
			}
			if monitoringFilter == "disabled" && status.MonitoringEnabled {
				continue
			}
		}

		// Alert status filter
		if alertFilter != "" && alertFilter != "all" {
			if alertFilter == "ok" && status.AlertStatus != "ok" {
				continue
			}
			if alertFilter == "firing" && status.AlertStatus != "firing" {
				continue
			}
		}

		filtered = append(filtered, status)
	}

	// Calculate pagination
	total := len(filtered)
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		start = 0
		end = 0
	}
	if end > total {
		end = total
	}

	result := filtered[start:end]

	response.Success(c, gin.H{
		"items":       result,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	})
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

	group, err := h.groupService.GetByID(c.Request.Context(), groupID)
	if err != nil || group == nil {
		response.Error(c, http.StatusNotFound, "Group not found")
		return
	}

	status, err := h.computeStatusWithGroup(c.Request.Context(), group, config)
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
	if config == nil {
		return nil, nil
	}
	group, err := h.groupService.GetByID(ctx, config.GroupID)
	if err != nil {
		return nil, err
	}
	return h.computeStatusWithGroup(ctx, group, config)
}

func (h *OpsGroupAvailabilityHandler) computeStatusWithGroup(ctx context.Context, group *service.Group, config *service.OpsGroupAvailabilityConfig) (*service.OpsGroupAvailabilityStatus, error) {
	if group == nil {
		return nil, nil
	}

	groupID := group.ID
	available, total, err := h.opsService.CountAvailableAccountsByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}

	disabled := 0
	errorAccounts := 0
	overload := 0

	monitoringEnabled := false
	minAvailable := 0
	thresholdMode := ""
	minAvailablePercentage := 0.0
	if config != nil {
		monitoringEnabled = config.Enabled
		minAvailable = config.MinAvailableAccounts
		thresholdMode = strings.ToLower(strings.TrimSpace(config.ThresholdMode))
		minAvailablePercentage = config.MinAvailablePercentage
	}

	isHealthy := true
	if config != nil {
		isHealthy = evaluateGroupAvailabilityHealthy(thresholdMode, available, total, minAvailable, minAvailablePercentage)
	}
	alertStatus := "ok"
	if config != nil && !isHealthy {
		alertStatus = "firing"
	}

	var event *service.OpsGroupAvailabilityEvent
	if config != nil && config.ID != 0 {
		event, _ = h.opsService.GetLatestGroupAvailabilityEvent(ctx, config.ID)
	}

	status := &service.OpsGroupAvailabilityStatus{
		GroupID:   groupID,
		GroupName: group.Name,
		Platform:  group.Platform,

		TotalAccounts:     total,
		AvailableAccounts: available,
		DisabledAccounts:  disabled,
		ErrorAccounts:     errorAccounts,
		OverloadAccounts:  overload,

		MonitoringEnabled:      monitoringEnabled,
		MinAvailableAccounts:   minAvailable,
		ThresholdMode:          thresholdMode,
		MinAvailablePercentage: minAvailablePercentage,

		IsHealthy:   isHealthy,
		AlertStatus: alertStatus,
	}

	if event != nil {
		status.LastAlertAt = &event.CreatedAt
	}

	return status, nil
}

func evaluateGroupAvailabilityHealthy(mode string, available, total, minAccounts int, minPercentage float64) bool {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		m = "count"
	}
	currentPercent := 0.0
	if total > 0 {
		currentPercent = (float64(available) / float64(total)) * 100
	}

	countOk := minAccounts <= 0 || available >= minAccounts
	percentOk := minPercentage <= 0 || currentPercent >= minPercentage

	switch m {
	case "percentage":
		return percentOk
	case "both":
		return countOk && percentOk
	default:
		return countOk
	}
}

func groupAvailabilityThresholdAccounts(mode string, total int, minAccounts int, minPercentage float64) int {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		m = "count"
	}
	requiredFromPercent := 0
	if total > 0 && minPercentage > 0 {
		requiredFromPercent = int(math.Ceil(float64(total) * minPercentage / 100))
	}
	switch m {
	case "percentage":
		return requiredFromPercent
	case "both":
		if requiredFromPercent > minAccounts {
			return requiredFromPercent
		}
		return minAccounts
	default:
		return minAccounts
	}
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
