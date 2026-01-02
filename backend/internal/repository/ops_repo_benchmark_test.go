package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	svc "github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// Benchmark notes:
// - Requires a Postgres DSN via TEST_DATABASE_URL.
// - Creates an isolated schema per benchmark run and applies migrations.
// - Populates raw ops logs, then populates ops_metrics_hourly/daily.
// - Compares legacy raw-log queries vs pre-aggregated queries.
//
// Run (example):
//
//	TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable" go test ./backend/internal/repository -run '^$' -bench 'BenchmarkOpsRepo' -benchmem
func BenchmarkOpsRepo_PreaggregatedQueries(b *testing.B) {
	db := benchOpenPostgres(b)
	defer func() { _ = db.Close() }()

	for _, rows := range []int{10_000, 100_000, 1_000_000} {
		rows := rows
		if rows == 1_000_000 && os.Getenv("OPS_BENCH_ENABLE_1M") == "" {
			continue
		}

		b.Run(fmt.Sprintf("rows=%d", rows), func(b *testing.B) {
			ctx := context.Background()

			schema := fmt.Sprintf("bench_ops_%d", time.Now().UnixNano())
			benchCreateSchema(b, db, schema)
			b.Cleanup(func() { benchDropSchema(b, db, schema) })

			benchSetSearchPath(b, db, schema)
			benchApplyMigrations(b, db)

			start, end := benchSeedOpsDataset(b, db, rows)

			repoLegacy := &OpsRepository{sql: db, usePreaggregatedTables: false}
			repoPreagg := &OpsRepository{sql: db, usePreaggregatedTables: true}

			opsSvcLegacy := svc.NewOpsService(repoLegacy, db)
			opsSvcPreagg := svc.NewOpsService(repoPreagg, db)

			// Ensure aggregates exist for the full range.
			b.StopTimer()
			requireNoErr(b, repoPreagg.UpsertHourlyMetrics(ctx, start, end), "UpsertHourlyMetrics")
			requireNoErr(b, repoPreagg.UpsertDailyMetrics(ctx, start, end), "UpsertDailyMetrics")
			b.StartTimer()

			// Service-level benchmarks (what the API endpoints actually call).
			b.Run("GetDashboardOverview", func(b *testing.B) {
				b.Run("legacy", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_, err := opsSvcLegacy.GetDashboardOverview(ctx, "168h")
						requireNoErr(b, err, "GetDashboardOverview(legacy)")
					}
				})
				b.Run("preaggregated", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_, err := opsSvcPreagg.GetDashboardOverview(ctx, "168h")
						requireNoErr(b, err, "GetDashboardOverview(preagg)")
					}
				})
			})

			b.Run("GetProviderHealth", func(b *testing.B) {
				b.Run("legacy", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_, err := opsSvcLegacy.GetProviderHealth(ctx, "168h")
						requireNoErr(b, err, "GetProviderHealth(legacy)")
					}
				})
				b.Run("preaggregated", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_, err := opsSvcPreagg.GetProviderHealth(ctx, "168h")
						requireNoErr(b, err, "GetProviderHealth(preagg)")
					}
				})
			})

			b.Run("GetOverviewStats", func(b *testing.B) {
				b.Run("legacy", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_, err := repoLegacy.GetOverviewStatsLegacy(ctx, start, end)
						requireNoErr(b, err, "GetOverviewStatsLegacy")
					}
				})
				b.Run("preaggregated", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_, err := repoPreagg.GetOverviewStats(ctx, start, end)
						requireNoErr(b, err, "GetOverviewStats(preagg)")
					}
				})
			})

			b.Run("GetWindowStats", func(b *testing.B) {
				b.Run("legacy", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_, err := repoLegacy.GetWindowStatsLegacy(ctx, start, end)
						requireNoErr(b, err, "GetWindowStatsLegacy")
					}
				})
				b.Run("preaggregated", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_, err := repoPreagg.GetWindowStats(ctx, start, end)
						requireNoErr(b, err, "GetWindowStats(preagg)")
					}
				})
			})

			b.Run("GetProviderStats", func(b *testing.B) {
				b.Run("legacy", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_, err := repoLegacy.GetProviderStatsLegacy(ctx, start, end)
						requireNoErr(b, err, "GetProviderStatsLegacy")
					}
				})
				b.Run("preaggregated", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						_, err := repoPreagg.GetProviderStats(ctx, start, end)
						requireNoErr(b, err, "GetProviderStats(preagg)")
					}
				})
			})
		})
	}
}

