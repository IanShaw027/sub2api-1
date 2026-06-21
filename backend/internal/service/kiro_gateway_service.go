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
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/webfetch"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	"github.com/gin-gonic/gin"
	gocache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

const (
	kiroPreludeSize             = 12
	kiroMinMsgSize              = kiroPreludeSize + 4
	kiroMaxBodySize             = 16 << 20
	kiroSameAccountRetryDelay   = 3 * time.Second
	kiroSameAccountRetryMax     = 3
	kiroRetryExhaustedCooldown  = time.Minute
	kiroRetryExhaustedReasonKey = "kiro_429_retry_exhausted"

	kiroStandardContextBudgetTokens     = 180000
	kiroStandardContextPromoteThreshold = 180000
	kiroOneMillionContextBudgetTokens   = 900000
	kiroContextUsagePercentKey          = "kiro_context_usage_percentage"
	kiroShortOutputTokenThreshold       = 20

	// kiroTransportFailureCooldown：transport 层故障（非客户端取消）短期冷却时长，
	// 让调度层在该窗口内跳过出错账号，避免重复打到不可达上游。命中后会一并触发 failover。
	kiroTransportFailureCooldown      = 2 * time.Minute
	kiroTransportFailureReasonKeyword = "kiro_transport_failure"
	kiroAccountStateUpdateTimeout     = 5 * time.Second
)

var kiroFirstForwardableEventTimeout = 30 * time.Second

var (
	kiroShadowWebSearchExecutor = doWebSearch
	kiroShadowWebFetchExecutor  = func(ctx context.Context, account *Account, req webfetch.FetchRequest) *webfetch.FetchResult {
		return webfetch.NewFetcher().Fetch(ctx, req)
	}
)

type KiroGatewayService struct {
	httpUpstream      HTTPUpstream
	tokenProvider     *KiroTokenProvider
	rateLimitService  *RateLimitService
	tlsFPProfileSvc   *TLSFingerprintProfileService
	settingService    *SettingService
	channelService    *ChannelService
	fakeCache         *gocache.Cache
	fakeCacheMu       sync.Mutex
	fakeCacheStrategy string
	fakeCacheGen      uint64
}

type kiroPreparedRequestMeta struct {
	ForwardInputTokens    int
	ForwardBody           []byte
	ContextBudgetTokens   int
	PromotedContextWindow bool
	Compacted             bool
	DroppedMessages       int
	ToolCount             int
}

type kiroResponseTelemetry struct {
	FramesSeen             int
	AssistantChars         int
	NativeThinkingChars    int
	ToolUseCount           int
	CompletedToolUseCount  int
	PartialToolUseCount    int
	ContextUsagePercentage *float64
}

func NewKiroGatewayService(
	httpUpstream HTTPUpstream,
	tokenProvider *KiroTokenProvider,
	rateLimitService *RateLimitService,
	tlsFPProfileSvc *TLSFingerprintProfileService,
	settingService *SettingService,
	channelService *ChannelService,
) *KiroGatewayService {
	return &KiroGatewayService{
		httpUpstream:     httpUpstream,
		tokenProvider:    tokenProvider,
		rateLimitService: rateLimitService,
		tlsFPProfileSvc:  tlsFPProfileSvc,
		settingService:   settingService,
		channelService:   channelService,
		fakeCache:        gocache.New(kiropkg.DefaultFakeCacheTTL, time.Minute),
	}
}

func (s *KiroGatewayService) Forward(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest) (*ForwardResult, error) {
	if s.shouldEmulateWebSearch(ctx, account, parsed) {
		return s.handleWebSearchEmulation(ctx, c, account, parsed)
	}

	runtimeSettings := s.resolveKiroRuntimeSettings(ctx)

	converted, billedInputTokens, meta, err := s.validateAndConvertRequest(c, account, parsed, runtimeSettings)
	if err != nil {
		return nil, err
	}
	if c != nil && converted != nil {
		setOpsUpstreamRequestBody(c, converted.Body)
		SetOpsUpstreamModel(c, converted.Model)
	}
	logKiroPreparedRequest(ctx, account, parsed, converted, billedInputTokens, meta)

	accessToken, err := s.resolveAccessToken(ctx, account)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": "Failed to get Kiro access token"},
		})
		return nil, err
	}

	fakeCachePlan, fakeCacheHit := s.prepareFakeCachePlan(account, parsed, meta, runtimeSettings)
	logKiroFakeCachePlan(ctx, account, parsed, fakeCachePlan, fakeCacheHit)
	req, err := s.buildRequest(ctx, account, converted.Body, accessToken, runtimeSettings)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": "Failed to build Kiro upstream request"},
		})
		return nil, err
	}

	s.emitGatewayDebugUpstreamRequest(c, account, req, converted.Body, 1)

	start := time.Now()
	resp, err := s.httpUpstream.DoWithTLS(req, accountProxyURL(account), account.ID, account.Concurrency, s.resolveTLSProfile(account))
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(start).Milliseconds())
	if err != nil {
		return nil, s.handleKiroTransportError(ctx, c, account, req.URL.String(), err)
	}
	needsDeferredClose := true
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if retryResp, retryErr := s.retryInvalidTokenResponse(ctx, account, req, resp.StatusCode, body, runtimeSettings); retryResp != nil {
			needsDeferredClose = false
			_ = resp.Body.Close()
			resp = retryResp
		} else {
			_ = retryErr
			resp.Body = io.NopCloser(bytes.NewReader(body))
		}
	}
	if needsDeferredClose {
		defer func() { _ = resp.Body.Close() }()
	} else if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		effectiveStatusCode := kiroSchedulingStatusCode(resp.StatusCode, body)
		s.handleUpstreamError(ctx, account, effectiveStatusCode, resp.Header, body)
		s.recordOpsHTTPError(c, account, req.URL.String(), resp.StatusCode, resp.Header, body)
		if shouldKiroFailover(effectiveStatusCode) {
			failoverErr := &UpstreamFailoverError{
				StatusCode:      effectiveStatusCode,
				ResponseBody:    body,
				ResponseHeaders: resp.Header.Clone(),
			}
			if effectiveStatusCode == http.StatusTooManyRequests && shouldKiroRetrySameAccount(resp.StatusCode, resp.Header, body) {
				failoverErr.RetryableOnSameAccount = true
				failoverErr.SameAccountRetryDelay = kiroSameAccountRetryDelay
				failoverErr.SameAccountRetryMax = kiroSameAccountRetryMax
				failoverErr.RetryExhaustedCooldown = kiroRetryExhaustedCooldown
				failoverErr.RetryExhaustedReason = kiroRetryExhaustedReasonKey
			}
			return nil, failoverErr
		}
		upstreamMessage := kiroSafeHTTPStatusErrorMessage("Kiro upstream", resp.StatusCode, body)
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": upstreamMessage},
		})
		return nil, fmt.Errorf("%s", upstreamMessage)
	}
	if parsed.Stream {
		return s.forwardStream(ctx, c, account, resp, parsed, converted, billedInputTokens, start, fakeCachePlan, fakeCacheHit, runtimeSettings, "")
	}
	return s.forwardNonStream(ctx, c, account, resp, parsed, converted, billedInputTokens, start, fakeCachePlan, fakeCacheHit, runtimeSettings, "")
}

func (s *KiroGatewayService) ForwardCountTokens(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest) error {
	_, billedInputTokens, _, err := s.validateAndConvertRequest(c, account, parsed, s.resolveKiroRuntimeSettings(ctx))
	if err != nil {
		return err
	}
	c.JSON(http.StatusOK, gin.H{"input_tokens": billedInputTokens})
	return nil
}

// Kiro does not proxy an upstream count_tokens API. Both the count-tokens
// endpoint and forwarded usage metadata intentionally share the same local
// tiktoken-backed estimator so callers see one stable contract.
func estimateKiroInputTokens(body []byte) int {
	return kiropkg.EstimateInputTokens(body)
}

func (s *KiroGatewayService) validateAndConvertRequest(c *gin.Context, account *Account, parsed *ParsedRequest, runtimeSettings *KiroRuntimeSettings) (*kiropkg.ConvertResult, int, *kiroPreparedRequestMeta, error) {
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	converted, billedInputTokens, meta, err := prepareKiroConvertedRequestWithRoutingWithMeta(ctx, s.settingService, account, parsed, runtimeSettings)
	if err != nil {
		writeKiroInvalidRequest(c, err)
		return nil, 0, nil, err
	}
	return converted, billedInputTokens, meta, nil
}

func prepareKiroConvertedRequest(account *Account, parsed *ParsedRequest, runtimeSettings *KiroRuntimeSettings) (*kiropkg.ConvertResult, int, error) {
	converted, billedInputTokens, _, err := prepareKiroConvertedRequestWithMeta(account, parsed, runtimeSettings)
	return converted, billedInputTokens, err
}

func prepareKiroConvertedRequestWithMeta(account *Account, parsed *ParsedRequest, runtimeSettings *KiroRuntimeSettings) (*kiropkg.ConvertResult, int, *kiroPreparedRequestMeta, error) {
	return prepareKiroConvertedRequestWithRoutingWithMeta(context.Background(), nil, account, parsed, runtimeSettings)
}

func prepareKiroConvertedRequestWithRoutingWithMeta(ctx context.Context, settingService *SettingService, account *Account, parsed *ParsedRequest, runtimeSettings *KiroRuntimeSettings) (*kiropkg.ConvertResult, int, *kiroPreparedRequestMeta, error) {
	if account == nil || parsed == nil {
		err := fmt.Errorf("invalid kiro request args")
		return nil, 0, nil, err
	}

	requestedModel, err := resolveKiroRequestedModelForRequestWithRouting(ctx, settingService, account, parsed, runtimeSettings)
	if err != nil {
		return nil, 0, nil, err
	}
	meta := &kiroPreparedRequestMeta{}

	rawBody := []byte(nil)
	if parsed.Body != nil {
		rawBody = parsed.Body.Bytes()
	}

	billedInputTokens := estimateKiroInputTokens(rawBody)
	meta.ForwardInputTokens = billedInputTokens
	if billedInputTokens >= kiroStandardContextPromoteThreshold && kiropkg.SupportsOneMillionContextModel(requestedModel) {
		meta.PromotedContextWindow = true
	}

	forwardBody := rawBody
	contextBudget := kiroContextBudgetTokensForModel(requestedModel)
	meta.ContextBudgetTokens = contextBudget
	if billedInputTokens > contextBudget {
		compactedBody, droppedMessages, compacted, compactErr := kiropkg.CompactAnthropicRequestToTokenBudget(rawBody, contextBudget)
		if compactErr != nil {
			return nil, 0, nil, compactErr
		}
		if compacted {
			forwardBody = compactedBody
			meta.Compacted = true
			meta.DroppedMessages = droppedMessages
		}
		if compactedTokens := estimateKiroInputTokens(forwardBody); compactedTokens > contextBudget {
			return nil, 0, nil, fmt.Errorf("kiro request exceeds context budget after compaction (%d > %d tokens)", compactedTokens, contextBudget)
		}
	}
	meta.ForwardInputTokens = estimateKiroInputTokens(forwardBody)
	meta.ForwardBody = forwardBody

	forwardBody = normalizeKiroShadowToolHistory(forwardBody)
	meta.ForwardBody = forwardBody

	forwardBody = patchKiroThinkingForModel(forwardBody, requestedModel, strings.TrimSpace(parsed.OutputEffort))

	converted, err := kiropkg.ConvertAnthropicRequestWithModel(forwardBody, requestedModel)
	if err != nil {
		return nil, 0, nil, err
	}
	meta.ToolCount = len(converted.ToolNameMap)
	return converted, billedInputTokens, meta, nil
}

func patchKiroThinkingForModel(forwardBody []byte, requestedModel string, outputEffort string) []byte {
	thinkingType := extractJSONStringField(forwardBody, "thinking", "type")
	if thinkingType == "" {
		return forwardBody
	}

	needsDowngrade := thinkingType == "enabled" && !kiropkg.SupportsExtendedThinking(requestedModel)
	needsEffortOverride := outputEffort != "" && (thinkingType == "adaptive" || needsDowngrade)

	if !needsDowngrade && !needsEffortOverride {
		return forwardBody
	}

	var body map[string]any
	if err := json.Unmarshal(forwardBody, &body); err != nil {
		return forwardBody
	}
	thinking, _ := body["thinking"].(map[string]any)
	if thinking == nil {
		return forwardBody
	}

	if needsDowngrade {
		thinking["type"] = "adaptive"
		if outputEffort != "" {
			thinking["thinking_effort"] = outputEffort
		} else {
			budget, _ := thinking["budget_tokens"].(float64)
			thinking["thinking_effort"] = budgetTokensToEffort(int(budget))
		}
		delete(thinking, "budget_tokens")
		if thinking["display"] == nil {
			thinking["display"] = "summarized"
		}
	} else if needsEffortOverride {
		thinking["thinking_effort"] = outputEffort
	}

	body["thinking"] = thinking
	patched, err := json.Marshal(body)
	if err != nil {
		return forwardBody
	}
	return patched
}

func renderKiroThinkingSimulation(parsed *ParsedRequest, converted *kiropkg.ConvertResult, runtimeSettings *KiroRuntimeSettings) string {
	if parsed == nil || runtimeSettings == nil {
		return ""
	}
	if normalizeKiroThinkingMode(runtimeSettings.ThinkingMode) != KiroThinkingModeSimulate {
		return ""
	}
	outputEffort := strings.ToLower(strings.TrimSpace(parsed.OutputEffort))
	if outputEffort == "" {
		return ""
	}
	threshold := strings.ToLower(strings.TrimSpace(runtimeSettings.ThinkingEffortThreshold))
	if threshold != "" && !kiroThinkingEffortAtLeast(outputEffort, threshold) {
		return ""
	}
	requestedModel := strings.TrimSpace(parsed.Model)
	upstreamModel := requestedModel
	if converted != nil && strings.TrimSpace(converted.Model) != "" {
		upstreamModel = strings.TrimSpace(converted.Model)
	}
	template := strings.TrimSpace(runtimeSettings.ThinkingSimulationTemplate)
	if template == "" {
		template = "think {effort} {model} {upstream_model} {detail}"
	}
	detail := "failure modes"
	rendered := strings.NewReplacer(
		"{effort}", outputEffort,
		"{model}", requestedModel,
		"{upstream_model}", upstreamModel,
		"{detail}", detail,
	).Replace(template)
	return strings.TrimSpace(rendered)
}

func normalizeKiroThinkingMode(mode KiroThinkingMode) KiroThinkingMode {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case string(KiroThinkingModeSimulate), string(KiroThinkingModeModelAndSimulate):
		return KiroThinkingModeSimulate
	default:
		return KiroThinkingModeDisabled
	}
}

func kiroThinkingEffortAtLeast(outputEffort, threshold string) bool {
	rank := func(value string) int {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "low":
			return 1
		case "medium":
			return 2
		case "high":
			return 3
		case "max":
			return 4
		default:
			return 0
		}
	}
	return rank(outputEffort) >= rank(threshold)
}

func budgetTokensToEffort(budget int) string {
	switch {
	case budget <= 4096:
		return "low"
	case budget <= 16384:
		return "medium"
	case budget <= 65536:
		return "high"
	default:
		return "max"
	}
}

