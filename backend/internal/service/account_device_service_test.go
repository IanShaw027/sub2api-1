//go:build unit

package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/accountdeviceprofile"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newAccountDeviceService(t *testing.T) (*service.AccountDeviceService, *dbent.Client) {
	t.Helper()
	svc, client, _ := newCountingAccountDeviceService(t)
	return svc, client
}

func mustCreateDeviceAccount(t *testing.T, client *dbent.Client, platform string, extra map[string]any) *service.Account {
	t.Helper()
	if extra == nil {
		extra = map[string]any{}
	}
	row, err := client.Account.Create().
		SetName("device-" + platform).
		SetPlatform(platform).
		SetType(service.AccountTypeAPIKey).
		SetStatus(service.StatusActive).
		SetCredentials(map[string]any{"api_key": "sk-test"}).
		SetExtra(extra).
		Save(context.Background())
	require.NoError(t, err)
	return &service.Account{
		ID:       row.ID,
		Name:     row.Name,
		Platform: row.Platform,
		Type:     row.Type,
		Extra:    row.Extra,
	}
}

func TestGetOrCreateInsertsValidBaselineAndIsIdempotent(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"account_uuid": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
	})

	first, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NoError(t, service.ValidateAccountDeviceProfile(first))
	require.Equal(t, account.ID, first.AccountID)
	require.Equal(t, int64(1), first.Revision)
	require.Equal(t, 1, first.SchemaVersion)
	require.Equal(t, service.PlatformAnthropic, first.Platform)
	require.Equal(t, service.DefaultClientFamily(service.PlatformAnthropic), first.ClientFamily)
	require.Equal(t, service.LearnedFromBaseline, first.LearnedFrom)
	require.False(t, first.LearningEnabled)
	require.Equal(t, service.TransportH1, first.TransportFamily)
	require.Nil(t, first.TLSProfileID)
	require.NotEqual(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", first.GatewayAccountUUID)
	require.NotEmpty(t, first.GatewayAccountUUID)
	require.NotEmpty(t, first.DeviceID)
	require.NotEmpty(t, first.InstallationID)
	require.NotEmpty(t, first.MachineID)
	require.Regexp(t, `^[0-9a-f]{32}$`, first.SessionNamespace)

	second, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.DeviceID, second.DeviceID)
	require.Equal(t, first.GatewayAccountUUID, second.GatewayAccountUUID)
	require.Equal(t, first.SessionNamespace, second.SessionNamespace)

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(account.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestGetOrCreateUsesOpenAIAndGrokRuntimes(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()

	openaiAccount := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, nil)
	openaiProfile, err := svc.GetOrCreate(ctx, openaiAccount)
	require.NoError(t, err)
	require.NoError(t, service.ValidateAccountDeviceProfile(openaiProfile))
	require.Equal(t, "codex_cli_rs", openaiProfile.Runtime)
	require.Equal(t, service.ClientFamilyCodexCLI, openaiProfile.ClientFamily)

	grokAccount := mustCreateDeviceAccount(t, client, service.PlatformGrok, nil)
	grokProfile, err := svc.GetOrCreate(ctx, grokAccount)
	require.NoError(t, err)
	require.NoError(t, service.ValidateAccountDeviceProfile(grokProfile))
	require.Equal(t, "grok-shell", grokProfile.Runtime)
	require.Equal(t, service.ClientFamilyGrokCLI, grokProfile.ClientFamily)
}

func TestGetOrCreateAdoptsOpenAIDeviceIDFromExtra(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	pinned := "11111111-1111-4111-8111-111111111111"
	account := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
		"openai_device_id": pinned,
		"account_uuid":     "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
	})

	got, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.NoError(t, service.ValidateAccountDeviceProfile(got))
	require.Equal(t, pinned, got.InstallationID)
	require.NotEqual(t, pinned, got.DeviceID)
	require.NotEqual(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", got.GatewayAccountUUID)
}

func TestGetOrCreateRejectsInvalidOpenAIDeviceIDExtra(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
		"openai_device_id": "dev-xyz",
	})

	got, err := svc.GetOrCreate(ctx, account)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.Nil(t, got)

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(account.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestGetOrCreateAdoptsKiroMachineIDFromCredential(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformKiro, nil)
	account.Credentials = map[string]any{
		"machine_id": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	}

	got, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.NoError(t, service.ValidateAccountDeviceProfile(got))
	require.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", got.MachineID)
}

func TestGetOrCreateRejectsInvalidKiroMachineIDCredential(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformKiro, nil)
	account.Credentials = map[string]any{
		"machine_id": "not-a-machine-id",
	}

	got, err := svc.GetOrCreate(ctx, account)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.Nil(t, got)

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(account.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestGetOrCreateRejectsNonStringOpenAIDeviceIDExtra(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, nil)
	account.Extra = map[string]any{"openai_device_id": true}

	got, err := svc.GetOrCreate(ctx, account)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.Nil(t, got)

	account.Extra = map[string]any{"openai_device_id": nil}
	got, err = svc.GetOrCreate(ctx, account)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.Nil(t, got)

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(account.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestGetOrCreateRejectsNonStringKiroMachineIDCredential(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformKiro, nil)
	account.Credentials = map[string]any{"machine_id": true}

	got, err := svc.GetOrCreate(ctx, account)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.Nil(t, got)

	account.Credentials = map[string]any{"machine_id": nil}
	got, err = svc.GetOrCreate(ctx, account)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.Nil(t, got)

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(account.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestGetOrCreateMintsKiroMachineIDWithoutRefreshToken(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformKiro, nil)
	account.Credentials = map[string]any{
		"refresh_token": "rt-should-not-derive-machine-id",
	}

	got, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.NoError(t, service.ValidateAccountDeviceProfile(got))
	require.Regexp(t, `^[0-9a-f]{64}$`, got.MachineID)
	require.NotEqual(t, kiro.GenerateMachineID("", "rt-should-not-derive-machine-id"), got.MachineID)
}

func TestGetOrCreateShadowAdoptsParentOpenAIDeviceID(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	pinned := "22222222-2222-4222-8222-222222222222"
	parent := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
		"openai_device_id": pinned,
	})
	shadow := mustCreateShadowAccount(t, client, parent)
	shadow.Extra = map[string]any{"openai_device_id": "33333333-3333-4333-8333-333333333333"}

	got, err := svc.GetOrCreate(ctx, shadow)
	require.NoError(t, err)
	require.Equal(t, parent.ID, got.AccountID)
	require.Equal(t, pinned, got.InstallationID)
}

