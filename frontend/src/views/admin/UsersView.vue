<template>
  <AppLayout>
    <PageHeader class="usr-header" :title="t('admin.users.title')" :description="t('admin.users.description')">
      <template #actions>
        <Button variant="secondary" :disabled="loading" :title="t('common.refresh')" @click="loadUsers">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </Button>

        <!-- 更多：筛选设置 + 属性配置 -->
        <div class="usr-menu" ref="moreDropdownRef">
          <Button variant="secondary" :aria-expanded="showMoreDropdown" @click="showMoreDropdown = !showMoreDropdown">
            <span>{{ t('common.more') }}</span>
            <Icon name="chevronDown" size="xs" />
          </Button>
          <div v-if="showMoreDropdown" class="dropdown usr-dropdown">
            <div class="dropdown-label">{{ t('admin.users.filterSettings') }}</div>
            <button
              v-for="filter in builtInFilters"
              :key="filter.key"
              type="button"
              class="dropdown-item"
              :class="{ 'is-active': visibleFilters.has(filter.key) }"
              @click="toggleBuiltInFilter(filter.key)"
            >
              <span>{{ filter.name }}</span>
              <Icon v-if="visibleFilters.has(filter.key)" name="check" size="sm" />
            </button>
            <template v-if="filterableAttributes.length > 0">
              <div class="dropdown-divider"></div>
              <button
                v-for="attr in filterableAttributes"
                :key="attr.id"
                type="button"
                class="dropdown-item"
                :class="{ 'is-active': visibleFilters.has(`attr_${attr.id}`) }"
                @click="toggleAttributeFilter(attr)"
              >
                <span>{{ attr.name }}</span>
                <Icon v-if="visibleFilters.has(`attr_${attr.id}`)" name="check" size="sm" />
              </button>
            </template>
            <div class="dropdown-divider"></div>
            <button type="button" class="dropdown-item" @click="showAttributesModal = true; showMoreDropdown = false">
              <Icon name="cog" size="sm" />
              <span>{{ t('admin.users.attributes.configButton') }}</span>
            </button>
          </div>
        </div>

        <Button
          v-if="selectedCount > 0"
          variant="secondary"
          data-test="bulk-edit-limits"
          @click="showBulkEditModal = true"
        >
          <Icon name="users" size="md" class="mr-2" />
          {{ t('admin.users.bulkLimits.action', { count: selectedCount }) }}
        </Button>
        <Button class="users-create-desktop" @click="showCreateModal = true">
          <Icon name="plus" size="md" />
          {{ t('admin.users.createUser') }}
        </Button>
      </template>
    </PageHeader>
    <TablePageLayout>
      <template #filters>
        <div class="usr-summary" role="group" :aria-label="t('admin.users.columns.status')">
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
        <div class="usr-filter-row">
          <SearchInput
            v-model="searchQuery"
            :placeholder="t('admin.users.searchUsers')"
            @search="handleSearch"
          />
          <div v-if="visibleFilters.has('role')" class="usr-filter-pill">
            <Select
              v-model="filters.role"
              :options="[
                { value: '', label: t('admin.users.allRoles') },
                { value: 'admin', label: t('admin.users.admin') },
                { value: 'user', label: t('admin.users.user') }
              ]"
              @change="applyFilter"
            />
          </div>
          <div v-if="visibleFilters.has('status')" class="usr-filter-pill">
            <Select
              v-model="filters.status"
              :options="[
                { value: '', label: t('admin.users.allStatus') },
                { value: 'active', label: t('common.active') },
                { value: 'disabled', label: t('admin.users.disabled') }
              ]"
              @change="applyFilter"
            />
          </div>
          <div v-if="visibleFilters.has('group')" class="usr-filter-pill usr-filter-pill-wide">
            <Select
              v-model="filters.group"
              :options="groupFilterOptions"
              searchable
              creatable
              :creatable-prefix="t('admin.users.fuzzySearch')"
              :search-placeholder="t('admin.users.searchAuthorizedGroups')"
              @change="applyFilter"
            />
          </div>
          <div v-if="visibleFilters.has('apiKeyGroup')" class="usr-filter-pill usr-filter-pill-wide">
            <Select
              v-model="filters.apiKeyGroup"
              :options="apiKeyGroupFilterOptions"
              searchable
              :search-placeholder="t('admin.users.searchApiKeyGroups')"
              @change="applyFilter"
            />
          </div>
          <template v-for="(value, attrId) in activeAttributeFilters" :key="attrId">
            <div v-if="visibleFilters.has(`attr_${attrId}`)" class="usr-filter-pill">
              <input
                v-if="['text', 'textarea', 'email', 'url', 'date'].includes(getAttributeDefinition(Number(attrId))?.type || 'text')"
                :value="value"
                :placeholder="getAttributeDefinitionName(Number(attrId))"
                class="input w-full"
                @input="(e) => updateAttributeFilter(Number(attrId), (e.target as HTMLInputElement).value)"
                @keyup.enter="applyFilter"
              />
              <input
                v-else-if="getAttributeDefinition(Number(attrId))?.type === 'number'"
                :value="value"
                type="number"
                :placeholder="getAttributeDefinitionName(Number(attrId))"
                class="input w-full"
                @input="(e) => updateAttributeFilter(Number(attrId), (e.target as HTMLInputElement).value)"
                @keyup.enter="applyFilter"
              />
              <Select
                v-else-if="['select', 'multi_select'].includes(getAttributeDefinition(Number(attrId))?.type || '')"
                :model-value="value"
                :options="[
                  { value: '', label: getAttributeDefinitionName(Number(attrId)) },
                  ...(getAttributeDefinition(Number(attrId))?.options || [])
                ]"
                @update:model-value="(val) => { updateAttributeFilter(Number(attrId), String(val ?? '')); applyFilter() }"
              />
              <input
                v-else
                :value="value"
                :placeholder="getAttributeDefinitionName(Number(attrId))"
                class="input w-full"
                @input="(e) => updateAttributeFilter(Number(attrId), (e.target as HTMLInputElement).value)"
                @keyup.enter="applyFilter"
              />
            </div>
          </template>

          <span class="usr-selection-count">
            {{ t('admin.users.selectedOfTotal') }}
            <b>{{ selectedCount }}</b> / {{ pagination.total }}
          </span>

          <div class="usr-menu" ref="columnDropdownRef">
            <button
              type="button"
              class="usr-icon-pill"
              :title="t('admin.users.columnSettings')"
              :aria-expanded="showColumnDropdown"
              @click="showColumnDropdown = !showColumnDropdown"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6" /></svg>
            </button>
            <div v-if="showColumnDropdown" class="dropdown usr-dropdown usr-columns-dropdown">
              <div class="dropdown-label">{{ t('admin.users.columnSettings') }}</div>
              <button
                v-for="col in toggleableColumns"
                :key="col.key"
                type="button"
                class="dropdown-item"
                :class="{ 'is-active': isColumnVisible(col.key) }"
                :disabled="isForcedVisibleColumn(col.key)"
                :title="isForcedVisibleColumn(col.key) ? t('admin.users.columnAlwaysVisible') : ''"
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
          :columns="cols"
          :data="sortedUsers"
          :loading="loading"
          row-key="id"
          selectable
          :selected-keys="selectedIds"
          :selection-label="getUserSelectionLabel"
          :actions-count="7"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          :sort-storage-key="USER_SORT_STORAGE_KEY"
          @sort="handleSort"
          @update:selected-keys="handleSelectedKeysUpdate"
        >
          <template #cell-email="{ value, row }">
            <div class="flex items-center gap-2">
              <div
                class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-full bg-accent/15"
              >
                <img
                  v-if="sanitizeAvatarUrl(row.avatar_url)"
                  :src="sanitizeAvatarUrl(row.avatar_url)"
                  :alt="value"
                  class="h-full w-full object-cover"
                >
                <span v-else class="text-sm font-medium text-accent">
                  {{ value.charAt(0).toUpperCase() }}
                </span>
              </div>
              <button
                type="button"
                class="font-medium text-foreground underline decoration-dashed decoration-line underline-offset-4 transition-colors hover:text-accent "
                :title="t('admin.users.viewUserDashboard')"
                @click.stop="handleUserDashboardJump(row)"
              >
                {{ value }}
              </button>
            </div>
          </template>

          <template #cell-id="{ value }">
            <MonoCell :value="value" />
          </template>

          <template #cell-username="{ value }">
            <span class="text-sm text-foreground">{{ value || '-' }}</span>
          </template>

          <template #cell-notes="{ value }">
            <div class="max-w-xs">
              <span
                v-if="value"
                :title="value.length > 30 ? value : undefined"
                class="block truncate text-sm text-muted"
              >
                {{ value.length > 30 ? value.substring(0, 25) + '...' : value }}
              </span>
              <span v-else class="text-sm text-muted">-</span>
            </div>
          </template>

          <!-- Dynamic attribute columns -->
          <template
            v-for="def in attributeDefinitions.filter(d => d.enabled)"
            :key="def.id"
            #[`cell-attr_${def.id}`]="{ row }"
          >
            <div class="max-w-xs">
              <span
                class="block truncate text-sm text-foreground"
                :title="getAttributeValue(row.id, def.id)"
              >
                {{ getAttributeValue(row.id, def.id) }}
              </span>
            </div>
          </template>

          <template #cell-role="{ value }">
            <span :class="['badge', value === 'admin' ? 'badge-purple' : 'badge-gray']">
              {{ t('admin.users.roles.' + value) }}
            </span>
          </template>

          <template #cell-groups="{ row }">
            <div v-if="allGroups.length > 0" class="flex flex-col gap-1">
              <!-- 专属分组行 -->
              <span
                v-if="getUserGroups(row).exclusive.length > 0"
                class="group/ex relative inline-flex cursor-pointer items-center gap-1 whitespace-nowrap text-xs"
                @click.stop="toggleExpandedGroup(row.id)"
              >
                <Icon name="shield" size="xs" class="h-3.5 w-3.5 text-purple-500 " />
                <span class="font-medium text-purple-600 ">{{ getUserGroups(row).exclusive.length }}</span>
                <span class="text-muted">{{ t('admin.users.exclusiveLabel') }}</span>
                <!-- Hover tooltip（操作菜单未打开时显示） -->
                <div
                  v-if="expandedGroupUserId !== row.id"
                  class="pointer-events-none absolute left-0 top-full z-50 mt-1.5 rounded bg-[var(--code-bg)] px-2.5 py-1.5 text-xs text-white opacity-0 shadow-[var(--shadow-pop)] transition-opacity duration-75 group-hover/ex:opacity-100 "
                >
                  <div class="absolute left-4 bottom-full border-4 border-transparent border-b-[var(--code-bg)] "></div>
                  <div class="flex flex-col gap-0.5 whitespace-nowrap">
                    <span v-for="g in getUserGroups(row).exclusive" :key="g.id">{{ g.name }}</span>
                  </div>
                </div>
                <!-- 点击展开分组操作菜单 -->
                <div
                  v-if="expandedGroupUserId === row.id"
                  class="absolute left-0 top-full z-50 mt-1.5 min-w-[160px] overflow-hidden rounded-lg border border-line bg-surface py-1 text-xs shadow-[var(--shadow-pop)]"
                >
                  <div class="border-b border-line px-3 py-1.5 text-[10px] font-medium uppercase tracking-wider text-muted ">
                    {{ t('admin.users.clickToReplace') }}
                  </div>
                  <div
                    v-for="g in getUserGroups(row).exclusive"
                    :key="g.id"
                    class="flex cursor-pointer items-center gap-2 px-3 py-2 text-foreground transition-colors hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] hover:text-accent   "
                    @click.stop="openGroupReplace(row, g)"
                  >
                    <Icon name="swap" size="xs" class="h-3.5 w-3.5 flex-shrink-0 opacity-50" />
                    <span class="flex-1">{{ g.name }}</span>
                  </div>
                </div>
              </span>
              <!-- 公开分组行 -->
              <span
                v-if="getUserGroups(row).publicGroups.length > 0"
                class="group/pub relative inline-flex cursor-default items-center gap-1 whitespace-nowrap text-xs"
              >
                <Icon name="globe" size="xs" class="h-3.5 w-3.5 text-muted" />
                <span class="font-medium text-muted ">{{ getUserGroups(row).publicGroups.length }}</span>
                <span class="text-muted">{{ t('admin.users.publicLabel') }}</span>
                <!-- Tooltip: 向下弹出 -->
                <div class="pointer-events-none absolute left-0 top-full z-50 mt-1.5 rounded bg-[var(--code-bg)] px-2.5 py-1.5 text-xs text-white opacity-0 shadow-[var(--shadow-pop)] transition-opacity duration-75 group-hover/pub:opacity-100 ">
                  <div class="absolute left-4 bottom-full border-4 border-transparent border-b-[var(--code-bg)] "></div>
                  <div class="flex flex-col gap-0.5 whitespace-nowrap">
                    <span v-for="g in getUserGroups(row).publicGroups" :key="g.id">{{ g.name }}</span>
                  </div>
                </div>
              </span>
              <!-- 都没有 -->
              <span
                v-if="getUserGroups(row).exclusive.length === 0 && getUserGroups(row).publicGroups.length === 0"
                class="text-xs text-muted"
              >-</span>
            </div>
            <span v-else class="text-xs text-muted">-</span>
          </template>

          <template #cell-subscriptions="{ row }">
            <div
              v-if="row.subscriptions && row.subscriptions.length > 0"
              class="flex flex-wrap gap-1.5"
            >
              <GroupBadge
                v-for="sub in row.subscriptions"
                :key="sub.id"
                :name="sub.group?.name || ''"
                :platform="sub.group?.platform"
                :subscription-type="sub.group?.subscription_type"
                :rate-multiplier="sub.group?.rate_multiplier"
                :days-remaining="sub.expires_at ? getDaysRemaining(sub.expires_at) : null"
                :title="sub.expires_at ? formatDateTime(sub.expires_at) : ''"
              />
            </div>
            <span
              v-else
              class="inline-flex items-center gap-1.5 rounded-md bg-surface-2 px-2 py-1 text-xs text-muted"
            >
              <Icon name="ban" size="xs" class="h-3.5 w-3.5" />
              <span>{{ t('admin.users.noSubscription') }}</span>
            </span>
          </template>

          <template #cell-balance="{ value, row }">
            <div class="flex items-center gap-2">
              <div class="group relative">
                <button
                  class="font-medium text-foreground underline decoration-dashed decoration-line underline-offset-4 transition-colors hover:text-accent "
                  @click="handleBalanceHistory(row)"
                >
                  ${{ value.toFixed(2) }}
                </button>
                <!-- Instant tooltip -->
                <div class="pointer-events-none absolute bottom-full left-1/2 z-50 mb-1.5 -translate-x-1/2 whitespace-nowrap rounded bg-[var(--code-bg)] px-2 py-1 text-xs text-white opacity-0 shadow-[var(--shadow-pop)] transition-opacity duration-75 group-hover:opacity-100 ">
                  {{ t('admin.users.balanceHistoryTip') }}
                  <div class="absolute left-1/2 top-full -translate-x-1/2 border-4 border-transparent border-t-[var(--code-bg)] "></div>
                </div>
              </div>
              <button
                @click.stop="handleDeposit(row)"
                class="rounded px-2 py-0.5 text-xs font-medium text-success-text transition-colors hover:bg-[color-mix(in_oklch,var(--success)_16%,transparent)]  "
                :title="t('admin.users.deposit')"
              >
                {{ t('admin.users.deposit') }}
              </button>
            </div>
          </template>

          <template #cell-balance_platform_quota="{ row }">
            <button
              type="button"
              class="block text-left underline decoration-dashed decoration-line underline-offset-4 transition-colors hover:decoration-accent "
              :title="t('admin.users.platformQuota.cellColumnTooltip')"
              @click="handlePlatformQuota(row)"
            >
              <UserPlatformQuotaCell :quotas="platformQuotaStats[row.id]" />
            </button>
          </template>

          <!-- 用量列自定义表头：列名 + 单个排序图标按钮，点击展开"今日/近30天"菜单。
               column.sortable=false，DataTable 内置点击逻辑不会触发；
               菜单项三态循环：desc → asc → off。 -->
          <template
            v-for="usageKey in USAGE_COLUMN_KEYS"
            :key="usageKey"
            #[`header-${usageKey}`]="{ column }"
          >
            <div class="flex items-center gap-1.5">
              <span>{{ column.label }}</span>
              <div class="usage-sort-trigger relative">
                <button
                  type="button"
                  class="flex items-center gap-1 rounded px-1 py-0.5 transition-colors hover:bg-surface-3"
                  :class="usageSort && usageSort.key === usageKey
 ? 'text-accent'
 : 'text-muted '"
                  :title="t('admin.users.sortBy')"
                  :data-test="`usage-sort-trigger-${usageKey}`"
                  @click.stop="toggleUsageSortMenu(usageKey)"
                >
                  <span
                    v-if="usageSort && usageSort.key === usageKey"
                    class="text-[10px] normal-case font-medium tracking-normal"
                  >{{ usageSort.metric === 'today' ? t('admin.users.today') : t('admin.users.total') }}</span>
                  <svg
                    v-if="usageSort && usageSort.key === usageKey"
                    class="h-3.5 w-3.5"
                    :class="{ 'rotate-180': usageSort.order === 'desc' }"
                    fill="currentColor"
                    viewBox="0 0 20 20"
                  >
                    <path
                      fill-rule="evenodd"
                      d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z"
                      clip-rule="evenodd"
                    />
                  </svg>
                  <svg v-else class="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 20 20">
                    <path d="M10 3l-4 5h8l-4-5zM10 17l4-5H6l4 5z" />
                  </svg>
                </button>
                <!-- 弹出菜单：今日 / 近30天，点击进行三态循环切换。 -->
                <div
                  v-if="openUsageSortMenu === usageKey"
                  class="absolute right-0 top-full z-50 mt-1 min-w-[120px] rounded-lg border border-line bg-surface py-1 shadow-[var(--shadow-pop)]"
                >
                  <button
                    v-for="metric in (['today', 'total'] as const)"
                    :key="metric"
                    type="button"
                    class="flex w-full items-center justify-between gap-3 px-3 py-1.5 text-left text-xs normal-case tracking-normal hover:bg-surface-2"
                    :class="isUsageSortActive(usageKey, metric)
 ? 'font-medium text-accent'
 : 'text-foreground'"
                    :data-test="`usage-sort-${usageKey}-${metric}`"
                    @click.stop="toggleUsageSort(usageKey, metric)"
                  >
                    <span>{{ metric === 'today' ? t('admin.users.today') : t('admin.users.total') }}</span>
                    <svg
                      v-if="getUsageSortOrder(usageKey, metric)"
                      class="h-3 w-3"
                      :class="{ 'rotate-180': getUsageSortOrder(usageKey, metric) === 'desc' }"
                      fill="currentColor"
                      viewBox="0 0 20 20"
                    >
                      <path
                        fill-rule="evenodd"
                        d="M14.707 12.707a1 1 0 01-1.414 0L10 9.414l-3.293 3.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 010 1.414z"
                        clip-rule="evenodd"
                      />
                    </svg>
                  </button>
                  <div class="mt-1 border-t border-line px-3 py-1 text-[10px] normal-case tracking-normal text-muted">
                    {{ usageKey === 'usage' ? t('admin.users.sortLast30dServer') : t('admin.users.sortCurrentPageOnly') }}
                  </div>
                </div>
              </div>
            </div>
          </template>

          <template #cell-usage="{ row }">
            <div class="space-y-0.5 text-sm">
              <div class="flex items-center gap-1.5">
                <span class="text-muted">{{ t('admin.users.todayBalance') }}:</span>
                <span class="font-medium tabular-nums text-foreground">
                  ${{ (row.today_balance_actual_cost ?? usageStats[row.id]?.today_actual_cost ?? 0).toFixed(4) }}
                </span>
              </div>
              <div class="flex items-center gap-1.5">
                <span class="text-muted">{{ t('admin.users.todaySubscription') }}:</span>
                <span class="font-medium tabular-nums text-foreground">
                  ${{ (row.today_subscription_actual_cost ?? 0).toFixed(4) }}
                </span>
              </div>
              <div class="flex items-center gap-1.5">
                <span class="text-muted">{{ t('admin.users.total') }}:</span>
                <span class="font-medium tabular-nums text-foreground">
                  ${{ (row.total_actual_cost ?? usageStats[row.id]?.total_actual_cost ?? 0).toFixed(4) }}
                </span>
              </div>
            </div>
          </template>

          <template #cell-usage_anthropic="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'anthropic')" />
          </template>

          <template #cell-usage_openai="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'openai')" />
          </template>

          <template #cell-usage_gemini="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'gemini')" />
          </template>

          <template #cell-usage_antigravity="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'antigravity')" />
          </template>

          <template #cell-concurrency="{ row }">
            <UserConcurrencyCell
              :current="row.current_concurrency ?? 0"
              :max="row.concurrency"
              :rpm-used="row.current_rpm ?? 0"
              :rpm-max="row.rpm_limit ?? 0"
            />
          </template>

          <template #cell-status="{ value }">
            <StatusCell
              :status="value"
              :label="value === 'active' ? t('common.active') : t('admin.users.disabled')"
            />
          </template>

          <template #cell-created_at="{ value }">
            <TimeCell :value="value" mode="absolute" />
          </template>

          <template #cell-last_used_at="{ value }">
            <TimeCell :value="value" mode="relative" />
          </template>

          <template #cell-last_active_at="{ value }">
            <TimeCell :value="value" mode="relative" />
          </template>

          <template #cell-actions="{ row }">
            <ActionsCell
              :edit-label="t('common.edit')"
              :more-label="t('common.more')"
              :items="getUserActionItems(row)"
              @edit="handleEdit(row)"
            >
              <template v-if="row.role !== 'admin'" #extra>
                <button
                  type="button"
                  class="icon-btn"
                  :class="{ 'icon-btn-danger': row.status === 'active' }"
                  :title="row.status === 'active' ? t('admin.users.disable') : t('admin.users.enable')"
                  :aria-label="row.status === 'active' ? t('admin.users.disable') : t('admin.users.enable')"
                  @click.stop="handleToggleStatus(row)"
                >
                  <Icon v-if="row.status === 'active'" name="ban" size="sm" :stroke-width="1.8" />
                  <Icon v-else name="checkCircle" size="sm" :stroke-width="1.8" />
                </button>
              </template>
            </ActionsCell>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.users.noUsersYet')"
              :description="t('admin.users.createFirstUser')"
              :action-text="t('admin.users.createUser')"
              @action="showCreateModal = true"
            />
          </template>
        </DataTable>
      </template>

      <!-- Pagination -->
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

    <Fab class="users-fab" :label="t('admin.users.createUser')" @click="showCreateModal = true">
      <Icon name="plus" size="md" />
      {{ t('admin.users.createUser') }}
    </Fab>
    <ConfirmDialog :show="showDeleteDialog" :title="t('admin.users.deleteUser')" :message="t('admin.users.deleteConfirm', { email: deletingUser?.email })" :danger="true" @confirm="confirmDelete" @cancel="showDeleteDialog = false" />
    <UserCreateModal :show="showCreateModal" @close="showCreateModal = false" @success="loadUsers" />
    <UserEditModal :show="showEditModal" :user="editingUser" @close="closeEditModal" @success="loadUsers" />
    <BulkEditUserModal
      :show="showBulkEditModal"
      :selected-ids="selectedIds"
      @close="showBulkEditModal = false"
      @success="handleBulkLimitsSuccess"
    />
    <UserPlatformQuotaModal
      :show="showPlatformQuotaModal"
      :user="platformQuotaUser"
      @close="closePlatformQuotaModal"
      @success="loadUsers"
    />
    <UserApiKeysModal :show="showApiKeysModal" :user="viewingUser" @close="closeApiKeysModal" />
    <UserAllowedGroupsModal :show="showAllowedGroupsModal" :user="allowedGroupsUser" @close="closeAllowedGroupsModal" @success="loadUsers" />
    <UserBalanceModal :show="showBalanceModal" :user="balanceUser" :operation="balanceOperation" @close="closeBalanceModal" @success="loadUsers" />
    <UserBalanceHistoryModal :show="showBalanceHistoryModal" :user="balanceHistoryUser" @close="closeBalanceHistoryModal" @deposit="handleDepositFromHistory" @withdraw="handleWithdrawFromHistory" />
    <GroupReplaceModal :show="showGroupReplaceModal" :user="groupReplaceUser" :old-group="groupReplaceOldGroup" :all-groups="allGroups" @close="closeGroupReplaceModal" @success="loadUsers" />
    <UserAttributesConfigModal :show="showAttributesModal" @close="handleAttributesModalClose" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useTableSelection } from '@/composables/useTableSelection'
