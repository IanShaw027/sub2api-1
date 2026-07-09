package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	entdialect "entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type invoiceTestMediaService struct {
	nextAssetID int64
	uploads     []UploadMediaInput
	uploadHook  func(context.Context, UploadMediaInput) error
	deletedIDs  []int64
}

func (s *invoiceTestMediaService) Upload(ctx context.Context, input UploadMediaInput) (*MediaAsset, error) {
	s.uploads = append(s.uploads, input)
	if s.uploadHook != nil {
		if err := s.uploadHook(ctx, input); err != nil {
			return nil, err
		}
	}
	s.nextAssetID++
	now := time.Now()
	return &MediaAsset{
		ID:               s.nextAssetID,
		BizType:          input.BizType,
		BizID:            input.BizID,
		Visibility:       input.Visibility,
		MIMEType:         input.ContentType,
		SizeBytes:        int64(len(input.File)),
		OwnerUserID:      input.OwnerUserID,
		Status:           MediaStatusActive,
		OriginalFileName: input.FileName,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func (s *invoiceTestMediaService) CreateDownloadURLForUser(_ context.Context, requesterUserID, id int64) (*MediaDownloadURL, error) {
	return &MediaDownloadURL{
		URL:       "https://files.example.com/user/" + time.Now().UTC().Format("20060102150405"),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}, nil
}

func (s *invoiceTestMediaService) CreateDownloadURLForAdmin(_ context.Context, id int64) (*MediaDownloadURL, error) {
	return &MediaDownloadURL{
		URL:       "https://files.example.com/admin/" + time.Now().UTC().Format("20060102150405"),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}, nil
}

func (s *invoiceTestMediaService) DeleteForAdmin(_ context.Context, id int64) error {
	s.deletedIDs = append(s.deletedIDs, id)
	return nil
}

func newInvoiceTestService(t *testing.T) (*dbent.Client, *invoiceTestMediaService, *InvoiceService) {
	t.Helper()
	client := newPaymentConfigServiceTestClient(t)
	media := &invoiceTestMediaService{}
	svc := NewInvoiceService(client, &PaymentService{entClient: client}, media)
	return client, media, svc
}

type invoiceTxTracker struct {
	active atomic.Int32
}

type trackingDriver struct {
	entdialect.Driver
	tracker *invoiceTxTracker
}

type trackingTx struct {
	entdialect.Tx
	tracker *invoiceTxTracker
	once    sync.Once
}

type invoiceQueryCounter struct {
	count atomic.Int32
}

func (c *invoiceQueryCounter) Reset() {
	c.count.Store(0)
}

func (c *invoiceQueryCounter) Count() int {
	return int(c.count.Load())
}

type queryCountingDriver struct {
	entdialect.Driver
	counter *invoiceQueryCounter
}

type queryCountingTx struct {
	entdialect.Tx
	counter *invoiceQueryCounter
}

func (d *queryCountingDriver) Query(ctx context.Context, query string, args, v any) error {
	d.counter.count.Add(1)
	return d.Driver.Query(ctx, query, args, v)
}

func (d *queryCountingDriver) Exec(ctx context.Context, query string, args, v any) error {
	return d.Driver.Exec(ctx, query, args, v)
}

func (d *queryCountingDriver) Tx(ctx context.Context) (entdialect.Tx, error) {
	tx, err := d.Driver.Tx(ctx)
	if err != nil {
		return nil, err
	}
	return &queryCountingTx{Tx: tx, counter: d.counter}, nil
}

func (tx *queryCountingTx) Query(ctx context.Context, query string, args, v any) error {
	tx.counter.count.Add(1)
	return tx.Tx.Query(ctx, query, args, v)
}

func (tx *queryCountingTx) Exec(ctx context.Context, query string, args, v any) error {
	return tx.Tx.Exec(ctx, query, args, v)
}

func (d *trackingDriver) Tx(ctx context.Context) (entdialect.Tx, error) {
	tx, err := d.Driver.Tx(ctx)
	if err != nil {
		return nil, err
	}
	d.tracker.active.Add(1)
	return &trackingTx{Tx: tx, tracker: d.tracker}, nil
}

func (tx *trackingTx) Commit() error {
	err := tx.Tx.Commit()
	tx.once.Do(func() {
		tx.tracker.active.Add(-1)
	})
	return err
}

func (tx *trackingTx) Rollback() error {
	err := tx.Tx.Rollback()
	tx.once.Do(func() {
		tx.tracker.active.Add(-1)
	})
	return err
}

func newTrackedInvoiceTestService(t *testing.T) (*dbent.Client, *invoiceTestMediaService, *InvoiceService, *invoiceTxTracker) {
	t.Helper()

	dbName := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()),
	)
	db, err := sql.Open("sqlite", dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	tracker := &invoiceTxTracker{}
	drv := &trackingDriver{
		Driver:  entsql.OpenDB(entdialect.SQLite, db),
		tracker: tracker,
	}
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	media := &invoiceTestMediaService{}
	svc := NewInvoiceService(client, &PaymentService{entClient: client}, media)
	return client, media, svc, tracker
}

func newQueryCountedInvoiceTestService(t *testing.T) (*dbent.Client, *invoiceTestMediaService, *InvoiceService, *invoiceQueryCounter) {
	t.Helper()

	dbName := fmt.Sprintf(
		"file:%s?mode=memory&cache=shared",
		strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()),
	)
	db, err := sql.Open("sqlite", dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	counter := &invoiceQueryCounter{}
	drv := &queryCountingDriver{
		Driver:  entsql.OpenDB(entdialect.SQLite, db),
		counter: counter,
	}
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	media := &invoiceTestMediaService{}
	svc := NewInvoiceService(client, &PaymentService{entClient: client}, media)
	return client, media, svc, counter
}

// seedInvoiceTestOrder 复用旧版本签名：返回一张 user / order / providerInstance。
func seedInvoiceTestOrder(t *testing.T, ctx context.Context, client *dbent.Client, invoiceEnabled bool, orderStatus string) (*User, *dbent.PaymentOrder, *dbent.PaymentProviderInstance) {
	t.Helper()

	user, err := client.User.Create().
		SetEmail("invoice-user@example.com").
		SetPasswordHash("hash").
		SetUsername("invoice-user").
		Save(ctx)
	require.NoError(t, err)

	providerInstance, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeWxpay).
		SetName("wechat-invoice-provider").
		SetConfig("{}").
		SetSupportedTypes("wxpay").
		SetEnabled(true).
		SetInvoiceEnabled(invoiceEnabled).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("INV-ORDER").
		SetOutTradeNo("sub2_invoice_order").
		SetPaymentType(payment.TypeWxpay).
		SetPaymentTradeNo("trade-invoice-order").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(orderStatus).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetCompletedAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetProviderInstanceID("1").
		SetProviderKey(payment.TypeWxpay).
		Save(ctx)
	require.NoError(t, err)

	return &User{ID: user.ID, Email: user.Email, Username: user.Username}, order, providerInstance
}

