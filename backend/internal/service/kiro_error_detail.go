package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func kiroErrorDetailFromBody(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return trimmed
	}

	detail := kiroErrorDetail(raw)
	if detail != "" {
		return detail
	}
	return trimmed
}

func kiroErrorDetail(raw any) string {
	switch typed := raw.(type) {
	case map[string]any:
		if data, ok := typed["data"]; ok {
			if detail := kiroErrorDetail(data); detail != "" {
				return detail
			}
		}
		if errValue, ok := typed["error"]; ok {
			switch nested := errValue.(type) {
			case map[string]any:
				if detail := kiroErrorDetail(nested); detail != "" {
					return detail
				}
			}
		}
		if errorsValue, ok := typed["errors"]; ok {
			if detail := kiroErrorDetail(errorsValue); detail != "" {
				return detail
			}
		}

		parts := []string{
			kiroErrorStringFromMap(typed, "error", "code", "errorCode"),
			kiroErrorStringFromMap(typed, "error_description", "errorDescription", "message", "detail", "reason"),
		}
		return strings.TrimSpace(strings.Join(filterEmptyStrings(parts), ": "))
	case []any:
		for _, item := range typed {
			if detail := kiroErrorDetail(item); detail != "" {
				return detail
			}
		}
	case string:
		return strings.TrimSpace(typed)
	}
	return ""
}

func kiroErrorStringFromMap(values map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case json.Number:
			return typed.String()
		case float64:
			if typed == float64(int64(typed)) {
				return fmt.Sprintf("%.0f", typed)
			}
			return fmt.Sprintf("%v", typed)
		}
	}
	return ""
}

func kiroHTTPStatusErrorMessage(prefix string, statusCode int, body []byte) string {
	if detail := kiroErrorDetailFromBody(body); detail != "" {
		return fmt.Sprintf("%s returned %d: %s", prefix, statusCode, detail)
	}

	switch statusCode {
	case http.StatusBadRequest:
		return fmt.Sprintf("%s returned %d: request rejected by upstream; check whether the token has expired, the selected model is available, and the account still has access", prefix, statusCode)
	case http.StatusUnauthorized:
		return fmt.Sprintf("%s returned %d: upstream rejected the current token; reauthorize or refresh the account and try again", prefix, statusCode)
	case http.StatusForbidden:
		return fmt.Sprintf("%s returned %d: upstream denied access; verify the account still has permission to use Kiro and the selected model", prefix, statusCode)
	default:
		return fmt.Sprintf("%s returned %d", prefix, statusCode)
	}
}

func buildKiroOAuthTokenExchangeError(statusCode int, body []byte) error {
	detail := kiroErrorDetailFromBody(body)
	if statusCode == http.StatusBadRequest {
		base := "Kiro 拒绝兑换授权码，授权码可能已过期、已被使用，或回调地址与本次授权流程不一致。请重新生成授权链接后重试"
		normalized := strings.ToLower(detail)
		if strings.Contains(normalized, "redirect") {
			base = "Kiro 拒绝兑换授权码，回调地址与授权时使用的不一致。请重新生成授权链接，并把同一轮授权流程最新地址栏中的完整回调 URL 粘贴回来"
		}
		if detail != "" {
			return fmt.Errorf("%s；上游返回：%s", base, detail)
		}
		return errors.New(base)
	}
	return fmt.Errorf("%s", kiroHTTPStatusErrorMessage("Kiro OAuth token API", statusCode, body))
}
