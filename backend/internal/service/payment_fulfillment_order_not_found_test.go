//go:build unit

package service

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// newOrderNotFoundTestClient wires an in-memory sqlite-backed ent.Client so
// tests can exercise HandlePaymentNotification's real DB lookup path without
// standing up a service stack.
func newOrderNotFoundTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", "file:payment_order_not_found?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// TestHandlePaymentNotification_UnknownOrder_ReturnsSentinel exercises the
// happy-path of the webhook 404 fix: when the notification references an
// out_trade_no that does not exist in our DB, HandlePaymentNotification must
// return an error that errors.Is(err, ErrOrderNotFound) recognizes. The
// webhook handler relies on that contract to ack with a 2xx so the provider
// stops retrying.
func TestHandlePaymentNotification_UnknownOrder_ReturnsSentinel(t *testing.T) {
	ctx := context.Background()
	client := newOrderNotFoundTestClient(t)

	svc := &PaymentService{
		entClient:       client,
		providersLoaded: true,
	}

	notification := &payment.PaymentNotification{
		OrderID: "sub2_does_not_exist_12345",
		TradeNo: "stripe_evt_test_xyz",
		Status:  payment.NotificationStatusSuccess,
		Amount:  1000,
	}

	err := svc.HandlePaymentNotification(ctx, notification, payment.TypeStripe)
	require.Error(t, err, "unknown out_trade_no should surface an error")
	require.ErrorIs(t, err, ErrOrderNotFound,
		"webhook handler relies on errors.Is(err, ErrOrderNotFound) to downgrade to 200")

	// Sanity: the wrapped error message should still include the out_trade_no
	// for operator diagnostics.
	require.Contains(t, err.Error(), notification.OrderID)
}

// TestHandlePaymentNotification_NonSuccessStatus_Skips documents the
// short-circuit that precedes the DB lookup: when the notification is not a
// success event (e.g. Stripe non-payment events that reach us via the webhook
// route), we return nil without touching the DB and the handler responds 200.
func TestHandlePaymentNotification_NonSuccessStatus_Skips(t *testing.T) {
	ctx := context.Background()
	client := newOrderNotFoundTestClient(t)

	svc := &PaymentService{
		entClient:       client,
		providersLoaded: true,
	}

	notification := &payment.PaymentNotification{
		OrderID: "sub2_does_not_exist_12345",
		Status:  "failed", // any value other than NotificationStatusSuccess
	}

	err := svc.HandlePaymentNotification(ctx, notification, payment.TypeStripe)
	require.NoError(t, err,
		"non-success notifications must short-circuit before the DB lookup")
}

// TestErrOrderNotFound_DistinctFromOtherErrors guards against an accidental
// collapse where a generic wrapped error would start matching ErrOrderNotFound
// (which would silently mask real DB failures).
func TestErrOrderNotFound_DistinctFromOtherErrors(t *testing.T) {
	genericErr := errors.New("some other failure")
	require.False(t, errors.Is(genericErr, ErrOrderNotFound))
	require.False(t, errors.Is(ErrOrderNotFound, genericErr))

	wrappedLookupErr := errors.New("lookup order failed for out_trade_no sub2_42: connection refused")
	require.False(t, errors.Is(wrappedLookupErr, ErrOrderNotFound),
		"DB connection failures must not masquerade as order-not-found")
}

func TestConfirmPaymentReturnsErrorWhenOrderGetFails(t *testing.T) {
	ctx := context.Background()
	client := newOrderNotFoundTestClient(t)
	svc := &PaymentService{entClient: client}

	err := svc.confirmPayment(ctx, 999999, "trade-no", 10, payment.TypeStripe, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrOrderNotFound)
	require.Contains(t, err.Error(), "sub2_999999")
}

