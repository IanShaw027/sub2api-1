//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAITLSFingerprintRuntimeUsesRouterMatch(t *testing.T) {
	setGinTestMode()
	router := &model.TLSFingerprintRouter{
		ID:      10,
		Name:    "openai clients",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "cursor",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Cursor/",
				TLSFingerprintProfileID: 7,
				UpstreamUserAgent:       "codex_cli_rs/0.125.0",
				UpstreamOriginator:      "codex_cli_rs",
			},
		},
	}
	svc := &OpenAIGatewayService{
		tlsFPRouterService: NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{
			localCache: map[int64]*model.TLSFingerprintProfile{
				7: {
					ID:            7,
					Name:          "Chrome Routed",
					ALPNProtocols: []string{"h2", "http/1.1"},
				},
			},
		},
	}
	req, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "Cursor/1.2.3")
	req.Header.Set("Originator", "codex_cli_rs")
	c := &gin.Context{Request: req}
	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"enable_tls_fingerprint": true, "tls_fingerprint_router_id": float64(10)},
	}

	runtime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), c, account, "")

	require.True(t, runtime.Matched)
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "Chrome Routed", runtime.Profile.Name)

	upstreamReq, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/responses", nil)
	require.NoError(t, err)
	upstreamReq.Header.Set("User-Agent", "original-client/1.0")
	applyOpenAITLSFingerprintRuntime(upstreamReq, runtime)
	require.Equal(t, "codex_cli_rs/0.125.0", upstreamReq.Header.Get("User-Agent"))
	require.Equal(t, "codex_cli_rs", upstreamReq.Header.Get("Originator"))
}

func TestOpenAITLSFingerprintRuntimeUsesAccountProfileWhenPlatformAntiBanDisabled(t *testing.T) {
	SetRuntimeAntiBanPlatforms(map[string]bool{PlatformOpenAI: false})
	t.Cleanup(func() { SetRuntimeAntiBanPlatforms(map[string]bool{}) })
	svc := &OpenAIGatewayService{
		tlsFPProfileService: &TLSFingerprintProfileService{
			localCache: map[int64]*model.TLSFingerprintProfile{
				7: {
					ID:         7,
					Name:       "Account Profile",
					Platform:   "openai",
					UserAgent:  "profile-ua/1.0",
					Originator: "profile-originator",
				},
			},
		},
	}
	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": float64(7),
		},
	}

	runtime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), nil, account, "http")

	require.False(t, runtime.Matched)
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "Account Profile", runtime.Profile.Name)
	require.Equal(t, "profile-ua/1.0", runtime.UpstreamUserAgent)
	require.Equal(t, "profile-originator", runtime.UpstreamOriginator)
}

func TestGrokTLSFingerprintRuntimeUsesAccountProfileWhenPlatformAntiBanDisabled(t *testing.T) {
	SetRuntimeAntiBanPlatforms(map[string]bool{PlatformGrok: false})
	t.Cleanup(func() { SetRuntimeAntiBanPlatforms(map[string]bool{}) })
	svc := &OpenAIGatewayService{
		tlsFPProfileService: &TLSFingerprintProfileService{
			localCache: map[int64]*model.TLSFingerprintProfile{
				8: {
					ID:         8,
					Name:       "Grok Account Profile",
					Platform:   "grok",
					UserAgent:  "grok-profile-ua/1.0",
					Originator: "grok-originator",
				},
			},
		},
	}
	account := &Account{
		ID:       2,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": float64(8),
		},
	}

	runtime := svc.resolveGrokTLSFingerprintRuntime(context.Background(), nil, account, "http")

	require.False(t, runtime.Matched)
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "Grok Account Profile", runtime.Profile.Name)
	require.Equal(t, "grok-profile-ua/1.0", runtime.UpstreamUserAgent)
	require.Equal(t, "grok-originator", runtime.UpstreamOriginator)
}

