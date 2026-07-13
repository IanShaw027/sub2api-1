package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const grokMediaVideoBoundModelContextKey = "grok_media_video_bound_model"

func SetGrokMediaVideoBoundModel(c *gin.Context, model string) {
	if c == nil {
		return
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return
	}
	c.Set(grokMediaVideoBoundModelContextKey, model)
}

func GetGrokMediaVideoBoundModel(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GetString(grokMediaVideoBoundModelContextKey))
}

type GrokMediaEndpoint string

const (
	GrokMediaEndpointImagesGenerations GrokMediaEndpoint = "images_generations"
	GrokMediaEndpointImagesEdits       GrokMediaEndpoint = "images_edits"
	GrokMediaEndpointVideosGenerations GrokMediaEndpoint = "videos_generations"
	GrokMediaEndpointVideoStatus       GrokMediaEndpoint = "video_status"

	// Official xAI multi-image edit limit (docs.x.ai Imagine edits).
	grokMediaMaxEditSourceImages = 3
	// Default billable duration when create/status responses omit seconds.
	// Aligns pending-create billing with xAI's common 8s default.
	grokMediaDefaultVideoSeconds = 8
)

func (e GrokMediaEndpoint) RequiresRequestBody() bool {
	return e != GrokMediaEndpointVideoStatus
}

func (e GrokMediaEndpoint) IsGenerationRequest() bool {
	switch e {
	case GrokMediaEndpointImagesGenerations, GrokMediaEndpointImagesEdits, GrokMediaEndpointVideosGenerations:
		return true
	default:
		return false
	}
}

type GrokMediaRequestInfo struct {
	Model          string
	Prompt         string
	N              int
	Size           string
	SizeTier       string
	InputImageURLs []string
	MaskImageURL   string
	Uploads        []OpenAIImagesUpload
	MaskUpload     *OpenAIImagesUpload
}

func (r GrokMediaRequestInfo) ModerationBody() []byte {
	payload := map[string]any{}
	if prompt := strings.TrimSpace(r.Prompt); prompt != "" {
		payload["prompt"] = prompt
	}

	// Moderation payload keeps a simple image_url list for the content filter;
	// image refs themselves are resolved via extractGrokMediaImageURL on parse.
	images := make([]map[string]string, 0, len(r.InputImageURLs)+len(r.Uploads)+1)
	for _, imageURL := range r.InputImageURLs {
		if imageURL = strings.TrimSpace(imageURL); imageURL != "" {
			images = append(images, map[string]string{"image_url": imageURL})
		}
	}
	for _, upload := range r.Uploads {
		if dataURL := upload.ModerationDataURL(); dataURL != "" {
			images = append(images, map[string]string{"image_url": dataURL})
		}
	}
	if maskURL := strings.TrimSpace(r.MaskImageURL); maskURL != "" {
		images = append(images, map[string]string{"image_url": maskURL})
	}
	if r.MaskUpload != nil {
		if dataURL := r.MaskUpload.ModerationDataURL(); dataURL != "" {
			images = append(images, map[string]string{"image_url": dataURL})
		}
	}
	if len(images) > 0 {
		payload["images"] = images
	}
	if len(payload) == 0 {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return body
}

func (e GrokMediaEndpoint) httpMethod() string {
	if e == GrokMediaEndpointVideoStatus {
		return http.MethodGet
	}
	return http.MethodPost
}

func ExtractGrokMediaModel(contentType string, body []byte) string {
	return ParseGrokMediaRequest(contentType, body).Model
}

func ParseGrokMediaRequest(contentType string, body []byte) GrokMediaRequestInfo {
	info := GrokMediaRequestInfo{N: 1}
	if gjson.ValidBytes(body) {
		parseGrokMediaJSONRequest(body, &info)
	} else {
		parseGrokMediaMultipartRequest(contentType, body, &info)
	}
	info.Model = strings.TrimSpace(info.Model)
	info.Prompt = strings.TrimSpace(info.Prompt)
	info.Size = strings.TrimSpace(info.Size)
	info.SizeTier = NormalizeImageBillingTierOrDefault(info.Size)
	if info.N <= 0 {
		info.N = 1
	}
	return info
}

func parseGrokMediaJSONRequest(body []byte, info *GrokMediaRequestInfo) {
	if info == nil {
		return
	}
	info.Model = strings.TrimSpace(gjson.GetBytes(body, "model").String())
	info.Prompt = strings.TrimSpace(gjson.GetBytes(body, "prompt").String())
	info.Size = strings.TrimSpace(gjson.GetBytes(body, "size").String())
	if n := gjson.GetBytes(body, "n"); n.Exists() && n.Type == gjson.Number {
		info.N = int(n.Int())
	}
	appendJSONImageURLs := func(value gjson.Result) {
		if !value.Exists() {
			return
		}
		switch {
		case value.IsArray():
			for _, item := range value.Array() {
				if imageURL := extractGrokMediaImageURL(item); imageURL != "" {
					info.InputImageURLs = append(info.InputImageURLs, imageURL)
				}
			}
		default:
			if imageURL := extractGrokMediaImageURL(value); imageURL != "" {
				info.InputImageURLs = append(info.InputImageURLs, imageURL)
			}
		}
	}
	appendJSONImageURLs(gjson.GetBytes(body, "image"))
	appendJSONImageURLs(gjson.GetBytes(body, "images"))
	// Mask: support both official xAI ({url,type}) and legacy {image_url} shapes.
	info.MaskImageURL = extractGrokMediaImageURL(gjson.GetBytes(body, "mask"))
}

// extractGrokMediaImageURL resolves an image reference from common client shapes:
//   - official xAI: {"url":"...","type":"image_url"}
//   - OpenAI-style nested: {"image_url":"..."} or {"image_url":{"url":"..."}}
//   - plain string URL / data URI
func extractGrokMediaImageURL(value gjson.Result) string {
	if !value.Exists() {
		return ""
	}
	if value.Type == gjson.String {
		return strings.TrimSpace(value.String())
	}
	// Official xAI Imagine field.
	if url := strings.TrimSpace(value.Get("url").String()); url != "" {
		return url
	}
	// Nested OpenAI-style image_url object.
	if nested := value.Get("image_url"); nested.Exists() {
		if nested.Type == gjson.String {
			if url := strings.TrimSpace(nested.String()); url != "" {
				return url
			}
		}
		if url := strings.TrimSpace(nested.Get("url").String()); url != "" {
			return url
		}
	}
	// Flat legacy key used by some gateways.
	if url := strings.TrimSpace(value.Get("image_url").String()); url != "" {
		return url
	}
	return ""
}

// grokMediaImageObject builds the official xAI image object shape.
func grokMediaImageObject(imageURL string) map[string]string {
	return map[string]string{
		"url":  imageURL,
		"type": "image_url",
	}
}

func parseGrokMediaMultipartRequest(contentType string, body []byte, info *GrokMediaRequestInfo) {
	if info == nil {
		return
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return
		}
		if err != nil {
			return
		}
		name := strings.TrimSpace(part.FormName())
		if name == "" {
			_ = part.Close()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, openAIImageMaxUploadPartSize))
		_ = part.Close()
		if err != nil {
			return
		}
		fileName := strings.TrimSpace(part.FileName())
		partContentType := strings.TrimSpace(part.Header.Get("Content-Type"))
		if fileName != "" {
			upload := OpenAIImagesUpload{
				FieldName:   name,
				FileName:    fileName,
				ContentType: partContentType,
				Data:        data,
			}
			if name == "mask" {
				info.MaskUpload = &upload
				continue
			}
			if name == "image" || strings.HasPrefix(name, "image[") {
				info.Uploads = append(info.Uploads, upload)
			}
			continue
		}

		value := strings.TrimSpace(string(data))
		switch name {
		case "model":
			info.Model = value
		case "prompt":
			info.Prompt = value
		case "size":
			info.Size = value
		case "n":
			if n, err := strconv.Atoi(value); err == nil {
				info.N = n
			}
		case "image", "image_url":
			if value != "" {
				info.InputImageURLs = append(info.InputImageURLs, value)
			}
		case "mask", "mask_image_url":
			info.MaskImageURL = value
		}
	}
}

