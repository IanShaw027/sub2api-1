package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardVideos_NormalizesVideoAliasOnUpstreamBody(t *testing.T) {
	setGinTestMode()
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")

	reqBody := []byte(`{"model":"grok-video-1.5","prompt":"waves on a beach","duration":6}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"video-alias-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{"request_id":"video-job-alias","model":"grok-imagine-video-1.5-preview","status":"pending"}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       91,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "xai-test",
			"base_url": "https://xai.test/v1",
		},
	}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "grok-imagine-video-1.5-preview", gjson.GetBytes(upstream.lastBody, "model").String(),
		"live ForwardVideos path must rewrite grok-video-1.5 → CPA's grok-imagine-video-1.5-preview")
	require.Equal(t, "grok-video-1.5", result.Model, "client model identity preserved for billing/logging")
	require.Equal(t, "grok-video-1.5", result.BillingModel)
	require.Equal(t, "grok-imagine-video-1.5-preview", result.UpstreamModel)
	require.Equal(t, "video-job-alias", result.ResponseID)
	require.Equal(t, 6, result.VideoSeconds)
}

func TestForwardVideos_CreateWithoutDurationDefaultsToCPASeconds(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-imagine-video","prompt":"pending create without duration"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"video-default-dur"},
		},
		Body: io.NopCloser(strings.NewReader(`{"request_id":"video-job-pending","status":"pending"}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       92,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 4, result.VideoSeconds,
		"OpenAI /v1/videos create must use CPA default duration")
	require.Equal(t, 1, result.VideoCount)
	require.NotEmpty(t, result.VideoSize)
	require.True(t, OpenAIForwardResultHasVideoBillingForUsage(result))
}

func TestPrepareGrokMediaForwardBody_VideosNormalizesOpenAIShapeToXAI(t *testing.T) {
	body := []byte(`{
		"model":"xai/grok-imagine-video-1.5-preview",
		"prompt":"animate",
		"seconds":"8",
		"size":"1280x720",
		"input_reference":{"image_url":"https://cdn.example/ref.png"}
	}`)

	out, contentType, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, "application/json")
	require.NoError(t, err)
	require.Equal(t, "application/json", contentType)
	require.True(t, json.Valid(out))
	require.Equal(t, "grok-imagine-video-1.5-preview", gjson.GetBytes(out, "model").String())
	require.Equal(t, int64(8), gjson.GetBytes(out, "duration").Int())
	require.Equal(t, "16:9", gjson.GetBytes(out, "aspect_ratio").String())
	require.Equal(t, "720p", gjson.GetBytes(out, "resolution").String())
	require.Equal(t, "https://cdn.example/ref.png", gjson.GetBytes(out, "image.url").String())
	require.False(t, gjson.GetBytes(out, "seconds").Exists())
	require.False(t, gjson.GetBytes(out, "size").Exists())
	require.False(t, gjson.GetBytes(out, "input_reference").Exists())
}

func TestPrepareGrokMediaForwardBody_VideosNormalizesReferenceImages(t *testing.T) {
	body := []byte(`{
		"model":"grok-imagine-video",
		"prompt":"animate refs",
		"seconds":"12",
		"reference_image_urls":["https://cdn.example/a.png", {"url":"https://cdn.example/ignored.png"}],
		"reference_images":[{"image_url":{"url":"https://cdn.example/b.png"}}]
	}`)

	out, _, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, "application/json")
	require.NoError(t, err)
	require.True(t, json.Valid(out))
	require.Equal(t, int64(10), gjson.GetBytes(out, "duration").Int(), "CPA caps reference-image video duration at 10s")
	require.Equal(t, "https://cdn.example/b.png", gjson.GetBytes(out, "reference_images.0.url").String())
	require.Equal(t, "https://cdn.example/a.png", gjson.GetBytes(out, "reference_images.1.url").String())
	require.Equal(t, "https://cdn.example/ignored.png", gjson.GetBytes(out, "reference_images.2.url").String())
	require.False(t, gjson.GetBytes(out, "reference_image_urls").Exists())
}