import { formatDateLocalInput, formatDateTime } from '@/utils/format'
import { sanitizeSupportQRUrl, sanitizeUrl } from '@/utils/url'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type { AdminUser, AdminGroup, UserAttributeDefinition } from '@/types'
import type { BatchUserUsageStats } from '@/api/admin/dashboard'
import type { PlatformQuotaItem } from '@/api/admin/users'
import type { Column } from '@/components/common/types'
import type { SelectOption } from '@/components/common/Select.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import Fab from '@/components/ui/Fab.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import { MonoCell, StatusCell, TimeCell, ActionsCell, type ActionsCellItem } from '@/components/common/cells'
import { buildApiKeyGroupFilterOptions } from './apiKeyGroupFilterOptions'
import UserAttributesConfigModal from '@/components/user/UserAttributesConfigModal.vue'
import UserConcurrencyCell from '@/components/user/UserConcurrencyCell.vue'
import PlatformCostCell from '@/components/user/PlatformCostCell.vue'
import UserPlatformQuotaCell from '@/components/user/UserPlatformQuotaCell.vue'
import UserCreateModal from '@/components/admin/user/UserCreateModal.vue'
import UserEditModal from '@/components/admin/user/UserEditModal.vue'
import BulkEditUserModal from '@/components/admin/user/BulkEditUserModal.vue'
import UserPlatformQuotaModal from '@/components/admin/user/UserPlatformQuotaModal.vue'
import UserApiKeysModal from '@/components/admin/user/UserApiKeysModal.vue'
import UserAllowedGroupsModal from '@/components/admin/user/UserAllowedGroupsModal.vue'
import UserBalanceModal from '@/components/admin/user/UserBalanceModal.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import GroupReplaceModal from '@/components/admin/user/GroupReplaceModal.vue'

