package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math"
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

const affiliateUserOverviewSQL = `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ua.aff_code,
       COALESCE(ua.aff_rebate_rate_percent, 0)::double precision,
       (ua.aff_rebate_rate_percent IS NOT NULL) AS has_custom_rate,
       ua.aff_count,
       COALESCE(rebated.rebated_invitee_count, 0),
       (ua.aff_quota + COALESCE(matured.matured_frozen_quota, 0))::double precision,
       ua.aff_history_quota::double precision
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id
LEFT JOIN (
    SELECT user_id, COUNT(DISTINCT source_user_id)::integer AS rebated_invitee_count
    FROM user_affiliate_ledger
    WHERE action = 'accrue' AND source_user_id IS NOT NULL
    GROUP BY user_id
) rebated ON rebated.user_id = ua.user_id
LEFT JOIN (
    SELECT user_id, COALESCE(SUM(amount), 0)::double precision AS matured_frozen_quota
    FROM user_affiliate_ledger
    WHERE action = 'accrue' AND frozen_until IS NOT NULL AND frozen_until <= NOW()
    GROUP BY user_id
) matured ON matured.user_id = ua.user_id
WHERE ua.user_id = $1
LIMIT 1`

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
	if input.InviterID <= 0 || input.InviteeUserID <= 0 || input.Amount <= 0 {
		return 0, nil
	}

	var appliedAmount float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var historyQuota float64
		err := scanSingleRow(txCtx, txClient, `
SELECT aff_history_quota::double precision
FROM user_affiliates
WHERE user_id = $1
FOR UPDATE`, []any{input.InviterID}, &historyQuota)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrUserNotFound
			}
			return fmt.Errorf("lock affiliate inviter: %w", err)
		}

		amount := input.Amount
		if input.RebateCap > 0 {
			remainingCap := input.RebateCap - historyQuota
			if remainingCap <= 0 {
				return nil
			}
			if amount > remainingCap {
				amount = remainingCap
			}
		}
		if amount <= 0 {
			return nil
		}

		if input.PerInviteeCap > 0 {
			inviteeTotal, err := queryAffiliateInviteeNetAccrued(txCtx, txClient, input.InviterID, input.InviteeUserID)
			if err != nil {
				return fmt.Errorf("query affiliate invitee accrued amount: %w", err)
			}
			remainingInviteeCap := input.PerInviteeCap - inviteeTotal
			if remainingInviteeCap <= 0 {
				return nil
			}
			if amount > remainingInviteeCap {
				amount = remainingInviteeCap
			}
		}
		if amount <= 0 {
			return nil
		}

		if input.SourceOrderID > 0 {
			var alreadyInserted bool
			err := scanSingleRow(txCtx, txClient, `
SELECT EXISTS(
  SELECT 1
  FROM user_affiliate_ledger
  WHERE user_id = $1
    AND source_order_id = $2
    AND action = 'accrue'
)`, []any{input.InviterID, input.SourceOrderID}, &alreadyInserted)
			if err != nil {
				return fmt.Errorf("check affiliate order ledger duplicate: %w", err)
			}
			if alreadyInserted {
				return nil
			}
		}

		inviteeSlotClaimed := false
		{
			var alreadyClaimed bool
			err := scanSingleRow(txCtx, txClient, `
SELECT EXISTS(
  SELECT 1
  FROM user_affiliate_ledger
  WHERE user_id = $1
    AND source_user_id = $2
    AND action = 'accrue'
    AND invitee_slot_claimed = true
)`, []any{input.InviterID, input.InviteeUserID}, &alreadyClaimed)
			if err != nil {
				return fmt.Errorf("check affiliate invitee slot: %w", err)
			}
			if !alreadyClaimed {
				if input.InviteeLimit > 0 {
					var rebatedCount int
					err := scanSingleRow(txCtx, txClient, `
SELECT COUNT(DISTINCT source_user_id)
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'accrue'
  AND invitee_slot_claimed = true
  AND source_user_id IS NOT NULL`, []any{input.InviterID}, &rebatedCount)
					if err != nil {
						return fmt.Errorf("count affiliate invitee slots: %w", err)
					}
					if rebatedCount >= input.InviteeLimit {
						return nil
					}
				}
				inviteeSlotClaimed = true
			}
		}

		var sourceOrderID any
		if input.SourceOrderID > 0 {
			sourceOrderID = input.SourceOrderID
		}

		insertSQL := `
INSERT INTO user_affiliate_ledger (
    user_id,
    action,
    amount,
    source_user_id,
    source_order_id,
    base_amount,
    rebate_rate,
    invitee_slot_claimed,
    created_at,
    updated_at`
		if input.FreezeHours > 0 {
			insertSQL += `,
    frozen_until`
		}
		insertSQL += `
)
VALUES ($1, 'accrue', $2, $3, $4, $5, $6, $7, NOW(), NOW()`
		if input.FreezeHours > 0 {
			insertSQL += `, NOW() + make_interval(hours => $8)`
		}
		insertSQL += `)`

		args := []any{input.InviterID, amount, input.InviteeUserID, sourceOrderID, input.BaseAmount, input.RebateRate, inviteeSlotClaimed}
		if input.FreezeHours > 0 {
			args = append(args, input.FreezeHours)
		}
		if _, execErr := txClient.ExecContext(txCtx, insertSQL, args...); execErr != nil {
			return fmt.Errorf("insert affiliate accrue ledger: %w", execErr)
		}

		updateSQL := "UPDATE user_affiliates SET aff_quota = aff_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2"
		if input.FreezeHours > 0 {
			updateSQL = "UPDATE user_affiliates SET aff_frozen_quota = aff_frozen_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2"
		}
		res, err := txClient.ExecContext(txCtx, updateSQL, amount, input.InviterID)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}

		appliedAmount = amount
		return nil
	})
	if err != nil {
		return 0, err
	}
	return appliedAmount, nil
}

