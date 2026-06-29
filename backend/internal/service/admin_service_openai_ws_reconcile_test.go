package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type adminOpenAIWSReconcileAccountRepo struct {
	AccountRepository
	mu       sync.Mutex
	accounts map[int64]*Account
}

func newAdminOpenAIWSReconcileAccountRepo(accounts ...*Account) *adminOpenAIWSReconcileAccountRepo {
	repo := &adminOpenAIWSReconcileAccountRepo{accounts: make(map[int64]*Account, len(accounts))}
	for _, account := range accounts {
		if account == nil {
			continue
		}
		repo.accounts[account.ID] = cloneAdminOpenAIWSReconcileAccount(account)
	}
	return repo
}

func cloneAdminOpenAIWSReconcileAccount(account *Account) *Account {
	if account == nil {
		return nil
	}
	cloned := *account
	if account.Credentials != nil {
		cloned.Credentials = MergeCredentials(nil, account.Credentials)
	}
	if account.Extra != nil {
		cloned.Extra = MergeCredentials(nil, account.Extra)
	}
	return &cloned
}

func (r *adminOpenAIWSReconcileAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	account := r.accounts[id]
	if account == nil {
		return nil, ErrAccountNotFound
	}
	return cloneAdminOpenAIWSReconcileAccount(account), nil
}

func (r *adminOpenAIWSReconcileAccountRepo) Create(_ context.Context, account *Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if account.ID == 0 {
		account.ID = int64(len(r.accounts) + 9000)
	}
	r.accounts[account.ID] = cloneAdminOpenAIWSReconcileAccount(account)
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		if account := r.accounts[id]; account != nil {
			out = append(out, cloneAdminOpenAIWSReconcileAccount(account))
		}
	}
	return out, nil
}

func (r *adminOpenAIWSReconcileAccountRepo) Update(_ context.Context, account *Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accounts[account.ID] = cloneAdminOpenAIWSReconcileAccount(account)
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) BulkUpdate(_ context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var affected int64
	for _, id := range ids {
		account := r.accounts[id]
		if account == nil {
			continue
		}
		if updates.Status != nil {
			account.Status = *updates.Status
		}
		if updates.Schedulable != nil {
			account.Schedulable = *updates.Schedulable
		}
		if updates.Concurrency != nil {
			account.Concurrency = *updates.Concurrency
		}
		if len(updates.Extra) > 0 {
			if account.Extra == nil {
				account.Extra = map[string]any{}
			}
			for key, value := range updates.Extra {
				account.Extra[key] = value
			}
		}
		if len(updates.Credentials) > 0 {
			if account.Credentials == nil {
				account.Credentials = map[string]any{}
			}
			for key, value := range updates.Credentials {
				account.Credentials[key] = value
			}
		}
		affected++
	}
	return affected, nil
}

func (r *adminOpenAIWSReconcileAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	account := r.accounts[id]
	if account == nil {
		return ErrAccountNotFound
	}
	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	for key, value := range updates {
		account.Extra[key] = value
	}
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) BindGroups(_ context.Context, _ int64, _ []int64) error {
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.accounts, id)
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) SetError(_ context.Context, id int64, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	account := r.accounts[id]
	if account == nil {
		return ErrAccountNotFound
	}
	account.ErrorMessage = errorMsg
	if errorMsg != "" {
		account.Status = StatusError
	}
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) ClearError(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	account := r.accounts[id]
	if account == nil {
		return ErrAccountNotFound
	}
	account.ErrorMessage = ""
	account.Status = StatusActive
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) ClearRateLimit(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.accounts[id] == nil {
		return ErrAccountNotFound
	}
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) ClearAntigravityQuotaScopes(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.accounts[id] == nil {
		return ErrAccountNotFound
	}
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) ClearModelRateLimits(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.accounts[id] == nil {
		return ErrAccountNotFound
	}
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) ClearTempUnschedulable(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.accounts[id] == nil {
		return ErrAccountNotFound
	}
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) ClearAccountSchedulingThresholdSnapshots(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.accounts[id] == nil {
		return ErrAccountNotFound
	}
	return nil
}

func (r *adminOpenAIWSReconcileAccountRepo) SetSchedulable(_ context.Context, id int64, schedulable bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	account := r.accounts[id]
	if account == nil {
		return ErrAccountNotFound
	}
	account.Schedulable = schedulable
	return nil
}

func installAdminOpenAIWSReconcileHook(t *testing.T) <-chan struct{} {
	t.Helper()
	RegisterOpenAIWSPoolReconcileHook(nil)
	ch := make(chan struct{}, 10)
	RegisterOpenAIWSPoolReconcileHook(func() { ch <- struct{}{} })
	t.Cleanup(func() { RegisterOpenAIWSPoolReconcileHook(nil) })
	return ch
}

func requireAdminOpenAIWSReconcileTriggered(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("expected OpenAI WS pool reconcile trigger")
	}
}

func requireAdminOpenAIWSReconcileNotTriggered(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("did not expect OpenAI WS pool reconcile trigger")
	case <-time.After(100 * time.Millisecond):
	}
}

func newAdminServiceForOpenAIWSReconcile(repo AccountRepository) *adminServiceImpl {
	return NewAdminService(
		nil,
		nil,
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}

func TestAdminService_CreateOpenAIOAuthAccountTriggersWSPoolReconcile(t *testing.T) {
	ch := installAdminOpenAIWSReconcileHook(t)
	repo := newAdminOpenAIWSReconcileAccountRepo()
	svc := newAdminServiceForOpenAIWSReconcile(repo)

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "openai-oauth",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeOAuth,
		Concurrency:          8,
		SkipDefaultGroupBind: true,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeCtxPool,
		},
	})
	require.NoError(t, err)
	require.True(t, account.IsOpenAIOAuth())
	requireAdminOpenAIWSReconcileTriggered(t, ch)
}

