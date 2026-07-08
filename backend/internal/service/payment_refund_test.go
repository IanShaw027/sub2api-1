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
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type refundTestUserRepo struct {
	users          map[int64]*User
	deductedAmount float64
}

func (r *refundTestUserRepo) Create(context.Context, *User) error { panic("unexpected") }
func (r *refundTestUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	if user := r.users[id]; user != nil {
		clone := *user
		return &clone, nil
	}
	return nil, ErrUserNotFound
}
func (r *refundTestUserRepo) GetByIDIncludeDeleted(context.Context, int64) (*User, error) {
	panic("unexpected GetByIDIncludeDeleted call")
}
func (r *refundTestUserRepo) GetByEmail(context.Context, string) (*User, error) { panic("unexpected") }
func (r *refundTestUserRepo) GetFirstAdmin(context.Context) (*User, error)      { panic("unexpected") }
func (r *refundTestUserRepo) Update(context.Context, *User) error               { panic("unexpected") }
func (r *refundTestUserRepo) Delete(context.Context, int64) error               { panic("unexpected") }
func (r *refundTestUserRepo) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) DeleteUserAvatar(context.Context, int64) error { panic("unexpected") }
func (r *refundTestUserRepo) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	panic("unexpected")
}
func (r *refundTestUserRepo) UpdateBalance(_ context.Context, id int64, amount float64) error {
	if user := r.users[id]; user != nil {
		user.Balance += amount
	}
	return nil
}
func (r *refundTestUserRepo) DeductBalance(_ context.Context, id int64, amount float64) error {
	r.deductedAmount += amount
	if user := r.users[id]; user != nil {
		user.Balance -= amount
	}
	return nil
}
func (r *refundTestUserRepo) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected")
}
func (r *refundTestUserRepo) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) ExistsByEmail(context.Context, string) (bool, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected")
}
func (r *refundTestUserRepo) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected")
}
func (r *refundTestUserRepo) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	panic("unexpected")
}
func (r *refundTestUserRepo) UnbindUserAuthProvider(context.Context, int64, string) error {
	panic("unexpected")
}
func (r *refundTestUserRepo) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected")
}
func (r *refundTestUserRepo) EnableTotp(context.Context, int64) error  { panic("unexpected") }
func (r *refundTestUserRepo) DisableTotp(context.Context, int64) error { panic("unexpected") }

type refundTestProvider struct {
	status      string
	refundCalls int
}

func (p *refundTestProvider) Name() string        { return "refund-test" }
func (p *refundTestProvider) ProviderKey() string { return payment.TypeAlipay }
func (p *refundTestProvider) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay}
}
func (p *refundTestProvider) CreatePayment(context.Context, payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	panic("unexpected")
}
func (p *refundTestProvider) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	panic("unexpected")
}
func (p *refundTestProvider) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	panic("unexpected")
}
func (p *refundTestProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	p.refundCalls++
	return &payment.RefundResponse{RefundID: "refund-test-id", Status: p.status}, nil
}

type refundTestLoadBalancer struct{}

func (l refundTestLoadBalancer) GetInstanceConfig(context.Context, int64) (map[string]string, error) {
	return map[string]string{}, nil
}
func (l refundTestLoadBalancer) SelectInstance(context.Context, string, payment.PaymentType, payment.Strategy, float64) (*payment.InstanceSelection, error) {
	panic("unexpected")
}

func TestValidateRefundRequestAllowsUniquelyResolvedLegacyProviderInstance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-legacy@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-legacy-user").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("REFUND-LEGACY-ORDER").
		SetOutTradeNo("sub2_refund_legacy_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-legacy-refund").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	validatedOrder, _, _, err := svc.validateRefundRequest(ctx, order.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, order.ID, validatedOrder.ID)
}

func TestPrepareRefundAllowsUniquelyResolvedLegacyProviderInstance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-legacy-admin@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-legacy-admin-user").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-admin-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(188).
		SetPayAmount(188).
		SetFeeRate(0).
		SetRechargeCode("REFUND-LEGACY-ADMIN-ORDER").
		SetOutTradeNo("sub2_refund_legacy_admin_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-legacy-admin-refund").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, false)
	require.NotNil(t, plan)
	require.Nil(t, result)
	require.NoError(t, err)
	require.Equal(t, order.ID, plan.OrderID)
	require.Equal(t, 188.0, plan.RefundAmount)
}

func TestValidateRefundRequestRejectsAmbiguousLegacyProviderInstance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-legacy-ambiguous@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-legacy-ambiguous-user").
		Save(ctx)
	require.NoError(t, err)

	for _, name := range []string{"alipay-refund-instance-a", "alipay-refund-instance-b"} {
		_, err = client.PaymentProviderInstance.Create().
			SetProviderKey(payment.TypeAlipay).
			SetName(name).
			SetConfig("{}").
			SetSupportedTypes("alipay").
			SetEnabled(true).
			SetAllowUserRefund(true).
			SetRefundEnabled(true).
			Save(ctx)
		require.NoError(t, err)
	}

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("REFUND-LEGACY-AMBIGUOUS-ORDER").
		SetOutTradeNo("sub2_refund_legacy_ambiguous_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-legacy-ambiguous-refund").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	_, _, _, err = svc.validateRefundRequest(ctx, order.ID, user.ID)
	require.Error(t, err)
	require.Equal(t, "REFUND_DISABLED", infraerrors.Reason(err))
}

func TestPrepareRefundRejectsMismatchedLegacyProviderBinding(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-legacy-mismatch@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-legacy-mismatch-user").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-mismatch-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetAllowUserRefund(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(188).
		SetPayAmount(188).
		SetFeeRate(0).
		SetRechargeCode("REFUND-LEGACY-MISMATCH-ORDER").
		SetOutTradeNo("sub2_refund_legacy_mismatch_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-legacy-mismatch-refund").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderKey(payment.TypeStripe).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, false)
	require.Nil(t, plan)
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "REFUND_DISABLED", infraerrors.Reason(err))
}

