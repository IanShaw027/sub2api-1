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
	require.Equal(t, isolateOpenAISessionID(3, "sess"), openaiOutboundSessionID(context.Background(), account, 3, "sess"))
}
