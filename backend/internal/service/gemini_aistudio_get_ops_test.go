package service

import (
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
)

type geminiAIStudioErrReadCloser struct {
	err error
}

func (r geminiAIStudioErrReadCloser) Read([]byte) (int, error) {
	return 0, r.err
}

func (r geminiAIStudioErrReadCloser) Close() error {
	return nil
}

func TestGeminiMessagesCompatService_ForwardAIStudioGET_SetsOpsUpstreamLatency(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)

	upstream := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"models":[]}`)),
		},
	}

	svc := &GeminiMessagesCompatService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}

	result, err := svc.ForwardAIStudioGET(context.Background(), c, &Account{
		ID:          1,
		Platform:    PlatformGemini,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com",
		},
	}, "/v1beta/models")

	require.NoError(t, err)
	require.NotNil(t, result)

	latencyValue, ok := c.Get(OpsUpstreamLatencyMsKey)
	require.True(t, ok)
	require.IsType(t, int64(0), latencyValue)
}

func TestGeminiMessagesCompatService_ForwardAIStudioGET_TooLargeBodyReturnsError(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)

	oversized := strings.Repeat("a", geminiAIStudioGETMaxBodyBytes+1)
	upstream := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-Id": []string{"rid-aistudio-too-large"},
			},
			Body: io.NopCloser(strings.NewReader(oversized)),
		},
	}

	svc := &GeminiMessagesCompatService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}

	result, err := svc.ForwardAIStudioGET(context.Background(), c, &Account{
		ID:          1,
		Platform:    PlatformGemini,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com",
		},
	}, "/v1beta/models")

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "response too large")

	upstreamMessage, ok := c.Get(OpsUpstreamErrorMessageKey)
	require.True(t, ok)
	require.Equal(t, "upstream response too large", upstreamMessage)

	raw, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := raw.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "upstream_response_too_large", events[0].Kind)
	require.Equal(t, http.StatusBadGateway, events[0].UpstreamStatusCode)
	require.Equal(t, "rid-aistudio-too-large", events[0].UpstreamRequestID)
	require.Contains(t, events[0].UpstreamURL, "/v1beta/models")
}

func TestGeminiMessagesCompatService_ForwardAIStudioGET_ReadErrorRecordsOpsContext(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)

	upstream := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-Id": []string{"rid-aistudio-read"},
			},
			Body: geminiAIStudioErrReadCloser{err: errors.New("read tcp reset by peer")},
		},
	}

	svc := &GeminiMessagesCompatService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}

	result, err := svc.ForwardAIStudioGET(context.Background(), c, &Account{
		ID:          1,
		Name:        "gemini-aistudio",
		Platform:    PlatformGemini,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com",
		},
	}, "/v1beta/models")

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "read ai studio get response failed")

	statusCode, ok := c.Get(OpsUpstreamStatusCodeKey)
	require.True(t, ok)
	require.Equal(t, http.StatusOK, statusCode)

	upstreamMessage, ok := c.Get(OpsUpstreamErrorMessageKey)
	require.True(t, ok)
	require.Equal(t, "read tcp reset by peer", upstreamMessage)

	raw, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := raw.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "upstream_read_error", events[0].Kind)
	require.Equal(t, http.StatusOK, events[0].UpstreamStatusCode)
	require.Equal(t, "rid-aistudio-read", events[0].UpstreamRequestID)
	require.Contains(t, events[0].UpstreamURL, "/v1beta/models")
}

func TestGeminiMessagesCompatService_ForwardAIStudioGET_RequestErrorRecordsOpsContext(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)

	upstream := &geminiCompatHTTPUpstreamStub{
		err: errors.New("dial tcp timeout"),
	}

	svc := &GeminiMessagesCompatService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}

	result, err := svc.ForwardAIStudioGET(context.Background(), c, &Account{
		ID:          1,
		Name:        "gemini-aistudio",
		Platform:    PlatformGemini,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com",
		},
	}, "/v1beta/models")

	require.Error(t, err)
	require.Nil(t, result)

	upstreamMessage, ok := c.Get(OpsUpstreamErrorMessageKey)
	require.True(t, ok)
	require.Equal(t, "Upstream transport request failed", upstreamMessage)

	raw, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := raw.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "request_error", events[0].Kind)
	require.Equal(t, 0, events[0].UpstreamStatusCode)
	require.Contains(t, events[0].UpstreamURL, "/v1beta/models")
}

func TestGeminiMessagesCompatService_ForwardAIStudioGET_HTTPErrorRecordsOpsContext(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)

	upstream := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header: http.Header{
				"Content-Type":     []string{"application/json"},
				"X-Request-Id":     []string{"rid-aistudio-503"},
				"Www-Authenticate": []string{"Bearer realm=\"google\""},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"backend unavailable","status":"UNAVAILABLE"}}`)),
		},
	}

	svc := &GeminiMessagesCompatService{
		httpUpstream: upstream,
		cfg:          &config.Config{},
	}

	result, err := svc.ForwardAIStudioGET(context.Background(), c, &Account{
		ID:          1,
		Name:        "gemini-aistudio",
		Platform:    PlatformGemini,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com",
		},
	}, "/v1beta/models")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusServiceUnavailable, result.StatusCode)

	statusCode, ok := c.Get(OpsUpstreamStatusCodeKey)
	require.True(t, ok)
	require.Equal(t, http.StatusServiceUnavailable, statusCode)

	upstreamMessage, ok := c.Get(OpsUpstreamErrorMessageKey)
	require.True(t, ok)
	require.Equal(t, "backend unavailable", upstreamMessage)

	upstreamDetail, ok := c.Get(OpsUpstreamErrorDetailKey)
	require.True(t, ok)
	require.NotEmpty(t, upstreamDetail)

	raw, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := raw.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "http_error", events[0].Kind)
	require.Equal(t, http.StatusServiceUnavailable, events[0].UpstreamStatusCode)
	require.Equal(t, "rid-aistudio-503", events[0].UpstreamRequestID)
	require.Contains(t, events[0].UpstreamURL, "/v1beta/models")
}

func TestGeminiMessagesCompatService_ForwardAIStudioGET_429UpdatesSchedulingState(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)

	upstream := &geminiCompatHTTPUpstreamStub{
		response: &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header: http.Header{
				"Content-Type":      []string{"application/json"},
				"X-Request-Id":      []string{"rid-aistudio-429"},
				"Retry-After":       []string{"60"},
				"X-RateLimit-Reset": []string{"60"},
			},
			Body: io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited","status":"RESOURCE_EXHAUSTED"}}`)),
		},
	}
	repo := &rateLimitAccountRepoStub{}
	svc := &GeminiMessagesCompatService{
		httpUpstream:         upstream,
		cfg:                  &config.Config{},
		rateLimitService:     NewRateLimitService(repo, nil, &config.Config{}, nil, nil),
		responseHeaderFilter: nil,
	}

	result, err := svc.ForwardAIStudioGET(context.Background(), c, &Account{
		ID:          1,
		Name:        "gemini-aistudio",
		Platform:    PlatformGemini,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "gemini-key",
			"base_url": "https://generativelanguage.googleapis.com",
		},
	}, "/v1beta/models")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, repo.rateLimitedCalls)
	require.False(t, repo.lastRateLimitedAt.IsZero())
}
