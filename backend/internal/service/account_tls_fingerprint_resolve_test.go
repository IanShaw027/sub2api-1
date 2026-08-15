//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/stretchr/testify/require"
)

func testTLSProfileService(profiles ...*model.TLSFingerprintProfile) *TLSFingerprintProfileService {
	svc := &TLSFingerprintProfileService{localCache: make(map[int64]*model.TLSFingerprintProfile)}
	for _, profile := range profiles {
		if profile != nil {
			svc.localCache[profile.ID] = profile
		}
	}
	return svc
}

func testTLSRouterService(routers ...*model.TLSFingerprintRouter) *TLSFingerprintRouterService {
	svc := &TLSFingerprintRouterService{}
	svc.setLocalCache(routers)
	return svc
}

func enabledTLSAccount(platform string, extra map[string]any) *Account {
	return &Account{
		ID:       7,
		Platform: platform,
		Type:     AccountTypeOAuth,
		Extra:    extra,
	}
}

func TestIsTLSFingerprintEnabled_AllPlatforms(t *testing.T) {
	platforms := []string{
		PlatformAnthropic, PlatformOpenAI, PlatformGemini,
		PlatformAntigravity, PlatformGrok, PlatformKiro,
	}
	for _, platform := range platforms {
		account := enabledTLSAccount(platform, map[string]any{"enable_tls_fingerprint": true})
		require.True(t, account.IsTLSFingerprintEnabled(), platform)
	}
	require.False(t, enabledTLSAccount(PlatformOpenAI, nil).IsTLSFingerprintEnabled())
	require.False(t, enabledTLSAccount(PlatformComposite, map[string]any{"enable_tls_fingerprint": true}).IsTLSFingerprintEnabled())
}

func TestLookupTLSFingerprintBinding_AndOrUnique(t *testing.T) {
	bindings := map[string]int64{
		"macos/codex-cli@responses": 11,
		"macos/codex-cli":           12,
		"macos":                     13,
		"responses":                 14,
	}

	require.Equal(t, int64(11), lookupTLSFingerprintBinding(bindings, "macos", "codex-cli", "responses"))
	require.Equal(t, int64(12), lookupTLSFingerprintBinding(bindings, "macos", "codex-cli", "messages"))
	require.Equal(t, int64(13), lookupTLSFingerprintBinding(bindings, "macos", "", "chat_completions"))
	require.Equal(t, int64(14), lookupTLSFingerprintBinding(bindings, "", "", "responses"))

	unique := map[string]int64{"*": 99}
	require.Equal(t, int64(99), lookupTLSFingerprintBinding(unique, "linux", "claude-code", "messages"))
}

func TestTLSFingerprintRouter_MatchConditions(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID:      3,
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "macos-codex-responses",
				Enabled:                 true,
				OS:                      "macos",
				ClientType:              "codex-cli",
				Protocol:                "responses",
				TLSFingerprintProfileID: 21,
			},
			{
				Name:                    "protocol-only",
				Enabled:                 true,
				Protocol:                "chat_completions",
				TLSFingerprintProfileID: 22,
			},
			{
				Name:                    "unique",
				Enabled:                 true,
				TLSFingerprintProfileID: 23,
			},
		},
	}
	svc := testTLSRouterService(router)
	ctx := context.Background()

	macosCodex := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Codex/1.0"
	match, ok := svc.MatchRequest(ctx, 3, PlatformOpenAI, macosCodex, "http", "responses")
	require.True(t, ok)
	require.Equal(t, int64(21), match.ProfileID)

	match, ok = svc.MatchRequest(ctx, 3, PlatformOpenAI, "curl/8.0", "http", "chat_completions")
	require.True(t, ok)
	require.Equal(t, int64(22), match.ProfileID)

	match, ok = svc.MatchRequest(ctx, 3, PlatformOpenAI, "curl/8.0", "http", "messages")
	require.True(t, ok)
	require.Equal(t, int64(23), match.ProfileID)
}

