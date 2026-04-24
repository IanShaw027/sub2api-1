//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type groupRepoStubForKiroCopyFilter struct {
	groupRepoStubForAdmin
	groups               map[int64]*Group
	accountIDsByGroupIDs []int64
	requestedGroupIDs    []int64
	boundGroupID         int64
	boundAccountIDs      []int64
	deletedGroupID       int64
}

func (s *groupRepoStubForKiroCopyFilter) Create(_ context.Context, g *Group) error {
	clone := *g
	if clone.ID == 0 {
		clone.ID = 101
		g.ID = clone.ID
	}
	s.created = &clone
	return nil
}

func (s *groupRepoStubForKiroCopyFilter) GetByID(_ context.Context, id int64) (*Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	clone := *group
	return &clone, nil
}

func (s *groupRepoStubForKiroCopyFilter) GetByIDLite(ctx context.Context, id int64) (*Group, error) {
	return s.GetByID(ctx, id)
}

func (s *groupRepoStubForKiroCopyFilter) DeleteAccountGroupsByGroupID(_ context.Context, groupID int64) (int64, error) {
	s.deletedGroupID = groupID
	return 1, nil
}

func (s *groupRepoStubForKiroCopyFilter) GetAccountIDsByGroupIDs(_ context.Context, groupIDs []int64) ([]int64, error) {
	s.requestedGroupIDs = append([]int64(nil), groupIDs...)
	return append([]int64(nil), s.accountIDsByGroupIDs...), nil
}

func (s *groupRepoStubForKiroCopyFilter) BindAccountsToGroup(_ context.Context, groupID int64, accountIDs []int64) error {
	s.boundGroupID = groupID
	s.boundAccountIDs = append([]int64(nil), accountIDs...)
	return nil
}

func TestAdminService_CreateGroup_FiltersAPIKeyAccountsForKiroOAuthOnlyCopy(t *testing.T) {
	t.Parallel()

	groupRepo := &groupRepoStubForKiroCopyFilter{
		groups: map[int64]*Group{
			9: {ID: 9, Name: "source", Platform: PlatformKiro},
		},
		accountIDsByGroupIDs: []int64{1, 2},
	}
	accountRepo := &accountRepoStubForBulkUpdate{
		getByIDsAccounts: []*Account{
			{ID: 1, Platform: PlatformKiro, Type: AccountTypeAPIKey},
			{ID: 2, Platform: PlatformKiro, Type: AccountTypeOAuth},
		},
	}
	svc := &adminServiceImpl{
		groupRepo:   groupRepo,
		accountRepo: accountRepo,
	}

	group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name:                     "kiro-target",
		Platform:                 PlatformKiro,
		RateMultiplier:           1.0,
		RequireOAuthOnly:         true,
		CopyAccountsFromGroupIDs: []int64{9},
	})
	require.NoError(t, err)
	require.NotNil(t, group)
	require.True(t, accountRepo.getByIDsCalled)
	require.Equal(t, []int64{9}, groupRepo.requestedGroupIDs)
	require.Equal(t, int64(101), groupRepo.boundGroupID)
	require.Equal(t, []int64{2}, groupRepo.boundAccountIDs)
}

func TestAdminService_UpdateGroup_FiltersAPIKeyAccountsForKiroOAuthOnlyCopy(t *testing.T) {
	t.Parallel()

	groupRepo := &groupRepoStubForKiroCopyFilter{
		groups: map[int64]*Group{
			55: {ID: 55, Name: "kiro-target", Platform: PlatformKiro, Status: StatusActive},
			9:  {ID: 9, Name: "source", Platform: PlatformKiro},
		},
		accountIDsByGroupIDs: []int64{1, 2},
	}
	accountRepo := &accountRepoStubForBulkUpdate{
		getByIDsAccounts: []*Account{
			{ID: 1, Platform: PlatformKiro, Type: AccountTypeAPIKey},
			{ID: 2, Platform: PlatformKiro, Type: AccountTypeOAuth},
		},
	}
	svc := &adminServiceImpl{
		groupRepo:   groupRepo,
		accountRepo: accountRepo,
	}

	requireOAuthOnly := true
	group, err := svc.UpdateGroup(context.Background(), 55, &UpdateGroupInput{
		RequireOAuthOnly:         &requireOAuthOnly,
		CopyAccountsFromGroupIDs: []int64{9},
	})
	require.NoError(t, err)
	require.NotNil(t, group)
	require.True(t, accountRepo.getByIDsCalled)
	require.Equal(t, int64(55), groupRepo.deletedGroupID)
	require.Equal(t, []int64{2}, groupRepo.boundAccountIDs)
	require.Equal(t, int64(55), groupRepo.boundGroupID)
}
