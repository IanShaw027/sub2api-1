# OpenAI OAuth 分组容量规划设计

## 目标与口径

容量看板以账号分组为一级维度，分别展示 OAuth 账号总数、可调度数、错误数、套餐构成、限流剩余时长分布，以及 5h/7d 窗口的美元消耗、推算总额度、可用额度和预计消耗。全局视图对账号去重；分组消耗严格使用请求落库时的 `usage_logs.group_id`，不会按账号当前绑定分组改写历史。

账号可能绑定多个分组。账号的物理额度仍是共享池，分组视图按本周期实际消耗占比分摊容量；无消耗时才按绑定分组数等分。界面必须标出该口径，不能把同一账号的全部额度重复算给每个分组。

美元消耗使用现有账号成本口径：

```text
SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1))
```

这与账号用量单元格和后台聚合的 `account cost` 一致，不使用受用户分组倍率影响的 `actual_cost`。

## 配额推算

上游 OAuth 遥测只提供已用百分比、窗口长度和重置时间，不提供美元上限。单账号单窗口的观测容量为：

```text
observed_capacity_usd = local_window_spend_usd / (upstream_used_percent / 100)
available_usd = max(observed_capacity_usd - local_window_spend_usd, 0)
```

百分比低于 5%、快照已过期或本地没有消耗时，不直接外推，改用同套餐同窗口已完成周期的稳健中位数；历史不足时再使用当前健康账号样本的中位数。Plus、Team、K12、Pro 分开学习，不硬编码套餐额度。看板同时返回样本数和 `high/medium/low` 置信度。

## 预测

预测保留两个可解释基线：最近 3 小时美元速率，以及前一日同一时段 3 小时美元速率。两者都有数据时采用 `65% recent + 35% previous-day`；只有一侧有数据时直接回退到该侧。每个账号按距自身重置时间的剩余时长计算预计新增消耗，然后聚合到分组，避免把不同滚动窗口强行对齐成一个固定周期。

周期归档保留实际值，后续可扩展为按分组保存预测值和误差，以回测并动态调整权重。补号建议使用预计容量缺口除以各套餐历史中位额度，分别给出 K12/Plus/Team/Pro 所需账号数，不假设不同套餐同质。

## 持久化

- `openai_oauth_quota_snapshots`: 每个账号最多每 15 分钟一条 5h/7d 遥测快照，保留百分比、窗口和重置时间；原始快照保留 90 天。
- `openai_oauth_quota_periods`: 检测到重置时间跨周期后，归档上一周期美元消耗、最终百分比和推算额度，长期作为套餐基线。
- `usage_logs`: 继续作为分组实际消耗的唯一事实表，不复制明细。

## 限流与告警

限流剩余时间使用互斥桶：`<=10m`、`10-30m`、`30m-1h`、`1-3h`、`3-5h`、`5h-1d`、`1-3d`、`>3d`，每个分组独立统计，全局按账号去重。错误数只统计账号 `status=error`，不会与限流数混为一谈。

建议告警同时满足短窗压力和长窗容量风险，例如最近 30 分钟预测缺口为正且 5h/7d 任一可用容量低于 15%，以降低瞬时尖峰误报；补号提前量另加采购/授权准备时间缓冲。

## 调研依据

