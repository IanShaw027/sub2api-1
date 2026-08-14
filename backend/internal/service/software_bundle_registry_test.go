package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestCompareSemverOrdersReleaseVersions(t *testing.T) {
	require.Positive(t, CompareSemver("2.1.0", "2.0.9"))
	require.Negative(t, CompareSemver("2.0.9", "2.1.0"))
	require.Zero(t, CompareSemver("2.1.0", "2.1.0"))
}

func TestCompareSemverOrdersPrereleaseBeforeRelease(t *testing.T) {
	require.Negative(t, CompareSemver("2.1.0-alpha.1", "2.1.0"))
	require.Positive(t, CompareSemver("2.1.0", "2.1.0-rc.1"))
	require.Negative(t, CompareSemver("2.1.0-alpha.1", "2.1.0-alpha.2"))
	require.Negative(t, CompareSemver("2.1.0-alpha.2", "2.1.0-beta.1"))
	require.Negative(t, CompareSemver("2.1.0-rc.1", "2.1.0"))
}

func TestCompareSemverDoesNotUseNumericOnlyCompareVersions(t *testing.T) {
	// CompareVersions only compares major.minor.patch and treats prerelease as equal.
	require.Zero(t, CompareVersions("2.1.0-rc.1", "2.1.0"))
	require.Negative(t, CompareSemver("2.1.0-rc.1", "2.1.0"))
}

func TestSoftwareBundleRegistryLookupMissesUnknownHighVersion(t *testing.T) {
	reg := NewSoftwareBundleRegistry()

	_, ok := reg.Lookup(domain.PlatformAnthropic, "claude-code", "999.0.0")
	require.False(t, ok)

	_, ok = reg.Lookup(domain.PlatformOpenAI, "codex-cli", "9.9.9")
	require.False(t, ok)
}

func TestSoftwareBundleRegistryLookupHitsCompileTimeBaselines(t *testing.T) {
	reg := NewSoftwareBundleRegistry()

	cases := []struct {
		platform string
		family   string
		version  string
	}{
		{domain.PlatformAnthropic, "claude-code", claude.CLICurrentVersion},
		{domain.PlatformOpenAI, "codex-cli", NormalizeCodexClientVersion(codexCLIVersion)},
		{domain.PlatformGrok, "grok-cli", xai.CLIClientVersion},
		{domain.PlatformKiro, "kiro-ide", defaultKiroVersion},
		{domain.PlatformGemini, "gemini-cli", "0.1.5"},
		{domain.PlatformAntigravity, "antigravity", antigravity.DefaultUserAgentVersion},
	}

	for _, tc := range cases {
		t.Run(tc.family, func(t *testing.T) {
			bundle, ok := reg.Lookup(tc.platform, tc.family, tc.version)
			require.True(t, ok, "expected compile-time row for %s %s %s", tc.platform, tc.family, tc.version)
			require.Equal(t, tc.platform, bundle.Platform)
			require.Equal(t, tc.family, bundle.ClientFamily)
			require.Equal(t, tc.version, bundle.ClientVersion)
			require.NoError(t, ValidateSoftwareBundle(bundle))
		})
	}
}

func TestSoftwareBundleRegistryCodexBaselineMeetsUpstreamFloor(t *testing.T) {
	reg := NewSoftwareBundleRegistry()
	version := NormalizeCodexClientVersion(codexCLIVersion)
	require.NotEmpty(t, version)
	require.GreaterOrEqual(t, CompareSemver(version, codexUpstreamMinVersion), 0)

	bundle, ok := reg.Lookup(domain.PlatformOpenAI, "codex-cli", version)
	require.True(t, ok)
	require.Equal(t, openai.CodexDefaultOriginator, bundle.Originator)
	require.Equal(t, version, openai.CodexUserAgentVersion(bundle.UserAgent))
	require.NoError(t, ValidateSoftwareBundle(bundle))
}

