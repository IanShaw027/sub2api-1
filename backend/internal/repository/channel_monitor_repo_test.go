//go:build unit

package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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

func TestChannelMonitorRepositoryAdjustAvailability7d_RaisesAvailability(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()

	repo := &channelMonitorRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(channelMonitorAvailabilityCountsSQL)).
		WithArgs(int64(42), "claude-sonnet-4").
		WillReturnRows(sqlmock.NewRows([]string{"total", "ok"}).AddRow(8, 4))
	mock.ExpectExec(regexp.QuoteMeta(channelMonitorAvailabilityRaiseSQL)).
		WithArgs(int64(42), "claude-sonnet-4", 2).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(regexp.QuoteMeta(channelMonitorAvailabilityCountsSQL)).
		WithArgs(int64(42), "claude-sonnet-4").
		WillReturnRows(sqlmock.NewRows([]string{"total", "ok"}).AddRow(8, 6))
	mock.ExpectCommit()

	got, err := repo.AdjustAvailability7d(context.Background(), 42, "claude-sonnet-4", 75)
	if err != nil {
		t.Fatalf("AdjustAvailability7d returned error: %v", err)
	}
	if got.TotalChecks != 8 ||
		got.PreviousOperationalChecks != 4 ||
		got.TargetOperationalChecks != 6 ||
		got.ActualOperationalChecks != 6 ||
		got.ChangedRows != 2 ||
		got.PreviousAvailabilityPct != 50 ||
		got.RequestedAvailabilityPct != 75 ||
		got.ActualAvailabilityPct != 75 {
		t.Fatalf("unexpected adjustment result: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
