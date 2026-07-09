package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/reqclientpool"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type failingOpenAIImageWriter struct {
	gin.ResponseWriter
	failAfter int
	writes    int
}

func TestNewOpenAIBackendAPIClient_DisablesCookieJar(t *testing.T) {
	reqclientpool.ResetForTest()

	client, err := newOpenAIBackendAPIClient("")
	require.NoError(t, err)
	require.Nil(t, client.GetClient().Jar)
}

type blockingOpenAIImagesHTTPUpstreamRecorder struct {
	mu       sync.Mutex
	active   int
	maxSeen  int
	requests []*http.Request
	bodies   [][]byte
}

func (u *blockingOpenAIImagesHTTPUpstreamRecorder) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *blockingOpenAIImagesHTTPUpstreamRecorder) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.mu.Lock()
	u.active++
	if u.active > u.maxSeen {
		u.maxSeen = u.active
	}
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		u.bodies = append(u.bodies, append([]byte(nil), b...))
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	u.requests = append(u.requests, req)
	requestIndex := len(u.requests)
	u.mu.Unlock()

	time.Sleep(75 * time.Millisecond)

	u.mu.Lock()
	u.active--
	u.mu.Unlock()

	body := fmt.Sprintf(
		"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":171000030%d,\"usage\":{\"input_tokens\":1,\"output_tokens\":2,\"output_tokens_details\":{\"image_tokens\":1}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"img-%d\",\"revised_prompt\":\"parallel %d\",\"output_format\":\"png\"}]}}\n\n"+
			"data: [DONE]\n\n",
		requestIndex,
		requestIndex,
		requestIndex,
	)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"X-Request-Id": []string{fmt.Sprintf("req_img_parallel_%d", requestIndex)},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}, nil
}

func (u *blockingOpenAIImagesHTTPUpstreamRecorder) maxConcurrency() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.maxSeen
}

type openAIImageSettingsRepoStub struct {
	values map[string]string
}

func resetOpenAIOAuthImageBridgeTransportSettingsTestCache(t *testing.T) {
	t.Helper()
	openAIOAuthImageBridgeTransportSettingsSF.Forget(openAIOAuthImageBridgeTransportSettingsKey)
	openAIOAuthImageBridgeTransportSettingsCache.Store((*cachedOpenAIOAuthImageBridgeTransportSettings)(nil))
	t.Cleanup(func() {
		openAIOAuthImageBridgeTransportSettingsSF.Forget(openAIOAuthImageBridgeTransportSettingsKey)
		openAIOAuthImageBridgeTransportSettingsCache.Store((*cachedOpenAIOAuthImageBridgeTransportSettings)(nil))
	})
}

