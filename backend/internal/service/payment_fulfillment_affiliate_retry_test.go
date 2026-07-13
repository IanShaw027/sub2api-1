//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type paymentFulfillmentAffiliateSettingRepoStub struct {
	values map[string]string
}

func (s *paymentFulfillmentAffiliateSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *paymentFulfillmentAffiliateSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if s.values == nil {
		return "", ErrSettingNotFound
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *paymentFulfillmentAffiliateSettingRepoStub) Set(context.Context, string, string) error {
	return nil
}

func (s *paymentFulfillmentAffiliateSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *paymentFulfillmentAffiliateSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (s *paymentFulfillmentAffiliateSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *paymentFulfillmentAffiliateSettingRepoStub) Delete(context.Context, string) error {
	return nil
}

type paymentFulfillmentAffiliateRepoStub struct {
	summary      *AffiliateSummary
	accrueErr    error
	accrueUsed   AffiliateAccrualInput
	accrueHits   int
	reverseUsed  AffiliateReversalInput
	reverseHits  int
	reverseRet   float64
	reverseInvtr int64
	reverseErr   error
}

func (s *paymentFulfillmentAffiliateRepoStub) EnsureUserAffiliate(context.Context, int64) (*AffiliateSummary, error) {
	if s.summary == nil {
		return &AffiliateSummary{}, nil
	}
	return s.summary, nil
}

func (s *paymentFulfillmentAffiliateRepoStub) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	return nil, ErrAffiliateProfileNotFound
}

func (s *paymentFulfillmentAffiliateRepoStub) BindInviter(context.Context, int64, int64) (bool, error) {
	return false, nil
}

func (s *paymentFulfillmentAffiliateRepoStub) AccrueQuota(_ context.Context, input AffiliateAccrualInput) (float64, error) {
	s.accrueHits++
	s.accrueUsed = input
	if s.accrueErr != nil {
		return 0, s.accrueErr
	}
	return input.Amount, nil
}

func (s *paymentFulfillmentAffiliateRepoStub) ReverseQuotaForOrder(_ context.Context, input AffiliateReversalInput) (float64, int64, error) {
	s.reverseHits++
	s.reverseUsed = input
	if s.reverseErr != nil {
		return 0, 0, s.reverseErr
	}
	return s.reverseRet, s.reverseInvtr, nil
}

func (s *paymentFulfillmentAffiliateRepoStub) ApplySignupBonus(context.Context, int64, float64) (bool, float64, error) {
	return false, 0, nil
}
func (s *paymentFulfillmentAffiliateRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	return 0, nil
}
func (s *paymentFulfillmentAffiliateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}

func (s *paymentFulfillmentAffiliateRepoStub) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	return 0, 0, nil
}

func (s *paymentFulfillmentAffiliateRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	return nil, nil
}

func (s *paymentFulfillmentAffiliateRepoStub) ListInviteeLedger(context.Context, int64, int64, int) ([]AffiliateLedgerEntry, error) {
	return nil, nil
}

func (s *paymentFulfillmentAffiliateRepoStub) CountRebatedInvitees(context.Context, int64) (int, error) {
	return 0, nil
}

func (s *paymentFulfillmentAffiliateRepoStub) ListAdminAffiliateStats(context.Context, AdminAffiliateListParams) ([]AdminAffiliateStatsRow, int64, error) {
	return nil, 0, nil
}
func (s *paymentFulfillmentAffiliateRepoStub) UpdateUserAffCode(context.Context, int64, string) error {
	return nil
}
func (s *paymentFulfillmentAffiliateRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	return "", nil
}
func (s *paymentFulfillmentAffiliateRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	return nil
}
func (s *paymentFulfillmentAffiliateRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	return nil
}
func (s *paymentFulfillmentAffiliateRepoStub) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	return nil, 0, nil
}
func (s *paymentFulfillmentAffiliateRepoStub) ListAffiliateInviteRecords(context.Context, AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	return nil, 0, nil
}
func (s *paymentFulfillmentAffiliateRepoStub) ListAffiliateRebateRecords(context.Context, AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	return nil, 0, nil
}
func (s *paymentFulfillmentAffiliateRepoStub) ListAffiliateTransferRecords(context.Context, AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	return nil, 0, nil
}
func (s *paymentFulfillmentAffiliateRepoStub) GetAffiliateUserOverview(context.Context, int64) (*AffiliateUserOverview, error) {
	return nil, nil
}

