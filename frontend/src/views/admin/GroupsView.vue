<template>
  <AppLayout>
    <PageHeader class="grp-header" :title="t('admin.groups.title')" :description="t('admin.groups.description')">
      <template #actions>
        <Button variant="secondary" :disabled="loading" :title="t('common.refresh')" @click="loadGroups">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </Button>

        <Button
          variant="secondary"
          :title="t('admin.groups.sortOrder')"
          :aria-label="t('admin.groups.sortOrder')"
          @click="showSortModal = true"
        >
          <Icon name="arrowsUpDown" size="md" />
        </Button>

        <Button class="groups-create-desktop" :data-tour="isMobile ? undefined : 'groups-create-btn'" @click="openCreateModal">
          <Icon name="plus" size="md" />
          {{ t("admin.groups.createGroup") }}
        </Button>
      </template>
    </PageHeader>
    <TablePageLayout>
      <template #filters>
        <div class="grp-summary" role="group" :aria-label="t('admin.groups.columns.status')">
          <button
            v-for="chip in summaryChips"
            :key="chip.key"
            type="button"
            class="summary-chip"
            :class="{ 'is-active': chip.active }"
            @click="chip.onClick"
          >
            <span class="summary-chip-label">
              <span class="summary-chip-dot" :style="{ background: chip.color }"></span>
              {{ chip.label }}
            </span>
            <span class="summary-chip-value num">{{ chip.count }}</span>
          </button>
        </div>
        <div class="grp-filter-row">
          <SearchInput
            v-model="searchQuery"
            :placeholder="t('admin.groups.searchGroups')"
            @search="handleSearch"
          />
          <div class="grp-filter-pill">
            <Select
              v-model="filters.platform"
              :options="platformFilterOptions"
              :placeholder="t('admin.groups.allPlatforms')"
              @change="applyFilter"
            />
          </div>
          <div class="grp-filter-pill">
            <Select
              v-model="filters.status"
              :options="statusOptions"
              :placeholder="t('admin.groups.allStatus')"
              @change="applyFilter"
            />
          </div>
          <div class="grp-filter-pill">
            <Select
              v-model="filters.is_exclusive"
              :options="exclusiveOptions"
              :placeholder="t('admin.groups.allGroups')"
              @change="applyFilter"
            />
          </div>

          <div class="grp-menu" ref="columnDropdownRef">
            <button
              type="button"
              class="grp-icon-pill"
              :title="t('admin.groups.columnSettings')"
              :aria-expanded="showColumnDropdown"
              @click="showColumnDropdown = !showColumnDropdown"
            >
              <Icon name="grid" size="sm" />
            </button>
            <div v-if="showColumnDropdown" class="dropdown grp-dropdown grp-columns-dropdown">
              <div class="dropdown-label">{{ t("admin.groups.columnSettings") }}</div>
              <button
                v-for="col in toggleableColumns"
                :key="col.key"
                type="button"
                class="dropdown-item"
                :class="{ 'is-active': isColumnVisible(col.key) }"
                @click="toggleColumn(col.key)"
              >
                <span>{{ col.label }}</span>
                <Icon v-if="isColumnVisible(col.key)" name="check" size="sm" />
              </button>
            </div>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="groups"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="sort_order"
          default-sort-order="asc"
          @sort="handleSort"
        >
          <template #cell-name="{ value }">
            <span class="font-medium text-foreground">{{
              value
            }}</span>
          </template>

          <template #cell-id="{ value }">
            <MonoCell :value="`#${value}`" />
          </template>

          <template #cell-platform="{ value }">
            <PlatformCell :platform="value" :label="t('admin.groups.platforms.' + value)" />
          </template>

          <template #cell-billing_type="{ row }">
            <div class="space-y-1">
              <!-- Type Badge -->
              <span
                :class="[
 'inline-block rounded-full px-2 py-0.5 text-xs font-medium',
 row.subscription_type === 'subscription'
 ? 'bg-accent-500/15 text-accent-700  '
 : 'bg-surface-2 text-muted  ',
 ]"
              >
                {{
                  row.subscription_type === "subscription"
                    ? t("admin.groups.subscription.subscription")
                    : t("admin.groups.subscription.standard")
                }}
              </span>
              <!-- Subscription Limits - compact single line -->
              <div
                v-if="row.subscription_type === 'subscription'"
                class="space-y-0.5 text-xs text-muted"
              >
                <div
                  v-if="
                    row.daily_limit_usd ||
                    row.weekly_limit_usd ||
                    row.monthly_limit_usd
                  "
                  class="flex flex-wrap items-center gap-x-1 gap-y-0.5"
                >
                  <span v-if="row.daily_limit_usd" class="whitespace-nowrap">
                    <span
                      v-if="usageLoading"
                      class="font-medium text-muted"
                      >—</span
                    >
                    <span
                      v-else
                      :class="getQuotaUsageClass(
 usageMap.get(row.id)?.today_cost ?? 0,
 row.daily_limit_usd
 )"
                      >{{
                        formatUsd(usageMap.get(row.id)?.today_cost ?? 0)
                      }}</span
                    >
                    <span class="text-muted">
                      / {{ formatUsd(row.daily_limit_usd) }}/{{
                        t("admin.groups.limitDay")
                      }}</span
                    >
                  </span>
                  <span
                    v-if="
                      row.daily_limit_usd &&
                      (row.weekly_limit_usd || row.monthly_limit_usd)
                    "
                    class="mx-1 text-muted"
                    >·</span
                  >
                  <span v-if="row.weekly_limit_usd" class="whitespace-nowrap"
                    >{{ formatUsd(row.weekly_limit_usd) }}/{{
                      t("admin.groups.limitWeek")
                    }}</span
                  >
                  <span
                    v-if="row.weekly_limit_usd && row.monthly_limit_usd"
                    class="mx-1 text-muted"
                    >·</span
                  >
                  <span v-if="row.monthly_limit_usd" class="whitespace-nowrap"
                    >{{ formatUsd(row.monthly_limit_usd) }}/{{
                      t("admin.groups.limitMonth")
                    }}</span
                  >
                </div>
                <span v-else class="text-muted">{{
                  t("admin.groups.subscription.noLimit")
                }}</span>
                <div class="text-muted">
                  {{ t("admin.groups.usageTotal") }}
                  <span class="ml-1 font-medium text-muted"
                    >{{
                      usageLoading
                        ? "—"
                        : formatUsd(usageMap.get(row.id)?.total_cost ?? 0)
                    }}</span
                  >
                </div>
              </div>
            </div>
          </template>

          <template #cell-rate_multiplier="{ value }">
            <span class="text-sm text-foreground"
              >{{ value }}x</span
            >
          </template>

          <template #cell-is_exclusive="{ value }">
            <TypeTagCell
              :label="value ? t('admin.groups.exclusive') : t('admin.groups.public')"
              :tone="value ? 'accent' : 'default'"
            />
          </template>

          <template #cell-account_count="{ row }">
            <div class="space-y-0.5 text-xs">
              <div>
                <span class="text-muted">{{
                  t("admin.groups.accountsAvailable")
                }}</span>
                <span
                  class="ml-1 font-medium text-success-text "
                  >{{ row.active_account_count || 0 }}</span
                >
                <span
                  class="ml-1 inline-flex items-center rounded bg-surface-2 px-1.5 py-0.5 font-medium text-foreground "
                  >{{ t("admin.groups.accountsUnit") }}</span
                >
              </div>
              <div v-if="row.rate_limited_account_count">
                <span class="text-muted">{{
                  t("admin.groups.accountsRateLimited")
                }}</span>
                <span
                  class="ml-1 font-medium text-warning-text "
                  >{{ row.rate_limited_account_count }}</span
                >
                <span
                  class="ml-1 inline-flex items-center rounded bg-surface-2 px-1.5 py-0.5 font-medium text-foreground "
                  >{{ t("admin.groups.accountsUnit") }}</span
                >
              </div>
              <div>
                <span class="text-muted">{{
                  t("admin.groups.accountsTotal")
                }}</span>
                <span
                  class="ml-1 font-medium text-foreground"
                  >{{ row.account_count || 0 }}</span
                >
                <span
                  class="ml-1 inline-flex items-center rounded bg-surface-2 px-1.5 py-0.5 font-medium text-foreground "
                  >{{ t("admin.groups.accountsUnit") }}</span
                >
              </div>
            </div>
          </template>

          <template #cell-capacity="{ row }">
            <GroupCapacityBadge
              v-if="capacityMap.get(row.id)"
              :concurrency-used="capacityMap.get(row.id)!.concurrencyUsed"
              :concurrency-max="capacityMap.get(row.id)!.concurrencyMax"
              :sessions-used="capacityMap.get(row.id)!.sessionsUsed"
              :sessions-max="capacityMap.get(row.id)!.sessionsMax"
              :rpm-used="capacityMap.get(row.id)!.rpmUsed"
              :rpm-max="capacityMap.get(row.id)!.rpmMax"
            />
            <span v-else class="text-xs text-muted">—</span>
          </template>

          <template #cell-usage="{ row }">
            <div v-if="usageLoading" class="text-xs text-muted">—</div>
            <div v-else class="space-y-0.5 text-xs">
              <div class="text-muted">
                <span class="text-muted">{{
                  t("admin.groups.usageToday")
                }}</span>
                <span class="ml-1 font-medium text-foreground"
                  >${{
                    formatCost(usageMap.get(row.id)?.today_cost ?? 0)
                  }}</span
                >
              </div>
              <div class="text-muted">
                <span class="text-muted">{{
                  t("admin.groups.usageYesterday")
                }}</span>
                <span class="ml-1 font-medium text-foreground"
                  >${{
                    formatCost(usageMap.get(row.id)?.yesterday_cost ?? 0)
                  }}</span
                >
              </div>
              <div class="text-muted">
                <span class="text-muted">{{
                  t("admin.groups.usageTotal")
                }}</span>
                <span class="ml-1 font-medium text-foreground"
                  >${{
                    formatCost(usageMap.get(row.id)?.total_cost ?? 0)
                  }}</span
                >
              </div>
            </div>
          </template>

          <template #cell-status="{ value }">
            <StatusCell :status="value" :label="t('admin.accounts.status.' + value)" />
          </template>

          <template #cell-actions="{ row }">
            <ActionsCell
              :edit-label="t('common.edit')"
              @edit="handleEdit(row)"
            >
              <template #extra>
                <button
                  type="button"
                  class="icon-btn"
                  data-testid="group-duplicate"
                  :title="
                    duplicatingGroupIds.has(row.id)
                      ? t('admin.groups.duplicating')
                      : t('admin.groups.duplicate')
                  "
                  :aria-label="
                    duplicatingGroupIds.has(row.id)
                      ? t('admin.groups.duplicating')
                      : t('admin.groups.duplicate')
                  "
                  :disabled="duplicatingGroupIds.has(row.id)"
                  @click.stop="handleDuplicate(row)"
                >
                  <Icon name="copy" size="sm" :stroke-width="1.8" />
                </button>
                <button
                  v-for="item in getGroupActionItems(row)"
                  :key="item.label"
                  type="button"
                  class="icon-btn"
                  :class="{ 'text-danger-text': item.danger }"
                  :title="item.label"
                  :aria-label="item.label"
                  :disabled="item.disabled"
                  @click.stop="item.onClick?.()"
                >
                  <Icon :name="(item.icon as any)" size="sm" :stroke-width="1.8" />
                </button>
              </template>
            </ActionsCell>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.groups.noGroupsYet')"
              :description="t('admin.groups.createFirstGroup')"
              :action-text="t('admin.groups.createGroup')"
              @action="openCreateModal"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
    <Fab class="groups-fab" :data-tour="isMobile ? 'groups-create-btn' : undefined" :label="t('admin.groups.createGroup')" @click="openCreateModal">
      <Icon name="plus" size="md" />
      {{ t("admin.groups.createGroup") }}
    </Fab>

    <!-- Create Group Modal -->
    <GroupCreateModal
      ref="createModalRef"
      :show="showCreateModal"
      :groups="groups"
      @close="showCreateModal = false"
      @created="loadGroups"
      @unsupported-live="pendingLiveForm = 'create'"
    />

    <!-- Edit Group Modal -->
    <GroupEditModal
      ref="editModalRef"
      :show="showEditModal"
      :groups="groups"
      @close="showEditModal = false"
      @updated="loadGroups"
      @unsupported-live="pendingLiveForm = 'edit'"
      @open="showEditModal = true"
    />


    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.groups.deleteGroup')"
      :message="deleteConfirmMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <ConfirmDialog
      :show="showUnsupportedLiveConfirm"
      :title="t('admin.groups.openaiLive.unsupportedTitle')"
      :message="t('admin.groups.openaiLive.unsupportedMessage')"
      :confirm-text="t('admin.groups.openaiLive.enableAnyway')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmUnsupportedLive"
      @cancel="cancelUnsupportedLive"
    />

    <!-- Sort Order Modal -->
    <GroupSortModal
      :show="showSortModal"
      @close="showSortModal = false"
      @saved="loadGroups"
    />

    <!-- Composite Routes Modal -->
    <GroupCompositeRoutesModal
      :show="showCompositeRoutesModal"
      :group="compositeRoutesGroup"
      @close="closeCompositeRoutesModal"
    />

    <!-- Group Rate Multipliers Modal -->
    <GroupRateMultipliersModal
      :show="showRateMultipliersModal"
      :group="rateMultipliersGroup"
      @close="showRateMultipliersModal = false"
      @success="loadGroups"
    />

    <!-- Group RPM Overrides Modal -->
    <GroupRPMOverridesModal
      :show="showRPMOverridesModal"
      :group="rpmOverridesGroup"
      @close="showRPMOverridesModal = false"
      @success="loadGroups"
    />
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useIsMobile } from "@/composables/useIsMobile";
import { useAppStore } from "@/stores/app";
import { adminAPI } from "@/api/admin";
import type { AdminGroup, GroupPlatform } from "@/types";
import { GROUP_PLATFORM_OPTIONS } from "@/constants/platforms";
import type { Column } from "@/components/common/types";
import AppLayout from "@/components/layout/AppLayout.vue";
import TablePageLayout from "@/components/layout/TablePageLayout.vue";
import PageHeader from "@/components/ui/PageHeader.vue";
import Button from "@/components/ui/Button.vue";
import Fab from "@/components/ui/Fab.vue";
import SearchInput from "@/components/common/SearchInput.vue";
import DataTable from "@/components/common/DataTable.vue";
import Pagination from "@/components/common/Pagination.vue";
import ConfirmDialog from "@/components/common/ConfirmDialog.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import Select from "@/components/common/Select.vue";
import { PlatformCell, TypeTagCell, StatusCell, MonoCell, ActionsCell, type ActionsCellItem } from "@/components/common/cells";
import Icon from "@/components/icons/Icon.vue";
import GroupRateMultipliersModal from "@/components/admin/group/GroupRateMultipliersModal.vue";
import GroupRPMOverridesModal from "@/components/admin/group/GroupRPMOverridesModal.vue";
import GroupSortModal from "@/components/admin/group/GroupSortModal.vue";
import GroupCompositeRoutesModal from "@/components/admin/group/GroupCompositeRoutesModal.vue";
import GroupCapacityBadge from "@/components/common/GroupCapacityBadge.vue";
import GroupCreateModal from "@/components/admin/group/GroupCreateModal.vue";
import GroupEditModal from "@/components/admin/group/GroupEditModal.vue";
import { extractApiErrorMessage } from "@/utils/apiError";
import { getPersistedPageSize } from "@/composables/usePersistedPageSize";
import { loadLiveCapability } from "@/components/admin/group/groupFormShared";

