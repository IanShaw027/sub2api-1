//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardInjectsMimicMetadataUserIDWhenFingerprintUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	account := &Account{
		ID:          1913,
		Name:        "anthropic-oauth-mimic-no-fp",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "oauth-token",
		},
		Status:      StatusActive,
		Schedulable: true,
	}
	profile := leftoverValidProfile(account.ID, PlatformAnthropic, ClientFamilyClaudeCode, "claude-cli/2.1.22 (external, cli)", "mid-mimic")
	installLeftoverOutboundProfile(t, profile)

	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"rid-mimic-no-fp"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"output_tokens":1}}`)),
	}}
	svc := &GatewayService{
		cfg:                  &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}},
		responseHeaderFilter: compileResponseHeaderFilter(&config.Config{}),
		httpUpstream:         upstream,
		rateLimitService:     &RateLimitService{},
		deferredService:      &DeferredService{},
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(body)))
	c.Request.Header.Set("User-Agent", "curl/8.0")

	parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
	require.NoError(t, err)

	_, err = svc.Forward(context.Background(), c, account, parsed)
	require.NoError(t, err)
	require.NotEmpty(t, upstream.lastBody)
	got := gjson.GetBytes(upstream.lastBody, "metadata.user_id").String()
	require.NotEmpty(t, got)
	require.Contains(t, got, profile.DeviceID)
	require.Contains(t, got, profile.GatewayAccountUUID)
}
