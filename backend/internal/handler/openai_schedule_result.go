package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func shouldReportOpenAIAccountScheduleFailure(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if service.GetOpsCyberPolicy(c) != nil {
		return false
	}
	if strings.Contains(strings.ToLower(err.Error()), "cyber_policy") {
		return false
	}
	if service.IsOpenAIWSSessionPreemptedError(err) {
		return false
	}
	return !errors.Is(err, context.Canceled)
}

func isOpenAIClientRequestCanceled(c *gin.Context, err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	if c == nil || c.Request == nil || c.Request.Context() == nil {
		return false
	}
	return errors.Is(c.Request.Context().Err(), context.Canceled)
}

func (h *OpenAIGatewayHandler) reportOpenAIAccountScheduleFailure(c *gin.Context, accountID int64, err error) {
	if h == nil || h.gatewayService == nil || !shouldReportOpenAIAccountScheduleFailure(c, err) {
		return
	}
	h.gatewayService.ReportOpenAIAccountScheduleResult(accountID, false, nil)
}