func TestResolveAccountTLSFingerprintRuntime_UniqueAndDimensions(t *testing.T) {
	profiles := testTLSProfileService(
		&model.TLSFingerprintProfile{ID: 21, Name: "macos-codex"},
		&model.TLSFingerprintProfile{ID: 23, Name: "unique"},
		&model.TLSFingerprintProfile{ID: 31, Name: "bound-macos"},
	)
	router := testTLSRouterService(&model.TLSFingerprintRouter{
		ID:      3,
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name:                    "unique",
			Enabled:                 true,
			TLSFingerprintProfileID: 23,
		}},
	})
	ctx := context.Background()

	uniqueAccount := enabledTLSAccount(PlatformOpenAI, map[string]any{
		"enable_tls_fingerprint":     true,
		"tls_fingerprint_profile_id": int64(23),
	})
	runtime := resolveAccountTLSFingerprintRuntime(ctx, uniqueAccount, profiles, nil, "", "http", "responses")
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "unique", runtime.Profile.Name)

	boundAccount := enabledTLSAccount(PlatformAnthropic, map[string]any{
		"enable_tls_fingerprint": true,
		"tls_fingerprint_bindings": map[string]any{
			"macos": int64(31),
		},
		"tls_fingerprint_default_os": "macos",
	})
	runtime = resolveAccountTLSFingerprintRuntime(ctx, boundAccount, profiles, nil, "", "http", "messages")
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "bound-macos", runtime.Profile.Name)

	routedAccount := enabledTLSAccount(PlatformGemini, map[string]any{
		"enable_tls_fingerprint":    true,
		"tls_fingerprint_router_id": int64(3),
	})
	runtime = resolveAccountTLSFingerprintRuntime(ctx, routedAccount, profiles, router, "curl/8.0", "http", "gemini")
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "unique", runtime.Profile.Name)
	require.True(t, runtime.Matched)
}

func TestResolveAccountTLSFingerprintRuntime_UsesAccountOSNotInboundUA(t *testing.T) {
	profiles := testTLSProfileService(
		&model.TLSFingerprintProfile{ID: 31, Name: "bound-macos"},
		&model.TLSFingerprintProfile{ID: 32, Name: "bound-windows"},
	)
	account := enabledTLSAccount(PlatformAnthropic, map[string]any{
		"enable_tls_fingerprint": true,
		"tls_fingerprint_bindings": map[string]any{
			"macos/claude-code":   int64(31),
			"windows/claude-code": int64(32),
		},
		"tls_fingerprint_default_os": "macos",
	})

	runtime := resolveAccountTLSFingerprintRuntime(
		context.Background(),
		account,
		profiles,
		nil,
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) claude-cli/2.1.220",
		"http",
		"messages",
	)

	require.NotNil(t, runtime.Profile)
	require.Equal(t, "bound-macos", runtime.Profile.Name)
}

func TestResolveAccountTLSFingerprintRuntime_UsesAccountClientFamilyNotInboundUA(t *testing.T) {
	profiles := testTLSProfileService(
		&model.TLSFingerprintProfile{ID: 41, Name: "macos-codex"},
		&model.TLSFingerprintProfile{ID: 42, Name: "macos-generic"},
	)
	account := enabledTLSAccount(PlatformOpenAI, map[string]any{
		"enable_tls_fingerprint": true,
		"tls_fingerprint_bindings": map[string]any{
			"macos/codex-cli": int64(41),
			"macos":           int64(42),
		},
		"tls_fingerprint_default_os": "macos",
	})

	emptyRuntime := resolveAccountTLSFingerprintRuntime(
		context.Background(),
		account,
		profiles,
		nil,
		"",
		"http",
		"responses",
	)
	windowsRuntime := resolveAccountTLSFingerprintRuntime(
		context.Background(),
		account,
		profiles,
		nil,
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) codex_cli_rs/0.200.1",
		"http",
		"responses",
	)

	require.NotNil(t, emptyRuntime.Profile)
	require.NotNil(t, windowsRuntime.Profile)
	require.Equal(t, "macos-codex", emptyRuntime.Profile.Name)
	require.Equal(t, emptyRuntime.Profile.Name, windowsRuntime.Profile.Name)
}

func TestResolveAccountTLSFingerprintRuntime_RejectsRandomProfileID(t *testing.T) {
	profiles := testTLSProfileService(&model.TLSFingerprintProfile{ID: 31, Name: "random-candidate"})
	account := enabledTLSAccount(PlatformOpenAI, map[string]any{
		"enable_tls_fingerprint":     true,
		"tls_fingerprint_profile_id": int64(-1),
	})

	runtime := resolveAccountTLSFingerprintRuntime(context.Background(), account, profiles, nil, "", "http", "responses")

	require.NotNil(t, runtime.Profile)
	require.NotEqual(t, "random-candidate", runtime.Profile.Name)
}