func TestOpenAITLSFingerprintRuntimeUsesTransportRouterWhenPlatformAntiBanDisabled(t *testing.T) {
	SetRuntimeAntiBanPlatforms(map[string]bool{PlatformOpenAI: false})
	t.Cleanup(func() { SetRuntimeAntiBanPlatforms(map[string]bool{}) })
	setGinTestMode()
	router := &model.TLSFingerprintRouter{
		ID:      13,
		Name:    "transport router",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "codex ws mac",
				Enabled:                 true,
				Transport:               model.TLSFingerprintRouterTransportWebSocket,
				MatchType:               model.TLSFingerprintRouterMatchContains,
				Pattern:                 "Mac OS",
				TLSFingerprintProfileID: 8,
				UpstreamUserAgent:       "ws-codex/1.0",
			},
			{
				Name:                    "codex http mac",
				Enabled:                 true,
				Transport:               model.TLSFingerprintRouterTransportHTTP,
				MatchType:               model.TLSFingerprintRouterMatchContains,
				Pattern:                 "Mac OS",
				TLSFingerprintProfileID: 7,
				UpstreamUserAgent:       "http-codex/1.0",
			},
		},
	}
	svc := &OpenAIGatewayService{
		tlsFPRouterService: NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{
			localCache: map[int64]*model.TLSFingerprintProfile{
				7: {ID: 7, Name: "HTTP Profile", Platform: "openai", Transport: "http"},
				8: {ID: 8, Name: "WS Profile", Platform: "openai", Transport: "websocket"},
			},
		},
	}
	req, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "codex-tui/0.141.0 (Mac OS 26.3.1; arm64)")
	c := &gin.Context{Request: req}
	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": float64(13),
		},
	}

	httpRuntime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), c, account, "http")
	wsRuntime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), c, account, "websocket")

	require.True(t, httpRuntime.Matched)
	require.Equal(t, "HTTP Profile", httpRuntime.Profile.Name)
	require.Equal(t, "http-codex/1.0", httpRuntime.UpstreamUserAgent)
	require.True(t, wsRuntime.Matched)
	require.Equal(t, "WS Profile", wsRuntime.Profile.Name)
	require.Equal(t, "ws-codex/1.0", wsRuntime.UpstreamUserAgent)
}

func TestOpenAICompatibleTLSFingerprintRuntimeUsesGrokRouterMatch(t *testing.T) {
	setGinTestMode()
	router := &model.TLSFingerprintRouter{
		ID:      12,
		Name:    "grok clients",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "grok chrome",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Chrome/",
				TLSFingerprintProfileID: 8,
				UpstreamUserAgent:       "grok-client/1.0",
				UpstreamOriginator:      "grok",
			},
		},
	}
	svc := &OpenAIGatewayService{
		tlsFPRouterService: NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{
			localCache: map[int64]*model.TLSFingerprintProfile{
				8: {ID: 8, Name: "Grok Routed"},
			},
		},
	}
	req, err := http.NewRequest(http.MethodPost, "/v1/videos", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "Chrome/130")
	c := &gin.Context{Request: req}
	account := &Account{
		ID:       2,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{"enable_tls_fingerprint": true, "tls_fingerprint_router_id": float64(12)},
	}

	openAIRuntime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), c, account, "http")
	grokRuntime := svc.resolveOpenAICompatibleTLSFingerprintRuntime(context.Background(), c, account, "http")

	require.False(t, openAIRuntime.Matched, "OpenAI-only resolver must not accidentally enable Grok routing")
	require.True(t, grokRuntime.Matched)
	require.NotNil(t, grokRuntime.Profile)
	require.Equal(t, "Grok Routed", grokRuntime.Profile.Name)
	require.Equal(t, "grok-client/1.0", grokRuntime.UpstreamUserAgent)
	require.Equal(t, "grok", grokRuntime.UpstreamOriginator)
}

func TestOpenAITLSFingerprintRuntimeOriginatorIsReplacementOnly(t *testing.T) {
	setGinTestMode()
	router := &model.TLSFingerprintRouter{
		ID:      11,
		Name:    "openai clients",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "cursor",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Cursor/",
				TLSFingerprintProfileID: 7,
				UpstreamUserAgent:       "codex_cli_rs/0.125.0",
				UpstreamOriginator:      "codex_cli_rs",
			},
		},
	}
	svc := &OpenAIGatewayService{
		tlsFPRouterService: NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{
			localCache: map[int64]*model.TLSFingerprintProfile{
				7: {ID: 7, Name: "Chrome Routed"},
			},
		},
	}
	req, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "Cursor/1.2.3")
	c := &gin.Context{Request: req}
	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"enable_tls_fingerprint": true, "tls_fingerprint_router_id": float64(11)},
	}

	runtime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), c, account, "")

	require.True(t, runtime.Matched)
	require.Equal(t, "codex_cli_rs", runtime.UpstreamOriginator)
}

