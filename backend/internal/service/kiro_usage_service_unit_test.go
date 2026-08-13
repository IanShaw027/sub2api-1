//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKiroUsageUnixFloatToTime_NormalizesMillisecondEpoch(t *testing.T) {
	seconds := float64(1783296000)
	milliseconds := seconds * 1000
	expected := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)

	require.Equal(t, expected, unixFloatToTime(seconds))
	require.Equal(t, expected, unixFloatToTime(milliseconds))
}
