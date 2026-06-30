package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestBuildDynamicToolMap_BelowThreshold(t *testing.T) {
	// Parrot 行为：tools 数量 ≤ 5 时不做动态映射。
	names := []string{"bash", "edit", "read", "write", "search"}
	require.Nil(t, buildDynamicToolMap(names))
}

func TestBuildDynamicToolMap_AboveThresholdIsStable(t *testing.T) {
	// Parrot 不变量：同一组 tool_names 在同进程内映射稳定（保证 cache 命中）。
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"}
	a := buildDynamicToolMap(names)
	b := buildDynamicToolMap(names)
	require.NotNil(t, a)
	require.Equal(t, a, b, "same input tool_names must yield identical mapping")
	require.Len(t, a, 6)
	for _, name := range names {
		require.Contains(t, a, name)
		require.NotEqual(t, name, a[name])
	}
}

func TestSanitizeToolName_StaticPrefix(t *testing.T) {
	require.Equal(t, "cc_sess_list", sanitizeToolName("sessions_list", nil))
	require.Equal(t, "cc_ses_get", sanitizeToolName("session_get", nil))
	require.Equal(t, "bash", sanitizeToolName("bash", nil))
}

func TestSanitizeToolName_DynamicTakesPrecedence(t *testing.T) {
	dyn := map[string]string{"sessions_list": "analyze_ses00"}
	got := sanitizeToolName("sessions_list", dyn)
	require.Equal(t, "analyze_ses00", got, "dynamic mapping wins over static prefix")
}

func TestRestoreToolNamesInBytes_LongestFirst(t *testing.T) {
	// 当假名 "abc_12" 是另一个更长假名的子串（真实场景极少但算法必须防御）时，
	// 长的必须先替换。本测试用显式构造的映射来验证排序不变量。
	rw := &ToolNameRewrite{
		Forward: map[string]string{"foo": "abc_12", "bar": "abc_12_ext"},
		Reverse: map[string]string{"abc_12": "foo", "abc_12_ext": "bar"},
	}
	// 手工构造 ReverseOrdered：长的在前
	rw.ReverseOrdered = [][2]string{
		{"abc_12_ext", "bar"},
		{"abc_12", "foo"},
	}
	data := []byte(`{"tool":"abc_12_ext","other":"abc_12"}`)
	restored := string(restoreToolNamesInBytes(data, rw))
	require.Equal(t, `{"tool":"bar","other":"foo"}`, restored)
}

func TestRestoreToolNamesInBytes_StaticPrefixRollbackRequiresRewriteContext(t *testing.T) {
	data := []byte(`{"name":"sessions_list","id":"cc_ses_xyz"}`)
	rw := buildToolNameRewriteFromBody([]byte(`{"tools":[{"name":"session_get","input_schema":{}}]}`), nil)
	require.NotNil(t, rw)

	got := string(restoreToolNamesInBytes(data, rw))
	require.Equal(t, `{"name":"sessions_list","id":"session_xyz"}`, got)
}

func TestReverseToolNamesIfPresent_NoRewriteContextIsNoop(t *testing.T) {
	data := []byte(`{"id":"cc_ses_xyz","text":"cc_sess_value"}`)
	got := reverseToolNamesIfPresent(nil, data)
	require.Equal(t, string(data), string(got))
}

func TestApplyToolNameRewriteToBody_RenamesToolsAndToolChoice(t *testing.T) {
	body := []byte(`{"tools":[{"name":"sessions_list","input_schema":{}},{"name":"session_get","input_schema":{}},{"name":"web_search","type":"web_search_20250305"}],"tool_choice":{"type":"tool","name":"sessions_list"}}`)
	rw := buildToolNameRewriteFromBody(body, nil)
	require.NotNil(t, rw)
	require.Contains(t, rw.Forward, "sessions_list")
	require.Contains(t, rw.Forward, "session_get")
	// web_search 是 server tool，不参与工具名改写
	require.NotContains(t, rw.Forward, "web_search")

	out := applyToolNameRewriteToBody(body, rw)

	// tools[0].name 和 tools[1].name 被改写，tools[2].name 保持不变
	require.Equal(t, "cc_sess_list", gjson.GetBytes(out, "tools.0.name").String())
	require.Equal(t, "cc_ses_get", gjson.GetBytes(out, "tools.1.name").String())
	require.Equal(t, "web_search", gjson.GetBytes(out, "tools.2.name").String())

	// tool_choice.name 被同步改写
	require.Equal(t, "cc_sess_list", gjson.GetBytes(out, "tool_choice.name").String())
	require.Equal(t, "tool", gjson.GetBytes(out, "tool_choice.type").String())
}

