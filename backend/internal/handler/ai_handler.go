package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/handler/skillkit"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AIHandler struct {
	aiService           *service.AICenterService
	openAIGateway       *OpenAIGatewayHandler
	apiKeyService       *service.APIKeyService
	subscriptionService *service.SubscriptionService
	mediaService        *service.MediaService
	skillModule         *skillkit.Module
}

func NewAIHandler(
	aiService *service.AICenterService,
	openAIGateway *OpenAIGatewayHandler,
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	mediaService *service.MediaService,
	skillModule *skillkit.Module,
) *AIHandler {
	return &AIHandler{
		aiService:           aiService,
		openAIGateway:       openAIGateway,
		apiKeyService:       apiKeyService,
		subscriptionService: subscriptionService,
		mediaService:        mediaService,
		skillModule:         skillModule,
	}
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

type aiCreateSessionRequest struct {
	Title        string         `json:"title"`
	SystemPrompt *string        `json:"system_prompt"`
	Metadata     map[string]any `json:"metadata"`
	Trace        aiTraceRequest `json:"trace"`
}

type aiUpdateSessionRequest struct {
	Title        *string         `json:"title"`
	Status       *string         `json:"status"`
	SystemPrompt **string        `json:"system_prompt"`
	Metadata     *map[string]any `json:"metadata"`
	Trace        aiTraceRequest  `json:"trace"`
}

type aiSendMessageRequest struct {
	Role                   string           `json:"role"`
	Content                string           `json:"content" binding:"required"`
	ContentParts           []map[string]any `json:"content_parts"`
	Model                  string           `json:"model"`
	Provider               string           `json:"provider"`
	Status                 string           `json:"status"`
	ReplyToMessageID       *int64           `json:"reply_to_message_id"`
	CreateAssistantReply   bool             `json:"create_assistant_reply"`
	AssistantReplyModel    string           `json:"assistant_reply_model"`
	AssistantReplyProvider string           `json:"assistant_reply_provider"`
	AssistantReplyStatus   string           `json:"assistant_reply_status"`
	AssistantReplyContent  string           `json:"assistant_reply_content"`
	AssistantReplyMetadata map[string]any   `json:"assistant_reply_metadata"`
	Trace                  aiTraceRequest   `json:"trace"`
}

type aiCreatePromptTemplateRequest struct {
	Title        string           `json:"title" binding:"required"`
	Description  *string          `json:"description"`
	Category     *string          `json:"category"`
	Tags         []string         `json:"tags"`
	Visibility   string           `json:"visibility"`
	ModelHint    *string          `json:"model_hint"`
	Content      string           `json:"content" binding:"required"`
	Variables    []map[string]any `json:"variables"`
	Metadata     map[string]any   `json:"metadata"`
	CoverAssetID *int64           `json:"cover_asset_id"`
	ChangeNote   *string          `json:"change_note"`
	Trace        aiTraceRequest   `json:"trace"`
}

type aiUpdatePromptTemplateRequest struct {
	Title        *string           `json:"title"`
	Description  **string          `json:"description"`
	Category     **string          `json:"category"`
	Tags         *[]string         `json:"tags"`
	Visibility   *string           `json:"visibility"`
	ModelHint    **string          `json:"model_hint"`
	Content      *string           `json:"content"`
	Variables    *[]map[string]any `json:"variables"`
	Metadata     *map[string]any   `json:"metadata"`
	CoverAssetID **int64           `json:"cover_asset_id"`
	ChangeNote   *string           `json:"change_note"`
	Trace        aiTraceRequest    `json:"trace"`
}

type aiCreateGenerationJobRequest struct {
	SessionID        *int64         `json:"session_id"`
	PromptTemplateID *int64         `json:"prompt_template_id"`
	Model            string         `json:"model" binding:"required"`
	Prompt           string         `json:"prompt" binding:"required"`
	NegativePrompt   *string        `json:"negative_prompt"`
	Size             *string        `json:"size"`
	ImageCount       *int           `json:"image_count"`
	Seed             *int64         `json:"seed"`
	Parameters       map[string]any `json:"parameters"`
	Trace            aiTraceRequest `json:"trace"`
}

type legacyCreatePromptRequest struct {
	Title       string   `json:"title" binding:"required"`
	Content     string   `json:"content" binding:"required"`
	Description *string  `json:"description"`
	Tags        []string `json:"tags"`
	Visibility  string   `json:"visibility"`
	Status      string   `json:"status"`
	LineID      *int64   `json:"line_id"`
}

type legacyUpdatePromptRequest struct {
	Title       *string   `json:"title"`
	Content     *string   `json:"content"`
	Description **string  `json:"description"`
	Tags        *[]string `json:"tags"`
	Visibility  *string   `json:"visibility"`
	Status      *string   `json:"status"`
	LineID      **int64   `json:"line_id"`
}

type legacyCreateArtworkRequest struct {
	Title            *string  `json:"title"`
	Prompt           string   `json:"prompt" binding:"required"`
	NegativePrompt   *string  `json:"negative_prompt"`
	Visibility       string   `json:"visibility"`
	Mode             string   `json:"mode"`
	LineID           *int64   `json:"line_id"`
	KeyID            *int64   `json:"key_id"`
	Size             *string  `json:"size"`
	Style            *string  `json:"style"`
	Tags             []string `json:"tags"`
	PromptTemplateID *int64   `json:"prompt_template_id"`
	SourceImage      *string  `json:"source_image"`
	MaskImage        *string  `json:"mask_image"`
	Featured         bool     `json:"featured"`
}

type legacyUpdateArtworkRequest struct {
	Title      *string   `json:"title"`
	Visibility *string   `json:"visibility"`
	Status     *string   `json:"status"`
	Featured   *bool     `json:"featured"`
	Tags       *[]string `json:"tags"`
}

type legacyChatRequest struct {
	SessionID        *int64           `json:"session_id"`
	Prompt           string           `json:"prompt" binding:"required"`
	LineID           *int64           `json:"line_id"`
	KeyID            *int64           `json:"key_id"`
	PromptTemplateID *int64           `json:"prompt_template_id"`
	History          []map[string]any `json:"history"`
	UseResponses     *bool            `json:"use_responses"`
}

func (h *AIHandler) ListSessions(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	items, result, err := h.aiService.ListSessions(c.Request.Context(), subject.UserID, userAIPagination(c), strings.TrimSpace(c.Query("status")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) CreateSession(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	var req aiCreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "ai:sessions:create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.aiService.CreateSession(ctx, subject.UserID, &service.AICreateSessionInput{
			Title:        req.Title,
			SystemPrompt: req.SystemPrompt,
			Metadata:     req.Metadata,
			Trace:        buildUserAITrace(req.Trace),
		})
	})
}

