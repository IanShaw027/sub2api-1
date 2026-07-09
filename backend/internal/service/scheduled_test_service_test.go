//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scheduledTestPlanRepoStub struct{}

func (scheduledTestPlanRepoStub) Create(_ context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	return plan, nil
}

func (scheduledTestPlanRepoStub) GetByID(_ context.Context, _ int64) (*ScheduledTestPlan, error) {
	return nil, nil
}

func (scheduledTestPlanRepoStub) ListByAccountID(_ context.Context, _ int64) ([]*ScheduledTestPlan, error) {
	return nil, nil
}

func (scheduledTestPlanRepoStub) ListDue(_ context.Context, _ time.Time) ([]*ScheduledTestPlan, error) {
	return nil, nil
}

func (scheduledTestPlanRepoStub) Update(_ context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	return plan, nil
}

func (scheduledTestPlanRepoStub) Delete(_ context.Context, _ int64) error {
	return nil
}

func (scheduledTestPlanRepoStub) Disable(_ context.Context, _ int64) error {
	return nil
}

func (scheduledTestPlanRepoStub) UpdateAfterRun(_ context.Context, _ int64, _ time.Time, _ time.Time) error {
	return nil
}

type scheduledTestResultRepoStub struct{}

func (scheduledTestResultRepoStub) Create(_ context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error) {
	return result, nil
}

func (scheduledTestResultRepoStub) ListByPlanID(_ context.Context, _ int64, _ int) ([]*ScheduledTestResult, error) {
	return nil, nil
}

func (scheduledTestResultRepoStub) PruneOldResults(_ context.Context, _ int64, _ int) error {
	return nil
}

func TestScheduledTestServiceCreatePlanRejectsNilPlan(t *testing.T) {
	svc := NewScheduledTestService(scheduledTestPlanRepoStub{}, scheduledTestResultRepoStub{})

	_, err := svc.CreatePlan(context.Background(), nil)

	require.EqualError(t, err, "scheduled test plan is required")
}

func TestScheduledTestServiceUpdatePlanRejectsNilPlan(t *testing.T) {
	svc := NewScheduledTestService(scheduledTestPlanRepoStub{}, scheduledTestResultRepoStub{})

	_, err := svc.UpdatePlan(context.Background(), nil)

	require.EqualError(t, err, "scheduled test plan is required")
}

func TestScheduledTestServiceSaveResultRejectsNilResult(t *testing.T) {
	svc := NewScheduledTestService(scheduledTestPlanRepoStub{}, scheduledTestResultRepoStub{})

	err := svc.SaveResult(context.Background(), 1, 10, nil)

	require.EqualError(t, err, "scheduled test result is required")
}
