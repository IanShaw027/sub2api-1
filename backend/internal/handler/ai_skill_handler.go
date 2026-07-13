package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/handler/skillkit"
	"github.com/Wei-Shaw/sub2api/internal/integration/skillrunner"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type skillPricingRequest struct {
	Mode            string   `json:"mode"`
	Amount          float64  `json:"amount"`
	Currency        string   `json:"currency"`
	SettlementRatio *float64 `json:"settlement_ratio"`
}

type skillUpsertRequest struct {
	Slug            string              `json:"slug"`
	Name            string              `json:"name" binding:"required"`
	Tagline         string              `json:"tagline"`
	Description     *string             `json:"description"`
	Type            string              `json:"type" binding:"required"`
	Visibility      string              `json:"visibility"`
	Status          string              `json:"status"`
	Category        *string             `json:"category"`
	CoverImageURL   *string             `json:"cover_image_url"`
	Tags            []string            `json:"tags"`
	Pricing         skillPricingRequest `json:"pricing"`
	PriceMode       string              `json:"price_mode"`
	PriceAmount     *float64            `json:"price_amount"`
	Currency        *string             `json:"currency"`
	SettlementRatio *float64            `json:"settlement_ratio"`
	SourceLocked    bool                `json:"source_locked"`
	VariableSchema  []map[string]any    `json:"variable_schema"`
	Content         map[string]any      `json:"content"`
	Readme          *string             `json:"readme"`
	InstallNote     *string             `json:"install_note"`
	Trace           aiTraceRequest      `json:"trace"`
}

type skillVersionRequest struct {
	Version        string           `json:"version"`
	Status         string           `json:"status"`
	Changelog      string           `json:"changelog"`
	SourceLocked   bool             `json:"source_locked"`
	VariableSchema []map[string]any `json:"variable_schema"`
	Content        map[string]any   `json:"content"`
	Trace          aiTraceRequest   `json:"trace"`
}

type skillRunRequest struct {
	VersionID      *int64           `json:"version_id"`
	Mode           string           `json:"mode"`
	Parameters     map[string]any   `json:"parameters"`
	Attachments    []map[string]any `json:"attachments"`
	IdempotencyKey string           `json:"idempotency_key"`
	Trace          aiTraceRequest   `json:"trace"`
}

func (h *AIHandler) ListSkills(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	params := skillPagination(c)
	scope := normalizeSkillScope(c.Query("scope"))
	filter := domain.AISkillListFilter{
		Scope:         scope,
		SkillType:     strings.TrimSpace(c.Query("type")),
		Category:      strings.TrimSpace(c.Query("category")),
		Status:        normalizeSkillStatus(c.Query("status")),
		Visibility:    normalizeSkillVisibility(c.Query("visibility")),
		Search:        strings.TrimSpace(c.Query("search")),
		PublishedOnly: scope == domain.AISkillScopeLibrary,
		Installed:     normalizeSkillInstalledFilter(c.Query("installed")),
	}
	if priceMode := strings.TrimSpace(c.Query("price_mode")); priceMode != "" && !strings.EqualFold(priceMode, "all") {
		filter.PriceMode = strings.ToLower(priceMode)
	}

	items, result, err := module.DomainRepo.ListSkills(c.Request.Context(), subject.UserID, false, params, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	installStates, installCounts, err := loadSkillInstallMaps(c.Request.Context(), module, subject.UserID, items)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.SkillSummary, 0, len(items))
	for i := range items {
		skill := &items[i]
		applySkillInstallMetadata(skill, installStates[skill.ID], installCounts[skill.ID])
		latestVersion, currentVersion, _ := loadSkillVersionPointers(c.Request.Context(), module, skill, subject.UserID)
		viewSkill := skillViewForViewer(skill, subject.UserID)
		out = append(out, *dto.SkillSummaryFromDomain(
			viewSkill,
			subject.UserID,
			domain.CanReadAISkillSource(skill.UserID, subject.UserID, false, skill.Visibility, skill.SourceVisibility, skill.Price, skill.PublishedVersionID),
			latestVersion,
			currentVersion,
		))
	}
	response.PaginatedWithResult(c, out, toResponsePagination(result))
}

func (h *AIHandler) GetSkillDetail(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	skill, canViewSource, err := loadSkillForViewer(c.Request.Context(), module, skillID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := applySkillInstallMetadataForViewer(c.Request.Context(), module, subject.UserID, skill); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	latestVersion, currentVersion, err := loadSkillVersionPointers(c.Request.Context(), module, skill, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SkillDetailFromDomain(skillViewForViewer(skill, subject.UserID), subject.UserID, canViewSource, latestVersion, currentVersion))
}

func (h *AIHandler) CreateSkill(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	var req skillUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := validateUserSkillCreateType(req.Type); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	executeUserIdempotentJSON(c, "skills:create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		metadata, cleanupIDs, err := h.buildSkillMetadata(ctx, subject.UserID, req, nil)
		if err != nil {
			return nil, err
		}
		created, err := module.SkillService.CreateSkill(ctx, subject.UserID, &service.AICreateSkillInput{
			Name:        strings.TrimSpace(req.Name),
			Description: req.Description,
			Type:        strings.TrimSpace(req.Type),
			Metadata:    metadata,
			Trace:       buildUserAITrace(req.Trace),
		})
		if err != nil {
			h.cleanupSkillMedia(ctx, subject.UserID, cleanupIDs)
			return nil, err
		}
		skill, loadErr := module.DomainRepo.GetSkillByUserAndID(ctx, subject.UserID, created.ID)
		if loadErr != nil {
			return nil, loadErr
		}
		return dto.SkillDetailFromDomain(skill, subject.UserID, true, nil, nil), nil
	})
}

