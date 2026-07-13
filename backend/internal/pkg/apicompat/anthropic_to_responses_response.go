package apicompat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Non-streaming: AnthropicResponse → ResponsesResponse
// ---------------------------------------------------------------------------

// AnthropicToResponsesResponse converts an Anthropic Messages response into a
// Responses API response. This is the reverse of ResponsesToAnthropic and
// enables Anthropic upstream responses to be returned in OpenAI Responses format.
func AnthropicToResponsesResponse(resp *AnthropicResponse) *ResponsesResponse {
	id := resp.ID
	if id == "" {
		id = generateResponsesID()
	}

	out := &ResponsesResponse{
		ID:     id,
		Object: "response",
		Model:  resp.Model,
	}

	var outputs []ResponsesOutput
	var msgParts []ResponsesContentPart
	serverToolUses := make(map[string]AnthropicContentBlock)
	emittedServerToolUses := make(map[string]bool)

	for _, block := range resp.Content {
		switch block.Type {
		case "thinking":
			if block.Thinking != "" {
				outputs = append(outputs, ResponsesOutput{
					Type: "reasoning",
					ID:   generateItemID(),
					Summary: []ResponsesSummary{{
						Type: "summary_text",
						Text: block.Thinking,
					}},
				})
			}
		case "text":
			if block.Text != "" {
				if resp.StopReason == "refusal" {
					msgParts = append(msgParts, ResponsesContentPart{
						Type:    "refusal",
						Refusal: block.Text,
					})
				} else {
					msgParts = append(msgParts, ResponsesContentPart{
						Type: "output_text",
						Text: block.Text,
					})
				}
			}
		case "tool_use":
			args := anthropicToolUseArguments(block.Input)
			outputs = append(outputs, ResponsesOutput{
				Type:      "function_call",
				ID:        generateItemID(),
				CallID:    toResponsesCallID(block.ID),
				Name:      block.Name,
				Arguments: args,
				Status:    "completed",
			})
		case "server_tool_use":
			if block.ID != "" {
				serverToolUses[block.ID] = block
			}
		case "web_search_tool_result":
			toolUse, ok := serverToolUses[block.ToolUseID]
			if !ok || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(toolUse.Name)), "web_search") {
				continue
			}
			emittedServerToolUses[block.ToolUseID] = true
			outputs = append(outputs, ResponsesOutput{
				Type: "web_search_call",
				ID:   responsesServerToolCallID(toolUse.ID),
				Action: &WebSearchAction{
					Type:    "search",
					Query:   anthropicServerToolQuery(toolUse.Input),
					Sources: anthropicWebSearchSources(block.Content),
				},
			})
		case "web_fetch_tool_result":
			toolUse, ok := serverToolUses[block.ToolUseID]
			if !ok || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(toolUse.Name)), "web_fetch") {
				continue
			}
			emittedServerToolUses[block.ToolUseID] = true
			outputs = append(outputs, ResponsesOutput{
				Type:      "function_call",
				ID:        generateItemID(),
				CallID:    toResponsesCallID(toolUse.ID),
				Name:      "webfetch",
				Arguments: mergeWebFetchArguments(toolUse.Input, block.Content),
				Status:    "completed",
			})
		}
	}
	for _, block := range resp.Content {
		if block.Type != "server_tool_use" || emittedServerToolUses[block.ID] {
			continue
		}
		switch {
		case strings.HasPrefix(strings.ToLower(strings.TrimSpace(block.Name)), "web_search"):
			outputs = append(outputs, ResponsesOutput{
				Type:   "web_search_call",
				ID:     responsesServerToolCallID(block.ID),
				Status: "completed",
				Action: &WebSearchAction{
					Type:  "search",
					Query: anthropicServerToolQuery(block.Input),
				},
			})
		case strings.HasPrefix(strings.ToLower(strings.TrimSpace(block.Name)), "web_fetch"):
			args := anthropicToolUseArguments(block.Input)
			outputs = append(outputs, ResponsesOutput{
				Type:      "function_call",
				ID:        generateItemID(),
				CallID:    toResponsesCallID(block.ID),
				Name:      "webfetch",
				Arguments: args,
				Status:    "completed",
			})
		}
	}

	// Assemble message output item from text parts
	if resp.StopReason == "refusal" && len(msgParts) == 0 && resp.StopDetails != nil && resp.StopDetails.Explanation != nil && *resp.StopDetails.Explanation != "" {
		msgParts = append(msgParts, ResponsesContentPart{
			Type:    "refusal",
			Refusal: *resp.StopDetails.Explanation,
		})
	}
	if len(msgParts) > 0 {
		outputs = append(outputs, ResponsesOutput{
			Type:    "message",
			ID:      generateItemID(),
			Role:    "assistant",
			Content: msgParts,
			Status:  "completed",
		})
	}

	if len(outputs) == 0 {
		outputs = append(outputs, ResponsesOutput{
			Type:    "message",
			ID:      generateItemID(),
			Role:    "assistant",
			Content: []ResponsesContentPart{{Type: "output_text", Text: ""}},
			Status:  "completed",
		})
	}
	out.Output = outputs

	// Map stop_reason → status
	out.Status = anthropicStopReasonToResponsesStatus(resp.StopReason, resp.Content)
	if out.Status == "incomplete" {
		out.IncompleteDetails = &ResponsesIncompleteDetails{Reason: anthropicStopReasonToResponsesIncompleteReason(resp.StopReason)}
	}

	// Usage
	// Anthropic's input_tokens excludes cache_read/cache_creation, while OpenAI
	// Responses' input_tokens is the total including cached tokens. Add them back
	// when converting so downstream consumers see OpenAI semantics.
	totalInputTokens := resp.Usage.InputTokens +
		resp.Usage.CacheReadInputTokens +
		resp.Usage.CacheCreationInputTokens
	out.Usage = &ResponsesUsage{
		InputTokens:              totalInputTokens,
		OutputTokens:             resp.Usage.OutputTokens,
		TotalTokens:              totalInputTokens + resp.Usage.OutputTokens,
		CacheCreationInputTokens: resp.Usage.CacheCreationInputTokens,
	}
	if resp.Usage.CacheReadInputTokens > 0 {
		out.Usage.InputTokensDetails = &ResponsesInputTokensDetails{
			CachedTokens: resp.Usage.CacheReadInputTokens,
		}
	}

	return out
}

