//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type memoryMediaStore struct {
	mu        sync.Mutex
	objects   map[string][]byte
	puts      []mediaPutRecord
	deleteErr error
}

type mediaPutRecord struct {
	Key         string
	ContentType string
	ACL         string
	SSE         string
}

func newMemoryMediaStore() *memoryMediaStore {
	return &memoryMediaStore{objects: map[string][]byte{}}
}

func mediaTestPNG(t *testing.T) []byte {
	return mediaTestPNGSize(t, 8, 8)
}

func mediaTestPNGSize(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 0x22, G: 0x66, B: 0xaa, A: 0xff})
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, img))
	return encoded.Bytes()
}

func (s *memoryMediaStore) Put(_ context.Context, key, contentType string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := append([]byte(nil), data...)
	s.objects[key] = copied
	s.puts = append(s.puts, mediaPutRecord{
		Key:         key,
		ContentType: contentType,
		ACL:         "",
		SSE:         "AES256",
	})
	return nil
}

func (s *memoryMediaStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return nil, infraerrors.NotFound("MEDIA_OBJECT_NOT_FOUND", "object not found")
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), nil
}

func (s *memoryMediaStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://s3.example.invalid/" + key + "?X-Amz-Signature=fake", nil
}

func (s *memoryMediaStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.objects, key)
	return nil
}

type memoryMediaRepo struct {
	mu     sync.Mutex
	nextID int64
	byID   map[int64]*MediaAsset
}

func newMemoryMediaRepo() *memoryMediaRepo {
	return &memoryMediaRepo{nextID: 1, byID: map[int64]*MediaAsset{}}
}

func (r *memoryMediaRepo) Create(_ context.Context, asset *MediaAsset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *asset
	cloned.ID = r.nextID
	r.nextID++
	r.byID[cloned.ID] = &cloned
	*asset = cloned
	return nil
}

func (r *memoryMediaRepo) GetByID(_ context.Context, id int64) (*MediaAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	asset, ok := r.byID[id]
	if !ok {
		return nil, infraerrors.NotFound("MEDIA_NOT_FOUND", "media asset not found")
	}
	cloned := *asset
	return &cloned, nil
}

func (r *memoryMediaRepo) UpdateVisibility(_ context.Context, id int64, visibility, publicBaseURL string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	asset, ok := r.byID[id]
	if !ok {
		return ErrMediaNotFound
	}
	asset.Visibility = visibility
	asset.PublicBaseURL = publicBaseURL
	return nil
}

func (r *memoryMediaRepo) MarkDeleted(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	asset, ok := r.byID[id]
	if !ok {
		return ErrMediaNotFound
	}
	asset.Status = MediaStatusDeleted
	return nil
}

func (r *memoryMediaRepo) ListByBiz(_ context.Context, ownerUserID int64, bizType, bizID string) ([]MediaAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]MediaAsset, 0)
	for _, asset := range r.byID {
		if asset.OwnerUserID == ownerUserID && asset.BizType == bizType && asset.BizID == bizID {
			out = append(out, *asset)
		}
	}
	return out, nil
}

type fixedMediaResolver struct {
	store   MediaObjectStore
	binding MediaStorageBinding
}

func (r fixedMediaResolver) Resolve(_ context.Context) (*MediaStorageBinding, MediaObjectStore, error) {
	binding := r.binding
	return &binding, r.store, nil
}

func newTestMediaService(t *testing.T) (*MediaService, *memoryMediaStore, *memoryMediaRepo) {
	t.Helper()
	store := newMemoryMediaStore()
	repo := newMemoryMediaRepo()
	svc := NewMediaService(repo, fixedMediaResolver{
		store: store,
		binding: MediaStorageBinding{
			ProfileID:     "backup",
			Prefix:        "media",
			PublicBaseURL: "https://cdn.example.test/media",
		},
	}, []byte("test-media-hmac-secret"))
	return svc, store, repo
}