const appStore = useAppStore()
const router = useRouter()

// Generate dynamic attribute columns from enabled definitions
const attributeColumns = computed<Column[]>(() =>
  attributeDefinitions.value
    .filter(def => def.enabled)
    .map(def => ({
      key: `attr_${def.id}`,
      label: def.name,
      sortable: false
    }))
)

// Get formatted attribute value for display in table
const getAttributeValue = (userId: number, attrId: number): string => {
  const userAttrs = userAttributeValues.value[userId]
  if (!userAttrs) return '-'
  const value = userAttrs[attrId]
  if (!value) return '-'

  // Find definition for this attribute
  const def = attributeDefinitions.value.find(d => d.id === attrId)
  if (!def) return value

  // Format based on type
  if (def.type === 'multi_select' && value) {
    try {
      const arr = JSON.parse(value)
      if (Array.isArray(arr)) {
        // Map values to labels
        return arr.map(v => {
          const opt = def.options?.find(o => o.value === v)
          return opt?.label || v
        }).join(', ')
      }
    } catch {
      return value
    }
  }

  if (def.type === 'select' && value && def.options) {
    const opt = def.options.find(o => o.value === value)
    return opt?.label || value
  }

  return value
}

// All possible columns (for column settings)
const allColumns = computed<Column[]>(() => [
  { key: 'email', label: t('admin.users.columns.user'), sortable: true },
  { key: 'id', label: t('admin.users.columns.id'), sortable: true },
  { key: 'username', label: t('admin.users.columns.username'), sortable: true },
  { key: 'notes', label: t('admin.users.columns.notes'), sortable: false },
  // Dynamic attribute columns
  ...attributeColumns.value,
  { key: 'role', label: t('admin.users.columns.role'), sortable: true },
  { key: 'groups', label: t('admin.users.columns.groups'), sortable: false },
  { key: 'subscriptions', label: t('admin.users.columns.subscriptions'), sortable: false },
  { key: 'balance', label: t('admin.users.columns.balance'), sortable: true },
  { key: 'balance_platform_quota', label: t('admin.users.columns.balancePlatformQuota'), sortable: false },
  { key: 'usage', label: t('admin.users.columns.usage'), sortable: false },
  { key: 'usage_anthropic', label: t('admin.users.columns.usageAnthropic'), sortable: false },
  { key: 'usage_openai', label: t('admin.users.columns.usageOpenAI'), sortable: false },
  { key: 'usage_gemini', label: t('admin.users.columns.usageGemini'), sortable: false },
  { key: 'usage_antigravity', label: t('admin.users.columns.usageAntigravity'), sortable: false },
  { key: 'concurrency', label: t('admin.users.columns.concurrency'), sortable: true },
  { key: 'status', label: t('admin.users.columns.status'), sortable: true },
  { key: 'last_active_at', label: t('admin.users.columns.lastActive'), sortable: true },
  { key: 'last_used_at', label: t('admin.users.columns.lastUsed'), sortable: true },
  { key: 'created_at', label: t('admin.users.columns.created'), sortable: true },
  { key: 'actions', label: t('admin.users.columns.actions'), sortable: false }
])

