//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
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

	rows, err := client.QueryContext(txCtx, `
SELECT amount::double precision,
       balance_after::double precision,
       aff_quota_after::double precision,
       aff_frozen_quota_after::double precision,
       aff_history_quota_after::double precision
FROM user_affiliate_ledger
WHERE user_id = $1 AND action = 'transfer'
LIMIT 1`, u.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next(), "expected transfer ledger")
	var amount, balanceAfter, quotaAfter, frozenAfter, historyAfter float64
	require.NoError(t, rows.Scan(&amount, &balanceAfter, &quotaAfter, &frozenAfter, &historyAfter))
	require.InDelta(t, 12.34, amount, 1e-9)
	require.InDelta(t, 17.84, balanceAfter, 1e-9)
	require.InDelta(t, 0.0, quotaAfter, 1e-9)
	require.InDelta(t, 0.0, frozenAfter, 1e-9)
	require.InDelta(t, 12.34, historyAfter, 1e-9)
}

func TestAffiliateRepository_CreditCreatorEarnings_IdempotentBySkillRun(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)
	creatorRepo, ok := repo.(interface {
		CreditCreatorEarnings(context.Context, service.AISkillCreatorEarningsInput) (float64, error)
	})
	require.True(t, ok, "production affiliate repository must support creator earnings")

	creator := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("skill-creator-earnings-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	buyer := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("skill-creator-buyer-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})

	skill, err := client.AISkill.Create().
		SetUserID(creator.ID).
		SetSkillType("prompt_chat").
		SetTitle("Creator earnings fixture").
		SetVisibility("private").
		SetSourceVisibility("public").
		SetBillingMode("per_request").
		SetPrice(10).
		Save(txCtx)
	require.NoError(t, err)

	version, err := client.AISkillVersion.Create().
		SetSkillID(skill.ID).
		SetUserID(creator.ID).
		SetVersion(1).
		SetReviewStatus("approved").
		SetContentFormat("prompt").
		SetRuntime("openai_chat").
		SetSourceContent("say hi").
		Save(txCtx)
	require.NoError(t, err)

	run, err := client.AISkillRun.Create().
		SetSkillID(skill.ID).
		SetVersionID(version.ID).
		SetUserID(buyer.ID).
		SetRunMode("use").
		SetStatus("succeeded").
		SetBillingMode("per_request").
		SetPrice(10).
		Save(txCtx)
	require.NoError(t, err)

	first, err := creatorRepo.CreditCreatorEarnings(txCtx, service.AISkillCreatorEarningsInput{
		CreatorUserID: creator.ID,
		BuyerUserID:   buyer.ID,
		SkillID:       skill.ID,
		VersionID:     version.ID,
		RunID:         run.ID,
		Amount:        7.5,
		Currency:      "credit",
	})
	require.NoError(t, err)
	require.InDelta(t, 7.5, first, 1e-9)

	second, err := creatorRepo.CreditCreatorEarnings(txCtx, service.AISkillCreatorEarningsInput{
		CreatorUserID: creator.ID,
		BuyerUserID:   buyer.ID,
		SkillID:       skill.ID,
		VersionID:     version.ID,
		RunID:         run.ID,
		Amount:        7.5,
		Currency:      "credit",
	})
	require.NoError(t, err)
	require.InDelta(t, 7.5, second, 1e-9)

	quota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", creator.ID)
	require.InDelta(t, 7.5, quota, 1e-9)
	history := querySingleFloat(t, txCtx, client,
		"SELECT aff_history_quota::double precision FROM user_affiliates WHERE user_id = $1", creator.ID)
	require.InDelta(t, 7.5, history, 1e-9)

	ledgerCount := querySingleInt(t, txCtx, client, `
SELECT COUNT(*)
FROM user_affiliate_ledger
WHERE user_id = $1
  AND source_skill_run_id = $2
  AND action = 'creator_earning'`, creator.ID, run.ID)
	require.Equal(t, 1, ledgerCount)

	rows, err := client.QueryContext(txCtx, `
SELECT amount::double precision,
       source_user_id,
       aff_quota_after::double precision,
       aff_history_quota_after::double precision
FROM user_affiliate_ledger
WHERE user_id = $1
  AND source_skill_run_id = $2
  AND action = 'creator_earning'
LIMIT 1`, creator.ID, run.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next(), "expected creator earning ledger")
	var amount, quotaAfter, historyAfter float64
	var sourceUserID int64
	require.NoError(t, rows.Scan(&amount, &sourceUserID, &quotaAfter, &historyAfter))
	require.InDelta(t, 7.5, amount, 1e-9)
	require.Equal(t, buyer.ID, sourceUserID)
	require.InDelta(t, 7.5, quotaAfter, 1e-9)
	require.InDelta(t, 7.5, historyAfter, 1e-9)
	require.NoError(t, rows.Err())
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

