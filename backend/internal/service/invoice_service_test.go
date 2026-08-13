//go:build unit

package service

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestInvoiceApplyRequiresCompletedInvoiceEnabledOrders(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewInvoiceService(client, nil)
	userID := createInvoiceTestUser(t, client, "apply@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	pending := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusPending, 10)
	_, err := svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "apply@example.com", OrderIDs: []int64{pending.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.Error(t, err)
	require.Equal(t, "INVOICE_ORDER_NOT_ELIGIBLE", infraerrors.Reason(err))

	disabled := createInvoiceTestProviderNamed(t, client, "disabled-inv", false)
	completedDisabled := createInvoiceTestOrder(t, client, userID, disabled.ID, OrderStatusCompleted, 20)
	_, err = svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "apply@example.com", OrderIDs: []int64{completedDisabled.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.Error(t, err)
	require.Equal(t, "INVOICE_CHANNEL_DISABLED", infraerrors.Reason(err))
}

func TestInvoiceApplyMergesOrdersAndBlocksOccupancy(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewInvoiceService(client, nil)
	userID := createInvoiceTestUser(t, client, "merge@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	o1 := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 11.5)
	o2 := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 8.5)

	view, err := svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "merge@example.com",
		OrderIDs: []int64{o1.ID, o2.ID, o1.ID},
		Title:    "Acme Ltd", TaxNumber: "91110000", Email: "billing@example.com",
		ContactName: "Ada", ContactPhone: "13800000000",
	})
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusApplied, view.Status)
	require.Equal(t, "merge@example.com", view.UserEmail)
	require.Equal(t, 20.0, view.InvoiceAmount)
	require.Equal(t, payment.DefaultPaymentCurrency, view.Currency)
	require.Equal(t, 2, view.OrderCount)
	require.True(t, view.UnreadByAdmin)
	require.Len(t, view.Orders, 2)

	_, err = svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "merge@example.com",
		OrderIDs: []int64{o2.ID},
		Title:    "Acme Ltd", Email: "billing@example.com",
	})
	require.Error(t, err)
	require.Equal(t, "INVOICE_ORDER_OCCUPIED", infraerrors.Reason(err))
}

