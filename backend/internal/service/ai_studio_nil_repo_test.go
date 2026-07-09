//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAIStudioRuntime_NilRepoDefaultsDisabled(t *testing.T) {
	svc := NewSettingService(nil, nil)

	runtime := svc.GetAIStudioRuntime(context.Background())

	require.False(t, runtime.Enabled)
}
