import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import adminComplianceAPI, { type AdminComplianceStatus } from '@/api/admin/compliance'
import { getLocale } from '@/i18n'

const FALLBACK_ZH_PHRASE = '我已阅读、理解并同意 Sub2API 部署与运营合规承诺'
const FALLBACK_EN_PHRASE = 'I have read, understood, and agree to the Sub2API Deployment and Operation Compliance Commitment'

export type AdminComplianceBlockReason = 'required' | 'unavailable' | null

export const useAdminComplianceStore = defineStore('adminCompliance', () => {
  const status = ref<AdminComplianceStatus | null>(null)
  const loading = ref(false)
  const submitting = ref(false)
  const initialized = ref(false)
  const forceVisible = ref(false)
  /** Distinguishes real ack-required (423) from temporary status-fetch failures. */
  const blockReason = ref<AdminComplianceBlockReason>(null)

  const required = computed(() => status.value?.required === true || blockReason.value === 'required')
  const unavailable = computed(() => blockReason.value === 'unavailable')
  const shouldShow = computed(() => required.value || unavailable.value || forceVisible.value)
  const currentLocale = computed(() => getLocale())
  const expectedPhrase = computed(() => {
    if (currentLocale.value === 'zh') {
      return status.value?.ack_phrase_zh || FALLBACK_ZH_PHRASE
    }
    return status.value?.ack_phrase_en || FALLBACK_EN_PHRASE
  })

  async function fetchStatus(): Promise<AdminComplianceStatus> {
    loading.value = true
    try {
      const nextStatus = await adminComplianceAPI.getStatus()
      status.value = nextStatus
      initialized.value = true
      blockReason.value = nextStatus.required ? 'required' : null
      forceVisible.value = nextStatus.required
      return nextStatus
    } finally {
      loading.value = false
    }
  }

  async function accept(phrase: string): Promise<AdminComplianceStatus> {
    submitting.value = true
    try {
      const nextStatus = await adminComplianceAPI.accept({
        phrase,
        language: currentLocale.value
      })
      status.value = nextStatus
      blockReason.value = nextStatus.required ? 'required' : null
      forceVisible.value = nextStatus.required
      return nextStatus
    } finally {
      submitting.value = false
    }
  }

  function requireAcknowledgement(partialStatus?: Partial<AdminComplianceStatus>): void {
    status.value = {
      required: true,
      version: partialStatus?.version || status.value?.version || 'v2026.06.10',
      document_path_zh: partialStatus?.document_path_zh || status.value?.document_path_zh || 'docs/legal/admin-compliance.zh.md',
      document_path_en: partialStatus?.document_path_en || status.value?.document_path_en || 'docs/legal/admin-compliance.en.md',
      document_url_zh: partialStatus?.document_url_zh || status.value?.document_url_zh || 'https://github.com/Wei-Shaw/sub2api/blob/main/docs/legal/admin-compliance.zh.md',
      document_url_en: partialStatus?.document_url_en || status.value?.document_url_en || 'https://github.com/Wei-Shaw/sub2api/blob/main/docs/legal/admin-compliance.en.md',
      ack_phrase_zh: partialStatus?.ack_phrase_zh || status.value?.ack_phrase_zh || FALLBACK_ZH_PHRASE,
      ack_phrase_en: partialStatus?.ack_phrase_en || status.value?.ack_phrase_en || FALLBACK_EN_PHRASE,
      acknowledgement: status.value?.acknowledgement
    }
    initialized.value = true
    blockReason.value = 'required'
    forceVisible.value = true
  }

  /** Fail-closed lockout when status cannot be loaded (network/5xx). Not a real ack dialog. */
  function markStatusUnavailable(): void {
    initialized.value = false
    blockReason.value = 'unavailable'
    forceVisible.value = true
  }

  async function retryStatusFetch(): Promise<void> {
    loading.value = true
    try {
      await fetchStatus()
    } catch (error) {
      const err = error as { status?: number; code?: string; metadata?: Partial<AdminComplianceStatus> }
      if (err.status === 423 && err.code === 'ADMIN_COMPLIANCE_ACK_REQUIRED') {
        requireAcknowledgement(err.metadata)
        return
      }
      markStatusUnavailable()
      throw error
    } finally {
      loading.value = false
    }
  }

  function reset(): void {
    status.value = null
    loading.value = false
    submitting.value = false
    initialized.value = false
    forceVisible.value = false
    blockReason.value = null
  }

  return {
    status,
    loading,
    submitting,
    initialized,
    blockReason,
    required,
    unavailable,
    shouldShow,
    expectedPhrase,
    fetchStatus,
    accept,
    requireAcknowledgement,
    markStatusUnavailable,
    retryStatusFetch,
    reset
  }
})
