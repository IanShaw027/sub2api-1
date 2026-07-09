package service

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type AISendMessageResult struct {
	Session      *AISession         `json:"session,omitempty"`
	Messages     []AISessionMessage `json:"messages,omitempty"`
	UserMessage  *AISessionMessage  `json:"user_message,omitempty"`
	AssistantMsg *AISessionMessage  `json:"assistant_message,omitempty"`
}

type AICreateGenerationJobResult struct {
	Job *AIGenerationJob `json:"job,omitempty"`
}

type AIRuntimeKeyService interface {
	GetAvailableRouteGroups(ctx context.Context, userID int64) ([]Group, error)
	SearchAPIKeys(ctx context.Context, userID int64, keyword string, limit int) ([]APIKey, error)
	CheckAPIKeyQuotaAndExpiry(apiKey *APIKey) error
}

type AIRuntimeLine struct {
	GroupID            int64          `json:"group_id"`
	Label              string         `json:"label"`
	Platform           string         `json:"platform"`
	Description        string         `json:"description,omitempty"`
	Keys               []AIRuntimeKey `json:"keys,omitempty"`
	KeyIDs             []int64        `json:"key_ids,omitempty"`
	KeyCount           int            `json:"key_count"`
	DefaultKeyID       *int64         `json:"default_key_id,omitempty"`
	DefaultMappedModel string         `json:"default_mapped_model,omitempty"`
}

type AIRuntimeKey struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type AIRuntimeModel struct {
	Name     string `json:"name"`
	Label    string `json:"label,omitempty"`
	GroupID  *int64 `json:"group_id,omitempty"`
	Platform string `json:"platform,omitempty"`
}

type AIRuntimeInfo struct {
	Lines       []AIRuntimeLine  `json:"lines,omitempty"`
	DefaultLine *AIRuntimeLine   `json:"default_line,omitempty"`
	LineCount   int              `json:"line_count"`
	KeyCount    int              `json:"key_count"`
	Models      []AIRuntimeModel `json:"models,omitempty"`
}

func (s *AICenterService) CreateSession(ctx context.Context, userID int64, input *AICreateSessionInput) (*AISession, error) {
	if s == nil || input == nil {
		return nil, infraerrors.BadRequest("AI_SESSION_INPUT_REQUIRED", "ai session input is required")
	}
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "New chat"
	}
	session := &AISession{
		UserID:       userID,
		Title:        title,
		Status:       AISessionStatusActive,
		Metadata:     cloneAIMap(input.Metadata),
		Trace:        normalizeAIWriteTrace(ctx, input.Trace),
		SystemPrompt: "",
	}
	if input.SystemPrompt != nil {
		session.SystemPrompt = strings.TrimSpace(*input.SystemPrompt)
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	if err := repo.CreateAuditLog(ctx, &AIAuditLog{
		OwnerUserID: &userID,
		EntityType:  domainAIAuditEntitySession,
		EntityID:    &session.ID,
		Action:      "create",
		AfterState:  cloneAIMap(map[string]any{"title": session.Title, "status": session.Status}),
		Trace:       session.Trace,
	}); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *AICenterService) GetSession(ctx context.Context, userID, sessionID int64) (*AISession, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetSessionByUserAndID(ctx, userID, sessionID)
}

func (s *AICenterService) ListSessions(ctx context.Context, userID int64, params pagination.PaginationParams, status string) ([]AISession, *pagination.PaginationResult, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, nil, err
	}
	return repo.ListSessions(ctx, userID, params, status)
}

func (s *AICenterService) UpdateSession(ctx context.Context, userID, sessionID int64, input *AIUpdateSessionInput) (*AISession, error) {
	if s == nil || input == nil {
		return nil, infraerrors.BadRequest("AI_SESSION_INPUT_REQUIRED", "ai session input is required")
	}
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	session, err := repo.GetSessionByUserAndID(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	before := cloneAIMap(map[string]any{"title": session.Title, "status": session.Status, "system_prompt": session.SystemPrompt})
	if input.Title != nil {
		session.Title = strings.TrimSpace(*input.Title)
	}
	if input.Status != nil {
		session.Status = strings.TrimSpace(*input.Status)
	}
	if input.SystemPrompt != nil {
		if *input.SystemPrompt == nil {
			session.SystemPrompt = ""
		} else {
			session.SystemPrompt = strings.TrimSpace(**input.SystemPrompt)
		}
	}
	if input.Metadata != nil {
		session.Metadata = cloneAIMap(*input.Metadata)
	}
	session.Trace = normalizeAIWriteTrace(ctx, input.Trace)
	if err := repo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}
	if err := repo.CreateAuditLog(ctx, &AIAuditLog{
		OwnerUserID: &userID,
		EntityType:  domainAIAuditEntitySession,
		EntityID:    &session.ID,
		Action:      "update",
		BeforeState: before,
		AfterState:  cloneAIMap(map[string]any{"title": session.Title, "status": session.Status, "system_prompt": session.SystemPrompt}),
		Trace:       session.Trace,
	}); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *AICenterService) DeleteSession(ctx context.Context, userID, sessionID int64) error {
	repo, err := s.requireRepo()
	if err != nil {
		return err
	}
	session, err := repo.GetSessionByUserAndID(ctx, userID, sessionID)
	if err != nil {
		return err
	}
	if err := repo.DeleteSession(ctx, sessionID); err != nil {
		return err
	}
	return repo.CreateAuditLog(ctx, &AIAuditLog{
		OwnerUserID: &userID,
		EntityType:  domainAIAuditEntitySession,
		EntityID:    &session.ID,
		Action:      "delete",
		Trace:       session.Trace,
	})
}

