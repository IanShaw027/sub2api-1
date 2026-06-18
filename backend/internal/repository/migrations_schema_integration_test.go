//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate(t *testing.T) {
	tx := testTx(t)

	// Re-apply migrations to verify idempotency (no errors, no duplicate rows).
	require.NoError(t, ApplyMigrations(context.Background(), integrationDB))

	// schema_migrations should have at least the current migration set.
	var applied int
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM schema_migrations").Scan(&applied))
	require.GreaterOrEqual(t, applied, 7, "expected schema_migrations to contain applied migrations")

	// users: columns required by repository queries
	requireColumn(t, tx, "users", "username", "character varying", 100, false)
	requireColumn(t, tx, "users", "notes", "text", 0, false)

	// accounts: schedulable and rate-limit fields
	requireColumn(t, tx, "accounts", "notes", "text", 0, true)
	requireColumn(t, tx, "accounts", "schedulable", "boolean", 0, false)
	requireColumn(t, tx, "accounts", "rate_limited_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "rate_limit_reset_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "overload_until", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "session_window_status", "character varying", 20, true)

	// api_keys: key length should be 128
	requireColumn(t, tx, "api_keys", "key", "character varying", 128, false)

	// redeem_codes: subscription fields
	requireColumn(t, tx, "redeem_codes", "group_id", "bigint", 0, true)
	requireColumn(t, tx, "redeem_codes", "validity_days", "integer", 0, false)

	// groups: display_name should match the Ent schema MaxLen(100).
	requireColumn(t, tx, "groups", "display_name", "character varying", 100, true)
	requireColumn(t, tx, "groups", "user_selectable", "boolean", 0, false)

	// usage_logs: billing_type used by filters/stats
	requireColumn(t, tx, "usage_logs", "billing_type", "smallint", 0, false)
	requireColumn(t, tx, "usage_logs", "request_type", "smallint", 0, false)
	requireColumn(t, tx, "usage_logs", "openai_ws_mode", "boolean", 0, false)
	requireColumn(t, tx, "usage_logs", "openai_ws_profile", "text", 0, false)
	requireColumn(t, tx, "usage_logs", "openai_ws_conn_reused", "boolean", 0, false)
	requireIndex(t, tx, "usage_logs", "idx_usage_logs_user_created_at_covering")

	// usage_billing_dedup: billing idempotency narrow table
	var usageBillingDedupRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.usage_billing_dedup')").Scan(&usageBillingDedupRegclass))
	require.True(t, usageBillingDedupRegclass.Valid, "expected usage_billing_dedup table to exist")
	requireColumn(t, tx, "usage_billing_dedup", "request_fingerprint", "character varying", 64, false)
	requireIndex(t, tx, "usage_billing_dedup", "idx_usage_billing_dedup_request_api_key")
	requireIndex(t, tx, "usage_billing_dedup", "idx_usage_billing_dedup_created_at_brin")

	var usageBillingDedupArchiveRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.usage_billing_dedup_archive')").Scan(&usageBillingDedupArchiveRegclass))
	require.True(t, usageBillingDedupArchiveRegclass.Valid, "expected usage_billing_dedup_archive table to exist")
	requireColumn(t, tx, "usage_billing_dedup_archive", "request_fingerprint", "character varying", 64, false)
	requireIndex(t, tx, "usage_billing_dedup_archive", "usage_billing_dedup_archive_pkey")

	// settings table should exist
	var settingsRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.settings')").Scan(&settingsRegclass))
	require.True(t, settingsRegclass.Valid, "expected settings table to exist")

	// security_secrets table should exist
	var securitySecretsRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.security_secrets')").Scan(&securitySecretsRegclass))
	require.True(t, securitySecretsRegclass.Valid, "expected security_secrets table to exist")

	// user_allowed_groups table should exist
	var uagRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.user_allowed_groups')").Scan(&uagRegclass))
	require.True(t, uagRegclass.Valid, "expected user_allowed_groups table to exist")

	// user_subscriptions: deleted_at for soft delete support (migration 012)
	requireColumn(t, tx, "user_subscriptions", "deleted_at", "timestamp with time zone", 0, true)

	// orphan_allowed_groups_audit table should exist (migration 013)
	var orphanAuditRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.orphan_allowed_groups_audit')").Scan(&orphanAuditRegclass))
	require.True(t, orphanAuditRegclass.Valid, "expected orphan_allowed_groups_audit table to exist")

	// account_groups: created_at should be timestamptz
	requireColumn(t, tx, "account_groups", "created_at", "timestamp with time zone", 0, false)

	// user_allowed_groups: created_at should be timestamptz
	requireColumn(t, tx, "user_allowed_groups", "created_at", "timestamp with time zone", 0, false)
}

