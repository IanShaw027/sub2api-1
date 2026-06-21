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
	// RecordedEffectiveCachedTokens is the cacheable weight recorded by the most
	// recent ResolveUsageWithConfig call. commitFakeCachePlan persists it to
	// SessionProgress so the next turn can cap cache reads to the cacheable span
	// that was visible at the end of this turn.
	RecordedEffectiveCachedTokens int
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
	// EffectiveCachedTokens is the cacheable weight carried over from the
	// previous turn (persisted under SessionProgressKey). It caps the ideal cache
	// read basis so compacted or changed histories cannot read beyond the
	// cacheable span that existed at the end of the previous turn.
	EffectiveCachedTokens int
}

type FakeCacheUsageConfig struct {
	HitRateScale   int
	MinBlockTokens int
}

type FakeCacheScope struct {
	AccountID int64
	UserID    int64
	APIKeyID  int64
}

func (s FakeCacheScope) keyScope(requestedModel, sessionID string) string {
	model := strings.ToLower(strings.TrimSpace(requestedModel))
	if s.UserID > 0 && s.APIKeyID > 0 {
		return fmt.Sprintf(
			"kiro:fakecache:v2:user:%d:key:%d:model:%s:session:%s",
			s.UserID,
			s.APIKeyID,
			model,
			sessionID,
		)
	}
	return fmt.Sprintf(
		"kiro:fakecache:v1:acct:%d:model:%s:session:%s",
		s.AccountID,
		model,
		sessionID,
	)
}

func BuildFakeCachePlan(body []byte, scope FakeCacheScope, requestedModel string) (*FakeCachePlan, error) {
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

	scopeKey := scope.keyScope(requestedModel, sessionID)

	plan := &FakeCachePlan{
		PreviousCacheableTokens: 0,
		SessionProgressKey:      scopeKey + ":session_progress",
	}
	if independentChain != "" {
		plan.IndependentKey = fakeCacheKey(scopeKey+":independent", independentChain)
		plan.IndependentCacheableTokens = fakeCacheIndependentCalibratedTokens(req)
	}
	prefixScope := scopeKey + ":prefix:independent:" + fakeCacheDigest(independentChain)
	if currentPrefixChain != "" {
		plan.CurrentPrefixKey = fakeCacheKey(prefixScope, currentPrefixChain)
		plan.CurrentPrefixCacheableTokens = fakeCacheContentCalibratedTokens(rawMessages)
		plan.CurrentKey = plan.CurrentPrefixKey
	}
	plan.CurrentCacheableTokens = plan.IndependentCacheableTokens + plan.CurrentPrefixCacheableTokens

	previousPrefixChain := buildFakeCachePrefixChain(rawMessages, false)
	if previousPrefixChain != "" {
		plan.PreviousPrefixKey = fakeCacheKey(prefixScope, previousPrefixChain)
		plan.PreviousPrefixCacheableTokens = fakeCacheContentCalibratedTokens(rawMessages[:len(rawMessages)-1])
		plan.PreviousKey = plan.PreviousPrefixKey
	}
	plan.PreviousCacheableTokens = plan.IndependentCacheableTokens + plan.PreviousPrefixCacheableTokens
	plan.Checkpoints = buildFakeCacheCheckpoints(scopeKey, plan.IndependentKey, independentChain, plan.IndependentCacheableTokens, rawMessages)

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

	prefixCurrentTokens, prefixReadTokens := p.resolvePrefixUsageBounds(totalInputTokens, hit, config.MinBlockTokens)

	currentTokens := prefixCurrentTokens
	idealRead := prefixReadTokens
	if len(p.Checkpoints) > 0 {
		currentTokens = eligibleFakeCacheCheckpointTokens(p.Checkpoints[len(p.Checkpoints)-1].Tokens, config.MinBlockTokens, totalInputTokens)
		idealRead = eligibleFakeCacheCheckpointTokens(hit.CheckpointTokens, config.MinBlockTokens, totalInputTokens)
		if prefixCurrentTokens > currentTokens {
			currentTokens = prefixCurrentTokens
		}
		if prefixReadTokens > idealRead {
			idealRead = prefixReadTokens
		}
		if idealRead > currentTokens {
			idealRead = currentTokens
		}
	}

	return p.resolveFakeCacheUsage(totalInputTokens, currentTokens, idealRead, hit.EffectiveCachedTokens, config.HitRateScale)
}

// resolveFakeCacheUsage splits the request into Anthropic-style non-cached
// input, cache read, and cache creation:
//
//   - currentTokens is the cacheable prefix visible this turn. It is the upper
//     bound for cache_read + cache_creation.
//   - idealRead is the cacheable prefix known to have existed before this turn,
//     capped by the previous session progress when available.
//   - hitRateScale reduces only the reported cache_read portion. The scaled-away
//     read stays inside the cacheable prefix and is reported as cache_creation,
//     so the non-cacheable input tail remains stable.
//   - inputTokens is totalInputTokens - currentTokens, matching Anthropic's
//     "tokens after the last cache breakpoint" usage shape.
//   - RecordedEffectiveCachedTokens stores currentTokens, not scaled read+write,
//     because the scale is a reporting calibration knob rather than a real cache
//     capacity limiter.
func (p *FakeCachePlan) resolveFakeCacheUsage(totalInputTokens, currentTokens, idealRead, effectiveCap, hitRateScale int) FakeCacheUsage {
	read := idealRead
	if effectiveCap > 0 {
		if read == 0 || effectiveCap < read {
			read = effectiveCap
		}
	}
	if read > currentTokens {
		read = currentTokens
	}
	read = clampFakeCacheTokens(read, totalInputTokens)
	currentTokens = clampFakeCacheTokens(currentTokens, totalInputTokens)
	scaledRead := read
	if hitRateScale < 100 {
		scaledRead = read * hitRateScale / 100
	}
	if scaledRead < 0 {
		scaledRead = 0
	}
	if scaledRead > currentTokens {
		scaledRead = currentTokens
	}
	created := currentTokens - scaledRead
	if created < 0 {
		created = 0
	}
	inputTokens := totalInputTokens - currentTokens
	if inputTokens < 0 {
		inputTokens = 0
	}

	if p != nil {
		p.RecordedEffectiveCachedTokens = currentTokens
	}

	return FakeCacheUsage{
		InputTokens:              inputTokens,
		CacheCreationInputTokens: created,
		CacheReadInputTokens:     scaledRead,
	}
}