func TestInvoiceServiceCreate_SingleOrder(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)

	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs:  []int64{order.ID},
		Title:     "ACME 有限公司",
		TaxNumber: "91110000MA00000001",
		Email:     "billing@acme.com",
	})
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusApplied, inv.Status)
	require.Equal(t, 1, inv.OrderCount)
	require.InDelta(t, order.PayAmount, inv.InvoiceAmount, 0.001)
	require.Len(t, inv.Orders, 1)
	require.Equal(t, order.ID, inv.Orders[0].OrderID)
	require.Equal(t, order.OutTradeNo, inv.Orders[0].OutTradeNo)
}

func TestFormatInvoiceAmountDisplay(t *testing.T) {
	t.Run("single known currency uses currency code prefix", func(t *testing.T) {
		require.Equal(t, "USD 318.00", formatInvoiceAmountDisplay(318, []string{"usd", "USD"}))
	})

	t.Run("mixed currencies fall back to raw amount", func(t *testing.T) {
		require.Equal(t, "318.00", formatInvoiceAmountDisplay(318, []string{"USD", "EUR"}))
	})

	t.Run("unknown currency falls back to raw amount", func(t *testing.T) {
		require.Equal(t, "318.00", formatInvoiceAmountDisplay(318, []string{""}))
	})
}

func TestInvoiceServiceCreate_MultipleOrders(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order1, providerInst := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	order2, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(50).SetPayAmount(50).SetFeeRate(0).
		SetRechargeCode("INV-ORDER-2").
		SetOutTradeNo("sub2_invoice_order_2").
		SetPaymentType(payment.TypeWxpay).
		SetPaymentTradeNo("trade-invoice-order-2").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).SetCompletedAt(time.Now()).
		SetClientIP("127.0.0.1").SetSrcHost("example.com").
		SetProviderInstanceID(fmt.Sprintf("%d", providerInst.ID)).
		SetProviderKey(payment.TypeWxpay).
		Save(ctx)
	require.NoError(t, err)

	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs:  []int64{order1.ID, order2.ID},
		Title:     "ACME",
		TaxNumber: "TX",
		Email:     "x@a.com",
	})
	require.NoError(t, err)
	require.Equal(t, 2, inv.OrderCount)
	require.InDelta(t, 150.0, inv.InvoiceAmount, 0.001)
}

