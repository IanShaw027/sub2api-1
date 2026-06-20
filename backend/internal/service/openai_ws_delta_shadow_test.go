package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TEMP_DIAG(openai_ws_delta_shadow) remove_after_debug=true — tests for shadow helpers.

func mustItemHash(t *testing.T, raw string) [32]byte {
	t.Helper()
	h, ok := openAIWSCanonicalItemHash([]byte(raw))
	require.True(t, ok, "canonical hash failed for %s", raw)
	return h
}

type openAIWSNthWriteFailConn struct {
	openAIWSCaptureConn
	failOnWrite int
	writeErr    error
	writeCount  int
}

func (c *openAIWSNthWriteFailConn) WriteJSON(ctx context.Context, value any) error {
	c.writeCount++
	_ = c.openAIWSCaptureConn.WriteJSON(ctx, value)
	if c.failOnWrite > 0 && c.writeCount == c.failOnWrite {
		if c.writeErr != nil {
			return c.writeErr
		}
		return errors.New("test websocket write failure")
	}
	return nil
}

func TestOpenAIWSCanonicalItemHash_VolatileEnvelopeStrippedButContentPreserved(t *testing.T) {
	// message: top-level id/status are volatile envelope -> stripped -> same hash.
	base := `{"type":"message","role":"user","content":"hi"}`
	withEnvelope := `{"type":"message","id":"msg_123","status":"completed","role":"user","content":"hi"}`
	require.Equal(t, mustItemHash(t, base), mustItemHash(t, withEnvelope),
		"volatile id/status must not affect message hash")

	// item_reference: id is semantic -> never stripped -> different ids hash differently.
	ref1 := `{"type":"item_reference","id":"resp_1"}`
	ref2 := `{"type":"item_reference","id":"resp_2"}`
	require.NotEqual(t, mustItemHash(t, ref1), mustItemHash(t, ref2),
		"item_reference id is semantic and must change the hash (false-match guard)")

	// key order is canonicalized -> same hash regardless of field order.
	reordered := `{"content":"hi","role":"user","type":"message"}`
	require.Equal(t, mustItemHash(t, base), mustItemHash(t, reordered))
}

func TestOpenAIWSCanonicalItemHash_ReplayEquivalentEnvelopeFields(t *testing.T) {
	rawReasoning := `{"type":"reasoning","id":"rs_1","status":"completed","metadata":{"turn_id":"turn_1"},"content":[],"summary":[],"encrypted_content":"enc"}`
	replayedReasoning := `{"type":"reasoning","summary":[],"encrypted_content":"enc"}`
	require.Equal(t, mustItemHash(t, rawReasoning), mustItemHash(t, replayedReasoning))

	rawReasoningWithInternalTurnID := `{"type":"reasoning","id":"rs_2","status":"completed","metadata":{"turn_id":"turn_1"},"internal_chat_message_metadata_passthrough":{"turn_id":"turn_1"},"content":[],"summary":[],"encrypted_content":"enc"}`
	replayedReasoningWithoutInternalTurnID := `{"type":"reasoning","summary":[],"encrypted_content":"enc"}`
	require.Equal(t, mustItemHash(t, rawReasoningWithInternalTurnID), mustItemHash(t, replayedReasoningWithoutInternalTurnID))

	rawMessage := `{"type":"message","id":"msg_1","status":"completed","metadata":{"turn_id":"turn_1"},"role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[],"logprobs":[]}]}`
	replayedMessage := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	require.Equal(t, mustItemHash(t, rawMessage), mustItemHash(t, replayedMessage))

	rawOutputMessage := `{"type":"message","id":"msg_2","status":"completed","metadata":null,"content":[{"type":"output_text","text":"hello","annotations":[],"logprobs":[]}]}`
	replayedOutputMessage := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	require.Equal(t, mustItemHash(t, rawOutputMessage), mustItemHash(t, replayedOutputMessage))

	rawFunctionCall := `{"type":"function_call","id":"fc_1","status":"completed","metadata":{"turn_id":"turn_1"},"call_id":"call_1","name":"run","arguments":"{}"}`
	replayedFunctionCall := `{"type":"function_call","call_id":"call_1","name":"run","arguments":"{}"}`
	require.Equal(t, mustItemHash(t, rawFunctionCall), mustItemHash(t, replayedFunctionCall))

	rawCustomToolCall := `{"type":"custom_tool_call","id":"ctc_1","status":"completed","metadata":{"turn_id":"turn_1"},"call_id":"call_1","name":"apply_patch","input":"diff"}`
	replayedCustomToolCall := `{"type":"custom_tool_call","status":"completed","call_id":"call_1","name":"apply_patch","input":"diff"}`
	require.Equal(t, mustItemHash(t, rawCustomToolCall), mustItemHash(t, replayedCustomToolCall))

	rawOutputText := `{"type":"message","id":"msg_3","status":"completed","metadata":{"turn_id":"turn_1"},"content":[{"type":"output_text","text":"hello","annotations":[],"logprobs":[]}]}`
	replayedAssistantInputText := `{"type":"message","role":"assistant","content":[{"type":"input_text","text":"hello"}]}`
	require.Equal(t, mustItemHash(t, rawOutputText), mustItemHash(t, replayedAssistantInputText))

	rawPhasedOutputText := `{"type":"message","id":"msg_4","status":"completed","metadata":{"turn_id":"turn_1"},"phase":"final","content":[{"type":"output_text","text":"hello","annotations":[],"logprobs":[]}]}`
	replayedAssistantInputTextWithoutType := `{"role":"assistant","content":[{"type":"input_text","text":"hello"}]}`
	require.Equal(t, mustItemHash(t, rawPhasedOutputText), mustItemHash(t, replayedAssistantInputTextWithoutType))

	rawToolSearchCall := `{"type":"tool_search_call","id":"ts_1","status":"completed","metadata":{"turn_id":"turn_1"},"execution":"complete","arguments":{"limit":10,"query":"hello"}}`
	replayedToolSearchCall := `{"type":"tool_search_call","execution":"complete","arguments":{"query":"hello","limit":10}}`
	require.Equal(t, mustItemHash(t, rawToolSearchCall), mustItemHash(t, replayedToolSearchCall))
}

func TestOpenAIWSCanonicalItemHash_DoesNotDropSemanticEnvelopeLookalikes(t *testing.T) {
	messageWithAnnotation := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello","annotations":[{"type":"url_citation"}]}]}`
	messageWithoutAnnotation := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	require.NotEqual(t, mustItemHash(t, messageWithAnnotation), mustItemHash(t, messageWithoutAnnotation))

	messageWithSemanticMetadata := `{"type":"message","role":"assistant","metadata":{"turn_id":"turn_1","semantic":"keep"},"content":[{"type":"output_text","text":"hello"}]}`
	messageWithoutMetadata := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	require.NotEqual(t, mustItemHash(t, messageWithSemanticMetadata), mustItemHash(t, messageWithoutMetadata))
}

