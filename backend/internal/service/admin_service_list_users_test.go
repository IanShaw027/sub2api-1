//go:build unit

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type userRepoStubForListUsers struct {
	userRepoStub
	users                   []User
	err                     error
	listWithFiltersParams   pagination.PaginationParams
	listWithFiltersCalls    []pagination.PaginationParams
	listWithFiltersFilters  []UserListFilters
	lastUsedByUserID        map[int64]*time.Time
	lastUsedErr             error
	populateUsageStatsCalls int
}

func (s *userRepoStubForListUsers) ListWithFilters(_ context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	s.listWithFiltersParams = params
	s.listWithFiltersCalls = append(s.listWithFiltersCalls, params)
	s.listWithFiltersFilters = append(s.listWithFiltersFilters, filters)
	if s.err != nil {
		return nil, nil, s.err
	}
	out := append([]User(nil), s.users...)
	start := params.Offset()
	if start >= len(out) {
		return []User{}, &pagination.PaginationResult{
			Total:    int64(len(out)),
			Page:     params.Page,
			PageSize: params.PageSize,
		}, nil
	}
	limit := params.Limit()
	end := start + limit
	if end > len(out) {
		end = len(out)
	}
	out = out[start:end]
	return out, &pagination.PaginationResult{
		Total:    int64(len(s.users)),
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

func (s *userRepoStubForListUsers) PopulateUsageStats(_ context.Context, users []User) error {
	s.populateUsageStatsCalls++
	for i := range users {
		users[i].TodayBalanceActualCost = float64(users[i].ID) / 100
	}
	return nil
}

func (s *userRepoStubForListUsers) GetLatestUsedAtByUserIDs(_ context.Context, userIDs []int64) (map[int64]*time.Time, error) {
	if s.lastUsedErr != nil {
		return nil, s.lastUsedErr
	}
	result := make(map[int64]*time.Time, len(userIDs))
	for _, userID := range userIDs {
		if ts, ok := s.lastUsedByUserID[userID]; ok {
			result[userID] = ts
		}
	}
	return result, nil
}

func (s *userRepoStubForListUsers) GetLatestUsedAtByUserID(_ context.Context, userID int64) (*time.Time, error) {
	if s.lastUsedErr != nil {
		return nil, s.lastUsedErr
	}
	return s.lastUsedByUserID[userID], nil
}

type userGroupRateRepoStubForListUsers struct {
	batchCalls int
	singleCall []int64

	batchErr  error
	batchData map[int64]map[int64]float64

	singleErr  map[int64]error
	singleData map[int64]map[int64]float64
}

func (s *userGroupRateRepoStubForListUsers) GetByUserIDs(_ context.Context, _ []int64) (map[int64]map[int64]float64, error) {
	s.batchCalls++
	if s.batchErr != nil {
		return nil, s.batchErr
	}
	return s.batchData, nil
}

func (s *userGroupRateRepoStubForListUsers) GetByUserID(_ context.Context, userID int64) (map[int64]float64, error) {
	s.singleCall = append(s.singleCall, userID)
	if err, ok := s.singleErr[userID]; ok {
		return nil, err
	}
	if rates, ok := s.singleData[userID]; ok {
		return rates, nil
	}
	return map[int64]float64{}, nil
}

func (s *userGroupRateRepoStubForListUsers) GetByUserAndGroup(_ context.Context, userID, groupID int64) (*float64, error) {
	panic("unexpected GetByUserAndGroup call")
}

func (s *userGroupRateRepoStubForListUsers) GetRPMOverrideByUserAndGroup(_ context.Context, _, _ int64) (*int, error) {
	panic("unexpected GetRPMOverrideByUserAndGroup call")
}

func (s *userGroupRateRepoStubForListUsers) SyncUserGroupRates(_ context.Context, userID int64, rates map[int64]*float64) error {
	panic("unexpected SyncUserGroupRates call")
}

func (s *userGroupRateRepoStubForListUsers) GetByGroupID(_ context.Context, _ int64) ([]UserGroupRateEntry, error) {
	panic("unexpected GetByGroupID call")
}

func (s *userGroupRateRepoStubForListUsers) SyncGroupRateMultipliers(_ context.Context, _ int64, _ []GroupRateMultiplierInput) error {
	panic("unexpected SyncGroupRateMultipliers call")
}

func (s *userGroupRateRepoStubForListUsers) SyncGroupRPMOverrides(_ context.Context, _ int64, _ []GroupRPMOverrideInput) error {
	panic("unexpected SyncGroupRPMOverrides call")
}

func (s *userGroupRateRepoStubForListUsers) ClearGroupRPMOverrides(_ context.Context, _ int64) error {
	panic("unexpected ClearGroupRPMOverrides call")
}

func (s *userGroupRateRepoStubForListUsers) DeleteByGroupID(_ context.Context, _ int64) error {
	panic("unexpected DeleteByGroupID call")
}

func (s *userGroupRateRepoStubForListUsers) DeleteByUserID(_ context.Context, userID int64) error {
	panic("unexpected DeleteByUserID call")
}

func TestAdminService_ListUsers_LoadsGroupRates(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		users: []User{
			{ID: 101, Username: "u1"},
			{ID: 202, Username: "u2"},
		},
	}
	rateRepo := &userGroupRateRepoStubForListUsers{
		batchData: map[int64]map[int64]float64{
			101: {11: 1.1},
			202: {22: 2.2},
		},
	}
	svc := &adminServiceImpl{
		userRepo:          userRepo,
		userGroupRateRepo: rateRepo,
	}

	users, total, err := svc.ListUsers(context.Background(), 1, 20, UserListFilters{}, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, users, 2)
	require.Equal(t, 1, rateRepo.batchCalls)
	require.Empty(t, rateRepo.singleCall)
	require.Equal(t, map[int64]float64{11: 1.1}, users[0].GroupRates)
	require.Equal(t, map[int64]float64{22: 2.2}, users[1].GroupRates)
}

func TestAdminService_ListUsers_PassesSortParams(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		users: []User{{ID: 1, Email: "a@example.com"}},
	}
	svc := &adminServiceImpl{userRepo: userRepo}

	_, _, err := svc.ListUsers(context.Background(), 2, 50, UserListFilters{}, "email", "ASC")
	require.NoError(t, err)
	require.Equal(t, pagination.PaginationParams{
		Page:      2,
		PageSize:  50,
		SortBy:    "email",
		SortOrder: "ASC",
	}, userRepo.listWithFiltersParams)
}

