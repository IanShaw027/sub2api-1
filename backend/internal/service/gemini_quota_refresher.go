package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
)

var geminiQuotaModels = []string{
	"gemini-2.0-flash-exp",
	"gemini-exp-1206",
	"gemini-2.0-flash-thinking-exp",
}

// GeminiQuotaRefresher periodically refreshes Gemini OAuth account quota info.
type GeminiQuotaRefresher struct {
	accountRepo   AccountRepository
	proxyRepo     ProxyRepository
	tokenProvider *GeminiTokenProvider
	cfg           *config.TokenRefreshConfig
	httpClient    *http.Client

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewGeminiQuotaRefresher creates a Gemini quota refresher.
func NewGeminiQuotaRefresher(
	accountRepo AccountRepository,
	proxyRepo ProxyRepository,
	tokenProvider *GeminiTokenProvider,
	cfg *config.Config,
) *GeminiQuotaRefresher {
	return &GeminiQuotaRefresher{
		accountRepo:   accountRepo,
		proxyRepo:     proxyRepo,
		tokenProvider: tokenProvider,
		cfg:           &cfg.TokenRefresh,
		httpClient:    newGeminiQuotaHTTPClient(),
		stopCh:        make(chan struct{}),
	}
}

// Start starts the background quota refresh service.
func (r *GeminiQuotaRefresher) Start() {
	if !r.cfg.Enabled {
		log.Println("[GeminiQuota] Service disabled by configuration")
		return
	}

	r.wg.Add(1)
	go r.refreshLoop()

	log.Printf("[GeminiQuota] Service started (check every %d minutes)", r.cfg.CheckIntervalMinutes)
}

// Stop stops the service.
func (r *GeminiQuotaRefresher) Stop() {
	close(r.stopCh)
	r.wg.Wait()
	log.Println("[GeminiQuota] Service stopped")
}

func (r *GeminiQuotaRefresher) refreshLoop() {
	defer r.wg.Done()

	checkInterval := time.Duration(r.cfg.CheckIntervalMinutes) * time.Minute
	if checkInterval < time.Minute {
		checkInterval = 5 * time.Minute
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	r.processRefresh()

	for {
		select {
		case <-ticker.C:
			r.processRefresh()
		case <-r.stopCh:
			return
		}
	}
}

func (r *GeminiQuotaRefresher) processRefresh() {
	ctx := context.Background()

	allAccounts, err := r.accountRepo.ListActive(ctx)
	if err != nil {
		log.Printf("[GeminiQuota] Failed to list accounts: %v", err)
		return
	}

	var accounts []Account
	for _, acc := range allAccounts {
		if acc.Platform == PlatformGemini && acc.Type == AccountTypeOAuth {
			accounts = append(accounts, acc)
		}
	}

	if len(accounts) == 0 {
		return
	}

	refreshed, failed := 0, 0
	for i := range accounts {
		account := &accounts[i]
		if err := r.refreshAccountQuota(ctx, account); err != nil {
			log.Printf("[GeminiQuota] Account %d (%s) failed: %v", account.ID, account.Name, err)
			failed++
		} else {
			refreshed++
		}
	}

	log.Printf("[GeminiQuota] Cycle complete: total=%d, refreshed=%d, failed=%d",
		len(accounts), refreshed, failed)
}

func (r *GeminiQuotaRefresher) refreshAccountQuota(ctx context.Context, account *Account) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if account == nil {
		return errors.New("account is nil")
	}
	if r.tokenProvider == nil {
		return errors.New("gemini token provider not configured")
	}

	accessToken, err := r.tokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	projectID := strings.TrimSpace(account.GetCredential("project_id"))

	baseURL := ""
	fallbackBaseURL := ""
	apiType := ""
	if projectID != "" {
		baseURL = geminicli.GeminiCliBaseURL
		fallbackBaseURL = strings.TrimSpace(account.GetCredential("base_url"))
		if fallbackBaseURL == "" {
			fallbackBaseURL = geminicli.AIStudioBaseURL
		}
		apiType = "code_assist"
	} else {
		baseURL = strings.TrimSpace(account.GetCredential("base_url"))
		if baseURL == "" {
			baseURL = geminicli.AIStudioBaseURL
		}
		apiType = "ai_studio"
	}
	log.Printf("[GeminiQuota] Account %d (%s) using %s API", account.ID, account.Name, apiType)

	var proxyURL string
	if account.ProxyID != nil {
		if r.proxyRepo != nil {
			proxy, err := r.proxyRepo.GetByID(ctx, *account.ProxyID)
			if err == nil && proxy != nil {
				proxyURL = proxy.URL()
			}
		}
	}

	proxyCtx := ctx
	if strings.TrimSpace(proxyURL) != "" {
		if parsed, err := url.Parse(proxyURL); err == nil {
			proxyCtx = withGeminiProxy(proxyCtx, parsed)
		}
	}

	client := r.httpClient
	if client == nil {
		client = newGeminiQuotaHTTPClient()
	}
	quota := make(map[string]any)
	if account.Extra != nil {
		if rawQuota, ok := account.Extra["quota"]; ok {
			if existing, ok := rawQuota.(map[string]any); ok {
				for key, value := range existing {
					quota[key] = value
				}
			}
		}
	}

	updated := 0
	for _, model := range geminiQuotaModels {
		modelQuota, err := fetchGeminiModelQuota(proxyCtx, client, baseURL, accessToken, model, projectID, fallbackBaseURL)
		if err != nil {
			log.Printf("[GeminiQuota] Account %d model %s failed: %v", account.ID, model, err)
			continue
		}
		quota[model] = map[string]any{
			"remaining":  modelQuota.Remaining,
			"reset_time": modelQuota.ResetTime,
		}
		updated++
	}

	if len(quota) == 0 {
		return nil
	}
	if updated == 0 {
		return nil
	}

	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	account.Extra["quota"] = quota
	account.Extra["last_quota_check"] = time.Now().Format(time.RFC3339)

	return r.accountRepo.Update(ctx, account)
}

type geminiModelQuota struct {
	Remaining int
	ResetTime string
}

func fetchGeminiModelQuota(ctx context.Context, client *http.Client, baseURL, accessToken, model, projectID, fallbackBaseURL string) (*geminiModelQuota, error) {
	if client == nil {
		return nil, errors.New("http client is nil")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, errors.New("model is empty")
	}

	projectID = strings.TrimSpace(projectID)
	if projectID != "" {
		log.Printf("[GeminiQuota] Model %s attempting code_assist quota API", model)
		quota, err := fetchGeminiModelQuotaCodeAssist(ctx, client, baseURL, accessToken, projectID, model)
		if err == nil {
			return quota, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		log.Printf("[GeminiQuota] Model %s code_assist quota API failed: %v", model, err)
		fallbackBaseURL = strings.TrimRight(strings.TrimSpace(fallbackBaseURL), "/")
		if fallbackBaseURL == "" {
			fallbackBaseURL = geminicli.AIStudioBaseURL
		}
		log.Printf("[GeminiQuota] Model %s attempting ai_studio quota API", model)
		return fetchGeminiModelQuotaAIStudio(ctx, client, fallbackBaseURL, accessToken, model)
	}

	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = geminicli.AIStudioBaseURL
	}

	log.Printf("[GeminiQuota] Model %s attempting ai_studio quota API", model)
	return fetchGeminiModelQuotaAIStudio(ctx, client, baseURL, accessToken, model)
}

func fetchGeminiModelQuotaAIStudio(ctx context.Context, client *http.Client, baseURL, accessToken, model string) (*geminiModelQuota, error) {
	modelPath := strings.TrimPrefix(model, "models/")
	fullURL := fmt.Sprintf("%s/v1beta/models/%s", baseURL, modelPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 500 {
			snippet = snippet[:500] + "..."
		}
		return nil, fmt.Errorf("models.get %s failed (HTTP %d): %s", modelPath, resp.StatusCode, snippet)
	}

	quota, err := parseGeminiQuota(body, resp.Header)
	if err != nil {
		return nil, err
	}
	return quota, nil
}

func fetchGeminiModelQuotaCodeAssist(ctx context.Context, client *http.Client, baseURL, accessToken, projectID, model string) (*geminiModelQuota, error) {
	if client == nil {
		return nil, errors.New("http client is nil")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, errors.New("project_id is empty")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, errors.New("model is empty")
	}

	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = geminicli.GeminiCliBaseURL
	}

	reqBody := map[string]string{
		"project": projectID,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	fullURL := fmt.Sprintf("%s/v1internal:fetchAvailableModels", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", geminicli.GeminiCLIUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 500 {
			snippet = snippet[:500] + "..."
		}
		return nil, fmt.Errorf("fetchAvailableModels failed (HTTP %d): %s", resp.StatusCode, snippet)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	modelKey := strings.TrimPrefix(model, "models/")
	modelPayload, err := lookupCodeAssistModel(payload, modelKey)
	if err != nil {
		return nil, err
	}

	if quota := extractQuotaFromPayload(modelPayload); quota != nil {
		return quota, nil
	}

	return nil, fmt.Errorf("quota info not found for model %s", modelKey)
}

func lookupCodeAssistModel(payload map[string]any, modelKey string) (map[string]any, error) {
	modelsRaw, ok := payload["models"]
	if !ok || modelsRaw == nil {
		return nil, errors.New("models not found in code_assist response")
	}
	models, ok := modelsRaw.(map[string]any)
	if !ok {
		return nil, errors.New("models has unexpected type in code_assist response")
	}

	candidates := []string{
		modelKey,
		"models/" + modelKey,
	}
	for _, candidate := range candidates {
		if raw, ok := models[candidate]; ok {
			if modelPayload, ok := raw.(map[string]any); ok {
				return modelPayload, nil
			}
			return nil, fmt.Errorf("model %s has unexpected type", candidate)
		}
	}

	for key, raw := range models {
		if strings.TrimPrefix(key, "models/") == modelKey {
			if modelPayload, ok := raw.(map[string]any); ok {
				return modelPayload, nil
			}
			return nil, fmt.Errorf("model %s has unexpected type", key)
		}
	}

	return nil, fmt.Errorf("model %s not found in code_assist response", modelKey)
}

type geminiProxyContextKey struct{}

func withGeminiProxy(ctx context.Context, proxyURL *url.URL) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if proxyURL == nil {
		return ctx
	}
	return context.WithValue(ctx, geminiProxyContextKey{}, proxyURL)
}

