package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scheduledTestAccountRepoStub struct {
	AccountRepository
	account *Account
}

func (s *scheduledTestAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if s.account == nil {
		return nil, ErrAccountNotFound
	}
	return s.account, nil
}

type scheduledTestPlanRepoStub struct {
	ScheduledTestPlanRepository
	updatedIDs  []int64
	disabledIDs []int64
}

func (s *scheduledTestPlanRepoStub) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	s.updatedIDs = append(s.updatedIDs, id)
	return nil
}

func (s *scheduledTestPlanRepoStub) Disable(ctx context.Context, id int64) error {
	s.disabledIDs = append(s.disabledIDs, id)
	return nil
}

type scheduledTestResultRepoStub struct {
	ScheduledTestResultRepository
	created []*ScheduledTestResult
}

func (s *scheduledTestResultRepoStub) Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error) {
	s.created = append(s.created, result)
	return result, nil
}

func (s *scheduledTestResultRepoStub) PruneOldResults(ctx context.Context, planID int64, keepCount int) error {
	return nil
}

func newScheduledTestRunnerForSkip(account *Account, autoRecover bool) (*ScheduledTestRunnerService, *scheduledTestPlanRepoStub) {
	planRepo := &scheduledTestPlanRepoStub{}
	return &ScheduledTestRunnerService{
		planRepo: planRepo,
		accountTestSvc: &AccountTestService{
			accountRepo: &scheduledTestAccountRepoStub{account: account},
		},
	}, planRepo
}

func TestScheduledTestRunner_SkipsDisabledAccount(t *testing.T) {
	runner, planRepo := newScheduledTestRunnerForSkip(&Account{
		ID:          7,
		Status:      StatusDisabled,
		Schedulable: true,
	}, false)

	runner.runOnePlan(context.Background(), &ScheduledTestPlan{
		ID:             3,
		AccountID:      7,
		CronExpression: "0 * * * *",
	})

	require.Equal(t, []int64{3}, planRepo.updatedIDs)
}

func TestScheduledTestRunner_SkipsManuallyUnschedulableAccount(t *testing.T) {
	runner, planRepo := newScheduledTestRunnerForSkip(&Account{
		ID:          8,
		Status:      StatusActive,
		Schedulable: false,
	}, false)

	runner.runOnePlan(context.Background(), &ScheduledTestPlan{
		ID:             4,
		AccountID:      8,
		CronExpression: "0 * * * *",
		AutoRecover:    true,
	})

	require.Equal(t, []int64{4}, planRepo.updatedIDs)
}

func TestScheduledTestRunner_DoesNotSkipSchedulableAccountInSkipCheck(t *testing.T) {
	runner, _ := newScheduledTestRunnerForSkip(&Account{
		ID:          9,
		Status:      StatusActive,
		Schedulable: true,
	}, false)

	require.False(t, runner.shouldSkipPlanForUnschedulableAccount(context.Background(), &ScheduledTestPlan{
		ID:        5,
		AccountID: 9,
	}))
}

func TestScheduledTestRunner_AutoRecoverTestsErrorAccount(t *testing.T) {
	runner, _ := newScheduledTestRunnerForSkip(&Account{
		ID:          10,
		Status:      StatusError,
		Schedulable: false,
	}, true)

	require.False(t, runner.shouldSkipPlanForUnschedulableAccount(context.Background(), &ScheduledTestPlan{
		ID:          6,
		AccountID:   10,
		AutoRecover: true,
	}))
}

func TestScheduledTestRunner_DisablesPlanWhenAccountNotFound(t *testing.T) {
	planRepo := &scheduledTestPlanRepoStub{}
	resultRepo := &scheduledTestResultRepoStub{}
	accountTestSvc := NewAccountTestService(
		&scheduledTestAccountRepoStub{},
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

	require.Equal(t, []int64{1}, planRepo.disabledIDs)
	require.Empty(t, planRepo.updatedIDs)
	require.Len(t, resultRepo.created, 1)
	require.Equal(t, "failed", resultRepo.created[0].Status)
	require.Equal(t, "Account not found", resultRepo.created[0].ErrorMessage)
}
