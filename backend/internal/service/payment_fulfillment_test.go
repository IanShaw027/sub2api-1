//go:build unit

package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite"
)

func TestCalculateGatewayRefundAmountUsesStrictCentComparison(t *testing.T) {
	got := calculateGatewayRefundAmount(100, 80, 99.99, "CNY")
	require.True(t, math.Abs(got-79.99) < 0.000001, "got %.8f", got)
}

type paymentFulfillmentTestProvider struct {
	key            string
	supportedTypes []payment.PaymentType
}

func (p paymentFulfillmentTestProvider) Name() string        { return p.key }
func (p paymentFulfillmentTestProvider) ProviderKey() string { return p.key }
func (p paymentFulfillmentTestProvider) SupportedTypes() []payment.PaymentType {
	return p.supportedTypes
}
func (p paymentFulfillmentTestProvider) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	panic("unexpected call")
}
func (p paymentFulfillmentTestProvider) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	panic("unexpected call")
}
func (p paymentFulfillmentTestProvider) VerifyNotification(ctx context.Context, rawBody string, headers map[string]string) (*payment.PaymentNotification, error) {
	panic("unexpected call")
}
func (p paymentFulfillmentTestProvider) Refund(ctx context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	panic("unexpected call")
}

type paymentFulfillmentGroupRepoStub struct {
	GroupRepository
	group *Group
}

func (s paymentFulfillmentGroupRepoStub) GetByID(context.Context, int64) (*Group, error) {
	return s.group, nil
}

type paymentFulfillmentUserSubRepoStub struct {
	UserSubscriptionRepository
	existing    *UserSubscription
	extendCalls int
	getByIDErr  error
}

func (s *paymentFulfillmentUserSubRepoStub) GetByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	return s.existing, nil
}

func (s *paymentFulfillmentUserSubRepoStub) GetByID(context.Context, int64) (*UserSubscription, error) {
	if s.getByIDErr != nil {
		return nil, s.getByIDErr
	}
	return s.existing, nil
}

func (s *paymentFulfillmentUserSubRepoStub) ExtendExpiry(context.Context, int64, time.Time) error {
	s.extendCalls++
	return nil
}

func (s *paymentFulfillmentUserSubRepoStub) Update(_ context.Context, sub *UserSubscription) error {
	s.extendCalls++
	s.existing = sub
	return nil
}

func (s *paymentFulfillmentUserSubRepoStub) UpdateStatus(context.Context, int64, string) error {
	return nil
}

func (s *paymentFulfillmentUserSubRepoStub) UpdateNotes(context.Context, int64, string) error {
	return nil
}

func newPaymentFulfillmentTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	dbName := "file:payment_fulfillment_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "?mode=memory&cache=shared&_fk=1"
	db, err := sql.Open("sqlite", dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func createPaymentFulfillmentOrder(t *testing.T, client *dbent.Client, status string, orderType string) *dbent.PaymentOrder {
	t.Helper()
	ctx := context.Background()

	user, err := client.User.Create().
		SetEmail("fulfillment@example.com").
		SetPasswordHash("hash").
		SetUsername("fulfillment-user").
		Save(ctx)
	require.NoError(t, err)

	create := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("FULFILLMENT-CODE").
		SetOutTradeNo("sub2_fulfillment_order").
		SetPaymentTradeNo("").
		SetPaymentType(payment.TypeAlipay).
		SetOrderType(orderType).
		SetStatus(status).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com")
	if orderType == payment.OrderTypeSubscription {
		gid := int64(10)
		days := 30
		create.SetSubscriptionGroupID(gid).SetSubscriptionDays(days)
	}
	order, err := create.Save(ctx)
	require.NoError(t, err)
	return order
}

func TestExecuteBalanceFulfillmentRollsBackCreditWhenCompletionUpdateFails(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeBalance)

	svc := &PaymentService{entClient: client}

	trigger := fmt.Sprintf(`
		CREATE TRIGGER fail_balance_completion
		BEFORE UPDATE OF status ON payment_orders
		WHEN NEW.id = %d AND NEW.status = '%s'
		BEGIN
			SELECT RAISE(FAIL, 'forced completion failure');
		END;
	`, order.ID, OrderStatusCompleted)
	_, err := client.ExecContext(ctx, trigger)
	require.NoError(t, err)

	err = svc.ExecuteBalanceFulfillment(ctx, order.ID)
	require.Error(t, err)

	gotOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusFailed, gotOrder.Status)
	require.NotNil(t, gotOrder.FailedAt)

	gotUser, err := client.User.Get(ctx, order.UserID)
	require.NoError(t, err)
	require.Zero(t, gotUser.Balance)
	require.Zero(t, gotUser.TotalRecharged)

	codeCount, err := client.RedeemCode.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, codeCount)

	successCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("RECHARGE_SUCCESS")).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, successCount)
}

