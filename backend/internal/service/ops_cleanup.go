package service

import (
	"context"
	"errors"
	"time"
)

// DeleteOldErrorLogs deletes ops error logs older than retentionDays.
// It returns the number of deleted rows.
func (s *OpsIngestService) DeleteOldErrorLogs(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ctxDB, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	deleted, err := s.repo.DeleteOldErrorLogs(ctxDB, retentionDays)
	if err != nil {
		return deleted, err
	}
	return deleted, nil
}

// DeleteOldMetrics deletes ops metrics older than retentionDays for the given windowMinutes.
// It returns the number of deleted rows.
func (s *OpsIngestService) DeleteOldMetrics(ctx context.Context, windowMinutes int, retentionDays int) (int64, error) {
	if windowMinutes <= 0 {
		return 0, errors.New("windowMinutes must be positive")
	}
	if retentionDays <= 0 {
		return 0, nil
	}
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ctxDB, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	deleted, err := s.repo.DeleteOldMetrics(ctxDB, windowMinutes, retentionDays)
	if err != nil {
		return deleted, err
	}
	return deleted, nil
}
