package admin

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var validOpsAlertMetricTypes = []string{
	service.OpsMetricSuccessRate,
	service.OpsMetricErrorRate,
	service.OpsMetricP95LatencyMs,
	service.OpsMetricP99LatencyMs,
	service.OpsMetricCPUUsagePercent,
	service.OpsMetricMemoryUsagePercent,
	service.OpsMetricQueueDepth,
}

var validOpsAlertMetricTypeSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(validOpsAlertMetricTypes))
	for _, v := range validOpsAlertMetricTypes {
		set[v] = struct{}{}
	}
	return set
}()

var validOpsAlertOperators = []string{">", "<", ">=", "<=", "==", "!="}

var validOpsAlertOperatorSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(validOpsAlertOperators))
	for _, v := range validOpsAlertOperators {
		set[v] = struct{}{}
	}
	return set
}()

type opsAlertRuleValidatedInput struct {
	Name             string
	MetricType       string
	Operator         string
	Threshold        float64
	WindowMinutes    int
	SustainedMinutes int
	CooldownMinutes  int

	WindowProvided    bool
	SustainedProvided bool
	CooldownProvided  bool
}

func isPercentOrRateMetric(metricType string) bool {
	switch metricType {
	case service.OpsMetricSuccessRate,
		service.OpsMetricErrorRate,
		service.OpsMetricCPUUsagePercent,
		service.OpsMetricMemoryUsagePercent:
		return true
	default:
		return false
	}
}

func validateOpsAlertRulePayload(raw map[string]json.RawMessage) (*opsAlertRuleValidatedInput, error) {
	if raw == nil {
		return nil, fmt.Errorf("invalid request body")
	}

	requiredFields := []string{"name", "metric_type", "operator", "threshold"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			return nil, fmt.Errorf("%s is required", field)
		}
	}

	var name string
	if err := json.Unmarshal(raw["name"], &name); err != nil || strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	name = strings.TrimSpace(name)

	var metricType string
	if err := json.Unmarshal(raw["metric_type"], &metricType); err != nil || strings.TrimSpace(metricType) == "" {
		return nil, fmt.Errorf("metric_type is required")
	}
	metricType = strings.TrimSpace(metricType)
	if _, ok := validOpsAlertMetricTypeSet[metricType]; !ok {
		return nil, fmt.Errorf("metric_type must be one of: %s", strings.Join(validOpsAlertMetricTypes, ", "))
	}

	var operator string
	if err := json.Unmarshal(raw["operator"], &operator); err != nil || strings.TrimSpace(operator) == "" {
		return nil, fmt.Errorf("operator is required")
	}
	operator = strings.TrimSpace(operator)
	if _, ok := validOpsAlertOperatorSet[operator]; !ok {
		return nil, fmt.Errorf("operator must be one of: %s", strings.Join(validOpsAlertOperators, ", "))
	}

	var threshold float64
	if err := json.Unmarshal(raw["threshold"], &threshold); err != nil {
		return nil, fmt.Errorf("threshold must be a number")
	}
	if math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return nil, fmt.Errorf("threshold must be a finite number")
	}
	if isPercentOrRateMetric(metricType) {
		if threshold < 0 || threshold > 100 {
			return nil, fmt.Errorf("threshold must be between 0 and 100 for metric_type %s", metricType)
		}
	} else if threshold < 0 {
		return nil, fmt.Errorf("threshold must be >= 0")
	}

	validated := &opsAlertRuleValidatedInput{
		Name:       name,
		MetricType: metricType,
		Operator:   operator,
		Threshold:  threshold,
	}

	if v, ok := raw["window_minutes"]; ok {
		validated.WindowProvided = true
		if err := json.Unmarshal(v, &validated.WindowMinutes); err != nil {
			return nil, fmt.Errorf("window_minutes must be an integer")
		}
		switch validated.WindowMinutes {
		case 1, 5, 60:
		default:
			return nil, fmt.Errorf("window_minutes must be one of: 1, 5, 60")
		}
	} else {
		validated.WindowMinutes = 1
	}

	if v, ok := raw["sustained_minutes"]; ok {
		validated.SustainedProvided = true
		if err := json.Unmarshal(v, &validated.SustainedMinutes); err != nil {
			return nil, fmt.Errorf("sustained_minutes must be an integer")
		}
		if validated.SustainedMinutes < 1 || validated.SustainedMinutes > 1440 {
			return nil, fmt.Errorf("sustained_minutes must be between 1 and 1440")
		}
	} else {
		validated.SustainedMinutes = 1
	}

	if v, ok := raw["cooldown_minutes"]; ok {
		validated.CooldownProvided = true
		if err := json.Unmarshal(v, &validated.CooldownMinutes); err != nil {
			return nil, fmt.Errorf("cooldown_minutes must be an integer")
		}
		if validated.CooldownMinutes < 0 || validated.CooldownMinutes > 1440 {
			return nil, fmt.Errorf("cooldown_minutes must be between 0 and 1440")
		}
	} else {
		validated.CooldownMinutes = 0
	}

	return validated, nil
}

