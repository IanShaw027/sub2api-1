package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	_ "golang.org/x/image/webp"
)

const (
	aiCreativeCenterDefaultChatModel  = "gpt-5.4-mini"
	aiCreativeCenterDefaultImageModel = "gpt-image-2"
	aiCreativeCenterKeyScanLimit      = 10000
)

type aiGatewayExecutionResult struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

func (h *AIHandler) chatViaGateway(ctx context.Context, userID int64, req *legacyChatRequest) (any, error) {
	apiKey, err := h.resolveAIExecutionKey(ctx, userID, req.LineID, req.KeyID)
	if err != nil {
		return nil, err
	}
	subscription, err := h.resolveAIExecutionSubscription(ctx, userID, apiKey.Group)
	if err != nil {
		return nil, err
	}
	sessionID := req.SessionID
	if sessionID == nil || *sessionID <= 0 {
		title := strings.TrimSpace(req.Prompt)
		if len(title) > 48 {
			title = title[:48]
		}
		traceRequestID := service.ResolveUsageRequestID(ctx, "")
		session, err := h.aiService.CreateSession(ctx, userID, &service.AICreateSessionInput{
			Title: title,
			Trace: service.AIWriteTrace{
				RequestID: traceRequestID,
				GroupID:   apiKey.GroupID,
				APIKeyID:  &apiKey.ID,
			},
		})
		if err != nil {
			return nil, err
		}
		sessionID = &session.ID
	}
	useResponses := shouldUseAIResponsesForChat(req)
	var (
		body []byte
		path string
		run  func(*gin.Context)
	)
	if useResponses {
		body = buildAIResponsesChatPayload(req, apiKey)
		path = "/openai/v1/responses"
		run = func(child *gin.Context) { h.openAIGateway.Responses(child) }
	} else {
		body = buildAIChatCompletionsPayload(req, apiKey)
		path = "/openai/v1/chat/completions"
		run = func(child *gin.Context) { h.openAIGateway.ChatCompletions(child) }
	}
	result, err := h.invokeOpenAIGateway(ctx, path, body, apiKey, subscription, run)
	if err != nil {
		return nil, err
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		return nil, openAIErrorFromGatewayResult(result)
	}
	replyContent := extractAIResponseText(result.Body)
	if !useResponses {
		replyContent = strings.TrimSpace(gjson.GetBytes(result.Body, "choices.0.message.content").String())
	}
	if replyContent == "" {
		replyContent = strings.TrimSpace(gjson.GetBytes(result.Body, "output_text").String())
	}
	if replyContent == "" {
		replyContent = strings.TrimSpace(gjson.GetBytes(result.Body, "response.output_text").String())
	}
	if replyContent == "" {
		replyContent = extractAIResponseText(result.Body)
	}
	if replyContent == "" {
		return nil, infraerrors.ServiceUnavailable("AI_GATEWAY_EMPTY_CHAT_RESPONSE", "empty response from gateway")
	}
	replyModel := strings.TrimSpace(gjson.GetBytes(result.Body, "model").String())
	if replyModel == "" {
		replyModel = strings.TrimSpace(gjson.GetBytes(result.Body, "response.model").String())
	}
	if replyModel == "" {
		replyModel = aiCreativeCenterDefaultChatModel
	}
	responseID := strings.TrimSpace(gjson.GetBytes(result.Body, "id").String())
	if responseID == "" {
		responseID = strings.TrimSpace(gjson.GetBytes(result.Body, "response.id").String())
	}
	traceRequestID := service.ResolveUsageRequestID(ctx, responseID)
	replyMetadata := map[string]any{
		"gateway_status": result.StatusCode,
		"gateway_id":     responseID,
		"request_id":     traceRequestID,
		"gateway_mode": func() string {
			if useResponses {
				return "responses"
			}
			return "chat_completions"
		}(),
	}
	if len(result.Body) > 0 {
		replyMetadata["gateway_response"] = json.RawMessage(result.Body)
	}
	reply, err := h.aiService.SendMessage(ctx, userID, *sessionID, &service.AISendMessageInput{
		Role:                   service.AIMessageRoleUser,
		Content:                strings.TrimSpace(req.Prompt),
		ContentParts:           nil,
		Model:                  replyModel,
		Provider:               "openai",
		Status:                 service.AIMessageStatusCompleted,
		CreateAssistantReply:   true,
		AssistantReplyModel:    replyModel,
		AssistantReplyProvider: "openai",
		AssistantReplyStatus:   service.AIMessageStatusCompleted,
		AssistantReplyContent:  replyContent,
		AssistantReplyMetadata: replyMetadata,
		Trace: service.AIWriteTrace{
			RequestID: traceRequestID,
			GroupID:   apiKey.GroupID,
			APIKeyID:  &apiKey.ID,
		},
	})
	if err != nil {
		return nil, err
	}
	if reply.AssistantMsg != nil {
		return gin.H{"message": reply.AssistantMsg}, nil
	}
	return gin.H{"message": reply.UserMessage}, nil
}

