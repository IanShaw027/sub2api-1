package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// buildOpenAIContextCompactionTriggerBody prepares the client-compatible
// remote-compaction request used only after an upstream context overflow. The
// original input is retained verbatim and the trigger is appended as a final
// item, so the upstream owns the actual summarization semantics.
func buildOpenAIContextCompactionTriggerBody(body []byte) ([]byte, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil, fmt.Errorf("invalid OpenAI request body")
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() || len(input.Array()) == 0 {
		return nil, fmt.Errorf("input must be a non-empty array")
	}
	var requestObject map[string]any
	if err := json.Unmarshal(body, &requestObject); err != nil {
		return nil, fmt.Errorf("decode OpenAI request body: %w", err)
	}
	if HasFunctionCallOutput(requestObject) || containsOpenAIContextCompactionTrigger(input) {
		return nil, fmt.Errorf("request is not safe for automatic compaction")
	}
	for _, item := range input.Array() {
		if !openAIContextCompactionInputItemSafe(item) {
			return nil, fmt.Errorf("request contains unsupported compaction input item")
		}
	}

	trigger := map[string]any{"type": "compaction_trigger"}
	triggerJSON, err := json.Marshal(trigger)
	if err != nil {
		return nil, err
	}
	nextInput := make([]json.RawMessage, 0, len(input.Array())+1)
	for _, item := range input.Array() {
		nextInput = append(nextInput, json.RawMessage(item.Raw))
	}
	nextInput = append(nextInput, triggerJSON)
	encodedInput, err := json.Marshal(nextInput)
	if err != nil {
		return nil, fmt.Errorf("marshal compaction input: %w", err)
	}
	updated, err := sjson.SetRawBytes(body, "input", encodedInput)
	if err != nil {
		return nil, fmt.Errorf("set compaction input: %w", err)
	}
	// The compact endpoint is unary upstream even when the client consumes SSE.
	updated, err = sjson.DeleteBytes(updated, "store")
	if err != nil {
		return nil, fmt.Errorf("remove compact store field: %w", err)
	}
	return sjson.DeleteBytes(updated, "previous_response_id")
}

func isOpenAIContextRemoteCompactionV2Request(c *gin.Context, body []byte) bool {
	if c == nil || c.Request == nil || !gjson.GetBytes(body, "stream").Bool() {
		return false
	}
	for _, header := range c.Request.Header.Values("x-codex-beta-features") {
		for _, feature := range strings.Split(header, ",") {
			if strings.EqualFold(strings.TrimSpace(feature), "remote_compaction_v2") {
				return true
			}
		}
	}
	return false
}

func isOpenAIContextCompactionStatus(status int) bool {
	return status == http.StatusBadRequest
}

func containsOpenAIContextCompactionTrigger(input gjson.Result) bool {
	for _, item := range input.Array() {
		if strings.EqualFold(strings.TrimSpace(item.Get("type").String()), "compaction_trigger") {
			return true
		}
	}
	return false
}

func openAIContextCompactionInputItemSafe(item gjson.Result) bool {
	if !item.IsObject() {
		return false
	}
	typeName := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
	if typeName == "" {
		typeName = "message"
	}
	if typeName != "message" && typeName != "reasoning" {
		return false
	}
	content := item.Get("content")
	if content.IsArray() {
		for _, part := range content.Array() {
			partType := strings.ToLower(strings.TrimSpace(part.Get("type").String()))
			if strings.Contains(partType, "image") || strings.Contains(partType, "file") {
				return false
			}
		}
	}
	return true
}
