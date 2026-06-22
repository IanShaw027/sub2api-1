//go:build unit

package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterTLSFingerprintCaptureRoutesDoesNotExposePublicSubmit(t *testing.T) {
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

	require.Equal(t, http.StatusNotFound, rec.Code)
}