func TestApplyToolNameRewriteToBody_RenamesToolUseInMessages(t *testing.T) {
	// sessions_list 通过静态前缀规则改写为 cc_sess_list
	// web_search 是 server tool（type != ""），不参与工具名改写
	// messages 中的 tool_use.name 必须同步改写，才能和 tools[] 保持一致
	body := []byte(`{"tools":[{"name":"sessions_list","input_schema":{}},{"name":"web_search","type":"web_search_20250305"}],"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]},{"role":"assistant","content":[{"type":"tool_use","id":"tu_01","name":"sessions_list","input":{}},{"type":"text","text":"thinking"}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_01","content":"ok"}]}]}`)
	rw := buildToolNameRewriteFromBody(body, nil)
	require.NotNil(t, rw)
	require.Equal(t, "cc_sess_list", rw.Forward["sessions_list"])

	out := applyToolNameRewriteToBody(body, rw)

	// tools[0].name 被改写
	require.Equal(t, "cc_sess_list", gjson.GetBytes(out, "tools.0.name").String())
	// tools[1].name 是 server tool，保持不变
	require.Equal(t, "web_search", gjson.GetBytes(out, "tools.1.name").String())
	// messages[1].content[0].name 是 tool_use，必须同步改写以匹配 tools[]
	require.Equal(t, "cc_sess_list", gjson.GetBytes(out, "messages.1.content.0.name").String())
	// messages[1].content[1] 是 text，保持不变
	require.Equal(t, "thinking", gjson.GetBytes(out, "messages.1.content.1.text").String())
	// messages[2].content[0] 是 tool_result，不包含 name 字段，保持不变
	require.Equal(t, "ok", gjson.GetBytes(out, "messages.2.content.0.content").String())
}

func TestApplyToolNameRewriteToBody_RenamesToolUseWithDynamicMapping(t *testing.T) {
	body := []byte(`{"tools":[{"name":"alpha_search","description":"alpha desc","input_schema":{}},{"name":"beta_lookup","description":"beta desc","input_schema":{}},{"name":"gamma_fetch","description":"gamma desc","input_schema":{}},{"name":"delta_update","description":"delta desc","input_schema":{}},{"name":"epsilon_parse","description":"epsilon desc","input_schema":{}},{"name":"zeta_render","description":"zeta desc","input_schema":{}},{"name":"web_search","type":"web_search_20250305","description":"server desc"}],"tool_choice":{"type":"tool","name":"gamma_fetch"},"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"tu_dyn","name":"gamma_fetch","input":{}},{"type":"tool_use","id":"tu_srv","name":"web_search","input":{}},{"type":"text","text":"done"}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_dyn","content":"ok"}]}]}`)
	rw := buildToolNameRewriteFromBody(body, nil)
	require.NotNil(t, rw)
	require.Len(t, rw.Forward, 6)

	fakeGamma := rw.Forward["gamma_fetch"]
	require.NotEmpty(t, fakeGamma)
	require.NotEqual(t, "gamma_fetch", fakeGamma)
	require.NotContains(t, rw.Forward, "web_search")

	out := applyToolNameRewriteToBody(body, rw)

	// 动态映射会改写 tools[]、tool_choice 和历史 tool_use 中的同一个工具名
	require.Equal(t, fakeGamma, gjson.GetBytes(out, "tools.2.name").String())
	require.Equal(t, fakeGamma, gjson.GetBytes(out, "tool_choice.name").String())
	require.Equal(t, fakeGamma, gjson.GetBytes(out, "messages.0.content.0.name").String())
	// server tool 不参与动态映射，历史 tool_use 中同名引用也保持不变
	require.Equal(t, "web_search", gjson.GetBytes(out, "tools.6.name").String())
	require.Equal(t, "web_search", gjson.GetBytes(out, "messages.0.content.1.name").String())
	require.Equal(t, "", gjson.GetBytes(out, "tools.2.description").String())
	require.Equal(t, "server desc", gjson.GetBytes(out, "tools.6.description").String())
	require.True(t, rw.DescStripped)
	// tool_result 依靠 tool_use_id 关联，不需要 name 字段
	require.Equal(t, "ok", gjson.GetBytes(out, "messages.1.content.0.content").String())
}