func extractJSONStringField(body []byte, keys ...string) string {
	var current any
	if err := json.Unmarshal(body, &current); err != nil {
		return ""
	}
	for _, key := range keys {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = m[key]
	}
	s, _ := current.(string)
	return s
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
		if strippedModel, hadVariant := stripKiroModelVariantSuffixes(strings.TrimSpace(mappedModel)); hadVariant {
			return strippedModel, nil
		}
		return mappedModel, nil
	}
	if strippedModel, hadVariant := stripKiroModelVariantSuffixes(requestedModel); hadVariant {
		return strippedModel, nil
	}
	return requestedModel, nil
}

func resolveKiroRequestedModelForRequest(account *Account, parsed *ParsedRequest, runtimeSettings *KiroRuntimeSettings) (string, error) {
	return resolveKiroRequestedModelForRequestWithRouting(context.Background(), nil, account, parsed, runtimeSettings)
}

func resolveKiroRequestedModelForRequestWithRouting(ctx context.Context, settingService *SettingService, account *Account, parsed *ParsedRequest, runtimeSettings *KiroRuntimeSettings) (string, error) {
	if parsed == nil {
		return "", fmt.Errorf("invalid kiro request args")
	}
	return resolveKiroRequestedModelWithRouting(ctx, settingService, account, parsed.Model)
}

func resolveKiroRequestedModelWithRouting(ctx context.Context, settingService *SettingService, account *Account, requestedModel string) (string, error) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return "", nil
	}
	if !hasModelRoutingConfigForAccount(ctx, settingService, account) {
		return resolveKiroRequestedModel(account, requestedModel)
	}
	routing := ResolveEffectiveModelRouting(ctx, settingService, account, requestedModel, false)
	if !routing.Supported {
		return "", fmt.Errorf("unsupported kiro model: %s", requestedModel)
	}
	effectiveModel := strings.TrimSpace(routing.Model)
	if effectiveModel == "" {
		effectiveModel = requestedModel
	}
	if kiropkg.MapModel(effectiveModel) == "" {
		return "", fmt.Errorf("unsupported kiro model: %s", requestedModel)
	}
	if strippedModel, hadVariant := stripKiroModelVariantSuffixes(effectiveModel); hadVariant {
		return strippedModel, nil
	}
	return effectiveModel, nil
}

// isKiroFreeAccount 判断 Kiro 账号是否为 Free 订阅。
// subscription_type 为空或含 "free" 时视为 Free。
func isKiroFreeAccount(account *Account) bool {
	if account == nil {
		return true
	}
	subscriptionType := strings.ToLower(strings.TrimSpace(account.GetCredential("subscription_type")))
	return subscriptionType == "" || strings.Contains(subscriptionType, "free")
}

func kiroContextBudgetTokensForModel(model string) int {
	if kiropkg.SupportsOneMillionContextModel(model) {
		return kiroOneMillionContextBudgetTokens
	}
	return kiroStandardContextBudgetTokens
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
		if s.settingService.settingRepo == nil {
			return DefaultKiroRuntimeSettings()
		}
		return s.settingService.GetKiroRuntimeSettings(ctx)
	}
	return DefaultKiroRuntimeSettings()
}

func (s *KiroGatewayService) shouldEmulateWebSearch(ctx context.Context, account *Account, parsed *ParsedRequest) bool {
	if s == nil || account == nil || parsed == nil || s.settingService == nil {
		return false
	}
	if getWebSearchManager() == nil || !isOnlyWebSearchToolInBody(parsed.Body.Bytes()) {
		return false
	}
	if !s.settingService.IsWebSearchEmulationEnabled(ctx) {
		return false
	}
	mode := account.GetWebSearchEmulationMode()
	switch mode {
	case WebSearchModeEnabled:
		return true
	case WebSearchModeDisabled:
		return false
	default:
		if parsed.GroupID == nil || s.channelService == nil {
			return false
		}
		ch, err := s.channelService.GetChannelForGroup(ctx, *parsed.GroupID)
		if err != nil || ch == nil {
			return false
		}
		return ch.IsWebSearchEmulationEnabled(account.Platform)
	}
}

func (s *KiroGatewayService) handleWebSearchEmulation(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *ParsedRequest,
) (*ForwardResult, error) {
	startTime := time.Now()
	query := extractSearchQueryFromBody(parsed.Body.Bytes())
	if query == "" {
		return nil, fmt.Errorf("web search emulation: no query found in messages")
	}
	resp, _, err := doWebSearch(ctx, account, query)
	if err != nil {
		if errors.Is(err, websearch.ErrProxyUnavailable) {
			return nil, &UpstreamFailoverError{
				StatusCode:   http.StatusBadGateway,
				ResponseBody: []byte(err.Error()),
			}
		}
		return nil, err
	}
	if parsed != nil && parsed.OnUpstreamAccepted != nil {
		parsed.OnUpstreamAccepted()
	}

	model := parsed.Model
	if model == "" {
		model = defaultWebSearchModel
	}
	if parsed.Stream {
		return writeWebSearchStreamResponse(c, query, resp, model, startTime)
	}
	return writeWebSearchNonStreamResponse(c, query, resp, model, startTime)
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
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
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

func (s *KiroGatewayService) retryInvalidTokenResponse(
	ctx context.Context,
	account *Account,
	req *http.Request,
	statusCode int,
	body []byte,
	runtimeSettings *KiroRuntimeSettings,
) (*http.Response, error) {
	if account == nil || account.Platform != PlatformKiro || account.Type != AccountTypeOAuth {
		return nil, nil
	}
	if statusCode != http.StatusUnauthorized && statusCode != http.StatusForbidden {
		return nil, nil
	}
	if !isKiroInvalidTokenResponse(body) {
		return nil, nil
	}
	kiroLogger(ctx, account).Warn(
		"kiro.invalid_token_retry_start",
		zap.Int("status_code", statusCode),
		zap.String("error_detail", truncateString(strings.TrimSpace(kiroErrorDetailFromBody(body)), 256)),
	)
	refreshedAccount, err := s.refreshKiroAccountForRetry(ctx, account)
	if err != nil {
		kiroLogger(ctx, account).Warn("kiro.invalid_token_retry_failed", zap.Int("status_code", statusCode), zap.Error(err))
		return nil, err
	}
	accessToken := refreshedAccount.GetCredential("access_token")
	if strings.TrimSpace(accessToken) == "" {
		err := errors.New("access_token not found after refresh")
		kiroLogger(ctx, refreshedAccount).Warn("kiro.invalid_token_retry_failed", zap.Int("status_code", statusCode), zap.Error(err))
		return nil, err
	}
	requestBody, err := bodyFromGetBody(req)
	if err != nil {
		kiroLogger(ctx, refreshedAccount).Warn("kiro.invalid_token_retry_failed", zap.Int("status_code", statusCode), zap.Error(err))
		return nil, err
	}
	retryReq, err := s.buildRequest(ctx, refreshedAccount, requestBody, accessToken, runtimeSettings)
	if err != nil {
		kiroLogger(ctx, refreshedAccount).Warn("kiro.invalid_token_retry_failed", zap.Int("status_code", statusCode), zap.Error(err))
		return nil, err
	}
	retryResp, err := s.httpUpstream.DoWithTLS(retryReq, accountProxyURL(refreshedAccount), refreshedAccount.ID, refreshedAccount.Concurrency, s.resolveTLSProfile(refreshedAccount))
	if err != nil {
		kiroLogger(ctx, refreshedAccount).Warn("kiro.invalid_token_retry_failed", zap.Int("status_code", statusCode), zap.Error(err))
		return nil, err
	}
	kiroLogger(ctx, refreshedAccount).Info(
		"kiro.invalid_token_retry_complete",
		zap.Int("status_code", statusCode),
		zap.Int("retry_status_code", retryResp.StatusCode),
	)
	return retryResp, nil
}

func (s *KiroGatewayService) refreshKiroAccountForRetry(ctx context.Context, account *Account) (*Account, error) {
	if s == nil || s.tokenProvider == nil {
		return nil, errors.New("kiro token provider is not configured")
	}
	return s.tokenProvider.RefreshAccount(ctx, account)
}

func isKiroInvalidTokenResponse(body []byte) bool {
	detail := strings.ToLower(strings.TrimSpace(kiroErrorDetailFromBody(body)))
	if detail == "" {
		detail = strings.ToLower(strings.TrimSpace(string(body)))
	}
	return strings.Contains(detail, "unauthor") ||
		strings.Contains(detail, "invalid token") ||
		strings.Contains(detail, "expired token") ||
		(strings.Contains(detail, "token") && strings.Contains(detail, "expired")) ||
		strings.Contains(detail, "invalid bearer") ||
		(strings.Contains(detail, "invalid credential") && strings.Contains(detail, "token"))
}

func bodyFromGetBody(req *http.Request) ([]byte, error) {
	if req == nil || req.GetBody == nil {
		return nil, errors.New("request body is not replayable")
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	return io.ReadAll(body)
}

func (s *KiroGatewayService) forwardNonStream(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, parsed *ParsedRequest, converted *kiropkg.ConvertResult, inputTokens int, start time.Time, fakeCachePlan *kiropkg.FakeCachePlan, fakeCacheHit kiropkg.FakeCacheHitState, runtimeSettings *KiroRuntimeSettings, _ string) (*ForwardResult, error) {
	debugAggregator := BeginKiroFrameAggregator(s.settingService, c)
	frames, err := readAllKiroFrames(resp.Body)
	if err != nil {
		debugAggregator.Finalize("upstream_response_body", map[string]any{
			"component":      "gateway_debug_timeline",
			"platform":       account.Platform,
			"account_id":     account.ID,
			"stream":         false,
			"frames_seen":    0,
			"upstream_model": converted.Model,
			"decode_error":   err.Error(),
		})
		s.handleProtocolError(ctx, c, account, parsed.Model, false, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"type":  "error",
			"error": gin.H{"type": "api_error", "message": "Failed to decode Kiro response"},
		})
		return nil, err
	}
	for _, frame := range frames {
		debugAggregator.Append(frame, rawStringField(frame.Payload, "content"))
	}
	defer func() {
		debugAggregator.Finalize("upstream_response_body", map[string]any{
			"component":      "gateway_debug_timeline",
			"platform":       account.Platform,
			"account_id":     account.ID,
			"stream":         false,
			"frames_seen":    len(frames),
			"upstream_model": converted.Model,
		})
	}()

	assistantContentBuilder := strings.Builder{}
	assistantTrailingHoldback := ""
	flushAssistantTrailingHoldback := func() {
		if assistantTrailingHoldback == "" {
			return
		}
		_, _ = assistantContentBuilder.WriteString(assistantTrailingHoldback)
		assistantTrailingHoldback = ""
	}
	appendAssistantContent := func(content string) {
		if content == "" {
			return
		}
		if assistantTrailingHoldback != "" {
			flushAssistantTrailingHoldback()
		}
		if kiropkg.IsPlaceholderFragment(content) {
			assistantTrailingHoldback = content
			return
		}
		_, _ = assistantContentBuilder.WriteString(content)
	}
	reasoningTextBuilder := strings.Builder{}
	reasoningSignatureBuilder := strings.Builder{}
	toolOutputBuilder := strings.Builder{}
	toolUses := make([]map[string]any, 0)
	toolBuffers := make(map[string]*kiroToolState)
	toolOrder := make([]string, 0)
	toolNames := make([]string, 0)
	stopReason := "end_turn"
	hasVisibleOutput := false
	var lastContextUsagePercentage *float64

	for _, frame := range frames {
		if failureErr := kiroFrameFailure(frame); failureErr != nil {
			return nil, s.handleFrameFailure(ctx, c, account, resp.Header.Get("x-amzn-requestid"), frame, failureErr, true)
		}

		switch frame.EventType {
		case "assistantResponseEvent":
			if content := rawStringField(frame.Payload, "content"); content != "" {
				appendAssistantContent(content)
			}
		case "reasoningContentEvent":
			if text := rawStringField(frame.Payload, "text"); text != "" {
				_, _ = reasoningTextBuilder.WriteString(text)
			}
			if signature := rawStringField(frame.Payload, "signature"); signature != "" {
				_, _ = reasoningSignatureBuilder.WriteString(signature)
			}
		case "contextUsageEvent":
			if usagePercent, ok := numericField(frame.Payload, "contextUsagePercentage"); ok {
				lastContextUsagePercentage = &usagePercent
				setKiroContextUsagePercentage(c, usagePercent)
				if reason := kiroStopReasonFromContextUsage(usagePercent); reason != "" {
					stopReason = reason
				}
			}
		case "toolUseEvent":
			assistantTrailingHoldback = ""
			state := ensureKiroToolState(toolBuffers, controlStringField(frame.Payload, "toolUseId"), controlStringField(frame.Payload, "name"))
			toolOrder = appendKiroToolStateOrder(toolOrder, state)
			inputChunk := rawStringField(frame.Payload, "input")
			_, _ = state.InputBuilder.WriteString(inputChunk)
			_, _ = toolOutputBuilder.WriteString(inputChunk)
			if booleanField(frame.Payload, "stop") {
				state.Stopped = true
				_, _ = toolOutputBuilder.WriteString(strings.TrimSpace(state.Name))
			}
		}
	}
	flushAssistantTrailingHoldback()
	visibleToolUses, completedToolUses, partialToolUses := kiroVisibleToolStateCounts(toolBuffers, toolOrder)
	shadowToolBlocks, shadowHandledIDs, shadowToolNames, shadowToolOutput, shadowExecuted, shadowErr := s.executeKiroShadowTools(ctx, account, converted, toolBuffers, toolOrder)
	if shadowErr != nil {
		return nil, shadowErr
	}
	shadowContent := make([]map[string]any, 0, len(shadowToolBlocks)*2)
	for _, toolUseID := range toolOrder {
		if _, handled := shadowHandledIDs[toolUseID]; handled {
			if shadowBlocks, ok := shadowToolBlocks[toolUseID]; ok {
				shadowContent = append(shadowContent, shadowBlocks...)
				hasVisibleOutput = true
			}
			continue
		}
		state := toolBuffers[toolUseID]
		toolUse, ok := buildKiroToolUseBlock(state, converted)
		if !ok {
			continue
		}
		toolUses = append(toolUses, toolUse)
		if name := strings.TrimSpace(kiroVisibleToolName(converted, state.Name)); name != "" {
			toolNames = append(toolNames, name)
		}
		hasVisibleOutput = true
	}
	toolNames = append(toolNames, shadowToolNames...)
	if shadowExecuted {
		if stopReason == "end_turn" || stopReason == "tool_use" {
			stopReason = "pause_turn"
		}
	} else if len(toolUses) > 0 && stopReason == "end_turn" {
		stopReason = "tool_use"
	}
	content := make([]map[string]any, 0, 2+len(toolUses))
	reasoningThinkingText := reasoningTextBuilder.String()
	reasoningThinkingSignature := reasoningSignatureBuilder.String()
	if strings.TrimSpace(reasoningThinkingText) != "" {
		block := map[string]any{"type": "thinking", "thinking": reasoningThinkingText}
		if reasoningThinkingSignature != "" {
			block["signature"] = reasoningThinkingSignature
		}
		content = append(content, block)
		hasVisibleOutput = true
	}
	assistantText := kiropkg.StripToolTurnPlaceholders(assistantContentBuilder.String())
	if assistantText != "" && (stopReason == "tool_use" || stopReason == "pause_turn") {
		assistantText = kiropkg.StripTrailingPlaceholderFragment(assistantText)
	}
	content, textOutput, nativeThinkingOutput := appendKiroNativeContentBlocks(assistantText, content)
	if textOutput != "" || nativeThinkingOutput != "" {
		hasVisibleOutput = true
	}
	if !hasVisibleOutput {
		emptyErr := errors.New("kiro response contained no assistant output")
		logKiroResponseAnomaly(ctx, account, parsed, false, "empty_output", emptyErr, 0, len(toolNames), nil)
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

	content = append(content, shadowContent...)
	content = append(content, toolUses...)
	thinkingText := nativeThinkingOutput + reasoningThinkingText
	outputTokens := estimateKiroOutputTokens(textOutput+thinkingText, toolOutputBuilder.String()+shadowToolOutput)
	thinkingTokens := estimateKiroOutputTokens(thinkingText, "")
	telemetry := &kiroResponseTelemetry{
		FramesSeen:             len(frames),
		AssistantChars:         len(textOutput),
		NativeThinkingChars:    len(nativeThinkingOutput) + len(reasoningThinkingText),
		ToolUseCount:           visibleToolUses,
		CompletedToolUseCount:  completedToolUses,
		PartialToolUseCount:    partialToolUses,
		ContextUsagePercentage: lastContextUsagePercentage,
	}
	if telemetry.PartialToolUseCount > 0 && telemetry.CompletedToolUseCount == 0 {
		incompleteErr := errors.New("kiro response completed with incomplete tool_use output")
		logKiroResponseAnomaly(ctx, account, parsed, false, "incomplete_tool_use_completed", incompleteErr, telemetry.FramesSeen, telemetry.ToolUseCount, telemetry.ContextUsagePercentage)
		if c != nil && account != nil {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:          account.Platform,
				AccountID:         account.ID,
				AccountName:       account.Name,
				UpstreamRequestID: strings.TrimSpace(resp.Header.Get("x-amzn-requestid")),
				Kind:              "response_anomaly",
				Message:           "kiro completed with anomalies: incomplete_tool_use_completed",
				Detail:            telemetry.opsDetail(stopReason, outputTokens, telemetry.shortOutput(outputTokens), []string{"incomplete_tool_use_completed"}),
			})
		}
		return nil, &UpstreamFailoverError{
			StatusCode:             http.StatusBadGateway,
			ResponseBody:           []byte(kiroIncompleteToolUseClientMessage()),
			RetryableOnSameAccount: true,
		}
	}
	fakeCacheUsage := resolveKiroFakeCacheUsage(fakeCachePlan, fakeCacheHit, inputTokens, runtimeSettings)
	inputTokens = fakeCacheUsage.InputTokens
	s.commitFakeCachePlan(fakeCachePlan, runtimeSettings)

	c.JSON(http.StatusOK, gin.H{
		"id":                 "msg_" + strings.ReplaceAll(generateRequestID(), "-", ""),
		"type":               "message",
		"role":               "assistant",
		"content":            content,
		"model":              parsed.Model,
		"stop_reason":        stopReason,
		"stop_sequence":      nil,
		"stop_details":       nil,
		"usage":              kiroAnthropicUsageForDelta(inputTokens, outputTokens, fakeCacheUsage, thinkingTokens),
		"context_management": gin.H{"applied_edits": []any{}},
	})

	result := &ForwardResult{
		RequestID:     resp.Header.Get("x-amzn-requestid"),
		Model:         parsed.Model,
		UpstreamModel: converted.Model,
		Stream:        false,
		Duration:      time.Since(start),
		Usage: ClaudeUsage{
			InputTokens:              inputTokens,
			OutputTokens:             outputTokens,
			CacheCreationInputTokens: fakeCacheUsage.CacheCreationInputTokens,
			CacheCreation5mTokens:    fakeCacheUsage.CacheCreationInputTokens,
			CacheReadInputTokens:     fakeCacheUsage.CacheReadInputTokens,
		},
	}
	s.recordKiroSuccessfulAnomalies(ctx, c, account, parsed, result.RequestID, false, outputTokens, stopReason, telemetry)
	logKiroRequestCompleted(ctx, account, parsed, result.RequestID, result.UpstreamModel, false, result.Duration, nil, inputTokens, outputTokens, fakeCacheUsage.CacheCreationInputTokens, fakeCacheUsage.CacheReadInputTokens, stopReason, toolNames, telemetry)
	return result, nil
}

