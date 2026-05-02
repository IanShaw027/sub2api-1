# AI 创作中心使用 / 接口文档

本文档把 AI 创作中心、OpenAI 兼容接口、`codex-image` 适配、MinIO 与 `source.qazwc.com` 资源链路串成一份可执行说明。

## 1. 核心结论

- 前台只选线路，不选具体账号 Key。
- `/user/ai/runtime` 用来拉取可选线路。
- `/user/ai/chat`、`/user/ai/artworks`、`/user/ai/gallery` 是创作中心主链路。
- Prompt 库由用户侧 `prompt-templates` / `prompts` 和管理员治理接口组成。
- `chat/completions`、`responses`、`images/generations`、`images/edits`、`images2api/*` 统一由后端路由层承接。
- `codex-image` 是 OAuth / Responses 图片工具链适配，不是前台可直接选择的 Key。
- MinIO 统一承载头像、公告、工单、AI、论坛图片，外部统一通过 `source.qazwc.com` 访问。

## 2. AI 创作中心用户流程

1. 前端先调用 `/api/v1/user/ai/runtime` 获取可用线路。
2. 用户选择线路或分组，而不是选择 Key。
3. 用户进入聊天、Prompt 库、图片生成或画廊入口。
4. 结果图片和附件写入 MinIO，再通过 `source.qazwc.com` 暴露。
5. 若需要公开展示，再进入管理员治理或发布流程。

## 3. 当前最终生效的 AI 路由集合

当前主 router 由 `RegisterUserRoutes` / `RegisterAdminRoutes` 挂载 AI 路由，`RegisterAIRoutes` 仅保留作兼容包装器。就主服务最终暴露的 AI 路由而言，当前生效集合是：

### 3.1 用户侧

- `GET /api/v1/user/ai/runtime`
- `GET /api/v1/user/ai/sessions`
- `POST /api/v1/user/ai/sessions`
- `GET /api/v1/user/ai/sessions/:id`
- `PUT /api/v1/user/ai/sessions/:id`
- `DELETE /api/v1/user/ai/sessions/:id`
- `GET /api/v1/user/ai/sessions/:id/messages`
- `POST /api/v1/user/ai/sessions/:id/messages`
- `GET /api/v1/user/ai/prompt-templates`
- `POST /api/v1/user/ai/prompt-templates`
- `GET /api/v1/user/ai/prompt-templates/:id`
- `PUT /api/v1/user/ai/prompt-templates/:id`
- `DELETE /api/v1/user/ai/prompt-templates/:id`
- `GET /api/v1/user/ai/prompts`
- `POST /api/v1/user/ai/prompts`
- `PUT /api/v1/user/ai/prompts/:id`
- `DELETE /api/v1/user/ai/prompts/:id`
- `POST /api/v1/user/ai/prompts/:id/clone`
- `GET /api/v1/user/ai/generation-jobs`
- `POST /api/v1/user/ai/generation-jobs`
- `GET /api/v1/user/ai/generation-jobs/:id`
- `POST /api/v1/user/ai/chat`
- `GET /api/v1/user/ai/artworks`
- `POST /api/v1/user/ai/artworks`
- `POST /api/v1/user/ai/artworks/edit`
- `PUT /api/v1/user/ai/artworks/:id`
- `DELETE /api/v1/user/ai/artworks/:id`
- `GET /api/v1/user/ai/gallery`
- `GET /api/v1/user/ai/assets/:id`

### 3.2 管理侧

- `GET /api/v1/admin/ai/prompt-templates`
- `GET /api/v1/admin/ai/prompt-templates/:id`
- `PUT /api/v1/admin/ai/prompt-templates/:id/moderation`
- `GET /api/v1/admin/ai/prompts`
- `PUT /api/v1/admin/ai/prompts/:id`
- `DELETE /api/v1/admin/ai/prompts/:id`
- `GET /api/v1/admin/ai/generation-jobs`
- `GET /api/v1/admin/ai/generation-jobs/:id`
- `PUT /api/v1/admin/ai/generation-jobs/:id/status`
- `GET /api/v1/admin/ai/assets`
- `GET /api/v1/admin/ai/assets/:id`
- `PUT /api/v1/admin/ai/assets/:id/moderation`
- `GET /api/v1/admin/ai/artworks`
- `PUT /api/v1/admin/ai/artworks/:id`
- `DELETE /api/v1/admin/ai/artworks/:id`
- `GET /api/v1/admin/ai/audit-logs`

说明：

- `RegisterAIRoutes` 只是兼容包装器，生产契约以 `RegisterUserRoutes` 和 `RegisterAdminRoutes` 的挂载结果为准。
- 文档以下所有“用户侧/管理侧主接口”都以这组最终暴露路径为准。

## 4. `/user/ai/runtime`

用途：

- 返回前台可选的 AI 线路列表。
- 前端用于驱动线路选择、能力展示和降级逻辑。

当前前端归一化后的字段语义：

