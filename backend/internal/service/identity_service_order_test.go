package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type identityCacheStub struct {
	maskedSessionID string
	setMaskedCalls  int
}

func (s *identityCacheStub) GetFingerprint(_ context.Context, _ int64) (*Fingerprint, error) {
	return nil, nil
}
func (s *identityCacheStub) SetFingerprint(_ context.Context, _ int64, _ *Fingerprint) error {
	return nil
}
func (s *identityCacheStub) GetMaskedSessionID(_ context.Context, _ int64) (string, error) {
	return s.maskedSessionID, nil
}
func (s *identityCacheStub) SetMaskedSessionID(_ context.Context, _ int64, sessionID string) error {
	s.setMaskedCalls++
	s.maskedSessionID = sessionID
	return nil
}
func (s *identityCacheStub) GetDeviceProfile(_ context.Context, _ int64) (*AccountDeviceProfile, error) {
	return nil, nil
}
func (s *identityCacheStub) SetDeviceProfile(_ context.Context, _ int64, _ *AccountDeviceProfile) error {
	return nil
}

func TestIdentityService_RewriteUserID_PreservesTopLevelFieldOrder(t *testing.T) {
	cache := &identityCacheStub{}
	svc := NewIdentityService(cache)

	originalUserID := FormatMetadataUserID(
		"d61f76d0730d2b920763648949bad5c79742155c27037fc77ac3f9805cb90169",
		"",
		"7578cf37-aaca-46e4-a45c-71285d9dbb83",
		"2.1.78",
	)
	body := []byte(`{"alpha":1,"messages":[],"metadata":{"user_id":` + strconvQuote(originalUserID) + `},"max_tokens":64000,"thinking":{"type":"adaptive"},"output_config":{"effort":"high"},"stream":true}`)

	result, err := svc.RewriteUserID(body, 123, "acc-uuid", "client-xyz", "claude-cli/2.1.78 (external, cli)")
	require.NoError(t, err)
	resultStr := string(result)

	assertJSONTokenOrder(t, resultStr, `"alpha"`, `"messages"`, `"metadata"`, `"max_tokens"`, `"thinking"`, `"output_config"`, `"stream"`)
	require.NotContains(t, resultStr, originalUserID)
	require.Contains(t, resultStr, `"metadata":{"user_id":"`)
}

func TestIdentityService_RewriteUserIDWithMasking_PreservesTopLevelFieldOrder(t *testing.T) {
	cache := &identityCacheStub{maskedSessionID: "11111111-2222-4333-8444-555555555555"}
	svc := NewIdentityService(cache)

	originalUserID := FormatMetadataUserID(
		"d61f76d0730d2b920763648949bad5c79742155c27037fc77ac3f9805cb90169",
		"",
		"7578cf37-aaca-46e4-a45c-71285d9dbb83",
		"2.1.78",
	)
	body := []byte(`{"alpha":1,"messages":[],"metadata":{"user_id":` + strconvQuote(originalUserID) + `},"max_tokens":64000,"thinking":{"type":"adaptive"},"output_config":{"effort":"high"},"stream":true}`)

	account := &Account{
		ID:       123,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"session_id_masking_enabled": true,
		},
	}

	result, err := svc.RewriteUserIDWithMasking(context.Background(), body, account, "acc-uuid", "client-xyz", "claude-cli/2.1.78 (external, cli)")
	require.NoError(t, err)
	resultStr := string(result)

	assertJSONTokenOrder(t, resultStr, `"alpha"`, `"messages"`, `"metadata"`, `"max_tokens"`, `"thinking"`, `"output_config"`, `"stream"`)
	require.Contains(t, resultStr, cache.maskedSessionID)
	require.True(t, strings.Contains(resultStr, `"metadata":{"user_id":"`))
}

