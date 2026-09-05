<template>
  <BaseDialog :show="show" :title="t('batchImage.create.title')" width="wide" @close="$emit('close')">
    <form class="space-y-5" @submit.prevent="$emit('submit')">
      <div class="grid gap-4 md:grid-cols-2">
        <div class="md:col-span-2">
          <label class="login-label">{{ t('batchImage.create.taskName') }}</label>
          <input
            v-model="form.taskName"
            type="text"
            maxlength="255"
            class="field"
            :placeholder="t('batchImage.create.taskNamePlaceholder')"
          />
        </div>

        <div class="md:col-span-2">
          <label class="login-label">API Key</label>
          <select v-model.number="form.apiKeyId" class="field" :disabled="loadingKeys">
            <option :value="0">{{ loadingKeys ? t('batchImage.create.loadingKeys') : t('batchImage.create.selectKeyPlaceholder') }}</option>
            <option v-for="key in geminiApiKeys" :key="key.id" :value="key.id">
              {{ key.name }} · {{ key.group?.name || 'Gemini' }}
            </option>
          </select>
          <p v-if="!loadingKeys && geminiApiKeys.length === 0" class="input-hint text-warning-text ">
            {{ t('batchImage.create.noKeysHint') }}
          </p>
        </div>

        <div>
          <label class="login-label">{{ t('batchImage.create.model') }}</label>
          <select v-model="form.model" class="field" :disabled="loadingModels || availableBatchImageModels.length === 0">
            <option v-if="loadingModels" value="">{{ batchImageText('loadingModels') }}</option>
            <option v-else-if="availableBatchImageModels.length === 0" value="">{{ batchImageText('noModels') }}</option>
            <option v-for="model in availableBatchImageModels" :key="model.value" :value="model.value">
              {{ model.label }}
            </option>
          </select>
          <p v-if="modelLoadError" class="input-hint text-warning-text ">
            {{ modelLoadError }}
          </p>
          <p v-else-if="selectedApiKey && !loadingModels && availableBatchImageModels.length === 0" class="input-hint text-warning-text ">
            {{ batchImageText('noModelsHint') }}
          </p>
        </div>

        <div>
          <label class="login-label">{{ t('batchImage.create.imageSize') }}</label>
          <div class="field flex items-center bg-surface-2 text-muted ">
            1K
          </div>
          <p class="input-hint">{{ t('batchImage.create.imageSizeHint') }}</p>
        </div>

        <div>
          <label class="login-label">{{ t('batchImage.create.outputFormat') }}</label>
          <select v-model="form.responseMimeType" class="field">
            <option value="image/png">PNG</option>
            <option value="image/jpeg">JPEG</option>
            <option value="image/webp">WebP</option>
          </select>
        </div>

        <div>
          <label class="login-label">{{ t('batchImage.create.estimatedOutput') }}</label>
          <div class="field flex items-center bg-surface-2 text-muted ">
            {{ t('batchImage.create.estimatedOutputValue', { images: estimatedOutputCount, prompts: promptRows.length }) }}
          </div>
        </div>
      </div>

      <div class="space-y-3">
        <div class="flex items-center justify-between gap-3">
          <label class="login-label mb-0">Prompt</label>
          <span class="text-xs text-muted">{{ t('batchImage.create.promptAdded', { count: promptRows.length }) }}</span>
        </div>
        <div class="rounded-lg border border-line p-3 ">
          <textarea
            v-model="promptDraft"
            rows="3"
            class="h-[76px] w-full resize-y rounded-md border border-line px-3 py-2 text-sm leading-5 outline-none focus:border-[var(--accent)] focus:ring-2 focus:ring-[color-mix(in_oklch,var(--accent)_18%,transparent)]"
            :placeholder="t('batchImage.create.promptPlaceholder')"
          />
          <div class="mt-2 grid gap-2 md:grid-cols-[minmax(0,1fr)_112px_132px_112px] md:items-center">
            <input
              v-model="customIdDraft"
              type="text"
              maxlength="255"
              class="field h-9 text-sm"
              :placeholder="t('batchImage.create.customIdPlaceholder')"
            />
            <select
              v-model.number="outputCountDraft"
              class="batch-output-count-select input h-9 text-sm"
              :title="t('batchImage.create.outputCountPerPrompt')"
              :aria-label="t('batchImage.create.outputCountPerPrompt')"
            >
              <option v-for="count in outputCountOptions" :key="count" :value="count">
                {{ t('batchImage.create.outputCountOption', { n: count }, count) }}
              </option>
            </select>
            <label
              class="btn-glass-secondary h-9 cursor-pointer justify-center text-sm"
              :class="referenceImageDrafts.length >= selectedModelReferenceLimit ? 'pointer-events-none opacity-60' : ''"
            >
              <Icon name="upload" size="sm" class="mr-1.5" />
              {{ t('batchImage.create.referenceImage') }}
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                multiple
                class="hidden"
                :disabled="referenceImageDrafts.length >= selectedModelReferenceLimit"
                @change="$emit('upload-reference', $event)"
              />
            </label>
            <button type="button" class="btn-glass-secondary h-9 justify-center whitespace-nowrap px-4 text-sm" :disabled="!promptDraft.trim()" @click="$emit('add-prompt')">
              <Icon name="plus" size="sm" class="mr-1.5" />
              {{ t('common.add') }}
            </button>
          </div>
          <div v-if="referenceImageDrafts.length" class="mt-3 flex flex-wrap gap-2">
            <span
              v-for="(ref, refIndex) in referenceImageDrafts"
              :key="`${ref.name}-${refIndex}`"
              class="inline-flex max-w-full items-center gap-1 rounded-md border border-line bg-surface-2 px-2 py-1 text-xs text-foreground "
            >
              <span class="max-w-[180px] truncate">{{ ref.name }}</span>
              <button type="button" class="text-muted hover:text-danger-text" :title="t('batchImage.create.removeReferenceImage')" @click="$emit('remove-reference', refIndex)">
                <Icon name="x" size="xs" />
              </button>
            </span>
          </div>
          <p class="mt-2 text-xs text-muted">
            {{ t('batchImage.create.limitsHint', { maxPerItem: maxOutputsPerItem, maxPerJob: maxOutputsPerJob, refLimit: selectedModelReferenceLimit }) }}
          </p>
        </div>
        <div v-if="promptRows.length" class="overflow-hidden rounded-lg border border-line">
          <div
            v-for="(row, index) in promptRows"
            :key="row.localId"
            class="flex items-center gap-3 border-b border-line px-3 py-2 last:border-b-0 "
          >
            <span class="w-20 flex-shrink-0 font-mono text-xs text-muted">{{ row.custom_id }}</span>
            <p class="min-w-0 flex-1 truncate text-sm text-foreground">{{ row.prompt }}</p>
            <span v-if="row.output_count > 1" class="flex-shrink-0 text-xs text-muted">
              x{{ row.output_count }}
            </span>
            <span v-if="row.reference_images.length" class="flex-shrink-0 text-xs text-muted">
              {{ t('batchImage.create.referenceCount', { n: row.reference_images.length }, row.reference_images.length) }}
            </span>
            <button type="button" class="btn-ghost btn-icon flex-shrink-0 text-danger-text hover:bg-[color-mix(in_oklch,var(--danger)_14%,transparent)]" :title="t('common.delete')" @click="$emit('remove-prompt', index)">
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>
        <div v-else class="rounded-lg border border-dashed border-line px-3 py-6 text-center text-sm text-muted ">
          {{ t('batchImage.create.noPrompts') }}
        </div>
      </div>

      <div class="rounded-lg border border-[color-mix(in_oklch,var(--warning)_35%,transparent)] bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] p-3 text-sm leading-6 text-warning-text ">
        {{ t('batchImage.create.cancelNotice') }}
      </div>
      <div v-if="submitting" class="rounded-lg border border-[var(--accent)] bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] p-3 text-sm leading-6 text-accent ">
        {{ t('batchImage.create.submittingNotice') }}
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn-glass-secondary" :disabled="submitting" @click="$emit('close')">{{ t('common.cancel') }}</button>
        <button type="button" class="btn-glass-primary inline-flex min-w-[120px] justify-center" :disabled="submitting || loadingModels || (parsedItemsCount === 0 && !promptDraft.trim()) || !selectedApiKey || !form.model" @click="$emit('submit')">
          <Icon v-if="submitting" name="refresh" size="sm" class="mr-2 animate-spin" />
          {{ submitting ? t('common.submitting') : t('batchImage.actions.submitJob') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { ApiKey } from '@/types'
import type { PromptRow, ReferenceImageDraft } from '@/views/user/batchImage/types'
import type { BatchImageTextKey } from '@/views/user/batchImage/errorMessages'

const form = defineModel<{ taskName: string; apiKeyId: number; model: string; responseMimeType: string }>('form', { required: true })
const promptDraft = defineModel<string>('promptDraft', { required: true })
const customIdDraft = defineModel<string>('customIdDraft', { required: true })
const outputCountDraft = defineModel<number>('outputCountDraft', { required: true })

defineProps<{
  show: boolean
  geminiApiKeys: ApiKey[]
  loadingKeys: boolean
  availableBatchImageModels: Array<{ value: string; label: string }>
  loadingModels: boolean
  modelLoadError: string
  selectedApiKey: ApiKey | null
  selectedModelReferenceLimit: number
  estimatedOutputCount: number
  promptRows: PromptRow[]
  referenceImageDrafts: ReferenceImageDraft[]
  submitting: boolean
  outputCountOptions: number[]
  maxOutputsPerItem: number
  maxOutputsPerJob: number
  parsedItemsCount: number
  batchImageText: (key: BatchImageTextKey) => string
}>()

defineEmits<{
  close: []
  submit: []
  'add-prompt': []
  'remove-prompt': [index: number]
  'remove-reference': [index: number]
  'upload-reference': [event: Event]
}>()

const { t } = useI18n()
</script>

<style scoped>
.batch-output-count-select {
  height: 36px;
  min-height: 36px;
  padding-top: 0;
  padding-bottom: 0;
  padding-left: 14px;
  padding-right: 34px;
  line-height: 36px;
}
</style>