func TestApplyToolNameRewriteToBody_PreservesUnchangedToolDescriptions(t *testing.T) {
	body := []byte(`{"tools":[
		{"name":"sessions_list","description":"session tool","input_schema":{}},
		{"name":"search","description":"normal tool","input_schema":{}}
	]}`)

	rw := buildToolNameRewriteFromBody(body, nil)
	require.NotNil(t, rw)
	require.Contains(t, rw.Forward, "sessions_list")
	require.NotContains(t, rw.Forward, "search")

	out := applyToolNameRewriteToBody(body, rw)

	require.Equal(t, "", gjson.GetBytes(out, "tools.0.description").String())
	require.Equal(t, "normal tool", gjson.GetBytes(out, "tools.1.description").String())
	require.Equal(t, "cc_sess_list", gjson.GetBytes(out, "tools.0.name").String())
	require.Equal(t, "search", gjson.GetBytes(out, "tools.1.name").String())
}

func TestApplyToolsLastCacheBreakpoint_InjectsDefault(t *testing.T) {
	body := []byte(`{"tools":[{"name":"a","input_schema":{}},{"name":"b","input_schema":{}}]}`)
	out := applyToolsLastCacheBreakpoint(body)
	require.Equal(t, "ephemeral", gjson.GetBytes(out, "tools.1.cache_control.type").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "tools.1.cache_control.ttl").String())
	// First tool untouched
	require.False(t, gjson.GetBytes(out, "tools.0.cache_control").Exists())
}

func TestApplyToolsLastCacheBreakpoint_PassesThroughClientTTL(t *testing.T) {
	body := []byte(`{"tools":[{"name":"a","input_schema":{},"cache_control":{"type":"ephemeral","ttl":"1h"}}]}`)
	out := applyToolsLastCacheBreakpoint(body)
	// User-provided ttl must be preserved.
	require.Equal(t, "1h", gjson.GetBytes(out, "tools.0.cache_control.ttl").String())
}

func TestApplyToolsLastCacheBreakpoint_NormalizesTypeToEphemeral(t *testing.T) {
	body := []byte(`{"tools":[{"name":"a","input_schema":{},"cache_control":{"type":"persistent","ttl":"1h"}}]}`)
	out := applyToolsLastCacheBreakpoint(body)
	require.Equal(t, "ephemeral", gjson.GetBytes(out, "tools.0.cache_control.type").String())
	require.Equal(t, "1h", gjson.GetBytes(out, "tools.0.cache_control.ttl").String())
}

func TestStripMessageCacheControl(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}}]}]}`)
	out := stripMessageCacheControl(body)
	require.False(t, gjson.GetBytes(out, "messages.0.content.0.cache_control").Exists())
}

func TestAddMessageCacheBreakpoints_LastMessageOnly(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	out := addMessageCacheBreakpoints(body)
	require.Equal(t, "ephemeral", gjson.GetBytes(out, "messages.0.content.0.cache_control.type").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
}

func TestAddMessageCacheBreakpoints_SecondToLastUserTurn(t *testing.T) {
	// Parrot 不变量：messages ≥ 4 时才打第二个断点，且位置是"倒数第二个 user turn"。
	body := []byte(`{"messages":[
        {"role":"user","content":[{"type":"text","text":"q1"}]},
        {"role":"assistant","content":[{"type":"text","text":"a1"}]},
        {"role":"user","content":[{"type":"text","text":"q2"}]},
        {"role":"assistant","content":[{"type":"text","text":"a2"}]}
    ]}`)
	out := addMessageCacheBreakpoints(body)
	// 最后一条 assistant 被打断点
	require.Equal(t, "ephemeral", gjson.GetBytes(out, "messages.3.content.0.cache_control.type").String())
	// 倒数第二个 user turn = index 0（唯一另一个 user）
	require.Equal(t, "ephemeral", gjson.GetBytes(out, "messages.0.content.0.cache_control.type").String())
	// 其他不打断点
	require.False(t, gjson.GetBytes(out, "messages.1.content.0.cache_control").Exists())
	require.False(t, gjson.GetBytes(out, "messages.2.content.0.cache_control").Exists())
}

func TestAddMessageCacheBreakpoints_StringContentPromoted(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	out := addMessageCacheBreakpoints(body)
	// content 升级成数组
	require.True(t, gjson.GetBytes(out, "messages.0.content").IsArray())
	require.Equal(t, "text", gjson.GetBytes(out, "messages.0.content.0.type").String())
	require.Equal(t, "hi", gjson.GetBytes(out, "messages.0.content.0.text").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
}

func TestMustJSONString_ReturnsValidJSONString(t *testing.T) {
	in := "line1\nline2\t\"quoted\" 😀"
	raw := mustJSONString(in)

	var decoded string
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, in, decoded)
}

func TestRewriteMessageCacheControlIfEnabled_DefaultKeepsClientAnchors(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":[{"type":"text","text":"stable","cache_control":{"type":"ephemeral","ttl":"1h"}}]},
		{"role":"assistant","content":[{"type":"text","text":"ok"}]},
		{"role":"user","content":[{"type":"text","text":"latest","cache_control":{"type":"ephemeral","ttl":"5m"}}]}
	]}`)

	out := (&GatewayService{}).rewriteMessageCacheControlIfEnabled(context.Background(), body)

	require.JSONEq(t, string(body), string(out))
	require.Equal(t, "1h", gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
	require.Equal(t, "5m", gjson.GetBytes(out, "messages.2.content.0.cache_control.ttl").String())
}