func TestRequestRefundDoesNotConsumeRefundableAmount(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-request@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-request-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-request-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-REQUEST-ORDER").
		SetOutTradeNo("sub2_refund_request_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-request").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
		userRepo:  &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
	}

	_, err = svc.RequestRefund(ctx, order.ID, user.ID, 100, "please refund")
	require.NoError(t, err)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundRequested, reloaded.Status)
	require.Zero(t, reloaded.RefundAmount)

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 100.0, plan.RefundAmount)
}

func TestRequestRefundRejectsActiveInvoiceApplication(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "userinvoice")

	voider := &stubInvoiceVoider{links: map[int64]OrderInvoiceLink{
		order.ID: {InvoiceID: 42, InvoiceStatus: InvoiceStatusApplied, HasFile: false},
	}}
	svc := &PaymentService{
		entClient: client,
		userRepo:  &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
	}
	svc.SetInvoiceVoider(voider)

	_, err := svc.RequestRefund(ctx, order.ID, user.ID, 100, "please refund")
	require.Error(t, err)
	require.Equal(t, "INVOICE_ACTIVE", infraerrors.Reason(err))

	reloaded, getErr := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, getErr)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
}

func TestGetRefundPreviewBalanceCapsByCurrentBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-preview-balance@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-preview-balance-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-preview-balance-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-PREVIEW-BALANCE-ORDER").
		SetOutTradeNo("sub2_refund_preview_balance_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-preview-balance").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
		userRepo:  &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 40}}},
	}

	preview, err := svc.GetRefundPreview(ctx, order.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, payment.OrderTypeBalance, preview.OrderType)
	require.Equal(t, 100.0, preview.OrderAmount)
	require.Equal(t, 40.0, preview.BalanceAvailable)
	require.Equal(t, 40.0, preview.MaxRefundAmount)
	require.True(t, preview.RefundEnabled)
	require.False(t, preview.AutoRefund)
}

func TestGetRefundPreviewSubscriptionUsesUsageAndRefundMultiplier(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	now := time.Now()

	user, err := client.User.Create().
		SetEmail("refund-preview-sub@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-preview-sub-user").
		Save(ctx)
	require.NoError(t, err)

	group, err := client.Group.Create().
		SetName("refund-preview-sub-group").
		SetDescription("subscription refund preview").
		SetPlatform(PlatformAnthropic).
		SetRateMultiplier(2).
		SetRefundRateMultiplier(1.5).
		SetSubscriptionType(SubscriptionTypeSubscription).
		SetStatus(StatusActive).
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-preview-sub-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-PREVIEW-SUB-ORDER").
		SetOutTradeNo("sub2_refund_preview_sub_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-preview-sub").
		SetOrderType(payment.OrderTypeSubscription).
		SetPlanID(7).
		SetSubscriptionGroupID(group.ID).
		SetSubscriptionDays(30).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(now.Add(time.Hour)).
		SetPaidAt(now.Add(-time.Hour)).
		SetCompletedAt(now.Add(-30 * time.Minute)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	sub, err := client.UserSubscription.Create().
		SetUserID(user.ID).
		SetGroupID(group.ID).
		SetStartsAt(now.Add(-time.Hour)).
		SetExpiresAt(now.Add(30 * 24 * time.Hour)).
		SetStatus(SubscriptionStatusActive).
		Save(ctx)
	require.NoError(t, err)

	account, err := client.Account.Create().
		SetName("refund-preview-sub-account").
		SetPlatform(PlatformAnthropic).
		SetType("api_key").
		Save(ctx)
	require.NoError(t, err)
	apiKey, err := client.APIKey.Create().
		SetUserID(user.ID).
		SetKey("refund-preview-sub-key").
		SetName("refund preview sub key").
		SetGroupID(group.ID).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetUserID(user.ID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetRequestID("refund-preview-sub-usage").
		SetModel("claude-test").
		SetGroupID(group.ID).
		SetSubscriptionID(sub.ID).
		SetTotalCost(20).
		SetActualCost(40).
		SetRateMultiplier(2).
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
		userRepo:  &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 0}}},
	}

	preview, err := svc.GetRefundPreview(ctx, order.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, payment.OrderTypeSubscription, preview.OrderType)
	require.Equal(t, 40.0, preview.UsageAmount)
	require.Equal(t, 2.0, preview.SubscriptionRateMultiplier)
	require.Equal(t, 1.5, preview.RefundRateMultiplier)
	require.Equal(t, 30.0, preview.UsedRefundValue)
	require.Equal(t, 70.0, preview.MaxRefundAmount)
}

func TestRequestRefundAutoRefundWhenProviderAllowsUserRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-auto@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-auto-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-auto-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		SetAllowUserRefund(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-AUTO-ORDER").
		SetOutTradeNo("sub2_refund_auto_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
		userRepo:  &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
	}

	result, err := svc.RequestRefund(ctx, order.ID, user.ID, 40, "auto refund")
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, 40.0, result.BalanceDeducted)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.Equal(t, 40.0, reloaded.RefundAmount)
}

func TestExecuteRefundPendingProviderResponseDoesNotMarkSuccess(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-pending@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-pending-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-pending-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-PENDING-ORDER").
		SetOutTradeNo("sub2_refund_pending_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-pending").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	userRepo := &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}}
	svc := &PaymentService{entClient: client, userRepo: userRepo}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "pending refund", false, true)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 100.0, plan.BalanceToDeduct)

	result, err = svc.handleRefundProviderResponse(ctx, plan, &payment.RefundResponse{RefundID: "refund-test-id", Status: payment.ProviderStatusPending})
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Contains(t, result.Warning, "pending")
	require.Zero(t, userRepo.deductedAmount)
	require.Equal(t, 100.0, userRepo.users[user.ID].Balance)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunding, reloaded.Status)
	require.Zero(t, reloaded.RefundAmount)
}

