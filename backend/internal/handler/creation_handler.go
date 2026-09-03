package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CreationHandler struct {
	creationService     *service.CreationService
	keyResolver         *service.CreationKeyResolver
	subscriptionService *service.SubscriptionService
	gateway             *GatewayHandler
	openAI              *OpenAIGatewayHandler
	asyncImage          *AsyncImageHandler
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
		if h.asyncImage == nil {
			imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "async image handler is unavailable")
			return
		}
		model, prompt := peekCreationImageModelPrompt(c)
		h.asyncImage.Submit(c)
		h.persistCreationImageJob(c, model, prompt)
	})
}

func (h *CreationHandler) ImageTask(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		ensureImageTaskIDParam(c)
		if h.asyncImage == nil {
			imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "async image handler is unavailable")
			return
		}
		task, ok := h.asyncImage.loadTaskForGet(c)
		if !ok {
			return
		}
		h.syncCreationImageJob(c, task)
		writeImageTaskJSON(c, task)
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
	h.attachCreationImageMediaURLs(c, subject.UserID, items)
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
	h.attachCreationImageMediaURL(c, subject.UserID, view)
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

func (h *CreationHandler) persistCreationImageJob(c *gin.Context, model, prompt string) {
	if h == nil || h.creationService == nil || c == nil {
		return
	}
	if c.Writer == nil || c.Writer.Status() != http.StatusAccepted {
		return
	}
	task := acceptedAsyncImageTask(c)
	if task == nil {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		return
	}
	groupID := creationGroupIDFromContext(c)
	if groupID <= 0 {
		return
	}
	taskID := strings.TrimSpace(task.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(task.ID)
	}
	if taskID == "" {
		return
	}
	status := strings.TrimSpace(task.Status)
	if status == "" {
		status = service.CreationImageJobStatusProcessing
	}
	if _, err := h.creationService.CreateImageJob(c.Request.Context(), service.CreateCreationImageJobInput{
		UserID:         subject.UserID,
		GroupID:        groupID,
		SessionID:      parseCreationSessionHeader(c),
		Model:          model,
		Prompt:         prompt,
		ProviderTaskID: taskID,
		Status:         status,
	}); err != nil {
		logger.L().Error("creation.image_job.persist_failed",
			zap.Error(err),
			zap.Int64("user_id", subject.UserID),
			zap.String("task_id", taskID))
	}
}

func (h *CreationHandler) syncCreationImageJob(c *gin.Context, task *service.ImageTask) {
	if h == nil || h.creationService == nil || c == nil || task == nil {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		return
	}
	if err := h.creationService.SyncImageJobFromTask(c.Request.Context(), subject.UserID, task); err != nil {
		logger.L().Error("creation.image_job.sync_failed",
			zap.Error(err),
			zap.Int64("user_id", subject.UserID))
	}
}

func (h *CreationHandler) attachCreationImageMediaURLs(c *gin.Context, userID int64, items []service.CreationImageJob) {
	for i := range items {
		h.attachCreationImageMediaURL(c, userID, &items[i])
	}
}

func (h *CreationHandler) attachCreationImageMediaURL(c *gin.Context, userID int64, job *service.CreationImageJob) {
	if job == nil || strings.TrimSpace(job.MediaURL) != "" || h == nil || h.asyncImage == nil {
		return
	}
	if job.ProviderTaskID == nil || strings.TrimSpace(*job.ProviderTaskID) == "" {
		return
	}
	ctx := c.Request.Context()
	if url := h.asyncImage.imageURLForUser(ctx, userID, *job.ProviderTaskID); url != "" {
		job.MediaURL = url
	}
}

func ensureImageTaskIDParam(c *gin.Context) {
	if c == nil {
		return
	}
	if strings.TrimSpace(c.Param("task_id")) != "" {
		return
	}
	if id := strings.TrimSpace(c.Param("id")); id != "" {
		c.Params = append(c.Params, gin.Param{Key: "task_id", Value: id})
	}
}

func peekCreationImageModelPrompt(c *gin.Context) (string, string) {
	if c == nil || c.Request == nil {
		return "", ""
	}
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		return "", ""
	}
	restoreCreationRequestBody(c, body)
	return parseCreationImageModelPrompt(c.GetHeader("Content-Type"), body)
}

func restoreCreationRequestBody(c *gin.Context, body []byte) {
	if c == nil || c.Request == nil {
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

func parseCreationImageModelPrompt(contentType string, body []byte) (string, string) {
	var envelope struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	_ = json.Unmarshal(body, &envelope)
	model := strings.TrimSpace(envelope.Model)
	prompt := strings.TrimSpace(envelope.Prompt)
	if model != "" && prompt != "" {
		return model, prompt
	}
	grok := service.ParseGrokMediaRequest(contentType, body)
	if model == "" {
		model = grok.Model
	}
	if prompt == "" {
		prompt = grok.Prompt
	}
	return model, prompt
}

func parseCreationSessionHeader(c *gin.Context) *int64 {
	if c == nil {
		return nil
	}
	raw := strings.TrimSpace(c.GetHeader("X-Session-Id"))
	if raw == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return nil
	}
	return &parsed
}

func creationGroupIDFromContext(c *gin.Context) int64 {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil {
		return 0
	}
	if apiKey.GroupID != nil && *apiKey.GroupID > 0 {
		return *apiKey.GroupID
	}
	if apiKey.Group != nil && apiKey.Group.ID > 0 {
		return apiKey.Group.ID
	}
	return 0
}
