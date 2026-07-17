<template>
  <div class="flex items-center gap-1">
    <button
      type="button"
      @click="$emit('run', row)"
      :disabled="running"
      class="flex flex-col items-center gap-0.5 rounded-control p-1.5 text-ink-soft transition-colors hover:bg-page hover:text-brand-600 dark:hover:bg-dark-700 dark:hover:text-brand-400"
    >
      <Icon name="refresh" size="sm" :class="running ? 'animate-spin' : ''" />
      <span class="text-xs">{{ t('admin.channelMonitor.runNow') }}</span>
    </button>
    <button
      type="button"
      @click="$emit('edit', row)"
      class="flex flex-col items-center gap-0.5 rounded-control p-1.5 text-ink-soft transition-colors hover:bg-page hover:text-brand-600 dark:hover:bg-dark-700 dark:hover:text-brand-400"
    >
      <Icon name="edit" size="sm" />
      <span class="text-xs">{{ t('common.edit') }}</span>
    </button>
    <button
      type="button"
      @click="$emit('adjust-availability', row)"
      class="flex flex-col items-center gap-0.5 rounded-control p-1.5 text-ink-soft transition-colors hover:bg-page hover:text-brand-600 dark:hover:bg-dark-700 dark:hover:text-brand-400"
    >
      <Icon name="chartBar" size="sm" />
      <span class="text-xs">{{ t('admin.channelMonitor.adjustAvailability.action') }}</span>
    </button>
    <button
      type="button"
      @click="$emit('delete', row)"
      class="flex flex-col items-center gap-0.5 rounded-control p-1.5 text-ink-soft transition-colors hover:bg-danger-soft hover:text-danger dark:hover:bg-red-900/20 dark:hover:text-red-400"
    >
      <Icon name="trash" size="sm" />
      <span class="text-xs">{{ t('common.delete') }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  row: ChannelMonitor
  running: boolean
}>()

defineEmits<{
  (e: 'run', row: ChannelMonitor): void
  (e: 'edit', row: ChannelMonitor): void
  (e: 'adjust-availability', row: ChannelMonitor): void
  (e: 'delete', row: ChannelMonitor): void
}>()

const { t } = useI18n()
</script>
