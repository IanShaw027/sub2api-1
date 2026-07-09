package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
)

const (
	kiroTokenRefreshSkew = 3 * time.Minute
	kiroTokenCacheSkew   = 5 * time.Minute
	kiroAdminRefreshSkew = 100 * 365 * 24 * time.Hour
	kiroAdminRefreshWait = 800 * time.Millisecond
	kiroAdminRefreshPoll = 100 * time.Millisecond
)

type KiroTokenProvider struct {
	accountRepo   AccountRepository
	tokenCache    GeminiTokenCache
	refreshAPI    *OAuthRefreshAPI
	executor      OAuthRefreshExecutor
	refreshPolicy ProviderRefreshPolicy
}

func NewKiroTokenProvider(
	accountRepo AccountRepository,
	tokenCache GeminiTokenCache,
) *KiroTokenProvider {
	return &KiroTokenProvider{
		accountRepo:   accountRepo,
		tokenCache:    tokenCache,
		refreshPolicy: KiroProviderRefreshPolicy(),
	}
}

func (p *KiroTokenProvider) SetRefreshAPI(api *OAuthRefreshAPI, executor OAuthRefreshExecutor) {
	p.refreshAPI = api
	p.executor = executor
}

func (p *KiroTokenProvider) SetRefreshPolicy(policy ProviderRefreshPolicy) {
	p.refreshPolicy = policy
}

func (p *KiroTokenProvider) RefreshAccount(ctx context.Context, account *Account) (*Account, error) {
	if account == nil {
		return nil, errors.New("account is required")
	}
	if account.Platform != PlatformKiro || account.Type != AccountTypeOAuth {
		return nil, errors.New("not a kiro oauth account")
	}

	if p.refreshAPI != nil && p.executor != nil {
		result, err := p.refreshAPI.RefreshIfNeeded(ctx, account, p.executor, kiroAdminRefreshSkew)
		if err != nil {
			kiroLogger(ctx, account).Warn("kiro.account_refresh_failed", zap.String("trigger", "admin"), zap.Error(err))
			return nil, err
		}
		if result != nil {
			if result.Account != nil {
				if result.LockHeld {
					kiroLogger(ctx, account).Info("kiro.account_refresh_wait", zap.String("trigger", "admin"))
					return p.awaitRefreshResult(ctx, account, result.Account)
				}
				kiroLogger(ctx, result.Account).Info("kiro.account_refresh_complete", zap.String("trigger", "admin"))
				return result.Account, nil
			}
			if result.LockHeld {
				kiroLogger(ctx, account).Info("kiro.account_refresh_wait", zap.String("trigger", "admin"))
				return p.awaitRefreshResult(ctx, account, nil)
			}
		}
		cloned := *account
		cloned.Credentials = cloneCredentials(account.Credentials)
		return &cloned, nil
	}

	if p.executor == nil {
		return nil, errors.New("kiro token refresh executor is not configured")
	}

	newCredentials, err := p.executor.Refresh(ctx, account)
	if err != nil {
		kiroLogger(ctx, account).Warn("kiro.account_refresh_failed", zap.String("trigger", "admin"), zap.Error(err))
		return nil, err
	}

	cloned := *account
	cloned.Credentials = cloneCredentials(newCredentials)
	if err := persistAccountCredentials(ctx, p.accountRepo, &cloned, newCredentials); err != nil {
		kiroLogger(ctx, &cloned).Warn("kiro.account_refresh_failed", zap.String("trigger", "admin"), zap.Error(err))
		return nil, err
	}
	kiroLogger(ctx, &cloned).Info("kiro.account_refresh_complete", zap.String("trigger", "admin"))
	return &cloned, nil
}

func (p *KiroTokenProvider) awaitRefreshResult(ctx context.Context, before *Account, candidate *Account) (*Account, error) {
	if kiroRefreshStateChanged(before, candidate) {
		return candidate, nil
	}
	if p.accountRepo == nil {
		return nil, errors.New("kiro token refresh already in progress")
	}

	deadline := time.NewTimer(kiroAdminRefreshWait)
	defer deadline.Stop()
	ticker := time.NewTicker(kiroAdminRefreshPoll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, errors.New("kiro token refresh already in progress")
		case <-ticker.C:
			latestAccount, err := p.accountRepo.GetByID(ctx, before.ID)
			if err != nil || latestAccount == nil {
				continue
			}
			if kiroRefreshStateChanged(before, latestAccount) {
				return latestAccount, nil
			}
		}
	}
}

func kiroRefreshStateChanged(before, after *Account) bool {
	if before == nil || after == nil {
		return false
	}
	for _, key := range []string{"_token_version", "access_token", "refresh_token", "expires_at", "profile_arn"} {
		if before.GetCredential(key) != after.GetCredential(key) {
			return true
		}
	}
	return false
}