// Columns that can be toggled (exclude email and actions which are always visible)
const toggleableColumns = computed(() =>
  allColumns.value.filter(col => col.key !== 'email' && col.key !== 'actions')
)

// Hidden columns (stored in Set - columns NOT in this set are visible)
// This way, new columns are visible by default
const hiddenColumns = reactive<Set<string>>(new Set())

// Default hidden columns (columns hidden by default on first load)
const DEFAULT_HIDDEN_COLUMNS = [
  'notes', 'groups', 'subscriptions', 'usage', 'concurrency',
  'usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity',
  'balance_platform_quota'
]
const REMOVED_COLUMNS = new Set(['last_login_at'])
// 强制可见列：加载时会被强制移出 hiddenColumns，并在列设置 UI 上 disabled。
// 当前没有列需要强制可见 —— last_active_at 已改为可被用户隐藏。
const FORCED_VISIBLE_COLUMNS = new Set<string>(['concurrency'])

// localStorage keys for column settings
const HIDDEN_COLUMNS_KEY = 'user-hidden-columns'
// 列设置 schema 版本号。每次给 DEFAULT_HIDDEN_COLUMNS 新增列时 bump 一次，
// 并在 VERSION_NEW_HIDDEN_COLUMNS 中登记该版本新增的 key。
// 这样老用户升级后这些新列会被自动隐藏一次，而不会影响他们对其它老列的偏好。
const COLUMN_SETTINGS_VERSION_KEY = 'user-column-settings-version'
const COLUMN_SETTINGS_VERSION = 3
const VERSION_NEW_HIDDEN_COLUMNS: Record<number, string[]> = {
  2: ['usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity'],
  3: ['balance_platform_quota']
}

// Load saved column settings
const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as string[]
      parsed
        .filter(key => !REMOVED_COLUMNS.has(key) && !FORCED_VISIBLE_COLUMNS.has(key))
        .forEach(key => hiddenColumns.add(key))

      // 老用户升级：把每个未应用过的版本里新增的默认隐藏列自动追加到 hiddenColumns。
      const storedVersion = Number(localStorage.getItem(COLUMN_SETTINGS_VERSION_KEY) ?? '1')
      if (storedVersion < COLUMN_SETTINGS_VERSION) {
        let mutated = false
        for (let v = storedVersion + 1; v <= COLUMN_SETTINGS_VERSION; v++) {
          for (const key of VERSION_NEW_HIDDEN_COLUMNS[v] ?? []) {
            if (REMOVED_COLUMNS.has(key) || FORCED_VISIBLE_COLUMNS.has(key)) continue
            if (!hiddenColumns.has(key)) {
              hiddenColumns.add(key)
              mutated = true
            }
          }
        }
        if (mutated) saveColumnsToStorage()
        else localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
      }
    } else {
      // Use default hidden columns on first load
      DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
      localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
    }
  } catch (e) {
    console.error('Failed to load saved columns:', e)
    DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
  }
}

// Save column settings to localStorage
const saveColumnsToStorage = () => {
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
    localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}

// Toggle column visibility
const isForcedVisibleColumn = (key: string) => FORCED_VISIBLE_COLUMNS.has(key)
const toggleColumn = (key: string) => {
  // 强制可见列(如 last_active_at)在加载时会被恢复成可见，
  // 这里阻止用户在当前会话隐藏它，避免"取消勾选 → 刷新又恢复"的反直觉行为。
  if (FORCED_VISIBLE_COLUMNS.has(key)) return
  const wasHidden = hiddenColumns.has(key)
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  saveColumnsToStorage()
  if (wasHidden && (key === 'usage' || key.startsWith('usage_') || key.startsWith('attr_') || key === 'balance_platform_quota')) {
    refreshCurrentPageSecondaryData()
  }
  if (key === 'subscriptions') {
    loadUsers()
  }
  if (wasHidden && key === 'groups') {
    loadAllGroups()
  }
}