func TestAffiliateRepository_AccrueQuota_IdempotentBySourceOrder(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-idem-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-idem-invitee-%d@example.com", time.Now().UnixNano()+1),
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
		SetRechargeCode(fmt.Sprintf("AFFILIATE-IDEM-%d", time.Now().UnixNano())).
		SetOutTradeNo(fmt.Sprintf("sub2_affiliate_idem_%d", time.Now().UnixNano())).
		SetPaymentType("alipay").
		SetPaymentTradeNo("").
		SetOrderType("balance").
		SetStatus("COMPLETED").
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetExpiresAt(time.Now().Add(24 * time.Hour)).
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

func TestAffiliateRepository_AccrueQuota_ClampsByGlobalAndPerInviteeCaps(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-cap-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	inviteeA := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-cap-invitee-a-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	inviteeB := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-cap-invitee-b-%d@example.com", time.Now().UnixNano()+2),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})

	for _, userID := range []int64{inviter.ID, inviteeA.ID, inviteeB.ID} {
		_, err := repo.EnsureUserAffiliate(txCtx, userID)
		require.NoError(t, err)
	}
	bound, err := repo.BindInviter(txCtx, inviteeA.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound)
	bound, err = repo.BindInviter(txCtx, inviteeB.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound)

	applied, err := repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: inviteeA.ID,
		Amount:        8,
		RebateCap:     100,
		PerInviteeCap: 10,
	})
	require.NoError(t, err)
	require.InDelta(t, 8.0, applied, 1e-9)

	applied, err = repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: inviteeA.ID,
		Amount:        8,
		RebateCap:     100,
		PerInviteeCap: 10,
	})
	require.NoError(t, err)
	require.InDelta(t, 2.0, applied, 1e-9)

	applied, err = repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: inviteeB.ID,
		Amount:        95,
		RebateCap:     15,
	})
	require.NoError(t, err)
	require.InDelta(t, 5.0, applied, 1e-9)

	quota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 15.0, quota, 1e-9)
}

func TestAffiliateRepository_AccrueQuota_EnforcesInviteeLimitOnlyForNewSlots(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-slot-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	inviteeA := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-slot-invitee-a-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	inviteeB := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-slot-invitee-b-%d@example.com", time.Now().UnixNano()+2),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})

	for _, userID := range []int64{inviter.ID, inviteeA.ID, inviteeB.ID} {
		_, err := repo.EnsureUserAffiliate(txCtx, userID)
		require.NoError(t, err)
	}
	bound, err := repo.BindInviter(txCtx, inviteeA.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound)
	bound, err = repo.BindInviter(txCtx, inviteeB.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound)

	firstApplied, err := repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: inviteeA.ID,
		Amount:        3,
		InviteeLimit:  1,
	})
	require.NoError(t, err)
	require.InDelta(t, 3.0, firstApplied, 1e-9)

	repeatApplied, err := repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: inviteeA.ID,
		Amount:        2,
		InviteeLimit:  1,
	})
	require.NoError(t, err)
	require.InDelta(t, 2.0, repeatApplied, 1e-9)

	blockedApplied, err := repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: inviteeB.ID,
		Amount:        4,
		InviteeLimit:  1,
	})
	require.NoError(t, err)
	require.InDelta(t, 0.0, blockedApplied, 1e-9)

	slotCount := querySingleInt(t, txCtx, client, `
SELECT COUNT(DISTINCT source_user_id)
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'accrue'
	AND invitee_slot_claimed = true`, inviter.ID)
	require.Equal(t, 1, slotCount)
}