func TestMigrationsRunner_AuthIdentityAndPaymentSchemaStayAligned(t *testing.T) {
	tx := testTx(t)

	requireColumn(t, tx, "auth_identity_migration_reports", "report_type", "character varying", 80, false)
	requireColumn(t, tx, "users", "signup_source", "character varying", 20, false)
	requireColumnDefaultContains(t, tx, "users", "signup_source", "email")
	requireConstraintDefinitionContains(
		t,
		tx,
		"users",
		"users_signup_source_check",
		"signup_source",
		"'email'",
		"'linuxdo'",
		"'wechat'",
		"'oidc'",
		"'github'",
		"'google'",
	)
	requireConstraintDefinitionContains(
		t,
		tx,
		"auth_identities",
		"auth_identities_provider_type_check",
		"provider_type",
		"'email'",
		"'linuxdo'",
		"'wechat'",
		"'oidc'",
		"'github'",
		"'google'",
	)
	requireConstraintDefinitionContains(
		t,
		tx,
		"auth_identity_channels",
		"auth_identity_channels_provider_type_check",
		"provider_type",
		"'email'",
		"'linuxdo'",
		"'wechat'",
		"'oidc'",
		"'github'",
		"'google'",
	)
	requireConstraintDefinitionContains(
		t,
		tx,
		"pending_auth_sessions",
		"pending_auth_sessions_provider_type_check",
		"provider_type",
		"'email'",
		"'linuxdo'",
		"'wechat'",
		"'oidc'",
		"'github'",
		"'google'",
	)

	requireForeignKeyOnDelete(t, tx, "auth_identities", "user_id", "users", "CASCADE")
	requireForeignKeyOnDelete(t, tx, "auth_identity_channels", "identity_id", "auth_identities", "CASCADE")
	requireForeignKeyOnDelete(t, tx, "pending_auth_sessions", "target_user_id", "users", "SET NULL")
	requireForeignKeyOnDelete(t, tx, "identity_adoption_decisions", "pending_auth_session_id", "pending_auth_sessions", "CASCADE")
	requireForeignKeyOnDelete(t, tx, "identity_adoption_decisions", "identity_id", "auth_identities", "SET NULL")
	requireForeignKeyOnDelete(t, tx, "user_platform_quotas", "user_id", "users", "CASCADE")

	requireIndex(t, tx, "payment_orders", "paymentorder_out_trade_no")
	requirePartialUniqueIndexDefinition(t, tx, "payment_orders", "paymentorder_out_trade_no", "out_trade_no", "WHERE")
	requireIndexAbsent(t, tx, "payment_orders", "paymentorder_out_trade_no_unique")

	var contentModerationLogsRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.content_moderation_logs')").Scan(&contentModerationLogsRegclass))
	require.True(t, contentModerationLogsRegclass.Valid, "expected content_moderation_logs table to exist")
	requireColumn(t, tx, "content_moderation_logs", "request_id", "character varying", 128, false)
	requireColumn(t, tx, "content_moderation_logs", "highest_score", "numeric", 0, false)
	requireColumn(t, tx, "content_moderation_logs", "category_scores", "jsonb", 0, false)
	requireColumn(t, tx, "content_moderation_logs", "created_at", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "content_moderation_logs", "idx_content_moderation_logs_created_at")

	var riskControlEnabled string
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT value FROM settings WHERE key = 'risk_control_enabled'").Scan(&riskControlEnabled))
	require.Equal(t, "false", riskControlEnabled)
}

