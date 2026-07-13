package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type anthropicWebSearchResult struct {
	Type  string `json:"type"`
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
}

type pendingAnthropicWebSearchResult struct {
	ToolUseID string
	ItemID    string
}

// ---------------------------------------------------------------------------
// Non-streaming: ResponsesResponse → AnthropicResponse
// ---------------------------------------------------------------------------

// ResponsesToAnthropic converts a Responses API response directly into an
// Anthropic Messages response. Reasoning output items are mapped to thinking
// blocks; function_call items become tool_use blocks.
func ResponsesToAnthropic(resp *ResponsesResponse, model string, nameMaps ...map[string]string) *AnthropicResponse {
	out := &AnthropicResponse{
		ID:                resp.ID,
		Type:              "message",
		Role:              "assistant",
		Container:         json.RawMessage("null"),
		ContextManagement: json.RawMessage("null"),
		Model:             model,
	}
	var toolNameMap map[string]string
	if len(nameMaps) > 0 {
		toolNameMap = nameMaps[0]
	}

	var blocks []AnthropicContentBlock
	annotationResults := responsesWebSearchResultsFromAnnotations(resp.Output)
	useAnnotationResults := responsesCountWebSearchCalls(resp.Output) == 1 && len(annotationResults) > 0

	for _, item := range resp.Output {
		switch item.Type {
		case "reasoning":
			summaryText := ""
			for _, s := range item.Summary {
				if s.Type == "summary_text" && s.Text != "" {
					summaryText += s.Text
				}
			}
			if summaryText != "" {
				blocks = append(blocks, AnthropicContentBlock{
					Type:     "thinking",
					Thinking: summaryText,
				})
			}
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" && part.Text != "" {
					blocks = append(blocks, AnthropicContentBlock{
						Type: "text",
						Text: part.Text,
					})
				}
				if part.Type == "refusal" && part.Refusal != "" {
					blocks = append(blocks, AnthropicContentBlock{
						Type: "text",
						Text: part.Refusal,
					})
				}
			}
		case "function_call":
			blocks = append(blocks, AnthropicContentBlock{
				Type:  "tool_use",
				ID:    fromResponsesCallID(item.CallID),
				Name:  MapClaudeToolName(item.Name, toolNameMap),
				Input: responsesFunctionCallInput(item.Arguments),
			})
		case "custom_tool_call":
			blocks = append(blocks, AnthropicContentBlock{
				Type:  "tool_use",
				ID:    fromResponsesCallID(responsesCallIDOrItemID(item)),
				Name:  MapClaudeToolName(item.Name, toolNameMap),
				Input: responsesFunctionCallInput(item.Input),
			})
		case "tool_search_call":
			blocks = append(blocks, AnthropicContentBlock{
				Type:  "tool_use",
				ID:    fromResponsesCallID(responsesCallIDOrItemID(item)),
				Name:  toolSearchProxyName,
				Input: toolSearchCallArgumentsJSON(item.Arguments),
			})
		case "web_search_call":
			toolUseID := "srvtoolu_" + item.ID
			query := ""
			if item.Action != nil {
				query = item.Action.Query
			}
			inputJSON, _ := json.Marshal(map[string]string{"query": query})
			blocks = append(blocks, AnthropicContentBlock{
				Type:  "server_tool_use",
				ID:    toolUseID,
				Name:  "web_search",
				Input: inputJSON,
			})
			results := responsesWebSearchResultsFromAction(item.Action)
			if len(results) == 0 && useAnnotationResults {
				results = annotationResults
			}
			resultsJSON, _ := json.Marshal(results)
			blocks = append(blocks, AnthropicContentBlock{
				Type:      "web_search_tool_result",
				ToolUseID: toolUseID,
				Content:   resultsJSON,
			})
		}
	}

	if len(blocks) == 0 {
		blocks = append(blocks, AnthropicContentBlock{Type: "text", Text: ""})
	}
	out.Content = blocks

	out.StopReason = responsesStatusToAnthropicStopReason(resp.Status, resp.IncompleteDetails, resp.Error, resp.Output, blocks)
	if out.StopReason == "refusal" {
		out.StopDetails = refusalStopDetails(responsesRefusalExplanation(resp.Output, resp.Error))
	}

	out.Usage = newAnthropicUsageEnvelope(0, 0, 0)
	if resp.Usage != nil {
		out.Usage = anthropicUsageFromResponsesUsage(resp.Usage)
	}

	return out
}

