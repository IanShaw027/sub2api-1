//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func validAnthropicBaseline() *AccountDeviceProfile {
	return &AccountDeviceProfile{
		AccountID:          42,
		Revision:           1,
		SchemaVersion:      1,
		Platform:           PlatformAnthropic,
		ClientFamily:       ClientFamilyClaudeCode,
		InstallationID:     "11111111-1111-4111-8111-111111111111",
		DeviceID:           "22222222-2222-4222-8222-222222222222",
		ClientID:           "33333333-3333-4333-8333-333333333333",
		MachineID:          "44444444-4444-4444-8444-444444444444",
		GatewayAccountUUID: "55555555-5555-4555-8555-555555555555",
		SessionNamespace:   "0123456789abcdef0123456789abcdef",
		OSFamily:           "macos",
		Arch:               "arm64",
		Runtime:            "node",
		RuntimeVersion:     "v24.3.0",
		ClientVersion:      "2.1.0",
		TLSProfileID:       nil,
		TransportFamily:    TransportH1,
		ProfilePayload: map[string]any{
			"user_agent":   "claude-cli/2.1.0 (external, cli)",
			"originator":   "claude-code",
			"stainless_os": "MacOS",
		},
		LearnedFrom:     LearnedFromBaseline,
		LearningEnabled: false,
	}
}

func TestDefaultClientFamilyMatchesPlatformTable(t *testing.T) {
	t.Parallel()

	require.Equal(t, ClientFamilyClaudeCode, DefaultClientFamily(PlatformAnthropic))
	require.Equal(t, ClientFamilyCodexCLI, DefaultClientFamily(PlatformOpenAI))
	require.Equal(t, ClientFamilyGrokCLI, DefaultClientFamily(PlatformGrok))
	require.Equal(t, ClientFamilyKiroIDE, DefaultClientFamily(PlatformKiro))
	require.Equal(t, ClientFamilyGeminiCLI, DefaultClientFamily(PlatformGemini))
	require.Equal(t, ClientFamilyAntigravity, DefaultClientFamily(PlatformAntigravity))
	require.Empty(t, DefaultClientFamily("unknown"))
}

func TestValidateAccountDeviceProfileAcceptsAnthropicBaseline(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateAccountDeviceProfile(validAnthropicBaseline()))
}

func TestValidateAccountDeviceProfileRejectsTLSMinusOne(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	id := int64(-1)
	p.TLSProfileID = &id

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "tls_profile_id")
}

func TestValidateAccountDeviceProfileRejectsUnknownPayloadKey(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.ProfilePayload["evil_header"] = "x"

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "profile_payload")
}

func TestValidateAccountDeviceProfileRejectsCTLInStrings(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.DeviceID = "device\x00id"

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "CTL")
}

func TestValidateAccountDeviceProfileRejectsLocalDevBuildVersions(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"2.1.0-local", "2.1.0-dev", "2.1.0+build"} {
		p := validAnthropicBaseline()
		p.ClientVersion = version
		err := ValidateAccountDeviceProfile(p)
		require.Error(t, err, version)
		require.ErrorContains(t, err, "client_version")
	}

	p := validAnthropicBaseline()
	p.RuntimeVersion = "v24.3.0-local"
	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "runtime_version")
}

func TestValidateAccountDeviceProfileRejectsMismatchedClientFamily(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.ClientFamily = ClientFamilyCodexCLI

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "client_family")
}

func TestValidateAccountDeviceProfileRejectsInvalidSessionNamespace(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.SessionNamespace = "not-hex"

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "session_namespace")
}

func TestValidateAccountDeviceProfileRejectsUnknownOSOnWrite(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.OSFamily = "plan9"

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "os_family")
}

func TestValidateOutboundBundleAcceptsUnknownOSArchRuntime(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.OSFamily = "plan9"
	p.Arch = "riscv64"
	p.Runtime = "made-up-runtime"

	require.NoError(t, ValidateOutboundBundle(p))
}