const { t } = useI18n();
const { isMobile } = useIsMobile();
const appStore = useAppStore();

const ALWAYS_VISIBLE_COLUMNS = new Set(["name", "actions"]);
// Default hidden columns (hidden on first load / after schema bumps).
const DEFAULT_HIDDEN_COLUMNS = ["id"];
const HIDDEN_COLUMNS_KEY = "group-hidden-columns";
// Bump when adding new default-hidden columns so existing admins pick them up once.
const COLUMN_SETTINGS_VERSION_KEY = "group-column-settings-version";
const COLUMN_SETTINGS_VERSION = 2;
const VERSION_NEW_HIDDEN_COLUMNS: Record<number, string[]> = {
  2: ["id"],
};

const allColumns = computed<Column[]>(() => [
  { key: "name", label: t("admin.groups.columns.name"), sortable: true },
  { key: "id", label: t("admin.groups.columns.id"), sortable: true },
  {
    key: "platform",
    label: t("admin.groups.columns.platform"),
    sortable: true,
  },
  {
    key: "billing_type",
    label: t("admin.groups.columns.billingType"),
    sortable: true,
  },
  {
    key: "rate_multiplier",
    label: t("admin.groups.columns.rateMultiplier"),
    sortable: true,
  },
  {
    key: "is_exclusive",
    label: t("admin.groups.columns.type"),
    sortable: true,
  },
  {
    key: "account_count",
    label: t("admin.groups.columns.accounts"),
    sortable: true,
  },
  {
    key: "capacity",
    label: t("admin.groups.columns.capacity"),
    sortable: false,
  },
  { key: "usage", label: t("admin.groups.columns.usage"), sortable: false },
  { key: "status", label: t("admin.groups.columns.status"), sortable: true },
  { key: "actions", label: t("admin.groups.columns.actions"), sortable: false },
]);

