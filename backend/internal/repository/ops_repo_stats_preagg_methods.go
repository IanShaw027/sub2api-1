package repository

import (
	"context"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *OpsRepository) getWindowStatsPreaggregated(ctx context.Context, startTime, endTime time.Time) (*service.OpsWindowStats, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if startTime.IsZero() || endTime.IsZero() || !startTime.Before(endTime) {
		return r.GetWindowStatsLegacy(ctx, startTime, endTime)
	}

	aggSafeEnd := r.preaggSafeEnd(endTime)
	aggFullStart := utcCeilToHour(startTime)
	aggFullEnd := utcFloorToHour(aggSafeEnd)

	// If there are no stable full-hour buckets, keep the raw-log path (real-time windows).
	if !aggFullStart.Before(aggFullEnd) {
		return r.GetWindowStatsLegacy(ctx, startTime, endTime)
	}

	agg, aggErr := r.queryOpsAggSummary(ctx, aggFullStart, aggFullEnd)
	if aggErr != nil {
		return nil, aggErr
	}

	// If aggregates returned no data but raw logs do have rows, treat it as "not populated yet"
	// and fall back to the legacy query for correctness.
	if agg.requestCount == 0 && agg.successCount == 0 && agg.errorCount == 0 {
		if exists, err := r.rawOpsDataExists(ctx, aggFullStart, aggFullEnd); err == nil && exists {
			return nil, errOpsPreaggregatedNotPopulated
		}
	}

	// Build a conservative approximation for the portion served by ops_metrics_*.
	out := &service.OpsWindowStats{
		SuccessCount:  agg.successCount,
		ErrorCount:    agg.errorCount,
		Error4xxCount: agg.error4xxCount,
		Error5xxCount: agg.error5xxCount,
		TimeoutCount:  agg.timeoutCount,
	}
	if agg.avgLatencyWeight > 0 && agg.avgLatencyWeightedSum > 0 {
		out.AvgLatencyMs = int(math.Round(agg.avgLatencyWeightedSum / float64(agg.avgLatencyWeight)))
	}
	if agg.p99LatencyMax > 0 {
		out.P99LatencyMs = int(math.Round(agg.p99LatencyMax))
	}

	// Merge in raw head/tail fragments.
	if startTime.Before(aggFullStart) {
		part, err := r.GetWindowStatsLegacy(ctx, startTime, minTime(endTime, aggFullStart))
		if err != nil {
			return nil, err
		}
		mergeWindowStats(out, part)
	}
	if aggFullEnd.Before(endTime) {
		part, err := r.GetWindowStatsLegacy(ctx, maxTime(startTime, aggFullEnd), endTime)
		if err != nil {
			return nil, err
		}
		mergeWindowStats(out, part)
	}

	return out, nil
}

func (r *OpsRepository) getOverviewStatsPreaggregated(ctx context.Context, startTime, endTime time.Time) (*service.OverviewStats, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if startTime.IsZero() || endTime.IsZero() || !startTime.Before(endTime) {
		return r.GetOverviewStatsLegacy(ctx, startTime, endTime)
	}

	aggSafeEnd := r.preaggSafeEnd(endTime)
	aggFullStart := utcCeilToHour(startTime)
	aggFullEnd := utcFloorToHour(aggSafeEnd)

	if !aggFullStart.Before(aggFullEnd) {
		return r.GetOverviewStatsLegacy(ctx, startTime, endTime)
	}

	agg, err := r.queryOpsAggSummary(ctx, aggFullStart, aggFullEnd)
	if err != nil {
		return nil, err
	}

	if agg.requestCount == 0 && agg.successCount == 0 && agg.errorCount == 0 {
		if exists, err := r.rawOpsDataExists(ctx, aggFullStart, aggFullEnd); err == nil && exists {
			return nil, errOpsPreaggregatedNotPopulated
		}
	}

	// Snapshot-style stats from aggregates.
	out := &service.OverviewStats{
		RequestCount:          agg.requestCount,
		SuccessCount:          agg.successCount,
		ErrorCount:            agg.errorCount,
		Error4xxCount:         agg.error4xxCount,
		Error5xxCount:         agg.error5xxCount,
		TimeoutCount:          agg.timeoutCount,
		TopErrorCode:          "",
		TopErrorMsg:           "",
		TopErrorCount:         0,
		LatencyP50:            0,
		LatencyP95:            0,
		LatencyP99:            int(math.Round(agg.p99LatencyMax)),
		LatencyAvg:            0,
		LatencyMax:            0,
		CPUUsage:              0,
		MemoryUsage:           0,
		MemoryUsedMB:          0,
		MemoryTotalMB:         0,
		ConcurrencyQueueDepth: 0,
	}
	if agg.avgLatencyWeight > 0 && agg.avgLatencyWeightedSum > 0 {
		out.LatencyAvg = int(math.Round(agg.avgLatencyWeightedSum / float64(agg.avgLatencyWeight)))
	}

	// Merge in raw head/tail fragments.
	if startTime.Before(aggFullStart) {
		part, err := r.GetOverviewStatsLegacy(ctx, startTime, minTime(endTime, aggFullStart))
		if err != nil {
			return nil, err
		}
		mergeOverviewStats(out, part)
	}
	if aggFullEnd.Before(endTime) {
		part, err := r.GetOverviewStatsLegacy(ctx, maxTime(startTime, aggFullEnd), endTime)
		if err != nil {
			return nil, err
		}
		mergeOverviewStats(out, part)
	}

	// Pull system snapshot from ops_system_metrics to keep dashboards responsive.
	if snap, err := r.getLatestSystemSnapshot(ctx); err == nil && snap != nil {
		out.CPUUsage = snap.CPUUsage
		out.MemoryUsage = snap.MemoryUsage
		out.MemoryUsedMB = snap.MemoryUsedMB
		out.MemoryTotalMB = snap.MemoryTotalMB
		out.ConcurrencyQueueDepth = snap.ConcurrencyQueueDepth
	}

	return out, nil
}

