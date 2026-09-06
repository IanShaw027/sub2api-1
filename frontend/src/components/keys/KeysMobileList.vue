<template>
  <div class="keys-mobile-wrap">
    <div class="keys-mobile-list">
      <template v-if="loading">
        <div v-for="i in 4" :key="i" class="glass-card keys-card">
          <div class="skeleton h-4 w-32"></div>
          <div class="skeleton h-10 w-full"></div>
          <div class="skeleton h-6 w-full"></div>
        </div>
      </template>
      <EmptyState
        v-else-if="apiKeys.length === 0"
        :title="t('keys.noKeysYet')"
        :description="t('keys.createFirstKey')"
        :action-text="t('keys.createKey')"
        @action="$emit('empty-action')"
      />
      <KeyMobileCard
        v-for="row in apiKeys"
        v-else
        :key="row.id"
        :row="row"
        :revealed="isKeyRevealed(row.id)"
        :copied="copiedKeyId === row.id"
        :today-cost="usageStats[row.id]?.today_actual_cost"
        :total-cost="usageStats[row.id]?.total_actual_cost"
        :now="now"
        :hide-ccs-import="hideCcsImport"
        :set-group-button-ref="setGroupButtonRef"
        @more="$emit('more', row, $event)"
        @toggle-reveal="$emit('toggle-reveal', row.id)"
        @copy="$emit('copy', row.key, row.id)"
        @use="$emit('use', row)"
        @import-ccs="$emit('import-ccs', row)"
        @toggle-status="$emit('toggle-status', row)"
        @edit="$emit('edit', row)"
        @delete="$emit('delete', row)"
        @reset-rate-limit="$emit('reset-rate-limit', row)"
        @open-group-selector="$emit('open-group-selector', row)"
      />
    </div>
    <ListFade class="keys-list-fade" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ComponentPublicInstance } from 'vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ListFade from '@/components/ui/ListFade.vue'
import KeyMobileCard from './KeyMobileCard.vue'
import type { ApiKey } from '@/types'
import type { BatchApiKeyUsageStats } from '@/api/usage'

defineProps<{
  loading: boolean
  apiKeys: ApiKey[]
  copiedKeyId: number | null
  usageStats: Record<string, BatchApiKeyUsageStats>
  now: Date
  isKeyRevealed: (id: number) => boolean
  hideCcsImport?: boolean
  setGroupButtonRef?: (id: number, el: Element | ComponentPublicInstance | null) => void
}>()

defineEmits<{
  more: [row: ApiKey, event: MouseEvent]
  'toggle-reveal': [id: number]
  copy: [key: string, id: number]
  use: [row: ApiKey]
  'import-ccs': [row: ApiKey]
  'toggle-status': [row: ApiKey]
  edit: [row: ApiKey]
  delete: [row: ApiKey]
  'reset-rate-limit': [row: ApiKey]
  'open-group-selector': [row: ApiKey]
  'empty-action': []
}>()

const { t } = useI18n()
</script>

<style scoped>
.keys-mobile-wrap {
  position: relative;
  flex: 1;
  min-height: 0;
}

.keys-mobile-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.keys-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border-radius: var(--radius-card);
}

.keys-list-fade {
  display: none;
}

@media (max-width: 767px) {
  .keys-list-fade {
    display: block;
  }

  .keys-mobile-list {
    padding-bottom: 72px;
  }
}
</style>