func (h *AIHandler) GetSession(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	sessionID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid session ID")
		return
	}
	item, err := h.aiService.GetSession(c.Request.Context(), subject.UserID, sessionID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *AIHandler) UpdateSession(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	sessionID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid session ID")
		return
	}
	var req aiUpdateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "ai:sessions:update", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.aiService.UpdateSession(ctx, subject.UserID, sessionID, &service.AIUpdateSessionInput{
			Title:        req.Title,
			Status:       req.Status,
			SystemPrompt: req.SystemPrompt,
			Metadata:     req.Metadata,
			Trace:        buildUserAITrace(req.Trace),
		})
	})
}

func (h *AIHandler) DeleteSession(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	sessionID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid session ID")
		return
	}
	executeUserIdempotentJSON(c, "ai:sessions:delete", map[string]any{"id": sessionID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if err := h.aiService.DeleteSession(ctx, subject.UserID, sessionID); err != nil {
			return nil, err
		}
		return gin.H{"deleted": true}, nil
	})
}

func (h *AIHandler) ListSessionMessages(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	sessionID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid session ID")
		return
	}
	items, result, err := h.aiService.ListSessionMessages(c.Request.Context(), subject.UserID, sessionID, userAIPagination(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) SendMessage(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	sessionID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid session ID")
		return
	}
	var req aiSendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "ai:sessions:messages:create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.aiService.SendMessage(ctx, subject.UserID, sessionID, &service.AISendMessageInput{
			Role:                   req.Role,
			Content:                req.Content,
			ContentParts:           req.ContentParts,
			Model:                  req.Model,
			Provider:               req.Provider,
			Status:                 req.Status,
			ReplyToMessageID:       req.ReplyToMessageID,
			CreateAssistantReply:   req.CreateAssistantReply,
			AssistantReplyModel:    req.AssistantReplyModel,
			AssistantReplyProvider: req.AssistantReplyProvider,
			AssistantReplyStatus:   req.AssistantReplyStatus,
			AssistantReplyContent:  req.AssistantReplyContent,
			AssistantReplyMetadata: req.AssistantReplyMetadata,
			Trace:                  buildUserAITrace(req.Trace),
		})
	})
}

