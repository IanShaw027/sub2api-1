//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type publicationMemoryRepo struct {
	mu                        sync.Mutex
	items                     map[int64]*CreationPublication
	createErr                 error
	commitBeforeError         bool
	activateErr               error
	lookupErr                 error
	lookupErrOnActivate       error
	commitActivateBeforeError bool
}

func (r *publicationMemoryRepo) GetByRequestID(_ context.Context, owner int64, request string) (*CreationPublication, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lookupErr != nil {
		return nil, r.lookupErr
	}
	for _, item := range r.items {
		if item.OwnerUserID == owner && item.RequestID == request {
			copy := *item
			return &copy, nil
		}
	}
	return nil, ErrCreationPublicationNotFound
}

func (r *publicationMemoryRepo) CreateIfAbsent(_ context.Context, p *CreationPublication) (*CreationPublication, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil && !r.commitBeforeError {
		return nil, false, r.createErr
	}
	for _, item := range r.items {
		if item.OwnerUserID == p.OwnerUserID && item.RequestID == p.RequestID {
			copy := *item
			return &copy, false, nil
		}
	}
	copy := *p
	copy.ID = int64(len(r.items) + 1)
	copy.CreatedAt = time.Now()
	r.items[copy.ID] = &copy
	result := copy
	return &result, true, r.createErr
}

func (r *publicationMemoryRepo) Activate(_ context.Context, id int64) (*CreationPublication, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.items[id]
	if item == nil || item.WithdrawnAt != nil {
		return nil, ErrCreationPublicationNotFound
	}
	if r.activateErr != nil {
		r.lookupErr = r.lookupErrOnActivate
		if !r.commitActivateBeforeError {
			return nil, r.activateErr
		}
	}
	item.Status = CreationPublicationPublished
	copy := *item
	return &copy, r.activateErr
}

func (r *publicationMemoryRepo) GetPublic(_ context.Context, id int64) (*CreationPublication, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if item := r.items[id]; item != nil && item.WithdrawnAt == nil && item.Status == CreationPublicationPublished {
		copy := *item
		return &copy, nil
	}
	return nil, ErrCreationPublicationNotFound
}

func (r *publicationMemoryRepo) List(_ context.Context, _ CreationPublicationFilter) ([]*CreationPublication, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []*CreationPublication{}
	for _, item := range r.items {
		if item.WithdrawnAt == nil && item.Status == CreationPublicationPublished {
			copy := *item
			items = append(items, &copy)
		}
	}
	return items, int64(len(items)), nil
}

func (r *publicationMemoryRepo) Withdraw(_ context.Context, owner, id int64) (*CreationPublication, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.items[id]
	if item == nil || item.OwnerUserID != owner {
		return nil, ErrCreationPublicationNotFound
	}
	if item.WithdrawnAt == nil {
		now := time.Now()
		item.WithdrawnAt = &now
	}
	copy := *item
	return &copy, nil
}

func publicationFixture(t *testing.T) (*CreationPublicationService, *publicationMemoryRepo, *memoryMediaStore, PublishCreationInput) {
	t.Helper()
	repo := &publicationMemoryRepo{items: map[int64]*CreationPublication{}}
	store := newMemoryMediaStore()
	svc := NewCreationPublicationService(repo, fixedMediaResolver{store: store, binding: MediaStorageBinding{ProfileID: "backup", Prefix: "media/", PublicBaseURL: "https://cdn.invalid"}})
	in := PublishCreationInput{OwnerUserID: 7, RequestID: uuid.NewString(), Title: "Published work", Prompt: "A landscape", Model: "image-model", Kind: "image", Visibility: "public", Data: mediaTestPNG(t)}
	return svc, repo, store, in
}

func TestCreationPublicationExplicitUploadAndIdempotency(t *testing.T) {
	svc, repo, store, in := publicationFixture(t)
	p, err := svc.Publish(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, repo.items, 1)
	require.Len(t, store.objects, 1)
	require.Equal(t, "/api/v1/creation/gallery/1/media", p.MediaURL)
	encoded, err := json.Marshal(p)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "storage_key")
	require.NotContains(t, string(encoded), "cdn.invalid")
	_, body, err := svc.Open(context.Background(), p.ID)
	require.NoError(t, err)
	require.Equal(t, in.Data, body)

	again, err := svc.Publish(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, p.ID, again.ID)
	require.Len(t, store.puts, 1)
	in.Title = "Different payload"
	_, err = svc.Publish(context.Background(), in)
	require.ErrorIs(t, err, ErrCreationPublicationConflict)
	_, err = svc.Status(context.Background(), 8, in.RequestID)
	require.ErrorIs(t, err, ErrCreationPublicationNotFound)
}

