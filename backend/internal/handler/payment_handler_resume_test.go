//go:build unit

package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type paymentHandlerResumeQueryProvider struct {
	queryCount int
	resp       *payment.QueryOrderResponse
}

func (p *paymentHandlerResumeQueryProvider) Name() string {
	return "payment-handler-resume-query-provider"
}

func (p *paymentHandlerResumeQueryProvider) ProviderKey() string { return payment.TypeAlipay }

func (p *paymentHandlerResumeQueryProvider) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay}
}

func (p *paymentHandlerResumeQueryProvider) CreatePayment(context.Context, payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	panic("unexpected call")
}

func (p *paymentHandlerResumeQueryProvider) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	p.queryCount++
	return p.resp, nil
}

func (p *paymentHandlerResumeQueryProvider) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	panic("unexpected call")
}

func (p *paymentHandlerResumeQueryProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	panic("unexpected call")
}

func TestApplyWeChatPaymentResumeClaims(t *testing.T) {
	t.Parallel()

	req := CreateOrderRequest{
		Amount:      0,
		PaymentType: payment.TypeWxpay,
		OrderType:   payment.OrderTypeBalance,
	}

	err := applyWeChatPaymentResumeClaims(&req, &service.WeChatPaymentResumeClaims{
		OpenID:      "openid-123",
		PaymentType: payment.TypeWxpay,
		Amount:      "12.50",
		OrderType:   payment.OrderTypeSubscription,
		PlanID:      7,
	})
	if err != nil {
		t.Fatalf("applyWeChatPaymentResumeClaims returned error: %v", err)
	}
	if req.OpenID != "openid-123" {
		t.Fatalf("openid = %q, want %q", req.OpenID, "openid-123")
	}
	if req.Amount != 12.5 {
		t.Fatalf("amount = %v, want 12.5", req.Amount)
	}
	if req.OrderType != payment.OrderTypeSubscription {
		t.Fatalf("order_type = %q, want %q", req.OrderType, payment.OrderTypeSubscription)
	}
	if req.PlanID != 7 {
		t.Fatalf("plan_id = %d, want 7", req.PlanID)
	}
}

func TestApplyWeChatPaymentResumeClaimsRejectsPaymentTypeMismatch(t *testing.T) {
	t.Parallel()

	req := CreateOrderRequest{
		PaymentType: payment.TypeAlipay,
	}

	err := applyWeChatPaymentResumeClaims(&req, &service.WeChatPaymentResumeClaims{
		OpenID:      "openid-123",
		PaymentType: payment.TypeWxpay,
		Amount:      "12.50",
		OrderType:   payment.OrderTypeBalance,
	})
	if err == nil {
		t.Fatal("applyWeChatPaymentResumeClaims should reject mismatched payment types")
	}
}