func responsesServerToolCallID(id string) string {
	if after, ok := strings.CutPrefix(id, "srvtoolu_"); ok && strings.TrimSpace(after) != "" {
		return after
	}
	return id
}

func anthropicServerToolQuery(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var payload struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return payload.Query
}

func anthropicToolUseArguments(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err == nil {
		return encoded
	}
	return string(raw)
}

func anthropicWebSearchSources(raw json.RawMessage) []ResponsesWebSearchSource {
	if len(raw) == 0 {
		return nil
	}
	var results []struct {
		Type  string `json:"type"`
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil
	}
	sources := make([]ResponsesWebSearchSource, 0, len(results))
	for _, result := range results {
		sources = append(sources, ResponsesWebSearchSource{
			Type:  result.Type,
			URL:   result.URL,
			Title: result.Title,
		})
	}
	return sources
}

// anthropicStopReasonToResponsesStatus maps Anthropic stop_reason to Responses status.
func anthropicStopReasonToResponsesStatus(stopReason string, blocks []AnthropicContentBlock) string {
	switch stopReason {
	case "max_tokens", "model_context_window_exceeded":
		return "incomplete"
	case "end_turn", "tool_use", "stop_sequence":
		return "completed"
	case "refusal":
		return "completed"
	default:
		return "completed"
	}
}

func anthropicStopReasonToResponsesIncompleteReason(stopReason string) string {
	switch stopReason {
	case "model_context_window_exceeded":
		return "model_context_window_exceeded"
	default:
		return "max_output_tokens"
	}
}

// ---------------------------------------------------------------------------
// Streaming: AnthropicStreamEvent → []ResponsesStreamEvent (stateful converter)
// ---------------------------------------------------------------------------

