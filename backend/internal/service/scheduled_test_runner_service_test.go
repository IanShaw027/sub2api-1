package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scheduledTestRunnerPlanRepoStub struct {
	ScheduledTestPlanRepository
	disabledPlanIDs     []int64
	updateAfterRunCalls []int64
}

func (s *scheduledTestRunnerPlanRepoStub) Disable(ctx context.Context, id int64) error {
	s.disabledPlanIDs = append(s.disabledPlanIDs, id)
	return nil
}

func (s *scheduledTestRunnerPlanRepoStub) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	s.updateAfterRunCalls = append(s.updateAfterRunCalls, id)
	return nil
}

type scheduledTestRunnerResultRepoStub struct {
	ScheduledTestResultRepository
	created []*ScheduledTestResult
}

func (s *scheduledTestRunnerResultRepoStub) Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error) {
	s.created = append(s.created, result)
	return result, nil
}

func (s *scheduledTestRunnerResultRepoStub) PruneOldResults(ctx context.Context, planID int64, keepCount int) error {
	return nil
}

type scheduledTestRunnerAccountRepoNotFoundStub struct {
	AccountRepository
}

func (s *scheduledTestRunnerAccountRepoNotFoundStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	return nil, ErrAccountNotFound
}

type scheduledTestRunnerAccountRepoStaticStub struct {
	AccountRepository
	account *Account
	err     error
}

func (s *scheduledTestRunnerAccountRepoStaticStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	return s.account, s.err
}

func TestScheduledTestRunner_DisablesPlanWhenAccountNotFound(t *testing.T) {
	planRepo := &scheduledTestRunnerPlanRepoStub{}
	resultRepo := &scheduledTestRunnerResultRepoStub{}
	accountTestSvc := NewAccountTestService(
		&scheduledTestRunnerAccountRepoNotFoundStub{},
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
	)
	runner := NewScheduledTestRunnerService(
		planRepo,
		NewScheduledTestService(planRepo, resultRepo),
		accountTestSvc,
		nil,
		nil,
	)

	runner.runOnePlan(context.Background(), &ScheduledTestPlan{
		ID:             1,
		AccountID:      203,
		ModelID:        "claude-opus-4-6",
		CronExpression: "*/30 * * * *",
		Enabled:        true,
		MaxResults:     100,
	})

	require.Equal(t, []int64{1}, planRepo.disabledPlanIDs)
	require.Empty(t, planRepo.updateAfterRunCalls)
	require.Len(t, resultRepo.created, 1)
	require.Equal(t, "failed", resultRepo.created[0].Status)
	require.Equal(t, "Account not found", resultRepo.created[0].ErrorMessage)
}

func TestScheduledTestRunner_SkipsUnschedulableAccountWithoutSavingFailure(t *testing.T) {
	planRepo := &scheduledTestRunnerPlanRepoStub{}
	resultRepo := &scheduledTestRunnerResultRepoStub{}
	accountRepo := &scheduledTestRunnerAccountRepoStaticStub{
		account: &Account{
			ID:          67220,
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: false,
		},
	}
	accountTestSvc := NewAccountTestService(
		accountRepo,
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
	)
	runner := NewScheduledTestRunnerService(
		planRepo,
		NewScheduledTestService(planRepo, resultRepo),
		accountTestSvc,
		nil,
		nil,
	)

	runner.runOnePlan(context.Background(), &ScheduledTestPlan{
		ID:             2,
		AccountID:      67220,
		ModelID:        "gpt-5.4-mini",
		CronExpression: "*/1 * * * *",
		Enabled:        true,
		MaxResults:     100,
	})

	require.Empty(t, resultRepo.created, "skip should not save a failed scheduled-test result")
	require.Equal(t, []int64{2}, planRepo.updateAfterRunCalls, "skip should still advance next_run_at")
	require.Empty(t, planRepo.disabledPlanIDs)
}

func TestScheduledTestRunner_AccountNotFoundDetectionUsesSentinelError(t *testing.T) {
	require.True(t, isScheduledTestAccountNotFound(
		&ScheduledTestResult{Status: "failed", ErrorMessage: "Account not found"},
		ErrAccountNotFound,
	))
	require.False(t, isScheduledTestAccountNotFound(
		&ScheduledTestResult{Status: "failed", ErrorMessage: "upstream says account not found"},
		errors.New("upstream says account not found"),
	))
}
