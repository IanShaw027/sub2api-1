package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type invoiceTestMediaService struct {
	nextAssetID int64
	uploads     []UploadMediaInput
}

func (s *invoiceTestMediaService) Upload(_ context.Context, input UploadMediaInput) (*MediaAsset, error) {
	s.uploads = append(s.uploads, input)
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

func TestInvoiceServiceApplyCancelAndReapply(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	mediaSvc := &invoiceTestMediaService{}
	invoiceSvc := NewInvoiceService(client, &PaymentService{entClient: client}, mediaSvc)

	created, err := invoiceSvc.Apply(ctx, order.ID, user.ID, ApplyInvoiceRequest{
		Title:       "测试科技有限公司",
		TaxNumber:   "91420100MA00000001",
		Email:       "finance@example.com",
		ContactName: "张三",
		ContactPhone:"13800000000",
	})
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusApplied, created.Status)
	require.Equal(t, order.PayAmount, created.InvoiceAmount)

	reloadedOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusApplied, reloadedOrder.InvoiceStatus)

	cancelled, err := invoiceSvc.CancelByOrderForUser(ctx, order.ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusCancelled, cancelled.Status)

	reloadedOrder, err = client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusCancelled, reloadedOrder.InvoiceStatus)

	reapplied, err := invoiceSvc.Apply(ctx, order.ID, user.ID, ApplyInvoiceRequest{
		Title:       "测试科技有限公司",
		TaxNumber:   "91420100MA00000001",
		Email:       "finance@example.com",
		ContactName: "李四",
		ContactPhone:"13900000000",
	})
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusApplied, reapplied.Status)
	require.Equal(t, "李四", reapplied.ContactName)
}

func TestInvoiceServiceApplyRejectsProviderWithoutInvoiceFlag(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, false, OrderStatusCompleted)
	invoiceSvc := NewInvoiceService(client, &PaymentService{entClient: client}, &invoiceTestMediaService{})

	_, err := invoiceSvc.Apply(ctx, order.ID, user.ID, ApplyInvoiceRequest{
		Title:     "测试科技有限公司",
		TaxNumber: "91420100MA00000001",
		Email:     "finance@example.com",
	})
	require.Error(t, err)
	require.Equal(t, 403, infraerrors.Code(err))
}

func TestInvoiceServiceApplyRejectsRefundedOrder(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusRefunded)
	invoiceSvc := NewInvoiceService(client, &PaymentService{entClient: client}, &invoiceTestMediaService{})

	_, err := invoiceSvc.Apply(ctx, order.ID, user.ID, ApplyInvoiceRequest{
		Title:     "测试科技有限公司",
		TaxNumber: "91420100MA00000001",
		Email:     "finance@example.com",
	})
	require.Error(t, err)
	require.Equal(t, 400, infraerrors.Code(err))
}

func TestInvoiceServiceUploadMarksApplicationIssuedAndSupportsDownload(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	user, order, _ := seedInvoiceTestOrder(t, ctx, client, true, OrderStatusCompleted)
	mediaSvc := &invoiceTestMediaService{}
	invoiceSvc := NewInvoiceService(client, &PaymentService{entClient: client}, mediaSvc)

	applied, err := invoiceSvc.Apply(ctx, order.ID, user.ID, ApplyInvoiceRequest{
		Title:     "测试科技有限公司",
		TaxNumber: "91420100MA00000001",
		Email:     "finance@example.com",
	})
	require.NoError(t, err)

	issued, err := invoiceSvc.UploadFile(ctx, applied.ID, InvoiceFileUploadInput{
		FileName:    "invoice.pdf",
		ContentType: "application/pdf",
		File:        []byte("pdf-binary"),
	})
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusIssued, issued.Status)
	require.NotNil(t, issued.FileMediaID)
	require.Len(t, mediaSvc.uploads, 1)
	require.Equal(t, "invoice", mediaSvc.uploads[0].BizType)

	reloadedOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, InvoiceStatusIssued, reloadedOrder.InvoiceStatus)
	require.NotNil(t, reloadedOrder.InvoiceFileMediaID)

	download, err := invoiceSvc.CreateDownloadURLForUser(ctx, order.ID, user.ID)
	require.NoError(t, err)
	require.NotEmpty(t, download.URL)
}

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

	return &User{
		ID:       user.ID,
		Email:    user.Email,
		Username: user.Username,
	}, order, providerInstance
}