func geminiProxyFromContext(req *http.Request) (*url.URL, error) {
	if req == nil {
		return nil, nil
	}
	if raw := req.Context().Value(geminiProxyContextKey{}); raw != nil {
		if proxyURL, ok := raw.(*url.URL); ok && proxyURL != nil {
			return proxyURL, nil
		}
	}
	return http.ProxyFromEnvironment(req)
}

func newGeminiQuotaHTTPClient() *http.Client {
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	var transport *http.Transport
	if ok && baseTransport != nil {
		transport = baseTransport.Clone()
	} else {
		transport = &http.Transport{}
	}
	transport.Proxy = geminiProxyFromContext
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}

func parseGeminiQuota(body []byte, headers http.Header) (*geminiModelQuota, error) {
	var payload map[string]any
	var bodyErr error

	if len(body) == 0 {
		bodyErr = errors.New("empty response body")
	} else if err := json.Unmarshal(body, &payload); err != nil {
		bodyErr = err
	}

	if bodyErr == nil {
		if quota := extractQuotaFromPayload(payload); quota != nil {
			return quota, nil
		}
	}

	if quota := extractQuotaFromHeaders(headers); quota != nil {
		return quota, nil
	}

	if bodyErr != nil {
		return nil, fmt.Errorf("parse quota: %w", bodyErr)
	}
	return nil, errors.New("quota info not found in response")
}

