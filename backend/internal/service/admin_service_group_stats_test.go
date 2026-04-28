//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminServiceGetGroupStatsReturnsNotFoundForMissingGroup(t *testing.T) {
	svc := &adminServiceImpl{
		groupRepo: &groupRepoStubForAdmin{getErr: ErrGroupNotFound},
		apiKeyRepo: &apiKeyRepoStubForGroupUpdate{
			key: &APIKey{ID: 1},
		},
	}

	_, err := svc.GetGroupStats(context.Background(), 42)

	require.ErrorIs(t, err, ErrGroupNotFound)
}
