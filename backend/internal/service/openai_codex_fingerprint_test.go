package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCodexFingerprintSeed = "11111111-1111-4111-8111-111111111111"

func newTestOAuthAccount(id int64, extra map[string]any) *Account {
	if codexFingerprintModeRequiresSeed(codexFingerprintModeFromExtra(extra)) {
		if extra == nil {
			extra = make(map[string]any)
		}
		if _, exists := extra[codexFingerprintSeedExtraKey]; !exists {
			extra[codexFingerprintSeedExtraKey] = testCodexFingerprintSeed
		}
	}
	return &Account{
		ID:       id,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra:    extra,
	}
}

type mapDeviceProfileRepo struct {
	profiles        map[int64]*AccountDeviceProfile
	getErr          error
	requireDeadline bool
	sawDeadline     bool
}

func (r *mapDeviceProfileRepo) GetByAccountID(ctx context.Context, accountID int64) (*AccountDeviceProfile, error) {
	if r.requireDeadline {
		if _, ok := ctx.Deadline(); !ok {
			return nil, fmt.Errorf("identity_reject: load context has no deadline")
		}
		r.sawDeadline = true
	}
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.profiles == nil {
		return nil, nil
	}
	return r.profiles[accountID], nil
}

func (r *mapDeviceProfileRepo) InsertBaseline(_ context.Context, p *AccountDeviceProfile) (*AccountDeviceProfile, error) {
	return nil, fmt.Errorf("identity_reject: unexpected baseline insert in test")
}

// mintingDeviceProfileRepo persists GetOrCreate baselines so a later
// GetIfExists/load can observe the same minted identity.
type mintingDeviceProfileRepo struct {
	mapDeviceProfileRepo
}

func (r *mintingDeviceProfileRepo) InsertBaseline(_ context.Context, p *AccountDeviceProfile) (*AccountDeviceProfile, error) {
	if r.profiles == nil {
		r.profiles = map[int64]*AccountDeviceProfile{}
	}
	copied := *p
	if p.ProfilePayload != nil {
		copied.ProfilePayload = maps.Clone(p.ProfilePayload)
	}
	r.profiles[p.AccountID] = &copied
	return &copied, nil
}

func (r *mapDeviceProfileRepo) UpdateCAS(context.Context, int64, int64, *AccountDeviceProfile) (bool, error) {
	return false, nil
}

func (r *mapDeviceProfileRepo) DeleteByAccountID(_ context.Context, accountID int64) error {
	if r.profiles != nil {
		delete(r.profiles, accountID)
	}
	return nil
}

func validOpenAIDeviceProfile(accountID int64) *AccountDeviceProfile {
	version := NormalizeCodexClientVersion(codexCLIVersion)
	if version == "" {
		version = codexCLIVersion
	}
	return &AccountDeviceProfile{
		ID:                 accountID,
		AccountID:          accountID,
		Revision:           1,
		SchemaVersion:      1,
		Platform:           PlatformOpenAI,
		ClientFamily:       ClientFamilyCodexCLI,
		InstallationID:     "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		DeviceID:           "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		MachineID:          "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		GatewayAccountUUID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
		SessionNamespace:   "0123456789abcdef0123456789abcdef",
		OSFamily:           "linux",
		Arch:               "x64",
		Runtime:            "codex_cli_rs",
		RuntimeVersion:     version,
		ClientVersion:      version,
		TransportFamily:    TransportH1,
		ProfilePayload: map[string]any{
			"user_agent": buildCodexCLIUserAgent(version),
			"originator": openai.CodexDefaultOriginator,
		},
		LearnedFrom: LearnedFromBaseline,
	}
}

func injectOutboundDeviceProfile(t *testing.T, accountID int64, profile *AccountDeviceProfile) {
	t.Helper()
	prev := OutboundDeviceProfileService()
	repo := &mapDeviceProfileRepo{profiles: map[int64]*AccountDeviceProfile{}}
	if profile != nil {
		repo.profiles[accountID] = profile
	}
	SetOutboundDeviceProfileService(NewAccountDeviceService(repo))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })
}

func injectOutboundDeviceProfileError(t *testing.T, err error) {
	t.Helper()
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(&mapDeviceProfileRepo{getErr: err}))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })
}

func injectProfileForAccount(t *testing.T, account *Account) *AccountDeviceProfile {
	t.Helper()
	profile := validOpenAIDeviceProfile(account.ID)
	injectOutboundDeviceProfile(t, account.ID, profile)
	return profile
}

type failAfterMapDeviceRepo struct {
	mapDeviceProfileRepo
	failAfter int
	gets      int
}

func (r *failAfterMapDeviceRepo) GetByAccountID(ctx context.Context, accountID int64) (*AccountDeviceProfile, error) {
	r.gets++
	if r.failAfter > 0 && r.gets > r.failAfter {
		return nil, fmt.Errorf("identity_reject: second profile load failed")
	}
	return r.mapDeviceProfileRepo.GetByAccountID(ctx, accountID)
}

// First load succeeds and second fails: body must not keep a profile/extra
// installation id unless the same resolve produced shared header IDs.
func TestCodexSharedRequestIdentity_SecondLoadFailureDoesNotKeepBodyInstallationID(t *testing.T) {
	profile := validOpenAIDeviceProfile(16)
	profile.InstallationID = "profile-install-id-16"
	repo := &failAfterMapDeviceRepo{
		mapDeviceProfileRepo: mapDeviceProfileRepo{
			profiles: map[int64]*AccountDeviceProfile{16: profile},
		},
		failAfter: 1,
	}
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(repo))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	account := newTestOAuthAccount(16, map[string]any{
		codexFingerprintModeExtraKey: "session",
		"openai_device_id":           "extra-device-id",
	})
	body := map[string]any{}

	ids := applyCodexSharedRequestIdentity(context.Background(), body, account, nil)

	bodyDump := fmt.Sprintf("%v", body)
	if ids == nil {
		require.NotContains(t, bodyDump, profile.InstallationID, "body must not keep profile installation id without shared header identity")
	} else {
		cm, _ := body["client_metadata"].(map[string]any)
		require.Equal(t, profile.InstallationID, cm["x-codex-installation-id"])
		require.Equal(t, profile.InstallationID, ids.installationID)
	}
	require.NotContains(t, bodyDump, "extra-device-id")
	require.Equal(t, 1, repo.gets, "request path must load the profile only once")
}

// Forward identity block: first Get succeeds and second fails. Body may keep
// x-codex-installation-id only when the same resolve stored header IDs.
// Restoring applyCodexClientMetadata + resolveCodexFingerprintIDsFromRequest
// in applyCodexForwardRequestIdentity (the Forward identity block) fails this.
func TestCodexForwardRequestIdentity_SecondLoadFailureDoesNotKeepBodyInstallationID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	profile := validOpenAIDeviceProfile(17)
	profile.InstallationID = "profile-install-id-17"
	repo := &failAfterMapDeviceRepo{
		mapDeviceProfileRepo: mapDeviceProfileRepo{
			profiles: map[int64]*AccountDeviceProfile{17: profile},
		},
		failAfter: 1,
	}
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(repo))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	account := newTestOAuthAccount(17, map[string]any{
		codexFingerprintModeExtraKey: "session",
		"openai_device_id":           "extra-device-id",
	})
	body := map[string]any{}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	ids := applyCodexForwardRequestIdentity(context.Background(), c, body, account, nil)

	_, stored := c.Get("codex_fingerprint_ids")
	cm, _ := body["client_metadata"].(map[string]any)
	bodyInstall, _ := cm["x-codex-installation-id"].(string)
	if bodyInstall != "" {
		require.NotNil(t, ids, "body installation id must not outlive a failed second profile load")
		require.True(t, stored, "body installation id is present only if shared header IDs were stored")
		require.Equal(t, profile.InstallationID, bodyInstall)
		require.Equal(t, profile.InstallationID, ids.installationID)
	} else {
		require.Nil(t, ids)
		require.False(t, stored)
	}
	require.NotContains(t, fmt.Sprintf("%v", body), "extra-device-id")
}

