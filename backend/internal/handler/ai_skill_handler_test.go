//go:build unit

package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler/skillkit"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const aiSkillHandlerTestPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO0lZC8AAAAASUVORK5CYII="

type aiSkillHandlerMediaRepo struct {
	nextID  int64
	created []*service.MediaAsset
	assets  map[int64]*service.MediaAsset
	deleted []int64
}

func (r *aiSkillHandlerMediaRepo) Create(_ context.Context, asset *service.MediaAsset) error {
	if asset != nil && asset.ID <= 0 {
		if r.nextID <= 0 {
			r.nextID = 1
		}
		asset.ID = r.nextID
		r.nextID++
		r.created = append(r.created, asset)
		if r.assets == nil {
			r.assets = make(map[int64]*service.MediaAsset)
		}
		r.assets[asset.ID] = asset
	}
	return nil
}

func (r *aiSkillHandlerMediaRepo) GetByID(_ context.Context, id int64) (*service.MediaAsset, error) {
	if r.assets != nil {
		if asset, ok := r.assets[id]; ok {
			return asset, nil
		}
	}
	return &service.MediaAsset{
		ID:         id,
		ObjectKey:  fmt.Sprintf("ai_skill/stub/%d.png", id),
		Visibility: service.MediaVisibilityPublic,
		Status:     service.MediaStatusActive,
	}, nil
}
func (*aiSkillHandlerMediaRepo) List(context.Context, pagination.PaginationParams, service.MediaListFilters) ([]service.MediaAsset, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (*aiSkillHandlerMediaRepo) UpdateVisibility(context.Context, int64, string) error { return nil }
func (r *aiSkillHandlerMediaRepo) MarkDeleted(_ context.Context, id int64, _ time.Time) error {
	r.deleted = append(r.deleted, id)
	if asset, ok := r.assets[id]; ok {
		asset.Status = service.MediaStatusDeleted
	}
	return nil
}

func (r *aiSkillHandlerMediaRepo) GetByObjectKey(_ context.Context, _, objectKey string) (*service.MediaAsset, error) {
	for _, asset := range r.assets {
		if asset != nil && asset.ObjectKey == objectKey {
			return asset, nil
		}
	}
	return nil, service.ErrMediaNotFound
}

type aiSkillHandlerMediaStore struct {
	uploads [][]byte
	types   []string
	deleted []string
}

func (s *aiSkillHandlerMediaStore) Upload(_ context.Context, _ service.MediaStorageRuntimeConfig, _, _ string, body []byte, contentType string) error {
	s.uploads = append(s.uploads, append([]byte(nil), body...))
	s.types = append(s.types, contentType)
	return nil
}

func (*aiSkillHandlerMediaStore) Download(context.Context, service.MediaStorageRuntimeConfig, string, string) (io.ReadCloser, error) {
	return nil, nil
}
func (s *aiSkillHandlerMediaStore) Delete(_ context.Context, _ service.MediaStorageRuntimeConfig, bucket, objectKey string) error {
	s.deleted = append(s.deleted, bucket+":"+objectKey)
	return nil
}
func (*aiSkillHandlerMediaStore) Stat(context.Context, service.MediaStorageRuntimeConfig, string, string) (int64, error) {
	return 0, nil
}
func (*aiSkillHandlerMediaStore) PresignGetObject(_ context.Context, _ service.MediaStorageRuntimeConfig, _, objectKey string, _ time.Duration) (string, error) {
	return "https://media.example.com/presigned/" + objectKey, nil
}

func TestAIHandlerBuildSkillMetadataStoresCoverImageURL(t *testing.T) {
	t.Parallel()

	handler, repo, store := newAIHandlerMediaTestHarness(t)
	png := aiSkillHandlerTestPNGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	defer srv.Close()

	metadata, cleanupIDs, err := handler.buildSkillMetadata(context.Background(), 42, skillUpsertRequest{
		Name:          "Example Skill",
		Type:          service.AISkillTypePromptChat,
		CoverImageURL: skillMediaPtrString(srv.URL + "/cover.png"),
	}, map[string]any{
		"legacy": true,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(metadata["cover_image_url"].(string), "https://media.example.com/"), "expected direct URL, got %v", metadata["cover_image_url"])
	require.Len(t, repo.created, 1)
	require.Len(t, store.uploads, 1)
	require.Equal(t, png, store.uploads[0])
	require.Equal(t, "image/png", store.types[0])
	require.Equal(t, []int64{1}, cleanupIDs)
}

func TestAIHandlerBuildSkillMetadataPreservesExistingCoverImageWhenOmitted(t *testing.T) {
	t.Parallel()

	handler := &AIHandler{}
	metadata, cleanupIDs, err := handler.buildSkillMetadata(context.Background(), 42, skillUpsertRequest{
		Name: "Example Skill",
		Type: service.AISkillTypePromptChat,
	}, map[string]any{
		"legacy":          true,
		"cover_image_url": "https://media.example.com/api/v1/media/public/9",
	})
	require.NoError(t, err)
	require.Equal(t, "https://media.example.com/api/v1/media/public/9", metadata["cover_image_url"])
	require.Empty(t, cleanupIDs)
}

func TestAIHandlerBuildSkillMetadataKeepsRawCoverImageWhenAdoptionFails(t *testing.T) {
	t.Parallel()

	handler, _, _ := newAIHandlerMediaTestHarness(t)
	raw := "data:image/png;base64,%%%invalid%%%"
	metadata, cleanupIDs, err := handler.buildSkillMetadata(context.Background(), 42, skillUpsertRequest{
		Name:          "Example Skill",
		Type:          service.AISkillTypePromptChat,
		CoverImageURL: skillMediaPtrString(raw),
	}, nil)
	require.NoError(t, err)
	require.Equal(t, raw, metadata["cover_image_url"])
	require.Empty(t, cleanupIDs)
}

func TestAIHandlerBuildRunAttachmentsStoresURLsAndPreservesExistingIDs(t *testing.T) {
	t.Parallel()

	handler, repo, store := newAIHandlerMediaTestHarness(t)
	png := aiSkillHandlerTestPNGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	defer srv.Close()

	attachments, cleanupIDs, err := handler.buildRunAttachments(context.Background(), 42, []map[string]any{
		{
			"url":       srv.URL + "/inline.png",
			"purpose":   "inline",
			"file_name": "inline.png",
		},
		{
			"url":       srv.URL + "/remote.png",
			"purpose":   "reference",
			"file_name": "remote.png",
		},
		{
			"url":       "https://media.example.com/api/v1/media/public/77",
			"purpose":   "existing",
			"file_name": "existing.png",
			"asset_id":  int64(11),
			"media_id":  int64(12),
		},
	})
	require.NoError(t, err)
	require.Len(t, attachments, 3)
	require.True(t, strings.HasPrefix(attachments[0].URL, "https://media.example.com/"), "expected direct URL, got %s", attachments[0].URL)
	require.NotNil(t, attachments[0].MediaID)
	require.Equal(t, int64(1), *attachments[0].MediaID)
	require.True(t, strings.HasPrefix(attachments[1].URL, "https://media.example.com/"), "expected direct URL, got %s", attachments[1].URL)
	require.NotNil(t, attachments[1].MediaID)
	require.Equal(t, int64(2), *attachments[1].MediaID)
	require.NotNil(t, attachments[2].AssetID)
	require.NotNil(t, attachments[2].MediaID)
	require.Equal(t, int64(11), *attachments[2].AssetID)
	require.Equal(t, int64(12), *attachments[2].MediaID)
	require.True(t, strings.HasPrefix(attachments[2].URL, "https://media.example.com/"), "expected direct URL, got %s", attachments[2].URL)
	require.Len(t, repo.created, 2)
	require.Len(t, store.uploads, 2)
	require.Equal(t, png, store.uploads[0])
	require.Equal(t, png, store.uploads[1])
	require.Equal(t, []int64{1, 2}, cleanupIDs)
}

func TestAIHandlerBuildSkillMetadataWithoutMediaPreservesRawCoverImageURL(t *testing.T) {
	t.Parallel()

	handler := &AIHandler{}
	metadata, cleanupIDs, err := handler.buildSkillMetadata(context.Background(), 42, skillUpsertRequest{
		Name:          "Example Skill",
		Type:          service.AISkillTypePromptChat,
		CoverImageURL: skillMediaPtrString("https://example.invalid/cover.png"),
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "https://example.invalid/cover.png", metadata["cover_image_url"])
	require.Empty(t, cleanupIDs)
}

func TestAIHandlerBuildRunAttachmentsWithoutMediaPreservesRawURLs(t *testing.T) {
	t.Parallel()

	handler := &AIHandler{}
	attachments, cleanupIDs, err := handler.buildRunAttachments(context.Background(), 42, []map[string]any{
		{
			"url":       "data:image/png;base64,QUJD",
			"purpose":   "input",
			"file_name": "inline.png",
		},
		{
			"url":       "https://example.invalid/remote.png",
			"purpose":   "reference",
			"file_name": "remote.png",
		},
	})
	require.NoError(t, err)
	require.Len(t, attachments, 2)
	require.Equal(t, "data:image/png;base64,QUJD", attachments[0].URL)
	require.Equal(t, "https://example.invalid/remote.png", attachments[1].URL)
	require.Nil(t, attachments[0].MediaID)
	require.Nil(t, attachments[1].MediaID)
	require.Empty(t, cleanupIDs)
}

func TestAIHandlerBuildRunAttachmentsDoesNotResolveOtherUsersPublicMedia(t *testing.T) {
	t.Parallel()

	handler, repo, _ := newAIHandlerMediaTestHarness(t)
	otherUserID := int64(99)
	repo.assets = map[int64]*service.MediaAsset{
		77: {
			ID:          77,
			ObjectKey:   "ai_skill/stub/77.png",
			Visibility:  service.MediaVisibilityPublic,
			Status:      service.MediaStatusActive,
			OwnerUserID: &otherUserID,
		},
	}

	attachments, cleanupIDs, err := handler.buildRunAttachments(context.Background(), 42, []map[string]any{
		{
			"media_id":  int64(77),
			"purpose":   "reference",
			"file_name": "other-user.png",
		},
	})
	require.NoError(t, err)
	require.Len(t, attachments, 1)
	require.Empty(t, attachments[0].URL)
	require.Empty(t, cleanupIDs)
}

func TestAIHandlerBuildSkillMetadataWithDisabledMediaPreservesRawCoverImageURL(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Media.Enabled = false
	cfg.Media.PublicBaseURL = "https://media.example.com"
	handler := &AIHandler{
		mediaService: service.NewMediaService(&aiSkillHandlerMediaRepo{}, &aiSkillHandlerMediaStore{}, cfg),
	}
	metadata, cleanupIDs, err := handler.buildSkillMetadata(context.Background(), 42, skillUpsertRequest{
		Name:          "Example Skill",
		Type:          service.AISkillTypePromptChat,
		CoverImageURL: skillMediaPtrString("https://example.invalid/cover.png"),
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "https://example.invalid/cover.png", metadata["cover_image_url"])
	require.Empty(t, cleanupIDs)
}

func TestAIHandlerCreateSkillCleansUpUploadedCoverImageOnPersistenceFailure(t *testing.T) {
	t.Parallel()

	handler, repo, store := newAIHandlerMediaTestHarness(t)
	handler.skillModule = &skillkit.Module{
		DomainRepo: &aiSkillHandlerViewerRepo{
			skill: &domain.AISkill{
				ID:     1,
				UserID: 42,
				Type:   domain.AISkillTypePromptChat,
			},
		},
		SkillService: service.NewAISkillService(&aiSkillHandlerCreateFailRepo{createErr: errors.New("persist failed")}),
	}

	png := aiSkillHandlerTestPNGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	defer srv.Close()

	ctx, _ := newAISkillHandlerJSONContext(t, http.MethodPost, "/api/v1/ai/skills", `{"name":"Example Skill","type":"prompt_chat","cover_image_url":"`+srv.URL+`/cover.png"}`)

	handler.CreateSkill(ctx)

	require.Len(t, repo.created, 1)
	require.Len(t, store.uploads, 1)
	require.Len(t, repo.deleted, 1)
	require.Len(t, store.deleted, 1)
}

func TestAIHandlerCreateSkillRejectsScriptUntilRuntimeExecutorIsAvailable(t *testing.T) {
	t.Parallel()

	handler := &AIHandler{}
	ctx, recorder := newAISkillHandlerJSONContext(t, http.MethodPost, "/api/v1/ai/skills", `{"name":"Script Skill","type":"script","content":{"type":"script","source_code":"console.log(1)"}}`)

	handler.CreateSkill(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "AI_SKILL_SCRIPT_CREATION_UNSUPPORTED")
}

func TestAIHandlerRunSkillCleansUpUploadedAttachmentsOnExecutionFailure(t *testing.T) {
	t.Parallel()

	handler, repo, store := newAIHandlerMediaTestHarness(t)
	runSvc := service.NewAISkillRunService(
		&aiSkillHandlerRunSkillRepo{
			skill: &service.AISkill{ID: 7, CreatorUserID: 1001, Type: service.AISkillTypePromptChat},
		},
		&aiSkillHandlerRunVersionRepo{
			version: &service.AISkillVersion{
				ID:            8,
				SkillID:       7,
				CreatorUserID: 1001,
				Type:          service.AISkillTypePromptChat,
				Status:        service.AISkillVersionStatusApproved,
				ExecutionSpec: service.AISkillExecutionSpec{
					Type: service.AISkillTypePromptChat,
					PromptChat: &service.AISkillPromptChatSpec{
						UserPromptTemplate: "Write about {{subject}}",
					},
				},
				BillingPolicy: service.AISkillBillingPolicy{Mode: service.AISkillBillingModeFree},
			},
		},
		&aiSkillHandlerRunRepo{createErr: errors.New("run persistence failed")},
		service.NewAISkillSettlementService(nil, nil, nil),
		nil,
	)
	handler.skillModule = &skillkit.Module{
		DomainRepo: &aiSkillHandlerViewerRepo{
			skill: &domain.AISkill{
				ID:               7,
				UserID:           42,
				Type:             domain.AISkillTypePromptChat,
				Visibility:       domain.AIVisibilityPublic,
				CurrentVersionID: func() *int64 { v := int64(8); return &v }(),
			},
		},
		RunService: runSvc,
	}

	png := aiSkillHandlerTestPNGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	defer srv.Close()

	ctx, _ := newAISkillHandlerJSONContext(t, http.MethodPost, "/api/v1/ai/skills/7/runs", `{"mode":"use","attachments":[{"url":"`+srv.URL+`/input.png","purpose":"input","file_name":"input.png"},{"media_id":321,"purpose":"reference","file_name":"managed.png"}]}`)
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}

	handler.RunSkill(ctx)

	require.Len(t, repo.created, 1)
	require.Len(t, store.uploads, 1)
	require.Len(t, repo.deleted, 1)
	require.Len(t, store.deleted, 1)
}

func TestAIHandlerRunSkillRejectsPrivateSkillBeforeUploadingAttachments(t *testing.T) {
	t.Parallel()

	handler, repo, store := newAIHandlerMediaTestHarness(t)
	runSvc := service.NewAISkillRunService(
		&aiSkillHandlerRunSkillRepo{
			skill: &service.AISkill{
				ID:            7,
				CreatorUserID: 1001,
				Type:          service.AISkillTypePromptChat,
				Metadata: map[string]any{
					"visibility": "private",
				},
			},
		},
		&aiSkillHandlerRunVersionRepo{
			version: &service.AISkillVersion{
				ID:            8,
				SkillID:       7,
				CreatorUserID: 1001,
				Type:          service.AISkillTypePromptChat,
				Status:        service.AISkillVersionStatusApproved,
				ExecutionSpec: service.AISkillExecutionSpec{
					Type:       service.AISkillTypePromptChat,
					PromptChat: &service.AISkillPromptChatSpec{},
				},
				BillingPolicy: service.AISkillBillingPolicy{Mode: service.AISkillBillingModeFree},
			},
		},
		&aiSkillHandlerRunRepo{},
		service.NewAISkillSettlementService(nil, nil, nil),
		nil,
	)
	handler.skillModule = &skillkit.Module{
		DomainRepo: &aiSkillHandlerViewerRepo{
			skill: &domain.AISkill{
				ID:         7,
				UserID:     1001,
				Type:       domain.AISkillTypePromptChat,
				Visibility: domain.AIVisibilityPrivate,
			},
		},
		RunService: runSvc,
	}

	png := aiSkillHandlerTestPNGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	}))
	defer srv.Close()

	ctx, recorder := newAISkillHandlerJSONContext(t, http.MethodPost, "/api/v1/ai/skills/7/runs", `{"mode":"use","attachments":[{"url":"`+srv.URL+`/input.png","purpose":"input","file_name":"input.png"}]}`)
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}

	handler.RunSkill(ctx)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Empty(t, repo.created)
	require.Empty(t, repo.deleted)
	require.Empty(t, store.uploads)
	require.Empty(t, store.deleted)
}

