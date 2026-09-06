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

type creationImageJobRepoStub struct {
	items map[int64]*CreationImageJob
	next  int64
}

func (s *creationImageJobRepoStub) Create(_ context.Context, job *CreationImageJob) error {
	s.next++
	job.ID = s.next
	if s.items == nil {
		s.items = map[int64]*CreationImageJob{}
	}
	copy := *job
	if job.SessionID != nil {
		v := *job.SessionID
		copy.SessionID = &v
	}
	if job.ProviderTaskID != nil {
		v := *job.ProviderTaskID
		copy.ProviderTaskID = &v
	}
	if job.MediaAssetID != nil {
		v := *job.MediaAssetID
		copy.MediaAssetID = &v
	}
	if job.Error != nil {
		v := *job.Error
		copy.Error = &v
	}
	s.items[job.ID] = &copy
	return nil
}

func (s *creationImageJobRepoStub) GetByID(_ context.Context, id int64) (*CreationImageJob, error) {
	if row, ok := s.items[id]; ok {
		return cloneCreationImageJob(row), nil
	}
	return nil, ErrCreationImageNotFound
}

func (s *creationImageJobRepoStub) GetForUser(_ context.Context, userID, id int64) (*CreationImageJob, error) {
	row, err := s.GetByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if row.UserID != userID {
		return nil, ErrCreationImageNotFound
	}
	return row, nil
}

func (s *creationImageJobRepoStub) GetByProviderTaskID(_ context.Context, userID int64, providerTaskID string) (*CreationImageJob, error) {
	for _, row := range s.items {
		if row.UserID == userID && row.ProviderTaskID != nil && *row.ProviderTaskID == providerTaskID {
			return cloneCreationImageJob(row), nil
		}
	}
	return nil, ErrCreationImageNotFound
}

func (s *creationImageJobRepoStub) ListForUser(_ context.Context, userID int64, _ CreationImageListFilters) ([]CreationImageJob, *pagination.PaginationResult, error) {
	out := make([]CreationImageJob, 0)
	for _, row := range s.items {
		if row.UserID == userID {
			out = append(out, *cloneCreationImageJob(row))
		}
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: 1, PageSize: 20}, nil
}

func (s *creationImageJobRepoStub) Update(_ context.Context, id int64, job *CreationImageJob) error {
	row, ok := s.items[id]
	if !ok {
		return ErrCreationImageNotFound
	}
	row.Status = job.Status
	row.MediaURL = job.MediaURL
	row.StorageID = job.StorageID
	row.StorageKey = job.StorageKey
	if job.MediaAssetID != nil {
		v := *job.MediaAssetID
		row.MediaAssetID = &v
	}
	if job.ProviderTaskID != nil {
		v := *job.ProviderTaskID
		row.ProviderTaskID = &v
	}
	if job.Error != nil {
		v := *job.Error
		row.Error = &v
	}
	return nil
}

func cloneCreationImageJob(job *CreationImageJob) *CreationImageJob {
	if job == nil {
		return nil
	}
	copy := *job
	if job.SessionID != nil {
		v := *job.SessionID
		copy.SessionID = &v
	}
	if job.ProviderTaskID != nil {
		v := *job.ProviderTaskID
		copy.ProviderTaskID = &v
	}
	if job.MediaAssetID != nil {
		v := *job.MediaAssetID
		copy.MediaAssetID = &v
	}
	if job.Error != nil {
		v := *job.Error
		copy.Error = &v
	}
	return &copy
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

func TestCreationService_RejectsInvalidSessionModeAndStatus(t *testing.T) {
	sessions := &creationSessionRepoStub{items: map[int64]*CreationSession{
		1: {ID: 1, UserID: 7, GroupID: 3, Mode: CreationSessionModeChat},
	}}
	svc := NewCreationService(
		sessions,
		&creationMessageRepoStub{},
		&creationImageJobRepoStub{},
		&groupRepoStubForGroupUpdate{group: &Group{ID: 3, Status: StatusActive}},
		&creationTestUserRepo{},
		&userSubRepoStubForGroupUpdate{getActiveErr: ErrSubscriptionNotFound},
	)

	_, err := svc.CreateSession(context.Background(), CreateCreationSessionInput{
		UserID: 7, GroupID: 3, Mode: "video",
	})
	require.ErrorIs(t, err, ErrCreationInvalidMode)

	invalidStatus := "deleted"
	_, err = svc.UpdateSession(context.Background(), 7, 1, UpdateCreationSessionInput{Status: &invalidStatus})
	require.ErrorIs(t, err, ErrCreationInvalidStatus)
}

func TestCreationService_CreateAndSyncImageJob(t *testing.T) {
	jobs := &creationImageJobRepoStub{}
	svc := NewCreationService(
		&creationSessionRepoStub{items: map[int64]*CreationSession{
			11: {ID: 11, UserID: 7, GroupID: 3, Title: "Studio", Mode: CreationSessionModeImage, Status: CreationSessionStatusActive},
		}},
		&creationMessageRepoStub{},
		jobs,
		&groupRepoStubForGroupUpdate{group: &Group{ID: 3, Status: StatusActive, Platform: PlatformOpenAI}},
		&creationTestUserRepo{},
		&userSubRepoStubForGroupUpdate{getActiveErr: ErrSubscriptionNotFound},
	)

	sessionID := int64(11)
	created, err := svc.CreateImageJob(context.Background(), CreateCreationImageJobInput{
		UserID:         7,
		GroupID:        3,
		SessionID:      &sessionID,
		Model:          "gpt-image-1",
		Prompt:         "a cat",
		ProviderTaskID: "imgtask_abc",
		Status:         CreationImageJobStatusProcessing,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), created.ID)
	require.Equal(t, "gpt-image-1", created.Model)
	require.Equal(t, "a cat", created.Prompt)
	require.Equal(t, CreationImageJobStatusProcessing, created.Status)
	require.NotNil(t, created.ProviderTaskID)
	require.Equal(t, "imgtask_abc", *created.ProviderTaskID)
	require.NotNil(t, created.SessionID)
	require.Equal(t, int64(11), *created.SessionID)

	err = svc.SyncImageJobFromTask(context.Background(), 7, &ImageTask{
		ID:       "imgtask_abc",
		TaskID:   "imgtask_abc",
		Status:   ImageTaskStatusCompleted,
		ImageURL: "/api/v1/media/public/42",
	})
	require.NoError(t, err)

	got, err := svc.GetImage(context.Background(), 7, 1)
	require.NoError(t, err)
	require.Equal(t, CreationImageJobStatusCompleted, got.Status)
	require.NotNil(t, got.MediaAssetID)
	require.Equal(t, int64(42), *got.MediaAssetID)
	require.Equal(t, MediaPublicPath(42), got.MediaURL)

	items, total, err := svc.ListImages(context.Background(), 7, CreationImageListFilters{})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, MediaPublicPath(42), items[0].MediaURL)
}

