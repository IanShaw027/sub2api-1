package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type responsesAnthropicIngressPlan struct {
	PrimaryBody      []byte
	FullReplayBody   []byte
	FullReplaySource string
}

type normalizedLegacyResponsesMessages struct {
	InputRaw      []byte
	Instructions string
}

func (p responsesAnthropicIngressPlan) CanRetryWithFullReplay() bool {
	return len(p.FullReplayBody) > 0 && !bytes.Equal(p.PrimaryBody, p.FullReplayBody)
}

func prepareResponsesAnthropicIngress(body []byte) (responsesAnthropicIngressPlan, error) {
	plan := responsesAnthropicIngressPlan{PrimaryBody: body}
	if len(body) == 0 {
		return plan, nil
	}

	inputValue := gjson.GetBytes(body, "input")
	messagesValue := gjson.GetBytes(body, "messages")
	hasInput := inputValue.Exists() && inputValue.Type != gjson.Null

	if hasInput {
		fullReplayBody, err := replaceResponsesIngressInput(body, []byte(inputValue.Raw), true)
		if err != nil {
			return plan, err
		}
		plan.FullReplayBody = dropResponsesIngressPreviousResponseID(fullReplayBody)
		plan.FullReplaySource = "input"
		return plan, nil
	}

	if messagesValue.Exists() {
		normalized, err := normalizeLegacyResponsesMessages(messagesValue.Raw)
		if err != nil {
			return plan, err
		}
		fullReplayBody, err := replaceResponsesIngressInput(body, normalized.InputRaw, true)
		if err != nil {
			return plan, err
		}
		existingInstructions := gjson.GetBytes(fullReplayBody, "instructions")
		if normalized.Instructions != "" &&
			(!existingInstructions.Exists() || existingInstructions.Type != gjson.String || strings.TrimSpace(existingInstructions.String()) == "") {
			fullReplayBody, err = sjson.SetBytes(fullReplayBody, "instructions", normalized.Instructions)
			if err != nil {
				return plan, fmt.Errorf("set normalized responses instructions: %w", err)
			}
		}
		plan.PrimaryBody = dropResponsesIngressPreviousResponseID(fullReplayBody)
		return plan, nil
	}

	return plan, nil
}

func normalizeLegacyResponsesMessages(messagesRaw string) (normalizedLegacyResponsesMessages, error) {
	var normalized normalizedLegacyResponsesMessages
	var messages []apicompat.ChatMessage
	if err := json.Unmarshal([]byte(messagesRaw), &messages); err != nil {
		return normalized, fmt.Errorf("normalize legacy responses messages: %w", err)
	}
	responsesReq, err := apicompat.ChatCompletionsToResponses(&apicompat.ChatCompletionsRequest{
		Messages: messages,
	})
	if err != nil {
		return normalized, fmt.Errorf("normalize legacy responses messages: %w", err)
	}
	if len(responsesReq.Input) == 0 {
		return normalized, fmt.Errorf("normalize legacy responses messages: empty input")
	}
	normalized.InputRaw = responsesReq.Input
	normalized.Instructions = strings.TrimSpace(responsesReq.Instructions)
	return normalized, nil
}

func replaceResponsesIngressInput(body, inputRaw []byte, deleteMessages bool) ([]byte, error) {
	updated, err := sjson.SetRawBytes(body, "input", inputRaw)
	if err != nil {
		return nil, fmt.Errorf("set normalized responses input: %w", err)
	}
	if !deleteMessages {
		return updated, nil
	}
	deleted, err := sjson.DeleteBytes(updated, "messages")
	if err != nil {
		return nil, fmt.Errorf("delete legacy responses messages: %w", err)
	}
	return deleted, nil
}

func dropResponsesIngressPreviousResponseID(body []byte) []byte {
	updated, err := sjson.DeleteBytes(body, "previous_response_id")
	if err != nil {
		return body
	}
	return updated
}

type responsesAnthropicFailureClassification struct {
	Reason              string
	RetryWithFullReplay bool
}

func classifyResponsesAnthropicFailure(statusCode int, upstreamBody []byte) responsesAnthropicFailureClassification {
	if statusCode < http.StatusBadRequest || statusCode >= http.StatusInternalServerError {
		return responsesAnthropicFailureClassification{}
	}

	errType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(upstreamBody, "error.type").String()))
	if errType != "" && errType != "invalid_request_error" {
		return responsesAnthropicFailureClassification{}
	}

	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(upstreamBody)))
	if msg == "" {
		msg = strings.ToLower(strings.TrimSpace(string(upstreamBody)))
	}
	if msg == "" {
		return responsesAnthropicFailureClassification{}
	}

	switch {
	case strings.Contains(msg, "previous_response_not_found"),
		strings.Contains(msg, "previous response not found"):
		return responsesAnthropicFailureClassification{Reason: "previous_response_not_found", RetryWithFullReplay: true}
	case strings.Contains(msg, "anchor") && containsAnyResponsesAnthropicFailureToken(msg, "invalid", "missing", "stale", "not found"):
		return responsesAnthropicFailureClassification{Reason: "invalid_anchor", RetryWithFullReplay: true}
	case strings.Contains(msg, "continuation") && containsAnyResponsesAnthropicFailureToken(msg, "invalid", "missing", "stale", "not found"):
		return responsesAnthropicFailureClassification{Reason: "invalid_continuation", RetryWithFullReplay: true}
	case strings.Contains(msg, "tool context"):
		return responsesAnthropicFailureClassification{Reason: "tool_context", RetryWithFullReplay: true}
	case strings.Contains(msg, "no tool call found") &&
		(strings.Contains(msg, "function call output") || strings.Contains(msg, "function_call_output")):
		return responsesAnthropicFailureClassification{Reason: "tool_continuation", RetryWithFullReplay: true}
	case strings.Contains(msg, "tool_result") && strings.Contains(msg, "tool_use") &&
		containsAnyResponsesAnthropicFailureToken(msg, "missing", "without", "follow", "preced"):
		return responsesAnthropicFailureClassification{Reason: "tool_continuation", RetryWithFullReplay: true}
	default:
		return responsesAnthropicFailureClassification{}
	}
}

func containsAnyResponsesAnthropicFailureToken(msg string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}
