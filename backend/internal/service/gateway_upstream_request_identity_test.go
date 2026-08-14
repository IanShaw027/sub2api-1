//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	testExtraAccountUUID     = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	testGatewayAccountUUID   = "11111111-2222-4333-8444-555555555555"
	testOriginalAccountUUID  = "99999999-9999-4999-8999-999999999999"
	testProfileDeviceID      = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testFingerprintClientID  = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	testProfileUserAgent     = "claude-cli/2.1.22 (external, cli)"
	testFingerprintUserAgent = "claude-cli/2.1.221 (external, cli)"
	testOriginalDeviceID     = "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	testOriginalSessionID    = "7578cf37-aaca-46e4-a45c-71285d9dbb83"
)

type stubOutboundDeviceRepo struct {
	profile *AccountDeviceProfile
	err     error
}

func (r *stubOutboundDeviceRepo) GetByAccountID(context.Context, int64) (*AccountDeviceProfile, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.profile, nil
}

func (r *stubOutboundDeviceRepo) InsertBaseline(_ context.Context, p *AccountDeviceProfile) (*AccountDeviceProfile, error) {
	return p, nil
}

func (r *stubOutboundDeviceRepo) UpdateCAS(context.Context, int64, int64, *AccountDeviceProfile) (bool, error) {
	return true, nil
}

func installOutboundDeviceProfile(t *testing.T, profile *AccountDeviceProfile) {
	t.Helper()
	prev := OutboundDeviceProfileService()
	if profile == nil {
		SetOutboundDeviceProfileService(nil)
	} else {
		SetOutboundDeviceProfileService(NewAccountDeviceService(&stubOutboundDeviceRepo{profile: profile}))
	}
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })
}

func testAnthropicDeviceProfile() *AccountDeviceProfile {
	return &AccountDeviceProfile{
		AccountID:          42,
		Revision:           1,
		SchemaVersion:      1,
		Platform:           PlatformAnthropic,
		ClientFamily:       ClientFamilyClaudeCode,
		InstallationID:     "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		DeviceID:           testProfileDeviceID,
		MachineID:          "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee",
		GatewayAccountUUID: testGatewayAccountUUID,
		SessionNamespace:   "0123456789abcdef0123456789abcdef",
		OSFamily:           "linux",
		Arch:               "arm64",
		Runtime:            "node",
		RuntimeVersion:     "v24.3.0",
		ClientVersion:      "2.1.22",
		TransportFamily:    TransportH1,
		LearnedFrom:        LearnedFromBaseline,
		ProfilePayload: map[string]any{
			"user_agent": testProfileUserAgent,
		},
	}
}

func testAnthropicOAuthAccount() *Account {
	return &Account{
		ID:       42,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
		Extra:    map[string]any{"account_uuid": testExtraAccountUUID},
	}
}

func testIdentityGatewayService() *GatewayService {
	return &GatewayService{
		identityService: NewIdentityService(&stubIdentityCache{
			fingerprint: &Fingerprint{
				ClientID:  testFingerprintClientID,
				UserAgent: testFingerprintUserAgent,
			},
		}),
	}
}

func testGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return c
}

func testUserIDBody(accountUUID string) []byte {
	userID := FormatMetadataUserID(testOriginalDeviceID, accountUUID, testOriginalSessionID, "2.1.22")
	return []byte(`{"model":"claude-sonnet-4-6","metadata":{"user_id":` + strconvQuote(userID) + `},"messages":[]}`)
}

func TestApplyClaudeCodeMimicHeaders_DoesNotOverwriteFingerprintIdentity(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)
	setHeaderRaw(req.Header, "User-Agent", "claude-cli/2.1.221 (external, cli)")
	setHeaderRaw(req.Header, "X-Stainless-OS", "macOS")
	setHeaderRaw(req.Header, "X-Stainless-Arch", "x64")

	applyClaudeCodeMimicHeaders(req, true)

	require.Equal(t, "claude-cli/2.1.221 (external, cli)", getHeaderRaw(req.Header, "User-Agent"))
	require.Equal(t, "macOS", getHeaderRaw(req.Header, "X-Stainless-OS"))
	require.Equal(t, "x64", getHeaderRaw(req.Header, "X-Stainless-Arch"))
	require.NotEqual(t, claude.DefaultHeaders["User-Agent"], getHeaderRaw(req.Header, "User-Agent"))
	require.Equal(t, "stream", getHeaderRaw(req.Header, "x-stainless-helper-method"))
	require.NotEmpty(t, getHeaderRaw(req.Header, "x-client-request-id"))
}

func TestBuildUpstreamRequest_RewriteUserIDUsesProfileGatewayUUID(t *testing.T) {
	installOutboundDeviceProfile(t, testAnthropicDeviceProfile())
	svc := testIdentityGatewayService()
	account := testAnthropicOAuthAccount()
	body := testUserIDBody(testExtraAccountUUID)

	_, outBody, err := svc.buildUpstreamRequest(
		context.Background(), testGinContext(), account, body,
		"oauth-tok", "oauth", "claude-sonnet-4-6", false, false,
	)
	require.NoError(t, err)
	got := gjson.GetBytes(outBody, "metadata.user_id").String()
	require.NotContains(t, got, testExtraAccountUUID)
	require.Contains(t, got, testGatewayAccountUUID)
	require.Contains(t, got, testProfileDeviceID)
	require.NotContains(t, got, testFingerprintClientID)
}

