package provider

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
	stripewebhook "github.com/stripe/stripe-go/v85/webhook"
)

func TestStripeVerifyNotificationIncludesCurrencyMetadata(t *testing.T) {
	provider, err := NewStripe("stripe-test", map[string]string{
		"secretKey":     "sk_test",
		"webhookSecret": "whsec_test_currency",
	})
	require.NoError(t, err)

	body := `{"id":"evt_currency","object":"event","api_version":"2026-03-25.dahlia","type":"payment_intent.succeeded","data":{"object":{"id":"pi_currency","object":"payment_intent","amount":10000,"currency":"usd","metadata":{"orderId":"sub2_currency"}}}}`
	signed := stripewebhook.GenerateTestSignedPayload(&stripewebhook.UnsignedPayload{
		Payload: []byte(body),
		Secret:  "whsec_test_currency",
	})

	notification, err := provider.VerifyNotification(context.Background(), body, map[string]string{
		"stripe-signature": signed.Header,
	})
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusSuccess, notification.Status)
	require.Equal(t, "USD", notification.Metadata["currency"])
}

func TestStripeRefundCreateParamsUsesStableRequestID(t *testing.T) {
	params, err := stripeRefundCreateParams(context.Background(), payment.RefundRequest{
		TradeNo:   "pi_123",
		Amount:    "12.34",
		RequestID: "refund-request-123",
	}, payment.DefaultPaymentCurrency)

	require.NoError(t, err)
	require.NotNil(t, params.IdempotencyKey)
	require.Equal(t, "refund-request-123", *params.IdempotencyKey)
}