// Check if column is visible (not in hidden set)
const isColumnVisible = (key: string) => !hiddenColumns.has(key)
// usage 主列或任意 usage_<platform> 子列可见时都需要批量拉取用量数据
// 列 key → 平台名（'usage' 主列汇总所有平台时为 null）
// 显式数组取代 Object.keys()：保证迭代顺序（决定列头排序按钮渲染顺序）
// 不会因 JS 引擎差异或 USAGE_COLUMN_PLATFORMS 属性顺序调整而静默变化。
const USAGE_COLUMN_KEYS: readonly string[] = ['usage', 'usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity']
const USAGE_COLUMN_PLATFORMS: Record<string, string | null> = {
  usage: null,
  usage_anthropic: 'anthropic',
  usage_openai: 'openai',
  usage_gemini: 'gemini',
  usage_antigravity: 'antigravity'
}
const PLATFORM_USAGE_COLUMNS = USAGE_COLUMN_KEYS.filter((k) => k !== 'usage')
const hasVisibleUsageColumn = computed(
  () => !hiddenColumns.has('usage') || PLATFORM_USAGE_COLUMNS.some((k) => !hiddenColumns.has(k))
)
const hasVisibleGroupsColumn = computed(() => !hiddenColumns.has('groups'))
const hasVisiblePlatformQuotaColumn = computed(() => !hiddenColumns.has('balance_platform_quota'))
const hasVisibleAttributeColumns = computed(() =>
  attributeDefinitions.value.some((def) => def.enabled && !hiddenColumns.has(`attr_${def.id}`))
)

// Filtered columns based on visibility
const columns = computed<Column[]>(() =>
  allColumns.value.filter(col =>
    col.key === 'email' || col.key === 'actions' || !hiddenColumns.has(col.key)
  )
)

// ListPage 配方：固定列宽通过 `usr-col-<key>` class 挂到 th/td 上，宽度在 <style scoped> 里用 :deep() 指定。
const cols = computed<Column[]>(() =>
  columns.value.map((col) => ({ ...col, class: `usr-col-${col.key}` }))
)

const users = ref<AdminUser[]>([])
const loading = ref(false)
const searchQuery = ref('')
const USER_SORT_STORAGE_KEY = 'admin-users-table-sort'
const loadInitialSortState = (): { sort_by: string; sort_order: 'asc' | 'desc' } => {
  const fallback = { sort_by: 'created_at', sort_order: 'desc' as 'asc' | 'desc' }
  const sortable = new Set(['email', 'id', 'username', 'role', 'balance', 'concurrency', 'current_concurrency', 'available_concurrency', 'status', 'last_used_at', 'last_active_at', 'created_at', 'last_30d_usage'])
  try {
    const raw = localStorage.getItem(USER_SORT_STORAGE_KEY)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    const key = typeof parsed.key === 'string' ? parsed.key : ''
    if (!sortable.has(key)) return fallback
    return {
      sort_by: key,
      sort_order: parsed.order === 'asc' ? 'asc' : 'desc'
    }
  } catch {
    return fallback
  }
}
const sortState = reactive(loadInitialSortState())

// Groups data for the groups column and the existing "authorised group" filter (active only)
const allGroups = ref<AdminGroup[]>([])
const loadAllGroups = async () => {
  if (allGroups.value.length > 0) return
  try {
    allGroups.value = await adminAPI.groups.getAll()
  } catch (e) {
    console.error('Failed to load groups:', e)
  }
}

// Groups for the API Key group filter — includes disabled groups so admins can
// filter users whose keys are still bound to a now-disabled group.
const allGroupsForApiKeyFilter = ref<AdminGroup[]>([])
const loadAllGroupsForApiKeyFilter = async () => {
  if (allGroupsForApiKeyFilter.value.length > 0) return
  try {
    allGroupsForApiKeyFilter.value = await adminAPI.groups.getAllIncludingInactive()
  } catch (e) {
    console.error('Failed to load groups for API key filter:', e)
  }
}
// Resolve user's accessible groups: exclusive groups first, then public groups
const getUserGroups = (user: AdminUser) => {
  const exclusive: AdminGroup[] = []
  const publicGroups: AdminGroup[] = []
  for (const g of allGroups.value) {
    if (g.status !== 'active' || g.subscription_type !== 'standard') continue
    if (g.is_exclusive) {
      if (user.allowed_groups?.includes(g.id)) {
        exclusive.push(g)
      }
    } else {
      publicGroups.push(g)
    }
  }
  return { exclusive, publicGroups }
}

// Group filter options: "All Groups" + active exclusive groups (value = group name for fuzzy match)
const groupFilterOptions = computed(() => {
  const options: { value: string; label: string }[] = [
    { value: '', label: t('admin.users.allAuthorizedGroups') }
  ]
  for (const g of allGroups.value) {
    if (g.status !== 'active' || !g.is_exclusive || g.subscription_type !== 'standard') continue
    options.push({ value: g.name, label: g.name })
  }
  return options
})

// API Key group filter options: "All" + groups partitioned by type (value = group id).
// Uses allGroupsForApiKeyFilter which includes disabled groups.
const apiKeyGroupFilterOptions = computed(() =>
  buildApiKeyGroupFilterOptions(allGroupsForApiKeyFilter.value, {
    all: t('admin.users.allApiKeyGroups'),
    exclusive: t('admin.users.apiKeyGroupExclusive'),
    public: t('admin.users.apiKeyGroupPublic'),
    subscription: t('admin.users.apiKeyGroupSubscription'),
    disabled: t('admin.users.apiKeyGroupDisabled'),
  }) as SelectOption[]
)

// Filter values (role, status, and custom attributes)
const filters = reactive({
  role: '',
  status: '',
  group: '',  // group name for fuzzy match, '' = all
  apiKeyGroup: null as number | null  // group id bound to the user's API keys, null = all
})
const activeAttributeFilters = reactive<Record<number, string>>({})

// Visible filters tracking (which filters are shown in the UI)
// Keys: 'role', 'status', 'attr_${id}'
const visibleFilters = reactive<Set<string>>(new Set())

// Dropdown states
const showFilterDropdown = ref(false)
const showColumnDropdown = ref(false)

// Dropdown refs for click outside detection
const filterDropdownRef = ref<HTMLElement | null>(null)
const columnDropdownRef = ref<HTMLElement | null>(null)

// localStorage keys
const FILTER_VALUES_KEY = 'user-filter-values'
const VISIBLE_FILTERS_KEY = 'user-visible-filters'

// All filterable attribute definitions (enabled attributes)
const filterableAttributes = computed(() =>
  attributeDefinitions.value.filter(def => def.enabled)
)

// Built-in filter definitions
const builtInFilters = computed(() => [
  { key: 'role', name: t('admin.users.columns.role'), type: 'select' as const },
  { key: 'status', name: t('admin.users.columns.status'), type: 'select' as const },
  { key: 'group', name: t('admin.users.authorizedGroupFilter'), type: 'select' as const },
  { key: 'apiKeyGroup', name: t('admin.users.apiKeyGroupFilter'), type: 'select' as const }
])

// Load saved filters from localStorage
const loadSavedFilters = () => {
  try {
    // Load visible filters
    const savedVisible = localStorage.getItem(VISIBLE_FILTERS_KEY)
    if (savedVisible) {
      const parsed = JSON.parse(savedVisible) as string[]
      parsed.forEach(key => visibleFilters.add(key))
    }
    // Load filter values
    const savedValues = localStorage.getItem(FILTER_VALUES_KEY)
    if (savedValues) {
      const parsed = JSON.parse(savedValues)
      if (parsed.role) filters.role = parsed.role
      if (parsed.status) filters.status = parsed.status
      if (parsed.group) filters.group = parsed.group
      if (typeof parsed.apiKeyGroup === 'number') filters.apiKeyGroup = parsed.apiKeyGroup
      if (parsed.attributes) {
        Object.assign(activeAttributeFilters, parsed.attributes)
      }
    }
  } catch (e) {
    console.error('Failed to load saved filters:', e)
  }
}

// Save filters to localStorage
const saveFiltersToStorage = () => {
  try {
    // Save visible filters
    localStorage.setItem(VISIBLE_FILTERS_KEY, JSON.stringify([...visibleFilters]))
    // Save filter values
    const values = {
      role: filters.role,
      status: filters.status,
      group: filters.group,
      apiKeyGroup: filters.apiKeyGroup,
      attributes: activeAttributeFilters
    }
    localStorage.setItem(FILTER_VALUES_KEY, JSON.stringify(values))
  } catch (e) {
    console.error('Failed to save filters:', e)
  }
}

