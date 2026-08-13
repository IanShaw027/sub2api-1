package handler

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

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