func TestExecuteRefundPendingProviderResponsePreservesRefundingAndDeductedBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-pending-keep-refunding@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-pending-keep-refunding-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-pending-keep-refunding-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-PENDING-KEEP-REFUNDING-ORDER").
		SetOutTradeNo("sub2_refund_pending_keep_refunding_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-pending-keep-refunding").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	userRepo := &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}}
	svc := &PaymentService{entClient: client, userRepo: userRepo}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "pending refund", false, true)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 100.0, plan.BalanceToDeduct)

	require.NoError(t, userRepo.DeductBalance(ctx, user.ID, plan.BalanceToDeduct))
	plan.DeductionApplied = true
	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetStatus(OrderStatusRefunding).Save(ctx)
	require.NoError(t, err)
	plan.Order.Status = OrderStatusRefunding

	result, err = svc.handleRefundProviderResponse(ctx, plan, &payment.RefundResponse{RefundID: "refund-test-id", Status: payment.ProviderStatusPending})
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Equal(t, 0.0, userRepo.users[user.ID].Balance)
	require.Equal(t, 100.0, userRepo.deductedAmount)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunding, reloaded.Status)
	require.Zero(t, reloaded.RefundAmount)
}

func TestPendingProviderResponseKeepsRefundingAndAppliedBalanceDeduction(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "pendingrollback")
	userRepo := &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}}
	svc := &PaymentService{entClient: client, userRepo: userRepo}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "pending refund", false, true)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 100.0, plan.BalanceToDeduct)

	require.NoError(t, userRepo.DeductBalance(ctx, user.ID, plan.BalanceToDeduct))
	plan.DeductionApplied = true
	require.Equal(t, 0.0, userRepo.users[user.ID].Balance)
	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetStatus(OrderStatusRefunding).Save(ctx)
	require.NoError(t, err)
	plan.Order.Status = OrderStatusRefunding

	result, err = svc.handleRefundProviderResponse(ctx, plan, &payment.RefundResponse{RefundID: "refund-test-id", Status: payment.ProviderStatusPending})
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Contains(t, result.Warning, "pending")
	require.Zero(t, userRepo.users[user.ID].Balance)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunding, reloaded.Status)
	require.Zero(t, reloaded.RefundAmount)
}

func TestGatewayRefundFailureRestoresRefundRequestedStatus(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "failrequested")
	_, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(OrderStatusRefundRequested).
		SetRefundRequestedAmount(40).
		SetRefundRequestReason("partial requested").
		Save(ctx)
	require.NoError(t, err)
	userRepo := &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}}
	svc := &PaymentService{entClient: client, userRepo: userRepo}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, true)
	require.NoError(t, err)
	require.Nil(t, result)
	require.NoError(t, userRepo.DeductBalance(ctx, user.ID, plan.BalanceToDeduct))
	plan.DeductionApplied = true
	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetStatus(OrderStatusRefunding).Save(ctx)
	require.NoError(t, err)
	plan.Order.Status = OrderStatusRefunding

	result, err = svc.handleGwFail(ctx, plan, errors.New("provider unavailable"))
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Contains(t, result.Warning, "rolled back")
	require.Equal(t, 100.0, userRepo.users[user.ID].Balance)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundRequested, reloaded.Status)
	require.Zero(t, reloaded.RefundAmount)
}

func TestGatewayRefundFailureRestoresPartiallyRefundedStatus(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "failpartial")
	_, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(OrderStatusPartiallyRefunded).
		SetRefundAmount(30).
		Save(ctx)
	require.NoError(t, err)
	userRepo := &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}}
	svc := &PaymentService{entClient: client, userRepo: userRepo}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 20, "second partial", false, true)
	require.NoError(t, err)
	require.Nil(t, result)
	require.NoError(t, userRepo.DeductBalance(ctx, user.ID, plan.BalanceToDeduct))
	plan.DeductionApplied = true
	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetStatus(OrderStatusRefunding).Save(ctx)
	require.NoError(t, err)
	plan.Order.Status = OrderStatusRefunding

	result, err = svc.handleGwFail(ctx, plan, errors.New("provider unavailable"))
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Contains(t, result.Warning, "rolled back")
	require.Equal(t, 100.0, userRepo.users[user.ID].Balance)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.Equal(t, 30.0, reloaded.RefundAmount)
}

func TestPrepareRefundUsesRequestedAmountWhenAdminApprovesRefundRequestWithoutAmount(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-requested-default@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-requested-default-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-requested-default-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-REQUESTED-DEFAULT").
		SetOutTradeNo("sub2_refund_requested_default").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-requested-default").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusRefundRequested).
		SetRefundRequestedAmount(40).
		SetRefundRequestReason("partial requested").
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
		userRepo:  &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
	}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, true)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 40.0, plan.RefundAmount)
	require.Equal(t, "partial requested", plan.Reason)
	require.Equal(t, 40.0, plan.BalanceToDeduct)
}

func TestGwRefundRejectsAlipayMerchantIdentitySnapshotMismatch(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-snapshot-mismatch@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-snapshot-mismatch-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-mismatch-instance").
		SetConfig(encryptWebhookProviderConfig(t, map[string]string{
			"appId":      "runtime-alipay-app",
			"privateKey": "runtime-private-key",
		})).
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("REFUND-SNAPSHOT-MISMATCH-ORDER").
		SetOutTradeNo("sub2_refund_snapshot_mismatch_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-refund-snapshot-mismatch").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		SetProviderSnapshot(map[string]any{
			"schema_version":       2,
			"provider_instance_id": instID,
			"provider_key":         payment.TypeAlipay,
			"merchant_app_id":      "expected-alipay-app",
		}).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient:    client,
		loadBalancer: newWebhookProviderTestLoadBalancer(client),
	}

	_, err = svc.gwRefund(ctx, &RefundPlan{
		OrderID:       order.ID,
		Order:         order,
		RefundAmount:  order.Amount,
		GatewayAmount: order.Amount,
		Reason:        "snapshot mismatch",
	})
	require.ErrorContains(t, err, "alipay app_id mismatch")
}

func TestRefundProviderRequestIDIsStableForSameRefundAttempt(t *testing.T) {
	order := &dbent.PaymentOrder{
		ID:           42,
		OutTradeNo:   "sub2_refund_request_id",
		RefundAmount: 0,
	}
	plan := &RefundPlan{
		OrderID:       order.ID,
		Order:         order,
		GatewayAmount: 4,
	}

	first := refundProviderRequestID(plan)
	second := refundProviderRequestID(plan)
	require.Equal(t, "sub2api-refund-42-0-400", first)
	require.Equal(t, first, second)

	order.RefundAmount = 3
	require.Equal(t, "sub2api-refund-42-300-400", refundProviderRequestID(plan))
}

