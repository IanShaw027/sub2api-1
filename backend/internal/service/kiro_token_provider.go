package service

import (
	"context"
	"errors"
	"fmt"
	"time"
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
		refreshPolicy: ClaudeProviderRefreshPolicy(),
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
		return nil, errors.New("account is nil")
	}
	if account.Platform != PlatformKiro || account.Type != AccountTypeOAuth {
		return nil, errors.New("not a kiro oauth account")
	}

	if p.refreshAPI != nil && p.executor != nil {
		result, err := p.refreshAPI.RefreshIfNeeded(ctx, account, p.executor, kiroAdminRefreshSkew)
		if err != nil {
			return nil, err
		}
		if result != nil {
			if result.Account != nil {
				if result.LockHeld {
					return p.awaitRefreshResult(ctx, account, result.Account)
				}
				return result.Account, nil
			}
			if result.LockHeld {
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
		return nil, err
	}

	cloned := *account
	cloned.Credentials = cloneCredentials(newCredentials)
	if err := persistAccountCredentials(ctx, p.accountRepo, &cloned, newCredentials); err != nil {
		return nil, err
	}
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
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformKiro || account.Type != AccountTypeOAuth {
		return "", errors.New("not a kiro oauth account")
	}

	cacheKey := KiroTokenCacheKey(account)
	if p.tokenCache != nil {
		if token, err := p.tokenCache.GetAccessToken(ctx, cacheKey); err == nil && token != "" {
			return token, nil
		}
	}

	expiresAt := account.GetCredentialAsTime("expires_at")
	needsRefresh := expiresAt == nil || time.Until(*expiresAt) <= kiroTokenRefreshSkew
	refreshFailed := false

	if needsRefresh && p.refreshAPI != nil && p.executor != nil {
		result, err := p.refreshAPI.RefreshIfNeeded(ctx, account, p.executor, kiroTokenRefreshSkew)
		if err != nil {
			if p.refreshPolicy.OnRefreshError == ProviderRefreshErrorReturn {
				return "", err
			}
			refreshFailed = true
		} else if result.LockHeld {
			if p.refreshPolicy.OnLockHeld == ProviderLockHeldWaitForCache && p.tokenCache != nil {
				time.Sleep(200 * time.Millisecond)
				if token, cacheErr := p.tokenCache.GetAccessToken(ctx, cacheKey); cacheErr == nil && token != "" {
					return token, nil
				}
			}
		} else {
			account = result.Account
			expiresAt = account.GetCredentialAsTime("expires_at")
		}
	}

	accessToken := account.GetCredential("access_token")
	if accessToken == "" {
		return "", errors.New("access_token not found in credentials")
	}

	if p.tokenCache != nil {
		latestAccount, isStale := CheckTokenVersion(ctx, account, p.accountRepo)
		if isStale && latestAccount != nil {
			accessToken = latestAccount.GetCredential("access_token")
			if accessToken == "" {
				return "", errors.New("access_token not found after version check")
			}
		} else {
			ttl := 30 * time.Minute
			if refreshFailed {
				if p.refreshPolicy.FailureTTL > 0 {
					ttl = p.refreshPolicy.FailureTTL
				} else {
					ttl = time.Minute
				}
			} else if expiresAt != nil {
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

func KiroMachineID(account *Account) string {
	if account == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s", account.GetCredential("machine_id"), account.GetCredential("refresh_token"))
}
