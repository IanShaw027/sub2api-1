package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const claudeTelemetryBatchURL = "https://api.anthropic.com/api/event_logging/batch"
const claudeTelemetryForwardTimeout = 3 * time.Second

// ForwardClaudeTelemetryBatch sanitizes and forwards Claude Code telemetry using
// Anthropic OAuth/SetupToken accounts only. If no OAuth account is available it
// is a no-op so API-key accounts are never used for telemetry forwarding.
func (s *GatewayService) ForwardClaudeTelemetryBatch(ctx context.Context, groupID *int64, body []byte) (int, error) {
	baseCtx := context.Background()
	if ctx != nil {
		baseCtx = context.WithoutCancel(ctx)
	}
	forwardCtx, cancel := context.WithTimeout(baseCtx, claudeTelemetryForwardTimeout)
	defer cancel()

	account, err := s.selectClaudeTelemetryOAuthAccount(forwardCtx, groupID)
	if err != nil {
		return http.StatusOK, err
	}
	if account == nil {
		return http.StatusOK, nil
	}

	token, err := s.getClaudeTelemetryOAuthToken(forwardCtx, account)
	if err != nil {
		return http.StatusOK, err
	}

	fp := &Fingerprint{}
	if s.identityService != nil {
		if got, fpErr := s.identityService.GetOrCreateFingerprint(forwardCtx, account.ID, http.Header{}); fpErr == nil && got != nil {
			fp = got
		}
	}
	profile := buildAccountEnvProfile(account.ID, fp)
	cleaned, ok := SanitizeClaudeTelemetryBatch(body, claudeTelemetrySanitizeOptions(account, fp, profile))
	if !ok {
		// fail-closed：无法确保脱敏时丢弃遥测，绝不转发未脱敏原文到 Anthropic。
		slog.Warn("dropping claude telemetry batch: sanitization could not be verified", "account_id", account.ID)
		return http.StatusOK, nil
	}

	req, err := http.NewRequestWithContext(forwardCtx, http.MethodPost, claudeTelemetryBatchURL, bytes.NewReader(cleaned))
	if err != nil {
		return http.StatusOK, err
	}
	setHeaderRaw(req.Header, "authorization", "Bearer "+token)
	setHeaderRaw(req.Header, "content-type", "application/json")
	if s.identityService != nil && fp != nil {
		s.identityService.ApplyFingerprint(req, fp)
	} else if fp != nil && fp.UserAgent != "" {
		setHeaderRaw(req.Header, "user-agent", fp.UserAgent)
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	if s == nil || s.httpUpstream == nil {
		return http.StatusOK, errors.New("http upstream not configured")
	}
	tlsRuntime := s.resolveGatewayTLSFingerprintRuntime(forwardCtx, nil, account, "http")
	applyGatewayTLSFingerprintRuntime(req, tlsRuntime)
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
	if err != nil {
		return http.StatusOK, err
	}
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	if resp == nil {
		return http.StatusOK, nil
	}
	return resp.StatusCode, nil
}

func (s *GatewayService) selectClaudeTelemetryOAuthAccount(ctx context.Context, groupID *int64) (*Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, nil
	}
	var accounts []Account
	var err error
	if groupID != nil && *groupID > 0 {
		accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformAnthropic)
	} else {
		accounts, err = s.accountRepo.ListSchedulableByPlatform(ctx, PlatformAnthropic)
	}
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		acc := &accounts[i]
		if acc.Platform == PlatformAnthropic && acc.IsOAuth() && acc.IsSchedulable() {
			return acc, nil
		}
	}
	return nil, nil
}

func (s *GatewayService) getClaudeTelemetryOAuthToken(ctx context.Context, account *Account) (string, error) {
	if account == nil || account.Platform != PlatformAnthropic || !account.IsOAuth() {
		return "", fmt.Errorf("claude telemetry requires anthropic oauth account")
	}
	if account.Type == AccountTypeOAuth && s != nil && s.claudeTokenProvider != nil {
		if token, err := s.claudeTokenProvider.GetAccessToken(ctx, account); err == nil && token != "" {
			return token, nil
		} else if err != nil {
			slog.Warn("claude_telemetry_token_provider_failed", "account_id", account.ID, "error", err)
		}
	}
	token := account.GetCredential("access_token")
	if token == "" {
		return "", errors.New("access_token not found in credentials")
	}
	return token, nil
}

func claudeTelemetrySanitizeOptions(account *Account, fp *Fingerprint, profile *AccountEnvProfile) ClaudeTelemetrySanitizeOptions {
	deviceID := ""
	if fp != nil {
		deviceID = fp.ClientID
	}
	if deviceID == "" && account != nil {
		deviceID = account.GetExtraString("device_id")
	}
	email := ""
	if profile != nil {
		email = profile.Email
	}
	var canonicalEnv map[string]any
	platform := ""
	arch := ""
	if profile != nil {
		platform = profile.Platform
		arch = profile.Arch
		canonicalEnv = map[string]any{
			"platform": profile.Platform,
			"arch":     profile.Arch,
			"shell":    profile.Shell,
			"is_ci":    false,
		}
	}
	return ClaudeTelemetrySanitizeOptions{
		DeviceID:          deviceID,
		Email:             email,
		CanonicalEnv:      canonicalEnv,
		Platform:          platform,
		Arch:              arch,
		ConstrainedMemory: 8 * 1024 * 1024 * 1024,
		RSSRange:          [2]int64{300000000, 450000000},
		HeapTotalRange:    [2]int64{100000000, 180000000},
		HeapUsedRange:     [2]int64{50000000, 120000000},
	}
}