func TestAffiliateRepository_ListInviteesIncludesHistoricalConsumptionAndSlotClaim(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-list-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-list-invitee-%d@example.com", time.Now().UnixNano()+1),
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

	order, err := client.PaymentOrder.Create().
		SetUserID(invitee.ID).
		SetUserEmail(invitee.Email).
		SetUserName(invitee.Username).
		SetAmount(23.5).
		SetPayAmount(23.5).
		SetFeeRate(0).
		SetRechargeCode(fmt.Sprintf("AFFILIATE-LIST-%d", time.Now().UnixNano())).
		SetOutTradeNo(fmt.Sprintf("sub2_affiliate_list_%d", time.Now().UnixNano())).
		SetPaymentType("alipay").
		SetPaymentTradeNo("").
		SetOrderType("balance").
		SetStatus("COMPLETED").
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetExpiresAt(time.Now().Add(24 * time.Hour)).
		Save(txCtx)
	require.NoError(t, err)

	applied, err := repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: invitee.ID,
		Amount:        4.7,
		BaseAmount:    0,
		RebateRate:    20,
		SourceOrderID: order.ID,
		RebateCap:     100,
		InviteeLimit:  1,
	})
	require.NoError(t, err)
	require.InDelta(t, 4.7, applied, 1e-9)

	invitees, err := repo.ListInvitees(txCtx, inviter.ID, 10)
	require.NoError(t, err)
	require.Len(t, invitees, 1)

	got := invitees[0]
	require.Equal(t, invitee.ID, got.UserID)
	require.InDelta(t, 23.5, got.TotalConsumed, 1e-9)
	require.InDelta(t, 4.7, got.TotalRebate, 1e-9)
	require.True(t, got.RebateSlotClaimed)
}

func TestAffiliateMigration132BackfillsHistoricalInviteeSlots(t *testing.T) {
	content, err := os.ReadFile("../../migrations/132_affiliate_policy_limits.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "UPDATE user_affiliate_ledger")
	require.Contains(t, sql, "invitee_slot_claimed = TRUE")
	require.Contains(t, sql, "action = 'accrue'")
	require.Contains(t, sql, "source_user_id IS NOT NULL")
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

// TestAffiliateRepository_AdminCustomCode covers the success path of admin
// invite-code rewrite + reset within a shared test transaction:
// - UpdateUserAffCode replaces aff_code, sets aff_code_custom=true, lookup works
// - the old code can no longer be found
// - ResetUserAffCode reverts aff_code_custom and assigns a new system-format code
//
// The conflict path (duplicate code → ErrAffiliateCodeTaken) lives in its own
// test because a unique-violation aborts the surrounding Postgres tx, which
// would poison subsequent assertions in the same transaction.
func TestAffiliateRepository_AdminCustomCode(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-custom-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	original, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.False(t, original.AffCodeCustom, "system-generated codes start as non-custom")
	originalCode := original.AffCode

	// Rewrite to a custom code
	customCode := fmt.Sprintf("VIP%09d", time.Now().UnixNano()%1_000_000_000)
	require.NoError(t, repo.UpdateUserAffCode(txCtx, u.ID, customCode))

	updated, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, customCode, updated.AffCode)
	require.True(t, updated.AffCodeCustom)

	// Lookup by new custom code finds the user
	byCode, err := repo.GetAffiliateByCode(txCtx, customCode)
	require.NoError(t, err)
	require.Equal(t, u.ID, byCode.UserID)

	// Old system code should no longer match
	_, err = repo.GetAffiliateByCode(txCtx, originalCode)
	require.ErrorIs(t, err, service.ErrAffiliateProfileNotFound)

	// Reset back to a fresh system code, clears custom flag
	newSysCode, err := repo.ResetUserAffCode(txCtx, u.ID)
	require.NoError(t, err)
	require.NotEqual(t, customCode, newSysCode)

	reset, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, newSysCode, reset.AffCode)
	require.False(t, reset.AffCodeCustom)

	// The old custom code is now free again
	_, err = repo.GetAffiliateByCode(txCtx, customCode)
	require.ErrorIs(t, err, service.ErrAffiliateProfileNotFound)
}

