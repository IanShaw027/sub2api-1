package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/integration/skillrunner"
	"github.com/stretchr/testify/require"
)

type aiSkillRunServiceTestStore struct {
	skills          map[int64]*AISkill
	versions        map[int64]*AISkillVersion
	runs            map[int64]*AISkillRun
	nextSkillID     int64
	nextVersionID   int64
	nextRunID       int64
	updateCalls     int
	updateErrOnCall int
	updateErr       error
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
	s.skills[skill.ID] = cloneAISkillEntityForRunRuntime(skill)
	return nil
}

func (s *aiSkillRunServiceTestStore) GetSkillByID(_ context.Context, id int64) (*AISkill, error) {
	skill, ok := s.skills[id]
	if !ok {
		return nil, ErrAISkillNotFound
	}
	return cloneAISkillEntityForRunRuntime(skill), nil
}

func (s *aiSkillRunServiceTestStore) GetSkillByCreatorAndID(_ context.Context, creatorUserID, id int64) (*AISkill, error) {
	skill, ok := s.skills[id]
	if !ok || skill.CreatorUserID != creatorUserID {
		return nil, ErrAISkillNotFound
	}
	return cloneAISkillEntityForRunRuntime(skill), nil
}

func (s *aiSkillRunServiceTestStore) UpdateSkill(_ context.Context, skill *AISkill) error {
	if _, ok := s.skills[skill.ID]; !ok {
		return ErrAISkillNotFound
	}
	s.skills[skill.ID] = cloneAISkillEntityForRunRuntime(skill)
	return nil
}

func (s *aiSkillRunServiceTestStore) CreateVersion(_ context.Context, version *AISkillVersion) error {
	version.ID = s.nextVersionID
	s.nextVersionID++
	s.versions[version.ID] = cloneAISkillVersionEntityForRunRuntime(version)
	return nil
}

func (s *aiSkillRunServiceTestStore) GetVersionByID(_ context.Context, id int64) (*AISkillVersion, error) {
	version, ok := s.versions[id]
	if !ok {
		return nil, ErrAISkillVersionNotFound
	}
	return cloneAISkillVersionEntityForRunRuntime(version), nil
}

func (s *aiSkillRunServiceTestStore) GetVersionByCreatorAndID(_ context.Context, creatorUserID, id int64) (*AISkillVersion, error) {
	version, ok := s.versions[id]
	if !ok || version.CreatorUserID != creatorUserID {
		return nil, ErrAISkillVersionNotFound
	}
	return cloneAISkillVersionEntityForRunRuntime(version), nil
}

func (s *aiSkillRunServiceTestStore) GetLatestApprovedVersionBySkillID(_ context.Context, skillID int64) (*AISkillVersion, error) {
	for _, version := range s.versions {
		if version.SkillID == skillID && normalizeAISkillVersionStatus(version.Status) == AISkillVersionStatusApproved {
			return cloneAISkillVersionEntityForRunRuntime(version), nil
		}
	}
	return nil, ErrAISkillVersionNotFound
}

func (s *aiSkillRunServiceTestStore) UpdateVersion(_ context.Context, version *AISkillVersion) error {
	if _, ok := s.versions[version.ID]; !ok {
		return ErrAISkillVersionNotFound
	}
	s.versions[version.ID] = cloneAISkillVersionEntityForRunRuntime(version)
	return nil
}

func (s *aiSkillRunServiceTestStore) CreateRun(_ context.Context, run *AISkillRun) error {
	run.ID = s.nextRunID
	s.nextRunID++
	s.runs[run.ID] = cloneAISkillRunEntityForRunRuntime(run)
	return nil
}

func (s *aiSkillRunServiceTestStore) GetRunByID(_ context.Context, id int64) (*AISkillRun, error) {
	run, ok := s.runs[id]
	if !ok {
		return nil, ErrAISkillRunNotFound
	}
	return cloneAISkillRunEntityForRunRuntime(run), nil
}

func (s *aiSkillRunServiceTestStore) UpdateRun(_ context.Context, run *AISkillRun) error {
	if _, ok := s.runs[run.ID]; !ok {
		return ErrAISkillRunNotFound
	}
	s.updateCalls++
	if s.updateErrOnCall > 0 && s.updateCalls == s.updateErrOnCall {
		return s.updateErr
	}
	s.runs[run.ID] = cloneAISkillRunEntityForRunRuntime(run)
	return nil
}

type aiSkillRunServiceTestSettlementRepo struct {
	created          []*AISkillSettlement
	nextSettlementID int64
}

