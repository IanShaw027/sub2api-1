package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIHandleFailoverExhausted_RetryableOverloadReturns503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	h := &OpenAIGatewayHandler{}
	h.handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:             http.StatusServiceUnavailable,
		RetryableOnSameAccount: true,
		ResponseBody:           []byte(`{"error":{"type":"upstream_error","message":"Upstream service overloaded, please retry later"}}`),
	}, nil, false)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	errObj, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "overloaded_error", errObj["type"])
	require.Equal(t, "Upstream service overloaded, please retry later", errObj["message"])
}

func TestOpenAIHandleFailoverExhausted_StreamRetryableOverloadReturnsSSEOverloadError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	h := &OpenAIGatewayHandler{}
	h.handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:             http.StatusServiceUnavailable,
		RetryableOnSameAccount: true,
		ResponseBody:           []byte(`{"error":{"type":"upstream_error","message":"Upstream service overloaded, please retry later"}}`),
	}, nil, true)

	body := w.Body.String()
	require.Contains(t, body, "event: response.failed\n")
	require.Contains(t, body, `"type":"response.failed"`)
	require.Contains(t, body, `"code":"overloaded_error"`)
	require.Contains(t, body, `"message":"Upstream service overloaded, please retry later"`)
	require.NotContains(t, body, "Upstream service temporarily unavailable")
}

func TestOpenAIHandleFailoverExhausted_PartialSSEOutputKeepsStreamingErrorShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Header("Content-Type", "text/event-stream")

	_, err := c.Writer.Write([]byte("data: {\"type\":\"response.output_text.delta\"}\n\n"))
	require.NoError(t, err)

	h := &OpenAIGatewayHandler{}
	h.handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:             http.StatusServiceUnavailable,
		RetryableOnSameAccount: true,
		ResponseBody:           []byte(`{"error":{"type":"upstream_error","message":"Upstream service overloaded, please retry later"}}`),
	}, nil, false)

	body := w.Body.String()
	require.Contains(t, body, "data: {\"type\":\"response.output_text.delta\"}\n\n")
	require.Contains(t, body, "event: response.failed\n")
	require.Contains(t, body, `"type":"response.failed"`)
	require.Contains(t, body, `"code":"overloaded_error"`)
	require.Contains(t, body, `"message":"Upstream service overloaded, please retry later"`)
}

func TestOpenAIHandleFailoverExhausted_Generic503StillReturnsBadGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	h := &OpenAIGatewayHandler{}
	h.handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:   http.StatusServiceUnavailable,
		ResponseBody: []byte(`{"error":{"type":"upstream_error","message":"temporary maintenance"}}`),
	}, nil, false)

	require.Equal(t, http.StatusBadGateway, w.Code)
	require.Contains(t, strings.TrimSpace(w.Body.String()), "Upstream service temporarily unavailable")
}

func TestOpenAIHandleFailoverExhausted_ContextWindowIsClientVisible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	h := &OpenAIGatewayHandler{}
	h.handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:   http.StatusBadRequest,
		ResponseBody: []byte(`{"error":{"code":"context_length_exceeded","message":"Your input exceeds the context window of this model. Please adjust your input and try again."}}`),
	}, nil, false)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid_request_error")
	require.Contains(t, w.Body.String(), "context window")
	require.NotContains(t, w.Body.String(), "Upstream request failed")
}

func TestOpenAIHandleFailoverExhausted_DeactivatedWorkspaceIsClientVisible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	h := &OpenAIGatewayHandler{}
	h.handleFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:   http.StatusPaymentRequired,
		ResponseBody: []byte(`{"detail":{"code":"deactivated_workspace"}}`),
	}, nil, false)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "Upstream workspace is deactivated")
}

func TestClearOpenAIStreamRetryReplayState_RemovesPriorAccountReplayState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set("openai_stream_retry_replay_state", map[string]any{
		"visibleFrameSignatures": []string{"response.created"},
		"emittedTextPrefix":      "partial output",
	})
	c.Set("unrelated", "keep")

	clearOpenAIStreamRetryReplayState(c)

	_, exists := c.Get("openai_stream_retry_replay_state")
	require.False(t, exists)
	other, exists := c.Get("unrelated")
	require.True(t, exists)
	require.Equal(t, "keep", other)
}
