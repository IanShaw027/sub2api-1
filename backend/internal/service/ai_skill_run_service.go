package service

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/integration/skillrunner"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// AISkillBillingAPIKeyLookup loads a buyer's API key for token billing attribution.
// Narrower than APIKeyRepository so tests can stub GetByID alone.
type AISkillBillingAPIKeyLookup interface {
	GetByID(ctx context.Context, id int64) (*APIKey, error)
}

type AISkillRunService struct {
	skillRepo         AISkillRepository
	versionRepo       AISkillVersionRepository
	runRepo           AISkillRunRepository
	settlementService *AISkillSettlementService
	runtimeGateway    AISkillRuntimeGateway
	apiKeyRepo        AISkillBillingAPIKeyLookup
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

// WithAPIKeyRepository enables server-side resolution of Trace.APIKeyID for
// gateway token billing attribution. Optional — without it skill runs still
// execute and settle marketplace fees.
func (s *AISkillRunService) WithAPIKeyRepository(repo AISkillBillingAPIKeyLookup) *AISkillRunService {
	if s != nil {
		s.apiKeyRepo = repo
	}
	return s
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
	// Upstream-consuming skill types must always bind a buyer API key so tokens
	// are attributed. Test mode used to skip this and could free-ride platform
	// accounts; both test and use now require billing attribution for prompt_* .
	if aiSkillModeRequiresBillingAPIKey(mode, version.Type) {
		if input.Trace.APIKeyID == nil || *input.Trace.APIKeyID <= 0 {
			return nil, infraerrors.BadRequest("AI_SKILL_API_KEY_REQUIRED", "skill runs that call upstream models require trace.api_key_id for token billing")
		}
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
	execution, err := s.buildExecutionRequest(ctx, run, skill, version, settlement)
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
	settleInput := AISkillSettleInput{
		RunID:         prepared.Run.ID,
		SkillID:       prepared.Skill.ID,
		VersionID:     prepared.Version.ID,
		BuyerUserID:   prepared.Run.UserID,
		CreatorUserID: prepared.Skill.CreatorUserID,
		BillingPolicy: prepared.Version.BillingPolicy,
		Metadata:      cloneAIMap(prepared.Run.Metadata),
		Trace:         prepared.Run.Trace,
	}
	intent, err := s.settlementService.CreateIntent(ctx, settleInput)
	if err != nil {
		prepared.Run.Status = AISkillRunStatusFailed
		prepared.Run.ErrorMessage = err.Error()
		prepared.Run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, prepared.Run)
		return nil, err
	}
	prepared.Settlement = intent
	prepared.Execution.Settlement = intent
	prepared.Run.SettlementID = &intent.ID
	dispatch, err := s.runtimeGateway.Execute(ctx, *prepared.Execution)
	if err != nil {
		s.failSettlementIntentAfterDispatch(ctx, prepared, err)
		prepared.Run.Status = AISkillRunStatusFailed
		prepared.Run.ErrorMessage = err.Error()
		prepared.Run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, prepared.Run)
		return nil, err
	}
	if dispatch == nil {
		dispatchErr := skillExecutionDispatchFailedError()
		s.failSettlementIntentAfterDispatch(ctx, prepared, dispatchErr)
		prepared.Run.Status = AISkillRunStatusFailed
		prepared.Run.ErrorMessage = dispatchErr.Error()
		prepared.Run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, prepared.Run)
		return nil, dispatchErr
	}
	if !isAISkillDispatchSuccess(dispatch.Status) {
		dispatchErr := skillExecutionDispatchFailedError()
		prepared.Run.Status = AISkillRunStatusFailed
		prepared.Run.Provider = strings.TrimSpace(dispatch.Provider)
		prepared.Run.ExternalJobID = strings.TrimSpace(dispatch.ExternalJobID)
		prepared.Run.Output = cloneAIMap(dispatch.Output)
		prepared.Run.ErrorMessage = dispatchErr.Error()
		s.failSettlementIntentAfterDispatch(ctx, prepared, dispatchErr)
		prepared.Run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, prepared.Run)
		return nil, dispatchErr
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
	confirmed, confirmErr := s.confirmSettlementIntentAfterDispatch(ctx, prepared.Run.ID, intent.ID)
	if confirmErr != nil {
		prepared.Run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, prepared.Run)
		slog.Error("ai_skill.settlement_intent_confirmation_deferred", "run_id", prepared.Run.ID, "settlement_id", intent.ID, "error", confirmErr)
		return &AISkillRunResult{
			Prepared:           prepared,
			Dispatch:           dispatch,
			SettlementDeferred: true,
			SettlementStatus:   intent.Status,
		}, nil
	}
	prepared.Settlement = confirmed
	prepared.Execution.Settlement = confirmed
	if err := s.runRepo.UpdateRun(ctx, prepared.Run); err != nil {
		return nil, err
	}
	if dispatchStatus != AISkillRunStatusSucceeded {
		return &AISkillRunResult{
			Prepared:         prepared,
			Dispatch:         dispatch,
			SettlementStatus: confirmed.Status,
		}, nil
	}
	settlement, err := s.settlementService.SettleIntent(ctx, settleInput, intent.ID)
	if err != nil {
		// Upstream execution has already succeeded and its actual token usage has
		// been recorded. Marketplace settlement is an independent, durable state
		// machine: keep the run deliverable and let the persisted settlement be
		// replayed instead of turning an internal accounting outage into a failed
		// execution or refunding real provider consumption.
		deferredStatus := AISkillSettlementStatusPending
		if persisted, getErr := s.settlementService.repo.GetSettlementByRunID(ctx, prepared.Run.ID); getErr == nil && persisted != nil {
			prepared.Settlement = persisted
			deferredStatus = strings.TrimSpace(persisted.Status)
			if prepared.Execution != nil {
				prepared.Execution.Settlement = persisted
			}
			if persisted.ID > 0 {
				prepared.Run.SettlementID = &persisted.ID
			}
		}
		prepared.Run.Status = AISkillRunStatusDispatched
		prepared.Run.ErrorMessage = ""
		prepared.Run.UpdatedAt = s.nowOrDefault()
		_ = s.runRepo.UpdateRun(ctx, prepared.Run)
		slog.Error("ai_skill.marketplace_settlement_deferred",
			"run_id", prepared.Run.ID,
			"settlement_status", deferredStatus,
			"error", err)
		return &AISkillRunResult{
			Prepared:           prepared,
			Dispatch:           dispatch,
			SettlementDeferred: true,
			SettlementStatus:   deferredStatus,
		}, nil
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
	s.persistSettledRunBestEffort(ctx, prepared.Run, settlement)
	return &AISkillRunResult{
		Prepared:         prepared,
		Dispatch:         dispatch,
		SettlementStatus: settlement.Status,
	}, nil
}