func benchOpenPostgres(b *testing.B) *sql.DB {
	b.Helper()

	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		b.Skip("未设置 TEST_DATABASE_URL，跳过 OpsRepository 基准测试")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		b.Fatalf("open postgres: %v", err)
	}

	// Keep a single session so SET search_path stays effective.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		b.Fatalf("ping postgres: %v", err)
	}

	return db
}

func benchCreateSchema(b *testing.B, db *sql.DB, schema string) {
	b.Helper()
	_, err := db.ExecContext(context.Background(), `CREATE SCHEMA IF NOT EXISTS `+pq.QuoteIdentifier(schema))
	requireNoErr(b, err, "create schema")
}

func benchDropSchema(b *testing.B, db *sql.DB, schema string) {
	b.Helper()
	_, _ = db.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+pq.QuoteIdentifier(schema)+` CASCADE`)
}

func benchSetSearchPath(b *testing.B, db *sql.DB, schema string) {
	b.Helper()
	_, err := db.ExecContext(context.Background(), `SET search_path TO `+pq.QuoteIdentifier(schema))
	requireNoErr(b, err, "set search_path")
}

func benchApplyMigrations(b *testing.B, db *sql.DB) {
	b.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	requireNoErr(b, ApplyMigrations(ctx, db), "ApplyMigrations")
}

func benchSeedOpsDataset(b *testing.B, db *sql.DB, totalRows int) (time.Time, time.Time) {
	b.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	userID, apiKeyByPlatform, groupByPlatform, accountByPlatform := benchInsertBaseEntities(b, ctx, db)

	// Seed in a window that is safely outside the "freshness lag" used by pre-aggregated reads.
	end := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Hour)
	start := end.Add(-8 * 24 * time.Hour).Truncate(time.Hour)
	window := end.Sub(start)

	// Use constant token counts (cheap) and varied durations (more realistic percentiles).
	rng := rand.New(rand.NewSource(1))

	// Split total rows between usage logs and error logs.
	usageRows := int(float64(totalRows) * 0.97)
	errorRows := totalRows - usageRows

	platforms := []string{"openai", "anthropic", "gemini"}
	platformWeights := []int{70, 20, 10}
	pickPlatform := func() string {
		x := rng.Intn(100)
		acc := 0
		for i, w := range platformWeights {
			acc += w
			if x < acc {
				return platforms[i]
			}
		}
		return platforms[0]
	}

	tx, err := db.BeginTx(ctx, nil)
	requireNoErr(b, err, "begin seed tx")
	defer func() { _ = tx.Rollback() }()

	usageStmt, err := tx.PrepareContext(ctx, pq.CopyIn(
		"usage_logs",
		"user_id",
		"api_key_id",
		"account_id",
		"group_id",
		"model",
		"input_tokens",
		"output_tokens",
		"cache_creation_tokens",
		"cache_read_tokens",
		"duration_ms",
		"created_at",
	))
	requireNoErr(b, err, "prepare COPY usage_logs")

	for i := 0; i < usageRows; i++ {
		platform := pickPlatform()
		groupID := groupByPlatform[platform]
		accountID := accountByPlatform[platform]
		apiKeyID := apiKeyByPlatform[platform]
		createdAt := start.Add(time.Duration(rng.Int63n(int64(window))))

		// Roughly log-normal-ish: base + spikes.
		base := 40 + rng.Intn(260) // 40..300
		spike := 0
		if rng.Intn(100) < 2 { // ~2% slow tail
			spike = 800 + rng.Intn(2000) // 800..2800
		}
		durationMs := base + spike

		_, err := usageStmt.ExecContext(
			ctx,
			userID,
			apiKeyID,
			accountID,
			groupID,
			"gpt-4o-mini",
			100,
			200,
			0,
			0,
			durationMs,
			createdAt,
		)
		requireNoErr(b, err, "COPY usage_logs row")
	}
	requireNoErr(b, usageStmt.Close(), "close COPY usage_logs")

	errorStmt, err := tx.PrepareContext(ctx, pq.CopyIn(
		"ops_error_logs",
		"account_id",
		"group_id",
		"error_phase",
		"error_type",
		"severity",
		"status_code",
		"platform",
		"error_message",
		"duration_ms",
		"created_at",
	))
	requireNoErr(b, err, "prepare COPY ops_error_logs")

	for i := 0; i < errorRows; i++ {
		platform := pickPlatform()
		groupID := groupByPlatform[platform]
		accountID := accountByPlatform[platform]
		createdAt := start.Add(time.Duration(rng.Int63n(int64(window))))

		// Mix 4xx / 5xx / timeouts.
		statusCode := 500
		errType := "upstream_error"
		msg := "upstream error"
		if rng.Intn(100) < 35 {
			statusCode = 429
			errType = "rate_limit"
			msg = "rate limited"
		}
		if rng.Intn(100) < 10 {
			statusCode = 504
			errType = "timeout"
			msg = "deadline exceeded"
		}

		_, err := errorStmt.ExecContext(
			ctx,
			accountID,
			groupID,
			"upstream",
			errType,
			"P2",
			statusCode,
			"", // prefer join-derived platform
			msg,
			0,
			createdAt,
		)
		requireNoErr(b, err, "COPY ops_error_logs row")
	}
	requireNoErr(b, errorStmt.Close(), "close COPY ops_error_logs")

	requireNoErr(b, tx.Commit(), "commit seed tx")

	return start, end
}

