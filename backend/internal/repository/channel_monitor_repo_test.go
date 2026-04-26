//go:build unit

package repository

import (
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func TestTranslateChannelMonitorTemplateFKError(t *testing.T) {
	for _, constraint := range []string{
		"channel_monitors_template_id_fkey",
		"channel_monitors_channel_monitor_request_templates_request_template",
	} {
		err := &pq.Error{Code: "23503", Constraint: constraint}
		got := translateChannelMonitorTemplateFKError(err)
		if !errors.Is(got, service.ErrChannelMonitorTemplateNotFound) {
			t.Fatalf("expected ErrChannelMonitorTemplateNotFound for %s, got %v", constraint, got)
		}
	}
}

func TestTranslateChannelMonitorTemplateFKError_OtherConstraint(t *testing.T) {
	err := &pq.Error{Code: "23503", Constraint: "some_other_fkey"}
	if got := translateChannelMonitorTemplateFKError(err); got != nil {
		t.Fatalf("expected nil for unrelated FK error, got %v", got)
	}
}
