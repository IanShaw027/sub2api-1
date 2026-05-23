package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/integration/skillrunner"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AISkillRunService struct {
	skillRepo         AISkillRepository
	versionRepo       AISkillVersionRepository
	runRepo           AISkillRunRepository
	settlementService *AISkillSettlementService
	runtimeGateway    AISkillRuntimeGateway
	now               func() time.Time
}

func NewAISkillRunService(
	skillRepo AISkillRepository,
	versionRepo AISkillVersionRepository,
	runRepo AISkillRunRepository,
	settlementService *AISkillSettlementService,
	runtimeGateway AISkillRuntimeGateway,
) *AISkillRunService {
	return &AISkillRunService{
		skillRepo:         skillRepo,
		versionRepo:       versionRepo,
		runRepo:           runRepo,
		settlementService: settlementService,
		runtimeGateway:    runtimeGateway,
		now:               time.Now,
	}
}

func (s *AISkillRunService) Prepare(ctx context.Context, userID int64, input *AISkillRunInput) (*AISkillPreparedRun, error) {
	if s == nil || s.skillRepo == nil || s.versionRepo == nil || s.runRepo == nil {
		return nil, ErrAISkillServiceUnavailable
	}
	if s.settlementService == nil {
		return nil, ErrAISkillSettlementUnavailable
	}
	if userID <= 0 || input == nil || input.SkillID <= 0 {
		return nil, infraerrors.BadRequest("AI_SKILL_RUN_INPUT_REQUIRED", "ai skill run input is required")
	}
	mode := normalizeAISkillRunMode(input.Mode)
	if mode == "" {
		return nil, ErrAISkillRunModeInvalid
	}
	skill, err := s.skillRepo.GetSkillByID(ctx, input.SkillID)
	if err != nil {
		return nil, err
	}
	version, err := s.resolveVersion(ctx, skill.ID, input.VersionID)
	if err != nil {
		return nil, err
	}
	if version.SkillID != skill.ID {
		return nil, ErrAISkillVersionNotFound
	}
	if !canAISkillVersionTestUseOrPublish(version.Status) {
		return nil, ErrAISkillVersionNotApproved
	}
	now := s.nowOrDefault()
	runMetadata := cloneAIMap(input.Metadata)
	if key := strings.TrimSpace(input.IdempotencyKey); key != "" {
		runMetadata["idempotency_key"] = key
	}
	run := &AISkillRun{
		SkillID:     skill.ID,
		VersionID:   version.ID,
		UserID:      userID,
		Mode:        mode,
		Type:        version.Type,
		Status:      AISkillRunStatusPrepared,
		Parameters:  cloneAIMap(input.Parameters),
		Attachments: cloneAISkillRunAttachments(input.Attachments),
		Metadata:    runMetadata,
		Trace:       normalizeAIWriteTrace(ctx, input.Trace),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.runRepo.CreateRun(ctx, run); err != nil {
		return nil, err
	}

	quote, err := s.settlementService.Quote(version.BillingPolicy)
	if err != nil {
		run.Status = AISkillRunStatusFailed
		run.ErrorMessage = err.Error()
		run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, run)
		return nil, err
	}

	settlement := buildAISkillSettlementPreview(run, skill, version, quote, runMetadata, run.Trace, s.nowOrDefault())
	execution, err := s.buildExecutionRequest(run, skill, version, settlement)
	if err != nil {
		run.Status = AISkillRunStatusFailed
		run.ErrorMessage = err.Error()
		run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, run)
		return nil, err
	}
	return &AISkillPreparedRun{
		Skill:      skill,
		Version:    version,
		Run:        run,
		Settlement: settlement,
		Execution:  execution,
	}, nil
}