func TestGetOrCreateShadowAdoptsParentKiroMachineID(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	parent := mustCreateDeviceAccount(t, client, service.PlatformKiro, nil)
	_, err := client.Account.UpdateOneID(parent.ID).
		SetCredentials(map[string]any{"machine_id": "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"}).
		Save(ctx)
	require.NoError(t, err)
	shadow := mustCreateShadowAccount(t, client, parent)
	shadow.Credentials = map[string]any{"machine_id": "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"}

	got, err := svc.GetOrCreate(ctx, shadow)
	require.NoError(t, err)
	require.Equal(t, parent.ID, got.AccountID)
	require.Equal(t, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", got.MachineID)
}

func TestGetOrCreateRejectsPlatformMismatchWhenBaselineInsertConflicts(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
	_, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)

	grokAccount := *account
	grokAccount.Platform = service.PlatformGrok
	got, err := svc.GetOrCreate(ctx, &grokAccount)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.ErrorContains(t, err, service.PlatformAnthropic)
	require.ErrorContains(t, err, "account_id=")
	require.ErrorContains(t, err, "insert baseline:")
	require.Nil(t, got)
}

func TestResetDeviceProfileDeletesStaleRowAndRemintsMatchingPlatform(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	cache := &recordingDeviceProfileCache{}
	svc = svc.WithCache(cache)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.Equal(t, service.PlatformAnthropic, created.Platform)

	grokAccount := *account
	grokAccount.Platform = service.PlatformGrok
	_, err = svc.GetOrCreate(ctx, &grokAccount)
	require.Error(t, err)

	reset, err := svc.Reset(ctx, &grokAccount)
	require.NoError(t, err)
	require.NotNil(t, reset)
	require.Equal(t, service.PlatformGrok, reset.Platform)
	require.Equal(t, service.ClientFamilyGrokCLI, reset.ClientFamily)
	require.NotEqual(t, created.InstallationID, reset.InstallationID)

	reloaded, err := svc.GetOrCreate(ctx, &grokAccount)
	require.NoError(t, err)
	require.Equal(t, reset.InstallationID, reloaded.InstallationID)
	require.Equal(t, service.PlatformGrok, cache.profiles[account.ID].Platform)
}

func TestResetDeviceProfileMintsWhenMissing(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, nil)

	reset, err := svc.Reset(ctx, account)
	require.NoError(t, err)
	require.Equal(t, service.PlatformOpenAI, reset.Platform)
	require.Equal(t, service.ClientFamilyCodexCLI, reset.ClientFamily)
}

func TestResetDeviceProfileShadowResetsParent(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	parent := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
	created, err := svc.GetOrCreate(ctx, parent)
	require.NoError(t, err)

	parentID := parent.ID
	shadow := &service.Account{
		ID:              parent.ID + 9000,
		Platform:        service.PlatformAnthropic,
		ParentAccountID: &parentID,
	}
	reset, err := svc.Reset(ctx, shadow)
	require.NoError(t, err)
	require.Equal(t, parent.ID, reset.AccountID)
	require.Equal(t, service.PlatformAnthropic, reset.Platform)
	require.NotEqual(t, created.InstallationID, reset.InstallationID)

	reloaded, err := svc.GetOrCreate(ctx, parent)
	require.NoError(t, err)
	require.Equal(t, reset.InstallationID, reloaded.InstallationID)
}