func (r *affiliateRepository) GetAccruedRebateFromInvitee(ctx context.Context, inviterID, inviteeUserID int64) (float64, error) {
	client := clientFromContext(ctx, r.client)
	total, err := queryAffiliateInviteeNetAccrued(ctx, client, inviterID, inviteeUserID)
	if err != nil {
		return 0, fmt.Errorf("query accrued rebate from invitee: %w", err)
	}
	return total, nil
}

func queryAffiliateInviteeNetAccrued(ctx context.Context, client affiliateQueryExecer, inviterID, inviteeUserID int64) (float64, error) {
	rows, err := client.QueryContext(ctx, `
SELECT GREATEST(COALESCE(SUM(amount), 0), 0)::double precision
FROM user_affiliate_ledger
WHERE user_id = $1
  AND source_user_id = $2
  AND action IN ('accrue', 'reverse')`, inviterID, inviteeUserID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	var total float64
	if rows.Next() {
		if err := rows.Scan(&total); err != nil {
			return 0, err
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return total, rows.Close()
}

// ReverseQuotaForOrder claws back affiliate rebate previously accrued for an
// order that is being refunded.
//
// Idempotency / partial-refund correctness:
//   - We sum the original accrual(s) for the order (action='accrue').
//   - We sum what has already been clawed back (action='reverse').
//   - target = round(accrued * RefundedAmount / OrderAmount); the caller passes
//     the *cumulative* refunded amount, so target is monotonic.
//   - We only reverse the delta (target - alreadyReversed). Retries with the same
//     cumulative refund amount compute a non-positive delta and become no-ops.
//
// Deduction cascade against the inviter's balances, in order:
//  1. aff_frozen_quota (rebate still in freeze window)
//  2. aff_quota (matured but not yet transferred)
//  3. users.balance (already transferred to spendable balance) — may go negative,
//     mirroring DeductBalance semantics, so the arbitrage cannot be cashed out.
//
// A single 'reverse' ledger row records the amount actually clawed back this call
// (negative amount) along with the inviter as user_id and the invitee as
// source_user_id, so reporting and future delta math stay consistent.
func (r *affiliateRepository) ReverseQuotaForOrder(ctx context.Context, input service.AffiliateReversalInput) (float64, int64, error) {
	if input.SourceOrderID <= 0 || input.RefundedAmount <= 0 || input.OrderAmount <= 0 {
		return 0, 0, nil
	}

	var reversedNow float64
	var reversedInviterID int64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		// Identify the inviter (user_id) and invitee (source_user_id) plus the
		// total originally accrued for this order. The serialization point is the
		// FOR UPDATE lock on the inviter's user_affiliates row below (Postgres
		// disallows FOR UPDATE together with GROUP BY); accrual ledger rows are
		// historical/immutable, so aggregating them without a row lock is safe.
		var inviterID, inviteeID sql.NullInt64
		var accruedTotal float64
		err := scanSingleRow(txCtx, txClient, `
SELECT user_id, MAX(source_user_id), COALESCE(SUM(amount), 0)::double precision
FROM user_affiliate_ledger
WHERE source_order_id = $1
  AND action = 'accrue'
GROUP BY user_id
ORDER BY user_id
LIMIT 1`, []any{input.SourceOrderID}, &inviterID, &inviteeID, &accruedTotal)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// No rebate was ever accrued for this order — nothing to reverse.
				return nil
			}
			return fmt.Errorf("lock affiliate accrual for reversal: %w", err)
		}
		if !inviterID.Valid || accruedTotal <= 0 {
			return nil
		}

		// Acquire the inviter's affiliate row lock first. This is the
		// serialization point: concurrent reversals for the same order/inviter
		// block here, so each sees a consistent already-reversed total below and
		// the delta math cannot double-claw.
		var availableQuota, frozenQuota float64
		if err := scanSingleRow(txCtx, txClient, `
SELECT aff_quota::double precision, aff_frozen_quota::double precision
FROM user_affiliates
WHERE user_id = $1
FOR UPDATE`, []any{inviterID.Int64}, &availableQuota, &frozenQuota); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrUserNotFound
			}
			return fmt.Errorf("lock affiliate inviter for reversal: %w", err)
		}

		// Already-reversed total for this order (stored as negative amounts).
		var alreadyReversed float64
		if err := scanSingleRow(txCtx, txClient, `
SELECT COALESCE(SUM(-amount), 0)::double precision
FROM user_affiliate_ledger
WHERE source_order_id = $1
  AND action = 'reverse'`, []any{input.SourceOrderID}, &alreadyReversed); err != nil {
			return fmt.Errorf("query prior affiliate reversal: %w", err)
		}

		// Proportional target reversal based on cumulative refunded amount.
		ratio := input.RefundedAmount / input.OrderAmount
		if ratio > 1 {
			ratio = 1
		}
		target := roundTo8(accruedTotal * ratio)
		toReverse := roundTo8(target - alreadyReversed)
		if toReverse <= 0 {
			return nil
		}
		if total := roundTo8(alreadyReversed + toReverse); total > accruedTotal {
			toReverse = roundTo8(accruedTotal - alreadyReversed)
		}
		if toReverse <= 0 {
			return nil
		}

		remaining := toReverse
		fromFrozen := math.Min(remaining, frozenQuota)
		remaining = roundTo8(remaining - fromFrozen)
		fromAvailable := math.Min(remaining, availableQuota)
		remaining = roundTo8(remaining - fromAvailable)
		fromBalance := remaining // whatever is left was already transferred out

		// Apply quota deductions. aff_history_quota is also reduced so cumulative
		// caps (RebateCap / PerInviteeCap) free up by the reversed amount.
		if _, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_frozen_quota = GREATEST(aff_frozen_quota - $1, 0),
    aff_quota = GREATEST(aff_quota - $2, 0),
    aff_history_quota = GREATEST(aff_history_quota - $3, 0),
    updated_at = NOW()
