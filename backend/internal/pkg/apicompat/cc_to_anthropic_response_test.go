package apicompat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatChunkToAnthropicEvents_PreservesReasoningDeltas(t *testing.T) {
	state := NewChatChunkToAnthropicState("claude-test")
	reasoning := "Let me think..."
	chunk := &ChatCompletionsChunk{
		Choices: []ChatChunkChoice{
			{
				Delta: ChatDelta{
					ReasoningContent: &reasoning,
				},
			},
		},
	}

	events := ChatChunkToAnthropicEvents(chunk, state)
	require.Len(t, events, 3)
	require.Equal(t, "message_start", events[0].Event)
	require.Equal(t, "content_block_start", events[1].Event)
	require.Contains(t, string(events[1].Data), `"type":"thinking"`)
	require.Equal(t, "content_block_delta", events[2].Event)
	require.Contains(t, string(events[2].Data), `"type":"thinking_delta"`)
	require.Contains(t, string(events[2].Data), reasoning)
}

func TestChatChunkToAnthropicEvents_ClosesThinkingBeforeText(t *testing.T) {
	state := NewChatChunkToAnthropicState("claude-test")
	reasoning := "plan"
	first := &ChatCompletionsChunk{
		Choices: []ChatChunkChoice{
			{
				Delta: ChatDelta{
					ReasoningContent: &reasoning,
				},
			},
		},
	}
	text := "answer"
	second := &ChatCompletionsChunk{
		Choices: []ChatChunkChoice{
			{
				Delta: ChatDelta{
					Content: &text,
				},
			},
		},
	}

	events := ChatChunkToAnthropicEvents(first, state)
	require.Len(t, events, 3)

	events = ChatChunkToAnthropicEvents(second, state)
	require.Len(t, events, 3)
	require.Equal(t, "content_block_stop", events[0].Event)
	require.Equal(t, "content_block_start", events[1].Event)
	require.Contains(t, string(events[1].Data), `"type":"text"`)
	require.Equal(t, "content_block_delta", events[2].Event)
	require.Contains(t, string(events[2].Data), `"type":"text_delta"`)
	require.Contains(t, string(events[2].Data), text)
}
