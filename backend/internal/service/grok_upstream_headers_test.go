package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

func TestApplyDefaultGrokUpstreamHeadersUsesCLIUserAgent(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	req, err := http.NewRequest(http.MethodGet, "https://api.x.ai/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "claude-code/1.2.3")
	req.Header.Set("x-grok-client-version", "none")

	applyDefaultGrokUpstreamHeaders(req)

	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
	require.Equal(t, xai.CLIClientVersion, req.Header.Get("x-grok-client-version"))
	require.Equal(t, xai.CLIClientIdentifier, req.Header.Get("x-grok-client-identifier"))
}

func TestApplyDefaultGrokUpstreamHeadersHonorsCLIVersionOverride(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "0.2.95")

	req, err := http.NewRequest(http.MethodGet, "https://api.x.ai/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "codex_cli_rs/0.144.0")

	applyDefaultGrokUpstreamHeaders(req)

	require.Equal(t, "0.2.95", req.Header.Get("x-grok-client-version"))
	require.Equal(t, xai.CLIUserAgent("0.2.95"), req.Header.Get("User-Agent"))
	require.Equal(t, "grok-shell", req.Header.Get("x-grok-client-identifier"))
}

func TestResolveGrokUpstreamUserAgentNeverPassthrough(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "claude-cli/2.0.0 (Mac OS; arm64)")

	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), resolveGrokUpstreamUserAgent(c))
	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), resolveGrokUpstreamUserAgent(nil))
}

func TestApplyGrokRuntimeHeadersKeepsCLIUserAgent(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "claude-code/9.9.9")

	applyGrokRuntimeHeaders(req, openAITLSFingerprintRuntime{
		UpstreamUserAgent:  "codex_cli_rs/0.144.0",
		UpstreamOriginator: "codex_cli_rs",
	})

	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
	require.Equal(t, "codex_cli_rs", req.Header.Get("Originator"))
	require.Equal(t, xai.CLIClientVersion, req.Header.Get("x-grok-client-version"))
}

func TestApplyGrokTLSProfileHeadersAlwaysUsesCLIUserAgent(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	req, err := http.NewRequest(http.MethodPost, "https://api.x.ai/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "grok-native/1.0")

	// HEAD Profile is TLS-only; Originator/UserAgent HTTP fields are not present.
	applyGrokTLSProfileHeaders(req, &tlsfingerprint.Profile{Name: "chrome"})

	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
	require.Equal(t, xai.CLIClientVersion, req.Header.Get("x-grok-client-version"))
}

func TestApplyGrokUpstreamHeadersFromAccountUsesProfileIdentity(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	profile := validGrokOutboundProfile()
	profile.ClientVersion = "0.2.200"
	profile.ProfilePayload["user_agent"] = "xai-grok-workspace/0.2.200"
	profile.ProfilePayload["grok_identifier"] = "grok-shell-learned"
	injectOutboundGrokProfile(t, profile)

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "claude-cli/2.0.0 (Mac OS; arm64)")
	req.Header.Set("x-grok-client-version", "inbound-claude")
	req.Header.Set("x-grok-client-identifier", "inbound-codex")

	err = applyGrokInteractiveUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 7, Platform: PlatformGrok})
	require.NoError(t, err)
	require.Equal(t, "xai-grok-workspace/0.2.200", req.Header.Get("User-Agent"))
	require.Equal(t, "0.2.200", req.Header.Get("x-grok-client-version"))
	require.Equal(t, "grok-shell-learned", req.Header.Get("x-grok-client-identifier"))
	require.Equal(t, grokClientModeInteractive, req.Header.Get("x-grok-client-mode"))
	require.Equal(t, grokClientModeInteractive, req.Header.Get("X-Grok-Client-Mode"))
	require.NotEqual(t, "claude-cli/2.0.0 (Mac OS; arm64)", req.Header.Get("User-Agent"))
	require.NotEqual(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
}

