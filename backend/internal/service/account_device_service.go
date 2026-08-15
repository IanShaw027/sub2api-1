package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/google/uuid"
)

type AccountDeviceProfileRepository interface {
	GetByAccountID(ctx context.Context, accountID int64) (*AccountDeviceProfile, error)
	InsertBaseline(ctx context.Context, p *AccountDeviceProfile) (*AccountDeviceProfile, error)
	UpdateCAS(ctx context.Context, accountID, expectedRevision int64, next *AccountDeviceProfile) (bool, error)
}

type AccountDeviceService struct {
	repo      AccountDeviceProfileRepository
	cache     IdentityCache
	learnMu   sync.Mutex
	accountMu map[int64]*sync.Mutex
}

// OfficialInbound is an inbound official-client observation used to decide
// whether LearnIfOfficial may CAS-upgrade software fields.
//
// Expected Claude inbound Payload keys (mapped from X-Stainless-* headers):
// stainless_lang, stainless_package_version, stainless_os, stainless_arch,
// stainless_runtime, stainless_runtime_version. Learn compares only software
// keys (lang, package_version, runtime, runtime_version) against the registry.
// stainless_os and stainless_arch are first-write-wins identity and are not
// required to match the registry. Extra inbound stainless_* keys such as
// stainless_retry_count and stainless_timeout do not block learn.
// Runtime and RuntimeVersion are reserved/gate-only; written software values
// come from the registry bundle, not from inbound.
type OfficialInbound struct {
	UserAgent      string
	Originator     string
	ClientVersion  string
	Runtime        string
	RuntimeVersion string
	Payload        map[string]any
}

var (
	grokOfficialUAPattern        = regexp.MustCompile(`(?i)^xai-grok-workspace/\d+\.\d+\.\d+`)
	geminiOfficialUAPattern      = regexp.MustCompile(`(?i)^GeminiCLI/\d+\.\d+\.\d+`)
	antigravityOfficialUAPattern = regexp.MustCompile(`(?i)^antigravity/\d+\.\d+\.\d+`)
	overlaySoftwarePayloadKeySet = map[string]struct{}{
		"user_agent":                {},
		"originator":                {},
		"stainless_lang":            {},
		"stainless_package_version": {},
		"stainless_runtime":         {},
		"stainless_runtime_version": {},
		"grok_token_auth":           {},
		"grok_identifier":           {},
		"kiro_system_version":       {},
		"kiro_node_version":         {},
		"kiro_commit":               {},
	}
	stainlessSoftwareCompareKeys = []string{
		"stainless_lang",
		"stainless_package_version",
		"stainless_runtime",
		"stainless_runtime_version",
	}
)

func NewAccountDeviceService(repo AccountDeviceProfileRepository) *AccountDeviceService {
	return &AccountDeviceService{
		repo:      repo,
		accountMu: make(map[int64]*sync.Mutex),
	}
}

// WithCache attaches an optional Redis projection. Redis errors must not fail GetOrCreate.
func (s *AccountDeviceService) WithCache(cache IdentityCache) *AccountDeviceService {
	if s == nil {
		return s
	}
	s.cache = cache
	return s
}

func (s *AccountDeviceService) projectDeviceProfile(ctx context.Context, p *AccountDeviceProfile) {
	if s == nil || s.cache == nil || p == nil {
		return
	}
	if err := s.cache.SetDeviceProfile(ctx, p.AccountID, p); err != nil {
		slog.Warn("device profile redis projection failed", "account_id", p.AccountID, "error", err)
	}
}