func TestInvoiceServiceCreate_AtomicRollbackOnIneligible(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, completedOrder, providerInst := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	pendingOrder, err := client.PaymentOrder.Create().
		SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
		SetAmount(50).SetPayAmount(50).SetFeeRate(0).
		SetRechargeCode("INV-ORDER-PEND").
		SetOutTradeNo("sub2_invoice_order_pending").
		SetPaymentType(payment.TypeWxpay).SetPaymentTradeNo("trade-x").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(OrderStatusPending).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").SetSrcHost("example.com").
		SetProviderInstanceID(fmt.Sprintf("%d", providerInst.ID)).
		SetProviderKey(payment.TypeWxpay).
		Save(ctx)
	require.NoError(t, err)

	_, err = svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{completedOrder.ID, pendingOrder.ID},
		Title:    "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.Error(t, err)
	require.Equal(t, "INVOICE_ORDER_INELIGIBLE", infraerrors.Reason(err))

	count, err := client.Invoice.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count, "no invoice should be created on rollback")

	linkCount, err := client.InvoiceOrder.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, linkCount, "no invoice_order should be created on rollback")
}

func TestInvoiceServiceCreate_RejectsOrderInActiveInvoice(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)

	// First Create succeeds.
	_, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	// Second Create on same order must fail with INVOICE_ALREADY_INVOICED.
	_, err = svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T2", TaxNumber: "TX", Email: "x@a.com",
	})
	require.Error(t, err)
	require.Equal(t, "INVOICE_ALREADY_INVOICED", infraerrors.Reason(err))

	// Verify only one invoice exists (no leak from the failed attempt).
	count, err := client.Invoice.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestInvoiceServiceCreate_MapsInvoiceOrderConstraintConflict(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)

	trigger := fmt.Sprintf(`
		CREATE TRIGGER invoice_order_active_conflict
		BEFORE INSERT ON invoice_orders
		WHEN NEW.order_id = %d AND NEW.is_active = 1
		BEGIN
			SELECT RAISE(ABORT, 'constraint failed: invoice_orders.order_id');
		END;
	`, order.ID)
	_, err := client.ExecContext(ctx, trigger)
	require.NoError(t, err)

	_, err = svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.Error(t, err)
	require.True(t, infraerrors.IsConflict(err), "expected conflict error, got %v", err)
	require.Equal(t, "INVOICE_ORDER_ALREADY_ACTIVE", infraerrors.Reason(err))
}

func TestInvoiceServiceCancel_ReleasesOrders(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)

	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	cancelled, err := svc.Cancel(ctx, inv.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusCancelled, cancelled.Status)
	require.NotNil(t, cancelled.CancelledAt)

	// 取消后订单应该可以再加入新发票
	inv2, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T2", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)
	require.NotEqual(t, inv.ID, inv2.ID)
}

func TestInvoiceServiceCancel_RejectsIssued(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	_, err = svc.UploadFile(ctx, inv.ID, InvoiceFileUploadInput{
		FileName: "invoice.pdf", ContentType: "application/pdf", File: []byte("PDF"),
	})
	require.NoError(t, err)

	_, err = svc.Cancel(ctx, inv.ID, user.ID)
	require.Error(t, err)
	require.Equal(t, "INVOICE_CANNOT_CANCEL", infraerrors.Reason(err))
	_ = client
}

func TestInvoiceServiceUploadFile_RejectsUnsupportedContentType(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	_, err = svc.UploadFile(ctx, inv.ID, InvoiceFileUploadInput{
		FileName:    "invoice.html",
		ContentType: "text/html",
		File:        []byte("<html></html>"),
	})
	require.Error(t, err)
	require.Equal(t, "INVOICE_FILE_CONTENT_TYPE_INVALID", infraerrors.Reason(err))
}

func TestInvoiceServiceUploadFile_SanitizesStoredFileName(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	issued, err := svc.UploadFile(ctx, inv.ID, InvoiceFileUploadInput{
		FileName:    "invoice\"\r\n2026.pdf",
		ContentType: "application/pdf",
		File:        []byte("PDF"),
	})
	require.NoError(t, err)
	require.Equal(t, "invoice2026.pdf", issued.FileName)
}

