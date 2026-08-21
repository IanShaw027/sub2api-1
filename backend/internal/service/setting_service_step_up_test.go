//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsStepUpEnabledDefaultsDisabledOnlyWhenSettingMissing(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{}
	service := &SettingService{settingRepo: repo}

	enabled, err := service.IsStepUpEnabled(context.Background())

	require.NoError(t, err)
	require.False(t, enabled)
}

func TestIsStepUpEnabledReturnsStorageError(t *testing.T) {
	readErr := errors.New("read failed")
	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{},
		errs:   map[string]error{SettingKeyStepUpEnabled: readErr},
	}
	service := &SettingService{settingRepo: repo}

	enabled, err := service.IsStepUpEnabled(context.Background())

	require.False(t, enabled)
	require.ErrorIs(t, err, readErr)
}

func TestIsStepUpEnabledReadsConfiguredValue(t *testing.T) {
	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{SettingKeyStepUpEnabled: "true"},
	}
	service := &SettingService{settingRepo: repo}

	enabled, err := service.IsStepUpEnabled(context.Background())

	require.NoError(t, err)
	require.True(t, enabled)
}
