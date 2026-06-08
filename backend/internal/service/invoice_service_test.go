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

func newInvoiceTestService(t *testing.T) (*dbent.Client, *invoiceTestMediaService, *InvoiceService) {
	t.Helper()
	client := newPaymentConfigServiceTestClient(t)
	media := &invoiceTestMediaService{}
	svc := NewInvoiceService(client, &PaymentService{entClient: client}, media)
	return client, media, svc
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

// 静态使用占位（避免未使用 import 报错），后续任务真正使用后删除。
var _ = context.Background
var _ infraerrors.Error
