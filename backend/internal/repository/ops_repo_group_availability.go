package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ListGroupAvailabilityConfigs 获取所有分组可用性监控配置
func (r *OpsRepository) ListGroupAvailabilityConfigs(ctx context.Context, enabledOnly bool) ([]service.OpsGroupAvailabilityConfig, error) {
	query := `
		SELECT
			id, group_id, enabled, min_available_accounts, threshold_mode, min_available_percentage,
			notify_email,
			severity, cooldown_minutes,
			created_at, updated_at
		FROM ops_group_availability_configs`
	if enabledOnly {
		query += " WHERE enabled = true"
	}
	query += " ORDER BY id ASC"

	rows, err := r.sql.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	configs := make([]service.OpsGroupAvailabilityConfig, 0)
	for rows.Next() {
		var config service.OpsGroupAvailabilityConfig
		if err := rows.Scan(
			&config.ID, &config.GroupID, &config.Enabled, &config.MinAvailableAccounts, &config.ThresholdMode, &config.MinAvailablePercentage,
			&config.NotifyEmail,
			&config.Severity, &config.CooldownMinutes,
			&config.CreatedAt, &config.UpdatedAt,
		); err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

// GetGroupAvailabilityConfig 获取指定分组的监控配置
func (r *OpsRepository) GetGroupAvailabilityConfig(ctx context.Context, groupID int64) (*service.OpsGroupAvailabilityConfig, error) {
	query := `
		SELECT
			id, group_id, enabled, min_available_accounts, threshold_mode, min_available_percentage,
			notify_email,
			severity, cooldown_minutes,
			created_at, updated_at
		FROM ops_group_availability_configs
		WHERE group_id = $1`

	var config service.OpsGroupAvailabilityConfig
	err := r.sql.QueryRowContext(ctx, query, groupID).Scan(
		&config.ID, &config.GroupID, &config.Enabled, &config.MinAvailableAccounts, &config.ThresholdMode, &config.MinAvailablePercentage,
		&config.NotifyEmail,
		&config.Severity, &config.CooldownMinutes,
		&config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

// CreateGroupAvailabilityConfig 创建分组可用性监控配置
func (r *OpsRepository) CreateGroupAvailabilityConfig(ctx context.Context, cfg *service.OpsGroupAvailabilityConfig) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	now := time.Now()
	cfg.CreatedAt = now
	cfg.UpdatedAt = now

	query := `
		INSERT INTO ops_group_availability_configs (
			group_id, enabled, min_available_accounts, threshold_mode, min_available_percentage,
			notify_email, severity, cooldown_minutes, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id`

	var id int64
	if err := r.sql.QueryRowContext(ctx, query,
		cfg.GroupID, cfg.Enabled, cfg.MinAvailableAccounts, cfg.ThresholdMode, cfg.MinAvailablePercentage,
		cfg.NotifyEmail, cfg.Severity, cfg.CooldownMinutes, cfg.CreatedAt, cfg.UpdatedAt,
	).Scan(&id); err != nil {
		return err
	}
	cfg.ID = id
	return nil
}

// UpdateGroupAvailabilityConfig 更新分组可用性监控配置
func (r *OpsRepository) UpdateGroupAvailabilityConfig(ctx context.Context, cfg *service.OpsGroupAvailabilityConfig) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	cfg.UpdatedAt = time.Now()

	query := `
		UPDATE ops_group_availability_configs
		SET enabled=$2, min_available_accounts=$3, threshold_mode=$4, min_available_percentage=$5,
			notify_email=$6, severity=$7, cooldown_minutes=$8, updated_at=$9
		WHERE id=$1`

	_, err := r.sql.ExecContext(ctx, query,
		cfg.ID, cfg.Enabled, cfg.MinAvailableAccounts, cfg.ThresholdMode, cfg.MinAvailablePercentage,
		cfg.NotifyEmail, cfg.Severity, cfg.CooldownMinutes, cfg.UpdatedAt,
	)
	return err
}

// DeleteGroupAvailabilityConfig 删除分组可用性监控配置
func (r *OpsRepository) DeleteGroupAvailabilityConfig(ctx context.Context, id int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM ops_group_availability_configs WHERE id=$1`, id)
	return err
}

// GetActiveGroupAvailabilityEvent 获取指定配置的活跃事件
func (r *OpsRepository) GetActiveGroupAvailabilityEvent(ctx context.Context, configID int64) (*service.OpsGroupAvailabilityEvent, error) {
	query := `
		SELECT id, config_id, group_id, status, severity, title, description,
		       available_accounts, threshold_accounts, total_accounts, email_sent,
		       fired_at, resolved_at, created_at
		FROM ops_group_availability_events
		WHERE config_id = $1 AND status = $2
		ORDER BY fired_at DESC
		LIMIT 1`

	var event service.OpsGroupAvailabilityEvent
	var resolvedAt sql.NullTime
	err := r.sql.QueryRowContext(ctx, query, configID, service.OpsAlertStatusFiring).Scan(
		&event.ID, &event.ConfigID, &event.GroupID, &event.Status, &event.Severity, &event.Title, &event.Description,
		&event.AvailableAccounts, &event.ThresholdAccounts, &event.TotalAccounts, &event.EmailSent,
		&event.FiredAt, &resolvedAt, &event.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if resolvedAt.Valid {
		event.ResolvedAt = &resolvedAt.Time
	}
	return &event, nil
}

// GetLatestGroupAvailabilityEvent 获取指定配置的最新事件
func (r *OpsRepository) GetLatestGroupAvailabilityEvent(ctx context.Context, configID int64) (*service.OpsGroupAvailabilityEvent, error) {
	query := `
		SELECT id, config_id, group_id, status, severity, title, description,
		       available_accounts, threshold_accounts, total_accounts, email_sent,
		       fired_at, resolved_at, created_at
		FROM ops_group_availability_events
		WHERE config_id = $1
		ORDER BY fired_at DESC
		LIMIT 1`

	var event service.OpsGroupAvailabilityEvent
	var resolvedAt sql.NullTime
	err := r.sql.QueryRowContext(ctx, query, configID).Scan(
		&event.ID, &event.ConfigID, &event.GroupID, &event.Status, &event.Severity, &event.Title, &event.Description,
		&event.AvailableAccounts, &event.ThresholdAccounts, &event.TotalAccounts, &event.EmailSent,
		&event.FiredAt, &resolvedAt, &event.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if resolvedAt.Valid {
		event.ResolvedAt = &resolvedAt.Time
	}
	return &event, nil
}

// CreateGroupAvailabilityEvent 创建分组可用性告警事件
func (r *OpsRepository) CreateGroupAvailabilityEvent(ctx context.Context, event *service.OpsGroupAvailabilityEvent) error {
	if event == nil {
		return errors.New("event cannot be nil")
	}
	if event.FiredAt.IsZero() {
		event.FiredAt = time.Now()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	var resolvedAt sql.NullTime
	if event.ResolvedAt != nil {
		resolvedAt = sql.NullTime{Time: *event.ResolvedAt, Valid: true}
	}
	query := `
		INSERT INTO ops_group_availability_events (
			config_id, group_id, status, severity, title, description,
			available_accounts, threshold_accounts, total_accounts,
			email_sent, fired_at, resolved_at, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id`
	var id int64
	if err := r.sql.QueryRowContext(ctx, query,
		event.ConfigID, event.GroupID, event.Status, event.Severity, event.Title, event.Description,
		event.AvailableAccounts, event.ThresholdAccounts, event.TotalAccounts,
		event.EmailSent, event.FiredAt, resolvedAt, event.CreatedAt,
	).Scan(&id); err != nil {
		return err
	}
	event.ID = id
	return nil
}

// UpdateGroupAvailabilityEventStatus 更新事件状态
func (r *OpsRepository) UpdateGroupAvailabilityEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	var resolved sql.NullTime
	if resolvedAt != nil {
		resolved = sql.NullTime{Time: *resolvedAt, Valid: true}
	}
	_, err := r.sql.ExecContext(ctx, `
		UPDATE ops_group_availability_events
		SET status = $2, resolved_at = $3
		WHERE id = $1
	`, eventID, status, resolved)
	return err
}

// UpdateGroupAvailabilityEventNotifications 更新事件通知状态
func (r *OpsRepository) UpdateGroupAvailabilityEventNotifications(ctx context.Context, eventID int64, emailSent bool) error {
	_, err := r.sql.ExecContext(ctx, `
		UPDATE ops_group_availability_events
		SET email_sent = $2
		WHERE id = $1
	`, eventID, emailSent)
	return err
}

// ListGroupAvailabilityEvents 获取分组可用性告警事件列表
func (r *OpsRepository) ListGroupAvailabilityEvents(ctx context.Context, limit int, status string) ([]service.OpsGroupAvailabilityEvent, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT
			id, config_id, group_id, status, severity,
			title, description,
			available_accounts, threshold_accounts, total_accounts,
			email_sent,
			fired_at, resolved_at, created_at
		FROM ops_group_availability_events`

	args := make([]any, 0)
	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
		query += " ORDER BY fired_at DESC LIMIT $2"
		args = append(args, limit)
	} else {
		query += " ORDER BY fired_at DESC LIMIT $1"
		args = append(args, limit)
	}

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	events := make([]service.OpsGroupAvailabilityEvent, 0)
	for rows.Next() {
		var event service.OpsGroupAvailabilityEvent
		var resolvedAt sql.NullTime
		if err := rows.Scan(
			&event.ID, &event.ConfigID, &event.GroupID, &event.Status, &event.Severity,
			&event.Title, &event.Description,
			&event.AvailableAccounts, &event.ThresholdAccounts, &event.TotalAccounts,
			&event.EmailSent,
			&event.FiredAt, &resolvedAt, &event.CreatedAt,
		); err != nil {
			return nil, err
		}
		if resolvedAt.Valid {
			event.ResolvedAt = &resolvedAt.Time
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// CountAvailableAccountsByGroup 统计分组的可用账号数
func (r *OpsRepository) CountAvailableAccountsByGroup(ctx context.Context, groupID int64) (available, total int, err error) {
	query := `
		SELECT
			a.id,
			a.status,
			a.overload_until
		FROM accounts a
		INNER JOIN account_groups ag ON a.id = ag.account_id
		WHERE ag.group_id = $1 AND a.deleted_at IS NULL`

	rows, err := r.sql.QueryContext(ctx, query, groupID)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()

	now := time.Now()
	total = 0
	available = 0

	for rows.Next() {
		var accountID int64
		var status string
		var overloadUntil sql.NullTime

		if err := rows.Scan(&accountID, &status, &overloadUntil); err != nil {
			return 0, 0, err
		}

		total++

		// 判断账号是否可调度（status 为 active 表示可用）
		if status == "active" {
			// 检查是否在过载期
			if overloadUntil.Valid && now.Before(overloadUntil.Time) {
				continue
			}
			available++
		}
	}

	return available, total, rows.Err()
}
