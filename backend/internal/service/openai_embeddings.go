package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/zap"
)

func (s *OpenAIGatewayService) ForwardEmbeddings(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
	if account == nil {
		return nil, errors.New("account is required")
	}
	body = s.normalizeOpenAICompatibleFingerprintJSONBody(ctx, c, account, body, "embeddings")
	startTime := time.Now()

	originalModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if originalModel == "" {
		writeOpenAIEmbeddingsError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, fmt.Errorf("missing model in request")
	}

	billingModel := resolveOpenAIForwardModelWithSettings(ctx, s.settingService, account, originalModel, defaultMappedModel)
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	upstreamBody := body
	if upstreamModel != originalModel {
		upstreamBody = ReplaceModelInBody(body, upstreamModel)
	}

	logger.L().Debug("openai embeddings: forwarding",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("billing_model", billingModel),
		zap.String("upstream_model", upstreamModel),
	)

	apiKey := account.GetOpenAIApiKey()
	if apiKey == "" {
		return nil, fmt.Errorf("account %d missing api_key", account.ID)
	}
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}
	targetURL := buildOpenAIEmbeddingsURL(validatedURL)

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(upstreamBody))
	if err != nil {
		releaseUpstreamCtx()
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	defer releaseUpstreamCtx()
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	upstreamReq.Header.Set("Accept", "application/json")
	for key, values := range c.Request.Header {
		lowerKey := strings.ToLower(key)
		if openaiCCRawAllowedHeaders[lowerKey] {
			for _, v := range values {
				upstreamReq.Header.Add(key, v)
			}
		}
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		upstreamReq.Header.Set("user-agent", customUA)
	}

	// 账号级请求头覆写（仅 openai api_key 账号启用时生效）
	account.ApplyHeaderOverrides(upstreamReq.Header)

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	tlsRuntime := s.resolveOpenAICompatibleTLSFingerprintRuntime(ctx, c, account, "http")
	applyOpenAITLSFingerprintRuntime(upstreamReq, tlsRuntime)
	upstreamReq = withOpenAIHTTP1RawHeaderReplay(upstreamReq, account, tlsRuntime.Profile)
	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: 0,
			Kind:               "request_error",
			Message:            safeErr,
		})
		writeOpenAIEmbeddingsError(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody := s.readUpstreamErrorBody(resp)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(respBody))

		if hit, code, cyberMsg := detectOpenAICyberPolicy(respBody); hit {
			MarkOpsCyberPolicy(c, CyberPolicyMark{
				Code:           code,
				Message:        cyberMsg,
				Body:           truncateString(string(respBody), 4096),
				UpstreamStatus: resp.StatusCode,
			})
			setOpsUpstreamError(c, resp.StatusCode, cyberMsg, truncateString(string(respBody), 2048))
			writeOpenAIEmbeddingsUpstreamResponse(c, resp, respBody, s.responseHeaderFilter)
			if cyberMsg == "" {
				return nil, fmt.Errorf("openai cyber_policy: %d", resp.StatusCode)
			}
			return nil, fmt.Errorf("openai cyber_policy: %s", cyberMsg)
		}

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		upstreamDetail := ""
		if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
			maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
			if maxBytes <= 0 {
				maxBytes = 2048
			}
			upstreamDetail = truncateString(string(respBody), maxBytes)
		}
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  resp.Header.Get("x-request-id"),
				Kind:               "failover",
				Message:            upstreamMsg,
				Detail:             upstreamDetail,
			})
			s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, upstreamModel)
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		writeOpenAIEmbeddingsUpstreamResponse(c, resp, respBody, s.responseHeaderFilter)
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			writeOpenAIEmbeddingsError(c, http.StatusBadGateway, "api_error", "Failed to read upstream response")
		}
		return nil, fmt.Errorf("read upstream body: %w", err)
	}
	_ = markOpsCyberPolicyIfDetected(c, respBody, resp.StatusCode, 0, 0)

	writeOpenAIEmbeddingsUpstreamResponse(c, resp, respBody, s.responseHeaderFilter)

	return &OpenAIForwardResult{
		RequestID:     firstNonEmptyString(resp.Header.Get("x-request-id"), resp.Header.Get("request-id")),
		Usage:         extractOpenAIEmbeddingsUsage(respBody),
		Model:         originalModel,
		BillingModel:  billingModel,
		UpstreamModel: upstreamModel,
		Stream:        false,
		Duration:      time.Since(startTime),
	}, nil
}

