package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/skillkit"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AIHandler struct {
	aiService   *service.AICenterService
	skillModule *skillkit.Module
}

func NewAIHandler(aiService *service.AICenterService, skillModule *skillkit.Module) *AIHandler {
	return &AIHandler{aiService: aiService, skillModule: skillModule}
}

func (h *AIHandler) skillModuleOrErr() (*skillkit.Module, error) {
	if h != nil && h.skillModule != nil {
		return h.skillModule, nil
	}
	return skillkit.Default()
}

type aiTraceRequest struct {
	RequestID  string `json:"request_id"`
	UsageLogID *int64 `json:"usage_log_id"`
	APIKeyID   *int64 `json:"api_key_id"`
	GroupID    *int64 `json:"group_id"`
}

type aiAdminModerationRequest struct {
	ModerationState string         `json:"moderation_state"`
	Reason          string         `json:"reason"`
	Trace           aiTraceRequest `json:"trace"`
}

type aiAdminGenerationJobRequest struct {
	Status         *string         `json:"status"`
	Model          *string         `json:"model"`
	Prompt         *string         `json:"prompt"`
	NegativePrompt *string         `json:"negative_prompt"`
	Size           *string         `json:"size"`
	ImageCount     *int            `json:"image_count"`
	Seed           *int64          `json:"seed"`
	Parameters     *map[string]any `json:"parameters"`
	ErrorMessage   *string         `json:"error_message"`
	Trace          aiTraceRequest  `json:"trace"`
}

type legacyAdminUpdatePromptRequest struct {
	Title       *string   `json:"title"`
	Content     *string   `json:"content"`
	Description **string  `json:"description"`
	Tags        *[]string `json:"tags"`
	Visibility  *string   `json:"visibility"`
	Status      *string   `json:"status"`
	LineID      **int64   `json:"line_id"`
}

type legacyAdminUpdateArtworkRequest struct {
	Title      *string   `json:"title"`
	Visibility *string   `json:"visibility"`
	Status     *string   `json:"status"`
	Featured   *bool     `json:"featured"`
	Tags       *[]string `json:"tags"`
}

func (h *AIHandler) ListPromptTemplates(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "updated_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	filter := service.AIListPromptTemplatesFilter{
		Scope:           strings.TrimSpace(c.DefaultQuery("scope", "all")),
		Visibility:      strings.TrimSpace(c.Query("visibility")),
		ModerationState: strings.TrimSpace(c.Query("moderation_state")),
		Search:          strings.TrimSpace(c.Query("search")),
	}
	items, result, err := h.aiService.AdminListPromptTemplates(c.Request.Context(), subject.UserID, params, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) GetPromptTemplate(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	templateID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	template, err := h.aiService.AdminGetPromptTemplate(c.Request.Context(), templateID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, template)
}

func (h *AIHandler) ModeratePromptTemplate(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	templateID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	var req aiAdminModerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeAdminIdempotentJSON(c, "ai:moderate-prompt-template", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.aiService.AdminModeratePromptTemplate(ctx, subject.UserID, templateID, req.ModerationState, req.Reason)
	})
}

func (h *AIHandler) ListGenerationJobs(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "updated_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	filter := service.AIListGenerationJobsFilter{
		Status:           strings.TrimSpace(c.Query("status")),
		SessionID:        parseOptionalAIID(c.Query("session_id")),
		PromptTemplateID: parseOptionalAIID(c.Query("prompt_template_id")),
	}
	items, result, err := h.aiService.AdminListGenerationJobs(c.Request.Context(), subject.UserID, params, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) GetGenerationJob(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	jobID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid job ID")
		return
	}
	job, err := h.aiService.AdminGetGenerationJob(c.Request.Context(), jobID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, job)
}

func (h *AIHandler) ModerateGenerationJob(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	jobID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid job ID")
		return
	}
	var req aiAdminGenerationJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeAdminIdempotentJSON(c, "ai:moderate-generation-job", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.aiService.AdminModerateGenerationJob(ctx, subject.UserID, jobID, &service.AIUpdateGenerationJobInput{
			Status:         req.Status,
			Model:          req.Model,
			Prompt:         req.Prompt,
			NegativePrompt: req.NegativePrompt,
			Size:           req.Size,
			ImageCount:     req.ImageCount,
			Seed:           req.Seed,
			Parameters:     req.Parameters,
			ErrorMessage:   req.ErrorMessage,
			Trace:          buildAITrace(req.Trace),
		})
	})
}

