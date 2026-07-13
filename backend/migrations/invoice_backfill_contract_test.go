package migrations

import (
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration158FailsLoudlyWhenInvoiceTargetsAlreadyContainData(t *testing.T) {
	content, err := FS.ReadFile("158_backfill_invoices_from_applications.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "IF EXISTS (SELECT 1 FROM invoices LIMIT 1)")
	require.Contains(t, sql, "OR EXISTS (SELECT 1 FROM invoice_orders LIMIT 1)")
	require.Contains(t, sql, "RAISE EXCEPTION")
	require.Contains(t, sql, "人工")
	require.NotContains(t, sql, "RAISE NOTICE 'invoices/invoice_orders 已有数据")
}

func TestMigration195AddsInvoiceOrderActiveGuardrail(t *testing.T) {
	content, err := FS.ReadFile("195_add_invoice_order_active_unique_guard.sql")
	require.NoError(t, err)

	sql := normalizeMigrationSQLForSafetyTest(string(content))
	require.Contains(t, sql, "ALTER TABLE INVOICE_ORDERS ADD COLUMN IF NOT EXISTS IS_ACTIVE BOOLEAN NOT NULL DEFAULT TRUE")
	require.Contains(t, sql, "UPDATE INVOICE_ORDERS IO")
	require.Contains(t, sql, "SET IS_ACTIVE = CASE WHEN I.STATUS = 'CANCELLED' THEN FALSE ELSE TRUE END")
	require.Contains(t, sql, "FROM INVOICES I")
	require.Contains(t, sql, "I.STATUS = 'CANCELLED'")
	require.Contains(t, sql, "RAISE EXCEPTION 'CANNOT ENFORCE INVOICE ORDER ACTIVE UNIQUENESS")
	require.Contains(t, sql, "HAVING COUNT(*) > 1")
	require.Contains(t, sql, "RESOLVE MANUALLY BEFORE MIGRATION")
	require.NotContains(t, sql, "ROW_NUMBER() OVER")
	require.NotContains(t, sql, "RN > 1")
	require.NotContains(t, sql, "DROP INDEX")
	require.NotContains(t, sql, "CREATE UNIQUE INDEX")

	notxContent, err := FS.ReadFile("195a_add_invoice_order_active_unique_guard_notx.sql")
	require.NoError(t, err)

	notxSQL := normalizeMigrationSQLForSafetyTest(string(notxContent))
	// Create-first under a new name so a failed unique build never removes the
	// legacy non-unique invoiceorder_order_id index.
	createStmt := "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS INVOICEORDER_ORDER_ID_ACTIVE_UNIQUE"
	dropStmt := "DROP INDEX CONCURRENTLY IF EXISTS INVOICEORDER_ORDER_ID"
	require.Contains(t, notxSQL, createStmt)
	require.Contains(t, notxSQL, dropStmt)
	require.Contains(t, notxSQL, "ON INVOICE_ORDERS (ORDER_ID)")
	require.Contains(t, notxSQL, "WHERE IS_ACTIVE = TRUE")
	require.Less(t, strings.Index(notxSQL, createStmt), strings.Index(notxSQL, dropStmt))
}

func TestTLSFingerprintSeedCleanupOnlyTargetsExactSeededAccountExtra(t *testing.T) {
	content, err := FS.ReadFile("172_cleanup_synthetic_tls_fingerprint_seed.sql")
	require.NoError(t, err)

	sql := string(content)
	require.NotContains(t, sql, "extra @>", "cleanup must not remove manually configured TLS fingerprint settings from richer account extra JSON")
	require.Contains(t, sql, `COALESCE(extra, '{}'::jsonb) = '{"enable_tls_fingerprint": true, "tls_fingerprint_profile_id": -1}'::jsonb`)
}

func TestTLSFingerprintCaptureSampleNativeColumnsMigration(t *testing.T) {
	content, err := FS.ReadFile("174_tls_fingerprint_capture_sample_native_columns.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ALTER TABLE tls_fingerprint_capture_samples")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS originator VARCHAR(50) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS raw_client_hello BYTEA")
}

func TestEmbeddedMigrationsDoNotContainDownSections(t *testing.T) {
	files, err := fs.Glob(FS, "*.sql")
	require.NoError(t, err)

	for _, file := range files {
		content, err := FS.ReadFile(file)
		require.NoError(t, err)
		require.NotContains(t, string(content), "+migrate Down", "%s contains a Down section, but the migration runner executes the whole file", file)
	}
}

func TestMigrationFilenameNumericPrefixesStayDeliberate(t *testing.T) {
	files, err := fs.Glob(FS, "*.sql")
	require.NoError(t, err)

	prefixPattern := regexp.MustCompile(`^\d+[a-z]?_`)
	knownDuplicatePrefixes := map[string][]string{
		"006": {"006_add_users_allowed_groups_compat.sql", "006_fix_invalid_subscription_expires_at.sql", "006b_guard_users_allowed_groups.sql"},
		"028": {"028_add_account_notes.sql", "028_add_usage_logs_user_agent.sql", "028_group_image_pricing.sql"},
		"029": {"029_add_group_claude_code_restriction.sql", "029_usage_log_image_fields.sql"},
		"033": {"033_add_promo_codes.sql", "033_ops_monitoring_vnext.sql"},
		"034": {"034_ops_upstream_error_events.sql", "034_usage_dashboard_aggregation_tables.sql"},
		"036": {"036_ops_error_logs_add_is_count_tokens.sql", "036_scheduler_outbox.sql"},
		"037": {"037_add_account_rate_multiplier.sql", "037_ops_alert_silences.sql"},
		"042": {"042_add_usage_cleanup_tasks.sql", "042b_add_ops_system_metrics_switch_count.sql"},
		"043": {"043_add_usage_cleanup_cancel_audit.sql", "043b_add_group_invalid_request_fallback.sql"},
		"044": {"044_add_user_totp.sql", "044b_add_group_mcp_xml_inject.sql"},
		"045": {"045_add_accounts_extra_index.sql", "045_add_announcements.sql", "045_add_api_key_quota.sql"},
		"046": {"046_add_sora_accounts.sql", "046_add_usage_log_reasoning_effort.sql", "046b_add_group_supported_model_scopes.sql"},
		"047": {"047_add_sora_pricing_and_media_type.sql", "047_add_user_group_rate_multipliers.sql"},
		"052": {"052_add_group_sort_order.sql", "052_migrate_upstream_to_apikey.sql"},
		"053": {"053_add_security_secrets.sql", "053_add_skip_monitoring_to_error_passthrough.sql"},
		"054": {"054_drop_legacy_cache_columns.sql", "054_ops_system_logs.sql"},
		"060": {"060_add_gemini31_flash_image_to_model_mapping.sql", "060_add_usage_log_openai_ws_mode.sql"},
		"070": {"070_add_scheduled_test_auto_recover.sql", "070_add_usage_log_service_tier.sql"},
		"071": {"071_add_gemini25_flash_image_to_model_mapping.sql", "071_add_usage_billing_dedup.sql"},
		"075": {"075_add_usage_log_upstream_model.sql", "075_map_haiku45_to_sonnet46.sql"},
		"081": {"081_add_group_account_filter.sql", "081_create_channels.sql"},
		"095": {"095_channel_features.sql", "095_subscription_plans.sql"},
		"101": {"101_add_account_stats_pricing.sql", "101_add_balance_notify_fields.sql", "101_add_channel_features_config.sql", "101_add_payment_mode.sql"},
		"102": {"102_add_balance_notify_threshold_type.sql", "102_add_out_trade_no_to_payment_orders.sql"},
		"108": {"108_auth_identity_foundation_core.sql", "108a_widen_auth_identity_migration_report_type.sql"},
		"120": {"120_enforce_payment_orders_out_trade_no_unique_notx.sql", "120a_align_payment_orders_out_trade_no_index_name.sql"},
		"125": {"125_add_channel_monitors.sql", "125_add_group_rpm_limit.sql"},
		"126": {"126_add_channel_monitor_aggregation.sql", "126_add_user_rpm_limit.sql"},
		"127": {"127_add_user_group_rpm_override.sql", "127_drop_channel_monitor_deleted_at.sql"},
		"128": {"128_add_channel_monitor_request_templates.sql", "128_add_support_tickets.sql"},
		"132": {"132_affiliate_custom_settings.sql", "132_affiliate_policy_limits.sql"},
		"133": {"133_add_user_token_version.sql", "133_affiliate_rebate_freeze.sql"},
		"134": {"134_affiliate_ledger_audit_snapshots.sql", "134_align_channel_monitor_request_template_constraints.sql", "134_image_generation_group_controls.sql"},
		"135": {"135_allow_email_oauth_provider_types.sql", "135_clean_channel_monitor_soft_deleted_rows.sql", "135_content_moderation.sql"},
		"136": {"136_add_dingtalk_provider_type.sql", "136_align_channel_monitor_template_index_and_cleanup.sql", "136_remove_ops_retry_replay.sql", "136_usage_log_image_size_metadata.sql"},
		"137": {"137_align_channel_monitor_core_indexes.sql", "137_redeem_code_expires_at.sql", "137_subscription_fulfillment_claim_dedupe.sql"},
		"138": {"138_channel_monitor_openai_api_mode.sql", "138_subscription_fulfillment_claim_unique_notx.sql"},
		"139": {"139_add_group_images2api_pricing.sql", "139_seed_openai_monitor_templates.sql"},
		"140": {"140_add_group_display_name_and_user_selectable.sql", "140_create_media_assets.sql", "140_extend_user_provider_default_grants_check.sql"},
		"141": {"141_add_media_thumbnail_mime_type.sql", "141_subscription_expiry_notify_enabled.sql"},
		"142": {"142_add_media_storage_profile_id.sql", "142_create_ai_skill_center.sql", "142_user_platform_quotas.sql"},
		"143": {"143_create_ai_skill_installs.sql", "143_group_models_list_config.sql"},
		"148": {"148_expand_usage_log_request_type_check.sql", "148a_validate_usage_log_request_type_check.sql"},
		"151": {"151_account_autopause_expiry_index_notx.sql", "151_apply_rpm_parallel_constraints_and_replace_claude_code_template.sql", "151_channel_monitor_jitter.sql"},
		"152": {"152_add_group_openai_image_main_model.sql", "152_payment_refund_self_service.sql", "152_scheduler_outbox_dedup_key.sql"},
		"153": {"153_add_dashboard_billing_split_costs.sql", "153_scheduler_outbox_pending_dedup_key_index_notx.sql"},
		"154": {"154_account_spark_shadow.sql", "154_add_ops_system_logs_api_key_id.sql", "154_add_usage_logs_user_created_at_covering_index_notx.sql", "154a_account_spark_shadow_indexes_notx.sql"},
		"155": {"155_add_ops_system_logs_api_key_id_index_notx.sql", "155_add_ticket_message_attachments.sql"},
		"156": {"156_content_moderation_matched_keyword.sql", "156_user_platform_quotas_add_kiro.sql"},
		"158": {"158_add_group_peak_rate_multiplier.sql", "158_backfill_invoices_from_applications.sql", "158_enable_grok_media_generation_groups.sql"},
		"159": {"159_ai_skill_versions_add_approved_artifact_digest.sql", "159_batch_image_foundation.sql"},
		"160": {"160_add_usage_log_openai_ws_profile.sql", "160_add_user_frozen_balance.sql", "160_batch_image_provider_refs.sql"},
		"161": {"161_add_opus48_to_model_mapping.sql", "161_batch_image_pricing_snapshot.sql", "161_channel_monitor_add_kiro_provider.sql"},
		"162": {"162_add_group_batch_image_generation_gate.sql", "162_create_codex_invite_reset_history.sql", "162_create_tls_fingerprint_routers.sql", "162_deleted_api_key_audit.sql"},
		"163": {"163_batch_image_default_discount_and_hold_ratio.sql", "163_ops_metrics_ttft_sample_count.sql"},
		"164": {"164_batch_image_download_and_user_delete.sql", "164_ops_error_log_api_key_prefix.sql"},
		"165": {"165_add_ops_error_logs_user_time_index_notx.sql", "165_hide_pre_upstream_batch_image_failures.sql"},
		"166": {"166_batch_image_task_name.sql", "166_proxy_expiry_fallback.sql"},
		"167": {"167_account_group_scheduler_indexes_notx.sql", "167_clear_auto_batch_image_task_names.sql"},
		"168": {"168_add_usage_log_provider.sql", "168_restore_empty_batch_image_task_names.sql"},
		"169": {"169_align_group_display_name_length.sql", "169_batch_image_parent_batch.sql"},
		"170": {"170_add_grok_video_pricing_controls.sql", "170_ai_skill_creator_earnings_source_run.sql"},
		"171": {"171_allow_video_usage_without_image_size.sql", "171_tls_fingerprint_capture_tasks.sql"},
		"172": {"172_cleanup_synthetic_tls_fingerprint_seed.sql", "172_video_per_second_billing_metadata.sql"},
		"176": {"176_add_tls_fingerprint_profile_transport.sql", "176_tls_fingerprint_capture_unification.sql"},
		"185": {"185_expand_usage_log_request_type_check.sql", "185a_validate_usage_log_request_type_check.sql"},
		"191": {"191_add_user_affiliate_ledger_reverse_action_unique_notx.sql", "191_restore_usage_request_type_cyber_value.sql", "191a_validate_usage_log_request_type_check.sql"},
		"192": {"192_audit_retention_created_at_indexes_notx.sql", "192_ops_sticky_schedule_events.sql"},
		"193": {"193_add_usage_log_video_billing_details.sql", "193_add_usage_log_video_billing_details_index_notx.sql", "193_create_usage_user_daily_cost.sql"},
		"195": {"195_add_invoice_order_active_unique_guard.sql", "195a_add_invoice_order_active_unique_guard_notx.sql"},
		"199": {"199_batch_image_idempotency_unique.sql", "199a_batch_image_idempotency_unique_notx.sql"},
	}

	byPrefix := make(map[string][]string)
	for _, file := range files {
		prefix := strings.TrimSuffix(prefixPattern.FindString(file), "_")
		require.NotEmpty(t, prefix, "migration filename must start with a numeric prefix: %s", file)
		prefix = strings.TrimRight(prefix, "abcdefghijklmnopqrstuvwxyz")
		byPrefix[prefix] = append(byPrefix[prefix], file)
	}

	for prefix, got := range byPrefix {
		if len(got) < 2 {
			continue
		}
		sort.Strings(got)
		require.Equal(t, knownDuplicatePrefixes[prefix], got, "unexpected duplicate migration prefix %s", prefix)
	}
}