func responsesFunctionCallInput(arguments string) json.RawMessage {
	return anthropicToolUseInputFromArguments(arguments)
}

func responsesCallIDOrItemID(item ResponsesOutput) string {
	if strings.TrimSpace(item.CallID) != "" {
		return item.CallID
	}
	return item.ID
}

func anthropicToolUseInputFromArguments(arguments string) json.RawMessage {
	arguments = strings.TrimSpace(arguments)
	if arguments == "" {
		return json.RawMessage("{}")
	}
	if json.Valid([]byte(arguments)) {
		return json.RawMessage(arguments)
	}
	quoted, _ := json.Marshal(arguments)
	return quoted
}

func newAnthropicUsageEnvelope(inputTokens, outputTokens, cacheReadTokens int) AnthropicUsage {
	return AnthropicUsage{
		InputTokens:              inputTokens,
		OutputTokens:             outputTokens,
		CacheCreationInputTokens: 0,
		CacheReadInputTokens:     cacheReadTokens,
		CacheCreation: &AnthropicCacheCreation{
			Ephemeral5mInputTokens: 0,
			Ephemeral1hInputTokens: 0,
		},
		ServiceTier:  "standard",
		InferenceGeo: "",
		Iterations:   []string{},
		Speed:        "standard",
	}
}

func anthropicUsageFromResponsesUsage(u *ResponsesUsage) AnthropicUsage {
	if u == nil {
		return newAnthropicUsageEnvelope(0, 0, 0)
	}

	cacheReadTokens := 0
	if u.InputTokensDetails != nil && u.InputTokensDetails.CachedTokens > 0 {
		cacheReadTokens = u.InputTokensDetails.CachedTokens
	}

	inputTokens := u.InputTokens - cacheReadTokens - u.CacheCreationInputTokens
	if inputTokens < 0 {
		inputTokens = 0
	}

	usage := newAnthropicUsageEnvelope(inputTokens, u.OutputTokens, cacheReadTokens)
	usage.CacheCreationInputTokens = u.CacheCreationInputTokens
	return usage
}

func refusalStopDetails(explanation string) *AnthropicStopDetails {
	if strings.TrimSpace(explanation) == "" {
		explanation = "content blocked by upstream policy"
	}
	return &AnthropicStopDetails{
		Type:        "refusal",
		Explanation: &explanation,
	}
}

func responsesRefusalExplanation(output []ResponsesOutput, failedErr *ResponsesError) string {
	if failedErr != nil {
		if msg := strings.TrimSpace(failedErr.Message); msg != "" {
			return msg
		}
	}
	for _, item := range output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "refusal" && strings.TrimSpace(part.Refusal) != "" {
				return strings.TrimSpace(part.Refusal)
			}
		}
	}
	return ""
}

func responsesStatusToAnthropicStopReason(
	status string,
	details *ResponsesIncompleteDetails,
	failedErr *ResponsesError,
	output []ResponsesOutput,
	blocks []AnthropicContentBlock,
) string {
	switch status {
	case "incomplete":
		if details != nil {
			switch details.Reason {
			case "max_output_tokens":
				return "max_tokens"
			case "content_filter":
				return "refusal"
			case "model_context_window_exceeded":
				return "model_context_window_exceeded"
			}
		}
		return "end_turn"
	case "completed":
		if responsesOutputHasRefusal(output) {
			return "refusal"
		}
		if anthropicBlocksContainType(blocks, "tool_use") {
			return "tool_use"
		}
		return "end_turn"
	case "failed":
		if responsesOutputHasRefusal(output) || responsesErrorLooksLikeContentFilter(failedErr) {
			return "refusal"
		}
		return "end_turn"
	default:
		return "end_turn"
	}
}

