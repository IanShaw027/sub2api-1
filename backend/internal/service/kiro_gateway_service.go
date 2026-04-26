package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	gocache "github.com/patrickmn/go-cache"
)

const (
	kiroPreludeSize = 12
	kiroMinMsgSize  = kiroPreludeSize + 4
	kiroMaxBodySize = 16 << 20
)

type KiroGatewayService struct {
	httpUpstream      HTTPUpstream
	tokenProvider     *KiroTokenProvider
	rateLimitService  *RateLimitService
	tlsFPProfileSvc   *TLSFingerprintProfileService
	settingService    *SettingService
	fakeCache         *gocache.Cache
	fakeCacheMu       sync.Mutex
	fakeCacheStrategy string
	fakeCacheGen      uint64
}

func NewKiroGatewayService(
	httpUpstream HTTPUpstream,
	tokenProvider *KiroTokenProvider,
	rateLimitService *RateLimitService,
	tlsFPProfileSvc *TLSFingerprintProfileService,
	settingService *SettingService,
) *KiroGatewayService {
	return &KiroGatewayService{
		httpUpstream:     httpUpstream,
		tokenProvider:    tokenProvider,
		rateLimitService: rateLimitService,
		tlsFPProfileSvc:  tlsFPProfileSvc,
		settingService:   settingService,
		fakeCache:        gocache.New(kiropkg.DefaultFakeCacheTTL, time.Minute),
	}
}

func (s *KiroGatewayService) Forward(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest) (*ForwardResult, error) {
	converted, err := s.validateAndConvertRequest(c, account, parsed)
	if err != nil {
		return nil, err
	}
	if c != nil && converted != nil {
		setOpsUpstreamRequestBody(c, converted.Body)
	}

	accessToken, err := s.resolveAccessToken(ctx, account)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": "Failed to get Kiro access token"},
		})
		return nil, err
	}

	runtimeSettings := s.resolveKiroRuntimeSettings(ctx)
	fakeCachePlan, fakeCacheHit := s.prepareFakeCachePlan(account, parsed, runtimeSettings)
	req, err := s.buildRequest(ctx, account, converted.Body, accessToken, runtimeSettings)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": "Failed to build Kiro upstream request"},
		})
		return nil, err
	}

	start := time.Now()
	resp, err := s.httpUpstream.DoWithTLS(req, accountProxyURL(account), account.ID, account.Concurrency, s.resolveTLSProfile(account))
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(start).Milliseconds())
	if err != nil {
		s.handleUpstreamError(ctx, account, http.StatusBadGateway, http.Header{}, []byte(err.Error()))
		s.recordOpsRequestError(c, account, req.URL.String(), err)
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": "Kiro upstream request failed"},
		})
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		s.handleUpstreamError(ctx, account, resp.StatusCode, resp.Header, body)
		s.recordOpsHTTPError(c, account, req.URL.String(), resp.StatusCode, resp.Header, body)
		if shouldKiroFailover(resp.StatusCode) {
			return nil, &UpstreamFailoverError{
				StatusCode:   resp.StatusCode,
				ResponseBody: body,
			}
		}
		upstreamMessage := kiroSafeHTTPStatusErrorMessage("Kiro upstream", resp.StatusCode, body)
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": upstreamMessage},
		})
		return nil, fmt.Errorf("%s", upstreamMessage)
	}
	inputTokens := kiropkg.EstimateInputTokens(parsed.Body)
	if parsed.Stream {
		return s.forwardStream(ctx, c, account, resp, parsed, converted, inputTokens, start, fakeCachePlan, fakeCacheHit, runtimeSettings)
	}
	return s.forwardNonStream(ctx, c, account, resp, parsed, converted, inputTokens, start, fakeCachePlan, fakeCacheHit, runtimeSettings)
}

func (s *KiroGatewayService) ForwardCountTokens(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest) error {
	if _, err := s.validateAndConvertRequest(c, account, parsed); err != nil {
		return err
	}
	inputTokens := kiropkg.EstimateInputTokens(parsed.Body)
	c.JSON(http.StatusOK, gin.H{"input_tokens": inputTokens})
	return nil
}

func (s *KiroGatewayService) validateAndConvertRequest(c *gin.Context, account *Account, parsed *ParsedRequest) (*kiropkg.ConvertResult, error) {
	if account == nil || parsed == nil {
		err := fmt.Errorf("invalid kiro request args")
		writeKiroInvalidRequest(c, err)
		return nil, err
	}

	requestedModel, err := resolveKiroRequestedModel(account, parsed.Model)
	if err != nil {
		writeKiroInvalidRequest(c, err)
		return nil, err
	}

	converted, err := kiropkg.ConvertAnthropicRequestWithModel(parsed.Body, requestedModel)
	if err != nil {
		writeKiroInvalidRequest(c, err)
		return nil, err
	}
	return converted, nil
}

func resolveKiroRequestedModel(account *Account, requestedModel string) (string, error) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return "", nil
	}
	if mapKiroModel(account, requestedModel) == "" {
		return "", fmt.Errorf("unsupported kiro model: %s", requestedModel)
	}
	if mappedModel, matched := resolveKiroMappedModel(account, requestedModel); matched {
		return mappedModel, nil
	}
	return requestedModel, nil
}

