package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type accountCredentialsRepoStub struct {
	account *Account
	err     error
}

func (s *accountCredentialsRepoStub) Create(ctx context.Context, account *Account) error { return nil }

func (s *accountCredentialsRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.account, nil
}

func (s *accountCredentialsRepoStub) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ExistsByID(ctx context.Context, id int64) (bool, error) {
	return false, nil
}

func (s *accountCredentialsRepoStub) GetByCRSAccountID(ctx context.Context, crsAccountID string) (*Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) FindByExtraField(ctx context.Context, key string, value any) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListCRSAccountIDs(ctx context.Context) (map[string]int64, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) Update(ctx context.Context, account *Account) error { return nil }
func (s *accountCredentialsRepoStub) Delete(ctx context.Context, id int64) error         { return nil }

func (s *accountCredentialsRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *accountCredentialsRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *accountCredentialsRepoStub) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListActive(ctx context.Context) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) UpdateLastUsed(ctx context.Context, id int64) error { return nil }

func (s *accountCredentialsRepoStub) BatchUpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	return nil
}

func (s *accountCredentialsRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	return nil
}

func (s *accountCredentialsRepoStub) ClearError(ctx context.Context, id int64) error { return nil }

func (s *accountCredentialsRepoStub) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	return nil
}

func (s *accountCredentialsRepoStub) AutoPauseExpiredAccounts(ctx context.Context, now time.Time) (int64, error) {
	return 0, nil
}

func (s *accountCredentialsRepoStub) BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error {
	return nil
}

func (s *accountCredentialsRepoStub) ListSchedulable(ctx context.Context) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	return nil
}

func (s *accountCredentialsRepoStub) SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time, reason ...string) error {
	return nil
}

func (s *accountCredentialsRepoStub) SetOverloaded(ctx context.Context, id int64, until time.Time) error {
	return nil
}

func (s *accountCredentialsRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	return nil
}

func (s *accountCredentialsRepoStub) ClearTempUnschedulable(ctx context.Context, id int64) error {
	return nil
}

func (s *accountCredentialsRepoStub) ClearRateLimit(ctx context.Context, id int64) error { return nil }

func (s *accountCredentialsRepoStub) ClearAntigravityQuotaScopes(ctx context.Context, id int64) error {
	return nil
}

func (s *accountCredentialsRepoStub) ClearModelRateLimits(ctx context.Context, id int64) error {
	return nil
}

func (s *accountCredentialsRepoStub) UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error {
	return nil
}

func (s *accountCredentialsRepoStub) UpdateSessionWindowEnd(ctx context.Context, id int64, end time.Time) error {
	return nil
}

func (s *accountCredentialsRepoStub) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	return nil
}

func (s *accountCredentialsRepoStub) BulkUpdate(ctx context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	return 0, nil
}

func (s *accountCredentialsRepoStub) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) error {
	return nil
}

func (s *accountCredentialsRepoStub) ResetQuotaUsed(ctx context.Context, id int64) error { return nil }

func (s *accountCredentialsRepoStub) RevertProxyFallback(ctx context.Context, accountID int64) error {
	return nil
}

func (s *accountCredentialsRepoStub) ListOAuthRefreshCandidates(ctx context.Context) ([]Account, error) {
	return nil, nil
}

func (s *accountCredentialsRepoStub) ListShadowsByParent(ctx context.Context, parentID int64) ([]*Account, error) {
	return nil, nil
}

