package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProxyHandlerUpdateDoesNotMarkNullableFieldsWhenOmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newStubAdminService()
	handler := NewProxyHandler(svc)
	router := gin.New()
	router.PUT("/proxies/:id", handler.Update)

	req := httptest.NewRequest(http.MethodPut, "/proxies/7", bytes.NewBufferString(`{"status":"inactive"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, svc.updatedProxies, 1)
	input := svc.updatedProxies[0]
	require.Equal(t, "inactive", input.Status)
	require.False(t, input.ExpiresAtSet)
	require.False(t, input.FallbackModeSet)
	require.False(t, input.BackupProxyIDSet)
	require.False(t, input.ExpiryWarnDaysSet)
}