func TestMediaUploadForcesInvoiceAndTicketPrivate(t *testing.T) {
	svc, store, _ := newTestMediaService(t)
	ctx := context.Background()

	for _, biz := range []string{MediaBizInvoice, MediaBizTicket} {
		asset, err := svc.Upload(ctx, UploadMediaInput{
			OwnerUserID:  11,
			BizType:      biz,
			BizID:        "42",
			Filename:     "file.pdf",
			Visibility:   MediaVisibilityPublic,
			ActorIsAdmin: biz == MediaBizInvoice,
			Data:         []byte("%PDF-1.4 fake"),
		})
		require.NoError(t, err, biz)
		require.Equal(t, MediaVisibilityPrivate, asset.Visibility, biz)
		require.Empty(t, asset.PublicBaseURL, biz)
		require.NotContains(t, asset.AccessURL, "s3.example", biz)
		require.NotContains(t, asset.AccessURL, "cdn.example.test", biz)
		require.False(t, strings.Contains(asset.AccessURL, "/media/public/"), biz)
	}
	require.NotEmpty(t, store.puts)
	for _, put := range store.puts {
		require.Empty(t, put.ACL)
		require.Equal(t, "AES256", put.SSE)
	}
}

func TestMediaPublicAccessUsesGatewayNotBucketACL(t *testing.T) {
	svc, store, _ := newTestMediaService(t)
	imageData := mediaTestPNG(t)

	asset, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID: 7,
		BizType:     MediaBizAvatar,
		BizID:       "7",
		Filename:    "avatar.png",
		Visibility:  MediaVisibilityPublic,
		Data:        imageData,
	})
	require.NoError(t, err)
	require.Equal(t, MediaVisibilityPublic, asset.Visibility)
	require.True(t, strings.HasPrefix(asset.AccessURL, "https://cdn.example.test/media/"))
	require.NotContains(t, asset.AccessURL, "s3.example")
	require.Empty(t, store.puts[0].ACL)

	opened, body, err := svc.OpenPublic(context.Background(), asset.ID)
	require.NoError(t, err)
	require.Equal(t, asset.ID, opened.ID)
	require.Equal(t, imageData, body)
}

func TestMediaOpenPublicRejectsPrivateAsset(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	asset, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  7,
		BizType:      MediaBizInvoice,
		BizID:        "9",
		Filename:     "invoice.pdf",
		Visibility:   MediaVisibilityPrivate,
		ActorIsAdmin: true,
		Data:         []byte("%PDF-1.4 invoice"),
	})
	require.NoError(t, err)

	_, _, err = svc.OpenPublic(context.Background(), asset.ID)
	require.Error(t, err)
	require.True(t, infraerrors.IsNotFound(err) || infraerrors.IsForbidden(err))
}

func TestMediaPrivateExpiredSignatureRejected(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	svc.now = func() time.Time { return time.Unix(1_700_000_000, 0) }

	asset, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID: 3,
		BizType:     MediaBizTicket,
		BizID:       "1",
		Filename:    "shot.png",
		Visibility:  MediaVisibilityPrivate,
		Data:        []byte("%PDF-1.4 ticket"),
	})
	require.NoError(t, err)

	grant, err := svc.CreateDownloadGrant(context.Background(), CreateDownloadGrantInput{
		AssetID:      asset.ID,
		ActorUserID:  3,
		ActorIsAdmin: false,
		TTLMinutes:   15,
	})
	require.NoError(t, err)
	require.Contains(t, grant.URL, "/api/v1/media/download/")
	require.NotContains(t, grant.URL, "s3.example")

	svc.now = func() time.Time { return time.Unix(1_700_000_000, 0).Add(16 * time.Minute) }
	_, _, err = svc.OpenSignedDownload(context.Background(), OpenSignedDownloadInput{
		AssetID: asset.ID,
		Expires: grant.ExpiresAt,
		Sig:     grant.Sig,
	})
	require.Error(t, err)
	require.True(t, infraerrors.IsForbidden(err) || infraerrors.IsUnauthorized(err) || infraerrors.IsBadRequest(err))
}

