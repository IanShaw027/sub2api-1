package service

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"golang.org/x/mod/semver"
)

const (
	softwareFamilyClaudeCode  = "claude-code"
	softwareFamilyCodexCLI    = "codex-cli"
	softwareFamilyGrokCLI     = "grok-cli"
	softwareFamilyKiroIDE     = "kiro-ide"
	softwareFamilyGeminiCLI   = "gemini-cli"
	softwareFamilyAntigravity = "antigravity"
)

// SoftwareBundle is a complete official-client software identity for one
// platform + family + version. Learning may replace a profile's software
// fields only with a registry-validated bundle.
type SoftwareBundle struct {
	Platform       string
	ClientFamily   string
	ClientVersion  string
	UserAgent      string
	Originator     string
	Runtime        string
	RuntimeVersion string
	Payload        map[string]any
}

type softwareBundleKey struct {
	platform string
	family   string
	version  string
}

// SoftwareBundleRegistry is a compile-time table of official software bundles.
// Panel-published versions can be empty in P1; unknown high versions miss.
type SoftwareBundleRegistry struct {
	bundles map[softwareBundleKey]SoftwareBundle
}

var softwareBundleFamilyByPlatform = map[string]string{
	domain.PlatformAnthropic:   softwareFamilyClaudeCode,
	domain.PlatformOpenAI:      softwareFamilyCodexCLI,
	domain.PlatformGrok:        softwareFamilyGrokCLI,
	domain.PlatformKiro:        softwareFamilyKiroIDE,
	domain.PlatformGemini:      softwareFamilyGeminiCLI,
	domain.PlatformAntigravity: softwareFamilyAntigravity,
}

// CompareSemver compares two semver strings, including prerelease ordering.
// It does not call CompareVersions, which only compares numeric major.minor.patch.
func CompareSemver(a, b string) int {
	return semver.Compare(canonicalSemver(a), canonicalSemver(b))
}

func canonicalSemver(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if version[0] == 'v' || version[0] == 'V' {
		return "v" + version[1:]
	}
	return "v" + version
}

// NewSoftwareBundleRegistry returns a registry seeded with compile-time official floors.
func NewSoftwareBundleRegistry() *SoftwareBundleRegistry {
	rows := compileTimeSoftwareBundles()
	reg := &SoftwareBundleRegistry{bundles: make(map[softwareBundleKey]SoftwareBundle, len(rows))}
	for _, bundle := range rows {
		reg.bundles[softwareBundleKey{
			platform: bundle.Platform,
			family:   bundle.ClientFamily,
			version:  bundle.ClientVersion,
		}] = bundle
	}
	return reg
}

// Lookup returns the exact platform+family+version row.
// A high version that is not in the table misses even if it looks official.
func (r *SoftwareBundleRegistry) Lookup(platform, family, version string) (SoftwareBundle, bool) {
	if r == nil {
		return SoftwareBundle{}, false
	}
	bundle, ok := r.bundles[softwareBundleKey{
		platform: strings.TrimSpace(platform),
		family:   strings.TrimSpace(family),
		version:  strings.TrimSpace(version),
	}]
	if !ok {
		return SoftwareBundle{}, false
	}
	return cloneSoftwareBundle(bundle), true
}

