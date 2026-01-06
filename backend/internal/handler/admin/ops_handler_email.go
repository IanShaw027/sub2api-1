package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GetEmailNotificationConfig 获取邮件通知配置
func (h *OpsHandler) GetEmailNotificationConfig(c *gin.Context) {
	config, err := h.opsService.GetEmailNotificationConfig(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get email notification config")
		return
	}
	response.Success(c, config)
}

// UpdateEmailNotificationConfig 更新邮件通知配置
func (h *OpsHandler) UpdateEmailNotificationConfig(c *gin.Context) {
	var req service.OpsEmailNotificationConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.opsService.UpdateEmailNotificationConfig(c.Request.Context(), &req); err != nil {
		// Most failures here are validation errors from request payload; treat as 400.
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.opsService.GetEmailNotificationConfig(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to load updated email notification config")
		return
	}
	response.Success(c, updated)
}