func GrokMediaVideoRequestSessionHash(requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ""
	}
	return "grok-video:" + DeriveSessionHashFromSeed(requestID)
}

func (s *OpenAIGatewayService) BindGrokMediaVideoRequestAccount(ctx context.Context, groupID *int64, requestID string, accountID int64) error {
	return s.BindStickySession(ctx, groupID, GrokMediaVideoRequestSessionHash(requestID), accountID)
}

func grokMediaVideoRequestModelSessionHash(requestID string) string {
	base := GrokMediaVideoRequestSessionHash(requestID)
	if base == "" {
		return ""
	}
	return "grok-video-model:" + strings.TrimPrefix(base, "grok-video:")
}

func (s *OpenAIGatewayService) BindGrokMediaVideoRequestModel(ctx context.Context, groupID *int64, requestID string, model string) error {
	if s == nil || s.cache == nil {
		return nil
	}
	sessionHash := grokMediaVideoRequestModelSessionHash(requestID)
	model = strings.TrimSpace(responseGrokVideoModel(model))
	if sessionHash == "" || model == "" {
		return nil
	}
	ttl := openaiStickySessionTTL
	if s.cfg != nil && s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds > 0 {
		ttl = time.Duration(s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds) * time.Second
	}
	return s.cache.SetOpenAIResponsesSessionWindow(ctx, derefGroupID(groupID), sessionHash, []byte(model), ttl)
}

func (s *OpenAIGatewayService) GetGrokMediaVideoRequestModel(ctx context.Context, groupID *int64, requestID string) (string, error) {
	if s == nil || s.cache == nil {
		return "", ErrGatewayCacheMiss
	}
	sessionHash := grokMediaVideoRequestModelSessionHash(requestID)
	if sessionHash == "" {
		return "", ErrGatewayCacheMiss
	}
	payload, err := s.cache.GetOpenAIResponsesSessionWindow(ctx, derefGroupID(groupID), sessionHash)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(payload)), nil
}

func (e GrokMediaEndpoint) upstreamURL(baseURL, requestID string) (string, error) {
	switch e {
	case GrokMediaEndpointImagesGenerations:
		return xai.BuildImagesGenerationsURL(baseURL)
	case GrokMediaEndpointImagesEdits:
		return xai.BuildImagesEditsURL(baseURL)
	case GrokMediaEndpointVideosGenerations:
		return xai.BuildVideosGenerationsURL(baseURL)
	case GrokMediaEndpointVideoStatus:
		return xai.BuildVideoURL(baseURL, requestID)
	default:
		return "", fmt.Errorf("unsupported grok media endpoint: %s", e)
	}
}

