# OpenAI 兼容与 AI 创作中心接入手册

本文档面向 OpenAI 兼容与 AI 创作中心接入场景，聚焦当前仓库已经能从代码和测试中落地的事实，统一说明 OpenAI 文本/图片兼容接口、AI 创作中心业务接口、`codex-image` 适配、提示词与媒体可见性、计费尺寸口径，以及用户/管理员/运维操作流程。

## 1. 结论先行

- 前台只选线路或分组，不选具体账号 Key。
- OpenAI OAuth 图片账号用于前台 AI 创作中心和 `codex-image`/Codex 路线。
- OpenAI API Key 图片账号用于标准 OpenAI 兼容客户端和原生 `images` API。
- `images2api` 与标准 `images` 线路分开计价。
- 当前仓库里，`prepare` 阶段固定 `model=auto`，最终发图对话才按 web 会话 `plan_type` 选择模型。
- 当前仓库里，`free` 的最终 web 会话模型仍是 `auto`，`plus` / `pro` / `team` 会切到 `gpt-5-5-thinking`。
- Prompt 与媒体都按私有优先设计；`forced_private` 会把有效可见性压成私有。
- MinIO 统一承载头像、公告、工单、AI、论坛图片。
- 资源访问域名应使用部署环境配置的公开媒体域名，不向前台暴露 MinIO 内网地址或桶域名。

## 2. 账号与线路规范

### 2.1 前台规则

- 前台只展示线路、分组或显示名，不展示底层账号、Key、桶地址。
- 账号选择、能力匹配、隐私校验、故障切换和负载均衡都由后端调度层处理。
- 即使兼容请求体里存在 `key_id` 字段，也只作为历史兼容或管理用途保留，不应在正式前台交互中暴露给用户。

### 2.2 推荐线路命名

- `codex`: OpenAI OAuth + `/v1/responses` + `image_generation` 工具链。
- `web2api`: OpenAI OAuth legacy 图片桥，兼容 `/v1/images2api/*`。
- `openai-apikey`: 标准 OpenAI API Key 图片兼容线路。

### 2.3 图片链路约定

| 场景 | 入口 | 账号类型 | 实际链路 |
| --- | --- | --- | --- |
| OpenAI 原生图片生成/编辑 | `/v1/images/generations` / `/v1/images/edits` | OAuth | 转成 Codex/Responses `image_generation` 工具链 |
| OpenAI 旧版 web2api 图片桥 | `/v1/images2api/generations` / `/v1/images2api/edits` | OAuth | 走 legacy web2api bridge |
| OpenAI 原生图片生成/编辑 | `/v1/images/generations` / `/v1/images/edits` | API Key | 走标准 OpenAI Images 转发 |
| OpenAI 旧版 web2api 图片桥 | `/v1/images2api/*` | API Key | 不支持；显式端点要求 OAuth，不会回落到标准 Images 转发 |

### 2.4 账号治理规则

- OAuth 图片账号优先分配给前台 AI 创作中心、Codex 图片工具、需要 ChatGPT 内部图片能力的链路。
- API Key 图片账号优先分配给第三方 OpenAI SDK、脚本、自动化客户端。
- 开启 `require_privacy_set` 的分组，只允许隐私设置完成的账号参与调度。
- 不允许把“资源公开/私有”与“选择哪个账号 Key”绑定在一起。

## 3. OpenAI 兼容调用规范

### 3.1 `/v1/chat/completions`

适用场景：

- 第三方 OpenAI SDK 文本对话。
- 不需要 `responses` 工具链的传统聊天调用。

示例：

```bash
curl https://gateway.example.com/v1/chat/completions \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4",
    "messages": [
      {"role": "user", "content": "hello"}
    ]
  }'
```

接入要求：

- 客户端只需要 API Key，不直接关心线路内的真实上游账号。
- 文本对话与图片生成应按请求类型分别计费和路由，不混用图片尺寸价格字段。

### 3.2 `/v1/responses`

适用场景：

- OpenAI 新版兼容客户端。
- Codex/OAuth 路线。
- 需要 `image_generation` 等工具调用的场景。

纯文本示例：

```bash
curl https://gateway.example.com/v1/responses \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4",
    "input": "hello"
  }'
```

图片工具示例：