func TestCreationPublicationRejectsImplicitAndInvalidMedia(t *testing.T) {
	for _, mode := range []string{"private", "missing-consent", "html", "wrong-kind", "invalid-image", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			svc, repo, store, in := publicationFixture(t)
			switch mode {
			case "private":
				in.Visibility = "private"
			case "missing-consent":
				in.Visibility = ""
			case "html":
				in.Data = []byte("<!DOCTYPE html><script>alert(1)</script>")
			case "wrong-kind":
				in.Kind = "video"
			case "invalid-image":
				in.Data = []byte("\x89PNG\r\n\x1a\ninvalid")
			case "oversize":
				in.Data = make([]byte, MaxCreationPublicationBytes+1)
			}
			_, err := svc.Publish(context.Background(), in)
			require.Error(t, err)
			require.Empty(t, store.objects)
			require.Empty(t, repo.items)
		})
	}
}

func TestCreationPublicationReservesBeforeUploadAndRecoversUncertainInsert(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "failed", true: "committed-before-response-error"}[committed], func(t *testing.T) {
			svc, repo, store, in := publicationFixture(t)
			repo.createErr = context.Canceled
			repo.commitBeforeError = committed
			_, err := svc.Publish(context.Background(), in)
			require.ErrorIs(t, err, context.Canceled)
			require.Empty(t, store.objects)
			if committed {
				require.Len(t, repo.items, 1)
				require.Equal(t, CreationPublicationPending, repo.items[1].Status)
			} else {
				require.Empty(t, repo.items)
			}
			repo.createErr = nil
			p, err := svc.Publish(context.Background(), in)
			require.NoError(t, err)
			require.Equal(t, CreationPublicationPublished, p.Status)
			require.Len(t, repo.items, 1)
			require.Len(t, store.objects, 1)
		})
	}
}

func TestCreationPublicationDatabaseOutageAfterUploadResumesTrackedKey(t *testing.T) {
	svc, repo, store, in := publicationFixture(t)
	repo.activateErr = errors.New("database disconnected")
	repo.lookupErrOnActivate = repo.activateErr
	_, err := svc.Publish(context.Background(), in)
	require.Error(t, err)
	require.Len(t, repo.items, 1)
	require.Len(t, store.objects, 1)
	key := repo.items[1].StorageKey
	require.Contains(t, store.objects, key)
	repo.lookupErr, repo.activateErr = nil, nil
	pending, err := svc.Status(context.Background(), in.OwnerUserID, in.RequestID)
	require.NoError(t, err)
	require.Equal(t, CreationPublicationPending, pending.Status)
	require.Empty(t, pending.MediaURL)
	_, _, err = svc.Open(context.Background(), pending.ID)
	require.ErrorIs(t, err, ErrCreationPublicationNotFound)
	page, err := svc.List(context.Background(), CreationPublicationFilter{Page: 1, PageSize: 24})
	require.NoError(t, err)
	require.Empty(t, page.Items)
	p, err := svc.Publish(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, key, p.StorageKey)
	require.Equal(t, CreationPublicationPublished, p.Status)
	require.Len(t, repo.items, 1)
	require.Len(t, store.objects, 1)
	require.Equal(t, store.puts[0].Key, store.puts[1].Key)
}

func TestCreationPublicationPendingObjectCanBeWithdrawnAndCleanupRetried(t *testing.T) {
	svc, repo, store, in := publicationFixture(t)
	repo.activateErr = errors.New("database write failed")
	_, err := svc.Publish(context.Background(), in)
	require.Error(t, err)
	pending, err := svc.Status(context.Background(), in.OwnerUserID, in.RequestID)
	require.NoError(t, err)
	require.Equal(t, CreationPublicationPending, pending.Status)
	store.deleteErr = errors.New("object storage offline")
	require.Error(t, svc.Delete(context.Background(), in.OwnerUserID, pending.ID))
	withdrawn, err := svc.Status(context.Background(), in.OwnerUserID, in.RequestID)
	require.NoError(t, err)
	require.NotNil(t, withdrawn.WithdrawnAt)
	store.deleteErr = nil
	require.NoError(t, svc.Delete(context.Background(), in.OwnerUserID, pending.ID))
	require.Empty(t, store.objects)
}

