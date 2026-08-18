package service

import (
	"bytes"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
)

// shouldSanitizeOpenAIResponsesToolSchemaPatterns reports whether lookaround
// pattern stripping should run for this account. OpenAI's schema validator
// rejects lookahead/lookbehind; Grok, native-Anthropic, and CN-vendor paths
// do not, so they must keep the original constraint.
func shouldSanitizeOpenAIResponsesToolSchemaPatterns(account *Account) bool {
	return account != nil && account.Platform == PlatformOpenAI
}

// openAIResponsesToolSchemaPattern 记录一处待删除的 lookaround pattern 成员，
// 区间覆盖原始 body 上的整段 `"pattern":"..."`（含配对逗号），便于一次性拼接。
type openAIResponsesToolSchemaPattern struct {
	offset int
	length int
}

const (
	// JSON Schema 可在 properties / items / anyOf 下多层嵌套；比 tools 嵌套更深，
	// 截断即可，避免畸形请求体把递归打满栈。
	openAIResponsesToolSchemaPatternMaxDepth = 16
	openAIResponsesToolSchemaPatternKey      = `"pattern"`
)

// sanitizeOpenAIResponsesToolSchemaPatterns removes JSON Schema patterns that
// use PCRE lookaround. OpenAI's schema validator rejects lookahead/lookbehind
// even though the client-side regex engine accepts them; dropping only the
// incompatible constraint preserves the rest of the tool contract.
//
// 只改命中的成员区间，不 json.Marshal 整份 body：/v1/responses 上限默认 256MB，
// 全量 decode+encode 会重排键并 HTML-escape（< → \u003c），破坏 wire mimic。
func sanitizeOpenAIResponsesToolSchemaPatterns(body []byte) ([]byte, bool, error) {
	if len(body) == 0 || !bytes.Contains(body, []byte(openAIResponsesToolSchemaPatternKey)) {
		return body, false, nil
	}

	hits := make([]openAIResponsesToolSchemaPattern, 0, 2)
	collectOpenAIResponsesToolSchemaPatternsFromTools(body, gjson.GetBytes(body, "tools"), 0, &hits)
	if input := gjson.GetBytes(body, "input"); input.IsArray() {
		input.ForEach(func(_, item gjson.Result) bool {
			if item.IsObject() {
				collectOpenAIResponsesToolSchemaPatternsFromTools(body, item.Get("tools"), 0, &hits)
			}
			return true
		})
	}
	if len(hits) == 0 {
		return body, false, nil
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].offset < hits[j].offset })

	sanitized := make([]byte, 0, len(body))
	cursor := 0
	for _, hit := range hits {
		if hit.offset < cursor {
			continue
		}
		sanitized = append(sanitized, body[cursor:hit.offset]...)
		cursor = hit.offset + hit.length
	}
	sanitized = append(sanitized, body[cursor:]...)
	return sanitized, true, nil
}

func collectOpenAIResponsesToolSchemaPatternsFromTools(
	body []byte, tools gjson.Result, depth int, hits *[]openAIResponsesToolSchemaPattern,
) {
	if depth > openAIResponsesToolSchemaMaxDepth || !tools.IsArray() {
		return
	}
	tools.ForEach(func(_, tool gjson.Result) bool {
		if !tool.IsObject() {
			return true
		}
		for _, suffix := range []string{"parameters", "function.parameters"} {
			params := tool.Get(suffix)
			if params.IsObject() {
				collectOpenAIResponsesToolSchemaLookaroundPatterns(body, params, 0, hits)
			}
		}
		collectOpenAIResponsesToolSchemaPatternsFromTools(body, tool.Get("tools"), depth+1, hits)
		return true
	})
}

func collectOpenAIResponsesToolSchemaLookaroundPatterns(
	body []byte, schema gjson.Result, depth int, hits *[]openAIResponsesToolSchemaPattern,
) {
	if depth > openAIResponsesToolSchemaPatternMaxDepth {
		return
	}
	if schema.IsObject() {
		schema.ForEach(func(key, value gjson.Result) bool {
			if key.String() == "pattern" && value.Type == gjson.String && hasRegexLookaround(value.String()) {
				appendOpenAIResponsesToolSchemaPattern(body, value, hits)
			}
			if value.IsObject() || value.IsArray() {
				collectOpenAIResponsesToolSchemaLookaroundPatterns(body, value, depth+1, hits)
			}
			return true
		})
		return
	}
	if schema.IsArray() {
		schema.ForEach(func(_, value gjson.Result) bool {
			if value.IsObject() || value.IsArray() {
				collectOpenAIResponsesToolSchemaLookaroundPatterns(body, value, depth+1, hits)
			}
			return true
		})
	}
}

