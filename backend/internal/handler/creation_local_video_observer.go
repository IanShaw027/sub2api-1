package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type localVideoResponseCapture struct {
	gin.ResponseWriter
	body     bytes.Buffer
	status   int
	written  bool
	overflow bool
}

func (w *localVideoResponseCapture) Write(data []byte) (int, error) {
	w.written = true
	remaining := (1 << 20) - w.body.Len()
	if len(data) > remaining {
		w.overflow = true
	}
	if remaining > 0 {
		_, _ = w.body.Write(data[:min(len(data), remaining)])
	}
	return len(data), nil
}
func (w *localVideoResponseCapture) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}
func (w *localVideoResponseCapture) WriteHeader(status int) {
	if !w.written {
		w.status = status
	}
}
func (w *localVideoResponseCapture) WriteHeaderNow() { w.written = true }
func (w *localVideoResponseCapture) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
func (w *localVideoResponseCapture) Size() int {
	if !w.written {
		return -1
	}
	return w.body.Len()
}
func (w *localVideoResponseCapture) Written() bool { return w.written }
func (w *localVideoResponseCapture) Flush()        { w.written = true }

func (h *CreationHandler) SetLocalVideoObserver(observer *service.CreationVideoObserver) {
	h.localVideoObserver = observer
	observer.Start(h.pollLocalVideoObservation)
}

func (h *CreationHandler) submitLocalVideo(c *gin.Context, submit gin.HandlerFunc) {
	h.withLocalVideoGateway(c, func(c *gin.Context) {
		key, _ := middleware2.GetAPIKeyFromContext(c)
		if key == nil || key.Group == nil || h.localVideoObserver == nil {
			imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "local video observation is unavailable")
			return
		}
		job := localVideoObservationMetadata(c, key)
		reserveCtx, reserveCancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		err := h.localVideoObserver.Reserve(reserveCtx, job)
		reserveCancel()
		if err != nil {
			c.Header("Retry-After", "5")
			imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "local video observation capacity is unavailable")
			return
		}
		capture := &localVideoResponseCapture{ResponseWriter: c.Writer}
		original := c.Writer
		c.Writer = capture
		defer func() { c.Writer = original }()
		originalRequest := c.Request
		submissionCtx, submissionCancel := service.NewCreationVideoSubmissionContext(originalRequest.Context())
		c.Request = originalRequest.WithContext(submissionCtx)
		defer func() { submissionCancel(); c.Request = originalRequest }()
		submit(c)
		c.Writer = original
		if capture.Status() < 200 || capture.Status() >= 300 {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
			defer cancel()
			_ = h.localVideoObserver.CancelReservation(cleanupCtx, job.ID)
			c.Data(capture.Status(), capture.Header().Get("Content-Type"), capture.body.Bytes())
			return
		}
		var accepted map[string]json.RawMessage
		if capture.overflow || json.Unmarshal(capture.body.Bytes(), &accepted) != nil || accepted == nil {
			logger.L().Error("creation.local_video.acceptance_unknown", zap.String("observation_id", job.ID))
			c.Header("Content-Length", "")
			imageTaskJSONError(c, http.StatusBadGateway, "api_error", "video acceptance response is invalid; submission status is unknown")
			return
		}
		var taskID string
		for _, field := range []string{"request_id", "task_id", "id"} {
			var value string
			if json.Unmarshal(accepted[field], &value) == nil && strings.TrimSpace(value) != "" {
				taskID = strings.TrimSpace(value)
				break
			}
		}
		bindCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), service.CreationVideoBindTimeout)
		defer cancel()
		err = bindLocalVideoObservation(bindCtx, h.localVideoObserver, job.ID, taskID)
		if err != nil {
			logger.L().Error("creation.local_video.accepted_observation_not_persisted", zap.String("observation_id", job.ID), zap.String("task_id", taskID), zap.Error(err))
			accepted["observation_persisted"] = json.RawMessage("false")
			accepted["observation_warning"], _ = json.Marshal("background observation could not be persisted; keep polling this request_id and do not submit a new generation")
			c.Header("Content-Length", "")
			c.JSON(http.StatusAccepted, accepted)
			return
		}
		c.Data(capture.Status(), capture.Header().Get("Content-Type"), capture.body.Bytes())
	})
}

