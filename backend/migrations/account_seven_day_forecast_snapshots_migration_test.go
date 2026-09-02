package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountSevenDayForecastSnapshotsMigration(t *testing.T) {
	content, err := FS.ReadFile("238_account_seven_day_forecast_snapshots.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS account_seven_day_forecast_snapshots")
	require.Contains(t, sql, "REFERENCES accounts(id) ON DELETE CASCADE")
	require.Contains(t, sql, "UNIQUE (account_id, window_start, bucket)")
	require.Contains(t, sql, "bucket BETWEEN 10 AND 100 AND bucket % 10 = 0")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_account_7d_forecast_latest")
}
