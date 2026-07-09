package service

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	geminiTokenRefreshSkew = 3 * time.Minute
	geminiTokenCacheSkew   = 5 * time.Minute
	// geminiProjectDetectTimeout 为共享 project 探测的整体超时上限：fetchProjectID 内部含 ~10s
	// onboard 轮询，此上限防止上游卡死时 detached 探测无限挂起。
	geminiProjectDetectTimeout = 30 * time.Second
)

// GeminiTokenProvider manages access_token for Gemini OAuth and Vertex service account accounts.
type GeminiTokenProvider struct {
	accountRepo        AccountRepository
	tokenCache         GeminiTokenCache
	geminiOAuthService *GeminiOAuthService
	refreshAPI         *OAuthRefreshAPI
	executor           OAuthRefreshExecutor
	refreshPolicy      ProviderRefreshPolicy
	// projectDetectSF 合并同一账号的并发 project 探测：fetchProjectID 含 ~10s onboard，
	// 无合并时 N 个并发取 token 请求各跑一轮，放大上游压力并钉住连接。
	projectDetectSF singleflight.Group
}

const errGeminiCodeAssistProjectIDNotConfigured = "gemini project_id not configured for project-scoped oauth account"

func NewGeminiTokenProvider(
	accountRepo AccountRepository,
	tokenCache GeminiTokenCache,
	geminiOAuthService *GeminiOAuthService,
) *GeminiTokenProvider {
	return &GeminiTokenProvider{
		accountRepo:        accountRepo,
		tokenCache:         tokenCache,
		geminiOAuthService: geminiOAuthService,
		refreshPolicy:      GeminiProviderRefreshPolicy(),
	}
}

// SetRefreshAPI injects unified OAuth refresh API and executor.
func (p *GeminiTokenProvider) SetRefreshAPI(api *OAuthRefreshAPI, executor OAuthRefreshExecutor) {
	p.refreshAPI = api
	p.executor = executor
}

// SetRefreshPolicy injects caller-side refresh policy.
func (p *GeminiTokenProvider) SetRefreshPolicy(policy ProviderRefreshPolicy) {
	p.refreshPolicy = policy
}

func (p *GeminiTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is required")
	}
	if account.Platform != PlatformGemini || (account.Type != AccountTypeOAuth && account.Type != AccountTypeServiceAccount) {
		return "", errors.New("not a gemini oauth or service account")
	}
	if account.Type == AccountTypeServiceAccount {
		return p.getServiceAccountAccessToken(ctx, account)
	}

	cacheKey := GeminiTokenCacheKey(account)
	projectID := strings.TrimSpace(account.GetCredential("project_id"))
	autoDetectProjectID := account.GetCredential("auto_detect_project_id") == "true"
	requiresProjectRouting := account.UsesGeminiCLIProjectRouting()
	cachedToken := ""
	refreshed := false

	// 1) Try cache first.
	if p.tokenCache != nil {
		if token, err := p.tokenCache.GetAccessToken(ctx, cacheKey); err == nil && strings.TrimSpace(token) != "" {
			cachedToken = strings.TrimSpace(token)
		}
	}

	// 2) Refresh if needed (pre-expiry skew).
	expiresAt := account.GetCredentialAsTime("expires_at")
	needsRefresh := expiresAt == nil || time.Until(*expiresAt) <= geminiTokenRefreshSkew

	if needsRefresh && p.refreshAPI != nil && p.executor != nil {
		result, err := p.refreshAPI.RefreshIfNeeded(ctx, account, p.executor, geminiTokenRefreshSkew)
		if err != nil {
			if p.refreshPolicy.OnRefreshError == ProviderRefreshErrorReturn {
				return "", err
			}
		} else if result.LockHeld {
			if p.refreshPolicy.OnLockHeld == ProviderLockHeldWaitForCache && p.tokenCache != nil {
				if token, cacheErr := p.tokenCache.GetAccessToken(ctx, cacheKey); cacheErr == nil && strings.TrimSpace(token) != "" {
					return token, nil
				}
			}
			slog.Debug("gemini_token_lock_held_use_old", "account_id", account.ID)
		} else {
			account = result.Account
			expiresAt = account.GetCredentialAsTime("expires_at")
			projectID = strings.TrimSpace(account.GetCredential("project_id"))
			autoDetectProjectID = account.GetCredential("auto_detect_project_id") == "true"
			requiresProjectRouting = account.UsesGeminiCLIProjectRouting()
			refreshed = true
		}
	} else if needsRefresh && p.tokenCache != nil {
		// Backward-compatible test path when refreshAPI is not injected.
		locked, lockErr := p.tokenCache.AcquireRefreshLock(ctx, cacheKey, 30*time.Second)
		if lockErr == nil && locked {
			defer func() { _ = p.tokenCache.ReleaseRefreshLock(ctx, cacheKey) }()
		} else if lockErr != nil {
			slog.Warn("gemini_token_lock_failed", "account_id", account.ID, "error", lockErr)
		}
	}

	accessToken := strings.TrimSpace(account.GetCredential("access_token"))
	if accessToken == "" {
		accessToken = cachedToken
	}
	if strings.TrimSpace(accessToken) == "" {
		return "", errors.New("access_token not found in credentials")
	}

	// Gemini CLI / Google One and Code Assist both rely on Code Assist metadata
	// to discover or validate the effective project.
	if requiresProjectRouting && projectID == "" && autoDetectProjectID {
		if p.geminiOAuthService == nil {
			return "", errors.New(errGeminiCodeAssistProjectIDNotConfigured)
		}
		detected, err := p.detectGeminiProjectID(ctx, account, accessToken)
		if err != nil {
			log.Printf("[GeminiTokenProvider] Auto-detect project_id failed: %v", err)
			return "", err
		}
		if detected != "" {
			projectID = detected
		}
	}

	if requiresProjectRouting && strings.TrimSpace(projectID) == "" {
		return "", errors.New(errGeminiCodeAssistProjectIDNotConfigured)
	}

	if cachedToken != "" && !refreshed {
		return cachedToken, nil
	}

	// 3) Populate cache with TTL.
	if p.tokenCache != nil {
		latestAccount, isStale := CheckTokenVersion(ctx, account, p.accountRepo)
		if isStale && latestAccount != nil {
			slog.Debug("gemini_token_version_stale_use_latest", "account_id", account.ID)
			accessToken = latestAccount.GetCredential("access_token")
			if strings.TrimSpace(accessToken) == "" {
				return "", errors.New("access_token not found after version check")
			}
		} else {
			ttl := 30 * time.Minute
			if expiresAt != nil {
				until := time.Until(*expiresAt)
				switch {
				case until > geminiTokenCacheSkew:
					ttl = until - geminiTokenCacheSkew
				case until > 0:
					ttl = until
				default:
					ttl = time.Minute
				}
			}
			_ = p.tokenCache.SetAccessToken(ctx, cacheKey, accessToken, ttl)
		}
	}

	return accessToken, nil
}

