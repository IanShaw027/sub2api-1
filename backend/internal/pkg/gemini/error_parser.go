package gemini

import (
	"encoding/json"
	"regexp"
)

var messageRegex = regexp.MustCompile(`"message"\s*:\s*"([^"]+)"`)

// ParseErrorMessage extracts error message from Gemini API response body.
func ParseErrorMessage(body []byte) string {
	// Try JSON parsing first.
	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return errResp.Error.Message
	}

	// Fallback to regex.
	if matches := messageRegex.FindSubmatch(body); len(matches) > 1 {
		return string(matches[1])
	}

	return ""
}