func (h *AIHandler) createArtworkViaGateway(ctx context.Context, userID int64, req *legacyCreateArtworkRequest) (any, error) {
	apiKey, err := h.resolveAIExecutionKey(ctx, userID, req.LineID, req.KeyID)
	if err != nil {
		return nil, err
	}
	subscription, err := h.resolveAIExecutionSubscription(ctx, userID, apiKey.Group)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(stringValue(req.Title))
	if title == "" {
		title = strings.TrimSpace(req.Prompt)
		if len(title) > 24 {
			title = title[:24]
		}
	}
	if title == "" {
		title = "AI Artwork"
	}
	model := resolveAIArtworkModel(apiKey)
	traceRequestID := service.ResolveUsageRequestID(ctx, "")
	job, err := h.aiService.CreateGenerationJob(ctx, userID, &service.AICreateGenerationJobInput{
		PromptTemplateID: req.PromptTemplateID,
		Model:            model,
		Prompt:           req.Prompt,
		NegativePrompt:   req.NegativePrompt,
		Size:             req.Size,
		ImageCount:       intPtr(1),
		Parameters: map[string]any{
			"title": title,
			"style": stringValue(req.Style),
		},
		Status: service.AIGenerationJobStatusRunning,
		Trace: service.AIWriteTrace{
			RequestID: traceRequestID,
			GroupID:   apiKey.GroupID,
			APIKeyID:  &apiKey.ID,
		},
	})
	if err != nil {
		return nil, err
	}

	endpoint := aiArtworkGenerationEndpoint(apiKey)
	payload := buildAIImageGenerationPayload(req, apiKey)
	result, err := h.invokeOpenAIGateway(ctx, endpoint, payload, apiKey, subscription, func(child *gin.Context) {
		h.openAIGateway.Images(child)
	})
	if err != nil {
		return nil, h.propagateAIGenerationJobFailure(ctx, userID, job.ID, apiKey.GroupID, &apiKey.ID, err)
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		err = openAIErrorFromGatewayResult(result)
		return nil, h.propagateAIGenerationJobFailure(ctx, userID, job.ID, apiKey.GroupID, &apiKey.ID, err)
	}
	successStatus := service.AIGenerationJobStatusSucceeded
	responseID := strings.TrimSpace(gjson.GetBytes(result.Body, "id").String())
	if responseID == "" {
		responseID = strings.TrimSpace(gjson.GetBytes(result.Body, "response.id").String())
	}
	usageRequestID := service.ResolveUsageRequestID(ctx, responseID)
	imageBytes, mimeType, revisedPrompt, err := decodeAIImageResult(ctx, result.Body)
	if err != nil {
		return nil, h.propagateAIGenerationJobFailure(ctx, userID, job.ID, apiKey.GroupID, &apiKey.ID, err)
	}
	asset, err := h.buildAIArtworkAsset(ctx, userID, job, apiKey, req, imageBytes, mimeType, revisedPrompt, usageRequestID)
	if err != nil {
		return nil, h.propagateAIGenerationJobFailure(ctx, userID, job.ID, apiKey.GroupID, &apiKey.ID, err)
	}
	gatewayResponseModel := strings.TrimSpace(gjson.GetBytes(result.Body, "model").String())
	if gatewayResponseModel == "" {
		gatewayResponseModel = strings.TrimSpace(gjson.GetBytes(result.Body, "response.model").String())
	}
	updateParams := map[string]any{
		"gateway_status":   result.StatusCode,
		"gateway_model":    model,
		"gateway_endpoint": endpoint,
	}
	if gatewayResponseModel != "" && gatewayResponseModel != model {
		updateParams["gateway_response_model"] = gatewayResponseModel
	}
	if len(result.Body) > 0 {
		updateParams["gateway_response"] = json.RawMessage(result.Body)
	}
	if err := h.completeAIGenerationJob(ctx, userID, job.ID, apiKey.GroupID, &apiKey.ID, &service.AIUpdateGenerationJobInput{
		Model:      &model,
		Status:     &successStatus,
		Parameters: &updateParams,
		Trace: service.AIWriteTrace{
			RequestID: usageRequestID,
			GroupID:   apiKey.GroupID,
			APIKeyID:  &apiKey.ID,
		},
	}); err != nil {
		return nil, err
	}
	return dto.AIArtworkFromService(asset, userID), nil
}

