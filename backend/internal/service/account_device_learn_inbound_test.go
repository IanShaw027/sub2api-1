//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/stretchr/testify/require"
)

func TestMaybeLearnOfficialDeviceProfileUpgradesOfficialClaude(t *testing.T) {
	account := leftoverClaudeLearnAccount(1940)
	profile := leftoverValidProfile(account.ID, PlatformAnthropic, ClientFamilyClaudeCode, "claude-cli/0.1.0 (external, cli)", "mid-learn")
	profile.ClientVersion = "0.1.0"
	profile.RuntimeVersion = "v18.0.0"
	profile.ProfilePayload = map[string]any{
		"user_agent":                "claude-cli/0.1.0 (external, cli)",
		"stainless_lang":            "js",
		"stainless_package_version": "0.1.0",
		"stainless_os":              "linux",
		"stainless_arch":            "arm64",
		"stainless_runtime":         "node",
		"stainless_runtime_version": "v18.0.0",
	}
	repo := installLeftoverLearnRepo(t, profile)

	maybeLearnOfficialDeviceProfile(context.Background(), account, officialClaudeLearnHeaders())
	require.Equal(t, 1, repo.casCount)
	got, err := repo.GetByAccountID(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, claude.CLICurrentVersion, got.ClientVersion)
	require.Equal(t, profile.DeviceID, got.DeviceID)
}

func TestMaybeLearnOfficialDeviceProfileSkipsWhenLearningDisabled(t *testing.T) {
	account := leftoverClaudeLearnAccount(1941)
	account.Extra = nil
	profile := leftoverValidProfile(account.ID, PlatformAnthropic, ClientFamilyClaudeCode, "claude-cli/0.1.0 (external, cli)", "mid-disabled")
	profile.ClientVersion = "0.1.0"
	repo := installLeftoverLearnRepo(t, profile)

	maybeLearnOfficialDeviceProfile(context.Background(), account, officialClaudeLearnHeaders())
	require.Equal(t, 0, repo.casCount)
	require.Equal(t, 0, repo.getCount(account.ID))
}

func TestMaybeLearnOfficialDeviceProfileSkipsUnofficialUA(t *testing.T) {
	account := leftoverClaudeLearnAccount(1942)
	profile := leftoverValidProfile(account.ID, PlatformAnthropic, ClientFamilyClaudeCode, "claude-cli/0.1.0 (external, cli)", "mid-unofficial")
	profile.ClientVersion = "0.1.0"
	repo := installLeftoverLearnRepo(t, profile)

	headers := officialClaudeLearnHeaders()
	headers.Set("User-Agent", "curl/8.0")
	maybeLearnOfficialDeviceProfile(context.Background(), account, headers)
	require.Equal(t, 0, repo.casCount)
	require.Equal(t, 0, repo.getCount(account.ID))
}

func TestMaybeLearnOfficialDeviceProfileDoesNotFailOnLearnError(t *testing.T) {
	account := leftoverClaudeLearnAccount(1943)
	installLeftoverOutboundProfileError(t, account.ID, fmt.Errorf("identity_reject: learn store down"))

	maybeLearnOfficialDeviceProfile(context.Background(), account, officialClaudeLearnHeaders())
}

func TestMaybeLearnOfficialDeviceProfileSkipsCodexVSCodeUA(t *testing.T) {
	account := leftoverOAuthAccount(1944, map[string]any{"device_learning_enabled": true})
	profile := leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex_cli_rs/0.1.0", "mid-vscode")
	profile.ClientVersion = "0.1.0"
	repo := installLeftoverLearnRepo(t, profile)

	headers := make(http.Header)
	headers.Set("User-Agent", "codex_vscode/0.146.0")
	headers.Set("originator", "codex_vscode")
	maybeLearnOfficialDeviceProfile(context.Background(), account, headers)
	require.Equal(t, 0, repo.casCount)
	require.Equal(t, 0, repo.getCount(account.ID))
}

func TestMaybeLearnOfficialDeviceProfileSkipsGrokOnOpenAIForwardPath(t *testing.T) {
	account := leftoverOAuthAccount(1945, map[string]any{"device_learning_enabled": true})
	account.Platform = PlatformGrok
	profile := leftoverValidProfile(account.ID, PlatformGrok, ClientFamilyGrokCLI, "xai-grok-workspace/1.0.0", "mid-grok")
	profile.ClientVersion = "0.1.0"
	repo := installLeftoverLearnRepo(t, profile)

	headers := make(http.Header)
	headers.Set("User-Agent", "xai-grok-workspace/1.2.3")
	maybeLearnOfficialDeviceProfile(context.Background(), account, headers)
	require.Equal(t, 0, repo.casCount)
	require.Equal(t, 0, repo.getCount(account.ID))
}

func TestLiveForwardsCallMaybeLearnOfficialDeviceProfile(t *testing.T) {
	for _, name := range []string{"gateway_forward.go", "openai_gateway_forward.go"} {
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		require.Contains(t, string(src), "maybeLearnOfficialDeviceProfile(", name+" must observe official inbound")
	}
}

func leftoverClaudeLearnAccount(id int64) *Account {
	return &Account{
		ID:       id,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{"device_learning_enabled": true},
	}
}

func officialClaudeLearnHeaders() http.Header {
	h := make(http.Header)
	h.Set("User-Agent", claude.DefaultHeaders["User-Agent"])
	for key, value := range claude.DefaultHeaders {
		if strings.HasPrefix(key, "X-Stainless-") {
			h.Set(key, value)
		}
	}
	return h
}

type leftoverLearnRepo struct {
	leftoverSharedDeviceRepo
	casCount int
}

func (r *leftoverLearnRepo) UpdateCAS(_ context.Context, accountID, expectedRevision int64, next *AccountDeviceProfile) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur := r.byID[accountID]
	if cur == nil || cur.Revision != expectedRevision {
		return false, nil
	}
	copied := *next
	copied.Revision = expectedRevision + 1
	r.byID[accountID] = &copied
	r.casCount++
	return true, nil
}

func installLeftoverLearnRepo(t *testing.T, p *AccountDeviceProfile) *leftoverLearnRepo {
	t.Helper()
	repo := &leftoverLearnRepo{
		leftoverSharedDeviceRepo: leftoverSharedDeviceRepo{
			byID:      map[int64]*AccountDeviceProfile{p.AccountID: p},
			errByID:   map[int64]error{},
			getCounts: map[int64]int{},
		},
	}
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(repo))
	t.Cleanup(func() {
		SetOutboundDeviceProfileService(prev)
		leftoverSharedRepo.clear(p.AccountID)
	})
	return repo
}
