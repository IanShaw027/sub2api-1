package service

import "testing"

func TestResolveThinkingProtocol(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  ThinkingProtocol
	}{
		{name: "claude strict", model: "claude-sonnet-4-5", want: ThinkingProtocolAnthropicStrict},
		{name: "opus strict", model: "opus-4-5", want: ThinkingProtocolAnthropicStrict},
		{name: "deepseek passback", model: "deepseek-v4-pro", want: ThinkingProtocolPassbackRequired},
		{name: "kimi passback", model: "kimi-k2.6", want: ThinkingProtocolPassbackRequired},
		{name: "moonshot passback", model: "moonshot-v1-32k", want: ThinkingProtocolPassbackRequired},
		{name: "glm passback", model: "glm-5.1", want: ThinkingProtocolPassbackRequired},
		{name: "minimax passback", model: "MiniMax-M2.7-highspeed", want: ThinkingProtocolPassbackRequired},
		{name: "qwen thinking passback", model: "qwen3-235b-a22b-thinking-2507", want: ThinkingProtocolPassbackRequired},
		{name: "qwen non thinking unknown", model: "qwen3-32b", want: ThinkingProtocolUnknown},
		{name: "gpt unknown", model: "gpt-5.5", want: ThinkingProtocolUnknown},
		{name: "empty unknown", model: "", want: ThinkingProtocolUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveThinkingProtocol(tt.model); got != tt.want {
				t.Fatalf("ResolveThinkingProtocol(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestThinkingProtocolGuards(t *testing.T) {
	if !ShouldPreFilterThinkingBlocks("claude-sonnet-4-5") {
		t.Fatal("claude models should pre-filter thinking blocks")
	}
	if ShouldPreFilterThinkingBlocks("deepseek-v4-pro") {
		t.Fatal("deepseek models should not pre-filter thinking blocks")
	}
	if !ShouldRectifyThinkingSignatureError("claude-sonnet-4-5") {
		t.Fatal("claude models should rectify signature errors")
	}
	if ShouldRectifyThinkingSignatureError("qwen3-235b-a22b-thinking-2507") {
		t.Fatal("qwen thinking models should not rectify signature errors")
	}
	if !ShouldApplyRetryFilters("haiku-4-5") {
		t.Fatal("haiku models should apply retry filters")
	}
	if ShouldApplyRetryFilters("moonshot-v1-32k") {
		t.Fatal("moonshot models should not apply retry filters")
	}
}
