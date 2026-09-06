//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
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
	out := make([]service.CreationSession, 0)
	for _, row := range s.items {
		if row.UserID != userID {
			continue
		}
		out = append(out, *row)
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: 1, PageSize: 20}, nil
}

func (s *handlerCreationSessionRepo) Update(ctx context.Context, id int64, input service.UpdateCreationSessionInput) (*service.CreationSession, error) {
	return s.GetByID(ctx, id)
}

func (s *handlerCreationSessionRepo) Delete(ctx context.Context, userID, id int64) error {
	if _, err := s.GetForUser(ctx, userID, id); err != nil {
		return err
	}
	delete(s.items, id)
	return nil
}

type handlerCreationMessageRepo struct {
	items         []service.CreationMessage
	next          int64
	exchangeInput *service.CreateCreationExchangeInput
}

func (s *handlerCreationMessageRepo) Create(ctx context.Context, msg *service.CreationMessage) error {
	s.next++
	msg.ID = s.next
	copy := *msg
	s.items = append(s.items, copy)
	return nil
}
func (s *handlerCreationMessageRepo) CreateExchange(_ context.Context, input service.CreateCreationExchangeInput) (*service.CreationExchange, error) {
	s.exchangeInput = &input
	userContent, _ := json.Marshal(input.UserContent)
	assistantContent, _ := json.Marshal(input.AssistantContent)
	return &service.CreationExchange{
		User:      service.CreationMessage{ID: 101, SessionID: input.SessionID, Role: "user", Content: userContent},
		Assistant: service.CreationMessage{ID: 102, SessionID: input.SessionID, Role: "assistant", Content: assistantContent, Model: &input.Model, InputTokens: input.InputTokens, OutputTokens: input.OutputTokens},
	}, nil
}
func (s *handlerCreationMessageRepo) ListBySession(ctx context.Context, sessionID int64) ([]service.CreationMessage, error) {
	out := make([]service.CreationMessage, 0)
	for _, msg := range s.items {
		if msg.SessionID == sessionID {
			out = append(out, msg)
		}
	}
	return out, nil
}

type handlerCreationImageJobRepo struct {
	items map[int64]*service.CreationImageJob
	next  int64
}

func (s *handlerCreationImageJobRepo) Create(_ context.Context, job *service.CreationImageJob) error {
	s.next++
	job.ID = s.next
	if s.items == nil {
		s.items = map[int64]*service.CreationImageJob{}
	}
	copy := *job
	if job.SessionID != nil {
		v := *job.SessionID
		copy.SessionID = &v
	}
	if job.ProviderTaskID != nil {
		v := *job.ProviderTaskID
		copy.ProviderTaskID = &v
	}
	if job.MediaAssetID != nil {
		v := *job.MediaAssetID
		copy.MediaAssetID = &v
	}
	if job.Error != nil {
		v := *job.Error
		copy.Error = &v
	}
	s.items[job.ID] = &copy
	return nil
}

func (s *handlerCreationImageJobRepo) GetByID(_ context.Context, id int64) (*service.CreationImageJob, error) {
	if row, ok := s.items[id]; ok {
		copy := *row
		return &copy, nil
	}
	return nil, service.ErrCreationImageNotFound
}

func (s *handlerCreationImageJobRepo) GetForUser(ctx context.Context, userID, id int64) (*service.CreationImageJob, error) {
	row, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.UserID != userID {
		return nil, service.ErrCreationImageNotFound
	}
	return row, nil
}

func (s *handlerCreationImageJobRepo) GetByProviderTaskID(_ context.Context, userID int64, providerTaskID string) (*service.CreationImageJob, error) {
	for _, row := range s.items {
		if row.UserID == userID && row.ProviderTaskID != nil && *row.ProviderTaskID == providerTaskID {
			copy := *row
			return &copy, nil
		}
	}
	return nil, service.ErrCreationImageNotFound
}

func (s *handlerCreationImageJobRepo) ListForUser(_ context.Context, userID int64, _ service.CreationImageListFilters) ([]service.CreationImageJob, *pagination.PaginationResult, error) {
	out := make([]service.CreationImageJob, 0)
	for _, row := range s.items {
		if row.UserID == userID {
			out = append(out, *row)
		}
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: 1, PageSize: 20}, nil
}