// appendOpenAIResponsesToolSchemaPattern 把 gjson 给出的 pattern 值扩成整段对象
// 成员（含配对逗号）。Index 不可用或对不上原文时跳过，避免拼错位置毁掉请求体。
func appendOpenAIResponsesToolSchemaPattern(
	body []byte, value gjson.Result, hits *[]openAIResponsesToolSchemaPattern,
) {
	end := value.Index + len(value.Raw)
	if value.Index <= 0 || end > len(body) {
		return
	}
	if !bytes.Equal(body[value.Index:end], []byte(value.Raw)) {
		return
	}
	start, length, ok := jsonObjectMemberSpan(body, value.Index, end)
	if !ok {
		return
	}
	*hits = append(*hits, openAIResponsesToolSchemaPattern{offset: start, length: length})
}

func jsonObjectMemberSpan(body []byte, valueStart, valueEnd int) (int, int, bool) {
	i := valueStart - 1
	for i >= 0 && isJSONSpace(body[i]) {
		i--
	}
	if i < 0 || body[i] != ':' {
		return 0, 0, false
	}
	i--
	for i >= 0 && isJSONSpace(body[i]) {
		i--
	}
	keyLen := len(openAIResponsesToolSchemaPatternKey)
	keyStart := i - keyLen + 1
	if keyStart < 0 || !bytes.Equal(body[keyStart:i+1], []byte(openAIResponsesToolSchemaPatternKey)) {
		return 0, 0, false
	}

	j := valueEnd
	for j < len(body) && isJSONSpace(body[j]) {
		j++
	}
	if j < len(body) && body[j] == ',' {
		return keyStart, j + 1 - keyStart, true
	}
	k := keyStart - 1
	for k >= 0 && isJSONSpace(body[k]) {
		k--
	}
	if k >= 0 && body[k] == ',' {
		return k, valueEnd - k, true
	}
	return keyStart, valueEnd - keyStart, true
}

func isJSONSpace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}

func hasRegexLookaround(pattern string) bool {
	return strings.Contains(pattern, "(?=") ||
		strings.Contains(pattern, "(?!") ||
		strings.Contains(pattern, "(?<=") ||
		strings.Contains(pattern, "(?<!")
}

const (
	// 工具定义在多轮历史里最多再嵌套一层 tools，留出余量后截断，避免畸形请求体
	// 造成无界递归。
	openAIResponsesToolSchemaMaxDepth = 4
	// JSON Schema 里 type 只能是字符串或字符串数组；显式 null 无论哪个方言都非法，
	// 补成 object 与 upstream 对该工具的实际期望一致。
	openAIResponsesToolSchemaFallbackType = `"object"`
	// 显式 null 在 JSON 里只有这一种字面量形态。
	openAIResponsesToolSchemaNullLiteral = "null"
)

// openAIResponsesToolSchemaNullType 记录一处待修正的 null，用原始 body 上的
// 绝对字节偏移表示，便于最后一次性拼接。
type openAIResponsesToolSchemaNullType struct {
	offset int
	length int
}