func TestBuildUpstreamRequest_LoadFailureDoesNotRewriteWithExtraUUID(t *testing.T) {
	installOutboundDeviceProfile(t, nil)
	svc := testIdentityGatewayService()
	account := testAnthropicOAuthAccount()
	body := testUserIDBody(testOriginalAccountUUID)

	_, outBody, err := svc.buildUpstreamRequest(
		context.Background(), testGinContext(), account, body,
		"oauth-tok", "oauth", "claude-sonnet-4-6", false, false,
	)
	require.NoError(t, err)
	got := gjson.GetBytes(outBody, "metadata.user_id").String()
	require.NotContains(t, got, testExtraAccountUUID)
	require.Contains(t, got, testOriginalAccountUUID)
	require.Equal(t, string(body), string(outBody))
}

func TestBuildCountTokensRequest_RewriteUserIDUsesProfileGatewayUUID(t *testing.T) {
	installOutboundDeviceProfile(t, testAnthropicDeviceProfile())
	svc := testIdentityGatewayService()
	account := testAnthropicOAuthAccount()
	body := testUserIDBody(testExtraAccountUUID)

	_, outBody, err := svc.buildCountTokensRequest(
		context.Background(), testGinContext(), account, body,
		"oauth-tok", "oauth", "claude-sonnet-4-6", false,
	)
	require.NoError(t, err)
	got := gjson.GetBytes(outBody, "metadata.user_id").String()
	require.NotContains(t, got, testExtraAccountUUID)
	require.Contains(t, got, testGatewayAccountUUID)
	require.Contains(t, got, testProfileDeviceID)
	require.NotContains(t, got, testFingerprintClientID)
}

func TestBuildCountTokensRequest_LoadFailureDoesNotRewriteWithExtraUUID(t *testing.T) {
	installOutboundDeviceProfile(t, nil)
	svc := testIdentityGatewayService()
	account := testAnthropicOAuthAccount()
	body := testUserIDBody(testOriginalAccountUUID)

	_, outBody, err := svc.buildCountTokensRequest(
		context.Background(), testGinContext(), account, body,
		"oauth-tok", "oauth", "claude-sonnet-4-6", false,
	)
	require.NoError(t, err)
	got := gjson.GetBytes(outBody, "metadata.user_id").String()
	require.NotContains(t, got, testExtraAccountUUID)
	require.Contains(t, got, testOriginalAccountUUID)
}

func TestBuildOAuthMetadataUserID_UsesProfileGatewayUUID(t *testing.T) {
	installOutboundDeviceProfile(t, testAnthropicDeviceProfile())
	svc := &GatewayService{}
	account := testAnthropicOAuthAccount()
	account.Extra["claude_user_id"] = "should-not-use-extra-device"
	parsed := &ParsedRequest{Model: "claude-sonnet-4-6"}
	fp := &Fingerprint{ClientID: testFingerprintClientID, UserAgent: testFingerprintUserAgent}

	got := svc.buildOAuthMetadataUserID(parsed, account, fp)
	require.NotEmpty(t, got)
	require.NotContains(t, got, testExtraAccountUUID)
	require.Contains(t, got, testGatewayAccountUUID)
	require.Contains(t, got, testProfileDeviceID)
	require.NotContains(t, got, testFingerprintClientID)
	require.NotContains(t, got, "should-not-use-extra-device")
}

func TestBuildOAuthMetadataUserID_LoadFailureDoesNotUseExtraUUID(t *testing.T) {
	installOutboundDeviceProfile(t, nil)
	svc := &GatewayService{}
	account := testAnthropicOAuthAccount()
	account.Extra["claude_user_id"] = "extra-device"
	parsed := &ParsedRequest{Model: "claude-sonnet-4-6"}
	fp := &Fingerprint{ClientID: testFingerprintClientID, UserAgent: testFingerprintUserAgent}

	got := svc.buildOAuthMetadataUserID(parsed, account, fp)
	require.Empty(t, got)
	require.NotContains(t, got, testExtraAccountUUID)
}

func TestBuildOAuthMetadataUserIDFromBody_UsesProfileGatewayUUID(t *testing.T) {
	installOutboundDeviceProfile(t, testAnthropicDeviceProfile())
	svc := &GatewayService{}
	account := testAnthropicOAuthAccount()
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}`)
	fp := &Fingerprint{ClientID: testFingerprintClientID, UserAgent: testFingerprintUserAgent}

	got := svc.buildOAuthMetadataUserIDFromBody(context.Background(), account, fp, body)
	require.NotEmpty(t, got)
	require.NotContains(t, got, testExtraAccountUUID)
	require.Contains(t, got, testGatewayAccountUUID)
	require.Contains(t, got, testProfileDeviceID)
	require.NotContains(t, got, testFingerprintClientID)
}

func TestBuildOAuthMetadataUserIDFromBody_LoadFailureDoesNotUseExtraUUID(t *testing.T) {
	installOutboundDeviceProfile(t, nil)
	svc := &GatewayService{}
	account := testAnthropicOAuthAccount()
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"hi"}]}`)
	fp := &Fingerprint{ClientID: testFingerprintClientID, UserAgent: testFingerprintUserAgent}

	got := svc.buildOAuthMetadataUserIDFromBody(context.Background(), account, fp, body)
	require.Empty(t, got)
	require.NotContains(t, got, testExtraAccountUUID)
}