func TestOpenAITLSFingerprintRuntimeSkipsRouterWhenTLSFingerprintDisabled(t *testing.T) {
	setGinTestMode()
	router := &model.TLSFingerprintRouter{
		ID:      10,
		Name:    "openai clients",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "cursor",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Cursor/",
				TLSFingerprintProfileID: 7,
				UpstreamUserAgent:       "codex_cli_rs/0.125.0",
				UpstreamOriginator:      "codex_cli_rs",
			},
		},
	}
	svc := &OpenAIGatewayService{
		tlsFPRouterService: NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{
			localCache: map[int64]*model.TLSFingerprintProfile{
				7: {ID: 7, Name: "Chrome Routed"},
			},
		},
	}
	req, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "Cursor/1.2.3")
	c := &gin.Context{Request: req}
	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"tls_fingerprint_router_id": float64(10)},
	}

	runtime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), c, account, "")

	require.False(t, runtime.Matched)
	require.Nil(t, runtime.Profile)
	require.Empty(t, runtime.UpstreamUserAgent)
	require.Empty(t, runtime.UpstreamOriginator)
}

func TestOpenAITLSFingerprintRuntimeIgnoresRouterMatchWithMissingProfile(t *testing.T) {
	setGinTestMode()
	router := &model.TLSFingerprintRouter{
		ID:      10,
		Name:    "openai clients",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "cursor",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Cursor/",
				TLSFingerprintProfileID: 404,
				UpstreamUserAgent:       "codex_cli_rs/0.125.0",
				UpstreamOriginator:      "codex_cli_rs",
			},
		},
	}
	svc := &OpenAIGatewayService{
		tlsFPRouterService:  NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{}},
	}
	req, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "Cursor/1.2.3")
	c := &gin.Context{Request: req}
	account := &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"enable_tls_fingerprint": true, "tls_fingerprint_router_id": float64(10)},
	}

	runtime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), c, account, "")

	require.False(t, runtime.Matched)
	require.NotNil(t, runtime.Profile)
	require.Equal(t, "Built-in Default (Node.js 24.x)", runtime.Profile.Name)
	require.Empty(t, runtime.UpstreamUserAgent)
	require.Empty(t, runtime.UpstreamOriginator)
}

func TestOpenAITLSFingerprintRuntimeFallsBackToRuleProfileWhenDimensionBindingProfileMismatches(t *testing.T) {
	SetRuntimeAntiBanPlatforms(map[string]bool{PlatformOpenAI: true})
	t.Cleanup(func() { SetRuntimeAntiBanPlatforms(map[string]bool{}) })

	testCases := []struct {
		name           string
		boundProfile   *model.TLSFingerprintProfile
		requestRuntime string
	}{
		{
			name:           "platform mismatch",
			requestRuntime: "http",
			boundProfile: &model.TLSFingerprintProfile{
				ID:         88,
				Name:       "Grok Bound",
				Platform:   "grok",
				OS:         "macos",
				ClientType: "codex-cli",
			},
		},
		{
			name:           "transport mismatch",
			requestRuntime: "http",
			boundProfile: &model.TLSFingerprintProfile{
				ID:         88,
				Name:       "WebSocket Bound",
				Platform:   "openai",
				Transport:  "websocket-h2",
				OS:         "macos",
				ClientType: "codex-cli",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setGinTestMode()
			router := &model.TLSFingerprintRouter{
				ID:      30,
				Name:    "dimension fallback",
				Enabled: true,
				Rules: []model.TLSFingerprintRouterRule{
					{
						Name:                    "codex mac",
						Enabled:                 true,
						MatchType:               model.TLSFingerprintRouterMatchContains,
						Pattern:                 "codex",
						OS:                      "macos",
						ClientType:              "codex-cli",
						TLSFingerprintProfileID: 77,
					},
				},
			}
			svc := &OpenAIGatewayService{
				tlsFPRouterService: NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil),
				tlsFPProfileService: &TLSFingerprintProfileService{
					localCache: map[int64]*model.TLSFingerprintProfile{
						77: {ID: 77, Name: "Rule Fallback", Platform: "openai", OS: "macos", ClientType: "codex-cli"},
						88: tc.boundProfile,
					},
				},
			}
			req, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
			require.NoError(t, err)
			req.Header.Set("User-Agent", "codex_cli_rs/1.0")
			c := &gin.Context{Request: req}
			account := &Account{
				ID:       1,
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Extra: map[string]any{
					"enable_tls_fingerprint":    true,
					"tls_fingerprint_router_id": float64(30),
					"tls_fingerprint_bindings":  map[string]any{"macos/codex-cli": float64(88)},
				},
			}

			runtime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), c, account, tc.requestRuntime)

			require.True(t, runtime.Matched)
			require.NotNil(t, runtime.Profile)
			require.Equal(t, "Rule Fallback", runtime.Profile.Name)
		})
	}
}
