package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	_ "golang.org/x/image/webp"
)

const creationGeminiImageMaxBytes = 20 << 20
const creationGeminiMaxReferences = 14

func (h *CreationHandler) localImageExecutor(local *AsyncImageHandler) func(string, *gin.Context) {
	return func(platform string, c *gin.Context) {
		if platform == service.PlatformGemini {
			local.executeCreationGeminiImage(c)
			return
		}
		if h.asyncImage.execute == nil {
			imageTaskError(c, service.ErrImageTaskUnavailable)
			return
		}
		h.asyncImage.execute(platform, c)
	}
}

type creationGeminiImageInput struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt"`
	ImageSize   string `json:"image_size"`
	Size        string `json:"size"`
	AspectRatio string `json:"aspect_ratio"`
	N           int    `json:"n"`
	Stream      bool   `json:"stream"`
}

type creationGeminiInlineData struct {
	MIME string `json:"mimeType"`
	Data string `json:"data"`
}

func parseCreationGeminiImageRequest(contentType, path string, body []byte) (string, []byte, error) {
	if len(body) == 0 || len(body) > creationGeminiImageMaxBytes {
		return "", nil, errors.New("Gemini image requests must be between 1 byte and 20 MiB")
	}
	var input creationGeminiImageInput
	var references []creationGeminiInlineData
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", nil, errors.New("invalid image request content type")
	}
	switch mediaType {
	case "application/json":
		if err := json.Unmarshal(body, &input); err != nil {
			return "", nil, errors.New("invalid Gemini image request JSON")
		}
	case "multipart/form-data":
		reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", nil, errors.New("invalid image multipart body")
			}
			data, err := io.ReadAll(io.LimitReader(part, creationGeminiImageMaxBytes+1))
			_ = part.Close()
			if err != nil {
				return "", nil, err
			}
			if part.FileName() != "" {
				switch part.FormName() {
				case "image", "image[]", "images", "images[]":
				default:
					return "", nil, errors.New("Gemini edits accept reference images only; masks are not supported")
				}
				if len(references) >= creationGeminiMaxReferences {
					return "", nil, errors.New("Gemini accepts at most 14 reference images")
				}
				imageMIME, err := creationGeminiImageMIME(data)
				if err != nil {
					return "", nil, err
				}
				references = append(references, creationGeminiInlineData{MIME: imageMIME, Data: base64.StdEncoding.EncodeToString(data)})
				continue
			}
			value := string(data)
			switch part.FormName() {
			case "model":
				input.Model = value
			case "prompt":
				input.Prompt = value
			case "image_size":
				input.ImageSize = value
			case "size":
				input.Size = value
			case "aspect_ratio":
				input.AspectRatio = value
			case "n":
				input.N, err = strconv.Atoi(value)
			case "stream":
				input.Stream, err = strconv.ParseBool(value)
			}
			if err != nil {
				return "", nil, errors.New("invalid Gemini image request option")
			}
		}
	default:
		return "", nil, errors.New("Gemini images require JSON or multipart form data")
	}
	input.Model = strings.TrimSpace(input.Model)
	if !service.IsSafeGeminiModelPathSegment(input.Model) || strings.TrimSpace(input.Prompt) == "" {
		return "", nil, errors.New("a valid model and prompt are required")
	}
	if input.Stream || (input.N != 0 && input.N != 1) {
		return "", nil, errors.New("Gemini asynchronous images require n=1 and stream=false")
	}
	if strings.Contains(path, "/edits") && len(references) == 0 {
		return "", nil, errors.New("Gemini image editing requires at least one uploaded reference image")
	}
	size := strings.ToUpper(strings.TrimSpace(input.ImageSize))
	if size == "" {
		size = strings.ToUpper(strings.TrimSpace(input.Size))
	}
	if size == "" {
		size = "1K"
	}
	if size != "1K" && size != "2K" && size != "4K" {
		return "", nil, errors.New("Gemini image_size must be 1K, 2K or 4K")
	}
	ratio := strings.TrimSpace(input.AspectRatio)
	switch ratio {
	case "", "1:1", "3:2", "16:9", "9:16", "4:3", "3:4":
	default:
		return "", nil, errors.New("unsupported Gemini image aspect_ratio")
	}
	parts := []any{gin.H{"text": input.Prompt}}
	for _, reference := range references {
		parts = append(parts, gin.H{"inlineData": reference})
	}
	imageConfig := gin.H{"imageSize": size}
	if ratio != "" {
		imageConfig["aspectRatio"] = ratio
	}
	native, err := json.Marshal(gin.H{
		"contents":         []any{gin.H{"role": "user", "parts": parts}},
		"generationConfig": gin.H{"responseModalities": []string{"TEXT", "IMAGE"}, "imageConfig": imageConfig},
	})
	return input.Model, native, err
}