func (h *AIHandler) editArtworkViaGateway(ctx context.Context, userID int64, req *legacyCreateArtworkRequest) (any, error) {
	sourceImage := strings.TrimSpace(stringValue(req.SourceImage))
	if sourceImage == "" {
		return nil, infraerrors.BadRequest("AI_ARTWORK_SOURCE_IMAGE_REQUIRED", "source_image is required for artwork edits")
	}

	apiKey, err := h.resolveAIExecutionKey(ctx, userID, req.LineID, req.KeyID)
	if err != nil {
		return nil, err
	}
	subscription, err := h.resolveAIExecutionSubscription(ctx, userID, apiKey.Group)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(stringValue(req.Title))
	if title == "" {
		title = strings.TrimSpace(req.Prompt)
		if len(title) > 24 {
			title = title[:24]
		}
	}
	if title == "" {
		title = "AI Artwork Edit"
	}

	model := resolveAIArtworkModel(apiKey)
	traceRequestID := service.ResolveUsageRequestID(ctx, "")
	job, err := h.aiService.CreateGenerationJob(ctx, userID, &service.AICreateGenerationJobInput{
		PromptTemplateID: req.PromptTemplateID,
		Model:            model,
		Prompt:           req.Prompt,
		NegativePrompt:   req.NegativePrompt,
		Size:             req.Size,
		ImageCount:       intPtr(1),
		Parameters: map[string]any{
			"title":        title,
			"style":        stringValue(req.Style),
			"mode":         "edit",
			"source_image": sourceImage,
			"mask_image":   strings.TrimSpace(stringValue(req.MaskImage)),
		},
		Status: service.AIGenerationJobStatusRunning,
		Trace: service.AIWriteTrace{
			RequestID: traceRequestID,
			GroupID:   apiKey.GroupID,
			APIKeyID:  &apiKey.ID,
		},
	})
	if err != nil {
		return nil, err
	}

	endpoint := aiArtworkEditEndpoint(apiKey)
	payload := buildAIImageEditPayload(req, apiKey)
	result, err := h.invokeOpenAIGateway(ctx, endpoint, payload, apiKey, subscription, func(child *gin.Context) {
		h.openAIGateway.Images(child)
	})
	if err != nil {
		return nil, h.propagateAIGenerationJobFailure(ctx, userID, job.ID, apiKey.GroupID, &apiKey.ID, err)
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		err = openAIErrorFromGatewayResult(result)
		return nil, h.propagateAIGenerationJobFailure(ctx, userID, job.ID, apiKey.GroupID, &apiKey.ID, err)
	}

	successStatus := service.AIGenerationJobStatusSucceeded
	responseID := strings.TrimSpace(gjson.GetBytes(result.Body, "id").String())
	if responseID == "" {
		responseID = strings.TrimSpace(gjson.GetBytes(result.Body, "response.id").String())
	}
	usageRequestID := service.ResolveUsageRequestID(ctx, responseID)
	imageBytes, mimeType, revisedPrompt, err := decodeAIImageResult(ctx, result.Body)
	if err != nil {
		return nil, h.propagateAIGenerationJobFailure(ctx, userID, job.ID, apiKey.GroupID, &apiKey.ID, err)
	}
	asset, err := h.buildAIArtworkAsset(ctx, userID, job, apiKey, req, imageBytes, mimeType, revisedPrompt, usageRequestID)
	if err != nil {
		return nil, h.propagateAIGenerationJobFailure(ctx, userID, job.ID, apiKey.GroupID, &apiKey.ID, err)
	}
	gatewayResponseModel := strings.TrimSpace(gjson.GetBytes(result.Body, "model").String())
	if gatewayResponseModel == "" {
		gatewayResponseModel = strings.TrimSpace(gjson.GetBytes(result.Body, "response.model").String())
	}
	updateParams := map[string]any{
		"gateway_status":   result.StatusCode,
		"gateway_model":    model,
		"gateway_endpoint": endpoint,
		"mode":             "edit",
	}
	if gatewayResponseModel != "" && gatewayResponseModel != model {
		updateParams["gateway_response_model"] = gatewayResponseModel
	}
	if len(result.Body) > 0 {
		updateParams["gateway_response"] = json.RawMessage(result.Body)
	}
	if err := h.completeAIGenerationJob(ctx, userID, job.ID, apiKey.GroupID, &apiKey.ID, &service.AIUpdateGenerationJobInput{
		Model:      &model,
		Status:     &successStatus,
		Parameters: &updateParams,
		Trace: service.AIWriteTrace{
			RequestID: usageRequestID,
			GroupID:   apiKey.GroupID,
			APIKeyID:  &apiKey.ID,
		},
	}); err != nil {
		return nil, err
	}
	return dto.AIArtworkFromService(asset, userID), nil
}

func (h *AIHandler) completeAIGenerationJob(ctx context.Context, userID, jobID int64, groupID, apiKeyID *int64, input *service.AIUpdateGenerationJobInput) error {
	if h.aiService == nil {
		return infraerrors.ServiceUnavailable("AI_SERVICE_UNAVAILABLE", "ai service is unavailable")
	}
	_, err := h.aiService.UpdateGenerationJob(ctx, userID, userID, jobID, input)
	if err == nil {
		return nil
	}
	return h.propagateAIGenerationJobFailure(ctx, userID, jobID, groupID, apiKeyID, err)
}

func (h *AIHandler) propagateAIGenerationJobFailure(ctx context.Context, userID, jobID int64, groupID, apiKeyID *int64, cause error) error {
	if cause == nil {
		return nil
	}
	if err := h.failAIGenerationJob(ctx, userID, jobID, groupID, apiKeyID, cause); err != nil {
		return errors.Join(cause, fmt.Errorf("mark ai generation job failed: %w", err))
	}
	return cause
}

