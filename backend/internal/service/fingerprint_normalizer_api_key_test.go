package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFingerprintNormalizer_ResolveCanonical_SkipsAPIKeyAccounts(t *testing.T) {
	n := NewFingerprintNormalizer(nil, nil, nil, nil, nil)

	canonical := n.ResolveCanonical(context.Background(), &Account{
		ID:       71716,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
	}, "claude-cli/1.0.0")

	require.Nil(t, canonical)
}

func TestFingerprintNormalizer_ApplyToRequest_SkipsAPIKeyAccounts(t *testing.T) {
	n := NewFingerprintNormalizer(nil, nil, nil, nil, nil)
	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)

	req, newBody, err := n.ApplyToRequest(nil, body, n.ResolveCanonical(context.Background(), &Account{
		ID:       71716,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
	}, "claude-cli/1.0.0"))

	require.NoError(t, err)
	require.Nil(t, req)
	require.Equal(t, body, newBody)
}

func TestFingerprintNormalizer_AntiBanGlobalDisabledOverridesPlatformToggle(t *testing.T) {
	n := NewFingerprintNormalizer(nil, nil, nil, &CanonicalFingerprintConfig{
		Enabled:             true,
		AntiBanEnabled:      false,
		EnabledByPlatform:   map[string]bool{"anthropic": true},
		SpoofProcessMetrics: true,
		TelemetryPaths:      []string{"telemetry"},
	}, nil)
	body := []byte(`{"metadata":{"telemetry":"leak"},"messages":[{"role":"user","content":"hi"}]}`)

	_, newBody, err := n.ApplyToRequest(nil, body, &CanonicalFingerprint{
		Platform: "anthropic",
		DeviceID: "device",
	})

	require.NoError(t, err)
	require.Equal(t, body, newBody)
}