```bash
curl https://gateway.example.com/v1/responses \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4",
    "input": "draw a cat in ukiyo-e style",
    "tools": [
      {"type": "image_generation"}
    ],
    "store": false
  }'
```

当前仓库已验证的行为：

- 当请求包含 `image_generation` 工具，且模型不是 `gpt-5.3-codex-spark` 时，网关会追加 `codex-image` 适配说明。
- 对 Spark 模型，网关会注入“不支持图片生成/编辑”的说明。
- OAuth 走 ChatGPT internal API 时，`store` 会被强制改成 `false`，即使客户端显式传入 `true`。

### 3.3 `/v1/images/generations`

适用场景：

- 标准 OpenAI Images 兼容客户端。
- 前台图片创作或后台图片任务的统一入口。

示例：

```bash
curl https://gateway.example.com/v1/images/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "draw a cat",
    "size": "1024x1024",
    "response_format": "b64_json"
  }'
```

接入要求：

- OAuth 路线会转入 `responses`/Codex 图片工具链。
- API Key 路线保持标准 OpenAI `images` 转发。
- `gpt-image-2` 尺寸按当前官方约束处理：最大边长 `<= 3840`，宽高都必须是 `16` 的倍数，长宽比 `<= 3:1`，总像素范围 `655,360 - 8,294,400`。
- 尺寸计费按 `1K`/`2K`/`4K` 统一口径结算。

### 3.4 `/v1/images/edits`

适用场景：

- 基于参考图或蒙版的图像编辑。

示例：

```bash
curl https://gateway.example.com/v1/images/edits \
  -H "Authorization: Bearer sk-xxx" \
  -F "model=gpt-image-2" \
  -F "prompt=replace background with aurora" \
  -F "image=@source.png" \
  -F "mask=@mask.png"
```

当前仓库已验证的行为：

- multipart 本地上传文件会被转成 `data:` URL 内联发送给上游。
- 远端 `image_url` 输入会按原 URL 透传。
- 当客户端请求强隐私链路时，不依赖上游托管公开文件。

### 3.5 `/v1/images2api/generations` 与 `/v1/images2api/edits`

适用场景：

- 旧版 web2api 图片兼容链路。
- 已有客户端不便迁移到标准 `images` 或 `responses` 时的过渡方案。
- 仅限 OAuth 账号；显式端点不会对 API Key 回落成标准 `images`。

示例：

```bash
curl https://gateway.example.com/v1/images2api/generations \
  -H "Authorization: Bearer sk-xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "draw a cat"
  }'
```

落地要求：

- `images2api` 使用独立价格字段，不与标准 `images` 共用计费口径。
- 业务上应把 `images2api` 视为兼容/迁移线路，不作为前台长期默认入口。

## 4. `codex-image` 适配说明

### 4.1 适配目标

- 让 OpenAI OAuth 账号承担前台 AI 创作中心和 Codex 图片工具链。
- 让前台与第三方客户端仍能通过 OpenAI 兼容格式调用图片功能。

### 4.2 当前已验证的契约

- 网关会为图片工具请求追加 bridge/compat 说明，而不是要求前台理解内部实现细节。
- `store=false` 是私有优先的强制规则，不允许前台通过请求体绕过。
- Spark Codex 模型不被宣称具备图片生成能力。

### 4.3 前台为什么只能选线路

- `codex-image` 是线路能力，不是用户可直接选择的账号能力。
- 成功率取决于账号类型、隐私状态、模型映射、分组策略和调度结果。
- 因此前台只应暴露 `line_id`、`group_id` 或对应显示名，不应暴露 `key_id`。

## 5. AI 创作中心业务接口

### 5.1 统一原则

- 用户态业务接口统一位于 `/api/v1/user/ai/*`。
- 管理态治理接口统一位于 `/api/v1/admin/ai/*`。
- 资源接口统一位于 `/api/v1/media/*` 和 `/api/v1/admin/media/*`。
- `group_id` 或 `line_id` 代表线路选择；`key_id` 仅作兼容保留字段。

### 5.2 用户侧接口

#### 会话与消息