// Get attribute definition by ID
const getAttributeDefinition = (attrId: number): UserAttributeDefinition | undefined => {
  return attributeDefinitions.value.find(d => d.id === attrId)
}
const usageStats = ref<Record<string, BatchUserUsageStats>>({})
const platformQuotaStats = ref<Record<number, PlatformQuotaItem[]>>({})

const getPlatformUsage = (userId: number, platform: string) =>
  usageStats.value[userId]?.by_platform?.find((p) => p.platform === platform)

// 用量列前端排序：DataTable 工作在 server-side-sort 模式，所有 sortable
// 字段都会触发后端查询，而用量列数据是异步批量拉取后再合并到当前页，
// 因此采用独立的前端排序状态对当前页 users 做本地排序。
// 排序状态独立于后端 sortState 持久化；缺失数据按 0 处理（desc 沉底、asc 置顶）。
type UsageMetric = 'today' | 'total'
type UsageSortState = { key: string; metric: UsageMetric; order: 'asc' | 'desc' } | null
const USAGE_SORT_STORAGE_KEY = 'admin-users-usage-sort'
// 列头排序按钮点击后弹出的"今日/近30天"选择菜单，同时只允许一个列展开。
const openUsageSortMenu = ref<string | null>(null)

const loadInitialUsageSort = (): UsageSortState => {
  try {
    const raw = localStorage.getItem(USAGE_SORT_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<{ key: string; metric: string; order: string }>
    if (!parsed.key || !USAGE_COLUMN_KEYS.includes(parsed.key)) return null
    const metric: UsageMetric = parsed.metric === 'total' ? 'total' : 'today'
    const order: 'asc' | 'desc' = parsed.order === 'asc' ? 'asc' : 'desc'
    return { key: parsed.key, metric, order }
  } catch {
    return null
  }
}
const usageSort = ref<UsageSortState>(loadInitialUsageSort())
const persistUsageSort = () => {
  try {
    if (usageSort.value) {
      localStorage.setItem(USAGE_SORT_STORAGE_KEY, JSON.stringify(usageSort.value))
    } else {
      localStorage.removeItem(USAGE_SORT_STORAGE_KEY)
    }
  } catch (e) {
    console.error('Failed to persist usage sort:', e)
  }
}
const clearUsageSort = () => {
  if (!usageSort.value) return
  usageSort.value = null
  openUsageSortMenu.value = null
  persistUsageSort()
}

const isUsageSortActive = (key: string, metric: UsageMetric) =>
  !!usageSort.value && usageSort.value.key === key && usageSort.value.metric === metric
const getUsageSortOrder = (key: string, metric: UsageMetric): 'asc' | 'desc' | null =>
  isUsageSortActive(key, metric) ? usageSort.value!.order : null

// 三态循环：desc → asc → off。选完即关闭菜单（用户大多希望"选中即应用"，
// 想再切换 order 时重新打开菜单点同一项即可）。
const isServerLast30dSort = computed(() =>
  usageSort.value?.key === 'usage' && usageSort.value?.metric === 'total'
)

const toggleUsageSort = (key: string, metric: UsageMetric) => {
  const cur = usageSort.value
  if (cur && cur.key === key && cur.metric === metric) {
    usageSort.value = cur.order === 'desc' ? { key, metric, order: 'asc' } : null
  } else {
    usageSort.value = { key, metric, order: 'desc' }
  }
  persistUsageSort()
  openUsageSortMenu.value = null
  if (key === 'usage' && metric === 'total') {
    pagination.page = 1
    loadUsers()
  }
}

// 点击图标本身不触发排序，仅开关菜单；首次排序由用户在菜单内选择 metric 触发（默认 desc，详见 toggleUsageSort）。
const toggleUsageSortMenu = (key: string) => {
  openUsageSortMenu.value = openUsageSortMenu.value === key ? null : key
}

const getUsageValue = (userId: number, key: string, metric: UsageMetric): number => {
  const stats = usageStats.value[userId]
  if (!stats) return 0
  const platform = USAGE_COLUMN_PLATFORMS[key]
  if (platform === null) {
    return metric === 'today' ? stats.today_actual_cost ?? 0 : stats.total_actual_cost ?? 0
  }
  const p = stats.by_platform?.find((x) => x.platform === platform)
  if (!p) return 0
  return metric === 'today' ? p.today_actual_cost ?? 0 : p.total_actual_cost ?? 0
}

// 在 server-side 排序结果之上叠加用量列的本地排序；无 usageSort 时直接透传原数组。
// 稳定排序：等值按原 index 保序，避免拉取新用量数据时表行抖动。
function sanitizeAvatarUrl(url?: string | null): string {
  const raw = url?.trim() || ''
  return sanitizeSupportQRUrl(raw) || sanitizeUrl(raw, { allowRelative: true, allowDataUrl: true })
}

const sortedUsers = computed(() => {
  const s = usageSort.value
  if (!s || isServerLast30dSort.value) return users.value
  return [...users.value]
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const av = getUsageValue(a.row.id, s.key, s.metric)
      const bv = getUsageValue(b.row.id, s.key, s.metric)
      if (av !== bv) return s.order === 'asc' ? av - bv : bv - av
      return a.index - b.index
    })
    .map((x) => x.row)
})

const {
  selectedIds,
  selectedCount,
  setSelectedIds,
  clear: clearSelection
} = useTableSelection<AdminUser>({
  rows: sortedUsers,
  getId: (user) => user.id
})

const handleSelectedKeysUpdate = (keys: Array<string | number>) => {
  setSelectedIds(keys.filter((key): key is number => typeof key === 'number'))
}

const getUserSelectionLabel = (user: AdminUser) =>
  t('admin.users.bulkLimits.selectUser', { email: user.email })

// User attribute definitions and values
const attributeDefinitions = ref<UserAttributeDefinition[]>([])
const userAttributeValues = ref<Record<number, Record<number, string>>>({})
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

// ListPage 配方：三段迷你统计卡 → 一行可点击的 .summary-chip。
// 计数只统计当前页（后端未提供全量分桶接口，与旧 MiniStatCard 的口径一致，非功能回退）。
const activeCountOnPage = computed(() => users.value.filter((user) => user.status === 'active').length)
const disabledCountOnPage = computed(() => users.value.filter((user) => user.status === 'disabled').length)
const adminCountOnPage = computed(() => users.value.filter((user) => user.role === 'admin').length)

interface SummaryChip {
  key: string
  label: string
  color: string
  count: number
  active: boolean
  onClick: () => void
}

const toggleStatusFilter = (value: string) => {
  filters.status = filters.status === value ? '' : value
  applyFilter()
}

const toggleRoleFilter = (value: string) => {
  filters.role = filters.role === value ? '' : value
  applyFilter()
}

const summaryChips = computed<SummaryChip[]>(() => [
  {
    key: 'all',
    label: t('common.total'),
    color: 'var(--accent)',
    count: pagination.total,
    active: filters.status === '' && filters.role === '',
    onClick: () => { filters.status = ''; filters.role = ''; applyFilter() }
  },
  {
    key: 'active',
    label: t('common.active'),
    color: 'var(--success)',
    count: activeCountOnPage.value,
    active: filters.status === 'active',
    onClick: () => toggleStatusFilter('active')
  },
  {
    key: 'disabled',
    label: t('admin.users.disabled'),
    color: 'var(--danger)',
    count: disabledCountOnPage.value,
    active: filters.status === 'disabled',
    onClick: () => toggleStatusFilter('disabled')
  },
  {
    key: 'admin',
    label: t('admin.users.admin'),
    color: 'var(--warning)',
    count: adminCountOnPage.value,
    active: filters.role === 'admin',
    onClick: () => toggleRoleFilter('admin')
  }
])

const showCreateModal = ref(false)
const showEditModal = ref(false)
const showBulkEditModal = ref(false)
const showDeleteDialog = ref(false)
const showApiKeysModal = ref(false)
const showAttributesModal = ref(false)
const showPlatformQuotaModal = ref(false)
const editingUser = ref<AdminUser | null>(null)
const deletingUser = ref<AdminUser | null>(null)
const viewingUser = ref<AdminUser | null>(null)
const platformQuotaUser = ref<AdminUser | null>(null)

const handlePlatformQuota = (user: AdminUser) => {
  platformQuotaUser.value = user
  showPlatformQuotaModal.value = true
}

const closePlatformQuotaModal = () => {
  showPlatformQuotaModal.value = false
  platformQuotaUser.value = null
}
let abortController: AbortController | null = null
let secondaryDataSeq = 0