func TestPartialRefundAllowsRemainingRefundAndMarksFinalRefunded(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-partial@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-partial-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-partial-refund-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-PARTIAL-ORDER").
		SetOutTradeNo("sub2_refund_partial_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-partial-refund").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 30, "first partial", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 30.0, plan.RefundAmount)
	require.Equal(t, 30.0, plan.GatewayAmount)
	plan.Order.PaymentTradeNo = ""
	result, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.Equal(t, 30.0, reloaded.RefundAmount)

	plan, result, err = svc.PrepareRefund(ctx, order.ID, 70.01, "too much", false, false)
	require.Nil(t, plan)
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "REFUND_AMOUNT_EXCEEDED", infraerrors.Reason(err))

	plan, result, err = svc.PrepareRefund(ctx, order.ID, 70, "final partial", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 70.0, plan.RefundAmount)
	require.Equal(t, 70.0, plan.GatewayAmount)
	plan.Order.PaymentTradeNo = ""
	result, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)

	reloaded, err = client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, reloaded.Status)
	require.Equal(t, 100.0, reloaded.RefundAmount)
}

func TestProviderRefundedStatusMarksRefundSuccess(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "providerrefunded")
	svc := &PaymentService{
		entClient: client,
		userRepo:  &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
	}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "provider refunded", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	plan.BalanceToDeduct = 0
	plan.DeductionType = ""

	result, err = svc.handleRefundProviderResponse(ctx, plan, &payment.RefundResponse{RefundID: "refund-test-id", Status: payment.ProviderStatusRefunded})
	require.NoError(t, err)
	require.True(t, result.Success)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, reloaded.Status)
	require.Equal(t, 100.0, reloaded.RefundAmount)
}

func TestGatewayRefundDoesNotTakeLocalShortcutAfterFailedOrderPaidMetadataRecovery(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-recovered@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-recovered-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeStripe).
		SetName("stripe-refund-recovered-instance").
		SetConfig("{}").
		SetSupportedTypes("stripe").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(inst.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-RECOVERED-FAILED-ORDER").
		SetOutTradeNo("sub2_refund_recovered_failed_order").
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusFailed).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetFailedAt(time.Now().Add(-10 * time.Minute)).
		SetFailedReason("create payment returned error after upstream charge").
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeStripe).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient:    client,
		loadBalancer: refundTestLoadBalancer{},
	}

	recoveredOrder, err := svc.markFailedOrderPaidAndReload(ctx, order.ID, "pi_recovered_paid", 100)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPaid, recoveredOrder.Status)
	require.Equal(t, "pi_recovered_paid", recoveredOrder.PaymentTradeNo)
	require.NotNil(t, recoveredOrder.PaidAt)
	require.Nil(t, recoveredOrder.FailedAt)
	require.Nil(t, recoveredOrder.FailedReason)

	recoveredOrder, err = client.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(OrderStatusCompleted).
		SetCompletedAt(time.Now()).
		Save(ctx)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, recoveredOrder.Status)

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "refund recovered payment", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, "pi_recovered_paid", plan.Order.PaymentTradeNo)

	_, err = svc.gwRefund(ctx, plan)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create provider from instance")

	noTradeNoAuditCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_NO_TRADE_NO")).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, noTradeNoAuditCount)
}

func TestCalculateGatewayRefundAmountUsesCurrencyPrecision(t *testing.T) {
	require.InDelta(t, 6.173, calculateGatewayRefundAmount(100, 12.345, 50, "KWD"), 1e-12)
	require.InDelta(t, 12.345, calculateGatewayRefundAmount(100, 12.345, 100, "KWD"), 1e-12)
	require.InDelta(t, 52, calculateGatewayRefundAmount(100, 103, 50, "JPY"), 1e-12)
}

func TestFormatGatewayRefundAmountUsesOrderCurrency(t *testing.T) {
	order := &dbent.PaymentOrder{
		ProviderSnapshot: map[string]any{
			"currency": "KWD",
		},
	}

	require.Equal(t, "12.345", formatGatewayRefundAmount(12.345, order))
}

func TestValidateRefundProviderResponseAcceptsPending(t *testing.T) {
	require.NoError(t, validateRefundProviderResponse(&payment.RefundResponse{Status: payment.ProviderStatusPending}))
	require.NoError(t, validateRefundProviderResponse(&payment.RefundResponse{Status: payment.ProviderStatusSuccess}))
	require.Error(t, validateRefundProviderResponse(&payment.RefundResponse{Status: payment.ProviderStatusFailed}))
	require.Error(t, validateRefundProviderResponse(nil))
}

// affiliateServiceWithReverseStub builds an AffiliateService whose repo records
// reversal calls, for asserting that the refund flow claws back rebate.
func affiliateServiceWithReverseStub(stub *paymentFulfillmentAffiliateRepoStub) *AffiliateService {
	return &AffiliateService{
		repo: stub,
		settingRepo: &paymentFulfillmentAffiliateSettingRepoStub{
			values: map[string]string{
				SettingKeyAffiliateEnabled:    "true",
				SettingKeyAffiliateRebateRate: "20",
			},
		},
	}
}

func seedRefundBalanceOrder(t *testing.T, ctx context.Context, client *dbent.Client, amount float64, suffix string) (*User, *dbent.PaymentOrder) {
	t.Helper()
	user, err := client.User.Create().
		SetEmail("refund-reverse-" + suffix + "@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-reverse-" + suffix).
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-reverse-" + suffix).
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(amount).
		SetPayAmount(amount).
		SetFeeRate(0).
		SetRechargeCode("REFUND-REVERSE-" + suffix).
		SetOutTradeNo("sub2_refund_reverse_" + suffix).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)
	return &User{ID: user.ID, Email: user.Email, Username: user.Username}, order
}

