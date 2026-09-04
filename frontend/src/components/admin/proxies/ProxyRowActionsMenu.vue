<template>
  <Teleport to="body">
    <div v-if="proxy && position" class="dropdown proxies-row-menu" :style="{ top: position.top + 'px', left: position.left + 'px' }">
      <button type="button" class="dropdown-item" :disabled="testing" @click="$emit('test-connection')">
        <Icon v-if="testing" name="refresh" size="sm" class="animate-spin" />
        <Icon v-else name="checkCircle" size="sm" />
        {{ t('admin.proxies.testConnection') }}
      </button>
      <button type="button" class="dropdown-item" :disabled="checking" @click="$emit('quality-check')">
        <Icon v-if="checking" name="refresh" size="sm" class="animate-spin" />
        <Icon v-else name="shield" size="sm" />
        {{ t('admin.proxies.qualityCheck') }}
      </button>
      <div class="dropdown-divider"></div>
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
import type { Proxy } from '@/types'

defineProps<{
  proxy: Proxy | null
  position: { top: number; left: number } | null
  testing: boolean
  checking: boolean
}>()

defineEmits<{
  'test-connection': []
  'quality-check': []
  delete: []
}>()

const { t } = useI18n()
</script>

<style scoped>
.proxies-row-menu {
  position: fixed;
  z-index: 100000030;
  min-width: 180px;
}

.proxies-row-menu .dropdown-item:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}
</style>
