package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
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
	voiceTickets        *service.CreationVoiceTicketService
	localVideoObserver  *service.CreationVideoObserver
	cfg                 *config.Config
}

func NewCreationHandler(
	creationService *service.CreationService,
	keyResolver *service.CreationKeyResolver,
	subscriptionService *service.SubscriptionService,
	gateway *GatewayHandler,
	openAI *OpenAIGatewayHandler,
	asyncImage *AsyncImageHandler,
	cfg *config.Config,
) *CreationHandler {
	return &CreationHandler{
		creationService:     creationService,
		keyResolver:         keyResolver,
		subscriptionService: subscriptionService,
		gateway:             gateway,
		openAI:              openAI,
		asyncImage:          asyncImage,
		cfg:                 cfg,
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

type createCreationExchangeRequest struct {
	RequestID        string `json:"request_id"`
	UserContent      string `json:"user_content"`
	AssistantContent string `json:"assistant_content"`
	Model            string `json:"model"`
	InputTokens      *int   `json:"input_tokens"`
	OutputTokens     *int   `json:"output_tokens"`
}

func (h *CreationHandler) ChatCompletions(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		apiKey, _ := middleware2.GetAPIKeyFromContext(c)
		if creationUsesOpenAIGateway(effectiveAPIKeyPlatform(c, apiKey)) {
			h.openAI.ChatCompletions(c)
			return
		}
		h.gateway.ChatCompletions(c)
	})
}

func (h *CreationHandler) Messages(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		apiKey, ok := middleware2.GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil {
			response.Unauthorized(c, "API key context missing")
			return
		}
		if creationUsesOpenAIGateway(effectiveAPIKeyPlatform(c, apiKey)) {
			h.openAI.Messages(c)
		} else {
			h.gateway.Messages(c)
		}
	})
}

func (h *CreationHandler) Images(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		apiKey, _ := middleware2.GetAPIKeyFromContext(c)
		switch effectiveAPIKeyPlatform(c, apiKey) {
		case service.PlatformOpenAI:
			h.openAI.Images(c)
		case service.PlatformGrok:
			h.openAI.GrokImages(c)
		default:
			imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "Images API is not supported for this platform")
		}
	})
}

func creationUsesOpenAIGateway(platform string) bool {
	switch platform {
	case service.PlatformOpenAI, service.PlatformGrok,
		service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek:
		return true
	default:
		return false
	}
}

func (h *CreationHandler) ImagesAsync(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		if h.asyncImage == nil {
			imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "async image handler is unavailable")
			return
		}
		model, prompt := peekCreationImageModelPrompt(c)
		model = clientRequestedModel(c, model)
		subject, _ := middleware2.GetAuthSubjectFromContext(c)
		input := service.CreateCreationImageJobInput{
			UserID: subject.UserID, GroupID: creationGroupIDFromContext(c),
			SessionID: parseCreationSessionHeader(c), Model: model, Prompt: prompt,
			Status: service.CreationImageJobStatusProcessing,
		}
		h.asyncImage.SubmitWithLifecycle(c, func(task *service.ImageTask) error {
			input.ProviderTaskID = task.TaskID
			_, err := h.creationService.CreateImageJob(c.Request.Context(), input)
			return err
		}, func(ctx context.Context, task *service.ImageTask) error {
			return h.persistCreationImageResult(ctx, input.UserID, task)
		})
	})
}