func (s *AICenterService) ListSessionMessages(ctx context.Context, userID, sessionID int64, params pagination.PaginationParams) ([]AISessionMessage, *pagination.PaginationResult, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, nil, err
	}
	if _, err := repo.GetSessionByUserAndID(ctx, userID, sessionID); err != nil {
		return nil, nil, err
	}
	return repo.ListSessionMessages(ctx, sessionID, params)
}

func (s *AICenterService) SendMessage(ctx context.Context, userID, sessionID int64, input *AISendMessageInput) (*AISendMessageResult, error) {
	if s == nil || input == nil {
		return nil, infraerrors.BadRequest("AI_MESSAGE_INPUT_REQUIRED", "ai message input is required")
	}
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	session, err := repo.GetSessionByUserAndID(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	message := &AISessionMessage{
		SessionID:    sessionID,
		UserID:       userID,
		Role:         normalizeAIAIMessageRole(input.Role),
		Status:       normalizeAIAIMessageStatus(input.Status, AIMessageStatusCompleted),
		Content:      strings.TrimSpace(input.Content),
		ContentParts: cloneAIMapSlice(input.ContentParts),
		Model:        strings.TrimSpace(input.Model),
		Provider:     strings.TrimSpace(input.Provider),
		Metadata:     map[string]any{},
		Trace:        normalizeAIWriteTrace(ctx, input.Trace),
	}
	if input.ReplyToMessageID != nil {
		message.ReplyToMessageID = input.ReplyToMessageID
	}
	if err := repo.CreateSessionMessages(ctx, []*AISessionMessage{message}); err != nil {
		return nil, err
	}
	msgs := []AISessionMessage{*message}
	var assistant *AISessionMessage
	if input.CreateAssistantReply {
		assistant = &AISessionMessage{
			SessionID: sessionID,
			UserID:    userID,
			Role:      AIMessageRoleAssistant,
			Status:    normalizeAIAIMessageStatus(input.AssistantReplyStatus, AIMessageStatusQueued),
			Content:   strings.TrimSpace(input.AssistantReplyContent),
			Model:     strings.TrimSpace(input.AssistantReplyModel),
			Provider:  strings.TrimSpace(input.AssistantReplyProvider),
			Metadata:  cloneAIMap(input.AssistantReplyMetadata),
			Trace:     normalizeAIWriteTrace(ctx, input.Trace),
		}
		if err := repo.CreateSessionMessages(ctx, []*AISessionMessage{assistant}); err != nil {
			return nil, err
		}
		msgs = append(msgs, *assistant)
	}
	now := time.Now()
	session.LastMessageAt = &now
	session.Trace = normalizeAIWriteTrace(ctx, input.Trace)
	if err := repo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}
	if err := repo.CreateAuditLog(ctx, &AIAuditLog{
		OwnerUserID: &userID,
		EntityType:  domainAIAuditEntitySessionMessage,
		EntityID:    &message.ID,
		Action:      "send",
		AfterState:  cloneAIMap(map[string]any{"role": message.Role, "status": message.Status}),
		Trace:       message.Trace,
	}); err != nil {
		return nil, err
	}
	return &AISendMessageResult{Session: session, Messages: msgs, UserMessage: message, AssistantMsg: assistant}, nil
}