// ValidateSoftwareBundle rejects an internally inconsistent software package.
// Codex requires User-Agent, originator, and version to pair, and the version
// must pass NormalizeCodexClientVersion at or above the upstream floor.
func ValidateSoftwareBundle(b SoftwareBundle) error {
	b.Platform = strings.TrimSpace(b.Platform)
	b.ClientFamily = strings.TrimSpace(b.ClientFamily)
	b.ClientVersion = strings.TrimSpace(b.ClientVersion)
	b.UserAgent = strings.TrimSpace(b.UserAgent)
	b.Originator = strings.TrimSpace(b.Originator)
	b.Runtime = strings.TrimSpace(b.Runtime)
	b.RuntimeVersion = strings.TrimSpace(b.RuntimeVersion)

	if b.Platform == "" || b.ClientFamily == "" || b.ClientVersion == "" {
		return fmt.Errorf("software bundle platform, client_family, and client_version are required")
	}
	if wantFamily, ok := softwareBundleFamilyByPlatform[b.Platform]; !ok || wantFamily != b.ClientFamily {
		return fmt.Errorf("software bundle client_family %q does not match platform %q", b.ClientFamily, b.Platform)
	}
	if err := validateSoftwareBundleVersion(b.ClientVersion); err != nil {
		return fmt.Errorf("software bundle client_version: %w", err)
	}
	if err := validateSoftwareBundleVersion(b.RuntimeVersion); err != nil {
		return fmt.Errorf("software bundle runtime_version: %w", err)
	}
	if err := validateSoftwareBundleUserAgent(b.UserAgent); err != nil {
		return err
	}
	if uaVersion := softwareBundleVersionFromUA(b.UserAgent); uaVersion != "" && uaVersion != b.ClientVersion {
		return fmt.Errorf("software bundle user-agent version %q does not match client_version %q", uaVersion, b.ClientVersion)
	}

	switch b.ClientFamily {
	case softwareFamilyCodexCLI:
		return validateCodexSoftwareBundle(b)
	case softwareFamilyGrokCLI:
		if !xai.IsSupportedCLIVersion(b.ClientVersion) {
			return fmt.Errorf("software bundle grok-cli version %q is below the supported CLI floor", b.ClientVersion)
		}
	}
	return nil
}

func validateCodexSoftwareBundle(b SoftwareBundle) error {
	version := NormalizeCodexClientVersion(b.ClientVersion)
	if version == "" {
		return fmt.Errorf("software bundle codex-cli version %q is not a valid Codex client version", b.ClientVersion)
	}
	if CompareSemver(version, codexUpstreamMinVersion) < 0 {
		return fmt.Errorf("software bundle codex-cli version %q is below the upstream floor %s", version, codexUpstreamMinVersion)
	}
	originator, _, ok := openai.PairCodexClientIdentity(b.UserAgent)
	if !ok {
		return fmt.Errorf("software bundle codex-cli user-agent is not an official paired identity")
	}
	if b.Originator != originator {
		return fmt.Errorf("software bundle codex-cli originator %q does not pair with user-agent", b.Originator)
	}
	if openai.CodexUserAgentVersion(b.UserAgent) != version {
		return fmt.Errorf("software bundle codex-cli user-agent version does not match client_version %q", version)
	}
	return nil
}

func validateSoftwareBundleVersion(version string) error {
	if version == "" {
		return nil
	}
	if len(version) > 64 {
		return fmt.Errorf("exceeds 64 characters")
	}
	if containsCTLOrNUL(version) {
		return fmt.Errorf("contains control characters")
	}
	lower := strings.ToLower(version)
	if strings.Contains(lower, "-local") || strings.Contains(lower, "-dev") || strings.Contains(lower, "+build") {
		return fmt.Errorf("rejects -local, -dev, and +build suffixes")
	}
	return nil
}

func validateSoftwareBundleUserAgent(userAgent string) error {
	if userAgent == "" {
		return nil
	}
	if len(userAgent) > 256 {
		return fmt.Errorf("software bundle user-agent exceeds 256 characters")
	}
	if containsCTLOrNUL(userAgent) {
		return fmt.Errorf("software bundle user-agent contains control characters")
	}
	return nil
}

func containsCTLOrNUL(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] == 0x7f {
			return true
		}
	}
	return false
}

func softwareBundleVersionFromUA(userAgent string) string {
	userAgent = strings.TrimSpace(userAgent)
	slash := strings.IndexByte(userAgent, '/')
	if slash <= 0 {
		return ""
	}
	rest := userAgent[slash+1:]
	if space := strings.IndexByte(rest, ' '); space >= 0 {
		rest = rest[:space]
	}
	return strings.TrimSpace(rest)
}

