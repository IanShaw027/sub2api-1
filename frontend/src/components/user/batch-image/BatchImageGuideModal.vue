<template>
  <BaseDialog :show="show" :title="t('batchImage.guide.title')" width="wide" @close="$emit('close')">
    <div class="space-y-5">
      <section class="space-y-3">
        <h3 class="text-sm font-semibold text-foreground">{{ t('batchImage.guide.uiTitle') }}</h3>
        <div class="rounded-lg border border-line bg-surface-2 p-3 text-sm leading-6 text-foreground ">
          <p>{{ t('batchImage.guide.step1') }}</p>
          <p>{{ t('batchImage.guide.step2') }}</p>
          <p>{{ t('batchImage.guide.step3') }}</p>
          <p>{{ t('batchImage.guide.step4') }}</p>
        </div>
      </section>
      <section class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h3 class="text-sm font-semibold text-foreground">{{ t('batchImage.guide.skillTitle') }}</h3>
          <p class="text-xs text-muted">{{ t('batchImage.guide.skillDesc') }}</p>
        </div>
        <textarea
          :value="instruction"
          readonly
          class="min-h-[420px] w-full resize-y rounded-md border border-line bg-surface-2 p-4 font-mono text-sm leading-6 text-foreground outline-none focus:border-[var(--accent)] focus:ring-2 focus:ring-[color-mix(in_oklch,var(--accent)_18%,transparent)]"
        />
      </section>
    </div>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn-glass-secondary" @click="$emit('close')">{{ t('common.close') }}</button>
        <button type="button" class="btn-glass-primary" @click="$emit('copy')">
          <Icon name="copy" size="sm" class="mr-2" />
          {{ t('batchImage.actions.copyInstruction') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  show: boolean
  instruction: string
}>()

defineEmits<{
  close: []
  copy: []
}>()

const { t } = useI18n()
</script>
