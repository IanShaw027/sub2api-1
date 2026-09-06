<template>
  <!-- CC Switch: pick a key to import -->
  <UiModal
    :open="showImportPicker"
    :title="t('keys.ccsImport.title')"
    width="md"
    :close-label="t('common.close')"
    @close="$emit('close-import')"
  >
    <p class="keys-confirm-text">{{ t('keys.ccsImport.description') }}</p>
    <ul class="keys-ccs-list">
      <li v-for="row in apiKeys" :key="row.id" class="keys-ccs-item">
        <span class="keys-ccs-name">{{ row.name }}</span>
        <code class="code keys-ccs-key">{{ maskApiKey(row.key) }}</code>
        <Button variant="secondary" @click="$emit('pick', row)">
          {{ t('keys.importToCcSwitch') }}
        </Button>
      </li>
    </ul>
  </UiModal>

  <!-- CC Switch client selection (Antigravity) -->
  <UiModal
    :open="showClientSelect"
    :title="t('keys.ccsClientSelect.title')"
    width="sm"
    :close-label="t('common.close')"
    @close="$emit('close-client-select')"
  >
    <p class="keys-confirm-text">{{ t('keys.ccsClientSelect.description') }}</p>
    <div class="keys-client-grid">
      <button type="button" class="keys-client-card" @click="$emit('select-client', 'claude')">
        <Icon name="terminal" size="lg" />
        <span class="keys-client-title">{{ t('keys.ccsClientSelect.claudeCode') }}</span>
        <span class="keys-client-desc">{{ t('keys.ccsClientSelect.claudeCodeDesc') }}</span>
      </button>
      <button type="button" class="keys-client-card" @click="$emit('select-client', 'gemini')">
        <Icon name="sparkles" size="lg" />
        <span class="keys-client-title">{{ t('keys.ccsClientSelect.geminiCli') }}</span>
        <span class="keys-client-desc">{{ t('keys.ccsClientSelect.geminiCliDesc') }}</span>
      </button>
    </div>
    <template #footer>
      <Button variant="secondary" @click="$emit('close-client-select')">{{ t('common.cancel') }}</Button>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import UiModal from '@/components/ui/UiModal.vue'
import Button from '@/components/ui/Button.vue'
import Icon from '@/components/icons/Icon.vue'
import type { ApiKey } from '@/types'
import { maskApiKey } from '@/utils/maskApiKey'

defineProps<{
  showImportPicker: boolean
  showClientSelect: boolean
  apiKeys: ApiKey[]
}>()

defineEmits<{
  'close-import': []
  pick: [row: ApiKey]
  'close-client-select': []
  'select-client': [clientType: 'claude' | 'gemini']
}>()

const { t } = useI18n()
</script>

<style scoped>
.keys-confirm-text {
  font-size: 13px;
  color: var(--muted);
  line-height: 1.6;
}

.keys-ccs-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 12px;
}

.keys-ccs-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-field);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
}

.keys-ccs-name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys-ccs-key {
  flex: none;
}

.keys-client-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 12px;
}

.keys-client-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 16px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-card);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  color: var(--muted);
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.keys-client-card:hover {
  border-color: var(--accent);
  background: color-mix(in oklch, var(--accent) 8%, transparent);
}

.keys-client-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.keys-client-desc {
  font-size: 11.5px;
  color: var(--muted);
}
</style>
