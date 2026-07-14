package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
)

// schemaMigrationsTableDDL 定义迁移记录表的 DDL。
// 该表用于跟踪已应用的迁移文件及其校验和。
// - filename: 迁移文件名，作为主键唯一标识每个迁移
// - checksum: 文件内容的 SHA256 哈希值，用于检测迁移文件是否被篡改
// - applied_at: 迁移应用时间戳
const schemaMigrationsTableDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	filename   TEXT PRIMARY KEY,
	checksum   TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const atlasSchemaRevisionsTableDDL = `
CREATE TABLE IF NOT EXISTS atlas_schema_revisions (
	version TEXT PRIMARY KEY,
	description TEXT NOT NULL,
	type INTEGER NOT NULL,
	applied INTEGER NOT NULL DEFAULT 0,
	total INTEGER NOT NULL DEFAULT 0,
	executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	execution_time BIGINT NOT NULL DEFAULT 0,
	error TEXT NULL,
	error_stmt TEXT NULL,
	hash TEXT NOT NULL DEFAULT '',
	partial_hashes TEXT[] NULL,
	operator_version TEXT NULL
);
`

// migrationsAdvisoryLockID 是用于序列化迁移操作的 PostgreSQL Advisory Lock ID。
// 在多实例部署场景下，该锁确保同一时间只有一个实例执行迁移。
// 任何稳定的 int64 值都可以，只要不与同一数据库中的其他锁冲突即可。
const migrationsAdvisoryLockID int64 = 694208311321144027
const migrationsLockRetryInterval = 500 * time.Millisecond
const nonTransactionalMigrationSuffix = "_notx.sql"
const paymentOrdersOutTradeNoUniqueMigration = "120_enforce_payment_orders_out_trade_no_unique_notx.sql"
const paymentOrdersOutTradeNoUniqueIndex = "paymentorder_out_trade_no_unique"
const subscriptionFulfillmentClaimUniqueMigration = "138_subscription_fulfillment_claim_unique_notx.sql"
const subscriptionFulfillmentClaimUniqueIndex = "idx_payment_audit_logs_order_action_uniq"
const affiliateLedgerOrderActionUniqueIndex = "idx_user_affiliate_ledger_order_action_unique"
const affiliateSignupBonusOnceIndex = "idx_user_affiliate_signup_bonus_once"
const accountAutopauseExpiryIndexMigration = "151_account_autopause_expiry_index_notx.sql"
const accountAutopauseExpiryIndex = "idx_accounts_autopause_expiry_due"
const schedulerOutboxPendingDedupKeyMigration = "153_scheduler_outbox_pending_dedup_key_index_notx.sql"
const schedulerOutboxPendingDedupKeyIndex = "idx_scheduler_outbox_pending_dedup_key"
const latestAPIKeyIPIndexMigration = "202_add_usage_logs_api_key_latest_ip_index_notx.sql"
const latestAPIKeyIPIndex = "idx_usage_logs_api_key_latest_ip"
const invoiceOrderActiveUniqueMigration = "195a_add_invoice_order_active_unique_guard_notx.sql"
const invoiceOrderActiveUniqueIndex = "invoiceorder_order_id_active_unique"
const batchImageIdempotencyUniqueMigration = "199a_batch_image_idempotency_unique_notx.sql"
const batchImageIdempotencyUniqueIndex = "batch_image_jobs_idempotency_owner_uq"

type migrationChecksumCompatibilityRule struct {
	fileChecksum       string
	acceptedDBChecksum map[string]struct{}
	acceptedChecksums  map[string]struct{}
}

