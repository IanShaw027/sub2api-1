<template>
    <!-- Edit Group Modal -->
    <BaseDialog
      :show="show"
      :title="t('admin.groups.editGroup')"
      width="wide"
      @close="closeEditModal"
    >
      <form
        v-if="editingGroup"
        id="edit-group-form"
        @submit.prevent="handleUpdateGroup"
        class="space-y-5"
      >
        <div>
          <label class="input-label">{{ t("admin.groups.form.name") }}</label>
          <input
            v-model="editForm.name"
            type="text"
            required
            class="input"
            data-tour="edit-group-form-name"
          />
        </div>
        <div>
          <label class="input-label">{{
            t("admin.groups.form.description")
          }}</label>
          <textarea
            v-model="editForm.description"
            rows="3"
            class="input"
          ></textarea>
        </div>
        <div>
          <label class="input-label">{{
            t("admin.groups.form.platform")
          }}</label>
          <Select
            v-model="editForm.platform"
            :options="platformOptions"
            :disabled="true"
            data-tour="group-form-platform"
          />
          <p class="input-hint">{{ t("admin.groups.platformNotEditable") }}</p>
        </div>
        <!-- 从分组复制账号（编辑时） -->
        <div v-if="copyAccountsGroupOptionsForEdit.length > 0">
          <div class="mb-1.5 flex items-center gap-1">
            <label class="text-sm font-medium text-foreground">
              {{ t("admin.groups.copyAccounts.title") }}
            </label>
            <div class="group relative inline-flex">
              <Icon
                name="questionCircle"
                size="sm"
                :stroke-width="2"
                class="cursor-help text-muted transition-colors hover:text-accent "
              />
              <div
                class="pointer-events-none absolute bottom-full left-0 z-50 mb-2 w-72 opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:opacity-100"
              >
                <div
                  class="rounded-lg bg-[var(--code-bg)] p-3 text-white shadow-[var(--shadow-pop)] "
                >
                  <p class="text-xs leading-relaxed text-muted">
                    {{ t("admin.groups.copyAccounts.tooltipEdit") }}
                  </p>
                  <div
                    class="absolute -bottom-1.5 left-3 h-3 w-3 rotate-45 bg-[var(--code-bg)] "
                  ></div>
                </div>
              </div>
            </div>
          </div>
          <!-- 已选分组标签 -->
          <div
            v-if="editForm.copy_accounts_from_group_ids.length > 0"
            class="flex flex-wrap gap-1.5 mb-2"
          >
            <span
              v-for="groupId in editForm.copy_accounts_from_group_ids"
              :key="groupId"
              class="inline-flex items-center gap-1 rounded-full bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] px-2.5 py-1 text-xs font-medium text-accent  "
            >
              {{
                copyAccountsGroupOptionsForEdit.find((o) => o.value === groupId)
                  ?.label || `#${groupId}`
              }}
              <button
                type="button"
                @click="
                  editForm.copy_accounts_from_group_ids =
                    editForm.copy_accounts_from_group_ids.filter(
                      (id) => id !== groupId,
                    )
                "
                class="ml-0.5 text-accent hover:text-accent "
              >
                <Icon name="x" size="xs" />
              </button>
            </span>
          </div>
          <!-- 分组选择下拉 -->
          <select
            class="input"
            @change="
              (e) => {
                const val = Number((e.target as HTMLSelectElement).value);
                if (
                  val &&
                  !editForm.copy_accounts_from_group_ids.includes(val)
                ) {
                  editForm.copy_accounts_from_group_ids.push(val);
                }
                (e.target as HTMLSelectElement).value = '';
              }
            "
          >
            <option value="">
              {{ t("admin.groups.copyAccounts.selectPlaceholder") }}
            </option>
            <option
              v-for="opt in copyAccountsGroupOptionsForEdit"
              :key="opt.value"
              :value="opt.value"
              :disabled="
                editForm.copy_accounts_from_group_ids.includes(opt.value)
              "
            >
              {{ opt.label }}
            </option>
          </select>
          <p class="input-hint">
            {{ t("admin.groups.copyAccounts.hintEdit") }}
          </p>
        </div>
        <div>
          <label class="input-label">{{
            t("admin.groups.form.rateMultiplier")
          }}</label>
          <input
            v-model.number="editForm.rate_multiplier"
            type="number"
            step="0.001"
            min="0.001"
            required
            class="input"
            data-tour="group-form-multiplier"
          />
        </div>
        <div>
          <label class="input-label">{{ t("admin.groups.form.rpmLimit") }}</label>
          <input
            v-model.number="editForm.rpm_limit"
            type="number"
            min="0"
            step="1"
            class="input"
            :placeholder="t('admin.groups.form.rpmLimitPlaceholder')"
          />
          <p class="input-hint">{{ t("admin.groups.form.rpmLimitHint") }}</p>
        </div>
        <ReasoningEffortPolicyFields
          v-if="supportsReasoningEffortPolicyPlatform(editForm.platform)"
          ref="editReasoningEffortPolicyRef"
          id-prefix="edit-group-reasoning"
          :platform="editForm.platform"
          v-model:max-effort="editForm.max_reasoning_effort"
          v-model:over-limit="editForm.max_reasoning_effort_over_limit"
          v-model:mappings="editForm.reasoning_effort_mappings"
        />
        <div v-if="editForm.subscription_type !== 'subscription'">
          <div class="mb-1.5 flex items-center gap-1">
            <label class="text-sm font-medium text-foreground">
              {{ t("admin.groups.form.exclusive") }}
            </label>
            <!-- Help Tooltip -->
            <div class="group relative inline-flex">
              <Icon
                name="questionCircle"
                size="sm"
                :stroke-width="2"
                class="cursor-help text-muted transition-colors hover:text-accent "
              />
              <!-- Tooltip Popover -->
              <div
                class="pointer-events-none absolute bottom-full left-0 z-50 mb-2 w-72 opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:opacity-100"
              >
                <div
                  class="rounded-lg bg-[var(--code-bg)] p-3 text-white shadow-[var(--shadow-pop)] "
                >
                  <p class="mb-2 text-xs font-medium">
                    {{ t("admin.groups.exclusiveTooltip.title") }}
                  </p>
                  <p class="mb-2 text-xs leading-relaxed text-muted">
                    {{ t("admin.groups.exclusiveTooltip.description") }}
                  </p>
                  <div class="rounded bg-[var(--code-bg)] p-2 ">
                    <p class="text-xs leading-relaxed text-muted">
                      <span
                        class="inline-flex items-center gap-1 text-accent"
                        ><Icon name="lightbulb" size="xs" />
                        {{ t("admin.groups.exclusiveTooltip.example") }}</span
                      >
                      {{ t("admin.groups.exclusiveTooltip.exampleContent") }}
                    </p>
                  </div>
                  <!-- Arrow -->
                  <div
                    class="absolute -bottom-1.5 left-3 h-3 w-3 rotate-45 bg-[var(--code-bg)] "
                  ></div>
                </div>
              </div>
            </div>
          </div>
          <div class="flex items-center gap-3">
            <button
              type="button"
              @click="editForm.is_exclusive = !editForm.is_exclusive"
              :class="[
 'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
 editForm.is_exclusive
 ? 'bg-accent'
 : 'bg-surface-3 ',
 ]"
            >
              <span
                :class="[
 'inline-block h-4 w-4 transform rounded-full bg-surface shadow transition-transform',
 editForm.is_exclusive ? 'translate-x-6' : 'translate-x-1',
 ]"
              />
            </button>
            <span class="text-sm text-muted">
              {{
                editForm.is_exclusive
                  ? t("admin.groups.exclusive")
                  : t("admin.groups.public")
              }}
            </span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t("admin.groups.form.status") }}</label>
          <Select v-model="editForm.status" :options="editStatusOptions" />
        </div>

        <!-- Subscription Configuration -->
        <div class="mt-4 border-t pt-4">
          <div>
            <label class="input-label">{{
              t("admin.groups.subscription.type")
            }}</label>
            <Select
              v-model="editForm.subscription_type"
              :options="subscriptionTypeOptions"
              :disabled="true"
            />
            <p class="input-hint">
              {{ t("admin.groups.subscription.typeNotEditable") }}
            </p>
          </div>

          <!-- Subscription limits (only show when subscription type is selected) -->
          <div
            v-if="editForm.subscription_type === 'subscription'"
            class="space-y-4 border-l-2 border-[color-mix(in_oklch,var(--accent)_28%,transparent)] pl-4 "
          >
            <div>
              <label class="input-label">{{
                t("admin.groups.subscription.dailyLimit")
              }}</label>
              <input
                v-model.number="editForm.daily_limit_usd"
                type="number"
                step="0.01"
                min="0"
                class="input"
                :placeholder="t('admin.groups.subscription.noLimit')"
              />
            </div>
            <div>
              <label class="input-label">{{
                t("admin.groups.subscription.weeklyLimit")
              }}</label>
              <input
                v-model.number="editForm.weekly_limit_usd"
                type="number"
                step="0.01"
                min="0"
                class="input"
                :placeholder="t('admin.groups.subscription.noLimit')"
              />
            </div>
            <div>
              <label class="input-label">{{
                t("admin.groups.subscription.monthlyLimit")
              }}</label>
              <input
                v-model.number="editForm.monthly_limit_usd"
                type="number"
                step="0.01"
                min="0"
                class="input"
                :placeholder="t('admin.groups.subscription.noLimit')"
              />
            </div>
          </div>
        </div>

        <div class="border-t pt-4">
          <div class="mb-3 flex items-center justify-between gap-3">
            <div>
              <label class="text-sm font-medium text-foreground">
                {{ t("admin.groups.modelsList.title") }}
              </label>
              <p class="mt-1 text-xs text-muted">
                {{ t("admin.groups.modelsList.hint") }}
              </p>
            </div>
            <button
              type="button"
              @click="editModelsListState.enabled = !editModelsListState.enabled"
              :class="[
 'relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors',
 editModelsListState.enabled
 ? 'bg-accent'
 : 'bg-surface-3 ',
 ]"
            >
              <span
                :class="[
 'inline-block h-4 w-4 transform rounded-full bg-surface shadow transition-transform',
 editModelsListState.enabled ? 'translate-x-6' : 'translate-x-1',
 ]"
              />
            </button>
          </div>
          <div
            v-if="editModelsListState.enabled"
            class="overflow-hidden rounded-lg border border-line bg-surface-2"
          >
            <div
              v-if="!editModelsListLoading && editModelsListState.items.length > 0"
              class="flex items-center justify-between gap-2 border-b border-line bg-surface-2 px-3 py-2 text-xs"
            >
              <span class="text-muted">
                {{
                  t("admin.groups.modelsList.selectedSummary", {
                    selected: editModelsListSelectedCount,
                    total: editModelsListState.items.length,
                  })
                }}
              </span>
              <div class="flex items-center gap-1.5">
                <button
                  type="button"
                  class="rounded px-2 py-1 font-medium text-accent transition-colors hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]  "
                  @click="selectAllModelsListItems(editModelsListState)"
                >
                  {{ t("admin.groups.modelsList.selectAll") }}
                </button>
                <button
                  type="button"
                  class="rounded px-2 py-1 font-medium text-muted transition-colors hover:bg-surface-2"
                  @click="invertModelsListSelection(editModelsListState)"
                >
                  {{ t("admin.groups.modelsList.invertSelection") }}
                </button>
              </div>
            </div>
            <div
              class="max-h-64 space-y-2 overflow-y-auto p-2"
            >
              <p v-if="editModelsListLoading" class="text-xs text-muted">
                {{ t("admin.groups.modelsList.loading") }}
              </p>
              <p
                v-else-if="editModelsListState.items.length === 0"
                class="text-xs text-muted"
              >
                {{ t("admin.groups.modelsList.empty") }}
              </p>
              <div
                v-for="(item, index) in editModelsListState.items"
                :key="item.id"
                class="flex items-center gap-2 rounded border border-line bg-surface px-3 py-2"
              >
                <input
                  v-model="item.selected"
                  type="checkbox"
                  class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
                />
                <span class="min-w-0 flex-1 break-all text-sm text-foreground">
                  {{ item.id }}
                </span>
                <button
                  type="button"
                  :disabled="index === 0"
                  class="rounded p-1 text-muted hover:bg-surface-2 hover:text-foreground disabled:opacity-40 "
                  @click="moveEditModelsListItem(index, index - 1)"
                >
                  <Icon name="arrowUp" size="sm" />
                </button>
                <button
                  type="button"
                  :disabled="index === editModelsListState.items.length - 1"
                  class="rounded p-1 text-muted hover:bg-surface-2 hover:text-foreground disabled:opacity-40 "
                  @click="moveEditModelsListItem(index, index + 1)"
                >
                  <Icon name="arrowDown" size="sm" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <GroupImagePricingFields v-model:form="editForm" />
        <GroupVideoPricingFields v-model:form="editForm" test-id-prefix="edit" />
        <!-- 高峰时段倍率配置（仅订阅类型分组） -->
        <div v-if="editForm.subscription_type === 'subscription'" class="border-t pt-4">
          <div class="mb-4 grid grid-cols-1 gap-3 md:grid-cols-2">
            <label class="flex items-center gap-2 text-sm text-foreground">
              <input
                v-model="editForm.peak_rate_enabled"
                type="checkbox"
                class="rounded border-line text-accent focus:ring-accent-500"
              />
              <span>{{ t("admin.groups.peakRate.enable") }}</span>
            </label>
          </div>
          <div
            v-if="editForm.peak_rate_enabled"
            class="mb-4 grid grid-cols-1 gap-3 sm:grid-cols-3"
          >
            <div>
              <label class="input-label">{{ t("admin.groups.peakRate.peakStart") }}</label>
              <input
                v-model="editForm.peak_start"
                type="time"
                class="input"
              />
            </div>
            <div>
              <label class="input-label">{{ t("admin.groups.peakRate.peakEnd") }}</label>
              <input
                v-model="editForm.peak_end"
                type="time"
                class="input"
              />
            </div>
            <div>
              <label class="input-label">{{ t("admin.groups.peakRate.peakMultiplier") }}</label>
              <input
                v-model.number="editForm.peak_rate_multiplier"
                type="number"
                step="0.001"
                min="0"
                class="input"
                placeholder="1"
                :title="t('admin.groups.peakRate.multiplierHint')"
              />
            </div>
          </div>
        </div>

        <!-- 分组利润控制（五个平台 token 请求） -->
        <div v-if="isProfitControlPlatform(editForm.platform)" class="border-t pt-4">
          <label class="flex items-center gap-2 text-sm text-foreground">
            <input
              v-model="editForm.profit_control_enabled"
              type="checkbox"
              class="rounded border-line text-accent focus:ring-accent-500"
            />
            <span>{{ t("admin.groups.profitControl.enable") }}</span>
          </label>
          <p class="mb-3 mt-1.5 text-xs text-muted">
            {{
              editForm.profit_control_enabled
                ? t("admin.groups.profitControl.enabledHint")
                : t("admin.groups.profitControl.disabledHint")
            }}
          </p>
          <div
            v-if="editForm.profit_control_enabled"
            class="mb-3 grid grid-cols-1 gap-3 sm:grid-cols-2"
          >
            <div>
              <label class="input-label">{{ t("admin.groups.profitControl.minMargin") }}</label>
              <input
                v-model.number="editForm.profit_min_margin_percent"
                type="number"
                step="0.1"
                min="0"
                max="99.99"
                class="input"
                placeholder="0"
                :title="t('admin.groups.profitControl.minMarginHint')"
              />
            </div>
            <div>
              <label class="input-label">{{ t("admin.groups.profitControl.safetyBuffer") }}</label>
              <input
                v-model.number="editForm.profit_safety_buffer_percent"
                type="number"
                step="0.1"
                min="0"
                max="99.99"
                class="input"
                placeholder="0"
                :title="t('admin.groups.profitControl.safetyBufferHint')"
              />
            </div>
          </div>
        </div>

        <!-- 支持的模型系列（仅 antigravity 平台） -->
        <div v-if="editForm.platform === 'antigravity'" class="border-t pt-4">
          <div class="mb-1.5 flex items-center gap-1">
            <label class="text-sm font-medium text-foreground">
              {{ t("admin.groups.supportedScopes.title") }}
            </label>
            <!-- Help Tooltip -->
            <div class="group relative inline-flex">
              <Icon
                name="questionCircle"
                size="sm"
                :stroke-width="2"
                class="cursor-help text-muted transition-colors hover:text-accent "
              />
              <div
                class="pointer-events-none absolute bottom-full left-0 z-50 mb-2 w-72 opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:opacity-100"
              >
                <div
                  class="rounded-lg bg-[var(--code-bg)] p-3 text-white shadow-[var(--shadow-pop)] "
                >
                  <p class="text-xs leading-relaxed text-muted">
                    {{ t("admin.groups.supportedScopes.tooltip") }}
                  </p>
                  <div
                    class="absolute -bottom-1.5 left-3 h-3 w-3 rotate-45 bg-[var(--code-bg)] "
                  ></div>
                </div>
              </div>
            </div>
          </div>
          <div class="space-y-2">
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                :checked="editForm.supported_model_scopes.includes('claude')"
                @change="toggleEditScope('claude')"
                class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
              />
              <span class="text-sm text-foreground">{{
                t("admin.groups.supportedScopes.claude")
              }}</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                :checked="
                  editForm.supported_model_scopes.includes('gemini_text')
                "
                @change="toggleEditScope('gemini_text')"
                class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
              />
              <span class="text-sm text-foreground">{{
                t("admin.groups.supportedScopes.geminiText")
              }}</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                :checked="
                  editForm.supported_model_scopes.includes('gemini_image')
                "
                @change="toggleEditScope('gemini_image')"
                class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
              />
              <span class="text-sm text-foreground">{{
                t("admin.groups.supportedScopes.geminiImage")
              }}</span>
            </label>
          </div>
          <p class="mt-2 text-xs text-muted">
            {{ t("admin.groups.supportedScopes.hint") }}
          </p>
        </div>

        <!-- MCP XML 协议注入（仅 antigravity 平台） -->
        <div v-if="editForm.platform === 'antigravity'" class="border-t pt-4">
          <div class="mb-1.5 flex items-center gap-1">
            <label class="text-sm font-medium text-foreground">
              {{ t("admin.groups.mcpXml.title") }}
            </label>
            <div class="group relative inline-flex">
              <Icon
                name="questionCircle"
                size="sm"
                :stroke-width="2"
                class="cursor-help text-muted transition-colors hover:text-accent "
              />
              <div
                class="pointer-events-none absolute bottom-full left-0 z-50 mb-2 w-72 opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:opacity-100"
              >
                <div
                  class="rounded-lg bg-[var(--code-bg)] p-3 text-white shadow-[var(--shadow-pop)] "
                >
                  <p class="text-xs leading-relaxed text-muted">
                    {{ t("admin.groups.mcpXml.tooltip") }}
                  </p>
                  <div
                    class="absolute -bottom-1.5 left-3 h-3 w-3 rotate-45 bg-[var(--code-bg)] "
                  ></div>
                </div>
              </div>
            </div>
          </div>
          <div class="flex items-center gap-3">
            <button
              type="button"
              @click="editForm.mcp_xml_inject = !editForm.mcp_xml_inject"
              :class="[
 'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
 editForm.mcp_xml_inject
 ? 'bg-accent'
 : 'bg-surface-3 ',
 ]"
            >
              <span
                :class="[
 'inline-block h-4 w-4 transform rounded-full bg-surface shadow transition-transform',
 editForm.mcp_xml_inject ? 'translate-x-6' : 'translate-x-1',
 ]"
              />
            </button>
            <span class="text-sm text-muted">
              {{
                editForm.mcp_xml_inject
                  ? t("admin.groups.mcpXml.enabled")
                  : t("admin.groups.mcpXml.disabled")
              }}
            </span>
          </div>
        </div>

        <!-- Claude Code 客户端限制（仅 anthropic 平台） -->
        <div v-if="editForm.platform === 'anthropic'" class="border-t pt-4">
          <div class="mb-1.5 flex items-center gap-1">
            <label class="text-sm font-medium text-foreground">
              {{ t("admin.groups.claudeCode.title") }}
            </label>
            <!-- Help Tooltip -->
            <div class="group relative inline-flex">
              <Icon
                name="questionCircle"
                size="sm"
                :stroke-width="2"
                class="cursor-help text-muted transition-colors hover:text-accent "
              />
              <div
                class="pointer-events-none absolute bottom-full left-0 z-50 mb-2 w-72 opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:opacity-100"
              >
                <div
                  class="rounded-lg bg-[var(--code-bg)] p-3 text-white shadow-[var(--shadow-pop)] "
                >
                  <p class="text-xs leading-relaxed text-muted">
                    {{ t("admin.groups.claudeCode.tooltip") }}
                  </p>
                  <div
                    class="absolute -bottom-1.5 left-3 h-3 w-3 rotate-45 bg-[var(--code-bg)] "
                  ></div>
                </div>
              </div>
            </div>
          </div>
          <div class="flex items-center gap-3">
            <button
              type="button"
              @click="editForm.claude_code_only = !editForm.claude_code_only"
              :class="[
 'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
 editForm.claude_code_only
 ? 'bg-accent'
 : 'bg-surface-3 ',
 ]"
            >
              <span
                :class="[
 'inline-block h-4 w-4 transform rounded-full bg-surface shadow transition-transform',
 editForm.claude_code_only ? 'translate-x-6' : 'translate-x-1',
 ]"
              />
            </button>
            <span class="text-sm text-muted">
              {{
                editForm.claude_code_only
                  ? t("admin.groups.claudeCode.enabled")
                  : t("admin.groups.claudeCode.disabled")
              }}
            </span>
          </div>
          <!-- 降级分组选择（仅当启用 claude_code_only 时显示） -->
          <div v-if="editForm.claude_code_only" class="mt-3">
            <label class="input-label">{{
              t("admin.groups.claudeCode.fallbackGroup")
            }}</label>
            <Select
              v-model="editForm.fallback_group_id"
              :options="fallbackGroupOptionsForEdit"
              :placeholder="t('admin.groups.claudeCode.noFallback')"
            />
            <p class="input-hint">
              {{ t("admin.groups.claudeCode.fallbackHint") }}
            </p>
          </div>
        </div>

        <!-- Codex 网页搜索按次计费（仅 openai 平台） -->
        <div
          v-if="editForm.platform === 'openai'"
          class="border-t border-line  pt-4 mt-4"
        >
          <h4 class="text-sm font-medium text-foreground mb-3">
            {{ t("admin.groups.webSearchPricing.title") }}
          </h4>
          <div>
            <label class="input-label">{{
              t("admin.groups.webSearchPricing.pricePerCall")
            }}</label>
            <input
              v-model.number="editForm.web_search_price_per_call"
              type="number"
              step="0.001"
              min="0"
              placeholder="0.01"
              class="input"
            />
            <p class="input-hint">
              {{ t("admin.groups.webSearchPricing.pricePerCallHint") }}
            </p>
            <div
              class="mt-2 rounded-lg bg-surface-2 p-3 text-xs text-muted"
            >
              {{
                t("admin.groups.webSearchPricing.finalPricePreview", {
                  price: editWebSearchFinalPricePreview,
                })
              }}
            </div>
          </div>
        </div>


        <div class="border-t border-line pt-4 mt-4 ">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <h4 class="text-sm font-medium text-foreground">{{ t("admin.groups.modelPricing.title") }}</h4>
              <p class="mt-1 text-xs text-muted">{{ t("admin.groups.modelPricing.description") }}</p>
            </div>
            <button type="button" class="btn btn-secondary shrink-0 whitespace-nowrap" @click="addGroupPricing(editForm.model_pricing)">
              <Icon name="plus" size="sm" class="mr-1" />{{ t("admin.groups.modelPricing.add") }}
            </button>
          </div>
          <label class="mt-3 flex items-start gap-2">
            <input v-model="editForm.long_context_pricing_enabled" type="checkbox" class="mt-0.5" />
            <span><span class="block text-sm text-foreground">{{ t("admin.groups.modelPricing.longContext") }}</span><span class="block text-xs text-muted">{{ t("admin.groups.modelPricing.longContextHint") }}</span></span>
          </label>
          <div class="mt-3 space-y-2">
            <PricingEntryCard v-for="(entry, index) in editForm.model_pricing" :key="index" :entry="entry" :platform="editForm.platform" hide-token-intervals @update="editForm.model_pricing[index] = $event" @remove="editForm.model_pricing.splice(index, 1)" />
          </div>
        </div>

        <!-- Grok Voice 显式定价（仅 grok 平台） -->
        <div
          v-if="editForm.platform === 'grok'"
          class="border-t border-line  pt-4 mt-4"
        >
          <h4 class="text-sm font-medium text-foreground mb-1">
            {{ t("admin.groups.explicitPricing.title") }}
          </h4>
          <p class="text-xs text-muted mb-3">
            {{ t("admin.groups.explicitPricing.description") }}
          </p>
          <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
            <div>
              <label class="input-label">{{ t("admin.groups.explicitPricing.searchPricePer1k") }}</label>
              <input
                v-model.number="editForm.search_price_per_1k"
                type="number"
                step="0.000001"
                min="0"
                class="input"
                :placeholder="t('admin.groups.explicitPricing.pricePlaceholder')"
                data-testid="edit-search-price"
              />
            </div>
            <div>
              <label class="input-label">{{ t("admin.groups.voicePricing.audioRealtimePerMin") }}</label>
              <input
                v-model.number="editForm.audio_realtime_price_per_min"
                type="number"
                step="0.000001"
                min="0"
                class="input"
                :placeholder="t('admin.groups.voicePricing.pricePlaceholder')"
                data-testid="edit-audio-realtime-price"
              />
            </div>
            <div>
              <label class="input-label">{{ t("admin.groups.voicePricing.audioTtsPerMillionChars") }}</label>
              <input
                v-model.number="editForm.audio_tts_price_per_million_chars"
                type="number"
                step="0.000001"
                min="0"
                class="input"
                :placeholder="t('admin.groups.voicePricing.pricePlaceholder')"
                data-testid="edit-audio-tts-price"
              />
            </div>
            <div>
              <label class="input-label">{{ t("admin.groups.voicePricing.audioSttPerHour") }}</label>
              <input
                v-model.number="editForm.audio_stt_price_per_hour"
                type="number"
                step="0.000001"
                min="0"
                class="input"
                :placeholder="t('admin.groups.voicePricing.pricePlaceholder')"
                data-testid="edit-audio-stt-price"
              />
            </div>
          </div>
        </div>
        <!-- OpenAI Fast 开关（OpenAI 与 Composite 平台） -->
        <div
          v-if="supportsGroupOpenAIFast(editForm.platform)"
          class="border-t border-line  pt-4 mt-4"
        >
          <h4 class="text-sm font-medium text-foreground mb-3">
            {{ t("admin.groups.openaiFast.title") }}
          </h4>
          <div class="flex items-center justify-between gap-4">
            <label class="text-sm text-muted">
              {{ t("admin.groups.openaiFast.force") }}
            </label>
            <button
              type="button"
              role="switch"
              :aria-checked="editForm.force_openai_fast"
              :aria-label="t('admin.groups.openaiFast.force')"
              data-testid="edit-force-openai-fast"
              @click="editForm.force_openai_fast = !editForm.force_openai_fast"
              class="relative inline-flex h-6 w-12 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
              :class="editForm.force_openai_fast
 ? 'bg-accent'
 : 'bg-surface-3 '"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-surface shadow ring-0 transition duration-200 ease-in-out"
                :class="editForm.force_openai_fast ? 'translate-x-6' : 'translate-x-1'"
              />
            </button>
          </div>
          <p class="text-xs text-muted mt-1">
            {{ t("admin.groups.openaiFast.hint") }}
          </p>
          <div class="flex items-center justify-between gap-4 mt-4">
            <label class="text-sm text-muted">
              {{ t("admin.groups.openaiFast.free") }}
            </label>
            <button
              type="button"
              role="switch"
              :aria-checked="editForm.free_openai_fast"
              :aria-label="t('admin.groups.openaiFast.free')"
              data-testid="edit-free-openai-fast"
              @click="editForm.free_openai_fast = !editForm.free_openai_fast"
              class="relative inline-flex h-6 w-12 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
              :class="editForm.free_openai_fast
 ? 'bg-success-500'
 : 'bg-surface-3 '"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-surface shadow ring-0 transition duration-200 ease-in-out"
                :class="editForm.free_openai_fast ? 'translate-x-6' : 'translate-x-1'"
              />
            </button>
          </div>
          <p class="text-xs text-muted mt-1">
            {{ t("admin.groups.openaiFast.freeHint") }}
          </p>
        </div>

        <!-- Codex Live 开关（OpenAI 与 Composite 平台） -->
        <div
          v-if="supportsLivePlatform(editForm.platform)"
          class="border-t border-line  pt-4 mt-4"
        >
          <h4 class="text-sm font-medium text-foreground mb-3">
            {{ t("admin.groups.openaiLive.title") }}
          </h4>
          <div class="flex items-center justify-between">
            <label class="text-sm text-muted">{{
              t("admin.groups.openaiLive.allow")
            }}</label>
            <button
              type="button"
              @click="toggleLive()"
              class="relative inline-flex h-6 w-12 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
              :class="editForm.allow_live
 ? 'bg-accent'
 : 'bg-surface-3 '"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-surface shadow ring-0 transition duration-200 ease-in-out"
                :class="editForm.allow_live ? 'translate-x-6' : 'translate-x-1'"
              />
            </button>
          </div>
          <p class="text-xs text-muted mt-1">
            {{ t("admin.groups.openaiLive.hint") }}
          </p>
        </div>

        <GroupMessagesDispatchFields v-model:form="editForm" />
        <!-- 账号过滤控制 (OpenAI/Antigravity/Anthropic/Gemini) -->
        <div
          v-if="
            ['openai', 'antigravity', 'anthropic', 'gemini'].includes(
              editForm.platform,
            )
          "
          class="border-t border-line  pt-4 mt-4 space-y-4"
        >
          <h4 class="text-sm font-medium text-foreground mb-3">
            {{ t("admin.groups.accountFilters.title") }}
          </h4>

          <!-- require_oauth_only toggle -->
          <div class="flex items-center justify-between">
            <div>
              <label class="text-sm text-muted"
                >{{ t("admin.groups.accountFilters.oauthOnly") }}</label
              >
              <p class="text-xs text-muted mt-0.5">
                {{
                  editForm.require_oauth_only
                    ? t("admin.groups.accountFilters.oauthOnlyEnabled")
                    : t("admin.groups.accountFilters.disabled")
                }}
              </p>
            </div>
            <button
              type="button"
              @click="
                editForm.require_oauth_only = !editForm.require_oauth_only
              "
              class="relative inline-flex h-6 w-12 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
              :class="editForm.require_oauth_only
 ? 'bg-accent'
 : 'bg-surface-3 '"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-surface shadow ring-0 transition duration-200 ease-in-out"
                :class="editForm.require_oauth_only
 ? 'translate-x-6'
 : 'translate-x-1'"
              />
            </button>
          </div>

          <!-- require_privacy_set toggle -->
          <div class="flex items-center justify-between">
            <div>
              <label class="text-sm text-muted"
                >{{ t("admin.groups.accountFilters.privacySetOnly") }}</label
              >
              <p class="text-xs text-muted mt-0.5">
                {{
                  editForm.require_privacy_set
                    ? t("admin.groups.accountFilters.privacySetOnlyEnabled")
                    : t("admin.groups.accountFilters.disabled")
                }}
              </p>
            </div>
            <button
              type="button"
              @click="
                editForm.require_privacy_set = !editForm.require_privacy_set
              "
              class="relative inline-flex h-6 w-12 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
              :class="editForm.require_privacy_set
 ? 'bg-accent'
 : 'bg-surface-3 '"
            >
              <span
                class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-surface shadow ring-0 transition duration-200 ease-in-out"
                :class="editForm.require_privacy_set
 ? 'translate-x-6'
 : 'translate-x-1'"
              />
            </button>
          </div>
        </div>

        <!-- 无效请求兜底（仅 anthropic/antigravity 平台，且非订阅分组） -->
        <div
          v-if="
            ['anthropic', 'antigravity'].includes(editForm.platform) &&
            editForm.subscription_type !== 'subscription'
          "
          class="border-t pt-4"
        >
          <label class="input-label">{{
            t("admin.groups.invalidRequestFallback.title")
          }}</label>
          <Select
            v-model="editForm.fallback_group_id_on_invalid_request"
            :options="invalidRequestFallbackOptionsForEdit"
            :placeholder="t('admin.groups.invalidRequestFallback.noFallback')"
          />
          <p class="input-hint">
            {{ t("admin.groups.invalidRequestFallback.hint") }}
          </p>
        </div>

        <GroupModelRoutingRulesFields
          ref="modelRoutingRef"
          :platform="editForm.platform"
          v-model:enabled="editForm.model_routing_enabled"
          v-model:rules="editModelRoutingRules"
        />
      </form>

      <template #footer>
        <div class="flex justify-end gap-3 pt-4">
          <button
            @click="closeEditModal"
            type="button"
            class="btn-glass-secondary"
          >
            {{ t("common.cancel") }}
          </button>
          <button
            type="submit"
            form="edit-group-form"
            :disabled="submitting"
            class="btn-glass-primary"
            data-tour="group-form-submit"
          >
            <svg
              v-if="submitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{ submitting ? t("admin.groups.updating") : t("common.update") }}
          </button>
        </div>
      </template>
    </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type { AdminGroup } from "@/types";