func TestOpenAIWSCanonicalItemHash_WhitespaceInContentBreaksHash(t *testing.T) {
	oneSpace := `{"type":"message","role":"user","content":"a b"}`
	twoSpace := `{"type":"message","role":"user","content":"a  b"}`
	require.NotEqual(t, mustItemHash(t, oneSpace), mustItemHash(t, twoSpace),
		"whitespace inside text content must change the hash")
}

func TestOpenAIWSNonInputHash_DenylistIgnoresVolatileFields(t *testing.T) {
	p1 := []byte(`{"model":"gpt","tools":[{"type":"x"}],"input":[{"a":1}],"previous_response_id":"resp_1","store":false,"client_metadata":{"x-client-request-id":"a"},"turn_metadata":"t1"}`)
	p2 := []byte(`{"model":"gpt","tools":[{"type":"x"}],"input":[{"b":2},{"c":3}],"previous_response_id":"resp_9","store":true,"client_metadata":{"x-client-request-id":"z"},"turn_metadata":"t2"}`)
	h1, ig1 := openAIWSNonInputHash(p1)
	h2, _ := openAIWSNonInputHash(p2)
	require.Equal(t, h1, h2, "denylist fields must not affect the non-input fingerprint")
	require.Subset(t, ig1, []string{"input", "previous_response_id", "store", "client_metadata", "turn_metadata"})

	// a semantic change (model) must change the fingerprint.
	p3 := []byte(`{"model":"gpt-other","tools":[{"type":"x"}],"input":[{"a":1}]}`)
	h3, _ := openAIWSNonInputHash(p3)
	require.NotEqual(t, h1, h3)

	// an unknown future field is included by default.
	p4 := []byte(`{"model":"gpt","tools":[{"type":"x"}],"future_param":true}`)
	h4, _ := openAIWSNonInputHash(p4)
	require.NotEqual(t, h1, h4, "unknown fields must be included so future params can't false-match")
}

func TestOpenAIWSExtractResponseOutputItems(t *testing.T) {
	items, ok := openAIWSExtractResponseOutputItems([]byte(`{"id":"resp_1","output":[{"type":"message","role":"assistant","content":"hi"}]}`))
	require.True(t, ok)
	require.Len(t, items, 1)

	_, ok = openAIWSExtractResponseOutputItems([]byte(`{"id":"resp_1"}`))
	require.False(t, ok, "missing output array must report not-captured")

	_, ok = openAIWSExtractResponseOutputItems([]byte(`{"id":"resp_1","output":[]}`))
	require.False(t, ok, "empty output array must report not-captured")
}