func TestAIHandlerRunSkillWithModeForwardsRequestParametersAndAttachments(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		path   string
		invoke func(*AIHandler, *gin.Context)
	}{
		{
			name: "test",
			path: "/api/v1/ai/skills/7/test",
			invoke: func(handler *AIHandler, ctx *gin.Context) {
				handler.TestSkill(ctx)
			},
		},
		{
			name: "use",
			path: "/api/v1/ai/skills/7/use",
			invoke: func(handler *AIHandler, ctx *gin.Context) {
				handler.UseSkill(ctx)
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runRepo := &aiSkillHandlerRunRepo{}
			handler := &AIHandler{}
			versionID := int64(8)
			runSvc := service.NewAISkillRunService(
				&aiSkillHandlerRunSkillRepo{
					skill: &service.AISkill{ID: 7, CreatorUserID: 42, Type: service.AISkillTypePromptChat},
				},
				&aiSkillHandlerRunVersionRepo{
					version: &service.AISkillVersion{
						ID:            versionID,
						SkillID:       7,
						CreatorUserID: 42,
						Type:          service.AISkillTypePromptChat,
						Status:        service.AISkillVersionStatusApproved,
						ExecutionSpec: service.AISkillExecutionSpec{
							Type: service.AISkillTypePromptChat,
							PromptChat: &service.AISkillPromptChatSpec{
								UserPromptTemplate: "Write about {{subject}}",
							},
						},
						BillingPolicy: service.AISkillBillingPolicy{Mode: service.AISkillBillingModeFree},
					},
				},
				runRepo,
				service.NewAISkillSettlementService(nil, nil, nil),
				nil,
			)
			handler.skillModule = &skillkit.Module{
				DomainRepo: &aiSkillHandlerViewerRepo{
					skill: &domain.AISkill{
						ID:               7,
						UserID:           42,
						Type:             domain.AISkillTypePromptChat,
						Visibility:       domain.AIVisibilityPublic,
						CurrentVersionID: &versionID,
					},
				},
				RunService: runSvc,
			}

			ctx, recorder := newAISkillHandlerJSONContext(t, http.MethodPost, tc.path+"?version_id=8", `{"parameters":{"subject":"sunrise","count":2,"nested":{"enabled":true}},"attachments":[{"url":"https://example.invalid/input.png","purpose":"input","file_name":"input.png"},{"media_id":321,"purpose":"reference","file_name":"managed.png"}]}`)
			ctx.Params = gin.Params{{Key: "id", Value: "7"}}

			tc.invoke(handler, ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Len(t, runRepo.created, 1)
			require.Equal(t, map[string]any{
				"subject": "sunrise",
				"count":   float64(2),
				"nested": map[string]any{
					"enabled": true,
				},
			}, runRepo.created[0].Parameters)
			require.Len(t, runRepo.created[0].Attachments, 2)
			require.Equal(t, "https://example.invalid/input.png", runRepo.created[0].Attachments[0].URL)
			require.Equal(t, "input", runRepo.created[0].Attachments[0].Purpose)
			require.Equal(t, "input.png", runRepo.created[0].Attachments[0].FileName)
			require.NotNil(t, runRepo.created[0].Attachments[1].MediaID)
			require.Equal(t, int64(321), *runRepo.created[0].Attachments[1].MediaID)
		})
	}
}

