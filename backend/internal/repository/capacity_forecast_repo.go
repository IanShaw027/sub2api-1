package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// capacityForecastRepository implements service.CapacityRepository with raw SQL
// against the generic account_quota_snapshots / account_quota_periods /
// capacity_hourly tables (migration 229) and the existing usage_logs table.
type capacityForecastRepository struct {
	sql sqlExecutor
}

// NewCapacityForecastRepository creates the generic capacity-forecast repository.
func NewCapacityForecastRepository(sqlDB *sql.DB) service.CapacityRepository {
	if sqlDB == nil {
		return nil
	}
	return &capacityForecastRepository{sql: sqlDB}
}

// GetHourlySpend returns actual per-hour spend (SUM(total_cost)) for the given
// platform scope. When groupID is non-nil, spend is scoped to that group;
// otherwise it is scoped to the whole platform via accounts.platform (the
// account -> platform relationship is authoritative, unlike the free-text
// usage_logs.provider column).
func (r *capacityForecastRepository) GetHourlySpend(ctx context.Context, platform string, groupID *int64, from, to time.Time) ([]service.CapacityHourlySpendPoint, error) {
	if r == nil || r.sql == nil {
		return nil, nil
	}
	const query = `
		SELECT date_trunc('hour', ul.created_at) AS bucket_start, COALESCE(SUM(ul.total_cost), 0) AS spend_usd
		FROM usage_logs ul
		JOIN accounts a ON a.id = ul.account_id
		WHERE a.platform = $1
		  AND ($2::bigint IS NULL OR ul.group_id = $2)
		  AND ul.created_at >= $3
		  AND ul.created_at < $4
		GROUP BY 1
		ORDER BY 1`
	rows, err := r.sql.QueryContext(ctx, query, platform, groupID, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var points []service.CapacityHourlySpendPoint
	for rows.Next() {
		var p service.CapacityHourlySpendPoint
		if err := rows.Scan(&p.BucketStart, &p.SpendUSD); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// GetAccountSpendBetween sums a single account's total_cost in [from, to).
func (r *capacityForecastRepository) GetAccountSpendBetween(ctx context.Context, accountID int64, from, to time.Time) (float64, error) {
	if r == nil || r.sql == nil || accountID <= 0 {
		return 0, nil
	}
	const query = `
		SELECT COALESCE(SUM(total_cost), 0)
		FROM usage_logs
		WHERE account_id = $1 AND created_at >= $2 AND created_at < $3`
	rows, err := r.sql.QueryContext(ctx, query, accountID, from, to)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	var spend float64
	if rows.Next() {
		if err := rows.Scan(&spend); err != nil {
			return 0, err
		}
	}
	return spend, rows.Err()
}

// capacityWindowSpendChunkSize bounds one batched VALUES query to 3 params per
// request, staying far below Postgres's 65535-parameter limit.
const capacityWindowSpendChunkSize = 500

// GetAccountWindowSpends sums each requested (account, window) pair's
// total_cost over its own [From, to) in one query per chunk, instead of one
// query per pair.
func (r *capacityForecastRepository) GetAccountWindowSpends(ctx context.Context, reqs []service.CapacityAccountWindowSpendRequest, to time.Time) (map[service.CapacityAccountWindowSpendKey]float64, error) {
	result := make(map[service.CapacityAccountWindowSpendKey]float64, len(reqs))
	if r == nil || r.sql == nil || len(reqs) == 0 {
		return result, nil
	}
	for start := 0; start < len(reqs); start += capacityWindowSpendChunkSize {
		end := start + capacityWindowSpendChunkSize
		if end > len(reqs) {
			end = len(reqs)
		}
		if err := r.getAccountWindowSpendsChunk(ctx, reqs[start:end], to, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *capacityForecastRepository) getAccountWindowSpendsChunk(ctx context.Context, reqs []service.CapacityAccountWindowSpendRequest, to time.Time, out map[service.CapacityAccountWindowSpendKey]float64) error {
	var values strings.Builder
	args := make([]any, 0, len(reqs)*3+1)
	args = append(args, to)
	for i, req := range reqs {
		if i > 0 {
			_, _ = values.WriteString(", ")
		}
		fmt.Fprintf(&values, "($%d::bigint, $%d::text, $%d::timestamptz)", len(args)+1, len(args)+2, len(args)+3)
		args = append(args, req.AccountID, req.WindowKind, req.From)
	}
	query := `
		SELECT v.account_id, v.window_kind, COALESCE(SUM(ul.total_cost), 0)
		FROM (VALUES ` + values.String() + `) AS v(account_id, window_kind, from_at)
		LEFT JOIN usage_logs ul
		  ON ul.account_id = v.account_id
		 AND ul.created_at >= v.from_at
		 AND ul.created_at < $1
		GROUP BY v.account_id, v.window_kind`
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var key service.CapacityAccountWindowSpendKey
		var spend float64
		if err := rows.Scan(&key.AccountID, &key.WindowKind, &spend); err != nil {
			return err
		}
		out[key] = spend
	}
	return rows.Err()
}

// GetHourlyFacts reads previously sealed capacity_hourly rows for display of
// past available/capacity — missing hours are simply absent from the map, the
// caller must render them as null rather than fabricating a value.
func (r *capacityForecastRepository) GetHourlyFacts(ctx context.Context, platform string, groupID *int64, from, to time.Time) (map[time.Time]service.CapacityHourlyFact, error) {
	if r == nil || r.sql == nil {
		return nil, nil
	}
	const query = `
		SELECT bucket_start, available_usd, capacity_usd
		FROM capacity_hourly
		WHERE platform = $1
		  AND ((group_id IS NULL AND $2::bigint IS NULL) OR group_id = $2)
		  AND bucket_start >= $3
		  AND bucket_start < $4`
	rows, err := r.sql.QueryContext(ctx, query, platform, groupID, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	facts := make(map[time.Time]service.CapacityHourlyFact)
	for rows.Next() {
		var (
			bucket    time.Time
			available sql.NullFloat64
			capacity  sql.NullFloat64
		)
		if err := rows.Scan(&bucket, &available, &capacity); err != nil {
			return nil, err
		}
		fact := service.CapacityHourlyFact{}
		if available.Valid {
			v := available.Float64
			fact.AvailableUSD = &v
		}
		if capacity.Valid {
			v := capacity.Float64
			fact.CapacityUSD = &v
		}
		facts[bucket.UTC()] = fact
	}
	return facts, rows.Err()
}

// UpsertHourlyFacts seals current-hour (or backstop) capacity_hourly rows.
// Rows are simply overwritten on conflict (last write wins) — the generic
// engine does not replicate the old OpenAI table's "freeze once sealed"
// behaviour, since the current hour is by definition still in progress.
func (r *capacityForecastRepository) UpsertHourlyFacts(ctx context.Context, facts []service.CapacityHourlyUpsert) error {
	if r == nil || r.sql == nil || len(facts) == 0 {
		return nil
	}
	const query = `
		INSERT INTO capacity_hourly (platform, group_id, bucket_start, spend_usd, available_usd, capacity_usd, sealed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (platform, COALESCE(group_id, 0), bucket_start)
		DO UPDATE SET
			spend_usd = EXCLUDED.spend_usd,
			available_usd = EXCLUDED.available_usd,
			capacity_usd = EXCLUDED.capacity_usd,
			sealed = EXCLUDED.sealed,
			updated_at = NOW()`
	for _, fact := range facts {
		if _, err := r.sql.ExecContext(ctx, query,
			fact.Platform, fact.GroupID, fact.BucketStart, fact.SpendUSD,
			fact.AvailableUSD, fact.CapacityUSD, fact.Sealed,
		); err != nil {
			return err
		}
	}
	return nil
}

// UpsertQuotaSnapshot writes the latest known window state for an account,
// replacing any previous snapshot for the same (platform, account, window_kind).
func (r *capacityForecastRepository) UpsertQuotaSnapshot(ctx context.Context, snapshot service.AccountQuotaSnapshot) error {
	if r == nil || r.sql == nil {
		return nil
	}
	const query = `
		INSERT INTO account_quota_snapshots (platform, account_id, window_kind, used_ratio, reset_at, window_minutes, observed_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (platform, account_id, window_kind)
		DO UPDATE SET
			used_ratio = EXCLUDED.used_ratio,
			reset_at = EXCLUDED.reset_at,
			window_minutes = EXCLUDED.window_minutes,
			observed_at = EXCLUDED.observed_at,
			updated_at = NOW()`
	var resetAt any
	if !snapshot.ResetAt.IsZero() {
		resetAt = snapshot.ResetAt
	}
	var windowMinutes any
	if snapshot.WindowMinutes > 0 {
		windowMinutes = snapshot.WindowMinutes
	}
	_, err := r.sql.ExecContext(ctx, query,
		snapshot.Platform, snapshot.AccountID, snapshot.WindowKind, snapshot.UsedRatio,
		resetAt, windowMinutes, snapshot.ObservedAt,
	)
	return err
}

// InsertQuotaPeriod archives a closed rolling window. Idempotent: once a
// (platform, account, window_kind, period_start) period is archived it is
// never overwritten.
func (r *capacityForecastRepository) InsertQuotaPeriod(ctx context.Context, period service.AccountQuotaPeriod) error {
	if r == nil || r.sql == nil {
		return nil
	}
	const query = `
		INSERT INTO account_quota_periods (platform, account_id, window_kind, period_start, period_end, final_used_ratio, spend_usd, inferred_capacity_usd, sample_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		ON CONFLICT (platform, account_id, window_kind, period_start) DO NOTHING`
	_, err := r.sql.ExecContext(ctx, query,
		period.Platform, period.AccountID, period.WindowKind, period.PeriodStart, period.PeriodEnd,
		period.FinalUsedRatio, period.SpendUSD, period.InferredCapacityUSD, period.SampleCount,
	)
	return err
}

// GetLatestQuotaPeriodCapacity returns the most recently closed period's
// inferred capacity for (platform, account, window_kind), used as the first
// capacity-inference fallback when the live window's used_ratio is too low.
func (r *capacityForecastRepository) GetLatestQuotaPeriodCapacity(ctx context.Context, platform string, accountID int64, windowKind string) (*float64, error) {
	if r == nil || r.sql == nil {
		return nil, nil
	}
	const query = `
		SELECT inferred_capacity_usd
		FROM account_quota_periods
		WHERE platform = $1 AND account_id = $2 AND window_kind = $3 AND inferred_capacity_usd IS NOT NULL
		ORDER BY period_end DESC
		LIMIT 1`
	rows, err := r.sql.QueryContext(ctx, query, platform, accountID, windowKind)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var capacity float64
	if err := rows.Scan(&capacity); err != nil {
		return nil, err
	}
	return &capacity, rows.Err()
}
