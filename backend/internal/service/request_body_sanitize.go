package service

import (
	"encoding/json"
	"strings"
)

// sanitizeRequestBody redacts obvious secrets from request payloads before persisting/logging them.
// It is best-effort (invalid JSON returns empty string) and capped to avoid large allocations.
func sanitizeRequestBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	// Limit size to 10KB.
	const maxSize = 10 * 1024
	if len(body) > maxSize {
		body = body[:maxSize]
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}

	sensitiveFields := []string{
		"api_key", "apiKey", "API_KEY",
		"password", "pwd", "passwd",
		"email", "mail",
		"phone", "mobile", "tel",
		"credit_card", "card_number",
		"token", "access_token", "refresh_token",
	}

	// Recursive sanitizer, supporting map, slice and basic types.
	var sanitize func(any) any
	sanitize = func(v any) any {
		switch val := v.(type) {
		case map[string]any:
			result := make(map[string]any, len(val))
			for k, inner := range val {
				lowerKey := strings.ToLower(k)
				isSensitive := false
				for _, field := range sensitiveFields {
					if strings.Contains(lowerKey, strings.ToLower(field)) {
						isSensitive = true
						break
					}
				}
				if isSensitive {
					result[k] = "***"
					continue
				}
				result[k] = sanitize(inner)
			}
			return result
		case []any:
			result := make([]any, len(val))
			for i, item := range val {
				result[i] = sanitize(item)
			}
			return result
		default:
			return val
		}
	}

	sanitized, _ := json.Marshal(sanitize(data))
	return string(sanitized)
}
