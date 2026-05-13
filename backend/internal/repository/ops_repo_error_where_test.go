package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBuildOpsErrorLogsWhere_QueryUsesQualifiedColumns(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		Query: "ACCESS_DENIED",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "e.request_id ILIKE $") {
		t.Fatalf("where should include qualified request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.client_request_id ILIKE $") {
		t.Fatalf("where should include qualified client_request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.error_message ILIKE $") {
		t.Fatalf("where should include qualified error_message condition: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_UserQueryUsesExistsSubquery(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		UserQuery: "admin@",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "EXISTS (SELECT 1 FROM users u WHERE u.id = e.user_id AND u.email ILIKE $") {
		t.Fatalf("where should include EXISTS user email condition: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_ViewTreats429AsExcluded(t *testing.T) {
	errorsWhere, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{View: "errors"})
	if !strings.Contains(errorsWhere, "NOT (COALESCE(e.is_business_limited,false) = true OR COALESCE(e.upstream_status_code, e.status_code, 0) IN (429, 529))") {
		t.Fatalf("errors view should exclude 429/529: %s", errorsWhere)
	}

	excludedWhere, _ := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{View: "excluded"})
	if !strings.Contains(excludedWhere, "(COALESCE(e.is_business_limited,false) = true OR COALESCE(e.upstream_status_code, e.status_code, 0) IN (429, 529))") {
		t.Fatalf("excluded view should include 429/529: %s", excludedWhere)
	}
}
