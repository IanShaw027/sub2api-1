# Kiro Server Tools Bridge Design

## Goal

让 Claude Code / Anthropic 客户端经由当前 Kiro 网关时，原生 server-side web tools 可以完整工作，而不是：

1. `web_search_20250305` 被错误下沉成普通客户端 tool 后触发 Kiro upstream `400`
2. `web_fetch_*` 根本没有桥接能力
3. mixed tools 场景下必须退回“只有一个 web_search 工具”的特殊 emulation
4. 返回链路丢失 `server_tool_use` / `*_tool_result` / continuation 语义

本轮目标是：

1. 支持 Anthropic 原生 `web_search_*` 和 `web_fetch_*`
2. 支持它们与普通客户端工具混合出现
3. 保持现有 Kiro 普通工具、thinking、streaming、failover 语义不变
4. 不依赖猜测 Kiro 私有原生 web-tool upstream 帧协议
5. 通过正式的协议桥把客户端可见行为补齐

## Scope

本轮覆盖：

1. Anthropic `/v1/messages` 走 Kiro 平台时的 server-side web tools 桥接
2. `web_search_*` 与 `web_fetch_*` 的请求侧工具定义转换
3. streaming / non-streaming 响应中的 `server_tool_use`、`web_search_tool_result`、`web_fetch_tool_result`
4. `pause_turn` continuation 与续跑
5. mixed tools 场景下的 Kiro shadow tool 设计
6. 新增 `webfetch` provider/service
7. 复用现有单工具 web-search emulation 的 provider 执行能力，但不再复用它的响应协议
8. 相关后端测试与最小必要的 live repro

本轮不覆盖：

1. 直接实现 Kiro 私有“原生 webSearch/webFetch upstream 协议”
2. 非 web 类 server-side tools，例如 `computer_*`
3. 前端管理面板新增 web-fetch 专用配置 UI
4. 删除现有“纯单工具 web_search emulation”配置体系

## Problem Statement

当前 Kiro 路径的问题不是“搜索 provider 不可用”，而是协议桥断裂：

1. `KiroGatewayService.Forward(...)` 只有在 `tools` 恰好只包含一个 web-search 工具时才进入现有 emulation 路径
2. 其余请求会进入 `backend/internal/pkg/kiro/converter.go`
3. converter 只过滤 `type == "server_tool"`，不会过滤 `web_search_20250305` / `web_fetch_*`
4. 这些 Anthropic server-side tools 最终被错误转换成 Kiro `toolSpecification`
5. Kiro upstream 将其视为格式错误的普通客户端工具并返回 `400`
6. 网关再把该 `400` 包成客户端看到的 `502`

同时，当前仓库虽然已经具备两类高价值资产：

1. Anthropic <-> Responses 的 `web_search_call` 协议转换能力
2. Kiro 普通 `toolUseEvent` / thinking / text 的稳定流式桥接能力

但它仍然缺三块：

1. mixed tools 下的正式 server-side web tools 路由
2. Anthropic server-side web tool 的 `pause_turn` continuation 闭环
3. `web_fetch` provider 与完整桥接

## Approach Options

### Option A: 直接过滤掉所有 server-side web tools

优点：

1. 改动最小
2. 可以立刻消除当前 `502`

缺点：

1. Claude Code 原生联网能力仍然不可用
2. 本质上是禁用，不是修复

### Option B: 放宽现有 web-search emulation 触发条件

优点：

1. 能部分复用现有 `gateway_websearch_emulation.go`

缺点：

1. 现有 emulation 只适合“单工具直接回答”，不适合 mixed tools
2. 它会生成 `server_tool_use` + `web_search_tool_result`，但以 `end_turn` 结束，不是 continuation 闭环
3. 无法自然扩展到 `web_fetch`

### Option C: 建立正式的 Kiro server-tools 协议桥

做法：

1. 不再把 Anthropic server-side web tools 当成 Kiro 普通工具原样下沉
2. 改为把它们转换成 Kiro 可理解的 shadow client tools
3. 在 Kiro `toolUseEvent` 返回时，由 gateway 拦截并执行真正的 search/fetch
4. 对客户端返回 Anthropic 原生 `server_tool_use` + `*_tool_result` + `pause_turn`
5. continuation 请求再被桥接回 Kiro 普通 `tool_use` / `tool_result` 语义完成续跑

优点：

1. 客户端行为完整
2. mixed tools 可工作
3. 不需要猜 Kiro 私有原生 web-tool upstream 帧
4. 能同时覆盖 `web_search` 与 `web_fetch`

缺点：

1. 工程量最大
2. 需要新增 `webfetch` provider/service
3. 需要补 continuation 测试矩阵

推荐采用 Option C。

## Design Summary

本轮采用“Anthropic server-side web tools <-> Kiro shadow tools <-> continuation replay”的正式桥接方案。

高层流程：

