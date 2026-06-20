package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayHandlerHandleResponsesFailoverExhausted_DoesNotSpecialCaseSilentRefusal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	h := &GatewayHandler{}
	h.handleResponsesFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode:   http.StatusTooManyRequests,
		ResponseBody: []byte(`{"error":{"code":"openai_silent_refusal","message":"empty completion"}}`),
	}, false)

	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.JSONEq(t, `{"error":{"code":"server_error","message":"All available accounts exhausted"}}`, w.Body.String())
}

func TestGatewayHandlerHandleResponsesFailoverExhausted_StreamStartedEmitsResponseFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	h := &GatewayHandler{}
	h.handleResponsesFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode: http.StatusTooManyRequests,
	}, true)

	require.Contains(t, w.Body.String(), "event: response.failed")
	require.Contains(t, w.Body.String(), "All available accounts exhausted")
	require.NotContains(t, w.Body.String(), "data: [DONE]")
}

func TestGatewayHandlerHandleCCFailoverExhausted_StreamStartedEmitsErrorAndDone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	h := &GatewayHandler{}
	h.handleCCFailoverExhausted(c, &service.UpstreamFailoverError{
		StatusCode: http.StatusTooManyRequests,
	}, true)

	require.Contains(t, w.Body.String(), `data: {"type":"error"`)
	require.Contains(t, w.Body.String(), "All available accounts exhausted")
	require.Contains(t, w.Body.String(), "data: [DONE]")
}