func (s *AISkillRunService) Execute(ctx context.Context, userID int64, input *AISkillRunInput) (*AISkillRunResult, error) {
	prepared, err := s.Prepare(ctx, userID, input)
	if err != nil {
		return nil, err
	}
	if s.runtimeGateway == nil {
		return &AISkillRunResult{Prepared: prepared}, nil
	}
	dispatch, err := s.runtimeGateway.Execute(ctx, *prepared.Execution)
	if err != nil {
		prepared.Run.Status = AISkillRunStatusFailed
		prepared.Run.ErrorMessage = err.Error()
		prepared.Run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, prepared.Run)
		return nil, err
	}
	if dispatch == nil {
		prepared.Run.Status = AISkillRunStatusFailed
		prepared.Run.ErrorMessage = skillExecutionDispatchFailedError().Error()
		prepared.Run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, prepared.Run)
		return nil, skillExecutionDispatchFailedError()
	}
	if !isAISkillDispatchSuccess(dispatch.Status) {
		prepared.Run.Status = AISkillRunStatusFailed
		prepared.Run.Provider = strings.TrimSpace(dispatch.Provider)
		prepared.Run.ExternalJobID = strings.TrimSpace(dispatch.ExternalJobID)
		prepared.Run.Output = cloneAIMap(dispatch.Output)
		prepared.Run.ErrorMessage = skillExecutionDispatchFailedError().Error()
		prepared.Run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, prepared.Run)
		return nil, skillExecutionDispatchFailedError()
	}
	dispatchStatus := normalizeAISkillDispatchStatus(dispatch.Status)
	prepared.Run.Status = dispatchStatus
	prepared.Run.Provider = strings.TrimSpace(dispatch.Provider)
	prepared.Run.ExternalJobID = strings.TrimSpace(dispatch.ExternalJobID)
	prepared.Run.Output = cloneAIMap(dispatch.Output)
	prepared.Run.UpdatedAt = s.nowOrDefault()
	if dispatchStatus == AISkillRunStatusSucceeded {
		prepared.Run.Status = AISkillRunStatusDispatched
	}
	if err := s.runRepo.UpdateRun(ctx, prepared.Run); err != nil {
		return nil, err
	}
	if dispatchStatus != AISkillRunStatusSucceeded {
		return &AISkillRunResult{
			Prepared: prepared,
			Dispatch: dispatch,
		}, nil
	}
	settlement, err := s.settlementService.Settle(ctx, AISkillSettleInput{
		RunID:         prepared.Run.ID,
		SkillID:       prepared.Skill.ID,
		VersionID:     prepared.Version.ID,
		BuyerUserID:   prepared.Run.UserID,
		CreatorUserID: prepared.Skill.CreatorUserID,
		BillingPolicy: prepared.Version.BillingPolicy,
		Metadata:      cloneAIMap(prepared.Run.Metadata),
		Trace:         prepared.Run.Trace,
	})
	if err != nil {
		prepared.Run.Status = AISkillRunStatusFailed
		prepared.Run.ErrorMessage = err.Error()
		prepared.Run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, prepared.Run)
		return nil, err
	}
	prepared.Settlement = settlement
	if prepared.Execution != nil {
		prepared.Execution.Settlement = settlement
	}
	prepared.Run.Status = AISkillRunStatusSucceeded
	prepared.Run.BillingMode = settlement.BillingMode
	prepared.Run.ChargeAmount = settlement.TotalAmount
	prepared.Run.Currency = settlement.Currency
	prepared.Run.SettlementID = &settlement.ID
	prepared.Run.UpdatedAt = s.nowOrDefault()
	if err := s.runRepo.UpdateRun(ctx, prepared.Run); err != nil {
		return nil, err
	}
	return &AISkillRunResult{
		Prepared: prepared,
		Dispatch: dispatch,
	}, nil
}

func (s *AISkillRunService) resolveVersion(ctx context.Context, skillID int64, versionID *int64) (*AISkillVersion, error) {
	if versionID != nil && *versionID > 0 {
		return s.versionRepo.GetVersionByID(ctx, *versionID)
	}
	return s.versionRepo.GetLatestApprovedVersionBySkillID(ctx, skillID)
}

