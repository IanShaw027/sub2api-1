package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (r *OpsRepository) queryOpsAggSummary(ctx context.Context, startTime, endTime time.Time) (opsAggSummary, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if !startTime.Before(endTime) {
		return opsAggSummary{}, nil
	}

	// Optimization:
	// - For short time ranges (<= 24h), use the hourly table for precision without scanning many daily buckets.
	// - For longer ranges (> 24h), use daily buckets for the full-day middle segment, and hourly buckets for the
	//   remaining partial-day hours at the edges (to preserve exact semantics without falling back to raw logs).
	if preaggUseHourlyOnly(startTime, endTime) {
		return r.queryOpsAggSummaryHourly(ctx, startTime, endTime)
	}

	var out opsAggSummary

	// Prefer daily for full-day buckets (if any), then fill the remaining hours from hourly.
	dayStart := utcCeilToDay(startTime)
	dayEnd := utcFloorToDay(endTime)

	// 1) Hourly head segment: [startTime, min(dayStart, endTime))
	headEnd := minTime(dayStart, endTime)
	if startTime.Before(headEnd) {
		hStart := utcCeilToHour(startTime)
		hEnd := utcFloorToHour(headEnd)
		if hStart.Before(hEnd) {
			part, err := r.queryOpsAggSummaryHourly(ctx, hStart, hEnd)
			if err != nil {
				return opsAggSummary{}, err
			}
			mergeOpsAggSummary(&out, &part)
		}
	}

	// 2) Daily middle segment: [dayStart, dayEnd)
	if dayStart.Before(dayEnd) {
		part, err := r.queryOpsAggSummaryDaily(ctx, dayStart, dayEnd)
		if err != nil {
			return opsAggSummary{}, err
		}

		// If daily isn't populated yet, fall back to raw-log queries at the method level.
		if part.requestCount == 0 && part.successCount == 0 && part.errorCount == 0 {
			if exists, err := r.rawOpsDataExists(ctx, dayStart, dayEnd); err == nil && exists {
				return opsAggSummary{}, errOpsPreaggregatedNotPopulated
			}
		}
		mergeOpsAggSummary(&out, &part)
	}

	// 3) Hourly tail segment: [max(dayEnd, startTime), endTime)
	tailStart := maxTime(dayEnd, startTime)
	if tailStart.Before(endTime) {
		hStart := utcCeilToHour(tailStart)
		hEnd := utcFloorToHour(endTime)
		if hStart.Before(hEnd) {
			part, err := r.queryOpsAggSummaryHourly(ctx, hStart, hEnd)
			if err != nil {
				return opsAggSummary{}, err
			}
			mergeOpsAggSummary(&out, &part)
		}
	}

	return out, nil
}

func (r *OpsRepository) queryOpsAggSummaryHourly(ctx context.Context, startTime, endTime time.Time) (opsAggSummary, error) {
	var out opsAggSummary
	var avgWeightedSum sql.NullFloat64
	var avgWeight sql.NullInt64
	var p99 sql.NullFloat64

	err := scanSingleRow(
		ctx,
		r.sql,
		`
			SELECT
				COALESCE(SUM(request_count), 0) AS request_count,
				COALESCE(SUM(success_count), 0) AS success_count,
				COALESCE(SUM(error_count), 0) AS error_count,
				COALESCE(SUM(error_4xx_count), 0) AS error_4xx_count,
				COALESCE(SUM(error_5xx_count), 0) AS error_5xx_count,
				COALESCE(SUM(timeout_count), 0) AS timeout_count,
				SUM(avg_latency_ms * success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weighted_sum,
				SUM(success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weight,
				MAX(p99_latency_ms) AS p99_latency_max
			FROM ops_metrics_hourly
			WHERE bucket_start >= $1 AND bucket_start < $2
		`,
		[]any{startTime, endTime},
		&out.requestCount,
		&out.successCount,
		&out.errorCount,
		&out.error4xxCount,
		&out.error5xxCount,
		&out.timeoutCount,
		&avgWeightedSum,
		&avgWeight,
		&p99,
	)
	if err != nil {
		return opsAggSummary{}, err
	}
	if avgWeightedSum.Valid {
		out.avgLatencyWeightedSum = avgWeightedSum.Float64
	}
	if avgWeight.Valid {
		out.avgLatencyWeight = avgWeight.Int64
	}
	if p99.Valid {
		out.p99LatencyMax = p99.Float64
	}
	return out, nil
}