func anthropicBlocksContainType(blocks []AnthropicContentBlock, want string) bool {
	for _, block := range blocks {
		if block.Type == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Streaming: ResponsesStreamEvent → []AnthropicStreamEvent (stateful converter)
// ---------------------------------------------------------------------------

// ResponsesEventToAnthropicState tracks state for converting a sequence of
// Responses SSE events directly into Anthropic SSE events.
type ResponsesEventToAnthropicState struct {
	MessageStartSent bool
	MessageStopSent  bool

	ContentBlockIndex         int
	ContentBlockOpen          bool
	CurrentBlockType          string // "text" | "thinking" | "tool_use"
	HasReceivedArgumentsDelta bool
	CurrentToolName           string
	CurrentToolArgs           string
	CurrentToolHadDelta       bool

	// SawToolUse is true once any client tool_use block was opened. Used by
	// FinalizeResponsesAnthropicStream when Response.Output is unavailable.
	SawToolUse bool

	// customToolInputBuf accumulates freeform custom_tool_call input deltas so
	// they can be JSON-encoded on done (matching anthropicToolUseInputFromArguments).
	customToolInputBuf string

	// OutputIndexToBlockIdx maps Responses output_index → Anthropic content block index.
	OutputIndexToBlockIdx map[int]int

	InputTokens              int
	OutputTokens             int
	CacheReadInputTokens     int
	CacheCreationInputTokens int

	ResponseID string
	Model      string
	Created    int64

	ToolNameMap map[string]string

	PendingWebSearchResults []pendingAnthropicWebSearchResult
}

// NewResponsesEventToAnthropicState returns an initialised stream state.
func NewResponsesEventToAnthropicState() *ResponsesEventToAnthropicState {
	return &ResponsesEventToAnthropicState{
		OutputIndexToBlockIdx: make(map[int]int),
		Created:               time.Now().Unix(),
	}
}

// ResponsesEventToAnthropicEvents converts a single Responses SSE event into
// zero or more Anthropic SSE events, updating state as it goes.
func ResponsesEventToAnthropicEvents(
	evt *ResponsesStreamEvent,
	state *ResponsesEventToAnthropicState,
) []AnthropicStreamEvent {
	switch evt.Type {
	case "response.created":
		return resToAnthHandleCreated(evt, state)
	case "response.output_item.added":
		return resToAnthHandleOutputItemAdded(evt, state)
	case "response.output_text.delta":
		return resToAnthHandleTextDelta(evt, state)
	case "response.output_text.done":
		return resToAnthHandleBlockDone(state)
	case "response.function_call_arguments.delta",
		// custom/freeform 工具的输入增量与 function_call 参数增量同形。
		"response.custom_tool_call_input.delta":
		return resToAnthHandleFuncArgsDelta(evt, state)
	case "response.function_call_arguments.done", "response.custom_tool_call_input.done":
		return resToAnthHandleFuncArgsDone(evt, state)
	case "response.output_item.done":
		return resToAnthHandleOutputItemDone(evt, state)
	case "response.reasoning_summary_text.delta",
		// 原始推理文本增量，与 reasoning summary 一样映射为 thinking。
		"response.reasoning_text.delta":
		return resToAnthHandleReasoningDelta(evt, state)
	case "response.reasoning_summary_text.done":
		return resToAnthHandleBlockDone(state)
	case "response.completed", "response.done", "response.incomplete", "response.failed", "response.cancelled", "response.canceled":
		return resToAnthHandleCompleted(evt, state)
	default:
		return nil
	}
}

// FinalizeResponsesAnthropicStream emits synthetic termination events if the
// stream ended without a proper completion event.
func FinalizeResponsesAnthropicStream(state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	if !state.MessageStartSent || state.MessageStopSent {
		return nil
	}

	var events []AnthropicStreamEvent
	events = append(events, closeCurrentBlock(state)...)
	stopReason := "end_turn"
	// Prefer SawToolUse: closeCurrentBlock does not clear CurrentBlockType, but a
	// later text block can overwrite it; SawToolUse survives mixed content.
	if state.SawToolUse || state.CurrentBlockType == "tool_use" {
		stopReason = "tool_use"
	}

	events = append(events,
		AnthropicStreamEvent{
			Type: "message_delta",
			Delta: &AnthropicDelta{
				StopReason: stopReason,
				Container:  json.RawMessage("null"),
			},
			ContextManagement: json.RawMessage("null"),
			Usage: func() *AnthropicUsage {
				usage := newAnthropicUsageEnvelope(state.InputTokens, state.OutputTokens, state.CacheReadInputTokens)
				usage.CacheCreationInputTokens = state.CacheCreationInputTokens
				return &usage
			}(),
		},
		AnthropicStreamEvent{Type: "message_stop"},
	)
	state.MessageStopSent = true
	return events
}

// ResponsesAnthropicEventToSSE formats an AnthropicStreamEvent as an SSE line pair.
func ResponsesAnthropicEventToSSE(evt AnthropicStreamEvent) (string, error) {
	data, err := json.Marshal(evt)
	if err != nil {
		return "", err
	}

	sse := fmt.Sprintf("event: %s\ndata: %s\n\n", evt.Type, data)
	if evt.Type == "message_start" {
		// Keep the post-start ping explicit for current Claude-compatible consumers.
		sse += "event: ping\ndata: {\"type\":\"ping\"}\n\n"
	}
	return sse, nil
}

// --- internal handlers ---

func resToAnthHandleCreated(evt *ResponsesStreamEvent, state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	if evt.Response != nil {
		state.ResponseID = evt.Response.ID
		// Only use upstream model if no override was set (e.g. originalModel)
		if state.Model == "" {
			state.Model = evt.Response.Model
		}
	}

	if state.MessageStartSent {
		return nil
	}
	state.MessageStartSent = true

	return []AnthropicStreamEvent{{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:                state.ResponseID,
			Type:              "message",
			Role:              "assistant",
			Container:         json.RawMessage("null"),
			ContextManagement: json.RawMessage("null"),
			Content:           []AnthropicContentBlock{},
			Model:             state.Model,
			Usage:             newAnthropicUsageEnvelope(0, 0, 0),
		},
	}}
}

func resToAnthHandleOutputItemAdded(evt *ResponsesStreamEvent, state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	if evt.Item == nil {
		return nil
	}

	switch evt.Item.Type {
	// function_call、custom_tool_call（freeform，如 apply_patch）与 tool_search_call
	// 同样映射为 Anthropic 的 tool_use 块。
	case "function_call", "custom_tool_call", "tool_search_call":
		var events []AnthropicStreamEvent
		events = append(events, closeCurrentBlock(state)...)

		idx := state.ContentBlockIndex
		state.OutputIndexToBlockIdx[evt.OutputIndex] = idx
		state.ContentBlockOpen = true
		state.CurrentBlockType = "tool_use"
		state.HasReceivedArgumentsDelta = false
		state.CurrentToolArgs = ""
		state.CurrentToolHadDelta = false
		state.customToolInputBuf = ""
		state.SawToolUse = true

		callID := evt.Item.CallID
		name := MapClaudeToolName(evt.Item.Name, state.ToolNameMap)
		switch evt.Item.Type {
		case "tool_search_call":
			callID = responsesCallIDOrItemID(*evt.Item)
			name = toolSearchProxyName
		case "custom_tool_call":
			callID = responsesCallIDOrItemID(*evt.Item)
		}
		state.CurrentToolName = name

		events = append(events, AnthropicStreamEvent{
			Type:  "content_block_start",
			Index: &idx,
			ContentBlock: &AnthropicContentBlock{
				Type:  "tool_use",
				ID:    fromResponsesCallID(callID),
				Name:  name,
				Input: json.RawMessage("{}"),
			},
		})
		return events

	case "reasoning":
		var events []AnthropicStreamEvent
		events = append(events, closeCurrentBlock(state)...)

		idx := state.ContentBlockIndex
		state.OutputIndexToBlockIdx[evt.OutputIndex] = idx
		state.ContentBlockOpen = true
		state.CurrentBlockType = "thinking"

		events = append(events, AnthropicStreamEvent{
			Type:  "content_block_start",
			Index: &idx,
			ContentBlock: &AnthropicContentBlock{
				Type:     "thinking",
				Thinking: "",
			},
		})
		return events

	case "message":
		return closeCurrentBlock(state)
	}

	return nil
}

func resToAnthHandleTextDelta(evt *ResponsesStreamEvent, state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	if evt.Delta == "" {
		return nil
	}

	var events []AnthropicStreamEvent

	if !state.ContentBlockOpen || state.CurrentBlockType != "text" {
		events = append(events, closeCurrentBlock(state)...)

		idx := state.ContentBlockIndex
		state.ContentBlockOpen = true
		state.CurrentBlockType = "text"

		events = append(events, AnthropicStreamEvent{
			Type:  "content_block_start",
			Index: &idx,
			ContentBlock: &AnthropicContentBlock{
				Type: "text",
				Text: "",
			},
		})
	}

	idx := state.ContentBlockIndex
	events = append(events, AnthropicStreamEvent{
		Type:  "content_block_delta",
		Index: &idx,
		Delta: &AnthropicDelta{
			Type: "text_delta",
			Text: evt.Delta,
		},
	})
	return events
}

func resToAnthHandleFuncArgsDelta(evt *ResponsesStreamEvent, state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	if evt.Delta == "" {
		return nil
	}

	// Freeform custom_tool_call input is plain text, not JSON fragments.
	// Buffer until done and encode with anthropicToolUseInputFromArguments so
	// PartialJSON is valid JSON (matches non-stream ResponsesToAnthropic).
	if evt.Type == "response.custom_tool_call_input.delta" {
		if _, ok := state.OutputIndexToBlockIdx[evt.OutputIndex]; !ok {
			return nil
		}
		state.customToolInputBuf += evt.Delta
		state.HasReceivedArgumentsDelta = true
		state.CurrentToolHadDelta = true
		return nil
	}

	blockIdx, ok := state.OutputIndexToBlockIdx[evt.OutputIndex]
	if !ok {
		return nil
	}
	state.HasReceivedArgumentsDelta = true
	state.CurrentToolHadDelta = true
	state.CurrentToolArgs += evt.Delta

	return []AnthropicStreamEvent{{
		Type:  "content_block_delta",
		Index: &blockIdx,
		Delta: &AnthropicDelta{
			Type:        "input_json_delta",
			PartialJSON: evt.Delta,
		},
	}}
}

func resToAnthHandleFuncArgsDone(evt *ResponsesStreamEvent, state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	var events []AnthropicStreamEvent
	isCustom := evt.Type == "response.custom_tool_call_input.done"
	arguments := evt.Arguments
	if isCustom {
		// Prefer authoritative full input from done; fall back to buffered deltas.
		if strings.TrimSpace(evt.Input) != "" {
			arguments = evt.Input
		} else {
			arguments = state.customToolInputBuf
		}
		state.customToolInputBuf = ""
	}

	// function_call: synthesize input_json_delta only when no prior deltas.
	// custom freeform: always emit once, JSON-encoded like non-stream.
	shouldEmit := false
	partial := arguments
	if isCustom {
		if strings.TrimSpace(arguments) != "" {
			partial = string(anthropicToolUseInputFromArguments(arguments))
			shouldEmit = true
		}
	} else if !state.HasReceivedArgumentsDelta && strings.TrimSpace(arguments) != "" {
		shouldEmit = true
	}

	if shouldEmit {
		blockIdx, ok := state.OutputIndexToBlockIdx[evt.OutputIndex]
		if !ok && state.ContentBlockOpen && state.CurrentBlockType == "tool_use" {
			blockIdx = state.ContentBlockIndex
			ok = true
		}
		if ok {
			events = append(events, AnthropicStreamEvent{
				Type:  "content_block_delta",
				Index: &blockIdx,
				Delta: &AnthropicDelta{
					Type:        "input_json_delta",
					PartialJSON: partial,
				},
			})
			state.HasReceivedArgumentsDelta = true
		}
	}
	if state.ContentBlockOpen && state.CurrentBlockType == "tool_use" {
		events = append(events, closeCurrentBlock(state)...)
	}
	return events
}

func resToAnthHandleReasoningDelta(evt *ResponsesStreamEvent, state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	if evt.Delta == "" {
		return nil
	}

	blockIdx, ok := state.OutputIndexToBlockIdx[evt.OutputIndex]
	if !ok {
		return nil
	}

	return []AnthropicStreamEvent{{
		Type:  "content_block_delta",
		Index: &blockIdx,
		Delta: &AnthropicDelta{
			Type:     "thinking_delta",
			Thinking: evt.Delta,
		},
	}}
}

func resToAnthHandleBlockDone(state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	if !state.ContentBlockOpen {
		return nil
	}
	return closeCurrentBlock(state)
}

func resToAnthHandleOutputItemDone(evt *ResponsesStreamEvent, state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	if evt.Item == nil {
		return nil
	}

	// Handle web_search_call → synthesize server_tool_use + web_search_tool_result blocks.
	if evt.Item.Type == "web_search_call" && evt.Item.Status == "completed" {
		return resToAnthHandleWebSearchDone(evt, state)
	}

	// tool_search_call typically carries full arguments only on output_item.done
	// (no function_call_arguments.delta/done). Synthesize input_json_delta then.
	if evt.Item.Type == "tool_search_call" && !state.HasReceivedArgumentsDelta {
		var events []AnthropicStreamEvent
		if strings.TrimSpace(evt.Item.Arguments) != "" {
			blockIdx, ok := state.OutputIndexToBlockIdx[evt.OutputIndex]
			if !ok && state.ContentBlockOpen && state.CurrentBlockType == "tool_use" {
				blockIdx = state.ContentBlockIndex
				ok = true
			}
			if ok {
				events = append(events, AnthropicStreamEvent{
					Type:  "content_block_delta",
					Index: &blockIdx,
					Delta: &AnthropicDelta{
						Type:        "input_json_delta",
						PartialJSON: string(toolSearchCallArgumentsJSON(evt.Item.Arguments)),
					},
				})
				state.HasReceivedArgumentsDelta = true
			}
		}
		if state.ContentBlockOpen && state.CurrentBlockType == "tool_use" {
			events = append(events, closeCurrentBlock(state)...)
		}
		return events
	}

	if state.ContentBlockOpen {
		return closeCurrentBlock(state)
	}
	return nil
}

// resToAnthHandleWebSearchDone converts an OpenAI web_search_call output item
// into Anthropic server_tool_use + web_search_tool_result content block pairs.
// This allows Claude Code to count the searches performed.
func resToAnthHandleWebSearchDone(evt *ResponsesStreamEvent, state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	var events []AnthropicStreamEvent
	events = append(events, closeCurrentBlock(state)...)

	toolUseID := "srvtoolu_" + evt.Item.ID
	query := ""
	if evt.Item.Action != nil {
		query = evt.Item.Action.Query
	}
	inputJSON, _ := json.Marshal(map[string]string{"query": query})

	// Emit server_tool_use block (start + stop).
	idx1 := state.ContentBlockIndex
	events = append(events, AnthropicStreamEvent{
		Type:  "content_block_start",
		Index: &idx1,
		ContentBlock: &AnthropicContentBlock{
			Type:  "server_tool_use",
			ID:    toolUseID,
			Name:  "web_search",
			Input: inputJSON,
		},
	})
	events = append(events, AnthropicStreamEvent{
		Type:  "content_block_stop",
		Index: &idx1,
	})
	state.ContentBlockIndex++

	results := responsesWebSearchResultsFromAction(evt.Item.Action)
	if len(results) == 0 {
		state.PendingWebSearchResults = append(state.PendingWebSearchResults, pendingAnthropicWebSearchResult{
			ToolUseID: toolUseID,
			ItemID:    evt.Item.ID,
		})
		return events
	}

	events = append(events, emitAnthropicWebSearchResultBlock(state, toolUseID, results)...)

	return events
}

func resToAnthHandleCompleted(evt *ResponsesStreamEvent, state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	if state.MessageStopSent {
		return nil
	}

	var events []AnthropicStreamEvent
	events = append(events, closeCurrentBlock(state)...)

	stopReason := "end_turn"
	var stopDetails *AnthropicStopDetails
	var refusalExplanation string
	if evt.Usage != nil {
		usage := anthropicUsageFromResponsesUsage(evt.Usage)
		state.InputTokens = usage.InputTokens
		state.OutputTokens = usage.OutputTokens
		state.CacheReadInputTokens = usage.CacheReadInputTokens
		state.CacheCreationInputTokens = usage.CacheCreationInputTokens
	}
	if evt.Response != nil {
		if evt.Response.Usage != nil {
			usage := anthropicUsageFromResponsesUsage(evt.Response.Usage)
			state.InputTokens = usage.InputTokens
			state.OutputTokens = usage.OutputTokens
			state.CacheReadInputTokens = usage.CacheReadInputTokens
			state.CacheCreationInputTokens = usage.CacheCreationInputTokens
		}
		switch evt.Response.Status {
		case "incomplete":
			if evt.Response.IncompleteDetails != nil {
				switch evt.Response.IncompleteDetails.Reason {
				case "max_output_tokens":
					stopReason = "max_tokens"
				case "content_filter":
					stopReason = "refusal"
					refusalExplanation = responsesRefusalExplanation(evt.Response.Output, evt.Response.Error)
					stopDetails = refusalStopDetails(refusalExplanation)
				case "model_context_window_exceeded":
					stopReason = "model_context_window_exceeded"
				}
			}
		case "completed":
			if responsesOutputHasRefusal(evt.Response.Output) {
				stopReason = "refusal"
				refusalExplanation = responsesRefusalExplanation(evt.Response.Output, evt.Response.Error)
				stopDetails = refusalStopDetails(refusalExplanation)
				break
			}
			// Prefer Response.Output (authoritative) over residual CurrentBlockType.
			// closeCurrentBlock does not clear CurrentBlockType, so residual state can
			// mis-classify stop_reason after mixed text/tool content. SawToolUse covers
			// streams where Output is empty but a tool_use block was opened.
			if responsesOutputHasToolCall(evt.Response.Output) || state.SawToolUse || state.CurrentBlockType == "tool_use" {
				stopReason = "tool_use"
			}
		case "failed":
			if responsesOutputHasRefusal(evt.Response.Output) || responsesErrorLooksLikeContentFilter(evt.Response.Error) {
				stopReason = "refusal"
				refusalExplanation = responsesRefusalExplanation(evt.Response.Output, evt.Response.Error)
				stopDetails = refusalStopDetails(refusalExplanation)
			}
		}
	}

	if len(state.PendingWebSearchResults) > 0 {
		actionResultsByToolUseID := map[string][]anthropicWebSearchResult{}
		if evt.Response != nil {
			actionResultsByToolUseID = responsesWebSearchResultsByToolUseIDFromOutput(evt.Response.Output)
		}
		annotationResults := []anthropicWebSearchResult{}
		if evt.Response != nil && responsesCountWebSearchCalls(evt.Response.Output) == 1 {
			annotationResults = responsesWebSearchResultsFromAnnotations(evt.Response.Output)
		}
		for i, pending := range state.PendingWebSearchResults {
			results := actionResultsByToolUseID[pending.ToolUseID]
			if len(results) == 0 && i == 0 && len(annotationResults) > 0 {
				results = annotationResults
			}
			events = append(events, emitAnthropicWebSearchResultBlock(state, pending.ToolUseID, results)...)
		}
		state.PendingWebSearchResults = nil
	}

	events = append(events,
		AnthropicStreamEvent{
			Type: "message_delta",
			Delta: &AnthropicDelta{
				StopReason:  stopReason,
				StopDetails: stopDetails,
				Container:   json.RawMessage("null"),
			},
			ContextManagement: json.RawMessage("null"),
			Usage: func() *AnthropicUsage {
				usage := newAnthropicUsageEnvelope(state.InputTokens, state.OutputTokens, state.CacheReadInputTokens)
				usage.CacheCreationInputTokens = state.CacheCreationInputTokens
				return &usage
			}(),
		},
		AnthropicStreamEvent{Type: "message_stop"},
	)
	state.MessageStopSent = true
	return events
}

func closeCurrentBlock(state *ResponsesEventToAnthropicState) []AnthropicStreamEvent {
	if !state.ContentBlockOpen {
		return nil
	}
	idx := state.ContentBlockIndex
	state.ContentBlockOpen = false
	state.ContentBlockIndex++
	return []AnthropicStreamEvent{{
		Type:  "content_block_stop",
		Index: &idx,
	}}
}

func responsesOutputHasRefusal(output []ResponsesOutput) bool {
	for _, item := range output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "refusal" && part.Refusal != "" {
				return true
			}
		}
	}
	return false
}

