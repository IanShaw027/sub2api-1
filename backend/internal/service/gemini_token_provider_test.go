//go:build unit

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/stretchr/testify/require"
)

type geminiTokenProviderCacheRecorder struct {
	getKeys     []string
	setKeys     []string
	cachedToken string
}

func (s *geminiTokenProviderCacheRecorder) GetAccessToken(_ context.Context, cacheKey string) (string, error) {
	s.getKeys = append(s.getKeys, cacheKey)
	if strings.TrimSpace(s.cachedToken) != "" {
		return s.cachedToken, nil
	}
	return "", fmt.Errorf("cache miss")
}

func (s *geminiTokenProviderCacheRecorder) SetAccessToken(_ context.Context, cacheKey string, token string, ttl time.Duration) error {
	_ = token
	_ = ttl
	s.setKeys = append(s.setKeys, cacheKey)
	return nil
}

func (s *geminiTokenProviderCacheRecorder) DeleteAccessToken(_ context.Context, cacheKey string) error {
	_ = cacheKey
	return nil
}

func (s *geminiTokenProviderCacheRecorder) AcquireRefreshLock(_ context.Context, cacheKey string, ttl time.Duration) (bool, error) {
	_ = cacheKey
	_ = ttl
	return true, nil
}

func (s *geminiTokenProviderCacheRecorder) ReleaseRefreshLock(_ context.Context, cacheKey string) error {
	_ = cacheKey
	return nil
}

func TestGeminiTokenProvider_GetAccessToken_BackfillsProjectIDWhenAutoDetectFlagSet(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       101,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":           "access-token",
			"oauth_type":             "code_assist",
			"auto_detect_project_id": "true",
		},
	}

	repo := &refreshAPIAccountRepo{account: account}
	oauthService := &GeminiOAuthService{
		codeAssist: &mockGeminiCodeAssistClient{
			loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
				return &geminicli.LoadCodeAssistResponse{
					CloudAICompanionProject: "detected-project",
					CurrentTier:             &geminicli.TierInfo{ID: "STANDARD"},
				}, nil
			},
		},
	}

	provider := NewGeminiTokenProvider(repo, nil, oauthService)

	token, err := provider.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "access-token", token)
	require.Equal(t, "detected-project", account.GetCredential("project_id"))
	require.Equal(t, "STANDARD", account.GetCredential("tier_id"))
	require.Equal(t, 1, repo.updateCredentialsCalls)
}

func TestGeminiTokenProvider_GetAccessToken_GoogleOneDoesNotBackfillProjectIDWhenAutoDetectFlagSet(t *testing.T) {
	t.Parallel()

	loadCodeAssistCalled := false
	account := &Account{
		ID:       102,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":           "access-token",
			"oauth_type":             "google_one",
			"auto_detect_project_id": "true",
		},
	}

	repo := &refreshAPIAccountRepo{account: account}
	oauthService := &GeminiOAuthService{
		codeAssist: &mockGeminiCodeAssistClient{
			loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
				loadCodeAssistCalled = true
				return &geminicli.LoadCodeAssistResponse{
					CloudAICompanionProject: "managed-google-one-project",
					CurrentTier:             &geminicli.TierInfo{ID: "g1-pro-tier"},
				}, nil
			},
		},
	}

	provider := NewGeminiTokenProvider(repo, nil, oauthService)

	token, err := provider.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "access-token", token)
	require.Empty(t, account.GetCredential("project_id"))
	require.Empty(t, account.GetCredential("tier_id"))
	require.False(t, loadCodeAssistCalled)
	require.Equal(t, 0, repo.updateCredentialsCalls)
}

func TestGeminiTokenProvider_GetAccessToken_GoogleOneTokenWithoutProjectIDDoesNotError(t *testing.T) {
	t.Parallel()

	cache := &geminiTokenProviderCacheRecorder{cachedToken: "cached-access-token"}
	provider := NewGeminiTokenProvider(nil, cache, nil)
	account := &Account{
		ID:       103,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "stored-access-token",
			"oauth_type":   "google_one",
			"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
		},
	}

	token, err := provider.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "cached-access-token", token)
	require.Equal(t, []string{"gemini:account:103"}, cache.getKeys)
	require.Empty(t, cache.setKeys)
}

