package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type kiroDefaultProxyRepoStub struct {
	getByIDFunc func(ctx context.Context, id int64) (*Proxy, error)
}

func (m *kiroDefaultProxyRepoStub) Create(ctx context.Context, proxy *Proxy) error {
	panic("Create not implemented")
}

func (m *kiroDefaultProxyRepoStub) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, fmt.Errorf("proxy not found")
}

func (m *kiroDefaultProxyRepoStub) ListByIDs(ctx context.Context, ids []int64) ([]Proxy, error) {
	panic("ListByIDs not implemented")
}

func (m *kiroDefaultProxyRepoStub) Update(ctx context.Context, proxy *Proxy) error {
	panic("Update not implemented")
}

func (m *kiroDefaultProxyRepoStub) Delete(ctx context.Context, id int64) error {
	panic("Delete not implemented")
}

func (m *kiroDefaultProxyRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	panic("List not implemented")
}

func (m *kiroDefaultProxyRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]Proxy, *pagination.PaginationResult, error) {
	panic("ListWithFilters not implemented")
}

func (m *kiroDefaultProxyRepoStub) ListWithFiltersAndAccountCount(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	panic("ListWithFiltersAndAccountCount not implemented")
}

func (m *kiroDefaultProxyRepoStub) ListActive(ctx context.Context) ([]Proxy, error) {
	panic("ListActive not implemented")
}

func (m *kiroDefaultProxyRepoStub) ListActiveWithAccountCount(ctx context.Context) ([]ProxyWithAccountCount, error) {
	panic("ListActiveWithAccountCount not implemented")
}

func (m *kiroDefaultProxyRepoStub) ExistsByHostPortAuth(ctx context.Context, host string, port int, username, password string) (bool, error) {
	panic("ExistsByHostPortAuth not implemented")
}

func (m *kiroDefaultProxyRepoStub) CountAccountsByProxyID(ctx context.Context, proxyID int64) (int64, error) {
	panic("CountAccountsByProxyID not implemented")
}

func (m *kiroDefaultProxyRepoStub) ListAccountSummariesByProxyID(ctx context.Context, proxyID int64) ([]ProxyAccountSummary, error) {
	panic("ListAccountSummariesByProxyID not implemented")
}

func (m *kiroDefaultProxyRepoStub) SweepExpiredProxies(ctx context.Context, now time.Time) (int64, error) {
	panic("SweepExpiredProxies not implemented")
}

func (m *kiroDefaultProxyRepoStub) ListAllForFallback(ctx context.Context) ([]Proxy, error) {
	panic("ListAllForFallback not implemented")
}

func (m *kiroDefaultProxyRepoStub) CountExpired(ctx context.Context) (int64, error) {
	panic("CountExpired not implemented")
}

func (m *kiroDefaultProxyRepoStub) CountExpiringSoon(ctx context.Context, now time.Time) (int64, error) {
	panic("CountExpiringSoon not implemented")
}

type kiroDefaultAccountRepoStub struct {
	accountsByID     map[int64]*Account
	getByIDsAccounts []*Account
	getByIDsErr      error
	getByIDsCalled   bool
	getByIDsIDs      []int64
	getByIDErrByID   map[int64]error
	getByIDCalled    []int64
	bindGroupErrByID map[int64]error
	bindGroupsCalls  []int64
	bulkUpdateErr    error
	bulkUpdateIDs    []int64
	bulkUpdateCreds  map[string]any
	listByGroupData  map[int64][]Account
	listByGroupErr   map[int64]error
	createdAccounts  []*Account
	updatedAccounts  []*Account
	existsByIDValue  bool
	existsByIDErr    error
}

func (s *kiroDefaultAccountRepoStub) Create(ctx context.Context, account *Account) error {
	s.createdAccounts = append(s.createdAccounts, account)
	if account != nil && account.ID == 0 {
		account.ID = 101
	}
	if s.accountsByID == nil {
		s.accountsByID = map[int64]*Account{}
	}
	if account != nil {
		s.accountsByID[account.ID] = account
	}
	return nil
}

func (s *kiroDefaultAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	s.getByIDCalled = append(s.getByIDCalled, id)
	if err, ok := s.getByIDErrByID[id]; ok {
		return nil, err
	}
	if account, ok := s.accountsByID[id]; ok {
		return account, nil
	}
	return nil, errors.New("account not found")
}

func (s *kiroDefaultAccountRepoStub) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	s.getByIDsCalled = true
	s.getByIDsIDs = append([]int64{}, ids...)
	if s.getByIDsErr != nil {
		return nil, s.getByIDsErr
	}
	return s.getByIDsAccounts, nil
}

func (s *kiroDefaultAccountRepoStub) ExistsByID(ctx context.Context, id int64) (bool, error) {
	if s.existsByIDErr != nil {
		return false, s.existsByIDErr
	}
	if s.accountsByID != nil {
		_, ok := s.accountsByID[id]
		return ok, nil
	}
	return s.existsByIDValue, nil
}

func (s *kiroDefaultAccountRepoStub) GetByCRSAccountID(ctx context.Context, crsAccountID string) (*Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) FindByExtraField(ctx context.Context, key string, value any) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListCRSAccountIDs(ctx context.Context) (map[string]int64, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) Update(ctx context.Context, account *Account) error {
	s.updatedAccounts = append(s.updatedAccounts, account)
	if s.accountsByID == nil {
		s.accountsByID = map[int64]*Account{}
	}
	if account != nil {
		s.accountsByID[account.ID] = account
	}
	return nil
}

func (s *kiroDefaultAccountRepoStub) Delete(ctx context.Context, id int64) error { return nil }