func extractQuotaFromPayload(payload map[string]any) *geminiModelQuota {
	if payload == nil {
		return nil
	}
	if quota := extractQuotaFromMap(payload); quota != nil {
		return quota
	}
	if quota := extractQuotaFromKey(payload, "quotaInfo"); quota != nil {
		return quota
	}
	if quota := extractQuotaFromKey(payload, "quota_info"); quota != nil {
		return quota
	}
	if quota := extractQuotaFromKey(payload, "quota"); quota != nil {
		return quota
	}
	if quota := extractQuotaFromRateLimits(payload, "rateLimits"); quota != nil {
		return quota
	}
	if quota := extractQuotaFromRateLimits(payload, "rate_limits"); quota != nil {
		return quota
	}
	if meta, ok := payload["metadata"].(map[string]any); ok {
		if quota := extractQuotaFromPayload(meta); quota != nil {
			return quota
		}
	}
	return nil
}

func extractQuotaFromKey(payload map[string]any, key string) *geminiModelQuota {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return nil
	}
	if m, ok := raw.(map[string]any); ok {
		return extractQuotaFromMap(m)
	}
	return nil
}

func extractQuotaFromRateLimits(payload map[string]any, key string) *geminiModelQuota {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return nil
	}
	limits, ok := raw.([]any)
	if !ok {
		return nil
	}
	for _, item := range limits {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if quota := extractQuotaFromMap(m); quota != nil {
			return quota
		}
	}
	return nil
}

