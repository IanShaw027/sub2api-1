//go:build unit

package repository

import (
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func TestTranslateChannelMonitorTemplateFKError(t *testing.T) {
	err := &pq.Error{Code: "23503", Constraint: "channel_monitors_template_id_fkey"}
	got := translateChannelMonitorTemplateFKError(err)
	if !errors.Is(got, service.ErrChannelMonitorTemplateNotFound) {
		t.Fatalf("expected ErrChannelMonitorTemplateNotFound, got %v", got)
	}
}

func TestTranslateChannelMonitorTemplateFKError_OtherConstraint(t *testing.T) {
	err := &pq.Error{Code: "23503", Constraint: "some_other_fkey"}
	if got := translateChannelMonitorTemplateFKError(err); got != nil {
		t.Fatalf("expected nil for unrelated FK error, got %v", got)
	}
}