func writeOpenAIEmbeddingsUpstreamResponse(c *gin.Context, resp *http.Response, body []byte, filter *responseheaders.CompiledHeaderFilter) {
	if c == nil || resp == nil {
		return
	}
	if c.Writer.Written() {
		return
	}
	if resp.Header != nil {
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, filter)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Writer.Header().Set("Content-Type", ct)
	} else {
		c.Writer.Header().Set("Content-Type", "application/json")
	}
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(body)
}

func writeOpenAIEmbeddingsError(c *gin.Context, statusCode int, errType, message string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

func extractOpenAIEmbeddingsUsage(body []byte) OpenAIUsage {
	usage := gjson.GetBytes(body, "usage")
	if !usage.Exists() || !usage.IsObject() {
		return OpenAIUsage{}
	}
	inputTokens := firstPositiveGJSONInt(
		usage.Get("prompt_tokens"),
		usage.Get("input_tokens"),
		usage.Get("total_tokens"),
	)
	outputTokens := firstPositiveGJSONInt(
		usage.Get("completion_tokens"),
		usage.Get("output_tokens"),
	)
	cacheReadTokens := firstPositiveGJSONInt(
		usage.Get("prompt_tokens_details.cached_tokens"),
		usage.Get("input_tokens_details.cached_tokens"),
		usage.Get("cache_read_tokens"),
		usage.Get("cache_read_input_tokens"),
	)
	cacheCreationTokens := firstPositiveGJSONInt(
		usage.Get("cache_creation_tokens"),
		usage.Get("cache_creation_input_tokens"),
		usage.Get("input_tokens_details.cache_creation_tokens"),
	)
	// 多模态 embedding（如 doubao-embedding-vision）回传图文 token 拆分，
	// 用于图文不同价计费；纯文本 embedding 该字段为 0，行为不变。
	imageInputTokens := firstPositiveGJSONInt(
		usage.Get("prompt_tokens_details.image_tokens"),
		usage.Get("input_tokens_details.image_tokens"),
	)
	return OpenAIUsage{
		InputTokens:              inputTokens,
		ImageInputTokens:         imageInputTokens,
		OutputTokens:             outputTokens,
		CacheReadInputTokens:     cacheReadTokens,
		CacheCreationInputTokens: cacheCreationTokens,
	}
}

func firstPositiveGJSONInt(values ...gjson.Result) int {
	for _, value := range values {
		if !value.Exists() {
			continue
		}
		n := int(value.Int())
		if n > 0 {
			return n
		}
	}
	return 0
}

func buildOpenAIEmbeddingsURL(base string) string {
	return buildOpenAIEndpointURL(base, "/v1/embeddings")
}

// ForwardVideos forwards to Grok/xAI-compatible /v1/videos (create/retrieve/content).
// Videos are a Grok-only surface; OpenAI and other OpenAI-compatible accounts
// must be rejected before any upstream request is built.
// Uses the Grok account token (API key or OAuth/GetAccessToken) + xAI base URL.
// Billing for video (resolution tier x seconds via group video_price_*_per_sec) should be applied by caller or usage recorder.
func (s *OpenAIGatewayService) ForwardVideos(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	targetPath string, // e.g. "/v1/videos" or "/v1/videos/xxx/content"
) (*OpenAIForwardResult, error) {
	if account == nil || account.Platform != PlatformGrok {
		return nil, fmt.Errorf("videos endpoint is only supported for Grok accounts")
	}
	body = s.normalizeOpenAICompatibleFingerprintJSONBody(ctx, c, account, body, "videos")
	startTime := time.Now()

	originalModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if originalModel == "" {
		originalModel = GetGrokMediaVideoBoundModel(c)
	}
	clientVideoRequestBody := append([]byte(nil), body...)
	if targetPath == "" {
		targetPath = "/v1/videos"
	}
	targetPath = canonicalOpenAIVideoTargetPath(targetPath)
	videoBillingRequest := isOpenAIVideoBillingRequest(c.Request.Method, targetPath)
	upstreamContentType := strings.TrimSpace(c.GetHeader("Content-Type"))
	if isOpenAIVideoContentPath(targetPath) {
		if variant := openAIVideoContentVariant(targetPath); variant != "" && variant != "video" {
			errVariant := fmt.Errorf("Invalid request: variant %q is not available for xAI video downloads", variant)
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": errVariant.Error(), "type": "invalid_request_error"}})
			return nil, errVariant
		}
	}

	// POST /v1/videos accepts OpenAI's video-create shape; xAI upstream expects
	// native Imagine fields. Normalize before account mapping so live ForwardVideos
	// matches the ForwardGrokMedia helper and CPA's xAI request builder.
	openAIVideoCreate := isOpenAIVideoCreateRequest(c.Request.Method, targetPath)
	if openAIVideoCreate && !gjson.ValidBytes(body) {
		if formBody, formErr := openAIVideoCreateJSONFromEncodedForm(body, upstreamContentType); formErr != nil {
			writeOpenAIVideoFailedResponse(c, http.StatusBadRequest, originalModel, "invalid_request_error", formErr.Error())
			return nil, formErr
		} else if len(formBody) > 0 {
			body = formBody
			clientVideoRequestBody = append([]byte(nil), formBody...)
			upstreamContentType = "application/json"
			originalModel = strings.TrimSpace(gjson.GetBytes(body, "model").String())
		}
	}
	if openAIVideoCreate && gjson.ValidBytes(body) {
		if !isSupportedGrokOpenAIVideoModel(originalModel) {
			errUnsupported := fmt.Errorf("Model %s is not supported on /v1/videos. Use sora-2.", originalModel)
			writeOpenAIVideoFailedResponse(c, http.StatusBadRequest, originalModel, "invalid_request_error", errUnsupported.Error())
			return nil, errUnsupported
		}
		if originalModel == "" {
			originalModel = xai.DefaultImagineVideoModel
			body = ReplaceModelInBody(body, originalModel)
			clientVideoRequestBody = ReplaceModelInBody(clientVideoRequestBody, originalModel)
		}
		var prepErr error
		body, prepErr = normalizeGrokOpenAIVideoCreateJSONRequest(body)
		if prepErr != nil {
			writeOpenAIVideoFailedResponse(c, http.StatusBadRequest, originalModel, "invalid_request_error", prepErr.Error())
			return nil, prepErr
		}
	} else if videoBillingRequest && gjson.ValidBytes(body) {
		if !isSupportedGrokNativeVideoModel(originalModel) {
			errUnsupported := fmt.Errorf("Model %s is not supported on %s, /v1/videos/edits, or /v1/videos/extensions. Use %s.", originalModel, strings.TrimRight(targetPath, "/"), xai.DefaultImagineVideoModel)
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": errUnsupported.Error(), "type": "invalid_request_error"}})
			return nil, errUnsupported
		}
		if originalModel == "" {
			originalModel = xai.DefaultImagineVideoModel
		}
	}

	// POST create/edit/extend: apply account mapping + Imagine alias normalization
	// so live traffic matches ForwardGrokMedia. Keep originalModel for billing identity.
	upstreamModel := originalModel
	if videoBillingRequest && originalModel != "" && gjson.ValidBytes(body) {
		upstreamModel = resolveGrokMediaUpstreamModel(account, GrokMediaEndpointVideosGenerations, originalModel)
		if upstreamModel != "" && upstreamModel != originalModel {
			body = ReplaceModelInBody(body, upstreamModel)
		}
	}

	logger.L().Debug("grok videos: forwarding",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("upstream_model", upstreamModel),
		zap.String("target_path", targetPath),
	)

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("get access token for video: %w", err)
	}
	baseURL := account.GetGrokBaseURL()
	if token == "" {
		return nil, fmt.Errorf("account %d missing token for video generation", account.ID)
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}
	if isOpenAIVideoContentPath(targetPath) {
		return s.forwardGrokVideoContentViaRetrieve(ctx, c, account, token, validatedURL, targetPath, originalModel, upstreamModel, startTime)
	}
	targetURL := buildOpenAIEndpointURL(validatedURL, targetPath)

	upstreamCtx, release := detachUpstreamContext(ctx)
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, c.Request.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		release()
		return nil, fmt.Errorf("build upstream video request: %w", err)
	}
	defer release()
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	if len(body) > 0 {
		if upstreamContentType != "" {
			upstreamReq.Header.Set("Content-Type", upstreamContentType)
		} else {
			upstreamReq.Header.Set("Content-Type", "application/json")
		}
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	if accept := strings.TrimSpace(c.GetHeader("Accept")); accept != "" {
		upstreamReq.Header.Set("Accept", accept)
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}
	// pass select headers
	for key, values := range c.Request.Header {
		lowerKey := strings.ToLower(key)
		if openaiCCRawAllowedHeaders[lowerKey] {
			for _, v := range values {
				upstreamReq.Header.Add(key, v)
			}
		}
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		upstreamReq.Header.Set("user-agent", customUA)
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	tlsRuntime := s.resolveOpenAICompatibleTLSFingerprintRuntime(ctx, c, account, "http")
	applyOpenAITLSFingerprintRuntime(upstreamReq, tlsRuntime)
	upstreamReq = withOpenAIHTTP1RawHeaderReplay(upstreamReq, account, tlsRuntime.Profile)

	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		return nil, &UpstreamFailoverError{StatusCode: 0, ResponseBody: nil, RetryableOnSameAccount: true}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 400 && !videoBillingRequest {
		pathJobID := ExtractGrokVideoRequestIDFromPath(targetPath)
		// Binary content streams early; sticky identity comes from the path job id.
		if isOpenAIVideoContentPath(targetPath) {
			responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
			if ct := strings.TrimSpace(resp.Header.Get("Content-Type")); ct != "" {
				c.Header("Content-Type", ct)
			}
			c.Status(resp.StatusCode)
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
			buf := make([]byte, 32*1024)
			for {
				n, readErr := resp.Body.Read(buf)
				if n > 0 {
					if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
						return nil, fmt.Errorf("stream upstream video response: %w", writeErr)
					}
					if flusher, ok := c.Writer.(http.Flusher); ok {
						flusher.Flush()
					}
				}
				if errors.Is(readErr, io.EOF) {
					break
				}
				if readErr != nil {
					return nil, fmt.Errorf("stream upstream video response: %w", readErr)
				}
			}
			videoModel := firstNonEmptyString(strings.TrimSpace(resp.Header.Get("openai-model")), originalModel)
			return &OpenAIForwardResult{
				RequestID:       firstNonEmptyString(resp.Header.Get("x-request-id"), resp.Header.Get("request-id"), resp.Header.Get("xai-request-id")),
				ResponseID:      pathJobID,
				Model:           videoModel,
				UpstreamModel:   firstNonEmptyString(upstreamModel, videoModel),
				Usage:           OpenAIUsage{},
				ResponseHeaders: resp.Header.Clone(),
				Duration:        time.Since(startTime),
			}, nil
		}

		// JSON status/retrieve: buffer so ResponseID can be extracted from body,
		// and normalize xAI's native retrieve payload to the OpenAI video object
		// shape that CPA exposes.
		respBody, readErr := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
		if readErr != nil {
			return nil, fmt.Errorf("read upstream video status response: %w", readErr)
		}
		if normalized, normalizeErr := buildOpenAIVideoRetrieveResponseFromGrok(pathJobID, respBody, originalModel); normalizeErr == nil {
			respBody = normalized
		}
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		if ct := strings.TrimSpace(resp.Header.Get("Content-Type")); ct != "" {
			c.Header("Content-Type", ct)
		}
		c.Status(resp.StatusCode)
		_, _ = c.Writer.Write(respBody)
		videoModel := firstNonEmptyString(
			strings.TrimSpace(gjson.GetBytes(respBody, "model").String()),
			strings.TrimSpace(resp.Header.Get("openai-model")),
			originalModel,
		)
		responseID := firstNonEmptyString(extractGrokMediaVideoRequestID(respBody), pathJobID)
		return &OpenAIForwardResult{
			RequestID:       firstNonEmptyString(resp.Header.Get("x-request-id"), resp.Header.Get("request-id"), resp.Header.Get("xai-request-id")),
			ResponseID:      responseID,
			Model:           videoModel,
			UpstreamModel:   firstNonEmptyString(upstreamModel, videoModel),
			Usage:           OpenAIUsage{},
			ResponseHeaders: resp.Header.Clone(),
			Duration:        time.Since(startTime),
		}, nil
	}

	if resp.StatusCode >= 400 {
		respBody, readErr := s.readUpstreamErrorBodyWithError(resp)
		if readErr != nil {
			return nil, fmt.Errorf("read upstream video response: %w", readErr)
		}
		if hit, code, cyberMsg := detectOpenAICyberPolicy(respBody); hit {
			MarkOpsCyberPolicy(c, CyberPolicyMark{
				Code:           code,
				Message:        cyberMsg,
				Body:           truncateString(string(respBody), 4096),
				UpstreamStatus: resp.StatusCode,
			})
			setOpsUpstreamError(c, resp.StatusCode, cyberMsg, truncateString(string(respBody), 2048))
			responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
			c.Header("Content-Type", "application/json")
			c.Status(resp.StatusCode)
			_, _ = c.Writer.Write(respBody)
			if cyberMsg == "" {
				return nil, fmt.Errorf("grok video cyber_policy: %d", resp.StatusCode)
			}
			return nil, fmt.Errorf("grok video cyber_policy: %s", cyberMsg)
		}

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		upstreamDetail := ""
		if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
			maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
			if maxBytes <= 0 {
				maxBytes = 2048
			}
			upstreamDetail = truncateString(string(respBody), maxBytes)
		}
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  resp.Header.Get("x-request-id"),
				UpstreamURL:        safeUpstreamURL(upstreamReq.URL.String()),
				Kind:               "failover",
				Message:            upstreamMsg,
				Detail:             upstreamDetail,
			})
			// Videos are Grok-only: route through the Grok upstream-error handler so
			// 429/401 honor the parsed x-ratelimit-reset-* window and the unified
			// pipeline (Retry-After was previously a no-op on this path).
			if account.Platform == PlatformGrok {
				s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
			} else {
				s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, originalModel)
			}
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				ResponseHeaders:        resp.Header.Clone(),
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}

		setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  resp.Header.Get("x-request-id"),
			UpstreamURL:        safeUpstreamURL(upstreamReq.URL.String()),
			Kind:               "upstream_error",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
		c.Header("Content-Type", "application/json")
		c.Status(resp.StatusCode)
		_, _ = c.Writer.Write(respBody)
		return nil, fmt.Errorf("upstream video error: status %d", resp.StatusCode)
	}

	respBody, readErr := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if readErr != nil {
		return nil, fmt.Errorf("read upstream video response: %w", readErr)
	}
	clientRespBody := respBody
	if isOpenAIVideoCreateRequest(c.Request.Method, targetPath) {
		if normalized, normalizeErr := buildOpenAIVideoCreateResponseFromGrok(respBody, clientVideoRequestBody, body, originalModel); normalizeErr == nil {
			clientRespBody = normalized
		}
	}

	// write success
	responseheaders.WriteFilteredHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	if ct := strings.TrimSpace(resp.Header.Get("Content-Type")); ct != "" {
		c.Header("Content-Type", ct)
	}
	c.Status(resp.StatusCode)
	_, _ = c.Writer.Write(clientRespBody)

	// Prefer original client model for billing/logging; upstream may echo rewritten id.
	videoModel := firstNonEmptyString(originalModel, strings.TrimSpace(gjson.GetBytes(clientRespBody, "model").String()))
	if isOpenAIVideoCreateRequest(c.Request.Method, targetPath) && isGrokOpenAISoraVideoModel(originalModel) {
		videoModel = firstNonEmptyString(strings.TrimSpace(gjson.GetBytes(clientRespBody, "model").String()), upstreamModel, responseGrokVideoModel(originalModel))
	}
	resultUpstreamModel := firstNonEmptyString(upstreamModel, strings.TrimSpace(gjson.GetBytes(respBody, "model").String()), videoModel)
	var videoSeconds int
	var videoSize string
	var videoCount int
	if videoBillingRequest {
		// Prefer explicit request duration; fall back to response, then shared default
		// so pending creates without duration do not under-bill as 1 second.
		videoSeconds = resolveGrokMediaVideoSeconds(
			gjsonPositiveInt(body, "duration"),
			gjsonPositiveInt(body, "duration_seconds"),
			gjsonPositiveInt(body, "seconds"),
			gjsonPositiveInt(respBody, "video.duration"),
			gjsonPositiveInt(respBody, "seconds"),
			gjsonPositiveInt(respBody, "duration"),
			gjsonPositiveInt(respBody, "duration_seconds"),
		)
		videoSize = NormalizeVideoBillingTierOrDefault(firstNonEmptyString(
			strings.TrimSpace(gjson.GetBytes(body, "resolution").String()),
			strings.TrimSpace(gjson.GetBytes(body, "size").String()),
			strings.TrimSpace(gjson.GetBytes(respBody, "video.resolution").String()),
			strings.TrimSpace(gjson.GetBytes(respBody, "size").String()),
			strings.TrimSpace(gjson.GetBytes(respBody, "resolution").String()),
		))
		// 按响应实际交付的视频数量计费优先。若响应明确返回 data 数组但为空，
		// 视为当前尚未交付任何视频，必须记 0；仅当响应完全不给出任何交付数量线索时，
		// 才回退请求声明的 n_variants/n，避免 200 + data:[] 这类占位态按请求数量超额计费。
		if data := gjson.GetBytes(respBody, "data"); data.Exists() && data.IsArray() {
			videoCount = len(data.Array())
		} else {
			videoCount = firstPositiveInt(
				gjsonPositiveInt(body, "n_variants"),
				gjsonPositiveInt(body, "n"),
				1,
			)
		}
	}
	// Capture async job id so the handler can pin sticky scheduling for GET polls.
	responseID := extractGrokMediaVideoRequestID(clientRespBody)
	if responseID == "" {
		responseID = extractGrokMediaVideoRequestID(respBody)
	}
	if responseID == "" {
		responseID = ExtractGrokVideoRequestIDFromPath(targetPath)
	}
	res := &OpenAIForwardResult{
		RequestID:       firstNonEmptyString(resp.Header.Get("x-request-id"), resp.Header.Get("request-id"), resp.Header.Get("xai-request-id")),
		ResponseID:      responseID,
		Model:           videoModel,
		BillingModel:    videoModel,
		UpstreamModel:   resultUpstreamModel,
		Usage:           OpenAIUsage{},
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
		VideoSeconds:    videoSeconds,
		VideoSize:       videoSize,
		VideoCount:      videoCount,
	}
	return res, nil
}