func (s *AICenterService) GetRuntime(ctx context.Context, userID int64, apiKeyService AIRuntimeKeyService) (*AIRuntimeInfo, error) {
	info := &AIRuntimeInfo{}
	if s == nil || apiKeyService == nil {
		return info, nil
	}

	groups, err := apiKeyService.GetAvailableRouteGroups(ctx, userID)
	if err != nil {
		return info, nil
	}
	keys, err := apiKeyService.SearchAPIKeys(ctx, userID, "", apiKeyRouteSelectionScanLimit)
	if err != nil {
		return info, nil
	}

	keyIDsByGroup := make(map[int64][]int64)
	keyNamesByID := make(map[int64]string, len(keys))
	for i := range keys {
		key := keys[i]
		if key.GroupID == nil || *key.GroupID <= 0 || !key.IsActive() {
			continue
		}
		if err := apiKeyService.CheckAPIKeyQuotaAndExpiry(&key); err != nil {
			continue
		}
		keyIDsByGroup[*key.GroupID] = append(keyIDsByGroup[*key.GroupID], key.ID)
		if name := strings.TrimSpace(key.Name); name != "" {
			keyNamesByID[key.ID] = name
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		left := strings.ToLower(groups[i].DisplayLabel())
		right := strings.ToLower(groups[j].DisplayLabel())
		if left == right {
			return groups[i].ID < groups[j].ID
		}
		return left < right
	})

	seenModels := make(map[string]struct{})
	for _, group := range groups {
		keyIDs := append([]int64(nil), keyIDsByGroup[group.ID]...)
		if len(keyIDs) == 0 {
			continue
		}
		displayKeyIDs := append([]int64(nil), keyIDs...)
		sort.Slice(displayKeyIDs, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(keyNamesByID[displayKeyIDs[i]]))
			right := strings.ToLower(strings.TrimSpace(keyNamesByID[displayKeyIDs[j]]))
			if left == right {
				return displayKeyIDs[i] < displayKeyIDs[j]
			}
			return left < right
		})
		displayKeys := make([]AIRuntimeKey, 0, len(displayKeyIDs))
		for _, keyID := range displayKeyIDs {
			keyName := strings.TrimSpace(keyNamesByID[keyID])
			if keyName == "" {
				keyName = "Key " + strconv.FormatInt(keyID, 10)
			}
			displayKeys = append(displayKeys, AIRuntimeKey{ID: keyID, Name: keyName})
		}
		line := AIRuntimeLine{
			GroupID:            group.ID,
			Label:              group.DisplayLabel(),
			Platform:           group.Platform,
			Description:        strings.TrimSpace(group.Description),
			Keys:               displayKeys,
			KeyIDs:             keyIDs,
			KeyCount:           len(keyIDs),
			DefaultMappedModel: strings.TrimSpace(group.DefaultMappedModel),
		}
		defaultKeyID := keyIDs[0]
		line.DefaultKeyID = &defaultKeyID
		info.Lines = append(info.Lines, line)
		info.KeyCount += len(keyIDs)

		modelName := strings.TrimSpace(group.DefaultMappedModel)
		if modelName == "" {
			continue
		}
		if _, exists := seenModels[modelName]; exists {
			continue
		}
		seenModels[modelName] = struct{}{}
		groupID := group.ID
		info.Models = append(info.Models, AIRuntimeModel{
			Name:     modelName,
			Label:    group.DisplayLabel(),
			GroupID:  &groupID,
			Platform: group.Platform,
		})
	}

	info.LineCount = len(info.Lines)
	if len(info.Lines) > 0 {
		defaultLine := info.Lines[0]
		info.DefaultLine = &defaultLine
	}
	sort.Slice(info.Models, func(i, j int) bool {
		if info.Models[i].Name == info.Models[j].Name {
			return info.Models[i].Label < info.Models[j].Label
		}
		return info.Models[i].Name < info.Models[j].Name
	})
	return info, nil
}

