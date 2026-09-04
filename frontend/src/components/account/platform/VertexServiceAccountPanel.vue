<template>
  <div class="space-y-4">
    <div>
      <label class="input-label">Service Account JSON</label>
      <input
        ref="fileInputRef"
        type="file"
        accept="application/json,.json"
        class="hidden"
        @change="handleFileChange"
      />
      <div
        :class="[
          'rounded-lg border-2 border-dashed px-4 py-5 transition-colors',
          dragActive
            ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_10%,transparent)]'
            : 'border-line bg-surface-2 hover:border-[color-mix(in_oklch,var(--accent)_45%,transparent)] hover:bg-[color-mix(in_oklch,var(--accent)_10%,transparent)]'
        ]"
        @dragenter.prevent="dragActive = true"
        @dragover.prevent="dragActive = true"
        @dragleave.prevent="dragActive = false"
        @drop.prevent="handleDrop"
      >
        <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="flex items-center gap-2 text-sm font-medium text-foreground">
              <Icon name="upload" size="sm" />
              <span>{{ clientEmail ? t('admin.accounts.vertexSaJsonLoaded') : t('admin.accounts.vertexSaJsonDrop') }}</span>
            </div>
            <p class="mt-1 text-xs text-muted">
              {{ clientEmail ? t('admin.accounts.vertexSaJsonKeyHidden') : t('admin.accounts.vertexSaJsonDropHint') }}
            </p>
          </div>
          <button
            type="button"
            class="btn btn-secondary shrink-0"
            @click="fileInputRef?.click()"
          >
            <Icon name="upload" size="sm" />
            {{ t('admin.accounts.vertexSaJsonSelectBtn') }}
          </button>
        </div>
        <div
          v-if="clientEmail"
          class="mt-3 rounded-md border border-[color-mix(in_oklch,var(--accent)_35%,transparent)] bg-surface px-3 py-2 text-xs text-accent"
        >
          <div class="truncate">Project ID: <span class="font-mono">{{ projectId }}</span></div>
          <div class="truncate">Client Email: <span class="font-mono">{{ clientEmail }}</span></div>
        </div>
      </div>
      <p class="input-hint">{{ t('admin.accounts.vertexSaJsonUploadHint') }}</p>
    </div>

    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <div>
        <label class="input-label">Project ID</label>
        <input
          :value="projectId"
          type="text"
          class="input font-mono"
          readonly
          :placeholder="t('admin.accounts.vertexProjectIdPlaceholder')"
        />
      </div>
      <div>
        <label class="input-label">Location</label>
        <select
          v-model="location"
          required
          class="input font-mono"
        >
          <optgroup
            v-for="group in VERTEX_LOCATION_OPTIONS"
            :key="group.label"
            :label="group.label"
          >
            <option
              v-for="option in group.options"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </optgroup>
        </select>
        <p class="input-hint">{{ t('admin.accounts.vertexLocationHint') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { VERTEX_LOCATION_OPTIONS } from '@/constants/account'

defineProps<{
  projectId: string
  clientEmail: string
}>()

const location = defineModel<string>('location', { required: true })

const emit = defineEmits<{
  'file-text-received': [text: string]
}>()

const { t } = useI18n()

const fileInputRef = ref<HTMLInputElement | null>(null)
const dragActive = ref(false)

const handleFileChange = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    emit('file-text-received', await file.text())
  } finally {
    input.value = ''
  }
}

const handleDrop = async (event: DragEvent) => {
  dragActive.value = false
  const file = event.dataTransfer?.files?.[0]
  if (!file) return
  emit('file-text-received', await file.text())
}
</script>
