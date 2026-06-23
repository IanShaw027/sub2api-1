package http2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTP2FingerprintStableAndSensitiveToValueChanges(t *testing.T) {
	settingsA := map[uint16]uint32{
		1: 4096,
		3: 100,
		4: 65535,
	}
	settingsB := map[uint16]uint32{
		4: 65535,
		1: 4096,
		3: 100,
	}
	settingsC := map[uint16]uint32{
		1: 4096,
		3: 200,
		4: 65535,
	}

	require.Equal(t, FingerprintFromSettings(settingsA), FingerprintFromSettings(settingsB))
	require.NotEqual(t, FingerprintFromSettings(settingsA), FingerprintFromSettings(settingsC))
}
