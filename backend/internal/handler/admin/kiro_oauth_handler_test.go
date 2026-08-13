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

func TestKiroOAuthHandlerDiscoverProfilesFromCredentials(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	upstream := &kiroDiscoverProfilesUpstream{}
	refresher := service.NewKiroTokenRefresher().WithTransport(upstream, &service.TLSFingerprintProfileService{})
	handler := NewKiroOAuthHandler(nil, refresher)
	router := gin.New()
	router.POST("/discover-profiles", handler.DiscoverProfiles)

	body := []byte(`{"credentials":{"refresh_token":"kiro-refresh-token-valid-123456","auth_method":"social","region":"us-east-1"}}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/discover-profiles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equalf(t, http.StatusOK, rec.Code, "body=%s requests=%v", rec.Body.String(), upstream.requests)
	require.True(t, upstream.sawRefresh)
	require.True(t, upstream.sawAvailableProfiles)
	require.Contains(t, rec.Body.String(), "eu-profile")
	require.Contains(t, rec.Body.String(), "arn:aws:codewhisperer:eu-central-1:123:profile/EU")
}

func TestKiroOAuthHandlerDiscoverProfilesRequiresConfiguredRefreshService(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewKiroOAuthHandler(nil, nil)
	router.POST("/discover-profiles", handler.DiscoverProfiles)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/discover-profiles", bytes.NewReader([]byte(`{"credentials":{"refresh_token":"kiro-refresh-token-valid-123456"}}`)))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), "Kiro OAuth refresh service is not configured")
}

func TestKiroOAuthHandlerDiscoverProfilesRejectsIDCWithoutClientCredentials(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewKiroOAuthHandler(nil, service.NewKiroTokenRefresher())
	router.POST("/discover-profiles", handler.DiscoverProfiles)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/discover-profiles", bytes.NewReader([]byte(`{"credentials":{"refresh_token":"kiro-refresh-token-valid-123456","auth_method":"idc"}}`)))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "kiro idc client_id and client_secret are required")
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

type kiroDiscoverProfilesUpstream struct {
	sawRefresh           bool
	sawAvailableProfiles bool
	requests             []string
}

func (u *kiroDiscoverProfilesUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *kiroDiscoverProfilesUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.requests = append(u.requests, req.Method+" "+req.URL.String()+" target="+req.Header.Get("x-amz-target"))
	switch {
	case req.Method == http.MethodPost && strings.HasSuffix(strings.ToLower(req.URL.Host), ".auth.desktop.kiro.dev") && req.URL.Path == "/refreshToken":
		u.sawRefresh = true
		return responseWithStatus(http.StatusOK, `{
			"accessToken":"new-access-token",
			"refreshToken":"new-refresh-token",
			"expiresIn":3600,
			"userId":"user-1"
		}`), nil
	case req.Method == http.MethodPost &&
		strings.HasPrefix(strings.ToLower(req.URL.Host), "q.") &&
		req.URL.Path == "/" &&
		req.Header.Get("x-amz-target") == "AmazonCodeWhispererService.ListAvailableProfiles":
		u.sawAvailableProfiles = true
		return responseWithStatus(http.StatusOK, `{
			"profiles":[
				{"arn":"arn:aws:codewhisperer:eu-central-1:123:profile/EU","profileName":"eu-profile","region":"eu-central-1"},
				{"arn":"arn:aws:codewhisperer:us-east-1:123:profile/US","profileName":"us-profile","region":"us-east-1"}
			]
		}`), nil
	default:
		return responseWithStatus(http.StatusInternalServerError, `{"message":"unexpected request"}`), nil
	}
}
