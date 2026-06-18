//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayHandlerHandleGroupModelUnsupportedError_ReturnsPermissionError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	h := &GatewayHandler{}
	err := &service.GroupModelUnsupportedError{
		Platform:        service.PlatformAnthropic,
		RequestedModel:  "claude-missing",
		AvailableModels: []string{"claude-fable-5"},
	}

	handled := h.handleGroupModelUnsupportedError(c, err, false)

	require.True(t, handled)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"permission_error"`)
	require.Contains(t, rec.Body.String(), `requested model \"claude-missing\"`)
	require.Contains(t, rec.Body.String(), "claude-fable-5")
}

func TestOpenAIHandlerHandleGroupModelUnsupportedError_ReturnsPermissionError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	h := &OpenAIGatewayHandler{}
	err := &service.GroupModelUnsupportedError{
		Platform:        service.PlatformOpenAI,
		RequestedModel:  "gpt-missing",
		AvailableModels: []string{"gpt-5.4-mini"},
	}

	handled := h.handleOpenAIGroupModelUnsupportedError(c, err, false)

	require.True(t, handled)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"permission_error"`)
	require.Contains(t, rec.Body.String(), `requested model \"gpt-missing\"`)
	require.Contains(t, rec.Body.String(), "gpt-5.4-mini")
}

func TestGeminiHandleGroupModelUnsupportedError_ReturnsGooglePermissionError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-missing:generateContent", nil)

	err := &service.GroupModelUnsupportedError{
		Platform:        service.PlatformGemini,
		RequestedModel:  "gemini-missing",
		AvailableModels: []string{"gemini-2.5-flash"},
	}

	handled := handleGeminiGroupModelUnsupportedError(c, err)

	require.True(t, handled)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"PERMISSION_DENIED"`)
	require.Contains(t, rec.Body.String(), `requested model \"gemini-missing\"`)
	require.Contains(t, rec.Body.String(), "gemini-2.5-flash")
}
