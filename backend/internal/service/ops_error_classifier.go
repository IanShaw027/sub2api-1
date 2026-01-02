package service

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
)

// ErrorClassification 错误分类结果
type ErrorClassification struct {
	Type                string  // 错误类型
	Phase               string  // 错误阶段
	Severity            string  // 严重程度
	IsRetryable         bool    // 是否可重试
	UpstreamErrorDetail *string // 上游错误详情
	ErrorSource         string  // 错误来源
	ErrorOwner          string  // 错误责任方
	AccountStatus       string  // 账号状态（仅上游错误）
}

// ClassifyError 根据错误信息自动分类错误
func ClassifyError(
	errorType string,
	errorPhase string,
	statusCode int,
	upstreamStatusCode *int,
	err error,
) ErrorClassification {
	result := ErrorClassification{}

	// 优先检查 timeout_error 和 network_error
	if err != nil {
		errMsg := err.Error()
		if strings.HasPrefix(errMsg, "timeout_error:") {
			detail := strings.TrimPrefix(errMsg, "timeout_error: ")
			return ErrorClassification{
				Type:                "timeout_error",
				Phase:               "upstream",
				Severity:            "P1",
				IsRetryable:         true,
				UpstreamErrorDetail: &detail,
			}
		}
		if strings.HasPrefix(errMsg, "network_error:") {
			detail := strings.TrimPrefix(errMsg, "network_error: ")
			return ErrorClassification{
				Type:                "network_error",
				Phase:               "upstream",
				Severity:            "P1",
				IsRetryable:         true,
				UpstreamErrorDetail: &detail,
			}
		}
	}

	// 1. 判断错误来源
	result.ErrorSource = classifyErrorSource(errorType, errorPhase, err)

	// 2. 判断错误责任方
	result.ErrorOwner = classifyErrorOwner(errorType, errorPhase, result.ErrorSource)

	// 3. 判断账号状态（仅上游错误）
	if result.ErrorSource == "upstream_business" || result.ErrorSource == "upstream_system" {
		result.AccountStatus = classifyAccountStatus(errorType, upstreamStatusCode)
	}

	return result
}

// classifyErrorSource 判断错误来源
func classifyErrorSource(errorType, errorPhase string, err error) string {
	// 基础设施错误
	if errorType == "database_error" || errorType == "redis_error" || errorType == "internal_error" {
		return "infrastructure"
	}

	// 网络错误 - 优先使用类型断言
	if err != nil {
		// 检查是否为网络超时
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return "upstream_system"
		}

		// 检查是否为网络操作错误
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			// 检查底层系统错误
			if errors.Is(opErr.Err, syscall.ECONNREFUSED) ||
				errors.Is(opErr.Err, syscall.EHOSTUNREACH) ||
				errors.Is(opErr.Err, syscall.ENETUNREACH) {
				return "upstream_system"
			}
		}

		// 检查 DNS 错误
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			return "upstream_system"
		}

		// 降级到字符串匹配（兼容性）
		errStr := err.Error()
		if strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "no such host") ||
			strings.Contains(errStr, "network") {
			return "upstream_system"
		}
	}

	// 根据 phase 和 type 判断
	switch errorPhase {
	case "auth", "billing":
		return "downstream_business"

	case "concurrency", "response":
		return "downstream_system"

	case "upstream":
		// 上游错误需要进一步区分业务错误和系统错误
		switch errorType {
		case "authentication_error", "permission_error", "rate_limit_error", "invalid_request_error":
			return "upstream_business"
		case "upstream_error", "overloaded_error", "timeout_error":
			return "upstream_system"
		default:
			return "upstream_system"
		}

	case "scheduling":
		return "internal"

	case "internal":
		return "infrastructure"

	default:
		return "internal"
	}
}

// classifyErrorOwner 判断错误责任方
func classifyErrorOwner(errorType, errorPhase, errorSource string) string {
	// 客户端错误
	if errorType == "authentication_error" && errorPhase == "auth" {
		return "client"
	}
	if errorType == "billing_error" || errorType == "subscription_error" {
		return "client"
	}
	if errorType == "invalid_request_error" && errorPhase == "response" {
		return "client"
	}

	// 上游供应商错误
	if errorSource == "upstream_business" || errorSource == "upstream_system" {
		return "provider"
	}

	// 基础设施错误
	if errorSource == "infrastructure" {
		return "infrastructure"
	}

	// 平台错误（默认）
	return "platform"
}

// classifyAccountStatus 判断账号状态
func classifyAccountStatus(errorType string, upstreamStatusCode *int) string {
	if upstreamStatusCode == nil {
		return ""
	}

	switch *upstreamStatusCode {
	case 401:
		return "auth_failed"
	case 403:
		return "permission_denied"
	case 429:
		return "rate_limited"
	case 402:
		return "quota_exceeded"
	default:
		if *upstreamStatusCode >= 500 {
			return "error"
		}
		return ""
	}
}

// ClassifyNetworkError 分类网络错误类型
func ClassifyNetworkError(err error) string {
	if err == nil {
		return ""
	}

	// 优先使用类型断言
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return "connection_refused"
		}
		if errors.Is(opErr.Err, syscall.ECONNRESET) {
			return "connection_reset"
		}
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns_error"
	}

	// 降级到字符串匹配（兼容性）
	errStr := err.Error()
	if strings.Contains(errStr, "connection refused") {
		return "connection_refused"
	}
	if strings.Contains(errStr, "no such host") {
		return "dns_error"
	}
	if strings.Contains(errStr, "connection reset") {
		return "connection_reset"
	}
	if strings.Contains(errStr, "TLS") || strings.Contains(errStr, "certificate") {
		return "tls_error"
	}
	if strings.Contains(errStr, "EOF") {
		return "unexpected_eof"
	}

	return "network_error"
}

// ExtractRetryAfter 从响应头中提取 Retry-After 值（秒）
// 支持两种格式：
// 1. 整数（秒）: "120"
// 2. HTTP-date: "Sat, 03 Jan 2026 12:00:00 GMT"
func ExtractRetryAfter(retryAfterHeader string) *int {
	if retryAfterHeader == "" {
		return nil
	}

	// 尝试解析为整数（秒）
	var seconds int
	if _, err := fmt.Sscanf(retryAfterHeader, "%d", &seconds); err == nil {
		return &seconds
	}

	// 尝试解析为 HTTP-date 格式（RFC 1123）
	httpDateFormats := []string{
		time.RFC1123,     // "Mon, 02 Jan 2006 15:04:05 MST"
		time.RFC1123Z,    // "Mon, 02 Jan 2006 15:04:05 -0700"
		time.RFC850,      // "Monday, 02-Jan-06 15:04:05 MST"
		time.ANSIC,       // "Mon Jan _2 15:04:05 2006"
	}

	for _, format := range httpDateFormats {
		if retryTime, err := time.Parse(format, retryAfterHeader); err == nil {
			// 计算从现在到指定时间的秒数
			duration := time.Until(retryTime)
			if duration > 0 {
				seconds := int(duration.Seconds())
				return &seconds
			}
			// 如果时间已过，返回 0
			zero := 0
			return &zero
		}
	}

	return nil
}
