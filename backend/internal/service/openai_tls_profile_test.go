package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/stretchr/testify/require"
)

func TestAccount_IsTLSFingerprintEnabled_AllowsExistingAnthropicAndKiroBehavior(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "anthropic oauth enabled",
			account: &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: map[string]any{
				"enable_tls_fingerprint": true,
			}},
			want: true,
		},
		{
			name: "anthropic setup token enabled",
			account: &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken, Extra: map[string]any{
				"enable_tls_fingerprint": true,
			}},
			want: true,
		},
		{
			name: "kiro oauth enabled",
			account: &Account{Platform: PlatformKiro, Type: AccountTypeOAuth, Extra: map[string]any{
				"enable_tls_fingerprint": true,
			}},
			want: true,
		},
		{
			name: "anthropic api key still blocked",
			account: &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Extra: map[string]any{
				"enable_tls_fingerprint": true,
			}},
			want: false,
		},
		{
			name: "kiro api key still blocked",
			account: &Account{Platform: PlatformKiro, Type: AccountTypeAPIKey, Extra: map[string]any{
				"enable_tls_fingerprint": true,
			}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.account.IsTLSFingerprintEnabled())
		})
	}
}

func TestAccount_IsOpenAITLSFingerprintEnabled_AllowsOnlyOpenAIOAuthAndAPIKeyWhenEnabled(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{
			name: "openai oauth enabled",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{
				"enable_tls_fingerprint": true,
			}},
			want: true,
		},
		{
			name: "openai api key enabled",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{
				"enable_tls_fingerprint": true,
			}},
			want: true,
		},
		{
			name:    "openai oauth disabled when missing flag",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{}},
			want:    false,
		},
		{
			name: "openai api key disabled when flag false",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{
				"enable_tls_fingerprint": false,
			}},
			want: false,
		},
		{
			name: "openai setup token blocked",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeSetupToken, Extra: map[string]any{
				"enable_tls_fingerprint": true,
			}},
			want: false,
		},
		{
			name: "non openai blocked",
			account: &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: map[string]any{
				"enable_tls_fingerprint": true,
			}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.account.IsOpenAITLSFingerprintEnabled())
		})
	}
}

func TestTLSFingerprintProfileService_ResolveTLSProfile_OpenAIUsesExistingProfileID(t *testing.T) {
	svc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			42: {ID: 42, Name: "OpenAI TLS Profile"},
		},
	}

	profile := svc.ResolveTLSProfile(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": int64(42),
		},
	})

	require.NotNil(t, profile)
	require.Equal(t, "OpenAI TLS Profile", profile.Name)
}

func TestTLSFingerprintProfileService_ResolveTLSProfile_OpenAIDisabledReturnsNil(t *testing.T) {
	svc := &TLSFingerprintProfileService{}

	profile := svc.ResolveTLSProfile(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			"enable_tls_fingerprint": false,
		},
	})

	require.Nil(t, profile)
}

func TestTLSFingerprintProfileService_ResolveTLSProfile_RandomModeDoesNotUseOtherPlatformProfile(t *testing.T) {
	svc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			7: {ID: 7, Platform: PlatformAnthropic, Name: "Anthropic TLS Profile"},
		},
	}

	profile := svc.ResolveTLSProfile(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_profile_id": int64(-1),
		},
	})

	require.NotNil(t, profile)
	require.Equal(t, "Built-in Default (Node.js 24.x)", profile.Name)
}
