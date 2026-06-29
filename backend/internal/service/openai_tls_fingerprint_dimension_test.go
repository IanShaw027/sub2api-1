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

// Router 规则输出判定维度 (os/client_type) 时，应走账号绑定矩阵解析，
// 从而实现「同一路由规则、不同账号解析出不同模板」。
func TestOpenAITLSFingerprintRuntimeUsesDimensionMatrix(t *testing.T) {
	setGinTestMode()
	router := &model.TLSFingerprintRouter{
		ID:      20,
		Name:    "codex by os",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:       "codex-macos",
				Enabled:    true,
				MatchType:  model.TLSFingerprintRouterMatchContains,
				Pattern:    "codex",
				OS:         "macos",
				ClientType: "codex-cli",
			},
		},
	}
	profileSvc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			101: {ID: 101, Name: "acct-A-macos-codex", Platform: "openai", OS: "macos", ClientType: "codex-cli"},
			202: {ID: 202, Name: "acct-B-macos-codex", Platform: "openai", OS: "macos", ClientType: "codex-cli"},
		},
	}
	svc := &OpenAIGatewayService{
		tlsFPRouterService:  NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil),
		tlsFPProfileService: profileSvc,
	}

	newCtx := func() *gin.Context {
		req, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
		require.NoError(t, err)
		req.Header.Set("User-Agent", "codex_cli_rs/1.0")
		return &gin.Context{Request: req}
	}

	accountA := &Account{
		ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Extra: map[string]any{
			"enable_tls_fingerprint":   true,
			"tls_fingerprint_router_id": float64(20),
			"tls_fingerprint_bindings": map[string]any{"macos/codex-cli": float64(101)},
		},
	}
	accountB := &Account{
		ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Extra: map[string]any{
			"enable_tls_fingerprint":   true,
			"tls_fingerprint_router_id": float64(20),
			"tls_fingerprint_bindings": map[string]any{"macos/codex-cli": float64(202)},
		},
	}

	runA := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), newCtx(), accountA, "http")
	require.True(t, runA.Matched)
	require.Equal(t, "acct-A-macos-codex", runA.Profile.Name)

	runB := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), newCtx(), accountB, "http")
	require.True(t, runB.Matched)
	require.Equal(t, "acct-B-macos-codex", runB.Profile.Name)
}

// 旧规则（直出 profileID、无维度）应保持向后兼容。
func TestOpenAITLSFingerprintRuntimeLegacyRuleStillWorks(t *testing.T) {
	setGinTestMode()
	router := &model.TLSFingerprintRouter{
		ID:      21,
		Name:    "legacy direct",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "direct",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchContains,
				Pattern:                 "codex",
				TLSFingerprintProfileID: 7,
			},
		},
	}
	svc := &OpenAIGatewayService{
		tlsFPRouterService: NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{
			localCache: map[int64]*model.TLSFingerprintProfile{
				7: {ID: 7, Name: "Direct Profile", Platform: "openai"},
			},
		},
	}
	req, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "codex_cli_rs/1.0")
	c := &gin.Context{Request: req}
	account := &Account{
		ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Extra: map[string]any{"enable_tls_fingerprint": true, "tls_fingerprint_router_id": float64(21)},
	}

	runtime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), c, account, "http")
	require.True(t, runtime.Matched)
	require.Equal(t, "Direct Profile", runtime.Profile.Name)
}

// 维度规则但账号矩阵未命中 → 降级到账号旧单值。
func TestOpenAITLSFingerprintRuntimeDimensionDegradesToLegacySingle(t *testing.T) {
	setGinTestMode()
	router := &model.TLSFingerprintRouter{
		ID:      22,
		Name:    "dim no profile",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:      "win",
				Enabled:   true,
				MatchType: model.TLSFingerprintRouterMatchContains,
				Pattern:   "codex",
				OS:        "windows",
			},
		},
	}
	svc := &OpenAIGatewayService{
		tlsFPRouterService: NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil),
		tlsFPProfileService: &TLSFingerprintProfileService{
			localCache: map[int64]*model.TLSFingerprintProfile{
				55: {ID: 55, Name: "Legacy Single", Platform: "openai"},
			},
		},
	}
	req, err := http.NewRequest(http.MethodPost, "/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", "codex_cli_rs/1.0")
	c := &gin.Context{Request: req}
	account := &Account{
		ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_router_id":  float64(22),
			"tls_fingerprint_profile_id": float64(55),
		},
	}

	runtime := svc.resolveOpenAITLSFingerprintRuntime(context.Background(), c, account, "http")
	require.True(t, runtime.Matched)
	require.Equal(t, "Legacy Single", runtime.Profile.Name)
}
