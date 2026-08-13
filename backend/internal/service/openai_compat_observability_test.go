package service

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func TestSnapshotOpenAICompatRuntimeMetrics(t *testing.T) {
	before := SnapshotOpenAICompatRuntimeMetrics()

	recordOpenAICompatStrippedField("temperature")
	recordOpenAICompatStrippedField("top_p")
	recordOpenAICompatDroppedFields(&apicompat.ResponsesRequest{DroppedCompatibilityFields: []string{"cache_control"}})
	recordOpenAICompatToolContinuationDetected()
	recordOpenAICompatPromptCacheInjected()
	recordOpenAICompatUpstreamStatus("gpt-5.4", http.StatusTooManyRequests)
	recordOpenAICompatUpstreamStatus("gpt-5.4", http.StatusBadGateway)

	after := SnapshotOpenAICompatRuntimeMetrics()
	require.GreaterOrEqual(t, after.StrippedTemperatureTotal, before.StrippedTemperatureTotal+1)
	require.GreaterOrEqual(t, after.StrippedTopPTotal, before.StrippedTopPTotal+1)
	require.GreaterOrEqual(t, after.StrippedCacheControlTotal, before.StrippedCacheControlTotal+1)
	require.GreaterOrEqual(t, after.ToolContinuationDetectedTotal, before.ToolContinuationDetectedTotal+1)
	require.GreaterOrEqual(t, after.PromptCacheInjectedTotal, before.PromptCacheInjectedTotal+1)
	require.GreaterOrEqual(t, after.Upstream4xxByModel["gpt-5.4"], before.Upstream4xxByModel["gpt-5.4"]+1)
	require.GreaterOrEqual(t, after.Upstream5xxByModel["gpt-5.4"], before.Upstream5xxByModel["gpt-5.4"]+1)
}
