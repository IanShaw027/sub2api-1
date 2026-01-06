package repository

import (
	"context"
	"errors"
	"time"

	"github.com/lib/pq"
)

// DeleteOldMetrics deletes metrics older than retentionDays for the given windowMinutes and returns the count of deleted rows.
func (r *OpsRepository) DeleteOldMetrics(ctx context.Context, windowMinutes int, retentionDays int) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	switch windowMinutes {
	case 60:
		// Hourly pre-aggregation table.
		result, err := r.sql.ExecContext(ctx, `DELETE FROM ops_metrics_hourly WHERE bucket_start < $1`, cutoffTime)
		if err != nil {
			return 0, err
		}
		deletedHourly, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}

		// Backward compatibility: also clean any system metrics recorded at 60-minute window.
		result, err = r.sql.ExecContext(ctx, `DELETE FROM ops_system_metrics WHERE window_minutes = $1 AND created_at < $2`, windowMinutes, cutoffTime)
		if err != nil {
			return 0, err
		}
		deletedSystem, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		return deletedHourly + deletedSystem, nil
	case 1440:
		// Daily pre-aggregation table.
		result, err := r.sql.ExecContext(ctx, `DELETE FROM ops_metrics_daily WHERE bucket_date < $1::date`, cutoffTime)
		if err != nil {
			return 0, err
		}
		deletedDaily, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}

		// Backward compatibility: also clean any system metrics recorded at 1440-minute window.
		result, err = r.sql.ExecContext(ctx, `DELETE FROM ops_system_metrics WHERE window_minutes = $1 AND created_at < $2`, windowMinutes, cutoffTime)
		if err != nil {
			return 0, err
		}
		deletedSystem, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		return deletedDaily + deletedSystem, nil
	default:
		// Primary system metrics store uses window_minutes.
		result, err := r.sql.ExecContext(ctx, `DELETE FROM ops_system_metrics WHERE window_minutes = $1 AND created_at < $2`, windowMinutes, cutoffTime)
		if err != nil {
			return 0, err
		}
		return result.RowsAffected()
	}
}

// GetRetentionConfig returns a map of table_name -> retention_days from the configuration table.
func (r *OpsRepository) GetRetentionConfig(ctx context.Context) (map[string]int, error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT table_name, retention_days FROM ops_data_retention_config WHERE enabled = true`)
	if err != nil {
		// ops_data_retention_config was introduced in an earlier migration and later removed.
		// Treat "undefined table" as a safe fallback (no retention config in DB).
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && string(pqErr.Code) == "42P01" {
			return map[string]int{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	config := make(map[string]int)
	for rows.Next() {
		var tableName string
		var days int
		if err := rows.Scan(&tableName, &days); err != nil {
			return nil, err
		}
		config[tableName] = days
	}
	return config, nil
}
