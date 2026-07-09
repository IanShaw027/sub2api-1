//go:build unit

package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/integration/skillrunner"
	"github.com/stretchr/testify/require"
)

type aiSkillStoreStub struct {
	nextSkillID      int64
	nextVersionID    int64
	nextRunID        int64
	nextSettlementID int64
	nextReviewID     int64

	skills      map[int64]*AISkill
	versions    map[int64]*AISkillVersion
	runs        map[int64]*AISkillRun
	settlements map[int64]*AISkillSettlement
	reviews     []AISkillReview
}

func newAISkillStoreStub() *aiSkillStoreStub {
	return &aiSkillStoreStub{
		nextSkillID:      1,
		nextVersionID:    1,
		nextRunID:        1,
		nextSettlementID: 1,
		nextReviewID:     1,
		skills:           map[int64]*AISkill{},
		versions:         map[int64]*AISkillVersion{},
		runs:             map[int64]*AISkillRun{},
		settlements:      map[int64]*AISkillSettlement{},
	}
}

func (s *aiSkillStoreStub) CreateSkill(_ context.Context, skill *AISkill) error {
	copy := cloneAISkillEntity(skill)
	copy.ID = s.nextSkillID
	s.nextSkillID++
	*skill = *cloneAISkillEntity(copy)
	s.skills[copy.ID] = copy
	return nil
}

func (s *aiSkillStoreStub) GetSkillByID(_ context.Context, id int64) (*AISkill, error) {
	skill, ok := s.skills[id]
	if !ok {
		return nil, ErrAISkillNotFound
	}
	return cloneAISkillEntity(skill), nil
}

func (s *aiSkillStoreStub) GetSkillByCreatorAndID(_ context.Context, creatorUserID, id int64) (*AISkill, error) {
	skill, ok := s.skills[id]
	if !ok || skill.CreatorUserID != creatorUserID {
		return nil, ErrAISkillNotFound
	}
	return cloneAISkillEntity(skill), nil
}

func (s *aiSkillStoreStub) UpdateSkill(_ context.Context, skill *AISkill) error {
	if _, ok := s.skills[skill.ID]; !ok {
		return ErrAISkillNotFound
	}
	s.skills[skill.ID] = cloneAISkillEntity(skill)
	return nil
}

func (s *aiSkillStoreStub) CreateVersion(_ context.Context, version *AISkillVersion) error {
	copy := cloneAISkillVersionEntity(version)
	copy.ID = s.nextVersionID
	s.nextVersionID++
	*version = *cloneAISkillVersionEntity(copy)
	s.versions[copy.ID] = copy
	return nil
}

func (s *aiSkillStoreStub) GetVersionByID(_ context.Context, id int64) (*AISkillVersion, error) {
	version, ok := s.versions[id]
	if !ok {
		return nil, ErrAISkillVersionNotFound
	}
	return cloneAISkillVersionEntity(version), nil
}

func (s *aiSkillStoreStub) GetVersionByCreatorAndID(_ context.Context, creatorUserID, id int64) (*AISkillVersion, error) {
	version, ok := s.versions[id]
	if !ok || version.CreatorUserID != creatorUserID {
		return nil, ErrAISkillVersionNotFound
	}
	return cloneAISkillVersionEntity(version), nil
}

func (s *aiSkillStoreStub) GetLatestApprovedVersionBySkillID(_ context.Context, skillID int64) (*AISkillVersion, error) {
	var best *AISkillVersion
	for _, version := range s.versions {
		if version.SkillID != skillID || normalizeAISkillVersionStatus(version.Status) != AISkillVersionStatusApproved {
			continue
		}
		if best == nil || version.Version > best.Version {
			best = version
		}
	}
	if best == nil {
		return nil, ErrAISkillVersionNotFound
	}
	return cloneAISkillVersionEntity(best), nil
}

func (s *aiSkillStoreStub) UpdateVersion(_ context.Context, version *AISkillVersion) error {
	if _, ok := s.versions[version.ID]; !ok {
		return ErrAISkillVersionNotFound
	}
	s.versions[version.ID] = cloneAISkillVersionEntity(version)
	return nil
}

func (s *aiSkillStoreStub) CreateReview(_ context.Context, review *AISkillReview) error {
	copy := *review
	copy.ID = s.nextReviewID
	s.nextReviewID++
	copy.Metadata = cloneAIMap(review.Metadata)
	s.reviews = append(s.reviews, copy)
	return nil
}

func (s *aiSkillStoreStub) CreateRun(_ context.Context, run *AISkillRun) error {
	copy := cloneAISkillRunEntity(run)
	copy.ID = s.nextRunID
	s.nextRunID++
	*run = *cloneAISkillRunEntity(copy)
	s.runs[copy.ID] = copy
	return nil
}