func (s *openAIImageSettingsRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *openAIImageSettingsRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *openAIImageSettingsRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *openAIImageSettingsRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *openAIImageSettingsRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *openAIImageSettingsRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *openAIImageSettingsRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func (w *failingOpenAIImageWriter) Write(p []byte) (int, error) {
	if w.writes >= w.failAfter {
		return 0, errors.New("write failed: client disconnected")
	}
	w.writes++
	return w.ResponseWriter.Write(p)
}

func newOpenAIImagesParseRequestContext(method, path string, body []byte, contentType string) *gin.Context {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	return c
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_JSON(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"1024x1024","quality":"high","stream":true,"conversation_id":"conv-json","parent_message_id":"parent-json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, "/v1/images/generations", parsed.Endpoint)
	require.Equal(t, "gpt-image-2", parsed.Model)
	require.Equal(t, "draw a cat", parsed.Prompt)
	require.Equal(t, "conv-json", parsed.ConversationID)
	require.Equal(t, "parent-json", parsed.ParentMessageID)
	require.True(t, parsed.Stream)
	require.Equal(t, "1024x1024", parsed.Size)
	require.Equal(t, "1K", parsed.SizeTier)
	require.Equal(t, OpenAIImagesCapabilityNative, parsed.RequiredCapability)
	require.False(t, parsed.Multipart)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_MultipartEdit(t *testing.T) {
	setGinTestMode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1.5"))
	require.NoError(t, writer.WriteField("prompt", "replace background"))
	require.NoError(t, writer.WriteField("size", "1536x1024"))
	require.NoError(t, writer.WriteField("conversation_id", "conv-multipart"))
	require.NoError(t, writer.WriteField("parent_message_id", "parent-multipart"))
	part, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake-image-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body.Bytes())
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, "/v1/images/edits", parsed.Endpoint)
	require.True(t, parsed.Multipart)
	require.Equal(t, "gpt-image-1.5", parsed.Model)
	require.Equal(t, "replace background", parsed.Prompt)
	require.Equal(t, "conv-multipart", parsed.ConversationID)
	require.Equal(t, "parent-multipart", parsed.ParentMessageID)
	require.Equal(t, "1536x1024", parsed.Size)
	require.Equal(t, "2K", parsed.SizeTier)
	require.Len(t, parsed.Uploads, 1)
	require.Equal(t, OpenAIImagesCapabilityNative, parsed.RequiredCapability)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsMalformedInputs(t *testing.T) {
	setGinTestMode()

	tests := []struct {
		name    string
		setup   func() (string, string, []byte)
		wantErr string
	}{
		{
			name: "empty body",
			setup: func() (string, string, []byte) {
				return "/v1/images/generations", "application/json", nil
			},
			wantErr: "request body is empty",
		},
		{
			name: "bad json",
			setup: func() (string, string, []byte) {
				return "/v1/images/generations", "application/json", []byte("{")
			},
			wantErr: "failed to parse request body",
		},
		{
			name: "bad stream type",
			setup: func() (string, string, []byte) {
				return "/v1/images/generations", "application/json", []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":"yes"}`)
			},
			wantErr: "invalid stream field type",
		},
		{
			name: "bad n type",
			setup: func() (string, string, []byte) {
				return "/v1/images/generations", "application/json", []byte(`{"model":"gpt-image-2","prompt":"draw a cat","n":"2"}`)
			},
			wantErr: "invalid n field type",
		},
		{
			name: "bad output compression type",
			setup: func() (string, string, []byte) {
				return "/v1/images/generations", "application/json", []byte(`{"model":"gpt-image-2","prompt":"draw a cat","output_compression":"90"}`)
			},
			wantErr: "invalid output_compression field type",
		},
		{
			name: "bad partial images type",
			setup: func() (string, string, []byte) {
				return "/v1/images/generations", "application/json", []byte(`{"model":"gpt-image-2","prompt":"draw a cat","partial_images":"2"}`)
			},
			wantErr: "invalid partial_images field type",
		},
		{
			name: "bad images type",
			setup: func() (string, string, []byte) {
				return "/v1/images/edits", "application/json", []byte(`{"model":"gpt-image-1.5","prompt":"replace the background","images":{}}`)
			},
			wantErr: "invalid images field type",
		},
		{
			name: "edit missing images",
			setup: func() (string, string, []byte) {
				return "/v1/images/edits", "application/json", []byte(`{"model":"gpt-image-1.5","prompt":"replace the background"}`)
			},
			wantErr: "images[].image_url or images[].file_id is required",
		},
		{
			name: "missing multipart boundary",
			setup: func() (string, string, []byte) {
				return "/v1/images/edits", "multipart/form-data", []byte("ignored")
			},
			wantErr: "multipart boundary is required",
		},
		{
			name: "multipart edit missing images",
			setup: func() (string, string, []byte) {
				var body bytes.Buffer
				writer := multipart.NewWriter(&body)
				require.NoError(t, writer.WriteField("model", "gpt-image-1.5"))
				require.NoError(t, writer.WriteField("prompt", "replace the background"))
				require.NoError(t, writer.Close())
				return "/v1/images/edits", writer.FormDataContentType(), body.Bytes()
			},
			wantErr: "image file is required",
		},
	}

	svc := &OpenAIGatewayService{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, contentType, body := tt.setup()
			c := newOpenAIImagesParseRequestContext(http.MethodPost, path, body, contentType)

			parsed, err := svc.ParseOpenAIImagesRequest(c, body)
			require.Nil(t, parsed)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsImages2APIGenerationsEndpoint(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","response_format":"b64_json"}`)

	c := newOpenAIImagesParseRequestContext(http.MethodPost, "/v1/images2api/generations", body, "application/json")

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "unsupported images endpoint")
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsImages2APIJSONRemoteImages(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-1.5","prompt":"replace background","images":[{"image_url":"https://example.com/source.png"}]}`)

	c := newOpenAIImagesParseRequestContext(http.MethodPost, "/v1/images2api/edits", body, "application/json")
	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "unsupported images endpoint")
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsImages2APIMultipartMask(t *testing.T) {
	setGinTestMode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1.5"))
	require.NoError(t, writer.WriteField("prompt", "replace background"))
	imagePart, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("fake-image-bytes"))
	require.NoError(t, err)
	maskPart, err := writer.CreateFormFile("mask", "mask.png")
	require.NoError(t, err)
	_, err = maskPart.Write([]byte("fake-mask-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c := newOpenAIImagesParseRequestContext(http.MethodPost, "/v1/images2api/edits", body.Bytes(), writer.FormDataContentType())
	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body.Bytes())
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "unsupported images endpoint")
}

func TestResolveOpenAIImageConversationModelByPlanType(t *testing.T) {
	repo := &openAIImageSettingsRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	tests := []struct {
		name     string
		setup    func()
		account  *Account
		expected string
	}{
		{
			name:     "nil account defaults to auto",
			setup:    func() {},
			account:  nil,
			expected: openAIChatGPTConversationModelAuto,
		},
		{
			name:  "free account uses auto",
			setup: func() {},
			account: &Account{
				Credentials: map[string]any{"plan_type": "free"},
			},
			expected: openAIChatGPTConversationModelAuto,
		},
		{
			name:  "plus account uses paid model",
			setup: func() {},
			account: &Account{
				Credentials: map[string]any{"plan_type": "plus"},
			},
			expected: openAIChatGPTConversationModelPaid,
		},
		{
			name:  "pro account uses paid model",
			setup: func() {},
			account: &Account{
				Credentials: map[string]any{"plan_type": " pro "},
			},
			expected: openAIChatGPTConversationModelPaid,
		},
		{
			name:  "team account uses paid model",
			setup: func() {},
			account: &Account{
				Credentials: map[string]any{"plan_type": "TEAM"},
			},
			expected: openAIChatGPTConversationModelPaid,
		},
		{
			name: "legacy settings no longer override paid model",
			setup: func() {
				repo.values["openai_image_web_free_model"] = "free-custom"
				repo.values["openai_image_web_paid_model"] = "paid-custom"
			},
			account: &Account{
				Credentials: map[string]any{"plan_type": "pro"},
			},
			expected: openAIChatGPTConversationModelPaid,
		},
		{
			name:  "unknown plan stays conservative",
			setup: func() {},
			account: &Account{
				Credentials: map[string]any{"plan_type": "enterprise"},
			},
			expected: openAIChatGPTConversationModelPaid,
		},
		{
			name: "unknown plan ignores legacy settings",
			setup: func() {
				repo.values["openai_image_web_free_model"] = "free-custom"
				repo.values["openai_image_web_paid_model"] = "paid-custom"
			},
			account: &Account{
				Credentials: map[string]any{"plan_type": "enterprise"},
			},
			expected: openAIChatGPTConversationModelPaid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.values = map[string]string{}
			tt.setup()
			require.Equal(t, tt.expected, resolveOpenAIImageConversationModel(context.Background(), svc, tt.account))
		})
	}
}

func TestBuildOpenAIImageConversationRequest_UsesPlanBasedModel(t *testing.T) {
	parsed := &OpenAIImagesRequest{Prompt: "draw a cat"}

	freeReq := buildOpenAIImageConversationRequest(
		openAIChatGPTConversationModelAuto,
		parsed,
		"parent-1",
		"message-1",
		nil,
		"",
	)
	require.Equal(t, openAIChatGPTConversationModelAuto, freeReq["model"])

	paidReq := buildOpenAIImageConversationRequest(
		openAIChatGPTConversationModelPaid,
		parsed,
		"parent-2",
		"message-2",
		nil,
		"",
	)
	require.Equal(t, openAIChatGPTConversationModelPaid, paidReq["model"])
	require.Equal(t, "standard", paidReq["thinking_effort"])
	systemHints, ok := paidReq["system_hints"].([]string)
	require.True(t, ok)
	require.Equal(t, []string{"picture_v2"}, systemHints)
}

func TestBuildOpenAIImageConversationRequest_UsesConversationIDWhenPresent(t *testing.T) {
	parsed := &OpenAIImagesRequest{
		Prompt:          "continue drawing",
		ConversationID:  "conv-123",
		ParentMessageID: "parent-123",
	}
	req := buildOpenAIImageConversationRequest(
		openAIChatGPTConversationModelPaid,
		parsed,
		parsed.ParentMessageID,
		"message-123",
		nil,
		"",
	)
	require.Equal(t, "conv-123", req["conversation_id"])
	require.Equal(t, "parent-123", req["parent_message_id"])
}

func TestBuildOpenAIImageConversationRequest_UsesLegacyInpaintingOperation(t *testing.T) {
	parsed := &OpenAIImagesRequest{
		Endpoint:        openAIImages2APIEditsEndpoint,
		OriginalFileID:  "file_original_123",
		OriginalGenID:   "gen_original_456",
		MaskFileID:      "file_mask_789",
		Prompt:          "这里不太好看啊",
		ConversationID:  "conv-edit-123",
		ParentMessageID: "parent-edit-123",
		HasMask:         true,
	}

	req := buildOpenAIImageConversationRequest(
		openAIChatGPTConversationModelPaid,
		parsed,
		parsed.ParentMessageID,
		"message-edit-123",
		nil,
		parsed.MaskFileID,
	)
	require.Equal(t, "success", req["client_prepare_state"])
	systemHints, ok := req["system_hints"].([]string)
	require.True(t, ok)
	require.Equal(t, []string{"picture_v2"}, systemHints)
	require.Equal(t, "conv-edit-123", req["conversation_id"])
	messages, ok := req["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 1)
	message, ok := messages[0].(map[string]any)
	require.True(t, ok)
	content, ok := message["content"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "text", content["content_type"])
	require.Equal(t, []any{"这里不太好看啊"}, content["parts"])
	metadata, ok := message["metadata"].(map[string]any)
	require.True(t, ok)
	metadataSystemHints, ok := metadata["system_hints"].([]string)
	require.True(t, ok)
	require.Equal(t, []string{"picture_v2"}, metadataSystemHints)
	dalle, ok := metadata["dalle"].(map[string]any)
	require.True(t, ok)
	fromClient, ok := dalle["from_client"].(map[string]any)
	require.True(t, ok)
	operation, ok := fromClient["operation"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "inpainting", operation["type"])
	require.Equal(t, "file_original_123", operation["original_file_id"])
	require.Equal(t, "file_mask_789", operation["mask_file_id"])
	require.Equal(t, "gen_original_456", operation["original_gen_id"])
}

func TestResolveOpenAIConversationParentMessageIDFromBody(t *testing.T) {
	body := []byte(`{"current_node":"msg-current","mapping":{"msg-current":{"message":{"id":"msg-current","create_time":2}},"msg-old":{"message":{"id":"msg-old","create_time":1}}}}`)
	require.Equal(t, "msg-current", resolveOpenAIConversationParentMessageIDFromBody(body))
}

func TestOpenAIImageConversationAsyncStatusRequestTarget(t *testing.T) {
	url, targetPath, targetRoute, referer := buildOpenAIConversationAsyncStatusRequestTarget("conv-123")
	require.Equal(t, "https://chatgpt.com/backend-api/conversation/conv-123/async-status", url)
	require.Equal(t, "/backend-api/conversation/conv-123/async-status", targetPath)
	require.Equal(t, "/backend-api/conversation/{conversation_id}/async-status", targetRoute)
	require.Equal(t, "https://chatgpt.com/c/conv-123", referer)
}

func TestBuildOpenAIImageConversationPrepareHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("OpenAI-Sentinel-Chat-Requirements-Token", "should-drop")
	headers.Set("OpenAI-Sentinel-Proof-Token", "should-drop")

	prepareHeaders := buildOpenAIImageConversationPrepareHeaders(headers)
	require.Equal(t, "*/*", prepareHeaders.Get("Accept"))
	require.Equal(t, "application/json", prepareHeaders.Get("Content-Type"))
	require.Equal(t, "no-token", prepareHeaders.Get("x-conduit-token"))
	require.Empty(t, prepareHeaders.Get("OpenAI-Sentinel-Chat-Requirements-Token"))
	require.Empty(t, prepareHeaders.Get("OpenAI-Sentinel-Proof-Token"))
	require.Equal(t, "/backend-api/f/conversation/prepare", prepareHeaders.Get("X-OpenAI-Target-Route"))
}

func TestApplyOpenAIConversationPageReferer(t *testing.T) {
	headers := http.Header{}
	headers.Set("Referer", "https://chatgpt.com/")

	updated := applyOpenAIConversationPageReferer(headers, "conv-123")
	require.Equal(t, "https://chatgpt.com/c/conv-123", updated.Get("Referer"))

	unchanged := applyOpenAIConversationPageReferer(headers, "")
	require.Equal(t, "https://chatgpt.com/", unchanged.Get("Referer"))
}

func TestOpenAIImagesRequestModerationBody_JSONEditIncludesInputImageURLs(t *testing.T) {
	parsed := &OpenAIImagesRequest{
		Endpoint:       openAIImagesEditsEndpoint,
		Prompt:         "replace background",
		InputImageURLs: []string{"https://example.com/source.png"},
		MaskImageURL:   "https://example.com/mask.png",
	}

	input := ExtractContentModerationInput(ContentModerationProtocolOpenAIImages, parsed.ModerationBody())

	require.Equal(t, "replace background", input.Text)
	require.Equal(t, []string{"https://example.com/source.png", "https://example.com/mask.png"}, input.Images)
}

func TestOpenAIImagesRequestModerationBody_MultipartEditIncludesUploadsInMemory(t *testing.T) {
	parsed := &OpenAIImagesRequest{
		Endpoint: openAIImagesEditsEndpoint,
		Prompt:   "replace background",
		Uploads: []OpenAIImagesUpload{{
			FieldName:   "image",
			FileName:    "source.png",
			ContentType: "image/png",
			Data:        []byte("fake-image-bytes"),
		}},
		MaskUpload: &OpenAIImagesUpload{
			FieldName:   "mask",
			FileName:    "mask.png",
			ContentType: "image/png",
			Data:        []byte("fake-mask-bytes"),
		},
	}

	input := ExtractContentModerationInput(ContentModerationProtocolOpenAIImages, parsed.ModerationBody())

	require.Equal(t, "replace background", input.Text)
	require.Equal(t, []string{
		"data:image/png;base64,ZmFrZS1pbWFnZS1ieXRlcw==",
		"data:image/png;base64,ZmFrZS1tYXNrLWJ5dGVz",
	}, input.Images)

	log := (&ContentModerationService{}).buildLog(ContentModerationCheckInput{}, defaultContentModerationConfig(), ContentModerationActionAllow, false, "", 0, nil, input.ExcerptText(), nil, nil, "")
	require.Equal(t, "replace background", log.InputExcerpt)
	require.NotContains(t, log.InputExcerpt, "ZmFrZS")
}

func openAIImagesTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_NormalizesOfficialAndCustomSizes(t *testing.T) {
	setGinTestMode()

	tests := []struct {
		size     string
		wantSize string
		wantTier string
	}{
		{size: "1024x1024", wantSize: "1024x1024", wantTier: "1K"},
		{size: "1536x1024", wantSize: "1536x1024", wantTier: "2K"},
		{size: "1024x1536", wantSize: "1024x1536", wantTier: "2K"},
		{size: "2048x2048", wantSize: "2048x2048", wantTier: "2K"},
		{size: "2048x1152", wantSize: "2048x1152", wantTier: "2K"},
		{size: "3840x2160", wantSize: "3840x2160", wantTier: "4K"},
		{size: "2160x3840", wantSize: "2160x3840", wantTier: "4K"},
		{size: "1024X768", wantSize: "1024x768", wantTier: "2K"},
		{size: "1280x768", wantSize: "1280x768", wantTier: "2K"},
		{size: "2560x1440", wantSize: "2560x1440", wantTier: "2K"},
		{size: "2560x1600", wantSize: "2560x1600", wantTier: "4K"},
		{size: "auto", wantSize: "auto", wantTier: "2K"},
		{size: "1k", wantSize: "1024x1024", wantTier: "1K"},
		{size: "2K", wantSize: "2048x2048", wantTier: "2K"},
		{size: "4k", wantSize: "3840x2160", wantTier: "4K"},
	}

	svc := &OpenAIGatewayService{}
	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"` + tt.size + `"}`)

			req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = req

			parsed, err := svc.ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			require.NotNil(t, parsed)
			require.Equal(t, tt.wantSize, parsed.Size)
			require.Equal(t, tt.wantTier, parsed.SizeTier)
		})
	}
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_AutoCorrectsInvalidSizes(t *testing.T) {
	setGinTestMode()

	tests := []struct {
		size     string
		wantSize string
		wantTier string
	}{
		{size: "2048x1153", wantSize: "2048x1152", wantTier: "2K"},
		{size: "4096x1024", wantSize: "3840x1280", wantTier: "4K"},
		{size: "3840x1024", wantSize: "3744x1248", wantTier: "4K"},
		{size: "512x512", wantSize: "816x816", wantTier: "2K"},
		{size: "3840x3840", wantSize: "2880x2880", wantTier: "4K"},
		{size: "invalid", wantSize: "1024x1024", wantTier: "1K"},
		{size: "999999999999999999999999999x2", wantSize: "1024x1024", wantTier: "1K"},
	}

	svc := &OpenAIGatewayService{}
	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"` + tt.size + `"}`)

			req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = req

			parsed, err := svc.ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			require.NotNil(t, parsed)
			require.Equal(t, tt.wantSize, parsed.Size)
			require.Equal(t, tt.wantTier, parsed.SizeTier)
		})
	}
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_AutoCorrectsInvalidMultipartEditSize(t *testing.T) {
	setGinTestMode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "replace background"))
	require.NoError(t, writer.WriteField("size", "2048x1153"))
	part, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	_, err = part.Write(openAIImagesTestPNG(t, 256, 256))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	parsed, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, body.Bytes())
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, "2048x1152", parsed.Size)
	require.Equal(t, "2K", parsed.SizeTier)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_ResizesMultipartMaskToImage(t *testing.T) {
	setGinTestMode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "replace foreground"))
	imageHeader := make(textproto.MIMEHeader)
	imageHeader.Set("Content-Disposition", `form-data; name="image"; filename="source.png"`)
	imageHeader.Set("Content-Type", "image/png")
	imagePart, err := writer.CreatePart(imageHeader)
	require.NoError(t, err)
	_, err = imagePart.Write(openAIImagesTestPNG(t, 32, 16))
	require.NoError(t, err)
	maskHeader := make(textproto.MIMEHeader)
	maskHeader.Set("Content-Disposition", `form-data; name="mask"; filename="mask.png"`)
	maskHeader.Set("Content-Type", "image/png")
	maskPart, err := writer.CreatePart(maskHeader)
	require.NoError(t, err)
	_, err = maskPart.Write(openAIImagesTestPNG(t, 16, 32))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c := newOpenAIImagesParseRequestContext(http.MethodPost, "/v1/images/edits", body.Bytes(), writer.FormDataContentType())
	parsed, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, body.Bytes())
	require.NoError(t, err)
	require.NotNil(t, parsed.MaskUpload)
	require.Equal(t, 32, parsed.Uploads[0].Width)
	require.Equal(t, 16, parsed.Uploads[0].Height)
	require.Equal(t, 32, parsed.MaskUpload.Width)
	require.Equal(t, 16, parsed.MaskUpload.Height)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(parsed.MaskUpload.Data))
	require.NoError(t, err)
	require.Equal(t, 32, cfg.Width)
	require.Equal(t, 16, cfg.Height)

	forwardBody, forwardContentType, err := normalizeOpenAIImagesForwardBody(body.Bytes(), parsed.ContentType, parsed)
	require.NoError(t, err)
	_, params, err := mime.ParseMediaType(forwardContentType)
	require.NoError(t, err)
	reader := multipart.NewReader(bytes.NewReader(forwardBody), params["boundary"])
	var forwardedMask []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if part.FormName() == "mask" {
			forwardedMask, err = io.ReadAll(part)
			require.NoError(t, err)
		}
		require.NoError(t, part.Close())
	}
	require.NotEmpty(t, forwardedMask)
	cfg, _, err = image.DecodeConfig(bytes.NewReader(forwardedMask))
	require.NoError(t, err)
	require.Equal(t, 32, cfg.Width)
	require.Equal(t, 16, cfg.Height)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsUndecodableMultipartMaskWhenResizeRequired(t *testing.T) {
	setGinTestMode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "replace foreground"))
	imageHeader := make(textproto.MIMEHeader)
	imageHeader.Set("Content-Disposition", `form-data; name="image"; filename="source.png"`)
	imageHeader.Set("Content-Type", "image/png")
	imagePart, err := writer.CreatePart(imageHeader)
	require.NoError(t, err)
	_, err = imagePart.Write(openAIImagesTestPNG(t, 32, 16))
	require.NoError(t, err)
	maskHeader := make(textproto.MIMEHeader)
	maskHeader.Set("Content-Disposition", `form-data; name="mask"; filename="mask.tiff"`)
	maskHeader.Set("Content-Type", "image/tiff")
	maskHeader.Set("X-Image-Width", "16")
	maskHeader.Set("X-Image-Height", "32")
	maskPart, err := writer.CreatePart(maskHeader)
	require.NoError(t, err)
	_, err = maskPart.Write([]byte("not a supported image"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c := newOpenAIImagesParseRequestContext(http.MethodPost, "/v1/images/edits", body.Bytes(), writer.FormDataContentType())
	parsed, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, body.Bytes())
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "decode mask image")
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsMultipartMaskResizeAbovePixelLimit(t *testing.T) {
	setGinTestMode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "replace foreground"))
	imageHeader := make(textproto.MIMEHeader)
	imageHeader.Set("Content-Disposition", `form-data; name="image"; filename="source.png"`)
	imageHeader.Set("Content-Type", "image/png")
	imageHeader.Set("X-Image-Width", "4096")
	imageHeader.Set("X-Image-Height", "4096")
	imagePart, err := writer.CreatePart(imageHeader)
	require.NoError(t, err)
	_, err = imagePart.Write(openAIImagesTestPNG(t, 16, 16))
	require.NoError(t, err)
	maskHeader := make(textproto.MIMEHeader)
	maskHeader.Set("Content-Disposition", `form-data; name="mask"; filename="mask.png"`)
	maskHeader.Set("Content-Type", "image/png")
	maskPart, err := writer.CreatePart(maskHeader)
	require.NoError(t, err)
	_, err = maskPart.Write(openAIImagesTestPNG(t, 16, 16))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c := newOpenAIImagesParseRequestContext(http.MethodPost, "/v1/images/edits", body.Bytes(), writer.FormDataContentType())
	parsed, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, body.Bytes())
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "mask resize target exceeds")
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_LegacyImageModelUnknownSizePassthrough(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-1.5","prompt":"draw a cat","size":"2048x1152"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, "2048x1152", parsed.Size)
	require.Equal(t, "2K", parsed.SizeTier)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_MultipartEditWithMaskAndNativeOptions(t *testing.T) {
	setGinTestMode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1.5"))
	require.NoError(t, writer.WriteField("prompt", "replace foreground"))
	require.NoError(t, writer.WriteField("output_format", "webp"))
	require.NoError(t, writer.WriteField("input_fidelity", "high"))
	require.NoError(t, writer.WriteField("output_compression", "80"))
	require.NoError(t, writer.WriteField("partial_images", "2"))

	imageHeader := make(textproto.MIMEHeader)
	imageHeader.Set("Content-Disposition", `form-data; name="image"; filename="source.png"`)
	imageHeader.Set("Content-Type", "image/png")
	imagePart, err := writer.CreatePart(imageHeader)
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("source-image-bytes"))
	require.NoError(t, err)

	maskHeader := make(textproto.MIMEHeader)
	maskHeader.Set("Content-Disposition", `form-data; name="mask"; filename="mask.png"`)
	maskHeader.Set("Content-Type", "image/png")
	maskPart, err := writer.CreatePart(maskHeader)
	require.NoError(t, err)
	_, err = maskPart.Write([]byte("mask-image-bytes"))
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body.Bytes())
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Len(t, parsed.Uploads, 1)
	require.NotNil(t, parsed.MaskUpload)
	require.True(t, parsed.HasMask)
	require.Equal(t, "webp", parsed.OutputFormat)
	require.Equal(t, "high", parsed.InputFidelity)
	require.NotNil(t, parsed.OutputCompression)
	require.Equal(t, 80, *parsed.OutputCompression)
	require.NotNil(t, parsed.PartialImages)
	require.Equal(t, 2, *parsed.PartialImages)
	require.Equal(t, OpenAIImagesCapabilityNative, parsed.RequiredCapability)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_MultipartEditAllowsFilesAbove20MB(t *testing.T) {
	setGinTestMode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "replace foreground"))

	imageHeader := make(textproto.MIMEHeader)
	imageHeader.Set("Content-Disposition", `form-data; name="image"; filename="source.png"`)
	imageHeader.Set("Content-Type", "image/png")
	imagePart, err := writer.CreatePart(imageHeader)
	require.NoError(t, err)
	_, err = imagePart.Write(bytes.Repeat([]byte("a"), (21<<20)+1))
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body.Bytes())
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Len(t, parsed.Uploads, 1)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_PromptOnlyDefaultsRemainBasic(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"prompt":"draw a cat"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, "gpt-image-2", parsed.Model)
	require.Equal(t, OpenAIImagesCapabilityBasic, parsed.RequiredCapability)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_ExplicitSizeRequiresNativeCapability(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"prompt":"draw a cat","size":"1024x1024"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, OpenAIImagesCapabilityNative, parsed.RequiredCapability)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsNonImageModel(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-5.4","prompt":"draw a cat"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.Nil(t, parsed)
	require.ErrorContains(t, err, `images endpoint requires an image model, got "gpt-5.4"`)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_JSONEditURLs(t *testing.T) {
	setGinTestMode()
	body := []byte(`{
			"model":"gpt-image-1.5",
			"prompt":"replace the background",
			"images":[{"image_url":"https://example.com/source.png"}],
			"mask":{"image_url":"https://example.com/mask.png"},
			"input_fidelity":"high",
			"output_compression":90,
			"partial_images":2,
			"response_format":"b64_json"
		}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, []string{"https://example.com/source.png"}, parsed.InputImageURLs)
	require.Equal(t, "https://example.com/mask.png", parsed.MaskImageURL)
	require.Equal(t, "high", parsed.InputFidelity)
	require.NotNil(t, parsed.OutputCompression)
	require.Equal(t, 90, *parsed.OutputCompression)
	require.NotNil(t, parsed.PartialImages)
	require.Equal(t, 2, *parsed.PartialImages)
	require.True(t, parsed.HasMask)
	require.Equal(t, OpenAIImagesCapabilityNative, parsed.RequiredCapability)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_JSONEditFileIDs(t *testing.T) {
	setGinTestMode()
	body := []byte(`{
			"model":"gpt-image-2",
			"prompt":"replace the background",
			"original_gen_id":"gen_source_789",
			"images":[{"file_id":"file_source_123"}],
			"mask":{"file_id":"file_mask_456"},
			"response_format":"b64_json"
		}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Equal(t, "file_source_123", parsed.OriginalFileID)
	require.Equal(t, "gen_source_789", parsed.OriginalGenID)
	require.Equal(t, []string{"file_source_123"}, parsed.InputImageFileIDs)
	require.Equal(t, "file_mask_456", parsed.MaskFileID)
	require.True(t, parsed.HasMask)
	require.False(t, parsed.UsesLegacyInpainting())
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsResponseFormatURL(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","response_format":"url"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "response_format=url is not supported for gpt-image models; use b64_json")
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsOutOfRangeN(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","n":11}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "n must be between 1 and 10")
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsOutOfRangePartialImages(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","partial_images":4}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "partial_images must be between 0 and 3")
}

func TestAccountSupportsOpenAIImageCapability_EmptyRequirementDoesNotRejectGrok(t *testing.T) {
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
	}

	require.True(t, account.SupportsOpenAIImageCapability(""))
	require.True(t, account.SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic))
	require.True(t, account.SupportsOpenAIImageCapability(OpenAIImagesCapabilityNative))

	apiKeyAccount := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
	}
	require.False(t, apiKeyAccount.SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic))
}

func TestAccountSupportsOpenAIEndpointCapability(t *testing.T) {
	t.Run("OpenAI APIKey 默认兼容 chat 和 embeddings", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
		}

		require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
		require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
		require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos))
	})

	t.Run("Grok 默认兼容 videos, images (subscription OAuth), Responses; chat rejected at route", func(t *testing.T) {
		account := &Account{
			Platform: PlatformGrok,
			Type:     AccountTypeOAuth,
		}

		require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos))
		require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityResponsesIngress))
		require.True(t, account.SupportsOpenAIImageRoute("native"))
		require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
	})

	t.Run("OpenAI OAuth 默认仅兼容 chat", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
		}

		require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
		require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
	})

	t.Run("显式列表支持同时声明 chat 和 embeddings", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Credentials: map[string]any{
				"openai_capabilities": []any{"chat_completions", "embeddings"},
			},
		}

		require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
		require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
	})

	t.Run("OpenAI 显式声明 videos 也不开放视频能力", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"openai_capabilities": []any{"videos"},
			},
		}

		require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos))
	})

	t.Run("显式列表只声明 chat 时不支持 embeddings", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"openai_capabilities": []any{"chat_completions"},
			},
		}

		require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
		require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
	})

	t.Run("显式 map 支持单独关闭 chat 并开启 embeddings", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"openai_capabilities": map[string]any{
					"chat_completions": false,
					"embeddings":       true,
				},
			},
		}

		require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
		require.True(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))
	})

	t.Run("未知能力不应默认放行", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
		}

		require.False(t, account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapability("unknown")))
	})
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsOutOfRangeOutputCompression(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","output_compression":101}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "output_compression must be between 0 and 100")
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsInputFidelityForGPTImage2(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","input_fidelity":"high"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "input_fidelity is not supported for gpt-image-2")
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsTransparentBackgroundWithJPEG(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-1","prompt":"draw a cat","background":"transparent","output_format":"jpeg"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "background=transparent requires output_format png or webp")
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_StripsTransparentBackgroundForGPTImage2(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","background":"transparent","output_format":"webp"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	require.Empty(t, parsed.Background)
	require.Equal(t, "webp", parsed.OutputFormat)
}

func TestOpenAIGatewayServiceParseOpenAIImagesRequest_RejectsOutputCompressionWithPNG(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","output_format":"png","output_compression":80}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.Nil(t, parsed)
	require.ErrorContains(t, err, "output_compression is only supported when output_format is jpeg or webp")
}

func TestCollectOpenAIImagePointers_RecognizesDirectAssets(t *testing.T) {
	items := collectOpenAIImagePointers([]byte(`{
		"revised_prompt": "cat astronaut",
		"parts": [
			{"b64_json":"QUJD"},
			{"download_url":"https://files.example.com/image.png?sig=1"},
			{"asset_pointer":"file-service://file_123"}
		]
	}`))

	require.Len(t, items, 3)
	var sawBase64, sawURL, sawPointer bool
	for _, item := range items {
		if item.B64JSON == "QUJD" {
			sawBase64 = true
			require.Equal(t, "cat astronaut", item.Prompt)
		}
		if item.DownloadURL == "https://files.example.com/image.png?sig=1" {
			sawURL = true
		}
		if item.Pointer == "file-service://file_123" {
			sawPointer = true
		}
	}
	require.True(t, sawBase64)
	require.True(t, sawURL)
	require.True(t, sawPointer)
}

func TestResolveOpenAIImageBytes_PrefersInlineBase64(t *testing.T) {
	data, err := resolveOpenAIImageBytes(context.Background(), nil, nil, nil, "", openAIImagePointerInfo{
		B64JSON: "data:image/png;base64,QUJD",
	}, openAIUpstreamErrorBodyReadLimit)
	require.NoError(t, err)
	require.Equal(t, []byte("ABC"), data)
}

func TestNewOpenAIImageStatusError_UsesProvidedReadLimit(t *testing.T) {
	padding := strings.Repeat("x", int(openAIUpstreamErrorBodyReadLimit)+1024)
	body := fmt.Sprintf(`{"error":{"padding":"%s","message":"diagnostic-marker"}}`, padding)
	resp := &req.Response{Response: &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}}

	err := newOpenAIImageStatusError(resp, "download image bytes failed", int64(len(body)))
	require.Error(t, err)
	require.Equal(t, "diagnostic-marker", err.Error())

	var statusErr *openAIImageStatusError
	require.ErrorAs(t, err, &statusErr)
	require.Len(t, statusErr.ResponseBody, len(body))
}

func TestOpenAIUpstreamErrorBodyReadLimitForConfig_RespectsDiagnosticLimit(t *testing.T) {
	cfg := &config.Config{Gateway: config.GatewayConfig{
		LogUpstreamErrorBody:         true,
		LogUpstreamErrorBodyMaxBytes: int(openAIUpstreamErrorBodyReadLimit) + 1024,
	}}

	require.Equal(t, int64(cfg.Gateway.LogUpstreamErrorBodyMaxBytes), openAIUpstreamErrorBodyReadLimitForConfig(cfg))
}

func TestAccountSupportsOpenAIImageCapability_OAuthSupportsNative(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}

	require.True(t, account.SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic))
	require.True(t, account.SupportsOpenAIImageCapability(OpenAIImagesCapabilityNative))
}