const toggleableColumns = computed(() =>
  allColumns.value.filter((col) => !ALWAYS_VISIBLE_COLUMNS.has(col.key)),
);
const hiddenColumns = reactive<Set<string>>(new Set());
const showColumnDropdown = ref(false);
const columnDropdownRef = ref<HTMLElement | null>(null);

// Header "更多" dropdown（排序入口）

const getValidHiddenColumnKeys = () =>
  new Set(toggleableColumns.value.map((col) => col.key));

const loadSavedColumns = () => {
  hiddenColumns.clear();
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY);
    const validKeys = getValidHiddenColumnKeys();

    if (saved) {
      const parsed = JSON.parse(saved);
      if (Array.isArray(parsed)) {
        parsed
          .filter(
            (key): key is string =>
              typeof key === "string" && validKeys.has(key),
          )
          .forEach((key) => hiddenColumns.add(key));
      }

      // Existing admins: auto-hide columns newly added as default-hidden.
      const storedVersion = Number(
        localStorage.getItem(COLUMN_SETTINGS_VERSION_KEY) ?? "1",
      );
      if (storedVersion < COLUMN_SETTINGS_VERSION) {
        let mutated = false;
        for (let v = storedVersion + 1; v <= COLUMN_SETTINGS_VERSION; v++) {
          for (const key of VERSION_NEW_HIDDEN_COLUMNS[v] ?? []) {
            if (validKeys.has(key) && !hiddenColumns.has(key)) {
              hiddenColumns.add(key);
              mutated = true;
            }
          }
        }
        if (mutated) {
          saveColumnsToStorage();
        } else {
          localStorage.setItem(
            COLUMN_SETTINGS_VERSION_KEY,
            String(COLUMN_SETTINGS_VERSION),
          );
        }
      }
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach((key) => {
        if (validKeys.has(key)) hiddenColumns.add(key);
      });
      saveColumnsToStorage();
    }
  } catch (error) {
    console.error("Failed to load group column settings:", error);
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key));
  }
};