func TestRewriteMessageCacheControlIfEnabled_OptInPreservesLegacyRewrite(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":[{"type":"text","text":"stable","cache_control":{"type":"ephemeral","ttl":"1h"}}]},
		{"role":"assistant","content":[{"type":"text","text":"ok"}]},
		{"role":"user","content":[{"type":"text","text":"latest","cache_control":{"type":"ephemeral","ttl":"1h"}}]},
		{"role":"assistant","content":[{"type":"text","text":"done"}]}
	]}`)
	repo := &gatewayTTLSettingRepo{data: map[string]string{
		SettingKeyRewriteMessageCacheControl: "true",
	}}
	gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{})
	svc := &GatewayService{settingService: NewSettingService(repo, &config.Config{})}

	out := svc.rewriteMessageCacheControlIfEnabled(context.Background(), body)

	require.Equal(t, "5m", gjson.GetBytes(out, "messages.0.content.0.cache_control.ttl").String())
	require.False(t, gjson.GetBytes(out, "messages.2.content.0.cache_control").Exists())
	require.Equal(t, "5m", gjson.GetBytes(out, "messages.3.content.0.cache_control.ttl").String())
}

func TestBuildToolNameRewriteFromBody_ReverseOrderedByLengthDesc(t *testing.T) {
	// 超过阈值触发动态映射，验证 ReverseOrdered 按假名长度倒序排列
	body := []byte(`{"tools":[
        {"name":"t1","input_schema":{}},
        {"name":"t2","input_schema":{}},
        {"name":"t3","input_schema":{}},
        {"name":"t4","input_schema":{}},
        {"name":"t5","input_schema":{}},
        {"name":"t6","input_schema":{}}
    ]}`)
	rw := buildToolNameRewriteFromBody(body, nil)
	require.NotNil(t, rw)
	require.NotEmpty(t, rw.ReverseOrdered)
	for i := 1; i < len(rw.ReverseOrdered); i++ {
		require.GreaterOrEqual(t, len(rw.ReverseOrdered[i-1][0]), len(rw.ReverseOrdered[i][0]),
			"ReverseOrdered must be sorted by fake-name length descending")
	}
}

func TestRestoreToolNamesInBytes_NoMapping_NoStaticMatch_IsNoop(t *testing.T) {
	data := []byte("plain text without any tool names")
	require.Equal(t, string(data), string(restoreToolNamesInBytes(data, nil)))
}

// Ensure the fake name format follows Parrot's "{prefix}{name[:3]}{i:02d}".
func TestBuildDynamicToolMap_FakeNameShape(t *testing.T) {
	names := []string{"alphabet", "bravo", "charlie", "delta", "echo", "foxtrot"}
	m := buildDynamicToolMap(names)
	require.NotNil(t, m)
	for _, name := range names {
		fake, ok := m[name]
		require.True(t, ok)
		// fake = prefix + head3 + "%02d"
		// ends with two decimal digits
		require.Regexp(t, `^[a-z]+_[a-z0-9]{1,3}\d{2}$`, fake)
		head := name
		if len(head) > 3 {
			head = head[:3]
		}
		require.True(t, strings.Contains(fake, head), "fake %q should contain head3 %q of %q", fake, head, name)
	}
}

func TestBuildToolNameRewriteFromBody_ShadowToolsAreSkipped(t *testing.T) {
	body := []byte(`{"tools":[
		{"name":"cc_srv_web_search","input_schema":{}},
		{"name":"cc_srv_web_fetch","input_schema":{}},
		{"name":"sessions_list","input_schema":{}},
		{"name":"alpha_search","input_schema":{}},
		{"name":"beta_lookup","input_schema":{}},
		{"name":"gamma_fetch","input_schema":{}},
		{"name":"delta_update","input_schema":{}},
		{"name":"epsilon_parse","input_schema":{}}
	]}`)

	rw := buildToolNameRewriteFromBody(body, nil)
	require.NotNil(t, rw)
	require.NotContains(t, rw.Forward, "cc_srv_web_search")
	require.NotContains(t, rw.Forward, "cc_srv_web_fetch")
	require.Contains(t, rw.Forward, "sessions_list")
	require.Contains(t, rw.Forward, "alpha_search")
}

func TestApplyToolNameRewriteToBody_ShadowToolsKeepStableNames(t *testing.T) {
	body := []byte(`{"tools":[
		{"name":"cc_srv_web_search","input_schema":{}},
		{"name":"sessions_list","input_schema":{}},
		{"name":"alpha_search","input_schema":{}},
		{"name":"beta_lookup","input_schema":{}},
		{"name":"gamma_fetch","input_schema":{}},
		{"name":"delta_update","input_schema":{}},
		{"name":"epsilon_parse","input_schema":{}}
	],"tool_choice":{"type":"tool","name":"cc_srv_web_search"},"messages":[
		{"role":"assistant","content":[
			{"type":"tool_use","id":"tu_shadow","name":"cc_srv_web_search","input":{"query":"golang"}},
			{"type":"tool_use","id":"tu_normal","name":"sessions_list","input":{}}
		]}
	]}`)

	rw := buildToolNameRewriteFromBody(body, nil)
	require.NotNil(t, rw)
	require.Contains(t, rw.Forward, "sessions_list")
	fakeSessions := rw.Forward["sessions_list"]
	require.NotEmpty(t, fakeSessions)

	out := applyToolNameRewriteToBody(body, rw)

	require.Equal(t, "cc_srv_web_search", gjson.GetBytes(out, "tools.0.name").String())
	require.Equal(t, "cc_srv_web_search", gjson.GetBytes(out, "tool_choice.name").String())
	require.Equal(t, "cc_srv_web_search", gjson.GetBytes(out, "messages.0.content.0.name").String())
	require.Equal(t, fakeSessions, gjson.GetBytes(out, "tools.1.name").String())
	require.Equal(t, fakeSessions, gjson.GetBytes(out, "messages.0.content.1.name").String())
}

func TestRewriteSchemaProperties_MultiplePropertiesDoesNotCorruptJSON(t *testing.T) {
	body := []byte(`{"tools":[{"name":"tool","input_schema":{"type":"object","properties":{"thread_id":{"type":"string","description":"thread"},"session_id":{"type":"string","description":"session"},"keep":{"type":"string"}},"required":["thread_id","session_id","keep"]}}],"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"tool","input":{"thread_id":"thr-1","session_id":"ses-1","keep":"ok"}}]}]}`)
	rw := buildToolNameRewriteFromBody(body, map[string]string{
		"thread_id":  "arg_thread_id",
		"session_id": "arg_session_id",
	})
	require.NotNil(t, rw)

	out := applyToolNameRewriteToBody(body, rw)
	require.True(t, gjson.ValidBytes(out), string(out))
	require.False(t, gjson.GetBytes(out, "tools.0.input_schema.properties.thread_id").Exists())
	require.False(t, gjson.GetBytes(out, "tools.0.input_schema.properties.session_id").Exists())
	require.Equal(t, "string", gjson.GetBytes(out, "tools.0.input_schema.properties.arg_thread_id.type").String())
	require.Equal(t, "string", gjson.GetBytes(out, "tools.0.input_schema.properties.arg_session_id.type").String())
	require.ElementsMatch(t, []string{"arg_thread_id", "arg_session_id", "keep"}, []string{
		gjson.GetBytes(out, "tools.0.input_schema.required.0").String(),
		gjson.GetBytes(out, "tools.0.input_schema.required.1").String(),
		gjson.GetBytes(out, "tools.0.input_schema.required.2").String(),
	})
	require.False(t, gjson.GetBytes(out, "messages.0.content.0.input.thread_id").Exists())
	require.False(t, gjson.GetBytes(out, "messages.0.content.0.input.session_id").Exists())
	require.Equal(t, "thr-1", gjson.GetBytes(out, "messages.0.content.0.input.arg_thread_id").String())
	require.Equal(t, "ses-1", gjson.GetBytes(out, "messages.0.content.0.input.arg_session_id").String())
	require.Equal(t, "ok", gjson.GetBytes(out, "messages.0.content.0.input.keep").String())
}