| 方法 | 路径 | 用途 | 关键字段 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/user/ai/sessions` | 会话列表 | `status`、分页 |
| `POST` | `/api/v1/user/ai/sessions` | 创建会话 | `title`、`system_prompt`、`metadata`、`trace` |
| `GET` | `/api/v1/user/ai/sessions/:id` | 查看会话 | 路径参数 `id` |
| `PUT` | `/api/v1/user/ai/sessions/:id` | 更新会话 | `title`、`status`、`system_prompt`、`metadata` |
| `DELETE` | `/api/v1/user/ai/sessions/:id` | 删除会话 | 路径参数 `id` |
| `GET` | `/api/v1/user/ai/sessions/:id/messages` | 会话消息列表 | 分页 |
| `POST` | `/api/v1/user/ai/sessions/:id/messages` | 发送消息 | `content` 必填，可带 `model`、`provider`、`content_parts`、`trace` |

消息发送示例：

```json
{
  "content": "帮我写一个海报提示词",
  "model": "gpt-5.4",
  "create_assistant_reply": true,
  "assistant_reply_model": "gpt-5.4",
  "trace": {
    "group_id": 12
  }
}
```

#### Prompt 模板与 Prompt 库

| 方法 | 路径 | 用途 | 关键字段 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/user/ai/prompt-templates` | 模板列表 | `scope`、`visibility`、`moderation_state`、`search`、`group_id` |
| `POST` | `/api/v1/user/ai/prompt-templates` | 新建模板 | `title`、`content` 必填，可带 `visibility`、`model_hint`、`tags`、`variables` |
| `GET` | `/api/v1/user/ai/prompt-templates/:id` | 查看模板 | 路径参数 `id` |
| `PUT` | `/api/v1/user/ai/prompt-templates/:id` | 更新模板 | 可更新标题、内容、可见性、模型提示、变量等 |
| `DELETE` | `/api/v1/user/ai/prompt-templates/:id` | 删除模板 | 路径参数 `id` |
| `GET` | `/api/v1/user/ai/prompts` | Prompt 列表 | `mine_only`、`visibility`、`status`、`line_id` |
| `POST` | `/api/v1/user/ai/prompts` | Prompt 创建 | `title`、`content` 必填，可带 `visibility`、`status`、`line_id` |
| `PUT` | `/api/v1/user/ai/prompts/:id` | Prompt 更新 | `title`、`content`、`visibility`、`status`、`line_id` |
| `DELETE` | `/api/v1/user/ai/prompts/:id` | 删除 Prompt | 路径参数 `id` |
| `POST` | `/api/v1/user/ai/prompts/:id/clone` | 克隆 Prompt | 路径参数 `id` |

模板创建示例：

```json
{
  "title": "电商海报提示词",
  "content": "为{{product}}生成一张主图海报，风格是{{style}}",
  "visibility": "private",
  "model_hint": "gpt-image-2",
  "tags": ["poster", "ecommerce"],
  "variables": [
    {"name": "product", "required": true},
    {"name": "style", "required": true}
  ],
  "trace": {
    "group_id": 12
  }
}
```

#### 生成任务、聊天与作品

| 方法 | 路径 | 用途 | 关键字段 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/user/ai/generation-jobs` | 生成任务列表 | `status`、`session_id`、`prompt_template_id`、`group_id` |
| `POST` | `/api/v1/user/ai/generation-jobs` | 创建生成任务 | `model`、`prompt` 必填，可带 `negative_prompt`、`size`、`image_count`、`seed`、`trace` |
| `GET` | `/api/v1/user/ai/generation-jobs/:id` | 查看生成任务 | 路径参数 `id` |
| `POST` | `/api/v1/user/ai/chat` | 聊天入口 | `prompt` 必填，可带 `session_id`、`line_id`、`prompt_template_id`、`history`、`use_responses` |
| `GET` | `/api/v1/user/ai/artworks` | 作品列表 | 与 `/gallery` 同类输出 |
| `POST` | `/api/v1/user/ai/artworks` | 创建作品 | `prompt` 必填，可带 `line_id`、`size`、`style`、`visibility` |
| `POST` | `/api/v1/user/ai/artworks/edit` | 编辑作品 | `prompt` 必填，可带 `mode=edit`、`line_id`、`key_id`、`source_image`、`mask_image`、`size`、`style` |
| `PUT` | `/api/v1/user/ai/artworks/:id` | 更新作品 | `title`、`visibility`、`status`、`featured`、`tags` |
| `DELETE` | `/api/v1/user/ai/artworks/:id` | 删除作品 | 路径参数 `id` |
| `GET` | `/api/v1/user/ai/gallery` | 画廊列表 | 同作品列表 |
| `GET` | `/api/v1/user/ai/assets/:id` | 查看素材/作品资产 | 路径参数 `id` |

作品创建示例：

```json
{
  "prompt": "生成一张极简海报，突出玻璃质感和蓝色霓虹边缘光",
  "visibility": "private",
  "line_id": 12,
  "size": "1024x1024",
  "style": "editorial"
}
```

`use_responses` 说明：

- 前端会把入口选择透传到 `/api/v1/user/ai/chat`。
- 后端 `Chat` 处理会把 `use_responses` 归一化为 `true`，再统一走 responses 入口。

### 5.3 管理员侧接口

#### Prompt 与模板治理

| 方法 | 路径 | 用途 | 关键字段 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/admin/ai/prompt-templates` | 模板治理列表 | `scope`、`visibility`、`moderation_state`、`search` |
| `GET` | `/api/v1/admin/ai/prompt-templates/:id` | 查看模板 | 路径参数 `id` |
| `PUT` | `/api/v1/admin/ai/prompt-templates/:id/moderation` | 审核模板 | `moderation_state`、`reason` |
| `GET` | `/api/v1/admin/ai/prompts` | Prompt 库治理列表 | `scope`、`status`、`moderation_state`、`line_id` |
| `PUT` | `/api/v1/admin/ai/prompts/:id` | 更新 Prompt | 标题、内容、可见性、状态、线路 |
| `DELETE` | `/api/v1/admin/ai/prompts/:id` | 删除 Prompt | 路径参数 `id` |