func validateUserSkillCreateType(rawType string) error {
	if strings.EqualFold(strings.TrimSpace(rawType), service.AISkillTypeScript) {
		return infraerrors.BadRequest("AI_SKILL_SCRIPT_CREATION_UNSUPPORTED", "script skills cannot be created until the runtime executor is available")
	}
	return nil
}

func (h *AIHandler) UpdateSkill(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	var req skillUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "skills:update", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		current, err := module.DomainRepo.GetSkillByUserAndID(ctx, subject.UserID, skillID)
		if err != nil {
			return nil, err
		}
		metadata, cleanupIDs, err := h.buildSkillMetadata(ctx, subject.UserID, req, current.Metadata)
		if err != nil {
			return nil, err
		}
		updated, err := module.SkillService.UpdateSkill(ctx, subject.UserID, skillID, &service.AIUpdateSkillInput{
			Name:        stringPtrNullable(req.Name),
			Description: stringDoublePtr(req.Description),
			Metadata:    mapPtr(metadata),
			Trace:       buildUserAITrace(req.Trace),
		})
		if err != nil {
			h.cleanupSkillMedia(ctx, subject.UserID, cleanupIDs)
			return nil, err
		}
		skill, loadErr := module.DomainRepo.GetSkillByUserAndID(ctx, subject.UserID, updated.ID)
		if loadErr != nil {
			return nil, loadErr
		}
		latestVersion, currentVersion, loadErr := loadSkillVersionPointers(ctx, module, skill, subject.UserID)
		if loadErr != nil {
			return nil, loadErr
		}
		return dto.SkillDetailFromDomain(skill, subject.UserID, true, latestVersion, currentVersion), nil
	})
}

func (h *AIHandler) InstallSkill(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	if _, _, err := loadSkillForViewer(c.Request.Context(), module, skillID, subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := module.DomainRepo.SetSkillInstall(c.Request.Context(), skillID, subject.UserID, true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	counts, err := module.DomainRepo.GetSkillInstallCounts(c.Request.Context(), []int64{skillID})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"message":       "ok",
		"skill_id":      skillID,
		"installed":     true,
		"install_count": counts[skillID],
	})
}

func (h *AIHandler) UninstallSkill(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	if _, _, err := loadSkillForViewer(c.Request.Context(), module, skillID, subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := module.DomainRepo.SetSkillInstall(c.Request.Context(), skillID, subject.UserID, false); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	counts, err := module.DomainRepo.GetSkillInstallCounts(c.Request.Context(), []int64{skillID})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"message":       "ok",
		"skill_id":      skillID,
		"installed":     false,
		"install_count": counts[skillID],
	})
}

func (h *AIHandler) ListSkillVersions(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	skill, canViewSource, err := loadSkillForViewer(c.Request.Context(), module, skillID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	versions, err := module.DomainRepo.ListSkillVersions(c.Request.Context(), skill.ID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.SkillVersionRecord, 0, len(versions))
	currentVersionID := skill.CurrentVersionID
	if skill.UserID != subject.UserID {
		currentVersionID = skill.PublishedVersionID
	}
	for i := range versions {
		version := &versions[i]
		if !skillVersionVisibleToViewer(skill.UserID, subject.UserID, version) {
			continue
		}
		out = append(out, *dto.SkillVersionRecordFromDomain(version, currentVersionID, canViewSource))
	}
	page, pageSize := response.ParsePagination(c)
	result := paginateVersionRecords(out, page, pageSize)
	response.PaginatedWithResult(c, result, toResponsePagination(&pagination.PaginationResult{
		Total:    int64(len(out)),
		Page:     page,
		PageSize: pageSize,
		Pages:    pageCount(len(out), pageSize),
	}))
}

func (h *AIHandler) CreateSkillVersion(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	skill, err := module.DomainRepo.GetSkillByUserAndID(c.Request.Context(), subject.UserID, skillID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req skillVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "skills:versions:create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		created, err := module.VersionService.CreateVersion(ctx, subject.UserID, skill.ID, buildCreateVersionInput(skill, req))
		if err != nil {
			return nil, err
		}
		if normalizeVersionStatusRequest(req.Status) == "published" {
			created, err = module.VersionService.SubmitVersion(ctx, subject.UserID, created.ID, &service.AISubmitSkillVersionInput{
				Comment: stringPtrNullable(req.Changelog),
				Trace:   buildUserAITrace(req.Trace),
			})
			if err != nil {
				return nil, err
			}
		}
		version, loadErr := module.DomainRepo.GetSkillVersionByID(ctx, created.ID)
		if loadErr != nil {
			return nil, loadErr
		}
		return dto.SkillVersionRecordFromDomain(version, skill.CurrentVersionID, true), nil
	})
}

