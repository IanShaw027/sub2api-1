//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func leftoverAntigravityOAuthAccount(id int64) *Account {
	return &Account{
		ID:       id,
		Name:     "ag-oauth",
		Platform: PlatformAntigravity,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
		Credentials: map[string]any{
			"access_token": "ya29.test-token",
			"project_id":   "project-1",
			"expires_at":   "2099-01-01T00:00:00Z",
		},
		Concurrency: 1,
	}
}

func TestAntigravityOAuthRetryLoopUsesProfileUserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(antigravityForwardBaseURLEnv, "")

	const profileUA = "antigravity/9.9.9 windows/amd64"
	account := leftoverAntigravityOAuthAccount(1410)
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformAntigravity, ClientFamilyAntigravity, profileUA, "mid-ag-retry"))

	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"response":{}}`)),
		},
	}
	svc := &AntigravityGatewayService{httpUpstream: httpStub}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/antigravity/v1/generateContent", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("User-Agent", "inbound-should-not-win")

	result, err := svc.antigravityRetryLoop(antigravityRetryLoopParams{
		ctx:          context.Background(),
		prefix:       "[test-ag-oauth-ua]",
		account:      account,
		accessToken:  "token",
		action:       "generateContent",
		body:         []byte(`{"input":"test"}`),
		c:            c,
		httpUpstream: httpStub,
		handleError: func(context.Context, string, *Account, int, http.Header, []byte, string, int64, string, bool) *handleModelRateLimitResult {
			return nil
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.resp)
	defer func() { _ = result.resp.Body.Close() }()
	require.NotNil(t, httpStub.lastReq)
	require.Equal(t, profileUA, httpStub.lastReq.Header.Get("User-Agent"))
	require.NotEqual(t, "inbound-should-not-win", httpStub.lastReq.Header.Get("User-Agent"))
	require.NotEqual(t, antigravity.GetUserAgentForContext(context.Background()), httpStub.lastReq.Header.Get("User-Agent"))
	require.Equal(t, 1, leftoverSharedRepo.getCount(account.ID), "profile must be loaded once per send, not once in a pre-call and again in sendAntigravityAccountHTTP")
}

func TestAntigravityOAuthRetryLoopAbortsWhenProfileStoreReturnsUnmarkedError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(antigravityForwardBaseURLEnv, "")

	account := leftoverAntigravityOAuthAccount(1414)
	installLeftoverOutboundProfileError(t, account.ID, errors.New("connection refused"))

	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"response":{}}`)),
		},
	}
	svc := &AntigravityGatewayService{httpUpstream: httpStub}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/antigravity/v1/generateContent", bytes.NewReader([]byte(`{}`)))

	result, err := svc.antigravityRetryLoop(antigravityRetryLoopParams{
		ctx:          context.Background(),
		prefix:       "[test-ag-oauth-ua-unmarked]",
		account:      account,
		accessToken:  "token",
		action:       "generateContent",
		body:         []byte(`{"input":"test"}`),
		c:            c,
		httpUpstream: httpStub,
		handleError: func(context.Context, string, *Account, int, http.Header, []byte, string, int64, string, bool) *handleModelRateLimitResult {
			return nil
		},
	})
	require.Error(t, err)
	require.Nil(t, result)
	require.Zero(t, httpStub.calls)
	require.Equal(t, 1, leftoverSharedRepo.getCount(account.ID), "unmarked store errors must abort without retrying the profile load")
}

func TestAntigravityOAuthRetryLoopAbortsWhenProfileLoadFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(antigravityForwardBaseURLEnv, "")

	account := leftoverAntigravityOAuthAccount(1411)
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"response":{}}`)),
		},
	}
	svc := &AntigravityGatewayService{httpUpstream: httpStub}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/antigravity/v1/generateContent", bytes.NewReader([]byte(`{}`)))
	c.Request.Header.Set("User-Agent", "inbound-should-not-win")

	result, err := svc.antigravityRetryLoop(antigravityRetryLoopParams{
		ctx:          context.Background(),
		prefix:       "[test-ag-oauth-ua-fail]",
		account:      account,
		accessToken:  "token",
		action:       "generateContent",
		body:         []byte(`{"input":"test"}`),
		c:            c,
		httpUpstream: httpStub,
		handleError: func(context.Context, string, *Account, int, http.Header, []byte, string, int64, string, bool) *handleModelRateLimitResult {
			return nil
		},
	})
	require.Error(t, err)
	require.Nil(t, result)
	require.Zero(t, httpStub.calls)
}

func TestSendAntigravityAccountHTTPPinsProfileUAOverTLSRuntime(t *testing.T) {
	const profileUA = "antigravity/8.8.8 windows/amd64"
	account := leftoverAntigravityOAuthAccount(1412)
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformAntigravity, ClientFamilyAntigravity, profileUA, "mid-ag-pin"))

	req, err := http.NewRequest(http.MethodPost, "https://example.test/v1internal:generateContent", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", antigravity.GetUserAgentForContext(context.Background()))

	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))},
	}
	p := &antigravityRetryLoopParams{
		ctx:          context.Background(),
		account:      account,
		httpUpstream: httpStub,
	}
	svc := &AntigravityGatewayService{}

	resp, err := p.sendAntigravityAccountHTTP(svc, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, profileUA, leftoverOutboundTLSRoutingUA(req))
	require.Equal(t, profileUA, httpStub.lastReq.Header.Get("User-Agent"))

	pinned := pinLeftoverOutboundUserAgent(httpStub, req)
	applyTLSFingerprintRuntimeHeaders(req, accountTLSFingerprintRuntime{UpstreamUserAgent: "router-rewritten-ua"})
	require.Equal(t, "router-rewritten-ua", req.Header.Get("User-Agent"))

	pinnedResp, err := pinned.DoWithTLS(req, "", account.ID, 1, nil)
	require.NoError(t, err)
	require.NotNil(t, pinnedResp)
	require.Equal(t, profileUA, httpStub.lastReq.Header.Get("User-Agent"))
	require.NotEqual(t, "router-rewritten-ua", httpStub.lastReq.Header.Get("User-Agent"))
}

func TestSendAntigravityAccountHTTPAbortsWhenProfileLoadFails(t *testing.T) {
	account := leftoverAntigravityOAuthAccount(1413)
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	req, err := http.NewRequest(http.MethodPost, "https://example.test/v1internal:generateContent", nil)
	require.NoError(t, err)
	req.Header.Set("User-Agent", antigravity.GetUserAgentForContext(context.Background()))

	httpStub := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))},
	}
	p := &antigravityRetryLoopParams{
		ctx:          context.Background(),
		account:      account,
		httpUpstream: httpStub,
	}

	resp, err := p.sendAntigravityAccountHTTP(&AntigravityGatewayService{}, req)
	require.Error(t, err)
	require.Nil(t, resp)
	require.Zero(t, httpStub.calls)
}