func TestCodexForward_IdentityBlockUsesSingleSharedLoad(t *testing.T) {
	src, err := os.ReadFile("openai_gateway_forward.go")
	require.NoError(t, err)
	text := string(src)
	if !strings.Contains(text, "applyCodexForwardRequestIdentity(") {
		t.Fatal("Forward must call applyCodexForwardRequestIdentity")
	}
	clientMeta := strings.Index(text, "applyCodexClientMetadata(")
	resolve := strings.Index(text, "resolveCodexFingerprintIDsFromRequest(")
	if clientMeta >= 0 && resolve > clientMeta {
		t.Fatal("Forward restored applyCodexClientMetadata + resolveCodexFingerprintIDsFromRequest")
	}
}

// --- deriveStableUUIDv4 ---

func TestDeriveStableUUIDv4_Deterministic(t *testing.T) {
	a := deriveStableUUIDv4("test-seed-1")
	b := deriveStableUUIDv4("test-seed-1")
	assert.Equal(t, a, b, "同一种子应返回相同结果")
}

func TestDeriveStableUUIDv4_DifferentSeeds(t *testing.T) {
	a := deriveStableUUIDv4("seed-a")
	b := deriveStableUUIDv4("seed-b")
	assert.NotEqual(t, a, b, "不同种子应返回不同结果")
}

func TestDeriveStableUUIDv4_ValidFormat(t *testing.T) {
	result := deriveStableUUIDv4("test-seed")
	parsed, err := uuid.Parse(result)
	require.NoError(t, err, "应返回合法 UUID 格式")
	assert.Equal(t, uuid.Version(4), parsed.Version(), "应为 UUIDv4")
	assert.Equal(t, uuid.RFC4122, parsed.Variant(), "应为 RFC4122 变体")
}

// --- GetCodexFingerprintMode ---

func TestGetCodexFingerprintMode(t *testing.T) {
	tests := []struct {
		name     string
		account  *Account
		expected codexFingerprintMode
	}{
		{"nil 账号", nil, codexFingerprintOff},
		{"非 OAuth 账号", &Account{Platform: PlatformOpenAI, Type: "api_key"}, codexFingerprintOff},
		{"OpenAI setup token", &Account{Platform: PlatformOpenAI, Type: AccountTypeSetupToken, Extra: map[string]any{codexFingerprintModeExtraKey: "session"}}, codexFingerprintSession},
		{"Anthropic setup token", &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken, Extra: map[string]any{codexFingerprintModeExtraKey: "session"}}, codexFingerprintOff},
		// 收敛是显式 opt-in：缺省/空/非法一律 off（#5610）。存量账号普遍没有这个
		// extra 键，升级不得把它们静默切进收敛。
		{"无 extra 默认 off", newTestOAuthAccount(1, nil), codexFingerprintOff},
		{"空值默认 off", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: ""}), codexFingerprintOff},
		{"非法值默认 off", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "invalid"}), codexFingerprintOff},
		{"显式 off", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "off"}), codexFingerprintOff},
		{"device", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "device"}), codexFingerprintDevice},
		{"session", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "session"}), codexFingerprintSession},
		{"full", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "full"}), codexFingerprintFull},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.account.GetCodexFingerprintMode())
		})
	}
}

// --- resolveConvergedInstallationID ---

func TestResolveConvergedInstallationID_IgnoresExtraDeviceID(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{"openai_device_id": "extra-device-id"})
	profile := injectProfileForAccount(t, account)
	require.NotEqual(t, "extra-device-id", profile.InstallationID)
	assert.Equal(t, profile.InstallationID, resolveConvergedInstallationIDFromProfile(context.Background(), account))
	assert.NotEqual(t, "extra-device-id", resolveConvergedInstallationIDFromProfile(context.Background(), account))
}

func TestResolveConvergedInstallationID_UsesDeviceID(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{"openai_device_id": "real-device-id"})
	assert.Equal(t, "real-device-id", resolveConvergedInstallationID(account, testCodexFingerprintSeed))
}

func TestResolveConvergedInstallationID_UsesInjectedProfile(t *testing.T) {
	account := newTestOAuthAccount(42, nil)
	profile := injectProfileForAccount(t, account)
	result := resolveConvergedInstallationIDFromProfile(context.Background(), account)
	_, err := uuid.Parse(result)
	require.NoError(t, err, "档案 installation_id 应为合法 UUID")
	assert.Equal(t, profile.InstallationID, result)
	assert.Equal(t, result, resolveConvergedInstallationIDFromProfile(context.Background(), account), "确定性")
}

func TestResolveConvergedInstallationID_DerivesFromSeed(t *testing.T) {
	account := newTestOAuthAccount(42, nil)
	result := resolveConvergedInstallationID(account, testCodexFingerprintSeed)
	_, err := uuid.Parse(result)
	require.NoError(t, err, "派生值应为合法 UUID")
	assert.Equal(t, result, resolveConvergedInstallationID(account, testCodexFingerprintSeed), "确定性")
}

func TestResolveConvergedInstallationID_EmptyInstallationIDSkips(t *testing.T) {
	profile := validOpenAIDeviceProfile(3)
	profile.InstallationID = ""
	injectOutboundDeviceProfile(t, 3, profile)

	account := newTestOAuthAccount(3, nil)
	require.Empty(t, resolveConvergedInstallationIDFromProfile(context.Background(), account))
	require.NotEqual(t, profile.DeviceID, resolveConvergedInstallationIDFromProfile(context.Background(), account))
	require.Nil(t, resolveCodexFingerprintIDsFromRequest(account, nil))
}

func TestResolveConvergedInstallationID_LoadUsesBoundedContext(t *testing.T) {
	profile := validOpenAIDeviceProfile(8)
	repo := &mapDeviceProfileRepo{
		profiles:        map[int64]*AccountDeviceProfile{8: profile},
		requireDeadline: true,
	}
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(repo))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	account := newTestOAuthAccount(8, nil)
	require.Equal(t, profile.InstallationID, resolveConvergedInstallationIDFromProfile(context.Background(), account))
	require.True(t, repo.sawDeadline, "LoadOutboundDeviceProfile 必须带 deadline，不能用裸 context.Background()")
}

func TestResolveConvergedInstallationID_LoadFailureLogsIdentityReject(t *testing.T) {
	var buf bytes.Buffer
	prevLog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prevLog) })

	injectOutboundDeviceProfileError(t, fmt.Errorf("identity_reject: simulated load failure"))
	account := newTestOAuthAccount(99, nil)
	require.Empty(t, resolveConvergedInstallationIDFromProfile(context.Background(), account))
	require.Contains(t, buf.String(), "identity_reject")
}

func TestResolveConvergedInstallationID_DifferentAccounts(t *testing.T) {
	accountA := newTestOAuthAccount(1, nil)
	accountB := newTestOAuthAccount(2, nil)
	profileA := validOpenAIDeviceProfile(1)
	profileA.InstallationID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	profileB := validOpenAIDeviceProfile(2)
	profileB.InstallationID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(&mapDeviceProfileRepo{
		profiles: map[int64]*AccountDeviceProfile{1: profileA, 2: profileB},
	}))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	a := resolveConvergedInstallationIDFromProfile(context.Background(), accountA)
	b := resolveConvergedInstallationIDFromProfile(context.Background(), accountB)
	assert.NotEqual(t, a, b)
	assert.Equal(t, profileA.InstallationID, a)
	assert.Equal(t, profileB.InstallationID, b)
}

func TestResolveConvergedInstallationID_DifferentSeeds(t *testing.T) {
	account := newTestOAuthAccount(1, nil)
	a := resolveConvergedInstallationID(account, testCodexFingerprintSeed)
	b := resolveConvergedInstallationID(account, "22222222-2222-4222-8222-222222222222")
	assert.NotEqual(t, a, b)
}

