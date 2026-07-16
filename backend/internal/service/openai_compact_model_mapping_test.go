package service

import (
	"bytes"
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

func TestOpenAIGatewayService_Forward_CompactOnlyModelMappingOverridesOAuthUpstreamModel(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","stream":false,"instructions":"compact-test","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-compact-map"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_123","status":"completed","model":"gpt-5.4-openai-compact","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          1,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":          "oauth-token",
			"chatgpt_account_id":    "chatgpt-acc",
			"compact_model_mapping": map[string]any{"gpt-5.4": "gpt-5.4-openai-compact"},
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-5.4", result.Model)
	require.Equal(t, "gpt-5.4-openai-compact", result.UpstreamModel)
	require.Equal(t, "gpt-5.4-openai-compact", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestOpenAIGatewayService_Forward_CompactUsesGlobalModelWithoutSpecificMapping(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.6-terra","stream":false,"instructions":"compact-test","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-global-compact"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_global_compact","status":"completed","model":"gpt-5.4","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{OpenAICompactModel: "gpt-5.4"}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          24,
		Name:        "openai-oauth-global-compact",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
			"model_mapping":      map[string]any{"gpt-5.6-terra": "gpt-5.6-terra"},
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-5.4", result.UpstreamModel)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestOpenAIGatewayService_Forward_NonCompactRequestIgnoresCompactOnlyModelMapping(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4","stream":false,"instructions":"normal-test","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-normal-map"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_124","status":"completed","model":"gpt-5.4","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          2,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":          "oauth-token",
			"chatgpt_account_id":    "chatgpt-acc",
			"compact_model_mapping": map[string]any{"gpt-5.4": "gpt-5.4-openai-compact"},
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-5.4", result.Model)
	require.Equal(t, "gpt-5.4", result.UpstreamModel)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestOpenAIGatewayService_Forward_CompactUsesPlatformDefaultCompactMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetPlatformModelRoutingConfigCacheForTest()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-4o-mini","stream":false,"instructions":"compact-test","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-platform-compact-map"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_platform_compact","status":"completed","model":"gpt-4o-mini","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`)),
	}}

	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		settingService: NewSettingService(&kiroRuntimeSettingRepoStub{
			values: map[string]string{
				SettingKeyPlatformDefaultAccountModelConfig: `{
					"openai": {
						"model_mapping": {"gpt-4o-mini": "gpt-5.4"},
						"compact_model_mapping": {"gpt-5.4": "gpt-5.4-mini"}
					}
				}`,
			},
		}, &config.Config{}),
	}
	account := &Account{
		ID:          22,
		Name:        "openai-oauth-platform-compact",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, body)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-4o-mini", result.Model)
	require.Equal(t, "gpt-5.4-mini", result.UpstreamModel)
	require.Equal(t, "gpt-5.4-mini", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestOpenAIGatewayService_Forward_CompactFallbackUsesPlatformDefaultCompactMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetPlatformModelRoutingConfigCacheForTest()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt6","stream":false,"instructions":"compact-fallback-test","input":"hello"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"unknown model"}}`)),
			},
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"stop after fallback capture"}}`)),
			},
		},
	}

	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		settingService: NewSettingService(&antigravityFallbackSettingRepoStub{values: map[string]string{
			SettingKeyEnableModelFallback: "true",
			SettingKeyFallbackModelOpenAI: "gpt-4o-mini",
			SettingKeyPlatformDefaultAccountModelConfig: `{
				"openai": {
					"model_mapping": {"gpt-4o-mini": "gpt-5.4"},
					"compact_model_mapping": {"gpt-5.4": "gpt-5.4-mini"}
				}
			}`,
		}}, &config.Config{}),
	}
	account := &Account{
		ID:          23,
		Name:        "openai-oauth-platform-compact-fallback",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, body)

	require.Error(t, err)
	require.Nil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "gpt6", gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Equal(t, "gpt-5.4-mini", gjson.GetBytes(upstream.bodies[1], "model").String())
}

func TestOpenAIGatewayService_OAuthPassthrough_CompactOnlyModelMappingOverridesUpstreamModel(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("Content-Type", "application/json")

	originalBody := []byte(`{"model":"gpt-5.4","stream":true,"store":true,"instructions":"compact-pass","input":[{"type":"text","text":"compact me"}]}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-compact-pass-map"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"cmp_124","model":"gpt-5.4-openai-compact","usage":{"input_tokens":2,"output_tokens":3}}`)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          3,
		Name:        "openai-oauth-pass",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":          "oauth-token",
			"chatgpt_account_id":    "chatgpt-acc",
			"compact_model_mapping": map[string]any{"gpt-5.4": "gpt-5.4-openai-compact"},
		},
		Extra:       map[string]any{"openai_passthrough": true},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-5.4", result.Model)
	require.Equal(t, "gpt-5.4-openai-compact", result.UpstreamModel)
	require.Equal(t, "gpt-5.4-openai-compact", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "gpt-5.4", gjson.GetBytes(rec.Body.Bytes(), "model").String())
}

func TestOpenAIGatewayService_OAuthPassthrough_CompactModelMappingHonorsBaseModelMapping(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.1.0")
	c.Request.Header.Set("Content-Type", "application/json")

	originalBody := []byte(`{"model":"gpt-5.4","stream":true,"store":true,"instructions":"compact-pass","input":[{"type":"text","text":"compact me"}]}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-compact-pass-map-chain"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"cmp_125","model":"gpt-5.4-openai-compact","usage":{"input_tokens":2,"output_tokens":3}}`)),
	}}

	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := &Account{
		ID:          4,
		Name:        "openai-oauth-pass-chain",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
			"model_mapping": map[string]any{
				"gpt-5.4": "gpt-5.4-base",
			},
			"compact_model_mapping": map[string]any{
				"gpt-5.4-base": "gpt-5.4-openai-compact",
			},
		},
		Extra:       map[string]any{"openai_passthrough": true},
		Status:      StatusActive,
		Schedulable: true,
	}

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-5.4", result.Model)
	require.Equal(t, "gpt-5.4-openai-compact", result.UpstreamModel)
	require.Equal(t, "gpt-5.4-openai-compact", gjson.GetBytes(upstream.lastBody, "model").String())
}