// responsesOutputHasToolCall reports whether Response.Output contains any
// client-executable tool call that should map to Anthropic stop_reason=tool_use.
func responsesOutputHasToolCall(output []ResponsesOutput) bool {
	for _, item := range output {
		switch item.Type {
		case "function_call", "custom_tool_call", "tool_search_call":
			return true
		}
	}
	return false
}

func emitAnthropicWebSearchResultBlock(
	state *ResponsesEventToAnthropicState,
	toolUseID string,
	results []anthropicWebSearchResult,
) []AnthropicStreamEvent {
	if results == nil {
		results = []anthropicWebSearchResult{}
	}
	resultsJSON, _ := json.Marshal(results)
	idx := state.ContentBlockIndex
	state.ContentBlockIndex++
	return []AnthropicStreamEvent{
		{
			Type:  "content_block_start",
			Index: &idx,
			ContentBlock: &AnthropicContentBlock{
				Type:      "web_search_tool_result",
				ToolUseID: toolUseID,
				Content:   resultsJSON,
			},
		},
		{
			Type:  "content_block_stop",
			Index: &idx,
		},
	}
}

func responsesCountWebSearchCalls(output []ResponsesOutput) int {
	count := 0
	for _, item := range output {
		if item.Type == "web_search_call" {
			count++
		}
	}
	return count
}

