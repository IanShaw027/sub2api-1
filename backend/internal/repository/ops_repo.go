package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/metrics"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultWindowMinutes = 1

	MaxErrorLogsLimit     = 500
	DefaultErrorLogsLimit = 200

	MaxRecentSystemMetricsLimit     = 500
	DefaultRecentSystemMetricsLimit = 60

	MaxMetricsLimit     = 1000
	DefaultMetricsLimit = 300
)

// opsMetricsPreaggFreshnessLag is the maximum "fresh" window we assume may not be
// covered by the hourly/daily aggregation tables.
//
// The pre-aggregation tables are intended to be populated by a background job; the
// newest hour is typically still being computed. For that most-recent slice we fall
// back to the legacy raw-log queries to keep real-time dashboards accurate.
const opsMetricsPreaggFreshnessLag = time.Hour

type OpsRepository struct {
	ent *dbent.Client
	sql sqlExecutor
	rdb *redis.Client

	// Feature flag: prefer pre-aggregated ops tables (ops_metrics_hourly/daily) for
	// expensive dashboard queries when available, with safe fallbacks to legacy raw-log queries.
	usePreaggregatedTables bool
}

type opsPreaggFallbackMethod string

const (
	opsPreaggFallbackWindowStats      opsPreaggFallbackMethod = "window_stats"
	opsPreaggFallbackOverviewStats    opsPreaggFallbackMethod = "overview_stats"
	opsPreaggFallbackProviderStats    opsPreaggFallbackMethod = "provider_stats"
	opsPreaggFallbackLatencyHistogram opsPreaggFallbackMethod = "latency_histogram"
)

const opsPreaggFallbackUnexpectedLogMinInterval = 30 * time.Second

var (
	opsPreaggFallbackLogMu      sync.Mutex
	opsPreaggFallbackLastLogUTC = make(map[opsPreaggFallbackMethod]time.Time)
)

func NewOpsRepository(entClient *dbent.Client, sqlDB *sql.DB, rdb *redis.Client, cfg *config.Config) service.OpsRepository {
	usePreagg := false
	if cfg != nil {
		usePreagg = cfg.Ops.UsePreaggregatedTables
	}
	return &OpsRepository{ent: entClient, sql: sqlDB, rdb: rdb, usePreaggregatedTables: usePreagg}
}

// GetWindowStats is the primary API used by the ops dashboard/collector.
//
// When enabled via config (`ops.use_preaggregated_tables`), it prefers the
// pre-aggregated tables for data old enough to be stable (hourly/daily buckets),
// and falls back to the legacy raw-log query for the most recent <1h slice and
// for any periods where aggregates are not yet populated.
func (r *OpsRepository) GetWindowStats(ctx context.Context, startTime, endTime time.Time) (*service.OpsWindowStats, error) {
	if !r.usePreaggregatedTables {
		return r.GetWindowStatsLegacy(ctx, startTime, endTime)
	}
	stats, err := r.getWindowStatsPreaggregated(ctx, startTime, endTime)
	if err == nil {
		return stats, nil
	}
	r.recordOpsPreaggFallback(opsPreaggFallbackWindowStats, startTime, endTime, err)
	return r.GetWindowStatsLegacy(ctx, startTime, endTime)
}