func (s *OpenAIGatewayService) ForwardGrokMedia(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint GrokMediaEndpoint,
	requestID string,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	if account.Platform != PlatformGrok {
		return nil, fmt.Errorf("account platform %s is not supported for grok media", account.Platform)
	}

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	// Imagine images/videos always use official api.x.ai (not cli-chat-proxy / system CLI mode).
	baseURL := xai.DefaultBaseURL
	if s != nil && s.settingService != nil {
		baseURL = s.settingService.ResolveGrokMediaBaseURL(ctx, account)
	} else if account != nil {
		baseURL = account.GetGrokBaseURLOr(xai.DefaultBaseURL)
		if isGrokCLIChatProxyBaseURL(baseURL) {
			baseURL = xai.DefaultBaseURL
		}
	}
	targetURL, err := endpoint.upstreamURL(baseURL, requestID)
	if err != nil {
		return nil, err
	}

	body, contentType, err = prepareGrokMediaForwardBody(endpoint, body, contentType)
	if err != nil {
		// Client-facing validation (e.g. >3 edit images) must surface as 400,
		// not a generic 502 from the handler when nothing has been written yet.
		writeGrokMediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}
	originalModel, upstreamModel := "", ""
	body, contentType, originalModel, upstreamModel, err = normalizeGrokMediaForwardBody(account, endpoint, body, contentType)
	if err != nil {
		writeGrokMediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return nil, err
	}

	var bodyReader io.Reader
	if endpoint.RequiresRequestBody() {
		bodyReader = bytes.NewReader(body)
	}
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, endpoint.httpMethod(), targetURL, bodyReader)
	if err != nil {
		return nil, err
	}
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	upstreamReq.Header.Set("Accept", "application/json")
	applyDefaultGrokUpstreamHeaders(upstreamReq)
	if endpoint.RequiresRequestBody() {
		contentType = strings.TrimSpace(contentType)
		if contentType == "" {
			contentType = "application/json"
		}
		upstreamReq.Header.Set("Content-Type", contentType)
	}
	// 与 /responses 走同一 OAuth token 命中同一 api.x.ai：media 端点必须复用相同的
	// TLS 指纹 + 浏览器 UA，否则按身份关联的检测器会看到 JA3/UA 自相矛盾（默认 UA 仅作兜底）。
	tlsRuntime := s.resolveGrokTLSFingerprintRuntime(ctx, c, account, "http")
	applyGrokRuntimeHeaders(upstreamReq, tlsRuntime)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStart := time.Now()
	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()

	requestIDHeader := firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id"))
	requestInfo := ParseGrokMediaRequest(contentType, body)
	// Prefer original client model for logging/billing identity; fall back to body model.
	requestModel := firstNonEmptyString(originalModel, requestInfo.Model)
	if resp.StatusCode >= 400 {
		s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
		return s.handleGrokMediaErrorResponse(ctx, resp, c, account, requestIDHeader, requestModel)
	}

	s.updateGrokUsageSnapshot(ctx, account.ID, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if endpoint == GrokMediaEndpointVideoStatus {
		if normalized, normalizeErr := buildOpenAIVideoRetrieveResponseFromGrok(requestID, respBody, requestModel); normalizeErr == nil {
			respBody = normalized
		}
	}
	writeGrokMediaResponse(c, resp, respBody, s.responseHeaderFilter)
	usage := grokMediaUsageFromResponse(endpoint, requestInfo, respBody)
	resultModel := firstNonEmptyString(originalModel, requestInfo.Model)
	resultUpstream := firstNonEmptyString(upstreamModel, requestInfo.Model)
	result := &OpenAIForwardResult{
		RequestID:       requestIDHeader,
		ResponseID:      usage.ResponseID,
		Usage:           usage.Usage,
		Model:           resultModel,
		BillingModel:    resultModel,
		UpstreamModel:   resultUpstream,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
	}
	switch endpoint {
	case GrokMediaEndpointVideosGenerations:
		// Video generation billing uses Video* fields, never ImageCount.
		result.VideoCount = usage.VideoCount
		result.VideoSeconds = usage.VideoSeconds
		result.VideoSize = usage.VideoSize
	default:
		result.ImageCount = usage.ImageCount
		result.ImageSize = usage.ImageSize
		result.ImageInputSize = usage.ImageInputSize
		result.ImageOutputSizes = usage.ImageOutputSizes
	}
	return result, nil
}

func prepareGrokMediaForwardBody(endpoint GrokMediaEndpoint, body []byte, contentType string) ([]byte, string, error) {
	switch endpoint {
	case GrokMediaEndpointImagesEdits:
		if gjson.ValidBytes(body) {
			// JSON edits: rewrite OpenAI-style image refs to official xAI shape.
			out, err := normalizeGrokMediaJSONImageRefs(body)
			return out, contentType, err
		}
		return prepareGrokMediaMultipartEditsBody(body, contentType)
	case GrokMediaEndpointVideosGenerations:
		if gjson.ValidBytes(body) {
			out, err := normalizeGrokVideoJSONRequest(body)
			return out, contentType, err
		}
		return body, contentType, nil
	default:
		return body, contentType, nil
	}
}

