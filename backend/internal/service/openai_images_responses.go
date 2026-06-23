package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type openAIResponsesImageResult struct {
	Result        string
	RevisedPrompt string
	OutputFormat  string
	Size          string
	Background    string
	Quality       string
	Model         string
}

type openAIImagesOAuthResponse struct {
	Results       []openAIResponsesImageResult
	CreatedAt     int64
	UsageRaw      []byte
	FirstMeta     openAIResponsesImageResult
	Usage         OpenAIUsage
	UpstreamReqID string
	StatusCode    int
	Header        http.Header
}

type openAIImagesOAuthFanOutAttempt struct {
	Response *openAIImagesOAuthResponse
	Err      error
	Context  *gin.Context
}

func normalizeOpenAIImagesResponseFormat(responseFormat string) string {
	format := strings.ToLower(strings.TrimSpace(responseFormat))
	switch format {
	case "", "b64_json":
		return "b64_json"
	case "url":
		return "url"
	default:
		return "b64_json"
	}
}

func buildOpenAIImagesUsageJSON(usage OpenAIUsage, imageCount int) []byte {
	if usage.InputTokens <= 0 && usage.OutputTokens <= 0 && usage.CacheReadInputTokens <= 0 && usage.CacheCreationInputTokens <= 0 && usage.ImageOutputTokens <= 0 && imageCount <= 0 {
		return nil
	}
	out := []byte(`{"input_tokens":0,"output_tokens":0}`)
	out, _ = sjson.SetBytes(out, "input_tokens", usage.InputTokens)
	out, _ = sjson.SetBytes(out, "output_tokens", usage.OutputTokens)
	if usage.CacheCreationInputTokens > 0 {
		out, _ = sjson.SetBytes(out, "input_tokens_details.cached_tokens_internal", usage.CacheCreationInputTokens)
	}
	if usage.CacheReadInputTokens > 0 {
		out, _ = sjson.SetBytes(out, "input_tokens_details.cached_tokens", usage.CacheReadInputTokens)
	}
	if usage.ImageOutputTokens > 0 {
		out, _ = sjson.SetBytes(out, "output_tokens_details.image_tokens", usage.ImageOutputTokens)
	}
	if imageCount > 0 {
		out, _ = sjson.SetBytes(out, "images", imageCount)
	}
	return out
}

func mergeOpenAIImagesUsageJSON(primary []byte, fallback []byte) []byte {
	if len(primary) > 0 && gjson.ValidBytes(primary) {
		return primary
	}
	if len(fallback) > 0 && gjson.ValidBytes(fallback) {
		return fallback
	}
	return nil
}

func openAIResponsesImageResultKey(itemID string, result openAIResponsesImageResult) string {
	if strings.TrimSpace(result.Result) != "" {
		return strings.TrimSpace(result.OutputFormat) + "|" + strings.TrimSpace(result.Result)
	}
	return "item:" + strings.TrimSpace(itemID)
}

func appendOpenAIResponsesImageResultDedup(results *[]openAIResponsesImageResult, seen map[string]struct{}, itemID string, result openAIResponsesImageResult) bool {
	if results == nil {
		return false
	}
	key := openAIResponsesImageResultKey(itemID, result)
	if key != "" {
		if _, exists := seen[key]; exists {
			return false
		}
		seen[key] = struct{}{}
	}
	*results = append(*results, result)
	return true
}

func mergeOpenAIResponsesImageMeta(dst *openAIResponsesImageResult, src openAIResponsesImageResult) {
	if dst == nil {
		return
	}
	if trimmed := strings.TrimSpace(src.OutputFormat); trimmed != "" {
		dst.OutputFormat = trimmed
	}
	if trimmed := strings.TrimSpace(src.Size); trimmed != "" {
		dst.Size = trimmed
	}
	if trimmed := strings.TrimSpace(src.Background); trimmed != "" {
		dst.Background = trimmed
	}
	if trimmed := strings.TrimSpace(src.Quality); trimmed != "" {
		dst.Quality = trimmed
	}
	if trimmed := strings.TrimSpace(src.Model); trimmed != "" {
		dst.Model = trimmed
	}
}

func extractOpenAIResponsesImageMetaFromLifecycleEvent(payload []byte) (openAIResponsesImageResult, int64, bool) {
	switch gjson.GetBytes(payload, "type").String() {
	case "response.created", "response.in_progress", "response.completed":
	default:
		return openAIResponsesImageResult{}, 0, false
	}

	response := gjson.GetBytes(payload, "response")
	if !response.Exists() {
		return openAIResponsesImageResult{}, 0, false
	}

	meta := openAIResponsesImageResult{
		OutputFormat: strings.TrimSpace(response.Get("tools.0.output_format").String()),
		Size:         strings.TrimSpace(response.Get("tools.0.size").String()),
		Background:   strings.TrimSpace(response.Get("tools.0.background").String()),
		Quality:      strings.TrimSpace(response.Get("tools.0.quality").String()),
		Model:        strings.TrimSpace(response.Get("tools.0.model").String()),
	}
	return meta, response.Get("created_at").Int(), true
}

func buildOpenAIImagesStreamPartialPayload(
	eventType string,
	b64 string,
	partialImageIndex int64,
	responseFormat string,
	createdAt int64,
	meta openAIResponsesImageResult,
) []byte {
	if createdAt <= 0 {
		createdAt = time.Now().Unix()
	}

	payload := []byte(`{"type":"","created_at":0,"partial_image_index":0,"b64_json":""}`)
	payload, _ = sjson.SetBytes(payload, "type", eventType)
	payload, _ = sjson.SetBytes(payload, "created_at", createdAt)
	payload, _ = sjson.SetBytes(payload, "partial_image_index", partialImageIndex)
	payload, _ = sjson.SetBytes(payload, "b64_json", b64)
	if strings.EqualFold(strings.TrimSpace(responseFormat), "url") {
		payload, _ = sjson.SetBytes(payload, "url", "data:"+openAIImageOutputMIMEType(meta.OutputFormat)+";base64,"+b64)
	}
	if meta.Background != "" {
		payload, _ = sjson.SetBytes(payload, "background", meta.Background)
	}
	if meta.OutputFormat != "" {
		payload, _ = sjson.SetBytes(payload, "output_format", meta.OutputFormat)
	}
	if meta.Quality != "" {
		payload, _ = sjson.SetBytes(payload, "quality", meta.Quality)
	}
	if meta.Size != "" {
		payload, _ = sjson.SetBytes(payload, "size", meta.Size)
	}
	if meta.Model != "" {
		payload, _ = sjson.SetBytes(payload, "model", meta.Model)
	}
	return payload
}

