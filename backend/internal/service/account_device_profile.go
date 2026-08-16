package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type AccountDeviceProfile struct {
	ID                 int64
	AccountID          int64
	Revision           int64
	SchemaVersion      int
	Platform           string
	ClientFamily       string
	InstallationID     string
	DeviceID           string
	ClientID           string
	MachineID          string
	GatewayAccountUUID string
	SessionNamespace   string
	OSFamily           string
	Arch               string
	Runtime            string
	RuntimeVersion     string
	ClientVersion      string
	TLSProfileID       *int64
	TransportFamily    string
	ProfilePayload     map[string]any
	LearnedFrom        string // baseline | official_traffic | baseline_floor
	LearningEnabled    bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	VersionUpgradedAt  *time.Time
}

const (
	ClientFamilyClaudeCode   = "claude-code"
	ClientFamilyCodexCLI     = "codex-cli"
	ClientFamilyGrokCLI      = "grok-cli"
	ClientFamilyKiroIDE      = "kiro-ide"
	ClientFamilyGeminiCLI    = "gemini-cli"
	ClientFamilyAntigravity  = "antigravity"
	TransportH1              = "h1"
	TransportH2              = "h2"
	LearnedFromBaseline      = "baseline"
	LearnedFromOfficial      = "official_traffic"
	LearnedFromBaselineFloor = "baseline_floor"
)

const (
	maxProfilePayloadBytes = 8 * 1024
	maxUserAgentLen        = 256
	maxHeaderValueLen      = 512
	maxVersionLen          = 64
	maxIdentityIDLen       = 64
	maxOSArchRuntimeLen    = 32
	minSessionNamespaceLen = 32
	maxSessionNamespaceLen = 64
	minSchemaVersion       = 1
	maxSchemaVersion       = 100
)

func DefaultClientFamily(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformAnthropic:
		return ClientFamilyClaudeCode
	case PlatformOpenAI:
		return ClientFamilyCodexCLI
	case PlatformGrok:
		return ClientFamilyGrokCLI
	case PlatformKiro:
		return ClientFamilyKiroIDE
	case PlatformGemini:
		return ClientFamilyGeminiCLI
	case PlatformAntigravity:
		return ClientFamilyAntigravity
	default:
		return ""
	}
}

func ValidateAccountDeviceProfile(p *AccountDeviceProfile) error {
	return validateAccountDeviceProfile(p, true)
}

func ValidateOutboundBundle(p *AccountDeviceProfile) error {
	return validateAccountDeviceProfile(p, false)
}

func validateAccountDeviceProfile(p *AccountDeviceProfile, strictOSArchRuntime bool) error {
	if p == nil {
		return fmt.Errorf("identity_reject: profile is required")
	}
	if p.Revision < 1 {
		return fmt.Errorf("identity_reject: revision must be >= 1")
	}

	if !isDeviceProfilePlatform(p.Platform) {
		return fmt.Errorf("identity_reject: platform %q is not allowed", p.Platform)
	}
	wantFamily := DefaultClientFamily(p.Platform)
	if p.ClientFamily != wantFamily {
		return fmt.Errorf("identity_reject: client_family %q does not match platform %q", p.ClientFamily, p.Platform)
	}

	if err := validateSessionNamespace(p.SessionNamespace); err != nil {
		return err
	}
	if err := validateTLSProfileID(p.TLSProfileID); err != nil {
		return err
	}
	if err := validateTransportFamily(p.TransportFamily); err != nil {
		return err
	}
	if err := validateLearnedFrom(p.LearnedFrom); err != nil {
		return err
	}
	if p.SchemaVersion < minSchemaVersion || p.SchemaVersion > maxSchemaVersion {
		return fmt.Errorf("identity_reject: schema_version must be %d–%d", minSchemaVersion, maxSchemaVersion)
	}
	if err := validateVersionField("runtime_version", p.RuntimeVersion); err != nil {
		return err
	}
	if err := validateVersionField("client_version", p.ClientVersion); err != nil {
		return err
	}
	if err := validateProfilePayload(p.ProfilePayload); err != nil {
		return err
	}
	if err := validateProfileStrings(p); err != nil {
		return err
	}
	if strictOSArchRuntime {
		if err := validateKnownOSArchRuntime(p); err != nil {
			return err
		}
	}
	return nil
}

