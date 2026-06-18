package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareOpsRetryBody_PreservesPassbackThinkingForLoggedUpstreamModel(t *testing.T) {
	input := []byte(`{"model":"qwen3-235b-a22b-thinking-2507","thinking":{"type":"enabled","budget_tokens":1024},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"keep original","signature":""},{"type":"text","text":"Answer"}]}]}`)
	errorLog := &OpsErrorLogDetail{
		OpsErrorLog: OpsErrorLog{
			RequestPath:    "/v1/messages",
			RequestedModel: "claude-sonnet-4-5",
			UpstreamModel:  "qwen3-235b-a22b-thinking-2507",
		},
		RequestBody: string(input),
	}

	out := prepareOpsRetryBody(errorLog)

	require.Equal(t, string(input), string(out))
}

func TestPrepareOpsRetryBody_AppliesRetryFilterForAnthropicStrictModel(t *testing.T) {
	input := []byte(`{"model":"claude-sonnet-4-5","thinking":{"type":"enabled","budget_tokens":1024},"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"convert me","signature":""},{"type":"text","text":"Answer"}]}]}`)
	errorLog := &OpsErrorLogDetail{
		OpsErrorLog: OpsErrorLog{
			RequestPath:   "/v1/messages",
			Model:         "claude-sonnet-4-5",
			UpstreamModel: "claude-sonnet-4-5",
		},
		RequestBody: string(input),
	}

	out := prepareOpsRetryBody(errorLog)

	require.NotEqual(t, string(input), string(out))
	require.NotContains(t, string(out), `"thinking":{"type":"enabled"`)
	require.Contains(t, string(out), `"text":"convert me"`)
}