func (h *AIHandler) UpdateSkillVersion(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	versionID, ok := parseUserAIID(c.Param("versionId"))
	if !ok {
		response.BadRequest(c, "Invalid version ID")
		return
	}
	skill, err := module.DomainRepo.GetSkillByUserAndID(c.Request.Context(), subject.UserID, skillID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req skillVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "skills:versions:update", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		currentVersion, err := module.DomainRepo.GetSkillVersionByID(ctx, versionID)
		if err != nil {
			return nil, err
		}
		if currentVersion.SkillID != skill.ID {
			return nil, service.ErrAISkillVersionNotFound
		}
		mergedReq := mergeSkillVersionRequest(req, currentVersion)
		updated, err := module.VersionService.UpdateVersionDraft(ctx, subject.UserID, versionID, buildUpdateVersionInput(skill, currentVersion.Metadata, mergedReq))
		if err != nil {
			return nil, err
		}
		if normalizeVersionStatusRequest(mergedReq.Status) == "published" {
			updated, err = module.VersionService.SubmitVersion(ctx, subject.UserID, updated.ID, &service.AISubmitSkillVersionInput{
				Comment: stringPtrNullable(mergedReq.Changelog),
				Trace:   buildUserAITrace(mergedReq.Trace),
			})
			if err != nil {
				return nil, err
			}
		}
		version, loadErr := module.DomainRepo.GetSkillVersionByID(ctx, updated.ID)
		if loadErr != nil {
			return nil, loadErr
		}
		return dto.SkillVersionRecordFromDomain(version, skill.CurrentVersionID, true), nil
	})
}

func (h *AIHandler) SubmitSkillVersion(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	versionID, ok := parseUserAIID(c.Param("versionId"))
	if !ok {
		response.BadRequest(c, "Invalid version ID")
		return
	}
	executeUserIdempotentJSON(c, "skills:versions:submit", map[string]any{"id": versionID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		currentVersion, err := module.DomainRepo.GetSkillVersionByID(ctx, versionID)
		if err != nil {
			return nil, err
		}
		if currentVersion.SkillID != skillID {
			return nil, service.ErrAISkillVersionNotFound
		}
		version, err := module.VersionService.SubmitVersion(ctx, subject.UserID, versionID, &service.AISubmitSkillVersionInput{})
		if err != nil {
			return nil, err
		}
		entity, err := module.DomainRepo.GetSkillVersionByID(ctx, version.ID)
		if err != nil {
			return nil, err
		}
		skill, err := module.DomainRepo.GetSkillByUserAndID(ctx, subject.UserID, version.SkillID)
		if err != nil {
			return nil, err
		}
		return dto.SkillVersionRecordFromDomain(entity, skill.CurrentVersionID, true), nil
	})
}

func (h *AIHandler) ListSkillRuns(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	if _, err := module.DomainRepo.GetSkillByUserAndID(c.Request.Context(), subject.UserID, skillID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	params := listParams(c, "created_at", "desc")
	versionID := parseOptionalUserAIID(c.Query("version_id"))
	items, result, err := module.Queries.ListSkillRuns(c.Request.Context(), subject.UserID, skillID, params, versionID, c.Query("status"), c.Query("search"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.SkillRunRecord, 0, len(items))
	for _, item := range items {
		out = append(out, dto.SkillRunRecordFromQuery(item))
	}
	response.PaginatedWithResult(c, out, toResponsePagination(result))
}

func (h *AIHandler) RunSkill(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	var req skillRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	idempotencyKey := normalizeSkillRunIdempotencyKey(c.GetHeader("Idempotency-Key"), req.IdempotencyKey)
	if idempotencyKey != "" {
		c.Request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	executeUserIdempotentJSON(c, "skills:runs:create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		skill, _, err := loadSkillForViewer(ctx, module, skillID, subject.UserID)
		if err != nil {
			return nil, err
		}
		versionID, err := resolveSkillRunVersionForViewer(skill, req.VersionID, subject.UserID)
		if err != nil {
			return nil, err
		}
		attachments, cleanupIDs, err := h.buildRunAttachments(ctx, subject.UserID, req.Attachments)
		if err != nil {
			return nil, err
		}
		mode := strings.TrimSpace(req.Mode)
		trace := buildUserAITrace(req.Trace)
		if mode == service.AISkillRunModeUse && (trace.APIKeyID == nil || *trace.APIKeyID <= 0) {
			return nil, infraerrors.BadRequest("AI_SKILL_API_KEY_REQUIRED", "use mode requires trace.api_key_id for token billing")
		}
		result, err := module.RunService.Execute(ctx, subject.UserID, &service.AISkillRunInput{
			SkillID:        skillID,
			VersionID:      versionID,
			Mode:           mode,
			Parameters:     req.Parameters,
			Attachments:    attachments,
			IdempotencyKey: idempotencyKey,
			Trace:          trace,
		})
		if err != nil {
			h.cleanupSkillMedia(ctx, subject.UserID, cleanupIDs)
			return nil, err
		}
		return result, nil
	})
}

func (h *AIHandler) TestSkill(c *gin.Context) {
	h.runSkillWithMode(c, service.AISkillRunModeTest)
}

func (h *AIHandler) UseSkill(c *gin.Context) {
	h.runSkillWithMode(c, service.AISkillRunModeUse)
}

func (h *AIHandler) runSkillWithMode(c *gin.Context, mode string) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	var req skillRunRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request body")
			return
		}
	}
	idempotencyKey := normalizeSkillRunIdempotencyKey(c.GetHeader("Idempotency-Key"), req.IdempotencyKey)
	if idempotencyKey != "" {
		c.Request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	versionID := parseOptionalUserAIID(c.Query("version_id"))
	parameters := req.Parameters
	if parameters == nil {
		parameters = map[string]any{}
	}
	executeUserIdempotentJSON(c, "skills:runs:"+mode, map[string]any{
		"skill_id":    skillID,
		"version_id":  versionID,
		"mode":        mode,
		"parameters":  parameters,
		"attachments": req.Attachments,
		"trace":       req.Trace,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		skill, _, err := loadSkillForViewer(ctx, module, skillID, subject.UserID)
		if err != nil {
			return nil, err
		}
		versionID, err := resolveSkillRunVersionForViewer(skill, versionID, subject.UserID)
		if err != nil {
			return nil, err
		}
		attachments, cleanupIDs, err := h.buildRunAttachments(ctx, subject.UserID, req.Attachments)
		if err != nil {
			return nil, err
		}
		trace := buildUserAITrace(req.Trace)
		// use mode requires a buyer API key so upstream tokens are billed to the user.
		if mode == service.AISkillRunModeUse && (trace.APIKeyID == nil || *trace.APIKeyID <= 0) {
			return nil, infraerrors.BadRequest("AI_SKILL_API_KEY_REQUIRED", "use mode requires trace.api_key_id for token billing")
		}
		result, err := module.RunService.Execute(ctx, subject.UserID, &service.AISkillRunInput{
			SkillID:        skillID,
			VersionID:      versionID,
			Mode:           mode,
			Parameters:     parameters,
			Attachments:    attachments,
			IdempotencyKey: idempotencyKey,
			Trace:          trace,
		})
		if err != nil {
			h.cleanupSkillMedia(ctx, subject.UserID, cleanupIDs)
			return nil, err
		}
		return result, nil
	})
}

