package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOpenAILenientJSONHandlerContext(t *testing.T, path string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	groupID := int64(2)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		ID:      101,
		GroupID: &groupID,
		User:    &service.User{ID: 1},
		Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowMessagesDispatch: true},
	})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 1})
	return c, rec
}

func TestOpenAIResponses_HandlerAcceptsLenientJSON(t *testing.T) {
	body := []byte("\xef\xbb\xbf{\"input\":\"hello\x00world\"}")
	c, rec := newOpenAILenientJSONHandlerContext(t, "/v1/responses", body)

	h := newOpenAIHandlerForPreviousResponseIDValidation(t, nil)
	h.Responses(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "model is required")
	require.NotContains(t, rec.Body.String(), "Failed to parse request body")
}

func TestOpenAIMessages_HandlerAcceptsLenientJSON(t *testing.T) {
	body := []byte("\xef\xbb\xbf{\"messages\":[{\"role\":\"user\",\"content\":\"hello\x00world\"}]}")
	c, rec := newOpenAILenientJSONHandlerContext(t, "/v1/messages", body)

	h := newOpenAIHandlerForPreviousResponseIDValidation(t, nil)
	h.Messages(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "model is required")
	require.NotContains(t, rec.Body.String(), "Failed to parse request body")
}
