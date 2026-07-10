//go:build unit

package service

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

// TestParseCodexRateLimitHeaders_ResetHeaderDualCompat covers the reset-after /
// reset-at dual-header compatibility added to mirror the current upstream Codex
// rate-limit header family.
func TestParseCodexRateLimitHeaders_ResetHeaderDualCompat(t *testing.T) {
	require.NoError(t, timezone.Init("UTC"))

	t.Run("only reset-after-seconds (legacy relative)", func(t *testing.T) {
		headers := http.Header{
			"X-Codex-Primary-Used-Percent":          []string{"10"},
			"X-Codex-Primary-Reset-After-Seconds":   []string{"3600"},
			"X-Codex-Secondary-Used-Percent":        []string{"5"},
			"X-Codex-Secondary-Reset-After-Seconds": []string{"600"},
		}
		snapshot := ParseCodexRateLimitHeaders(headers)
		require.NotNil(t, snapshot)
		require.NotNil(t, snapshot.PrimaryResetAfterSeconds)
		require.Equal(t, 3600, *snapshot.PrimaryResetAfterSeconds)
		require.NotNil(t, snapshot.SecondaryResetAfterSeconds)
		require.Equal(t, 600, *snapshot.SecondaryResetAfterSeconds)
	})

	t.Run("only reset-at (absolute unix seconds) converts to relative", func(t *testing.T) {
		now := currentOpenAICodexSnapshotTime()
		primaryAt := now.Add(2 * time.Hour).Unix()
		secondaryAt := now.Add(30 * time.Minute).Unix()
		headers := http.Header{
			"X-Codex-Primary-Used-Percent":   []string{"10"},
			"X-Codex-Primary-Reset-At":       []string{itoa64(primaryAt)},
			"X-Codex-Secondary-Used-Percent": []string{"5"},
			"X-Codex-Secondary-Reset-At":     []string{itoa64(secondaryAt)},
		}
		snapshot := ParseCodexRateLimitHeaders(headers)
		require.NotNil(t, snapshot)
		require.NotNil(t, snapshot.PrimaryResetAfterSeconds)
		// Allow a 2s slack for the clock between header build and parse.
		require.InDelta(t, 7200, *snapshot.PrimaryResetAfterSeconds, 2)
		require.NotNil(t, snapshot.SecondaryResetAfterSeconds)
		require.InDelta(t, 1800, *snapshot.SecondaryResetAfterSeconds, 2)
	})

	t.Run("reset-after-seconds wins when both present", func(t *testing.T) {
		now := currentOpenAICodexSnapshotTime()
		headers := http.Header{
			"X-Codex-Primary-Used-Percent":        []string{"10"},
			"X-Codex-Primary-Reset-After-Seconds": []string{"111"},
			"X-Codex-Primary-Reset-At":            []string{itoa64(now.Add(9 * time.Hour).Unix())},
		}
		snapshot := ParseCodexRateLimitHeaders(headers)
		require.NotNil(t, snapshot)
		require.NotNil(t, snapshot.PrimaryResetAfterSeconds)
		require.Equal(t, 111, *snapshot.PrimaryResetAfterSeconds)
	})

	t.Run("reset-at in the past clamps to zero", func(t *testing.T) {
		now := currentOpenAICodexSnapshotTime()
		headers := http.Header{
			"X-Codex-Primary-Used-Percent": []string{"10"},
			"X-Codex-Primary-Reset-At":     []string{itoa64(now.Add(-1 * time.Hour).Unix())},
		}
		snapshot := ParseCodexRateLimitHeaders(headers)
		require.NotNil(t, snapshot)
		require.NotNil(t, snapshot.PrimaryResetAfterSeconds)
		require.Equal(t, 0, *snapshot.PrimaryResetAfterSeconds)
	})
}

func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}
