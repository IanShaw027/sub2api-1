//go:build unit

package service

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	tlsRemintPreviousProfileID int64 = 25
	tlsRemintNextProfileID     int64 = 29
	tlsRemintUnknownProfileID  int64 = 999
)

type tlsRemintAccountRepo struct {
	accountRepoStubForBulkUpdate
	mu      sync.Mutex
	created []*Account
}

func newTLSRemintAccountRepo(accounts ...*Account) *tlsRemintAccountRepo {
	repo := &tlsRemintAccountRepo{
		accountRepoStubForBulkUpdate: accountRepoStubForBulkUpdate{
			getByIDAccounts:  map[int64]*Account{},
			getByIDsAccounts: make([]*Account, 0, len(accounts)),
		},
	}
	for _, account := range accounts {
		if account == nil {
			continue
		}
		cp := cloneTLSRemintAccount(account)
		repo.getByIDAccounts[account.ID] = cp
		repo.getByIDsAccounts = append(repo.getByIDsAccounts, cp)
	}
	return repo
}

func (r *tlsRemintAccountRepo) store(account *Account) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := cloneTLSRemintAccount(account)
	if r.getByIDAccounts == nil {
		r.getByIDAccounts = map[int64]*Account{}
	}
	r.getByIDAccounts[account.ID] = cp
	found := false
	for i, existing := range r.getByIDsAccounts {
		if existing != nil && existing.ID == account.ID {
			r.getByIDsAccounts[i] = cp
			found = true
			break
		}
	}
	if !found {
		r.getByIDsAccounts = append(r.getByIDsAccounts, cp)
	}
}

func (r *tlsRemintAccountRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.accountRepoStubForBulkUpdate.GetByID(ctx, id)
}

func (r *tlsRemintAccountRepo) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.accountRepoStubForBulkUpdate.GetByIDs(ctx, ids)
}

func (r *tlsRemintAccountRepo) Create(_ context.Context, account *Account) error {
	if account.ID == 0 {
		account.ID = int64(len(r.getByIDAccounts) + 1)
	}
	r.store(account)
	r.mu.Lock()
	r.created = append(r.created, cloneTLSRemintAccount(account))
	r.mu.Unlock()
	return nil
}

func (r *tlsRemintAccountRepo) Update(_ context.Context, account *Account) error {
	r.store(account)
	return nil
}

func (r *tlsRemintAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	account, err := r.GetByID(context.Background(), id)
	if err != nil {
		return err
	}
	if account.Extra == nil {
		account.Extra = map[string]any{}
	}
	for key, value := range updates {
		account.Extra[key] = value
	}
	r.store(account)
	return nil
}

func (r *tlsRemintAccountRepo) BulkUpdate(_ context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	for _, id := range ids {
		account, err := r.GetByID(context.Background(), id)
		if err != nil {
			continue
		}
		if len(updates.Extra) > 0 {
			if account.Extra == nil {
				account.Extra = map[string]any{}
			}
			for key, value := range updates.Extra {
				account.Extra[key] = value
			}
		}
		r.store(account)
	}
	return int64(len(ids)), nil
}

type tlsRemintDeviceRepo struct {
	mu       sync.Mutex
	profiles map[int64]*AccountDeviceProfile
}

