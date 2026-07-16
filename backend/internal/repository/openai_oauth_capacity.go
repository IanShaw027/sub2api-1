package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const openAIOAuthCapacityCostSQL = `COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)`

func hasOpenAIOAuthQuotaSnapshotUpdates(updates map[string]any) bool {
	for key := range updates {
		if strings.HasPrefix(key, "codex_5h_") || strings.HasPrefix(key, "codex_7d_") {
			return true
		}
	}
	return false
}

func (r *accountRepository) persistOpenAIOAuthQuotaSnapshot(ctx context.Context, accountID int64) error {
	if r == nil || r.sql == nil || accountID <= 0 {
		return nil
	}
	// Unit repositories use lightweight SQL recorders that do not own the
	// migrated analytics tables. Production and integration paths use *sql.DB.
	if _, ok := r.sql.(*sql.DB); !ok {
		return nil
	}
	if _, err := r.sql.ExecContext(ctx, openAIOAuthArchiveCompletedPeriodsSQL, accountID); err != nil {
		return err
	}
	_, err := r.sql.ExecContext(ctx, openAIOAuthUpsertSnapshotSQL, accountID)
	return err
}

const openAIOAuthArchiveCompletedPeriodsSQL = `
	WITH current_state AS (
		SELECT
			a.id AS account_id,
			COALESCE(NULLIF(a.credentials->>'plan_type', ''), NULLIF(a.extra->>'subscription_type', ''), 'unknown') AS plan_type,
			NULLIF(a.extra->>'codex_5h_reset_at', '')::timestamptz AS five_reset,
			COALESCE(NULLIF(a.extra->>'codex_5h_window_minutes', '')::int, 300) AS five_minutes,
			NULLIF(a.extra->>'codex_7d_reset_at', '')::timestamptz AS seven_reset,
			COALESCE(NULLIF(a.extra->>'codex_7d_window_minutes', '')::int, 10080) AS seven_minutes
		FROM accounts a
		WHERE a.id = $1
		  AND a.deleted_at IS NULL
		  AND a.platform = 'openai'
		  AND a.type IN ('oauth', 'setup-token')
	), previous AS (
		SELECT s.*
		FROM openai_oauth_quota_snapshots s
		WHERE s.account_id = $1
		ORDER BY s.observed_at DESC
		LIMIT 1
	), completed AS (
		SELECT
			p.account_id,
			p.plan_type,
			'5h'::text AS window_kind,
			p.five_hour_reset_at - make_interval(mins => COALESCE(p.five_hour_window_minutes, 300)) AS period_start,
			p.five_hour_reset_at AS period_end,
			p.five_hour_used_percent AS final_used_percent,
			COALESCE(p.five_hour_window_minutes, 300) AS window_minutes
		FROM previous p
		JOIN current_state c ON c.account_id = p.account_id
		WHERE p.five_hour_reset_at IS NOT NULL
		  AND c.five_reset > p.five_hour_reset_at + make_interval(mins => COALESCE(p.five_hour_window_minutes, 300) / 2)
		UNION ALL
		SELECT
			p.account_id,
			p.plan_type,
			'7d'::text,
			p.seven_day_reset_at - make_interval(mins => COALESCE(p.seven_day_window_minutes, 10080)),
			p.seven_day_reset_at,
			p.seven_day_used_percent,
			COALESCE(p.seven_day_window_minutes, 10080)
		FROM previous p
		JOIN current_state c ON c.account_id = p.account_id
		WHERE p.seven_day_reset_at IS NOT NULL
		  AND c.seven_reset > p.seven_day_reset_at + make_interval(mins => COALESCE(p.seven_day_window_minutes, 10080) / 2)
	), summarized AS (
		SELECT
			c.*,
			COALESCE((
				SELECT SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1))
				FROM usage_logs ul
				WHERE ul.account_id = c.account_id
				  AND ul.created_at >= c.period_start
				  AND ul.created_at < c.period_end
			), 0) AS spend_usd,
			(
				SELECT COUNT(*)::int
				FROM openai_oauth_quota_snapshots sample
				WHERE sample.account_id = c.account_id
				  AND sample.observed_at >= c.period_start
				  AND sample.observed_at <= c.period_end
			) AS sample_count
		FROM completed c
	)
	INSERT INTO openai_oauth_quota_periods (
		account_id, plan_type, window_kind, period_start, period_end,
		final_used_percent, spend_usd, inferred_capacity_usd, sample_count
	)
	SELECT
		account_id,
		plan_type,
		window_kind,
		period_start,
		period_end,
		final_used_percent,
		spend_usd,
		CASE
			WHEN final_used_percent >= 5 AND spend_usd > 0
			THEN spend_usd * 100 / final_used_percent
		END,
		sample_count
	FROM summarized
	ON CONFLICT (account_id, window_kind, period_end) DO NOTHING
`

