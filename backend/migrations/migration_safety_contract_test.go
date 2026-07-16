package migrations

import (
	"io/fs"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageLogsCheckConstraintsOnExistingTableAreAddedNotValid(t *testing.T) {
	files, err := fs.Glob(FS, "*.sql")
	require.NoError(t, err)

	legacyExceptions := map[string]string{
		"061_add_usage_log_request_type.sql": "legacy migration predates the hot-table NOT VALID contract",
	}

	for _, file := range files {
		content, err := FS.ReadFile(file)
		require.NoError(t, err)

		statements := strings.Split(normalizeMigrationSQLForSafetyTest(string(content)), ";")
		for _, statement := range statements {
			if !strings.Contains(statement, "ALTER TABLE USAGE_LOGS") ||
				!strings.Contains(statement, "ADD CONSTRAINT") ||
				!strings.Contains(statement, "CHECK") {
				continue
			}
			if _, ok := legacyExceptions[file]; ok {
				continue
			}

			require.Contains(
				t,
				statement,
				"NOT VALID",
				"%s adds a usage_logs CHECK constraint without NOT VALID: %s",
				file,
				statement,
			)
		}
	}
}

func TestUsageLogsRequestTypeCheckValidationRunsAfterNotValidAdd(t *testing.T) {
	tests := []struct {
		addMigration      string
		validateMigration string
		checkExpression   string
	}{
		{
			addMigration:      "148_expand_usage_log_request_type_check.sql",
			validateMigration: "148a_validate_usage_log_request_type_check.sql",
			checkExpression:   "CHECK (REQUEST_TYPE IN (0, 1, 2, 3, 4, 5)) NOT VALID",
		},
		{
			addMigration:      "185_expand_usage_log_request_type_check.sql",
			validateMigration: "185a_validate_usage_log_request_type_check.sql",
			checkExpression:   "CHECK (REQUEST_TYPE IN (0, 1, 2, 3, 4, 5, 6, 7)) NOT VALID",
		},
		{
			addMigration:      "191_restore_usage_request_type_cyber_value.sql",
			validateMigration: "191a_validate_usage_log_request_type_check.sql",
			checkExpression:   "CHECK (REQUEST_TYPE IN (0, 1, 2, 3, 4, 5, 6, 7, 8)) NOT VALID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.addMigration, func(t *testing.T) {
			addContent, err := FS.ReadFile(tt.addMigration)
			require.NoError(t, err)
			addSQL := normalizeMigrationSQLForSafetyTest(string(addContent))

			require.Contains(t, addSQL, "ADD CONSTRAINT USAGE_LOGS_REQUEST_TYPE_CHECK")
			require.Contains(t, addSQL, tt.checkExpression)
			require.NotContains(t, addSQL, "VALIDATE CONSTRAINT")

			validateContent, err := FS.ReadFile(tt.validateMigration)
			require.NoError(t, err)
			validateSQL := normalizeMigrationSQLForSafetyTest(string(validateContent))
			require.Contains(t, validateSQL, "ALTER TABLE USAGE_LOGS VALIDATE CONSTRAINT USAGE_LOGS_REQUEST_TYPE_CHECK")
		})
	}
}

func TestNoNewTransactionalHotTableBackfills(t *testing.T) {
	// Guardrail for future migrations only. 207/213 already shipped and are
	// documented as operational hazards; rewrite via offline jobs instead of
	// editing applied SQL.
	files, err := fs.Glob(FS, "*.sql")
	require.NoError(t, err)

	legacyHotTableBackfills := map[string]struct{}{
		"207_ip_multi_account_security.sql":          {},
		"213_backfill_usage_log_video_seconds.sql":   {},
		"191_restore_usage_request_type_cyber_value.sql": {},
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_notx.sql") {
			continue
		}
		if _, ok := legacyHotTableBackfills[file]; ok {
			continue
		}
		// Only enforce on migrations numbered >= 216 (post-review policy).
		base := strings.TrimSuffix(file, ".sql")
		numPart := strings.SplitN(base, "_", 2)[0]
		numPart = strings.TrimRight(numPart, "abcdefghijklmnopqrstuvwxyz")
		n, err := strconv.Atoi(numPart)
		if err != nil || n < 216 {
			continue
		}
		content, err := FS.ReadFile(file)
		require.NoError(t, err)
		upper := strings.ToUpper(string(content))
		if strings.Contains(upper, "UPDATE USAGE_LOGS") {
			require.Failf(t, "hot table backfill",
				"%s performs UPDATE usage_logs in a transactional startup migration; use an offline batched job or _notx path", file)
		}
		if strings.Contains(upper, "CREATE INDEX") && strings.Contains(upper, "ON USAGE_LOGS") &&
			!strings.Contains(upper, "CONCURRENTLY") {
			require.Failf(t, "hot table index",
				"%s creates a non-CONCURRENT index on usage_logs in a transactional migration; use *_notx.sql", file)
		}
	}
}

