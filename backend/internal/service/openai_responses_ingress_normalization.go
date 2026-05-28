package service

import (
	"bytes"
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
	InputRaw        []byte
	Instructions    string
	MaxOutputTokens *int
	Temperature     *float64
	TopP            *float64
	ToolsRaw        []byte
	ReasoningRaw    []byte
	ToolChoiceRaw   []byte
	ServiceTier     string
}

func normalizeOpenAIResponsesIngress(body []byte) (openAIResponsesIngressNormalization, error) {
	normalized := openAIResponsesIngressNormalization{PrimaryBody: body}
	if len(body) == 0 {
		return normalized, nil
	}

	inputValue := gjson.GetBytes(body, "input")
	messagesValue := gjson.GetBytes(body, "messages")
	hasInput := inputValue.Exists() && inputValue.Type != gjson.Null
	hasLegacyMessages := messagesValue.Exists() && messagesValue.Type == gjson.JSON && len(messagesValue.Array()) > 0

	if hasInput {
		fullReplayInputRaw := []byte(inputValue.Raw)
		fullReplaySource := "input"
		if hasLegacyMessages {
			legacyMessages, legacyErr := normalizeLegacyResponsesMessages(body)
			if legacyErr == nil && len(legacyMessages.InputRaw) > 0 {
				mergedInputRaw, merged, err := mergeResponsesReplayInputs(legacyMessages.InputRaw, fullReplayInputRaw)
				if err != nil {
					return normalized, err
				}
				if merged {
					fullReplayInputRaw = mergedInputRaw
					fullReplaySource = "input+messages"
				}
			}
		}

		fullReplayBody, err := replaceOpenAIResponsesIngressInput(body, fullReplayInputRaw, true)
		if err != nil {
			return normalized, err
		}
		if hasLegacyMessages {
			if legacyMessages, legacyErr := normalizeLegacyResponsesMessages(body); legacyErr == nil && len(legacyMessages.InputRaw) > 0 {
				fullReplayBody, err = setOpenAIResponsesIngressInstructions(fullReplayBody, legacyMessages.Instructions)
				if err != nil {
					return normalized, err
				}
				fullReplayBody, err = setNormalizedLegacyResponsesFields(fullReplayBody, legacyMessages)
				if err != nil {
					return normalized, err
				}
			}
		}
		normalized.FullReplayBody = dropOpenAIResponsesIngressPreviousResponseID(fullReplayBody)
		normalized.FullReplaySource = fullReplaySource
		return normalized, nil
	}

	if messagesValue.Exists() {
		legacyMessages, err := normalizeLegacyResponsesMessages(body)
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
		primaryBody, err = setNormalizedLegacyResponsesFields(primaryBody, legacyMessages)
		if err != nil {
			return normalized, err
		}
		normalized.PrimaryBody = dropOpenAIResponsesIngressPreviousResponseID(primaryBody)
	}

	return normalized, nil
}

func normalizeLegacyResponsesMessages(body []byte) (normalizedLegacyResponsesMessages, error) {
	var normalized normalizedLegacyResponsesMessages
	var chatReq apicompat.ChatCompletionsRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		return normalized, fmt.Errorf("normalize legacy responses messages: %w", err)
	}
	responsesReq, err := apicompat.ChatCompletionsToResponses(&chatReq)
	if err != nil {
		return normalized, fmt.Errorf("normalize legacy responses messages: %w", err)
	}
	if len(responsesReq.Input) == 0 {
		return normalized, fmt.Errorf("normalize legacy responses messages: empty input")
	}
	normalized.InputRaw = responsesReq.Input
	normalized.Instructions = strings.TrimSpace(responsesReq.Instructions)
	normalized.MaxOutputTokens = responsesReq.MaxOutputTokens
	normalized.Temperature = responsesReq.Temperature
	normalized.TopP = responsesReq.TopP
	normalized.ServiceTier = strings.TrimSpace(responsesReq.ServiceTier)

	if len(responsesReq.Tools) > 0 {
		toolsRaw, err := json.Marshal(responsesReq.Tools)
		if err != nil {
			return normalized, fmt.Errorf("normalize legacy responses messages tools: %w", err)
		}
		normalized.ToolsRaw = toolsRaw
	}
	if responsesReq.Reasoning != nil {
		reasoningRaw, err := json.Marshal(responsesReq.Reasoning)
		if err != nil {
			return normalized, fmt.Errorf("normalize legacy responses messages reasoning: %w", err)
		}
		normalized.ReasoningRaw = reasoningRaw
	}
	if len(responsesReq.ToolChoice) > 0 {
		normalized.ToolChoiceRaw = responsesReq.ToolChoice
	}
	return normalized, nil
}