func (s *OpenAIGatewayService) forwardGrokVideoContentViaRetrieve(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	validatedBaseURL string,
	targetPath string,
	originalModel string,
	upstreamModel string,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	videoID := ExtractGrokVideoRequestIDFromPath(targetPath)
	if strings.TrimSpace(videoID) == "" {
		return nil, fmt.Errorf("video_id is required for video content")
	}
	statusPath := openAIVideoContentRetrieveTargetPath(targetPath)
	statusURL := buildOpenAIEndpointURL(validatedBaseURL, statusPath)
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	tlsRuntime := s.resolveOpenAICompatibleTLSFingerprintRuntime(ctx, c, account, "http")

	statusCtx, releaseStatus := detachUpstreamContext(ctx)
	statusReq, err := http.NewRequestWithContext(statusCtx, http.MethodGet, statusURL, nil)
	if err != nil {
		releaseStatus()
		return nil, fmt.Errorf("build upstream video status request: %w", err)
	}
	statusReq = statusReq.WithContext(WithHTTPUpstreamProfile(statusReq.Context(), HTTPUpstreamProfileOpenAI))
	statusReq.Header.Set("Authorization", "Bearer "+token)
	statusReq.Header.Set("Accept", "application/json")
	applyOpenAITLSFingerprintRuntime(statusReq, tlsRuntime)
	statusReq = withOpenAIHTTP1RawHeaderReplay(statusReq, account, tlsRuntime.Profile)
	statusResp, err := s.httpUpstream.DoWithTLS(statusReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
	releaseStatus()
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		return nil, &UpstreamFailoverError{StatusCode: 0, ResponseBody: nil, RetryableOnSameAccount: true}
	}
	defer func() { _ = statusResp.Body.Close() }()
	if statusResp.StatusCode >= 400 {
		respBody, readErr := s.readUpstreamErrorBodyWithError(statusResp)
		if readErr != nil {
			return nil, fmt.Errorf("read upstream video status response: %w", readErr)
		}
		responseheaders.WriteFilteredHeaders(c.Writer.Header(), statusResp.Header, s.responseHeaderFilter)
		c.Header("Content-Type", "application/json")
		c.Status(statusResp.StatusCode)
		_, _ = c.Writer.Write(respBody)
		return nil, fmt.Errorf("upstream video status error: status %d", statusResp.StatusCode)
	}
	statusBody, readErr := ReadUpstreamResponseBody(statusResp.Body, s.cfg, c, openAITooLargeError)
	if readErr != nil {
		return nil, fmt.Errorf("read upstream video status response: %w", readErr)
	}
	contentURL, err := grokVideoContentURLFromPayload(statusBody)
	if err != nil {
		return nil, err
	}

	contentCtx, releaseContent := detachUpstreamContext(ctx)
	contentReq, err := http.NewRequestWithContext(contentCtx, http.MethodGet, contentURL, nil)
	if err != nil {
		releaseContent()
		return nil, fmt.Errorf("build upstream video content request: %w", err)
	}
	contentReq = contentReq.WithContext(WithHTTPUpstreamProfile(contentReq.Context(), HTTPUpstreamProfileOpenAI))
	if accept := strings.TrimSpace(c.GetHeader("Accept")); accept != "" {
		contentReq.Header.Set("Accept", accept)
	}
	applyOpenAITLSFingerprintRuntime(contentReq, tlsRuntime)
	contentResp, err := s.httpUpstream.DoWithTLS(contentReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
	releaseContent()
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		return nil, fmt.Errorf("download upstream video content: %s", safeErr)
	}
	defer func() { _ = contentResp.Body.Close() }()
	if contentResp.StatusCode < 200 || contentResp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(contentResp.Body)
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = contentResp.Status
		}
		setOpsUpstreamError(c, contentResp.StatusCode, sanitizeUpstreamErrorMessage(msg), "")
		c.Status(contentResp.StatusCode)
		_, _ = c.Writer.Write(respBody)
		return nil, fmt.Errorf("video content download failed: %s", msg)
	}

	responseheaders.WriteFilteredHeaders(c.Writer.Header(), contentResp.Header, s.responseHeaderFilter)
	copyGrokVideoContentHeaders(c.Writer.Header(), contentResp.Header)
	if ct := strings.TrimSpace(contentResp.Header.Get("Content-Type")); ct != "" {
		c.Header("Content-Type", ct)
	}
	c.Status(contentResp.StatusCode)
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
	buf := make([]byte, 32*1024)
	for {
		n, readErr := contentResp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
				return nil, fmt.Errorf("stream upstream video content: %w", writeErr)
			}
			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("stream upstream video content: %w", readErr)
		}
	}

	videoModel := firstNonEmptyString(strings.TrimSpace(gjson.GetBytes(statusBody, "model").String()), originalModel, responseGrokVideoModel(upstreamModel))
	return &OpenAIForwardResult{
		RequestID:       firstNonEmptyString(statusResp.Header.Get("x-request-id"), statusResp.Header.Get("request-id"), statusResp.Header.Get("xai-request-id")),
		ResponseID:      videoID,
		Model:           videoModel,
		UpstreamModel:   firstNonEmptyString(upstreamModel, videoModel),
		Usage:           OpenAIUsage{},
		ResponseHeaders: contentResp.Header.Clone(),
		Duration:        time.Since(startTime),
	}, nil
}