func TestConfirmPaymentRejectsStripeCurrencyMismatch(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPending, payment.OrderTypeBalance)
	_, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("pi_currency_mismatch").
		SetProviderKey(payment.TypeStripe).
		SetProviderSnapshot(map[string]any{
			"schema_version": 2,
			"provider_key":   payment.TypeStripe,
			"currency":       "CNY",
		}).
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{entClient: client}
	err = svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		OrderID:  order.OutTradeNo,
		TradeNo:  "pi_currency_mismatch",
		Amount:   100,
		Status:   payment.NotificationStatusSuccess,
		Metadata: map[string]string{"currency": "usd"},
	}, payment.TypeStripe)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stripe currency mismatch")

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, reloaded.Status)
}

func TestHandlePaymentNotificationCancelledOrderWithinGraceRecoversAndFulfills(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusCancelled, payment.OrderTypeBalance)

	svc := &PaymentService{entClient: client}
	err := svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		OrderID: order.OutTradeNo,
		TradeNo: "provider-trade-cancelled",
		Amount:  100,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeAlipay)
	require.NoError(t, err)

	got, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, got.Status)
	require.Equal(t, "provider-trade-cancelled", got.PaymentTradeNo)
	require.NotNil(t, got.PaidAt)
	require.NotNil(t, got.CompletedAt)
	orderID := strconv.FormatInt(order.ID, 10)

	paidCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(orderID), paymentauditlog.ActionEQ("ORDER_PAID")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, paidCount)

	lateCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(orderID), paymentauditlog.ActionEQ("PAYMENT_AFTER_CANCELLED")).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, lateCount)

	recoveredCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(orderID), paymentauditlog.ActionEQ("ORDER_RECOVERED")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, recoveredCount)
}

func TestHandlePaymentNotificationCancelledOrderBeyondGraceDoesNotFulfill(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusCancelled, payment.OrderTypeBalance)
	_, err := client.PaymentOrder.UpdateOneID(order.ID).SetUpdatedAt(time.Now().Add(-2 * paymentGraceMinutes * time.Minute)).Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{entClient: client}
	err = svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		OrderID: order.OutTradeNo,
		TradeNo: "provider-trade-cancelled-late",
		Amount:  100,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeAlipay)
	require.NoError(t, err)

	got, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCancelled, got.Status)
	require.Empty(t, got.PaymentTradeNo)
	require.Nil(t, got.PaidAt)

	orderID := strconv.FormatInt(order.ID, 10)
	lateCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(orderID), paymentauditlog.ActionEQ("PAYMENT_AFTER_CANCELLED")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, lateCount)
}

func TestHandlePaymentNotificationFailedOrderReconcilesPaidMetadataBeforeFulfillment(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusFailed, payment.OrderTypeBalance)
	_, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetFailedAt(time.Now().Add(-2 * time.Minute)).
		SetFailedReason("lost callback after local failure").
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{entClient: client}
	err = svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		OrderID: order.OutTradeNo,
		TradeNo: "provider-trade-failed-recovered",
		Amount:  100,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeAlipay)
	require.NoError(t, err)

	got, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, got.Status)
	require.Equal(t, "provider-trade-failed-recovered", got.PaymentTradeNo)
	require.NotNil(t, got.PaidAt)
	require.NotNil(t, got.CompletedAt)
	require.Nil(t, got.FailedAt)
	require.Nil(t, got.FailedReason)
}