const loadUsersSecondaryData = async (
  userIds: number[],
  signal?: AbortSignal,
  expectedSeq?: number
) => {
  if (userIds.length === 0) return

  const tasks: Promise<void>[] = []

  if (hasVisibleUsageColumn.value) {
    tasks.push(
      (async () => {
        try {
          const usageResponse = await adminAPI.dashboard.getBatchUsersUsage(userIds)
          if (signal?.aborted) return
          if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
          usageStats.value = usageResponse.stats
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load usage stats:', e)
        }
      })()
    )
  }

  if (attributeDefinitions.value.length > 0 && hasVisibleAttributeColumns.value) {
    tasks.push(
      (async () => {
        try {
          const attrResponse = await adminAPI.userAttributes.getBatchUserAttributes(userIds)
          if (signal?.aborted) return
          if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
          userAttributeValues.value = attrResponse.attributes
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load user attribute values:', e)
        }
      })()
    )
  }

  if (hasVisiblePlatformQuotaColumn.value) {
    tasks.push(
      (async () => {
        try {
          // 无批量端点：对当前页用户逐个拉取，分块并发（每批 6），批间检查中止条件，避免大 pageSize 时请求洪峰
          const CHUNK = 6
          for (let i = 0; i < userIds.length; i += CHUNK) {
            if (signal?.aborted) return
            if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
            const chunk = userIds.slice(i, i + CHUNK)
            const results = await Promise.allSettled(
              chunk.map((id) => adminAPI.users.getPlatformQuotas(id))
            )
            if (signal?.aborted) return
            if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
            const merged = { ...platformQuotaStats.value }
            results.forEach((r, idx) => {
              if (r.status === 'fulfilled') {
                merged[chunk[idx]] = r.value.platform_quotas || []
              }
            })
            platformQuotaStats.value = merged
          }
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load platform quotas:', e)
        }
      })()
    )
  }

  if (tasks.length > 0) {
    await Promise.allSettled(tasks)
  }
}

const refreshCurrentPageSecondaryData = () => {
  const userIds = users.value.map((u) => u.id)
  if (userIds.length === 0) return
  const seq = ++secondaryDataSeq
  void loadUsersSecondaryData(userIds, undefined, seq)
}

// ListPage 配方：操作列改用共享的 ActionsCell（编辑图标 + `…` 溢出菜单），
// 不再手写 Teleport 悬浮菜单与定位逻辑。菜单项与旧版一一对应，零功能损失。
const getUserActionItems = (user: AdminUser): ActionsCellItem[] => {
  const items: ActionsCellItem[] = [
    { label: t('admin.users.apiKeys'), icon: 'key', onClick: () => handleViewApiKeys(user) },
    { label: t('admin.users.groups'), icon: 'users', onClick: () => handleAllowedGroups(user) },
    { label: t('admin.users.deposit'), icon: 'plus', onClick: () => handleDeposit(user) },
    { label: t('admin.users.withdraw'), icon: 'arrowDown', onClick: () => handleWithdraw(user) },
    { label: t('admin.users.platformQuota.menuItem'), icon: 'chartBar', onClick: () => handlePlatformQuota(user) },
    { label: t('admin.users.balanceHistory'), icon: 'clock', onClick: () => handleBalanceHistory(user) }
  ]
  if (user.role !== 'admin') {
    items.push({ label: t('common.delete'), icon: 'trash', danger: true, onClick: () => handleDelete(user) })
  }
  return items
}

// Header "更多" dropdown（属性配置入口）
const showMoreDropdown = ref(false)
const moreDropdownRef = ref<HTMLElement | null>(null)

// Close menu when clicking outside
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  // Close filter dropdown when clicking outside
  if (filterDropdownRef.value && !filterDropdownRef.value.contains(target)) {
    showFilterDropdown.value = false
  }
  // Close column dropdown when clicking outside
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(target)) {
    showColumnDropdown.value = false
  }
  // Close header "more" dropdown when clicking outside
  if (moreDropdownRef.value && !moreDropdownRef.value.contains(target)) {
    showMoreDropdown.value = false
  }
  // Close usage sort dropdown when clicking outside any usage-sort-trigger
  if (openUsageSortMenu.value !== null && !target.closest('.usage-sort-trigger')) {
    openUsageSortMenu.value = null
  }
  // Close expanded group dropdown when clicking outside
  if (expandedGroupUserId.value !== null) {
    expandedGroupUserId.value = null
  }
}

// Allowed groups modal state
const showAllowedGroupsModal = ref(false)
const allowedGroupsUser = ref<AdminUser | null>(null)

// Expanded group dropdown state (click to show exclusive groups list)
const expandedGroupUserId = ref<number | null>(null)
const toggleExpandedGroup = (userId: number) => {
  expandedGroupUserId.value = expandedGroupUserId.value === userId ? null : userId
}

// Group replace modal state
const showGroupReplaceModal = ref(false)
const groupReplaceUser = ref<AdminUser | null>(null)
const groupReplaceOldGroup = ref<{ id: number; name: string } | null>(null)

// Balance (Deposit/Withdraw) modal state
const showBalanceModal = ref(false)
const balanceUser = ref<AdminUser | null>(null)
const balanceOperation = ref<'add' | 'subtract'>('add')

// Balance History modal state
const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)

// 计算剩余天数
const getDaysRemaining = (expiresAt: string): number => {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diffMs = expires.getTime() - now.getTime()
  return Math.ceil(diffMs / (1000 * 60 * 60 * 24))
}

const loadAttributeDefinitions = async () => {
  try {
    attributeDefinitions.value = await adminAPI.userAttributes.listEnabledDefinitions()
  } catch (e) {
    console.error('Failed to load attribute definitions:', e)
  }
}

// Handle attributes modal close - reload definitions and users
const handleAttributesModalClose = async () => {
  showAttributesModal.value = false
  await loadAttributeDefinitions()
  loadUsers()
}

const loadUsers = async () => {
  abortController?.abort()
  const currentAbortController = new AbortController()
  abortController = currentAbortController
  const { signal } = currentAbortController
  loading.value = true
  try {
    // Build attribute filters from active filters
    const attrFilters: Record<number, string> = {}
    for (const [attrId, value] of Object.entries(activeAttributeFilters)) {
      if (value) {
        attrFilters[Number(attrId)] = value
      }
    }

    const response = await adminAPI.users.list(
      pagination.page,
      pagination.page_size,
      {
        role: filters.role as any,
        status: filters.status as any,
        search: searchQuery.value || undefined,
        group_name: filters.group || undefined,
        api_key_group_id: filters.apiKeyGroup ?? undefined,
        attributes: Object.keys(attrFilters).length > 0 ? attrFilters : undefined,
        // 始终请求 subscriptions：列隐藏时仍需用于 UserPlatformQuotaModal 的 active-subscription 警示 banner
        include_subscriptions: true,
        include_usage_stats: true,
        sort_by: isServerLast30dSort.value
          ? 'last_30d_usage'
          : (sortState.sort_by === 'concurrency' ? 'current_concurrency' : sortState.sort_by),
        sort_order: isServerLast30dSort.value ? usageSort.value!.order : sortState.sort_order
      },
      { signal }
    )
    if (signal.aborted) {
      return
    }
    users.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
    usageStats.value = {}
    userAttributeValues.value = {}
    platformQuotaStats.value = {}

    // Defer heavy secondary data so table can render first.
    if (response.items.length > 0) {
      const userIds = response.items.map((u) => u.id)
      const seq = ++secondaryDataSeq
      window.setTimeout(() => {
        if (signal.aborted || seq !== secondaryDataSeq) return
        void loadUsersSecondaryData(userIds, signal, seq)
      }, 50)
    }
  } catch (error: any) {
    const errorInfo = error as { name?: string; code?: string }
    if (errorInfo?.name === 'AbortError' || errorInfo?.name === 'CanceledError' || errorInfo?.code === 'ERR_CANCELED') {
      return
    }
    const message = error.response?.data?.detail || error.message || t('admin.users.failedToLoad')
    appStore.showError(message)
    console.error('Error loading users:', error)
  } finally {
    if (abortController === currentAbortController) {
      loading.value = false
    }
  }
}

const handleBulkLimitsSuccess = async () => {
  clearSelection()
  await loadUsers()
}

// SearchInput 组件内置 300ms 防抖，这里只需响应它 debounce 后触发的 `search` 事件。
const handleSearch = () => {
  pagination.page = 1
  loadUsers()
}

const handlePageChange = (page: number) => {
  // 确保页码在有效范围内
  const validPage = Math.max(1, Math.min(page, pagination.pages || 1))
  pagination.page = validPage
  loadUsers()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadUsers()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  clearUsageSort()
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadUsers()
}

