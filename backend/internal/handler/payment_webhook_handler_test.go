//go:build unit

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stripewebhook "github.com/stripe/stripe-go/v85/webhook"
	_ "modernc.org/sqlite"
)

const stripeWebhookHandlerTestSecret = "whsec_payment_webhook_handler_test"

func TestWriteSuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		providerKey     string
		wantCode        int
		wantContentType string
		wantBody        string
		checkJSON       bool
		wantJSONCode    string
		wantJSONMessage string
	}{
		{
			name:            "wxpay returns JSON with code SUCCESS",
			providerKey:     "wxpay",
			wantCode:        http.StatusOK,
			wantContentType: "application/json",
			checkJSON:       true,
			wantJSONCode:    "SUCCESS",
			wantJSONMessage: "成功",
		},
		{
			name:            "stripe returns empty 200",
			providerKey:     "stripe",
			wantCode:        http.StatusOK,
			wantContentType: "text/plain",
			wantBody:        "",
		},
		{
			name:            "easypay returns plain text success",
			providerKey:     "easypay",
			wantCode:        http.StatusOK,
			wantContentType: "text/plain",
			wantBody:        "success",
		},
		{
			name:            "alipay returns plain text success",
			providerKey:     "alipay",
			wantCode:        http.StatusOK,
			wantContentType: "text/plain",
			wantBody:        "success",
		},
		{
			name:            "unknown provider returns plain text success",
			providerKey:     "unknown_provider",
			wantCode:        http.StatusOK,
			wantContentType: "text/plain",
			wantBody:        "success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			writeSuccessResponse(c, tt.providerKey)

			assert.Equal(t, tt.wantCode, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), tt.wantContentType)

			if tt.checkJSON {
				var resp wxpaySuccessResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err, "response body should be valid JSON")
				assert.Equal(t, tt.wantJSONCode, resp.Code)
				assert.Equal(t, tt.wantJSONMessage, resp.Message)
			} else {
				assert.Equal(t, tt.wantBody, w.Body.String())
			}
		})
	}
}

// TestUnknownOrderWebhookAcksWithSuccess exercises the response contract that
// handleNotify relies on when HandlePaymentNotification returns ErrOrderNotFound:
// we still need to emit the provider-specific 2xx so the provider stops
// retrying. We can't easily drive handleNotify end-to-end without mocking the
// concrete *service.PaymentService, so this test locks down the two ingredients
// the fix depends on:
//  1. errors.Is recognises the sentinel through fmt.Errorf %w wrapping (which
//     is how service layer wraps it with the out_trade_no context).
//  2. writeSuccessResponse produces the provider-specific body for Stripe
//     (empty 200) — matching what handleNotify calls on the ack path.
//
// If either contract breaks, the Stripe "unknown order → 500 loop" regresses.
func TestUnknownOrderWebhookAcksWithSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 1) Sentinel recognition through wrapping.
	wrapped := fmt.Errorf("%w: out_trade_no=sub2_missing_42", service.ErrOrderNotFound)
	require.True(t, errors.Is(wrapped, service.ErrOrderNotFound),
		"handleNotify uses errors.Is on the wrapped service error; regression here "+
			"would mean unknown-order webhooks go back to returning 500 and looping forever")

	// A distinct error must NOT match — otherwise a DB failure would be silently
	// swallowed as an ack.
	other := errors.New("lookup order failed: connection refused")
	require.False(t, errors.Is(other, service.ErrOrderNotFound))

	// 2) Provider-specific success body is what handleNotify emits on the
	// ack path. Asserted again here because this is the shape Stripe expects
	// to consider the webhook acknowledged.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeSuccessResponse(c, payment.TypeStripe)
	require.Equal(t, http.StatusOK, w.Code,
		"Stripe requires 2xx to stop retrying; anything else restarts the retry loop")
	require.Empty(t, w.Body.String(), "Stripe expects an empty body on the ack path")
}

func TestWebhookConstants(t *testing.T) {
	t.Run("maxWebhookBodySize is 1MB", func(t *testing.T) {
		assert.Equal(t, int64(1<<20), int64(maxWebhookBodySize))
	})

	t.Run("webhookLogTruncateLen is 200", func(t *testing.T) {
		assert.Equal(t, 200, webhookLogTruncateLen)
	})
}