func (s *AICenterService) CreatePromptTemplate(ctx context.Context, userID int64, input *AICreatePromptTemplateInput) (*AIPromptTemplate, error) {
	if s == nil || input == nil {
		return nil, infraerrors.BadRequest("AI_PROMPT_TEMPLATE_INPUT_REQUIRED", "ai prompt template input is required")
	}
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, infraerrors.BadRequest("AI_PROMPT_TEMPLATE_TITLE_REQUIRED", "prompt template title is required")
	}
	template := &AIPromptTemplate{
		UserID:          userID,
		Title:           title,
		Description:     strings.TrimSpace(valueOrEmpty(input.Description)),
		Category:        strings.TrimSpace(valueOrEmpty(input.Category)),
		Tags:            cloneStringSlice(input.Tags),
		Visibility:      domain.NormalizeAIVisibility(input.Visibility),
		ModerationState: AIModerationStateNormal,
		CurrentVersion:  1,
		ModelHint:       strings.TrimSpace(valueOrEmpty(input.ModelHint)),
		Content:         strings.TrimSpace(input.Content),
		Variables:       cloneAIMapSlice(input.Variables),
		Metadata:        cloneAIMap(input.Metadata),
		CoverAssetID:    input.CoverAssetID,
		Trace:           normalizeAIWriteTrace(ctx, input.Trace),
	}
	version := &AIPromptTemplateVersion{
		UserID:     userID,
		Version:    1,
		Title:      template.Title,
		Content:    template.Content,
		ModelHint:  template.ModelHint,
		Variables:  cloneAIMapSlice(template.Variables),
		ChangeNote: strings.TrimSpace(valueOrEmpty(input.ChangeNote)),
		Metadata:   cloneAIMap(template.Metadata),
		Trace:      template.Trace,
	}
	if err := repo.CreatePromptTemplate(ctx, template, version); err != nil {
		return nil, err
	}
	if err := repo.CreateAuditLog(ctx, &AIAuditLog{
		OwnerUserID: &userID,
		EntityType:  domainAIAuditEntityPromptTemplate,
		EntityID:    &template.ID,
		Action:      "create",
		AfterState:  cloneAIMap(map[string]any{"title": template.Title, "visibility": template.Visibility, "moderation_state": template.ModerationState}),
		Trace:       template.Trace,
	}); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *AICenterService) UpdatePromptTemplate(ctx context.Context, userID, templateID int64, input *AIUpdatePromptTemplateInput) (*AIPromptTemplate, error) {
	if s == nil || input == nil {
		return nil, infraerrors.BadRequest("AI_PROMPT_TEMPLATE_INPUT_REQUIRED", "ai prompt template input is required")
	}
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	template, err := repo.GetPromptTemplateByUserAndID(ctx, userID, templateID)
	if err != nil {
		return nil, err
	}
	before := cloneAIMap(map[string]any{"title": template.Title, "content": template.Content, "visibility": template.Visibility, "moderation_state": template.ModerationState})
	if input.Title != nil {
		template.Title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		template.Description = strings.TrimSpace(valueOrEmpty(normalizeAIStringPtr(input.Description)))
	}
	if input.Category != nil {
		template.Category = strings.TrimSpace(valueOrEmpty(normalizeAIStringPtr(input.Category)))
	}
	if input.Tags != nil {
		template.Tags = cloneStringSlice(*input.Tags)
	}
	if input.Visibility != nil {
		template.Visibility = domain.NormalizeAIVisibility(*input.Visibility)
	}
	if input.ModelHint != nil {
		template.ModelHint = strings.TrimSpace(valueOrEmpty(normalizeAIStringPtr(input.ModelHint)))
	}
	if input.Content != nil {
		template.Content = strings.TrimSpace(*input.Content)
	}
	if input.Variables != nil {
		template.Variables = cloneAIMapSlice(*input.Variables)
	}
	if input.Metadata != nil {
		template.Metadata = cloneAIMap(*input.Metadata)
	}
	if input.CoverAssetID != nil {
		template.CoverAssetID = normalizeAIInt64Ptr(input.CoverAssetID)
	}
	template.CurrentVersion++
	template.Trace = normalizeAIWriteTrace(ctx, input.Trace)
	version := &AIPromptTemplateVersion{
		UserID:     userID,
		Version:    template.CurrentVersion,
		Title:      template.Title,
		Content:    template.Content,
		ModelHint:  template.ModelHint,
		Variables:  cloneAIMapSlice(template.Variables),
		ChangeNote: strings.TrimSpace(valueOrEmpty(input.ChangeNote)),
		Metadata:   cloneAIMap(template.Metadata),
		Trace:      template.Trace,
	}
	if err := repo.UpdatePromptTemplate(ctx, template, version); err != nil {
		return nil, err
	}
	if err := repo.CreateAuditLog(ctx, &AIAuditLog{
		OwnerUserID: &userID,
		EntityType:  domainAIAuditEntityPromptTemplate,
		EntityID:    &template.ID,
		Action:      "update",
		BeforeState: before,
		AfterState:  cloneAIMap(map[string]any{"title": template.Title, "content": template.Content, "visibility": template.Visibility, "moderation_state": template.ModerationState}),
		Trace:       template.Trace,
	}); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *AICenterService) DeletePromptTemplate(ctx context.Context, userID, templateID int64) error {
	repo, err := s.requireRepo()
	if err != nil {
		return err
	}
	template, err := repo.GetPromptTemplateByUserAndID(ctx, userID, templateID)
	if err != nil {
		return err
	}
	if err := repo.DeletePromptTemplate(ctx, templateID); err != nil {
		return err
	}
	return repo.CreateAuditLog(ctx, &AIAuditLog{
		OwnerUserID: &userID,
		EntityType:  domainAIAuditEntityPromptTemplate,
		EntityID:    &template.ID,
		Action:      "delete",
		Trace:       template.Trace,
	})
}

func (s *AICenterService) GetPromptTemplate(ctx context.Context, userID int64, templateID int64) (*AIPromptTemplate, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	template, err := repo.GetPromptTemplateByUserAndID(ctx, userID, templateID)
	if err == nil {
		return template, nil
	}
	template, err = repo.GetPromptTemplateByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if !domain.CanReadPromptTemplate(template.UserID, userID, false, template.Visibility, template.ModerationState) {
		return nil, ErrAIPromptTemplateNotFound
	}
	return template, nil
}