func cloneSoftwareBundle(bundle SoftwareBundle) SoftwareBundle {
	if bundle.Payload == nil {
		return bundle
	}
	payload := make(map[string]any, len(bundle.Payload))
	for key, value := range bundle.Payload {
		payload[key] = value
	}
	bundle.Payload = payload
	return bundle
}

func compileTimeSoftwareBundles() []SoftwareBundle {
	claudeUA := claude.DefaultHeaders["User-Agent"]
	codexVersion := NormalizeCodexClientVersion(codexCLIVersion)
	geminiVersion := softwareBundleVersionFromUA(geminicli.GeminiCLIUserAgent)
	antigravityUA := "antigravity/" + antigravity.DefaultUserAgentVersion + " windows/amd64"

	return []SoftwareBundle{
		{
			Platform:       domain.PlatformAnthropic,
			ClientFamily:   softwareFamilyClaudeCode,
			ClientVersion:  claude.CLICurrentVersion,
			UserAgent:      claudeUA,
			Runtime:        claude.DefaultHeaders["X-Stainless-Runtime"],
			RuntimeVersion: claude.DefaultHeaders["X-Stainless-Runtime-Version"],
			Payload: map[string]any{
				"user_agent":                claudeUA,
				"stainless_lang":            claude.DefaultHeaders["X-Stainless-Lang"],
				"stainless_package_version": claude.DefaultHeaders["X-Stainless-Package-Version"],
				"stainless_os":              claude.DefaultHeaders["X-Stainless-OS"],
				"stainless_arch":            claude.DefaultHeaders["X-Stainless-Arch"],
				"stainless_runtime":         claude.DefaultHeaders["X-Stainless-Runtime"],
				"stainless_runtime_version": claude.DefaultHeaders["X-Stainless-Runtime-Version"],
			},
		},
		{
			Platform:      domain.PlatformOpenAI,
			ClientFamily:  softwareFamilyCodexCLI,
			ClientVersion: codexVersion,
			UserAgent:     buildCodexCLIUserAgent(codexVersion),
			Originator:    openai.CodexDefaultOriginator,
			Payload: map[string]any{
				"user_agent": buildCodexCLIUserAgent(codexVersion),
				"originator": openai.CodexDefaultOriginator,
			},
		},
		{
			Platform:      domain.PlatformGrok,
			ClientFamily:  softwareFamilyGrokCLI,
			ClientVersion: xai.CLIClientVersion,
			UserAgent:     xai.CLIUserAgent(xai.CLIClientVersion),
			Runtime:       xai.CLIClientIdentifier,
			Payload: map[string]any{
				"user_agent":      xai.CLIUserAgent(xai.CLIClientVersion),
				"grok_token_auth": xai.CLITokenAuth,
				"grok_identifier": xai.CLIClientIdentifier,
			},
		},
		{
			Platform:       domain.PlatformKiro,
			ClientFamily:   softwareFamilyKiroIDE,
			ClientVersion:  defaultKiroVersion,
			UserAgent:      "KiroIDE-" + defaultKiroVersion,
			Runtime:        "node",
			RuntimeVersion: defaultKiroNodeVersion,
			Payload: map[string]any{
				"kiro_system_version": defaultKiroSystemVersion,
				"kiro_node_version":   defaultKiroNodeVersion,
			},
		},
		{
			Platform:      domain.PlatformGemini,
			ClientFamily:  softwareFamilyGeminiCLI,
			ClientVersion: geminiVersion,
			UserAgent:     geminicli.GeminiCLIUserAgent,
		},
		{
			Platform:      domain.PlatformAntigravity,
			ClientFamily:  softwareFamilyAntigravity,
			ClientVersion: antigravity.DefaultUserAgentVersion,
			UserAgent:     antigravityUA,
		},
	}
}