func TestRewriteSchemaProperties_EscapesSpecialPropertyNames(t *testing.T) {
	body := []byte(`{"tools":[{"name":"tool","input_schema":{"type":"object","properties":{"thread.id":{"type":"string","description":"thread"},"session/id":{"type":"string","description":"session"},"keep":{"type":"string"}},"required":["thread.id","session/id","keep"]}}],"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"tool","input":{"thread.id":"thr-1","session/id":"ses-1","keep":"ok"}}]}]}`)
	rw := buildToolNameRewriteFromBody(body, map[string]string{
		"thread.id":  "arg.thread.id",
		"session/id": "arg/session/id",
	})
	require.NotNil(t, rw)

	out := applyToolNameRewriteToBody(body, rw)
	require.True(t, gjson.ValidBytes(out), string(out))

	props := gjson.GetBytes(out, "tools.0.input_schema.properties").Map()
	require.NotContains(t, props, "thread.id")
	require.NotContains(t, props, "session/id")
	require.Contains(t, props, "arg.thread.id")
	require.Contains(t, props, "arg/session/id")
	require.Equal(t, "string", props["arg.thread.id"].Get("type").String())
	require.Equal(t, "string", props["arg/session/id"].Get("type").String())
	require.ElementsMatch(t, []string{"arg.thread.id", "arg/session/id", "keep"}, []string{
		gjson.GetBytes(out, "tools.0.input_schema.required.0").String(),
		gjson.GetBytes(out, "tools.0.input_schema.required.1").String(),
		gjson.GetBytes(out, "tools.0.input_schema.required.2").String(),
	})

	input := gjson.GetBytes(out, "messages.0.content.0.input").Map()
	require.NotContains(t, input, "thread.id")
	require.NotContains(t, input, "session/id")
	require.Equal(t, "thr-1", input["arg.thread.id"].String())
	require.Equal(t, "ses-1", input["arg/session/id"].String())
	require.Equal(t, "ok", input["keep"].String())
}

