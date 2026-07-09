//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAdminAPIKeyStatus_NilRepoReturnsNotConfigured(t *testing.T) {
	svc := NewSettingService(nil, nil)

	maskedKey, exists, err := svc.GetAdminAPIKeyStatus(context.Background())

	require.NoError(t, err)
	require.False(t, exists)
	require.Empty(t, maskedKey)
}

func TestGetAdminAPIKey_NilRepoReturnsEmptyKey(t *testing.T) {
	svc := NewSettingService(nil, nil)

	key, err := svc.GetAdminAPIKey(context.Background())

	require.NoError(t, err)
	require.Empty(t, key)
}

func TestDeleteAdminAPIKey_NilRepoReturnsError(t *testing.T) {
	svc := NewSettingService(nil, nil)

	err := svc.DeleteAdminAPIKey(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "setting repository unavailable")
}

func TestGenerateAdminAPIKey_NilRepoReturnsError(t *testing.T) {
	svc := NewSettingService(nil, nil)

	_, err := svc.GenerateAdminAPIKey(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "setting repository unavailable")
}