func (s *KiroGatewayService) resolveAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Type == AccountTypeAPIKey {
		apiKey := strings.TrimSpace(account.GetCredential("api_key"))
		if apiKey == "" {
			return "", errors.New("api_key not found in credentials")
		}
		return apiKey, nil
	}
	if s.tokenProvider == nil {
		return "", errors.New("kiro token provider is not configured")
	}
	return s.tokenProvider.GetAccessToken(ctx, account)
}

func (s *KiroGatewayService) resolveKiroRuntimeSettings(ctx context.Context) *KiroRuntimeSettings {
	if s != nil && s.settingService != nil {
		return s.settingService.GetKiroRuntimeSettings(ctx)
	}
	return DefaultKiroRuntimeSettings()
}

func writeKiroInvalidRequest(c *gin.Context, err error) {
	if c == nil || err == nil {
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"type":  "error",
		"error": gin.H{"type": "invalid_request_error", "message": err.Error()},
	})
}

func (s *KiroGatewayService) buildRequest(ctx context.Context, account *Account, body []byte, accessToken string, runtimeSettings *KiroRuntimeSettings) (*http.Request, error) {
	return buildKiroGenerateAssistantRequest(ctx, account, body, accessToken, runtimeSettings)
}

func buildKiroGenerateAssistantRequest(ctx context.Context, account *Account, body []byte, accessToken string, runtimeSettings *KiroRuntimeSettings) (*http.Request, error) {
	url := fmt.Sprintf("https://q.%s.amazonaws.com/generateAssistantResponse", KiroRegion(account))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	runtimeSettings = normalizeKiroRuntimeSettings(runtimeSettings)
	machineID := kiropkg.GenerateMachineID(account.GetCredential("machine_id"), "", account.GetCredential("refresh_token"))
	host := fmt.Sprintf("q.%s.amazonaws.com", KiroRegion(account))
	kiroVersion := runtimeSettings.KiroVersion
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("host", host)
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Header.Set("x-amz-user-agent", fmt.Sprintf("aws-sdk-js/1.0.27 KiroIDE-%s-%s", kiroVersion, machineID))
	req.Header.Set("User-Agent", fmt.Sprintf("aws-sdk-js/1.0.27 ua/2.1 os/%s lang/js md/nodejs#%s api/codewhispererstreaming#1.0.27 m/E KiroIDE-%s-%s", runtimeSettings.SystemVersion, runtimeSettings.NodeVersion, kiroVersion, machineID))
	if runtimeSettings.KiroCommit != "" {
		req.Header.Set("x-amzn-kiro-commit", runtimeSettings.KiroCommit)
	}
	req.Header.Set("amz-sdk-invocation-id", generateRequestID())
	req.Header.Set("amz-sdk-request", "attempt=1; max=3")
	return req, nil
}

func (s *KiroGatewayService) forwardNonStream(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, parsed *ParsedRequest, converted *kiropkg.ConvertResult, inputTokens int, start time.Time, fakeCachePlan *kiropkg.FakeCachePlan, fakeCacheHit kiropkg.FakeCacheHitState, runtimeSettings *KiroRuntimeSettings) (*ForwardResult, error) {
	frames, err := readAllKiroFrames(resp.Body)
	if err != nil {
		s.handleProtocolError(ctx, c, account, parsed.Model, false, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": "Failed to decode Kiro response"},
		})
		return nil, err
	}

	textBuilder := strings.Builder{}
	toolUses := make([]map[string]any, 0)
	toolBuffers := make(map[string]*kiroToolState)
	stopReason := "end_turn"
	contextInputTokens := 0
	hasVisibleOutput := false

	for _, frame := range frames {
		if failureErr := kiroFrameFailure(frame); failureErr != nil {
			return nil, s.handleFrameFailure(ctx, c, account, resp.Header.Get("x-amzn-requestid"), frame, failureErr, true)
		}

		switch frame.EventType {
		case "assistantResponseEvent":
			if content := rawStringField(frame.Payload, "content"); content != "" {
				_, _ = textBuilder.WriteString(content)
				hasVisibleOutput = true
			}
		case "toolUseEvent":
			state := ensureKiroToolState(toolBuffers, controlStringField(frame.Payload, "toolUseId"), controlStringField(frame.Payload, "name"))
			_, _ = state.InputBuilder.WriteString(rawStringField(frame.Payload, "input"))
			if booleanField(frame.Payload, "stop") {
				state.Stopped = true
				input := map[string]any{}
				_ = json.Unmarshal([]byte(state.InputBuilder.String()), &input)
				toolUses = append(toolUses, map[string]any{
					"type":  "tool_use",
					"id":    state.ToolUseID,
					"name":  state.Name,
					"input": input,
				})
				stopReason = "tool_use"
				hasVisibleOutput = true
			}
		case "contextUsageEvent":
			contextInputTokens = contextUsageToTokens(numberField(frame.Payload, "contextUsagePercentage"))
		}
	}
	if !hasVisibleOutput {
		emptyErr := errors.New("kiro response contained no assistant output")
		s.handleProtocolError(ctx, c, account, parsed.Model, false, emptyErr)
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": "Kiro upstream returned no assistant output"},
		})
		return nil, emptyErr
	}
	if parsed.OnUpstreamAccepted != nil {
		parsed.OnUpstreamAccepted()
	}

	content := make([]map[string]any, 0)
	if text := textBuilder.String(); text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	content = append(content, toolUses...)
	if contextInputTokens > 0 {
		inputTokens = contextInputTokens
	}
	fakeCacheUsage := resolveKiroFakeCacheUsage(fakeCachePlan, fakeCacheHit, inputTokens, runtimeSettings)
	inputTokens = fakeCacheUsage.InputTokens
	outputTokens := kiropkg.EstimateOutputTokens(textBuilder.String())
	s.commitFakeCachePlan(fakeCachePlan, runtimeSettings)

	c.JSON(http.StatusOK, gin.H{
		"id":            "msg_" + strings.ReplaceAll(generateRequestID(), "-", ""),
		"type":          "message",
		"role":          "assistant",
		"content":       content,
		"model":         parsed.Model,
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage": gin.H{
			"input_tokens":                inputTokens,
			"output_tokens":               outputTokens,
			"cache_creation_input_tokens": fakeCacheUsage.CacheCreationInputTokens,
			"cache_read_input_tokens":     fakeCacheUsage.CacheReadInputTokens,
		},
	})

	return &ForwardResult{
		RequestID:     resp.Header.Get("x-amzn-requestid"),
		Model:         parsed.Model,
		UpstreamModel: converted.Model,
		Stream:        false,
		Duration:      time.Since(start),
		Usage: ClaudeUsage{
			InputTokens:              inputTokens,
			OutputTokens:             outputTokens,
			CacheCreationInputTokens: fakeCacheUsage.CacheCreationInputTokens,
			CacheReadInputTokens:     fakeCacheUsage.CacheReadInputTokens,
		},
	}, nil
}