func (s *AICenterService) ListPromptTemplates(ctx context.Context, userID int64, isAdmin bool, params pagination.PaginationParams, filter AIListPromptTemplatesFilter) ([]AIPromptTemplate, *pagination.PaginationResult, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, nil, err
	}
	return repo.ListPromptTemplates(ctx, userID, isAdmin, params, filter)
}

func (s *AICenterService) CreateGenerationJob(ctx context.Context, userID int64, input *AICreateGenerationJobInput) (*AIGenerationJob, error) {
	if s == nil || input == nil {
		return nil, infraerrors.BadRequest("AI_GENERATION_JOB_INPUT_REQUIRED", "ai generation job input is required")
	}
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	job := &AIGenerationJob{
		UserID:           userID,
		SessionID:        input.SessionID,
		PromptTemplateID: input.PromptTemplateID,
		Status:           normalizeAIGenerationJobStatus(input.Status),
		Model:            strings.TrimSpace(input.Model),
		Prompt:           strings.TrimSpace(input.Prompt),
		NegativePrompt:   strings.TrimSpace(valueOrEmpty(input.NegativePrompt)),
		Size:             strings.TrimSpace(valueOrEmpty(input.Size)),
		ImageCount:       valueOrDefault(input.ImageCount, 1),
		Seed:             input.Seed,
		Parameters:       cloneAIMap(input.Parameters),
		Trace:            normalizeAIWriteTrace(ctx, input.Trace),
	}
	if job.Status == "" {
		job.Status = AIGenerationJobStatusQueued
	}
	if job.Model == "" || job.Prompt == "" {
		return nil, infraerrors.BadRequest("AI_GENERATION_JOB_INVALID", "model and prompt are required")
	}
	if err := repo.CreateGenerationJob(ctx, job); err != nil {
		return nil, err
	}
	if err := repo.CreateAuditLog(ctx, &AIAuditLog{
		OwnerUserID: &userID,
		EntityType:  domainAIAuditEntityGenerationJob,
		EntityID:    &job.ID,
		Action:      "create",
		AfterState:  cloneAIMap(map[string]any{"status": job.Status, "model": job.Model}),
		Trace:       job.Trace,
	}); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *AICenterService) CreateAssets(ctx context.Context, assets []*AIAsset) error {
	if s == nil {
		return infraerrors.BadRequest("AI_ASSET_INPUT_REQUIRED", "ai assets are required")
	}
	repo, err := s.requireRepo()
	if err != nil {
		return err
	}
	return repo.CreateAssets(ctx, assets)
}

func (s *AICenterService) ListGenerationJobs(ctx context.Context, userID int64, params pagination.PaginationParams, filter AIListGenerationJobsFilter) ([]AIGenerationJob, *pagination.PaginationResult, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, nil, err
	}
	return repo.ListGenerationJobs(ctx, userID, false, params, filter)
}

func (s *AICenterService) GetGenerationJob(ctx context.Context, userID, jobID int64) (*AIGenerationJob, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetGenerationJobByUserAndID(ctx, userID, jobID)
}

func (s *AICenterService) ListGallery(ctx context.Context, userID int64, params pagination.PaginationParams, filter AIListAssetsFilter) ([]AIAsset, *pagination.PaginationResult, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, nil, err
	}
	items, result, err := repo.ListAssets(ctx, userID, false, params, filter)
	if err != nil {
		return nil, nil, err
	}
	decorated := make([]AIAsset, 0, len(items))
	for i := range items {
		item := items[i]
		if resolved := s.decorateAIAsset(ctx, userID, false, &item); resolved != nil {
			decorated = append(decorated, *resolved)
			continue
		}
		decorated = append(decorated, item)
	}
	return decorated, result, nil
}

func (s *AICenterService) ListAssets(ctx context.Context, userID int64, isAdmin bool, params pagination.PaginationParams, filter AIListAssetsFilter) ([]AIAsset, *pagination.PaginationResult, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, nil, err
	}
	return repo.ListAssets(ctx, userID, isAdmin, params, filter)
}