func TestPrepareGrokMediaForwardBody_VideosRejectsImageAndReferenceImages(t *testing.T) {
	body := []byte(`{
		"model":"grok-imagine-video",
		"prompt":"bad refs",
		"image_url":"https://cdn.example/base.png",
		"reference_images":["https://cdn.example/a.png"]
	}`)

	_, _, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, "application/json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "image and reference_images cannot be combined")
}

func TestPrepareGrokMediaForwardBody_VideosRejectsUnsupportedFileIDReference(t *testing.T) {
	body := []byte(`{
		"model":"grok-imagine-video",
		"prompt":"bad file",
		"input_reference":{"file_id":"file_123"}
	}`)

	_, _, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, "application/json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "input_reference.file_id is not supported")
}

func TestPrepareGrokMediaForwardBody_VideosRejectsTooManyReferenceImages(t *testing.T) {
	body := []byte(`{
		"model":"grok-imagine-video",
		"prompt":"too many refs",
		"reference_images":[
			"https://cdn.example/1.png",
			"https://cdn.example/2.png",
			"https://cdn.example/3.png",
			"https://cdn.example/4.png",
			"https://cdn.example/5.png",
			"https://cdn.example/6.png",
			"https://cdn.example/7.png",
			"https://cdn.example/8.png"
		]
	}`)

	_, _, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, "application/json")
	require.Error(t, err)
	require.Contains(t, err.Error(), "reference_images supports at most 7")
}

func TestPrepareGrokMediaForwardBody_VideosHonorsExplicitAspectRatioAndResolution(t *testing.T) {
	body := []byte(`{
		"model":"grok-imagine-video",
		"prompt":"explicit options",
		"size":"720x1280",
		"aspect_ratio":"1:1",
		"resolution":"480p"
	}`)

	out, _, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, "application/json")
	require.NoError(t, err)
	require.Equal(t, "1:1", gjson.GetBytes(out, "aspect_ratio").String())
	require.Equal(t, "480p", gjson.GetBytes(out, "resolution").String())
	require.False(t, gjson.GetBytes(out, "size").Exists())
}

func TestForwardVideos_OpenAICreateFormRequestLikeCPA(t *testing.T) {
	setGinTestMode()

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	require.NoError(t, writer.WriteField("model", "sora-2"))
	require.NoError(t, writer.WriteField("prompt", "form video"))
	require.NoError(t, writer.WriteField("seconds", "6"))
	require.NoError(t, writer.WriteField("size", "1280x720"))
	require.NoError(t, writer.WriteField("input_reference[image_url]", "https://cdn.example/ref.png"))
	require.NoError(t, writer.Close())

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(form.Bytes()))
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"video-form-job","status":"pending"}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 107, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, form.Bytes(), "/v1/videos")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	require.JSONEq(t, `{"model":"grok-imagine-video","prompt":"form video","duration":6,"aspect_ratio":"16:9","resolution":"720p","image":{"url":"https://cdn.example/ref.png"}}`, string(upstream.lastBody))
	require.Equal(t, "video-form-job", result.ResponseID)
	require.Equal(t, "1280x720", gjson.Get(rec.Body.String(), "size").String())
	require.Equal(t, 6, result.VideoSeconds)
	require.Equal(t, "720p", result.VideoSize)
}

func TestForwardVideos_OpenAICreateDefaultsDurationAndSizeLikeCPA(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-imagine-video","prompt":"default options"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"video-default-job","status":"pending"}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 98, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(4), gjson.GetBytes(upstream.lastBody, "duration").Int())
	require.Equal(t, "9:16", gjson.GetBytes(upstream.lastBody, "aspect_ratio").String())
	require.Equal(t, "720p", gjson.GetBytes(upstream.lastBody, "resolution").String())
	require.Equal(t, "4", gjson.Get(rec.Body.String(), "seconds").String())
	require.Equal(t, "720x1280", gjson.Get(rec.Body.String(), "size").String())
	require.Equal(t, 4, result.VideoSeconds)
	// Native xAI request field should not leak back into the OpenAI response shape.
	require.False(t, gjson.Get(rec.Body.String(), "duration").Exists())
}

func TestForwardVideos_OpenAICreateClampsSecondsLikeCPA(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-imagine-video","prompt":"long","seconds":"99","size":"1280x720"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"video-clamped-job","status":"pending"}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 99, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(15), gjson.GetBytes(upstream.lastBody, "duration").Int())
	require.Equal(t, "15", gjson.Get(rec.Body.String(), "seconds").String())
	require.Equal(t, 15, result.VideoSeconds)
}