func (s *aiSkillStoreStub) GetRunByID(_ context.Context, id int64) (*AISkillRun, error) {
	run, ok := s.runs[id]
	if !ok {
		return nil, ErrAISkillRunNotFound
	}
	return cloneAISkillRunEntity(run), nil
}

func (s *aiSkillStoreStub) UpdateRun(_ context.Context, run *AISkillRun) error {
	if _, ok := s.runs[run.ID]; !ok {
		return ErrAISkillRunNotFound
	}
	s.runs[run.ID] = cloneAISkillRunEntity(run)
	return nil
}

func (s *aiSkillStoreStub) CreateSettlement(_ context.Context, settlement *AISkillSettlement) error {
	copy := cloneAISkillSettlementEntity(settlement)
	copy.ID = s.nextSettlementID
	s.nextSettlementID++
	*settlement = *cloneAISkillSettlementEntity(copy)
	s.settlements[copy.ID] = copy
	return nil
}

func (s *aiSkillStoreStub) GetSettlementByRunID(_ context.Context, runID int64) (*AISkillSettlement, error) {
	for _, settlement := range s.settlements {
		if settlement.RunID == runID {
			return cloneAISkillSettlementEntity(settlement), nil
		}
	}
	return nil, ErrAISkillSettlementNotFound
}

func (s *aiSkillStoreStub) UpdateSettlement(_ context.Context, settlement *AISkillSettlement) error {
	if _, ok := s.settlements[settlement.ID]; !ok {
		return ErrAISkillSettlementNotFound
	}
	s.settlements[settlement.ID] = cloneAISkillSettlementEntity(settlement)
	return nil
}

type aiSkillUniqueSettlementStoreStub struct {
	*aiSkillStoreStub
}

func newAISkillUniqueSettlementStoreStub() *aiSkillUniqueSettlementStoreStub {
	return &aiSkillUniqueSettlementStoreStub{aiSkillStoreStub: newAISkillStoreStub()}
}

func (s *aiSkillUniqueSettlementStoreStub) CreateSettlement(ctx context.Context, settlement *AISkillSettlement) error {
	if existing, err := s.GetSettlementByRunID(ctx, settlement.RunID); err == nil && existing != nil {
		return errors.New("ai skill settlement already exists for run")
	}
	return s.aiSkillStoreStub.CreateSettlement(ctx, settlement)
}

type aiSkillBalanceChargerStub struct {
	charges   []AISkillBalanceChargeInput
	refunds   []AISkillBalanceRefundInput
	result    *AISkillBalanceChargeResult
	err       error
	refundErr error
}

func (s *aiSkillBalanceChargerStub) ChargeUserBalance(_ context.Context, input AISkillBalanceChargeInput) (*AISkillBalanceChargeResult, error) {
	s.charges = append(s.charges, input)
	if s.err != nil {
		return nil, s.err
	}
	if s.result == nil {
		return &AISkillBalanceChargeResult{ChargedAmount: input.Amount}, nil
	}
	copy := *s.result
	return &copy, nil
}

func (s *aiSkillBalanceChargerStub) RefundUserBalance(_ context.Context, input AISkillBalanceRefundInput) (*AISkillBalanceRefundResult, error) {
	s.refunds = append(s.refunds, input)
	if s.refundErr != nil {
		return nil, s.refundErr
	}
	return &AISkillBalanceRefundResult{RefundedAmount: input.Amount}, nil
}

type aiSkillCreatorCreditorStub struct {
	inputs        []AISkillCreatorEarningsInput
	reversals     []AISkillCreatorEarningsReversalInput
	amount        float64
	err           error
	reverseAmount float64
	reverseErr    error
}

func (s *aiSkillCreatorCreditorStub) CreditCreatorEarnings(_ context.Context, input AISkillCreatorEarningsInput) (float64, error) {
	s.inputs = append(s.inputs, input)
	if s.err != nil {
		return 0, s.err
	}
	if s.amount == 0 {
		return input.Amount, nil
	}
	return s.amount, nil
}

func (s *aiSkillCreatorCreditorStub) ReverseCreatorEarnings(_ context.Context, input AISkillCreatorEarningsReversalInput) (float64, error) {
	s.reversals = append(s.reversals, input)
	if s.reverseErr != nil {
		return 0, s.reverseErr
	}
	if s.reverseAmount == 0 {
		return input.Amount, nil
	}
	return s.reverseAmount, nil
}

type aiSkillRuntimeGatewayStub struct {
	requests []AISkillExecutionRequest
	result   *AISkillDispatchResult
	err      error
}

