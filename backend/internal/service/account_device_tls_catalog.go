package service

import (
	"fmt"
	"strings"
	"sync"
)

const DeviceTLSProfileIDExtraKey = "device_tls_profile_id"

type deviceTLSCatalogGuardFunc func(platform string, extra map[string]any) error

var (
	deviceTLSCatalogGuardMu sync.RWMutex
	deviceTLSCatalogGuard   deviceTLSCatalogGuardFunc
)

func setDeviceTLSCatalogGuard(fn deviceTLSCatalogGuardFunc) {
	deviceTLSCatalogGuardMu.Lock()
	deviceTLSCatalogGuard = fn
	deviceTLSCatalogGuardMu.Unlock()
}

func rejectUnusableDeviceTLSSelection(platform string, extra map[string]any) error {
	if !deviceTLSSelectionRequiresCatalog(extra) {
		return nil
	}
	deviceTLSCatalogGuardMu.RLock()
	fn := deviceTLSCatalogGuard
	deviceTLSCatalogGuardMu.RUnlock()
	if fn == nil {
		return identityReject(DeviceTLSProfileIDExtraKey + " catalog guard is not configured")
	}
	return fn(platform, extra)
}

func deviceTLSSelectionRequiresCatalog(extra map[string]any) bool {
	id, present, err := optionalCapacityInt(extra, DeviceTLSProfileIDExtraKey)
	return err == nil && present && id > 0
}

type ParsedTLSPinName struct {
	ClientFamily string
	OSFamily     string
	Transport    string
	Variant      string
}

func ParseTLSPinProfileName(name string) (ParsedTLSPinName, bool) {
	trimmed := strings.TrimSpace(name)
	if !strings.HasPrefix(trimmed, "pin:") {
		return ParsedTLSPinName{}, false
	}

	parts := strings.Split(trimmed, ":")
	if len(parts) < 4 {
		return ParsedTLSPinName{}, false
	}

	family := parts[1]
	osFamily := parts[2]
	transport := parts[3]
	if !isKnownTLSPinFamily(family) || !isAllowedPinnedOSFamily(osFamily) || !isAllowedPinnedTransport(transport) {
		return ParsedTLSPinName{}, false
	}

	return ParsedTLSPinName{
		ClientFamily: family,
		OSFamily:     osFamily,
		Transport:    transport,
		Variant:      strings.Join(parts[4:], ":"),
	}, true
}

func TLSPinCatalogComplete(name string, cipherSuites []uint16, extensions []uint16, alpn []string) bool {
	_, ok := ParseTLSPinProfileName(name)
	return ok && len(cipherSuites) > 0 && len(extensions) > 0 && len(alpn) > 0
}

func TLSPinFamilyMatchesPlatform(family, platform string) bool {
	return DefaultClientFamily(platform) == family
}

func (s *TLSFingerprintProfileService) RejectUnusableDeviceTLSSelection(platform string, extra map[string]any) error {
	id, present, err := optionalCapacityInt(extra, DeviceTLSProfileIDExtraKey)
	if err != nil {
		return err
	}
	if !present || id <= 0 {
		return nil
	}
	if s == nil {
		return identityReject(DeviceTLSProfileIDExtraKey + " catalog is not configured")
	}

	s.localMu.RLock()
	profile := s.localCache[id]
	s.localMu.RUnlock()
	if profile == nil {
		return identityReject(fmt.Sprintf("%s references unknown TLS profile %d", DeviceTLSProfileIDExtraKey, id))
	}
	parsed, ok := ParseTLSPinProfileName(profile.Name)
	if !ok || !TLSPinCatalogComplete(profile.Name, profile.CipherSuites, profile.Extensions, profile.ALPNProtocols) {
		return identityReject(DeviceTLSProfileIDExtraKey + " must reference a complete device TLS pin profile")
	}
	if !TLSPinFamilyMatchesPlatform(parsed.ClientFamily, platform) {
		return identityReject(DeviceTLSProfileIDExtraKey + " does not match account platform")
	}
	return rejectUnusableTLSPin(parsed)
}

func rejectUnusableTLSPin(parsed ParsedTLSPinName) error {
	switch parsed.OSFamily {
	case "ios", "android":
		return identityReject(fmt.Sprintf("%s cannot use %s device TLS pin", DeviceTLSProfileIDExtraKey, parsed.OSFamily))
	}
	if parsed.Transport == TransportH2 {
		return identityReject(DeviceTLSProfileIDExtraKey + " cannot use h2 device TLS pin")
	}
	return nil
}

func ArchForPinnedOS(platform, osFamily string) string {
	switch osFamily {
	case "macos":
		return "arm64"
	case "windows":
		return "x64"
	case "linux":
		_, arch := baselineOSArch(platform)
		return arch
	default:
		return ""
	}
}

func isKnownTLSPinFamily(family string) bool {
	switch family {
	case ClientFamilyClaudeCode, ClientFamilyCodexCLI, ClientFamilyGrokCLI,
		ClientFamilyKiroIDE, ClientFamilyGeminiCLI, ClientFamilyAntigravity:
		return true
	default:
		return false
	}
}

func isAllowedPinnedOSFamily(osFamily string) bool {
	switch osFamily {
	case "linux", "macos", "windows", "ios", "android":
		return true
	default:
		return false
	}
}

func isAllowedPinnedTransport(transport string) bool {
	switch transport {
	case TransportH1, TransportH2:
		return true
	default:
		return false
	}
}