func (r *tlsRemintDeviceRepo) GetByAccountID(_ context.Context, accountID int64) (*AccountDeviceProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.profiles[accountID]
	if p == nil {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

func (r *tlsRemintDeviceRepo) InsertBaseline(_ context.Context, p *AccountDeviceProfile) (*AccountDeviceProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.profiles == nil {
		r.profiles = map[int64]*AccountDeviceProfile{}
	}
	cp := *p
	if cp.ID == 0 {
		cp.ID = cp.AccountID
	}
	r.profiles[p.AccountID] = &cp
	out := cp
	return &out, nil
}

func (r *tlsRemintDeviceRepo) UpdateCAS(context.Context, int64, int64, *AccountDeviceProfile) (bool, error) {
	return false, nil
}

func (r *tlsRemintDeviceRepo) DeleteByAccountID(_ context.Context, accountID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.profiles, accountID)
	return nil
}

func cloneTLSRemintAccount(account *Account) *Account {
	if account == nil {
		return nil
	}
	cp := *account
	if account.Extra != nil {
		cp.Extra = cloneTLSRemintExtra(account.Extra)
	}
	return &cp
}

func cloneTLSRemintExtra(extra map[string]any) map[string]any {
	if extra == nil {
		return nil
	}
	out := make(map[string]any, len(extra))
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func setupTLSRemintCatalog(t *testing.T) {
	t.Helper()
	prevGuard := deviceTLSCatalogGuard
	setDeviceTLSCatalogGuard(func(platform string, extra map[string]any) error {
		id, present, err := optionalCapacityInt(extra, DeviceTLSProfileIDExtraKey)
		if err != nil || !present || id <= 0 {
			return err
		}
		switch id {
		case tlsRemintPreviousProfileID, tlsRemintNextProfileID:
			if platform != PlatformAnthropic {
				return identityReject(DeviceTLSProfileIDExtraKey + " does not match account platform")
			}
			return nil
		default:
			return identityReject(DeviceTLSProfileIDExtraKey + " references unknown TLS profile")
		}
	})
	SetTLSProfilePinNameLookup(func(id int64) string {
		switch id {
		case tlsRemintPreviousProfileID:
			return "pin:claude-code:linux:h1"
		case tlsRemintNextProfileID:
			return "pin:claude-code:macos:h1"
		default:
			return ""
		}
	})
	t.Cleanup(func() {
		setDeviceTLSCatalogGuard(prevGuard)
		SetTLSProfilePinNameLookup(nil)
	})
}

func setupTLSRemintDeviceService(t *testing.T, repo *tlsRemintDeviceRepo) {
	t.Helper()
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(repo))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })
}

func newTLSRemintAccount(id int64, extra map[string]any) *Account {
	return &Account{
		ID:       id,
		Name:     "claude-account",
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
		Extra:    extra,
	}
}

func newTLSRemintDeviceRow(accountID, profileID int64, deviceID string) *AccountDeviceProfile {
	return &AccountDeviceProfile{
		AccountID:          accountID,
		Revision:           1,
		SchemaVersion:      1,
		Platform:           PlatformAnthropic,
		ClientFamily:       ClientFamilyClaudeCode,
		OSFamily:           "linux",
		Arch:               "arm64",
		TransportFamily:    TransportH1,
		TLSProfileID:       &profileID,
		InstallationID:     "11111111-1111-4111-8111-111111111111",
		DeviceID:           deviceID,
		GatewayAccountUUID: "55555555-5555-4555-8555-555555555555",
		SessionNamespace:   "0123456789abcdef0123456789abcdef",
	}
}

func TestUpdateAccountExtraRemintsWhenDeviceTLSProfileIDChanges(t *testing.T) {
	setupTLSRemintCatalog(t)
	const accountID int64 = 3
	const oldDeviceID = "22222222-2222-4222-8222-222222222222"
	accountRepo := newTLSRemintAccountRepo(newTLSRemintAccount(accountID, map[string]any{
		"device_learning_enabled":  true,
		DeviceTLSProfileIDExtraKey: tlsRemintPreviousProfileID,
	}))
	deviceRepo := &tlsRemintDeviceRepo{profiles: map[int64]*AccountDeviceProfile{
		accountID: newTLSRemintDeviceRow(accountID, tlsRemintPreviousProfileID, oldDeviceID),
	}}
	setupTLSRemintDeviceService(t, deviceRepo)
	svc := &adminServiceImpl{accountRepo: accountRepo}

	err := svc.UpdateAccountExtra(context.Background(), accountID, map[string]any{
		"device_learning_enabled":  true,
		DeviceTLSProfileIDExtraKey: tlsRemintNextProfileID,
	})
	require.NoError(t, err)

	got, err := deviceRepo.GetByAccountID(context.Background(), accountID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotEqual(t, oldDeviceID, got.DeviceID)
	require.NotNil(t, got.TLSProfileID)
	require.Equal(t, tlsRemintNextProfileID, *got.TLSProfileID)
}

func TestBulkUpdateAccountsRemintsWhenDeviceTLSProfileIDChanges(t *testing.T) {
	setupTLSRemintCatalog(t)
	const accountID int64 = 3
	const oldDeviceID = "22222222-2222-4222-8222-222222222222"
	accountRepo := newTLSRemintAccountRepo(newTLSRemintAccount(accountID, map[string]any{
		"device_learning_enabled":  true,
		DeviceTLSProfileIDExtraKey: tlsRemintPreviousProfileID,
	}))
	deviceRepo := &tlsRemintDeviceRepo{profiles: map[int64]*AccountDeviceProfile{
		accountID: newTLSRemintDeviceRow(accountID, tlsRemintPreviousProfileID, oldDeviceID),
	}}
	setupTLSRemintDeviceService(t, deviceRepo)
	svc := &adminServiceImpl{accountRepo: accountRepo}

	_, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs: []int64{accountID},
		Extra: map[string]any{
			"device_learning_enabled":  true,
			DeviceTLSProfileIDExtraKey: tlsRemintNextProfileID,
		},
	})
	require.NoError(t, err)

	got, err := deviceRepo.GetByAccountID(context.Background(), accountID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotEqual(t, oldDeviceID, got.DeviceID)
	require.NotNil(t, got.TLSProfileID)
	require.Equal(t, tlsRemintNextProfileID, *got.TLSProfileID)
}

