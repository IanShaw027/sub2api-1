# OpenAI Response Rewrite And Model Support Errors Design

## Goal

解决当前 OpenAI `/v1/responses` 网关里两类对用户不友好的错误响应：

1. 本地选账号失败时，经常直接返回模糊的 `503 Service temporarily unavailable`
2. 某些上游错误虽然不适合直接透传，但又需要按账号维度定制最终返回给用户的文案

本轮目标是：

1. 本地“没有账号支持这个模型”必须明确返回给用户
2. 保持现有切号、重试、failover 行为不变
3. 仅在最终返回给客户端时支持按账号配置“状态码 + 关键词 -> 响应文案改写”
4. 保留现有全局 `error_passthrough_rules` 作为平台级兜底
5. 运维侧继续保留原始上游错误信息，不因对客户端改写而丢失排障线索

## Scope

本轮覆盖：

1. OpenAI `/v1/responses`、`/v1/chat/completions`、`/v1/embeddings`、`/v1/images` 最终错误返回链路中的本地无账号文案修正
2. 新增账号级 `response_rewrite_rules`
3. 账号创建/编辑弹窗中的规则配置 UI
4. 后端账号配置解析、规则匹配与最终响应改写
5. 相关后端与前端测试

本轮不覆盖：

1. 改变现有重试、切号、failover 的时机与次数
2. 按账号改写 HTTP 状态码
3. 独立新增数据库表或 migration
4. 替换现有全局 `error_passthrough_rules`
5. 非 OpenAI 平台的新响应改写体系

## Problem Statement

当前 OpenAI handler 中，当 `SelectAccountWithScheduler...` 失败且还没有形成 `lastFailoverErr` 时，很多分支直接返回：

- `503`
- `api_error`
- `Service temporarily unavailable`

这会把两类完全不同的问题混在一起：

1. 本地调度失败，例如没有账号支持请求模型
2. 上游真实不可用，例如过载、限流、欠费、鉴权失败

用户会误以为这是上游统一故障，而不是当前 group/key 下没有可用账号支持该模型。

同时，现有全局 `error_passthrough_rules` 适合平台级统一策略，但不适合“某个账号遇到某类错误时，最终文案要特殊处理”的场景。

## Design Summary

本轮采用两层改造：

1. 本地错误单独明确化
2. 上游最终错误增加账号级响应改写

分层顺序如下：

1. 本地选路失败优先返回明确文案
2. 保留现有 OpenAI overload 特殊分支
3. 上游最终错误优先匹配账号级 `response_rewrite_rules`
4. 若账号级未命中，再匹配全局 `error_passthrough_rules`
5. 最后落回现有默认错误映射

这样可以同时满足：

1. 本地“没有可用账号支持模型”不再伪装成上游 503
2. 上游特定错误可以按账号做最终文案定制
3. 平台级统一规则仍可继续使用

## Local Error Behavior

### Explicit Selection Failure Messages

对于 OpenAI 选账号阶段的本地失败，不再默认返回 `Service temporarily unavailable`。

行为：

1. 已有更精确的本地错误分支保持原样，不被本轮统一文案覆盖，例如：
   - `compact_not_supported`
   - `No available compatible accounts`
   - 其他已经明确表达 endpoint-specific 兼容性约束的分支
2. 仍然落在通用“本地选账号失败”分支，且错误包含“supporting model: <model>”时：
   - 返回 `503`
   - `type=api_error`
   - `message=No available accounts supporting model: <model>`
3. 仍然落在通用“本地选账号失败”分支，且属于普通无账号时：
   - 返回 `503`
   - `type=api_error`
   - `message=No available accounts`
4. 仅在无法归类为本地可解释错误时，才保留 `Service temporarily unavailable`

### Why Keep 503

本轮只改 message，不改状态码。

原因：

1. 现有客户端很可能把这类账号池问题当作服务端错误处理
2. 用户当前最痛的是“原因不清楚”，不是“状态码不对”
3. 保持 503 可以减少协议兼容风险

## Account-Level Response Rewrite Rules

### Storage Shape

账号级规则挂在 `credentials.response_rewrite_rules`，风格对齐现有 `credentials.temp_unschedulable_rules`。

值形态：

