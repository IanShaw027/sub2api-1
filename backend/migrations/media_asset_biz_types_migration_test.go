package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandMediaAssetBizTypesMigrationUsesValidatedReplacementConstraint(t *testing.T) {
	sqlBytes, err := os.ReadFile("237_expand_media_asset_biz_types.sql")
	require.NoError(t, err)
	sql := string(sqlBytes)

	for _, bizType := range []string{
		"invoice",
		"ticket",
		"avatar",
		"site_logo",
		"support_qr",
		"announcement",
		"payment_help",
		"image_task",
	} {
		require.Contains(t, sql, "'"+bizType+"'")
	}
	addAt := strings.Index(sql, "ADD CONSTRAINT media_assets_biz_type_check_v2")
	validateAt := strings.Index(sql, "VALIDATE CONSTRAINT media_assets_biz_type_check_v2")
	dropAt := strings.Index(sql, "DROP CONSTRAINT IF EXISTS media_assets_biz_type_check")
	renameAt := strings.Index(sql, "RENAME CONSTRAINT media_assets_biz_type_check_v2")
	require.GreaterOrEqual(t, addAt, 0)
	require.Contains(t, sql[addAt:], "NOT VALID")
	require.Greater(t, validateAt, addAt)
	require.Greater(t, dropAt, validateAt)
	require.Greater(t, renameAt, dropAt)
}