func TestMediaInvoiceEmailGrantAllows24hAndOpenForActorDoesNotExpire(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	svc.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	asset, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  8,
		BizType:      MediaBizInvoice,
		BizID:        "12",
		Filename:     "invoice.pdf",
		Visibility:   MediaVisibilityPrivate,
		ActorIsAdmin: true,
		Data:         []byte("%PDF-1.4 invoice"),
	})
	require.NoError(t, err)

	clamped, err := svc.CreateDownloadGrant(context.Background(), CreateDownloadGrantInput{
		AssetID:     asset.ID,
		ActorUserID: 8,
		TTLMinutes:  InvoiceEmailDownloadTTLMinutes,
	})
	require.NoError(t, err)
	require.Equal(t, MaxMediaTTLMinutes, clamped.TTL)

	grant, err := svc.CreateDownloadGrant(context.Background(), CreateDownloadGrantInput{
		AssetID:       asset.ID,
		ActorUserID:   8,
		TTLMinutes:    InvoiceEmailDownloadTTLMinutes,
		MaxTTLMinutes: InvoiceEmailDownloadTTLMinutes,
	})
	require.NoError(t, err)
	require.Equal(t, InvoiceEmailDownloadTTLMinutes, grant.TTL)
	require.Equal(t, "https://api.example.com/api/v1/media/download/1?expires=1&sig=x", AbsoluteMediaURL("https://api.example.com", "/api/v1/media/download/1?expires=1&sig=x"))

	svc.now = func() time.Time { return time.Unix(1_700_000_000, 0).Add(48 * time.Hour) }
	opened, rc, err := svc.OpenForActor(context.Background(), OpenForActorInput{
		AssetID:     asset.ID,
		ActorUserID: 8,
	})
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	require.Equal(t, asset.ID, opened.ID)
	body, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, []byte("%PDF-1.4 invoice"), body)
}

func TestMediaCrossUserDownloadForbidden(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	asset, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  3,
		BizType:      MediaBizInvoice,
		BizID:        "88",
		Filename:     "tax.pdf",
		Visibility:   MediaVisibilityPrivate,
		ActorIsAdmin: true,
		Data:         []byte("%PDF-1.4 secret"),
	})
	require.NoError(t, err)

	_, err = svc.CreateDownloadGrant(context.Background(), CreateDownloadGrantInput{
		AssetID:      asset.ID,
		ActorUserID:  99,
		ActorIsAdmin: false,
		TTLMinutes:   15,
	})
	require.Error(t, err)
	require.True(t, infraerrors.IsForbidden(err))

	_, err = svc.CreateDownloadGrant(context.Background(), CreateDownloadGrantInput{
		AssetID:      asset.ID,
		ActorUserID:  1,
		ActorIsAdmin: true,
		TTLMinutes:   15,
	})
	require.NoError(t, err)

	_, err = svc.CreateDownloadGrant(context.Background(), CreateDownloadGrantInput{
		AssetID:        asset.ID,
		ActorUserID:    99,
		VerifiedAccess: true,
		TTLMinutes:     15,
	})
	require.NoError(t, err)
}

