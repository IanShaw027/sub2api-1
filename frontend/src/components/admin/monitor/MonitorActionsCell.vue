<template>
  <ActionsCell
    :edit-label="t('common.edit')"
    :items="items"
    @edit="$emit('edit', row)"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import ActionsCell, { type ActionsCellItem } from '@/components/common/cells/ActionsCell.vue'

const props = defineProps<{
  row: ChannelMonitor
  running: boolean
  duplicating: boolean
}>()

const emit = defineEmits<{
  (e: 'run', row: ChannelMonitor): void
  (e: 'duplicate', row: ChannelMonitor): void
  (e: 'edit', row: ChannelMonitor): void
  (e: 'delete', row: ChannelMonitor): void
  (e: 'detail', row: ChannelMonitor): void
}>()

const { t } = useI18n()

const items = computed<ActionsCellItem[]>(() => [
  {
    label: t('admin.channelMonitor.viewDetails'),
    icon: 'chart',
    onClick: () => emit('detail', props.row),
  },
  {
    label: t('admin.channelMonitor.runNow'),
    icon: 'refresh',
    disabled: props.running,
    onClick: () => emit('run', props.row),
  },
  {
    label: props.row.api_key_decrypt_failed
      ? t('admin.channelMonitor.duplicateKeyUnavailable')
      : props.duplicating
        ? t('admin.channelMonitor.duplicating')
        : t('admin.channelMonitor.duplicate'),
    icon: 'copy',
    disabled: props.duplicating || Boolean(props.row.api_key_decrypt_failed),
    onClick: () => emit('duplicate', props.row),
  },
  {
    label: t('common.delete'),
    icon: 'trash',
    danger: true,
    onClick: () => emit('delete', props.row),
  },
])
</script>
