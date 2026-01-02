package service

import (
	"context"
	"log"
	"time"
)

type OpsAccountStatusService struct {
	repo OpsRepository
}

func NewOpsAccountStatusService(repo OpsRepository) *OpsAccountStatusService {
	return &OpsAccountStatusService{repo: repo}
}

// UpdateAccountStatus 更新账号状态
func (s *OpsAccountStatusService) UpdateAccountStatus(ctx context.Context, accountID int64) error {
	// 1. 统计过去 1h 和 24h 的错误/成功次数
	stats1h, err := s.repo.GetAccountStats(ctx, accountID, time.Hour)
	if err != nil {
		return err
	}

	stats24h, err := s.repo.GetAccountStats(ctx, accountID, 24*time.Hour)
	if err != nil {
		return err
	}

	// 2. 获取最近一次错误信息
	lastError, err := s.repo.GetLastAccountError(ctx, accountID)
	if err != nil {
		return err
	}

	// 如果没有错误记录，跳过
	if lastError == nil {
		return nil
	}

	// 3. 判断账号状态
	status := determineAccountStatus(stats1h, lastError)

	// 4. 更新或插入 ops_account_status 表
	return s.repo.UpsertAccountStatus(ctx, &OpsAccountStatus{
		AccountID:         accountID,
		Platform:          lastError.Platform,
		Status:            status,
		LastErrorType:     lastError.Type,
		LastErrorMessage:  lastError.Message,
		LastErrorTime:     lastError.CreatedAt,
		ErrorCount1h:      stats1h.ErrorCount,
		SuccessCount1h:    stats1h.SuccessCount,
		TimeoutCount1h:    stats1h.TimeoutCount,
		RateLimitCount1h:  stats1h.RateLimitCount,
		ErrorCount24h:     stats24h.ErrorCount,
		SuccessCount24h:   stats24h.SuccessCount,
		TimeoutCount24h:   stats24h.TimeoutCount,
		RateLimitCount24h: stats24h.RateLimitCount,
		UpdatedAt:         time.Now(),
	})
}

// determineAccountStatus 根据统计数据判断账号状态
func determineAccountStatus(stats *AccountStats, lastError *OpsErrorLog) string {
	if lastError == nil {
		return "normal"
	}

	// 如果最近 5 分钟内有错误，使用错误中的账号状态
	if time.Since(lastError.CreatedAt) < 5*time.Minute && lastError.AccountStatus != "" {
		return lastError.AccountStatus
	}

	// 如果错误率过高
	if stats.ErrorCount > 0 {
		totalCount := stats.ErrorCount + stats.SuccessCount
		if totalCount > 0 {
			errorRate := float64(stats.ErrorCount) / float64(totalCount)
			if errorRate > 0.5 {
				return "error"
			}
		}
	}

	return "normal"
}

// StartAccountStatusUpdater 启动账号状态更新定时任务
func (s *OpsAccountStatusService) StartAccountStatusUpdater(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.updateAllAccountStatus(ctx)
		}
	}
}

func (s *OpsAccountStatusService) updateAllAccountStatus(ctx context.Context) {
	// 获取所有活跃账号
	accounts, err := s.repo.GetActiveAccounts(ctx)
	if err != nil {
		log.Printf("[ERROR] Failed to get active accounts: %v", err)
		return
	}

	for _, accountID := range accounts {
		if err := s.UpdateAccountStatus(ctx, accountID); err != nil {
			log.Printf("[ERROR] Failed to update account status for %d: %v", accountID, err)
		}
	}
}
