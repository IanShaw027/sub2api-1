package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func (s *OpenAIGatewayService) ForwardEmbeddings(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
) (*OpenAIForwardResult, error) {
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
	releaseUpstreamCtx()
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
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

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	tlsRuntime := s.resolveOpenAITLSFingerprintRuntime(ctx, c, account, "http")
	applyOpenAITLSFingerprintRuntime(upstreamReq, tlsRuntime)
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
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			upstreamDetail := ""
			if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
				maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
				if maxBytes <= 0 {
					maxBytes = 2048
				}
				upstreamDetail = truncateString(string(respBody), maxBytes)
			}
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

// ForwardVideos forwards to OpenAI-compatible /v1/videos (create/retrieve/content).
// Supports provider accounts that expose an OpenAI-compatible videos endpoint.
// Uses account token (API key or OAuth/GetAccessToken) + appropriate base URL.
// Billing for video (resolution tier x seconds via group video_price_*_per_sec) should be applied by caller or usage recorder.
func (s *OpenAIGatewayService) ForwardVideos(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	targetPath string, // e.g. "/v1/videos" or "/v1/videos/xxx/content"
) (*OpenAIForwardResult, error) {
	startTime := time.Now()

	originalModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())

	logger.L().Debug("openai videos: forwarding",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", originalModel),
		zap.String("target_path", targetPath),
	)

	// Support OpenAI-compatible providers that need bearer tokens or API keys.
	token := account.GetOpenAIApiKey()
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	if account.Platform == PlatformGrok || account.Type == AccountTypeOAuth {
		tok, _, err := s.GetAccessToken(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("get access token for video: %w", err)
		}
		token = tok
		if grokBase := account.GetGrokBaseURL(); grokBase != "" {
			baseURL = grokBase
		} else if account.Platform == PlatformGrok {
			baseURL = "https://api.x.ai"
		}
	}
	if token == "" {
		return nil, fmt.Errorf("account %d missing token for video generation", account.ID)
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}
	if targetPath == "" {
		targetPath = "/v1/videos"
	}
	targetURL := buildOpenAIEndpointURL(validatedURL, targetPath)

	upstreamCtx, release := detachUpstreamContext(ctx)
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, c.Request.Method, targetURL, bytes.NewReader(body))
	release()
	if err != nil {
		return nil, fmt.Errorf("build upstream video request: %w", err)
	}
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	upstreamReq.Header.Set("Content-Type", "application/json")
	if len(body) == 0 || c.ContentType() == "" {
		// allow form for reference uploads if needed
		if ct := c.GetHeader("Content-Type"); ct != "" {
			upstreamReq.Header.Set("Content-Type", ct)
		}
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	upstreamReq.Header.Set("Accept", "application/json")
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
	tlsRuntime := s.resolveOpenAITLSFingerprintRuntime(ctx, c, account, "http")
	applyOpenAITLSFingerprintRuntime(upstreamReq, tlsRuntime)

	resp, err := s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		return nil, &UpstreamFailoverError{StatusCode: 0, ResponseBody: nil, RetryableOnSameAccount: true}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		if hit, code, cyberMsg := detectOpenAICyberPolicy(respBody); hit {
			MarkOpsCyberPolicy(c, CyberPolicyMark{
				Code:           code,
				Message:        cyberMsg,
				Body:           truncateString(string(respBody), 4096),
				UpstreamStatus: resp.StatusCode,
			})
			setOpsUpstreamError(c, resp.StatusCode, cyberMsg, truncateString(string(respBody), 2048))
			c.Header("Content-Type", "application/json")
			c.Status(resp.StatusCode)
			_, _ = c.Writer.Write(respBody)
			if cyberMsg == "" {
				return nil, fmt.Errorf("openai video cyber_policy: %d", resp.StatusCode)
			}
			return nil, fmt.Errorf("openai video cyber_policy: %s", cyberMsg)
		}

		upstreamMsg := strings.TrimSpace(extractUpstreamErrorMessage(respBody))
		upstreamMsg = sanitizeUpstreamErrorMessage(upstreamMsg)
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMsg, respBody) {
			upstreamDetail := ""
			if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
				maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
				if maxBytes <= 0 {
					maxBytes = 2048
				}
				upstreamDetail = truncateString(string(respBody), maxBytes)
			}
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
			s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, originalModel)
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				ResponseHeaders:        resp.Header.Clone(),
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}

		c.Header("Content-Type", "application/json")
		c.Status(resp.StatusCode)
		_, _ = c.Writer.Write(respBody)
		return nil, fmt.Errorf("upstream video error: status %d", resp.StatusCode)
	}

	// write success
	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Header(k, v)
		}
	}
	c.Status(resp.StatusCode)
	_, _ = c.Writer.Write(respBody)

	// minimal result
	videoModel := firstNonEmptyString(strings.TrimSpace(gjson.GetBytes(respBody, "model").String()), originalModel)
	videoSeconds := firstPositiveInt(
		gjsonPositiveInt(body, "seconds"),
		gjsonPositiveInt(respBody, "seconds"),
		gjsonPositiveInt(respBody, "duration"),
		gjsonPositiveInt(respBody, "duration_seconds"),
	)
	videoSize := NormalizeVideoBillingTierOrDefault(firstNonEmptyString(
		strings.TrimSpace(gjson.GetBytes(body, "size").String()),
		strings.TrimSpace(gjson.GetBytes(respBody, "size").String()),
		strings.TrimSpace(gjson.GetBytes(respBody, "resolution").String()),
	))
	videoCount := firstPositiveInt(
		gjsonPositiveInt(body, "n_variants"),
		gjsonPositiveInt(body, "n"),
		int(gjson.GetBytes(respBody, "data.#").Int()),
		1,
	)
	res := &OpenAIForwardResult{
		RequestID:       firstNonEmptyString(resp.Header.Get("x-request-id"), resp.Header.Get("request-id"), resp.Header.Get("xai-request-id")),
		Model:           videoModel,
		UpstreamModel:   videoModel,
		Usage:           OpenAIUsage{},
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
		VideoSeconds:    videoSeconds,
		VideoSize:       videoSize,
		VideoCount:      videoCount,
	}
	_ = startTime
	return res, nil
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
