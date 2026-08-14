package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
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

	platform := strings.TrimSpace(p.Platform)
	if !isDeviceProfilePlatform(platform) {
		return fmt.Errorf("identity_reject: platform %q is not allowed", p.Platform)
	}
	wantFamily := DefaultClientFamily(platform)
	if strings.TrimSpace(p.ClientFamily) != wantFamily {
		return fmt.Errorf("identity_reject: client_family %q does not match platform %q", p.ClientFamily, platform)
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
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok, PlatformKiro:
		return true
	default:
		return false
	}
}

func validateSessionNamespace(ns string) error {
	ns = strings.TrimSpace(ns)
	if len(ns) < minSessionNamespaceLen || len(ns) > maxSessionNamespaceLen {
		return fmt.Errorf("identity_reject: session_namespace must be %d–%d hex characters", minSessionNamespaceLen, maxSessionNamespaceLen)
	}
	for _, r := range ns {
		if !isHexRune(r) {
			return fmt.Errorf("identity_reject: session_namespace must be %d–%d hex characters", minSessionNamespaceLen, maxSessionNamespaceLen)
		}
	}
	return nil
}

func isHexRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
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
	switch strings.TrimSpace(family) {
	case TransportH1, TransportH2:
		return nil
	default:
		return fmt.Errorf("identity_reject: transport_family must be %s or %s", TransportH1, TransportH2)
	}
}

func validateLearnedFrom(v string) error {
	switch strings.TrimSpace(v) {
	case LearnedFromBaseline, LearnedFromOfficial, LearnedFromBaselineFloor:
		return nil
	default:
		return fmt.Errorf("identity_reject: learned_from %q is not allowed", v)
	}
}

func validateVersionField(field, value string) error {
	value = strings.TrimSpace(value)
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
	if osFamily := strings.TrimSpace(p.OSFamily); osFamily != "" && !isKnownOSFamily(osFamily) {
		return fmt.Errorf("identity_reject: os_family %q is not allowed", p.OSFamily)
	}
	if arch := strings.TrimSpace(p.Arch); arch != "" && !isKnownArch(arch) {
		return fmt.Errorf("identity_reject: arch %q is not allowed", p.Arch)
	}
	if runtime := strings.TrimSpace(p.Runtime); runtime != "" && !isKnownRuntime(runtime) {
		return fmt.Errorf("identity_reject: runtime %q is not allowed", p.Runtime)
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
		if !isAllowedProfilePayloadKey(key) {
			return fmt.Errorf("identity_reject: profile_payload key %q is not allowed", key)
		}
		if err := validatePayloadValue(key, value); err != nil {
			return err
		}
	}
	return nil
}

func isAllowedProfilePayloadKey(key string) bool {
	switch strings.TrimSpace(key) {
	case "user_agent", "originator", "grok_token_auth", "grok_identifier",
		"kiro_system_version", "kiro_node_version", "kiro_commit":
		return true
	default:
		return strings.HasPrefix(strings.TrimSpace(key), "stainless_")
	}
}

func validatePayloadValue(key string, value any) error {
	s, ok := value.(string)
	if !ok {
		return nil
	}
	if err := rejectCTL("profile_payload."+key, s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
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
		if err := rejectCTL(field.name, field.value); err != nil {
			return err
		}
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