func (r *OpsRepository) GetWindowStatsGrouped(ctx context.Context, startTime, endTime time.Time, groupBy string) ([]*service.OpsWindowStatsGroupedItem, error) {
	if startTime.IsZero() || endTime.IsZero() {
		return nil, nil
	}
	if startTime.After(endTime) {
		startTime, endTime = endTime, startTime
	}

	queries := map[string]string{
		"platform": `
			SELECT
				COALESCE(NULLIF(o.platform, ''), g.platform, a.platform, 'unknown') AS group_value,
				COUNT(*) AS error_count,
				COUNT(*) FILTER (
					WHERE
						o.error_type = 'network_error'
						OR o.error_message ILIKE '%http2%'
						OR o.error_message ILIKE '%http/2%'
				) AS http2_errors,
				COUNT(*) FILTER (WHERE o.status_code >= 400 AND o.status_code < 500) AS error_4xx_count,
				COUNT(*) FILTER (WHERE o.status_code >= 500) AS error_5xx_count,
				COUNT(*) FILTER (
					WHERE
						o.error_type IN ('timeout', 'timeout_error')
						OR o.error_message ILIKE '%timeout%'
						OR o.error_message ILIKE '%deadline exceeded%'
				) AS timeout_count
			FROM ops_error_logs o
			LEFT JOIN groups g ON g.id = o.group_id
			LEFT JOIN accounts a ON a.id = o.account_id
			WHERE o.created_at >= $1 AND o.created_at < $2
			GROUP BY 1
			ORDER BY error_count DESC, 1 ASC
		`,
		"phase": `
			SELECT
				COALESCE(NULLIF(o.error_phase, ''), 'unknown') AS group_value,
				COUNT(*) AS error_count,
				COUNT(*) FILTER (
					WHERE
						o.error_type = 'network_error'
						OR o.error_message ILIKE '%http2%'
						OR o.error_message ILIKE '%http/2%'
				) AS http2_errors,
				COUNT(*) FILTER (WHERE o.status_code >= 400 AND o.status_code < 500) AS error_4xx_count,
				COUNT(*) FILTER (WHERE o.status_code >= 500) AS error_5xx_count,
				COUNT(*) FILTER (
					WHERE
						o.error_type IN ('timeout', 'timeout_error')
						OR o.error_message ILIKE '%timeout%'
						OR o.error_message ILIKE '%deadline exceeded%'
				) AS timeout_count
			FROM ops_error_logs o
			LEFT JOIN groups g ON g.id = o.group_id
			LEFT JOIN accounts a ON a.id = o.account_id
			WHERE o.created_at >= $1 AND o.created_at < $2
			GROUP BY 1
			ORDER BY error_count DESC, 1 ASC
		`,
		"severity": `
			SELECT
				COALESCE(NULLIF(o.severity, ''), 'unknown') AS group_value,
				COUNT(*) AS error_count,
				COUNT(*) FILTER (
					WHERE
						o.error_type = 'network_error'
						OR o.error_message ILIKE '%http2%'
						OR o.error_message ILIKE '%http/2%'
				) AS http2_errors,
				COUNT(*) FILTER (WHERE o.status_code >= 400 AND o.status_code < 500) AS error_4xx_count,
				COUNT(*) FILTER (WHERE o.status_code >= 500) AS error_5xx_count,
				COUNT(*) FILTER (
					WHERE
						o.error_type IN ('timeout', 'timeout_error')
						OR o.error_message ILIKE '%timeout%'
						OR o.error_message ILIKE '%deadline exceeded%'
				) AS timeout_count
			FROM ops_error_logs o
			LEFT JOIN groups g ON g.id = o.group_id
			LEFT JOIN accounts a ON a.id = o.account_id
			WHERE o.created_at >= $1 AND o.created_at < $2
			GROUP BY 1
			ORDER BY error_count DESC, 1 ASC
		`,
	}

	query, ok := queries[groupBy]
	if !ok {
		return nil, fmt.Errorf("invalid groupBy: %q", groupBy)
	}

	rows, err := r.sql.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]*service.OpsWindowStatsGroupedItem, 0)
	for rows.Next() {
		var item service.OpsWindowStatsGroupedItem
		var http2Errors int64
		if err := rows.Scan(
			&item.Group,
			&item.ErrorCount,
			&http2Errors,
			&item.Error4xxCount,
			&item.Error5xxCount,
			&item.TimeoutCount,
		); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// GetOverviewStats powers the ops "dashboard overview" endpoint.
//
// The legacy implementation runs percentile queries on raw logs. When the feature
// flag is enabled, this method prefers the pre-aggregation tables for older data
// (full buckets only) and uses the raw-log query only for the newest <1h slice.
func (r *OpsRepository) GetOverviewStats(ctx context.Context, startTime, endTime time.Time) (*service.OverviewStats, error) {
	if !r.usePreaggregatedTables {
		return r.GetOverviewStatsLegacy(ctx, startTime, endTime)
	}
	stats, err := r.getOverviewStatsPreaggregated(ctx, startTime, endTime)
	if err == nil {
		return stats, nil
	}
	r.recordOpsPreaggFallback(opsPreaggFallbackOverviewStats, startTime, endTime, err)
	return r.GetOverviewStatsLegacy(ctx, startTime, endTime)
}

// GetProviderStats backs the "provider health" dashboard view.
//
// With `ops.use_preaggregated_tables=true`, it sums ops_metrics_hourly/ops_metrics_daily
// for full buckets and uses the legacy raw-log query only for the newest <1h slice
// (and for small boundary fragments that don't align to full hour buckets).
func (r *OpsRepository) GetProviderStats(ctx context.Context, startTime, endTime time.Time) ([]*service.ProviderStats, error) {
	if !r.usePreaggregatedTables {
		return r.GetProviderStatsLegacy(ctx, startTime, endTime)
	}
	stats, err := r.getProviderStatsPreaggregated(ctx, startTime, endTime)
	if err == nil {
		return stats, nil
	}
	r.recordOpsPreaggFallback(opsPreaggFallbackProviderStats, startTime, endTime, err)
	return r.GetProviderStatsLegacy(ctx, startTime, endTime)
}

// GetLatencyHistogram returns a coarse latency histogram used by the ops UI.
//
// Note: ops_metrics_hourly/daily do not store per-request latency distributions; when
// pre-aggregation is enabled this method approximates older data by bucketing each
// aggregated row by its avg_latency_ms (weighted by success_count), and uses raw logs
// for the newest <1h slice.
//
// Bucket definitions (milliseconds, left-inclusive / right-exclusive):
// - "0-100ms"      : duration_ms/avg_latency_ms < 100
// - "100-200ms"    : duration_ms/avg_latency_ms < 200
// - "200-500ms"    : duration_ms/avg_latency_ms < 500
// - "500-1000ms"   : duration_ms/avg_latency_ms < 1000
// - "1000-2000ms"  : duration_ms/avg_latency_ms < 2000
// - "2000ms+"      : duration_ms/avg_latency_ms >= 2000
//
// Important: both the legacy (raw logs) and pre-aggregated SQL paths must keep the
// same bucket boundaries and labels so the frontend can render consistent charts. Since
// bucketing happens at query-time, changing bucket definitions does not require a data
// backfill/re-aggregation, but historical charts will shift accordingly.
func (r *OpsRepository) GetLatencyHistogram(ctx context.Context, startTime, endTime time.Time) ([]*service.LatencyHistogramItem, error) {
	if !r.usePreaggregatedTables {
		return r.GetLatencyHistogramLegacy(ctx, startTime, endTime)
	}
	items, err := r.getLatencyHistogramPreaggregated(ctx, startTime, endTime)
	if err == nil {
		return items, nil
	}
	r.recordOpsPreaggFallback(opsPreaggFallbackLatencyHistogram, startTime, endTime, err)
	return r.GetLatencyHistogramLegacy(ctx, startTime, endTime)
}

func (r *OpsRepository) recordOpsPreaggFallback(method opsPreaggFallbackMethod, startTime, endTime time.Time, err error) {
	if err == nil {
		return
	}

	isNotPopulated := errors.Is(err, errOpsPreaggregatedNotPopulated)

	switch method {
	case opsPreaggFallbackWindowStats:
		if isNotPopulated {
			metrics.IncOpsPreaggFallbackWindowStatsNotPopulated()
		} else {
			metrics.IncOpsPreaggFallbackWindowStatsUnexpectedError()
		}
	case opsPreaggFallbackOverviewStats:
		if isNotPopulated {
			metrics.IncOpsPreaggFallbackOverviewStatsNotPopulated()
		} else {
			metrics.IncOpsPreaggFallbackOverviewStatsUnexpectedError()
		}
	case opsPreaggFallbackProviderStats:
		if isNotPopulated {
			metrics.IncOpsPreaggFallbackProviderStatsNotPopulated()
		} else {
			metrics.IncOpsPreaggFallbackProviderStatsUnexpectedError()
		}
	case opsPreaggFallbackLatencyHistogram:
		if isNotPopulated {
			metrics.IncOpsPreaggFallbackLatencyHistogramNotPopulated()
		} else {
			metrics.IncOpsPreaggFallbackLatencyHistogramUnexpectedError()
		}
	default:
		metrics.IncOpsPreaggFallbackUnknownMethod()
		return
	}

	// Keep logs quiet for "not populated" (expected during backfills / initial rollout).
	// For unexpected errors, log so operators have immediate context.
	if !isNotPopulated {
		now := time.Now().UTC()
		opsPreaggFallbackLogMu.Lock()
		last := opsPreaggFallbackLastLogUTC[method]
		shouldLog := last.IsZero() || now.Sub(last) >= opsPreaggFallbackUnexpectedLogMinInterval
		if shouldLog {
			opsPreaggFallbackLastLogUTC[method] = now
		}
		opsPreaggFallbackLogMu.Unlock()

		if !shouldLog {
			return
		}

		log.Printf(
			"ops preagg fallback: method=%s start=%s end=%s err=%v",
			string(method),
			startTime.UTC().Format(time.RFC3339Nano),
			endTime.UTC().Format(time.RFC3339Nano),
			err,
		)
	}
}

func nullInt64Ptr(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}
