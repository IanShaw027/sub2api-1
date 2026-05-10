package handler

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

const aiSkillMediaTestPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO0lZC8AAAAASUVORK5CYII="

type aiSkillMediaRepoStub struct {
	nextID int64
}

func (r *aiSkillMediaRepoStub) Create(_ context.Context, asset *service.MediaAsset) error {
	r.nextID++
	asset.ID = r.nextID
	return nil
}

func (*aiSkillMediaRepoStub) GetByID(context.Context, int64) (*service.MediaAsset, error) {
	return nil, nil
}

func (*aiSkillMediaRepoStub) List(context.Context, pagination.PaginationParams, service.MediaListFilters) ([]service.MediaAsset, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (*aiSkillMediaRepoStub) UpdateVisibility(context.Context, int64, string) error { return nil }
func (*aiSkillMediaRepoStub) MarkDeleted(context.Context, int64, time.Time) error   { return nil }

type aiSkillMediaStoreStub struct{}

func (*aiSkillMediaStoreStub) Upload(context.Context, service.MediaStorageRuntimeConfig, string, string, []byte, string) error {
	return nil
}
func (*aiSkillMediaStoreStub) Download(context.Context, service.MediaStorageRuntimeConfig, string, string) (io.ReadCloser, error) {
	return nil, nil
}
func (*aiSkillMediaStoreStub) Delete(context.Context, service.MediaStorageRuntimeConfig, string, string) error {
	return nil
}
func (*aiSkillMediaStoreStub) Stat(context.Context, service.MediaStorageRuntimeConfig, string, string) (int64, error) {
	return 0, nil
}

func newAISkillMediaService() *service.MediaService {
	cfg := &config.Config{}
	cfg.Media.Enabled = true
	cfg.Media.Endpoint = "https://storage.example.com"
	cfg.Media.Region = "auto"
	cfg.Media.Bucket = "media"
	cfg.Media.AccessKeyID = "test-ak"
	cfg.Media.SecretAccessKey = "test-sk"
	cfg.Media.PublicBaseURL = "https://media.example"
	cfg.Media.MaxUploadSizeBytes = 1024 * 1024
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	return service.NewMediaService(&aiSkillMediaRepoStub{}, &aiSkillMediaStoreStub{}, cfg)
}

func aiSkillMediaTestPNGBytes(t *testing.T) []byte {
	t.Helper()
	body, err := base64.StdEncoding.DecodeString(aiSkillMediaTestPNGBase64)
	require.NoError(t, err)
	return body
}

func TestAIHandlerNormalizeSkillCoverImageToMediaURL(t *testing.T) {
	h := &AIHandler{mediaService: newAISkillMediaService()}
	req := &skillUpsertRequest{CoverImageURL: stringPtr("data:image/png;base64," + aiSkillMediaTestPNGBase64)}

	require.NoError(t, h.normalizeSkillCoverImage(context.Background(), req, "skill-1"))
	require.Equal(t, "https://media.example/api/v1/media/public/1", *req.CoverImageURL)
}

func TestAIHandlerNormalizeSkillCoverImageStoresRemoteURLInMedia(t *testing.T) {
	h := &AIHandler{mediaService: newAISkillMediaService()}
	png := aiSkillMediaTestPNGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	defer srv.Close()

	req := &skillUpsertRequest{CoverImageURL: stringPtr(srv.URL + "/cover.png")}

	require.NoError(t, h.normalizeSkillCoverImage(context.Background(), req, "skill-1"))
	require.Equal(t, "https://media.example/api/v1/media/public/1", *req.CoverImageURL)
}

func TestAIHandlerBuildRunAttachmentsWithMedia(t *testing.T) {
	h := &AIHandler{mediaService: newAISkillMediaService()}
	png := aiSkillMediaTestPNGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	defer srv.Close()

	attachments, err := h.buildRunAttachmentsWithMedia(context.Background(), 99, []map[string]any{
		{"url": srv.URL + "/input.png", "purpose": "input", "file_name": "input.png"},
		{"asset_id": 123, "url": "https://example.invalid/keep.png"},
	})
	require.NoError(t, err)
	require.Len(t, attachments, 2)
	require.Equal(t, "https://media.example/api/v1/media/public/1", attachments[0].URL)
	require.Equal(t, "input", attachments[0].Purpose)
	require.Equal(t, "input.png", attachments[0].FileName)
	require.NotNil(t, attachments[1].AssetID)
	require.EqualValues(t, 123, *attachments[1].AssetID)
	require.Equal(t, "https://example.invalid/keep.png", attachments[1].URL)
}

func TestAIHandlerBuildRunAttachmentsWithMediaPromotesManagedIDsToURLs(t *testing.T) {
	h := &AIHandler{mediaService: newAISkillMediaService()}
	attachments, err := h.buildRunAttachmentsWithMedia(context.Background(), 99, []map[string]any{
		{"media_id": 321, "purpose": "reference", "file_name": "reference.png"},
	})
	require.NoError(t, err)
	require.Len(t, attachments, 1)
	require.NotNil(t, attachments[0].MediaID)
	require.EqualValues(t, 321, *attachments[0].MediaID)
	require.Equal(t, "https://media.example/api/v1/media/public/321", attachments[0].URL)
}

func TestAIHandlerBuildRunAttachmentsWithMediaKeepsRawURLWhenAdoptionFails(t *testing.T) {
	h := &AIHandler{mediaService: newAISkillMediaService()}
	attachments, err := h.buildRunAttachmentsWithMedia(context.Background(), 99, []map[string]any{
		{"url": "data:image/png;base64,%%%invalid%%%", "purpose": "input", "file_name": "input.png"},
	})
	require.NoError(t, err)
	require.Len(t, attachments, 1)
	require.Equal(t, "data:image/png;base64,%%%invalid%%%", attachments[0].URL)
	require.Nil(t, attachments[0].MediaID)
}

func stringPtr(value string) *string {
	return &value
}
