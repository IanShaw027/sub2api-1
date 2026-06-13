package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestResolveGatewayAnthropicForwardModel_DefaultBuildUsesPlatformRoutingForOAuthAndServiceAccount(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformModelRoutingConfig: `{
				"anthropic": {
					"model_whitelist": ["claude-sonnet-4-6"],
					"model_mapping": {"claude-3-7-sonnet-20250219": "claude-sonnet-4-6"}
				}
			}`,
		},
	}, &config.Config{})

	for _, accountType := range []string{AccountTypeOAuth, AccountTypeServiceAccount, AccountTypeSetupToken} {
		t.Run(accountType, func(t *testing.T) {
			account := &Account{Platform: PlatformAnthropic, Type: accountType}

			mapped, source := resolveGatewayAnthropicForwardModel(context.Background(), svc, account, "claude-3-7-sonnet-20250219")

			require.Equal(t, "claude-sonnet-4-6", mapped)
			require.Equal(t, "system", source)
		})
	}
}