func (s *KiroGatewayService) forwardStream(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, parsed *ParsedRequest, converted *kiropkg.ConvertResult, inputTokens int, start time.Time, fakeCachePlan *kiropkg.FakeCachePlan, fakeCacheHit kiropkg.FakeCacheHitState, runtimeSettings *KiroRuntimeSettings) (*ForwardResult, error) {
	writer := c.Writer
	msgID := "msg_" + strings.ReplaceAll(generateRequestID(), "-", "")
	reader := bufio.NewReader(resp.Body)
	buffer := make([]byte, 0, 64*1024)
	streamStarted := false
	textBlockOpen := false
	textBlockIndex := -1
	nextBlockIndex := 0
	toolStates := make(map[string]*kiroToolState)
	var firstTokenMs *int
	var outputBuilder strings.Builder
	stopReason := "end_turn"
	contextInputTokens := 0
	framesSeen := 0
	completedToolUses := 0

	for {
		chunk := make([]byte, 4096)
		n, readErr := reader.Read(chunk)
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)
			if len(buffer) > kiroMaxBodySize {
				err := fmt.Errorf("kiro stream buffer exceeded limit %d", kiroMaxBodySize)
				s.handleProtocolError(ctx, c, account, parsed.Model, true, err)
				return nil, err
			}
			for {
				frame, consumed, ok, err := parseKiroFrame(buffer)
				if err != nil {
					s.handleProtocolError(ctx, c, account, parsed.Model, true, err)
					return nil, err
				}
				if !ok {
					break
				}
				buffer = buffer[consumed:]
				framesSeen++

				if failureErr := kiroFrameFailure(frame); failureErr != nil {
					if streamStarted {
						handledErr := s.handleFrameFailure(ctx, c, account, resp.Header.Get("x-amzn-requestid"), frame, failureErr, false)
						if err := closeOpenKiroBlocks(writer, textBlockOpen, textBlockIndex, toolStates); err != nil {
							return nil, err
						}
						_ = writeKiroStreamError(writer, failureErr.Error())
						var failoverErr *UpstreamFailoverError
						if handledErr != nil && !errors.As(handledErr, &failoverErr) {
							return nil, handledErr
						}
						return nil, failureErr
					}
					return nil, s.handleFrameFailure(ctx, c, account, resp.Header.Get("x-amzn-requestid"), frame, failureErr, true)
				}

				switch frame.EventType {
				case "assistantResponseEvent":
					content := rawStringField(frame.Payload, "content")
					if content == "" {
						continue
					}
					if !streamStarted {
						initialInputTokens := inputTokens
						if contextInputTokens > 0 {
							initialInputTokens = contextInputTokens
						}
						if parsed.OnUpstreamAccepted != nil {
							parsed.OnUpstreamAccepted()
						}
						if err := startKiroStream(writer, msgID, parsed.Model, resolveKiroFakeCacheUsage(fakeCachePlan, fakeCacheHit, initialInputTokens, runtimeSettings)); err != nil {
							return nil, err
						}
						streamStarted = true
					}
					if firstTokenMs == nil {
						v := int(time.Since(start).Milliseconds())
						firstTokenMs = &v
					}
					if !textBlockOpen {
						textBlockIndex = nextBlockIndex
						nextBlockIndex++
						textBlockOpen = true
						if err := writeSSEEvent(writer, "content_block_start", map[string]any{
							"type":  "content_block_start",
							"index": textBlockIndex,
							"content_block": map[string]any{
								"type": "text",
								"text": "",
							},
						}); err != nil {
							return nil, err
						}
					}
					_, _ = outputBuilder.WriteString(content)
					if err := writeSSEEvent(writer, "content_block_delta", map[string]any{
						"type":  "content_block_delta",
						"index": textBlockIndex,
						"delta": map[string]any{
							"type": "text_delta",
							"text": content,
						},
					}); err != nil {
						return nil, err
					}
				case "toolUseEvent":
					toolUseID := controlStringField(frame.Payload, "toolUseId")
					state := ensureKiroToolState(toolStates, toolUseID, controlStringField(frame.Payload, "name"))
					if !streamStarted {
						initialInputTokens := inputTokens
						if contextInputTokens > 0 {
							initialInputTokens = contextInputTokens
						}
						if parsed.OnUpstreamAccepted != nil {
							parsed.OnUpstreamAccepted()
						}
						if err := startKiroStream(writer, msgID, parsed.Model, resolveKiroFakeCacheUsage(fakeCachePlan, fakeCacheHit, initialInputTokens, runtimeSettings)); err != nil {
							return nil, err
						}
						streamStarted = true
					}
					if !state.Started {
						if textBlockOpen {
							if err := writeSSEEvent(writer, "content_block_stop", map[string]any{
								"type":  "content_block_stop",
								"index": textBlockIndex,
							}); err != nil {
								return nil, err
							}
							textBlockOpen = false
						}
						state.Started = true
						state.BlockIndex = nextBlockIndex
						nextBlockIndex++
						if err := writeSSEEvent(writer, "content_block_start", map[string]any{
							"type":  "content_block_start",
							"index": state.BlockIndex,
							"content_block": map[string]any{
								"type":  "tool_use",
								"id":    state.ToolUseID,
								"name":  state.Name,
								"input": map[string]any{},
							},
						}); err != nil {
							return nil, err
						}
					}
					inputChunk := rawStringField(frame.Payload, "input")
					if inputChunk != "" {
						_, _ = state.InputBuilder.WriteString(inputChunk)
						if err := writeSSEEvent(writer, "content_block_delta", map[string]any{
							"type":  "content_block_delta",
							"index": state.BlockIndex,
							"delta": map[string]any{
								"type":         "input_json_delta",
								"partial_json": inputChunk,
							},
						}); err != nil {
							return nil, err
						}
					}
					if booleanField(frame.Payload, "stop") {
						stopReason = "tool_use"
						state.Stopped = true
						completedToolUses++
						if err := writeSSEEvent(writer, "content_block_stop", map[string]any{
							"type":  "content_block_stop",
							"index": state.BlockIndex,
						}); err != nil {
							return nil, err
						}
					}
				case "contextUsageEvent":
					contextInputTokens = contextUsageToTokens(numberField(frame.Payload, "contextUsagePercentage"))
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				if len(buffer) > 0 {
					incompleteErr := fmt.Errorf("incomplete kiro frame at EOF: %w", io.ErrUnexpectedEOF)
					if streamStarted {
						if err := closeOpenKiroBlocks(writer, textBlockOpen, textBlockIndex, toolStates); err != nil {
							return nil, err
						}
						_ = writeKiroStreamError(writer, incompleteErr.Error())
					}
					s.handleProtocolError(ctx, c, account, parsed.Model, true, incompleteErr)
					return nil, incompleteErr
				}
				if framesSeen == 0 {
					emptyErr := errors.New("empty kiro response body")
					s.handleProtocolError(ctx, c, account, parsed.Model, true, emptyErr)
					return nil, emptyErr
				}
				break
			}
			if streamStarted {
				if err := closeOpenKiroBlocks(writer, textBlockOpen, textBlockIndex, toolStates); err != nil {
					return nil, err
				}
				_ = writeKiroStreamError(writer, readErr.Error())
			}
			s.handleProtocolError(ctx, c, account, parsed.Model, true, readErr)
			return nil, readErr
		}
	}
	if !streamStarted || (outputBuilder.Len() == 0 && completedToolUses == 0) {
		emptyErr := errors.New("kiro response contained no assistant output")
		s.handleProtocolError(ctx, c, account, parsed.Model, true, emptyErr)
		return nil, emptyErr
	}

	if textBlockOpen {
		if err := writeSSEEvent(writer, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": textBlockIndex,
		}); err != nil {
			return nil, err
		}
	}

	if contextInputTokens > 0 {
		inputTokens = contextInputTokens
	}
	finalFakeCacheUsage := resolveKiroFakeCacheUsage(fakeCachePlan, fakeCacheHit, inputTokens, runtimeSettings)
	inputTokens = finalFakeCacheUsage.InputTokens
	outputTokens := kiropkg.EstimateOutputTokens(outputBuilder.String())
	if err := writeSSEEvent(writer, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
		"usage": map[string]any{
			"input_tokens":                inputTokens,
			"output_tokens":               outputTokens,
			"cache_creation_input_tokens": finalFakeCacheUsage.CacheCreationInputTokens,
			"cache_read_input_tokens":     finalFakeCacheUsage.CacheReadInputTokens,
		},
	}); err != nil {
		return nil, err
	}
	if err := writeSSEEvent(writer, "message_stop", map[string]any{"type": "message_stop"}); err != nil {
		return nil, err
	}
	s.commitFakeCachePlan(fakeCachePlan, runtimeSettings)

	return &ForwardResult{
		RequestID:     resp.Header.Get("x-amzn-requestid"),
		Model:         parsed.Model,
		UpstreamModel: converted.Model,
		Stream:        true,
		Duration:      time.Since(start),
		FirstTokenMs:  firstTokenMs,
		Usage: ClaudeUsage{
			InputTokens:              inputTokens,
			OutputTokens:             outputTokens,
			CacheCreationInputTokens: finalFakeCacheUsage.CacheCreationInputTokens,
			CacheReadInputTokens:     finalFakeCacheUsage.CacheReadInputTokens,
		},
	}, nil
}

