//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIWSLiveRelayResponseSetEvictsOldest(t *testing.T) {
	ids := newOpenAIWSLiveRelayResponseSet(2)

	ids.Remember("resp_1")
	ids.Remember("resp_2")
	require.True(t, ids.Contains("resp_1"))
	require.True(t, ids.Contains("resp_2"))

	ids.Remember("resp_3")
	require.False(t, ids.Contains("resp_1"))
	require.True(t, ids.Contains("resp_2"))
	require.True(t, ids.Contains("resp_3"))

	ids.Remember("resp_3")
	ids.Remember("resp_4")
	require.False(t, ids.Contains("resp_2"))
	require.True(t, ids.Contains("resp_3"))
	require.True(t, ids.Contains("resp_4"))
}