func TestMediaUploadRejectsOversize(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	_, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID: 1,
		BizType:     MediaBizAvatar,
		Filename:    "huge.bin",
		Visibility:  MediaVisibilityPrivate,
		Data:        bytes.Repeat([]byte("a"), MaxMediaUploadBytes+1),
	})
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestMediaDownloadGrantTTLClamped(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	svc.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	asset, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID: 3,
		BizType:     MediaBizAvatar,
		Filename:    "a.png",
		Visibility:  MediaVisibilityPrivate,
		Data:        mediaTestPNG(t),
	})
	require.NoError(t, err)

	tooLong, err := svc.CreateDownloadGrant(context.Background(), CreateDownloadGrantInput{
		AssetID:      asset.ID,
		ActorUserID:  3,
		ActorIsAdmin: false,
		TTLMinutes:   120,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1_700_000_000+60*60), tooLong.ExpiresAt)

	zero, err := svc.CreateDownloadGrant(context.Background(), CreateDownloadGrantInput{
		AssetID:      asset.ID,
		ActorUserID:  3,
		ActorIsAdmin: false,
		TTLMinutes:   0,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1_700_000_000+15*60), zero.ExpiresAt)
}

func TestMediaUploadRejectsPublicHTML(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	_, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID: 7,
		BizType:     MediaBizAvatar,
		Filename:    "attack.html",
		Visibility:  MediaVisibilityPublic,
		Data:        []byte("<!DOCTYPE html><html><body>xss</body></html>"),
	})
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestMediaUploadRejectsCorruptAndOversizedDimensionImages(t *testing.T) {
	svc, store, _ := newTestMediaService(t)
	input := UploadMediaInput{
		OwnerUserID:  7,
		BizType:      MediaBizAnnouncement,
		Filename:     "notice.png",
		Visibility:   MediaVisibilityPublic,
		ActorIsAdmin: true,
		Data:         []byte("\x89PNG\r\n\x1a\n"),
	}

	_, err := svc.Upload(context.Background(), input)
	require.ErrorIs(t, err, ErrMediaUnsupportedType)
	require.Empty(t, store.puts)

	input.Data = mediaTestPNGSize(t, MaxMediaImageDimension+1, 1)
	_, err = svc.Upload(context.Background(), input)
	require.Error(t, err)
	require.Equal(t, "MEDIA_IMAGE_DIMENSIONS_INVALID", infraerrors.Reason(err))
	require.Empty(t, store.puts)
}

func TestMediaUploadRejectsHTMLDisguisedAsPDF(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	_, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  3,
		BizType:      MediaBizInvoice,
		Filename:     "invoice.pdf",
		Visibility:   MediaVisibilityPrivate,
		ActorIsAdmin: true,
		Data:         []byte("<html><script>alert(1)</script></html>"),
	})
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestMediaUploadPreservesOriginalFilename(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	asset, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  3,
		BizType:      MediaBizInvoice,
		BizID:        "99",
		Filename:     "March Invoice.pdf",
		Visibility:   MediaVisibilityPublic,
		ActorIsAdmin: true,
		Data:         []byte("%PDF-1.4 fake"),
	})
	require.NoError(t, err)
	require.Equal(t, MediaVisibilityPrivate, asset.Visibility)
	require.Equal(t, "March Invoice.pdf", asset.Filename)
}

type failingMediaRepo struct {
	memoryMediaRepo
}

func (r *failingMediaRepo) Create(context.Context, *MediaAsset) error {
	return infraerrors.InternalServer("MEDIA_DB", "create failed")
}

func TestMediaUploadDeletesObjectWhenCreateFails(t *testing.T) {
	store := newMemoryMediaStore()
	repo := &failingMediaRepo{}
	svc := NewMediaService(repo, fixedMediaResolver{
		store:   store,
		binding: MediaStorageBinding{ProfileID: "backup", Prefix: "media"},
	}, []byte("test-media-hmac-secret"))

	_, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  1,
		BizType:      MediaBizInvoice,
		Filename:     "a.pdf",
		ActorIsAdmin: true,
		Data:         []byte("%PDF-1.4"),
	})
	require.Error(t, err)
	require.Empty(t, store.objects)
}

