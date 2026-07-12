//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

type grokRoundTripFunc func(*http.Request) (*http.Response, error)

func (f grokRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGrokOAuthClientExchangeAndRefreshUseFormFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		require.Equal(t, "client-id", r.Form.Get("client_id"))

		switch r.Form.Get("grant_type") {
		case "authorization_code":
			require.Equal(t, "auth-code", r.Form.Get("code"))
			require.Equal(t, "http://127.0.0.1:56121/callback", r.Form.Get("redirect_uri"))
			require.Equal(t, "verifier", r.Form.Get("code_verifier"))
			require.Empty(t, r.Form.Get("code_challenge"))
			require.Empty(t, r.Form.Get("code_challenge_method"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "exchange-access",
				"refresh_token": "exchange-refresh",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"scope":         "openid api:access",
			})
		case "refresh_token":
			require.Equal(t, "refresh-token", r.Form.Get("refresh_token"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "refresh-access",
				"refresh_token": "refresh-rotated",
				"token_type":    "Bearer",
				"expires_in":    7200,
			})
		default:
			http.Error(w, "unexpected grant_type", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	t.Setenv(xai.EnvTokenURL, server.URL)

	client := NewGrokOAuthClient()

	exchanged, err := client.ExchangeCode(
		context.Background(),
		"auth-code",
		"verifier",
		"http://127.0.0.1:56121/callback",
		"",
		"client-id",
	)
	require.NoError(t, err)
	require.Equal(t, "exchange-access", exchanged.AccessToken)
	require.Equal(t, "exchange-refresh", exchanged.RefreshToken)
	require.Equal(t, int64(3600), exchanged.ExpiresIn)
	require.Equal(t, "openid api:access", exchanged.Scope)

	refreshed, err := client.RefreshToken(context.Background(), "refresh-token", "", "client-id")
	require.NoError(t, err)
	require.Equal(t, "refresh-access", refreshed.AccessToken)
	require.Equal(t, "refresh-rotated", refreshed.RefreshToken)
	require.Equal(t, int64(7200), refreshed.ExpiresIn)
}

func TestGrokOAuthClientRefreshForbiddenClassifiesEntitlement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"subscription required"}`))
	}))
	defer server.Close()
	t.Setenv(xai.EnvTokenURL, server.URL)

	client := NewGrokOAuthClient()
	_, err := client.RefreshToken(context.Background(), "refresh-token", "", "client-id")
	require.Error(t, err)
	require.Contains(t, strings.ToUpper(err.Error()), "GROK_OAUTH_ENTITLEMENT_DENIED")
}

func TestGrokOAuthClientStatusErrorRedactsSensitiveResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","access_token":"access-secret","refresh_token":"refresh-secret","code_verifier":"verifier-secret"}`))
	}))
	defer server.Close()
	t.Setenv(xai.EnvTokenURL, server.URL)

	client := NewGrokOAuthClient()
	_, err := client.RefreshToken(context.Background(), "refresh-secret", "", "client-id")
	require.Error(t, err)

	errText := err.Error()
	require.Contains(t, errText, "status 400")
	require.Contains(t, errText, `\"refresh_token\":\"***\"`)
	require.NotContains(t, errText, "access-secret")
	require.NotContains(t, errText, "refresh-secret")
	require.NotContains(t, errText, "verifier-secret")
}

func TestExtractGrokSSOTokenRejectsUnsafeCookieSetterURLWithoutRequest(t *testing.T) {
	requestCount := 0
	client := &http.Client{Transport: grokRoundTripFunc(func(*http.Request) (*http.Response, error) {
		requestCount++
		return nil, nil
	})}

	for _, rawURL := range []string{
		"http://accounts.x.ai/cookies",
		"https://evil.example/cookies",
		"https://accounts.x.ai.evil.example/cookies",
		"https://user@accounts.x.ai/cookies",
		"https://accounts.x.ai:8443/cookies",
		"://malformed",
	} {
		t.Run(rawURL, func(t *testing.T) {
			_, err := extractGrokSSOToken(context.Background(), client, rawURL)
			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid cookie setter url")
		})
	}
	require.Zero(t, requestCount)
}

func TestExtractGrokSSOTokenAllowsTrustedAccountsURL(t *testing.T) {
	client := &http.Client{Transport: grokRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "https", req.URL.Scheme)
		require.Equal(t, "accounts.x.ai", req.URL.Host)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Set-Cookie": []string{"sso=trusted-token; Path=/; Secure; HttpOnly"}},
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    req,
		}, nil
	})}

	token, err := extractGrokSSOToken(context.Background(), client, "https://accounts.x.ai/cookie/set")
	require.NoError(t, err)
	require.Equal(t, "trusted-token", token)
}

func TestAutoAuthorizeGrokDeviceCodeChecksVerifyAndApproveStatuses(t *testing.T) {
	tests := []struct {
		name            string
		verifyStatus    int
		verifyLocation  string
		approveStatus   int
		approveLocation string
		wantErrCode     string
		wantApprove     bool
	}{
		{name: "success", verifyStatus: http.StatusOK, approveStatus: http.StatusOK, wantApprove: true},
		{name: "trusted approve redirect", verifyStatus: http.StatusOK, approveStatus: http.StatusSeeOther, approveLocation: "https://accounts.x.ai/oauth2/device/success", wantApprove: true},
		{name: "verify unauthorized", verifyStatus: http.StatusUnauthorized, wantErrCode: "GROK_OAUTH_DEVICE_VERIFY_FAILED"},
		{name: "verify login redirect", verifyStatus: http.StatusSeeOther, verifyLocation: "https://accounts.x.ai/sign-in", wantErrCode: "GROK_OAUTH_DEVICE_VERIFY_FAILED"},
		{name: "verify external redirect", verifyStatus: http.StatusSeeOther, verifyLocation: "https://evil.example/continue", wantErrCode: "GROK_OAUTH_DEVICE_VERIFY_FAILED"},
		{name: "approve throttled", verifyStatus: http.StatusOK, approveStatus: http.StatusTooManyRequests, wantErrCode: "GROK_OAUTH_DEVICE_APPROVE_FAILED", wantApprove: true},
		{name: "approve login redirect", verifyStatus: http.StatusOK, approveStatus: http.StatusSeeOther, approveLocation: "https://accounts.x.ai/login", wantErrCode: "GROK_OAUTH_DEVICE_APPROVE_FAILED", wantApprove: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			approveCalled := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/verify":
					if tt.verifyLocation != "" {
						w.Header().Set("Location", tt.verifyLocation)
					}
					w.WriteHeader(tt.verifyStatus)
				case "/approve":
					approveCalled = true
					if tt.approveLocation != "" {
						w.Header().Set("Location", tt.approveLocation)
					}
					w.WriteHeader(tt.approveStatus)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client := server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

			err := autoAuthorizeGrokDeviceCode(context.Background(), client, "sso", "code", server.URL+"/verify", server.URL+"/approve")
			if tt.wantErrCode == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErrCode)
			}
			require.Equal(t, tt.wantApprove, approveCalled)
		})
	}
}
