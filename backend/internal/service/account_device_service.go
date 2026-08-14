package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

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
	repo AccountDeviceProfileRepository
}

func NewAccountDeviceService(repo AccountDeviceProfileRepository) *AccountDeviceService {
	return &AccountDeviceService{repo: repo}
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
	if existing != nil {
		return existing, nil
	}

	baseline, err := buildAccountDeviceBaseline(account)
	if err != nil {
		return nil, err
	}
	if err := ValidateAccountDeviceProfile(baseline); err != nil {
		return nil, err
	}

	created, err := s.repo.InsertBaseline(ctx, baseline)
	if err != nil {
		existing, getErr := s.repo.GetByAccountID(ctx, account.ID)
		if getErr == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}
	return created, nil
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