func TestValidateSoftwareBundleRejectsCodexIdentityMismatch(t *testing.T) {
	version := NormalizeCodexClientVersion(codexCLIVersion)
	require.NotEmpty(t, version)
	matchedUA := buildCodexCLIUserAgent(version)

	t.Run("originator does not pair with user-agent", func(t *testing.T) {
		err := ValidateSoftwareBundle(SoftwareBundle{
			Platform:      domain.PlatformOpenAI,
			ClientFamily:  "codex-cli",
			ClientVersion: version,
			UserAgent:     matchedUA,
			Originator:    openai.CodexCLIOriginator,
		})
		require.Error(t, err)
	})

	t.Run("client version does not match user-agent version", func(t *testing.T) {
		err := ValidateSoftwareBundle(SoftwareBundle{
			Platform:      domain.PlatformOpenAI,
			ClientFamily:  "codex-cli",
			ClientVersion: "0.200.0",
			UserAgent:     matchedUA,
			Originator:    openai.CodexDefaultOriginator,
		})
		require.Error(t, err)
	})

	t.Run("trailer-official leading-unofficial user-agent is rewritten", func(t *testing.T) {
		err := ValidateSoftwareBundle(SoftwareBundle{
			Platform:      domain.PlatformOpenAI,
			ClientFamily:  "codex-cli",
			ClientVersion: "0.146.0",
			UserAgent:     "cccc/0.146.0 (Ubuntu 22.4.0; x86_64) (codex-tui; 0.146.0)",
			Originator:    "codex-tui",
		})
		require.Error(t, err)
	})
}

func TestValidateSoftwareBundleAcceptsMatchedCodexIdentity(t *testing.T) {
	version := NormalizeCodexClientVersion(codexCLIVersion)
	require.NotEmpty(t, version)

	err := ValidateSoftwareBundle(SoftwareBundle{
		Platform:      domain.PlatformOpenAI,
		ClientFamily:  "codex-cli",
		ClientVersion: version,
		UserAgent:     buildCodexCLIUserAgent(version),
		Originator:    openai.CodexDefaultOriginator,
	})
	require.NoError(t, err)
}

func TestSoftwareBundleRegistryGeminiBaselineUsesOfficialUserAgent(t *testing.T) {
	reg := NewSoftwareBundleRegistry()
	bundle, ok := reg.Lookup(domain.PlatformGemini, "gemini-cli", "0.1.5")
	require.True(t, ok)
	require.Equal(t, geminicli.GeminiCLIUserAgent, bundle.UserAgent)
}

func TestSoftwareBundleRegistryGrokBaselineUsesSupportedCLIVersion(t *testing.T) {
	require.True(t, xai.IsSupportedCLIVersion(xai.CLIClientVersion))

	reg := NewSoftwareBundleRegistry()
	bundle, ok := reg.Lookup(domain.PlatformGrok, "grok-cli", xai.CLIClientVersion)
	require.True(t, ok)
	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), bundle.UserAgent)
	require.NoError(t, ValidateSoftwareBundle(bundle))
}

func TestSoftwareBundleRegistryKiroBaselineLeavesUserAgentEmpty(t *testing.T) {
	reg := NewSoftwareBundleRegistry()
	bundle, ok := reg.Lookup(domain.PlatformKiro, "kiro-ide", defaultKiroVersion)
	require.True(t, ok)
	require.Empty(t, bundle.UserAgent)
	require.Equal(t, defaultKiroSystemVersion, bundle.Payload["kiro_system_version"])
	require.Equal(t, defaultKiroNodeVersion, bundle.Payload["kiro_node_version"])
	require.NoError(t, ValidateSoftwareBundle(bundle))
}

func TestSoftwareBundleRegistryAntigravityBaselineUsesBuildUserAgent(t *testing.T) {
	reg := NewSoftwareBundleRegistry()
	bundle, ok := reg.Lookup(domain.PlatformAntigravity, "antigravity", antigravity.DefaultUserAgentVersion)
	require.True(t, ok)
	require.Equal(t, antigravity.BuildUserAgent(antigravity.DefaultUserAgentVersion), bundle.UserAgent)
	require.NoError(t, ValidateSoftwareBundle(bundle))
}
