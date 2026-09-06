package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageTaskMemoryStore struct {
	task    *ImageTaskRecord
	ttl     time.Duration
	saveErr error
	getErr  error
}

func (s *imageTaskMemoryStore) FinishIfProcessing(ctx context.Context, task *ImageTaskRecord, ttl time.Duration) (*ImageTaskRecord, error) {
	current, err := s.Get(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	if current.Status != ImageTaskStatusProcessing {
		return current, nil
	}
	if err := s.Save(ctx, task, ttl); err != nil {
		return nil, err
	}
	return s.Get(ctx, task.ID)
}

func (s *imageTaskMemoryStore) Save(_ context.Context, task *ImageTaskRecord, ttl time.Duration) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	copy := *task
	s.task = &copy
	s.ttl = ttl
	return nil
}

func (s *imageTaskMemoryStore) Get(_ context.Context, _ string) (*ImageTaskRecord, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.task == nil {
		return nil, ErrImageTaskNotFound
	}
	copy := *s.task
	return &copy, nil
}

func TestImageTaskServiceLifecycleAndOwnership(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, 10*time.Minute)
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}

	created, err := svc.Create(context.Background(), owner)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusProcessing, created.Status)
	require.Equal(t, created.ID, created.TaskID)
	require.Equal(t, "image.generation.task", created.Object)
	require.Equal(t, time.Hour, store.ttl)
	require.Equal(t, owner.UserID, store.task.UserID)
	require.Equal(t, owner.APIKeyID, store.task.APIKeyID)

	_, err = svc.Get(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 10}, created.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)

	result := json.RawMessage(`{"created":123,"data":[{"url":"https://example.test/image.png"}]}`)
	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK, result))

	completed, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusCompleted, completed.Status)
	require.Equal(t, http.StatusOK, completed.HTTPStatus)
	require.Equal(t, "https://example.test/image.png", completed.ImageURL)
	require.JSONEq(t, string(result), string(completed.Result))
	require.NotNil(t, completed.CompletedAt)

	byUser, err := svc.GetByIDForUser(context.Background(), 7, created.ID)
	require.NoError(t, err)
	require.Equal(t, "https://example.test/image.png", byUser.ImageURL)

	_, err = svc.GetByIDForUser(context.Background(), 8, created.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)
}

func TestImageTaskServiceInvalidResultBecomesFailed(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)
	created, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
	require.NoError(t, err)

	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK, json.RawMessage(`not-json`)))
	got, err := svc.Get(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2}, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status)
	require.Equal(t, http.StatusBadGateway, got.HTTPStatus)
	require.Contains(t, string(got.Error), "non-JSON")
}

func TestImageTaskServiceMapsStoreFailures(t *testing.T) {
	store := &imageTaskMemoryStore{saveErr: errors.New("redis down")}
	svc := NewImageTaskService(store)

	_, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
	require.ErrorIs(t, err, ErrImageTaskUnavailable)
}

func TestImageTaskServiceReconcilesExpiredProcessingAfterRestart(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "persisted deadline", true: "legacy record"}[legacy], func(t *testing.T) {
			store := &imageTaskMemoryStore{}
			owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}
			beforeRestart := NewImageTaskService(store)
			created, err := beforeRestart.Create(context.Background(), owner)
			require.NoError(t, err)
			store.task.CreatedAt = time.Now().Add(-time.Hour).Unix()
			store.task.ProcessingDeadlineAt = time.Now().Add(-time.Second).Unix()
			if legacy {
				store.task.ProcessingDeadlineAt = 0
			}
			restarted := NewImageTaskService(store)
			_, err = restarted.Get(context.Background(), ImageTaskOwner{UserID: 8, APIKeyID: 9}, created.ID)
			require.ErrorIs(t, err, ErrImageTaskNotFound)
			require.Equal(t, ImageTaskStatusProcessing, store.task.Status, "unauthorized polling must not mutate tasks")

			var got *ImageTask
			if legacy {
				got, err = restarted.Get(context.Background(), owner, created.ID)
			} else {
				got, err = restarted.GetByIDForUser(context.Background(), owner.UserID, created.ID)
			}
			require.NoError(t, err)
			require.Equal(t, ImageTaskStatusFailed, got.Status)
			require.Equal(t, http.StatusGatewayTimeout, got.HTTPStatus)
			require.Contains(t, string(got.Error), "timeout_error")
			require.NotNil(t, got.CompletedAt)
			require.Equal(t, defaultImageTaskTTL, store.ttl)

			var observed *ImageTask
			err = restarted.Complete(context.Background(), created.ID, http.StatusOK,
				json.RawMessage(`{"data":[{"url":"https://cdn.test/late.png"}]}`),
				func(_ context.Context, task *ImageTask) error { observed = task; return nil })
			require.NoError(t, err)
			require.Equal(t, ImageTaskStatusFailed, observed.Status)
			got, err = restarted.Get(context.Background(), owner, created.ID)
			require.NoError(t, err)
			require.Equal(t, ImageTaskStatusFailed, got.Status)
		})
	}
}

