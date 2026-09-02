//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type handlerCreationSessionRepo struct {
	items map[int64]*service.CreationSession
	next  int64
}

func (s *handlerCreationSessionRepo) Create(ctx context.Context, input *service.CreationSession) error {
	s.next++
	input.ID = s.next
	if s.items == nil {
		s.items = map[int64]*service.CreationSession{}
	}
	copy := *input
	s.items[input.ID] = &copy
	return nil
}

func (s *handlerCreationSessionRepo) GetByID(ctx context.Context, id int64) (*service.CreationSession, error) {
	if row, ok := s.items[id]; ok {
		return row, nil
	}
	return nil, service.ErrCreationSessionNotFound
}

func (s *handlerCreationSessionRepo) GetForUser(ctx context.Context, userID, id int64) (*service.CreationSession, error) {
	row, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.UserID != userID {
		return nil, service.ErrCreationSessionNotFound
	}
	return row, nil
}

func (s *handlerCreationSessionRepo) ListForUser(ctx context.Context, userID int64, filters service.CreationSessionListFilters) ([]service.CreationSession, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *handlerCreationSessionRepo) Update(ctx context.Context, id int64, input service.UpdateCreationSessionInput) (*service.CreationSession, error) {
	return s.GetByID(ctx, id)
}

func (s *handlerCreationSessionRepo) Delete(ctx context.Context, userID, id int64) error {
	return nil
}

type handlerCreationMessageRepo struct{}

func (s *handlerCreationMessageRepo) Create(ctx context.Context, msg *service.CreationMessage) error {
	return nil
}
func (s *handlerCreationMessageRepo) ListBySession(ctx context.Context, sessionID int64) ([]service.CreationMessage, error) {
	return nil, nil
}

type handlerCreationImageJobRepo struct{}

func (s *handlerCreationImageJobRepo) Create(ctx context.Context, job *service.CreationImageJob) error {
	return nil
}
func (s *handlerCreationImageJobRepo) GetByID(ctx context.Context, id int64) (*service.CreationImageJob, error) {
	return nil, service.ErrCreationImageNotFound
}
func (s *handlerCreationImageJobRepo) GetForUser(ctx context.Context, userID, id int64) (*service.CreationImageJob, error) {
	return nil, service.ErrCreationImageNotFound
}
func (s *handlerCreationImageJobRepo) ListForUser(ctx context.Context, userID int64, filters service.CreationImageListFilters) ([]service.CreationImageJob, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *handlerCreationImageJobRepo) Update(ctx context.Context, id int64, job *service.CreationImageJob) error {
	return nil
}

type handlerCreationUserRepo struct {
	service.UserRepository
}

func (s *handlerCreationUserRepo) GetByID(ctx context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Status: service.StatusActive}, nil
}

type handlerCreationGroupRepo struct {
	service.GroupRepository
}

func (s *handlerCreationGroupRepo) GetByID(ctx context.Context, id int64) (*service.Group, error) {
	return &service.Group{ID: id, Status: service.StatusActive, Platform: service.PlatformOpenAI}, nil
}

type handlerCreationUserSubRepo struct {
	service.UserSubscriptionRepository
}

func (s *handlerCreationUserSubRepo) GetActiveByUserIDAndGroupID(ctx context.Context, userID, groupID int64) (*service.UserSubscription, error) {
	return nil, service.ErrSubscriptionNotFound
}

func TestCreationHandler_CreateAndGetSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewCreationService(
		&handlerCreationSessionRepo{items: map[int64]*service.CreationSession{}},
		&handlerCreationMessageRepo{},
		&handlerCreationImageJobRepo{},
		&handlerCreationGroupRepo{},
		&handlerCreationUserRepo{},
		&handlerCreationUserSubRepo{},
	)
	h := NewCreationHandler(svc, nil, nil, nil, nil, nil)

	r := gin.New()
	v1 := r.Group("/api/v1")
	creation := v1.Group("/creation")
	creation.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		c.Next()
	})
	creation.POST("/sessions", h.CreateSession)
	creation.GET("/sessions/:id", h.GetSession)

	body := `{"group_id":3,"title":"Studio","mode":"chat"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/creation/sessions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	data := created["data"].(map[string]any)
	require.Equal(t, float64(1), data["id"])

	req = httptest.NewRequest(http.MethodGet, "/api/v1/creation/sessions/1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
