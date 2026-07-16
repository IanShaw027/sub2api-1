package service

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	blockTypeServerToolUse       = "server_tool_use"
	blockTypeWebSearchToolResult = "web_search_tool_result"
)

var (
	patternServerToolUse       = []byte(`"server_tool_use"`)
	patternWebSearchToolResult = []byte(`"web_search_tool_result"`)
)

// FilterWebSearchHistoryBlocks removes locally emulated web-search blocks and
// web-search blocks that passback-required upstreams cannot accept.
//
// Blocks are dropped surgically via sjson so that every kept block retains its
// exact original JSON bytes. A full json.Unmarshal into []any would decode all
// numbers as float64 and re-encode them on marshal, silently corrupting large
// integer IDs (> 2^53) anywhere in the otherwise-untouched history.
func FilterWebSearchHistoryBlocks(body []byte, mappedModel string) []byte {
	if !bytes.Contains(body, patternServerToolUse) && !bytes.Contains(body, patternWebSearchToolResult) {
		return body
	}

	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body
	}

	stripAll := ResolveThinkingProtocol(mappedModel) == ThinkingProtocolPassbackRequired

	out := body
	modified := false
	for msgIdx, msg := range messages.Array() {
		content := msg.Get("content")
		if !content.IsArray() {
			continue
		}

		blocks := content.Array()
		drop := make([]bool, len(blocks))
		strippedResultToolUseIDs := make(map[string]struct{})
		anyDrop := false
		for i, block := range blocks {
			if block.IsObject() && shouldStripWebSearchBlock(block, stripAll) {
				drop[i] = true
				anyDrop = true
				if block.Get("type").String() == blockTypeWebSearchToolResult {
					if id := block.Get("tool_use_id").String(); id != "" {
						strippedResultToolUseIDs[id] = struct{}{}
					}
				}
			}
		}
		// A stripped web_search_tool_result leaves its paired server_tool_use orphaned
		// (a tool call with no result), which passback-required upstreams reject. Drop
		// the matching server_tool_use even if its name/type wasn't independently
		// recognized as web search.
		if len(strippedResultToolUseIDs) > 0 {
			for i, block := range blocks {
				if drop[i] || !block.IsObject() {
					continue
				}
				if block.Get("type").String() != blockTypeServerToolUse {
					continue
				}
				if _, ok := strippedResultToolUseIDs[block.Get("id").String()]; ok {
					drop[i] = true
					anyDrop = true
				}
			}
		}
		if !anyDrop {
			continue
		}

		kept := make([]string, 0, len(blocks))
		for i, block := range blocks {
			if drop[i] {
				continue
			}
			// Preserve the block's raw bytes verbatim (objects and non-objects alike).
			kept = append(kept, block.Raw)
		}

		var newContent string
		if len(kept) == 0 {
			// Anthropic rejects an empty content array; keep a minimal placeholder.
			if msg.Get("role").String() == "assistant" {
				newContent = `[{"type":"text","text":"(assistant content removed)"}]`
			} else {
				newContent = `[{"type":"text","text":"(content removed)"}]`
			}
		} else {
			newContent = "[" + strings.Join(kept, ",") + "]"
		}

		next, err := sjson.SetRawBytes(out, "messages."+strconv.Itoa(msgIdx)+".content", []byte(newContent))
		if err != nil {
			return body
		}
		out = next
		modified = true
	}

	if !modified {
		return body
	}
	return out
}

func shouldStripWebSearchBlock(block gjson.Result, stripAll bool) bool {
	switch block.Get("type").String() {
	case blockTypeServerToolUse:
		if strings.HasPrefix(block.Get("id").String(), webSearchToolUseIDPrefix) {
			return true
		}
		return stripAll && isWebSearchServerToolUse(block)
	case blockTypeWebSearchToolResult:
		if stripAll {
			return true
		}
		return strings.HasPrefix(block.Get("tool_use_id").String(), webSearchToolUseIDPrefix)
	default:
		return false
	}
}

func isWebSearchServerToolUse(block gjson.Result) bool {
	name := strings.ToLower(strings.TrimSpace(block.Get("name").String()))
	if name == toolNameWebSearch || name == toolNameGoogleSearch || name == toolNameWebSearch2025 {
		return true
	}
	blockType := strings.ToLower(strings.TrimSpace(block.Get("type").String()))
	return strings.HasPrefix(blockType, toolTypeWebSearchPrefix)
}