const openAIOAuthUpsertSnapshotSQL = `
	WITH upserted AS (
	INSERT INTO openai_oauth_quota_snapshots (
		account_id,
		plan_type,
		observed_at,
		five_hour_used_percent,
		five_hour_reset_at,
		five_hour_window_minutes,
		seven_day_used_percent,
		seven_day_reset_at,
		seven_day_window_minutes
	)
	SELECT
		a.id,
		COALESCE(NULLIF(a.credentials->>'plan_type', ''), NULLIF(a.extra->>'subscription_type', ''), 'unknown'),
		date_bin(
			INTERVAL '15 minutes',
			COALESCE(NULLIF(a.extra->>'codex_usage_updated_at', '')::timestamptz, NOW()),
			TIMESTAMPTZ '2000-01-01 00:00:00+00'
		),
		NULLIF(a.extra->>'codex_5h_used_percent', '')::numeric,
		NULLIF(a.extra->>'codex_5h_reset_at', '')::timestamptz,
		COALESCE(NULLIF(a.extra->>'codex_5h_window_minutes', '')::int, 300),
		NULLIF(a.extra->>'codex_7d_used_percent', '')::numeric,
		NULLIF(a.extra->>'codex_7d_reset_at', '')::timestamptz,
		COALESCE(NULLIF(a.extra->>'codex_7d_window_minutes', '')::int, 10080)
	FROM accounts a
	WHERE a.id = $1
	  AND a.deleted_at IS NULL
	  AND a.platform = 'openai'
	  AND a.type IN ('oauth', 'setup-token')
	ON CONFLICT (account_id, observed_at) DO UPDATE SET
		plan_type = EXCLUDED.plan_type,
		five_hour_used_percent = EXCLUDED.five_hour_used_percent,
		five_hour_reset_at = EXCLUDED.five_hour_reset_at,
		five_hour_window_minutes = EXCLUDED.five_hour_window_minutes,
		seven_day_used_percent = EXCLUDED.seven_day_used_percent,
		seven_day_reset_at = EXCLUDED.seven_day_reset_at,
		seven_day_window_minutes = EXCLUDED.seven_day_window_minutes
	WHERE openai_oauth_quota_snapshots.five_hour_reset_at IS DISTINCT FROM EXCLUDED.five_hour_reset_at
	   OR openai_oauth_quota_snapshots.seven_day_reset_at IS DISTINCT FROM EXCLUDED.seven_day_reset_at
	   OR ABS(COALESCE(openai_oauth_quota_snapshots.five_hour_used_percent, 0) - COALESCE(EXCLUDED.five_hour_used_percent, 0)) >= 0.5
	   OR ABS(COALESCE(openai_oauth_quota_snapshots.seven_day_used_percent, 0) - COALESCE(EXCLUDED.seven_day_used_percent, 0)) >= 0.5
	RETURNING id
	)
	DELETE FROM openai_oauth_quota_snapshots
	WHERE account_id = $1
	  AND observed_at < NOW() - INTERVAL '90 days'
`