func (h *AIHandler) ListPromptTemplates(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	filter := service.AIListPromptTemplatesFilter{
		Scope:           strings.TrimSpace(c.DefaultQuery("scope", "mine")),
		Visibility:      strings.TrimSpace(c.Query("visibility")),
		Status:          strings.TrimSpace(c.Query("status")),
		ModerationState: strings.TrimSpace(c.Query("moderation_state")),
		Search:          strings.TrimSpace(c.Query("search")),
		OwnerUserID:     parseOptionalUserAIID(c.Query("owner_user_id")),
		GroupID:         parseOptionalUserAIID(c.Query("group_id")),
	}
	items, result, err := h.aiService.ListPromptTemplates(c.Request.Context(), subject.UserID, false, userAIPagination(c), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) CreatePromptTemplate(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	var req aiCreatePromptTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "ai:prompt-templates:create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.aiService.CreatePromptTemplate(ctx, subject.UserID, &service.AICreatePromptTemplateInput{
			Title:        req.Title,
			Description:  req.Description,
			Category:     req.Category,
			Tags:         req.Tags,
			Visibility:   req.Visibility,
			ModelHint:    req.ModelHint,
			Content:      req.Content,
			Variables:    req.Variables,
			Metadata:     req.Metadata,
			CoverAssetID: req.CoverAssetID,
			ChangeNote:   req.ChangeNote,
			Trace:        buildUserAITrace(req.Trace),
		})
	})
}

func (h *AIHandler) GetPromptTemplate(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	templateID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	item, err := h.aiService.GetPromptTemplate(c.Request.Context(), subject.UserID, templateID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *AIHandler) ListPrompts(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	scope := "all"
	if raw := strings.TrimSpace(c.Query("mine_only")); raw == "true" || raw == "1" {
		scope = "mine"
	}
	filter := service.AIListPromptTemplatesFilter{
		Scope:           scope,
		Visibility:      strings.TrimSpace(c.Query("visibility")),
		Status:          strings.TrimSpace(c.Query("status")),
		ModerationState: strings.TrimSpace(c.Query("moderation_state")),
		Search:          strings.TrimSpace(c.Query("search")),
		GroupID:         parseOptionalUserAIID(c.Query("line_id")),
	}
	items, result, err := h.aiService.ListPromptTemplates(c.Request.Context(), subject.UserID, false, userAIPagination(c), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) CreatePrompt(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	var req legacyCreatePromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "ai:prompts:create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		metadata := map[string]any{}
		if status := strings.TrimSpace(req.Status); status != "" && status != "all" {
			metadata["status"] = status
		}
		return h.aiService.CreatePromptTemplate(ctx, subject.UserID, &service.AICreatePromptTemplateInput{
			Title:       req.Title,
			Description: req.Description,
			Tags:        req.Tags,
			Visibility:  req.Visibility,
			Content:     req.Content,
			Metadata:    metadata,
			Trace: service.AIWriteTrace{
				GroupID: req.LineID,
			},
		})
	})
}

func (h *AIHandler) UpdatePrompt(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	templateID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	var req legacyUpdatePromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "ai:prompts:update", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		existing, err := h.aiService.GetPromptTemplate(ctx, subject.UserID, templateID)
		if err != nil {
			return nil, err
		}
		metadata := cloneAIMap(existing.Metadata)
		if req.Status != nil {
			if status := strings.TrimSpace(*req.Status); status != "" && status != "all" {
				metadata["status"] = status
			} else {
				delete(metadata, "status")
			}
		}
		return h.aiService.UpdatePromptTemplate(ctx, subject.UserID, templateID, &service.AIUpdatePromptTemplateInput{
			Title:       req.Title,
			Description: req.Description,
			Tags:        req.Tags,
			Visibility:  req.Visibility,
			Content:     req.Content,
			Metadata:    &metadata,
			Trace: service.AIWriteTrace{
				GroupID: flattenInt64Ptr(req.LineID),
			},
		})
	})
}

func (h *AIHandler) DeletePrompt(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	templateID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	executeUserIdempotentJSON(c, "ai:prompts:delete", map[string]any{"id": templateID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if err := h.aiService.DeletePromptTemplate(ctx, subject.UserID, templateID); err != nil {
			return nil, err
		}
		return gin.H{"message": "ok"}, nil
	})
}

func (h *AIHandler) ClonePrompt(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	templateID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	executeUserIdempotentJSON(c, "ai:prompts:clone", map[string]any{"id": templateID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		item, err := h.aiService.GetPromptTemplate(ctx, subject.UserID, templateID)
		if err != nil {
			return nil, err
		}
		metadata := cloneAIMap(item.Metadata)
		metadata["cloned_from_id"] = item.ID
		metadata["status"] = "draft"
		return h.aiService.CreatePromptTemplate(ctx, subject.UserID, &service.AICreatePromptTemplateInput{
			Title:       item.Title + " Copy",
			Description: nullableString(item.Description),
			Category:    nullableString(item.Category),
			Tags:        append([]string(nil), item.Tags...),
			Visibility:  service.AIVisibilityPrivate,
			ModelHint:   nullableString(item.ModelHint),
			Content:     item.Content,
			Variables:   item.Variables,
			Metadata:    metadata,
			Trace: service.AIWriteTrace{
				GroupID: item.Trace.GroupID,
			},
		})
	})
}

