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
// Non-streaming: ResponsesResponse → ChatCompletionsResponse
// ---------------------------------------------------------------------------

// ResponsesToChatCompletions converts a Responses API response into a Chat
// Completions response. Text output items are concatenated into
// choices[0].message.content; function_call items become tool_calls.
func ResponsesToChatCompletions(resp *ResponsesResponse, model string) *ChatCompletionsResponse {
	id := resp.ID
	if id == "" {
		id = generateChatCmplID()
	}

	out := &ChatCompletionsResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
	}

	var contentText string
	var reasoningText string
	var toolCalls []ChatToolCall

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" && part.Text != "" {
					contentText += part.Text
				}
			}
		case "function_call":
			toolCalls = append(toolCalls, ChatToolCall{
				ID:   responsesChatToolCallID(item),
				Type: "function",
				Function: ChatFunctionCall{
					Name:      item.Name,
					Arguments: item.Arguments,
				},
			})
		case "custom_tool_call":
			// Freeform/custom tools (e.g. Codex exec/apply_patch) surface as
			// function tool_calls with {"input":...} envelope for Chat Completions.
			toolCalls = append(toolCalls, ChatToolCall{
				ID:   responsesChatToolCallID(item),
				Type: "function",
				Function: ChatFunctionCall{
					Name:      item.Name,
					Arguments: responsesCustomToolCallChatArguments(item.Input),
				},
			})
		case "tool_search_call":
			toolCalls = append(toolCalls, ChatToolCall{
				ID:   responsesChatToolCallID(item),
				Type: "function",
				Function: ChatFunctionCall{
					Name:      toolSearchProxyName,
					Arguments: item.Arguments,
				},
			})
		case "reasoning":
			for _, s := range item.Summary {
				if s.Type == "summary_text" && s.Text != "" {
					reasoningText += s.Text
				}
			}
		case "web_search_call":
			// silently consumed — results already incorporated into text output
		}
	}

	msg := ChatMessage{Role: "assistant"}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	if contentText != "" {
		raw, _ := json.Marshal(contentText)
		msg.Content = raw
	}
	if reasoningText != "" {
		msg.ReasoningContent = reasoningText
	}

	finishReason := responsesStatusToChatFinishReason(resp.Status, resp.IncompleteDetails, resp.Error, toolCalls)

	out.Choices = []ChatChoice{{
		Index:        0,
		Message:      msg,
		FinishReason: finishReason,
	}}

	out.Usage = chatUsageFromResponsesUsage(resp.Usage)

	return out
}

func responsesChatToolCallID(item ResponsesOutput) string {
	if strings.TrimSpace(item.CallID) != "" {
		return item.CallID
	}
	return item.ID
}

func responsesCustomToolCallChatArguments(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return `{"input":""}`
	}
	// Already a JSON object (or other structured value) — keep as-is when valid object with "input".
	var obj map[string]any
	if err := json.Unmarshal([]byte(input), &obj); err == nil {
		if _, ok := obj["input"]; ok {
			return input
		}
	}
	encoded, err := json.Marshal(map[string]string{"input": input})
	if err != nil {
		return `{"input":""}`
	}
	return string(encoded)
}

func responsesStatusToChatFinishReason(status string, details *ResponsesIncompleteDetails, failedErr *ResponsesError, toolCalls []ChatToolCall) string {
	switch status {
	case "incomplete":
		if details != nil && details.Reason == "max_output_tokens" {
			return "length"
		}
		if details != nil && details.Reason == "content_filter" {
			return "content_filter"
		}
		return "stop"
	case "completed":
		if len(toolCalls) > 0 {
			return "tool_calls"
		}
		return "stop"
	case "failed":
		return responsesFailedToChatFinishReason(failedErr)
	default:
		return "stop"
	}
}

func responsesFailedToChatFinishReason(failedErr *ResponsesError) string {
	if responsesErrorLooksLikeContentFilter(failedErr) {
		return "content_filter"
	}
	// Chat Completions has no generic "error" finish_reason enum.
	return "stop"
}