func TestHandlePaymentNotificationExpiredBeyondGraceAuditsProviderTradeNo(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusExpired, payment.OrderTypeBalance)
	_, err := client.PaymentOrder.UpdateOneID(order.ID).SetUpdatedAt(time.Now().Add(-2 * paymentGraceMinutes * time.Minute)).Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{entClient: client}
	err = svc.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		OrderID: order.OutTradeNo,
		TradeNo: "provider-trade-expired",
		Amount:  100,
		Status:  payment.NotificationStatusSuccess,
	}, payment.TypeAlipay)
	require.NoError(t, err)

	got, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusExpired, got.Status)
	require.Empty(t, got.PaymentTradeNo)
	require.Nil(t, got.PaidAt)
	orderID := strconv.FormatInt(order.ID, 10)

	logs, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(orderID), paymentauditlog.ActionEQ("PAYMENT_AFTER_EXPIRY")).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, payment.TypeAlipay, logs[0].Operator)
	require.Contains(t, logs[0].Detail, "provider-trade-expired")
}

func TestExecuteSubscriptionFulfillmentCompletesWhenPostCommitReloadFails(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeSubscription)

	repo := &paymentFulfillmentUserSubRepoStub{existing: &UserSubscription{
		ID:        88,
		UserID:    order.UserID,
		GroupID:   *order.SubscriptionGroupID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    SubscriptionStatusActive,
	}, getByIDErr: errors.New("reload failed after commit")}
	groupRepo := paymentFulfillmentGroupRepoStub{group: &Group{
		ID:                  *order.SubscriptionGroupID,
		Status:              payment.EntityStatusActive,
		SubscriptionType:    SubscriptionTypeSubscription,
		DefaultValidityDays: 30,
	}}
	subscriptionSvc := NewSubscriptionService(groupRepo, repo, nil, client, nil)
	svc := &PaymentService{entClient: client, groupRepo: groupRepo, subscriptionSvc: subscriptionSvc}

	err := svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, 1, repo.extendCalls)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.NotNil(t, reloaded.CompletedAt)
	require.Nil(t, reloaded.FailedAt)
	require.True(t, svc.hasAuditLog(ctx, order.ID, "SUBSCRIPTION_FULFILLMENT_CLAIMED"))
}

func TestExecuteSubscriptionFulfillmentRetryAfterClaimDoesNotExtendAgain(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusFailed, payment.OrderTypeSubscription)

	repo := &paymentFulfillmentUserSubRepoStub{existing: &UserSubscription{
		ID:        77,
		UserID:    order.UserID,
		GroupID:   *order.SubscriptionGroupID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    SubscriptionStatusActive,
	}}
	groupRepo := paymentFulfillmentGroupRepoStub{group: &Group{
		ID:                  *order.SubscriptionGroupID,
		Status:              payment.EntityStatusActive,
		SubscriptionType:    SubscriptionTypeSubscription,
		DefaultValidityDays: 30,
	}}
	subscriptionSvc := NewSubscriptionService(groupRepo, repo, nil, client, nil)
	svc := &PaymentService{entClient: client, groupRepo: groupRepo, subscriptionSvc: subscriptionSvc}

	err := svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, 1, repo.extendCalls)

	_, err = client.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(OrderStatusFailed).
		ClearCompletedAt().
		Save(ctx)
	require.NoError(t, err)

	err = svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, 1, repo.extendCalls)

	claimCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("SUBSCRIPTION_FULFILLMENT_CLAIMED")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, claimCount)

	successCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("SUBSCRIPTION_SUCCESS")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, successCount)
}

func TestExecuteSubscriptionFulfillmentRetryWithOrphanedClaimStillAssignsSubscription(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusFailed, payment.OrderTypeSubscription)

	repo := &paymentFulfillmentUserSubRepoStub{existing: &UserSubscription{
		ID:        66,
		UserID:    order.UserID,
		GroupID:   *order.SubscriptionGroupID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    SubscriptionStatusActive,
	}}
	groupRepo := paymentFulfillmentGroupRepoStub{group: &Group{
		ID:                  *order.SubscriptionGroupID,
		Status:              payment.EntityStatusActive,
		SubscriptionType:    SubscriptionTypeSubscription,
		DefaultValidityDays: 30,
	}}
	subscriptionSvc := NewSubscriptionService(groupRepo, repo, nil, client, nil)
	svc := &PaymentService{entClient: client, groupRepo: groupRepo, subscriptionSvc: subscriptionSvc}

	svc.writeAuditLog(ctx, order.ID, "SUBSCRIPTION_FULFILLMENT_CLAIMED", "system", map[string]any{
		"groupID": *order.SubscriptionGroupID,
		"days":    *order.SubscriptionDays,
		"status":  "reserved",
	})

	err := svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, 1, repo.extendCalls)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
	require.NotNil(t, reloaded.CompletedAt)

	claimCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("SUBSCRIPTION_FULFILLMENT_CLAIMED")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, claimCount)

	successCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("SUBSCRIPTION_SUCCESS")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, successCount)
}