func (s *KiroGatewayService) forwardStream(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, parsed *ParsedRequest, converted *kiropkg.ConvertResult, inputTokens int, start time.Time, fakeCachePlan *kiropkg.FakeCachePlan, fakeCacheHit kiropkg.FakeCacheHitState, runtimeSettings *KiroRuntimeSettings, _ string) (*ForwardResult, error) {
	writer := c.Writer
	msgID := "msg_" + strings.ReplaceAll(generateRequestID(), "-", "")
	reader := bufio.NewReader(resp.Body)
	buffer := make([]byte, 0, 64*1024)
	streamStarted := false
	var firstForwardableStarted atomic.Bool
	var firstForwardableTimeoutTriggered atomic.Bool
	var firstForwardableWatchdog *time.Timer
	stopFirstForwardableWatchdog := func() {
		if firstForwardableWatchdog == nil {
			return
		}
		firstForwardableWatchdog.Stop()
		firstForwardableWatchdog = nil
	}
	if kiroFirstForwardableEventTimeout > 0 {
		firstForwardableWatchdog = time.AfterFunc(kiroFirstForwardableEventTimeout, func() {
			if firstForwardableStarted.Load() {
				return
			}
			firstForwardableTimeoutTriggered.Store(true)
			_ = resp.Body.Close()
		})
		defer stopFirstForwardableWatchdog()
	}
	textBlockOpen := false
	textBlockIndex := -1
	thinkingBlockOpen := false
	thinkingBlockIndex := -1
	reasoningThinkingActive := false
	nextBlockIndex := 0
	toolStates := make(map[string]*kiroToolState)
	var firstTokenMs *int
	var textOutputBuilder strings.Builder
	var nativeThinkingBuilder strings.Builder
	var toolOutputBuilder strings.Builder
	stopReason := "end_turn"
	framesSeen := 0
	completedToolUses := 0
	completedShadowToolUses := 0
	toolOrder := make([]string, 0)
	var toolNames []string
	normalToolSeen := false
	shadowToolSeen := false
	shadowMaxUsesLimit := 0
	var lastContextUsagePercentage *float64
	nativeThinkingBuffer := ""
	nativeThinkingExtracted := false
	stripThinkingLeadingNewline := false
	// Placeholder suppression: the gateway pads tool-only turns sent upstream
	// with a fixed phrase ("I will call the requested tools."); Kiro sometimes
	// echoes it back verbatim as the entire assistant text. We hold back leading
	// text that could be that placeholder until we can confirm it is real output,
	// so a spurious placeholder line never reaches the client. Once any real text
	// has been emitted, suppression is disabled for the rest of the stream.
	placeholderPending := ""
	placeholderResolved := false
	// Trailing holdback: after real text has been emitted, short fragments that
	// look like placeholder echoes (e.g. "call") are held back until the next
	// event confirms whether they are genuine text or pre-tool-use noise.
	trailingHoldback := ""
	debugAggregator := BeginKiroFrameAggregator(s.settingService, c)
	defer func() {
		debugAggregator.Finalize("upstream_response_body", map[string]any{
			"component":      "gateway_debug_timeline",
			"platform":       account.Platform,
			"account_id":     account.ID,
			"stream":         true,
			"frames_seen":    framesSeen,
			"upstream_model": converted.Model,
		})
	}()
	markFirstToken := func() {
		if firstTokenMs != nil {
			return
		}
		v := int(time.Since(start).Milliseconds())
		firstTokenMs = &v
	}
	startStream := func(initialInputTokens int) error {
		firstForwardableStarted.Store(true)
		stopFirstForwardableWatchdog()
		if parsed.OnUpstreamAccepted != nil {
			parsed.OnUpstreamAccepted()
		}
		if err := startKiroStream(writer, msgID, parsed.Model, resolveKiroFakeCacheUsage(fakeCachePlan, fakeCacheHit, initialInputTokens, runtimeSettings)); err != nil {
			return err
		}
		streamStarted = true
		markFirstToken()
		return nil
	}
	// flushPendingPlaceholder is forward-declared so closeTextBlock can drain the
	// placeholder-suppression buffer before closing the text block. Assigned once
	// emitTextDeltaRaw is in scope.
	var flushPendingPlaceholder func() error
	closeTextBlock := func() error {
		if flushPendingPlaceholder != nil {
			if err := flushPendingPlaceholder(); err != nil {
				return err
			}
		}
		if !textBlockOpen {
			return nil
		}
		if err := writeSSEEvent(writer, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": textBlockIndex,
		}); err != nil {
			return err
		}
		textBlockOpen = false
		textBlockIndex = -1
		return nil
	}
	ensureTextBlock := func() error {
		if textBlockOpen {
			return nil
		}
		textBlockIndex = nextBlockIndex
		nextBlockIndex++
		textBlockOpen = true
		return writeSSEEvent(writer, "content_block_start", map[string]any{
			"type":  "content_block_start",
			"index": textBlockIndex,
			"content_block": map[string]any{
				"type": "text",
				"text": "",
			},
		})
	}
	emitTextDeltaRaw := func(text string) error {
		if text == "" {
			return nil
		}
		if !streamStarted {
			if err := startStream(inputTokens); err != nil {
				return err
			}
		}
		if err := ensureTextBlock(); err != nil {
			return err
		}
		_, _ = textOutputBuilder.WriteString(text)
		return writeSSEEvent(writer, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": textBlockIndex,
			"delta": map[string]any{
				"type": "text_delta",
				"text": text,
			},
		})
	}
	// emitTextDelta wraps the raw emitter with leading-placeholder suppression.
	// Until real text is confirmed, candidate placeholder text is buffered rather
	// than streamed; an exact placeholder match is dropped, anything else flushes.
	// After real text has been emitted, short trailing fragments that match known
	// placeholder words are held back until the next event resolves them.
	flushTrailingHoldback := func() error {
		if trailingHoldback == "" {
			return nil
		}
		flush := trailingHoldback
		trailingHoldback = ""
		return emitTextDeltaRaw(flush)
	}
	suppressTrailingHoldback := func() {
		trailingHoldback = ""
	}
	emitTextDelta := func(text string) error {
		if text == "" {
			return nil
		}
		if placeholderResolved {
			if err := flushTrailingHoldback(); err != nil {
				return err
			}
			if kiropkg.IsPlaceholderFragment(text) {
				trailingHoldback = text
				return nil
			}
			return emitTextDeltaRaw(text)
		}
		placeholderPending += text
		exact, prefix := kiropkg.ToolTurnPlaceholderMatch(placeholderPending)
		if exact {
			// Entire buffered text is a placeholder echo: drop it and keep
			// suppression armed in case more placeholder text follows.
			placeholderPending = ""
			return nil
		}
		if prefix {
			// Still could become a placeholder; keep buffering.
			return nil
		}
		// Confirmed real text: flush buffer and disable suppression.
		placeholderResolved = true
		flush := placeholderPending
		placeholderPending = ""
		return emitTextDeltaRaw(flush)
	}
	flushPendingPlaceholder = func() error {
		if err := flushTrailingHoldback(); err != nil {
			return err
		}
		if placeholderResolved || placeholderPending == "" {
			placeholderPending = ""
			return nil
		}
		if exact, _ := kiropkg.ToolTurnPlaceholderMatch(placeholderPending); exact {
			placeholderPending = ""
			return nil
		}
		placeholderResolved = true
		flush := placeholderPending
		placeholderPending = ""
		return emitTextDeltaRaw(flush)
	}
	openThinkingBlock := func() error {
		if thinkingBlockOpen {
			return nil
		}
		if err := closeTextBlock(); err != nil {
			return err
		}
		if !streamStarted {
			if err := startStream(inputTokens); err != nil {
				return err
			}
		}
		thinkingBlockIndex = nextBlockIndex
		nextBlockIndex++
		thinkingBlockOpen = true
		return writeKiroThinkingBlockStart(writer, thinkingBlockIndex)
	}
	emitThinkingDelta := func(thinking string) error {
		if thinking == "" {
			return nil
		}
		_, _ = nativeThinkingBuilder.WriteString(thinking)
		return writeKiroThinkingBlockDelta(writer, thinkingBlockIndex, thinking)
	}
	closeThinkingBlock := func() error {
		if !thinkingBlockOpen {
			return nil
		}
		if err := writeKiroThinkingBlockDelta(writer, thinkingBlockIndex, ""); err != nil {
			return err
		}
		if err := writeKiroThinkingBlockStop(writer, thinkingBlockIndex); err != nil {
			return err
		}
		thinkingBlockOpen = false
		thinkingBlockIndex = -1
		return nil
	}
	processAssistantContent := func(content string) error {
		nativeThinkingBuffer += content
		for {
			if !thinkingBlockOpen && !nativeThinkingExtracted {
				start := strings.Index(nativeThinkingBuffer, "<thinking>")
				if start >= 0 {
					before := nativeThinkingBuffer[:start]
					if strings.TrimSpace(before) != "" {
						if err := emitTextDelta(before); err != nil {
							return err
						}
					}
					nativeThinkingBuffer = nativeThinkingBuffer[start+len("<thinking>"):]
					stripThinkingLeadingNewline = true
					if err := openThinkingBlock(); err != nil {
						return err
					}
					continue
				}
				targetLen := len(nativeThinkingBuffer) - len("<thinking>")
				if targetLen > 0 {
					safeContent := utf8SafePrefix(nativeThinkingBuffer, targetLen)
					if strings.TrimSpace(safeContent) != "" {
						if err := emitTextDelta(safeContent); err != nil {
							return err
						}
						nativeThinkingBuffer = nativeThinkingBuffer[len(safeContent):]
					}
				}
				return nil
			}
			if thinkingBlockOpen {
				if stripThinkingLeadingNewline {
					if strings.HasPrefix(nativeThinkingBuffer, "\n") {
						nativeThinkingBuffer = nativeThinkingBuffer[1:]
						stripThinkingLeadingNewline = false
					} else if nativeThinkingBuffer != "" {
						stripThinkingLeadingNewline = false
					} else {
						return nil
					}
				}
				end := strings.Index(nativeThinkingBuffer, "</thinking>")
				if end >= 0 {
					if err := emitThinkingDelta(nativeThinkingBuffer[:end]); err != nil {
						return err
					}
					if err := closeThinkingBlock(); err != nil {
						return err
					}
					nativeThinkingExtracted = true
					nativeThinkingBuffer = strings.TrimLeft(nativeThinkingBuffer[end+len("</thinking>"):], "\n")
					continue
				}
				targetLen := len(nativeThinkingBuffer) - len("</thinking>\n\n")
				if targetLen > 0 {
					safeContent := utf8SafePrefix(nativeThinkingBuffer, targetLen)
					if err := emitThinkingDelta(safeContent); err != nil {
						return err
					}
					nativeThinkingBuffer = nativeThinkingBuffer[len(safeContent):]
				}
				return nil
			}
			if nativeThinkingBuffer == "" {
				return nil
			}
			if err := emitTextDelta(nativeThinkingBuffer); err != nil {
				return err
			}
			nativeThinkingBuffer = ""
			return nil
		}
	}

	for {
		chunk := make([]byte, 4096)
		n, readErr := reader.Read(chunk)
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)
			if len(buffer) > kiroMaxBodySize {
				err := fmt.Errorf("kiro stream buffer exceeded limit %d", kiroMaxBodySize)
				if !streamStarted {
					return nil, s.newKiroPreStartStreamFailoverError(ctx, c, account, resp.Header.Get("x-amzn-requestid"), resp.Header.Clone(), http.StatusBadGateway, kiroTransportFailureReasonKeyword, "Kiro upstream disconnected before first forwardable event", err.Error())
				}
				s.handleProtocolError(ctx, c, account, parsed.Model, true, err)
				return nil, err
			}
			for {
				frame, consumed, ok, err := parseKiroFrame(buffer)
				if err != nil {
					if !streamStarted {
						return nil, s.newKiroPreStartStreamFailoverError(ctx, c, account, resp.Header.Get("x-amzn-requestid"), resp.Header.Clone(), http.StatusBadGateway, kiroTransportFailureReasonKeyword, "Kiro upstream disconnected before first forwardable event", err.Error())
					}
					s.handleProtocolError(ctx, c, account, parsed.Model, true, err)
					return nil, err
				}
				if !ok {
					break
				}
				buffer = buffer[consumed:]
				framesSeen++

				if debugAggregator != nil && debugAggregator.enabled {
					logKiroFrameDiagnostic(ctx, account, parsed, frame)
				}
				debugAggregator.Append(frame, rawStringField(frame.Payload, "content"))

				if failureErr := kiroFrameFailure(frame); failureErr != nil {
					if streamStarted {
						handledErr := s.handleFrameFailure(ctx, c, account, resp.Header.Get("x-amzn-requestid"), frame, failureErr, false)
						if err := closeOpenKiroBlocks(writer, textBlockOpen, textBlockIndex, thinkingBlockOpen, thinkingBlockIndex, toolStates); err != nil {
							return nil, err
						}
						_ = writeKiroStreamError(writer, failureErr.Error())
						logKiroResponseAnomaly(ctx, account, parsed, true, "frame_failure", failureErr, framesSeen, completedToolUses, lastContextUsagePercentage)
						var failoverErr *UpstreamFailoverError
						if handledErr != nil && !errors.As(handledErr, &failoverErr) {
							return nil, handledErr
						}
						return nil, failureErr
					}
					return nil, s.handleFrameFailure(ctx, c, account, resp.Header.Get("x-amzn-requestid"), frame, failureErr, true)
				}

				switch frame.EventType {
				case "contextUsageEvent":
					if usagePercent, ok := numericField(frame.Payload, "contextUsagePercentage"); ok {
						lastContextUsagePercentage = &usagePercent
						setKiroContextUsagePercentage(c, usagePercent)
						if reason := kiroStopReasonFromContextUsage(usagePercent); reason != "" {
							stopReason = reason
						}
					}
				case "reasoningContentEvent":
					reasoningText := rawStringField(frame.Payload, "text")
					reasoningSignature := rawStringField(frame.Payload, "signature")
					if reasoningText == "" && reasoningSignature == "" {
						continue
					}
					if !streamStarted {
						if err := startStream(inputTokens); err != nil {
							return nil, err
						}
					}
					if !thinkingBlockOpen {
						if err := openThinkingBlock(); err != nil {
							return nil, err
						}
						reasoningThinkingActive = true
					}
					if reasoningText != "" {
						if err := emitThinkingDelta(reasoningText); err != nil {
							return nil, err
						}
					}
					if reasoningSignature != "" {
						if err := writeKiroThinkingBlockSignatureDelta(writer, thinkingBlockIndex, reasoningSignature); err != nil {
							return nil, err
						}
					}
				case "assistantResponseEvent":
					content := rawStringField(frame.Payload, "content")
					if content == "" {
						continue
					}
					if reasoningThinkingActive && thinkingBlockOpen {
						if err := closeThinkingBlock(); err != nil {
							return nil, err
						}
						reasoningThinkingActive = false
						nativeThinkingExtracted = true
					}
					if err := processAssistantContent(content); err != nil {
						return nil, err
					}
				case "toolUseEvent":
					suppressTrailingHoldback()
					toolUseID := controlStringField(frame.Payload, "toolUseId")
					state := ensureKiroToolState(toolStates, toolUseID, controlStringField(frame.Payload, "name"))
					toolOrder = appendKiroToolStateOrder(toolOrder, state)
					if shadowBridge, ok := kiroShadowToolBridgeForState(converted, state); ok {
						shadowMaxUsesLimit = mergeShadowMaxUsesLimit(shadowMaxUsesLimit, shadowBridge.MaxUses)
						inputChunk := rawStringField(frame.Payload, "input")
						if inputChunk != "" {
							_, _ = state.InputBuilder.WriteString(inputChunk)
						}
						if normalToolSeen {
							if booleanField(frame.Payload, "stop") {
								state.Stopped = true
							}
							continue
						}
						shadowToolSeen = true
						if !streamStarted {
							if err := startStream(inputTokens); err != nil {
								return nil, err
							}
						}
						if inputChunk != "" {
							_, _ = toolOutputBuilder.WriteString(inputChunk)
						}
						if booleanField(frame.Payload, "stop") {
							if err := ensureShadowMaxUsesNotExceeded(completedShadowToolUses, shadowMaxUsesLimit); err != nil {
								return nil, err
							}
							if thinkingBlockOpen {
								if err := emitThinkingDelta(nativeThinkingBuffer); err != nil {
									return nil, err
								}
								nativeThinkingBuffer = ""
								if err := closeThinkingBlock(); err != nil {
									return nil, err
								}
								nativeThinkingExtracted = true
								reasoningThinkingActive = false
							}
							if !nativeThinkingExtracted && nativeThinkingBuffer != "" {
								if strings.TrimSpace(nativeThinkingBuffer) != "" {
									if err := emitTextDelta(nativeThinkingBuffer); err != nil {
										return nil, err
									}
								}
								nativeThinkingBuffer = ""
							}
							suppressTrailingHoldback()
							if err := closeTextBlock(); err != nil {
								return nil, err
							}
							shadowBlocks, shadowOutput, shadowErr := s.executeKiroShadowTool(ctx, account, state, shadowBridge)
							if shadowErr != nil {
								return nil, shadowErr
							}
							rawShadowInput := state.InputBuilder.String()
							for _, block := range shadowBlocks {
								blockIndex := nextBlockIndex
								nextBlockIndex++
								if err := writeKiroShadowStreamBlock(writer, blockIndex, block, rawShadowInput); err != nil {
									return nil, err
								}
							}
							if stopReason == "end_turn" || stopReason == "tool_use" {
								stopReason = "pause_turn"
							}
							state.Stopped = true
							completedToolUses++
							completedShadowToolUses++
							if name := kiroShadowAnthropicToolName(shadowBridge, state.Name); name != "" {
								toolNames = append(toolNames, name)
							}
							_, _ = toolOutputBuilder.WriteString(shadowOutput)
						}
						continue
					}
					if shadowToolSeen {
						conflictErr := kiroShadowToolConflictError(state.Name)
						if streamStarted {
							if err := closeOpenKiroBlocks(writer, textBlockOpen, textBlockIndex, thinkingBlockOpen, thinkingBlockIndex, toolStates); err != nil {
								return nil, err
							}
						}
						_ = writeKiroStreamError(writer, conflictErr.Error())
						logKiroResponseAnomaly(ctx, account, parsed, true, "shadow_web_tool_conflict", conflictErr, framesSeen, completedToolUses, lastContextUsagePercentage)
						s.handleProtocolError(ctx, c, account, parsed.Model, true, conflictErr)
						return nil, conflictErr
					}
					normalToolSeen = true
					if kiroIsBufferedResponseTool(converted, state.Name) {
						inputChunk := rawStringField(frame.Payload, "input")
						if inputChunk != "" {
							_, _ = state.InputBuilder.WriteString(inputChunk)
							_, _ = toolOutputBuilder.WriteString(inputChunk)
						}
						if booleanField(frame.Payload, "stop") {
							if stopReason == "end_turn" {
								stopReason = "tool_use"
							}
							state.Stopped = true
							completedToolUses++
							if !streamStarted {
								if err := startStream(inputTokens); err != nil {
									return nil, err
								}
							}
							if thinkingBlockOpen {
								if err := emitThinkingDelta(nativeThinkingBuffer); err != nil {
									return nil, err
								}
								nativeThinkingBuffer = ""
								if err := closeThinkingBlock(); err != nil {
									return nil, err
								}
								nativeThinkingExtracted = true
								reasoningThinkingActive = false
							}
							if !nativeThinkingExtracted && nativeThinkingBuffer != "" {
								if strings.TrimSpace(nativeThinkingBuffer) != "" {
									if err := emitTextDelta(nativeThinkingBuffer); err != nil {
										return nil, err
									}
								}
								nativeThinkingBuffer = ""
							}
							suppressTrailingHoldback()
							if err := closeTextBlock(); err != nil {
								return nil, err
							}
							toolUseBlock, ok := buildKiroToolUseBlock(state, converted)
							if ok {
								blockIndex := nextBlockIndex
								nextBlockIndex++
								if strings.TrimSpace(kiroShadowStringField(toolUseBlock, "type")) == "server_tool_use" {
									if err := writeKiroShadowStreamBlock(writer, blockIndex, toolUseBlock, state.InputBuilder.String()); err != nil {
										return nil, err
									}
								} else {
									if err := writeKiroCompleteToolStreamBlock(writer, blockIndex, toolUseBlock); err != nil {
										return nil, err
									}
								}
								if name := strings.TrimSpace(kiroShadowStringField(toolUseBlock, "name")); name != "" {
									toolNames = append(toolNames, name)
								}
							}
							_, _ = toolOutputBuilder.WriteString(state.Name)
						}
						continue
					}
					if !streamStarted {
						if err := startStream(inputTokens); err != nil {
							return nil, err
						}
					}
					if !state.Started {
						if thinkingBlockOpen {
							if err := emitThinkingDelta(nativeThinkingBuffer); err != nil {
								return nil, err
							}
							nativeThinkingBuffer = ""
							if err := closeThinkingBlock(); err != nil {
								return nil, err
							}
							nativeThinkingExtracted = true
							reasoningThinkingActive = false
						}
						if !nativeThinkingExtracted && nativeThinkingBuffer != "" {
							if strings.TrimSpace(nativeThinkingBuffer) != "" {
								if err := emitTextDelta(nativeThinkingBuffer); err != nil {
									return nil, err
								}
							}
							nativeThinkingBuffer = ""
						}
						suppressTrailingHoldback()
						if err := closeTextBlock(); err != nil {
							return nil, err
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
						_, _ = toolOutputBuilder.WriteString(inputChunk)
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
						if stopReason == "end_turn" {
							stopReason = "tool_use"
						}
						state.Stopped = true
						completedToolUses++
						if name := strings.TrimSpace(kiroVisibleToolName(converted, state.Name)); name != "" {
							toolNames = append(toolNames, name)
						}
						_, _ = toolOutputBuilder.WriteString(state.Name)
						if err := writeSSEEvent(writer, "content_block_stop", map[string]any{
							"type":  "content_block_stop",
							"index": state.BlockIndex,
						}); err != nil {
							return nil, err
						}
					}
				}
			}
		}
		if readErr != nil {
			if isClientDisconnectError(c, readErr) {
				return nil, readErr
			}
			if !streamStarted && firstForwardableTimeoutTriggered.Load() {
				timeoutErr := fmt.Errorf("kiro upstream did not emit a forwardable event within %s", kiroFirstForwardableEventTimeout)
				return nil, s.newKiroPreStartStreamFailoverError(ctx, c, account, resp.Header.Get("x-amzn-requestid"), resp.Header.Clone(), http.StatusGatewayTimeout, kiroTransportFailureReasonKeyword, timeoutErr.Error(), readErr.Error())
			}
			if readErr == io.EOF {
				if len(buffer) > 0 {
					incompleteErr := fmt.Errorf("incomplete kiro frame at EOF: %w", io.ErrUnexpectedEOF)
					if !streamStarted {
						return nil, s.newKiroPreStartStreamFailoverError(ctx, c, account, resp.Header.Get("x-amzn-requestid"), resp.Header.Clone(), http.StatusBadGateway, kiroTransportFailureReasonKeyword, "Kiro upstream disconnected before first forwardable event", incompleteErr.Error())
					}
					if streamStarted {
						if err := closeOpenKiroBlocks(writer, textBlockOpen, textBlockIndex, thinkingBlockOpen, thinkingBlockIndex, toolStates); err != nil {
							return nil, err
						}
					}
					_ = writeKiroStreamError(writer, incompleteErr.Error())
					logKiroResponseAnomaly(ctx, account, parsed, true, "incomplete_frame_eof", incompleteErr, framesSeen, completedToolUses, lastContextUsagePercentage)
					s.handleProtocolError(ctx, c, account, parsed.Model, true, incompleteErr)
					return nil, incompleteErr
				}
				if framesSeen == 0 {
					emptyErr := errors.New("empty kiro response body")
					if !streamStarted {
						return nil, s.newKiroPreStartStreamFailoverError(ctx, c, account, resp.Header.Get("x-amzn-requestid"), resp.Header.Clone(), http.StatusBadGateway, kiroTransportFailureReasonKeyword, "Kiro upstream returned no assistant output before first forwardable event", emptyErr.Error())
					}
					_ = writeKiroStreamError(writer, emptyErr.Error())
					logKiroResponseAnomaly(ctx, account, parsed, true, "empty_body", emptyErr, framesSeen, completedToolUses, lastContextUsagePercentage)
					s.handleProtocolError(ctx, c, account, parsed.Model, true, emptyErr)
					return nil, emptyErr
				}
				break
			}
			if !streamStarted {
				return nil, s.newKiroPreStartStreamFailoverError(ctx, c, account, resp.Header.Get("x-amzn-requestid"), resp.Header.Clone(), http.StatusBadGateway, kiroTransportFailureReasonKeyword, "Kiro upstream disconnected before first forwardable event", readErr.Error())
			}
			if streamStarted {
				if err := closeOpenKiroBlocks(writer, textBlockOpen, textBlockIndex, thinkingBlockOpen, thinkingBlockIndex, toolStates); err != nil {
					return nil, err
				}
			}
			_ = writeKiroStreamError(writer, readErr.Error())
			logKiroResponseAnomaly(ctx, account, parsed, true, "stream_read_error", readErr, framesSeen, completedToolUses, lastContextUsagePercentage)
			s.handleProtocolError(ctx, c, account, parsed.Model, true, readErr)
			return nil, readErr
		}
	}
	if thinkingBlockOpen {
		if err := emitThinkingDelta(nativeThinkingBuffer); err != nil {
			return nil, err
		}
		nativeThinkingBuffer = ""
		if err := closeThinkingBlock(); err != nil {
			return nil, err
		}
		nativeThinkingExtracted = true
	} else if nativeThinkingBuffer != "" {
		if strings.TrimSpace(nativeThinkingBuffer) != "" {
			if err := emitTextDelta(nativeThinkingBuffer); err != nil {
				return nil, err
			}
		}
		nativeThinkingBuffer = ""
	}
	// Flush any buffered leading text now that the stream has ended. A
	// pure-placeholder buffer is dropped here, leaving textOutputBuilder empty so
	// the empty-output guard below can engage the fallback path.
	if err := flushPendingPlaceholder(); err != nil {
		return nil, err
	}
	visibleToolUses, completedVisibleToolUses, partialToolUses := kiroVisibleToolStateCounts(toolStates, toolOrder)
	hasVisibleToolOutput := visibleToolUses > 0
	if hasVisibleToolOutput && stopReason == "end_turn" {
		stopReason = "tool_use"
	}
	buildStreamTelemetry := func() *kiroResponseTelemetry {
		return &kiroResponseTelemetry{
			FramesSeen:             framesSeen,
			AssistantChars:         textOutputBuilder.Len(),
			NativeThinkingChars:    nativeThinkingBuilder.Len(),
			ToolUseCount:           visibleToolUses,
			CompletedToolUseCount:  completedVisibleToolUses,
			PartialToolUseCount:    partialToolUses,
			ContextUsagePercentage: lastContextUsagePercentage,
		}
	}
	computeStreamUsageTokens := func() (outputTokens int, thinkingTokens int) {
		streamThinkingText := nativeThinkingBuilder.String()
		return estimateKiroOutputTokens(textOutputBuilder.String()+streamThinkingText, toolOutputBuilder.String()),
			estimateKiroOutputTokens(streamThinkingText, "")
	}
	partialTelemetry := buildStreamTelemetry()
	if partialTelemetry.PartialToolUseCount > 0 && partialTelemetry.CompletedToolUseCount == 0 {
		outputTokens, _ := computeStreamUsageTokens()
		incompleteErr := errors.New("kiro response completed with incomplete tool_use output")
		if err := writeKiroStreamError(writer, kiroIncompleteToolUseClientMessage()); err != nil {
			return nil, err
		}
		logKiroResponseAnomaly(ctx, account, parsed, true, "incomplete_tool_use_completed", incompleteErr, partialTelemetry.FramesSeen, partialTelemetry.ToolUseCount, partialTelemetry.ContextUsagePercentage)
		if c != nil && account != nil {
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:          account.Platform,
				AccountID:         account.ID,
				AccountName:       account.Name,
				UpstreamRequestID: strings.TrimSpace(resp.Header.Get("x-amzn-requestid")),
				Kind:              "response_anomaly",
				Message:           "kiro completed with anomalies: incomplete_tool_use_completed",
				Detail:            partialTelemetry.opsDetail(stopReason, outputTokens, partialTelemetry.shortOutput(outputTokens), []string{"incomplete_tool_use_completed"}),
			})
		}
		return nil, incompleteErr
	}
	if !streamStarted || (textOutputBuilder.Len() == 0 && nativeThinkingBuilder.Len() == 0 && !hasVisibleToolOutput) {
		emptyErr := errors.New("kiro response contained no assistant output")
		if !streamStarted {
			return nil, s.newKiroPreStartStreamFailoverError(ctx, c, account, resp.Header.Get("x-amzn-requestid"), resp.Header.Clone(), http.StatusBadGateway, kiroTransportFailureReasonKeyword, "Kiro upstream returned no assistant output before first forwardable event", emptyErr.Error())
		}
		if streamStarted {
			if err := closeOpenKiroBlocks(writer, textBlockOpen, textBlockIndex, thinkingBlockOpen, thinkingBlockIndex, toolStates); err != nil {
				return nil, err
			}
		}
		_ = writeKiroStreamError(writer, kiroEmptyOutputClientMessage(lastContextUsagePercentage))
		logKiroResponseAnomaly(ctx, account, parsed, true, "empty_output", emptyErr, framesSeen, completedToolUses, lastContextUsagePercentage)
		s.handleProtocolError(ctx, c, account, parsed.Model, true, emptyErr)
		return nil, emptyErr
	}
	outputTokens, streamThinkingTokens := computeStreamUsageTokens()
	telemetry := buildStreamTelemetry()
	if err := closeTextBlock(); err != nil {
		return nil, err
	}
	if err := closeOpenKiroBlocks(writer, false, 0, false, 0, toolStates); err != nil {
		return nil, err
	}

	finalFakeCacheUsage := resolveKiroFakeCacheUsage(fakeCachePlan, fakeCacheHit, inputTokens, runtimeSettings)
	inputTokens = finalFakeCacheUsage.InputTokens
	if err := writeSSEEvent(writer, "message_delta", map[string]any{
		"type":               "message_delta",
		"delta":              map[string]any{"stop_reason": stopReason, "stop_sequence": nil, "stop_details": nil},
		"usage":              kiroAnthropicUsageForDelta(inputTokens, outputTokens, finalFakeCacheUsage, streamThinkingTokens),
		"context_management": map[string]any{"applied_edits": []any{}},
	}); err != nil {
		return nil, err
	}
	if err := writeSSEEvent(writer, "message_stop", map[string]any{"type": "message_stop"}); err != nil {
		return nil, err
	}
	s.commitFakeCachePlan(fakeCachePlan, runtimeSettings)

	result := &ForwardResult{
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
			CacheCreation5mTokens:    finalFakeCacheUsage.CacheCreationInputTokens,
			CacheReadInputTokens:     finalFakeCacheUsage.CacheReadInputTokens,
		},
	}
	s.recordKiroSuccessfulAnomalies(ctx, c, account, parsed, result.RequestID, true, outputTokens, stopReason, telemetry)
	logKiroRequestCompleted(ctx, account, parsed, result.RequestID, result.UpstreamModel, true, result.Duration, firstTokenMs, inputTokens, outputTokens, finalFakeCacheUsage.CacheCreationInputTokens, finalFakeCacheUsage.CacheReadInputTokens, stopReason, toolNames, telemetry)
	return result, nil
}