func normalizeSkillRunIdempotencyKey(headerValue, bodyValue string) string {
	headerKey := strings.TrimSpace(headerValue)
	if headerKey != "" {
		return headerKey
	}
	return strings.TrimSpace(bodyValue)
}

func (h *AIHandler) PublishSkillVersion(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	versionID, ok := parseUserAIID(c.Param("versionId"))
	if !ok {
		response.BadRequest(c, "Invalid version ID")
		return
	}
	executeUserIdempotentJSON(c, "skills:versions:publish", map[string]any{"version_id": versionID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		version, err := module.DomainRepo.GetSkillVersionByID(ctx, versionID)
		if err != nil {
			return nil, err
		}
		if version.UserID != subject.UserID {
			return nil, infraerrors.NotFound("AI_SKILL_VERSION_NOT_FOUND", "ai skill version not found")
		}
		if !domain.CanPublishAISkillVersion(version.ReviewStatus) {
			return nil, domain.ErrAISkillVersionNotApproved
		}
		skill, err := module.DomainRepo.GetSkillByUserAndID(ctx, subject.UserID, version.SkillID)
		if err != nil {
			return nil, err
		}
		currentVersionID := version.ID
		skill.CurrentVersionID = &currentVersionID
		skill.PublishedVersionID = &currentVersionID
		skill.LatestApprovedVersionID = &currentVersionID
		if err := module.DomainRepo.UpdateSkill(ctx, skill); err != nil {
			return nil, err
		}
		return dto.SkillVersionRecordFromDomain(version, skill.CurrentVersionID, true), nil
	})
}

