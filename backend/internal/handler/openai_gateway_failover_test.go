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
	}, false)

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
	}, true)

	body := w.Body.String()
	require.Contains(t, body, "event: error\n")
	require.Contains(t, body, `"type":"overloaded_error"`)
	require.Contains(t, body, `"message":"Upstream service overloaded, please retry later"`)
	require.NotContains(t, body, "Upstream service temporarily unavailable")
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
	}, false)

	require.Equal(t, http.StatusBadGateway, w.Code)
	require.Contains(t, strings.TrimSpace(w.Body.String()), "Upstream service temporarily unavailable")
}