func TestMigrationsRunner_ChannelMonitorRequestTemplateSchemaStayAligned(t *testing.T) {
	tx := testTx(t)

	requireIndex(t, tx, "channel_monitors", "channelmonitor_enabled_last_checked_at")
	requireIndex(t, tx, "channel_monitors", "channelmonitor_provider")
	requireIndex(t, tx, "channel_monitors", "channelmonitor_group_name")
	requireIndexAbsent(t, tx, "channel_monitors", "idx_channel_monitors_enabled_last_checked")
	requireIndexAbsent(t, tx, "channel_monitors", "idx_channel_monitors_provider")
	requireIndexAbsent(t, tx, "channel_monitors", "idx_channel_monitors_group_name")
	requireIndex(t, tx, "channel_monitor_request_templates", "channelmonitorrequesttemplate_provider_name")
	requireIndexAbsent(t, tx, "channel_monitor_request_templates", "channel_monitor_request_templates_provider_name")
	requireIndex(t, tx, "channel_monitors", "channelmonitor_template_id")
	requireIndexAbsent(t, tx, "channel_monitors", "idx_channel_monitors_template_id")
	requireNonPartialIndexDefinition(t, tx, "channel_monitors", "channelmonitor_template_id", "template_id")
	requireConstraint(t, tx, "channel_monitors", "channel_monitors_channel_monitor_request_templates_request_template")
	requireForeignKeyOnDelete(t, tx, "channel_monitors", "template_id", "channel_monitor_request_templates", "SET NULL")
	requireIndex(t, tx, "channel_monitor_histories", "channelmonitorhistory_monitor_id_model_checked_at")
	requireIndex(t, tx, "channel_monitor_histories", "channelmonitorhistory_checked_at")
	requireIndexAbsent(t, tx, "channel_monitor_histories", "idx_channel_monitor_histories_monitor_model_checked")
	requireIndexAbsent(t, tx, "channel_monitor_histories", "idx_channel_monitor_histories_checked_at_id")
	requireIndex(t, tx, "channel_monitor_daily_rollups", "channelmonitordailyrollup_monitor_id_model_bucket_date")
	requireIndex(t, tx, "channel_monitor_daily_rollups", "channelmonitordailyrollup_bucket_date")
	requireIndexAbsent(t, tx, "channel_monitor_daily_rollups", "idx_channel_monitor_daily_rollups_unique")
	requireIndexAbsent(t, tx, "channel_monitor_daily_rollups", "idx_channel_monitor_daily_rollups_bucket_id")
	requireColumnAbsent(t, tx, "channel_monitor_histories", "deleted_at")
	requireColumnAbsent(t, tx, "channel_monitor_daily_rollups", "deleted_at")
}

func TestMigrationsRunner_AICenterCoreSchemaStayAligned(t *testing.T) {
	tx := testTx(t)

	requireColumn(t, tx, "ai_sessions", "user_id", "bigint", 0, false)
	requireColumn(t, tx, "ai_sessions", "title", "character varying", 200, false)
	requireColumn(t, tx, "ai_sessions", "deleted_at", "timestamp with time zone", 0, true)
	requireIndex(t, tx, "ai_sessions", "aisession_user_id_updated_at")
	requireIndex(t, tx, "ai_sessions", "aisession_user_id_last_message_at")

	requireColumn(t, tx, "ai_session_messages", "session_id", "bigint", 0, false)
	requireColumn(t, tx, "ai_session_messages", "provider", "character varying", 50, true)
	requireIndex(t, tx, "ai_session_messages", "aisessionmessage_session_id_created_at")
	requireForeignKeyOnDelete(t, tx, "ai_session_messages", "session_id", "ai_sessions", "NO ACTION")

	requireColumn(t, tx, "ai_prompt_templates", "visibility", "character varying", 32, false)
	requireColumn(t, tx, "ai_prompt_templates", "moderation_state", "character varying", 32, false)
	requireColumn(t, tx, "ai_prompt_templates", "cover_asset_id", "bigint", 0, true)
	requireIndex(t, tx, "ai_prompt_templates", "aiprompttemplate_visibility_moderation_state")

	requireColumn(t, tx, "ai_prompt_template_versions", "template_id", "bigint", 0, false)
	requireColumn(t, tx, "ai_prompt_template_versions", "version", "integer", 0, false)
	requireIndex(t, tx, "ai_prompt_template_versions", "aiprompttemplateversion_template_id_version")
	requireForeignKeyOnDelete(t, tx, "ai_prompt_template_versions", "template_id", "ai_prompt_templates", "NO ACTION")

	requireColumn(t, tx, "ai_generation_jobs", "prompt_template_id", "bigint", 0, true)
	requireColumn(t, tx, "ai_generation_jobs", "session_id", "bigint", 0, true)
	requireIndex(t, tx, "ai_generation_jobs", "aigenerationjob_user_id_created_at")
	requireForeignKeyOnDelete(t, tx, "ai_generation_jobs", "prompt_template_id", "ai_prompt_templates", "SET NULL")
	requireForeignKeyOnDelete(t, tx, "ai_generation_jobs", "session_id", "ai_sessions", "SET NULL")

	requireColumn(t, tx, "ai_assets", "storage_kind", "character varying", 32, true)
	requireColumn(t, tx, "ai_assets", "storage_path", "text", 0, true)
	requireColumn(t, tx, "ai_assets", "deleted_at", "timestamp with time zone", 0, true)
	requireIndex(t, tx, "ai_assets", "aiasset_user_id_created_at")
	requireForeignKeyOnDelete(t, tx, "ai_assets", "generation_job_id", "ai_generation_jobs", "SET NULL")
	requireForeignKeyOnDelete(t, tx, "ai_assets", "prompt_template_id", "ai_prompt_templates", "SET NULL")
	requireForeignKeyOnDelete(t, tx, "ai_assets", "session_id", "ai_sessions", "SET NULL")

	requireColumn(t, tx, "ai_audit_logs", "entity_type", "character varying", 64, false)
	requireColumn(t, tx, "ai_audit_logs", "created_at", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "ai_audit_logs", "aiauditlog_entity_type_entity_id_created_at")
}

