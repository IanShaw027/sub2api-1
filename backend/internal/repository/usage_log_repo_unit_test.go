//go:build unit

package repository

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSafeDateFormat(t *testing.T) {
	tests := []struct {
		name        string
		granularity string
		expected    string
	}{
		// 合法值
		{"hour", "hour", "YYYY-MM-DD HH24:00"},
		{"day", "day", "YYYY-MM-DD"},
		{"week", "week", "IYYY-IW"},
		{"month", "month", "YYYY-MM"},

		// 非法值回退到默认
		{"空字符串", "", "YYYY-MM-DD"},
		{"未知粒度 year", "year", "YYYY-MM-DD"},
		{"未知粒度 minute", "minute", "YYYY-MM-DD"},

		// 恶意字符串
		{"SQL 注入尝试", "'; DROP TABLE users; --", "YYYY-MM-DD"},
		{"带引号", "day'", "YYYY-MM-DD"},
		{"带括号", "day)", "YYYY-MM-DD"},
		{"Unicode", "日", "YYYY-MM-DD"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := safeDateFormat(tc.granularity)
			require.Equal(t, tc.expected, got, "safeDateFormat(%q)", tc.granularity)
		})
	}
}

func TestBuildUsageLogBatchInsertQuery_UsesConflictDoNothing(t *testing.T) {
	log := &service.UsageLog{
		UserID:       1,
		APIKeyID:     2,
		AccountID:    3,
		RequestID:    "req-batch-no-update",
		Model:        "gpt-5",
		InputTokens:  10,
		OutputTokens: 5,
		TotalCost:    1.2,
		ActualCost:   1.2,
		CreatedAt:    time.Now().UTC(),
	}
	prepared := prepareUsageLogInsert(log)

	query, _ := buildUsageLogBatchInsertQuery([]string{usageLogBatchKey(log.RequestID, log.APIKeyID)}, map[string]usageLogInsertPrepared{
		usageLogBatchKey(log.RequestID, log.APIKeyID): prepared,
	})

	require.Contains(t, query, "ON CONFLICT (request_id, api_key_id) DO NOTHING")
	require.NotContains(t, strings.ToUpper(query), "DO UPDATE")
}

func TestBuildUsageLogInsertQueries_DoNotDuplicateTargetColumns(t *testing.T) {
	log := &service.UsageLog{
		UserID:       1,
		APIKeyID:     2,
		AccountID:    3,
		RequestID:    "req-no-duplicate-insert-columns",
		Model:        "gpt-5.6-sol",
		InputTokens:  10,
		OutputTokens: 5,
		TotalCost:    1.2,
		ActualCost:   1.2,
		CreatedAt:    time.Now().UTC(),
	}
	prepared := prepareUsageLogInsert(log)

	batchQuery, _ := buildUsageLogBatchInsertQuery([]string{usageLogBatchKey(log.RequestID, log.APIKeyID)}, map[string]usageLogInsertPrepared{
		usageLogBatchKey(log.RequestID, log.APIKeyID): prepared,
	})
	assertNoDuplicateUsageLogInsertColumns(t, batchQuery)

	bestEffortQuery, _ := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	assertNoDuplicateUsageLogInsertColumns(t, bestEffortQuery)
}

func TestPrepareUsageLogInsert_PreservesVideoBillingMetadata(t *testing.T) {
	resolution := "720p"
	videoSeconds := 10
	videoDurationSeconds := 12
	videoUnitPrice := 0.02
	prepared := prepareUsageLogInsert(&service.UsageLog{
		UserID:               1,
		APIKeyID:             2,
		AccountID:            3,
		RequestID:            "req-video-metadata",
		Model:                "gpt-5.6-sol",
		RequestedModel:       "gpt-5.6-sol",
		VideoCount:           2,
		VideoResolution:      &resolution,
		VideoSeconds:         &videoSeconds,
		VideoDurationSeconds: &videoDurationSeconds,
		VideoUnitPrice:       &videoUnitPrice,
		CreatedAt:            time.Now().UTC(),
	})

	require.Equal(t, 2, prepared.args[40])
	require.Equal(t, sql.NullString{String: resolution, Valid: true}, prepared.args[41])
	require.Equal(t, sql.NullInt64{Int64: int64(videoDurationSeconds), Valid: true}, prepared.args[42])
	require.Equal(t, sql.NullInt64{Int64: int64(videoSeconds), Valid: true}, prepared.args[53])
	require.Equal(t, sql.NullFloat64{Float64: videoUnitPrice, Valid: true}, prepared.args[54])
}

func assertNoDuplicateUsageLogInsertColumns(t *testing.T, query string) {
	t.Helper()

	const marker = "INSERT INTO usage_logs ("
	start := strings.Index(query, marker)
	require.NotEqual(t, -1, start, "query should contain usage_logs INSERT")
	remainder := query[start+len(marker):]
	end := strings.Index(remainder, ")")
	require.NotEqual(t, -1, end, "query should close usage_logs INSERT column list")
	columnBlock := remainder[:end]

	seen := map[string]struct{}{}
	for _, raw := range strings.Split(columnBlock, ",") {
		column := strings.TrimSpace(raw)
		require.NotEmpty(t, column)
		if _, ok := seen[column]; ok {
			t.Fatalf("duplicate usage_logs INSERT column %q in query:\n%s", column, query)
		}
		seen[column] = struct{}{}
	}
}