func TestInvoiceServiceUploadFile_RechecksStateAfterUpload(t *testing.T) {
	ctx := context.Background()
	client, media, svc, tracker := newTrackedInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	var activeTxDuringUpload int32 = -1
	media.uploadHook = func(ctx context.Context, _ UploadMediaInput) error {
		activeTxDuringUpload = tracker.active.Load()
		return nil
	}

	issued, err := svc.UploadFile(ctx, inv.ID, InvoiceFileUploadInput{
		FileName:    "invoice.pdf",
		ContentType: "application/pdf",
		File:        []byte("PDF"),
	})
	require.NoError(t, err)
	require.Equal(t, int32(0), activeTxDuringUpload, "upload must happen before opening the invoice transaction")
	require.Equal(t, InvoiceStatusIssued, issued.Status)
}

func TestInvoiceServiceUploadFile_DeletesUploadedAssetWhenStateChangesAfterUpload(t *testing.T) {
	ctx := context.Background()
	client, media, svc, _ := newTrackedInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	media.uploadHook = func(ctx context.Context, _ UploadMediaInput) error {
		_, err := client.Invoice.UpdateOneID(inv.ID).SetStatus(InvoiceStatusCancelled).Save(ctx)
		return err
	}

	issued, err := svc.UploadFile(ctx, inv.ID, InvoiceFileUploadInput{
		FileName:    "invoice.pdf",
		ContentType: "application/pdf",
		File:        []byte("PDF"),
	})
	require.Nil(t, issued)
	require.Error(t, err)
	require.Equal(t, "INVOICE_CANNOT_UPLOAD", infraerrors.Reason(err))
	require.Equal(t, []int64{1}, media.deletedIDs)
}

func TestInvoiceServiceList_FiltersByUserAndStatus(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	uid := user.ID
	items, total, err := svc.List(ctx, InvoiceListParams{UserID: &uid, Status: InvoiceStatusApplied})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inv.ID, items[0].ID)
	require.Nil(t, items[0].Orders, "list view must not eager-load orders")

	// admin 视图（UserID 为 nil）
	itemsAll, totalAll, err := svc.List(ctx, InvoiceListParams{})
	require.NoError(t, err)
	require.Equal(t, 1, totalAll)
	require.Len(t, itemsAll, 1)

	// 不匹配的 user_id
	otherID := user.ID + 999
	_, totalEmpty, err := svc.List(ctx, InvoiceListParams{UserID: &otherID})
	require.NoError(t, err)
	require.Equal(t, 0, totalEmpty)
}

func TestInvoiceServiceList_KeywordMatchOnOutTradeNoDoesNotRequirePrefetchQuery(t *testing.T) {
	ctx := context.Background()
	client, _, svc, counter := newQueryCountedInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	counter.Reset()
	items, total, err := svc.List(ctx, InvoiceListParams{Keyword: order.OutTradeNo})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, inv.ID, items[0].ID)
	require.Equal(t, 2, counter.Count(), "keyword list should stay on count+page queries without prefetching invoice IDs")
}

func TestInvoiceServiceGetActiveLinksByOrderIDs(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	links, err := svc.GetActiveLinksByOrderIDs(ctx, []int64{order.ID})
	require.NoError(t, err)
	require.Contains(t, links, order.ID)
	require.Equal(t, inv.ID, links[order.ID].InvoiceID)
	require.Equal(t, InvoiceStatusApplied, links[order.ID].InvoiceStatus)

	// 取消后链接消失
	_, err = svc.Cancel(ctx, inv.ID, user.ID)
	require.NoError(t, err)
	links2, err := svc.GetActiveLinksByOrderIDs(ctx, []int64{order.ID})
	require.NoError(t, err)
	require.NotContains(t, links2, order.ID)
	_ = client
}

func TestInvoiceServiceGetForUser_RejectsForeign(t *testing.T) {
	ctx := context.Background()
	client, _, svc := newInvoiceTestService(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	inv, err := svc.Create(ctx, user.ID, CreateInvoiceRequest{
		OrderIDs: []int64{order.ID}, Title: "T", TaxNumber: "TX", Email: "x@a.com",
	})
	require.NoError(t, err)

	_, err = svc.GetForUser(ctx, inv.ID, user.ID+1)
	require.Error(t, err)
	require.Equal(t, "FORBIDDEN", infraerrors.Reason(err))

	got, err := svc.GetForUser(ctx, inv.ID, user.ID)
	require.NoError(t, err)
	require.Len(t, got.Orders, 1)
	_ = client
}
