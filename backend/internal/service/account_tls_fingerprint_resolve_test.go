//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResolveAccountTLSFingerprintRuntimeUsesRouterDimensions(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID:      7,
		Name:    "openai-dimension-pack",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:          "Codex CLI",
				Enabled:       true,
				MatchType:     model.TLSFingerprintRouterMatchContains,
				Pattern:       "codex_cli_rs",
				OS:            "",
				ClientType:    "codex-cli",
				CaseSensitive: false,
			},
		},
	}
	profileSvc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			101: {
				ID:         101,
				Name:       "macos-codex",
				Platform:   "openai",
				OS:         "macos",
				ClientType: "codex-cli",
				UserAgent:  "codex_cli_rs/test",
			},
		},
	}
	routerSvc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil)

	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": float64(7),
			"tls_fingerprint_bindings": map[string]any{
				"macos/codex-cli": float64(101),
			},
		},
	}

	runtime := resolveAccountTLSFingerprintRuntime(
		context.Background(),
		account,
		profileSvc,
		routerSvc,
		"codex_cli_rs/0.140.0 (Mac OS X)",
		"http",
	)
	require.True(t, runtime.Matched)
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "macos-codex", runtime.Profile.Name)
	require.Equal(t, "codex_cli_rs/test", runtime.UpstreamUserAgent)
}

func TestResolveAccountTLSFingerprintRuntimeInfersWindowsDimensionFromUA(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID:      8,
		Name:    "openai-dimension-pack",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name:       "Codex CLI",
			Enabled:    true,
			MatchType:  model.TLSFingerprintRouterMatchContains,
			Pattern:    "codex_cli_rs",
			ClientType: "codex-cli",
		}},
	}
	profileSvc := &TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{
		102: {ID: 102, Name: "windows-codex", Platform: PlatformOpenAI, OS: "windows", ClientType: "codex-cli"},
	}}
	routerSvc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil)
	account := &Account{
		ID: 43, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": float64(8),
			"tls_fingerprint_bindings":  map[string]any{"windows/codex-cli": float64(102)},
		},
	}

	runtime := resolveAccountTLSFingerprintRuntime(
		context.Background(), account, profileSvc, routerSvc,
		"codex_cli_rs/0.140.0 (Windows 11; x86_64)", "http",
	)
	require.True(t, runtime.Matched)
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "windows-codex", runtime.Profile.Name)
}

func TestResolveAccountTLSFingerprintRuntimeReadsInboundUAFromDetachedContext(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID: 9, Name: "detached", Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name: "Claude", Enabled: true, MatchType: model.TLSFingerprintRouterMatchContains,
			Pattern: "claude-cli", ClientType: "claude-code",
		}},
	}
	profileSvc := &TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{
		103: {ID: 103, Name: "linux-claude", Platform: PlatformAnthropic, OS: "linux", ClientType: "claude-code"},
	}}
	routerSvc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil)
	account := &Account{
		ID: 44, Platform: PlatformAnthropic, Type: AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": float64(9),
			"tls_fingerprint_bindings":  map[string]any{"linux/claude-code": float64(103)},
		},
	}
	ctx := WithTLSFingerprintInboundUserAgent(context.Background(), "claude-cli/2.1.0 (Linux; x86_64)")

	runtime := resolveAccountTLSFingerprintRuntime(ctx, account, profileSvc, routerSvc, "", "http")
	require.True(t, runtime.Matched)
	require.Equal(t, "linux-claude", runtime.Profile.Name)
}

func TestResolveAccountTLSFingerprintRuntimeStaticDefaultOS(t *testing.T) {
	profileSvc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			55: {
				ID:        55,
				Name:      "linux-kiro",
				Platform:  "kiro",
				OS:        "linux",
				UserAgent: "KiroIDE/static",
			},
		},
	}

	account := &Account{
		ID:       9,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_default_os": "linux",
			"tls_fingerprint_bindings": map[string]any{
				"linux/kiro-ide": float64(55),
			},
		},
	}

	runtime := resolveAccountTLSFingerprintRuntime(
		context.Background(),
		account,
		profileSvc,
		nil,
		"",
		"http",
	)
	require.False(t, runtime.Matched)
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "linux-kiro", runtime.Profile.Name)
}

func TestGatewayTLSFingerprintRuntimeCachesPerAccountAndAppliesHeaders(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID: 31, Name: "anthropic", Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{{
			Name: "claude", Enabled: true, MatchType: model.TLSFingerprintRouterMatchContains,
			Pattern: "claude-cli", TLSFingerprintProfileID: 81,
			UpstreamUserAgent: "claude-cli/routed", UpstreamOriginator: "claude-code",
		}},
	}
	svc := &GatewayService{
		tlsFPProfileService: &TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{
			81: {ID: 81, Name: "Claude Routed", Platform: PlatformAnthropic},
		}},
		tlsFPRouterService: &TLSFingerprintRouterService{localCache: map[int64]*model.TLSFingerprintRouter{31: router}},
	}
	account := &Account{ID: 100, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: map[string]any{
		"enable_tls_fingerprint": true, "tls_fingerprint_router_id": float64(31),
	}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("User-Agent", "claude-cli/2.0")

	runtime := svc.resolveGatewayTLSFingerprintRuntime(context.Background(), c, account, "http")
	require.True(t, runtime.Matched)
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "Claude Routed", runtime.Profile.Name)

	req := httptest.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer token")
	applyGatewayTLSFingerprintRuntime(req, runtime)
	require.Equal(t, "claude-cli/routed", req.Header.Get("User-Agent"))
	require.Equal(t, "claude-code", req.Header.Get("Originator"))
	require.Equal(t, "Bearer token", req.Header.Get("Authorization"))

	router.Rules[0].UpstreamUserAgent = "changed-after-first-attempt"
	cached := svc.resolveGatewayTLSFingerprintRuntime(context.Background(), c, account, "http")
	require.Equal(t, "claude-cli/routed", cached.UpstreamUserAgent, "retries must reuse the first resolved runtime")
}