func TestSubscriptionFulfillmentWritesSuccessSentinelBeforeOrderCompletion(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeSubscription)

	repo := &paymentFulfillmentUserSubRepoStub{existing: &UserSubscription{
		ID:        55,
		UserID:    order.UserID,
		GroupID:   *order.SubscriptionGroupID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    SubscriptionStatusActive,
	}}
	groupRepo := paymentFulfillmentGroupRepoStub{group: &Group{
		ID:                  *order.SubscriptionGroupID,
		Status:              payment.EntityStatusActive,
		SubscriptionType:    SubscriptionTypeSubscription,
		DefaultValidityDays: 30,
	}}
	subscriptionSvc := NewSubscriptionService(groupRepo, repo, nil, client, nil)
	var notifications []string
	svc := &PaymentService{
		entClient:       client,
		groupRepo:       groupRepo,
		subscriptionSvc: subscriptionSvc,
		notificationDispatchHook: func(order *dbent.PaymentOrder, auditAction string) {
			if order != nil {
				notifications = append(notifications, auditAction)
			}
		},
	}

	trigger := fmt.Sprintf(`
		CREATE TRIGGER fail_subscription_completion_once
		BEFORE UPDATE OF status ON payment_orders
		WHEN NEW.id = %d AND NEW.status = '%s'
		BEGIN
			SELECT RAISE(FAIL, 'forced subscription completion failure');
		END;
	`, order.ID, OrderStatusCompleted)
	_, err := client.ExecContext(ctx, trigger)
	require.NoError(t, err)

	err = svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mark completed")
	require.Equal(t, 1, repo.extendCalls)

	successCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("SUBSCRIPTION_SUCCESS")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, successCount)

	_, err = client.ExecContext(ctx, `DROP TRIGGER fail_subscription_completion_once`)
	require.NoError(t, err)

	err = svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, 1, repo.extendCalls)
	require.Equal(t, []string{"SUBSCRIPTION_SUCCESS"}, notifications)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
}

// 回归防线：订阅发放与 SUCCESS 哨兵必须原子——哨兵写入失败时 assign 整体回滚、claim 释放，
// 不留「已发放但无哨兵」的孤儿状态（正是该状态会让 orphaned-claim 恢复路径重复发放）；
// 重试则干净发放一次。
func TestSubscriptionFulfillmentRollsBackAssignWhenSuccessSentinelWriteFails(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeSubscription)

	repo := &paymentFulfillmentUserSubRepoStub{existing: &UserSubscription{
		ID:        88,
		UserID:    order.UserID,
		GroupID:   *order.SubscriptionGroupID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Status:    SubscriptionStatusActive,
	}}
	groupRepo := paymentFulfillmentGroupRepoStub{group: &Group{
		ID:                  *order.SubscriptionGroupID,
		Status:              payment.EntityStatusActive,
		SubscriptionType:    SubscriptionTypeSubscription,
		DefaultValidityDays: 30,
	}}
	subscriptionSvc := NewSubscriptionService(groupRepo, repo, nil, client, nil)
	svc := &PaymentService{entClient: client, groupRepo: groupRepo, subscriptionSvc: subscriptionSvc}

	trigger := fmt.Sprintf(`
		CREATE TRIGGER fail_subscription_success_sentinel_once
		BEFORE INSERT ON payment_audit_logs
		WHEN NEW.action = 'SUBSCRIPTION_SUCCESS' AND NEW.order_id = '%d'
		BEGIN
			SELECT RAISE(FAIL, 'forced subscription success sentinel failure');
		END;
	`, order.ID)
	_, err := client.ExecContext(ctx, trigger)
	require.NoError(t, err)

	err = svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subscription success audit")

	// 哨兵未持久化（assign+哨兵 事务整体回滚）
	successCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("SUBSCRIPTION_SUCCESS")).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, successCount)

	// claim 已释放，重试重新发放而非卡在 orphaned-claim
	claimCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("SUBSCRIPTION_FULFILLMENT_CLAIMED")).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, claimCount)

	// 订单未被标记完成
	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.NotEqual(t, OrderStatusCompleted, reloaded.Status)

	// 移除故障后重试：干净发放一次并完成
	_, err = client.ExecContext(ctx, `DROP TRIGGER fail_subscription_success_sentinel_once`)
	require.NoError(t, err)

	err = svc.ExecuteSubscriptionFulfillment(ctx, order.ID)
	require.NoError(t, err)

	successCount, err = client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("SUBSCRIPTION_SUCCESS")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, successCount)

	reloaded, err = client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloaded.Status)
}