func TestAIHandlerRunSkillWithModeCleansUpUploadedAttachmentsOnExecutionFailure(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		path   string
		invoke func(*AIHandler, *gin.Context)
	}{
		{
			name: "test",
			path: "/api/v1/ai/skills/7/test",
			invoke: func(handler *AIHandler, ctx *gin.Context) {
				handler.TestSkill(ctx)
			},
		},
		{
			name: "use",
			path: "/api/v1/ai/skills/7/use",
			invoke: func(handler *AIHandler, ctx *gin.Context) {
				handler.UseSkill(ctx)
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, repo, store := newAIHandlerMediaTestHarness(t)
			versionID := int64(8)
			runSvc := service.NewAISkillRunService(
				&aiSkillHandlerRunSkillRepo{
					skill: &service.AISkill{ID: 7, CreatorUserID: 1001, Type: service.AISkillTypePromptChat},
				},
				&aiSkillHandlerRunVersionRepo{
					version: &service.AISkillVersion{
						ID:            versionID,
						SkillID:       7,
						CreatorUserID: 1001,
						Type:          service.AISkillTypePromptChat,
						Status:        service.AISkillVersionStatusApproved,
						ExecutionSpec: service.AISkillExecutionSpec{
							Type: service.AISkillTypePromptChat,
							PromptChat: &service.AISkillPromptChatSpec{
								UserPromptTemplate: "Write about {{subject}}",
							},
						},
						BillingPolicy: service.AISkillBillingPolicy{Mode: service.AISkillBillingModeFree},
					},
				},
				&aiSkillHandlerRunRepo{createErr: errors.New("run persistence failed")},
				service.NewAISkillSettlementService(nil, nil, nil),
				nil,
			)
			handler.skillModule = &skillkit.Module{
				DomainRepo: &aiSkillHandlerViewerRepo{
					skill: &domain.AISkill{
						ID:               7,
						UserID:           42,
						Type:             domain.AISkillTypePromptChat,
						Visibility:       domain.AIVisibilityPublic,
						CurrentVersionID: &versionID,
					},
				},
				RunService: runSvc,
			}

			png := aiSkillHandlerTestPNGBytes(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(png)
			}))
			defer srv.Close()

			ctx, _ := newAISkillHandlerJSONContext(t, http.MethodPost, tc.path+"?version_id=8", `{"attachments":[{"url":"`+srv.URL+`/input.png","purpose":"input","file_name":"input.png"}]}`)
			ctx.Params = gin.Params{{Key: "id", Value: "7"}}

			tc.invoke(handler, ctx)

			require.Len(t, repo.created, 1)
			require.Len(t, store.uploads, 1)
			require.Len(t, repo.deleted, 1)
			require.Len(t, store.deleted, 1)
		})
	}
}

