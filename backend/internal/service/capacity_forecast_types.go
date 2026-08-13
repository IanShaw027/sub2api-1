package service

import (
	"context"
	"time"
)

// capacityMinimumInferenceRatio mirrors the OpenAI-OAuth-specific engine's
// openAIOAuthMinimumInferencePercent (5%) threshold: below this used_ratio,
// spend/used_ratio is too noisy to trust as an inferred capacity and the
// engine falls back to the last archived period, then the scope median.
const capacityMinimumInferenceRatio = 0.05

// CapacityHourlySpendPoint is one hourly actual-spend aggregate row.
type CapacityHourlySpendPoint struct {
	BucketStart time.Time
	SpendUSD    float64
}

// CapacityHourlyFact is a previously sealed capacity_hourly row's
// available/capacity figures (nil = unknown, must render as null).
type CapacityHourlyFact struct {
	AvailableUSD *float64
	CapacityUSD  *float64
}

// CapacityHourlyUpsert is one row to seal into capacity_hourly.
type CapacityHourlyUpsert struct {
	Platform     string
	GroupID      *int64
	BucketStart  time.Time
	SpendUSD     float64
	AvailableUSD *float64
	CapacityUSD  *float64
	Sealed       bool
}

// AccountQuotaSnapshot is the latest known window state for one account,
// persisted to account_quota_snapshots.
type AccountQuotaSnapshot struct {
	Platform      string
	AccountID     int64
	WindowKind    string
	UsedRatio     float64
	ResetAt       time.Time
	WindowMinutes int
	ObservedAt    time.Time
}

// AccountQuotaPeriod is an archived (closed) rolling quota window, persisted
// to account_quota_periods.
type AccountQuotaPeriod struct {
	Platform            string
	AccountID           int64
	WindowKind          string
	PeriodStart         time.Time
	PeriodEnd           time.Time
	FinalUsedRatio      *float64
	SpendUSD            float64
	InferredCapacityUSD *float64
	SampleCount         int
}

// CapacityAccountWindowSpendKey identifies one (account, window kind) pair in
// a batched window-spend result.
type CapacityAccountWindowSpendKey struct {
	AccountID  int64
	WindowKind string
}

// CapacityAccountWindowSpendRequest asks for one account window's spend,
// summed from From (the window's start) up to the batch's shared `to`.
type CapacityAccountWindowSpendRequest struct {
	AccountID  int64
	WindowKind string
	From       time.Time
}

// CapacityRepository is the data-access surface for the generic capacity
// forecasting engine. It is intentionally a brand-new interface (rather than
// an addition to AccountRepository/GroupRepository) so existing repository
// mocks/stubs are unaffected.
type CapacityRepository interface {
	// GetHourlySpend returns actual per-hour SUM(total_cost) in [from, to).
	// groupID nil scopes to the whole platform; non-nil scopes to that group.
	GetHourlySpend(ctx context.Context, platform string, groupID *int64, from, to time.Time) ([]CapacityHourlySpendPoint, error)
	// GetAccountSpendBetween sums a single account's total_cost in [from, to).
	GetAccountSpendBetween(ctx context.Context, accountID int64, from, to time.Time) (float64, error)
	// GetAccountWindowSpends batches per-(account, window) spend sums — each
	// request's [From, to) — into a bounded number of round-trips, replacing an
	// N-queries-per-scope pattern during capacity inference.
	GetAccountWindowSpends(ctx context.Context, reqs []CapacityAccountWindowSpendRequest, to time.Time) (map[CapacityAccountWindowSpendKey]float64, error)

	// GetHourlyFacts reads previously sealed capacity_hourly rows in [from, to).
	GetHourlyFacts(ctx context.Context, platform string, groupID *int64, from, to time.Time) (map[time.Time]CapacityHourlyFact, error)
	// UpsertHourlyFacts seals (overwrites) the given hourly rows.
	UpsertHourlyFacts(ctx context.Context, facts []CapacityHourlyUpsert) error

	// UpsertQuotaSnapshot replaces the latest known snapshot for (platform, account, window).
	UpsertQuotaSnapshot(ctx context.Context, snapshot AccountQuotaSnapshot) error
	// InsertQuotaPeriod archives a closed window (idempotent — no-op if already archived).
	InsertQuotaPeriod(ctx context.Context, period AccountQuotaPeriod) error
	// GetLatestQuotaPeriodCapacity returns the most recently archived capacity for
	// (platform, account, window_kind), or nil if none is known.
	GetLatestQuotaPeriodCapacity(ctx context.Context, platform string, accountID int64, windowKind string) (*float64, error)
}

