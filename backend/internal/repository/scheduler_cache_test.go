package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFilterSchedulerCredentialsKeepsSubscriptionPlanType(t *testing.T) {
	filtered := filterSchedulerCredentials(map[string]any{
		"plan_type":     "plus",
		"access_token":  "secret-access-token",
		"refresh_token": "secret-refresh-token",
	})

	require.Equal(t, "plus", filtered["plan_type"])
	require.NotContains(t, filtered, "access_token")
	require.NotContains(t, filtered, "refresh_token")
}

func TestSchedulerMetadataAccountKeepsOpenAISubscriptionIdentity(t *testing.T) {
	account := service.Account{
		ID:       24,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"plan_type":    "plus",
			"access_token": "secret-access-token",
		},
	}

	metadata := buildSchedulerMetadataAccount(account)

	require.True(t, metadata.IsOpenAIChatGPTSubscription())
	require.Empty(t, metadata.GetCredential("access_token"))
}

func TestFilterSchedulerCredentialsDropsAPIKey(t *testing.T) {
	filtered := filterSchedulerCredentials(map[string]any{
		"api_key":       "secret-api-key",
		"plan_type":     "plus",
		"model_mapping": map[string]any{"a": "b"},
	})
	require.NotContains(t, filtered, "api_key")
	require.Equal(t, "plus", filtered["plan_type"])
	require.Contains(t, filtered, "model_mapping")
}

func TestMarshalSchedulerCacheAccountDropsSecretsFromAllPayloads(t *testing.T) {
	account := service.Account{
		ID:       42,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":       "sk-secret",
			"access_token":  "oauth-secret",
			"model_mapping": map[string]any{"gpt-4": "gpt-4"},
		},
	}

	fullPayload, metaPayload, err := marshalSchedulerCacheAccount(account)
	require.NoError(t, err)
	for _, payload := range [][]byte{fullPayload, metaPayload} {
		require.NotContains(t, string(payload), "sk-secret")
		require.NotContains(t, string(payload), "oauth-secret")
	}

	decoded, err := decodeCachedAccount(fullPayload)
	require.NoError(t, err)
	require.Empty(t, decoded.GetCredential("api_key"))
	require.Equal(t, "gpt-4", decoded.GetModelMapping()["gpt-4"])
}
