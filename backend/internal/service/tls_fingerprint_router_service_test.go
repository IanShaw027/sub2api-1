//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/stretchr/testify/require"
)

type tlsFingerprintRouterRepoStub struct {
	routers []*model.TLSFingerprintRouter
}

func (r *tlsFingerprintRouterRepoStub) List(context.Context) ([]*model.TLSFingerprintRouter, error) {
	return r.routers, nil
}

func (r *tlsFingerprintRouterRepoStub) GetByID(_ context.Context, id int64) (*model.TLSFingerprintRouter, error) {
	for _, router := range r.routers {
		if router.ID == id {
			return router, nil
		}
	}
	return nil, nil
}

func (r *tlsFingerprintRouterRepoStub) Create(_ context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	r.routers = append(r.routers, router)
	return router, nil
}

func (r *tlsFingerprintRouterRepoStub) Update(_ context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	for i, existing := range r.routers {
		if existing.ID == router.ID {
			r.routers[i] = router
			return router, nil
		}
	}
	r.routers = append(r.routers, router)
	return router, nil
}

func (r *tlsFingerprintRouterRepoStub) Delete(_ context.Context, id int64) error {
	next := r.routers[:0]
	for _, router := range r.routers {
		if router.ID != id {
			next = append(next, router)
		}
	}
	r.routers = next
	return nil
}

func TestTLSFingerprintRouterServiceMatchUserAgent(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID:      10,
		Name:    "clients",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "disabled first",
				Enabled:                 false,
				MatchType:               model.TLSFingerprintRouterMatchContains,
				Pattern:                 "Codex",
				TLSFingerprintProfileID: 99,
			},
			{
				Name:                    "cursor prefix",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Cursor/",
				CaseSensitive:           false,
				TLSFingerprintProfileID: 12,
				UpstreamUserAgent:       "codex_cli_rs/0.125.0",
				UpstreamOriginator:      "codex_cli_rs",
			},
			{
				Name:                    "fallback",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchContains,
				Pattern:                 "Cursor",
				TLSFingerprintProfileID: 13,
			},
		},
	}
	svc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil)

	result, ok := svc.MatchRequest(context.Background(), 10, "cursor/1.2.3", "")

	require.True(t, ok)
	require.Equal(t, int64(12), result.ProfileID)
	require.Equal(t, "codex_cli_rs/0.125.0", result.UpstreamUserAgent)
	require.Equal(t, "codex_cli_rs", result.UpstreamOriginator)
}

func TestTLSFingerprintRouterServiceCreateRejectsNilRouter(t *testing.T) {
	svc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{}, nil)

	_, err := svc.Create(context.Background(), nil)

	require.EqualError(t, err, "tls fingerprint router is required")
}

func TestTLSFingerprintRouterServiceUpdateRejectsNilRouter(t *testing.T) {
	svc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{}, nil)

	_, err := svc.Update(context.Background(), nil)

	require.EqualError(t, err, "tls fingerprint router is required")
}

func TestTLSFingerprintRouterServiceUpstreamOriginatorIsPassthrough(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID:      11,
		Name:    "originator passthrough",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "codex",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Codex/",
				TLSFingerprintProfileID: 14,
				UpstreamOriginator:      "codex_cli_rs",
			},
		},
	}
	svc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil)

	result, ok := svc.MatchRequest(context.Background(), 11, "Codex/1.2.3", "")
	require.True(t, ok)
	require.Equal(t, int64(14), result.ProfileID)
	require.Equal(t, "codex_cli_rs", result.UpstreamOriginator)
}

func TestTLSFingerprintRouterServiceCanonicalTransportValuesValidateAndMatchSupportedRuntimeFamilies(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID:      12,
		Name:    "canonical transport",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "codex h2",
				Enabled:                 true,
				Transport:               "h2",
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Codex/",
				TLSFingerprintProfileID: 15,
			},
			{
				Name:                    "codex http1",
				Enabled:                 true,
				Transport:               "http1",
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Codex/",
				TLSFingerprintProfileID: 17,
			},
			{
				Name:                    "codex ws h2",
				Enabled:                 true,
				Transport:               "websocket-h2",
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Codex/",
				TLSFingerprintProfileID: 16,
			},
			{
				Name:                    "codex ws http1",
				Enabled:                 true,
				Transport:               "websocket-http1",
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Codex/",
				TLSFingerprintProfileID: 18,
			},
		},
	}
	require.NoError(t, router.Validate())
	svc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil)

	result, ok := svc.MatchRequest(context.Background(), 12, "Codex/1.2.3", model.TLSFingerprintRouterTransportHTTP)
	require.True(t, ok)
	require.Equal(t, int64(15), result.ProfileID, "coarse HTTP runtime should allow an ordered h2 rule")

	result, ok = svc.MatchRequest(context.Background(), 12, "Codex/1.2.3", model.TLSFingerprintRouterTransportWebSocket)
	require.True(t, ok)
	require.Equal(t, int64(16), result.ProfileID, "coarse websocket runtime should allow an ordered websocket-h2 rule")
}

func TestTLSFingerprintRouterServiceLegacyCoarseTransportValuesStillValidate(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID:      13,
		Name:    "legacy transport",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "legacy http",
				Enabled:                 true,
				Transport:               "http",
				MatchType:               model.TLSFingerprintRouterMatchPrefix,
				Pattern:                 "Codex/",
				TLSFingerprintProfileID: 17,
			},
		},
	}
	require.NoError(t, router.Validate())
}

func TestTLSFingerprintRouterServiceMatchUserAgentRegex(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID:      20,
		Name:    "regex",
		Enabled: true,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "codex version",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchRegex,
				Pattern:                 `codex_cli_rs/\d+\.\d+`,
				TLSFingerprintProfileID: 44,
			},
		},
	}
	svc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil)

	result, ok := svc.MatchUserAgent(context.Background(), 20, "Codex_CLI_RS/0.125")

	require.True(t, ok)
	require.Equal(t, int64(44), result.ProfileID)
}

func TestTLSFingerprintRouterServiceMatchUserAgentNoMatch(t *testing.T) {
	router := &model.TLSFingerprintRouter{
		ID:      30,
		Name:    "disabled",
		Enabled: false,
		Rules: []model.TLSFingerprintRouterRule{
			{
				Name:                    "codex",
				Enabled:                 true,
				MatchType:               model.TLSFingerprintRouterMatchContains,
				Pattern:                 "codex",
				TLSFingerprintProfileID: 1,
			},
		},
	}
	svc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{router}}, nil)

	_, ok := svc.MatchUserAgent(context.Background(), 30, "codex_cli_rs")

	require.False(t, ok)
}