func normalizeGrokVideoJSONRequest(body []byte) ([]byte, error) {
	out := body
	var err error
	if model := normalizeGrokMediaModelForEndpoint(GrokMediaEndpointVideosGenerations, gjson.GetBytes(out, "model").String()); model != "" {
		out, err = sjson.SetBytes(out, "model", model)
		if err != nil {
			return nil, err
		}
	}
	if seconds := strings.TrimSpace(gjson.GetBytes(out, "seconds").String()); seconds != "" {
		duration, parseErr := strconv.Atoi(seconds)
		if parseErr != nil || duration <= 0 {
			return nil, fmt.Errorf("seconds must be a positive integer")
		}
		out, err = sjson.SetBytes(out, "duration", duration)
		if err != nil {
			return nil, err
		}
		out, err = sjson.DeleteBytes(out, "seconds")
		if err != nil {
			return nil, err
		}
	}
	if size := strings.TrimSpace(gjson.GetBytes(out, "size").String()); size != "" {
		aspectRatio, resolution, sizeErr := grokVideoSizeOptions(size)
		if sizeErr != nil {
			return nil, sizeErr
		}
		if strings.TrimSpace(gjson.GetBytes(out, "aspect_ratio").String()) == "" {
			out, err = sjson.SetBytes(out, "aspect_ratio", aspectRatio)
			if err != nil {
				return nil, err
			}
		}
		if strings.TrimSpace(gjson.GetBytes(out, "resolution").String()) == "" {
			out, err = sjson.SetBytes(out, "resolution", resolution)
			if err != nil {
				return nil, err
			}
		}
		out, err = sjson.DeleteBytes(out, "size")
		if err != nil {
			return nil, err
		}
	}
	if aspectRatio := grokVideoAspectRatioOption(gjson.GetBytes(out, "aspect_ratio").String(), ""); aspectRatio != "" {
		out, err = sjson.SetBytes(out, "aspect_ratio", aspectRatio)
		if err != nil {
			return nil, err
		}
	}
	if resolution := grokVideoResolutionOption(gjson.GetBytes(out, "resolution").String(), ""); resolution != "" {
		out, err = sjson.SetBytes(out, "resolution", resolution)
		if err != nil {
			return nil, err
		}
	}
	imageURL, err := grokVideoInputImageURL(out)
	if err != nil {
		return nil, err
	}
	if imageURL != "" {
		out, err = sjson.SetBytes(out, "image.url", imageURL)
		if err != nil {
			return nil, err
		}
		out, _ = sjson.DeleteBytes(out, "input_reference")
		out, _ = sjson.DeleteBytes(out, "image_url")
	}
	referenceImages := collectGrokVideoReferenceImages(out)
	if len(referenceImages) > 7 {
		return nil, fmt.Errorf("reference_images supports at most 7 images on xAI")
	}
	if imageURL != "" && len(referenceImages) > 0 {
		return nil, fmt.Errorf("image and reference_images cannot be combined on xAI")
	}
	if len(referenceImages) > 0 {
		if duration := gjson.GetBytes(out, "duration"); duration.Exists() && duration.Int() > 10 {
			out, err = sjson.SetBytes(out, "duration", 10)
			if err != nil {
				return nil, err
			}
		}
		out, _ = sjson.DeleteBytes(out, "reference_images")
		out, _ = sjson.DeleteBytes(out, "reference_image_urls")
		for _, image := range referenceImages {
			out, err = sjson.SetBytes(out, "reference_images.-1.url", image)
			if err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func grokVideoAspectRatioOption(raw string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1:1", "square":
		return "1:1"
	case "16:9", "landscape":
		return "16:9"
	case "9:16", "portrait":
		return "9:16"
	case "4:3":
		return "4:3"
	case "3:4":
		return "3:4"
	case "3:2":
		return "3:2"
	case "2:3":
		return "2:3"
	default:
		return fallback
	}
}

func grokVideoResolutionOption(raw string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "480p":
		return "480p"
	case "720p":
		return "720p"
	default:
		return fallback
	}
}

func normalizeGrokOpenAIVideoCreateJSONRequest(body []byte) ([]byte, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, nil
	}
	if strings.TrimSpace(gjson.GetBytes(body, "prompt").String()) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	out, err := normalizeGrokVideoJSONRequest(body)
	if err != nil {
		return nil, err
	}
	duration := gjsonPositiveInt(out, "duration")
	if duration <= 0 {
		duration = 4
	}
	if duration < 1 {
		duration = 1
	}
	if duration > 15 {
		duration = 15
	}
	out, err = sjson.SetBytes(out, "duration", duration)
	if err != nil {
		return nil, err
	}
	aspectRatio := grokVideoAspectRatioOption(gjson.GetBytes(out, "aspect_ratio").String(), "9:16")
	out, err = sjson.SetBytes(out, "aspect_ratio", aspectRatio)
	if err != nil {
		return nil, err
	}
	resolution := grokVideoResolutionOption(gjson.GetBytes(out, "resolution").String(), "720p")
	out, err = sjson.SetBytes(out, "resolution", resolution)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func grokVideoSizeOptions(size string) (aspectRatio string, resolution string, err error) {
	switch strings.TrimSpace(size) {
	case "720x1280", "1024x1792":
		return "9:16", "720p", nil
	case "1280x720", "1792x1024":
		return "16:9", "720p", nil
	default:
		return "", "", fmt.Errorf("size must be one of 720x1280, 1280x720, 1024x1792, or 1792x1024")
	}
}

func grokVideoInputImageURL(body []byte) (string, error) {
	inputRef := gjson.GetBytes(body, "input_reference")
	if inputRef.Exists() {
		imageURL := strings.TrimSpace(inputRef.Get("image_url").String())
		fileID := strings.TrimSpace(inputRef.Get("file_id").String())
		if imageURL != "" && fileID != "" {
			return "", fmt.Errorf("input_reference must provide exactly one of image_url or file_id")
		}
		if fileID != "" {
			return "", fmt.Errorf("input_reference.file_id is not supported for xAI video generation; use input_reference.image_url")
		}
		if imageURL != "" {
			return imageURL, nil
		}
	}
	image := gjson.GetBytes(body, "image")
	if image.Exists() {
		if image.Type == gjson.String {
			return strings.TrimSpace(image.String()), nil
		}
		if url := strings.TrimSpace(image.Get("url").String()); url != "" {
			return url, nil
		}
		if url := strings.TrimSpace(image.Get("image_url.url").String()); url != "" {
			return url, nil
		}
	}
	return strings.TrimSpace(gjson.GetBytes(body, "image_url").String()), nil
}

func collectGrokVideoReferenceImages(body []byte) []string {
	out := make([]string, 0)
	appendRef := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	collectArray := func(result gjson.Result) {
		if !result.IsArray() {
			return
		}
		result.ForEach(func(_, item gjson.Result) bool {
			if item.Type == gjson.String {
				appendRef(item.String())
				return true
			}
			if value := item.Get("url").String(); value != "" {
				appendRef(value)
				return true
			}
			if value := item.Get("image_url.url").String(); value != "" {
				appendRef(value)
			}
			return true
		})
	}
	collectArray(gjson.GetBytes(body, "reference_images"))
	collectArray(gjson.GetBytes(body, "reference_image_urls"))
	return out
}

func prepareGrokMediaMultipartEditsBody(body []byte, contentType string) ([]byte, string, error) {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return body, contentType, nil
	}

	info := ParseGrokMediaRequest(contentType, body)
	payload := make(map[string]any)
	if info.Model != "" {
		payload["model"] = info.Model
	}
	if info.Prompt != "" {
		payload["prompt"] = info.Prompt
	}
	if info.N > 1 {
		payload["n"] = info.N
	}
	if info.Size != "" {
		payload["size"] = info.Size
	}

	// Emit official xAI Imagine image objects ({url, type:image_url}) so
	// multipart→JSON conversion matches docs.x.ai examples and SDK clients.
	images := make([]map[string]string, 0, len(info.InputImageURLs)+len(info.Uploads))
	for _, imageURL := range info.InputImageURLs {
		if imageURL = strings.TrimSpace(imageURL); imageURL != "" {
			images = append(images, grokMediaImageObject(imageURL))
		}
	}
	for _, upload := range info.Uploads {
		dataURL, err := openAIImageUploadToDataURL(upload)
		if err != nil {
			return nil, "", err
		}
		images = append(images, grokMediaImageObject(dataURL))
	}
	if len(images) > grokMediaMaxEditSourceImages {
		return nil, "", fmt.Errorf(
			"a maximum of %d source images is supported for image edits",
			grokMediaMaxEditSourceImages,
		)
	}
	if len(images) > 0 {
		payload["image"] = images[0]
		if len(images) > 1 {
			payload["images"] = images
		}
	}

	maskImageURL := strings.TrimSpace(info.MaskImageURL)
	if info.MaskUpload != nil {
		dataURL, err := openAIImageUploadToDataURL(*info.MaskUpload)
		if err != nil {
			return nil, "", err
		}
		maskImageURL = dataURL
	}
	if maskImageURL != "" {
		payload["mask"] = grokMediaImageObject(maskImageURL)
	}

	out, err := marshalOpenAIUpstreamJSON(payload)
	if err != nil {
		return nil, "", err
	}
	return out, "application/json", nil
}

