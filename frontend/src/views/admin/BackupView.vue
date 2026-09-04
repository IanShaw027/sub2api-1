<template>
    <div class="space-y-6">
      <!-- S3 Storage Config -->
      <SettingsSection>
        <template #header>
          <h2 class="ui-settings-section-title">
            {{ t('admin.backup.s3.title') }}
          </h2>
          <p class="ui-settings-section-description">
            {{ t('admin.backup.s3.descriptionPrefix') }}
            <button type="button" class="text-accent underline hover:text-accent" @click="showR2Guide = true">Cloudflare R2</button>
            {{ t('admin.backup.s3.descriptionSuffix') }}
          </p>
        </template>

        <SettingRow :label="t('admin.backup.s3.endpoint')">
          <input v-model="s3Form.endpoint" class="input" placeholder="https://<account_id>.r2.cloudflarestorage.com" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.s3.region')">
          <input v-model="s3Form.region" class="input" placeholder="auto" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.s3.bucket')">
          <input v-model="s3Form.bucket" class="input" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.s3.prefix')">
          <input v-model="s3Form.prefix" class="input" placeholder="backups/" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.s3.accessKeyId')">
          <input v-model="s3Form.access_key_id" class="input" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.s3.secretAccessKey')">
          <input v-model="s3Form.secret_access_key" type="password" class="input" :placeholder="s3SecretConfigured ? t('admin.backup.s3.secretConfigured') : ''" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.s3.forcePathStyle')">
          <Toggle v-model="s3Form.force_path_style" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.s3.mediaEnabled')">
          <Toggle v-model="mediaEnabled" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.s3.mediaPublicBaseUrl')">
          <input v-model="s3Form.media_public_base_url" class="input" :placeholder="t('admin.backup.s3.mediaPublicBaseUrlPlaceholder')" :disabled="!s3Form.media_enabled" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.s3.mediaPrefix')">
          <input v-model="s3Form.media_prefix" class="input" placeholder="media/" :disabled="!s3Form.media_enabled" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.s3.mediaSigningSecret')">
          <input
            v-model="s3Form.media_download_signing_secret"
            type="password"
            class="input"
            :placeholder="s3Form.media_download_signing_secret_configured ? t('admin.backup.s3.secretConfigured') : t('admin.backup.s3.mediaSigningSecretPlaceholder')"
            :disabled="!s3Form.media_enabled"
          />
        </SettingRow>

        <div class="settings-card-body">
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="testingS3" @click="testS3">
              {{ testingS3 ? t('common.loading') : t('admin.backup.s3.testConnection') }}
            </button>
            <button type="button" class="btn btn-primary btn-sm" :disabled="savingS3" @click="saveS3Config">
              {{ savingS3 ? t('common.loading') : t('common.save') }}
            </button>
          </div>
        </div>
      </SettingsSection>

      <!-- Async image object storage -->
      <SettingsSection
        :title="t('admin.backup.imageStorage.title')"
        :description="t('admin.backup.imageStorage.description')"
      >
        <SettingRow :label="t('admin.backup.imageStorage.enabled')">
          <Toggle v-model="imageStorageForm.enabled" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.imageStorage.reuseBackupS3')">
          <Toggle v-model="imageStorageForm.reuse_backup_s3" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.imageStorage.bucket')">
          <input v-model="imageStorageForm.bucket" class="input" :placeholder="imageStorageForm.reuse_backup_s3 ? t('admin.backup.imageStorage.bucketInherited') : ''" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.imageStorage.prefix')">
          <input v-model="imageStorageForm.prefix" class="input" placeholder="images/" />
        </SettingRow>

        <template v-if="!imageStorageForm.reuse_backup_s3">
          <SettingRow :label="t('admin.backup.s3.endpoint')">
            <input v-model="imageStorageForm.endpoint" class="input" placeholder="https://<account_id>.r2.cloudflarestorage.com" />
          </SettingRow>
          <SettingRow :label="t('admin.backup.s3.region')">
            <input v-model="imageStorageForm.region" class="input" placeholder="auto" />
          </SettingRow>
          <SettingRow :label="t('admin.backup.s3.accessKeyId')">
            <input v-model="imageStorageForm.access_key_id" class="input" />
          </SettingRow>
          <SettingRow :label="t('admin.backup.s3.secretAccessKey')">
            <input v-model="imageStorageForm.secret_access_key" type="password" class="input" :placeholder="imageStorageSecretConfigured ? t('admin.backup.s3.secretConfigured') : ''" />
          </SettingRow>
          <SettingRow :label="t('admin.backup.s3.forcePathStyle')">
            <Toggle v-model="imageStorageForm.force_path_style" />
          </SettingRow>
        </template>

        <SettingRow :label="t('admin.backup.imageStorage.publicBaseUrl')">
          <input v-model="imageStorageForm.public_base_url" class="input" :placeholder="t('admin.backup.imageStorage.publicBaseUrlPlaceholder')" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.imageStorage.presignExpiryHours')">
          <input v-model.number="imageStorageForm.presign_expiry_hours" type="number" min="1" class="input" />
        </SettingRow>

        <div class="settings-card-body">
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="testingImageStorage" @click="testImageStorage">
              {{ testingImageStorage ? t('common.loading') : t('admin.backup.s3.testConnection') }}
            </button>
            <button type="button" class="btn btn-primary btn-sm" :disabled="savingImageStorage" @click="saveImageStorageConfig">
              {{ savingImageStorage ? t('common.loading') : t('common.save') }}
            </button>
          </div>
        </div>
      </SettingsSection>

      <!-- Schedule Config -->
      <SettingsSection
        :title="t('admin.backup.schedule.title')"
        :description="t('admin.backup.schedule.description')"
      >
        <SettingRow :label="t('admin.backup.schedule.enabled')">
          <Toggle v-model="scheduleForm.enabled" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.schedule.cronExpr')" :description="t('admin.backup.schedule.cronHint')">
          <input v-model="scheduleForm.cron_expr" class="input" placeholder="0 2 * * *" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.schedule.retainDays')" :description="t('admin.backup.schedule.retainDaysHint')">
          <input v-model.number="scheduleForm.retain_days" type="number" min="0" class="input" />
        </SettingRow>
        <SettingRow :label="t('admin.backup.schedule.retainCount')" :description="t('admin.backup.schedule.retainCountHint')">
          <input v-model.number="scheduleForm.retain_count" type="number" min="0" class="input" />
        </SettingRow>

        <div class="settings-card-body">
          <button type="button" class="btn btn-primary btn-sm" :disabled="savingSchedule" @click="saveSchedule">
            {{ savingSchedule ? t('common.loading') : t('common.save') }}
          </button>
        </div>
      </SettingsSection>

      <!-- Backup Operations -->
      <SettingsSection
        :title="t('admin.backup.operations.title')"
        :description="t('admin.backup.operations.description')"
      >
        <div class="settings-card-body">
          <div class="mb-4 flex flex-wrap items-center justify-end gap-2">
            <div class="flex items-center gap-1">
              <label class="text-xs text-muted">{{ t('admin.backup.operations.expireDays') }}</label>
              <input v-model.number="manualExpireDays" type="number" min="0" class="input w-20 text-xs" />
            </div>
            <button type="button" class="btn btn-primary btn-sm" :disabled="creatingBackup" @click="createBackup">
              {{ creatingBackup ? t('admin.backup.operations.backing') : t('admin.backup.operations.createBackup') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="loadingBackups" @click="loadBackups">
              {{ loadingBackups ? t('common.loading') : t('common.refresh') }}
            </button>
          </div>

          <DataTable :columns="backupColumns" :data="backups" :loading="loadingBackups" row-key="id">
            <template #empty>
              <div class="empty-state">
                <Icon name="inbox" class="empty-state-icon" :stroke-width="1.6" aria-hidden="true" />
                <p class="empty-state-title">{{ t('admin.backup.empty') }}</p>
              </div>
            </template>
            <template #cell-id="{ value }">
              <span class="font-mono text-xs">{{ value }}</span>
            </template>
            <template #cell-status="{ row }">
              <span
                class="rounded px-2 py-0.5 text-xs"
                :class="statusClass(row.status)"
              >
                {{ row.status === 'running' && row.progress
                  ? t(`admin.backup.progress.${row.progress}`)
                  : t(`admin.backup.status.${row.status}`) }}
              </span>
            </template>
            <template #cell-parts="{ row }">
              {{ row.parts?.length || (row.status === 'running' ? '-' : 1) }}
            </template>
            <template #cell-expires_at="{ value }">
              {{ value ? formatDate(value) : t('admin.backup.neverExpire') }}
            </template>
            <template #cell-triggered_by="{ value }">
              {{ value === 'scheduled' ? t('admin.backup.trigger.scheduled') : t('admin.backup.trigger.manual') }}
            </template>
            <template #cell-started_at="{ value }">
              {{ formatDate(value) }}
            </template>
            <template #cell-actions="{ row }">
              <div class="flex flex-wrap gap-1">
                <button
                  v-if="row.status === 'completed'"
                  type="button"
                  class="btn btn-secondary btn-xs"
                  @click="downloadBackup(row.id)"
                >
                  {{ t('admin.backup.actions.download') }}
                </button>
                <button
                  v-if="row.status === 'completed'"
                  type="button"
                  class="btn btn-secondary btn-xs"
                  :disabled="restoringId === row.id"
                  @click="promptRestoreBackup(row.id)"
                >
                  {{ restoringId === row.id ? t('common.loading') : t('admin.backup.actions.restore') }}
                </button>
                <button
                  v-if="row.status !== 'running'"
                  type="button"
                  class="btn btn-danger btn-xs"
                  @click="promptRemoveBackup(row.id)"
                >
                  {{ t('common.delete') }}
                </button>
              </div>
            </template>
          </DataTable>
        </div>
      </SettingsSection>
    </div>

    <!-- Cloudflare R2 Setup Guide Modal -->
    <teleport to="body">
      <transition name="modal">
        <div v-if="showR2Guide" class="fixed inset-0 z-50 flex items-center justify-center p-4" @mousedown.self="showR2Guide = false">
          <div class="fixed inset-0 glass-modal-scrim" @click="showR2Guide = false"></div>
          <div class="relative max-h-[85vh] w-full max-w-2xl overflow-y-auto glass-card-solid rounded-hero p-6">
            <button type="button" class="modal-close absolute right-4 top-4" @click="showR2Guide = false">
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>

            <h2 class="mb-4 text-lg font-bold text-foreground ">{{ t('admin.backup.r2Guide.title') }}</h2>
            <p class="mb-4 text-sm text-muted ">{{ t('admin.backup.r2Guide.intro') }}</p>

            <!-- Step 1 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground ">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-xs font-bold text-accent  ">1</span>
                {{ t('admin.backup.r2Guide.step1.title') }}
              </h3>
              <ol class="ml-8 list-decimal space-y-1 text-sm text-muted ">
                <li>{{ t('admin.backup.r2Guide.step1.line1') }}</li>
                <li>{{ t('admin.backup.r2Guide.step1.line2') }}</li>
                <li>{{ t('admin.backup.r2Guide.step1.line3') }}</li>
              </ol>
            </div>

            <!-- Step 2 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground ">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-xs font-bold text-accent  ">2</span>
                {{ t('admin.backup.r2Guide.step2.title') }}
              </h3>
              <ol class="ml-8 list-decimal space-y-1 text-sm text-muted ">
                <li>{{ t('admin.backup.r2Guide.step2.line1') }}</li>
                <li>{{ t('admin.backup.r2Guide.step2.line2') }}</li>
                <li>{{ t('admin.backup.r2Guide.step2.line3') }}</li>
                <li>{{ t('admin.backup.r2Guide.step2.line4') }}</li>
              </ol>
              <div class="mt-2 rounded-lg bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] p-3 text-xs text-warning-text  ">
                {{ t('admin.backup.r2Guide.step2.warning') }}
              </div>
            </div>

            <!-- Step 3 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground ">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-xs font-bold text-accent  ">3</span>
                {{ t('admin.backup.r2Guide.step3.title') }}
              </h3>
              <p class="ml-8 text-sm text-muted ">{{ t('admin.backup.r2Guide.step3.desc') }}</p>
              <code class="ml-8 mt-1 block rounded bg-surface-2 px-3 py-2 text-xs text-foreground  ">https://&lt;{{ t('admin.backup.r2Guide.step3.accountId') }}&gt;.r2.cloudflarestorage.com</code>
            </div>

            <!-- Step 4: Fill form -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground ">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-xs font-bold text-accent  ">4</span>
                {{ t('admin.backup.r2Guide.step4.title') }}
              </h3>
              <div class="ml-8 overflow-hidden rounded-lg border border-line ">
                <table class="w-full text-sm">
                  <tbody>
                    <tr v-for="(row, i) in r2ConfigRows" :key="i" class="border-b border-line  last:border-0">
                      <td class="whitespace-nowrap bg-surface-2 px-3 py-2 font-medium text-foreground  ">{{ row.field }}</td>
                      <td class="px-3 py-2 text-muted "><code class="text-xs">{{ row.value }}</code></td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Free tier note -->
            <div class="rounded-lg bg-[color-mix(in_oklch,var(--success)_16%,transparent)] p-3 text-xs text-success-text  ">
              {{ t('admin.backup.r2Guide.freeTier') }}
            </div>

            <div class="mt-4 text-right">
              <button type="button" class="btn btn-primary btn-sm" @click="showR2Guide = false">{{ t('common.close') }}</button>
            </div>
          </div>
        </div>
      </transition>
    </teleport>
    <!-- 分卷下载链接 -->
    <teleport to="body">
      <transition name="modal">
        <div
          v-if="downloadPartsModalOpen"
          class="fixed inset-0 z-50 flex items-center justify-center p-4"
          @mousedown.self="closeDownloadParts"
        >
          <div class="fixed inset-0 glass-modal-scrim" @click="closeDownloadParts"></div>
          <div class="relative max-h-[85vh] w-full max-w-lg overflow-y-auto glass-card-solid rounded-hero p-6">
            <button
              type="button"
              class="modal-close absolute right-4 top-4"
              :aria-label="t('common.close')"
              @click="closeDownloadParts"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
            <h2 class="mb-1 text-lg font-bold text-foreground ">{{ t('admin.backup.actions.downloadParts') }}</h2>
            <p class="mb-4 text-sm text-muted ">{{ t('admin.backup.actions.downloadPartsHint') }}</p>
            <div class="space-y-2">
              <div
                v-for="part in downloadParts"
                :key="part.index"
                class="flex items-center justify-between gap-3 rounded-lg border border-line px-3 py-2 "
              >
                <span class="text-sm text-foreground ">
                  {{ t('admin.backup.actions.partLabel', { index: part.index }) }}
                  <span class="ml-2 text-xs text-muted ">{{ formatSize(part.size_bytes) }}</span>
                </span>
                <a :href="part.url" class="btn btn-secondary btn-xs" rel="noopener">
                  {{ t('admin.backup.actions.download') }}
                </a>
              </div>
            </div>
            <div class="mt-4 text-right">
              <button type="button" class="btn btn-primary btn-sm" @click="closeDownloadParts">{{ t('common.close') }}</button>
            </div>
          </div>
        </div>
      </transition>
    </teleport>

    <!-- Restore confirmation (password required) -->
    <ConfirmDialog
      :show="restoreConfirmOpen"
      :title="t('admin.backup.actions.restore')"
      :message="t('admin.backup.actions.restoreConfirm')"
      tone="danger"
      :confirm-text="t('admin.backup.actions.restore')"
      :confirming="restoreConfirming"
      @confirm="confirmRestoreBackup"
      @cancel="cancelRestoreConfirm"
    >
      <label class="input-label">{{ t('admin.backup.actions.restorePasswordPrompt') }}</label>
      <input
        v-model="restorePassword"
        type="password"
        class="input w-full"
        autofocus
        @keydown.enter="confirmRestoreBackup"
      />
    </ConfirmDialog>

    <!-- Delete confirmation -->
    <ConfirmDialog
      :show="deleteConfirmOpen"
      :title="t('common.delete')"
      :message="t('admin.backup.actions.deleteConfirm')"
      tone="danger"
      :confirm-text="t('common.delete')"
      :confirming="deleteConfirming"
      @confirm="confirmRemoveBackup"
      @cancel="cancelDeleteConfirm"
    />

    <TotpStepUpDialog :controller="backupStepUp" />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import { useAppStore } from '@/stores'
import type {
  BackupS3Config,
  BackupScheduleConfig,
  BackupRecord,
  BackupDownloadPart,
  ImageStorageConfig,
} from '@/api/admin/backup'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import SettingsSection from '@/components/ui/SettingsSection.vue'
import SettingRow from '@/components/ui/SettingRow.vue'
import Toggle from '@/components/common/Toggle.vue'
import DataTable from '@/components/common/DataTable.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'

const { t } = useI18n()
const appStore = useAppStore()
const backupStepUp = useStepUp()

// 敏感操作被 2FA 门控拦截时的统一提示。
function reportStepUpBlocked(error: unknown): boolean {
  if (!isStepUpBlocked(error)) return false
  appStore.showError(
    stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
      ? t('stepUp.adminApiKeyForbidden')
      : t('stepUp.notEnabled')
  )
  return true
}

// S3 config
const s3Form = ref<BackupS3Config>({
  endpoint: '',
  region: 'auto',
  bucket: '',
  access_key_id: '',
  secret_access_key: '',
  prefix: 'backups/',
  force_path_style: false,
  media_enabled: true,
  media_public_base_url: '',
  media_prefix: 'media/',
  media_download_signing_secret: '',
  media_download_signing_secret_configured: false,
})
const s3SecretConfigured = ref(false)
const savingS3 = ref(false)
const testingS3 = ref(false)
// `media_enabled` is optional on the wire (may be omitted by older configs); Toggle
// requires a non-optional boolean v-model, so normalize through a computed proxy.
const mediaEnabled = computed<boolean>({
  get: () => s3Form.value.media_enabled !== false,
  set: (value) => { s3Form.value.media_enabled = value },
})

// Async image object storage. Shares the S3 client with backups, so the default is
// to reuse the credentials configured above and only differ by prefix.
const imageStorageForm = ref<ImageStorageConfig>({
  enabled: false,
  reuse_backup_s3: true,
  bucket: '',
  prefix: 'images/',
  public_base_url: '',
  presign_expiry_hours: 24,
  max_download_bytes: 33554432,
  endpoint: '',
  region: 'auto',
  access_key_id: '',
  secret_access_key: '',
  force_path_style: false,
})
const imageStorageSecretConfigured = ref(false)
const savingImageStorage = ref(false)
const testingImageStorage = ref(false)

// Schedule config
const scheduleForm = ref<BackupScheduleConfig>({
  enabled: false,
  cron_expr: '0 2 * * *',
  retain_days: 14,
  retain_count: 10,
})
const savingSchedule = ref(false)

// Backups
const backups = ref<BackupRecord[]>([])
const loadingBackups = ref(false)
const creatingBackup = ref(false)
const restoringId = ref('')
const manualExpireDays = ref(14)
const downloadParts = ref<BackupDownloadPart[]>([])
const downloadPartsModalOpen = ref(false)

const backupColumns = computed<Column[]>(() => [
  { key: 'id', label: 'ID' },
  { key: 'status', label: t('admin.backup.columns.status') },
  { key: 'file_name', label: t('admin.backup.columns.fileName') },
  { key: 'size_bytes', label: t('admin.backup.columns.size'), formatter: (value: number) => formatSize(value) },
  { key: 'parts', label: t('admin.backup.columns.parts') },
  { key: 'expires_at', label: t('admin.backup.columns.expiresAt') },
  { key: 'triggered_by', label: t('admin.backup.columns.triggeredBy') },
  { key: 'started_at', label: t('admin.backup.columns.startedAt') },
  { key: 'actions', label: t('admin.backup.columns.actions') },
])

// Restore confirmation (glass ConfirmDialog replaces window.confirm + window.prompt)
const restoreConfirmOpen = ref(false)
const restorePassword = ref('')
const pendingRestoreId = ref('')
const restoreConfirming = ref(false)

function promptRestoreBackup(id: string) {
  pendingRestoreId.value = id
  restorePassword.value = ''
  restoreConfirmOpen.value = true
}

function cancelRestoreConfirm() {
  restoreConfirmOpen.value = false
  pendingRestoreId.value = ''
  restorePassword.value = ''
}

// Delete confirmation (glass ConfirmDialog replaces window.confirm)
const deleteConfirmOpen = ref(false)
const pendingDeleteId = ref('')
const deleteConfirming = ref(false)

function promptRemoveBackup(id: string) {
  pendingDeleteId.value = id
  deleteConfirmOpen.value = true
}

function cancelDeleteConfirm() {
  deleteConfirmOpen.value = false
  pendingDeleteId.value = ''
}

// Polling
const pollingTimer = ref<ReturnType<typeof setInterval> | null>(null)
const restoringPollingTimer = ref<ReturnType<typeof setInterval> | null>(null)
const MAX_POLL_COUNT = 900

function updateRecordInList(updated: BackupRecord) {
  const idx = backups.value.findIndex(r => r.id === updated.id)
  if (idx >= 0) {
    backups.value[idx] = updated
  }
}

function startPolling(backupId: string) {
  stopPolling()
  let count = 0
  pollingTimer.value = setInterval(async () => {
    if (count++ >= MAX_POLL_COUNT) {
      stopPolling()
      creatingBackup.value = false
      appStore.showWarning(t('admin.backup.operations.backupRunning'))
      return
    }
    try {
      const record = await adminAPI.backup.getBackup(backupId)
      updateRecordInList(record)
      if (record.status === 'completed' || record.status === 'failed') {
        stopPolling()
        creatingBackup.value = false
        if (record.status === 'completed') {
          appStore.showSuccess(t('admin.backup.operations.backupCreated'))
        } else {
          appStore.showError(record.error_message || t('admin.backup.operations.backupFailed'))
        }
        await loadBackups()
      }
    } catch {
      // 轮询失败时不中断
    }
  }, 2000)
}

function stopPolling() {
  if (pollingTimer.value) {
    clearInterval(pollingTimer.value)
    pollingTimer.value = null
  }
}

function startRestorePolling(backupId: string) {
  stopRestorePolling()
  let count = 0
  restoringPollingTimer.value = setInterval(async () => {
    if (count++ >= MAX_POLL_COUNT) {
      stopRestorePolling()
      restoringId.value = ''
      appStore.showWarning(t('admin.backup.operations.restoreRunning'))
      return
    }
    try {
      const record = await adminAPI.backup.getBackup(backupId)
      updateRecordInList(record)
      if (record.restore_status === 'completed' || record.restore_status === 'failed') {
        stopRestorePolling()
        restoringId.value = ''
        if (record.restore_status === 'completed') {
          appStore.showSuccess(t('admin.backup.actions.restoreSuccess'))
        } else {
          appStore.showError(record.restore_error || t('admin.backup.operations.restoreFailed'))
        }
        await loadBackups()
      }
    } catch {
      // 轮询失败时不中断
    }
  }, 2000)
}

function stopRestorePolling() {
  if (restoringPollingTimer.value) {
    clearInterval(restoringPollingTimer.value)
    restoringPollingTimer.value = null
  }
}

function handleVisibilityChange() {
  if (document.hidden) {
    stopPolling()
    stopRestorePolling()
  } else {
    // 标签页恢复时刷新列表，检查是否仍有活跃操作
    loadBackups().then(() => {
      const running = backups.value.find(r => r.status === 'running')
      if (running) {
        creatingBackup.value = true
        startPolling(running.id)
      }
      const restoring = backups.value.find(r => r.restore_status === 'running')
      if (restoring) {
        restoringId.value = restoring.id
        startRestorePolling(restoring.id)
      }
    })
  }
}

// R2 guide
const showR2Guide = ref(false)
const r2ConfigRows = computed(() => [
  { field: t('admin.backup.s3.endpoint'), value: 'https://<account_id>.r2.cloudflarestorage.com' },
  { field: t('admin.backup.s3.region'), value: 'auto' },
  { field: t('admin.backup.s3.bucket'), value: t('admin.backup.r2Guide.step4.bucketValue') },
  { field: t('admin.backup.s3.prefix'), value: 'backups/' },
  { field: 'Access Key ID', value: t('admin.backup.r2Guide.step4.fromStep2') },
  { field: 'Secret Access Key', value: t('admin.backup.r2Guide.step4.fromStep2') },
  { field: t('admin.backup.s3.forcePathStyle'), value: t('admin.backup.r2Guide.step4.unchecked') },
])

async function loadS3Config() {
  try {
    const cfg = await adminAPI.backup.getS3Config()
    s3Form.value = {
      endpoint: cfg.endpoint || '',
      region: cfg.region || 'auto',
      bucket: cfg.bucket || '',
      access_key_id: cfg.access_key_id || '',
      secret_access_key: '',
      prefix: cfg.prefix || 'backups/',
      force_path_style: Boolean(cfg.force_path_style),
      media_enabled: cfg.media_enabled !== false,
      media_public_base_url: cfg.media_public_base_url || '',
      media_prefix: cfg.media_prefix || 'media/',
      media_download_signing_secret: '',
      media_download_signing_secret_configured: Boolean(cfg.media_download_signing_secret_configured),
    }
    s3SecretConfigured.value = Boolean(cfg.access_key_id)
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

async function saveS3Config() {
  savingS3.value = true
  try {
    await backupStepUp.run(() => adminAPI.backup.updateS3Config(s3Form.value))
    appStore.showSuccess(t('admin.backup.s3.saved'))
    await loadS3Config()
  } catch (error) {
    if (isStepUpCancelled(error)) {
      savingS3.value = false
      return
    }
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    savingS3.value = false
  }
}

async function loadImageStorageConfig() {
  try {
    const { config, secret_configured } = await adminAPI.backup.getImageStorageConfig()
    imageStorageForm.value = {
      ...config,
      prefix: config.prefix || 'images/',
      region: config.region || 'auto',
      secret_access_key: '',
      enabled: Boolean(config.enabled),
      reuse_backup_s3: Boolean(config.reuse_backup_s3),
      force_path_style: Boolean(config.force_path_style),
    }
    imageStorageSecretConfigured.value = secret_configured
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

async function saveImageStorageConfig() {
  savingImageStorage.value = true
  try {
    await backupStepUp.run(() => adminAPI.backup.updateImageStorageConfig(imageStorageForm.value))
    appStore.showSuccess(t('admin.backup.imageStorage.saved'))
    await loadImageStorageConfig()
  } catch (error) {
    if (isStepUpCancelled(error)) {
      savingImageStorage.value = false
      return
    }
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    savingImageStorage.value = false
  }
}

async function testImageStorage() {
  testingImageStorage.value = true
  try {
    const result = await adminAPI.backup.testImageStorageConnection(imageStorageForm.value)
    if (result.ok) {
      appStore.showSuccess(result.message || t('admin.backup.s3.testSuccess'))
    } else {
      appStore.showError(result.message || t('admin.backup.s3.testFailed'))
    }
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    testingImageStorage.value = false
  }
}

async function testS3() {
  testingS3.value = true
  try {
    const result = await adminAPI.backup.testS3Connection(s3Form.value)
    if (result.ok) {
      appStore.showSuccess(result.message || t('admin.backup.s3.testSuccess'))
    } else {
      appStore.showError(result.message || t('admin.backup.s3.testFailed'))
    }
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    testingS3.value = false
  }
}

async function loadSchedule() {
  try {
    const cfg = await adminAPI.backup.getSchedule()
    scheduleForm.value = {
      enabled: cfg.enabled,
      cron_expr: cfg.cron_expr || '0 2 * * *',
      retain_days: cfg.retain_days || 14,
      retain_count: cfg.retain_count || 10,
    }
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

async function saveSchedule() {
  savingSchedule.value = true
  try {
    await adminAPI.backup.updateSchedule(scheduleForm.value)
    appStore.showSuccess(t('admin.backup.schedule.saved'))
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    savingSchedule.value = false
  }
}

async function loadBackups() {
  loadingBackups.value = true
  try {
    const result = await adminAPI.backup.listBackups()
    backups.value = result.items || []
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    loadingBackups.value = false
  }
}

async function createBackup() {
  creatingBackup.value = true
  try {
    const record = await backupStepUp.run(() => adminAPI.backup.createBackup({ expire_days: manualExpireDays.value }))
    // 插入到列表顶部
    backups.value.unshift(record)
    startPolling(record.id)
  } catch (error: any) {
    if (isStepUpCancelled(error)) {
      creatingBackup.value = false
      return
    }
    if (reportStepUpBlocked(error)) {
      creatingBackup.value = false
      return
    }
    if (error?.response?.status === 409) {
      appStore.showWarning(t('admin.backup.operations.alreadyInProgress'))
    } else {
      appStore.showError(error?.message || t('errors.networkError'))
    }
    creatingBackup.value = false
  }
}

async function downloadBackup(id: string) {
  try {
    const result = await backupStepUp.run(() => adminAPI.backup.getDownloadURL(id))
    if (result.parts && result.parts.length > 0) {
      downloadParts.value = result.parts
      downloadPartsModalOpen.value = true
      return
    }
    if (!result.url) {
      throw new Error(t('admin.backup.actions.downloadFailed'))
    }
    // 预签名 URL 带 attachment disposition，同页 anchor 导航直接触发下载；
    // 不用 window.open：step-up 弹窗 await 会耗尽瞬态用户激活，新标签页会被浏览器拦截。
    const link = document.createElement('a')
    link.href = result.url
    link.rel = 'noopener'
    link.click()
  } catch (error) {
    if (isStepUpCancelled(error)) return
    if (reportStepUpBlocked(error)) return
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

function closeDownloadParts() {
  downloadPartsModalOpen.value = false
  downloadParts.value = []
}

async function confirmRestoreBackup() {
  if (!restorePassword.value) return
  const id = pendingRestoreId.value
  const password = restorePassword.value
  restoreConfirming.value = true
  try {
    const record = await backupStepUp.run(() => adminAPI.backup.restoreBackup(id, password))
    updateRecordInList(record)
    restoreConfirmOpen.value = false
    restoringId.value = id
    startRestorePolling(id)
  } catch (error: any) {
    if (isStepUpCancelled(error)) return
    if (reportStepUpBlocked(error)) return
    // apiClient 拦截器把 HTTP 错误归一化为顶层 { status } 平面对象（无 response 字段）
    if (error?.status === 409 || error?.response?.status === 409) {
      appStore.showWarning(t('admin.backup.operations.restoreRunning'))
    } else {
      appStore.showError(error?.message || t('errors.networkError'))
    }
  } finally {
    restoreConfirming.value = false
    restorePassword.value = ''
    pendingRestoreId.value = ''
  }
}

async function confirmRemoveBackup() {
  const id = pendingDeleteId.value
  deleteConfirming.value = true
  try {
    await backupStepUp.run(() => adminAPI.backup.deleteBackup(id))
    appStore.showSuccess(t('admin.backup.actions.deleted'))
    deleteConfirmOpen.value = false
    pendingDeleteId.value = ''
    await loadBackups()
  } catch (error) {
    if (isStepUpCancelled(error)) return
    if (reportStepUpBlocked(error)) return
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    deleteConfirming.value = false
  }
}

function statusClass(status: string): string {
  switch (status) {
    case 'completed':
      return 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text  '
    case 'running':
      return 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent  '
    case 'failed':
      return 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text  '
    default:
      return 'bg-surface-2 text-foreground  '
  }
}

function formatSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatDate(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

onMounted(async () => {
  document.addEventListener('visibilitychange', handleVisibilityChange)
  await Promise.all([loadS3Config(), loadImageStorageConfig(), loadSchedule(), loadBackups()])

  // 如果有正在 running 的备份，恢复轮询
  const runningBackup = backups.value.find(r => r.status === 'running')
  if (runningBackup) {
    creatingBackup.value = true
    startPolling(runningBackup.id)
  }
  const restoringBackup = backups.value.find(r => r.restore_status === 'running')
  if (restoringBackup) {
    restoringId.value = restoringBackup.id
    startRestorePolling(restoringBackup.id)
  }
})

onBeforeUnmount(() => {
  stopPolling()
  stopRestorePolling()
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
