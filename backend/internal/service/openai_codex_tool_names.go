package service

import (
	"encoding/json"
	"fmt"
	"strings"
)

const codexToolNameReverseKey = "openai_codex_tool_name_reverse"

func collectCodexToolNameReverse(reqBody map[string]any) (map[string]string, error) {
	reverse := make(map[string]string)
	remember := func(name string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		sanitized := sanitizeCodexToolName(name)
		if sanitized == "" || sanitized == name {
			return nil
		}
		if previous, exists := reverse[sanitized]; exists && previous != name {
			return fmt.Errorf("tool names %q and %q both normalize to %q", previous, name, sanitized)
		}
		reverse[sanitized] = name
		return nil
	}
	collect := func(rawTools any) error {
		tools, ok := rawTools.([]any)
		if !ok {
			return nil
		}
		for _, raw := range tools {
			tool, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if err := remember(firstNonEmptyString(tool["name"])); err != nil {
				return err
			}
			if function, ok := tool["function"].(map[string]any); ok {
				if err := remember(firstNonEmptyString(function["name"])); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := collect(reqBody["tools"]); err != nil {
		return nil, err
	}
	if err := collect(reqBody["functions"]); err != nil {
		return nil, err
	}
	input, _ := reqBody["input"].([]any)
	for _, raw := range input {
		item, ok := raw.(map[string]any)
		if !ok || strings.TrimSpace(firstNonEmptyString(item["type"])) != "additional_tools" {
			continue
		}
		if err := collect(item["tools"]); err != nil {
			return nil, err
		}
	}
	if len(reverse) == 0 {
		return nil, nil
	}
	return reverse, nil
}

func setCodexToolNameReverse(c interface{ Set(string, any) }, reverse map[string]string) {
	if c == nil || len(reverse) == 0 {
		return
	}
	copyMap := make(map[string]string, len(reverse))
	for sanitized, original := range reverse {
		copyMap[sanitized] = original
	}
	c.Set(codexToolNameReverseKey, copyMap)
}

func codexToolNameReverseFromContext(c interface{ Get(string) (any, bool) }) map[string]string {
	if c == nil {
		return nil
	}
	raw, ok := c.Get(codexToolNameReverseKey)
	if !ok {
		return nil
	}
	reverse, _ := raw.(map[string]string)
	return reverse
}

func restoreCodexToolNamesInJSON(data []byte, reverse map[string]string) []byte {
	if len(data) == 0 || len(reverse) == 0 || !json.Valid(data) {
		return data
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return data
	}
	changed := restoreCodexToolNameFields(decoded, reverse)
	if !changed {
		return data
	}
	out, err := json.Marshal(decoded)
	if err != nil {
		return data
	}
	return out
}

func restoreCodexToolNameFields(value any, reverse map[string]string) bool {
	changed := false
	switch typed := value.(type) {
	case map[string]any:
		if name, ok := typed["name"].(string); ok {
			if original, exists := reverse[name]; exists {
				typed["name"] = original
				changed = true
			}
		}
		for _, child := range typed {
			if restoreCodexToolNameFields(child, reverse) {
				changed = true
			}
		}
	case []any:
		for _, child := range typed {
			if restoreCodexToolNameFields(child, reverse) {
				changed = true
			}
		}
	}
	return changed
}