func TestIdentityService_RewriteUserIDWithMasking_DoesNotMintRandomSessionID(t *testing.T) {
	cache := &identityCacheStub{}
	svc := NewIdentityService(cache)

	originalUserID := FormatMetadataUserID(
		"d61f76d0730d2b920763648949bad5c79742155c27037fc77ac3f9805cb90169",
		"",
		"7578cf37-aaca-46e4-a45c-71285d9dbb83",
		"2.1.78",
	)
	body := []byte(`{"metadata":{"user_id":` + strconvQuote(originalUserID) + `}}`)
	account := &Account{
		ID:       123,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"session_id_masking_enabled": true,
		},
	}

	result, err := svc.RewriteUserIDWithMasking(context.Background(), body, account, "acc-uuid", "client-xyz", "claude-cli/2.1.78 (external, cli)")
	require.NoError(t, err)

	parsed := ParseMetadataUserID(gjson.GetBytes(result, "metadata.user_id").String())
	require.NotNil(t, parsed)
	require.Equal(t, generateUUIDFromSeed("123::7578cf37-aaca-46e4-a45c-71285d9dbb83"), parsed.SessionID)
	require.Empty(t, cache.maskedSessionID)
	require.Zero(t, cache.setMaskedCalls)
}

func TestRewriteUserID_UsesGatewayUUIDNotExtraUUID(t *testing.T) {
	svc := NewIdentityService(&identityCacheStub{})
	const extraUUID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const gatewayUUID = "11111111-2222-4333-8444-555555555555"
	const deviceID = "d61f76d0730d2b920763648949bad5c79742155c27037fc77ac3f9805cb90169"
	originalUserID := FormatMetadataUserID(deviceID, extraUUID, "7578cf37-aaca-46e4-a45c-71285d9dbb83", "2.1.22")
	body := []byte(`{"metadata":{"user_id":` + strconvQuote(originalUserID) + `}}`)

	result, err := svc.RewriteUserID(body, 123, gatewayUUID, deviceID, "claude-cli/2.1.22 (external, cli)")
	require.NoError(t, err)
	resultStr := string(result)
	require.NotContains(t, resultStr, extraUUID)
	require.Contains(t, resultStr, gatewayUUID)
	require.Contains(t, resultStr, "user_"+deviceID+"_account_"+gatewayUUID+"_session_")
}

func TestRewriteUserID_EmptyGatewayUUIDSkipsRewrite(t *testing.T) {
	svc := NewIdentityService(&identityCacheStub{})
	const extraUUID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const deviceID = "d61f76d0730d2b920763648949bad5c79742155c27037fc77ac3f9805cb90169"
	originalUserID := FormatMetadataUserID(deviceID, extraUUID, "7578cf37-aaca-46e4-a45c-71285d9dbb83", "2.1.22")
	body := []byte(`{"metadata":{"user_id":` + strconvQuote(originalUserID) + `}}`)

	result, err := svc.RewriteUserID(body, 123, "", deviceID, "claude-cli/2.1.22 (external, cli)")
	require.NoError(t, err)
	require.Equal(t, string(body), string(result))
}

func TestRewriteUserIDWithMasking_DoesNotReadExtraAccountUUID(t *testing.T) {
	svc := NewIdentityService(&identityCacheStub{})
	const extraUUID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	const gatewayUUID = "11111111-2222-4333-8444-555555555555"
	const deviceID = "d61f76d0730d2b920763648949bad5c79742155c27037fc77ac3f9805cb90169"
	originalUserID := FormatMetadataUserID(deviceID, extraUUID, "7578cf37-aaca-46e4-a45c-71285d9dbb83", "2.1.22")
	body := []byte(`{"metadata":{"user_id":` + strconvQuote(originalUserID) + `}}`)
	account := &Account{
		ID:       123,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{"account_uuid": extraUUID},
	}

	result, err := svc.RewriteUserIDWithMasking(context.Background(), body, account, gatewayUUID, deviceID, "claude-cli/2.1.22 (external, cli)")
	require.NoError(t, err)
	resultStr := string(result)
	require.NotContains(t, resultStr, extraUUID)
	require.Contains(t, resultStr, gatewayUUID)
}

func TestIdentityService_GatewayAccountUUIDForRewrite_UsesProfileUUID(t *testing.T) {
	profile := &AccountDeviceProfile{GatewayAccountUUID: "11111111-2222-4333-8444-555555555555"}
	got := GatewayAccountUUIDForRewrite(profile, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	require.Equal(t, "11111111-2222-4333-8444-555555555555", got)
}

func TestIdentityService_GatewayAccountUUIDForRewrite_EmptyProfileDoesNotFallBackToExtra(t *testing.T) {
	require.Empty(t, GatewayAccountUUIDForRewrite(&AccountDeviceProfile{}, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"))
	require.Empty(t, GatewayAccountUUIDForRewrite(nil, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"))
}

func strconvQuote(v string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`) + `"`
}
