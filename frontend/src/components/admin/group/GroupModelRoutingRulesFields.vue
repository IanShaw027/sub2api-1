<template>
<!-- 模型路由配置（仅 anthropic 平台） -->
<div v-if="platform === 'anthropic'" class="border-t pt-4">
  <div class="mb-1.5 flex items-center gap-1">
    <label class="text-sm font-medium text-foreground">
      {{ t("admin.groups.modelRouting.title") }}
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
        class="pointer-events-none absolute bottom-full left-0 z-50 mb-2 w-80 opacity-0 transition-all duration-200 group-hover:pointer-events-auto group-hover:opacity-100"
      >
        <div
          class="rounded-lg bg-[var(--code-bg)] p-3 text-white shadow-[var(--shadow-pop)] "
        >
          <p class="text-xs leading-relaxed text-muted">
            {{ t("admin.groups.modelRouting.tooltip") }}
          </p>
          <div
            class="absolute -bottom-1.5 left-3 h-3 w-3 rotate-45 bg-[var(--code-bg)] "
          ></div>
        </div>
      </div>
    </div>
  </div>
  <!-- 启用开关 -->
  <div class="flex items-center gap-3 mb-3">
    <button
      type="button"
      @click="
        enabled =
          !enabled
      "
      :class="[
 'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
 enabled
 ? 'bg-accent'
 : 'bg-surface-3 ',
 ]"
    >
      <span
        :class="[
 'inline-block h-4 w-4 transform rounded-full bg-surface shadow transition-transform',
 enabled
 ? 'translate-x-6'
 : 'translate-x-1',
 ]"
      />
    </button>
    <span class="text-sm text-muted">
      {{
        enabled
          ? t("admin.groups.modelRouting.enabled")
          : t("admin.groups.modelRouting.disabled")
      }}
    </span>
  </div>
  <p
    v-if="!enabled"
    class="text-xs text-muted mb-3"
  >
    {{ t("admin.groups.modelRouting.disabledHint") }}
  </p>
  <p v-else class="text-xs text-muted mb-3">
    {{ t("admin.groups.modelRouting.noRulesHint") }}
  </p>
  <!-- 路由规则列表（仅在启用时显示） -->
  <div v-if="enabled" class="space-y-3">
    <div
      v-for="rule in rules"
      :key="getRuleKey(rule)"
      class="rounded-lg border border-line p-3"
    >
      <div class="flex items-start gap-3">
        <div class="flex-1 space-y-2">
          <div>
            <label class="input-label text-xs">{{
              t("admin.groups.modelRouting.modelPattern")
            }}</label>
            <input
              v-model="rule.pattern"
              type="text"
              class="input text-sm"
              :placeholder="
                t('admin.groups.modelRouting.modelPatternPlaceholder')
              "
            />
          </div>
          <div>
            <label class="input-label text-xs">{{
              t("admin.groups.modelRouting.accounts")
            }}</label>
            <!-- 已选账号标签 -->
            <div
              v-if="rule.accounts.length > 0"
              class="flex flex-wrap gap-1.5 mb-2"
            >
              <span
                v-for="account in rule.accounts"
                :key="account.id"
                class="inline-flex items-center gap-1 rounded-full bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] px-2.5 py-1 text-xs font-medium text-accent  "
              >
                {{ account.name }}
                <button
                  type="button"
                  @click="removeSelectedAccount(rule, account.id)"
                  class="ml-0.5 text-accent hover:text-accent "
                >
                  <Icon name="x" size="xs" />
                </button>
              </span>
            </div>
            <!-- 账号搜索输入框 -->
            <div class="relative account-search-container">
              <input
                v-model="
                  accountSearchKeyword[getRuleSearchKey(rule)]
                "
                type="text"
                class="input text-sm"
                :placeholder="
                  t(
                    'admin.groups.modelRouting.searchAccountPlaceholder',
                  )
                "
                @input="searchAccountsByRule(rule)"
                @focus="onAccountSearchFocus(rule)"
              />
              <!-- 搜索结果下拉框 -->
              <div
                v-if="
                  showAccountDropdown[getRuleSearchKey(rule)] &&
                  accountSearchResults[getRuleSearchKey(rule)]
                    ?.length > 0
                "
                class="absolute z-50 mt-1 max-h-48 w-full overflow-auto rounded-lg border bg-surface shadow-[var(--shadow-pop)]"
              >
                <button
                  v-for="account in accountSearchResults[
                    getRuleSearchKey(rule)
                  ]"
                  :key="account.id"
                  type="button"
                  @click="selectAccount(rule, account)"
                  class="w-full px-3 py-2 text-left text-sm hover:bg-surface-2"
                  :class="{
 'opacity-50': rule.accounts.some(
 (a) => a.id === account.id,
 ),
 }"
                  :disabled="
                    rule.accounts.some((a) => a.id === account.id)
                  "
                >
                  <span>{{ account.name }}</span>
                  <span class="ml-2 text-xs text-muted"
                    >#{{ account.id }}</span
                  >
                </button>
              </div>
            </div>
            <p class="text-xs text-muted mt-1">
              {{ t("admin.groups.modelRouting.accountsHint") }}
            </p>
          </div>
        </div>
        <button
          type="button"
          @click="removeRule(rule)"
          class="mt-5 p-1.5 text-muted hover:text-danger-text transition-colors"
          :title="t('admin.groups.modelRouting.removeRule')"
        >
          <Icon name="trash" size="sm" />
        </button>
      </div>
    </div>
  </div>
  <!-- 添加规则按钮（仅在启用时显示） -->
  <button
    v-if="enabled"
    type="button"
    @click="addRule"
    class="mt-3 flex items-center gap-1.5 text-sm text-accent hover:text-accent  "
  >
    <Icon name="plus" size="sm" />
    {{ t("admin.groups.modelRouting.addRule") }}
  </button>
