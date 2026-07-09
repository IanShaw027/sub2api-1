package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type blockingVideoContentBody struct {
	firstChunk []byte
	release    <-chan struct{}
	mu         sync.Mutex
	readCount  int
}

func (b *blockingVideoContentBody) Read(p []byte) (int, error) {
	b.mu.Lock()
	b.readCount++
	readCount := b.readCount
	b.mu.Unlock()
	if readCount == 1 {
		return copy(p, b.firstChunk), nil
	}
	<-b.release
	return 0, io.EOF
}

func (b *blockingVideoContentBody) Close() error { return nil }

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

func TestForwardVideos_ExtractsBillingMetadataFromXAIOfficialFields(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-imagine-video","prompt":"make a video","duration":8,"aspect_ratio":"16:9","resolution":"1080p"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"video-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{"request_id":"video-job","model":"grok-imagine-video","status":"pending"}`)),
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

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos/generations")

	require.NoError(t, err)
	require.Equal(t, "grok-imagine-video", result.Model)
	require.Equal(t, "video-rid", result.RequestID)
	require.Equal(t, 8, result.VideoSeconds)
	require.Equal(t, VideoBillingTier1080p, result.VideoSize)
	require.Equal(t, 1, result.VideoCount)
}

func TestForwardVideos_DoesNotFallbackToRequestedVariantCountWhenResponseHasEmptyDataArray(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-4.3","prompt":"make a video","seconds":4,"n_variants":5}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"video-rid-empty-data"},
		},
		Body: io.NopCloser(strings.NewReader(`{"data":[],"model":"grok-4.3","status":"pending"}`)),
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

	require.NoError(t, err)
	require.Equal(t, "grok-4.3", result.Model)
	require.Equal(t, "video-rid-empty-data", result.RequestID)
	require.Equal(t, 0, result.VideoCount)
}

func TestForwardVideos_FiltersUpstreamResponseHeaders(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-4.3","prompt":"make a video","seconds":4}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Set-Cookie":   []string{"secret=upstream; Path=/; HttpOnly"},
			"X-Request-Id": []string{"video-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"video-job","object":"video","model":"grok-4.3"}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       46,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	_, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Empty(t, rec.Header().Values("Set-Cookie"))
	require.Equal(t, "video-rid", rec.Header().Get("X-Request-Id"))
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

func TestForwardVideos_GETDoesNotExposeBillingMetadata(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video-job", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"video-job","object":"video","model":"grok-4.3","size":"720p","duration":8}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       47,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	result, err := svc.ForwardVideos(context.Background(), c, account, nil, "/v1/videos/video-job")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, OpenAIForwardResultHasVideoBillingForUsage(result))
	require.Zero(t, result.VideoSeconds)
	require.Zero(t, result.VideoCount)
	require.Empty(t, result.VideoSize)
}

func TestForwardVideos_GETContentStreamsBeforeUpstreamEOF(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video-job/content", nil)
	c.Request.Header.Set("Accept", "video/mp4")

	release := make(chan struct{})
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"video/mp4"},
		},
		Body: &blockingVideoContentBody{
			firstChunk: []byte("first-video-bytes"),
			release:    release,
		},
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       51,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	done := make(chan error, 1)
	go func() {
		_, err := svc.ForwardVideos(context.Background(), c, account, nil, "/v1/videos/video-job/content")
		done <- err
	}()
	defer func() {
		close(release)
		require.NoError(t, <-done)
	}()

	require.Eventually(t, func() bool {
		return strings.Contains(rec.Body.String(), "first-video-bytes")
	}, 500*time.Millisecond, 10*time.Millisecond, "video content should be flushed before upstream EOF")
	require.Equal(t, "video/mp4", rec.Header().Get("Content-Type"))
}

func TestForwardVideos_GETContentFlushesToHTTPClientBeforeUpstreamEOF(t *testing.T) {
	setGinTestMode()

	release := make(chan struct{})
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"video/mp4"},
		},
		Body: &blockingVideoContentBody{
			firstChunk: []byte("first-video-bytes"),
			release:    release,
		},
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       52,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := gin.CreateTestContext(w)
		c.Request = r
		_, err := svc.ForwardVideos(r.Context(), c, account, nil, "/v1/videos/video-job/content")
		if err != nil && !c.Writer.Written() {
			http.Error(w, err.Error(), http.StatusBadGateway)
		}
	}))
	defer downstream.Close()
	var releaseOnce sync.Once
	releaseUpstream := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}

	got := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		client := downstream.Client()
		client.Timeout = 2 * time.Second
		resp, err := client.Get(downstream.URL + "/v1/videos/video-job/content")
		if err != nil {
			errCh <- err
			return
		}
		defer func() { _ = resp.Body.Close() }()
		buf := make([]byte, len("first-video-bytes"))
		n, readErr := io.ReadFull(resp.Body, buf)
		if readErr != nil {
			errCh <- readErr
			return
		}
		got <- string(buf[:n])
	}()
	defer releaseUpstream()

	select {
	case chunk := <-got:
		require.Equal(t, "first-video-bytes", chunk)
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(500 * time.Millisecond):
		releaseUpstream()
		downstream.CloseClientConnections()
		t.Fatal("video content should be flushed to a real HTTP client before upstream EOF")
	}
}

func TestForwardVideos_RejectsNonGrokAccounts(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/videos?limit=10&after=abc", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"object":"list","data":[]}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       48,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
		},
	}

	_, err := svc.ForwardVideos(context.Background(), c, account, nil, "/videos?limit=10&after=abc")

	require.Error(t, err)
	require.Contains(t, err.Error(), "only supported for Grok accounts")
	require.Nil(t, upstream.lastReq)
}

func TestForwardVideos_GrokForwardsQueryStringAndNormalizesRootAlias(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/videos?limit=10&after=abc", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"object":"list","data":[]}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       50,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	_, err := svc.ForwardVideos(context.Background(), c, account, nil, "/videos?limit=10&after=abc")

	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://api.x.ai/v1/videos?limit=10&after=abc", upstream.lastReq.URL.String())
}

func TestForwardVideos_PreservesMultipartContentType(t *testing.T) {
	setGinTestMode()

	reqBody := []byte("--boundary\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\ngrok-4.3\r\n--boundary--\r\n")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/edits", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"video-job","object":"video","model":"grok-4.3"}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       49,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	_, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos/edits")

	require.NoError(t, err)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "multipart/form-data; boundary=boundary", upstream.lastReq.Header.Get("Content-Type"))
	require.Equal(t, reqBody, upstream.lastBody)
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

func TestForwardVideos_ClientErrorRecordsOpsUpstreamError(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-4.3","prompt":"make a video","seconds":4}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"video-bad-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"bad video request"}}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       46,
		Name:     "grok-video",
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")

	require.Nil(t, result)
	require.ErrorContains(t, err, "upstream video error")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "bad video request")
	require.Equal(t, http.StatusBadRequest, c.GetInt(OpsUpstreamStatusCodeKey))
	require.Equal(t, "bad video request", c.GetString(OpsUpstreamErrorMessageKey))
	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, PlatformGrok, events[0].Platform)
	require.Equal(t, int64(46), events[0].AccountID)
	require.Equal(t, http.StatusBadRequest, events[0].UpstreamStatusCode)
	require.Equal(t, "video-bad-rid", events[0].UpstreamRequestID)
	require.Equal(t, "bad video request", events[0].Message)
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

func TestForwardVideos_GETStatusReadErrorFailsWithoutWritingTruncatedBody(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video-job-status", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: errReadCloser{err: errors.New("status read failed")},
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       47,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	result, err := svc.ForwardVideos(context.Background(), c, account, nil, "/v1/videos/video-job-status")

	require.Nil(t, result)
	require.ErrorContains(t, err, "read upstream video status response")
	require.Empty(t, rec.Body.String())
}

func TestForwardVideos_ClientErrorReadErrorFailsWithoutParsingTruncatedBody(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-4.3","prompt":"make a video","seconds":4}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: errReadCloser{err: errors.New("error body read failed")},
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       48,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")

	require.Nil(t, result)
	require.ErrorContains(t, err, "read upstream video response")
	require.Empty(t, rec.Body.String())
}