func estimateKiroOutputTokens(textOutput, toolOutput string) int {
	return kiropkg.EstimateOutputTokens(textOutput + toolOutput)
}

func kiroAnthropicUsageForStart(inputTokens, outputTokens int, fakeCacheUsage kiropkg.FakeCacheUsage) gin.H {
	return gin.H{
		"input_tokens":                inputTokens,
		"output_tokens":               outputTokens,
		"cache_creation_input_tokens": fakeCacheUsage.CacheCreationInputTokens,
		"cache_read_input_tokens":     fakeCacheUsage.CacheReadInputTokens,
		"cache_creation": gin.H{
			"ephemeral_5m_input_tokens": fakeCacheUsage.CacheCreationInputTokens,
			"ephemeral_1h_input_tokens": 0,
		},
	}
}

func kiroAnthropicUsageForDelta(inputTokens, outputTokens int, fakeCacheUsage kiropkg.FakeCacheUsage, thinkingTokens int) gin.H {
	usage := gin.H{
		"input_tokens":                inputTokens,
		"output_tokens":               outputTokens,
		"cache_creation_input_tokens": fakeCacheUsage.CacheCreationInputTokens,
		"cache_read_input_tokens":     fakeCacheUsage.CacheReadInputTokens,
		"output_tokens_details":       gin.H{"thinking_tokens": thinkingTokens},
	}
	if fakeCacheUsage.CacheCreationInputTokens > 0 {
		usage["cache_creation"] = gin.H{
			"ephemeral_5m_input_tokens": fakeCacheUsage.CacheCreationInputTokens,
		}
	}
	return usage
}

