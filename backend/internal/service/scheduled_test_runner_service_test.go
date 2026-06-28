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