func openAIVideoContentVariant(targetPath string) string {
	trimmed := strings.TrimSpace(targetPath)
	idx := strings.IndexAny(trimmed, "?#")
	if idx < 0 || idx+1 >= len(trimmed) {
		return ""
	}
	rawQuery := trimmed[idx+1:]
	if strings.HasPrefix(rawQuery, "?") || strings.HasPrefix(rawQuery, "#") {
		rawQuery = rawQuery[1:]
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(values.Get("variant"))
}

func copyGrokVideoContentHeaders(dst http.Header, src http.Header) {
	if dst == nil || src == nil {
		return
	}
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Disposition", "Cache-Control", "ETag", "Last-Modified"} {
		if value := src.Get(key); value != "" {
			dst.Set(key, value)
		}
	}
}

func openAIVideoContentRetrieveTargetPath(targetPath string) string {
	canonical := canonicalOpenAIVideoTargetPath(targetPath)
	if idx := strings.IndexAny(canonical, "?#"); idx >= 0 {
		canonical = canonical[:idx]
	}
	canonical = strings.TrimRight(canonical, "/")
	if strings.HasSuffix(strings.ToLower(canonical), "/content") {
		canonical = canonical[:len(canonical)-len("/content")]
	}
	if canonical == "" {
		return "/v1/videos"
	}
	return canonical
}

