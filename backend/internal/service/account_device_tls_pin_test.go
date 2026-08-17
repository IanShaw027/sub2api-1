//go:build unit

package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTLSPinProfileName(t *testing.T) {
	require.Equal(t, "pin:codex-cli:linux:h1", service.TLSPinProfileName(service.ClientFamilyCodexCLI, "linux", service.TransportH1))
}

func TestGetOrCreatePinsTLSProfileFromCatalogOnce(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	pin, err := client.TLSFingerprintProfile.Create().
		SetName("pin:claude-code:linux:h1").
		Save(context.Background())
	require.NoError(t, err)
	id := pin.ID
	service.SetTLSProfilePinLookup(func(family, osFamily, transport string) *int64 {
		if family == service.ClientFamilyClaudeCode && osFamily == "linux" && transport == service.TransportH1 {
			return &id
		}
		return nil
	})
	t.Cleanup(func() { service.SetTLSProfilePinLookup(nil) })

	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)

	first, err := svc.GetOrCreate(context.Background(), account)
	require.NoError(t, err)
	require.NotNil(t, first.TLSProfileID)
	require.Equal(t, id, *first.TLSProfileID)

	second, err := svc.GetOrCreate(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, first.TLSProfileID, second.TLSProfileID)
	require.Equal(t, first.DeviceID, second.DeviceID)
}

func TestGetOrCreateHonorsChosenDeviceTLSProfileID(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	macosPin, err := client.TLSFingerprintProfile.Create().
		SetName("pin:claude-code:macos:h1").
		Save(ctx)
	require.NoError(t, err)
	linuxPin, err := client.TLSFingerprintProfile.Create().
		SetName("pin:claude-code:linux:h1").
		Save(ctx)
	require.NoError(t, err)
	macosID := macosPin.ID
	linuxID := linuxPin.ID
	service.SetTLSProfilePinLookup(func(family, osFamily, transport string) *int64 {
		if family != service.ClientFamilyClaudeCode || transport != service.TransportH1 {
			return nil
		}
		switch osFamily {
		case "macos":
			return &macosID
		case "linux":
			return &linuxID
		default:
			return nil
		}
	})
	service.SetTLSProfilePinNameLookup(func(id int64) string {
		switch id {
		case macosID:
			return "pin:claude-code:macos:h1"
		case linuxID:
			return "pin:claude-code:linux:h1"
		default:
			return ""
		}
	})
	t.Cleanup(func() {
		service.SetTLSProfilePinLookup(nil)
		service.SetTLSProfilePinNameLookup(nil)
	})

	chosen := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled":          true,
		service.DeviceTLSProfileIDExtraKey: float64(macosID),
	})

	first, err := svc.GetOrCreate(ctx, chosen)
	require.NoError(t, err)
	require.Equal(t, "macos", first.OSFamily)
	require.Equal(t, "arm64", first.Arch)
	require.Equal(t, service.TransportH1, first.TransportFamily)
	require.NotNil(t, first.TLSProfileID)
	require.Equal(t, macosID, *first.TLSProfileID)
	require.Equal(t, claude.CLICurrentVersion, first.ClientVersion)
	require.Equal(t, "MacOS", first.ProfilePayload["stainless_os"])
	require.Equal(t, "arm64", first.ProfilePayload["stainless_arch"])

	second, err := svc.GetOrCreate(ctx, chosen)
	require.NoError(t, err)
	require.Equal(t, first.DeviceID, second.DeviceID)
	require.Equal(t, first.TLSProfileID, second.TLSProfileID)

	plain := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})
	got, err := svc.GetOrCreate(ctx, plain)
	require.NoError(t, err)
	require.Equal(t, "linux", got.OSFamily)
	require.Equal(t, "arm64", got.Arch)
	require.NotNil(t, got.TLSProfileID)
	require.Equal(t, linuxID, *got.TLSProfileID)
}

func TestGetOrCreateChosenCodexMacOSPinRewritesUAPlatform(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	macosPin, err := client.TLSFingerprintProfile.Create().
		SetName("pin:codex-cli:macos:h1").
		Save(ctx)
	require.NoError(t, err)
	macosID := macosPin.ID
	service.SetTLSProfilePinNameLookup(func(id int64) string {
		if id == macosID {
			return "pin:codex-cli:macos:h1"
		}
		return ""
	})
	t.Cleanup(func() {
		service.SetTLSProfilePinLookup(nil)
		service.SetTLSProfilePinNameLookup(nil)
	})

	account := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
		"device_learning_enabled":          true,
		service.DeviceTLSProfileIDExtraKey: float64(macosID),
	})

	got, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.Equal(t, "macos", got.OSFamily)
	require.Equal(t, "arm64", got.Arch)
	require.Equal(t, macosID, *got.TLSProfileID)
	ua, _ := got.ProfilePayload["user_agent"].(string)
	require.Contains(t, strings.ToLower(ua), "macos")
	require.NotContains(t, strings.ToLower(ua), "ubuntu")
}

