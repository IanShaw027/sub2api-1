package handler

import (
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestSanitizePaymentOrderForResponseIncludesInvoiceFields(t *testing.T) {
	now := time.Now().UTC()
	invoiceFileMediaID := int64(99)
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
		InvoiceStatus:         "ISSUED",
		InvoiceFileMediaID:    &invoiceFileMediaID,
	}

	result := sanitizePaymentOrderForResponse(order)

	require.NotNil(t, result)
	require.Equal(t, 12.5, result.RefundRequestedAmount)
	require.Equal(t, "ISSUED", result.InvoiceStatus)
	require.Equal(t, &invoiceFileMediaID, result.InvoiceFileMediaID)
	require.Equal(t, strPtr("provider-1"), result.ProviderInstanceID)
}

func strPtr(v string) *string {
	return &v
}