func (h *AIHandler) GetSkillRevenue(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	if _, err := module.DomainRepo.GetSkillByUserAndID(c.Request.Context(), subject.UserID, skillID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	detail, err := module.Queries.GetSkillRevenue(c.Request.Context(), subject.UserID, skillID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SkillRevenueDetailFromQuery(detail))
}

func loadSkillForViewer(ctx context.Context, module *skillkit.Module, skillID, viewerUserID int64) (*domain.AISkill, bool, error) {
	skill, err := module.DomainRepo.GetSkillByID(ctx, skillID)
	if err != nil {
		return nil, false, err
	}
	canRead := domain.CanReadAISkill(skill.UserID, viewerUserID, false, skill.Visibility, skill.PublishedVersionID)
	if !canRead {
		return nil, false, infraerrors.Forbidden("AI_SKILL_ACCESS_DENIED", "not authorized to access this skill")
	}
	canViewSource := domain.CanReadAISkillSource(skill.UserID, viewerUserID, false, skill.Visibility, skill.SourceVisibility, skill.Price, skill.PublishedVersionID)
	return skill, canViewSource, nil
}

func loadSkillInstallMaps(ctx context.Context, module *skillkit.Module, viewerUserID int64, skills []domain.AISkill) (map[int64]bool, map[int64]int, error) {
	skillIDs := make([]int64, 0, len(skills))
	for i := range skills {
		if skills[i].ID > 0 {
			skillIDs = append(skillIDs, skills[i].ID)
		}
	}
	installStates, err := module.DomainRepo.GetSkillInstallStates(ctx, viewerUserID, skillIDs)
	if err != nil {
		return nil, nil, err
	}
	installCounts, err := module.DomainRepo.GetSkillInstallCounts(ctx, skillIDs)
	if err != nil {
		return nil, nil, err
	}
	return installStates, installCounts, nil
}

func applySkillInstallMetadataForViewer(ctx context.Context, module *skillkit.Module, viewerUserID int64, skill *domain.AISkill) error {
	if skill == nil {
		return nil
	}
	installed, err := module.DomainRepo.HasSkillInstall(ctx, skill.ID, viewerUserID)
	if err != nil {
		return err
	}
	counts, err := module.DomainRepo.GetSkillInstallCounts(ctx, []int64{skill.ID})
	if err != nil {
		return err
	}
	applySkillInstallMetadata(skill, installed, counts[skill.ID])
	return nil
}

func applySkillInstallMetadata(skill *domain.AISkill, installed bool, installCount int) {
	if skill == nil {
		return
	}
	if skill.Metadata == nil {
		skill.Metadata = map[string]any{}
	}
	skill.Metadata["installed"] = installed
	skill.Metadata["install_count"] = installCount
}

func loadSkillVersionPointers(ctx context.Context, module *skillkit.Module, skill *domain.AISkill, viewerUserID int64) (*domain.AISkillVersion, *domain.AISkillVersion, error) {
	if skill == nil {
		return nil, nil, nil
	}
	versions, err := module.DomainRepo.ListSkillVersions(ctx, skill.ID)
	if err != nil {
		return nil, nil, err
	}
	latestVersion, currentVersion := selectSkillVersionPointersForViewer(skill, versions, viewerUserID)
	return latestVersion, currentVersion, nil
}

func selectSkillVersionPointersForViewer(skill *domain.AISkill, versions []domain.AISkillVersion, viewerUserID int64) (*domain.AISkillVersion, *domain.AISkillVersion) {
	if skill == nil {
		return nil, nil
	}
	var latestVersion *domain.AISkillVersion
	var currentVersion *domain.AISkillVersion
	var publishedVersion *domain.AISkillVersion
	owned := skill.UserID > 0 && skill.UserID == viewerUserID
	for i := range versions {
		version := versions[i]
		if owned && latestVersion == nil {
			copyVersion := version
			latestVersion = &copyVersion
		}
		if skill.CurrentVersionID != nil && *skill.CurrentVersionID == version.ID {
			copyVersion := version
			currentVersion = &copyVersion
		}
		if skill.PublishedVersionID != nil && *skill.PublishedVersionID == version.ID {
			copyVersion := version
			publishedVersion = &copyVersion
		}
		if !owned && latestVersion == nil && skillVersionVisibleToViewer(skill.UserID, viewerUserID, &version) {
			copyVersion := version
			latestVersion = &copyVersion
		}
	}
	if !owned {
		if publishedVersion != nil && skillVersionVisibleToViewer(skill.UserID, viewerUserID, publishedVersion) {
			return publishedVersion, publishedVersion
		}
		return latestVersion, nil
	}
	if latestVersion == nil {
		latestVersion = publishedVersion
	}
	return latestVersion, currentVersion
}

func skillVersionVisibleToViewer(ownerUserID, viewerUserID int64, version *domain.AISkillVersion) bool {
	if version == nil {
		return false
	}
	if ownerUserID > 0 && ownerUserID == viewerUserID {
		return true
	}
	return domain.CanUseAISkillVersion(version.ReviewStatus)
}

func resolveSkillRunVersionForViewer(skill *domain.AISkill, requestedVersionID *int64, viewerUserID int64) (*int64, error) {
	if skill == nil {
		return nil, infraerrors.NotFound("AI_SKILL_NOT_FOUND", "ai skill not found")
	}
	if skill.UserID > 0 && skill.UserID == viewerUserID {
		return requestedVersionID, nil
	}
	if skill.PublishedVersionID == nil || *skill.PublishedVersionID <= 0 {
		return nil, infraerrors.Forbidden("AI_SKILL_ACCESS_DENIED", "not authorized to access this skill")
	}
	publishedVersionID := *skill.PublishedVersionID
	if requestedVersionID != nil && *requestedVersionID > 0 && *requestedVersionID != publishedVersionID {
		return nil, infraerrors.Forbidden("AI_SKILL_ACCESS_DENIED", "not authorized to access this skill")
	}
	return &publishedVersionID, nil
}

func skillViewForViewer(skill *domain.AISkill, viewerUserID int64) *domain.AISkill {
	if skill == nil {
		return nil
	}
	if skill.UserID > 0 && skill.UserID == viewerUserID {
		return skill
	}
	view := *skill
	if skill.PublishedVersionID == nil {
		view.CurrentVersionID = nil
		return &view
	}
	publishedVersionID := *skill.PublishedVersionID
	view.CurrentVersionID = &publishedVersionID
	return &view
}

func (h *AIHandler) buildSkillMetadata(ctx context.Context, userID int64, req skillUpsertRequest, base map[string]any) (map[string]any, []int64, error) {
	metadata := cloneStringAnyMap(base)
	cleanupIDs := make([]int64, 0, 1)
	metadata["slug"] = strings.TrimSpace(req.Slug)
	metadata["status"] = skillFirstNonEmpty(req.Status, "draft")
	metadata["tagline"] = strings.TrimSpace(req.Tagline)
	if req.CoverImageURL != nil {
		coverImage, err := h.storeSkillCoverImage(ctx, userID, trimPtr(req.CoverImageURL))
		if err != nil {
			return nil, nil, err
		}
		metadata["cover_image_url"] = coverImage.URL
		if coverImage.Created && coverImage.MediaID != nil {
			cleanupIDs = append(cleanupIDs, *coverImage.MediaID)
		}
	}
	metadata["source_locked"] = req.SourceLocked
	metadata["variable_schema"] = req.VariableSchema
	metadata["content"] = req.Content
	metadata["readme"] = trimPtr(req.Readme)
	metadata["install_note"] = trimPtr(req.InstallNote)
	metadata["pricing"] = buildPricingMetadata(req)
	if sourceCode := scriptSourceCode(req.Content); sourceCode != "" {
		metadata["source_code"] = sourceCode
	}
	if req.Category != nil {
		metadata["category"] = strings.TrimSpace(*req.Category)
	}
	if len(req.Tags) > 0 {
		metadata["tags"] = req.Tags
	}
	if visibility := normalizeSkillVisibility(req.Visibility); visibility != "" {
		metadata["visibility"] = visibility
	}
	return metadata, cleanupIDs, nil
}

func buildCreateVersionInput(skill *domain.AISkill, req skillVersionRequest) *service.AICreateSkillVersionInput {
	meta := map[string]any{
		"version_name":        strings.TrimSpace(req.Version),
		"source_locked":       req.SourceLocked,
		"variable_schema":     req.VariableSchema,
		"content":             req.Content,
		"presentation_status": normalizeVersionStatusRequest(req.Status),
	}
	if sourceCode := scriptSourceCode(req.Content); sourceCode != "" {
		meta["source_code"] = sourceCode
	}
	copySkillMetadata(meta, skill.Metadata)
	return &service.AICreateSkillVersionInput{
		ExecutionSpec: buildExecutionSpec(skill.Type, req.Content, req.VariableSchema),
		BillingPolicy: buildVersionBillingPolicy(skill.Metadata, skill.Price),
		ChangeNote:    stringPtrNullable(req.Changelog),
		Metadata:      meta,
		Trace:         buildUserAITrace(req.Trace),
	}
}

func buildUpdateVersionInput(skill *domain.AISkill, base map[string]any, req skillVersionRequest) *service.AIUpdateSkillVersionInput {
	meta := cloneStringAnyMap(base)
	meta["version_name"] = strings.TrimSpace(req.Version)
	meta["source_locked"] = req.SourceLocked
	meta["variable_schema"] = req.VariableSchema
	meta["content"] = req.Content
	meta["presentation_status"] = normalizeVersionStatusRequest(req.Status)
	if sourceCode := scriptSourceCode(req.Content); sourceCode != "" {
		meta["source_code"] = sourceCode
	}
	copySkillMetadata(meta, skill.Metadata)
	spec := buildExecutionSpec(skill.Type, req.Content, req.VariableSchema)
	billing := buildVersionBillingPolicy(skill.Metadata, skill.Price)
	return &service.AIUpdateSkillVersionInput{
		ExecutionSpec: &spec,
		BillingPolicy: &billing,
		ChangeNote:    stringDoublePtr(stringPtrNullable(req.Changelog)),
		Metadata:      &meta,
		Trace:         buildUserAITrace(req.Trace),
	}
}

func cloneStringAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func buildExecutionSpec(skillType string, content map[string]any, variableSchema []map[string]any) service.AISkillExecutionSpec {
	spec := service.AISkillExecutionSpec{Type: skillType}
	switch skillType {
	case service.AISkillTypePromptImage:
		spec.PromptImage = &service.AISkillPromptImageSpec{
			Model:                  mapString(content, "model"),
			PromptTemplate:         mapString(content, "prompt_template"),
			NegativePromptTemplate: mapString(content, "negative_prompt_template"),
			Size:                   mapString(content, "size"),
			ImageCount:             mapInt(content, "image_count", 1),
			Variables:              variableSchema,
		}
	case service.AISkillTypeScript:
		spec.Script = &service.AISkillScriptSpec{
			Runtime:        mapFirstString(content, "runtime", "runtime_id"),
			ScriptName:     mapFirstString(content, "language", "script_name", "name"),
			EntryPoint:     mapFirstString(content, "entrypoint", "entry"),
			Protocol:       skillrunner.ProtocolJSONFileV1,
			TimeoutSeconds: mapInt(content, "timeout_seconds", 30),
		}
	default:
		spec.Type = service.AISkillTypePromptChat
		spec.PromptChat = &service.AISkillPromptChatSpec{
			Model:              mapString(content, "model"),
			SystemPrompt:       mapString(content, "system_prompt"),
			UserPromptTemplate: mapString(content, "user_prompt_template"),
			Variables:          variableSchema,
		}
	}
	return spec
}

func buildVersionBillingPolicy(skillMetadata map[string]any, fallbackPrice float64) service.AISkillBillingPolicy {
	price := fallbackPrice
	commission := 0.3
	currency := "CNY"
	if pricing, ok := skillMetadata["pricing"].(map[string]any); ok {
		if amount, ok := pricing["amount"].(float64); ok && amount > 0 {
			price = amount
		}
		if ratio, ok := pricing["settlement_ratio"].(float64); ok && ratio >= 0 && ratio <= 1 {
			commission = 1 - ratio
		}
		if value, ok := pricing["currency"].(string); ok && strings.TrimSpace(value) != "" {
			currency = strings.TrimSpace(value)
		}
		if mode, ok := pricing["mode"].(string); ok && strings.TrimSpace(mode) == "free" {
			price = 0
		}
	}
	if price <= 0 {
		return service.AISkillBillingPolicy{Mode: service.AISkillBillingModeFree}
	}
	return service.AISkillBillingPolicy{
		Mode:                   service.AISkillBillingModePerRun,
		PricePerRun:            price,
		PlatformCommissionRate: commission,
		Currency:               currency,
	}
}

func mergeSkillVersionRequest(req skillVersionRequest, version *domain.AISkillVersion) skillVersionRequest {
	if version == nil {
		return req
	}
	meta := version.Metadata
	if strings.TrimSpace(req.Version) == "" {
		if value, ok := meta["version_name"].(string); ok {
			req.Version = value
		}
	}
	if strings.TrimSpace(req.Status) == "" {
		if value, ok := meta["presentation_status"].(string); ok {
			req.Status = value
		}
	}
	if strings.TrimSpace(req.Changelog) == "" {
		req.Changelog = version.ChangeNote
	}
	if len(req.VariableSchema) == 0 {
		if raw, ok := meta["variable_schema"].([]any); ok {
			req.VariableSchema = make([]map[string]any, 0, len(raw))
			for _, item := range raw {
				if record, ok := item.(map[string]any); ok {
					req.VariableSchema = append(req.VariableSchema, record)
				}
			}
		}
	}
	if len(req.Content) == 0 {
		if content, ok := meta["content"].(map[string]any); ok {
			req.Content = content
		}
	}
	return req
}

func (h *AIHandler) buildRunAttachments(ctx context.Context, userID int64, items []map[string]any) ([]service.AISkillRunAttachment, []int64, error) {
	if len(items) == 0 {
		return nil, nil, nil
	}
	out := make([]service.AISkillRunAttachment, 0, len(items))
	cleanupIDs := make([]int64, 0, len(items))
	for _, item := range items {
		attachment := service.AISkillRunAttachment{
			URL:      mapString(item, "url"),
			Purpose:  mapString(item, "purpose"),
			FileName: mapString(item, "file_name"),
		}
		if id, ok := int64FromAny(item["asset_id"]); ok {
			attachment.AssetID = &id
		}
		if id, ok := int64FromAny(item["media_id"]); ok {
			attachment.MediaID = &id
		}
		if attachment.AssetID == nil && attachment.MediaID == nil {
			storedMedia, err := h.storeSkillAttachmentMedia(ctx, userID, attachment.URL, attachment.FileName)
			if err != nil {
				return nil, nil, err
			}
			attachment.URL = storedMedia.URL
			if storedMedia.MediaID != nil {
				attachment.MediaID = storedMedia.MediaID
				if storedMedia.Created {
					cleanupIDs = append(cleanupIDs, *storedMedia.MediaID)
				}
			}
		} else if strings.TrimSpace(attachment.URL) == "" {
			attachment.URL = h.skillManagedMediaURL(ctx, userID, attachment.MediaID, attachment.AssetID)
		}
		out = append(out, attachment)
	}
	return out, cleanupIDs, nil
}

func (h *AIHandler) normalizeSkillCoverImage(ctx context.Context, req *skillUpsertRequest, bizID string) error {
	if req == nil || req.CoverImageURL == nil {
		return nil
	}
	value := strings.TrimSpace(*req.CoverImageURL)
	if value == "" {
		return nil
	}
	storedMedia, err := h.storeSkillCoverImage(ctx, 0, value)
	if err != nil {
		return err
	}
	if strings.TrimSpace(storedMedia.URL) != "" {
		req.CoverImageURL = &storedMedia.URL
	}
	return nil
}

func (h *AIHandler) buildRunAttachmentsWithMedia(ctx context.Context, userID int64, items []map[string]any) ([]service.AISkillRunAttachment, error) {
	attachments, _, err := h.buildRunAttachments(ctx, userID, items)
	return attachments, err
}

func (h *AIHandler) storeSkillCoverImage(ctx context.Context, userID int64, source string) (skillMediaReference, error) {
	return h.storeSkillMediaReferenceWithIDBestEffort(ctx, userID, "ai_skill_cover", source, "")
}

func (h *AIHandler) storeSkillAttachmentMedia(ctx context.Context, userID int64, source, fileName string) (skillMediaReference, error) {
	return h.storeSkillMediaReferenceWithIDBestEffort(ctx, userID, "ai_skill_run_attachment", source, fileName)
}

func (h *AIHandler) storeSkillMediaReferenceWithIDBestEffort(ctx context.Context, userID int64, bizType, source, fileName string) (skillMediaReference, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return skillMediaReference{}, nil
	}
	storedMedia, err := h.storeSkillMediaReferenceWithID(ctx, userID, bizType, source, fileName)
	if err != nil {
		return skillMediaReference{URL: source}, nil
	}
	if strings.TrimSpace(storedMedia.URL) == "" {
		storedMedia.URL = source
	}
	return storedMedia, nil
}

