package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestResolveKiroRequestedModelWithRouting_DefaultBuildUsesPlatformDefaultConfig(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformDefaultAccountModelConfig: `{
				"kiro": {
					"model_mapping": {"claude-sonnet-4-5": "claude-sonnet-4.6"}
				}
			}`,
		},
	}, &config.Config{})
	account := &Account{ID: 110, Platform: PlatformKiro, Type: AccountTypeOAuth}

	model, err := resolveKiroRequestedModelWithRouting(context.Background(), svc, account, "claude-sonnet-4-5")

	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4.6", model)
}