func TestTryClaimSubscriptionFulfillmentAuditSkipsExistingSuccessSentinel(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeSubscription)
	svc := &PaymentService{entClient: client}

	svc.writeAuditLog(ctx, order.ID, "SUBSCRIPTION_SUCCESS", "system", map[string]any{"status": "completed"})

	claimed, err := svc.tryClaimSubscriptionFulfillmentAudit(ctx, order, *order.SubscriptionGroupID, *order.SubscriptionDays)
	require.NoError(t, err)
	require.False(t, claimed)

	claimCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("SUBSCRIPTION_FULFILLMENT_CLAIMED")).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, claimCount)
}

func TestTryClaimSubscriptionFulfillmentAuditCreatesClaimLog(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeSubscription)
	svc := &PaymentService{entClient: client}

	claimed, err := svc.tryClaimSubscriptionFulfillmentAudit(ctx, order, *order.SubscriptionGroupID, *order.SubscriptionDays)
	require.NoError(t, err)
	require.True(t, claimed)

	entry, err := client.PaymentAuditLog.Query().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
			paymentauditlog.ActionEQ("SUBSCRIPTION_FULFILLMENT_CLAIMED"),
		).
		Only(ctx)
	require.NoError(t, err)
	require.Contains(t, entry.Detail, `"groupID":`)
	require.Contains(t, entry.Detail, `"status":"reserved"`)
}

func TestTryClaimAffiliateRebateAuditIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client := newPaymentFulfillmentTestClient(t)
	order := createPaymentFulfillmentOrder(t, client, OrderStatusPaid, payment.OrderTypeBalance)
	svc := &PaymentService{entClient: client}

	claimed, err := svc.tryClaimAffiliateRebateAudit(ctx, client, order.ID, order.Amount)
	require.NoError(t, err)
	require.True(t, claimed)

	claimed, err = svc.tryClaimAffiliateRebateAudit(ctx, client, order.ID, order.Amount)
	require.NoError(t, err)
	require.False(t, claimed)

	entry, err := client.PaymentAuditLog.Query().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
			paymentauditlog.ActionEQ("AFFILIATE_REBATE_APPLIED"),
		).
		Only(ctx)
	require.NoError(t, err)
	require.Contains(t, entry.Detail, `"baseAmount":100`)
	require.Contains(t, entry.Detail, `"status":"reserved"`)
}

