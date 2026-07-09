package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestComplianceHandlerGetStatus_NilSettingServiceReturnsInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/compliance", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})

	handler := NewComplianceHandler(nil)
	require.NotPanics(t, func() {
		handler.GetStatus(c)
	})
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "setting service is unavailable")
}

func TestComplianceHandlerAccept_NilSettingServiceReturnsInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/compliance/accept",
		strings.NewReader(`{"phrase":"ok","language":"zh"}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})

	handler := NewComplianceHandler(nil)
	require.NotPanics(t, func() {
		handler.Accept(c)
	})
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "setting service is unavailable")
}
