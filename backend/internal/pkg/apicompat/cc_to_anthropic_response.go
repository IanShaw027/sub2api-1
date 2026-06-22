package apicompat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// ChatChunkToAnthropicState tracks state for converting a sequence of Chat
// Completions SSE chunks into Anthropic Messages SSE events.
type ChatChunkToAnthropicState struct {
	Model            string
	MessageID        string
	Created          int64
	ContentIndex     int
	ActiveToolCalls  map[int]*activeToolCallState
	SentMessageStart bool
	TextBlockOpen    bool
	Usage            *AnthropicUsage
}

type activeToolCallState struct {
	blockIndex int
	id         string
	name       string
	argsBuf    string
	sentStart  bool
}

// NewChatChunkToAnthropicState returns an initialised stream state.
func NewChatChunkToAnthropicState(model string) *ChatChunkToAnthropicState {
	return &ChatChunkToAnthropicState{
		Model:           model,
		MessageID:       generateAnthropicMsgID(),
		Created:         time.Now().Unix(),
		ActiveToolCalls: make(map[int]*activeToolCallState),
	}
}

// AnthropicSSELine represents one SSE event to emit.
type AnthropicSSELine struct {
	Event string // e.g. "message_start", "content_block_delta"
	Data  []byte // JSON-encoded payload
}

// ChatChunkToAnthropicEvents converts a single CC streaming chunk into zero or
// more Anthropic SSE event lines.
func ChatChunkToAnthropicEvents(chunk *ChatCompletionsChunk, state *ChatChunkToAnthropicState) []AnthropicSSELine {
	if chunk == nil {
		return nil
	}

	if chunk.Model != "" {
		state.Model = chunk.Model
	}

	var events []AnthropicSSELine

	if !state.SentMessageStart {
		events = append(events, state.buildMessageStart())
		state.SentMessageStart = true
	}

	if chunk.Usage != nil {
		state.Usage = CcUsageToAnthropic(chunk.Usage)
	}

	for _, choice := range chunk.Choices {
		events = append(events, state.processChoice(&choice)...)
	}

	return events
}

// FinalizeAnthropicStream emits message_delta and message_stop events.
func FinalizeAnthropicStream(state *ChatChunkToAnthropicState, stopReason string) []AnthropicSSELine {
	var events []AnthropicSSELine

	if state.TextBlockOpen {
		events = append(events, state.buildContentBlockStop())
		state.TextBlockOpen = false
	}

	for _, idx := range sortedActiveToolCallIndexes(state.ActiveToolCalls) {
		tc := state.ActiveToolCalls[idx]
		if tc != nil && tc.sentStart {
			events = append(events, buildBlockStop(tc.blockIndex))
		}
	}

	if stopReason == "" {
		stopReason = "end_turn"
	}

	usage := state.Usage
	if usage == nil {
		usage = &AnthropicUsage{}
	}

	delta := AnthropicDelta{
		Type:       "message_delta",
		StopReason: stopReason,
	}
	deltaJSON, _ := json.Marshal(struct {
		Type  string          `json:"type"`
		Delta AnthropicDelta  `json:"delta"`
		Usage *AnthropicUsage `json:"usage"`
	}{Type: "message_delta", Delta: delta, Usage: usage})
	events = append(events, AnthropicSSELine{Event: "message_delta", Data: deltaJSON})

	stopJSON, _ := json.Marshal(struct {
		Type string `json:"type"`
	}{Type: "message_stop"})
	events = append(events, AnthropicSSELine{Event: "message_stop", Data: stopJSON})

	return events
}

func (s *ChatChunkToAnthropicState) buildMessageStart() AnthropicSSELine {
	msg := struct {
		Type    string `json:"type"`
		Message struct {
			ID           string                  `json:"id"`
			Type         string                  `json:"type"`
			Role         string                  `json:"role"`
			Content      []AnthropicContentBlock `json:"content"`
			Model        string                  `json:"model"`
			StopReason   *string                 `json:"stop_reason"`
			StopSequence *string                 `json:"stop_sequence"`
			Usage        AnthropicUsage          `json:"usage"`
		} `json:"message"`
	}{Type: "message_start"}
	msg.Message.ID = s.MessageID
	msg.Message.Type = "message"
	msg.Message.Role = "assistant"
	msg.Message.Content = []AnthropicContentBlock{}
	msg.Message.Model = s.Model
	data, _ := json.Marshal(msg)
	return AnthropicSSELine{Event: "message_start", Data: data}
}