func TestApplyGrokUpstreamHeadersFromAccountLoadFailureDoesNotStampMixedBundle(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")
	injectOutboundGrokProfileError(t, errors.New("identity_reject: profile load failed"))

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "claude-cli/2.0.0 (Mac OS; arm64)")
	req.Header.Set("x-grok-client-version", "inbound-mixed")
	req.Header.Set("x-grok-client-identifier", "inbound-id")

	err = applyGrokUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 7, Platform: PlatformGrok})
	require.Error(t, err)
	require.Equal(t, "claude-cli/2.0.0 (Mac OS; arm64)", req.Header.Get("User-Agent"))
	require.Equal(t, "inbound-mixed", req.Header.Get("x-grok-client-version"))
	require.Equal(t, "inbound-id", req.Header.Get("x-grok-client-identifier"))
	require.NotEqual(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
}

func TestApplyGrokUpstreamHeadersFromAccountDropsUnknownXGrokHeaders(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")
	injectOutboundGrokProfile(t, validGrokOutboundProfile())

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "codex_cli_rs/0.144.0")
	req.Header.Set("x-grok-foo", "client-invented")
	req.Header.Set(openCodeSessionAffinityHeader, "opencode-affinity")
	req.Header.Set("chatgpt-account-id", "acc-123")
	req.Header.Set("X-Account-Id", "99")

	err = applyGrokUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 7, Platform: PlatformGrok})
	require.NoError(t, err)
	require.Empty(t, req.Header.Get("x-grok-foo"))
	require.Empty(t, req.Header.Values("x-grok-foo"))
	require.Empty(t, req.Header.Get(openCodeSessionAffinityHeader))
	require.Empty(t, req.Header.Get("chatgpt-account-id"))
	require.Empty(t, req.Header.Get("X-Account-Id"))
}

func TestApplyGrokUpstreamHeadersFromAccountMissingUAUsesProfileVersion(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	profile := validGrokOutboundProfile()
	profile.ClientVersion = "0.2.200"
	delete(profile.ProfilePayload, "user_agent")
	injectOutboundGrokProfile(t, profile)

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)

	err = applyGrokUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 7, Platform: PlatformGrok})
	require.NoError(t, err)
	require.Equal(t, "0.2.200", req.Header.Get("x-grok-client-version"))
	require.Equal(t, xai.CLIUserAgent("0.2.200"), req.Header.Get("User-Agent"))
	require.NotEqual(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
}

func TestApplyGrokUpstreamHeadersFromAccountDropsGatewaySessionAffinityHeaders(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")
	injectOutboundGrokProfile(t, validGrokOutboundProfile())

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("session_id", "sess-1")
	req.Header.Set("conversation_id", "conv-1")
	req.Header.Set("X-Conversation-ID", "xconv-1")

	err = applyGrokUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 7, Platform: PlatformGrok})
	require.NoError(t, err)
	require.Empty(t, req.Header.Get("session_id"))
	require.Empty(t, req.Header.Get("conversation_id"))
	require.Empty(t, req.Header.Get("X-Conversation-ID"))
}

func TestApplyGrokUpstreamHeadersFromAccountStampsClientMode(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")
	injectOutboundGrokProfile(t, validGrokOutboundProfile())

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("x-grok-client-mode", "inbound-invented")

	err = applyGrokUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 7, Platform: PlatformGrok})
	require.NoError(t, err)
	require.Equal(t, grokClientModeHeader, req.Header.Get("x-grok-client-mode"))
	require.NotEqual(t, "inbound-invented", req.Header.Get("x-grok-client-mode"))
}

func TestApplyGrokUpstreamHeadersFromAccountNilAccountStampsPinnedCLIUA(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "codex_cli_rs/0.144.0")
	req.Header.Set("x-grok-foo", "client-invented")

	err = applyGrokUpstreamHeadersFromAccount(context.Background(), req, nil)
	require.NoError(t, err)
	require.Equal(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
	require.Equal(t, xai.CLIClientVersion, req.Header.Get("x-grok-client-version"))
	require.Equal(t, xai.CLIClientIdentifier, req.Header.Get("x-grok-client-identifier"))
	require.Equal(t, xai.CLIClientMode, req.Header.Get("x-grok-client-mode"))
	require.Empty(t, req.Header.Get("x-grok-foo"))
}