func TestResolveSkillRunVersionForViewerKeepsOwnerDraftAndViewerPublished(t *testing.T) {
	t.Parallel()

	currentVersionID := int64(11)
	publishedVersionID := int64(22)
	skill := &domain.AISkill{
		UserID:             7,
		CurrentVersionID:   &currentVersionID,
		PublishedVersionID: &publishedVersionID,
	}

	ownerVersionID, err := resolveSkillRunVersionForViewer(skill, &currentVersionID, 7)
	require.NoError(t, err)
	require.NotNil(t, ownerVersionID)
	require.Equal(t, currentVersionID, *ownerVersionID)

	viewerVersionID, err := resolveSkillRunVersionForViewer(skill, nil, 9001)
	require.NoError(t, err)
	require.NotNil(t, viewerVersionID)
	require.Equal(t, publishedVersionID, *viewerVersionID)

	forbiddenVersionID, err := resolveSkillRunVersionForViewer(skill, &currentVersionID, 9001)
	require.Error(t, err)
	require.Nil(t, forbiddenVersionID)
}

func TestSkillViewForViewerKeepsOwnerCurrentVersionAndPinsViewerToPublishedVersion(t *testing.T) {
	t.Parallel()

	currentVersionID := int64(11)
	publishedVersionID := int64(22)
	skill := &domain.AISkill{
		UserID:             7,
		CurrentVersionID:   &currentVersionID,
		PublishedVersionID: &publishedVersionID,
	}

	ownerView := skillViewForViewer(skill, 7)
	require.NotNil(t, ownerView)
	require.NotNil(t, ownerView.CurrentVersionID)
	require.Equal(t, currentVersionID, *ownerView.CurrentVersionID)

	viewerView := skillViewForViewer(skill, 9001)
	require.NotNil(t, viewerView)
	require.NotNil(t, viewerView.CurrentVersionID)
	require.Equal(t, publishedVersionID, *viewerView.CurrentVersionID)
}