func (r *OpsRepository) getProviderStatsPreaggregated(ctx context.Context, startTime, endTime time.Time) ([]*service.ProviderStats, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if startTime.IsZero() || endTime.IsZero() || !startTime.Before(endTime) {
		return r.GetProviderStatsLegacy(ctx, startTime, endTime)
	}

	aggSafeEnd := r.preaggSafeEnd(endTime)
	aggFullStart := utcCeilToHour(startTime)
	aggFullEnd := utcFloorToHour(aggSafeEnd)

	if !aggFullStart.Before(aggFullEnd) {
		return r.GetProviderStatsLegacy(ctx, startTime, endTime)
	}

	agg, err := r.queryProviderAgg(ctx, aggFullStart, aggFullEnd)
	if err != nil {
		return nil, err
	}

	// If aggregates returned no data but raw logs exist, treat as "not populated".
	if len(agg) == 0 {
		if exists, err := r.rawOpsDataExists(ctx, aggFullStart, aggFullEnd); err == nil && exists {
			return nil, errOpsPreaggregatedNotPopulated
		}
	}

	// Merge raw head/tail fragments into the aggregated map.
	if startTime.Before(aggFullStart) {
		items, err := r.GetProviderStatsLegacy(ctx, startTime, minTime(endTime, aggFullStart))
		if err != nil {
			return nil, err
		}
		mergeProviderStatsAgg(agg, items)
	}
	if aggFullEnd.Before(endTime) {
		items, err := r.GetProviderStatsLegacy(ctx, maxTime(startTime, aggFullEnd), endTime)
		if err != nil {
			return nil, err
		}
		mergeProviderStatsAgg(agg, items)
	}

	// Convert to the service DTO, keeping ordering stable (by request_count desc, then platform asc).
	out := make([]*service.ProviderStats, 0, len(agg))
	for platform, row := range agg {
		if row == nil {
			continue
		}
		item := &service.ProviderStats{
			Platform:      platform,
			RequestCount:  row.requestCount,
			SuccessCount:  row.successCount,
			ErrorCount:    row.errorCount,
			Error4xxCount: row.error4xxCount,
			Error5xxCount: row.error5xxCount,
			TimeoutCount:  row.timeoutCount,
			AvgLatencyMs:  0,
			P99LatencyMs:  0,
		}
		if row.avgLatencyWeight > 0 && row.avgLatencyWeightedSum > 0 {
			item.AvgLatencyMs = int(math.Round(row.avgLatencyWeightedSum / float64(row.avgLatencyWeight)))
		}
		if row.p99LatencyMax > 0 {
			item.P99LatencyMs = int(math.Round(row.p99LatencyMax))
		}
		out = append(out, item)
	}

	// Sort.
	sortProviderStats(out)
	return out, nil
}

func (r *OpsRepository) getLatencyHistogramPreaggregated(ctx context.Context, startTime, endTime time.Time) ([]*service.LatencyHistogramItem, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if startTime.IsZero() || endTime.IsZero() || !startTime.Before(endTime) {
		return r.GetLatencyHistogramLegacy(ctx, startTime, endTime)
	}

	aggSafeEnd := r.preaggSafeEnd(endTime)
	aggFullStart := utcCeilToHour(startTime)
	aggFullEnd := utcFloorToHour(aggSafeEnd)

	if !aggFullStart.Before(aggFullEnd) {
		return r.GetLatencyHistogramLegacy(ctx, startTime, endTime)
	}

	counts, err := r.queryLatencyHistogramCounts(ctx, aggFullStart, aggFullEnd)
	if err != nil {
		return nil, err
	}
	if len(counts) == 0 {
		if exists, err := r.rawOpsDataExists(ctx, aggFullStart, aggFullEnd); err == nil && exists {
			return nil, errOpsPreaggregatedNotPopulated
		}
	}

	// Merge in raw head/tail fragments.
	if startTime.Before(aggFullStart) {
		items, err := r.GetLatencyHistogramLegacy(ctx, startTime, minTime(endTime, aggFullStart))
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if it != nil {
				counts[it.Range] += it.Count
			}
		}
	}
	if aggFullEnd.Before(endTime) {
		items, err := r.GetLatencyHistogramLegacy(ctx, maxTime(startTime, aggFullEnd), endTime)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if it != nil {
				counts[it.Range] += it.Count
			}
		}
	}

	total := int64(0)
	for _, c := range counts {
		total += c
	}
	if total <= 0 {
		return []*service.LatencyHistogramItem{}, nil
	}

	out := make([]*service.LatencyHistogramItem, 0, len(latencyHistogramOrderedRanges))
	for _, name := range latencyHistogramOrderedRanges {
		count := counts[name]
		if count <= 0 {
			continue
		}
		out = append(out, &service.LatencyHistogramItem{
			Range:      name,
			Count:      count,
			Percentage: math.Round((float64(count)/float64(total))*10000) / 100,
		})
	}
	return out, nil
}
