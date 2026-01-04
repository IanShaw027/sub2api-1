//go:build integration

package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// End-to-end integration:
// raw logs -> aggregation UPSERTs -> ops dashboard queries (preaggregated).
func TestOpsService_EndToEnd_RawToAggregatesToDashboard(t *testing.T) {
	db := opsITDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fixture := seedOpsFixture(t, ctx, db, time.Now().UTC().Add(-3*time.Hour).Truncate(time.Hour))

	opsRepo := newOpsRepoWithPreagg(t, db)
	require.NoError(t, opsRepo.UpsertHourlyMetrics(ctx, fixture.start, fixture.end))
	require.NoError(t, opsRepo.UpsertDailyMetrics(ctx, fixture.start, fixture.end))

	opsSvc := service.NewOpsService(opsRepo, db, nil, nil)

	overview, err := opsSvc.GetDashboardOverview(ctx, "24h")
	require.NoError(t, err)
	require.NotNil(t, overview)
	require.Equal(t, fixture.wantTotalErrors, overview.Errors.TotalCount)
	require.Equal(t, fixture.want5xxErrors, overview.Errors.Count5xx)
	require.Equal(t, fixture.want4xxErrors, overview.Errors.Count4xx)

	providers, err := opsSvc.GetProviderHealth(ctx, "24h")
	require.NoError(t, err)
	require.NotEmpty(t, providers)

	var found bool
	for _, p := range providers {
		if p != nil && p.Name == "OpenAI" {
			found = true
			require.Equal(t, fixture.wantOpenAIRequests, p.RequestCount)
			break
		}
	}
	require.True(t, found, "expected OpenAI provider stats to be present")
}

type opsFixture struct {
	start time.Time
	end   time.Time

	wantTotalErrors    int64
	want4xxErrors      int64
	want5xxErrors      int64
	wantOpenAIRequests int64
}

func seedOpsFixture(t *testing.T, ctx context.Context, db *sql.DB, hour0 time.Time) opsFixture {
	t.Helper()

	userID := insertUser(t, ctx, db)
	openAIGroup := insertGroup(t, ctx, db, "openai")
	anthropicGroup := insertGroup(t, ctx, db, "anthropic")
	openAIAccount := insertAccount(t, ctx, db, "openai")
	anthropicAccount := insertAccount(t, ctx, db, "anthropic")
	openAIKey := insertAPIKey(t, ctx, db, userID, openAIGroup)
	anthropicKey := insertAPIKey(t, ctx, db, userID, anthropicGroup)

	// Two full hour buckets: [hour0, hour0+2h).
	start := hour0
	end := hour0.Add(2 * time.Hour)

	// Usage (success) logs.
	insertUsageN(t, ctx, db, userID, openAIKey, openAIAccount, openAIGroup, 100, 10, hour0.Add(10*time.Minute))
	insertUsageN(t, ctx, db, userID, openAIKey, openAIAccount, openAIGroup, 100, 20, hour0.Add(70*time.Minute))
	insertUsageN(t, ctx, db, userID, anthropicKey, anthropicAccount, anthropicGroup, 200, 5, hour0.Add(20*time.Minute))
	insertUsageN(t, ctx, db, userID, anthropicKey, anthropicAccount, anthropicGroup, 200, 5, hour0.Add(80*time.Minute))

	// Error logs.
	insertError(t, ctx, db, openAIAccount, openAIGroup, "timeout", 504, hour0.Add(30*time.Minute), "")
	insertError(t, ctx, db, openAIAccount, openAIGroup, "upstream_error", 500, hour0.Add(40*time.Minute), "")
	insertError(t, ctx, db, openAIAccount, openAIGroup, "rate_limit", 429, hour0.Add(90*time.Minute), "")
	insertError(t, ctx, db, anthropicAccount, anthropicGroup, "bad_request", 404, hour0.Add(15*time.Minute), "")
	insertError(t, ctx, db, anthropicAccount, anthropicGroup, "upstream_error", 500, hour0.Add(95*time.Minute), "")

	return opsFixture{
		start:              start,
		end:                end,
		wantTotalErrors:    5,
		want4xxErrors:      2, // 429 + 404
		want5xxErrors:      3, // 500 + 504 + 500
		wantOpenAIRequests: 33,
	}
}