func TestValidateOutboundBundleRejectsIllegalFields(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	id := int64(-1)
	p.TLSProfileID = &id
	require.Error(t, ValidateOutboundBundle(p))

	p = validAnthropicBaseline()
	p.ProfilePayload["not_whitelisted"] = "x"
	require.Error(t, ValidateOutboundBundle(p))

	p = validAnthropicBaseline()
	p.ClientVersion = "9.9.9-local"
	require.Error(t, ValidateOutboundBundle(p))

	p = validAnthropicBaseline()
	p.InstallationID = "id\nwith\nctl"
	require.Error(t, ValidateOutboundBundle(p))
}

func TestValidateAccountDeviceProfileRejectsOversizedUserAgent(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.ProfilePayload["user_agent"] = strings.Repeat("a", 257)

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "user_agent")
}

func TestValidateAccountDeviceProfileRejectsUnknownPlatform(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.Platform = PlatformComposite
	p.ClientFamily = DefaultClientFamily(PlatformComposite)

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "platform")
}

func TestValidateAccountDeviceProfileRejectsPaddedCanonicalStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		mut  func(*AccountDeviceProfile)
		want string
	}{
		{
			name: "padded session_namespace",
			mut:  func(p *AccountDeviceProfile) { p.SessionNamespace = " 0123456789abcdef0123456789abcdef" },
			want: "session_namespace",
		},
		{
			name: "padded platform",
			mut:  func(p *AccountDeviceProfile) { p.Platform = " anthropic" },
			want: "platform",
		},
		{
			name: "padded client_family",
			mut:  func(p *AccountDeviceProfile) { p.ClientFamily = "claude-code " },
			want: "client_family",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := validAnthropicBaseline()
			tc.mut(p)
			err := ValidateAccountDeviceProfile(p)
			require.Error(t, err)
			require.ErrorContains(t, err, "identity_reject")
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestValidateAccountDeviceProfileRejectsUppercasePlatform(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.Platform = "ANTHROPIC"
	p.ClientFamily = ClientFamilyClaudeCode

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.ErrorContains(t, err, "platform")
}

func TestValidateAccountDeviceProfileRejectsPaddedUserAgentKey(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	delete(p.ProfilePayload, "user_agent")
	p.ProfilePayload[" user_agent"] = "claude-cli/2.1.0 (external, cli)"

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.ErrorContains(t, err, "not allowed")
}

func TestValidateAccountDeviceProfileRejectsPaddedUserAgentKeyOver256(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	delete(p.ProfilePayload, "user_agent")
	p.ProfilePayload[" user_agent"] = strings.Repeat("a", 257)

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.NotContains(t, err.Error(), "exceeds 512")
}

func TestValidateAccountDeviceProfileRejectsNestedPayloadCTL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
	}{
		{name: "array", value: []any{"MacOS\n\rX"}},
		{name: "object", value: map[string]any{"v": "\x00"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := validAnthropicBaseline()
			p.ProfilePayload["stainless_os"] = tc.value
			err := ValidateAccountDeviceProfile(p)
			require.Error(t, err)
			require.ErrorContains(t, err, "identity_reject")
		})
	}
}

func TestValidateAccountDeviceProfileRejectsNonStringPayloadValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
	}{
		{name: "array", value: []any{"MacOS"}},
		{name: "object", value: map[string]any{"v": "ok"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := validAnthropicBaseline()
			p.ProfilePayload["stainless_os"] = tc.value
			err := ValidateAccountDeviceProfile(p)
			require.Error(t, err)
			require.ErrorContains(t, err, "identity_reject")
		})
	}
}

func TestValidateAccountDeviceProfileRejectsUppercaseSessionNamespace(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.SessionNamespace = "0123456789ABCDEF0123456789ABCDEF"

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.ErrorContains(t, err, "session_namespace")
}

func TestValidateAccountDeviceProfileRejectsEmptyOSFamilyOnWrite(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.OSFamily = ""

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.ErrorContains(t, err, "os_family")

	require.NoError(t, ValidateOutboundBundle(p))
}

func TestValidateAccountDeviceProfileRejectsRevisionZero(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.Revision = 0

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.ErrorContains(t, err, "revision")
}

func TestValidateAccountDeviceProfileRejectsOversizedID(t *testing.T) {
	t.Parallel()

	p := validAnthropicBaseline()
	p.DeviceID = strings.Repeat("d", 65)

	err := ValidateAccountDeviceProfile(p)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.ErrorContains(t, err, "device_id")
}