审核示例：

```json
{
  "moderation_state": "forced_private",
  "reason": "包含内部品牌素材，允许保留但不得公开"
}
```

#### 生成任务、资产、作品与审计

| 方法 | 路径 | 用途 | 关键字段 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/admin/ai/generation-jobs` | 任务治理列表 | `status`、`session_id`、`prompt_template_id` |
| `GET` | `/api/v1/admin/ai/generation-jobs/:id` | 查看任务 | 路径参数 `id` |
| `PUT` | `/api/v1/admin/ai/generation-jobs/:id/status` | 干预任务 | `status`、`prompt`、`size`、`error_message` 等 |
| `GET` | `/api/v1/admin/ai/assets` | 资产治理列表 | `status`、`visibility`、`moderation_state`、`generation_job_id` |
| `GET` | `/api/v1/admin/ai/assets/:id` | 查看资产 | 路径参数 `id` |
| `PUT` | `/api/v1/admin/ai/assets/:id/moderation` | 审核资产 | `moderation_state`、`reason` |
| `GET` | `/api/v1/admin/ai/artworks` | 作品治理列表 | `status`、`visibility`、`moderation_state`、`line_id`、`featured` |
| `PUT` | `/api/v1/admin/ai/artworks/:id` | 更新作品 | `title`、`visibility`、`status`、`featured`、`tags` |
| `DELETE` | `/api/v1/admin/ai/artworks/:id` | 删除作品 | 路径参数 `id` |
| `GET` | `/api/v1/admin/ai/audit-logs` | 审计日志列表 | `entity_type`、`entity_id`、`action`、`request_id`、`operator_user_id` |

### 5.4 媒体与资源接口

#### 用户侧媒体接口

| 方法 | 路径 | 用途 | 关键字段 |
| --- | --- | --- | --- |
| `POST` | `/api/v1/media/upload` | 上传资源 | multipart `file` 必填，可带 `thumbnail`、`biz_type`、`biz_id`、`visibility` |
| `GET` | `/api/v1/media/:id` | 获取资源元数据 | 路径参数 `id` |
| `DELETE` | `/api/v1/media/:id` | 删除资源 | 路径参数 `id` |
| `POST` | `/api/v1/media/:id/visibility` | 切换公开/私有 | JSON `visibility` |
| `POST` | `/api/v1/media/:id/presign-download` | 获取下载链接 | 路径参数 `id` |
| `GET` | `/api/v1/media/public/:id` | 公开资源直出 | 路径参数 `id` |
| `GET` | `/api/v1/media/public/:id/thumbnail` | 公开缩略图直出 | 路径参数 `id` |
| `GET` | `/api/v1/media/download/:id` | 签名下载直出 | 需要 `expires`、`sig` |
| `GET` | `/api/v1/media/download/:id/thumbnail` | 签名缩略图直出 | 需要 `expires`、`sig` |

私有缩略图下载与原图共用同一签名机制，只是路径末尾增加 `/thumbnail`。

上传示例：

```bash
curl https://gateway.example.com/api/v1/media/upload \
  -H "Authorization: Bearer user-jwt" \
  -F "file=@poster.png" \
  -F "thumbnail=@poster-thumb.png" \
  -F "biz_type=ai" \
  -F "biz_id=job_1001" \
  -F "visibility=private"