// TestAffiliateRepository_AdminCustomCode_Conflict isolates the unique-violation
// path. PostgreSQL aborts the enclosing tx when a unique constraint fires, so
// this test must be the only assertion and run in its own tx — production
// callers each have their own outer tx, so this matches real behavior.
func TestAffiliateRepository_AdminCustomCode_Conflict(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	taker := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-conflict-taker-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	requester := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-conflict-req-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})

	takenCode := fmt.Sprintf("HOT%09d", time.Now().UnixNano()%1_000_000_000)
	require.NoError(t, repo.UpdateUserAffCode(txCtx, taker.ID, takenCode))

	// Now requester tries to grab the same code → conflict.
	err := repo.UpdateUserAffCode(txCtx, requester.ID, takenCode)
	require.ErrorIs(t, err, service.ErrAffiliateCodeTaken)
}

// TestAffiliateRepository_AdminRebateRate covers per-user exclusive rate
// set/clear and the Batch variant including NULL semantics.
func TestAffiliateRepository_AdminRebateRate(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u1 := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rate-%d-a@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})
	u2 := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rate-%d-b@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	// Set exclusive rate for u1
	rate := 42.5
	require.NoError(t, repo.SetUserRebateRate(txCtx, u1.ID, &rate))

	got, err := repo.EnsureUserAffiliate(txCtx, u1.ID)
	require.NoError(t, err)
	require.NotNil(t, got.AffRebateRatePercent)
	require.InDelta(t, 42.5, *got.AffRebateRatePercent, 1e-9)

	// Clear exclusive rate
	require.NoError(t, repo.SetUserRebateRate(txCtx, u1.ID, nil))
	cleared, err := repo.EnsureUserAffiliate(txCtx, u1.ID)
	require.NoError(t, err)
	require.Nil(t, cleared.AffRebateRatePercent)

	// Batch set both users
	batchRate := 15.0
	require.NoError(t, repo.BatchSetUserRebateRate(txCtx, []int64{u1.ID, u2.ID}, &batchRate))

	for _, uid := range []int64{u1.ID, u2.ID} {
		v, err := repo.EnsureUserAffiliate(txCtx, uid)
		require.NoError(t, err)
		require.NotNil(t, v.AffRebateRatePercent)
		require.InDelta(t, 15.0, *v.AffRebateRatePercent, 1e-9)
	}

	// Batch clear
	require.NoError(t, repo.BatchSetUserRebateRate(txCtx, []int64{u1.ID, u2.ID}, nil))
	for _, uid := range []int64{u1.ID, u2.ID} {
		v, err := repo.EnsureUserAffiliate(txCtx, uid)
		require.NoError(t, err)
		require.Nil(t, v.AffRebateRatePercent)
	}
}

