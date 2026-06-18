package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

func detectOpenAICyberPolicy(payload []byte) (matched bool, code, msg string) {
	if len(payload) == 0 {
		return false, "", ""
	}

	candidates := []struct {
		codePath string
		msgPath  string
	}{
		{codePath: "error.code", msgPath: "error.message"},
		{codePath: "response.error.code", msgPath: "response.error.message"},
	}

	for _, candidate := range candidates {
		codeValue := strings.TrimSpace(gjson.GetBytes(payload, candidate.codePath).String())
		if !strings.EqualFold(codeValue, "cyber_policy") {
			continue
		}
		return true, codeValue, strings.TrimSpace(gjson.GetBytes(payload, candidate.msgPath).String())
	}

	if bytes.Contains(payload, []byte("data:")) || bytes.Contains(payload, []byte("event:")) {
		for _, frame := range openAICompatSSEFramesFromBody(string(payload)) {
			data := strings.TrimSpace(openAICompatPayloadWithEventType(frame.Data, frame.EventType))
			if data == "" || data == "[DONE]" {
				continue
			}
			if matched, code, msg := detectOpenAICyberPolicy([]byte(data)); matched {
				return true, code, msg
			}
		}
	}

	return false, "", ""
}

func (s *RateLimitService) HandleOpenAICyberPolicy(ctx context.Context, account *Account, responseBody []byte) bool {
	if s == nil || account == nil || account.Platform != PlatformOpenAI {
		return false
	}
	matched, code, upstreamMsg := detectOpenAICyberPolicy(responseBody)
	if !matched {
		return false
	}

	upstreamMsg = sanitizeUpstreamErrorMessage(strings.TrimSpace(upstreamMsg))
	if upstreamMsg != "" {
		upstreamMsg = truncateForLog([]byte(upstreamMsg), 512)
	}
	msg := fmt.Sprintf("被风控命中(%s)", strings.TrimSpace(code))
	if upstreamMsg != "" {
		msg += ": " + upstreamMsg
	}
	s.handleAuthError(ctx, account, msg)
	return true
}
