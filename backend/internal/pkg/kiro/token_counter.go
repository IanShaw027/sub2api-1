package kiro

import (
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

var (
	tokenEncoderOnce sync.Once
	tokenEncoder     *tiktoken.Tiktoken
	tokenEncoderErr  error
)

// getTokenEncoder returns a cached cl100k_base encoder for stable estimates.
func getTokenEncoder() (*tiktoken.Tiktoken, error) {
	tokenEncoderOnce.Do(func() {
		tokenEncoder, tokenEncoderErr = tiktoken.GetEncoding("cl100k_base")
	})
	return tokenEncoder, tokenEncoderErr
}

// AccurateTokenCount uses tiktoken for a closer estimate than the legacy rune
// heuristic. It falls back to roughTokenCount if tiktoken is unavailable.
func AccurateTokenCount(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	encoder, err := getTokenEncoder()
	if err != nil {
		// Fallback to rough estimation
		return roughTokenCount(text)
	}

	tokens := encoder.Encode(text, nil, nil)
	return len(tokens)
}

// roughTokenCount provides a fast but less accurate token estimation
// Used as fallback when tiktoken is unavailable
func roughTokenCount(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	if runes <= 0 {
		return 0
	}
	return (runes + 3) / 4
}