func TestResolveConvergedInstallationID_UsesProfileInstallationID(t *testing.T) {
	profile := validOpenAIDeviceProfile(7)
	profile.InstallationID = "11111111-1111-4111-8111-111111111111"
	profile.DeviceID = "22222222-2222-4222-8222-222222222222"
	injectOutboundDeviceProfile(t, 7, profile)

	account := newTestOAuthAccount(7, map[string]any{
		codexFingerprintModeExtraKey: "device",
		"openai_device_id":           "extra-device-id",
	})
	require.Equal(t, profile.InstallationID, resolveConvergedInstallationIDFromProfile(context.Background(), account))

	ids := resolveCodexFingerprintIDsFromRequest(account, nil)
	require.NotNil(t, ids)
	require.Equal(t, profile.InstallationID, ids.installationID)
}

func TestResolveConvergedInstallationID_LoadFailureDoesNotMint(t *testing.T) {
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(nil)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	account := newTestOAuthAccount(42, map[string]any{"openai_device_id": "extra-device-id"})
	minted := deriveStableUUIDv4(fmt.Sprintf("sub2api:codex-install-id:v1:%d", account.ID))

	got := resolveConvergedInstallationIDFromProfile(context.Background(), account)
	require.Empty(t, got)
	require.NotEqual(t, minted, got)
	require.NotEqual(t, "extra-device-id", got)
	require.Nil(t, resolveCodexFingerprintIDsFromRequest(account, nil))
}

func TestResolveConvergedInstallationID_ValidateFailureDoesNotMint(t *testing.T) {
	injectOutboundDeviceProfileError(t, fmt.Errorf("identity_reject: simulated load failure"))

	account := newTestOAuthAccount(42, nil)
	minted := deriveStableUUIDv4(fmt.Sprintf("sub2api:codex-install-id:v1:%d", account.ID))

	got := resolveConvergedInstallationIDFromProfile(context.Background(), account)
	require.Empty(t, got)
	require.NotEqual(t, minted, got)
	require.Nil(t, resolveCodexFingerprintIDsFromRequest(account, nil))
}

// --- resolveConvergedThreadID ---

func TestResolveConvergedThreadID_PerClientSession(t *testing.T) {
	a := resolveConvergedThreadID(testSessionNSA, "session-aaa")
	b := resolveConvergedThreadID(testSessionNSA, "session-bbb")
	assert.NotEqual(t, a, b, "不同客户端 session 应得到不同 thread_id")

	seedA := resolveConvergedThreadID(testCodexFingerprintSeed, "session-aaa")
	seedB := resolveConvergedThreadID(testCodexFingerprintSeed, "session-bbb")
	assert.NotEqual(t, seedA, seedB, "不同客户端 session 应得到不同 thread_id")
}

func TestResolveConvergedThreadID_Deterministic(t *testing.T) {
	a := resolveConvergedThreadID(testSessionNSA, "session-aaa")
	b := resolveConvergedThreadID(testSessionNSA, "session-aaa")
	assert.Equal(t, a, b, "同一客户端 session 应得到相同 thread_id")

	seedA := resolveConvergedThreadID(testCodexFingerprintSeed, "session-aaa")
	seedB := resolveConvergedThreadID(testCodexFingerprintSeed, "session-aaa")
	assert.Equal(t, seedA, seedB, "同一客户端 session 应得到相同 thread_id")
}

func TestResolveConvergedThreadID_EmptySession(t *testing.T) {
	assert.Equal(t, "", resolveConvergedThreadID(testSessionNSA, ""))
	assert.Equal(t, "", resolveConvergedThreadID(testCodexFingerprintSeed, ""))
}

// --- off 模式：resolveCodexFingerprintIDsFromRequest 返回 nil ---

func TestResolveCodexFingerprintIDsFromRequest_ExplicitOff(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "off"})
	ids := resolveCodexFingerprintIDsFromRequest(account, nil)
	assert.Nil(t, ids, "显式 off 模式应返回 nil")
}

// 未显式配置的存量账号不得被收敛（#5610）：默认返回 nil，出站身份保持
// v0.1.175 之前的客户端原值。
func TestResolveCodexFingerprintIDsFromRequest_DefaultIsOff(t *testing.T) {
	account := newTestOAuthAccount(1, nil)
	injectProfileForAccount(t, account)
	assert.Nil(t, resolveCodexFingerprintIDsFromRequest(account, nil), "无 extra 应视为 off，即使已有设备档案")
}

// 管理员显式 opt-in 的账号行为不变。
func TestResolveCodexFingerprintIDsFromRequest_ExplicitOptInHonored(t *testing.T) {
	for _, mode := range []string{"device", "session", "full"} {
		t.Run(mode, func(t *testing.T) {
			account := newTestOAuthAccount(50101, map[string]any{codexFingerprintModeExtraKey: mode})
			ids := resolveCodexFingerprintIDsFromRequest(account, nil)
			require.NotNil(t, ids, "显式配置必须生效")
			assert.Equal(t, codexFingerprintMode(mode), ids.mode)
			assert.NotEmpty(t, ids.installationID)
		})
	}
}

func TestResolveCodexFingerprintIDsFromRequest_EnabledModesRequireValidSeed(t *testing.T) {
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(nil)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })
	for _, tt := range []struct {
		name  string
		extra map[string]any
	}{
		{name: "missing", extra: map[string]any{codexFingerprintModeExtraKey: "device"}},
		{name: "missing with device override", extra: map[string]any{codexFingerprintModeExtraKey: "device", "openai_device_id": "real-device"}},
		{name: "blank", extra: map[string]any{codexFingerprintModeExtraKey: "session", codexFingerprintSeedExtraKey: ""}},
		{name: "uppercase", extra: map[string]any{codexFingerprintModeExtraKey: "full", codexFingerprintSeedExtraKey: "11111111-1111-4111-8111-AAAAAAAAAAAA"}},
		{name: "nil uuid", extra: map[string]any{codexFingerprintModeExtraKey: "device", codexFingerprintSeedExtraKey: "00000000-0000-0000-0000-000000000000"}},
		{name: "non string", extra: map[string]any{codexFingerprintModeExtraKey: "session", codexFingerprintSeedExtraKey: 123}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: tt.extra}
			require.Nil(t, resolveCodexFingerprintIDsFromRequest(account, nil))
		})
	}
}

func TestResolveCodexFingerprintIDs_MintsOnceWhenNoProfileExists(t *testing.T) {
	account := newTestOAuthAccount(8801, map[string]any{codexFingerprintModeExtraKey: "session"})
	seed, ok := codexFingerprintSeed(account.Extra)
	require.True(t, ok)
	wantInstall := resolveConvergedInstallationID(account, seed)
	wantSession := resolveConvergedSessionID(seed)

	repo := &mintingDeviceProfileRepo{mapDeviceProfileRepo: mapDeviceProfileRepo{
		profiles: map[int64]*AccountDeviceProfile{},
	}}
	svc := NewAccountDeviceService(repo)
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(svc)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	ids := resolveCodexFingerprintIDsWithContext(context.Background(), account, "client-session", codexFingerprintSession)
	require.NotNil(t, ids)
	require.Equal(t, wantInstall, ids.installationID)
	require.Equal(t, wantSession, ids.sessionID)

	existing, err := svc.GetIfExists(context.Background(), account)
	require.NoError(t, err)
	require.NotNil(t, existing, "first resolve must mint so later GetIfExists sees the same profile")
	require.Equal(t, wantInstall, existing.InstallationID)
	require.Equal(t, existing.InstallationID, ids.installationID)
	require.Equal(t, fingerprintSeedSessionNamespace(seed), existing.SessionNamespace)
	require.Equal(t, wantSession, resolveConvergedSessionID(codexFingerprintFinishKey(account, existing)))

	ids2 := resolveCodexFingerprintIDsWithContext(context.Background(), account, "client-session", codexFingerprintSession)
	require.NotNil(t, ids2)
	require.Equal(t, ids.installationID, ids2.installationID)
	require.Equal(t, ids.sessionID, ids2.sessionID)

	created, err := svc.GetOrCreate(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, ids.installationID, created.InstallationID)
	require.Equal(t, existing.SessionNamespace, created.SessionNamespace)
}