func extractQuotaFromMap(m map[string]any) *geminiModelQuota {
	remaining, remainingOk := lookupFloat(m, []string{
		"remaining",
		"remainingRequests",
		"remaining_requests",
		"remainingTokens",
		"remaining_tokens",
		"remainingQuota",
		"remaining_quota",
	})
	limit, limitOk := lookupFloat(m, []string{
		"limit",
		"limitRequests",
		"limit_requests",
		"requestLimit",
		"request_limit",
		"requestsLimit",
		"requests_limit",
		"limitTokens",
		"limit_tokens",
		"maxRequests",
		"max_requests",
		"maxTokens",
		"max_tokens",
		"quotaLimit",
		"quota_limit",
		"maxQuota",
		"max_quota",
	})

	reset := lookupString(m, []string{
		"resetTime",
		"reset_time",
		"resetAt",
		"reset_at",
		"reset",
		"resetDate",
		"quotaResetDelay",
		"quota_reset_delay",
	})

	if remainingOk && limitOk {
		remainingPercent, ok := percentFromRemainingLimit(remaining, limit, "payload")
		if !ok {
			return nil
		}
		return &geminiModelQuota{
			Remaining: remainingPercent,
			ResetTime: normalizeResetTime(reset),
		}
	}

	if fraction, ok := lookupFloat(m, []string{
		"remainingFraction",
		"remaining_fraction",
	}); ok {
		remainingPercent, ok := percentFromFraction(fraction, "payload")
		if !ok {
			return nil
		}
		return &geminiModelQuota{
			Remaining: remainingPercent,
			ResetTime: normalizeResetTime(reset),
		}
	}

	if remainingOk && !limitOk {
		log.Printf("[GeminiQuota] Quota payload missing limit; remaining=%v", remaining)
	}

	return nil
}

func percentFromRemainingLimit(remaining, limit float64, source string) (int, bool) {
	if limit <= 0 {
		log.Printf("[GeminiQuota] %s limit invalid: %v", source, limit)
		return 0, false
	}

	percent := (remaining / limit) * 100
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		log.Printf("[GeminiQuota] %s percent invalid (remaining=%v limit=%v)", source, remaining, limit)
		return 0, false
	}

	if percent < 0 {
		log.Printf("[GeminiQuota] %s percent below 0 (remaining=%v limit=%v)", source, remaining, limit)
		percent = 0
	} else if percent > 100 {
		log.Printf("[GeminiQuota] %s percent above 100 (remaining=%v limit=%v)", source, remaining, limit)
		percent = 100
	}

	return int(percent), true
}

