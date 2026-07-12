package service

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

const (
	openAICodexProbeMinVersion      = "0.144.0"
	openAICodexProbePromotedVersion = "0.144.1"
)

func normalizeOpenAICodexProbeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || CompareVersions(version, openAICodexProbeMinVersion) < 0 {
		return openAICodexProbePromotedVersion
	}
	return version
}

func applyOpenAICodexProbeIdentityHeaders(header http.Header, account *Account, profile *tlsfingerprint.Profile) {
	if header == nil {
		return
	}
	userAgent := resolveOpenAICodexProbeUserAgent(account, profile)
	originator := resolveOpenAICodexProbeOriginator(userAgent, profile)
	header.Set("Originator", originator)
	header.Set("User-Agent", userAgent)
	header.Set("Version", normalizeOpenAICodexProbeVersion(codexCLIVersion))
	enforceCodexIdentityHeaders(header)
}

func resolveOpenAICodexProbeUserAgent(account *Account, profile *tlsfingerprint.Profile) string {
	if profile != nil {
		if ua := strings.TrimSpace(profile.UserAgent); ua != "" {
			return ua
		}
	}
	if account != nil {
		if ua := strings.TrimSpace(account.GetOpenAIUserAgent()); ua != "" {
			return ua
		}
	}
	return codexCLIUserAgent
}

func resolveOpenAICodexProbeOriginator(userAgent string, profile *tlsfingerprint.Profile) string {
	if profile != nil {
		if originator := strings.TrimSpace(profile.Originator); originator != "" {
			return originator
		}
	}
	ua := strings.TrimSpace(userAgent)
	uaLower := strings.ToLower(ua)
	switch {
	case strings.HasPrefix(uaLower, "codex-tui/") || strings.Contains(uaLower, "(codex-tui;"):
		return "codex-tui"
	case strings.HasPrefix(uaLower, "codex_cli_rs/"):
		return codexDefaultOriginator
	case strings.HasPrefix(uaLower, "codex_vscode_copilot/"):
		return "codex_vscode_copilot"
	case strings.HasPrefix(uaLower, "codex_vscode/"):
		return "codex_vscode"
	case strings.HasPrefix(uaLower, "codex_exec/"):
		return "codex_exec"
	case strings.HasPrefix(uaLower, "codex_sdk_ts/"):
		return "codex_sdk_ts"
	case strings.HasPrefix(uaLower, "codex desktop/"):
		return "Codex Desktop"
	case strings.HasPrefix(uaLower, "codex chatgpt desktop/"):
		return "codex_chatgpt_desktop"
	case strings.HasPrefix(uaLower, "codex atlas/"):
		return "codex_atlas"
	default:
		return codexDefaultOriginator
	}
}