func TestHotTableCheckConstraintsAreAddedNotValid(t *testing.T) {
	tests := []struct {
		migration       string
		constraintName  string
		checkExpression string
	}{
		{
			migration:       "156_user_platform_quotas_add_kiro.sql",
			constraintName:  "USER_PLATFORM_QUOTAS_PLATFORM_CHECK",
			checkExpression: "CHECK (PLATFORM IN ('ANTHROPIC', 'OPENAI', 'GEMINI', 'ANTIGRAVITY', 'KIRO')) NOT VALID",
		},
		{
			migration:       "181_user_platform_quotas_add_grok.sql",
			constraintName:  "USER_PLATFORM_QUOTAS_PLATFORM_CHECK",
			checkExpression: "CHECK (PLATFORM IN ('ANTHROPIC', 'OPENAI', 'GEMINI', 'ANTIGRAVITY', 'KIRO', 'GROK')) NOT VALID",
		},
		{
			migration:       "187_allow_native_image_route_and_video_price_checks.sql",
			constraintName:  "GROUPS_IMAGE_GENERATION_ROUTE_CHECK",
			checkExpression: "CHECK (IMAGE_GENERATION_ROUTE IN ('CODEX', 'WEB2API', 'NATIVE')) NOT VALID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.migration, func(t *testing.T) {
			content, err := FS.ReadFile(tt.migration)
			require.NoError(t, err)

			sql := normalizeMigrationSQLForSafetyTest(string(content))
			require.Contains(t, sql, "ADD CONSTRAINT "+tt.constraintName)
			require.Contains(t, sql, tt.checkExpression)
		})
	}
}

func TestHotTableCheckConstraintValidationRunsInFollowupMigration(t *testing.T) {
	content, err := FS.ReadFile("196_validate_hot_table_check_constraints.sql")
	require.NoError(t, err)

	sql := normalizeMigrationSQLForSafetyTest(string(content))
	for _, statement := range []string{
		"ALTER TABLE USER_PLATFORM_QUOTAS VALIDATE CONSTRAINT USER_PLATFORM_QUOTAS_PLATFORM_CHECK",
		"ALTER TABLE GROUPS VALIDATE CONSTRAINT GROUPS_IMAGE_GENERATION_ROUTE_CHECK",
		"ALTER TABLE GROUPS VALIDATE CONSTRAINT GROUPS_VIDEO_PRICE_480P_PER_SEC_NON_NEGATIVE",
		"ALTER TABLE GROUPS VALIDATE CONSTRAINT GROUPS_VIDEO_PRICE_720P_PER_SEC_NON_NEGATIVE",
		"ALTER TABLE GROUPS VALIDATE CONSTRAINT GROUPS_VIDEO_PRICE_1080P_PER_SEC_NON_NEGATIVE",
		"ALTER TABLE GROUPS VALIDATE CONSTRAINT GROUPS_VIDEO_PRICE_4K_PER_SEC_NON_NEGATIVE",
	} {
		require.Contains(t, sql, statement)
	}
}

func TestPostReleaseNonNegativeChecksOnExistingTablesAreAddedNotValid(t *testing.T) {
	tests := []struct {
		migration       string
		constraintName  string
		checkExpression string
	}{
		{
			migration:       "151_apply_rpm_parallel_constraints_and_replace_claude_code_template.sql",
			constraintName:  "GROUPS_RPM_LIMIT_NON_NEGATIVE",
			checkExpression: "CHECK (RPM_LIMIT >= 0) NOT VALID",
		},
		{
			migration:       "151_apply_rpm_parallel_constraints_and_replace_claude_code_template.sql",
			constraintName:  "USERS_RPM_LIMIT_NON_NEGATIVE",
			checkExpression: "CHECK (RPM_LIMIT >= 0) NOT VALID",
		},
		{
			migration:       "151_apply_rpm_parallel_constraints_and_replace_claude_code_template.sql",
			constraintName:  "USER_GROUP_RATE_MULTIPLIERS_RPM_OVERRIDE_NON_NEGATIVE",
			checkExpression: "CHECK (RPM_OVERRIDE IS NULL OR RPM_OVERRIDE >= 0) NOT VALID",
		},
		{
			migration:       "188_add_group_audio_search_price_checks.sql",
			constraintName:  "GROUPS_SEARCH_PRICE_PER_1K_NON_NEGATIVE",
			checkExpression: "CHECK (SEARCH_PRICE_PER_1K IS NULL OR SEARCH_PRICE_PER_1K >= 0) NOT VALID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.migration+"_"+tt.constraintName, func(t *testing.T) {
			content, err := FS.ReadFile(tt.migration)
			require.NoError(t, err)

			sql := normalizeMigrationSQLForSafetyTest(string(content))
			require.Contains(t, sql, "ADD CONSTRAINT "+tt.constraintName)
			require.Contains(t, sql, tt.checkExpression)
		})
	}
}