func responsesErrorLooksLikeContentFilter(failedErr *ResponsesError) bool {
	if failedErr == nil {
		return false
	}
	combined := strings.ToLower(strings.TrimSpace(failedErr.Code + " " + failedErr.Message))
	if combined == "" {
		return false
	}
	contentFilterMarkers := []string{
		"content_filter",
		"content policy",
		"content_policy",
		"policy_violation",
		"safety",
		"not allowed",
		"high-risk cyber",
	}
	for _, marker := range contentFilterMarkers {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Streaming: ResponsesStreamEvent → []ChatCompletionsChunk (stateful converter)
// ---------------------------------------------------------------------------

// ResponsesEventToChatState tracks state for converting a sequence of Responses
// SSE events into Chat Completions SSE chunks.
type ResponsesEventToChatState struct {
	ID                     string
	Model                  string
	Created                int64
	SentRole               bool
	SawToolCall            bool
	SawText                bool
	Finalized              bool        // true after finish chunk has been emitted
	NextToolCallIndex      int         // next sequential tool_call index to assign
	OutputIndexToToolIndex map[int]int // Responses output_index → Chat tool_calls index
	ToolArgDeltaSeen       map[int]bool
	// CustomToolArgAccum holds freeform custom_tool_call input text accumulated
	// from *_input.delta events. Freeform args are not forwarded as raw deltas;
	// they are emitted once as a {"input":...} Chat envelope on done.
	CustomToolArgAccum map[int]string
	IncludeUsage       bool
	Usage              *ChatUsage
}

// NewResponsesEventToChatState returns an initialised stream state.
func NewResponsesEventToChatState() *ResponsesEventToChatState {
	return &ResponsesEventToChatState{
		ID:                     generateChatCmplID(),
		Created:                time.Now().Unix(),
		OutputIndexToToolIndex: make(map[int]int),
		ToolArgDeltaSeen:       make(map[int]bool),
		CustomToolArgAccum:     make(map[int]string),
	}
}

// ResponsesEventToChatChunks converts a single Responses SSE event into zero
// or more Chat Completions chunks, updating state as it goes.
func ResponsesEventToChatChunks(evt *ResponsesStreamEvent, state *ResponsesEventToChatState) []ChatCompletionsChunk {
	switch evt.Type {
	case "response.created":
		return resToChatHandleCreated(evt, state)
	case "response.output_text.delta":
		return resToChatHandleTextDelta(evt, state)
	case "response.output_item.added":
		return resToChatHandleOutputItemAdded(evt, state)
	case "response.function_call_arguments.delta",
		// custom/freeform 工具（如新版 apply_patch）的输入增量与 function_call 参数增量同形，
		// 均按 OutputIndex 累加到对应工具调用。
		"response.custom_tool_call_input.delta":
		return resToChatHandleFuncArgsDelta(evt, state)
	case "response.function_call_arguments.done",
		"response.custom_tool_call_input.done":
		return resToChatHandleFuncArgsDone(evt, state)
	case "response.reasoning_summary_text.delta",
		// 原始推理文本增量（真实 Codex 客户端消费的 reasoning_text.delta），
		// 与 reasoning summary 一样映射为 reasoning_content。
		"response.reasoning_text.delta":
		return resToChatHandleReasoningDelta(evt, state)
	case "response.reasoning_summary_text.done":
		return nil
	// response.done 是 Realtime/WS 与项目透传路径使用的终止别名；
	// 普通 Responses HTTP SSE 的公开终止事件仍以 response.completed 为主。
	case "response.completed", "response.done", "response.incomplete", "response.failed":
		return resToChatHandleCompleted(evt, state)
	default:
		return nil
	}
}

// FinalizeResponsesChatStream emits a final chunk with finish_reason if the
// stream ended without a proper completion event (e.g. upstream disconnect).
// It is idempotent: if a completion event already emitted the finish chunk,
// this returns nil.
func FinalizeResponsesChatStream(state *ResponsesEventToChatState) []ChatCompletionsChunk {
	if state.Finalized {
		return nil
	}
	state.Finalized = true

	finishReason := "stop"
	if state.SawToolCall {
		finishReason = "tool_calls"
	}

	chunks := []ChatCompletionsChunk{makeChatFinishChunk(state, finishReason)}

	if state.IncludeUsage && state.Usage != nil {
		chunks = append(chunks, ChatCompletionsChunk{
			ID:      state.ID,
			Object:  "chat.completion.chunk",
			Created: state.Created,
			Model:   state.Model,
			Choices: []ChatChunkChoice{},
			Usage:   state.Usage,
		})
	}

	return chunks
}

// ChatChunkToSSE formats a ChatCompletionsChunk as an SSE data line.
func ChatChunkToSSE(chunk ChatCompletionsChunk) (string, error) {
	data, err := json.Marshal(chunk)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("data: %s\n\n", data), nil
}

// --- internal handlers ---

func resToChatHandleCreated(evt *ResponsesStreamEvent, state *ResponsesEventToChatState) []ChatCompletionsChunk {
	if evt.Response != nil {
		if evt.Response.ID != "" {
			state.ID = evt.Response.ID
		}
		if state.Model == "" && evt.Response.Model != "" {
			state.Model = evt.Response.Model
		}
	}
	// Emit the role chunk.
	if state.SentRole {
		return nil
	}
	state.SentRole = true

	role := "assistant"
	return []ChatCompletionsChunk{makeChatDeltaChunk(state, ChatDelta{Role: role})}
}

func resToChatHandleTextDelta(evt *ResponsesStreamEvent, state *ResponsesEventToChatState) []ChatCompletionsChunk {
	if evt.Delta == "" {
		return nil
	}
	state.SawText = true
	content := evt.Delta
	return []ChatCompletionsChunk{makeChatDeltaChunk(state, ChatDelta{Content: &content})}
}

func resToChatHandleOutputItemAdded(evt *ResponsesStreamEvent, state *ResponsesEventToChatState) []ChatCompletionsChunk {
	// function_call、custom_tool_call（custom/freeform）与 tool_search_call 均按
	// 工具调用注册，以便后续 *_input.delta / *_arguments.delta 能映射到正确索引。
	if evt.Item == nil {
		return nil
	}
	switch evt.Item.Type {
	case "function_call", "custom_tool_call", "tool_search_call":
	default:
		return nil
	}

	state.SawToolCall = true
	idx := state.NextToolCallIndex
	state.OutputIndexToToolIndex[evt.OutputIndex] = idx
	state.NextToolCallIndex++

	name := evt.Item.Name
	if evt.Item.Type == "tool_search_call" {
		// Non-stream maps tool_search_call → function tool named toolSearchProxyName.
		name = toolSearchProxyName
	}

	return []ChatCompletionsChunk{makeChatDeltaChunk(state, ChatDelta{
		ToolCalls: []ChatToolCall{{
			Index: &idx,
			ID:    evt.Item.CallID,
			Type:  "function",
			Function: ChatFunctionCall{
				Name: name,
			},
		}},
	})}
}

func resToChatHandleFuncArgsDelta(evt *ResponsesStreamEvent, state *ResponsesEventToChatState) []ChatCompletionsChunk {
	if evt.Delta == "" {
		return nil
	}

	idx, ok := state.OutputIndexToToolIndex[evt.OutputIndex]
	if !ok {
		return nil
	}

	// Freeform custom tool inputs are raw text (e.g. apply_patch body). Emitting
	// them as Chat tool argument deltas would disagree with the non-stream path,
	// which always surfaces {"input":...}. Accumulate and wrap on done instead.
	if evt.Type == "response.custom_tool_call_input.delta" {
		state.CustomToolArgAccum[evt.OutputIndex] += evt.Delta
		return nil
	}

	state.ToolArgDeltaSeen[evt.OutputIndex] = true

	return []ChatCompletionsChunk{makeChatDeltaChunk(state, ChatDelta{
		ToolCalls: []ChatToolCall{{
			Index: &idx,
			Function: ChatFunctionCall{
				Arguments: evt.Delta,
			},
		}},
	})}
}

func resToChatHandleFuncArgsDone(evt *ResponsesStreamEvent, state *ResponsesEventToChatState) []ChatCompletionsChunk {
	idx, ok := state.OutputIndexToToolIndex[evt.OutputIndex]
	if !ok {
		return nil
	}

	// custom_tool_call: emit the Chat {"input":...} envelope once on done.
	// Prefer the full done payload; fall back to accumulated freeform deltas.
	if evt.Type == "response.custom_tool_call_input.done" {
		if state.ToolArgDeltaSeen[evt.OutputIndex] {
			return nil
		}
		args := responseEventDoneArguments(evt)
		if args == "" {
			args = state.CustomToolArgAccum[evt.OutputIndex]
		}
		state.ToolArgDeltaSeen[evt.OutputIndex] = true
		return []ChatCompletionsChunk{makeChatDeltaChunk(state, ChatDelta{
			ToolCalls: []ChatToolCall{{
				Index: &idx,
				Function: ChatFunctionCall{
					Arguments: responsesCustomToolCallChatArguments(args),
				},
			}},
		})}
	}

	if state.ToolArgDeltaSeen[evt.OutputIndex] {
		return nil
	}
	args := responseEventDoneArguments(evt)
	if args == "" {
		return nil
	}
	state.ToolArgDeltaSeen[evt.OutputIndex] = true

	return []ChatCompletionsChunk{makeChatDeltaChunk(state, ChatDelta{
		ToolCalls: []ChatToolCall{{
			Index: &idx,
			Function: ChatFunctionCall{
				Arguments: args,
			},
		}},
	})}
}

func responseEventDoneArguments(evt *ResponsesStreamEvent) string {
	if evt.Arguments != "" {
		return evt.Arguments
	}
	if evt.Input != "" {
		return evt.Input
	}
	if evt.Item != nil {
		return evt.Item.Arguments
	}
	return ""
}

func resToChatHandleReasoningDelta(evt *ResponsesStreamEvent, state *ResponsesEventToChatState) []ChatCompletionsChunk {
	if evt.Delta == "" {
		return nil
	}
	reasoning := evt.Delta
	return []ChatCompletionsChunk{makeChatDeltaChunk(state, ChatDelta{ReasoningContent: &reasoning})}
}

func resToChatHandleCompleted(evt *ResponsesStreamEvent, state *ResponsesEventToChatState) []ChatCompletionsChunk {
	state.Finalized = true
	finishReason := "stop"

	if evt.Usage != nil {
		state.Usage = chatUsageFromResponsesUsage(evt.Usage)
	}
	if evt.Response != nil {
		if evt.Response.Usage != nil {
			state.Usage = chatUsageFromResponsesUsage(evt.Response.Usage)
		}

		switch evt.Response.Status {
		case "incomplete":
			if evt.Response.IncompleteDetails != nil {
				switch evt.Response.IncompleteDetails.Reason {
				case "max_output_tokens":
					finishReason = "length"
				case "content_filter":
					finishReason = "content_filter"
				}
			}
		case "completed":
			if state.SawToolCall {
				finishReason = "tool_calls"
			}
		case "failed":
			finishReason = responsesFailedToChatFinishReason(evt.Response.Error)
		}
	} else if state.SawToolCall {
		finishReason = "tool_calls"
	}

	var chunks []ChatCompletionsChunk
	for _, refusal := range responseRefusalDeltas(evt.Response) {
		state.SawText = true
		refusalCopy := refusal
		chunks = append(chunks, makeChatDeltaChunk(state, ChatDelta{Refusal: &refusalCopy}))
	}
	chunks = append(chunks, makeChatFinishChunk(state, finishReason))

	if state.IncludeUsage && state.Usage != nil {
		chunks = append(chunks, ChatCompletionsChunk{
			ID:      state.ID,
			Object:  "chat.completion.chunk",
			Created: state.Created,
			Model:   state.Model,
			Choices: []ChatChunkChoice{},
			Usage:   state.Usage,
		})
	}

	return chunks
}

func responseRefusalDeltas(resp *ResponsesResponse) []string {
	if resp == nil {
		return nil
	}
	var refusals []string
	for _, item := range resp.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type != "refusal" {
				continue
			}
			refusal := strings.TrimSpace(part.Refusal)
			if refusal == "" {
				continue
			}
			refusals = append(refusals, refusal)
		}
	}
	return refusals
}