func TestResolveCodexFingerprintIDs_SessionModeUsesDeriveSessionIDs(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "session"})
	profile := injectProfileForAccount(t, account)
	wantSession, _, _, err := DeriveSessionIDs(profile.SessionNamespace, "")
	require.NoError(t, err)
	wantThread, _, _, err := DeriveSessionIDs(profile.SessionNamespace, "client-session-aaa")
	require.NoError(t, err)

	clientHeaders := http.Header{}
	clientHeaders.Set("session-id", "client-session-aaa")
	ids := resolveCodexFingerprintIDsFromRequest(account, clientHeaders)
	require.NotNil(t, ids)
	require.Equal(t, wantSession, ids.sessionID)
	require.Equal(t, wantThread, ids.threadID)
	require.Equal(t, wantThread+":0", ids.windowID)
	require.NotEqual(t, deriveStableUUIDv4(fmt.Sprintf("sub2api:codex-session-id:v1:%d", account.ID)), ids.sessionID)
	require.NotEqual(t, deriveStableUUIDv4(fmt.Sprintf("sub2api:codex-thread-id:v1:%d:%s", account.ID, "client-session-aaa")), ids.threadID)
}

func TestResolveCodexFingerprintIDs_FailoverUsesTargetNamespace(t *testing.T) {
	accountA := newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "session"})
	accountB := newTestOAuthAccount(2, map[string]any{codexFingerprintModeExtraKey: "session"})
	profileA := validOpenAIDeviceProfile(accountA.ID)
	profileB := validOpenAIDeviceProfile(accountB.ID)
	profileB.SessionNamespace = "fedcba9876543210fedcba9876543210"
	prev := OutboundDeviceProfileService()
	repo := &mapDeviceProfileRepo{profiles: map[int64]*AccountDeviceProfile{
		accountA.ID: profileA,
		accountB.ID: profileB,
	}}
	SetOutboundDeviceProfileService(NewAccountDeviceService(repo))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	headers := http.Header{}
	headers.Set("session-id", "shared-anchor")
	idsA := resolveCodexFingerprintIDsFromRequest(accountA, headers)
	idsB := resolveCodexFingerprintIDsFromRequest(accountB, headers)
	require.NotNil(t, idsA)
	require.NotNil(t, idsB)
	require.NotEqual(t, idsA.sessionID, idsB.sessionID)
	require.NotEqual(t, idsA.threadID, idsB.threadID)
	require.NotEqual(t, idsA.windowID, idsB.windowID)

	wantSessionB, _, _, err := DeriveSessionIDs(profileB.SessionNamespace, "")
	require.NoError(t, err)
	wantThreadB, _, _, err := DeriveSessionIDs(profileB.SessionNamespace, "shared-anchor")
	require.NoError(t, err)
	require.Equal(t, wantSessionB, idsB.sessionID)
	require.Equal(t, wantThreadB, idsB.threadID)
	require.Equal(t, wantThreadB+":0", idsB.windowID)
}

// --- applyCodexFingerprintHeaders: off 模式 ---

func TestApplyCodexFingerprintHeaders_OffMode(t *testing.T) {
	h := http.Header{}
	h.Set("x-codex-installation-id", "original-install-id")
	h.Set("x-codex-window-id", "original-window-id")

	applyCodexFingerprintHeaders(h, nil)

	assert.Equal(t, "original-install-id", h.Get("x-codex-installation-id"), "nil ids 不改写")
	assert.Equal(t, "original-window-id", h.Get("x-codex-window-id"), "nil ids 不改写")
}

// --- applyCodexFingerprintHeaders: device 模式 ---

func TestApplyCodexFingerprintHeaders_DeviceMode(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{
		codexFingerprintModeExtraKey: "device",
		"openai_device_id":           "extra-device-id",
	})
	profile := injectProfileForAccount(t, account)
	require.NotEqual(t, "extra-device-id", profile.InstallationID)
	turnMetadata := `{"installation_id":"user-install","session_id":"user-session","sandbox":"seccomp"}`
	h := http.Header{}
	h.Set("x-codex-installation-id", "user-install")
	h.Set("x-codex-window-id", "user-window:0")
	h.Set("x-codex-turn-metadata", turnMetadata)

	ids := resolveCodexFingerprintIDsFromRequest(account, nil)
	applyCodexFingerprintHeaders(h, ids)

	assert.Equal(t, profile.InstallationID, h.Get("x-codex-installation-id"), "installation_id 应收敛到档案")
	assert.NotEqual(t, "extra-device-id", h.Get("x-codex-installation-id"))
	assert.Equal(t, "user-window:0", h.Get("x-codex-window-id"), "device 模式不改写 window_id")

	var meta map[string]any
	require.NoError(t, json.Unmarshal([]byte(h.Get("x-codex-turn-metadata")), &meta))
	assert.Equal(t, profile.InstallationID, meta["installation_id"])
	assert.Equal(t, "user-session", meta["session_id"], "device 模式不改写 session_id")
	assert.Equal(t, "seccomp", meta["sandbox"], "非指纹字段保留原样")
}

// --- applyCodexFingerprintHeaders: session 模式 ---

func TestApplyCodexFingerprintHeaders_SessionMode(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{
		codexFingerprintModeExtraKey: "session",
	})
	injectProfileForAccount(t, account)
	clientHeaders := http.Header{}
	clientHeaders.Set("session-id", "client-session-aaa")

	turnMetadata := `{"installation_id":"user-install","session_id":"user-session","thread_id":"user-thread","turn_id":"user-turn","window_id":"user-thread:0","sandbox":"seccomp","thread_source":"user"}`
	h := http.Header{}
	h.Set("x-codex-installation-id", "user-install")
	h.Set("x-codex-window-id", "user-thread:0")
	h.Set("x-codex-turn-metadata", turnMetadata)
	h.Set("x-client-request-id", "user-thread")

	ids := resolveCodexFingerprintIDsFromRequest(account, clientHeaders)
	applyCodexFingerprintHeaders(h, ids)

	convergedInstall := resolveConvergedInstallationIDFromProfile(context.Background(), account)
	convergedSession := resolveConvergedSessionID(testSessionNSA)
	convergedThread := resolveConvergedThreadID(testSessionNSA, "client-session-aaa")

	assert.Equal(t, convergedInstall, h.Get("x-codex-installation-id"))
	assert.Equal(t, convergedSession, h.Get("session-id"))
	assert.Equal(t, convergedSession, h.Get("session_id"), "下划线形式也应被改写")
	assert.Equal(t, convergedThread, h.Get("thread-id"))
	assert.Equal(t, convergedThread, h.Get("x-client-request-id"))
	assert.Equal(t, convergedThread+":0", h.Get("x-codex-window-id"))

	var meta map[string]any
	require.NoError(t, json.Unmarshal([]byte(h.Get("x-codex-turn-metadata")), &meta))
	assert.Equal(t, convergedInstall, meta["installation_id"])
	assert.Equal(t, convergedSession, meta["session_id"])
	assert.Equal(t, convergedThread, meta["thread_id"])
	assert.NotEqual(t, "user-turn", meta["turn_id"], "turn_id 应被新生成的值替换")
	assert.Equal(t, "seccomp", meta["sandbox"], "sandbox 保留原样")
	assert.Equal(t, "user", meta["thread_source"], "thread_source 保留原样")
}

// --- session 模式：不同客户端得到不同 thread ---

