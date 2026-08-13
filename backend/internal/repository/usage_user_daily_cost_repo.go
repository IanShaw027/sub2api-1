package repository

import (
	"context"
	"database/sql"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type usageUserDailyCostRepository struct {
	sql sqlExecutor
}

func NewUsageUserDailyCostRepository(_ *dbent.Client, sqlDB *sql.DB) service.UsageUserDailyCostRepository {
	return &usageUserDailyCostRepository{sql: sqlDB}
}

const usageUserDailyCostDeleteDaySQL = `DELETE FROM usage_user_daily_cost WHERE bucket_date = $1::date`

const usageUserDailyCostInsertDaySQL = `
INSERT INTO usage_user_daily_cost (user_id, bucket_date, actual_cost, balance_actual_cost, subscription_actual_cost, updated_at)
SELECT
  user_id,
  $1::date,
  COALESCE(SUM(actual_cost), 0),
  COALESCE(SUM(actual_cost) FILTER (WHERE billing_type = $4), 0),
  COALESCE(SUM(actual_cost) FILTER (WHERE billing_type = $5), 0),
  NOW()
FROM usage_logs
WHERE created_at >= $2 AND created_at < $3
GROUP BY user_id`

func (r *usageUserDailyCostRepository) RecomputeDay(ctx context.Context, bucketDate string, dayStartUTC, dayEndUTC time.Time) error {
	if r == nil || r.sql == nil {
		return nil
	}
	if db, ok := r.sql.(*sql.DB); ok {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := recomputeUsageUserDailyCostInTx(ctx, tx, bucketDate, dayStartUTC, dayEndUTC); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	}
	return recomputeUsageUserDailyCostInTx(ctx, r.sql, bucketDate, dayStartUTC, dayEndUTC)
}

func recomputeUsageUserDailyCostInTx(ctx context.Context, exec sqlExecutor, bucketDate string, dayStartUTC, dayEndUTC time.Time) error {
	if _, err := exec.ExecContext(ctx, usageUserDailyCostDeleteDaySQL, bucketDate); err != nil {
		return err
	}
	_, err := exec.ExecContext(ctx, usageUserDailyCostInsertDaySQL, bucketDate, dayStartUTC.UTC(), dayEndUTC.UTC(), service.BillingTypeBalance, service.BillingTypeSubscription)
	return err
}

func (r *usageUserDailyCostRepository) DeleteOlderThan(ctx context.Context, cutoffDate string) (int64, error) {
	if r == nil || r.sql == nil {
		return 0, nil
	}
	res, err := r.sql.ExecContext(ctx, `DELETE FROM usage_user_daily_cost WHERE bucket_date < $1::date`, cutoffDate)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