func (s *AISkillRunService) failSettlementIntentAfterDispatch(ctx context.Context, prepared *AISkillPreparedRun, cause error) {
	if s == nil || s.settlementService == nil || prepared == nil || prepared.Run == nil || prepared.Settlement == nil {
		return
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var settlement *AISkillSettlement
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		settlement, err = s.settlementService.FailIntentForDispatch(persistCtx, prepared.Run.ID, prepared.Settlement.ID, cause)
		if err == nil {
			break
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
		}
	}
	if err != nil {
		slog.Error("ai_skill.settlement_intent_terminalize_failed", "run_id", prepared.Run.ID, "settlement_id", prepared.Settlement.ID, "error", err)
		return
	}
	prepared.Settlement = settlement
	if prepared.Execution != nil {
		prepared.Execution.Settlement = settlement
	}
}

func (s *AISkillRunService) confirmSettlementIntentAfterDispatch(ctx context.Context, runID, intentID int64) (*AISkillSettlement, error) {
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var settlement *AISkillSettlement
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		settlement, err = s.settlementService.ConfirmIntent(persistCtx, runID, intentID)
		if err == nil {
			return settlement, nil
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
		}
	}
	return nil, err
}

func (s *AISkillRunService) persistSettledRunBestEffort(ctx context.Context, run *AISkillRun, settlement *AISkillSettlement) {
	if s == nil || s.runRepo == nil || run == nil || settlement == nil {
		return
	}
	if err := s.runRepo.UpdateRun(ctx, run); err == nil {
		return
	}
	current, err := s.runRepo.GetRunByID(ctx, run.ID)
	if err != nil || current == nil {
		return
	}
	current.Status = AISkillRunStatusSucceeded
	current.Provider = strings.TrimSpace(run.Provider)
	current.ExternalJobID = strings.TrimSpace(run.ExternalJobID)
	current.Output = cloneAIMap(run.Output)
	current.ErrorMessage = ""
	current.BillingMode = settlement.BillingMode
	current.ChargeAmount = settlement.TotalAmount
	current.Currency = settlement.Currency
	current.SettlementID = &settlement.ID
	current.UpdatedAt = s.nowOrDefault()
	if err := s.runRepo.UpdateRun(ctx, current); err != nil {
		return
	}
	*run = *current
}

