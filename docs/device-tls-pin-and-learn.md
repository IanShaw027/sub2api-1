# 设备 TLS Pin 与官方学习

每个上游账号一行 `account_device_profiles`：ClientHello 钉在 `tls_profile_id`，官方流量只升级同一行的软件包。出站只读这一行，不再按请求路由指纹。

设计摘要：[`docs/superpowers/specs/2026-08-17-device-tls-pin-once-design.md`](superpowers/specs/2026-08-17-device-tls-pin-once-design.md)。

学习关时首建仍按平台基线自动钉，且后续把学习关掉不会重铸也不会清掉当前 pin；学习开时可选填完整可用目录项，不选仍在首建时自动钉，换选会整套重铸设备身份；列表显示 `name`、平台 / family、OS + 传输、当前软件包、`description`，手选 id 记在 `extra.device_tls_profile_id`：见已实现规格 [`docs/superpowers/specs/2026-08-17-device-tls-catalog-select-design.md`](superpowers/specs/2026-08-17-device-tls-catalog-select-design.md)。

## 两套东西，两种更新节奏

| | 软件包 | TLS 指纹 |
|---|---|---|
| 存哪 | 同一行的 `client_version` / UA / Stainless | 同一行的 `tls_profile_id` → `tls_fingerprint_profiles` |
| 何时变 | 官方 CLI 打进来，且编译期登记表已有该版本 | **不会**随版本学习自动变 |
| 后期怎么升 | 改编译期 pin，部署后再用官方 CLI 打一次 | **原地更新**已有 `pin:…` 目录行（id 不变） |

CLI 改版本很勤，TLS 栈（Node / rustls）很少改。入站也不学握手：Grok rustls 每次扩展顺序会随机。

## 数据模型

`account_device_profiles` 一行 = 一份身份：

- 钉死：`device_id`、`installation_id`、`os_family`、`arch`、`tls_profile_id`、`transport_family`
- 可学：`client_version`、`profile_payload.user_agent`、Stainless 软件字段、`learned_from`、`version_upgraded_at`

目录名只在选模板时用一次：

```text
pin:{client_family}:{os_family}:{transport}
```

例如 `pin:claude-code:macos:h1`、`pin:codex-cli:linux:h1`、`pin:grok-cli:macos:h1`。

新行的 `os_family` 来自基线，不是入站 UA：

| 平台 | 基线 OS / arch |
|---|---|
| Anthropic | linux / arm64 |
| OpenAI | linux / x64 |
| Grok、Kiro | macos / arm64 |
| Gemini、Antigravity | windows / x64 |

已有行的 OS 不会被学习改掉。第一次 `GetOrCreate` 若 `tls_profile_id` 为空，按该行的 family + OS + transport 补钉，之后不再改。

## 出站

1. 有设备行且已钉 `tls_profile_id` → 只用这个 id。
2. 有设备行但未钉 → 不回退 router / bindings / 入站 UA。
3. 没有设备服务时才走旧 extra（单测）。

学习写的是**编译期 bundle**，不是入站原样。官方 Claude 入站可能是 `(external, sdk-cli)` + Node v26 + Darwin；出站仍是登记表的 `(external, cli)` + 登记表 Stainless。OS/arch 身份列保持账号自己的值。

## 学习门闩

总开关：账号 `extra.device_learning_enabled = true`，或该行 `learning_enabled = true`。**不要默认全库打开。**

官方 UA（前缀必须对上）：

| 平台 | 会学 | 不学 |
|---|---|---|
| Anthropic | `claude-cli/x.y.z` | 其它 |
| OpenAI | 仅 `codex_cli_rs/`、`codex-tui/` | `codex exec`、`codex_vscode/`、只靠 originator |
| Grok | `xai-grok-workspace/`、`grok-pager/`、`grok-shell/` | 其它 |
| Gemini / Antigravity | 各自官方 UA | 其它 |

版本候选：`version` / `X-Client-Version` / `x-grok-client-version`，否则从 UA 取。UA 里的版本若和候选不一致，整次跳过。候选必须能在软件登记表 `Lookup` 到，且 semver **高于**当前行。

