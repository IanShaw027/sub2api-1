# Other Action Equivalence Audit

基线：`f1c8ab7da6284179461613cec8a30a8f87f2c545`。候选源：`actions-candidates.json`。本表覆盖 **31 个旧文件、123 条 action 候选**，每条候选单独列出 ID。当前证据为本次共享工作树，不把 HEAD 当成修复后的源码快照。

## 范围与结论

- 本表123条 + `action-equivalence-accounts.md` 的121条 + `action-equivalence-lists.md` 的65条 + `action-equivalence-settings.md` 的47条，共356条。
- `source-equivalent` 表示已人工追踪模板事件、组件emit/model、父级处理器及相关可见条件，并确认可达替代；**不是浏览器全交互验收通过**。
- `fixed-tested` 表示发现真实回退后已修改生产代码，并有针对性行为测试通过。
- `fixed-awaiting-tests` 表示生产修复已落盘，但测试尚未完成，不能记为验证通过。
- 所有当前证据路径相对于 `frontend/src/`。每个分组标题为旧文件；旧行号来自基线快照，当前行号仅为定位提示，函数/事件名为稳定追踪依据。
- 本表仅核查 action；不重复 API/store 调用、链接集合与31条锚点的独立审计。

本轮发现并修复的非等价变化：

1. `action-098`：有已选记录时，按全部筛选结果批量编辑的独立按钮消失。现恢复直接按钮，不要求先清空选择。
2. `action-100/101/103`：监控行 run/duplicate/delete被收进More。现恢复直接入口，保留旧禁用和确认语义。
3. `action-148`：Passkey删除从form改ConfirmDialog后，Enter确认和空密码按钮禁用消失。现恢复Enter、可选confirmDisabled、busy与重复提交保护。
4. `action-159`：账号容量预测从直接按钮变为More子项。现恢复直接按钮。
5. `action-350`：工单直接关闭按钮变为More子项。现恢复直接入口，同时保留已有More扩展。
6. `action-351`：独立工单创建页取消/返回导致原列表筛选和页码丢失。现通过路由query保存和恢复，浏览器Back也保留上下文。

## 已有验证证据

- Passkey/ConfirmDialog/Tickets直接操作：3文件6项测试通过；对应ESLint与diff-check通过。
- AccountBulkActionsBar/AccountsView.bulkEdit/AccountsHeaderMenus：3文件12项测试通过，执行代理shared_truth。
- MonitorActionsCell：5项回归测试与定向ESLint通过，执行代理theme_primitives。
- 工单取消上下文：`TicketsView.actions.spec.ts` 与 `TicketCreateView.return.spec.ts` 2文件3项测试通过；定向ESLint通过。覆盖原筛选/页码请求恢复、进入创建前先更新历史列表query、创建页取消与返回链接携带同一query。
- typecheck、完整前端测试和更新后视觉矩阵由主代理统一验收，本表不重复声明。

## 逐项映射

### frontend/src/components/account/BulkEditAccountModal.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-001 | `input:concurrency = Math.max(1, concurrency \|\| 1)` (L730) | `components/account/bulk/ConcurrencyPrioritySection.vue:145; components/account/BulkEditAccountModal.vue:337`：updateConcurrency 将原生值规范化为至少1并写回 concurrency model；父级双向绑定保留。 | source-equivalent |
| action-002 | `input:loadFactor = (loadFactor && loadFactor >= 1) ? loadFactor : null` (L759) | `components/account/bulk/ConcurrencyPrioritySection.vue:152; components/account/BulkEditAccountModal.vue:339`：updateLoadFactor 将小于1/空值规范化为null，再写回 loadFactor model。 | source-equivalent |
| action-003 | `click:rpmLimitEnabled = !rpmLimitEnabled` (L1306) | `components/account/bulk/RpmLimitSection.vue:28; components/account/BulkEditAccountModal.vue:435`：直接 ToggleSwitch v-model rpmLimitEnabled，未增加隐藏层级。 | source-equivalent |

### frontend/src/components/account/QuotaLimitCard.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-095 | `click:localEnabled = !localEnabled` (L150) | `components/account/QuotaLimitCard.vue:149`：原按钮反转 localEnabled 改为 ToggleSwitch v-model，同一 computed setter。 | source-equivalent |

### frontend/src/components/account/QuotaNotifyToggle.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-096 | `click:emit('update:enabled', !enabled)` (L21) | `components/account/QuotaNotifyToggle.vue:23`：ToggleSwitch update:model-value 直接转发 update:enabled；仍改变同一 enabled。 | source-equivalent |

### frontend/src/components/account/SyncFromCrsModal.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-097 | `change:toggleSelect(acc.crs_account_id)` (L129) | `components/account/SyncFromCrsModal.vue:120`：Checkbox update:model-value 仍调用 toggleSelect(acc.crs_account_id)，保留选择集。 | source-equivalent |

