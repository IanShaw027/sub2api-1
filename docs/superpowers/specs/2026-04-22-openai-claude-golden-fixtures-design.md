# OpenAI Claude Golden Fixtures Design

## Goal

固化 OpenAI/Claude 兼容层的关键行为回归样本，避免后续优化在不知情的情况下破坏协议语义、整链路整形、或运行时续链行为。

## Scope

本轮只覆盖 OpenAI/Claude 兼容层的高价值回归面，不扩展到 Gemini、Antigravity、或通用 WebSocket relay 全矩阵。

覆盖范围分两层：

1. `backend/internal/pkg/apicompat`
协议转换层，验证 Anthropic Messages 与 OpenAI Responses 之间的结构与语义映射。

2. `backend/internal/service`
服务链路层，验证 `ForwardAsAnthropic` 在真实 HTTP bridge 中的请求整形、上游请求体、以及下游响应行为。

## Approach Options

### Option A: Only end-to-end service fixtures

优点：
- 最贴近真实流量
- 一套 fixture 就能覆盖完整路径

缺点：
- 失败定位差
- 依赖较重，测试更慢
- 无法单独锁住协议级回归

### Option B: Only apicompat fixtures

优点：
- 运行快
- 语义边界清晰
- golden 更稳定

缺点：
- 覆盖不到 `ForwardAsAnthropic` 的整形与网关逻辑
- 无法锁住 `prompt_cache_key`、`previous_response_id` 等链路行为

### Option C: Two-layer fixtures

优点：
- `apicompat` 负责协议语义
- `service` 负责链路关键路径
- 出问题时定位快，fixture 也能长期复用

缺点：
- 需要维护两组 fixture

推荐采用 Option C。

## Fixture Layout

新增目录：

- `backend/internal/pkg/apicompat/testdata/openai_claude_compat/`
- `backend/internal/service/testdata/openai_claude_compat/`

协议层 fixture 以场景目录组织，每个场景包含：

- `responses.json`
- `name_map.json` 或 `tools.json`
- `expected_anthropic.json`

服务层 fixture 以链路场景目录组织，每个场景包含：

- `anthropic_request.json`
- `expected_upstream_request.json`
- `upstream_sse.txt`
- `expected_downstream_response.json`

第一批只引入 3 个高价值场景：

1. `tool_name_round_trip`
锁住 Claude 原始工具名在桥接后的保真行为。

2. `prompt_cache_key_ordering`
锁住整形后请求体中 `prompt_cache_key` 与 `input` 的顺序关系。

3. `previous_response_id_http_strip`
锁住 HTTP bridge 下 `previous_response_id` 的现有剥离语义，避免与 WebSocket 路径混淆。

## Test Design

### Protocol Layer

在 `backend/internal/pkg/apicompat/anthropic_responses_test.go` 中新增 fixture-driven tests：

- 读取 `responses.json`
- 用 `tools.json` 构建 Claude tool name map
- 调用 `ResponsesToAnthropic`
- 与 `expected_anthropic.json` 做 JSON 级断言

这里只锁语义，不关心服务层 header、HTTP 状态码、或上游 transport 细节。

### Service Layer

在 `backend/internal/service/openai_compat_model_test.go` 中新增 fixture-driven tests：

- 读取 `anthropic_request.json`
- 用现有 `httpUpstreamRecorder` 注入 `upstream_sse.txt`
- 调用 `ForwardAsAnthropic`
- 分别断言：
  - 上游请求体与 `expected_upstream_request.json`
  - 下游响应体与 `expected_downstream_response.json`

整链路断言保留最小必要字段，避免把不稳定字段写死。

## Data Flow

1. 测试从 fixture 读取输入请求与预期结果。
2. `apicompat` 测试只经过纯转换函数。
3. `service` 测试经过 `ForwardAsAnthropic -> request shaping -> upstream SSE -> Anthropic response bridge`。
4. 结果通过 JSON 语义对比验证，不依赖 map 序列化偶然顺序。

## Error Handling

- fixture 读取失败直接 `require.NoError`
- JSON 解析失败直接报测试错误
- 缺字段时优先让 JSON 断言暴露语义差异，而不是隐藏在 helper 内
- 不实现“自动更新 golden”，避免误把行为漂移提交进仓库

## Verification

本轮实现完成后执行：

- `cd backend && go test ./internal/pkg/apicompat -run 'Fixture|ToolUse|ResponsesToAnthropic'`
- `cd backend && go test ./internal/service -run 'Fixture|ForwardAsAnthropic|PromptCacheKey|PreviousResponseID'`
- `cd backend && go test ./...`

## Non-Goals

- 不把所有兼容测试统一重构成 fixture 框架
- 不引入 golden 自动刷新命令
- 不覆盖 WebSocket v2 relay 的完整回归矩阵
- 不修改生产行为，除非为了让新增失败测试通过必须补最小实现