Claude 额外：UA 将要变时，入站 Stainless 的 `lang` / `package_version` / `runtime` 必须等于登记表。`os` / `arch` / `runtime_version` 不是门闩。官方 Claude 2.1.233 的 package 是 `0.112.1`；登记表停在旧值时，官方流量会 `ua_changed_without_stainless` 直接跳过。

Grok 的 runtime 必须是 `grok-shell`（基线如此）。学习钩子在：

- `gateway_forward.go`（Claude `/v1/messages`）
- `openai_gateway_forward.go`（`/v1/responses`）
- `openai_gateway_chat_completions.go`（Grok / OpenAI `/v1/chat/completions`）
- Gemini / Antigravity 兼容转发、`gateway_count_tokens.go`

WS 重连（`openai_ws_forwarder_v2.go`）不再学。选号失败发生在 Forward 之前时，学习不会跑。

## 当前编译期 pin（2026-08-17）

| 客户端 | 常量 | 值 |
|---|---|---|
| Claude CLI | `claude.CLICurrentVersion` | `2.1.233` |
| Claude Stainless | `claude.CLIStainlessPackageVersion` | `0.112.1` |
| Codex | `codexCLIVersion` | `0.147.0`（不要把 0.148 alpha 当 pin） |
| Grok | `xai.CLIClientVersion` | `1.0.4`（对齐 `https://x.ai/cli/stable`） |

`identity_service` 的默认指纹必须和 Claude `DefaultHeaders` 用同一套常量。

测试机已验证的账号（pin-once + 学习）：

| 账号 | 客户端 | 软件 | `tls_profile_id` |
|---|---|---|---|
| 80672 | Claude | 2.1.233 | 25 `pin:claude-code:macos:h1` |
| 81600 | Codex | 0.147.0 | 27 `pin:codex-cli:linux:h1` |
| 81481 | Grok | 1.0.4 | 29 `pin:grok-cli:macos:h1` |

Linux / macOS 的 Claude 瘦字段可以相同；**Codex Linux ≠ macOS**（Linux 30 cipher + TLS1.3/ML-KEM，macOS rustls 22 cipher）。不要把一边的 hello 克隆到另一边。Grok rustls 扩展集合相同、顺序随机。

## 后期：升软件版本

1. 确认官方稳定版（Claude `claude --version`，Codex 不要用 alpha，Grok 看 `https://x.ai/cli/stable`）。
2. 对 Claude：用隔离 settings 打本地 dump，记下真实 `User-Agent` 和 `X-Stainless-Package-Version`。官方 2.1.233 不发 `version` 头，UA 是 `(external, sdk-cli)`。
3. 改编译期常量（上表）。Claude 只改常量，`DefaultHeaders` / `identity_service` 会跟着走。
4. 登记表由 `compileTimeSoftwareBundles()` 从这些常量生成，**不要**另插一条重复版本。
5. 跑相关单测：`go test -tags=unit ./internal/service -run 'TestLearnIfOfficial|TestMaybeLearnOfficial'`。
6. 部署后**重启**进程。用官方 CLI 打已打开学习的账号。
7. 验收：`client_version` 升到新 pin，`tls_profile_id` / `device_id` / `os_family` 不变，`learned_from=official_traffic`。Claude 出站 UA 仍是 `(external, cli)`，不是入站 `sdk-cli`。

没有打开学习的账号，出站软件跟编译期基线走，但已钉的 TLS 仍用旧 `tls_profile_id`。

## 后期：TLS 握手变了

学习**不会**改 `tls_profile_id`。正确做法是改目录、不改账号：

1. 按下面「采集」再抓官方 ClientHello（Linux / macOS 分开，除非瘦字段确认相同）。
2. **UPDATE 原行**，保持 `id` 和 `name` 不变。所有钉着这个 id 的账号下次出站自动用新握手。
3. 改完 SQL 后**必须重启** `sub2api`：目录缓存在进程内。
4. 不要 INSERT 一条新 `pin:…` 再指望学习去切 id。只有改 `tls_profile_id` 或重铸设备行才会换指纹。

