//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
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

func creationGeminiTestPNG(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	return buffer.Bytes()
}

func creationGeminiTestResponse(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(gin.H{"candidates": []any{gin.H{"content": gin.H{"parts": []any{
		gin.H{"text": "Generated image"},
		gin.H{"inlineData": gin.H{"mimeType": "image/png", "data": base64.StdEncoding.EncodeToString(creationGeminiTestPNG(t))}},
	}}}}, "usageMetadata": gin.H{"promptTokenCount": 20, "candidatesTokenCount": 40, "totalTokenCount": 60}})
	require.NoError(t, err)
	return body
}

func TestCreationGeminiImageJSONContract(t *testing.T) {
	for _, size := range []string{"1K", "2K", "4K"} {
		t.Run(size, func(t *testing.T) {
			model, body, err := parseCreationGeminiImageRequest("application/json", "/creation/local/images/generations/async", []byte(`{"model":"gemini-3.1-flash-image","prompt":"A poster","image_size":"`+size+`","aspect_ratio":"16:9","n":1}`))
			require.NoError(t, err)
			require.Equal(t, "gemini-3.1-flash-image", model)
			var native map[string]any
			require.NoError(t, json.Unmarshal(body, &native))
			require.Equal(t, map[string]any{"imageSize": size, "aspectRatio": "16:9"}, native["generationConfig"].(map[string]any)["imageConfig"])
			require.Equal(t, []any{"TEXT", "IMAGE"}, native["generationConfig"].(map[string]any)["responseModalities"])
		})
	}
	_, body, err := parseCreationGeminiImageRequest("application/json", "/generations", []byte(`{"model":"gemini-3-pro-image","prompt":"A poster"}`))
	require.NoError(t, err)
	require.Contains(t, string(body), `"imageSize":"1K"`)
	for _, body := range []string{
		`{"model":"../bad","prompt":"x"}`,
		`{"model":"gemini-3-pro-image","prompt":"x","n":2}`,
		`{"model":"gemini-3-pro-image","prompt":"x","stream":true}`,
		`{"model":"gemini-3-pro-image","prompt":"x","image_size":"8K"}`,
		`{"model":"gemini-3-pro-image","prompt":"x","size":"2048x2048"}`,
		`{"model":"gemini-3-pro-image","prompt":"x","aspect_ratio":"unknown"}`,
	} {
		_, _, err := parseCreationGeminiImageRequest("application/json", "/generations", []byte(body))
		require.Error(t, err, body)
	}
	_, _, err = parseCreationGeminiImageRequest("application/json", "/images/edits/async", []byte(`{"model":"gemini-3-pro-image","prompt":"Edit this"}`))
	require.ErrorContains(t, err, "reference image")
}

func TestCreationGeminiMultipartReferences(t *testing.T) {
	for _, count := range []int{2, 14, 15} {
		var buffer bytes.Buffer
		form := multipart.NewWriter(&buffer)
		for key, value := range map[string]string{"model": "gemini-3-pro-image", "prompt": "Combine these", "image_size": "4K", "aspect_ratio": "3:4"} {
			require.NoError(t, form.WriteField(key, value))
		}
		for range count {
			part, err := form.CreateFormFile("image[]", "source.png")
			require.NoError(t, err)
			_, err = part.Write(creationGeminiTestPNG(t))
			require.NoError(t, err)
		}
		require.NoError(t, form.Close())
		_, body, err := parseCreationGeminiImageRequest(form.FormDataContentType(), "/images/edits/async", buffer.Bytes())
		if count > creationGeminiMaxReferences {
			require.ErrorContains(t, err, "14")
			continue
		}
		require.NoError(t, err)
		require.Equal(t, count, strings.Count(string(body), `"inlineData"`))
		require.Contains(t, string(body), `"mimeType":"image/png"`)
		require.Contains(t, string(body), `"imageSize":"4K"`)
	}
	_, err := creationGeminiImageMIME([]byte("not an image"))
	require.Error(t, err)
}

