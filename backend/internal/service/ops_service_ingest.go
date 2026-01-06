package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/infraerror"
)

func (s *OpsIngestService) RecordError(ctx context.Context, log *OpsErrorLog, requestBody []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Value(opsRecordErrorGuardKey{}) != nil {
		// Prevent recursion: RecordError (or its error path) should never attempt to record again.
		return nil
	}
	ctx = context.WithValue(ctx, opsRecordErrorGuardKey{}, true)

	if log == nil {
		return nil
	}
	if s == nil || s.repo == nil {
		return nil
	}
	if s.isOpsEnabled != nil && !s.isOpsEnabled(ctx) {
		return nil
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	if log.Severity == "" {
		log.Severity = "P2"
	}
	if log.Phase == "" {
		log.Phase = "internal"
	}
	if log.Type == "" {
		log.Type = "unknown_error"
	}
	if log.Message == "" {
		log.Message = "Unknown error"
	}

	// 统一脱敏错误消息中的敏感信息
	log.Message = sanitizeErrorMessage(log.Message)
	log.ErrorBody = sanitizeErrorMessage(log.ErrorBody)
	log.UpstreamErrorMessage = sanitizeErrorMessage(log.UpstreamErrorMessage)
	if log.UpstreamErrorDetail != nil {
		sanitized := sanitizeErrorMessage(*log.UpstreamErrorDetail)
		log.UpstreamErrorDetail = &sanitized
	}

	// 脱敏并存储请求体（仅失败请求）
	if len(requestBody) > 0 {
		log.RequestBody = sanitizeRequestBody(requestBody)
	}

	ctxDB, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()
	ctxDB = infraerror.WithRecordingDisabled(ctxDB)

	if err := s.repo.CreateErrorLog(ctxDB, log); err != nil {
		// Best-effort fallback: if the ops log persistence fails due to infrastructure issues
		// (DB down, timeouts), write to the independent infra error channel instead.
		recordInfrastructureError(ctx, "db", "OpsIngestService.RecordError", err)
		return err
	}
	return nil
}

// RecordOpsError is a compatibility wrapper around RecordError.
func (s *OpsIngestService) RecordOpsError(ctx context.Context, log *OpsErrorLog, requestBody []byte) error {
	return s.RecordError(ctx, log, requestBody)
}

func (s *OpsIngestService) RecordMetrics(ctx context.Context, metric *OpsMetrics) error {
	if metric == nil {
		return nil
	}
	if s == nil || s.repo == nil {
		return nil
	}
	if s.isOpsEnabled != nil && !s.isOpsEnabled(ctx) {
		return nil
	}
	if metric.UpdatedAt.IsZero() {
		metric.UpdatedAt = time.Now()
	}

	ctxDB, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()
	if err := s.repo.CreateSystemMetric(ctxDB, metric); err != nil {
		return err
	}

	// Latest metrics snapshot is queried frequently by the ops dashboard; keep a short-lived cache
	// to avoid unnecessary DB pressure. Only cache the default (1-minute) window metrics.
	windowMinutes := metric.WindowMinutes
	if windowMinutes == 0 {
		windowMinutes = 1
	}
	if windowMinutes == 1 {
		if repo := s.repo; repo != nil {
			_ = repo.SetCachedLatestSystemMetric(ctx, metric)
		}
	}
	return nil
}
