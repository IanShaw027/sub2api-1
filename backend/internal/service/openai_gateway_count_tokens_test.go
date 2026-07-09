package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type countTokensRuntimeStateRepo struct {
	AccountRepository
	tempUnschedCalls int
	setErrorCalls    int
}

func (r *countTokensRuntimeStateRepo) SetTempUnschedulable(_ context.Context, _ int64, _ time.Time, _ string) error {
	r.tempUnschedCalls++
	return nil
}

func (r *countTokensRuntimeStateRepo) SetError(_ context.Context, _ int64, _ string) error {
	r.setErrorCalls++
	return nil
}

func TestOpenAIGatewayService_ForwardCountTokensAsAnthropic_APIKeyUsesResponsesInputTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"claude-sonnet-4-5","system":"You are helpful.","messages":[{"role":"user","content":"hello"}],"tools":[{"name":"lookup","input_schema":{"type":"object"}}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"object":"response.input_tokens","input_tokens":42}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{
			Enabled:           false,
			AllowInsecureHTTP: true,
		}}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          101,
		Name:        "openai-apikey",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "http://upstream.example",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	err := svc.ForwardCountTokensAsAnthropic(context.Background(), c, account, body, "gpt-5.3-codex")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"input_tokens":42}`, rec.Body.String())
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "http://upstream.example/v1/responses/input_tokens", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("authorization"))
	require.Equal(t, "gpt-5.3-codex", gjson.GetBytes(upstream.lastBody, "model").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "input").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
}

func TestOpenAIGatewayService_ForwardCountTokensAsAnthropic_OAuthFallsBackWhenPlatformEndpointUnsupported(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"claude-opus-4-1","messages":[{"role":"user","content":"hello"}]}`)
	account := &Account{
		ID:          202,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "oauth-token",
			"refresh_token": "oauth-refresh-token",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	prepared, err := prepareOpenAIInputTokensCountRequest(body, account, "gpt-5.4")
	require.NoError(t, err)
	expectedEstimate, err := estimateOpenAIInputTokens(prepared.Request)
	require.NoError(t, err)

	cases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "401_missing_responses_write_scope",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"type":"invalid_request_error","code":"missing_scope","message":"You have insufficient permissions for this operation. Missing scopes: api.responses.write."}}`,
		},
		{
			name:       "403_missing_responses_write_scope",
			statusCode: http.StatusForbidden,
			body:       `{"error":{"type":"invalid_request_error","code":"missing_scope","message":"Missing scopes: api.responses.write"}}`,
		},
		{
			name:       "404_input_tokens_unsupported",
			statusCode: http.StatusNotFound,
			body:       `{"error":{"type":"invalid_request_error","message":"The /v1/responses/input_tokens endpoint was not found"}}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Request.Header.Set("User-Agent", "Claude-Code/1.0")

			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}}
			repo := &countTokensRuntimeStateRepo{}
			svc := &OpenAIGatewayService{
				cfg:              &config.Config{},
				httpUpstream:     upstream,
				rateLimitService: &RateLimitService{accountRepo: repo, cfg: &config.Config{}},
			}

			err := svc.ForwardCountTokensAsAnthropic(context.Background(), c, account, body, "gpt-5.4")
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rec.Code)
			require.JSONEq(t, `{"input_tokens":`+strconv.Itoa(expectedEstimate)+`}`, rec.Body.String())
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, "https://api.openai.com/v1/responses/input_tokens", upstream.lastReq.URL.String())
			require.Equal(t, "Bearer oauth-token", upstream.lastReq.Header.Get("authorization"))
			require.Empty(t, upstream.lastReq.Header.Get("Chatgpt-Account-Id"))
			require.Zero(t, repo.tempUnschedCalls, "OAuth input_tokens unsupported errors must not temp-unschedule the account")
			require.Zero(t, repo.setErrorCalls, "OAuth input_tokens unsupported errors must not mark the account error")
		})
	}
}

func TestOpenAIGatewayService_ForwardCountTokensAsAnthropic_GrokReturnsNotSupported(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"grok","messages":[{"role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages/count_tokens", bytes.NewReader(body))

	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:       303,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "tok",
		},
	}

	err := svc.ForwardCountTokensAsAnthropic(context.Background(), c, account, body, "grok-4.5")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "not_found_error")
	require.Contains(t, rec.Body.String(), "Token counting is not supported for Grok groups")
	require.NotContains(t, strings.ToLower(rec.Body.String()), "no available")
	require.Nil(t, upstream.lastReq, "Grok count_tokens must not call upstream input_tokens")
}

func TestOpenAIEndpointURL_ResponsesInputTokensBaseAlreadyAtResponses(t *testing.T) {
	require.Equal(t,
		"http://upstream.example/v1/responses/input_tokens",
		buildOpenAIResponsesInputTokensURL("http://upstream.example/v1/responses"),
	)
	require.Equal(t,
		"http://upstream.example/v1/responses/input_tokens",
		buildOpenAIResponsesInputTokensURL("http://upstream.example/v1/responses/"),
	)
}

func TestOpenAIGatewayService_SelectCountTokensAccountDoesNotAcquireSlotOrBindSticky(t *testing.T) {
	groupID := int64(7)
	acquireCalls := map[int64]int{}
	gatewayCache := &schedulerTestGatewayCache{}
	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
		accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{{
			ID:          101,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			Credentials: map[string]any{
				"api_key": "sk-test",
			},
		}}},
		cache:              gatewayCache,
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireCalls: acquireCalls}),
	}

	selection, _, err := svc.SelectAccountWithSchedulerForCapabilityNoAcquire(
		context.Background(),
		&groupID,
		"count-token-session",
		"gpt-5",
		nil,
		OpenAIUpstreamTransportAny,
		OpenAIEndpointCapabilityResponsesInputTokens,
		false,
		PlatformOpenAI,
	)

	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(101), selection.Account.ID)
	require.False(t, selection.Acquired)
	require.Nil(t, selection.ReleaseFunc)
	require.Nil(t, selection.WaitPlan)
	require.Empty(t, acquireCalls)
	require.Empty(t, gatewayCache.sessionBindings)
}
