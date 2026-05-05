//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func querySingleFloat(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) float64 {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next(), "expected one row")
	var value float64
	require.NoError(t, rows.Scan(&value))
	require.NoError(t, rows.Err())
	return value
}

func querySingleInt(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) int {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next(), "expected one row")
	var value int
	require.NoError(t, rows.Scan(&value))
	require.NoError(t, rows.Err())
	return value
}

func TestAffiliateRepository_TransferQuotaToBalance_UsesClaimedQuotaBeforeClear(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-transfer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      5.5,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFF%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, $3, $3, NOW(), NOW())`, u.ID, affCode, 12.34)
	require.NoError(t, err)

	transferred, balance, err := repo.TransferQuotaToBalance(txCtx, u.ID)
	require.NoError(t, err)
	require.InDelta(t, 12.34, transferred, 1e-9)
	require.InDelta(t, 17.84, balance, 1e-9)

	affQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID)
	require.InDelta(t, 0.0, affQuota, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 17.84, persistedBalance, 1e-9)

	ledgerCount := querySingleInt(t, txCtx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'transfer'", u.ID)
	require.Equal(t, 1, ledgerCount)
}

// TestAffiliateRepository_AccrueQuota_ReusesOuterTransaction guards the
// cross-layer tx propagation invariant: when AccrueQuota is called with a ctx
// that already carries a transaction (via dbent.NewTxContext), repo.withTx
// must reuse that tx rather than opening a nested one. If this invariant
// breaks, AccrueQuota would commit independently and survive a rollback of
// the outer tx, which would violate payment_fulfillment's all-or-nothing
// semantics.
func TestAffiliateRepository_AccrueQuota_ReusesOuterTransaction(t *testing.T) {
	ctx := context.Background()

	outerTx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err, "begin outer tx")
	// Defensive cleanup: if any require.* below fires before the explicit
	// Rollback, this prevents the tx from leaking until container teardown.
	// Rollback is idempotent at the driver level (extra rollback returns an
	// error we ignore).
	t.Cleanup(func() { _ = outerTx.Rollback() })
	client := outerTx.Client()
	txCtx := dbent.NewTxContext(ctx, outerTx)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-invitee-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})

	repo := NewAffiliateRepository(client, integrationDB)
	_, err = repo.EnsureUserAffiliate(txCtx, inviter.ID)
	require.NoError(t, err)
	_, err = repo.EnsureUserAffiliate(txCtx, invitee.ID)
	require.NoError(t, err)

	bound, err := repo.BindInviter(txCtx, invitee.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound, "invitee must bind to inviter")

	applied, err := repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: invitee.ID,
		Amount:        3.5,
	})
	require.NoError(t, err)
	require.InDelta(t, 3.5, applied, 1e-9)

	// Visible inside the outer tx.
	innerQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 3.5, innerQuota, 1e-9)

	// Roll back the outer tx; if AccrueQuota had opened its own inner tx and
	// committed it, the rows would still be visible to the global client.
	require.NoError(t, outerTx.Rollback())

	rows, err := integrationEntClient.QueryContext(ctx,
		"SELECT COUNT(*) FROM user_affiliates WHERE user_id IN ($1, $2)",
		inviter.ID, invitee.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var postRollbackCount int
	require.NoError(t, rows.Scan(&postRollbackCount))
	require.Equal(t, 0, postRollbackCount,
		"AccrueQuota must propagate the outer tx — found persisted rows after rollback")
}

func TestAffiliateRepository_TransferQuotaToBalance_EmptyQuota(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-empty-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      3.21,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFF%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, 0, 0, NOW(), NOW())`, u.ID, affCode)
	require.NoError(t, err)

	transferred, balance, err := repo.TransferQuotaToBalance(txCtx, u.ID)
	require.ErrorIs(t, err, service.ErrAffiliateQuotaEmpty)
	require.InDelta(t, 0.0, transferred, 1e-9)
	require.InDelta(t, 0.0, balance, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 3.21, persistedBalance, 1e-9)
}

func TestAffiliateRepository_ApplySignupBonus_IsIdempotentAcrossMigratedSchema(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-signup-bonus-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      6.25,
		Concurrency:  5,
	})

	applied, balance, err := repo.ApplySignupBonus(txCtx, u.ID, 8.5)
	require.NoError(t, err)
	require.True(t, applied)
	require.InDelta(t, 14.75, balance, 1e-9)

	applied, balance, err = repo.ApplySignupBonus(txCtx, u.ID, 8.5)
	require.NoError(t, err)
	require.False(t, applied)
	require.InDelta(t, 0.0, balance, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 14.75, persistedBalance, 1e-9)

	ledgerCount := querySingleInt(t, txCtx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'signup_bonus'", u.ID)
	require.Equal(t, 1, ledgerCount)
}

func TestAffiliateRepository_AccrueQuota_IdempotentBySourceOrder(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-idempotent-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-idempotent-invitee-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})

	_, err := repo.EnsureUserAffiliate(txCtx, inviter.ID)
	require.NoError(t, err)
	_, err = repo.EnsureUserAffiliate(txCtx, invitee.ID)
	require.NoError(t, err)
	bound, err := repo.BindInviter(txCtx, invitee.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound)

	sourceOrder, err := client.PaymentOrder.Create().
		SetUserID(invitee.ID).
		SetUserEmail(invitee.Email).
		SetUserName(invitee.Username).
		SetAmount(2.25).
		SetPayAmount(2.25).
		SetFeeRate(0).
		SetRechargeCode(fmt.Sprintf("AFFILIATE-SOURCE-%d", time.Now().UnixNano())).
		SetOutTradeNo(fmt.Sprintf("sub2_affiliate_source_%d", time.Now().UnixNano())).
		SetPaymentType("alipay").
		SetPaymentTradeNo("").
		SetOrderType("balance").
		SetStatus("COMPLETED").
		Save(txCtx)
	require.NoError(t, err)

	firstApplied, err := repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: invitee.ID,
		Amount:        2.25,
		SourceOrderID: sourceOrder.ID,
	})
	require.NoError(t, err)
	require.InDelta(t, 2.25, firstApplied, 1e-9)

	secondApplied, err := repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: invitee.ID,
		Amount:        2.25,
		SourceOrderID: sourceOrder.ID,
	})
	require.NoError(t, err)
	require.InDelta(t, 0.0, secondApplied, 1e-9)

	quota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 2.25, quota, 1e-9)

	ledgerCount := querySingleInt(t, txCtx, client, `
SELECT COUNT(*)
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'accrue'
  AND source_order_id = $2`, inviter.ID, sourceOrder.ID)
	require.Equal(t, 1, ledgerCount)
}

func TestAffiliateRepository_EnsureUserAffiliate_ReturnsErrUserNotFoundOnMissingUser(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)
	_, err := repo.EnsureUserAffiliate(txCtx, 9_999_999_999)
	require.True(t, errors.Is(err, service.ErrUserNotFound))
}
