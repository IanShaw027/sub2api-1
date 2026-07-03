//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type aiSkillRefundUserRepo struct {
	mockUserRepo
	balance            float64
	refundCalls        []float64
	updateBalanceCalls []float64
}

func (r *aiSkillRefundUserRepo) AddBalanceWithoutRecharge(_ context.Context, _ int64, amount float64) error {
	r.refundCalls = append(r.refundCalls, amount)
	r.balance += amount
	return nil
}

func (r *aiSkillRefundUserRepo) UpdateBalance(_ context.Context, _ int64, amount float64) error {
	r.updateBalanceCalls = append(r.updateBalanceCalls, amount)
	return nil
}

func (r *aiSkillRefundUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	return &User{ID: id, Balance: r.balance}, nil
}

func TestAISkillBalanceChargerRefundDoesNotUseRechargeBalancePath(t *testing.T) {
	repo := &aiSkillRefundUserRepo{balance: 10}
	charger := &aiSkillBalanceCharger{userRepo: repo}

	result, err := charger.RefundUserBalance(t.Context(), AISkillBalanceRefundInput{
		UserID: 7,
		Amount: 3.5,
	})

	require.NoError(t, err)
	require.Equal(t, []float64{3.5}, repo.refundCalls)
	require.Empty(t, repo.updateBalanceCalls)
	require.InDelta(t, 3.5, result.RefundedAmount, 1e-9)
	require.InDelta(t, 13.5, result.BalanceAfter, 1e-9)
}
