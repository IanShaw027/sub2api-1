package service

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestService_TestKiroAccountConnection_UsesKiroTLSProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/kiro/test", nil)

	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":message-type": "event",
				":event-type":   "assistantResponseEvent",
			}, map[string]any{"content": "hello from kiro"}))),
		},
	}
	profileSvc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			12: {ID: 12, Name: "Kiro Custom Profile"},
		},
	}
	svc := &AccountTestService{
		httpUpstream:        upstream,
		tlsFPProfileService: profileSvc,
	}
	account := &Account{
		ID:          91,
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Concurrency: 2,
		Credentials: map[string]any{
			"access_token":  "kiro-access-token",
			"refresh_token": "kiro-refresh-token",
			"expires_at":    time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": int64(12),
		},
	}

	err := svc.testKiroAccountConnection(c, account, "claude-sonnet-4-5-20250929")
	require.NoError(t, err)
	require.NotNil(t, upstream.profile)
	require.Equal(t, "Kiro Custom Profile", upstream.profile.Name)
	require.Equal(t, "vibe", upstream.req.Header.Get("x-amzn-kiro-agent-mode"))
	require.Contains(t, upstream.req.URL.String(), "generateAssistantResponse")
	require.Empty(t, upstream.req.Header.Values("Connection"))
	require.Contains(t, rec.Body.String(), "Kiro connection OK")
}
