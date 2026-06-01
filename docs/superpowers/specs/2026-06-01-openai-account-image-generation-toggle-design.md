# OpenAI Account Image Generation Toggle Design

## Goal

为 OpenAI 账号增加账号级“允许生图”开关，默认开启。关闭后，该账号不能被用于实际触发生图的 OpenAI 请求，包括：

1. `/v1/images` 与 `images2api` 路径
2. `/v1/responses` 中明确要求生成图片的请求
3. WebSocket `response.create` / 后续 turn 中明确要求生成图片的请求

当分组允许生图、但当前没有任何“允许生图且可调度”的 OpenAI 账号时，保持现有调度失败语义，返回“无可用账号 / 服务不可用”，不返回账号级权限错误。

## Scope

本轮只覆盖 OpenAI 账号的账号级生图开关，以及与其直接相关的管理端、调度器、请求识别和测试。

覆盖范围：

1. OpenAI 账号创建、编辑、批量编辑
2. 账号 DTO 与前端类型
3. OpenAI 图片专用调度链路
4. OpenAI Responses HTTP / WebSocket 生图意图链路
5. 相关后端与前端测试

不覆盖：

1. 非 OpenAI 平台账号
2. 分组级生图开关与计费规则
3. 图片能力统计面板或筛选器
4. 账号表新增实体列

## Approach Options

### Option A: 新增 `accounts.allow_image_generation` 列

优点：

- 数据模型最直接
- 后续 SQL 查询与统计更清晰

缺点：

- 需要新增 migration、Ent schema、生成代码
- 这次主要消费点在内存账号对象与调度过滤，收益有限

### Option B: 使用 `accounts.extra.openai_image_generation_enabled`

优点：

- 与现有 OpenAI 账号级特性保持一致
- 不需要新增数据库列
- 能直接复用现有创建、编辑、批量编辑的 `extra` 合并路径

缺点：

- 不是强 schema 字段，需要 helper 和 DTO 统一约束

### Option C: 只在前端临时写 `extra`，后端按需读取

优点：

- 最快

缺点：

- 类型与测试面容易遗漏
- 批量编辑和 DTO 显示层容易出现行为漂移

推荐采用 Option B。

## Data Model

账号级开关存储在：

- `accounts.extra.openai_image_generation_enabled`

语义：

- `false`：该 OpenAI 账号不允许承接实际生图请求
- `true`：允许
- 缺失：默认按 `true` 处理

仅 `platform=openai` 生效。非 OpenAI 账号即使存在该 key，也不参与此功能语义。

## Account Helper Design

在 `backend/internal/service/account.go` 新增统一 helper：

- `OpenAIImageGenerationAllowed() bool`

规则：

1. 非 OpenAI 账号返回 `true`
2. `extra.openai_image_generation_enabled == false` 时返回 `false`
3. 其他情况返回 `true`

该 helper 作为账号级图片调度与请求兜底判断的唯一入口，避免多个代码路径各自解析 `extra`。

## Request Classification Rules

账号级过滤必须继续沿用当前“显式意图才算生图”的规则，不能把“仅声明工具能力”误判为生图请求。

判定来源沿用现有能力：

1. `/v1/images*` 请求天然视为生图请求
2. `/v1/responses`：
   - 指向图片模型算生图
   - `tool_choice` 明确选择 `image_generation` 算生图
   - 仅 `tools` 里带 `image_generation` capability 不算生图
3. WebSocket `response.create` 与后续 turn 使用相同规则

Codex image bridge 会在部分请求中自动注入 `image_generation` tool capability，因此调度过滤不能只看 `tools[]` 是否包含该 tool，必须只在“显式生图意图”为真时要求账号具备“允许生图”资格。

## Scheduler Integration

### Images / Images2API

`/v1/images` 与 `images2api` 已走 `SelectAccountWithSchedulerForImages(...)`。

本轮在现有图片路由可调度判断中并入账号级开关：

- `IsSelectableForOpenAIImageRoute(...)`
- `IsSchedulableForOpenAIImageRoute(...)`

新增约束：

- 当账号 `OpenAIImageGenerationAllowed() == false` 时，不进入图片专用候选池

结果：

- 关闭生图的 OpenAI 账号不会被图片专用调度选中
- 没有候选账号时，沿用现有“无可用兼容账号 / 服务不可用”语义

### Responses HTTP

`backend/internal/handler/openai_gateway_handler.go` 现有逻辑已经在调度前后用 `classifyOpenAIResponsesImageRequest(...)` 区分：

- `imageIntent`
- `needsImageSlot`

本轮扩展 `SelectAccountWithScheduler(...)` 的请求条件，新增一个显式调度约束，语义等价于：

- “当前请求需要可生图账号”

当 `imageIntent=true` 时：

- 调度器过滤掉 `!account.OpenAIImageGenerationAllowed()` 的账号

当 `imageIntent=false` 时：

- 不因账号关闭生图而排除
- 继续允许文本请求和仅携带图片工具 capability 的请求走这些账号

### Responses WebSocket

