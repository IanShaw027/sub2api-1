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
