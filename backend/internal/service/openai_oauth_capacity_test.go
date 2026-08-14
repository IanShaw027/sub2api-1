//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthCapacityWindowFromExtra(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	resetAt := now.Add(2 * time.Hour)
	window := openAIOAuthWindowFromExtra(map[string]any{
		"codex_5h_used_percent":   80.0,
		"codex_5h_reset_at":       resetAt.Format(time.RFC3339),
		"codex_5h_window_minutes": 300,
	}, "5h", now, 12.0)

	require.NotNil(t, window.RemainingPercent)
	require.InDelta(t, 20.0, *window.RemainingPercent, 0.01)
	require.Greater(t, window.BurnRate, 1.0)
	require.NotNil(t, window.ExhaustsAt)
	require.Equal(t, "warning", window.Alert)
}

func TestOpenAIOAuthCapacityWarning_RequiresSustainedBurn(t *testing.T) {
	require.Equal(t, "", openAIOAuthAlertLevel(20, 0.8, 0.8, false))
	require.Equal(t, "warning", openAIOAuthAlertLevel(10, 1.2, 1.1, false))
	require.Equal(t, "critical", openAIOAuthAlertLevel(8, 2.0, 1.5, true))
}

func TestOpenAIOAuthCapacityWindowRecoversExpiredReset(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	window := openAIOAuthWindowFromExtra(map[string]any{
		"codex_5h_used_percent":   90.0,
		"codex_5h_reset_at":       now.Add(-time.Hour).Format(time.RFC3339),
		"codex_5h_window_minutes": 300,
	}, "5h", now, 0)
	require.NotNil(t, window.RemainingPercent)
	require.InDelta(t, 100.0, *window.RemainingPercent, 0.01)
	require.NotNil(t, window.ResetAt)
	require.True(t, window.ResetAt.After(now))
	require.Equal(t, "", window.Alert)
}

func TestOpenAIOAuthUnmeasuredLongWindowDoesNotSuppressShortAlert(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	scope := buildOpenAIOAuthScope([]Account{{
		Extra: map[string]any{
			"codex_5h_used_percent":   95.0,
			"codex_5h_reset_at":       now.Add(30 * time.Minute).Format(time.RFC3339),
			"codex_5h_window_minutes": 300,
		},
	}}, nil, "all", now)
	require.NotEmpty(t, scope.Windows[0].Alert)
}

func TestOpenAIOAuthAggregateAlertRequiresBothWindows(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(2 * time.Hour)
	reset7d := now.Add(5 * 24 * time.Hour)
	scope := buildOpenAIOAuthScope([]Account{{
		Extra: map[string]any{
			"codex_5h_used_percent":   90.0,
			"codex_5h_reset_at":       reset5h.Format(time.RFC3339),
			"codex_5h_window_minutes": 300,
			"codex_7d_used_percent":   10.0,
			"codex_7d_reset_at":       reset7d.Format(time.RFC3339),
			"codex_7d_window_minutes": 10080,
		},
	}}, nil, "all", now)
	require.Equal(t, "", scope.Windows[0].Alert)
	require.Equal(t, "", scope.Windows[1].Alert)
	require.Equal(t, "", scope.Alert)
}

func TestOpenAIOAuthRateLimitBuckets(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	in20m := now.Add(20 * time.Minute)
	in2d := now.Add(50 * time.Hour)
	buckets := openAIOAuthRateLimitBuckets([]Account{
		{RateLimitResetAt: &in20m},
		{RateLimitResetAt: &in2d},
		{},
	}, now)
	require.Equal(t, 2, buckets.Total)
	require.Equal(t, 1, buckets.From10To30)
	require.Equal(t, 1, buckets.From1To3Days)
}
