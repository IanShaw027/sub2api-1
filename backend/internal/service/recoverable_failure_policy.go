package service

import "strings"

type recoverableFailureReason string

const (
	recoverableFailurePreviousResponseNotFound recoverableFailureReason = "previous_response_not_found"
	recoverableFailureInvalidAnchor            recoverableFailureReason = "invalid_anchor"
	recoverableFailureInvalidContinuation      recoverableFailureReason = "invalid_continuation"
	recoverableFailureToolContext              recoverableFailureReason = "tool_context"
	recoverableFailureToolContinuation         recoverableFailureReason = "tool_continuation"
)

type recoverableReplayPolicy struct {
	RetryWithFullReplay bool
}

func classifyRecoverableFailureReason(message string) (recoverableFailureReason, bool) {
	msg := strings.ToLower(strings.TrimSpace(message))
	if msg == "" {
		return "", false
	}

	switch {
	case strings.Contains(msg, "previous_response_not_found"),
		strings.Contains(msg, "previous response not found"):
		return recoverableFailurePreviousResponseNotFound, true
	case strings.Contains(msg, "anchor") && containsAnyRecoverableFailureToken(msg, "invalid", "missing", "stale", "not found"):
		return recoverableFailureInvalidAnchor, true
	case strings.Contains(msg, "continuation") && containsAnyRecoverableFailureToken(msg, "invalid", "missing", "stale", "not found"):
		return recoverableFailureInvalidContinuation, true
	case strings.Contains(msg, "tool context"):
		return recoverableFailureToolContext, true
	case strings.Contains(msg, "no tool call found") &&
		(strings.Contains(msg, "function call output") || strings.Contains(msg, "function_call_output")):
		return recoverableFailureToolContinuation, true
	case strings.Contains(msg, "tool_result") && strings.Contains(msg, "tool_use") &&
		containsAnyRecoverableFailureToken(msg, "missing", "without", "follow", "preced"):
		return recoverableFailureToolContinuation, true
	default:
		return "", false
	}
}

func replayPolicyForRecoverableFailureReason(reason recoverableFailureReason) recoverableReplayPolicy {
	switch reason {
	case recoverableFailurePreviousResponseNotFound,
		recoverableFailureInvalidAnchor,
		recoverableFailureInvalidContinuation,
		recoverableFailureToolContext,
		recoverableFailureToolContinuation:
		return recoverableReplayPolicy{RetryWithFullReplay: true}
	default:
		return recoverableReplayPolicy{}
	}
}

func containsAnyRecoverableFailureToken(msg string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}