// AnthropicEventToResponsesState tracks state for converting a sequence of
// Anthropic SSE events into Responses SSE events.
type AnthropicEventToResponsesState struct {
	ResponseID     string
	Model          string
	Created        int64
	SequenceNumber int

	// CreatedSent tracks whether response.created has been emitted.
	CreatedSent bool
	// CompletedSent tracks whether the terminal event has been emitted.
	CompletedSent bool

	// Current output tracking
	OutputIndex     int
	CurrentItemID   string
	CurrentItemType string // "message" | "function_call" | "reasoning"

	// For message output: accumulate text parts
	ContentIndex int

	// For function_call: track per-output info
	CurrentCallID         string
	CurrentName           string
	CurrentArgs           string
	CurrentArgsEmptyStart bool
	StopReason            string
	CurrentServerToolType string
	CurrentServerToolOpen bool
	CurrentWebSearch      *WebSearchAction
	OutputItems           []ResponsesOutput

	// Usage from message_start / message_delta. InputTokens here follows
	// Anthropic semantics (excludes cached tokens); they are added back when
	// emitting the OpenAI Responses usage.
	InputTokens              int
	OutputTokens             int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// NewAnthropicEventToResponsesState returns an initialised stream state.
func NewAnthropicEventToResponsesState() *AnthropicEventToResponsesState {
	return &AnthropicEventToResponsesState{
		Created: time.Now().Unix(),
	}
}

// AnthropicEventToResponsesEvents converts a single Anthropic SSE event into
// zero or more Responses SSE events, updating state as it goes.
func AnthropicEventToResponsesEvents(
	evt *AnthropicStreamEvent,
	state *AnthropicEventToResponsesState,
) []ResponsesStreamEvent {
	switch evt.Type {
	case "message_start":
		return anthToResHandleMessageStart(evt, state)
	case "content_block_start":
		return anthToResHandleContentBlockStart(evt, state)
	case "content_block_delta":
		return anthToResHandleContentBlockDelta(evt, state)
	case "content_block_stop":
		return anthToResHandleContentBlockStop(evt, state)
	case "message_delta":
		return anthToResHandleMessageDelta(evt, state)
	case "message_stop":
		return anthToResHandleMessageStop(state)
	default:
		return nil
	}
}

// FinalizeAnthropicResponsesStream emits synthetic termination events if the
// stream ended without a proper message_stop.
func FinalizeAnthropicResponsesStream(state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if !state.CreatedSent || state.CompletedSent {
		return nil
	}

	var events []ResponsesStreamEvent

	// Close any open item
	events = append(events, closeCurrentResponsesItem(state)...)

	// Emit response.completed
	events = append(events, makeResponsesCompletedEvent(state, "completed", nil))
	state.CompletedSent = true
	return events
}

// ResponsesEventToSSE formats a ResponsesStreamEvent as an SSE data line.
func ResponsesEventToSSE(evt ResponsesStreamEvent) (string, error) {
	data, err := json.Marshal(evt)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", evt.Type, data), nil
}

// --- internal handlers ---

func anthToResHandleMessageStart(evt *AnthropicStreamEvent, state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if evt.Message != nil {
		state.ResponseID = evt.Message.ID
		if state.Model == "" {
			state.Model = evt.Message.Model
		}
		if evt.Message.Usage.InputTokens > 0 {
			state.InputTokens = evt.Message.Usage.InputTokens
		}
		if evt.Message.Usage.CacheReadInputTokens > 0 {
			state.CacheReadInputTokens = evt.Message.Usage.CacheReadInputTokens
		}
		if evt.Message.Usage.CacheCreationInputTokens > 0 {
			state.CacheCreationInputTokens = evt.Message.Usage.CacheCreationInputTokens
		}
	}

	if state.CreatedSent {
		return nil
	}
	state.CreatedSent = true

	// Emit response.created
	return []ResponsesStreamEvent{makeResponsesCreatedEvent(state)}
}

func anthToResHandleContentBlockStart(evt *AnthropicStreamEvent, state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if evt.ContentBlock == nil {
		return nil
	}

	var events []ResponsesStreamEvent

	switch evt.ContentBlock.Type {
	case "thinking":
		state.CurrentItemID = generateItemID()
		state.CurrentItemType = "reasoning"
		state.ContentIndex = 0

		events = append(events, makeResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
			OutputIndex: state.OutputIndex,
			Item: &ResponsesOutput{
				Type: "reasoning",
				ID:   state.CurrentItemID,
			},
		}))

	case "text":
		// If we don't have an open message item, open one
		if state.CurrentItemType != "message" {
			state.CurrentItemID = generateItemID()
			state.CurrentItemType = "message"
			state.ContentIndex = 0

			events = append(events, makeResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
				OutputIndex: state.OutputIndex,
				Item: &ResponsesOutput{
					Type:   "message",
					ID:     state.CurrentItemID,
					Role:   "assistant",
					Status: "in_progress",
				},
			}))
		}

	case "tool_use":
		// Close previous item if any
		events = append(events, closeCurrentResponsesItem(state)...)

		state.CurrentItemID = generateItemID()
		state.CurrentItemType = "function_call"
		state.CurrentCallID = toResponsesCallID(evt.ContentBlock.ID)
		state.CurrentName = evt.ContentBlock.Name
		state.CurrentArgs = ""
		state.CurrentArgsEmptyStart = false
		if input := strings.TrimSpace(anthropicToolUseArguments(evt.ContentBlock.Input)); input != "" {
			state.CurrentArgs = input
			state.CurrentArgsEmptyStart = input == "{}"
		}

		events = append(events, makeResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
			OutputIndex: state.OutputIndex,
			Item: &ResponsesOutput{
				Type:   "function_call",
				ID:     state.CurrentItemID,
				CallID: state.CurrentCallID,
				Name:   state.CurrentName,
				Status: "in_progress",
			},
		}))
	case "server_tool_use":
		events = append(events, closeCurrentResponsesItem(state)...)

		state.CurrentArgs = ""
		state.CurrentArgsEmptyStart = false
		if input := strings.TrimSpace(anthropicToolUseArguments(evt.ContentBlock.Input)); input != "" {
			state.CurrentArgs = input
			state.CurrentArgsEmptyStart = input == "{}"
		}
		state.CurrentServerToolOpen = true
		state.CurrentWebSearch = nil

		switch {
		case strings.HasPrefix(strings.ToLower(strings.TrimSpace(evt.ContentBlock.Name)), "web_search"):
			state.CurrentItemID = responsesServerToolCallID(evt.ContentBlock.ID)
			state.CurrentItemType = "web_search_call"
			state.CurrentServerToolType = "web_search"
			state.CurrentCallID = ""
			state.CurrentName = ""
			events = append(events, makeResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
				OutputIndex: state.OutputIndex,
				Item: &ResponsesOutput{
					Type:   "web_search_call",
					ID:     state.CurrentItemID,
					Status: "in_progress",
				},
			}))
		case strings.HasPrefix(strings.ToLower(strings.TrimSpace(evt.ContentBlock.Name)), "web_fetch"):
			state.CurrentItemID = generateItemID()
			state.CurrentItemType = "function_call"
			state.CurrentServerToolType = "web_fetch"
			state.CurrentCallID = toResponsesCallID(evt.ContentBlock.ID)
			state.CurrentName = "webfetch"
			events = append(events, makeResponsesEvent(state, "response.output_item.added", &ResponsesStreamEvent{
				OutputIndex: state.OutputIndex,
				Item: &ResponsesOutput{
					Type:   "function_call",
					ID:     state.CurrentItemID,
					CallID: state.CurrentCallID,
					Name:   state.CurrentName,
					Status: "in_progress",
				},
			}))
		}
	case "web_search_tool_result":
		if state.CurrentServerToolType != "web_search" {
			return events
		}
		state.CurrentServerToolOpen = false
		state.CurrentWebSearch = &WebSearchAction{
			Type:    "search",
			Query:   anthropicServerToolQuery(json.RawMessage(state.CurrentArgs)),
			Sources: anthropicWebSearchSources(evt.ContentBlock.Content),
		}
	case "web_fetch_tool_result":
		if state.CurrentServerToolType != "web_fetch" {
			return events
		}
		state.CurrentServerToolOpen = false
		state.CurrentArgs = mergeWebFetchArguments(json.RawMessage(state.CurrentArgs), evt.ContentBlock.Content)
		state.CurrentArgsEmptyStart = false
	}

	return events
}