func writeKiroThinkingBlock(writer gin.ResponseWriter, index int, thinking string) error {
	if thinking == "" {
		return nil
	}
	if err := writeKiroThinkingBlockStart(writer, index); err != nil {
		return err
	}
	if err := writeKiroThinkingBlockDelta(writer, index, thinking); err != nil {
		return err
	}
	return writeKiroThinkingBlockStop(writer, index)
}

func writeKiroThinkingBlockStart(writer gin.ResponseWriter, index int) error {
	return writeSSEEvent(writer, "content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": index,
		"content_block": map[string]any{
			"type":     "thinking",
			"thinking": "",
		},
	})
}

func writeKiroThinkingBlockDelta(writer gin.ResponseWriter, index int, thinking string) error {
	return writeSSEEvent(writer, "content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": index,
		"delta": map[string]any{
			"type":     "thinking_delta",
			"thinking": thinking,
		},
	})
}

func writeKiroThinkingBlockSignatureDelta(writer gin.ResponseWriter, index int, signature string) error {
	if signature == "" {
		return nil
	}
	return writeSSEEvent(writer, "content_block_delta", map[string]any{
		"type":  "content_block_delta",
		"index": index,
		"delta": map[string]any{
			"type":      "signature_delta",
			"signature": signature,
		},
	})
}

func writeKiroThinkingBlockStop(writer gin.ResponseWriter, index int) error {
	return writeSSEEvent(writer, "content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": index,
	})
}

func writeKiroShadowStreamBlock(writer gin.ResponseWriter, index int, block map[string]any, rawInput string) error {
	if strings.TrimSpace(kiroShadowStringField(block, "type")) != "server_tool_use" {
		return writeKiroCompleteToolStreamBlock(writer, index, block)
	}

	serverToolUse := map[string]any{
		"type":  "server_tool_use",
		"id":    kiroShadowStringField(block, "id"),
		"name":  kiroShadowStringField(block, "name"),
		"input": map[string]any{},
	}
	if err := writeSSEEvent(writer, "content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         index,
		"content_block": serverToolUse,
	}); err != nil {
		return err
	}
	if rawInput != "" {
		if err := writeSSEEvent(writer, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": index,
			"delta": map[string]any{
				"type":         "input_json_delta",
				"partial_json": rawInput,
			},
		}); err != nil {
			return err
		}
	}
	return writeSSEEvent(writer, "content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": index,
	})
}

func writeKiroCompleteToolStreamBlock(writer gin.ResponseWriter, index int, block map[string]any) error {
	if err := writeSSEEvent(writer, "content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         index,
		"content_block": block,
	}); err != nil {
		return err
	}
	return writeSSEEvent(writer, "content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": index,
	})
}

func splitKiroThinkingContent(text string) (before, thinking, after string, found bool) {
	start := strings.Index(text, "<thinking>")
	if start < 0 {
		return text, "", "", false
	}
	afterOpen := text[start+len("<thinking>"):]
	end := strings.Index(afterOpen, "</thinking>")
	if end < 0 {
		return text, "", "", false
	}
	before = text[:start]
	thinking = afterOpen[:end]
	thinking = strings.TrimPrefix(thinking, "\n")
	after = afterOpen[end+len("</thinking>"):]
	after = strings.TrimLeft(after, "\n")
	return before, thinking, after, true
}

func appendKiroNativeContentBlocks(text string, content []map[string]any) ([]map[string]any, string, string) {
	remaining := text
	var textOutput strings.Builder
	var thinkingOutput strings.Builder
	for remaining != "" {
		before, thinking, after, found := splitKiroThinkingContent(remaining)
		if !found {
			if trimmed := strings.TrimSpace(remaining); trimmed != "" {
				content = append(content, map[string]any{"type": "text", "text": remaining})
				_, _ = textOutput.WriteString(remaining)
			}
			break
		}
		if strings.TrimSpace(before) != "" {
			content = append(content, map[string]any{"type": "text", "text": before})
			_, _ = textOutput.WriteString(before)
		}
		if thinking != "" {
			content = append(content, map[string]any{"type": "thinking", "thinking": thinking})
			_, _ = thinkingOutput.WriteString(thinking)
		}
		remaining = after
	}
	return content, textOutput.String(), thinkingOutput.String()
}

func collectKiroAssistantResponseText(frames []*kiroFrame) (string, bool) {
	var textBuilder strings.Builder
	hasAssistantResponse := false
	for _, frame := range frames {
		if frame == nil {
			return "", false
		}
		if failureErr := kiroFrameFailure(frame); failureErr != nil {
			return "", false
		}
		if frame.EventType != "assistantResponseEvent" {
			continue
		}
		content := rawStringField(frame.Payload, "content")
		if content == "" {
			continue
		}
		hasAssistantResponse = true
		_, _ = textBuilder.WriteString(content)
	}
	return textBuilder.String(), hasAssistantResponse
}

func extractKiroNativeThinkingText(text string) string {
	content, _, _ := appendKiroNativeContentBlocks(text, nil)
	thinkingParts := make([]string, 0, len(content))
	for _, block := range content {
		if rawType, ok := block["type"].(string); !ok || rawType != "thinking" {
			continue
		}
		thinking, _ := block["thinking"].(string)
		thinking = strings.TrimSpace(thinking)
		if thinking == "" {
			continue
		}
		thinkingParts = append(thinkingParts, thinking)
	}
	return strings.TrimSpace(strings.Join(thinkingParts, "\n\n"))
}

func normalizeKiroShadowToolHistory(body []byte) []byte {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}
	messages, _ := req["messages"].([]any)
	if len(messages) == 0 {
		return body
	}

	shadowToolNames := map[string]string{}
	shadowToolBridges := map[string]kiropkg.ShadowToolBridge{}
	changed := false
	for _, item := range messages {
		msg, _ := item.(map[string]any)
		blocks, _ := msg["content"].([]any)
		if len(blocks) == 0 {
			continue
		}
		role := strings.TrimSpace(kiroShadowStringField(msg, "role"))
		nextBlocks := make([]any, 0, len(blocks))
		msgChanged := false
		for _, rawBlock := range blocks {
			block, _ := rawBlock.(map[string]any)
			switch role {
			case "assistant":
				if strings.TrimSpace(kiroShadowStringField(block, "type")) == "server_tool_use" {
					if shadowName := kiroShadowToolNameForAnthropicName(kiroShadowStringField(block, "name")); shadowName != "" {
						toolUseID := strings.TrimSpace(kiroShadowStringField(block, "id"))
						if toolUseID != "" {
							shadowToolNames[toolUseID] = shadowName
							shadowToolBridges[toolUseID] = kiroShadowBridgeFromServerToolBlock(block, shadowName)
						}
						nextBlock := map[string]any{
							"type":  "tool_use",
							"id":    toolUseID,
							"name":  shadowName,
							"input": block["input"],
						}
						if bridgePayload := kiroShadowBridgePayload(shadowToolBridges[toolUseID]); len(bridgePayload) > 0 {
							nextBlock["_shadow_bridge"] = bridgePayload
						}
						nextBlocks = append(nextBlocks, nextBlock)
						msgChanged = true
						continue
					}
				}
			case "user":
				blockType := strings.TrimSpace(kiroShadowStringField(block, "type"))
				if (blockType == "web_search_tool_result" || blockType == "web_fetch_tool_result") && shadowToolNames[strings.TrimSpace(kiroShadowStringField(block, "tool_use_id"))] != "" {
					toolResult := map[string]any{
						"type":        "tool_result",
						"tool_use_id": kiroShadowStringField(block, "tool_use_id"),
						"content":     block["content"],
					}
					if blockType == "web_fetch_tool_result" && kiroShadowWebFetchContentIsError(block["content"]) {
						toolResult["is_error"] = true
					}
					nextBlocks = append(nextBlocks, toolResult)
					msgChanged = true
					continue
				}
			}
			nextBlocks = append(nextBlocks, rawBlock)
		}
		if msgChanged {
			msg["content"] = nextBlocks
			changed = true
		}
	}
	if !changed {
		return body
	}
	normalized, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return normalized
}