### frontend/src/components/admin/account/AccountBulkActionsBar.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-098 | `click:$emit('edit-filtered')` (L55) | `components/admin/account/AccountBulkActionsBar.vue:39; views/admin/accounts/AccountsFilterBar.vue:30`：恢复始终显示的 edit-filtered 独立按钮；选中记录时仍能按全部筛选结果批量编辑，不再必须先清空选择。12项账号入口回归测试通过。 | fixed-tested |

### frontend/src/components/admin/account/OpenAIOAuthCapacityDialog.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-099 | `click:selectedRange = option` (L28) | `components/admin/account/PlatformCapacityDialog.vue:66; :388; :865`：旧 OpenAIOAuthCapacityDialog 被 PlatformCapacityDialog 承接；12h/24h/48h/7d均保留，selectedRange写入与watch刷新保留。 | source-equivalent |

### frontend/src/components/admin/monitor/MonitorActionsCell.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-100 | `click:$emit('run', row)` (L4) | `components/admin/monitor/MonitorActionsCell.vue:14`：恢复直接 run 按钮；running禁用和旋转指示保留。监控5项回归测试通过。 | fixed-tested |
| action-101 | `click:$emit('duplicate', row)` (L15) | `components/admin/monitor/MonitorActionsCell.vue:26`：恢复直接 duplicate 按钮、monitor-duplicate锚点和 duplicating/解密失败禁用。监控5项回归测试通过。 | fixed-tested |
| action-102 | `click:$emit('edit', row)` (L24) | `components/admin/monitor/MonitorActionsCell.vue:35`：edit仍直接可见，现与其余旧操作按基线顺序显示。监控5项回归测试通过。 | fixed-tested |
| action-103 | `click:$emit('delete', row)` (L31) | `components/admin/monitor/MonitorActionsCell.vue:44`：恢复直接 delete 按钮，仍仅emit并进入父级确认链；新增detail留在More。监控5项回归测试通过。 | fixed-tested |

### frontend/src/components/auth/LoginAgreementPrompt.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-114 | `change:handleCheckboxChange` (L12) | `components/auth/LoginAgreementPrompt.vue:6; :127`：原 input change/Event.target.checked 改为 Checkbox 的boolean update:model-value；accept/reject分支相同。 | source-equivalent |

### frontend/src/components/layout/AppSidebar.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-123 | `click:handleGroupClick(item)` (L49) | `components/layout/sidebar/SidebarItem.vue:17; components/layout/AppSidebar.vue:44`：SidebarItem onGroupClick -> group-click -> handleGroupClick，分组点击行为移到子组件。 | source-equivalent |
| action-124 | `click:handleMenuItemClick(child.path)` (L76) | `components/layout/sidebar/SidebarItem.vue:67; :89; components/layout/AppSidebar.vue:43`：子项导航经 navigate(child.path)传回 handleMenuItemClick；展开子项与折叠flyout均有入口。 | source-equivalent |
| action-125 | `click:handleMenuItemClick(item.path)` (L103) | `components/layout/sidebar/SidebarItem.vue:115; components/layout/AppSidebar.vue:43`：原不同区域item链接统一由 SidebarItem navigate(item.path)转回同一handleMenuItemClick；导航路径另见nav审计。 | source-equivalent |
| action-126 | `click:handleMenuItemClick(item.path)` (L134) | `components/layout/sidebar/SidebarItem.vue:115; components/layout/AppSidebar.vue:43`：原不同区域item链接统一由 SidebarItem navigate(item.path)转回同一handleMenuItemClick；导航路径另见nav审计。 | source-equivalent |
| action-127 | `click:handleMenuItemClick(item.path)` (L160) | `components/layout/sidebar/SidebarItem.vue:115; components/layout/AppSidebar.vue:43`：原不同区域item链接统一由 SidebarItem navigate(item.path)转回同一handleMenuItemClick；导航路径另见nav审计。 | source-equivalent |
| action-128 | `click:toggleSidebar` (L194) | `components/layout/sidebar/SidebarFooter.vue:49`：桌面折叠按钮仍直接调用 appStore.toggleSidebar，位置迁至footer。 | source-equivalent |
| action-129 | `click:closeMobile` (L211) | `components/layout/MobileDrawer.vue:7; :45`：旧移动遮罩closeMobile由MobileDrawer遮罩/关闭按钮close承接；焦点、Escape另见interaction-verification。 | source-equivalent |

### frontend/src/components/tickets/forms/TicketFormConcurrency.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-130 | `input:updateField('target_concurrency', ($event.target as HTMLInputElement).value)` (L10) | `components/tickets/forms/TicketFormConcurrency.vue:13`：target_concurrency 改由TextInput update:model-value -> String -> 同一updateField，readonly保留。 | source-equivalent |
| action-131 | `input:updateField('usage_scenario', ($event.target as HTMLTextAreaElement).value)` (L15) | `components/tickets/forms/TicketFormConcurrency.vue:21`：usage_scenario改由TextArea update:model-value -> updateField，字符串payload不变。 | source-equivalent |
| action-132 | `input:updateField('peak_window', ($event.target as HTMLInputElement).value)` (L19) | `components/tickets/forms/TicketFormConcurrency.vue:27`：peak_window改由TextInput update:model-value -> String -> updateField。 | source-equivalent |

