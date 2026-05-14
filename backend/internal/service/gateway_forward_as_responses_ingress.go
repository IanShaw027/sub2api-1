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

func prepareResponsesAnthropicIngress(body []byte) (responsesAnthropicIngressPlan, error) {
	normalized, err := normalizeOpenAIResponsesIngress(body)
	if err != nil {
		return responsesAnthropicIngressPlan{PrimaryBody: body}, err
	}
	return responsesAnthropicIngressPlan{
		PrimaryBody:      normalized.PrimaryBody,
		FullReplayBody:   normalized.FullReplayBody,
		FullReplaySource: normalized.FullReplaySource,
	}, nil
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