func (r *OpsRepository) queryOpsAggSummaryDaily(ctx context.Context, startTime, endTime time.Time) (opsAggSummary, error) {
	var out opsAggSummary
	var avgWeightedSum sql.NullFloat64
	var avgWeight sql.NullInt64
	var p99 sql.NullFloat64

	err := scanSingleRow(
		ctx,
		r.sql,
		`
			SELECT
				COALESCE(SUM(request_count), 0) AS request_count,
				COALESCE(SUM(success_count), 0) AS success_count,
				COALESCE(SUM(error_count), 0) AS error_count,
				COALESCE(SUM(error_4xx_count), 0) AS error_4xx_count,
				COALESCE(SUM(error_5xx_count), 0) AS error_5xx_count,
				COALESCE(SUM(timeout_count), 0) AS timeout_count,
				SUM(avg_latency_ms * success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weighted_sum,
				SUM(success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weight,
				MAX(p99_latency_ms) AS p99_latency_max
			FROM ops_metrics_daily
			WHERE bucket_date >= $1::date AND bucket_date < $2::date
		`,
		[]any{startTime, endTime},
		&out.requestCount,
		&out.successCount,
		&out.errorCount,
		&out.error4xxCount,
		&out.error5xxCount,
		&out.timeoutCount,
		&avgWeightedSum,
		&avgWeight,
		&p99,
	)
	if err != nil {
		return opsAggSummary{}, err
	}
	if avgWeightedSum.Valid {
		out.avgLatencyWeightedSum = avgWeightedSum.Float64
	}
	if avgWeight.Valid {
		out.avgLatencyWeight = avgWeight.Int64
	}
	if p99.Valid {
		out.p99LatencyMax = p99.Float64
	}
	return out, nil
}

func (r *OpsRepository) queryProviderAgg(ctx context.Context, startTime, endTime time.Time) (map[string]*providerStatsAgg, error) {
	startTime, endTime = normalizeTimeRange(startTime, endTime)
	if !startTime.Before(endTime) {
		return map[string]*providerStatsAgg{}, nil
	}

	out := make(map[string]*providerStatsAgg)

	// Same optimization rule as queryOpsAggSummary:
	// - <=24h: hourly only
	// - >24h: daily full days + hourly edges
	if preaggUseHourlyOnly(startTime, endTime) {
		if err := r.mergeProviderAggHourly(ctx, out, startTime, endTime); err != nil {
			return nil, err
		}
		return out, nil
	}

	dayStart := utcCeilToDay(startTime)
	dayEnd := utcFloorToDay(endTime)

	// Hourly head segment.
	headEnd := minTime(dayStart, endTime)
	if startTime.Before(headEnd) {
		hStart := utcCeilToHour(startTime)
		hEnd := utcFloorToHour(headEnd)
		if hStart.Before(hEnd) {
			if err := r.mergeProviderAggHourly(ctx, out, hStart, hEnd); err != nil {
				return nil, err
			}
		}
	}

	// Daily middle segment (fall back to raw-log queries if daily not populated).
	if dayStart.Before(dayEnd) {
		dailyRows, err := r.queryProviderAggDaily(ctx, dayStart, dayEnd)
		if err != nil {
			return nil, err
		}
		if len(dailyRows) == 0 {
			if exists, err := r.rawOpsDataExists(ctx, dayStart, dayEnd); err == nil && exists {
				return nil, errOpsPreaggregatedNotPopulated
			}
			// No data in raw logs either -> nothing to merge.
		} else {
			mergeProviderAggMap(out, dailyRows)
		}
	}

	// Hourly tail segment.
	tailStart := maxTime(dayEnd, startTime)
	if tailStart.Before(endTime) {
		hStart := utcCeilToHour(tailStart)
		hEnd := utcFloorToHour(endTime)
		if hStart.Before(hEnd) {
			if err := r.mergeProviderAggHourly(ctx, out, hStart, hEnd); err != nil {
				return nil, err
			}
		}
	}

	return out, nil
}

