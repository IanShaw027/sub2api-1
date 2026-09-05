<template>
  <BaseDialog :show="!!item" :title="item?.custom_id || t('batchImage.imagePreview.title')" width="extra-wide" :z-index="60" @close="$emit('close')">
    <div class="space-y-3">
      <div class="rounded-lg border border-[color-mix(in_oklch,var(--warning)_35%,transparent)] bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-3 py-2 text-sm text-warning-text ">
        {{ t('batchImage.imagePreview.notice') }}
      </div>
      <div class="flex min-h-[420px] items-center justify-center rounded-lg bg-surface-2 p-4 ">
        <img
          v-if="imageUrl"
          :src="imageUrl"
          class="max-h-[70vh] max-w-full rounded-md object-contain"
          :alt="item?.custom_id || ''"
        />
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { BatchImageItem } from '@/api/batchImage'

defineProps<{
  item: BatchImageItem | null
  imageUrl: string
}>()

defineEmits<{
  close: []
}>()

const { t } = useI18n()
</script>