func (p *FakeCachePlan) resolvePrefixUsageBounds(totalInputTokens int, hit FakeCacheHitState, minBlockTokens int) (currentTokens, cacheRead int) {
	if p == nil {
		return 0, 0
	}

	independentCurrent := eligibleFakeCacheCheckpointTokens(p.IndependentCacheableTokens, minBlockTokens, totalInputTokens)
	previousCumulative := eligibleFakeCacheCheckpointTokens(p.IndependentCacheableTokens+p.PreviousPrefixCacheableTokens, minBlockTokens, totalInputTokens)
	currentCumulative := eligibleFakeCacheCheckpointTokens(p.IndependentCacheableTokens+p.CurrentPrefixCacheableTokens, minBlockTokens, totalInputTokens)

	currentTokens = currentCumulative
	if currentTokens == 0 {
		currentTokens = independentCurrent
	}

	if hit.Prefix && previousCumulative > 0 {
		cacheRead = previousCumulative
	} else if hit.Independent && independentCurrent > 0 {
		cacheRead = independentCurrent
	}
	if cacheRead > currentTokens {
		cacheRead = currentTokens
	}
	return currentTokens, cacheRead
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

func buildFakeCacheCheckpoints(scope, independentKey, independentChain string, independentTokens int, messages []any) []FakeCacheCheckpoint {
	checkpoints := make([]FakeCacheCheckpoint, 0)
	base := strings.TrimSpace(independentChain)
	if base != "" {
		checkpoints = append(checkpoints, FakeCacheCheckpoint{
			Key:    independentKey,
			Tokens: independentTokens,
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
			Tokens: independentTokens + AccurateTokenCount(chain)*kiroTokenEstimateContentScale100/100,
		})
	}
	return checkpoints
}

// fakeCacheIndependentCalibratedTokens estimates the calibrated token weight of
// the cacheable independent block (system prompt + tool definitions). It mirrors
// the system/tool terms of calibrateKiroInputTokens so a cache hit offsets the
// same scaled tokens EstimateInputTokens bills; the raw tiktoken count used
// before left a content-proportional residual (plus the 493-token tool system
// scaffold) permanently in input_tokens.
func fakeCacheIndependentCalibratedTokens(req map[string]any) int {
	var systemBuilder strings.Builder
	if systemText := joinSystem(req["system"]); systemText != "" {
		appendKiroTokenEstimateText(&systemBuilder, systemText)
	}
	systemTokens := AccurateTokenCount(systemBuilder.String())

	toolCount := 0
	toolSchemaTokens := 0
	if tools, _ := req["tools"].([]any); len(tools) > 0 {
		toolCount = len(tools)
		var toolBuilder strings.Builder
		for _, item := range tools {
			tool, _ := item.(map[string]any)
			appendKiroTokenEstimateText(&toolBuilder, stringField(tool, "name"))
			appendKiroTokenEstimateText(&toolBuilder, stringField(tool, "description"))
			appendKiroTokenEstimateJSON(&toolBuilder, tool["input_schema"])
		}
		toolSchemaTokens = AccurateTokenCount(toolBuilder.String())
	}

	total := systemTokens * kiroTokenEstimateContentScale100 / 100
	if toolCount > 0 {
		total += kiroTokenEstimateToolSystemBase
		total += toolCount * kiroTokenEstimatePerTool
		total += toolSchemaTokens * kiroTokenEstimateToolSchemaScale / 100
	}
	return total
}

// fakeCacheContentCalibratedTokens estimates the calibrated token weight of a
// span of conversation messages (the cacheable prefix). It mirrors the content
// scaling, per-message and per-tool-block overhead of calibrateKiroInputTokens
// so a prefix cache hit offsets the billed tokens instead of the raw count.
func fakeCacheContentCalibratedTokens(messages []any) int {
	if len(messages) == 0 {
		return 0
	}
	var contentBuilder strings.Builder
	toolBlockCount := 0
	for _, item := range messages {
		msg, _ := item.(map[string]any)
		appendKiroTokenEstimateContent(&contentBuilder, msg["content"])
		toolBlockCount += countKiroToolBlocks(msg["content"])
	}
	contentTokens := AccurateTokenCount(contentBuilder.String())
	return contentTokens*kiroTokenEstimateContentScale100/100 +
		len(messages)*kiroTokenEstimatePerMessage +
		toolBlockCount*kiroTokenEstimatePerToolBlock
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
