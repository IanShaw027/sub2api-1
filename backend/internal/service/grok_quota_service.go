package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"golang.org/x/sync/singleflight"
)

const (
	grokQuotaUpstreamTimeout    = 20 * time.Second
	grokBillingUpstreamTimeout  = 15 * time.Second
	grokQuotaProbeInput         = "."
	grokQuotaDefaultModel       = xai.DefaultTextModel
	grokBillingSnapshotExtraKey = "grok_billing_snapshot"
)

type GrokQuotaProbeResult struct {
	Source          string               `json:"source"`
	Model           string               `json:"model,omitempty"`
	Snapshot        *xai.QuotaSnapshot   `json:"snapshot,omitempty"`
	StatusCode      int                  `json:"status_code,omitempty"`
	HeadersObserved bool                 `json:"headers_observed"`
	ResetSupported  bool                 `json:"reset_supported"`
	FetchedAt       int64                `json:"fetched_at"`
	Billing         *xai.BillingSnapshot `json:"billing,omitempty"`
}

type GrokQuotaResetResult struct {
	Supported bool   `json:"supported"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type GrokQuotaService struct {
	accountRepo     AccountRepository
	proxyRepo       ProxyRepository
	tokenProvider   *GrokTokenProvider
	httpUpstream    HTTPUpstream
	tlsFPProfileSvc *TLSFingerprintProfileService
	settingService  *SettingService
	probeFlight     singleflight.Group
}

func NewGrokQuotaService(
	accountRepo AccountRepository,
	proxyRepo ProxyRepository,
	tokenProvider *GrokTokenProvider,
	httpUpstream HTTPUpstream,
	tlsFPProfileSvc *TLSFingerprintProfileService,
) *GrokQuotaService {
	return &GrokQuotaService{
		accountRepo:     accountRepo,
		proxyRepo:       proxyRepo,
		tokenProvider:   tokenProvider,
		httpUpstream:    httpUpstream,
		tlsFPProfileSvc: tlsFPProfileSvc,
	}
}

// SetSettingService injects system settings (used for Grok default base URL mode).
func (s *GrokQuotaService) SetSettingService(settingService *SettingService) {
	if s != nil {
		s.settingService = settingService
	}
}

func (s *GrokQuotaService) SetTLSFingerprintProfileService(profileService *TLSFingerprintProfileService) {
	if s != nil {
		s.tlsFPProfileSvc = profileService
	}
}

func (s *GrokQuotaService) ProbeUsage(ctx context.Context, accountID int64) (*GrokQuotaProbeResult, error) {
	return s.runProbeFlight(ctx, "active:"+strconv.FormatInt(accountID, 10), func(sharedCtx context.Context) (*GrokQuotaProbeResult, error) {
		return s.probeUsage(sharedCtx, accountID)
	})
}

func (s *GrokQuotaService) probeUsage(ctx context.Context, accountID int64) (*GrokQuotaProbeResult, error) {
	account, token, proxyURL, err := s.prepareProbe(ctx, accountID)
	if err != nil {
		return nil, err
	}

	probeModel := resolveGrokQuotaProbeModel(account)
	body, err := buildGrokQuotaProbeBody(probeModel)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadRequest, "GROK_QUOTA_PROBE_BODY_ERROR", "failed to build probe body: %v", err)
	}
	baseURL := account.GetGrokBaseURL()
	if s.settingService != nil {
		baseURL = s.settingService.ResolveGrokBaseURL(ctx, account)
	}
	targetURL, err := xai.BuildResponsesURL(baseURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadRequest, "GROK_QUOTA_BASE_URL_INVALID", "invalid Grok base_url: %v", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, grokQuotaUpstreamTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_QUOTA_PROBE_REQUEST_BUILD_FAILED", "failed to build upstream request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	applyDefaultGrokUpstreamHeaders(req)

	// 探针命中与 /responses 同一 api.x.ai + 同一 OAuth token：必须复用账号绑定的
	// TLS 指纹与浏览器 UA，否则周期性探针会以不一致的 JA3/UA 暴露整个订阅账号。
	// 探针无入站 UA 上下文，按 transport 维度解析账号绑定 profile（profile 自带 UA 时覆盖默认探针 UA）。
	var tlsProfile *tlsfingerprint.Profile
	if s.tlsFPProfileSvc != nil {
		tlsProfile = s.tlsFPProfileSvc.ResolveTLSProfileForTransport(account, "http")
	}
	applyGrokTLSProfileHeaders(req, tlsProfile)

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, maxInt(account.Concurrency, 1), tlsProfile)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_QUOTA_PROBE_REQUEST_FAILED", "upstream probe failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	snapshot := xai.ObserveQuotaHeaders(resp.Header, resp.StatusCode, "active_probe")
	result := &GrokQuotaProbeResult{
		Source:          "active_probe",
		Model:           probeModel,
		Snapshot:        snapshot,
		StatusCode:      resp.StatusCode,
		HeadersObserved: snapshot.HeadersObserved,
		ResetSupported:  false,
		FetchedAt:       time.Now().Unix(),
	}
	// Persist only successful probes or 429 (which often carries rate-limit headers).
	// 401/403/5xx empty/noisy headers must not overwrite a previously good snapshot.
	if resp.StatusCode < 400 || resp.StatusCode == http.StatusTooManyRequests {
		if snapshot.HeadersObserved || resp.StatusCode == http.StatusOK {
			_ = s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{
				grokQuotaSnapshotExtraKey: snapshot,
			})
		}
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return result, nil
	}
	if resp.StatusCode >= 400 {
		const reason = "GROK_QUOTA_PROBE_UPSTREAM_ERROR"
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		slog.Warn(
			"grok_quota_probe_failed",
			"account_id", account.ID,
			"model", probeModel,
			"status", resp.StatusCode,
			"reason", reason,
		)
		return nil, infraerrors.Newf(
			mapUpstreamStatus(resp.StatusCode),
			reason,
			"upstream returned %d for probe model %q",
			resp.StatusCode,
			probeModel,
		)
	}
	return result, nil
}

// QueryQuota prefers billing endpoints and only sends a real inference probe
// when billing did not return an authoritative usage window.
func (s *GrokQuotaService) QueryQuota(ctx context.Context, accountID int64) (*GrokQuotaProbeResult, error) {
	billing, billingErr := s.FetchBilling(ctx, accountID)
	if billing != nil && (billing.Credits != nil || billing.Monthly != nil) {
		return &GrokQuotaProbeResult{
			Source:    "active_billing",
			Billing:   billing,
			FetchedAt: time.Now().Unix(),
		}, nil
	}
	result, probeErr := s.ProbeUsage(ctx, accountID)
	if probeErr != nil {
		if billing != nil {
			return &GrokQuotaProbeResult{Source: "active_billing", Billing: billing, FetchedAt: time.Now().Unix()}, nil
		}
		if billingErr != nil {
			return nil, billingErr
		}
		return nil, probeErr
	}
	if result != nil {
		if grokBillingSnapshotHasUsefulData(billing) {
			result.Billing = billing
			result.Source = "hybrid_probe"
		}
	}
	return result, nil
}

func grokBillingSnapshotHasUsefulData(snapshot *xai.BillingSnapshot) bool {
	return snapshot != nil && (snapshot.Credits != nil ||
		snapshot.Monthly != nil ||
		strings.TrimSpace(snapshot.SubscriptionTier) != "" ||
		strings.TrimSpace(snapshot.Email) != "" ||
		snapshot.HasGrokCodeAccess)
}

func (s *GrokQuotaService) ResetQuota(ctx context.Context, accountID int64) (*GrokQuotaResetResult, error) {
	if _, err := s.loadGrokOAuthAccount(ctx, accountID); err != nil {
		return nil, err
	}
	return nil, infraerrors.New(http.StatusNotImplemented, "GROK_QUOTA_RESET_UNSUPPORTED", "xAI does not expose a Grok subscription quota reset endpoint for OAuth accounts")
}

// FetchBilling actively pulls Grok CLI /usage billing endpoints and persists a snapshot
// into account.Extra[grok_billing_snapshot]. Endpoints live on cli-chat-proxy, not api.x.ai.
func (s *GrokQuotaService) FetchBilling(ctx context.Context, accountID int64) (*xai.BillingSnapshot, error) {
	return s.runBillingFlight(ctx, "billing:"+strconv.FormatInt(accountID, 10), func(sharedCtx context.Context) (*xai.BillingSnapshot, error) {
		return s.fetchBilling(sharedCtx, accountID)
	})
}

func (s *GrokQuotaService) fetchBilling(ctx context.Context, accountID int64) (*xai.BillingSnapshot, error) {
	account, token, proxyURL, err := s.prepareProbe(ctx, accountID)
	if err != nil {
		return nil, err
	}

	baseURL := xai.CLIBillingBaseURL()
	callCtx, cancel := context.WithTimeout(ctx, grokBillingUpstreamTimeout)
	defer cancel()

	var tlsProfile *tlsfingerprint.Profile
	if s.tlsFPProfileSvc != nil {
		tlsProfile = s.tlsFPProfileSvc.ResolveTLSProfileForTransport(account, "http")
	}

	snapshot := &xai.BillingSnapshot{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Source:    "active_billing",
	}
	previous := grokBillingSnapshotFromExtra(account.Extra)
	creditsFresh := false
	monthlyFresh := false

	// Parallel-ish sequential is fine; keep simple and share auth headers.
	creditsBody, creditsErr := s.doGrokBillingGET(callCtx, account, token, proxyURL, tlsProfile, baseURL, xai.BillingPathCredits)
	if creditsErr != nil {
		if previous != nil {
			snapshot.Credits = previous.Credits
		}
		snapshot.FetchError = creditsErr.Error()
		slog.Warn("grok_billing_credits_failed", "account_id", account.ID, "err", creditsErr)
	} else if parsed, parseErr := xai.ParseCreditsBilling(creditsBody); parseErr != nil || parsed == nil || parsed.Config == nil {
		if previous != nil {
			snapshot.Credits = previous.Credits
		}
		if parseErr != nil {
			snapshot.FetchError = "parse credits billing: " + parseErr.Error()
		} else {
			snapshot.FetchError = "parse credits billing: missing config"
		}
	} else {
		snapshot.Credits = parsed.Config
		creditsFresh = true
	}

	monthlyBody, monthlyErr := s.doGrokBillingGET(callCtx, account, token, proxyURL, tlsProfile, baseURL, xai.BillingPathMonthly)
	if monthlyErr != nil {
		if previous != nil {
			snapshot.Monthly = previous.Monthly
		}
		if snapshot.FetchError == "" {
			snapshot.FetchError = monthlyErr.Error()
		}
		slog.Warn("grok_billing_monthly_failed", "account_id", account.ID, "err", monthlyErr)
	} else if parsed, parseErr := xai.ParseMonthlyBilling(monthlyBody); parseErr != nil || parsed == nil || parsed.Config == nil {
		if previous != nil {
			snapshot.Monthly = previous.Monthly
		}
		if snapshot.FetchError == "" {
			if parseErr != nil {
				snapshot.FetchError = "parse monthly billing: " + parseErr.Error()
			} else {
				snapshot.FetchError = "parse monthly billing: missing config"
			}
		}
	} else {
		snapshot.Monthly = parsed.Config
		monthlyFresh = true
	}

	userBody, userErr := s.doGrokBillingGET(callCtx, account, token, proxyURL, tlsProfile, baseURL, xai.UserPathSubscription)
	if userErr == nil {
		if parsed, parseErr := xai.ParseUserSubscription(userBody); parseErr == nil && parsed != nil {
			snapshot.SubscriptionTier = parsed.SubscriptionTier
			snapshot.Email = parsed.Email
			snapshot.HasGrokCodeAccess = parsed.HasGrokCodeAccess
		} else if previous != nil {
			snapshot.SubscriptionTier = previous.SubscriptionTier
			snapshot.Email = previous.Email
			snapshot.HasGrokCodeAccess = previous.HasGrokCodeAccess
		}
	} else if previous != nil {
		snapshot.SubscriptionTier = previous.SubscriptionTier
		snapshot.Email = previous.Email
		snapshot.HasGrokCodeAccess = previous.HasGrokCodeAccess
	}

	if !creditsFresh && !monthlyFresh && previous != nil {
		// Retained values keep their original freshness. A failed refresh must not
		// make an old authoritative snapshot appear newly observed.
		snapshot.UpdatedAt = previous.UpdatedAt
	}

	// Persist even partial results so list/passive can show last known state.
	if s.accountRepo != nil {
		if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{
			grokBillingSnapshotExtraKey: snapshot,
		}); err != nil {
			slog.Warn("grok_billing_snapshot_persist_failed", "account_id", account.ID, "err", err)
		}
	}

	if !creditsFresh && !monthlyFresh {
		if snapshot.FetchError != "" {
			return snapshot, infraerrors.Newf(http.StatusBadGateway, "GROK_BILLING_FETCH_FAILED", "failed to fetch grok billing: %s", snapshot.FetchError)
		}
		return snapshot, infraerrors.New(http.StatusBadGateway, "GROK_BILLING_FETCH_FAILED", "failed to fetch grok billing: empty response")
	}
	return snapshot, nil
}

// RefreshAccountUsage runs billing fetch (primary) and best-effort rate-limit probe.
func (s *GrokQuotaService) RefreshAccountUsage(ctx context.Context, accountID int64) {
	if s == nil {
		return
	}
	if _, err := s.QueryQuota(ctx, accountID); err != nil {
		slog.Warn("grok_usage_refresh_failed", "account_id", accountID, "err", err)
	}
}

func (s *GrokQuotaService) runProbeFlight(ctx context.Context, key string, fn func(context.Context) (*GrokQuotaProbeResult, error)) (*GrokQuotaProbeResult, error) {
	resultCh := s.probeFlight.DoChan(key, func() (any, error) {
		return fn(context.WithoutCancel(ctx))
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		value, _ := result.Val.(*GrokQuotaProbeResult)
		return value, nil
	}
}

func (s *GrokQuotaService) runBillingFlight(ctx context.Context, key string, fn func(context.Context) (*xai.BillingSnapshot, error)) (*xai.BillingSnapshot, error) {
	resultCh := s.probeFlight.DoChan(key, func() (any, error) {
		return fn(context.WithoutCancel(ctx))
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			value, _ := result.Val.(*xai.BillingSnapshot)
			return value, result.Err
		}
		value, _ := result.Val.(*xai.BillingSnapshot)
		return value, nil
	}
}

func (s *GrokQuotaService) doGrokBillingGET(
	ctx context.Context,
	account *Account,
	token, proxyURL string,
	tlsProfile *tlsfingerprint.Profile,
	baseURL, pathWithQuery string,
) ([]byte, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "GROK_QUOTA_NOT_CONFIGURED", "grok quota service is not configured")
	}
	targetURL, err := xai.BuildBillingURL(baseURL, pathWithQuery)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadRequest, "GROK_BILLING_URL_INVALID", "invalid billing url: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusInternalServerError, "GROK_BILLING_REQUEST_BUILD_FAILED", "failed to build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-grok-client-version", grokClientVersionHeader)
	req.Header.Set("x-grok-client-mode", grokClientModeHeader)
	applyDefaultGrokUpstreamHeaders(req)
	applyGrokTLSProfileHeaders(req, tlsProfile)

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, maxInt(account.Concurrency, 1), tlsProfile)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_BILLING_REQUEST_FAILED", "upstream billing request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		bodyText := truncate(strings.TrimSpace(string(body)), 240)
		return nil, infraerrors.Newf(mapUpstreamStatus(resp.StatusCode), "GROK_BILLING_UPSTREAM_ERROR", "upstream returned %d for %s: %s", resp.StatusCode, pathWithQuery, bodyText)
	}
	return body, nil
}

func (s *GrokQuotaService) prepareProbe(ctx context.Context, accountID int64) (*Account, string, string, error) {
	if s == nil || s.tokenProvider == nil || s.httpUpstream == nil {
		return nil, "", "", infraerrors.New(http.StatusInternalServerError, "GROK_QUOTA_NOT_CONFIGURED", "grok quota service is not configured")
	}
	account, err := s.loadGrokOAuthAccount(ctx, accountID)
	if err != nil {
		return nil, "", "", err
	}

	token, err := s.tokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return nil, "", "", infraerrors.Newf(http.StatusBadGateway, "GROK_QUOTA_TOKEN_UNAVAILABLE", "failed to acquire access token: %v", err)
	}
	if strings.TrimSpace(token) == "" {
		return nil, "", "", infraerrors.New(http.StatusBadGateway, "GROK_QUOTA_TOKEN_UNAVAILABLE", "access token is empty")
	}

	return account, token, s.resolveProxyURL(ctx, account), nil
}

func (s *GrokQuotaService) resolveProxyURL(ctx context.Context, account *Account) string {
	if account == nil || account.ProxyID == nil {
		return ""
	}
	switch {
	case account.Proxy != nil:
		return account.Proxy.URL()
	case s != nil && s.proxyRepo != nil:
		if proxy, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && proxy != nil {
			return proxy.URL()
		}
	}
	return ""
}

func (s *GrokQuotaService) loadGrokOAuthAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, infraerrors.New(http.StatusInternalServerError, "GROK_QUOTA_NOT_CONFIGURED", "grok quota service is not configured")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusNotFound, "GROK_QUOTA_ACCOUNT_NOT_FOUND", "account not found: %v", err)
	}
	if account == nil {
		return nil, infraerrors.New(http.StatusNotFound, "GROK_QUOTA_ACCOUNT_NOT_FOUND", "account not found")
	}
	if account.Platform != PlatformGrok {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_QUOTA_INVALID_PLATFORM", "account is not a Grok account")
	}
	if account.Type != AccountTypeOAuth {
		return nil, infraerrors.New(http.StatusBadRequest, "GROK_QUOTA_INVALID_TYPE", "account is not an OAuth account")
	}
	return account, nil
}

func grokQuotaProbeModel() string {
	return grokQuotaDefaultModel
}

func buildGrokQuotaProbeBody(modelOrAccount any) ([]byte, error) {
	model := resolveGrokQuotaProbeModel(modelOrAccount)
	if model == "" {
		model = grokQuotaDefaultModel
	}
	return json.Marshal(map[string]any{
		"model":             model,
		"input":             grokQuotaProbeInput,
		"max_output_tokens": 1,
		"store":             false,
	})
}

func resolveGrokQuotaProbeModel(modelOrAccount any) string {
	switch v := modelOrAccount.(type) {
	case string:
		return strings.TrimSpace(v)
	case *Account:
		if v != nil {
			// Probe from the public Grok alias first so account-level model_mapping
			// stays consistent with real traffic (for example grok -> grok-4.3).
			if mapped, matched := v.ResolveMappedModel("grok"); matched && strings.TrimSpace(mapped) != "" {
				return strings.TrimSpace(mapped)
			}
			if mapped, matched := v.ResolveMappedModel(grokQuotaProbeModel()); matched && strings.TrimSpace(mapped) != "" {
				return strings.TrimSpace(mapped)
			}
		}
	}
	return grokQuotaProbeModel()
}