import Select from "@/components/common/Select.vue";
import BaseDialog from "@/components/common/BaseDialog.vue";
import Icon from "@/components/icons/Icon.vue";
import ReasoningEffortPolicyFields from "@/components/admin/group/ReasoningEffortPolicyFields.vue";
import PricingEntryCard from "@/components/admin/channel/PricingEntryCard.vue";
import GroupImagePricingFields from "./GroupImagePricingFields.vue";
import GroupVideoPricingFields from "./GroupVideoPricingFields.vue";
import GroupMessagesDispatchFields from "./GroupMessagesDispatchFields.vue";
import GroupModelRoutingRulesFields from "./GroupModelRoutingRulesFields.vue";
import { addGroupPricing, supportsLivePlatform } from "./groupFormShared";
import { supportsGroupOpenAIFast } from "@/views/admin/groupsOpenAIFast";
import {
  invertModelsListSelection,
  selectAllModelsListItems,
} from "@/views/admin/groupsModelsList";
import { isProfitControlPlatform } from "@/views/admin/groupsProfitControl";
import { supportsReasoningEffortPolicyPlatform } from "@/views/admin/groupsReasoningEffort";
import { useEditGroupForm } from "./useEditGroupForm";

const props = defineProps<{
  show: boolean;
  groups: AdminGroup[];
}>();

const emit = defineEmits<{
  close: [];
  updated: [];
  unsupportedLive: [];
  open: [];
}>();

const { t } = useI18n();

const {
  submitting,
  editingGroup,
  editForm,
  editModelRoutingRules,
  editModelsListState,
  editModelsListLoading,
  editModelsListSelectedCount,
  editReasoningEffortPolicyRef,
  modelRoutingRef,
  platformOptions,
  editStatusOptions,
  subscriptionTypeOptions,
  fallbackGroupOptionsForEdit,
  invalidRequestFallbackOptionsForEdit,
  copyAccountsGroupOptionsForEdit,
  toggleEditScope,
  moveEditModelsListItem,
  editWebSearchFinalPricePreview,
  toggleLive,
  confirmLive,
  open,
  closeEditModal,
  handleUpdateGroup,
} = useEditGroupForm(props, emit);

defineExpose({ open, confirmLive });
</script>
