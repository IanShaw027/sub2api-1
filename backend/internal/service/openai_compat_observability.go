package service

import (
	"strings"
	"sync"
	"sync/atomic"
)

type OpenAICompatRuntimeMetricsSnapshot struct {
	StrippedVerbosityTotal        int64            `json:"stripped_verbosity_total"`
	StrippedTemperatureTotal      int64            `json:"stripped_temperature_total"`
	StrippedTopPTotal             int64            `json:"stripped_top_p_total"`
	StrippedCacheControlTotal     int64            `json:"stripped_cache_control_total"`
	PromptCacheInjectedTotal      int64            `json:"prompt_cache_injected_total"`
	ToolContinuationDetectedTotal int64            `json:"tool_continuation_detected_total"`
	Upstream4xxByModel            map[string]int64 `json:"upstream_4xx_by_model"`
	Upstream5xxByModel            map[string]int64 `json:"upstream_5xx_by_model"`
}

const (
	maxOpenAICompatModelMetricKeys     = 1000
	openAICompatModelMetricOverflowKey = "__overflow__"
)

var (
	openAICompatStrippedVerbosityTotal        atomic.Int64
	openAICompatStrippedTemperatureTotal      atomic.Int64
	openAICompatStrippedTopPTotal             atomic.Int64
	openAICompatStrippedCacheControlTotal     atomic.Int64
	openAICompatPromptCacheInjectedTotal      atomic.Int64
	openAICompatToolContinuationDetectedTotal atomic.Int64

	openAICompatUpstreamMetricsMu  sync.Mutex
	openAICompatUpstream4xxByModel = make(map[string]int64)
	openAICompatUpstream5xxByModel = make(map[string]int64)
)

func recordBoundedModelMetric(counts map[string]int64, model string) {
	if _, ok := counts[model]; ok {
		counts[model]++
		return
	}
	if len(counts) >= maxOpenAICompatModelMetricKeys {
		counts[openAICompatModelMetricOverflowKey]++
		return
	}
	counts[model]++
}

func recordOpenAICompatStrippedField(field string) {
	switch strings.TrimSpace(field) {
	case "verbosity":
		openAICompatStrippedVerbosityTotal.Add(1)
	case "temperature":
		openAICompatStrippedTemperatureTotal.Add(1)
	case "top_p":
		openAICompatStrippedTopPTotal.Add(1)
	case "cache_control":
		openAICompatStrippedCacheControlTotal.Add(1)
	}
}

func recordOpenAICompatPromptCacheInjected() {
	openAICompatPromptCacheInjectedTotal.Add(1)
}

func recordOpenAICompatToolContinuationDetected() {
	openAICompatToolContinuationDetectedTotal.Add(1)
}

func recordOpenAICompatUpstreamStatus(model string, statusCode int) {
	trimmedModel := strings.TrimSpace(model)
	if trimmedModel == "" {
		return
	}

	openAICompatUpstreamMetricsMu.Lock()
	defer openAICompatUpstreamMetricsMu.Unlock()

	switch {
	case statusCode >= 400 && statusCode < 500:
		recordBoundedModelMetric(openAICompatUpstream4xxByModel, trimmedModel)
	case statusCode >= 500:
		recordBoundedModelMetric(openAICompatUpstream5xxByModel, trimmedModel)
	}
}

func SnapshotOpenAICompatRuntimeMetrics() OpenAICompatRuntimeMetricsSnapshot {
	openAICompatUpstreamMetricsMu.Lock()
	defer openAICompatUpstreamMetricsMu.Unlock()

	upstream4xx := make(map[string]int64, len(openAICompatUpstream4xxByModel))
	for model, count := range openAICompatUpstream4xxByModel {
		upstream4xx[model] = count
	}

	upstream5xx := make(map[string]int64, len(openAICompatUpstream5xxByModel))
	for model, count := range openAICompatUpstream5xxByModel {
		upstream5xx[model] = count
	}

	return OpenAICompatRuntimeMetricsSnapshot{
		StrippedVerbosityTotal:        openAICompatStrippedVerbosityTotal.Load(),
		StrippedTemperatureTotal:      openAICompatStrippedTemperatureTotal.Load(),
		StrippedTopPTotal:             openAICompatStrippedTopPTotal.Load(),
		StrippedCacheControlTotal:     openAICompatStrippedCacheControlTotal.Load(),
		PromptCacheInjectedTotal:      openAICompatPromptCacheInjectedTotal.Load(),
		ToolContinuationDetectedTotal: openAICompatToolContinuationDetectedTotal.Load(),
		Upstream4xxByModel:            upstream4xx,
		Upstream5xxByModel:            upstream5xx,
	}
}