// TestAffiliateRepository_ListUsersWithCustomSettings verifies the admin list
// only includes users with at least one override applied.
func TestAffiliateRepository_ListUsersWithCustomSettings(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	// User without any custom config — should NOT appear in the list.
	plainEmail := fmt.Sprintf("affiliate-plain-%d@example.com", time.Now().UnixNano())
	uPlain := mustCreateUser(t, client, &service.User{
		Email: plainEmail, PasswordHash: "hash",
		Role: service.RoleUser, Status: service.StatusActive,
	})
	_, err := repo.EnsureUserAffiliate(txCtx, uPlain.ID)
	require.NoError(t, err)

	// User with a custom code — should appear.
	uCode := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-codeonly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	require.NoError(t, repo.UpdateUserAffCode(txCtx, uCode.ID, fmt.Sprintf("VIP%09d", time.Now().UnixNano()%1_000_000_000)))

	// User with only an exclusive rate — should appear.
	uRate := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rateonly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	r := 33.3
	require.NoError(t, repo.SetUserRebateRate(txCtx, uRate.ID, &r))

	entries, total, err := repo.ListUsersWithCustomSettings(txCtx, service.AffiliateAdminFilter{
		Page: 1, PageSize: 100,
	})
	require.NoError(t, err)

	// Build a quick lookup to assert per-user attributes (other tests may have
	// inserted custom rows in the same DB; we only care about our 3).
	byUserID := make(map[int64]service.AffiliateAdminEntry, len(entries))
	for _, e := range entries {
		byUserID[e.UserID] = e
	}

	require.NotContains(t, byUserID, uPlain.ID, "users without overrides must not appear")

	codeEntry, ok := byUserID[uCode.ID]
	require.True(t, ok, "custom-code user missing from list")
	require.True(t, codeEntry.AffCodeCustom)
	require.Nil(t, codeEntry.AffRebateRatePercent)

	rateEntry, ok := byUserID[uRate.ID]
	require.True(t, ok, "custom-rate user missing from list")
	require.False(t, rateEntry.AffCodeCustom)
	require.NotNil(t, rateEntry.AffRebateRatePercent)
	require.InDelta(t, 33.3, *rateEntry.AffRebateRatePercent, 1e-9)

	require.GreaterOrEqual(t, total, int64(2), "total must include at least our 2 custom rows")
}

// seedReversalFixture creates inviter+invitee bound pair and an accrued rebate
// for a COMPLETED balance order, returning the repo, ids and order id.
func seedReversalFixture(t *testing.T, ctx context.Context, txCtx context.Context, client *dbent.Client, freezeHours int, orderAmount, rebate float64) (service.AffiliateRepository, int64, int64, int64) {
	t.Helper()
	repo := NewAffiliateRepository(client, integrationDB)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("aff-rev-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("aff-rev-invitee-%d@example.com", time.Now().UnixNano()+1),
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

	order, err := client.PaymentOrder.Create().
		SetUserID(invitee.ID).
		SetUserEmail(invitee.Email).
		SetUserName(invitee.Username).
		SetAmount(orderAmount).
		SetPayAmount(orderAmount).
		SetFeeRate(0).
		SetRechargeCode(fmt.Sprintf("AFF-REV-%d", time.Now().UnixNano())).
		SetOutTradeNo(fmt.Sprintf("sub2_aff_rev_%d", time.Now().UnixNano())).
		SetPaymentType("alipay").
		SetPaymentTradeNo("").
		SetOrderType("balance").
		SetStatus("COMPLETED").
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetExpiresAt(time.Now().Add(24 * time.Hour)).
		Save(txCtx)
	require.NoError(t, err)

	applied, err := repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviter.ID,
		InviteeUserID: invitee.ID,
		Amount:        rebate,
		BaseAmount:    orderAmount,
		SourceOrderID: order.ID,
		FreezeHours:   freezeHours,
	})
	require.NoError(t, err)
	require.InDelta(t, rebate, applied, 1e-9)

	return repo, inviter.ID, invitee.ID, order.ID
}

func mustCreateAffiliateBalanceOrder(t *testing.T, txCtx context.Context, client *dbent.Client, userID int64, amount float64, tag string) int64 {
	t.Helper()

	now := time.Now().UnixNano()
	order, err := client.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail(fmt.Sprintf("affiliate-order-user-%d@example.com", userID)).
		SetUserName(fmt.Sprintf("affiliate-order-user-%d", userID)).
		SetAmount(amount).
		SetPayAmount(amount).
		SetFeeRate(0).
		SetRechargeCode(fmt.Sprintf("AFF-%s-%d", tag, now)).
		SetOutTradeNo(fmt.Sprintf("sub2_aff_%s_%d", tag, now)).
		SetPaymentType("alipay").
		SetPaymentTradeNo("").
		SetOrderType("balance").
		SetStatus("COMPLETED").
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetExpiresAt(time.Now().Add(24 * time.Hour)).
		Save(txCtx)
	require.NoError(t, err)
	return order.ID
}

