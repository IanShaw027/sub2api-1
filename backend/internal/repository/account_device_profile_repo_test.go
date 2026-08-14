//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/accountdeviceprofile"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newAccountDeviceProfileRepo(t *testing.T) (service.AccountDeviceProfileRepository, *dbent.Client) {
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

	return NewAccountDeviceProfileRepository(client), client
}

func mustCreateRepoAccount(t *testing.T, client *dbent.Client) int64 {
	t.Helper()
	row, err := client.Account.Create().
		SetName("repo-account").
		SetPlatform(service.PlatformAnthropic).
		SetType(service.AccountTypeAPIKey).
		SetStatus(service.StatusActive).
		SetCredentials(map[string]any{"api_key": "sk-test"}).
		Save(context.Background())
	require.NoError(t, err)
	return row.ID
}

func validRepoBaseline(accountID int64) *service.AccountDeviceProfile {
	return &service.AccountDeviceProfile{
		AccountID:          accountID,
		Revision:           1,
		SchemaVersion:      1,
		Platform:           service.PlatformAnthropic,
		ClientFamily:       service.ClientFamilyClaudeCode,
		InstallationID:     "11111111-1111-4111-8111-111111111111",
		DeviceID:           "22222222-2222-4222-8222-222222222222",
		MachineID:          "44444444-4444-4444-8444-444444444444",
		GatewayAccountUUID: "55555555-5555-4555-8555-555555555555",
		SessionNamespace:   "0123456789abcdef0123456789abcdef",
		OSFamily:           "macos",
		Arch:               "arm64",
		Runtime:            "node",
		RuntimeVersion:     "v24.3.0",
		ClientVersion:      "2.1.220",
		TransportFamily:    service.TransportH1,
		ProfilePayload: map[string]any{
			"user_agent": "claude-cli/2.1.220 (external, cli)",
		},
		LearnedFrom:     service.LearnedFromBaseline,
		LearningEnabled: false,
	}
}

func TestAccountDeviceProfileRepositoryGetByAccountIDMissing(t *testing.T) {
	repo, _ := newAccountDeviceProfileRepo(t)

	got, err := repo.GetByAccountID(context.Background(), 999)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestAccountDeviceProfileRepositoryInsertBaselineUniqueAccountID(t *testing.T) {
	repo, client := newAccountDeviceProfileRepo(t)
	ctx := context.Background()
	accountID := mustCreateRepoAccount(t, client)

	first, err := repo.InsertBaseline(ctx, validRepoBaseline(accountID))
	require.NoError(t, err)
	require.NotNil(t, first)

	_, err = repo.InsertBaseline(ctx, validRepoBaseline(accountID))
	require.Error(t, err)
	require.True(t, dbent.IsConstraintError(err))

	n, err := client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(accountID)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestAccountDeviceProfileRepositoryUpdateCASSuccessAndMismatch(t *testing.T) {
	repo, client := newAccountDeviceProfileRepo(t)
	ctx := context.Background()
	accountID := mustCreateRepoAccount(t, client)

	created, err := repo.InsertBaseline(ctx, validRepoBaseline(accountID))
	require.NoError(t, err)
	require.Equal(t, int64(1), created.Revision)

	next := validRepoBaseline(accountID)
	next.ClientVersion = "2.1.221"
	next.SessionNamespace = "ffffffffffffffffffffffffffffffff"
	next.DeviceID = created.DeviceID
	next.InstallationID = created.InstallationID
	next.MachineID = created.MachineID
	next.GatewayAccountUUID = created.GatewayAccountUUID

	ok, err := repo.UpdateCAS(ctx, accountID, created.Revision, next)
	require.NoError(t, err)
	require.True(t, ok)

	updated, err := repo.GetByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, int64(2), updated.Revision)
	require.Equal(t, "2.1.221", updated.ClientVersion)
	require.Equal(t, created.SessionNamespace, updated.SessionNamespace)

	ok, err = repo.UpdateCAS(ctx, accountID, created.Revision, next)
	require.NoError(t, err)
	require.False(t, ok)

	still, err := repo.GetByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, int64(2), still.Revision)
	require.Equal(t, created.SessionNamespace, still.SessionNamespace)
}

func TestAccountDeviceProfileRepositoryUpdateCASPreservesFirstWriteIdentity(t *testing.T) {
	repo, client := newAccountDeviceProfileRepo(t)
	ctx := context.Background()
	accountID := mustCreateRepoAccount(t, client)

	created, err := repo.InsertBaseline(ctx, validRepoBaseline(accountID))
	require.NoError(t, err)

	next := validRepoBaseline(accountID)
	next.ClientVersion = "2.1.221"
	next.Runtime = "bun"
	next.DeviceID = "99999999-9999-4999-8999-999999999999"
	next.GatewayAccountUUID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	next.OSFamily = "linux"
	next.Arch = "x64"
	next.InstallationID = "88888888-8888-4888-8888-888888888888"
	next.MachineID = "77777777-7777-4777-8777-777777777777"
	next.ClientID = "changed-client"
	next.Platform = service.PlatformOpenAI
	next.ClientFamily = service.ClientFamilyCodexCLI

	ok, err := repo.UpdateCAS(ctx, accountID, created.Revision, next)
	require.NoError(t, err)
	require.True(t, ok)

	updated, err := repo.GetByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.Equal(t, int64(2), updated.Revision)
	require.Equal(t, "2.1.221", updated.ClientVersion)
	require.Equal(t, "bun", updated.Runtime)
	require.Equal(t, created.DeviceID, updated.DeviceID)
	require.Equal(t, created.GatewayAccountUUID, updated.GatewayAccountUUID)
	require.Equal(t, created.OSFamily, updated.OSFamily)
	require.Equal(t, created.Arch, updated.Arch)
	require.Equal(t, created.InstallationID, updated.InstallationID)
	require.Equal(t, created.MachineID, updated.MachineID)
	require.Equal(t, created.ClientID, updated.ClientID)
	require.Equal(t, created.Platform, updated.Platform)
	require.Equal(t, created.ClientFamily, updated.ClientFamily)
	require.Equal(t, created.SessionNamespace, updated.SessionNamespace)
}