func (s *aiSkillRuntimeGatewayStub) Execute(_ context.Context, req AISkillExecutionRequest) (*AISkillDispatchResult, error) {
	s.requests = append(s.requests, req)
	if s.err != nil {
		return nil, s.err
	}
	if s.result == nil {
		return &AISkillDispatchResult{Status: AISkillRunStatusDispatched}, nil
	}
	copy := *s.result
	copy.Output = cloneAIMap(s.result.Output)
	return &copy, nil
}

func TestAISkillVersionSubmitAndApproveWorkflow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	store := newAISkillStoreStub()
	skillSvc := NewAISkillService(store)
	skillSvc.now = func() time.Time { return now }
	versionSvc := NewAISkillVersionService(store, store, store)
	versionSvc.now = func() time.Time { return now.Add(1 * time.Minute) }
	reviewSvc := NewAISkillReviewService(store, store, store)
	reviewSvc.now = func() time.Time { return now.Add(2 * time.Minute) }

	skill, err := skillSvc.CreateSkill(ctx, 101, &AICreateSkillInput{
		Name: "Prompt Crafter",
		Type: AISkillTypePromptChat,
	})
	require.NoError(t, err)
	require.Equal(t, AISkillTypePromptChat, skill.Type)

	version, err := versionSvc.CreateVersion(ctx, 101, skill.ID, &AICreateSkillVersionInput{
		ExecutionSpec: AISkillExecutionSpec{
			Type: AISkillTypePromptChat,
			PromptChat: &AISkillPromptChatSpec{
				UserPromptTemplate: "Explain {{topic}}",
			},
		},
		BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
	})
	require.NoError(t, err)
	require.Equal(t, AISkillVersionStatusDraft, version.Status)
	require.Equal(t, 1, version.Version)

	submitted, err := versionSvc.SubmitVersion(ctx, 101, version.ID, &AISubmitSkillVersionInput{
		Comment: stringPtr("ready for review"),
	})
	require.NoError(t, err)
	require.Equal(t, AISkillVersionStatusSubmitted, submitted.Status)
	require.NotNil(t, submitted.SubmittedAt)

	approved, err := reviewSvc.ApproveVersion(ctx, 9001, version.ID, &AIReviewSkillVersionInput{
		Comment: stringPtr("approved"),
	})
	require.NoError(t, err)
	require.Equal(t, AISkillVersionStatusApproved, approved.Status)
	require.NotNil(t, approved.ApprovedAt)
	require.NotNil(t, approved.ReviewedAt)
	require.NotNil(t, approved.ReviewerUserID)
	require.Equal(t, int64(9001), *approved.ReviewerUserID)
	require.Len(t, store.reviews, 2)
	require.Equal(t, AISkillReviewActionSubmitted, store.reviews[0].Action)
	require.Equal(t, AISkillReviewActionApproved, store.reviews[1].Action)
}

func TestAISkillRunServiceRejectsNonApprovedVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillStoreStub()
	skill := &AISkill{
		ID:            1,
		CreatorUserID: 200,
		Name:          "Image Draft",
		Type:          AISkillTypePromptImage,
	}
	version := &AISkillVersion{
		ID:            1,
		SkillID:       skill.ID,
		CreatorUserID: skill.CreatorUserID,
		Version:       1,
		Type:          skill.Type,
		Status:        AISkillVersionStatusDraft,
		ExecutionSpec: AISkillExecutionSpec{
			Type: AISkillTypePromptImage,
			PromptImage: &AISkillPromptImageSpec{
				PromptTemplate: "render {{subject}}",
			},
		},
		BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
	}
	store.skills[skill.ID] = cloneAISkillEntity(skill)
	store.versions[version.ID] = cloneAISkillVersionEntity(version)

	settlementSvc := NewAISkillSettlementService(store, &aiSkillBalanceChargerStub{}, &aiSkillCreatorCreditorStub{})
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, nil)

	_, err := runSvc.Prepare(ctx, 300, &AISkillRunInput{
		SkillID:   skill.ID,
		VersionID: int64Ptr(version.ID),
		Mode:      AISkillRunModeUse,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAISkillVersionNotApproved)
}