### frontend/src/components/tickets/forms/TicketFormConsult.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-133 | `input:updateField('question', ($event.target as HTMLTextAreaElement).value)` (L9) | `components/tickets/forms/TicketFormConsult.vue:8`：question改由TextArea update:model-value -> updateField。 | source-equivalent |

### frontend/src/components/tickets/forms/TicketFormOther.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-134 | `input:updateField('details', ($event.target as HTMLTextAreaElement).value)` (L9) | `components/tickets/forms/TicketFormOther.vue:8`：details改由TextArea update:model-value -> updateField。 | source-equivalent |

### frontend/src/components/tickets/forms/TicketFormRate.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-135 | `input:updateField('target_rate', ($event.target as HTMLInputElement).value)` (L52) | `components/tickets/forms/TicketFormRate.vue:54`：target_rate改由TextInput update:model-value -> String -> updateField；分组选择和readonly未移除。 | source-equivalent |
| action-136 | `input:updateField('usage_scenario', ($event.target as HTMLTextAreaElement).value)` (L57) | `components/tickets/forms/TicketFormRate.vue:62`：usage_scenario改由TextArea update:model-value -> updateField。 | source-equivalent |

### frontend/src/components/tickets/forms/TicketFormRefund.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-137 | `input:updateField('order_no', ($event.target as HTMLInputElement).value)` (L6) | `components/tickets/forms/TicketFormRefund.vue:8`：order_no改由TextInput update:model-value -> String -> updateField。 | source-equivalent |
| action-138 | `input:updateField('refund_amount', ($event.target as HTMLInputElement).value)` (L10) | `components/tickets/forms/TicketFormRefund.vue:14`：refund_amount改由TextInput update:model-value -> String -> updateField；expected_amount兼容读取保留。 | source-equivalent |
| action-139 | `input:updateField('reason', ($event.target as HTMLTextAreaElement).value)` (L15) | `components/tickets/forms/TicketFormRefund.vue:22`：reason改由TextArea update:model-value -> updateField。 | source-equivalent |
| action-140 | `input:updateField('evidence', ($event.target as HTMLTextAreaElement).value)` (L19) | `components/tickets/forms/TicketFormRefund.vue:29`：evidence改由TextArea update:model-value -> updateField。 | source-equivalent |

### frontend/src/components/user/dashboard/UserDashboardCharts.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-141 | `update:startDate:$emit('update:startDate', $event)` (L8) | `views/user/DashboardView.vue:44`：日期选择器移至父级顶部，update:startDate直接赋值原startDate。 | source-equivalent |
| action-142 | `update:endDate:$emit('update:endDate', $event)` (L8) | `views/user/DashboardView.vue:45`：日期选择器移至父级顶部，update:endDate直接赋值原endDate。 | source-equivalent |
| action-143 | `change:$emit('dateRangeChange', $event)` (L8) | `views/user/DashboardView.vue:46; :263`：日期change由父级onDateRangeChange接收，仍loadCharts并新增loadRecent。 | source-equivalent |
| action-144 | `update:model-value:$emit('update:granularity', $event)` (L16) | `components/user/dashboard/UserDashboardCharts.vue:15; :77`：Select改SegmentedControl，onGranularityChange先emit update:granularity。 | source-equivalent |
| action-145 | `change:$emit('granularityChange')` (L16) | `components/user/dashboard/UserDashboardCharts.vue:79; views/user/DashboardView.vue:104`：onGranularityChange随后emit granularityChange，父级loadCharts仍连接。 | source-equivalent |

### frontend/src/components/user/dashboard/UserDashboardStats.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-146 | `click:emit('balance-history')` (L5) | `views/user/DashboardView.vue:61; :114`：余额卡移至dashboard顶部mini card，标准模式直接打开同一BalanceHistoryModal；simple模式沿用隐藏余额业务语义。 | source-equivalent |

### frontend/src/components/user/profile/ProfilePasskeyCard.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-147 | `click:closeDeleteDialog` (L135) | `components/user/profile/ProfilePasskeyCard.vue:153; components/common/ConfirmDialog.vue:2; :100`：旧遮罩/取消按钮由ConfirmDialog->BaseDialog close/handleCancel->closeDeleteDialog承接。 | source-equivalent |
| action-148 | `submit:confirmDelete` (L145) | `components/user/profile/ProfilePasskeyCard.vue:150; :168; :274; components/common/ConfirmDialog.vue:32`：修复原form Enter丢失：密码输入keydown.enter.prevent调用confirmDelete，空密码confirmDisabled、busy禁用与重复提交guard恢复。3文件6项定向测试通过。 | fixed-tested |
| action-149 | `click:closeDeleteDialog` (L161) | `components/user/profile/ProfilePasskeyCard.vue:153; components/common/ConfirmDialog.vue:2; :100`：旧遮罩/取消按钮由ConfirmDialog->BaseDialog close/handleCancel->closeDeleteDialog承接。 | source-equivalent |