// normalizeGrokMediaJSONImageRefs rewrites image/images/mask fields from common
// OpenAI client shapes into official xAI Imagine objects:
//
//	{"url":"...","type":"image_url"}
//
// Known inputs: plain string URL, {"image_url":"..."}, {"image_url":{"url":"..."}},
// and already-official {"url","type"}. Rejects more than 3 source images.
func normalizeGrokMediaJSONImageRefs(body []byte) ([]byte, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, nil
	}
	info := ParseGrokMediaRequest("application/json", body)
	if len(info.InputImageURLs) > grokMediaMaxEditSourceImages {
		return nil, fmt.Errorf(
			"a maximum of %d source images is supported for image edits",
			grokMediaMaxEditSourceImages,
		)
	}
	if !gjson.GetBytes(body, "image").Exists() &&
		!gjson.GetBytes(body, "images").Exists() &&
		!gjson.GetBytes(body, "mask").Exists() {
		return body, nil
	}

	out := body
	var err error
	if out, err = rewriteGrokMediaJSONImageField(out, "image"); err != nil {
		return nil, err
	}
	if out, err = rewriteGrokMediaJSONImageField(out, "images"); err != nil {
		return nil, err
	}
	if out, err = rewriteGrokMediaJSONImageField(out, "mask"); err != nil {
		return nil, err
	}
	return out, nil
}

func rewriteGrokMediaJSONImageField(body []byte, path string) ([]byte, error) {
	value := gjson.GetBytes(body, path)
	if !value.Exists() {
		return body, nil
	}
	if value.IsArray() {
		rewritten := make([]map[string]string, 0, len(value.Array()))
		changed := false
		for _, item := range value.Array() {
			imageURL := extractGrokMediaImageURL(item)
			if imageURL == "" {
				// Leave unparseable entries as-is by aborting rewrite of this field.
				return body, nil
			}
			obj := grokMediaImageObject(imageURL)
			rewritten = append(rewritten, obj)
			if !item.IsObject() ||
				strings.TrimSpace(item.Get("url").String()) != obj["url"] ||
				strings.TrimSpace(item.Get("type").String()) != obj["type"] {
				changed = true
			}
		}
		if !changed {
			return body, nil
		}
		out, err := sjson.SetBytes(body, path, rewritten)
		if err != nil {
			return nil, fmt.Errorf("rewrite grok media %s: %w", path, err)
		}
		return out, nil
	}

	imageURL := extractGrokMediaImageURL(value)
	if imageURL == "" {
		return body, nil
	}
	obj := grokMediaImageObject(imageURL)
	// Skip rewrite when already official {url,type} without legacy keys.
	if value.IsObject() &&
		strings.TrimSpace(value.Get("url").String()) == obj["url"] &&
		strings.TrimSpace(value.Get("type").String()) == obj["type"] &&
		!value.Get("image_url").Exists() {
		return body, nil
	}
	out, err := sjson.SetBytes(body, path, obj)
	if err != nil {
		return nil, fmt.Errorf("rewrite grok media %s: %w", path, err)
	}
	return out, nil
}

// resolveGrokMediaUpstreamModel applies account model mapping then endpoint
// Imagine aliases. originalModel is the client-facing model identity.
func resolveGrokMediaUpstreamModel(account *Account, endpoint GrokMediaEndpoint, originalModel string) string {
	originalModel = strings.TrimSpace(originalModel)
	mapped := originalModel
	if account != nil {
		mapped = account.GetMappedModel(originalModel)
	}
	return normalizeGrokMediaModelForEndpoint(endpoint, mapped)
}