func TestAccountSupportsOpenAIImageCapability_APIKeySupportsNative(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	}

	require.True(t, account.SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic))
	require.True(t, account.SupportsOpenAIImageCapability(OpenAIImagesCapabilityNative))
}

func TestIsOpenAIImageGenerationToolUnsupportedError(t *testing.T) {
	body := []byte(`{"error":{"message":"Tool choice 'image_generation' not found in 'tools' parameter.","type":"invalid_request_error"}}`)

	require.True(t, isOpenAIImageGenerationToolUnsupportedError(
		http.StatusBadRequest,
		"Tool choice 'image_generation' not found in 'tools' parameter.",
		body,
	))
	require.False(t, isOpenAIImageGenerationToolUnsupportedError(
		http.StatusBadRequest,
		"unsupported response_format",
		[]byte(`{"error":{"message":"unsupported response_format"}}`),
	))
	require.False(t, isOpenAIImageGenerationToolUnsupportedError(
		http.StatusInternalServerError,
		"Tool choice 'image_generation' not found in 'tools' parameter.",
		body,
	))
}

func TestBuildOpenAIImagesURL_HandlesVersionedBaseURL(t *testing.T) {
	require.Equal(t,
		"https://image-upstream.example/v1/images/generations",
		buildOpenAIImagesURL("https://image-upstream.example/v1", openAIImagesGenerationsEndpoint),
	)
	require.Equal(t,
		"https://image-upstream.example/v1/images/edits",
		buildOpenAIImagesURL("https://image-upstream.example/v1/", openAIImagesEditsEndpoint),
	)
	require.Equal(t,
		"https://image-upstream.example/v1/images/generations",
		buildOpenAIImagesURL("https://image-upstream.example", openAIImagesGenerationsEndpoint),
	)
	require.Equal(t,
		"https://image-upstream.example/v1/images/generations",
		buildOpenAIImagesURL("https://image-upstream.example/v1/images/generations", openAIImagesGenerationsEndpoint),
	)
}