// TestRefundReversesAffiliateRebate is the regression guard for the arbitrage
// hole: a successful refund must claw back the rebate accrued to the inviter.
// Before the fix, markRefundOk never called the affiliate reversal path.
func TestRefundReversesAffiliateRebate(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "full")

	stub := &paymentFulfillmentAffiliateRepoStub{reverseRet: 20}
	svc := &PaymentService{
		entClient:        client,
		userRepo:         &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
		affiliateService: affiliateServiceWithReverseStub(stub),
	}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "full refund", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	plan.Order.PaymentTradeNo = ""
	result, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)

	require.Equal(t, 1, stub.reverseHits, "refund must trigger affiliate rebate reversal")
	require.Equal(t, order.ID, stub.reverseUsed.SourceOrderID)
	require.Equal(t, 100.0, stub.reverseUsed.RefundedAmount)
	require.Equal(t, 100.0, stub.reverseUsed.OrderAmount)
	require.True(t, svc.hasAuditLog(ctx, order.ID, "AFFILIATE_REBATE_REVERSED"))
}

// TestPartialRefundReversesProportionalRebateCumulatively verifies the reversal
// receives the *cumulative* refunded amount on each partial refund, which is what
// makes the repository-side proportional clawback idempotent and correct.
func TestPartialRefundReversesProportionalRebateCumulatively(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "partial")

	stub := &paymentFulfillmentAffiliateRepoStub{reverseRet: 6}
	svc := &PaymentService{
		entClient:        client,
		userRepo:         &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
		affiliateService: affiliateServiceWithReverseStub(stub),
	}

	plan, _, err := svc.PrepareRefund(ctx, order.ID, 30, "first partial", false, false)
	require.NoError(t, err)
	plan.Order.PaymentTradeNo = ""
	_, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, 30.0, stub.reverseUsed.RefundedAmount)

	plan, _, err = svc.PrepareRefund(ctx, order.ID, 70, "final partial", false, false)
	require.NoError(t, err)
	plan.Order.PaymentTradeNo = ""
	_, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, 2, stub.reverseHits)
	require.Equal(t, 100.0, stub.reverseUsed.RefundedAmount, "second partial passes cumulative total")
}

func TestExecuteRefundRejectsStalePreparedPlanAfterConcurrentPartialRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "staleplan")

	stub := &paymentFulfillmentAffiliateRepoStub{reverseRet: 6}
	svc := &PaymentService{
		entClient:        client,
		userRepo:         &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
		affiliateService: affiliateServiceWithReverseStub(stub),
	}

	firstPlan, result, err := svc.PrepareRefund(ctx, order.ID, 60, "first partial", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	stalePlan, result, err := svc.PrepareRefund(ctx, order.ID, 50, "stale partial", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	firstPlan.Order.PaymentTradeNo = ""
	stalePlan.Order.PaymentTradeNo = ""

	result, err = svc.ExecuteRefund(ctx, firstPlan)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, 1, stub.reverseHits)
	require.Equal(t, 60.0, stub.reverseUsed.RefundedAmount)

	result, err = svc.ExecuteRefund(ctx, stalePlan)
	require.Error(t, err)
	require.Equal(t, "CONFLICT", infraerrors.Reason(err))
	require.Nil(t, result)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.Equal(t, 60.0, reloaded.RefundAmount)
	require.Equal(t, 1, stub.reverseHits, "stale plan must not trigger a second affiliate reversal with an old total")
}

// TestRefundReversalFailureDoesNotFailRefund ensures a clawback error is recorded
// for follow-up but never rolls back the gateway-confirmed refund.
func TestRefundReversalFailureDoesNotFailRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "reversefail")

	stub := &paymentFulfillmentAffiliateRepoStub{reverseErr: errors.New("ledger down")}
	svc := &PaymentService{
		entClient:        client,
		userRepo:         &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
		affiliateService: affiliateServiceWithReverseStub(stub),
	}

	plan, _, err := svc.PrepareRefund(ctx, order.ID, 100, "full refund", false, false)
	require.NoError(t, err)
	plan.Order.PaymentTradeNo = ""
	result, err := svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, reloaded.Status)
	require.True(t, svc.hasAuditLog(ctx, order.ID, "AFFILIATE_REBATE_REVERSAL_FAILED"))
}

// stubInvoiceVoider implements invoiceRefundVoider for refund invoice-gate tests.
type stubInvoiceVoider struct {
	links            map[int64]OrderInvoiceLink
	cancelledInvoice int64
	cancelHits       int
	cancelErr        error
	getErr           error
}

func (s *stubInvoiceVoider) GetActiveLinksByOrderIDs(_ context.Context, orderIDs []int64) (map[int64]OrderInvoiceLink, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	out := map[int64]OrderInvoiceLink{}
	for _, id := range orderIDs {
		if l, ok := s.links[id]; ok {
			out[id] = l
		}
	}
	return out, nil
}

func (s *stubInvoiceVoider) CancelByAdmin(_ context.Context, invoiceID int64) (*InvoiceDetail, error) {
	s.cancelHits++
	s.cancelledInvoice = invoiceID
	if s.cancelErr != nil {
		return nil, s.cancelErr
	}
	return &InvoiceDetail{ID: invoiceID, Status: InvoiceStatusCancelled}, nil
}

// TestPrepareRefundBlocksWhenIssuedInvoiceWithoutForce is the compliance guard:
// an order with an ISSUED (filed) invoice cannot be refunded without force.
func TestPrepareRefundBlocksWhenIssuedInvoiceWithoutForce(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "issuedgate")
	_ = user

	voider := &stubInvoiceVoider{links: map[int64]OrderInvoiceLink{
		order.ID: {InvoiceID: 42, InvoiceStatus: InvoiceStatusIssued, HasFile: true},
	}}
	svc := &PaymentService{entClient: client}
	svc.SetInvoiceVoider(voider)

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "refund", false, false)
	require.NoError(t, err)
	require.Nil(t, plan)
	require.NotNil(t, result)
	require.False(t, result.Success)
	require.True(t, result.RequireForce)
	require.True(t, svc.hasAuditLog(ctx, order.ID, "REFUND_BLOCKED_ISSUED_INVOICE"))
}