func (s *AISkillRunService) resolveVersion(ctx context.Context, skillID int64, versionID *int64) (*AISkillVersion, error) {
	if versionID != nil && *versionID > 0 {
		return s.versionRepo.GetVersionByID(ctx, *versionID)
	}
	return s.versionRepo.GetLatestApprovedVersionBySkillID(ctx, skillID)
}

// resolveBillingAPIKey loads the buyer's API key referenced by Trace.APIKeyID.
// Ownership is enforced: key.UserID must match the run user.
//
// Returns (nil, nil) when no API key was requested, or when lookup is not wired
// (caller decides fail-closed for use mode). Explicit ids with a wired repo that
// fail ownership / missing checks always return an error.
func (s *AISkillRunService) resolveBillingAPIKey(ctx context.Context, userID int64, trace AIWriteTrace) (*APIKey, error) {
	if userID <= 0 {
		return nil, nil
	}
	if trace.APIKeyID == nil || *trace.APIKeyID <= 0 {
		return nil, nil
	}
	if s == nil || s.apiKeyRepo == nil {
		return nil, nil
	}
	key, err := s.apiKeyRepo.GetByID(ctx, *trace.APIKeyID)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, infraerrors.NotFound("AI_SKILL_API_KEY_NOT_FOUND", "api key not found for skill billing")
	}
	if key.UserID != userID {
		return nil, infraerrors.Forbidden("AI_SKILL_API_KEY_FORBIDDEN", "api key does not belong to the skill runner")
	}
	if err := validateAISkillBillingAPIKey(key); err != nil {
		return nil, err
	}
	if key.User == nil {
		// Hydrate minimal user for RecordUsage; full balance checks happen inside billing.
		key.User = &User{ID: userID}
	}
	return key, nil
}

func validateAISkillBillingAPIKey(key *APIKey) error {
	if key == nil {
		return infraerrors.NotFound("AI_SKILL_API_KEY_NOT_FOUND", "api key not found for skill billing")
	}
	switch key.Status {
	case "", StatusActive:
	case StatusAPIKeyExpired:
		return ErrAPIKeyExpired
	case StatusAPIKeyQuotaExhausted:
		return ErrAPIKeyQuotaExhausted
	default:
		return infraerrors.Unauthorized("API_KEY_INACTIVE", "api key is not active")
	}
	if key.IsExpired() {
		return ErrAPIKeyExpired
	}
	if key.IsQuotaExhausted() {
		return ErrAPIKeyQuotaExhausted
	}
	if key.RateLimit5h > 0 && key.EffectiveUsage5h() >= key.RateLimit5h {
		return ErrAPIKeyRateLimit5hExceeded
	}
	if key.RateLimit1d > 0 && key.EffectiveUsage1d() >= key.RateLimit1d {
		return ErrAPIKeyRateLimit1dExceeded
	}
	if key.RateLimit7d > 0 && key.EffectiveUsage7d() >= key.RateLimit7d {
		return ErrAPIKeyRateLimit7dExceeded
	}
	if key.User != nil && strings.TrimSpace(key.User.Status) != "" && !key.User.IsActive() {
		return ErrUserNotActive
	}
	return nil
}

