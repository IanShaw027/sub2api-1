package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountTestKiroRuntimeSettingRepoStub struct {
	values map[string]string
}

func (s *accountTestKiroRuntimeSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *accountTestKiroRuntimeSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *accountTestKiroRuntimeSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *accountTestKiroRuntimeSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = s.values[key]
	}
	return result, nil
}

func (s *accountTestKiroRuntimeSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *accountTestKiroRuntimeSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *accountTestKiroRuntimeSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestAccountTestService_TestKiroAccountConnection_UsesKiroTLSProfile(t *testing.T) {
	setGinTestMode()

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
	require.Contains(t, rec.Body.String(), "hello from kiro")
}

func TestAccountTestService_TestKiroAccountConnection_UsesAPIKeyForManualAccounts(t *testing.T) {
	setGinTestMode()

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
	svc := &AccountTestService{
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          92,
		Platform:    PlatformKiro,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key": "kiro-manual-token",
		},
	}

	err := svc.testKiroAccountConnection(c, account, "claude-sonnet-4-5-20250929")
	require.NoError(t, err)
	require.Equal(t, "Bearer kiro-manual-token", upstream.req.Header.Get("Authorization"))
	require.Contains(t, rec.Body.String(), "hello from kiro")
}

func TestAccountTestService_TestKiroAccountConnection_UsesSharedKiroModelMapping(t *testing.T) {
	setGinTestMode()

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
	svc := &AccountTestService{
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       94,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-manual-token",
			"model_mapping": map[string]any{
				"claude-sonnet-*": "claude-sonnet-4-5-20250929",
			},
		},
	}

	err := svc.testKiroAccountConnection(c, account, "claude-sonnet-4-7")
	require.NoError(t, err)

	body, err := io.ReadAll(upstream.req.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `"modelId":"claude-sonnet-4.5"`)
	require.Contains(t, rec.Body.String(), "hello from kiro")
}

func TestAccountTestService_TestKiroAccountConnection_UsesRuntimeSettings(t *testing.T) {
	setGinTestMode()
	kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
	kiroRuntimeSettingsSF.Forget("kiro_runtime")

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
	settings := NewSettingService(&accountTestKiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyKiroDefaultVersion:       "0.12.0",
			SettingKeyKiroDefaultCommit:        "commit-runtime",
			SettingKeyKiroDefaultSystemVersion: "linux#6.9.0",
			SettingKeyKiroDefaultNodeVersion:   "23.1.0",
		},
	}, &config.Config{})
	svc := &AccountTestService{
		httpUpstream:   upstream,
		settingService: settings,
	}
	account := &Account{
		ID:       93,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":    "kiro-manual-token",
			"machine_id": "machine-id",
		},
	}

	err := svc.testKiroAccountConnection(c, account, "claude-sonnet-4-5-20250929")
	require.NoError(t, err)
	require.Contains(t, upstream.req.Header.Get("x-amz-user-agent"), "KiroIDE-0.12.0-")
	require.Contains(t, upstream.req.Header.Get("User-Agent"), "os/linux#6.9.0")
	require.Contains(t, upstream.req.Header.Get("User-Agent"), "md/nodejs#23.1.0")
	require.Equal(t, "commit-runtime", upstream.req.Header.Get("x-amzn-kiro-commit"))
}

func TestAccountTestService_TestKiroAccountConnection_UsesAccountModelMapping(t *testing.T) {
	setGinTestMode()

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
	svc := &AccountTestService{
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       94,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-manual-token",
			"model_mapping": map[string]any{
				"claude-sonnet-4-7": "claude-sonnet-4-5-20250929",
			},
		},
	}

	err := svc.testKiroAccountConnection(c, account, "claude-sonnet-4-7")
	require.NoError(t, err)

	var payload struct {
		ConversationState struct {
			CurrentMessage struct {
				UserInputMessage struct {
					ModelID string `json:"modelId"`
				} `json:"userInputMessage"`
			} `json:"currentMessage"`
		} `json:"conversationState"`
	}
	reqBody, err := io.ReadAll(upstream.req.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(reqBody, &payload))
	require.Equal(t, kiropkg.MapModel("claude-sonnet-4-5-20250929"), payload.ConversationState.CurrentMessage.UserInputMessage.ModelID)
	require.Contains(t, rec.Body.String(), "hello from kiro")
}

func TestAccountTestService_TestKiroAccountConnection_IncludesUpstreamErrorDetail(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/kiro/test", nil)

	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader([]byte(`{
				"error":"invalid_request",
				"message":"selected model is not available for this account"
			}`))),
		},
	}
	svc := &AccountTestService{
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       95,
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "kiro-manual-token",
		},
	}

	err := svc.testKiroAccountConnection(c, account, "claude-sonnet-4-5-20250929")
	require.Error(t, err)
	require.Contains(t, err.Error(), "Kiro API returned 400")
	require.Contains(t, err.Error(), "selected model is not available for this account")
	require.Contains(t, rec.Body.String(), "selected model is not available for this account")
}