func (h *AIHandler) UpdatePromptTemplate(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	templateID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	var req aiUpdatePromptTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "ai:prompt-templates:update", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.aiService.UpdatePromptTemplate(ctx, subject.UserID, templateID, &service.AIUpdatePromptTemplateInput{
			Title:        req.Title,
			Description:  req.Description,
			Category:     req.Category,
			Tags:         req.Tags,
			Visibility:   req.Visibility,
			ModelHint:    req.ModelHint,
			Content:      req.Content,
			Variables:    req.Variables,
			Metadata:     req.Metadata,
			CoverAssetID: req.CoverAssetID,
			ChangeNote:   req.ChangeNote,
			Trace:        buildUserAITrace(req.Trace),
		})
	})
}

func (h *AIHandler) DeletePromptTemplate(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	templateID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid template ID")
		return
	}
	executeUserIdempotentJSON(c, "ai:prompt-templates:delete", map[string]any{"id": templateID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if err := h.aiService.DeletePromptTemplate(ctx, subject.UserID, templateID); err != nil {
			return nil, err
		}
		return gin.H{"deleted": true}, nil
	})
}

func (h *AIHandler) ListGenerationJobs(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	filter := service.AIListGenerationJobsFilter{
		Status:           strings.TrimSpace(c.Query("status")),
		SessionID:        parseOptionalUserAIID(c.Query("session_id")),
		PromptTemplateID: parseOptionalUserAIID(c.Query("prompt_template_id")),
		GroupID:          parseOptionalUserAIID(c.Query("group_id")),
	}
	items, result, err := h.aiService.ListGenerationJobs(c.Request.Context(), subject.UserID, userAIPagination(c), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.PaginatedWithResult(c, items, toResponsePagination(result))
}

func (h *AIHandler) CreateGenerationJob(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	var req aiCreateGenerationJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "ai:generation-jobs:create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.aiService.CreateGenerationJob(ctx, subject.UserID, &service.AICreateGenerationJobInput{
			SessionID:        req.SessionID,
			PromptTemplateID: req.PromptTemplateID,
			Model:            req.Model,
			Prompt:           req.Prompt,
			NegativePrompt:   req.NegativePrompt,
			Size:             req.Size,
			ImageCount:       req.ImageCount,
			Seed:             req.Seed,
			Parameters:       req.Parameters,
			Trace:            buildUserAITrace(req.Trace),
		})
	})
}

func (h *AIHandler) GetGenerationJob(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	jobID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid job ID")
		return
	}
	item, err := h.aiService.GetGenerationJob(c.Request.Context(), subject.UserID, jobID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *AIHandler) CreateArtwork(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	var req legacyCreateArtworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	h.submitArtwork(c, subject.UserID, req, false)
}

func (h *AIHandler) EditArtwork(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	var req legacyCreateArtworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	h.submitArtwork(c, subject.UserID, req, true)
}

func (h *AIHandler) UpdateArtwork(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	assetID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid artwork ID")
		return
	}
	var req legacyUpdateArtworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeUserIdempotentJSON(c, "ai:artworks:update", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		item, err := h.aiService.GetAsset(ctx, subject.UserID, assetID)
		if err != nil {
			return nil, err
		}
		metadata := cloneAIMap(item.Metadata)
		if req.Title != nil {
			metadata["title"] = strings.TrimSpace(*req.Title)
		}
		if req.Featured != nil {
			metadata["featured"] = *req.Featured
		}
		if req.Tags != nil {
			metadata["tags"] = *req.Tags
		}
		updated, err := h.aiService.UpdateAsset(ctx, subject.UserID, subject.UserID, assetID, &service.AIUpdateAssetInput{
			Visibility: req.Visibility,
			Status:     req.Status,
			Metadata:   &metadata,
			Trace: service.AIWriteTrace{
				GroupID: item.Trace.GroupID,
			},
		})
		if err != nil {
			return nil, err
		}
		return dto.AIArtworkFromService(updated, subject.UserID), nil
	})
}