func TestForwardVideos_OpenAICreateInvalidSizeReturnsFailedVideoResource(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-imagine-video","prompt":"bad size","size":"1920x1080"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"request_id":"should-not-call"}`))}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 100, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq, "invalid OpenAI create request must fail before upstream")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := rec.Body.String()
	require.Equal(t, "video", gjson.Get(body, "object").String())
	require.Equal(t, "failed", gjson.Get(body, "status").String())
	require.Equal(t, int64(0), gjson.Get(body, "progress").Int())
	require.Equal(t, "grok-imagine-video", gjson.Get(body, "model").String())
	require.Equal(t, "invalid_request_error", gjson.Get(body, "error.code").String())
	require.Contains(t, gjson.Get(body, "error.message").String(), "size must be one of")
}

func TestForwardVideos_OpenAICreateCanonicalizesAspectRatioAndResolutionLikeCPA(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-imagine-video","prompt":"alias options","aspect_ratio":"landscape","resolution":"1080p"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"video-alias-options","status":"pending"}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 101, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	_, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.NoError(t, err)
	require.Equal(t, "16:9", gjson.GetBytes(upstream.lastBody, "aspect_ratio").String())
	// CPA accepts only 480p/720p for this path; unsupported values fall back to 720p.
	require.Equal(t, "720p", gjson.GetBytes(upstream.lastBody, "resolution").String())
}

func TestPrepareGrokMediaForwardBody_VideosCanonicalizesExplicitOpenAIOptionsLikeCPA(t *testing.T) {
	body := []byte(`{"model":"grok-imagine-video","prompt":"alias native","aspect_ratio":"square","resolution":"480p"}`)

	out, _, err := prepareGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, body, "application/json")
	require.NoError(t, err)
	require.Equal(t, "1:1", gjson.GetBytes(out, "aspect_ratio").String())
	require.Equal(t, "480p", gjson.GetBytes(out, "resolution").String())
}

func TestForwardVideos_OpenAICreateMapsSoraModelToXAIBackendLikeCPA(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"sora-2","prompt":"sora client","seconds":"8"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"video-sora-job","status":"pending"}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 102, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, xai.DefaultImagineVideoModel, gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, xai.DefaultImagineVideoModel, gjson.Get(rec.Body.String(), "model").String())
	require.Equal(t, xai.DefaultImagineVideoModel, result.Model)
	require.Equal(t, xai.DefaultImagineVideoModel, result.BillingModel)
	require.Equal(t, xai.DefaultImagineVideoModel, result.UpstreamModel)
}

func TestForwardVideos_OpenAICreateDefaultsMissingModelToXAIBackendLikeCPA(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"prompt":"missing model client"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"video-default-model","status":"pending"}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 103, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, xai.DefaultImagineVideoModel, gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, xai.DefaultImagineVideoModel, gjson.Get(rec.Body.String(), "model").String())
	require.Equal(t, xai.DefaultImagineVideoModel, result.Model)
	require.Equal(t, xai.DefaultImagineVideoModel, result.UpstreamModel)
}

func TestForwardVideos_OpenAICreateRejectsUnsupportedModelLikeCPA(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"codex/grok-imagine-video","prompt":"bad model"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"request_id":"should-not-call"}`))}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 104, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq, "unsupported OpenAI video model must fail before upstream")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "video", gjson.Get(rec.Body.String(), "object").String())
	require.Equal(t, "failed", gjson.Get(rec.Body.String(), "status").String())
	require.Equal(t, "codex/grok-imagine-video", gjson.Get(rec.Body.String(), "model").String())
	require.Equal(t, "invalid_request_error", gjson.Get(rec.Body.String(), "error.code").String())
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "Model codex/grok-imagine-video is not supported")
}

func TestForwardVideos_XAINativeRejectsLegacyVideoAliasLikeCPA(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"grok-video","prompt":"native legacy alias should fail"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"request_id":"should-not-call"}`))}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 106, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos/generations")
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq, "native xAI video endpoint must reject non-CPA legacy aliases before upstream")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "Model grok-video is not supported")
}