func TestResetDeviceProfileWaitsForInFlightLearn(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})
	_, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	seedOldClaudeSoftware(t, client, account.ID, false)

	aboutToCAS := make(chan struct{})
	releaseCAS := make(chan struct{})
	repo.beforeCAS = func() {
		select {
		case <-aboutToCAS:
		default:
			close(aboutToCAS)
		}
		<-releaseCAS
	}

	learnDone := make(chan error, 1)
	go func() {
		_, learnErr := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
		learnDone <- learnErr
	}()
	select {
	case <-aboutToCAS:
	case <-time.After(2 * time.Second):
		t.Fatal("learn did not reach CAS")
	}

	grokAccount := *account
	grokAccount.Platform = service.PlatformGrok
	resetDone := make(chan error, 1)
	go func() {
		_, resetErr := svc.Reset(ctx, &grokAccount)
		resetDone <- resetErr
	}()
	select {
	case err := <-resetDone:
		t.Fatalf("reset completed while learn held the account lock: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseCAS)
	require.NoError(t, <-learnDone)
	require.NoError(t, <-resetDone)

	got, err := svc.GetOrCreate(ctx, &grokAccount)
	require.NoError(t, err)
	require.Equal(t, service.PlatformGrok, got.Platform)
	require.Equal(t, service.ClientFamilyGrokCLI, got.ClientFamily)
	ua, _ := got.ProfilePayload["user_agent"].(string)
	require.NotContains(t, ua, "claude-cli/")
}

func TestResetDeviceProfileWaitsForInFlightGetOrCreate(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.Equal(t, service.PlatformAnthropic, created.Platform)

	aboutToInsert := make(chan struct{})
	releaseInsert := make(chan struct{})
	repo.beforeInsert = func() {
		select {
		case <-aboutToInsert:
		default:
			close(aboutToInsert)
		}
		<-releaseInsert
	}

	grokAccount := *account
	grokAccount.Platform = service.PlatformGrok
	resetDone := make(chan error, 1)
	go func() {
		_, resetErr := svc.Reset(ctx, &grokAccount)
		resetDone <- resetErr
	}()
	select {
	case <-aboutToInsert:
	case <-time.After(2 * time.Second):
		t.Fatal("reset did not reach baseline insert")
	}

	getDone := make(chan error, 1)
	go func() {
		_, getErr := svc.GetOrCreate(ctx, account)
		getDone <- getErr
	}()
	select {
	case err := <-getDone:
		t.Fatalf("GetOrCreate completed while reset held the account lock: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseInsert)
	require.NoError(t, <-resetDone)
	require.Error(t, <-getDone)

	got, err := svc.GetOrCreate(ctx, &grokAccount)
	require.NoError(t, err)
	require.Equal(t, service.PlatformGrok, got.Platform)
	require.Equal(t, service.ClientFamilyGrokCLI, got.ClientFamily)
}

func TestResetDeviceProfileRetriesWhenOtherInstanceInsertsStalePlatform(t *testing.T) {
	svcA, client, repo := newCountingAccountDeviceService(t)
	svcB := service.NewAccountDeviceService(repo)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
	_, err := svcA.GetOrCreate(ctx, account)
	require.NoError(t, err)

	deleted := make(chan struct{})
	bInserted := make(chan struct{})
	deletes := 0
	repo.afterDelete = func() {
		deletes++
		if deletes == 1 {
			close(deleted)
			<-bInserted
		}
	}

	grokAccount := *account
	grokAccount.Platform = service.PlatformGrok
	resetDone := make(chan error, 1)
	var reset *service.AccountDeviceProfile
	go func() {
		var resetErr error
		reset, resetErr = svcA.Reset(ctx, &grokAccount)
		resetDone <- resetErr
	}()
	select {
	case <-deleted:
	case <-time.After(2 * time.Second):
		t.Fatal("reset did not delete")
	}
	_, err = svcB.GetOrCreate(ctx, account)
	require.NoError(t, err)
	close(bInserted)
	require.NoError(t, <-resetDone)
	require.NotNil(t, reset)
	require.Equal(t, service.PlatformGrok, reset.Platform)
	require.Equal(t, service.ClientFamilyGrokCLI, reset.ClientFamily)
}

func TestGetOrCreateRaceStillOneRow(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)

	const workers = 8
	profiles := make([]*service.AccountDeviceProfile, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			profiles[i], errs[i] = svc.GetOrCreate(ctx, account)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "worker %d", i)
		require.NotNil(t, profiles[i])
	}
	for i := 1; i < workers; i++ {
		require.Equal(t, profiles[0].ID, profiles[i].ID)
		require.Equal(t, profiles[0].DeviceID, profiles[i].DeviceID)
		require.Equal(t, profiles[0].GatewayAccountUUID, profiles[i].GatewayAccountUUID)
		require.Equal(t, profiles[0].SessionNamespace, profiles[i].SessionNamespace)
	}

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(account.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestGetOrCreateRejectsInvalidBaselineWithoutInsert(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformComposite, nil)

	got, err := svc.GetOrCreate(ctx, account)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.Nil(t, got)

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(account.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

type countingDeviceProfileRepo struct {
	inner        service.AccountDeviceProfileRepository
	mu           sync.Mutex
	gets         map[int64]int
	inserts      map[int64]int
	casCalls     map[int64]int
	casWrites    map[int64]int
	beforeCAS    func()
	beforeInsert func()
	beforeGet    func()
	afterGet     func()
	afterDelete  func()
}

func wrapCountingDeviceProfileRepo(inner service.AccountDeviceProfileRepository) *countingDeviceProfileRepo {
	return &countingDeviceProfileRepo{
		inner:     inner,
		gets:      map[int64]int{},
		inserts:   map[int64]int{},
		casCalls:  map[int64]int{},
		casWrites: map[int64]int{},
	}
}

func (r *countingDeviceProfileRepo) GetByAccountID(ctx context.Context, accountID int64) (*service.AccountDeviceProfile, error) {
	if r.beforeGet != nil {
		r.beforeGet()
	}
	r.mu.Lock()
	r.gets[accountID]++
	r.mu.Unlock()
	p, err := r.inner.GetByAccountID(ctx, accountID)
	if r.afterGet != nil {
		r.afterGet()
	}
	return p, err
}

func (r *countingDeviceProfileRepo) InsertBaseline(ctx context.Context, p *service.AccountDeviceProfile) (*service.AccountDeviceProfile, error) {
	if r.beforeInsert != nil {
		r.beforeInsert()
	}
	r.mu.Lock()
	r.inserts[p.AccountID]++
	r.mu.Unlock()
	return r.inner.InsertBaseline(ctx, p)
}

func (r *countingDeviceProfileRepo) UpdateCAS(ctx context.Context, accountID, expectedRevision int64, next *service.AccountDeviceProfile) (bool, error) {
	r.mu.Lock()
	r.casCalls[accountID]++
	r.mu.Unlock()
	if r.beforeCAS != nil {
		r.beforeCAS()
	}
	ok, err := r.inner.UpdateCAS(ctx, accountID, expectedRevision, next)
	if err == nil && ok {
		r.mu.Lock()
		r.casWrites[accountID]++
		r.mu.Unlock()
	}
	return ok, err
}

func (r *countingDeviceProfileRepo) DeleteByAccountID(ctx context.Context, accountID int64) error {
	err := r.inner.DeleteByAccountID(ctx, accountID)
	if r.afterDelete != nil {
		r.afterDelete()
	}
	return err
}

func (r *countingDeviceProfileRepo) getCount(accountID int64) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gets[accountID]
}

func (r *countingDeviceProfileRepo) insertCount(accountID int64) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inserts[accountID]
}

func (r *countingDeviceProfileRepo) casCallCount(accountID int64) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.casCalls[accountID]
}

func (r *countingDeviceProfileRepo) casWriteCount(accountID int64) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.casWrites[accountID]
}

func newCountingAccountDeviceService(t *testing.T) (*service.AccountDeviceService, *dbent.Client, *countingDeviceProfileRepo) {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(10)

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	counting := wrapCountingDeviceProfileRepo(repository.NewAccountDeviceProfileRepository(client))
	svc := service.NewAccountDeviceService(counting).WithAccountLookup(&entAccountIdentityLookup{client: client})
	return svc, client, counting
}

type entAccountIdentityLookup struct {
	client *dbent.Client
}

func (l *entAccountIdentityLookup) GetByID(ctx context.Context, id int64) (*service.Account, error) {
	row, err := l.client.Account.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &service.Account{
		ID:              row.ID,
		Platform:        row.Platform,
		Type:            row.Type,
		Extra:           row.Extra,
		Credentials:     row.Credentials,
		ParentAccountID: row.ParentAccountID,
	}, nil
}

func mustCreateShadowAccount(t *testing.T, client *dbent.Client, parent *service.Account) *service.Account {
	t.Helper()
	shadowRow, err := client.Account.Create().
		SetName("shadow").
		SetPlatform(parent.Platform).
		SetType(service.AccountTypeAPIKey).
		SetStatus(service.StatusActive).
		SetCredentials(map[string]any{}).
		SetParentAccountID(parent.ID).
		Save(context.Background())
	require.NoError(t, err)
	shadow := &service.Account{
		ID:              shadowRow.ID,
		Platform:        shadowRow.Platform,
		ParentAccountID: shadowRow.ParentAccountID,
	}
	require.True(t, shadow.IsShadow())
	return shadow
}