func (h *AIHandler) DeleteArtwork(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	assetID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid artwork ID")
		return
	}
	executeUserIdempotentJSON(c, "ai:artworks:delete", map[string]any{"id": assetID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		item, err := h.aiService.GetAsset(ctx, subject.UserID, assetID)
		if err != nil {
			return nil, err
		}
		status := service.AIAssetStatusDeleted
		if _, err := h.aiService.UpdateAsset(ctx, subject.UserID, subject.UserID, assetID, &service.AIUpdateAssetInput{
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

func (h *AIHandler) Chat(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	var req legacyChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		response.BadRequest(c, "Prompt is required")
		return
	}
	executeUserIdempotentJSON(c, "ai:chat", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.chatViaGateway(ctx, subject.UserID, &req)
	})
}

func (h *AIHandler) GetRuntime(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	var runtimeInfo *service.AIRuntimeInfo
	if h.aiService != nil {
		runtimeInfo, _ = h.aiService.GetRuntime(c.Request.Context(), subject.UserID, h.apiKeyService)
	}
	info := service.MediaRuntimeInfo{}
	if h.mediaService != nil {
		info = h.mediaService.RuntimeInfo()
	}
	response.Success(c, dto.AIRuntimeFromServiceRuntime(runtimeInfo, info))
}

func (h *AIHandler) ListGallery(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	filter := service.AIListAssetsFilter{
		Status:           strings.TrimSpace(c.Query("status")),
		Visibility:       strings.TrimSpace(c.Query("visibility")),
		ModerationState:  strings.TrimSpace(c.Query("moderation_state")),
		GenerationJobID:  parseOptionalUserAIID(c.Query("generation_job_id")),
		SessionID:        parseOptionalUserAIID(c.Query("session_id")),
		PromptTemplateID: parseOptionalUserAIID(c.Query("prompt_template_id")),
		GroupID:          parseOptionalUserAIID(c.Query("group_id")),
		Search:           strings.TrimSpace(c.Query("search")),
	}
	items, result, err := h.aiService.ListGallery(c.Request.Context(), subject.UserID, userAIPagination(c), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.AIArtwork, 0, len(items))
	for i := range items {
		if artwork := dto.AIArtworkFromService(&items[i], subject.UserID); artwork != nil {
			out = append(out, *artwork)
		}
	}
	response.PaginatedWithResult(c, out, toResponsePagination(result))
}

func (h *AIHandler) GetAsset(c *gin.Context) {
	subject, ok := requireUserAIAuth(c)
	if !ok {
		return
	}
	assetID, ok := parseUserAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid asset ID")
		return
	}
	item, err := h.aiService.GetAsset(c.Request.Context(), subject.UserID, assetID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AIAssetPublicFromService(item))
}

func requireUserAIAuth(c *gin.Context) (servermiddleware.AuthSubject, bool) {
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return servermiddleware.AuthSubject{}, false
	}
	return subject, true
}

func userAIPagination(c *gin.Context) pagination.PaginationParams {
	page, pageSize := response.ParsePagination(c)
	return pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "updated_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
}

func buildUserAITrace(req aiTraceRequest) service.AIWriteTrace {
	return service.AIWriteTrace{
		RequestID:  strings.TrimSpace(req.RequestID),
		UsageLogID: req.UsageLogID,
		APIKeyID:   req.APIKeyID,
		GroupID:    req.GroupID,
	}
}

func parseUserAIID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func parseOptionalUserAIID(raw string) *int64 {
	id, ok := parseUserAIID(raw)
	if !ok {
		return nil
	}
	return &id
}

func flattenInt64Ptr(value **int64) *int64 {
	if value == nil || *value == nil {
		return nil
	}
	out := **value
	return &out
}

func nullableString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func cloneAIMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func defaultAIVisibility(raw string) string {
	switch strings.TrimSpace(raw) {
	case service.AIVisibilityPublic:
		return service.AIVisibilityPublic
	default:
		return service.AIVisibilityPrivate
	}
}

func toResponsePagination(p *pagination.PaginationResult) *response.PaginationResult {
	if p == nil {
		return nil
	}
	return &response.PaginationResult{
		Total:    p.Total,
		Page:     p.Page,
		PageSize: p.PageSize,
		Pages:    p.Pages,
	}
}

func (h *AIHandler) submitArtwork(c *gin.Context, userID int64, req legacyCreateArtworkRequest, forceEdit bool) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	edit := forceEdit || mode == "edit"
	if edit {
		req.Mode = "edit"
	} else {
		req.Mode = "generate"
	}
	action := "create"
	if edit {
		action = "edit"
	}
	executeUserIdempotentJSON(c, "ai:artworks:"+action, req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if edit {
			return h.editArtworkViaGateway(ctx, userID, &req)
		}
		return h.createArtworkViaGateway(ctx, userID, &req)
	})
}