func TestAISkillRunServiceExecutePerRunSettlesAndDispatches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillStoreStub()
	skill := &AISkill{
		ID:            1,
		CreatorUserID: 700,
		Name:          "Chat Skill",
		Type:          AISkillTypePromptChat,
	}
	version := &AISkillVersion{
		ID:            1,
		SkillID:       skill.ID,
		CreatorUserID: skill.CreatorUserID,
		Version:       2,
		Type:          skill.Type,
		Status:        AISkillVersionStatusApproved,
		ExecutionSpec: AISkillExecutionSpec{
			Type: AISkillTypePromptChat,
			PromptChat: &AISkillPromptChatSpec{
				Model:              "gpt-5.4",
				UserPromptTemplate: "Summarize {{topic}}",
			},
		},
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            12.5,
			PlatformCommissionRate: 0.2,
		},
	}
	store.skills[skill.ID] = cloneAISkillEntity(skill)
	store.versions[version.ID] = cloneAISkillVersionEntity(version)

	balance := &aiSkillBalanceChargerStub{
		result: &AISkillBalanceChargeResult{ChargedAmount: 12.5, BalanceAfter: 87.5},
	}
	creator := &aiSkillCreatorCreditorStub{}
	settlementSvc := NewAISkillSettlementService(store, balance, creator)
	runtime := &aiSkillRuntimeGatewayStub{
		result: &AISkillDispatchResult{
			Status:        AISkillRunStatusSucceeded,
			Provider:      "openai",
			ExternalJobID: "resp_123",
			Output:        map[string]any{"answer": "done"},
		},
	}
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, runtime)

	result, err := runSvc.Execute(ctx, 900, &AISkillRunInput{
		SkillID:    skill.ID,
		VersionID:  int64Ptr(version.ID),
		Mode:       AISkillRunModeUse,
		Parameters: map[string]any{"topic": "golang"},
		Metadata:   map[string]any{"scene": "unit"},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Prepared)
	require.NotNil(t, result.Dispatch)
	require.Equal(t, AISkillRunStatusSucceeded, result.Prepared.Run.Status)
	require.Equal(t, 12.5, result.Prepared.Settlement.TotalAmount)
	require.Equal(t, 2.5, result.Prepared.Settlement.PlatformAmount)
	require.Equal(t, 10.0, result.Prepared.Settlement.CreatorAmount)
	require.Equal(t, 10.0, result.Prepared.Settlement.CreatorCreditedAmount)
	require.Len(t, balance.charges, 1)
	require.Len(t, creator.inputs, 1)
	require.Equal(t, 10.0, creator.inputs[0].Amount)
	require.Len(t, runtime.requests, 1)
	require.NotNil(t, runtime.requests[0].PromptChat)
	require.Equal(t, "Summarize {{topic}}", runtime.requests[0].PromptChat.UserPromptTemplate)
	require.Equal(t, "openai", result.Prepared.Run.Provider)
	require.Equal(t, "resp_123", result.Prepared.Run.ExternalJobID)
}

func TestAISkillRunServiceExecuteFailedDispatchDoesNotCharge(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillStoreStub()
	skill := &AISkill{
		ID:            2,
		CreatorUserID: 701,
		Name:          "Failed Chat Skill",
		Type:          AISkillTypePromptChat,
	}
	version := &AISkillVersion{
		ID:            2,
		SkillID:       skill.ID,
		CreatorUserID: skill.CreatorUserID,
		Version:       1,
		Type:          skill.Type,
		Status:        AISkillVersionStatusApproved,
		ExecutionSpec: AISkillExecutionSpec{
			Type: AISkillTypePromptChat,
			PromptChat: &AISkillPromptChatSpec{
				UserPromptTemplate: "Summarize {{topic}}",
			},
		},
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            9.9,
			PlatformCommissionRate: 0.1,
		},
	}
	store.skills[skill.ID] = cloneAISkillEntity(skill)
	store.versions[version.ID] = cloneAISkillVersionEntity(version)

	balance := &aiSkillBalanceChargerStub{}
	creator := &aiSkillCreatorCreditorStub{}
	settlementSvc := NewAISkillSettlementService(store, balance, creator)
	runtime := &aiSkillRuntimeGatewayStub{
		result: &AISkillDispatchResult{
			Status:        AISkillRunStatusFailed,
			Provider:      "openai",
			ExternalJobID: "resp_failed",
		},
	}
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, runtime)

	result, err := runSvc.Execute(ctx, 901, &AISkillRunInput{
		SkillID:    skill.ID,
		VersionID:  int64Ptr(version.ID),
		Mode:       AISkillRunModeUse,
		Parameters: map[string]any{"topic": "golang"},
	})
	require.Error(t, err)
	require.Nil(t, result)
	require.Len(t, runtime.requests, 1)
	require.Empty(t, balance.charges)
	require.Empty(t, creator.inputs)
	require.Empty(t, store.settlements)

	run := store.runs[1]
	require.NotNil(t, run)
	require.Equal(t, AISkillRunStatusFailed, run.Status)
	require.Nil(t, run.SettlementID)
	require.Equal(t, 0.0, run.ChargeAmount)
	require.Equal(t, "openai", run.Provider)
	require.Equal(t, "resp_failed", run.ExternalJobID)
}

