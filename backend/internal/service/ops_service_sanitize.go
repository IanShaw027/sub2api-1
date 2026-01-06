package service

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

type opsRecordErrorGuardKey struct{}

var (
	// 敏感信息正则表达式
	// NOTE:
	// - API Key / Token 可能包含 Base64 字符（+ / =）或 URL 编码字符（%）
	// - 一些 key（如 sk-proj-...）还会包含 '-' 等分隔符
	apiKeyRegex     = regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9_\-+/=%]{8,}|(?:key|apikey|api_key)[=:]\s*[a-zA-Z0-9_\-+/=%]{12,})`)
	tokenRegex      = regexp.MustCompile(`(?i)(bearer\s+[a-zA-Z0-9_\-\.+/=%]+|token[=:]\s*[a-zA-Z0-9_\-\.+/=%]+)`)
	emailRegex      = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	queryParamRegex = regexp.MustCompile(`(?i)([?&](?:key|apikey|api_key|token|access_token|refresh_token|client_secret)=)[^&\s"]+`)
)

// sanitizeAPIKey 脱敏 API Key，只保留前 8 位
func sanitizeAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "***"
}

// sanitizeToken 完全脱敏 Token
func sanitizeToken(token string) string {
	return "***"
}

// hashSensitiveData 对敏感数据进行哈希处理
func hashSensitiveData(data string) string {
	if data == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16] // 只保留前 16 位哈希
}

// sanitizeErrorMessage 统一脱敏错误消息中的敏感信息
func sanitizeErrorMessage(msg string) string {
	if msg == "" {
		return msg
	}

	// 脱敏 API Key（保留前 8 位）
	msg = apiKeyRegex.ReplaceAllStringFunc(msg, func(match string) string {
		// 如果匹配包含 key= 前缀，保留前缀
		if idx := strings.Index(strings.ToLower(match), "key="); idx >= 0 {
			prefix := match[:idx+4]
			key := match[idx+4:]
			return prefix + sanitizeAPIKey(key)
		}
		if idx := strings.Index(strings.ToLower(match), "key:"); idx >= 0 {
			prefix := match[:idx+4]
			key := match[idx+4:]
			return prefix + sanitizeAPIKey(strings.TrimSpace(key))
		}
		return sanitizeAPIKey(match)
	})

	// 脱敏 Token（完全脱敏）
	msg = tokenRegex.ReplaceAllString(msg, "***")

	// 脱敏 Email（哈希处理）
	msg = emailRegex.ReplaceAllStringFunc(msg, func(match string) string {
		return "email_" + hashSensitiveData(match)
	})

	// 脱敏 URL 查询参数中的敏感信息
	msg = queryParamRegex.ReplaceAllString(msg, `$1***`)

	return msg
}