type openAIImageTestSSEEvent struct {
	Name string
	Data string
}

func parseOpenAIImageTestSSEEvents(body string) []openAIImageTestSSEEvent {
	chunks := strings.Split(body, "\n\n")
	events := make([]openAIImageTestSSEEvent, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		var event openAIImageTestSSEEvent
		for _, line := range strings.Split(chunk, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				event.Name = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
			case strings.HasPrefix(line, "data: "):
				event.Data = strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			}
		}
		if event.Name != "" || event.Data != "" {
			events = append(events, event)
		}
	}
	return events
}

func findOpenAIImageTestSSEEvent(events []openAIImageTestSSEEvent, name string) (openAIImageTestSSEEvent, bool) {
	for _, event := range events {
		if event.Name == name {
			return event, true
		}
	}
	return openAIImageTestSSEEvent{}, false
}

func TestOpenAIGatewayServiceForwardImages_OAuthUsesResponsesAPI(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"1024x1024","quality":"high"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("api_key", &APIKey{ID: 42})

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"req_img_123"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000000,\"usage\":{\"input_tokens\":11,\"output_tokens\":22,\"input_tokens_details\":{\"cached_tokens\":3},\"output_tokens_details\":{\"image_tokens\":7}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"aGVsbG8=\",\"revised_prompt\":\"draw a cat\",\"output_format\":\"png\",\"quality\":\"high\",\"size\":\"1024x1024\"}]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc.httpUpstream = upstream

	account := &Account{
		ID:       1,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "token-123",
			"chatgpt_account_id": "acct-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "gpt-image-2", result.Model)
	require.Equal(t, "gpt-image-2", result.UpstreamModel)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 22, result.Usage.OutputTokens)
	require.Equal(t, 7, result.Usage.ImageOutputTokens)
	require.Empty(t, result.TokenBillingModel)

	require.NotNil(t, upstream.lastReq)
	require.Equal(t, chatgptCodexURL, upstream.lastReq.URL.String())
	require.Equal(t, "chatgpt.com", upstream.lastReq.Host)
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, "acct-123", upstream.lastReq.Header.Get("chatgpt-account-id"))
	require.Equal(t, "responses=experimental", upstream.lastReq.Header.Get("OpenAI-Beta"))

	require.Equal(t, openAIImagesResponsesMainModel, gjson.GetBytes(upstream.lastBody, "model").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "image_generation", gjson.GetBytes(upstream.lastBody, "tools.0.type").String())
	require.Equal(t, "generate", gjson.GetBytes(upstream.lastBody, "tools.0.action").String())
	require.Equal(t, "gpt-image-2", gjson.GetBytes(upstream.lastBody, "tools.0.model").String())
	require.Equal(t, "1024x1024", gjson.GetBytes(upstream.lastBody, "tools.0.size").String())
	require.Equal(t, "high", gjson.GetBytes(upstream.lastBody, "tools.0.quality").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "tools.0.n").Exists())
	require.Equal(t, "draw a cat", gjson.GetBytes(upstream.lastBody, "input.0.content.0.text").String())

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "gpt-image-2", gjson.Get(rec.Body.String(), "model").String())
	require.Equal(t, "aGVsbG8=", gjson.Get(rec.Body.String(), "data.0.b64_json").String())
	require.Equal(t, "draw a cat", gjson.Get(rec.Body.String(), "data.0.revised_prompt").String())
}

