package repository

import (
	"math"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func mergeOpsAggSummary(dst, src *opsAggSummary) {
	dst.requestCount += src.requestCount
	dst.successCount += src.successCount
	dst.errorCount += src.errorCount
	dst.error4xxCount += src.error4xxCount
	dst.error5xxCount += src.error5xxCount
	dst.timeoutCount += src.timeoutCount
	dst.avgLatencyWeightedSum += src.avgLatencyWeightedSum
	dst.avgLatencyWeight += src.avgLatencyWeight
	if src.p99LatencyMax > dst.p99LatencyMax {
		dst.p99LatencyMax = src.p99LatencyMax
	}
}

func mergeProviderAgg(dst, src *providerStatsAgg) {
	dst.requestCount += src.requestCount
	dst.successCount += src.successCount
	dst.errorCount += src.errorCount
	dst.error4xxCount += src.error4xxCount
	dst.error5xxCount += src.error5xxCount
	dst.timeoutCount += src.timeoutCount
	dst.avgLatencyWeightedSum += src.avgLatencyWeightedSum
	dst.avgLatencyWeight += src.avgLatencyWeight
	if src.p99LatencyMax > dst.p99LatencyMax {
		dst.p99LatencyMax = src.p99LatencyMax
	}
}

func mergeProviderAggMap(dst map[string]*providerStatsAgg, src map[string]*providerStatsAgg) {
	for platform, row := range src {
		if row == nil {
			continue
		}
		existing := dst[platform]
		if existing == nil {
			existing = &providerStatsAgg{}
			dst[platform] = existing
		}
		mergeProviderAgg(existing, row)
	}
}

func mergeProviderStatsAgg(dst map[string]*providerStatsAgg, raw []*service.ProviderStats) {
	for _, it := range raw {
		if it == nil {
			continue
		}
		existing := dst[it.Platform]
		if existing == nil {
			existing = &providerStatsAgg{}
			dst[it.Platform] = existing
		}
		existing.requestCount += it.RequestCount
		existing.successCount += it.SuccessCount
		existing.errorCount += it.ErrorCount
		existing.error4xxCount += it.Error4xxCount
		existing.error5xxCount += it.Error5xxCount
		existing.timeoutCount += it.TimeoutCount
		if it.SuccessCount > 0 && it.AvgLatencyMs > 0 {
			existing.avgLatencyWeightedSum += float64(it.AvgLatencyMs) * float64(it.SuccessCount)
			existing.avgLatencyWeight += it.SuccessCount
		}
		if float64(it.P99LatencyMs) > existing.p99LatencyMax {
			existing.p99LatencyMax = float64(it.P99LatencyMs)
		}
	}
}

func mergeHistogramCounts(dst map[string]int64, src map[string]int64) {
	for k, v := range src {
		dst[k] += v
	}
}

func mergeWindowStats(dst *service.OpsWindowStats, src *service.OpsWindowStats) {
	if dst == nil || src == nil {
		return
	}
	dst.SuccessCount += src.SuccessCount
	dst.ErrorCount += src.ErrorCount
	dst.Error4xxCount += src.Error4xxCount
	dst.Error5xxCount += src.Error5xxCount
	dst.TimeoutCount += src.TimeoutCount
	dst.TokenConsumed += src.TokenConsumed

	// Conservative merge for latency percentiles: keep the worst/highest observed.
	if src.P50LatencyMs > dst.P50LatencyMs {
		dst.P50LatencyMs = src.P50LatencyMs
	}
	if src.P95LatencyMs > dst.P95LatencyMs {
		dst.P95LatencyMs = src.P95LatencyMs
	}
	if src.P99LatencyMs > dst.P99LatencyMs {
		dst.P99LatencyMs = src.P99LatencyMs
	}
	if src.MaxLatencyMs > dst.MaxLatencyMs {
		dst.MaxLatencyMs = src.MaxLatencyMs
	}

	// Average latency is weighted by success_count (best available proxy).
	weightDst := dst.SuccessCount - src.SuccessCount
	weightSrc := src.SuccessCount
	if weightDst > 0 && weightSrc > 0 && dst.AvgLatencyMs > 0 && src.AvgLatencyMs > 0 {
		dst.AvgLatencyMs = int(math.Round(
			(float64(dst.AvgLatencyMs)*float64(weightDst) + float64(src.AvgLatencyMs)*float64(weightSrc)) /
				float64(weightDst+weightSrc),
		))
	} else if dst.AvgLatencyMs == 0 && src.AvgLatencyMs > 0 {
		dst.AvgLatencyMs = src.AvgLatencyMs
	}
}

func mergeOverviewStats(dst *service.OverviewStats, src *service.OverviewStats) {
	if dst == nil || src == nil {
		return
	}
	dst.RequestCount += src.RequestCount
	dst.SuccessCount += src.SuccessCount
	dst.ErrorCount += src.ErrorCount
	dst.Error4xxCount += src.Error4xxCount
	dst.Error5xxCount += src.Error5xxCount
	dst.TimeoutCount += src.TimeoutCount

	if src.TopErrorCount > dst.TopErrorCount {
		dst.TopErrorCode = src.TopErrorCode
		dst.TopErrorMsg = src.TopErrorMsg
		dst.TopErrorCount = src.TopErrorCount
	}

	if src.LatencyP50 > dst.LatencyP50 {
		dst.LatencyP50 = src.LatencyP50
	}
	if src.LatencyP95 > dst.LatencyP95 {
		dst.LatencyP95 = src.LatencyP95
	}
	if src.LatencyP99 > dst.LatencyP99 {
		dst.LatencyP99 = src.LatencyP99
	}
	if src.LatencyMax > dst.LatencyMax {
		dst.LatencyMax = src.LatencyMax
	}

	weightDst := dst.SuccessCount - src.SuccessCount
	weightSrc := src.SuccessCount
	if weightDst > 0 && weightSrc > 0 && dst.LatencyAvg > 0 && src.LatencyAvg > 0 {
		dst.LatencyAvg = int(math.Round(
			(float64(dst.LatencyAvg)*float64(weightDst) + float64(src.LatencyAvg)*float64(weightSrc)) /
				float64(weightDst+weightSrc),
		))
	} else if dst.LatencyAvg == 0 && src.LatencyAvg > 0 {
		dst.LatencyAvg = src.LatencyAvg
	}
}