func (s *AccountDeviceService) GetOrCreate(ctx context.Context, account *Account) (*AccountDeviceProfile, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("identity_reject: account device service is not configured")
	}
	if account == nil {
		return nil, fmt.Errorf("identity_reject: account is required")
	}
	if account.IsShadow() {
		if *account.ParentAccountID == account.ID {
			return nil, fmt.Errorf("identity_reject: shadow account %d parent cycle", account.ID)
		}
		parent := *account
		parent.ID = *account.ParentAccountID
		parent.ParentAccountID = nil
		return s.GetOrCreate(ctx, &parent)
	}

	existing, err := s.repo.GetByAccountID(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil && !deviceProfilePlatformMismatch(existing, account) {
		s.projectDeviceProfile(ctx, existing)
		return existing, nil
	}

	// Mismatched or missing row: insert a new baseline. leftoverSharedRepo
	// overwrites (test remint). Production unique(account_id) makes this
	// INSERT fail; the conflict handler below returns identity_reject and
	// never the stale row. That conflict is the intended fail-closed path
	// until an admin "reset device profile" action exists.

	baseline, err := buildAccountDeviceBaseline(account)
	if err != nil {
		return nil, err
	}
	if err := ValidateAccountDeviceProfile(baseline); err != nil {
		return nil, err
	}

	created, err := s.repo.InsertBaseline(ctx, baseline)
	if err != nil {
		if deviceProfilePlatformMismatch(existing, account) {
			return nil, fmt.Errorf("%w (insert baseline: %v)", deviceProfilePlatformMismatchError(existing, account), err)
		}
		existing, getErr := s.repo.GetByAccountID(ctx, account.ID)
		if getErr == nil && existing != nil {
			if deviceProfilePlatformMismatch(existing, account) {
				return nil, fmt.Errorf("%w (insert baseline: %v)", deviceProfilePlatformMismatchError(existing, account), err)
			}
			s.projectDeviceProfile(ctx, existing)
			return existing, nil
		}
		return nil, err
	}
	if deviceProfilePlatformMismatch(created, account) {
		return nil, deviceProfilePlatformMismatchError(created, account)
	}
	s.projectDeviceProfile(ctx, created)
	return created, nil
}

func (s *AccountDeviceService) LearnIfOfficial(ctx context.Context, account *Account, inbound OfficialInbound) (*AccountDeviceProfile, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("identity_reject: account device service is not configured")
	}
	if account == nil {
		return nil, fmt.Errorf("identity_reject: account is required")
	}

	unlock := s.lockAccount(canonicalDeviceAccountID(account))
	defer unlock()

	profile, err := s.GetOrCreate(ctx, account)
	if err != nil {
		return nil, err
	}
	if !deviceLearningEnabled(profile, account) {
		return profile, nil
	}
	if !isOfficialInbound(profile.Platform, inbound) {
		return profile, nil
	}

	candidate := inboundCandidateVersion(inbound)
	if candidate == "" {
		return profile, nil
	}
	if uaVersion := softwareBundleVersionFromUA(inbound.UserAgent); uaVersion != "" && uaVersion != candidate {
		return profile, nil
	}
	bundle, ok := NewSoftwareBundleRegistry().Lookup(profile.Platform, profile.ClientFamily, candidate)
	if !ok {
		return profile, nil
	}
	if CompareSemver(candidate, profile.ClientVersion) <= 0 {
		return profile, nil
	}
	if err := ValidateSoftwareBundle(bundle); err != nil {
		return nil, fmt.Errorf("identity_reject: %w", err)
	}
	if profile.Platform == PlatformAnthropic && claudeUAChanged(profile, bundle) && !stainlessMatchesRegistry(inbound.Payload, bundle.Payload) {
		slog.Warn("identity_reject", "reason", "ua_changed_without_stainless", "account_id", profile.AccountID)
		return profile, nil
	}
	if bundle.Runtime != "" && bundle.Runtime != profile.Runtime {
		return profile, nil
	}

	next := applyOfficialSoftwareBundle(profile, bundle)
	if err := ValidateAccountDeviceProfile(next); err != nil {
		slog.Warn("identity_reject", "reason", err.Error(), "account_id", profile.AccountID)
		return nil, err
	}

	wrote, err := s.repo.UpdateCAS(ctx, profile.AccountID, profile.Revision, next)
	if err != nil {
		return nil, fmt.Errorf("identity_reject: cas update: %w", err)
	}
	if !wrote {
		current, getErr := s.repo.GetByAccountID(ctx, profile.AccountID)
		if getErr != nil {
			return nil, getErr
		}
		if current != nil {
			return current, nil
		}
		return profile, nil
	}
	updated, err := s.repo.GetByAccountID(ctx, profile.AccountID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return next, nil
	}
	return updated, nil
}

func (s *AccountDeviceService) lockAccount(accountID int64) func() {
	s.learnMu.Lock()
	if s.accountMu == nil {
		s.accountMu = make(map[int64]*sync.Mutex)
	}
	m := s.accountMu[accountID]
	if m == nil {
		m = &sync.Mutex{}
		s.accountMu[accountID] = m
	}
	s.learnMu.Unlock()
	m.Lock()
	return m.Unlock
}

