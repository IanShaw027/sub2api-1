//go:build unit

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
)

func TestParseTLSPinProfileName(t *testing.T) {
	got, ok := ParseTLSPinProfileName("pin:claude-code:macos:h1")
	require.True(t, ok)
	require.Equal(t, ParsedTLSPinName{
		ClientFamily: ClientFamilyClaudeCode,
		OSFamily:     "macos",
		Transport:    TransportH1,
	}, got)

	got, ok = ParseTLSPinProfileName("pin:claude-code:macos:h1:node-26")
	require.True(t, ok)
	require.Equal(t, "node-26", got.Variant)

	_, ok = ParseTLSPinProfileName("claude-code-macos")
	require.False(t, ok)
}

func TestTLSPinCatalogComplete(t *testing.T) {
	require.True(t, TLSPinCatalogComplete("pin:grok-cli:linux:h1", []uint16{1}, []uint16{2}, []string{"http/1.1"}))
	require.False(t, TLSPinCatalogComplete("pin:grok-cli:linux:h1", nil, []uint16{2}, []string{"http/1.1"}))
	require.False(t, TLSPinCatalogComplete("pin:grok-cli:linux:h1", []uint16{1}, []uint16{2}, nil))
}

func TestTLSPinFamilyMatchesPlatform(t *testing.T) {
	require.True(t, TLSPinFamilyMatchesPlatform(ClientFamilyClaudeCode, PlatformAnthropic))
	require.False(t, TLSPinFamilyMatchesPlatform(ClientFamilyClaudeCode, PlatformGrok))
}

func TestArchForPinnedOS(t *testing.T) {
	require.Equal(t, "arm64", ArchForPinnedOS(PlatformAnthropic, "macos"))
	require.Equal(t, "arm64", ArchForPinnedOS(PlatformAnthropic, "linux"))
	require.Equal(t, "x64", ArchForPinnedOS(PlatformOpenAI, "linux"))
	require.Equal(t, "x64", ArchForPinnedOS(PlatformGemini, "windows"))
}

func TestRejectUnusableDeviceTLSSelection(t *testing.T) {
	completeMatchingID := int64(101)
	incompleteNameID := int64(102)
	wrongPlatformID := int64(103)
	missingFieldsID := int64(104)
	h2ID := int64(105)
	iosID := int64(106)
	androidID := int64(107)
	svc := testTLSProfileService(
		&model.TLSFingerprintProfile{
			ID:            completeMatchingID,
			Name:          "pin:claude-code:macos:h1",
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"http/1.1"},
		},
		&model.TLSFingerprintProfile{
			ID:            incompleteNameID,
			Name:          "claude-code-macos-h1",
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"http/1.1"},
		},
		&model.TLSFingerprintProfile{
			ID:            wrongPlatformID,
			Name:          "pin:codex-cli:macos:h1",
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"http/1.1"},
		},
		&model.TLSFingerprintProfile{
			ID:            missingFieldsID,
			Name:          "pin:claude-code:macos:h1",
			CipherSuites:  nil,
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"http/1.1"},
		},
		&model.TLSFingerprintProfile{
			ID:            h2ID,
			Name:          "pin:claude-code:macos:h2",
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"h2", "http/1.1"},
		},
		&model.TLSFingerprintProfile{
			ID:            iosID,
			Name:          "pin:claude-code:ios:h1",
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"http/1.1"},
		},
		&model.TLSFingerprintProfile{
			ID:            androidID,
			Name:          "pin:claude-code:android:h1",
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"http/1.1"},
		},
	)

	for _, tc := range []struct {
		name string
		id   int64
	}{
		{name: "incomplete name", id: incompleteNameID},
		{name: "wrong platform", id: wrongPlatformID},
		{name: "missing fields", id: missingFieldsID},
		{name: "missing row", id: 999},
		{name: "h2", id: h2ID},
		{name: "ios", id: iosID},
		{name: "android", id: androidID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.RejectUnusableDeviceTLSSelection(PlatformAnthropic, map[string]any{
				DeviceTLSProfileIDExtraKey: tc.id,
			})
			require.ErrorContains(t, err, DeviceTLSProfileIDExtraKey)
		})
	}

	require.NoError(t, svc.RejectUnusableDeviceTLSSelection(PlatformAnthropic, map[string]any{
		DeviceTLSProfileIDExtraKey: completeMatchingID,
	}))
	require.NoError(t, svc.RejectUnusableDeviceTLSSelection(PlatformAnthropic, map[string]any{
		DeviceTLSProfileIDExtraKey: int64(0),
	}))
	require.NoError(t, svc.RejectUnusableDeviceTLSSelection(PlatformAnthropic, nil))
}