func TestHandlePaymentNotification_LegacyFallbackUnknownOrder_ReturnsSentinel(t *testing.T) {
	ctx := context.Background()
	client := newOrderNotFoundTestClient(t)
	svc := &PaymentService{
		entClient:       client,
		providersLoaded: true,
	}

	notification := &payment.PaymentNotification{
		OrderID: "sub2_999999",
		TradeNo: "legacy-trade-missing",
		Status:  payment.NotificationStatusSuccess,
		Amount:  100,
	}

	err := svc.HandlePaymentNotification(ctx, notification, payment.TypeStripe)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrOrderNotFound)
	require.Contains(t, err.Error(), notification.OrderID)
}

func TestConfirmPaymentRecoversPaidAndRejectsFreshRechargingLease(t *testing.T) {
	ctx := context.Background()
	client := newOrderNotFoundTestClient(t)
	paidAt := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	rechargingPaidAt := time.Now().Add(-90 * time.Minute).UTC().Truncate(time.Second)

	user, err := client.User.Create().
		SetEmail("paid-idempotent@example.com").
		SetPasswordHash("hash").
		SetUsername("paid-idempotent").
		Save(ctx)
	require.NoError(t, err)

	paidOrder, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("IDEMPOTENT-PAID").
		SetOutTradeNo("sub2_idempotent_paid").
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("trade-existing").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusPaid).
		SetPaidAt(paidAt).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)
	rechargingOrder, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("IDEMPOTENT-RECHARGING").
		SetOutTradeNo("sub2_idempotent_recharging").
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("trade-existing").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusRecharging).
		SetPaidAt(rechargingPaidAt).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		Save(ctx)
	require.NoError(t, err)

	svc := &PaymentService{
		entClient: client,
		registry:  payment.NewRegistry(),
	}

	err = svc.confirmPayment(ctx, paidOrder.ID, "trade-existing", paidOrder.PayAmount, payment.TypeStripe, nil)
	require.NoError(t, err)
	err = svc.confirmPayment(ctx, paidOrder.ID, "trade-existing", paidOrder.PayAmount, payment.TypeStripe, nil)
	require.NoError(t, err)

	err = svc.confirmPayment(ctx, rechargingOrder.ID, "trade-existing", rechargingOrder.PayAmount, payment.TypeStripe, nil)
	require.Error(t, err)
	require.Equal(t, "CONFLICT", infraerrors.Reason(err))
	err = svc.confirmPayment(ctx, rechargingOrder.ID, "trade-existing", rechargingOrder.PayAmount, payment.TypeStripe, nil)
	require.Error(t, err)
	require.Equal(t, "CONFLICT", infraerrors.Reason(err))

	reloadedPaid, err := client.PaymentOrder.Get(ctx, paidOrder.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusCompleted, reloadedPaid.Status)
	require.Equal(t, "trade-existing", reloadedPaid.PaymentTradeNo)
	require.NotNil(t, reloadedPaid.PaidAt)
	require.Equal(t, paidAt, reloadedPaid.PaidAt.UTC().Truncate(time.Second))
	require.NotNil(t, reloadedPaid.CompletedAt)

	reloadedRecharging, err := client.PaymentOrder.Get(ctx, rechargingOrder.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRecharging, reloadedRecharging.Status)
	require.Equal(t, "trade-existing", reloadedRecharging.PaymentTradeNo)
	require.NotNil(t, reloadedRecharging.PaidAt)
	require.Equal(t, rechargingPaidAt, reloadedRecharging.PaidAt.UTC().Truncate(time.Second))
	require.Nil(t, reloadedRecharging.CompletedAt)

	reloadedUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, paidOrder.Amount, reloadedUser.Balance)

	paidSuccessCount, err := client.PaymentAuditLog.Query().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(paidOrder.ID, 10)),
			paymentauditlog.ActionEQ("RECHARGE_SUCCESS"),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, paidSuccessCount, "repeated PAID webhook must fulfill exactly once")

	rechargingLogCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(rechargingOrder.ID, 10))).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, rechargingLogCount, "fresh lease conflict must not emit fulfillment side effects")
}
