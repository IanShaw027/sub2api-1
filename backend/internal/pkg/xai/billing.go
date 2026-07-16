package xai

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Billing endpoints live on the Grok CLI chat proxy, not on api.x.ai inference base_url.
// Grok CLI /usage show → GET {CLIBase}/billing?format=credits
// Monthly used/limit   → GET {CLIBase}/billing

const (
	BillingPathCredits   = "/billing?format=credits"
	BillingPathMonthly   = "/billing"
	UserPathSubscription = "/user?include=subscription"
)

// MoneyVal mirrors xAI protobuf-style { "val": number } wrappers.
type MoneyVal struct {
	Val float64 `json:"val"`
}

// UsagePeriod is the official weekly credit window.
type UsagePeriod struct {
	Type  string `json:"type"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// ProductUsageEntry is per-product utilization within the credit window.
type ProductUsageEntry struct {
	Product      string  `json:"product"`
	UsagePercent float64 `json:"usagePercent"`
}

// CreditsBillingConfig is the body of GET /billing?format=credits → config.
type CreditsBillingConfig struct {
	CurrentPeriod        *UsagePeriod        `json:"currentPeriod,omitempty"`
	CreditUsagePercent   float64             `json:"creditUsagePercent"`
	OnDemandCap          *MoneyVal           `json:"onDemandCap,omitempty"`
	OnDemandUsed         *MoneyVal           `json:"onDemandUsed,omitempty"`
	ProductUsage         []ProductUsageEntry `json:"productUsage,omitempty"`
	IsUnifiedBillingUser bool                `json:"isUnifiedBillingUser"`
	PrepaidBalance       *MoneyVal           `json:"prepaidBalance,omitempty"`
	TopUpMethod          string              `json:"topUpMethod,omitempty"`
	BillingPeriodStart   string              `json:"billingPeriodStart,omitempty"`
	BillingPeriodEnd     string              `json:"billingPeriodEnd,omitempty"`
}

// CreditsBillingResponse is GET /billing?format=credits.
type CreditsBillingResponse struct {
	Config *CreditsBillingConfig `json:"config,omitempty"`
}

// BillingHistoryEntry is one month in the monthly billing history.
type BillingHistoryEntry struct {
	BillingCycle *struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	} `json:"billingCycle,omitempty"`
	IncludedUsed *MoneyVal `json:"includedUsed,omitempty"`
	OnDemandUsed *MoneyVal `json:"onDemandUsed,omitempty"`
	TotalUsed    *MoneyVal `json:"totalUsed,omitempty"`
}

// MonthlyBillingConfig is the body of GET /billing → config.
type MonthlyBillingConfig struct {
	MonthlyLimit       *MoneyVal             `json:"monthlyLimit,omitempty"`
	Used               *MoneyVal             `json:"used,omitempty"`
	OnDemandCap        *MoneyVal             `json:"onDemandCap,omitempty"`
	BillingPeriodStart string                `json:"billingPeriodStart,omitempty"`
	BillingPeriodEnd   string                `json:"billingPeriodEnd,omitempty"`
	History            []BillingHistoryEntry `json:"history,omitempty"`
}

// MonthlyBillingResponse is GET /billing (no format).
type MonthlyBillingResponse struct {
	Config *MonthlyBillingConfig `json:"config,omitempty"`
}

// UserSubscriptionResponse is GET /user?include=subscription.
type UserSubscriptionResponse struct {
	UserID            string `json:"userId,omitempty"`
	Email             string `json:"email,omitempty"`
	SubscriptionTier  string `json:"subscriptionTier,omitempty"`
	HasGrokCodeAccess bool   `json:"hasGrokCodeAccess,omitempty"`
	UserBlockedReason string `json:"userBlockedReason,omitempty"`
}

// BillingSnapshot is the persisted aggregate for passive UI + cache.
type BillingSnapshot struct {
	UpdatedAt         string                `json:"updated_at"`
	Source            string                `json:"source,omitempty"`
	Credits           *CreditsBillingConfig `json:"credits,omitempty"`
	Monthly           *MonthlyBillingConfig `json:"monthly,omitempty"`
	SubscriptionTier  string                `json:"subscription_tier,omitempty"`
	Email             string                `json:"email,omitempty"`
	HasGrokCodeAccess bool                  `json:"has_grok_code_access,omitempty"`
	FetchError        string                `json:"fetch_error,omitempty"`
}

// CLIBillingBaseURL returns the host used for /usage billing APIs.
// Account inference base_url (api.x.ai) does not serve these endpoints.
func CLIBillingBaseURL() string {
	return strings.TrimRight(DefaultCLIBaseURL, "/")
}

// BuildBillingURL joins the CLI proxy base with a path that may include a query string.
func BuildBillingURL(baseURL, pathWithQuery string) (string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = CLIBillingBaseURL()
	}
	base = strings.TrimRight(base, "/")
	pathWithQuery = strings.TrimSpace(pathWithQuery)
	if pathWithQuery == "" {
		return "", fmt.Errorf("billing path is empty")
	}
	if !strings.HasPrefix(pathWithQuery, "/") {
		pathWithQuery = "/" + pathWithQuery
	}
	// path may be "/billing?format=credits"
	parts := strings.SplitN(pathWithQuery, "?", 2)
	pathOnly := parts[0]
	u, err := url.Parse(base + pathOnly)
	if err != nil {
		return "", err
	}
	if len(parts) == 2 {
		u.RawQuery = parts[1]
	}
	return u.String(), nil
}

// ParseCreditsBilling decodes GET /billing?format=credits body.
func ParseCreditsBilling(body []byte) (*CreditsBillingResponse, error) {
	var out CreditsBillingResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ParseMonthlyBilling decodes GET /billing body.
func ParseMonthlyBilling(body []byte) (*MonthlyBillingResponse, error) {
	var out MonthlyBillingResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ParseUserSubscription decodes GET /user?include=subscription body.
func ParseUserSubscription(body []byte) (*UserSubscriptionResponse, error) {
	var out UserSubscriptionResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Money returns the numeric value or 0.
func Money(v *MoneyVal) float64 {
	if v == nil {
		return 0
	}
	return v.Val
}

// ParseRFC3339Flexible parses xAI billing timestamps.
func ParseRFC3339Flexible(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return &t, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// WeeklyUtilization returns creditUsagePercent from credits config.
func WeeklyUtilization(cfg *CreditsBillingConfig) float64 {
	if cfg == nil {
		return 0
	}
	return cfg.CreditUsagePercent
}

// WeeklyPeriodBounds returns official weekly window start/end.
func WeeklyPeriodBounds(cfg *CreditsBillingConfig) (start, end *time.Time) {
	if cfg == nil || cfg.CurrentPeriod == nil {
		return nil, nil
	}
	if t, err := ParseRFC3339Flexible(cfg.CurrentPeriod.Start); err == nil {
		start = t
	}
	if t, err := ParseRFC3339Flexible(cfg.CurrentPeriod.End); err == nil {
		end = t
	}
	return start, end
}

// MonthlyUtilization returns used/limit percent (0-100+).
func MonthlyUtilization(cfg *MonthlyBillingConfig) float64 {
	if cfg == nil {
		return 0
	}
	limit := Money(cfg.MonthlyLimit)
	if limit <= 0 {
		return 0
	}
	return Money(cfg.Used) / limit * 100
}

// MonthlyPeriodEnd returns billingPeriodEnd.
func MonthlyPeriodEnd(cfg *MonthlyBillingConfig) *time.Time {
	if cfg == nil {
		return nil
	}
	t, err := ParseRFC3339Flexible(cfg.BillingPeriodEnd)
	if err != nil {
		return nil
	}
	return t
}
