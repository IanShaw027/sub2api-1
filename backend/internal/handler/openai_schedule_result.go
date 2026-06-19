package handler

import (
	"context"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func shouldReportOpenAIAccountScheduleFailure(err error) bool {
	if service.IsOpenAIWSSessionPreemptedError(err) {
		return false
	}
	return !errors.Is(err, context.Canceled)
}

func (h *OpenAIGatewayHandler) reportOpenAIAccountScheduleFailure(accountID int64, err error) {
	if h == nil || h.gatewayService == nil || !shouldReportOpenAIAccountScheduleFailure(err) {
		return
	}
	h.gatewayService.ReportOpenAIAccountScheduleResult(accountID, false, nil)
}