func localVideoObservationMetadata(c *gin.Context, key *service.APIKey) *service.CreationVideoObservation {
	job := &service.CreationVideoObservation{UserID: key.UserID, GroupID: key.Group.ID, APIKeyID: key.ID}
	if subscription, ok := middleware2.GetSubscriptionFromContext(c); ok && subscription != nil {
		job.SubscriptionID = subscription.ID
	}
	return job
}

func bindLocalVideoObservation(ctx context.Context, observer *service.CreationVideoObserver, id, taskID string) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = observer.Bind(ctx, id, taskID)
		if err == nil || taskID == "" {
			return err
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return err
		case <-timer.C:
		}
	}
	return err
}

func (h *CreationHandler) pollLocalVideoObservation(ctx context.Context, job *service.CreationVideoObservation, key *service.APIKey, subscription *service.UserSubscription) (service.CreationVideoPollResult, error) {
	request, keys := localVideoObserverRequest(ctx, job, key, subscription)
	recorder := httptest.NewRecorder()
	poll, _ := gin.CreateTestContext(recorder)
	poll.Request = request
	for name, value := range keys {
		poll.Set(name, value)
	}
	poll.Set(creationLocalVideoReadKey, true)
	poll.Params = gin.Params{{Key: "request_id", Value: job.TaskID}}
	// Run mandatory completion billing synchronously, then let the observer
	// confirm its committed ledger entry before acknowledging the durable job.
	gateway := *h.openAI
	gateway.usageRecordWorkerPool = nil
	gateway.GrokVideoStatus(poll)
	if recorder.Code < 200 || recorder.Code >= 300 {
		return service.CreationVideoPollResult{}, fmt.Errorf("video status returned %d", recorder.Code)
	}
	return service.CreationVideoPollResult{Terminal: localVideoTerminalResponse(recorder.Body.Bytes()), Billable: service.IsGrokVideoStatusBillable(recorder.Body.Bytes())}, nil
}

func localVideoObserverRequest(ctx context.Context, job *service.CreationVideoObservation, key *service.APIKey, subscription *service.UserSubscription) (*http.Request, map[string]any) {
	ctx = service.WithCreationVideoTaskContext(ctx)
	ctx = context.WithValue(ctx, ctxkey.UserID, job.UserID)
	ctx = context.WithValue(ctx, ctxkey.Group, key.Group)
	ctx = service.WithResolvedTargetPlatform(ctx, service.PlatformGrok)
	const prefix = "/api/v1/creation/local/videos/"
	request := (&http.Request{Method: http.MethodGet, URL: &url.URL{Path: prefix + job.TaskID, RawPath: prefix + url.PathEscape(job.TaskID)}, Header: make(http.Header), Body: http.NoBody}).WithContext(ctx)
	request.RequestURI = request.URL.RequestURI()
	keys := map[string]any{
		string(middleware2.ContextKeyAPIKey):   key,
		string(middleware2.ContextKeyUser):     middleware2.AuthSubject{UserID: job.UserID, Concurrency: key.User.Concurrency},
		string(middleware2.ContextKeyUserRole): key.User.Role,
	}
	if subscription != nil {
		keys[string(middleware2.ContextKeySubscription)] = subscription
	}
	return request, keys
}

func localVideoTerminalResponse(body []byte) bool {
	var result struct {
		Status string `json:"status"`
		State  string `json:"state"`
	}
	if json.Unmarshal(body, &result) != nil {
		return false
	}
	switch strings.ToLower(firstNonEmptyString(result.Status, result.State)) {
	case "done", "completed", "succeeded", "success", "failed", "error", "expired", "canceled", "cancelled":
		return true
	default:
		return false
	}
}