const saveColumnsToStorage = () => {
  try {
    const validKeys = getValidHiddenColumnKeys();
    const keys = [...hiddenColumns].filter((key) => validKeys.has(key));
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify(keys));
    localStorage.setItem(
      COLUMN_SETTINGS_VERSION_KEY,
      String(COLUMN_SETTINGS_VERSION),
    );
  } catch (error) {
    console.error("Failed to save group column settings:", error);
  }
};

const isColumnVisible = (key: string) => !hiddenColumns.has(key);
const hasVisibleUsageSummaryConsumer = computed(
  () => isColumnVisible("usage") || isColumnVisible("billing_type"),
);
const hasVisibleCapacityColumn = computed(() => isColumnVisible("capacity"));

const toggleColumn = (key: string) => {
  const validKeys = getValidHiddenColumnKeys();
  if (!validKeys.has(key)) return;

  const wasHidden = hiddenColumns.has(key);
  if (wasHidden) {
    hiddenColumns.delete(key);
  } else {
    hiddenColumns.add(key);
  }
  saveColumnsToStorage();

  if (wasHidden && (key === "usage" || key === "billing_type")) {
    loadUsageSummary();
  }
  if (wasHidden && key === "capacity") {
    loadCapacitySummary();
  }
};

const columns = computed<Column[]>(() =>
  allColumns.value.filter(
    (col) => ALWAYS_VISIBLE_COLUMNS.has(col.key) || !hiddenColumns.has(col.key),
  ),
);