func newAIHandlerMediaTestHarness(t *testing.T) (*AIHandler, *aiSkillHandlerMediaRepo, *aiSkillHandlerMediaStore) {
	t.Helper()

	repo := &aiSkillHandlerMediaRepo{}
	store := &aiSkillHandlerMediaStore{}
	cfg := &config.Config{}
	cfg.Media.Enabled = true
	cfg.Media.Endpoint = "https://storage.example.com"
	cfg.Media.Region = "auto"
	cfg.Media.Bucket = "media"
	cfg.Media.AccessKeyID = "test-ak"
	cfg.Media.SecretAccessKey = "test-sk"
	cfg.Media.PublicBaseURL = "https://media.example.com"
	cfg.Media.MaxUploadSizeBytes = 1024 * 1024
	cfg.Security.URLAllowlist.Enabled = true
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true

	return &AIHandler{
		mediaService: service.NewMediaService(repo, store, cfg),
	}, repo, store
}

func skillMediaPtrString(value string) *string {
	return &value
}

func newAISkillHandlerJSONContext(t *testing.T, method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Idempotency-Key", "test-key")
	ctx.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 42})
	return ctx, recorder
}

func aiSkillHandlerTestPNGBytes(t *testing.T) []byte {
	t.Helper()

	body, err := base64.StdEncoding.DecodeString(aiSkillHandlerTestPNGBase64)
	require.NoError(t, err)
	return body
}

