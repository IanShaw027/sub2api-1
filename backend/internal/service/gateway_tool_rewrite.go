package service

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// toolNameRewriteKey 是 gin.Context 上存 ToolNameRewrite 映射的 key。
// 请求阶段写入，响应阶段读取，用于 bytes 级逆向还原假名 → 真名。
const toolNameRewriteKey = "claude_tool_name_rewrite"

// staticToolNameRewrites 是"静态前缀映射"，与 Parrot src/transform/cc_mimicry.py
// TOOL_NAME_REWRITES 完全一致。只有以这些前缀开头的工具会被重写。
var staticToolNameRewrites = map[string]string{
	"sessions_": "cc_sess_",
	"session_":  "cc_ses_",
}

// fakeToolNamePrefixes 是"动态映射"的前缀池，与 Parrot _FAKE_PREFIXES 一致。
// 当 tools 数量 > dynamicToolMapThreshold 时随机选用其中前缀生成可读假名。
var fakeToolNamePrefixes = []string{
	"analyze_", "compute_", "fetch_", "generate_", "lookup_", "modify_",
	"process_", "query_", "render_", "resolve_", "sync_", "update_",
	"validate_", "convert_", "extract_", "manage_", "monitor_", "parse_",
	"review_", "search_", "transform_", "handle_", "invoke_", "notify_",
}

// dynamicToolMapThreshold 与 Parrot 一致：tools 数量超过 5 才启用动态映射。
// 少量工具不需要混淆（一般是 Claude Code 自己的核心工具 bash/edit/read 等）。
const dynamicToolMapThreshold = 5

// ToolNameRewrite 是单次请求内的工具名混淆映射。
//   - Forward: real → fake，请求阶段在 body 上应用。
//   - Reverse: fake → real，响应阶段对每个 chunk 做 bytes.Replace 还原。
//
// ReverseOrdered 是按假名长度倒序的 (fake, real) 列表，仅包含工具名对，
// 用于防止短假名是长假名的子串时 bytes.Replace 先被吃掉。
//
// PropForward/PropReverse 存储 schema 属性名重命名映射（real → fake / fake → real）。
// 属性反向映射通过独立的 restorePropNamesInJSON 做 JSON-key-aware 替换，
// 不走 ReverseOrdered 的 byte-level 路径，避免误替换 model 文本中的通用标识符。
//
// DescStripped 标记是否对工具描述做了剥离（用于日志/调试）。
type ToolNameRewrite struct {
	Forward        map[string]string
	Reverse        map[string]string
	ReverseOrdered [][2]string
	PropForward    map[string]string
	PropReverse    map[string]string
	DescStripped   bool
}

// buildDynamicToolMap 构造 tools 的动态假名映射。
//
// 与 Parrot _build_dynamic_tool_map 语义等价：
//   - tools 数量 ≤ dynamicToolMapThreshold 时返回 nil（不做动态映射，走静态 fallback）
//   - 同一组 tool_names 在同进程内映射稳定（保证 cache 命中）
//
// Parrot 用 `random.Random(hash(tuple(tool_names)))` 作 seed + shuffle 前缀池；
// Go 无法字节级复刻 Python hash，但"稳定性"和"前缀池打散"两个不变量都保留：
// 用 fnv64a(strings.Join(names, "\x00")) 作 seed 喂 math/rand.New。
// 字节级不同不影响上游判定（Anthropic 不会验证我们的随机种子算法）。
func buildDynamicToolMap(toolNames []string) map[string]string {
	if len(toolNames) <= dynamicToolMapThreshold {
		return nil
	}
	h := fnv.New64a()
	for i, n := range toolNames {
		if i > 0 {
			_, _ = h.Write([]byte{0})
		}
		_, _ = h.Write([]byte(n))
	}
	rng := rand.New(rand.NewSource(int64(h.Sum64())))

	available := make([]string, len(fakeToolNamePrefixes))
	copy(available, fakeToolNamePrefixes)
	rng.Shuffle(len(available), func(i, j int) { available[i], available[j] = available[j], available[i] })

	mapping := make(map[string]string, len(toolNames))
	for i, name := range toolNames {
		prefix := available[i%len(available)]
		headLen := 3
		if len(name) < 3 {
			headLen = len(name)
		}
		fake := fmt.Sprintf("%s%s%02d", prefix, name[:headLen], i)
		mapping[name] = fake
	}
	return mapping
}