if (typeof window !== "undefined") {
  loadSavedColumns();
}


// Filter options
const statusOptions = computed(() => [
  { value: "", label: t("admin.groups.allStatus") },
  { value: "active", label: t("admin.accounts.status.active") },
  { value: "inactive", label: t("admin.accounts.status.inactive") },
]);

// ListPage 配方：三段迷你统计卡 → 一行可点击的 .summary-chip。
// 计数只统计当前页（后端未提供全量分桶接口，与旧 MiniStatCard 的口径一致，非功能回退）。
interface GroupSummaryChip {
  key: string;
  label: string;
  color: string;
  count: number;
  active: boolean;
  onClick: () => void;
}

const applyFilter = () => {
  pagination.page = 1;
  loadGroups();
};

const toggleStatusFilter = (value: string) => {
  filters.status = filters.status === value ? "" : value;
  applyFilter();
};

const toggleExclusiveFilter = (value: string) => {
  filters.is_exclusive = filters.is_exclusive === value ? "" : value;
  applyFilter();
};

const summaryChips = computed<GroupSummaryChip[]>(() => [
  {
    key: "all",
    label: t("common.total"),
    color: "var(--accent)",
    count: pagination.total,
    active: filters.status === "" && filters.is_exclusive === "",
    onClick: () => {
      filters.status = "";
      filters.is_exclusive = "";
      applyFilter();
    },
  },
  {
    key: "active",
    label: t("common.currentPageLabel", { label: t("common.active") }),
    color: "var(--success)",
    count: groups.value.filter((group) => group.status === "active").length,
    active: filters.status === "active",
    onClick: () => toggleStatusFilter("active"),
  },
  {
    key: "exclusive",
    label: t("common.currentPageLabel", { label: t("admin.groups.exclusive") }),
    color: "var(--warning)",
    count: groups.value.filter((group) => group.is_exclusive).length,
    active: filters.is_exclusive === "true",
    onClick: () => toggleExclusiveFilter("true"),
  },
]);