```json
[
  {
    "status_code": 503,
    "keywords": ["欠费", "insufficient_quota"],
    "match_mode": "all",
    "response_message": "Service temporarily unavailable",
    "description": "503 欠费时对客户端隐藏上游细节"
  },
  {
    "status_code": 429,
    "keywords": [],
    "match_mode": "any",
    "response_message": "Service temporarily unavailable",
    "description": "429 统一模糊返回"
  }
]
```

### Rule Fields

单条规则结构：

1. `status_code`
   - 允许为空
   - `100-599`
2. `keywords`
   - 允许为空
   - 多个关键词，任一命中即可
   - 不区分大小写
3. `match_mode`
   - `all`
   - `any`
4. `response_message`
   - 必填
   - 最终返回给客户端的 message
5. `description`
   - 允许为空
   - 仅供管理端展示

### Validation Rule

单条规则必须满足：

1. `response_message` 非空
2. `status_code` 和 `keywords` 至少有一个非空
3. `match_mode` 仅允许 `all` 或 `any`

### Why Credentials Instead Of New Table

采用账号 `credentials` JSON，而不是新增数据库实体，原因是：

1. 语义上它是账号私有策略
2. 仓库里已有 `temp_unschedulable_rules` 先例
3. 本轮无需引入新的 CRUD 列表页与 migration
4. 可以直接复用账号创建/编辑弹窗的写回路径

## Matching Semantics

### Input

账号级规则只匹配“最终要返回给客户端的上游错误”：

1. 原始上游 status code
2. 原始上游响应体提取出的 message / body 文本

不使用已经过 `mapUpstreamError(...)` 之后的最终客户端状态码作为匹配输入。

原因：

1. 用户配置的规则语义是“上游返回了什么”
2. 现有默认映射可能把 `401/429/503` 改写成其他客户端状态码
3. 若按映射后的状态码匹配，账号级规则会变得不可预测

### Match Mode

#### `all`

要求：

1. `status_code` 命中
2. 且 `keywords` 至少一项命中

若某一侧为空，则只要求另一侧命中。

#### `any`

要求：

1. `status_code` 命中
2. 或 `keywords` 至少一项命中

### Ordering

规则按数组顺序匹配，命中第一条即停止。

这与现有 `temp_unschedulable_rules` 的“顺序敏感”交互一致，也便于用户做精确优先级控制。

## Runtime Integration

### High-Level Order

最终响应前的处理顺序：

1. 判断是否是本地选路失败
2. 若是本地选路失败，直接返回明确本地文案
3. 若命中现有 OpenAI overload 特殊分支，保持现有 overload 行为
4. 若是普通上游最终失败，先尝试账号级改写
5. 若账号级未命中，再尝试全局 `error_passthrough_rules`
6. 若全局也未命中，走现有默认错误映射

### Account-Level Rewrite Trigger Point

账号级改写只在“最终失败即将返回客户端”时触发。

不在以下阶段触发：

1. 单次上游失败但后续切号成功
2. 中间重试过程中
3. 本地选路失败但根本没有上游响应体时

这样可以保证：

1. 不改变现有 failover 行为
2. 不会因为某个中间失败就污染最终成功请求

### Final Failing Account Ownership

账号级改写必须绑定“最终失败的那个账号”，不能只拿 `failoverErr` 本身做平台级匹配。

原因：

1. 现有 `UpstreamFailoverError` 只携带状态码、响应体和响应头，不携带账号 ID
2. 同一次请求可能经过多个账号，只有最后那个失败账号的规则才应该生效

本轮要求：

1. handler 在保存 `lastFailoverErr` 时，同时保存对应的 `lastFailoverAccountID` 或 `lastFailoverAccount`
2. 对于没有切号、直接在当前账号上终止的上游错误，直接使用当前 `account`
3. 最终错误处理 helper 需要显式接收账号上下文，例如：
   - `handleFailoverExhausted(c, failoverErr, account, streamStarted)`
   - 或等价的 `accountID` 形态
4. 若最终失败账号上下文缺失，则跳过账号级改写，继续走全局规则和默认映射

### Local Failure Handling

OpenAI handler 中当前这些位置需要从模糊 503 调整为明确本地错误：

1. `/v1/responses`
2. `/v1/chat/completions`
3. `/v1/embeddings`
4. `/v1/images`

