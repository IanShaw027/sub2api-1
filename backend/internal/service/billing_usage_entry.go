package service

import "time"

// BillingUsageEntry is an immutable ledger entry representing the billing delta for a usage log.
// It is the foundation for reconciliation between usage_logs and billing state.
type BillingUsageEntry struct {
	ID int64

	UsageLogID int64
	UserID     int64
	APIKeyID   int64

	SubscriptionID *int64

	BillingType int8
	Applied     bool
	DeltaUSD    float64

	CreatedAt time.Time
}

