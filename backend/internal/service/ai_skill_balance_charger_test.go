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

type aiSkillBalanceLedgerStub struct {
	chargeCalls int
	refundCalls int
	balance     float64
}

func (s *aiSkillBalanceLedgerStub) ApplyAISkillBalanceCharge(_ context.Context, input AISkillBalanceChargeInput) (*AISkillBalanceLedgerResult, error) {
	s.chargeCalls++
	before := s.balance
	s.balance -= input.Amount
	return &AISkillBalanceLedgerResult{Amount: input.Amount, BalanceBefore: before, BalanceAfter: s.balance}, nil
}

func (s *aiSkillBalanceLedgerStub) ApplyAISkillBalanceRefund(_ context.Context, input AISkillBalanceRefundInput) (*AISkillBalanceLedgerResult, error) {
	s.refundCalls++
	before := s.balance
	s.balance += input.Amount
	return &AISkillBalanceLedgerResult{Amount: input.Amount, BalanceBefore: before, BalanceAfter: s.balance}, nil
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
	ledger := &aiSkillBalanceLedgerStub{balance: 10}
	charger := &aiSkillBalanceCharger{ledgerRepo: ledger, userRepo: repo}

	result, err := charger.RefundUserBalance(t.Context(), AISkillBalanceRefundInput{
		UserID:    7,
		Amount:    3.5,
		Reference: "ai_skill_run:7",
	})

	require.NoError(t, err)
	require.Empty(t, repo.refundCalls)
	require.Empty(t, repo.updateBalanceCalls)
	require.Equal(t, 1, ledger.refundCalls)
	require.InDelta(t, 3.5, result.RefundedAmount, 1e-9)
	require.InDelta(t, 13.5, result.BalanceAfter, 1e-9)
}