func TestHandleNotify_ProviderResolutionMissingOrder_AcksStripe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := newPaymentWebhookHandlerTestClient(t)
	_, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeStripe).
		SetName("stripe-a").
		SetConfig("{}").
		SetSupportedTypes("stripe").
		SetEnabled(true).
		Save(context.Background())
	require.NoError(t, err)
	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeStripe).
		SetName("stripe-b").
		SetConfig("{}").
		SetSupportedTypes("stripe").
		SetEnabled(true).
		Save(context.Background())
	require.NoError(t, err)

	registry := payment.NewRegistry()
	paymentService := service.NewPaymentService(client, registry, nil, nil, nil, nil, nil, nil, nil)
	h := NewPaymentWebhookHandler(paymentService, registry)

	body := `{"data":{"object":{"metadata":{"out_trade_no":"sub2_resolution_missing_order"}}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment/webhook/stripe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.handleNotify(c, payment.TypeStripe)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Body.String())
}

func TestHandleNotify_LegacyFallbackMissingOrder_AcksStripe(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := newPaymentWebhookHandlerTestClient(t)
	registry := payment.NewRegistry()
	registry.Register(webhookHandlerProviderStub{
		key: payment.TypeStripe,
		notification: &payment.PaymentNotification{
			OrderID: "sub2_987654",
			TradeNo: "stripe-legacy-missing-order",
			Status:  payment.NotificationStatusSuccess,
			Amount:  100,
		},
	})
	paymentService := service.NewPaymentService(client, registry, nil, nil, nil, nil, nil, nil, nil)
	h := NewPaymentWebhookHandler(paymentService, registry)

	body := `{"data":{"object":{"metadata":{"out_trade_no":"sub2_987654"}}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment/webhook/stripe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.handleNotify(c, payment.TypeStripe)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Body.String())
}

func TestExtractOutTradeNo(t *testing.T) {
	tests := []struct {
		name        string
		providerKey string
		rawBody     string
		want        string
	}{
		{
			name:        "easypay query payload",
			providerKey: "easypay",
			rawBody:     "out_trade_no=sub2_123&trade_status=TRADE_SUCCESS",
			want:        "sub2_123",
		},
		{
			name:        "alipay query payload",
			providerKey: "alipay",
			rawBody:     "notify_time=2026-04-20+12%3A00%3A00&out_trade_no=sub2_456",
			want:        "sub2_456",
		},
		{
			name:        "unknown provider",
			providerKey: "wxpay",
			rawBody:     "{}",
			want:        "",
		},
		{
			name:        "stripe nested metadata",
			providerKey: payment.TypeStripe,
			rawBody:     `{"data":{"object":{"metadata":{"out_trade_no":"sub2_789"}}}}`,
			want:        "sub2_789",
		},
		{
			name:        "stripe payment intent orderId metadata",
			providerKey: payment.TypeStripe,
			rawBody:     `{"type":"payment_intent.succeeded","data":{"object":{"object":"payment_intent","metadata":{"orderId":"sub2_stripe_order_id"}}}}`,
			want:        "sub2_stripe_order_id",
		},
		{
			name:        "wxpay resource out_trade_no",
			providerKey: payment.TypeWxpay,
			rawBody:     `{"resource":{"out_trade_no":"sub2_900"}}`,
			want:        "sub2_900",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractOutTradeNo(tt.rawBody, tt.providerKey))
		})
	}
}