func TestGetOrCreateShadowDoesNotInsert(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	parent := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
	shadow := mustCreateShadowAccount(t, client, parent)

	got, err := svc.GetOrCreate(ctx, shadow)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, parent.ID, got.AccountID)
	require.Zero(t, repo.insertCount(shadow.ID))

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(shadow.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestGetOrCreateShadowReturnsParentProfile(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	parent := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
	shadow := mustCreateShadowAccount(t, client, parent)

	parentProfile, err := svc.GetOrCreate(ctx, parent)
	require.NoError(t, err)

	shadowProfile, err := svc.GetOrCreate(ctx, shadow)
	require.NoError(t, err)
	require.Equal(t, parentProfile.DeviceID, shadowProfile.DeviceID)
	require.Equal(t, parentProfile.GatewayAccountUUID, shadowProfile.GatewayAccountUUID)
	require.Equal(t, parentProfile.SessionNamespace, shadowProfile.SessionNamespace)
	require.Equal(t, parentProfile.AccountID, shadowProfile.AccountID)
}

func TestGetOrCreateShadowCreatesMissingParentBaseline(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	parent := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
	shadow := mustCreateShadowAccount(t, client, parent)

	got, err := svc.GetOrCreate(ctx, shadow)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, parent.ID, got.AccountID)
	require.NoError(t, service.ValidateAccountDeviceProfile(got))
	require.Equal(t, service.LearnedFromBaseline, got.LearnedFrom)
	require.False(t, got.LearningEnabled)
	require.Equal(t, 1, repo.insertCount(parent.ID))
	require.Zero(t, repo.insertCount(shadow.ID))

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(shadow.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)

	parentRows, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(parent.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, parentRows)
}

func TestGetOrCreateShadowRejectsWhenLookupMissing(t *testing.T) {
	_, client, repo := newCountingAccountDeviceService(t)
	svc := service.NewAccountDeviceService(repo)
	ctx := context.Background()
	parent := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
		"openai_device_id": "22222222-2222-4222-8222-222222222222",
	})
	shadow := mustCreateShadowAccount(t, client, parent)
	shadow.Extra = map[string]any{"openai_device_id": "33333333-3333-4333-8333-333333333333"}

	got, err := svc.GetOrCreate(ctx, shadow)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.Nil(t, got)

	n, err := client.AccountDeviceProfile.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestGetOrCreateShadowRejectsWhenParentIsShadow(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	canonical := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
		"openai_device_id": "22222222-2222-4222-8222-222222222222",
	})
	mid := mustCreateShadowAccount(t, client, canonical)
	leaf := mustCreateShadowAccount(t, client, mid)
	leaf.Extra = map[string]any{"openai_device_id": "33333333-3333-4333-8333-333333333333"}

	got, err := svc.GetOrCreate(ctx, leaf)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.ErrorContains(t, err, "itself a shadow")
	require.Nil(t, got)

	n, err := client.AccountDeviceProfile.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestGetOrCreateShadowRejectsWhenLookupFails(t *testing.T) {
	_, client, repo := newCountingAccountDeviceService(t)
	svc := service.NewAccountDeviceService(repo).WithAccountLookup(&errAccountIdentityLookup{err: fmt.Errorf("db down")})
	ctx := context.Background()
	parent := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
		"openai_device_id": "22222222-2222-4222-8222-222222222222",
	})
	shadow := mustCreateShadowAccount(t, client, parent)
	shadow.Extra = map[string]any{"openai_device_id": "33333333-3333-4333-8333-333333333333"}

	got, err := svc.GetOrCreate(ctx, shadow)
	require.Error(t, err)
	require.ErrorContains(t, err, "identity_reject")
	require.ErrorContains(t, err, "db down")
	require.Nil(t, got)

	n, err := client.AccountDeviceProfile.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

type errAccountIdentityLookup struct{ err error }

func (l *errAccountIdentityLookup) GetByID(context.Context, int64) (*service.Account, error) {
	return nil, l.err
}

func TestGetOrCreateShadowRejectsSelfParentCycle(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
	account.ParentAccountID = &account.ID
	require.True(t, account.IsShadow())

	got, err := svc.GetOrCreate(ctx, account)
	require.Error(t, err)
	require.ErrorContains(t, err, "cycle")
	require.Nil(t, got)

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(account.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestGetOrCreateRejectsNonExactPlatformWithoutInsert(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()

	for _, platform := range []string{"ANTHROPIC", " anthropic "} {
		t.Run(platform, func(t *testing.T) {
			account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)
			account.Platform = platform

			got, err := svc.GetOrCreate(ctx, account)
			require.Error(t, err)
			require.ErrorContains(t, err, "identity_reject")
			require.Nil(t, got)

			n, err := client.AccountDeviceProfile.Query().
				Where(accountdeviceprofile.AccountID(account.ID)).
				Count(ctx)
			require.NoError(t, err)
			require.Zero(t, n)
		})
	}
}

func TestGetOrCreateBaselineOSArchMatchesPayload(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()

	cases := []struct {
		platform string
		osFamily string
		arch     string
		uaHint   string
	}{
		{service.PlatformAnthropic, "linux", "arm64", ""},
		{service.PlatformOpenAI, "linux", "x64", "ubuntu"},
		{service.PlatformGemini, "windows", "x64", "windows"},
		{service.PlatformAntigravity, "windows", "x64", "windows"},
	}

	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			account := mustCreateDeviceAccount(t, client, tc.platform, nil)
			profile, err := svc.GetOrCreate(ctx, account)
			require.NoError(t, err)
			require.NoError(t, service.ValidateAccountDeviceProfile(profile))
			require.Equal(t, tc.osFamily, profile.OSFamily)
			require.Equal(t, tc.arch, profile.Arch)
			require.NotEqual(t, "amd64", profile.Arch)
			require.NotEqual(t, "x86_64", profile.Arch)

			if tc.uaHint != "" {
				ua, _ := profile.ProfilePayload["user_agent"].(string)
				require.Contains(t, strings.ToLower(ua), tc.uaHint)
			}
			if osVal, ok := profile.ProfilePayload["stainless_os"].(string); ok {
				require.Equal(t, strings.ToLower(osVal), profile.OSFamily)
			}
			if archVal, ok := profile.ProfilePayload["stainless_arch"].(string); ok {
				require.Equal(t, strings.ToLower(archVal), profile.Arch)
			}
		})
	}
}

func officialClaudeInbound() service.OfficialInbound {
	bundle, ok := service.NewSoftwareBundleRegistry().Lookup(
		service.PlatformAnthropic,
		service.ClientFamilyClaudeCode,
		claude.CLICurrentVersion,
	)
	if !ok {
		panic("missing compile-time Claude software bundle")
	}
	return service.OfficialInbound{
		UserAgent:      bundle.UserAgent,
		ClientVersion:  bundle.ClientVersion,
		Runtime:        bundle.Runtime,
		RuntimeVersion: bundle.RuntimeVersion,
		Payload:        bundle.Payload,
	}
}

func oldClaudePayload() map[string]any {
	return map[string]any{
		"user_agent":                "claude-cli/0.1.0 (external, cli)",
		"stainless_lang":            "js",
		"stainless_package_version": "0.1.0",
		"stainless_os":              "Linux",
		"stainless_arch":            "arm64",
		"stainless_runtime":         "node",
		"stainless_runtime_version": "v18.0.0",
	}
}

