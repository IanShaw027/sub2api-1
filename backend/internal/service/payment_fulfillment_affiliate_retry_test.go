//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
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
	summary    *AffiliateSummary
	accrueErr  error
	accrueUsed AffiliateAccrualInput
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
	s.accrueUsed = input
	if s.accrueErr != nil {
		return 0, s.accrueErr
	}
	return input.Amount, nil
}

func (s *paymentFulfillmentAffiliateRepoStub) ApplySignupBonus(context.Context, int64, float64) (bool, float64, error) {
	return false, 0, nil
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
	redeemService := NewRedeemService(redeemRepo, nil, nil, nil, nil, client, nil)

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
		SetRechargeCode("AFFILIATE-COMMIT-FAIL-RECHARGE-CODE").
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
	redeemService := NewRedeemService(redeemRepo, nil, nil, nil, nil, client, nil)

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
