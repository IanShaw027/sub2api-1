package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type ipSecurityRepository struct{ db *sql.DB }

func NewIPSecurityRepository(db *sql.DB) service.IPSecurityRepository {
	return &ipSecurityRepository{db: db}
}

func (r *ipSecurityRepository) GetUserIPs(ctx context.Context, userID int64) ([]string, bool, error) {
	var ips []string
	var saturated bool
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(ips,'{}'::text[]), COALESCE(ip_history_saturated,FALSE) FROM users WHERE id=$1`, userID).Scan(pq.Array(&ips), &saturated)
	return ips, saturated, err
}

func (r *ipSecurityRepository) AddUserIPActivity(ctx context.Context, a service.IPSecurityActivity, now time.Time) (bool, bool, error) {
	metadata, err := json.Marshal(a.Metadata)
	if err != nil {
		metadata = []byte(`{}`)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, false, err
	}
	defer tx.Rollback()
	var ips []string
	var saturated bool
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(ips,'{}'::text[]), COALESCE(ip_history_saturated,FALSE) FROM users WHERE id=$1 FOR UPDATE`, a.UserID).Scan(pq.Array(&ips), &saturated); err != nil {
		return false, saturated, err
	}
	for _, ip := range ips {
		if ip == a.IPAddress {
			return false, saturated, tx.Commit()
		}
	}
	if saturated {
		return false, true, tx.Commit()
	}
	if len(ips) >= 256 {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET ip_history_saturated=TRUE WHERE id=$1`, a.UserID); err != nil {
			return false, false, err
		}
		return false, true, tx.Commit()
	}
	ips = append(ips, a.IPAddress)
	saturated = len(ips) >= 256
	if _, err := tx.ExecContext(ctx, `UPDATE users SET ips=$2, ip_history_saturated=$3 WHERE id=$1`, a.UserID, pq.Array(ips), saturated); err != nil {
		return false, saturated, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO ip_security_activity
 (ip_address,peer_ip,forwarded_for,user_id,source,api_key_id,method,path,request_id,first_seen_at,metadata)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (user_id,ip_address) DO NOTHING`,
		a.IPAddress, a.PeerIP, a.ForwardedFor, a.UserID, a.Source, a.APIKeyID,
		a.Method, a.Path, a.RequestID, now, metadata); err != nil {
		return false, saturated, err
	}
	return true, saturated, tx.Commit()
}

func (r *ipSecurityRepository) CreateBan(ctx context.Context, b *service.IPSecurityBan) (bool, error) {
	err := r.db.QueryRowContext(ctx, `
INSERT INTO ip_security_bans
 (ip_address,status,reason,account_threshold,window_minutes,detected_account_count,first_seen_at,last_seen_at,created_at)
VALUES ($1,'active',$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (ip_address) DO UPDATE SET
 status='active',reason=EXCLUDED.reason,account_threshold=EXCLUDED.account_threshold,
 window_minutes=EXCLUDED.window_minutes,detected_account_count=EXCLUDED.detected_account_count,
 first_seen_at=EXCLUDED.first_seen_at,last_seen_at=EXCLUDED.last_seen_at,
 created_at=EXCLUDED.created_at,released_at=NULL,released_by=NULL
WHERE ip_security_bans.status <> 'whitelisted'
RETURNING id`, b.IPAddress, b.Reason, b.AccountThreshold, b.WindowMinutes,
		b.DetectedAccountCount, b.FirstSeenAt, b.LastSeenAt, b.CreatedAt).Scan(&b.ID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (r *ipSecurityRepository) IsIPStatus(ctx context.Context, ip, status string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM ip_security_bans WHERE ip_address=$1 AND status=$2)`, ip, status).Scan(&exists)
	return exists, err
}

func (r *ipSecurityRepository) LoadIPStatuses(ctx context.Context) (active, whitelisted []string, err error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ip_address,status FROM ip_security_bans WHERE status IN ('active','whitelisted')`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ip, status string
		if err := rows.Scan(&ip, &status); err != nil {
			return nil, nil, err
		}
		if status == "active" {
			active = append(active, ip)
		} else {
			whitelisted = append(whitelisted, ip)
		}
	}
	return active, whitelisted, rows.Err()
}