func requireConstraint(t *testing.T, tx *sql.Tx, table, constraint string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1
	FROM pg_constraint c
	JOIN pg_class tbl ON tbl.oid = c.conrelid
	JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
	WHERE ns.nspname = 'public'
	  AND tbl.relname = $1
	  AND c.conname = $2
)
`, table, constraint).Scan(&exists)
	require.NoError(t, err, "query pg_constraint for %s.%s", table, constraint)
	require.True(t, exists, "expected constraint %s on %s", constraint, table)
}

func requireIndex(t *testing.T, tx *sql.Tx, table, index string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1
	FROM pg_indexes
	WHERE schemaname = 'public'
	  AND tablename = $1
	  AND indexname = $2
)
`, table, index).Scan(&exists)
	require.NoError(t, err, "query pg_indexes for %s.%s", table, index)
	require.True(t, exists, "expected index %s on %s", index, table)
}

func requireIndexAbsent(t *testing.T, tx *sql.Tx, table, index string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1
	FROM pg_indexes
	WHERE schemaname = 'public'
	  AND tablename = $1
	  AND indexname = $2
)
`, table, index).Scan(&exists)
	require.NoError(t, err, "query pg_indexes for %s.%s", table, index)
	require.False(t, exists, "expected index %s on %s to be absent", index, table)
}

func requirePartialUniqueIndexDefinition(t *testing.T, tx *sql.Tx, table, index string, fragments ...string) {
	t.Helper()

	var (
		unique bool
		def    string
	)

	err := tx.QueryRowContext(context.Background(), `
SELECT
	i.indisunique,
	pg_get_indexdef(i.indexrelid)
FROM pg_class idx
JOIN pg_index i ON i.indexrelid = idx.oid
JOIN pg_class tbl ON tbl.oid = i.indrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
WHERE ns.nspname = 'public'
  AND tbl.relname = $1
  AND idx.relname = $2
`, table, index).Scan(&unique, &def)
	require.NoError(t, err, "query index definition for %s.%s", table, index)
	require.True(t, unique, "expected index %s on %s to be unique", index, table)

	for _, fragment := range fragments {
		require.Contains(t, def, fragment, "expected index definition for %s.%s to contain %q", table, index, fragment)
	}
}

func requireNonPartialIndexDefinition(t *testing.T, tx *sql.Tx, table, index string, fragments ...string) {
	t.Helper()

	var def string
	err := tx.QueryRowContext(context.Background(), `