func (s *KiroGatewayService) prepareFakeCachePlan(account *Account, parsed *ParsedRequest, runtimeSettings *KiroRuntimeSettings) (*kiropkg.FakeCachePlan, kiropkg.FakeCacheHitState) {
	if s == nil || account == nil || parsed == nil {
		return nil, kiropkg.FakeCacheHitState{}
	}

	plan, err := kiropkg.BuildFakeCachePlan(parsed.Body, account.ID, parsed.Model)
	if err != nil || plan == nil {
		return nil, kiropkg.FakeCacheHitState{}
	}

	hit := kiropkg.FakeCacheHitState{}
	if s.fakeCache != nil {
		strategy := kiroFakeCacheStrategy(runtimeSettings)
		s.fakeCacheMu.Lock()
		s.refreshFakeCacheStrategyLocked(strategy)
		plan.CacheStrategy = s.fakeCacheStrategy
		plan.CacheStrategyGeneration = s.fakeCacheGen
		if plan.IndependentKey != "" {
			_, hit.Independent = s.fakeCache.Get(plan.IndependentKey)
		}
		if plan.PreviousPrefixKey != "" {
			_, hit.Prefix = s.fakeCache.Get(plan.PreviousPrefixKey)
		}
		s.fakeCacheMu.Unlock()
	}

	return plan, hit
}