func TestAdminService_ListUsers_SortsByLiveAvailableConcurrency(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		users: []User{
			{ID: 1, Email: "one@example.com", Concurrency: 5},
			{ID: 2, Email: "two@example.com", Concurrency: 10},
			{ID: 3, Email: "three@example.com", Concurrency: 3},
		},
	}
	cache := &stubConcurrencyCacheForTest{
		usersLoadBatch: map[int64]*UserLoadInfo{
			1: {UserID: 1, CurrentConcurrency: 1},
			2: {UserID: 2, CurrentConcurrency: 5},
			3: {UserID: 3, CurrentConcurrency: 0},
		},
	}
	svc := &adminServiceImpl{
		userRepo:           userRepo,
		concurrencyService: NewConcurrencyService(cache),
	}

	users, total, err := svc.ListUsers(context.Background(), 1, 2, UserListFilters{}, "available_concurrency", "asc")
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, users, 2)
	require.Equal(t, int64(3), users[0].ID)
	require.Equal(t, int64(1), users[1].ID)
}

func TestAdminService_ListUsers_LiveConcurrencySortLoadsBeyondPaginationCap(t *testing.T) {
	users := make([]User, 0, 1001)
	loadMap := make(map[int64]*UserLoadInfo, 1001)
	for i := 1; i <= 1001; i++ {
		id := int64(i)
		users = append(users, User{ID: id, Email: strconv.Itoa(i) + "@example.com", Concurrency: 5})
		loadMap[id] = &UserLoadInfo{UserID: id, CurrentConcurrency: i}
	}
	userRepo := &userRepoStubForListUsers{users: users}
	cache := &stubConcurrencyCacheForTest{
		usersLoadBatch: loadMap,
	}
	svc := &adminServiceImpl{
		userRepo:           userRepo,
		concurrencyService: NewConcurrencyService(cache),
	}

	got, total, err := svc.ListUsers(context.Background(), 1, 2, UserListFilters{}, "current_concurrency", "desc")
	require.NoError(t, err)
	require.Equal(t, int64(1001), total)
	require.Len(t, got, 2)
	require.Equal(t, int64(1001), got[0].ID)
	require.Equal(t, int64(1000), got[1].ID)
	require.Len(t, userRepo.listWithFiltersCalls, 3)
	require.Equal(t, 1, userRepo.listWithFiltersCalls[0].Page)
	require.Equal(t, 1, userRepo.listWithFiltersCalls[0].PageSize)
	require.Equal(t, 1, userRepo.listWithFiltersCalls[1].Page)
	require.Equal(t, 1000, userRepo.listWithFiltersCalls[1].PageSize)
	require.Equal(t, 2, userRepo.listWithFiltersCalls[2].Page)
	require.Equal(t, 1000, userRepo.listWithFiltersCalls[2].PageSize)
	require.Len(t, userRepo.listWithFiltersFilters, 3)
	for _, filters := range userRepo.listWithFiltersFilters {
		require.NotNil(t, filters.IncludeUsageStats)
		require.False(t, *filters.IncludeUsageStats)
	}
	require.Equal(t, 1, userRepo.populateUsageStatsCalls)
	require.InDelta(t, 10.01, got[0].TodayBalanceActualCost, 0.0001)
	require.InDelta(t, 10.00, got[1].TodayBalanceActualCost, 0.0001)
}