func TestOpenAIGatewayServiceForwardImages_OAuthRecordsEmptyOutputTimeline(t *testing.T) {
	resetGatewayDebugTimelineStateForTest(t)
	setGinTestMode()
	dir := filepath.Join(t.TempDir(), "timeline")
	settingService := testGatewayDebugTimelineSettingService(t, dir)
	require.NoError(t, settingService.settingRepo.SetMultiple(context.TODO(), map[string]string{
		SettingKeyGatewayDebugTimelineIncludeBody: "true",
		SettingKeyGatewayDebugTimelineBodyMaxKB:   "32",
	}))
	gatewayDebugTimelineSettingsSF.Forget("gateway_debug_timeline")
	gatewayDebugTimelineSettingsCache.Store(&cachedGatewayDebugTimelineSettings{
		settings:  DefaultGatewayDebugTimelineSettings(),
		expiresAt: 0,
	})

	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"1024x1024","quality":"high"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("api_key", &APIKey{ID: 42})

	svc := &OpenAIGatewayService{settingService: settingService}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"req_img_empty"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000000,\"usage\":{\"input_tokens\":11,\"output_tokens\":22},\"output\":[]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc.httpUpstream = upstream
	account := &Account{
		ID:       7,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Contains(t, string(failoverErr.ResponseBody), "upstream did not return image output")

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "empty_image_output", events[0].Kind)
	require.Equal(t, "req_img_empty", events[0].UpstreamRequestID)
	require.Contains(t, events[0].UpstreamResponseBody, "response.completed")

	content := readGatewayDebugTimelineLog(t, dir)
	require.Contains(t, content, `"stage":"upstream_request_body"`)
	require.Contains(t, content, `"stage":"upstream_response_body"`)
	require.Contains(t, content, `"endpoint_kind":"images"`)
	require.Contains(t, content, `"account_id":7`)
	require.Contains(t, content, `"upstream_request_id":"req_img_empty"`)
	require.Contains(t, content, "response.completed")
	require.Contains(t, content, "upstream did not return image output")
}

func TestOpenAIGatewayServiceForwardImages_OAuthStreamingUnexpectedEOFFailoversBeforeOutput(t *testing.T) {
	resetGatewayDebugTimelineStateForTest(t)
	setGinTestMode()
	dir := filepath.Join(t.TempDir(), "timeline")
	settingService := testGatewayDebugTimelineSettingService(t, dir)
	require.NoError(t, settingService.settingRepo.SetMultiple(context.TODO(), map[string]string{
		SettingKeyGatewayDebugTimelineIncludeBody: "true",
		SettingKeyGatewayDebugTimelineBodyMaxKB:   "32",
	}))
	gatewayDebugTimelineSettingsSF.Forget("gateway_debug_timeline")
	gatewayDebugTimelineSettingsCache.Store(&cachedGatewayDebugTimelineSettings{
		settings:  DefaultGatewayDebugTimelineSettings(),
		expiresAt: 0,
	})

	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":true,"response_format":"b64_json"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{settingService: settingService}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	svc.httpUpstream = &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"req_img_stream_eof"},
			},
			Body: io.NopCloser(iotest.ErrReader(io.ErrUnexpectedEOF)),
		},
	}
	account := &Account{
		ID:       8,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, failoverErr.RetryableOnSameAccount)
	require.Contains(t, string(failoverErr.ResponseBody), "unexpected EOF")
	require.Empty(t, rec.Body.String())

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "stream_error", events[0].Kind)
	require.Equal(t, "req_img_stream_eof", events[0].UpstreamRequestID)
	require.Contains(t, events[0].Message, "unexpected EOF")

	content := readGatewayDebugTimelineLog(t, dir)
	require.Contains(t, content, `"stage":"upstream_response_body"`)
	require.Contains(t, content, `"body_event":"stream_error"`)
	require.Contains(t, content, "unexpected EOF")
}

func TestOpenAIGatewayServiceForwardImages_OAuthAppliesUpstreamTransportExperimentFlags(t *testing.T) {
	resetOpenAIOAuthImageBridgeTransportSettingsTestCache(t)
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"1024x1024"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("api_key", &APIKey{ID: 42})

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"req_img_transport_flags"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000000,\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"aGVsbG8=\"}]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIOAuthImageBridgeDisableKeepAlives:   true,
				OpenAIOAuthImageBridgeFreshUpstreamClient: true,
			},
		},
		httpUpstream: upstream,
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       1,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "token-123",
			"chatgpt_account_id": "acct-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)

	opts := HTTPUpstreamRequestOptionsFromContext(upstream.lastReq.Context())
	require.True(t, opts.DisableKeepAlives)
	require.True(t, opts.FreshClient)
}

func TestOpenAIGatewayServiceForwardImages_OAuthAppliesUpstreamTransportExperimentFlagsFromSystemSettings(t *testing.T) {
	resetOpenAIOAuthImageBridgeTransportSettingsTestCache(t)
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","size":"1024x1024"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("api_key", &APIKey{ID: 42})

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"req_img_transport_flags_settings"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000000,\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"aGVsbG8=\"}]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}

	settingSvc := NewSettingService(&openAIImageSettingsRepoStub{values: map[string]string{
		SettingKeyOpenAIOAuthImageBridgeDisableKeepAlives:   "true",
		SettingKeyOpenAIOAuthImageBridgeFreshUpstreamClient: "true",
	}}, &config.Config{})
	svc := &OpenAIGatewayService{
		cfg:            &config.Config{},
		httpUpstream:   upstream,
		settingService: settingSvc,
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       1,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "token-123",
			"chatgpt_account_id": "acct-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)

	opts := HTTPUpstreamRequestOptionsFromContext(upstream.lastReq.Context())
	require.True(t, opts.DisableKeepAlives)
	require.True(t, opts.FreshClient)
}

func TestOpenAIGatewayServiceForwardImages_OAuthFansOutMultipleImages(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","n":2}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	upstream := &openAIImagesHTTPUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
					"X-Request-Id": []string{"req_img_n_1"},
				},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000100,\"usage\":{\"input_tokens\":3,\"output_tokens\":5,\"output_tokens_details\":{\"image_tokens\":2}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"Zmlyc3Q=\",\"revised_prompt\":\"draw a cat one\",\"output_format\":\"png\"}]}}\n\n" +
						"data: [DONE]\n\n",
				)),
			},
			{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
					"X-Request-Id": []string{"req_img_n_2"},
				},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000101,\"usage\":{\"input_tokens\":4,\"output_tokens\":6,\"output_tokens_details\":{\"image_tokens\":3}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"c2Vjb25k\",\"revised_prompt\":\"draw a cat two\",\"output_format\":\"png\"}]}}\n\n" +
						"data: [DONE]\n\n",
				)),
			},
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       1,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "token-123",
			"chatgpt_account_id": "acct-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, result.ImageCount)
	require.Len(t, upstream.requests, 2)
	created := gjson.Get(rec.Body.String(), "created").Int()
	require.Contains(t, []int64{1710000100, 1710000101}, created)
	data := gjson.Get(rec.Body.String(), "data").Array()
	require.Len(t, data, 2)
	gotImages := []string{
		gjson.Get(data[0].Raw, "b64_json").String(),
		gjson.Get(data[1].Raw, "b64_json").String(),
	}
	require.ElementsMatch(t, []string{"Zmlyc3Q=", "c2Vjb25k"}, gotImages)
	gotPrompts := []string{
		gjson.Get(data[0].Raw, "revised_prompt").String(),
		gjson.Get(data[1].Raw, "revised_prompt").String(),
	}
	require.ElementsMatch(t, []string{"draw a cat one", "draw a cat two"}, gotPrompts)
	require.Equal(t, 7, result.Usage.InputTokens)
	require.Equal(t, 11, result.Usage.OutputTokens)
	require.Equal(t, 5, result.Usage.ImageOutputTokens)
	for _, sent := range upstream.bodies {
		require.False(t, gjson.GetBytes(sent, "tools.0.n").Exists())
	}
}

func TestOpenAIGatewayServiceForwardImages_OAuthFanOutRunsRequestsConcurrently(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","n":3}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	upstream := &blockingOpenAIImagesHTTPUpstreamRecorder{}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       11,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 3, result.ImageCount)
	require.Greater(t, upstream.maxConcurrency(), 1, "fan-out should not serialize all upstream image requests")
	require.Len(t, upstream.requests, 3)
	require.Len(t, gjson.Get(rec.Body.String(), "data").Array(), 3)
	for _, sent := range upstream.bodies {
		require.False(t, gjson.GetBytes(sent, "tools.0.n").Exists())
	}
}

func TestOpenAIGatewayServiceForwardImages_OAuthFanOutReturnsPartialSuccessOnLaterFailure(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","n":2}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	upstream := &openAIImagesHTTPUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
					"X-Request-Id": []string{"req_img_partial_1"},
				},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000200,\"usage\":{\"input_tokens\":3,\"output_tokens\":5,\"output_tokens_details\":{\"image_tokens\":2}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"cGFydGlhbA==\",\"revised_prompt\":\"draw a partial cat\",\"output_format\":\"png\"}]}}\n\n" +
						"data: [DONE]\n\n",
				)),
			},
			{
				StatusCode: http.StatusTooManyRequests,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"X-Request-Id": []string{"req_img_partial_2"},
				},
				Body: io.NopCloser(strings.NewReader(`{"error":{"message":"rate limit on second image","type":"rate_limit_error"}}`)),
			},
		},
	}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       9,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "req_img_partial_1", result.RequestID)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, int64(1710000200), gjson.Get(rec.Body.String(), "created").Int())
	require.Equal(t, "cGFydGlhbA==", gjson.Get(rec.Body.String(), "data.0.b64_json").String())
	require.Equal(t, 1, len(gjson.Get(rec.Body.String(), "data").Array()))
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
	require.Equal(t, 2, result.Usage.ImageOutputTokens)

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.NotEmpty(t, events)
	require.Equal(t, "failover", events[len(events)-1].Kind)
	require.Equal(t, "req_img_partial_2", events[len(events)-1].UpstreamRequestID)
}

func TestOpenAIGatewayServiceForwardImages_OAuthNonStreamServerErrorReturnsFailoverBeforeFlush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{
		httpUpstream: &httpUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
					"X-Request-Id": []string{"req_img_server_error"},
				},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.created\",\"response\":{\"created_at\":1710000021}}\n\n" +
						"data: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"The image service is temporarily unavailable.\"}}\n\n",
				)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	account := &Account{
		ID:       21,
		Name:     "openai-oauth-server-error",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "temporarily unavailable")
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "failover", events[0].Kind)
	require.Equal(t, account.ID, events[0].AccountID)
	require.Equal(t, http.StatusBadGateway, events[0].UpstreamStatusCode)
}

func TestOpenAIGatewayServiceForwardImages_OAuthStreamServerErrorAfterFlushDoesNotFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":true,"response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{
		httpUpstream: &httpUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
					"X-Request-Id": []string{"req_img_server_error_after_partial"},
				},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.image_generation_call.partial_image\",\"partial_image_b64\":\"cGFydGlhbA==\",\"partial_image_index\":0,\"output_format\":\"png\"}\n\n" +
						"data: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"code\":\"server_error\",\"message\":\"The image service failed after partial output.\"}}\n\n",
				)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	account := &Account{
		ID:       22,
		Name:     "openai-oauth-partial-server-error",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	var upstreamErr *OpenAIImagesUpstreamError
	require.ErrorAs(t, err, &upstreamErr)
	require.True(t, IsOpenAIImagesRetryableUpstreamError(upstreamErr))
	require.True(t, c.Writer.Written())
	require.Contains(t, rec.Body.String(), "event: image_generation.partial_image")
	require.Contains(t, rec.Body.String(), "event: error")
	require.Contains(t, rec.Body.String(), "failed after partial output")

	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "retry_exhausted_failover", events[0].Kind)
	require.Equal(t, account.ID, events[0].AccountID)
}