// normalizeGrokMediaForwardBody rewrites the JSON model field for upstream while
// returning the original client model for billing/logging identity.
func normalizeGrokMediaForwardBody(
	account *Account,
	endpoint GrokMediaEndpoint,
	body []byte,
	contentType string,
) (out []byte, outContentType string, originalModel string, upstreamModel string, err error) {
	out = body
	outContentType = contentType
	if !endpoint.RequiresRequestBody() || !gjson.ValidBytes(body) {
		return out, outContentType, "", "", nil
	}
	originalModel = strings.TrimSpace(gjson.GetBytes(body, "model").String())
	upstreamModel = resolveGrokMediaUpstreamModel(account, endpoint, originalModel)
	if upstreamModel == "" || upstreamModel == originalModel {
		return out, outContentType, originalModel, upstreamModel, nil
	}
	out, err = sjson.SetBytes(body, "model", upstreamModel)
	if err != nil {
		return nil, "", originalModel, upstreamModel, fmt.Errorf("rewrite grok media model: %w", err)
	}
	return out, outContentType, originalModel, upstreamModel, nil
}

func normalizeGrokMediaModelForEndpoint(endpoint GrokMediaEndpoint, model string) string {
	model = xai.StripGrokProviderPrefix(model)
	switch endpoint {
	case GrokMediaEndpointImagesGenerations, GrokMediaEndpointImagesEdits:
		switch strings.ToLower(model) {
		case "grok-imagine", "grok-imagine-1", "grok-imagine-edit":
			return xai.DefaultImagineImageQualityModel
		}
	case GrokMediaEndpointVideosGenerations:
		switch strings.ToLower(model) {
		case "sora-2", "sora-2-pro", "grok-imagine-video", "grok-video", "grok-video-latest":
			return xai.DefaultImagineVideoModel
		case "grok-imagine-video-1.5", "grok-imagine-video-1.5-preview", "grok-video-1.5":
			return xai.DefaultImagineVideo15Model
		}
	}
	return model
}

type grokMediaUsageMetadata struct {
	ResponseID       string
	Usage            OpenAIUsage
	ImageCount       int
	ImageSize        string
	ImageInputSize   string
	ImageOutputSizes []string
	VideoCount       int
	VideoSeconds     int
	VideoSize        string
}

func grokMediaUsageFromResponse(endpoint GrokMediaEndpoint, requestInfo GrokMediaRequestInfo, responseBody []byte) grokMediaUsageMetadata {
	usage, _ := extractOpenAIUsageFromJSONBytes(responseBody)
	meta := grokMediaUsageMetadata{Usage: usage}
	switch endpoint {
	case GrokMediaEndpointImagesGenerations, GrokMediaEndpointImagesEdits:
		imageCount := countOpenAIResponseImageOutputsFromJSONBytes(responseBody)
		if imageCount <= 0 {
			imageCount = requestInfo.N
		}
		if imageCount <= 0 {
			imageCount = 1
		}
		meta.ImageCount = imageCount
		meta.ImageSize = requestInfo.SizeTier
		meta.ImageInputSize = requestInfo.Size
		meta.ImageOutputSizes = collectOpenAIResponseImageOutputSizesFromJSONBytes(responseBody)
	case GrokMediaEndpointVideosGenerations:
		meta.ResponseID = extractGrokMediaVideoRequestID(responseBody)
		meta.VideoCount = 1
		if n := requestInfo.N; n > 1 {
			meta.VideoCount = n
		}
		// Prefer explicit duration from response; fall back to documented default
		// so pending creates do not under-bill as 1s via calculateOpenAIVideoRequestCost.
		meta.VideoSeconds = resolveGrokMediaVideoSeconds(
			gjsonPositiveInt(responseBody, "video.duration"),
			gjsonPositiveInt(responseBody, "duration"),
			gjsonPositiveInt(responseBody, "seconds"),
		)
		meta.VideoSize = NormalizeVideoBillingTierOrDefault(firstNonEmptyString(
			requestInfo.Size,
			strings.TrimSpace(gjson.GetBytes(responseBody, "video.resolution").String()),
			strings.TrimSpace(gjson.GetBytes(responseBody, "resolution").String()),
			strings.TrimSpace(gjson.GetBytes(responseBody, "size").String()),
		))
	}
	return meta
}

func writeOpenAIVideoFailedResponse(c *gin.Context, statusCode int, model string, code string, message string) {
	if c == nil || c.Writer == nil || c.Writer.Written() {
		return
	}
	if statusCode <= 0 {
		statusCode = http.StatusBadRequest
	}
	body := buildOpenAIVideoFailedResponse(model, code, message)
	c.Data(statusCode, "application/json", body)
}

func buildOpenAIVideoFailedResponse(model string, code string, message string) []byte {
	model = responseGrokVideoModel(model)
	code = strings.TrimSpace(code)
	if code == "" {
		code = "invalid_request_error"
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "Video generation failed"
	}
	out := []byte(`{"object":"video","status":"failed","progress":0}`)
	out, _ = sjson.SetBytes(out, "id", fmt.Sprintf("video_%d", time.Now().UnixNano()))
	out, _ = sjson.SetBytes(out, "model", model)
	out, _ = sjson.SetBytes(out, "error.code", code)
	out, _ = sjson.SetBytes(out, "error.message", message)
	return out
}

func isOpenAIVideoCreateRequest(method, targetPath string) bool {
	if !strings.EqualFold(strings.TrimSpace(method), http.MethodPost) {
		return false
	}
	canonical := canonicalOpenAIVideoTargetPath(targetPath)
	if idx := strings.IndexAny(canonical, "?#"); idx >= 0 {
		canonical = canonical[:idx]
	}
	return strings.EqualFold(strings.TrimRight(canonical, "/"), "/v1/videos")
}