func (r *aiSkillRunServiceTestSettlementRepo) CreateSettlement(_ context.Context, settlement *AISkillSettlement) error {
	if r.nextSettlementID <= 0 {
		r.nextSettlementID = 1
	}
	settlement.ID = r.nextSettlementID
	r.nextSettlementID++
	copy := *settlement
	copy.Metadata = cloneAIMap(settlement.Metadata)
	r.created = append(r.created, &copy)
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

func TestAISkillRunServiceExecuteFailedDispatchDoesNotChargeRuntimeStore(t *testing.T) {
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
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, runtime).WithAPIKeyRepository(&skillBillingAPIKeyRepoStub{ownerUserID: 901})

	result, err := runSvc.Execute(ctx, 901, &AISkillRunInput{
		SkillID:   skill.ID,
		VersionID: aiSkillRunServiceTestInt64Ptr(version.ID),
		Mode:      AISkillRunModeUse,
		Trace:     AIWriteTrace{APIKeyID: int64PtrForTest(99)},
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

func TestAISkillRunServicePrepareAppliesPromptVariableDefaults(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillRunServiceTestStore()
	skill := &AISkill{
		CreatorUserID: 701,
		Name:          "Prompt Skill",
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
				UserPromptTemplate: "Summarize {{topic}} in {{tone}} with {{format}}",
				Variables: []map[string]any{
					{"key": "topic"},
					{"key": "tone", "default_value": "formal"},
					{"key": "format", "default_value": "bullet points"},
				},
			},
		},
		BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
	}
	require.NoError(t, store.CreateVersion(ctx, version))

	settlementSvc := NewAISkillSettlementService(&aiSkillRunServiceTestSettlementRepo{}, nil, nil)
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, nil).WithAPIKeyRepository(&skillBillingAPIKeyRepoStub{ownerUserID: 702})

	prepared, err := runSvc.Prepare(ctx, 702, &AISkillRunInput{
		SkillID:   skill.ID,
		VersionID: aiSkillRunServiceTestInt64Ptr(version.ID),
		Mode:      AISkillRunModeUse,
		Trace:     AIWriteTrace{APIKeyID: int64PtrForTest(99)},
		Parameters: map[string]any{
			"topic": "golang",
			"tone":  "casual",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, prepared)
	require.NotNil(t, prepared.Execution)
	require.NotNil(t, prepared.Execution.PromptChat)
	require.Equal(t, map[string]any{
		"topic":  "golang",
		"tone":   "casual",
		"format": "bullet points",
	}, prepared.Execution.PromptChat.Parameters)
}

func TestAISkillRunServicePrepareAppliesScriptVariableDefaultsFromMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillRunServiceTestStore()
	skill := &AISkill{
		CreatorUserID: 703,
		Name:          "Script Skill",
		Type:          AISkillTypeScript,
	}
	require.NoError(t, store.CreateSkill(ctx, skill))
	version := &AISkillVersion{
		SkillID:       skill.ID,
		CreatorUserID: skill.CreatorUserID,
		Version:       1,
		Type:          skill.Type,
		Status:        AISkillVersionStatusApproved,
		ExecutionSpec: AISkillExecutionSpec{
			Type: AISkillTypeScript,
			Script: &AISkillScriptSpec{
				Runtime:    skillrunner.RuntimeNode20,
				ScriptName: "script_skill",
				EntryPoint: "main.mjs",
				Protocol:   skillrunner.ProtocolJSONFileV1,
			},
		},
		BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
		Metadata: map[string]any{
			"variable_schema": []map[string]any{
				{"key": "subject", "default_value": "sunrise"},
				{"key": "mode", "default_value": "batch"},
				{"key": "attempts", "default_value": 2},
			},
			"source_code": "console.log('hello')",
		},
	}
	require.NoError(t, store.CreateVersion(ctx, version))

	settlementSvc := NewAISkillSettlementService(&aiSkillRunServiceTestSettlementRepo{}, nil, nil)
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, nil).WithAPIKeyRepository(&skillBillingAPIKeyRepoStub{ownerUserID: 704})

	prepared, err := runSvc.Prepare(ctx, 704, &AISkillRunInput{
		SkillID:   skill.ID,
		VersionID: aiSkillRunServiceTestInt64Ptr(version.ID),
		Mode:      AISkillRunModeUse,
		Trace:     AIWriteTrace{APIKeyID: int64PtrForTest(99)},
		Parameters: map[string]any{
			"mode": "interactive",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, prepared)
	require.NotNil(t, prepared.Execution)
	require.NotNil(t, prepared.Execution.Script)
	require.Equal(t, map[string]any{
		"mode":     "interactive",
		"subject":  "sunrise",
		"attempts": 2,
	}, prepared.Execution.Script.Parameters)
}

func TestAISkillRunServiceExecuteScriptDispatchAckReturnsDispatchedRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillRunServiceTestStore()
	skill := &AISkill{
		CreatorUserID: 902,
		Name:          "Script Skill",
		Type:          AISkillTypeScript,
	}
	require.NoError(t, store.CreateSkill(ctx, skill))
	version := &AISkillVersion{
		SkillID:       skill.ID,
		CreatorUserID: skill.CreatorUserID,
		Version:       1,
		Type:          skill.Type,
		Status:        AISkillVersionStatusApproved,
		ExecutionSpec: AISkillExecutionSpec{
			Type: AISkillTypeScript,
			Script: &AISkillScriptSpec{
				Runtime:    skillrunner.RuntimeNode20,
				EntryPoint: "main.mjs",
				Protocol:   skillrunner.ProtocolJSONFileV1,
			},
		},
		BillingPolicy: AISkillBillingPolicy{
			Mode:                   AISkillBillingModePerRun,
			PricePerRun:            3.5,
			PlatformCommissionRate: 0.2,
		},
		Metadata: map[string]any{
			"source_code": "console.log('hello')",
		},
	}
	require.NoError(t, store.CreateVersion(ctx, version))

	settlementSvc := NewAISkillSettlementService(&aiSkillRunServiceTestSettlementRepo{}, nil, nil)
	runtime := &aiSkillRunServiceTestRuntime{
		result: &AISkillDispatchResult{
			Status:        AISkillRunStatusDispatched,
			Provider:      "skillrunner",
			ExternalJobID: "skillrunner:1",
			Output:        map[string]any{"plan": map[string]any{"mode": "dispatch"}},
		},
	}
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, runtime).WithAPIKeyRepository(&skillBillingAPIKeyRepoStub{ownerUserID: 903})

	result, err := runSvc.Execute(ctx, 903, &AISkillRunInput{
		SkillID:   skill.ID,
		VersionID: aiSkillRunServiceTestInt64Ptr(version.ID),
		Mode:      AISkillRunModeUse,
		Trace:     AIWriteTrace{APIKeyID: int64PtrForTest(99)},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, runtime.requests, 1)
	run := store.runs[1]
	require.NotNil(t, run)
	require.Equal(t, AISkillRunStatusDispatched, run.Status)
	require.Equal(t, "skillrunner", run.Provider)
	require.Equal(t, "skillrunner:1", run.ExternalJobID)
	require.Empty(t, run.ErrorMessage)
	require.Equal(t, map[string]any{"plan": map[string]any{"mode": "dispatch"}}, run.Output)
	settlementRepo, ok := settlementSvc.repo.(*aiSkillRunServiceTestSettlementRepo)
	require.True(t, ok)
	require.Len(t, settlementRepo.created, 0)
	require.Equal(t, 0.0, run.ChargeAmount)
	require.Empty(t, run.BillingMode)
	require.Empty(t, run.Currency)
	require.Nil(t, run.SettlementID)
}

func TestAISkillRunServiceExecuteSuccessRecoversWhenFinalUpdateFailsAfterSettlement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillRunServiceTestStore()
	store.updateErrOnCall = 2
	store.updateErr = context.Canceled
	skill := &AISkill{
		CreatorUserID: 704,
		Name:          "Settled Chat Skill",
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
			Mode:        AISkillBillingModeFree,
			PricePerRun: 0,
		},
	}
	require.NoError(t, store.CreateVersion(ctx, version))

	settlementSvc := NewAISkillSettlementService(&aiSkillRunServiceTestSettlementRepo{}, nil, nil)
	runtime := &aiSkillRunServiceTestRuntime{
		result: &AISkillDispatchResult{
			Status:        AISkillRunStatusSucceeded,
			Provider:      "openai",
			ExternalJobID: "resp_settled",
			Output:        map[string]any{"text": "summary"},
		},
	}
	runSvc := NewAISkillRunService(store, store, store, settlementSvc, runtime).WithAPIKeyRepository(&skillBillingAPIKeyRepoStub{ownerUserID: 904})

	result, err := runSvc.Execute(ctx, 904, &AISkillRunInput{
		SkillID:   skill.ID,
		VersionID: aiSkillRunServiceTestInt64Ptr(version.ID),
		Mode:      AISkillRunModeUse,
		Trace:     AIWriteTrace{APIKeyID: int64PtrForTest(99)},
		Parameters: map[string]any{
			"topic": "golang",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, runtime.requests, 1)
	run := store.runs[1]
	require.NotNil(t, run)
	require.Equal(t, AISkillRunStatusSucceeded, run.Status)
	require.Equal(t, "openai", run.Provider)
	require.Equal(t, "resp_settled", run.ExternalJobID)
	require.Empty(t, run.ErrorMessage)
	require.Equal(t, 0.0, run.ChargeAmount)
	require.Equal(t, AISkillBillingModeFree, run.BillingMode)
	require.Equal(t, "credit", run.Currency)
	require.NotNil(t, run.SettlementID)
	require.Equal(t, int64(1), *run.SettlementID)
	settlementRepo, ok := settlementSvc.repo.(*aiSkillRunServiceTestSettlementRepo)
	require.True(t, ok)
	require.Len(t, settlementRepo.created, 1)
	require.Equal(t, int64(1), settlementRepo.created[0].ID)
	require.Equal(t, 3, store.updateCalls)
}

func cloneAISkillEntityForRunRuntime(skill *AISkill) *AISkill {
	if skill == nil {
		return nil
	}
	copy := *skill
	copy.Metadata = cloneAIMap(skill.Metadata)
	return &copy
}

func cloneAISkillVersionEntityForRunRuntime(version *AISkillVersion) *AISkillVersion {
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

func cloneAISkillRunEntityForRunRuntime(run *AISkillRun) *AISkillRun {
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