func (r *OpsRepository) mergeProviderAggHourly(ctx context.Context, acc map[string]*providerStatsAgg, startTime, endTime time.Time) error {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			platform,
			COALESCE(SUM(request_count), 0) AS request_count,
			COALESCE(SUM(success_count), 0) AS success_count,
			COALESCE(SUM(error_count), 0) AS error_count,
			COALESCE(SUM(error_4xx_count), 0) AS error_4xx_count,
			COALESCE(SUM(error_5xx_count), 0) AS error_5xx_count,
			COALESCE(SUM(timeout_count), 0) AS timeout_count,
			SUM(avg_latency_ms * success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weighted_sum,
			SUM(success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weight,
			MAX(p99_latency_ms) AS p99_latency_max
		FROM ops_metrics_hourly
		WHERE bucket_start >= $1 AND bucket_start < $2 AND platform <> ''
		GROUP BY platform
	`, startTime, endTime)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var platform string
		var row providerStatsAgg
		var avgWeightedSum sql.NullFloat64
		var avgWeight sql.NullInt64
		var p99 sql.NullFloat64
		if err := rows.Scan(
			&platform,
			&row.requestCount,
			&row.successCount,
			&row.errorCount,
			&row.error4xxCount,
			&row.error5xxCount,
			&row.timeoutCount,
			&avgWeightedSum,
			&avgWeight,
			&p99,
		); err != nil {
			return err
		}
		if avgWeightedSum.Valid {
			row.avgLatencyWeightedSum = avgWeightedSum.Float64
		}
		if avgWeight.Valid {
			row.avgLatencyWeight = avgWeight.Int64
		}
		if p99.Valid {
			row.p99LatencyMax = p99.Float64
		}

		existing := acc[platform]
		if existing == nil {
			existing = &providerStatsAgg{}
			acc[platform] = existing
		}
		mergeProviderAgg(existing, &row)
	}
	return rows.Err()
}

func (r *OpsRepository) queryProviderAggDaily(ctx context.Context, startTime, endTime time.Time) (map[string]*providerStatsAgg, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			platform,
			COALESCE(SUM(request_count), 0) AS request_count,
			COALESCE(SUM(success_count), 0) AS success_count,
			COALESCE(SUM(error_count), 0) AS error_count,
			COALESCE(SUM(error_4xx_count), 0) AS error_4xx_count,
			COALESCE(SUM(error_5xx_count), 0) AS error_5xx_count,
			COALESCE(SUM(timeout_count), 0) AS timeout_count,
			SUM(avg_latency_ms * success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weighted_sum,
			SUM(success_count) FILTER (WHERE avg_latency_ms IS NOT NULL) AS avg_latency_weight,
			MAX(p99_latency_ms) AS p99_latency_max
		FROM ops_metrics_daily
		WHERE bucket_date >= $1::date AND bucket_date < $2::date AND platform <> ''
		GROUP BY platform
	`, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]*providerStatsAgg)
	for rows.Next() {
		var platform string
		var row providerStatsAgg
		var avgWeightedSum sql.NullFloat64
		var avgWeight sql.NullInt64
		var p99 sql.NullFloat64
		if err := rows.Scan(
			&platform,
			&row.requestCount,
			&row.successCount,
			&row.errorCount,
			&row.error4xxCount,
			&row.error5xxCount,
			&row.timeoutCount,
			&avgWeightedSum,
			&avgWeight,
			&p99,
		); err != nil {
			return nil, err
		}
		if avgWeightedSum.Valid {
			row.avgLatencyWeightedSum = avgWeightedSum.Float64
		}
		if avgWeight.Valid {
			row.avgLatencyWeight = avgWeight.Int64
		}
		if p99.Valid {
			row.p99LatencyMax = p99.Float64
		}
		out[platform] = &row
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *OpsRepository) queryLatencyHistogramCounts(ctx context.Context, startTime, endTime time.Time) (map[string]int64, error) {
	counts := make(map[string]int64)

	// Keep behavior consistent with other ops_* pre-aggregation queries:
	// - <=24h: hourly only
	// - >24h: daily full days + hourly edges
	if preaggUseHourlyOnly(startTime, endTime) {
		part, err := r.queryLatencyHistogramCountsHourly(ctx, startTime, endTime)
		if err != nil {
			return nil, err
		}
		mergeHistogramCounts(counts, part)
		return counts, nil
	}

	dayStart := utcCeilToDay(startTime)
	dayEnd := utcFloorToDay(endTime)

	// Hourly head segment.
	headEnd := minTime(dayStart, endTime)
	if startTime.Before(headEnd) {
		hStart := utcCeilToHour(startTime)
		hEnd := utcFloorToHour(headEnd)
		if hStart.Before(hEnd) {
			part, err := r.queryLatencyHistogramCountsHourly(ctx, hStart, hEnd)
			if err != nil {
				return nil, err
			}
			mergeHistogramCounts(counts, part)
		}
	}

	// Daily middle segment (fall back to raw-log queries if daily not populated).
	if dayStart.Before(dayEnd) {
		part, err := r.queryLatencyHistogramCountsDaily(ctx, dayStart, dayEnd)
		if err != nil {
			return nil, err
		}
		if len(part) == 0 {
			if exists, err := r.rawOpsDataExists(ctx, dayStart, dayEnd); err == nil && exists {
				return nil, errOpsPreaggregatedNotPopulated
			}
		}
		mergeHistogramCounts(counts, part)
	}

	// Hourly tail segment.
	tailStart := maxTime(dayEnd, startTime)
	if tailStart.Before(endTime) {
		hStart := utcCeilToHour(tailStart)
		hEnd := utcFloorToHour(endTime)
		if hStart.Before(hEnd) {
			part, err := r.queryLatencyHistogramCountsHourly(ctx, hStart, hEnd)
			if err != nil {
				return nil, err
			}
			mergeHistogramCounts(counts, part)
		}
	}

	return counts, nil
}

func (r *OpsRepository) queryLatencyHistogramCountsHourly(ctx context.Context, startTime, endTime time.Time) (map[string]int64, error) {
	rows, err := r.sql.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			%[1]s AS range_name,
			COALESCE(SUM(success_count), 0) AS count
		FROM ops_metrics_hourly
		WHERE bucket_start >= $1 AND bucket_start < $2 AND avg_latency_ms IS NOT NULL
		GROUP BY 1
	`, latencyHistogramRangeCaseExpr("avg_latency_ms")), startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]int64)
	for rows.Next() {
		var name string
		var c int64
		if err := rows.Scan(&name, &c); err != nil {
			return nil, err
		}
		out[name] += c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *OpsRepository) queryLatencyHistogramCountsDaily(ctx context.Context, startTime, endTime time.Time) (map[string]int64, error) {
	rows, err := r.sql.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			%[1]s AS range_name,
			COALESCE(SUM(success_count), 0) AS count
		FROM ops_metrics_daily
		WHERE bucket_date >= $1::date AND bucket_date < $2::date AND avg_latency_ms IS NOT NULL
		GROUP BY 1
	`, latencyHistogramRangeCaseExpr("avg_latency_ms")), startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]int64)
	for rows.Next() {
		var name string
		var c int64
		if err := rows.Scan(&name, &c); err != nil {
			return nil, err
		}
		out[name] += c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