SELECT pg_get_indexdef(i.indexrelid)
FROM pg_class idx
JOIN pg_index i ON i.indexrelid = idx.oid
JOIN pg_class tbl ON tbl.oid = i.indrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
WHERE ns.nspname = 'public'
  AND tbl.relname = $1
  AND idx.relname = $2
	`, table, index).Scan(&def)
	require.NoError(t, err, "query index definition for %s.%s", table, index)
	require.NotContains(t, def, " WHERE ", "expected index %s on %s to be non-partial", index, table)

	for _, fragment := range fragments {
		require.Contains(t, def, fragment, "expected index definition for %s.%s to contain %q", table, index, fragment)
	}
}

func requireColumnAbsent(t *testing.T, tx *sql.Tx, table, column string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = $1
      AND column_name = $2
)
	`, table, column).Scan(&exists)
	require.NoError(t, err, "query information_schema.columns for %s.%s", table, column)
	require.False(t, exists, "expected column %s.%s to be absent", table, column)
}

func requireForeignKeyOnDelete(t *testing.T, tx *sql.Tx, table, column, refTable, expected string) {
	t.Helper()

	var actual string
	err := tx.QueryRowContext(context.Background(), `
SELECT CASE c.confdeltype
	WHEN 'a' THEN 'NO ACTION'
	WHEN 'r' THEN 'RESTRICT'
	WHEN 'c' THEN 'CASCADE'
	WHEN 'n' THEN 'SET NULL'
	WHEN 'd' THEN 'SET DEFAULT'
END
FROM pg_constraint c
JOIN pg_class tbl ON tbl.oid = c.conrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
JOIN pg_class ref_tbl ON ref_tbl.oid = c.confrelid
JOIN pg_attribute attr ON attr.attrelid = tbl.oid AND attr.attnum = ANY(c.conkey)
WHERE ns.nspname = 'public'
  AND c.contype = 'f'
  AND tbl.relname = $1
  AND attr.attname = $2
  AND ref_tbl.relname = $3
LIMIT 1
`, table, column, refTable).Scan(&actual)
	require.NoError(t, err, "query foreign key action for %s.%s -> %s", table, column, refTable)
	require.Equal(t, expected, actual, "unexpected ON DELETE action for %s.%s -> %s", table, column, refTable)
}

func requireConstraintDefinitionContains(t *testing.T, tx *sql.Tx, table, constraint string, fragments ...string) {
	t.Helper()

	var def string
	err := tx.QueryRowContext(context.Background(), `
SELECT pg_get_constraintdef(c.oid)
FROM pg_constraint c
JOIN pg_class tbl ON tbl.oid = c.conrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
WHERE ns.nspname = 'public'
  AND tbl.relname = $1
  AND c.conname = $2
`, table, constraint).Scan(&def)
	require.NoError(t, err, "query constraint definition for %s.%s", table, constraint)

	for _, fragment := range fragments {
		require.Contains(t, def, fragment, "expected constraint definition for %s.%s to contain %q", table, constraint, fragment)
	}
}

func requireColumnDefaultContains(t *testing.T, tx *sql.Tx, table, column string, fragments ...string) {
	t.Helper()

	var columnDefault sql.NullString
	err := tx.QueryRowContext(context.Background(), `
SELECT column_default
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = $1
  AND column_name = $2
`, table, column).Scan(&columnDefault)
	require.NoError(t, err, "query column_default for %s.%s", table, column)
	require.True(t, columnDefault.Valid, "expected column_default for %s.%s", table, column)

	for _, fragment := range fragments {
		require.Contains(t, columnDefault.String, fragment, "expected default for %s.%s to contain %q", table, column, fragment)
	}
}

func requireColumn(t *testing.T, tx *sql.Tx, table, column, dataType string, maxLen int, nullable bool) {
	t.Helper()

	var row struct {
		DataType string
		MaxLen   sql.NullInt64
		Nullable string
	}

	err := tx.QueryRowContext(context.Background(), `
SELECT
  data_type,
  character_maximum_length,
  is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = $1
  AND column_name = $2
`, table, column).Scan(&row.DataType, &row.MaxLen, &row.Nullable)
	require.NoError(t, err, "query information_schema.columns for %s.%s", table, column)
	require.Equal(t, dataType, row.DataType, "data_type mismatch for %s.%s", table, column)

	if maxLen > 0 {
		require.True(t, row.MaxLen.Valid, "expected maxLen for %s.%s", table, column)
		require.Equal(t, int64(maxLen), row.MaxLen.Int64, "maxLen mismatch for %s.%s", table, column)
	}

	if nullable {
		require.Equal(t, "YES", row.Nullable, "nullable mismatch for %s.%s", table, column)
	} else {
		require.Equal(t, "NO", row.Nullable, "nullable mismatch for %s.%s", table, column)
	}
}
