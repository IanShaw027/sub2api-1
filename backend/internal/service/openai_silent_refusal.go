package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

const (
	openAISilentRefusalErrorCode     = "openai_silent_refusal"
	openAISilentRefusalClientMessage = "Upstream returned an empty completion without usage; no fallback account was available"
)

// IsOpenAISilentRefusalErrorBody reports whether a failover body uses the
// silent-refusal error code.
func IsOpenAISilentRefusalErrorBody(body []byte) bool {
	return strings.TrimSpace(gjson.GetBytes(body, "error.code").String()) == openAISilentRefusalErrorCode
}

// OpenAISilentRefusalClientMessage returns the exhausted-failover client message
// for OpenAI silent refusals.
func OpenAISilentRefusalClientMessage() string {
	return openAISilentRefusalClientMessage
}
