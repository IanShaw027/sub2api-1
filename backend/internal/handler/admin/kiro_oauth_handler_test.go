package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestKiroOAuthHandlerDeviceCompleteRequiresSessionID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewKiroOAuthHandler(nil, nil)
	router.POST("/device-complete", handler.DeviceComplete)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/device-complete", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestKiroOAuthHandlerDeviceCompleteRequiresConfiguredService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewKiroOAuthHandler(nil, nil)
	router.POST("/device-complete", handler.DeviceComplete)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/device-complete", bytes.NewReader([]byte(`{"session_id":"session-1"}`)))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "Kiro OAuth service is not configured")
}

func TestKiroExchangeCallbackResponseData_UnwrapsTokenInfo(t *testing.T) {
	t.Parallel()

	result := &service.KiroOAuthProgressResult{
		TokenInfo: &service.KiroTokenInfo{
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		},
	}

	data := kiroExchangeCallbackResponseData(result)

	tokenInfo, ok := data.(*service.KiroTokenInfo)
	require.True(t, ok)
	require.Equal(t, "access-token", tokenInfo.AccessToken)
	require.Equal(t, "refresh-token", tokenInfo.RefreshToken)
}

func TestKiroExchangeCallbackResponseData_PreservesContinuationWrapper(t *testing.T) {
	t.Parallel()

	result := &service.KiroOAuthProgressResult{
		Continuation: &service.KiroIDCContinuationInfo{
			SessionID:       "session-1",
			Status:          "authorization_pending",
			AuthMethod:      "idc",
			IntervalSeconds: 5,
			ExpiresAt:       time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339),
		},
	}

	data := kiroExchangeCallbackResponseData(result)

	progress, ok := data.(*service.KiroOAuthProgressResult)
	require.True(t, ok)
	require.NotNil(t, progress.Continuation)
	require.Equal(t, "session-1", progress.Continuation.SessionID)
	require.Equal(t, "authorization_pending", progress.Continuation.Status)
}

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

func TestKiroOAuthHandlerRefreshTokenRejectsShortRefreshToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewKiroOAuthHandler(nil, service.NewKiroTokenRefresher())
	router.POST("/refresh-token", handler.RefreshToken)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/refresh-token", bytes.NewReader([]byte(`{"credentials":{"refresh_token":"short"}}`)))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "kiro refresh_token is too short")
}

func TestKiroOAuthHandlerRefreshTokenRejectsTruncatedRefreshToken(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewKiroOAuthHandler(nil, service.NewKiroTokenRefresher())
	router.POST("/refresh-token", handler.RefreshToken)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/refresh-token", bytes.NewReader([]byte(`{"credentials":{"refresh_token":"kiro-refresh-token..."}}`)))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "kiro refresh_token appears truncated")
}
