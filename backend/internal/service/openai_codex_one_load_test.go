//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildUpstreamRequestReusesForwardDeviceProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := leftoverOAuthAccount(1920, nil)
	profile := leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-cli/1.2.3", "mid-one-load")
	installLeftoverOutboundProfile(t, profile)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("session_id", "client-session")

	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, account, c.Request.Header)
	require.Equal(t, 1, leftoverSharedRepo.getCount(account.ID))

	svc := &OpenAIGatewayService{}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", true, "cache-key", true)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, 1, leftoverSharedRepo.getCount(account.ID), "buildUpstreamRequest must reuse the Forward-loaded profile for session headers and UA")
	require.Equal(t, resolveCodexOutboundIdentityFromProfile(profile, "").userAgent, req.Header.Get("User-Agent"))
}

func TestBuildUpstreamRequestLoadsOnceWhenForwardSkipsFingerprint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{
		ID:       1921,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{codexFingerprintModeExtraKey: "off"},
		Credentials: map[string]any{
			"access_token": "oauth-token",
		},
	}
	profile := leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-cli/1.2.3", "mid-one-load-off")
	installLeftoverOutboundProfile(t, profile)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"model":"gpt-5.4"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, account, c.Request.Header)
	require.Equal(t, 0, leftoverSharedRepo.getCount(account.ID), "fingerprint off must skip the Forward profile load")

	svc := &OpenAIGatewayService{}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", true, "cache-key", true)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, 1, leftoverSharedRepo.getCount(account.ID), "buildUpstreamRequest must load the profile once for session headers and UA")
	require.Equal(t, resolveCodexOutboundIdentityFromProfile(profile, "").userAgent, req.Header.Get("User-Agent"))
	require.Equal(t, openaiOutboundSessionIDFromProfile(profile, 0, "cache-key"), req.Header.Get("session_id"))
}

func TestBuildUpstreamRequestDoesNotReuseOtherAccountProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accountA := leftoverOAuthAccount(1922, nil)
	accountB := leftoverOAuthAccount(1923, map[string]any{codexFingerprintModeExtraKey: "off"})
	profileA := leftoverValidProfile(accountA.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex_cli_rs/0.146.0 (Windows NT 10.0; Win64; x64)", "mid-a")
	profileB := leftoverValidProfile(accountB.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex_cli_rs/0.146.0 (X11; Linux x86_64)", "mid-b")
	profileB.SessionNamespace = "fedcba9876543210fedcba9876543210"
	installLeftoverOutboundProfile(t, profileA)
	installLeftoverOutboundProfile(t, profileB)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("session_id", "client-session")

	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, accountA, c.Request.Header)
	require.Equal(t, 1, leftoverSharedRepo.getCount(accountA.ID))
	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, accountB, c.Request.Header)
	require.Equal(t, 0, leftoverSharedRepo.getCount(accountB.ID), "fingerprint off must not load during Forward identity")

	svc := &OpenAIGatewayService{}
	body := []byte(`{"model":"gpt-5.4"}`)
	reqB, err := svc.buildUpstreamRequest(context.Background(), c, accountB, body, "oauth-token-b", true, "cache-key", true)
	require.NoError(t, err)
	require.NotNil(t, reqB)
	require.Equal(t, 1, leftoverSharedRepo.getCount(accountB.ID), "failover to another account must load that account's profile")
	require.Equal(t, resolveCodexOutboundIdentityFromProfile(profileB, "").userAgent, reqB.Header.Get("User-Agent"))
	require.NotEqual(t, resolveCodexOutboundIdentityFromProfile(profileA, "").userAgent, reqB.Header.Get("User-Agent"))
	require.Equal(t, openaiOutboundSessionIDFromProfile(profileB, 0, "cache-key"), reqB.Header.Get("session_id"))
}

func TestBuildUpstreamRequestCachesFailedProfileLoad(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := leftoverOAuthAccount(1924, map[string]any{codexFingerprintModeExtraKey: "off"})
	installLeftoverOutboundProfileError(t, account.ID, fmt.Errorf("identity_reject: profile unavailable"))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, account, c.Request.Header)
	require.Equal(t, 0, leftoverSharedRepo.getCount(account.ID))

	svc := &OpenAIGatewayService{}
	body := []byte(`{"model":"gpt-5.4"}`)
	req1, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", true, "cache-key", true)
	require.NoError(t, err)
	require.NotNil(t, req1)
	require.Equal(t, 1, leftoverSharedRepo.getCount(account.ID))

	req2, err := svc.buildUpstreamRequest(context.Background(), c, account, body, "oauth-token", true, "cache-key", true)
	require.NoError(t, err)
	require.NotNil(t, req2)
	require.Equal(t, 1, leftoverSharedRepo.getCount(account.ID), "a failed load must be reused for later rebuilds on the same account")
}