func buildOpenAIImagesStreamCompletedPayload(
	eventType string,
	img openAIResponsesImageResult,
	responseFormat string,
	createdAt int64,
	usageRaw []byte,
) []byte {
	if createdAt <= 0 {
		createdAt = time.Now().Unix()
	}

	payload := []byte(`{"type":"","created_at":0,"b64_json":""}`)
	payload, _ = sjson.SetBytes(payload, "type", eventType)
	payload, _ = sjson.SetBytes(payload, "created_at", createdAt)
	payload, _ = sjson.SetBytes(payload, "b64_json", img.Result)
	if strings.EqualFold(strings.TrimSpace(responseFormat), "url") {
		payload, _ = sjson.SetBytes(payload, "url", "data:"+openAIImageOutputMIMEType(img.OutputFormat)+";base64,"+img.Result)
	}
	if img.Background != "" {
		payload, _ = sjson.SetBytes(payload, "background", img.Background)
	}
	if img.OutputFormat != "" {
		payload, _ = sjson.SetBytes(payload, "output_format", img.OutputFormat)
	}
	if img.Quality != "" {
		payload, _ = sjson.SetBytes(payload, "quality", img.Quality)
	}
	if img.Size != "" {
		payload, _ = sjson.SetBytes(payload, "size", img.Size)
	}
	if img.Model != "" {
		payload, _ = sjson.SetBytes(payload, "model", img.Model)
	}
	if len(usageRaw) > 0 && gjson.ValidBytes(usageRaw) {
		payload, _ = sjson.SetRawBytes(payload, "usage", usageRaw)
	}
	return payload
}

