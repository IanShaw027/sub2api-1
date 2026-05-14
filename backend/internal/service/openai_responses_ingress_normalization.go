package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type openAIResponsesIngressNormalization struct {
	PrimaryBody      []byte
	FullReplayBody   []byte
	FullReplaySource string
}

type normalizedLegacyResponsesMessages struct {
	InputRaw     []byte
	Instructions string
}

func normalizeOpenAIResponsesIngress(body []byte) (openAIResponsesIngressNormalization, error) {
	normalized := openAIResponsesIngressNormalization{PrimaryBody: body}
	if len(body) == 0 {
		return normalized, nil
	}

	inputValue := gjson.GetBytes(body, "input")
	messagesValue := gjson.GetBytes(body, "messages")
	hasInput := inputValue.Exists() && inputValue.Type != gjson.Null

	if hasInput {
		fullReplayBody, err := replaceOpenAIResponsesIngressInput(body, []byte(inputValue.Raw), true)
		if err != nil {
			return normalized, err
		}
		normalized.FullReplayBody = dropOpenAIResponsesIngressPreviousResponseID(fullReplayBody)
		normalized.FullReplaySource = "input"
		return normalized, nil
	}

	if messagesValue.Exists() {
		legacyMessages, err := normalizeLegacyResponsesMessages(messagesValue.Raw)
		if err != nil {
			return normalized, err
		}
		primaryBody, err := replaceOpenAIResponsesIngressInput(body, legacyMessages.InputRaw, true)
		if err != nil {
			return normalized, err
		}
		primaryBody, err = setOpenAIResponsesIngressInstructions(primaryBody, legacyMessages.Instructions)
		if err != nil {
			return normalized, err
		}
		normalized.PrimaryBody = dropOpenAIResponsesIngressPreviousResponseID(primaryBody)
	}

	return normalized, nil
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

func replaceOpenAIResponsesIngressInput(body, inputRaw []byte, deleteMessages bool) ([]byte, error) {
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

func setOpenAIResponsesIngressInstructions(body []byte, instructions string) ([]byte, error) {
	if instructions == "" {
		return body, nil
	}
	existingInstructions := gjson.GetBytes(body, "instructions")
	if existingInstructions.Exists() &&
		existingInstructions.Type == gjson.String &&
		strings.TrimSpace(existingInstructions.String()) != "" {
		return body, nil
	}
	updated, err := sjson.SetBytes(body, "instructions", instructions)
	if err != nil {
		return nil, fmt.Errorf("set normalized responses instructions: %w", err)
	}
	return updated, nil
}

func dropOpenAIResponsesIngressPreviousResponseID(body []byte) []byte {
	updated, err := sjson.DeleteBytes(body, "previous_response_id")
	if err != nil {
		return body
	}
	return updated
}