func TestBuildUpstreamRequestAppliesParentFingerprintForShadowAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	parent := leftoverOAuthAccount(1930, nil)
	shadow := leftoverOAuthAccount(1931, nil)
	shadow.ParentAccountID = &parent.ID
	profile := leftoverValidProfile(parent.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex_cli_rs/0.146.0 (Windows NT 10.0; Win64; x64)", "mid-parent")
	installLeftoverOutboundProfile(t, profile)
	installLeftoverAccountLookup(t, parent)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("session_id", "client-session")

	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, shadow, c.Request.Header)
	require.Equal(t, 1, leftoverSharedRepo.getCount(parent.ID))

	svc := &OpenAIGatewayService{accountRepo: &leftoverShadowAccountRepo{parent: parent}}
	req, err := svc.buildUpstreamRequest(context.Background(), c, shadow, []byte(`{"model":"gpt-5.4"}`), "oauth-token", true, "cache-key", true)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, 1, leftoverSharedRepo.getCount(parent.ID), "shadow outbound must reuse the parent device profile")
	require.Equal(t, profile.InstallationID, getHeaderRaw(req.Header, "x-codex-installation-id"))
	require.Equal(t, resolveCodexOutboundIdentityFromProfile(profile, "").userAgent, req.Header.Get("User-Agent"))
}

func TestBuildUpstreamRequestDoesNotReloadAfterForwardLoadFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := leftoverOAuthAccount(1932, nil)
	installLeftoverOutboundProfileError(t, account.ID, fmt.Errorf("identity_reject: profile unavailable"))

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, account, c.Request.Header)
	require.Equal(t, 1, leftoverSharedRepo.getCount(account.ID))

	svc := &OpenAIGatewayService{}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"gpt-5.4"}`), "oauth-token", true, "cache-key", true)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, 1, leftoverSharedRepo.getCount(account.ID), "Forward load failure must be reused by buildUpstreamRequest")
}

func TestBuildUpstreamRequestClearsStaleFingerprintIDsAfterForwardLoadFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := leftoverOAuthAccount(1933, nil)
	profile := leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex_cli_rs/0.146.0 (Windows NT 10.0; Win64; x64)", "mid-stale")
	installLeftoverOutboundProfile(t, profile)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("session_id", "client-session")

	require.NotNil(t, applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, account, c.Request.Header))
	leftoverSharedRepo.putErr(account.ID, fmt.Errorf("identity_reject: profile unavailable"))
	t.Cleanup(func() { leftoverSharedRepo.clear(account.ID) })

	seedIDs := applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, account, c.Request.Header)
	require.NotNil(t, seedIDs, "load failure must seed-fallback instead of sending the client identity unmodified")
	require.NotEqual(t, profile.InstallationID, seedIDs.installationID)

	svc := &OpenAIGatewayService{}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"gpt-5.4"}`), "oauth-token", true, "cache-key", true)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.NotEqual(t, profile.InstallationID, getHeaderRaw(req.Header, "x-codex-installation-id"), "stale profile fingerprint IDs must not survive a later Forward load failure")
	require.NotEqual(t, resolveCodexOutboundIdentityFromProfile(profile, "").userAgent, req.Header.Get("User-Agent"))
}

func leftoverOAuthAccount(id int64, extra map[string]any) *Account {
	if extra == nil {
		extra = map[string]any{
			codexFingerprintModeExtraKey: "session",
			codexFingerprintSeedExtraKey: testCodexFingerprintSeed,
		}
	}
	return &Account{
		ID:       id,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra:    extra,
		Credentials: map[string]any{
			"access_token": "oauth-token",
		},
	}
}

type leftoverShadowAccountRepo struct {
	AccountRepository
	parent *Account
}

func (r *leftoverShadowAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if r != nil && r.parent != nil && r.parent.ID == id {
		return r.parent, nil
	}
	return nil, nil
}

type leftoverAccountLookup struct {
	byID map[int64]*Account
}

func (l *leftoverAccountLookup) GetByID(_ context.Context, id int64) (*Account, error) {
	if l == nil || l.byID == nil {
		return nil, nil
	}
	return l.byID[id], nil
}

func installLeftoverAccountLookup(t *testing.T, accounts ...*Account) {
	t.Helper()
	byID := make(map[int64]*Account, len(accounts))
	for _, account := range accounts {
		if account != nil {
			byID[account.ID] = account
		}
	}
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(leftoverSharedRepo).WithAccountLookup(&leftoverAccountLookup{byID: byID}))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })
}
