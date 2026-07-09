package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestAdminPaymentOrdersIncludeActiveInvoiceFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := t.Context()
	client := newAdminPaymentHandlerTestClient(t)
	paymentSvc := service.NewPaymentService(client, nil, nil, nil, nil, nil, nil, nil, nil)
	invoiceSvc := service.NewInvoiceService(client, paymentSvc, nil)
	handler := NewPaymentHandler(paymentSvc, nil, invoiceSvc)

	user, err := client.User.Create().
		SetEmail("admin-invoice-user@example.com").
		SetPasswordHash("hash").
		SetUsername("admin-invoice-user").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeWxpay).
		SetName("admin-invoice-provider").
		SetConfig("{}").
		SetSupportedTypes("wxpay").
		SetEnabled(true).
		SetInvoiceEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("ADMIN-INV-ORDER").
		SetOutTradeNo("sub2_admin_invoice_order").
		SetPaymentType(payment.TypeWxpay).
		SetPaymentTradeNo("trade-admin-invoice-order").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(service.OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetCompletedAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetProviderInstanceID("1").
		SetProviderKey(payment.TypeWxpay).
		Save(ctx)
	require.NoError(t, err)

	inv, err := invoiceSvc.Create(ctx, user.ID, service.CreateInvoiceRequest{
		OrderIDs:  []int64{order.ID},
		Title:     "ACME",
		TaxNumber: "TX",
		Email:     "billing@example.com",
	})
	require.NoError(t, err)
	router := gin.New()
	router.GET("/admin/payment/orders", handler.ListOrders)
	router.GET("/admin/payment/orders/:id", handler.GetOrderDetail)

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/admin/payment/orders", nil))
	require.Equal(t, http.StatusOK, listRecorder.Code)
	var listResp struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRecorder.Body.Bytes(), &listResp))
	require.Len(t, listResp.Data.Items, 1)
	require.Equal(t, float64(inv.ID), listResp.Data.Items[0]["invoice_id"])
	require.Equal(t, service.InvoiceStatusApplied, listResp.Data.Items[0]["invoice_status"])
	require.NotContains(t, listResp.Data.Items[0], "provider_snapshot")

	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, httptest.NewRequest(http.MethodGet, "/admin/payment/orders/1", nil))
	require.Equal(t, http.StatusOK, detailRecorder.Code)
	var detailResp struct {
		Data struct {
			Order map[string]any `json:"order"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(detailRecorder.Body.Bytes(), &detailResp))
	require.Equal(t, float64(inv.ID), detailResp.Data.Order["invoice_id"])
	require.Equal(t, service.InvoiceStatusApplied, detailResp.Data.Order["invoice_status"])
	require.NotContains(t, detailResp.Data.Order, "provider_snapshot")
}

func TestAdminRefundPreviewAllowsRefundRequestedOrderWithActiveInvoice(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := t.Context()
	client := newAdminPaymentHandlerTestClient(t)
	paymentSvc := service.NewPaymentService(client, nil, nil, nil, nil, nil, nil, nil, nil)
	invoiceSvc := service.NewInvoiceService(client, paymentSvc, nil)
	handler := NewPaymentHandler(paymentSvc, nil, invoiceSvc)

	user, err := client.User.Create().
		SetEmail("admin-refund-preview@example.com").
		SetPasswordHash("hash").
		SetUsername("admin-refund-preview-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeWxpay).
		SetName("admin-refund-preview-provider").
		SetConfig("{}").
		SetSupportedTypes("wxpay").
		SetEnabled(true).
		SetRefundEnabled(true).
		SetInvoiceEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("ADMIN-REFUND-PREVIEW").
		SetOutTradeNo("sub2_admin_refund_preview").
		SetPaymentType(payment.TypeWxpay).
		SetPaymentTradeNo("trade-admin-refund-preview").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(service.OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetCompletedAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		SetProviderKey(payment.TypeWxpay).
		Save(ctx)
	require.NoError(t, err)

	_, err = invoiceSvc.Create(ctx, user.ID, service.CreateInvoiceRequest{
		OrderIDs:  []int64{order.ID},
		Title:     "ACME",
		TaxNumber: "TX",
		Email:     "billing@example.com",
	})
	require.NoError(t, err)
	order, err = client.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(service.OrderStatusRefundRequested).
		SetRefundRequestedAmount(40).
		SetRefundRequestedAt(time.Now()).
		SetRefundRequestedBy("user").
		SetRefundRequestReason("please refund").
		Save(ctx)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/admin/payment/orders/:id/refund-preview", handler.GetRefundPreview)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/payment/orders/1/refund-preview", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Data service.RefundPreview `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, order.ID, resp.Data.OrderID)
	require.Equal(t, 100.0, resp.Data.MaxRefundAmount)
}

func TestAdminRetryRefundAffiliateRebateReversalRouteIsCallable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := t.Context()
	client := newAdminPaymentHandlerTestClient(t)
	paymentSvc := service.NewPaymentService(client, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewPaymentHandler(paymentSvc, nil, nil)

	user, err := client.User.Create().
		SetEmail("admin-rebate-retry@example.com").
		SetPasswordHash("hash").
		SetUsername("admin-rebate-retry-user").
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeAlipay).
		SetName("admin-rebate-retry-provider").
		SetConfig("{}").
		SetSupportedTypes("alipay").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("ADMIN-REBATE-RETRY").
		SetOutTradeNo("sub2_admin_rebate_retry").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-admin-rebate-retry").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(service.OrderStatusRefunded).
		SetRefundAmount(100).
		SetRefundAt(time.Now()).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetCompletedAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		SetProviderKey(payment.TypeAlipay).
		Save(ctx)
	require.NoError(t, err)

	router := gin.New()
	router.POST("/admin/payment/orders/:id/refund/rebate-reversal-retry", handler.RetryRefundAffiliateRebateReversal)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/admin/payment/orders/1/refund/rebate-reversal-retry", nil))
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func newAdminPaymentHandlerTestClient(t *testing.T) *dbent.Client {
	t.Helper()
	db, err := sql.Open("sqlite", "file:admin_payment_handler_invoice?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}