func (p *KiroTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is required")
	}
	if account.Platform != PlatformKiro || account.Type != AccountTypeOAuth {
		return "", errors.New("not a kiro oauth account")
	}

	callerAccount := account
	cacheKey := KiroTokenCacheKey(account)
	expiresAt := account.GetCredentialAsTime("expires_at")
	if p.tokenCache != nil {
		cacheUsable := expiresAt != nil && time.Until(*expiresAt) > kiroTokenRefreshSkew
		if token, err := p.tokenCache.GetAccessToken(ctx, cacheKey); err == nil && token != "" && cacheUsable {
			kiroLogger(ctx, account).Info("kiro.access_token_cache_hit")
			return token, nil
		}
		kiroLogger(ctx, account).Info("kiro.access_token_cache_miss")
	}

	needsRefresh := expiresAt == nil || time.Until(*expiresAt) <= kiroTokenRefreshSkew
	refreshFailed := false

	if needsRefresh && p.refreshAPI != nil && p.executor != nil {
		result, err := p.refreshAPI.RefreshIfNeeded(ctx, account, p.executor, kiroTokenRefreshSkew)
		if err != nil {
			if p.refreshPolicy.OnRefreshError == ProviderRefreshErrorReturn {
				kiroLogger(ctx, account).Warn("kiro.access_token_refresh_failed", zap.Int("policy", int(p.refreshPolicy.OnRefreshError)), zap.Error(err))
				return "", err
			}
			refreshFailed = true
			kiroLogger(ctx, account).Warn("kiro.access_token_refresh_failed", zap.Int("policy", int(p.refreshPolicy.OnRefreshError)), zap.Error(err))
		} else if result.LockHeld {
			kiroLogger(ctx, account).Info("kiro.access_token_refresh_wait", zap.Bool("cache_wait", p.refreshPolicy.OnLockHeld == ProviderLockHeldWaitForCache))
			if p.refreshPolicy.OnLockHeld == ProviderLockHeldWaitForCache && p.tokenCache != nil {
				time.Sleep(200 * time.Millisecond)
				if token, cacheErr := p.tokenCache.GetAccessToken(ctx, cacheKey); cacheErr == nil && token != "" {
					return token, nil
				}
			}
			if p.refreshPolicy.OnLockHeld == ProviderLockHeldWaitForCache {
				refreshedAccount, waitErr := p.awaitRefreshResult(ctx, account, nil)
				if waitErr != nil {
					return "", waitErr
				}
				copyKiroAccountRuntimeState(callerAccount, refreshedAccount)
				account = callerAccount
				expiresAt = account.GetCredentialAsTime("expires_at")
				kiroLogger(ctx, account).Info("kiro.access_token_refresh_complete", zap.Bool("waited_for_refresh", true), zap.Duration("expires_in", kiroExpiresIn(expiresAt)))
			}
		} else if result.Account != nil {
			copyKiroAccountRuntimeState(callerAccount, result.Account)
			account = callerAccount
			expiresAt = account.GetCredentialAsTime("expires_at")
			kiroLogger(ctx, account).Info("kiro.access_token_refresh_complete", zap.Bool("waited_for_refresh", false), zap.Duration("expires_in", kiroExpiresIn(expiresAt)))
		}
	}

	accessToken := account.GetCredential("access_token")
	if accessToken == "" {
		return "", errors.New("access_token not found in credentials")
	}

	if p.tokenCache != nil {
		latestAccount, isStale := CheckTokenVersion(ctx, account, p.accountRepo)
		if isStale && latestAccount != nil {
			copyKiroAccountRuntimeState(callerAccount, latestAccount)
			account = callerAccount
			accessToken = latestAccount.GetCredential("access_token")
			if accessToken == "" {
				return "", errors.New("access_token not found after version check")
			}
			kiroLogger(ctx, account).Info("kiro.access_token_version_reloaded")
		} else if !refreshFailed {
			ttl := 30 * time.Minute
			if expiresAt != nil {
				until := time.Until(*expiresAt)
				switch {
				case until > kiroTokenCacheSkew:
					ttl = until - kiroTokenCacheSkew
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

func kiroExpiresIn(expiresAt *time.Time) time.Duration {
	if expiresAt == nil {
		return 0
	}
	return time.Until(*expiresAt).Round(time.Second)
}

func copyKiroAccountRuntimeState(dst, src *Account) {
	if dst == nil || src == nil {
		return
	}
	dst.Credentials = cloneCredentials(src.Credentials)
	dst.Extra = cloneCredentials(src.Extra)
	dst.ProxyID = src.ProxyID
	dst.Proxy = src.Proxy
}

func KiroRegion(account *Account) string {
	if account == nil {
		return "us-east-1"
	}
	if value := account.GetCredential("api_region"); value != "" {
		return value
	}
	if value := account.GetCredential("auth_region"); value != "" {
		return value
	}
	if value := account.GetCredential("region"); value != "" {
		return value
	}
	return "us-east-1"
}

func KiroAuthRegion(account *Account) string {
	if account == nil {
		return "us-east-1"
	}
	if value := account.GetCredential("auth_region"); value != "" {
		return value
	}
	if value := account.GetCredential("region"); value != "" {
		return value
	}
	return "us-east-1"
}