func (s *KiroGatewayService) prepareFakeCachePlan(account *Account, parsed *ParsedRequest, meta *kiroPreparedRequestMeta, runtimeSettings *KiroRuntimeSettings) (*kiropkg.FakeCachePlan, kiropkg.FakeCacheHitState) {
	if s == nil || account == nil || parsed == nil {
		return nil, kiropkg.FakeCacheHitState{}
	}

	plan, err := kiropkg.BuildFakeCachePlan(kiroFakeCachePlanBody(parsed, meta), kiropkg.FakeCacheScope{
		AccountID: account.ID,
		UserID:    parsed.UserID,
		APIKeyID:  parsed.APIKeyID,
	}, parsed.Model)
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
		if plan.SessionProgressKey != "" {
			if value, ok := s.fakeCache.Get(plan.SessionProgressKey); ok {
				if tokens, ok := value.(int); ok && tokens > hit.EffectiveCachedTokens {
					hit.EffectiveCachedTokens = tokens
				}
			}
		}
		for _, checkpoint := range plan.Checkpoints {
			if checkpoint.Key == "" || checkpoint.Tokens <= hit.CheckpointTokens {
				continue
			}
			if _, ok := s.fakeCache.Get(checkpoint.Key); ok {
				hit.CheckpointTokens = checkpoint.Tokens
			}
		}
		s.fakeCacheMu.Unlock()
	}

	return plan, hit
}

func kiroFakeCachePlanBody(parsed *ParsedRequest, meta *kiroPreparedRequestMeta) []byte {
	if meta != nil && len(meta.ForwardBody) > 0 {
		return meta.ForwardBody
	}
	if parsed == nil {
		return nil
	}
	return parsed.Body.Bytes()
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
	}
	if plan.CurrentKey != "" && plan.CurrentCacheableTokens > 0 {
		s.fakeCache.Set(plan.CurrentKey, struct{}{}, time.Duration(runtimeSettings.CachePrefixTTLSecs)*time.Second)
	}
	for _, checkpoint := range plan.Checkpoints {
		if checkpoint.Key == "" || checkpoint.Tokens <= 0 {
			continue
		}
		if checkpoint.Key == plan.IndependentKey {
			continue
		}
		s.fakeCache.Set(checkpoint.Key, struct{}{}, time.Duration(runtimeSettings.CachePrefixTTLSecs)*time.Second)
	}
	if plan.SessionProgressKey != "" {
		// Persist the effective cached weight (cache read + cache creation) recorded
		// for this turn, not the ideal cumulative. A sub-100% hit rate makes this
		// smaller than the ideal, so the next turn reads from a shrunken basis and
		// the imperfect-hit-rate effect chains forward. Falls back to the ideal
		// cumulative when no usage was resolved (RecordedEffectiveCachedTokens == 0).
		effectiveTokens := plan.RecordedEffectiveCachedTokens
		if effectiveTokens <= 0 {
			effectiveTokens = plan.CurrentCacheableTokens
			if checkpointTokens := plan.CurrentCheckpointTokens(); checkpointTokens > effectiveTokens {
				effectiveTokens = checkpointTokens
			}
		}
		if effectiveTokens > 0 {
			s.fakeCache.Set(plan.SessionProgressKey, effectiveTokens, time.Duration(runtimeSettings.CachePrefixTTLSecs)*time.Second)
		}
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

func appendKiroToolStateOrder(order []string, state *kiroToolState) []string {
	if state == nil || strings.TrimSpace(state.ToolUseID) == "" {
		return order
	}
	for _, existing := range order {
		if existing == state.ToolUseID {
			return order
		}
	}
	return append(order, state.ToolUseID)
}

func kiroToolStateHasVisibleOutput(state *kiroToolState) bool {
	if state == nil {
		return false
	}
	return state.Started || strings.TrimSpace(state.Name) != "" || strings.TrimSpace(state.InputBuilder.String()) != ""
}

func kiroToolStateHasCompleteInput(state *kiroToolState) bool {
	if state == nil {
		return false
	}
	raw := strings.TrimSpace(state.InputBuilder.String())
	if raw == "" {
		return true
	}
	var parsed any
	return json.Unmarshal([]byte(raw), &parsed) == nil
}

func kiroToolStateIsComplete(state *kiroToolState) bool {
	if !kiroToolStateHasVisibleOutput(state) {
		return false
	}
	return state.Stopped || kiroToolStateHasCompleteInput(state)
}

func buildKiroToolUseBlock(state *kiroToolState, converted *kiropkg.ConvertResult) (map[string]any, bool) {
	if !kiroToolStateHasVisibleOutput(state) {
		return nil, false
	}

	input := any(map[string]any{})
	raw := strings.TrimSpace(state.InputBuilder.String())
	if raw != "" {
		var parsed any
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil && parsed != nil {
			input = parsed
		}
	}
	input = repairKiroControlPlaneToolInput(state.Name, input)
	if serverToolUse, ok := kiropkg.AnthropicServerToolUseFromKiroToolUse(state.Name, input, kiroResponseToolMetadata(converted)); ok {
		serverToolUse["id"] = state.ToolUseID
		return serverToolUse, true
	}
	visibleName, visibleInput := kiropkg.AnthropicToolUseFromKiroToolUse(state.Name, input, kiroResponseToolMetadata(converted))

	return map[string]any{
		"type":  "tool_use",
		"id":    state.ToolUseID,
		"name":  visibleName,
		"input": visibleInput,
	}, true
}

func kiroResponseToolMetadata(converted *kiropkg.ConvertResult) *kiropkg.ToolMetadata {
	if converted == nil {
		return nil
	}
	return converted.ToolMetadata
}

func kiroVisibleToolName(converted *kiropkg.ConvertResult, name string) string {
	visibleName, _ := kiropkg.AnthropicToolUseFromKiroToolUse(name, map[string]any{}, kiroResponseToolMetadata(converted))
	return visibleName
}

func kiroIsBufferedResponseTool(converted *kiropkg.ConvertResult, name string) bool {
	metadata := kiroResponseToolMetadata(converted)
	if metadata == nil || metadata.ResponseTools == nil {
		return false
	}
	_, ok := metadata.ResponseTools[strings.TrimSpace(name)]
	return ok
}

func repairKiroControlPlaneToolInput(name string, input any) any {
	if !strings.EqualFold(strings.TrimSpace(name), "AskUserQuestion") {
		return input
	}
	obj, ok := input.(map[string]any)
	if !ok {
		return input
	}
	questions, ok := obj["questions"].([]any)
	if !ok {
		return input
	}
	for _, raw := range questions {
		question, ok := raw.(map[string]any)
		if !ok || strings.TrimSpace(anyString(question["question"])) != "" {
			continue
		}
		if header := strings.TrimSpace(anyString(question["header"])); header != "" {
			question["question"] = header
		}
	}
	return obj
}

func anyString(value any) string {
	text, _ := value.(string)
	return text
}

func kiroShadowToolBridgeForState(converted *kiropkg.ConvertResult, state *kiroToolState) (kiropkg.ShadowToolBridge, bool) {
	if converted == nil || converted.BridgeMetadata == nil || state == nil {
		return kiropkg.ShadowToolBridge{}, false
	}
	bridge, ok := converted.BridgeMetadata.ShadowTools[strings.TrimSpace(state.Name)]
	return bridge, ok
}

func kiroShadowToolNameForAnthropicName(name string) string {
	return ""
}

func (s *KiroGatewayService) executeKiroShadowTools(
	ctx context.Context,
	account *Account,
	converted *kiropkg.ConvertResult,
	states map[string]*kiroToolState,
	order []string,
) (map[string][]map[string]any, map[string]struct{}, []string, string, bool, error) {
	shadowHandledIDs, shadowExecutableIDs, err := kiroShadowToolExecutionPlan(converted, states, order)
	if err != nil {
		return nil, nil, nil, "", false, err
	}
	blocksByID := make(map[string][]map[string]any)
	toolNames := make([]string, 0, len(order))
	var outputBuilder strings.Builder
	shadowExecuted := false
	shadowUsed := 0
	shadowMaxUsesLimit := 0
	for _, toolUseID := range order {
		if _, ok := shadowExecutableIDs[toolUseID]; !ok {
			continue
		}
		state := states[toolUseID]
		bridge, _ := kiroShadowToolBridgeForState(converted, state)
		shadowMaxUsesLimit = mergeShadowMaxUsesLimit(shadowMaxUsesLimit, bridge.MaxUses)
		if err := ensureShadowMaxUsesNotExceeded(shadowUsed, shadowMaxUsesLimit); err != nil {
			return nil, nil, nil, "", false, err
		}
		blocks, outputText, err := s.executeKiroShadowTool(ctx, account, state, bridge)
		if err != nil {
			return nil, nil, nil, "", false, err
		}
		blocksByID[toolUseID] = blocks
		if name := kiroShadowAnthropicToolName(bridge, state.Name); name != "" {
			toolNames = append(toolNames, name)
		}
		_, _ = outputBuilder.WriteString(outputText)
		shadowExecuted = true
		shadowUsed++
	}
	return blocksByID, shadowHandledIDs, toolNames, outputBuilder.String(), shadowExecuted, nil
}

func (s *KiroGatewayService) executeKiroShadowTool(
	ctx context.Context,
	account *Account,
	state *kiroToolState,
	bridge kiropkg.ShadowToolBridge,
) ([]map[string]any, string, error) {
	if state == nil {
		return nil, "", nil
	}
	input, _ := kiroShadowToolInput(state)
	anthropicName := kiroShadowAnthropicToolName(bridge, state.Name)
	serverToolUse := map[string]any{
		"type":  "server_tool_use",
		"id":    state.ToolUseID,
		"name":  anthropicName,
		"input": input,
	}

	switch {
	case strings.HasPrefix(strings.ToLower(strings.TrimSpace(bridge.AnthropicType)), "web_search"):
		query := strings.TrimSpace(kiroShadowStringField(input, "query"))
		if query == "" {
			query = strings.TrimSpace(kiroShadowStringField(input, "q"))
		}
		resp, _, err := kiroShadowWebSearchExecutor(ctx, account, query)
		if err != nil {
			if errors.Is(err, websearch.ErrProxyUnavailable) {
				return nil, "", &UpstreamFailoverError{
					StatusCode:   http.StatusBadGateway,
					ResponseBody: []byte(err.Error()),
				}
			}
			return nil, "", err
		}
		return []map[string]any{
			serverToolUse,
			{
				"type":        "web_search_tool_result",
				"tool_use_id": state.ToolUseID,
				"content":     kiroShadowWebSearchResults(resp.Results),
			},
		}, buildTextSummary(query, resp.Results), nil
	case strings.HasPrefix(strings.ToLower(strings.TrimSpace(bridge.AnthropicType)), "web_fetch"):
		urlValue := strings.TrimSpace(kiroShadowStringField(input, "url"))
		fetchResult := kiroShadowWebFetchExecutor(ctx, account, webfetch.FetchRequest{
			URL:               urlValue,
			ProxyURL:          resolveAccountProxyURL(account),
			AllowedHosts:      bridge.AllowedDomains,
			BlockedHosts:      bridge.BlockedDomains,
			AllowPrivate:      s.kiroShadowAllowPrivateHosts(),
			AllowInsecureHTTP: s.kiroShadowAllowInsecureHTTP(),
		})
		fetchResult = applyShadowWebFetchContentLimit(fetchResult, bridge.MaxContentTokens)
		return []map[string]any{
			serverToolUse,
			{
				"type":        "web_fetch_tool_result",
				"tool_use_id": state.ToolUseID,
				"content":     kiroShadowWebFetchResultContent(fetchResult),
			},
		}, kiroShadowWebFetchSummary(fetchResult), nil
	default:
		return nil, "", nil
	}
}

func kiroShadowToolExecutionPlan(converted *kiropkg.ConvertResult, states map[string]*kiroToolState, order []string) (map[string]struct{}, map[string]struct{}, error) {
	handledShadowIDs := make(map[string]struct{})
	executableShadowIDs := make(map[string]struct{})
	normalToolSeen := false
	shadowToolSeen := false

	for _, toolUseID := range order {
		state := states[toolUseID]
		if !kiroToolStateHasVisibleOutput(state) {
			continue
		}
		_, isShadow := kiroShadowToolBridgeForState(converted, state)
		if !isShadow {
			if shadowToolSeen {
				return nil, nil, kiroShadowToolConflictError(state.Name)
			}
			normalToolSeen = true
			continue
		}

		handledShadowIDs[toolUseID] = struct{}{}
		if normalToolSeen {
			continue
		}
		shadowToolSeen = true
		if kiroToolStateIsComplete(state) {
			executableShadowIDs[toolUseID] = struct{}{}
		}
	}

	return handledShadowIDs, executableShadowIDs, nil
}

func kiroShadowToolConflictError(name string) error {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return errors.New("shadow web tool conflict: normal tool followed shadow web tool in same assistant turn")
	}
	return fmt.Errorf("shadow web tool conflict: normal tool %q followed shadow web tool in same assistant turn", trimmedName)
}

func kiroShadowToolInput(state *kiroToolState) (map[string]any, bool) {
	if state == nil {
		return map[string]any{}, false
	}
	raw := strings.TrimSpace(state.InputBuilder.String())
	if raw == "" {
		return map[string]any{}, true
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return map[string]any{}, false
	}
	return input, true
}

func kiroShadowStringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	value, _ := m[key].(string)
	return value
}

func kiroShadowAnthropicToolName(bridge kiropkg.ShadowToolBridge, fallback string) string {
	if name := strings.TrimSpace(bridge.AnthropicName); name != "" {
		return name
	}
	switch strings.TrimSpace(fallback) {
	case kiropkg.ShadowToolWebSearch:
		return "web_search"
	case kiropkg.ShadowToolWebFetch:
		return "web_fetch"
	default:
		return strings.TrimSpace(fallback)
	}
}

func kiroShadowWebSearchResults(results []websearch.SearchResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, result := range results {
		out = append(out, map[string]any{
			"type":  "url",
			"url":   result.URL,
			"title": result.Title,
		})
	}
	return out
}

func kiroShadowWebFetchResultContent(fetchResult *webfetch.FetchResult) map[string]any {
	if fetchResult == nil {
		return map[string]any{
			"type":       "web_fetch_tool_error",
			"error_code": "unavailable",
			"text":       "web fetch returned no result",
		}
	}

	urlValue := kiroShadowWebFetchURL(fetchResult)
	if fetchResult.Error != nil {
		return map[string]any{
			"type":       "web_fetch_tool_error",
			"url":        urlValue,
			"error_code": kiroShadowWebFetchErrorCode(fetchResult.Error),
			"text":       strings.TrimSpace(fetchResult.Error.Message),
		}
	}

	document := map[string]any{
		"type": "document",
		"source": map[string]any{
			"type":       "text",
			"media_type": "text/plain",
			"data":       fetchResult.Text,
		},
	}
	if title := strings.TrimSpace(fetchResult.Title); title != "" {
		document["title"] = title
	}

	content := map[string]any{
		"type":     "web_fetch_result",
		"url":      urlValue,
		"title":    strings.TrimSpace(fetchResult.Title),
		"text":     fetchResult.Text,
		"document": document,
	}
	if urlValue != "" {
		content["source"] = map[string]any{
			"type": "url",
			"url":  urlValue,
		}
	}
	return content
}

