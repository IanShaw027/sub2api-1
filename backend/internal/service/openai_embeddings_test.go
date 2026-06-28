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

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildOpenAIEmbeddingsURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		want string
	}{
		{"bare domain", "https://api.openai.com", "https://api.openai.com/v1/embeddings"},
		{"bare /v1", "https://api.openai.com/v1", "https://api.openai.com/v1/embeddings"},
		{"already embeddings", "https://api.openai.com/v1/embeddings", "https://api.openai.com/v1/embeddings"},
		{"third-party versioned path", "https://open.bigmodel.cn/api/paas/v4", "https://open.bigmodel.cn/api/paas/v4/embeddings"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, buildOpenAIEmbeddingsURL(tt.base))
		})
	}
}

func TestForwardEmbeddings_APIKeyPassthroughRecordsUsageAndBatchInput(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{
		"model":"nowledge-embedding",
		"input":["hello","world"],
		"encoding_format":"float",
		"dimensions":256
	}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"emb-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{
			"object":"list",
			"data":[
				{"object":"embedding","index":0,"embedding":[0.1,0.2]},
				{"object":"embedding","index":1,"embedding":[0.3,0.4]}
			],
			"model":"jina-embeddings-v5-text-small",
			"usage":{"prompt_tokens":13,"total_tokens":13}
		}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://api.jina.ai",
			"model_mapping": map[string]any{
				"nowledge-embedding": "jina-embeddings-v5-text-small",
			},
		},
	}

	result, err := svc.ForwardEmbeddings(context.Background(), c, account, reqBody, "")

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, result)
	require.Equal(t, "emb-rid", result.RequestID)
	require.Equal(t, "nowledge-embedding", result.Model)
	require.Equal(t, "jina-embeddings-v5-text-small", result.BillingModel)
	require.Equal(t, "jina-embeddings-v5-text-small", result.UpstreamModel)
	require.Equal(t, 13, result.Usage.InputTokens)
	require.Equal(t, 0, result.Usage.OutputTokens)
	require.Equal(t, "https://api.jina.ai/v1/embeddings", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "jina-embeddings-v5-text-small", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, int64(2), gjson.GetBytes(upstream.lastBody, "input.#").Int())
	require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "input.0").String())
	require.Equal(t, "world", gjson.GetBytes(upstream.lastBody, "input.1").String())
	require.Equal(t, "float", gjson.GetBytes(upstream.lastBody, "encoding_format").String())
	require.Equal(t, int64(256), gjson.GetBytes(upstream.lastBody, "dimensions").Int())
}

func TestForwardVideos_ExtractsBillingMetadataFromRequest(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-4.3","prompt":"make a video","seconds":4,"size":"720x1280","n_variants":2}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"video-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"video-job","object":"video","model":"grok-4.3"}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       43,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")

	require.NoError(t, err)
	require.Equal(t, "grok-4.3", result.Model)
	require.Equal(t, "video-rid", result.RequestID)
	require.Equal(t, 4, result.VideoSeconds)
	require.Equal(t, VideoBillingTier720p, result.VideoSize)
	require.Equal(t, 2, result.VideoCount)
}

func TestForwardVideos_NormalizesRootAliasToV1Videos(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-4.3","prompt":"make a video","seconds":4}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"video-job","object":"video","model":"grok-4.3"}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       44,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	_, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/videos")

	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://api.x.ai/v1/videos", upstream.lastReq.URL.String())
}

func TestForwardVideos_ServerErrorReturnsFailoverWithoutWritingResponse(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-4.3","prompt":"make a video","seconds":4}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"video-fail-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{"error":{"type":"server_error","message":"video backend unavailable"}}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       45,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr), "video 5xx must trigger handler failover")
	require.Equal(t, http.StatusInternalServerError, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "video backend unavailable")
	require.Empty(t, rec.Body.String(), "failover path must not write upstream error before handler exhausts accounts")
}

func TestForwardEmbeddings_CyberPolicyMarksAndDoesNotFailover(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"text-embedding-3-large","input":"blocked text"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := `{"error":{"code":"cyber_policy","message":"blocked by policy"}}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusForbidden,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"emb-cyber-rid"},
		},
		Body: io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       43,
		Name:     "embedding-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	result, err := svc.ForwardEmbeddings(context.Background(), c, account, reqBody, "")

	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, upstreamBody, rec.Body.String())
	mark := GetOpsCyberPolicy(c)
	require.NotNil(t, mark)
	require.Equal(t, "cyber_policy", mark.Code)
	require.Equal(t, "blocked by policy", mark.Message)
	require.Equal(t, http.StatusForbidden, mark.UpstreamStatus)
}