func TestUpdateAccountClearingDeviceTLSProfileIDDoesNotRemint(t *testing.T) {
	setupTLSRemintCatalog(t)
	const accountID int64 = 3
	const oldDeviceID = "22222222-2222-4222-8222-222222222222"
	accountRepo := newTLSRemintAccountRepo(newTLSRemintAccount(accountID, map[string]any{
		"device_learning_enabled":  true,
		DeviceTLSProfileIDExtraKey: tlsRemintPreviousProfileID,
	}))
	deviceRepo := &tlsRemintDeviceRepo{profiles: map[int64]*AccountDeviceProfile{
		accountID: newTLSRemintDeviceRow(accountID, tlsRemintPreviousProfileID, oldDeviceID),
	}}
	setupTLSRemintDeviceService(t, deviceRepo)
	svc := &adminServiceImpl{accountRepo: accountRepo}

	_, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Extra: map[string]any{"device_learning_enabled": true},
	})
	require.NoError(t, err)

	got, err := deviceRepo.GetByAccountID(context.Background(), accountID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, oldDeviceID, got.DeviceID)
	require.NotNil(t, got.TLSProfileID)
	require.Equal(t, tlsRemintPreviousProfileID, *got.TLSProfileID)
}

func TestCreateAccountRejectsUnusableDeviceTLSProfileID(t *testing.T) {
	setupTLSRemintCatalog(t)
	accountRepo := newTLSRemintAccountRepo()
	svc := &adminServiceImpl{accountRepo: accountRepo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:     "claude-new",
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"device_learning_enabled":  true,
			DeviceTLSProfileIDExtraKey: tlsRemintUnknownProfileID,
		},
		SkipDefaultGroupBind: true,
	})
	require.Nil(t, account)
	require.Error(t, err)
	require.ErrorContains(t, err, DeviceTLSProfileIDExtraKey)
	require.Empty(t, accountRepo.created)
}

func TestUpdateAccountRemintsWhenDeviceRowPinMismatchesSameExtra(t *testing.T) {
	setupTLSRemintCatalog(t)
	const accountID int64 = 3
	const oldDeviceID = "22222222-2222-4222-8222-222222222222"
	accountRepo := newTLSRemintAccountRepo(newTLSRemintAccount(accountID, map[string]any{
		"device_learning_enabled":  true,
		DeviceTLSProfileIDExtraKey: tlsRemintNextProfileID,
	}))
	deviceRepo := &tlsRemintDeviceRepo{profiles: map[int64]*AccountDeviceProfile{
		accountID: newTLSRemintDeviceRow(accountID, tlsRemintPreviousProfileID, oldDeviceID),
	}}
	setupTLSRemintDeviceService(t, deviceRepo)
	svc := &adminServiceImpl{accountRepo: accountRepo}

	_, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Extra: map[string]any{
			"device_learning_enabled":  true,
			DeviceTLSProfileIDExtraKey: tlsRemintNextProfileID,
		},
	})
	require.NoError(t, err)

	got, err := deviceRepo.GetByAccountID(context.Background(), accountID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotEqual(t, oldDeviceID, got.DeviceID)
	require.NotNil(t, got.TLSProfileID)
	require.Equal(t, tlsRemintNextProfileID, *got.TLSProfileID)
}