func kiroShadowWebFetchSummary(fetchResult *webfetch.FetchResult) string {
	if fetchResult == nil {
		return ""
	}
	if fetchResult.Error != nil {
		return strings.TrimSpace(kiroShadowWebFetchURL(fetchResult) + "\n" + fetchResult.Error.Message)
	}
	summaryParts := make([]string, 0, 2)
	if title := strings.TrimSpace(fetchResult.Title); title != "" {
		summaryParts = append(summaryParts, title)
	}
	if text := strings.TrimSpace(fetchResult.Text); text != "" {
		summaryParts = append(summaryParts, text)
	}
	return strings.TrimSpace(strings.Join(summaryParts, "\n"))
}

func mergeShadowMaxUsesLimit(current, incoming int) int {
	if incoming <= 0 {
		return current
	}
	if current <= 0 || incoming < current {
		return incoming
	}
	return current
}

func ensureShadowMaxUsesNotExceeded(alreadyUsed, limit int) error {
	if limit <= 0 || alreadyUsed < limit {
		return nil
	}
	return fmt.Errorf("shadow web tool max_uses exceeded: used %d, limit %d", alreadyUsed+1, limit)
}

func applyShadowWebFetchContentLimit(fetchResult *webfetch.FetchResult, maxContentTokens int) *webfetch.FetchResult {
	if fetchResult == nil || fetchResult.Error != nil || maxContentTokens <= 0 {
		return fetchResult
	}
	text, truncated := truncateShadowWebFetchText(fetchResult.Text, maxContentTokens)
	if !truncated {
		return fetchResult
	}
	cloned := *fetchResult
	cloned.Text = text
	cloned.Truncated = true
	return &cloned
}

func truncateShadowWebFetchText(text string, maxContentTokens int) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" || maxContentTokens <= 0 {
		return text, false
	}
	if kiropkg.AccurateTokenCount(text) <= maxContentTokens {
		return text, false
	}
	runes := []rune(text)
	low, high := 0, len(runes)
	best := ""
	for low <= high {
		mid := (low + high) / 2
		candidate := strings.TrimSpace(string(runes[:mid]))
		if candidate == "" {
			low = mid + 1
			continue
		}
		if kiropkg.AccurateTokenCount(candidate) <= maxContentTokens {
			best = candidate
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if best == "" {
		return "", true
	}
	return best, true
}

func kiroShadowBridgeFromServerToolBlock(block map[string]any, shadowName string) kiropkg.ShadowToolBridge {
	bridge := kiropkg.ShadowToolBridge{
		AnthropicType: strings.TrimSpace(kiroShadowStringField(block, "name")),
		AnthropicName: strings.TrimSpace(kiroShadowStringField(block, "name")),
	}
	if bridge.AnthropicType == "" {
		bridge = kiroShadowFallbackBridgeForName(shadowName)
	}
	bridge.AllowedDomains = kiroStringArrayField(block["allowed_domains"])
	bridge.BlockedDomains = kiroStringArrayField(block["blocked_domains"])
	bridge.MaxUses = kiroIntField(block["max_uses"])
	bridge.MaxContentTokens = kiroIntField(block["max_content_tokens"])
	return mergeKiroShadowBridge(kiroShadowFallbackBridgeForName(shadowName), bridge)
}

func kiroShadowFallbackBridgeForName(shadowName string) kiropkg.ShadowToolBridge {
	switch shadowName {
	case kiropkg.ShadowToolWebSearch:
		return kiropkg.ShadowToolBridge{AnthropicType: "web_search", AnthropicName: "web_search"}
	case kiropkg.ShadowToolWebFetch:
		return kiropkg.ShadowToolBridge{AnthropicType: "web_fetch", AnthropicName: "web_fetch"}
	default:
		return kiropkg.ShadowToolBridge{}
	}
}

func mergeKiroShadowBridge(base, override kiropkg.ShadowToolBridge) kiropkg.ShadowToolBridge {
	if strings.TrimSpace(override.AnthropicType) != "" {
		base.AnthropicType = override.AnthropicType
	}
	if strings.TrimSpace(override.AnthropicName) != "" {
		base.AnthropicName = override.AnthropicName
	}
	if len(override.AllowedDomains) > 0 {
		base.AllowedDomains = append([]string(nil), override.AllowedDomains...)
	}
	if len(override.BlockedDomains) > 0 {
		base.BlockedDomains = append([]string(nil), override.BlockedDomains...)
	}
	if override.MaxUses > 0 {
		base.MaxUses = override.MaxUses
	}
	if override.MaxContentTokens > 0 {
		base.MaxContentTokens = override.MaxContentTokens
	}
	return base
}

func kiroShadowBridgePayload(bridge kiropkg.ShadowToolBridge) map[string]any {
	payload := map[string]any{}
	if strings.TrimSpace(bridge.AnthropicType) != "" {
		payload["anthropic_type"] = bridge.AnthropicType
	}
	if strings.TrimSpace(bridge.AnthropicName) != "" {
		payload["anthropic_name"] = bridge.AnthropicName
	}
	if len(bridge.AllowedDomains) > 0 {
		payload["allowed_domains"] = append([]string(nil), bridge.AllowedDomains...)
	}
	if len(bridge.BlockedDomains) > 0 {
		payload["blocked_domains"] = append([]string(nil), bridge.BlockedDomains...)
	}
	if bridge.MaxUses > 0 {
		payload["max_uses"] = bridge.MaxUses
	}
	if bridge.MaxContentTokens > 0 {
		payload["max_content_tokens"] = bridge.MaxContentTokens
	}
	return payload
}

func kiroShadowWebFetchContentIsError(raw any) bool {
	content, _ := raw.(map[string]any)
	return strings.TrimSpace(kiroShadowStringField(content, "type")) == "web_fetch_tool_error"
}

func kiroShadowWebFetchURL(fetchResult *webfetch.FetchResult) string {
	if fetchResult == nil {
		return ""
	}
	if finalURL := strings.TrimSpace(fetchResult.FinalURL); finalURL != "" {
		return finalURL
	}
	return strings.TrimSpace(fetchResult.RequestedURL)
}

func kiroShadowWebFetchErrorCode(fetchErr *webfetch.FetchError) string {
	if fetchErr == nil {
		return "unavailable"
	}
	switch fetchErr.Code {
	case webfetch.ErrorCodeInvalidURL:
		return "invalid_input"
	case webfetch.ErrorCodeDomainBlocked:
		return "url_not_allowed"
	case webfetch.ErrorCodeHTTPStatus:
		if fetchErr.StatusCode == http.StatusTooManyRequests {
			return "too_many_requests"
		}
		return "url_not_accessible"
	case webfetch.ErrorCodeTooManyRedirects, webfetch.ErrorCodeRequestFailed, webfetch.ErrorCodeReadFailed:
		return "url_not_accessible"
	case webfetch.ErrorCodeClientConfig:
		return "unavailable"
	default:
		return "unavailable"
	}
}

func kiroStringArrayField(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			text, _ := item.(string)
			text = strings.TrimSpace(text)
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func kiroIntField(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return 0
	}
}

func (s *KiroGatewayService) kiroShadowAllowPrivateHosts() bool {
	if s == nil || s.settingService == nil || s.settingService.cfg == nil {
		return false
	}
	return s.settingService.cfg.Security.URLAllowlist.AllowPrivateHosts
}

func (s *KiroGatewayService) kiroShadowAllowInsecureHTTP() bool {
	if s == nil || s.settingService == nil || s.settingService.cfg == nil {
		return false
	}
	return s.settingService.cfg.Security.URLAllowlist.AllowInsecureHTTP
}

func kiroVisibleToolStateCounts(states map[string]*kiroToolState, order []string) (visible int, completed int, partial int) {
	for _, toolUseID := range order {
		state := states[toolUseID]
		if !kiroToolStateHasVisibleOutput(state) {
			continue
		}
		visible++
		if kiroToolStateIsComplete(state) {
			completed++
			continue
		}
		partial++
	}
	return visible, completed, partial
}

func kiroStopReasonFromContextUsage(usagePercent float64) string {
	if usagePercent >= 100 {
		return "model_context_window_exceeded"
	}
	return ""
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

func utf8SafePrefix(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if maxBytes >= len(s) {
		return s
	}
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}
	return s[:maxBytes]
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

func kiroSchedulingStatusCode(statusCode int, body []byte) int {
	if classifyKiroHTTPErrorSemantic(statusCode, body) == kiroHTTPErrorSemanticQuotaExhausted {
		return http.StatusTooManyRequests
	}
	return statusCode
}

func shouldKiroRetrySameAccount(statusCode int, headers http.Header, body []byte) bool {
	if statusCode == http.StatusPaymentRequired && classifyKiroHTTPErrorSemantic(statusCode, body) == kiroHTTPErrorSemanticQuotaExhausted {
		return false
	}
	if kiro429LooksQuotaExhausted(body) {
		return false
	}
	if parseKiro429ResetAt(headers, body) != nil {
		return false
	}
	return kiro429LooksShortBurst(body)
}

func kiro429LooksShortBurst(body []byte) bool {
	detail := strings.ToLower(strings.TrimSpace(kiroErrorDetailFromBody(body)))
	if detail == "" {
		detail = strings.ToLower(strings.TrimSpace(string(body)))
	}
	if detail == "" {
		return false
	}
	if strings.Contains(detail, "suspicious activity") {
		return false
	}
	return strings.Contains(detail, "too many requests")
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

func kiroAccountStateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(base, kiroAccountStateUpdateTimeout)
}

// handleKiroTransportError 处理上游 transport 层失败（DoWithTLS 直接返回 err）。
//
// 调用方在 kiro 流式入口和双请求路径上调用。函数职责：
//  1. 若是真正的客户端断开（gin context 已 Canceled），不记录 upstream ops、不标记账号、不 failover。
//  2. 否则视为上游 transport 故障：
//     - 调用 SetTempUnschedulable 给账号一个短冷却，防止反复撞同一个不可达上游。
//     - 返回 *UpstreamFailoverError 让 handler 主循环切到下一个账号。
//
// 注意：返回 failover 后 handler 会写最终响应，所以这里不能再 c.JSON。
func (s *KiroGatewayService) handleKiroTransportError(ctx context.Context, c *gin.Context, account *Account, upstreamURL string, transportErr error) error {
	if isClientDisconnectError(c, transportErr) {
		return transportErr
	}
	s.recordOpsRequestError(c, account, upstreamURL, transportErr)
	stateCtx, cancel := kiroAccountStateContext(ctx)
	defer cancel()
	s.markKiroFailureUnschedulable(stateCtx, account, http.StatusBadGateway, kiroTransportFailureReasonKeyword, sanitizeUpstreamErrorMessage(transportErr.Error()), kiroTransportFailureCooldown)
	return &UpstreamFailoverError{
		StatusCode:   http.StatusBadGateway,
		ResponseBody: []byte(sanitizeUpstreamErrorMessage(transportErr.Error())),
	}
}

// isClientDisconnectError 判断 transport err 是否由客户端取消触发。
//
// 仅当 gin 请求 context 被客户端取消，且 transport err 本身也是 cancellation 形态时才视作
// 客户端断开；不能因为请求 context 已取消就吞掉 connection reset / deadline 等上游故障。
func isClientDisconnectError(c *gin.Context, err error) bool {
	if c == nil || c.Request == nil {
		return false
	}
	reqCtx := c.Request.Context()
	if reqCtx == nil {
		return false
	}
	if !errors.Is(reqCtx.Err(), context.Canceled) {
		return false
	}
	return errors.Is(err, context.Canceled)
}

// markKiroFailureUnschedulable 把账号标记为短期临时不可调度，
// 让调度层在 cooldown 窗口内跳过这个上游故障的账号。
func (s *KiroGatewayService) markKiroFailureUnschedulable(ctx context.Context, account *Account, statusCode int, reasonKeyword string, message string, cooldown time.Duration) {
	if s == nil || s.rateLimitService == nil || s.rateLimitService.accountRepo == nil || account == nil || account.ID <= 0 || cooldown <= 0 {
		return
	}
	now := time.Now()
	until := now.Add(cooldown)
	reasonKeyword = strings.TrimSpace(reasonKeyword)
	if reasonKeyword == "" {
		reasonKeyword = kiroTransportFailureReasonKeyword
	}
	state := &TempUnschedState{
		UntilUnix:       until.Unix(),
		TriggeredAtUnix: now.Unix(),
		StatusCode:      statusCode,
		MatchedKeyword:  reasonKeyword,
		RuleIndex:       -1,
		ErrorMessage:    truncateTempUnschedMessage([]byte(sanitizeUpstreamErrorMessage(message)), tempUnschedMessageMaxBytes),
	}
	reason := ""
	if raw, marshalErr := json.Marshal(state); marshalErr == nil {
		reason = string(raw)
	}
	if reason == "" {
		reason = "Kiro upstream failure: " + state.ErrorMessage
	}

	if err := s.rateLimitService.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason); err != nil {
		slog.Warn("kiro_failure_set_temp_unsched_failed", "account_id", account.ID, "error", err)
		return
	}
	if s.rateLimitService.tempUnschedCache != nil {
		if err := s.rateLimitService.tempUnschedCache.SetTempUnsched(ctx, account.ID, state); err != nil {
			slog.Warn("kiro_failure_temp_unsched_cache_set_failed", "account_id", account.ID, "error", err)
		}
	}
	s.rateLimitService.notifyAccountSchedulingBlocked(account, until, reasonKeyword)
	slog.Warn("kiro_failure_temp_unschedulable",
		"account_id", account.ID,
		"account_name", account.Name,
		"until", until,
		"error", state.ErrorMessage,
		"status_code", statusCode,
		"reason_keyword", reasonKeyword,
	)
}

func (s *KiroGatewayService) newKiroPreStartStreamFailoverError(ctx context.Context, c *gin.Context, account *Account, upstreamRequestID string, headers http.Header, statusCode int, reasonKeyword string, message string, detail string) *UpstreamFailoverError {
	message = sanitizeUpstreamErrorMessage(strings.TrimSpace(message))
	if message == "" {
		message = "Kiro upstream disconnected before first forwardable event"
	}
	detail = strings.TrimSpace(detail)
	if detail == "" {
		detail = message
	}
	stateCtx, cancel := kiroAccountStateContext(ctx)
	defer cancel()
	s.markKiroFailureUnschedulable(stateCtx, account, statusCode, reasonKeyword, message, kiroTransportFailureCooldown)
	if c != nil {
		s.recordOpsErrorEvent(c, account, statusCode, upstreamRequestID, "", "failover", message, detail)
	}
	body, _ := json.Marshal(gin.H{
		"error": gin.H{
			"type":    "upstream_error",
			"message": message,
		},
	})
	var respHeaders http.Header
	if headers != nil {
		respHeaders = headers.Clone()
	} else {
		respHeaders = http.Header{}
	}
	return &UpstreamFailoverError{
		StatusCode:      statusCode,
		ResponseBody:    body,
		ResponseHeaders: respHeaders,
	}
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
		if statusCode >= http.StatusInternalServerError {
			stateCtx, cancel := kiroAccountStateContext(ctx)
			s.markKiroFailureUnschedulable(stateCtx, account, statusCode, kiroTransportFailureReasonKeyword, sanitizeUpstreamErrorMessage(failureErr.Error()), kiroTransportFailureCooldown)
			cancel()
		}
		return &UpstreamFailoverError{
			StatusCode:      statusCode,
			ResponseBody:    body,
			ResponseHeaders: kiroFrameFailureHeaders(upstreamRequestID),
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
	s.recordOpsErrorEvent(c, account, 0, "", "", "request_error", err.Error(), kiroProtocolErrorDetail(c))
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
	if err := writeSSEEvent(writer, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            msgID,
			"type":          "message",
			"role":          "assistant",
			"model":         model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage":         kiroAnthropicUsageForStart(fakeCacheUsage.InputTokens, 1, fakeCacheUsage),
		},
	}); err != nil {
		return err
	}
	return writeSSEEvent(writer, "ping", map[string]any{"type": "ping"})
}