func TestOpenAIWSExtractOutputItemDoneItem(t *testing.T) {
	idx, item, ok := openAIWSExtractOutputItemDoneItem([]byte(`{"type":"response.output_item.done","output_index":2,"item":{"type":"message","role":"assistant","content":"hi"}}`))
	require.True(t, ok)
	require.Equal(t, 2, idx)
	require.JSONEq(t, `{"type":"message","role":"assistant","content":"hi"}`, string(item))

	_, _, ok = openAIWSExtractOutputItemDoneItem([]byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"message"}}`))
	require.False(t, ok)

	ordered := openAIWSOrderedOutputDoneItems(map[int]json.RawMessage{
		2: json.RawMessage(`{"type":"message","content":"two"}`),
		0: json.RawMessage(`{"type":"message","content":"zero"}`),
	})
	require.Len(t, ordered, 2)
	require.JSONEq(t, `{"type":"message","content":"zero"}`, string(ordered[0]))
	require.JSONEq(t, `{"type":"message","content":"two"}`, string(ordered[1]))
}

func TestOpenAIWSHashPrefixBreak(t *testing.T) {
	a := mustItemHash(t, `{"type":"message","content":"1"}`)
	b := mustItemHash(t, `{"type":"message","content":"2"}`)
	c := mustItemHash(t, `{"type":"message","content":"3"}`)

	matched, idx := openAIWSHashPrefixBreak([][32]byte{a, b, c}, [][32]byte{a, b})
	require.True(t, matched)
	require.Equal(t, -1, idx)

	matched, idx = openAIWSHashPrefixBreak([][32]byte{a, c, c}, [][32]byte{a, b})
	require.False(t, matched)
	require.Equal(t, 1, idx)

	// prefix longer than full -> break at len(full).
	matched, idx = openAIWSHashPrefixBreak([][32]byte{a}, [][32]byte{a, b})
	require.False(t, matched)
	require.Equal(t, 1, idx)
}

func buildShadowCandidateFixture(t *testing.T) (payload []byte, cached openAIWSSessionContextValue) {
	t.Helper()
	msg1 := `{"type":"message","role":"user","content":"hi"}`
	out1 := `{"type":"message","role":"assistant","content":"hello"}`
	newMsg := `{"type":"message","role":"user","content":"again"}`
	payload = []byte(`{"model":"gpt","input":[` + msg1 + `,` + out1 + `,` + newMsg + `],"store":false,"previous_response_id":"resp_1"}`)

	h1 := mustItemHash(t, msg1)
	h2 := mustItemHash(t, out1)
	nonInput, _ := openAIWSNonInputHash(payload)
	cached = openAIWSSessionContextValue{
		accountID:               5,
		connID:                  "conn_a",
		lastResponseID:          "resp_1",
		materializedHashes:      [][32]byte{h1, h2},
		materializedCount:       2,
		inputCount:              1,
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}
	return payload, cached
}

func TestEvaluateDeltaShadowCandidate_Happy(t *testing.T) {
	payload, cached := buildShadowCandidateFixture(t)
	log := evaluateOpenAIWSDeltaShadowCandidate(openAIWSDeltaShadowInput{
		RequestID:                "req1",
		AccountID:                5,
		LeaseConnID:              "conn_a",
		ConnMostRecentResponseID: "resp_1",
		CurrentPayload:           payload,
		HasFunctionCallOutput:    false,
		Cached:                   cached,
		CachedFound:              true,
	})
	require.True(t, log.Candidate)
	require.True(t, log.PrefixMatch)
	require.Equal(t, 1, log.DeltaItems, "only the trailing new item should be the delta")
	require.Equal(t, 3, log.FullItems)
	require.Less(t, log.DeltaBytes, log.FullBytes)
}

func TestEvaluateDeltaShadowCandidate_AllowsPerTurnGenerationParameterChanges(t *testing.T) {
	msg1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}`
	out1 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"two"}]}`
	newMsg := `{"type":"message","role":"user","content":[{"type":"input_text","text":"three"}]}`
	previousPayload := []byte(`{
		"type":"response.create",
		"model":"gpt-5.5",
		"stream":true,
		"prompt_cache_key":"session-a",
		"store":false,
		"instructions":"old instructions",
		"include":["reasoning.encrypted_content"],
		"parallel_tool_calls":true,
		"reasoning":{"effort":"medium"},
		"text":{"verbosity":"medium"},
		"tool_choice":"auto",
		"tools":[{"type":"function","name":"old_tool"}],
		"input":[` + msg1 + `,` + out1 + `]
	}`)
	currentPayload := []byte(`{
		"type":"response.create",
		"model":"gpt-5.5",
		"stream":true,
		"prompt_cache_key":"session-a",
		"store":false,
		"instructions":"new turn instructions",
		"include":["reasoning.encrypted_content","web_search_call.action.sources"],
		"parallel_tool_calls":false,
		"reasoning":{"effort":"high"},
		"text":{"verbosity":"low"},
		"tool_choice":{"type":"function","name":"new_tool"},
		"tools":[{"type":"function","name":"new_tool"}],
		"input":[` + msg1 + `,` + out1 + `,` + newMsg + `]
	}`)
	nonInput, _ := openAIWSNonInputHash(previousPayload)
	cached := openAIWSSessionContextValue{
		accountID: 5, connID: "conn_a", lastResponseID: "resp_1",
		materializedHashes:      [][32]byte{mustItemHash(t, msg1), mustItemHash(t, out1)},
		materializedCount:       2,
		inputCount:              1,
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}

	deltaPayload, log, applied, err := buildOpenAIWSActiveDeltaPayload(openAIWSDeltaShadowInput{
		RequestID: "r", AccountID: 5, LeaseConnID: "conn_a",
		ConnMostRecentResponseID: "resp_1", CurrentPayload: currentPayload,
		Cached: cached, CachedFound: true,
	})
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, log.Candidate)
	require.Equal(t, 1, log.DeltaItems)
	require.Equal(t, "resp_1", deltaPayload["previous_response_id"])
	inputItems, ok := deltaPayload["input"].([]any)
	require.True(t, ok)
	require.Len(t, inputItems, 1)
	require.Equal(t, "new turn instructions", deltaPayload["instructions"])
	require.Equal(t, false, deltaPayload["parallel_tool_calls"])
}

func TestEvaluateDeltaShadowCandidate_FallbackReasons(t *testing.T) {
	payload, cached := buildShadowCandidateFixture(t)
	base := openAIWSDeltaShadowInput{
		RequestID: "r", AccountID: 5, LeaseConnID: "conn_a",
		ConnMostRecentResponseID: "resp_1", CurrentPayload: payload,
		Cached: cached, CachedFound: true,
	}

	// no session context
	noCtx := base
	noCtx.CachedFound = false
	require.Equal(t, "no_session_context", evaluateOpenAIWSDeltaShadowCandidate(noCtx).FallbackReason)

	// conn mismatch
	connMis := base
	connMis.LeaseConnID = "conn_b"
	r := evaluateOpenAIWSDeltaShadowCandidate(connMis)
	require.False(t, r.Candidate)
	require.Equal(t, "conn_mismatch", r.FallbackReason)

	// not most recent
	notRecent := base
	notRecent.ConnMostRecentResponseID = "resp_other"
	require.Equal(t, "not_most_recent", evaluateOpenAIWSDeltaShadowCandidate(notRecent).FallbackReason)

	// raw vs client divergent
	divergent := base
	divCached := cached
	divCached.rawVsClientVisibleEqual = false
	divergent.Cached = divCached
	require.Equal(t, "raw_client_divergent", evaluateOpenAIWSDeltaShadowCandidate(divergent).FallbackReason)

	// tool continuation in the candidate delta is still blocked.
	toolInput := `{"type":"message","role":"user","content":"call tool"}`
	toolCall := `{"type":"function_call","call_id":"call_1","name":"shell","arguments":"{}"}`
	toolOutput := `{"type":"function_call_output","call_id":"call_1","output":"ok"}`
	toolPayload := []byte(`{"model":"gpt","input":[` + toolInput + `,` + toolCall + `,` + toolOutput + `]}`)
	toolNonInput, _ := openAIWSNonInputHash(toolPayload)
	tool := openAIWSDeltaShadowInput{
		RequestID:                "r",
		AccountID:                5,
		LeaseConnID:              "conn_a",
		ConnMostRecentResponseID: "resp_1",
		CurrentPayload:           toolPayload,
		CachedFound:              true,
		Cached: openAIWSSessionContextValue{
			accountID:               5,
			connID:                  "conn_a",
			lastResponseID:          "resp_1",
			materializedHashes:      [][32]byte{mustItemHash(t, toolInput)},
			materializedCount:       1,
			inputCount:              1,
			nonInputHash:            toolNonInput,
			rawVsClientVisibleEqual: true,
		},
	}
	require.Equal(t, "delta_tool_continuation_self_contained", evaluateOpenAIWSDeltaShadowCandidate(tool).FallbackReason)
}

func TestEvaluateDeltaShadowCandidate_OutputBoundaryBreak(t *testing.T) {
	// current input echoes the previous OUTPUT item with a changed content -> break at output boundary.
	msg1 := `{"type":"message","role":"user","content":"hi"}`
	out1 := `{"type":"message","role":"assistant","content":"hello"}`
	out1Changed := `{"type":"message","role":"assistant","content":"HELLO-changed"}`
	newMsg := `{"type":"message","role":"user","content":"again"}`
	payload := []byte(`{"model":"gpt","input":[` + msg1 + `,` + out1Changed + `,` + newMsg + `]}`)

	nonInput, _ := openAIWSNonInputHash(payload)
	cached := openAIWSSessionContextValue{
		accountID: 5, connID: "conn_a", lastResponseID: "resp_1",
		materializedHashes:      [][32]byte{mustItemHash(t, msg1), mustItemHash(t, out1)},
		materializedCount:       2,
		inputCount:              1, // index 0 is input; index 1 is output boundary
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}
	log := evaluateOpenAIWSDeltaShadowCandidate(openAIWSDeltaShadowInput{
		RequestID: "r", AccountID: 5, LeaseConnID: "conn_a",
		ConnMostRecentResponseID: "resp_1", CurrentPayload: payload,
		Cached: cached, CachedFound: true,
	})
	require.False(t, log.Candidate)
	require.False(t, log.PrefixMatch)
	require.Equal(t, "output", log.BreakBoundary)
	require.Equal(t, "prefix_break_output", log.FallbackReason)
}

func TestEvaluateDeltaShadowCandidate_OutputBoundaryBreakIncludesShapeDiagnostics(t *testing.T) {
	msg1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	rawReasoning := `{"type":"reasoning","id":"rs_1","status":"completed","summary":[{"type":"summary_text","text":"hidden raw reasoning"}]}`
	replayedReasoning := `{"type":"reasoning","content":[{"type":"output_text","text":"hidden raw reasoning"}]}`
	newMsg := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	payload := []byte(`{"model":"gpt","input":[` + msg1 + `,` + replayedReasoning + `,` + newMsg + `]}`)

	materializedItems := []json.RawMessage{
		json.RawMessage(msg1),
		json.RawMessage(rawReasoning),
	}
	materializedShapes, ok := openAIWSItemShapes(materializedItems)
	require.True(t, ok)
	nonInput, _ := openAIWSNonInputHash(payload)
	cached := openAIWSSessionContextValue{
		accountID: 5, connID: "conn_a", lastResponseID: "resp_1",
		materializedHashes:      [][32]byte{mustItemHash(t, msg1), mustItemHash(t, rawReasoning)},
		materializedShapes:      materializedShapes,
		materializedCount:       2,
		inputCount:              1,
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}

	log := evaluateOpenAIWSDeltaShadowCandidate(openAIWSDeltaShadowInput{
		RequestID: "r", AccountID: 5, LeaseConnID: "conn_a",
		ConnMostRecentResponseID: "resp_1", CurrentPayload: payload,
		Cached: cached, CachedFound: true,
	})
	require.False(t, log.Candidate)
	require.Equal(t, "output", log.BreakBoundary)
	require.Equal(t, "reasoning", log.BreakItemType)
	require.Equal(t, "reasoning", log.BreakCachedItemType)
	require.Contains(t, log.BreakCachedShape, "summary[]")
	require.Contains(t, log.BreakCurrentShape, "content[]")
	require.NotContains(t, log.BreakCachedShape, "hidden raw reasoning")
	require.NotContains(t, log.BreakCurrentShape, "hidden raw reasoning")
}

func TestEvaluateDeltaShadowCandidate_ReplayedAssistantOutputWithoutTypeMatches(t *testing.T) {
	msg1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	rawOutput := `{"type":"message","id":"msg_1","status":"completed","metadata":{"turn_id":"turn_1"},"phase":"final","content":[{"type":"output_text","text":"hello","annotations":[],"logprobs":[]}]}`
	replayedOutput := `{"role":"assistant","content":[{"type":"input_text","text":"hello"}]}`
	newMsg := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	payload := []byte(`{"model":"gpt","input":[` + msg1 + `,` + replayedOutput + `,` + newMsg + `],"store":false}`)

	nonInput, _ := openAIWSNonInputHash(payload)
	cached := openAIWSSessionContextValue{
		accountID: 5, connID: "conn_a", lastResponseID: "resp_1",
		materializedHashes:      [][32]byte{mustItemHash(t, msg1), mustItemHash(t, rawOutput)},
		materializedCount:       2,
		inputCount:              1,
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}

	deltaPayload, log, applied, err := buildOpenAIWSActiveDeltaPayload(openAIWSDeltaShadowInput{
		RequestID: "r", AccountID: 5, LeaseConnID: "conn_a",
		ConnMostRecentResponseID: "resp_1", CurrentPayload: payload,
		Cached: cached, CachedFound: true,
	})
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, log.Candidate)
	require.Equal(t, "resp_1", deltaPayload["previous_response_id"])
	require.False(t, deltaPayload["store"].(bool))
	inputItems, ok := deltaPayload["input"].([]any)
	require.True(t, ok)
	require.Len(t, inputItems, 1)
}

func TestOpenAIWSActiveDelta_InputOnlyContextDropsReplayedAssistantOutput(t *testing.T) {
	msg1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"input_text","text":"hello"}]}`
	newMsg := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	payload := []byte(`{"model":"gpt","input":[` + msg1 + `,` + replayedOutput + `,` + newMsg + `],"store":false}`)
	nonInput, _ := openAIWSNonInputHash(payload)
	cached := openAIWSSessionContextValue{
		accountID:               5,
		connID:                  "conn_a",
		lastResponseID:          "resp_1",
		materializedHashes:      [][32]byte{mustItemHash(t, msg1)},
		materializedCount:       1,
		inputCount:              1,
		inputOnlyContext:        true,
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}

	deltaPayload, log, applied, err := buildOpenAIWSActiveDeltaPayload(openAIWSDeltaShadowInput{
		RequestID:                "r_input_only_assistant",
		AccountID:                5,
		LeaseConnID:              "conn_a",
		ConnMostRecentResponseID: "resp_1",
		CurrentPayload:           payload,
		Cached:                   cached,
		CachedFound:              true,
	})
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, log.Candidate)
	require.Equal(t, 1, log.DeltaItems)
	deltaJSON := requestToJSONString(deltaPayload)
	require.Equal(t, "resp_1", gjson.Get(deltaJSON, "previous_response_id").String())
	require.Len(t, gjson.Get(deltaJSON, "input").Array(), 1)
	require.Equal(t, "again", gjson.Get(deltaJSON, "input.0.content.0.text").String())
}

