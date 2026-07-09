//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type promoRepoStub struct {
	getByIDFn func(context.Context, int64) (*PromoCode, error)
}

func (s *promoRepoStub) Create(context.Context, *PromoCode) error { panic("not implemented") }

func (s *promoRepoStub) GetByID(ctx context.Context, id int64) (*PromoCode, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	panic("not implemented")
}

func (s *promoRepoStub) GetByCode(context.Context, string) (*PromoCode, error) {
	panic("not implemented")
}

func (s *promoRepoStub) GetByCodeForUpdate(context.Context, string) (*PromoCode, error) {
	panic("not implemented")
}

func (s *promoRepoStub) Update(context.Context, *PromoCode) error { panic("not implemented") }

func (s *promoRepoStub) Delete(context.Context, int64) error { panic("not implemented") }

func (s *promoRepoStub) List(context.Context, pagination.PaginationParams) ([]PromoCode, *pagination.PaginationResult, error) {
	panic("not implemented")
}

func (s *promoRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string) ([]PromoCode, *pagination.PaginationResult, error) {
	panic("not implemented")
}

func (s *promoRepoStub) CreateUsage(context.Context, *PromoCodeUsage) error { panic("not implemented") }

func (s *promoRepoStub) GetUsageByPromoCodeAndUser(context.Context, int64, int64) (*PromoCodeUsage, error) {
	panic("not implemented")
}

func (s *promoRepoStub) ListUsagesByPromoCode(context.Context, int64, pagination.PaginationParams) ([]PromoCodeUsage, *pagination.PaginationResult, error) {
	panic("not implemented")
}

func (s *promoRepoStub) IncrementUsedCount(context.Context, int64) error { panic("not implemented") }

func TestPromoServiceCreateRejectsNilInput(t *testing.T) {
	svc := NewPromoService(&promoRepoStub{}, nil, nil, nil, nil)

	_, err := svc.Create(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "promo input is required")
}

func TestPromoServiceUpdateRejectsNilInput(t *testing.T) {
	svc := NewPromoService(&promoRepoStub{
		getByIDFn: func(context.Context, int64) (*PromoCode, error) {
			return &PromoCode{ID: 1, Code: "PROMO1", Status: PromoCodeStatusActive}, nil
		},
	}, nil, nil, nil, nil)

	_, err := svc.Update(context.Background(), 1, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "promo input is required")
}