// --- API response contract (must match the documented JSON field names exactly) ---

// CapacityKPIs summarizes the timeseries for quick display.
type CapacityKPIs struct {
	CurrentAvailableUSD    float64    `json:"current_available_usd"`
	FutureForecastSpendUSD float64    `json:"future_forecast_spend_usd"`
	FirstShortfallAt       *time.Time `json:"first_shortfall_at"`
	SuggestAccounts        int        `json:"suggest_accounts"`
}

// CapacityPoint is a single hourly bucket in the timeseries.
type CapacityPoint struct {
	BucketStart          time.Time `json:"bucket_start"`
	Segment              string    `json:"segment"` // "past" | "current" | "future"
	SpendUSD             *float64  `json:"spend_usd"`
	ForecastSpendUSD     *float64  `json:"forecast_spend_usd"`
	AvailableUSD         *float64  `json:"available_usd"`
	ForecastAvailableUSD *float64  `json:"forecast_available_usd"`
	RecoveredUSD         float64   `json:"recovered_usd"`
}

// CapacityEventAccountRef is a minimal account reference attached to an event.
type CapacityEventAccountRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// CapacityEvent is a discrete "recover" (quota window reset) or "shortfall"
// (forecast demand exceeds forecast supply) occurrence.
type CapacityEvent struct {
	At           time.Time                 `json:"at"`
	Type         string                    `json:"type"` // "recover" | "shortfall"
	WindowKind   string                    `json:"window_kind"`
	AccountCount int                       `json:"account_count"`
	AmountUSD    float64                   `json:"amount_usd"`
	Accounts     []CapacityEventAccountRef `json:"accounts"`
}

// CapacityRecommendation suggests how many accounts to add to close a gap.
type CapacityRecommendation struct {
	At              time.Time `json:"at"`
	GapUSD          float64   `json:"gap_usd"`
	SuggestAccounts int       `json:"suggest_accounts"`
	Basis           string    `json:"basis"`
}

// CapacityTimeseries is the full GET .../capacity/timeseries response body.
type CapacityTimeseries struct {
	GeneratedAt     time.Time                `json:"generated_at"`
	Now             time.Time                `json:"now"`
	Platform        string                   `json:"platform"`
	GroupID         *int64                   `json:"group_id"`
	Unit            string                   `json:"unit"`
	Range           string                   `json:"range"`
	KPIs            CapacityKPIs             `json:"kpis"`
	Points          []CapacityPoint          `json:"points"`
	Events          []CapacityEvent          `json:"events"`
	Recommendations []CapacityRecommendation `json:"recommendations"`
}

// CapacityProbeFailure is one failed account probe in the batch response.
type CapacityProbeFailure struct {
	AccountID int64  `json:"account_id"`
	Name      string `json:"name"`
	Error     string `json:"error"`
}

// CapacityProbeResult is the POST .../capacity/probe response body.
type CapacityProbeResult struct {
	Total       int                    `json:"total"`
	Probed      int                    `json:"probed"`
	OK          int                    `json:"ok"`
	RateLimited int                    `json:"rate_limited"`
	Failed      int                    `json:"failed"`
	DurationMS  int64                  `json:"duration_ms"`
	Failures    []CapacityProbeFailure `json:"failures"`
}
