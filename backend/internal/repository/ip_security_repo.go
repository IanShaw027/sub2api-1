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
	return r.createBan(ctx, b, false)
}

func (r *ipSecurityRepository) ForceCreateBan(ctx context.Context, b *service.IPSecurityBan) (bool, error) {
	return r.createBan(ctx, b, true)
}

func (r *ipSecurityRepository) createBan(ctx context.Context, b *service.IPSecurityBan, force bool) (bool, error) {
	query := `
INSERT INTO ip_security_bans
 (ip_address,status,reason,account_threshold,window_minutes,detected_account_count,first_seen_at,last_seen_at,created_at)
VALUES ($1,'active',$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (ip_address) DO UPDATE SET
 status='active',reason=EXCLUDED.reason,account_threshold=EXCLUDED.account_threshold,
 window_minutes=EXCLUDED.window_minutes,detected_account_count=EXCLUDED.detected_account_count,
 first_seen_at=EXCLUDED.first_seen_at,last_seen_at=EXCLUDED.last_seen_at,
 created_at=EXCLUDED.created_at,released_at=NULL,released_by=NULL
WHERE ip_security_bans.status <> 'whitelisted'
RETURNING id`
	if force {
		query = `
INSERT INTO ip_security_bans
 (ip_address,status,reason,account_threshold,window_minutes,detected_account_count,first_seen_at,last_seen_at,created_at)
VALUES ($1,'active',$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (ip_address) DO UPDATE SET
 status='active',reason=EXCLUDED.reason,account_threshold=EXCLUDED.account_threshold,
 window_minutes=EXCLUDED.window_minutes,detected_account_count=EXCLUDED.detected_account_count,
 first_seen_at=EXCLUDED.first_seen_at,last_seen_at=EXCLUDED.last_seen_at,
 created_at=EXCLUDED.created_at,released_at=NULL,released_by=NULL
RETURNING id`
	}
	err := r.db.QueryRowContext(ctx, query, b.IPAddress, b.Reason, b.AccountThreshold, b.WindowMinutes,
		b.DetectedAccountCount, b.FirstSeenAt, b.LastSeenAt, b.CreatedAt).Scan(&b.ID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (r *ipSecurityRepository) GetBanByIP(ctx context.Context, ip string) (*service.IPSecurityBan, error) {
	var b service.IPSecurityBan
	err := r.db.QueryRowContext(ctx, `
SELECT id,ip_address,status,reason,account_threshold,window_minutes,detected_account_count,
 first_seen_at,last_seen_at,created_at,released_at
FROM ip_security_bans WHERE ip_address=$1`, ip).Scan(&b.ID, &b.IPAddress, &b.Status, &b.Reason,
		&b.AccountThreshold, &b.WindowMinutes, &b.DetectedAccountCount,
		&b.FirstSeenAt, &b.LastSeenAt, &b.CreatedAt, &b.ReleasedAt)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
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
	// The activity table stores only the first new-IP judgment per user.
	// Enrich with usage aggregates per (user, api_key) — never per request_id —
	// so the admin detail view stays one row per effective judgment surface.
	usageRows, err := r.db.QueryContext(ctx, `
SELECT ul.ip_address, ul.user_id, ul.api_key_id,
       COUNT(*)::bigint, MIN(ul.created_at), MAX(ul.created_at),
       COALESCE(u.email,''), COALESCE(u.username,'')
FROM usage_logs ul
JOIN users u ON u.id=ul.user_id
WHERE ul.ip_address=$1 AND ul.created_at >= $2 AND ul.created_at <= $3
  AND ul.ip_address IS NOT NULL AND ul.ip_address <> ''
GROUP BY ul.ip_address, ul.user_id, ul.api_key_id, u.email, u.username
ORDER BY MAX(ul.created_at) DESC`, ip, since, until)
	if err != nil {
		return nil, err
	}
	defer usageRows.Close()

	type activityKey struct {
		userID   int64
		apiKeyID int64
	}
	merged := make(map[activityKey]int, len(result))
	for i := range result {
		key := activityKey{userID: result[i].UserID, apiKeyID: result[i].APIKeyID}
		merged[key] = i
	}
	for usageRows.Next() {
		var d service.IPSecurityActivityDetail
		if err := usageRows.Scan(&d.IPAddress, &d.UserID, &d.APIKeyID,
			&d.RequestCount, &d.FirstSeenAt, &d.LastSeenAt, &d.UserEmail, &d.UserUsername); err != nil {
			return nil, err
		}
		d.Source = service.IPSecuritySourceAPIKey
		if d.APIKeyID == 0 {
			d.Source = service.IPSecuritySourceWeb
		}
		key := activityKey{userID: d.UserID, apiKeyID: d.APIKeyID}
		if idx, ok := merged[key]; ok {
			result[idx].RequestCount = d.RequestCount
			if d.FirstSeenAt.Before(result[idx].FirstSeenAt) {
				result[idx].FirstSeenAt = d.FirstSeenAt
			}
			if d.LastSeenAt.After(result[idx].LastSeenAt) {
				result[idx].LastSeenAt = d.LastSeenAt
			}
			if result[idx].UserEmail == "" {
				result[idx].UserEmail = d.UserEmail
			}
			if result[idx].UserUsername == "" {
				result[idx].UserUsername = d.UserUsername
			}
			continue
		}
		merged[key] = len(result)
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

func (r *ipSecurityRepository) GetUserIPPinState(ctx context.Context, userID int64) (*service.UserIPPinState, error) {
	var state service.UserIPPinState
	var enabledAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(pin_known_ips,FALSE), pin_known_ips_enabled_at,
       COALESCE(ips,'{}'::text[]), COALESCE(ip_history_saturated,FALSE)
FROM users WHERE id=$1`, userID).Scan(&state.Pinned, &enabledAt, pq.Array(&state.IPs), &state.Saturated)
	if err != nil {
		return nil, err
	}
	if enabledAt.Valid {
		t := enabledAt.Time
		state.EnabledAt = &t
	}
	return &state, nil
}

func (r *ipSecurityRepository) SetUserIPPin(ctx context.Context, userID int64, pinned bool, ips []string, enabledAt *time.Time) error {
	if ips == nil {
		ips = []string{}
	}
	if len(ips) > 256 {
		ips = ips[:256]
	}
	saturated := len(ips) >= 256
	_, err := r.db.ExecContext(ctx, `
UPDATE users
SET pin_known_ips=$2,
    pin_known_ips_enabled_at=$3,
    ips=$4,
    ip_history_saturated=$5
WHERE id=$1`, userID, pinned, enabledAt, pq.Array(ips), saturated)
	return err
}

func (r *ipSecurityRepository) AppendUserAllowedIP(ctx context.Context, userID int64, ipAddr string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var ips []string
	var saturated bool
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(ips,'{}'::text[]), COALESCE(ip_history_saturated,FALSE)
FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(pq.Array(&ips), &saturated); err != nil {
		return err
	}
	for _, existing := range ips {
		if existing == ipAddr {
			return tx.Commit()
		}
	}
	if saturated || len(ips) >= 256 {
		return fmt.Errorf("user ip history is saturated")
	}
	ips = append(ips, ipAddr)
	saturated = len(ips) >= 256
	if _, err := tx.ExecContext(ctx, `
UPDATE users SET ips=$2, ip_history_saturated=$3 WHERE id=$1`, userID, pq.Array(ips), saturated); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *ipSecurityRepository) ListUsageIPsForUser(ctx context.Context, userID int64, since time.Time) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT DISTINCT ip_address
FROM usage_logs
WHERE user_id=$1 AND created_at >= $2
  AND ip_address IS NOT NULL AND ip_address <> ''
ORDER BY ip_address`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var ipAddr string
		if err := rows.Scan(&ipAddr); err != nil {
			return nil, err
		}
		out = append(out, ipAddr)
	}
	return out, rows.Err()
}

func (r *ipSecurityRepository) ListUserIPSummary(ctx context.Context, userID int64, since time.Time) (*service.UserIPSummary, error) {
	pinState, err := r.GetUserIPPinState(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT ul.ip_address, COUNT(*)::bigint, COALESCE(SUM(ul.total_cost),0)::float8,
       MIN(ul.created_at), MAX(ul.created_at)
FROM usage_logs ul
WHERE ul.user_id=$1 AND ul.created_at >= $2
  AND ul.ip_address IS NOT NULL AND ul.ip_address <> ''
GROUP BY ul.ip_address
ORDER BY COUNT(*) DESC, MAX(ul.created_at) DESC`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := &service.UserIPSummary{
		UserID:      userID,
		PinKnownIPs: pinState.Pinned,
		Items:       []service.UserIPSummaryItem{},
	}
	var ips []string
	for rows.Next() {
		var item service.UserIPSummaryItem
		if err := rows.Scan(&item.IPAddress, &item.RequestCount, &item.TotalCost, &item.FirstSeenAt, &item.LastSeenAt); err != nil {
			return nil, err
		}
		item.BanStatus = "normal"
		item.SharedUsers = []service.UserIPSharedUser{}
		summary.Items = append(summary.Items, item)
		ips = append(ips, item.IPAddress)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(summary.Items) == 0 {
		return summary, nil
	}
	summary.Items[0].IsTop = true
	summary.TopIP = summary.Items[0].IPAddress

	banRows, err := r.db.QueryContext(ctx, `
SELECT id, ip_address, status, COALESCE(reason,'')
FROM ip_security_bans
WHERE ip_address = ANY($1)`, pq.Array(ips))
	if err != nil {
		return nil, err
	}
	defer banRows.Close()
	banByIP := map[string]struct {
		id     int64
		status string
		reason string
	}{}
	for banRows.Next() {
		var id int64
		var ipAddr, status, reason string
		if err := banRows.Scan(&id, &ipAddr, &status, &reason); err != nil {
			return nil, err
		}
		banByIP[ipAddr] = struct {
			id     int64
			status string
			reason string
		}{id: id, status: status, reason: reason}
	}
	if err := banRows.Err(); err != nil {
		return nil, err
	}

	sharedRows, err := r.db.QueryContext(ctx, `
SELECT ul.ip_address, u.id, COALESCE(u.email,''), COUNT(*)::bigint
FROM usage_logs ul
JOIN users u ON u.id = ul.user_id
WHERE ul.ip_address = ANY($1)
  AND ul.created_at >= $2
  AND ul.user_id <> $3
  AND ul.ip_address IS NOT NULL AND ul.ip_address <> ''
GROUP BY ul.ip_address, u.id, u.email
ORDER BY ul.ip_address, COUNT(*) DESC`, pq.Array(ips), since, userID)
	if err != nil {
		return nil, err
	}
	defer sharedRows.Close()
	sharedByIP := map[string][]service.UserIPSharedUser{}
	for sharedRows.Next() {
		var ipAddr, email string
		var id, reqs int64
		if err := sharedRows.Scan(&ipAddr, &id, &email, &reqs); err != nil {
			return nil, err
		}
		list := sharedByIP[ipAddr]
		if len(list) >= 20 {
			continue
		}
		sharedByIP[ipAddr] = append(list, service.UserIPSharedUser{ID: id, Email: email, RequestCount: reqs})
	}
	if err := sharedRows.Err(); err != nil {
		return nil, err
	}

	for i := range summary.Items {
		if ban, ok := banByIP[summary.Items[i].IPAddress]; ok {
			summary.Items[i].BanID = ban.id
			summary.Items[i].BanStatus = ban.status
			summary.Items[i].BanReason = ban.reason
		}
		if shared, ok := sharedByIP[summary.Items[i].IPAddress]; ok {
			summary.Items[i].SharedUsers = shared
		}
	}
	return summary, nil
}
