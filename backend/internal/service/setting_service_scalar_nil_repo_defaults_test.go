//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingServiceScalarHelpers_NilRepoUseDocumentedDefaults(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{
		Server: config.ServerConfig{
			FrontendURL: "https://app.example.com",
		},
		Default: config.DefaultConfig{
			UserConcurrency: 7,
			UserBalance:     3.5,
		},
	})

	require.False(t, svc.IsEmailVerifyEnabled(context.Background()))
	require.Empty(t, svc.GetRegistrationEmailSuffixWhitelist(context.Background()))
	require.True(t, svc.IsPromoCodeEnabled(context.Background()))
	require.False(t, svc.IsInvitationCodeEnabled(context.Background()))
	require.Equal(t, "[]", svc.GetCustomMenuItemsRaw(context.Background()))
	require.False(t, svc.IsAffiliateEnabled(context.Background()))
	require.InDelta(t, AffiliateRebateRateDefault, svc.GetAffiliateRebateRatePercent(context.Background()), 1e-12)
	require.Equal(t, AffiliateRebateFreezeHoursDefault, svc.GetAffiliateRebateFreezeHours(context.Background()))
	require.Equal(t, AffiliateRebateDurationDaysDefault, svc.GetAffiliateRebateDurationDays(context.Background()))
	require.InDelta(t, AffiliateRebatePerInviteeCapDefault, svc.GetAffiliateRebatePerInviteeCap(context.Background()), 1e-12)
	require.False(t, svc.IsPasswordResetEnabled(context.Background()))
	require.False(t, svc.IsTotpEnabled(context.Background()))
	require.False(t, svc.IsTicketEnabled(context.Background()))
	require.False(t, svc.IsTurnstileEnabled(context.Background()))
	require.Empty(t, svc.GetTurnstileSecretKey(context.Background()))
	require.True(t, svc.IsIdentityPatchEnabled(context.Background()))
	require.Empty(t, svc.GetIdentityPatchPrompt(context.Background()))
	require.False(t, svc.IsModelFallbackEnabled(context.Background()))
	require.Equal(t, "claude-3-5-sonnet-20241022", svc.GetFallbackModel(context.Background(), PlatformAnthropic))
	require.Equal(t, "gpt-4o", svc.GetFallbackModel(context.Background(), PlatformOpenAI))
	require.Equal(t, "gemini-2.5-pro", svc.GetFallbackModel(context.Background(), PlatformGemini))
	require.Equal(t, "gemini-2.5-pro", svc.GetFallbackModel(context.Background(), PlatformAntigravity))
	require.Empty(t, svc.GetFallbackModel(context.Background(), "unknown"))
	require.False(t, svc.IsUngroupedKeySchedulingAllowed(context.Background()))
	require.Equal(t, "https://app.example.com", svc.GetFrontendURL(context.Background()))
	require.Equal(t, "Sub2API", svc.GetSiteName(context.Background()))
	require.Equal(t, 7, svc.GetDefaultConcurrency(context.Background()))
	require.InDelta(t, 3.5, svc.GetDefaultBalance(context.Background()), 1e-12)
	require.Equal(t, 0, svc.GetDefaultUserRPMLimit(context.Background()))
	require.Nil(t, svc.GetDefaultSubscriptions(context.Background()))
}

func TestSettingServiceGetAuthSourceDefaultSettings_NilRepoReturnsEmptyDefaults(t *testing.T) {
	svc := NewSettingService(nil, &config.Config{})

	settings, err := svc.GetAuthSourceDefaultSettings(context.Background())
	require.NoError(t, err)
	require.NotNil(t, settings)
	require.False(t, settings.ForceEmailOnThirdPartySignup)
	require.Equal(t, ProviderDefaultGrantSettings{}, settings.Email)
	require.Equal(t, ProviderDefaultGrantSettings{}, settings.LinuxDo)
	require.Equal(t, ProviderDefaultGrantSettings{}, settings.OIDC)
	require.Equal(t, ProviderDefaultGrantSettings{}, settings.WeChat)
	require.Equal(t, ProviderDefaultGrantSettings{}, settings.GitHub)
	require.Equal(t, ProviderDefaultGrantSettings{}, settings.Google)
	require.Equal(t, ProviderDefaultGrantSettings{}, settings.DingTalk)
}

func TestIsTotpEncryptionKeyConfigured_NilConfigReturnsFalse(t *testing.T) {
	svc := NewSettingService(nil, nil)
	require.False(t, svc.IsTotpEncryptionKeyConfigured())
}
