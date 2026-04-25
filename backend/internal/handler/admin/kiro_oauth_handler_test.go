package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestKiroOAuthHandlerRefreshTokenRejectsIDCRefreshAliasesWithoutClientCredentials(t *testing.T) {
	t.Parallel()

	for _, authMethod := range []string{"IDC", "builder-id", "iam"} {
		authMethod := authMethod
		t.Run(authMethod, func(t *testing.T) {
			t.Parallel()

			gin.SetMode(gin.TestMode)
			router := gin.New()
			handler := NewKiroOAuthHandler(nil, service.NewKiroTokenRefresher())
			router.POST("/refresh-token", handler.RefreshToken)

			body := []byte(`{"credentials":{"refresh_token":"refresh-token","auth_method":"` + authMethod + `"}}`)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/refresh-token", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, rec.Body.String(), "kiro idc client_id and client_secret are required")
		})
	}
}
