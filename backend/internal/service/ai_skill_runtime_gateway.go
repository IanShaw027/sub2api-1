package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/integration/skillrunner"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	defaultAISkillChatModel  = "gpt-5.4-mini"
	defaultAISkillImageModel = "gpt-image-1"
)

var (
	_ AISkillRuntimeGateway     = (*DefaultAISkillRuntimeGateway)(nil)
	_ AISkillOpenAIChatRuntime  = (*AISkillOpenAIRuntime)(nil)
	_ AISkillOpenAIImageRuntime = (*AISkillOpenAIRuntime)(nil)
	_ AISkillScriptRuntime      = (*AISkillScriptRunnerRuntime)(nil)
)

type DefaultAISkillRuntimeGateway struct {
	chatRuntime   AISkillOpenAIChatRuntime
	imageRuntime  AISkillOpenAIImageRuntime
	scriptRuntime AISkillScriptRuntime
}

func NewAISkillRuntimeGateway(
	chatRuntime AISkillOpenAIChatRuntime,
	imageRuntime AISkillOpenAIImageRuntime,
	scriptRuntime AISkillScriptRuntime,
) *DefaultAISkillRuntimeGateway {
	return &DefaultAISkillRuntimeGateway{
		chatRuntime:   chatRuntime,
		imageRuntime:  imageRuntime,
		scriptRuntime: scriptRuntime,
	}
}

func (g *DefaultAISkillRuntimeGateway) Execute(ctx context.Context, req AISkillExecutionRequest) (*AISkillDispatchResult, error) {
	switch normalizeAISkillType(req.Type) {
	case AISkillTypePromptChat:
		return g.executePromptChat(ctx, req)
	case AISkillTypePromptImage:
		return g.executePromptImage(ctx, req)
	case AISkillTypeScript:
		return g.executeScript(ctx, req)
	default:
		return nil, ErrAISkillExecutionSpecInvalid
	}
}