func TestPrepareRefundBlocksWhenInvoiceLookupFailsWithoutForce(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "issuedlookupfail")
	_ = user

	voider := &stubInvoiceVoider{getErr: errors.New("invoice lookup failed")}
	svc := &PaymentService{entClient: client}
	svc.SetInvoiceVoider(voider)

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "refund", false, false)
	require.NoError(t, err)
	require.Nil(t, plan)
	require.NotNil(t, result)
	require.False(t, result.Success)
	require.True(t, result.RequireForce)
	require.Contains(t, result.Warning, "invoice state is unavailable")
	require.True(t, svc.hasAuditLog(ctx, order.ID, "REFUND_BLOCKED_INVOICE_LOOKUP_FAILED"))
}

// TestRefundIssuedInvoiceWithForceFlagsCreditNote: with force, refund proceeds and
// records that a credit note (红冲) is needed for the issued invoice.
func TestRefundIssuedInvoiceWithForceFlagsCreditNote(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "issuedforce")

	voider := &stubInvoiceVoider{links: map[int64]OrderInvoiceLink{
		order.ID: {InvoiceID: 42, InvoiceStatus: InvoiceStatusIssued, HasFile: true},
	}}
	svc := &PaymentService{
		entClient: client,
		userRepo:  &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
	}
	svc.SetInvoiceVoider(voider)

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "refund", true, false)
	require.NoError(t, err)
	require.Nil(t, result)
	require.NotNil(t, plan)
	plan.Order.PaymentTradeNo = ""
	result, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)

	require.Zero(t, voider.cancelHits, "issued invoice must not be auto-cancelled")
	require.True(t, svc.hasAuditLog(ctx, order.ID, "INVOICE_NEEDS_CREDIT_NOTE"))
}

// TestRefundAutoCancelsAppliedInvoice: an APPLIED (no file) invoice is auto-cancelled
// after a refund, with no force required.
func TestRefundAutoCancelsAppliedInvoice(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, order := seedRefundBalanceOrder(t, ctx, client, 100, "appliedcancel")

	voider := &stubInvoiceVoider{links: map[int64]OrderInvoiceLink{
		order.ID: {InvoiceID: 77, InvoiceStatus: InvoiceStatusApplied, HasFile: false},
	}}
	svc := &PaymentService{
		entClient: client,
		userRepo:  &refundTestUserRepo{users: map[int64]*User{user.ID: {ID: user.ID, Balance: 100}}},
	}
	svc.SetInvoiceVoider(voider)

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "refund", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	plan.Order.PaymentTradeNo = ""
	result, err = svc.ExecuteRefund(ctx, plan)
	require.NoError(t, err)
	require.True(t, result.Success)

	require.Equal(t, 1, voider.cancelHits)
	require.Equal(t, int64(77), voider.cancelledInvoice)
	require.True(t, svc.hasAuditLog(ctx, order.ID, "INVOICE_AUTO_CANCELLED_AFTER_REFUND"))
}

func createPendingRefundOrderForTest(t *testing.T, ctx context.Context, client *dbent.Client, suffix string) *dbent.PaymentOrder {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(suffix + "@example.com").
		SetPasswordHash("hash").
		SetUsername(suffix).
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeStripe).
		SetName(suffix + "-provider").
		SetConfig("{}").
		SetSupportedTypes("stripe").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-" + suffix).
		SetOutTradeNo("sub2_" + suffix).
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("pi_" + suffix).
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusRefundPending).
		SetRefundAmount(0).
		SetRefundRequestedAmount(100).
		SetRefundReason("pending refund").
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("REFUND_PENDING").
		SetOperator("admin").
		SetDetail(`{"refundID":"rf_test","refundAmount":100,"deductionRollbackOK":true}`).
		Save(ctx)
	require.NoError(t, err)
	return order
}

func replacePaymentProviderFactoryForTest(t *testing.T, prov payment.Provider) func() {
	t.Helper()
	original := createPaymentProviderFromInstance
	createPaymentProviderFromInstance = func(providerKey, instanceID string, config map[string]string) (payment.Provider, error) {
		return prov, nil
	}
	return func() { createPaymentProviderFromInstance = original }
}

func TestFinishRefundPendingMarksOrderPendingAndRollsBackDeduction(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-pending@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-pending-user").
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("REFUND-PENDING-ORDER").
		SetOutTradeNo("sub2_refund_pending_order").
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("pi_refund_pending").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusRefunding).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	var rolledBack float64
	userRepo := &mockUserRepo{}
	userRepo.updateBalanceFn = func(ctx context.Context, id int64, amount float64) error {
		require.Equal(t, user.ID, id)
		rolledBack += amount
		return nil
	}
	svc := &PaymentService{
		entClient: client,
		userRepo:  userRepo,
	}
	plan := &RefundPlan{
		OrderID:          order.ID,
		Order:            order,
		RefundAmount:     40,
		GatewayAmount:    40,
		Reason:           "gateway accepted but not final",
		Force:            true,
		DeductionType:    payment.DeductionTypeBalance,
		BalanceToDeduct:  40,
		DeductionApplied: true,
	}

	result, err := svc.finishRefund(ctx, plan, &payment.RefundResponse{Status: payment.ProviderStatusPending})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Success)
	require.Contains(t, result.Warning, "pending confirmation")
	require.Equal(t, 40.0, rolledBack)
	require.Zero(t, plan.BalanceToDeduct)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, reloaded.Status)
	require.Zero(t, reloaded.RefundAmount)
	require.Equal(t, 40.0, reloaded.RefundRequestedAmount)
	require.NotNil(t, reloaded.RefundReason)
	require.Equal(t, "gateway accepted but not final", *reloaded.RefundReason)
	require.Nil(t, reloaded.RefundAt)

	pendingAudits, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_PENDING")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, pendingAudits)
	successAudits, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_SUCCESS")).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, successAudits)
}