// TestAffiliateRepository_ReverseQuotaForOrder_DeductsFrozenFirst verifies that a
// full refund of an order whose rebate is still frozen claws back from
// aff_frozen_quota (and aff_history_quota), leaving balance untouched.
func TestAffiliateRepository_ReverseQuotaForOrder_DeductsFrozenFirst(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo, inviterID, _, orderID := seedReversalFixture(t, ctx, txCtx, client, 24, 100, 20)

	frozen := querySingleFloat(t, txCtx, client,
		"SELECT aff_frozen_quota::double precision FROM user_affiliates WHERE user_id = $1", inviterID)
	require.InDelta(t, 20.0, frozen, 1e-9)

	reversed, gotInviter, err := repo.ReverseQuotaForOrder(txCtx, service.AffiliateReversalInput{
		SourceOrderID:  orderID,
		RefundedAmount: 100,
		OrderAmount:    100,
	})
	require.NoError(t, err)
	require.InDelta(t, 20.0, reversed, 1e-9)
	require.Equal(t, inviterID, gotInviter)

	frozen = querySingleFloat(t, txCtx, client,
		"SELECT aff_frozen_quota::double precision FROM user_affiliates WHERE user_id = $1", inviterID)
	require.InDelta(t, 0.0, frozen, 1e-9)
	history := querySingleFloat(t, txCtx, client,
		"SELECT aff_history_quota::double precision FROM user_affiliates WHERE user_id = $1", inviterID)
	require.InDelta(t, 0.0, history, 1e-9)
	balance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", inviterID)
	require.InDelta(t, 0.0, balance, 1e-9)
}

func TestAffiliateRepository_ReverseQuotaForOrder_FreesPerInviteeCap(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo, inviterID, inviteeID, orderID := seedReversalFixture(t, ctx, txCtx, client, 24, 100, 10)

	reversed, gotInviter, err := repo.ReverseQuotaForOrder(txCtx, service.AffiliateReversalInput{
		SourceOrderID:  orderID,
		RefundedAmount: 100,
		OrderAmount:    100,
	})
	require.NoError(t, err)
	require.InDelta(t, 10.0, reversed, 1e-9)
	require.Equal(t, inviterID, gotInviter)

	netAccrued, err := repo.GetAccruedRebateFromInvitee(txCtx, inviterID, inviteeID)
	require.NoError(t, err)
	require.InDelta(t, 0.0, netAccrued, 1e-9)

	secondOrderID := mustCreateAffiliateBalanceOrder(t, txCtx, client, inviteeID, 100, "CAP-FREE")
	applied, err := repo.AccrueQuota(txCtx, service.AffiliateAccrualInput{
		InviterID:     inviterID,
		InviteeUserID: inviteeID,
		Amount:        10,
		BaseAmount:    100,
		RebateCap:     100,
		PerInviteeCap: 10,
		SourceOrderID: secondOrderID,
		FreezeHours:   24,
	})
	require.NoError(t, err)
	require.InDelta(t, 10.0, applied, 1e-9)

	netAccrued, err = repo.GetAccruedRebateFromInvitee(txCtx, inviterID, inviteeID)
	require.NoError(t, err)
	require.InDelta(t, 10.0, netAccrued, 1e-9)
}