func TestRestorePropNamesInJSON_DoesNotTouchFreeTextContent(t *testing.T) {
	rw := &ToolNameRewrite{
		PropReverse: map[string]string{
			"arg_thread_id":  "thread_id",
			"arg_session_id": "session_id",
		},
	}
	data := []byte(`{"tools":[{"input_schema":{"properties":{"arg_thread_id":{"type":"string"},"arg_session_id":{"type":"string"}},"required":["arg_thread_id","arg_session_id"]}}],"messages":[{"role":"assistant","content":[{"type":"text","text":"keep arg_thread_id and arg_session_id in prose"},{"type":"tool_use","id":"tu_1","name":"tool","input":{"arg_thread_id":"thr-1","arg_session_id":"ses-1"}}]}]}`)

	got := restorePropNamesInJSON(data, rw)

	require.True(t, gjson.ValidBytes(got), string(got))
	require.False(t, gjson.GetBytes(got, "tools.0.input_schema.properties.arg_thread_id").Exists())
	require.False(t, gjson.GetBytes(got, "tools.0.input_schema.properties.arg_session_id").Exists())
	require.True(t, gjson.GetBytes(got, "tools.0.input_schema.properties.thread_id").Exists())
	require.True(t, gjson.GetBytes(got, "tools.0.input_schema.properties.session_id").Exists())
	require.Equal(t, "string", gjson.GetBytes(got, "tools.0.input_schema.properties.thread_id.type").String())
	require.Equal(t, "string", gjson.GetBytes(got, "tools.0.input_schema.properties.session_id.type").String())
	require.ElementsMatch(t, []string{"thread_id", "session_id"}, []string{
		gjson.GetBytes(got, "tools.0.input_schema.required.0").String(),
		gjson.GetBytes(got, "tools.0.input_schema.required.1").String(),
	})
	require.Equal(t, "keep arg_thread_id and arg_session_id in prose", gjson.GetBytes(got, "messages.0.content.0.text").String())
	require.False(t, gjson.GetBytes(got, "messages.0.content.1.input.arg_thread_id").Exists())
	require.False(t, gjson.GetBytes(got, "messages.0.content.1.input.arg_session_id").Exists())
	require.Equal(t, "thr-1", gjson.GetBytes(got, "messages.0.content.1.input.thread_id").String())
	require.Equal(t, "ses-1", gjson.GetBytes(got, "messages.0.content.1.input.session_id").String())
}

