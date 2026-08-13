package service

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
)

// Debug-timeline helpers used by the Kiro gateway. This branch does not ship
// the full timeline writer; the methods stay no-ops so capture can be added later.

func GatewayDebugTimelineEnabled(context.Context, *SettingService) bool {
	return false
}

func shouldRecordGatewayDebugBodyForPlatform(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "anthropic", "kiro":
		return true
	default:
		return false
	}
}

func RecordGatewayDebugTimelineBody(*SettingService, *gin.Context, string, []byte, string, map[string]any) {
}

type KiroFrameAggregator struct {
	enabled bool
}

func BeginKiroFrameAggregator(*SettingService, *gin.Context) *KiroFrameAggregator {
	return &KiroFrameAggregator{}
}

func (agg *KiroFrameAggregator) Append(*kiroFrame, string) {}

func (agg *KiroFrameAggregator) Finalize(string, map[string]any) {}