func (h *AIHandler) ListAssets(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	filter := service.AIListAssetsFilter{
		Status:           strings.TrimSpace(c.Query("status")),
		Visibility:       strings.TrimSpace(c.Query("visibility")),
		ModerationState:  strings.TrimSpace(c.Query("moderation_state")),
		GenerationJobID:  parseOptionalAIID(c.Query("generation_job_id")),
		SessionID:        parseOptionalAIID(c.Query("session_id")),
		PromptTemplateID: parseOptionalAIID(c.Query("prompt_template_id")),
	}
	ctx := contextWithRequestBaseURL(c.Request.Context(), c)
	items, result, err := h.aiService.AdminListAssets(ctx, subject.UserID, params, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) GetAsset(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	assetID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid asset ID")
		return
	}
	ctx := contextWithRequestBaseURL(c.Request.Context(), c)
	asset, err := h.aiService.AdminGetAsset(ctx, assetID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, asset)
}

func (h *AIHandler) ModerateAsset(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	assetID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid asset ID")
		return
	}
	var req aiAdminModerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeAdminIdempotentJSON(c, "ai:moderate-asset", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		state := strings.TrimSpace(req.ModerationState)
		visibility := ""
		if state != "" && state != service.AIModerationStateNormal {
			visibility = service.AIVisibilityPrivate
		}
		return h.aiService.AdminModerateAsset(ctx, subject.UserID, assetID, &service.AIUpdateAssetInput{
			Visibility:      nullableString(visibility),
			ModerationState: nullableString(state),
			Trace:           buildAITrace(req.Trace),
		})
	})
}

func (h *AIHandler) ListAuditLogs(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	filter := service.AIListAuditLogsFilter{
		EntityType:     strings.TrimSpace(c.Query("entity_type")),
		EntityID:       parseOptionalAIID(c.Query("entity_id")),
		Action:         strings.TrimSpace(c.Query("action")),
		RequestID:      strings.TrimSpace(c.Query("request_id")),
		OperatorUserID: parseOptionalAIID(c.Query("operator_user_id")),
	}
	items, result, err := h.aiService.AdminListAuditLogs(c.Request.Context(), params, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) ListPrompts(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	filter := service.AIListPromptTemplatesFilter{
		Scope:           strings.TrimSpace(c.DefaultQuery("scope", "all")),
		Visibility:      strings.TrimSpace(c.Query("visibility")),
		Status:          strings.TrimSpace(c.Query("status")),
		ModerationState: strings.TrimSpace(c.Query("moderation_state")),
		Search:          strings.TrimSpace(c.Query("search")),
		GroupID:         parseOptionalAIID(c.Query("line_id")),
	}
	items, result, err := h.aiService.AdminListPromptTemplates(c.Request.Context(), subject.UserID, adminAIPagination(c), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) UpdatePrompt(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	templateID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	var req legacyAdminUpdatePromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeAdminIdempotentJSON(c, "ai:prompts:update", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		item, err := h.aiService.AdminGetPromptTemplate(ctx, templateID)
		if err != nil {
			return nil, err
		}
		metadata := cloneAdminAIMap(item.Metadata)
		if req.Status != nil {
			if status := strings.TrimSpace(*req.Status); status != "" && status != "all" {
				metadata["status"] = status
			} else {
				delete(metadata, "status")
			}
		}
		return h.aiService.UpdatePromptTemplate(ctx, item.UserID, templateID, &service.AIUpdatePromptTemplateInput{
			Title:       req.Title,
			Description: req.Description,
			Tags:        req.Tags,
			Visibility:  req.Visibility,
			Content:     req.Content,
			Metadata:    &metadata,
			Trace: service.AIWriteTrace{
				GroupID: flattenAdminInt64Ptr(req.LineID),
			},
		})
	})
}

func (h *AIHandler) DeletePrompt(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	templateID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	executeAdminIdempotentJSON(c, "ai:prompts:delete", map[string]any{"id": templateID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		item, err := h.aiService.AdminGetPromptTemplate(ctx, templateID)
		if err != nil {
			return nil, err
		}
		if err := h.aiService.DeletePromptTemplate(ctx, item.UserID, templateID); err != nil {
			return nil, err
		}
		return gin.H{"message": "ok"}, nil
	})
}

func (h *AIHandler) ListArtworks(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	filter := service.AIListAssetsFilter{
		Status:           strings.TrimSpace(c.Query("status")),
		Visibility:       strings.TrimSpace(c.Query("visibility")),
		ModerationState:  strings.TrimSpace(c.Query("moderation_state")),
		GenerationJobID:  parseOptionalAIID(c.Query("generation_job_id")),
		SessionID:        parseOptionalAIID(c.Query("session_id")),
		PromptTemplateID: parseOptionalAIID(c.Query("prompt_template_id")),
		GroupID:          parseOptionalAIID(c.Query("line_id")),
		Featured:         parseOptionalBool(c.Query("featured")),
		Search:           strings.TrimSpace(c.Query("search")),
	}
	ctx := contextWithRequestBaseURL(c.Request.Context(), c)
	items, result, err := h.aiService.AdminListAssets(ctx, subject.UserID, adminAIPagination(c), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) UpdateArtwork(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	assetID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid asset ID")
		return
	}
	var req legacyAdminUpdateArtworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeAdminIdempotentJSON(c, "ai:artworks:update", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		item, err := h.aiService.AdminGetAsset(ctx, assetID)
		if err != nil {
			return nil, err
		}
		metadata := cloneAdminAIMap(item.Metadata)
		if req.Title != nil {
			metadata["title"] = strings.TrimSpace(*req.Title)
		}
		if req.Featured != nil {
			metadata["featured"] = *req.Featured
		}
		if req.Tags != nil {
			metadata["tags"] = *req.Tags
		}
		return h.aiService.UpdateAsset(ctx, subject.UserID, item.UserID, assetID, &service.AIUpdateAssetInput{
			Visibility: req.Visibility,
			Status:     req.Status,
			Metadata:   &metadata,
			Trace: service.AIWriteTrace{
				GroupID: item.Trace.GroupID,
			},
		})
	})
}