func TestAISkillSettlementRefundsBuyerWhenCreatorCreditFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillStoreStub()
	balance := &aiSkillBalanceChargerStub{
		result: &AISkillBalanceChargeResult{ChargedAmount: 12.5, BalanceAfter: 87.5},
	}
	creator := &aiSkillCreatorCreditorStub{err: errors.New("credit down")}
	settlementSvc := NewAISkillSettlementService(store, balance, creator)

	settlement, err := settlementSvc.Settle(ctx, AISkillSettleInput{
		RunID:         3001,
		SkillID:       1001,
		VersionID:     2001,
		BuyerUserID:   900,
		CreatorUserID: 700,
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            12.5,
			PlatformCommissionRate: 0.2,
		},
		Metadata: map[string]any{"scene": "creator-fail"},
	})

	require.Error(t, err)
	require.Nil(t, settlement)
	require.Len(t, balance.charges, 1)
	require.Len(t, balance.refunds, 1)
	require.Equal(t, int64(900), balance.refunds[0].UserID)
	require.Equal(t, 12.5, balance.refunds[0].Amount)
	require.Equal(t, "ai_skill_run:3001", balance.refunds[0].Reference)
	require.Len(t, store.settlements, 1)
	for _, stored := range store.settlements {
		require.Equal(t, AISkillSettlementStatusFailed, stored.Status)
		require.Contains(t, stored.FailureReason, "credit down")
	}
}

func TestAISkillSettlementReversesCreatorEarningsWhenCreditedAmountMismatches(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillStoreStub()
	balance := &aiSkillBalanceChargerStub{
		result: &AISkillBalanceChargeResult{ChargedAmount: 12.5, BalanceAfter: 87.5},
	}
	creator := &aiSkillCreatorCreditorStub{amount: 9.5}
	settlementSvc := NewAISkillSettlementService(store, balance, creator)

	settlement, err := settlementSvc.Settle(ctx, AISkillSettleInput{
		RunID:         3002,
		SkillID:       1002,
		VersionID:     2002,
		BuyerUserID:   901,
		CreatorUserID: 701,
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            12.5,
			PlatformCommissionRate: 0.2,
		},
		Metadata: map[string]any{"scene": "creator-mismatch"},
	})

	require.ErrorIs(t, err, ErrAISkillCreatorEarningsMismatch)
	require.Nil(t, settlement)
	require.Len(t, balance.charges, 1)
	require.Len(t, balance.refunds, 1)
	require.Len(t, creator.inputs, 1)
	require.Len(t, creator.reversals, 1)
	require.Equal(t, int64(701), creator.reversals[0].CreatorUserID)
	require.Equal(t, int64(3002), creator.reversals[0].RunID)
	require.Equal(t, 9.5, creator.reversals[0].Amount)

	require.Len(t, store.settlements, 1)
	for _, stored := range store.settlements {
		require.Equal(t, AISkillSettlementStatusFailed, stored.Status)
		require.Contains(t, stored.FailureReason, ErrAISkillCreatorEarningsMismatch.Error())
	}
}

func TestAISkillSettlementSettle_ReplaysExistingSettledRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillStoreStub()
	balance := &aiSkillBalanceChargerStub{
		result: &AISkillBalanceChargeResult{ChargedAmount: 12.5, BalanceAfter: 87.5},
	}
	creator := &aiSkillCreatorCreditorStub{}
	settlementSvc := NewAISkillSettlementService(store, balance, creator)

	first, err := settlementSvc.Settle(ctx, AISkillSettleInput{
		RunID:         3003,
		SkillID:       1003,
		VersionID:     2003,
		BuyerUserID:   902,
		CreatorUserID: 702,
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            12.5,
			PlatformCommissionRate: 0.2,
		},
		Metadata: map[string]any{"scene": "idempotent-settle"},
	})
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, AISkillSettlementStatusSettled, first.Status)
	require.Len(t, balance.charges, 1)
	require.Len(t, creator.inputs, 1)

	replayed, err := settlementSvc.Settle(ctx, AISkillSettleInput{
		RunID:         3003,
		SkillID:       9999,
		VersionID:     9999,
		BuyerUserID:   1,
		CreatorUserID: 2,
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            88,
			PlatformCommissionRate: 0.9,
		},
		Metadata: map[string]any{"scene": "replayed"},
	})
	require.NoError(t, err)
	require.NotNil(t, replayed)
	require.Equal(t, first.ID, replayed.ID)
	require.Equal(t, first.RunID, replayed.RunID)
	require.Equal(t, AISkillSettlementStatusSettled, replayed.Status)
	require.Len(t, balance.charges, 1, "replayed settle must not charge buyer twice")
	require.Len(t, creator.inputs, 1, "replayed settle must not credit creator twice")
	require.Len(t, store.settlements, 1, "replayed settle must not create duplicate settlement rows")
}

