package service

import (
	"strings"
	"sync"
)

// TLSPinProfileName is the catalog name used once at pin time.
// Request-time outbound never re-routes by this triple.
func TLSPinProfileName(family, osFamily, transport string) string {
	return "pin:" + family + ":" + osFamily + ":" + transport
}

type tlsProfilePinLookupFunc func(family, osFamily, transport string) *int64
type tlsProfilePinNameLookupFunc func(id int64) string

var (
	tlsProfilePinMu         sync.RWMutex
	tlsProfilePinLookup     tlsProfilePinLookupFunc
	tlsProfilePinNameLookup tlsProfilePinNameLookupFunc
)

// SetTLSProfilePinLookup registers the catalog lookup used by GetOrCreate.
// Tests pass a stub; production registers TLSFingerprintProfileService.
func SetTLSProfilePinLookup(fn tlsProfilePinLookupFunc) {
	tlsProfilePinMu.Lock()
	tlsProfilePinLookup = fn
	tlsProfilePinMu.Unlock()
}

func SetTLSProfilePinNameLookup(fn tlsProfilePinNameLookupFunc) {
	tlsProfilePinMu.Lock()
	tlsProfilePinNameLookup = fn
	tlsProfilePinMu.Unlock()
}

func lookupTLSProfilePin(family, osFamily, transport string) *int64 {
	tlsProfilePinMu.RLock()
	fn := tlsProfilePinLookup
	tlsProfilePinMu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(family, osFamily, transport)
}

func lookupTLSProfilePinName(id int64) string {
	tlsProfilePinMu.RLock()
	fn := tlsProfilePinNameLookup
	tlsProfilePinMu.RUnlock()
	if fn == nil {
		return ""
	}
	return fn(id)
}

func ChosenDeviceTLSProfileID(account *Account) *int64 {
	if !accountExtraDeviceLearningEnabled(account) {
		return nil
	}
	id, present, err := optionalCapacityInt(account.Extra, DeviceTLSProfileIDExtraKey)
	if err != nil || !present || id <= 0 {
		return nil
	}
	copied := id
	return &copied
}

// DeviceTLSProfileIDChangedToDifferentPositive reports whether the raw extra
// value changed to a different positive device TLS profile ID.
func DeviceTLSProfileIDChangedToDifferentPositive(previousExtra, nextExtra map[string]any) (bool, error) {
	nextID, nextPresent, err := optionalCapacityInt(nextExtra, DeviceTLSProfileIDExtraKey)
	if err != nil {
		return false, err
	}
	if !nextPresent || nextID <= 0 {
		return false, nil
	}
	previousID, previousPresent, err := optionalCapacityInt(previousExtra, DeviceTLSProfileIDExtraKey)
	if err != nil {
		return false, err
	}
	if !previousPresent || previousID <= 0 {
		return true, nil
	}
	return previousID != nextID, nil
}

func ApplyChosenTLSPinMeta(p *AccountDeviceProfile, pinName string) error {
	if p == nil {
		return identityReject(DeviceTLSProfileIDExtraKey + " profile is required")
	}
	parsed, ok := ParseTLSPinProfileName(pinName)
	if !ok {
		return identityReject(DeviceTLSProfileIDExtraKey + " must reference a complete device TLS pin profile")
	}
	if err := rejectUnusableTLSPin(parsed); err != nil {
		return err
	}
	arch := ArchForPinnedOS(p.Platform, parsed.OSFamily)
	if arch == "" {
		return identityReject(DeviceTLSProfileIDExtraKey + " cannot use this device TLS pin OS")
	}
	p.ClientFamily = parsed.ClientFamily
	p.OSFamily = parsed.OSFamily
	p.TransportFamily = parsed.Transport
	p.Arch = arch
	applyPinnedOSToPayload(p)
	return nil
}

func applyPinnedOSToPayload(p *AccountDeviceProfile) {
	if p == nil {
		return
	}
	if p.ProfilePayload == nil {
		p.ProfilePayload = map[string]any{}
	}
	if _, ok := p.ProfilePayload["stainless_os"]; ok || p.ClientFamily == ClientFamilyClaudeCode {
		p.ProfilePayload["stainless_os"] = stainlessOSForPinnedFamily(p.OSFamily)
		p.ProfilePayload["stainless_arch"] = p.Arch
	}
	if ua, ok := p.ProfilePayload["user_agent"].(string); ok && p.ClientFamily == ClientFamilyCodexCLI {
		p.ProfilePayload["user_agent"] = rewriteCodexUAPlatform(ua, p.OSFamily, p.Arch)
	}
}

func stainlessOSForPinnedFamily(osFamily string) string {
	switch osFamily {
	case "macos":
		return "MacOS"
	case "windows":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return osFamily
	}
}

func rewriteCodexUAPlatform(ua, osFamily, arch string) string {
	start := strings.IndexByte(ua, '(')
	end := strings.IndexByte(ua, ')')
	if start < 0 || end <= start {
		return ua
	}
	return ua[:start+1] + pinnedCodexPlatformToken(osFamily) + "; " + pinnedCodexArchToken(arch) + ua[end:]
}

func pinnedCodexPlatformToken(osFamily string) string {
	switch osFamily {
	case "macos":
		return "macOS"
	case "windows":
		return "Windows"
	default:
		return "Ubuntu 22.4.0"
	}
}

func pinnedCodexArchToken(arch string) string {
	if arch == "x64" {
		return "x86_64"
	}
	return arch
}

func applyTLSProfilePin(p *AccountDeviceProfile) {
	if p == nil || p.TLSProfileID != nil {
		return
	}
	id := lookupTLSProfilePin(p.ClientFamily, p.OSFamily, p.TransportFamily)
	if id == nil || *id <= 0 {
		return
	}
	copied := *id
	p.TLSProfileID = &copied
}
