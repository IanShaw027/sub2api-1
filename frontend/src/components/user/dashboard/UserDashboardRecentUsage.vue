<template>
  <div class="card">
    <div class="flex items-center justify-between border-b border-line px-5 py-4 dark:border-dark-700">
      <h2 class="text-base font-semibold text-ink dark:text-white">{{ t('dashboard.recentUsage') }}</h2>
      <span class="badge badge-gray">{{ t('dashboard.last7Days') }}</span>
    </div>
    <div class="p-5">
      <div v-if="loading" class="space-y-3 py-2">
        <div v-for="n in 3" :key="n" class="flex items-center gap-4 rounded-card border border-line/60 bg-page/60 p-4 dark:border-dark-700/50 dark:bg-dark-800/30">
          <Skeleton variant="circle" :width="40" :height="40" />
          <div class="min-w-0 flex-1 space-y-2">
            <Skeleton variant="text" width="40%" height="14px" />
            <Skeleton variant="text" width="28%" height="12px" />
          </div>
          <div class="space-y-2 text-right">
            <Skeleton variant="text" width="64px" height="14px" class="ml-auto" />
            <Skeleton variant="text" width="48px" height="12px" class="ml-auto" />
          </div>
        </div>
      </div>
      <div v-else-if="data.length === 0" class="py-8">
        <EmptyState :title="t('dashboard.noUsageRecords')" :description="t('dashboard.startUsingApi')" />
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="log in data"
          :key="log.id"
          class="flex items-center justify-between rounded-card border border-line/70 bg-page/50 p-4 transition-colors duration-150 hover:border-divider hover:bg-page dark:border-dark-700/50 dark:bg-dark-800/40 dark:hover:bg-dark-800"
        >
          <div class="flex items-center gap-4">
            <div class="flex h-10 w-10 items-center justify-center rounded-control bg-brand-50 dark:bg-brand-900/30">
              <Icon name="beaker" size="md" class="text-brand-600 dark:text-brand-400" />
            </div>
            <div>
              <p class="text-sm font-medium text-ink dark:text-white">
                {{ log.model }}
                <span v-if="log.upstream_model && log.upstream_model !== log.model" class="ml-1 text-xs font-normal text-ink-soft dark:text-dark-400">
                  ↳ {{ log.upstream_model }}
                </span>
              </p>
              <p class="text-xs text-ink-soft dark:text-dark-400">{{ formatDateTime(log.created_at) }}</p>
            </div>
          </div>
          <div class="text-right">
            <p class="text-sm font-semibold tabular-nums">
              <span
                :class="log.billed_by_higher_priced_upstream ? 'text-danger dark:text-red-400' : 'text-success dark:text-emerald-400'"
                :title="t('dashboard.actual')"
              >${{ formatCost(log.actual_cost) }}</span>
              <span class="font-normal text-ink-faint dark:text-dark-500" :title="t('dashboard.standard')"> / ${{ formatCost(log.total_cost) }}</span>
            </p>
            <p class="text-xs tabular-nums text-ink-soft dark:text-dark-400">{{ (log.input_tokens + log.output_tokens).toLocaleString() }} tokens</p>
          </div>
        </div>

        <router-link
          to="/usage"
          class="flex items-center justify-center gap-2 py-3 text-sm font-medium text-brand-600 transition-colors duration-150 hover:text-brand-700 dark:text-brand-400 dark:hover:text-brand-300"
        >
          {{ t('dashboard.viewAllUsage') }}
          <Icon name="arrowRight" size="sm" />
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import EmptyState from '@/components/common/EmptyState.vue'
import Skeleton from '@/components/common/Skeleton.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import type { UsageLog } from '@/types'

defineProps<{
  data: UsageLog[]
  loading: boolean
}>()
const { t } = useI18n()
const formatCost = (c: number) => c.toFixed(4)
</script>
