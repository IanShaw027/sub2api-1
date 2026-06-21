//go:build unit

package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterTLSFingerprintCaptureRoutesIncludesPublicSubmit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	handlers := &handler.Handlers{
		Admin: &handler.AdminHandlers{
			TLSFingerprintProfile: &admin.TLSFingerprintProfileHandler{},
		},
	}

	RegisterTLSFingerprintCaptureRoutes(v1, handlers)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tls-fingerprint-captures/submit", nil)
	router.ServeHTTP(rec, req)

	require.NotEqual(t, http.StatusNotFound, rec.Code)
}

func TestRegisterTLSFingerprintCaptureRoutesLimitsPublicSubmitBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	v1 := router.Group("/api/v1")
	handlers := &handler.Handlers{
		Admin: &handler.AdminHandlers{
			TLSFingerprintProfile: &admin.TLSFingerprintProfileHandler{},
		},
	}

	RegisterTLSFingerprintCaptureRoutes(v1, handlers)

	oversizedPayload := strings.Repeat("x", 300*1024)
	body := `{"token":"token","platform":"openai","user_agent":"codex","payload":"` + oversizedPayload + `"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tls-fingerprint-captures/submit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "request body too large")
}