func TestOpenAIWSActiveDelta_InputOnlyContextDropsReplayedToolCallOutput(t *testing.T) {
	msg1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"run"}]}`
	replayedCall := `{"type":"function_call","call_id":"call_1","name":"shell","arguments":"{}"}`
	toolOutput := `{"type":"function_call_output","call_id":"call_1","output":"ok"}`
	payload := []byte(`{"model":"gpt","input":[` + msg1 + `,` + replayedCall + `,` + toolOutput + `],"store":false}`)
	nonInput, _ := openAIWSNonInputHash(payload)
	cached := openAIWSSessionContextValue{
		accountID:               5,
		connID:                  "conn_a",
		lastResponseID:          "resp_1",
		materializedHashes:      [][32]byte{mustItemHash(t, msg1)},
		materializedCount:       1,
		inputCount:              1,
		inputOnlyContext:        true,
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}

	deltaPayload, log, applied, err := buildOpenAIWSActiveDeltaPayload(openAIWSDeltaShadowInput{
		RequestID:                "r_input_only_tool",
		AccountID:                5,
		LeaseConnID:              "conn_a",
		ConnMostRecentResponseID: "resp_1",
		CurrentPayload:           payload,
		HasFunctionCallOutput:    true,
		Cached:                   cached,
		CachedFound:              true,
	})
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, log.Candidate)
	require.Equal(t, 1, log.DeltaItems)
	deltaJSON := requestToJSONString(deltaPayload)
	require.Equal(t, "resp_1", gjson.Get(deltaJSON, "previous_response_id").String())
	require.Len(t, gjson.Get(deltaJSON, "input").Array(), 1)
	require.Equal(t, "function_call_output", gjson.Get(deltaJSON, "input.0.type").String())
	require.Equal(t, "call_1", gjson.Get(deltaJSON, "input.0.call_id").String())
}

func TestEvaluateDeltaShadowCandidate_ItemCountDecreaseBreaks(t *testing.T) {
	msg1 := `{"type":"message","role":"user","content":"hi"}`
	payload := []byte(`{"model":"gpt","input":[` + msg1 + `]}`) // shorter than materialized prefix
	nonInput, _ := openAIWSNonInputHash(payload)
	cached := openAIWSSessionContextValue{
		accountID: 5, connID: "conn_a", lastResponseID: "resp_1",
		materializedHashes:      [][32]byte{mustItemHash(t, msg1), mustItemHash(t, `{"type":"message","content":"x"}`)},
		materializedCount:       2,
		inputCount:              1,
		nonInputHash:            nonInput,
		rawVsClientVisibleEqual: true,
	}
	log := evaluateOpenAIWSDeltaShadowCandidate(openAIWSDeltaShadowInput{
		RequestID: "r", AccountID: 5, LeaseConnID: "conn_a",
		ConnMostRecentResponseID: "resp_1", CurrentPayload: payload,
		Cached: cached, CachedFound: true,
	})
	require.False(t, log.Candidate)
	require.False(t, log.PrefixMatch)
}

func TestOpenAIWSStateStore_SessionContextRoundTrip(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	val := openAIWSSessionContextValue{
		accountID:          5,
		connID:             "conn_a",
		lastResponseID:     "resp_1",
		materializedHashes: [][32]byte{{1}, {2}},
		materializedCount:  2,
		inputCount:         1,
		nonInputHash:       [32]byte{9},
	}
	store.BindSessionContext(7, 11, "sess", val, time.Minute)

	got, ok := store.GetSessionContext(7, 11, "sess")
	require.True(t, ok)
	require.Equal(t, val, got)

	// api-key isolation
	_, ok = store.GetSessionContext(7, 12, "sess")
	require.False(t, ok)

	store.DeleteSessionContext(7, 11, "sess")
	_, ok = store.GetSessionContext(7, 11, "sess")
	require.False(t, ok)
}

func TestOpenAIWSStateStore_ConnLastResponseAndInFlight(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)

	store.BindConnLastResponse("conn_a", "resp_1", time.Minute)
	got, ok := store.GetConnLastResponse("conn_a")
	require.True(t, ok)
	require.Equal(t, "resp_1", got)

	store.DeleteConnLastResponse("conn_a")
	_, ok = store.GetConnLastResponse("conn_a")
	require.False(t, ok)

	// in-flight is atomic: first TryBegin owns; second fails until End.
	require.True(t, store.TrySessionInFlight(7, 11, "sess"))
	require.False(t, store.TrySessionInFlight(7, 11, "sess"))
	store.EndSessionInFlight(7, 11, "sess")
	require.True(t, store.TrySessionInFlight(7, 11, "sess"))
	store.EndSessionInFlight(7, 11, "sess")
}

func TestOpenAIWSConnEvictHookInvalidatesConnLastResponse(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	store.BindConnLastResponse("conn_x", "resp_1", time.Minute)

	RegisterOpenAIWSConnEvictHook(func(connID string) { store.DeleteConnLastResponse(connID) })
	t.Cleanup(func() { RegisterOpenAIWSConnEvictHook(nil) })

	ap := &openAIWSAccountPool{conns: map[string]*openAIWSConn{"conn_x": {}}}
	deleteOpenAIWSAccountConnLocked(ap, "conn_x")

	_, present := ap.conns["conn_x"]
	require.False(t, present, "conn must be removed from pool")
	_, ok := store.GetConnLastResponse("conn_x")
	require.False(t, ok, "evict hook must invalidate conn_id -> last_response_id")
}

func TestOpenAIWSActiveDelta_ForwardWSV2SessionBoundSendsOnlyTrailingInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 3600
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	output1 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	output2 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}`

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.output_item.done","response_id":"resp_delta_forward_1","output_index":0,"item":` + output1 + `}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_delta_forward_1","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":2}}}`),
			[]byte(`{"type":"response.output_item.done","response_id":"resp_delta_forward_2","output_index":0,"item":` + output2 + `}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_delta_forward_2","model":"gpt-5.1","usage":{"input_tokens":5,"output_tokens":2}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          78001,
		Name:        "openai-delta-shadow-forward",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token-delta-forward"},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}

	groupID := int64(78010)
	apiKeyID := int64(78011)
	newContext := func() *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
		c.Request.Header.Set("originator", "codex_exec")
		c.Request.Header.Set("session_id", "sess-delta-shadow-forward")
		c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
		return c
	}

	firstCtx := newContext()
	sessionHash := svc.GenerateSessionHash(firstCtx, nil)
	require.NotEmpty(t, sessionHash)
	firstBody := []byte(`{"model":"gpt-5.1","stream":true,"input":[` + input1 + `]}`)
	firstResult, err := svc.Forward(context.Background(), firstCtx, account, firstBody)
	require.NoError(t, err)
	require.Equal(t, "resp_delta_forward_1", firstResult.RequestID)

	stateStore := svc.getOpenAIWSStateStore()
	cached, ok := stateStore.GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok, "session-bound forward WS path must write delta-shadow session context")
	require.Equal(t, account.ID, cached.accountID)
	require.Equal(t, "resp_delta_forward_1", cached.lastResponseID)
	require.Equal(t, 2, cached.materializedCount, "context must be input_N + raw output_N")
	require.Equal(t, 1, cached.inputCount)
	require.True(t, cached.rawVsClientVisibleEqual)

	connLast, ok := stateStore.GetConnLastResponse(cached.connID)
	require.True(t, ok)
	require.Equal(t, "resp_delta_forward_1", connLast)

	secondCtx := newContext()
	secondBody := []byte(`{"model":"gpt-5.1","stream":true,"input":[` + input1 + `,` + output1 + `,` + newInput + `]}`)
	secondResult, err := svc.Forward(context.Background(), secondCtx, account, secondBody)
	require.NoError(t, err)
	require.Equal(t, "resp_delta_forward_2", secondResult.RequestID)

	captureConn.mu.Lock()
	writes := append([]map[string]any(nil), captureConn.writes...)
	captureConn.mu.Unlock()
	require.Len(t, writes, 2)
	secondWrite := requestToJSONString(writes[1])
	require.Equal(t, "resp_delta_forward_1", gjson.Get(secondWrite, "previous_response_id").String())
	require.True(t, gjson.Get(secondWrite, "store").Exists())
	require.False(t, gjson.Get(secondWrite, "store").Bool(), "active delta keeps store=false/ZDR behavior")
	require.Len(t, gjson.Get(secondWrite, "input").Array(), 1, "active delta must send only the trailing new item")
	require.Equal(t, "again", gjson.Get(secondWrite, "input.0.content.0.text").String())

	cached, ok = stateStore.GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok)
	require.Equal(t, "resp_delta_forward_2", cached.lastResponseID)
	require.Equal(t, 4, cached.materializedCount, "next context must still be based on full replay input + raw output")
}