func TestGetOrCreateRejectsChosenIOSPin(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	iosPin, err := client.TLSFingerprintProfile.Create().
		SetName("pin:claude-code:ios:h1").
		Save(ctx)
	require.NoError(t, err)
	iosID := iosPin.ID
	service.SetTLSProfilePinNameLookup(func(id int64) string {
		if id == iosID {
			return "pin:claude-code:ios:h1"
		}
		return ""
	})
	t.Cleanup(func() {
		service.SetTLSProfilePinLookup(nil)
		service.SetTLSProfilePinNameLookup(nil)
	})

	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled":          true,
		service.DeviceTLSProfileIDExtraKey: float64(iosID),
	})

	got, err := svc.GetOrCreate(ctx, account)
	require.Error(t, err)
	require.ErrorContains(t, err, service.DeviceTLSProfileIDExtraKey)
	require.Nil(t, got)
}

func TestGetOrCreateDoesNotApplyChosenTLSProfileIDToExistingRow(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	macosPin, err := client.TLSFingerprintProfile.Create().
		SetName("pin:claude-code:macos:h1").
		Save(ctx)
	require.NoError(t, err)
	linuxPin, err := client.TLSFingerprintProfile.Create().
		SetName("pin:claude-code:linux:h1").
		Save(ctx)
	require.NoError(t, err)
	macosID := macosPin.ID
	linuxID := linuxPin.ID
	service.SetTLSProfilePinNameLookup(func(id int64) string {
		if id == macosID {
			return "pin:claude-code:macos:h1"
		}
		return ""
	})
	t.Cleanup(func() {
		service.SetTLSProfilePinLookup(nil)
		service.SetTLSProfilePinNameLookup(nil)
	})

	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})
	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.Equal(t, "linux", created.OSFamily)
	require.Nil(t, created.TLSProfileID)

	extra := map[string]any{
		"device_learning_enabled":          true,
		service.DeviceTLSProfileIDExtraKey: float64(macosID),
	}
	_, err = client.Account.UpdateOneID(account.ID).SetExtra(extra).Save(ctx)
	require.NoError(t, err)
	account.Extra = extra
	service.SetTLSProfilePinLookup(func(family, osFamily, transport string) *int64 {
		if family == service.ClientFamilyClaudeCode && osFamily == "linux" && transport == service.TransportH1 {
			return &linuxID
		}
		return nil
	})

	got, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Equal(t, "linux", got.OSFamily)
	require.NotNil(t, got.TLSProfileID)
	require.NotEqual(t, macosID, *got.TLSProfileID)
	require.Equal(t, linuxID, *got.TLSProfileID)
}

func TestGetOrCreateLeavesTLSUnpinnedWhenCatalogMisses(t *testing.T) {
	service.SetTLSProfilePinLookup(func(string, string, string) *int64 { return nil })
	t.Cleanup(func() { service.SetTLSProfilePinLookup(nil) })

	svc, client := newAccountDeviceService(t)
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
	got, err := svc.GetOrCreate(context.Background(), account)
	require.NoError(t, err)
	require.Nil(t, got.TLSProfileID)
}

func TestLearnIfOfficialDoesNotChangePinnedTLSProfileID(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	pin, err := client.TLSFingerprintProfile.Create().
		SetName("pin:claude-code:linux:h1").
		Save(context.Background())
	require.NoError(t, err)
	id := pin.ID
	service.SetTLSProfilePinLookup(func(string, string, string) *int64 { return &id })
	t.Cleanup(func() { service.SetTLSProfilePinLookup(nil) })

	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})
	created, err := svc.GetOrCreate(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, id, *created.TLSProfileID)
	seedOldClaudeSoftware(t, client, account.ID, true)

	got, err := svc.LearnIfOfficial(context.Background(), account, officialClaudeInbound())
	require.NoError(t, err)
	require.Equal(t, claude.CLICurrentVersion, got.ClientVersion)
	require.NotNil(t, got.TLSProfileID)
	require.Equal(t, id, *got.TLSProfileID)
	require.Equal(t, created.DeviceID, got.DeviceID)
}
