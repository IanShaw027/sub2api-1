//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayDoAccountHTTPPinsGrokProfileUAAgainstTLSRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const profileUA = "xai-grok-workspace/0.2.200"
	const routerUA = "router-rewritten-grok-ua"

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/responses", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", profileUA)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "Mozilla/5.0 inbound-should-not-win")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Header:     make(http.Header),
	}}
	account := &Account{
		ID:       1912,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": int64(8),
		},
	}
	svc := &OpenAIGatewayService{
		httpUpstream:        upstream,
		tlsFPProfileService: testTLSProfileService(),
		tlsFPRouterService: testTLSRouterService(&model.TLSFingerprintRouter{
			ID:      8,
			Enabled: true,
			Rules: []model.TLSFingerprintRouterRule{{
				Name:              "rewrite-ua",
				Enabled:           true,
				UpstreamUserAgent: routerUA,
			}},
		}),
	}

	resp, err := svc.doAccountHTTP(context.Background(), c, account, req, "", "responses")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, profileUA, upstream.lastReq.Header.Get("User-Agent"))
	require.NotEqual(t, routerUA, upstream.lastReq.Header.Get("User-Agent"))
}

func TestOpenAIGatewayDoOpenAIUpstreamPinsGrokProfileUAAgainstTLSRouter(t *testing.T) {
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(nil)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	const profileUA = "xai-grok-workspace/0.2.200"
	const routerUA = "router-rewritten-grok-ua"

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/chat/completions", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", profileUA)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Header:     make(http.Header),
	}}
	account := &Account{
		ID:       1913,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": int64(8),
		},
	}
	svc := &OpenAIGatewayService{
		httpUpstream:        upstream,
		tlsFPProfileService: testTLSProfileService(),
		tlsFPRouterService: testTLSRouterService(&model.TLSFingerprintRouter{
			ID:      8,
			Enabled: true,
			Rules: []model.TLSFingerprintRouterRule{{
				Name:              "rewrite-ua",
				Enabled:           true,
				UpstreamUserAgent: routerUA,
			}},
		}),
	}

	resp, err := svc.doOpenAIUpstream(req, "", account)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, profileUA, upstream.lastReq.Header.Get("User-Agent"))
	require.NotEqual(t, routerUA, upstream.lastReq.Header.Get("User-Agent"))
}

func TestOpenAIGatewayDoOpenAIUpstreamInfersProtocolFromPath(t *testing.T) {
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(nil)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", nil)
	require.NoError(t, err)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Header:     make(http.Header),
	}}
	account := &Account{
		ID:       1914,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": int64(9),
		},
	}
	svc := &OpenAIGatewayService{
		httpUpstream:        upstream,
		tlsFPProfileService: testTLSProfileService(),
		tlsFPRouterService: testTLSRouterService(&model.TLSFingerprintRouter{
			ID:      9,
			Enabled: true,
			Rules: []model.TLSFingerprintRouterRule{
				{
					Name:              "responses",
					Enabled:           true,
					Protocol:          "responses",
					UpstreamUserAgent: "responses-router-ua",
				},
				{
					Name:              "chat-completions",
					Enabled:           true,
					Protocol:          "chat_completions",
					UpstreamUserAgent: "chat-completions-router-ua",
				},
			},
		}),
	}

	resp, err := svc.doOpenAIUpstream(req, "", account)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "chat-completions-router-ua", upstream.lastReq.Header.Get("User-Agent"))
	require.NotEqual(t, "responses-router-ua", upstream.lastReq.Header.Get("User-Agent"))
}

type tlsProfileUpstreamRecorder struct {
	httpUpstreamRecorder
	lastProfileName string
}

func (u *tlsProfileUpstreamRecorder) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	if profile != nil {
		u.lastProfileName = profile.Name
	}
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestOpenAIAccountTestUpstreamPinsGrokLeftoverAndUsesRouterProfile(t *testing.T) {
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(nil)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	const profileUA = "xai-grok-workspace/0.2.200"
	const routerUA = "router-rewritten-grok-ua"

	req, err := http.NewRequest(http.MethodPost, "https://cli-chat-proxy.grok.com/v1/chat/completions", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", profileUA)

	upstream := &tlsProfileUpstreamRecorder{httpUpstreamRecorder: httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Header:     make(http.Header),
	}}}
	account := &Account{
		ID:       1915,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": int64(11),
			"tls_fingerprint_router_id":  int64(8),
		},
	}
	svc := &AccountTestService{
		httpUpstream:        upstream,
		tlsFPProfileService: testTLSProfileService(
			&model.TLSFingerprintProfile{ID: 11, Name: "account-default"},
			&model.TLSFingerprintProfile{ID: 22, Name: "router-profile"},
		),
		tlsFPRouterService: testTLSRouterService(&model.TLSFingerprintRouter{
			ID:      8,
			Enabled: true,
			Rules: []model.TLSFingerprintRouterRule{{
				Name:                    "rewrite-ua",
				Enabled:                 true,
				TLSFingerprintProfileID: 22,
				UpstreamUserAgent:       routerUA,
			}},
		}),
	}

	resp, err := svc.doOpenAIAccountTestUpstream(req, "", account, true)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, profileUA, upstream.lastReq.Header.Get("User-Agent"))
	require.NotEqual(t, routerUA, upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, "router-profile", upstream.lastProfileName)
}

func TestOpenAIAccountTestUpstreamInfersProtocolFromPath(t *testing.T) {
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(nil)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", nil)
	require.NoError(t, err)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
		Header:     make(http.Header),
	}}
	account := &Account{
		ID:       1916,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": int64(9),
		},
	}
	svc := &AccountTestService{
		httpUpstream:        upstream,
		tlsFPProfileService: testTLSProfileService(),
		tlsFPRouterService: testTLSRouterService(&model.TLSFingerprintRouter{
			ID:      9,
			Enabled: true,
			Rules: []model.TLSFingerprintRouterRule{
				{
					Name:              "responses",
					Enabled:           true,
					Protocol:          "responses",
					UpstreamUserAgent: "responses-router-ua",
				},
				{
					Name:              "chat-completions",
					Enabled:           true,
					Protocol:          "chat_completions",
					UpstreamUserAgent: "chat-completions-router-ua",
				},
			},
		}),
	}

	resp, err := svc.doOpenAIAccountTestUpstream(req, "", account, true)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "chat-completions-router-ua", upstream.lastReq.Header.Get("User-Agent"))
}
