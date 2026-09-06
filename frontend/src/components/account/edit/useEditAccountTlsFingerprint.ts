// Extracted verbatim from EditAccountModal.vue's <script setup>: TLS
// fingerprint + device-learning state and helpers. Deliberately NOT shared
// with create/useCreateAccountTlsFingerprint.ts — Edit's applyTLSFingerprintToExtra
// must be able to CLEAR previously-set extra fields when the toggle is turned
// off (editing an existing account), whereas Create's version only ever sets
// fields on a blank new account. Edit also owns loadTLSProfiles /
// loadCompleteDeviceTLSProfiles (remote catalog fetches) and the
// deviceLearning-related refs, which Create has no equivalent of.
import { ref, computed } from 'vue'
import { adminAPI } from '@/api/admin'
import type { DeviceTLSCatalogOption } from '@/api/admin/tlsFingerprintProfile'

export interface EditAccountTlsFingerprintDeps {
  t: (key: string, params?: Record<string, unknown>) => string
}

export function useEditAccountTlsFingerprint(deps: EditAccountTlsFingerprintDeps) {
  const { t } = deps

  const tlsFingerprintEnabled = ref(false)
  const tlsFingerprintProfileId = ref<number | null>(null)
  const tlsFingerprintProfiles = ref<{ id: number; name: string }[]>([])
  const tlsFingerprintRouters = ref<{ id: number; name: string }[]>([])
  const tlsFingerprintRouterId = ref<number | null>(null)
  const tlsFingerprintDefaultOS = ref('')
  const tlsFingerprintBindingRows = ref<{ os: string; client: string; protocol: string; profileId: number }[]>([])
  const deviceLearningEnabled = ref(false)
  const deviceTLSProfileId = ref<number | null>(null)
  const deviceTLSCatalogOptions = ref<DeviceTLSCatalogOption[]>([])
  const deviceTLSProfileSelection = computed({
    get: () => (deviceTLSProfileId.value == null ? '' : String(deviceTLSProfileId.value)),
    set: (value: string) => {
      const id = Number(value)
      deviceTLSProfileId.value = Number.isInteger(id) && id > 0 ? id : null
    }
  })

  function formatDeviceTLSProfileLabel(option: DeviceTLSCatalogOption) {
    const label = t('admin.accounts.deviceLearning.tlsProfileOption', {
      name: option.name,
      family: option.client_family,
      os: option.os_family,
      transport: option.transport,
      software: option.software_label
    })
    return option.description ? `${label} - ${option.description}` : label
  }

  function readDeviceTLSProfileId(value: unknown): number | null {
    const id = Number(value)
    return Number.isInteger(id) && id > 0 ? id : null
  }

  function supportsTLSFingerprint(platform?: string | null) {
    return ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kiro'].includes(platform || '')
  }

  function applyTLSFingerprintToExtra(extra: Record<string, unknown>) {
    if (tlsFingerprintEnabled.value) {
      extra.enable_tls_fingerprint = true
      if (tlsFingerprintProfileId.value) {
        extra.tls_fingerprint_profile_id = tlsFingerprintProfileId.value
      } else {
        delete extra.tls_fingerprint_profile_id
      }
      if (tlsFingerprintRouterId.value) {
        extra.tls_fingerprint_router_id = tlsFingerprintRouterId.value
      } else {
        delete extra.tls_fingerprint_router_id
      }
      if (tlsFingerprintDefaultOS.value) {
        extra.tls_fingerprint_default_os = tlsFingerprintDefaultOS.value
      } else {
        delete extra.tls_fingerprint_default_os
      }
      const bindings = tlsFingerprintBindingsFromRows()
      if (bindings) {
        extra.tls_fingerprint_bindings = bindings
      } else {
        delete extra.tls_fingerprint_bindings
      }
      return
    }
    delete extra.enable_tls_fingerprint
    delete extra.tls_fingerprint_profile_id
    delete extra.tls_fingerprint_router_id
    delete extra.tls_fingerprint_default_os
    delete extra.tls_fingerprint_bindings
  }

  function tlsFingerprintBindingsFromRows(): Record<string, number> | undefined {
    const out: Record<string, number> = {}
    for (const row of tlsFingerprintBindingRows.value) {
      const parts = [row.os, row.client].filter((part) => part.trim())
      let key = parts.join('/').toLowerCase()
      if (row.protocol.trim()) {
        key = key ? `${key}@${row.protocol.trim().toLowerCase()}` : row.protocol.trim().toLowerCase()
      }
      if (!key || !row.profileId) continue
      out[key] = row.profileId
    }
    return Object.keys(out).length ? out : undefined
  }

  function tlsFingerprintRowsFromBindings(bindings?: Record<string, number> | null) {
    tlsFingerprintBindingRows.value = Object.entries(bindings || {}).map(([key, profileId]) => {
      const [left, protocol = ''] = key.split('@')
      const [os = '', client = ''] = left.split('/')
      return { os, client, protocol, profileId }
    })
  }

  async function loadTLSProfiles() {
    try {
      const [profiles, routers] = await Promise.all([
        adminAPI.tlsFingerprintProfiles.list(),
        adminAPI.tlsFingerprintRouters.list()
      ])
      tlsFingerprintProfiles.value = profiles.map(p => ({ id: p.id, name: p.name }))
      tlsFingerprintRouters.value = routers.map(r => ({ id: r.id, name: r.name }))
    } catch {
      tlsFingerprintProfiles.value = []
      tlsFingerprintRouters.value = []
    }
  }

  async function loadCompleteDeviceTLSProfiles(platform?: string | null) {
    if (!platform) {
      deviceTLSCatalogOptions.value = []
      return
    }
    try {
      deviceTLSCatalogOptions.value = await adminAPI.tlsFingerprintProfiles.listComplete(platform)
    } catch {
      deviceTLSCatalogOptions.value = []
    }
  }

  return {
    tlsFingerprintEnabled,
    tlsFingerprintProfileId,
    tlsFingerprintProfiles,
    tlsFingerprintRouters,
    tlsFingerprintRouterId,
    tlsFingerprintDefaultOS,
    tlsFingerprintBindingRows,
    deviceLearningEnabled,
    deviceTLSProfileId,
    deviceTLSCatalogOptions,
    deviceTLSProfileSelection,
    formatDeviceTLSProfileLabel,
    readDeviceTLSProfileId,
    supportsTLSFingerprint,
    applyTLSFingerprintToExtra,
    tlsFingerprintBindingsFromRows,
    tlsFingerprintRowsFromBindings,
    loadTLSProfiles,
    loadCompleteDeviceTLSProfiles
  }
}