1. 请求进入网关时，识别 `web_search_*` / `web_fetch_*`
2. 这些工具不会直接作为 Anthropic server tools 原样送入 Kiro converter
3. 网关把它们转换成内部 shadow client tools，并为 Kiro 生成合法 `toolSpecification`
4. Kiro upstream 把它们当普通可调用工具使用，返回标准 `toolUseEvent`
5. 网关发现命中的工具属于 shadow server tool 后：
   - 本地执行真实 search/fetch
   - 对客户端发回 `server_tool_use` + `*_tool_result`
   - 结束当前响应，`stop_reason = pause_turn`
6. 客户端带着 assistant 的 server-tool blocks 发起 continuation
7. 网关把这些 blocks 重新桥接为 Kiro 期望的 assistant `tool_use` + synthetic user `tool_result`
8. Kiro 继续生成最终回答

这样，客户端看到的是正式 Anthropic server-tool 协议，Kiro upstream 看到的是它已经支持的普通工具协议。

## Tool Model

### Supported Tool Families

本轮只桥接两类 Anthropic server-side tools：

1. `web_search_*`
2. `web_fetch_*`

额外兼容：

1. `google_search` 继续归并到 `web_search`
2. 历史别名 `web_search` 也归并到 `web_search_*`

不在本轮支持的 server-side tools：

1. `computer_*`
2. `tool_search_*`
3. 其他未知 server-side tool families

对于这些未支持类型，网关必须直接拒绝为客户端错误，不能再错误下沉成普通 Kiro `toolSpecification`。

## Legacy Emulation Role

现有 `gateway_websearch_emulation.go` 不再充当 Kiro server-side web tools 的协议出口。

它在本轮后的职责收缩为：

1. 复用其中已经稳定的 search provider 执行逻辑
2. 继续作为历史兼容代码存在，直到新的正式桥路径完全替代 Kiro 入口中的单工具 shortcut

Kiro 平台的新 server-side web tools 行为统一走正式桥，不再依赖 “tools 里只有一个 web_search” 的特殊分支。

### Shadow Tool Names

内部为 Kiro 暴露固定 shadow tool：

1. `cc_srv_web_search`
2. `cc_srv_web_fetch`

命名要求：

1. 与客户端显式工具名区分开
2. 不参与现有 tool-name rewrite 混淆
3. 在 continuation 重放时可稳定识别

### Shadow Tool Schemas

`cc_srv_web_search` 输入：

```json
{
  "type": "object",
  "properties": {
    "query": { "type": "string" }
  },
  "required": ["query"]
}
```

`cc_srv_web_fetch` 输入：

```json
{
  "type": "object",
  "properties": {
    "url": { "type": "string" }
  },
  "required": ["url"]
}
```

本轮先不把 `allowed_domains`、`blocked_domains`、`max_uses`、`max_content_tokens` 暴露给 Kiro 当调用参数；这些属于工具定义侧约束，由 gateway 本地保留并在执行时应用。

## Request-Side Design

### Tool Partitioning

进入 Kiro 路径前，先把请求 `tools[]` 分为三类：

1. 普通客户端工具
2. 支持的 server-side web tools
3. 其他不支持的 server-side tools

规则：

1. 普通客户端工具按现有逻辑继续进入 Kiro converter
2. 支持的 server-side web tools 被替换成 shadow tools
3. 不支持的 server-side tools 不允许继续错误下沉

### Bridge Metadata

`ParsedRequest` 或 Kiro convert result 需要新增 bridge metadata，至少包含：

1. 哪些 shadow tool 对应 `web_search` / `web_fetch`
2. 原始 Anthropic tool type
3. 工具定义约束，例如 `allowed_domains` / `blocked_domains`
4. continuation replay 所需的稳定映射

该 metadata 只在本次请求与 continuation replay 内部使用，不直接透传给客户端。

### Continuation Replay Normalization

当新的 Anthropic request 中出现 assistant message，且其 content 含有：

1. `server_tool_use`
2. `web_search_tool_result`
3. `web_fetch_tool_result`

网关要把这段 assistant message 规范化为 Kiro 可继续推理的历史：

1. assistant 侧保留一条 shadow `tool_use`
2. synthetic user 侧追加对应 `tool_result`
3. 若同一 assistant message 里还带 text / thinking，保持原顺序与现有历史语义一致

这样 Kiro 会把 continuation 视为“上一轮调用过工具并且已经收到结果”，从而生成后续答案。

## Response-Side Design

### Streaming

当 Kiro streaming 返回 `toolUseEvent` 时：

1. 普通客户端工具：沿用现有 `tool_use` SSE 行为
2. shadow web tools：
   - 不对客户端发普通 `tool_use`
   - 累积完整 input
   - 执行 search/fetch
   - 对客户端发 `server_tool_use`
   - 对客户端发 `web_search_tool_result` 或 `web_fetch_tool_result`
   - `message_delta.stop_reason = pause_turn`
   - `message_stop`

当前 turn 到此结束，不继续在同一响应里生成最终答案。

### Non-Streaming

buffered 路径行为与 streaming 一致，只是一次性返回完整 message：

1. content 中包含 `server_tool_use`
2. 紧跟对应 `*_tool_result`
3. `stop_reason = pause_turn`
4. 不补“工具执行后的最终文字答案”