personal-main 只存瘦字段：`cipher_suites`、`curves`、`point_formats`、`signature_algorithms`、`alpn_protocols`、`supported_versions`、`key_share_groups`、`psk_modes`、`extensions`、`enable_grease`。不从网关入站学握手，不把 HTTP/2 指纹或 UA 写进目录。

rustls 常不带 ALPN，采集器导入会报 `alpn_protocols is required`。入库时写成 `["http/1.1"]`。没有 `import-captures` 管道。

## 采集

采集器 UI：`https://tls.clomio.ai`。**不要点「推送到 Sub2API」**（会写到允许的生产主机）。

Capture：`https://tls.clomio.ai:18444/capture/{claude,openai/v1,gemini,grok/v1,kiro,antigravity}`。

必须隔离用户日常配置，否则会打到生产网关：

- Claude：`--bare --settings`，settings 里只写采集/测试 `ANTHROPIC_BASE_URL`。不要用 `~/.claude/settings.json`。
- Codex：`-c model_provider=…`，不要用默认 `sub2api` / 生产 base URL。
- Grok：独立 `HOME`，`[auth] preferred_method = "api_key"`，并覆盖 `[endpoints]` / `GROK_CLI_CHAT_PROXY_BASE_URL`。不要把这段写进用户日常 `~/.grok/config.toml`，除非就要走 Sub2API API key。不要把 `~/.grok/auth.json` 拷进 Docker。

本地 rustls peek 可用 `/tmp/tls-hello-capture/`（`CAPTURE_RAW=1` 只留 raw）。macOS 没有 `timeout`，用 `perl -e 'alarm N; exec @ARGV'`。

## 打开学习

按账号写 `accounts.extra.device_learning_enabled = true`。改 extra 后若调度缓存仍是旧对象，重启一次网关。

验证学习时，把该行 `client_version` 降到登记表以下再打官方流量；基线已经是当前 pin 时，学习会因「版本没有更高」跳过。

## 验收清单

- [ ] 官方 CLI 对测试网关 200，回复正常。
- [ ] `client_version` / UA / Stainless 升到编译期 pin。
- [ ] `tls_profile_id`、`device_id`、`os_family` 未变。
- [ ] Claude 出站 UA 是登记表 `(external, cli)`。
- [ ] 目录 SQL 之后进程已重启。
- [ ] 临时 `schedulable=false` 的实验账号已恢复。

## 坑

- **Claude Stainless 过期**：只升 CLI 版本、不升 `CLIStainlessPackageVersion`，官方流量学不上去。
- **Codex `exec` 不学**：要用 TUI / `codex_cli_rs` UA，或 curl 带 `codex-tui/x.y.z`。
- **Grok 走 chat completions**：钩子在 `ForwardAsChatCompletions`，不在 `Forward()`。
- **选号失败无学习**：账号全死、模型不在组里、代理不通，都到不了 Forward。
- **测试机 `127.0.0.1` 出口**：导入的 WARP/local-egress 在测试机上不存在。无代理直连可能让 x.ai 吊销 OAuth（81524 已发生）。
- **采集打到生产**：没隔离 Claude/Codex/Grok 的默认 base URL。
- **不要**把 Codex Linux hello 当成 macOS，或反过来。

## 代码入口

| 职责 | 位置 |
|---|---|
| Pin 名 / 补钉 | `backend/internal/service/account_device_tls_pin.go` |
| GetOrCreate、LearnIfOfficial、基线 OS | `account_device_service.go` |
| 入站观察 | `account_device_learn_inbound.go` |
| 软件登记表 | `software_bundle_registry.go` |
| 出站钉死 runtime | `account_tls_fingerprint.go`（`pinnedDeviceTLSRuntime`） |
| 目录缓存 / `lookupPin` | `tls_fingerprint_profile_service.go` |
| Claude 版本 + Stainless | `backend/internal/pkg/claude/constants.go` |
| Codex 版本 | `openai_gateway_service.go`（`codexCLIVersion`） |
| Grok 版本 + UA | `backend/internal/pkg/xai/billing.go`、`cli_identity.go` |
