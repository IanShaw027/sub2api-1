<template>
  <GlassCard padding="sm">
    <template #header>
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-semibold text-foreground">{{ t('dashboard.recentUsage') }}</h2>
        <span class="badge-tone-muted">{{ t('dashboard.last7Days') }}</span>
      </div>
    </template>
    <div v-if="loading" class="flex items-center justify-center py-12">
      <LoadingSpinner size="lg" />
    </div>
    <div v-else-if="data.length === 0" class="py-8">
      <EmptyState :title="t('dashboard.noUsageRecords')" :description="t('dashboard.startUsingApi')" />
    </div>
    <div v-else class="space-y-3">
      <div v-for="log in data" :key="log.id" class="flex items-center justify-between rounded-xl bg-surface-2 p-4 transition-colors hover:bg-surface-3">
        <div class="flex items-center gap-4">
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-surface-3">
            <Icon name="beaker" size="md" class="text-accent" />
          </div>
          <div>
            <p class="text-sm font-medium text-foreground">{{ log.model }}</p>
            <p class="text-xs text-muted">{{ formatDateTime(log.created_at) }}</p>
          </div>
        </div>
        <div class="text-right">
          <p class="text-sm font-semibold">
            <span class="text-success-text" :title="t('dashboard.actual')">${{ formatCost(log.actual_cost) }}</span>
            <span class="font-normal text-muted" :title="t('dashboard.standard')"> / ${{ formatCost(log.total_cost) }}</span>
          </p>
          <p class="text-xs text-muted">{{ (log.input_tokens + log.output_tokens).toLocaleString() }} tokens</p>
        </div>
      </div>

      <router-link to="/usage" class="flex items-center justify-center gap-2 py-3 text-sm font-medium text-accent transition-colors hover:text-foreground">
        {{ t('dashboard.viewAllUsage') }}
        <Icon name="arrowRight" size="sm" />
      </router-link>
    </div>
  </GlassCard>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import { formatDateTime } from '@/utils/format'
import type { UsageLog } from '@/types'

defineProps<{
  data: UsageLog[]
  loading: boolean
}>()
const { t } = useI18n()
const formatCost = (c: number) => c.toFixed(4)
</script>
