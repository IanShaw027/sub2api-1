package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/stretchr/testify/require"
)

type fingerprintNormalizerRouterRepoStub struct {
	routers []*model.TLSFingerprintRouter
}

func (r *fingerprintNormalizerRouterRepoStub) List(context.Context) ([]*model.TLSFingerprintRouter, error) {
	return r.routers, nil
}

func (r *fingerprintNormalizerRouterRepoStub) GetByID(_ context.Context, id int64) (*model.TLSFingerprintRouter, error) {
	for _, router := range r.routers {
		if router.ID == id {
			return router, nil
		}
	}
	return nil, nil
}

func (r *fingerprintNormalizerRouterRepoStub) Create(_ context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	r.routers = append(r.routers, router)
	return router, nil
}

func (r *fingerprintNormalizerRouterRepoStub) Update(_ context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	for i, existing := range r.routers {
		if existing.ID == router.ID {
			r.routers[i] = router
			return router, nil
		}
	}
	r.routers = append(r.routers, router)
	return router, nil
}

func (r *fingerprintNormalizerRouterRepoStub) Delete(_ context.Context, id int64) error {
	next := r.routers[:0]
	for _, router := range r.routers {
		if router.ID != id {
			next = append(next, router)
		}
	}
	r.routers = next
	return nil
}

func TestFingerprintNormalizer_ResolveCanonical_SkipsAPIKeyAccounts(t *testing.T) {
	n := NewFingerprintNormalizer(nil, nil, nil, nil, nil)

	canonical := n.ResolveCanonical(context.Background(), &Account{
		ID:       71716,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
	}, "claude-cli/1.0.0")

	require.Nil(t, canonical)
}

func TestFingerprintNormalizer_ApplyToRequest_SkipsAPIKeyAccounts(t *testing.T) {
	n := NewFingerprintNormalizer(nil, nil, nil, nil, nil)
	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)

	req, newBody, err := n.ApplyToRequest(nil, body, n.ResolveCanonical(context.Background(), &Account{
		ID:       71716,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
	}, "claude-cli/1.0.0"))

	require.NoError(t, err)
	require.Nil(t, req)
	require.Equal(t, body, newBody)
}

func TestFingerprintNormalizer_AntiBanGlobalDisabledOverridesPlatformToggle(t *testing.T) {
	n := NewFingerprintNormalizer(nil, nil, nil, &CanonicalFingerprintConfig{
		Enabled:             true,
		AntiBanEnabled:      false,
		EnabledByPlatform:   map[string]bool{"anthropic": true},
		SpoofProcessMetrics: true,
		TelemetryPaths:      []string{"telemetry"},
	}, nil)
	body := []byte(`{"metadata":{"telemetry":"leak"},"messages":[{"role":"user","content":"hi"}]}`)

	_, newBody, err := n.ApplyToRequest(nil, body, &CanonicalFingerprint{
		Platform: "anthropic",
		DeviceID: "device",
	})

	require.NoError(t, err)
	require.Equal(t, body, newBody)
}

func TestFingerprintNormalizerResolveCanonicalRejectsWebSocketOnlyRouterProfileForHTTP(t *testing.T) {
	routerSvc := NewTLSFingerprintRouterService(&fingerprintNormalizerRouterRepoStub{routers: []*model.TLSFingerprintRouter{
		{
			ID:      10,
			Name:    "codex",
			Enabled: true,
			Rules: []model.TLSFingerprintRouterRule{
				{
					Name:                    "ws-only",
					Enabled:                 true,
					Transport:               model.TLSFingerprintRouterTransportHTTP,
					MatchType:               model.TLSFingerprintRouterMatchPrefix,
					Pattern:                 "Codex Desktop/",
					TLSFingerprintProfileID: 7,
					UpstreamUserAgent:       "WebSocket Routed UA",
				},
			},
		},
	}}, nil)
	profileSvc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			7: {
				ID:        7,
				Name:      "WebSocket Only",
				Platform:  PlatformOpenAI,
				Transport: model.TLSFingerprintRouterTransportWSH2,
			},
		},
	}
	n := NewFingerprintNormalizer(profileSvc, routerSvc, nil, nil, nil)
	account := &Account{
		ID:       71800,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": float64(10),
		},
	}

	canonical := n.ResolveCanonical(context.Background(), account, "Codex Desktop/26.1")

	require.NotNil(t, canonical)
	require.Equal(t, "Codex Desktop/26.1", canonical.UserAgent)
}
