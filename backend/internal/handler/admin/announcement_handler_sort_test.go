package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type announcementRepoCapture struct {
	service.AnnouncementRepository
	listParams pagination.PaginationParams
}

func (r *announcementRepoCapture) List(ctx context.Context, params pagination.PaginationParams, filters service.AnnouncementListFilters) ([]service.Announcement, *pagination.PaginationResult, error) {
	r.listParams = params
	return []service.Announcement{}, &pagination.PaginationResult{
		Total:    0,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    0,
	}, nil
}

func (r *announcementRepoCapture) GetByID(ctx context.Context, id int64) (*service.Announcement, error) {
	return &service.Announcement{
		ID:        id,
		Title:     "announcement",
		Content:   "content",
		Status:    service.AnnouncementStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

type announcementUserRepoCapture struct {
	service.UserRepository
	listParams  pagination.PaginationParams
	listFilters service.UserListFilters
	users       []service.User
}

func (r *announcementUserRepoCapture) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	r.listParams = params
	r.listFilters = filters
	filtered := make([]service.User, 0, len(r.users))
	needle := strings.ToLower(strings.TrimSpace(filters.Search))
	for _, user := range r.users {
		if needle != "" {
			email := strings.ToLower(user.Email)
			username := strings.ToLower(user.Username)
			if !strings.Contains(email, needle) && !strings.Contains(username, needle) {
				continue
			}
		}
		filtered = append(filtered, user)
	}

	return filtered, &pagination.PaginationResult{
		Total:    int64(len(filtered)),
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    1,
	}, nil
}

type announcementReadRepoCapture struct {
	service.AnnouncementReadRepository
	readMapByUsers map[int64]time.Time
}

func (r *announcementReadRepoCapture) GetReadMapByUsers(ctx context.Context, announcementID int64, userIDs []int64) (map[int64]time.Time, error) {
	if r.readMapByUsers != nil {
		return r.readMapByUsers, nil
	}
	return map[int64]time.Time{}, nil
}

type announcementUserSubRepoCapture struct {
	service.UserSubscriptionRepository
}

func (r *announcementUserSubRepoCapture) ListActiveByUserID(ctx context.Context, userID int64) ([]service.UserSubscription, error) {
	return nil, nil
}

func newAnnouncementSortTestRouter(announcementRepo *announcementRepoCapture, userRepo *announcementUserRepoCapture) *gin.Engine {
	return newAnnouncementSortTestRouterWithReadMap(announcementRepo, userRepo, nil)
}

func newAnnouncementSortTestRouterWithReadMap(announcementRepo *announcementRepoCapture, userRepo *announcementUserRepoCapture, readMap map[int64]time.Time) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := service.NewAnnouncementService(
		announcementRepo,
		&announcementReadRepoCapture{readMapByUsers: readMap},
		userRepo,
		&announcementUserSubRepoCapture{},
	)
	handler := NewAnnouncementHandler(svc)
	router := gin.New()
	router.GET("/admin/announcements", handler.List)
	router.GET("/admin/announcements/:id/read-status", handler.ListReadStatus)
	return router
}

func TestAdminAnnouncementListSortParams(t *testing.T) {
	announcementRepo := &announcementRepoCapture{}
	userRepo := &announcementUserRepoCapture{}
	router := newAnnouncementSortTestRouter(announcementRepo, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/announcements?sort_by=title&sort_order=ASC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "title", announcementRepo.listParams.SortBy)
	require.Equal(t, "ASC", announcementRepo.listParams.SortOrder)
}

func TestAdminAnnouncementListSortDefaults(t *testing.T) {
	announcementRepo := &announcementRepoCapture{}
	userRepo := &announcementUserRepoCapture{}
	router := newAnnouncementSortTestRouter(announcementRepo, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/announcements", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "created_at", announcementRepo.listParams.SortBy)
	require.Equal(t, "desc", announcementRepo.listParams.SortOrder)
}

func TestAdminAnnouncementReadStatusSortParams(t *testing.T) {
	announcementRepo := &announcementRepoCapture{}
	userRepo := &announcementUserRepoCapture{}
	router := newAnnouncementSortTestRouter(announcementRepo, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/announcements/1/read-status?sort_by=balance&sort_order=DESC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "balance", userRepo.listParams.SortBy)
	require.Equal(t, "DESC", userRepo.listParams.SortOrder)
}

func TestAdminAnnouncementReadStatusSortDefaults(t *testing.T) {
	announcementRepo := &announcementRepoCapture{}
	userRepo := &announcementUserRepoCapture{}
	router := newAnnouncementSortTestRouter(announcementRepo, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/announcements/1/read-status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "email", userRepo.listParams.SortBy)
	require.Equal(t, "asc", userRepo.listParams.SortOrder)
}

func TestAdminAnnouncementReadStatusFilterParams(t *testing.T) {
	announcementRepo := &announcementRepoCapture{}
	userRepo := &announcementUserRepoCapture{
		users: []service.User{
			{ID: 1, Email: "alice-unread@example.com", Username: "alice"},
			{ID: 2, Email: "alice-read@example.com", Username: "alice"},
			{ID: 3, Email: "bob-unread@example.com", Username: "bob"},
		},
	}
	router := newAnnouncementSortTestRouterWithReadMap(announcementRepo, userRepo, map[int64]time.Time{
		2: time.Unix(1776790020, 0),
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/announcements/1/read-status?read_status=unread&search=alice", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "alice", userRepo.listFilters.Search)

	var body struct {
		Data struct {
			Items []service.AnnouncementUserReadStatus `json:"items"`
			Total int64                                `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.EqualValues(t, 1, body.Data.Total)
	require.Len(t, body.Data.Items, 1)
	require.EqualValues(t, 1, body.Data.Items[0].UserID)
	require.Nil(t, body.Data.Items[0].ReadAt)
}
