package repository

import (
	"strings"
	"testing"
)

func TestOpsErrorLogInsertDoesNotPersistRequestReplayFields(t *testing.T) {
	disallowedColumns := []string{
		"request_body",
		"request_headers",
		"request_body_truncated",
		"request_body_bytes",
		"is_retryable",
		"retry_count",
		"resolved_retry_id",
	}

	insertSQL := strings.ToLower(insertOpsErrorLogSQL)
	for _, column := range disallowedColumns {
		if strings.Contains(insertSQL, column) {
			t.Fatalf("ops error log insert still references dropped replay column %q", column)
		}
	}
}