func anthToResHandleContentBlockDelta(evt *AnthropicStreamEvent, state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if evt.Delta == nil {
		return nil
	}

	switch evt.Delta.Type {
	case "text_delta":
		if evt.Delta.Text == "" {
			return nil
		}
		return []ResponsesStreamEvent{makeResponsesEvent(state, "response.output_text.delta", &ResponsesStreamEvent{
			OutputIndex:  state.OutputIndex,
			ContentIndex: state.ContentIndex,
			Delta:        evt.Delta.Text,
			ItemID:       state.CurrentItemID,
		})}

	case "thinking_delta":
		if evt.Delta.Thinking == "" {
			return nil
		}
		return []ResponsesStreamEvent{makeResponsesEvent(state, "response.reasoning_summary_text.delta", &ResponsesStreamEvent{
			OutputIndex:  state.OutputIndex,
			SummaryIndex: 0,
			Delta:        evt.Delta.Thinking,
			ItemID:       state.CurrentItemID,
		})}

	case "input_json_delta":
		if evt.Delta.PartialJSON == "" {
			return nil
		}
		if state.CurrentArgsEmptyStart && state.CurrentArgs == "{}" {
			state.CurrentArgs = ""
		}
		state.CurrentArgsEmptyStart = false
		state.CurrentArgs += evt.Delta.PartialJSON
		if state.CurrentServerToolType == "web_search" {
			return nil
		}
		return []ResponsesStreamEvent{makeResponsesEvent(state, "response.function_call_arguments.delta", &ResponsesStreamEvent{
			OutputIndex: state.OutputIndex,
			Delta:       evt.Delta.PartialJSON,
			ItemID:      state.CurrentItemID,
			CallID:      state.CurrentCallID,
			Name:        state.CurrentName,
		})}

	case "signature_delta":
		// Anthropic signature deltas have no Responses equivalent; skip
		return nil
	}

	return nil
}

