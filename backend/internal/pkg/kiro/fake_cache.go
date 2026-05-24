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
	DefaultFakeCacheHitRateScale   = 100
	DefaultFakeCacheMinBlockTokens = 1024
	DefaultFakeCacheIndependentTTL = 3600
	DefaultFakeCachePrefixTTL      = 300
)

type FakeCachePlan struct {
	CacheStrategy                 string
	CacheStrategyGeneration       uint64
	SessionProgressKey            string
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
	Checkpoints                   []FakeCacheCheckpoint
}

type FakeCacheCheckpoint struct {
	Key    string
	Tokens int
}

type FakeCacheUsage struct {
	InputTokens              int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

type FakeCacheHitState struct {
	Independent      bool
	Prefix           bool
	CheckpointTokens int
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
		SessionProgressKey:      scope + ":session_progress",
	}
	if independentChain != "" {
		plan.IndependentKey = fakeCacheKey(scope+":independent", independentChain)
		plan.IndependentCacheableTokens = AccurateTokenCount(independentChain)
	}
	prefixScope := scope + ":prefix:independent:" + fakeCacheDigest(independentChain)
	if currentPrefixChain != "" {
		plan.CurrentPrefixKey = fakeCacheKey(prefixScope, currentPrefixChain)
		plan.CurrentPrefixCacheableTokens = AccurateTokenCount(currentPrefixChain)
		plan.CurrentKey = plan.CurrentPrefixKey
	}
	plan.CurrentCacheableTokens = plan.IndependentCacheableTokens + plan.CurrentPrefixCacheableTokens

	previousPrefixChain := buildFakeCachePrefixChain(rawMessages, false)
	if previousPrefixChain != "" {
		plan.PreviousPrefixKey = fakeCacheKey(prefixScope, previousPrefixChain)
		plan.PreviousPrefixCacheableTokens = AccurateTokenCount(previousPrefixChain)
		plan.PreviousKey = plan.PreviousPrefixKey
	}
	plan.PreviousCacheableTokens = plan.IndependentCacheableTokens + plan.PreviousPrefixCacheableTokens
	plan.Checkpoints = buildFakeCacheCheckpoints(scope, plan.IndependentKey, independentChain, rawMessages)

	return plan, nil
}

func (p *FakeCachePlan) CurrentCheckpointTokens() int {
	if p == nil || len(p.Checkpoints) == 0 {
		return 0
	}
	return p.Checkpoints[len(p.Checkpoints)-1].Tokens
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
	// If all keys are empty (no session ID), don't simulate cache at all
	// This prevents incorrect statistics where all input tokens are counted as cache creation
	if p.IndependentKey == "" && p.CurrentPrefixKey == "" && p.PreviousPrefixKey == "" && len(p.Checkpoints) == 0 {
		return FakeCacheUsage{InputTokens: totalInputTokens}
	}

	if len(p.Checkpoints) > 0 {
		currentTokens := eligibleFakeCacheCheckpointTokens(p.Checkpoints[len(p.Checkpoints)-1].Tokens, config.MinBlockTokens, totalInputTokens)
		cacheRead := eligibleFakeCacheCheckpointTokens(hit.CheckpointTokens, config.MinBlockTokens, totalInputTokens)
		if cacheRead > currentTokens {
			cacheRead = currentTokens
		}
		cacheWrite := currentTokens - cacheRead
		return fakeCacheUsageFromReadWrite(totalInputTokens, cacheRead, cacheWrite, config.HitRateScale)
	}

	independentCurrent := eligibleFakeCacheCheckpointTokens(p.IndependentCacheableTokens, config.MinBlockTokens, totalInputTokens)
	previousCumulative := eligibleFakeCacheCheckpointTokens(p.IndependentCacheableTokens+p.PreviousPrefixCacheableTokens, config.MinBlockTokens, totalInputTokens)
	currentCumulative := eligibleFakeCacheCheckpointTokens(p.IndependentCacheableTokens+p.CurrentPrefixCacheableTokens, config.MinBlockTokens, totalInputTokens)

	cacheRead := 0
	if hit.Prefix && previousCumulative > 0 {
		cacheRead = previousCumulative
	} else if hit.Independent && independentCurrent > 0 {
		cacheRead = independentCurrent
	}

	cacheWrite := 0
	if currentCumulative > 0 {
		cacheWrite = currentCumulative - cacheRead
	} else if independentCurrent > 0 && !hit.Independent {
		cacheWrite = independentCurrent
	}
	if cacheWrite < 0 {
		cacheWrite = 0
	}
	return fakeCacheUsageFromReadWrite(totalInputTokens, cacheRead, cacheWrite, config.HitRateScale)
}