func seedOldClaudeSoftware(t *testing.T, client *dbent.Client, accountID int64, learningEnabled bool) {
	t.Helper()
	n, err := client.AccountDeviceProfile.Update().
		Where(accountdeviceprofile.AccountID(accountID)).
		SetClientVersion("0.1.0").
		SetRuntimeVersion("v18.0.0").
		SetProfilePayload(oldClaudePayload()).
		SetLearningEnabled(learningEnabled).
		Save(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestLearnIfOfficialLearningDisabledDoesNotWrite(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	seedOldClaudeSoftware(t, client, account.ID, false)

	got, err := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Equal(t, "0.1.0", got.ClientVersion)
	require.Equal(t, service.LearnedFromBaseline, got.LearnedFrom)
	require.Zero(t, repo.casWriteCount(account.ID))
	require.Equal(t, int64(1), got.Revision)
}

func TestLearnIfOfficialUnknownHighVersionDoesNotWrite(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	seedOldClaudeSoftware(t, client, account.ID, true)

	inbound := officialClaudeInbound()
	inbound.ClientVersion = "999.0.0"
	inbound.UserAgent = "claude-cli/999.0.0 (external, cli)"
	if inbound.Payload != nil {
		inbound.Payload["user_agent"] = inbound.UserAgent
	}

	got, err := svc.LearnIfOfficial(ctx, account, inbound)
	require.NoError(t, err)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Equal(t, "0.1.0", got.ClientVersion)
	require.Zero(t, repo.casWriteCount(account.ID))
}

func TestLearnIfOfficialEqualOrLowerVersionDoesNotWrite(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.Equal(t, claude.CLICurrentVersion, created.ClientVersion)

	got, err := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
	require.NoError(t, err)
	require.Equal(t, created.Revision, got.Revision)
	require.Equal(t, created.ClientVersion, got.ClientVersion)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Zero(t, repo.casWriteCount(account.ID))
}

func TestLearnIfOfficialSameVersionOfficialDoesNotCallUpdateCAS(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	getsAfterCreate := repo.getCount(account.ID)
	insertsAfterCreate := repo.insertCount(account.ID)

	got, err := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
	require.NoError(t, err)
	require.Equal(t, created.Revision, got.Revision)
	require.Equal(t, created.ClientVersion, got.ClientVersion)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Zero(t, repo.casCallCount(account.ID))
	require.Zero(t, repo.casWriteCount(account.ID))
	require.Equal(t, insertsAfterCreate, repo.insertCount(account.ID))
	require.Equal(t, 1, repo.getCount(account.ID)-getsAfterCreate)
}

func TestLearnIfOfficialSameVersionPathIsLockFree(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)

	const workers = 2
	entered := make(chan struct{}, workers)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseAll)

	repo.beforeGet = func() {
		entered <- struct{}{}
		<-release
	}

	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			got, err := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
			errs[i] = err
			if err == nil && got != nil && got.DeviceID != created.DeviceID {
				errs[i] = fmt.Errorf("worker %d device id changed", i)
			}
		}(i)
	}

	for i := 0; i < workers; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatalf("LearnIfOfficial did not reach GetByAccountID concurrently (got %d/%d); same-version path is not lock-free", i, workers)
		}
	}
	releaseAll()
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "worker %d", i)
	}
	require.Zero(t, repo.casCallCount(account.ID))
}

func TestLearnIfOfficialSameVersionOfficialDoesNotProjectCache(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	cache := &recordingDeviceProfileCache{}
	svc = svc.WithCache(cache)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.NotZero(t, cache.sets)
	setsAfterCreate := cache.sets

	got, err := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
	require.NoError(t, err)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Zero(t, repo.casCallCount(account.ID))
	require.Equal(t, setsAfterCreate, cache.sets)
}

func TestLearnIfOfficialLockFreeSkipDoesNotOverwriteResetProjection(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	cache := &recordingDeviceProfileCache{}
	svc = svc.WithCache(cache)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.Equal(t, created.DeviceID, cache.profiles[account.ID].DeviceID)

	var reminted *service.AccountDeviceProfile
	repo.afterGet = func() {
		if reminted != nil {
			return
		}
		repo.afterGet = nil
		var resetErr error
		reminted, resetErr = svc.Reset(ctx, account)
		require.NoError(t, resetErr)
		require.NotNil(t, reminted)
		require.NotEqual(t, created.DeviceID, reminted.DeviceID)
	}

	got, err := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, reminted)
	require.Zero(t, repo.casCallCount(account.ID))
	require.Equal(t, reminted.DeviceID, cache.profiles[account.ID].DeviceID)
	require.Equal(t, reminted.InstallationID, cache.profiles[account.ID].InstallationID)
	require.Equal(t, reminted.GatewayAccountUUID, cache.profiles[account.ID].GatewayAccountUUID)
	require.NotEqual(t, created.DeviceID, cache.profiles[account.ID].DeviceID)
}

func TestLearnIfOfficialOfficialClaudeHigherVersionUpdatesSoftwareOnly(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	seedOldClaudeSoftware(t, client, account.ID, false)

	got, err := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
	require.NoError(t, err)
	require.NoError(t, service.ValidateAccountDeviceProfile(got))
	require.Equal(t, 1, repo.casWriteCount(account.ID))
	require.Equal(t, claude.CLICurrentVersion, got.ClientVersion)
	require.Equal(t, service.LearnedFromOfficial, got.LearnedFrom)
	require.Equal(t, claude.DefaultHeaders["User-Agent"], got.ProfilePayload["user_agent"])
	require.Equal(t, claude.DefaultHeaders["X-Stainless-Package-Version"], got.ProfilePayload["stainless_package_version"])
	require.Equal(t, claude.DefaultHeaders["X-Stainless-Runtime-Version"], got.RuntimeVersion)
	require.NotNil(t, got.VersionUpgradedAt)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Equal(t, created.InstallationID, got.InstallationID)
	require.Equal(t, created.GatewayAccountUUID, got.GatewayAccountUUID)
	require.Equal(t, created.SessionNamespace, got.SessionNamespace)
	require.Equal(t, created.MachineID, got.MachineID)
	require.Equal(t, created.OSFamily, got.OSFamily)
	require.Equal(t, created.Arch, got.Arch)
	require.Equal(t, created.Platform, got.Platform)
	require.Equal(t, created.ClientFamily, got.ClientFamily)
	require.Equal(t, int64(2), got.Revision)
}