func TestOpenAIImagesSSEClientErrorsAreNotRetryable(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantStatus int
	}{
		{
			name:       "invalid request",
			payload:    `{"type":"error","error":{"type":"invalid_request_error","code":"invalid_value","message":"bad size"}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "content policy",
			payload:    `{"type":"error","error":{"type":"image_generation_user_error","code":"content_policy_violation","message":"blocked"}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rate limit remains distinct from server error",
			payload:    `{"type":"error","error":{"type":"rate_limit_exceeded","code":"rate_limit_exceeded","message":"try again"}}`,
			wantStatus: http.StatusTooManyRequests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstreamErr := openAIImagesUpstreamErrorFromSSEPayload([]byte(tt.payload))
			require.NotNil(t, upstreamErr)
			require.Equal(t, tt.wantStatus, upstreamErr.StatusCode)
			require.False(t, IsOpenAIImagesRetryableUpstreamError(upstreamErr))
		})
	}
}

func TestOpenAIGatewayServiceForwardImages_APIKeyGenerationUsesConfiguredV1BaseURL(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
		httpUpstream: &openAIImagesHTTPUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"X-Request-Id": []string{"req_img_apikey"},
				},
				Body: io.NopCloser(strings.NewReader(`{"created":1710000007,"data":[{"b64_json":"aGVsbG8=","revised_prompt":"draw a cat"}]}`)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       6,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://image-upstream.example/v1",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "gpt-image-2", result.Model)
	require.Equal(t, "gpt-image-2", result.UpstreamModel)

	upstream, ok := svc.httpUpstream.(*openAIImagesHTTPUpstreamRecorder)
	require.True(t, ok)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://image-upstream.example/v1/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer test-api-key", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	require.Equal(t, "gpt-image-2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "aGVsbG8=", gjson.Get(rec.Body.String(), "data.0.b64_json").String())
}

func TestOpenAIGatewayServiceForwardImages_GrokOAuthUsesNativeImagesAPI(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"grok-imagine-1","prompt":"draw a cat","response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-Id": []string{"req_img_grok_native"},
			},
			Body: io.NopCloser(strings.NewReader(`{"created":1710000008,"data":[{"b64_json":"aGVsbG8=","revised_prompt":"draw a cat"}]}`)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       7,
		Name:     "grok-oauth",
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "xai-token",
			"base_url":     "https://api.x.ai",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	// Legacy alias grok-imagine-1 maps to official quality model for upstream.
	require.Equal(t, "grok-imagine-1", result.Model)
	require.Equal(t, "grok-imagine-image-quality", result.UpstreamModel)

	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://api.x.ai/v1/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer xai-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	require.Equal(t, "grok-imagine-image-quality", gjson.GetBytes(upstream.lastBody, "model").String())
	require.NotContains(t, upstream.lastReq.URL.String(), "chatgpt.com/backend-api/codex")
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestOpenAIGatewayServiceForwardImages_APIKeyCyberPolicyMarksAndDoesNotFailover(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"blocked image","response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	upstreamBody := `{"error":{"code":"cyber_policy","message":"blocked by policy"}}`
	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
		httpUpstream: &openAIImagesHTTPUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusForbidden,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"X-Request-Id": []string{"req_img_cyber"},
				},
				Body: io.NopCloser(strings.NewReader(upstreamBody)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       7,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "test-api-key",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")

	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, upstreamBody, rec.Body.String())
	mark := GetOpsCyberPolicy(c)
	require.NotNil(t, mark)
	require.Equal(t, "cyber_policy", mark.Code)
	require.Equal(t, "blocked by policy", mark.Message)
	require.Equal(t, http.StatusForbidden, mark.UpstreamStatus)
}

func TestOpenAIGatewayServiceForwardImages_OAuthNonStreamCyberPolicyDoesNotFailover(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"blocked image","response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	upstreamBody := "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_img_cyber\",\"error\":{\"code\":\"cyber_policy\",\"message\":\"blocked by policy\"},\"usage\":{\"input_tokens\":13,\"output_tokens\":0}}}\n\n"
	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
		httpUpstream: &openAIImagesHTTPUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
					"X-Request-Id": []string{"req_img_oauth_cyber"},
				},
				Body: io.NopCloser(strings.NewReader(upstreamBody)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       8,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "test-oauth-token",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")

	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	var upstreamErr *OpenAIImagesUpstreamError
	require.ErrorAs(t, err, &upstreamErr)
	require.False(t, IsOpenAIImagesRetryableUpstreamError(upstreamErr))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "cyber_policy", gjson.Get(rec.Body.String(), "error.code").String())
	mark := GetOpsCyberPolicy(c)
	require.NotNil(t, mark)
	require.Equal(t, "cyber_policy", mark.Code)
	require.Equal(t, "blocked by policy", mark.Message)
	require.Equal(t, http.StatusOK, mark.UpstreamStatus)
	require.Equal(t, 13, mark.UpstreamInTok)
}

func TestOpenAIGatewayServiceForwardImages_APIKeyGenerationStripsOAuthOnlyPassthroughHeaders(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-codex-window-id", "oauth-only-window")
	req.Header.Set("x-openai-subagent", "worker-1")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
		httpUpstream: &openAIImagesHTTPUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"X-Request-Id": []string{"req_img_header_filter"},
				},
				Body: io.NopCloser(strings.NewReader(`{"created":1710000007,"data":[{"b64_json":"aGVsbG8="}]}`)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       61,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://image-upstream.example/v1",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	upstream, ok := svc.httpUpstream.(*openAIImagesHTTPUpstreamRecorder)
	require.True(t, ok)
	require.NotNil(t, upstream.lastReq)
	require.Empty(t, upstream.lastReq.Header.Get("x-codex-window-id"))
	require.Empty(t, upstream.lastReq.Header.Get("x-openai-subagent"))
}

func TestOpenAIGatewayServiceForwardImages_APIKeyStreamJSONResponseBillsImage(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":true,"response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
		httpUpstream: &openAIImagesHTTPUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"X-Request-Id": []string{"req_img_stream_json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"created":1710000008,"usage":{"input_tokens":12,"output_tokens":21,"output_tokens_details":{"image_tokens":9}},"data":[{"b64_json":"aGVsbG8=","revised_prompt":"draw a cat"}]}`)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       7,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://image-upstream.example/v1",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, 21, result.Usage.OutputTokens)
	require.Equal(t, 9, result.Usage.ImageOutputTokens)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "aGVsbG8=", gjson.Get(rec.Body.String(), "data.0.b64_json").String())
}

func TestOpenAIGatewayServiceForwardImages_APIKeyStreamRawJSONEventStreamFallbackBillsImage(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":true,"response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
		httpUpstream: &openAIImagesHTTPUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
					"X-Request-Id": []string{"req_img_stream_json_mislabeled"},
				},
				Body: io.NopCloser(strings.NewReader(`{"created":1710000009,"usage":{"input_tokens":10,"output_tokens":18,"output_tokens_details":{"image_tokens":8}},"data":[{"b64_json":"ZmluYWw="}]}`)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       8,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://image-upstream.example/v1",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 10, result.Usage.InputTokens)
	require.Equal(t, 18, result.Usage.OutputTokens)
	require.Equal(t, 8, result.Usage.ImageOutputTokens)
	require.Equal(t, "ZmluYWw=", gjson.Get(rec.Body.String(), "data.0.b64_json").String())
}

func TestOpenAIGatewayServiceForwardImages_APIKeyStreamMultilineSSEDataBillsImage(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":true,"response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
		httpUpstream: &openAIImagesHTTPUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
					"X-Request-Id": []string{"req_img_stream_multiline"},
				},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"image_generation.completed\",\n" +
						"data: \"usage\":{\"input_tokens\":10,\"output_tokens\":18,\"output_tokens_details\":{\"image_tokens\":8}},\n" +
						"data: \"b64_json\":\"ZmluYWw=\",\"output_format\":\"png\"}\n\n" +
						"data: [DONE]\n\n",
				)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       8,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "test-api-key",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 10, result.Usage.InputTokens)
	require.Equal(t, 18, result.Usage.OutputTokens)
	require.Equal(t, 8, result.Usage.ImageOutputTokens)
}

func TestExtractOpenAIImagesBillableCountFromJSONBytes_CompletedEvent(t *testing.T) {
	body := []byte(`{"type":"image_generation.completed","b64_json":"ZmluYWw=","usage":{"input_tokens":10,"output_tokens":18}}`)

	require.Equal(t, 1, extractOpenAIImagesBillableCountFromJSONBytes(body))
}