func (s *KiroGatewayService) commitFakeCachePlan(plan *kiropkg.FakeCachePlan, runtimeSettings *KiroRuntimeSettings) {
	if s == nil || s.fakeCache == nil || plan == nil {
		return
	}
	strategy := kiroFakeCacheStrategy(runtimeSettings)
	s.fakeCacheMu.Lock()
	defer s.fakeCacheMu.Unlock()
	if plan.CacheStrategy == "" || plan.CacheStrategyGeneration == 0 {
		return
	}
	if strategy != plan.CacheStrategy || s.fakeCacheStrategy != plan.CacheStrategy || s.fakeCacheGen != plan.CacheStrategyGeneration {
		return
	}
	runtimeSettings = normalizeKiroRuntimeSettings(runtimeSettings)
	if plan.IndependentKey != "" && plan.IndependentCacheableTokens > 0 {
		s.fakeCache.Set(plan.IndependentKey, struct{}{}, time.Duration(runtimeSettings.CacheIndependentTTLSecs)*time.Second)
	}
	if plan.CurrentPrefixKey != "" && plan.CurrentPrefixCacheableTokens > 0 {
		s.fakeCache.Set(plan.CurrentPrefixKey, struct{}{}, time.Duration(runtimeSettings.CachePrefixTTLSecs)*time.Second)
		return
	}
	if plan.CurrentKey != "" && plan.CurrentCacheableTokens > 0 {
		s.fakeCache.Set(plan.CurrentKey, struct{}{}, time.Duration(runtimeSettings.CachePrefixTTLSecs)*time.Second)
	}
}

func (s *KiroGatewayService) refreshFakeCacheStrategy(runtimeSettings *KiroRuntimeSettings) {
	if s == nil || s.fakeCache == nil {
		return
	}
	strategy := kiroFakeCacheStrategy(runtimeSettings)
	s.fakeCacheMu.Lock()
	defer s.fakeCacheMu.Unlock()
	s.refreshFakeCacheStrategyLocked(strategy)
}

func kiroFakeCacheStrategy(runtimeSettings *KiroRuntimeSettings) string {
	runtimeSettings = normalizeKiroRuntimeSettings(runtimeSettings)
	return fmt.Sprintf(
		"hit:%d|min:%d|ind:%d|prefix:%d",
		runtimeSettings.CacheHitRateScale,
		runtimeSettings.CacheMinBlockTokens,
		runtimeSettings.CacheIndependentTTLSecs,
		runtimeSettings.CachePrefixTTLSecs,
	)
}

func (s *KiroGatewayService) refreshFakeCacheStrategyLocked(strategy string) {
	if s.fakeCacheStrategy == "" {
		s.fakeCacheStrategy = strategy
		s.fakeCacheGen = 1
		return
	}
	if s.fakeCacheStrategy != strategy {
		s.fakeCache.Flush()
		s.fakeCacheStrategy = strategy
		s.fakeCacheGen++
	}
}

