package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestGatewayEnsureForwardErrorResponse_WritesFallbackWhenNotWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false, nil)

	require.True(t, wrote)
	require.Equal(t, http.StatusBadGateway, w.Code)

	var parsed map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "error", parsed["type"])
	errorObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upstream_error", errorObj["type"])
	assert.Equal(t, "Upstream request failed", errorObj["message"])
}

// Writer 已写后 ensureForwardErrorResponse 必须把错误以 SSE 形式追加，
// 而不是 silent EOF。非 /responses 路径走 legacy data:{"type":"error"} 分支。
func TestGatewayEnsureForwardErrorResponse_AppendsSSEAfterWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.WriteHeader(http.StatusTeapot)
	_, _ = c.Writer.WriteString("already written")

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false, nil)

	require.True(t, wrote)
	require.Equal(t, http.StatusTeapot, w.Code)
	assert.Contains(t, w.Body.String(), "already written")
	assert.Contains(t, w.Body.String(), `data: {"type":"error"`)
}

func TestGatewayEnsureForwardErrorResponse_DoesNotAppendAfterJSONWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.JSON(http.StatusBadRequest, gin.H{
		"error": gin.H{
			"type":    "invalid_request_error",
			"message": "already written",
		},
	})

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false, errors.New("upstream error: 400"))

	require.False(t, wrote)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NotContains(t, w.Body.String(), `data: {"type":"error"`)
	require.Contains(t, w.Body.String(), "already written")
}

// case B 回归：Anthropic-backed /responses，Writer 已被写过时
// ensureForwardErrorResponse 仍要发 response.failed。
func TestGatewayEnsureForwardErrorResponse_ResponsesRouteAfterWrittenEmitsResponseFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointResponses, nil)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	_, _ = c.Writer.WriteString(":\n\n")

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false, nil)

	require.True(t, wrote)
	body := w.Body.String()
	assert.Contains(t, body, ":\n\n")
	assert.Contains(t, body, "event: response.failed\n")
	assert.Contains(t, body, `"type":"response.failed"`)
}

func TestGatewayEnsureForwardErrorResponse_DoesNotAppendAfterServiceCommitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointResponses, nil)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	_, _ = c.Writer.WriteString("event: response.failed\n")
	_, _ = c.Writer.WriteString(`data: {"type":"response.failed"}` + "\n\n")
	service.MarkResponseCommitted(c)

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, true, errors.New("stream read error"))

	require.False(t, wrote)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, strings.Count(w.Body.String(), "event: response.failed\n"))
}

func TestGatewayEnsureForwardErrorResponse_UsesDetailedForwardError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false, errors.New("dial tcp 127.0.0.1:443: connect: connection refused"))

	require.True(t, wrote)
	require.Equal(t, http.StatusBadGateway, w.Code)

	var parsed map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "error", parsed["type"])
	errorObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upstream_connect_error", errorObj["type"])
	assert.Equal(t, "Failed to connect to upstream service", errorObj["message"])
}

func TestGatewayEnsureForwardErrorResponse_ClientDisconnectSkipsFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(reqCtx)

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false, context.Canceled)

	require.False(t, wrote)
	require.False(t, c.Writer.Written())
	require.Empty(t, w.Body.String())
}

func TestGatewayEnsureForwardErrorResponse_PreservesExistingDetailedContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	service.SetOpsUpstreamErrorWithType(
		c,
		"upstream_http2_internal_error",
		0,
		"Upstream HTTP/2 peer reset the stream with INTERNAL_ERROR",
		"stream error: stream ID 3; INTERNAL_ERROR; received from peer",
	)

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false, errors.New(`upstream request failed: GET "https://api.example.com/v1?access_token=secret&refresh_token=abc"`))

	require.True(t, wrote)
	require.Equal(t, http.StatusBadGateway, w.Code)

	var parsed map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &parsed)
	require.NoError(t, err)
	errorObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "upstream_http2_internal_error", errorObj["type"])
	assert.Equal(t, "Upstream HTTP/2 peer reset the stream with INTERNAL_ERROR", errorObj["message"])

	typed, _ := c.Get(service.OpsUpstreamErrorTypeKey)
	assert.Equal(t, "upstream_http2_internal_error", typed)
	detail, _ := c.Get(service.OpsUpstreamErrorDetailKey)
	assert.Equal(t, "stream error: stream ID 3; INTERNAL_ERROR; received from peer", detail)
}
