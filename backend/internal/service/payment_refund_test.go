//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

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

func TestValidateRefundRequestRejectsLegacyGuessedProviderInstance(t *testing.T) {
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

	_, err = svc.validateRefundRequest(ctx, order.ID, user.ID)
	require.Error(t, err)
	require.Equal(t, "USER_REFUND_DISABLED", infraerrors.Reason(err))
}

func TestPrepareRefundRejectsLegacyGuessedProviderInstance(t *testing.T) {
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
		SetAllowUserRefund(true).
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

	require.NoError(t, svc.RequestRefund(ctx, order.ID, user.ID, "please refund"))

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundRequested, reloaded.Status)
	require.Zero(t, reloaded.RefundAmount)

	plan, result, err := svc.PrepareRefund(ctx, order.ID, 0, "", false, false)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Equal(t, 100.0, plan.RefundAmount)
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

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.Zero(t, reloaded.RefundAmount)
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