func TestOpenAIWSActiveDelta_ForwardWSV2BindsInputOnlyContextWithoutOutputCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 3600
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	replayedOutput := `{"type":"message","role":"assistant","content":[{"type":"input_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	output2 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}`

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.output_text.delta","delta":"hello"}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_input_only_1","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":1}}}`),
			[]byte(`{"type":"response.output_item.done","response_id":"resp_input_only_2","output_index":0,"item":` + output2 + `}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_input_only_2","model":"gpt-5.1","usage":{"input_tokens":4,"output_tokens":1}}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     &httpUpstreamRecorder{},
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          78007,
		Name:        "openai-delta-input-only-forward",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token-delta-input-only"},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}

	groupID := int64(78014)
	apiKeyID := int64(78015)
	newContext := func() *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
		c.Request.Header.Set("originator", "codex_exec")
		c.Request.Header.Set("session_id", "sess-delta-input-only-forward")
		c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
		return c
	}

	firstCtx := newContext()
	sessionHash := svc.GenerateSessionHash(firstCtx, nil)
	firstResult, err := svc.Forward(context.Background(), firstCtx, account, []byte(`{"model":"gpt-5.1","stream":true,"input":[`+input1+`]}`))
	require.NoError(t, err)
	require.Equal(t, "resp_input_only_1", firstResult.RequestID)

	cached, ok := svc.getOpenAIWSStateStore().GetSessionContext(groupID, apiKeyID, sessionHash)
	require.True(t, ok, "successful session-bound turn must bind input-only context even when upstream emits no output_item.done")
	require.Equal(t, "resp_input_only_1", cached.lastResponseID)
	require.Equal(t, 1, cached.materializedCount)
	require.Equal(t, 1, cached.inputCount)
	require.True(t, cached.inputOnlyContext)

	secondCtx := newContext()
	secondResult, err := svc.Forward(context.Background(), secondCtx, account, []byte(`{"model":"gpt-5.1","stream":true,"input":[`+input1+`,`+replayedOutput+`,`+newInput+`]}`))
	require.NoError(t, err)
	require.Equal(t, "resp_input_only_2", secondResult.RequestID)

	captureConn.mu.Lock()
	writes := append([]map[string]any(nil), captureConn.writes...)
	captureConn.mu.Unlock()
	require.Len(t, writes, 2)
	secondWrite := requestToJSONString(writes[1])
	require.Equal(t, "resp_input_only_1", gjson.Get(secondWrite, "previous_response_id").String())
	require.Len(t, gjson.Get(secondWrite, "input").Array(), 1)
	require.Equal(t, "again", gjson.Get(secondWrite, "input.0.content.0.text").String())
}

