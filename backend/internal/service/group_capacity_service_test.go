//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type groupCapacityAccountRepoStub struct {
	AccountRepository
	accountsByGroup map[int64][]Account
}

func (r groupCapacityAccountRepoStub) ListSchedulableByGroupID(_ context.Context, groupID int64) ([]Account, error) {
	return append([]Account(nil), r.accountsByGroup[groupID]...), nil
}

type groupCapacityGroupRepoStub struct {
	GroupRepository
	groups []Group
}

func (r groupCapacityGroupRepoStub) ListActive(_ context.Context) ([]Group, error) {
	return append([]Group(nil), r.groups...), nil
}

type groupCapacityConcurrencyCacheStub struct {
	ConcurrencyCache
	groupConcurrency map[int64]int
}

func (c groupCapacityConcurrencyCacheStub) GetGroupConcurrency(_ context.Context, groupID int64) (int, error) {
	return c.groupConcurrency[groupID], nil
}

func TestGroupCapacityUsesGroupScopedConcurrencyOnly(t *testing.T) {
	account := Account{ID: 101, Concurrency: 10}
	accountRepo := groupCapacityAccountRepoStub{
		accountsByGroup: map[int64][]Account{
			10: {account},
			20: {account},
		},
	}
	groupRepo := groupCapacityGroupRepoStub{
		groups: []Group{
			{ID: 10, Status: StatusActive},
			{ID: 20, Status: StatusActive},
		},
	}
	concurrencyCache := groupCapacityConcurrencyCacheStub{
		groupConcurrency: map[int64]int{
			10: 2,
			20: 0,
		},
	}
	svc := NewGroupCapacityService(
		accountRepo,
		groupRepo,
		NewConcurrencyService(concurrencyCache),
		nil,
		nil,
	)

	summaries, err := svc.GetAllGroupCapacity(context.Background())

	require.NoError(t, err)
	require.Len(t, summaries, 2)
	byGroup := map[int64]GroupCapacitySummary{}
	for _, summary := range summaries {
		byGroup[summary.GroupID] = summary
	}
	require.Equal(t, 2, byGroup[10].ConcurrencyUsed)
	require.Zero(t, byGroup[10].ConcurrencyMax, "shared account max concurrency must not be shown as group capacity")
	require.Zero(t, byGroup[20].ConcurrencyUsed)
	require.Zero(t, byGroup[20].ConcurrencyMax)
}