func TestPostReleaseNonNegativeCheckValidationRunsInFollowupMigration(t *testing.T) {
	content, err := FS.ReadFile("197_validate_post_release_non_negative_checks.sql")
	require.NoError(t, err)

	sql := normalizeMigrationSQLForSafetyTest(string(content))
	for _, statement := range []string{
		"ALTER TABLE GROUPS VALIDATE CONSTRAINT GROUPS_RPM_LIMIT_NON_NEGATIVE",
		"ALTER TABLE USERS VALIDATE CONSTRAINT USERS_RPM_LIMIT_NON_NEGATIVE",
		"ALTER TABLE USER_GROUP_RATE_MULTIPLIERS VALIDATE CONSTRAINT USER_GROUP_RATE_MULTIPLIERS_RPM_OVERRIDE_NON_NEGATIVE",
		"ALTER TABLE GROUPS VALIDATE CONSTRAINT GROUPS_SEARCH_PRICE_PER_1K_NON_NEGATIVE",
		"ALTER TABLE GROUPS VALIDATE CONSTRAINT GROUPS_AUDIO_REALTIME_PRICE_PER_MIN_NON_NEGATIVE",
		"ALTER TABLE GROUPS VALIDATE CONSTRAINT GROUPS_AUDIO_TTS_PRICE_PER_MILLION_CHARS_NON_NEGATIVE",
		"ALTER TABLE GROUPS VALIDATE CONSTRAINT GROUPS_AUDIO_STT_PRICE_PER_HOUR_NON_NEGATIVE",
	} {
		require.Contains(t, sql, statement)
	}
}

func TestHotTableUniqueIndexesAreCreatedConcurrently(t *testing.T) {
	tests := []struct {
		transactionalMigration string
		notxMigration          string
		dropIndex              string
		createIndex            string
	}{
		{
			transactionalMigration: "195_add_invoice_order_active_unique_guard.sql",
			notxMigration:          "195a_add_invoice_order_active_unique_guard_notx.sql",
			// Create under a new unique name first; only drop the legacy non-unique
			// invoiceorder_order_id after uniqueness exists (create-first safety).
			dropIndex:   "DROP INDEX CONCURRENTLY IF EXISTS INVOICEORDER_ORDER_ID",
			createIndex: "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS INVOICEORDER_ORDER_ID_ACTIVE_UNIQUE",
		},
		{
			transactionalMigration: "199_batch_image_idempotency_unique.sql",
			notxMigration:          "199a_batch_image_idempotency_unique_notx.sql",
			dropIndex:              "DROP INDEX CONCURRENTLY IF EXISTS BATCH_IMAGE_JOBS_IDEMPOTENCY_KEY_IDX",
			createIndex:            "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS BATCH_IMAGE_JOBS_IDEMPOTENCY_OWNER_UQ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.transactionalMigration, func(t *testing.T) {
			content, err := FS.ReadFile(tt.transactionalMigration)
			require.NoError(t, err)
			sql := normalizeMigrationSQLForSafetyTest(string(content))
			require.NotContains(t, sql, "CREATE UNIQUE INDEX")
			require.NotContains(t, sql, "DROP INDEX")

			notxContent, err := FS.ReadFile(tt.notxMigration)
			require.NoError(t, err)
			notxSQL := normalizeMigrationSQLForSafetyTest(string(notxContent))
			require.Contains(t, notxSQL, tt.dropIndex)
			require.Contains(t, notxSQL, tt.createIndex)

			// Create-first: if CREATE fails mid-migration, the legacy index must
			// still exist. Dropping first would leave a uniqueness gap.
			createPos := strings.Index(notxSQL, tt.createIndex)
			dropPos := strings.Index(notxSQL, tt.dropIndex)
			require.NotEqual(t, -1, createPos, "create statement missing in %s", tt.notxMigration)
			require.NotEqual(t, -1, dropPos, "drop statement missing in %s", tt.notxMigration)
			require.Less(t, createPos, dropPos, "%s must CREATE unique index before DROP of legacy index", tt.notxMigration)
		})
	}
}

func TestGroupDisplayNameLengthMigrationFailsBeforeTruncating(t *testing.T) {
	content, err := FS.ReadFile("169_align_group_display_name_length.sql")
	require.NoError(t, err)

	rawSQL := string(content)
	sql := normalizeMigrationSQLForSafetyTest(rawSQL)
	require.NotContains(t, strings.ToUpper(rawSQL), "USING LEFT(DISPLAY_NAME")
	require.NotContains(t, strings.ToUpper(rawSQL), "LEFT(DISPLAY_NAME, 100)")
	require.Contains(t, sql, "RAISE EXCEPTION")
	require.Contains(t, sql, "CHAR_LENGTH(DISPLAY_NAME) > 100")
	require.Contains(t, sql, "ALTER COLUMN DISPLAY_NAME TYPE VARCHAR(100)")
}

func normalizeMigrationSQLForSafetyTest(sql string) string {
	return strings.ToUpper(strings.Join(strings.Fields(sql), " "))
}