func (h *AIHandler) failAIGenerationJob(ctx context.Context, userID, jobID int64, groupID, apiKeyID *int64, cause error) error {
	if h.aiService == nil || cause == nil {
		return nil
	}
	status := service.AIGenerationJobStatusFailed
	message := strings.TrimSpace(cause.Error())
	_, err := h.aiService.UpdateGenerationJob(ctx, userID, userID, jobID, &service.AIUpdateGenerationJobInput{
		Status:       &status,
		ErrorMessage: &message,
		Trace: service.AIWriteTrace{
			RequestID: service.ResolveUsageRequestID(ctx, ""),
			GroupID:   groupID,
			APIKeyID:  apiKeyID,
		},
	})
	return err
}

func (h *AIHandler) resolveAIExecutionKey(ctx context.Context, userID int64, lineID, keyID *int64) (*service.APIKey, error) {
	if h.apiKeyService == nil {
		return nil, infraerrors.ServiceUnavailable("AI_KEY_SERVICE_UNAVAILABLE", "api key service is unavailable")
	}
	if keyID != nil && *keyID > 0 {
		return h.loadAIExecutionKeyByID(ctx, userID, lineID, *keyID)
	}
	keys, err := h.apiKeyService.SearchAPIKeys(ctx, userID, "", aiCreativeCenterKeyScanLimit)
	if err != nil {
		return nil, err
	}
	for i := range keys {
		key := &keys[i]
		if key.GroupID == nil || !key.IsActive() {
			continue
		}
		if lineID != nil && *lineID > 0 && *key.GroupID != *lineID {
			continue
		}
		if err := h.apiKeyService.CheckAPIKeyQuotaAndExpiry(key); err != nil {
			continue
		}
		return h.loadAIExecutionKeyByID(ctx, userID, lineID, key.ID)
	}
	return nil, infraerrors.NotFound("AI_KEY_NOT_FOUND", "no usable API key found for the selected line")
}

func (h *AIHandler) loadAIExecutionKeyByID(ctx context.Context, userID int64, lineID *int64, keyID int64) (*service.APIKey, error) {
	key, err := h.apiKeyService.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}
	if key.UserID != userID {
		return nil, infraerrors.Forbidden("AI_KEY_FORBIDDEN", "api key does not belong to the current user")
	}
	if key.GroupID == nil || key.Group == nil || !key.Group.IsActive() {
		return nil, infraerrors.Forbidden("AI_KEY_GROUP_INVALID", "api key is not bound to an active group")
	}
	if lineID != nil && *lineID > 0 && *lineID != *key.GroupID {
		return nil, infraerrors.BadRequest("AI_LINE_MISMATCH", "selected key does not belong to the selected line")
	}
	if err := h.apiKeyService.CheckAPIKeyQuotaAndExpiry(key); err != nil {
		return nil, err
	}
	return key, nil
}

func (h *AIHandler) resolveAIExecutionSubscription(ctx context.Context, userID int64, group *service.Group) (*service.UserSubscription, error) {
	if h.subscriptionService == nil || group == nil || !group.IsSubscriptionType() {
		return nil, nil
	}
	sub, err := h.subscriptionService.GetActiveSubscription(ctx, userID, group.ID)
	if err != nil && !errors.Is(err, service.ErrSubscriptionNotFound) {
		return nil, err
	}
	return sub, nil
}

func (h *AIHandler) invokeOpenAIGateway(parentCtx context.Context, path string, payload []byte, apiKey *service.APIKey, subscription *service.UserSubscription, run func(*gin.Context)) (*aiGatewayExecutionResult, error) {
	if h.openAIGateway == nil {
		return nil, infraerrors.ServiceUnavailable("AI_GATEWAY_UNAVAILABLE", "openai gateway handler is unavailable")
	}
	if apiKey == nil || apiKey.User == nil || apiKey.Group == nil {
		return nil, infraerrors.BadRequest("AI_GATEWAY_INVALID_CONTEXT", "api key context is incomplete")
	}
	recorder := httptest.NewRecorder()
	child, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	if parentCtx != nil {
		req = req.WithContext(parentCtx)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	child.Request = req
	child.Params = gin.Params{}
	child.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	child.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{
		UserID:      apiKey.User.ID,
		Concurrency: apiKey.User.Concurrency,
	})
	child.Set(string(middleware2.ContextKeyUserRole), apiKey.User.Role)
	if subscription != nil {
		child.Set(string(middleware2.ContextKeySubscription), subscription)
	}
	run(child)
	return &aiGatewayExecutionResult{
		StatusCode: recorder.Code,
		Body:       append([]byte(nil), recorder.Body.Bytes()...),
		Header:     recorder.Header().Clone(),
	}, nil
}

func buildAIChatCompletionsPayload(req *legacyChatRequest, apiKey *service.APIKey) []byte {
	messages := buildAIChatMessages(req)
	model := aiCreativeCenterDefaultChatModel
	if apiKey != nil && apiKey.Group != nil {
		if mapped := strings.TrimSpace(apiKey.Group.DefaultMappedModel); mapped != "" {
			model = mapped
		}
	}
	payload := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
	}
	body, _ := json.Marshal(payload)
	return body
}