type aiSkillHandlerCreateFailRepo struct {
	createErr error
}

func (r *aiSkillHandlerCreateFailRepo) CreateSkill(context.Context, *service.AISkill) error {
	return r.createErr
}

func (*aiSkillHandlerCreateFailRepo) GetSkillByID(context.Context, int64) (*service.AISkill, error) {
	return nil, service.ErrAISkillNotFound
}

func (*aiSkillHandlerCreateFailRepo) GetSkillByCreatorAndID(context.Context, int64, int64) (*service.AISkill, error) {
	return nil, service.ErrAISkillNotFound
}

func (*aiSkillHandlerCreateFailRepo) UpdateSkill(context.Context, *service.AISkill) error {
	return nil
}

type aiSkillHandlerRunSkillRepo struct {
	skill *service.AISkill
}

func (*aiSkillHandlerRunSkillRepo) CreateSkill(context.Context, *service.AISkill) error {
	return nil
}

func (r *aiSkillHandlerRunSkillRepo) GetSkillByID(context.Context, int64) (*service.AISkill, error) {
	return r.skill, nil
}

func (*aiSkillHandlerRunSkillRepo) GetSkillByCreatorAndID(context.Context, int64, int64) (*service.AISkill, error) {
	return nil, service.ErrAISkillNotFound
}

