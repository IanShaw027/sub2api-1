//go:build unit

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

type renewableImageStorage struct{ fakeImageStorage }

func (*renewableImageStorage) StorageID() string { return "bucket-a" }
func (*renewableImageStorage) URL(_ context.Context, key string) (string, error) {
	return "https://cdn.test/" + key + "?renewed=true", nil
}

func TestCreationImageCompletionSurvivesRedisExpiry(t *testing.T) {
	ctx := context.Background()
	jobs := &creationImageJobRepoStub{}
	creation := NewCreationService(nil, nil, jobs, nil, nil, nil)
	store := &imageTaskMemoryStore{}
	uploader := NewImageResultUploader(&renewableImageStorage{}, "images/", 0, nil)
	tasks := NewImageTaskServiceWithUploader(store, uploader, time.Hour, time.Minute)
	task, err := tasks.Create(ctx, ImageTaskOwner{UserID: 7, APIKeyID: 9})
	require.NoError(t, err)
	job, err := creation.CreateImageJob(ctx, CreateCreationImageJobInput{
		UserID: 7, GroupID: 3, ProviderTaskID: task.ID,
	})
	require.NoError(t, err)
	err = tasks.Complete(ctx, task.ID, http.StatusOK, json.RawMessage(`{"data":[{"b64_json":"iVBORw0KGgo="}]}`), func(ctx context.Context, result *ImageTask) error {
		return creation.SyncImageJobFromTask(ctx, 7, result)
	})
	require.NoError(t, err)
	store.task = nil
	got, err := creation.GetImage(ctx, 7, job.ID)
	require.NoError(t, err)
	require.Equal(t, CreationImageJobStatusCompleted, got.Status)
	require.Equal(t, "bucket-a", got.StorageID)
	require.Contains(t, got.StorageKey, task.ID)
	require.NotEmpty(t, got.MediaURL)
	renewed, err := tasks.ResolveStorageURL(ctx, ImageStorageReference{StorageID: got.StorageID, Key: got.StorageKey})
	require.NoError(t, err)
	require.Contains(t, renewed, "renewed=true")
	_, err = tasks.ResolveStorageURL(ctx, ImageStorageReference{StorageID: "different-bucket", Key: got.StorageKey})
	require.ErrorIs(t, err, ErrImageTaskUnavailable)
	_, err = creation.GetImage(ctx, 8, job.ID)
	require.ErrorIs(t, err, ErrCreationImageNotFound)
	require.NoError(t, creation.SyncImageJobFromTask(ctx, 7, task))
	got, err = creation.GetImage(ctx, 7, job.ID)
	require.NoError(t, err)
	require.Equal(t, CreationImageJobStatusCompleted, got.Status, "stale processing polls cannot regress a completed job")
}

func TestCreationImageCompletionPersistsBeforeRedisWrite(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "failed"}[failed], func(t *testing.T) {
			ctx := context.Background()
			store := &imageTaskMemoryStore{}
			tasks := NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
			task, err := tasks.Create(ctx, ImageTaskOwner{UserID: 7, APIKeyID: 9})
			require.NoError(t, err)
			store.saveErr = errors.New("redis unavailable")
			var persisted *ImageTask
			observer := func(_ context.Context, result *ImageTask) error { persisted = result; return nil }
			if failed {
				err = tasks.Fail(ctx, task.ID, http.StatusBadGateway, json.RawMessage(`{"message":"upstream failed"}`), observer)
			} else {
				err = tasks.Complete(ctx, task.ID, http.StatusOK, json.RawMessage(`{"data":[{"url":"https://cdn.test/result.png"}]}`), observer)
			}
			require.Error(t, err)
			require.NotNil(t, persisted)
			require.NotEqual(t, ImageTaskStatusProcessing, persisted.Status)
		})
	}
}

func TestCreationImageCompletionPersistsWhenRedisReadFails(t *testing.T) {
	store := &imageTaskMemoryStore{getErr: errors.New("redis unavailable")}
	tasks := NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	var persisted *ImageTask
	err := tasks.Complete(context.Background(), "imgtask_missing", http.StatusOK,
		json.RawMessage(`{"data":[{"url":"https://cdn.test/result.png"}]}`),
		func(_ context.Context, task *ImageTask) error { persisted = task; return nil })
	require.ErrorIs(t, err, ErrImageTaskUnavailable)
	require.NotNil(t, persisted)
	require.Equal(t, ImageTaskStatusCompleted, persisted.Status)
	require.Equal(t, "imgtask_missing", persisted.TaskID)
	require.Equal(t, "https://cdn.test/result.png", persisted.ImageURL)
	require.Nil(t, store.task, "do not recreate a public Redis task without its original ownership")
}
