package admin

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
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

func TestKiroOAuthHandlerGenerateAuthURLRejectsMalformedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewKiroOAuthHandler(nil, nil)
	router.POST("/auth-url", handler.GenerateAuthURL)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth-url", strings.NewReader(`{`))
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

func TestKiroOAuthHandlerRefreshTokenRejectsExternalIDPWhenKiroUsageForbidden(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	upstream := &kiroExternalIDPValidationUpstream{}
	oauthService := service.NewKiroOAuthService(nil, upstream, &service.TLSFingerprintProfileService{}, nil)
	refresher := service.NewKiroTokenRefresher().WithTransport(upstream, &service.TLSFingerprintProfileService{})
	handler := NewKiroOAuthHandler(oauthService, refresher)
	router := gin.New()
	router.POST("/refresh-token", handler.RefreshToken)

	body := []byte(`{"credentials":{
		"refresh_token":"external-refresh-token",
		"auth_method":"external_idp",
		"client_id":"microsoft-public-client",
		"token_endpoint":"https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
		"profile_arn":"arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD"
	}}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/refresh-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "Kiro rejected")
	require.True(t, upstream.sawMicrosoftRefresh, "should still validate the external IdP refresh token first")
	require.True(t, upstream.sawKiroUsageCheck, "should verify refreshed credentials against Kiro before returning success")
	require.Equal(t, "EXTERNAL_IDP", upstream.kiroTokenType)
}

type kiroExternalIDPValidationUpstream struct {
	sawMicrosoftRefresh bool
	sawKiroUsageCheck   bool
	kiroTokenType       string
}

func (u *kiroExternalIDPValidationUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *kiroExternalIDPValidationUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	switch {
	case req.Method == http.MethodPost && strings.EqualFold(req.URL.Host, "login.microsoftonline.com"):
		u.sawMicrosoftRefresh = true
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		form := string(body)
		if !strings.Contains(form, "grant_type=refresh_token") {
			return responseWithStatus(http.StatusBadRequest, `{"error":"invalid_request"}`), nil
		}
		return responseWithStatus(http.StatusOK, `{
			"access_token":"new-ms-access-token",
			"refresh_token":"new-ms-refresh-token",
			"expires_in":3600,
			"token_type":"Bearer"
		}`), nil
	case req.Method == http.MethodGet && strings.EqualFold(req.URL.Host, "q.us-east-1.amazonaws.com") && req.URL.Path == "/getUsageLimits":
		u.sawKiroUsageCheck = true
		u.kiroTokenType = req.Header.Get("TokenType")
		return responseWithStatus(http.StatusForbidden, `{"message":"User is not authorized to make this call."}`), nil
	default:
		return responseWithStatus(http.StatusInternalServerError, `{"message":"unexpected request"}`), nil
	}
}

func responseWithStatus(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