func anthToResHandleContentBlockStop(evt *AnthropicStreamEvent, state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if state.CurrentServerToolType != "" && state.CurrentServerToolOpen {
		return nil
	}
	switch state.CurrentItemType {
	case "reasoning":
		// Emit reasoning summary done + output item done
		events := []ResponsesStreamEvent{
			makeResponsesEvent(state, "response.reasoning_summary_text.done", &ResponsesStreamEvent{
				OutputIndex:  state.OutputIndex,
				SummaryIndex: 0,
				ItemID:       state.CurrentItemID,
			}),
		}
		events = append(events, closeCurrentResponsesItem(state)...)
		return events

	case "function_call":
		// Emit function_call_arguments.done + output item done
		events := []ResponsesStreamEvent{
			makeResponsesEvent(state, "response.function_call_arguments.done", &ResponsesStreamEvent{
				OutputIndex: state.OutputIndex,
				ItemID:      state.CurrentItemID,
				CallID:      state.CurrentCallID,
				Name:        state.CurrentName,
				Arguments:   state.CurrentArgs,
			}),
		}
		events = append(events, closeCurrentResponsesItem(state)...)
		return events

	case "message":
		// Emit output_text.done (text block is done, but message item stays open for potential more blocks)
		return []ResponsesStreamEvent{
			makeResponsesEvent(state, "response.output_text.done", &ResponsesStreamEvent{
				OutputIndex:  state.OutputIndex,
				ContentIndex: state.ContentIndex,
				ItemID:       state.CurrentItemID,
			}),
		}
	case "web_search_call":
		return closeCurrentResponsesItem(state)
	}

	return nil
}

func anthToResHandleMessageDelta(evt *AnthropicStreamEvent, state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	// Update usage
	if evt.Usage != nil {
		state.OutputTokens = evt.Usage.OutputTokens
		if evt.Usage.InputTokens > 0 {
			state.InputTokens = evt.Usage.InputTokens
		}
		if evt.Usage.CacheReadInputTokens > 0 {
			state.CacheReadInputTokens = evt.Usage.CacheReadInputTokens
		}
		if evt.Usage.CacheCreationInputTokens > 0 {
			state.CacheCreationInputTokens = evt.Usage.CacheCreationInputTokens
		}
	}
	if evt.Delta != nil && strings.TrimSpace(evt.Delta.StopReason) != "" {
		state.StopReason = strings.TrimSpace(evt.Delta.StopReason)
	}

	return nil
}

func anthToResHandleMessageStop(state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if state.CompletedSent {
		return nil
	}

	var events []ResponsesStreamEvent

	// Close any open item
	events = append(events, closeCurrentResponsesItem(state)...)

	// Determine status
	status := anthropicStopReasonToResponsesStatus(state.StopReason, nil)
	var incompleteDetails *ResponsesIncompleteDetails
	if status == "incomplete" {
		incompleteDetails = &ResponsesIncompleteDetails{Reason: anthropicStopReasonToResponsesIncompleteReason(state.StopReason)}
	}

	// Emit response.completed
	events = append(events, makeResponsesCompletedEvent(state, status, incompleteDetails))
	state.CompletedSent = true
	return events
}

// --- helper functions ---

