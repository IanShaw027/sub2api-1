package handler

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CreationHandler struct {
	creationService     *service.CreationService
	keyResolver         *service.CreationKeyResolver
	subscriptionService *service.SubscriptionService
	gateway             *GatewayHandler
	openAI              *OpenAIGatewayHandler
	asyncImage            *AsyncImageHandler
}

func NewCreationHandler(
	creationService *service.CreationService,
	keyResolver *service.CreationKeyResolver,
	subscriptionService *service.SubscriptionService,
	gateway *GatewayHandler,
	openAI *OpenAIGatewayHandler,
	asyncImage *AsyncImageHandler,
) *CreationHandler {
	return &CreationHandler{
		creationService:     creationService,
		keyResolver:         keyResolver,
		subscriptionService: subscriptionService,
		gateway:             gateway,
		openAI:              openAI,
		asyncImage:          asyncImage,
	}
}

type createCreationSessionRequest struct {
	GroupID  int64           `json:"group_id"`
	Title    string          `json:"title"`
	Model    string          `json:"model"`
	Mode     string          `json:"mode"`
	Metadata json.RawMessage `json:"metadata"`
}

type updateCreationSessionRequest struct {
	Title    *string         `json:"title"`
	Model    *string         `json:"model"`
	Status   *string         `json:"status"`
	Metadata json.RawMessage `json:"metadata"`
}

type createCreationMessageRequest struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Model   *string         `json:"model"`
}

func (h *CreationHandler) ChatCompletions(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		h.openAI.ChatCompletions(c)
	})
}

func (h *CreationHandler) Messages(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		apiKey, ok := middleware2.GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil {
			response.Unauthorized(c, "API key context missing")
			return
		}
		switch apiKey.Group.Platform {
		case service.PlatformOpenAI, service.PlatformGrok,
			service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek:
			h.openAI.Messages(c)
		default:
			h.gateway.Messages(c)
		}
	})
}

func (h *CreationHandler) Images(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		h.openAI.Images(c)
	})
}

func (h *CreationHandler) ImagesAsync(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		h.asyncImage.Submit(c)
	})
}

func (h *CreationHandler) ImageTask(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		h.asyncImage.Get(c)
	})
}

func (h *CreationHandler) Models(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		h.gateway.Models(c)
	})
}

func (h *CreationHandler) requireCreationService(c *gin.Context) bool {
	if h == nil || h.creationService == nil {
		response.InternalError(c, "creation service unavailable")
		return false
	}
	return true
}

func (h *CreationHandler) ListSessions(c *gin.Context) {
	if !h.requireCreationService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.creationService.ListSessions(c.Request.Context(), subject.UserID, service.CreationSessionListFilters{
		Mode:     strings.TrimSpace(c.Query("mode")),
		Status:   strings.TrimSpace(c.Query("status")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *CreationHandler) CreateSession(c *gin.Context) {
	if !h.requireCreationService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createCreationSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	view, err := h.creationService.CreateSession(c.Request.Context(), service.CreateCreationSessionInput{
		UserID:   subject.UserID,
		GroupID:  req.GroupID,
		Title:    req.Title,
		Model:    req.Model,
		Mode:     req.Mode,
		Metadata: req.Metadata,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *CreationHandler) GetSession(c *gin.Context) {
	if !h.requireCreationService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseCreationSessionID(c)
	if !ok {
		return
	}
	view, err := h.creationService.GetSession(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *CreationHandler) UpdateSession(c *gin.Context) {
	if !h.requireCreationService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseCreationSessionID(c)
	if !ok {
		return
	}
	var req updateCreationSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	view, err := h.creationService.UpdateSession(c.Request.Context(), subject.UserID, id, service.UpdateCreationSessionInput{
		Title:    req.Title,
		Model:    req.Model,
		Status:   req.Status,
		Metadata: req.Metadata,
		HasMeta:  req.Metadata != nil,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *CreationHandler) DeleteSession(c *gin.Context) {
	if !h.requireCreationService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseCreationSessionID(c)
	if !ok {
		return
	}
	if err := h.creationService.DeleteSession(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *CreationHandler) ListSessionMessages(c *gin.Context) {
	if !h.requireCreationService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseCreationSessionID(c)
	if !ok {
		return
	}
	items, err := h.creationService.ListMessages(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *CreationHandler) CreateSessionMessage(c *gin.Context) {
	if !h.requireCreationService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseCreationSessionID(c)
	if !ok {
		return
	}
	var req createCreationMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	view, err := h.creationService.CreateMessage(c.Request.Context(), service.CreateCreationMessageInput{
		UserID:    subject.UserID,
		SessionID: id,
		Role:      req.Role,
		Content:   req.Content,
		Model:     req.Model,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *CreationHandler) ListImages(c *gin.Context) {
	if !h.requireCreationService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	var sessionID *int64
	if raw := strings.TrimSpace(c.Query("session_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "invalid session_id")
			return
		}
		sessionID = &parsed
	}
	items, total, err := h.creationService.ListImages(c.Request.Context(), subject.UserID, service.CreationImageListFilters{
		SessionID: sessionID,
		Page:      page,
		PageSize:  pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *CreationHandler) GetImage(c *gin.Context) {
	if !h.requireCreationService(c) {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid image id")
		return
	}
	view, err := h.creationService.GetImage(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, view)
}

func (h *CreationHandler) withGatewayContext(c *gin.Context, next func(*gin.Context)) {
	if h == nil || h.keyResolver == nil {
		response.InternalError(c, "creation key resolver unavailable")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	groupID, ok := parseCreationGroupID(c)
	if !ok {
		return
	}
	apiKey, err := h.keyResolver.ResolveAuthKey(c.Request.Context(), subject.UserID, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var subscription *service.UserSubscription
	if apiKey.Group != nil && apiKey.Group.IsSubscriptionType() && h.subscriptionService != nil {
		subscription, err = h.subscriptionService.GetActiveSubscription(c.Request.Context(), subject.UserID, groupID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}
	middleware2.InjectGatewayContextFromAPIKey(c, apiKey, subscription)
	next(c)
}

func parseCreationGroupID(c *gin.Context) (int64, bool) {
	raw := strings.TrimSpace(c.Query("group_id"))
	if raw == "" {
		raw = strings.TrimSpace(c.GetHeader("X-Group-Id"))
	}
	if raw == "" {
		response.ErrorFrom(c, service.ErrCreationGroupRequired)
		return 0, false
	}
	groupID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "invalid group_id")
		return 0, false
	}
	return groupID, true
}

func parseCreationSessionID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid session id")
		return 0, false
	}
	return id, true
}
