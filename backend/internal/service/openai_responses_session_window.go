package service

import (
	"bytes"
	"encoding/json"
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

func parseOpenAIResponsesSessionWindowReplayInput(raw []byte) ([]json.RawMessage, bool, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false, nil
	}
	if bytes.HasPrefix(trimmed, []byte("[")) {
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, true, err
		}
		return items, true, nil
	}
	return []json.RawMessage{append(json.RawMessage(nil), trimmed...)}, true, nil
}