type skillMediaReference struct {
	URL     string
	MediaID *int64
	Created bool
}

func (h *AIHandler) storeSkillMediaReferenceWithID(ctx context.Context, userID int64, bizType, source, fileName string) (skillMediaReference, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return skillMediaReference{}, nil
	}
	if h == nil || h.mediaService == nil || !h.mediaService.RuntimeInfo().Enabled {
		return skillMediaReference{URL: source}, nil
	}
	if mediaID, ok := service.ParseManagedMediaID(h.mediaService, source); ok {
		return skillMediaReference{URL: source, MediaID: &mediaID}, nil
	}
	bizID := fmt.Sprintf("user-%d", userID)
	var ownerUserID *int64
	if userID > 0 {
		ownerUserID = &userID
	} else {
		bizID = bizType
	}
	asset, err := h.mediaService.IngestImageReference(ctx, service.IngestImageReferenceInput{
		BizType:     bizType,
		BizID:       bizID,
		Visibility:  service.MediaVisibilityPublic,
		OwnerUserID: ownerUserID,
		Source:      source,
		FileName:    fileName,
	})
	if err != nil {
		return skillMediaReference{}, err
	}
	storedURL := h.mediaService.PublicURL(asset)
	if strings.TrimSpace(storedURL) == "" {
		storedURL = source
	}
	return skillMediaReference{URL: storedURL, MediaID: &asset.ID, Created: true}, nil
}

