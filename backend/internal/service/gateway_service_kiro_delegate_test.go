package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type kiroDelegateHTTPUpstream struct {
	lastReq *http.Request
	calls   int
}

func (u *kiroDelegateHTTPUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *kiroDelegateHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.calls++
	u.lastReq = req
	return &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"message":"ThrottlingException: Rate exceeded"}`)),
	}, nil
}

func TestGatewayServiceForward_DelegatesKiroAccountsToKiroGateway(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	upstream := &kiroDelegateHTTPUpstream{}
	svc := &GatewayService{
		kiroGatewayService: &KiroGatewayService{
			httpUpstream: upstream,
		},
	}

	account := &Account{
		ID:          501,
		Name:        "kiro-delegate",
		Platform:    PlatformKiro,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":    "kiro-api-key",
			"api_region": "us-east-1",
			"region":     "us-east-1",
		},
	}
	parsed := &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body:  []byte(`{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`),
	}

	_, err := svc.Forward(context.Background(), c, account, parsed)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, 1, upstream.calls)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "q.us-east-1.amazonaws.com", upstream.lastReq.URL.Host)
	require.Equal(t, "/generateAssistantResponse", upstream.lastReq.URL.Path)
	require.Equal(t, "Bearer kiro-api-key", upstream.lastReq.Header.Get("Authorization"))
}

func TestGatewayServiceForwardCountTokens_DelegatesKiroAccountsToKiroGateway(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &GatewayService{
		kiroGatewayService: &KiroGatewayService{},
	}

	err := svc.ForwardCountTokens(context.Background(), c, &Account{
		ID:       502,
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
	}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body:  []byte(`{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":[{"type":"text","text":"hello from delegated count tokens"}]}]}`),
	})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"input_tokens":`)
}

func TestGatewayServiceForward_KiroReturnsConfiguredErrorWhenDelegateMissing(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &GatewayService{}

	_, err := svc.Forward(context.Background(), c, &Account{Platform: PlatformKiro, Type: AccountTypeOAuth}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body:  []byte(`{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`),
	})

	require.EqualError(t, err, "kiro gateway service is not configured")
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "Kiro gateway service is not configured")
}

func TestGatewayServiceForwardCountTokens_KiroReturnsConfiguredErrorWhenDelegateMissing(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &GatewayService{}

	err := svc.ForwardCountTokens(context.Background(), c, &Account{Platform: PlatformKiro, Type: AccountTypeOAuth}, &ParsedRequest{
		Model: "claude-sonnet-4-5-20250929",
		Body:  []byte(`{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`),
	})

	require.EqualError(t, err, "kiro gateway service is not configured")
	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), "Kiro gateway service is not configured")
}
