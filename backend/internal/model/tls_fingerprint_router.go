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
	MatchType               string `json:"match_type"`
	Pattern                 string `json:"pattern"`
	CaseSensitive           bool   `json:"case_sensitive"`
	TLSFingerprintProfileID int64  `json:"tls_fingerprint_profile_id"`
	UpstreamUserAgent       string `json:"upstream_user_agent,omitempty"`
	UpstreamOriginator      string `json:"upstream_originator,omitempty"`
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
	if r.TLSFingerprintProfileID <= 0 {
		return &ValidationError{Field: prefix + ".tls_fingerprint_profile_id", Message: "tls fingerprint profile is required"}
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