// migrationChecksumCompatibilityRules 仅用于兼容历史上误修改过的迁移文件 checksum。
// 规则必须同时匹配「迁移名 + 数据库 checksum + 当前文件 checksum」且两者都落在该迁移的已知版本集合内才会放行，
// 避免放宽全局校验，也允许将误改的历史 migration 回滚为已发布版本而不要求人工修 checksum。
var migrationChecksumCompatibilityRules = map[string]migrationChecksumCompatibilityRule{
	"054_drop_legacy_cache_columns.sql":                       newMigrationChecksumCompatibilityRule("82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d", "182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4"),
	"061_add_usage_log_request_type.sql":                      newMigrationChecksumCompatibilityRule("66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c", "08a248652cbab7cfde147fc6ef8cda464f2477674e20b718312faa252e0481c0", "222b4a09c797c22e5922b6b172327c824f5463aaa8760e4f621bc5c22e2be0f3"),
	"109_auth_identity_compat_backfill.sql":                   newMigrationChecksumCompatibilityRule("0580b4602d85435edf9aca1633db580bb3932f26517f75134106f80275ec2ace", "551e498aa5616d2d91096e9d72cf9fb36e418ee22eacc557f8811cadbc9e20ee"),
	"110_pending_auth_and_provider_default_grants.sql":        newMigrationChecksumCompatibilityRule("32cf87ee787b1bb36b5c691367c96eee37518fa3eed6f3322cf68795e3745279", "e3d1f433be2b564cfbdc549adf98fce13c5c7b363ebc20fd05b765d0563b0925"),
	"112_add_payment_order_provider_key_snapshot.sql":         newMigrationChecksumCompatibilityRule("b75f8f56d39455682787696a3d92ad25b055444ca328fb7fca9a460a15d68d99", "ffd3e8a2c9295fa9cbefefd629a78268877e5b51bc970a82d9b3f46ec4ebd15e"),
	"115_auth_identity_legacy_external_backfill.sql":          newMigrationChecksumCompatibilityRule("022aadd97bb53e755f0cf7a3a957e0cb1a1353b0c39ec4de3234acd2871fd04f", "4cf39e508be9fd1a5aa41610cbbebeb80385c9adda45bf78a706de9db4f1385f"),
	"116_auth_identity_legacy_external_safety_reports.sql":    newMigrationChecksumCompatibilityRule("07edb09fa8d04ffb172b0621e3c22f4d1757d20a24ae267b3b36b087ab72d488", "f7757bd929ac67ffb08ce69fa4cf20fad39dbff9d5a5085fb2adabb7607e5877"),
	"118_wechat_dual_mode_and_auth_source_defaults.sql":       newMigrationChecksumCompatibilityRule("b54194d7a3e4fbf710e0a3590d22a2fe7966804c487052a356e0b55f53ef96b0", "e0cdf835d6c688d64100f483d31bc02ac9ebad414bf1837af239a84bf75b8227", "a38243ca0a72c3a01c0a92b7986423054d6133c0399441f853b99802852720fb"),
	"119_enforce_payment_orders_out_trade_no_unique.sql":      newMigrationChecksumCompatibilityRule("0bbe809ae48a9d811dabda1ba1c74955bd71c4a9cc610f9128816818dfa6c11e", "ebd2c67cce0116393fb4f1b5d5116a67c6aceb73820dfb5133d1ff6f36d72d34"),
	"120_enforce_payment_orders_out_trade_no_unique_notx.sql": newMigrationChecksumCompatibilityRule("34aadc0db59a4e390f92a12b73bd74642d9724f33124f73638ae00089ea5e074", "e77921f79d539bc24575cb9c16cbe566d2b23ce816190343d0a7568f6a3fcf61", "707431450603e70a43ce9fbd61e0c12fa67da4875158ccefabacea069587ab22", "04b082b5a239c525154fe9185d324ee2b05ff90da9297e10dba19f9be79aa59a"),
	"123_fix_legacy_auth_source_grant_on_signup_defaults.sql": newMigrationChecksumCompatibilityRule("2ce43c2cd89e9f9e1febd34a407ed9e84d177386c5544b6f02c1f58a21129f57", "6cd33422f215dcd1f486ab6f35c0ea5805d9ca69bb25906d94bc649156657145"),
	"125_add_channel_monitors.sql":                            newMigrationChecksumCompatibilityRule("a73ccde9efd29a767f91173d1a8bc6ffc929dd971e0c185da85b26a03be5cddd", "d49e8d1024ee1912ead12487d997a146943c6da86400f2fa443bb16eb01aac59"),
	"126_add_channel_monitor_aggregation.sql":                 newMigrationChecksumCompatibilityRule("88b9b7b70a823173a721ca5ed0d703891800d4f3f2fdcddaa4445b129e2c0146", "a631f4adc0fe0b9f4665805fe3d708942cf18e30e74a3061073cc86febfd3f69"),
	"131_affiliate_rebate_hardening.sql":                      newMigrationChecksumCompatibilityRule("7c612c5ad546c8b0c8ff23a1b3238066dd75e2f75893584d5981a84436eb36ff", "d40933ed0257fb7355a541c5dfd65b3a4487fa587fe76dc18625793b96804b36", "00b2290e6646666df46409564b545b1de91b222db8f3ca0c992164a6dc4034e7", "9fd0a6021290b24c7e76d4ff6405824eef528a6afe969e845c8cc3bf8053ba15", "706c8102d96d0a10f2e2a23156a8cd8b414a241591fd65ab3e26425b2a54fe29", "c4b74b9dd08e3634ac9b752376e92ce41f27fa0cb8046d7932944ca61e5f351c", "b20a2678be74db6a5a9a376004f4bf5bc7844ab46ee2f1e09194e8b1c48d49fd", "da8f7e442df20609449c51b13c250a2f79d3bb95c50f1c11b96b8108e5dddb02"),
	"132_affiliate_policy_limits.sql":                         newMigrationChecksumCompatibilityRule("1f98490f748148b96420b0bd0840fd8bfe3e0278e254f01ed0982a00eeb69b14", "ed8d931267f0dd1fb4e6f4abd7f7a5171df61760b4ffdab18b40bfbeff6fdd0f", "f8104c1e67f4e56e34c59a5831d1a3a75efffb3f8c3183d1910f7ec0555c4994", "1b06272a1b5ed48a0cd4aaef5abf2ef098232cf011f49d309d586acb31b687b7", "51f95d399e30dc499e9d1bc3bdefc5a7f5b358726ac83242ec64a363a6bfe092"),
	"137_subscription_fulfillment_claim_dedupe.sql":           newMigrationChecksumCompatibilityRule("882f6362973892afeafc640196fe10d216f066d0205ffa788ef82e3289422655", "1e4fb3a58b36572fd37a86c1a363f9d1458887daf8dc1b55c0b98f6b136bd50c"),
	"138_subscription_fulfillment_claim_unique_notx.sql":      newMigrationChecksumCompatibilityRule("23a0c91410feb00cb45231181811b35c6bbcef77768986b61444c07cb78c8df4", "fcdbbbcfa9010f6b2b0e9b6210a63d103eec5081358e70f591dec8a818c93009"),
	// 148/169/176/185 均在 backup 前首次落地（已部署环境记录旧 checksum），273032e08 修改了文件内容，
	// 无兼容规则会在存量库启动 ApplyMigrations 时 checksum mismatch 硬失败。补 旧→新 两版兼容，放行升级。
	"148_expand_usage_log_request_type_check.sql":                             newMigrationChecksumCompatibilityRule("eeb7d72ab005f1a78adb32207f170ba8f1383f7aa362b5597192da551cb58b98", "84cabecdfbf6d3bd471e2f420469545d35dd8504180b23c16261e25007634c41"),
	"151_apply_rpm_parallel_constraints_and_replace_claude_code_template.sql": newMigrationChecksumCompatibilityRule("9d59ab5830cd2e6f2f6acbb2014020e7698a7352ccda2af9aa9bf1a31030fc87", "cecde146e195c8099884db447eefc55da25c0c7a32ab0b0b79e2558c6154503d", "0fe932f50177afc05846b8489715030518659ef97c7f4ac76a381b2c57d26199"),
	"156_user_platform_quotas_add_kiro.sql":                                   newMigrationChecksumCompatibilityRule("e52d977e3b6daf7fde6b1acdcbd90c06826acaa5eeca825cd87b20931e4d49fb", "7b92806712dbd12b5bd2af0f582a76a0b8f3503f91b92b4efab361e5c53a3105"),
	"169_align_group_display_name_length.sql":                                 newMigrationChecksumCompatibilityRule("f018d1b5507a042dd25bbfb3387d7dd9698e805f70a7c6ca866d6edca9f6a00c", "fd9c23ed850576acc68daff7ff38a2ce9b2aab4f3f3468f70e0874b66cdd3d98"),
	"181_user_platform_quotas_add_grok.sql":                                   newMigrationChecksumCompatibilityRule("ebb03b1881d3c87266e652a04e6c4a516d938038aaecc29ef1b5cae255673944", "4fc04d9a7550719aa672d4d4377eb099f37edfa688ea4caeafc8f3a77ef86db7"),
	"176_add_tls_fingerprint_profile_transport.sql":                           newMigrationChecksumCompatibilityRule("dda41c69e1a76010d35c2b4aeeeb640a0d0c533ce54887a9f2fdda14f5fa0c90", "a7a4de298c589f32d3bf5badfd6ce524762929a25b525a2f94af71cf00c88a6c"),
	"176_tls_fingerprint_capture_unification.sql":                             newMigrationChecksumCompatibilityRule("8a9503c8e3c26f2eb3a9ae5bfdc1acca0a17253d92df82db1f362f05fcd8e071", "bf4072bb168f5a2af0eb33c46cf69ff4410d7c3bccaf7c6b1906e7e1ee006951"),
	"184_add_group_audio_search_pricing.sql":                                  newMigrationChecksumCompatibilityRule("7eeb54c89f2eaae6da3ef59f67b9f421cb272a335bd6985a26f6e602ff8c8525", "56b65dbc1dc1927c193b5b5bf74cf458a20070d5c12c23a28f95933f87d72ae9"),
	"185_expand_usage_log_request_type_check.sql":                             newMigrationChecksumCompatibilityRule("7bd5bbbc8ff72472958f6b86d6e02078409d65590057c5770afa8e890a930471", "641c9dd756f0174ac42741a810086971b9fd2cee631089a176d01db1273d851f"),
	"187_allow_native_image_route_and_video_price_checks.sql":                 newMigrationChecksumCompatibilityRule("95b068111a938093d2dd0db20d9695c45231edf62b0fde2a671ccda1b399e988", "547c29809e0c72d00b9aebbbd792e718e677a4906e8e6cd744031aa15a021c14"),
	"188_add_group_audio_search_price_checks.sql":                             newMigrationChecksumCompatibilityRule("483bb83fed29a0f571e5c8278bf8c070fdc8094c26170b7971d5d6ba521046bd", "e6cf495ea038076bc1f9d03b36eec81f6e517cfb81dbf346a4ef3d802f7e0be0"),
	"195_add_invoice_order_active_unique_guard.sql":                           newMigrationChecksumCompatibilityRule("ddeda6f9fcf3063d9e7fcefbd308e9bc8539a938baa3ca784a30970acbccbe92", "a775587a040b17780ed37e5fe8efcf223de52263f1f75a5bb4b7c1f6af0b7a99", "c5e9a08d3cd3370379b5e7be71692a286c75e5a55314d6eb660540c711e0b770"),
	"199_batch_image_idempotency_unique.sql":                                  newMigrationChecksumCompatibilityRule("522656bdbe527d30332718d52c51770085f32b842f5a4a7d18e755c9921b3598", "7b75bba89c5e33f996a6bdde61bec4751daf6c410cd9259e63a5b71829fe898b"),
}