WHERE user_id = $4`, fromFrozen, fromAvailable, toReverse, inviterID.Int64); err != nil {
			return fmt.Errorf("apply affiliate quota reversal: %w", err)
		}

		// Claw back the already-transferred portion from spendable balance. This
		// may push the balance negative, which is intended: the rebate was
		// converted to balance and must be reclaimed even if spent.
		if fromBalance > 0 {
			if _, err := txClient.User.Update().
				Where(user.IDEQ(inviterID.Int64)).
				AddBalance(-fromBalance).
				Save(txCtx); err != nil {
				return fmt.Errorf("reclaim affiliate balance: %w", err)
			}
		}

		var inviteeArg any
		if inviteeID.Valid {
			inviteeArg = inviteeID.Int64
		}
		if _, err := txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (user_id, action, amount, source_user_id, source_order_id, created_at, updated_at)
VALUES ($1, 'reverse', $2, $3, $4, NOW(), NOW())`,
			inviterID.Int64, -toReverse, inviteeArg, input.SourceOrderID); err != nil {
			return fmt.Errorf("insert affiliate reverse ledger: %w", err)
		}

		reversedNow = toReverse
		reversedInviterID = inviterID.Int64
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return reversedNow, reversedInviterID, nil
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
			currentBalance, err := queryUserBalance(txCtx, txClient, userID)
			if err != nil {
				return err
			}
			balance = currentBalance
			applied = false
			return nil
		}

		updated, err := txClient.User.Update().
			Where(user.IDEQ(userID)).
			AddBalance(amount).
			Save(txCtx)
		if err != nil {
			return fmt.Errorf("credit affiliate signup bonus: %w", err)
		}
		if updated == 0 {
			return service.ErrUserNotFound
		}

		currentBalance, err := queryUserBalance(txCtx, txClient, userID)
		if err != nil {
			return err
		}
		balance = currentBalance

		if _, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliate_ledger
SET balance_after = $2,
    updated_at = NOW()
WHERE user_id = $1
  AND action = 'signup_bonus'`, userID, balance); err != nil {
			return fmt.Errorf("update affiliate signup bonus snapshot: %w", err)
		}

		applied = true
		return nil
	})
	if err != nil {
		return false, 0, err
	}
	return applied, balance, nil
}

func (r *affiliateRepository) CreditCreatorEarnings(ctx context.Context, input service.AISkillCreatorEarningsInput) (float64, error) {
	if input.CreatorUserID <= 0 {
		return 0, service.ErrUserNotFound
	}
	if input.Amount <= 0 || math.IsNaN(input.Amount) || math.IsInf(input.Amount, 0) {
		return 0, nil
	}
	if input.RunID <= 0 {
		return 0, fmt.Errorf("ai skill run id is required for creator earnings")
	}

	amount := roundTo8(input.Amount)
	if amount <= 0 {
		return 0, nil
	}

	var appliedAmount float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, input.CreatorUserID); err != nil {
			return err
		}

		var existingAmount float64
		err := scanSingleRow(txCtx, txClient, `
SELECT amount::double precision
FROM user_affiliate_ledger
WHERE user_id = $1
  AND source_skill_run_id = $2
  AND action = 'creator_earning'
LIMIT 1`, []any{input.CreatorUserID, input.RunID}, &existingAmount)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("check creator earnings duplicate: %w", err)
		}
		if err == nil {
			appliedAmount = existingAmount
			return nil
		}

		var currentQuota, frozenQuota, historyQuota float64
		if err := scanSingleRow(txCtx, txClient, `
SELECT aff_quota::double precision,
       aff_frozen_quota::double precision,
       aff_history_quota::double precision
FROM user_affiliates
WHERE user_id = $1
FOR UPDATE`, []any{input.CreatorUserID}, &currentQuota, &frozenQuota, &historyQuota); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return service.ErrUserNotFound
			}
			return fmt.Errorf("lock affiliate creator for earnings: %w", err)
		}

		err = scanSingleRow(txCtx, txClient, `
SELECT amount::double precision
FROM user_affiliate_ledger
WHERE user_id = $1
  AND source_skill_run_id = $2
  AND action = 'creator_earning'
LIMIT 1`, []any{input.CreatorUserID, input.RunID}, &existingAmount)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("recheck creator earnings duplicate: %w", err)
		}
		if err == nil {
			appliedAmount = existingAmount
			return nil
		}

		quotaAfter := roundTo8(currentQuota + amount)
		historyAfter := roundTo8(historyQuota + amount)
		res, err := txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (
    user_id,
    action,
    amount,
    source_user_id,
    source_skill_run_id,
    aff_quota_after,
    aff_frozen_quota_after,
    aff_history_quota_after,
    created_at,
    updated_at
)
VALUES ($1, 'creator_earning', $2, $3, $4, $5, $6, $7, NOW(), NOW())
ON CONFLICT DO NOTHING`,
			input.CreatorUserID,
			amount,
			nullablePositiveInt64(input.BuyerUserID),
			input.RunID,
			quotaAfter,
			frozenQuota,
			historyAfter,
		)
		if err != nil {
			return fmt.Errorf("insert creator earnings ledger: %w", err)
		}
		inserted, _ := res.RowsAffected()
		if inserted == 0 {
			if err := scanSingleRow(txCtx, txClient, `
SELECT amount::double precision
FROM user_affiliate_ledger
WHERE user_id = $1
  AND source_skill_run_id = $2
  AND action = 'creator_earning'
LIMIT 1`, []any{input.CreatorUserID, input.RunID}, &appliedAmount); err != nil {
				return fmt.Errorf("query creator earnings duplicate after conflict: %w", err)
			}
			return nil
		}

		res, err = txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_quota = aff_quota + $1,
    aff_history_quota = aff_history_quota + $1,
    updated_at = NOW()