func TestApplyTLSFingerprintRuntimeHeaders_UsesLowercaseOriginator(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.test", nil)
	require.NoError(t, err)

	applyTLSFingerprintRuntimeHeaders(req, accountTLSFingerprintRuntime{
		UpstreamUserAgent:  "codex_cli_rs/0.200.1",
		UpstreamOriginator: "codex_cli_rs",
	})

	require.Equal(t, "codex_cli_rs/0.200.1", req.Header.Get("User-Agent"))
	require.Equal(t, "codex_cli_rs", getHeaderRaw(req.Header, "originator"))
	require.Contains(t, req.Header, "originator")
	require.Empty(t, req.Header.Get("X-Originator"))
	_, hasXOriginator := req.Header["X-Originator"]
	require.False(t, hasXOriginator)
}

func TestResolveAccountTLSFingerprintRuntime_StampsDeviceTransportFamily(t *testing.T) {
	for _, tc := range []struct {
		name     string
		platform string
		device   *AccountDeviceProfile
	}{
		{name: "codex", platform: PlatformOpenAI, device: validCodexBaseline()},
		{name: "kiro", platform: PlatformKiro, device: validKiroBaseline()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.device.TransportFamily = TransportH2
			prev := OutboundDeviceProfileService()
			SetOutboundDeviceProfileService(NewAccountDeviceService(&stampDeviceProfileRepo{profile: tc.device}))
			t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

			account := enabledTLSAccount(tc.platform, map[string]any{
				"enable_tls_fingerprint":     true,
				"tls_fingerprint_profile_id": int64(77),
			})
			account.ID = tc.device.AccountID
			profiles := testTLSProfileService(&model.TLSFingerprintProfile{
				ID:            77,
				Name:          "runtime-" + tc.name,
				ALPNProtocols: []string{"h2", "http/1.1"},
			})

			runtime := resolveAccountTLSFingerprintRuntime(context.Background(), account, profiles, nil, "", "http", "responses")
			require.NotNil(t, runtime.Profile)
			require.Equal(t, TransportH2, runtime.Profile.TransportFamily, "device-profile family must be stamped after ToTLSProfile")
			require.Equal(t, []string{"h2", "http/1.1"}, runtime.Profile.ALPNProtocols)
		})
	}
}

type stampDeviceProfileRepo struct {
	profile *AccountDeviceProfile
}

func (r *stampDeviceProfileRepo) GetByAccountID(context.Context, int64) (*AccountDeviceProfile, error) {
	return r.profile, nil
}

func (r *stampDeviceProfileRepo) InsertBaseline(_ context.Context, p *AccountDeviceProfile) (*AccountDeviceProfile, error) {
	return p, nil
}

func (r *stampDeviceProfileRepo) UpdateCAS(context.Context, int64, int64, *AccountDeviceProfile) (bool, error) {
	return false, nil
}

func (r *stampDeviceProfileRepo) DeleteByAccountID(context.Context, int64) error {
	return nil
}

func TestResolveTLSProfile_RandomProfilePreservesCanonicalID(t *testing.T) {
	profiles := testTLSProfileService(
		&model.TLSFingerprintProfile{ID: 101, Name: "shared-profile"},
	)
	account := enabledTLSAccount(PlatformOpenAI, map[string]any{
		"enable_tls_fingerprint":     true,
		"tls_fingerprint_profile_id": int64(-1),
	})

	profile := profiles.ResolveTLSProfile(account)
	require.NotNil(t, profile)
	require.Equal(t, int64(101), profile.ID)
	require.Equal(t, "shared-profile", profile.Name)
}

func TestTLSFingerprintRouter_RegexMatchUsesCompiledPattern(t *testing.T) {
	svc := testTLSRouterService(&model.TLSFingerprintRouter{
		ID:      4,
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name:                    "re",
			Enabled:                 true,
			MatchType:               model.TLSFingerprintRouterMatchRegex,
			Pattern:                 `Codex/\d+`,
			TLSFingerprintProfileID: 21,
		}},
	})
	ctx := context.Background()

	match, ok := svc.MatchRequest(ctx, 4, PlatformOpenAI, "Codex/12", "http", "responses")
	require.True(t, ok)
	require.Equal(t, int64(21), match.ProfileID)

	_, ok = svc.MatchRequest(ctx, 4, PlatformOpenAI, "curl/8", "http", "responses")
	require.False(t, ok)
}

func TestTLSFingerprintRouter_MissingIDDoesNotRefreshWhenReady(t *testing.T) {
	repo := &countingTLSRouterRepo{routers: []*model.TLSFingerprintRouter{{
		ID:      3,
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name:                    "unique",
			Enabled:                 true,
			TLSFingerprintProfileID: 1,
		}},
	}}}
	svc := NewTLSFingerprintRouterService(repo, nil)
	require.Equal(t, 1, repo.listCalls)

	_, ok := svc.MatchRequest(context.Background(), 99, PlatformOpenAI, "curl/8", "http", "responses")
	require.False(t, ok)
	require.Equal(t, 1, repo.listCalls)
}

