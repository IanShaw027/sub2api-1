package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/model"
)

type ResponseRewriteRule struct {
	StatusCode      *int
	Keywords        []string
	MatchMode       string
	ResponseMessage string
	Description     string
}

func (a *Account) GetResponseRewriteRules() []ResponseRewriteRule {
	if a == nil || a.Credentials == nil {
		return nil
	}

	raw, ok := a.Credentials["response_rewrite_rules"]
	if !ok || raw == nil {
		return nil
	}

	arr, ok := raw.([]any)
	if !ok {
		return nil
	}

	rules := make([]ResponseRewriteRule, 0, len(arr))
	for _, item := range arr {
		entry, ok := item.(map[string]any)
		if !ok || entry == nil {
			continue
		}

		var statusCode *int
		if parsed := parseTempUnschedInt(entry["status_code"]); parsed >= 100 && parsed <= 599 {
			code := parsed
			statusCode = &code
		}

		rule := ResponseRewriteRule{
			StatusCode:      statusCode,
			Keywords:        parseTempUnschedStrings(entry["keywords"]),
			MatchMode:       normalizeResponseRewriteMatchMode(parseTempUnschedString(entry["match_mode"])),
			ResponseMessage: parseTempUnschedString(entry["response_message"]),
			Description:     parseTempUnschedString(entry["description"]),
		}

		if rule.ResponseMessage == "" {
			continue
		}
		if rule.MatchMode == "" {
			continue
		}
		if rule.StatusCode == nil && len(rule.Keywords) == 0 {
			continue
		}

		rules = append(rules, rule)
	}

	return rules
}

func (a *Account) MatchResponseRewriteRule(upstreamStatusCode int, responseBody []byte) *ResponseRewriteRule {
	rules := a.GetResponseRewriteRules()
	if len(rules) == 0 {
		return nil
	}

	var bodyLower string
	var bodyLowerDone bool
	for i := range rules {
		if responseRewriteRuleMatches(rules[i], upstreamStatusCode, responseBody, &bodyLower, &bodyLowerDone) {
			return &rules[i]
		}
	}

	return nil
}

func normalizeResponseRewriteMatchMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case model.MatchModeAll:
		return model.MatchModeAll
	case model.MatchModeAny:
		return model.MatchModeAny
	default:
		return ""
	}
}

func responseRewriteRuleMatches(rule ResponseRewriteRule, upstreamStatusCode int, responseBody []byte, bodyLower *string, bodyLowerDone *bool) bool {
	hasStatus := rule.StatusCode != nil
	hasKeywords := len(rule.Keywords) > 0
	if !hasStatus && !hasKeywords {
		return false
	}

	statusMatch := hasStatus && *rule.StatusCode == upstreamStatusCode
	keywordMatch := hasKeywords && containsAnyResponseRewriteKeyword(ensureBodyLower(responseBody, bodyLower, bodyLowerDone), rule.Keywords)

	if rule.MatchMode == model.MatchModeAll {
		if hasStatus && !statusMatch {
			return false
		}
		if hasKeywords && !keywordMatch {
			return false
		}
		return true
	}

	if hasStatus && statusMatch {
		return true
	}
	if hasKeywords && keywordMatch {
		return true
	}
	return false
}

func containsAnyResponseRewriteKeyword(bodyLower string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(bodyLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}
