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