const exclusiveOptions = computed(() => [
  { value: "", label: t("admin.groups.allGroups") },
  { value: "true", label: t("admin.groups.exclusive") },
  { value: "false", label: t("admin.groups.nonExclusive") },
]);


const platformFilterOptions = computed(() => [
  { value: "", label: t("admin.groups.allPlatforms") },
  ...GROUP_PLATFORM_OPTIONS,
]);

const groups = ref<AdminGroup[]>([]);
const loading = ref(false);
type GroupUsageSummary = {
  today_cost: number;
  yesterday_cost: number;
  total_cost: number;
};

const usageMap = ref<Map<number, GroupUsageSummary>>(new Map());
const usageLoading = ref(false);
const capacityMap = ref<
  Map<
    number,
    {
      concurrencyUsed: number;
      concurrencyMax: number;
      sessionsUsed: number;
      sessionsMax: number;
      rpmUsed: number;
      rpmMax: number;
    }
  >
>(new Map());
const searchQuery = ref("");
const filters = reactive({
  platform: "",
  status: "",
  is_exclusive: "",
});
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0,
});
const sortState = reactive({
  sort_by: "sort_order",
  sort_order: "asc" as "asc" | "desc",
});

let abortController: AbortController | null = null;

const showCreateModal = ref(false);
const showEditModal = ref(false);
const showDeleteDialog = ref(false);
const pendingLiveForm = ref<"create" | "edit" | null>(null);
const showUnsupportedLiveConfirm = computed(
  () => pendingLiveForm.value !== null,
);

const showSortModal = ref(false);

const deletingGroup = ref<AdminGroup | null>(null);
const duplicatingGroupIds = reactive(new Set<number>());
const showRateMultipliersModal = ref(false);
const rateMultipliersGroup = ref<AdminGroup | null>(null);
const showRPMOverridesModal = ref(false);
const rpmOverridesGroup = ref<AdminGroup | null>(null);
const showCompositeRoutesModal = ref(false);
const compositeRoutesGroup = ref<AdminGroup | null>(null);

const createModalRef = ref<{
  confirmLive: () => void;
  refreshModelsList: () => void;
} | null>(null);
const editModalRef = ref<{
  confirmLive: () => void;
  open: (group: AdminGroup) => void;
} | null>(null);

const deleteConfirmMessage = computed(() => {
  if (!deletingGroup.value) {
    return "";
  }
  if (deletingGroup.value.subscription_type === "subscription") {
    return t("admin.groups.deleteConfirmSubscription", {
      name: deletingGroup.value.name,
    });
  }
  return t("admin.groups.deleteConfirm", { name: deletingGroup.value.name });
});

const confirmUnsupportedLive = () => {
  if (pendingLiveForm.value === "create") createModalRef.value?.confirmLive();
  if (pendingLiveForm.value === "edit") editModalRef.value?.confirmLive();
  pendingLiveForm.value = null;
};

