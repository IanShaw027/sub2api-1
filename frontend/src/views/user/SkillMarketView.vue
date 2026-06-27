<template>
  <AppLayout>
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <SkillCenterNav :active="installedOnly ? 'installed' : 'market'" />

      <section class="card p-6">
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-6">
          <Input
            v-model="skillsStore.marketFilters.search"
            :label="t('common.search', '搜索')"
            :placeholder="t('skills.market.searchPlaceholder', '搜索技能名、描述、标签')"
          />

          <div>
            <label class="input-label mb-1.5 block">{{ t('skills.editor.type', '类型') }}</label>
            <Select
              :model-value="skillsStore.marketFilters.type"
              :options="typeOptions"
              @update:model-value="(value) => void updateTypeFilter(normalizeType(value))"
            />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('skills.market.priceMode', '收费方式') }}</label>
            <Select
              :model-value="skillsStore.marketFilters.price_mode"
              :options="priceModeOptions"
              @update:model-value="(value) => void updatePriceModeFilter(normalizePriceMode(value))"
            />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('skills.market.installStatus', '安装状态') }}</label>
            <Select
              :model-value="skillsStore.marketFilters.installed"
              :options="installOptions"
              :disabled="installedOnly"
              @update:model-value="(value) => void updateInstalledFilter(normalizeInstalled(value))"
            />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('skills.market.category', '分类') }}</label>
            <Select
              :model-value="skillsStore.marketFilters.category"
              :options="categoryOptions"
              @update:model-value="(value) => void updateCategoryFilter(normalizeCategory(value))"
            />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('skills.market.sort', '排序') }}</label>
            <Select
              :model-value="skillsStore.marketFilters.sort"
              :options="sortOptions"
              @update:model-value="(value) => void updateSortFilter(normalizeSort(value))"
            />
          </div>
        </div>

        <div class="mt-4 flex flex-wrap justify-end gap-3">
          <button class="btn btn-secondary" type="button" @click="void resetFilters()">{{ t('common.reset', '重置') }}</button>
          <button class="btn btn-primary" type="button" :disabled="skillsStore.loadingMarket" @click="void applySearchFilters()">
            <Icon name="refresh" size="sm" class="mr-2" :class="skillsStore.loadingMarket ? 'animate-spin' : ''" />
            {{ installedOnly ? t('common.refresh', '刷新') : t('common.search', '搜索') }}
          </button>
        </div>
      </section>

      <section class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        <SkillCard
          v-for="skill in skillsStore.marketPagination.items"
          :key="skill.id"
          :skill="skill"
          :busy="installingSkillId === skill.id && skillsStore.togglingInstall"
          @toggle-install="handleToggleInstall"
        />

        <div
          v-if="!skillsStore.loadingMarket && skillsStore.marketPagination.items.length === 0"
          class="md:col-span-2 xl:col-span-3"
        >
          <div class="card p-12">
            <EmptyState
              :title="installedOnly ? t('skills.installed.emptyTitle', '还没有已安装技能') : t('skills.market.emptyTitle', '还没有技能')"
              :description="installedOnly ? t('skills.installed.emptyDescription', '先去技能市场安装一些技能。') : t('skills.market.emptyDescription', '当前筛选条件下没有可展示的技能。')"
            />
          </div>
        </div>
      </section>

      <Pagination
        v-if="skillsStore.marketPagination.total > skillsStore.marketPagination.page_size"
        :page="skillsStore.marketPagination.page"
        :total="skillsStore.marketPagination.total"
        :page-size="skillsStore.marketPagination.page_size"
        @update:page="(page) => void updatePage(page)"
        @update:pageSize="(pageSize) => void updatePageSize(pageSize)"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import SkillCard from '@/components/skills/SkillCard.vue'
import SkillCenterNav from '@/components/skills/SkillCenterNav.vue'
import { useSkillsCenterStore } from '@/stores/skillsCenter'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { SkillInstallFilter, SkillPriceMode, SkillSortKey, SkillSummary, SkillType } from '@/types/skills'

const { t } = useI18n()
const appStore = useAppStore()
const skillsStore = useSkillsCenterStore()
const route = useRoute()
const router = useRouter()
const props = withDefaults(defineProps<{ installedOnly?: boolean }>(), {
  installedOnly: false
})

const installingSkillId = ref<number | null>(null)
let suppressRouteFilterReload = false

const typeOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'prompt_chat', label: 'prompt_chat' },
  { value: 'prompt_image', label: 'prompt_image' },
  { value: 'script', label: 'script' }
])

const priceModeOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'free', label: t('skills.market.free', '免费') },
  { value: 'paid', label: t('skills.market.paid', '付费') }
])

const installOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'installed', label: t('skills.market.installedOnly', '已安装') },
  { value: 'not_installed', label: t('skills.market.notInstalledOnly', '未安装') }
])

const sortOptions = computed(() => [
  { value: 'latest', label: t('skills.market.sortLatest', '最新') },
  { value: 'popular', label: t('skills.market.sortPopular', '热门') },
  { value: 'runs', label: t('skills.market.sortRuns', '运行量') },
  { value: 'revenue', label: t('skills.market.sortRevenue', '收益') },
  { value: 'price_low', label: t('skills.market.sortPriceLow', '价格升序') },
  { value: 'price_high', label: t('skills.market.sortPriceHigh', '价格降序') }
])

const categoryOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  ...skillsStore.availableCategories.map((item) => ({
    value: item,
    label: item
  }))
])

function normalizeType(value: string | number | boolean | null): SkillType | 'all' {
  return value === 'prompt_chat' || value === 'prompt_image' || value === 'script' ? value : 'all'
}

function normalizePriceMode(value: string | number | boolean | null): SkillPriceMode | 'all' {
  if (typeof value !== 'string') return 'all'
  const normalized = value.trim().toLowerCase()
  return normalized === 'free' || normalized === 'paid' ? normalized : 'all'
}

function normalizeInstalled(value: string | number | boolean | null): SkillInstallFilter {
  return value === 'installed' || value === 'not_installed' ? value : 'all'
}

function normalizeSort(value: string | number | boolean | null): SkillSortKey {
  return value === 'popular' || value === 'runs' || value === 'revenue' || value === 'price_low' || value === 'price_high'
    ? value
    : 'latest'
}

function normalizeCategory(value: string | number | boolean | null): string | 'all' {
  return typeof value === 'string' && value.trim() ? value : 'all'
}

function extractQueryString(value: unknown): string | null {
  if (typeof value === 'string') return value
  if (Array.isArray(value) && typeof value[0] === 'string') return value[0]
  return null
}