func canonicalDeviceAccountID(account *Account) int64 {
	if account.IsShadow() && *account.ParentAccountID != account.ID {
		return *account.ParentAccountID
	}
	return account.ID
}

func deviceLearningEnabled(profile *AccountDeviceProfile, account *Account) bool {
	if profile != nil && profile.LearningEnabled {
		return true
	}
	if account == nil || account.Extra == nil {
		return false
	}
	enabled, ok := account.Extra["device_learning_enabled"].(bool)
	return ok && enabled
}

func isOfficialInbound(platform string, inbound OfficialInbound) bool {
	ua := strings.TrimSpace(inbound.UserAgent)
	switch platform {
	case PlatformAnthropic:
		return claudeCodeUAPattern.MatchString(ua)
	case PlatformOpenAI:
		return openai.IsCodexOfficialClientByHeaders(ua, inbound.Originator)
	default:
		family := DefaultClientFamily(platform)
		if family == "" {
			return false
		}
		return inboundMatchesOfficialFamily(family, ua)
	}
}

func inboundMatchesOfficialFamily(family, userAgent string) bool {
	ua := strings.TrimSpace(userAgent)
	switch family {
	case ClientFamilyGrokCLI:
		return grokOfficialUAPattern.MatchString(ua)
	case ClientFamilyGeminiCLI:
		return geminiOfficialUAPattern.MatchString(ua)
	case ClientFamilyAntigravity:
		return antigravityOfficialUAPattern.MatchString(ua)
	default:
		return false
	}
}

func inboundCandidateVersion(inbound OfficialInbound) string {
	if version := strings.TrimSpace(inbound.ClientVersion); version != "" {
		return version
	}
	return softwareBundleVersionFromUA(inbound.UserAgent)
}

func claudeUAChanged(profile *AccountDeviceProfile, bundle SoftwareBundle) bool {
	profileUA, _ := profile.ProfilePayload["user_agent"].(string)
	if strings.TrimSpace(bundle.UserAgent) != strings.TrimSpace(profileUA) {
		return true
	}
	return strings.TrimSpace(bundle.ClientVersion) != strings.TrimSpace(profile.ClientVersion)
}

func stainlessMatchesRegistry(inbound, registry map[string]any) bool {
	want := stainlessSoftwareFields(registry)
	if len(want) == 0 {
		return true
	}
	got := stainlessSoftwareFields(inbound)
	for key, value := range want {
		if got[key] != value {
			return false
		}
	}
	return true
}

func stainlessSoftwareFields(payload map[string]any) map[string]string {
	out := make(map[string]string, len(stainlessSoftwareCompareKeys))
	if payload == nil {
		return out
	}
	for _, key := range stainlessSoftwareCompareKeys {
		if s, ok := payload[key].(string); ok {
			out[key] = s
		}
	}
	return out
}

func applyOfficialSoftwareBundle(profile *AccountDeviceProfile, bundle SoftwareBundle) *AccountDeviceProfile {
	next := *profile
	next.ClientVersion = bundle.ClientVersion
	if bundle.RuntimeVersion != "" {
		next.RuntimeVersion = bundle.RuntimeVersion
	}
	next.LearnedFrom = LearnedFromOfficial
	now := time.Now()
	next.VersionUpgradedAt = &now
	next.ProfilePayload = overlaySoftwarePayload(profile.ProfilePayload, bundle)
	return &next
}

func overlaySoftwarePayload(existing map[string]any, bundle SoftwareBundle) map[string]any {
	out := make(map[string]any, len(existing)+len(bundle.Payload))
	for key, value := range existing {
		out[key] = value
	}
	for key, value := range bundle.Payload {
		if !isOverlaySoftwareKey(key) {
			continue
		}
		out[key] = value
	}
	if bundle.UserAgent != "" {
		out["user_agent"] = bundle.UserAgent
	}
	if bundle.Originator != "" {
		out["originator"] = bundle.Originator
	}
	return out
}

func isOverlaySoftwareKey(key string) bool {
	_, ok := overlaySoftwarePayloadKeySet[key]
	return ok
}