func TestApplyCodexFingerprintHeaders_SessionMode_DifferentClients(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{
		codexFingerprintModeExtraKey: "session",
	})
	injectProfileForAccount(t, account)

	makeTurnMeta := func() string {
		return `{"installation_id":"x","session_id":"x","thread_id":"x","turn_id":"x","window_id":"x:0"}`
	}

	clientA := http.Header{}
	clientA.Set("session-id", "client-A")
	idsA := resolveCodexFingerprintIDsFromRequest(account, clientA)
	hA := http.Header{}
	hA.Set("x-codex-turn-metadata", makeTurnMeta())
	applyCodexFingerprintHeaders(hA, idsA)

	clientB := http.Header{}
	clientB.Set("session-id", "client-B")
	idsB := resolveCodexFingerprintIDsFromRequest(account, clientB)
	hB := http.Header{}
	hB.Set("x-codex-turn-metadata", makeTurnMeta())
	applyCodexFingerprintHeaders(hB, idsB)

	assert.Equal(t, hA.Get("session-id"), hB.Get("session-id"), "session_id 应相同")
	assert.NotEqual(t, hA.Get("thread-id"), hB.Get("thread-id"), "不同客户端 thread_id 应不同")
	assert.NotEqual(t, hA.Get("x-codex-window-id"), hB.Get("x-codex-window-id"), "不同客户端 window_id 应不同")
	assert.Equal(t, hA.Get("x-codex-installation-id"), hB.Get("x-codex-installation-id"))
}

// --- full 模式 ---

func TestApplyCodexFingerprintHeaders_FullMode(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{
		codexFingerprintModeExtraKey: "full",
	})
	injectProfileForAccount(t, account)
	convergedSession := resolveConvergedSessionID(testSessionNSA)

	clientA := http.Header{}
	clientA.Set("session-id", "client-A")
	idsA := resolveCodexFingerprintIDsFromRequest(account, clientA)
	hA := http.Header{}
	hA.Set("x-codex-turn-metadata", `{"installation_id":"x","session_id":"x","thread_id":"x","turn_id":"x","window_id":"x:0"}`)
	applyCodexFingerprintHeaders(hA, idsA)

	clientB := http.Header{}
	clientB.Set("session-id", "client-B")
	idsB := resolveCodexFingerprintIDsFromRequest(account, clientB)
	hB := http.Header{}
	hB.Set("x-codex-turn-metadata", `{"installation_id":"x","session_id":"x","thread_id":"x","turn_id":"x","window_id":"x:0"}`)
	applyCodexFingerprintHeaders(hB, idsB)

	assert.Equal(t, hA.Get("thread-id"), hB.Get("thread-id"), "full 模式 thread_id 应相同")
	assert.Equal(t, convergedSession, hA.Get("thread-id"), "full 模式 thread_id 应等于 session_id")
	assert.Equal(t, hA.Get("x-codex-window-id"), hB.Get("x-codex-window-id"), "full 模式 window_id 应相同")
}

// --- H1 修复验证：头和体的 turn_id 一致性 ---

func TestFingerprintIDs_HeaderAndBody_TurnID_Consistent(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{
		codexFingerprintModeExtraKey: "session",
	})
	injectProfileForAccount(t, account)
	clientHeaders := http.Header{}
	clientHeaders.Set("session-id", "client-session-xyz")

	ids := resolveCodexFingerprintIDsFromRequest(account, clientHeaders)
	require.NotNil(t, ids)

	// 头改写
	h := http.Header{}
	h.Set("x-codex-turn-metadata", `{"installation_id":"x","session_id":"x","thread_id":"x","turn_id":"x","window_id":"x:0"}`)
	applyCodexFingerprintHeaders(h, ids)

	// 体改写（使用同一份 ids）
	reqBody := map[string]any{
		"client_metadata": map[string]any{
			"x-codex-installation-id": "x",
			"session_id":              "x",
			"turn_id":                 "x",
			"x-codex-turn-metadata":   `{"installation_id":"x","session_id":"x","thread_id":"x","turn_id":"x","window_id":"x:0"}`,
		},
	}
	applyCodexFingerprintClientMetadata(reqBody, ids)

	// 从头 turn-metadata JSON 提取 turn_id
	var headerMeta map[string]any
	require.NoError(t, json.Unmarshal([]byte(h.Get("x-codex-turn-metadata")), &headerMeta))
	headerTurnID, ok := headerMeta["turn_id"].(string)
	require.True(t, ok, "头 turn-metadata 应包含 string 类型的 turn_id")

	// 从体 client_metadata 提取 turn_id
	cm, ok := reqBody["client_metadata"].(map[string]any)
	require.True(t, ok, "请求体应包含 client_metadata")
	bodyTurnID, ok := cm["turn_id"].(string)
	require.True(t, ok, "体 client_metadata 应包含 string 类型的 turn_id")

	// 从体内嵌 turn-metadata JSON 提取 turn_id
	embeddedRaw, ok := cm["x-codex-turn-metadata"].(string)
	require.True(t, ok, "体 client_metadata 应包含 x-codex-turn-metadata 字符串")
	var bodyMeta map[string]any
	require.NoError(t, json.Unmarshal([]byte(embeddedRaw), &bodyMeta))
	bodyEmbeddedTurnID, ok := bodyMeta["turn_id"].(string)
	require.True(t, ok, "体内嵌 turn-metadata 应包含 string 类型的 turn_id")

	assert.Equal(t, headerTurnID, bodyTurnID, "头和体的 turn_id 必须一致")
	assert.Equal(t, headerTurnID, bodyEmbeddedTurnID, "头和体内嵌 turn-metadata 的 turn_id 必须一致")
	assert.Equal(t, ids.turnID, headerTurnID, "所有 turn_id 都应来自同一份 ids")
	assert.Equal(t, headerMeta["turn_started_at_unix_ms"], bodyMeta["turn_started_at_unix_ms"], "头和体的 timestamp 必须一致")
	assert.Equal(t, float64(ids.turnStartedAtUnixMs), headerMeta["turn_started_at_unix_ms"])
}

func TestFingerprintIDs_MalformedEmbeddedMetadataRebuiltConsistently(t *testing.T) {
	account := newTestOAuthAccount(50102, map[string]any{codexFingerprintModeExtraKey: "session"})
	clientHeaders := make(http.Header)
	clientHeaders.Set("session-id", "client-session-malformed")
	ids := resolveCodexFingerprintIDsFromRequest(account, clientHeaders)
	require.NotNil(t, ids)

	h := make(http.Header)
	h.Set("x-codex-turn-metadata", "{malformed")
	applyCodexFingerprintHeaders(h, ids)

	reqBody := map[string]any{
		"client_metadata": map[string]any{
			"session_id":            "client-session-malformed",
			"x-codex-turn-metadata": "[malformed",
		},
	}
	require.True(t, applyCodexFingerprintClientMetadata(reqBody, ids))

	var headerMeta map[string]any
	require.NoError(t, json.Unmarshal([]byte(h.Get("x-codex-turn-metadata")), &headerMeta))
	clientMetadata, ok := reqBody["client_metadata"].(map[string]any)
	require.True(t, ok)
	bodyRaw, ok := clientMetadata["x-codex-turn-metadata"].(string)
	require.True(t, ok)
	var bodyMeta map[string]any
	require.NoError(t, json.Unmarshal([]byte(bodyRaw), &bodyMeta))

	for _, key := range []string{"installation_id", "session_id", "thread_id", "turn_id", "window_id", "turn_started_at_unix_ms"} {
		assert.Equal(t, headerMeta[key], bodyMeta[key], "rebuilt metadata field %s must match", key)
	}
}

// --- applyCodexFingerprintClientMetadata ---

func TestApplyCodexFingerprintClientMetadata_OffMode(t *testing.T) {
	reqBody := map[string]any{
		"client_metadata": map[string]any{
			"x-codex-installation-id": "original",
		},
	}
	modified := applyCodexFingerprintClientMetadata(reqBody, nil)
	assert.False(t, modified, "nil ids 不改写")
}

