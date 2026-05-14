package service

import "math"

// OpenAIAccountRuntimeSnapshot is a read-only account-level runtime view.
type OpenAIAccountRuntimeSnapshot struct {
	AccountID          int64   `json:"account_id"`
	ErrorRateEWMA      float64 `json:"error_rate_ewma"`
	TTFTEWMA           float64 `json:"ttft_ewma"`
	HasTTFTEWMA        bool    `json:"has_ttft_ewma"`
	LastRecoveryReason string  `json:"last_recovery_reason"`
}

func newOpenAIAccountRuntimeSnapshot(accountID int64, stat *openAIAccountRuntimeStat) OpenAIAccountRuntimeSnapshot {
	snapshot := OpenAIAccountRuntimeSnapshot{AccountID: accountID}
	if stat == nil {
		return snapshot
	}

	snapshot.ErrorRateEWMA = clamp01(math.Float64frombits(stat.errorRateEWMABits.Load()))
	if reason, ok := stat.lastRecoveryReason.Load().(string); ok {
		snapshot.LastRecoveryReason = reason
	}

	ttft := math.Float64frombits(stat.ttftEWMABits.Load())
	if !math.IsNaN(ttft) {
		snapshot.TTFTEWMA = ttft
		snapshot.HasTTFTEWMA = true
	}
	return snapshot
}
