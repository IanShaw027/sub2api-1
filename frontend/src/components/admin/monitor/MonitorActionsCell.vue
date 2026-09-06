<template>
  <ActionsCell
    :show-edit="false"
    :items="items"
  >
    <template #extra>
      <button
        type="button"
        class="icon-btn"
        :aria-label="t('admin.channelMonitor.runNow')"
        :title="t('admin.channelMonitor.runNow')"
        :disabled="running"
        :aria-busy="running || undefined"
        @click.stop="$emit('run', row)"
      >
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': running }" />
      </button>
      <button
        type="button"
        class="icon-btn"
        data-testid="monitor-duplicate"
        :aria-label="duplicateTitle"
        :title="duplicateTitle"
        :disabled="duplicating || Boolean(row.api_key_decrypt_failed)"
        :aria-busy="duplicating || undefined"
        @click.stop="$emit('duplicate', row)"
      >
        <Icon name="copy" size="sm" />
      </button>
      <button
        type="button"
        class="icon-btn"
        :aria-label="t('common.edit')"
        :title="t('common.edit')"
        @click.stop="$emit('edit', row)"
      >
        <Icon name="edit" size="sm" />
      </button>
      <button
        type="button"
        class="icon-btn icon-btn-danger"
        :aria-label="t('common.delete')"
        :title="t('common.delete')"
        @click.stop="$emit('delete', row)"
      >
        <Icon name="trash" size="sm" />
      </button>
    </template>
  </ActionsCell>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import ActionsCell, { type ActionsCellItem } from '@/components/common/cells/ActionsCell.vue'
import Icon from '@/components/icons/Icon.vue'

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

const duplicateTitle = computed(() => {
  if (props.row.api_key_decrypt_failed) return t('admin.channelMonitor.duplicateKeyUnavailable')
  if (props.duplicating) return t('admin.channelMonitor.duplicating')
  return t('admin.channelMonitor.duplicate')
})

const items = computed<ActionsCellItem[]>(() => [
  {
    label: t('admin.channelMonitor.viewDetails'),
    icon: 'chart',
    onClick: () => emit('detail', props.row),
  },
])
</script>