func TestAdminService_UpdateOpenAIOAuthAccountTriggersWSPoolReconcile(t *testing.T) {
	ch := installAdminOpenAIWSReconcileHook(t)
	repo := newAdminOpenAIWSReconcileAccountRepo(&Account{
		ID:          7001,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 8,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeCtxPool,
		},
	})
	svc := newAdminServiceForOpenAIWSReconcile(repo)

	concurrency := 4
	updated, err := svc.UpdateAccount(context.Background(), 7001, &UpdateAccountInput{Concurrency: &concurrency})
	require.NoError(t, err)
	require.Equal(t, 4, updated.Concurrency)
	requireAdminOpenAIWSReconcileTriggered(t, ch)
}

func TestAdminService_UpdateNonOpenAIOAuthAccountDoesNotTriggerWSPoolReconcile(t *testing.T) {
	ch := installAdminOpenAIWSReconcileHook(t)
	repo := newAdminOpenAIWSReconcileAccountRepo(&Account{
		ID:          7011,
		Name:        "anthropic-oauth",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 8,
	})
	svc := newAdminServiceForOpenAIWSReconcile(repo)

	concurrency := 4
	updated, err := svc.UpdateAccount(context.Background(), 7011, &UpdateAccountInput{Concurrency: &concurrency})
	require.NoError(t, err)
	require.Equal(t, 4, updated.Concurrency)
	requireAdminOpenAIWSReconcileNotTriggered(t, ch)
}

func TestAdminService_UpdateOpenAIOAuthExtraTriggersWSPoolReconcile(t *testing.T) {
	ch := installAdminOpenAIWSReconcileHook(t)
	repo := newAdminOpenAIWSReconcileAccountRepo(&Account{
		ID:          7002,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 8,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeCtxPool,
		},
	})
	svc := newAdminServiceForOpenAIWSReconcile(repo)

	err := svc.UpdateAccountExtra(context.Background(), 7002, map[string]any{
		"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff,
	})
	require.NoError(t, err)
	requireAdminOpenAIWSReconcileTriggered(t, ch)
}

func TestAdminService_BulkUpdateOpenAIOAuthAccountTriggersWSPoolReconcile(t *testing.T) {
	ch := installAdminOpenAIWSReconcileHook(t)
	repo := newAdminOpenAIWSReconcileAccountRepo(&Account{
		ID:          7005,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 8,
	})
	svc := newAdminServiceForOpenAIWSReconcile(repo)

	concurrency := 4
	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs:  []int64{7005},
		Concurrency: &concurrency,
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.Success)
	requireAdminOpenAIWSReconcileTriggered(t, ch)
}

func TestAdminService_DeleteOpenAIOAuthAccountTriggersWSPoolReconcile(t *testing.T) {
	ch := installAdminOpenAIWSReconcileHook(t)
	repo := newAdminOpenAIWSReconcileAccountRepo(&Account{
		ID:          7003,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 8,
	})
	svc := newAdminServiceForOpenAIWSReconcile(repo)

	require.NoError(t, svc.DeleteAccount(context.Background(), 7003))
	requireAdminOpenAIWSReconcileTriggered(t, ch)
}

func TestAdminService_SetOpenAIOAuthErrorTriggersWSPoolReconcile(t *testing.T) {
	ch := installAdminOpenAIWSReconcileHook(t)
	repo := newAdminOpenAIWSReconcileAccountRepo(&Account{
		ID:          7006,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 8,
	})
	svc := newAdminServiceForOpenAIWSReconcile(repo)

	require.NoError(t, svc.SetAccountError(context.Background(), 7006, "test error"))
	requireAdminOpenAIWSReconcileTriggered(t, ch)
}

func TestAdminService_ClearOpenAIOAuthErrorTriggersWSPoolReconcile(t *testing.T) {
	ch := installAdminOpenAIWSReconcileHook(t)
	repo := newAdminOpenAIWSReconcileAccountRepo(&Account{
		ID:           7007,
		Name:         "openai-oauth",
		Platform:     PlatformOpenAI,
		Type:         AccountTypeOAuth,
		Status:       StatusError,
		Schedulable:  true,
		Concurrency:  8,
		ErrorMessage: "test error",
	})
	svc := newAdminServiceForOpenAIWSReconcile(repo)

	updated, err := svc.ClearAccountError(context.Background(), 7007)
	require.NoError(t, err)
	require.Equal(t, StatusActive, updated.Status)
	requireAdminOpenAIWSReconcileTriggered(t, ch)
}

func TestAdminService_SetOpenAIOAuthSchedulableTriggersWSPoolReconcile(t *testing.T) {
	ch := installAdminOpenAIWSReconcileHook(t)
	repo := newAdminOpenAIWSReconcileAccountRepo(&Account{
		ID:          7004,
		Name:        "openai-oauth",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 8,
	})
	svc := newAdminServiceForOpenAIWSReconcile(repo)

	updated, err := svc.SetAccountSchedulable(context.Background(), 7004, false)
	require.NoError(t, err)
	require.False(t, updated.Schedulable)
	requireAdminOpenAIWSReconcileTriggered(t, ch)
}