func grokVideoContentURLFromPayload(payload []byte) (string, error) {
	rawURL := strings.TrimSpace(gjson.GetBytes(payload, "video.url").String())
	if rawURL == "" {
		return "", fmt.Errorf("xAI video response did not include video.url")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("xAI video response included invalid video.url")
	}
	return rawURL, nil
}

func openAIVideoCreateJSONFromEncodedForm(body []byte, contentType string) ([]byte, error) {
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return nil, nil
	}
	values := make(url.Values)
	switch strings.ToLower(mediaType) {
	case "application/x-www-form-urlencoded":
		parsed, parseErr := url.ParseQuery(string(body))
		if parseErr != nil {
			return nil, parseErr
		}
		values = parsed
	case "multipart/form-data":
		boundary := strings.TrimSpace(params["boundary"])
		if boundary == "" {
			return nil, fmt.Errorf("multipart boundary is required")
		}
		reader := multipart.NewReader(strings.NewReader(string(body)), boundary)
		form, readErr := reader.ReadForm(32 << 20)
		if readErr != nil {
			return nil, readErr
		}
		if form != nil {
			defer func() { _ = form.RemoveAll() }()
			values = form.Value
		}
	default:
		return nil, nil
	}
	if len(values) == 0 {
		return nil, nil
	}
	rawJSON := []byte(`{}`)
	var errSet error
	setString := func(path string, value string) error {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		rawJSON, errSet = sjson.SetBytes(rawJSON, path, value)
		return errSet
	}
	for _, field := range []string{"model", "prompt", "seconds", "size", "aspect_ratio", "resolution"} {
		if err := setString(field, values.Get(field)); err != nil {
			return nil, err
		}
	}
	if err := setString("input_reference.image_url", firstFormValue(values, "input_reference[image_url]", "input_reference.image_url", "image_url")); err != nil {
		return nil, err
	}
	if err := setString("input_reference.file_id", firstFormValue(values, "input_reference[file_id]", "input_reference.file_id", "file_id")); err != nil {
		return nil, err
	}
	for _, ref := range strings.Split(strings.TrimSpace(values.Get("reference_image_urls")), ",") {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		rawJSON, errSet = sjson.SetBytes(rawJSON, "reference_image_urls.-1", ref)
		if errSet != nil {
			return nil, errSet
		}
	}
	return rawJSON, nil
}

