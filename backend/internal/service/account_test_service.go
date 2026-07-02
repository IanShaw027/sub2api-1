package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	pkglogger "github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// sseDataPrefix matches SSE data lines with optional whitespace after colon.
// Some upstream APIs return non-standard "data:" without space (should be "data: ").
var sseDataPrefix = regexp.MustCompile(`^data:\s*`)
var accountTestReturnedStatusCodePattern = regexp.MustCompile(`\breturned\s+(\d{3})\b`)

const (
	testClaudeAPIURL   = "https://api.anthropic.com/v1/messages?beta=true"
	chatgptCodexAPIURL = "https://chatgpt.com/backend-api/codex/responses"
)

// TestEvent represents a SSE event for account testing
type TestEvent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Model    string `json:"model,omitempty"`
	Status   string `json:"status,omitempty"`
	Code     string `json:"code,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Data     any    `json:"data,omitempty"`
	Success  bool   `json:"success,omitempty"`
	Error    string `json:"error,omitempty"`
}

const (
	defaultGeminiTextTestPrompt  = "hi"
	defaultGeminiImageTestPrompt = "Generate a cute orange cat astronaut sticker on a clean pastel background."
	defaultOpenAIImageTestPrompt = "Generate a cute orange cat astronaut sticker on a clean pastel background."
	openAIImageTestModeCodex     = "codex"
)

const (
	// AccountTestContextRequestedModeKey stores optional request test_mode on gin context.
	AccountTestContextRequestedModeKey = "account_test_requested_mode"
	accountTestOpsAccountIDKey         = "account_test_ops_account_id"
	accountTestOpsPlatformKey          = "account_test_ops_platform"
	accountTestOpsTypeKey              = "account_test_ops_type"
	accountTestOpsNameKey              = "account_test_ops_name"
	accountTestOpsModelKey             = "account_test_ops_model"
)

// isOpenAIImageModel checks if the model is an OpenAI image generation model (e.g. gpt-image-2).
func isOpenAIImageModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(model), "gpt-image-")
}

func NormalizeOpenAIImageTestMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case openAIImageTestModeCodex, "image_api", "images_api", "images-api", "openai_images_api", "openai-images-api":
		return openAIImageTestModeCodex
	default:
		return ""
	}
}

func resolveOpenAIImageExecutionMode(c *gin.Context) string {
	if c != nil {
		if raw, ok := c.Get(AccountTestContextRequestedModeKey); ok {
			if mode, ok := raw.(string); ok {
				if normalized := NormalizeOpenAIImageTestMode(mode); normalized != "" {
					return normalized
				}
			}
		}
	}
	return openAIImageTestModeCodex
}

// AccountTestService handles account testing operations
type geminiAccountAccessTokenProvider interface {
	GetAccessToken(context.Context, *Account) (string, error)
}

type AccountTestService struct {
	accountRepo               AccountRepository
	geminiTokenProvider       geminiAccountAccessTokenProvider
	kiroTokenProvider         *KiroTokenProvider
	claudeTokenProvider       *ClaudeTokenProvider
	grokTokenProvider         *GrokTokenProvider
	antigravityGatewayService *AntigravityGatewayService
	httpUpstream              HTTPUpstream
	cfg                       *config.Config
	tlsFPProfileService       *TLSFingerprintProfileService
	settingService            *SettingService
	opsService                *OpsService
}

func (s *AccountTestService) doUpstreamWithTLS(c *gin.Context, req *http.Request, account *Account, proxyURL string, profile *tlsfingerprint.Profile) (*http.Response, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, errors.New("http upstream is not configured")
	}
	upstreamStart := time.Now()
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, profile)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	return resp, err
}

// NewAccountTestService creates a new AccountTestService
func NewAccountTestService(
	accountRepo AccountRepository,
	geminiTokenProvider geminiAccountAccessTokenProvider,
	kiroTokenProvider *KiroTokenProvider,
	claudeTokenProvider *ClaudeTokenProvider,
	grokTokenProvider *GrokTokenProvider,
	antigravityGatewayService *AntigravityGatewayService,
	httpUpstream HTTPUpstream,
	cfg *config.Config,
	tlsFPProfileService *TLSFingerprintProfileService,
	settingService *SettingService,
	opsService *OpsService,
) *AccountTestService {
	return &AccountTestService{
		accountRepo:               accountRepo,
		geminiTokenProvider:       geminiTokenProvider,
		kiroTokenProvider:         kiroTokenProvider,
		claudeTokenProvider:       claudeTokenProvider,
		grokTokenProvider:         grokTokenProvider,
		antigravityGatewayService: antigravityGatewayService,
		httpUpstream:              httpUpstream,
		cfg:                       cfg,
		tlsFPProfileService:       tlsFPProfileService,
		settingService:            settingService,
		opsService:                opsService,
	}
}

func (s *AccountTestService) validateUpstreamBaseURL(raw string) (string, error) {
	if s.cfg == nil {
		return "", errors.New("config is not available")
	}
	if !s.cfg.Security.URLAllowlist.Enabled {
		return urlvalidator.ValidateURLFormat(raw, s.cfg.Security.URLAllowlist.AllowInsecureHTTP)
	}
	normalized, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{
		AllowedHosts:     s.cfg.Security.URLAllowlist.UpstreamHosts,
		RequireAllowlist: true,
		AllowPrivate:     s.cfg.Security.URLAllowlist.AllowPrivateHosts,
	})
	if err != nil {
		return "", err
	}
	return normalized, nil
}

// generateSessionString generates a Claude Code style session string.
// The output format is determined by the UA version in claude.DefaultHeaders,
// ensuring consistency between the user_id format and the UA sent to upstream.
func generateSessionString() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	hex64 := hex.EncodeToString(b)
	sessionUUID := uuid.New().String()
	uaVersion := ExtractCLIVersion(claude.DefaultHeaders["User-Agent"])
	return FormatMetadataUserID(hex64, "", sessionUUID, uaVersion), nil
}

// createTestPayload creates a Claude Code style test request payload
func createTestPayload(modelID string) (map[string]any, error) {
	sessionID, err := generateSessionString()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"model": modelID,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "hi",
						"cache_control": map[string]string{
							"type": "ephemeral",
						},
					},
				},
			},
		},
		"system": []map[string]any{
			{
				"type": "text",
				"text": claudeCodeSystemPrompt,
				"cache_control": map[string]string{
					"type": "ephemeral",
				},
			},
		},
		"metadata": map[string]string{
			"user_id": sessionID,
		},
		"max_tokens":  1024,
		"temperature": 1,
		"stream":      true,
	}, nil
}

// TestAccountConnection tests an account's connection by sending a test request
// All account types use full Claude Code client characteristics, only auth header differs
// modelID is optional - if empty, defaults to claude.DefaultTestModel
// mode is optional - "compact" routes OpenAI accounts to the /responses/compact probe path
func (s *AccountTestService) TestAccountConnection(c *gin.Context, accountID int64, modelID string, prompt string, mode string) error {
	ctx := c.Request.Context()
	requestStart := time.Now()

	// Get account
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		sendErr := s.sendErrorAndEnd(c, "Account not found")
		if errors.Is(err, ErrAccountNotFound) {
			return fmt.Errorf("%w: %v", ErrAccountNotFound, sendErr)
		}
		return sendErr
	}
	c.Set(accountTestOpsAccountIDKey, account.ID)
	c.Set(accountTestOpsPlatformKey, account.Platform)
	c.Set(accountTestOpsTypeKey, account.Type)
	c.Set(accountTestOpsNameKey, account.Name)
	if trimmedModelID := strings.TrimSpace(modelID); trimmedModelID != "" {
		c.Set(accountTestOpsModelKey, trimmedModelID)
	}
	SetOpsLatencyMs(c, OpsAuthLatencyMsKey, time.Since(requestStart).Milliseconds())

	// Route to platform-specific test method
	if account.IsOpenAI() {
		return s.testOpenAIAccountConnection(c, account, modelID, prompt, normalizeAccountTestMode(mode))
	}

	if account.IsGemini() {
		return s.testGeminiAccountConnection(c, account, modelID, prompt)
	}

	if account.Platform == PlatformKiro {
		return s.testKiroAccountConnection(c, account, modelID)
	}

	if account.Platform == PlatformGrok {
		return s.testGrokAccountConnection(c, account, modelID, prompt)
	}

	if account.Platform == PlatformAntigravity {
		return s.routeAntigravityTest(c, account, modelID, prompt)
	}

	return s.testClaudeAccountConnection(c, account, modelID)
}

func setAccountTestOpsModelIfMissing(c *gin.Context, modelID string) {
	if c == nil {
		return
	}
	if accountTestOpsContextString(c, accountTestOpsModelKey) != "" {
		return
	}
	if trimmed := strings.TrimSpace(modelID); trimmed != "" {
		c.Set(accountTestOpsModelKey, trimmed)
	}
}

func (s *AccountTestService) testKiroAccountConnection(c *gin.Context, account *Account, modelID string) error {
	ctx := c.Request.Context()
	testModelID := modelID
	if strings.TrimSpace(testModelID) == "" {
		testModelID = "claude-sonnet-4-5-20250929"
	}
	setAccountTestOpsModelIfMissing(c, testModelID)
	convertedModelID, err := resolveKiroRequestedModel(account, testModelID)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported Kiro model: %s", testModelID))
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	accessToken := account.GetCredential("access_token")
	if account.Type == AccountTypeAPIKey {
		accessToken = account.GetCredential("api_key")
	}
	expiresAt := account.GetCredentialAsTime("expires_at")
	if account.Type == AccountTypeOAuth && (accessToken == "" || expiresAt == nil || expiresAt.Before(time.Now().Add(3*time.Minute))) {
		if s.kiroTokenProvider == nil {
			return s.sendErrorAndEnd(c, "Kiro token provider is not configured")
		}
		refreshedToken, err := s.kiroTokenProvider.GetAccessToken(ctx, account)
		if err != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to refresh Kiro token: %s", err.Error()))
		}
		accessToken = refreshedToken
	}
	if accessToken == "" {
		return s.sendErrorAndEnd(c, "No Kiro access token available")
	}

	payload := map[string]any{
		"model": convertedModelID,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "hi"},
				},
			},
		},
		"max_tokens": 128,
		"stream":     true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create Kiro test payload")
	}
	body = injectKiroProfileARNIntoAnthropicBody(body, account)
	converted, err := kiropkg.ConvertAnthropicRequestWithModel(body, convertedModelID)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to convert Kiro payload: %s", err.Error()))
	}

	req, err := buildKiroGenerateAssistantRequest(ctx, account, converted.Body, accessToken, s.resolveKiroRuntimeSettings(ctx))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create Kiro request")
	}

	resp, err := s.doUpstreamWithTLS(c, req, account, accountProxyURL(account), resolveKiroTLSProfile(account, s.tlsFPProfileService))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Kiro request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Kiro API returned %d", resp.StatusCode))
		}
		return s.sendErrorAndEnd(c, kiroHTTPStatusErrorMessage("Kiro API", resp.StatusCode, body))
	}
	frames, err := readAllKiroFrames(resp.Body)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to decode Kiro response: %s", err.Error()))
	}
	for _, frame := range frames {
		if failureErr := kiroFrameFailure(frame); failureErr != nil {
			return s.sendErrorAndEnd(c, failureErr.Error())
		}
	}

	assistantText, hasAssistantResponse := collectKiroAssistantResponseText(frames)
	if !hasAssistantResponse {
		return s.sendErrorAndEnd(c, "Kiro upstream returned no assistant content")
	}
	if trimmed := strings.TrimSpace(assistantText); trimmed != "" {
		s.sendEvent(c, TestEvent{Type: "content", Text: trimmed})
	}
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

func (s *AccountTestService) resolveKiroRuntimeSettings(ctx context.Context) *KiroRuntimeSettings {
	if s != nil && s.settingService != nil {
		return s.settingService.GetKiroRuntimeSettings(ctx)
	}
	return DefaultKiroRuntimeSettings()
}

func (s *AccountTestService) testGrokAccountConnection(c *gin.Context, account *Account, modelID string, prompt string) error {
	ctx := c.Request.Context()

	testModelID := strings.TrimSpace(modelID)
	if testModelID == "" {
		testModelID = "grok-4.3"
	}
	setAccountTestOpsModelIfMissing(c, testModelID)
	upstreamModel := account.GetMappedModel(testModelID)
	if strings.TrimSpace(upstreamModel) == "" {
		upstreamModel = testModelID
	}

	var authToken string
	switch account.Type {
	case AccountTypeOAuth:
		if s.grokTokenProvider != nil {
			refreshedToken, err := s.grokTokenProvider.GetAccessToken(ctx, account)
			if err != nil {
				return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to get Grok access token: %s", err.Error()))
			}
			authToken = refreshedToken
		} else {
			authToken = account.GetGrokAccessToken()
		}
		if strings.TrimSpace(authToken) == "" {
			return s.sendErrorAndEnd(c, "No Grok access token available")
		}
	case AccountTypeAPIKey:
		authToken = account.GetCredential("api_key")
		if strings.TrimSpace(authToken) == "" {
			return s.sendErrorAndEnd(c, "No Grok API key available")
		}
	default:
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported Grok account type: %s", account.Type))
	}

	return s.testOpenAIResponsesLikeAccountConnection(
		c,
		ctx,
		account,
		upstreamModel,
		prompt,
		authToken,
		account.GetGrokBaseURL(),
		"grok",
		true,
	)
}

func (s *AccountTestService) reconcileGrokTestState(ctx context.Context, account *Account, statusCode int, headers http.Header) {
	if s == nil || s.accountRepo == nil || account == nil {
		return
	}
	if snapshot := xai.ParseQuotaHeaders(headers, statusCode); snapshot != nil {
		_ = s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{
			grokQuotaSnapshotExtraKey: snapshot,
		})
	}

	var cooldown time.Duration
	reason := ""
	switch statusCode {
	case http.StatusUnauthorized:
		cooldown = 10 * time.Minute
		reason = "grok oauth token unauthorized"
	case http.StatusForbidden:
		cooldown = 30 * time.Minute
		reason = "grok entitlement or subscription tier denied"
	case http.StatusTooManyRequests:
		cooldown = 2 * time.Minute
		if snapshot := xai.ParseQuotaHeaders(headers, statusCode); snapshot != nil && snapshot.RetryAfterSeconds != nil && *snapshot.RetryAfterSeconds > 0 {
			cooldown = time.Duration(*snapshot.RetryAfterSeconds) * time.Second
		}
		reason = "grok rate limited"
	default:
		if statusCode >= 500 {
			cooldown = 2 * time.Minute
			reason = "grok upstream temporary error"
		}
	}
	if cooldown <= 0 || strings.TrimSpace(reason) == "" {
		return
	}
	until := time.Now().Add(cooldown)
	if account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(until) {
		until = *account.TempUnschedulableUntil
	}
	_ = s.accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason)
}

// testClaudeAccountConnection tests an Anthropic Claude account's connection
func (s *AccountTestService) testClaudeAccountConnection(c *gin.Context, account *Account, modelID string) error {
	ctx := c.Request.Context()

	// Determine the model to use
	testModelID := modelID
	if testModelID == "" {
		testModelID = claude.DefaultTestModel
	}
	setAccountTestOpsModelIfMissing(c, testModelID)

	// API Key 账号测试连接时也需要应用通配符模型映射。
	if account.Type == "apikey" {
		testModelID = account.GetMappedModel(testModelID)
	}

	// Bedrock accounts use a separate test path
	if account.IsBedrock() {
		return s.testBedrockAccountConnection(c, ctx, account, testModelID)
	}
	if account.Type == AccountTypeServiceAccount {
		return s.testClaudeVertexServiceAccountConnection(c, ctx, account, testModelID)
	}

	// Determine authentication method and API URL
	var authToken string
	var useBearer bool
	var apiURL string

	if account.IsOAuth() {
		// OAuth or Setup Token - use Bearer token
		useBearer = true
		apiURL = testClaudeAPIURL
		authToken = account.GetCredential("access_token")
		if authToken == "" {
			return s.sendErrorAndEnd(c, "No access token available")
		}
	} else if account.Type == "apikey" {
		// API Key - use x-api-key header
		useBearer = false
		authToken = account.GetCredential("api_key")
		if authToken == "" {
			return s.sendErrorAndEnd(c, "No API key available")
		}

		baseURL := account.GetBaseURL()
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid base URL: %s", err.Error()))
		}
		apiURL = strings.TrimSuffix(normalizedBaseURL, "/") + "/v1/messages?beta=true"
	} else {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported account type: %s", account.Type))
	}

	// Set SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	// Create Claude Code style payload (same for all account types)
	payload, err := createTestPayload(testModelID)
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create test payload")
	}
	payloadBytes, _ := json.Marshal(payload)

	// Send test_start event
	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create request")
	}

	// Set common headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	// Set authentication header
	if useBearer {
		// OAuth accounts: apply Claude Code mimicry headers
		for key, value := range claude.DefaultHeaders {
			req.Header.Set(key, value)
		}
		req.Header.Set("anthropic-beta", claude.DefaultBetaHeader)
		req.Header.Set("Authorization", "Bearer "+authToken)
	} else {
		// API key accounts: no Claude Code mimicry
		req.Header.Set("anthropic-beta", claude.APIKeyBetaHeader)
		req.Header.Set("x-api-key", authToken)
	}

	// Get proxy URL
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.doUpstreamWithTLS(c, req, account, proxyURL, s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http"))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body))

		// 403 表示账号被上游封禁，标记为 error 状态
		if resp.StatusCode == http.StatusForbidden {
			_ = s.accountRepo.SetError(ctx, account.ID, errMsg)
		}

		return s.sendErrorAndEnd(c, errMsg)
	}

	// Process SSE stream
	return s.processClaudeStream(c, resp.Body)
}

func (s *AccountTestService) testClaudeVertexServiceAccountConnection(c *gin.Context, ctx context.Context, account *Account, testModelID string) error {
	if mappedModel, matched := account.ResolveMappedModel(testModelID); matched {
		testModelID = mappedModel
	} else {
		testModelID = normalizeVertexAnthropicModelID(claude.NormalizeModelID(testModelID))
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	payload, err := createTestPayload(testModelID)
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create test payload")
	}
	payloadBytes, _ := json.Marshal(payload)
	vertexBody, err := buildVertexAnthropicRequestBody(payloadBytes)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to create Vertex request body: %s", err.Error()))
	}

	if s.claudeTokenProvider == nil {
		return s.sendErrorAndEnd(c, "Claude token provider not configured")
	}
	accessToken, err := s.claudeTokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to get service account access token: %s", err.Error()))
	}

	fullURL, err := buildVertexAnthropicURL(account.VertexProjectID(), account.VertexLocation(testModelID), testModelID, true)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to build Vertex URL: %s", err.Error()))
	}

	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(vertexBody))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http"))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body))
		if resp.StatusCode == http.StatusForbidden {
			_ = s.accountRepo.SetError(ctx, account.ID, errMsg)
		}
		return s.sendErrorAndEnd(c, errMsg)
	}

	return s.processClaudeStream(c, resp.Body)
}

// testBedrockAccountConnection tests a Bedrock (SigV4 or API Key) account using non-streaming invoke
func (s *AccountTestService) testBedrockAccountConnection(c *gin.Context, ctx context.Context, account *Account, testModelID string) error {
	region := bedrockRuntimeRegion(account)
	resolvedModelID, ok := ResolveBedrockModelID(account, testModelID)
	if !ok {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported Bedrock model: %s", testModelID))
	}
	testModelID = resolvedModelID

	// Set SSE headers (test UI expects SSE)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	// Create a minimal Bedrock-compatible payload (no stream, no cache_control)
	bedrockPayload := map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "text",
						"text": "hi",
					},
				},
			},
		},
		"max_tokens":  256,
		"temperature": 1,
	}
	bedrockBody, _ := json.Marshal(bedrockPayload)

	// Use non-streaming endpoint (response is standard Claude JSON)
	apiURL := BuildBedrockURL(region, testModelID, false)

	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bedrockBody))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")

	// Sign or set auth based on account type
	if account.IsBedrockAPIKey() {
		apiKey := account.GetCredential("api_key")
		if apiKey == "" {
			return s.sendErrorAndEnd(c, "No API key available")
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		signer, err := NewBedrockSignerFromAccount(account)
		if err != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to create Bedrock signer: %s", err.Error()))
		}
		if err := signer.SignRequest(ctx, req, bedrockBody); err != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to sign request: %s", err.Error()))
		}
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.doUpstreamWithTLS(c, req, account, proxyURL, nil)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return s.sendErrorAndEnd(c, fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)))
	}

	// Bedrock non-streaming response is standard Claude JSON, extract the text
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to parse response: %s", err.Error()))
	}

	text := ""
	if len(result.Content) > 0 {
		text = result.Content[0].Text
	}
	if text == "" {
		text = "(empty response)"
	}

	s.sendEvent(c, TestEvent{Type: "content", Text: text})
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

// testOpenAIAccountConnection tests an OpenAI account's connection
func (s *AccountTestService) testOpenAIAccountConnection(c *gin.Context, account *Account, modelID string, prompt string, mode string) error {
	ctx := c.Request.Context()
	mode = normalizeAccountTestMode(mode)

	// Default to openai.DefaultTestModel for OpenAI testing
	testModelID := modelID
	if testModelID == "" {
		testModelID = openai.DefaultTestModel
	}
	setAccountTestOpsModelIfMissing(c, testModelID)

	// Align test routing with gateway behavior: OpenAI accounts apply normal
	// account model mapping, and compact mode applies compact-only mapping on top.
	testModelID = account.GetMappedModel(testModelID)

	// Route to image generation test if an image model is selected
	if isOpenAIImageModel(testModelID) {
		imagePrompt := strings.TrimSpace(prompt)
		if imagePrompt == "" {
			imagePrompt = defaultOpenAIImageTestPrompt
		}
		switch resolveOpenAIImageExecutionMode(c) {
		case openAIImageTestModeCodex:
			if account.Type == AccountTypeOAuth {
				return s.testOpenAIImageOAuth(c, ctx, account, testModelID, imagePrompt)
			}
			return s.testOpenAIImageAPIKey(c, ctx, account, testModelID, imagePrompt)
		default:
			return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported OpenAI image test mode: %s", resolveOpenAIImageExecutionMode(c)))
		}
	}
	if mode == AccountTestModeCompact {
		testModelID = resolveOpenAICompactForwardModel(account, testModelID)
		return s.testOpenAICompactConnection(c, account, testModelID)
	}

	// Determine authentication method and API URL
	var authToken string
	var apiURL string
	var isOAuth bool

	if account.IsOAuth() {
		isOAuth = true
		// OAuth - use Bearer token with ChatGPT internal API
		authToken = account.GetOpenAIAccessToken()
		if authToken == "" {
			return s.sendErrorAndEnd(c, "No access token available")
		}

		// OAuth uses ChatGPT internal API
		apiURL = chatgptCodexAPIURL
	} else if account.Type == "apikey" {
		// API Key - use Platform API
		authToken = account.GetOpenAIApiKey()
		if authToken == "" {
			return s.sendErrorAndEnd(c, "No API key available")
		}

		baseURL := account.GetOpenAIBaseURL()
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid base URL: %s", err.Error()))
		}
		if !openai_compat.ShouldUseResponsesAPI(account.Extra) {
			return s.testOpenAIChatCompletionsConnection(c, account, testModelID, prompt, normalizedBaseURL, authToken)
		}
		apiURL = buildOpenAIResponsesURL(normalizedBaseURL)
	} else {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported account type: %s", account.Type))
	}

	// Set SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	// Create OpenAI Responses API payload
	payload := createOpenAITestPayloadWithPrompt(testModelID, isOAuth, prompt)
	payloadBytes, _ := json.Marshal(payload)

	// Send test_start event
	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create request")
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))

	// Set common headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)

	// Set OAuth-specific headers for ChatGPT internal API
	if isOAuth {
		req.Host = "chatgpt.com"
		req.Header.Set("accept", "text/event-stream")
		setOpenAIChatGPTAccountHeaders(req.Header, account)
	}

	// Get proxy URL
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.doUpstreamWithTLS(c, req, account, proxyURL, s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http"))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	if isOAuth && s.accountRepo != nil {
		if updates, err := extractOpenAICodexProbeUpdates(resp); err == nil && len(updates) > 0 {
			_ = s.accountRepo.UpdateExtra(ctx, account.ID, updates)
			mergeAccountExtra(account, updates)
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusTooManyRequests {
			s.reconcileOpenAI429State(ctx, account, resp.Header, body)
		}
		// 401 Unauthorized: 标记账号为永久错误
		if resp.StatusCode == http.StatusUnauthorized && s.accountRepo != nil {
			errMsg := fmt.Sprintf("Authentication failed (401): %s", string(body))
			_ = s.accountRepo.SetError(ctx, account.ID, errMsg)
		}
		return s.sendErrorAndEnd(c, fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)))
	}

	// Process SSE stream
	return s.processOpenAIStream(c, resp.Body)
}

// testGrokAccountConnection tests a Grok OAuth account through xAI's Responses API.
// testOpenAIChatCompletionsConnection tests an OpenAI-compatible APIKey account
// through the raw /v1/chat/completions endpoint.
func (s *AccountTestService) testOpenAIChatCompletionsConnection(
	c *gin.Context,
	account *Account,
	testModelID string,
	prompt string,
	normalizedBaseURL string,
	authToken string,
) error {
	ctx := c.Request.Context()
	apiURL := buildOpenAIChatCompletionsURL(normalizedBaseURL)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	payload := createOpenAIChatCompletionsTestPayload(testModelID, prompt)
	payloadBytes, _ := json.Marshal(payload)

	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})
	s.sendEvent(c, TestEvent{Type: "status", Text: "正在通过 /v1/chat/completions 测试连接"})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create Chat Completions request")
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+authToken)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http"))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Chat Completions API (/v1/chat/completions) request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusTooManyRequests {
			s.reconcileOpenAI429State(ctx, account, resp.Header, body)
		}
		if resp.StatusCode == http.StatusUnauthorized && s.accountRepo != nil {
			errMsg := fmt.Sprintf("Chat Completions authentication failed (401): %s", string(body))
			_ = s.accountRepo.SetError(ctx, account.ID, errMsg)
		}
		return s.sendErrorAndEnd(c, fmt.Sprintf("Chat Completions API (/v1/chat/completions) returned %d: %s", resp.StatusCode, string(body)))
	}

	return s.processOpenAIChatCompletionsStream(c, resp.Body)
}

func (s *AccountTestService) testOpenAIResponsesLikeAccountConnection(
	c *gin.Context,
	ctx context.Context,
	account *Account,
	testModelID string,
	prompt string,
	authToken string,
	baseURL string,
	providerName string,
	isOAuth bool,
) error {
	apiURL, err := xai.BuildResponsesURL(baseURL)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid %s base URL: %s", providerName, err.Error()))
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	payload := createOpenAITestPayloadWithPrompt(testModelID, isOAuth, prompt)
	payloadBytes, _ := json.Marshal(payload)

	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})
	s.sendEvent(c, TestEvent{Type: "status", Text: fmt.Sprintf("正在通过 %s /v1/responses 测试连接", providerName)})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to create %s test request", providerName))
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+authToken)
	if providerName == "grok" {
		req.Header.Set("User-Agent", "sub2api-grok/1.0")
	}
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.doUpstreamWithTLS(c, req, account, proxyURL, s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http"))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("%s /v1/responses request failed: %s", providerName, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if providerName == "grok" {
			s.reconcileGrokTestState(ctx, account, resp.StatusCode, resp.Header)
		} else if resp.StatusCode == http.StatusTooManyRequests {
			s.reconcileOpenAI429State(ctx, account, resp.Header, body)
		}
		if resp.StatusCode == http.StatusUnauthorized && s.accountRepo != nil {
			errMsg := fmt.Sprintf("%s authentication failed (401): %s", providerName, string(body))
			_ = s.accountRepo.SetError(ctx, account.ID, errMsg)
		}
		return s.sendErrorAndEnd(c, fmt.Sprintf("%s /v1/responses returned %d: %s", providerName, resp.StatusCode, string(body)))
	}

	return s.processOpenAIStream(c, resp.Body)
}

// testOpenAICompactConnection probes /responses/compact and persists the
// resulting capability state on the account.
func (s *AccountTestService) testOpenAICompactConnection(c *gin.Context, account *Account, testModelID string) error {
	ctx := c.Request.Context()

	authToken := ""
	apiURL := ""
	isOAuth := false

	switch {
	case account.IsOAuth():
		isOAuth = true
		authToken = account.GetOpenAIAccessToken()
		if authToken == "" {
			return s.sendErrorAndEnd(c, "No access token available")
		}
		apiURL = chatgptCodexAPIURL + "/compact"
	case account.Type == AccountTypeAPIKey:
		authToken = account.GetOpenAIApiKey()
		if authToken == "" {
			return s.sendErrorAndEnd(c, "No API key available")
		}
		baseURL := account.GetOpenAIBaseURL()
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid base URL: %s", err.Error()))
		}
		apiURL = appendOpenAIResponsesRequestPathSuffix(buildOpenAIResponsesURL(normalizedBaseURL), "/compact")
	default:
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported account type: %s", account.Type))
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()
	payloadBytes, _ := json.Marshal(createOpenAICompactProbePayload(testModelID))
	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create request")
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)

	if isOAuth {
		req.Header.Set("Originator", "codex_cli_rs")
		req.Header.Set("User-Agent", codexCLIUserAgent)
		req.Header.Set("Version", codexCLIVersion)
		probeSessionID := compactProbeSessionID(account.ID)
		req.Header.Set("Session_ID", probeSessionID)
		req.Host = "chatgpt.com"
		setOpenAIChatGPTAccountHeaders(req.Header, account)
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.doUpstreamWithTLS(c, req, account, proxyURL, s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http"))
	if err != nil {
		if s.accountRepo != nil {
			updates := buildOpenAICompactProbeExtraUpdates(nil, nil, err, time.Now())
			_ = s.accountRepo.UpdateExtra(ctx, account.ID, updates)
			mergeAccountExtra(account, updates)
		}
		return s.sendErrorAndEnd(c, fmt.Sprintf("Request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))

	if s.accountRepo != nil {
		updates := buildOpenAICompactProbeExtraUpdates(resp, body, nil, time.Now())
		if codexUpdates, err := extractOpenAICodexProbeUpdates(resp); err == nil && len(codexUpdates) > 0 {
			updates = mergeExtraUpdates(updates, codexUpdates)
		}
		if len(updates) > 0 {
			_ = s.accountRepo.UpdateExtra(ctx, account.ID, updates)
			mergeAccountExtra(account, updates)
		}
		// 探测如返回 429,主动同步限流状态,避免后续短时间内继续选中。
		if resp.StatusCode == http.StatusTooManyRequests {
			s.reconcileOpenAI429State(ctx, account, resp.Header, body)
		}
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized && s.accountRepo != nil {
			errMsg := fmt.Sprintf("Authentication failed (401): %s", string(body))
			_ = s.accountRepo.SetError(ctx, account.ID, errMsg)
		}
		return s.sendErrorAndEnd(c, fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)))
	}

	s.sendEvent(c, TestEvent{Type: "content", Text: extractOpenAICompactProbeText(body)})
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

func (s *AccountTestService) testOpenAIImageAPIEndpoint(c *gin.Context, ctx context.Context, account *Account, modelID, prompt, endpointPath string) error {
	var authToken string
	switch account.Type {
	case AccountTypeAPIKey:
		authToken = account.GetOpenAIApiKey()
		if authToken == "" {
			return s.sendErrorAndEnd(c, "No API key available")
		}
	case AccountTypeOAuth:
		authToken = account.GetOpenAIAccessToken()
		if authToken == "" {
			return s.sendErrorAndEnd(c, "No access token available")
		}
	default:
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported account type: %s", account.Type))
	}

	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid base URL: %s", err.Error()))
	}
	apiURL := buildOpenAIImagesURL(normalizedBaseURL, endpointPath)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	s.sendEvent(c, TestEvent{Type: "test_start", Model: modelID})

	payload := map[string]any{
		"model":           modelID,
		"prompt":          prompt,
		"n":               1,
		"response_format": "b64_json",
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create request")
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.doUpstreamWithTLS(c, req, account, proxyURL, s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http"))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to read response: %s", err.Error()))
	}
	if resp.StatusCode != http.StatusOK {
		return s.sendErrorAndEnd(c, fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)))
	}

	var result struct {
		Data []struct {
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to parse response: %s", err.Error()))
	}
	if len(result.Data) == 0 {
		return s.sendErrorAndEnd(c, "No images returned from API")
	}
	for _, item := range result.Data {
		if item.RevisedPrompt != "" {
			s.sendEvent(c, TestEvent{Type: "content", Text: item.RevisedPrompt})
		}
		if item.B64JSON != "" {
			s.sendEvent(c, TestEvent{
				Type:     "image",
				ImageURL: "data:image/png;base64," + item.B64JSON,
				MimeType: "image/png",
			})
		}
	}

	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

func (s *AccountTestService) testOpenAIImageGatewayEndpoint(c *gin.Context, ctx context.Context, account *Account, modelID, prompt, endpointPath string) error {
	payload := map[string]any{
		"model":           modelID,
		"prompt":          prompt,
		"n":               1,
		"response_format": "b64_json",
		"stream":          true,
	}
	payloadBytes, _ := json.Marshal(payload)

	outboundUserAgent := strings.TrimSpace(account.GetOpenAIUserAgent())
	if outboundUserAgent == "" {
		outboundUserAgent = codexCLIUserAgent
	}
	isOfficialClient := openai.IsCodexOfficialClientByHeaders(outboundUserAgent, c.GetHeader("originator"))
	outboundOriginator := resolveOpenAIUpstreamOriginator(c, isOfficialClient)

	req := httptest.NewRequest(http.MethodPost, endpointPath, bytes.NewReader(payloadBytes)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", outboundUserAgent)
	req.Header.Set("originator", outboundOriginator)
	setOpenAIChatGPTAccountHeaders(req.Header, account)
	c.Request = req

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	s.sendEvent(c, TestEvent{Type: "test_start", Model: modelID})

	gateway := &OpenAIGatewayService{
		cfg:                  s.cfg,
		httpUpstream:         s.httpUpstream,
		openAITokenProvider:  nil,
		responseHeaderFilter: compileResponseHeaderFilter(s.cfg),
	}
	parsed, err := gateway.ParseOpenAIImagesRequest(c, payloadBytes)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to build image request: %s", err.Error()))
	}

	imageRec := httptest.NewRecorder()
	imageCtx, _ := gin.CreateTestContext(imageRec)
	imageCtx.Request = req
	result, err := gateway.ForwardImages(ctx, imageCtx, account, payloadBytes, parsed, "")
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Images API request failed: %s", err.Error()))
	}

	images, err := collectOpenAIImageTestResults(imageRec.Body.Bytes())
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to parse image response: %s", err.Error()))
	}
	if len(images) == 0 {
		return s.sendErrorAndEnd(c, "No images returned from API")
	}
	for _, item := range images {
		mimeType := openAIImageOutputMIMEType(item.OutputFormat)
		if item.RevisedPrompt != "" {
			s.sendEvent(c, TestEvent{Type: "content", Text: item.RevisedPrompt})
		}
		imageURL := item.URL
		if imageURL == "" && item.B64JSON != "" {
			imageURL = "data:" + mimeType + ";base64," + item.B64JSON
		}
		if imageURL != "" {
			s.sendEvent(c, TestEvent{Type: "image", ImageURL: imageURL, MimeType: mimeType})
		}
	}
	if result != nil && result.ImageCount <= 0 {
		return s.sendErrorAndEnd(c, "No images returned from API")
	}

	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

type openAIImageTestResult struct {
	B64JSON       string `json:"b64_json"`
	URL           string `json:"url"`
	RevisedPrompt string `json:"revised_prompt"`
	OutputFormat  string `json:"output_format"`
}

func collectOpenAIImageTestResults(body []byte) ([]openAIImageTestResult, error) {
	var response struct {
		Data         []openAIImageTestResult `json:"data"`
		OutputFormat string                  `json:"output_format"`
	}
	if err := json.Unmarshal(body, &response); err == nil {
		for i := range response.Data {
			if response.Data[i].OutputFormat == "" {
				response.Data[i].OutputFormat = response.OutputFormat
			}
		}
		return response.Data, nil
	}

	results := make([]openAIImageTestResult, 0, 1)
	for _, line := range strings.Split(string(body), "\n") {
		data, ok := extractOpenAISSEDataLine(strings.TrimRight(line, "\r"))
		if !ok || data == "" || data == "[DONE]" {
			continue
		}
		eventType := strings.TrimSpace(gjson.Get(data, "type").String())
		if strings.HasSuffix(eventType, ".partial_image") {
			continue
		}
		var item openAIImageTestResult
		if err := json.Unmarshal([]byte(data), &item); err != nil {
			continue
		}
		if item.URL != "" || item.B64JSON != "" {
			results = append(results, item)
		}
	}
	return results, nil
}

func (s *AccountTestService) testOpenAIImageWeb2API(c *gin.Context, ctx context.Context, account *Account, modelID, prompt string) error {
	authToken := account.GetOpenAIAccessToken()
	if authToken == "" {
		return s.sendErrorAndEnd(c, "No access token available")
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	s.sendEvent(c, TestEvent{Type: "test_start", Model: modelID})
	s.sendEvent(c, TestEvent{Type: "content", Text: "Initializing ChatGPT backend...\n"})

	gateway := &OpenAIGatewayService{
		accountRepo:         s.accountRepo,
		settingService:      s.settingService,
		tlsFPProfileService: s.tlsFPProfileService,
	}
	headers, err := gateway.buildOpenAIBackendAPIHeaders(account, authToken)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to build backend headers: %s", err.Error()))
	}
	profile := ResolveOpenAIImageWebProfile(account)
	if profile == nil || !profile.HasOpenAIImageWeb2APIProfile() {
		return s.sendErrorAndEnd(c, "OpenAI web2api image route requires complete web_profile (browser headers and cookie values)")
	}
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	client, err := newOpenAIBackendAPIClient(proxyURL)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to create client: %s", err.Error()))
	}

	if bootstrapErr := bootstrapOpenAIBackendAPI(ctx, client, headers); bootstrapErr != nil {
		log.Printf("OpenAI image test bootstrap warning: %v", bootstrapErr)
	}

	s.sendEvent(c, TestEvent{Type: "content", Text: "Fetching chat requirements...\n"})
	chatReqs, err := fetchOpenAIChatRequirements(ctx, client, headers, account, profile, gateway)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Chat requirements failed: %s", err.Error()))
	}
	if chatReqs.Arkose.Required {
		return s.sendErrorAndEnd(c, "Unsupported challenge: arkose required")
	}

	s.sendEvent(c, TestEvent{Type: "content", Text: "Preparing image conversation...\n"})
	parentMessageID := uuid.NewString()
	proofToken := generateOpenAIProofToken(chatReqs.ProofOfWork.Required, chatReqs.ProofOfWork.Seed, chatReqs.ProofOfWork.Difficulty, headers.Get("User-Agent"))
	_ = initializeOpenAIImageConversation(ctx, client, headers, account, profile, gateway)
	conduitToken, err := prepareOpenAIImageConversation(ctx, client, headers, account, profile, gateway, prompt, parentMessageID, chatReqs.Token, proofToken)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Conversation prepare failed: %s", err.Error()))
	}

	convReq := buildOpenAIImageTestConversationRequest(prompt, parentMessageID)
	convHeaders := cloneHTTPHeader(headers)
	convHeaders.Set("Accept", "text/event-stream")
	convHeaders.Set("Content-Type", "application/json")
	setOpenAIBackendAPIRequestTarget(convHeaders, openAIChatGPTConversationURL, "/backend-api/f/conversation")
	setOpenAIBackendAPIRequestCookieHeader(convHeaders, profile, openAIChatGPTConversationURL)
	convHeaders.Set("openai-sentinel-chat-requirements-token", chatReqs.Token)
	if conduitToken != "" {
		convHeaders.Set("x-conduit-token", conduitToken)
	}
	if proofToken != "" {
		convHeaders.Set("openai-sentinel-proof-token", proofToken)
	}

	s.sendEvent(c, TestEvent{Type: "content", Text: "Generating image...\n"})
	resp, err := client.R().
		SetContext(ctx).
		DisableAutoReadResponse().
		SetHeaders(headerToMap(convHeaders)).
		SetBodyJsonMarshal(convReq).
		Post(openAIChatGPTConversationURL)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Conversation request failed: %s", err.Error()))
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()
	gateway.applyOpenAIBackendAPIResponseState(ctx, account, profile, headers, resp.Response)
	if resp.StatusCode >= 400 {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Conversation API returned %d", resp.StatusCode))
	}

	conversationID, pointerInfos, _, _, err := readOpenAIImageConversationStream(resp, time.Now())
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Stream read failed: %s", err.Error()))
	}
	pointerInfos = mergeOpenAIImagePointerInfos(pointerInfos, nil)
	if conversationID != "" && !hasOpenAIFileServicePointerInfos(pointerInfos) {
		s.sendEvent(c, TestEvent{Type: "content", Text: "Waiting for image generation to complete...\n"})
		polledPointers, pollErr := pollOpenAIImageConversation(ctx, client, headers, account, profile, gateway, conversationID)
		if pollErr != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Poll failed: %s", pollErr.Error()))
		}
		pointerInfos = mergeOpenAIImagePointerInfos(pointerInfos, polledPointers)
	}
	pointerInfos = preferOpenAIFileServicePointerInfos(pointerInfos)
	if len(pointerInfos) == 0 {
		return s.sendErrorAndEnd(c, "No images returned from conversation")
	}

	s.sendEvent(c, TestEvent{Type: "content", Text: "Downloading generated image...\n"})
	for _, pointer := range pointerInfos {
		data, err := resolveOpenAIImageBytes(ctx, client, headers, profile, conversationID, pointer, openAIUpstreamErrorBodyReadLimit)
		if err != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Image download failed: %s", err.Error()))
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		mimeType := pointer.MimeType
		if strings.TrimSpace(mimeType) == "" {
			mimeType = http.DetectContentType(data)
		}
		if pointer.Prompt != "" {
			s.sendEvent(c, TestEvent{Type: "content", Text: pointer.Prompt})
		}
		s.sendEvent(c, TestEvent{
			Type:     "image",
			ImageURL: "data:" + mimeType + ";base64," + b64,
			MimeType: mimeType,
		})
	}

	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

func buildOpenAIImageTestConversationRequest(prompt, parentMessageID string) map[string]any {
	promptText := strings.TrimSpace(prompt)
	if promptText == "" {
		promptText = "Generate an image."
	}
	metadata := map[string]any{
		"developer_mode_connector_ids": []any{},
		"selected_github_repos":        []any{},
		"selected_all_github_repos":    false,
		"system_hints":                 []string{"picture_v2"},
		"serialization_metadata": map[string]any{
			"custom_symbol_offsets": []any{},
		},
	}
	message := map[string]any{
		"id":     uuid.NewString(),
		"author": map[string]any{"role": "user"},
		"content": map[string]any{
			"content_type": "text",
			"parts":        []any{promptText},
		},
		"metadata":    metadata,
		"create_time": float64(time.Now().UnixMilli()) / 1000,
	}
	return map[string]any{
		"action":                   "next",
		"client_prepare_state":     "sent",
		"parent_message_id":        parentMessageID,
		"messages":                 []any{message},
		"model":                    "auto",
		"timezone_offset_min":      openAITimezoneOffsetMinutes(),
		"timezone":                 openAITimezoneName(),
		"conversation_mode":        map[string]any{"kind": "primary_assistant"},
		"system_hints":             []string{"picture_v2"},
		"supports_buffering":       true,
		"supported_encodings":      []string{"v1"},
		"client_contextual_info":   map[string]any{"app_name": "chatgpt.com"},
		"force_nulligen":           false,
		"force_paragen":            false,
		"force_paragen_model_slug": "",
		"force_rate_limit":         false,
		"websocket_request_id":     uuid.NewString(),
	}
}

func (s *AccountTestService) reconcileOpenAI429State(ctx context.Context, account *Account, headers http.Header, body []byte) {
	if s == nil || s.accountRepo == nil || account == nil {
		return
	}

	persistOpenAI429PlanType(ctx, s.accountRepo, account, body)

	var resetAt *time.Time
	if calculated := calculateOpenAI429ResetTime(headers); calculated != nil {
		resetAt = calculated
	} else if unixTs := parseOpenAIRateLimitResetTime(body); unixTs != nil {
		t := time.Unix(*unixTs, 0)
		resetAt = &t
	}
	if resetAt == nil {
		return
	}

	if err := s.accountRepo.SetRateLimited(ctx, account.ID, *resetAt); err != nil {
		return
	}

	now := time.Now()
	account.RateLimitedAt = &now
	account.RateLimitResetAt = resetAt

	if account.Status == StatusError {
		if err := s.accountRepo.ClearError(ctx, account.ID); err != nil {
			return
		}
		account.Status = StatusActive
		account.ErrorMessage = ""
	}
}

// testGeminiAccountConnection tests a Gemini account's connection
func (s *AccountTestService) testGeminiAccountConnection(c *gin.Context, account *Account, modelID string, prompt string) error {
	ctx := c.Request.Context()

	// Determine the model to use
	testModelID := modelID
	if testModelID == "" {
		testModelID = geminicli.DefaultTestModel
	}
	setAccountTestOpsModelIfMissing(c, testModelID)

	// For static upstream credentials with model mapping, map the model
	if account.Type == AccountTypeAPIKey || account.Type == AccountTypeServiceAccount {
		mapping := account.GetModelMapping()
		if len(mapping) > 0 {
			if mappedModel, exists := mapping[testModelID]; exists {
				testModelID = mappedModel
			}
		}
	}

	// Set SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	// Create test payload (Gemini format)
	payload := createGeminiTestPayload(testModelID, prompt)

	// Build request based on account type
	var req *http.Request
	var err error

	switch account.Type {
	case AccountTypeAPIKey:
		req, err = s.buildGeminiAPIKeyRequest(ctx, account, testModelID, payload)
	case AccountTypeOAuth:
		req, err = s.buildGeminiOAuthRequest(ctx, account, testModelID, payload)
	case AccountTypeServiceAccount:
		req, err = s.buildGeminiServiceAccountRequest(ctx, account, testModelID, payload)
	default:
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported account type: %s", account.Type))
	}

	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to build request: %s", err.Error()))
	}

	// Send test_start event
	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	// Get proxy and execute request
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.doUpstreamWithTLS(c, req, account, proxyURL, s.tlsFPProfileService.ResolveTLSProfileForTransport(account, "http"))
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return s.sendErrorAndEnd(c, fmt.Sprintf("API returned %d: %s", resp.StatusCode, string(body)))
	}

	// Process SSE stream
	return s.processGeminiStream(c, resp.Body)
}

// routeAntigravityTest 路由 Antigravity 账号的测试请求。
// APIKey 类型走原生协议（与 gateway_handler 路由一致），OAuth/Upstream 走 CRS 中转。
func (s *AccountTestService) routeAntigravityTest(c *gin.Context, account *Account, modelID string, prompt string) error {
	if account.Type == AccountTypeAPIKey {
		if strings.HasPrefix(modelID, "gemini-") {
			return s.testGeminiAccountConnection(c, account, modelID, prompt)
		}
		return s.testClaudeAccountConnection(c, account, modelID)
	}
	return s.testAntigravityAccountConnection(c, account, modelID)
}

// testAntigravityAccountConnection tests an Antigravity account's connection
// 支持 Claude 和 Gemini 两种协议，使用非流式请求
func (s *AccountTestService) testAntigravityAccountConnection(c *gin.Context, account *Account, modelID string) error {
	ctx := c.Request.Context()

	// 默认模型：Claude 使用 claude-sonnet-4-5，Gemini 使用 gemini-3-pro-preview
	testModelID := modelID
	if testModelID == "" {
		testModelID = "claude-sonnet-4-5"
	}
	setAccountTestOpsModelIfMissing(c, testModelID)

	if s.antigravityGatewayService == nil {
		return s.sendErrorAndEnd(c, "Antigravity gateway service not configured")
	}

	// Set SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	// Send test_start event
	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	// 调用 AntigravityGatewayService.TestConnection（复用协议转换逻辑）
	result, err := s.antigravityGatewayService.TestConnection(ctx, account, testModelID)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}

	// 发送响应内容
	if result.Text != "" {
		s.sendEvent(c, TestEvent{Type: "content", Text: result.Text})
	}

	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

// buildGeminiAPIKeyRequest builds request for Gemini API Key accounts
func (s *AccountTestService) buildGeminiAPIKeyRequest(ctx context.Context, account *Account, modelID string, payload []byte) (*http.Request, error) {
	apiKey := account.GetCredential("api_key")
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("no API key available")
	}

	baseURL := account.GetCredential("base_url")
	if baseURL == "" {
		baseURL = geminicli.AIStudioBaseURL
	}
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}

	// Use streamGenerateContent for real-time feedback
	fullURL := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse",
		strings.TrimRight(normalizedBaseURL, "/"), modelID)

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	return req, nil
}

// buildGeminiOAuthRequest builds request for Gemini OAuth accounts
func (s *AccountTestService) buildGeminiOAuthRequest(ctx context.Context, account *Account, modelID string, payload []byte) (*http.Request, error) {
	if s.geminiTokenProvider == nil {
		return nil, fmt.Errorf("gemini token provider not configured")
	}

	// Get access token (auto-refreshes if needed)
	accessToken, err := s.geminiTokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	if oauthType := account.GeminiOAuthTypeSafe(); oauthType == "code_assist" {
		projectID := strings.TrimSpace(account.GetCredential("project_id"))
		if projectID == "" {
			return nil, errors.New(errGeminiCodeAssistProjectIDNotConfigured)
		}
		return s.buildCodeAssistRequest(ctx, accessToken, projectID, modelID, payload)
	}

	// Google One and AI Studio-style OAuth accounts call the public Gemini API
	// directly with a bearer token. Unknown/legacy oauth_type stays on this path.
	baseURL := account.GetCredential("base_url")
	if strings.TrimSpace(baseURL) == "" {
		baseURL = geminicli.AIStudioBaseURL
	}
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	fullURL := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", strings.TrimRight(normalizedBaseURL, "/"), modelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return req, nil
}

func (s *AccountTestService) buildGeminiServiceAccountRequest(ctx context.Context, account *Account, modelID string, payload []byte) (*http.Request, error) {
	if s.geminiTokenProvider == nil {
		return nil, fmt.Errorf("gemini token provider not configured")
	}
	accessToken, err := s.geminiTokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("failed to get service account access token: %w", err)
	}
	fullURL, err := buildVertexGeminiURL(account.VertexProjectID(), account.VertexLocation(modelID), modelID, "streamGenerateContent", true)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return req, nil
}

// buildCodeAssistRequest builds request for Google Code Assist API (used by Gemini CLI and Antigravity)
func (s *AccountTestService) buildCodeAssistRequest(ctx context.Context, accessToken, projectID, modelID string, payload []byte) (*http.Request, error) {
	var inner map[string]any
	if err := json.Unmarshal(payload, &inner); err != nil {
		return nil, err
	}

	wrapped := map[string]any{
		"model":   modelID,
		"project": projectID,
		"request": inner,
	}
	wrappedBytes, _ := json.Marshal(wrapped)

	normalizedBaseURL, err := s.validateUpstreamBaseURL(geminicli.GeminiCliBaseURL)
	if err != nil {
		return nil, err
	}
	fullURL := fmt.Sprintf("%s/v1internal:streamGenerateContent?alt=sse", normalizedBaseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, bytes.NewReader(wrappedBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", geminicli.GeminiCLIUserAgent)

	return req, nil
}

// createGeminiTestPayload creates a minimal test payload for Gemini API.
// Image models use the image-generation path so the frontend can preview the returned image.
func createGeminiTestPayload(modelID string, prompt string) []byte {
	if isImageGenerationModel(modelID) {
		imagePrompt := strings.TrimSpace(prompt)
		if imagePrompt == "" {
			imagePrompt = defaultGeminiImageTestPrompt
		}

		payload := map[string]any{
			"contents": []map[string]any{
				{
					"role": "user",
					"parts": []map[string]any{
						{"text": imagePrompt},
					},
				},
			},
			"generationConfig": map[string]any{
				"responseModalities": []string{"TEXT", "IMAGE"},
				"imageConfig": map[string]any{
					"aspectRatio": "1:1",
				},
			},
		}
		bytes, _ := json.Marshal(payload)
		return bytes
	}

	textPrompt := strings.TrimSpace(prompt)
	if textPrompt == "" {
		textPrompt = defaultGeminiTextTestPrompt
	}

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{"text": textPrompt},
				},
			},
		},
		"systemInstruction": map[string]any{
			"parts": []map[string]any{
				{"text": "You are a helpful AI assistant."},
			},
		},
	}
	bytes, _ := json.Marshal(payload)
	return bytes
}

// processGeminiStream processes SSE stream from Gemini API
func (s *AccountTestService) processGeminiStream(c *gin.Context, body io.Reader) error {
	reader := bufio.NewReader(body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
				return nil
			}
			return s.sendErrorAndEnd(c, fmt.Sprintf("Stream read error: %s", err.Error()))
		}

		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}

		jsonStr := strings.TrimPrefix(line, "data: ")
		if jsonStr == "[DONE]" {
			s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
			return nil
		}

		var data map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			continue
		}

		// Support two Gemini response formats:
		// - AI Studio: {"candidates": [...]}
		// - Gemini CLI: {"response": {"candidates": [...]}}
		if resp, ok := data["response"].(map[string]any); ok && resp != nil {
			data = resp
		}
		if candidates, ok := data["candidates"].([]any); ok && len(candidates) > 0 {
			if candidate, ok := candidates[0].(map[string]any); ok {
				// Extract content first (before checking completion)
				if content, ok := candidate["content"].(map[string]any); ok {
					if parts, ok := content["parts"].([]any); ok {
						for _, part := range parts {
							if partMap, ok := part.(map[string]any); ok {
								if text, ok := partMap["text"].(string); ok && text != "" {
									s.sendEvent(c, TestEvent{Type: "content", Text: text})
								}
								if inlineData, ok := partMap["inlineData"].(map[string]any); ok {
									mimeType, _ := inlineData["mimeType"].(string)
									data, _ := inlineData["data"].(string)
									if strings.HasPrefix(strings.ToLower(mimeType), "image/") && data != "" {
										s.sendEvent(c, TestEvent{
											Type:     "image",
											ImageURL: fmt.Sprintf("data:%s;base64,%s", mimeType, data),
											MimeType: mimeType,
										})
									}
								}
							}
						}
					}
				}

				// Check for completion after extracting content
				if finishReason, ok := candidate["finishReason"].(string); ok && finishReason != "" {
					s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
					return nil
				}
			}
		}

		// Handle errors
		if errData, ok := data["error"].(map[string]any); ok {
			errorMsg := "Unknown error"
			if msg, ok := errData["message"].(string); ok {
				errorMsg = msg
			}
			return s.sendErrorAndEnd(c, errorMsg)
		}
	}
}

// createOpenAITestPayload creates a test payload for OpenAI Responses API
func createOpenAITestPayload(modelID string, isOAuth bool) map[string]any {
	return createOpenAITestPayloadWithPrompt(modelID, isOAuth, "")
}

func createOpenAITestPayloadWithPrompt(modelID string, isOAuth bool, prompt string) map[string]any {
	testPrompt := strings.TrimSpace(prompt)
	if testPrompt == "" {
		testPrompt = "hi"
	}
	payload := map[string]any{
		"model": modelID,
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_text",
						"text": testPrompt,
					},
				},
			},
		},
		"stream": true,
	}

	// OAuth accounts using ChatGPT internal API require store: false
	if isOAuth {
		payload["store"] = false
	}

	// All accounts require instructions for Responses API
	payload["instructions"] = openai.DefaultInstructions

	return payload
}

func createOpenAIChatCompletionsTestPayload(modelID string, prompt string) map[string]any {
	testPrompt := strings.TrimSpace(prompt)
	if testPrompt == "" {
		testPrompt = "hi"
	}

	return map[string]any{
		"model": modelID,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": testPrompt,
			},
		},
		"stream": true,
	}
}

// processClaudeStream processes the SSE stream from Claude API
func (s *AccountTestService) processClaudeStream(c *gin.Context, body io.Reader) error {
	reader := bufio.NewReader(body)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
				return nil
			}
			return s.sendErrorAndEnd(c, fmt.Sprintf("Stream read error: %s", err.Error()))
		}

		line = strings.TrimSpace(line)
		if line == "" || !sseDataPrefix.MatchString(line) {
			continue
		}

		jsonStr := sseDataPrefix.ReplaceAllString(line, "")
		if jsonStr == "[DONE]" {
			s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
			return nil
		}

		var data map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			continue
		}

		eventType, _ := data["type"].(string)

		switch eventType {
		case "content_block_delta":
			if delta, ok := data["delta"].(map[string]any); ok {
				if text, ok := delta["text"].(string); ok {
					s.sendEvent(c, TestEvent{Type: "content", Text: text})
				}
			}
		case "message_stop":
			s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
			return nil
		case "error":
			errorMsg := "Unknown error"
			if errData, ok := data["error"].(map[string]any); ok {
				if msg, ok := errData["message"].(string); ok {
					errorMsg = msg
				}
			}
			return s.sendErrorAndEnd(c, errorMsg)
		}
	}
}

// processOpenAIChatCompletionsStream processes SSE chunks from the
// OpenAI-compatible Chat Completions API.
func (s *AccountTestService) processOpenAIChatCompletionsStream(c *gin.Context, body io.Reader) error {
	reader := bufio.NewReader(body)
	seenJSON := false
	seenFinish := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if seenFinish {
					s.sendEvent(c, TestEvent{Type: "status", Text: "已通过 /v1/chat/completions 验证"})
					s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
					return nil
				}
				if seenJSON {
					return s.sendErrorAndEnd(c, "Chat Completions stream from /v1/chat/completions ended before [DONE]")
				}
				return s.sendErrorAndEnd(c, "Invalid Chat Completions response from /v1/chat/completions: expected SSE JSON data")
			}
			return s.sendErrorAndEnd(c, fmt.Sprintf("Chat Completions stream read error from /v1/chat/completions: %s", err.Error()))
		}

		line = strings.TrimSpace(line)
		if line == "" || !sseDataPrefix.MatchString(line) {
			continue
		}

		jsonStr := sseDataPrefix.ReplaceAllString(line, "")
		if jsonStr == "[DONE]" {
			s.sendEvent(c, TestEvent{Type: "status", Text: "已通过 /v1/chat/completions 验证"})
			s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
			return nil
		}

		var data map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			return s.sendErrorAndEnd(c, "Invalid Chat Completions response from /v1/chat/completions: expected JSON data")
		}
		seenJSON = true

		if errData, ok := data["error"].(map[string]any); ok {
			errorMsg := "Chat Completions API (/v1/chat/completions) returned an error"
			if msg, ok := errData["message"].(string); ok && msg != "" {
				errorMsg = msg
			}
			return s.sendErrorAndEnd(c, fmt.Sprintf("Chat Completions API (/v1/chat/completions) error: %s", errorMsg))
		}

		choices, ok := data["choices"].([]any)
		if !ok {
			continue
		}
		for _, choiceValue := range choices {
			choice, ok := choiceValue.(map[string]any)
			if !ok {
				continue
			}
			if delta, ok := choice["delta"].(map[string]any); ok {
				if text, ok := delta["content"].(string); ok && text != "" {
					s.sendEvent(c, TestEvent{Type: "content", Text: text})
				}
			}
			if message, ok := choice["message"].(map[string]any); ok {
				if text, ok := message["content"].(string); ok && text != "" {
					s.sendEvent(c, TestEvent{Type: "content", Text: text})
				}
			}
			if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
				seenFinish = true
			}
		}
	}
}

// processOpenAIStream processes the SSE stream from OpenAI Responses API
func (s *AccountTestService) processOpenAIStream(c *gin.Context, body io.Reader) error {
	reader := bufio.NewReader(body)
	seenCompleted := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if seenCompleted {
					s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
					return nil
				}
				return s.sendErrorAndEnd(c, "Stream ended before response.completed")
			}
			return s.sendErrorAndEnd(c, fmt.Sprintf("Stream read error: %s", err.Error()))
		}

		line = strings.TrimSpace(line)
		if line == "" || !sseDataPrefix.MatchString(line) {
			continue
		}

		jsonStr := sseDataPrefix.ReplaceAllString(line, "")
		if jsonStr == "[DONE]" {
			if seenCompleted {
				s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
				return nil
			}
			return s.sendErrorAndEnd(c, "Stream ended before response.completed")
		}

		var data map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			continue
		}

		eventType, _ := data["type"].(string)

		switch eventType {
		case "response.output_text.delta":
			// OpenAI Responses API uses "delta" field for text content
			if delta, ok := data["delta"].(string); ok && delta != "" {
				s.sendEvent(c, TestEvent{Type: "content", Text: delta})
			}
		case "response.completed", "response.done":
			s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
			return nil
		case "response.failed":
			errorMsg := "OpenAI response failed"
			if responseData, ok := data["response"].(map[string]any); ok {
				if errData, ok := responseData["error"].(map[string]any); ok {
					if msg, ok := errData["message"].(string); ok && msg != "" {
						errorMsg = msg
					}
				}
			}
			return s.sendErrorAndEnd(c, errorMsg)
		case "error":
			errorMsg := "Unknown error"
			if errData, ok := data["error"].(map[string]any); ok {
				if msg, ok := errData["message"].(string); ok {
					errorMsg = msg
				}
			}
			return s.sendErrorAndEnd(c, errorMsg)
		}
	}
}

// testOpenAIImageAPIKey tests OpenAI image generation using an API Key account.
func (s *AccountTestService) testOpenAIImageAPIKey(c *gin.Context, ctx context.Context, account *Account, modelID, prompt string) error {
	return s.testOpenAIImageAPIEndpoint(c, ctx, account, modelID, prompt, "/v1/images/generations")
}

// testOpenAIImageOAuth tests OpenAI image generation through the Images API shape, pinned to this OAuth account.
func (s *AccountTestService) testOpenAIImageOAuth(c *gin.Context, ctx context.Context, account *Account, modelID, prompt string) error {
	return s.testOpenAIImageGatewayEndpoint(c, ctx, account, modelID, prompt, "/v1/images/generations")
}

func (s *AccountTestService) sendEvent(c *gin.Context, event TestEvent) {
	eventJSON, _ := json.Marshal(event)
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON); err != nil {
		log.Printf("failed to write SSE event: %v", err)
		return
	}
	c.Writer.Flush()
}

// sendErrorAndEnd sends an error event and ends the stream
func (s *AccountTestService) sendErrorAndEnd(c *gin.Context, errorMsg string) error {
	log.Printf("Account test error: %s", errorMsg)
	s.recordOpsError(c, errorMsg)
	fields := map[string]any{}
	if c != nil {
		if v, ok := c.Get(accountTestOpsAccountIDKey); ok {
			fields["account_id"] = v
		}
		if v, ok := c.Get(accountTestOpsPlatformKey); ok {
			fields["platform"] = v
		}
		if v, ok := c.Get(accountTestOpsTypeKey); ok {
			fields["account_type"] = v
		}
		if v, ok := c.Get(accountTestOpsNameKey); ok {
			fields["account_name"] = v
		}
		if c.Request != nil && c.Request.URL != nil {
			fields["request_path"] = c.Request.URL.Path
		}
	}
	pkglogger.WriteSinkEvent("error", "account.test", errorMsg, fields)
	s.sendEvent(c, TestEvent{Type: "error", Error: errorMsg})
	return fmt.Errorf("%s", errorMsg)
}

type accountTestOpsErrorClassification struct {
	statusCode         int
	errorPhase         string
	errorType          string
	severity           string
	errorSource        string
	errorOwner         string
	isRetryable        bool
	isBusinessLimited  bool
	upstreamStatusCode *int
	upstreamMessage    *string
	upstreamDetail     *string
}

func (s *AccountTestService) recordOpsError(c *gin.Context, errorMsg string) {
	if s == nil || s.opsService == nil || c == nil {
		return
	}

	entry := buildAccountTestOpsErrorEntry(c, errorMsg)
	if entry == nil {
		return
	}

	ctx := context.Background()
	if c.Request != nil && c.Request.Context() != nil {
		ctx = c.Request.Context()
	}
	if ShouldSkipOpsErrorLog(ctx, s.opsService, OpsErrorLogSkipInput{
		Message:     entry.ErrorMessage,
		Body:        entry.ErrorBody,
		RequestPath: entry.RequestPath,
		StatusCode: func() int {
			if entry.UpstreamStatusCode != nil && *entry.UpstreamStatusCode > 0 {
				return *entry.UpstreamStatusCode
			}
			return entry.StatusCode
		}(),
	}) {
		return
	}
	writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := s.opsService.RecordError(writeCtx, entry, nil); err != nil {
		log.Printf("Account test ops error log failed: %v", err)
	}
}

func buildAccountTestOpsErrorEntry(c *gin.Context, errorMsg string) *OpsInsertErrorLogInput {
	if c == nil {
		return nil
	}
	errorMsg = strings.TrimSpace(errorMsg)
	if errorMsg == "" {
		return nil
	}

	requestType := int16(RequestTypeStream)
	classification := classifyAccountTestOpsError(errorMsg)
	requestPath := accountTestOpsRequestPath(c)
	model := accountTestOpsContextString(c, accountTestOpsModelKey)

	entry := &OpsInsertErrorLogInput{
		RequestID:       accountTestOpsRequestID(c),
		ClientRequestID: accountTestOpsClientRequestID(c),
		AccountID:       accountTestOpsAccountID(c),
		Platform:        accountTestOpsContextString(c, accountTestOpsPlatformKey),
		Model:           model,
		RequestPath:     requestPath,
		Stream:          true,
		InboundEndpoint: requestPath,
		RequestedModel:  model,
		RequestType:     &requestType,
		UserAgent: func() string {
			if c.Request == nil {
				return ""
			}
			return strings.TrimSpace(c.GetHeader("User-Agent"))
		}(),

		ErrorPhase:        classification.errorPhase,
		ErrorType:         classification.errorType,
		Severity:          classification.severity,
		StatusCode:        classification.statusCode,
		IsBusinessLimited: classification.isBusinessLimited,
		IsCountTokens:     false,
		ErrorMessage:      errorMsg,
		ErrorBody:         errorMsg,
		ErrorSource:       classification.errorSource,
		ErrorOwner:        classification.errorOwner,
		UpstreamStatusCode: func() *int {
			return classification.upstreamStatusCode
		}(),
		UpstreamErrorMessage: classification.upstreamMessage,
		UpstreamErrorDetail:  classification.upstreamDetail,
		IsRetryable:          classification.isRetryable,
		RetryCount:           0,
		CreatedAt:            time.Now(),
	}
	accountTestApplyOpsLatencyFields(c, entry)
	return entry
}

func classifyAccountTestOpsError(errorMsg string) accountTestOpsErrorClassification {
	lower := strings.ToLower(strings.TrimSpace(errorMsg))
	statusCode, hasStatus := parseAccountTestReturnedStatusCode(errorMsg)
	isKiroUpstreamFrameFailure := strings.Contains(lower, "kiro upstream returned exception frame") ||
		strings.Contains(lower, "kiro upstream returned error frame")
	classification := accountTestOpsErrorClassification{
		statusCode:  500,
		errorPhase:  "internal",
		errorType:   "api_error",
		errorSource: "admin_account_test",
		errorOwner:  "admin",
	}

	if hasStatus {
		classification.statusCode = statusCode
		classification.errorPhase = "upstream"
		classification.errorType = "upstream_error"
		classification.errorSource = "upstream_http"
		classification.errorOwner = "provider"
		classification.upstreamStatusCode = accountTestIntPtr(statusCode)
		classification.upstreamMessage = accountTestStringPtr(strings.TrimSpace(errorMsg))
		if detail := accountTestOpsErrorDetail(errorMsg); detail != "" {
			classification.upstreamDetail = accountTestStringPtr(detail)
		}
	}

	switch {
	case strings.Contains(lower, "too many requests") || strings.Contains(lower, "rate limit"):
		classification.statusCode = 429
		classification.errorPhase = "upstream"
		classification.errorType = "rate_limit_error"
		classification.errorSource = "upstream_http"
		classification.errorOwner = "provider"
		classification.isBusinessLimited = true
		if classification.upstreamStatusCode == nil {
			classification.upstreamStatusCode = accountTestIntPtr(429)
		}
		if classification.upstreamMessage == nil {
			classification.upstreamMessage = accountTestStringPtr(strings.TrimSpace(errorMsg))
		}
	case isKiroUpstreamFrameFailure:
		classification.statusCode = http.StatusBadGateway
		classification.errorPhase = "upstream"
		classification.errorType = "upstream_error"
		classification.errorSource = "upstream_http"
		classification.errorOwner = "provider"
		if classification.upstreamStatusCode == nil {
			classification.upstreamStatusCode = accountTestIntPtr(http.StatusBadGateway)
		}
		if classification.upstreamMessage == nil {
			classification.upstreamMessage = accountTestStringPtr(strings.TrimSpace(errorMsg))
		}
	case classification.statusCode == http.StatusUnauthorized || classification.statusCode == http.StatusForbidden ||
		strings.Contains(lower, "access token") ||
		strings.Contains(lower, "refresh token") ||
		strings.Contains(lower, "api key available") ||
		strings.Contains(lower, "token provider is not configured"):
		if !hasStatus {
			classification.statusCode = http.StatusUnauthorized
		}
		classification.errorPhase = "auth"
		classification.errorType = "authentication_error"
		classification.errorSource = "account_credentials"
		if !hasStatus {
			classification.errorOwner = "admin"
		}
	case classification.statusCode == http.StatusBadRequest ||
		strings.Contains(lower, "unsupported") ||
		strings.Contains(lower, "invalid ") ||
		strings.Contains(lower, " is required"):
		if !hasStatus {
			classification.statusCode = http.StatusBadRequest
		}
		classification.errorPhase = "request_validation"
		classification.errorType = "invalid_request_error"
		if hasStatus {
			classification.errorPhase = "upstream"
			classification.errorSource = "upstream_http"
			classification.errorOwner = "provider"
		}
	case hasStatus ||
		strings.Contains(lower, "request failed") ||
		strings.Contains(lower, "stream read error") ||
		strings.Contains(lower, "failed to decode") ||
		strings.Contains(lower, "failed to parse response") ||
		strings.Contains(lower, "failed to parse image response") ||
		strings.Contains(lower, "no images returned") ||
		strings.Contains(lower, "poll failed") ||
		strings.Contains(lower, "image download failed"):
		if !hasStatus {
			classification.statusCode = http.StatusBadGateway
		}
		classification.errorPhase = "upstream"
		classification.errorType = "upstream_error"
		classification.errorSource = "upstream_http"
		classification.errorOwner = "provider"
		if classification.upstreamMessage == nil {
			classification.upstreamMessage = accountTestStringPtr(strings.TrimSpace(errorMsg))
		}
	}

	classification.severity = classifyAccountTestOpsSeverity(classification.errorType, classification.statusCode)
	classification.isRetryable = classifyAccountTestOpsRetryable(classification.errorType, classification.statusCode)
	return classification
}

func classifyAccountTestOpsSeverity(errorType string, statusCode int) string {
	switch errorType {
	case "invalid_request_error", "authentication_error", "billing_error", "subscription_error":
		return "P3"
	}
	if statusCode >= 500 || statusCode == http.StatusTooManyRequests {
		return "P1"
	}
	if statusCode >= 400 {
		return "P2"
	}
	return "P3"
}

func classifyAccountTestOpsRetryable(errorType string, statusCode int) bool {
	switch errorType {
	case "authentication_error", "invalid_request_error", "billing_error", "subscription_error":
		return false
	case "rate_limit_error", "timeout_error":
		return true
	case "upstream_error":
		return statusCode >= 500 || statusCode == http.StatusTooManyRequests
	default:
		return statusCode >= 500
	}
}

func parseAccountTestReturnedStatusCode(errorMsg string) (int, bool) {
	match := accountTestReturnedStatusCodePattern.FindStringSubmatch(strings.TrimSpace(errorMsg))
	if len(match) != 2 {
		return 0, false
	}
	code, err := strconv.Atoi(match[1])
	if err != nil || code < 100 || code > 599 {
		return 0, false
	}
	return code, true
}

func accountTestOpsErrorDetail(errorMsg string) string {
	idx := strings.Index(errorMsg, ":")
	if idx < 0 || idx+1 >= len(errorMsg) {
		return ""
	}
	return strings.TrimSpace(errorMsg[idx+1:])
}

func accountTestOpsRequestID(c *gin.Context) string {
	if c != nil && c.Request != nil {
		if requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string); strings.TrimSpace(requestID) != "" {
			return strings.TrimSpace(requestID)
		}
		if requestID := strings.TrimSpace(c.Writer.Header().Get("X-Request-Id")); requestID != "" {
			return requestID
		}
		if requestID := strings.TrimSpace(c.Writer.Header().Get("x-request-id")); requestID != "" {
			return requestID
		}
	}
	return "acctest_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}

func accountTestOpsClientRequestID(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if clientRequestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(clientRequestID) != "" {
		return strings.TrimSpace(clientRequestID)
	}
	return strings.TrimSpace(c.GetHeader("X-Client-Request-Id"))
}

func accountTestIntPtr(v int) *int {
	return &v
}

func accountTestStringPtr(v string) *string {
	return &v
}

func accountTestOpsRequestPath(c *gin.Context) string {
	if c != nil && c.Request != nil && c.Request.URL != nil {
		if path := strings.TrimSpace(c.Request.URL.Path); path != "" {
			return path
		}
	}
	return "/internal/account-tests"
}

func accountTestOpsContextString(c *gin.Context, key string) string {
	if c == nil || strings.TrimSpace(key) == "" {
		return ""
	}
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func accountTestOpsAccountID(c *gin.Context) *int64 {
	if c == nil {
		return nil
	}
	v, ok := c.Get(accountTestOpsAccountIDKey)
	if !ok {
		return nil
	}
	accountID, ok := v.(int64)
	if !ok || accountID <= 0 {
		return nil
	}
	return &accountID
}

func accountTestApplyOpsLatencyFields(c *gin.Context, entry *OpsInsertErrorLogInput) {
	if c == nil || entry == nil {
		return
	}
	if v, ok := accountTestContextInt64(c, OpsAuthLatencyMsKey); ok {
		entry.AuthLatencyMs = &v
	}
	if v, ok := accountTestContextInt64(c, OpsRoutingLatencyMsKey); ok {
		entry.RoutingLatencyMs = &v
	}
	if v, ok := accountTestContextInt64(c, OpsUpstreamLatencyMsKey); ok {
		entry.UpstreamLatencyMs = &v
	}
	if v, ok := accountTestContextInt64(c, OpsResponseLatencyMsKey); ok {
		entry.ResponseLatencyMs = &v
	}
	if v, ok := accountTestContextInt64(c, OpsTimeToFirstTokenMsKey); ok {
		entry.TimeToFirstTokenMs = &v
	}
}

func accountTestContextInt64(c *gin.Context, key string) (int64, bool) {
	if c == nil || strings.TrimSpace(key) == "" {
		return 0, false
	}
	v, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	switch typed := v.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	default:
		return 0, false
	}
}

// RunTestBackground executes an account test in-memory (no real HTTP client),
// capturing SSE output via httptest.NewRecorder, then parses the result.
func (s *AccountTestService) RunTestBackground(ctx context.Context, accountID int64, modelID string) (*ScheduledTestResult, error) {
	startedAt := time.Now()

	w := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/internal/scheduled-tests/accounts/%d/test", accountID), nil)
	ginCtx.Request = req.WithContext(ctx)

	testErr := s.TestAccountConnection(ginCtx, accountID, modelID, "", AccountTestModeDefault)

	finishedAt := time.Now()
	body := w.Body.String()
	responseText, errMsg := parseTestSSEOutput(body)

	status := "success"
	if testErr != nil || errMsg != "" {
		status = "failed"
		if errMsg == "" && testErr != nil {
			errMsg = testErr.Error()
		}
	}

	return &ScheduledTestResult{
		Status:       status,
		ResponseText: responseText,
		ErrorMessage: errMsg,
		LatencyMs:    finishedAt.Sub(startedAt).Milliseconds(),
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
	}, testErr
}

// parseTestSSEOutput extracts response text and error message from captured SSE output.
func parseTestSSEOutput(body string) (responseText, errMsg string) {
	var texts []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		jsonStr := strings.TrimPrefix(line, "data: ")
		var event TestEvent
		if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
			continue
		}
		switch event.Type {
		case "content":
			if event.Text != "" {
				texts = append(texts, event.Text)
			}
		case "error":
			errMsg = event.Error
		}
	}
	responseText = strings.Join(texts, "")
	return
}
