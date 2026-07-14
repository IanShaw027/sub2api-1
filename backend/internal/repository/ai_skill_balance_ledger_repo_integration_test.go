//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAISkillBalanceLedgerRepository_ChargeAndRefundAreIdempotent(t *testing.T) {
	ctx := context.Background()
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email:        fmt.Sprintf("skill-ledger-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      100,
		Concurrency:  5,
	})
	t.Cleanup(func() { _ = integrationEntClient.User.DeleteOneID(user.ID).Exec(context.Background()) })
	repo := NewAISkillBalanceLedgerRepository(integrationDB)
	charge := service.AISkillBalanceChargeInput{
		UserID: user.ID, Amount: 12.5, Reference: fmt.Sprintf("ai_skill_run:%d", user.ID),
	}

	first, err := repo.ApplyAISkillBalanceCharge(ctx, charge)
	require.NoError(t, err)
	require.False(t, first.Duplicate)
	require.InDelta(t, 100, first.BalanceBefore, 1e-9)
	require.InDelta(t, 87.5, first.BalanceAfter, 1e-9)
	second, err := repo.ApplyAISkillBalanceCharge(ctx, charge)
	require.NoError(t, err)
	require.True(t, second.Duplicate)
	require.InDelta(t, first.BalanceAfter, second.BalanceAfter, 1e-9)
	require.InDelta(t, 87.5, querySingleFloat(t, ctx, integrationEntClient,
		"SELECT balance::double precision FROM users WHERE id = $1", user.ID), 1e-9)

	refund := service.AISkillBalanceRefundInput{
		UserID: user.ID, Amount: charge.Amount, Reference: charge.Reference,
	}
	refunded, err := repo.ApplyAISkillBalanceRefund(ctx, refund)
	require.NoError(t, err)
	require.False(t, refunded.Duplicate)
	require.InDelta(t, 100, refunded.BalanceAfter, 1e-9)
	refundedAgain, err := repo.ApplyAISkillBalanceRefund(ctx, refund)
	require.NoError(t, err)
	require.True(t, refundedAgain.Duplicate)
	require.InDelta(t, 100, querySingleFloat(t, ctx, integrationEntClient,
		"SELECT balance::double precision FROM users WHERE id = $1", user.ID), 1e-9)

	afterRefund, err := repo.ApplyAISkillBalanceCharge(ctx, charge)
	require.NoError(t, err)
	require.True(t, afterRefund.Duplicate)
	require.True(t, afterRefund.Refunded)
	require.Equal(t, 2, querySingleInt(t, ctx, integrationEntClient,
		"SELECT COUNT(*) FROM ai_skill_balance_ledger WHERE reference = $1", charge.Reference))
}

func TestAISkillBalanceLedgerRepository_ConcurrentChargeMutatesBalanceOnce(t *testing.T) {
	ctx := context.Background()
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email:        fmt.Sprintf("skill-ledger-concurrent-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      100,
		Concurrency:  5,
	})
	t.Cleanup(func() { _ = integrationEntClient.User.DeleteOneID(user.ID).Exec(context.Background()) })
	repo := NewAISkillBalanceLedgerRepository(integrationDB)
	input := service.AISkillBalanceChargeInput{
		UserID: user.ID, Amount: 7.25, Reference: fmt.Sprintf("ai_skill_run:concurrent:%d", user.ID),
	}

	const callers = 16
	start := make(chan struct{})
	results := make(chan *service.AISkillBalanceLedgerResult, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := repo.ApplyAISkillBalanceCharge(ctx, input)
			results <- result
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	nonDuplicates := 0
	for result := range results {
		require.NotNil(t, result)
		if !result.Duplicate {
			nonDuplicates++
		}
	}
	require.Equal(t, 1, nonDuplicates)
	require.InDelta(t, 92.75, querySingleFloat(t, ctx, integrationEntClient,
		"SELECT balance::double precision FROM users WHERE id = $1", user.ID), 1e-9)
	require.Equal(t, 1, querySingleInt(t, ctx, integrationEntClient,
		"SELECT COUNT(*) FROM ai_skill_balance_ledger WHERE reference = $1 AND operation = 'charge'", input.Reference))
}