function extractPositiveQueryNumber(value: unknown, fallback: number): number {
  const raw = extractQueryString(value)
  if (!raw) return fallback
  const parsed = Number.parseInt(raw, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function syncRouteDrivenFilters(installedOnly: boolean): void {
  skillsStore.marketFilters.search = extractQueryString(route.query.search) ?? ''
  skillsStore.marketFilters.type = normalizeType(extractQueryString(route.query.type))
  skillsStore.marketFilters.price_mode = normalizePriceMode(extractQueryString(route.query.price_mode))
  skillsStore.marketFilters.category = normalizeCategory(extractQueryString(route.query.category))
  skillsStore.marketFilters.installed = installedOnly
    ? 'installed'
    : normalizeInstalled(extractQueryString(route.query.installed))
  skillsStore.marketFilters.sort = normalizeSort(extractQueryString(route.query.sort))
}

function currentRoutePage(): number {
  return extractPositiveQueryNumber(route.query.page, 1)
}

function currentRoutePageSize(): number {
  return extractPositiveQueryNumber(route.query.page_size, skillsStore.marketPagination.page_size)
}

async function replaceMarketQuery(page = currentRoutePage(), pageSize = currentRoutePageSize()): Promise<boolean> {
  const nextQuery = { ...route.query }
  const nextSearch = skillsStore.marketFilters.search.trim() || null
  const nextType = skillsStore.marketFilters.type !== 'all' ? skillsStore.marketFilters.type : null
  const nextPriceMode = skillsStore.marketFilters.price_mode !== 'all' ? skillsStore.marketFilters.price_mode : null
  const nextCategory = skillsStore.marketFilters.category !== 'all' ? skillsStore.marketFilters.category : null
  const nextInstalled =
    props.installedOnly || skillsStore.marketFilters.installed === 'all' ? null : skillsStore.marketFilters.installed
  const nextSort = skillsStore.marketFilters.sort !== 'latest' ? skillsStore.marketFilters.sort : null
  const nextPage = page > 1 ? String(page) : null
  const nextPageSize = pageSize !== 18 ? String(pageSize) : null

  if (nextSearch) nextQuery.search = nextSearch
  else delete nextQuery.search

  if (nextType) nextQuery.type = nextType
  else delete nextQuery.type

  if (nextPriceMode) nextQuery.price_mode = nextPriceMode
  else delete nextQuery.price_mode

  if (nextCategory) nextQuery.category = nextCategory
  else delete nextQuery.category

  if (nextInstalled) nextQuery.installed = nextInstalled
  else delete nextQuery.installed

  if (nextSort) nextQuery.sort = nextSort
  else delete nextQuery.sort

  if (nextPage) nextQuery.page = nextPage
  else delete nextQuery.page

  if (nextPageSize) nextQuery.page_size = nextPageSize
  else delete nextQuery.page_size

  const currentSearch = extractQueryString(route.query.search)
  const currentType = extractQueryString(route.query.type)
  const currentPriceMode = extractQueryString(route.query.price_mode)
  const currentCategory = extractQueryString(route.query.category)
  const currentInstalled = extractQueryString(route.query.installed)
  const currentSort = extractQueryString(route.query.sort)
  const currentPage = extractQueryString(route.query.page)
  const currentPageSize = extractQueryString(route.query.page_size)

  if (
    currentSearch === nextSearch &&
    currentType === nextType &&
    currentPriceMode === nextPriceMode &&
    currentCategory === nextCategory &&
    currentInstalled === nextInstalled &&
    currentSort === nextSort &&
    currentPage === nextPage &&
    currentPageSize === nextPageSize
  ) {
    return false
  }

  suppressRouteFilterReload = true
  await router.replace({ query: nextQuery })
  return true
}

async function reloadMarket(): Promise<void> {
  try {
    await skillsStore.loadMarket(currentRoutePage(), currentRoutePageSize())
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function applySearchFilters(): Promise<void> {
  await replaceMarketQuery(1, skillsStore.marketPagination.page_size)
  await reloadMarket()
}

async function updateTypeFilter(value: SkillType | 'all'): Promise<void> {
  skillsStore.marketFilters.type = value
  await replaceMarketQuery(1, skillsStore.marketPagination.page_size)
  await reloadMarket()
}

async function updatePriceModeFilter(value: SkillPriceMode | 'all'): Promise<void> {
  skillsStore.marketFilters.price_mode = value
  await replaceMarketQuery(1, skillsStore.marketPagination.page_size)
  await reloadMarket()
}

async function updateInstalledFilter(value: SkillInstallFilter): Promise<void> {
  skillsStore.marketFilters.installed = props.installedOnly ? 'installed' : value
  await replaceMarketQuery(1, skillsStore.marketPagination.page_size)
  await reloadMarket()
}

async function updateCategoryFilter(value: string | 'all'): Promise<void> {
  skillsStore.marketFilters.category = value
  await replaceMarketQuery(1, skillsStore.marketPagination.page_size)
  await reloadMarket()
}

async function updateSortFilter(value: SkillSortKey): Promise<void> {
  skillsStore.marketFilters.sort = value
  await replaceMarketQuery(1, skillsStore.marketPagination.page_size)
  await reloadMarket()
}

async function resetFilters(): Promise<void> {
  skillsStore.resetMarketFilters()
  if (props.installedOnly) {
    skillsStore.marketFilters.installed = 'installed'
  }
  await replaceMarketQuery(1, 18)
  await reloadMarket()
}

async function updatePage(page: number): Promise<void> {
  await replaceMarketQuery(page, skillsStore.marketPagination.page_size)
  await reloadMarket()
}

async function updatePageSize(pageSize: number): Promise<void> {
  await replaceMarketQuery(1, pageSize)
  await reloadMarket()
}

async function handleToggleInstall(skill: SkillSummary): Promise<void> {
  installingSkillId.value = skill.id
  try {
    const shouldRefresh = props.installedOnly || skillsStore.marketFilters.installed !== 'all'
    await skillsStore.toggleInstall(skill.id, skill.installed)
    if (shouldRefresh) {
      await reloadMarket()
    }
    appStore.showSuccess(skill.installed ? t('skills.market.uninstallSuccess', '已卸载技能') : t('skills.market.installSuccess', '已安装技能'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    installingSkillId.value = null
  }
}

watch(
  () => [
    route.query.type,
    route.query.search,
    route.query.price_mode,
    route.query.category,
    route.query.installed,
    route.query.sort,
    route.query.page,
    route.query.page_size,
    props.installedOnly
  ],
  async () => {
    syncRouteDrivenFilters(props.installedOnly)
    if (suppressRouteFilterReload) {
      suppressRouteFilterReload = false
      return
    }
    await reloadMarket()
  },
  { immediate: true }
)
</script>