func buildOpenAIVideoCreateResponseFromGrok(payload []byte, requestBody []byte, upstreamRequestBody []byte, fallbackModel string) ([]byte, error) {
	requestID := strings.TrimSpace(gjson.GetBytes(payload, "request_id").String())
	if requestID == "" {
		requestID = strings.TrimSpace(gjson.GetBytes(payload, "id").String())
	}
	if requestID == "" {
		return nil, fmt.Errorf("xAI video response did not include request_id")
	}

	out := []byte(`{"object":"video","progress":0,"status":"queued"}`)
	out, _ = sjson.SetBytes(out, "id", requestID)
	out, _ = sjson.SetBytes(out, "model", responseGrokVideoModel(fallbackModel))
	if prompt := strings.TrimSpace(gjson.GetBytes(requestBody, "prompt").String()); prompt != "" {
		out, _ = sjson.SetBytes(out, "prompt", prompt)
	}
	out, _ = sjson.SetBytes(out, "seconds", grokOpenAIVideoCreateSeconds(upstreamRequestBody, requestBody))
	out, _ = sjson.SetBytes(out, "size", grokOpenAIVideoCreateSize(requestBody))
	out, _ = sjson.SetBytes(out, "created_at", time.Now().Unix())
	if status := openAIGrokVideoStatus(gjson.GetBytes(payload, "status").String()); status != "" {
		out, _ = sjson.SetBytes(out, "status", status)
	}
	if progress := gjson.GetBytes(payload, "progress"); progress.Exists() {
		out, _ = sjson.SetRawBytes(out, "progress", []byte(progress.Raw))
	}
	return out, nil
}

func grokOpenAIVideoCreateSeconds(bodies ...[]byte) string {
	for _, requestBody := range bodies {
		for _, path := range []string{"duration", "seconds", "duration_seconds"} {
			value := gjson.GetBytes(requestBody, path)
			if !value.Exists() {
				continue
			}
			if n := value.Int(); n > 0 {
				return strconv.FormatInt(n, 10)
			}
			if raw := strings.TrimSpace(value.String()); raw != "" {
				if n, err := strconv.Atoi(raw); err == nil && n > 0 {
					return strconv.Itoa(n)
				}
			}
		}
	}
	return "4"
}

func grokOpenAIVideoCreateSize(requestBody []byte) string {
	if size := strings.TrimSpace(gjson.GetBytes(requestBody, "size").String()); size != "" {
		return size
	}
	switch strings.ToLower(strings.TrimSpace(gjson.GetBytes(requestBody, "aspect_ratio").String())) {
	case "16:9", "landscape":
		return "1280x720"
	case "9:16", "portrait":
		return "720x1280"
	}
	return "720x1280"
}

func buildOpenAIVideoRetrieveResponseFromGrok(videoID string, payload []byte, fallbackModel string) ([]byte, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		videoID = extractGrokMediaVideoRequestID(payload)
	}
	out := []byte(`{"object":"video"}`)
	if videoID != "" {
		out, _ = sjson.SetBytes(out, "id", videoID)
	}
	model := strings.TrimSpace(gjson.GetBytes(payload, "model").String())
	if model == "" {
		model = responseGrokVideoModel(fallbackModel)
	}
	out, _ = sjson.SetBytes(out, "model", model)

	for _, field := range []string{"created_at", "completed_at", "expires_at", "prompt", "remixed_from_video_id", "size"} {
		if value := gjson.GetBytes(payload, field); value.Exists() {
			out, _ = sjson.SetRawBytes(out, field, []byte(value.Raw))
		}
	}

	if status := openAIGrokVideoStatus(gjson.GetBytes(payload, "status").String()); status != "" {
		out, _ = sjson.SetBytes(out, "status", status)
	}
	if progress := gjson.GetBytes(payload, "progress"); progress.Exists() {
		out, _ = sjson.SetRawBytes(out, "progress", []byte(progress.Raw))
	}
	if seconds := gjson.GetBytes(payload, "seconds"); seconds.Exists() {
		out, _ = sjson.SetRawBytes(out, "seconds", []byte(seconds.Raw))
	} else if duration := gjson.GetBytes(payload, "video.duration"); duration.Exists() {
		out, _ = sjson.SetBytes(out, "seconds", duration.String())
	}
	if videoURL := strings.TrimSpace(gjson.GetBytes(payload, "video.url").String()); videoURL != "" {
		out, _ = sjson.SetBytes(out, "video_url", videoURL)
	}
	out = setOpenAIVideoErrorFromGrok(out, payload)
	return out, nil
}

func isGrokOpenAISoraVideoModel(model string) bool {
	base := strings.ToLower(strings.TrimSpace(xai.StripGrokProviderPrefix(model)))
	return base == "sora-2" || strings.HasPrefix(base, "sora-2-")
}

func isSupportedGrokOpenAIVideoModel(model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return true
	}
	if isGrokOpenAISoraVideoModel(model) {
		return true
	}
	prefix, base := grokVideoProviderModelParts(model)
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix != "" && prefix != "xai" && prefix != "x-ai" && prefix != "grok" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(base)) {
	case xai.DefaultImagineVideoModel, "grok-video", "grok-video-latest",
		xai.DefaultImagineVideo15LegacyModel, xai.DefaultImagineVideo15Model, "grok-video-1.5":
		return true
	default:
		return false
	}
}

func isSupportedGrokNativeVideoModel(model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return true
	}
	prefix, base := grokVideoProviderModelParts(model)
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix != "" && prefix != "xai" && prefix != "x-ai" && prefix != "grok" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(base)) {
	case xai.DefaultImagineVideoModel, xai.DefaultImagineVideo15Model:
		return true
	default:
		return false
	}
}

func grokVideoProviderModelParts(model string) (string, string) {
	trimmed := strings.TrimSpace(model)
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		return strings.ToLower(strings.TrimSpace(trimmed[:idx])), strings.TrimSpace(trimmed[idx+1:])
	}
	return "", trimmed
}

