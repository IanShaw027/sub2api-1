package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	assets map[int64]*service.MediaAsset
}

func (r *aiSkillMediaRepoStub) Create(_ context.Context, asset *service.MediaAsset) error {
	r.nextID++
	asset.ID = r.nextID
	if r.assets == nil {
		r.assets = make(map[int64]*service.MediaAsset)
	}
	cloned := *asset
	r.assets[asset.ID] = &cloned
	return nil
}

func (r *aiSkillMediaRepoStub) GetByID(_ context.Context, id int64) (*service.MediaAsset, error) {
	if asset, ok := r.assets[id]; ok && asset != nil {
		cloned := *asset
		return &cloned, nil
	}
	return &service.MediaAsset{
		ID:         id,
		ObjectKey:  fmt.Sprintf("ai_image/stub/%d.png", id),
		Visibility: service.MediaVisibilityPublic,
		Status:     service.MediaStatusActive,
	}, nil
}

func (*aiSkillMediaRepoStub) List(context.Context, pagination.PaginationParams, service.MediaListFilters) ([]service.MediaAsset, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (*aiSkillMediaRepoStub) UpdateVisibility(context.Context, int64, string) error { return nil }
func (*aiSkillMediaRepoStub) MarkDeleted(context.Context, int64, time.Time) error   { return nil }
func (*aiSkillMediaRepoStub) GetByObjectKey(context.Context, string, string) (*service.MediaAsset, error) {
	return nil, service.ErrMediaNotFound
}

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
func (*aiSkillMediaStoreStub) PresignGetObject(context.Context, service.MediaStorageRuntimeConfig, string, string, time.Duration) (string, error) {
	return "", nil
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
	require.True(t, strings.HasPrefix(*req.CoverImageURL, "https://media.example/"), "expected direct URL, got %s", *req.CoverImageURL)
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
	require.True(t, strings.HasPrefix(*req.CoverImageURL, "https://media.example/"), "expected direct URL, got %s", *req.CoverImageURL)
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
	require.True(t, strings.HasPrefix(attachments[0].URL, "https://media.example/"), "expected direct URL, got %s", attachments[0].URL)
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
	require.True(t, strings.HasPrefix(attachments[0].URL, "https://media.example/"), "expected direct URL, got %s", attachments[0].URL)
}

func TestAIHandlerBuildRunAttachmentsWithMediaRejectsInvalidDataURL(t *testing.T) {
	h := &AIHandler{mediaService: newAISkillMediaService()}
	_, err := h.buildRunAttachmentsWithMedia(context.Background(), 99, []map[string]any{
		{"url": "data:image/png;base64,%%%invalid%%%", "purpose": "input", "file_name": "input.png"},
	})
	require.Error(t, err)
}

func stringPtr(value string) *string {
	return &value
}
