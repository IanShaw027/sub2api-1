package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const (
	affiliateCodeLength      = 12
	affiliateCodeMaxAttempts = 12
)

var affiliateCodeCharset = []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")

type affiliateQueryExecer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type affiliateRepository struct {
	client *dbent.Client
}

func NewAffiliateRepository(client *dbent.Client, _ *sql.DB) service.AffiliateRepository {
	return &affiliateRepository{client: client}
}

func (r *affiliateRepository) EnsureUserAffiliate(ctx context.Context, userID int64) (*service.AffiliateSummary, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	client := clientFromContext(ctx, r.client)
	return ensureUserAffiliateWithClient(ctx, client, userID)
}

func (r *affiliateRepository) GetAffiliateByCode(ctx context.Context, code string) (*service.AffiliateSummary, error) {
	client := clientFromContext(ctx, r.client)
	return queryAffiliateByCode(ctx, client, code)
}

func (r *affiliateRepository) BindInviter(ctx context.Context, userID, inviterID int64) (bool, error) {
	var bound bool
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, inviterID); err != nil {
			return err
		}

		res, err := txClient.ExecContext(txCtx,
			"UPDATE user_affiliates SET inviter_id = $1, updated_at = NOW() WHERE user_id = $2 AND inviter_id IS NULL",
			inviterID, userID,
		)
		if err != nil {
			return fmt.Errorf("bind inviter: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			bound = false
			return nil
		}

		if _, err = txClient.ExecContext(txCtx,
			"UPDATE user_affiliates SET aff_count = aff_count + 1, updated_at = NOW() WHERE user_id = $1",
			inviterID,
		); err != nil {
			return fmt.Errorf("increment inviter aff_count: %w", err)
		}
		bound = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return bound, nil
}

func (r *affiliateRepository) AccrueQuota(ctx context.Context, input service.AffiliateAccrualInput) (float64, error) {
	if input.Amount <= 0 {
		return 0, nil
	}

	var appliedAmount float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var historyQuota float64
		rows, err := txClient.QueryContext(txCtx, `
SELECT aff_history_quota::double precision
FROM user_affiliates
WHERE user_id = $1
FOR UPDATE`, input.InviterID)
		if err != nil {
			return fmt.Errorf("lock affiliate inviter: %w", err)
		}
		if !rows.Next() {
			_ = rows.Close()
			appliedAmount = 0
			return nil
		}
		if err := rows.Scan(&historyQuota); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}

		amount := input.Amount
		if input.RebateCap > 0 {
			remainingCap := input.RebateCap - historyQuota
			if remainingCap <= 0 {
				appliedAmount = 0
				return nil
			}
			if amount > remainingCap {
				amount = remainingCap
			}
		}
		if amount <= 0 {
			appliedAmount = 0
			return nil
		}

		inviteeSlotClaimed := false
		alreadyRows, err := txClient.QueryContext(txCtx, `
SELECT EXISTS(
  SELECT 1
  FROM user_affiliate_ledger
  WHERE user_id = $1
    AND source_user_id = $2
    AND action = 'accrue'
    AND invitee_slot_claimed = true
)`, input.InviterID, input.InviteeUserID)
		if err != nil {
			return fmt.Errorf("check affiliate invitee slot: %w", err)
		}
		var alreadyClaimed bool
		if alreadyRows.Next() {
			if err := alreadyRows.Scan(&alreadyClaimed); err != nil {
				_ = alreadyRows.Close()
				return err
			}
		}
		if err := alreadyRows.Close(); err != nil {
			return err
		}
		if !alreadyClaimed {
			if input.InviteeLimit > 0 {
				countRows, err := txClient.QueryContext(txCtx, `
SELECT COUNT(DISTINCT source_user_id)
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'accrue'
  AND invitee_slot_claimed = true
  AND source_user_id IS NOT NULL`, input.InviterID)
				if err != nil {
					return fmt.Errorf("count affiliate invitee slots: %w", err)
				}
				var rebatedCount int
				if countRows.Next() {
					if err := countRows.Scan(&rebatedCount); err != nil {
						_ = countRows.Close()
						return err
					}
				}
				if err := countRows.Close(); err != nil {
					return err
				}
				if rebatedCount >= input.InviteeLimit {
					appliedAmount = 0
					return nil
				}
			}
			inviteeSlotClaimed = true
		}

		res, err := txClient.ExecContext(txCtx,
			"UPDATE user_affiliates SET aff_quota = aff_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2",
			amount, input.InviterID,
		)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			appliedAmount = 0
			return nil
		}

		var sourceOrderID any
		if input.SourceOrderID > 0 {
			sourceOrderID = input.SourceOrderID
		}
		if _, err = txClient.ExecContext(txCtx, `
	INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, source_order_id, base_amount, rebate_rate, invitee_slot_claimed, created_at, updated_at)
	VALUES ($1, 'accrue', $2, $3, $4, $5, $6, $7, NOW(), NOW())`, input.InviterID, amount, input.InviteeUserID, sourceOrderID, input.BaseAmount, input.RebateRate, inviteeSlotClaimed); err != nil {
			return fmt.Errorf("insert affiliate accrue ledger: %w", err)
		}

		appliedAmount = amount
		return nil
	})
	if err != nil {
		return 0, err
	}
	return appliedAmount, nil
}