// sanitizeToolName 把真名转成假名。
// 与 Parrot _sanitize_tool_name 语义一致：动态映射优先，再走静态前缀映射。
func sanitizeToolName(name string, dynamic map[string]string) string {
	if dynamic != nil {
		if fake, ok := dynamic[name]; ok {
			return fake
		}
	}
	for prefix, replacement := range staticToolNameRewrites {
		if strings.HasPrefix(name, prefix) {
			return replacement + name[len(prefix):]
		}
	}
	return name
}

// shouldMimicToolName 指示某个 tool 是否需要重命名。
// server tool（type != "" 且不是 "function" / "custom"）是 Anthropic 协议语义的一部分，
// 比如 "web_search_20250305" / "computer_20250124"；误改会导致上游拒绝。
func shouldMimicToolName(toolType string) bool {
	if toolType == "" || toolType == "function" || toolType == "custom" {
		return true
	}
	return false
}

func shouldSkipShadowToolName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), kiropkg.ShadowToolPrefix)
}

// buildToolNameRewriteFromBody 扫描 body 的 tools[*].name，构造 ToolNameRewrite
// 并返回它。若不需要混淆（tools 数量不足 + 没有匹配静态前缀的工具）且无 schema prop
// 重命名，返回 nil。
//
// schemaPropRewrites 为 schema 属性名→替换名映射，nil/empty 时跳过属性重命名。
// 只有在 tools 中实际出现匹配的 properties key 时才记录。
//
// 注意：只扫描，不改 body。真正的 body 改写在 applyToolNameRewriteToBody。
func buildToolNameRewriteFromBody(body []byte, schemaPropRewrites map[string]string) *ToolNameRewrite {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return nil
	}

	mimicableNames := make([]string, 0)
	toolsArr := tools.Array()
	for _, t := range toolsArr {
		if !shouldMimicToolName(t.Get("type").String()) {
			continue
		}
		name := t.Get("name").String()
		if name == "" || shouldSkipShadowToolName(name) {
			continue
		}
		mimicableNames = append(mimicableNames, name)
	}

	dynamic := buildDynamicToolMap(mimicableNames)

	rw := &ToolNameRewrite{
		Forward: make(map[string]string),
		Reverse: make(map[string]string),
	}
	for _, name := range mimicableNames {
		fake := sanitizeToolName(name, dynamic)
		if fake == name {
			continue
		}
		rw.Forward[name] = fake
		rw.Reverse[fake] = name
	}

	// Scan schema properties for rewriting
	if len(schemaPropRewrites) > 0 {
		propFwd, propRev := scanSchemaProperties(body, schemaPropRewrites)
		if len(propFwd) > 0 {
			rw.PropForward = propFwd
			rw.PropReverse = propRev
		}
	}

	if len(rw.Forward) == 0 && len(rw.PropForward) == 0 {
		return nil
	}

	rw.ReverseOrdered = make([][2]string, 0, len(rw.Reverse))
	for fake, real := range rw.Reverse {
		rw.ReverseOrdered = append(rw.ReverseOrdered, [2]string{fake, real})
	}
	sort.SliceStable(rw.ReverseOrdered, func(i, j int) bool {
		return len(rw.ReverseOrdered[i][0]) > len(rw.ReverseOrdered[j][0])
	})

	return rw
}