func resolveKiroFakeCacheUsage(plan *kiropkg.FakeCachePlan, hit kiropkg.FakeCacheHitState, totalInputTokens int, runtimeSettings *KiroRuntimeSettings) kiropkg.FakeCacheUsage {
	if totalInputTokens < 0 {
		totalInputTokens = 0
	}
	if plan == nil {
		return kiropkg.FakeCacheUsage{InputTokens: totalInputTokens}
	}
	runtimeSettings = normalizeKiroRuntimeSettings(runtimeSettings)
	return plan.ResolveUsageWithConfig(totalInputTokens, hit, kiropkg.FakeCacheUsageConfig{
		HitRateScale:   runtimeSettings.CacheHitRateScale,
		MinBlockTokens: runtimeSettings.CacheMinBlockTokens,
	})
}

type kiroFrame struct {
	MessageType string
	EventType   string
	Payload     map[string]any
}

type kiroToolState struct {
	ToolUseID    string
	Name         string
	BlockIndex   int
	Started      bool
	Stopped      bool
	InputBuilder strings.Builder
}

func ensureKiroToolState(states map[string]*kiroToolState, toolUseID, name string) *kiroToolState {
	if toolUseID == "" {
		toolUseID = generateRequestID()
	}
	if state, ok := states[toolUseID]; ok {
		if state.Name == "" {
			state.Name = name
		}
		return state
	}
	state := &kiroToolState{ToolUseID: toolUseID, Name: name}
	states[toolUseID] = state
	return state
}

func parseKiroFrame(buffer []byte) (*kiroFrame, int, bool, error) {
	if len(buffer) < kiroPreludeSize {
		return nil, 0, false, nil
	}

	totalLength := int(binary.BigEndian.Uint32(buffer[0:4]))
	headerLength := int(binary.BigEndian.Uint32(buffer[4:8]))
	preludeCRC := binary.BigEndian.Uint32(buffer[8:12])
	if totalLength > kiroMaxBodySize {
		return nil, 0, false, fmt.Errorf("kiro frame exceeded limit %d", kiroMaxBodySize)
	}
	if totalLength < kiroMinMsgSize || headerLength < 0 {
		return nil, 0, false, fmt.Errorf("invalid kiro frame size")
	}
	if kiroPreludeSize+headerLength > totalLength-4 {
		return nil, 0, false, fmt.Errorf("invalid kiro frame header size")
	}
	if len(buffer) < totalLength {
		return nil, 0, false, nil
	}
	if crc32.ChecksumIEEE(buffer[:8]) != preludeCRC {
		return nil, 0, false, fmt.Errorf("kiro prelude crc mismatch")
	}

	messageCRC := binary.BigEndian.Uint32(buffer[totalLength-4 : totalLength])
	if crc32.ChecksumIEEE(buffer[:totalLength-4]) != messageCRC {
		return nil, 0, false, fmt.Errorf("kiro message crc mismatch")
	}

	headersBytes := buffer[kiroPreludeSize : kiroPreludeSize+headerLength]
	headers, err := parseKiroHeaders(headersBytes)
	if err != nil {
		return nil, 0, false, err
	}

	payloadBytes := buffer[kiroPreludeSize+headerLength : totalLength-4]
	payload := map[string]any{}
	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			return nil, 0, false, fmt.Errorf("invalid kiro payload json: %w", err)
		}
	}

	frame := &kiroFrame{
		MessageType: headers[":message-type"],
		EventType:   headers[":event-type"],
		Payload:     payload,
	}
	if frame.MessageType == "" {
		if _, ok := headers[":exception-type"]; ok {
			frame.MessageType = "exception"
		} else if _, ok := headers[":error-code"]; ok {
			frame.MessageType = "error"
		}
	}
	if frame.MessageType == "" {
		frame.MessageType = "event"
	}

	return frame, totalLength, true, nil
}

func parseKiroHeaders(data []byte) (map[string]string, error) {
	headers := make(map[string]string)
	for offset := 0; offset < len(data); {
		nameLen := int(data[offset])
		offset++
		if offset+nameLen > len(data) {
			return nil, fmt.Errorf("kiro header out of bounds")
		}
		name := string(data[offset : offset+nameLen])
		offset += nameLen
		if offset >= len(data) {
			return nil, fmt.Errorf("kiro header missing type")
		}
		valueType := data[offset]
		offset++
		switch valueType {
		case 7:
			if offset+2 > len(data) {
				return nil, fmt.Errorf("kiro header string length missing")
			}
			valueLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2
			if offset+valueLen > len(data) {
				return nil, fmt.Errorf("kiro header string out of bounds")
			}
			headers[name] = string(data[offset : offset+valueLen])
			offset += valueLen
		case 0:
			headers[name] = "true"
		case 1:
			headers[name] = "false"
		case 2:
			offset++
		case 3:
			offset += 2
		case 4:
			offset += 4
		case 5, 8:
			offset += 8
		case 6:
			if offset+2 > len(data) {
				return nil, fmt.Errorf("kiro header byte array length missing")
			}
			valueLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2 + valueLen
		case 9:
			offset += 16
		default:
			return nil, fmt.Errorf("unknown kiro header type %d", valueType)
		}
		if offset > len(data) {
			return nil, fmt.Errorf("kiro header parse overflow")
		}
	}
	return headers, nil
}

