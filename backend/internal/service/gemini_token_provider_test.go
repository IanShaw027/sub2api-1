//go:build unit

package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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

func (s *geminiTokenProviderCacheRecorder) AcquireRefreshLock(_ context.Context, cacheKey string, ttl time.Duration) (string, bool, error) {
	_ = cacheKey
	_ = ttl
	return "lease", true, nil
}

func (s *geminiTokenProviderCacheRecorder) ReleaseRefreshLock(_ context.Context, cacheKey string, _ string) error {
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

// 并发取 token 的 project 探测必须被 singleflight 合并为一次上游 onboard，避免 N 个并发各跑一轮 ~10s。
func TestGeminiTokenProvider_GetAccessToken_CollapsesConcurrentProjectDetection(t *testing.T) {
	var loadCalls int32
	release := make(chan struct{})
	oauthService := &GeminiOAuthService{
		codeAssist: &mockGeminiCodeAssistClient{
			loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
				atomic.AddInt32(&loadCalls, 1)
				<-release // 阻塞，制造并发窗口让其余 goroutine 都进入并阻塞在 singleflight.Do
				return &geminicli.LoadCodeAssistResponse{
					CloudAICompanionProject: "detected-project",
					CurrentTier:             &geminicli.TierInfo{ID: "STANDARD"},
				}, nil
			},
		},
	}
	repo := &refreshAPIAccountRepo{account: &Account{ID: 201}}
	provider := NewGeminiTokenProvider(repo, nil, oauthService)

	const n = 20
	var wg sync.WaitGroup
	tokens := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// 每个 goroutine 用独立 account（同 ID），避免共享 Credentials map 的并发写。
			acct := &Account{
				ID:       201,
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"access_token":           "access-token",
					"oauth_type":             "code_assist",
					"auto_detect_project_id": "true",
				},
			}
			tokens[i], errs[i] = provider.GetAccessToken(context.Background(), acct)
		}(i)
	}
	// 给所有 goroutine 时间进入 singleflight.Do（winner 阻塞在 mock，其余在 Do 上等待）后再放行。
	time.Sleep(200 * time.Millisecond)
	close(release)
	wg.Wait()

	for i := 0; i < n; i++ {
		require.NoErrorf(t, errs[i], "goroutine %d", i)
		require.Equal(t, "access-token", tokens[i])
	}
	require.Equal(t, int32(1), atomic.LoadInt32(&loadCalls), "并发 project 探测必须合并为一次上游 onboard")
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

func TestGeminiTokenProvider_GetAccessToken_RejectsNilAccount(t *testing.T) {
	t.Parallel()

	provider := NewGeminiTokenProvider(nil, nil, nil)

	token, err := provider.GetAccessToken(context.Background(), nil)

	require.Empty(t, token)
	require.EqualError(t, err, "account is required")
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