```

#### 管理员侧媒体接口

| 方法 | 路径 | 用途 | 关键字段 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/admin/media` | 资源治理列表 | `biz_type`、`biz_id`、`visibility`、`status`、`owner_user_id` |
| `POST` | `/api/v1/admin/media/upload` | 管理上传资源 | 用户态字段外，额外支持 `owner_user_id` |
| `GET` | `/api/v1/admin/media/:id` | 查看资源 | 路径参数 `id` |
| `DELETE` | `/api/v1/admin/media/:id` | 删除资源 | 路径参数 `id` |
| `POST` | `/api/v1/admin/media/:id/visibility` | 切换资源可见性 | JSON `visibility` |
| `POST` | `/api/v1/admin/media/:id/presign-download` | 获取下载链接 | 路径参数 `id` |

## 6. 提示词与媒体公开/私有规范

### 6.1 Prompt 可见性与审核状态

当前代码中存在以下可见性和审核状态：

- Prompt/作品可见性：`private`、`unlisted`、`public`
- 审核状态：`normal`、`forced_private`、`blocked`

有效规则：

- `forced_private` 和 `blocked` 会把 Prompt 的有效可见性压成 `private`。
- 拥有者与管理员可读取自己的私有内容。
- 非拥有者只有在 `moderation_state=normal` 且可见性为 `public` 或 `unlisted` 时才可读。
- Prompt 库公开列表只应展示 `public + normal` 的内容，拥有者查看自己的内容除外。

### 6.2 图片请求私有化规则

当前仓库中已验证的契约：

- OAuth/Codex 图片请求强制 `store=false`。
- 本地上传图片会被转成 `data:` URL 内联发往上游。
- 远端公开图片 URL 会按原 URL 透传。
- 当客户端请求 `response_format=url` 且走私有优先链路时，兼容层不应假设上游会返回长期公开 CDN。

### 6.3 媒体公开/私有规则

- 媒体资源可见性只区分 `public` 和 `private`。
- 公开资源通过 `/api/v1/media/public/:id` 或统一资源域名访问。
- 私有资源通过短时效签名下载或业务代理访问。
- AI 成品图默认应按私有优先处理，公开展示前需经过产品和治理流程。

## 7. 尺寸与计费口径

### 7.1 尺寸分档

- `1K`: `1024x1024`
- `2K`: `1536x1024`、`1024x1536`、`1792x1024`、`1024x1792`，以及未识别尺寸的默认回退
- `4K`: 预留给分组高分辨率价格档位

### 7.2 计费字段

- 标准图片线路使用 `image_price_1k`、`image_price_2k`、`image_price_4k`
- `images2api` 线路使用 `images2api_price_1k`、`images2api_price_2k`、`images2api_price_4k`
- `images2api` 与标准 `images` 不共用价格字段
- 未识别尺寸按 `2K` 回退计费

## 8. 用户、管理员、运维操作流程

### 8.1 用户流程

1. 在前台进入 AI 创作中心。
2. 选择线路或分组，不选择账号 Key。
3. 选择文本聊天、Prompt 模板或图片生成入口。
4. 如需参考图，先上传到统一媒体接口或直接走图片编辑接口。
5. 查看生成结果，按业务需要保存为私有作品或提交公开治理流程。

### 8.2 管理员流程

1. 维护 OpenAI OAuth 和 API Key 账号池。
2. 把账号绑定到线路分组，而不是暴露给用户。
3. 为分组设置标准 `images` 与 `images2api` 两套图片价格。
4. 治理 Prompt、生成任务、资产、作品和审计日志。
5. 对含敏感内容的 Prompt 或作品设置 `forced_private` 或 `blocked`。

### 8.3 运维流程

1. 准备 MinIO、资源桶、生命周期和审计策略。
2. 准备 `<MEDIA_DOMAIN>` TLS、反向代理、缓存和签名下载能力。
3. 检查前台与接口返回值，确认没有泄露 MinIO 内网地址。
4. 定期检查私有资源 TTL、公开资源缓存、跨业务串读和对象回收。

