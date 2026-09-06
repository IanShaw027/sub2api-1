//go:build unit

package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type publicationHandlerRepo struct {
	service.CreationPublicationRepository
	p *service.CreationPublication
}

func (r *publicationHandlerRepo) GetByRequestID(context.Context, int64, string) (*service.CreationPublication, error) {
	return nil, service.ErrCreationPublicationNotFound
}

func (r *publicationHandlerRepo) CreateIfAbsent(_ context.Context, p *service.CreationPublication) (*service.CreationPublication, bool, error) {
	copy := *p
	copy.ID, copy.CreatedAt = 3, time.Now()
	r.p = &copy
	return &copy, true, nil
}

func (r *publicationHandlerRepo) GetPublic(_ context.Context, id int64) (*service.CreationPublication, error) {
	if r.p == nil || r.p.ID != id || r.p.WithdrawnAt != nil || r.p.Status != service.CreationPublicationPublished {
		return nil, service.ErrCreationPublicationNotFound
	}
	copy := *r.p
	return &copy, nil
}

func (r *publicationHandlerRepo) Activate(_ context.Context, id int64) (*service.CreationPublication, error) {
	if r.p == nil || r.p.ID != id || r.p.WithdrawnAt != nil {
		return nil, service.ErrCreationPublicationNotFound
	}
	r.p.Status = service.CreationPublicationPublished
	copy := *r.p
	return &copy, nil
}

func TestCreationPublicationUploadRequiresAuthAndExplicitPublicIntent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, authorized := range []bool{false, true} {
		for _, consent := range []string{"", "public"} {
			t.Run(consent+map[bool]string{true: "-authenticated", false: "-anonymous"}[authorized], func(t *testing.T) {
				repo := &publicationHandlerRepo{}
				store := &handlerMediaStore{}
				h := NewCreationPublicationHandler(service.NewCreationPublicationService(repo, handlerMediaResolver{store: store}))
				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				for key, value := range map[string]string{"request_id": uuid.NewString(), "title": "Title", "kind": "image", "visibility": consent, "owner_user_id": "999"} {
					require.NoError(t, writer.WriteField(key, value))
				}
				part, err := writer.CreateFormFile("file", "untrusted-name.html")
				require.NoError(t, err)
				_, err = part.Write(handlerTestPNG(t))
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/creation/publications", &body)
				c.Request.Header.Set("Content-Type", writer.FormDataContentType())
				if authorized {
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
				}
				h.Upload(c)
				if !authorized {
					require.Equal(t, http.StatusUnauthorized, w.Code)
				} else if consent == "" {
					require.Equal(t, http.StatusBadRequest, w.Code)
				} else {
					require.Equal(t, http.StatusOK, w.Code)
					require.Equal(t, int64(7), repo.p.OwnerUserID)
					require.Equal(t, "image/png", repo.p.MIME)
					require.NotContains(t, w.Body.String(), "storage_key")
				}
			})
		}
	}
}

func TestCreationPublicationMediaRangesAndRevocation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &handlerMediaStore{}
	repo := &publicationHandlerRepo{p: &service.CreationPublication{ID: 3, Status: service.CreationPublicationPublished, MIME: "video/mp4", Size: 10, StorageKey: "private/video.mp4", StorageProfileID: "backup", CreatedAt: time.Now()}}
	require.NoError(t, store.Put(context.Background(), "private/video.mp4", "video/mp4", []byte("0123456789")))
	h := NewCreationPublicationHandler(service.NewCreationPublicationService(repo, handlerMediaResolver{store: store}))
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/creation/gallery/3/media", nil)
		c.Request.Header.Set("Range", "bytes=2-5")
		c.Params = gin.Params{{Key: "id", Value: "3"}}
		h.Media(c)
		return w
	}
	w := request()
	require.Equal(t, http.StatusPartialContent, w.Code)
	require.Equal(t, "2345", w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	now := time.Now()
	repo.p.WithdrawnAt = &now
	require.Equal(t, http.StatusNotFound, request().Code)
}