func chatUsageFromResponsesUsage(u *ResponsesUsage) *ChatUsage {
	if u == nil {
		return nil
	}
	usage := &ChatUsage{
		PromptTokens:     u.InputTokens,
		CompletionTokens: u.OutputTokens,
		TotalTokens:      u.InputTokens + u.OutputTokens,
	}
	usage.PromptTokensDetails = promptDetailsFromResponses(u.InputTokensDetails)
	if u.CacheCreationInputTokens > 0 {
		if usage.PromptTokensDetails == nil {
			usage.PromptTokensDetails = &ChatTokenDetails{}
		}
		if usage.PromptTokensDetails.CacheWriteTokens == 0 && usage.PromptTokensDetails.CacheCreationTokens == 0 {
			usage.PromptTokensDetails.CacheCreationTokens = u.CacheCreationInputTokens
		}
	}
	usage.CompletionTokensDetails = completionDetailsFromResponses(u.OutputTokensDetails)
	return usage
}

// promptDetailsFromResponses maps Responses-API input_tokens_details into a
// Chat-Completions prompt_tokens_details. Returns nil when nothing would be
// emitted, so upstreams that do not break down prompt usage stay clean.
func promptDetailsFromResponses(src *ResponsesInputTokensDetails) *ChatTokenDetails {
	if src == nil {
		return nil
	}
	if src.CachedTokens == 0 && src.AudioTokens == 0 && src.CacheCreationTokens == 0 && src.CacheWriteTokens == 0 {
		return nil
	}
	return &ChatTokenDetails{
		CachedTokens:        src.CachedTokens,
		AudioTokens:         src.AudioTokens,
		CacheCreationTokens: src.CacheCreationTokens,
		CacheWriteTokens:    src.CacheWriteTokens,
	}
}