func (s *handlerCreationImageJobRepo) Update(_ context.Context, id int64, job *service.CreationImageJob) error {
	row, ok := s.items[id]
	if !ok {
		return service.ErrCreationImageNotFound
	}
	row.Status = job.Status
	row.MediaURL = job.MediaURL
	row.StorageID = job.StorageID
	row.StorageKey = job.StorageKey
	if job.MediaAssetID != nil {
		v := *job.MediaAssetID
		row.MediaAssetID = &v
	}
	if job.ProviderTaskID != nil {
		v := *job.ProviderTaskID
		row.ProviderTaskID = &v
	}
	if job.Error != nil {
		v := *job.Error
		row.Error = &v
	}
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
	h := NewCreationHandler(svc, nil, nil, nil, nil, nil, nil)

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

func TestCreationHandlerExchangePersistsDisplayTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	messages := &handlerCreationMessageRepo{}
	svc := service.NewCreationService(&handlerCreationSessionRepo{items: map[int64]*service.CreationSession{1: {ID: 1, UserID: 7}}}, messages, nil, nil, nil, nil)
	h := NewCreationHandler(svc, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7}) })
	r.POST("/sessions/:id/exchanges", h.CreateSessionExchange)
	request := httptest.NewRequest(http.MethodPost, "/sessions/1/exchanges", strings.NewReader(`{"request_id":"request-1","user_content":"question","assistant_content":"answer","model":"gpt-4o","input_tokens":123,"output_tokens":45}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, messages.exchangeInput)
	require.Equal(t, 7, int(messages.exchangeInput.UserID))
	require.Equal(t, 123, *messages.exchangeInput.InputTokens)
	require.Equal(t, 45, *messages.exchangeInput.OutputTokens)
	var response struct {
		Data service.CreationExchange `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "user", response.Data.User.Role)
	require.Equal(t, "assistant", response.Data.Assistant.Role)
	require.Equal(t, 123, *response.Data.Assistant.InputTokens)
	require.Equal(t, 45, *response.Data.Assistant.OutputTokens)
}

func TestCreationHandlerReconcilesOrphanImageJobsWithoutExpiringLiveTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"/images", "/tasks/imgtask_orphan"} {
		for _, expired := range []bool{false, true} {
			for _, cached := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/expired=%t/cached=%t", endpoint, expired, cached), func(t *testing.T) {
					createdAt := time.Now()
					if expired {
						createdAt = createdAt.Add(-time.Hour)
					}
					taskID := "imgtask_orphan"
					jobs := &handlerCreationImageJobRepo{}
					job := &service.CreationImageJob{UserID: 7, GroupID: 3, ProviderTaskID: &taskID, Status: service.CreationImageJobStatusProcessing, CreatedAt: createdAt}
					require.NoError(t, jobs.Create(context.Background(), job))
					store := &asyncImageMemoryStore{tasks: map[string]*service.ImageTaskRecord{}}
					if cached {
						store.tasks[taskID] = &service.ImageTaskRecord{ID: taskID, UserID: 7, APIKeyID: 9, Status: service.ImageTaskStatusProcessing, CreatedAt: createdAt.Unix(), ExpiresAt: time.Now().Add(23 * time.Hour).Unix()}
					}
					tasks := service.NewImageTaskServiceWithOptions(store, 24*time.Hour, 30*time.Minute)
					h := NewCreationHandler(service.NewCreationService(nil, nil, jobs, nil, nil, nil), nil, nil, nil, nil, &AsyncImageHandler{tasks: tasks}, nil)
					r := gin.New()
					r.Use(func(c *gin.Context) {
						c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
						groupID := int64(3)
						c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 9, UserID: 7, GroupID: &groupID})
						c.Set(creationGatewayPreparedKey, true)
					})
					r.GET("/images", h.ListImages)
					r.GET("/tasks/:task_id", h.ImageTask)
					recorder := httptest.NewRecorder()
					r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, endpoint, nil))
					require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
					want := service.CreationImageJobStatusProcessing
					if expired {
						want = service.CreationImageJobStatusFailed
					}
					persisted, err := jobs.GetForUser(context.Background(), 7, job.ID)
					require.NoError(t, err)
					require.Equal(t, want, persisted.Status)
					require.Contains(t, recorder.Body.String(), `"status":"`+want+`"`)
				})
			}
		}
	}
}