// sanitizeOpenAIResponsesToolParameterTypes 修正请求体中显式为 null 的
// tools[].parameters.type。
//
// Codex Desktop 内置的 automation_update 工具会带 parameters.type = null，
// OpenAI 直接回 400 invalid_function_parameters，而网关把该状态归一成可重试的
// 502 upstream_error；该工具定义又会沉进多轮历史，导致之后每一轮继续失败并在
// 账号池里反复重放同一份坏 Schema。
//
// 只修正显式 null：缺失 type 的 Schema 本身合法（等价于不约束），补写会收窄
// 客户端语义，因此保持原样。
//
// 实现上先收集全部命中的绝对偏移，再一次性拼出新 body：逐个 sjson.SetBytes 每次
// 都会重扫并全量拷贝整个文档，命中 N 处就是 N 次全量拷贝，而 /v1/responses 的
// body 上限是 gateway.max_body_size（默认 256MB），构造请求能塞进百万级命中。
func sanitizeOpenAIResponsesToolParameterTypes(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}

	hits := make([]openAIResponsesToolSchemaNullType, 0, 2)
	collectOpenAIResponsesToolSchemaNullTypes(body, gjson.GetBytes(body, "tools"), 0, &hits)
	if input := gjson.GetBytes(body, "input"); input.IsArray() {
		input.ForEach(func(_, item gjson.Result) bool {
			if item.IsObject() {
				collectOpenAIResponsesToolSchemaNullTypes(body, item.Get("tools"), 0, &hits)
			}
			return true
		})
	}
	if len(hits) == 0 {
		return body, false, nil
	}

	// tools 与 input 在 body 里的先后顺序由客户端决定，收集顺序不保证单调。
	sort.Slice(hits, func(i, j int) bool { return hits[i].offset < hits[j].offset })

	sanitized := make([]byte, 0, len(body)+len(hits)*len(openAIResponsesToolSchemaFallbackType))
	cursor := 0
	for _, hit := range hits {
		// 收集阶段已逐个校验过区间，这里再挡一次重叠，保证拼接严格单调向前。
		if hit.offset < cursor {
			continue
		}
		sanitized = append(sanitized, body[cursor:hit.offset]...)
		sanitized = append(sanitized, openAIResponsesToolSchemaFallbackType...)
		cursor = hit.offset + hit.length
	}
	sanitized = append(sanitized, body[cursor:]...)
	return sanitized, true, nil
}

// collectOpenAIResponsesToolSchemaNullTypes 收集一个 tools 数组里所有需要修正的
// parameters.type 位置。不按 tool type 过滤：null 的 schema type 在 function、
// custom 以及任何 hosted 工具上都同样非法。
func collectOpenAIResponsesToolSchemaNullTypes(
	body []byte, tools gjson.Result, depth int, hits *[]openAIResponsesToolSchemaNullType,
) {
	if depth > openAIResponsesToolSchemaMaxDepth || !tools.IsArray() {
		return
	}
	tools.ForEach(func(_, tool gjson.Result) bool {
		if !tool.IsObject() {
			return true
		}
		// Responses 形态用顶层 parameters，ChatCompletions 形态用 function.parameters，
		// 两种都可能出现在 Responses 请求里（见 normalizeCodexTools）。
		for _, suffix := range []string{"parameters", "function.parameters"} {
			params := tool.Get(suffix)
			if !params.IsObject() {
				continue
			}
			// gjson 用 Type==Null 同时表示「显式 null」和「路径不存在」，靠 Raw
			// 区分：不存在时 Raw 为空串。
			if typ := params.Get("type"); typ.Type == gjson.Null && typ.Raw == openAIResponsesToolSchemaNullLiteral {
				appendOpenAIResponsesToolSchemaNullType(body, typ, hits)
			}
		}
		// 历史输入里的工具定义会再嵌套一层 tools（upstream 报错路径形如
		// input[234].tools[0].tools[3].parameters）。
		collectOpenAIResponsesToolSchemaNullTypes(body, tool.Get("tools"), depth+1, hits)
		return true
	})
}

// appendOpenAIResponsesToolSchemaNullType 先校验 gjson 给出的偏移确实指向原始
// body 上那段 null，再记录。gjson 对嵌套取值同样返回相对原始文档的绝对偏移，但
// Index 为 0 表示未知；偏移不可用时跳过该处而不是猜位置——少修一个工具只是维持
// 现状，拼错位置会损坏整个请求体。
func appendOpenAIResponsesToolSchemaNullType(
	body []byte, typ gjson.Result, hits *[]openAIResponsesToolSchemaNullType,
) {
	end := typ.Index + len(typ.Raw)
	if typ.Index <= 0 || end > len(body) {
		return
	}
	if !bytes.Equal(body[typ.Index:end], []byte(typ.Raw)) {
		return
	}
	*hits = append(*hits, openAIResponsesToolSchemaNullType{offset: typ.Index, length: len(typ.Raw)})
}
