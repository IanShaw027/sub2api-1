package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCommonRoutesClaudeAuxTelemetryStubReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/event_logging/batch", strings.NewReader(`{"events":[]}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{}`, w.Body.String())
}

func TestCommonRoutesClaudeAuxPolicyLimitsStub(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCommonRoutes(router, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/claude_code/policy_limits", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"restrictions":{}}`, w.Body.String())
}
