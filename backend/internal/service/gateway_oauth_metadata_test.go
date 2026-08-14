//go:build unit

package service

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildOAuthMetadataUserID_FallbackWithoutAccountUUID(t *testing.T) {
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(nil)
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	svc := &GatewayService{}

	parsed := &ParsedRequest{
		Model:          "claude-sonnet-4-5",
		Stream:         true,
		MetadataUserID: "",
	}

	account := &Account{
		ID:    123,
		Type:  AccountTypeOAuth,
		Extra: map[string]any{}, // intentionally missing account_uuid / claude_user_id
	}

	fp := &Fingerprint{ClientID: "deadbeef"}

	got := svc.buildOAuthMetadataUserID(context.Background(), parsed, account, fp)
	require.Empty(t, got, "load failure must fail closed and not mint a device id")
}

func TestBuildOAuthMetadataUserID_UsesAccountUUIDWhenPresent(t *testing.T) {
	profile := testAnthropicDeviceProfile()
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(&stubOutboundDeviceRepo{profile: profile}))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	svc := &GatewayService{}

	parsed := &ParsedRequest{
		Model:          "claude-sonnet-4-5",
		Stream:         true,
		MetadataUserID: "",
	}

	account := &Account{
		ID:   123,
		Type: AccountTypeOAuth,
		Extra: map[string]any{
			"account_uuid":      "acc-uuid",
			"claude_user_id":    "clientid123",
			"anthropic_user_id": "",
		},
	}

	got := svc.buildOAuthMetadataUserID(context.Background(), parsed, account, nil)
	require.NotEmpty(t, got)
	require.NotContains(t, got, "acc-uuid")
	require.Contains(t, got, testGatewayAccountUUID)
	require.Contains(t, got, profile.DeviceID)
	re := regexp.MustCompile(`^user_` + regexp.QuoteMeta(profile.DeviceID) + `_account_` + regexp.QuoteMeta(testGatewayAccountUUID) + `_session_[a-f0-9-]{36}$`)
	require.True(t, re.MatchString(got), "unexpected user_id format: %s", got)
}

// TestBuildOAuthMetadataUserID_SessionIDStableAcrossTurns 验证伪装路径合成的
// metadata.user_id 在同一会话多轮请求间保持不变（session_id 稳定），贴近真实 Claude Code
// 进程级稳定的 session。账号 / 指纹 / UA 版本均相同，唯一可能变化的就是 session_id，
// 因此直接比较完整 user_id 字符串即可判定 session_id 是否稳定。
func TestBuildOAuthMetadataUserID_SessionIDStableAcrossTurns(t *testing.T) {
	profile := testAnthropicDeviceProfile()
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(&stubOutboundDeviceRepo{profile: profile}))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })

	svc := &GatewayService{}
	account := &Account{ID: 777, Type: AccountTypeOAuth, Extra: map[string]any{"account_uuid": "acc-uuid"}}
	fp := &Fingerprint{ClientID: "clientid777", UserAgent: "claude-cli/2.1.161 (external, cli)"}

	mustParse := func(body string) *ParsedRequest {
		parsed, err := ParseGatewayRequest(NewRequestBodyRef([]byte(body)), PlatformAnthropic)
		require.NoError(t, err)
		return parsed
	}

	round1 := mustParse(`{"model":"claude-sonnet-4-5","system":"sys","messages":[` +
		`{"role":"user","content":"first question"}]}`)
	round2 := mustParse(`{"model":"claude-sonnet-4-5","system":"sys","messages":[` +
		`{"role":"user","content":"first question"},` +
		`{"role":"assistant","content":"answer 1"},` +
		`{"role":"user","content":"second question"}]}`)
	round3 := mustParse(`{"model":"claude-sonnet-4-5","system":"sys","messages":[` +
		`{"role":"user","content":"first question"},` +
		`{"role":"assistant","content":"answer 1"},` +
		`{"role":"user","content":"second question"},` +
		`{"role":"assistant","content":"answer 2"},` +
		`{"role":"user","content":"third question"}]}`)

	id1 := svc.buildOAuthMetadataUserID(context.Background(), round1, account, fp)
	id2 := svc.buildOAuthMetadataUserID(context.Background(), round2, account, fp)
	id3 := svc.buildOAuthMetadataUserID(context.Background(), round3, account, fp)

	require.NotEmpty(t, id1)
	require.Equal(t, id1, id2, "session_id 应随对话增长保持不变")
	require.Equal(t, id2, id3, "session_id 应跨所有轮次保持不变")

	// 不同的首条 user 消息应派生出不同的 session_id（不同会话）。
	other := mustParse(`{"model":"claude-sonnet-4-5","system":"sys","messages":[` +
		`{"role":"user","content":"a completely different opener"}]}`)
	idOther := svc.buildOAuthMetadataUserID(context.Background(), other, account, fp)
	require.NotEqual(t, id1, idOther, "不同首条消息应派生不同 session_id")
}