// completionDetailsFromResponses maps Responses-API output_tokens_details
// into a Chat-Completions completion_tokens_details. Mirrors the OpenAI
// official CompletionUsage schema: reasoning_tokens, audio_tokens, and
// the predicted-outputs accepted/rejected counts. Returns nil when nothing
// would be emitted so non-reasoning, non-audio responses stay clean.
func completionDetailsFromResponses(src *ResponsesOutputTokensDetails) *ChatTokenDetails {
	if src == nil {
		return nil
	}
	if src.ReasoningTokens == 0 && src.AudioTokens == 0 &&
		src.AcceptedPredictionTokens == 0 && src.RejectedPredictionTokens == 0 {
		return nil
	}
	return &ChatTokenDetails{
		ReasoningTokens:          src.ReasoningTokens,
		AudioTokens:              src.AudioTokens,
		AcceptedPredictionTokens: src.AcceptedPredictionTokens,
		RejectedPredictionTokens: src.RejectedPredictionTokens,
	}
}

func makeChatDeltaChunk(state *ResponsesEventToChatState, delta ChatDelta) ChatCompletionsChunk {
	return ChatCompletionsChunk{
		ID:      state.ID,
		Object:  "chat.completion.chunk",
		Created: state.Created,
		Model:   state.Model,
		Choices: []ChatChunkChoice{{
			Index:        0,
			Delta:        delta,
			FinishReason: nil,
		}},
	}
}