// ListAlertRules returns all alert rules.
// GET /api/v1/admin/ops/alert-rules
func (h *OpsHandler) ListAlertRules(c *gin.Context) {
	rules, err := h.opsService.ListAlertRules(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list alert rules")
		return
	}
	response.Success(c, rules)
}

// CreateAlertRule creates a new alert rule.
// POST /api/v1/admin/ops/alert-rules
func (h *OpsHandler) CreateAlertRule(c *gin.Context) {
	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWith(&raw, binding.JSON); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	validated, err := validateOpsAlertRulePayload(raw)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var rule service.OpsAlertRule
	if err := c.ShouldBindBodyWith(&rule, binding.JSON); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	rule.Name = validated.Name
	rule.MetricType = validated.MetricType
	rule.Operator = validated.Operator
	rule.Threshold = validated.Threshold
	rule.WindowMinutes = validated.WindowMinutes
	rule.SustainedMinutes = validated.SustainedMinutes
	rule.CooldownMinutes = validated.CooldownMinutes

	if err := h.opsService.CreateAlertRule(c.Request.Context(), &rule); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create alert rule")
		return
	}
	response.Success(c, rule)
}

// UpdateAlertRule updates an existing alert rule.
// PUT /api/v1/admin/ops/alert-rules/:id
func (h *OpsHandler) UpdateAlertRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid rule ID")
		return
	}

	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWith(&raw, binding.JSON); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	validated, err := validateOpsAlertRulePayload(raw)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var rule service.OpsAlertRule
	if err := c.ShouldBindBodyWith(&rule, binding.JSON); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	rule.Name = validated.Name
	rule.MetricType = validated.MetricType
	rule.Operator = validated.Operator
	rule.Threshold = validated.Threshold
	rule.WindowMinutes = validated.WindowMinutes
	rule.SustainedMinutes = validated.SustainedMinutes
	rule.CooldownMinutes = validated.CooldownMinutes

	rule.ID = id
	if err := h.opsService.UpdateAlertRule(c.Request.Context(), &rule); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to update alert rule")
		return
	}
	response.Success(c, rule)
}

// DeleteAlertRule deletes an alert rule.
// DELETE /api/v1/admin/ops/alert-rules/:id
func (h *OpsHandler) DeleteAlertRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid rule ID")
		return
	}
	if err := h.opsService.DeleteAlertRule(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to delete alert rule")
		return
	}
	response.Success(c, nil)
}

// ListAlertEvents returns alert event history.
// GET /api/v1/admin/ops/alert-events
func (h *OpsHandler) ListAlertEvents(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	events, err := h.opsService.ListAlertEvents(c.Request.Context(), limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list alert events")
		return
	}
	response.Success(c, events)
}