- [OpenAI Rate limits](https://developers.openai.com/api/docs/guides/rate-limits): 限额响应应保留 limit、remaining、reset 等元数据，并对不同限制维度分别处理。
- [AWS predictive scaling](https://docs.aws.amazon.com/autoscaling/ec2/userguide/predictive-scaling-policy-overview.html): 至少 24 小时数据后预测，先以 forecast-only 评估准确性；日/周周期负载适合预测扩容，预测与动态扩容应组合使用。
- [Google SRE Alerting on SLOs](https://sre.google/workbook/alerting-on-slos/): 单一短窗口精度低，推荐长短窗口组合，兼顾检测速度、准确率与恢复时间。
- [FinOps Planning & Estimating](https://www.finops.org/framework/capabilities/planning-estimating/): 按业务维度分配成本，持续比较 forecast、actual 与 variance，并保留单位成本指标。

## 时序趋势视图（hourly chart）

管理端容量对话框以工具条 + 健康条 + KPI + 每小时折线为主阅读路径。

### API

- 快照（保留）：`GET /api/v1/admin/accounts/openai-oauth-capacity`
- 时序：`GET /api/v1/admin/accounts/openai-oauth-capacity/timeseries`
  - `range`: `12h` | `24h` | `48h` | `7d` | `custom`（custom 需 `from`/`to` RFC3339）
  - `window`: `5h` | `7d` | `both`
  - `group_id`（可选）
  - `force=true` 绕过 1 分钟进程内缓存

默认 `24h` 表示 **近 12 个整点历史 + 当前小时 + 未来 12 小时**；`12h` 为 past 6 + future 6。`custom` 跨度上限与 `openAIOAuthTimeseriesMaxPoints`（192 小时）对齐。

### 每小时点语义

| segment | 消耗线 | 可用额度 |
|---------|--------|----------|
| `past` | 实线，`spent_usd` = 该小时实际 account-cost | 用滚动窗口内历史 spend 回放，定格 |
| `current` | 合成：`spent partial + forecast remainder` | 以 now 为截止推算 |
| `future` | 虚线，仅 `forecast_usd` | 历史 spend + 预测 spend 推演 |

前端将 past/current/future 画成**同一条消耗系列**：future 段 `borderDash`，current 点高亮；可用额度为第二条趋势线；可选双窗口时叠加 7d 可用额度。

### 预测

```text
r_hour = normalize(
  0.50 * recent_3h_rate +
  0.30 * previous_day_same_3h_rate +
  0.20 * last_15m_rate_as_hourly
)
forecast(H) = r_hour * seasonal_shape(H) * hour_fraction
```

`seasonal_shape` 取前一日同时钟小时的 spend，相对未来窗口均值归一，并限制在 `[0.25, 3]`，避免水平直线；缺形状时退化为 1。

### 查询与缓存

- **历史小时必须持久化**：表 `openai_oauth_capacity_hourly`（migration `215`）。
  - 已结束小时 `sealed=true` 后 spend 只读该表，不再扫 `usage_logs`。
  - 缺 sealed 行时，按底层 `usage_logs` 回算 spend 并 UPSERT；没有聚合结果的空小时不封存，允许迟到日志后续补齐。
  - 历史 capacity/available 没有对应时点的持久化遥测时保持 NULL，禁止用查看当下的账号池容量倒灌历史。
  - 当前未结束小时写 `sealed=false` 的 live 快照，可被覆盖；**从不定格**。
  - 持久化键使用 `group_id=-1` 表示全局去重汇总；`group_id=0` 仅表示请求时未分组。
  - `available_*` / `capacity_*` 推算不出则为 NULL；timeseries summary 用指针，前端显示 `—`，禁止 `$0.00` 冒充。
- 号池健康 / 限流桶 / 补号建议复用 snapshot overview 聚合结果。
- timeseries 与 overview 各自 1 分钟 singleflight 缓存；`force` 不重写已 sealed 的 spent。
- 未知 `group_id` 返回 400；`plan_type` 查询参数暂未实现端到端过滤，服务端忽略。
- overview / timeseries 的 capacity、available、used_percent、shortfall 在不可测时均为 **null**（不再用 0 冒充）。
- 小时事实 UPSERT 使用 **multi-row 批量写入**（每批最多 100 行）。
- `custom` 落在同一 UTC 小时内的区间会扩展为至少 1 个小时桶。

## 发布边界

迁移、后端 API 和前端资产必须通过仓库根目录 `./deploy.sh` 一次性发布。源码完成或前端单独构建都不代表生产生效；部署和服务重启必须取得当前任务的明确授权。
