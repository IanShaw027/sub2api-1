package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type CreationService struct {
	sessions  CreationSessionRepository
	messages  CreationMessageRepository
	imageJobs CreationImageJobRepository
	groups    GroupRepository
	users     UserRepository
	userSubs  UserSubscriptionRepository
}

func NewCreationService(
	sessions CreationSessionRepository,
	messages CreationMessageRepository,
	imageJobs CreationImageJobRepository,
	groups GroupRepository,
	users UserRepository,
	userSubs UserSubscriptionRepository,
) *CreationService {
	return &CreationService{
		sessions:  sessions,
		messages:  messages,
		imageJobs: imageJobs,
		groups:    groups,
		users:     users,
		userSubs:  userSubs,
	}
}

func ProvideCreationService(
	sessions CreationSessionRepository,
	messages CreationMessageRepository,
	imageJobs CreationImageJobRepository,
	groups GroupRepository,
	users UserRepository,
	userSubs UserSubscriptionRepository,
) *CreationService {
	return NewCreationService(sessions, messages, imageJobs, groups, users, userSubs)
}

func (s *CreationService) CreateSession(ctx context.Context, input CreateCreationSessionInput) (*CreationSession, error) {
	rawMode := strings.TrimSpace(input.Mode)
	mode := NormalizeCreationSessionMode(rawMode)
	if rawMode == "" {
		mode = CreationSessionModeChat
	} else if mode == "" {
		return nil, ErrCreationInvalidMode
	}
	if err := s.EnsureUserCanUseGroup(ctx, input.UserID, input.GroupID); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "New session"
	}
	row := &CreationSession{
		UserID:   input.UserID,
		GroupID:  input.GroupID,
		Title:    title,
		Model:    strings.TrimSpace(input.Model),
		Mode:     mode,
		Status:   CreationSessionStatusActive,
		Metadata: input.Metadata,
	}
	if len(row.Metadata) == 0 {
		row.Metadata = json.RawMessage(`{}`)
	}
	if err := s.sessions.Create(ctx, row); err != nil {
		return nil, fmt.Errorf("create creation session: %w", err)
	}
	return row, nil
}

func (s *CreationService) ListSessions(ctx context.Context, userID int64, filters CreationSessionListFilters) ([]CreationSession, int64, error) {
	items, page, err := s.sessions.ListForUser(ctx, userID, filters)
	if err != nil {
		return nil, 0, err
	}
	total := int64(0)
	if page != nil {
		total = page.Total
	}
	return items, total, nil
}