func TestAdminService_ListUsers_LiveConcurrencySortReturnsErrorWhenLoadFails(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		users: []User{
			{ID: 1, Email: "one@example.com", Concurrency: 5},
			{ID: 2, Email: "two@example.com", Concurrency: 10},
			{ID: 3, Email: "three@example.com", Concurrency: 3},
		},
	}
	cache := &stubConcurrencyCacheForTest{
		usersLoadErr: errors.New("redis unavailable"),
	}
	svc := &adminServiceImpl{
		userRepo:           userRepo,
		concurrencyService: NewConcurrencyService(cache),
	}

	users, total, err := svc.ListUsers(context.Background(), 1, 3, UserListFilters{}, "available_concurrency", "desc")
	require.Error(t, err)
	require.ErrorContains(t, err, "available_concurrency")
	require.ErrorContains(t, err, "redis unavailable")
	require.Zero(t, total)
	require.Nil(t, users)
}

func TestAdminService_ListUsers_LiveConcurrencySortReturnsErrorWhenServiceUnavailable(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		users: []User{
			{ID: 1, Email: "one@example.com", Concurrency: 5},
			{ID: 2, Email: "two@example.com", Concurrency: 10},
		},
	}
	svc := &adminServiceImpl{
		userRepo: userRepo,
	}

	users, total, err := svc.ListUsers(context.Background(), 1, 2, UserListFilters{}, "current_concurrency", "asc")
	require.Error(t, err)
	require.ErrorContains(t, err, "current_concurrency")
	require.ErrorContains(t, err, "requires concurrency service")
	require.Zero(t, total)
	require.Nil(t, users)
}

func TestAdminService_ListUsers_LiveConcurrencySortReturnsErrorWhenLoadDataIsIncomplete(t *testing.T) {
	userRepo := &userRepoStubForListUsers{
		users: []User{
			{ID: 1, Email: "one@example.com", Concurrency: 5},
			{ID: 2, Email: "two@example.com", Concurrency: 10},
		},
	}
	cache := &stubConcurrencyCacheForTest{
		usersLoadBatch: map[int64]*UserLoadInfo{
			1: {UserID: 1, CurrentConcurrency: 1},
		},
	}
	svc := &adminServiceImpl{
		userRepo:           userRepo,
		concurrencyService: NewConcurrencyService(cache),
	}

	users, total, err := svc.ListUsers(context.Background(), 1, 2, UserListFilters{}, "current_concurrency", "desc")
	require.Error(t, err)
	require.ErrorContains(t, err, "current_concurrency")
	require.ErrorContains(t, err, "missing user 2")
	require.Zero(t, total)
	require.Nil(t, users)
}

func TestAdminService_ListUsers_PopulatesLastUsedAt(t *testing.T) {
	lastUsed := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	userRepo := &userRepoStubForListUsers{
		users: []User{{ID: 101, Email: "u@example.com"}},
		lastUsedByUserID: map[int64]*time.Time{
			101: &lastUsed,
		},
	}
	svc := &adminServiceImpl{userRepo: userRepo}

	users, total, err := svc.ListUsers(context.Background(), 1, 20, UserListFilters{}, "", "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, users, 1)
	require.NotNil(t, users[0].LastUsedAt)
	require.WithinDuration(t, lastUsed, *users[0].LastUsedAt, time.Second)
}
