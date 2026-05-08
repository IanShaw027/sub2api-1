package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type redeemStatsRepoStub struct {
	stats *RedeemCodeStats
	err   error
	calls int
}

func (s *redeemStatsRepoStub) Create(context.Context, *RedeemCode) error {
	panic("unexpected Create call")
}
func (s *redeemStatsRepoStub) CreateBatch(context.Context, []RedeemCode) error {
	panic("unexpected CreateBatch call")
}
func (s *redeemStatsRepoStub) GetByID(context.Context, int64) (*RedeemCode, error) {
	panic("unexpected GetByID call")
}
func (s *redeemStatsRepoStub) GetByCode(context.Context, string) (*RedeemCode, error) {
	panic("unexpected GetByCode call")
}
func (s *redeemStatsRepoStub) Update(context.Context, *RedeemCode) error {
	panic("unexpected Update call")
}
func (s *redeemStatsRepoStub) Delete(context.Context, int64) error { panic("unexpected Delete call") }
func (s *redeemStatsRepoStub) Use(context.Context, int64, int64) error {
	panic("unexpected Use call")
}
func (s *redeemStatsRepoStub) List(context.Context, pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *redeemStatsRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (s *redeemStatsRepoStub) ListByUser(context.Context, int64, int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}
func (s *redeemStatsRepoStub) ListByUserPaginated(context.Context, int64, pagination.PaginationParams, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}
func (s *redeemStatsRepoStub) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}
func (s *redeemStatsRepoStub) GetStats(context.Context) (*RedeemCodeStats, error) {
	s.calls++
	return s.stats, s.err
}

func TestRedeemService_GetStats(t *testing.T) {
	repo := &redeemStatsRepoStub{stats: &RedeemCodeStats{
		TotalCodes:            5,
		UnusedCodes:           2,
		UsedCodes:             2,
		ExpiredCodes:          1,
		TotalValue:            145.5,
		TotalValueDistributed: 120.5,
		ByType: map[string]int64{
			RedeemTypeBalance:      3,
			RedeemTypeConcurrency:  1,
			RedeemTypeSubscription: 1,
		},
	}}
	svc := NewRedeemService(repo, nil, nil, nil, nil, nil, nil, nil)

	stats, err := svc.GetStats(context.Background())

	require.NoError(t, err)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, int64(5), stats["total_codes"])
	require.Equal(t, int64(2), stats["unused_codes"])
	require.Equal(t, int64(2), stats["used_codes"])
	require.Equal(t, float64(145.5), stats["total_value"])
	require.Equal(t, int64(2), stats["active_codes"])
	require.Equal(t, int64(1), stats["expired_codes"])
	require.Equal(t, float64(120.5), stats["total_value_distributed"])
	require.Equal(t, map[string]int64{
		RedeemTypeBalance:      3,
		RedeemTypeConcurrency:  1,
		RedeemTypeSubscription: 1,
	}, stats["by_type"])
}