// detectGeminiProjectID 用 singleflight 合并同一账号的并发 project 探测，返回探测到的 project_id。
// 探测（fetchProjectID 内含 ~10s onboard）同一账号同一时刻只跑一次，其余并发共享结果，消除
// N 个并发请求各跑一轮的放大。成功后 project_id 会持久化到账号，后续请求走缓存/凭证分支不再探测。
// 共享探测用脱离调用方取消的 context（加超时上限）执行，避免“胜出者”请求取消时把结果连累给所有等待者。
func (p *GeminiTokenProvider) detectGeminiProjectID(ctx context.Context, account *Account, accessToken string) (string, error) {
	var proxyURL string
	if account.ProxyID != nil && p.geminiOAuthService.proxyRepo != nil {
		if proxy, err := p.geminiOAuthService.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	key := strconv.FormatInt(account.ID, 10)
	res, err, _ := p.projectDetectSF.Do(key, func() (any, error) {
		detectCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), geminiProjectDetectTimeout)
		defer cancel()

		snapshot, err := p.geminiOAuthService.fetchProjectID(detectCtx, accessToken, proxyURL, "")
		if err != nil {
			return "", err
		}
		detected := strings.TrimSpace(snapshot.ProjectID)
		tierID := strings.TrimSpace(snapshot.TierID)
		if detected != "" {
			if account.Credentials == nil {
				account.Credentials = make(map[string]any)
			}
			account.Credentials["project_id"] = detected
			if tierID != "" {
				account.Credentials["tier_id"] = tierID
			}
			_ = persistAccountCredentials(detectCtx, p.accountRepo, account, account.Credentials)
		}
		return detected, nil
	})
	if err != nil {
		return "", err
	}
	detected, _ := res.(string)
	return detected, nil
}

func (p *GeminiTokenProvider) getServiceAccountAccessToken(ctx context.Context, account *Account) (string, error) {
	return getVertexServiceAccountAccessToken(ctx, p.tokenCache, account)
}

func GeminiTokenCacheKey(account *Account) string {
	if account != nil && account.Type == AccountTypeServiceAccount {
		if key, err := parseVertexServiceAccountKey(account); err == nil {
			return vertexServiceAccountCacheKey(account, key)
		}
	}
	return "gemini:account:" + strconv.FormatInt(account.ID, 10)
}