func TestLearnIfOfficialClaudeUAChangeWithoutStainlessDoesNotWrite(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	seedOldClaudeSoftware(t, client, account.ID, true)

	inbound := officialClaudeInbound()
	inbound.Payload = oldClaudePayload()
	inbound.Payload["user_agent"] = inbound.UserAgent

	got, err := svc.LearnIfOfficial(ctx, account, inbound)
	require.NoError(t, err)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Equal(t, "0.1.0", got.ClientVersion)
	require.Equal(t, "claude-cli/0.1.0 (external, cli)", got.ProfilePayload["user_agent"])
	require.Zero(t, repo.casWriteCount(account.ID))
}

func TestLearnIfOfficialConcurrentSameAccountDoesNotCorrupt(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	seedOldClaudeSoftware(t, client, account.ID, true)

	const workers = 8
	profiles := make([]*service.AccountDeviceProfile, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			profiles[i], errs[i] = svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "worker %d", i)
		require.NotNil(t, profiles[i])
		require.NoError(t, service.ValidateAccountDeviceProfile(profiles[i]))
		require.Equal(t, created.DeviceID, profiles[i].DeviceID)
		require.Equal(t, created.InstallationID, profiles[i].InstallationID)
		require.Equal(t, created.GatewayAccountUUID, profiles[i].GatewayAccountUUID)
		require.Equal(t, created.SessionNamespace, profiles[i].SessionNamespace)
		require.Equal(t, claude.CLICurrentVersion, profiles[i].ClientVersion)
		require.Equal(t, service.LearnedFromOfficial, profiles[i].LearnedFrom)
	}
	require.Equal(t, 1, repo.casWriteCount(account.ID))

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(account.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestLearnIfOfficialPreservesPinnedOSArchPayload(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)

	pinned := oldClaudePayload()
	pinned["stainless_os"] = "Darwin"
	pinned["stainless_arch"] = "x64"
	n, err := client.AccountDeviceProfile.Update().
		Where(accountdeviceprofile.AccountID(account.ID)).
		SetClientVersion("0.1.0").
		SetRuntimeVersion("v18.0.0").
		SetProfilePayload(pinned).
		SetLearningEnabled(true).
		Save(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	got, err := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
	require.NoError(t, err)
	require.NoError(t, service.ValidateAccountDeviceProfile(got))
	require.Equal(t, 1, repo.casWriteCount(account.ID))
	require.Equal(t, claude.CLICurrentVersion, got.ClientVersion)
	require.Equal(t, "Darwin", got.ProfilePayload["stainless_os"])
	require.Equal(t, "x64", got.ProfilePayload["stainless_arch"])
	require.Equal(t, created.OSFamily, got.OSFamily)
	require.Equal(t, created.Arch, got.Arch)
	require.NotEqual(t, claude.DefaultHeaders["X-Stainless-OS"], got.ProfilePayload["stainless_os"])
	require.NotEqual(t, claude.DefaultHeaders["X-Stainless-Arch"], got.ProfilePayload["stainless_arch"])
}

func TestLearnIfOfficialUnprovenGeminiUADoesNotWrite(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformGemini, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	n, err := client.AccountDeviceProfile.Update().
		Where(accountdeviceprofile.AccountID(account.ID)).
		SetClientVersion("0.1.0").
		SetLearningEnabled(true).
		Save(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	bundle, ok := service.NewSoftwareBundleRegistry().Lookup(
		service.PlatformGemini,
		service.ClientFamilyGeminiCLI,
		created.ClientVersion,
	)
	require.True(t, ok)

	got, err := svc.LearnIfOfficial(ctx, account, service.OfficialInbound{
		UserAgent:     "curl/8.0 (gemini)",
		ClientVersion: bundle.ClientVersion,
	})
	require.NoError(t, err)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Equal(t, "0.1.0", got.ClientVersion)
	require.Equal(t, service.LearnedFromBaseline, got.LearnedFrom)
	require.Zero(t, repo.casWriteCount(account.ID))
}

func TestLearnIfOfficialUnverifiedH2IsPersistedAsH1(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	n, err := client.AccountDeviceProfile.Update().
		Where(accountdeviceprofile.AccountID(account.ID)).
		SetClientVersion("0.1.0").
		SetTransportFamily(service.TransportH2).
		SetLearningEnabled(true).
		Save(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	bundle, ok := service.NewSoftwareBundleRegistry().Lookup(
		service.PlatformOpenAI,
		service.ClientFamilyCodexCLI,
		created.ClientVersion,
	)
	require.True(t, ok)

	got, err := svc.LearnIfOfficial(ctx, account, service.OfficialInbound{
		UserAgent:     "codex_cli_rs/" + bundle.ClientVersion + " (Ubuntu 22.4.0; x86_64) xterm-256color",
		Originator:    "codex_cli_rs",
		ClientVersion: bundle.ClientVersion,
	})
	require.NoError(t, err)
	require.Equal(t, 1, repo.casWriteCount(account.ID))
	require.Equal(t, service.TransportH1, got.TransportFamily, "Learn/UpdateCAS must not keep unverified h2")

	stored, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(account.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, service.TransportH1, stored.TransportFamily)
}

func TestLearnIfOfficialOfficialCodexTUICLIHigherVersionUpdatesSoftwareOnly(t *testing.T) {
	cases := []struct {
		name       string
		userAgent  func(version string) string
		originator string
	}{
		{
			name: "codex_cli_rs",
			userAgent: func(version string) string {
				return "codex_cli_rs/" + version + " (Ubuntu 22.4.0; x86_64) xterm-256color"
			},
			originator: "codex_cli_rs",
		},
		{
			name: "codex-tui",
			userAgent: func(version string) string {
				return openai.CodexDefaultOriginator + "/" + version + " (Mac OS X 15.1.0; arm64) iTerm.app"
			},
			originator: openai.CodexDefaultOriginator,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, client, repo := newCountingAccountDeviceService(t)
			ctx := context.Background()
			account := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
				"device_learning_enabled": true,
			})

			created, err := svc.GetOrCreate(ctx, account)
			require.NoError(t, err)
			n, err := client.AccountDeviceProfile.Update().
				Where(accountdeviceprofile.AccountID(account.ID)).
				SetClientVersion("0.1.0").
				SetLearningEnabled(true).
				Save(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, n)

			bundle, ok := service.NewSoftwareBundleRegistry().Lookup(
				service.PlatformOpenAI,
				service.ClientFamilyCodexCLI,
				created.ClientVersion,
			)
			require.True(t, ok)

			got, err := svc.LearnIfOfficial(ctx, account, service.OfficialInbound{
				UserAgent:     tc.userAgent(bundle.ClientVersion),
				Originator:    tc.originator,
				ClientVersion: bundle.ClientVersion,
			})
			require.NoError(t, err)
			require.NoError(t, service.ValidateAccountDeviceProfile(got))
			require.Equal(t, 1, repo.casWriteCount(account.ID))
			require.Equal(t, bundle.ClientVersion, got.ClientVersion)
			require.Equal(t, service.LearnedFromOfficial, got.LearnedFrom)
			require.Equal(t, created.DeviceID, got.DeviceID)
			require.Equal(t, created.InstallationID, got.InstallationID)
			require.Equal(t, created.GatewayAccountUUID, got.GatewayAccountUUID)
			require.Equal(t, created.SessionNamespace, got.SessionNamespace)
			require.Equal(t, created.MachineID, got.MachineID)
			require.Equal(t, created.OSFamily, got.OSFamily)
			require.Equal(t, created.Arch, got.Arch)
			require.Equal(t, created.Platform, got.Platform)
			require.Equal(t, created.ClientFamily, got.ClientFamily)
		})
	}
}