// applyToolNameRewriteToBody 把已构造的 ToolNameRewrite 应用到 body 上：
//
//   - 改写 $.tools[*].name（仅对 shouldMimicToolName 通过的 tool）
//   - 改写 $.tool_choice.name（仅当 $.tool_choice.type == "tool"）
//   - 改写 $.messages[*].content[*].name（仅当 type == "tool_use"）
//   - 在 $.tools[last].cache_control 上打 ephemeral 缓存断点
//
// 响应侧 bytes.Replace 会连带还原假名 → 真名。
func applyToolNameRewriteToBody(body []byte, rw *ToolNameRewrite) []byte {
	// Apply schema property renames if configured
	if rw != nil && len(rw.PropForward) > 0 {
		body = rewriteSchemaProperties(body, rw.PropForward)
	}

	if rw == nil || len(rw.Forward) == 0 {
		body = applyToolsLastCacheBreakpoint(body)
		return body
	}

	tools := gjson.GetBytes(body, "tools")
	if tools.IsArray() {
		idx := -1
		tools.ForEach(func(_, t gjson.Result) bool {
			idx++
			if !shouldMimicToolName(t.Get("type").String()) {
				return true
			}
			name := t.Get("name").String()
			if name == "" || shouldSkipShadowToolName(name) {
				return true
			}
			// Tool name rewrite
			fake, ok := rw.Forward[name]
			if !ok {
				return true
			}
			if next, err := sjson.SetBytes(body, fmt.Sprintf("tools.%d.name", idx), fake); err == nil {
				body = next
			}
			if t.Get("description").Exists() {
				if next, err := sjson.SetBytes(body, fmt.Sprintf("tools.%d.description", idx), ""); err == nil {
					body = next
				}
			}
			return true
		})
	}

	if tc := gjson.GetBytes(body, "tool_choice"); tc.Exists() && tc.Get("type").String() == "tool" {
		name := tc.Get("name").String()
		if shouldSkipShadowToolName(name) {
			// Stable internal shadow tools must survive request-side rewrite untouched.
		} else if fake, ok := rw.Forward[name]; ok {
			if next, err := sjson.SetBytes(body, "tool_choice.name", fake); err == nil {
				body = next
			}
		}
	}

	// 同步改写历史消息中的 tool_use.name，确保它和 tools[] 中的假名一致。
	// 否则 Anthropic 会因为 tool_use 引用了未声明的原始工具名而拒绝请求。
	messages := gjson.GetBytes(body, "messages")
	if messages.IsArray() {
		messages.ForEach(func(msgKey, msg gjson.Result) bool {
			msgIdx := int(msgKey.Num)
			content := msg.Get("content")
			if !content.IsArray() {
				return true
			}
			content.ForEach(func(blkKey, blk gjson.Result) bool {
				blkIdx := int(blkKey.Num)
				if blk.Get("type").String() != "tool_use" {
					return true
				}
				name := blk.Get("name").String()
				if name == "" || shouldSkipShadowToolName(name) {
					return true
				}
				if fake, ok := rw.Forward[name]; ok {
					path := fmt.Sprintf("messages.%d.content.%d.name", msgIdx, blkIdx)
					if next, err := sjson.SetBytes(body, path, fake); err == nil {
						body = next
					}
				}
				return true
			})
			return true
		})
	}

	body = applyToolsLastCacheBreakpoint(body)
	return body
}

// scanSchemaProperties scans tools[*].input_schema.properties for keys matching
// schemaPropRewrites and returns forward (real→fake) and reverse (fake→real) maps.
// Only properties that actually exist in at least one tool are included.
func scanSchemaProperties(body []byte, rewrites map[string]string) (forward, reverse map[string]string) {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return nil, nil
	}
	forward = make(map[string]string)
	reverse = make(map[string]string)
	tools.ForEach(func(_, t gjson.Result) bool {
		if !shouldMimicToolName(t.Get("type").String()) {
			return true
		}
		props := t.Get("input_schema.properties")
		if !props.IsObject() {
			return true
		}
		props.ForEach(func(key, _ gjson.Result) bool {
			propName := key.String()
			if replacement, ok := rewrites[propName]; ok && replacement != propName {
				forward[propName] = replacement
				reverse[replacement] = propName
			}
			return true
		})
		return true
	})
	if len(forward) == 0 {
		return nil, nil
	}
	return forward, reverse
}

