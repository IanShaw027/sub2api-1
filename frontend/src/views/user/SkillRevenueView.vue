<template>
  <AppLayout>
    <div v-if="showRevenuePage" class="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <SkillCenterNav
        active="revenue"
        :skill-id="skillId"
        :can-edit-skill="Boolean(skill?.editable)"
        :can-view-runs="Boolean(skill?.owned)"
        :can-view-revenue="Boolean(skill?.owned)"
      />

      <div v-if="revenue" class="space-y-6">
        <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <div class="card p-5">
            <p class="text-xs uppercase tracking-[0.35em] text-ink-faint dark:text-dark-400">{{ t('skills.revenue.totalRevenue', '累计收益') }}</p>
            <p class="mt-2 text-3xl font-bold text-ink dark:text-white">{{ formatCurrency(revenue.summary.total_revenue, revenue.summary.currency) }}</p>
          </div>
          <div class="card p-5">
            <p class="text-xs uppercase tracking-[0.35em] text-ink-faint dark:text-dark-400">{{ t('skills.revenue.totalSales', '销售次数') }}</p>
            <p class="mt-2 text-3xl font-bold text-ink dark:text-white">{{ revenue.summary.total_sales }}</p>
          </div>
          <div class="card p-5">
            <p class="text-xs uppercase tracking-[0.35em] text-ink-faint dark:text-dark-400">{{ t('skills.revenue.totalRuns', '执行次数') }}</p>
            <p class="mt-2 text-3xl font-bold text-ink dark:text-white">{{ revenue.summary.total_runs }}</p>
          </div>
          <div class="card p-5">
            <p class="text-xs uppercase tracking-[0.35em] text-ink-faint dark:text-dark-400">{{ t('skills.revenue.pendingAmount', '待结算') }}</p>
            <p class="mt-2 text-3xl font-bold text-ink dark:text-white">{{ formatCurrency(revenue.summary.pending_amount, revenue.summary.currency) }}</p>
          </div>
          <div class="card p-5">
            <p class="text-xs uppercase tracking-[0.35em] text-ink-faint dark:text-dark-400">{{ t('skills.revenue.settledAmount', '已结算') }}</p>
            <p class="mt-2 text-3xl font-bold text-ink dark:text-white">{{ formatCurrency(revenue.summary.settled_amount, revenue.summary.currency) }}</p>
          </div>
          <div class="card p-5">
            <p class="text-xs uppercase tracking-[0.35em] text-ink-faint dark:text-dark-400">{{ t('skills.revenue.refundedAmount', '退款') }}</p>
            <p class="mt-2 text-3xl font-bold text-ink dark:text-white">{{ formatCurrency(revenue.summary.refunded_amount, revenue.summary.currency) }}</p>
          </div>
        </section>

        <section class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_420px]">
          <div class="card p-6">
            <div class="mb-4 flex items-center justify-between gap-3">
              <div>
                <h2 class="text-lg font-semibold text-ink dark:text-white">{{ t('skills.revenue.trend', '收益趋势') }}</h2>
                <p class="mt-1 text-sm text-ink-soft dark:text-dark-400">{{ t('skills.revenue.trendHint', '按接口返回的时间粒度展示收益、销售和执行次数。') }}</p>
              </div>
            </div>

            <div class="space-y-3">
              <div
                v-for="point in revenue.trend"
                :key="point.date"
                class="grid gap-3 rounded-card border border-line p-4 md:grid-cols-4 dark:border-dark-700"
              >
                <div>
                  <p class="text-xs uppercase tracking-[0.25em] text-ink-faint dark:text-dark-400">{{ t('common.date', '日期') }}</p>
                  <p class="mt-1 font-medium text-ink dark:text-white">{{ point.date }}</p>
                </div>
                <div>
                  <p class="text-xs uppercase tracking-[0.25em] text-ink-faint dark:text-dark-400">{{ t('skills.revenue.totalRevenue', '累计收益') }}</p>
                  <p class="mt-1 font-medium text-ink dark:text-white">{{ formatCurrency(point.revenue, revenue.summary.currency) }}</p>
                </div>
                <div>
                  <p class="text-xs uppercase tracking-[0.25em] text-ink-faint dark:text-dark-400">{{ t('skills.revenue.totalSales', '销售次数') }}</p>
                  <p class="mt-1 font-medium text-ink dark:text-white">{{ point.sales }}</p>
                </div>
                <div>
                  <p class="text-xs uppercase tracking-[0.25em] text-ink-faint dark:text-dark-400">{{ t('skills.revenue.totalRuns', '执行次数') }}</p>
                  <p class="mt-1 font-medium text-ink dark:text-white">{{ point.runs }}</p>
                </div>
              </div>

              <EmptyState
                v-if="revenue.trend.length === 0"
                :title="t('skills.revenue.emptyTrendTitle', '还没有趋势数据')"
                :description="t('skills.revenue.emptyTrendDescription', '有订单或执行后，这里会展示时间趋势。')"
              />
            </div>
          </div>

          <div v-if="skill" class="card p-6">
            <h2 class="text-lg font-semibold text-ink dark:text-white">{{ t('skills.detail.meta', '元信息') }}</h2>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('common.name', '名称') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.name }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('skills.editor.type', '类型') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.type }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('skills.versions.currentVersion', '当前版本') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.latest_version?.version || '-' }}</dd>
              </div>
            </dl>
          </div>
        </section>

        <section class="card p-6">
          <div class="mb-4">
            <h2 class="text-lg font-semibold text-ink dark:text-white">{{ t('skills.revenue.orders', '收益订单') }}</h2>
            <p class="mt-1 text-sm text-ink-soft dark:text-dark-400">{{ t('skills.revenue.ordersHint', '这里展示买家、版本、金额与结算状态。') }}</p>
          </div>
          <DataTable :columns="columns" :data="revenue.orders" :loading="skillsStore.loadingRevenue">
            <template #cell-amount="{ row }">
              <span class="font-medium text-ink dark:text-white">{{ formatCurrency(row.amount, row.currency) }}</span>
            </template>

            <template #cell-status="{ row }">
              <span class="rounded-full bg-line px-2.5 py-1 text-[11px] font-medium text-ink-body dark:bg-dark-800 dark:text-dark-200">
                {{ row.status }}
              </span>
            </template>
          </DataTable>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, watch, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import type { Column } from '@/components/common/types'
