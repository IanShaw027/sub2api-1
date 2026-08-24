//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeriveOpenAIOutboundSessionIDUsesSessionNamespace(t *testing.T) {
	account := &Account{ID: 1811, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-cli/1.2.3", "mid-oa-sess"))

	got := deriveOpenAIOutboundSessionID(context.Background(), account, 99, "cache-key-123")
	isolated := isolateOpenAISessionID(99, "cache-key-123")
	want, _, _, err := DeriveSessionIDs(leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "", "").SessionNamespace, isolated)
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.NotEqual(t, isolated, got)
	require.NotEqual(t, generateSessionUUID(isolated), got)
}

func TestDeriveOpenAIOutboundSessionIDFailoverUsesTargetNamespace(t *testing.T) {
	accountA := &Account{ID: 1812, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	accountB := &Account{ID: 1813, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	profileA := leftoverValidProfile(accountA.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-cli/1.2.3", "mid-a")
	profileB := leftoverValidProfile(accountB.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-cli/1.2.3", "mid-b")
	profileB.SessionNamespace = testSessionNSB
	installLeftoverOutboundProfile(t, profileA)
	installLeftoverOutboundProfile(t, profileB)

	gotA := deriveOpenAIOutboundSessionID(context.Background(), accountA, 7, "same-anchor")
	gotB := deriveOpenAIOutboundSessionID(context.Background(), accountB, 7, "same-anchor")
	require.NotEqual(t, gotA, gotB)
	isolated := isolateOpenAISessionID(7, "same-anchor")
	wantA, _, _, err := DeriveSessionIDs(profileA.SessionNamespace, isolated)
	require.NoError(t, err)
	wantB, _, _, err := DeriveSessionIDs(profileB.SessionNamespace, isolated)
	require.NoError(t, err)
	require.Equal(t, wantA, gotA)
	require.Equal(t, wantB, gotB)
}

func TestDeriveOpenAIOutboundSessionIDMintsBaselineWhenProfileMissing(t *testing.T) {
	account := &Account{ID: 1814, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	t.Cleanup(func() { leftoverSharedRepo.clear(account.ID) })
	got := deriveOpenAIOutboundSessionID(context.Background(), account, 3, "sess")
	require.NotEmpty(t, got)
	profile, err := LoadOutboundDeviceProfile(context.Background(), account)
	require.NoError(t, err)
	require.NotNil(t, profile)
	isolated := isolateOpenAISessionID(3, "sess")
	want, _, _, err := DeriveSessionIDs(profile.SessionNamespace, isolated)
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.NotEqual(t, isolated, got)
}

func TestDeriveOpenAIOutboundSessionIDFallsBackWhenProfileLoadFails(t *testing.T) {
	account := &Account{ID: 1815, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))
	require.Empty(t, deriveOpenAIOutboundSessionID(context.Background(), account, 3, "sess"))
	require.Equal(t, isolateOpenAIUpstreamSessionID(3, account, "sess"), openaiOutboundSessionID(context.Background(), account, 3, "sess"))
	require.Equal(t, generateSessionUUID(isolateOpenAIUpstreamSessionID(3, account, "sess")), openaiOutboundSessionUUID(context.Background(), account, 3, "sess"))
}

func TestOpenAIOutboundSessionIDFallsBackToAccountNamespaceWhenProfileLoadFails(t *testing.T) {
	account := &Account{
		ID:          1820,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"chatgpt_account_id": "acct-failover-a"},
	}
	installLeftoverOutboundProfileError(t, account.ID, errors.New("identity_reject: profile store down"))

	gotID := openaiOutboundSessionID(context.Background(), account, 3, "sess")
	gotUUID := openaiOutboundSessionUUID(context.Background(), account, 3, "sess")
	wantIsolated := isolateOpenAIUpstreamSessionID(3, account, "sess")
	require.Equal(t, wantIsolated, gotID)
	require.Equal(t, generateSessionUUID(wantIsolated), gotUUID)
	require.NotEqual(t, isolateOpenAISessionID(3, "sess"), gotID)
	require.NotEqual(t, generateSessionUUID(isolateOpenAISessionID(3, "sess")), gotUUID)
}

func TestDeriveOpenAIOutboundSessionIDDifferentAPIKeysStillDiffer(t *testing.T) {
	account := &Account{ID: 1816, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-cli/1.2.3", "mid-oa-keys"))

	gotA := deriveOpenAIOutboundSessionID(context.Background(), account, 1, "same-anchor")
	gotB := deriveOpenAIOutboundSessionID(context.Background(), account, 2, "same-anchor")
	require.NotEmpty(t, gotA)
	require.NotEmpty(t, gotB)
	require.NotEqual(t, gotA, gotB)
}

func TestDeriveOpenAIOutboundSessionIDSkipsNonOpenAIOAuthAccounts(t *testing.T) {
	account := &Account{ID: 1817, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	t.Cleanup(func() { leftoverSharedRepo.clear(account.ID) })

	require.Empty(t, deriveOpenAIOutboundSessionID(context.Background(), account, 3, "sess"))
	require.Equal(t, isolateOpenAISessionID(3, "sess"), openaiOutboundSessionID(context.Background(), account, 3, "sess"))
	require.Equal(t, generateSessionUUID(isolateOpenAISessionID(3, "sess")), openaiOutboundSessionUUID(context.Background(), account, 3, "sess"))
	require.Equal(t, 0, leftoverSharedRepo.getCount(account.ID))
}

func TestOpenAIOutboundSessionUUIDUsesSessionNamespace(t *testing.T) {
	account := &Account{ID: 1818, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	installLeftoverOutboundProfile(t, leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-cli/1.2.3", "mid-oa-uuid"))

	got := openaiOutboundSessionUUID(context.Background(), account, 99, "cache-key-123")
	want := deriveOpenAIOutboundSessionID(context.Background(), account, 99, "cache-key-123")
	require.Equal(t, want, got)
	require.NotEqual(t, generateSessionUUID(isolateOpenAISessionID(99, "cache-key-123")), got)
}

func TestOpenAIOutboundSessionPairLoadsProfileOnce(t *testing.T) {
	account := &Account{ID: 1819, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	profile := leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-cli/1.2.3", "mid-oa-pair")
	installLeftoverOutboundProfile(t, profile)

	sessionID, conversationID := openaiOutboundSessionPair(context.Background(), account, 9, "sess-a", "conv-b")
	require.Equal(t, 1, leftoverSharedRepo.getCount(account.ID))

	wantSession, _, _, err := DeriveSessionIDs(profile.SessionNamespace, isolateOpenAISessionID(9, "sess-a"))
	require.NoError(t, err)
	wantConversation, _, _, err := DeriveSessionIDs(profile.SessionNamespace, isolateOpenAISessionID(9, "conv-b"))
	require.NoError(t, err)
	require.Equal(t, wantSession, sessionID)
	require.Equal(t, wantConversation, conversationID)
}
