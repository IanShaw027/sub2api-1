package kiro

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultFakeCacheTTL            = 5 * time.Minute
	DefaultFakeCacheHitRateScale   = 95
	DefaultFakeCacheMinBlockTokens = 1024
	DefaultFakeCacheIndependentTTL = 3600
	DefaultFakeCachePrefixTTL      = 300
)

type FakeCachePlan struct {
	CacheStrategy                 string
	CacheStrategyGeneration       uint64
	PreviousKey                   string
	CurrentKey                    string
	PreviousCacheableTokens       int
	CurrentCacheableTokens        int
	IndependentKey                string
	IndependentCacheableTokens    int
	PreviousPrefixKey             string
	CurrentPrefixKey              string
	PreviousPrefixCacheableTokens int
	CurrentPrefixCacheableTokens  int
}

type FakeCacheUsage struct {
	InputTokens              int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

type FakeCacheHitState struct {
	Independent bool
	Prefix      bool
}

type FakeCacheUsageConfig struct {
	HitRateScale   int
	MinBlockTokens int
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

	independentChain := buildFakeCacheIndependentChain(req)
	currentPrefixChain := buildFakeCachePrefixChain(rawMessages, true)
	if independentChain == "" && currentPrefixChain == "" {
		return nil, nil
	}

	scope := fmt.Sprintf(
		"kiro:fakecache:v1:acct:%d:model:%s:session:%s",
		accountID,
		strings.ToLower(strings.TrimSpace(requestedModel)),
		sessionID,
	)

	plan := &FakeCachePlan{
		PreviousCacheableTokens: 0,
	}
	if independentChain != "" {
		plan.IndependentKey = fakeCacheKey(scope+":independent", independentChain)
		plan.IndependentCacheableTokens = roughTokenCount(independentChain)
	}
	prefixScope := scope + ":prefix:independent:" + fakeCacheDigest(independentChain)
	if currentPrefixChain != "" {
		plan.CurrentPrefixKey = fakeCacheKey(prefixScope, currentPrefixChain)
		plan.CurrentPrefixCacheableTokens = roughTokenCount(currentPrefixChain)
		plan.CurrentKey = plan.CurrentPrefixKey
	}
	plan.CurrentCacheableTokens = plan.IndependentCacheableTokens + plan.CurrentPrefixCacheableTokens

	previousPrefixChain := buildFakeCachePrefixChain(rawMessages, false)
	if previousPrefixChain != "" {
		plan.PreviousPrefixKey = fakeCacheKey(prefixScope, previousPrefixChain)
		plan.PreviousPrefixCacheableTokens = roughTokenCount(previousPrefixChain)
		plan.PreviousKey = plan.PreviousPrefixKey
	}
	plan.PreviousCacheableTokens = plan.IndependentCacheableTokens + plan.PreviousPrefixCacheableTokens

	return plan, nil
}

func (p *FakeCachePlan) ResolveUsage(totalInputTokens int, hit bool) FakeCacheUsage {
	return p.ResolveUsageWithConfig(totalInputTokens, FakeCacheHitState{Prefix: hit}, FakeCacheUsageConfig{
		HitRateScale: 100,
	})
}

func (p *FakeCachePlan) ResolveUsageWithConfig(totalInputTokens int, hit FakeCacheHitState, config FakeCacheUsageConfig) FakeCacheUsage {
	if totalInputTokens < 0 {
		totalInputTokens = 0
	}
	if p == nil {
		return FakeCacheUsage{InputTokens: totalInputTokens}
	}

	if config.HitRateScale < 0 || config.HitRateScale > 100 {
		config.HitRateScale = 100
	}
	independentCurrent := clampFakeCacheTokens(applyMinBlockTokens(p.IndependentCacheableTokens, config.MinBlockTokens), totalInputTokens)
	currentPrefix := clampFakeCacheTokens(applyMinBlockTokens(p.CurrentPrefixCacheableTokens, config.MinBlockTokens), totalInputTokens)
	previousPrefix := clampFakeCacheTokens(applyMinBlockTokens(p.PreviousPrefixCacheableTokens, config.MinBlockTokens), totalInputTokens)
	if independentCurrent == 0 && currentPrefix == 0 && previousPrefix == 0 &&
		p.IndependentKey == "" && p.CurrentPrefixKey == "" && p.PreviousPrefixKey == "" {
		currentPrefix = clampFakeCacheTokens(applyMinBlockTokens(p.CurrentCacheableTokens, config.MinBlockTokens), totalInputTokens)
		previousPrefix = clampFakeCacheTokens(applyMinBlockTokens(p.PreviousCacheableTokens, config.MinBlockTokens), totalInputTokens)
	}

	cacheRead := 0
	cacheWrite := 0
	if hit.Independent {
		cacheRead += independentCurrent
	} else {
		cacheWrite += independentCurrent
	}
	if hit.Prefix {
		cacheRead += previousPrefix
		cacheWrite += currentPrefix - previousPrefix
	} else {
		cacheWrite += currentPrefix
	}
	if cacheWrite < 0 {
		cacheWrite = 0
	}
	cacheRead = clampFakeCacheTokens(cacheRead, totalInputTokens)
	cacheWrite = clampFakeCacheTokens(cacheWrite, totalInputTokens-cacheRead)

	if config.HitRateScale < 100 {
		unscaledRead := cacheRead
		scaledRead := cacheRead * config.HitRateScale / 100
		if scaledRead < 0 {
			scaledRead = 0
		}
		cacheRead = scaledRead
		cacheWrite += unscaledRead - scaledRead
		cacheWrite = clampFakeCacheTokens(cacheWrite, totalInputTokens-cacheRead)
	}
	inputTokens := totalInputTokens - cacheRead - cacheWrite
	if inputTokens < 0 {
		inputTokens = 0
	}

	return FakeCacheUsage{
		InputTokens:              inputTokens,
		CacheCreationInputTokens: cacheWrite,
		CacheReadInputTokens:     cacheRead,
	}
}

func buildFakeCacheIndependentChain(req map[string]any) string {
	var parts []string

	if systemText := joinSystem(req["system"]); systemText != "" {
		parts = append(parts, "system:"+systemText+"\n"+systemChunkedPolicy)
	}

	if tools, _ := req["tools"].([]any); len(tools) > 0 {
		for _, item := range tools {
			tool, _ := item.(map[string]any)
			name := stringField(tool, "name")
			if name == "" {
				continue
			}
			parts = append(parts,
				"tool:"+name+
					"\ndescription:"+stringField(tool, "description")+
					"\ninput_schema:"+fakeCacheJSON(normalizeJSONSchema(jsonValue(tool["input_schema"]))),
			)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func buildFakeCachePrefixChain(messages []any, includeCurrent bool) string {
	var parts []string
	limit := len(messages)
	if !includeCurrent && limit > 0 {
		limit--
	}
	for idx := 0; idx < limit; idx++ {
		msg, _ := messages[idx].(map[string]any)
		role := strings.ToLower(strings.TrimSpace(stringField(msg, "role")))
		switch role {
		case "user":
			if content := fakeCacheUserContent(msg["content"]); content != "" {
				parts = append(parts, "user:"+content)
			}
		case "assistant":
			if content := fakeCacheAssistantContent(msg["content"]); content != "" {
				parts = append(parts, "assistant:"+content)
			}
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func applyMinBlockTokens(tokens, minBlockTokens int) int {
	if tokens <= 0 {
		return 0
	}
	if minBlockTokens > 0 && tokens < minBlockTokens {
		return 0
	}
	return tokens
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
				status := "success"
				if isError, ok := block["is_error"].(bool); ok && isError {
					status = "error"
				}
				parts = append(parts,
					"tool_result:"+
						"\ntool_use_id:"+stringField(block, "tool_use_id")+
						"\nstatus:"+status+
						"\ncontent:"+strings.TrimSpace(toolResultContent(block["content"])),
				)
			case "image":
				parts = append(parts, "image:"+fakeCacheJSON(block))
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return ""
	}
}

func fakeCacheAssistantContent(content any) string {
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
			case "tool_use":
				parts = append(parts,
					"tool_use:"+
						"\nid:"+stringField(block, "id")+
						"\nname:"+stringField(block, "name")+
						"\ninput:"+fakeCacheJSON(jsonValue(block["input"])),
				)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		return ""
	}
}

func fakeCacheJSON(v any) string {
	encoded, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(encoded)
}

func fakeCacheKey(scope, chain string) string {
	return scope + ":chain:" + fakeCacheDigest(chain)
}

func fakeCacheDigest(chain string) string {
	sum := sha256.Sum256([]byte(chain))
	return hex.EncodeToString(sum[:])
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
