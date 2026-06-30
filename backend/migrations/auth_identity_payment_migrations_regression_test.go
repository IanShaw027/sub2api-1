package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration112UsesIdempotentAddColumn(t *testing.T) {
	content, err := FS.ReadFile("112_add_payment_order_provider_key_snapshot.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS provider_key VARCHAR(30)")
	require.NotContains(t, sql, "ADD COLUMN provider_key VARCHAR(30);")
}

func TestMigration118DoesNotForceOverwriteAuthSourceGrantDefaults(t *testing.T) {
	content, err := FS.ReadFile("118_wechat_dual_mode_and_auth_source_defaults.sql")
	require.NoError(t, err)

	sql := string(content)
	require.NotContains(t, sql, "UPDATE settings")
	require.NotContains(t, sql, "SET value = 'false'")
	require.True(t, strings.Contains(sql, "ON CONFLICT (key) DO NOTHING"))
	require.Contains(t, sql, "THEN ''")
}

func TestAuthIdentityReportTypeWideningRunsBeforeLongReportWritersAndStillReconcilesAt121(t *testing.T) {
	preflightContent, err := FS.ReadFile("108a_widen_auth_identity_migration_report_type.sql")
	require.NoError(t, err)

	preflightSQL := string(preflightContent)
	require.Contains(t, preflightSQL, "ALTER TABLE auth_identity_migration_reports")
	require.Contains(t, preflightSQL, "ALTER COLUMN report_type TYPE VARCHAR(80)")

	content, err := FS.ReadFile("109_auth_identity_compat_backfill.sql")
	require.NoError(t, err)

	sql := string(content)
	require.NotContains(t, sql, "ALTER TABLE auth_identity_migration_reports")

	followupContent, err := FS.ReadFile("121_auth_identity_migration_report_type_widen.sql")
	require.NoError(t, err)

	followupSQL := string(followupContent)
	require.Contains(t, followupSQL, "ALTER TABLE auth_identity_migration_reports")
	require.Contains(t, followupSQL, "ALTER COLUMN report_type TYPE VARCHAR(80)")
}

