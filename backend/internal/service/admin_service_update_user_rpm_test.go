//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// rpmUserRepoStub 复用 admin_service_update_balance_test.go 的基础 stub 结构，
// 只在 Update 时把入参克隆一份，便于断言修改后的 RPMLimit。
type rpmUserRepoStub struct {
	*userRepoStub
	lastUpdated *User
}

func (s *rpmUserRepoStub) Update(_ context.Context, user *User) error {
	if user == nil {
		return nil
	}
	clone := *user
	s.lastUpdated = &clone
	if s.userRepoStub != nil {
		s.userRepoStub.user = &clone
	}
	return nil
}

func TestAdminService_UpdateUser_InvalidatesAuthCacheOnRPMLimitChange(t *testing.T) {
	base := &userRepoStub{
		user:   &User{ID: 42, Email: "u@example.com", RPMLimit: 10},
		avatar: &UserAvatar{StorageProvider: "remote_url", URL: "https://cdn.example.com/u.png", ContentType: "image/png"},
	}
	repo := &rpmUserRepoStub{userRepoStub: base}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       &redeemRepoStub{},
		authCacheInvalidator: invalidator,
	}

	newRPM := 60
	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{
		RPMLimit: &newRPM,
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 60, updated.RPMLimit)
	require.Equal(t, base.avatar.URL, updated.AvatarURL)
	require.Equal(t, []int64{42}, base.avatarLookups)
	require.Equal(t, []int64{42}, invalidator.userIDs, "仅修改 RPMLimit 也应失效 API Key 认证缓存")
}

func TestAdminService_UpdateUser_RejectsNilInput(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", RPMLimit: 10}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	svc := &adminServiceImpl{
		userRepo:       repo,
		redeemCodeRepo: &redeemRepoStub{},
	}

	_, err := svc.UpdateUser(context.Background(), 42, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "user input is required")
	require.Nil(t, repo.lastUpdated)
}

func TestAdminService_UpdateUser_NoInvalidateWhenRPMLimitUnchanged(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", RPMLimit: 10, Username: "old"}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       &redeemRepoStub{},
		authCacheInvalidator: invalidator,
	}

	newName := "new"
	sameRPM := 10
	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{
		Username: &newName,
		RPMLimit: &sameRPM,
	})
	require.NoError(t, err)
	require.Empty(t, invalidator.userIDs, "只改 username 不应触发认证缓存失效")
}

func TestAdminService_UpdateUser_RejectsNegativeRPMLimit(t *testing.T) {
	base := &userRepoStub{user: &User{ID: 42, Email: "u@example.com", RPMLimit: 10}}
	repo := &rpmUserRepoStub{userRepoStub: base}
	svc := &adminServiceImpl{
		userRepo:       repo,
		redeemCodeRepo: &redeemRepoStub{},
	}

	negative := -1
	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{
		RPMLimit: &negative,
	})
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Nil(t, repo.lastUpdated)
}

func TestAdminService_CreateUser_RejectsNegativeRPMLimit(t *testing.T) {
	repo := &userRepoStub{}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:       "u@example.com",
		Password:    "pass",
		RPMLimit:    -1,
		Concurrency: 1,
	})
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
}