func (s *kiroDefaultAccountRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListByGroup(ctx context.Context, groupID int64) ([]Account, error) {
	if err, ok := s.listByGroupErr[groupID]; ok {
		return nil, err
	}
	if rows, ok := s.listByGroupData[groupID]; ok {
		return rows, nil
	}
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListActive(ctx context.Context) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) UpdateLastUsed(ctx context.Context, id int64) error { return nil }

func (s *kiroDefaultAccountRepoStub) BatchUpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) ClearError(ctx context.Context, id int64) error { return nil }

func (s *kiroDefaultAccountRepoStub) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) AutoPauseExpiredAccounts(ctx context.Context, now time.Time) (int64, error) {
	return 0, nil
}

func (s *kiroDefaultAccountRepoStub) BindGroups(ctx context.Context, accountID int64, groupIDs []int64) error {
	s.bindGroupsCalls = append(s.bindGroupsCalls, accountID)
	if err, ok := s.bindGroupErrByID[accountID]; ok {
		return err
	}
	return nil
}

func (s *kiroDefaultAccountRepoStub) ListSchedulable(ctx context.Context) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListSchedulableByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListSchedulableByGroupIDAndPlatform(ctx context.Context, groupID int64, platform string) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListSchedulableByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, groupID int64, platforms []string) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListSchedulableUngroupedByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) ListSchedulableUngroupedByPlatforms(ctx context.Context, platforms []string) ([]Account, error) {
	return nil, nil
}

func (s *kiroDefaultAccountRepoStub) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time, reason ...string) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) SetOverloaded(ctx context.Context, id int64, until time.Time) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) ClearTempUnschedulable(ctx context.Context, id int64) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) ClearRateLimit(ctx context.Context, id int64) error { return nil }

func (s *kiroDefaultAccountRepoStub) ClearAntigravityQuotaScopes(ctx context.Context, id int64) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) ClearModelRateLimits(ctx context.Context, id int64) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) UpdateSessionWindow(ctx context.Context, id int64, start, end *time.Time, status string) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) UpdateSessionWindowEnd(ctx context.Context, id int64, end time.Time) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) BulkUpdate(ctx context.Context, ids []int64, updates AccountBulkUpdate) (int64, error) {
	s.bulkUpdateIDs = append([]int64{}, ids...)
	s.bulkUpdateCreds = cloneCredentials(updates.Credentials)
	if s.bulkUpdateErr != nil {
		return 0, s.bulkUpdateErr
	}
	return int64(len(ids)), nil
}

func (s *kiroDefaultAccountRepoStub) IncrementQuotaUsed(ctx context.Context, id int64, amount float64) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) ResetQuotaUsed(ctx context.Context, id int64) error { return nil }

func (s *kiroDefaultAccountRepoStub) RevertProxyFallback(ctx context.Context, accountID int64) error {
	return nil
}

func (s *kiroDefaultAccountRepoStub) ListOAuthRefreshCandidates(ctx context.Context) ([]Account, error) {
	return nil, nil
}

type kiroDefaultGroupRepoStub struct {
	getByID *Group
	getErr  error
}

func (s *kiroDefaultGroupRepoStub) Create(_ context.Context, g *Group) error { return nil }

func (s *kiroDefaultGroupRepoStub) GetByID(_ context.Context, _ int64) (*Group, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.getByID, nil
}

func (s *kiroDefaultGroupRepoStub) GetByIDLite(_ context.Context, _ int64) (*Group, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.getByID, nil
}

func (s *kiroDefaultGroupRepoStub) Update(_ context.Context, g *Group) error { return nil }

func (s *kiroDefaultGroupRepoStub) Delete(_ context.Context, _ int64) error {
	panic("unexpected Delete call")
}

func (s *kiroDefaultGroupRepoStub) DeleteCascade(_ context.Context, _ int64) ([]int64, error) {
	panic("unexpected DeleteCascade call")
}

func (s *kiroDefaultGroupRepoStub) List(_ context.Context, _ pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *kiroDefaultGroupRepoStub) ListWithFilters(_ context.Context, _ pagination.PaginationParams, _, _, _ string, _ *bool) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *kiroDefaultGroupRepoStub) ListActive(_ context.Context) ([]Group, error) {
	panic("unexpected ListActive call")
}

func (s *kiroDefaultGroupRepoStub) ListActiveByPlatform(_ context.Context, _ string) ([]Group, error) {
	panic("unexpected ListActiveByPlatform call")
}

func (s *kiroDefaultGroupRepoStub) ExistsByName(_ context.Context, _ string) (bool, error) {
	panic("unexpected ExistsByName call")
}

func (s *kiroDefaultGroupRepoStub) GetAccountCount(_ context.Context, _ int64) (int64, int64, error) {
	panic("unexpected GetAccountCount call")
}

func (s *kiroDefaultGroupRepoStub) DeleteAccountGroupsByGroupID(_ context.Context, _ int64) (int64, error) {
	panic("unexpected DeleteAccountGroupsByGroupID call")
}

func (s *kiroDefaultGroupRepoStub) GetAccountIDsByGroupIDs(_ context.Context, _ []int64) ([]int64, error) {
	panic("unexpected GetAccountIDsByGroupIDs call")
}

func (s *kiroDefaultGroupRepoStub) BindAccountsToGroup(_ context.Context, _ int64, _ []int64) error {
	panic("unexpected BindAccountsToGroup call")
}

func (s *kiroDefaultGroupRepoStub) UpdateSortOrders(_ context.Context, _ []GroupSortOrderUpdate) error {
	return nil
}