func firstFormValue(values url.Values, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

// ExtractGrokVideoRequestIDFromPath returns the async video job id from a client path
// such as /v1/videos/{id}, /videos/{id}/content, or /v1/videos/{id}?foo=bar.
func ExtractGrokVideoRequestIDFromPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if idx := strings.IndexAny(trimmed, "?#"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	trimmed = strings.Trim(trimmed, "/")
	parts := strings.Split(trimmed, "/")
	// Accept both /v1/videos/{id}[...] and /videos/{id}[...].
	for i := 0; i < len(parts); i++ {
		if !strings.EqualFold(parts[i], "videos") {
			continue
		}
		if i+1 >= len(parts) {
			return ""
		}
		candidate := strings.TrimSpace(parts[i+1])
		switch strings.ToLower(candidate) {
		case "", "generations", "edits", "extensions":
			return ""
		default:
			return candidate
		}
	}
	return ""
}

func canonicalOpenAIVideoTargetPath(targetPath string) string {
	trimmed := strings.TrimSpace(targetPath)
	if trimmed == "" {
		return "/v1/videos"
	}
	path := trimmed
	query := ""
	if idx := strings.IndexAny(path, "?#"); idx >= 0 {
		query = path[idx:]
		path = path[:idx]
	}
	if path == "" {
		path = "/v1/videos"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	lowerPath := strings.ToLower(path)
	if lowerPath == "/videos" || strings.HasPrefix(lowerPath, "/videos/") {
		path = "/v1" + path
	}
	return path + query
}

func isOpenAIVideoBillingRequest(method, targetPath string) bool {
	if !strings.EqualFold(strings.TrimSpace(method), http.MethodPost) {
		return false
	}
	canonical := canonicalOpenAIVideoTargetPath(targetPath)
	if idx := strings.IndexAny(canonical, "?#"); idx >= 0 {
		canonical = canonical[:idx]
	}
	switch strings.ToLower(strings.TrimRight(canonical, "/")) {
	case "/v1/videos", "/v1/videos/generations", "/v1/videos/edits", "/v1/videos/extensions":
		return true
	default:
		return false
	}
}

// isOpenAIVideoContentPath reports GET .../videos/{id}/content binary download paths.
func isOpenAIVideoContentPath(targetPath string) bool {
	canonical := canonicalOpenAIVideoTargetPath(targetPath)
	if idx := strings.IndexAny(canonical, "?#"); idx >= 0 {
		canonical = canonical[:idx]
	}
	canonical = strings.ToLower(strings.TrimRight(canonical, "/"))
	return strings.HasSuffix(canonical, "/content") && strings.Contains(canonical, "/videos/")
}

func gjsonPositiveInt(body []byte, path string) int {
	value := gjson.GetBytes(body, path)
	if !value.Exists() {
		return 0
	}
	if value.Type == gjson.Number {
		return int(value.Int())
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value.String()))
	if err != nil {
		return 0
	}
	return parsed
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