func TestRestorePropNamesInJSON_RestoresSpecialPropertyNames(t *testing.T) {
	rw := &ToolNameRewrite{
		PropReverse: map[string]string{
			"arg.thread.id":  "thread.id",
			"arg/session/id": "session/id",
		},
	}
	data := []byte(`{"tools":[{"input_schema":{"properties":{"arg.thread.id":{"type":"string"},"arg/session/id":{"type":"string"}},"required":["arg.thread.id","arg/session/id"]}}],"messages":[{"role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"tool","input":{"arg.thread.id":"thr-1","arg/session/id":"ses-1"}}]}]}`)

	got := restorePropNamesInJSON(data, rw)

	require.True(t, gjson.ValidBytes(got), string(got))
	props := gjson.GetBytes(got, "tools.0.input_schema.properties").Map()
	require.NotContains(t, props, "arg.thread.id")
	require.NotContains(t, props, "arg/session/id")
	require.Contains(t, props, "thread.id")
	require.Contains(t, props, "session/id")
	require.ElementsMatch(t, []string{"thread.id", "session/id"}, []string{
		gjson.GetBytes(got, "tools.0.input_schema.required.0").String(),
		gjson.GetBytes(got, "tools.0.input_schema.required.1").String(),
	})
	input := gjson.GetBytes(got, "messages.0.content.0.input").Map()
	require.Equal(t, "thr-1", input["thread.id"].String())
	require.Equal(t, "ses-1", input["session/id"].String())
}

func TestRestorePropNamesInJSON_RewritesFunctionArgumentsString(t *testing.T) {
	rw := &ToolNameRewrite{
		PropReverse: map[string]string{
			"arg_thread_id": "thread_id",
		},
	}
	data := []byte(`{"choices":[{"delta":{"tool_calls":[{"function":{"arguments":"{\"arg_thread_id\":\"thr-1\",\"keep\":\"ok\"}"}}]}}]}`)

	got := restorePropNamesInJSON(data, rw)

	args := gjson.GetBytes(got, "choices.0.delta.tool_calls.0.function.arguments").String()
	require.JSONEq(t, `{"thread_id":"thr-1","keep":"ok"}`, args)
}

func TestRestorePropNamesInJSON_RewritesFunctionArgumentsJSONFragment(t *testing.T) {
	rw := &ToolNameRewrite{
		PropReverse: map[string]string{
			"arg_thread_id": "thread_id",
		},
	}
	data := []byte(`{"choices":[{"delta":{"tool_calls":[{"function":{"arguments":"{\"arg_thread_id\":\""}}]}}]}`)

	got := restorePropNamesInJSON(data, rw)

	args := gjson.GetBytes(got, "choices.0.delta.tool_calls.0.function.arguments").String()
	require.Equal(t, `{"thread_id":"`, args)
	require.NotContains(t, args, "arg_thread_id")
}

func TestRestoreToolNamesInBytes_RewritesPropNamesInsideSSEData(t *testing.T) {
	rw := &ToolNameRewrite{
		PropReverse: map[string]string{
			"arg_thread_id": "thread_id",
		},
	}
	data := []byte("event: response.output_item.done\n" +
		"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"arguments\":\"{\\\"arg_thread_id\\\":\\\"thr-1\\\"}\"}}\n\n")

	got := restoreToolNamesInBytes(data, rw)

	require.Contains(t, string(got), "event: response.output_item.done\n")
	payload := strings.TrimSpace(strings.TrimPrefix(strings.Split(string(got), "\n")[1], "data: "))
	require.JSONEq(t, `{"type":"response.output_item.done","item":{"type":"function_call","arguments":"{\"thread_id\":\"thr-1\"}"}}`, payload)
}