// TestAffiliateRepository_ReverseQuotaForOrder_ClawsBackTransferredBalance is the
// core arbitrage guard: when the rebate has already matured and been transferred
// to spendable balance, the reversal must reclaim it from balance (allowing it to
// go negative), not silently no-op.
func TestAffiliateRepository_ReverseQuotaForOrder_ClawsBackTransferredBalance(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	// No freeze: rebate lands directly in aff_quota.
	repo, inviterID, _, orderID := seedReversalFixture(t, ctx, txCtx, client, 0, 100, 20)

	available := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", inviterID)
	require.InDelta(t, 20.0, available, 1e-9)

	// Transfer the matured quota out to balance (the arbitrage cash-out step).
	transferred, _, err := repo.TransferQuotaToBalance(txCtx, inviterID)
	require.NoError(t, err)
	require.InDelta(t, 20.0, transferred, 1e-9)
	balance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", inviterID)
	require.InDelta(t, 20.0, balance, 1e-9)

	// Now refund the order: quota is gone, so reversal must hit balance.
	reversed, gotInviter, err := repo.ReverseQuotaForOrder(txCtx, service.AffiliateReversalInput{
		SourceOrderID:  orderID,
		RefundedAmount: 100,
		OrderAmount:    100,
	})
	require.NoError(t, err)
	require.InDelta(t, 20.0, reversed, 1e-9)
	require.Equal(t, inviterID, gotInviter)

	balance = querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", inviterID)
	require.InDelta(t, 0.0, balance, 1e-9, "transferred rebate must be reclaimed from balance")
}

// TestAffiliateRepository_ReverseQuotaForOrder_ProportionalAndIdempotent verifies
// cumulative proportional reversal across repeated partial refunds and idempotent
// retries with the same cumulative amount.
func TestAffiliateRepository_ReverseQuotaForOrder_ProportionalAndIdempotent(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo, inviterID, _, orderID := seedReversalFixture(t, ctx, txCtx, client, 24, 100, 20)

	// Refund 30% cumulative -> reverse 6.
	reversed, _, err := repo.ReverseQuotaForOrder(txCtx, service.AffiliateReversalInput{
		SourceOrderID: orderID, RefundedAmount: 30, OrderAmount: 100,
	})
	require.NoError(t, err)
	require.InDelta(t, 6.0, reversed, 1e-9)

	// Retry with same cumulative amount -> no-op.
	reversed, _, err = repo.ReverseQuotaForOrder(txCtx, service.AffiliateReversalInput{
		SourceOrderID: orderID, RefundedAmount: 30, OrderAmount: 100,
	})
	require.NoError(t, err)
	require.InDelta(t, 0.0, reversed, 1e-9)

	// Refund the rest (cumulative 100%) -> reverse remaining 14.
	reversed, _, err = repo.ReverseQuotaForOrder(txCtx, service.AffiliateReversalInput{
		SourceOrderID: orderID, RefundedAmount: 100, OrderAmount: 100,
	})
	require.NoError(t, err)
	require.InDelta(t, 14.0, reversed, 1e-9)

	frozen := querySingleFloat(t, txCtx, client,
		"SELECT aff_frozen_quota::double precision FROM user_affiliates WHERE user_id = $1", inviterID)
	require.InDelta(t, 0.0, frozen, 1e-9)

	totalReversed := querySingleFloat(t, txCtx, client, `
SELECT COALESCE(SUM(-amount), 0)::double precision
FROM user_affiliate_ledger
WHERE source_order_id = $1 AND action = 'reverse'`, orderID)
	require.InDelta(t, 20.0, totalReversed, 1e-9)
}

// TestAffiliateRepository_ReverseQuotaForOrder_NoAccrualNoOp ensures reversal is a
// safe no-op when the order never accrued any rebate.
func TestAffiliateRepository_ReverseQuotaForOrder_NoAccrualNoOp(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)
	reversed, gotInviter, err := repo.ReverseQuotaForOrder(txCtx, service.AffiliateReversalInput{
		SourceOrderID: 999999999, RefundedAmount: 50, OrderAmount: 100,
	})
	require.NoError(t, err)
	require.InDelta(t, 0.0, reversed, 1e-9)
	require.Zero(t, gotInviter)
}
