//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type deletedUserAvatarRepo struct {
	*userRepoStub
	avatar *UserAvatar
}

func (r *deletedUserAvatarRepo) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	return r.avatar, nil
}

func TestAdminService_GetUserIncludeDeleted(t *testing.T) {
	ts := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	repo := &deletedUserAvatarRepo{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Email: "del@test.com", DeletedAt: &ts}},
		avatar: &UserAvatar{
			StorageProvider: "inline",
			URL:             "data:image/png;base64,dmFsaWQ=",
			ContentType:     "image/png",
			ByteSize:        5,
		},
	}
	svc := &adminServiceImpl{userRepo: repo}

	got, err := svc.GetUserIncludeDeleted(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, int64(7), got.ID)
	require.NotNil(t, got.DeletedAt)
	require.Equal(t, repo.avatar.URL, got.AvatarURL)
	require.Equal(t, repo.avatar.StorageProvider, got.AvatarSource)
}