func (h *CreationHandler) ImageTask(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		ensureImageTaskIDParam(c)
		if h.asyncImage == nil || h.asyncImage.tasks == nil {
			imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "async image handler is unavailable")
			return
		}
		subject, _ := middleware2.GetAuthSubjectFromContext(c)
		taskID := strings.TrimSpace(c.Param("task_id"))
		job, jobErr := h.creationService.GetImageByTaskID(c.Request.Context(), subject.UserID, taskID)
		if jobErr != nil && !errors.Is(jobErr, service.ErrCreationImageNotFound) {
			response.ErrorFrom(c, jobErr)
			return
		}
		if job != nil && job.GroupID != creationGroupIDFromContext(c) {
			imageTaskError(c, service.ErrImageTaskNotFound)
			return
		}
		if job != nil && (job.Status == service.CreationImageJobStatusCompleted || job.Status == service.CreationImageJobStatusFailed) {
			h.attachCreationImageMediaURL(c, subject.UserID, job)
			writeImageTaskJSON(c, creationImageJobTask(job, taskID))
			return
		}
		apiKey, _ := middleware2.GetAPIKeyFromContext(c)
		task, err := h.asyncImage.tasks.Get(c.Request.Context(), service.ImageTaskOwner{UserID: subject.UserID, APIKeyID: apiKey.ID}, taskID)
		if err != nil {
			if job == nil || !errors.Is(err, service.ErrImageTaskNotFound) {
				imageTaskError(c, err)
				return
			}
			task = creationImageJobTask(job, taskID)
			if !time.Now().Before(h.asyncImage.tasks.ProcessingDeadline(job.CreatedAt)) {
				task.Status = service.ImageTaskStatusFailed
				task.Error = imageTaskErrorPayload("task_expired", "image task expired before its result was saved")
			}
		}
		h.syncCreationImageJob(c, task)
		if fresh, err := h.creationService.GetImageByTaskID(c.Request.Context(), subject.UserID, taskID); err == nil &&
			(fresh.Status == service.CreationImageJobStatusCompleted || fresh.Status == service.CreationImageJobStatusFailed) {
			h.attachCreationImageMediaURL(c, subject.UserID, fresh)
			task = creationImageJobTask(fresh, taskID)
		}
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

func (h *CreationHandler) CreateSessionExchange(c *gin.Context) {
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
	var req createCreationExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	exchange, err := h.creationService.CreateExchange(c.Request.Context(), service.CreateCreationExchangeInput{
		UserID: subject.UserID, SessionID: id, RequestID: req.RequestID,
		UserContent: req.UserContent, AssistantContent: req.AssistantContent, Model: req.Model,
		InputTokens: req.InputTokens, OutputTokens: req.OutputTokens,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, exchange)
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

const creationGatewayPreparedKey = "creation.gateway_prepared"

// GatewayContext prepares authentication before the shared gateway routing middleware.
func (h *CreationHandler) GatewayContext(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) { c.Next() })
	if !c.GetBool(creationGatewayPreparedKey) {
		c.Abort()
	}
}