func writeSSEEvent(w gin.ResponseWriter, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte("event: " + event + "\n")); err != nil {
		return err
	}
	if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
		return err
	}
	w.Flush()
	return nil
}

func readAllKiroFrames(body io.Reader) ([]*kiroFrame, error) {
	raw, err := io.ReadAll(io.LimitReader(body, kiroMaxBodySize+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > kiroMaxBodySize {
		return nil, fmt.Errorf("kiro response body exceeded limit %d", kiroMaxBodySize)
	}
	if len(raw) == 0 {
		return nil, errors.New("empty kiro response body")
	}
	frames := make([]*kiroFrame, 0)
	for len(raw) > 0 {
		frame, consumed, ok, err := parseKiroFrame(raw)
		if err != nil {
			return nil, err
		}
		if !ok {
			if len(raw) > 0 {
				return nil, fmt.Errorf("incomplete kiro frame: %w", io.ErrUnexpectedEOF)
			}
			break
		}
		frames = append(frames, frame)
		raw = raw[consumed:]
	}
	return frames, nil
}

func shouldKiroFailover(statusCode int) bool {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return true
	default:
		return statusCode >= 500
	}
}

func accountProxyURL(account *Account) string {
	if account != nil && account.Proxy != nil {
		return account.Proxy.URL()
	}
	return ""
}

func (s *KiroGatewayService) resolveTLSProfile(account *Account) *tlsfingerprint.Profile {
	if s == nil || s.tlsFPProfileSvc == nil {
		return nil
	}
	return resolveKiroTLSProfile(account, s.tlsFPProfileSvc)
}

func (s *KiroGatewayService) handleUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, body []byte) bool {
	if s == nil || s.rateLimitService == nil || account == nil {
		return false
	}
	return s.rateLimitService.HandleUpstreamError(ctx, account, statusCode, headers, body)
}

func (s *KiroGatewayService) handleProtocolError(ctx context.Context, c *gin.Context, account *Account, model string, isStream bool, err error) {
	if err == nil {
		return
	}
	body := []byte(err.Error())
	s.recordOpsProtocolError(c, account, err)
	if isStream && errors.Is(err, io.ErrUnexpectedEOF) && s != nil && s.rateLimitService != nil {
		s.rateLimitService.HandleStreamTimeout(ctx, account, model)
		return
	}
	s.handleUpstreamError(ctx, account, http.StatusBadGateway, http.Header{}, body)
}

func (s *KiroGatewayService) handleFrameFailure(ctx context.Context, c *gin.Context, account *Account, upstreamRequestID string, frame *kiroFrame, failureErr error, writeClientError bool) error {
	if failureErr == nil {
		return nil
	}
	statusCode := kiroFrameFailureStatusCode(frame)
	body := []byte(failureErr.Error())
	s.recordOpsFrameFailure(c, account, upstreamRequestID, statusCode, failureErr)
	s.handleUpstreamError(ctx, account, statusCode, http.Header{}, body)
	if shouldKiroFailover(statusCode) {
		return &UpstreamFailoverError{
			StatusCode:   statusCode,
			ResponseBody: body,
		}
	}
	if writeClientError && c != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": failureErr.Error()},
		})
	}
	return failureErr
}

func (s *KiroGatewayService) recordOpsRequestError(c *gin.Context, account *Account, upstreamURL string, err error) {
	if err == nil {
		return
	}
	s.recordOpsErrorEvent(c, account, 0, "", upstreamURL, "request_error", err.Error(), "")
}

func (s *KiroGatewayService) recordOpsHTTPError(c *gin.Context, account *Account, upstreamURL string, statusCode int, headers http.Header, body []byte) {
	detail := kiroErrorDetailFromBody(body)
	if detail == "" {
		detail = truncateString(strings.TrimSpace(string(body)), 2048)
	}
	requestID := strings.TrimSpace(headers.Get("x-amzn-requestid"))
	if requestID == "" {
		requestID = strings.TrimSpace(headers.Get("x-request-id"))
	}
	s.recordOpsErrorEvent(
		c,
		account,
		statusCode,
		requestID,
		upstreamURL,
		"http_error",
		kiroSafeHTTPStatusErrorMessage("Kiro upstream", statusCode, body),
		detail,
	)
}

func (s *KiroGatewayService) recordOpsProtocolError(c *gin.Context, account *Account, err error) {
	if err == nil {
		return
	}
	s.recordOpsErrorEvent(c, account, 0, "", "", "request_error", err.Error(), "")
}

func (s *KiroGatewayService) recordOpsFrameFailure(c *gin.Context, account *Account, upstreamRequestID string, statusCode int, failureErr error) {
	if failureErr == nil {
		return
	}
	s.recordOpsErrorEvent(c, account, statusCode, upstreamRequestID, "", "http_error", failureErr.Error(), "")
}