func (s *AISkillRunService) buildExecutionRequest(ctx context.Context, run *AISkillRun, skill *AISkill, version *AISkillVersion, settlement *AISkillSettlement) (*AISkillExecutionRequest, error) {
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
	key, err := s.resolveBillingAPIKey(ctx, run.UserID, run.Trace)
	if err != nil {
		return nil, err
	}
	req.BillingAPIKey = key
	// Fail closed: any mode that hits upstream must have a resolved owned key.
	if aiSkillModeRequiresBillingAPIKey(run.Mode, version.Type) {
		if run.Trace.APIKeyID == nil || *run.Trace.APIKeyID <= 0 {
			return nil, infraerrors.BadRequest("AI_SKILL_API_KEY_REQUIRED", "skill runs that call upstream models require trace.api_key_id for token billing")
		}
		if req.BillingAPIKey == nil {
			if s.apiKeyRepo == nil {
				return nil, infraerrors.InternalServer("AI_SKILL_API_KEY_LOOKUP_UNAVAILABLE", "api key lookup is not configured for skill billing")
			}
			return nil, infraerrors.BadRequest("AI_SKILL_API_KEY_REQUIRED", "skill runs that call upstream models require a valid owned api key for token billing")
		}
	}
	switch version.Type {
	case AISkillTypePromptChat:
		parameters := mergeAISkillExecutionParameters(run.Parameters, spec.PromptChat.Variables)
		req.PromptChat = &AISkillPromptChatExecution{
			Model:              spec.PromptChat.Model,
			SystemPrompt:       spec.PromptChat.SystemPrompt,
			UserPromptTemplate: spec.PromptChat.UserPromptTemplate,
			Variables:          cloneAIMapSlice(spec.PromptChat.Variables),
			ResponseFormat:     cloneAIMap(spec.PromptChat.ResponseFormat),
			Parameters:         parameters,
			Attachments:        cloneAISkillRunAttachments(run.Attachments),
		}
	case AISkillTypePromptImage:
		parameters := mergeAISkillExecutionParameters(run.Parameters, spec.PromptImage.Variables)
		req.PromptImage = &AISkillPromptImageExecution{
			Model:                  spec.PromptImage.Model,
			PromptTemplate:         spec.PromptImage.PromptTemplate,
			NegativePromptTemplate: spec.PromptImage.NegativePromptTemplate,
			Size:                   spec.PromptImage.Size,
			ImageCount:             spec.PromptImage.ImageCount,
			Variables:              cloneAIMapSlice(spec.PromptImage.Variables),
			Parameters:             parameters,
			Attachments:            cloneAISkillRunAttachments(run.Attachments),
		}
	case AISkillTypeScript:
		parameters := mergeAISkillExecutionParameters(run.Parameters, extractAISkillVariableSchema(version.Metadata))
		resolved, err := resolveAISkillScriptArtifact(skill, version, spec.Script)
		if err != nil {
			return nil, err
		}
		req.Script = &AISkillScriptExecution{
			Runtime:                resolved.Runtime,
			ScriptName:             resolved.ScriptName,
			EntryPoint:             resolved.EntryPoint,
			Protocol:               resolved.Protocol,
			ArchivePath:            resolved.ArchivePath,
			ArchiveBase64:          resolved.ArchiveBase64,
			ApprovedArtifactDigest: strings.TrimSpace(version.ApprovedArtifactDigest),
			VersionStatus:          version.Status,
			ReviewerUserID:         version.ReviewerUserID,
			TimeoutSeconds:         spec.Script.TimeoutSeconds,
			Environment:            cloneAISkillStringMap(spec.Script.Environment),
			Arguments:              cloneAIMapSlice(spec.Script.Arguments),
			Parameters:             parameters,
		}
	default:
		return nil, ErrAISkillExecutionSpecInvalid
	}
	return req, nil
}

// aiSkillResolvedScriptArtifact captures the normalized script fields and the
// resolved archive source. It is produced from a (skill, version, spec) tuple
// using a single deterministic code path so that the archive bytes computed at
// approval time (for digest capture) and at run time are byte-identical.
type aiSkillResolvedScriptArtifact struct {
	Runtime       string
	ScriptName    string
	EntryPoint    string
	Protocol      string
	ArchivePath   string
	ArchiveBase64 string
}

