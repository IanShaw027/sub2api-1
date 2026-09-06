<template>
  <div class="keys-inline-actions">
    <button type="button" class="icon-btn" :title="t('keys.useKey')" :aria-label="t('keys.useKey')" @click.stop="$emit('use')">
      <Icon name="terminal" size="sm" />
    </button>
    <button v-if="!hideCcsImport" type="button" class="icon-btn" :title="t('keys.importToCcSwitch')" :aria-label="t('keys.importToCcSwitch')" @click.stop="$emit('import-ccs')">
      <Icon name="upload" size="sm" />
    </button>
    <button type="button" class="icon-btn" :title="row.status === 'active' ? t('keys.disable') : t('keys.enable')" :aria-label="row.status === 'active' ? t('keys.disable') : t('keys.enable')" @click.stop="$emit('toggle-status')">
      <Icon :name="row.status === 'active' ? 'ban' : 'checkCircle'" size="sm" />
    </button>
    <button type="button" class="icon-btn" :title="t('common.edit')" :aria-label="t('common.edit')" @click.stop="$emit('edit')">
      <Icon name="edit" size="sm" />
    </button>
    <button type="button" class="icon-btn icon-btn-danger" :title="t('common.delete')" :aria-label="t('common.delete')" @click.stop="$emit('delete')">
      <Icon name="trash" size="sm" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { ApiKey } from '@/types'

defineProps<{ row: ApiKey; hideCcsImport?: boolean }>()
defineEmits<{ use: []; 'import-ccs': []; 'toggle-status': []; edit: []; delete: [] }>()
const { t } = useI18n()
</script>

<style scoped>
.keys-inline-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 2px; }
.keys-inline-actions .icon-btn { flex: none; width: 32px; height: 32px; }
@media (max-width: 767px) {
  .keys-inline-actions .icon-btn { width: 44px; height: 44px; }
}
</style>