func TestCreationHandler_ImagesAsyncPersistsJobAndImageTaskForwardsTaskID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jobs := &handlerCreationImageJobRepo{}
	sessions := &handlerCreationSessionRepo{items: map[int64]*service.CreationSession{
		11: {ID: 11, UserID: 7, GroupID: 3, Title: "Studio", Mode: service.CreationSessionModeImage, Status: service.CreationSessionStatusActive},
	}}
	creationSvc := service.NewCreationService(
		sessions,
		&handlerCreationMessageRepo{},
		jobs,
		&handlerCreationGroupRepo{},
		&handlerCreationUserRepo{},
		&handlerCreationUserSubRepo{},
	)

	groupID := int64(3)
	apiKey := &service.APIKey{
		ID:      9,
		UserID:  7,
		Key:     "sk-creation-test-key-123456",
		GroupID: &groupID,
		Status:  service.StatusAPIKeyActive,
		Purpose: service.APIKeyPurposeCreation,
		User:    &service.User{ID: 7, Role: "user", Status: service.StatusActive, Concurrency: 2},
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformOpenAI,
			Status:               service.StatusActive,
			AllowImageGeneration: true,
		},
	}
	keyRepo := &handlerCreationAPIKeyRepo{key: apiKey}
	apiKeySvc := service.NewAPIKeyService(keyRepo, nil, nil, nil, nil, nil, &config.Config{
		Default: config.DefaultConfig{APIKeyPrefix: "sk-"},
	})
	resolver := service.NewCreationKeyResolver(keyRepo, apiKeySvc)

	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	uploader := service.NewImageResultUploader(&creationTestImageStorage{}, "images/", 0, nil)
	tasks := service.NewImageTaskServiceWithUploader(store, uploader, time.Hour, time.Minute)
	release := make(chan struct{})
	asyncImage := &AsyncImageHandler{tasks: tasks}
	asyncImage.execute = func(_ string, c *gin.Context) {
		<-release
		c.JSON(http.StatusOK, gin.H{"created": 1, "data": []gin.H{{"b64_json": "iVBORw0KGgo="}}})
	}

	h := NewCreationHandler(creationSvc, resolver, nil, nil, nil, asyncImage, nil)
	r := gin.New()
	creation := r.Group("/api/v1/creation")
	creation.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		c.Next()
	})
	creation.POST("/images/generations/async", h.ImagesAsync)
	creation.GET("/images/tasks/:task_id", h.ImageTask)
	creation.GET("/images", h.ListImages)
	creation.GET("/images/:id", h.GetImage)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/creation/images/generations/async?group_id=3", bytes.NewBufferString(`{"model":"gpt-image-1","prompt":"a cat"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Id", "11")
	requestCtx, cancelRequest := context.WithCancel(req.Context())
	req = req.WithContext(requestCtx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	var accepted struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	require.NotEmpty(t, accepted.TaskID)
	require.Equal(t, service.ImageTaskStatusProcessing, accepted.Status)

	require.Len(t, jobs.items, 1)
	var persisted *service.CreationImageJob
	for _, job := range jobs.items {
		persisted = job
	}
	require.NotNil(t, persisted)
	require.Equal(t, int64(7), persisted.UserID)
	require.Equal(t, int64(3), persisted.GroupID)
	require.Equal(t, "gpt-image-1", persisted.Model)
	require.Equal(t, "a cat", persisted.Prompt)
	require.Equal(t, service.CreationImageJobStatusProcessing, persisted.Status)
	require.NotNil(t, persisted.ProviderTaskID)
	require.Equal(t, accepted.TaskID, *persisted.ProviderTaskID)
	require.NotNil(t, persisted.SessionID)
	require.Equal(t, int64(11), *persisted.SessionID)

	cancelRequest()
	close(release)
	require.Eventually(t, func() bool {
		got, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, accepted.TaskID)
		return err == nil && got.Status == service.ImageTaskStatusCompleted
	}, time.Second, 10*time.Millisecond)

	// Completion, not polling, has already made the result durable.
	durable, err := jobs.GetByProviderTaskID(context.Background(), 7, accepted.TaskID)
	require.NoError(t, err)
	require.Equal(t, service.CreationImageJobStatusCompleted, durable.Status)
	require.Equal(t, "test-bucket", durable.StorageID)
	require.NotEmpty(t, durable.StorageKey)
	store.mu.Lock()
	delete(store.tasks, accepted.TaskID)
	store.mu.Unlock()

	pollReq := httptest.NewRequest(http.MethodGet, "/api/v1/creation/images/tasks/"+accepted.TaskID+"?group_id=3", nil)
	pollWriter := httptest.NewRecorder()
	r.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusOK, pollWriter.Code)
	require.Contains(t, pollWriter.Body.String(), accepted.TaskID)
	require.Contains(t, pollWriter.Body.String(), "https://example.test/cat.png")
	require.Contains(t, pollWriter.Body.String(), "renewed=true")

	synced, err := jobs.GetByProviderTaskID(context.Background(), 7, accepted.TaskID)
	require.NoError(t, err)
	require.Equal(t, service.CreationImageJobStatusCompleted, synced.Status)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/creation/images", nil)
	listWriter := httptest.NewRecorder()
	r.ServeHTTP(listWriter, listReq)
	require.Equal(t, http.StatusOK, listWriter.Code)
	require.Contains(t, listWriter.Body.String(), "https://example.test/cat.png")
	require.Contains(t, listWriter.Body.String(), accepted.TaskID)
	require.NotContains(t, listWriter.Body.String(), "storage_key")
	require.NotContains(t, listWriter.Body.String(), "test-bucket")
}

