package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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

	err = applyGrokUpstreamHeadersFromAccount(context.Background(), req, &Account{ID: 7, Platform: PlatformGrok})
	require.NoError(t, err)
	require.Equal(t, "xai-grok-workspace/0.2.200", req.Header.Get("User-Agent"))
	require.Equal(t, "0.2.200", req.Header.Get("x-grok-client-version"))
	require.Equal(t, "grok-shell-learned", req.Header.Get("x-grok-client-identifier"))
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
	require.Empty(t, req.Header.Get("x-grok-foo"))
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