### frontend/src/components/user/profile/ProfileTotpCard.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-150 | `click:showDisableDialog = true` (L54) | `components/user/profile/ProfileTotpCard.vue:49; :95`：ToggleSwitch切到false时handleToggle打开原TotpDisableDialog，不直接变更服务器状态。 | source-equivalent |
| action-151 | `click:showSetupModal = true` (L80) | `components/user/profile/ProfileTotpCard.vue:49; :95`：ToggleSwitch切到true时handleToggle打开原TotpSetupModal，原成功/关闭回调保留。 | source-equivalent |

### frontend/src/features/channel-monitor-v2/MonitorSettingsPanel.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-152 | `click:draft.refresh_interval_seconds = 60` (L70) | `features/channel-monitor-v2/MonitorSettingsPanel.vue:62; :333`：两个刷新间隔按钮合为始终可见SegmentedControl；computed setter仅映射60/300，值集合未减少。 | source-equivalent |
| action-153 | `click:draft.refresh_interval_seconds = 300` (L78) | `features/channel-monitor-v2/MonitorSettingsPanel.vue:62; :333`：两个刷新间隔按钮合为始终可见SegmentedControl；computed setter仅映射60/300，值集合未减少。 | source-equivalent |

### frontend/src/features/prompt-audit/components/EventWorkspace.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-154 | `change:toggleOne(event.id)` (L77) | `features/prompt-audit/components/EventWorkspace.vue:62`：DataTable scoped row替代旧event局部变量，checkbox仍toggleOne(row.id)。 | source-equivalent |
| action-155 | `click:$emit('view', event.id)` (L95) | `features/prompt-audit/components/EventWorkspace.vue:92`：直接view按钮仍emit(view,row.id)，未移入菜单。 | source-equivalent |
| action-156 | `click:$emit('delete', event.id)` (L96) | `features/prompt-audit/components/EventWorkspace.vue:93`：直接delete按钮仍emit(delete,row.id)，未移入菜单。 | source-equivalent |

### frontend/src/views/KeyUsageView.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-157 | `click:setDailyUsageDays(option.value)` (L303) | `views/KeyUsageView.vue:172`：日用量范围由按钮列表改SegmentedControl，update:model-value仍setDailyUsageDays，options沿用dailyUsageOptions。 | source-equivalent |

### frontend/src/views/admin/AccountsView.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-158 | `update:filters:(newFilters) => Object.assign(params, newFilters)` (L10) | `views/admin/accounts/AccountsFilterBar.vue:43`：AccountTableFilters迁到子组件；update:filters仍Object.assign(params,newFilters)，不替换筛选对象。 | source-equivalent |
| action-159 | `click:showCapacityForecast = true` (L20) | `views/admin/accounts/AccountsHeaderMenus.vue:5; views/admin/AccountsView.vue:19`：恢复独立容量预测按钮 -> open-capacity-forecast -> showCapacityForecast；不再只能经More。12项账号入口回归测试通过。 | fixed-tested |
| action-160 | `click: showAutoRefreshDropdown = !showAutoRefreshDropdown; showAccountToolsDropdown = false` (L29) | `views/admin/accounts/AccountsFilterBar.vue:56; views/admin/accounts/useAccountToolbarMenus.ts:72`：自动刷新触发器迁到筛选条，toggleAutoRefreshDropdown仍反转自身并关闭其他下拉。 | source-equivalent |
| action-161 | `click:openSyncFromCrs` (L97) | `views/admin/accounts/AccountsHeaderMenus.vue:18; :123; views/admin/AccountsView.vue:16`：CRS同步从原More数据组迁至Import/Export菜单；仍关闭菜单后emit open-sync -> 原openSyncFromCrs。基线本来在菜单内。 | source-equivalent |
| action-162 | `click:openImportData` (L103) | `views/admin/accounts/AccountsHeaderMenus.vue:22; :128; views/admin/AccountsView.vue:17`：导入仍在数据菜单，wrapper emit open-import -> 原openImportData；无新增层级。 | source-equivalent |
| action-163 | `click:openExportDataDialogFromMenu` (L109) | `views/admin/accounts/AccountsHeaderMenus.vue:26; :133; views/admin/AccountsView.vue:18`：导出仍在数据菜单，wrapper emit open-export -> 原导出确认；selected IDs保留。 | source-equivalent |
| action-164 | `click:handleToggleSchedulable(row)` (L330) | `views/admin/AccountsView.vue:147`：调度按钮改直接ToggleSwitch update:model-value -> handleToggleSchedulable(row)，仍按当前账户状态执行。 | source-equivalent |

### frontend/src/views/admin/BackupView.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-165 | `click:downloadBackup(record.id)` (L261) | `views/admin/BackupView.vue:205`：DataTable row替代旧record变量，直接downloadBackup(row.id)保留。 | source-equivalent |
| action-166 | `click:restoreBackup(record.id)` (L270) | `views/admin/BackupView.vue:214; :367; :862`：直接恢复入口改promptRestoreBackup，再经ConfirmDialog确认/Enter -> confirmRestoreBackup；密码/step-up链仍在。 | source-equivalent |
| action-167 | `click:removeBackup(record.id)` (L278) | `views/admin/BackupView.vue:222; :388; :897`：直接删除入口改promptRemoveBackup，再经确认->confirmRemoveBackup，同一备份id。 | source-equivalent |