WebSocket 首包和后续 turn 已分别做显式生图意图判断。

本轮规则：

1. 首次选账号时，若首包 `imageIntent=true`，调度器只从允许生图账号中选择
2. 首包 `imageIntent=false` 时，不强制要求允许生图账号
3. 后续 turn 继续沿用现有逐 turn 显式意图识别，只决定是否申请图片并发槽位，不改变账号级错误语义

目标是避免长连接一开始绑定到禁生图账号上，同时保持现有 turn 内图片并发控制行为。

## Service-Layer Safeguards

分组级禁生图仍维持现有 `403 permission_error` 语义，不做变更。

账号级禁生图不作为用户可见权限错误暴露。主路径应通过调度过滤保证不选到禁生图账号。

服务层只保留防御式兜底：

- 若未来某条路径绕过调度，把“显式生图请求”发送到了禁生图账号，记录日志并走内部失败路径
- 不新增面向用户的账号级 `403 permission_error`

## Admin UI

### Create Account

在 `frontend/src/components/account/CreateAccountModal.vue` 新增 OpenAI 账号开关：

- 仅在 `platform=openai` 且账号类型为 `oauth` / `apikey` 时显示
- 默认值为开启
- 提交时写入 `extra.openai_image_generation_enabled`

### Edit Account

在 `frontend/src/components/account/EditAccountModal.vue` 新增相同开关：

- 仅在 OpenAI `oauth` / `apikey` 账号显示
- 打开弹窗时从 DTO 显式字段回填
- 保存时只更新该 key，不覆盖其他 OpenAI `extra`

### Bulk Edit

在 `frontend/src/components/account/BulkEditAccountModal.vue` 增加可选批量编辑项：

- `修改允许生图`
- 值为开 / 关

发送时使用：

- `updates.extra.openai_image_generation_enabled`

后端沿用现有 OpenAI 账号逐条 merge `extra` 的分支，确保不会覆盖其他 `extra` 配置。

## DTO and Type Design

为避免前端重复解析 `extra`，响应 DTO 增加显式字段：

- `openai_image_generation_enabled`

规则：

- OpenAI 账号返回解析后的布尔值
- 非 OpenAI 账号返回默认 `true`

同步调整：

1. `backend/internal/handler/dto/types.go`
2. `backend/internal/handler/dto/mappers.go`
3. `frontend/src/types/index.ts`

写路径继续复用 `extra`，不新增 admin account create/update 顶层字段。

## Error Semantics

### Keep As-Is

1. 分组禁生图：
   - 返回现有 `403 permission_error`
2. 图片并发槽不足：
   - 返回现有图片并发限制错误
3. 上游图片路由不可用：
   - 返回现有 failover / service unavailable 语义

### New Account-Level Behavior

当分组允许生图，但当前没有任何允许生图的 OpenAI 账号可用时：

- `/v1/images*`：继续返回现有“无可用兼容账号 / 服务不可用”
- `/v1/responses`：继续返回现有“无可用账号 / 服务不可用”
- WebSocket：继续返回现有“no available account”或等价的调度失败关闭语义

不新增账号级权限错误文案。

## Implementation Notes

1. 对非 OpenAI 账号，后端应忽略该字段的业务语义
2. 批量编辑即使误带该字段到非 OpenAI 账号，也不应影响其调度逻辑
3. 所有请求级“是否必须使用允许生图账号”的判断，应以显式生图意图为准，不能把单纯 capability 当作意图
4. 现有 group 级图片能力判断、路由选择和计费逻辑保持不变

## Test Plan

### Backend

1. `account.go` helper tests
   - 缺省值按开启
   - 显式 `false` 按关闭
   - 非 OpenAI 账号忽略该字段

2. `openai_account_scheduler_test.go`
   - `/v1/images` 请求跳过关闭生图账号
   - `/v1/responses` 显式生图请求跳过关闭生图账号
   - 文本请求不会因为账号关闭生图而被排除
   - 仅声明 `image_generation` capability 的请求不会强制过滤账号
   - 当候选只剩关闭生图账号时，结果为调度失败而非权限错误

3. handler / service tests
   - 保持 group 级禁生图仍为 `403`
   - 验证账号级禁生图通过调度体现为“无可用账号 / 服务不可用”
   - 验证 WebSocket 首包显式生图时不会选中禁生图账号

### Frontend

1. `CreateAccountModal.spec.ts`
   - 默认开启
   - 关闭后 payload 写入 `extra.openai_image_generation_enabled=false`

2. `EditAccountModal.spec.ts`
   - 能从 DTO 显式字段回填
   - 保存时只更新目标 key

3. `BulkEditAccountModal` tests
   - 启用该项后写入 `updates.extra.openai_image_generation_enabled`
   - 不误删其他 `extra` 键

## Non-Goals

1. 不新增数据库实体列
2. 不新增账号列表筛选项或表格徽标
3. 不修改分组级生图开关语义
4. 不改变 group 级禁生图返回 `403` 的行为
5. 不把“仅具备图片工具能力”的请求一律视为生图请求
