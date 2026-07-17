<template>
  <div class="card">
    <div class="border-b border-line px-5 py-4 dark:border-dark-700">
      <h2 class="text-base font-semibold text-ink dark:text-white">{{ t('dashboard.quickActions') }}</h2>
    </div>
    <div class="space-y-3 p-4">
      <button
        @click="router.push('/keys')"
        class="group flex w-full items-center gap-4 rounded-card border border-line/70 bg-page/50 p-4 text-left transition-all duration-150 hover:border-brand/30 hover:bg-brand-50/40 dark:border-dark-700/50 dark:bg-dark-800/40 dark:hover:border-brand/40 dark:hover:bg-brand/10"
      >
        <div class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-control bg-brand-50 transition-transform duration-150 group-hover:scale-105 dark:bg-brand-900/30">
          <Icon name="key" size="md" class="text-brand-600 dark:text-brand-400" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-ink dark:text-white">{{ t('dashboard.createApiKey') }}</p>
          <p class="text-xs text-ink-soft dark:text-dark-400">{{ t('dashboard.generateNewKey') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-ink-faint transition-colors duration-150 group-hover:text-brand-500 dark:text-dark-500"
        />
      </button>

      <button
        @click="router.push('/usage')"
        class="group flex w-full items-center gap-4 rounded-card border border-line/70 bg-page/50 p-4 text-left transition-all duration-150 hover:border-brand/30 hover:bg-brand-50/40 dark:border-dark-700/50 dark:bg-dark-800/40 dark:hover:border-brand/40 dark:hover:bg-brand/10"
      >
        <div class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-control bg-success-soft transition-transform duration-150 group-hover:scale-105 dark:bg-emerald-900/30">
          <Icon name="chart" size="md" class="text-success dark:text-emerald-400" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-ink dark:text-white">{{ t('dashboard.viewUsage') }}</p>
          <p class="text-xs text-ink-soft dark:text-dark-400">{{ t('dashboard.checkDetailedLogs') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-ink-faint transition-colors duration-150 group-hover:text-success dark:text-dark-500"
        />
      </button>

      <button
        v-if="canUseBatchImage"
        @click="router.push('/batch-image')"
        class="group flex w-full items-center gap-4 rounded-card border border-line/70 bg-page/50 p-4 text-left transition-all duration-150 hover:border-accent/30 hover:bg-accent-50/40 dark:border-dark-700/50 dark:bg-dark-800/40 dark:hover:border-accent/40 dark:hover:bg-accent/10"
      >
        <div class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-control bg-accent-50 transition-transform duration-150 group-hover:scale-105 dark:bg-accent-900/30">
          <Icon name="sparkles" size="md" class="text-accent-600 dark:text-accent-400" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-ink dark:text-white">{{ t('dashboard.batchImageAgent') }}</p>
          <p class="text-xs text-ink-soft dark:text-dark-400">{{ t('dashboard.batchImageAgentDesc') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-ink-faint transition-colors duration-150 group-hover:text-accent-500 dark:text-dark-500"
        />
      </button>

      <button
        @click="router.push('/redeem')"
        class="group flex w-full items-center gap-4 rounded-card border border-line/70 bg-page/50 p-4 text-left transition-all duration-150 hover:border-warning/30 hover:bg-warning-soft/60 dark:border-dark-700/50 dark:bg-dark-800/40 dark:hover:border-warning/40 dark:hover:bg-warning/10"
      >
        <div class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-control bg-warning-soft transition-transform duration-150 group-hover:scale-105 dark:bg-amber-900/30">
          <Icon name="gift" size="md" class="text-warning dark:text-amber-400" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-ink dark:text-white">{{ t('dashboard.redeemCode') }}</p>
          <p class="text-xs text-ink-soft dark:text-dark-400">{{ t('dashboard.addBalanceWithCode') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-ink-faint transition-colors duration-150 group-hover:text-warning dark:text-dark-500"
        />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
const router = useRouter()
const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

onMounted(() => {
  void refreshBatchImageAccess()
})
</script>