// Filter helpers
const getAttributeDefinitionName = (attrId: number): string => {
  const def = attributeDefinitions.value.find(d => d.id === attrId)
  return def?.name || String(attrId)
}

// Toggle a built-in filter (role/status)
const toggleBuiltInFilter = (key: string) => {
  if (visibleFilters.has(key)) {
    visibleFilters.delete(key)
    if (key === 'role') filters.role = ''
    if (key === 'status') filters.status = ''
    if (key === 'group') filters.group = ''
    if (key === 'apiKeyGroup') filters.apiKeyGroup = null
  } else {
    visibleFilters.add(key)
    if (key === 'group') loadAllGroups()
    if (key === 'apiKeyGroup') loadAllGroupsForApiKeyFilter()
  }
  saveFiltersToStorage()
  pagination.page = 1
  loadUsers()
}

// Toggle a custom attribute filter
const toggleAttributeFilter = (attr: UserAttributeDefinition) => {
  const key = `attr_${attr.id}`
  if (visibleFilters.has(key)) {
    visibleFilters.delete(key)
    delete activeAttributeFilters[attr.id]
  } else {
    visibleFilters.add(key)
    activeAttributeFilters[attr.id] = ''
  }
  saveFiltersToStorage()
  pagination.page = 1
  loadUsers()
}

const updateAttributeFilter = (attrId: number, value: string) => {
  activeAttributeFilters[attrId] = value
}

// Apply filter and save to localStorage
const applyFilter = () => {
  saveFiltersToStorage()
  loadUsers()
}

const handleUserDashboardJump = (user: AdminUser) => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(user.id),
      start_date: formatDateLocalInput(start),
      end_date: formatDateLocalInput(end)
    }
  })
}

const handleEdit = (user: AdminUser) => {
  editingUser.value = user
  showEditModal.value = true
}

const closeEditModal = () => {
  showEditModal.value = false
  editingUser.value = null
}

const handleToggleStatus = async (user: AdminUser) => {
  const newStatus = user.status === 'active' ? 'disabled' : 'active'
  try {
    await adminAPI.users.toggleStatus(user.id, newStatus)
    appStore.showSuccess(
      newStatus === 'active' ? t('admin.users.userEnabled') : t('admin.users.userDisabled')
    )
    loadUsers()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToToggle'))
    console.error('Error toggling user status:', error)
  }
}

const handleViewApiKeys = (user: AdminUser) => {
  viewingUser.value = user
  showApiKeysModal.value = true
}

const closeApiKeysModal = () => {
  showApiKeysModal.value = false
  viewingUser.value = null
}

const handleAllowedGroups = (user: AdminUser) => {
  allowedGroupsUser.value = user
  showAllowedGroupsModal.value = true
}

const closeAllowedGroupsModal = () => {
  showAllowedGroupsModal.value = false
  allowedGroupsUser.value = null
}

const openGroupReplace = (user: AdminUser, group: { id: number; name: string }) => {
  expandedGroupUserId.value = null
  groupReplaceUser.value = user
  groupReplaceOldGroup.value = group
  showGroupReplaceModal.value = true
}

const closeGroupReplaceModal = () => {
  showGroupReplaceModal.value = false
  groupReplaceUser.value = null
  groupReplaceOldGroup.value = null
}

const handleDelete = (user: AdminUser) => {
  deletingUser.value = user
  showDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingUser.value) return
  try {
    await adminAPI.users.delete(deletingUser.value.id)
    appStore.showSuccess(t('common.success'))
    showDeleteDialog.value = false
    deletingUser.value = null
    loadUsers()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToDelete'))
    console.error('Error deleting user:', error)
  }
}

const handleDeposit = (user: AdminUser) => {
  balanceUser.value = user
  balanceOperation.value = 'add'
  showBalanceModal.value = true
}

const handleWithdraw = (user: AdminUser) => {
  balanceUser.value = user
  balanceOperation.value = 'subtract'
  showBalanceModal.value = true
}

const closeBalanceModal = () => {
  showBalanceModal.value = false
  balanceUser.value = null
}

const handleBalanceHistory = (user: AdminUser) => {
  balanceHistoryUser.value = user
  showBalanceHistoryModal.value = true
}

const closeBalanceHistoryModal = () => {
  showBalanceHistoryModal.value = false
  balanceHistoryUser.value = null
}

// Handle deposit from balance history modal
const handleDepositFromHistory = () => {
  if (balanceHistoryUser.value) {
    handleDeposit(balanceHistoryUser.value)
  }
}

// Handle withdraw from balance history modal
const handleWithdrawFromHistory = () => {
  if (balanceHistoryUser.value) {
    handleWithdraw(balanceHistoryUser.value)
  }
}

onMounted(async () => {
  await loadAttributeDefinitions()
  loadSavedFilters()
  loadSavedColumns()
  loadUsers()
  if (hasVisibleGroupsColumn.value || visibleFilters.has('group')) {
    loadAllGroups()
  }
  if (visibleFilters.has('apiKeyGroup')) {
    loadAllGroupsForApiKeyFilter()
  }
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  abortController?.abort()
})
</script>
<style scoped>
/* ---------- Header "更多" 下拉 ---------- */
.usr-menu {
  position: relative;
  display: inline-flex;
  flex: none;
}

.usr-dropdown {
  top: 100%;
  right: 0;
  margin-top: 6px;
  min-width: 200px;
}

.usr-columns-dropdown {
  max-height: 60vh;
  overflow-y: auto;
}

/* ---------- Summary chips ---------- */
.usr-summary {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
  margin-bottom: 14px;
}

.usr-summary .summary-chip {
  width: 100%;
  text-align: left;
}

/* ---------- Filter row ---------- */
.usr-filter-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-height: 36px;
}

.usr-filter-row :deep(.search-input) {
  width: 260px;
  flex: none;
}

.usr-filter-pill {
  width: 112px;
  flex: none;
}

.usr-filter-pill-wide {
  width: 179px;
  flex: none;
}

.usr-selection-count {
  margin-left: auto;
  font-size: 12.5px;
  color: var(--muted);
  white-space: nowrap;
}

.usr-selection-count b {
  color: var(--foreground);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.usr-icon-pill {
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

.usr-icon-pill:hover {
  color: var(--foreground);
  border-color: color-mix(in oklch, var(--foreground) 18%, transparent);
}

/* ---------- 列宽（应用于 usr-col-<key> class 挂到 th/td 上）---------- */
:deep(.usr-col-email) { width: 220px; }
:deep(.usr-col-id) { width: 68px; }
:deep(.usr-col-username) { width: 120px; }
:deep(.usr-col-notes) { width: 160px; }
:deep(.usr-col-role) { width: 90px; }
:deep(.usr-col-groups) { width: 180px; }
:deep(.usr-col-subscriptions) { width: 180px; }
:deep(.usr-col-balance) { width: 140px; }
:deep(.usr-col-balance_platform_quota) { width: 160px; }
:deep(.usr-col-usage) { width: 140px; }
:deep(.usr-col-usage_anthropic),
:deep(.usr-col-usage_openai),
:deep(.usr-col-usage_gemini),
:deep(.usr-col-usage_antigravity) { width: 120px; }
:deep(.usr-col-concurrency) { width: 90px; }
:deep(.usr-col-status) { width: 90px; }
:deep(.usr-col-last_active_at),
:deep(.usr-col-last_used_at),
:deep(.usr-col-created_at) { width: 128px; }
:deep(.usr-col-actions) { width: 96px; }

.users-fab {
  display: none;
}

@media (max-width: 767px) {
  .users-create-desktop {
    display: none;
  }
  .users-fab {
    display: inline-flex;
  }

  .usr-summary {
    grid-template-columns: repeat(2, 1fr);
    gap: 8px;
  }

  .usr-filter-row {
    gap: 8px;
  }

  .usr-filter-row :deep(.search-input) {
    width: 100%;
  }

  .usr-filter-pill,
  .usr-filter-pill-wide {
    width: 100%;
  }

  .usr-selection-count {
    order: 5;
  }

  :deep(.data-table-mobile-card) {
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  :deep(.cell-actions) {
    justify-content: flex-start;
    gap: 8px;
  }

  :deep(.cell-actions .icon-btn) {
    width: 44px;
    height: 44px;
    border-radius: 12px;
    border: 1px solid var(--border);
    background: color-mix(in oklch, var(--surface) 80%, transparent);
  }
}
</style>
