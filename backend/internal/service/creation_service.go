package service

import (
	"context"
	"encoding/json"
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
	mode := NormalizeCreationSessionMode(input.Mode)
	if mode == "" {
		mode = CreationSessionModeChat
	}
	if err := s.ensureUserCanUseGroup(ctx, input.UserID, input.GroupID); err != nil {
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
		if NormalizeCreationSessionStatus(*input.Status) == "" {
			return nil, ErrCreationInvalidMode
		}
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
	return items, total, nil
}

func (s *CreationService) GetImage(ctx context.Context, userID, imageID int64) (*CreationImageJob, error) {
	return s.imageJobs.GetForUser(ctx, userID, imageID)
}

func (s *CreationService) ensureUserCanUseGroup(ctx context.Context, userID, groupID int64) error {
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
	if group.IsSubscriptionType() {
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
