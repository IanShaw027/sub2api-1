package service

import (
	"fmt"
	"strings"
)

type openAIResponsesSessionWindow struct {
	LatestResponseID string `json:"latest_response_id,omitempty"`
	PromptCacheKey   string `json:"prompt_cache_key,omitempty"`
	ReplayInputRaw   []byte `json:"replay_input_raw,omitempty"`
	Compacted        bool   `json:"compacted,omitempty"`
}

func (w openAIResponsesSessionWindow) clone() openAIResponsesSessionWindow {
	cloned := w
	if len(w.ReplayInputRaw) > 0 {
		cloned.ReplayInputRaw = append([]byte(nil), w.ReplayInputRaw...)
	}
	return cloned
}

func normalizeOpenAIResponsesSessionWindow(window openAIResponsesSessionWindow) openAIResponsesSessionWindow {
	normalized := window.clone()
	normalized.LatestResponseID = strings.TrimSpace(normalized.LatestResponseID)
	normalized.PromptCacheKey = strings.TrimSpace(normalized.PromptCacheKey)
	return normalized
}

func openAIResponsesSessionWindowHasData(window openAIResponsesSessionWindow) bool {
	return strings.TrimSpace(window.LatestResponseID) != "" ||
		strings.TrimSpace(window.PromptCacheKey) != "" ||
		len(window.ReplayInputRaw) > 0 ||
		window.Compacted
}

func openAIResponsesSessionWindowCacheKey(apiKeyID int64, sessionHash string) string {
	hash := strings.TrimSpace(sessionHash)
	if hash == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s", apiKeyID, hash)
}