原则一致：

1. 有模型支持信息时返回 `No available accounts supporting model: ...`
2. 无模型支持信息时返回 `No available accounts`

## Interaction With Global Error Passthrough Rules

全局 `error_passthrough_rules` 继续保留。

推荐优先级：

1. 本地错误
2. 账号级 `response_rewrite_rules`
3. 全局 `error_passthrough_rules`
4. 默认映射

原因：

1. 本地错误最确定，不应被任何上游规则覆盖
2. 账号级策略应优先于平台级兜底
3. 全局规则继续处理跨账号统一行为

## Admin UI Design

### Placement

在 `CreateAccountModal` 和 `EditAccountModal` 中新增一个与 `Temp Unschedulable Rules` 同风格的 section：

- `Response Rewrite Rules`

放在账号策略相关区块内，不单独新开管理页面。

该 section 仅对 `platform === openai` 的账号显示。

本轮不在 Anthropic、Gemini、Kiro、Antigravity、Sora 账号上展示该配置，避免生成无效配置。

### UI Shape

包含：

1. 总开关
2. 规则列表
3. 上移 / 下移 / 删除
4. 新增规则按钮
5. 预设按钮

### Fields

每条规则展示：

1. `status_code`
2. `match_mode`
3. `keywords`
4. `response_message`
5. `description`

### Presets

首版提供两个预设：

1. `503 + 欠费`
2. `429 + 限流`

目的不是内置业务逻辑，而是降低手工配置成本。

### Serialization

保存时：

1. 开关关闭且无规则：删除 `credentials.response_rewrite_rules`
2. 开关开启但规则为空：删除 `credentials.response_rewrite_rules`
3. 关键词按逗号分隔输入，保存时去空、去两端空白

## Ops And Logging

### Client-Facing Response

客户端只看到改写后的 `message`。

### Monitoring

运维日志仍保留：

1. 原始上游状态码
2. 原始上游响应体提取信息
3. 本地错误时的真实本地失败原因

不因为对客户端的 message 改写而丢掉排障信息。

### Optional Future Enhancement

后续可以在 ops 上下文里额外记录：

1. `response_rewrite_rule_matched=true`
2. `response_rewrite_rule_index`

但本轮不是必需项。

## Testing Plan

### Backend Parsing Tests

新增账号级规则解析测试：

1. 读取 `credentials.response_rewrite_rules`
2. 空数组
3. 非法项过滤
4. 顺序保持

### Backend Match Tests

覆盖：

1. 仅状态码匹配
2. 仅关键词匹配
3. `all` 模式
4. `any` 模式
5. 第一条优先

### Handler Behavior Tests

覆盖：

1. `/v1/responses` 本地模型不支持时，不再返回 `Service temporarily unavailable`
2. 返回 `No available accounts supporting model: gpt-5`
3. 普通无账号时返回 `No available accounts`
4. `compact_not_supported` 仍保持现有精确错误，不被通用文案覆盖
5. `No available compatible accounts` 仍保持现有精确错误，不被通用文案覆盖
6. 上游 `503 + insufficient_quota` 命中账号规则后，只替换 message
7. 上游 `429` 命中账号规则后，只替换 message
8. 两账号 failover 场景下，以最终失败账号的规则为准，而不是中间失败账号
9. 未命中账号规则时，仍走全局规则或默认映射

### Frontend Tests

覆盖：

1. OpenAI 账号显示该 section，非 OpenAI 账号不显示
2. 账号弹窗规则增删改排序
3. 关键词序列化
4. 开关关闭时字段移除
5. 校验提示

## Risks

1. 用户可能误以为账号级改写会影响切号逻辑
   - 需要在 UI hint 中明确“仅影响最终返回文案”
2. 账号级与全局规则同时存在时可能混淆
   - 通过固定优先级解决
3. 本地错误 message 变化可能影响少量依赖旧文案的客户端
   - 但这是有意纠偏，收益高于兼容旧错误文案

## Rollout

本轮直接随代码上线，无需 migration。

推荐上线后重点验证：

1. `gpt-5` 无支持账号时返回明确文案
2. 配置 `503 + 欠费关键词` 的账号，在最终失败时能返回改写后的 message
3. `ops_error_logs` 仍保留原始上游细节