- `group_id`
- `label`
- `platform`
- `description`
- `keys`（`id` + `name`）
- `key_ids`
- `key_count`
- `default_key_id`

说明：

- 这个接口是前台入口，不是账号明细接口。
- 用户不应看到真实 OpenAI 账号 Key。

## 5. 用户侧主接口

### 5.1 `/user/ai/chat`

- 用于 AI 对话入口。
- 发送 `prompt`，可附带 `line_id`、`prompt_template_id`、`history`、`use_responses`。
- 后端会把请求映射到 OpenAI 兼容链路，再选择可用账号。

### 5.2 `/user/ai/artworks`

- 用于创建、更新、删除用户作品。
- `POST /user/ai/artworks` 用于创建作品。
- `PUT /user/ai/artworks/:id` 用于编辑作品元信息。
- `DELETE /user/ai/artworks/:id` 用于删除作品。

### 5.3 `/user/ai/gallery`

- 用于拉取用户画廊列表。
- 是作品集合视图，和 `/user/ai/artworks` 共享同一业务域。

### 5.4 `/user/ai/prompt-templates`

- `prompt-templates` 是当前主 router 已生效的结构化模板库接口。
- 适合创作中心编辑器和模板治理。
- Prompt 默认按私有优先处理。
- `forced_private` 和 `blocked` 会把有效可见性压成私有。

### 5.5 `/user/ai/prompts`

- `prompts` 是用户侧兼容 Prompt 库接口，但当前生产 router 已实际暴露。
- 它和 `prompt-templates` 一起构成前台可见的 Prompt 库入口。
- 管理侧对应 `/api/v1/admin/ai/prompts`。

## 6. OpenAI 兼容链路

### 6.1 `/v1/chat/completions`

- 传统 OpenAI 文本兼容入口。
- 适合第三方 SDK 和旧客户端。

### 6.2 `/v1/responses`

- 新版 OpenAI 兼容入口。
- 图片工具链、Codex 路线和内部适配优先走这里。

### 6.3 `/v1/images/generations`

- 标准图片生成入口。
- AI 创作中心里的 `codex` 线路直接走这一条，不再绕到 `/v1/responses`。
- API Key 账号走标准兼容转发。

### 6.4 `/v1/images/edits`

- 标准图片编辑入口。
- 本地上传会内联成 `data:` URL。
- 外部图片 URL 会保留原始地址透传。

### 6.5 `/v1/images2api/*`

- 旧版 web2api 兼容入口。
- AI 创作中心里的 `2api` 线路直接走这一条，不再绕到 `/v1/responses`。
- 与标准 `images` 分开计价。

### 6.6 `codex-image`

- `image_generation` 工具请求会追加适配说明。
- Spark 模型不承诺图片生成能力。
- OAuth 图片请求强制 `store=false`。

## 7. 管理员治理流程

1. 管理员维护 OpenAI OAuth / API Key 账号池。
2. 管理员把账号绑定到线路分组。
3. 管理员使用 Prompt 模板治理、资产审核、作品审核和审计日志。
4. `forced_private`、`blocked`、私有资源、工单附件等都按受控访问处理。
5. 公开资源统一写入 MinIO，并通过 `source.qazwc.com` 访问。

### 7.1 Prompt 治理

- 用户侧主契约：`/api/v1/user/ai/prompt-templates`、`/api/v1/user/ai/prompts`
- 管理侧：`/api/v1/admin/ai/prompt-templates`、`/api/v1/admin/ai/prompts`
- 审核态：`normal`、`forced_private`、`blocked`

### 7.2 作品 / 资产治理

- 用户侧：`/api/v1/user/ai/artworks`、`/api/v1/user/ai/gallery`、`/api/v1/user/ai/assets/:id`
- 管理侧：`/api/v1/admin/ai/assets`、`/api/v1/admin/ai/artworks`

## 8. MinIO / `source.qazwc.com`

- MinIO 统一承载头像、公告、工单、AI、论坛图片。
- 对外统一使用 `source.qazwc.com`。
- 不直接暴露 MinIO 原始地址。
- 私有资源使用短时效签名或业务代理。

## 9. 当前实现差异与注意事项

- `prompts` 已是生产用户路由的一部分，不应再按“仅兼容参考”处理。
- `POST /api/v1/user/ai/artworks/edit` 已在生产路由中直接可用，前端 `editArtwork()` 也直接调用它。
- `/api/v1/user/ai/chat` 接受 `use_responses` 字段；前端会透传入口选择，后端会在 Chat 处理里归一化到 responses 入口。

## 10. 验收清单

1. `/user/ai/runtime` 能返回可选线路。
2. `/user/ai/chat`、`/user/ai/prompts`、`/user/ai/artworks`、`/user/ai/artworks/edit`、`/user/ai/gallery` 都已挂到主 router。
3. `editArtwork()` 直接 POST `/user/ai/artworks/edit`。
4. Prompt 的 `forced_private` 能把有效可见性压成私有。
5. 作品和附件最终只通过 `source.qazwc.com` 暴露。