func TestOpenAIWSActiveDelta_ForwardWSV2WriteFailureRetryKeepsDeltaOnNewConn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.RetryBackoffInitialMS = 1
	cfg.Gateway.OpenAIWS.RetryBackoffMaxMS = 1
	cfg.Gateway.OpenAIWS.RetryJitterRatio = 0
	cfg.Gateway.OpenAIWS.RetryTotalBudgetMS = 1000
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 3600
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	output1 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`
	output2 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}`

	reusedConn := &openAIWSNthWriteFailConn{
		failOnWrite: 2,
		writeErr:    errors.New("fail to flush flate: fail to write frame"),
	}
	reusedConn.events = [][]byte{
		[]byte(`{"type":"response.output_item.done","response_id":"resp_delta_write_seed","output_index":0,"item":` + output1 + `}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_delta_write_seed","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":2}}}`),
	}
	retryConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.output_item.done","response_id":"resp_delta_write_retry","output_index":0,"item":` + output2 + `}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_delta_write_retry","model":"gpt-5.1","usage":{"input_tokens":5,"output_tokens":2}}}`),
		},
	}
	dialer := &openAIWSQueueDialer{conns: []openAIWSClientConn{reusedConn, retryConn}}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(dialer)
	t.Cleanup(pool.Close)

	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          78008,
		Name:        "openai-delta-write-retry",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token-delta-write-retry"},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}

	groupID := int64(78016)
	apiKeyID := int64(78017)
	newContext := func() *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
		c.Request.Header.Set("originator", "codex_exec")
		c.Request.Header.Set("session_id", "sess-delta-write-retry")
		c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
		return c
	}

	firstResult, err := svc.Forward(context.Background(), newContext(), account, []byte(`{"model":"gpt-5.1","stream":true,"input":[`+input1+`]}`))
	require.NoError(t, err)
	require.Equal(t, "resp_delta_write_seed", firstResult.RequestID)

	secondResult, err := svc.Forward(context.Background(), newContext(), account, []byte(`{"model":"gpt-5.1","stream":true,"input":[`+input1+`,`+output1+`,`+newInput+`]}`))
	require.NoError(t, err)
	require.Equal(t, "resp_delta_write_retry", secondResult.RequestID)
	require.Nil(t, upstream.lastReq, "write failure retry must stay on WS instead of HTTP fallback")
	require.Equal(t, 2, dialer.DialCount(), "write failure must evict the broken reused conn and dial one replacement")

	reusedConn.mu.Lock()
	reusedWrites := append([]map[string]any(nil), reusedConn.writes...)
	reusedConn.mu.Unlock()
	require.Len(t, reusedWrites, 2)
	firstSecondTurnWrite := requestToJSONString(reusedWrites[1])
	require.Equal(t, "resp_delta_write_seed", gjson.Get(firstSecondTurnWrite, "previous_response_id").String())
	require.False(t, gjson.Get(firstSecondTurnWrite, "store").Bool())
	require.Len(t, gjson.Get(firstSecondTurnWrite, "input").Array(), 1, "first second-turn attempt should be active delta")

	retryConn.mu.Lock()
	retryWrites := append([]map[string]any(nil), retryConn.writes...)
	retryConn.mu.Unlock()
	require.Len(t, retryWrites, 1)
	retryWrite := requestToJSONString(retryWrites[0])
	require.Equal(t, "resp_delta_write_seed", gjson.Get(retryWrite, "previous_response_id").String())
	require.True(t, gjson.Get(retryWrite, "store").Exists())
	require.False(t, gjson.Get(retryWrite, "store").Bool())
	require.Len(t, gjson.Get(retryWrite, "input").Array(), 1, "retry on a new conn must preserve active delta instead of full replay")
	require.Equal(t, "again", gjson.Get(retryWrite, "input.0.content.0.text").String())
}

func TestOpenAIWSActiveDelta_PreviousResponseNotFoundRetriesFullPayloadOverWS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("OPENAI_WS_DELTA_SHADOW_DISABLED", "")
	t.Setenv("OPENAI_WS_ACTIVE_DELTA_DISABLED", "")

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 3600
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}`
	output1 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}`

	firstConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.output_item.done","response_id":"resp_delta_prev_1","output_index":0,"item":` + output1 + `}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_delta_prev_1","model":"gpt-5.1","usage":{"input_tokens":3,"output_tokens":2}}}`),
			[]byte(`{"type":"error","error":{"code":"previous_response_not_found","type":"invalid_request_error","message":"previous response not found"}}`),
		},
	}
	secondConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"response.completed","response":{"id":"resp_delta_prev_2","model":"gpt-5.1","usage":{"input_tokens":5,"output_tokens":2}}}`),
		},
	}
	dialer := &openAIWSQueueDialer{conns: []openAIWSClientConn{firstConn, secondConn}}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(dialer)
	t.Cleanup(pool.Close)

	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          78004,
		Name:        "openai-delta-prev-recover",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token-delta-prev"},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}

	groupID := int64(78012)
	apiKeyID := int64(78013)
	newContext := func() *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
		c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
		c.Request.Header.Set("originator", "codex_exec")
		c.Request.Header.Set("session_id", "sess-delta-prev-recover")
		c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})
		return c
	}

	firstBody := []byte(`{"model":"gpt-5.1","stream":true,"input":[` + input1 + `]}`)
	firstResult, err := svc.Forward(context.Background(), newContext(), account, firstBody)
	require.NoError(t, err)
	require.Equal(t, "resp_delta_prev_1", firstResult.RequestID)

	secondBody := []byte(`{"model":"gpt-5.1","stream":true,"input":[` + input1 + `,` + output1 + `,` + newInput + `]}`)
	secondResult, err := svc.Forward(context.Background(), newContext(), account, secondBody)
	require.NoError(t, err)
	require.NotNil(t, secondResult)
	require.Equal(t, "resp_delta_prev_2", secondResult.RequestID)
	require.Nil(t, upstream.lastReq, "active-delta previous_response_not_found must recover over WS, not HTTP fallback")
	require.Equal(t, 2, dialer.DialCount(), "recovery should replace the broken WS connection once")

	firstConn.mu.Lock()
	firstWrites := append([]map[string]any(nil), firstConn.writes...)
	firstConn.mu.Unlock()
	require.Len(t, firstWrites, 2)
	deltaWrite := requestToJSONString(firstWrites[1])
	require.Equal(t, "resp_delta_prev_1", gjson.Get(deltaWrite, "previous_response_id").String())
	require.False(t, gjson.Get(deltaWrite, "store").Bool())
	require.Len(t, gjson.Get(deltaWrite, "input").Array(), 1, "first second-turn attempt should be active delta")

	secondConn.mu.Lock()
	secondWrites := append([]map[string]any(nil), secondConn.writes...)
	secondConn.mu.Unlock()
	require.Len(t, secondWrites, 1)
	retryWrite := requestToJSONString(secondWrites[0])
	require.False(t, gjson.Get(retryWrite, "previous_response_id").Exists(), "recovery retry must full-create without the stale active-delta anchor")
	require.True(t, gjson.Get(retryWrite, "store").Exists())
	require.False(t, gjson.Get(retryWrite, "store").Bool())
	require.Len(t, gjson.Get(retryWrite, "input").Array(), 3, "recovery retry must send the full original input sequence")
	require.Equal(t, "again", gjson.Get(retryWrite, "input.2.content.0.text").String())
}

func TestOpenAIWSFallbackToHTTPUsesFullPayloadAfterWSError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
	c.Request.Header.Set("originator", "codex_exec")
	c.Request.Header.Set("session_id", "sess-ws-http-fallback")
	groupID := int64(78020)
	apiKeyID := int64(78021)
	c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.RetryBackoffInitialMS = 0
	cfg.Gateway.OpenAIWS.RetryTotalBudgetMS = 1
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 3600
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	captureConn := &openAIWSCaptureConn{
		events: [][]byte{
			[]byte(`{"type":"error","error":{"code":"upgrade_required","type":"invalid_request_error","message":"websocket upgrade required"}}`),
		},
	}
	captureDialer := &openAIWSCaptureDialer{conn: captureConn}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			`data: {"type":"response.completed","response":{"id":"resp_http_fallback","object":"response","model":"gpt-5.1","status":"completed","output":[],"usage":{"input_tokens":9,"output_tokens":1,"total_tokens":10}}}` + "\n\n" +
				"data: [DONE]\n\n",
		)),
	}}
	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          78002,
		Name:        "openai-ws-http-fallback",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token-ws-http-fallback"},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}

	body := []byte(`{"model":"gpt-5.1","stream":false,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"two"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"three"}]}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "resp_http_fallback", result.ResponseID)
	require.Len(t, upstream.bodies, 1, "WS error before downstream output must fall back to one HTTP request")
	httpBody := string(upstream.bodies[0])
	require.False(t, gjson.Get(httpBody, "previous_response_id").Exists())
	require.True(t, gjson.Get(httpBody, "store").Exists())
	require.False(t, gjson.Get(httpBody, "store").Bool())
	require.Len(t, gjson.Get(httpBody, "input").Array(), 3, "HTTP fallback must use the full original payload, not a delta")
	require.Equal(t, "three", gjson.Get(httpBody, "input.2.content.0.text").String())
}