func TestAccountService_TestCredentials_ValidatesPlatformCredentials(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		wantErr string
	}{
		{
			name: "anthropic apikey missing api_key",
			account: &Account{
				ID:          1,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Credentials: map[string]any{},
			},
			wantErr: "anthropic api_key is required",
		},
		{
			name: "anthropic apikey rejects invalid base_url",
			account: &Account{
				ID:       1,
				Platform: PlatformAnthropic,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key":  "sk-ant-test",
					"base_url": "://bad-url",
				},
			},
			wantErr: "anthropic base_url is invalid",
		},
		{
			name: "anthropic apikey success",
			account: &Account{
				ID:       1,
				Platform: PlatformAnthropic,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key":  "sk-ant-test",
					"base_url": "https://api.anthropic.com",
				},
			},
		},
		{
			name: "anthropic oauth missing token",
			account: &Account{
				ID:          1,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeOAuth,
				Credentials: map[string]any{},
			},
			wantErr: "anthropic access_token or refresh_token is required",
		},
		{
			name: "anthropic setup token success",
			account: &Account{
				ID:       1,
				Platform: PlatformAnthropic,
				Type:     AccountTypeSetupToken,
				Credentials: map[string]any{
					"access_token": "anthropic-token",
				},
			},
		},
		{
			name: "openai apikey missing api_key",
			account: &Account{
				ID:          2,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Credentials: map[string]any{},
			},
			wantErr: "openai api_key is required",
		},
		{
			name: "openai apikey rejects invalid base_url",
			account: &Account{
				ID:       2,
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key":  "sk-test",
					"base_url": "ftp://api.openai.example",
				},
			},
			wantErr: "openai base_url is invalid",
		},
		{
			name: "openai apikey success",
			account: &Account{
				ID:       2,
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key":  "sk-test",
					"base_url": "https://api.openai.com/v1",
				},
			},
		},
		{
			name: "openai oauth missing token",
			account: &Account{
				ID:          2,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeOAuth,
				Credentials: map[string]any{},
			},
			wantErr: "openai access_token or refresh_token is required",
		},
		{
			name: "openai oauth success with refresh token",
			account: &Account{
				ID:       2,
				Platform: PlatformOpenAI,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"refresh_token": "openai-refresh-token",
				},
			},
		},
		{
			name: "gemini apikey missing api_key",
			account: &Account{
				ID:          3,
				Platform:    PlatformGemini,
				Type:        AccountTypeAPIKey,
				Credentials: map[string]any{},
			},
			wantErr: "gemini api_key is required",
		},
		{
			name: "gemini apikey rejects invalid base_url",
			account: &Account{
				ID:       3,
				Platform: PlatformGemini,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key":  "gemini-test",
					"base_url": "localhost:8080",
				},
			},
			wantErr: "gemini base_url is invalid",
		},
		{
			name: "gemini apikey success",
			account: &Account{
				ID:       3,
				Platform: PlatformGemini,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key":  "gemini-test",
					"base_url": "https://generativelanguage.googleapis.com",
				},
			},
		},
		{
			name: "gemini oauth missing token",
			account: &Account{
				ID:          3,
				Platform:    PlatformGemini,
				Type:        AccountTypeOAuth,
				Credentials: map[string]any{},
			},
			wantErr: "gemini access_token or refresh_token is required",
		},
		{
			name: "gemini oauth success with access token",
			account: &Account{
				ID:       3,
				Platform: PlatformGemini,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"access_token": "gemini-access-token",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &AccountService{accountRepo: &accountCredentialsRepoStub{account: tt.account}}

			err := svc.TestCredentials(context.Background(), tt.account.ID)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestAccountService_TestCredentials_PropagatesRepositoryError(t *testing.T) {
	repoErr := errors.New("database unavailable")
	svc := &AccountService{accountRepo: &accountCredentialsRepoStub{err: repoErr}}

	err := svc.TestCredentials(context.Background(), 99)

	require.ErrorIs(t, err, repoErr)
}

func TestAccountCredentialValidators_RejectNilAccount(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*Account) error
	}{
		{
			name: "apikey credentials",
			fn:   validateAPIKeyCredentials,
		},
		{
			name: "oauth credentials",
			fn:   validateOAuthCredentials,
		},
		{
			name: "account test credentials",
			fn:   validateAccountTestCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn(nil)
			require.EqualError(t, err, "account is required")
		})
	}
}

func (s *accountCredentialsRepoStub) ListAllWithFilters(ctx context.Context, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, error) {
	return nil, nil
}