func TestGeminiTokenProvider_GetAccessToken_UsesAccountScopedCacheKeyEvenWhenProjectMatches(t *testing.T) {
	t.Parallel()

	cache := &geminiTokenProviderCacheRecorder{}
	provider := NewGeminiTokenProvider(nil, cache, nil)

	accountA := &Account{
		ID:       201,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-a",
			"project_id":   "shared-project",
			"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
		},
	}
	accountB := &Account{
		ID:       202,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-b",
			"project_id":   "shared-project",
			"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
		},
	}

	tokenA, err := provider.GetAccessToken(context.Background(), accountA)
	require.NoError(t, err)
	require.Equal(t, "token-a", tokenA)

	tokenB, err := provider.GetAccessToken(context.Background(), accountB)
	require.NoError(t, err)
	require.Equal(t, "token-b", tokenB)

	require.Equal(t, []string{"gemini:account:201", "gemini:account:202"}, cache.getKeys)
	require.Equal(t, []string{"gemini:account:201", "gemini:account:202"}, cache.setKeys)
}

func TestGeminiTokenProvider_GetAccessToken_RejectsCachedCodeAssistTokenWithoutProjectID(t *testing.T) {
	t.Parallel()

	cache := &geminiTokenProviderCacheRecorder{cachedToken: "cached-access-token"}
	provider := NewGeminiTokenProvider(nil, cache, nil)
	account := &Account{
		ID:       203,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "stored-access-token",
			"oauth_type":   "code_assist",
			"expires_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
		},
	}

	token, err := provider.GetAccessToken(context.Background(), account)
	require.ErrorContains(t, err, errGeminiCodeAssistProjectIDNotConfigured)
	require.Empty(t, token)
	require.Equal(t, []string{"gemini:account:203"}, cache.getKeys)
	require.Empty(t, cache.setKeys)
}

func TestGeminiTokenProvider_GetAccessToken_RefreshRecomputesProjectRoutingState(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       204,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":           "stale-access",
			"refresh_token":          "stale-refresh",
			"oauth_type":             "code_assist",
			"auto_detect_project_id": "false",
			"expires_at":             time.Now().Add(-time.Minute).Format(time.RFC3339),
		},
	}
	repo := &refreshAPIAccountRepo{account: account}
	cache := &geminiTokenProviderCacheRecorder{}
	executor := &refreshAPIExecutorStub{
		needsRefresh: true,
		credentials: map[string]any{
			"access_token":  "fresh-access",
			"refresh_token": "fresh-refresh",
			"project_id":    "refreshed-project",
			"oauth_type":    "code_assist",
			"expires_at":    time.Now().Add(time.Hour).Format(time.RFC3339),
		},
	}

	provider := NewGeminiTokenProvider(repo, cache, nil)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)

	token, err := provider.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "fresh-access", token)
	require.Equal(t, "refreshed-project", account.GetCredential("project_id"))
}

func TestGeminiTokenProvider_GetAccessToken_RefreshDoesNotReturnStaleCachedToken(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       205,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  "stale-access",
			"refresh_token": "stale-refresh",
			"project_id":    "shared-project",
			"expires_at":    time.Now().Add(-time.Minute).Format(time.RFC3339),
		},
	}
	repo := &refreshAPIAccountRepo{account: account}
	cache := &geminiTokenProviderCacheRecorder{cachedToken: "cached-stale-token"}
	executor := &refreshAPIExecutorStub{
		needsRefresh: true,
		credentials: map[string]any{
			"access_token":  "fresh-access",
			"refresh_token": "fresh-refresh",
			"project_id":    "shared-project",
			"expires_at":    time.Now().Add(time.Hour).Format(time.RFC3339),
		},
	}

	provider := NewGeminiTokenProvider(repo, cache, nil)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)

	token, err := provider.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "fresh-access", token)
}
