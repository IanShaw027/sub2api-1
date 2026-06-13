//go:build unit

package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountGetResponseRewriteRules_PreservesOrderAndSkipsInvalidEntries(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"response_rewrite_rules": []any{
				map[string]any{
					"status_code":      float64(503),
					"keywords":         []any{"欠费", " insufficient_quota "},
					"match_mode":       "all",
					"response_message": "Service temporarily unavailable",
					"description":      "quota mask",
				},
				map[string]any{
					"status_code":      float64(0),
					"keywords":         []any{},
					"match_mode":       "all",
					"response_message": "",
				},
				map[string]any{
					"status_code":      float64(429),
					"keywords":         []any{"too many requests"},
					"match_mode":       "bogus",
					"response_message": "should be skipped",
				},
				map[string]any{
					"keywords":         []any{"rate limit"},
					"match_mode":       "any",
					"response_message": "Service temporarily unavailable",
				},
			},
		},
	}

	rules := account.GetResponseRewriteRules()
	require.Len(t, rules, 2)
	require.NotNil(t, rules[0].StatusCode)
	require.Equal(t, 503, *rules[0].StatusCode)
	require.Equal(t, []string{"欠费", "insufficient_quota"}, rules[0].Keywords)
	require.Equal(t, "all", rules[0].MatchMode)
	require.Equal(t, "quota mask", rules[0].Description)
	require.Nil(t, rules[1].StatusCode)
	require.Equal(t, []string{"rate limit"}, rules[1].Keywords)
	require.Equal(t, "any", rules[1].MatchMode)
}

func TestAccountMatchResponseRewriteRule_UsesRawUpstreamStatusAndBody(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"response_rewrite_rules": []any{
				map[string]any{
					"status_code":      float64(401),
					"keywords":         []any{"token_invalidated"},
					"match_mode":       "all",
					"response_message": "Service temporarily unavailable",
				},
			},
		},
	}

	body := []byte(`{"error":{"message":"Your authentication token has been invalidated.","code":"token_invalidated"}}`)
	rule := account.MatchResponseRewriteRule(401, body)
	require.NotNil(t, rule)
	require.Equal(t, "Service temporarily unavailable", rule.ResponseMessage)

	require.Nil(t, account.MatchResponseRewriteRule(502, body), "mapped client status must not be used for matching")
}

func TestAccountMatchResponseRewriteRule_AnyModeMatchesKeywordWithoutStatus(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"response_rewrite_rules": []any{
				map[string]any{
					"keywords":         []any{"too many requests"},
					"match_mode":       "any",
					"response_message": "Service temporarily unavailable",
				},
			},
		},
	}

	body := []byte(`{"error":{"message":"Too many requests, please wait before trying again."}}`)
	rule := account.MatchResponseRewriteRule(429, body)
	require.NotNil(t, rule)
}

func TestAccountGetResponseRewriteRules_RejectsNonIntegerStatusCodes(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"response_rewrite_rules": []any{
				map[string]any{
					"status_code":      float64(502.5),
					"match_mode":       "any",
					"response_message": "fractional status should be ignored",
				},
				map[string]any{
					"status_code":      math.NaN(),
					"match_mode":       "any",
					"response_message": "nan status should be ignored",
				},
				map[string]any{
					"status_code":      math.Inf(1),
					"match_mode":       "any",
					"response_message": "infinite status should be ignored",
				},
				map[string]any{
					"status_code":      float64(503),
					"match_mode":       "any",
					"response_message": "valid status",
				},
			},
		},
	}

	rules := account.GetResponseRewriteRules()

	require.Len(t, rules, 1)
	require.NotNil(t, rules[0].StatusCode)
	require.Equal(t, 503, *rules[0].StatusCode)
	require.Equal(t, "valid status", rules[0].ResponseMessage)
}
