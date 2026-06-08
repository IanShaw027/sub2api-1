package handler

import (
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestSanitizePaymentOrderForResponse(t *testing.T) {
	now := time.Now().UTC()
	order := &dbent.PaymentOrder{
		ID:                    42,
		UserID:                7,
		Amount:                100,
		PayAmount:             95,
		FeeRate:               0.05,
		PaymentType:           "alipay",
		OutTradeNo:            "trade-42",
		Status:                "COMPLETED",
		OrderType:             "balance",
		CreatedAt:             now,
		ExpiresAt:             now.Add(time.Hour),
		RefundRequestedAmount: 12.5,
		ProviderInstanceID:    strPtr("provider-1"),
	}

	result := sanitizePaymentOrderForResponse(order)

	require.NotNil(t, result)
	require.Equal(t, 12.5, result.RefundRequestedAmount)
	require.Equal(t, strPtr("provider-1"), result.ProviderInstanceID)
	// Invoice fields (InvoiceStatus, InvoiceID) are populated by
	// enrichOrdersWithInvoice via a join on Invoice/InvoiceOrder, not by
	// sanitizePaymentOrderForResponse. Coverage lives in invoice service tests.
	require.Empty(t, result.InvoiceStatus)
	require.Nil(t, result.InvoiceID)
}

func strPtr(v string) *string {
	return &v
}