func (*aiSkillHandlerRunSkillRepo) UpdateSkill(context.Context, *service.AISkill) error {
	return nil
}

type aiSkillHandlerRunVersionRepo struct {
	version *service.AISkillVersion
}

func (*aiSkillHandlerRunVersionRepo) CreateVersion(context.Context, *service.AISkillVersion) error {
	return nil
}

func (r *aiSkillHandlerRunVersionRepo) GetVersionByID(_ context.Context, id int64) (*service.AISkillVersion, error) {
	if r != nil && r.version != nil && r.version.ID == id {
		return r.version, nil
	}
	return nil, service.ErrAISkillVersionNotFound
}

func (*aiSkillHandlerRunVersionRepo) GetVersionByCreatorAndID(context.Context, int64, int64) (*service.AISkillVersion, error) {
	return nil, service.ErrAISkillVersionNotFound
}

func (r *aiSkillHandlerRunVersionRepo) GetLatestApprovedVersionBySkillID(context.Context, int64) (*service.AISkillVersion, error) {
	return r.version, nil
}

func (*aiSkillHandlerRunVersionRepo) UpdateVersion(context.Context, *service.AISkillVersion) error {
	return nil
}

type aiSkillHandlerRunRepo struct {
	createErr error
	created   []*service.AISkillRun
}

func (r *aiSkillHandlerRunRepo) CreateRun(_ context.Context, run *service.AISkillRun) error {
	if r != nil {
		copy := *run
		r.created = append(r.created, &copy)
	}
	return r.createErr
}

func (*aiSkillHandlerRunRepo) GetRunByID(context.Context, int64) (*service.AISkillRun, error) {
	return nil, service.ErrAISkillRunNotFound
}

func (*aiSkillHandlerRunRepo) UpdateRun(context.Context, *service.AISkillRun) error {
	return nil
}

type aiSkillHandlerViewerRepo struct {
	repository.AISkillRepository

	skill *domain.AISkill
}

func (r *aiSkillHandlerViewerRepo) GetSkillByID(context.Context, int64) (*domain.AISkill, error) {
	if r == nil || r.skill == nil {
		return nil, domain.ErrAISkillNotFound
	}
	copy := *r.skill
	if r.skill.Metadata != nil {
		copy.Metadata = make(map[string]any, len(r.skill.Metadata))
		for key, value := range r.skill.Metadata {
			copy.Metadata[key] = value
		}
	}
	return &copy, nil
}