func TestLearnIfOfficialCodexVSCodeUADoesNotWrite(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	n, err := client.AccountDeviceProfile.Update().
		Where(accountdeviceprofile.AccountID(account.ID)).
		SetClientVersion("0.1.0").
		SetLearningEnabled(true).
		Save(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	bundle, ok := service.NewSoftwareBundleRegistry().Lookup(
		service.PlatformOpenAI,
		service.ClientFamilyCodexCLI,
		created.ClientVersion,
	)
	require.True(t, ok)

	got, err := svc.LearnIfOfficial(ctx, account, service.OfficialInbound{
		UserAgent:     "codex_vscode/" + bundle.ClientVersion,
		Originator:    "codex_vscode",
		ClientVersion: bundle.ClientVersion,
	})
	require.NoError(t, err)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Equal(t, "0.1.0", got.ClientVersion)
	require.Equal(t, service.LearnedFromBaseline, got.LearnedFrom)
	require.Zero(t, repo.casWriteCount(account.ID))
}

func TestLearnIfOfficialCodexOriginatorOnlyDoesNotWrite(t *testing.T) {
	cases := []struct {
		name      string
		userAgent string
	}{
		{name: "empty UA", userAgent: ""},
		{name: "non-CLI UA without version", userAgent: "python-httpx"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, client, repo := newCountingAccountDeviceService(t)
			ctx := context.Background()
			account := mustCreateDeviceAccount(t, client, service.PlatformOpenAI, map[string]any{
				"device_learning_enabled": true,
			})

			created, err := svc.GetOrCreate(ctx, account)
			require.NoError(t, err)
			n, err := client.AccountDeviceProfile.Update().
				Where(accountdeviceprofile.AccountID(account.ID)).
				SetClientVersion("0.1.0").
				SetLearningEnabled(true).
				Save(ctx)
			require.NoError(t, err)
			require.Equal(t, 1, n)

			bundle, ok := service.NewSoftwareBundleRegistry().Lookup(
				service.PlatformOpenAI,
				service.ClientFamilyCodexCLI,
				created.ClientVersion,
			)
			require.True(t, ok)

			got, err := svc.LearnIfOfficial(ctx, account, service.OfficialInbound{
				UserAgent:     tc.userAgent,
				Originator:    "codex_cli_rs",
				ClientVersion: bundle.ClientVersion,
			})
			require.NoError(t, err)
			require.Equal(t, created.DeviceID, got.DeviceID)
			require.Equal(t, "0.1.0", got.ClientVersion)
			require.Equal(t, service.LearnedFromBaseline, got.LearnedFrom)
			require.Zero(t, repo.casWriteCount(account.ID))
		})
	}
}

func TestLearnIfOfficialClaudeSupersetStainlessStillLearns(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	seedOldClaudeSoftware(t, client, account.ID, true)

	inbound := officialClaudeInbound()
	inbound.Payload["stainless_retry_count"] = "0"
	inbound.Payload["stainless_timeout"] = "600"

	got, err := svc.LearnIfOfficial(ctx, account, inbound)
	require.NoError(t, err)
	require.NoError(t, service.ValidateAccountDeviceProfile(got))
	require.Equal(t, 1, repo.casWriteCount(account.ID))
	require.Equal(t, claude.CLICurrentVersion, got.ClientVersion)
	require.Equal(t, service.LearnedFromOfficial, got.LearnedFrom)
	require.Equal(t, created.DeviceID, got.DeviceID)
}