func makeChatFinishChunk(state *ResponsesEventToChatState, finishReason string) ChatCompletionsChunk {
	empty := ""
	return ChatCompletionsChunk{
		ID:      state.ID,
		Object:  "chat.completion.chunk",
		Created: state.Created,
		Model:   state.Model,
		Choices: []ChatChunkChoice{{
			Index:        0,
			Delta:        ChatDelta{Content: &empty},
			FinishReason: &finishReason,
		}},
	}
}

// generateChatCmplID returns a "chatcmpl-" prefixed random hex ID.
func generateChatCmplID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "chatcmpl-" + hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// BufferedResponseAccumulator: accumulates SSE delta events for non-streaming
// paths where the terminal event may have empty output.
// ---------------------------------------------------------------------------

type bufferedFuncCall struct {
	CallID      string
	Name        string
	Args        strings.Builder
	HasArgDelta bool
}

// BufferedResponseAccumulator collects content from Responses SSE delta events
// so that non-streaming handlers can reconstruct output when the terminal event
// (response.completed / response.done) carries an empty output array.
type BufferedResponseAccumulator struct {
	text                 strings.Builder
	reasoning            strings.Builder
	funcCalls            []bufferedFuncCall
	outputIndexToFuncIdx map[int]int
}

// NewBufferedResponseAccumulator returns an initialised accumulator.
func NewBufferedResponseAccumulator() *BufferedResponseAccumulator {
	return &BufferedResponseAccumulator{
		outputIndexToFuncIdx: make(map[int]int),
	}
}

