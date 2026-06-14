package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestResolveGatewayAnthropicForwardModel_DefaultBuildIgnoresPlatformDefaultForOAuthAndServiceAccount(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformDefaultAccountModelConfig: `{
				"anthropic": {
					"model_whitelist": ["claude-sonnet-4-6"],
					"model_mapping": {"claude-3-7-sonnet-20250219": "claude-sonnet-4-6"}
				}
			}`,
		},
	}, &config.Config{})

	tests := []struct {
		accountType string
		wantModel   string
		wantSource  string
	}{
		{accountType: AccountTypeOAuth, wantModel: "claude-3-7-sonnet-20250219", wantSource: ""},
		{accountType: AccountTypeServiceAccount, wantModel: "claude-3-7-sonnet@20250219", wantSource: "vertex"},
		{accountType: AccountTypeSetupToken, wantModel: "claude-3-7-sonnet-20250219", wantSource: ""},
	}
	for _, tt := range tests {
		t.Run(tt.accountType, func(t *testing.T) {
			account := &Account{Platform: PlatformAnthropic, Type: tt.accountType}

			mapped, source := resolveGatewayAnthropicForwardModel(context.Background(), svc, account, "claude-3-7-sonnet-20250219")

			require.Equal(t, tt.wantModel, mapped)
			require.Equal(t, tt.wantSource, source)
		})
	}
}