func fakeCacheUsageFromReadWrite(totalInputTokens, cacheRead, cacheWrite, hitRateScale int) FakeCacheUsage {
	cacheRead = clampFakeCacheTokens(cacheRead, totalInputTokens)
	cacheWrite = clampFakeCacheTokens(cacheWrite, totalInputTokens-cacheRead)

	// Apply hit rate scaling only to cache reads.
	// Claude-style input_tokens is the non-cached remainder after subtracting
	// both cache_read and cache_creation.
	if hitRateScale < 100 {
		scaledRead := cacheRead * hitRateScale / 100
		if scaledRead < 0 {
			scaledRead = 0
		}
		cacheRead = scaledRead
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

func buildFakeCacheCheckpoints(scope, independentKey, independentChain string, messages []any) []FakeCacheCheckpoint {
	checkpoints := make([]FakeCacheCheckpoint, 0)
	base := strings.TrimSpace(independentChain)
	if base != "" {
		checkpoints = append(checkpoints, FakeCacheCheckpoint{
			Key:    independentKey,
			Tokens: AccurateTokenCount(base),
		})
	}

	for _, chain := range buildFakeCacheControlPrefixChains(messages) {
		cumulative := strings.TrimSpace(strings.Join([]string{base, chain}, "\n"))
		if cumulative == "" {
			continue
		}
		key := fakeCacheKey(scope+":checkpoint", cumulative)
		if len(checkpoints) > 0 && checkpoints[len(checkpoints)-1].Key == key {
			continue
		}
		checkpoints = append(checkpoints, FakeCacheCheckpoint{
			Key:    key,
			Tokens: AccurateTokenCount(cumulative),
		})
	}
	return checkpoints
}

func buildFakeCacheControlPrefixChains(messages []any) []string {
	checkpoints := make([]string, 0)
	parts := make([]string, 0, len(messages))
	for _, item := range messages {
		msg, _ := item.(map[string]any)
		role := strings.ToLower(strings.TrimSpace(stringField(msg, "role")))
		content := msg["content"]
		switch role {
		case "user":
			for _, prefix := range fakeCacheUserContentPrefixes(content) {
				checkpoints = append(checkpoints, strings.TrimSpace(strings.Join(append(append([]string{}, parts...), "user:"+prefix), "\n")))
			}
			if rendered := fakeCacheUserContent(content); rendered != "" {
				parts = append(parts, "user:"+rendered)
			}
		case "assistant":
			for _, prefix := range fakeCacheAssistantContentPrefixes(content) {
				checkpoints = append(checkpoints, strings.TrimSpace(strings.Join(append(append([]string{}, parts...), "assistant:"+prefix), "\n")))
			}
			if rendered := fakeCacheAssistantContent(content); rendered != "" {
				parts = append(parts, "assistant:"+rendered)
			}
		}
	}
	return checkpoints
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

func eligibleFakeCacheCheckpointTokens(tokens, minBlockTokens, totalInputTokens int) int {
	return clampFakeCacheTokens(applyMinBlockTokens(tokens, minBlockTokens), totalInputTokens)
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

func fakeCacheUserContentPrefixes(content any) []string {
	items, ok := content.([]any)
	if !ok {
		return nil
	}
	prefixes := make([]string, 0)
	parts := make([]string, 0, len(items))
	for _, item := range items {
		block, _ := item.(map[string]any)
		if rendered := fakeCacheUserBlock(block); rendered != "" {
			parts = append(parts, rendered)
		}
		if hasFakeCacheControl(block) {
			if prefix := strings.TrimSpace(strings.Join(parts, "\n")); prefix != "" {
				prefixes = append(prefixes, prefix)
			}
		}
	}
	return prefixes
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

func fakeCacheAssistantContentPrefixes(content any) []string {
	items, ok := content.([]any)
	if !ok {
		return nil
	}
	prefixes := make([]string, 0)
	parts := make([]string, 0, len(items))
	for _, item := range items {
		block, _ := item.(map[string]any)
		if rendered := fakeCacheAssistantBlock(block); rendered != "" {
			parts = append(parts, rendered)
		}
		if hasFakeCacheControl(block) {
			if prefix := strings.TrimSpace(strings.Join(parts, "\n")); prefix != "" {
				prefixes = append(prefixes, prefix)
			}
		}
	}
	return prefixes
}

func fakeCacheUserBlock(block map[string]any) string {
	switch strings.TrimSpace(stringField(block, "type")) {
	case "text":
		return stringField(block, "text")
	case "tool_result":
		status := "success"
		if isError, ok := block["is_error"].(bool); ok && isError {
			status = "error"
		}
		return "tool_result:" +
			"\ntool_use_id:" + stringField(block, "tool_use_id") +
			"\nstatus:" + status +
			"\ncontent:" + strings.TrimSpace(toolResultContent(block["content"]))
	case "image":
		return "image:" + fakeCacheJSON(block)
	default:
		return ""
	}
}

func fakeCacheAssistantBlock(block map[string]any) string {
	switch strings.TrimSpace(stringField(block, "type")) {
	case "text":
		return stringField(block, "text")
	case "tool_use":
		return "tool_use:" +
			"\nid:" + stringField(block, "id") +
			"\nname:" + stringField(block, "name") +
			"\ninput:" + fakeCacheJSON(jsonValue(block["input"]))
	default:
		return ""
	}
}

func hasFakeCacheControl(block map[string]any) bool {
	if block == nil {
		return false
	}
	_, ok := block["cache_control"]
	return ok
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