func (r *affiliateRepository) ApplySignupBonus(ctx context.Context, userID int64, amount float64) (bool, float64, error) {
	if userID <= 0 || amount <= 0 {
		return false, 0, nil
	}
	var applied bool
	var balance float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		res, err := txClient.ExecContext(txCtx, `
	INSERT INTO user_affiliate_ledger (user_id, action, amount, created_at, updated_at)
	VALUES ($1, 'signup_bonus', $2, NOW(), NOW())
	ON CONFLICT DO NOTHING`, userID, amount)
		if err != nil {
			return fmt.Errorf("insert affiliate signup bonus ledger: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return nil
		}
		rows, err := txClient.QueryContext(txCtx, `
		UPDATE users
		SET balance = balance + $1, updated_at = NOW()
		WHERE id = $2
		RETURNING balance::double precision`, amount, userID)
		if err != nil {
			return fmt.Errorf("apply affiliate signup bonus: %w", err)
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			return service.ErrUserNotFound
		}
		if err := rows.Scan(&balance); err != nil {
			return fmt.Errorf("apply affiliate signup bonus: %w", err)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		applied = true
		return nil
	})
	return applied, balance, err
}

func (r *affiliateRepository) TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error) {
	var transferred float64
	var newBalance float64

	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}

		rows, err := txClient.QueryContext(txCtx, `
WITH claimed AS (
	SELECT aff_quota::double precision AS amount
	FROM user_affiliates
	WHERE user_id = $1
	  AND aff_quota > 0
	FOR UPDATE
),
cleared AS (
	UPDATE user_affiliates ua
	SET aff_quota = 0,
	    updated_at = NOW()
	FROM claimed c
	WHERE ua.user_id = $1
	RETURNING c.amount
)
SELECT amount
FROM cleared`, userID)
		if err != nil {
			return fmt.Errorf("claim affiliate quota: %w", err)
		}

		if !rows.Next() {
			_ = rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
			return service.ErrAffiliateQuotaEmpty
		}
		if err := rows.Scan(&transferred); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if transferred <= 0 {
			return service.ErrAffiliateQuotaEmpty
		}

		affected, err := txClient.User.Update().
			Where(user.IDEQ(userID)).
			AddBalance(transferred).
			Save(txCtx)
		if err != nil {
			return fmt.Errorf("credit user balance by affiliate quota: %w", err)
		}
		if affected == 0 {
			return service.ErrUserNotFound
		}

		newBalance, err = queryUserBalance(txCtx, txClient, userID)
		if err != nil {
			return err
		}

		if _, err = txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, created_at, updated_at)
VALUES ($1, 'transfer', $2, NULL, NOW(), NOW())`, userID, transferred); err != nil {
			return fmt.Errorf("insert affiliate transfer ledger: %w", err)
		}

		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	return transferred, newBalance, nil
}

func (r *affiliateRepository) ListInvitees(ctx context.Context, inviterID int64, limit int) ([]service.AffiliateInvitee, error) {
	if limit <= 0 {
		limit = 100
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
	SELECT ua.user_id,
	       COALESCE(u.email, ''),
	       COALESCE(u.username, ''),
	       ua.created_at,
	       COALESCE(SUM(CASE WHEN l.action = 'accrue' THEN l.base_amount ELSE 0 END), 0)::double precision AS total_consumed,
	       COALESCE(SUM(CASE WHEN l.action = 'accrue' THEN l.amount ELSE 0 END), 0)::double precision AS total_rebate,
	       COALESCE(BOOL_OR(l.invitee_slot_claimed), false) AS rebate_slot_claimed
	FROM user_affiliates ua
	LEFT JOIN users u ON u.id = ua.user_id
	LEFT JOIN user_affiliate_ledger l ON l.user_id = ua.inviter_id AND l.source_user_id = ua.user_id
	WHERE ua.inviter_id = $1
	GROUP BY ua.user_id, u.email, u.username, ua.created_at
	ORDER BY ua.created_at DESC
	LIMIT $2`, inviterID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	invitees := make([]service.AffiliateInvitee, 0)
	for rows.Next() {
		var item service.AffiliateInvitee
		var createdAt time.Time
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &createdAt, &item.TotalConsumed, &item.TotalRebate, &item.RebateSlotClaimed); err != nil {
			return nil, err
		}
		item.CreatedAt = &createdAt
		invitees = append(invitees, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return invitees, nil
}