func TestRestoreToolNamesInBytes_RewritesResponsesFunctionCallArgumentsDelta(t *testing.T) {
	rw := &ToolNameRewrite{
		PropReverse: map[string]string{
			"arg_thread_id": "thread_id",
		},
	}
	data := []byte("event: response.function_call_arguments.delta\n" +
		"data: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"{\\\"arg_thread_id\\\":\\\"thr-1\\\"}\"}\n\n")

	got := restoreToolNamesInBytes(data, rw)

	require.Contains(t, string(got), "event: response.function_call_arguments.delta\n")
	payload := strings.TrimSpace(strings.TrimPrefix(strings.Split(string(got), "\n")[1], "data: "))
	require.JSONEq(t, `{"type":"response.function_call_arguments.delta","delta":"{\"thread_id\":\"thr-1\"}"}`, payload)
	require.NotContains(t, payload, "arg_thread_id")
}

func TestRestoreToolNamesInBytes_RewritesSplitResponsesFunctionCallArgumentsDelta(t *testing.T) {
	rw := &ToolNameRewrite{
		PropReverse: map[string]string{
			"arg_thread_id": "thread_id",
		},
	}
	first := []byte("event: response.function_call_arguments.delta\n" +
		"data: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"fc_1\",\"delta\":\"{\\\"arg_th\"}\n\n")
	second := []byte("event: response.function_call_arguments.delta\n" +
		"data: {\"type\":\"response.function_call_arguments.delta\",\"item_id\":\"fc_1\",\"delta\":\"read_id\\\":\\\"thr-1\\\"}\"}\n\n")

	gotFirst := restoreToolNamesInBytes(first, rw)
	gotSecond := restoreToolNamesInBytes(second, rw)

	payloadFirst := strings.TrimSpace(strings.TrimPrefix(strings.Split(string(gotFirst), "\n")[1], "data: "))
	payloadSecond := strings.TrimSpace(strings.TrimPrefix(strings.Split(string(gotSecond), "\n")[1], "data: "))
	combinedDelta := gjson.Get(payloadFirst, "delta").String() + gjson.Get(payloadSecond, "delta").String()
	require.JSONEq(t, `{"thread_id":"thr-1"}`, combinedDelta)
	require.NotContains(t, combinedDelta, "arg_thread_id")
}

func TestRestoreToolNamesInBytes_RewritesSplitChatCompletionFunctionArguments(t *testing.T) {
	rw := &ToolNameRewrite{
		PropReverse: map[string]string{
			"arg_thread_id": "thread_id",
		},
	}
	first := []byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"arguments\":\"{\\\"arg_th\"}}]}}]}\n\n")
	second := []byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"arguments\":\"read_id\\\":\\\"thr-1\\\"}\"}}]}}]}\n\n")

	gotFirst := restoreToolNamesInBytes(first, rw)
	gotSecond := restoreToolNamesInBytes(second, rw)

	payloadFirst := strings.TrimSpace(strings.TrimPrefix(strings.Split(string(gotFirst), "\n")[0], "data: "))
	payloadSecond := strings.TrimSpace(strings.TrimPrefix(strings.Split(string(gotSecond), "\n")[0], "data: "))
	argsFirst := gjson.Get(payloadFirst, "choices.0.delta.tool_calls.0.function.arguments").String()
	argsSecond := gjson.Get(payloadSecond, "choices.0.delta.tool_calls.0.function.arguments").String()
	require.JSONEq(t, `{"thread_id":"thr-1"}`, argsFirst+argsSecond)
	require.NotContains(t, argsFirst+argsSecond, "arg_thread_id")
}

func TestRestoreToolNamesInBytes_DoesNotRewriteOutputTextDelta(t *testing.T) {
	rw := &ToolNameRewrite{
		PropReverse: map[string]string{
			"arg_thread_id": "thread_id",
		},
	}
	data := []byte("event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"literal arg_thread_id text\"}\n\n")

	got := restoreToolNamesInBytes(data, rw)

	payload := strings.TrimSpace(strings.TrimPrefix(strings.Split(string(got), "\n")[1], "data: "))
	require.JSONEq(t, `{"type":"response.output_text.delta","delta":"literal arg_thread_id text"}`, payload)
}
