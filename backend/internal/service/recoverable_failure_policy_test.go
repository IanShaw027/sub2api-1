package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyRecoverableFailureReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
		reason  recoverableFailureReason
	}{
		{
			name:    "previous_response_not_found",
			message: "Previous response not found for continuation anchor",
			reason:  recoverableFailurePreviousResponseNotFound,
		},
		{
			name:    "invalid_anchor",
			message: "stale anchor not found for replay",
			reason:  recoverableFailureInvalidAnchor,
		},
		{
			name:    "invalid_continuation",
			message: "invalid continuation state",
			reason:  recoverableFailureInvalidContinuation,
		},
		{
			name:    "tool_context",
			message: "tool context is required for this request",
			reason:  recoverableFailureToolContext,
		},
		{
			name:    "tool_continuation_function_call_output",
			message: "No tool call found for function call output with call_id call_1.",
			reason:  recoverableFailureToolContinuation,
		},
		{
			name:    "tool_continuation_tool_use_pairing",
			message: "tool_result block missing matching tool_use block",
			reason:  recoverableFailureToolContinuation,
		},
		{
			name:    "empty_messages",
			message: "messages: at least one message is required",
			reason:  recoverableFailureEmptyMessages,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reason, ok := classifyRecoverableFailureReason(tt.message)
			require.True(t, ok)
			require.Equal(t, tt.reason, reason)
		})
	}
}

func TestReplayPolicyForRecoverableFailureReason(t *testing.T) {
	t.Parallel()

	recoverableReasons := []recoverableFailureReason{
		recoverableFailurePreviousResponseNotFound,
		recoverableFailureInvalidAnchor,
		recoverableFailureInvalidContinuation,
		recoverableFailureToolContext,
		recoverableFailureToolContinuation,
		recoverableFailureEmptyMessages,
	}

	for _, reason := range recoverableReasons {
		reason := reason
		t.Run(string(reason), func(t *testing.T) {
			t.Parallel()

			policy := replayPolicyForRecoverableFailureReason(reason)
			require.True(t, policy.RetryWithFullReplay)
		})
	}

	require.False(t, replayPolicyForRecoverableFailureReason(recoverableFailureReason("other")).RetryWithFullReplay)
}
