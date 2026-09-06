<template>
  <div class="notice notice-info acct-bulk-overlay">
    <div class="acct-bulk-info">
      <span v-if="allResultsSelected" class="acct-bulk-label">
        {{ t('admin.accounts.bulkActions.selectedAll', { count: selectedIds.length }) }}
      </span>
      <span v-else-if="selectedIds.length > 0" class="acct-bulk-label">
        {{ t('admin.accounts.bulkActions.selected', { count: selectedIds.length }) }}
      </span>
      <span v-else class="acct-bulk-label">{{ t('admin.accounts.bulkEdit.title') }}</span>
      <button v-if="selectedIds.length > 0" type="button" class="acct-bulk-link" @click="$emit('select-page')">
        {{ t('admin.accounts.bulkActions.selectCurrentPage') }}
      </button>
      <template v-if="!allResultsSelected && totalResults > selectedIds.length">
        <span v-if="selectedIds.length > 0" class="acct-bulk-sep">·</span>
        <button type="button" class="acct-bulk-link" :disabled="selectingAll" @click="$emit('select-all-results')">
          {{
            selectingAll
              ? t('admin.accounts.bulkActions.selectingAll')
              : t('admin.accounts.bulkActions.selectAllResults', { count: totalResults })
          }}
        </button>
      </template>
      <span v-if="selectedIds.length > 0" class="acct-bulk-sep">·</span>
      <button v-if="selectedIds.length > 0" type="button" class="acct-bulk-link" @click="$emit('clear')">
        {{ t('admin.accounts.bulkActions.clear') }}
      </button>
    </div>
    <div class="acct-bulk-actions">
      <template v-if="selectedIds.length > 0">
      <Button variant="danger" size="sm" @click="$emit('delete')">{{ t('admin.accounts.bulkActions.delete') }}</Button>
      <Button variant="secondary" size="sm" @click="$emit('reset-status')">{{ t('admin.accounts.bulkActions.resetStatus') }}</Button>
      <Button variant="secondary" size="sm" @click="$emit('refresh-token')">{{ t('admin.accounts.bulkActions.refreshToken') }}</Button>
      <Button variant="secondary" size="sm" @click="$emit('probe-upstream-billing')">{{ t('admin.accounts.bulkActions.probeUpstreamBilling') }}</Button>
      <Button variant="success" size="sm" @click="$emit('toggle-schedulable', true)">{{ t('admin.accounts.bulkActions.enableScheduling') }}</Button>
      <Button variant="warning" size="sm" @click="$emit('toggle-schedulable', false)">{{ t('admin.accounts.bulkActions.disableScheduling') }}</Button>
      <Button variant="primary" size="sm" @click="$emit('edit-selected')">{{ t('admin.accounts.bulkActions.edit') }}</Button>
      </template>
      <Button variant="secondary" size="sm" @click="$emit('edit-filtered')">{{ t('admin.accounts.bulkEdit.submit') }}</Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Button from '@/components/ui/Button.vue'

defineProps<{
  selectedIds: number[]
  totalResults: number
  selectingAll: boolean
  allResultsSelected: boolean
}>()

defineEmits([
  'delete',
  'edit-selected',
  'edit-filtered',
  'clear',
  'select-page',
  'select-all-results',
  'toggle-schedulable',
  'reset-status',
  'refresh-token',
  'probe-upstream-billing'
])

const { t } = useI18n()
</script>

<style scoped>
.acct-bulk-overlay {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  background: var(--surface);
}

.acct-bulk-info {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.acct-bulk-label {
  font-weight: 600;
}

.acct-bulk-sep {
  color: var(--muted);
}

.acct-bulk-link {
  border: 0;
  background: transparent;
  padding: 0;
  font-weight: 500;
  color: inherit;
  text-decoration: underline;
  text-underline-offset: 2px;
  cursor: pointer;
}

.acct-bulk-link:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.acct-bulk-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
</style>