func TestAISkillSettlementSettle_RetriesExistingFailedRunWhenCompensationCompleted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillUniqueSettlementStoreStub()
	balance := &aiSkillBalanceChargerStub{
		result: &AISkillBalanceChargeResult{ChargedAmount: 12.5, BalanceAfter: 87.5},
	}
	creator := &aiSkillCreatorCreditorStub{err: errors.New("credit down")}
	settlementSvc := NewAISkillSettlementService(store, balance, creator)

	first, err := settlementSvc.Settle(ctx, AISkillSettleInput{
		RunID:         3004,
		SkillID:       1004,
		VersionID:     2004,
		BuyerUserID:   903,
		CreatorUserID: 703,
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            12.5,
			PlatformCommissionRate: 0.2,
		},
		Metadata: map[string]any{"scene": "retry-failed"},
	})
	require.Error(t, err)
	require.Nil(t, first)
	require.Len(t, store.settlements, 1)

	creator.err = nil
	replayed, err := settlementSvc.Settle(ctx, AISkillSettleInput{
		RunID:         3004,
		SkillID:       1004,
		VersionID:     2004,
		BuyerUserID:   903,
		CreatorUserID: 703,
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            12.5,
			PlatformCommissionRate: 0.2,
		},
		Metadata: map[string]any{"scene": "retry-failed"},
	})
	require.NoError(t, err)
	require.NotNil(t, replayed)
	require.Equal(t, AISkillSettlementStatusSettled, replayed.Status)
	require.Len(t, store.settlements, 1, "retry should reuse the failed settlement row instead of inserting a duplicate")
	require.Len(t, balance.charges, 2, "retry should attempt a fresh buyer charge after prior refund completed")
	require.Len(t, balance.refunds, 1, "successful retry should not emit an extra refund")
	require.Len(t, creator.inputs, 2, "retry should re-attempt creator credit")
}

func TestAISkillSettlementReplaySettlement_RetriesSafeFailedRunByRunID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillUniqueSettlementStoreStub()
	balance := &aiSkillBalanceChargerStub{
		result: &AISkillBalanceChargeResult{ChargedAmount: 12.5, BalanceAfter: 87.5},
	}
	creator := &aiSkillCreatorCreditorStub{err: errors.New("credit down")}
	settlementSvc := NewAISkillSettlementService(store, balance, creator)

	first, err := settlementSvc.Settle(ctx, AISkillSettleInput{
		RunID:         3005,
		SkillID:       1005,
		VersionID:     2005,
		BuyerUserID:   904,
		CreatorUserID: 704,
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            12.5,
			PlatformCommissionRate: 0.2,
		},
		Metadata: map[string]any{"scene": "replay-by-run-id"},
	})
	require.Error(t, err)
	require.Nil(t, first)
	require.Len(t, store.settlements, 1)

	creator.err = nil
	replayed, err := settlementSvc.ReplaySettlement(ctx, 3005)
	require.NoError(t, err)
	require.NotNil(t, replayed)
	require.Equal(t, AISkillSettlementStatusSettled, replayed.Status)
	require.Equal(t, int64(3005), replayed.RunID)
	require.Len(t, store.settlements, 1)
	require.Len(t, balance.charges, 2, "replay should issue a fresh buyer charge after prior refund completed")
	require.Len(t, balance.refunds, 1)
	require.Len(t, creator.inputs, 2)
}

func TestAISkillSettlementReplaySettlement_RejectsUnsafeFailedRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillUniqueSettlementStoreStub()
	balance := &aiSkillBalanceChargerStub{
		result:    &AISkillBalanceChargeResult{ChargedAmount: 12.5, BalanceAfter: 87.5},
		refundErr: errors.New("refund down"),
	}
	creator := &aiSkillCreatorCreditorStub{err: errors.New("credit down")}
	settlementSvc := NewAISkillSettlementService(store, balance, creator)

	first, err := settlementSvc.Settle(ctx, AISkillSettleInput{
		RunID:         3006,
		SkillID:       1006,
		VersionID:     2006,
		BuyerUserID:   905,
		CreatorUserID: 705,
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            12.5,
			PlatformCommissionRate: 0.2,
		},
		Metadata: map[string]any{"scene": "unsafe-replay"},
	})
	require.Error(t, err)
	require.Nil(t, first)

	replayed, err := settlementSvc.ReplaySettlement(ctx, 3006)
	require.ErrorIs(t, err, ErrAISkillSettlementReplayUnsafe)
	require.Nil(t, replayed)
	require.Len(t, store.settlements, 1)
	require.Len(t, balance.charges, 1, "unsafe replay must not attempt a second buyer charge")
	require.Len(t, creator.inputs, 1, "unsafe replay must not attempt a second creator credit")
}