func (s *AICenterService) GetAsset(ctx context.Context, userID, assetID int64) (*AIAsset, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	asset, err := repo.GetAssetByUserAndID(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	return s.decorateAIAsset(ctx, userID, false, asset), nil
}

func (s *AICenterService) UpdateGenerationJob(ctx context.Context, operatorUserID, userID, jobID int64, input *AIUpdateGenerationJobInput) (*AIGenerationJob, error) {
	if s == nil || input == nil {
		return nil, infraerrors.BadRequest("AI_GENERATION_JOB_INPUT_REQUIRED", "ai generation job input is required")
	}
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	job, err := repo.GetGenerationJobByUserAndID(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	before := cloneAIMap(map[string]any{"status": job.Status, "model": job.Model})
	if input.Status != nil {
		job.Status = normalizeAIGenerationJobStatus(*input.Status)
	}
	if input.Model != nil {
		job.Model = strings.TrimSpace(*input.Model)
	}
	if input.Prompt != nil {
		job.Prompt = strings.TrimSpace(*input.Prompt)
	}
	if input.NegativePrompt != nil {
		job.NegativePrompt = strings.TrimSpace(*input.NegativePrompt)
	}
	if input.Size != nil {
		job.Size = strings.TrimSpace(*input.Size)
	}
	if input.ImageCount != nil {
		job.ImageCount = *input.ImageCount
	}
	if input.Seed != nil {
		job.Seed = input.Seed
	}
	if input.ErrorMessage != nil {
		job.ErrorMessage = strings.TrimSpace(*input.ErrorMessage)
	}
	if input.Parameters != nil {
		job.Parameters = cloneAIMap(*input.Parameters)
	}
	job.Trace = normalizeAIWriteTrace(ctx, input.Trace)
	if err := repo.UpdateGenerationJob(ctx, job); err != nil {
		return nil, err
	}
	if err := repo.CreateAuditLog(ctx, &AIAuditLog{
		OperatorUserID: &operatorUserID,
		OwnerUserID:    &userID,
		EntityType:     domainAIAuditEntityGenerationJob,
		EntityID:       &job.ID,
		Action:         "update",
		BeforeState:    before,
		AfterState:     cloneAIMap(map[string]any{"status": job.Status, "model": job.Model}),
		Trace:          job.Trace,
	}); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *AICenterService) UpdateAsset(ctx context.Context, operatorUserID, userID, assetID int64, input *AIUpdateAssetInput) (*AIAsset, error) {
	if s == nil || input == nil {
		return nil, infraerrors.BadRequest("AI_ASSET_INPUT_REQUIRED", "ai asset input is required")
	}
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	asset, err := repo.GetAssetByUserAndID(ctx, userID, assetID)
	if err != nil {
		return nil, err
	}
	before := cloneAIMap(map[string]any{"status": asset.Status, "visibility": asset.Visibility, "moderation_state": asset.ModerationState})
	if input.Status != nil {
		asset.Status = normalizeAIAssetStatus(*input.Status)
	}
	if input.Visibility != nil {
		asset.Visibility = domain.NormalizeAIVisibility(*input.Visibility)
	}
	if input.ModerationState != nil {
		asset.ModerationState = domain.NormalizeAIModerationState(*input.ModerationState)
	}
	if input.StorageKind != nil {
		asset.StorageKind = valueOrEmpty(normalizeAIStringPtr(input.StorageKind))
	}
	if input.StoragePath != nil {
		asset.StoragePath = valueOrEmpty(normalizeAIStringPtr(input.StoragePath))
	}
	if input.SourceURL != nil {
		asset.SourceURL = valueOrEmpty(normalizeAIStringPtr(input.SourceURL))
	}
	if input.MIMEType != nil {
		asset.MIMEType = valueOrEmpty(normalizeAIStringPtr(input.MIMEType))
	}
	if input.Width != nil {
		asset.Width = normalizeAIIntPtr(input.Width)
	}
	if input.Height != nil {
		asset.Height = normalizeAIIntPtr(input.Height)
	}
	if input.ByteSize != nil {
		asset.ByteSize = normalizeAIInt64Ptr(input.ByteSize)
	}
	if input.Checksum != nil {
		asset.Checksum = valueOrEmpty(normalizeAIStringPtr(input.Checksum))
	}
	if input.Metadata != nil {
		asset.Metadata = cloneAIMap(*input.Metadata)
	}
	asset.Trace = normalizeAIWriteTrace(ctx, input.Trace)
	if err := repo.UpdateAsset(ctx, asset); err != nil {
		return nil, err
	}
	if err := repo.CreateAuditLog(ctx, &AIAuditLog{
		OperatorUserID: &operatorUserID,
		OwnerUserID:    &userID,
		EntityType:     domainAIAuditEntityAsset,
		EntityID:       &asset.ID,
		Action:         "update",
		BeforeState:    before,
		AfterState:     cloneAIMap(map[string]any{"status": asset.Status, "visibility": asset.Visibility, "moderation_state": asset.ModerationState}),
		Trace:          asset.Trace,
	}); err != nil {
		return nil, err
	}
	return asset, nil
}

func (s *AICenterService) AdminListPromptTemplates(ctx context.Context, operatorUserID int64, params pagination.PaginationParams, filter AIListPromptTemplatesFilter) ([]AIPromptTemplate, *pagination.PaginationResult, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, nil, err
	}
	return repo.ListPromptTemplates(ctx, operatorUserID, true, params, filter)
}

func (s *AICenterService) AdminGetPromptTemplate(ctx context.Context, templateID int64) (*AIPromptTemplate, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetPromptTemplateByID(ctx, templateID)
}

func (s *AICenterService) AdminModeratePromptTemplate(ctx context.Context, operatorUserID, templateID int64, moderationState string, reason string) (*AIPromptTemplate, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	template, err := repo.GetPromptTemplateByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	before := cloneAIMap(map[string]any{"moderation_state": template.ModerationState, "visibility": template.Visibility})
	template.ModerationState = domain.NormalizeAIModerationState(moderationState)
	if template.ModerationState == AIModerationStateForcedPrivate {
		template.Visibility = AIVisibilityPrivate
	}
	template.Trace = normalizeAIWriteTrace(ctx, AIWriteTrace{})
	if err := repo.UpdatePromptTemplate(ctx, template, nil); err != nil {
		return nil, err
	}
	if err := repo.CreateAuditLog(ctx, &AIAuditLog{
		OperatorUserID: &operatorUserID,
		OwnerUserID:    &template.UserID,
		EntityType:     domainAIAuditEntityPromptTemplate,
		EntityID:       &template.ID,
		Action:         "moderate",
		Reason:         reason,
		BeforeState:    before,
		AfterState:     cloneAIMap(map[string]any{"moderation_state": template.ModerationState, "visibility": template.Visibility}),
		Trace:          template.Trace,
	}); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *AICenterService) AdminListGenerationJobs(ctx context.Context, operatorUserID int64, params pagination.PaginationParams, filter AIListGenerationJobsFilter) ([]AIGenerationJob, *pagination.PaginationResult, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, nil, err
	}
	return repo.ListGenerationJobs(ctx, operatorUserID, true, params, filter)
}

