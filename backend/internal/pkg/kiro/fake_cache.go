package kiro

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const FakeCacheTTL = 5 * time.Minute

type FakeCachePlan struct {
	PreviousKey             string
	CurrentKey              string
	PreviousCacheableTokens int
	CurrentCacheableTokens  int
}

type FakeCacheUsage struct {
	InputTokens              int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

func BuildFakeCachePlan(body []byte, accountID int64, requestedModel string) (*FakeCachePlan, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	sessionID := extractSessionID(req)
	if sessionID == "" {
		return nil, nil
	}

	rawMessages, _ := req["messages"].([]any)
	rawMessages = trimTrailingNonUserMessages(rawMessages)
	if len(rawMessages) == 0 {
		return nil, nil
	}

	currentChain := buildFakeCacheChain(req, rawMessages, true)
	if currentChain == "" {
		return nil, nil
	}

	scope := fmt.Sprintf(
		"kiro:fakecache:v1:acct:%d:model:%s:session:%s",
		accountID,
		strings.ToLower(strings.TrimSpace(requestedModel)),
		sessionID,
	)

	plan := &FakeCachePlan{
		CurrentKey:              fakeCacheKey(scope, currentChain),
		CurrentCacheableTokens:  roughTokenCount(currentChain),
		PreviousCacheableTokens: 0,
	}

	previousChain := buildFakeCacheChain(req, rawMessages, false)
	if previousChain != "" {
		plan.PreviousKey = fakeCacheKey(scope, previousChain)
		plan.PreviousCacheableTokens = roughTokenCount(previousChain)
	}

	return plan, nil
}

func (p *FakeCachePlan) ResolveUsage(totalInputTokens int, hit bool) FakeCacheUsage {
	if totalInputTokens < 0 {
		totalInputTokens = 0
	}
	if p == nil {
		return FakeCacheUsage{InputTokens: totalInputTokens}
	}

	currentCacheable := clampFakeCacheTokens(p.CurrentCacheableTokens, totalInputTokens)
	previousCacheable := clampFakeCacheTokens(p.PreviousCacheableTokens, currentCacheable)

	if !hit {
		return FakeCacheUsage{
			InputTokens:              totalInputTokens,
			CacheCreationInputTokens: currentCacheable,
		}
	}

	cacheRead := clampFakeCacheTokens(previousCacheable, totalInputTokens)
	cacheWrite := currentCacheable - previousCacheable
	if cacheWrite < 0 {
		cacheWrite = 0
	}

	inputTokens := totalInputTokens - cacheRead
	if inputTokens < 0 {
		inputTokens = 0
	}

	return FakeCacheUsage{
		InputTokens:              inputTokens,
		CacheCreationInputTokens: cacheWrite,
		CacheReadInputTokens:     cacheRead,
	}
}

func buildFakeCacheChain(req map[string]any, messages []any, includeCurrent bool) string {
	var parts []string

	if systemText := joinSystem(req["system"]); systemText != "" {
		parts = append(parts, "system:"+systemText)
	}

	if tools, _ := req["tools"].([]any); len(tools) > 0 {
		for _, item := range tools {
			tool, _ := item.(map[string]any)
			name := stringField(tool, "name")
			description := stringField(tool, "description")
			if name == "" && description == "" {
				continue
			}
			parts = append(parts, "tool:"+name+"\n"+description)
		}
	}

	limit := len(messages)
	if !includeCurrent && limit > 0 {
		limit--
	}
	for idx := 0; idx < limit; idx++ {
		msg, _ := messages[idx].(map[string]any)
		if !strings.EqualFold(stringField(msg, "role"), "user") {
			continue
		}
		content := fakeCacheUserContent(msg["content"])
		if content == "" {
			continue
		}
		parts = append(parts, "user:"+content)
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func fakeCacheUserContent(content any) string {
	switch v := content.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			block, _ := item.(map[string]any)
			switch strings.TrimSpace(stringField(block, "type")) {
			case "text":
				if text := stringField(block, "text"); text != "" {
					parts = append(parts, text)
				}
			case "tool_result":
				if text := strings.TrimSpace(toolResultContent(block["content"])); text != "" {
					parts = append(parts, text)
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return ""
	}
}

func fakeCacheKey(scope, chain string) string {
	sum := sha256.Sum256([]byte(chain))
	return scope + ":chain:" + hex.EncodeToString(sum[:])
}

func clampFakeCacheTokens(tokens, upper int) int {
	if tokens < 0 {
		return 0
	}
	if upper >= 0 && tokens > upper {
		return upper
	}
	return tokens
}
