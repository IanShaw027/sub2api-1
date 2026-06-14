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

func TestResolveEffectiveModelRouting_IgnoresLegacyPlatformModelRoutingConfig(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			"platform_model_routing_config": `{
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
	require.False(t, result.Matched)
	require.Equal(t, "gpt-4o-mini", result.Model)
	require.Equal(t, "none", result.Source)
}

func TestResolveEffectiveModelRouting_EmptyAccountMappingIgnoresPlatformDefault(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformDefaultAccountModelConfig: `{
				"kiro": {
					"model_mapping": {
						"anthropic-opus-4-8": "claude-opus-4.8"
					}
				}
			}`,
		},
	}, &config.Config{})
	account := &Account{
		Platform: PlatformKiro,
		Credentials: map[string]any{
			"model_mapping": map[string]any{},
		},
	}

	result := ResolveEffectiveModelRouting(context.Background(), svc, account, "anthropic-opus-4-8", false)

	require.True(t, result.Supported)
	require.False(t, result.Matched)
	require.Equal(t, "anthropic-opus-4-8", result.Model)
	require.Equal(t, "none", result.Source)
}

func TestResolveEffectiveModelRouting_MappingOnlyDoesNotRestrictUnmappedModels(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformDefaultAccountModelConfig: `{
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

func TestResolveEffectiveModelRouting_IgnoresPlatformDefaultMapping(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformDefaultAccountModelConfig: `{
				"openai": {
					"model_mapping": {"gpt-4o-mini": "gpt-5.4"}
				}
			}`,
		},
	}, &config.Config{})
	account := &Account{Platform: PlatformOpenAI}

	result := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-4o-mini", false)

	require.True(t, result.Supported)
	require.False(t, result.Matched)
	require.Equal(t, "gpt-4o-mini", result.Model)
	require.Equal(t, "none", result.Source)
}

func TestResolveEffectiveModelRouting_MappingOnlyKeepsAntigravityDefaultMapping(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			"platform_model_routing_config": `{
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

func TestResolveEffectiveModelRouting_AccountRulesOverridePlatformDefaultConfig(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	svc := NewSettingService(&kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyPlatformDefaultAccountModelConfig: `{
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
			SettingKeyPlatformDefaultAccountModelConfig: `{
				"openai": {
					"model_mapping": {"gpt-4o-mini": "gpt-5.4"},
					"compact_model_mapping": {"gpt-5.4": "gpt-5.4-mini"}
				}
			}`,
		},
	}, &config.Config{})
	account := &Account{Platform: PlatformOpenAI}

	normal := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-4o-mini", false)
	require.Equal(t, "gpt-4o-mini", normal.Model)
	require.False(t, normal.Matched)
	require.Equal(t, "none", normal.Source)

	compact := ResolveEffectiveModelRouting(context.Background(), svc, account, "gpt-4o-mini", true)
	require.Equal(t, "gpt-4o-mini", compact.Model)
	require.False(t, compact.Matched)
	require.Equal(t, "none", compact.Source)
}

func TestResolveGatewayAnthropicForwardModel_IgnoresPlatformDefaultForOAuthAndServiceAccount(t *testing.T) {
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