func percentFromFraction(fraction float64, source string) (int, bool) {
	percent := fraction * 100
	if math.IsNaN(percent) || math.IsInf(percent, 0) {
		log.Printf("[GeminiQuota] %s fraction percent invalid: %v", source, percent)
		return 0, false
	}
	if percent < 0 {
		log.Printf("[GeminiQuota] %s remaining fraction below 0: %v", source, fraction)
		percent = 0
	} else if percent > 100 {
		log.Printf("[GeminiQuota] %s remaining fraction above 1: %v", source, fraction)
		percent = 100
	}
	return int(percent), true
}

func extractQuotaFromHeaders(headers http.Header) *geminiModelQuota {
	remaining, remainingOk := headerFloat(headers, []string{
		"x-ratelimit-remaining",
		"x-ratelimit-remaining-requests",
		"x-ratelimit-remaining-tokens",
		"x-goog-quota-remaining",
		"x-goog-quota-remaining-requests",
	})
	if !remainingOk {
		return nil
	}

	limit, limitOk := headerFloat(headers, []string{
		"x-ratelimit-limit",
		"x-ratelimit-limit-requests",
		"x-ratelimit-limit-tokens",
		"x-goog-quota-limit",
		"x-goog-quota-limit-requests",
	})
	if !limitOk {
		log.Printf("[GeminiQuota] Quota headers missing limit; remaining=%v", remaining)
		return nil
	}

	remainingPercent, ok := percentFromRemainingLimit(remaining, limit, "headers")
	if !ok {
		return nil
	}

	reset := headerString(headers, []string{
		"x-ratelimit-reset",
		"x-ratelimit-reset-requests",
		"x-ratelimit-reset-tokens",
		"retry-after",
	})

	return &geminiModelQuota{
		Remaining: remainingPercent,
		ResetTime: normalizeResetTime(reset),
	}
}

func headerFloat(headers http.Header, keys []string) (float64, bool) {
	for _, key := range keys {
		if v := strings.TrimSpace(headers.Get(key)); v != "" {
			if f, ok := toFloat(v); ok {
				return f, true
			}
		}
	}
	return 0, false
}

func headerString(headers http.Header, keys []string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(headers.Get(key)); v != "" {
			return v
		}
	}
	return ""
}

func lookupString(m map[string]any, keys []string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s, ok := toString(v); ok {
				return s
			}
		}
	}
	return ""
}

func lookupFloat(m map[string]any, keys []string) (float64, bool) {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if f, ok := toFloat(v); ok {
				return f, true
			}
		}
	}
	return 0, false
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, false
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func toString(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v), true
	case json.Number:
		return v.String(), true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32), true
	case int:
		return strconv.Itoa(v), true
	case int64:
		return strconv.FormatInt(v, 10), true
	default:
		return "", false
	}
}

func normalizeResetTime(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Format(time.RFC3339)
	}
	if t, err := http.ParseTime(raw); err == nil {
		return t.Format(time.RFC3339)
	}
	if dur, err := time.ParseDuration(raw); err == nil {
		return time.Now().Add(dur).Format(time.RFC3339)
	}
	if ts, err := strconv.ParseInt(raw, 10, 64); err == nil {
		switch {
		case ts > 1e12:
			return time.Unix(0, ts*int64(time.Millisecond)).Format(time.RFC3339)
		case ts > 1e9:
			return time.Unix(ts, 0).Format(time.RFC3339)
		default:
			return time.Now().Add(time.Duration(ts) * time.Second).Format(time.RFC3339)
		}
	}

	return raw
}