func TestTLSFingerprintProfileServiceListCompleteOptionsFiltersCatalogPins(t *testing.T) {
	description := "Claude Code macOS HTTP/1.1"
	svc := testTLSProfileService(
		&model.TLSFingerprintProfile{
			ID:            201,
			Name:          "pin:claude-code:macos:h1",
			Description:   &description,
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"http/1.1"},
		},
		&model.TLSFingerprintProfile{
			ID:            202,
			Name:          "pin:claude-code:linux:h1",
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: nil,
		},
		&model.TLSFingerprintProfile{
			ID:            203,
			Name:          "pin:grok-cli:macos:h1",
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"http/1.1"},
		},
		&model.TLSFingerprintProfile{
			ID:            204,
			Name:          "pin:claude-code:macos:h2",
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"h2", "http/1.1"},
		},
		&model.TLSFingerprintProfile{
			ID:            205,
			Name:          "pin:claude-code:ios:h1",
			CipherSuites:  []uint16{0x1301},
			Extensions:    []uint16{0},
			ALPNProtocols: []string{"http/1.1"},
		},
	)

	got, err := svc.ListCompleteOptions(context.Background(), PlatformAnthropic)
	require.NoError(t, err)
	require.Len(t, got, 1)

	require.Equal(t, int64(201), got[0].ID)
	require.Equal(t, "pin:claude-code:macos:h1", got[0].Name)
	require.Equal(t, description, got[0].Description)
	require.Equal(t, ClientFamilyClaudeCode, got[0].ClientFamily)
	require.Equal(t, "macos", got[0].OSFamily)
	require.Equal(t, TransportH1, got[0].Transport)
	require.Equal(t, fmt.Sprintf("%s / %s", claude.CLICurrentVersion, claude.CLIStainlessPackageVersion), got[0].SoftwareLabel)
}

func TestApplyChosenTLSPinMetaRewritesClaudePayloadOS(t *testing.T) {
	p := &AccountDeviceProfile{
		Platform:        PlatformAnthropic,
		OSFamily:        "linux",
		Arch:            "arm64",
		ClientFamily:    ClientFamilyClaudeCode,
		TransportFamily: TransportH1,
		ProfilePayload: map[string]any{
			"stainless_os":   "Linux",
			"stainless_arch": "arm64",
		},
	}

	err := ApplyChosenTLSPinMeta(p, "pin:claude-code:macos:h1")
	require.NoError(t, err)
	require.Equal(t, "macos", p.OSFamily)
	require.Equal(t, "arm64", p.Arch)
	require.Equal(t, "MacOS", p.ProfilePayload["stainless_os"])
	require.Equal(t, "arm64", p.ProfilePayload["stainless_arch"])
}

func TestApplyChosenTLSPinMetaRewritesCodexUAPlatform(t *testing.T) {
	p := &AccountDeviceProfile{
		Platform:        PlatformOpenAI,
		OSFamily:        "linux",
		Arch:            "x64",
		ClientFamily:    ClientFamilyCodexCLI,
		TransportFamily: TransportH1,
		ProfilePayload: map[string]any{
			"user_agent": "codex-tui/0.147.0 (Ubuntu 22.4.0; x86_64) xterm-256color",
		},
	}

	err := ApplyChosenTLSPinMeta(p, "pin:codex-cli:macos:h1")
	require.NoError(t, err)
	ua, _ := p.ProfilePayload["user_agent"].(string)
	require.Contains(t, strings.ToLower(ua), "macos")
	require.NotContains(t, strings.ToLower(ua), "ubuntu")
}

func TestApplyChosenTLSPinMetaRejectsUnusablePins(t *testing.T) {
	for _, name := range []string{
		"pin:claude-code:ios:h1",
		"pin:claude-code:android:h1",
		"pin:claude-code:macos:h2",
	} {
		t.Run(name, func(t *testing.T) {
			p := &AccountDeviceProfile{
				Platform:     PlatformAnthropic,
				OSFamily:     "linux",
				ClientFamily: ClientFamilyClaudeCode,
			}
			err := ApplyChosenTLSPinMeta(p, name)
			require.Error(t, err)
			require.ErrorContains(t, err, DeviceTLSProfileIDExtraKey)
			require.Equal(t, "linux", p.OSFamily)
			require.Nil(t, p.TLSProfileID)
		})
	}
}