func responseGrokVideoModel(model string) string {
	model = normalizeGrokMediaModelForEndpoint(GrokMediaEndpointVideosGenerations, model)
	if strings.TrimSpace(model) == "" {
		return xai.DefaultImagineVideoModel
	}
	return model
}

func setOpenAIVideoErrorFromGrok(out []byte, payload []byte) []byte {
	if errPayload := gjson.GetBytes(payload, "error"); errPayload.Exists() {
		out = markOpenAIVideoFailed(out)
		if errPayload.Type == gjson.JSON && json.Valid([]byte(errPayload.Raw)) {
			message := strings.TrimSpace(errPayload.Get("message").String())
			if message != "" {
				code := strings.TrimSpace(gjson.GetBytes(payload, "code").String())
				if code == "" {
					code = strings.TrimSpace(errPayload.Get("code").String())
				}
				if code == "" {
					code = "video_generation_failed"
				}
				out, _ = sjson.SetBytes(out, "error.code", code)
				out, _ = sjson.SetBytes(out, "error.message", message)
			}
			return out
		}
		message := strings.TrimSpace(errPayload.String())
		if message != "" {
			code := strings.TrimSpace(gjson.GetBytes(payload, "code").String())
			if code == "" {
				code = "video_generation_failed"
			}
			out, _ = sjson.SetBytes(out, "error.code", code)
			out, _ = sjson.SetBytes(out, "error.message", message)
		}
		return out
	}

	code := strings.TrimSpace(gjson.GetBytes(payload, "code").String())
	if code != "" {
		out = markOpenAIVideoFailed(out)
		out, _ = sjson.SetBytes(out, "error.code", code)
		out, _ = sjson.SetBytes(out, "error.message", code)
	}
	return out
}

func markOpenAIVideoFailed(out []byte) []byte {
	if !gjson.GetBytes(out, "status").Exists() {
		out, _ = sjson.SetBytes(out, "status", "failed")
	}
	if !gjson.GetBytes(out, "progress").Exists() {
		out, _ = sjson.SetRawBytes(out, "progress", []byte("0"))
	}
	return out
}

func openAIGrokVideoStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "pending":
		return "queued"
	case "in_progress", "processing", "running":
		return "in_progress"
	case "completed", "done", "succeeded", "success":
		return "completed"
	case "failed", "error", "expired", "cancelled", "canceled":
		return "failed"
	default:
		return ""
	}
}

func extractGrokMediaVideoRequestID(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	for _, path := range []string{"request_id", "id", "data.request_id", "data.id", "video.request_id", "video.id"} {
		if id := strings.TrimSpace(gjson.GetBytes(body, path).String()); id != "" {
			return id
		}
	}
	return ""
}

// resolveGrokMediaVideoSeconds picks the first positive candidate, otherwise the
// shared default used by ForwardGrokMedia and ForwardVideos billing paths.
func resolveGrokMediaVideoSeconds(candidates ...int) int {
	if seconds := firstPositiveInt(candidates...); seconds > 0 {
		return seconds
	}
	return grokMediaDefaultVideoSeconds
}

func (s *OpenAIGatewayService) handleGrokMediaErrorResponse(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
	requestIDHeader string,
	requestedModel string,
) (*OpenAIForwardResult, error) {
	body := s.readUpstreamErrorBody(resp)
	upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	if upstreamMsg == "" {
		upstreamMsg = fmt.Sprintf("xAI upstream returned status %d", resp.StatusCode)
	}

	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(body), maxBytes)
	}
	setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)

	if status, errType, errMsg, matched := applyErrorPassthroughRule(
		c,
		account.Platform,
		resp.StatusCode,
		body,
		http.StatusBadGateway,
		"upstream_error",
		"Upstream request failed",
	); matched {
		MarkResponseCommitted(c)
		writeGrokMediaErrorResponse(c, status, errType, errMsg)
		return nil, fmt.Errorf("upstream error: %d (passthrough rule matched) message=%s", resp.StatusCode, upstreamMsg)
	}

	if !account.ShouldHandleErrorCode(resp.StatusCode) {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  requestIDHeader,
			Kind:               "http_error",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		MarkResponseCommitted(c)
		writeGrokMediaErrorResponse(c, http.StatusInternalServerError, "upstream_error", "Upstream gateway error")
		return nil, fmt.Errorf("upstream error: %d (not in custom error codes) message=%s", resp.StatusCode, upstreamMsg)
	}

	s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, body)
	kind := "http_error"
	if s.shouldFailoverUpstreamError(resp.StatusCode) {
		kind = "failover"
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  requestIDHeader,
		Kind:               kind,
		Message:            upstreamMsg,
		Detail:             upstreamDetail,
	})
	if kind == "failover" {
		return nil, &UpstreamFailoverError{
			StatusCode:             resp.StatusCode,
			ResponseBody:           body,
			RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
		}
	}

	MarkResponseCommitted(c)
	writeGrokMediaErrorResponse(c, resp.StatusCode, grokMediaErrorType(resp.StatusCode), upstreamMsg)
	return nil, fmt.Errorf("upstream error: %d %s", resp.StatusCode, upstreamMsg)
}

func grokMediaErrorType(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	default:
		return "upstream_error"
	}
}

func writeGrokMediaErrorResponse(c *gin.Context, statusCode int, errType, message string) {
	if c == nil || c.Writer == nil || c.Writer.Written() {
		return
	}
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"type":    strings.TrimSpace(errType),
			"message": strings.TrimSpace(message),
		},
	})
}

func writeGrokMediaResponse(c *gin.Context, resp *http.Response, body []byte, filter *responseheaders.CompiledHeaderFilter) {
	if c == nil || resp == nil {
		return
	}
	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, filter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, body)
}