func TestForwardVideos_XAINativeRejectsSoraModelLikeCPA(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"sora-2","prompt":"native bad model"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"request_id":"should-not-call"}`))}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 105, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos/generations")
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, upstream.lastReq, "native xAI video endpoint must reject Sora before upstream")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid_request_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "Model sora-2 is not supported")
}

func TestForwardVideos_NormalizesOpenAICreateShapeOnUpstreamBody(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"xai/grok-imagine-video-1.5-preview","prompt":"animate","seconds":"8","size":"1280x720","input_reference":{"image_url":"https://cdn.example/ref.png"}}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"request_id":"video-create-job","status":"pending"}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 97, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	_, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.NoError(t, err)
	require.Equal(t, "grok-imagine-video-1.5-preview", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, int64(8), gjson.GetBytes(upstream.lastBody, "duration").Int())
	require.Equal(t, "16:9", gjson.GetBytes(upstream.lastBody, "aspect_ratio").String())
	require.Equal(t, "720p", gjson.GetBytes(upstream.lastBody, "resolution").String())
	require.Equal(t, "https://cdn.example/ref.png", gjson.GetBytes(upstream.lastBody, "image.url").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "seconds").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "size").Exists())
	require.False(t, gjson.GetBytes(upstream.lastBody, "input_reference").Exists())
}

func TestForwardVideos_CreateNormalizesXAIResponseToOpenAIVideoObject(t *testing.T) {
	setGinTestMode()

	reqBody := []byte(`{"model":"xai/grok-imagine-video-1.5-preview","prompt":"animate this","seconds":"8","size":"1280x720"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"video-create-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{"request_id":"video-create-job","model":"grok-imagine-video-1.5-preview","status":"pending","usage":{"cost_in_usd_ticks":1}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 96, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, reqBody, "/v1/videos")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "video-create-job", result.ResponseID)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Equal(t, "video", gjson.Get(body, "object").String())
	require.Equal(t, "video-create-job", gjson.Get(body, "id").String())
	require.Equal(t, "grok-imagine-video-1.5-preview", gjson.Get(body, "model").String())
	require.Equal(t, "animate this", gjson.Get(body, "prompt").String())
	require.Equal(t, "8", gjson.Get(body, "seconds").String())
	require.Equal(t, "1280x720", gjson.Get(body, "size").String())
	require.Equal(t, "queued", gjson.Get(body, "status").String())
	require.Equal(t, int64(0), gjson.Get(body, "progress").Int())
	require.True(t, gjson.Get(body, "created_at").Exists())
	require.False(t, gjson.Get(body, "request_id").Exists())
	require.False(t, gjson.Get(body, "usage").Exists())
}

func TestForwardVideos_GETStatusUsesBoundModelFallbackLikeCPA(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video-preview-job", nil)
	SetGrokMediaVideoBoundModel(c, xai.DefaultImagineVideo15Model)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"video-preview-job","status":"completed","progress":100}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 106, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, nil, "/v1/videos/video-preview-job")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, xai.DefaultImagineVideo15Model, gjson.Get(rec.Body.String(), "model").String())
	require.Equal(t, xai.DefaultImagineVideo15Model, result.Model)
}

func TestForwardVideos_GETStatusExtractsResponseID(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video-job-status", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"status-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{"object":"video","id":"video-job-status","model":"grok-imagine-video","status":"completed","progress":100,"seconds":"4","video":{"url":"https://vidgen.x.ai/video.mp4","duration":4},"usage":{"cost_in_usd_ticks":2800000000}}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:       93,
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "xai-test",
		},
	}

	result, err := svc.ForwardVideos(context.Background(), c, account, nil, "/v1/videos/video-job-status")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "video-job-status", result.ResponseID)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "video", gjson.Get(rec.Body.String(), "object").String())
	require.Equal(t, "completed", gjson.Get(rec.Body.String(), "status").String())
	require.Equal(t, "https://vidgen.x.ai/video.mp4", gjson.Get(rec.Body.String(), "video_url").String())
	require.Equal(t, "4", gjson.Get(rec.Body.String(), "seconds").String())
	require.False(t, gjson.Get(rec.Body.String(), "video").Exists())
	require.False(t, gjson.Get(rec.Body.String(), "usage").Exists())
	require.Zero(t, result.VideoSeconds)
}