</div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { adminAPI } from "@/api/admin";
import Icon from "@/components/icons/Icon.vue";
import { createStableObjectKeyResolver } from "@/utils/stableObjectKey";
import { useKeyedDebouncedSearch } from "@/composables/useKeyedDebouncedSearch";
import type { GroupPlatform } from "@/types";
import type { ModelRoutingRule, SimpleAccount } from "./groupFormShared";

defineProps<{
  platform: GroupPlatform;
}>();

const { t } = useI18n();

const enabled = defineModel<boolean>("enabled", { required: true });
const rules = defineModel<ModelRoutingRule[]>("rules", { required: true });

// 规则对象稳定 key（避免使用 index 导致状态错位）
const resolveRuleKey = createStableObjectKeyResolver<ModelRoutingRule>("rule");
const getRuleKey = (rule: ModelRoutingRule) => resolveRuleKey(rule);
const getRuleSearchKey = (rule: ModelRoutingRule) => resolveRuleKey(rule);

// 账号搜索相关状态
const accountSearchKeyword = ref<Record<string, string>>({});
const accountSearchResults = ref<Record<string, SimpleAccount[]>>({});
const showAccountDropdown = ref<Record<string, boolean>>({});

const clearAccountSearchStateByKey = (key: string) => {
  delete accountSearchKeyword.value[key];
  delete accountSearchResults.value[key];
  delete showAccountDropdown.value[key];
};

const clearAllAccountSearchState = () => {
  accountSearchKeyword.value = {};
  accountSearchResults.value = {};
  showAccountDropdown.value = {};
};

const accountSearchRunner = useKeyedDebouncedSearch<SimpleAccount[]>({
  delay: 300,
  search: async (keyword, { signal }) => {
    const res = await adminAPI.accounts.list(
      1,
      20,
      {
        search: keyword,
        platform: "anthropic",
      },
      { signal },
    );
    return res.items.map((account) => ({ id: account.id, name: account.name }));
  },
  onSuccess: (key, result) => {
    accountSearchResults.value[key] = result;
  },
  onError: (key) => {
    accountSearchResults.value[key] = [];
  },
});

// 搜索账号（仅限 anthropic 平台）
const searchAccounts = (key: string) => {
  accountSearchRunner.trigger(key, accountSearchKeyword.value[key] || "");
};

const searchAccountsByRule = (rule: ModelRoutingRule) => {
  searchAccounts(getRuleSearchKey(rule));
};

// 选择账号
const selectAccount = (rule: ModelRoutingRule, account: SimpleAccount) => {
  if (!rule) return;

  // 检查是否已选择
  if (!rule.accounts.some((a) => a.id === account.id)) {
    rule.accounts.push(account);
  }

  // 清空搜索
  const key = getRuleSearchKey(rule);
  accountSearchKeyword.value[key] = "";
  showAccountDropdown.value[key] = false;
};

// 移除已选账号
const removeSelectedAccount = (rule: ModelRoutingRule, accountId: number) => {
  if (!rule) return;

  rule.accounts = rule.accounts.filter((a) => a.id !== accountId);
};

// 处理账号搜索输入框聚焦
const onAccountSearchFocus = (rule: ModelRoutingRule) => {
  const key = getRuleSearchKey(rule);
  showAccountDropdown.value[key] = true;
  // 如果没有搜索结果，触发一次搜索
  if (!accountSearchResults.value[key]?.length) {
    searchAccounts(key);
  }
};

// 添加路由规则
const addRule = () => {
  rules.value.push({ pattern: "", accounts: [] });
};

// 删除路由规则
const removeRule = (rule: ModelRoutingRule) => {
  const index = rules.value.indexOf(rule);
  if (index === -1) return;

  const key = getRuleSearchKey(rule);
  accountSearchRunner.clearKey(key);
  clearAccountSearchStateByKey(key);
  rules.value.splice(index, 1);
};

const reset = () => {
  rules.value = [];
  clearAllAccountSearchState();
};

// 点击外部关闭账号搜索下拉框
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement;
  if (!target.closest(".account-search-container")) {
    Object.keys(showAccountDropdown.value).forEach((key) => {
      showAccountDropdown.value[key] = false;
    });
  }
};

onMounted(() => {
  document.addEventListener("click", handleClickOutside);
});

onUnmounted(() => {
  document.removeEventListener("click", handleClickOutside);
  accountSearchRunner.clearAll();
  clearAllAccountSearchState();
});

defineExpose({ reset });
</script>