func TestFinishRefundSuccessStatusesFinalize(t *testing.T) {
	for _, status := range []string{payment.ProviderStatusSuccess, payment.ProviderStatusRefunded} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)

			user, err := client.User.Create().
				SetEmail("refund-success-" + status + "@example.com").
				SetPasswordHash("hash").
				SetUsername("refund-success-" + status).
				Save(ctx)
			require.NoError(t, err)

			order, err := client.PaymentOrder.Create().
				SetUserID(user.ID).
				SetUserEmail(user.Email).
				SetUserName(user.Username).
				SetAmount(100).
				SetPayAmount(100).
				SetFeeRate(0).
				SetRechargeCode("REFUND-SUCCESS-" + status).
				SetOutTradeNo("sub2_refund_success_" + status).
				SetPaymentType(payment.TypeStripe).
				SetPaymentTradeNo("pi_refund_success_" + status).
				SetOrderType(payment.OrderTypeBalance).
				SetStatus(OrderStatusRefunding).
				SetExpiresAt(time.Now().Add(time.Hour)).
				SetPaidAt(time.Now()).
				SetClientIP("127.0.0.1").
				SetSrcHost("api.example.com").
				Save(ctx)
			require.NoError(t, err)

			svc := &PaymentService{entClient: client}
			plan := &RefundPlan{
				OrderID:         order.ID,
				Order:           order,
				RefundAmount:    100,
				GatewayAmount:   100,
				Reason:          "final success",
				DeductionType:   payment.DeductionTypeBalance,
				BalanceToDeduct: 100,
			}

			result, err := svc.finishRefund(ctx, plan, &payment.RefundResponse{Status: status})
			require.NoError(t, err)
			require.NotNil(t, result)
			require.True(t, result.Success)
			require.Equal(t, 100.0, result.BalanceDeducted)

			reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, OrderStatusRefunded, reloaded.Status)
			require.NotNil(t, reloaded.RefundAt)

			successAudits, err := client.PaymentAuditLog.Query().
				Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_SUCCESS")).
				Count(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, successAudits)
			pendingAudits, err := client.PaymentAuditLog.Query().
				Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_PENDING")).
				Count(ctx)
			require.NoError(t, err)
			require.Zero(t, pendingAudits)
		})
	}
}

func TestPrepareRefundAllowsPaymentTypeOnlyLegacyProviderInstance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-legacy-admin@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-legacy-admin-user").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-admin-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetAllowUserRefund(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(188).
		SetPayAmount(188).
		SetFeeRate(0).
		SetRechargeCode("REFUND-LEGACY-ADMIN-ORDER").
		SetOutTradeNo("sub2_refund_legacy_admin_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-legacy-admin-refund").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, false)
	require.Nil(t, result)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Equal(t, order.ID, plan.OrderID)
	require.Equal(t, 188.0, plan.RefundAmount)
}

func TestQueryAndFinalizeRefundFinalizesProviderStatuses(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     string
		wantStatus string
		wantDeduct float64
	}{
		{name: "success", status: payment.ProviderStatusSuccess, wantStatus: OrderStatusRefunded, wantDeduct: 100},
		{name: "failed", status: payment.ProviderStatusFailed, wantStatus: OrderStatusRefundFailed},
		{name: "pending", status: payment.ProviderStatusPending, wantStatus: OrderStatusRefundPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createPendingRefundOrderForTest(t, ctx, client, "query-finalize-"+tc.name)

			var deducted float64
			svc := &PaymentService{
				entClient:    client,
				loadBalancer: &captureLoadBalancer{},
				userRepo: &mockUserRepo{
					getByIDUser: &User{ID: order.UserID, Balance: 100},
					deductBalanceFn: func(ctx context.Context, id int64, amount float64) error {
						deducted += amount
						return nil
					},
				},
			}
			restore := replacePaymentProviderFactoryForTest(t, &refundQueryProviderTestDouble{
				refundResponse: &payment.RefundResponse{RefundID: "rf_test", Status: tc.status},
			})
			defer restore()

			result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tc.status == payment.ProviderStatusSuccess, result.Success)
			require.Equal(t, tc.wantDeduct, deducted)

			reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, reloaded.Status)
		})
	}
}

func TestQueryAndFinalizeRefundUsesPendingAuditAmountWithoutDoubling(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "query-finalize-no-double")

	var deducted float64
	svc := &PaymentService{
		entClient:    client,
		loadBalancer: &captureLoadBalancer{},
		userRepo: &mockUserRepo{
			getByIDUser: &User{ID: order.UserID, Balance: 100},
			deductBalanceFn: func(ctx context.Context, id int64, amount float64) error {
				deducted += amount
				return nil
			},
		},
	}
	queryProvider := &refundQueryProviderTestDouble{
		refundResponse: &payment.RefundResponse{RefundID: "rf_test", Status: payment.ProviderStatusSuccess},
	}
	restore := replacePaymentProviderFactoryForTest(t, queryProvider)
	defer restore()

	result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)
	require.Equal(t, 100.0, deducted)
	require.Equal(t, 1, queryProvider.queryCalls)
	require.Equal(t, "rf_test", queryProvider.lastQuery.RefundID)
	require.Equal(t, refundProviderRequestID(&RefundPlan{
		OrderID:       order.ID,
		Order:         order,
		RefundAmount:  100,
		GatewayAmount: 100,
	}), queryProvider.lastQuery.RequestID)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, reloaded.Status)
	require.Equal(t, 100.0, reloaded.RefundAmount)
}

func TestQueryAndFinalizeRefundCapsBalanceDeductionToCurrentBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "query-finalize-balance-cap")

	var deducted float64
	svc := &PaymentService{
		entClient:    client,
		loadBalancer: &captureLoadBalancer{},
		userRepo: &mockUserRepo{
			getByIDUser: &User{ID: order.UserID, Balance: 10},
			deductBalanceFn: func(ctx context.Context, id int64, amount float64) error {
				deducted += amount
				return nil
			},
		},
	}
	restore := replacePaymentProviderFactoryForTest(t, &refundQueryProviderTestDouble{
		refundResponse: &payment.RefundResponse{RefundID: "rf_test", Status: payment.ProviderStatusSuccess},
	})
	defer restore()

	result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)
	require.Equal(t, 10.0, deducted)
	require.Equal(t, 10.0, result.BalanceDeducted)
}