const cancelUnsupportedLive = () => {
  pendingLiveForm.value = null;
};

const loadGroups = async () => {
  if (abortController) {
    abortController.abort();
  }
  const currentController = new AbortController();
  abortController = currentController;
  const { signal } = currentController;
  loading.value = true;
  try {
    const response = await adminAPI.groups.list(
      pagination.page,
      pagination.page_size,
      {
        platform: (filters.platform as GroupPlatform) || undefined,
        status: filters.status as any,
        is_exclusive: filters.is_exclusive
          ? filters.is_exclusive === "true"
          : undefined,
        search: searchQuery.value.trim() || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order,
      },
      { signal },
    );
    if (signal.aborted) return;
    groups.value = response.items;
    pagination.total = response.total;
    pagination.pages = response.pages;
    if (hasVisibleUsageSummaryConsumer.value) {
      loadUsageSummary();
    } else {
      usageLoading.value = false;
    }
    if (hasVisibleCapacityColumn.value) {
      loadCapacitySummary();
    }
  } catch (error: any) {
    if (
      signal.aborted ||
      error?.name === "AbortError" ||
      error?.code === "ERR_CANCELED"
    ) {
      return;
    }
    appStore.showError(t("admin.groups.failedToLoad"));
    console.error("Error loading groups:", error);
  } finally {
    if (abortController === currentController && !signal.aborted) {
      loading.value = false;
    }
  }
};

const formatCost = (cost: number): string => {
  if (cost >= 1000) return cost.toFixed(0);
  if (cost >= 100) return cost.toFixed(1);
  return cost.toFixed(2);
};

const formatUsd = (cost: number | null | undefined): string =>
  `$${formatCost(cost ?? 0)}`;

const getQuotaUsageClass = (
  used: number,
  limit: number | null | undefined,
): string => {
  if (!limit || limit <= 0) {
    return "font-medium text-foreground ";
  }
  const ratio = used / limit;
  if (ratio >= 1) {
    return "font-semibold text-danger-text ";
  }
  if (ratio >= 0.8) {
    return "font-semibold text-warning-text ";
  }
  return "font-medium text-foreground ";
};

const loadUsageSummary = async () => {
  if (!hasVisibleUsageSummaryConsumer.value) {
    usageLoading.value = false;
    return;
  }
  usageLoading.value = true;
  try {
    const data = await adminAPI.groups.getUsageSummary();
    const map = new Map<number, GroupUsageSummary>();
    for (const item of data) {
      map.set(item.group_id, {
        today_cost: item.today_cost,
        yesterday_cost: item.yesterday_cost,
        total_cost: item.total_cost,
      });
    }
    usageMap.value = map;
  } catch (error) {
    console.error("Error loading group usage summary:", error);
  } finally {
    usageLoading.value = false;
  }
};

const loadCapacitySummary = async () => {
  if (!hasVisibleCapacityColumn.value) {
    return;
  }
  try {
    const data = await adminAPI.groups.getCapacitySummary();
    const map = new Map<
      number,
      {
        concurrencyUsed: number;
        concurrencyMax: number;
        sessionsUsed: number;
        sessionsMax: number;
        rpmUsed: number;
        rpmMax: number;
      }
    >();
    for (const item of data) {
      map.set(item.group_id, {
        concurrencyUsed: item.concurrency_used,
        concurrencyMax: item.concurrency_max,
        sessionsUsed: item.sessions_used,
        sessionsMax: item.sessions_max,
        rpmUsed: item.rpm_used,
        rpmMax: item.rpm_max,
      });
    }
    capacityMap.value = map;
  } catch (error) {
    console.error("Error loading group capacity summary:", error);
  }
};

// ListPage 配方：SearchInput 组件内置 300ms 防抖，这里只需响应它 debounce 后触发的 `search` 事件。
const handleSearch = () => {
  pagination.page = 1;
  loadGroups();
};

const handlePageChange = (page: number) => {
  pagination.page = page;
  loadGroups();
};

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize;
  pagination.page = 1;
  loadGroups();
};

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key;
  sortState.sort_order = order;
  pagination.page = 1;
  loadGroups();
};


const openCreateModal = () => {
  showCreateModal.value = true;
  createModalRef.value?.refreshModelsList();
};

const handleEdit = (group: AdminGroup) => {
  editModalRef.value?.open(group);
};

const handleRateMultipliers = (group: AdminGroup) => {
  rateMultipliersGroup.value = group;
  showRateMultipliersModal.value = true;
};

