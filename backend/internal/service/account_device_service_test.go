//go:build unit

package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/accountdeviceprofile"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
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
	inner     service.AccountDeviceProfileRepository
	mu        sync.Mutex
	inserts   map[int64]int
	casWrites map[int64]int
}

func wrapCountingDeviceProfileRepo(inner service.AccountDeviceProfileRepository) *countingDeviceProfileRepo {
	return &countingDeviceProfileRepo{inner: inner, inserts: map[int64]int{}, casWrites: map[int64]int{}}
}

func (r *countingDeviceProfileRepo) GetByAccountID(ctx context.Context, accountID int64) (*service.AccountDeviceProfile, error) {
	return r.inner.GetByAccountID(ctx, accountID)
}

func (r *countingDeviceProfileRepo) InsertBaseline(ctx context.Context, p *service.AccountDeviceProfile) (*service.AccountDeviceProfile, error) {
	r.mu.Lock()
	r.inserts[p.AccountID]++
	r.mu.Unlock()
	return r.inner.InsertBaseline(ctx, p)
}

func (r *countingDeviceProfileRepo) UpdateCAS(ctx context.Context, accountID, expectedRevision int64, next *service.AccountDeviceProfile) (bool, error) {
	ok, err := r.inner.UpdateCAS(ctx, accountID, expectedRevision, next)
	if err == nil && ok {
		r.mu.Lock()
		r.casWrites[accountID]++
		r.mu.Unlock()
	}
	return ok, err
}

func (r *countingDeviceProfileRepo) insertCount(accountID int64) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inserts[accountID]
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
	return service.NewAccountDeviceService(counting), client, counting
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