func TestApplyGrokInteractiveUpstreamHeadersFromAccountFailsClosedOnProfileLoadError(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")
	injectOutboundGrokProfileError(t, errors.New("identity_reject: profile load failed"))

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "claude-cli/2.0.0 (Mac OS; arm64)")
	req.Header.Set("x-grok-client-version", "inbound-mixed")
	req.Header.Set("x-grok-client-mode", "inbound-invented")

	err = applyGrokInteractiveUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 7, Platform: PlatformGrok})
	require.Error(t, err)
	require.Contains(t, err.Error(), "identity_reject")
	require.Equal(t, "claude-cli/2.0.0 (Mac OS; arm64)", req.Header.Get("User-Agent"))
	require.Equal(t, "inbound-mixed", req.Header.Get("x-grok-client-version"))
	require.Equal(t, "inbound-invented", req.Header.Get("x-grok-client-mode"))
	require.NotEqual(t, xai.CLIUserAgent(xai.CLIClientVersion), req.Header.Get("User-Agent"))
}

func TestStampGrokCLIIdentityModeSplit(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	cliReq, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	stampGrokCLIIdentity(cliReq, nil, xai.CLIClientMode)
	require.Equal(t, xai.CLIClientMode, cliReq.Header.Get("x-grok-client-mode"))

	interactiveReq, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	stampGrokCLIIdentity(interactiveReq, nil, grokClientModeInteractive)
	require.Equal(t, grokClientModeInteractive, interactiveReq.Header.Get("x-grok-client-mode"))
	require.Equal(t, grokClientModeInteractive, interactiveReq.Header.Get("X-Grok-Client-Mode"))
}

func TestApplyGrokUpstreamHeadersFromAccountRejectsNonGrokProfile(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	foreign := leftoverValidProfile(7, PlatformKiro, ClientFamilyKiroIDE, "KiroIDE/0.10.0", "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee")
	foreign.ClientVersion = "0.10.0"
	injectOutboundGrokRepo(t, &conflictGrokDeviceProfileRepo{profile: foreign})

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "claude-cli/2.0.0 (Mac OS; arm64)")
	req.Header.Set("x-grok-client-version", "inbound-mixed")

	err = applyGrokInteractiveUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 7, Platform: PlatformGrok})
	require.Error(t, err)
	require.Contains(t, err.Error(), "identity_reject")
	require.Contains(t, err.Error(), PlatformKiro)
	require.Contains(t, err.Error(), "account_id=7")
	require.Equal(t, "claude-cli/2.0.0 (Mac OS; arm64)", req.Header.Get("User-Agent"))
	require.Equal(t, "inbound-mixed", req.Header.Get("x-grok-client-version"))
	require.NotEqual(t, "0.10.0", req.Header.Get("x-grok-client-version"))
	require.NotEqual(t, "KiroIDE/0.10.0", req.Header.Get("User-Agent"))
}

func TestApplyGrokUpstreamHeadersFromAccountRemintsLeftoverForeignProfile(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")

	foreign := leftoverValidProfile(900007, PlatformKiro, ClientFamilyKiroIDE, "KiroIDE/0.10.0", "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee")
	foreign.ClientVersion = "0.10.0"
	installLeftoverOutboundProfile(t, foreign)

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "claude-cli/2.0.0 (Mac OS; arm64)")
	req.Header.Set("x-grok-client-version", "inbound-mixed")

	err = applyGrokInteractiveUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 900007, Platform: PlatformGrok})
	require.NoError(t, err)
	require.NotEqual(t, "KiroIDE/0.10.0", req.Header.Get("User-Agent"))
	require.NotEqual(t, "0.10.0", req.Header.Get("x-grok-client-version"))
	require.Equal(t, grokClientModeInteractive, req.Header.Get("x-grok-client-mode"))
	require.True(t, strings.HasPrefix(req.Header.Get("User-Agent"), "xai-grok-workspace/"))
}

func TestApplyGrokUpstreamHeadersFromAccountTimesOutProfileLoad(t *testing.T) {
	t.Setenv(xai.CLIVersionEnv, "")
	injectOutboundGrokRepo(t, &hangingGrokDeviceProfileRepo{})

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "claude-cli/2.0.0 (Mac OS; arm64)")
	req.Header.Set("x-grok-client-version", "inbound-mixed")

	done := make(chan error, 1)
	go func() {
		done <- applyGrokInteractiveUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 7, Platform: PlatformGrok})
	}()

	select {
	case err := <-done:
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, "claude-cli/2.0.0 (Mac OS; arm64)", req.Header.Get("User-Agent"))
		require.Equal(t, "inbound-mixed", req.Header.Get("x-grok-client-version"))
	case <-time.After(outboundDeviceProfileLoadTimeout + time.Second):
		t.Fatal("profile load did not bound to outboundDeviceProfileLoadTimeout")
	}
}