func openAIImageOutputMIMEType(outputFormat string) string {
	if outputFormat == "" {
		return "image/png"
	}
	if strings.Contains(outputFormat, "/") {
		return outputFormat
	}
	switch strings.ToLower(strings.TrimSpace(outputFormat)) {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func openAIImageUploadToDataURL(upload OpenAIImagesUpload) (string, error) {
	if len(upload.Data) == 0 {
		return "", fmt.Errorf("upload %q is empty", strings.TrimSpace(upload.FileName))
	}
	contentType := strings.TrimSpace(upload.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(upload.Data)
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(upload.Data), nil
}

func openAIImagesResponsesEffectiveN(parsed *OpenAIImagesRequest) int {
	if parsed == nil {
		return 1
	}
	if parsed.N <= 0 {
		return 1
	}
	return parsed.N
}

func buildOpenAIImagesResponsesRequest(parsed *OpenAIImagesRequest, toolModel, mainModel string) ([]byte, error) {
	if parsed == nil {
		return nil, fmt.Errorf("parsed images request is required")
	}
	prompt := strings.TrimSpace(parsed.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	type openAIResponsesInputImage struct {
		ImageURL string
		FileID   string
	}
	orderedInputImages := parsed.orderedInputImages()
	inputImages := make([]openAIResponsesInputImage, 0, len(orderedInputImages)+len(parsed.Uploads))
	for _, image := range orderedInputImages {
		if trimmed := strings.TrimSpace(image.ImageURL); trimmed != "" {
			inputImages = append(inputImages, openAIResponsesInputImage{ImageURL: trimmed})
			continue
		}
		if trimmed := strings.TrimSpace(image.FileID); trimmed != "" {
			inputImages = append(inputImages, openAIResponsesInputImage{FileID: trimmed})
		}
	}
	for _, upload := range parsed.Uploads {
		dataURL, err := openAIImageUploadToDataURL(upload)
		if err != nil {
			return nil, err
		}
		inputImages = append(inputImages, openAIResponsesInputImage{ImageURL: dataURL})
	}
	if parsed.IsEdits() && len(inputImages) == 0 {
		return nil, fmt.Errorf("image input is required")
	}

	req := []byte(`{"instructions":"","stream":true,"reasoning":{"effort":"medium","summary":"auto"},"parallel_tool_calls":true,"include":["reasoning.encrypted_content"],"model":"","store":false}`)
	req, _ = sjson.SetBytes(req, "model", NormalizeOpenAIImageMainModel(mainModel))
	// Responses-tool image generation currently requires upstream SSE even when the
	// downstream client requested a synchronous image response.
	req, _ = sjson.SetBytes(req, "stream", true)

	input := []byte(`[{"type":"message","role":"user","content":[{"type":"input_text","text":""}]}]`)
	input, _ = sjson.SetBytes(input, "0.content.0.text", prompt)
	for index, imageRef := range inputImages {
		part := []byte(`{"type":"input_image"}`)
		if strings.TrimSpace(imageRef.FileID) != "" {
			part, _ = sjson.SetBytes(part, "file_id", strings.TrimSpace(imageRef.FileID))
		} else {
			part, _ = sjson.SetBytes(part, "image_url", strings.TrimSpace(imageRef.ImageURL))
		}
		input, _ = sjson.SetRawBytes(input, fmt.Sprintf("0.content.%d", index+1), part)
	}
	req, _ = sjson.SetRawBytes(req, "input", input)

	action := "generate"
	if parsed.IsEdits() {
		action = "edit"
	}
	tool := []byte(`{"type":"image_generation","action":"","model":""}`)
	tool, _ = sjson.SetBytes(tool, "action", action)
	tool, _ = sjson.SetBytes(tool, "model", strings.TrimSpace(toolModel))

	for _, field := range []struct {
		path  string
		value string
	}{
		{path: "size", value: parsed.Size},
		{path: "quality", value: parsed.Quality},
		{path: "background", value: parsed.Background},
		{path: "output_format", value: parsed.OutputFormat},
		{path: "moderation", value: parsed.Moderation},
	} {
		if trimmed := strings.TrimSpace(field.value); trimmed != "" {
			tool, _ = sjson.SetBytes(tool, field.path, trimmed)
		}
	}
	if parsed.OutputCompression != nil {
		tool, _ = sjson.SetBytes(tool, "output_compression", *parsed.OutputCompression)
	}
	if parsed.PartialImages != nil {
		tool, _ = sjson.SetBytes(tool, "partial_images", *parsed.PartialImages)
	}

	maskImageURL := strings.TrimSpace(parsed.MaskImageURL)
	maskFileID := strings.TrimSpace(parsed.MaskFileID)
	if parsed.MaskUpload != nil {
		dataURL, err := openAIImageUploadToDataURL(*parsed.MaskUpload)
		if err != nil {
			return nil, err
		}
		maskImageURL = dataURL
		maskFileID = ""
	}
	if maskFileID != "" {
		tool, _ = sjson.SetBytes(tool, "input_image_mask.file_id", maskFileID)
	} else if maskImageURL != "" {
		tool, _ = sjson.SetBytes(tool, "input_image_mask.image_url", maskImageURL)
	}

	req, _ = sjson.SetRawBytes(req, "tools", []byte(`[]`))
	req, _ = sjson.SetRawBytes(req, "tools.-1", tool)
	req, _ = sjson.SetBytes(req, "tool_choice.type", "image_generation")
	return req, nil
}

func extractOpenAIImagesFromResponsesCompleted(payload []byte) ([]openAIResponsesImageResult, int64, []byte, openAIResponsesImageResult, error) {
	if gjson.GetBytes(payload, "type").String() != "response.completed" {
		return nil, 0, nil, openAIResponsesImageResult{}, fmt.Errorf("unexpected event type")
	}

	createdAt := gjson.GetBytes(payload, "response.created_at").Int()
	if createdAt <= 0 {
		createdAt = time.Now().Unix()
	}

	var (
		results   []openAIResponsesImageResult
		firstMeta openAIResponsesImageResult
	)
	output := gjson.GetBytes(payload, "response.output")
	if output.IsArray() {
		for _, item := range output.Array() {
			if item.Get("type").String() != "image_generation_call" {
				continue
			}
			result := strings.TrimSpace(item.Get("result").String())
			if result == "" {
				continue
			}
			entry := openAIResponsesImageResult{
				Result:        result,
				RevisedPrompt: strings.TrimSpace(item.Get("revised_prompt").String()),
				OutputFormat:  strings.TrimSpace(item.Get("output_format").String()),
				Size:          strings.TrimSpace(item.Get("size").String()),
				Background:    strings.TrimSpace(item.Get("background").String()),
				Quality:       strings.TrimSpace(item.Get("quality").String()),
			}
			if len(results) == 0 {
				firstMeta = entry
			}
			results = append(results, entry)
		}
	}

	var usageRaw []byte
	if usage := gjson.GetBytes(payload, "response.tool_usage.image_gen"); usage.Exists() && usage.IsObject() {
		usageRaw = []byte(usage.Raw)
	}
	return results, createdAt, usageRaw, firstMeta, nil
}

func extractOpenAIImageFromResponsesOutputItemDone(payload []byte) (openAIResponsesImageResult, string, bool, error) {
	if gjson.GetBytes(payload, "type").String() != "response.output_item.done" {
		return openAIResponsesImageResult{}, "", false, fmt.Errorf("unexpected event type")
	}

	item := gjson.GetBytes(payload, "item")
	if !item.Exists() || item.Get("type").String() != "image_generation_call" {
		return openAIResponsesImageResult{}, "", false, nil
	}

	result := strings.TrimSpace(item.Get("result").String())
	if result == "" {
		return openAIResponsesImageResult{}, "", false, nil
	}

	entry := openAIResponsesImageResult{
		Result:        result,
		RevisedPrompt: strings.TrimSpace(item.Get("revised_prompt").String()),
		OutputFormat:  strings.TrimSpace(item.Get("output_format").String()),
		Size:          strings.TrimSpace(item.Get("size").String()),
		Background:    strings.TrimSpace(item.Get("background").String()),
		Quality:       strings.TrimSpace(item.Get("quality").String()),
	}
	return entry, strings.TrimSpace(item.Get("id").String()), true, nil
}

func extractOpenAIImagesFromResponsesJSONBody(body []byte) ([]openAIResponsesImageResult, int64, []byte, openAIResponsesImageResult, bool, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil, 0, nil, openAIResponsesImageResult{}, false, nil
	}
	status := strings.TrimSpace(gjson.GetBytes(body, "status").String())
	if status == "" {
		return nil, 0, nil, openAIResponsesImageResult{}, false, nil
	}
	if status == "failed" {
		msg := extractOpenAISSEErrorMessage(body)
		if msg == "" {
			msg = "upstream image response failed"
		}
		return nil, 0, nil, openAIResponsesImageResult{}, true, fmt.Errorf("%s", msg)
	}
	if status != "completed" && status != "incomplete" {
		return nil, 0, nil, openAIResponsesImageResult{}, false, nil
	}
	payload := []byte(`{"type":"","response":{}}`)
	payload, _ = sjson.SetBytes(payload, "type", "response.completed")
	payload, _ = sjson.SetRawBytes(payload, "response", body)
	results, createdAt, usageRaw, firstMeta, err := extractOpenAIImagesFromResponsesCompleted(payload)
	if err != nil {
		return nil, 0, nil, openAIResponsesImageResult{}, true, err
	}
	if len(results) == 0 {
		return nil, createdAt, usageRaw, firstMeta, true, nil
	}
	return results, createdAt, usageRaw, firstMeta, true, nil
}

func collectOpenAIImagesFromResponsesBody(body []byte) ([]openAIResponsesImageResult, int64, []byte, openAIResponsesImageResult, bool, error) {
	if results, createdAt, usageRaw, firstMeta, foundFinal, err := extractOpenAIImagesFromResponsesJSONBody(body); foundFinal || err != nil {
		return results, createdAt, usageRaw, firstMeta, foundFinal, err
	}

	var (
		fallbackResults []openAIResponsesImageResult
		fallbackSeen    = make(map[string]struct{})
		finalResults    []openAIResponsesImageResult
		createdAt       int64
		usageRaw        []byte
		foundFinal      bool
		collectErr      error
		responseMeta    openAIResponsesImageResult
	)

	forEachOpenAISSEDataPayload(string(body), func(payload []byte) {
		if collectErr != nil || foundFinal || len(payload) == 0 || !gjson.ValidBytes(payload) {
			return
		}
		if meta, eventCreatedAt, ok := extractOpenAIResponsesImageMetaFromLifecycleEvent(payload); ok {
			mergeOpenAIResponsesImageMeta(&responseMeta, meta)
			if eventCreatedAt > 0 {
				createdAt = eventCreatedAt
			}
		}

		switch gjson.GetBytes(payload, "type").String() {
		case "response.failed":
			msg := extractOpenAISSEErrorMessage(payload)
			if msg == "" {
				msg = "upstream image response failed"
			}
			foundFinal = true
			collectErr = fmt.Errorf("%s", msg)
		case "response.output_item.done":
			result, itemID, ok, err := extractOpenAIImageFromResponsesOutputItemDone(payload)
			if err != nil {
				collectErr = err
				return
			}
			if ok {
				mergeOpenAIResponsesImageMeta(&result, responseMeta)
				appendOpenAIResponsesImageResultDedup(&fallbackResults, fallbackSeen, itemID, result)
			}
		case "response.completed":
			results, completedAt, completedUsageRaw, firstMeta, err := extractOpenAIImagesFromResponsesCompleted(payload)
			if err != nil {
				collectErr = err
				return
			}
			foundFinal = true
			if completedAt > 0 {
				createdAt = completedAt
			}
			if len(completedUsageRaw) > 0 {
				usageRaw = completedUsageRaw
			}
			if len(results) > 0 {
				mergeOpenAIResponsesImageMeta(&firstMeta, responseMeta)
				finalResults = results
				responseMeta = firstMeta
				return
			}
			if len(fallbackResults) > 0 {
				finalResults = fallbackResults
				responseMeta = fallbackResults[0]
				mergeOpenAIResponsesImageMeta(&responseMeta, firstMeta)
				return
			}
		}
	})
	if collectErr != nil {
		return nil, createdAt, usageRaw, openAIResponsesImageResult{}, foundFinal, collectErr
	}
	if len(finalResults) > 0 {
		firstMeta := finalResults[0]
		mergeOpenAIResponsesImageMeta(&firstMeta, responseMeta)
		return finalResults, createdAt, usageRaw, firstMeta, foundFinal, nil
	}

	if len(fallbackResults) > 0 {
		firstMeta := fallbackResults[0]
		mergeOpenAIResponsesImageMeta(&firstMeta, responseMeta)
		return fallbackResults, createdAt, usageRaw, firstMeta, foundFinal, nil
	}
	return nil, createdAt, usageRaw, openAIResponsesImageResult{}, foundFinal, nil
}

func buildOpenAIImagesAPIResponse(
	results []openAIResponsesImageResult,
	createdAt int64,
	usageRaw []byte,
	firstMeta openAIResponsesImageResult,
	responseFormat string,
) ([]byte, error) {
	if createdAt <= 0 {
		createdAt = time.Now().Unix()
	}
	out := []byte(`{"created":0,"data":[]}`)
	out, _ = sjson.SetBytes(out, "created", createdAt)

	format := normalizeOpenAIImagesResponseFormat(responseFormat)
	for _, img := range results {
		item := []byte(`{}`)
		if format == "url" {
			item, _ = sjson.SetBytes(item, "url", "data:"+openAIImageOutputMIMEType(img.OutputFormat)+";base64,"+img.Result)
		} else {
			item, _ = sjson.SetBytes(item, "b64_json", img.Result)
		}
		if img.RevisedPrompt != "" {
			item, _ = sjson.SetBytes(item, "revised_prompt", img.RevisedPrompt)
		}
		out, _ = sjson.SetRawBytes(out, "data.-1", item)
	}
	if firstMeta.Background != "" {
		out, _ = sjson.SetBytes(out, "background", firstMeta.Background)
	}
	if firstMeta.OutputFormat != "" {
		out, _ = sjson.SetBytes(out, "output_format", firstMeta.OutputFormat)
	}
	if firstMeta.Quality != "" {
		out, _ = sjson.SetBytes(out, "quality", firstMeta.Quality)
	}
	if firstMeta.Size != "" {
		out, _ = sjson.SetBytes(out, "size", firstMeta.Size)
	}
	if firstMeta.Model != "" {
		out, _ = sjson.SetBytes(out, "model", firstMeta.Model)
	}
	if len(usageRaw) > 0 && gjson.ValidBytes(usageRaw) {
		out, _ = sjson.SetRawBytes(out, "usage", usageRaw)
	}
	return out, nil
}

func openAIImagesStreamPrefix(parsed *OpenAIImagesRequest) string {
	if parsed != nil && parsed.IsEdits() {
		return "image_edit"
	}
	return "image_generation"
}

func addOpenAIUsage(dst *OpenAIUsage, src OpenAIUsage) {
	if dst == nil {
		return
	}
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.CacheCreationInputTokens += src.CacheCreationInputTokens
	dst.CacheReadInputTokens += src.CacheReadInputTokens
	dst.ImageOutputTokens += src.ImageOutputTokens
}

func appendOpenAIImagesOAuthFanOutContextEvents(dst, src *gin.Context) {
	if dst == nil || src == nil || dst == src {
		return
	}
	if rawEvents, ok := src.Get(OpsUpstreamErrorsKey); ok {
		if events, ok := rawEvents.([]*OpsUpstreamErrorEvent); ok {
			for _, ev := range events {
				if ev == nil {
					continue
				}
				appendOpsUpstreamError(dst, *ev)
			}
		}
	}
	if skip, ok := src.Get(OpsSkipPassthroughKey); ok {
		dst.Set(OpsSkipPassthroughKey, skip)
	}
}

func buildOpenAIImagesStreamErrorBody(message string) []byte {
	body := []byte(`{"type":"error","error":{"type":"upstream_error","message":""}}`)
	if strings.TrimSpace(message) == "" {
		message = "upstream request failed"
	}
	body, _ = sjson.SetBytes(body, "error.message", message)
	return body
}

func (s *OpenAIGatewayService) openAIImagesStreamFailedDetail(payload []byte) string {
	if len(payload) == 0 || s == nil || s.cfg == nil || !s.cfg.Gateway.LogUpstreamErrorBody {
		return ""
	}
	maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
	if maxBytes <= 0 {
		maxBytes = 2048
	}
	return truncateString(string(payload), maxBytes)
}

func openAIImagesOAuthDebugTimelineFields(account *Account, parsed *OpenAIImagesRequest, requestedModel string) map[string]any {
	fields := map[string]any{
		"component":       "gateway_debug_timeline",
		"platform":        PlatformOpenAI,
		"endpoint_kind":   "images",
		"requested_model": strings.TrimSpace(requestedModel),
	}
	if account != nil {
		fields["account_id"] = account.ID
		fields["account_name"] = strings.TrimSpace(account.Name)
		fields["account_type"] = strings.TrimSpace(string(account.Type))
		fields["account_platform"] = strings.TrimSpace(account.Platform)
	}
	if parsed != nil {
		fields["image_endpoint"] = strings.TrimSpace(parsed.Endpoint)
		fields["stream"] = parsed.Stream
		fields["multipart"] = parsed.Multipart
		fields["image_size"] = strings.TrimSpace(parsed.SizeTier)
		fields["response_format"] = strings.TrimSpace(parsed.ResponseFormat)
	}
	return fields
}

func (s *OpenAIGatewayService) writeOpenAIImagesStreamEvent(c *gin.Context, flusher http.Flusher, eventName string, payload []byte) error {
	if strings.TrimSpace(eventName) != "" {
		if _, err := fmt.Fprintf(c.Writer, "event: %s\n", eventName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func cloneHeaderFromResponse(resp *http.Response) http.Header {
	if resp == nil || resp.Header == nil {
		return nil
	}
	return resp.Header.Clone()
}

func (s *OpenAIGatewayService) newOpenAIImagesOAuthUpstreamFailover(
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	resp *http.Response,
	upstreamURL string,
	requestModel string,
	message string,
	kind string,
	bodyEvent string,
	body []byte,
	contentType string,
) *UpstreamFailoverError {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "upstream image response failed"
	}
	upstreamStatus := http.StatusBadGateway
	upstreamRequestID := ""
	responseHeaders := http.Header(nil)
	if resp != nil {
		if resp.StatusCode > 0 {
			upstreamStatus = resp.StatusCode
		}
		upstreamRequestID = strings.TrimSpace(resp.Header.Get("x-request-id"))
		responseHeaders = resp.Header.Clone()
	}
	if strings.TrimSpace(kind) == "" {
		kind = "failover"
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/json"
	}
	responseBody := body
	if len(responseBody) == 0 || kind == "stream_error" || kind == "empty_image_output" {
		responseBody = []byte(fmt.Sprintf(`{"error":{"message":%q,"type":"upstream_error"}}`, message))
	}

	fields := openAIImagesOAuthDebugTimelineFields(account, parsed, requestModel)
	fields["upstream_request_id"] = upstreamRequestID
	fields["upstream_status_code"] = upstreamStatus
	fields["error"] = message
	if bodyEvent != "" {
		fields["body_event"] = bodyEvent
	}
	RecordGatewayDebugTimelineBody(s.settingService, c, "upstream_response_body", body, contentType, fields)

	detail := s.openAIImagesStreamFailedDetail(body)
	if detail == "" && len(body) > 0 {
		detail = truncateString(string(body), 2048)
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:             account.Platform,
		AccountID:            account.ID,
		AccountName:          account.Name,
		UpstreamStatusCode:   upstreamStatus,
		UpstreamRequestID:    upstreamRequestID,
		UpstreamURL:          safeUpstreamURL(upstreamURL),
		UpstreamResponseBody: detail,
		Kind:                 kind,
		Message:              message,
		Detail:               detail,
	})

	return &UpstreamFailoverError{
		StatusCode:             http.StatusBadGateway,
		ResponseBody:           responseBody,
		ResponseHeaders:        responseHeaders,
		RetryableOnSameAccount: true,
	}
}

func (s *OpenAIGatewayService) collectOpenAIImagesOAuthNonStreamingResponse(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	responseFormat string,
	fallbackModel string,
	upstreamURL string,
) (*openAIImagesOAuthResponse, error) {
	body, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, s.newOpenAIImagesOAuthUpstreamFailover(c, account, parsed, resp, upstreamURL, fallbackModel, err.Error(), "stream_error", "read_error", body, resp.Header.Get("Content-Type"))
	}
	_ = s.markOpenAICyberPolicyIfDetected(ctx, account, body)

	var usage OpenAIUsage
	if parsedUsage, ok := extractOpenAIUsageFromJSONBytes(body); ok {
		usage = parsedUsage
	}
	forEachOpenAISSEDataPayload(string(body), func(dataBytes []byte) {
		s.parseSSEUsageBytes(dataBytes, &usage)
	})
	results, createdAt, usageRaw, firstMeta, _, err := collectOpenAIImagesFromResponsesBody(body)
	if err != nil {
		return nil, s.newOpenAIImagesOAuthUpstreamFailover(c, account, parsed, resp, upstreamURL, fallbackModel, err.Error(), "response_error", "response_error", body, resp.Header.Get("Content-Type"))
	}
	if len(results) == 0 {
		return nil, s.newOpenAIImagesOAuthUpstreamFailover(c, account, parsed, resp, upstreamURL, fallbackModel, "upstream did not return image output", "empty_image_output", "empty_image_output", body, resp.Header.Get("Content-Type"))
	}
	if strings.TrimSpace(firstMeta.Model) == "" {
		firstMeta.Model = strings.TrimSpace(fallbackModel)
	}

	return &openAIImagesOAuthResponse{
		Results:       results,
		CreatedAt:     createdAt,
		UsageRaw:      usageRaw,
		FirstMeta:     firstMeta,
		Usage:         usage,
		UpstreamReqID: resp.Header.Get("x-request-id"),
		StatusCode:    resp.StatusCode,
		Header:        resp.Header.Clone(),
	}, nil
}

func (s *OpenAIGatewayService) handleOpenAIImagesOAuthNonStreamingResponse(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	responseFormat string,
	fallbackModel string,
	upstreamURL string,
) (OpenAIUsage, int, error) {
	collected, err := s.collectOpenAIImagesOAuthNonStreamingResponse(ctx, resp, c, account, parsed, responseFormat, fallbackModel, upstreamURL)
	if err != nil {
		return OpenAIUsage{}, 0, err
	}
	responseBody, err := buildOpenAIImagesAPIResponse(collected.Results, collected.CreatedAt, collected.UsageRaw, collected.FirstMeta, responseFormat)
	if err != nil {
		return OpenAIUsage{}, 0, err
	}
	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	c.Data(resp.StatusCode, "application/json; charset=utf-8", responseBody)
	return collected.Usage, len(collected.Results), nil
}

func (s *OpenAIGatewayService) handleOpenAIImagesOAuthStreamingResponse(
	resp *http.Response,
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	upstreamURL string,
	startTime time.Time,
	responseFormat string,
	streamPrefix string,
	fallbackModel string,
) (OpenAIUsage, int, *int, error) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return OpenAIUsage{}, 0, nil, fmt.Errorf("streaming is not supported by response writer")
	}

	format := normalizeOpenAIImagesResponseFormat(responseFormat)

	maxLineSize := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLineSize = s.cfg.Gateway.MaxLineSize
	}
	scanner := bufio.NewScanner(resp.Body)
	initialBufferSize := min(64*1024, maxLineSize)
	if initialBufferSize < 1 {
		initialBufferSize = 1
	}
	scanner.Buffer(make([]byte, 0, initialBufferSize), maxLineSize)
	usage := OpenAIUsage{}
	imageCount := 0
	var firstTokenMs *int
	emitted := make(map[string]struct{})
	pendingResults := make([]openAIResponsesImageResult, 0, 1)
	pendingSeen := make(map[string]struct{})
	streamMeta := openAIResponsesImageResult{Model: strings.TrimSpace(fallbackModel)}
	var createdAt int64
	var (
		clientDisconnected bool
		streamCompleted    bool
		processErr         error
		responseCommitted  bool
	)
	commitResponse := func() {
		if responseCommitted {
			return
		}
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Status(resp.StatusCode)
		responseCommitted = true
	}
	tryWriteEvent := func(eventName string, payload []byte) error {
		if clientDisconnected {
			return nil
		}
		commitResponse()
		if err := s.writeOpenAIImagesStreamEvent(c, flusher, eventName, payload); err != nil {
			if isOpenAIImagesDownstreamDisconnect(err) {
				clientDisconnected = true
				logger.LegacyPrintf("service.openai_gateway", "[OpenAI] Images responses stream client disconnected, continue draining upstream for billing")
				return nil
			}
			return err
		}
		return nil
	}
	processPayload := func(dataBytes []byte) {
		if processErr != nil || streamCompleted || len(dataBytes) == 0 {
			return
		}
		_ = s.markOpenAICyberPolicyIfDetected(c.Request.Context(), account, dataBytes)
		if firstTokenMs == nil {
			ms := int(time.Since(startTime).Milliseconds())
			firstTokenMs = &ms
		}
		s.parseSSEUsageBytes(dataBytes, &usage)
		if !gjson.ValidBytes(dataBytes) {
			return
		}
		if meta, eventCreatedAt, ok := extractOpenAIResponsesImageMetaFromLifecycleEvent(dataBytes); ok {
			mergeOpenAIResponsesImageMeta(&streamMeta, meta)
			if eventCreatedAt > 0 {
				createdAt = eventCreatedAt
			}
		}
		switch gjson.GetBytes(dataBytes, "type").String() {
		case "response.image_generation_call.partial_image":
			b64 := strings.TrimSpace(gjson.GetBytes(dataBytes, "partial_image_b64").String())
			if b64 == "" {
				return
			}
			eventName := streamPrefix + ".partial_image"
			partialMeta := streamMeta
			mergeOpenAIResponsesImageMeta(&partialMeta, openAIResponsesImageResult{
				OutputFormat: strings.TrimSpace(gjson.GetBytes(dataBytes, "output_format").String()),
				Background:   strings.TrimSpace(gjson.GetBytes(dataBytes, "background").String()),
			})
			payload := buildOpenAIImagesStreamPartialPayload(
				eventName,
				b64,
				gjson.GetBytes(dataBytes, "partial_image_index").Int(),
				format,
				createdAt,
				partialMeta,
			)
			processErr = tryWriteEvent(eventName, payload)
		case "response.output_item.done":
			img, itemID, ok, extractErr := extractOpenAIImageFromResponsesOutputItemDone(dataBytes)
			if extractErr != nil {
				_ = tryWriteEvent("error", buildOpenAIImagesStreamErrorBody(extractErr.Error()))
				processErr = extractErr
				return
			}
			if !ok {
				return
			}
			mergeOpenAIResponsesImageMeta(&streamMeta, img)
			mergeOpenAIResponsesImageMeta(&img, streamMeta)
			key := openAIResponsesImageResultKey(itemID, img)
			if _, exists := emitted[key]; exists {
				return
			}
			if _, exists := pendingSeen[key]; exists {
				return
			}
			pendingSeen[key] = struct{}{}
			pendingResults = append(pendingResults, img)
		case "response.completed":
			results, _, usageRaw, firstMeta, extractErr := extractOpenAIImagesFromResponsesCompleted(dataBytes)
			if extractErr != nil {
				_ = tryWriteEvent("error", buildOpenAIImagesStreamErrorBody(extractErr.Error()))
				processErr = extractErr
				return
			}
			usageRaw = mergeOpenAIImagesUsageJSON(usageRaw, buildOpenAIImagesUsageJSON(usage, len(results)))
			mergeOpenAIResponsesImageMeta(&streamMeta, firstMeta)
			finalResults := make([]openAIResponsesImageResult, 0, len(results)+len(pendingResults))
			finalSeen := make(map[string]struct{})
			for _, img := range results {
				mergeOpenAIResponsesImageMeta(&img, streamMeta)
				appendOpenAIResponsesImageResultDedup(&finalResults, finalSeen, "", img)
			}
			for _, img := range pendingResults {
				mergeOpenAIResponsesImageMeta(&img, streamMeta)
				appendOpenAIResponsesImageResultDedup(&finalResults, finalSeen, "", img)
			}
			if len(finalResults) == 0 {
				processErr = fmt.Errorf("upstream did not return image output")
				if !responseCommitted && imageCount == 0 && len(emitted) == 0 {
					processErr = s.newOpenAIImagesOAuthUpstreamFailover(c, account, parsed, resp, upstreamURL, fallbackModel, processErr.Error(), "empty_image_output", "empty_image_output", dataBytes, "application/json")
					return
				}
				fields := openAIImagesOAuthDebugTimelineFields(account, parsed, fallbackModel)
				fields["upstream_request_id"] = strings.TrimSpace(resp.Header.Get("x-request-id"))
				fields["upstream_status_code"] = resp.StatusCode
				fields["error"] = processErr.Error()
				fields["image_count"] = 0
				fields["body_event"] = "empty_image_output"
				fields["sse_event_type"] = "response.completed"
				RecordGatewayDebugTimelineBody(s.settingService, c, "upstream_response_body", dataBytes, "application/json", fields)
				_ = tryWriteEvent("error", buildOpenAIImagesStreamErrorBody(processErr.Error()))
				return
			}
			eventName := streamPrefix + ".completed"
			for _, img := range finalResults {
				key := openAIResponsesImageResultKey("", img)
				if _, exists := emitted[key]; exists {
					continue
				}
				payload := buildOpenAIImagesStreamCompletedPayload(eventName, img, format, createdAt, usageRaw)
				if err := tryWriteEvent(eventName, payload); err != nil {
					processErr = err
					return
				}
				emitted[key] = struct{}{}
			}
			imageCount = len(emitted)
			streamCompleted = true
		case "response.failed":
			failedMessage := extractOpenAISSEErrorMessage(dataBytes)
			if failedMessage == "" {
				failedMessage = "upstream image response failed"
			}
			errCodeRaw, errTypeRaw, _ := parseOpenAIWSResponseFailedErrorFields(dataBytes)
			upstreamStatus := openAIWSErrorHTTPStatusFromRaw(errCodeRaw, errTypeRaw)
			upstreamDetail := s.openAIImagesStreamFailedDetail(dataBytes)
			fields := openAIImagesOAuthDebugTimelineFields(account, parsed, fallbackModel)
			fields["upstream_request_id"] = strings.TrimSpace(resp.Header.Get("x-request-id"))
			fields["upstream_status_code"] = upstreamStatus
			fields["error"] = failedMessage
			fields["body_event"] = "response_failed"
			fields["sse_event_type"] = "response.failed"
			RecordGatewayDebugTimelineBody(s.settingService, c, "upstream_response_body", dataBytes, "application/json", fields)
			setOpsUpstreamError(c, upstreamStatus, failedMessage, upstreamDetail)
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: upstreamStatus,
				UpstreamRequestID:  strings.TrimSpace(resp.Header.Get("x-request-id")),
				UpstreamURL:        safeUpstreamURL(upstreamURL),
				Kind:               "http_error",
				Message:            failedMessage,
				Detail:             upstreamDetail,
			})
			_ = tryWriteEvent("error", buildOpenAIImagesStreamErrorBody(failedMessage))
			processErr = fmt.Errorf("upstream image response failed: %s", failedMessage)
		}
	}
	var sseData openAISSEDataAccumulator
	for scanner.Scan() {
		sseData.AddLine(scanner.Text(), processPayload)
		if processErr != nil {
			return OpenAIUsage{}, imageCount, firstTokenMs, processErr
		}
		if streamCompleted {
			return usage, imageCount, firstTokenMs, nil
		}
	}
	sseData.Flush(processPayload)
	if processErr != nil {
		return OpenAIUsage{}, imageCount, firstTokenMs, processErr
	}
	if streamCompleted {
		return usage, imageCount, firstTokenMs, nil
	}
	if err := scanner.Err(); err != nil {
		streamErr := err
		if errors.Is(err, bufio.ErrTooLong) {
			streamErr = fmt.Errorf("upstream image stream exceeded maximum token size of %d bytes", maxLineSize)
		}
		if !responseCommitted && imageCount == 0 && len(emitted) == 0 && len(pendingResults) == 0 {
			return OpenAIUsage{}, imageCount, firstTokenMs, s.newOpenAIImagesOAuthUpstreamFailover(c, account, parsed, resp, upstreamURL, fallbackModel, streamErr.Error(), "stream_error", "stream_error", nil, resp.Header.Get("Content-Type"))
		}
		_ = tryWriteEvent("error", buildOpenAIImagesStreamErrorBody(streamErr.Error()))
		return OpenAIUsage{}, imageCount, firstTokenMs, streamErr
	}

	if imageCount > 0 {
		return usage, imageCount, firstTokenMs, nil
	}
	if len(pendingResults) > 0 {
		eventName := streamPrefix + ".completed"
		for _, img := range pendingResults {
			mergeOpenAIResponsesImageMeta(&img, streamMeta)
			key := openAIResponsesImageResultKey("", img)
			if _, exists := emitted[key]; exists {
				continue
			}
			payload := buildOpenAIImagesStreamCompletedPayload(eventName, img, format, createdAt, nil)
			if err := tryWriteEvent(eventName, payload); err != nil {
				return OpenAIUsage{}, imageCount, firstTokenMs, err
			}
			emitted[key] = struct{}{}
		}
		imageCount = len(emitted)
		return usage, imageCount, firstTokenMs, nil
	}

	streamErr := fmt.Errorf("stream disconnected before image generation completed")
	if !responseCommitted && imageCount == 0 && len(emitted) == 0 {
		return OpenAIUsage{}, imageCount, firstTokenMs, s.newOpenAIImagesOAuthUpstreamFailover(c, account, parsed, resp, upstreamURL, fallbackModel, streamErr.Error(), "stream_error", "stream_disconnected", nil, resp.Header.Get("Content-Type"))
	}
	_ = tryWriteEvent("error", buildOpenAIImagesStreamErrorBody(streamErr.Error()))
	return OpenAIUsage{}, imageCount, firstTokenMs, streamErr
}

