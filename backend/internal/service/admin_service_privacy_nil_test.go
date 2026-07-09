//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminServicePrivacyHelpersSkipNilAccount(t *testing.T) {
	t.Parallel()

	svc := &adminServiceImpl{}

	require.Equal(t, "", svc.EnsureOpenAIPrivacy(context.Background(), nil))
	require.Equal(t, "", svc.ForceOpenAIPrivacy(context.Background(), nil))
	require.Equal(t, "", svc.EnsureAntigravityPrivacy(context.Background(), nil))
	require.Equal(t, "", svc.ForceAntigravityPrivacy(context.Background(), nil))
}