func TestExecuteBalanceFulfillment_AffiliateAccrualFailureCompletesOrder(t *testing.T) {
	ctx := context.Background()
	client := newOrderNotFoundTestClient(t)

	user, err := client.User.Create().
		SetEmail("affiliate-accrue-fail@example.com").
		SetPasswordHash("hash").
		SetUsername("affiliate-accrue-fail").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("AFFILIATE-FAIL-RECHARGE-CODE").
		SetOutTradeNo("sub2_affiliate_accrue_fail").
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("trade-affiliate-accrue-fail").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusPaid).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)

	redeemRepo := &paymentOrderLifecycleRedeemRepo{
		codesByCode: map[string]*RedeemCode{
			order.RechargeCode: {
				ID:     1,
				Code:   order.RechargeCode,
				Type:   RedeemTypeBalance,
				Value:  order.Amount,
				Status: StatusUsed,
			},
		},
	}
	redeemService := NewRedeemService(redeemRepo, nil, nil, nil, nil, client, nil, nil)

	inviterID := int64(12345)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		summary: &AffiliateSummary{
			UserID:    user.ID,
			InviterID: &inviterID,
		},
		accrueErr: errors.New("accrue failed"),
	}
	affiliateService := &AffiliateService{
		repo: affiliateRepo,
		settingRepo: &paymentFulfillmentAffiliateSettingRepoStub{
			values: map[string]string{
				SettingKeyAffiliateEnabled:    "true",
				SettingKeyAffiliateRebateRate: "20",
			},
		},
	}

	svc := &PaymentService{
		entClient:        client,
		redeemService:    redeemService,
		affiliateService: affiliateService,
	}

	err = svc.ExecuteBalanceFulfillment(ctx, order.ID)
	require.NoError(t, err)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.NotNil(t, reloaded.CompletedAt)
	require.Nil(t, reloaded.FailedAt)
	require.True(t, svc.hasAuditLog(ctx, order.ID, "AFFILIATE_REBATE_FAILED"))
}

func TestExecuteBalanceFulfillment_AffiliateCommitFailureCompletesOrder(t *testing.T) {
	ctx := context.Background()
	client := newOrderNotFoundTestClient(t)

	user, err := client.User.Create().
		SetEmail("affiliate-commit-fail@example.com").
		SetPasswordHash("hash").
		SetUsername("affiliate-commit-fail").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("AFFILIATE-COMMIT-FAIL").
		SetOutTradeNo("sub2_affiliate_commit_fail").
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("trade-affiliate-commit-fail").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusPaid).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)

	redeemRepo := &paymentOrderLifecycleRedeemRepo{
		codesByCode: map[string]*RedeemCode{
			order.RechargeCode: {
				ID:     1,
				Code:   order.RechargeCode,
				Type:   RedeemTypeBalance,
				Value:  order.Amount,
				Status: StatusUsed,
			},
		},
	}
	redeemService := NewRedeemService(redeemRepo, nil, nil, nil, nil, client, nil, nil)

	inviterID := int64(67890)
	affiliateService := &AffiliateService{
		repo: &paymentFulfillmentAffiliateRepoStub{
			summary: &AffiliateSummary{
				UserID:    user.ID,
				InviterID: &inviterID,
			},
		},
		settingRepo: &paymentFulfillmentAffiliateSettingRepoStub{
			values: map[string]string{
				SettingKeyAffiliateEnabled:    "true",
				SettingKeyAffiliateRebateRate: "20",
			},
		},
	}

	svc := &PaymentService{
		entClient:        client,
		redeemService:    redeemService,
		affiliateService: affiliateService,
		commitPaymentTx: func(*dbent.Tx) error {
			return errors.New("commit failed")
		},
	}

	err = svc.ExecuteBalanceFulfillment(ctx, order.ID)
	require.NoError(t, err)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.NotNil(t, reloaded.CompletedAt)
	require.Nil(t, reloaded.FailedAt)
	require.True(t, svc.hasAuditLog(ctx, order.ID, "AFFILIATE_REBATE_FAILED"))
}

