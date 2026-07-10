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

func TestForwardVideos_CreateWithoutDurationDefaultsVideoSeconds(t *testing.T) {
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
	require.Equal(t, grokMediaDefaultVideoSeconds, result.VideoSeconds,
		"missing duration must default to shared xAI default, not 0/1s under-bill")
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
	require.Equal(t, "https://cdn.example/a.png", gjson.GetBytes(out, "reference_images.0.url").String())
	require.Equal(t, "https://cdn.example/b.png", gjson.GetBytes(out, "reference_images.1.url").String())
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
		Body: io.NopCloser(strings.NewReader(`{"request_id":"video-job-status","status":"completed","model":"grok-imagine-video"}`)),
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
	require.Contains(t, rec.Body.String(), "completed")
	require.Zero(t, result.VideoSeconds)
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
