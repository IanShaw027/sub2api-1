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

// TLSFingerprintRouterRule 按 OS / 客户端 / 协议 / UA 选择上游 TLS profile。
// 规则内条件为 AND；规则之间 first-match-wins（OR）。
// 不填任何匹配条件且只填 profile_id 时，作为唯一指纹兜底。
type TLSFingerprintRouterRule struct {
	Name                    string `json:"name"`
	Enabled                 bool   `json:"enabled"`
	Transport               string `json:"transport,omitempty"`
	Protocol                string `json:"protocol,omitempty"`
	MatchType               string `json:"match_type,omitempty"`
	Pattern                 string `json:"pattern,omitempty"`
	CaseSensitive           bool   `json:"case_sensitive"`
	TLSFingerprintProfileID int64  `json:"tls_fingerprint_profile_id"`
	OS                      string `json:"os,omitempty"`
	ClientType              string `json:"client_type,omitempty"`
	UpstreamUserAgent       string `json:"upstream_user_agent,omitempty"`
	UpstreamOriginator      string `json:"upstream_originator,omitempty"`
}

// Validate 验证路由配置。
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

// Validate 验证单条规则。禁用规则允许暂存未完成配置。
func (r *TLSFingerprintRouterRule) Validate(index int) error {
	if r == nil || !r.Enabled {
		return nil
	}
	prefix := fmt.Sprintf("rules[%d]", index)
	if strings.TrimSpace(r.Name) == "" {
		return &ValidationError{Field: prefix + ".name", Message: "rule name is required"}
	}
	if r.TLSFingerprintProfileID <= 0 &&
		strings.TrimSpace(r.OS) == "" &&
		strings.TrimSpace(r.ClientType) == "" &&
		strings.TrimSpace(r.Protocol) == "" &&
		strings.TrimSpace(r.Pattern) == "" {
		return &ValidationError{
			Field:   prefix + ".tls_fingerprint_profile_id",
			Message: "rule must set a profile or at least one match condition",
		}
	}
	switch strings.ToLower(strings.TrimSpace(r.OS)) {
	case "", "windows", "macos", "linux", "ios", "android":
	default:
		return &ValidationError{Field: prefix + ".os", Message: "os must be empty, windows, macos, linux, ios, or android"}
	}
	switch strings.ToLower(strings.TrimSpace(r.Protocol)) {
	case "", "messages", "responses", "chat_completions", "images", "embeddings", "gemini", "antigravity", "kiro":
	default:
		return &ValidationError{Field: prefix + ".protocol", Message: "protocol must be empty, messages, responses, chat_completions, images, embeddings, gemini, antigravity, or kiro"}
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
		return &ValidationError{Field: prefix + ".transport", Message: "transport is invalid"}
	}
	if strings.TrimSpace(r.Pattern) == "" {
		return nil
	}
	switch strings.TrimSpace(r.MatchType) {
	case "", TLSFingerprintRouterMatchContains, TLSFingerprintRouterMatchPrefix, TLSFingerprintRouterMatchExact:
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