func (s *ChatChunkToAnthropicState) processChoice(choice *ChatChunkChoice) []AnthropicSSELine {
	var events []AnthropicSSELine

	// Handle reasoning content (reasoning_content or kimi's reasoning field).
	// Emit as text block since clients may reject unsigned thinking blocks.
	if reasoning := choice.Delta.EffectiveReasoningContent(); reasoning != "" {
		if !s.TextBlockOpen {
			events = append(events, s.buildContentBlockStart("text", ""))
			s.TextBlockOpen = true
		}
		events = append(events, s.buildTextDelta(reasoning))
	}

	if choice.Delta.Content != nil && *choice.Delta.Content != "" {
		if !s.TextBlockOpen {
			events = append(events, s.buildContentBlockStart("text", ""))
			s.TextBlockOpen = true
		}
		events = append(events, s.buildTextDelta(*choice.Delta.Content))
	}

	for _, tc := range choice.Delta.ToolCalls {
		idx := 0
		if tc.Index != nil {
			idx = *tc.Index
		}
		events = append(events, s.processToolCall(idx, &tc)...)
	}

	if choice.FinishReason != nil {
		if s.TextBlockOpen {
			events = append(events, s.buildContentBlockStop())
			s.TextBlockOpen = false
		}
		for _, idx := range sortedActiveToolCallIndexes(s.ActiveToolCalls) {
			tc := s.ActiveToolCalls[idx]
			if tc != nil && tc.sentStart {
				events = append(events, buildBlockStop(tc.blockIndex))
			}
		}
		stopReason := ccFinishReasonToAnthropic(*choice.FinishReason)
		events = append(events, s.buildMessageDelta(stopReason)...)
	}

	return events
}

