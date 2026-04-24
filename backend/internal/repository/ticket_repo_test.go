//go:build unit

package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateTicketNo_FormatAndUniqueness(t *testing.T) {
	now := time.Unix(1777000000, 0)
	seen := make(map[string]struct{}, 512)

	for range 512 {
		ticketNo := generateTicketNo(now)
		require.Len(t, ticketNo, 32)
		require.True(t, strings.HasPrefix(ticketNo, "TK"+now.Format("20060102150405")))
		_, exists := seen[ticketNo]
		require.False(t, exists)
		seen[ticketNo] = struct{}{}
	}
}