func TestCreationGeminiResponseConversion(t *testing.T) {
	result, err := extractCreationGeminiImages(creationGeminiTestResponse(t))
	require.NoError(t, err)
	require.Contains(t, string(result), `"b64_json"`)
	require.Contains(t, string(result), `"mime_type":"image/png"`)
	require.Contains(t, string(result), `"input_tokens":20`)
	snake := strings.ReplaceAll(strings.ReplaceAll(string(creationGeminiTestResponse(t)), "inlineData", "inline_data"), "mimeType", "mime_type")
	_, err = extractCreationGeminiImages([]byte(snake))
	require.NoError(t, err)
	for _, body := range []string{
		`{"candidates":[{"content":{"parts":[{"text":"No image"}]}}]}`,
		`{"promptFeedback":{"blockReason":"SAFETY"}}`,
		`{"candidates":[{"content":{"parts":[{"inlineData":{"data":"invalid base64"}}]}}]}`,
	} {
		_, err := extractCreationGeminiImages([]byte(body))
		require.Error(t, err)
	}
	thought := strings.Replace(string(creationGeminiTestResponse(t)), `"inlineData":`, `"thought":true,"inlineData":`, 1)
	_, err = extractCreationGeminiImages([]byte(thought))
	require.ErrorContains(t, err, "did not return an image")
}

func TestCreationGeminiUsesNativeGatewayContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/creation/local/images/generations", strings.NewReader(`{"model":"gemini-3.1-flash-image","prompt":"A poster","image_size":"2K","aspect_ratio":"16:9"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	key := &service.APIKey{ID: 9, UserID: 7, Group: &service.Group{Platform: service.PlatformGemini, AllowImageGeneration: true}}
	c.Set(string(middleware2.ContextKeyAPIKey), key)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	called := false
	forwardCreationGeminiImage(c, func(native *gin.Context) {
		called = true
		actual, ok := middleware2.GetAPIKeyFromContext(native)
		require.True(t, ok)
		require.Same(t, key, actual)
		require.Equal(t, "/gemini-3.1-flash-image:generateContent", native.Param("modelAction"))
		require.Equal(t, "/v1beta/models/gemini-3.1-flash-image:generateContent", native.Request.URL.Path)
		body, err := io.ReadAll(native.Request.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), `"imageSize":"2K"`)
		native.Data(http.StatusOK, "application/json", creationGeminiTestResponse(t))
	})
	require.True(t, called)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"b64_json"`)
}

func TestCreationGeminiGatewayErrorsRemainFailures(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/images/generations", strings.NewReader(`{"model":"gemini-3-pro-image","prompt":"A poster"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	forwardCreationGeminiImage(c, func(native *gin.Context) {
		native.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"message": "Quota exhausted"}})
	})
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Quota exhausted")
	require.NotContains(t, recorder.Body.String(), "b64_json")
}

func TestCreationGeminiAdmissionAndLocalTaskIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, allowed := range []bool{false, true} {
		store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
		serviceWithForbiddenUploader := service.NewImageTaskServiceWithResolver(store, func() (*service.ImageResultUploader, bool) { panic("local Gemini must not resolve permanent storage") }, 24*time.Hour, time.Minute)
		tasks := serviceWithForbiddenUploader.ForLocalResults()
		h := &AsyncImageHandler{tasks: tasks, gemini: &GatewayHandler{}}
		nativeResponse := creationGeminiTestResponse(t)
		h.execute = func(platform string, c *gin.Context) {
			forwardCreationGeminiImage(c, func(native *gin.Context) { native.Data(http.StatusOK, "application/json", nativeResponse) })
		}
		router := gin.New()
		router.POST("/api/v1/creation/local/images/generations/async", func(c *gin.Context) {
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 9, UserID: 7, Group: &service.Group{Platform: service.PlatformGemini, AllowImageGeneration: allowed}})
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
			h.Submit(c)
		})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/creation/local/images/generations/async", strings.NewReader(`{"model":"gemini-3-pro-image","prompt":"A poster"}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if !allowed {
			require.Equal(t, http.StatusForbidden, recorder.Code)
			require.Empty(t, store.tasks)
			continue
		}
		require.Equal(t, http.StatusAccepted, recorder.Code, recorder.Body.String())
		var accepted struct {
			TaskID string `json:"task_id"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &accepted))
		require.Eventually(t, func() bool {
			task, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, accepted.TaskID)
			return err == nil && task.Status == service.ImageTaskStatusCompleted
		}, 3*time.Second, 10*time.Millisecond)
		task, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, accepted.TaskID)
		require.NoError(t, err)
		record, err := store.Get(context.Background(), accepted.TaskID)
		require.NoError(t, err)
		require.True(t, record.LocalOnly)
		require.LessOrEqual(t, task.ExpiresAt-task.CreatedAt, int64(time.Hour/time.Second))
		require.Contains(t, string(task.Result), `"b64_json"`)
		_, err = tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 8, APIKeyID: 9}, accepted.TaskID)
		require.Error(t, err)
		_, err = serviceWithForbiddenUploader.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, accepted.TaskID)
		require.Error(t, err)
	}
}