func kiroFrameFailureHeaders(upstreamRequestID string) http.Header {
	headers := http.Header{}
	if requestID := strings.TrimSpace(upstreamRequestID); requestID != "" {
		headers.Set("x-amzn-requestid", requestID)
	}
	return headers
}

func closeOpenKiroBlocks(writer gin.ResponseWriter, textBlockOpen bool, textBlockIndex int, thinkingBlockOpen bool, thinkingBlockIndex int, toolStates map[string]*kiroToolState) error {
	if textBlockOpen {
		if err := writeSSEEvent(writer, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": textBlockIndex,
		}); err != nil {
			return err
		}
	}
	if thinkingBlockOpen {
		if err := writeKiroThinkingBlockStop(writer, thinkingBlockIndex); err != nil {
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
	if !writer.Written() {
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Header().Set("Cache-Control", "no-cache")
		writer.Header().Set("Connection", "keep-alive")
		writer.Header().Set("X-Accel-Buffering", "no")
		writer.WriteHeader(http.StatusOK)
	}
	return writeSSEEvent(writer, "error", map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "api_error",
			"message": message,
		},
	})
}

func kiroIncompleteToolUseClientMessage() string {
	return "Kiro upstream returned incomplete tool_use output; retry the request to continue"
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

func kiroEmptyOutputClientMessage(contextUsagePercentage *float64) string {
	if contextUsagePercentage == nil {
		return "kiro response contained no assistant output"
	}
	if *contextUsagePercentage >= 95 {
		return fmt.Sprintf("kiro response contained no assistant output after context usage reached %.0f%%", *contextUsagePercentage)
	}
	return "kiro response contained no assistant output"
}

func setKiroContextUsagePercentage(c *gin.Context, value float64) {
	if c == nil || value < 0 {
		return
	}
	c.Set(kiroContextUsagePercentKey, value)
}

func kiroProtocolErrorDetail(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, exists := c.Get(kiroContextUsagePercentKey)
	if !exists {
		return ""
	}
	switch typed := value.(type) {
	case float64:
		return fmt.Sprintf("context_usage_percentage=%.2f", typed)
	case float32:
		return fmt.Sprintf("context_usage_percentage=%.2f", typed)
	case int:
		return fmt.Sprintf("context_usage_percentage=%d", typed)
	case int64:
		return fmt.Sprintf("context_usage_percentage=%d", typed)
	default:
		return ""
	}
}

func numericField(obj map[string]any, key string) (float64, bool) {
	if obj == nil {
		return 0, false
	}
	switch value := obj[key].(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int32:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func kiroLogger(ctx context.Context, account *Account) *zap.Logger {
	fields := []zap.Field{zap.String("platform", PlatformKiro)}
	if account != nil {
		fields = append(fields,
			zap.Int64("account_id", account.ID),
			zap.String("account_type", account.Type),
		)
	}
	return logger.FromContext(ctx).With(fields...)
}

func logKiroPreparedRequest(ctx context.Context, account *Account, parsed *ParsedRequest, converted *kiropkg.ConvertResult, billedInputTokens int, meta *kiroPreparedRequestMeta) {
	if parsed == nil || converted == nil || meta == nil {
		return
	}
	_, requestedHadVariant := stripKiroModelVariantSuffixes(strings.TrimSpace(parsed.Model))
	_, resolvedHadVariant := stripKiroModelVariantSuffixes(strings.TrimSpace(converted.RequestedModel))
	kiroLogger(ctx, account).Info(
		"kiro.request_prepared",
		zap.String("requested_model", parsed.Model),
		zap.String("resolved_requested_model", converted.RequestedModel),
		zap.String("upstream_model", converted.Model),
		zap.Bool("stream", parsed.Stream),
		zap.Bool("thinking_enabled", parsed.ThinkingEnabled),
		zap.Bool("requested_model_had_variant_suffix", requestedHadVariant),
		zap.Bool("resolved_model_had_variant_suffix", resolvedHadVariant),
		zap.Bool("supports_one_million_context", kiropkg.SupportsOneMillionContextModel(converted.RequestedModel)),
		zap.Bool("mapping_changed_requested_model", !strings.EqualFold(strings.TrimSpace(parsed.Model), strings.TrimSpace(converted.RequestedModel))),
		zap.Bool("has_metadata_user_id", strings.TrimSpace(parsed.MetadataUserID) != ""),
		zap.Int("billed_input_tokens", billedInputTokens),
		zap.Int("forward_input_tokens", meta.ForwardInputTokens),
		zap.Int("context_budget_tokens", meta.ContextBudgetTokens),
		zap.Int("tool_count", meta.ToolCount),
		zap.Bool("compacted", meta.Compacted),
		zap.Int("dropped_messages", meta.DroppedMessages),
		zap.Bool("promoted_context_window", meta.PromotedContextWindow),
	)
}

func logKiroFakeCachePlan(ctx context.Context, account *Account, parsed *ParsedRequest, plan *kiropkg.FakeCachePlan, hit kiropkg.FakeCacheHitState) {
	if parsed == nil || plan == nil {
		return
	}
	kiroLogger(ctx, account).Info(
		"kiro.fake_cache_plan",
		zap.String("requested_model", parsed.Model),
		zap.Bool("independent_hit", hit.Independent),
		zap.Bool("prefix_hit", hit.Prefix),
		zap.Int("checkpoint_hit_tokens", hit.CheckpointTokens),
		zap.Int("checkpoint_count", len(plan.Checkpoints)),
		zap.Int("current_checkpoint_tokens", plan.CurrentCheckpointTokens()),
		zap.Int("independent_cacheable_tokens", plan.IndependentCacheableTokens),
		zap.Int("previous_prefix_cacheable_tokens", plan.PreviousPrefixCacheableTokens),
		zap.Int("prefix_cacheable_tokens", plan.CurrentPrefixCacheableTokens),
		zap.Int("previous_cacheable_tokens", plan.PreviousCacheableTokens),
		zap.Int("current_cacheable_tokens", plan.CurrentCacheableTokens),
	)
}

func logKiroResponseAnomaly(ctx context.Context, account *Account, parsed *ParsedRequest, stream bool, kind string, err error, framesSeen int, toolUseCount int, contextUsagePercentage *float64) {
	fields := []zap.Field{
		zap.Bool("stream", stream),
		zap.String("kind", kind),
		zap.Int("frames_seen", framesSeen),
		zap.Int("tool_use_count", toolUseCount),
	}
	if parsed != nil {
		fields = append(fields, zap.String("requested_model", parsed.Model))
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	if contextUsagePercentage != nil {
		fields = append(fields, zap.Float64("context_usage_percentage", *contextUsagePercentage))
	}
	kiroLogger(ctx, account).Warn("kiro.response_anomaly", fields...)
}

func (t *kiroResponseTelemetry) shortOutput(outputTokens int) bool {
	if t == nil {
		return false
	}
	return outputTokens > 0 && outputTokens <= kiroShortOutputTokenThreshold
}

func (t *kiroResponseTelemetry) anomalyKinds(stopReason string) []string {
	if t == nil {
		return nil
	}
	kinds := make([]string, 0, 3)
	if t.PartialToolUseCount > 0 && t.CompletedToolUseCount == 0 {
		kinds = append(kinds, "incomplete_tool_use_completed")
	}
	if stopReason == "model_context_window_exceeded" {
		kinds = append(kinds, "context_window_exceeded")
	}
	return kinds
}

func (t *kiroResponseTelemetry) completionKinds() []string {
	if t == nil {
		return nil
	}
	kinds := make([]string, 0, 4)
	if t.AssistantChars > 0 {
		kinds = append(kinds, "text")
	}
	if t.NativeThinkingChars > 0 {
		kinds = append(kinds, "native_thinking")
	}
	if t.ToolUseCount > 0 {
		kinds = append(kinds, "tool_use")
	}
	return kinds
}

func (t *kiroResponseTelemetry) opsDetail(stopReason string, outputTokens int, shortOutput bool, anomalyKinds []string) string {
	if t == nil {
		return ""
	}
	payload := map[string]any{
		"frames_seen":              t.FramesSeen,
		"assistant_chars":          t.AssistantChars,
		"native_thinking_chars":    t.NativeThinkingChars,
		"tool_use_count":           t.ToolUseCount,
		"completed_tool_use_count": t.CompletedToolUseCount,
		"partial_tool_use_count":   t.PartialToolUseCount,
		"stop_reason":              stopReason,
		"output_tokens":            outputTokens,
		"short_output":             shortOutput,
		"completion_kinds":         t.completionKinds(),
		"anomaly_kinds":            anomalyKinds,
	}
	if t.ContextUsagePercentage != nil {
		payload["context_usage_percentage"] = *t.ContextUsagePercentage
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (s *KiroGatewayService) recordKiroSuccessfulAnomalies(ctx context.Context, c *gin.Context, account *Account, parsed *ParsedRequest, upstreamRequestID string, stream bool, outputTokens int, stopReason string, telemetry *kiroResponseTelemetry) {
	if telemetry == nil {
		return
	}
	anomalyKinds := telemetry.anomalyKinds(stopReason)
	shortOutput := telemetry.shortOutput(outputTokens)
	if len(anomalyKinds) == 0 && !shortOutput {
		return
	}
	detail := telemetry.opsDetail(stopReason, outputTokens, shortOutput, anomalyKinds)
	for _, kind := range anomalyKinds {
		logKiroResponseAnomaly(ctx, account, parsed, stream, kind, nil, telemetry.FramesSeen, telemetry.ToolUseCount, telemetry.ContextUsagePercentage)
	}
	if shortOutput {
		logKiroResponseAnomaly(ctx, account, parsed, stream, "short_output", nil, telemetry.FramesSeen, telemetry.ToolUseCount, telemetry.ContextUsagePercentage)
	}
	if len(anomalyKinds) == 0 || c == nil || account == nil {
		return
	}
	messageParts := append([]string{}, anomalyKinds...)
	if shortOutput {
		messageParts = append(messageParts, "short_output")
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:          account.Platform,
		AccountID:         account.ID,
		AccountName:       account.Name,
		UpstreamRequestID: strings.TrimSpace(upstreamRequestID),
		Kind:              "response_anomaly",
		Message:           "kiro completed with anomalies: " + strings.Join(messageParts, ","),
		Detail:            detail,
	})
}

func logKiroRequestCompleted(ctx context.Context, account *Account, parsed *ParsedRequest, upstreamRequestID string, upstreamModel string, stream bool, duration time.Duration, firstTokenMs *int, inputTokens int, outputTokens int, cacheCreationTokens int, cacheReadTokens int, stopReason string, toolNames []string, telemetry *kiroResponseTelemetry) {
	fields := []zap.Field{
		zap.Bool("stream", stream),
		zap.String("upstream_request_id", strings.TrimSpace(upstreamRequestID)),
		zap.String("upstream_model", upstreamModel),
		zap.Duration("duration", duration),
		zap.Int("input_tokens", inputTokens),
		zap.Int("output_tokens", outputTokens),
		zap.Int("cache_creation_input_tokens", cacheCreationTokens),
		zap.Int("cache_read_input_tokens", cacheReadTokens),
		zap.String("stop_reason", stopReason),
		zap.Int("tool_use_count", len(toolNames)),
		zap.Strings("tool_names", toolNames),
	}
	if parsed != nil {
		fields = append(fields, zap.String("requested_model", parsed.Model))
	}
	if firstTokenMs != nil {
		fields = append(fields, zap.Int("first_token_ms", *firstTokenMs))
	}
	if telemetry != nil {
		fields = append(fields,
			zap.Int("frames_seen", telemetry.FramesSeen),
			zap.Int("assistant_chars", telemetry.AssistantChars),
			zap.Int("native_thinking_chars", telemetry.NativeThinkingChars),
			zap.Int("completed_tool_use_count", telemetry.CompletedToolUseCount),
			zap.Int("partial_tool_use_count", telemetry.PartialToolUseCount),
			zap.Strings("completion_kinds", telemetry.completionKinds()),
			zap.Bool("short_output", telemetry.shortOutput(outputTokens)),
			zap.Strings("anomaly_kinds", telemetry.anomalyKinds(stopReason)),
		)
		if telemetry.ContextUsagePercentage != nil {
			fields = append(fields, zap.Float64("context_usage_percentage", *telemetry.ContextUsagePercentage))
		}
	}
	kiroLogger(ctx, account).Info("kiro.request_completed", fields...)
}

// logKiroFrameDiagnostic emits a structured info log for every upstream Kiro
// frame so unfamiliar event types (e.g. reasoning frames produced by Adaptive
// Thinking on 4.7/4.8) can be identified without dumping payload values.
func logKiroFrameDiagnostic(ctx context.Context, account *Account, parsed *ParsedRequest, frame *kiroFrame) {
	if frame == nil {
		return
	}
	keys := make([]string, 0, len(frame.Payload))
	for key := range frame.Payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	model := ""
	if parsed != nil {
		model = parsed.Model
	}
	kiroLogger(ctx, account).Info(
		"kiro.frame_received",
		zap.String("event_type", frame.EventType),
		zap.String("message_type", frame.MessageType),
		zap.String("requested_model", model),
		zap.Strings("payload_keys", keys),
	)
}

// emitGatewayDebugUpstreamRequest writes the Kiro upstream request body to the
// debug timeline (when capture is enabled) so operators can inspect tool
// definitions, fake-cache plans and Adaptive Thinking parameters end-to-end.
func (s *KiroGatewayService) emitGatewayDebugUpstreamRequest(c *gin.Context, account *Account, upstreamReq *http.Request, body []byte, attempt int) {
	if s == nil || s.settingService == nil || c == nil || c.Request == nil {
		return
	}
	if !GatewayDebugTimelineEnabled(c.Request.Context(), s.settingService) {
		return
	}
	platform := ""
	accountID := int64(0)
	if account != nil {
		platform = account.Platform
		accountID = account.ID
	}
	if !shouldRecordGatewayDebugBodyForPlatform(platform) {
		return
	}
	endpoint := ""
	method := ""
	contentType := ""
	if upstreamReq != nil {
		if upstreamReq.URL != nil {
			endpoint = safeUpstreamURL(upstreamReq.URL.String())
		}
		method = upstreamReq.Method
		contentType = upstreamReq.Header.Get("Content-Type")
	}
	fields := map[string]any{
		"component":         "gateway_debug_timeline",
		"platform":          platform,
		"account_id":        accountID,
		"upstream_endpoint": endpoint,
		"upstream_method":   method,
		"attempt":           attempt,
	}
	RecordGatewayDebugTimelineBody(s.settingService, c, "upstream_request_body", body, contentType, fields)
}