func (h *CreationHandler) withGatewayContext(c *gin.Context, next func(*gin.Context)) {
	if c.GetBool(creationGatewayPreparedKey) {
		next(c)
		return
	}
	if h == nil || h.keyResolver == nil || h.creationService == nil {
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
	readLocalVideo := c.Request.Method == http.MethodGet && strings.Contains(c.Request.URL.Path, "/creation/local/videos/")
	readTask := c.Request.Method == http.MethodGet && (strings.Contains(c.Request.URL.Path, "/images/tasks/") || readLocalVideo)
	checkGroup := h.creationService.EnsureUserCanUseGroup
	if readTask {
		checkGroup = h.creationService.EnsureUserCanReadGroup
	}
	if err := checkGroup(c.Request.Context(), subject.UserID, groupID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	apiKey, err := h.keyResolver.ResolveAuthKey(c.Request.Context(), subject.UserID, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var subscription *service.UserSubscription
	if (!readTask || readLocalVideo) && apiKey.Group != nil && apiKey.Group.IsSubscriptionType() && h.subscriptionService != nil {
		if readLocalVideo {
			subscription, err = h.creationService.SubscriptionForAcceptedTask(c.Request.Context(), subject.UserID, groupID)
		} else {
			subscription, err = h.subscriptionService.GetActiveSubscription(c.Request.Context(), subject.UserID, groupID)
		}
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if c.Request.Method == http.MethodPost && (h.cfg == nil || h.cfg.RunMode != config.RunModeSimple) {
			needsMaintenance, validateErr := h.subscriptionService.ValidateAndCheckLimits(subscription, apiKey.Group)
			if needsMaintenance {
				subscription, err = h.subscriptionService.EnsureWindowMaintenance(c.Request.Context(), subscription)
				if err != nil {
					response.ErrorFrom(c, err)
					return
				}
				_, validateErr = h.subscriptionService.ValidateAndCheckLimits(subscription, apiKey.Group)
			}
			if validateErr != nil {
				response.ErrorFrom(c, validateErr)
				return
			}
		}
	}
	middleware2.InjectGatewayContextFromAPIKey(c, apiKey, subscription)
	c.Set(creationGatewayPreparedKey, true)
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

func (h *CreationHandler) persistCreationImageResult(ctx context.Context, userID int64, task *service.ImageTask) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = h.creationService.SyncImageJobFromTask(ctx, userID, task); err == nil {
			return nil
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return errors.Join(err, ctx.Err())
			case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
			}
		}
	}
	return err
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
	if job == nil || h == nil || h.asyncImage == nil || h.asyncImage.tasks == nil {
		return
	}
	ctx := c.Request.Context()
	if job.ProviderTaskID != nil && (job.StorageKey == "" || job.Status == service.CreationImageJobStatusProcessing || job.Status == service.CreationImageJobStatusPending) {
		if task, err := h.asyncImage.tasks.GetByIDForUser(ctx, userID, *job.ProviderTaskID); err == nil {
			h.syncCreationImageJob(c, task)
			if fresh, err := h.creationService.GetImage(ctx, userID, job.ID); err == nil {
				*job = *fresh
			}
		} else if errors.Is(err, service.ErrImageTaskNotFound) &&
			(job.Status == service.CreationImageJobStatusProcessing || job.Status == service.CreationImageJobStatusPending) &&
			!time.Now().Before(h.asyncImage.tasks.ProcessingDeadline(job.CreatedAt)) {
			task := creationImageJobTask(job, *job.ProviderTaskID)
			task.Status = service.ImageTaskStatusFailed
			task.Error = imageTaskErrorPayload("task_expired", "image task expired before its result was saved")
			h.syncCreationImageJob(c, task)
			if fresh, err := h.creationService.GetImage(ctx, userID, job.ID); err == nil {
				*job = *fresh
			}
		}
	}
	if job.StorageKey != "" {
		// Never reuse a stale signed URL or resolve a key against a different bucket.
		job.MediaURL = ""
		if url, err := h.asyncImage.tasks.ResolveStorageURL(ctx, service.ImageStorageReference{StorageID: job.StorageID, Key: job.StorageKey}); err == nil {
			job.MediaURL = url
		}
	} else if job.MediaAssetID != nil && *job.MediaAssetID > 0 {
		job.MediaURL = service.MediaPublicPath(*job.MediaAssetID)
	}
}

func creationImageJobTask(job *service.CreationImageJob, taskID string) *service.ImageTask {
	task := &service.ImageTask{
		ID: taskID, TaskID: taskID, Object: "image.generation.task",
		Status: job.Status, ImageURL: job.MediaURL, CreatedAt: job.CreatedAt.Unix(),
	}
	if job.Status == service.CreationImageJobStatusCompleted || job.Status == service.CreationImageJobStatusFailed {
		completedAt := job.UpdatedAt.Unix()
		task.CompletedAt = &completedAt
	}
	if job.MediaURL != "" {
		task.Result, _ = json.Marshal(gin.H{"data": []gin.H{{"url": job.MediaURL}}})
	}
	if job.Error != nil && *job.Error != "" {
		task.Error = imageTaskErrorPayload("api_error", *job.Error)
	}
	return task
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
