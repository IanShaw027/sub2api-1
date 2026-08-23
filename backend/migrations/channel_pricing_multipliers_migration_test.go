package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelPricingMultipliersMigration(t *testing.T) {
	content, err := FS.ReadFile("236_channel_pricing_multipliers.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, column := range []string{
		"fast_multiplier NUMERIC(12,6)",
		"flex_multiplier NUMERIC(12,6)",
		"input_multiplier NUMERIC(12,6)",
		"output_multiplier NUMERIC(12,6)",
		"cache_write_multiplier NUMERIC(12,6)",
		"cache_read_multiplier NUMERIC(12,6)",
	} {
		require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS "+column)
	}

	require.Equal(t, 6, strings.Count(sql, "CHECK ("))
	require.Equal(t, 6, strings.Count(sql, "IS NULL OR"))
	require.Equal(t, 6, strings.Count(sql, "> 0)"))

	_, err = FS.ReadFile("228_channel_pricing_multipliers.sql")
	require.NoError(t, err, "228_channel_pricing_multipliers.sql must remain (IF NOT EXISTS)")
}
