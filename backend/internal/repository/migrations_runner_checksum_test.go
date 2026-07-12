package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func currentMigrationChecksumForTest(t *testing.T, filename string) string {
	t.Helper()
	content, err := fs.ReadFile(migrations.FS, filename)
	require.NoError(t, err)
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(content))))
	return hex.EncodeToString(sum[:])
}

func TestIsMigrationChecksumCompatible(t *testing.T) {
	t.Run("054历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"054_drop_legacy_cache_columns.sql",
			"182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d",
		)
		require.True(t, ok)
	})

	t.Run("054在未知文件checksum下不兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"054_drop_legacy_cache_columns.sql",
			"182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"0000000000000000000000000000000000000000000000000000000000000000",
		)
		require.False(t, ok)
	})

	t.Run("061历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"061_add_usage_log_request_type.sql",
			"08a248652cbab7cfde147fc6ef8cda464f2477674e20b718312faa252e0481c0",
			"66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c",
		)
		require.True(t, ok)
	})

	t.Run("061第二个历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"061_add_usage_log_request_type.sql",
			"222b4a09c797c22e5922b6b172327c824f5463aaa8760e4f621bc5c22e2be0f3",
			"66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c",
		)
		require.True(t, ok)
	})

	t.Run("非白名单迁移不兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"001_init.sql",
			"182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d",
		)
		require.False(t, ok)
	})

	t.Run("109历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"109_auth_identity_compat_backfill.sql",
			"551e498aa5616d2d91096e9d72cf9fb36e418ee22eacc557f8811cadbc9e20ee",
			"0580b4602d85435edf9aca1633db580bb3932f26517f75134106f80275ec2ace",
		)
		require.True(t, ok)
	})

	t.Run("109当前checksum可兼容历史checksum", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"109_auth_identity_compat_backfill.sql",
			"551e498aa5616d2d91096e9d72cf9fb36e418ee22eacc557f8811cadbc9e20ee",
			"0580b4602d85435edf9aca1633db580bb3932f26517f75134106f80275ec2ace",
		)
		require.True(t, ok)
	})

	t.Run("109回滚到历史文件后仍兼容已应用的新checksum", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"109_auth_identity_compat_backfill.sql",
			"0580b4602d85435edf9aca1633db580bb3932f26517f75134106f80275ec2ace",
			"551e498aa5616d2d91096e9d72cf9fb36e418ee22eacc557f8811cadbc9e20ee",
		)
		require.True(t, ok)
	})

	t.Run("110历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"110_pending_auth_and_provider_default_grants.sql",
			"e3d1f433be2b564cfbdc549adf98fce13c5c7b363ebc20fd05b765d0563b0925",
			"32cf87ee787b1bb36b5c691367c96eee37518fa3eed6f3322cf68795e3745279",
		)
		require.True(t, ok)
	})

	t.Run("112历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"112_add_payment_order_provider_key_snapshot.sql",
			"ffd3e8a2c9295fa9cbefefd629a78268877e5b51bc970a82d9b3f46ec4ebd15e",
			"b75f8f56d39455682787696a3d92ad25b055444ca328fb7fca9a460a15d68d99",
		)
		require.True(t, ok)
	})

	t.Run("115历史checksum可兼容修复后的legacy external backfill", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"115_auth_identity_legacy_external_backfill.sql",
			"4cf39e508be9fd1a5aa41610cbbebeb80385c9adda45bf78a706de9db4f1385f",
			"022aadd97bb53e755f0cf7a3a957e0cb1a1353b0c39ec4de3234acd2871fd04f",
		)
		require.True(t, ok)
	})

	t.Run("116历史checksum可兼容修复后的legacy external safety reports", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"116_auth_identity_legacy_external_safety_reports.sql",
			"f7757bd929ac67ffb08ce69fa4cf20fad39dbff9d5a5085fb2adabb7607e5877",
			"07edb09fa8d04ffb172b0621e3c22f4d1757d20a24ae267b3b36b087ab72d488",
		)
		require.True(t, ok)
	})

	t.Run("119历史checksum可兼容占位文件", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"119_enforce_payment_orders_out_trade_no_unique.sql",
			"ebd2c67cce0116393fb4f1b5d5116a67c6aceb73820dfb5133d1ff6f36d72d34",
			"0bbe809ae48a9d811dabda1ba1c74955bd71c4a9cc610f9128816818dfa6c11e",
		)
		require.True(t, ok)
	})

	t.Run("118多个历史checksum都可兼容当前版本", func(t *testing.T) {
		for _, dbChecksum := range []string{
			"a38243ca0a72c3a01c0a92b7986423054d6133c0399441f853b99802852720fb",
			"e0cdf835d6c688d64100f483d31bc02ac9ebad414bf1837af239a84bf75b8227",
		} {
			ok := isMigrationChecksumCompatible(
				"118_wechat_dual_mode_and_auth_source_defaults.sql",
				dbChecksum,
				"b54194d7a3e4fbf710e0a3590d22a2fe7966804c487052a356e0b55f53ef96b0",
			)
			require.True(t, ok)
		}
	})

	t.Run("120多个历史checksum都可兼容新的notx修复版本", func(t *testing.T) {
		for _, dbChecksum := range []string{
			"e77921f79d539bc24575cb9c16cbe566d2b23ce816190343d0a7568f6a3fcf61",
			"707431450603e70a43ce9fbd61e0c12fa67da4875158ccefabacea069587ab22",
			"04b082b5a239c525154fe9185d324ee2b05ff90da9297e10dba19f9be79aa59a",
		} {
			ok := isMigrationChecksumCompatible(
				"120_enforce_payment_orders_out_trade_no_unique_notx.sql",
				dbChecksum,
				"34aadc0db59a4e390f92a12b73bd74642d9724f33124f73638ae00089ea5e074",
			)
			require.True(t, ok)
		}
	})

	t.Run("131历史checksum可兼容当前版本", func(t *testing.T) {
		for _, dbChecksum := range []string{
			"00b2290e6646666df46409564b545b1de91b222db8f3ca0c992164a6dc4034e7",
			"9fd0a6021290b24c7e76d4ff6405824eef528a6afe969e845c8cc3bf8053ba15",
			"706c8102d96d0a10f2e2a23156a8cd8b414a241591fd65ab3e26425b2a54fe29",
			"c4b74b9dd08e3634ac9b752376e92ce41f27fa0cb8046d7932944ca61e5f351c",
			"b20a2678be74db6a5a9a376004f4bf5bc7844ab46ee2f1e09194e8b1c48d49fd",
		} {
			ok := isMigrationChecksumCompatible(
				"131_affiliate_rebate_hardening.sql",
				dbChecksum,
				currentMigrationChecksumForTest(t, "131_affiliate_rebate_hardening.sql"),
			)
			require.True(t, ok)
		}
	})

	t.Run("131回滚到历史文件checksum时仍兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"131_affiliate_rebate_hardening.sql",
			"da8f7e442df20609449c51b13c250a2f79d3bb95c50f1c11b96b8108e5dddb02",
			"9fd0a6021290b24c7e76d4ff6405824eef528a6afe969e845c8cc3bf8053ba15",
		)
		require.True(t, ok)
	})

	t.Run("132历史checksum可兼容当前版本", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"132_affiliate_policy_limits.sql",
			"51f95d399e30dc499e9d1bc3bdefc5a7f5b358726ac83242ec64a363a6bfe092",
			currentMigrationChecksumForTest(t, "132_affiliate_policy_limits.sql"),
		)
		require.True(t, ok)
	})

	t.Run("137历史checksum可兼容当前版本", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"137_subscription_fulfillment_claim_dedupe.sql",
			"1e4fb3a58b36572fd37a86c1a363f9d1458887daf8dc1b55c0b98f6b136bd50c",
			currentMigrationChecksumForTest(t, "137_subscription_fulfillment_claim_dedupe.sql"),
		)
		require.True(t, ok)
	})

	t.Run("138历史checksum可兼容当前版本", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"138_subscription_fulfillment_claim_unique_notx.sql",
			"fcdbbbcfa9010f6b2b0e9b6210a63d103eec5081358e70f591dec8a818c93009",
			currentMigrationChecksumForTest(t, "138_subscription_fulfillment_claim_unique_notx.sql"),
		)
		require.True(t, ok)
	})

	t.Run("184约束误追加checksum可兼容回滚后的原始迁移", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"184_add_group_audio_search_pricing.sql",
			"56b65dbc1dc1927c193b5b5bf74cf458a20070d5c12c23a28f95933f87d72ae9",
			currentMigrationChecksumForTest(t, "184_add_group_audio_search_pricing.sql"),
		)
		require.True(t, ok)
	})

	t.Run("184原始checksum可兼容曾误追加约束的文件checksum", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"184_add_group_audio_search_pricing.sql",
			currentMigrationChecksumForTest(t, "184_add_group_audio_search_pricing.sql"),
			"56b65dbc1dc1927c193b5b5bf74cf458a20070d5c12c23a28f95933f87d72ae9",
		)
		require.True(t, ok)
	})

	t.Run("151历史checksum可兼容当前版本", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"151_apply_rpm_parallel_constraints_and_replace_claude_code_template.sql",
			"0fe932f50177afc05846b8489715030518659ef97c7f4ac76a381b2c57d26199",
			currentMigrationChecksumForTest(t, "151_apply_rpm_parallel_constraints_and_replace_claude_code_template.sql"),
		)
		require.True(t, ok)
	})

	t.Run("156历史checksum可兼容当前版本", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"156_user_platform_quotas_add_kiro.sql",
			"7b92806712dbd12b5bd2af0f582a76a0b8f3503f91b92b4efab361e5c53a3105",
			currentMigrationChecksumForTest(t, "156_user_platform_quotas_add_kiro.sql"),
		)
		require.True(t, ok)
	})

	t.Run("181历史checksum可兼容当前版本", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"181_user_platform_quotas_add_grok.sql",
			"4fc04d9a7550719aa672d4d4377eb099f37edfa688ea4caeafc8f3a77ef86db7",
			currentMigrationChecksumForTest(t, "181_user_platform_quotas_add_grok.sql"),
		)
		require.True(t, ok)
	})

	t.Run("176_tls_capture_unification历史checksum可兼容当前版本", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"176_tls_fingerprint_capture_unification.sql",
			"bf4072bb168f5a2af0eb33c46cf69ff4410d7c3bccaf7c6b1906e7e1ee006951",
			currentMigrationChecksumForTest(t, "176_tls_fingerprint_capture_unification.sql"),
		)
		require.True(t, ok)
	})

	t.Run("187历史checksum可兼容当前版本", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"187_allow_native_image_route_and_video_price_checks.sql",
			"547c29809e0c72d00b9aebbbd792e718e677a4906e8e6cd744031aa15a021c14",
			currentMigrationChecksumForTest(t, "187_allow_native_image_route_and_video_price_checks.sql"),
		)
		require.True(t, ok)
	})

	t.Run("188历史checksum可兼容当前版本", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"188_add_group_audio_search_price_checks.sql",
			"e6cf495ea038076bc1f9d03b36eec81f6e517cfb81dbf346a4ef3d802f7e0be0",
			currentMigrationChecksumForTest(t, "188_add_group_audio_search_price_checks.sql"),
		)
		require.True(t, ok)
	})

	t.Run("195历史checksum可兼容拆分后的事务迁移", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"195_add_invoice_order_active_unique_guard.sql",
			"a775587a040b17780ed37e5fe8efcf223de52263f1f75a5bb4b7c1f6af0b7a99",
			currentMigrationChecksumForTest(t, "195_add_invoice_order_active_unique_guard.sql"),
		)
		require.True(t, ok)
	})

	t.Run("199历史checksum可兼容拆分后的事务迁移", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"199_batch_image_idempotency_unique.sql",
			"7b75bba89c5e33f996a6bdde61bec4751daf6c410cd9259e63a5b71829fe898b",
			currentMigrationChecksumForTest(t, "199_batch_image_idempotency_unique.sql"),
		)
		require.True(t, ok)
	})

	t.Run("119未知checksum不兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"119_enforce_payment_orders_out_trade_no_unique.sql",
			"ebd2c67cce0116393fb4f1b5d5116a67c6aceb73820dfb5133d1ff6f36d72d34",
			"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		)
		require.False(t, ok)
	})
}
