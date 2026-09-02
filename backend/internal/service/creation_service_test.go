//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type creationSessionRepoStub struct {
	items map[int64]*CreationSession
	next  int64
}

func (s *creationSessionRepoStub) Create(ctx context.Context, input *CreationSession) error {
	s.next++
	input.ID = s.next
	if s.items == nil {
		s.items = map[int64]*CreationSession{}
	}
	copy := *input
	s.items[input.ID] = &copy
	return nil
}

func (s *creationSessionRepoStub) GetByID(ctx context.Context, id int64) (*CreationSession, error) {
	if row, ok := s.items[id]; ok {
		return row, nil
	}
	return nil, ErrCreationSessionNotFound
}

func (s *creationSessionRepoStub) GetForUser(ctx context.Context, userID, id int64) (*CreationSession, error) {
	row, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.UserID != userID {
		return nil, ErrCreationSessionNotFound
	}
	return row, nil
}

func (s *creationSessionRepoStub) ListForUser(ctx context.Context, userID int64, filters CreationSessionListFilters) ([]CreationSession, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *creationSessionRepoStub) Update(ctx context.Context, id int64, input UpdateCreationSessionInput) (*CreationSession, error) {
	row, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Title != nil {
		row.Title = *input.Title
	}
	return row, nil
}

func (s *creationSessionRepoStub) Delete(ctx context.Context, userID, id int64) error {
	if _, err := s.GetForUser(ctx, userID, id); err != nil {
		return err
	}
	delete(s.items, id)
	return nil
}

type creationMessageRepoStub struct{}

func (s *creationMessageRepoStub) Create(ctx context.Context, msg *CreationMessage) error { return nil }
func (s *creationMessageRepoStub) ListBySession(ctx context.Context, sessionID int64) ([]CreationMessage, error) {
	return nil, nil
}

type creationImageJobRepoStub struct{}

func (s *creationImageJobRepoStub) Create(ctx context.Context, job *CreationImageJob) error { return nil }
func (s *creationImageJobRepoStub) GetByID(ctx context.Context, id int64) (*CreationImageJob, error) {
	return nil, ErrCreationImageNotFound
}
func (s *creationImageJobRepoStub) GetForUser(ctx context.Context, userID, id int64) (*CreationImageJob, error) {
	return nil, ErrCreationImageNotFound
}
func (s *creationImageJobRepoStub) ListForUser(ctx context.Context, userID int64, filters CreationImageListFilters) ([]CreationImageJob, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *creationImageJobRepoStub) Update(ctx context.Context, id int64, job *CreationImageJob) error {
	return nil
}

func TestCreationService_SessionCRUD(t *testing.T) {
	sessions := &creationSessionRepoStub{items: map[int64]*CreationSession{}}
	groupID := int64(3)
	svc := NewCreationService(
		sessions,
		&creationMessageRepoStub{},
		&creationImageJobRepoStub{},
		&groupRepoStubForGroupUpdate{group: &Group{
			ID:       groupID,
			Status:   StatusActive,
			Platform: PlatformOpenAI,
		}},
		&creationTestUserRepo{},
		&userSubRepoStubForGroupUpdate{getActiveErr: ErrSubscriptionNotFound},
	)

	created, err := svc.CreateSession(context.Background(), CreateCreationSessionInput{
		UserID:   7,
		GroupID:  groupID,
		Title:    "Studio",
		Mode:     CreationSessionModeChat,
		Metadata: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), created.ID)

	got, err := svc.GetSession(context.Background(), 7, 1)
	require.NoError(t, err)
	require.Equal(t, "Studio", got.Title)

	title := "Renamed"
	updated, err := svc.UpdateSession(context.Background(), 7, 1, UpdateCreationSessionInput{Title: &title})
	require.NoError(t, err)
	require.Equal(t, "Renamed", updated.Title)

	require.NoError(t, svc.DeleteSession(context.Background(), 7, 1))
	_, err = svc.GetSession(context.Background(), 7, 1)
	require.ErrorIs(t, err, ErrCreationSessionNotFound)
}

type creationTestUserRepo struct {
	userRepoStubForGroupUpdate
}

func (s *creationTestUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	return &User{ID: id, Status: StatusActive}, nil
}
