//go:build unit

package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/accountdeviceprofile"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newAccountDeviceService(t *testing.T) (*service.AccountDeviceService, *dbent.Client) {
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

	repo := repository.NewAccountDeviceProfileRepository(client)
	return service.NewAccountDeviceService(repo), client
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

func TestGetOrCreateShadowDoesNotInsert(t *testing.T) {
	svc, client := newAccountDeviceService(t)
	ctx := context.Background()
	parent := mustCreateDeviceAccount(t, client, service.PlatformAnthropic, nil)

	shadowRow, err := client.Account.Create().
		SetName("shadow").
		SetPlatform(service.PlatformAnthropic).
		SetType(service.AccountTypeAPIKey).
		SetStatus(service.StatusActive).
		SetCredentials(map[string]any{}).
		SetParentAccountID(parent.ID).
		Save(ctx)
	require.NoError(t, err)

	shadow := &service.Account{
		ID:              shadowRow.ID,
		Platform:        shadowRow.Platform,
		ParentAccountID: shadowRow.ParentAccountID,
	}
	require.True(t, shadow.IsShadow())

	got, err := svc.GetOrCreate(ctx, shadow)
	require.Error(t, err)
	require.Nil(t, got)

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(shadow.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}