type creationTestImageStorage struct{}

func (*creationTestImageStorage) Save(context.Context, string, string, []byte) (string, error) {
	return "https://example.test/cat.png?expired=true", nil
}

func (*creationTestImageStorage) StorageID() string { return "test-bucket" }

func (*creationTestImageStorage) URL(context.Context, string) (string, error) {
	return "https://example.test/cat.png?renewed=true", nil
}

func TestCreationHandler_ImageTaskFallsBackToIDParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	created, err := tasks.Create(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9})
	require.NoError(t, err)

	groupID := int64(3)
	apiKey := &service.APIKey{
		ID:      9,
		UserID:  7,
		Key:     "sk-creation-test-key-123456",
		GroupID: &groupID,
		Status:  service.StatusAPIKeyActive,
		Purpose: service.APIKeyPurposeCreation,
		User:    &service.User{ID: 7, Role: "user", Status: service.StatusActive, Concurrency: 2},
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformOpenAI,
			Status:               service.StatusActive,
			AllowImageGeneration: true,
		},
	}
	keyRepo := &handlerCreationAPIKeyRepo{key: apiKey}
	apiKeySvc := service.NewAPIKeyService(keyRepo, nil, nil, nil, nil, nil, &config.Config{
		Default: config.DefaultConfig{APIKeyPrefix: "sk-"},
	})
	h := NewCreationHandler(
		service.NewCreationService(
			&handlerCreationSessionRepo{items: map[int64]*service.CreationSession{}},
			&handlerCreationMessageRepo{},
			&handlerCreationImageJobRepo{},
			&handlerCreationGroupRepo{},
			&handlerCreationUserRepo{},
			&handlerCreationUserSubRepo{},
		),
		service.NewCreationKeyResolver(keyRepo, apiKeySvc),
		nil, nil, nil,
		&AsyncImageHandler{tasks: tasks},
		nil,
	)

	r := gin.New()
	creation := r.Group("/api/v1/creation")
	creation.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		c.Next()
	})
	creation.GET("/images/tasks/:id", h.ImageTask)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/creation/images/tasks/"+created.ID+"?group_id=3", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), created.ID)
}

type handlerCreationAPIKeyRepo struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (s *handlerCreationAPIKeyRepo) GetByUserGroupAndPurpose(_ context.Context, userID, groupID int64, purpose string) (*service.APIKey, error) {
	if s.key != nil && s.key.UserID == userID && s.key.GroupID != nil && *s.key.GroupID == groupID && s.key.Purpose == purpose {
		return s.key, nil
	}
	return nil, nil
}

func (s *handlerCreationAPIKeyRepo) GetByKeyForAuth(_ context.Context, key string) (*service.APIKey, error) {
	if s.key != nil && s.key.Key == key {
		return s.key, nil
	}
	return nil, service.ErrAPIKeyNotFound
}

