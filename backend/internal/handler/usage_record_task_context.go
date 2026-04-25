package handler

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func wrapUsageRecordTaskWithRequestContext(c *gin.Context, task service.UsageRecordTask) service.UsageRecordTask {
	if task == nil || c == nil || c.Request == nil {
		return task
	}

	requestCtx := c.Request.Context()
	clientRequestID, _ := requestCtx.Value(ctxkey.ClientRequestID).(string)
	requestID, _ := requestCtx.Value(ctxkey.RequestID).(string)
	requestLogger := logger.FromContext(requestCtx)

	return func(taskCtx context.Context) {
		if taskCtx == nil {
			taskCtx = context.Background()
		}
		if trimmed := strings.TrimSpace(clientRequestID); trimmed != "" {
			taskCtx = context.WithValue(taskCtx, ctxkey.ClientRequestID, trimmed)
		}
		if trimmed := strings.TrimSpace(requestID); trimmed != "" {
			taskCtx = context.WithValue(taskCtx, ctxkey.RequestID, trimmed)
		}
		taskCtx = logger.IntoContext(taskCtx, requestLogger)
		task(taskCtx)
	}
}