func TestAISkillSettlementReplaySettlement_AllowsReplayAfterSuccessfulCreatorReversalAndBuyerRefund(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillUniqueSettlementStoreStub()
	balance := &aiSkillBalanceChargerStub{
		result: &AISkillBalanceChargeResult{ChargedAmount: 12.5, BalanceAfter: 87.5},
	}
	creator := &aiSkillCreatorCreditorStub{amount: 9.5}
	settlementSvc := NewAISkillSettlementService(store, balance, creator)

	first, err := settlementSvc.Settle(ctx, AISkillSettleInput{
		RunID:         3007,
		SkillID:       1007,
		VersionID:     2007,
		BuyerUserID:   906,
		CreatorUserID: 706,
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            12.5,
			PlatformCommissionRate: 0.2,
		},
		Metadata: map[string]any{"scene": "replay-after-compensation"},
	})
	require.ErrorIs(t, err, ErrAISkillCreatorEarningsMismatch)
	require.Nil(t, first)
	require.Len(t, balance.charges, 1)
	require.Len(t, balance.refunds, 1)
	require.Len(t, creator.inputs, 1)
	require.Len(t, creator.reversals, 1)

	creator.amount = 0
	replayed, err := settlementSvc.ReplaySettlement(ctx, 3007)
	require.NoError(t, err)
	require.NotNil(t, replayed)
	require.Equal(t, AISkillSettlementStatusSettled, replayed.Status)
	require.Len(t, store.settlements, 1)
	require.Len(t, balance.charges, 2, "safe replay should issue a fresh buyer charge after prior refund completed")
	require.Len(t, balance.refunds, 1)
	require.Len(t, creator.inputs, 2)
}

func TestAISkillRunServicePrepareBuildsPromptImageAndFreeSettlement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillStoreStub()
	skill := &AISkill{
		ID:            11,
		CreatorUserID: 88,
		Name:          "Image Skill",
		Type:          AISkillTypePromptImage,
	}
	version := &AISkillVersion{
		ID:            21,
		SkillID:       skill.ID,
		CreatorUserID: skill.CreatorUserID,
		Version:       1,
		Type:          skill.Type,
		Status:        AISkillVersionStatusApproved,
		ExecutionSpec: AISkillExecutionSpec{
			Type: AISkillTypePromptImage,
			PromptImage: &AISkillPromptImageSpec{
				Model:          "gpt-image-1",
				PromptTemplate: "draw {{subject}}",
				ImageCount:     2,
			},
		},
		BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
	}
	store.skills[skill.ID] = cloneAISkillEntity(skill)
	store.versions[version.ID] = cloneAISkillVersionEntity(version)

	balance := &aiSkillBalanceChargerStub{}
	settlementSvc := NewAISkillSettlementService(store, balance, &aiSkillCreatorCreditorStub{})
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, nil)

	prepared, err := runSvc.Prepare(ctx, 99, &AISkillRunInput{
		SkillID:    skill.ID,
		Mode:       AISkillRunModeTest,
		Parameters: map[string]any{"subject": "sunrise"},
		Attachments: []AISkillRunAttachment{
			{URL: "https://example.invalid/ref.png", Purpose: "reference"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, AISkillSettlementStatusSkipped, prepared.Settlement.Status)
	require.Equal(t, 0.0, prepared.Settlement.TotalAmount)
	require.Empty(t, balance.charges)
	require.Equal(t, AISkillRunStatusPrepared, prepared.Run.Status)
	require.NotNil(t, prepared.Execution.PromptImage)
	require.Equal(t, 2, prepared.Execution.PromptImage.ImageCount)
	require.Equal(t, "draw {{subject}}", prepared.Execution.PromptImage.PromptTemplate)
	require.Len(t, prepared.Execution.PromptImage.Attachments, 1)
}

func TestAISkillRunServicePrepareBuildsScriptArchiveFromSourceContent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillStoreStub()
	sourceCode := "console.log('skill archive ok')\n"
	skill := &AISkill{
		ID:            31,
		CreatorUserID: 88,
		Name:          "Node Echo",
		Type:          AISkillTypeScript,
		Metadata: map[string]any{
			"slug": "node_echo",
		},
	}
	version := &AISkillVersion{
		ID:            41,
		SkillID:       skill.ID,
		CreatorUserID: skill.CreatorUserID,
		Version:       3,
		Type:          skill.Type,
		Status:        AISkillVersionStatusApproved,
		ExecutionSpec: AISkillExecutionSpec{
			Type: AISkillTypeScript,
			Script: &AISkillScriptSpec{
				Runtime:        "nodejs18",
				ScriptName:     "node_echo",
				EntryPoint:     "main",
				TimeoutSeconds: 25,
			},
		},
		BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
		Metadata: map[string]any{
			"version_name": "v1.2.3",
			"content": map[string]any{
				"type":            "script",
				"language":        "node_echo",
				"runtime":         "nodejs18",
				"entrypoint":      "main",
				"source_code":     sourceCode,
				"timeout_seconds": 25,
			},
		},
	}
	store.skills[skill.ID] = cloneAISkillEntity(skill)
	store.versions[version.ID] = cloneAISkillVersionEntity(version)

	settlementSvc := NewAISkillSettlementService(store, &aiSkillBalanceChargerStub{}, &aiSkillCreatorCreditorStub{})
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, nil)

	prepared, err := runSvc.Prepare(ctx, 99, &AISkillRunInput{
		SkillID:   skill.ID,
		VersionID: int64Ptr(version.ID),
		Mode:      AISkillRunModeTest,
	})
	require.NoError(t, err)
	require.NotNil(t, prepared.Execution)
	require.NotNil(t, prepared.Execution.Script)
	require.Equal(t, skillrunner.RuntimeNode20, prepared.Execution.Script.Runtime)
	require.Equal(t, "main.mjs", prepared.Execution.Script.EntryPoint)
	require.Equal(t, skillrunner.ProtocolJSONFileV1, prepared.Execution.Script.Protocol)
	require.NotEmpty(t, prepared.Execution.Script.ArchiveBase64)

	archive, err := base64.StdEncoding.DecodeString(prepared.Execution.Script.ArchiveBase64)
	require.NoError(t, err)
	inspector := skillrunner.NewBundleInspector(skillrunner.ArchiveConstraints{}, nil)
	bundle, err := inspector.InspectArchive(ctx, archive)
	require.NoError(t, err)
	require.Equal(t, "node_echo", bundle.Manifest.Metadata.Name)
	require.Equal(t, "v1.2.3", bundle.Manifest.Metadata.Version)
	require.Equal(t, skillrunner.RuntimeNode20, bundle.Manifest.Spec.Runtime)
	require.Equal(t, "main.mjs", bundle.Manifest.Spec.Entrypoint)
	require.Equal(t, skillrunner.ProtocolJSONFileV1, bundle.Manifest.Spec.Protocol)
	require.Equal(t, sourceCode, readAISkillArchiveFile(t, archive, "main.mjs"))
}