func TestResolveAccountTLSFingerprintRuntime_UpstreamRewriteWithoutProfile(t *testing.T) {
	profiles := testTLSProfileService()
	router := testTLSRouterService(&model.TLSFingerprintRouter{
		ID:      5,
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name:               "protocol-only",
			Enabled:            true,
			Protocol:           "responses",
			UpstreamUserAgent:  "rewritten-ua",
			UpstreamOriginator: "rewritten-originator",
		}},
	})
	account := enabledTLSAccount(PlatformOpenAI, map[string]any{
		"enable_tls_fingerprint":    true,
		"tls_fingerprint_router_id": int64(5),
	})
	runtime := resolveAccountTLSFingerprintRuntime(context.Background(), account, profiles, router, "Codex/1", "http", "responses")
	require.True(t, runtime.Matched)
	require.Equal(t, "rewritten-ua", runtime.UpstreamUserAgent)
	require.Equal(t, "rewritten-originator", runtime.UpstreamOriginator)
}

func TestResolveAccountTLSFingerprintRuntime_RouterDoesNotDependOnInboundUA(t *testing.T) {
	profiles := testTLSProfileService(
		&model.TLSFingerprintProfile{ID: 51, Name: "ua-specific"},
		&model.TLSFingerprintProfile{ID: 52, Name: "stable"},
	)
	router := testTLSRouterService(&model.TLSFingerprintRouter{
		ID:      6,
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "codex-only",
				Enabled:                 true,
				Pattern:                 "Codex",
				TLSFingerprintProfileID: 51,
				UpstreamUserAgent:       "codex-upstream",
				UpstreamOriginator:      "codex-originator",
			},
			{
				Name:                    "fallback",
				Enabled:                 true,
				TLSFingerprintProfileID: 52,
				UpstreamUserAgent:       "stable-upstream",
				UpstreamOriginator:      "stable-originator",
			},
		},
	})
	account := enabledTLSAccount(PlatformOpenAI, map[string]any{
		"enable_tls_fingerprint":    true,
		"tls_fingerprint_router_id": int64(6),
	})

	codexRuntime := resolveAccountTLSFingerprintRuntime(
		context.Background(),
		account,
		profiles,
		router,
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Codex/1.0",
		"http",
		"responses",
	)
	curlRuntime := resolveAccountTLSFingerprintRuntime(
		context.Background(),
		account,
		profiles,
		router,
		"curl/8.0",
		"http",
		"responses",
	)

	require.True(t, codexRuntime.Matched)
	require.True(t, curlRuntime.Matched)
	require.NotNil(t, codexRuntime.Profile)
	require.NotNil(t, curlRuntime.Profile)
	require.Equal(t, "stable", codexRuntime.Profile.Name)
	require.Equal(t, codexRuntime.Profile.Name, curlRuntime.Profile.Name)
	require.Equal(t, codexRuntime.UpstreamUserAgent, curlRuntime.UpstreamUserAgent)
	require.Equal(t, codexRuntime.UpstreamOriginator, curlRuntime.UpstreamOriginator)
}

func TestInboundProtocolFromPath_AntigravityBeforeMessages(t *testing.T) {
	require.Equal(t, "antigravity", inboundProtocolFromPath("/antigravity/v1/messages"))
	require.Equal(t, "messages", inboundProtocolFromPath("/v1/messages"))
	require.Equal(t, "kiro", inboundProtocolFromPath("/kiro/v1/messages"))
	require.Equal(t, "chat_completions", inboundProtocolFromPath("/v1/chat/completions"))
}

type countingTLSRouterRepo struct {
	listCalls int
	routers   []*model.TLSFingerprintRouter
}

func (r *countingTLSRouterRepo) List(context.Context) ([]*model.TLSFingerprintRouter, error) {
	r.listCalls++
	return r.routers, nil
}

func (r *countingTLSRouterRepo) GetByID(context.Context, int64) (*model.TLSFingerprintRouter, error) {
	return nil, nil
}

func (r *countingTLSRouterRepo) Create(_ context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	return router, nil
}

func (r *countingTLSRouterRepo) Update(_ context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	return router, nil
}

func (r *countingTLSRouterRepo) Delete(context.Context, int64) error {
	return nil
}