func TestUpdateAccountRejectsNewDeviceTLSProfileIDWhenLearningDisabled(t *testing.T) {
	setupTLSRemintCatalog(t)
	const accountID int64 = 3
	const oldDeviceID = "22222222-2222-4222-8222-222222222222"
	accountRepo := newTLSRemintAccountRepo(newTLSRemintAccount(accountID, map[string]any{
		"device_learning_enabled":  false,
		DeviceTLSProfileIDExtraKey: tlsRemintPreviousProfileID,
	}))
	deviceRepo := &tlsRemintDeviceRepo{profiles: map[int64]*AccountDeviceProfile{
		accountID: newTLSRemintDeviceRow(accountID, tlsRemintPreviousProfileID, oldDeviceID),
	}}
	setupTLSRemintDeviceService(t, deviceRepo)
	svc := &adminServiceImpl{accountRepo: accountRepo}

	_, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Extra: map[string]any{
			"device_learning_enabled":  false,
			DeviceTLSProfileIDExtraKey: tlsRemintNextProfileID,
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, DeviceTLSProfileIDExtraKey)

	got, err := deviceRepo.GetByAccountID(context.Background(), accountID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, oldDeviceID, got.DeviceID)
}

func TestUpdateAccountKeepsPinWhenLearningDisabledWithoutNewSelection(t *testing.T) {
	setupTLSRemintCatalog(t)
	const accountID int64 = 3
	const oldDeviceID = "22222222-2222-4222-8222-222222222222"
	accountRepo := newTLSRemintAccountRepo(newTLSRemintAccount(accountID, map[string]any{
		"device_learning_enabled":  true,
		DeviceTLSProfileIDExtraKey: tlsRemintPreviousProfileID,
	}))
	deviceRepo := &tlsRemintDeviceRepo{profiles: map[int64]*AccountDeviceProfile{
		accountID: newTLSRemintDeviceRow(accountID, tlsRemintPreviousProfileID, oldDeviceID),
	}}
	setupTLSRemintDeviceService(t, deviceRepo)
	svc := &adminServiceImpl{accountRepo: accountRepo}

	_, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Extra: map[string]any{
			"device_learning_enabled":  false,
			DeviceTLSProfileIDExtraKey: tlsRemintPreviousProfileID,
		},
	})
	require.NoError(t, err)

	got, err := deviceRepo.GetByAccountID(context.Background(), accountID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, oldDeviceID, got.DeviceID)
	require.NotNil(t, got.TLSProfileID)
	require.Equal(t, tlsRemintPreviousProfileID, *got.TLSProfileID)
}

func TestUpdateAccountRejectsShadowDeviceTLSProfileID(t *testing.T) {
	setupTLSRemintCatalog(t)
	const accountID int64 = 4
	parentID := int64(3)
	const oldDeviceID = "22222222-2222-4222-8222-222222222222"
	accountRepo := newTLSRemintAccountRepo(&Account{
		ID:              accountID,
		Name:            "shadow",
		Platform:        PlatformAnthropic,
		Type:            AccountTypeOAuth,
		Status:          StatusActive,
		ParentAccountID: &parentID,
		Extra:           map[string]any{"device_learning_enabled": true},
	})
	deviceRepo := &tlsRemintDeviceRepo{profiles: map[int64]*AccountDeviceProfile{
		parentID: newTLSRemintDeviceRow(parentID, tlsRemintPreviousProfileID, oldDeviceID),
	}}
	setupTLSRemintDeviceService(t, deviceRepo)
	svc := &adminServiceImpl{accountRepo: accountRepo}

	_, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Extra: map[string]any{
			"device_learning_enabled":  true,
			DeviceTLSProfileIDExtraKey: tlsRemintNextProfileID,
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, DeviceTLSProfileIDExtraKey)

	got, err := deviceRepo.GetByAccountID(context.Background(), parentID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, oldDeviceID, got.DeviceID)
}
