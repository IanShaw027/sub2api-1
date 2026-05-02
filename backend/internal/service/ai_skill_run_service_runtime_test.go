package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type aiSkillRunServiceTestStore struct {
	skills        map[int64]*AISkill
	versions      map[int64]*AISkillVersion
	runs          map[int64]*AISkillRun
	nextSkillID   int64
	nextVersionID int64
	nextRunID     int64
}

func newAISkillRunServiceTestStore() *aiSkillRunServiceTestStore {
	return &aiSkillRunServiceTestStore{
		skills:        map[int64]*AISkill{},
		versions:      map[int64]*AISkillVersion{},
		runs:          map[int64]*AISkillRun{},
		nextSkillID:   1,
		nextVersionID: 1,
		nextRunID:     1,
	}
}

func (s *aiSkillRunServiceTestStore) CreateSkill(_ context.Context, skill *AISkill) error {
	skill.ID = s.nextSkillID
	s.nextSkillID++
	s.skills[skill.ID] = cloneAISkillEntity(skill)
	return nil
}

func (s *aiSkillRunServiceTestStore) GetSkillByID(_ context.Context, id int64) (*AISkill, error) {
	skill, ok := s.skills[id]
	if !ok {
		return nil, ErrAISkillNotFound
	}
	return cloneAISkillEntity(skill), nil
}

func (s *aiSkillRunServiceTestStore) GetSkillByCreatorAndID(_ context.Context, creatorUserID, id int64) (*AISkill, error) {
	skill, ok := s.skills[id]
	if !ok || skill.CreatorUserID != creatorUserID {
		return nil, ErrAISkillNotFound
	}
	return cloneAISkillEntity(skill), nil
}

func (s *aiSkillRunServiceTestStore) UpdateSkill(_ context.Context, skill *AISkill) error {
	if _, ok := s.skills[skill.ID]; !ok {
		return ErrAISkillNotFound
	}
	s.skills[skill.ID] = cloneAISkillEntity(skill)
	return nil
}

func (s *aiSkillRunServiceTestStore) CreateVersion(_ context.Context, version *AISkillVersion) error {
	version.ID = s.nextVersionID
	s.nextVersionID++
	s.versions[version.ID] = cloneAISkillVersionEntity(version)
	return nil
}

func (s *aiSkillRunServiceTestStore) GetVersionByID(_ context.Context, id int64) (*AISkillVersion, error) {
	version, ok := s.versions[id]
	if !ok {
		return nil, ErrAISkillVersionNotFound
	}
	return cloneAISkillVersionEntity(version), nil
}

func (s *aiSkillRunServiceTestStore) GetVersionByCreatorAndID(_ context.Context, creatorUserID, id int64) (*AISkillVersion, error) {
	version, ok := s.versions[id]
	if !ok || version.CreatorUserID != creatorUserID {
		return nil, ErrAISkillVersionNotFound
	}
	return cloneAISkillVersionEntity(version), nil
}

func (s *aiSkillRunServiceTestStore) GetLatestApprovedVersionBySkillID(_ context.Context, skillID int64) (*AISkillVersion, error) {
	for _, version := range s.versions {
		if version.SkillID == skillID && normalizeAISkillVersionStatus(version.Status) == AISkillVersionStatusApproved {
			return cloneAISkillVersionEntity(version), nil
		}
	}
	return nil, ErrAISkillVersionNotFound
}

func (s *aiSkillRunServiceTestStore) UpdateVersion(_ context.Context, version *AISkillVersion) error {
	if _, ok := s.versions[version.ID]; !ok {
		return ErrAISkillVersionNotFound
	}
	s.versions[version.ID] = cloneAISkillVersionEntity(version)
	return nil
}

func (s *aiSkillRunServiceTestStore) CreateRun(_ context.Context, run *AISkillRun) error {
	run.ID = s.nextRunID
	s.nextRunID++
	s.runs[run.ID] = cloneAISkillRunEntity(run)
	return nil
}

func (s *aiSkillRunServiceTestStore) GetRunByID(_ context.Context, id int64) (*AISkillRun, error) {
	run, ok := s.runs[id]
	if !ok {
		return nil, ErrAISkillRunNotFound
	}
	return cloneAISkillRunEntity(run), nil
}

func (s *aiSkillRunServiceTestStore) UpdateRun(_ context.Context, run *AISkillRun) error {
	if _, ok := s.runs[run.ID]; !ok {
		return ErrAISkillRunNotFound
	}
	s.runs[run.ID] = cloneAISkillRunEntity(run)
	return nil
}

type aiSkillRunServiceTestSettlementRepo struct{}

func (r *aiSkillRunServiceTestSettlementRepo) CreateSettlement(context.Context, *AISkillSettlement) error {
	return nil
}

func (r *aiSkillRunServiceTestSettlementRepo) GetSettlementByRunID(context.Context, int64) (*AISkillSettlement, error) {
	return nil, ErrAISkillSettlementNotFound
}

func (r *aiSkillRunServiceTestSettlementRepo) UpdateSettlement(context.Context, *AISkillSettlement) error {
	return nil
}

type aiSkillRunServiceTestRuntime struct {
	requests []AISkillExecutionRequest
	result   *AISkillDispatchResult
	err      error
}

func (r *aiSkillRunServiceTestRuntime) Execute(_ context.Context, req AISkillExecutionRequest) (*AISkillDispatchResult, error) {
	r.requests = append(r.requests, req)
	if r.err != nil {
		return nil, r.err
	}
	if r.result == nil {
		return &AISkillDispatchResult{Status: AISkillRunStatusDispatched}, nil
	}
	copy := *r.result
	copy.Output = cloneAIMap(r.result.Output)
	return &copy, nil
}

func TestAISkillRunServiceExecuteFailedDispatchDoesNotCharge(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillRunServiceTestStore()
	skill := &AISkill{
		CreatorUserID: 701,
		Name:          "Failed Chat Skill",
		Type:          AISkillTypePromptChat,
	}
	require.NoError(t, store.CreateSkill(ctx, skill))
	version := &AISkillVersion{
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
	require.NoError(t, store.CreateVersion(ctx, version))

	settlementSvc := NewAISkillSettlementService(&aiSkillRunServiceTestSettlementRepo{}, nil, nil)
	runtime := &aiSkillRunServiceTestRuntime{
		result: &AISkillDispatchResult{
			Status:        AISkillRunStatusFailed,
			Provider:      "openai",
			ExternalJobID: "resp_failed",
		},
	}
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, runtime)

	result, err := runSvc.Execute(ctx, 901, &AISkillRunInput{
		SkillID:   skill.ID,
		VersionID: aiSkillRunServiceTestInt64Ptr(version.ID),
		Mode:      AISkillRunModeUse,
		Parameters: map[string]any{
			"topic": "golang",
		},
	})
	require.Error(t, err)
	require.Nil(t, result)
	require.Len(t, runtime.requests, 1)
	run := store.runs[1]
	require.NotNil(t, run)
	require.Equal(t, AISkillRunStatusFailed, run.Status)
	require.Nil(t, run.SettlementID)
	require.Zero(t, run.ChargeAmount)
	require.Empty(t, run.BillingMode)
	require.Empty(t, run.Currency)
	require.Empty(t, run.Output)
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

func aiSkillRunServiceTestInt64Ptr(v int64) *int64 {
	return &v
}
