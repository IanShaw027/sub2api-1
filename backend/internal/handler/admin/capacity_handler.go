// Package admin provides HTTP handlers for administrative operations.
package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CapacityHandler exposes the generic (multi-platform) capacity forecasting
// engine: a read-only timeseries endpoint for the admin capacity chart, and a
// write endpoint that batch-refreshes ("probes"/测活) upstream quota for a
// scope of accounts. It is intentionally a standalone handler (not folded
// into the already very large AccountHandler) since it depends only on
// CapacityForecastService.
type CapacityHandler struct {
	capacityService *service.CapacityForecastService
}

// NewCapacityHandler constructs the capacity forecasting handler.
func NewCapacityHandler(capacityService *service.CapacityForecastService) *CapacityHandler {
	return &CapacityHandler{capacityService: capacityService}
}

// capacityProbeRequest is the POST .../capacity/probe request body.
type capacityProbeRequest struct {
	Platform      string `json:"platform" binding:"required"`
	GroupID       *int64 `json:"group_id"`
	IncludeNormal bool   `json:"include_normal"`
}

// GetTimeseries returns the hourly spend/forecast/available capacity series.
// GET /api/v1/admin/accounts/capacity/timeseries?platform=openai&group_id=&range=24h&force=
func (h *CapacityHandler) GetTimeseries(c *gin.Context) {
	if h == nil || h.capacityService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Capacity forecast service is not configured")
		return
	}

	platform := strings.TrimSpace(c.Query("platform"))
	if platform == "" {
		response.BadRequest(c, "platform is required")
		return
	}

	var groupID *int64
	if groupRaw := strings.TrimSpace(c.Query("group_id")); groupRaw != "" {
		id, err := strconv.ParseInt(groupRaw, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		groupID = &id
	}

	rangeKey := c.DefaultQuery("range", "24h")
	force := c.Query("force") == "true"

	// Bound the forecast/simulation work (usage_logs aggregation + upstream
	// window math) so a slow scope cannot hang the request indefinitely.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()

	result, err := h.capacityService.GetTimeseries(ctx, platform, groupID, rangeKey, force)
	if err != nil {
		if errors.Is(err, service.ErrCapacityInvalidRange) {
			response.BadRequest(c, "Invalid range")
			return
		}
		if errors.Is(err, service.ErrCapacityProviderNotFound) {
			response.BadRequest(c, "No capacity provider registered for this platform")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// Probe batch-refreshes ("测活") upstream quota for a scope of OAuth accounts.
// POST /api/v1/admin/accounts/capacity/probe
func (h *CapacityHandler) Probe(c *gin.Context) {
	if h == nil || h.capacityService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Capacity forecast service is not configured")
		return
	}

	var req capacityProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		response.BadRequest(c, "platform is required")
		return
	}

	// The probe batch itself is bounded (capacityForecastProbeTimeout, 110s);
	// give the handler a little headroom beyond that.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	result, err := h.capacityService.Probe(ctx, platform, req.GroupID, req.IncludeNormal)
	if err != nil {
		if errors.Is(err, service.ErrCapacityProviderNotFound) {
			response.BadRequest(c, "No capacity provider registered for this platform")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