// rewriteSchemaProperties renames property keys in tools[*].input_schema.properties
// and synchronizes the required[] array for each tool.
// Also renames matching keys in messages[*].content[*].input (tool_use blocks' input objects).
func rewriteSchemaProperties(body []byte, propForward map[string]string) []byte {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return body
	}
	idx := -1
	tools.ForEach(func(_, t gjson.Result) bool {
		idx++
		if !shouldMimicToolName(t.Get("type").String()) {
			return true
		}
		props := t.Get("input_schema.properties")
		if !props.IsObject() {
			return true
		}
		propsPath := fmt.Sprintf("tools.%d.input_schema.properties", idx)
		reqPath := fmt.Sprintf("tools.%d.input_schema.required", idx)
		type propRename struct {
			oldName string
			newName string
			raw     string
		}
		renames := make([]propRename, 0)
		props.ForEach(func(key, val gjson.Result) bool {
			propName := key.String()
			newName, ok := propForward[propName]
			if !ok {
				return true
			}
			renames = append(renames, propRename{oldName: propName, newName: newName, raw: val.Raw})
			return true
		})
		for _, rename := range renames {
			// Copy value to new key after the gjson iterator has finished reading the old body.
			if next, err := sjson.SetRawBytes(body, propsPath+"."+rename.newName, []byte(rename.raw)); err == nil {
				body = next
			}
			// Delete old key after setting the replacement.
			if next, err := sjson.DeleteBytes(body, propsPath+"."+rename.oldName); err == nil {
				body = next
			}
		}
		// Sync required[] array
		reqArr := gjson.GetBytes(body, reqPath)
		if reqArr.IsArray() {
			newReqs := make([]string, 0)
			reqArr.ForEach(func(_, v gjson.Result) bool {
				name := v.String()
				if newName, ok := propForward[name]; ok {
					newReqs = append(newReqs, newName)
				} else {
					newReqs = append(newReqs, name)
				}
				return true
			})
			if next, err := sjson.SetBytes(body, reqPath, newReqs); err == nil {
				body = next
			}
		}
		return true
	})
	// Also rename matching property keys in historical tool_use input objects
	body = rewriteToolUseInputProperties(body, propForward)
	return body
}

// rewriteToolUseInputProperties renames property keys in messages[*].content[*].input
// where content type is "tool_use", keeping the tool_use input consistent with the
// renamed schema properties.
func rewriteToolUseInputProperties(body []byte, propForward map[string]string) []byte {
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return body
	}
	messages.ForEach(func(msgKey, msg gjson.Result) bool {
		msgIdx := int(msgKey.Num)
		content := msg.Get("content")
		if !content.IsArray() {
			return true
		}
		content.ForEach(func(blkKey, blk gjson.Result) bool {
			blkIdx := int(blkKey.Num)
			if blk.Get("type").String() != "tool_use" {
				return true
			}
			input := blk.Get("input")
			if !input.IsObject() {
				return true
			}
			inputPath := fmt.Sprintf("messages.%d.content.%d.input", msgIdx, blkIdx)
			type inputRename struct {
				oldName string
				newName string
				raw     string
			}
			renames := make([]inputRename, 0)
			input.ForEach(func(key, val gjson.Result) bool {
				propName := key.String()
				newName, ok := propForward[propName]
				if !ok {
					return true
				}
				renames = append(renames, inputRename{oldName: propName, newName: newName, raw: val.Raw})
				return true
			})
			for _, rename := range renames {
				if next, err := sjson.SetRawBytes(body, inputPath+"."+rename.newName, []byte(rename.raw)); err == nil {
					body = next
				}
				if next, err := sjson.DeleteBytes(body, inputPath+"."+rename.oldName); err == nil {
					body = next
				}
			}
			return true
		})
		return true
	})
	return body
}

// applyToolsLastCacheBreakpoint 在 tools 数组最后一个工具上注入 cache_control
// 断点，对齐 Parrot `tools[-1]["cache_control"] = {"type":"ephemeral","ttl":"1h"}`
// 行为，但 ttl 按本仓规则：
//   - 客户端已为该 tool 显式设置 cache_control.ttl → 完全透传不覆盖
//   - 否则注入 {"type":"ephemeral","ttl": claude.DefaultCacheControlTTL}
//
// 纯副作用函数，tools 不存在或为空数组时 no-op。
func applyToolsLastCacheBreakpoint(body []byte) []byte {
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return body
	}
	arr := tools.Array()
	if len(arr) == 0 {
		return body
	}
	lastIdx := len(arr) - 1
	existingCC := arr[lastIdx].Get("cache_control")
	pathPrefix := "tools." + strconv.Itoa(lastIdx) + ".cache_control"

	if existingCC.Exists() {
		// Keep any client ttl, but always normalize to Claude Code's ephemeral type.
		if next, err := sjson.SetBytes(body, pathPrefix+".type", "ephemeral"); err == nil {
			body = next
		}
		if existingCC.Get("ttl").String() != "" {
			return body
		}
		if next, err := sjson.SetBytes(body, pathPrefix+".ttl", claude.DefaultCacheControlTTL); err == nil {
			body = next
		}
		return body
	}

	raw := fmt.Sprintf(`{"type":"ephemeral","ttl":%q}`, claude.DefaultCacheControlTTL)
	if next, err := sjson.SetRawBytes(body, pathPrefix, []byte(raw)); err == nil {
		body = next
	}
	return body
}

