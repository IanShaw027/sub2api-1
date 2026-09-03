package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreationKeyUniquenessMigration(t *testing.T) {
	content, err := FS.ReadFile("241_creation_key_uniqueness.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	dedupe := strings.Index(sql, "UPDATE api_keys SET deleted_at = NOW(), updated_at = NOW()")
	uniqueIndex := strings.Index(sql, "CREATE UNIQUE INDEX IF NOT EXISTS api_keys_creation_user_group_unique_idx")
	require.GreaterOrEqual(t, dedupe, 0)
	require.Greater(t, uniqueIndex, dedupe)
	require.Contains(t, sql, "PARTITION BY user_id, group_id")
	require.Contains(t, sql, "WHERE deleted_at IS NULL AND purpose = 'creation'")
}

func TestCreationImageJobTaskUniquenessMigration(t *testing.T) {
	content, err := FS.ReadFile("242_creation_image_job_task_unique.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS creation_image_jobs_user_provider_task_unique_idx")
	require.Contains(t, sql, "ON creation_image_jobs (user_id, provider_task_id)")
}