func TestOpenAIWSPreflightLargePayloadFallsBackToHTTPBeforeDial(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousThreshold := openAIWSOutboundPayloadHTTPFallbackThresholdBytes
	openAIWSOutboundPayloadHTTPFallbackThresholdBytes = 64
	t.Cleanup(func() {
		openAIWSOutboundPayloadHTTPFallbackThresholdBytes = previousThreshold
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_exec/0.124.0")
	c.Request.Header.Set("originator", "codex_exec")
	c.Request.Header.Set("session_id", "sess-ws-large-http-preflight")
	groupID := int64(78030)
	apiKeyID := int64(78031)
	c.Set("api_key", &APIKey{ID: apiKeyID, GroupID: &groupID})

	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StoreDisabledConnMode = openAIWSStoreDisabledConnModeStrict
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 3600
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600

	captureDialer := &openAIWSCaptureDialer{conn: &openAIWSCaptureConn{}}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(captureDialer)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			`data: {"type":"response.completed","response":{"id":"resp_large_http_preflight","object":"response","model":"gpt-5.1","status":"completed","output":[],"usage":{"input_tokens":9,"output_tokens":1,"total_tokens":10}}}` + "\n\n" +
				"data: [DONE]\n\n",
		)),
	}}
	svc := &OpenAIGatewayService{
		cfg:              cfg,
		httpUpstream:     upstream,
		cache:            &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg),
		toolCorrector:    NewCodexToolCorrector(),
		openaiWSPool:     pool,
	}
	account := &Account{
		ID:          78032,
		Name:        "openai-ws-large-http-preflight",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token-ws-large-http-preflight"},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}

	largeText := strings.Repeat("x", 128)
	body := []byte(`{"model":"gpt-5.1","stream":false,"store":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"` + largeText + `"}]}]}`)
	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 0, captureDialer.DialCount(), "large payload preflight should skip WS dial entirely")
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, largeText, gjson.GetBytes(upstream.bodies[0], "input.0.content.0.text").String())
}

