# 完整可用 TLS 目录自选（开学习后）

日期：2026-08-17  
分支：personal-main  
状态：已实现  
活文档：[`docs/device-tls-pin-and-learn.md`](../../device-tls-pin-and-learn.md)  
前提：[`2026-08-17-device-tls-pin-once-design.md`](2026-08-17-device-tls-pin-once-design.md)

## 目标

目录里可以挂多套完整官方采集，随时增改。账号打开设备学习后，从本平台「完整可用」的目录里选一套。出站用这一套的全部：TLS、OS/arch、软件包。换选则整套替换并重铸 `device_id` / `installation_id`。

学习仍然只升**当前这一套**的软件，不因入站 UA 换握手、不跨套。

## 非目标

- 不恢复按请求路由 / bindings / 入站 UA 选指纹。
- 不默认全库打开学习。
- 不从网关入站学 ClientHello。
- 不把不完整目录行放进下拉。
- 不在下拉里展示 cipher / ALPN 明细（摘要里有名称、平台、OS、传输、软件包、备注即可）。

## 目录

一行 `tls_fingerprint_profiles` = 一套可出站的官方采集。

命名：

```text
pin:{client_family}:{os_family}:{transport}
pin:{client_family}:{os_family}:{transport}:{variant}
```

`variant` 用来挂同一 family+OS+transport 的第二份样本。自动补钉（未手选）只认不带 variant 的标准名，和今天一样。手选按 **id**，所以多套可以共存。

**完整可用**（必须同时满足才进下拉、才允许保存）：

- `name` 能解析出 `client_family`、`os_family`、`transport`（h1 或 h2）
- `client_family` 属于该账号平台
- `cipher_suites`、`extensions` 非空
- `alpn_protocols` 非空（rustls 无 ALPN 的入库写成 `["http/1.1"]`）
- 该 family 在编译期软件登记表里有当前 pin，能生成出站软件包

后期维护：新增行、或**原地 UPDATE** 已有 id（所有钉着该 id 的账号下次出站自动用新握手）。改完目录必须重启网关（进程内缓存）。

## 账号

| 学习开关 | 选择 | 第一次出站 |
|---|---|---|
| 关 | 不展示下拉 | 按平台基线 OS 自动钉标准名 `pin:{family}:{os}:h1` |
| 开，未选 | 下拉可选，允许空 | 同上，自动钉 |
| 开，已选完整行 | 下拉展示完整摘要 | 按该 **id** 钉；OS/arch/软件包跟这套走，不跟平台基线 |

换选另一套完整行：

1. `Reset` 重铸设备行（新 `device_id`、`installation_id`、`gateway_account_uuid`、`session_namespace` 等）
2. 写入新的 `tls_profile_id`
3. `os_family` / `transport_family` 来自该目录行的 name；`arch` 按 OS 取默认（macos→arm64，linux 用该平台基线 arch，windows→x64）
4. 软件包写成该 family 的当前编译期 bundle
5. 下次出站全部用新套

关掉学习：不重铸、不改 pin。下拉隐藏。已钉的行继续出站。

保存时若选中的行不完整或平台不匹配，拒绝保存。

## 界面

账号编辑里，打开「设备学习」后出现下拉。每条选项和选中摘要都显示：

- 目录 `name`
- 平台 / 客户端 family
- OS + 传输
- 「完整可用」
- 当前登记表软件包（如 Claude 2.1.233 / Stainless 0.112.1）
- `description`（有则显示）

不完整的行不出现。旧的 router / bindings / `tls_fingerprint_default_os` 不再做出站依据（有设备行时本来就不走）。本功能的下拉取代「开了学习还要人手选指纹」的需求，不再让人填裸 id。

## 出站与学习

- 有设备行且已钉 → 只用该 `tls_profile_id`。
- `LearnIfOfficial` 只改当前行软件字段，不改 `tls_profile_id`、不改 OS、不改 device id。
- 人手换套才重铸并换 pin。

## 存储

手选的目录 id 写在账号 `extra.device_tls_profile_id`，出站真相仍是 `account_device_profiles.tls_profile_id`。换选时两者一起更新。未手选时 extra 无此键，GetOrCreate 走自动补钉。

不把选择写进旧的 `extra.tls_fingerprint_profile_id` 路由字段，避免和已废弃的 router 路径混用。

## 验收

- 开学习后下拉只有本平台完整可用行，且每条摘要完整。
- 不选手动：新号仍按平台基线自动钉。
- 手选 macos Claude：新号钉对应 id，OS 为 macos，软件为当前 Claude bundle。
- 再选 linux 另一套：device id 变了，TLS/OS/软件整套换成新的。
- 官方流量学习只升软件，pin 与 device id 不变。
- 不完整行不能被选中或保存。
