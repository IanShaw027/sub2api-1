package service

import "encoding/json"

func prepareOpenAIWSUnsafeToolContinuationFullReplayPreflight(current map[string]any, originalFullPayload []byte) (map[string]any, bool, error) {
	if len(current) == 0 {
		return current, false, nil
	}
	if openAIWSPayloadString(current, "previous_response_id") == "" {
		return current, false, nil
	}
	if !HasFunctionCallOutput(current) {
		return current, false, nil
	}

	replayInputItems, replayInputExists, replayInputErr := openAIWSExtractNormalizedInputSequence(originalFullPayload)
	if replayInputErr != nil {
		return current, false, replayInputErr
	}
	if !replayInputExists || !openAIWSRawItemsHaveToolCallContextForOutputs(replayInputItems) {
		return current, false, nil
	}

	replayReqBody := map[string]any{}
	if err := json.Unmarshal(originalFullPayload, &replayReqBody); err != nil {
		return current, false, err
	}
	delete(replayReqBody, "previous_response_id")
	replayReqBody["store"] = false
	trimOpenAIStoreFalseReasoningItems(replayReqBody)
	return replayReqBody, true, nil
}