func TestExecuteBalanceFulfillment_DoesNotApplyAffiliateRebateBeforeCompletionSucceeds(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeBalance)

	trigger := fmt.Sprintf(`
		CREATE TRIGGER fail_balance_completion_with_affiliate
		BEFORE UPDATE OF status ON payment_orders
		WHEN NEW.id = %d AND NEW.status = '%s'
		BEGIN
			SELECT RAISE(FAIL, 'forced completion failure');
		END;
	`, order.ID, OrderStatusCompleted)
	_, err := client.ExecContext(ctx, trigger)
	require.NoError(t, err)

	inviterID := int64(9988)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		summary: &AffiliateSummary{
			UserID:    order.UserID,
			InviterID: &inviterID,
		},
	}
	affiliateService := &AffiliateService{
		repo: affiliateRepo,
		settingRepo: &paymentFulfillmentAffiliateSettingRepoStub{
			values: map[string]string{
				SettingKeyAffiliateEnabled:    "true",
				SettingKeyAffiliateRebateRate: "20",
			},
		},
	}

	svc := &PaymentService{
		entClient:        client,
		affiliateService: affiliateService,
	}

	err = svc.ExecuteBalanceFulfillment(ctx, order.ID)
	require.Error(t, err)
	require.Zero(t, affiliateRepo.accrueHits)
	require.False(t, svc.hasAuditLog(ctx, order.ID, "AFFILIATE_REBATE_APPLIED"))
	require.False(t, svc.hasAuditLog(ctx, order.ID, "AFFILIATE_REBATE_SKIPPED"))
	require.False(t, svc.hasAuditLog(ctx, order.ID, "AFFILIATE_REBATE_FAILED"))
}

func TestExecuteSubscriptionFulfillmentAppliesAffiliateRebate(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeSubscription)

	inviterID := int64(6677)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		summary: &AffiliateSummary{UserID: order.UserID, InviterID: &inviterID},
	}
	affiliateService := &AffiliateService{
		repo: affiliateRepo,
		settingRepo: &paymentFulfillmentAffiliateSettingRepoStub{values: map[string]string{
			SettingKeyAffiliateEnabled:    "true",
			SettingKeyAffiliateRebateRate: "20",
		}},
	}
	groupRepo := paymentFulfillmentGroupRepoStub{group: &Group{
		ID:               *order.SubscriptionGroupID,
		Status:           payment.EntityStatusActive,
		SubscriptionType: SubscriptionTypeSubscription,
	}}
	subRepo := &paymentFulfillmentUserSubRepoStub{existing: &UserSubscription{
		ID:        77,
		UserID:    order.UserID,
		GroupID:   *order.SubscriptionGroupID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    SubscriptionStatusActive,
	}}
	svc := &PaymentService{
		entClient:        client,
		groupRepo:        groupRepo,
		subscriptionSvc:  NewSubscriptionService(groupRepo, subRepo, nil, client, nil),
		affiliateService: affiliateService,
	}

	require.NoError(t, svc.ExecuteSubscriptionFulfillment(ctx, order.ID))
	require.Equal(t, 1, affiliateRepo.accrueHits)
	require.Equal(t, order.ID, affiliateRepo.accrueUsed.SourceOrderID)
	require.True(t, svc.hasAuditLog(ctx, order.ID, "AFFILIATE_REBATE_APPLIED"))
}

