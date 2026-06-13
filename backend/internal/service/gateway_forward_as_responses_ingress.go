package service

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

type responsesAnthropicIngressPlan struct {
	PrimaryBody      []byte
	FullReplayBody   []byte
	FullReplaySource string
}

func (p responsesAnthropicIngressPlan) CanRetryWithFullReplay() bool {
	return len(p.FullReplayBody) > 0 && !bytes.Equal(p.PrimaryBody, p.FullReplayBody)
}

func (p responsesAnthropicIngressPlan) CanRetryWithFullReplayForReason(reason string) bool {
	if !p.CanRetryWithFullReplay() {
		return false
	}
	switch recoverableFailureReason(reason) {
	case recoverableFailureInvalidContinuation,
		recoverableFailureToolContext,
		recoverableFailureToolContinuation,
		recoverableFailureEmptyMessages:
		return responsesFullReplayHasCompleteToolContinuationContext(p.FullReplayBody)
	default:
		return true
	}
}

func responsesFullReplayHasCompleteToolContinuationContext(body []byte) bool {
	input := gjson.GetBytes(body, "input")
	if !input.Exists() {
		return false
	}

	functionCalls := make(map[string]struct{})
	functionOutputs := make(map[string]struct{})
	visitInputItem := func(item gjson.Result) {
		itemType := strings.TrimSpace(item.Get("type").String())
		callID := strings.TrimSpace(item.Get("call_id").String())
		switch itemType {
		case "function_call":
			if callID != "" {
				functionCalls[callID] = struct{}{}
			}
		case "function_call_output":
			if callID != "" {
				functionOutputs[callID] = struct{}{}
			}
		}
	}

	if input.IsArray() {
		input.ForEach(func(_, item gjson.Result) bool {
			visitInputItem(item)
			return true
		})
	} else if input.IsObject() {
		visitInputItem(input)
	}

	if len(functionOutputs) == 0 {
		return true
	}
	for callID := range functionOutputs {
		if _, ok := functionCalls[callID]; !ok {
			return false
		}
	}
	return true
}

func prepareResponsesAnthropicIngress(body []byte) (responsesAnthropicIngressPlan, error) {
	normalized, err := normalizeOpenAIResponsesIngress(body)
	if err != nil {
		return responsesAnthropicIngressPlan{PrimaryBody: body}, err
	}
	return responsesAnthropicIngressPlan(normalized), nil
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

	reason, ok := classifyRecoverableFailureReason(msg)
	if !ok {
		return responsesAnthropicFailureClassification{}
	}

	policy := replayPolicyForRecoverableFailureReason(reason)
	return responsesAnthropicFailureClassification{
		Reason:              string(reason),
		RetryWithFullReplay: policy.RetryWithFullReplay,
	}
}
