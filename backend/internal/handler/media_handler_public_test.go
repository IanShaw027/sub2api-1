//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mediaHandlerPublicRepoStub struct {
	asset *service.MediaAsset
	err   error
}

func (s mediaHandlerPublicRepoStub) Create(context.Context, *service.MediaAsset) error { panic("unexpected") }
func (s mediaHandlerPublicRepoStub) GetByID(context.Context, int64) (*service.MediaAsset, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.asset, nil
}
func (s mediaHandlerPublicRepoStub) GetByObjectKey(context.Context, string, string) (*service.MediaAsset, error) {
	panic("unexpected")
}
func (s mediaHandlerPublicRepoStub) List(context.Context, pagination.PaginationParams, service.MediaListFilters) ([]service.MediaAsset, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s mediaHandlerPublicRepoStub) UpdateVisibility(context.Context, int64, string) error { panic("unexpected") }
func (s mediaHandlerPublicRepoStub) MarkDeleted(context.Context, int64, time.Time) error   { panic("unexpected") }

func TestMediaHandlerServePublicRejectsNonPublicAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		asset        *service.MediaAsset
		targetPath   string
		thumbnailReq bool
	}{
		{
			name: "private original",
			asset: &service.MediaAsset{
				ID:         42,
				Visibility: service.MediaVisibilityPrivate,
				Status:     service.MediaStatusActive,
			},
			targetPath:   "/api/v1/media/public/42",
			thumbnailReq: false,
		},
		{
			name: "deleted thumbnail",
			asset: &service.MediaAsset{
				ID:         42,
				Visibility: service.MediaVisibilityPublic,
				Status:     service.MediaStatusDeleted,
			},
			targetPath:   "/api/v1/media/public/42/thumbnail",
			thumbnailReq: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Params = gin.Params{{Key: "id", Value: "42"}}
			c.Request = httptest.NewRequest(http.MethodGet, tt.targetPath, nil)

			svc := service.NewMediaService(mediaHandlerPublicRepoStub{asset: tt.asset}, nil, &config.Config{})
			h := NewMediaHandler(svc)

			if tt.thumbnailReq {
				h.ServePublicThumbnail(c)
			} else {
				h.ServePublic(c)
			}

			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Contains(t, rec.Body.String(), "MEDIA_FORBIDDEN")
		})
	}
}