func TestSubscriptionFulfillmentClaimMigrationIncludesSubscriptionSentinels(t *testing.T) {
	body, err := os.ReadFile("../../migrations/138_subscription_fulfillment_claim_unique_notx.sql")
	require.NoError(t, err)
	sql := string(body)
	require.Contains(t, sql, "idx_payment_audit_logs_order_action_uniq")
	require.Contains(t, sql, "SUBSCRIPTION_FULFILLMENT_CLAIMED")
	require.Contains(t, sql, "SUBSCRIPTION_SUCCESS")
	require.Contains(t, sql, "AFFILIATE_REBATE_APPLIED")
	require.Contains(t, sql, "AFFILIATE_REBATE_SKIPPED")
	require.Contains(t, sql, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS")
	require.Contains(t, sql, "DROP INDEX CONCURRENTLY IF EXISTS")
}

func TestSubscriptionFulfillmentClaimDedupeMigrationRemovesHistoricalDuplicates(t *testing.T) {
	body, err := os.ReadFile("../../migrations/137_subscription_fulfillment_claim_dedupe.sql")
	require.NoError(t, err)
	sql := string(body)
	require.Contains(t, sql, "ROW_NUMBER() OVER (PARTITION BY order_id, action ORDER BY id)")
	require.Contains(t, sql, "DELETE FROM payment_audit_logs")
	require.Contains(t, sql, "SUBSCRIPTION_FULFILLMENT_CLAIMED")
	require.Contains(t, sql, "SUBSCRIPTION_SUCCESS")
	require.Contains(t, sql, "AFFILIATE_REBATE_APPLIED")
	require.Contains(t, sql, "AFFILIATE_REBATE_SKIPPED")
}

// ---------------------------------------------------------------------------
// resolveRedeemAction — pure idempotency decision logic
// ---------------------------------------------------------------------------

func TestResolveRedeemAction_CodeNotFound(t *testing.T) {
	t.Parallel()
	action := resolveRedeemAction(nil, nil)
	assert.Equal(t, redeemActionCreate, action, "nil code with nil error should create")
}

func TestResolveRedeemAction_LookupError(t *testing.T) {
	t.Parallel()
	action := resolveRedeemAction(nil, errors.New("db connection lost"))
	assert.Equal(t, redeemActionCreate, action, "lookup error should fall back to create")
}

func TestResolveRedeemAction_LookupErrorWithNonNilCode(t *testing.T) {
	t.Parallel()
	// Edge case: both code and error are non-nil (shouldn't happen in practice,
	// but the function should still treat error as authoritative)
	code := &RedeemCode{Status: StatusUnused}
	action := resolveRedeemAction(code, errors.New("partial error"))
	assert.Equal(t, redeemActionCreate, action, "non-nil error should always result in create regardless of code")
}

func TestResolveRedeemAction_CodeExistsAndUsed(t *testing.T) {
	t.Parallel()
	code := &RedeemCode{
		Code:   "test-code-123",
		Status: StatusUsed,
		Type:   RedeemTypeBalance,
		Value:  10.0,
	}
	action := resolveRedeemAction(code, nil)
	assert.Equal(t, redeemActionSkipCompleted, action, "used code should skip to completed")
}

func TestResolveRedeemAction_CodeExistsAndUnused(t *testing.T) {
	t.Parallel()
	code := &RedeemCode{
		Code:   "test-code-456",
		Status: StatusUnused,
		Type:   RedeemTypeBalance,
		Value:  25.0,
	}
	action := resolveRedeemAction(code, nil)
	assert.Equal(t, redeemActionRedeem, action, "unused code should skip creation and proceed to redeem")
}

func TestResolveRedeemAction_CodeExistsWithExpiredStatus(t *testing.T) {
	t.Parallel()
	// A code with a non-standard status (neither "unused" nor "used")
	// should NOT be treated as used, so it falls through to redeemActionRedeem.
	code := &RedeemCode{
		Code:   "expired-code",
		Status: StatusExpired,
	}
	action := resolveRedeemAction(code, nil)
	assert.Equal(t, redeemActionRedeem, action, "expired-status code is not IsUsed(), should redeem")
}

// ---------------------------------------------------------------------------
// Table-driven comprehensive test
// ---------------------------------------------------------------------------

func TestResolveRedeemAction_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		code     *RedeemCode
		err      error
		expected redeemAction
	}{
		{
			name:     "nil code, nil error — first run",
			code:     nil,
			err:      nil,
			expected: redeemActionCreate,
		},
		{
			name:     "nil code, lookup error — treat as not found",
			code:     nil,
			err:      ErrRedeemCodeNotFound,
			expected: redeemActionCreate,
		},
		{
			name:     "nil code, generic DB error — treat as not found",
			code:     nil,
			err:      errors.New("connection refused"),
			expected: redeemActionCreate,
		},
		{
			name:     "code exists, used — previous run completed redeem",
			code:     &RedeemCode{Status: StatusUsed},
			err:      nil,
			expected: redeemActionSkipCompleted,
		},
		{
			name:     "code exists, unused — previous run created code but crashed before redeem",
			code:     &RedeemCode{Status: StatusUnused},
			err:      nil,
			expected: redeemActionRedeem,
		},
		{
			name:     "code exists but error also set — error takes precedence",
			code:     &RedeemCode{Status: StatusUsed},
			err:      errors.New("unexpected"),
			expected: redeemActionCreate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveRedeemAction(tt.code, tt.err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// ---------------------------------------------------------------------------
// redeemAction enum value sanity
// ---------------------------------------------------------------------------

func TestRedeemAction_DistinctValues(t *testing.T) {
	t.Parallel()
	// Ensure the three actions have distinct values (iota correctness)
	assert.NotEqual(t, redeemActionCreate, redeemActionRedeem)
	assert.NotEqual(t, redeemActionCreate, redeemActionSkipCompleted)
	assert.NotEqual(t, redeemActionRedeem, redeemActionSkipCompleted)
}

// ---------------------------------------------------------------------------
// RedeemCode.IsUsed / CanUse interaction with resolveRedeemAction
// ---------------------------------------------------------------------------

func TestResolveRedeemAction_IsUsedCanUseConsistency(t *testing.T) {
	t.Parallel()

	usedCode := &RedeemCode{Status: StatusUsed}
	unusedCode := &RedeemCode{Status: StatusUnused}

	// Verify our decision function is consistent with the domain model methods
	assert.True(t, usedCode.IsUsed())
	assert.False(t, usedCode.CanUse())
	assert.Equal(t, redeemActionSkipCompleted, resolveRedeemAction(usedCode, nil))

	assert.False(t, unusedCode.IsUsed())
	assert.True(t, unusedCode.CanUse())
	assert.Equal(t, redeemActionRedeem, resolveRedeemAction(unusedCode, nil))
}

func TestExpectedNotificationProviderKeyPrefersOrderInstanceProvider(t *testing.T) {
	t.Parallel()

	registry := payment.NewRegistry()
	registry.Register(paymentFulfillmentTestProvider{
		key:            payment.TypeAlipay,
		supportedTypes: []payment.PaymentType{payment.TypeAlipay},
	})

	assert.Equal(t,
		payment.TypeEasyPay,
		expectedNotificationProviderKey(registry, payment.TypeAlipay, "", payment.TypeEasyPay),
	)
}

func TestExpectedNotificationProviderKeyUsesRegistryMappingForLegacyOrders(t *testing.T) {
	t.Parallel()

	registry := payment.NewRegistry()
	registry.Register(paymentFulfillmentTestProvider{
		key:            payment.TypeEasyPay,
		supportedTypes: []payment.PaymentType{payment.TypeAlipay},
	})

	assert.Equal(t,
		payment.TypeEasyPay,
		expectedNotificationProviderKey(registry, payment.TypeAlipay, "", ""),
	)
}

func TestExpectedNotificationProviderKeyFallsBackToPaymentType(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		payment.TypeWxpay,
		expectedNotificationProviderKey(nil, payment.TypeWxpay, "", ""),
	)
}

func TestExpectedNotificationProviderKeyPrefersOrderSnapshotProviderKey(t *testing.T) {
	t.Parallel()

	registry := payment.NewRegistry()
	registry.Register(paymentFulfillmentTestProvider{
		key:            payment.TypeAlipay,
		supportedTypes: []payment.PaymentType{payment.TypeAlipay},
	})

	assert.Equal(t,
		payment.TypeEasyPay,
		expectedNotificationProviderKey(registry, payment.TypeAlipay, payment.TypeEasyPay, ""),
	)
}

func TestExpectedNotificationProviderKeyForOrderUsesSnapshotProviderKey(t *testing.T) {
	t.Parallel()

	registry := payment.NewRegistry()
	registry.Register(paymentFulfillmentTestProvider{
		key:            payment.TypeAlipay,
		supportedTypes: []payment.PaymentType{payment.TypeAlipay},
	})

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeAlipay,
		ProviderSnapshot: map[string]any{
			"schema_version": 1,
			"provider_key":   payment.TypeEasyPay,
		},
	}

	assert.Equal(t,
		payment.TypeEasyPay,
		expectedNotificationProviderKeyForOrder(registry, order, ""),
	)
}

func TestValidateProviderNotificationMetadataRejectsWxpaySnapshotMismatch(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeWxpay,
		ProviderSnapshot: map[string]any{
			"schema_version":  1,
			"merchant_app_id": "wx-app-expected",
			"merchant_id":     "mch-expected",
			"currency":        "CNY",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeWxpay, map[string]string{
		"appid":       "wx-app-other",
		"mchid":       "mch-expected",
		"currency":    "CNY",
		"trade_state": "SUCCESS",
	})
	assert.ErrorContains(t, err, "wxpay appid mismatch")
}

func TestValidateProviderNotificationMetadataAllowsLegacyOrdersWithoutSnapshotFields(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeWxpay,
		ProviderSnapshot: map[string]any{
			"schema_version":       1,
			"provider_instance_id": "9",
			"provider_key":         payment.TypeWxpay,
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeWxpay, map[string]string{
		"appid":       "wx-app-runtime",
		"mchid":       "mch-runtime",
		"currency":    "CNY",
		"trade_state": "SUCCESS",
	})
	assert.NoError(t, err)
}

func TestParseLegacyPaymentOrderID(t *testing.T) {
	t.Parallel()

	oid, ok := parseLegacyPaymentOrderID("sub2_42", &dbent.NotFoundError{})
	assert.True(t, ok)
	assert.EqualValues(t, 42, oid)

	_, ok = parseLegacyPaymentOrderID("42", &dbent.NotFoundError{})
	assert.False(t, ok)

	_, ok = parseLegacyPaymentOrderID("sub2_42", errors.New("db down"))
	assert.False(t, ok)
}

func TestIsValidProviderAmount(t *testing.T) {
	t.Parallel()

	assert.True(t, isValidProviderAmount(0.01))
	assert.False(t, isValidProviderAmount(0))
	assert.False(t, isValidProviderAmount(-1))
	assert.False(t, isValidProviderAmount(math.NaN()))
	assert.False(t, isValidProviderAmount(math.Inf(1)))
}

func TestValidateProviderNotificationMetadataRejectsAlipaySnapshotMismatch(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeAlipay,
		ProviderSnapshot: map[string]any{
			"schema_version":  2,
			"merchant_app_id": "alipay-app-expected",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeAlipay, map[string]string{
		"app_id": "alipay-app-other",
	})
	assert.ErrorContains(t, err, "alipay app_id mismatch")
}

func TestValidateProviderNotificationMetadataRejectsEasyPaySnapshotMismatch(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeAlipay,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"merchant_id":    "pid-expected",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeEasyPay, map[string]string{
		"pid": "pid-other",
	})
	assert.ErrorContains(t, err, "easypay pid mismatch")
}