func isOpenAIImagesDownstreamDisconnect(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "client disconnected") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer")
}

func (s *OpenAIGatewayService) doOpenAIImagesOAuthRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	requestModel string,
	mainModel string,
	imageRoute string,
) (*http.Response, string, error) {
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, "", err
	}

	responsesBody, err := buildOpenAIImagesResponsesRequest(parsed, requestModel, mainModel)
	if err != nil {
		return nil, "", err
	}
	setOpsUpstreamRequestBody(c, responsesBody)

	upstreamReq, err := s.buildUpstreamRequest(ctx, c, account, responsesBody, token, true, parsed.StickySessionSeed(), isOpenAICodexOfficialClientRequest(c))
	if err != nil {
		return nil, "", err
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Accept", "text/event-stream")
	upstreamReq = s.applyOpenAIOAuthImageBridgeUpstreamOptions(upstreamReq)
	requestFields := openAIImagesOAuthDebugTimelineFields(account, parsed, requestModel)
	requestFields["upstream_endpoint"] = safeUpstreamURL(upstreamReq.URL.String())
	requestFields["upstream_method"] = upstreamReq.Method
	requestFields["upstream_accept"] = upstreamReq.Header.Get("Accept")
	requestFields["main_model"] = mainModel
	RecordGatewayDebugTimelineBody(s.settingService, c, "upstream_request_body", responsesBody, upstreamReq.Header.Get("Content-Type"), requestFields)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	tlsRuntime := s.resolveOpenAITLSFingerprintRuntime(ctx, c, account, "http")
	applyOpenAITLSFingerprintRuntime(upstreamReq, tlsRuntime)
	upstreamStart := time.Now()
	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	upstreamURL := upstreamReq.URL.String()
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		recordDetailedUpstreamTransportError(c, err)
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			UpstreamURL:        safeUpstreamURL(upstreamURL),
			Kind:               "request_error",
			Message:            safeErr,
			Detail:             safeErr,
		})
		return nil, upstreamURL, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	if resp.StatusCode >= 400 {
		respBody := s.readUpstreamErrorBody(resp)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  resp.Header.Get("x-request-id"),
				UpstreamURL:        safeUpstreamURL(upstreamURL),
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			if s.rateLimitService != nil {
				if !s.rateLimitService.handleOpenAIImageRoute429(ctx, account, imageRoute, resp.StatusCode, resp.Header, respBody, true) {
					s.handleFailoverSideEffects(ctx, resp, account, requestModel)
				}
			} else {
				s.handleFailoverSideEffects(ctx, resp, account, requestModel)
			}
			return nil, upstreamURL, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		_, err := s.handleErrorResponse(ctx, resp, c, account, responsesBody, requestModel)
		return nil, upstreamURL, err
	}
	return resp, upstreamURL, nil
}