func TestImageTaskServiceAllowsObjectStorageGraceAfterGeneration(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskService(store)
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}
	created, err := svc.Create(context.Background(), owner)
	require.NoError(t, err)
	store.task.CreatedAt = time.Now().Add(-defaultImageTaskExecutionTimeout - time.Minute).Unix()
	store.task.ProcessingDeadlineAt = svc.ProcessingDeadline(time.Unix(store.task.CreatedAt, 0)).Unix()
	// A restart with a shorter configured timeout cannot shorten an existing task's lease.
	svc = NewImageTaskServiceWithOptions(store, defaultImageTaskTTL, time.Second)

	got, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusProcessing, got.Status)

	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK,
		json.RawMessage(`{"data":[{"url":"https://cdn.test/on-time.png"}]}`)))
	got, err = svc.GetByIDForUser(context.Background(), owner.UserID, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusCompleted, got.Status)

	require.NoError(t, svc.Fail(context.Background(), created.ID, http.StatusGatewayTimeout,
		imageTaskErrorJSON("timeout_error", "late failure")))
	require.Equal(t, ImageTaskStatusCompleted, store.task.Status)
}

type deadlineCheckingImageStorage struct {
	deadline time.Time
}

func (s *deadlineCheckingImageStorage) Save(ctx context.Context, _, _ string, _ []byte) (string, error) {
	s.deadline, _ = ctx.Deadline()
	return "https://cdn.test/image.png", nil
}

func TestImageTaskServiceBoundsBackgroundObjectStorageContext(t *testing.T) {
	store := &imageTaskMemoryStore{}
	storage := &deadlineCheckingImageStorage{}
	svc := NewImageTaskServiceWithUploader(store, NewImageResultUploader(storage, "images/", 0, nil), time.Hour, time.Minute)
	created, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 9})
	require.NoError(t, err)
	store.task.ProcessingDeadlineAt = time.Now().Add(time.Minute).Unix()
	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK,
		json.RawMessage(`{"data":[{"b64_json":"iVBORw0KGgo="}]}`)))
	require.Equal(t, time.Unix(store.task.ProcessingDeadlineAt, 0), storage.deadline)
}

type imageTaskCompletionRaceStore struct {
	imageTaskMemoryStore
}

func (s *imageTaskCompletionRaceStore) FinishIfProcessing(_ context.Context, _ *ImageTaskRecord, _ time.Duration) (*ImageTaskRecord, error) {
	completedAt := time.Now().Unix()
	s.task.Status = ImageTaskStatusCompleted
	s.task.HTTPStatus = http.StatusOK
	s.task.Result = json.RawMessage(`{"data":[{"url":"https://cdn.test/winner.png"}]}`)
	s.task.CompletedAt = &completedAt
	return s.Get(context.Background(), s.task.ID)
}

func TestImageTaskServiceExpirationDoesNotOverwriteConcurrentCompletion(t *testing.T) {
	store := &imageTaskCompletionRaceStore{}
	svc := NewImageTaskService(store)
	created, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 9})
	require.NoError(t, err)
	store.task.ProcessingDeadlineAt = time.Now().Add(-time.Second).Unix()
	got, err := svc.GetByIDForUser(context.Background(), 7, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusCompleted, got.Status)
	require.Equal(t, "https://cdn.test/winner.png", got.ImageURL)
}