func buildAIResponsesChatPayload(req *legacyChatRequest, apiKey *service.APIKey) []byte {
	model := aiCreativeCenterDefaultChatModel
	if apiKey != nil && apiKey.Group != nil {
		if mapped := strings.TrimSpace(apiKey.Group.DefaultMappedModel); mapped != "" {
			model = mapped
		}
	}
	payload := map[string]any{
		"model":  model,
		"stream": false,
		"store":  false,
		"input":  buildAIResponsesInputMessages(req),
	}
	body, _ := json.Marshal(payload)
	return body
}

func buildAIChatMessages(req *legacyChatRequest) []map[string]any {
	if req == nil {
		return nil
	}
	messages := make([]map[string]any, 0, len(req.History)+1)
	prompt := strings.TrimSpace(req.Prompt)
	for i := range req.History {
		msg := cloneAIMap(req.History[i])
		role := strings.TrimSpace(firstStringValue(msg["role"]))
		content := strings.TrimSpace(firstStringValue(msg["content"]))
		if role == "" || content == "" {
			continue
		}
		if role != service.AIMessageRoleSystem && role != service.AIMessageRoleUser && role != service.AIMessageRoleAssistant && role != service.AIMessageRoleTool {
			role = service.AIMessageRoleUser
		}
		messages = append(messages, map[string]any{"role": role, "content": content})
	}
	if len(messages) == 0 {
		messages = append(messages, map[string]any{"role": service.AIMessageRoleUser, "content": prompt})
		return messages
	}
	lastRole := strings.TrimSpace(firstStringValue(messages[len(messages)-1]["role"]))
	lastContent := strings.TrimSpace(firstStringValue(messages[len(messages)-1]["content"]))
	if lastRole != service.AIMessageRoleUser || lastContent != prompt {
		messages = append(messages, map[string]any{"role": service.AIMessageRoleUser, "content": prompt})
	}
	return messages
}

func buildAIResponsesInputMessages(req *legacyChatRequest) []map[string]any {
	messages := buildAIChatMessages(req)
	if len(messages) == 0 {
		return nil
	}
	input := make([]map[string]any, 0, len(messages))
	for i := range messages {
		role := strings.TrimSpace(firstStringValue(messages[i]["role"]))
		content := strings.TrimSpace(firstStringValue(messages[i]["content"]))
		if role == "" || content == "" {
			continue
		}
		input = append(input, map[string]any{
			"type": "message",
			"role": role,
			"content": []map[string]any{
				{
					"type": "input_text",
					"text": content,
				},
			},
		})
	}
	return input
}

func buildAIImageGenerationPayload(req *legacyCreateArtworkRequest, apiKey *service.APIKey) []byte {
	model := resolveAIArtworkModel(apiKey)
	prompt := strings.TrimSpace(req.Prompt)
	if negative := strings.TrimSpace(stringValue(req.NegativePrompt)); negative != "" {
		prompt += "\n\nNegative prompt: " + negative
	}
	payload := map[string]any{
		"model":           model,
		"prompt":          prompt,
		"n":               1,
		"response_format": "b64_json",
	}
	if size := strings.TrimSpace(stringValue(req.Size)); size != "" {
		payload["size"] = size
	}
	if style := strings.TrimSpace(stringValue(req.Style)); style != "" {
		payload["style"] = style
	}
	body, _ := json.Marshal(payload)
	return body
}

func buildAIResponsesImageGenerationPayload(req *legacyCreateArtworkRequest, apiKey *service.APIKey) []byte {
	model := resolveAIArtworkModel(apiKey)
	prompt := strings.TrimSpace(req.Prompt)
	if negative := strings.TrimSpace(stringValue(req.NegativePrompt)); negative != "" {
		prompt += "\n\nNegative prompt: " + negative
	}
	input := []map[string]any{
		{
			"type": "message",
			"role": "user",
			"content": []map[string]any{
				{
					"type": "input_text",
					"text": prompt,
				},
			},
		},
	}
	tool := map[string]any{
		"type":          "image_generation",
		"action":        "generate",
		"model":         model,
		"output_format": "png",
	}
	if size := strings.TrimSpace(stringValue(req.Size)); size != "" {
		tool["size"] = size
	}
	if style := strings.TrimSpace(stringValue(req.Style)); style != "" {
		tool["style"] = style
	}
	payload := map[string]any{
		"model":        model,
		"stream":       false,
		"store":        false,
		"instructions": "",
		"input":        input,
		"tool_choice":  map[string]any{"type": "image_generation"},
		"tools":        []map[string]any{tool},
	}
	body, _ := json.Marshal(payload)
	return body
}

