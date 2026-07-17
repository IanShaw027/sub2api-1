<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.dataImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="import-data-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-ink-soft dark:text-dark-300">
        {{ t('admin.accounts.dataImportHint') }}
      </div>
      <div
        class="rounded-control border border-warning/25 bg-warning-soft p-3 text-xs text-warning dark:border-warning/30 dark:bg-warning/20 dark:text-warning"
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
          class="flex items-center justify-between gap-3 rounded-control border border-dashed px-4 py-3 transition-colors"
          :class="dragActive
            ? 'border-brand-400 bg-brand-50/70 dark:border-brand-500 dark:bg-brand-900/20'
            : 'border-line bg-page dark:border-dark-600 dark:bg-dark-800'"
          @dragenter.prevent="handleDragEnter"
          @dragover.prevent
          @dragleave.prevent="handleDragLeave"
          @drop.prevent="handleDrop"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-ink-body dark:text-dark-200" :title="fileListTitle">
              {{ selectedFilesLabel || t('admin.accounts.dataImportSelectFile') }}
            </div>
            <div class="text-xs text-ink-soft dark:text-dark-400">
              {{ t('admin.accounts.dataImportFileHint') }}
              <span v-if="files.length > 1"> · {{ fileListTitle }}</span>
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
          multiple
          @change="handleFileChange"
        />
      </div>

      <!-- JSON import result -->
      <div
        v-if="jsonResult"
        class="space-y-2 rounded-card border border-line p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-ink dark:text-white">
          {{ t('admin.accounts.dataImportResult') }}
        </div>
        <div class="text-sm text-ink-body dark:text-dark-300">
          {{ t('admin.accounts.dataImportResultSummary', jsonResult) }}
        </div>

        <div v-if="jsonErrorItems.length" class="mt-2">
          <div class="text-sm font-medium text-danger dark:text-danger">
            {{ t('admin.accounts.dataImportErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-control bg-page p-3 font-mono text-xs dark:bg-dark-800"
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
        class="space-y-2 rounded-card border border-line p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-ink dark:text-white">
          {{ t('admin.accounts.dataImportResult') }}
        </div>
        <div class="text-sm text-ink-body dark:text-dark-300">
          {{ t('admin.accounts.dataImportArchiveSummary', { format: archiveResult.format, total: archiveResult.total_entries, codex: archiveResult.codex_entries, sub2api: archiveResult.sub2api_entries, unknown: archiveResult.unknown_entries }) }}
        </div>

        <!-- sub2api result -->
        <div v-if="archiveResult.sub2api_result" class="text-sm text-ink-body dark:text-dark-300">
          {{ t('admin.accounts.dataImportResultSummary', archiveResult.sub2api_result) }}
        </div>

        <!-- codex result -->
        <div v-if="archiveResult.codex_result" class="text-sm text-ink-body dark:text-dark-300">
          {{ t('admin.accounts.dataImportArchiveCodexSummary', archiveResult.codex_result) }}
        </div>

        <!-- parse errors -->
        <div v-if="archiveResult.parse_errors?.length" class="mt-2">
          <div class="text-sm font-medium text-danger dark:text-danger">
            {{ t('admin.accounts.dataImportArchiveParseErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-control bg-page p-3 font-mono text-xs dark:bg-dark-800"
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
import type { AdminDataImportPayload, AdminDataImportResult, AdminDataPayload, ArchiveImportResult } from '@/types'

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
const files = ref<File[]>([])
const dragDepth = ref(0)
const dragActive = computed(() => dragDepth.value > 0)
const hasImportedChanges = ref(false)
const jsonResult = ref<AdminDataImportResult | null>(null)
const archiveResult = ref<ArchiveImportResult | null>(null)
const dedupMode = ref<'none' | 'overwrite' | 'ignore'>('none')

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFilesLabel = computed(() => {
  if (files.value.length === 0) return ''
  if (files.value.length === 1) return files.value[0]?.name || ''
  return t('admin.accounts.selectedCount', { count: files.value.length })
})
const fileListTitle = computed(() => files.value.map((item) => item.name).join(', '))

const jsonErrorItems = computed(() => jsonResult.value?.errors || [])

watch(
  () => props.show,
  (open) => {
    if (open) {
      files.value = []
      dragDepth.value = 0
      hasImportedChanges.value = false
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
  setSelectedFiles(target.files)
  target.value = ''
}

const handleClose = () => {
  if (importing.value) return
  if (hasImportedChanges.value) {
    hasImportedChanges.value = false
    emit('imported')
  }
  emit('close')
}

const isArchiveFile = (f: File): boolean => {
  const name = f.name.toLowerCase()
  return name.endsWith('.zip') || name.endsWith('.cpa')
}

const isJsonFile = (sourceFile: File): boolean => {
  const name = sourceFile.name.toLowerCase()
  return name.endsWith('.json') || sourceFile.type === 'application/json'
}

const setSelectedFiles = (sourceFiles: FileList | File[] | null | undefined) => {
  if (importing.value) return
  const incoming = Array.from(sourceFiles || [])
  const picked = incoming.filter((item) => isJsonFile(item) || isArchiveFile(item))
  if (!picked.length) {
    appStore.showError(t('admin.accounts.dataImportSelectFile'))
    return
  }
  if (picked.length < incoming.length) {
    appStore.showWarning(
      t('admin.accounts.dataImportIgnoredFiles', { count: incoming.length - picked.length })
    )
  }
  if (picked.some(isArchiveFile) && (picked.length !== 1 || !isArchiveFile(picked[0]!))) {
    appStore.showError(t('admin.accounts.dataImportArchiveSingleFile'))
    return
  }
  files.value = picked
  jsonResult.value = null
  archiveResult.value = null
}

const handleDragEnter = () => {
  if (importing.value) return
  dragDepth.value += 1
}

const handleDragLeave = () => {
  dragDepth.value = Math.max(0, dragDepth.value - 1)
}

const handleDrop = (event: DragEvent) => {
  dragDepth.value = 0
  if (importing.value) return
  setSelectedFiles(event.dataTransfer?.files)
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

const SUPPORTED_DATA_TYPES = ['sub2api-data', 'sub2api-bundle']
const SUPPORTED_DATA_VERSION = 1

const isValidDataPayload = (payload: unknown): payload is AdminDataImportPayload => {
  if (Array.isArray(payload)) {
    return payload.every((item) => Boolean(item) && typeof item === 'object' && !Array.isArray(item))
  }
  if (!payload || typeof payload !== 'object') return false
  const candidate = payload as Record<string, unknown>
  if (
    candidate.type !== undefined &&
    candidate.type !== '' &&
    !SUPPORTED_DATA_TYPES.includes(candidate.type as string)
  ) {
    return false
  }
  if (
    candidate.version !== undefined &&
    candidate.version !== 0 &&
    candidate.version !== SUPPORTED_DATA_VERSION
  ) {
    return false
  }
  return Array.isArray(candidate.proxies) && Array.isArray(candidate.accounts)
}

const mergeDataPayloads = (payloads: AdminDataImportPayload[]): AdminDataImportPayload | null => {
  const [firstPayload] = payloads
  if (payloads.length === 1 && firstPayload) return firstPayload

  const arrayPayloads = payloads.filter(Array.isArray)
  if (arrayPayloads.length === payloads.length) {
    return arrayPayloads.flatMap((item) => item)
  }
  if (arrayPayloads.length > 0) return null

  const objectPayloads = payloads as AdminDataPayload[]
  return {
    type: objectPayloads.find((item) => typeof item.type === 'string')?.type,
    version: objectPayloads.find((item) => typeof item.version === 'number')?.version,
    exported_at: new Date().toISOString(),
    proxies: objectPayloads.flatMap((item) => item.proxies),
    accounts: objectPayloads.flatMap((item) => item.accounts),
    skipped_shadows: objectPayloads.reduce((sum, item) => {
      const count = Number(item.skipped_shadows || 0)
      return Number.isFinite(count) ? sum + count : sum
    }, 0)
  }
}

const handleImport = async () => {
  if (files.value.length === 0) {
    appStore.showError(t('admin.accounts.dataImportSelectFile'))
    return
  }

  importing.value = true
  jsonResult.value = null
  archiveResult.value = null

  try {
    const archiveFile = files.value.length === 1 && files.value[0] && isArchiveFile(files.value[0])
      ? files.value[0]
      : null
    if (archiveFile) {
      const res = await adminAPI.accounts.importArchive(archiveFile, {
        dedup_mode: dedupMode.value,
        skip_default_group_bind: true,
        update_existing: true
      })

      archiveResult.value = res

      const hasFailed =
        (res.codex_result?.failed ?? 0) > 0 ||
        (res.sub2api_result?.account_failed ?? 0) > 0 ||
        (res.sub2api_result?.proxy_failed ?? 0) > 0 ||
        (res.parse_errors?.length ?? 0) > 0

      if (hasFailed) {
        const mutated =
          (res.codex_result?.created ?? 0) +
          (res.codex_result?.updated ?? 0) +
          (res.sub2api_result?.account_created ?? 0) +
          (res.sub2api_result?.account_updated ?? 0) +
          (res.sub2api_result?.proxy_created ?? 0) +
          (res.sub2api_result?.proxy_reused ?? 0)
        if (mutated > 0) hasImportedChanges.value = true
        appStore.showError(t('admin.accounts.dataImportCompletedWithErrors', {
          account_failed: (res.codex_result?.failed ?? 0) + (res.sub2api_result?.account_failed ?? 0),
          proxy_failed: res.sub2api_result?.proxy_failed ?? 0
        }))
      } else {
        const created = (res.codex_result?.created ?? 0) + (res.sub2api_result?.account_created ?? 0)
        appStore.showSuccess(t('admin.accounts.dataImportArchiveSuccess', { created, format: res.format }))
        hasImportedChanges.value = false
        emit('imported')
      }
    } else {
      const dataPayloads: AdminDataImportPayload[] = []
      for (const sourceFile of files.value) {
        let parsed: unknown
        try {
          parsed = JSON.parse(await readFileAsText(sourceFile))
        } catch {
          appStore.showError(
            t('admin.accounts.dataImportParseFailedFile', { name: sourceFile.name })
          )
          return
        }
        if (!isValidDataPayload(parsed)) {
          appStore.showError(t('admin.accounts.dataImportInvalidFile', { name: sourceFile.name }))
          return
        }
        dataPayloads.push(parsed)
      }
      const dataPayload = mergeDataPayloads(dataPayloads)
      if (!dataPayload) {
        appStore.showError(t('admin.accounts.dataImportMixedFormats'))
        return
      }

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
        if (
          res.account_created > 0 ||
          (res.account_updated ?? 0) > 0 ||
          res.proxy_created > 0 ||
          res.proxy_reused > 0
        ) {
          hasImportedChanges.value = true
        }
        appStore.showError(t('admin.accounts.dataImportCompletedWithErrors', msgParams))
      } else {
        appStore.showSuccess(t('admin.accounts.dataImportSuccess', msgParams))
        hasImportedChanges.value = false
        emit('imported')
      }
    }
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.dataImportFailed'))
  } finally {
    importing.value = false
  }
}
</script>