func TestMigration119DefersPaymentIndexRolloutToOnlineFollowup(t *testing.T) {
	content, err := FS.ReadFile("119_enforce_payment_orders_out_trade_no_unique.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "120_enforce_payment_orders_out_trade_no_unique_notx.sql")
	require.Contains(t, sql, "NULL;")
	require.NotContains(t, sql, "CREATE UNIQUE INDEX")
	require.NotContains(t, sql, "DROP INDEX")

	followupContent, err := FS.ReadFile("120_enforce_payment_orders_out_trade_no_unique_notx.sql")
	require.NoError(t, err)

	followupSQL := string(followupContent)
	require.Contains(t, followupSQL, "explicit duplicate out_trade_no precheck")
	require.Contains(t, followupSQL, "stale invalid paymentorder_out_trade_no_unique index")
	require.Contains(t, followupSQL, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS paymentorder_out_trade_no_unique")
	require.NotContains(t, followupSQL, "DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no_unique")
	require.Contains(t, followupSQL, "DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no")
	require.Contains(t, followupSQL, "WHERE out_trade_no <> ''")

	alignmentContent, err := FS.ReadFile("120a_align_payment_orders_out_trade_no_index_name.sql")
	require.NoError(t, err)

	alignmentSQL := string(alignmentContent)
	require.Contains(t, alignmentSQL, "paymentorder_out_trade_no_unique")
	require.Contains(t, alignmentSQL, "RENAME TO paymentorder_out_trade_no")
}

func TestAffiliateMigrationsDeferHotUniqueIndexesToOnlineFollowup(t *testing.T) {
	content131, err := FS.ReadFile("131_affiliate_rebate_hardening.sql")
	require.NoError(t, err)

	sql131 := string(content131)
	require.NotContains(t, sql131, "CREATE UNIQUE INDEX")
	require.NotContains(t, sql131, "DROP INDEX")

	content132, err := FS.ReadFile("132_affiliate_policy_limits.sql")
	require.NoError(t, err)

	sql132 := string(content132)
	require.NotContains(t, sql132, "CREATE UNIQUE INDEX")

	followupContent, err := FS.ReadFile("138_subscription_fulfillment_claim_unique_notx.sql")
	require.NoError(t, err)

	followupSQL := string(followupContent)
	require.Contains(t, followupSQL, "duplicate payment_audit_logs/order_id+action precheck")
	require.Contains(t, followupSQL, "user_affiliate_ledger/user_id+source_order_id+action precheck")
	require.Contains(t, followupSQL, "user_affiliate_ledger/user_id+action precheck")
	require.Contains(t, followupSQL, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_payment_audit_logs_order_action_uniq")
	require.Contains(t, followupSQL, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliate_ledger_order_action_unique")
	require.Contains(t, followupSQL, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliate_signup_bonus_once")
}

func TestMigration110SeedsAuthSourceSignupGrantsDisabledByDefault(t *testing.T) {
	content, err := FS.ReadFile("110_pending_auth_and_provider_default_grants.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "('auth_source_default_email_grant_on_signup', 'false')")
	require.Contains(t, sql, "('auth_source_default_linuxdo_grant_on_signup', 'false')")
	require.Contains(t, sql, "('auth_source_default_oidc_grant_on_signup', 'false')")
	require.Contains(t, sql, "('auth_source_default_wechat_grant_on_signup', 'false')")
	require.NotContains(t, sql, "('auth_source_default_email_grant_on_signup', 'true')")
}

func TestMigration122ScrubsPendingOAuthCompletionTokensAtRest(t *testing.T) {
	content, err := FS.ReadFile("122_pending_auth_completion_token_cleanup.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "UPDATE pending_auth_sessions")
	require.Contains(t, sql, "completion_response")
	require.Contains(t, sql, "access_token")
	require.Contains(t, sql, "refresh_token")
	require.Contains(t, sql, "expires_in")
	require.Contains(t, sql, "token_type")
}

func TestMigration123BackfillsLegacyAuthSourceGrantDefaultsSafely(t *testing.T) {
	content, err := FS.ReadFile("123_fix_legacy_auth_source_grant_on_signup_defaults.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "110_pending_auth_and_provider_default_grants.sql")
	require.Contains(t, sql, "schema_migrations")
	require.Contains(t, sql, "updated_at")
	require.Contains(t, sql, "'_grant_on_signup'")
	require.Contains(t, sql, "value = 'false'")
	require.Contains(t, sql, "auth_identity_migration_reports")
}

func TestMigration124BackfillsLegacyOIDCSecurityFlagsSafely(t *testing.T) {
	content, err := FS.ReadFile("124_backfill_legacy_oidc_security_flags.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "oidc_connect_use_pkce")
	require.Contains(t, sql, "oidc_connect_validate_id_token")
	require.Contains(t, sql, "ON CONFLICT (key) DO NOTHING")
	require.Contains(t, sql, "oidc_connect_enabled")
	require.Contains(t, sql, "'false'")
}

func TestMigration134AddsAffiliateLedgerAuditFieldsWithoutJSONCast(t *testing.T) {
	content, err := FS.ReadFile("134_affiliate_ledger_audit_snapshots.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS source_order_id BIGINT")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS balance_after DECIMAL(20,8)")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS aff_quota_after DECIMAL(20,8)")
	require.Contains(t, sql, "substring(")
	require.Contains(t, sql, `"rebateAmount"`)
	require.Contains(t, sql, "COUNT(*) OVER (PARTITION BY ra.order_id) AS order_match_count")
	require.Contains(t, sql, "COUNT(*) OVER (PARTITION BY ual.id) AS ledger_match_count")
	require.NotContains(t, sql, "detail::jsonb")
}

func TestMigration134AddsImageGenerationGroupControlsWithoutRepricingExistingColumns(t *testing.T) {
	content, err := FS.ReadFile("134_image_generation_group_controls.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS allow_image_generation BOOLEAN NOT NULL DEFAULT false")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS image_rate_independent BOOLEAN NOT NULL DEFAULT false")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS image_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0")
	require.Contains(t, sql, "WHERE platform IN ('openai', 'gemini', 'antigravity')")
	require.Contains(t, sql, "SET image_rate_independent = false,")
	require.Contains(t, sql, "image_rate_multiplier = 1.0")
	require.Contains(t, sql, "COMMENT ON COLUMN groups.allow_image_generation")
	require.Contains(t, sql, "COMMENT ON COLUMN groups.image_rate_independent")
	require.Contains(t, sql, "COMMENT ON COLUMN groups.image_rate_multiplier")
	require.NotContains(t, sql, "ALTER COLUMN image_price_1k")
	require.NotContains(t, sql, "ALTER COLUMN image_price_2k")
	require.NotContains(t, sql, "ALTER COLUMN image_price_4k")
	require.NotContains(t, sql, "UPDATE groups\nSET image_price_")
}

func TestMigration135AllowsGitHubAndGoogleAuthProviders(t *testing.T) {
	content, err := FS.ReadFile("135_allow_email_oauth_provider_types.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "users_signup_source_check")
	require.Contains(t, sql, "auth_identities_provider_type_check")
	require.Contains(t, sql, "auth_identity_channels_provider_type_check")
	require.Contains(t, sql, "pending_auth_sessions_provider_type_check")
	require.Contains(t, sql, "'github'")
	require.Contains(t, sql, "'google'")
}

func TestMigration153UsesConfiguredTimezoneBucketsForSplitCostBackfill(t *testing.T) {
	content, err := FS.ReadFile("153_add_dashboard_billing_split_costs.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "date_trunc('hour', created_at AT TIME ZONE current_setting('TIMEZONE')) AT TIME ZONE current_setting('TIMEZONE')")
	require.Contains(t, sql, "(created_at AT TIME ZONE current_setting('TIMEZONE'))::date AS bucket_date")
	require.NotContains(t, sql, "AT TIME ZONE 'UTC'")
}

func TestMigration187AllowsNativeImageRouteAndAddsVideoPriceChecks(t *testing.T) {
	content, err := FS.ReadFile("187_allow_native_image_route_and_video_price_checks.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS groups_image_generation_route_check")
	require.Contains(t, sql, "image_generation_route IN ('codex', 'web2api', 'native')")
	for _, name := range []string{
		"groups_video_price_480p_per_sec_non_negative",
		"groups_video_price_720p_per_sec_non_negative",
		"groups_video_price_1080p_per_sec_non_negative",
		"groups_video_price_4k_per_sec_non_negative",
	} {
		require.Contains(t, sql, name)
	}
	require.Contains(t, sql, "video_price_480p_per_sec IS NULL OR video_price_480p_per_sec >= 0")
	require.Contains(t, sql, "video_price_720p_per_sec IS NULL OR video_price_720p_per_sec >= 0")
	require.Contains(t, sql, "video_price_1080p_per_sec IS NULL OR video_price_1080p_per_sec >= 0")
	require.Contains(t, sql, "video_price_4k_per_sec IS NULL OR video_price_4k_per_sec >= 0")
}

func TestMigration188AddsAudioSearchPriceChecksWithoutMutating184(t *testing.T) {
	content184, err := FS.ReadFile("184_add_group_audio_search_pricing.sql")
	require.NoError(t, err)
	sql184 := string(content184)
	require.Contains(t, sql184, "ADD COLUMN IF NOT EXISTS search_price_per_1k")
	for _, name := range []string{
		"groups_search_price_per_1k_non_negative",
		"groups_audio_realtime_price_per_min_non_negative",
		"groups_audio_tts_price_per_million_chars_non_negative",
		"groups_audio_stt_price_per_hour_non_negative",
	} {
		require.NotContains(t, sql184, name)
	}

	content188, err := FS.ReadFile("188_add_group_audio_search_price_checks.sql")
	require.NoError(t, err)
	sql188 := string(content188)
	for _, name := range []string{
		"groups_search_price_per_1k_non_negative",
		"groups_audio_realtime_price_per_min_non_negative",
		"groups_audio_tts_price_per_million_chars_non_negative",
		"groups_audio_stt_price_per_hour_non_negative",
	} {
		require.Contains(t, sql188, "IF NOT EXISTS")
		require.Contains(t, sql188, name)
	}
	require.Contains(t, sql188, "search_price_per_1k IS NULL OR search_price_per_1k >= 0")
	require.Contains(t, sql188, "audio_realtime_price_per_min IS NULL OR audio_realtime_price_per_min >= 0")
	require.Contains(t, sql188, "audio_tts_price_per_million_chars IS NULL OR audio_tts_price_per_million_chars >= 0")
	require.Contains(t, sql188, "audio_stt_price_per_hour IS NULL OR audio_stt_price_per_hour >= 0")
}

func TestMigration189UpdatesOpsErrorRequestTypeComment(t *testing.T) {
	content, err := FS.ReadFile("189_update_ops_error_request_type_comment.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "COMMENT ON COLUMN ops_error_logs.request_type")
	require.Contains(t, sql, "6=cyber")
	require.Contains(t, sql, "7=video")
}

func TestMigration190ClearsNonGrokVideoGenerationConfig(t *testing.T) {
	content, err := FS.ReadFile("190_clear_non_grok_video_generation_config.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "UPDATE groups")
	require.Contains(t, sql, "allow_video_generation = false")
	require.Contains(t, sql, "video_generation_route = 'native'")
	require.Contains(t, sql, "video_price_480p_per_sec = NULL")
	require.Contains(t, sql, "video_price_720p_per_sec = NULL")
	require.Contains(t, sql, "video_price_1080p_per_sec = NULL")
	require.Contains(t, sql, "video_price_4k_per_sec = NULL")
	require.Contains(t, sql, "WHERE platform IS DISTINCT FROM 'grok'")
	require.Contains(t, sql, "allow_video_generation IS DISTINCT FROM false")
	require.NotContains(t, sql, "WHERE platform = 'grok'")
}

func TestMigration132BackfillsHistoricalAffiliateLedgerRowsSafely(t *testing.T) {
	content, err := FS.ReadFile("132_affiliate_policy_limits.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "AFFILIATE_REBATE_APPLIED")
	require.Contains(t, sql, "source_order_id IS NULL")
	require.Contains(t, sql, "order_match_count = 1")
	require.Contains(t, sql, "ledger_match_count = 1")
	require.Contains(t, sql, "base_amount = CASE")
	require.Contains(t, sql, "rebate_rate = CASE")
}

func TestMigration145PreservesTemplateAssociationsWhileConvergingTemplateSet(t *testing.T) {
	content, err := FS.ReadFile("145_seed_client_spoof_channel_monitor_templates.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "UPDATE channel_monitors")
	require.Contains(t, sql, "template_id")
	require.Contains(t, sql, "Anthropic 请求模板示例")
	require.Contains(t, sql, "DELETE FROM channel_monitor_request_templates")
	require.Contains(t, sql, "WHERE id = v_example_id")
}

func TestMigration149BackfillsLegacyWeb2APIGroupsBeforeEnforcingRouteDefault(t *testing.T) {
	content, err := FS.ReadFile("149_add_group_image_generation_route.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS image_generation_route character varying(20)")
	require.Contains(t, sql, "SET image_generation_route = 'web2api'")
	require.Contains(t, sql, "image_rate_independent = TRUE")
	require.Contains(t, sql, "images2api_price_1k IS NOT NULL")
	require.Contains(t, sql, "SET DEFAULT 'codex'")
	require.Contains(t, sql, "AND NOT (")
}