func (h *AIHandler) DeleteArtwork(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	assetID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid asset ID")
		return
	}
	executeAdminIdempotentJSON(c, "ai:artworks:delete", map[string]any{"id": assetID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		item, err := h.aiService.AdminGetAsset(ctx, assetID)
		if err != nil {
			return nil, err
		}
		status := service.AIAssetStatusDeleted
		if _, err := h.aiService.UpdateAsset(ctx, subject.UserID, item.UserID, assetID, &service.AIUpdateAssetInput{
			Status: &status,
			Trace: service.AIWriteTrace{
				GroupID: item.Trace.GroupID,
			},
		}); err != nil {
			return nil, err
		}
		return gin.H{"message": "ok"}, nil
	})
}

func requireAdminAIAuth(c *gin.Context) (servermiddleware.AuthSubject, bool) {
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Admin not authenticated")
		return servermiddleware.AuthSubject{}, false
	}
	return subject, true
}

func buildAITrace(req aiTraceRequest) service.AIWriteTrace {
	return service.AIWriteTrace{
		RequestID:  strings.TrimSpace(req.RequestID),
		UsageLogID: req.UsageLogID,
		APIKeyID:   req.APIKeyID,
		GroupID:    req.GroupID,
	}
}

func adminAIPagination(c *gin.Context) pagination.PaginationParams {
	page, pageSize := response.ParsePagination(c)
	return pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "updated_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
}

func parseOptionalBool(raw string) *bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "all" {
		return nil
	}
	value := trimmed == "true" || trimmed == "1"
	return &value
}

func flattenAdminInt64Ptr(value **int64) *int64 {
	if value == nil || *value == nil {
		return nil
	}
	out := **value
	return &out
}

func cloneAdminAIMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func parseAIID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func parseOptionalAIID(raw string) *int64 {
	id, ok := parseAIID(raw)
	if !ok {
		return nil
	}
	return &id
}

func nullableString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}
