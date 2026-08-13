//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// ---------------------------------------------------------------------------
// Anthropic
// ---------------------------------------------------------------------------

func TestAnthropicExtractWindowsNilAccount(t *testing.T) {
	p := NewAnthropicCapacityProvider(nil, nil)
	require.Nil(t, p.ExtractWindows(nil, time.Now()))
}

func TestAnthropicExtractWindowsMissingSnapshot(t *testing.T) {
	p := NewAnthropicCapacityProvider(nil, nil)
	acc := &Account{ID: 1, Platform: PlatformAnthropic}
	require.Nil(t, p.ExtractWindows(acc, time.Now()))
}

func TestAnthropicExtractWindows5hFromStatus(t *testing.T) {
	p := NewAnthropicCapacityProvider(nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	resetAt := now.Add(2 * time.Hour)
	acc := &Account{
		ID:                  1,
		Platform:            PlatformAnthropic,
		SessionWindowStatus: "rejected",
		SessionWindowEnd:    &resetAt,
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	require.Equal(t, anthropicCapacityWindow5h, windows[0].Kind)
	require.InDelta(t, 1.0, windows[0].UsedRatio, 1e-9)
	require.Equal(t, resetAt, windows[0].ResetAt)
}

func TestAnthropicExtractWindows5hExpiredIsRecovered(t *testing.T) {
	p := NewAnthropicCapacityProvider(nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	start := now.Add(-6 * time.Hour)
	resetAt := now.Add(-1 * time.Hour) // already elapsed
	acc := &Account{
		ID:                 1,
		Platform:           PlatformAnthropic,
		SessionWindowStart: &start,
		SessionWindowEnd:   &resetAt,
		Extra:              map[string]any{"session_window_utilization": 0.9},
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	w := windows[0]
	require.Equal(t, anthropicCapacityWindow5h, w.Kind)
	require.Zero(t, w.UsedRatio)
	require.True(t, w.ResetAt.After(now))
}

func TestAnthropicExtractWindows7dFromExtra(t *testing.T) {
	p := NewAnthropicCapacityProvider(nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	resetAt := now.Add(48 * time.Hour)
	acc := &Account{
		ID:       1,
		Platform: PlatformAnthropic,
		Extra: map[string]any{
			"passive_usage_7d_utilization": 0.42,
			"passive_usage_7d_reset":       float64(resetAt.Unix()),
		},
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	require.Equal(t, anthropicCapacityWindow7d, windows[0].Kind)
	require.InDelta(t, 0.42, windows[0].UsedRatio, 1e-9)
	require.WithinDuration(t, resetAt, windows[0].ResetAt, time.Second)
}

func TestAnthropicExtractWindows7dExpiredIsRecovered(t *testing.T) {
	p := NewAnthropicCapacityProvider(nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	resetAt := now.Add(-24 * time.Hour)
	acc := &Account{
		ID:       1,
		Platform: PlatformAnthropic,
		Extra: map[string]any{
			"passive_usage_7d_utilization": 0.9,
			"passive_usage_7d_reset":       float64(resetAt.Unix()),
		},
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	require.Zero(t, windows[0].UsedRatio)
	require.True(t, windows[0].ResetAt.After(now))
}

// ---------------------------------------------------------------------------
// Grok
// ---------------------------------------------------------------------------

func TestGrokExtractWindowsMissingSnapshot(t *testing.T) {
	p := NewGrokCapacityProvider(nil, nil)
	acc := &Account{ID: 1, Platform: PlatformGrok}
	require.Nil(t, p.ExtractWindows(acc, time.Now()))
}

func TestGrokExtractWindowsFromSnapshot(t *testing.T) {
	p := NewGrokCapacityProvider(nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	updatedAt := now.Add(-30 * time.Minute)
	resetAt := now.Add(30 * time.Minute)
	limit := int64(100)
	remaining := int64(20)
	resetUnix := resetAt.Unix()
	acc := &Account{
		ID:       1,
		Platform: PlatformGrok,
		Extra: map[string]any{
			grokQuotaSnapshotExtraKey: &xai.QuotaSnapshot{
				Requests:  &xai.QuotaWindow{Limit: &limit, Remaining: &remaining, ResetUnix: &resetUnix},
				UpdatedAt: updatedAt.Format(time.RFC3339),
			},
		},
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	require.InDelta(t, 0.8, windows[0].UsedRatio, 1e-9)
	require.WithinDuration(t, resetAt, windows[0].ResetAt, time.Second)
}

func TestGrokExtractWindowsExpiredIsRecovered(t *testing.T) {
	p := NewGrokCapacityProvider(nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	updatedAt := now.Add(-2 * time.Hour)
	resetAt := now.Add(-1 * time.Hour) // already elapsed relative to now
	limit := int64(100)
	remaining := int64(0)
	resetUnix := resetAt.Unix()
	acc := &Account{
		ID:       1,
		Platform: PlatformGrok,
		Extra: map[string]any{
			grokQuotaSnapshotExtraKey: &xai.QuotaSnapshot{
				Requests:  &xai.QuotaWindow{Limit: &limit, Remaining: &remaining, ResetUnix: &resetUnix},
				UpdatedAt: updatedAt.Format(time.RFC3339),
			},
		},
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	require.Zero(t, windows[0].UsedRatio)
	require.True(t, windows[0].ResetAt.After(now))
}

// ---------------------------------------------------------------------------
// Antigravity
// ---------------------------------------------------------------------------

func TestAntigravityExtractWindowsMissingSnapshot(t *testing.T) {
	p := NewAntigravityCapacityProvider(nil, nil, nil)
	acc := &Account{ID: 1, Platform: PlatformAntigravity}
	require.Nil(t, p.ExtractWindows(acc, time.Now()))
}

func TestAntigravityExtractWindowsFromExtra(t *testing.T) {
	p := NewAntigravityCapacityProvider(nil, nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	observedAt := now.Add(-1 * time.Hour)
	resetAt := now.Add(4 * time.Hour)
	acc := &Account{
		ID:       1,
		Platform: PlatformAntigravity,
		Extra: map[string]any{
			"antigravity_sched_utilization":      70,
			"antigravity_sched_reset_at":         resetAt.Format(time.RFC3339),
			"antigravity_sched_usage_updated_at": observedAt.Format(time.RFC3339),
		},
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	require.Equal(t, antigravityCapacityWindowKind, windows[0].Kind)
	require.InDelta(t, 0.7, windows[0].UsedRatio, 1e-9)
	require.WithinDuration(t, resetAt, windows[0].ResetAt, time.Second)
	require.Equal(t, resetAt.Sub(observedAt), windows[0].Duration)
}

func TestAntigravityExtractWindowsExpiredIsRecovered(t *testing.T) {
	p := NewAntigravityCapacityProvider(nil, nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	observedAt := now.Add(-6 * time.Hour)
	resetAt := now.Add(-1 * time.Hour)
	acc := &Account{
		ID:       1,
		Platform: PlatformAntigravity,
		Extra: map[string]any{
			"antigravity_sched_utilization":      100,
			"antigravity_sched_reset_at":         resetAt.Format(time.RFC3339),
			"antigravity_sched_usage_updated_at": observedAt.Format(time.RFC3339),
		},
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	require.Zero(t, windows[0].UsedRatio)
	require.True(t, windows[0].ResetAt.After(now))
}

// ---------------------------------------------------------------------------
// Gemini
// ---------------------------------------------------------------------------

func TestGeminiExtractWindowsMissingSnapshot(t *testing.T) {
	p := NewGeminiCapacityProvider(nil, nil, nil, nil)
	acc := &Account{ID: 1, Platform: PlatformGemini}
	require.Nil(t, p.ExtractWindows(acc, time.Now()))
}

func TestGeminiExtractWindowsFromProbedExtra(t *testing.T) {
	p := NewGeminiCapacityProvider(nil, nil, nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	resetAt := now.Add(6 * time.Hour)
	acc := &Account{
		ID:       1,
		Platform: PlatformGemini,
		Extra: map[string]any{
			geminiCapacityDayUsedRatioExtraKey: 0.33,
			geminiCapacityDayResetAtExtraKey:   resetAt.Format(time.RFC3339),
		},
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	require.Equal(t, geminiCapacityWindowKind, windows[0].Kind)
	require.InDelta(t, 0.33, windows[0].UsedRatio, 1e-9)
	require.WithinDuration(t, resetAt, windows[0].ResetAt, time.Second)
}

func TestGeminiExtractWindowsExpiredIsRecovered(t *testing.T) {
	p := NewGeminiCapacityProvider(nil, nil, nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	resetAt := now.Add(-1 * time.Hour)
	acc := &Account{
		ID:       1,
		Platform: PlatformGemini,
		Extra: map[string]any{
			geminiCapacityDayUsedRatioExtraKey: 0.99,
			geminiCapacityDayResetAtExtraKey:   resetAt.Format(time.RFC3339),
		},
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	require.Zero(t, windows[0].UsedRatio)
	require.True(t, windows[0].ResetAt.After(now))
}

func TestGeminiExtractWindowsFallsBackToUpstreamSnapshot(t *testing.T) {
	p := NewGeminiCapacityProvider(nil, nil, nil, nil)
	now := mustUTC(t, "2026-07-23T10:00:00Z")
	resetAt := now.Add(3 * time.Hour)
	acc := &Account{
		ID:       1,
		Platform: PlatformGemini,
		Credentials: map[string]any{
			"gemini_usage_raw": map[string]any{
				"buckets": []any{
					map[string]any{
						"modelId":           "models/gemini-2.5-pro",
						"remainingFraction": 0.25,
						"resetTime":         resetAt.Format(time.RFC3339),
					},
				},
			},
		},
	}
	windows := p.ExtractWindows(acc, now)
	require.Len(t, windows, 1)
	require.InDelta(t, 0.75, windows[0].UsedRatio, 1e-9)
	require.WithinDuration(t, resetAt, windows[0].ResetAt, time.Second)
}