// ApplyMigrations 将嵌入的 SQL 迁移文件应用到指定的数据库。
//
// 该函数可以在每次应用启动时安全调用：
// - 已应用的迁移会被自动跳过（通过校验 filename 判断）
// - 如果迁移文件内容被修改（checksum 不匹配），会返回错误
// - 使用 PostgreSQL Advisory Lock 确保多实例并发安全
//
// 参数：
//   - ctx: 上下文，用于超时控制和取消
//   - db: 数据库连接
//
// 返回：
//   - error: 迁移过程中的任何错误
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("nil sql db")
	}
	return applyMigrationsFS(ctx, db, migrations.FS)
}

// applyMigrationsFS 是迁移执行的核心实现。
// 它从指定的文件系统读取 SQL 迁移文件并按顺序应用。
//
// 迁移执行流程：
//  1. 获取 PostgreSQL Advisory Lock，防止多实例并发迁移
//  2. 确保 schema_migrations 表存在
//  3. 按文件名排序读取所有 .sql 文件
//  4. 对于每个迁移文件：
//     - 计算文件内容的 SHA256 校验和
//     - 检查该迁移是否已应用（通过 filename 查询）
//     - 如果已应用，验证校验和是否匹配
//     - 如果未应用，在事务中执行迁移并记录
//  5. 释放 Advisory Lock
//
// 参数：
//   - ctx: 上下文
//   - db: 数据库连接
//   - fsys: 包含迁移文件的文件系统（通常是 embed.FS）
func applyMigrationsFS(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	if db == nil {
		return errors.New("nil sql db")
	}

	// 获取分布式锁，确保多实例部署时只有一个实例执行迁移。
	// 这是 PostgreSQL 特有的 Advisory Lock 机制。
	if err := pgAdvisoryLock(ctx, db); err != nil {
		return err
	}
	defer func() {
		// 无论迁移是否成功，都要释放锁。
		// 使用 context.Background() 确保即使原 ctx 已取消也能释放锁。
		_ = pgAdvisoryUnlock(context.Background(), db)
	}()

	// 创建迁移记录表（如果不存在）。
	// 该表记录所有已应用的迁移及其校验和。
	if _, err := db.ExecContext(ctx, schemaMigrationsTableDDL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// 获取所有 .sql 迁移文件并按文件名排序。
	// 命名规范：使用零填充数字前缀（如 001_init.sql, 002_add_users.sql）。
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(files) // 确保按文件名顺序执行迁移

	for _, name := range files {
		// 读取迁移文件内容
		contentBytes, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		content := strings.TrimSpace(string(contentBytes))
		if content == "" {
			continue // 跳过空文件
		}

		// 计算文件内容的 SHA256 校验和，用于检测文件是否被修改。
		// 这是一种防篡改机制：如果有人修改了已应用的迁移文件，系统会拒绝启动。
		sum := sha256.Sum256([]byte(content))
		checksum := hex.EncodeToString(sum[:])

		// 检查该迁移是否已经应用
		var existing string
		rowErr := db.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE filename = $1", name).Scan(&existing)
		if rowErr == nil {
			// 迁移已应用，验证校验和是否匹配
			if existing != checksum {
				// 兼容特定历史误改场景（仅白名单规则），其余仍保持严格不可变约束。
				if isMigrationChecksumCompatible(name, existing, checksum) {
					continue
				}
				// 校验和不匹配意味着迁移文件在应用后被修改，这是危险的。
				// 正确的做法是创建新的迁移文件来进行变更。
				return fmt.Errorf(
					"migration %s checksum mismatch (db=%s file=%s)\n"+
						"This means the migration file was modified after being applied to the database.\n"+
						"Solutions:\n"+
						"  1. Revert to original: git log --oneline -- migrations/%s && git checkout <commit> -- migrations/%s\n"+
						"  2. For new changes, create a new migration file instead of modifying existing ones\n"+
						"Note: Modifying applied migrations breaks the immutability principle and can cause inconsistencies across environments",
					name, existing, checksum, name, name,
				)
			}
			continue // 迁移已应用且校验和匹配，跳过
		}
		if !errors.Is(rowErr, sql.ErrNoRows) {
			return fmt.Errorf("check migration %s: %w", name, rowErr)
		}

		nonTx, err := validateMigrationExecutionMode(name, content)
		if err != nil {
			return fmt.Errorf("validate migration %s: %w", name, err)
		}

		if nonTx {
			if err := prepareNonTransactionalMigration(ctx, db, name); err != nil {
				return fmt.Errorf("prepare migration %s: %w", name, err)
			}

			// *_notx.sql：用于 CREATE/DROP INDEX CONCURRENTLY 场景，必须非事务执行。
			// 逐条语句执行，避免将多条 CONCURRENTLY 语句放入同一个隐式事务块。
			statements := splitSQLStatements(content)
			for i, stmt := range statements {
				trimmed := strings.TrimSpace(stmt)
				if trimmed == "" {
					continue
				}
				if stripSQLLineComment(trimmed) == "" {
					continue
				}
				if _, err := db.ExecContext(ctx, trimmed); err != nil {
					return fmt.Errorf("apply migration %s (non-tx statement %d): %w", name, i+1, err)
				}
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", name, checksum); err != nil {
				return fmt.Errorf("record migration %s (non-tx): %w", name, err)
			}
			continue
		}

		// 默认迁移在事务中执行，确保原子性：要么完全成功，要么完全回滚。
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}

		// 执行迁移 SQL
		if _, err := tx.ExecContext(ctx, content); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		// 记录迁移已完成，保存文件名和校验和
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", name, checksum); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		// 提交事务
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	// 只有在 legacy SQL 迁移全部成功后，才为 Atlas 补齐基线，避免失败时错误地将 Atlas 固定到 HEAD。
	if err := ensureAtlasBaselineAligned(ctx, db); err != nil {
		return err
	}

	return nil
}

func prepareNonTransactionalMigration(ctx context.Context, db *sql.DB, name string) error {
	switch name {
	case paymentOrdersOutTradeNoUniqueMigration:
		return preparePaymentOrdersOutTradeNoUniqueMigration(ctx, db)
	case subscriptionFulfillmentClaimUniqueMigration:
		return prepareSubscriptionFulfillmentClaimUniqueMigration(ctx, db)
	case accountAutopauseExpiryIndexMigration:
		return prepareInvalidIndexRetry(ctx, db, accountAutopauseExpiryIndex)
	case schedulerOutboxPendingDedupKeyMigration:
		return prepareInvalidIndexRetry(ctx, db, schedulerOutboxPendingDedupKeyIndex)
	case latestAPIKeyIPIndexMigration:
		return prepareInvalidIndexRetry(ctx, db, latestAPIKeyIPIndex)
	case invoiceOrderActiveUniqueMigration:
		// Drop INVALID partial unique left by a failed CREATE INDEX CONCURRENTLY
		// so IF NOT EXISTS does not no-op past a non-enforcing index.
		return prepareInvalidIndexRetry(ctx, db, invoiceOrderActiveUniqueIndex)
	case batchImageIdempotencyUniqueMigration:
		return prepareInvalidIndexRetry(ctx, db, batchImageIdempotencyUniqueIndex)
	default:
		return nil
	}
}

func preparePaymentOrdersOutTradeNoUniqueMigration(ctx context.Context, db *sql.DB) error {
	duplicates, err := findDuplicatePaymentOrderOutTradeNos(ctx, db)
	if err != nil {
		return fmt.Errorf("precheck duplicate out_trade_no: %w", err)
	}
	if len(duplicates) > 0 {
		return fmt.Errorf(
			"duplicate out_trade_no values block %s; remediate duplicates before retrying: %s",
			paymentOrdersOutTradeNoUniqueMigration,
			strings.Join(duplicates, ", "),
		)
	}

	return prepareInvalidIndexRetry(ctx, db, paymentOrdersOutTradeNoUniqueIndex)
}

func prepareSubscriptionFulfillmentClaimUniqueMigration(ctx context.Context, db *sql.DB) error {
	auditDuplicates, err := findDuplicatePaymentAuditLogOrderActions(ctx, db)
	if err != nil {
		return fmt.Errorf("precheck duplicate payment_audit_logs order_id/action: %w", err)
	}
	if len(auditDuplicates) > 0 {
		return fmt.Errorf(
			"duplicate payment_audit_logs rows block %s; remediate duplicate order_id/action rows before retrying: %s",
			subscriptionFulfillmentClaimUniqueMigration,
			strings.Join(auditDuplicates, ", "),
		)
	}

	ledgerDuplicates, err := findDuplicateAffiliateLedgerOrderActions(ctx, db)
	if err != nil {
		return fmt.Errorf("precheck duplicate user_affiliate_ledger user_id/source_order_id/action: %w", err)
	}
	if len(ledgerDuplicates) > 0 {
		return fmt.Errorf(
			"duplicate user_affiliate_ledger accrue rows block %s; remediate duplicate user_id/source_order_id/action rows before retrying: %s",
			subscriptionFulfillmentClaimUniqueMigration,
			strings.Join(ledgerDuplicates, ", "),
		)
	}

	signupBonusDuplicates, err := findDuplicateAffiliateSignupBonusRows(ctx, db)
	if err != nil {
		return fmt.Errorf("precheck duplicate user_affiliate_ledger user_id/action: %w", err)
	}
	if len(signupBonusDuplicates) > 0 {
		return fmt.Errorf(
			"duplicate user_affiliate_ledger signup_bonus rows block %s; remediate duplicate user_id/action rows before retrying: %s",
			subscriptionFulfillmentClaimUniqueMigration,
			strings.Join(signupBonusDuplicates, ", "),
		)
	}

	for _, indexName := range []string{
		subscriptionFulfillmentClaimUniqueIndex,
		affiliateLedgerOrderActionUniqueIndex,
		affiliateSignupBonusOnceIndex,
	} {
		if err := prepareInvalidIndexRetry(ctx, db, indexName); err != nil {
			return err
		}
	}
	return nil
}

func prepareInvalidIndexRetry(ctx context.Context, db *sql.DB, indexName string) error {
	invalid, err := indexIsInvalid(ctx, db, indexName)
	if err != nil {
		return fmt.Errorf("check invalid index %s: %w", indexName, err)
	}
	if !invalid {
		return nil
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf("DROP INDEX CONCURRENTLY IF EXISTS %s", indexName)); err != nil {
		return fmt.Errorf("drop invalid index %s: %w", indexName, err)
	}
	return nil
}

func findDuplicatePaymentOrderOutTradeNos(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT out_trade_no, COUNT(*) AS duplicate_count
		FROM payment_orders
		WHERE out_trade_no <> ''
		GROUP BY out_trade_no
		HAVING COUNT(*) > 1
		ORDER BY duplicate_count DESC, out_trade_no
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	duplicates := make([]string, 0, 5)
	for rows.Next() {
		var outTradeNo string
		var duplicateCount int
		if err := rows.Scan(&outTradeNo, &duplicateCount); err != nil {
			return nil, err
		}
		duplicates = append(duplicates, fmt.Sprintf("%s (count=%d)", outTradeNo, duplicateCount))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return duplicates, nil
}

func findDuplicatePaymentAuditLogOrderActions(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT order_id, action, COUNT(*) AS duplicate_count
		FROM payment_audit_logs
		WHERE action IN (
			'AFFILIATE_REBATE_APPLIED',
			'AFFILIATE_REBATE_SKIPPED',
			'SUBSCRIPTION_FULFILLMENT_CLAIMED',
			'SUBSCRIPTION_SUCCESS'
		)
		GROUP BY order_id, action
		HAVING COUNT(*) > 1
		ORDER BY duplicate_count DESC, order_id, action
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	duplicates := make([]string, 0, 5)
	for rows.Next() {
		var (
			orderID        string
			action         string
			duplicateCount int
		)
		if err := rows.Scan(&orderID, &action, &duplicateCount); err != nil {
			return nil, err
		}
		duplicates = append(duplicates, fmt.Sprintf("%s/%s (count=%d)", orderID, action, duplicateCount))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return duplicates, nil
}

func findDuplicateAffiliateLedgerOrderActions(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT user_id, source_order_id, action, COUNT(*) AS duplicate_count
		FROM user_affiliate_ledger
		WHERE source_order_id IS NOT NULL
		  AND action = 'accrue'
		GROUP BY user_id, source_order_id, action
		HAVING COUNT(*) > 1
		ORDER BY duplicate_count DESC, user_id, source_order_id, action
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	duplicates := make([]string, 0, 5)
	for rows.Next() {
		var (
			userID         int64
			sourceOrderID  int64
			action         string
			duplicateCount int
		)
		if err := rows.Scan(&userID, &sourceOrderID, &action, &duplicateCount); err != nil {
			return nil, err
		}
		duplicates = append(duplicates, fmt.Sprintf("user_id=%d/source_order_id=%d/action=%s (count=%d)", userID, sourceOrderID, action, duplicateCount))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return duplicates, nil
}

func findDuplicateAffiliateSignupBonusRows(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT user_id, action, COUNT(*) AS duplicate_count
		FROM user_affiliate_ledger
		WHERE action = 'signup_bonus'
		GROUP BY user_id, action
		HAVING COUNT(*) > 1
		ORDER BY duplicate_count DESC, user_id, action
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	duplicates := make([]string, 0, 5)
	for rows.Next() {
		var (
			userID         int64
			action         string
			duplicateCount int
		)
		if err := rows.Scan(&userID, &action, &duplicateCount); err != nil {
			return nil, err
		}
		duplicates = append(duplicates, fmt.Sprintf("user_id=%d/action=%s (count=%d)", userID, action, duplicateCount))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return duplicates, nil
}

func indexIsInvalid(ctx context.Context, db *sql.DB, indexName string) (bool, error) {
	var invalid bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_class idx
			JOIN pg_namespace ns ON ns.oid = idx.relnamespace
			JOIN pg_index i ON i.indexrelid = idx.oid
			WHERE ns.nspname = 'public'
			  AND idx.relname = $1
			  AND NOT i.indisvalid
		)
	`, indexName).Scan(&invalid)
	return invalid, err
}

func ensureAtlasBaselineAligned(ctx context.Context, db *sql.DB) error {
	hasLegacy, err := tableExists(ctx, db, "schema_migrations")
	if err != nil {
		return fmt.Errorf("check schema_migrations: %w", err)
	}
	if !hasLegacy {
		return nil
	}

	hasAtlas, err := tableExists(ctx, db, "atlas_schema_revisions")
	if err != nil {
		return fmt.Errorf("check atlas_schema_revisions: %w", err)
	}
	if !hasAtlas {
		if _, err := db.ExecContext(ctx, atlasSchemaRevisionsTableDDL); err != nil {
			return fmt.Errorf("create atlas_schema_revisions: %w", err)
		}
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM atlas_schema_revisions").Scan(&count); err != nil {
		return fmt.Errorf("count atlas_schema_revisions: %w", err)
	}
	if count > 0 {
		return nil
	}

	version, description, hash, err := latestAppliedMigrationBaseline(ctx, db)
	if err != nil {
		return fmt.Errorf("atlas baseline version: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO atlas_schema_revisions (version, description, type, applied, total, executed_at, execution_time, hash)
		VALUES ($1, $2, $3, 0, 0, NOW(), 0, $4)
	`, version, description, 1, hash); err != nil {
		return fmt.Errorf("insert atlas baseline: %w", err)
	}
	return nil
}

func tableExists(ctx context.Context, db *sql.DB, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, tableName).Scan(&exists)
	return exists, err
}

func latestAppliedMigrationBaseline(ctx context.Context, db *sql.DB) (string, string, string, error) {
	var (
		filename string
		checksum string
	)
	err := db.QueryRowContext(ctx, `
		SELECT filename, checksum
		FROM schema_migrations
		ORDER BY filename DESC
		LIMIT 1
	`).Scan(&filename, &checksum)
	if errors.Is(err, sql.ErrNoRows) {
		return "baseline", "baseline", "", nil
	}
	if err != nil {
		return "", "", "", err
	}

	version := strings.TrimSuffix(filename, ".sql")
	return version, version, checksum, nil
}

func checksumSet(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func newMigrationChecksumCompatibilityRule(fileChecksum string, acceptedDBChecksums ...string) migrationChecksumCompatibilityRule {
	return migrationChecksumCompatibilityRule{
		fileChecksum:       fileChecksum,
		acceptedDBChecksum: checksumSet(acceptedDBChecksums...),
		acceptedChecksums:  checksumSet(append([]string{fileChecksum}, acceptedDBChecksums...)...),
	}
}

func isMigrationChecksumCompatible(name, dbChecksum, fileChecksum string) bool {
	rule, ok := migrationChecksumCompatibilityRules[name]
	if !ok {
		return false
	}
	_, dbOK := rule.acceptedChecksums[dbChecksum]
	if !dbOK {
		return false
	}
	_, fileOK := rule.acceptedChecksums[fileChecksum]
	return fileOK
}

// IsMigrationChecksumCompatible reports whether an exact database/file checksum
// pair is a known historical migration edit. Deploy tooling uses the same
// rules as startup validation so checksum rewrites cannot broaden compatibility.
func IsMigrationChecksumCompatible(name, dbChecksum, fileChecksum string) bool {
	return isMigrationChecksumCompatible(name, dbChecksum, fileChecksum)
}

func validateMigrationExecutionMode(name, content string) (bool, error) {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	upperContent := strings.ToUpper(stripSQLFullLineComments(content))
	nonTx := strings.HasSuffix(normalizedName, nonTransactionalMigrationSuffix)

	if !nonTx {
		if strings.Contains(upperContent, "CONCURRENTLY") {
			return false, errors.New("CONCURRENTLY statements must be placed in *_notx.sql migrations")
		}
		return false, nil
	}

	if strings.Contains(upperContent, "BEGIN") || strings.Contains(upperContent, "COMMIT") || strings.Contains(upperContent, "ROLLBACK") {
		return false, errors.New("*_notx.sql must not contain transaction control statements (BEGIN/COMMIT/ROLLBACK)")
	}

	statements := splitSQLStatements(content)
	for _, stmt := range statements {
		normalizedStmt := strings.ToUpper(stripSQLLineComment(strings.TrimSpace(stmt)))
		if normalizedStmt == "" {
			continue
		}

		if strings.Contains(normalizedStmt, "CONCURRENTLY") {
			isCreateIndex := strings.Contains(normalizedStmt, "CREATE") && strings.Contains(normalizedStmt, "INDEX")
			isDropIndex := strings.Contains(normalizedStmt, "DROP") && strings.Contains(normalizedStmt, "INDEX")
			if !isCreateIndex && !isDropIndex {
				return false, errors.New("*_notx.sql currently only supports CREATE/DROP INDEX CONCURRENTLY statements")
			}
			if isCreateIndex && !strings.Contains(normalizedStmt, "IF NOT EXISTS") {
				return false, errors.New("CREATE INDEX CONCURRENTLY in *_notx.sql must include IF NOT EXISTS for idempotency")
			}
			if isDropIndex && !strings.Contains(normalizedStmt, "IF EXISTS") {
				return false, errors.New("DROP INDEX CONCURRENTLY in *_notx.sql must include IF EXISTS for idempotency")
			}
			continue
		}

		return false, errors.New("*_notx.sql must not mix non-CONCURRENTLY SQL statements")
	}

	return true, nil
}

func splitSQLStatements(content string) []string {
	parts := strings.Split(content, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func stripSQLLineComment(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func stripSQLFullLineComments(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines[i] = ""
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// pgAdvisoryLock 获取 PostgreSQL Advisory Lock。
// Advisory Lock 是一种轻量级的锁机制，不与任何特定的数据库对象关联。
// 它非常适合用于应用层面的分布式锁场景，如迁移序列化。
func pgAdvisoryLock(ctx context.Context, db *sql.DB) error {
	ticker := time.NewTicker(migrationsLockRetryInterval)
	defer ticker.Stop()

	for {
		var locked bool
		if err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationsAdvisoryLockID).Scan(&locked); err != nil {
			return fmt.Errorf("acquire migrations lock: %w", err)
		}
		if locked {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquire migrations lock: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// pgAdvisoryUnlock 释放 PostgreSQL Advisory Lock。
// 必须在获取锁后确保释放，否则会阻塞其他实例的迁移操作。
func pgAdvisoryUnlock(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationsAdvisoryLockID)
	if err != nil {
		return fmt.Errorf("release migrations lock: %w", err)
	}
	return nil
}
