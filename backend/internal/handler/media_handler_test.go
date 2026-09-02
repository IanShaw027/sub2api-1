//go:build unit

package handler

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func handlerTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{R: 0x22, G: 0x66, B: 0xaa, A: 0xff})
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, img))
	return encoded.Bytes()
}

type handlerMediaStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *handlerMediaStore) Put(_ context.Context, key, _ string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.objects == nil {
		s.objects = map[string][]byte{}
	}
	s.objects[key] = append([]byte(nil), data...)
	return nil
}

func (s *handlerMediaStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.objects[key]
	return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), nil
}

func (s *handlerMediaStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://s3.example.invalid/" + key, nil
}

func (s *handlerMediaStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

type handlerMediaRepo struct {
	mu     sync.Mutex
	nextID int64
	byID   map[int64]*service.MediaAsset
}

func (r *handlerMediaRepo) Create(_ context.Context, asset *service.MediaAsset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byID == nil {
		r.byID = map[int64]*service.MediaAsset{}
		r.nextID = 1
	}
	cloned := *asset
	cloned.ID = r.nextID
	r.nextID++
	r.byID[cloned.ID] = &cloned
	*asset = cloned
	return nil
}

func (r *handlerMediaRepo) GetByID(_ context.Context, id int64) (*service.MediaAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	asset := r.byID[id]
	cloned := *asset
	return &cloned, nil
}

func (r *handlerMediaRepo) ListByBiz(_ context.Context, _ int64, _, _ string) ([]service.MediaAsset, error) {
	return nil, nil
}

func (r *handlerMediaRepo) UpdateVisibility(_ context.Context, _ int64, _, _ string) error {
	return nil
}

func (r *handlerMediaRepo) MarkDeleted(_ context.Context, _ int64) error {
	return nil
}

type handlerMediaResolver struct {
	store service.MediaObjectStore
}

func (r handlerMediaResolver) Resolve(_ context.Context) (*service.MediaStorageBinding, service.MediaObjectStore, error) {
	return &service.MediaStorageBinding{ProfileID: "backup", Prefix: "media"}, r.store, nil
}

func TestMediaPublicGetSetsCacheControlAndDoesNotExposeS3(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &handlerMediaStore{}
	repo := &handlerMediaRepo{}
	svc := service.NewMediaService(repo, handlerMediaResolver{store: store}, []byte("handler-secret"))
	h := NewMediaHandler(svc)

	asset, err := svc.Upload(context.Background(), service.UploadMediaInput{
		OwnerUserID: 1,
		BizType:     service.MediaBizAvatar,
		Filename:    "a.png",
		Visibility:  service.MediaVisibilityPublic,
		Data:        handlerTestPNG(t),
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, service.MediaPublicPath(asset.ID), nil)
	c.Params = []gin.Param{{Key: "id", Value: strconv.FormatInt(asset.ID, 10)}}
	h.PublicGet(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "public, max-age=300", w.Header().Get("Cache-Control"))
	require.Equal(t, "sandbox", w.Header().Get("Content-Security-Policy"))
	require.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	require.NotContains(t, w.Body.String(), "s3.example.invalid")
}

func TestMediaPresignDownloadForbiddenForOtherUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &handlerMediaStore{}
	repo := &handlerMediaRepo{}
	svc := service.NewMediaService(repo, handlerMediaResolver{store: store}, []byte("handler-secret"))
	h := NewMediaHandler(svc)

	asset, err := svc.Upload(context.Background(), service.UploadMediaInput{
		OwnerUserID: 3,
		BizType:     service.MediaBizTicket,
		Filename:    "a.pdf",
		Visibility:  service.MediaVisibilityPrivate,
		Data:        []byte("%PDF-1.4"),
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/"+strconv.FormatInt(asset.ID, 10)+"/presign-download", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = []gin.Param{{Key: "id", Value: strconv.FormatInt(asset.ID, 10)}}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
	h.PresignDownload(c)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.NotContains(t, w.Body.String(), "s3.example.invalid")
}
