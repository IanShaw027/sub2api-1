<template>
  <Teleport to="body">
    <div
      v-if="apiKey && position"
      class="dropdown keys-row-menu"
      :style="{ top: position.top + 'px', left: position.left + 'px' }"
    >
      <button type="button" class="dropdown-item" @click="$emit('use')">
        <Icon name="terminal" size="sm" />
        {{ t('keys.useKey') }}
      </button>
      <button
        v-if="!hideCcsImport"
        type="button"
        class="dropdown-item"
        @click="$emit('import-ccs')"
      >
        <Icon name="upload" size="sm" />
        {{ t('keys.importToCcSwitch') }}
      </button>
      <button type="button" class="dropdown-item" @click="$emit('copy')">
        <Icon name="clipboard" size="sm" />
        {{ t('keys.copyToClipboard') }}
      </button>
      <div class="dropdown-divider" />
      <button type="button" class="dropdown-item" @click="$emit('edit')">
        <Icon name="edit" size="sm" />
        {{ t('common.edit') }}
      </button>
      <button type="button" class="dropdown-item" @click="$emit('toggle-status')">
        <Icon :name="apiKey.status === 'active' ? 'ban' : 'checkCircle'" size="sm" />
        {{ apiKey.status === 'active' ? t('keys.disable') : t('keys.enable') }}
      </button>
      <button
        v-if="showResetRateLimit"
        type="button"
        class="dropdown-item"
        @click="$emit('reset-rate-limit')"
      >
        <Icon name="refresh" size="sm" />
        {{ t('keys.resetRateLimitUsage') }}
      </button>
      <div class="dropdown-divider" />
      <button type="button" class="dropdown-item dropdown-item-danger" @click="$emit('delete')">
        <Icon name="trash" size="sm" />
        {{ t('common.delete') }}
      </button>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { ApiKey } from '@/types'

defineProps<{
  apiKey: ApiKey | null
  position: { top: number; left: number } | null
  hideCcsImport: boolean
  showResetRateLimit: boolean
}>()

defineEmits<{
  use: []
  'import-ccs': []
  copy: []
  edit: []
  'toggle-status': []
  'reset-rate-limit': []
  delete: []
}>()

const { t } = useI18n()
</script>

<style scoped>
.keys-row-menu {
  position: fixed;
  z-index: 100000030;
  min-width: 190px;
}
</style>
