package handler

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var selectionModelRateLimitedPattern = regexp.MustCompile(`(?:model_rate_limited|rate_limited)=(\d+)`)

// classifySelectionFailureError preserves the useful scheduler diagnosis on
// the client-facing error path. The scheduler includes compact counters in
// ErrNoAvailableAccounts; when model_rate_limited is present, a generic 503
// hides the actionable reason and prevents clients from applying their retry
// policy.
func classifySelectionFailureError(err error, fallback noAccountErrorClassification) noAccountErrorClassification {
	if err == nil {
		return fallback
	}
	message := strings.ToLower(err.Error())
	match := selectionModelRateLimitedPattern.FindStringSubmatch(message)
	if len(match) != 2 {
		return fallback
	}
	count, parseErr := strconv.Atoi(match[1])
	if parseErr != nil || count <= 0 {
		return fallback
	}
	return noAccountErrorClassification{
		Status:  http.StatusTooManyRequests,
		ErrType: "rate_limit_error",
		Message: "All available accounts are currently rate-limited. Please retry later.",
	}
}

func buildOpenAISelectionFailureMessage(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	if errors.Is(err, service.ErrNoAvailableCompactAccounts) {
		return fallback
	}

	message := strings.TrimSpace(err.Error())
	messageLower := strings.ToLower(message)

	if idx := strings.LastIndex(messageLower, "supporting model:"); idx >= 0 {
		suffix := strings.TrimSpace(message[idx+len("supporting model:"):])
		if suffix != "" {
			if stop := strings.Index(suffix, "("); stop >= 0 {
				suffix = strings.TrimSpace(suffix[:stop])
			}
			if stop := strings.Index(strings.ToLower(suffix), ": no available accounts"); stop >= 0 {
				suffix = strings.TrimSpace(suffix[:stop])
			}
			if suffix != "" {
				return fmt.Sprintf("No available accounts supporting model: %s", suffix)
			}
		}
	}

	if errors.Is(err, service.ErrNoAvailableAccounts) ||
		strings.Contains(messageLower, "no available openai accounts") ||
		strings.Contains(messageLower, "no available accounts") {
		return "No available accounts"
	}

	return fallback
}

func buildOpenAISelectionExhaustedMessage(err error, fallback string, hadLocalExclusions bool) string {
	if hadLocalExclusions {
		return fallback
	}
	return buildOpenAISelectionFailureMessage(err, fallback)
}