func setNormalizedLegacyResponsesFields(body []byte, legacy normalizedLegacyResponsesMessages) ([]byte, error) {
	updated := body
	var err error
	if legacy.MaxOutputTokens != nil {
		updated, err = sjson.SetBytes(updated, "max_output_tokens", *legacy.MaxOutputTokens)
		if err != nil {
			return nil, fmt.Errorf("set normalized responses max_output_tokens: %w", err)
		}
	}
	if legacy.Temperature != nil {
		updated, err = sjson.SetBytes(updated, "temperature", *legacy.Temperature)
		if err != nil {
			return nil, fmt.Errorf("set normalized responses temperature: %w", err)
		}
	}
	if legacy.TopP != nil {
		updated, err = sjson.SetBytes(updated, "top_p", *legacy.TopP)
		if err != nil {
			return nil, fmt.Errorf("set normalized responses top_p: %w", err)
		}
	}
	if len(legacy.ToolsRaw) > 0 {
		updated, err = sjson.SetRawBytes(updated, "tools", legacy.ToolsRaw)
		if err != nil {
			return nil, fmt.Errorf("set normalized responses tools: %w", err)
		}
	}
	if len(legacy.ReasoningRaw) > 0 {
		updated, err = sjson.SetRawBytes(updated, "reasoning", legacy.ReasoningRaw)
		if err != nil {
			return nil, fmt.Errorf("set normalized responses reasoning: %w", err)
		}
	}
	if len(legacy.ToolChoiceRaw) > 0 {
		updated, err = sjson.SetRawBytes(updated, "tool_choice", legacy.ToolChoiceRaw)
		if err != nil {
			return nil, fmt.Errorf("set normalized responses tool_choice: %w", err)
		}
	}
	if legacy.ServiceTier != "" {
		updated, err = sjson.SetBytes(updated, "service_tier", legacy.ServiceTier)
		if err != nil {
			return nil, fmt.Errorf("set normalized responses service_tier: %w", err)
		}
	}

	for _, key := range []string{"max_tokens", "max_completion_tokens", "reasoning_effort", "functions", "function_call"} {
		updated, err = sjson.DeleteBytes(updated, key)
		if err != nil {
			return nil, fmt.Errorf("delete legacy chat completions field %q: %w", key, err)
		}
	}
	return updated, nil
}

func mergeResponsesReplayInputs(legacyInputRaw, inputRaw []byte) ([]byte, bool, error) {
	legacyItems, err := decodeResponsesReplayInputItems(legacyInputRaw)
	if err != nil {
		return nil, false, fmt.Errorf("decode legacy replay input: %w", err)
	}
	currentItems, err := decodeResponsesReplayInputItems(inputRaw)
	if err != nil {
		return nil, false, fmt.Errorf("decode current replay input: %w", err)
	}
	if len(legacyItems) == 0 {
		return inputRaw, false, nil
	}
	if len(currentItems) == 0 {
		mergedRaw, err := json.Marshal(legacyItems)
		if err != nil {
			return nil, false, fmt.Errorf("marshal legacy replay input: %w", err)
		}
		return mergedRaw, true, nil
	}

	seen := make(map[string]struct{}, len(legacyItems)+len(currentItems))
	merged := make([]json.RawMessage, 0, len(legacyItems)+len(currentItems))
	appendUnique := func(items []json.RawMessage) error {
		for _, item := range items {
			key, err := compactJSONRaw(item)
			if err != nil {
				return err
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			merged = append(merged, item)
		}
		return nil
	}
	if err := appendUnique(legacyItems); err != nil {
		return nil, false, fmt.Errorf("compact legacy replay input: %w", err)
	}
	if err := appendUnique(currentItems); err != nil {
		return nil, false, fmt.Errorf("compact current replay input: %w", err)
	}

	if len(merged) == len(currentItems) {
		same := true
		for i := range currentItems {
			currentKey, err := compactJSONRaw(currentItems[i])
			if err != nil {
				return nil, false, fmt.Errorf("compact merged replay input: %w", err)
			}
			mergedKey, err := compactJSONRaw(merged[i])
			if err != nil {
				return nil, false, fmt.Errorf("compact merged replay input: %w", err)
			}
			if currentKey != mergedKey {
				same = false
				break
			}
		}
		if same {
			return inputRaw, false, nil
		}
	}

	mergedRaw, err := json.Marshal(merged)
	if err != nil {
		return nil, false, fmt.Errorf("marshal merged replay input: %w", err)
	}
	return mergedRaw, true, nil
}

func decodeResponsesReplayInputItems(inputRaw []byte) ([]json.RawMessage, error) {
	input := gjson.ParseBytes(inputRaw)
	switch {
	case !input.Exists():
		return nil, nil
	case input.IsArray():
		items := make([]json.RawMessage, 0, len(input.Array()))
		input.ForEach(func(_, item gjson.Result) bool {
			items = append(items, json.RawMessage(item.Raw))
			return true
		})
		return items, nil
	case input.IsObject():
		return []json.RawMessage{json.RawMessage(input.Raw)}, nil
	default:
		return nil, nil
	}
}

func compactJSONRaw(raw []byte) (string, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return "", err
	}
	return buf.String(), nil
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