func (s *AICenterService) AdminGetGenerationJob(ctx context.Context, jobID int64) (*AIGenerationJob, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetGenerationJobByID(ctx, jobID)
}

func (s *AICenterService) AdminModerateGenerationJob(ctx context.Context, operatorUserID, jobID int64, input *AIUpdateGenerationJobInput) (*AIGenerationJob, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	job, err := repo.GetGenerationJobByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return s.UpdateGenerationJob(ctx, operatorUserID, job.UserID, jobID, input)
}

func (s *AICenterService) AdminListAssets(ctx context.Context, operatorUserID int64, params pagination.PaginationParams, filter AIListAssetsFilter) ([]AIAsset, *pagination.PaginationResult, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, nil, err
	}
	items, result, err := repo.ListAssets(ctx, operatorUserID, true, params, filter)
	if err != nil {
		return nil, nil, err
	}
	decorated := make([]AIAsset, 0, len(items))
	for i := range items {
		item := items[i]
		if resolved := s.decorateAIAsset(ctx, operatorUserID, true, &item); resolved != nil {
			decorated = append(decorated, *resolved)
			continue
		}
		decorated = append(decorated, item)
	}
	return decorated, result, nil
}

func (s *AICenterService) AdminGetAsset(ctx context.Context, assetID int64) (*AIAsset, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	asset, err := repo.GetAssetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return s.decorateAIAsset(ctx, asset.UserID, true, asset), nil
}

func (s *AICenterService) AdminModerateAsset(ctx context.Context, operatorUserID, assetID int64, input *AIUpdateAssetInput) (*AIAsset, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, err
	}
	asset, err := repo.GetAssetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return s.UpdateAsset(ctx, operatorUserID, asset.UserID, assetID, input)
}

func (s *AICenterService) AdminListAuditLogs(ctx context.Context, params pagination.PaginationParams, filter AIListAuditLogsFilter) ([]AIAuditLog, *pagination.PaginationResult, error) {
	repo, err := s.requireRepo()
	if err != nil {
		return nil, nil, err
	}
	return repo.ListAuditLogs(ctx, params, filter)
}

func (s *AICenterService) decorateAIAsset(ctx context.Context, viewerUserID int64, isAdmin bool, asset *AIAsset) *AIAsset {
	if s == nil || asset == nil || s.mediaService == nil {
		return asset
	}
	mediaAssetID, ok := aiMediaAssetIDFromMetadata(asset.Metadata)
	if !ok || mediaAssetID <= 0 {
		return asset
	}
	mediaAsset, err := s.resolveMediaAsset(ctx, viewerUserID, isAdmin, mediaAssetID)
	if err != nil || mediaAsset == nil {
		return asset
	}
	resolved := *asset
	resolved.Metadata = cloneAIMap(asset.Metadata)
	imageURL, thumbnailURL := s.resolveMediaAssetURLs(ctx, viewerUserID, isAdmin, mediaAsset)
	if strings.TrimSpace(imageURL) != "" {
		resolved.SourceURL = imageURL
		resolved.Metadata["image_url"] = imageURL
	}
	if strings.TrimSpace(thumbnailURL) != "" {
		resolved.Metadata["thumbnail_url"] = thumbnailURL
	} else if strings.TrimSpace(imageURL) != "" {
		resolved.Metadata["thumbnail_url"] = imageURL
	}
	return &resolved
}