func TestInvoiceApplyRejectsOtherUsersOrdersAndTooMany(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewInvoiceService(client, nil)
	owner := createInvoiceTestUser(t, client, "owner@example.com")
	other := createInvoiceTestUser(t, client, "other@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	order := createInvoiceTestOrder(t, client, owner, inst.ID, OrderStatusCompleted, 5)

	_, err := svc.Apply(ctx, ApplyInvoiceInput{
		UserID: other, UserEmail: "other@example.com", OrderIDs: []int64{order.ID},
		Title: "X", Email: "x@example.com",
	})
	require.Error(t, err)
	require.Equal(t, "INVOICE_ORDER_NOT_ELIGIBLE", infraerrors.Reason(err))

	ids := make([]int64, 0, MaxInvoiceOrders+1)
	for i := 0; i < MaxInvoiceOrders+1; i++ {
		ids = append(ids, int64(i+1))
	}
	_, err = svc.Apply(ctx, ApplyInvoiceInput{
		UserID: owner, UserEmail: "owner@example.com", OrderIDs: ids,
		Title: "X", Email: "x@example.com",
	})
	require.Error(t, err)
	require.Equal(t, "INVOICE_TOO_MANY_ORDERS", infraerrors.Reason(err))
}

func TestInvoiceUserCancelReleasesOrdersAdminCannotCancelIssued(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewInvoiceService(client, nil)
	userID := createInvoiceTestUser(t, client, "cancel@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	order := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 30)

	applied, err := svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "cancel@example.com", OrderIDs: []int64{order.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.NoError(t, err)

	cancelled, err := svc.Cancel(ctx, applied.ID, userID, false)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusCancelled, cancelled.Status)
	require.False(t, cancelled.Orders[0].IsActive)

	reapplied, err := svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "cancel@example.com", OrderIDs: []int64{order.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.NoError(t, err)

	mediaID := createInvoiceTestMedia(t, client, userID, reapplied.ID)
	issued, err := svc.Issue(ctx, reapplied.ID, mediaID)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusIssued, issued.Status)
	require.True(t, issued.HasFile)

	_, err = svc.Cancel(ctx, issued.ID, userID, false)
	require.Error(t, err)
	require.Equal(t, "INVOICE_INVALID_STATUS", infraerrors.Reason(err))
	_, err = svc.Cancel(ctx, issued.ID, userID, true)
	require.Error(t, err)
	require.Equal(t, "INVOICE_INVALID_STATUS", infraerrors.Reason(err))
}

func TestInvoiceIssueRequiresPrivateInvoiceMediaAndSendsDetailURLOnly(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewInvoiceService(client, nil)
	svc.frontendURL = "https://app.example.com"
	userID := createInvoiceTestUser(t, client, "issue@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	order := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 12)
	applied, err := svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "issue@example.com", OrderIDs: []int64{order.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.NoError(t, err)

	_, err = svc.Issue(ctx, applied.ID, 0)
	require.Equal(t, "INVOICE_FILE_REQUIRED", infraerrors.Reason(err))

	publicMedia, err := client.MediaAsset.Create().
		SetOwnerUserID(userID).
		SetBizType(MediaBizInvoice).
		SetBizID(strconv.FormatInt(applied.ID, 10)).
		SetStorageKey("media/public-invoice.pdf").
		SetSha256("aa").
		SetMime("application/pdf").
		SetFilename("invoice.pdf").
		SetSize(12).
		SetVisibility(domain.MediaVisibilityPublic).
		SetStatus(MediaStatusReady).
		Save(ctx)
	require.NoError(t, err)
	_, err = svc.Issue(ctx, applied.ID, publicMedia.ID)
	require.Equal(t, "INVOICE_FILE_INVALID", infraerrors.Reason(err))

	mediaID := createInvoiceTestMedia(t, client, userID, applied.ID)
	issued, err := svc.Issue(ctx, applied.ID, mediaID)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusIssued, issued.Status)
	require.Equal(t, "https://app.example.com/invoices/"+strconv.FormatInt(issued.ID, 10), svc.InvoiceDetailURL(ctx, issued.ID))
	require.NotContains(t, svc.InvoiceDetailURL(ctx, issued.ID), "media")
	require.NotContains(t, svc.InvoiceDetailURL(ctx, issued.ID), "download")

	relative := NewInvoiceService(client, nil)
	require.Empty(t, relative.InvoiceDetailURL(ctx, issued.ID))
}

func TestInvoiceIssueRejectsWhenLinkedOrderNoLongerCompleted(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewInvoiceService(client, nil)
	userID := createInvoiceTestUser(t, client, "race@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	order := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 18)
	applied, err := svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "", OrderIDs: []int64{order.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, "user@example.com", applied.UserEmail)

	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetStatus(OrderStatusRefunded).Save(ctx)
	require.NoError(t, err)
	_, err = svc.Issue(ctx, applied.ID, createInvoiceTestMedia(t, client, userID, applied.ID))
	require.Equal(t, "INVOICE_ORDER_NOT_ELIGIBLE", infraerrors.Reason(err))
}

func TestInvoiceRefundInterlockIssuedBlocksAppliedCancels(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewInvoiceService(client, nil)
	userID := createInvoiceTestUser(t, client, "refund@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	appliedOrder := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 9)
	issuedOrder := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 15)

	applied, err := svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "refund@example.com", OrderIDs: []int64{appliedOrder.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.NoError(t, err)
	issuedApp, err := svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "refund@example.com", OrderIDs: []int64{issuedOrder.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.NoError(t, err)
	_, err = svc.Issue(ctx, issuedApp.ID, createInvoiceTestMedia(t, client, userID, issuedApp.ID))
	require.NoError(t, err)

	issued, err := svc.HasActiveIssuedInvoiceForOrder(ctx, issuedOrder.ID)
	require.NoError(t, err)
	require.True(t, issued)
	appliedIssued, err := svc.HasActiveIssuedInvoiceForOrder(ctx, appliedOrder.ID)
	require.NoError(t, err)
	require.False(t, appliedIssued)

	require.NoError(t, svc.CancelAppliedInvoicesForOrder(ctx, appliedOrder.ID))
	reloaded, err := svc.Get(ctx, applied.ID, userID, true)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusCancelled, reloaded.Status)
	require.False(t, reloaded.Orders[0].IsActive)
}

func TestInvoiceAppliedCancelsOnRefundSuccess(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	invoiceSvc := NewInvoiceService(client, nil)
	userID := createInvoiceTestUser(t, client, "refund-cancel@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	order := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 22)
	applied, err := invoiceSvc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "refund-cancel@example.com", OrderIDs: []int64{order.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.NoError(t, err)

	pay := &PaymentService{entClient: client, invoiceGuard: invoiceSvc}
	_, err = pay.markRefundOk(ctx, &RefundPlan{
		OrderID:      order.ID,
		Order:        order,
		RefundAmount: 22,
		Reason:       "user refund",
	})
	require.NoError(t, err)
	reloaded, err := invoiceSvc.Get(ctx, applied.ID, userID, true)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusCancelled, reloaded.Status)
	require.False(t, reloaded.Orders[0].IsActive)
}

func TestInvoiceIssuedBlocksRefundUnlessForced(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	invoiceSvc := NewInvoiceService(client, nil)
	userID := createInvoiceTestUser(t, client, "block-refund@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	_, err := client.PaymentProviderInstance.UpdateOneID(inst.ID).
		SetRefundEnabled(true).
		SetAllowUserRefund(true).
		Save(ctx)
	require.NoError(t, err)
	order := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 40)
	applied, err := invoiceSvc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "block-refund@example.com", OrderIDs: []int64{order.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.NoError(t, err)
	_, err = invoiceSvc.Issue(ctx, applied.ID, createInvoiceTestMedia(t, client, userID, applied.ID))
	require.NoError(t, err)

	pay := &PaymentService{entClient: client, invoiceGuard: invoiceSvc}
	_, err = pay.validateRefundRequest(ctx, order.ID, userID)
	require.Equal(t, "INVOICE_ISSUED", infraerrors.Reason(err))
	_, _, err = pay.PrepareRefund(ctx, order.ID, 40, "test", false, false)
	require.Equal(t, "INVOICE_ISSUED", infraerrors.Reason(err))
	plan, result, err := pay.PrepareRefund(ctx, order.ID, 40, "force", true, false)
	require.NoError(t, err)
	require.Nil(t, result)
	require.NotNil(t, plan)

	_, err = pay.markRefundOk(ctx, &RefundPlan{
		OrderID:      order.ID,
		Order:        order,
		RefundAmount: 40,
		Reason:       "force",
		Force:        true,
	})
	require.NoError(t, err)
	reloaded, err := invoiceSvc.Get(ctx, applied.ID, userID, true)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusCancelled, reloaded.Status)
	require.False(t, reloaded.Orders[0].IsActive)
	issued, err := invoiceSvc.HasActiveIssuedInvoiceForOrder(ctx, order.ID)
	require.NoError(t, err)
	require.False(t, issued)
}

func TestInvoiceGetHidesOtherUsersAndMarksAdminRead(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewInvoiceService(client, nil)
	userID := createInvoiceTestUser(t, client, "read@example.com")
	other := createInvoiceTestUser(t, client, "nosy@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	order := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 4)
	applied, err := svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "read@example.com", OrderIDs: []int64{order.ID},
		Title: "Acme", Email: "billing@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, 1, mustInvoiceUnread(t, svc))

	_, err = svc.Get(ctx, applied.ID, other, false)
	require.Equal(t, "INVOICE_FORBIDDEN", infraerrors.Reason(err))

	peek, err := svc.Get(ctx, applied.ID, other, true)
	require.NoError(t, err)
	require.True(t, peek.UnreadByAdmin)
	require.Equal(t, 1, mustInvoiceUnread(t, svc))

	view, err := svc.GetAndMarkRead(ctx, applied.ID, other, true)
	require.NoError(t, err)
	require.False(t, view.UnreadByAdmin)
	require.Equal(t, 0, mustInvoiceUnread(t, svc))
}

func TestInvoiceApplyRejectsMixedCurrency(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewInvoiceService(client, nil)
	userID := createInvoiceTestUser(t, client, "fx@example.com")
	inst := createInvoiceTestProvider(t, client, true)
	cny := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 10)
	usd := createInvoiceTestOrder(t, client, userID, inst.ID, OrderStatusCompleted, 12)
	_, err := client.PaymentOrder.UpdateOneID(usd.ID).
		SetProviderSnapshot(map[string]any{"currency": "USD"}).
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.Apply(ctx, ApplyInvoiceInput{
		UserID: userID, UserEmail: "fx@example.com",
		OrderIDs: []int64{cny.ID, usd.ID},
		Title:    "Acme", Email: "billing@example.com",
	})
	require.Error(t, err)
	require.Equal(t, "INVOICE_MIXED_CURRENCY", infraerrors.Reason(err))
}

func mustInvoiceUnread(t *testing.T, svc *InvoiceService) int {
	t.Helper()
	n, err := svc.CountUnreadApplied(context.Background())
	require.NoError(t, err)
	return n
}

func createInvoiceTestUser(t *testing.T, client *dbent.Client, email string) int64 {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetUsername(strings.ReplaceAll(email, "@", "-")).
		Save(context.Background())
	require.NoError(t, err)
	return user.ID
}

func createInvoiceTestProvider(t *testing.T, client *dbent.Client, invoiceEnabled bool) *dbent.PaymentProviderInstance {
	t.Helper()
	return createInvoiceTestProviderNamed(t, client, "invoice-alipay", invoiceEnabled)
}

func createInvoiceTestProviderNamed(t *testing.T, client *dbent.Client, name string, invoiceEnabled bool) *dbent.PaymentProviderInstance {
	t.Helper()
	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName(name).
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetInvoiceEnabled(invoiceEnabled).
		Save(context.Background())
	require.NoError(t, err)
	return inst
}

func createInvoiceTestOrder(t *testing.T, client *dbent.Client, userID, providerID int64, status string, payAmount float64) *dbent.PaymentOrder {
	t.Helper()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail("user@example.com").
		SetUserName("user").
		SetAmount(payAmount).
		SetPayAmount(payAmount).
		SetFeeRate(0).
		SetRechargeCode("INV-" + suffix).
		SetOutTradeNo("sub2_inv_" + suffix).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-" + suffix).
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(status).
		SetProviderInstanceID(strconv.FormatInt(providerID, 10)).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(context.Background())
	require.NoError(t, err)
	return order
}

func createInvoiceTestMedia(t *testing.T, client *dbent.Client, ownerUserID, invoiceID int64) int64 {
	t.Helper()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	asset, err := client.MediaAsset.Create().
		SetOwnerUserID(ownerUserID).
		SetBizType(MediaBizInvoice).
		SetBizID(strconv.FormatInt(invoiceID, 10)).
		SetStorageKey("media/invoice-" + suffix + ".pdf").
		SetSha256("deadbeef").
		SetMime("application/pdf").
		SetFilename("invoice.pdf").
		SetSize(128).
		SetVisibility(MediaVisibilityPrivate).
		SetStatus(MediaStatusReady).
		Save(context.Background())
	require.NoError(t, err)
	return asset.ID
}