func benchInsertBaseEntities(b *testing.B, ctx context.Context, db *sql.DB) (userID int64, apiKeyByPlatform, groupByPlatform, accountByPlatform map[string]int64) {
	b.Helper()

	apiKeyByPlatform = map[string]int64{}
	groupByPlatform = map[string]int64{}
	accountByPlatform = map[string]int64{}

	requireNoErr(b, db.QueryRowContext(
		ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3) RETURNING id`,
		fmt.Sprintf("bench_%d@example.com", time.Now().UnixNano()),
		"hash",
		"user",
	).Scan(&userID), "insert user")

	platforms := []string{"openai", "anthropic", "gemini"}
	for _, p := range platforms {
		var gid int64
		requireNoErr(b, db.QueryRowContext(
			ctx,
			`INSERT INTO groups (name, description, platform) VALUES ($1, $2, $3) RETURNING id`,
			"bench_"+p+"_"+strconv.FormatInt(time.Now().UnixNano(), 10),
			"bench",
			p,
		).Scan(&gid), "insert group "+p)
		groupByPlatform[p] = gid

		var aid int64
		requireNoErr(b, db.QueryRowContext(
			ctx,
			`INSERT INTO accounts (name, platform, type, credentials, extra, status) VALUES ($1, $2, $3, '{}'::jsonb, '{}'::jsonb, 'active') RETURNING id`,
			"bench_"+p,
			p,
			"apikey",
		).Scan(&aid), "insert account "+p)
		accountByPlatform[p] = aid

		var apiKeyID int64
		requireNoErr(b, db.QueryRowContext(
			ctx,
			`INSERT INTO api_keys (user_id, key, name, group_id, status) VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
			userID,
			fmt.Sprintf("sk-bench-%s-%d", p, time.Now().UnixNano()),
			"bench",
			gid,
		).Scan(&apiKeyID), "insert api_key "+p)
		apiKeyByPlatform[p] = apiKeyID
	}

	return userID, apiKeyByPlatform, groupByPlatform, accountByPlatform
}

func requireNoErr(b *testing.B, err error, what string) {
	b.Helper()
	if err != nil {
		b.Fatalf("%s: %v", what, err)
	}
}