func (r *affiliateRepository) ListInviteeLedger(ctx context.Context, inviterID, inviteeUserID int64, limit int) ([]service.AffiliateLedgerEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT id,
       created_at,
       action,
       amount::double precision,
       source_user_id,
       source_order_id,
       COALESCE(base_amount, 0)::double precision,
       COALESCE(rebate_rate, 0)::double precision,
       COALESCE(invitee_slot_claimed, false)
FROM user_affiliate_ledger
WHERE user_id = $1
  AND source_user_id = $2
  AND action = 'accrue'
ORDER BY created_at DESC, id DESC
LIMIT $3`, inviterID, inviteeUserID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	entries := make([]service.AffiliateLedgerEntry, 0)
	for rows.Next() {
		var item service.AffiliateLedgerEntry
		var sourceUserID sql.NullInt64
		var sourceOrderID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.CreatedAt, &item.Action, &item.Amount, &sourceUserID, &sourceOrderID, &item.BaseAmount, &item.RebateRate, &item.InviteeSlotClaimed); err != nil {
			return nil, err
		}
		if sourceUserID.Valid {
			item.SourceUserID = &sourceUserID.Int64
		}
		if sourceOrderID.Valid {
			item.SourceOrderID = &sourceOrderID.Int64
		}
		entries = append(entries, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *affiliateRepository) CountRebatedInvitees(ctx context.Context, inviterID int64) (int, error) {
	client := clientFromContext(ctx, r.client)
	var count int
	rows, err := client.QueryContext(ctx, `
SELECT COUNT(DISTINCT source_user_id)
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'accrue'
  AND invitee_slot_claimed = true
  AND source_user_id IS NOT NULL`, inviterID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, nil
	}
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}
	return count, rows.Err()
}

func (r *affiliateRepository) withTx(ctx context.Context, fn func(txCtx context.Context, txClient *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin affiliate transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit affiliate transaction: %w", err)
	}
	return nil
}

func ensureUserAffiliateWithClient(ctx context.Context, client affiliateQueryExecer, userID int64) (*service.AffiliateSummary, error) {
	summary, err := queryAffiliateByUserID(ctx, client, userID)
	if err == nil {
		return summary, nil
	}
	if !errors.Is(err, service.ErrAffiliateProfileNotFound) {
		return nil, err
	}

	for i := 0; i < affiliateCodeMaxAttempts; i++ {
		code, codeErr := generateAffiliateCode()
		if codeErr != nil {
			return nil, codeErr
		}
		_, insertErr := client.ExecContext(ctx, `
INSERT INTO user_affiliates (user_id, aff_code, created_at, updated_at)
VALUES ($1, $2, NOW(), NOW())
ON CONFLICT (user_id) DO NOTHING`, userID, code)
		if insertErr == nil {
			break
		}
		if isAffiliateUniqueViolation(insertErr) {
			continue
		}
		return nil, insertErr
	}

	return queryAffiliateByUserID(ctx, client, userID)
}

func queryAffiliateByUserID(ctx context.Context, client affiliateQueryExecer, userID int64) (*service.AffiliateSummary, error) {
	rows, err := client.QueryContext(ctx, `
SELECT user_id,
       aff_code,
       inviter_id,
       aff_count,
       aff_quota::double precision,
       aff_history_quota::double precision,
       created_at,
       updated_at
FROM user_affiliates
WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAffiliateProfileNotFound
	}

	var out service.AffiliateSummary
	var inviterID sql.NullInt64
	if err := rows.Scan(
		&out.UserID,
		&out.AffCode,
		&inviterID,
		&out.AffCount,
		&out.AffQuota,
		&out.AffHistoryQuota,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if inviterID.Valid {
		out.InviterID = &inviterID.Int64
	}
	return &out, nil
}

func queryAffiliateByCode(ctx context.Context, client affiliateQueryExecer, code string) (*service.AffiliateSummary, error) {
	rows, err := client.QueryContext(ctx, `
SELECT user_id,
       aff_code,
       inviter_id,
       aff_count,
       aff_quota::double precision,
       aff_history_quota::double precision,
       created_at,
       updated_at
FROM user_affiliates
WHERE aff_code = $1
LIMIT 1`, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrAffiliateProfileNotFound
	}

	var out service.AffiliateSummary
	var inviterID sql.NullInt64
	if err := rows.Scan(
		&out.UserID,
		&out.AffCode,
		&inviterID,
		&out.AffCount,
		&out.AffQuota,
		&out.AffHistoryQuota,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if inviterID.Valid {
		out.InviterID = &inviterID.Int64
	}
	return &out, nil
}

func queryUserBalance(ctx context.Context, client affiliateQueryExecer, userID int64) (float64, error) {
	rows, err := client.QueryContext(ctx,
		"SELECT balance::double precision FROM users WHERE id = $1 LIMIT 1",
		userID,
	)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, service.ErrUserNotFound
	}
	var balance float64
	if err := rows.Scan(&balance); err != nil {
		return 0, err
	}
	return balance, nil
}

func generateAffiliateCode() (string, error) {
	buf := make([]byte, affiliateCodeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate affiliate code: %w", err)
	}
	for i := range buf {
		buf[i] = affiliateCodeCharset[int(buf[i])%len(affiliateCodeCharset)]
	}
	return string(buf), nil
}

func isAffiliateUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code) == "23505"
	}
	return false
}