func TestApplyCodexFingerprintClientMetadata_DeviceMode(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{
		codexFingerprintModeExtraKey: "device",
		"openai_device_id":           "extra-device-id",
	})
	profile := injectProfileForAccount(t, account)
	require.NotEqual(t, "extra-device-id", profile.InstallationID)
	ids := resolveCodexFingerprintIDsFromRequest(account, nil)
	require.NotNil(t, ids)

	embeddedMeta := `{"installation_id":"x","session_id":"user-session","sandbox":"seccomp"}`
	reqBody := map[string]any{
		"client_metadata": map[string]any{
			"x-codex-installation-id": "original-install",
			"session_id":              "user-session",
			"x-codex-turn-metadata":   embeddedMeta,
		},
	}

	modified := applyCodexFingerprintClientMetadata(reqBody, ids)
	require.True(t, modified)

	cm, ok := reqBody["client_metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, profile.InstallationID, cm["x-codex-installation-id"])
	assert.NotEqual(t, "extra-device-id", cm["x-codex-installation-id"])
	assert.Equal(t, "user-session", cm["session_id"], "device 模式不改 session_id")

	turnMetaStr, ok := cm["x-codex-turn-metadata"].(string)
	require.True(t, ok)
	var meta map[string]any
	require.NoError(t, json.Unmarshal([]byte(turnMetaStr), &meta))
	assert.Equal(t, profile.InstallationID, meta["installation_id"])
	assert.Equal(t, "seccomp", meta["sandbox"], "非指纹字段保留原样")
}

func TestApplyCodexFingerprintClientMetadata_SessionMode(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{
		codexFingerprintModeExtraKey: "session",
	})
	injectProfileForAccount(t, account)
	clientHeaders := http.Header{}
	clientHeaders.Set("session-id", "client-session-aaa")

	ids := resolveCodexFingerprintIDsFromRequest(account, clientHeaders)
	require.NotNil(t, ids)

	embeddedMeta := `{"installation_id":"x","session_id":"x","thread_id":"x","turn_id":"x","window_id":"x:0","sandbox":"seccomp"}`
	reqBody := map[string]any{
		"client_metadata": map[string]any{
			"x-codex-installation-id": "original-install",
			"session_id":              "original-session",
			"x-codex-turn-metadata":   embeddedMeta,
		},
	}

	modified := applyCodexFingerprintClientMetadata(reqBody, ids)
	require.True(t, modified)

	cm, ok := reqBody["client_metadata"].(map[string]any)
	require.True(t, ok)
	convergedInstall := resolveConvergedInstallationIDFromProfile(context.Background(), account)
	convergedSession := resolveConvergedSessionID(testSessionNSA)
	convergedThread := resolveConvergedThreadID(testSessionNSA, "client-session-aaa")

	assert.Equal(t, convergedInstall, cm["x-codex-installation-id"])
	assert.Equal(t, convergedSession, cm["session_id"])
	assert.Equal(t, convergedThread, cm["thread_id"])
	assert.Equal(t, convergedThread+":0", cm["x-codex-window-id"])

	turnMetaStr, ok := cm["x-codex-turn-metadata"].(string)
	require.True(t, ok)
	var meta map[string]any
	require.NoError(t, json.Unmarshal([]byte(turnMetaStr), &meta))
	assert.Equal(t, convergedInstall, meta["installation_id"])
	assert.Equal(t, convergedSession, meta["session_id"])
	assert.Equal(t, "seccomp", meta["sandbox"], "非指纹字段保留原样")
}

func TestApplyCodexFingerprintClientMetadata_FullMode(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{
		codexFingerprintModeExtraKey: "full",
	})
	injectProfileForAccount(t, account)
	clientHeaders := http.Header{}
	clientHeaders.Set("session-id", "any-client")

	ids := resolveCodexFingerprintIDsFromRequest(account, clientHeaders)
	require.NotNil(t, ids)

	reqBody := map[string]any{
		"client_metadata": map[string]any{
			"session_id":            "x",
			"thread_id":             "x",
			"x-codex-turn-metadata": `{"installation_id":"x","session_id":"x","thread_id":"x","turn_id":"x","window_id":"x:0"}`,
		},
	}

	modified := applyCodexFingerprintClientMetadata(reqBody, ids)
	require.True(t, modified)

	cm, ok := reqBody["client_metadata"].(map[string]any)
	require.True(t, ok)
	convergedSession := resolveConvergedSessionID(testSessionNSA)

	assert.Equal(t, convergedSession, cm["session_id"])
	assert.Equal(t, convergedSession, cm["thread_id"], "full 模式 thread_id 应等于 session_id")
}

// --- extractClientSessionID ---

func TestExtractClientSessionID(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		expected string
	}{
		{"连字符形式优先", func() http.Header {
			h := http.Header{}
			h.Set("session-id", "hyphen-form")
			h.Set("session_id", "underscore-form")
			return h
		}(), "hyphen-form"},
		{"回退到下划线形式", func() http.Header {
			h := http.Header{}
			h.Set("session_id", "underscore-form")
			return h
		}(), "underscore-form"},
		{"都没有", http.Header{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractClientSessionID(tt.headers))
		})
	}
}

// --- 透传路径：raw 字节版 client_metadata 改写 ---

// rawVsMapClientMetadata 用同一份 ids 分别跑 map 版与 raw 字节版，
// 返回两侧最终的 client_metadata 解码结果。
func rawVsMapClientMetadata(t *testing.T, body []byte, ids *codexFingerprintIDs) (map[string]any, map[string]any) {
	t.Helper()

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	applyCodexFingerprintClientMetadata(decoded, ids)
	mapCM, _ := decoded["client_metadata"].(map[string]any)

	rawBody, changed, err := applyCodexFingerprintClientMetadataRaw(body, ids)
	require.NoError(t, err)
	require.True(t, changed)
	var rawDecoded map[string]any
	require.NoError(t, json.Unmarshal(rawBody, &rawDecoded))
	rawCM, _ := rawDecoded["client_metadata"].(map[string]any)
	return mapCM, rawCM
}

func cloneCodexFingerprintIDsForTest(ids *codexFingerprintIDs) *codexFingerprintIDs {
	if ids == nil {
		return nil
	}
	cloned := *ids
	cloned.originalBodySessionID = ""
	cloned.originalBodySessionIDCaptured = false
	return &cloned
}

func applyMapAndRawFingerprintBodiesForTest(t *testing.T, body []byte, ids *codexFingerprintIDs) (map[string]any, map[string]any) {
	t.Helper()

	mapIDs := cloneCodexFingerprintIDsForTest(ids)
	rawIDs := cloneCodexFingerprintIDsForTest(ids)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	applyCodexFingerprintClientMetadata(decoded, mapIDs)

	rawBody, _, err := applyCodexFingerprintClientMetadataRaw(body, rawIDs)
	require.NoError(t, err)
	var rawDecoded map[string]any
	require.NoError(t, json.Unmarshal(rawBody, &rawDecoded))
	return decoded, rawDecoded
}

