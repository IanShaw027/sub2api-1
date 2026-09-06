//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
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

func TestCreationLocalImagesPreserveParametersWithoutCloudHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, edits := range []bool{false, true} {
		t.Run(map[bool]string{false: "generation", true: "edits"}[edits], func(t *testing.T) {
			store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
			tasks := service.NewImageTaskServiceWithResolver(store, func() (*service.ImageResultUploader, bool) {
				panic("local generation attempted to resolve persistent storage")
			}, 24*time.Hour, time.Minute)
			type forwarded struct {
				path string
				body []byte
			}
			observed := make(chan forwarded, 1)
			async := &AsyncImageHandler{tasks: tasks}
			async.execute = func(_ string, c *gin.Context) {
				body, _ := io.ReadAll(c.Request.Body)
				observed <- forwarded{c.Request.URL.Path, body}
				c.JSON(http.StatusOK, gin.H{"data": []gin.H{{"b64_json": "iVBORw0KGgo="}}})
			}
			jobs := &handlerCreationImageJobRepo{}
			h := &CreationHandler{asyncImage: async, creationService: service.NewCreationService(nil, nil, jobs, nil, nil, nil)}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				userID := int64(7)
				if c.GetHeader("X-Test-Other-User") != "" {
					userID = 8
				}
				groupID := int64(3)
				c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
				c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 9, UserID: userID, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true}})
				c.Set(creationGatewayPreparedKey, true)
			})
			const base = "/api/v1/creation/local/images"
			router.POST(base+"/generations/async", h.LocalImagesAsync)
			router.POST(base+"/edits/async", h.LocalImagesAsync)
			router.GET(base+"/tasks/:task_id", h.LocalImageTask)
			router.GET("/api/v1/creation/images/tasks/:task_id", h.ImageTask)
			path := base + "/generations/async"
			body := []byte(`{"model":"gpt-image-2","prompt":"test","size":"2048x1152","quality":"high","aspect_ratio":"16:9"}`)
			contentType := "application/json"
			if edits {
				path = base + "/edits/async"
				var buffer bytes.Buffer
				form := multipart.NewWriter(&buffer)
				for name, value := range map[string]string{"model": "gpt-image-2", "prompt": "edit", "size": "2048x1152", "quality": "high", "aspect_ratio": "16:9"} {
					require.NoError(t, form.WriteField(name, value))
				}
				for _, filename := range []string{"one.png", "two.png"} {
					part, err := form.CreateFormFile("image", filename)
					require.NoError(t, err)
					_, err = part.Write([]byte("image bytes"))
					require.NoError(t, err)
				}
				require.NoError(t, form.Close())
				body, contentType = buffer.Bytes(), form.FormDataContentType()
			}
			request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
			request.Header.Set("Content-Type", contentType)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusAccepted, recorder.Code, recorder.Body.String())
			var accepted struct {
				TaskID  string `json:"task_id"`
				PollURL string `json:"poll_url"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &accepted))
			require.Equal(t, base+"/tasks/"+accepted.TaskID, accepted.PollURL)
			select {
			case forwarded := <-observed:
				require.Equal(t, strings.TrimSuffix(path, "/async"), forwarded.path)
				require.Equal(t, body, forwarded.body)
			case <-time.After(5 * time.Second):
				t.Fatal("local image request did not execute")
			}
			require.Eventually(t, func() bool {
				task, err := tasks.ForLocalResults().Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, accepted.TaskID)
				return err == nil && task.Status == service.ImageTaskStatusCompleted
			}, 5*time.Second, 10*time.Millisecond)
			poll := httptest.NewRecorder()
			router.ServeHTTP(poll, httptest.NewRequest(http.MethodGet, accepted.PollURL, nil))
			require.Equal(t, http.StatusOK, poll.Code)
			require.Contains(t, poll.Body.String(), `"b64_json"`)
			legacy := httptest.NewRecorder()
			router.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/api/v1/creation/images/tasks/"+accepted.TaskID, nil))
			require.Equal(t, http.StatusNotFound, legacy.Code, "old cloud task lookup must not consume local-only results")
			require.Empty(t, jobs.items, "local submit and both polling paths must not create persistent image history")
			otherRequest := httptest.NewRequest(http.MethodGet, accepted.PollURL, nil)
			otherRequest.Header.Set("X-Test-Other-User", "true")
			denied := httptest.NewRecorder()
			router.ServeHTTP(denied, otherRequest)
			require.Equal(t, http.StatusNotFound, denied.Code)
		})
	}
}

func TestCreationLocalVideoBridgeRetainsOwnerAndRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, platform := range []string{service.PlatformGrok, service.PlatformComposite, service.PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			h := &CreationHandler{openAI: &OpenAIGatewayHandler{}}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			payload := `{"model":"grok-imagine-video","prompt":"test","duration":15,"resolution":"720p","image":{"url":"data:image/png;base64,aGVsbG8="}}`
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/creation/local/videos/generations", strings.NewReader(payload))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(creationGatewayPreparedKey, true)
			key := &service.APIKey{ID: 9, UserID: 7, Group: &service.Group{ID: 3, Platform: platform}}
			c.Set(string(middleware2.ContextKeyAPIKey), key)
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
			called := false
			h.withLocalVideoGateway(c, func(c *gin.Context) {
				called = true
				actual, ok := middleware2.GetAPIKeyFromContext(c)
				require.True(t, ok)
				require.Same(t, key, actual)
				body, err := io.ReadAll(c.Request.Body)
				require.NoError(t, err)
				require.Equal(t, payload, string(body))
			})
			require.Equal(t, platform != service.PlatformOpenAI, called)
			if !called {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			}
		})
	}
}

func TestCreationLocalVideoReadExemptionDoesNotApplyToGenerationOrOrdinaryGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/creation/local/videos/id/content", nil)
	require.False(t, isLocalCreationVideoRead(c, service.GrokMediaEndpointVideoContent))
	c.Set(creationLocalVideoReadKey, true)
	require.True(t, isLocalCreationVideoRead(c, service.GrokMediaEndpointVideoContent))
	require.True(t, isLocalCreationVideoRead(c, service.GrokMediaEndpointVideoStatus))
	require.False(t, isLocalCreationVideoRead(c, service.GrokMediaEndpointVideosGenerations))
	c.Request.Method = http.MethodPost
	require.False(t, isLocalCreationVideoRead(c, service.GrokMediaEndpointVideoContent))
}