func newOpsRepoWithPreagg(t *testing.T, db *sql.DB) *repository.OpsRepository {
	t.Helper()

	repoIface := repository.NewOpsRepository(nil, db, nil, &config.Config{
		Ops: config.OpsConfig{UsePreaggregatedTables: true},
	})

	opsRepo, ok := repoIface.(*repository.OpsRepository)
	require.True(t, ok, "expected repository.NewOpsRepository to return *repository.OpsRepository")
	return opsRepo
}

func insertUser(t *testing.T, ctx context.Context, db *sql.DB) int64 {
	t.Helper()

	var id int64
	require.NoError(t, db.QueryRowContext(
		ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3) RETURNING id`,
		fmt.Sprintf("it_%d@example.com", time.Now().UnixNano()),
		"hash",
		"user",
	).Scan(&id))
	return id
}

func insertGroup(t *testing.T, ctx context.Context, db *sql.DB, platform string) int64 {
	t.Helper()

	var id int64
	require.NoError(t, db.QueryRowContext(
		ctx,
		`INSERT INTO groups (name, description, platform) VALUES ($1, $2, $3) RETURNING id`,
		fmt.Sprintf("it_%s_%d", platform, time.Now().UnixNano()),
		"it",
		platform,
	).Scan(&id))
	return id
}

func insertAccount(t *testing.T, ctx context.Context, db *sql.DB, platform string) int64 {
	t.Helper()

	var id int64
	require.NoError(t, db.QueryRowContext(
		ctx,
		`INSERT INTO accounts (name, platform, type, credentials, extra, status) VALUES ($1, $2, $3, '{}'::jsonb, '{}'::jsonb, 'active') RETURNING id`,
		fmt.Sprintf("it_%s", platform),
		platform,
		"apikey",
	).Scan(&id))
	return id
}

func insertAPIKey(t *testing.T, ctx context.Context, db *sql.DB, userID, groupID int64) int64 {
	t.Helper()

	var id int64
	require.NoError(t, db.QueryRowContext(
		ctx,
		`INSERT INTO api_keys (user_id, key, name, group_id, status) VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		userID,
		fmt.Sprintf("sk-it-%d", time.Now().UnixNano()),
		"it",
		groupID,
	).Scan(&id))
	return id
}

func insertUsageN(t *testing.T, ctx context.Context, db *sql.DB, userID, apiKeyID, accountID, groupID int64, durationMs, n int, createdAt time.Time) {
	t.Helper()

	for i := 0; i < n; i++ {
		_, err := db.ExecContext(
			ctx,
			`INSERT INTO usage_logs (user_id, api_key_id, account_id, group_id, model, duration_ms, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			userID,
			apiKeyID,
			accountID,
			groupID,
			"gpt-4o-mini",
			durationMs,
			createdAt,
		)
		require.NoError(t, err)
	}
}

func insertError(t *testing.T, ctx context.Context, db *sql.DB, accountID, groupID int64, errType string, statusCode int, createdAt time.Time, platform string) {
	t.Helper()

	_, err := db.ExecContext(
		ctx,
		`INSERT INTO ops_error_logs (
			account_id, group_id, error_phase, error_type, severity, status_code, platform, error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		accountID,
		groupID,
		"upstream",
		errType,
		"P2",
		statusCode,
		platform,
		"error",
		createdAt,
	)
	require.NoError(t, err)
}

func opsITDB(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	if !opsDockerAvailable(ctx) {
		t.Skip("docker is not available; skipping ops integration tests")
	}

	pgContainer, err := tcpostgres.Run(
		ctx,
		"postgres:18.1-alpine3.23",
		tcpostgres.WithDatabase("sub2api_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pgContainer.Terminate(context.Background())
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, db.PingContext(pingCtx))

	schema := fmt.Sprintf("it_ops_%d", time.Now().UnixNano())
	_, err = db.ExecContext(context.Background(), `CREATE SCHEMA `+pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+pq.QuoteIdentifier(schema)+` CASCADE`)
	})

	_, err = db.ExecContext(context.Background(), `SET search_path TO `+pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `SET TIME ZONE 'UTC'`)
	require.NoError(t, err)

	migCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	require.NoError(t, repository.ApplyMigrations(migCtx, db))

	return db
}

func opsDockerAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "info")
	return cmd.Run() == nil
}