func cloneAISkillEntity(skill *AISkill) *AISkill {
	if skill == nil {
		return nil
	}
	copy := *skill
	copy.Metadata = cloneAIMap(skill.Metadata)
	return &copy
}

func cloneAISkillVersionEntity(version *AISkillVersion) *AISkillVersion {
	if version == nil {
		return nil
	}
	copy := *version
	copy.ExecutionSpec = cloneAISkillExecutionSpec(version.ExecutionSpec)
	copy.BillingPolicy = version.BillingPolicy
	copy.Metadata = cloneAIMap(version.Metadata)
	if version.SubmittedAt != nil {
		ts := *version.SubmittedAt
		copy.SubmittedAt = &ts
	}
	if version.ReviewedAt != nil {
		ts := *version.ReviewedAt
		copy.ReviewedAt = &ts
	}
	if version.ApprovedAt != nil {
		ts := *version.ApprovedAt
		copy.ApprovedAt = &ts
	}
	if version.RejectedAt != nil {
		ts := *version.RejectedAt
		copy.RejectedAt = &ts
	}
	if version.DisabledAt != nil {
		ts := *version.DisabledAt
		copy.DisabledAt = &ts
	}
	if version.ReviewerUserID != nil {
		id := *version.ReviewerUserID
		copy.ReviewerUserID = &id
	}
	return &copy
}

func cloneAISkillRunEntity(run *AISkillRun) *AISkillRun {
	if run == nil {
		return nil
	}
	copy := *run
	copy.Parameters = cloneAIMap(run.Parameters)
	copy.Attachments = cloneAISkillRunAttachments(run.Attachments)
	copy.Metadata = cloneAIMap(run.Metadata)
	copy.Output = cloneAIMap(run.Output)
	if run.SettlementID != nil {
		id := *run.SettlementID
		copy.SettlementID = &id
	}
	return &copy
}

func cloneAISkillSettlementEntity(settlement *AISkillSettlement) *AISkillSettlement {
	if settlement == nil {
		return nil
	}
	copy := *settlement
	copy.Metadata = cloneAIMap(settlement.Metadata)
	if settlement.BalanceAfter != nil {
		balance := *settlement.BalanceAfter
		copy.BalanceAfter = &balance
	}
	return &copy
}

func int64Ptr(v int64) *int64 {
	return &v
}

func stringPtr(v string) *string {
	return &v
}

func readAISkillArchiveFile(t *testing.T, archive []byte, target string) string {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	for _, file := range reader.File {
		if file.Name != target {
			continue
		}
		handle, err := file.Open()
		require.NoError(t, err)
		raw, err := io.ReadAll(handle)
		_ = handle.Close()
		require.NoError(t, err)
		return string(raw)
	}
	t.Fatalf("archive entry %q not found", target)
	return ""
}