func TestQueryAndFinalizeRefundAddsOnlyPendingAmountToExistingRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "query-finalize-partial")
	_, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetRefundAmount(30).
		SetRefundRequestedAmount(20).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("REFUND_SUCCESS").
		SetOperator("admin").
		SetDetail(`{"refundAmount":30,"totalRefunded":30,"reason":"first partial","balanceDeducted":30}`).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("REFUND_PENDING").
		SetOperator("admin").
		SetDetail(`{"refundID":"rf_second","refundAmount":20,"deductionRollbackOK":true}`).
		Save(ctx)
	require.NoError(t, err)

	var deducted float64
	svc := &PaymentService{
		entClient:    client,
		loadBalancer: &captureLoadBalancer{},
		userRepo: &mockUserRepo{
			getByIDUser: &User{ID: order.UserID, Balance: 20},
			deductBalanceFn: func(ctx context.Context, id int64, amount float64) error {
				deducted += amount
				return nil
			},
		},
	}
	restore := replacePaymentProviderFactoryForTest(t, &refundQueryProviderTestDouble{
		refundResponse: &payment.RefundResponse{RefundID: "rf_second", Status: payment.ProviderStatusSuccess},
	})
	defer restore()

	result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, 20.0, deducted)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPartiallyRefunded, reloaded.Status)
	require.Equal(t, 50.0, reloaded.RefundAmount)
}

func TestQueryAndFinalizeRefundRestoresPendingWhenFinalDeductionFails(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "query-finalize-deduct-fail")

	deductErr := errors.New("balance deduction unavailable")
	var deducted float64
	svc := &PaymentService{
		entClient:    client,
		loadBalancer: &captureLoadBalancer{},
		userRepo: &mockUserRepo{
			getByIDUser: &User{ID: order.UserID, Balance: 100},
			deductBalanceFn: func(ctx context.Context, id int64, amount float64) error {
				deducted += amount
				return deductErr
			},
		},
	}
	restore := replacePaymentProviderFactoryForTest(t, &refundQueryProviderTestDouble{
		refundResponse: &payment.RefundResponse{RefundID: "rf_test", Status: payment.ProviderStatusSuccess},
	})
	defer restore()

	result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)

	require.Nil(t, result)
	require.Error(t, err)
	require.ErrorIs(t, err, deductErr)
	require.Equal(t, 100.0, deducted)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, reloaded.Status)
	require.Zero(t, reloaded.RefundAmount)

	exists, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_FINAL_DEDUCTION_FAILED")).
		Exist(ctx)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestQueryAndFinalizeRefundRestoresPendingWhenRefundPersistFailsAfterDeduction(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "query-finalize-persist-fail")

	var deducted float64
	var restored float64
	svc := &PaymentService{
		entClient:    client,
		loadBalancer: &captureLoadBalancer{},
		userRepo: &mockUserRepo{
			getByIDUser: &User{ID: order.UserID, Balance: 100},
			deductBalanceFn: func(ctx context.Context, id int64, amount float64) error {
				deducted += amount
				return nil
			},
			updateBalanceFn: func(ctx context.Context, id int64, amount float64) error {
				restored += amount
				return nil
			},
		},
	}
	restore := replacePaymentProviderFactoryForTest(t, &refundQueryProviderTestDouble{
		refundResponse: &payment.RefundResponse{RefundID: "rf_query_final", Status: payment.ProviderStatusSuccess},
	})
	defer restore()

	trigger := fmt.Sprintf(`
		CREATE TRIGGER fail_refund_finalize_once
		BEFORE UPDATE OF status ON payment_orders
		WHEN NEW.id = %d AND NEW.status = '%s'
		BEGIN
			SELECT RAISE(FAIL, 'forced refund finalize failure');
		END;
	`, order.ID, OrderStatusRefunded)
	_, err := client.ExecContext(ctx, trigger)
	require.NoError(t, err)

	result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "forced refund finalize failure")
	require.Equal(t, 100.0, deducted)
	require.Equal(t, 100.0, restored)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundPending, reloaded.Status)

	exists, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_FINALIZE_PERSIST_FAILED")).
		Exist(ctx)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestPrepareRefundRejectsPendingRefundToAvoidDuplicateProviderRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "prepare-pending-reject")

	svc := &PaymentService{entClient: client}
	plan, result, err := svc.PrepareRefund(ctx, order.ID, 100, "retry provider refund", false, true)
	require.Nil(t, plan)
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "REFUND_PENDING", infraerrors.Reason(err))
}

func TestQueryAndFinalizeRefundUnsupportedProviderReturnsClearError(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "query-finalize-unsupported")
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}}
	restore := replacePaymentProviderFactoryForTest(t, &refundProviderTestDouble{})
	defer restore()

	result, err := svc.QueryAndFinalizeRefund(ctx, order.ID)
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, "REFUND_QUERY_UNSUPPORTED", infraerrors.Reason(err))
}

func TestValidateRefundRequestAllowsPaymentTypeOnlyLegacyProviderInstance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, err := client.User.Create().
		SetEmail("refund-legacy@example.com").
		SetPasswordHash("hash").
		SetUsername("refund-legacy-user").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("alipay-refund-instance").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetAllowUserRefund(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("REFUND-LEGACY-ORDER").
		SetOutTradeNo("sub2_refund_legacy_order").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-legacy-refund").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
	}

	_, _, _, err = svc.validateRefundRequest(ctx, order.ID, user.ID)
	require.NoError(t, err)
}

type refundProviderTestDouble struct {
	refundCalls int
	lastRefund  payment.RefundRequest
}

func (p *refundProviderTestDouble) Name() string { return "refund-test" }
func (p *refundProviderTestDouble) ProviderKey() string {
	return payment.TypeStripe
}
func (p *refundProviderTestDouble) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeStripe}
}
func (p *refundProviderTestDouble) CreatePayment(context.Context, payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	return nil, nil
}
func (p *refundProviderTestDouble) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	return nil, nil
}
func (p *refundProviderTestDouble) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	return nil, nil
}
func (p *refundProviderTestDouble) Refund(_ context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	p.refundCalls++
	p.lastRefund = req
	return &payment.RefundResponse{RefundID: "rf_test", Status: payment.ProviderStatusSuccess}, nil
}

type refundQueryProviderTestDouble struct {
	refundProviderTestDouble
	refundResponse *payment.RefundResponse
	queryCalls     int
	lastQuery      payment.RefundQueryRequest
}

func (p *refundQueryProviderTestDouble) QueryRefund(_ context.Context, req payment.RefundQueryRequest) (*payment.RefundResponse, error) {
	p.queryCalls++
	p.lastQuery = req
	return p.refundResponse, nil
}
