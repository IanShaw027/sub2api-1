// Package model 定义服务层使用的数据模型。
package model

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	TLSFingerprintRouterMatchContains = "contains"
	TLSFingerprintRouterMatchPrefix   = "prefix"
	TLSFingerprintRouterMatchExact    = "exact"
	TLSFingerprintRouterMatchRegex    = "regex"

	TLSFingerprintRouterTransportHTTP      = "http"
	TLSFingerprintRouterTransportWebSocket = "websocket"
	TLSFingerprintRouterTransportHTTP1     = "http1"
	TLSFingerprintRouterTransportH2        = "h2"
	TLSFingerprintRouterTransportWSHTTP1   = "websocket-http1"
	TLSFingerprintRouterTransportWSH2      = "websocket-h2"
)

// TLSFingerprintRouter TLS 指纹路由规则集。
type TLSFingerprintRouter struct {
	ID          int64                      `json:"id"`
	Name        string                     `json:"name"`
	Description *string                    `json:"description"`
	Enabled     bool                       `json:"enabled"`
	Rules       []TLSFingerprintRouterRule `json:"rules"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
}

// TLSFingerprintRouterRule 根据入站 UA 选择上游 TLS profile 和可选上游头覆盖。
type TLSFingerprintRouterRule struct {
	Name                    string `json:"name"`
	Enabled                 bool   `json:"enabled"`
	Transport               string `json:"transport,omitempty"`
	MatchType               string `json:"match_type"`
	Pattern                 string `json:"pattern"`
	CaseSensitive           bool   `json:"case_sensitive"`
	TLSFingerprintProfileID int64  `json:"tls_fingerprint_profile_id"`
	// OS / ClientType: 命中后输出的「判定维度」。非空时，账号侧据此在
	// tls_fingerprint_bindings 矩阵中解析出该账号专属的具体模板（每账号可不同）。
	// 留空表示该规则不参与维度判定，沿用 TLSFingerprintProfileID 直出（向后兼容）。
	OS                 string `json:"os,omitempty"`
	ClientType         string `json:"client_type,omitempty"`
	UpstreamUserAgent  string `json:"upstream_user_agent,omitempty"`
	UpstreamOriginator string `json:"upstream_originator,omitempty"`
}

// Validate 验证路由配置的有效性。
func (r *TLSFingerprintRouter) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	for i := range r.Rules {
		if err := r.Rules[i].Validate(i); err != nil {
			return err
		}
	}
	return nil
}

// Validate 验证单条路由规则。禁用规则允许暂存未完成配置。
func (r *TLSFingerprintRouterRule) Validate(index int) error {
	if r == nil || !r.Enabled {
		return nil
	}
	prefix := fmt.Sprintf("rules[%d]", index)
	if strings.TrimSpace(r.Name) == "" {
		return &ValidationError{Field: prefix + ".name", Message: "rule name is required"}
	}
	if strings.TrimSpace(r.Pattern) == "" {
		return &ValidationError{Field: prefix + ".pattern", Message: "rule pattern is required"}
	}
	// 规则必须能产出 profile：要么直出 profileID，要么输出判定维度（OS/ClientType）
	// 让账号矩阵去解析。两者都没有则无效。
	hasDimension := strings.TrimSpace(r.OS) != "" || strings.TrimSpace(r.ClientType) != ""
	if r.TLSFingerprintProfileID <= 0 && !hasDimension {
		return &ValidationError{Field: prefix + ".tls_fingerprint_profile_id", Message: "either tls fingerprint profile or os/client_type dimension is required"}
	}
	switch strings.ToLower(strings.TrimSpace(r.OS)) {
	case "", "windows", "macos", "linux":
	default:
		return &ValidationError{Field: prefix + ".os", Message: "os must be empty, windows, macos, or linux"}
	}
	switch strings.TrimSpace(r.Transport) {
	case "",
		TLSFingerprintRouterTransportHTTP,
		TLSFingerprintRouterTransportWebSocket,
		TLSFingerprintRouterTransportHTTP1,
		TLSFingerprintRouterTransportH2,
		TLSFingerprintRouterTransportWSHTTP1,
		TLSFingerprintRouterTransportWSH2:
	default:
		return &ValidationError{Field: prefix + ".transport", Message: "transport must be empty, http1, h2, websocket-http1, websocket-h2, or legacy http/websocket"}
	}
	switch strings.TrimSpace(r.MatchType) {
	case TLSFingerprintRouterMatchContains, TLSFingerprintRouterMatchPrefix, TLSFingerprintRouterMatchExact:
		return nil
	case TLSFingerprintRouterMatchRegex:
		pattern := r.Pattern
		if !r.CaseSensitive {
			pattern = "(?i)" + pattern
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return &ValidationError{Field: prefix + ".pattern", Message: "regex pattern is invalid: " + err.Error()}
		}
		return nil
	default:
		return &ValidationError{Field: prefix + ".match_type", Message: "match_type must be contains, prefix, exact, or regex"}
	}
}
