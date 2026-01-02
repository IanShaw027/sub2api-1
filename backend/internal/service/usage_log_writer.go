package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/metrics"
)

const (
	usageLogWriteMaxRetries = 2
	usageLogWriteRetryDelay = 100 * time.Millisecond
)

func createUsageLogWithRetry(ctx context.Context, repo UsageLogRepository, usageLog *UsageLog) error {
	if repo == nil || usageLog == nil {
		return nil
	}

	var lastErr error
	attempts := 0

	for attempt := 0; attempt <= usageLogWriteMaxRetries; attempt++ {
		attempts = attempt + 1

		if attempt > 0 {
			if err := sleepWithContext(ctx, usageLogWriteRetryDelay); err != nil {
				lastErr = err
				break
			}
		}

		if err := repo.Create(ctx, usageLog); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	metrics.IncUsageLogsFailed()

	payload, err := json.Marshal(usageLog)
	if err != nil {
		log.Printf(
			"Create usage log failed after retries: err=%v attempts=%d retry_delay=%s request_id=%s marshal_err=%v",
			lastErr,
			attempts,
			usageLogWriteRetryDelay,
			usageLog.RequestID,
			err,
		)
		return lastErr
	}

	log.Printf(
		"Create usage log failed after retries: err=%v attempts=%d retry_delay=%s usage=%s",
		lastErr,
		attempts,
		usageLogWriteRetryDelay,
		string(payload),
	)
	return lastErr
}