type delayedPublicationStore struct {
	*memoryMediaStore
	started chan struct{}
	release chan struct{}
}

func (s *delayedPublicationStore) Put(ctx context.Context, key, mime string, data []byte) error {
	close(s.started)
	<-s.release
	return s.memoryMediaStore.Put(ctx, key, mime, data)
}

func TestCreationPublicationLateUploadAfterWithdrawalIsCleaned(t *testing.T) {
	svc, repo, store, in := publicationFixture(t)
	delayed := &delayedPublicationStore{memoryMediaStore: store, started: make(chan struct{}), release: make(chan struct{})}
	svc.resolver = fixedMediaResolver{store: delayed, binding: MediaStorageBinding{ProfileID: "backup", Prefix: "media/"}}
	var release sync.Once
	defer release.Do(func() { close(delayed.release) })
	result := make(chan error, 1)
	go func() { _, err := svc.Publish(context.Background(), in); result <- err }()
	select {
	case <-delayed.started:
	case <-time.After(5 * time.Second):
		t.Fatal("upload did not start")
	}
	pending, err := svc.Status(context.Background(), in.OwnerUserID, in.RequestID)
	require.NoError(t, err)
	require.NoError(t, svc.Delete(context.Background(), in.OwnerUserID, pending.ID))
	release.Do(func() { close(delayed.release) })
	require.ErrorIs(t, <-result, ErrCreationPublicationConflict)
	require.Empty(t, store.objects)
	require.NotNil(t, repo.items[pending.ID].WithdrawnAt)
}

func TestCreationPublicationAmbiguousActivationDoesNotDeletePublishedObject(t *testing.T) {
	svc, repo, store, in := publicationFixture(t)
	repo.activateErr = context.Canceled
	repo.commitActivateBeforeError = true
	p, err := svc.Publish(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, CreationPublicationPublished, p.Status)
	require.Len(t, store.objects, 1)
}

func TestCreationPublicationVideoUsesDetectedContainerMIME(t *testing.T) {
	for mime, data := range map[string][]byte{
		"video/mp4":  {0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'},
		"video/webm": {0x1a, 0x45, 0xdf, 0xa3, 0x9f, 0x42, 0x82, 0x84, 'w', 'e', 'b', 'm'},
	} {
		t.Run(mime, func(t *testing.T) {
			svc, _, _, in := publicationFixture(t)
			in.Kind, in.Data = "video", data
			p, err := svc.Publish(context.Background(), in)
			require.NoError(t, err)
			require.Equal(t, mime, p.MIME)
		})
	}
}

func TestCreationPublicationWithdrawPrecedesCleanupAndRequiresOwner(t *testing.T) {
	svc, _, store, in := publicationFixture(t)
	p, err := svc.Publish(context.Background(), in)
	require.NoError(t, err)
	require.ErrorIs(t, svc.Delete(context.Background(), 8, p.ID), ErrCreationPublicationNotFound)
	_, _, err = svc.Open(context.Background(), p.ID)
	require.NoError(t, err)
	store.deleteErr = errors.New("object storage offline")
	require.Error(t, svc.Delete(context.Background(), 7, p.ID))
	_, _, err = svc.Open(context.Background(), p.ID)
	require.ErrorIs(t, err, ErrCreationPublicationNotFound)
	page, err := svc.List(context.Background(), CreationPublicationFilter{Page: 1, PageSize: 24})
	require.NoError(t, err)
	require.Empty(t, page.Items)
	status, err := svc.Status(context.Background(), 7, in.RequestID)
	require.NoError(t, err)
	require.NotNil(t, status.WithdrawnAt)
	require.Empty(t, status.MediaURL)
	_, err = svc.Publish(context.Background(), in)
	require.ErrorIs(t, err, ErrCreationPublicationConflict)
	store.deleteErr = nil
	require.NoError(t, svc.Delete(context.Background(), 7, p.ID))
	require.Empty(t, store.objects)
}

func TestCreationPublicationConcurrentRetryHasOnePublicObject(t *testing.T) {
	svc, repo, store, in := publicationFixture(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := svc.Publish(context.Background(), in); results <- err }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.Len(t, repo.items, 1)
	require.Len(t, store.objects, 1)
}