// ProcessEvent inspects a single Responses SSE event and accumulates any
// content it carries. Only delta events that contribute to the final output
// are handled; all other event types are silently ignored.
func (a *BufferedResponseAccumulator) ProcessEvent(event *ResponsesStreamEvent) {
	switch event.Type {
	case "response.output_text.delta":
		if event.Delta != "" {
			_, _ = a.text.WriteString(event.Delta)
		}
	case "response.output_item.added":
		if event.Item != nil && (event.Item.Type == "function_call" || event.Item.Type == "custom_tool_call") {
			idx := len(a.funcCalls)
			a.outputIndexToFuncIdx[event.OutputIndex] = idx
			a.funcCalls = append(a.funcCalls, bufferedFuncCall{
				CallID: event.Item.CallID,
				Name:   event.Item.Name,
			})
		}
	case "response.function_call_arguments.delta", "response.custom_tool_call_input.delta":
		if event.Delta != "" {
			if idx, ok := a.outputIndexToFuncIdx[event.OutputIndex]; ok {
				_, _ = a.funcCalls[idx].Args.WriteString(event.Delta)
				a.funcCalls[idx].HasArgDelta = true
			}
		}
	case "response.function_call_arguments.done", "response.custom_tool_call_input.done":
		args := responseEventDoneArguments(event)
		if args != "" {
			if idx, ok := a.outputIndexToFuncIdx[event.OutputIndex]; ok && !a.funcCalls[idx].HasArgDelta {
				_, _ = a.funcCalls[idx].Args.WriteString(args)
				a.funcCalls[idx].HasArgDelta = true
			}
		}
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		if event.Delta != "" {
			_, _ = a.reasoning.WriteString(event.Delta)
		}
	}
}

// HasContent reports whether any content has been accumulated.
func (a *BufferedResponseAccumulator) HasContent() bool {
	return a.text.Len() > 0 || len(a.funcCalls) > 0 || a.reasoning.Len() > 0
}

// BuildOutput constructs a []ResponsesOutput from the accumulated delta
// content. The order matches what ResponsesToChatCompletions expects:
// reasoning → message → function_calls.
func (a *BufferedResponseAccumulator) BuildOutput() []ResponsesOutput {
	var out []ResponsesOutput

	if a.reasoning.Len() > 0 {
		out = append(out, ResponsesOutput{
			Type: "reasoning",
			Summary: []ResponsesSummary{{
				Type: "summary_text",
				Text: a.reasoning.String(),
			}},
		})
	}

	if a.text.Len() > 0 {
		out = append(out, ResponsesOutput{
			Type: "message",
			Role: "assistant",
			Content: []ResponsesContentPart{{
				Type: "output_text",
				Text: a.text.String(),
			}},
		})
	}

	for i := range a.funcCalls {
		out = append(out, ResponsesOutput{
			Type:      "function_call",
			CallID:    a.funcCalls[i].CallID,
			Name:      a.funcCalls[i].Name,
			Arguments: a.funcCalls[i].Args.String(),
		})
	}

	return out
}

// SupplementResponseOutput fills resp.Output from accumulated delta content
// when the terminal event delivered an empty output array or when a terminal
// message block exists but its output_text content is empty.
func (a *BufferedResponseAccumulator) SupplementResponseOutput(resp *ResponsesResponse) {
	if resp == nil {
		return
	}
	if !a.HasContent() {
		return
	}
	if len(resp.Output) == 0 {
		resp.Output = a.BuildOutput()
		return
	}
	a.supplementFunctionCallArguments(resp)
	if a.text.Len() == 0 {
		return
	}
	for i := range resp.Output {
		if resp.Output[i].Type != "message" {
			continue
		}
		if len(resp.Output[i].Content) == 0 {
			resp.Output[i].Content = []ResponsesContentPart{{
				Type: "output_text",
				Text: a.text.String(),
			}}
			return
		}
		for j := range resp.Output[i].Content {
			if resp.Output[i].Content[j].Type == "output_text" && resp.Output[i].Content[j].Text == "" {
				resp.Output[i].Content[j].Text = a.text.String()
				return
			}
		}
	}
}

func (a *BufferedResponseAccumulator) supplementFunctionCallArguments(resp *ResponsesResponse) {
	if len(a.funcCalls) == 0 {
		return
	}
	byCallID := make(map[string]string, len(a.funcCalls))
	for i := range a.funcCalls {
		args := a.funcCalls[i].Args.String()
		if args == "" || a.funcCalls[i].CallID == "" {
			continue
		}
		byCallID[a.funcCalls[i].CallID] = args
	}
	if len(byCallID) == 0 {
		return
	}
	for i := range resp.Output {
		if resp.Output[i].Type != "function_call" || resp.Output[i].Arguments != "" {
			continue
		}
		if args, ok := byCallID[resp.Output[i].CallID]; ok {
			resp.Output[i].Arguments = args
		}
	}
}