func validGrokOutboundProfile() *AccountDeviceProfile {
	return &AccountDeviceProfile{
		AccountID:          7,
		Revision:           1,
		SchemaVersion:      1,
		Platform:           PlatformGrok,
		ClientFamily:       ClientFamilyGrokCLI,
		InstallationID:     "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		DeviceID:           "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		MachineID:          "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		GatewayAccountUUID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
		SessionNamespace:   "0123456789abcdef0123456789abcdef",
		OSFamily:           "macos",
		Arch:               "arm64",
		Runtime:            "grok-shell",
		RuntimeVersion:     xai.CLIClientVersion,
		ClientVersion:      xai.CLIClientVersion,
		TransportFamily:    TransportH1,
		ProfilePayload: map[string]any{
			"user_agent":      xai.CLIUserAgent(xai.CLIClientVersion),
			"grok_token_auth": xai.CLITokenAuth,
			"grok_identifier": xai.CLIClientIdentifier,
		},
		LearnedFrom: LearnedFromBaseline,
	}
}

func injectOutboundGrokProfile(t *testing.T, profile *AccountDeviceProfile) {
	t.Helper()
	injectOutboundGrokRepo(t, &fixedGrokDeviceProfileRepo{profile: profile})
}

func injectOutboundGrokProfileError(t *testing.T, err error) {
	t.Helper()
	injectOutboundGrokRepo(t, &fixedGrokDeviceProfileRepo{err: err})
}

func injectOutboundGrokRepo(t *testing.T, repo AccountDeviceProfileRepository) {
	t.Helper()
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(repo))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })
}

type fixedGrokDeviceProfileRepo struct {
	profile *AccountDeviceProfile
	err     error
}

func (r *fixedGrokDeviceProfileRepo) GetByAccountID(_ context.Context, _ int64) (*AccountDeviceProfile, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.profile, nil
}

func (r *fixedGrokDeviceProfileRepo) InsertBaseline(_ context.Context, p *AccountDeviceProfile) (*AccountDeviceProfile, error) {
	if r.err != nil {
		return nil, r.err
	}
	return p, nil
}

func (r *fixedGrokDeviceProfileRepo) UpdateCAS(_ context.Context, _, _ int64, _ *AccountDeviceProfile) (bool, error) {
	return false, r.err
}

func (r *fixedGrokDeviceProfileRepo) DeleteByAccountID(context.Context, int64) error {
	return r.err
}

type hangingGrokDeviceProfileRepo struct{}

func (r *hangingGrokDeviceProfileRepo) GetByAccountID(ctx context.Context, _ int64) (*AccountDeviceProfile, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (r *hangingGrokDeviceProfileRepo) InsertBaseline(context.Context, *AccountDeviceProfile) (*AccountDeviceProfile, error) {
	return nil, errors.New("insert unused")
}

func (r *hangingGrokDeviceProfileRepo) UpdateCAS(context.Context, int64, int64, *AccountDeviceProfile) (bool, error) {
	return false, nil
}

func (r *hangingGrokDeviceProfileRepo) DeleteByAccountID(context.Context, int64) error {
	return nil
}

type conflictGrokDeviceProfileRepo struct {
	profile *AccountDeviceProfile
}

func (r *conflictGrokDeviceProfileRepo) GetByAccountID(context.Context, int64) (*AccountDeviceProfile, error) {
	return r.profile, nil
}

func (r *conflictGrokDeviceProfileRepo) InsertBaseline(context.Context, *AccountDeviceProfile) (*AccountDeviceProfile, error) {
	return nil, errors.New("duplicate account device profile")
}

func (r *conflictGrokDeviceProfileRepo) UpdateCAS(context.Context, int64, int64, *AccountDeviceProfile) (bool, error) {
	return false, nil
}

func (r *conflictGrokDeviceProfileRepo) DeleteByAccountID(context.Context, int64) error {
	return nil
}