func TestForwardVideos_GETStatusNormalizesTopLevelXAIError(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video-job-error", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"status-error-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{"code":"invalid-argument","error":"1080p is not available for this request"}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 94, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, nil, "/v1/videos/video-job-error")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Equal(t, "video", gjson.Get(body, "object").String())
	require.Equal(t, "video-job-error", gjson.Get(body, "id").String())
	require.Equal(t, "failed", gjson.Get(body, "status").String())
	require.Equal(t, int64(0), gjson.Get(body, "progress").Int())
	require.Equal(t, "invalid-argument", gjson.Get(body, "error.code").String())
	require.Equal(t, "1080p is not available for this request", gjson.Get(body, "error.message").String())
	require.False(t, gjson.Get(body, "error.type").Exists())
}

func TestForwardVideos_GETStatusNormalizesNestedXAIError(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video-job-nested-error", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"status-nested-error-rid"},
		},
		Body: io.NopCloser(strings.NewReader(`{"id":"video-job-nested-error","model":"grok-imagine-video","status":"error","error":{"message":"blocked by content policy","type":"invalid_request_error","code":"content_policy_violation"}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 95, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "xai-test"}}

	result, err := svc.ForwardVideos(context.Background(), c, account, nil, "/v1/videos/video-job-nested-error")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Equal(t, "video", gjson.Get(body, "object").String())
	require.Equal(t, "video-job-nested-error", gjson.Get(body, "id").String())
	require.Equal(t, "failed", gjson.Get(body, "status").String())
	require.Equal(t, int64(0), gjson.Get(body, "progress").Int())
	require.Equal(t, "content_policy_violation", gjson.Get(body, "error.code").String())
	require.Equal(t, "blocked by content policy", gjson.Get(body, "error.message").String())
	require.False(t, gjson.Get(body, "error.type").Exists())
}

func TestPrepareGrokMediaForwardBody_JSONRewritesOpenAIImageShape(t *testing.T) {
	body := []byte(`{
		"model":"grok-imagine-edit",
		"prompt":"make it blue",
		"image":{"image_url":"https://cdn.example/a.png"},
		"mask":{"image_url":{"url":"https://cdn.example/mask.png"}}
	}`)

	out, contentType, err := prepareGrokMediaForwardBody(GrokMediaEndpointImagesEdits, body, "application/json")
	require.NoError(t, err)
	require.Equal(t, "application/json", contentType)
	require.True(t, json.Valid(out))
	require.Equal(t, "https://cdn.example/a.png", gjson.GetBytes(out, "image.url").String())
	require.Equal(t, "image_url", gjson.GetBytes(out, "image.type").String())
	require.Equal(t, "https://cdn.example/mask.png", gjson.GetBytes(out, "mask.url").String())
	require.Equal(t, "image_url", gjson.GetBytes(out, "mask.type").String())
	// Legacy keys must not remain as the primary official shape fields.
	require.Equal(t, "", gjson.GetBytes(out, "image.image_url").String())
}

func TestPrepareGrokMediaForwardBody_JSONRejectsMoreThanThreeImages(t *testing.T) {
	body := []byte(`{
		"model":"grok-imagine-image-quality",
		"prompt":"collage",
		"images":[
			{"url":"https://cdn.example/1.png","type":"image_url"},
			{"url":"https://cdn.example/2.png","type":"image_url"},
			{"url":"https://cdn.example/3.png","type":"image_url"},
			{"url":"https://cdn.example/4.png","type":"image_url"}
		]
	}`)

	out, _, err := prepareGrokMediaForwardBody(GrokMediaEndpointImagesEdits, body, "application/json")
	require.Error(t, err)
	require.Nil(t, out)
	require.Contains(t, err.Error(), "maximum of 3")
}

func TestPrepareGrokMediaForwardBody_MultipartRejectsMoreThanThreeImages(t *testing.T) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("model", "grok-imagine-edit"))
	require.NoError(t, writer.WriteField("prompt", "too many images"))
	for i, name := range []string{"a.png", "b.png", "c.png", "d.png"} {
		partHeader := textproto.MIMEHeader{}
		partHeader.Set("Content-Disposition", `form-data; name="image"; filename="`+name+`"`)
		partHeader.Set("Content-Type", "image/png")
		part, err := writer.CreatePart(partHeader)
		require.NoError(t, err)
		_, err = part.Write([]byte{0x89, 0x50, 0x4e, 0x47, byte(i), 0x0a, 0x1a, 0x0a})
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	out, _, err := prepareGrokMediaForwardBody(
		GrokMediaEndpointImagesEdits,
		buf.Bytes(),
		writer.FormDataContentType(),
	)
	require.Error(t, err)
	require.Nil(t, out)
	require.Contains(t, err.Error(), "maximum of 3")
}

func TestForwardGrokMedia_PreservesOriginalModelVsUpstreamModel(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"grok-imagine","prompt":"draw a cat"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	account := &Account{
		ID:          94,
		Name:        "grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "api-key",
			"base_url": "https://xai.test/v1",
		},
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Xai-Request-Id": []string{"xai-image-req"},
		},
		Body: io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"YQ=="}]}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	result, err := svc.ForwardGrokMedia(context.Background(), c, account, GrokMediaEndpointImagesGenerations, "", body, "application/json")
	require.NoError(t, err)
	require.Equal(t, "grok-imagine-image-quality", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "grok-imagine", result.Model)
	require.Equal(t, "grok-imagine", result.BillingModel)
	require.Equal(t, "grok-imagine-image-quality", result.UpstreamModel)
}

func TestForwardGrokMedia_JSONEditImageRewriteAndTooManyImagesError(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)

	// Happy path: legacy image_url shape is rewritten before upstream.
	{
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		body := []byte(`{"model":"grok-imagine-image-quality","prompt":"edit","image":{"image_url":"https://cdn.example/in.png"}}`)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		account := &Account{
			ID:          95,
			Platform:    PlatformGrok,
			Type:        AccountTypeAPIKey,
			Concurrency: 1,
			Credentials: map[string]any{
				"api_key":  "api-key",
				"base_url": "https://xai.test/v1",
			},
		}
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
		}}
		svc := &OpenAIGatewayService{httpUpstream: upstream}

		_, err := svc.ForwardGrokMedia(context.Background(), c, account, GrokMediaEndpointImagesEdits, "", body, "application/json")
		require.NoError(t, err)
		require.Equal(t, "https://cdn.example/in.png", gjson.GetBytes(upstream.lastBody, "image.url").String())
		require.Equal(t, "image_url", gjson.GetBytes(upstream.lastBody, "image.type").String())
	}

	// >3 images → 400 invalid_request_error, no upstream call.
	{
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		body := []byte(`{
			"model":"grok-imagine-image-quality",
			"prompt":"edit",
			"images":[
				"https://cdn.example/1.png",
				"https://cdn.example/2.png",
				"https://cdn.example/3.png",
				"https://cdn.example/4.png"
			]
		}`)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		account := &Account{
			ID:          96,
			Platform:    PlatformGrok,
			Type:        AccountTypeAPIKey,
			Concurrency: 1,
			Credentials: map[string]any{
				"api_key":  "api-key",
				"base_url": "https://xai.test/v1",
			},
		}
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
		}}
		svc := &OpenAIGatewayService{httpUpstream: upstream}

		result, err := svc.ForwardGrokMedia(context.Background(), c, account, GrokMediaEndpointImagesEdits, "", body, "application/json")
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, upstream.lastReq, "must not call upstream when image count exceeds limit")
		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Equal(t, "invalid_request_error", gjson.GetBytes(recorder.Body.Bytes(), "error.type").String())
		require.Contains(t, gjson.GetBytes(recorder.Body.Bytes(), "error.message").String(), "maximum of 3")
	}
}

func TestResolveGrokMediaVideoSeconds_Defaults(t *testing.T) {
	require.Equal(t, 6, resolveGrokMediaVideoSeconds(0, 6, 8))
	require.Equal(t, grokMediaDefaultVideoSeconds, resolveGrokMediaVideoSeconds(0, 0))
	require.Equal(t, grokMediaDefaultVideoSeconds, resolveGrokMediaVideoSeconds())
}