func (s *AISkillRunService) buildExecutionRequest(run *AISkillRun, skill *AISkill, version *AISkillVersion, settlement *AISkillSettlement) (*AISkillExecutionRequest, error) {
	if run == nil || skill == nil || version == nil {
		return nil, ErrAISkillExecutionSpecInvalid
	}
	spec, err := normalizeAISkillExecutionSpec(version.Type, version.ExecutionSpec)
	if err != nil {
		return nil, err
	}
	req := &AISkillExecutionRequest{
		RunID:      run.ID,
		SkillID:    skill.ID,
		VersionID:  version.ID,
		UserID:     run.UserID,
		Mode:       run.Mode,
		Type:       version.Type,
		Skill:      skill,
		Version:    version,
		Settlement: settlement,
		Trace:      run.Trace,
	}
	switch version.Type {
	case AISkillTypePromptChat:
		req.PromptChat = &AISkillPromptChatExecution{
			Model:              spec.PromptChat.Model,
			SystemPrompt:       spec.PromptChat.SystemPrompt,
			UserPromptTemplate: spec.PromptChat.UserPromptTemplate,
			Variables:          cloneAIMapSlice(spec.PromptChat.Variables),
			ResponseFormat:     cloneAIMap(spec.PromptChat.ResponseFormat),
			Parameters:         cloneAIMap(run.Parameters),
			Attachments:        cloneAISkillRunAttachments(run.Attachments),
		}
	case AISkillTypePromptImage:
		req.PromptImage = &AISkillPromptImageExecution{
			Model:                  spec.PromptImage.Model,
			PromptTemplate:         spec.PromptImage.PromptTemplate,
			NegativePromptTemplate: spec.PromptImage.NegativePromptTemplate,
			Size:                   spec.PromptImage.Size,
			ImageCount:             spec.PromptImage.ImageCount,
			Variables:              cloneAIMapSlice(spec.PromptImage.Variables),
			Parameters:             cloneAIMap(run.Parameters),
			Attachments:            cloneAISkillRunAttachments(run.Attachments),
		}
	case AISkillTypeScript:
		scriptRuntime := normalizeAISkillScriptRuntime(spec.Script.Runtime)
		scriptName := normalizeAISkillScriptBundleName(spec.Script.ScriptName, skill, version)
		scriptEntryPoint := normalizeAISkillScriptEntryPoint(spec.Script.EntryPoint, scriptRuntime)
		scriptProtocol := firstNonEmptyString(strings.TrimSpace(spec.Script.Protocol), skillrunner.ProtocolJSONFileV1)
		scriptArchivePath := strings.TrimSpace(spec.Script.ArchivePath)
		scriptArchiveBase64 := strings.TrimSpace(spec.Script.ArchiveBase64)
		if scriptArchivePath == "" && scriptArchiveBase64 == "" {
			scriptSource := extractAISkillScriptSourceCode(version.Metadata)
			if strings.TrimSpace(scriptSource) == "" {
				return nil, ErrAISkillExecutionSpecInvalid
			}
			generatedArchive, err := buildAISkillScriptArchiveBase64(
				scriptName,
				normalizeAISkillScriptVersionName(version),
				firstNonEmptyString(strings.TrimSpace(skill.Name), scriptName),
				strings.TrimSpace(skill.Description),
				scriptRuntime,
				scriptEntryPoint,
				scriptProtocol,
				scriptSource,
			)
			if err != nil {
				return nil, err
			}
			scriptArchiveBase64 = generatedArchive
		}
		req.Script = &AISkillScriptExecution{
			Runtime:        scriptRuntime,
			ScriptName:     scriptName,
			EntryPoint:     scriptEntryPoint,
			Protocol:       scriptProtocol,
			ArchivePath:    scriptArchivePath,
			ArchiveBase64:  scriptArchiveBase64,
			TimeoutSeconds: spec.Script.TimeoutSeconds,
			Environment:    cloneAISkillStringMap(spec.Script.Environment),
			Arguments:      cloneAIMapSlice(spec.Script.Arguments),
			Parameters:     cloneAIMap(run.Parameters),
		}
	default:
		return nil, ErrAISkillExecutionSpecInvalid
	}
	return req, nil
}

func (s *AISkillRunService) nowOrDefault() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

func buildAISkillSettlementPreview(run *AISkillRun, skill *AISkill, version *AISkillVersion, quote *AISkillSettlementQuote, metadata map[string]any, trace AIWriteTrace, now time.Time) *AISkillSettlement {
	if run == nil || skill == nil || version == nil || quote == nil {
		return nil
	}
	settlement := &AISkillSettlement{
		RunID:                  run.ID,
		SkillID:                skill.ID,
		VersionID:              version.ID,
		BuyerUserID:            run.UserID,
		CreatorUserID:          skill.CreatorUserID,
		BillingMode:            quote.BillingMode,
		Currency:               quote.Currency,
		TotalAmount:            quote.TotalAmount,
		PlatformAmount:         quote.PlatformAmount,
		CreatorAmount:          quote.CreatorAmount,
		PlatformCommissionRate: quote.PlatformCommissionRate,
		Status:                 AISkillSettlementStatusPending,
		Metadata:               cloneAIMap(metadata),
		Trace:                  trace,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if quote.TotalAmount == 0 {
		settlement.Status = AISkillSettlementStatusSkipped
	}
	return settlement
}

func skillExecutionDispatchFailedError() error {
	return infraerrors.InternalServer("AI_SKILL_EXECUTION_FAILED", "ai skill execution failed")
}

func isAISkillDispatchSuccess(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case AISkillRunStatusSucceeded, AISkillRunStatusDispatched:
		return true
	default:
		return false
	}
}