### Web Fetch Result Shape

`web_fetch_tool_result` 采用与 `web_search_tool_result` 平行的 Anthropic content block，内部最少包含：

1. `url`
2. `title`
3. `page_content`

当抓取失败时：

1. `page_content` 放错误说明
2. 结果块仍然返回，供模型在 continuation 中消费

### Why `pause_turn`

本轮采用 `pause_turn` 而不是“同轮直接给最终答案”，原因：

1. 它与 Anthropic server-side tool continuation 语义对齐
2. mixed tools 情况下更稳定，不会把“工具执行”和“最终推理”硬塞进一次上游 Kiro turn
3. 续跑时可以统一走已有 tool continuation / replay 体系

## Provider Design

### Web Search

复用现有 `backend/internal/pkg/websearch`：

1. provider 选择
2. proxy
3. quota
4. 失败回退

当前 `gateway_websearch_emulation.go` 里的 provider 调用逻辑抽为可复用 helper，避免继续把 search 执行散落在 emulation 文件内。

### Web Fetch

新增 `backend/internal/pkg/webfetch`，最小职责：

1. 拉取指定 URL
2. 跟随有限重定向
3. 支持代理
4. 解析 `text/html` 与纯文本响应
5. 将 HTML 提取为可控长度的纯文本内容

实现原则：

1. 优先使用现有依赖与标准库
2. HTML 文本提取先采用保守策略：
   - 删除 `script/style/noscript`
   - 保留 title
   - 提取正文纯文本
   - 截断到配置上限
3. 不做浏览器渲染

结果块至少包含：

1. `url`
2. `title`
3. `content`

## Error Handling

### Unsupported Server Tools

当请求里出现当前不支持的 server-side tool family：

1. 不再错误下沉到 Kiro upstream
2. 直接返回 `400 invalid_request_error`
3. `message` 明确指出当前 Kiro bridge 不支持该 server-side tool family
4. 运维日志保留原始 tool 定义，便于后续扩展

### Tool Execution Failure

当 shadow `web_search` / `web_fetch` 执行失败：

1. 仍然返回 `server_tool_use`
2. tool result 标记为错误内容
3. `pause_turn` 继续成立

这样 continuation 时模型仍有机会根据错误结果决定如何回复用户。

### Mixed Tool Ordering

本轮只处理“当前 Kiro turn 实际调用到的第一个 shadow web tool”：

1. 若模型先调用普通客户端工具，保持现有 `tool_use` 停止
2. 若模型先调用 shadow web tool，返回 `pause_turn`
3. 续跑后模型可以继续决定是否再调用其他工具

这与现有 Anthropic / Responses continuation 体系更一致，也避免在一个 stream 内混出多种 stop reason。

## Testing

本轮测试按红-绿顺序补齐：

1. `backend/internal/pkg/kiro/converter_test.go`
   - server-side web tools 不再被错误当作普通 Kiro tools
   - shadow tool 注入行为正确
2. `backend/internal/service/kiro_gateway_service_test.go`
   - streaming `server_tool_use` + `web_search_tool_result` + `pause_turn`
   - buffered `pause_turn`
   - continuation replay 后生成最终答案
   - mixed tools 下普通 `tool_use` 与 shadow web tools 的优先级
3. `backend/internal/pkg/apicompat/anthropic_to_responses_response_test.go`
   - reverse bridge 正确识别 server-side web tool blocks
4. `backend/internal/pkg/apicompat/responses_to_anthropic_request_test.go`
   - `web_fetch` 映射与保留语义
5. `backend/internal/service/gateway_forward_as_responses_test.go`
   - `/v1/responses` streaming / replay 行为与终止事件一致
6. `backend/internal/pkg/webfetch/*_test.go`
   - HTML/text 提取、截断、错误路径

## Verification

完成实现后需要两层验证：

1. 仓库内 focused tests
2. 使用当前 `ANTHROPIC_BASE_URL=https://crs.qazwc.com` 做 live repro

live repro 至少覆盖：

1. baseline 普通消息仍然 `200`
2. `web_search` 不再返回当前的 `502 -> Kiro upstream returned 400`
3. continuation 后能够产出最终回答

## Open Questions Resolved

### 为什么不直接实现 Kiro 原生 webSearch/webFetch upstream 协议

因为当前仓库和现有抓包证据里：

1. 没有 Kiro 原生 `web_fetch` upstream 帧样本
2. 也没有稳定的 Kiro 原生 `web_search` 专用事件族证据
3. 现在直接写只能靠猜私有协议

本轮选择正式协议桥，是为了在现有证据下做出完整、可验证、可维护的实现，而不是把行为建立在未证实的 upstream 私有帧格式上。

### 为什么不继续扩现有单工具 emulation

因为它的职责边界就是“单次搜索并直接拼回答”，并不具备：

1. mixed tools
2. `pause_turn`
3. continuation replay
4. `web_fetch`

继续在那条路径上加逻辑只会让协议分叉更多。
