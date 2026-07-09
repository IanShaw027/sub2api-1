package http2

import (
	"testing"

	"github.com/stretchr/testify/require"
	xhttp2 "golang.org/x/net/http2"
)

func TestHTTP2FingerprintFromSettingsMapIsCanonicalizedAndSensitiveToValueChanges(t *testing.T) {
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

	require.Equal(t, FingerprintFromSettings(settingsA), FingerprintFromSettings(settingsB), "map helper intentionally canonicalizes by setting id because wire order is unavailable")
	require.NotEqual(t, FingerprintFromSettings(settingsA), FingerprintFromSettings(settingsC))
}

func TestHTTP2FingerprintPreservesSettingsOrderAndExtraSignals(t *testing.T) {
	fingerprint := Fingerprint(
		[]xhttp2.Setting{
			{ID: xhttp2.SettingInitialWindowSize, Val: 65535},
			{ID: xhttp2.SettingHeaderTableSize, Val: 4096},
			{ID: xhttp2.SettingMaxConcurrentStreams, Val: 100},
		},
		[]uint32{12345, 65535},
		[]PriorityFrame{
			{StreamID: 1, Dependency: 0, Exclusive: true, Weight: 200},
			{StreamID: 3, Dependency: 1, Exclusive: false, Weight: 16},
		},
		[]string{":method", ":scheme", ":authority", ":path"},
	)

	require.Equal(t, "4:65535,1:4096,3:100|wu:12345,65535|p:1:0:1:200,3:1:0:16|ph::method,:scheme,:authority,:path", fingerprint)
}

func TestValidateFingerprint(t *testing.T) {
	tests := []struct {
		name        string
		fingerprint string
		wantErr     string
	}{
		{
			name:        "canonical fingerprint",
			fingerprint: "4:65535,1:4096,3:100|wu:12345|p:1:0:1:200|ph::method,:scheme,:authority,:path",
		},
		{
			name:        "settings only",
			fingerprint: "1:4096,3:100,4:65535",
		},
		{
			name:        "empty window update list",
			fingerprint: "1:4096|wu:",
			wantErr:     "invalid window update segment",
		},
		{
			name:        "duplicate pseudo header segment",
			fingerprint: "1:4096|ph::method,:scheme|ph::authority,:path",
			wantErr:     "duplicate pseudo header segment",
		},
		{
			name:        "invalid priority exclusive flag",
			fingerprint: "1:4096|p:1:0:2:200|ph::method,:scheme,:authority,:path",
			wantErr:     "exclusive flag must be 0 or 1",
		},
		{
			name:        "priority weight out of range",
			fingerprint: "1:4096|p:1:0:1:300|ph::method,:scheme,:authority,:path",
			wantErr:     "weight must be <= 255",
		},
		{
			name:        "setting value out of range",
			fingerprint: "1:4294967296",
			wantErr:     "setting value must be <= 4294967295",
		},
		{
			name:        "window update out of range",
			fingerprint: "1:4096|wu:4294967296",
			wantErr:     "window update must be <= 4294967295",
		},
		{
			name:        "pseudo header missing colon",
			fingerprint: "1:4096|ph:method,:scheme,:authority,:path",
			wantErr:     "must start with ':'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFingerprint(tt.fingerprint)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestParseFingerprintRoundTripsStructuredSignals(t *testing.T) {
	raw := "4:65535,1:4096,3:100|wu:12345,65535|p:1:0:1:200,3:1:0:16|ph::method,:scheme,:authority,:path"

	parsed, err := ParseFingerprint(raw)

	require.NoError(t, err)
	require.Equal(t, []xhttp2.Setting{
		{ID: xhttp2.SettingInitialWindowSize, Val: 65535},
		{ID: xhttp2.SettingHeaderTableSize, Val: 4096},
		{ID: xhttp2.SettingMaxConcurrentStreams, Val: 100},
	}, parsed.Settings)
	require.Equal(t, []uint32{12345, 65535}, parsed.ConnWindowUpdates)
	require.Equal(t, []PriorityFrame{
		{StreamID: 1, Dependency: 0, Exclusive: true, Weight: 200},
		{StreamID: 3, Dependency: 1, Exclusive: false, Weight: 16},
	}, parsed.Priorities)
	require.Equal(t, []string{":method", ":scheme", ":authority", ":path"}, parsed.PseudoHeaderOrder)
}

func TestParseFingerprintRejectsMalformedSegments(t *testing.T) {
	_, err := ParseFingerprint("1:4096|wu:abc")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid window update segment")
}