func TestOpenAIWSActiveDelta_AllowsFunctionCallOutputDeltaWithPreviousResponseID(t *testing.T) {
	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}`
	output1 := `{"type":"function_call","call_id":"call_1","name":"shell","arguments":"{}"}`
	toolOutput := `{"type":"function_call_output","call_id":"call_1","output":"ok"}`
	payload := []byte(`{"model":"gpt-5.1","store":false,"input":[` + input1 + `,` + output1 + `,` + toolOutput + `]}`)
	nonInputHash, _ := openAIWSNonInputHash(payload)

	deltaPayload, deltaLog, applied, err := buildOpenAIWSActiveDeltaPayload(openAIWSDeltaShadowInput{
		RequestID:                "req_tool_continuation",
		AccountID:                78003,
		LeaseConnID:              "oa_ws_78003_1",
		ConnMostRecentResponseID: "resp_tool_1",
		CurrentPayload:           payload,
		HasFunctionCallOutput:    true,
		CachedFound:              true,
		Cached: openAIWSSessionContextValue{
			accountID:               78003,
			connID:                  "oa_ws_78003_1",
			lastResponseID:          "resp_tool_1",
			materializedHashes:      [][32]byte{mustItemHash(t, input1), mustItemHash(t, output1)},
			materializedCount:       2,
			inputCount:              1,
			nonInputHash:            nonInputHash,
			rawVsClientVisibleEqual: true,
		},
	})
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, deltaLog.Candidate)
	require.True(t, deltaLog.Active)
	require.Equal(t, 1, deltaLog.DeltaItems)
	deltaJSON := requestToJSONString(deltaPayload)
	require.Equal(t, "resp_tool_1", gjson.Get(deltaJSON, "previous_response_id").String())
	require.False(t, gjson.Get(deltaJSON, "store").Bool())
	require.Len(t, gjson.Get(deltaJSON, "input").Array(), 1)
	require.Equal(t, "function_call_output", gjson.Get(deltaJSON, "input.0.type").String())
	require.Equal(t, "call_1", gjson.Get(deltaJSON, "input.0.call_id").String())
}

func TestOpenAIWSActiveDelta_AllowsMixedToolContextDeltaWithPreviousResponseID(t *testing.T) {
	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}`
	output1 := `{"type":"function_call","call_id":"call_1","name":"shell","arguments":"{}"}`
	toolOutput := `{"type":"function_call_output","call_id":"call_1","output":"ok"}`
	nextCall := `{"type":"function_call","call_id":"call_2","name":"search","arguments":"{}"}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}`
	payload := []byte(`{"model":"gpt-5.1","store":false,"input":[` + input1 + `,` + output1 + `,` + toolOutput + `,` + nextCall + `,` + newInput + `]}`)
	nonInputHash, _ := openAIWSNonInputHash(payload)

	deltaPayload, deltaLog, applied, err := buildOpenAIWSActiveDeltaPayload(openAIWSDeltaShadowInput{
		RequestID:                "req_mixed_tool_context_delta",
		AccountID:                78006,
		LeaseConnID:              "oa_ws_78006_1",
		ConnMostRecentResponseID: "resp_tool_mixed_1",
		CurrentPayload:           payload,
		HasFunctionCallOutput:    true,
		CachedFound:              true,
		Cached: openAIWSSessionContextValue{
			accountID:               78006,
			connID:                  "oa_ws_78006_1",
			lastResponseID:          "resp_tool_mixed_1",
			materializedHashes:      [][32]byte{mustItemHash(t, input1), mustItemHash(t, output1)},
			materializedCount:       2,
			inputCount:              1,
			nonInputHash:            nonInputHash,
			rawVsClientVisibleEqual: true,
		},
	})
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, deltaLog.Candidate)
	require.True(t, deltaLog.Active)
	require.Equal(t, 3, deltaLog.DeltaItems)
	deltaJSON := requestToJSONString(deltaPayload)
	require.Equal(t, "resp_tool_mixed_1", gjson.Get(deltaJSON, "previous_response_id").String())
	require.False(t, gjson.Get(deltaJSON, "store").Bool())
	require.Len(t, gjson.Get(deltaJSON, "input").Array(), 3)
	require.Equal(t, "function_call_output", gjson.Get(deltaJSON, "input.0.type").String())
	require.Equal(t, "function_call", gjson.Get(deltaJSON, "input.1.type").String())
	require.Equal(t, "continue", gjson.Get(deltaJSON, "input.2.content.0.text").String())
}

func TestOpenAIWSActiveDelta_AllowsHistoricalFunctionCallOutputWhenDeltaIsUserMessage(t *testing.T) {
	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}`
	output1 := `{"type":"function_call","call_id":"call_1","name":"shell","arguments":"{}"}`
	toolOutput := `{"type":"function_call_output","call_id":"call_1","output":"ok"}`
	output2 := `{"type":"message","role":"assistant","content":[{"type":"output_text","text":"tool ok"}]}`
	newInput := `{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}`
	payload := []byte(`{"model":"gpt-5.1","store":false,"input":[` + input1 + `,` + output1 + `,` + toolOutput + `,` + output2 + `,` + newInput + `]}`)
	nonInputHash, _ := openAIWSNonInputHash(payload)

	deltaPayload, deltaLog, applied, err := buildOpenAIWSActiveDeltaPayload(openAIWSDeltaShadowInput{
		RequestID:                "req_historical_tool_output",
		AccountID:                78004,
		LeaseConnID:              "oa_ws_78004_1",
		ConnMostRecentResponseID: "resp_tool_2",
		CurrentPayload:           payload,
		HasFunctionCallOutput:    true,
		CachedFound:              true,
		Cached: openAIWSSessionContextValue{
			accountID:      78004,
			connID:         "oa_ws_78004_1",
			lastResponseID: "resp_tool_2",
			materializedHashes: [][32]byte{
				mustItemHash(t, input1),
				mustItemHash(t, output1),
				mustItemHash(t, toolOutput),
				mustItemHash(t, output2),
			},
			materializedCount:       4,
			inputCount:              2,
			nonInputHash:            nonInputHash,
			rawVsClientVisibleEqual: true,
		},
	})
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, deltaLog.Candidate)
	require.Equal(t, 1, deltaLog.DeltaItems)
	deltaJSON := requestToJSONString(deltaPayload)
	require.Equal(t, "resp_tool_2", gjson.Get(deltaJSON, "previous_response_id").String())
	require.Len(t, gjson.Get(deltaJSON, "input").Array(), 1)
	require.Equal(t, "continue", gjson.Get(deltaJSON, "input.0.content.0.text").String())
}

func TestOpenAIWSActiveDelta_SkipsSelfContainedFunctionCallOutputDelta(t *testing.T) {
	input1 := `{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}`
	toolCall := `{"type":"function_call","call_id":"call_1","name":"shell","arguments":"{}"}`
	toolOutput := `{"type":"function_call_output","call_id":"call_1","output":"ok"}`
	payload := []byte(`{"model":"gpt-5.1","store":false,"input":[` + input1 + `,` + toolCall + `,` + toolOutput + `]}`)
	nonInputHash, _ := openAIWSNonInputHash(payload)

	deltaPayload, deltaLog, applied, err := buildOpenAIWSActiveDeltaPayload(openAIWSDeltaShadowInput{
		RequestID:                "req_self_contained_tool_delta",
		AccountID:                78005,
		LeaseConnID:              "oa_ws_78005_1",
		ConnMostRecentResponseID: "resp_tool_context_1",
		CurrentPayload:           payload,
		HasFunctionCallOutput:    true,
		CachedFound:              true,
		Cached: openAIWSSessionContextValue{
			accountID:               78005,
			connID:                  "oa_ws_78005_1",
			lastResponseID:          "resp_tool_context_1",
			materializedHashes:      [][32]byte{mustItemHash(t, input1)},
			materializedCount:       1,
			inputCount:              1,
			nonInputHash:            nonInputHash,
			rawVsClientVisibleEqual: true,
		},
	})
	require.NoError(t, err)
	require.False(t, applied)
	require.Nil(t, deltaPayload)
	require.False(t, deltaLog.Candidate)
	require.Equal(t, "delta_tool_continuation_self_contained", deltaLog.FallbackReason)
	require.False(t, deltaLog.Active)
}