func TestCreationService_SyncImageJobFailedStoresError(t *testing.T) {
	jobs := &creationImageJobRepoStub{}
	svc := NewCreationService(
		&creationSessionRepoStub{items: map[int64]*CreationSession{}},
		&creationMessageRepoStub{},
		jobs,
		&groupRepoStubForGroupUpdate{group: &Group{ID: 3, Status: StatusActive, Platform: PlatformOpenAI}},
		&creationTestUserRepo{},
		&userSubRepoStubForGroupUpdate{getActiveErr: ErrSubscriptionNotFound},
	)
	_, err := svc.CreateImageJob(context.Background(), CreateCreationImageJobInput{
		UserID:         7,
		GroupID:        3,
		ProviderTaskID: "imgtask_fail",
		Status:         CreationImageJobStatusProcessing,
	})
	require.NoError(t, err)

	err = svc.SyncImageJobFromTask(context.Background(), 7, &ImageTask{
		ID:     "imgtask_fail",
		TaskID: "imgtask_fail",
		Status: ImageTaskStatusFailed,
		Error:  json.RawMessage(`{"type":"api_error","message":"upstream failed"}`),
	})
	require.NoError(t, err)

	got, err := svc.GetImage(context.Background(), 7, 1)
	require.NoError(t, err)
	require.Equal(t, CreationImageJobStatusFailed, got.Status)
	require.NotNil(t, got.Error)
	require.Equal(t, "upstream failed", *got.Error)
}

func TestCreationService_SyncImageJobMissingIsNoop(t *testing.T) {
	svc := NewCreationService(
		&creationSessionRepoStub{items: map[int64]*CreationSession{}},
		&creationMessageRepoStub{},
		&creationImageJobRepoStub{},
		&groupRepoStubForGroupUpdate{group: &Group{ID: 3, Status: StatusActive, Platform: PlatformOpenAI}},
		&creationTestUserRepo{},
		&userSubRepoStubForGroupUpdate{getActiveErr: ErrSubscriptionNotFound},
	)
	err := svc.SyncImageJobFromTask(context.Background(), 7, &ImageTask{
		ID:     "imgtask_missing",
		TaskID: "imgtask_missing",
		Status: ImageTaskStatusCompleted,
	})
	require.NoError(t, err)
}

type creationTestUserRepo struct {
	userRepoStubForGroupUpdate
	user *User
}

func (s *creationTestUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	if s.user != nil {
		return s.user, nil
	}
	return &User{ID: id, Status: StatusActive}, nil
}

func TestCreationService_EnsureUserCanUseGroupRejectsInactiveOrUnauthorizedGroup(t *testing.T) {
	groupID := int64(3)
	newService := func(user *User, group *Group) *CreationService {
		return NewCreationService(
			&creationSessionRepoStub{},
			&creationMessageRepoStub{},
			&creationImageJobRepoStub{},
			&groupRepoStubForGroupUpdate{group: group},
			&creationTestUserRepo{user: user},
			&userSubRepoStubForGroupUpdate{getActiveErr: ErrSubscriptionNotFound},
		)
	}

	t.Run("disabled group", func(t *testing.T) {
		svc := newService(
			&User{ID: 7, Status: StatusActive},
			&Group{ID: groupID, Status: StatusDisabled},
		)
		require.ErrorIs(t, svc.EnsureUserCanUseGroup(context.Background(), 7, groupID), ErrCreationGroupNotAllowed)
	})

	t.Run("exclusive group", func(t *testing.T) {
		svc := newService(
			&User{ID: 7, Status: StatusActive},
			&Group{ID: groupID, Status: StatusActive, IsExclusive: true},
		)
		require.ErrorIs(t, svc.EnsureUserCanUseGroup(context.Background(), 7, groupID), ErrCreationGroupNotAllowed)
	})
}