### frontend/src/views/admin/DashboardView.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-168 | `ranking-click:goToUserUsage` (L360) | `components/admin/dashboard/DashTrendDistRow.vue:73; views/admin/DashboardView.vue:61; :590`：用户排名点击转为userRowClick(userId)，父级goToUserUsageById携带user_id/start_date/end_date跳到同一usage页面。 | source-equivalent |

### frontend/src/views/admin/RedeemView.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-214 | `click:showGenerateDialog = false` (L278) | `components/admin/redeem/RedeemGenerateModal.vue:7; :91; views/admin/RedeemView.vue:255`：生成弹窗移子组件，UiModal close与取消按钮仍emit close令showGenerateDialog=false。 | source-equivalent |
| action-215 | `click:generateForm.expiry_option = option.value` (L366) | `components/admin/redeem/RedeemGenerateModal.vue:71`：过期选项按钮改SegmentedControl v-model generateForm.expiry_option，custom分支和过期计算保留。 | source-equivalent |
| action-216 | `click:showGenerateDialog = false` (L400) | `components/admin/redeem/RedeemGenerateModal.vue:7; :91; views/admin/RedeemView.vue:255`：生成弹窗移子组件，UiModal close与取消按钮仍emit close令showGenerateDialog=false。 | source-equivalent |
| action-217 | `click:closeBatchUpdateDialog` (L418) | `components/admin/redeem/RedeemBatchUpdateModal.vue:7; :106; views/admin/RedeemView.vue:264`：批量更新弹窗的遮罩/取消通过UiModal close与子组件close关闭同一showBatchUpdateDialog。 | source-equivalent |
| action-218 | `click:closeBatchUpdateDialog` (L509) | `components/admin/redeem/RedeemBatchUpdateModal.vue:7; :106; views/admin/RedeemView.vue:264`：批量更新弹窗的遮罩/取消通过UiModal close与子组件close关闭同一showBatchUpdateDialog。 | source-equivalent |
| action-219 | `click:closeResultDialog` (L529) | `components/admin/redeem/RedeemResultModal.vue:7; views/admin/RedeemView.vue:272`：结果弹窗关闭迁为UiModal close->handleClose->父级closeResultDialog；共享右上关闭入口代替手绘按钮。 | source-equivalent |
| action-220 | `click:closeResultDialog` (L563) | `components/admin/redeem/RedeemResultModal.vue:7; views/admin/RedeemView.vue:272`：结果弹窗关闭迁为UiModal close->handleClose->父级closeResultDialog；共享右上关闭入口代替手绘按钮。 | source-equivalent |

### frontend/src/views/user/AffiliateView.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-267 | `click:copyCode` (L56) | `components/ui/EndpointCard.vue:13; views/user/AffiliateView.vue:34`：邀请码复制按钮由EndpointCard直接发copy -> 原copyCode；URL参数附加不改变忽略参数的handler。 | source-equivalent |
| action-268 | `click:copyInviteLink` (L67) | `components/ui/EndpointCard.vue:13; views/user/AffiliateView.vue:40`：邀请链接复制按钮由EndpointCard直接发copy -> 原copyInviteLink。 | source-equivalent |