func (r *usageLogRepository) GetOpenAIOAuthCapacityRawData(ctx context.Context, now time.Time) (*service.OpenAIOAuthCapacityRawData, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("usage log repository is not configured")
	}
	rows, err := r.sql.QueryContext(ctx, openAIOAuthCapacityQuery, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := &service.OpenAIOAuthCapacityRawData{}
	for rows.Next() {
		var item service.OpenAIOAuthCapacityRawRow
		var rateLimitResetAt, usageUpdatedAt sql.NullTime
		var fiveUsed, sevenUsed sql.NullFloat64
		var fiveReset, sevenReset sql.NullTime
		if err := rows.Scan(
			&item.AccountID,
			&item.GroupID,
			&item.GroupName,
			&item.PlanType,
			&item.Status,
			&item.Schedulable,
			&rateLimitResetAt,
			&usageUpdatedAt,
			&fiveUsed,
			&fiveReset,
			&item.FiveHourWindowMinutes,
			&sevenUsed,
			&sevenReset,
			&item.SevenDayWindowMinutes,
			&item.FiveHourAccountSpend,
			&item.FiveHourGroupSpend,
			&item.SevenDayAccountSpend,
			&item.SevenDayGroupSpend,
			&item.RecentThreeHourSpend,
			&item.PreviousDayThreeHourSpend,
		); err != nil {
			return nil, err
		}
		item.RateLimitResetAt = capacityNullTimePtr(rateLimitResetAt)
		item.UsageUpdatedAt = capacityNullTimePtr(usageUpdatedAt)
		item.FiveHourUsedPercent = capacityNullFloat64Ptr(fiveUsed)
		item.FiveHourResetAt = capacityNullTimePtr(fiveReset)
		item.SevenDayUsedPercent = capacityNullFloat64Ptr(sevenUsed)
		item.SevenDayResetAt = capacityNullTimePtr(sevenReset)
		result.Rows = append(result.Rows, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	baselines, err := r.getOpenAIOAuthCapacityBaselines(ctx, now)
	if err != nil {
		return nil, err
	}
	result.Baselines = baselines
	return result, nil
}

func (r *usageLogRepository) getOpenAIOAuthCapacityBaselines(ctx context.Context, now time.Time) ([]service.OpenAIOAuthCapacityBaselineRaw, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT
			plan_type,
			window_kind,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY inferred_capacity_usd)::double precision AS median_capacity_usd,
			COUNT(*)::int AS samples
		FROM openai_oauth_quota_periods
		WHERE period_end >= $1::timestamptz - INTERVAL '90 days'
		  AND inferred_capacity_usd > 0
		GROUP BY plan_type, window_kind
		HAVING COUNT(*) >= 3
		ORDER BY plan_type, window_kind
	`, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.OpenAIOAuthCapacityBaselineRaw, 0)
	for rows.Next() {
		var item service.OpenAIOAuthCapacityBaselineRaw
		if err := rows.Scan(&item.PlanType, &item.Window, &item.MedianCapacityUSD, &item.Samples); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func capacityNullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func capacityNullFloat64Ptr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

// GetOpenAIOAuthCapacityHourlySpend returns per-hour account-cost USD spend for OpenAI OAuth accounts.
// Buckets are UTC hour starts. group_id uses the request-time usage_logs.group_id (0 when null).
// Prefer GetOpenAIOAuthCapacityHourlyFacts for sealed history; this scans usage_logs directly.
func (r *usageLogRepository) GetOpenAIOAuthCapacityHourlySpend(ctx context.Context, from, to time.Time) ([]service.OpenAIOAuthHourlySpendRow, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("usage log repository is not configured")
	}
	if !to.After(from) {
		return []service.OpenAIOAuthHourlySpendRow{}, nil
	}
	rows, err := r.sql.QueryContext(ctx, openAIOAuthHourlySpendQuery, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.OpenAIOAuthHourlySpendRow, 0)
	for rows.Next() {
		var item service.OpenAIOAuthHourlySpendRow
		if err := rows.Scan(&item.BucketStart, &item.GroupID, &item.SpendUSD); err != nil {
			return nil, err
		}
		item.BucketStart = item.BucketStart.UTC()
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetOpenAIOAuthCapacityHourlyFacts returns sealed (and optional unsealed) hourly facts.
// groupID nil means global total rows (group_id=-1).
func (r *usageLogRepository) GetOpenAIOAuthCapacityHourlyFacts(
	ctx context.Context,
	from, to time.Time,
	groupID *int64,
	sealedOnly bool,
) ([]service.OpenAIOAuthCapacityHourlyFact, error) {
	if r == nil || r.sql == nil {
		return nil, fmt.Errorf("usage log repository is not configured")
	}
	if !to.After(from) {
		return []service.OpenAIOAuthCapacityHourlyFact{}, nil
	}
	gid := service.OpenAIOAuthCapacityAllGroupsID
	if groupID != nil {
		gid = *groupID
	}
	query := openAIOAuthHourlyFactsQuery
	if sealedOnly {
		query = openAIOAuthHourlyFactsSealedQuery
	}
	rows, err := r.sql.QueryContext(ctx, query, from.UTC(), to.UTC(), gid)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.OpenAIOAuthCapacityHourlyFact, 0)
	for rows.Next() {
		var (
			item                                     service.OpenAIOAuthCapacityHourlyFact
			cap5, avail5, used5, cap7, avail7, used7 sql.NullFloat64
		)
		if err := rows.Scan(
			&item.BucketStart,
			&item.GroupID,
			&item.PlanType,
			&item.SpentUSD,
			&cap5, &avail5, &used5,
			&cap7, &avail7, &used7,
			&item.Sealed,
			&item.Source,
		); err != nil {
			return nil, err
		}
		item.BucketStart = item.BucketStart.UTC()
		item.Capacity5hUSD = capacityNullFloat64Ptr(cap5)
		item.Available5hUSD = capacityNullFloat64Ptr(avail5)
		item.UsedPercent5h = capacityNullFloat64Ptr(used5)
		item.Capacity7dUSD = capacityNullFloat64Ptr(cap7)
		item.Available7dUSD = capacityNullFloat64Ptr(avail7)
		item.UsedPercent7d = capacityNullFloat64Ptr(used7)
		result = append(result, item)
	}
	return result, rows.Err()
}

// UpsertOpenAIOAuthCapacityHourlyFacts persists hourly facts in one multi-row statement.
// Sealed spend freezes; nullable metric columns are retained for schema compatibility.
func (r *usageLogRepository) UpsertOpenAIOAuthCapacityHourlyFacts(ctx context.Context, facts []service.OpenAIOAuthCapacityHourlyFact) error {
	if r == nil || r.sql == nil {
		return fmt.Errorf("usage log repository is not configured")
	}
	if len(facts) == 0 {
		return nil
	}
	// Chunk large batches to keep query size bounded.
	const chunkSize = 100
	for start := 0; start < len(facts); start += chunkSize {
		end := start + chunkSize
		if end > len(facts) {
			end = len(facts)
		}
		if err := r.upsertOpenAIOAuthCapacityHourlyFactsChunk(ctx, facts[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (r *usageLogRepository) upsertOpenAIOAuthCapacityHourlyFactsChunk(ctx context.Context, facts []service.OpenAIOAuthCapacityHourlyFact) error {
	if len(facts) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString(`INSERT INTO openai_oauth_capacity_hourly (
		bucket_start, group_id, plan_type, spent_usd,
		capacity_5h_usd, available_5h_usd, used_percent_5h,
		capacity_7d_usd, available_7d_usd, used_percent_7d,
		sealed, source, computed_at
	) VALUES `)
	args := make([]any, 0, len(facts)*12)
	for i, fact := range facts {
		if i > 0 {
			b.WriteByte(',')
		}
		base := i*12 + 1
		fmt.Fprintf(&b, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,NOW())",
			base, base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10, base+11)
		args = append(args,
			fact.BucketStart.UTC(),
			fact.GroupID,
			normalizeCapacityPlanType(fact.PlanType),
			fact.SpentUSD,
			nullFloat64Arg(fact.Capacity5hUSD),
			nullFloat64Arg(fact.Available5hUSD),
			nullFloat64Arg(fact.UsedPercent5h),
			nullFloat64Arg(fact.Capacity7dUSD),
			nullFloat64Arg(fact.Available7dUSD),
			nullFloat64Arg(fact.UsedPercent7d),
			fact.Sealed,
			defaultCapacitySource(fact.Source),
		)
	}
	b.WriteString(openAIOAuthHourlyUpsertConflictSQL)
	_, err := r.sql.ExecContext(ctx, b.String(), args...)
	return err
}

// GetOpenAIOAuthCapacityRecentSpend sums account-cost USD spend in [from, to) for OpenAI OAuth accounts.
// When groupID is non-nil, only that group's request-time usage is included.
func (r *usageLogRepository) GetOpenAIOAuthCapacityRecentSpend(ctx context.Context, from, to time.Time, groupID *int64) (float64, error) {
	if r == nil || r.sql == nil {
		return 0, fmt.Errorf("usage log repository is not configured")
	}
	if !to.After(from) {
		return 0, nil
	}
	var (
		rows *sql.Rows
		err  error
	)
	if groupID == nil {
		rows, err = r.sql.QueryContext(ctx, openAIOAuthRecentSpendAllQuery, from.UTC(), to.UTC())
	} else {
		rows, err = r.sql.QueryContext(ctx, openAIOAuthRecentSpendGroupQuery, from.UTC(), to.UTC(), *groupID)
	}
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var total float64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, rows.Err()
}

func normalizeCapacityPlanType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "all"
	}
	return value
}

func defaultCapacitySource(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "usage_logs"
	}
	return value
}

func nullFloat64Arg(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

var openAIOAuthHourlySpendQuery = fmt.Sprintf(`
	SELECT
		date_trunc('hour', ul.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket_start,
		COALESCE(ul.group_id, 0) AS group_id,
		COALESCE(SUM(%[1]s), 0)::double precision AS spend_usd
	FROM usage_logs ul
	JOIN accounts a ON a.id = ul.account_id
	WHERE a.platform = 'openai'
	  AND a.type IN ('oauth', 'setup-token')
	  AND ul.created_at >= $1::timestamptz
	  AND ul.created_at < $2::timestamptz
	GROUP BY 1, 2
	ORDER BY 1, 2
`, openAIOAuthCapacityCostSQL)

const openAIOAuthHourlyFactsSelect = `
	SELECT
		bucket_start,
		group_id,
		plan_type,
		spent_usd::double precision,
		capacity_5h_usd::double precision,
		available_5h_usd::double precision,
		used_percent_5h::double precision,
		capacity_7d_usd::double precision,
		available_7d_usd::double precision,
		used_percent_7d::double precision,
		sealed,
		source
	FROM openai_oauth_capacity_hourly
	WHERE bucket_start >= $1::timestamptz
	  AND bucket_start < $2::timestamptz
	  AND group_id = $3
	  AND plan_type = 'all'
`

const openAIOAuthHourlyFactsQuery = openAIOAuthHourlyFactsSelect + `
	ORDER BY bucket_start
`

const openAIOAuthHourlyFactsSealedQuery = openAIOAuthHourlyFactsSelect + `
	  AND sealed = TRUE
	ORDER BY bucket_start
`

const openAIOAuthHourlyUpsertConflictSQL = `
	ON CONFLICT (bucket_start, group_id, plan_type) DO UPDATE SET
		spent_usd = CASE
			WHEN openai_oauth_capacity_hourly.sealed THEN openai_oauth_capacity_hourly.spent_usd
			ELSE EXCLUDED.spent_usd
		END,
		capacity_5h_usd = CASE
			WHEN openai_oauth_capacity_hourly.sealed AND openai_oauth_capacity_hourly.capacity_5h_usd IS NOT NULL
				THEN openai_oauth_capacity_hourly.capacity_5h_usd
			ELSE COALESCE(EXCLUDED.capacity_5h_usd, openai_oauth_capacity_hourly.capacity_5h_usd)
		END,
		available_5h_usd = CASE
			WHEN openai_oauth_capacity_hourly.sealed AND openai_oauth_capacity_hourly.available_5h_usd IS NOT NULL
				THEN openai_oauth_capacity_hourly.available_5h_usd
			ELSE COALESCE(EXCLUDED.available_5h_usd, openai_oauth_capacity_hourly.available_5h_usd)
		END,
		used_percent_5h = CASE
			WHEN openai_oauth_capacity_hourly.sealed AND openai_oauth_capacity_hourly.used_percent_5h IS NOT NULL
				THEN openai_oauth_capacity_hourly.used_percent_5h
			ELSE COALESCE(EXCLUDED.used_percent_5h, openai_oauth_capacity_hourly.used_percent_5h)
		END,
		capacity_7d_usd = CASE
			WHEN openai_oauth_capacity_hourly.sealed AND openai_oauth_capacity_hourly.capacity_7d_usd IS NOT NULL
				THEN openai_oauth_capacity_hourly.capacity_7d_usd
			ELSE COALESCE(EXCLUDED.capacity_7d_usd, openai_oauth_capacity_hourly.capacity_7d_usd)
		END,
		available_7d_usd = CASE
			WHEN openai_oauth_capacity_hourly.sealed AND openai_oauth_capacity_hourly.available_7d_usd IS NOT NULL
				THEN openai_oauth_capacity_hourly.available_7d_usd
			ELSE COALESCE(EXCLUDED.available_7d_usd, openai_oauth_capacity_hourly.available_7d_usd)
		END,
		used_percent_7d = CASE
			WHEN openai_oauth_capacity_hourly.sealed AND openai_oauth_capacity_hourly.used_percent_7d IS NOT NULL
				THEN openai_oauth_capacity_hourly.used_percent_7d
			ELSE COALESCE(EXCLUDED.used_percent_7d, openai_oauth_capacity_hourly.used_percent_7d)
		END,
		sealed = openai_oauth_capacity_hourly.sealed OR EXCLUDED.sealed,
		source = CASE
			WHEN openai_oauth_capacity_hourly.sealed THEN openai_oauth_capacity_hourly.source
			ELSE EXCLUDED.source
		END,
		computed_at = NOW()
`

var openAIOAuthRecentSpendAllQuery = fmt.Sprintf(`
	SELECT COALESCE(SUM(%[1]s), 0)::double precision
	FROM usage_logs ul
	JOIN accounts a ON a.id = ul.account_id
	WHERE a.deleted_at IS NULL
	  AND a.platform = 'openai'
	  AND a.type IN ('oauth', 'setup-token')
	  AND ul.created_at >= $1::timestamptz
	  AND ul.created_at < $2::timestamptz
`, openAIOAuthCapacityCostSQL)

var openAIOAuthRecentSpendGroupQuery = fmt.Sprintf(`
	SELECT COALESCE(SUM(%[1]s), 0)::double precision
	FROM usage_logs ul
	JOIN accounts a ON a.id = ul.account_id
	WHERE a.deleted_at IS NULL
	  AND a.platform = 'openai'
	  AND a.type IN ('oauth', 'setup-token')
	  AND ul.created_at >= $1::timestamptz
	  AND ul.created_at < $2::timestamptz
	  AND COALESCE(ul.group_id, 0) = $3
`, openAIOAuthCapacityCostSQL)

var openAIOAuthCapacityQuery = fmt.Sprintf(`
	WITH target AS (
		SELECT
			a.id,
			COALESCE(NULLIF(a.credentials->>'plan_type', ''), NULLIF(a.extra->>'subscription_type', ''), 'unknown') AS plan_type,
			a.status,
			a.schedulable,
			GREATEST(
				a.rate_limit_reset_at,
				CASE
					WHEN a.temp_unschedulable_reason LIKE '%%"source":"account_scheduling_threshold"%%'
					THEN a.temp_unschedulable_until
				END
			) AS rate_limit_reset_at,
			NULLIF(a.extra->>'codex_usage_updated_at', '')::timestamptz AS usage_updated_at,
			NULLIF(a.extra->>'codex_5h_used_percent', '')::double precision AS five_used,
			NULLIF(a.extra->>'codex_5h_reset_at', '')::timestamptz AS five_reset,
			COALESCE(NULLIF(a.extra->>'codex_5h_window_minutes', '')::int, 300) AS five_minutes,
			NULLIF(a.extra->>'codex_7d_used_percent', '')::double precision AS seven_used,
			NULLIF(a.extra->>'codex_7d_reset_at', '')::timestamptz AS seven_reset,
			COALESCE(NULLIF(a.extra->>'codex_7d_window_minutes', '')::int, 10080) AS seven_minutes
		FROM accounts a
		WHERE a.deleted_at IS NULL
		  AND a.platform = 'openai'
		  AND a.type IN ('oauth', 'setup-token')
	), usage_by_group AS (
		SELECT
			t.id AS account_id,
			COALESCE(ul.group_id, 0) AS group_id,
			COALESCE(SUM(%[1]s) FILTER (
				WHERE t.five_reset IS NOT NULL
				  AND t.five_reset > $1::timestamptz
				  AND ul.created_at >= t.five_reset - make_interval(mins => t.five_minutes)
				  AND ul.created_at < $1::timestamptz
			), 0)::double precision AS spend_5h,
			COALESCE(SUM(%[1]s) FILTER (
				WHERE t.seven_reset IS NOT NULL
				  AND t.seven_reset > $1::timestamptz
				  AND ul.created_at >= t.seven_reset - make_interval(mins => t.seven_minutes)
				  AND ul.created_at < $1::timestamptz
			), 0)::double precision AS spend_7d,
			COALESCE(SUM(%[1]s) FILTER (
				WHERE ul.created_at >= $1::timestamptz - INTERVAL '3 hours' AND ul.created_at < $1::timestamptz
			), 0)::double precision AS recent_3h,
			COALESCE(SUM(%[1]s) FILTER (
				WHERE ul.created_at >= $1::timestamptz - INTERVAL '27 hours'
				  AND ul.created_at < $1::timestamptz - INTERVAL '24 hours'
			), 0)::double precision AS previous_day_3h
		FROM target t
		JOIN usage_logs ul
		  ON ul.account_id = t.id
		 AND ul.created_at >= $1::timestamptz - INTERVAL '7 days'
		 AND ul.created_at < $1::timestamptz
		GROUP BY t.id, COALESCE(ul.group_id, 0)
	), usage_by_account AS (
		SELECT
			account_id,
			SUM(spend_5h)::double precision AS spend_5h,
			SUM(spend_7d)::double precision AS spend_7d
		FROM usage_by_group
		GROUP BY account_id
	), scope_groups AS (
		SELECT a.id AS account_id, COALESCE(ag.group_id, 0) AS group_id
		FROM target a
		LEFT JOIN account_groups ag ON ag.account_id = a.id
		UNION
		SELECT account_id, group_id
		FROM usage_by_group
	)
	SELECT
		t.id,
		sg.group_id,
		COALESCE(g.name, CASE WHEN sg.group_id = 0 THEN 'Ungrouped' ELSE 'Deleted group #' || sg.group_id::text END),
		t.plan_type,
		t.status,
		t.schedulable,
		t.rate_limit_reset_at,
		t.usage_updated_at,
		t.five_used,
		t.five_reset,
		t.five_minutes,
		t.seven_used,
		t.seven_reset,
		t.seven_minutes,
		COALESCE(ua.spend_5h, 0),
		COALESCE(ug.spend_5h, 0),
		COALESCE(ua.spend_7d, 0),
		COALESCE(ug.spend_7d, 0),
		COALESCE(ug.recent_3h, 0),
		COALESCE(ug.previous_day_3h, 0)
	FROM target t
	JOIN scope_groups sg ON sg.account_id = t.id
	LEFT JOIN groups g ON g.id = sg.group_id
	LEFT JOIN usage_by_group ug ON ug.account_id = t.id AND ug.group_id = sg.group_id
	LEFT JOIN usage_by_account ua ON ua.account_id = t.id
	ORDER BY t.id, sg.group_id
`, openAIOAuthCapacityCostSQL)