func (r *ipSecurityRepository) ListBans(ctx context.Context, status string, limit, offset int) ([]service.IPSecurityBan, int64, error) {
	where := ""
	args := []any{}
	if status != "" {
		where = ` WHERE status=$1`
		args = append(args, status)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_security_bans`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT id,ip_address,status,reason,account_threshold,window_minutes,detected_account_count,first_seen_at,last_seen_at,created_at,released_at FROM ip_security_bans` + where
	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT %d OFFSET %d`, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []service.IPSecurityBan{}
	for rows.Next() {
		var b service.IPSecurityBan
		if err := rows.Scan(&b.ID, &b.IPAddress, &b.Status, &b.Reason, &b.AccountThreshold,
			&b.WindowMinutes, &b.DetectedAccountCount, &b.FirstSeenAt, &b.LastSeenAt,
			&b.CreatedAt, &b.ReleasedAt); err != nil {
			return nil, 0, err
		}
		result = append(result, b)
	}
	return result, total, rows.Err()
}

func (r *ipSecurityRepository) GetBan(ctx context.Context, id int64) (*service.IPSecurityBan, error) {
	var b service.IPSecurityBan
	err := r.db.QueryRowContext(ctx, `
SELECT id,ip_address,status,reason,account_threshold,window_minutes,detected_account_count,
 first_seen_at,last_seen_at,created_at,released_at
FROM ip_security_bans WHERE id=$1`, id).Scan(&b.ID, &b.IPAddress, &b.Status, &b.Reason,
		&b.AccountThreshold, &b.WindowMinutes, &b.DetectedAccountCount, &b.FirstSeenAt,
		&b.LastSeenAt, &b.CreatedAt, &b.ReleasedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *ipSecurityRepository) ListActivity(ctx context.Context, ip string, since, until time.Time) ([]service.IPSecurityActivityDetail, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT a.ip_address,a.peer_ip,a.forwarded_for,a.user_id,a.source,a.api_key_id,a.method,a.path,
 a.request_id,a.first_seen_at,a.metadata,COALESCE(u.email,''),COALESCE(u.username,'')
FROM ip_security_activity a JOIN users u ON u.id=a.user_id
WHERE a.ip_address=$1 AND a.first_seen_at >= $2 AND a.first_seen_at <= $3
ORDER BY a.first_seen_at DESC`, ip, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []service.IPSecurityActivityDetail{}
	for rows.Next() {
		var d service.IPSecurityActivityDetail
		var metadata []byte
		if err := rows.Scan(&d.IPAddress, &d.PeerIP, &d.ForwardedFor, &d.UserID, &d.Source,
			&d.APIKeyID, &d.Method, &d.Path, &d.RequestID, &d.FirstSeenAt, &metadata,
			&d.UserEmail, &d.UserUsername); err != nil {
			return nil, err
		}
		d.RequestCount = 1
		d.LastSeenAt = d.FirstSeenAt
		_ = json.Unmarshal(metadata, &d.Metadata)
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The activity table intentionally stores only the first new-IP event per user.
	// For administrator detail views, enrich it with actual request-level usage rows.
	usageRows, err := r.db.QueryContext(ctx, `
SELECT ul.ip_address, ul.user_id, ul.api_key_id,
       COALESCE(ul.inbound_endpoint,''), COALESCE(ul.request_id,''),
       COUNT(*)::bigint, MIN(ul.created_at), MAX(ul.created_at),
       COALESCE(u.email,''), COALESCE(u.username,'')
FROM usage_logs ul
JOIN users u ON u.id=ul.user_id
WHERE ul.ip_address=$1 AND ul.created_at >= $2 AND ul.created_at <= $3
  AND NOT EXISTS (
      SELECT 1 FROM ip_security_activity a
      WHERE a.ip_address=$1 AND a.request_id <> ''
        AND a.request_id=COALESCE(ul.request_id,'')
  )
GROUP BY ul.ip_address, ul.user_id, ul.api_key_id,
         COALESCE(ul.inbound_endpoint,''), COALESCE(ul.request_id,''), u.email, u.username
ORDER BY MAX(ul.created_at) DESC`, ip, since, until)
	if err != nil {
		return nil, err
	}
	defer usageRows.Close()
	for usageRows.Next() {
		var d service.IPSecurityActivityDetail
		if err := usageRows.Scan(&d.IPAddress, &d.UserID, &d.APIKeyID, &d.Path, &d.RequestID,
			&d.RequestCount, &d.FirstSeenAt, &d.LastSeenAt, &d.UserEmail, &d.UserUsername); err != nil {
			return nil, err
		}
		d.Source = service.IPSecuritySourceAPIKey
		if d.APIKeyID == 0 {
			d.Source = service.IPSecuritySourceWeb
		}
		result = append(result, d)
	}
	if err := usageRows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].LastSeenAt.After(result[j].LastSeenAt) })
	return result, nil
}

func (r *ipSecurityRepository) WhitelistBan(ctx context.Context, id, releasedBy int64, now time.Time) (*service.IPSecurityBan, error) {
	var b service.IPSecurityBan
	err := r.db.QueryRowContext(ctx, `
UPDATE ip_security_bans SET status='whitelisted',released_at=$2,released_by=$3
WHERE id=$1 AND status='active'
RETURNING id,ip_address,status,reason,account_threshold,window_minutes,detected_account_count,
 first_seen_at,last_seen_at,created_at,released_at`, id, now, releasedBy).Scan(&b.ID, &b.IPAddress,
		&b.Status, &b.Reason, &b.AccountThreshold, &b.WindowMinutes, &b.DetectedAccountCount,
		&b.FirstSeenAt, &b.LastSeenAt, &b.CreatedAt, &b.ReleasedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *ipSecurityRepository) RemoveWhitelist(ctx context.Context, id, removedBy int64, now time.Time) (*service.IPSecurityBan, error) {
	var b service.IPSecurityBan
	err := r.db.QueryRowContext(ctx, `
	UPDATE ip_security_bans SET status='released',released_at=$2,released_by=$3
	WHERE id=$1 AND status='whitelisted'
RETURNING id,ip_address,status,reason,account_threshold,window_minutes,detected_account_count,
		 first_seen_at,last_seen_at,created_at,released_at`, id, now, removedBy).Scan(&b.ID, &b.IPAddress,
		&b.Status, &b.Reason, &b.AccountThreshold, &b.WindowMinutes, &b.DetectedAccountCount,
		&b.FirstSeenAt, &b.LastSeenAt, &b.CreatedAt, &b.ReleasedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}
