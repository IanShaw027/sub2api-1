//go:build unit

package service

import (
	"context"
	"errors"
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

type groupCapacityBatchAccountRepoStub struct {
	groupCapacityAccountRepoStub
	capacityRows []GroupAccountCapacityRow
}

func (r groupCapacityBatchAccountRepoStub) ListSchedulableCapacityByGroupIDs(_ context.Context, groupIDs []int64) ([]GroupAccountCapacityRow, error) {
	allowed := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		allowed[groupID] = struct{}{}
	}
	rows := make([]GroupAccountCapacityRow, 0, len(r.capacityRows))
	for _, row := range r.capacityRows {
		if _, ok := allowed[row.GroupID]; ok {
			rows = append(rows, row)
		}
	}
	return rows, nil
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
	groupConcurrency           map[int64]int
	groupConcurrencyErr        error
	groupConcurrencyBatchCalls int
	accountConcurrencyBatch    map[int64]int
	accountConcurrencyBatchErr error
}

func (c groupCapacityConcurrencyCacheStub) GetGroupConcurrency(_ context.Context, groupID int64) (int, error) {
	if c.groupConcurrencyErr != nil {
		return 0, c.groupConcurrencyErr
	}
	return c.groupConcurrency[groupID], nil
}

func (c *groupCapacityConcurrencyCacheStub) GetGroupConcurrencyBatch(_ context.Context, groupIDs []int64) (map[int64]int, error) {
	c.groupConcurrencyBatchCalls++
	if c.groupConcurrencyErr != nil {
		return nil, c.groupConcurrencyErr
	}
	result := make(map[int64]int, len(groupIDs))
	for _, groupID := range groupIDs {
		result[groupID] = c.groupConcurrency[groupID]
	}
	return result, nil
}

func (c groupCapacityConcurrencyCacheStub) GetAccountConcurrencyBatch(_ context.Context, accountIDs []int64) (map[int64]int, error) {
	if c.accountConcurrencyBatchErr != nil {
		return nil, c.accountConcurrencyBatchErr
	}
	result := make(map[int64]int, len(accountIDs))
	for _, accountID := range accountIDs {
		result[accountID] = c.accountConcurrencyBatch[accountID]
	}
	return result, nil
}

func TestGroupCapacityReturnsGroupScopedUsedAndMax(t *testing.T) {
	account := Account{ID: 101, Concurrency: 10}
	account2 := Account{ID: 202, Concurrency: 4}
	accountRepo := groupCapacityAccountRepoStub{
		accountsByGroup: map[int64][]Account{
			10: {account, account2},
			20: {account},
		},
	}
	groupRepo := groupCapacityGroupRepoStub{
		groups: []Group{
			{ID: 10, Status: StatusActive},
			{ID: 20, Status: StatusActive},
		},
	}
	concurrencyCache := &groupCapacityConcurrencyCacheStub{
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
	require.Equal(t, 14, byGroup[10].ConcurrencyMax)
	require.Zero(t, byGroup[20].ConcurrencyUsed)
	require.Equal(t, 10, byGroup[20].ConcurrencyMax)
}

func TestGroupCapacityBatchUsesGroupScopedConcurrencyForSharedAccounts(t *testing.T) {
	accountRepo := groupCapacityBatchAccountRepoStub{
		capacityRows: []GroupAccountCapacityRow{
			{GroupID: 10, AccountID: 101, Concurrency: 10},
			{GroupID: 20, AccountID: 101, Concurrency: 10},
		},
	}
	groupRepo := groupCapacityGroupRepoStub{
		groups: []Group{
			{ID: 10, Status: StatusActive},
			{ID: 20, Status: StatusActive},
		},
	}
	concurrencyCache := &groupCapacityConcurrencyCacheStub{
		groupConcurrency: map[int64]int{
			10: 1,
			20: 0,
		},
		accountConcurrencyBatch: map[int64]int{
			101: 5,
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
	byGroup := map[int64]GroupCapacitySummary{}
	for _, summary := range summaries {
		byGroup[summary.GroupID] = summary
	}
	require.Equal(t, 1, byGroup[10].ConcurrencyUsed)
	require.Equal(t, 10, byGroup[10].ConcurrencyMax)
	require.Zero(t, byGroup[20].ConcurrencyUsed)
	require.Equal(t, 10, byGroup[20].ConcurrencyMax)
	require.Equal(t, 1, concurrencyCache.groupConcurrencyBatchCalls)
}

func TestGroupCapacityReturnsErrorWhenGroupConcurrencyReadFails(t *testing.T) {
	account := Account{ID: 101, Concurrency: 10}
	account2 := Account{ID: 202, Concurrency: 4}
	accountRepo := groupCapacityAccountRepoStub{
		accountsByGroup: map[int64][]Account{
			10: {account, account2},
		},
	}
	groupRepo := groupCapacityGroupRepoStub{
		groups: []Group{{ID: 10, Status: StatusActive}},
	}
	concurrencyCache := &groupCapacityConcurrencyCacheStub{
		groupConcurrencyErr: errors.New("redis read failed"),
		accountConcurrencyBatch: map[int64]int{
			101: 2,
			202: 3,
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

	require.Error(t, err)
	require.Nil(t, summaries)
}