func buildAIImageEditPayload(req *legacyCreateArtworkRequest, apiKey *service.APIKey) []byte {
	prompt := strings.TrimSpace(req.Prompt)
	if negative := strings.TrimSpace(stringValue(req.NegativePrompt)); negative != "" {
		prompt += "\n\nNegative prompt: " + negative
	}
	payload := map[string]any{
		"model":           resolveAIArtworkEditModel(apiKey),
		"prompt":          prompt,
		"n":               1,
		"stream":          false,
		"response_format": "b64_json",
		"images": []map[string]any{
			{
				"image_url": strings.TrimSpace(stringValue(req.SourceImage)),
			},
		},
	}
	if size := strings.TrimSpace(stringValue(req.Size)); size != "" {
		payload["size"] = size
	}
	if style := strings.TrimSpace(stringValue(req.Style)); style != "" {
		payload["style"] = style
	}
	if maskImage := strings.TrimSpace(stringValue(req.MaskImage)); maskImage != "" {
		payload["mask"] = map[string]any{"image_url": maskImage}
	}
	body, _ := json.Marshal(payload)
	return body
}

func openAIErrorFromGatewayResult(result *aiGatewayExecutionResult) error {
	if result == nil {
		return infraerrors.ServiceUnavailable("AI_GATEWAY_EMPTY_RESULT", "empty gateway result")
	}
	message := strings.TrimSpace(gjson.GetBytes(result.Body, "error.message").String())
	if message == "" {
		message = strings.TrimSpace(gjson.GetBytes(result.Body, "message").String())
	}
	if message == "" {
		message = strings.TrimSpace(string(result.Body))
	}
	if message == "" {
		message = "gateway request failed"
	}
	return infraerrors.New(result.StatusCode, "AI_GATEWAY_ERROR", message)
}

