import { ref } from 'vue'

export function useCreateAccountTlsFingerprint() {
  const tlsFingerprintEnabled = ref(false)
  const tlsFingerprintProfileId = ref<number | null>(null)
  const tlsFingerprintProfiles = ref<{ id: number; name: string }[]>([])
  const tlsFingerprintRouters = ref<{ id: number; name: string }[]>([])
  const tlsFingerprintRouterId = ref<number | null>(null)
  const tlsFingerprintDefaultOS = ref('')
  const tlsFingerprintBindingRows = ref<{ os: string; client: string; protocol: string; profileId: number }[]>([])

  function supportsTLSFingerprint(platform?: string | null) {
    return ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kiro'].includes(platform || '')
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

  function applyTLSFingerprintToExtra(extra: Record<string, unknown>) {
    if (!tlsFingerprintEnabled.value) return
    extra.enable_tls_fingerprint = true
    if (tlsFingerprintProfileId.value) {
      extra.tls_fingerprint_profile_id = tlsFingerprintProfileId.value
    }
    if (tlsFingerprintRouterId.value) {
      extra.tls_fingerprint_router_id = tlsFingerprintRouterId.value
    }
    if (tlsFingerprintDefaultOS.value) {
      extra.tls_fingerprint_default_os = tlsFingerprintDefaultOS.value
    }
    const bindings = tlsFingerprintBindingsFromRows()
    if (bindings) {
      extra.tls_fingerprint_bindings = bindings
    }
  }

  function withTLSFingerprintExtra(base?: Record<string, unknown>) {
    const extra: Record<string, unknown> = { ...(base || {}) }
    applyTLSFingerprintToExtra(extra)
    return extra
  }

  return {
    tlsFingerprintEnabled,
    tlsFingerprintProfileId,
    tlsFingerprintProfiles,
    tlsFingerprintRouters,
    tlsFingerprintRouterId,
    tlsFingerprintDefaultOS,
    tlsFingerprintBindingRows,
    supportsTLSFingerprint,
    tlsFingerprintBindingsFromRows,
    applyTLSFingerprintToExtra,
    withTLSFingerprintExtra
  }
}