func isDeviceProfilePlatform(platform string) bool {
	switch platform {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok, PlatformKiro:
		return true
	default:
		return false
	}
}

func validateSessionNamespace(ns string) error {
	if len(ns) < minSessionNamespaceLen || len(ns) > maxSessionNamespaceLen {
		return fmt.Errorf("identity_reject: session_namespace must be %d–%d hex characters", minSessionNamespaceLen, maxSessionNamespaceLen)
	}
	for _, r := range ns {
		if !isLowerHexRune(r) {
			return fmt.Errorf("identity_reject: session_namespace must be %d–%d hex characters", minSessionNamespaceLen, maxSessionNamespaceLen)
		}
	}
	return nil
}

func isLowerHexRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
}

func validateTLSProfileID(id *int64) error {
	if id == nil {
		return nil
	}
	if *id <= 0 {
		return fmt.Errorf("identity_reject: tls_profile_id must be nil or >0")
	}
	return nil
}

func validateTransportFamily(family string) error {
	switch family {
	case TransportH1, TransportH2:
		return nil
	default:
		return fmt.Errorf("identity_reject: transport_family must be %s or %s", TransportH1, TransportH2)
	}
}

// EffectiveTransportFamily returns h2 only when the profile claims h2, the
// configured/negotiated ALPN list includes h2, and the HTTP transport actually
// enables HTTP/2. Otherwise it returns h1.
func EffectiveTransportFamily(claimed string, alpn []string, http2Enabled bool) string {
	if claimed != TransportH2 || !http2Enabled || !ALPNContainsH2(alpn) {
		return TransportH1
	}
	return TransportH2
}

// PersistDeviceTransportFamily downgrades an unverified h2 claim. Invalid or
// empty families are left untouched so ValidateAccountDeviceProfile can still
// identity_reject them.
func PersistDeviceTransportFamily(p *AccountDeviceProfile, alpn []string, http2Enabled bool) {
	if p == nil || p.TransportFamily != TransportH2 {
		return
	}
	p.TransportFamily = EffectiveTransportFamily(p.TransportFamily, alpn, http2Enabled)
}

// StampTLSProfileFromDevice copies the device-profile transport family onto a
// runtime TLS profile after ToTLSProfile. ToTLSProfile itself has no device.
func StampTLSProfileFromDevice(profile *tlsfingerprint.Profile, device *AccountDeviceProfile) {
	if profile == nil || device == nil {
		return
	}
	profile.TransportFamily = device.TransportFamily
}

// ALPNContainsH2 reports whether the ALPN list includes the h2 token.
func ALPNContainsH2(alpn []string) bool {
	for _, proto := range alpn {
		if proto == "h2" {
			return true
		}
	}
	return false
}

func validateLearnedFrom(v string) error {
	switch v {
	case LearnedFromBaseline, LearnedFromOfficial, LearnedFromBaselineFloor:
		return nil
	default:
		return fmt.Errorf("identity_reject: learned_from %q is not allowed", v)
	}
}

func validateVersionField(field, value string) error {
	if value == "" {
		return nil
	}
	if len(value) > maxVersionLen {
		return fmt.Errorf("identity_reject: %s exceeds %d characters", field, maxVersionLen)
	}
	if err := rejectCTL(field, value); err != nil {
		return err
	}
	lower := strings.ToLower(value)
	for _, banned := range []string{"-local", "-dev", "+build"} {
		if strings.Contains(lower, banned) {
			return fmt.Errorf("identity_reject: %s must not contain %s", field, banned)
		}
	}
	return nil
}

func validateKnownOSArchRuntime(p *AccountDeviceProfile) error {
	if err := validateRequiredKnownField("os_family", p.OSFamily, isKnownOSFamily); err != nil {
		return err
	}
	if err := validateRequiredKnownField("arch", p.Arch, isKnownArch); err != nil {
		return err
	}
	if err := validateRequiredKnownField("runtime", p.Runtime, isKnownRuntime); err != nil {
		return err
	}
	return nil
}

