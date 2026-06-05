<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.dataImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="import-data-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.dataImportHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-600 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{ t('admin.accounts.dataImportWarning') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.dataImportDedupMode') }}</label>
        <select v-model="dedupMode" class="input">
          <option value="none">{{ t('admin.accounts.dataImportDedupNone') }}</option>
          <option value="overwrite">{{ t('admin.accounts.dataImportDedupOverwrite') }}</option>
          <option value="ignore">{{ t('admin.accounts.dataImportDedupIgnore') }}</option>
        </select>
        <p class="input-hint">{{ t('admin.accounts.dataImportDedupHint') }}</p>
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.dataImportFile') }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200">
              {{ fileName || t('admin.accounts.dataImportSelectFile') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.accounts.dataImportFileHint') }}
            </div>
          </div>
          <button type="button" class="btn btn-secondary shrink-0" @click="openFilePicker">
            {{ t('common.chooseFile') }}
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          accept="application/json,.json,.zip,.cpa,application/zip"
          @change="handleFileChange"
        />
      </div>

      <!-- JSON import result -->
      <div
        v-if="jsonResult"
        class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.dataImportResult') }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.dataImportResultSummary', jsonResult) }}
        </div>

        <div v-if="jsonErrorItems.length" class="mt-2">
          <div class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t('admin.accounts.dataImportErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
          >
            <div v-for="(item, idx) in jsonErrorItems" :key="idx" class="whitespace-pre-wrap">
              {{ item.kind }} {{ item.name || item.proxy_key || t('common.notAvailable') }} — {{ item.message }}
            </div>
          </div>
        </div>
      </div>

      <!-- Archive import result -->
      <div
        v-if="archiveResult"
        class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.dataImportResult') }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.dataImportArchiveSummary', { format: archiveResult.format, total: archiveResult.total_entries, codex: archiveResult.codex_entries, sub2api: archiveResult.sub2api_entries, unknown: archiveResult.unknown_entries }) }}
        </div>

        <!-- sub2api result -->
        <div v-if="archiveResult.sub2api_result" class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.dataImportResultSummary', archiveResult.sub2api_result) }}
        </div>

        <!-- codex result -->
        <div v-if="archiveResult.codex_result" class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.dataImportArchiveCodexSummary', archiveResult.codex_result) }}
        </div>

        <!-- parse errors -->
        <div v-if="archiveResult.parse_errors?.length" class="mt-2">
          <div class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t('admin.accounts.dataImportArchiveParseErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
          >
            <div v-for="(item, idx) in archiveResult.parse_errors" :key="idx" class="whitespace-pre-wrap">
              {{ item.entry }} — {{ item.message }}
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="importing" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="import-data-form"
          :disabled="importing"
        >
          {{ importing ? t('admin.accounts.dataImporting') : t('admin.accounts.dataImportButton') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { AdminDataImportResult, ArchiveImportResult } from '@/types'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'imported'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()

const importing = ref(false)
const file = ref<File | null>(null)
const jsonResult = ref<AdminDataImportResult | null>(null)
const archiveResult = ref<ArchiveImportResult | null>(null)
const dedupMode = ref<'none' | 'overwrite' | 'ignore'>('none')

const fileInput = ref<HTMLInputElement | null>(null)
const fileName = computed(() => file.value?.name || '')

const jsonErrorItems = computed(() => jsonResult.value?.errors || [])

watch(
  () => props.show,
  (open) => {
    if (open) {
      file.value = null
      jsonResult.value = null
      archiveResult.value = null
      dedupMode.value = 'none'
      if (fileInput.value) {
        fileInput.value.value = ''
      }
    }
  }
)

const openFilePicker = () => {
  fileInput.value?.click()
}

const handleFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement
  file.value = target.files?.[0] || null
}

const handleClose = () => {
  if (importing.value) return
  emit('close')
}

const isArchiveFile = (f: File): boolean => {
  const name = f.name.toLowerCase()
  return name.endsWith('.zip') || name.endsWith('.cpa')
}

const readFileAsText = async (sourceFile: File): Promise<string> => {
  if (typeof sourceFile.text === 'function') {
    return sourceFile.text()
  }

  if (typeof sourceFile.arrayBuffer === 'function') {
    const buffer = await sourceFile.arrayBuffer()
    return new TextDecoder().decode(buffer)
  }

  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(reader.error || new Error(t('common.failedToReadFile')))
    reader.readAsText(sourceFile)
  })
}

const handleImport = async () => {
  if (!file.value) {
    appStore.showError(t('admin.accounts.dataImportSelectFile'))
    return
  }

  importing.value = true
  jsonResult.value = null
  archiveResult.value = null

  try {
    if (isArchiveFile(file.value)) {
      const res = await adminAPI.accounts.importArchive(file.value, {
        dedup_mode: dedupMode.value,
        skip_default_group_bind: true,
        update_existing: true
      })

      archiveResult.value = res

      const hasFailed =
        (res.codex_result?.failed ?? 0) > 0 ||
        (res.sub2api_result?.account_failed ?? 0) > 0 ||
        (res.parse_errors?.length ?? 0) > 0

      if (hasFailed) {
        appStore.showError(t('admin.accounts.dataImportCompletedWithErrors', {
          account_failed: (res.codex_result?.failed ?? 0) + (res.sub2api_result?.account_failed ?? 0),
          proxy_failed: res.sub2api_result?.proxy_failed ?? 0
        }))
      } else {
        const created = (res.codex_result?.created ?? 0) + (res.sub2api_result?.account_created ?? 0)
        appStore.showSuccess(t('admin.accounts.dataImportArchiveSuccess', { created, format: res.format }))
        emit('imported')
      }
    } else {
      const text = await readFileAsText(file.value)
      const dataPayload = JSON.parse(text)

      const res = await adminAPI.accounts.importData({
        data: dataPayload,
        skip_default_group_bind: true,
        dedup_mode: dedupMode.value
      })

      jsonResult.value = res

      const msgParams: Record<string, unknown> = {
        account_created: res.account_created,
        account_updated: res.account_updated ?? 0,
        account_skipped: res.account_skipped ?? 0,
        account_failed: res.account_failed,
        proxy_created: res.proxy_created,
        proxy_reused: res.proxy_reused,
        proxy_failed: res.proxy_failed,
      }
      if (res.account_failed > 0 || res.proxy_failed > 0) {
        appStore.showError(t('admin.accounts.dataImportCompletedWithErrors', msgParams))
      } else {
        appStore.showSuccess(t('admin.accounts.dataImportSuccess', msgParams))
        emit('imported')
      }
    }
  } catch (error: any) {
    if (error instanceof SyntaxError) {
      appStore.showError(t('admin.accounts.dataImportParseFailed'))
    } else {
      appStore.showError(error?.message || t('admin.accounts.dataImportFailed'))
    }
  } finally {
    importing.value = false
  }
}
</script>