func buildAccountDeviceBaseline(account *Account) (*AccountDeviceProfile, error) {
	sessionNamespace, err := randomSessionNamespace()
	if err != nil {
		return nil, fmt.Errorf("identity_reject: generate session_namespace: %w", err)
	}

	runtime := "node"
	switch account.Platform {
	case PlatformOpenAI:
		runtime = "codex_cli_rs"
	case PlatformGrok:
		runtime = "grok-shell"
	}

	osFamily, arch := baselineOSArch(account.Platform)
	clientVersion, runtimeVersion, payload := baselineSoftwareBundle(account.Platform)
	p := &AccountDeviceProfile{
		AccountID:          account.ID,
		Revision:           1,
		SchemaVersion:      1,
		Platform:           account.Platform,
		ClientFamily:       DefaultClientFamily(account.Platform),
		InstallationID:     uuid.NewString(),
		DeviceID:           uuid.NewString(),
		MachineID:          uuid.NewString(),
		GatewayAccountUUID: uuid.NewString(),
		SessionNamespace:   sessionNamespace,
		OSFamily:           osFamily,
		Arch:               arch,
		Runtime:            runtime,
		RuntimeVersion:     runtimeVersion,
		ClientVersion:      clientVersion,
		TLSProfileID:       nil,
		TransportFamily:    TransportH1,
		ProfilePayload:     payload,
		LearnedFrom:        LearnedFromBaseline,
		LearningEnabled:    false,
	}
	return p, nil
}

func baselineOSArch(platform string) (osFamily, arch string) {
	switch platform {
	case PlatformAnthropic:
		return "linux", "arm64"
	case PlatformOpenAI:
		return "linux", "x64"
	case PlatformGemini, PlatformAntigravity:
		return "windows", "x64"
	default:
		return "macos", "arm64"
	}
}

func baselineSoftwareBundle(platform string) (clientVersion, runtimeVersion string, payload map[string]any) {
	switch platform {
	case PlatformAnthropic:
		ua := claude.DefaultHeaders["User-Agent"]
		return claude.CLICurrentVersion, claude.DefaultHeaders["X-Stainless-Runtime-Version"], map[string]any{
			"user_agent":                ua,
			"stainless_lang":            claude.DefaultHeaders["X-Stainless-Lang"],
			"stainless_package_version": claude.DefaultHeaders["X-Stainless-Package-Version"],
			"stainless_os":              claude.DefaultHeaders["X-Stainless-OS"],
			"stainless_arch":            claude.DefaultHeaders["X-Stainless-Arch"],
			"stainless_runtime":         claude.DefaultHeaders["X-Stainless-Runtime"],
			"stainless_runtime_version": claude.DefaultHeaders["X-Stainless-Runtime-Version"],
		}
	case PlatformOpenAI:
		version := NormalizeCodexClientVersion(codexCLIVersion)
		ua := buildCodexCLIUserAgent(version)
		return version, version, map[string]any{
			"user_agent": ua,
			"originator": openai.CodexDefaultOriginator,
		}
	case PlatformGrok:
		ua := xai.CLIUserAgent(xai.CLIClientVersion)
		return xai.CLIClientVersion, xai.CLIClientVersion, map[string]any{
			"user_agent":      ua,
			"grok_token_auth": xai.CLITokenAuth,
			"grok_identifier": xai.CLIClientIdentifier,
		}
	case PlatformKiro:
		return defaultKiroVersion, defaultKiroNodeVersion, map[string]any{
			"kiro_system_version": defaultKiroSystemVersion,
			"kiro_node_version":   defaultKiroNodeVersion,
		}
	case PlatformGemini:
		version := softwareBundleVersionFromUA(geminicli.GeminiCLIUserAgent)
		return version, claude.DefaultHeaders["X-Stainless-Runtime-Version"], map[string]any{
			"user_agent": geminicli.GeminiCLIUserAgent,
		}
	case PlatformAntigravity:
		return antigravity.DefaultUserAgentVersion, claude.DefaultHeaders["X-Stainless-Runtime-Version"], map[string]any{
			"user_agent": antigravity.BuildUserAgent(antigravity.DefaultUserAgentVersion),
		}
	default:
		return "1.0.0", claude.DefaultHeaders["X-Stainless-Runtime-Version"], map[string]any{}
	}
}

func randomSessionNamespace() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