func TestOpenAIGatewayServiceForwardImages_APIKeyEditUsesConfiguredV1BaseURL(t *testing.T) {
	setGinTestMode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1.5"))
	require.NoError(t, writer.WriteField("prompt", "replace background"))
	imagePart, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("png-image-content"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{
		cfg: &config.Config{},
		httpUpstream: &openAIImagesHTTPUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"X-Request-Id": []string{"req_img_edit_apikey"},
				},
				Body: io.NopCloser(strings.NewReader(`{"created":1710000008,"data":[{"b64_json":"ZWRpdGVk","revised_prompt":"replace background"}]}`)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body.Bytes())
	require.NoError(t, err)

	account := &Account{
		ID:       7,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://image-upstream.example/v1/",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body.Bytes(), parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)

	upstream, ok := svc.httpUpstream.(*openAIImagesHTTPUpstreamRecorder)
	require.True(t, ok)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://image-upstream.example/v1/images/edits", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer test-api-key", upstream.lastReq.Header.Get("Authorization"))
	require.Contains(t, upstream.lastReq.Header.Get("Content-Type"), "multipart/form-data")
	require.Contains(t, string(upstream.lastBody), `name="model"`)
	require.Contains(t, string(upstream.lastBody), "gpt-image-1.5")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "ZWRpdGVk", gjson.Get(rec.Body.String(), "data.0.b64_json").String())
}

func TestOpenAIGatewayServiceForwardImages_OAuthStreamingTransformsEvents(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":true,"response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"req_img_stream"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.created\",\"response\":{\"created_at\":1710000001,\"tools\":[{\"type\":\"image_generation\",\"model\":\"gpt-image-2\",\"background\":\"auto\",\"output_format\":\"png\",\"quality\":\"high\",\"size\":\"1024x1024\"}]}}\n\n" +
					"data: {\"type\":\"response.image_generation_call.partial_image\",\"partial_image_b64\":\"cGFydGlhbA==\",\"partial_image_index\":0,\"output_format\":\"png\",\"background\":\"auto\"}\n\n" +
					"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000001,\"usage\":{\"input_tokens\":5,\"output_tokens\":9,\"output_tokens_details\":{\"image_tokens\":4}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"tools\":[{\"type\":\"image_generation\",\"model\":\"gpt-image-2\",\"background\":\"auto\",\"output_format\":\"png\",\"quality\":\"high\",\"size\":\"1024x1024\"}],\"output\":[{\"type\":\"image_generation_call\",\"result\":\"ZmluYWw=\",\"output_format\":\"png\"}]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc.httpUpstream = upstream

	account := &Account{
		ID:       2,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.Equal(t, 1, result.ImageCount)
	events := parseOpenAIImageTestSSEEvents(rec.Body.String())
	partial, ok := findOpenAIImageTestSSEEvent(events, "image_generation.partial_image")
	require.True(t, ok)
	require.Equal(t, "image_generation.partial_image", gjson.Get(partial.Data, "type").String())
	require.Equal(t, int64(1710000001), gjson.Get(partial.Data, "created_at").Int())
	require.Equal(t, "cGFydGlhbA==", gjson.Get(partial.Data, "b64_json").String())
	require.False(t, gjson.Get(partial.Data, "url").Exists())
	require.Equal(t, "gpt-image-2", gjson.Get(partial.Data, "model").String())
	require.Equal(t, "png", gjson.Get(partial.Data, "output_format").String())
	require.Equal(t, "high", gjson.Get(partial.Data, "quality").String())
	require.Equal(t, "1024x1024", gjson.Get(partial.Data, "size").String())
	require.Equal(t, "auto", gjson.Get(partial.Data, "background").String())

	completed, ok := findOpenAIImageTestSSEEvent(events, "image_generation.completed")
	require.True(t, ok)
	require.Equal(t, "image_generation.completed", gjson.Get(completed.Data, "type").String())
	require.Equal(t, int64(1710000001), gjson.Get(completed.Data, "created_at").Int())
	require.Equal(t, "ZmluYWw=", gjson.Get(completed.Data, "b64_json").String())
	require.False(t, gjson.Get(completed.Data, "url").Exists())
	require.Equal(t, "gpt-image-2", gjson.Get(completed.Data, "model").String())
	require.Equal(t, "png", gjson.Get(completed.Data, "output_format").String())
	require.Equal(t, "high", gjson.Get(completed.Data, "quality").String())
	require.Equal(t, "1024x1024", gjson.Get(completed.Data, "size").String())
	require.Equal(t, "auto", gjson.Get(completed.Data, "background").String())
	require.JSONEq(t, `{"images":1}`, gjson.Get(completed.Data, "usage").Raw)
	require.False(t, gjson.Get(completed.Data, "revised_prompt").Exists())
}

func TestOpenAIGatewayServiceForwardImages_APIKeyStreamingDrainsAfterClientDisconnect(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":true}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Writer = &failingOpenAIImageWriter{ResponseWriter: c.Writer, failAfter: 1}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				ImageStreamDataIntervalTimeout: 1,
				ImageStreamKeepaliveInterval:   0,
			},
		},
		httpUpstream: &openAIImagesHTTPUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
					"X-Request-Id": []string{"req_img_stream_disconnect_apikey"},
				},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"image_generation.partial_image\",\"b64_json\":\"cGFydGlhbA==\"}\n\n" +
						"data: {\"type\":\"image_generation.completed\",\"usage\":{\"input_tokens\":3,\"output_tokens\":4,\"output_tokens_details\":{\"image_tokens\":2}},\"b64_json\":\"ZmluYWw=\",\"output_format\":\"png\"}\n\n" +
						"data: [DONE]\n\n",
				)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       8,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "test-api-key",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.Equal(t, 2, result.Usage.ImageOutputTokens)
}

func TestOpenAIGatewayServiceForwardImages_OAuthEditsMultipartUsesResponsesAPI(t *testing.T) {
	setGinTestMode()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gpt-image-1.5"))
	require.NoError(t, writer.WriteField("prompt", "replace background with aurora"))
	require.NoError(t, writer.WriteField("input_fidelity", "high"))
	require.NoError(t, writer.WriteField("output_format", "webp"))
	require.NoError(t, writer.WriteField("quality", "high"))

	imageHeader := make(textproto.MIMEHeader)
	imageHeader.Set("Content-Disposition", `form-data; name="image"; filename="source.png"`)
	imageHeader.Set("Content-Type", "image/png")
	imagePart, err := writer.CreatePart(imageHeader)
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("png-image-content"))
	require.NoError(t, err)

	maskHeader := make(textproto.MIMEHeader)
	maskHeader.Set("Content-Disposition", `form-data; name="mask"; filename="mask.png"`)
	maskHeader.Set("Content-Type", "image/png")
	maskPart, err := writer.CreatePart(maskHeader)
	require.NoError(t, err)
	_, err = maskPart.Write([]byte("png-mask-content"))
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("api_key", &APIKey{ID: 100})

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body.Bytes())
	require.NoError(t, err)

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"req_img_edit_123"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000002,\"usage\":{\"input_tokens\":13,\"output_tokens\":21,\"output_tokens_details\":{\"image_tokens\":8}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"ZWRpdGVk\",\"revised_prompt\":\"replace background with aurora\",\"output_format\":\"webp\",\"quality\":\"high\"}]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc.httpUpstream = upstream

	account := &Account{
		ID:       3,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body.Bytes(), parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "gpt-image-1.5", gjson.GetBytes(upstream.lastBody, "tools.0.model").String())
	require.Equal(t, "edit", gjson.GetBytes(upstream.lastBody, "tools.0.action").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "tools.0.input_fidelity").Exists())
	require.Equal(t, "webp", gjson.GetBytes(upstream.lastBody, "tools.0.output_format").String())
	require.True(t, strings.HasPrefix(gjson.GetBytes(upstream.lastBody, "input.0.content.1.image_url").String(), "data:image/png;base64,"))
	require.True(t, strings.HasPrefix(gjson.GetBytes(upstream.lastBody, "tools.0.input_image_mask.image_url").String(), "data:image/png;base64,"))
	require.Equal(t, "replace background with aurora", gjson.GetBytes(upstream.lastBody, "input.0.content.0.text").String())
	require.Equal(t, "ZWRpdGVk", gjson.Get(rec.Body.String(), "data.0.b64_json").String())
	require.Equal(t, "replace background with aurora", gjson.Get(rec.Body.String(), "data.0.revised_prompt").String())
}