func (s *KiroGatewayService) recordOpsErrorEvent(
	c *gin.Context,
	account *Account,
	statusCode int,
	upstreamRequestID string,
	upstreamURL string,
	kind string,
	message string,
	detail string,
) {
	if c == nil || account == nil {
		return
	}
	message = sanitizeUpstreamErrorMessage(message)
	detail = strings.TrimSpace(detail)
	SetOpsUpstreamError(c, statusCode, message, detail)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: statusCode,
		UpstreamRequestID:  strings.TrimSpace(upstreamRequestID),
		UpstreamURL:        safeUpstreamURL(upstreamURL),
		Kind:               kind,
		Message:            message,
		Detail:             detail,
	})
}

func kiroFrameFailure(frame *kiroFrame) error {
	if !kiroFrameIsFailure(frame) {
		return nil
	}

	kind := "error"
	if kiroFrameIsException(frame) {
		kind = "exception"
	}

	message := kiroFrameFailureMessage(frame)
	if message == "" {
		return fmt.Errorf("kiro upstream returned %s frame", kind)
	}
	return fmt.Errorf("kiro upstream returned %s frame: %s", kind, message)
}

func kiroFrameFailureStatusCode(frame *kiroFrame) int {
	message := strings.ToLower(kiroFrameFailureMessage(frame))
	eventType := strings.ToLower(strings.TrimSpace(frame.EventType))
	messageType := strings.ToLower(strings.TrimSpace(frame.MessageType))
	combined := strings.TrimSpace(message + " " + eventType + " " + messageType)
	switch {
	case strings.Contains(combined, "forbidden"), strings.Contains(combined, "access denied"), strings.Contains(combined, "denied"):
		return http.StatusForbidden
	case strings.Contains(combined, "unauthor"), strings.Contains(combined, "auth"), strings.Contains(combined, "expired token"):
		return http.StatusUnauthorized
	case strings.Contains(combined, "rate"), strings.Contains(combined, "thrott"), strings.Contains(combined, "too many"):
		return http.StatusTooManyRequests
	case strings.Contains(combined, "overload"), strings.Contains(combined, "unavailable"), strings.Contains(combined, "timeout"), strings.Contains(combined, "internal"), strings.Contains(combined, "server"):
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadGateway
	}
}

func kiroFrameIsFailure(frame *kiroFrame) bool {
	return kiroFrameIsException(frame) ||
		strings.EqualFold(strings.TrimSpace(frame.MessageType), "error") ||
		strings.EqualFold(strings.TrimSpace(frame.EventType), "error")
}

func kiroFrameIsException(frame *kiroFrame) bool {
	if frame == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(frame.MessageType), "exception") ||
		strings.EqualFold(strings.TrimSpace(frame.EventType), "exception")
}

func kiroFrameFailureMessage(frame *kiroFrame) string {
	if frame == nil || frame.Payload == nil {
		return ""
	}

	for _, key := range []string{"message", "Message", "errorMessage", "error_message"} {
		if value := controlStringField(frame.Payload, key); value != "" {
			return value
		}
	}
	return ""
}

func startKiroStream(writer gin.ResponseWriter, msgID, model string, fakeCacheUsage kiropkg.FakeCacheUsage) error {
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	return writeSSEEvent(writer, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            msgID,
			"type":          "message",
			"role":          "assistant",
			"model":         model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":                fakeCacheUsage.InputTokens,
				"output_tokens":               0,
				"cache_creation_input_tokens": fakeCacheUsage.CacheCreationInputTokens,
				"cache_read_input_tokens":     fakeCacheUsage.CacheReadInputTokens,
			},
		},
	})
}

func closeOpenKiroBlocks(writer gin.ResponseWriter, textBlockOpen bool, textBlockIndex int, toolStates map[string]*kiroToolState) error {
	if textBlockOpen {
		if err := writeSSEEvent(writer, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": textBlockIndex,
		}); err != nil {
			return err
		}
	}
	for _, state := range toolStates {
		if state == nil || !state.Started || state.Stopped {
			continue
		}
		if err := writeSSEEvent(writer, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": state.BlockIndex,
		}); err != nil {
			return err
		}
		state.Stopped = true
	}
	return nil
}

func writeKiroStreamError(writer gin.ResponseWriter, message string) error {
	return writeSSEEvent(writer, "error", map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "api_error",
			"message": message,
		},
	})
}

func rawStringField(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	if value, ok := obj[key].(string); ok {
		return value
	}
	return ""
}

func controlStringField(obj map[string]any, key string) string {
	return strings.TrimSpace(rawStringField(obj, key))
}

func booleanField(obj map[string]any, key string) bool {
	if obj == nil {
		return false
	}
	if value, ok := obj[key].(bool); ok {
		return value
	}
	return false
}

func numberField(obj map[string]any, key string) float64 {
	if obj == nil {
		return 0
	}
	if value, ok := obj[key].(float64); ok {
		return value
	}
	return 0
}

func contextUsageToTokens(percentage float64) int {
	if percentage <= 0 {
		return 0
	}
	return int((percentage * 1000000.0) / 100.0)
}
