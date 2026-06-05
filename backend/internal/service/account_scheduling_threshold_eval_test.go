//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEvaluateAccountSchedulingThreshold_OpenAIChoosesLatestResetWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	wantUntil := now.Add(72 * time.Hour)
	account := &Account{
		Platform: PlatformOpenAI,
		Extra: map[string]any{
			"codex_5h_used_percent": 90.0,
			"codex_5h_reset_at":     now.Add(2 * time.Hour).Format(time.RFC3339),
			"codex_7d_used_percent": 85.0,
			"codex_7d_reset_at":     wantUntil.Format(time.RFC3339),
		},
	}

	decision := EvaluateAccountSchedulingThreshold(account, map[string]int{
		PlatformOpenAI: 80,
	}, now)

	require.True(t, decision.ShouldPause)
	require.Equal(t, PlatformOpenAI, decision.Platform)
	require.Equal(t, "7d", decision.Window)
	require.Empty(t, decision.Scope)
	require.Equal(t, 85.0, decision.UsedPercent)
	require.NotNil(t, decision.Until)
	require.True(t, wantUntil.Equal(*decision.Until))
}

func TestEvaluateAccountSchedulingThreshold_AnthropicIgnoresExpiredFiveHourWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	expiredEnd := now.Add(-30 * time.Minute)
	wantUntil := now.Add(5 * 24 * time.Hour)
	account := &Account{
		Platform:         PlatformAnthropic,
		SessionWindowEnd: &expiredEnd,
		Extra: map[string]any{
			"session_window_utilization":   0.99,
			"passive_usage_7d_utilization": 0.82,
			"passive_usage_7d_reset":       float64(wantUntil.Unix()),
		},
	}

	decision := EvaluateAccountSchedulingThreshold(account, map[string]int{
		PlatformAnthropic: 80,
	}, now)

	require.True(t, decision.ShouldPause)
	require.Equal(t, PlatformAnthropic, decision.Platform)
	require.Equal(t, "7d", decision.Window)
	require.Empty(t, decision.Scope)
	require.Equal(t, 82.0, decision.UsedPercent)
	require.NotNil(t, decision.Until)
	require.True(t, wantUntil.Equal(*decision.Until))
}

func TestEvaluateAccountSchedulingThreshold_GeminiUsesHighestUtilizationBucket(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	wantUntil := now.Add(4 * time.Hour)
	account := &Account{
		Platform: PlatformGemini,
		Extra: map[string]any{
			"gemini_usage_raw": map[string]any{
				"buckets": []any{
					map[string]any{
						"modelId":           "gemini-2.5-pro",
						"remainingFraction": 0.30,
						"resetTime":         now.Add(2 * time.Hour).Format(time.RFC3339),
					},
					map[string]any{
						"modelId":           "gemini-2.5-flash",
						"remainingFraction": 0.10,
						"resetTime":         wantUntil.Format(time.RFC3339),
					},
				},
			},
		},
	}

	decision := EvaluateAccountSchedulingThreshold(account, map[string]int{
		PlatformGemini: 80,
	}, now)

	require.True(t, decision.ShouldPause)
	require.Equal(t, PlatformGemini, decision.Platform)
	require.Equal(t, "daily", decision.Window)
	require.Equal(t, "flash", decision.Scope)
	require.Equal(t, 90.0, decision.UsedPercent)
	require.NotNil(t, decision.Until)
	require.True(t, wantUntil.Equal(*decision.Until))
}

func TestEvaluateAccountSchedulingThreshold_KiroPausesWhenThresholdReached(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	wantUntil := now.Add(24 * time.Hour)
	account := &Account{
		Platform: PlatformKiro,
		Extra: map[string]any{
			"kiro_sched_utilization": 0.91,
			"kiro_sched_reset_at":    wantUntil.Format(time.RFC3339),
		},
	}

	decision := EvaluateAccountSchedulingThreshold(account, map[string]int{
		PlatformKiro: 90,
	}, now)

	require.True(t, decision.ShouldPause)
	require.Equal(t, PlatformKiro, decision.Platform)
	require.Equal(t, "quota", decision.Window)
	require.Empty(t, decision.Scope)
	require.Equal(t, 91.0, decision.UsedPercent)
	require.NotNil(t, decision.Until)
	require.True(t, wantUntil.Equal(*decision.Until))
}

func TestEvaluateAccountSchedulingThreshold_KiroThresholdHundredDisablesPlatform(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	account := &Account{
		Platform: PlatformKiro,
		Extra: map[string]any{
			"kiro_sched_utilization": 0.99,
			"kiro_sched_reset_at":    now.Add(24 * time.Hour).Format(time.RFC3339),
		},
	}

	decision := EvaluateAccountSchedulingThreshold(account, map[string]int{
		PlatformKiro: 100,
	}, now)

	require.False(t, decision.ShouldPause)
}

func TestEvaluateAccountSchedulingThreshold_AntigravityCarriesScope(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	wantUntil := now.Add(48 * time.Hour)
	account := &Account{
		Platform: PlatformAntigravity,
		Extra: map[string]any{
			"antigravity_sched_utilization": 0.92,
			"antigravity_sched_reset_at":    wantUntil.Format(time.RFC3339),
			"antigravity_sched_scope":       "gemini",
		},
	}

	decision := EvaluateAccountSchedulingThreshold(account, map[string]int{
		PlatformAntigravity: 90,
	}, now)

	require.True(t, decision.ShouldPause)
	require.Equal(t, PlatformAntigravity, decision.Platform)
	require.Equal(t, "quota", decision.Window)
	require.Equal(t, "gemini", decision.Scope)
	require.Equal(t, 92.0, decision.UsedPercent)
	require.NotNil(t, decision.Until)
	require.True(t, wantUntil.Equal(*decision.Until))
}
