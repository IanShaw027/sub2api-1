package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type wireProxyRepoStub struct{}

func (wireProxyRepoStub) Create(ctx context.Context, proxy *Proxy) error {
	panic("unexpected Create call")
}
func (wireProxyRepoStub) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	panic("unexpected GetByID call")
}
func (wireProxyRepoStub) ListByIDs(ctx context.Context, ids []int64) ([]Proxy, error) {
	panic("unexpected ListByIDs call")
}
func (wireProxyRepoStub) Update(ctx context.Context, proxy *Proxy) error {
	panic("unexpected Update call")
}
func (wireProxyRepoStub) Delete(ctx context.Context, id int64) error { panic("unexpected Delete call") }
func (wireProxyRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (wireProxyRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]Proxy, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (wireProxyRepoStub) ListWithFiltersAndAccountCount(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFiltersAndAccountCount call")
}
func (wireProxyRepoStub) ListActive(ctx context.Context) ([]Proxy, error) {
	panic("unexpected ListActive call")
}
func (wireProxyRepoStub) ListActiveWithAccountCount(ctx context.Context) ([]ProxyWithAccountCount, error) {
	panic("unexpected ListActiveWithAccountCount call")
}
func (wireProxyRepoStub) ExistsByHostPortAuth(ctx context.Context, host string, port int, username, password string) (bool, error) {
	panic("unexpected ExistsByHostPortAuth call")
}
func (wireProxyRepoStub) CountAccountsByProxyID(ctx context.Context, proxyID int64) (int64, error) {
	panic("unexpected CountAccountsByProxyID call")
}
func (wireProxyRepoStub) ListAccountSummariesByProxyID(ctx context.Context, proxyID int64) ([]ProxyAccountSummary, error) {
	panic("unexpected ListAccountSummariesByProxyID call")
}
func (wireProxyRepoStub) SweepExpiredProxies(ctx context.Context, now time.Time) (int64, error) {
	panic("unexpected SweepExpiredProxies call")
}
func (wireProxyRepoStub) ListAllForFallback(ctx context.Context) ([]Proxy, error) {
	panic("unexpected ListAllForFallback call")
}
func (wireProxyRepoStub) CountExpired(ctx context.Context) (int64, error) {
	panic("unexpected CountExpired call")
}
func (wireProxyRepoStub) CountExpiringSoon(ctx context.Context, now time.Time) (int64, error) {
	panic("unexpected CountExpiringSoon call")
}

func TestProvideTokenRefreshService_InjectsKiroProxyRepo(t *testing.T) {
	proxyRepo := &wireProxyRepoStub{}

	svc := ProvideTokenRefreshService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		&config.Config{},
		nil,
		nil,
		proxyRepo,
		nil,
		nil,
		nil,
	)

	require.NotNil(t, svc)
	require.NotNil(t, svc.kiroRefresher)
	storedProxyRepo, ok := svc.kiroRefresher.proxyRepo.(*wireProxyRepoStub)
	require.True(t, ok)
	require.Same(t, proxyRepo, storedProxyRepo)
}