func closeCurrentResponsesItem(state *AnthropicEventToResponsesState) []ResponsesStreamEvent {
	if state.CurrentItemType == "" {
		return nil
	}

	itemType := state.CurrentItemType
	itemID := state.CurrentItemID

	// Reset
	item := ResponsesOutput{
		Type:   itemType,
		ID:     itemID,
		Status: "completed",
	}
	switch itemType {
	case "function_call":
		item.CallID = state.CurrentCallID
		item.Name = state.CurrentName
		item.Arguments = state.CurrentArgs
	case "web_search_call":
		item.Action = state.CurrentWebSearch
		if item.Action == nil {
			item.Action = &WebSearchAction{
				Type:  "search",
				Query: anthropicServerToolQuery(json.RawMessage(state.CurrentArgs)),
			}
		}
	case "message":
		item.Role = "assistant"
	}
	state.OutputItems = append(state.OutputItems, item)
	state.CurrentItemType = ""
	state.CurrentItemID = ""
	state.CurrentCallID = ""
	state.CurrentName = ""
	state.CurrentArgs = ""
	state.CurrentArgsEmptyStart = false
	state.CurrentServerToolType = ""
	state.CurrentServerToolOpen = false
	state.CurrentWebSearch = nil
	state.OutputIndex++
	state.ContentIndex = 0

	return []ResponsesStreamEvent{makeResponsesEvent(state, "response.output_item.done", &ResponsesStreamEvent{
		OutputIndex: state.OutputIndex - 1, // Use the index before increment
		Item:        &item,
	})}
}

func makeResponsesCreatedEvent(state *AnthropicEventToResponsesState) ResponsesStreamEvent {
	seq := state.SequenceNumber
	state.SequenceNumber++
	return ResponsesStreamEvent{
		Type:           "response.created",
		SequenceNumber: seq,
		Response: &ResponsesResponse{
			ID:     state.ResponseID,
			Object: "response",
			Model:  state.Model,
			Status: "in_progress",
			Output: []ResponsesOutput{},
		},
	}
}

func makeResponsesCompletedEvent(
	state *AnthropicEventToResponsesState,
	status string,
	incompleteDetails *ResponsesIncompleteDetails,
) ResponsesStreamEvent {
	seq := state.SequenceNumber
	state.SequenceNumber++

	// Anthropic's input_tokens excludes cache_read/cache_creation; add them
	// back to match OpenAI Responses semantics where input_tokens is the total.
	totalInputTokens := state.InputTokens + state.CacheReadInputTokens + state.CacheCreationInputTokens
	usage := &ResponsesUsage{
		InputTokens:              totalInputTokens,
		OutputTokens:             state.OutputTokens,
		TotalTokens:              totalInputTokens + state.OutputTokens,
		CacheCreationInputTokens: state.CacheCreationInputTokens,
	}
	if state.CacheReadInputTokens > 0 {
		usage.InputTokensDetails = &ResponsesInputTokensDetails{
			CachedTokens: state.CacheReadInputTokens,
		}
	}

	eventType := "response.completed"
	if status == "incomplete" {
		eventType = "response.incomplete"
	}
	return ResponsesStreamEvent{
		Type:           eventType,
		SequenceNumber: seq,
		Response: &ResponsesResponse{
			ID:                state.ResponseID,
			Object:            "response",
			Model:             state.Model,
			Status:            status,
			Output:            append([]ResponsesOutput(nil), state.OutputItems...),
			Usage:             usage,
			IncompleteDetails: incompleteDetails,
		},
	}
}

func makeResponsesEvent(state *AnthropicEventToResponsesState, eventType string, template *ResponsesStreamEvent) ResponsesStreamEvent {
	seq := state.SequenceNumber
	state.SequenceNumber++

	evt := *template
	evt.Type = eventType
	evt.SequenceNumber = seq
	return evt
}

func generateResponsesID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "resp_" + hex.EncodeToString(b)
}

func generateItemID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "item_" + hex.EncodeToString(b)
}

func mergeWebFetchArguments(input json.RawMessage, result json.RawMessage) string {
	payload := map[string]any{}
	if len(input) > 0 {
		_ = json.Unmarshal(input, &payload)
	}
	if len(result) > 0 {
		var resultPayload any
		if err := json.Unmarshal(result, &resultPayload); err == nil {
			payload["_sub2api_server_tool_result"] = resultPayload
		}
	}
	if len(payload) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}