func TestApplyCodexFingerprintPromptCacheKey_MapRawEquivalence(t *testing.T) {
	for _, mode := range []codexFingerprintMode{codexFingerprintSession, codexFingerprintFull} {
		t.Run(string(mode)+"/default", func(t *testing.T) {
			account := newTestOAuthAccount(4300, map[string]any{codexFingerprintModeExtraKey: string(mode)})
			ids := resolveCodexFingerprintIDs(account, "header-session", mode)
			require.NotNil(t, ids)

			body := []byte(`{"model":"gpt-5.6-sol","prompt_cache_key":"body-session","client_metadata":{"session_id":" body-session ","trace":"keep"},"input":[]}`)
			mapBody, rawBody := applyMapAndRawFingerprintBodiesForTest(t, body, ids)

			require.Equal(t, mapBody["prompt_cache_key"], rawBody["prompt_cache_key"])
			require.Equal(t, ids.sessionID, mapBody["prompt_cache_key"])
			mapCM, _ := mapBody["client_metadata"].(map[string]any)
			rawCM, _ := rawBody["client_metadata"].(map[string]any)
			require.Equal(t, ids.sessionID, mapCM["session_id"])
			require.Equal(t, mapCM["session_id"], rawCM["session_id"])
			require.Equal(t, "keep", rawCM["trace"])
		})
	}

	t.Run("explicit override", func(t *testing.T) {
		account := newTestOAuthAccount(4301, map[string]any{codexFingerprintModeExtraKey: "session"})
		ids := resolveCodexFingerprintIDs(account, "header-session", codexFingerprintSession)
		require.NotNil(t, ids)

		body := []byte(`{"model":"gpt-5.6-sol","prompt_cache_key":"explicit-cache","client_metadata":{"session_id":"body-session"},"input":[]}`)
		mapBody, rawBody := applyMapAndRawFingerprintBodiesForTest(t, body, ids)

		require.Equal(t, "explicit-cache", mapBody["prompt_cache_key"])
		require.Equal(t, "explicit-cache", rawBody["prompt_cache_key"])
		mapCM, _ := mapBody["client_metadata"].(map[string]any)
		rawCM, _ := rawBody["client_metadata"].(map[string]any)
		require.Equal(t, ids.sessionID, mapCM["session_id"])
		require.Equal(t, ids.sessionID, rawCM["session_id"])
	})
}

func TestApplyCodexFingerprintPromptCacheKey_Negatives(t *testing.T) {
	sessionAccount := newTestOAuthAccount(4310, map[string]any{codexFingerprintModeExtraKey: "session"})
	sessionIDs := resolveCodexFingerprintIDs(sessionAccount, "header-session", codexFingerprintSession)
	require.NotNil(t, sessionIDs)
	deviceAccount := newTestOAuthAccount(4311, map[string]any{codexFingerprintModeExtraKey: "device"})
	deviceIDs := resolveCodexFingerprintIDs(deviceAccount, "header-session", codexFingerprintDevice)
	require.NotNil(t, deviceIDs)

	tests := []struct {
		name          string
		body          []byte
		ids           *codexFingerprintIDs
		wantExists    bool
		wantCacheKey  any
		wantRawString string
	}{
		{
			name:       "missing key is not injected",
			body:       []byte(`{"client_metadata":{"session_id":"body-session"}}`),
			ids:        sessionIDs,
			wantExists: false,
		},
		{
			name:         "empty key preserved",
			body:         []byte(`{"prompt_cache_key":"","client_metadata":{"session_id":"body-session"}}`),
			ids:          sessionIDs,
			wantExists:   true,
			wantCacheKey: "",
		},
		{
			name:         "whitespace-different key is an explicit override",
			body:         []byte(`{"prompt_cache_key":" body-session ","client_metadata":{"session_id":"body-session"}}`),
			ids:          sessionIDs,
			wantExists:   true,
			wantCacheKey: " body-session ",
		},
		{
			name:         "non-string key preserved",
			body:         []byte(`{"prompt_cache_key":123,"client_metadata":{"session_id":"body-session"}}`),
			ids:          sessionIDs,
			wantExists:   true,
			wantCacheKey: float64(123),
		},
		{
			name:         "missing source metadata preserves key",
			body:         []byte(`{"prompt_cache_key":"body-session"}`),
			ids:          sessionIDs,
			wantExists:   true,
			wantCacheKey: "body-session",
		},
		{
			name:         "non-string source session preserves key",
			body:         []byte(`{"prompt_cache_key":"123","client_metadata":{"session_id":123}}`),
			ids:          sessionIDs,
			wantExists:   true,
			wantCacheKey: "123",
		},
		{
			name:         "non-object source metadata preserves key",
			body:         []byte(`{"prompt_cache_key":"body-session","client_metadata":"bad"}`),
			ids:          sessionIDs,
			wantExists:   true,
			wantCacheKey: "body-session",
		},
		{
			name:         "device mode preserves key",
			body:         []byte(`{"prompt_cache_key":"body-session","client_metadata":{"session_id":"body-session"}}`),
			ids:          deviceIDs,
			wantExists:   true,
			wantCacheKey: "body-session",
		},
		{
			name:          "off mode preserves body",
			body:          []byte(`{"prompt_cache_key":"body-session","client_metadata":{"session_id":"body-session"}}`),
			ids:           nil,
			wantExists:    true,
			wantCacheKey:  "body-session",
			wantRawString: `{"prompt_cache_key":"body-session","client_metadata":{"session_id":"body-session"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mapBody map[string]any
			require.NoError(t, json.Unmarshal(tt.body, &mapBody))
			changedMap := applyCodexFingerprintClientMetadata(mapBody, cloneCodexFingerprintIDsForTest(tt.ids))

			rawBody, changedRaw, err := applyCodexFingerprintClientMetadataRaw(tt.body, cloneCodexFingerprintIDsForTest(tt.ids))
			require.NoError(t, err)
			if tt.ids == nil {
				require.False(t, changedMap)
				require.False(t, changedRaw)
				require.JSONEq(t, tt.wantRawString, string(rawBody))
				return
			}
			require.True(t, changedMap)
			require.True(t, changedRaw)

			rawDecoded := map[string]any{}
			require.NoError(t, json.Unmarshal(rawBody, &rawDecoded))
			_, mapExists := mapBody["prompt_cache_key"]
			_, rawExists := rawDecoded["prompt_cache_key"]
			require.Equal(t, tt.wantExists, mapExists)
			require.Equal(t, tt.wantExists, rawExists)
			if tt.wantExists {
				require.Equal(t, tt.wantCacheKey, mapBody["prompt_cache_key"])
				require.Equal(t, tt.wantCacheKey, rawDecoded["prompt_cache_key"])
			}
		})
	}
}

func TestApplyCodexFingerprintClientMetadataRaw_MatchesMapVariant(t *testing.T) {
	embedded := `{\"installation_id\":\"real-install\",\"session_id\":\"real-session\",\"sandbox\":\"seatbelt\"}`
	bodies := map[string]string{
		"no_client_metadata": `{"model":"gpt-5.6-sol","input":[],"stream":true}`,
		"object_with_extras": `{"model":"gpt-5.6-sol","client_metadata":{"session_id":"client-session","traceparent":"00-abc-def-01","x-codex-turn-metadata":"` + embedded + `"},"stream":true}`,
		"non_object_value":   `{"model":"gpt-5.6-sol","client_metadata":"bogus","stream":true}`,
	}
	for _, mode := range []codexFingerprintMode{codexFingerprintDevice, codexFingerprintSession, codexFingerprintFull} {
		account := newTestOAuthAccount(4242, map[string]any{codexFingerprintModeExtraKey: string(mode)})
		ids := resolveCodexFingerprintIDs(account, "client-sess-raw", mode)
		require.NotNil(t, ids)
		for name, body := range bodies {
			t.Run(string(mode)+"/"+name, func(t *testing.T) {
				mapCM, rawCM := rawVsMapClientMetadata(t, []byte(body), ids)
				assert.Equal(t, mapCM, rawCM, "raw 字节版与 map 版的 client_metadata 结果必须逐点一致")
			})
		}
	}
}

func TestApplyCodexFingerprintClientMetadataRaw_PreservesUnrelatedFields(t *testing.T) {
	account := newTestOAuthAccount(4243, map[string]any{codexFingerprintModeExtraKey: "session"})
	ids := resolveCodexFingerprintIDs(account, "client-sess-preserve", codexFingerprintSession)
	require.NotNil(t, ids)

	body := []byte(`{"model":"gpt-5.6-sol","input":[{"type":"message","role":"user","content":"hi"}],"stream":true,"prompt_cache_key":"pck-1"}`)
	out, changed, err := applyCodexFingerprintClientMetadataRaw(body, ids)
	require.NoError(t, err)
	require.True(t, changed)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(out, &decoded))
	assert.Equal(t, "gpt-5.6-sol", decoded["model"])
	assert.Equal(t, "pck-1", decoded["prompt_cache_key"])
	assert.Equal(t, true, decoded["stream"])
	cm, _ := decoded["client_metadata"].(map[string]any)
	require.NotNil(t, cm)
	assert.Equal(t, ids.sessionID, cm["session_id"])
	assert.Equal(t, ids.turnID, cm["turn_id"])
}

func TestApplyCodexFingerprintClientMetadataRaw_Noop(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol"}`)
	out, changed, err := applyCodexFingerprintClientMetadataRaw(body, nil)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, body, out)

	out, changed, err = applyCodexFingerprintClientMetadataRaw(nil, &codexFingerprintIDs{mode: codexFingerprintSession, installationID: "x"})
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Nil(t, out)
}

// --- context 暂存与出站头应用（透传/非透传共用 seam）---

func newFingerprintStageTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c
}