func creationGeminiImageMIME(data []byte) (string, error) {
	kind := http.DetectContentType(data)
	if kind != "image/png" && kind != "image/jpeg" && kind != "image/webp" {
		return "", errors.New("Gemini reference images must contain PNG, JPEG or WebP bytes")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 16384 || config.Height > 16384 || int64(config.Width)*int64(config.Height) > 40_000_000 {
		return "", errors.New("invalid reference image dimensions")
	}
	return kind, nil
}

func (h *AsyncImageHandler) checkCreationGeminiImageAudit(c *gin.Context, apiKey *service.APIKey, body []byte) bool {
	if h.gemini == nil {
		imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "Gemini image gateway is unavailable")
		return false
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		imageTaskJSONError(c, http.StatusUnauthorized, "authentication_error", "User context not found")
		return false
	}
	model, native, err := parseCreationGeminiImageRequest(c.GetHeader("Content-Type"), c.Request.URL.Path, body)
	if err != nil {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return false
	}
	reqLog := requestLogger(c, "handler.creation.gemini_image", zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID), zap.String("model", model))
	if decision := h.gemini.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolGemini, model, native); decision != nil && !decision.AllowNextStage {
		googleSecurityAuditError(c, decision)
		return false
	}
	return true
}

func (h *AsyncImageHandler) executeCreationGeminiImage(c *gin.Context) {
	if h.gemini == nil {
		imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "Gemini image gateway is unavailable")
		return
	}
	forwardCreationGeminiImage(c, h.gemini.GeminiV1BetaModels)
}

// Run the native gateway intact so scheduling, security auditing and billing
// remain on its existing path; only adapt the final response for local tasks.
func forwardCreationGeminiImage(c *gin.Context, gateway gin.HandlerFunc) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, creationGeminiImageMaxBytes+1))
	if err != nil {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "failed to read image request")
		return
	}
	model, native, err := parseCreationGeminiImageRequest(c.GetHeader("Content-Type"), c.Request.URL.Path, body)
	if err != nil {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	request := c.Request.Clone(c.Request.Context())
	request.URL.Path = "/v1beta/models/" + model + ":generateContent"
	request.URL.RawPath = ""
	request.URL.RawQuery = ""
	request.Header.Set("Content-Type", "application/json")
	request.Body = io.NopCloser(bytes.NewReader(native))
	request.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(native)), nil }
	request.ContentLength = int64(len(native))
	c.Request = request
	c.Params = gin.Params{{Key: "modelAction", Value: "/" + model + ":generateContent"}}
	writer := c.Writer
	recorder := httptest.NewRecorder()
	recordedContext, _ := gin.CreateTestContext(recorder)
	c.Writer = recordedContext.Writer
	defer func() { c.Writer = writer }()
	gateway(c)
	c.Writer = writer
	if recorder.Code < 200 || recorder.Code >= 300 {
		c.Data(recorder.Code, "application/json", recorder.Body.Bytes())
		return
	}
	result, err := extractCreationGeminiImages(recorder.Body.Bytes())
	if err != nil {
		imageTaskJSONError(c, http.StatusBadGateway, "api_error", err.Error())
		return
	}
	c.Data(http.StatusOK, "application/json", result)
}

func extractCreationGeminiImages(body []byte) ([]byte, error) {
	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Thought     bool                      `json:"thought"`
					Inline      *creationGeminiInlineData `json:"inlineData"`
					SnakeInline *struct {
						MIME string `json:"mime_type"`
						Data string `json:"data"`
					} `json:"inline_data"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		PromptFeedback struct {
			BlockReason string `json:"blockReason"`
		} `json:"promptFeedback"`
		Usage struct {
			Input  int `json:"promptTokenCount"`
			Output int `json:"candidatesTokenCount"`
			Total  int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}
	if len(body) > 32<<20 || json.Unmarshal(body, &response) != nil {
		return nil, errors.New("Gemini returned an invalid image response")
	}
	images := make([]gin.H, 0)
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Thought {
				continue
			}
			inline := part.Inline
			if inline == nil && part.SnakeInline != nil {
				inline = &creationGeminiInlineData{MIME: part.SnakeInline.MIME, Data: part.SnakeInline.Data}
			}
			if inline == nil || inline.Data == "" {
				continue
			}
			data, err := base64.StdEncoding.DecodeString(inline.Data)
			if err != nil {
				return nil, errors.New("Gemini returned invalid base64 image data")
			}
			kind, err := creationGeminiImageMIME(data)
			if err != nil {
				return nil, errors.New("Gemini returned invalid image data")
			}
			images = append(images, gin.H{"b64_json": inline.Data, "mime_type": kind})
		}
	}
	if len(images) == 0 {
		if response.PromptFeedback.BlockReason != "" {
			return nil, fmt.Errorf("Gemini image request was blocked: %s", response.PromptFeedback.BlockReason)
		}
		return nil, errors.New("Gemini did not return an image")
	}
	return json.Marshal(gin.H{"created": time.Now().Unix(), "data": images, "usage": gin.H{"input_tokens": response.Usage.Input, "output_tokens": response.Usage.Output, "total_tokens": response.Usage.Total}})
}
