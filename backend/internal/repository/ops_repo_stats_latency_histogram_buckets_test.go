//go:build unit

package repository

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLatencyHistogramBuckets_AreConsistent(t *testing.T) {
	require.NotEmpty(t, latencyHistogramBuckets)
	require.Equal(t, len(latencyHistogramBuckets), len(latencyHistogramOrderedRanges))

	// Ordered ranges should match bucket labels, in-order.
	for i, b := range latencyHistogramBuckets {
		require.Equal(t, b.label, latencyHistogramOrderedRanges[i])
	}

	// Default bucket should be last and non-empty.
	last := latencyHistogramBuckets[len(latencyHistogramBuckets)-1]
	require.NotZero(t, last.label)
	require.Zero(t, last.upperMs)

	// CASE expressions should include all labels.
	rangeCase := latencyHistogramRangeCaseExpr("duration_ms")
	orderCase := latencyHistogramRangeOrderCaseExpr("duration_ms")
	for _, b := range latencyHistogramBuckets {
		require.True(t, strings.Contains(rangeCase, "'"+b.label+"'"), "range case missing label %q", b.label)
	}
	require.Contains(t, rangeCase, "duration_ms")
	require.Contains(t, orderCase, "duration_ms")
}