func TestOpenAIGatewayServiceForwardImages_OAuthEditsStreamingTransformsEvents(t *testing.T) {
	setGinTestMode()
	body := []byte(`{
		"model":"gpt-image-2",
		"prompt":"replace background with aurora",
		"images":[{"image_url":"https://example.com/source.png"}],
		"mask":{"image_url":"https://example.com/mask.png"},
		"stream":true,
		"response_format":"b64_json"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.created\",\"response\":{\"created_at\":1710000003,\"tools\":[{\"type\":\"image_generation\",\"model\":\"gpt-image-2\",\"background\":\"transparent\",\"output_format\":\"webp\",\"quality\":\"high\",\"size\":\"1024x1024\"}]}}\n\n" +
					"data: {\"type\":\"response.image_generation_call.partial_image\",\"partial_image_b64\":\"cGFydGlhbA==\",\"partial_image_index\":0,\"output_format\":\"webp\",\"background\":\"transparent\"}\n\n" +
					"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000003,\"usage\":{\"input_tokens\":7,\"output_tokens\":10,\"output_tokens_details\":{\"image_tokens\":5}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"tools\":[{\"type\":\"image_generation\",\"model\":\"gpt-image-2\",\"background\":\"transparent\",\"output_format\":\"webp\",\"quality\":\"high\",\"size\":\"1024x1024\"}],\"output\":[{\"type\":\"image_generation_call\",\"result\":\"ZWRpdGVk\",\"revised_prompt\":\"replace background with aurora\",\"output_format\":\"webp\"}]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc.httpUpstream = upstream

	account := &Account{
		ID:       4,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "edit", gjson.GetBytes(upstream.lastBody, "tools.0.action").String())
	require.Equal(t, "https://example.com/source.png", gjson.GetBytes(upstream.lastBody, "input.0.content.1.image_url").String())
	require.Equal(t, "https://example.com/mask.png", gjson.GetBytes(upstream.lastBody, "tools.0.input_image_mask.image_url").String())
	events := parseOpenAIImageTestSSEEvents(rec.Body.String())
	partial, ok := findOpenAIImageTestSSEEvent(events, "image_edit.partial_image")
	require.True(t, ok)
	require.Equal(t, "image_edit.partial_image", gjson.Get(partial.Data, "type").String())
	require.Equal(t, int64(1710000003), gjson.Get(partial.Data, "created_at").Int())
	require.Equal(t, "cGFydGlhbA==", gjson.Get(partial.Data, "b64_json").String())
	require.False(t, gjson.Get(partial.Data, "url").Exists())
	require.Equal(t, "gpt-image-2", gjson.Get(partial.Data, "model").String())
	require.Equal(t, "webp", gjson.Get(partial.Data, "output_format").String())
	require.Equal(t, "high", gjson.Get(partial.Data, "quality").String())
	require.Equal(t, "1024x1024", gjson.Get(partial.Data, "size").String())
	require.Equal(t, "transparent", gjson.Get(partial.Data, "background").String())

	completed, ok := findOpenAIImageTestSSEEvent(events, "image_edit.completed")
	require.True(t, ok)
	require.Equal(t, "image_edit.completed", gjson.Get(completed.Data, "type").String())
	require.Equal(t, int64(1710000003), gjson.Get(completed.Data, "created_at").Int())
	require.Equal(t, "ZWRpdGVk", gjson.Get(completed.Data, "b64_json").String())
	require.False(t, gjson.Get(completed.Data, "url").Exists())
	require.Equal(t, "gpt-image-2", gjson.Get(completed.Data, "model").String())
	require.Equal(t, "webp", gjson.Get(completed.Data, "output_format").String())
	require.Equal(t, "high", gjson.Get(completed.Data, "quality").String())
	require.Equal(t, "1024x1024", gjson.Get(completed.Data, "size").String())
	require.Equal(t, "transparent", gjson.Get(completed.Data, "background").String())
	require.JSONEq(t, `{"images":1}`, gjson.Get(completed.Data, "usage").Raw)
	require.False(t, gjson.Get(completed.Data, "revised_prompt").Exists())
}

func TestOpenAIGatewayServiceForwardImages_OAuthEditsFileIDsUseResponsesAPI(t *testing.T) {
	setGinTestMode()
	body := []byte(`{
		"model":"gpt-image-2",
		"prompt":"replace background with aurora",
		"images":[{"file_id":"file_source_123"}],
		"mask":{"file_id":"file_mask_456"},
		"stream":true,
		"response_format":"b64_json"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000003,\"usage\":{\"input_tokens\":7,\"output_tokens\":10,\"output_tokens_details\":{\"image_tokens\":5}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"tools\":[{\"type\":\"image_generation\",\"model\":\"gpt-image-2\",\"background\":\"transparent\",\"output_format\":\"webp\",\"quality\":\"high\",\"size\":\"1024x1024\"}],\"output\":[{\"type\":\"image_generation_call\",\"result\":\"ZWRpdGVk\",\"revised_prompt\":\"replace background with aurora\",\"output_format\":\"webp\"}]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc.httpUpstream = upstream

	account := &Account{
		ID:       14,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "edit", gjson.GetBytes(upstream.lastBody, "tools.0.action").String())
	require.Equal(t, "file_source_123", gjson.GetBytes(upstream.lastBody, "input.0.content.1.file_id").String())
	require.Equal(t, "file_mask_456", gjson.GetBytes(upstream.lastBody, "tools.0.input_image_mask.file_id").String())
	events := parseOpenAIImageTestSSEEvents(rec.Body.String())
	completed, ok := findOpenAIImageTestSSEEvent(events, "image_edit.completed")
	require.True(t, ok)
	require.Equal(t, "ZWRpdGVk", gjson.Get(completed.Data, "b64_json").String())
}

func TestBuildOpenAIImagesResponsesRequest_AllowsMultipleImagesForFanOutCaller(t *testing.T) {
	parsed := &OpenAIImagesRequest{
		Endpoint: openAIImagesGenerationsEndpoint,
		Model:    "gpt-image-2",
		Prompt:   "draw a cat",
		N:        2,
	}

	body, err := buildOpenAIImagesResponsesRequest(parsed, "gpt-image-2", "")
	require.NoError(t, err)
	require.NotNil(t, body)
	require.Equal(t, "draw a cat", gjson.GetBytes(body, "input.0.content.0.text").String())
	require.False(t, gjson.GetBytes(body, "tools.0.n").Exists())
}

func TestBuildOpenAIImagesResponsesRequest_RejectsMissingPrompt(t *testing.T) {
	parsed := &OpenAIImagesRequest{
		Endpoint: openAIImagesGenerationsEndpoint,
		Model:    "gpt-image-2",
		N:        1,
	}

	body, err := buildOpenAIImagesResponsesRequest(parsed, "gpt-image-2", "")
	require.Nil(t, body)
	require.ErrorContains(t, err, "prompt is required")
}

func TestBuildOpenAIImagesResponsesRequest_StripsInputFidelity(t *testing.T) {
	parsed := &OpenAIImagesRequest{
		Endpoint:      openAIImagesEditsEndpoint,
		Model:         "gpt-image-2",
		Prompt:        "replace background",
		InputFidelity: "high",
		InputImageURLs: []string{
			"https://example.com/source.png",
		},
	}

	body, err := buildOpenAIImagesResponsesRequest(parsed, "gpt-image-2", "")
	require.NoError(t, err)
	require.NotNil(t, body)
	require.False(t, gjson.GetBytes(body, "tools.0.input_fidelity").Exists())
	require.Equal(t, "edit", gjson.GetBytes(body, "tools.0.action").String())
}

func TestBuildOpenAIImagesResponsesRequest_ForcesUpstreamStreaming(t *testing.T) {
	parsed := &OpenAIImagesRequest{
		Endpoint: openAIImagesGenerationsEndpoint,
		Model:    "gpt-image-1",
		Prompt:   "draw a cat",
		Stream:   false,
	}

	body, err := buildOpenAIImagesResponsesRequest(parsed, "gpt-image-1", "")
	require.NoError(t, err)
	require.NotNil(t, body)
	require.True(t, gjson.GetBytes(body, "stream").Bool())
}

func TestBuildOpenAIImagesResponsesRequest_StripsStyle(t *testing.T) {
	parsed := &OpenAIImagesRequest{
		Endpoint: openAIImagesGenerationsEndpoint,
		Model:    "gpt-image-2",
		Prompt:   "draw a cat",
		Style:    "vivid",
	}

	body, err := buildOpenAIImagesResponsesRequest(parsed, "gpt-image-2", "")
	require.NoError(t, err)
	require.NotNil(t, body)
	require.False(t, gjson.GetBytes(body, "tools.0.style").Exists())
}

func TestBuildOpenAIImagesResponsesRequest_PreservesMixedImageReferenceOrder(t *testing.T) {
	setGinTestMode()
	body := []byte(`{
		"model":"gpt-image-2",
		"prompt":"replace background",
		"images":[
			{"file_id":"file_source_1"},
			{"image_url":"https://example.com/source-2.png"},
			{"file_id":"file_source_3"}
		]
	}`)

	c := newOpenAIImagesParseRequestContext(http.MethodPost, "/v1/images/edits", body, "application/json")
	parsed, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	reqBody, err := buildOpenAIImagesResponsesRequest(parsed, "gpt-image-2", "")
	require.NoError(t, err)
	require.Equal(t, "file_source_1", gjson.GetBytes(reqBody, "input.0.content.1.file_id").String())
	require.Equal(t, "https://example.com/source-2.png", gjson.GetBytes(reqBody, "input.0.content.2.image_url").String())
	require.Equal(t, "file_source_3", gjson.GetBytes(reqBody, "input.0.content.3.file_id").String())
}

func TestBuildOpenAIImagesResponsesRequest_UsesCustomMainModel(t *testing.T) {
	parsed := &OpenAIImagesRequest{
		Endpoint: openAIImagesGenerationsEndpoint,
		Model:    "gpt-image-2",
		Prompt:   "draw a cat",
	}

	body, err := buildOpenAIImagesResponsesRequest(parsed, "gpt-image-2", "gpt-5.4")
	require.NoError(t, err)
	require.Equal(t, "gpt-5.4", gjson.GetBytes(body, "model").String())
}

func TestCollectOpenAIImagesFromResponsesBody_FallsBackToOutputItemDone(t *testing.T) {
	body := []byte(
		"data: {\"type\":\"response.created\",\"response\":{\"created_at\":1710000004}}\n\n" +
			"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"ig_123\",\"type\":\"image_generation_call\",\"result\":\"aGVsbG8=\",\"revised_prompt\":\"draw a cat\",\"output_format\":\"png\",\"quality\":\"high\"}}\n\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000004,\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[]}}\n\n" +
			"data: [DONE]\n\n",
	)

	results, createdAt, usageRaw, firstMeta, foundFinal, err := collectOpenAIImagesFromResponsesBody(body)
	require.NoError(t, err)
	require.True(t, foundFinal)
	require.Equal(t, int64(1710000004), createdAt)
	require.Len(t, results, 1)
	require.Equal(t, "aGVsbG8=", results[0].Result)
	require.Equal(t, "draw a cat", results[0].RevisedPrompt)
	require.Equal(t, "png", firstMeta.OutputFormat)
	require.JSONEq(t, `{"images":1}`, string(usageRaw))
}

func TestCollectOpenAIImagesFromResponsesBody_MultilineSSE(t *testing.T) {
	body := []byte(
		"data: {\"type\":\"response.completed\",\n" +
			"data: \"response\":{\"created_at\":1710000010,\"usage\":{\"input_tokens\":5,\"output_tokens\":9,\"output_tokens_details\":{\"image_tokens\":4}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"ZmluYWw=\",\"output_format\":\"png\"}]}}\n\n" +
			"data: [DONE]\n\n",
	)

	results, createdAt, usageRaw, firstMeta, foundFinal, err := collectOpenAIImagesFromResponsesBody(body)
	require.NoError(t, err)
	require.True(t, foundFinal)
	require.Equal(t, int64(1710000010), createdAt)
	require.Len(t, results, 1)
	require.Equal(t, "ZmluYWw=", results[0].Result)
	require.Equal(t, "png", firstMeta.OutputFormat)
	require.JSONEq(t, `{"images":1}`, string(usageRaw))
}

func TestOpenAIGatewayServiceForwardImages_OAuthStreamingHandlesOutputItemDoneFallback(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":true,"response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"req_img_stream_output_item_done"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"ig_123\",\"type\":\"image_generation_call\",\"result\":\"ZmluYWw=\",\"revised_prompt\":\"draw a cat\",\"output_format\":\"png\"}}\n\n" +
					"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000005,\"usage\":{\"input_tokens\":5,\"output_tokens\":9,\"output_tokens_details\":{\"image_tokens\":4}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc.httpUpstream = upstream

	account := &Account{
		ID:       5,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.Equal(t, 1, result.ImageCount)
	events := parseOpenAIImageTestSSEEvents(rec.Body.String())
	completed, ok := findOpenAIImageTestSSEEvent(events, "image_generation.completed")
	require.True(t, ok)
	require.Equal(t, "image_generation.completed", gjson.Get(completed.Data, "type").String())
	require.Equal(t, int64(1710000005), gjson.Get(completed.Data, "created_at").Int())
	require.Equal(t, "ZmluYWw=", gjson.Get(completed.Data, "b64_json").String())
	require.False(t, gjson.Get(completed.Data, "url").Exists())
	require.Equal(t, "gpt-image-2", gjson.Get(completed.Data, "model").String())
	require.JSONEq(t, `{"images":1}`, gjson.Get(completed.Data, "usage").Raw)
	require.NotContains(t, rec.Body.String(), "event: error")
}

func TestOpenAIGatewayServiceForwardImages_OAuthStreamingHandlesMultilineSSE(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":true,"response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	svc.httpUpstream = &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"req_img_stream_multiline_oauth"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.completed\",\n" +
					"data: \"response\":{\"created_at\":1710000011,\"usage\":{\"input_tokens\":6,\"output_tokens\":10,\"output_tokens_details\":{\"image_tokens\":5}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"TXVsdGlsaW5l\",\"output_format\":\"png\"}]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}

	account := &Account{
		ID:       11,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 6, result.Usage.InputTokens)
	require.Equal(t, 10, result.Usage.OutputTokens)
	require.Equal(t, 5, result.Usage.ImageOutputTokens)
	events := parseOpenAIImageTestSSEEvents(rec.Body.String())
	completed, ok := findOpenAIImageTestSSEEvent(events, "image_generation.completed")
	require.True(t, ok)
	require.Equal(t, "TXVsdGlsaW5l", gjson.Get(completed.Data, "b64_json").String())
	require.JSONEq(t, `{"images":1}`, gjson.Get(completed.Data, "usage").Raw)
	require.NotContains(t, rec.Body.String(), "event: error")
}

func TestOpenAIGatewayServiceForwardImages_OAuthStreamingDrainsAfterClientDisconnect(t *testing.T) {
	setGinTestMode()
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat","stream":true,"response_format":"b64_json"}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Writer = &failingOpenAIImageWriter{ResponseWriter: c.Writer, failAfter: 1}

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				ImageStreamDataIntervalTimeout: 1,
				ImageStreamKeepaliveInterval:   0,
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	upstream := &openAIImagesHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
				"X-Request-Id": []string{"req_img_stream_disconnect_oauth"},
			},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.image_generation_call.partial_image\",\"partial_image_b64\":\"cGFydGlhbA==\",\"partial_image_index\":0,\"output_format\":\"png\"}\n\n" +
					"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000009,\"usage\":{\"input_tokens\":5,\"output_tokens\":9,\"output_tokens_details\":{\"image_tokens\":4}},\"tool_usage\":{\"image_gen\":{\"images\":1}},\"output\":[{\"type\":\"image_generation_call\",\"result\":\"ZmluYWw=\",\"output_format\":\"png\"}]}}\n\n" +
					"data: [DONE]\n\n",
			)),
		},
	}
	svc.httpUpstream = upstream

	account := &Account{
		ID:       9,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 5, result.Usage.InputTokens)
	require.Equal(t, 9, result.Usage.OutputTokens)
	require.Equal(t, 4, result.Usage.ImageOutputTokens)
}