func decodeAIImageResult(ctx context.Context, body []byte) ([]byte, string, string, error) {
	if len(body) == 0 {
		return nil, "", "", infraerrors.ServiceUnavailable("AI_GATEWAY_EMPTY_IMAGE_RESPONSE", "empty image response")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if imageBytes, mimeType, revisedPrompt, ok, err := decodeAIImageResultFromResponses(body); ok || err != nil {
		return imageBytes, mimeType, revisedPrompt, err
	}
	data := gjson.GetBytes(body, "data.0")
	if !data.Exists() {
		return nil, "", "", infraerrors.ServiceUnavailable("AI_GATEWAY_IMAGE_DATA_MISSING", "image response is missing data")
	}
	b64 := strings.TrimSpace(data.Get("b64_json").String())
	if b64 == "" {
		b64 = strings.TrimSpace(data.Get("base64").String())
	}
	if b64 == "" {
		b64 = strings.TrimSpace(data.Get("image_base64").String())
	}
	var imageBytes []byte
	var err error
	if b64 != "" {
		imageBytes, err = base64.StdEncoding.DecodeString(normalizeImageBase64(b64))
		if err != nil {
			return nil, "", "", err
		}
	} else {
		imageURL := strings.TrimSpace(data.Get("url").String())
		if imageURL == "" {
			imageURL = strings.TrimSpace(data.Get("image_url").String())
		}
		if imageURL == "" {
			return nil, "", "", infraerrors.ServiceUnavailable("AI_GATEWAY_IMAGE_URL_MISSING", "image response is missing a url or base64 payload")
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
		if err != nil {
			return nil, "", "", err
		}
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return nil, "", "", err
		}
		defer func() { _ = resp.Body.Close() }()
		imageBytes, err = io.ReadAll(io.LimitReader(resp.Body, 20<<20))
		if err != nil {
			return nil, "", "", err
		}
	}
	mimeType := http.DetectContentType(imageBytes)
	if mimeType == "application/octet-stream" {
		mimeType = "image/png"
	}
	revisedPrompt := strings.TrimSpace(data.Get("revised_prompt").String())
	return imageBytes, mimeType, revisedPrompt, nil
}

func decodeAIImageResultFromResponses(body []byte) ([]byte, string, string, bool, error) {
	if !gjson.ValidBytes(body) {
		return nil, "", "", false, nil
	}
	if strings.TrimSpace(gjson.GetBytes(body, "type").String()) == "response.completed" {
		body = []byte(gjson.GetBytes(body, "response").Raw)
	}
	output := gjson.GetBytes(body, "response.output")
	if !output.Exists() || !output.IsArray() {
		output = gjson.GetBytes(body, "output")
	}
	if !output.Exists() || !output.IsArray() {
		return nil, "", "", false, nil
	}
	for _, item := range output.Array() {
		if strings.TrimSpace(item.Get("type").String()) != "image_generation_call" {
			continue
		}
		result := strings.TrimSpace(item.Get("result").String())
		if result == "" {
			continue
		}
		imageBytes, err := base64.StdEncoding.DecodeString(normalizeImageBase64(result))
		if err != nil {
			return nil, "", "", true, err
		}
		outputFormat := strings.TrimSpace(item.Get("output_format").String())
		if outputFormat == "" {
			outputFormat = "png"
		}
		mimeType := aiImageOutputMIMEType(outputFormat)
		revisedPrompt := strings.TrimSpace(item.Get("revised_prompt").String())
		return imageBytes, mimeType, revisedPrompt, true, nil
	}
	return nil, "", "", false, nil
}

func aiImageOutputMIMEType(outputFormat string) string {
	if outputFormat == "" {
		return "image/png"
	}
	if strings.Contains(outputFormat, "/") {
		return outputFormat
	}
	switch strings.ToLower(strings.TrimSpace(outputFormat)) {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func extractAIResponseText(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	for _, path := range []string{
		"output_text",
		"response.output_text",
		"response.output.0.content.0.text",
		"response.output.0.content.0.output_text.text",
		"output.0.content.0.text",
		"output.0.content.0.output_text.text",
	} {
		if text := strings.TrimSpace(gjson.GetBytes(body, path).String()); text != "" {
			return text
		}
	}
	output := gjson.GetBytes(body, "response.output")
	if !output.Exists() || !output.IsArray() {
		output = gjson.GetBytes(body, "output")
	}
	if output.Exists() && output.IsArray() {
		for _, item := range output.Array() {
			if strings.TrimSpace(item.Get("type").String()) != "message" {
				continue
			}
			content := item.Get("content")
			if !content.Exists() || !content.IsArray() {
				continue
			}
			for _, part := range content.Array() {
				if text := strings.TrimSpace(part.Get("text").String()); text != "" {
					return text
				}
				if text := strings.TrimSpace(part.Get("output_text").String()); text != "" {
					return text
				}
			}
		}
	}
	return ""
}

func shouldUseAIResponsesForChat(req *legacyChatRequest) bool {
	if req == nil {
		return false
	}
	if req.UseResponses != nil {
		return *req.UseResponses
	}
	prompt := strings.ToLower(strings.TrimSpace(req.Prompt))
	if prompt == "" {
		return false
	}
	if strings.Contains(prompt, "generate image") ||
		strings.Contains(prompt, "create image") ||
		strings.Contains(prompt, "draw") ||
		strings.Contains(prompt, "image_generation") ||
		strings.Contains(prompt, "生成图片") ||
		strings.Contains(prompt, "画图") ||
		strings.Contains(prompt, "绘图") {
		return true
	}
	for i := range req.History {
		msg := req.History[i]
		if strings.Contains(strings.ToLower(firstStringValue(msg["type"])), "image") {
			return true
		}
		if strings.Contains(strings.ToLower(firstStringValue(msg["role"])), "tool") {
			return true
		}
		if content := firstStringValue(msg["content"]); strings.Contains(strings.ToLower(content), "image_generation") {
			return true
		}
	}
	return false
}

func resolveAIArtworkModel(apiKey *service.APIKey) string {
	if apiKey != nil && apiKey.Group != nil {
		model := strings.TrimSpace(apiKey.Group.DefaultMappedModel)
		if strings.HasPrefix(strings.ToLower(model), "gpt-image-") {
			return model
		}
	}
	return aiCreativeCenterDefaultImageModel
}

func resolveAIArtworkEditModel(apiKey *service.APIKey) string {
	return resolveAIArtworkModel(apiKey)
}

func aiArtworkGenerationEndpoint(apiKey *service.APIKey) string {
	if !shouldUseAIImages2APIEndpoint(apiKey) {
		return "/openai/v1/images/generations"
	}
	return "/openai/v1/images2api/generations"
}

func aiArtworkEditEndpoint(apiKey *service.APIKey) string {
	if !shouldUseAIImages2APIEndpoint(apiKey) {
		return "/openai/v1/images/edits"
	}
	return "/openai/v1/images2api/edits"
}

func shouldUseAIImages2APIEndpoint(apiKey *service.APIKey) bool {
	if apiKey == nil || apiKey.Group == nil {
		return false
	}
	group := apiKey.Group
	if group.Platform != service.PlatformOpenAI {
		return true
	}
	if group.Images2APIPrice1K != nil || group.Images2APIPrice2K != nil || group.Images2APIPrice4K != nil {
		return true
	}
	label := strings.ToLower(strings.TrimSpace(group.DisplayLabel()))
	name := strings.ToLower(strings.TrimSpace(group.Name))
	return strings.Contains(label, "images2api") ||
		strings.Contains(label, "2api") ||
		strings.Contains(name, "images2api") ||
		strings.Contains(name, "2api")
}

func (h *AIHandler) buildAIArtworkAsset(ctx context.Context, userID int64, job *service.AIGenerationJob, apiKey *service.APIKey, req *legacyCreateArtworkRequest, imageBytes []byte, mimeType string, revisedPrompt string, requestID string) (*service.AIAsset, error) {
	if job == nil || apiKey == nil || apiKey.User == nil || apiKey.Group == nil {
		return nil, infraerrors.BadRequest("AI_ARTWORK_CONTEXT_INVALID", "artwork context is incomplete")
	}
	width, height := detectImageDimensions(imageBytes)
	title := strings.TrimSpace(stringValue(req.Title))
	if title == "" {
		title = strings.TrimSpace(req.Prompt)
		if len(title) > 24 {
			title = title[:24]
		}
	}
	if title == "" {
		title = "AI Artwork"
	}
	checksum := sha256.Sum256(imageBytes)
	checksumHex := hex.EncodeToString(checksum[:])
	metadata := map[string]any{
		"title":           title,
		"prompt":          strings.TrimSpace(req.Prompt),
		"revised_prompt":  revisedPrompt,
		"negative_prompt": strings.TrimSpace(stringValue(req.NegativePrompt)),
		"mode":            strings.TrimSpace(req.Mode),
		"source_image":    strings.TrimSpace(stringValue(req.SourceImage)),
		"mask_image":      strings.TrimSpace(stringValue(req.MaskImage)),
		"size":            strings.TrimSpace(stringValue(req.Size)),
		"style":           strings.TrimSpace(stringValue(req.Style)),
		"tags":            append([]string(nil), req.Tags...),
		"featured":        req.Featured,
		"line_id":         apiKey.Group.ID,
		"key_id":          apiKey.ID,
		"job_id":          job.ID,
	}

	storageKind := "inline"
	storagePath := ""
	sourceURL := buildInlineDataURL(imageBytes, mimeType)
	visibility := defaultAIVisibility(req.Visibility)
	if h.mediaService != nil {
		if uploaded, err := h.mediaService.Upload(ctx, service.UploadMediaInput{
			BizType:    "ai_image",
			BizID:      strconv.FormatInt(job.ID, 10),
			Visibility: visibility,
			OwnerUserID: func() *int64 {
				v := userID
				return &v
			}(),
			FileName:    sanitizeAIImageFileName(title, mimeType),
			ContentType: mimeType,
			SizeBytes:   int64(len(imageBytes)),
			SHA256:      checksumHex,
			Width:       width,
			Height:      height,
			File:        imageBytes,
		}); err == nil && uploaded != nil {
			storageKind = "media"
			storagePath = uploaded.ObjectKey
			metadata["media_asset_id"] = uploaded.ID
			if visibility == service.MediaVisibilityPublic {
				if publicURL := h.mediaService.PublicURL(uploaded); publicURL != "" {
					sourceURL = publicURL
				}
				if uploaded.ThumbnailObjectKey != "" {
					if thumbURL := h.mediaService.ThumbnailPublicURL(uploaded); thumbURL != "" {
						metadata["thumbnail_url"] = thumbURL
					}
				}
			} else if signed, signErr := h.mediaService.CreateDownloadURLForUser(ctx, userID, uploaded.ID); signErr == nil && signed != nil {
				if signedURL := strings.TrimSpace(signed.URL); signedURL != "" {
					sourceURL = signedURL
				}
				if thumbSigned, thumbErr := h.mediaService.CreateThumbnailDownloadURLForUser(ctx, userID, uploaded.ID); thumbErr == nil && thumbSigned != nil {
					if signedURL := strings.TrimSpace(thumbSigned.URL); signedURL != "" {
						metadata["thumbnail_url"] = signedURL
					}
				}
			}
		}
	}
	if strings.TrimSpace(sourceURL) != "" {
		metadata["image_url"] = sourceURL
		if _, ok := metadata["thumbnail_url"]; !ok {
			metadata["thumbnail_url"] = sourceURL
		}
	}
	metadata["mime_type"] = mimeType
	asset := &service.AIAsset{
		UserID:           userID,
		GenerationJobID:  &job.ID,
		PromptTemplateID: req.PromptTemplateID,
		AssetType:        service.AIAssetTypeImage,
		Status:           service.AIAssetStatusReady,
		Visibility:       visibility,
		ModerationState:  service.AIModerationStateNormal,
		StorageKind:      storageKind,
		StoragePath:      storagePath,
		SourceURL:        sourceURL,
		MIMEType:         mimeType,
		Width:            width,
		Height:           height,
		ByteSize:         int64Ptr(int64(len(imageBytes))),
		Checksum:         checksumHex,
		Metadata:         metadata,
		Trace: service.AITraceRef{
			RequestID: strings.TrimSpace(requestID),
			GroupID:   apiKey.GroupID,
			APIKeyID:  &apiKey.ID,
		},
	}
	if err := h.aiService.CreateAssets(ctx, []*service.AIAsset{asset}); err != nil {
		return nil, err
	}
	if asset.ID > 0 {
		metadata["asset_id"] = asset.ID
		asset.Metadata = metadata
	}
	return asset, nil
}

func buildInlineDataURL(body []byte, mimeType string) string {
	if len(body) == 0 {
		return ""
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/png"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(body)
}

func sanitizeAIImageFileName(title, mimeType string) string {
	ext := ".png"
	if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
		ext = exts[0]
	}
	name := strings.TrimSpace(title)
	if name == "" {
		name = "ai-artwork"
	}
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	if len(name) > 48 {
		name = name[:48]
	}
	return name + ext
}

func detectImageDimensions(body []byte) (*int, *int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return nil, nil
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, nil
	}
	w := cfg.Width
	h := cfg.Height
	return &w, &h
}

func normalizeImageBase64(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(raw), "data:") {
		if idx := strings.Index(raw, ","); idx >= 0 && idx+1 < len(raw) {
			raw = raw[idx+1:]
		}
	}
	raw = strings.TrimSpace(raw)
	if mod := len(raw) % 4; mod != 0 {
		raw += strings.Repeat("=", 4-mod)
	}
	return raw
}

func intPtr(v int) *int {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func firstStringValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return ""
	}
}