// restoreToolNamesInBytes 对 bytes chunk 做逆向还原：假名 → 真名。
// 按 ReverseOrdered 的假名长度倒序逐个 bytes.Replace，防止子串冲突
// （与 Parrot _restore_tool_names_in_chunk 的 sorted(..., reverse=True) 等价）。
// 只有请求侧确实产生了 ToolNameRewrite 时才做任何响应还原；APIKey 或
// anti-ban 关闭路径没有 rewrite context 时必须 no-op。
func restoreToolNamesInBytes(data []byte, rw *ToolNameRewrite) []byte {
	if rw == nil {
		return data
	}
	if rw != nil {
		for _, pair := range rw.ReverseOrdered {
			fake, real := pair[0], pair[1]
			if fake == "" || fake == real {
				continue
			}
			data = replaceAllBytes(data, fake, real)
		}
	}
	if len(rw.Forward) > 0 {
		for prefix, replacement := range staticToolNameRewrites {
			data = replaceAllBytes(data, replacement, prefix)
		}
	}
	// JSON-key-aware prop name reversal (avoids mutating free-text occurrences)
	data = restorePropNamesInJSON(data, rw)
	return data
}

// restorePropNamesInJSON reverses schema property name renames in a JSON-key-aware
// manner. Instead of blind strings.ReplaceAll (which would corrupt model-generated
// prose containing common words like "thread_id"), it only replaces occurrences
// that appear in JSON key position ("fake_name":) or as quoted JSON string values
// ("fake_name"). This is safe for SSE streaming because each `data:` line is a
// complete JSON fragment — keys are not split across chunk boundaries.
func restorePropNamesInJSON(data []byte, rw *ToolNameRewrite) []byte {
	if rw == nil || len(rw.PropReverse) == 0 {
		return data
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return data
	}
	decoded = restorePropNamesJSONValue(decoded, rw.PropReverse, "")
	out, err := json.Marshal(decoded)
	if err != nil {
		return data
	}
	return out
}

func restorePropNamesJSONValue(v any, reverse map[string]string, parentKey string) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for key, child := range val {
			newKey := key
			if real, ok := reverse[key]; ok {
				newKey = real
			}
			out[newKey] = restorePropNamesJSONValue(child, reverse, newKey)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, child := range val {
			if s, ok := child.(string); ok && parentKey == "required" {
				if real, ok := reverse[s]; ok {
					out[i] = real
					continue
				}
			}
			out[i] = restorePropNamesJSONValue(child, reverse, parentKey)
		}
		return out
	default:
		return v
	}
}

// replaceAllBytes 是 bytes.ReplaceAll 的便捷封装，避免每个调用点各自做 []byte 转换。
func replaceAllBytes(data []byte, from, to string) []byte {
	if len(data) == 0 || from == to || !strings.Contains(string(data), from) {
		return data
	}
	return []byte(strings.ReplaceAll(string(data), from, to))
}

// toolNameRewriteFromContext 从 gin.Context 取出请求阶段保存的工具名映射。
// 找不到（c==nil 或 key 不存在或类型不对）时返回 nil；调用方必须能处理 nil。
func toolNameRewriteFromContext(c interface {
	Get(string) (any, bool)
}) *ToolNameRewrite {
	if c == nil {
		return nil
	}
	raw, ok := c.Get(toolNameRewriteKey)
	if !ok || raw == nil {
		return nil
	}
	rw, _ := raw.(*ToolNameRewrite)
	return rw
}

// reverseToolNamesIfPresent 是响应侧注入点的统一封装：从 c 取出本次请求
// 保存的 mapping，并对 chunk 做假名→真名替换。没有 mapping 时必须 no-op，
// 防止 APIKey 或 anti-ban 关闭响应中的自然 cc_sess_/cc_ses_ 文本被误改。
func reverseToolNamesIfPresent(c interface {
	Get(string) (any, bool)
}, chunk []byte) []byte {
	rw := toolNameRewriteFromContext(c)
	if rw == nil {
		return chunk
	}
	return restoreToolNamesInBytes(chunk, rw)
}
