package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
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
	if r.tokenProvider == nil {
		return errors.New("gemini token provider not configured")
	}

	accessToken, err := r.tokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	baseURL := strings.TrimSpace(account.GetCredential("base_url"))
	if baseURL == "" {
		baseURL = geminicli.AIStudioBaseURL
	}

	var proxyURL string
	if account.ProxyID != nil {
		proxy, err := r.proxyRepo.GetByID(ctx, *account.ProxyID)
		if err == nil && proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	client := newGeminiQuotaHTTPClient(proxyURL)
	quota := make(map[string]any)

	for _, model := range geminiQuotaModels {
		modelQuota, err := fetchGeminiModelQuota(ctx, client, baseURL, accessToken, model)
		if err != nil {
			log.Printf("[GeminiQuota] Account %d model %s failed: %v", account.ID, model, err)
			continue
		}
		quota[model] = map[string]any{
			"remaining":  modelQuota.Remaining,
			"reset_time": modelQuota.ResetTime,
		}
	}

	if len(quota) == 0 {
		return errors.New("no quota data fetched")
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

func fetchGeminiModelQuota(ctx context.Context, client *http.Client, baseURL, accessToken, model string) (*geminiModelQuota, error) {
	if client == nil {
		return nil, errors.New("http client is nil")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, errors.New("model is empty")
	}

	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = geminicli.AIStudioBaseURL
	}

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

func newGeminiQuotaHTTPClient(proxyURL string) *http.Client {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	if strings.TrimSpace(proxyURL) == "" {
		return client
	}

	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		return client
	}

	client.Transport = &http.Transport{
		Proxy: http.ProxyURL(proxyURLParsed),
	}
	return client
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
	remaining, ok := lookupInt(m, []string{
		"remaining",
		"remainingRequests",
		"remaining_requests",
		"remainingTokens",
		"remaining_tokens",
		"remainingQuota",
		"remaining_quota",
	})
	if !ok {
		if fraction, ok := lookupFloat(m, []string{
			"remainingFraction",
			"remaining_fraction",
		}); ok {
			remaining = int(fraction * 100)
		} else {
			return nil
		}
	}

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

	return &geminiModelQuota{
		Remaining: remaining,
		ResetTime: normalizeResetTime(reset),
	}
}

func extractQuotaFromHeaders(headers http.Header) *geminiModelQuota {
	remaining, ok := headerInt(headers, []string{
		"x-ratelimit-remaining",
		"x-ratelimit-remaining-requests",
		"x-ratelimit-remaining-tokens",
		"x-goog-quota-remaining",
		"x-goog-quota-remaining-requests",
	})
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
		Remaining: remaining,
		ResetTime: normalizeResetTime(reset),
	}
}

func headerInt(headers http.Header, keys []string) (int, bool) {
	for _, key := range keys {
		if v := strings.TrimSpace(headers.Get(key)); v != "" {
			if i, ok := toInt(v); ok {
				return i, true
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

func lookupInt(m map[string]any, keys []string) (int, bool) {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if i, ok := toInt(v); ok {
				return i, true
			}
		}
	}
	return 0, false
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

func toInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
		if f, err := v.Float64(); err == nil {
			return int(f), true
		}
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return 0, false
		}
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return int(i), true
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return int(f), true
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