func (s *OpenAIGatewayService) forwardOpenAIImagesOAuthSingle(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	requestModel string,
	mainModel string,
	imageRoute string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	resp, upstreamURL, err := s.doOpenAIImagesOAuthRequest(ctx, c, account, parsed, requestModel, mainModel, imageRoute)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var (
		usage        OpenAIUsage
		imageCount   int
		firstTokenMs *int
	)
	if parsed.Stream {
		usage, imageCount, firstTokenMs, err = s.handleOpenAIImagesOAuthStreamingResponse(
			resp,
			c,
			account,
			parsed,
			upstreamURL,
			startTime,
			parsed.ResponseFormat,
			openAIImagesStreamPrefix(parsed),
			requestModel,
		)
		if err != nil {
			return nil, err
		}
	} else {
		usage, imageCount, err = s.handleOpenAIImagesOAuthNonStreamingResponse(ctx, resp, c, account, parsed, parsed.ResponseFormat, requestModel, upstreamURL)
		if err != nil {
			return nil, err
		}
	}
	if imageCount <= 0 {
		imageCount = openAIImagesResponsesEffectiveN(parsed)
	}
	return &OpenAIForwardResult{
		RequestID:       resp.Header.Get("x-request-id"),
		Usage:           usage,
		Model:           requestModel,
		UpstreamModel:   requestModel,
		Stream:          parsed.Stream,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
		FirstTokenMs:    firstTokenMs,
		ImageCount:      imageCount,
		ImageSize:       parsed.SizeTier,
	}, nil
}