func (h *AIHandler) cleanupSkillMedia(ctx context.Context, userID int64, mediaIDs []int64) {
	if h == nil || h.mediaService == nil || userID <= 0 || len(mediaIDs) == 0 {
		return
	}
	seen := make(map[int64]struct{}, len(mediaIDs))
	for _, mediaID := range mediaIDs {
		if mediaID <= 0 {
			continue
		}
		if _, ok := seen[mediaID]; ok {
			continue
		}
		seen[mediaID] = struct{}{}
		_ = h.mediaService.DeleteForUser(ctx, userID, mediaID)
	}
}

func (h *AIHandler) skillManagedMediaURL(ctx context.Context, userID int64, primaryID, fallbackID *int64) string {
	if h == nil || h.mediaService == nil || userID <= 0 {
		return ""
	}
	for _, id := range []*int64{primaryID, fallbackID} {
		if id == nil || *id <= 0 {
			continue
		}
		asset, err := h.mediaService.GetForUser(ctx, userID, *id)
		if err != nil || asset == nil {
			continue
		}
		if asset.OwnerUserID != nil && *asset.OwnerUserID != userID {
			continue
		}
		if url := h.mediaService.PublicURL(asset); strings.TrimSpace(url) != "" {
			return url
		}
	}
	return ""
}