func TestLearnIfOfficialRuntimeFamilyChangeDoesNotWrite(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	n, err := client.AccountDeviceProfile.Update().
		Where(accountdeviceprofile.AccountID(account.ID)).
		SetClientVersion("0.1.0").
		SetRuntime("bun").
		SetRuntimeVersion("v18.0.0").
		SetProfilePayload(oldClaudePayload()).
		SetLearningEnabled(true).
		Save(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	got, err := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
	require.NoError(t, err)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Equal(t, "0.1.0", got.ClientVersion)
	require.Equal(t, "bun", got.Runtime)
	require.Equal(t, service.LearnedFromBaseline, got.LearnedFrom)
	require.Zero(t, repo.casWriteCount(account.ID))
}

func TestLearnIfOfficialClaudeInboundNodeVersionMismatchStillLearns(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	getsAfterCreate := repo.getCount(account.ID)
	seedOldClaudeSoftware(t, client, account.ID, true)

	inbound := officialClaudeInbound()
	payload := make(map[string]any, len(inbound.Payload))
	for key, value := range inbound.Payload {
		payload[key] = value
	}
	payload["stainless_runtime_version"] = "v22.14.0"
	inbound.Payload = payload
	inbound.RuntimeVersion = "v22.14.0"

	got, err := svc.LearnIfOfficial(ctx, account, inbound)
	require.NoError(t, err)
	require.NoError(t, service.ValidateAccountDeviceProfile(got))
	require.Equal(t, 1, repo.casCallCount(account.ID))
	require.Equal(t, 1, repo.casWriteCount(account.ID))
	require.GreaterOrEqual(t, repo.getCount(account.ID)-getsAfterCreate, 2)
	require.Equal(t, claude.CLICurrentVersion, got.ClientVersion)
	require.Equal(t, service.LearnedFromOfficial, got.LearnedFrom)
	require.Equal(t, claude.DefaultHeaders["X-Stainless-Runtime-Version"], got.RuntimeVersion)
	require.Equal(t, claude.DefaultHeaders["X-Stainless-Runtime-Version"], got.ProfilePayload["stainless_runtime_version"])
	require.NotEqual(t, "v22.14.0", got.RuntimeVersion)
	require.NotEqual(t, "v22.14.0", got.ProfilePayload["stainless_runtime_version"])
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Equal(t, created.InstallationID, got.InstallationID)
	require.Equal(t, created.OSFamily, got.OSFamily)
	require.Equal(t, created.Arch, got.Arch)
}

func TestLearnIfOfficialMismatchedUAVersionDoesNotWrite(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	seedOldClaudeSoftware(t, client, account.ID, true)

	inbound := officialClaudeInbound()
	inbound.UserAgent = "claude-cli/0.1.0 (external, cli)"
	inbound.Payload = oldClaudePayload()

	got, err := svc.LearnIfOfficial(ctx, account, inbound)
	require.NoError(t, err)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.Equal(t, "0.1.0", got.ClientVersion)
	require.Equal(t, "claude-cli/0.1.0 (external, cli)", got.ProfilePayload["user_agent"])
	require.Equal(t, service.LearnedFromBaseline, got.LearnedFrom)
	require.Equal(t, int64(1), got.Revision)
	require.Zero(t, repo.casWriteCount(account.ID))
}

func TestLearnIfOfficialDarwinX64InboundStillLearnsSoftware(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)

	pinned := oldClaudePayload()
	pinned["stainless_os"] = "Darwin"
	pinned["stainless_arch"] = "x64"
	n, err := client.AccountDeviceProfile.Update().
		Where(accountdeviceprofile.AccountID(account.ID)).
		SetClientVersion("0.1.0").
		SetRuntimeVersion("v18.0.0").
		SetOsFamily("macos").
		SetArch("x64").
		SetProfilePayload(pinned).
		SetLearningEnabled(true).
		Save(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	inbound := officialClaudeInbound()
	payload := make(map[string]any, len(inbound.Payload)+2)
	for key, value := range inbound.Payload {
		payload[key] = value
	}
	payload["stainless_os"] = "Darwin"
	payload["stainless_arch"] = "x64"
	inbound.Payload = payload

	got, err := svc.LearnIfOfficial(ctx, account, inbound)
	require.NoError(t, err)
	require.NoError(t, service.ValidateAccountDeviceProfile(got))
	require.Equal(t, 1, repo.casWriteCount(account.ID))
	require.Equal(t, claude.CLICurrentVersion, got.ClientVersion)
	require.Equal(t, service.LearnedFromOfficial, got.LearnedFrom)
	require.Equal(t, claude.DefaultHeaders["X-Stainless-Package-Version"], got.ProfilePayload["stainless_package_version"])
	require.Equal(t, claude.DefaultHeaders["X-Stainless-Lang"], got.ProfilePayload["stainless_lang"])
	require.Equal(t, claude.DefaultHeaders["X-Stainless-Runtime"], got.ProfilePayload["stainless_runtime"])
	require.Equal(t, claude.DefaultHeaders["X-Stainless-Runtime-Version"], got.ProfilePayload["stainless_runtime_version"])
	require.Equal(t, "Darwin", got.ProfilePayload["stainless_os"])
	require.Equal(t, "x64", got.ProfilePayload["stainless_arch"])
	require.Equal(t, "macos", got.OSFamily)
	require.Equal(t, "x64", got.Arch)
	require.Equal(t, created.DeviceID, got.DeviceID)
	require.NotEqual(t, claude.DefaultHeaders["X-Stainless-OS"], got.ProfilePayload["stainless_os"])
	require.NotEqual(t, claude.DefaultHeaders["X-Stainless-Arch"], got.ProfilePayload["stainless_arch"])
}

type recordingDeviceProfileCache struct {
	mu       sync.Mutex
	profiles map[int64]*service.AccountDeviceProfile
	setErr   error
	sets     int
}

func (c *recordingDeviceProfileCache) GetFingerprint(context.Context, int64) (*service.Fingerprint, error) {
	return nil, nil
}
func (c *recordingDeviceProfileCache) SetFingerprint(context.Context, int64, *service.Fingerprint) error {
	return nil
}
func (c *recordingDeviceProfileCache) GetMaskedSessionID(context.Context, int64) (string, error) {
	return "", nil
}
func (c *recordingDeviceProfileCache) SetMaskedSessionID(context.Context, int64, string) error {
	return nil
}
func (c *recordingDeviceProfileCache) GetDeviceProfile(_ context.Context, accountID int64) (*service.AccountDeviceProfile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.profiles == nil {
		return nil, nil
	}
	return c.profiles[accountID], nil
}
func (c *recordingDeviceProfileCache) SetDeviceProfile(_ context.Context, accountID int64, p *service.AccountDeviceProfile) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets++
	if c.setErr != nil {
		return c.setErr
	}
	if c.profiles == nil {
		c.profiles = map[int64]*service.AccountDeviceProfile{}
	}
	c.profiles[accountID] = p
	return nil
}
func (c *recordingDeviceProfileCache) DeleteDeviceProfile(_ context.Context, accountID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.profiles, accountID)
	return nil
}

func TestGetOrCreateProjectsDeviceProfileToCacheOnHitAndInsert(t *testing.T) {
	svc, client, _ := newCountingAccountDeviceService(t)
	cache := &recordingDeviceProfileCache{}
	svc = svc.WithCache(cache)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, 1, cache.sets)
	require.Equal(t, created.DeviceID, cache.profiles[account.ID].DeviceID)

	hit, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.Equal(t, created.ID, hit.ID)
	require.Equal(t, 2, cache.sets)
}

func TestGetOrCreateCacheErrorDoesNotFailRequest(t *testing.T) {
	svc, client, _ := newCountingAccountDeviceService(t)
	cache := &recordingDeviceProfileCache{setErr: fmt.Errorf("redis unavailable")}
	svc = svc.WithCache(cache)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)

	got, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, 1, cache.sets)
}

func TestNewAccountDeviceServiceWorksWithoutCache(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)

	got, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestLearnIfOfficialProjectsDeviceProfileAfterCAS(t *testing.T) {
	svc, client, repo := newCountingAccountDeviceService(t)
	cache := &recordingDeviceProfileCache{}
	svc = svc.WithCache(cache)
	ctx := context.Background()
	account := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, map[string]any{
		"device_learning_enabled": true,
	})

	created, err := svc.GetOrCreate(ctx, account)
	require.NoError(t, err)
	require.Equal(t, 1, cache.sets)
	seedOldClaudeSoftware(t, client, account.ID, false)

	got, err := svc.LearnIfOfficial(ctx, account, officialClaudeInbound())
	require.NoError(t, err)
	require.Equal(t, 1, repo.casWriteCount(account.ID))
	require.Equal(t, 3, cache.sets)
	require.Equal(t, got.ClientVersion, cache.profiles[account.ID].ClientVersion)
	require.Equal(t, claude.CLICurrentVersion, cache.profiles[account.ID].ClientVersion)
	require.Equal(t, created.DeviceID, cache.profiles[account.ID].DeviceID)
}
