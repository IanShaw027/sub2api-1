# Device TLS pin-once（与学习同一套）

日期：2026-08-17  
分支：personal-main

机制、版本升级、指纹维护、采集和验收见活文档：[`docs/device-tls-pin-and-learn.md`](../../device-tls-pin-and-learn.md)。本文只保留当初锁定的规则。

开学习后自选完整目录行（换选整套重铸）是后续规格：[`2026-08-17-device-tls-catalog-select-design.md`](2026-08-17-device-tls-catalog-select-design.md)。

## 目标

每个账号一份 `account_device_profiles`：预先采集的 ClientHello 钉在 `tls_profile_id`，后期学习只升级同一行的软件包。出站两边都读这一行，不再按请求路由指纹。

## 规则

1. `client_family` + `os_family` + `transport` **只在选模板时用一次**。目录名：`pin:{family}:{os}:{transport}`。
2. 选中后写入 `tls_profile_id`。已有档案若该字段为 nil，第一次 GetOrCreate 补钉，之后不再改。
3. 学习（`LearnIfOfficial`）只改 UA/版本/Stainless，**不改** `tls_profile_id` / device_id。
4. 出站：档案已钉则只用该 id。有档案但未钉时不用 router/bindings/入站 UA。无设备服务时才走旧 extra 路径（单测）。
5. 采集器是样本源。personal-main 只收 ClientHello 瘦字段；不接 `import-captures`。

## 非目标

不从网关入站学握手；不把 HTTP/2 指纹/UA 写进 `tls_fingerprint_profiles`；不默认打开全库 `device_learning_enabled`。