func TestStageCodexFingerprintIDs_NilOverwritesPreviousAccount(t *testing.T) {
	c := newFingerprintStageTestContext(t)
	accountA := newTestOAuthAccount(1001, map[string]any{codexFingerprintModeExtraKey: "session"})
	idsA := resolveCodexFingerprintIDs(accountA, "sess-x", codexFingerprintSession)
	require.NotNil(t, idsA)
	stageCodexFingerprintIDs(c, idsA)

	// failover 切到 off 模式账号：无条件覆写为 nil，上一账号 IDs 不得残留
	stageCodexFingerprintIDs(c, nil)

	h := http.Header{}
	h.Set("session_id", "isolated-session")
	accountB := newTestOAuthAccount(1002, map[string]any{"codex_fingerprint_mode": "off"})
	applyStagedCodexFingerprintHeaders(c, accountB, h)
	assert.Equal(t, "isolated-session", h.Get("session_id"), "off 账号不得应用上一账号的收敛 ID")
	assert.Empty(t, h.Get("x-codex-installation-id"))
}

func TestApplyStagedCodexFingerprintRejectsDifferentOAuthAccount(t *testing.T) {
	c := newFingerprintStageTestContext(t)
	accountA := newTestOAuthAccount(1003, map[string]any{codexFingerprintModeExtraKey: "session"})
	idsA := resolveCodexFingerprintIDs(accountA, "sess-a", codexFingerprintSession)
	require.NotNil(t, idsA)
	stageCodexFingerprintIDs(c, idsA)

	accountB := newTestOAuthAccount(1004, map[string]any{codexFingerprintModeExtraKey: "session"})
	h := make(http.Header)
	h.Set("session-id", "account-b-session")
	applyStagedCodexFingerprintHeaders(c, accountB, h)
	assert.Equal(t, "account-b-session", h.Get("session-id"))
	assert.Empty(t, h.Get("x-codex-installation-id"))

	body := map[string]any{"client_metadata": map[string]any{"session_id": "account-b-session"}}
	assert.False(t, applyStagedCodexFingerprintClientMetadata(c, accountB, body))
	clientMetadata, ok := body["client_metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "account-b-session", clientMetadata["session_id"])
}

func TestApplyStagedCodexFingerprintHeaders_SkipsNonOAuthAccount(t *testing.T) {
	c := newFingerprintStageTestContext(t)
	oauthIDs := resolveCodexFingerprintIDs(newTestOAuthAccount(1003, map[string]any{codexFingerprintModeExtraKey: "session"}), "sess-y", codexFingerprintSession)
	require.NotNil(t, oauthIDs)
	stageCodexFingerprintIDs(c, oauthIDs)

	h := http.Header{}
	apiKeyAccount := &Account{ID: 1004, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	applyStagedCodexFingerprintHeaders(c, apiKeyAccount, h)
	assert.Empty(t, h.Get("x-codex-installation-id"), "stale 收敛 ID 不得应用到非 OAuth 账号")
}

func TestBuildUpstreamRequestOpenAIPassthrough_AppliesStagedFingerprint(t *testing.T) {
	svc := &OpenAIGatewayService{}
	// 收敛是显式 opt-in（#5610）：显式开启后验证透传路径的出站头收敛。
	account := newTestOAuthAccount(2001, map[string]any{
		"openai_oauth_passthrough": true,
		"codex_fingerprint_mode":   "session",
	})

	c := newFingerprintStageTestContext(t)
	c.Request.Header.Set("session_id", "real-client-session")
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.144.1 (Ubuntu 22.4.0; x86_64) xterm-256color")
	c.Request.Header.Set("originator", "codex_cli_rs")
	c.Request.Header.Set("x-codex-turn-metadata", `{"installation_id":"real-install","session_id":"real-session","sandbox":"seatbelt"}`)

	// 复刻 forwardOpenAIPassthrough 的解析+暂存 seam（默认 session 模式）
	ids := resolveCodexFingerprintIDsFromRequest(account, c.Request.Header)
	require.NotNil(t, ids)
	stageCodexFingerprintIDs(c, ids)

	body := []byte(`{"model":"gpt-5.6-sol","input":[],"stream":true}`)
	req, err := svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, account, body, "test-token")
	require.NoError(t, err)

	assert.Equal(t, ids.sessionID, req.Header.Get("session_id"), "session 模式下出站 session_id 应为账号级收敛值")
	assert.Equal(t, ids.installationID, req.Header.Get("x-codex-installation-id"))
	assert.Equal(t, ids.windowID, req.Header.Get("x-codex-window-id"))
	assert.Equal(t, ids.threadID, req.Header.Get("x-client-request-id"))
	turnMetadata := req.Header.Get("x-codex-turn-metadata")
	require.NotEmpty(t, turnMetadata)
	assert.Contains(t, turnMetadata, ids.sessionID, "turn-metadata JSON 中的 session_id 应被收敛")
	assert.Contains(t, turnMetadata, `"sandbox":"seatbelt"`, "turn-metadata 未指定字段应原样保留")
}

func TestBuildUpstreamRequestOpenAIPassthrough_OffModeKeepsIsolatedSession(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := newTestOAuthAccount(2002, map[string]any{
		"openai_oauth_passthrough": true,
		"codex_fingerprint_mode":   "off",
	})

	c := newFingerprintStageTestContext(t)
	c.Request.Header.Set("session_id", "real-client-session")
	c.Request.Header.Set("originator", "codex_cli_rs")

	ids := resolveCodexFingerprintIDsFromRequest(account, c.Request.Header)
	require.Nil(t, ids)
	stageCodexFingerprintIDs(c, ids)

	body := []byte(`{"model":"gpt-5.6-sol","input":[],"stream":true}`)
	req, err := svc.buildUpstreamRequestOpenAIPassthrough(context.Background(), c, account, body, "test-token")
	require.NoError(t, err)

	assert.NotEmpty(t, req.Header.Get("session_id"))
	assert.NotEqual(t, resolveConvergedSessionID(testCodexFingerprintSeed), req.Header.Get("session_id"), "off 模式不得收敛 session_id")
	assert.Empty(t, req.Header.Get("x-codex-window-id"))
}

func TestApplyCodexFingerprintClientMetadataRaw_NonObjectBodyUntouched(t *testing.T) {
	account := newTestOAuthAccount(4244, map[string]any{codexFingerprintModeExtraKey: "session"})
	ids := resolveCodexFingerprintIDs(account, "client-sess-nonobj", codexFingerprintSession)
	require.NotNil(t, ids)

	for _, body := range []string{`[1,2,3]`, `"plain string"`, `not json at all`} {
		out, changed, err := applyCodexFingerprintClientMetadataRaw([]byte(body), ids)
		require.NoError(t, err)
		assert.False(t, changed, "非 JSON 对象 body 不应被改写: %s", body)
		assert.Equal(t, []byte(body), out)
	}
}