func (s *OpenAIGatewayService) forwardOpenAIImagesOAuthFanOut(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	requestModel string,
	mainModel string,
	imageRoute string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	n := openAIImagesResponsesEffectiveN(parsed)
	attempts := make([]openAIImagesOAuthFanOutAttempt, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			attempt := *parsed
			attempt.N = 1
			attemptCtx := c
			if c != nil {
				attemptCtx = c.Copy()
			}
			resp, upstreamURL, err := s.doOpenAIImagesOAuthRequest(ctx, attemptCtx, account, &attempt, requestModel, mainModel, imageRoute)
			if err != nil {
				attempts[i] = openAIImagesOAuthFanOutAttempt{Err: err, Context: attemptCtx}
				return
			}
			collected, err := s.collectOpenAIImagesOAuthNonStreamingResponse(ctx, resp, attemptCtx, account, &attempt, attempt.ResponseFormat, requestModel, upstreamURL)
			_ = resp.Body.Close()
			attempts[i] = openAIImagesOAuthFanOutAttempt{
				Response: collected,
				Err:      err,
				Context:  attemptCtx,
			}
		}()
	}
	wg.Wait()

	aggregatedResults := make([]openAIResponsesImageResult, 0, n)
	var aggregatedUsage OpenAIUsage
	var firstCreatedAt int64
	var firstMeta openAIResponsesImageResult
	var firstHeader http.Header
	var firstRequestID string
	var partialErr error

	for _, attempt := range attempts {
		appendOpenAIImagesOAuthFanOutContextEvents(c, attempt.Context)
		if attempt.Err != nil {
			if partialErr == nil {
				partialErr = attempt.Err
			}
			continue
		}
		collected := attempt.Response
		if collected == nil {
			err := fmt.Errorf("upstream image response is missing")
			if partialErr == nil {
				partialErr = err
			}
			continue
		}
		if len(aggregatedResults) == 0 {
			firstCreatedAt = collected.CreatedAt
			firstMeta = collected.FirstMeta
			firstHeader = collected.Header.Clone()
			firstRequestID = strings.TrimSpace(collected.UpstreamReqID)
		}
		addOpenAIUsage(&aggregatedUsage, collected.Usage)
		aggregatedResults = append(aggregatedResults, collected.Results...)
	}
	if len(aggregatedResults) == 0 {
		if partialErr != nil {
			return nil, partialErr
		}
		return nil, fmt.Errorf("upstream did not return image output")
	}

	responseBody, err := buildOpenAIImagesAPIResponse(
		aggregatedResults,
		firstCreatedAt,
		buildOpenAIImagesUsageJSON(aggregatedUsage, len(aggregatedResults)),
		firstMeta,
		parsed.ResponseFormat,
	)
	if err != nil {
		return nil, err
	}
	if partialErr != nil {
		responseBody, err = sjson.SetBytes(responseBody, "partial_error", partialErr.Error())
		if err != nil {
			return nil, fmt.Errorf("set partial image fan-out error: %w", err)
		}
		responseBody, err = sjson.SetBytes(responseBody, "partial_count", len(aggregatedResults))
		if err != nil {
			return nil, fmt.Errorf("set partial image fan-out count: %w", err)
		}
		responseBody, err = sjson.SetBytes(responseBody, "requested_count", n)
		if err != nil {
			return nil, fmt.Errorf("set partial image fan-out requested count: %w", err)
		}
	}
	responseheaders.WriteFilteredHeaders(c.Writer.Header(), firstHeader, s.responseHeaderFilter)
	c.Data(http.StatusOK, "application/json; charset=utf-8", responseBody)
	return &OpenAIForwardResult{
		RequestID:       firstRequestID,
		Usage:           aggregatedUsage,
		Model:           requestModel,
		UpstreamModel:   requestModel,
		Stream:          false,
		ResponseHeaders: firstHeader.Clone(),
		Duration:        time.Since(startTime),
		ImageCount:      len(aggregatedResults),
		ImageSize:       parsed.SizeTier,
	}, nil
}

func (s *OpenAIGatewayService) forwardOpenAIImagesOAuth(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	channelMappedModel string,
	imageRoute string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	requestModel := strings.TrimSpace(parsed.Model)
	if mapped := strings.TrimSpace(channelMappedModel); mapped != "" {
		requestModel = mapped
	}
	if requestModel == "" {
		requestModel = "gpt-image-2"
	}
	if err := validateOpenAIImagesModel(requestModel); err != nil {
		return nil, err
	}
	mainModel := resolveOpenAIResponsesImageMainModel(c.Request.Context())
	logger.LegacyPrintf(
		"service.openai_gateway",
		"[OpenAI] Images request routing request_model=%s endpoint=%s account_type=%s uploads=%d main_model=%s",
		requestModel,
		parsed.Endpoint,
		account.Type,
		len(parsed.Uploads),
		mainModel,
	)
	if !parsed.Stream && openAIImagesResponsesEffectiveN(parsed) > 1 {
		return s.forwardOpenAIImagesOAuthFanOut(ctx, c, account, parsed, requestModel, mainModel, imageRoute, startTime)
	}
	return s.forwardOpenAIImagesOAuthSingle(ctx, c, account, parsed, requestModel, mainModel, imageRoute, startTime)
}