WHERE user_id = $2`, amount, input.CreatorUserID)
		if err != nil {
			return fmt.Errorf("credit creator affiliate quota: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}

		appliedAmount = amount
		return nil
	})
	if err != nil {
		return 0, err
	}
	return appliedAmount, nil
}

func (r *affiliateRepository) CountRebatedInvitees(ctx context.Context, inviterID int64) (int, error) {
	if inviterID <= 0 {
		return 0, nil
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT COUNT(DISTINCT source_user_id)
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'accrue'
  AND invitee_slot_claimed = true
  AND source_user_id IS NOT NULL`, inviterID)
	if err != nil {
		return 0, fmt.Errorf("count rebated invitees: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var count int
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *affiliateRepository) ThawFrozenQuota(ctx context.Context, userID int64) (float64, error) {
	var thawed float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		var err error
		thawed, err = thawFrozenQuotaTx(txCtx, txClient, userID)
		return err
	})
	return thawed, err
}

// thawFrozenQuotaTx moves matured frozen quota to available quota within an existing tx.
func thawFrozenQuotaTx(txCtx context.Context, txClient *dbent.Client, userID int64) (float64, error) {
	rows, err := txClient.QueryContext(txCtx, `
WITH matured AS (
    UPDATE user_affiliate_ledger
    SET frozen_until = NULL, updated_at = NOW()
    WHERE user_id = $1
      AND frozen_until IS NOT NULL
      AND frozen_until <= NOW()
    RETURNING amount
)
SELECT COALESCE(SUM(amount), 0) FROM matured`, userID)
	if err != nil {
		return 0, fmt.Errorf("thaw frozen quota: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var thawed float64
	if rows.Next() {
		if err := rows.Scan(&thawed); err != nil {
			return 0, err
		}
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if thawed <= 0 {
		return 0, nil
	}

	_, err = txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_quota = aff_quota + $1,
    aff_frozen_quota = GREATEST(aff_frozen_quota - $1, 0),
    updated_at = NOW()
WHERE user_id = $2`, thawed, userID)
	if err != nil {
		return 0, fmt.Errorf("move thawed quota: %w", err)
	}
	return thawed, nil
}

func (r *affiliateRepository) TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error) {
	var transferred float64
	var newBalance float64

	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}

		// Thaw any matured frozen quota before transfer.
		if _, err := thawFrozenQuotaTx(txCtx, txClient, userID); err != nil {
			return fmt.Errorf("thaw before transfer: %w", err)
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
			AddTotalRecharged(transferred).
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

		snapshot, err := queryAffiliateTransferSnapshot(txCtx, txClient, userID)
		if err != nil {
			return err
		}

		if _, err = txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (
    user_id,
    action,
    amount,
    source_user_id,
    balance_after,
    aff_quota_after,
    aff_frozen_quota_after,
    aff_history_quota_after,
    created_at,
    updated_at
)
VALUES ($1, 'transfer', $2, NULL, $3, $4, $5, $6, NOW(), NOW())`,
			userID,
			transferred,
			snapshot.BalanceAfter,
			snapshot.AvailableQuotaAfter,
			snapshot.FrozenQuotaAfter,
			snapshot.HistoryQuotaAfter,
		); err != nil {
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
       COALESCE(SUM(CASE WHEN COALESCE(ual.base_amount, 0) > 0 THEN ual.base_amount ELSE COALESCE(po.amount, 0) END), 0)::double precision AS total_consumed,
       COALESCE(SUM(ual.amount), 0)::double precision AS total_rebate,
       COALESCE(BOOL_OR(COALESCE(ual.invitee_slot_claimed, false)), false) AS rebate_slot_claimed
FROM user_affiliates ua
LEFT JOIN users u ON u.id = ua.user_id
LEFT JOIN user_affiliate_ledger ual
       ON ual.user_id = $1
      AND ual.source_user_id = ua.user_id
      AND ual.action = 'accrue'
LEFT JOIN payment_orders po ON po.id = ual.source_order_id
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
	if inviterID <= 0 || inviteeUserID <= 0 {
		return []service.AffiliateLedgerEntry{}, nil
	}
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
ORDER BY created_at DESC, id DESC
LIMIT $3`, inviterID, inviteeUserID, limit)
	if err != nil {
		return nil, fmt.Errorf("list invitee ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateLedgerEntry, 0, limit)
	for rows.Next() {
		var item service.AffiliateLedgerEntry
		var sourceUserID sql.NullInt64
		var sourceOrderID sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&item.CreatedAt,
			&item.Action,
			&item.Amount,
			&sourceUserID,
			&sourceOrderID,
			&item.BaseAmount,
			&item.RebateRate,
			&item.InviteeSlotClaimed,
		); err != nil {
			return nil, err
		}
		if sourceUserID.Valid {
			item.SourceUserID = &sourceUserID.Int64
		}
		if sourceOrderID.Valid {
			item.SourceOrderID = &sourceOrderID.Int64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *affiliateRepository) ListAdminAffiliateStats(ctx context.Context, params service.AdminAffiliateListParams) ([]service.AdminAffiliateStatsRow, int64, error) {
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	client := clientFromContext(ctx, r.client)

	whereParts := []string{"1=1"}
	args := make([]any, 0, 8)
	if search := strings.TrimSpace(params.Search); search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		idx := len(args)
		whereParts = append(whereParts, fmt.Sprintf(`(
LOWER(COALESCE(u.email, '')) LIKE $%d OR
LOWER(COALESCE(u.username, '')) LIKE $%d OR
LOWER(COALESCE(ua.aff_code, '')) LIKE $%d OR
ua.user_id::text LIKE $%d
)`, idx, idx, idx, idx))
	}
	if params.StartAt != nil {
		args = append(args, params.StartAt.UTC())
		whereParts = append(whereParts, fmt.Sprintf("ua.created_at >= $%d", len(args)))
	}
	if params.EndAt != nil {
		args = append(args, params.EndAt.UTC())
		whereParts = append(whereParts, fmt.Sprintf("ua.created_at <= $%d", len(args)))
	}
	whereClause := "WHERE " + strings.Join(whereParts, " AND ")

	countRows, err := client.QueryContext(ctx, `
SELECT COUNT(*)
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id
`+whereClause, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("count admin affiliate stats: %w", err)
	}
	defer func() { _ = countRows.Close() }()

	var total int64
	if countRows.Next() {
		if err := countRows.Scan(&total); err != nil {
			return nil, 0, err
		}
	}
	if err := countRows.Err(); err != nil {
		return nil, 0, err
	}

	periodInviteFilter := ""
	periodRebateFilter := ""
	periodArgs := make([]any, 0, 4)
	if params.StartAt != nil {
		periodArgs = append(periodArgs, params.StartAt.UTC())
		periodInviteFilter += fmt.Sprintf(" AND ua2.created_at >= $%d", len(args)+len(periodArgs))
		periodRebateFilter += fmt.Sprintf(" AND ual.created_at >= $%d", len(args)+len(periodArgs))
	}
	if params.EndAt != nil {
		periodArgs = append(periodArgs, params.EndAt.UTC())
		periodInviteFilter += fmt.Sprintf(" AND ua2.created_at <= $%d", len(args)+len(periodArgs))
		periodRebateFilter += fmt.Sprintf(" AND ual.created_at <= $%d", len(args)+len(periodArgs))
	}

	queryArgs := append(append([]any{}, args...), periodArgs...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	limitIdx := len(queryArgs) - 1
	offsetIdx := len(queryArgs)

	rows, err := client.QueryContext(ctx, `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       COALESCE(ua.aff_code, ''),
       ua.inviter_id,
       ua.aff_count,
       ua.aff_quota::double precision,
       ua.aff_history_quota::double precision,
       COALESCE(rebated.rebated_invitee_count, 0),
       COALESCE(period_invites.period_invited_count, 0),
       COALESCE(period_rebate.period_rebate_amount, 0)::double precision,
       ua.created_at,
       ua.updated_at
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id
LEFT JOIN LATERAL (
    SELECT COUNT(DISTINCT ual.source_user_id)::integer AS rebated_invitee_count
    FROM user_affiliate_ledger ual
    WHERE ual.user_id = ua.user_id
      AND ual.action = 'accrue'
      AND ual.invitee_slot_claimed = true
      AND ual.source_user_id IS NOT NULL
) rebated ON true
LEFT JOIN LATERAL (
    SELECT COUNT(*)::integer AS period_invited_count
    FROM user_affiliates ua2
    WHERE ua2.inviter_id = ua.user_id
`+periodInviteFilter+`
) period_invites ON true
LEFT JOIN LATERAL (
    SELECT COALESCE(SUM(ual.amount), 0) AS period_rebate_amount
    FROM user_affiliate_ledger ual
    WHERE ual.user_id = ua.user_id
      AND ual.action = 'accrue'
`+periodRebateFilter+`
) period_rebate ON true
`+whereClause+`
ORDER BY ua.created_at DESC, ua.user_id DESC
LIMIT $`+fmt.Sprint(limitIdx)+` OFFSET $`+fmt.Sprint(offsetIdx), queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin affiliate stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AdminAffiliateStatsRow, 0, pageSize)
	for rows.Next() {
		var item service.AdminAffiliateStatsRow
		var inviterID sql.NullInt64
		if err := rows.Scan(
			&item.UserID,
			&item.Email,
			&item.Username,
			&item.AffCode,
			&inviterID,
			&item.AffCount,
			&item.AffQuota,
			&item.AffHistoryQuota,
			&item.RebatedInviteeCount,
			&item.PeriodInvitedCount,
			&item.PeriodRebateAmount,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if inviterID.Valid {
			item.InviterID = &inviterID.Int64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) ListAffiliateInviteRecords(ctx context.Context, filter service.AffiliateRecordFilter) ([]service.AffiliateInviteRecord, int64, error) {
	client := clientFromContext(ctx, r.client)
	where, args := buildAffiliateRecordWhere(filter, "ua.created_at", []string{
		"inviter.email", "inviter.username", "invitee.email", "invitee.username",
		"ua.inviter_id::text", "ua.user_id::text", "inviter_aff.aff_code",
	})

	total, err := queryAffiliateRecordCount(ctx, client, `
SELECT COUNT(*)
FROM user_affiliates ua
JOIN users invitee ON invitee.id = ua.user_id
JOIN users inviter ON inviter.id = ua.inviter_id
JOIN user_affiliates inviter_aff ON inviter_aff.user_id = ua.inviter_id
`+where, args...)
	if err != nil {
		return nil, 0, err
	}

	orderBy := buildAffiliateRecordOrderBy(filter, map[string]string{
		"inviter":      "inviter.email",
		"invitee":      "invitee.email",
		"aff_code":     "inviter_aff.aff_code",
		"total_rebate": "total_rebate",
		"created_at":   "ua.created_at",
	}, "ua.created_at")
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := client.QueryContext(ctx, `
SELECT ua.inviter_id,
       COALESCE(inviter.email, ''),
       COALESCE(inviter.username, ''),
       ua.user_id,
       COALESCE(invitee.email, ''),
       COALESCE(invitee.username, ''),
       COALESCE(inviter_aff.aff_code, ''),
       COALESCE(SUM(ual.amount), 0)::double precision AS total_rebate,
       ua.created_at
FROM user_affiliates ua
JOIN users invitee ON invitee.id = ua.user_id
JOIN users inviter ON inviter.id = ua.inviter_id
JOIN user_affiliates inviter_aff ON inviter_aff.user_id = ua.inviter_id
LEFT JOIN user_affiliate_ledger ual
       ON ual.user_id = ua.inviter_id
      AND ual.source_user_id = ua.user_id
      AND ual.action = 'accrue'
`+where+`
GROUP BY ua.inviter_id, inviter.email, inviter.username, ua.user_id, invitee.email, invitee.username, inviter_aff.aff_code, ua.created_at
`+orderBy+`
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateInviteRecord, 0)
	for rows.Next() {
		var item service.AffiliateInviteRecord
		if err := rows.Scan(
			&item.InviterID,
			&item.InviterEmail,
			&item.InviterUsername,
			&item.InviteeID,
			&item.InviteeEmail,
			&item.InviteeUsername,
			&item.AffCode,
			&item.TotalRebate,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) ListAffiliateRebateRecords(ctx context.Context, filter service.AffiliateRecordFilter) ([]service.AffiliateRebateRecord, int64, error) {
	client := clientFromContext(ctx, r.client)
	where, args := buildAffiliateRecordWhere(filter, "ual.created_at", []string{
		"inviter.email", "inviter.username", "invitee.email", "invitee.username",
		"po.id::text", "po.out_trade_no", "po.payment_type", "po.status",
	})
	baseJoin := `
FROM user_affiliate_ledger ual
JOIN payment_orders po ON po.id = ual.source_order_id
JOIN users invitee ON invitee.id = ual.source_user_id
JOIN users inviter ON inviter.id = ual.user_id
WHERE ual.action = 'accrue'
  AND ual.source_order_id IS NOT NULL`
	if where != "" {
		where = strings.Replace(where, "WHERE ", " AND ", 1)
	}

	total, err := queryAffiliateRecordCount(ctx, client, "SELECT COUNT(*) "+baseJoin+where, args...)
	if err != nil {
		return nil, 0, err
	}

	orderBy := buildAffiliateRecordOrderBy(filter, map[string]string{
		"order":         "po.id",
		"inviter":       "inviter.email",
		"invitee":       "invitee.email",
		"order_amount":  "po.amount",
		"pay_amount":    "po.pay_amount",
		"rebate_amount": "ual.amount",
		"payment_type":  "po.payment_type",
		"order_status":  "po.status",
		"created_at":    "ual.created_at",
	}, "ual.created_at")
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := client.QueryContext(ctx, `
SELECT po.id,
       po.out_trade_no,
       ual.user_id,
       COALESCE(inviter.email, ''),
       COALESCE(inviter.username, ''),
       ual.source_user_id,
       COALESCE(invitee.email, ''),
       COALESCE(invitee.username, ''),
       po.amount::double precision,
       po.pay_amount::double precision,
       ual.amount::double precision,
       po.payment_type,
       po.status,
       ual.created_at
`+baseJoin+where+`
`+orderBy+`
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateRebateRecord, 0)
	for rows.Next() {
		var item service.AffiliateRebateRecord
		if err := rows.Scan(
			&item.OrderID,
			&item.OutTradeNo,
			&item.InviterID,
			&item.InviterEmail,
			&item.InviterUsername,
			&item.InviteeID,
			&item.InviteeEmail,
			&item.InviteeUsername,
			&item.OrderAmount,
			&item.PayAmount,
			&item.RebateAmount,
			&item.PaymentType,
			&item.OrderStatus,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) ListAffiliateTransferRecords(ctx context.Context, filter service.AffiliateRecordFilter) ([]service.AffiliateTransferRecord, int64, error) {
	client := clientFromContext(ctx, r.client)
	where, args := buildAffiliateRecordWhere(filter, "ual.created_at", []string{
		"u.email", "u.username", "u.id::text",
	})
	baseJoin := `
FROM user_affiliate_ledger ual
JOIN users u ON u.id = ual.user_id
WHERE ual.action = 'transfer'`
	if where != "" {
		where = strings.Replace(where, "WHERE ", " AND ", 1)
	}

	total, err := queryAffiliateRecordCount(ctx, client, "SELECT COUNT(*) "+baseJoin+where, args...)
	if err != nil {
		return nil, 0, err
	}

	orderBy := buildAffiliateRecordOrderBy(filter, map[string]string{
		"user":                  "u.email",
		"amount":                "ual.amount",
		"balance_after":         "ual.balance_after",
		"available_quota_after": "ual.aff_quota_after",
		"frozen_quota_after":    "ual.aff_frozen_quota_after",
		"history_quota_after":   "ual.aff_history_quota_after",
		"created_at":            "ual.created_at",
	}, "ual.created_at")
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := client.QueryContext(ctx, `
SELECT ual.id,
       ual.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ual.amount::double precision,
       ual.balance_after::double precision,
       ual.aff_quota_after::double precision,
       ual.aff_frozen_quota_after::double precision,
       ual.aff_history_quota_after::double precision,
       ual.created_at
`+baseJoin+where+`
`+orderBy+`
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AffiliateTransferRecord, 0)
	for rows.Next() {
		var item service.AffiliateTransferRecord
		var balanceAfter sql.NullFloat64
		var availableQuotaAfter sql.NullFloat64
		var frozenQuotaAfter sql.NullFloat64
		var historyQuotaAfter sql.NullFloat64
		if err := rows.Scan(
			&item.LedgerID,
			&item.UserID,
			&item.UserEmail,
			&item.Username,
			&item.Amount,
			&balanceAfter,
			&availableQuotaAfter,
			&frozenQuotaAfter,
			&historyQuotaAfter,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		item.BalanceAfter = nullableFloat64Ptr(balanceAfter)
		item.AvailableQuotaAfter = nullableFloat64Ptr(availableQuotaAfter)
		item.FrozenQuotaAfter = nullableFloat64Ptr(frozenQuotaAfter)
		item.HistoryQuotaAfter = nullableFloat64Ptr(historyQuotaAfter)
		item.SnapshotAvailable = balanceAfter.Valid &&
			availableQuotaAfter.Valid &&
			frozenQuotaAfter.Valid &&
			historyQuotaAfter.Valid
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *affiliateRepository) GetAffiliateUserOverview(ctx context.Context, userID int64) (*service.AffiliateUserOverview, error) {
	if userID <= 0 {
		return nil, service.ErrUserNotFound
	}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, affiliateUserOverviewSQL, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrUserNotFound
	}

	var overview service.AffiliateUserOverview
	var customRate float64
	var hasCustomRate bool
	if err := rows.Scan(
		&overview.UserID,
		&overview.Email,
		&overview.Username,
		&overview.AffCode,
		&customRate,
		&hasCustomRate,
		&overview.InvitedCount,
		&overview.RebatedInviteeCount,
		&overview.AvailableQuota,
		&overview.HistoryQuota,
	); err != nil {
		return nil, err
	}
	if hasCustomRate {
		overview.RebateRatePercent = customRate
		overview.RebateRateCustom = true
	}
	return &overview, rows.Err()
}

func buildAffiliateRecordWhere(filter service.AffiliateRecordFilter, timeColumn string, searchColumns []string) (string, []any) {
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if filter.StartAt != nil {
		args = append(args, *filter.StartAt)
		clauses = append(clauses, fmt.Sprintf("%s >= $%d", timeColumn, len(args)))
	}
	if filter.EndAt != nil {
		args = append(args, *filter.EndAt)
		clauses = append(clauses, fmt.Sprintf("%s <= $%d", timeColumn, len(args)))
	}
	search := strings.TrimSpace(filter.Search)
	if search != "" && len(searchColumns) > 0 {
		args = append(args, "%"+strings.ToLower(search)+"%")
		parts := make([]string, 0, len(searchColumns))
		for _, col := range searchColumns {
			parts = append(parts, fmt.Sprintf("LOWER(%s) LIKE $%d", col, len(args)))
		}
		clauses = append(clauses, "("+strings.Join(parts, " OR ")+")")
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func buildAffiliateRecordOrderBy(filter service.AffiliateRecordFilter, sortColumns map[string]string, fallbackColumn string) string {
	column := sortColumns[filter.SortBy]
	if column == "" {
		column = fallbackColumn
	}
	direction := "DESC"
	if !filter.SortDesc {
		direction = "ASC"
	}
	return "ORDER BY " + column + " " + direction + " NULLS LAST"
}

func queryAffiliateRecordCount(ctx context.Context, client affiliateQueryExecer, query string, args ...any) (int64, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var total int64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, rows.Err()
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
       aff_code_custom,
       aff_rebate_rate_percent,
       inviter_id,
       aff_count,
       aff_quota::double precision,
       aff_frozen_quota::double precision,
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
	var rebateRate sql.NullFloat64
	if err := rows.Scan(
		&out.UserID,
		&out.AffCode,
		&out.AffCodeCustom,
		&rebateRate,
		&inviterID,
		&out.AffCount,
		&out.AffQuota,
		&out.AffFrozenQuota,
		&out.AffHistoryQuota,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if inviterID.Valid {
		out.InviterID = &inviterID.Int64
	}
	if rebateRate.Valid {
		v := rebateRate.Float64
		out.AffRebateRatePercent = &v
	}
	return &out, nil
}

func queryAffiliateByCode(ctx context.Context, client affiliateQueryExecer, code string) (*service.AffiliateSummary, error) {
	rows, err := client.QueryContext(ctx, `
SELECT user_id,
       aff_code,
       aff_code_custom,
       aff_rebate_rate_percent,
       inviter_id,
       aff_count,
       aff_quota::double precision,
       aff_frozen_quota::double precision,
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
	var rebateRate sql.NullFloat64
	if err := rows.Scan(
		&out.UserID,
		&out.AffCode,
		&out.AffCodeCustom,
		&rebateRate,
		&inviterID,
		&out.AffCount,
		&out.AffQuota,
		&out.AffFrozenQuota,
		&out.AffHistoryQuota,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if inviterID.Valid {
		out.InviterID = &inviterID.Int64
	}
	if rebateRate.Valid {
		v := rebateRate.Float64
		out.AffRebateRatePercent = &v
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

type affiliateTransferSnapshot struct {
	BalanceAfter        float64
	AvailableQuotaAfter float64
	FrozenQuotaAfter    float64
	HistoryQuotaAfter   float64
}

func queryAffiliateTransferSnapshot(ctx context.Context, client affiliateQueryExecer, userID int64) (*affiliateTransferSnapshot, error) {
	rows, err := client.QueryContext(ctx, `
SELECT u.balance::double precision,
       ua.aff_quota::double precision,
       ua.aff_frozen_quota::double precision,
       ua.aff_history_quota::double precision
FROM users u
JOIN user_affiliates ua ON ua.user_id = u.id
WHERE u.id = $1
LIMIT 1`, userID)
	if err != nil {
		return nil, fmt.Errorf("query affiliate transfer snapshot: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrUserNotFound
	}

	var snapshot affiliateTransferSnapshot
	if err := rows.Scan(
		&snapshot.BalanceAfter,
		&snapshot.AvailableQuotaAfter,
		&snapshot.FrozenQuotaAfter,
		&snapshot.HistoryQuotaAfter,
	); err != nil {
		return nil, err
	}
	return &snapshot, rows.Err()
}

func nullableFloat64Ptr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func nullablePositiveInt64(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

// roundTo8 rounds a monetary amount to 8 decimal places, matching the
// DECIMAL(20,8) precision of the affiliate quota/ledger columns. This avoids
// float drift accumulating across repeated partial-refund reversals.
func roundTo8(v float64) float64 {
	return math.Round(v*1e8) / 1e8
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

// UpdateUserAffCode 改写用户的邀请码（自定义专属邀请码）。
// 唯一性冲突返回 ErrAffiliateCodeTaken。
func (r *affiliateRepository) UpdateUserAffCode(ctx context.Context, userID int64, newCode string) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	code := strings.ToUpper(strings.TrimSpace(newCode))
	if code == "" {
		return service.ErrAffiliateCodeInvalid
	}

	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_code = $1,
    aff_code_custom = true,
    updated_at = NOW()
WHERE user_id = $2`, code, userID)
		if err != nil {
			if isAffiliateUniqueViolation(err) {
				return service.ErrAffiliateCodeTaken
			}
			return fmt.Errorf("update aff_code: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}
		return nil
	})
}

// ResetUserAffCode 把 aff_code 还原为系统随机码，并清除 aff_code_custom 标记。
func (r *affiliateRepository) ResetUserAffCode(ctx context.Context, userID int64) (string, error) {
	if userID <= 0 {
		return "", service.ErrUserNotFound
	}
	var newCode string
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		for i := 0; i < affiliateCodeMaxAttempts; i++ {
			candidate, codeErr := generateAffiliateCode()
			if codeErr != nil {
				return codeErr
			}
			res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_code = $1,
    aff_code_custom = false,
    updated_at = NOW()
WHERE user_id = $2`, candidate, userID)
			if err != nil {
				if isAffiliateUniqueViolation(err) {
					continue
				}
				return fmt.Errorf("reset aff_code: %w", err)
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return service.ErrUserNotFound
			}
			newCode = candidate
			return nil
		}
		return fmt.Errorf("reset aff_code: exhausted attempts")
	})
	if err != nil {
		return "", err
	}
	return newCode, nil
}

// SetUserRebateRate 设置或清除用户专属返利比例。ratePercent==nil 表示清除（沿用全局）。
func (r *affiliateRepository) SetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		// nullableArg lets us use a single UPDATE for both "set value" and
		// "clear" cases — database/sql converts nil interface{} to SQL NULL.
		res, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_rebate_rate_percent = $1,
    updated_at = NOW()
WHERE user_id = $2`, nullableArg(ratePercent), userID)
		if err != nil {
			return fmt.Errorf("set aff_rebate_rate_percent: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}
		return nil
	})
}

// BatchSetUserRebateRate 批量为多个用户设置专属比例（nil 清除）。
func (r *affiliateRepository) BatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		for _, uid := range userIDs {
			if uid <= 0 {
				continue
			}
			if _, err := ensureUserAffiliateWithClient(txCtx, txClient, uid); err != nil {
				return err
			}
		}
		_, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_rebate_rate_percent = $1,
    updated_at = NOW()
WHERE user_id = ANY($2)`, nullableArg(ratePercent), pq.Array(userIDs))
		if err != nil {
			return fmt.Errorf("batch set aff_rebate_rate_percent: %w", err)
		}
		return nil
	})
}

// nullableArg unwraps a *float64 into an interface{} suitable for SQL parameter
// binding: nil pointer → SQL NULL, non-nil → the float value.
func nullableArg(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

// ListUsersWithCustomSettings 列出有专属配置（自定义码或专属比例）的用户。
//
// 单一查询同时处理"无搜索"与"按邮箱/用户名模糊搜索"：
// 空 search 时拼接出的 LIKE 模式为 "%%"，匹配所有行；非空时按 ILIKE 子串匹配。
// 这避免了为两种情况维护两份 SQL 模板。
func (r *affiliateRepository) ListUsersWithCustomSettings(ctx context.Context, filter service.AffiliateAdminFilter) ([]service.AffiliateAdminEntry, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	likePattern := "%" + strings.TrimSpace(filter.Search) + "%"

	const baseFrom = `
FROM user_affiliates ua
JOIN users u ON u.id = ua.user_id
WHERE (ua.aff_code_custom = true OR ua.aff_rebate_rate_percent IS NOT NULL)
  AND (u.email ILIKE $1 OR u.username ILIKE $1)`

	client := clientFromContext(ctx, r.client)

	total, err := scanInt64(ctx, client, "SELECT COUNT(*)"+baseFrom, likePattern)
	if err != nil {
		return nil, 0, fmt.Errorf("count affiliate admin entries: %w", err)
	}

	listQuery := `
SELECT ua.user_id,
       COALESCE(u.email, ''),
       COALESCE(u.username, ''),
       ua.aff_code,
       ua.aff_code_custom,
       ua.aff_rebate_rate_percent,
       ua.aff_count` + baseFrom + `
ORDER BY ua.updated_at DESC
LIMIT $2 OFFSET $3`

	rows, err := client.QueryContext(ctx, listQuery, likePattern, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list affiliate admin entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries := make([]service.AffiliateAdminEntry, 0)
	for rows.Next() {
		var e service.AffiliateAdminEntry
		var rebate sql.NullFloat64
		if err := rows.Scan(&e.UserID, &e.Email, &e.Username, &e.AffCode,
			&e.AffCodeCustom, &rebate, &e.AffCount); err != nil {
			return nil, 0, err
		}
		if rebate.Valid {
			v := rebate.Float64
			e.AffRebateRatePercent = &v
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

// scanInt64 runs a query expected to return a single int64 column (e.g. COUNT).
func scanInt64(ctx context.Context, client affiliateQueryExecer, query string, args ...any) (int64, error) {
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	var v int64
	if err := rows.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}