func responsesWebSearchResultsFromAction(action *WebSearchAction) []anthropicWebSearchResult {
	if action == nil {
		return []anthropicWebSearchResult{}
	}

	results := make([]anthropicWebSearchResult, 0, len(action.Sources))
	seen := make(map[string]struct{}, len(action.Sources))
	for _, source := range action.Sources {
		if source.URL == "" {
			continue
		}
		key := source.URL + "\n" + source.Title
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		results = append(results, anthropicWebSearchResult{
			Type:  "web_search_result",
			URL:   source.URL,
			Title: source.Title,
		})
	}
	if len(results) == 0 {
		return []anthropicWebSearchResult{}
	}
	return results
}

func responsesWebSearchResultsFromAnnotations(output []ResponsesOutput) []anthropicWebSearchResult {
	var results []anthropicWebSearchResult
	seen := map[string]struct{}{}

	for _, item := range output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			for _, ann := range part.Annotations {
				if ann.Type != "url_citation" || ann.URL == "" {
					continue
				}
				key := ann.URL + "\n" + ann.Title
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				results = append(results, anthropicWebSearchResult{
					Type:  "web_search_result",
					URL:   ann.URL,
					Title: ann.Title,
				})
			}
		}
	}

	if len(results) == 0 {
		return []anthropicWebSearchResult{}
	}
	return results
}

func responsesWebSearchResultsByToolUseIDFromOutput(output []ResponsesOutput) map[string][]anthropicWebSearchResult {
	resultsByID := make(map[string][]anthropicWebSearchResult)
	for _, item := range output {
		if item.Type != "web_search_call" || item.ID == "" {
			continue
		}
		results := responsesWebSearchResultsFromAction(item.Action)
		if len(results) == 0 {
			continue
		}
		resultsByID["srvtoolu_"+item.ID] = results
	}
	return resultsByID
}