func TestRetryFulfillmentCompletedSubscriptionDoesNotDuplicateAffiliateRebate(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeSubscription)
	now := time.Now()
	_, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(OrderStatusCompleted).
		SetPaidAt(now).
		SetCompletedAt(now).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentAuditLog.Create().
		SetOrderID(fmt.Sprintf("%d", order.ID)).
		SetAction("AFFILIATE_REBATE_APPLIED").
		SetDetail(`{"baseAmount":100,"rebateAmount":20}`).
		SetOperator("system").
		Save(ctx)
	require.NoError(t, err)

	inviterID := int64(7789)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		summary: &AffiliateSummary{UserID: order.UserID, InviterID: &inviterID},
	}
	svc := &PaymentService{
		entClient: client,
		affiliateService: &AffiliateService{
			repo: affiliateRepo,
			settingRepo: &paymentFulfillmentAffiliateSettingRepoStub{values: map[string]string{
				SettingKeyAffiliateEnabled:    "true",
				SettingKeyAffiliateRebateRate: "20",
			}},
		},
	}

	require.NoError(t, svc.RetryFulfillment(ctx, order.ID))
	require.Zero(t, affiliateRepo.accrueHits)
}

func TestRetryFulfillment_CompletedOrderBackfillsMissingAffiliateRebate(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeBalance)
	now := time.Now()

	_, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(OrderStatusCompleted).
		SetPaidAt(now).
		SetCompletedAt(now).
		Save(ctx)
	require.NoError(t, err)

	inviterID := int64(7788)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		summary: &AffiliateSummary{
			UserID:    order.UserID,
			InviterID: &inviterID,
		},
	}
	affiliateService := &AffiliateService{
		repo: affiliateRepo,
		settingRepo: &paymentFulfillmentAffiliateSettingRepoStub{
			values: map[string]string{
				SettingKeyAffiliateEnabled:    "true",
				SettingKeyAffiliateRebateRate: "20",
			},
		},
	}

	svc := &PaymentService{
		entClient:        client,
		affiliateService: affiliateService,
	}

	err = svc.RetryFulfillment(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, 1, affiliateRepo.accrueHits)
	require.True(t, svc.hasAuditLog(ctx, order.ID, "AFFILIATE_REBATE_APPLIED"))
}

func TestApplyAffiliateRebateForSubscriptionUsesPayAmount(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusCompleted, payment.OrderTypeSubscription)
	order, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetAmount(9.99).
		SetPayAmount(71.43).
		Save(ctx)
	require.NoError(t, err)

	inviterID := int64(12345)
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		summary: &AffiliateSummary{
			UserID:    order.UserID,
			InviterID: &inviterID,
		},
	}
	affiliateService := &AffiliateService{
		repo: affiliateRepo,
		settingRepo: &paymentFulfillmentAffiliateSettingRepoStub{
			values: map[string]string{
				SettingKeyAffiliateEnabled:    "true",
				SettingKeyAffiliateRebateRate: "20",
			},
		},
	}
	svc := &PaymentService{entClient: client, affiliateService: affiliateService}

	require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, order))
	require.Equal(t, 1, affiliateRepo.accrueHits)
	require.InDelta(t, 71.43, affiliateRepo.accrueUsed.BaseAmount, 0.000001)
	require.InDelta(t, 14.286, affiliateRepo.accrueUsed.Amount, 0.000001)

	entry, err := client.PaymentAuditLog.Query().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
			paymentauditlog.ActionEQ("AFFILIATE_REBATE_APPLIED"),
		).
		Only(ctx)
	require.NoError(t, err)
	require.Contains(t, entry.Detail, `"baseAmount":71.43`)
}