### frontend/src/views/user/BatchImageGuideView.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-269 | `click:refreshPage` (L24) | `components/user/batch-image/BatchImageFiltersBar.vue:21; views/user/BatchImageGuideView.vue:19`：刷新按钮emit refresh -> 原refreshPage，loadingKeys/loadingJobs禁用保留。 | source-equivalent |
| action-270 | `click:downloadSelectedJobs` (L58) | `components/user/batch-image/BatchImageFiltersBar.vue:55; views/user/BatchImageGuideView.vue:22`：选中时显示批量下载 -> download-selected -> downloadSelectedJobs，下载资格禁用条件保留。 | source-equivalent |
| action-271 | `click:deleteSelectedJobs` (L67) | `components/user/batch-image/BatchImageFiltersBar.vue:64; views/user/BatchImageGuideView.vue:23`：选中时显示批量删除 -> delete-selected -> deleteSelectedJobs，bulkDeleting禁用保留。 | source-equivalent |
| action-272 | `change:toggleJobSelection(row.id, ($event.target as HTMLInputElement).checked)` (L100) | `components/user/batch-image/BatchImageJobsTable.vue:24; views/user/BatchImageGuideView.vue:49`：行checkbox转发row.id与checked，父级仍toggleJobSelection。 | source-equivalent |
| action-273 | `click:toggleChildRows(row.id)` (L112) | `components/user/batch-image/BatchImageJobsTable.vue:36; views/user/BatchImageGuideView.vue:50`：父任务展开按钮转发toggle-child(row.id)，原子任务条件保留。 | source-equivalent |
| action-274 | `click:selectJob(row.id)` (L117) | `components/user/batch-image/BatchImageJobsTable.vue:41; views/user/BatchImageGuideView.vue:51`：任务名直接点击select-job(row.id) -> selectJob。 | source-equivalent |
| action-275 | `click:selectJob(row.id)` (L184) | `components/user/batch-image/BatchImageJobsTable.vue:109; views/user/BatchImageGuideView.vue:51`：操作列查看按钮直接select-job(row.id) -> selectJob，双入口均保留。 | source-equivalent |
| action-276 | `click:downloadJob(row)` (L195) | `components/user/batch-image/BatchImageJobsTable.vue:119; views/user/BatchImageGuideView.vue:52`：直接行下载emit download-job(row) -> downloadJob，canDownload/downloading禁用保留。 | source-equivalent |
| action-277 | `click:toggleMoreMenu(row, $event)` (L210) | `components/user/batch-image/BatchImageJobsTable.vue:134; views/user/BatchImageGuideView.vue:53`：原More按钮转发row与原MouseEvent，定位菜单所需事件对象保留。 | source-equivalent |
| action-278 | `change:handlePageSizeChange` (L253) | `components/user/batch-image/BatchImagePaginationBar.vue:23; views/user/BatchImageGuideView.vue:66`：页大小change原值经emit change-page-size -> handlePageSizeChange。 | source-equivalent |
| action-279 | `click:handlePageChange(pagination.page - 1)` (L262) | `components/user/batch-image/BatchImagePaginationBar.vue:32; views/user/BatchImageGuideView.vue:65`：上一页按钮emit change-page(page-1) -> handlePageChange；页边界/loading禁用保留。 | source-equivalent |
| action-280 | `click:handlePageChange(pagination.page + 1)` (L271) | `components/user/batch-image/BatchImagePaginationBar.vue:41; views/user/BatchImageGuideView.vue:65`：下一页按钮emit change-page(page+1) -> handlePageChange；hasMore/loading禁用保留。 | source-equivalent |
| action-281 | `click:retryFailedJob(job)` (L295) | `components/user/batch-image/BatchImageMoreMenu.vue:16; views/user/BatchImageGuideView.vue:79`：原菜单重试emit retry(job) -> retryFailedJob，canRetry与retrying禁用保留。 | source-equivalent |
| action-282 | `click:deleteJob(job)` (L305) | `components/user/batch-image/BatchImageMoreMenu.vue:26; views/user/BatchImageGuideView.vue:80`：原菜单删除emit delete(job) -> deleteJob，canDeleteRecord与deleting禁用保留。 | source-equivalent |
| action-283 | `mouseenter:cancelPromptPopoverClose` (L320) | `components/user/batch-image/BatchImagePromptPopover.vue:7; views/user/BatchImageGuideView.vue:87`：mouseenter -> cancel-close -> cancelPromptPopoverClose，维持跨触发器/浮层停留。 | source-equivalent |
| action-284 | `mouseleave:schedulePromptPopoverClose` (L321) | `components/user/batch-image/BatchImagePromptPopover.vue:8; views/user/BatchImageGuideView.vue:88`：mouseleave -> schedule-close -> schedulePromptPopoverClose。 | source-equivalent |
| action-285 | `click:copyPromptPopover` (L328) | `components/user/batch-image/BatchImagePromptPopover.vue:15; views/user/BatchImageGuideView.vue:89`：浮层copy -> copyPromptPopover。 | source-equivalent |
| action-286 | `click:refreshDetail` (L374) | `components/user/batch-image/BatchImageDetailModal.vue:37; views/user/BatchImageGuideView.vue:122`：详情refresh -> refreshDetail，refreshing/loadingItems禁用保留。 | source-equivalent |
| action-287 | `pointerenter:schedulePromptPopoverOpen($event, item.prompt_preview \|\| '-')` (L418) | `components/user/batch-image/BatchImageDetailModal.vue:81; views/user/BatchImageGuideView.vue:129`：pointerenter -> prompt-hover(event,prompt) -> schedulePromptPopoverOpen。 | source-equivalent |
| action-288 | `pointerleave:schedulePromptPopoverClose` (L419) | `components/user/batch-image/BatchImageDetailModal.vue:82; views/user/BatchImageGuideView.vue:130`：pointerleave -> prompt-leave -> schedulePromptPopoverClose。 | source-equivalent |
| action-289 | `mouseenter:schedulePromptPopoverOpen($event, item.prompt_preview \|\| '-')` (L420) | `components/user/batch-image/BatchImageDetailModal.vue:83; views/user/BatchImageGuideView.vue:129`：mouseenter兼容路径 -> prompt-hover，原事件与文本参数保留。 | source-equivalent |
| action-290 | `mouseleave:schedulePromptPopoverClose` (L421) | `components/user/batch-image/BatchImageDetailModal.vue:84; views/user/BatchImageGuideView.vue:130`：mouseleave兼容路径 -> prompt-leave。 | source-equivalent |
| action-291 | `click:showPromptPopover($event, item.prompt_preview \|\| '-')` (L422) | `components/user/batch-image/BatchImageDetailModal.vue:85; views/user/BatchImageGuideView.vue:131`：点击prompt -> prompt-show(event,prompt) -> showPromptPopover，触屏入口保留。 | source-equivalent |
| action-292 | `focus:showPromptPopover($event, item.prompt_preview \|\| '-')` (L423) | `components/user/batch-image/BatchImageDetailModal.vue:86; views/user/BatchImageGuideView.vue:131`：focus -> prompt-show(event,prompt)，键盘入口保留。 | source-equivalent |
| action-293 | `focusin:showPromptPopover($event, item.prompt_preview \|\| '-')` (L424) | `components/user/batch-image/BatchImageDetailModal.vue:87; views/user/BatchImageGuideView.vue:131`：focusin -> prompt-show(event,prompt)，子元素聚焦兼容保留。 | source-equivalent |
| action-294 | `blur:schedulePromptPopoverClose` (L425) | `components/user/batch-image/BatchImageDetailModal.vue:88; views/user/BatchImageGuideView.vue:130`：blur -> prompt-leave -> schedulePromptPopoverClose。 | source-equivalent |
| action-295 | `click:openImagePreview(item)` (L442) | `components/user/batch-image/BatchImageDetailModal.vue:105; views/user/BatchImageGuideView.vue:126`：已有预览图按钮emit open-preview(item) -> openImagePreview。 | source-equivalent |
| action-296 | `error:handlePreviewError(itemPreviewKey(item))` (L448) | `components/user/batch-image/BatchImageDetailModal.vue:111; views/user/BatchImageGuideView.vue:128`：img error emit preview-error(itemPreviewKey(item)) -> handlePreviewError，仍按key记录失败。 | source-equivalent |
| action-297 | `click:loadItemPreview(item)` (L457) | `components/user/batch-image/BatchImageDetailModal.vue:120; views/user/BatchImageGuideView.vue:127`：缺图/失败时加载按钮emit load-preview(item) -> loadItemPreview，loading禁用保留。 | source-equivalent |
| action-298 | `click:cancelSelected` (L492) | `components/user/batch-image/BatchImageDetailModal.vue:155; views/user/BatchImageGuideView.vue:123`：详情取消任务emit cancel -> cancelSelected，canCancel/cancelling禁用保留。 | source-equivalent |
| action-299 | `click:retrySelected` (L501) | `components/user/batch-image/BatchImageDetailModal.vue:164; views/user/BatchImageGuideView.vue:124`：详情重试emit retry -> retrySelected，canRetry与job匹配禁用保留。 | source-equivalent |
| action-300 | `click:downloadSelected` (L510) | `components/user/batch-image/BatchImageDetailModal.vue:173; views/user/BatchImageGuideView.vue:125`：详情下载emit download -> downloadSelected，canDownload/downloading禁用保留。 | source-equivalent |
| action-301 | `change:handleReferenceImageFiles` (L651) | `components/user/batch-image/BatchImageCreateModal.vue:113; views/user/BatchImageGuideView.vue:167`：文件选择change透传原Event -> handleReferenceImageFiles，文件读取target未丢失。 | source-equivalent |
| action-302 | `click:addPromptRow` (L654) | `components/user/batch-image/BatchImageCreateModal.vue:116; views/user/BatchImageGuideView.vue:164`：添加prompt按钮emit add-prompt -> addPromptRow，空prompt禁用保留。 | source-equivalent |
| action-303 | `click:removeReferenceImageDraft(refIndex)` (L666) | `components/user/batch-image/BatchImageCreateModal.vue:128; views/user/BatchImageGuideView.vue:166`：参考图删除emit remove-reference(refIndex) -> removeReferenceImageDraft。 | source-equivalent |
| action-304 | `click:removePromptRow(index)` (L689) | `components/user/batch-image/BatchImageCreateModal.vue:151; views/user/BatchImageGuideView.vue:165`：prompt行删除emit remove-prompt(index) -> removePromptRow。 | source-equivalent |
| action-305 | `click:submitJob` (L710) | `components/user/batch-image/BatchImageCreateModal.vue:3; :172; views/user/BatchImageGuideView.vue:163`：创建的form submit和直接按钮均emit submit -> submitJob；submitting/model/API key资格禁用保留。 | source-equivalent |
| action-306 | `click:showGuideModal = false` (L743) | `components/user/batch-image/BatchImageGuideModal.vue:2; :27; views/user/BatchImageGuideView.vue:173`：指南共享关闭与页脚关闭均emit close -> showGuideModal=false。 | source-equivalent |
| action-307 | `click:copyInstruction` (L744) | `components/user/batch-image/BatchImageGuideModal.vue:28; views/user/BatchImageGuideView.vue:174`：指南复制emit copy -> copyInstruction。 | source-equivalent |

