package gemini

import "strings"

// ThinkingConfig uses generateContent field names, not the Interactions schema.
type ThinkingConfig struct {
	IncludeThoughts bool   `json:"includeThoughts"`
	ThinkingBudget  *int   `json:"thinkingBudget,omitempty"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
}

// ClaudeThinkingConfig adapts Anthropic controls for the final Gemini model.
// Gemini 3 and 2.5 Pro cannot fully disable thinking; disabled requests use
// their minimum supported setting and omit thought summaries instead.
func ClaudeThinkingConfig(model, thinkingType string, budget int, effort string) *ThinkingConfig {
	model = strings.ToLower(strings.TrimSpace(model))
	if slash := strings.LastIndex(model, "/"); slash >= 0 {
		model = model[slash+1:]
	}
	isThree := strings.HasPrefix(model, "gemini-3-") || strings.HasPrefix(model, "gemini-3.")
	isTwoFive := strings.HasPrefix(model, "gemini-2.5-") && !strings.Contains(model, "image")
	if !isThree && !isTwoFive {
		return nil
	}
	effort = strings.ToLower(strings.TrimSpace(effort))
	switch effort {
	case "minimal", "low", "medium", "high", "none":
	case "xhigh", "max":
		effort = "high"
	default:
		effort = ""
	}
	disabled := thinkingType == "disabled" || effort == "none"
	enabled := thinkingType == "enabled" || thinkingType == "adaptive"
	if !disabled && !enabled && effort == "" {
		return nil
	}
	config := &ThinkingConfig{IncludeThoughts: !disabled}
	if isThree {
		level := effort
		if disabled {
			level = "minimal"
		}
		if level == "" && budget > 0 {
			switch {
			case budget <= 1024:
				level = "low"
			case budget <= 8192:
				level = "medium"
			default:
				level = "high"
			}
		}
		if level == "minimal" && (strings.Contains(model, "pro") || strings.HasPrefix(model, "gemini-3.7-") || strings.HasPrefix(model, "gemini-3.8-")) {
			level = "low"
		}
		if level == "medium" && strings.HasPrefix(model, "gemini-3-pro") {
			level = "high"
		}
		if strings.Contains(model, "flash") && strings.Contains(model, "image") {
			if level == "low" {
				level = "minimal"
			}
			if level == "medium" {
				level = "high"
			}
		}
		config.ThinkingLevel = level
		return config
	}
	value := -1
	if disabled {
		value = 0
		if strings.Contains(model, "pro") {
			value = 128
		}
	} else if budget > 0 {
		value = budget
	} else {
		switch effort {
		case "minimal", "low":
			value = 1024
		case "medium":
			value = 8192
		case "high":
			value = 24576
		}
	}
	if value > 0 {
		minimum, maximum := 1, 24576
		if strings.Contains(model, "pro") {
			minimum, maximum = 128, 32768
		}
		if strings.Contains(model, "flash-lite") {
			minimum = 512
		}
		value = max(minimum, min(value, maximum))
	}
	config.ThinkingBudget = &value
	return config
}