func TestValidateProviderNotificationMetadataRejectsAirwallexSnapshotMismatch(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeAirwallex,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"merchant_id":    "acct_expected",
			"currency":       "CNY",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeAirwallex, map[string]string{
		"account_id": "acct_other",
		"currency":   "CNY",
		"status":     "SUCCEEDED",
	})
	assert.ErrorContains(t, err, "airwallex account_id mismatch")

	err = validateProviderNotificationMetadata(order, payment.TypeAirwallex, map[string]string{
		"account_id": "acct_expected",
		"currency":   "USD",
		"status":     "SUCCEEDED",
	})
	assert.ErrorContains(t, err, "airwallex currency mismatch")
}

func TestValidateProviderNotificationMetadataRejectsStripeCurrencyMismatch(t *testing.T) {
	t.Parallel()

	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeStripe,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"currency":       "HKD",
		},
	}

	err := validateProviderNotificationMetadata(order, payment.TypeStripe, map[string]string{
		"currency": "USD",
	})
	assert.ErrorContains(t, err, "stripe currency mismatch")
}

func TestPaymentAmountToleranceForThreeDecimalCurrency(t *testing.T) {
	t.Parallel()

	assert.Equal(t, amountToleranceCNY, paymentAmountToleranceForCurrency("CNY"))
	assert.Equal(t, amountToleranceCNY, paymentAmountToleranceForCurrency("JPY"))
	assert.InDelta(t, 0.0005, paymentAmountToleranceForCurrency("KWD"), 1e-12)
}
