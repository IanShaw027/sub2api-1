//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

func TestKiroTokenRefresherRefreshDefaultsMissingExpiresIn(t *testing.T) {
	tests := []struct {
		name        string
		wantPath    string
		credentials map[string]any
	}{
		{
			name:     "social",
			wantPath: "/refreshToken",
			credentials: map[string]any{
				"refresh_token": "kiro-refresh-token",
			},
		},
		{
			name:     "idc",
			wantPath: "/token",
			credentials: map[string]any{
				"auth_method":    "idc",
				"client_id":      "client-1",
				"client_secret":  "secret-1",
				"refresh_token":  "kiro-refresh-token",
				"login_provider": "awsidc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &kiroHTTPUpstreamRecorder{
				doFunc: func(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
					if req.Method != http.MethodPost || req.URL.Path != tt.wantPath {
						t.Fatalf("unexpected refresh request: %s %s", req.Method, req.URL.String())
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"accessToken":"new-access-token","refreshToken":"new-refresh-token"}`)),
						Header:     make(http.Header),
					}, nil
				},
			}
			refresher := NewKiroTokenRefresher().WithTransport(upstream, &TLSFingerprintProfileService{})
			before := time.Now().UTC()

			credentials, err := refresher.Refresh(context.Background(), &Account{
				Platform:    PlatformKiro,
				Type:        AccountTypeOAuth,
				Credentials: tt.credentials,
				Concurrency: 1,
			})
			after := time.Now().UTC()

			if err != nil {
				t.Fatalf("Refresh returned error: %v", err)
			}
			expiresAtText := strings.TrimSpace(stringCredential(credentials, "expires_at"))
			expiresAt, err := time.Parse(time.RFC3339, expiresAtText)
			if err != nil {
				t.Fatalf("expires_at %q should parse as RFC3339: %v", expiresAtText, err)
			}
			if expiresAt.Before(before.Add(3599*time.Second)) || expiresAt.After(after.Add(3601*time.Second)) {
				t.Fatalf("expires_at = %s, want about now+3600s (before=%s after=%s)", expiresAt.Format(time.RFC3339), before.Format(time.RFC3339), after.Format(time.RFC3339))
			}
		})
	}
}