func (s *AICenterService) resolveMediaAsset(ctx context.Context, viewerUserID int64, isAdmin bool, mediaAssetID int64) (*MediaAsset, error) {
	if s == nil || s.mediaService == nil {
		return nil, nil
	}
	if isAdmin {
		return s.mediaService.GetForAdmin(ctx, mediaAssetID)
	}
	return s.mediaService.GetForUser(ctx, viewerUserID, mediaAssetID)
}

func (s *AICenterService) resolveMediaAssetURLs(ctx context.Context, viewerUserID int64, isAdmin bool, mediaAsset *MediaAsset) (string, string) {
	if mediaAsset == nil || s == nil || s.mediaService == nil {
		return "", ""
	}
	visibility := strings.TrimSpace(mediaAsset.Visibility)
	var imageURL, thumbnailURL string
	if visibility == MediaVisibilityPublic {
		imageURL = s.mediaService.PublicURL(mediaAsset)
		thumbnailURL = s.mediaService.ThumbnailPublicURL(mediaAsset)
		return imageURL, thumbnailURL
	}
	if isAdmin {
		if signed, err := s.mediaService.CreateDownloadURLForAdmin(ctx, mediaAsset.ID); err == nil && signed != nil {
			imageURL = strings.TrimSpace(signed.URL)
		}
		if signed, err := s.mediaService.CreateThumbnailDownloadURLForAdmin(ctx, mediaAsset.ID); err == nil && signed != nil {
			thumbnailURL = strings.TrimSpace(signed.URL)
		}
	} else {
		if signed, err := s.mediaService.CreateDownloadURLForUser(ctx, viewerUserID, mediaAsset.ID); err == nil && signed != nil {
			imageURL = strings.TrimSpace(signed.URL)
		}
		if signed, err := s.mediaService.CreateThumbnailDownloadURLForUser(ctx, viewerUserID, mediaAsset.ID); err == nil && signed != nil {
			thumbnailURL = strings.TrimSpace(signed.URL)
		}
	}
	if thumbnailURL == "" {
		thumbnailURL = imageURL
	}
	return imageURL, thumbnailURL
}

func aiMediaAssetIDFromMetadata(metadata map[string]any) (int64, bool) {
	if metadata == nil {
		return 0, false
	}
	switch v := metadata["media_asset_id"].(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && parsed > 0 {
			return parsed, true
		}
	}
	return 0, false
}

func normalizeAIAIMessageRole(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AIMessageRoleSystem:
		return AIMessageRoleSystem
	case AIMessageRoleAssistant:
		return AIMessageRoleAssistant
	case AIMessageRoleTool:
		return AIMessageRoleTool
	default:
		return AIMessageRoleUser
	}
}

func normalizeAIAIMessageStatus(raw, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AIMessageStatusAccepted:
		return AIMessageStatusAccepted
	case AIMessageStatusQueued:
		return AIMessageStatusQueued
	case AIMessageStatusCompleted:
		return AIMessageStatusCompleted
	case AIMessageStatusFailed:
		return AIMessageStatusFailed
	default:
		return fallback
	}
}

func normalizeAIGenerationJobStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AIGenerationJobStatusRunning:
		return AIGenerationJobStatusRunning
	case AIGenerationJobStatusSucceeded:
		return AIGenerationJobStatusSucceeded
	case AIGenerationJobStatusFailed:
		return AIGenerationJobStatusFailed
	case AIGenerationJobStatusCanceled:
		return AIGenerationJobStatusCanceled
	default:
		return AIGenerationJobStatusQueued
	}
}

func normalizeAIAssetStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AIAssetStatusReady:
		return AIAssetStatusReady
	case AIAssetStatusHidden:
		return AIAssetStatusHidden
	case AIAssetStatusDeleted:
		return AIAssetStatusDeleted
	default:
		return AIAssetStatusPending
	}
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func valueOrDefault(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}

const (
	domainAIAuditEntitySession        = "ai_session"
	domainAIAuditEntitySessionMessage = "ai_session_message"
	domainAIAuditEntityPromptTemplate = "ai_prompt_template"
	domainAIAuditEntityGenerationJob  = "ai_generation_job"
	domainAIAuditEntityAsset          = "ai_asset"
)