func sortedActiveToolCallIndexes(calls map[int]*activeToolCallState) []int {
	indexes := make([]int, 0, len(calls))
	for idx := range calls {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	return indexes
}

func (s *ChatChunkToAnthropicState) processToolCall(idx int, tc *ChatToolCall) []AnthropicSSELine {
	var events []AnthropicSSELine

	active, exists := s.ActiveToolCalls[idx]
	if !exists {
		active = &activeToolCallState{
			blockIndex: s.ContentIndex,
			id:         tc.ID,
			name:       tc.Function.Name,
		}
		s.ActiveToolCalls[idx] = active
		s.ContentIndex++
	}
	if tc.ID != "" {
		active.id = tc.ID
	}
	if tc.Function.Name != "" {
		active.name = tc.Function.Name
	}

	if !active.sentStart {
		if s.TextBlockOpen {
			events = append(events, s.buildContentBlockStop())
			s.TextBlockOpen = false
		}
		blockStart, _ := json.Marshal(struct {
			Type         string `json:"type"`
			Index        int    `json:"index"`
			ContentBlock struct {
				Type  string          `json:"type"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content_block"`
		}{
			Type:  "content_block_start",
			Index: active.blockIndex,
			ContentBlock: struct {
				Type  string          `json:"type"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			}{Type: "tool_use", ID: active.id, Name: active.name, Input: json.RawMessage("{}")},
		})
		events = append(events, AnthropicSSELine{Event: "content_block_start", Data: blockStart})
		active.sentStart = true
	}

	if tc.Function.Arguments != "" {
		delta, _ := json.Marshal(struct {
			Type  string         `json:"type"`
			Index int            `json:"index"`
			Delta AnthropicDelta `json:"delta"`
		}{
			Type:  "content_block_delta",
			Index: active.blockIndex,
			Delta: AnthropicDelta{Type: "input_json_delta", PartialJSON: tc.Function.Arguments},
		})
		events = append(events, AnthropicSSELine{Event: "content_block_delta", Data: delta})
	}

	return events
}

func (s *ChatChunkToAnthropicState) buildContentBlockStart(blockType, text string) AnthropicSSELine {
	idx := s.ContentIndex
	s.ContentIndex++
	data, _ := json.Marshal(struct {
		Type         string `json:"type"`
		Index        int    `json:"index"`
		ContentBlock struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content_block"`
	}{
		Type:  "content_block_start",
		Index: idx,
		ContentBlock: struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{Type: blockType, Text: text},
	})
	return AnthropicSSELine{Event: "content_block_start", Data: data}
}

func (s *ChatChunkToAnthropicState) buildTextDelta(text string) AnthropicSSELine {
	idx := s.ContentIndex - 1
	data, _ := json.Marshal(struct {
		Type  string         `json:"type"`
		Index int            `json:"index"`
		Delta AnthropicDelta `json:"delta"`
	}{
		Type:  "content_block_delta",
		Index: idx,
		Delta: AnthropicDelta{Type: "text_delta", Text: text},
	})
	return AnthropicSSELine{Event: "content_block_delta", Data: data}
}

func (s *ChatChunkToAnthropicState) buildThinkingDelta(thinking string) AnthropicSSELine {
	idx := s.ContentIndex - 1
	data, _ := json.Marshal(struct {
		Type  string         `json:"type"`
		Index int            `json:"index"`
		Delta AnthropicDelta `json:"delta"`
	}{
		Type:  "content_block_delta",
		Index: idx,
		Delta: AnthropicDelta{Type: "thinking_delta", Thinking: thinking},
	})
	return AnthropicSSELine{Event: "content_block_delta", Data: data}
}

func (s *ChatChunkToAnthropicState) buildContentBlockStop() AnthropicSSELine {
	idx := s.ContentIndex - 1
	return buildBlockStop(idx)
}

func buildBlockStop(idx int) AnthropicSSELine {
	data, _ := json.Marshal(struct {
		Type  string `json:"type"`
		Index int    `json:"index"`
	}{Type: "content_block_stop", Index: idx})
	return AnthropicSSELine{Event: "content_block_stop", Data: data}
}

func (s *ChatChunkToAnthropicState) buildMessageDelta(stopReason string) []AnthropicSSELine {
	usage := s.Usage
	if usage == nil {
		usage = &AnthropicUsage{}
	}
	delta, _ := json.Marshal(struct {
		Type  string `json:"type"`
		Delta struct {
			StopReason   string  `json:"stop_reason"`
			StopSequence *string `json:"stop_sequence"`
		} `json:"delta"`
		Usage *AnthropicUsage `json:"usage"`
	}{
		Type: "message_delta",
		Delta: struct {
			StopReason   string  `json:"stop_reason"`
			StopSequence *string `json:"stop_sequence"`
		}{StopReason: stopReason},
		Usage: usage,
	})
	stop, _ := json.Marshal(struct {
		Type string `json:"type"`
	}{Type: "message_stop"})
	return []AnthropicSSELine{
		{Event: "message_delta", Data: delta},
		{Event: "message_stop", Data: stop},
	}
}

func ccFinishReasonToAnthropic(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "end_turn"
	default:
		return "end_turn"
	}
}

// CcFinishReasonToAnthropicStop converts a CC finish_reason to Anthropic stop_reason (exported).
func CcFinishReasonToAnthropicStop(reason string) string {
	return ccFinishReasonToAnthropic(reason)
}

// AccumulateToolCall accumulates tool call delta data into state for non-streaming assembly.
func AccumulateToolCall(state *ChatChunkToAnthropicState, tc *ChatToolCall) {
	idx := 0
	if tc.Index != nil {
		idx = *tc.Index
	}
	active, exists := state.ActiveToolCalls[idx]
	if !exists {
		active = &activeToolCallState{
			blockIndex: state.ContentIndex,
			id:         tc.ID,
			name:       tc.Function.Name,
		}
		state.ActiveToolCalls[idx] = active
		state.ContentIndex++
	}
	if tc.ID != "" {
		active.id = tc.ID
	}
	if tc.Function.Name != "" {
		active.name = tc.Function.Name
	}
	if tc.Function.Arguments != "" {
		active.argsBuf += tc.Function.Arguments
	}
}

// BuildToolUseBlocks assembles accumulated tool calls into Anthropic content blocks.
func BuildToolUseBlocks(state *ChatChunkToAnthropicState) []AnthropicContentBlock {
	if len(state.ActiveToolCalls) == 0 {
		return nil
	}
	blocks := make([]AnthropicContentBlock, 0, len(state.ActiveToolCalls))
	for i := 0; i < len(state.ActiveToolCalls); i++ {
		tc, ok := state.ActiveToolCalls[i]
		if !ok {
			continue
		}
		input := json.RawMessage(tc.argsBuf)
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		blocks = append(blocks, AnthropicContentBlock{
			Type:  "tool_use",
			ID:    tc.id,
			Name:  tc.name,
			Input: input,
		})
	}
	return blocks
}

func CcUsageToAnthropic(u *ChatUsage) *AnthropicUsage {
	if u == nil {
		return nil
	}
	cacheReadTokens := 0
	if u.PromptTokensDetails != nil {
		cacheReadTokens = u.PromptTokensDetails.CachedTokens
	}
	inputTokens := u.PromptTokens - cacheReadTokens
	if inputTokens < 0 {
		inputTokens = 0
	}
	au := &AnthropicUsage{
		InputTokens:          inputTokens,
		OutputTokens:         u.CompletionTokens,
		CacheReadInputTokens: cacheReadTokens,
	}
	return au
}

func generateAnthropicMsgID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("msg_%s", hex.EncodeToString(b))
}