import SkillCenterNav from '@/components/skills/SkillCenterNav.vue'
import { skillPaths } from '@/components/skills/paths'
import { formatCurrency } from '@/components/skills/presentation'
import { useSkillsCenterStore } from '@/stores/skillsCenter'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const skillsStore = useSkillsCenterStore()

const skillId = computed(() => {
  const value = Number(route.params.id)
  return Number.isFinite(value) && value > 0 ? value : 0
})
const skill = computed(() => skillsStore.detail)
const revenue = computed(() => skillsStore.revenue)
const router = useRouter()
const loadedSkillId = ref<number | null>(null)
const showRevenuePage = computed(() => loadedSkillId.value === skillId.value)

const columns = computed<Column[]>(() => [
  { key: 'buyer_name', label: t('skills.revenue.buyer', '买家'), class: 'w-40' },
  { key: 'version', label: t('skills.versions.version', '版本号'), class: 'w-32' },
  { key: 'amount', label: t('skills.revenue.amount', '金额'), class: 'w-32' },
  { key: 'status', label: t('common.status', '状态'), class: 'w-28' },
  { key: 'created_at', label: t('common.date', '日期'), class: 'w-44' }
])

async function loadPage(force = false): Promise<void> {
  if (!skillId.value) return
  loadedSkillId.value = null
  try {
    await skillsStore.loadSkillDetail(skillId.value, force)
    if (!skill.value?.owned) {
      await router.replace(skillPaths.detail(skillId.value))
      return
    }
    await skillsStore.loadRevenue(skillId.value, force)
    loadedSkillId.value = skillId.value
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

watch(skillId, () => {
  void loadPage(true)
})

onMounted(() => {
  void loadPage()
})
</script>