func TestCreationHandler_RejectsCrossUserSessionAndImageAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerID := int64(7)
	attackerID := int64(99)
	sessionID := int64(11)
	imageID := int64(21)
	taskID := "task_owner_only"

	sessions := &handlerCreationSessionRepo{items: map[int64]*service.CreationSession{
		sessionID: {
			ID:      sessionID,
			UserID:  ownerID,
			GroupID: 3,
			Title:   "Owner session",
			Model:   "gpt-4o",
			Mode:    service.CreationSessionModeChat,
			Status:  service.CreationSessionStatusActive,
		},
	}}
	messages := &handlerCreationMessageRepo{items: []service.CreationMessage{{
		ID:        1,
		SessionID: sessionID,
		Role:      service.CreationMessageRoleUser,
		Content:   json.RawMessage(`"hello"`),
	}}}
	jobs := &handlerCreationImageJobRepo{items: map[int64]*service.CreationImageJob{
		imageID: {
			ID:             imageID,
			UserID:         ownerID,
			GroupID:        3,
			Status:         service.CreationImageJobStatusProcessing,
			Model:          "gpt-image-1",
			Prompt:         "a cat",
			ProviderTaskID: &taskID,
		},
	}}
	jobs.next = imageID

	h := NewCreationHandler(
		service.NewCreationService(
			sessions,
			messages,
			jobs,
			&handlerCreationGroupRepo{},
			&handlerCreationUserRepo{},
			&handlerCreationUserSubRepo{},
		),
		nil, nil, nil, nil, nil, nil,
	)

	var currentUser int64
	r := gin.New()
	creation := r.Group("/api/v1/creation")
	creation.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: currentUser})
		c.Next()
	})
	creation.GET("/sessions", h.ListSessions)
	creation.GET("/sessions/:id", h.GetSession)
	creation.PATCH("/sessions/:id", h.UpdateSession)
	creation.DELETE("/sessions/:id", h.DeleteSession)
	creation.GET("/sessions/:id/messages", h.ListSessionMessages)
	creation.POST("/sessions/:id/messages", h.CreateSessionMessage)
	creation.GET("/images", h.ListImages)
	creation.GET("/images/:id", h.GetImage)

	assertNotFound := func(t *testing.T, method, path string, body string) {
		t.Helper()
		var req *http.Request
		if body != "" {
			req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req = httptest.NewRequest(method, path, nil)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code, "%s %s body=%s", method, path, w.Body.String())
		require.NotContains(t, w.Body.String(), "Owner session")
		require.NotContains(t, w.Body.String(), "hello")
		require.NotContains(t, w.Body.String(), "a cat")
		require.NotContains(t, w.Body.String(), taskID)
	}

	currentUser = attackerID
	assertNotFound(t, http.MethodGet, "/api/v1/creation/sessions/11", "")
	assertNotFound(t, http.MethodPatch, "/api/v1/creation/sessions/11", `{"title":"hijacked"}`)
	assertNotFound(t, http.MethodDelete, "/api/v1/creation/sessions/11", "")
	assertNotFound(t, http.MethodGet, "/api/v1/creation/sessions/11/messages", "")
	assertNotFound(t, http.MethodPost, "/api/v1/creation/sessions/11/messages", `{"role":"user","content":"stolen"}`)
	assertNotFound(t, http.MethodGet, "/api/v1/creation/images/21", "")

	listSessions := httptest.NewRecorder()
	r.ServeHTTP(listSessions, httptest.NewRequest(http.MethodGet, "/api/v1/creation/sessions", nil))
	require.Equal(t, http.StatusOK, listSessions.Code)
	require.NotContains(t, listSessions.Body.String(), "Owner session")
	require.NotContains(t, listSessions.Body.String(), `"id":11`)

	listImages := httptest.NewRecorder()
	r.ServeHTTP(listImages, httptest.NewRequest(http.MethodGet, "/api/v1/creation/images", nil))
	require.Equal(t, http.StatusOK, listImages.Code)
	require.NotContains(t, listImages.Body.String(), "a cat")
	require.NotContains(t, listImages.Body.String(), taskID)

	_, err := sessions.GetByID(context.Background(), sessionID)
	require.NoError(t, err)
	_, err = jobs.GetByID(context.Background(), imageID)
	require.NoError(t, err)
	require.Len(t, messages.items, 1)

	currentUser = ownerID
	ownerGet := httptest.NewRecorder()
	r.ServeHTTP(ownerGet, httptest.NewRequest(http.MethodGet, "/api/v1/creation/sessions/11", nil))
	require.Equal(t, http.StatusOK, ownerGet.Code)
	require.Contains(t, ownerGet.Body.String(), "Owner session")

	ownerImage := httptest.NewRecorder()
	r.ServeHTTP(ownerImage, httptest.NewRequest(http.MethodGet, "/api/v1/creation/images/21", nil))
	require.Equal(t, http.StatusOK, ownerImage.Code)
	require.Contains(t, ownerImage.Body.String(), "a cat")
}
