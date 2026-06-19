package apicompat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// AnthropicToCCChunkState tracks state for converting Anthropic Messages SSE
// events into Chat Completions streaming chunks.
type AnthropicToCCChunkState struct {
	ID         string
	Model      string
	Created    int64
	SentRole   bool
	Usage      *AnthropicUsage
	TextBuf    string
	ToolCalls  []ChatToolCall
	ToolIndex  int
	StopReason string
}

// NewAnthropicToCCChunkState returns an initialised stream state.
func NewAnthropicToCCChunkState(model string) *AnthropicToCCChunkState {
	return &AnthropicToCCChunkState{
		ID:      generateCCChunkID(),
		Model:   model,
		Created: time.Now().Unix(),
	}
}

// AnthropicEventToCCChunks converts one Anthropic SSE event to CC chunks.
func AnthropicEventToCCChunks(evt *AnthropicStreamEvent, state *AnthropicToCCChunkState) []ChatCompletionsChunk {
	if evt == nil {
		return nil
	}

	switch evt.Type {
	case "message_start":
		if evt.Message != nil && evt.Message.Model != "" {
			state.Model = evt.Message.Model
		}
		if evt.Message != nil {
			state.Usage = &evt.Message.Usage
		}
		chunk := state.newChunk()
		role := "assistant"
		chunk.Choices = []ChatChunkChoice{{
			Index: 0,
			Delta: ChatDelta{Role: role},
		}}
		state.SentRole = true
		return []ChatCompletionsChunk{chunk}

	case "content_block_start":
		if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
			tc := ChatToolCall{
				Index:    ccIntPtr(state.ToolIndex),
				ID:       evt.ContentBlock.ID,
				Type:     "function",
				Function: ChatFunctionCall{Name: evt.ContentBlock.Name},
			}
			chunk := state.newChunk()
			chunk.Choices = []ChatChunkChoice{{
				Index: 0,
				Delta: ChatDelta{ToolCalls: []ChatToolCall{tc}},
			}}
			state.ToolCalls = append(state.ToolCalls, tc)
			state.ToolIndex++
			return []ChatCompletionsChunk{chunk}
		}
		return nil

	case "content_block_delta":
		if evt.Delta == nil {
			return nil
		}
		switch evt.Delta.Type {
		case "text_delta":
			text := evt.Delta.Text
			chunk := state.newChunk()
			chunk.Choices = []ChatChunkChoice{{
				Index: 0,
				Delta: ChatDelta{Content: &text},
			}}
			state.TextBuf += text
			return []ChatCompletionsChunk{chunk}
		case "input_json_delta":
			idx := state.ToolIndex - 1
			if idx < 0 {
				idx = 0
			}
			chunk := state.newChunk()
			chunk.Choices = []ChatChunkChoice{{
				Index: 0,
				Delta: ChatDelta{ToolCalls: []ChatToolCall{{
					Index:    ccIntPtr(idx),
					Function: ChatFunctionCall{Arguments: evt.Delta.PartialJSON},
				}}},
			}}
			// Accumulate args for non-streaming BuildNonStreamingResponse
			if idx < len(state.ToolCalls) {
				state.ToolCalls[idx].Function.Arguments += evt.Delta.PartialJSON
			}
			return []ChatCompletionsChunk{chunk}
		case "thinking_delta":
			text := evt.Delta.Thinking
			chunk := state.newChunk()
			chunk.Choices = []ChatChunkChoice{{
				Index: 0,
				Delta: ChatDelta{ReasoningContent: &text},
			}}
			return []ChatCompletionsChunk{chunk}
		}
		return nil

	case "message_delta":
		if evt.Delta != nil && evt.Delta.StopReason != "" {
			state.StopReason = evt.Delta.StopReason
		}
		if evt.Usage != nil {
			state.Usage = evt.Usage
		}
		finishReason := anthropicStopReasonToCC(state.StopReason)
		chunk := state.newChunk()
		chunk.Choices = []ChatChunkChoice{{
			Index:        0,
			Delta:        ChatDelta{},
			FinishReason: &finishReason,
		}}
		return []ChatCompletionsChunk{chunk}

	case "message_stop":
		return nil
	}

	return nil
}

// BuildUsageChunk creates a final chunk with only usage data (for stream_options.include_usage).
func (s *AnthropicToCCChunkState) BuildUsageChunk() ChatCompletionsChunk {
	chunk := s.newChunk()
	chunk.Choices = nil
	if s.Usage != nil {
		chunk.Usage = &ChatUsage{
			PromptTokens:     s.Usage.InputTokens,
			CompletionTokens: s.Usage.OutputTokens,
			TotalTokens:      s.Usage.InputTokens + s.Usage.OutputTokens,
		}
		if s.Usage.CacheReadInputTokens > 0 {
			chunk.Usage.PromptTokensDetails = &ChatTokenDetails{
				CachedTokens: s.Usage.CacheReadInputTokens,
			}
		}
	}
	return chunk
}

// BuildNonStreamingResponse assembles a full CC response from accumulated state.
func (s *AnthropicToCCChunkState) BuildNonStreamingResponse(chunks []ChatCompletionsChunk) *ChatCompletionsResponse {
	resp := &ChatCompletionsResponse{
		ID:      s.ID,
		Object:  "chat.completion",
		Created: s.Created,
		Model:   s.Model,
	}

	msg := ChatMessage{Role: "assistant"}
	if s.TextBuf != "" {
		raw, _ := json.Marshal(s.TextBuf)
		msg.Content = raw
	}
	if len(s.ToolCalls) > 0 {
		msg.ToolCalls = s.ToolCalls
	}

	finishReason := anthropicStopReasonToCC(s.StopReason)
	resp.Choices = []ChatChoice{{
		Index:        0,
		Message:      msg,
		FinishReason: finishReason,
	}}

	if s.Usage != nil {
		resp.Usage = &ChatUsage{
			PromptTokens:     s.Usage.InputTokens,
			CompletionTokens: s.Usage.OutputTokens,
			TotalTokens:      s.Usage.InputTokens + s.Usage.OutputTokens,
		}
	}
	return resp
}

func (s *AnthropicToCCChunkState) newChunk() ChatCompletionsChunk {
	return ChatCompletionsChunk{
		ID:      s.ID,
		Object:  "chat.completion.chunk",
		Created: s.Created,
		Model:   s.Model,
	}
}

func anthropicStopReasonToCC(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop"
	default:
		return "stop"
	}
}

func ccIntPtr(i int) *int { return &i }

func generateCCChunkID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("chatcmpl-%s", hex.EncodeToString(b))
}
