package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/opsalertevent"
	"github.com/Wei-Shaw/sub2api/ent/opsalertrule"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *OpsRepository) ListAlertRules(ctx context.Context) ([]service.OpsAlertRule, error) {
	if r != nil && r.ent != nil {
		entRules, err := r.ent.OpsAlertRule.
			Query().
			Order(dbent.Asc(opsalertrule.FieldID)).
			All(ctx)
		if err != nil {
			return nil, err
		}

		rules := make([]service.OpsAlertRule, 0, len(entRules))
		for _, rule := range entRules {
			description := ""
			if rule.Description != nil {
				description = *rule.Description
			}

			rules = append(rules, service.OpsAlertRule{
				ID:               rule.ID,
				Name:             rule.Name,
				Description:      description,
				Enabled:          rule.Enabled,
				MetricType:       rule.MetricType,
				Operator:         rule.Operator,
				Threshold:        rule.Threshold,
				WindowMinutes:    rule.WindowMinutes,
				SustainedMinutes: rule.SustainedMinutes,
				Severity:         rule.Severity,
				NotifyEmail:      rule.NotifyEmail,
				CooldownMinutes:  rule.CooldownMinutes,
				DimensionFilters: rule.DimensionFilters,
				NotifyChannels:   rule.NotifyChannels,
				NotifyConfig:     rule.NotifyConfig,
				CreatedAt:        rule.CreatedAt,
				UpdatedAt:        rule.UpdatedAt,
			})
		}
		return rules, nil
	}

	query := `
		SELECT
			id,
			name,
			description,
			enabled,
			metric_type,
			operator,
			threshold,
			window_minutes,
			sustained_minutes,
			severity,
			notify_email,
			cooldown_minutes,
			dimension_filters,
			notify_channels,
			notify_config,
			created_at,
			updated_at
		FROM ops_alert_rules
		ORDER BY id ASC
	`

	rows, err := r.sql.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	rules := make([]service.OpsAlertRule, 0)
	for rows.Next() {
		var rule service.OpsAlertRule
		var description sql.NullString
		var dimensionFilters, notifyChannels, notifyConfig []byte
		if err := rows.Scan(
			&rule.ID,
			&rule.Name,
			&description,
			&rule.Enabled,
			&rule.MetricType,
			&rule.Operator,
			&rule.Threshold,
			&rule.WindowMinutes,
			&rule.SustainedMinutes,
			&rule.Severity,
			&rule.NotifyEmail,
			&rule.CooldownMinutes,
			&dimensionFilters,
			&notifyChannels,
			&notifyConfig,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if description.Valid {
			rule.Description = description.String
		}
		if len(dimensionFilters) > 0 {
			_ = json.Unmarshal(dimensionFilters, &rule.DimensionFilters)
		}
		if len(notifyChannels) > 0 {
			_ = json.Unmarshal(notifyChannels, &rule.NotifyChannels)
		}
		if len(notifyConfig) > 0 {
			_ = json.Unmarshal(notifyConfig, &rule.NotifyConfig)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *OpsRepository) CreateAlertRule(ctx context.Context, rule *service.OpsAlertRule) error {
	if rule == nil {
		return errors.New("rule cannot be nil")
	}
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	if r != nil && r.ent != nil {
		builder := r.ent.OpsAlertRule.
			Create().
			SetName(rule.Name).
			SetDescription(rule.Description).
			SetEnabled(rule.Enabled).
			SetMetricType(rule.MetricType).
			SetOperator(rule.Operator).
			SetThreshold(rule.Threshold).
			SetWindowMinutes(rule.WindowMinutes).
			SetSustainedMinutes(rule.SustainedMinutes).
			SetSeverity(rule.Severity).
			SetNotifyEmail(rule.NotifyEmail).
			SetCooldownMinutes(rule.CooldownMinutes).
			SetCreatedAt(rule.CreatedAt).
			SetUpdatedAt(rule.UpdatedAt)

		if len(rule.DimensionFilters) > 0 {
			builder.SetDimensionFilters(rule.DimensionFilters)
		}
		if len(rule.NotifyChannels) > 0 {
			builder.SetNotifyChannels(rule.NotifyChannels)
		}
		if rule.NotifyConfig != nil {
			builder.SetNotifyConfig(rule.NotifyConfig)
		}

		entRule, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		rule.ID = entRule.ID
		return nil
	}

	dimensionFiltersJSON, _ := json.Marshal(rule.DimensionFilters)
	notifyChannelsJSON, _ := json.Marshal(rule.NotifyChannels)
	notifyConfigJSON, _ := json.Marshal(rule.NotifyConfig)

	query := `
		INSERT INTO ops_alert_rules (
			name,
			description,
			enabled,
			metric_type,
			operator,
			threshold,
			window_minutes,
			sustained_minutes,
			severity,
			notify_email,
			cooldown_minutes,
			dimension_filters,
			notify_channels,
			notify_config,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16
		)
		RETURNING id
	`

	var id int64
	err := r.sql.QueryRowContext(ctx, query,
		rule.Name,
		nullString(rule.Description),
		rule.Enabled,
		rule.MetricType,
		rule.Operator,
		rule.Threshold,
		rule.WindowMinutes,
		rule.SustainedMinutes,
		rule.Severity,
		rule.NotifyEmail,
		rule.CooldownMinutes,
		dimensionFiltersJSON,
		notifyChannelsJSON,
		notifyConfigJSON,
		rule.CreatedAt,
		rule.UpdatedAt,
	).Scan(&id)
	if err != nil {
		return err
	}
	rule.ID = id
	return nil
}

func (r *OpsRepository) UpdateAlertRule(ctx context.Context, rule *service.OpsAlertRule) error {
	if rule == nil {
		return errors.New("rule cannot be nil")
	}
	rule.UpdatedAt = time.Now()

	if r != nil && r.ent != nil {
		builder := r.ent.OpsAlertRule.UpdateOneID(rule.ID).
			SetName(rule.Name).
			SetDescription(rule.Description).
			SetEnabled(rule.Enabled).
			SetMetricType(rule.MetricType).
			SetOperator(rule.Operator).
			SetThreshold(rule.Threshold).
			SetWindowMinutes(rule.WindowMinutes).
			SetSustainedMinutes(rule.SustainedMinutes).
			SetSeverity(rule.Severity).
			SetNotifyEmail(rule.NotifyEmail).
			SetCooldownMinutes(rule.CooldownMinutes).
			SetUpdatedAt(rule.UpdatedAt)

		if rule.DimensionFilters != nil {
			builder.SetDimensionFilters(rule.DimensionFilters)
		}
		if rule.NotifyChannels != nil {
			builder.SetNotifyChannels(rule.NotifyChannels)
		}
		if rule.NotifyConfig != nil {
			builder.SetNotifyConfig(rule.NotifyConfig)
		}

		_, err := builder.Save(ctx)
		return err
	}

	dimensionFiltersJSON, _ := json.Marshal(rule.DimensionFilters)
	notifyChannelsJSON, _ := json.Marshal(rule.NotifyChannels)
	notifyConfigJSON, _ := json.Marshal(rule.NotifyConfig)

	query := `
		UPDATE ops_alert_rules
		SET
			name = $2,
			description = $3,
			enabled = $4,
			metric_type = $5,
			operator = $6,
			threshold = $7,
			window_minutes = $8,
			sustained_minutes = $9,
			severity = $10,
			notify_email = $11,
			cooldown_minutes = $12,
			dimension_filters = $13,
			notify_channels = $14,
			notify_config = $15,
			updated_at = $16
		WHERE id = $1
	`

	_, err := r.sql.ExecContext(ctx, query,
		rule.ID,
		rule.Name,
		nullString(rule.Description),
		rule.Enabled,
		rule.MetricType,
		rule.Operator,
		rule.Threshold,
		rule.WindowMinutes,
		rule.SustainedMinutes,
		rule.Severity,
		rule.NotifyEmail,
		rule.CooldownMinutes,
		dimensionFiltersJSON,
		notifyChannelsJSON,
		notifyConfigJSON,
		rule.UpdatedAt,
	)
	return err
}

func (r *OpsRepository) DeleteAlertRule(ctx context.Context, id int64) error {
	if r != nil && r.ent != nil {
		return r.ent.OpsAlertRule.DeleteOneID(id).Exec(ctx)
	}

	_, err := r.sql.ExecContext(ctx, `DELETE FROM ops_alert_rules WHERE id = $1`, id)
	return err
}

func (r *OpsRepository) ListAlertEvents(ctx context.Context, limit int) ([]service.OpsAlertEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	if limit > 500 {
		limit = 500
	}

	if r != nil && r.ent != nil {
		entEvents, err := r.ent.OpsAlertEvent.
			Query().
			Order(dbent.Desc(opsalertevent.FieldCreatedAt)).
			Limit(limit).
			All(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]service.OpsAlertEvent, 0, len(entEvents))
		for _, e := range entEvents {
			title := ""
			if e.Title != nil {
				title = *e.Title
			}
			description := ""
			if e.Description != nil {
				description = *e.Description
			}

			metricValue := float64(0)
			if e.MetricValue != nil {
				metricValue = *e.MetricValue
			}
			thresholdValue := float64(0)
			if e.ThresholdValue != nil {
				thresholdValue = *e.ThresholdValue
			}

			events = append(events, service.OpsAlertEvent{
				ID:             e.ID,
				RuleID:         e.RuleID,
				Severity:       e.Severity,
				Status:         e.Status,
				Title:          title,
				Description:    description,
				MetricValue:    metricValue,
				ThresholdValue: thresholdValue,
				FiredAt:        e.FiredAt,
				ResolvedAt:     e.ResolvedAt,
				EmailSent:      e.EmailSent,
				CreatedAt:      e.CreatedAt,
			})
		}
		return events, nil
	}

	query := `
		SELECT
			id,
			rule_id,
			severity,
			status,
			title,
			description,
			metric_value,
			threshold_value,
			fired_at,
			resolved_at,
			email_sent,
			created_at
		FROM ops_alert_events
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.sql.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	events := make([]service.OpsAlertEvent, 0)
	for rows.Next() {
		var event service.OpsAlertEvent
		var resolvedAt sql.NullTime
		var metricValue sql.NullFloat64
		var thresholdValue sql.NullFloat64
		if err := rows.Scan(
			&event.ID,
			&event.RuleID,
			&event.Severity,
			&event.Status,
			&event.Title,
			&event.Description,
			&metricValue,
			&thresholdValue,
			&event.FiredAt,
			&resolvedAt,
			&event.EmailSent,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		if metricValue.Valid {
			event.MetricValue = metricValue.Float64
		}
		if thresholdValue.Valid {
			event.ThresholdValue = thresholdValue.Float64
		}
		if resolvedAt.Valid {
			event.ResolvedAt = &resolvedAt.Time
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *OpsRepository) GetActiveAlertEvent(ctx context.Context, ruleID int64) (*service.OpsAlertEvent, error) {
	if r != nil && r.ent != nil {
		entEvent, err := r.ent.OpsAlertEvent.
			Query().
			Where(
				opsalertevent.RuleID(ruleID),
				opsalertevent.Status(service.OpsAlertStatusFiring),
			).
			Order(dbent.Desc(opsalertevent.FieldFiredAt)).
			First(ctx)
		if err != nil {
			if dbent.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}

		title := ""
		if entEvent.Title != nil {
			title = *entEvent.Title
		}
		description := ""
		if entEvent.Description != nil {
			description = *entEvent.Description
		}

		metricValue := float64(0)
		if entEvent.MetricValue != nil {
			metricValue = *entEvent.MetricValue
		}
		thresholdValue := float64(0)
		if entEvent.ThresholdValue != nil {
			thresholdValue = *entEvent.ThresholdValue
		}

		return &service.OpsAlertEvent{
			ID:             entEvent.ID,
			RuleID:         entEvent.RuleID,
			Severity:       entEvent.Severity,
			Status:         entEvent.Status,
			Title:          title,
			Description:    description,
			MetricValue:    metricValue,
			ThresholdValue: thresholdValue,
			FiredAt:        entEvent.FiredAt,
			ResolvedAt:     entEvent.ResolvedAt,
			EmailSent:      entEvent.EmailSent,
			CreatedAt:      entEvent.CreatedAt,
		}, nil
	}

	return r.getAlertEvent(ctx, "WHERE rule_id = $1 AND status = $2", []any{ruleID, service.OpsAlertStatusFiring})
}

func (r *OpsRepository) GetLatestAlertEvent(ctx context.Context, ruleID int64) (*service.OpsAlertEvent, error) {
	if r != nil && r.ent != nil {
		entEvent, err := r.ent.OpsAlertEvent.
			Query().
			Where(opsalertevent.RuleID(ruleID)).
			Order(dbent.Desc(opsalertevent.FieldFiredAt)).
			First(ctx)
		if err != nil {
			if dbent.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}

		title := ""
		if entEvent.Title != nil {
			title = *entEvent.Title
		}
		description := ""
		if entEvent.Description != nil {
			description = *entEvent.Description
		}

		metricValue := float64(0)
		if entEvent.MetricValue != nil {
			metricValue = *entEvent.MetricValue
		}
		thresholdValue := float64(0)
		if entEvent.ThresholdValue != nil {
			thresholdValue = *entEvent.ThresholdValue
		}

		return &service.OpsAlertEvent{
			ID:             entEvent.ID,
			RuleID:         entEvent.RuleID,
			Severity:       entEvent.Severity,
			Status:         entEvent.Status,
			Title:          title,
			Description:    description,
			MetricValue:    metricValue,
			ThresholdValue: thresholdValue,
			FiredAt:        entEvent.FiredAt,
			ResolvedAt:     entEvent.ResolvedAt,
			EmailSent:      entEvent.EmailSent,
			CreatedAt:      entEvent.CreatedAt,
		}, nil
	}

	return r.getAlertEvent(ctx, "WHERE rule_id = $1", []any{ruleID})
}

func (r *OpsRepository) CreateAlertEvent(ctx context.Context, event *service.OpsAlertEvent) error {
	if event == nil {
		return errors.New("event cannot be nil")
	}
	if event.FiredAt.IsZero() {
		event.FiredAt = time.Now()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	if r != nil && r.ent != nil {
		builder := r.ent.OpsAlertEvent.
			Create().
			SetRuleID(event.RuleID).
			SetSeverity(event.Severity).
			SetStatus(event.Status).
			SetTitle(event.Title).
			SetDescription(event.Description).
			SetFiredAt(event.FiredAt).
			SetEmailSent(event.EmailSent).
			SetCreatedAt(event.CreatedAt)

		if event.MetricValue != 0 {
			builder.SetMetricValue(event.MetricValue)
		}
		if event.ThresholdValue != 0 {
			builder.SetThresholdValue(event.ThresholdValue)
		}
		if event.ResolvedAt != nil {
			builder.SetResolvedAt(*event.ResolvedAt)
		}

		entEvent, err := builder.Save(ctx)
		if err != nil {
			return err
		}
		event.ID = entEvent.ID
		return nil
	}

	var resolvedAt sql.NullTime
	if event.ResolvedAt != nil {
		resolvedAt = sql.NullTime{Time: *event.ResolvedAt, Valid: true}
	}

	query := `
		INSERT INTO ops_alert_events (
			rule_id,
			severity,
			status,
			title,
			description,
			metric_value,
			threshold_value,
			fired_at,
			resolved_at,
			email_sent,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11
		)
		RETURNING id
	`

	var id int64
	err := r.sql.QueryRowContext(ctx, query,
		event.RuleID,
		event.Severity,
		event.Status,
		event.Title,
		event.Description,
		event.MetricValue,
		event.ThresholdValue,
		event.FiredAt,
		resolvedAt,
		event.EmailSent,
		event.CreatedAt,
	).Scan(&id)
	if err != nil {
		return err
	}
	event.ID = id
	return nil
}

func (r *OpsRepository) UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	if r != nil && r.ent != nil {
		upd := r.ent.OpsAlertEvent.UpdateOneID(eventID).SetStatus(status)
		if resolvedAt != nil {
			upd.SetResolvedAt(*resolvedAt)
		} else {
			upd.ClearResolvedAt()
		}
		_, err := upd.Save(ctx)
		return err
	}

	query := `
		UPDATE ops_alert_events
		SET status = $2, resolved_at = $3
		WHERE id = $1
	`

	var resolved sql.NullTime
	if resolvedAt != nil {
		resolved = sql.NullTime{Time: *resolvedAt, Valid: true}
	}
	_, err := r.sql.ExecContext(ctx, query, eventID, status, resolved)
	return err
}

func (r *OpsRepository) UpdateAlertEventNotifications(ctx context.Context, eventID int64, emailSent bool) error {
	if r != nil && r.ent != nil {
		_, err := r.ent.OpsAlertEvent.UpdateOneID(eventID).SetEmailSent(emailSent).Save(ctx)
		return err
	}
	_, err := r.sql.ExecContext(ctx, `
		UPDATE ops_alert_events
		SET email_sent = $2
		WHERE id = $1
	`, eventID, emailSent)
	return err
}

func (r *OpsRepository) CountActiveAlerts(ctx context.Context) (int, error) {
	if r != nil && r.ent != nil {
		count, err := r.ent.OpsAlertEvent.
			Query().
			Where(opsalertevent.Status(service.OpsAlertStatusFiring)).
			Count(ctx)
		if err != nil {
			return 0, err
		}
		return count, nil
	}

	query := `
		SELECT COUNT(*)
		FROM ops_alert_events
		WHERE status = $1
	`

	var count int64
	if err := r.sql.QueryRowContext(ctx, query, service.OpsAlertStatusFiring).Scan(&count); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return int(count), nil
}

func (r *OpsRepository) getAlertEvent(ctx context.Context, whereClause string, args []any) (*service.OpsAlertEvent, error) {
	query := fmt.Sprintf(`
		SELECT
			id,
			rule_id,
			severity,
			status,
			title,
			description,
			metric_value,
			threshold_value,
			fired_at,
			resolved_at,
			email_sent,
			created_at
		FROM ops_alert_events
		%s
		ORDER BY fired_at DESC
		LIMIT 1
	`, whereClause)

	var event service.OpsAlertEvent
	var resolvedAt sql.NullTime
	var metricValue sql.NullFloat64
	var thresholdValue sql.NullFloat64
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		args,
		&event.ID,
		&event.RuleID,
		&event.Severity,
		&event.Status,
		&event.Title,
		&event.Description,
		&metricValue,
		&thresholdValue,
		&event.FiredAt,
		&resolvedAt,
		&event.EmailSent,
		&event.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if metricValue.Valid {
		event.MetricValue = metricValue.Float64
	}
	if thresholdValue.Valid {
		event.ThresholdValue = thresholdValue.Float64
	}
	if resolvedAt.Valid {
		event.ResolvedAt = &resolvedAt.Time
	}
	return &event, nil
}