func TestHandleNotify_StripeSignedPaymentIntentUsesOrderIDMetadataForPinnedInstance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()

	client := newPaymentWebhookHandlerTestClient(t)
	instA, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeStripe).
		SetName("stripe-a").
		SetConfig(stripeWebhookHandlerConfig(t, map[string]string{
			"secretKey":     "sk_test_a",
			"webhookSecret": stripeWebhookHandlerTestSecret,
		})).
		SetSupportedTypes("stripe").
		SetEnabled(true).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeStripe).
		SetName("stripe-b").
		SetConfig(stripeWebhookHandlerConfig(t, map[string]string{
			"secretKey":     "sk_test_b",
			"webhookSecret": "whsec_wrong_instance",
		})).
		SetSupportedTypes("stripe").
		SetEnabled(true).
		Save(ctx)
	require.NoError(t, err)
	user, err := client.User.Create().
		SetEmail("stripe-webhook@example.com").
		SetPasswordHash("hash").
		SetUsername("stripe-webhook-user").
		Save(ctx)
	require.NoError(t, err)

	instID := strconv.FormatInt(instA.ID, 10)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(100).
		SetPayAmount(100).
		SetFeeRate(0).
		SetRechargeCode("STRIPE-WEBHOOK-ORDERID").
		SetOutTradeNo("sub2_stripe_webhook_orderid").
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("pi_stripe_webhook_orderid").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(service.OrderStatusCompleted).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(instID).
		SetProviderKey(payment.TypeStripe).
		Save(ctx)
	require.NoError(t, err)

	registry := payment.NewRegistry()
	paymentService := service.NewPaymentService(client, registry, newStripeWebhookHandlerLoadBalancer(client), nil, nil, nil, nil, nil, nil)
	h := NewPaymentWebhookHandler(paymentService, registry)

	body := fmt.Sprintf(`{
		"id":"evt_stripe_webhook_orderid",
		"object":"event",
		"api_version":"2026-03-25.dahlia",
		"type":"payment_intent.succeeded",
		"data":{"object":{
			"id":"pi_stripe_webhook_orderid",
			"object":"payment_intent",
			"amount":10000,
			"metadata":{"orderId":%q}
		}}
	}`, order.OutTradeNo)
	signed := stripewebhook.GenerateTestSignedPayload(&stripewebhook.UnsignedPayload{
		Payload: []byte(body),
		Secret:  stripeWebhookHandlerTestSecret,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment/webhook/stripe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signed.Header)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.handleNotify(c, payment.TypeStripe)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Body.String())
}

func TestVerifyNotificationWithProvidersReturnsMatchedProvider(t *testing.T) {
	firstErr := errors.New("wrong provider")
	providers := []payment.Provider{
		webhookHandlerProviderStub{
			key:       payment.TypeWxpay,
			verifyErr: firstErr,
		},
		webhookHandlerProviderStub{
			key: payment.TypeWxpay,
			notification: &payment.PaymentNotification{
				OrderID: "sub2_42",
				TradeNo: "trade-42",
				Status:  payment.NotificationStatusSuccess,
			},
		},
	}

	providerKey, notification, err := verifyNotificationWithProviders(context.Background(), providers, "{}", map[string]string{"wechatpay-signature": "sig"})
	require.NoError(t, err)
	require.Equal(t, payment.TypeWxpay, providerKey)
	require.NotNil(t, notification)
	require.Equal(t, "sub2_42", notification.OrderID)
}

func TestVerifyNotificationWithProvidersFailsWhenAllProvidersReject(t *testing.T) {
	providers := []payment.Provider{
		webhookHandlerProviderStub{
			key:       payment.TypeWxpay,
			verifyErr: errors.New("verify failed a"),
		},
		webhookHandlerProviderStub{
			key:       payment.TypeWxpay,
			verifyErr: errors.New("verify failed b"),
		},
	}

	_, _, err := verifyNotificationWithProviders(context.Background(), providers, "{}", nil)
	require.Error(t, err)
}

type webhookHandlerProviderStub struct {
	key          string
	notification *payment.PaymentNotification
	verifyErr    error
}

func (p webhookHandlerProviderStub) Name() string        { return p.key }
func (p webhookHandlerProviderStub) ProviderKey() string { return p.key }
func (p webhookHandlerProviderStub) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.PaymentType(p.key)}
}
func (p webhookHandlerProviderStub) CreatePayment(context.Context, payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	panic("unexpected call")
}
func (p webhookHandlerProviderStub) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	panic("unexpected call")
}
func (p webhookHandlerProviderStub) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	if p.verifyErr != nil {
		return nil, p.verifyErr
	}
	return p.notification, nil
}
func (p webhookHandlerProviderStub) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	panic("unexpected call")
}

func newPaymentWebhookHandlerTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", "file:payment_webhook_handler?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func stripeWebhookHandlerConfig(t *testing.T, config map[string]string) string {
	t.Helper()

	data, err := json.Marshal(config)
	require.NoError(t, err)

	encrypted, err := payment.Encrypt(string(data), []byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	return encrypted
}

func newStripeWebhookHandlerLoadBalancer(client *dbent.Client) payment.LoadBalancer {
	return payment.NewDefaultLoadBalancer(client, []byte("0123456789abcdef0123456789abcdef"))
}