func validateRequiredKnownField(field, value string, known func(string) bool) error {
	if value == "" || len(value) > maxOSArchRuntimeLen || !known(value) {
		return fmt.Errorf("identity_reject: %s %q is not allowed", field, value)
	}
	return nil
}

func isKnownOSFamily(v string) bool {
	switch strings.ToLower(v) {
	case "macos", "linux", "windows", "freebsd", "openbsd", "ios", "android", "other", "unknown":
		return true
	default:
		return false
	}
}

func isKnownArch(v string) bool {
	switch strings.ToLower(v) {
	case "arm64", "x64", "x32", "arm", "unknown", "other":
		return true
	default:
		return false
	}
}

func isKnownRuntime(v string) bool {
	switch strings.ToLower(v) {
	case "node", "bun", "deno", "codex_cli_rs", "grok-shell":
		return true
	default:
		return false
	}
}

func validateProfilePayload(payload map[string]any) error {
	if payload == nil {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("identity_reject: profile_payload is not a valid object: %w", err)
	}
	if len(raw) > maxProfilePayloadBytes {
		return fmt.Errorf("identity_reject: profile_payload exceeds 8KB")
	}
	for key, value := range payload {
		if err := rejectCTL("profile_payload key", key); err != nil {
			return err
		}
		if strings.TrimSpace(key) != key || !isAllowedProfilePayloadKey(key) {
			return fmt.Errorf("identity_reject: profile_payload key %q is not allowed", key)
		}
		if err := validatePayloadValue(key, value); err != nil {
			return err
		}
	}
	return nil
}

func isAllowedProfilePayloadKey(key string) bool {
	switch key {
	case "user_agent", "originator", "grok_token_auth", "grok_identifier",
		"kiro_system_version", "kiro_node_version", "kiro_commit":
		return true
	default:
		return strings.HasPrefix(key, "stainless_")
	}
}

func validatePayloadValue(key string, value any) error {
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("identity_reject: profile_payload.%s must be a string", key)
	}
	if err := rejectUntrimmed("profile_payload."+key, s); err != nil {
		return err
	}
	if err := rejectCTL("profile_payload."+key, s); err != nil {
		return err
	}
	if key == "user_agent" && len(s) > maxUserAgentLen {
		return fmt.Errorf("identity_reject: user_agent exceeds %d characters", maxUserAgentLen)
	}
	if key != "user_agent" && len(s) > maxHeaderValueLen {
		return fmt.Errorf("identity_reject: profile_payload.%s exceeds %d characters", key, maxHeaderValueLen)
	}
	return nil
}

func validateProfileStrings(p *AccountDeviceProfile) error {
	fields := []struct {
		name  string
		value string
	}{
		{"platform", p.Platform},
		{"client_family", p.ClientFamily},
		{"installation_id", p.InstallationID},
		{"device_id", p.DeviceID},
		{"client_id", p.ClientID},
		{"machine_id", p.MachineID},
		{"gateway_account_uuid", p.GatewayAccountUUID},
		{"session_namespace", p.SessionNamespace},
		{"os_family", p.OSFamily},
		{"arch", p.Arch},
		{"runtime", p.Runtime},
		{"runtime_version", p.RuntimeVersion},
		{"client_version", p.ClientVersion},
		{"transport_family", p.TransportFamily},
		{"learned_from", p.LearnedFrom},
	}
	for _, field := range fields {
		if err := rejectUntrimmed(field.name, field.value); err != nil {
			return err
		}
		if err := rejectCTL(field.name, field.value); err != nil {
			return err
		}
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"installation_id", p.InstallationID},
		{"device_id", p.DeviceID},
		{"client_id", p.ClientID},
		{"machine_id", p.MachineID},
		{"gateway_account_uuid", p.GatewayAccountUUID},
	} {
		if len(field.value) > maxIdentityIDLen {
			return fmt.Errorf("identity_reject: %s exceeds %d characters", field.name, maxIdentityIDLen)
		}
	}
	return nil
}

func rejectUntrimmed(field, value string) error {
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("identity_reject: %s must be trimmed", field)
	}
	return nil
}

func rejectCTL(field, value string) error {
	for _, r := range value {
		if r == 0 || unicode.IsControl(r) {
			return fmt.Errorf("identity_reject: %s contains CTL/NUL", field)
		}
	}
	return nil
}