func (s *CreationService) GetSession(ctx context.Context, userID, sessionID int64) (*CreationSession, error) {
	row, err := s.sessions.GetForUser(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *CreationService) UpdateSession(ctx context.Context, userID, sessionID int64, input UpdateCreationSessionInput) (*CreationSession, error) {
	if _, err := s.sessions.GetForUser(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	if input.Status != nil {
		normalized := NormalizeCreationSessionStatus(strings.TrimSpace(*input.Status))
		if normalized == "" {
			return nil, ErrCreationInvalidStatus
		}
		input.Status = &normalized
	}
	return s.sessions.Update(ctx, sessionID, input)
}

func (s *CreationService) DeleteSession(ctx context.Context, userID, sessionID int64) error {
	return s.sessions.Delete(ctx, userID, sessionID)
}

func (s *CreationService) ListMessages(ctx context.Context, userID, sessionID int64) ([]CreationMessage, error) {
	if _, err := s.sessions.GetForUser(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	return s.messages.ListBySession(ctx, sessionID)
}

func (s *CreationService) CreateMessage(ctx context.Context, input CreateCreationMessageInput) (*CreationMessage, error) {
	role := NormalizeCreationMessageRole(input.Role)
	if role == "" {
		return nil, infraerrors.BadRequest("CREATION_INVALID_ROLE", "invalid message role")
	}
	if _, err := s.sessions.GetForUser(ctx, input.UserID, input.SessionID); err != nil {
		return nil, err
	}
	if len(input.Content) == 0 {
		return nil, infraerrors.BadRequest("CREATION_EMPTY_CONTENT", "message content is required")
	}
	msg := &CreationMessage{
		SessionID: input.SessionID,
		Role:      role,
		Content:   input.Content,
		Model:     input.Model,
	}
	if err := s.messages.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("create creation message: %w", err)
	}
	return msg, nil
}

func (s *CreationService) ListImages(ctx context.Context, userID int64, filters CreationImageListFilters) ([]CreationImageJob, int64, error) {
	items, page, err := s.imageJobs.ListForUser(ctx, userID, filters)
	if err != nil {
		return nil, 0, err
	}
	total := int64(0)
	if page != nil {
		total = page.Total
	}
	for i := range items {
		populateCreationImageMediaURL(&items[i])
	}
	return items, total, nil
}

func (s *CreationService) GetImage(ctx context.Context, userID, imageID int64) (*CreationImageJob, error) {
	job, err := s.imageJobs.GetForUser(ctx, userID, imageID)
	if err != nil {
		return nil, err
	}
	populateCreationImageMediaURL(job)
	return job, nil
}

func (s *CreationService) CreateImageJob(ctx context.Context, input CreateCreationImageJobInput) (*CreationImageJob, error) {
	if s == nil || s.imageJobs == nil {
		return nil, fmt.Errorf("creation image job repository unavailable")
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = CreationImageJobStatusProcessing
	}
	sessionID := input.SessionID
	if sessionID != nil && *sessionID > 0 && s.sessions != nil {
		if _, err := s.sessions.GetForUser(ctx, input.UserID, *sessionID); err != nil {
			sessionID = nil
		}
	} else {
		sessionID = nil
	}
	job := &CreationImageJob{
		UserID:    input.UserID,
		GroupID:   input.GroupID,
		SessionID: sessionID,
		Status:    status,
		Model:     strings.TrimSpace(input.Model),
		Prompt:    strings.TrimSpace(input.Prompt),
	}
	if taskID := strings.TrimSpace(input.ProviderTaskID); taskID != "" {
		job.ProviderTaskID = &taskID
	}
	if err := s.imageJobs.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create creation image job: %w", err)
	}
	populateCreationImageMediaURL(job)
	return job, nil
}

func (s *CreationService) SyncImageJobFromTask(ctx context.Context, userID int64, task *ImageTask) error {
	if s == nil || s.imageJobs == nil || task == nil {
		return nil
	}
	taskID := strings.TrimSpace(task.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(task.ID)
	}
	if taskID == "" || userID <= 0 {
		return nil
	}
	job, err := s.imageJobs.GetByProviderTaskID(ctx, userID, taskID)
	if err != nil {
		if errors.Is(err, ErrCreationImageNotFound) {
			return nil
		}
		return err
	}
	if job == nil {
		return nil
	}
	// A late processing poll must not overwrite a background terminal update.
	if (job.Status == CreationImageJobStatusCompleted || job.Status == CreationImageJobStatusFailed) &&
		(task.Status == ImageTaskStatusProcessing || task.Status == CreationImageJobStatusPending) {
		return nil
	}
	job.Status = creationImageJobStatusFromTask(task.Status)
	job.Error = creationImageJobErrorFromTask(task)
	if task.ImageURL != "" {
		job.MediaURL = task.ImageURL
	}
	if task.StorageObject != nil {
		job.StorageID = task.StorageObject.StorageID
		job.StorageKey = task.StorageObject.Key
	}
	if mediaAssetID := creationImageMediaAssetIDFromTask(task); mediaAssetID != nil {
		job.MediaAssetID = mediaAssetID
	}
	if err := s.imageJobs.Update(ctx, job.ID, job); err != nil {
		return fmt.Errorf("sync creation image job: %w", err)
	}
	return nil
}

func populateCreationImageMediaURL(job *CreationImageJob) {
	if job == nil || strings.TrimSpace(job.MediaURL) != "" {
		return
	}
	if job.MediaAssetID != nil && *job.MediaAssetID > 0 {
		job.MediaURL = MediaPublicPath(*job.MediaAssetID)
	}
}

func (s *CreationService) GetImageByTaskID(ctx context.Context, userID int64, taskID string) (*CreationImageJob, error) {
	return s.imageJobs.GetByProviderTaskID(ctx, userID, taskID)
}

func creationImageJobStatusFromTask(status string) string {
	switch strings.TrimSpace(status) {
	case CreationImageJobStatusCompleted:
		return CreationImageJobStatusCompleted
	case CreationImageJobStatusFailed:
		return CreationImageJobStatusFailed
	case CreationImageJobStatusPending:
		return CreationImageJobStatusPending
	default:
		return CreationImageJobStatusProcessing
	}
}

func creationImageJobErrorFromTask(task *ImageTask) *string {
	if task == nil {
		return nil
	}
	if task.Status == ImageTaskStatusCompleted {
		empty := ""
		return &empty
	}
	if len(task.Error) == 0 {
		return nil
	}
	var envelope struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(task.Error, &envelope) == nil && strings.TrimSpace(envelope.Message) != "" {
		msg := strings.TrimSpace(envelope.Message)
		return &msg
	}
	raw := strings.TrimSpace(string(task.Error))
	if raw == "" || raw == "null" {
		return nil
	}
	return &raw
}

func creationImageMediaAssetIDFromTask(task *ImageTask) *int64 {
	if task == nil {
		return nil
	}
	if id, ok := ParseManagedMediaID(task.ImageURL); ok {
		return &id
	}
	return nil
}

// EnsureUserCanUseGroup validates the same group boundary for session CRUD and
// delegated gateway requests. Client-side available-group filtering is not an
// authorization boundary.
func (s *CreationService) EnsureUserCanUseGroup(ctx context.Context, userID, groupID int64) error {
	return s.ensureUserCanAccessGroup(ctx, userID, groupID, true)
}

// EnsureUserCanReadGroup preserves access to already accepted tasks after a subscription expires.
func (s *CreationService) EnsureUserCanReadGroup(ctx context.Context, userID, groupID int64) error {
	return s.ensureUserCanAccessGroup(ctx, userID, groupID, false)
}

func (s *CreationService) ensureUserCanAccessGroup(ctx context.Context, userID, groupID int64, requireSubscription bool) error {
	if s == nil || s.users == nil || s.groups == nil || s.userSubs == nil {
		return fmt.Errorf("creation group authorization is not configured")
	}
	if userID <= 0 {
		return ErrCreationGroupNotAllowed
	}
	if groupID <= 0 {
		return ErrCreationGroupRequired
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	group, err := s.groups.GetByID(ctx, groupID)
	if err != nil {
		return err
	}
	if !user.IsActive() || !group.IsActive() {
		return ErrCreationGroupNotAllowed
	}
	if group.IsSubscriptionType() {
		if !requireSubscription {
			return nil
		}
		_, err = s.userSubs.GetActiveByUserIDAndGroupID(ctx, userID, groupID)
		if err != nil {
			return ErrCreationGroupNotAllowed
		}
		return nil
	}
	if !user.CanBindGroup(group.ID, group.IsExclusive) {
		return ErrCreationGroupNotAllowed
	}
	return nil
}