## 9. 运维注意事项

- 不要让前台或第三方客户端直接选择 OpenAI 账号 Key。
- 不要把 `store=true` 当成可保留私有历史的可靠手段。
- 不要让 MinIO 直链、桶名、端口号出现在外部页面或接口返回值里。
- 不要把 `images2api` 价格与标准 `images` 价格混算。
- 不要把 `forced_private` 内容仍然放进公开 Prompt 库或公开画廊。

## 10. 验证清单

### 10.1 已有自动化覆盖点

- 图片线路选择：
  - OAuth + `/v1/images/*` 走 Codex/Responses
  - OAuth + `/v1/images2api/*` 走 legacy bridge
  - API Key + `/v1/images2api/*` 应被视为不支持的显式端点
- `codex-image`：
  - 图片工具请求会追加适配指令
  - Spark 模型不宣称支持图片生成
  - `store=false` 被强制保留
- 提示词与媒体：
  - `forced_private` 强制私有化
  - 本地图片上传转 `data:` URL
  - 公开媒体 URL 保持透传
- 计费：
  - `images2api_price_1k/2k/4k`
  - 未识别尺寸回退 `2K`
- 基础 API 契约：
  - 分组契约返回 `images2api_price_*` 字段

### 10.2 建议人工验收项

1. 用同一前台账号分别选择 `codex` 与 `web2api`，确认页面只显示线路，不显示账号 Key。
2. 调 `/v1/responses` 图片工具链，确认请求被强制 `store=false`。
3. 调 `/v1/images/edits` 上传本地文件，确认不会泄露公开外链。
4. 调 `/api/v1/user/ai/prompt-templates` 创建 `public` Prompt，再由管理员改成 `forced_private`，确认普通用户不再能在库里看到。
5. 调 `/api/v1/media/:id/visibility` 把资源改为 `public`，确认资源可以通过 `<MEDIA_DOMAIN>` 访问。
6. 把同一资源改回 `private`，确认只能通过签名下载或代理访问。
7. 对 `1K`、`2K`、未知尺寸分别提交图片任务，确认计费落到正确价格档。

## 11. 视频生成 (Videos API)

Videos 是 Grok/xAI-only 能力。sub2api 暴露 OpenAI-style 的入站路径只是为了客户端调用形态统一；实际只允许 Grok 分组进入，并透传到 Grok/xAI 视频上游：

- `POST /v1/videos` / `POST /videos` / `POST /videos/generations`
- `GET /v1/videos/...` （用于查询状态、下载内容）

**常见参数**（以 Grok/xAI 上游为准）：
- `prompt`（必填）
- `model`（如控制台开放的 Grok 视频模型）
- `duration`（xAI 官方时长字段，如 `8`；旧客户端 `seconds` 仍兼容）
- `aspect_ratio`（xAI 官方画幅字段，如 `16:9`）
- `resolution`（xAI 官方分辨率字段，如 `720p`/`1080p`；旧客户端 `size` 仍兼容）
- `image` / `reference_images` / `input_reference`（可选，用于图生视频或参考图视频）
- 其他如 `n_variants`

**限制与配置**：
- 仅 Grok platform 的分组可用；OpenAI/Claude/其他分组在路由层返回 `Videos API is not supported for this platform`。
- 分组需 `allow_video_generation: true`
- 计费：按分辨率 tier + `video_price_*_per_sec`（480p/720p/1080p/4k）
- 路由：`video_generation_route` 当前规范化为 `native`
- 调度：使用 `SelectAccountWithSchedulerForCapability(..., videos)` 选择支持 Grok videos 的账号（支持 failover）。

当前实现透传请求到 Grok/xAI 上游，异步 job 由客户端轮询；POST 会从请求/响应提取 `duration`/`resolution` 等视频计费元数据，GET 查询只透传状态/结果，不提取视频生成计费元数据。

客户端应使用 Grok 分组 Key 指向 sub2api base_url 调用 videos。

示例：
```bash
curl https://.../v1/videos/generations \
  -H "Authorization: Bearer sk-xxx" \
  -d '{"model":"grok-imagine-video","prompt":"a cat on piano","duration":8,"aspect_ratio":"16:9","resolution":"720p"}'
```