const handleRPMOverrides = (group: AdminGroup) => {
  rpmOverridesGroup.value = group;
  showRPMOverridesModal.value = true;
};

const handleDuplicate = async (group: AdminGroup) => {
  if (duplicatingGroupIds.has(group.id)) return;

  duplicatingGroupIds.add(group.id);
  try {
    const duplicate = await adminAPI.groups.duplicate(group.id);
    appStore.showSuccess(
      t("admin.groups.duplicateSuccess", { name: duplicate.name }),
    );
    await loadGroups();
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t("admin.groups.duplicateFailed")),
    );
  } finally {
    duplicatingGroupIds.delete(group.id);
  }
};

const handleCompositeRoutes = (group: AdminGroup) => {
  compositeRoutesGroup.value = group;
  showCompositeRoutesModal.value = true;
};

const closeCompositeRoutesModal = () => {
  showCompositeRoutesModal.value = false;
  compositeRoutesGroup.value = null;
};

const handleDelete = (group: AdminGroup) => {
  deletingGroup.value = group;
  showDeleteDialog.value = true;
};

// Keep the pre-Glass row actions directly visible; the shared cell supplies edit.
const getGroupActionItems = (group: AdminGroup): ActionsCellItem[] => {
  const items: ActionsCellItem[] = [];
  if (group.platform === "composite") {
    items.push({
      label: t("admin.groups.compositeRoutes.action"),
      icon: "swap",
      onClick: () => handleCompositeRoutes(group),
    });
  }
  items.push(
    { label: t("admin.groups.rateMultipliers"), icon: "dollar", onClick: () => handleRateMultipliers(group) },
    { label: t("admin.groups.rpmOverrides"), icon: "bolt", onClick: () => handleRPMOverrides(group) },
    { label: t("common.delete"), icon: "trash", danger: true, onClick: () => handleDelete(group) },
  );
  return items;
};

const confirmDelete = async () => {
  if (!deletingGroup.value) return;

  try {
    await adminAPI.groups.delete(deletingGroup.value.id);
    appStore.showSuccess(t("admin.groups.groupDeleted"));
    showDeleteDialog.value = false;
    deletingGroup.value = null;
    loadGroups();
  } catch (error: any) {
    appStore.showError(
      error.response?.data?.detail || t("admin.groups.failedToDelete"),
    );
    console.error("Error deleting group:", error);
  }
};

// 监听 subscription_type 变化，订阅模式时 is_exclusive 默认为 true；标准模式清空高峰配置

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement;
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(target)) {
    showColumnDropdown.value = false;
  }
};

onMounted(() => {
  loadGroups();
  void loadLiveCapability();
  document.addEventListener("click", handleClickOutside);
});

onUnmounted(() => {
  document.removeEventListener("click", handleClickOutside);
});
</script>
<style scoped>
/* ---------- Header "更多" 下拉 ---------- */
.grp-menu {
  position: relative;
  display: inline-flex;
  flex: none;
}

.grp-dropdown {
  top: 100%;
  right: 0;
  margin-top: 6px;
  min-width: 200px;
}

.grp-columns-dropdown {
  max-height: 60vh;
  overflow-y: auto;
}

/* ---------- Summary chips ---------- */
.grp-summary {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin-bottom: 14px;
}

.grp-summary .summary-chip {
  width: 100%;
  text-align: left;
}

/* ---------- Filter row ---------- */
.grp-filter-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-height: 36px;
}

.grp-filter-row :deep(.search-input) {
  width: 260px;
  flex: none;
}

.grp-filter-pill {
  width: 112px;
  flex: none;
}

.grp-filter-row > .grp-menu {
  margin-left: auto;
}

.grp-icon-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  flex: none;
  border-radius: var(--radius-field);
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 85%, transparent);
  box-shadow: var(--field-shadow);
  color: var(--muted);
  cursor: pointer;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.grp-icon-pill:hover {
  color: var(--foreground);
  border-color: color-mix(in oklch, var(--foreground) 18%, transparent);
}

.groups-fab {
  display: none;
}
@media (max-width: 767px) {
  .groups-create-desktop {
    display: none;
  }
  .groups-fab {
    display: inline-flex;
  }

  .grp-summary {
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
  }

  .grp-filter-row :deep(.search-input) {
    width: 100%;
  }
}
</style>