// resolveAISkillScriptArtifact normalizes the script spec for a version and, when
// no explicit archive is supplied, regenerates the bundle from source_content.
// This is the single source of truth for the executable artifact; both run
// dispatch and approval-time digest capture call it to guarantee the same bytes.
func resolveAISkillScriptArtifact(skill *AISkill, version *AISkillVersion, scriptSpec *AISkillScriptSpec) (*aiSkillResolvedScriptArtifact, error) {
	if version == nil || scriptSpec == nil {
		return nil, ErrAISkillExecutionSpecInvalid
	}
	scriptRuntime := normalizeAISkillScriptRuntime(scriptSpec.Runtime)
	scriptName := normalizeAISkillScriptBundleName(scriptSpec.ScriptName, skill, version)
	scriptEntryPoint := normalizeAISkillScriptEntryPoint(scriptSpec.EntryPoint, scriptRuntime)
	scriptProtocol := firstNonEmptyString(strings.TrimSpace(scriptSpec.Protocol), skillrunner.ProtocolJSONFileV1)
	scriptArchivePath := strings.TrimSpace(scriptSpec.ArchivePath)
	scriptArchiveBase64 := strings.TrimSpace(scriptSpec.ArchiveBase64)
	if scriptArchivePath == "" && scriptArchiveBase64 == "" {
		scriptSource := extractAISkillScriptSourceCode(version.Metadata)
		if strings.TrimSpace(scriptSource) == "" {
			return nil, ErrAISkillExecutionSpecInvalid
		}
		skillName := scriptName
		skillDescription := ""
		if skill != nil {
			skillName = firstNonEmptyString(strings.TrimSpace(skill.Name), scriptName)
			skillDescription = strings.TrimSpace(skill.Description)
		}
		generatedArchive, err := buildAISkillScriptArchiveBase64(
			scriptName,
			normalizeAISkillScriptVersionName(version),
			skillName,
			skillDescription,
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
	return &aiSkillResolvedScriptArtifact{
		Runtime:       scriptRuntime,
		ScriptName:    scriptName,
		EntryPoint:    scriptEntryPoint,
		Protocol:      scriptProtocol,
		ArchivePath:   scriptArchivePath,
		ArchiveBase64: scriptArchiveBase64,
	}, nil
}

// computeAISkillVersionApprovedDigest derives the digest of the executable
// artifact for a script version, using the exact same normalization and
// archive-generation path as run-time dispatch. This is what review approval
// persists so that run-time validation can bind execution to the reviewed
// artifact. It returns an empty digest (no error) for non-script versions or
// when the artifact is supplied as an on-disk path rather than inline source.
func computeAISkillVersionApprovedDigest(skill *AISkill, version *AISkillVersion) (string, error) {
	if version == nil {
		return "", ErrAISkillExecutionSpecInvalid
	}
	if normalizeAISkillType(version.Type) != AISkillTypeScript {
		return "", nil
	}
	spec, err := normalizeAISkillExecutionSpec(version.Type, version.ExecutionSpec)
	if err != nil {
		return "", err
	}
	resolved, err := resolveAISkillScriptArtifact(skill, version, spec.Script)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resolved.ArchiveBase64) == "" {
		// On-disk archive paths cannot be digested deterministically here; leave
		// the approved digest empty so run-time fails closed and requires the
		// supported inline-source flow.
		return "", nil
	}
	return computeAISkillScriptArtifactDigest(resolved.ArchiveBase64)
}

// aiSkillModeRequiresBillingAPIKey reports whether this run will invoke an
// upstream model gateway and therefore must attribute usage to a buyer key.
// Script skills bill via settlement only (no OpenAI gateway path here).
func aiSkillModeRequiresBillingAPIKey(mode, skillType string) bool {
	switch normalizeAISkillType(skillType) {
	case AISkillTypePromptChat, AISkillTypePromptImage:
		switch normalizeAISkillRunMode(mode) {
		case AISkillRunModeTest, AISkillRunModeUse:
			return true
		}
	}
	return false
}

func mergeAISkillExecutionParameters(parameters map[string]any, schemas ...[]map[string]any) map[string]any {
	merged := cloneAIMap(parameters)
	for _, schema := range schemas {
		for _, field := range schema {
			key := strings.TrimSpace(extractAISkillVariableKey(field))
			if key == "" {
				continue
			}
			if _, exists := merged[key]; exists {
				continue
			}
			if defaultValue, ok := extractAISkillVariableDefaultValue(field); ok {
				merged[key] = defaultValue
			}
		}
	}
	return merged
}

func extractAISkillVariableSchema(meta map[string]any) []map[string]any {
	if len(meta) == 0 {
		return nil
	}
	raw, ok := meta["variable_schema"]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []map[string]any:
		return cloneAIMapSlice(typed)
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			record, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, cloneAIMap(record))
		}
		return out
	default:
		return nil
	}
}

func extractAISkillVariableKey(field map[string]any) string {
	if len(field) == 0 {
		return ""
	}
	for _, key := range []string{"key", "name", "id"} {
		if value, ok := field[key]; ok {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				return text
			}
		}
	}
	return ""
}

func extractAISkillVariableDefaultValue(field map[string]any) (any, bool) {
	if len(field) == 0 {
		return nil, false
	}
	for _, key := range []string{"default_value", "default"} {
		if value, ok := field[key]; ok && value != nil {
			return value, true
		}
	}
	return nil, false
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