### frontend/src/views/user/ChannelStatusV2View.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-308 | `click:setRange(option.value)` (L105) | `features/channel-monitor-v2/MonitorToolbar.vue:75; views/user/ChannelStatusV2View.vue:113`：范围按钮改SegmentedControl update:range -> setRange，range值集合保留。 | source-equivalent |
| action-309 | `click:clearDimensions` (L139) | `features/channel-monitor-v2/MonitorToolbar.vue:109; views/user/ChannelStatusV2View.vue:112`：清维度按钮emit clear -> clearDimensions。 | source-equivalent |
| action-310 | `click:trendView = 'pulse'` (L162) | `features/channel-monitor-v2/MonitorToolbar.vue:126; views/user/ChannelStatusV2View.vue:118`：pulse/line按钮合SegmentedControl trendViewOptions -> update:trendView；父级写回trendView。 | source-equivalent |
| action-311 | `click:trendView = 'line'` (L170) | `features/channel-monitor-v2/MonitorToolbar.vue:126; views/user/ChannelStatusV2View.vue:118`：pulse/line按钮合SegmentedControl trendViewOptions -> update:trendView；父级写回trendView。 | source-equivalent |
| action-312 | `click:healthMode = option.value` (L188) | `features/channel-monitor-v2/MonitorToolbar.vue:135; views/user/ChannelStatusV2View.vue:119`：healthMode按钮组改SegmentedControl healthModeOptions -> update:healthMode。 | source-equivalent |
| action-313 | `click:activeTab = item.value` (L283) | `features/channel-monitor-v2/MonitorDataTabs.vue:8; views/user/ChannelStatusV2View.vue:166`：Tab选择移子组件SegmentedControl -> update:activeTab，父级写回activeTab。 | source-equivalent |
| action-314 | `click:drillModel(row)` (L307) | `features/channel-monitor-v2/MonitorDataTabs.vue:29; views/user/ChannelStatusV2View.vue:167`：模型行点击emit drillModel(row) -> drillModel，同一row参数。 | source-equivalent |
| action-315 | `click:toggleError(row.category)` (L346) | `features/channel-monitor-v2/MonitorDataTabs.vue:68; views/user/ChannelStatusV2View.vue:168`：错误行点击emit toggleError(row.category) -> toggleError，同一category。 | source-equivalent |

