//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestResolveEffectiveModelRouting_EmptySystemConfigKeepsAccountBehavior(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{}, &config.Config{})
	account := &Account{Platform: PlatformOpenAI}

	result := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-custom", false)

	require.True(t, result.Supported)
	require.False(t, result.Matched)
	require.Equal(t, "gpt-custom", result.Model)
	require.Equal(t, "none", result.Source)
}

func TestResolveEffectiveModelRouting_UsesSystemConfigWhenAccountHasNoModelRules(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformModelRoutingConfig: `{
				"openai": {
					"model_whitelist": ["gpt-5.4-mini"],
					"model_mapping": {"gpt-4o-mini": "gpt-5.4", "gpt-4o*": "gpt-5.4"}
				}
			}`,
		},
	}, &config.Config{})
	account := &Account{Platform: PlatformOpenAI}

	result := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-4o-mini", false)

	require.True(t, result.Supported)
	require.True(t, result.Matched)
	require.Equal(t, "gpt-5.4", result.Model)
	require.Equal(t, "system", result.Source)

	whitelistResult := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-5.4-mini", false)
	require.True(t, whitelistResult.Supported)
	require.True(t, whitelistResult.Matched)
	require.Equal(t, "gpt-5.4-mini", whitelistResult.Model)
	require.Equal(t, "system", whitelistResult.Source)

	unsupportedResult := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-3.5-turbo", false)
	require.False(t, unsupportedResult.Supported)
	require.False(t, unsupportedResult.Matched)
	require.Equal(t, "gpt-3.5-turbo", unsupportedResult.Model)
}

func TestResolveEffectiveModelRouting_MappingOnlyDoesNotRestrictUnmappedModels(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformModelRoutingConfig: `{
				"openai": {
					"model_mapping": {"gpt-4o-mini": "gpt-5.4"}
				}
			}`,
		},
	}, &config.Config{})
	account := &Account{Platform: PlatformOpenAI}

	result := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-5.5", false)

	require.True(t, result.Supported)
	require.False(t, result.Matched)
	require.Equal(t, "gpt-5.5", result.Model)
	require.Equal(t, "none", result.Source)
}

func TestResolveEffectiveModelRouting_MappingOnlyKeepsAntigravityDefaultMapping(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformModelRoutingConfig: `{
				"antigravity": {
					"model_mapping": {"custom-model": "custom-upstream"}
				}
			}`,
		},
	}, &config.Config{})
	account := &Account{Platform: PlatformAntigravity}

	result := ResolveEffectiveModelRouting(context.Background(), svc, account, "claude-opus-4-6", false)

	require.True(t, result.Supported)
	require.False(t, result.Matched)
	require.Equal(t, "claude-opus-4-6-thinking", result.Model)
	require.Equal(t, "none", result.Source)
}

func TestResolveEffectiveModelRouting_AccountRulesOverrideSystemConfig(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformModelRoutingConfig: `{
				"openai": {"model_mapping": {"gpt-4o-mini": "gpt-5.4"}}
			}`,
		},
	}, &config.Config{})
	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"gpt-4o-mini": "gpt-account"},
		},
	}

	result := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-4o-mini", false)

	require.True(t, result.Supported)
	require.True(t, result.Matched)
	require.Equal(t, "gpt-account", result.Model)
	require.Equal(t, "account", result.Source)
}

func TestResolveEffectiveModelRouting_CompactMappingIsSeparate(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformModelRoutingConfig: `{
				"openai": {
					"model_mapping": {"gpt-4o-mini": "gpt-5.4"},
					"compact_model_mapping": {"gpt-5.4": "gpt-5.4-mini"}
				}
			}`,
		},
	}, &config.Config{})
	account := &Account{Platform: PlatformOpenAI}

	normal := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-4o-mini", false)
	require.Equal(t, "gpt-5.4", normal.Model)

	compact := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-4o-mini", true)
	require.Equal(t, "gpt-5.4-mini", compact.Model)
}

func TestResolveGatewayAnthropicForwardModel_UsesPlatformRoutingForOAuthAndServiceAccount(t *testing.T) {
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
