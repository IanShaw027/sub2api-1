package migrations

import (
	"io/fs"
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
