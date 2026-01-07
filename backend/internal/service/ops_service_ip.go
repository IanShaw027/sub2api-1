package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

// GetAllActiveAccountStatus returns stats for all active accounts.
func (s *OpsQueryService) GetAllActiveAccountStatus(ctx context.Context, platform string, groupID int64) ([]AccountStatusSummary, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	ctxDB, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()
	return s.repo.GetAllActiveAccountStatus(ctxDB, strings.TrimSpace(platform), groupID)
}

// GetErrorStatsByIP 获取IP错误统计
func (s *OpsQueryService) GetErrorStatsByIP(ctx context.Context, startTime, endTime time.Time, limit int, sortBy, sortOrder string) ([]IPErrorStats, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	ctxDB, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()
	return s.repo.GetErrorStatsByIP(ctxDB, startTime, endTime, limit, sortBy, sortOrder)
}

// GetErrorsByIP 获取特定IP的错误详情
func (s *OpsQueryService) GetErrorsByIP(ctx context.Context, ip string, startTime, endTime time.Time, page, pageSize int) ([]OpsErrorLog, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, nil
	}
	ctxDB, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()
	return s.repo.GetErrorsByIP(ctxDB, ip, startTime, endTime, page, pageSize)
}

// GetErrorLogByID retrieves a single error log by its ID with all details.
func (s *OpsQueryService) GetErrorLogByID(ctx context.Context, id int64) (*OpsErrorLog, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("ops service not initialized")
	}
	ctxDB, cancel := context.WithTimeout(ctx, opsDBQueryTimeout)
	defer cancel()
	return s.repo.GetErrorLogByID(ctxDB, id)
}
