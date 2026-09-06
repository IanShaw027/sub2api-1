//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type handlerVideoObservationQueue struct {
	service.CreationVideoObservationQueue
	reserved            *service.CreationVideoObservation
	reserveErr, bindErr error
	boundTask           string
	cancelled           bool
	bindContextErr      error
}

func (q *handlerVideoObservationQueue) Reserve(_ context.Context, job *service.CreationVideoObservation, _ int, _ time.Duration) error {
	copy := *job
	q.reserved = &copy
	return q.reserveErr
}
func (q *handlerVideoObservationQueue) Bind(ctx context.Context, _, task string, _ time.Duration) error {
	q.bindContextErr = ctx.Err()
	q.boundTask = task
	return q.bindErr
}
func (q *handlerVideoObservationQueue) CancelReservation(context.Context, string) error {
	q.cancelled = true
	return nil
}

func localVideoObserverTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/creation/local/videos/generations", strings.NewReader("private prompt and media"))
	c.Request.Header.Set("Authorization", "Bearer browser-secret")
	c.Request.Header.Set("Cookie", "private-cookie")
	c.Set(creationGatewayPreparedKey, true)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 9, UserID: 7, Group: &service.Group{ID: 3, Platform: service.PlatformGrok}})
	c.Set(string(middleware2.ContextKeySubscription), &service.UserSubscription{ID: 42, UserID: 7, GroupID: 3})
	return c, w
}

func videoObserverHandler(q *handlerVideoObservationQueue) *CreationHandler {
	return &CreationHandler{openAI: &OpenAIGatewayHandler{}, localVideoObserver: service.NewCreationVideoObserver(q, nil, nil, nil, nil, nil)}
}

func TestLocalVideoObservationCapacityIsReservedBeforeUpstreamSubmission(t *testing.T) {
	q := &handlerVideoObservationQueue{reserveErr: service.ErrCreationVideoObservationFull}
	c, w := localVideoObserverTestContext()
	called := false
	videoObserverHandler(q).submitLocalVideo(c, func(*gin.Context) { called = true })
	require.False(t, called)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)

	q = &handlerVideoObservationQueue{}
	c, w = localVideoObserverTestContext()
	videoObserverHandler(q).submitLocalVideo(c, func(c *gin.Context) { c.JSON(http.StatusBadRequest, gin.H{"error": "rejected"}) })
	require.True(t, q.cancelled)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLocalVideoObservationPersistsBeforeSuccessAndSurvivesBrowserCancellation(t *testing.T) {
	q := &handlerVideoObservationQueue{}
	c, w := localVideoObserverTestContext()
	ctx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(ctx)
	videoObserverHandler(q).submitLocalVideo(c, func(c *gin.Context) {
		require.NotNil(t, q.reserved)
		deadline, ok := c.Request.Context().Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), service.CreationVideoSubmissionTimeout)
		c.JSON(http.StatusAccepted, gin.H{"request_id": "video-1"})
		require.Empty(t, w.Body.String(), "success must not reach the browser before Bind")
		cancel()
		require.NoError(t, c.Request.Context().Err(), "native owner binding and billing snapshot must survive browser cancellation")
	})
	require.NoError(t, q.bindContextErr)
	require.Equal(t, "video-1", q.boundTask)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.JSONEq(t, `{"request_id":"video-1"}`, w.Body.String())
	metadata, err := json.Marshal(q.reserved)
	require.NoError(t, err)
	require.NotContains(t, string(metadata), "private")
	require.NotContains(t, string(metadata), "browser-secret")
	require.NotContains(t, string(metadata), "prompt")
	require.Equal(t, int64(42), q.reserved.SubscriptionID)
}

func TestLocalVideoAcceptedBindFailureKeepsOriginalTaskAndWarnsWithoutResubmission(t *testing.T) {
	q := &handlerVideoObservationQueue{bindErr: errors.New("redis unavailable")}
	c, w := localVideoObserverTestContext()
	calls := 0
	videoObserverHandler(q).submitLocalVideo(c, func(c *gin.Context) {
		calls++
		c.JSON(http.StatusAccepted, gin.H{"request_id": "video-accepted", "status": "pending"})
	})
	require.Equal(t, 1, calls)
	require.Equal(t, http.StatusAccepted, w.Code)
	var result map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.Equal(t, "video-accepted", result["request_id"])
	require.Equal(t, false, result["observation_persisted"])
	require.False(t, q.cancelled, "unknown accepted reservation expires rather than implying rejection")
}

func TestLocalVideoObserverRehydratesFreshRequestWithoutBrowserSecrets(t *testing.T) {
	key := &service.APIKey{ID: 9, UserID: 7, Group: &service.Group{ID: 3}, User: &service.User{ID: 7, Concurrency: 5, Role: "user"}}
	job := &service.CreationVideoObservation{UserID: 7, GroupID: 3, APIKeyID: 9, SubscriptionID: 42, TaskID: "video-1"}
	sub := &service.UserSubscription{ID: 42, UserID: 7, GroupID: 3}
	pollCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	request, keys := localVideoObserverRequest(pollCtx, job, key, sub)
	require.Equal(t, http.NoBody, request.Body)
	require.Empty(t, request.Header)
	require.Nil(t, request.Form)
	require.Nil(t, request.MultipartForm)
	require.Equal(t, "/api/v1/creation/local/videos/video-1", request.URL.Path)
	require.Same(t, key, keys[string(middleware2.ContextKeyAPIKey)])
	require.Same(t, sub, keys[string(middleware2.ContextKeySubscription)])
	cancel()
	require.ErrorIs(t, request.Context().Err(), context.Canceled)
	require.True(t, localVideoTerminalResponse([]byte(`{"status":"done"}`)))
	require.False(t, localVideoTerminalResponse([]byte(`{"status":"pending"}`)))
}
