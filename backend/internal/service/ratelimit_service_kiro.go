package service

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const kiroTempUnsched429MaxWindow = 5 * time.Minute
func parseRetryAfterHeader(raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		resetAt := time.Now().Add(time.Duration(seconds) * time.Second)
		return &resetAt
	}
	if when, err := http.ParseTime(raw); err == nil {
		return &when
	}
	return nil
}
var kiro429ResetTimePaths = []string{
	"retry_after",
	"retryAfter",
	"retry_after_seconds",
	"retryAfterSeconds",
	"reset_at",
	"resetAt",
	"rate_limit_reset_at",
	"rateLimitResetAt",
	"throttle_until",
	"throttleUntil",
	"deadline",
	"data.retry_after",
	"data.retryAfter",
	"data.retry_after_seconds",
	"data.retryAfterSeconds",
	"data.reset_at",
	"data.resetAt",
	"data.rate_limit_reset_at",
	"data.rateLimitResetAt",
	"data.throttle_until",
	"data.throttleUntil",
	"data.deadline",
	"error.retry_after",
	"error.retryAfter",
	"error.retry_after_seconds",
	"error.retryAfterSeconds",
	"error.reset_at",
	"error.resetAt",
	"error.rate_limit_reset_at",
	"error.rateLimitResetAt",
	"error.throttle_until",
	"error.throttleUntil",
	"error.deadline",
}

func parseKiro429ResetAt(headers http.Header, responseBody []byte) *time.Time {
	if resetAt := parseRetryAfterHeader(headers.Get("Retry-After")); resetAt != nil {
		return resetAt
	}
	return parseKiro429ResetAtFromBody(responseBody)
}

func parseKiro429ResetAtFromBody(responseBody []byte) *time.Time {
	if len(responseBody) == 0 {
		return nil
	}
	for _, path := range kiro429ResetTimePaths {
		if resetAt := parseKiro429ResetField(path, gjson.GetBytes(responseBody, path)); resetAt != nil {
			return resetAt
		}
	}
	return nil
}

func parseKiro429ResetField(path string, value gjson.Result) *time.Time {
	if !value.Exists() {
		return nil
	}

	switch value.Type {
	case gjson.Number:
		return parseKiro429ResetNumber(path, value.Float())
	case gjson.String:
		return parseKiro429ResetString(path, value.String())
	default:
		return nil
	}
}

func parseKiro429ResetString(path string, raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if n, err := strconv.ParseFloat(raw, 64); err == nil {
		return parseKiro429ResetNumber(path, n)
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if when, err := time.Parse(layout, raw); err == nil {
			return &when
		}
	}
	return parseRetryAfterHeader(raw)
}

func parseKiro429ResetNumber(path string, raw float64) *time.Time {
	if raw <= 0 {
		return nil
	}
	value := int64(raw)
	lowerPath := strings.ToLower(strings.TrimSpace(path))
	if strings.Contains(lowerPath, "reset") || strings.Contains(lowerPath, "until") || strings.Contains(lowerPath, "deadline") || strings.HasSuffix(lowerPath, "_at") || strings.HasSuffix(lowerPath, "at") {
		if value >= 1_000_000_000_000 {
			when := time.UnixMilli(value)
			return &when
		}
		if value >= 1_000_000_000 {
			when := time.Unix(value, 0)
			return &when
		}
	}
	when := time.Now().Add(time.Duration(raw * float64(time.Second)))
	return &when
}

func kiro429LooksQuotaExhausted(responseBody []byte) bool {
	detail := strings.TrimSpace(kiroErrorDetailFromBody(responseBody))
	if detail == "" {
		detail = strings.TrimSpace(string(responseBody))
	}
	return kiroQuotaExhaustedDetail(detail)
}

func (s *RateLimitService) applyKiro429ExplicitCooldown(ctx context.Context, account *Account, resetAt time.Time, responseBody []byte, reason string) {
	if account == nil {
		return
	}
	now := time.Now()
	if !resetAt.After(now) {
		slog.Info("kiro_429_explicit_reset_expired", "account_id", account.ID, "reset_at", resetAt)
		return
	}
	if resetAt.Sub(now) <= kiroTempUnsched429MaxWindow {
		if s.persistTempUnschedulableState(ctx, account, resetAt, http.StatusTooManyRequests, "kiro_explicit_reset", -1, responseBody, "kiro_429_temp_unschedulable") {
			slog.Info("kiro_account_temp_unschedulable_retry_after", "account_id", account.ID, "until", resetAt, "reset_in", time.Until(resetAt).Truncate(time.Second))
		}
		return
	}
	s.notifyAccountSchedulingBlocked(account, resetAt, "kiro_429_rate_limited")
	if err := s.accountRepo.SetRateLimited(ctx, account.ID, resetAt); err != nil {
		slog.Warn("rate_limit_set_failed", "account_id", account.ID, "error", err)
		return
	}
	slog.Info("kiro_account_rate_limited_retry_after", "account_id", account.ID, "reset_at", resetAt, "reset_in", time.Until(resetAt).Truncate(time.Second))
}