func TestVerifyOrderPublicReturnsLegacyOrderState(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	db, err := sql.Open("sqlite", "file:payment_handler_public_verify?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	user, err := client.User.Create().
		SetEmail("public-verify@example.com").
		SetPasswordHash("hash").
		SetUsername("public-verify-user").
		Save(context.Background())
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(90.64).
		SetFeeRate(0.03).
		SetRechargeCode("PUBLIC-VERIFY").
		SetOutTradeNo("legacy-order-no").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-public-verify").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(service.OrderStatusPending).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderSnapshot(map[string]any{"currency": "HKD"}).
		Save(context.Background())
	require.NoError(t, err)

	paymentSvc := service.NewPaymentService(client, payment.NewRegistry(), nil, nil, nil, nil, nil, nil, nil)
	h := NewPaymentHandler(paymentSvc, nil, nil, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/payment/public/orders/verify",
		bytes.NewBufferString(`{"out_trade_no":"legacy-order-no"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.VerifyOrderPublic(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, "legacy-order-no", resp.Data["out_trade_no"])
	require.Equal(t, service.OrderStatusPending, resp.Data["status"])
	require.Equal(t, false, resp.Data["paid"])
	require.NotEmpty(t, resp.Data["created_at"])
	require.NotEmpty(t, resp.Data["expires_at"])
	for _, field := range []string{
		"id",
		"amount",
		"pay_amount",
		"fee_rate",
		"currency",
		"payment_type",
		"order_type",
		"refund_amount",
		"refund_reason",
		"refund_requested_at",
		"refund_requested_by",
		"refund_request_reason",
		"plan_id",
	} {
		require.NotContains(t, resp.Data, field)
	}
	require.NotZero(t, order.ID)
}

func TestVerifyOrderPublicReconcilesLegacyFailedOrderWithoutResumeToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	db, err := sql.Open("sqlite", "file:payment_handler_public_verify_failed?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	user, err := client.User.Create().
		SetEmail("public-verify-failed@example.com").
		SetPasswordHash("hash").
		SetUsername("public-verify-failed-user").
		Save(context.Background())
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("PUBLIC-VERIFY-FAILED").
		SetOutTradeNo("legacy-failed-order-no").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(service.OrderStatusFailed).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetFailedAt(time.Now().Add(-5 * time.Minute)).
		SetFailedReason("legacy result page re-poll after local failure").
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(context.Background())
	require.NoError(t, err)

	registry := payment.NewRegistry()
	provider := &paymentHandlerResumeQueryProvider{
		resp: &payment.QueryOrderResponse{
			TradeNo: "upstream-trade-public-handler-recovery",
			Status:  payment.ProviderStatusPaid,
			Amount:  88,
		},
	}
	registry.Register(provider)

	paymentSvc := service.NewPaymentService(client, registry, nil, nil, nil, nil, nil, nil, nil)
	h := NewPaymentHandler(paymentSvc, nil, nil, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/payment/public/orders/verify",
		bytes.NewBufferString(`{"out_trade_no":"legacy-failed-order-no"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.VerifyOrderPublic(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, provider.queryCount)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			OutTradeNo  string `json:"out_trade_no"`
			Status      string `json:"status"`
			Paid        bool   `json:"paid"`
			PaidAt      string `json:"paid_at"`
			CompletedAt string `json:"completed_at"`
			CreatedAt   string `json:"created_at"`
			ExpiresAt   string `json:"expires_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, "legacy-failed-order-no", resp.Data.OutTradeNo)
	require.Equal(t, service.OrderStatusCompleted, resp.Data.Status)
	require.True(t, resp.Data.Paid)
	require.NotEmpty(t, resp.Data.PaidAt)
	require.NotEmpty(t, resp.Data.CompletedAt)
	require.NotEmpty(t, resp.Data.CreatedAt)
	require.NotEmpty(t, resp.Data.ExpiresAt)

	reloadedOrder, err := client.PaymentOrder.Get(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, service.OrderStatusCompleted, reloadedOrder.Status)
	require.Equal(t, "upstream-trade-public-handler-recovery", reloadedOrder.PaymentTradeNo)
	require.NotNil(t, reloadedOrder.PaidAt)
	require.NotNil(t, reloadedOrder.CompletedAt)
	require.Nil(t, reloadedOrder.FailedAt)
	require.Nil(t, reloadedOrder.FailedReason)

	reloadedUser, err := client.User.Get(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, 88.0, reloadedUser.Balance)
	require.Equal(t, 88.0, reloadedUser.TotalRecharged)
}

func TestResolveOrderPublicByResumeTokenReturnsFrontendContractFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PAYMENT_RESUME_SIGNING_KEY", "0123456789abcdef0123456789abcdef")

	db, err := sql.Open("sqlite", "file:payment_handler_public_resolve?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	user, err := client.User.Create().
		SetEmail("public-resolve@example.com").
		SetPasswordHash("hash").
		SetUsername("public-resolve-user").
		Save(context.Background())
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(103).
		SetFeeRate(0.03).
		SetRechargeCode("PUBLIC-RESOLVE").
		SetOutTradeNo("resolve-order-no").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-public-resolve").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(service.OrderStatusPaid).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderSnapshot(map[string]any{"currency": "USD"}).
		Save(context.Background())
	require.NoError(t, err)

	resumeSvc := service.NewPaymentResumeService([]byte("0123456789abcdef0123456789abcdef"))
	token, err := resumeSvc.CreateToken(service.ResumeTokenClaims{
		OrderID:            order.ID,
		UserID:             user.ID,
		PaymentType:        payment.TypeAlipay,
		CanonicalReturnURL: "https://app.example.com/payment/result",
	})
	require.NoError(t, err)

	configSvc := service.NewPaymentConfigService(client, nil, []byte("0123456789abcdef0123456789abcdef"))
	paymentSvc := service.NewPaymentService(client, payment.NewRegistry(), nil, nil, nil, configSvc, nil, nil, nil)
	h := NewPaymentHandler(paymentSvc, nil, nil, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/payment/public/orders/resolve",
		bytes.NewBufferString(`{"resume_token":"`+token+`"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.ResolveOrderPublicByResumeToken(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, float64(order.ID), resp.Data["id"])
	require.Equal(t, "resolve-order-no", resp.Data["out_trade_no"])
	require.Equal(t, 100.0, resp.Data["amount"])
	require.Equal(t, 103.0, resp.Data["pay_amount"])
	require.Equal(t, 0.03, resp.Data["fee_rate"])
	require.Equal(t, "USD", resp.Data["currency"])
	require.Equal(t, payment.TypeAlipay, resp.Data["payment_type"])
	require.Equal(t, payment.OrderTypeBalance, resp.Data["order_type"])
	require.Equal(t, service.OrderStatusPaid, resp.Data["status"])
	require.Contains(t, resp.Data, "created_at")
	require.Contains(t, resp.Data, "expires_at")
	require.Contains(t, resp.Data, "refund_amount")
}

func TestResolveOrderPublicByResumeTokenReturnsBadRequestForMismatchedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PAYMENT_RESUME_SIGNING_KEY", "0123456789abcdef0123456789abcdef")

	db, err := sql.Open("sqlite", "file:payment_handler_public_resolve_mismatch?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	user, err := client.User.Create().
		SetEmail("public-resolve-mismatch@example.com").
		SetPasswordHash("hash").
		SetUsername("public-resolve-mismatch-user").
		Save(context.Background())
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(103).
		SetFeeRate(0.03).
		SetRechargeCode("PUBLIC-RESOLVE-MISMATCH").
		SetOutTradeNo("resolve-order-mismatch-no").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-public-resolve-mismatch").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(service.OrderStatusPaid).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(context.Background())
	require.NoError(t, err)

	resumeSvc := service.NewPaymentResumeService([]byte("0123456789abcdef0123456789abcdef"))
	token, err := resumeSvc.CreateToken(service.ResumeTokenClaims{
		OrderID:            order.ID,
		UserID:             user.ID + 999,
		PaymentType:        payment.TypeAlipay,
		CanonicalReturnURL: "https://app.example.com/payment/result",
	})
	require.NoError(t, err)

	configSvc := service.NewPaymentConfigService(client, nil, []byte("0123456789abcdef0123456789abcdef"))
	paymentSvc := service.NewPaymentService(client, payment.NewRegistry(), nil, nil, nil, configSvc, nil, nil, nil)
	h := NewPaymentHandler(paymentSvc, nil, nil, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/payment/public/orders/resolve",
		bytes.NewBufferString(`{"resume_token":"`+token+`"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.ResolveOrderPublicByResumeToken(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var resp struct {
		Code    int    `json:"code"`
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Equal(t, "INVALID_RESUME_TOKEN", resp.Reason)
}

func TestVerifyOrderPublicRejectsBlankOutTradeNo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := sql.Open("sqlite", "file:payment_handler_public_verify_blank?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	paymentSvc := service.NewPaymentService(client, payment.NewRegistry(), nil, nil, nil, nil, nil, nil, nil)
	h := NewPaymentHandler(paymentSvc, nil, nil, nil)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/payment/public/orders/verify",
		bytes.NewBufferString(`{"out_trade_no":"   "}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.VerifyOrderPublic(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)

	var resp struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, http.StatusBadRequest, resp.Code)
	require.Equal(t, "INVALID_OUT_TRADE_NO", resp.Reason)
}