func int64FromAny(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case string:
		id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			return id, true
		}
	}
	return 0, false
}

func skillPagination(c *gin.Context) pagination.PaginationParams {
	params := listParams(c, "updated_at", "desc")
	switch strings.TrimSpace(c.Query("sort")) {
	case "popular":
		params.SortBy = "like_count"
		params.SortOrder = "desc"
	case "runs", "revenue":
		params.SortBy = "run_count"
		params.SortOrder = "desc"
	case "price_low":
		params.SortBy = "price"
		params.SortOrder = "asc"
	case "price_high":
		params.SortBy = "price"
		params.SortOrder = "desc"
	case "latest":
		params.SortBy = "updated_at"
		params.SortOrder = "desc"
	}
	return params
}

func listParams(c *gin.Context, defaultSortBy, defaultSortOrder string) pagination.PaginationParams {
	page, pageSize := response.ParsePagination(c)
	return pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", defaultSortBy),
		SortOrder: c.DefaultQuery("sort_order", defaultSortOrder),
	}
}

func normalizeSkillScope(raw string) string {
	if strings.TrimSpace(raw) == "market" {
		return domain.AISkillScopeLibrary
	}
	return domain.AISkillScopeMine
}

func normalizeSkillVisibility(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "public":
		return domain.AIVisibilityPublic
	case "private":
		return domain.AIVisibilityPrivate
	default:
		return ""
	}
}

func normalizeSkillStatus(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "draft", "published", "archived", "hidden":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return ""
	}
}

func normalizeSkillInstalledFilter(raw string) *bool {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "installed", "true", "1", "yes":
		value := true
		return &value
	case "not_installed", "false", "0", "no":
		value := false
		return &value
	default:
		return nil
	}
}

func normalizeVersionStatusRequest(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "":
		return "draft"
	case "draft":
		return "draft"
	case "published":
		return "published"
	case "deprecated":
		return "deprecated"
	case "archived":
		return "archived"
	default:
		return "draft"
	}
}

func buildPricingMetadata(req skillUpsertRequest) map[string]any {
	mode := strings.TrimSpace(req.Pricing.Mode)
	if mode == "" {
		mode = strings.TrimSpace(req.PriceMode)
	}
	amount := req.Pricing.Amount
	if amount == 0 && req.PriceAmount != nil {
		amount = *req.PriceAmount
	}
	currency := skillFirstNonEmpty(req.Pricing.Currency, trimPtr(req.Currency), "CNY")
	settlementRatio := req.Pricing.SettlementRatio
	if settlementRatio == nil {
		settlementRatio = req.SettlementRatio
	}
	var ratioValue any
	if settlementRatio != nil {
		ratioValue = *settlementRatio
	}
	return map[string]any{
		"mode":             skillFirstNonEmpty(mode, "free"),
		"amount":           amount,
		"currency":         currency,
		"settlement_ratio": ratioValue,
	}
}

func scriptSourceCode(content map[string]any) string {
	return mapFirstRawString(content, "source_code", "code", "script")
}

func mapFirstString(src map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := mapString(src, key); value != "" {
			return value
		}
	}
	return ""
}

func mapFirstRawString(src map[string]any, keys ...string) string {
	if src == nil {
		return ""
	}
	for _, key := range keys {
		value, ok := src[key]
		if !ok || value == nil {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func mapString(src map[string]any, key string) string {
	if src == nil {
		return ""
	}
	value, ok := src[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func mapInt(src map[string]any, key string, fallback int) int {
	if src == nil {
		return fallback
	}
	value, ok := src[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return fallback
	}
}

func trimPtr(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func stringPtrNullable(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringDoublePtr(value *string) **string {
	if value == nil {
		return nil
	}
	return &value
}

func mapPtr(value map[string]any) *map[string]any {
	return &value
}

func skillFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func copySkillMetadata(dst map[string]any, src map[string]any) {
	if dst == nil || src == nil {
		return
	}
	if pricing, ok := src["pricing"]; ok {
		dst["pricing"] = pricing
	}
}

func paginateVersionRecords(items []dto.SkillVersionRecord, page, pageSize int) []dto.SkillVersionRecord {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []dto.SkillVersionRecord{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func pageCount(total, pageSize int) int {
	if pageSize <= 0 {
		pageSize = 20
	}
	if total == 0 {
		return 1
	}
	pages := total / pageSize
	if total%pageSize != 0 {
		pages++
	}
	if pages <= 0 {
		return 1
	}
	return pages
}