func TestMediaUploadReportsObjectCleanupFailureWhenCreateFails(t *testing.T) {
	store := newMemoryMediaStore()
	store.deleteErr = errors.New("s3 cleanup failed")
	repo := &failingMediaRepo{}
	svc := NewMediaService(repo, fixedMediaResolver{
		store:   store,
		binding: MediaStorageBinding{ProfileID: "backup", Prefix: "media"},
	}, []byte("test-media-hmac-secret"))

	_, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  1,
		BizType:      MediaBizInvoice,
		Filename:     "a.pdf",
		ActorIsAdmin: true,
		Data:         []byte("%PDF-1.4"),
	})
	require.ErrorContains(t, err, "create failed")
	require.ErrorContains(t, err, "clean up media object")
	require.Len(t, store.objects, 1)
}

func TestMediaDeleteKeepsAssetReadyWhenObjectDeleteFails(t *testing.T) {
	svc, store, repo := newTestMediaService(t)
	asset, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  7,
		BizType:      MediaBizSiteLogo,
		Filename:     "site-logo.png",
		Visibility:   MediaVisibilityPublic,
		ActorIsAdmin: true,
		Data:         mediaTestPNG(t),
	})
	require.NoError(t, err)
	store.deleteErr = errors.New("s3 delete failed")

	err = svc.Delete(context.Background(), asset.ID, 7, true)
	require.ErrorContains(t, err, "delete media object")
	stored, getErr := repo.GetByID(context.Background(), asset.ID)
	require.NoError(t, getErr)
	require.Equal(t, MediaStatusReady, stored.Status)
	require.Contains(t, store.objects, asset.StorageKey)
}

func TestMediaUploadAnnouncementRequiresAdminAndAllowsPublic(t *testing.T) {
	svc, store, _ := newTestMediaService(t)
	_, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  3,
		BizType:      MediaBizAnnouncement,
		Filename:     "notice.png",
		Visibility:   MediaVisibilityPublic,
		ActorIsAdmin: false,
		Data:         mediaTestPNG(t),
	})
	require.Error(t, err)
	require.Equal(t, "MEDIA_ADMIN_ONLY", infraerrors.Reason(err))

	asset, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  3,
		BizType:      MediaBizAnnouncement,
		Filename:     "notice.png",
		Visibility:   MediaVisibilityPublic,
		ActorIsAdmin: true,
		Data:         mediaTestPNG(t),
	})
	require.NoError(t, err)
	require.Equal(t, MediaVisibilityPublic, asset.Visibility)
	require.NotEmpty(t, store.puts)
}

func TestMediaUploadSettingsImagesRequireAdminAndAllowPublic(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	for _, bizType := range []string{MediaBizSiteLogo, MediaBizSupportQR, MediaBizPaymentHelp} {
		input := UploadMediaInput{
			OwnerUserID: 7,
			BizType:     bizType,
			Filename:    "settings-image.png",
			Visibility:  MediaVisibilityPublic,
			Data:        mediaTestPNG(t),
		}

		_, err := svc.Upload(context.Background(), input)
		require.Error(t, err)
		require.Equal(t, "MEDIA_ADMIN_ONLY", infraerrors.Reason(err))

		input.ActorIsAdmin = true
		asset, err := svc.Upload(context.Background(), input)
		require.NoError(t, err)
		require.Equal(t, bizType, asset.BizType)
		require.Equal(t, MediaVisibilityPublic, asset.Visibility)
	}
}

func TestMediaUploadInvoiceRequiresAdmin(t *testing.T) {
	svc, _, _ := newTestMediaService(t)
	_, err := svc.Upload(context.Background(), UploadMediaInput{
		OwnerUserID:  3,
		BizType:      MediaBizInvoice,
		Filename:     "invoice.pdf",
		Visibility:   MediaVisibilityPrivate,
		ActorIsAdmin: false,
		Data:         []byte("%PDF-1.4 invoice"),
	})
	require.Error(t, err)
	require.Equal(t, "MEDIA_ADMIN_ONLY", infraerrors.Reason(err))
}