func (g *DefaultAISkillRuntimeGateway) executePromptChat(ctx context.Context, req AISkillExecutionRequest) (*AISkillDispatchResult, error) {
	if g == nil || g.chatRuntime == nil || req.PromptChat == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	model := firstNonEmptyString(strings.TrimSpace(req.PromptChat.Model), defaultAISkillChatModel)
	body, err := buildAISkillChatRequestBody(req.PromptChat, model)
	if err != nil {
		return nil, err
	}

	result, err := g.chatRuntime.ExecuteChatCompat(ctx, AISkillOpenAIChatRuntimeInput{
		GroupID:        resolveAISkillGroupID(req),
		SessionHash:    buildAISkillSessionHash(req, model),
		PromptCacheKey: buildAISkillPromptCacheKey(req, model),
		Model:          model,
		Body:           body,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	output := buildAISkillChatOutput(result)
	return &AISkillDispatchResult{
		Status:        AISkillRunStatusSucceeded,
		Provider:      PlatformOpenAI,
		ExternalJobID: resolveAISkillExternalJobID(result.Forward, result.ResponseBody),
		Output:        output,
		Metadata:      buildAISkillOpenAIMetadata(result.Forward),
	}, nil
}

func (g *DefaultAISkillRuntimeGateway) executePromptImage(ctx context.Context, req AISkillExecutionRequest) (*AISkillDispatchResult, error) {
	if g == nil || g.imageRuntime == nil || req.PromptImage == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	model := firstNonEmptyString(strings.TrimSpace(req.PromptImage.Model), defaultAISkillImageModel)
	path, body, err := buildAISkillImageRequestBody(req.PromptImage, model)
	if err != nil {
		return nil, err
	}

	result, err := g.imageRuntime.ExecuteImages(ctx, AISkillOpenAIImageRuntimeInput{
		GroupID:     resolveAISkillGroupID(req),
		SessionHash: buildAISkillSessionHash(req, model),
		Model:       model,
		Path:        path,
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	output := buildAISkillImageOutput(result)
	return &AISkillDispatchResult{
		Status:        AISkillRunStatusSucceeded,
		Provider:      PlatformOpenAI,
		ExternalJobID: resolveAISkillExternalJobID(result.Forward, result.ResponseBody),
		Output:        output,
		Metadata:      buildAISkillOpenAIMetadata(result.Forward),
	}, nil
}

func (g *DefaultAISkillRuntimeGateway) executeScript(ctx context.Context, req AISkillExecutionRequest) (*AISkillDispatchResult, error) {
	if g == nil || g.scriptRuntime == nil || req.Script == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	result, err := g.scriptRuntime.ExecuteScript(ctx, AISkillScriptRuntimeInput{
		RunID:          req.RunID,
		SkillID:        req.SkillID,
		VersionID:      req.VersionID,
		UserID:         req.UserID,
		Mode:           req.Mode,
		Runtime:        req.Script.Runtime,
		ScriptName:     req.Script.ScriptName,
		EntryPoint:     req.Script.EntryPoint,
		Protocol:       req.Script.Protocol,
		ArchivePath:    req.Script.ArchivePath,
		ArchiveBase64:  req.Script.ArchiveBase64,
		TimeoutSeconds: req.Script.TimeoutSeconds,
		Environment:    cloneAISkillStringMap(req.Script.Environment),
		Arguments:      cloneAIMapSlice(req.Script.Arguments),
		Parameters:     cloneAIMap(req.Script.Parameters),
		Trace:          req.Trace,
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	return &AISkillDispatchResult{
		Status:        normalizeAISkillDispatchStatus(result.Status),
		Provider:      "skillrunner",
		ExternalJobID: strings.TrimSpace(result.ExternalJobID),
		Output:        cloneAIMap(result.Output),
		Metadata:      cloneAIMap(result.Metadata),
	}, nil
}

type AISkillOpenAIRuntime struct {
	gateway *OpenAIGatewayService
}

func NewAISkillOpenAIRuntime(gateway *OpenAIGatewayService) *AISkillOpenAIRuntime {
	return &AISkillOpenAIRuntime{gateway: gateway}
}

func (r *AISkillOpenAIRuntime) ExecuteChatCompat(ctx context.Context, input AISkillOpenAIChatRuntimeInput) (*AISkillOpenAIChatRuntimeResult, error) {
	if r == nil || r.gateway == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	model := firstNonEmptyString(strings.TrimSpace(input.Model), defaultAISkillChatModel)
	account, err := r.gateway.SelectAccountForModel(ctx, input.GroupID, strings.TrimSpace(input.SessionHash), model)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	requestID := buildAISkillContextRequestID("chat", input.SessionHash, model)
	ginCtx, recorder, err := newAISkillOpenAIGinContext(ctx, http.MethodPost, "/v1/chat/completions", "application/json", input.Body, input.SessionHash, requestID)
	if err != nil {
		return nil, err
	}

	forward, err := r.gateway.ForwardAsChatCompletions(
		ginCtx.Request.Context(),
		ginCtx,
		account,
		input.Body,
		strings.TrimSpace(input.PromptCacheKey),
		model,
		"",
	)
	if err != nil {
		return nil, err
	}

	return &AISkillOpenAIChatRuntimeResult{
		Forward:      forward,
		ResponseBody: append([]byte(nil), recorder.Body.Bytes()...),
	}, nil
}

func (r *AISkillOpenAIRuntime) ExecuteImages(ctx context.Context, input AISkillOpenAIImageRuntimeInput) (*AISkillOpenAIImageRuntimeResult, error) {
	if r == nil || r.gateway == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	model := firstNonEmptyString(strings.TrimSpace(input.Model), defaultAISkillImageModel)
	account, err := r.gateway.SelectAccountForModel(ctx, input.GroupID, strings.TrimSpace(input.SessionHash), model)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	requestPath := normalizeAISkillImageEndpoint(input.Path)
	requestID := buildAISkillContextRequestID("image", input.SessionHash, model)
	ginCtx, recorder, err := newAISkillOpenAIGinContext(
		ctx,
		http.MethodPost,
		requestPath,
		firstNonEmptyString(strings.TrimSpace(input.ContentType), "application/json"),
		input.Body,
		input.SessionHash,
		requestID,
	)
	if err != nil {
		return nil, err
	}

	parsed, err := r.gateway.ParseOpenAIImagesRequest(ginCtx, input.Body)
	if err != nil {
		return nil, err
	}

	forward, err := r.gateway.ForwardImages(
		ginCtx.Request.Context(),
		ginCtx,
		account,
		input.Body,
		parsed,
		model,
	)
	if err != nil {
		return nil, err
	}

	return &AISkillOpenAIImageRuntimeResult{
		Forward:      forward,
		ResponseBody: append([]byte(nil), recorder.Body.Bytes()...),
	}, nil
}

type AISkillScriptRunnerRuntime struct {
	runner skillrunner.Runner
	now    func() time.Time
}

func NewAISkillScriptRunnerRuntime(runner skillrunner.Runner) *AISkillScriptRunnerRuntime {
	if runner == nil {
		runner = skillrunner.NewScriptRunner()
	}
	return &AISkillScriptRunnerRuntime{
		runner: runner,
		now:    time.Now,
	}
}

func (r *AISkillScriptRunnerRuntime) ExecuteScript(ctx context.Context, input AISkillScriptRuntimeInput) (*AISkillScriptRuntimeResult, error) {
	if r == nil || r.runner == nil {
		return nil, ErrAISkillServiceUnavailable
	}

	archive, source, err := resolveAISkillScriptArchive(input)
	if err != nil {
		return nil, err
	}

	bundle, err := r.runner.InspectArchive(ctx, archive)
	if err != nil {
		return nil, err
	}
	if err := validateAISkillScriptBundle(input, bundle); err != nil {
		return nil, err
	}

	approvedAt := r.nowOrDefault()
	environment := renderAISkillStringMap(input.Environment, input.Parameters)
	dispatchInput := map[string]any{
		"run_id":         input.RunID,
		"skill_id":       input.SkillID,
		"version_id":     input.VersionID,
		"user_id":        input.UserID,
		"mode":           normalizeAISkillRunMode(input.Mode),
		"script_name":    strings.TrimSpace(input.ScriptName),
		"runtime":        strings.TrimSpace(input.Runtime),
		"entry_point":    strings.TrimSpace(input.EntryPoint),
		"protocol":       strings.TrimSpace(input.Protocol),
		"parameters":     cloneAIMap(input.Parameters),
		"arguments":      cloneAIMapSlice(input.Arguments),
		"environment":    stringMapToAnyMap(environment),
		"trace":          input.Trace,
		"archive_source": source,
	}

	dispatch, err := r.runner.Dispatch(ctx, skillrunner.DispatchRequest{
		Bundle: bundle,
		Review: skillrunner.ReviewGate{
			ArtifactDigest: bundle.Digest,
			Status:         skillrunner.ReviewStatusApproved,
			ApprovedAt:     &approvedAt,
			ApprovedBy:     "ai-skill-service",
		},
		Archive:     archive,
		Input:       dispatchInput,
		Environment: stringMapToAnyMap(environment),
	})
	if err != nil {
		return nil, err
	}

	if dispatch != nil && dispatch.Plan != nil {
		if input.TimeoutSeconds > 0 {
			dispatch.Plan.ResourceLimits.Timeout = time.Duration(input.TimeoutSeconds) * time.Second
		}
		if dispatch.Plan.Environment == nil {
			dispatch.Plan.Environment = map[string]string{}
		}
		for key, value := range environment {
			dispatch.Plan.Environment[key] = value
		}
	}

	planValue := any(nil)
	if dispatch != nil {
		planValue = marshalAISkillValue(dispatch.Plan)
	}
	output := map[string]any{
		"bundle":         marshalAISkillValue(bundle),
		"plan":           planValue,
		"paths":          map[string]any{},
		"environment":    stringMapToAnyMap(environment),
		"dispatch_input": dispatchInput,
	}
	if dispatch != nil {
		output["paths"] = map[string]any{
			"host_skill_dir":   strings.TrimSpace(dispatch.HostSkillDir),
			"host_scratch_dir": strings.TrimSpace(dispatch.HostScratchDir),
			"input_path":       strings.TrimSpace(dispatch.InputPath),
			"output_path":      strings.TrimSpace(dispatch.OutputPath),
		}
	}

	externalJobID := strings.TrimSpace(bundle.Digest)
	if externalJobID == "" {
		externalJobID = fmt.Sprintf("skillrunner:%d", input.RunID)
	}

	return &AISkillScriptRuntimeResult{
		Status:        AISkillRunStatusDispatched,
		ExternalJobID: externalJobID,
		Output:        output,
		Metadata: map[string]any{
			"provider":       "skillrunner",
			"bundle_digest":  strings.TrimSpace(bundle.Digest),
			"archive_source": source,
		},
	}, nil
}

func (r *AISkillScriptRunnerRuntime) nowOrDefault() time.Time {
	if r != nil && r.now != nil {
		return r.now()
	}
	return time.Now()
}

func buildAISkillChatRequestBody(exec *AISkillPromptChatExecution, model string) ([]byte, error) {
	if exec == nil {
		return nil, ErrAISkillExecutionSpecInvalid
	}

	renderedPrompt := renderAISkillTemplate(exec.UserPromptTemplate, exec.Parameters)
	messages := make([]map[string]any, 0, 2)
	if systemPrompt := renderAISkillTemplate(exec.SystemPrompt, exec.Parameters); strings.TrimSpace(systemPrompt) != "" {
		messages = append(messages, map[string]any{
			"role":    AIMessageRoleSystem,
			"content": systemPrompt,
		})
	}
	messages = append(messages, map[string]any{
		"role":    AIMessageRoleUser,
		"content": buildAISkillChatUserContent(renderedPrompt, exec.Attachments),
	})

	body := map[string]any{
		"model":    firstNonEmptyString(strings.TrimSpace(model), defaultAISkillChatModel),
		"messages": messages,
		"stream":   false,
	}
	if len(exec.ResponseFormat) > 0 {
		body["response_format"] = cloneAIMap(exec.ResponseFormat)
	}
	applyAISkillChatRuntimeParameters(body, exec.Parameters)

	return json.Marshal(body)
}

func buildAISkillChatUserContent(prompt string, attachments []AISkillRunAttachment) any {
	imageParts := make([]map[string]any, 0, len(attachments)+1)
	if strings.TrimSpace(prompt) != "" {
		imageParts = append(imageParts, map[string]any{
			"type": "text",
			"text": prompt,
		})
	}
	for _, attachment := range attachments {
		if url := strings.TrimSpace(attachment.URL); url != "" {
			imageParts = append(imageParts, map[string]any{
				"type": "image_url",
				"image_url": map[string]any{
					"url": url,
				},
			})
		}
	}
	if len(imageParts) == 0 {
		return prompt
	}
	if len(imageParts) == 1 && imageParts[0]["type"] == "text" {
		return prompt
	}
	return imageParts
}

func applyAISkillChatRuntimeParameters(body map[string]any, params map[string]any) {
	if body == nil {
		return
	}
	if params == nil {
		params = map[string]any{}
	}
	if temperature, ok := intOrFloatAISkillParameter(params, "temperature"); ok {
		body["temperature"] = temperature
	}
	if topP, ok := intOrFloatAISkillParameter(params, "top_p"); ok {
		body["top_p"] = topP
	}
	if maxTokens, ok := intAISkillParameter(params, "max_tokens"); ok {
		body["max_tokens"] = maxTokens
	}
	if maxCompletionTokens, ok := intAISkillParameter(params, "max_completion_tokens"); ok {
		body["max_completion_tokens"] = maxCompletionTokens
	}
	if reasoningEffort, ok := stringAISkillParameter(params, "reasoning_effort"); ok {
		body["reasoning_effort"] = reasoningEffort
	}
	if serviceTier, ok := stringAISkillParameter(params, "service_tier"); ok {
		body["service_tier"] = serviceTier
	}
	if responseFormat, ok := mapAISkillParameter(params, "response_format"); ok && len(responseFormat) > 0 {
		body["response_format"] = responseFormat
	}
	for _, key := range []string{"stop", "tools", "tool_choice", "functions", "function_call"} {
		if value, ok := params[key]; ok && value != nil {
			body[key] = value
		}
	}
}

func buildAISkillImageRequestBody(exec *AISkillPromptImageExecution, model string) (string, []byte, error) {
	if exec == nil {
		return "", nil, ErrAISkillExecutionSpecInvalid
	}

	prompt := renderAISkillTemplate(exec.PromptTemplate, exec.Parameters)
	if negativePrompt := renderAISkillTemplate(exec.NegativePromptTemplate, exec.Parameters); strings.TrimSpace(negativePrompt) != "" {
		prompt = strings.TrimSpace(prompt + "\nNegative prompt: " + negativePrompt)
	}
	path := openAIImagesGenerationsEndpoint
	body := map[string]any{
		"model":           firstNonEmptyString(strings.TrimSpace(model), defaultAISkillImageModel),
		"prompt":          prompt,
		"n":               maxAISkillImageCount(exec.ImageCount),
		"response_format": "b64_json",
		"stream":          false,
	}
	if size := strings.TrimSpace(exec.Size); size != "" {
		body["size"] = size
	}
	applyAISkillImageRuntimeParameters(body, exec.Parameters)

	imageURLs := make([]map[string]any, 0, len(exec.Attachments))
	maskURL := ""
	for _, attachment := range exec.Attachments {
		url := strings.TrimSpace(attachment.URL)
		if url == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(attachment.Purpose), "mask") && maskURL == "" {
			maskURL = url
			continue
		}
		imageURLs = append(imageURLs, map[string]any{"image_url": url})
	}
	if len(imageURLs) > 0 {
		path = openAIImagesEditsEndpoint
		body["images"] = imageURLs
		if maskURL != "" {
			body["mask"] = map[string]any{"image_url": maskURL}
		}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", nil, err
	}
	return path, raw, nil
}

func applyAISkillImageRuntimeParameters(body map[string]any, params map[string]any) {
	if body == nil || params == nil {
		return
	}
	if n, ok := intAISkillParameter(params, "n"); ok && n > 0 {
		body["n"] = n
	}
	if imageCount, ok := intAISkillParameter(params, "image_count"); ok && imageCount > 0 {
		body["n"] = imageCount
	}
	for _, key := range []string{
		"size",
		"quality",
		"background",
		"output_format",
		"moderation",
		"input_fidelity",
		"style",
		"response_format",
	} {
		if value, ok := stringAISkillParameter(params, key); ok {
			body[key] = value
		}
	}
	for _, key := range []string{"output_compression", "partial_images"} {
		if value, ok := intAISkillParameter(params, key); ok {
			body[key] = value
		}
	}
}

func buildAISkillChatOutput(result *AISkillOpenAIChatRuntimeResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	output := buildAISkillOpenAIOutputBase(result.Forward, result.ResponseBody)
	if text := extractAISkillChatText(result.ResponseBody); strings.TrimSpace(text) != "" {
		output["text"] = text
	}
	return output
}

func buildAISkillImageOutput(result *AISkillOpenAIImageRuntimeResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	output := buildAISkillOpenAIOutputBase(result.Forward, result.ResponseBody)
	if images := extractAISkillImageItems(result.ResponseBody); len(images) > 0 {
		output["images"] = images
	}
	if revisedPrompt := strings.TrimSpace(gjson.GetBytes(result.ResponseBody, "data.0.revised_prompt").String()); revisedPrompt != "" {
		output["revised_prompt"] = revisedPrompt
	}
	return output
}

func buildAISkillOpenAIOutputBase(forward *OpenAIForwardResult, raw []byte) map[string]any {
	output := map[string]any{}
	if decoded, ok := decodeAISkillJSONMap(raw); ok {
		output["response"] = decoded
	} else if len(raw) > 0 {
		output["raw"] = string(raw)
	}
	if forward != nil {
		if requestID := strings.TrimSpace(forward.RequestID); requestID != "" {
			output["request_id"] = requestID
		}
		if model := strings.TrimSpace(firstNonEmptyString(forward.UpstreamModel, forward.Model)); model != "" {
			output["model"] = model
		}
		output["usage"] = map[string]any{
			"input_tokens":                forward.Usage.InputTokens,
			"output_tokens":               forward.Usage.OutputTokens,
			"cache_creation_input_tokens": forward.Usage.CacheCreationInputTokens,
			"cache_read_input_tokens":     forward.Usage.CacheReadInputTokens,
			"image_output_tokens":         forward.Usage.ImageOutputTokens,
		}
		if forward.ImageCount > 0 {
			output["image_count"] = forward.ImageCount
		}
		if size := strings.TrimSpace(forward.ImageSize); size != "" {
			output["image_size"] = size
		}
	}
	return output
}

func buildAISkillOpenAIMetadata(forward *OpenAIForwardResult) map[string]any {
	if forward == nil {
		return map[string]any{}
	}
	metadata := map[string]any{
		"request_id":          strings.TrimSpace(forward.RequestID),
		"model":               strings.TrimSpace(forward.Model),
		"upstream_model":      strings.TrimSpace(forward.UpstreamModel),
		"billing_model":       strings.TrimSpace(forward.BillingModel),
		"token_billing_model": strings.TrimSpace(forward.TokenBillingModel),
		"stream":              forward.Stream,
		"openai_ws_mode":      forward.OpenAIWSMode,
		"image_count":         forward.ImageCount,
		"image_size":          strings.TrimSpace(forward.ImageSize),
	}
	if forward.ServiceTier != nil && strings.TrimSpace(*forward.ServiceTier) != "" {
		metadata["service_tier"] = strings.TrimSpace(*forward.ServiceTier)
	}
	if forward.ReasoningEffort != nil && strings.TrimSpace(*forward.ReasoningEffort) != "" {
		metadata["reasoning_effort"] = strings.TrimSpace(*forward.ReasoningEffort)
	}
	return metadata
}

func resolveAISkillExternalJobID(forward *OpenAIForwardResult, raw []byte) string {
	if responseID := strings.TrimSpace(gjson.GetBytes(raw, "id").String()); responseID != "" {
		return responseID
	}
	if requestID := strings.TrimSpace(gjson.GetBytes(raw, "response.id").String()); requestID != "" {
		return requestID
	}
	if forward != nil {
		return strings.TrimSpace(forward.RequestID)
	}
	return ""
}

func extractAISkillChatText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	content := gjson.GetBytes(raw, "choices.0.message.content")
	switch content.Type {
	case gjson.String:
		return content.String()
	case gjson.JSON:
		var parts []string
		for _, item := range content.Array() {
			switch item.Get("type").String() {
			case "text", "output_text":
				if text := strings.TrimSpace(item.Get("text").String()); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func extractAISkillImageItems(raw []byte) []map[string]any {
	if len(raw) == 0 {
		return []map[string]any{}
	}
	data := gjson.GetBytes(raw, "data")
	if !data.Exists() || !data.IsArray() {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(data.Array()))
	for _, item := range data.Array() {
		decoded, ok := decodeAISkillJSONMap([]byte(item.Raw))
		if !ok {
			continue
		}
		out = append(out, decoded)
	}
	return out
}

func newAISkillOpenAIGinContext(
	ctx context.Context,
	method string,
	path string,
	contentType string,
	body []byte,
	sessionHash string,
	requestID string,
) (*gin.Context, *httptest.ResponseRecorder, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = withAISkillRuntimeRequestID(ctx, requestID)

	recorder := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequestWithContext(ctx, method, "http://skill-runtime.local"+path, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", firstNonEmptyString(strings.TrimSpace(contentType), "application/json"))
	req.Header.Set("User-Agent", codexCLIUserAgent)
	req.Header.Set("originator", resolveOpenAIUpstreamOriginator(nil, true))
	if strings.TrimSpace(sessionHash) != "" {
		req.Header.Set("session_id", strings.TrimSpace(sessionHash))
	}
	ginCtx.Request = req
	return ginCtx, recorder, nil
}

func withAISkillRuntimeRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if existing, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(existing) != "" {
		return ctx
	}
	if requestID = strings.TrimSpace(requestID); requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.RequestID, requestID)
}

func buildAISkillSessionHash(req AISkillExecutionRequest, model string) string {
	seed := buildAISkillRuntimeSeed(req, model)
	sum := sha256.Sum256([]byte(seed))
	return "ai-skill-" + hex.EncodeToString(sum[:8])
}

func buildAISkillPromptCacheKey(req AISkillExecutionRequest, model string) string {
	seed := buildAISkillRuntimeSeed(req, model)
	sum := sha256.Sum256([]byte(seed))
	return "ai-skill:" + hex.EncodeToString(sum[:])
}

func buildAISkillContextRequestID(kind string, sessionHash string, model string) string {
	seed := strings.Join([]string{
		"ai-skill-runtime",
		strings.TrimSpace(kind),
		strings.TrimSpace(sessionHash),
		strings.TrimSpace(model),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "ai-skill-" + hex.EncodeToString(sum[:8])
}

func buildAISkillRuntimeSeed(req AISkillExecutionRequest, model string) string {
	parts := []string{
		"ai-skill",
		strconv.FormatInt(req.RunID, 10),
		strconv.FormatInt(req.SkillID, 10),
		strconv.FormatInt(req.VersionID, 10),
		strconv.FormatInt(req.UserID, 10),
		strings.TrimSpace(req.Mode),
		strings.TrimSpace(req.Type),
		strings.TrimSpace(model),
	}
	return strings.Join(parts, "|")
}

func resolveAISkillGroupID(req AISkillExecutionRequest) *int64 {
	for _, candidate := range []*int64{
		req.Trace.GroupID,
		traceGroupID(req.Settlement),
		traceGroupID(req.Version),
		traceGroupID(req.Skill),
	} {
		if candidate != nil && *candidate > 0 {
			return candidate
		}
	}
	return nil
}

func traceGroupID(entity any) *int64 {
	switch value := entity.(type) {
	case *AISkill:
		if value != nil {
			return value.Trace.GroupID
		}
	case *AISkillVersion:
		if value != nil {
			return value.Trace.GroupID
		}
	case *AISkillSettlement:
		if value != nil {
			return value.Trace.GroupID
		}
	}
	return nil
}

func renderAISkillTemplate(template string, params map[string]any) string {
	template = strings.TrimSpace(template)
	if template == "" {
		return ""
	}
	if len(params) == 0 {
		return template
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rendered := template
	for _, key := range keys {
		value, ok := lookupAISkillTemplateValue(params, key)
		if !ok {
			continue
		}
		replacement := stringifyAISkillTemplateValue(value)
		for _, token := range []string{
			"{{" + key + "}}",
			"{{ " + key + " }}",
			"{{ " + key + "}}",
			"{{" + key + " }}",
		} {
			rendered = strings.ReplaceAll(rendered, token, replacement)
		}
	}
	return rendered
}

func lookupAISkillTemplateValue(params map[string]any, key string) (any, bool) {
	if params == nil {
		return nil, false
	}
	if value, ok := params[key]; ok {
		return value, true
	}
	current := any(params)
	for _, part := range strings.Split(key, ".") {
		if strings.TrimSpace(part) == "" {
			return nil, false
		}
		node, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = node[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func stringifyAISkillTemplateValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		raw, err := json.Marshal(typed)
		if err == nil && string(raw) != "null" && string(raw) != `""` {
			if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
				unquoted, unquoteErr := strconv.Unquote(string(raw))
				if unquoteErr == nil {
					return unquoted
				}
			}
			return string(raw)
		}
		return fmt.Sprint(typed)
	}
}

func renderAISkillStringMap(values map[string]string, params map[string]any) map[string]string {
	rendered := cloneAISkillStringMap(values)
	for key, value := range rendered {
		rendered[key] = renderAISkillTemplate(value, params)
	}
	return rendered
}

func intAISkillParameter(params map[string]any, key string) (int, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func intOrFloatAISkillParameter(params map[string]any, key string) (float64, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func stringAISkillParameter(params map[string]any, key string) (string, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		typed = strings.TrimSpace(typed)
		return typed, typed != ""
	case fmt.Stringer:
		value := strings.TrimSpace(typed.String())
		return value, value != ""
	default:
		value := strings.TrimSpace(fmt.Sprint(typed))
		return value, value != ""
	}
}

func mapAISkillParameter(params map[string]any, key string) (map[string]any, bool) {
	value, ok := params[key]
	if !ok || value == nil {
		return nil, false
	}
	typed, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return cloneAIMap(typed), true
}

func maxAISkillImageCount(v int) int {
	if v <= 0 {
		return 1
	}
	return v
}

func normalizeAISkillImageEndpoint(path string) string {
	switch strings.TrimSpace(path) {
	case openAIImagesEditsEndpoint:
		return openAIImagesEditsEndpoint
	default:
		return openAIImagesGenerationsEndpoint
	}
}

func decodeAISkillJSONMap(raw []byte) (map[string]any, bool) {
	if len(raw) == 0 || !gjson.ValidBytes(raw) {
		return nil, false
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, false
	}
	return decoded, true
}

func marshalAISkillValue(value any) any {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	return decoded
}

func buildAISkillScriptArchiveBase64(
	scriptName string,
	versionName string,
	displayName string,
	description string,
	runtime string,
	entrypoint string,
	protocol string,
	sourceCode string,
) (string, error) {
	scriptName = strings.TrimSpace(scriptName)
	versionName = strings.TrimSpace(versionName)
	runtime = normalizeAISkillScriptRuntime(runtime)
	entrypoint = normalizeAISkillScriptEntryPoint(entrypoint, runtime)
	protocol = firstNonEmptyString(strings.TrimSpace(protocol), skillrunner.ProtocolJSONFileV1)
	if scriptName == "" || versionName == "" || runtime == "" || entrypoint == "" || strings.TrimSpace(sourceCode) == "" {
		return "", ErrAISkillExecutionSpecInvalid
	}

	manifest := skillrunner.Manifest{
		APIVersion: skillrunner.ManifestAPIVersion,
		Kind:       skillrunner.ManifestKind,
		Metadata: skillrunner.ManifestMeta{
			Name:        scriptName,
			Version:     versionName,
			DisplayName: truncateAISkillScriptText(displayName, 80),
			Description: truncateAISkillScriptText(description, 512),
		},
		Spec: skillrunner.ScriptSpec{
			Type:       skillrunner.SkillTypeScript,
			Runtime:    runtime,
			Entrypoint: entrypoint,
			Protocol:   protocol,
		},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	manifestFile, err := writer.Create(skillrunner.ManifestFile)
	if err != nil {
		return "", err
	}
	if _, err := manifestFile.Write(manifestRaw); err != nil {
		return "", err
	}
	sourceFile, err := writer.Create(entrypoint)
	if err != nil {
		return "", err
	}
	if _, err := sourceFile.Write([]byte(sourceCode)); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(archive.Bytes()), nil
}

func extractAISkillScriptSourceCode(meta map[string]any) string {
	if meta == nil {
		return ""
	}
	if raw, ok := meta["content"]; ok {
		if content, ok := raw.(map[string]any); ok {
			if source := firstNonEmptyAISkillScriptValue(content, "source_code", "code", "script"); source != "" {
				return source
			}
		}
	}
	return firstNonEmptyAISkillScriptValue(meta, "source_code", "code", "script")
}

func normalizeAISkillScriptRuntime(raw string) string {
	switch normalized := strings.ToLower(strings.TrimSpace(raw)); {
	case normalized == "":
		return skillrunner.RuntimeNode20
	case strings.HasPrefix(normalized, "node"), normalized == "javascript", normalized == "js":
		return skillrunner.RuntimeNode20
	case strings.HasPrefix(normalized, "python"):
		return skillrunner.RuntimePython311
	default:
		return strings.TrimSpace(raw)
	}
}

func normalizeAISkillScriptEntryPoint(raw string, runtime string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, `\`, "/"))
	if trimmed == "" {
		return defaultAISkillScriptEntryPoint(runtime)
	}
	cleaned := path.Clean(strings.TrimSuffix(trimmed, "/"))
	if cleaned == "." || cleaned == "/" || strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	if path.Ext(cleaned) == "" {
		cleaned += defaultAISkillScriptEntryPointSuffix(runtime)
	}
	return cleaned
}

func defaultAISkillScriptEntryPoint(runtime string) string {
	return "main" + defaultAISkillScriptEntryPointSuffix(runtime)
}

func defaultAISkillScriptEntryPointSuffix(runtime string) string {
	if normalizeAISkillScriptRuntime(runtime) == skillrunner.RuntimePython311 {
		return ".py"
	}
	return ".mjs"
}

func normalizeAISkillScriptBundleName(raw string, skill *AISkill, version *AISkillVersion) string {
	if name := sanitizeAISkillScriptBundleName(raw); name != "" {
		return name
	}
	if skill != nil {
		if slug := sanitizeAISkillScriptBundleName(firstNonEmptyStringValue(skill.Metadata, "slug")); slug != "" {
			return slug
		}
		if skill.ID > 0 && version != nil && version.ID > 0 {
			return fmt.Sprintf("skill-%d-%d", skill.ID, version.ID)
		}
		if skill.ID > 0 {
			return fmt.Sprintf("skill-%d", skill.ID)
		}
	}
	if version != nil && version.ID > 0 {
		return fmt.Sprintf("skill-version-%d", version.ID)
	}
	return "script-skill"
}

func sanitizeAISkillScriptBundleName(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	lastSeparator := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastSeparator = false
		case r == '-' || r == '_':
			if builder.Len() == 0 || lastSeparator {
				continue
			}
			builder.WriteRune(r)
			lastSeparator = true
		default:
			if builder.Len() == 0 || lastSeparator {
				continue
			}
			builder.WriteByte('-')
			lastSeparator = true
		}
	}
	out := strings.Trim(builder.String(), "-_")
	if len(out) < 3 {
		return ""
	}
	if len(out) > 64 {
		out = strings.Trim(out[:64], "-_")
		if len(out) < 3 {
			return ""
		}
	}
	return out
}

func normalizeAISkillScriptVersionName(version *AISkillVersion) string {
	if version == nil {
		return "v1.0.0"
	}
	if name := firstNonEmptyStringValue(version.Metadata, "version_name", "version"); name != "" {
		normalized := strings.TrimSpace(name)
		if isAISkillScriptVersionLike(normalized) {
			return normalized
		}
	}
	if version.Version > 0 {
		return fmt.Sprintf("v%d.0.0", version.Version)
	}
	return "v1.0.0"
}

func isAISkillScriptVersionLike(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	trimmed = strings.TrimPrefix(strings.TrimPrefix(trimmed, "v"), "V")
	if strings.Count(trimmed, ".") < 2 {
		return false
	}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '-', r == '+':
		default:
			return false
		}
	}
	return true
}

func truncateAISkillScriptText(raw string, maxBytes int) string {
	text := strings.TrimSpace(raw)
	if maxBytes <= 0 || text == "" {
		return ""
	}
	if len(text) <= maxBytes {
		return text
	}
	var builder strings.Builder
	size := 0
	for _, r := range text {
		runeSize := len(string(r))
		if size+runeSize > maxBytes {
			break
		}
		builder.WriteRune(r)
		size += runeSize
	}
	return builder.String()
}

func firstNonEmptyAISkillScriptValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func resolveAISkillScriptArchive(input AISkillScriptRuntimeInput) ([]byte, string, error) {
	if archivePath := strings.TrimSpace(input.ArchivePath); archivePath != "" {
		raw, err := os.ReadFile(archivePath)
		if err != nil {
			return nil, "", err
		}
		return raw, archivePath, nil
	}
	if archiveBase64 := strings.TrimSpace(input.ArchiveBase64); archiveBase64 != "" {
		raw, err := decodeAISkillArchiveBase64(archiveBase64)
		if err != nil {
			return nil, "", err
		}
		return raw, "inline_base64", nil
	}
	return nil, "", ErrAISkillExecutionSpecInvalid
}

func decodeAISkillArchiveBase64(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "data:application/zip;base64,")
	trimmed = strings.TrimPrefix(trimmed, "data:application/octet-stream;base64,")
	trimmed = strings.ReplaceAll(trimmed, "\n", "")
	trimmed = strings.ReplaceAll(trimmed, "\r", "")
	trimmed = strings.ReplaceAll(trimmed, "\t", "")
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	if trimmed == "" {
		return nil, ErrAISkillExecutionSpecInvalid
	}
	trimmed = strings.TrimRight(trimmed, "=") + strings.Repeat("=", (4-len(trimmed)%4)%4)
	return base64.StdEncoding.DecodeString(trimmed)
}

func validateAISkillScriptBundle(input AISkillScriptRuntimeInput, bundle *skillrunner.Bundle) error {
	if bundle == nil {
		return ErrAISkillExecutionSpecInvalid
	}
	if scriptName := strings.TrimSpace(input.ScriptName); scriptName != "" && !strings.EqualFold(strings.TrimSpace(bundle.Manifest.Metadata.Name), scriptName) {
		return fmt.Errorf("skill archive name mismatch: want %s got %s", scriptName, bundle.Manifest.Metadata.Name)
	}
	if runtime := strings.TrimSpace(input.Runtime); runtime != "" && !strings.EqualFold(strings.TrimSpace(bundle.Manifest.Spec.Runtime), runtime) {
		return fmt.Errorf("skill archive runtime mismatch: want %s got %s", runtime, bundle.Manifest.Spec.Runtime)
	}
	if entrypoint := strings.TrimSpace(input.EntryPoint); entrypoint != "" && strings.TrimSpace(bundle.Manifest.Spec.Entrypoint) != entrypoint {
		return fmt.Errorf("skill archive entrypoint mismatch: want %s got %s", entrypoint, bundle.Manifest.Spec.Entrypoint)
	}
	if protocol := strings.TrimSpace(input.Protocol); protocol != "" && !strings.EqualFold(strings.TrimSpace(bundle.Manifest.Spec.Protocol), protocol) {
		return fmt.Errorf("skill archive protocol mismatch: want %s got %s", protocol, bundle.Manifest.Spec.Protocol)
	}
	return nil
}