### frontend/src/views/user/DashboardView.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-316 | `balance-history:showBalanceHistory = true` (L14) | `views/user/DashboardView.vue:61; :114`：余额卡移至dashboard顶部mini card，标准模式直接打开同一BalanceHistoryModal；simple模式沿用隐藏余额业务语义。 | source-equivalent |
| action-317 | `dateRangeChange:loadCharts` (L16) | `views/user/DashboardView.vue:46; :263`：旧charts dateRangeChange移至父级DateRangePicker onDateRangeChange，loadCharts仍执行。 | source-equivalent |
| action-318 | `refresh:refreshAll` (L16) | `views/user/DashboardView.vue:48`：刷新动作移至父级日期工具栏，直接refreshAll，非错误态也有入口。 | source-equivalent |

### frontend/src/views/user/TicketsView.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-350 | `click:closeTicketItem(row.id)` (L93) | `views/user/TicketsView.vue:106; :272`：恢复直接关闭图标按钮，不需先开More；闭合/撤回状态不显示，与旧canCloseTicket保持一致。TicketsView直接操作回归测试通过。 | fixed-tested |
| action-351 | `close:closeCreateDialog` (L129) | `views/user/TicketsView.vue:191; views/user/TicketCreateView.vue:5; :35; :64`：发现并修复独立创建页取消会重置列表上下文：进入创建前先把filters/page写入当前列表query，再携带至/tickets/new；取消/返回和浏览器Back恢复原query，列表初始化还原同一筛选与页码。 | fixed-tested |
| action-352 | `submit:createTicket` (L130) | `views/user/TicketCreateView.vue:36; :95`：创建submit迁至独立页直接按钮 -> submit，仍validateTicketPayload，提交成功转同一详情路由；取消上下文由action-351恢复，业务调用另见独立审计。 | source-equivalent |

### frontend/src/views/user/UsageView.vue

| ID | 基线事件与行号 | 当前等价入口证据 | 结论 |
|---|---|---|---|
| action-353 | `click:activeTab = 'usage'` (L171) | `views/user/UsageView.vue:68; views/user/usage/useUserUsage.ts:138`：SegmentedControl -> onTabChange('usage')直接写回activeTab。 | source-equivalent |
| action-354 | `click:switchToErrors` (L174) | `views/user/UsageView.vue:68; views/user/usage/useUserUsage.ts:138; :655`：SegmentedControl -> onTabChange('errors')调用原switchToErrors，错误视图权限门禁保留。 | source-equivalent |

## 验收边界

- 事件字符串消失不能直接认定为功能删除；本表给出的是每个候选的实际可达替代链，不仅是同名函数仍存在。
- 菜单内基线动作可迁到等价菜单，不能把旧直接动作默认收进More而声称无回退；本轮发现的此类问题均已单独列出。
- 数据表由`record/event`改为slot局部`row`、原生input改为`update:model-value`、按钮组选项改为SegmentedControl，只在取值、参数及父级回调均对齐时记为源码等价。
- 本报告不代替真实网络成功/失败、键盘、移动端和视觉验收；这些结果应由最终验证记录关联。
