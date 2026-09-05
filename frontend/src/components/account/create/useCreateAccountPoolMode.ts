import { ref } from 'vue'

export function useCreateAccountPoolMode() {
  const DEFAULT_POOL_MODE_RETRY_COUNT = 3
  const MAX_POOL_MODE_RETRY_COUNT = 10
  const poolModeEnabled = ref(false)
  const poolModeRetryCount = ref(DEFAULT_POOL_MODE_RETRY_COUNT)
  const poolModeRetryStatusCodesInput = ref('')

  function parsePoolModeRetryStatusCodes(input: string): number[] {
    if (!input || !input.trim()) return []
    const seen = new Set<number>()
    const out: number[] = []
    for (const token of input.split(/[,\s]+/)) {
      const trimmed = token.trim()
      if (!trimmed) continue
      const n = Number(trimmed)
      if (!Number.isFinite(n) || !Number.isInteger(n)) continue
      if (n < 100 || n > 599) continue
      if (seen.has(n)) continue
      seen.add(n)
      out.push(n)
    }
    return out.sort((a, b) => a - b)
  }

  const normalizePoolModeRetryCount = (value: number) => {
    if (!Number.isFinite(value)) {
      return DEFAULT_POOL_MODE_RETRY_COUNT
    }
    const normalized = Math.trunc(value)
    if (normalized < 0) {
      return 0
    }
    if (normalized > MAX_POOL_MODE_RETRY_COUNT) {
      return MAX_POOL_MODE_RETRY_COUNT
    }
    return normalized
  }

  return {
    DEFAULT_POOL_MODE_RETRY_COUNT,
    MAX_POOL_MODE_RETRY_COUNT,
    poolModeEnabled,
    poolModeRetryCount,
    poolModeRetryStatusCodesInput,
    parsePoolModeRetryStatusCodes,
    normalizePoolModeRetryCount
  }
}
