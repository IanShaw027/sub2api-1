//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiQuotaTierKeyForAccount_ProjectIDWithoutOAuthTypeFallsBackSafely(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"project_id": "project-1",
		},
	}

	require.Equal(t, GeminiTierAIStudioFree, geminiQuotaTierKeyForAccount(account))

	svc := NewGeminiQuotaService(nil, nil)
	quota, ok := svc.QuotaForAccount(context.Background(), account)
	require.True(t, ok)
	require.Equal(t, int64(50), quota.ProRPD)
	require.Equal(t, int64(1500), quota.FlashRPD)
}
