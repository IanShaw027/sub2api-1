import { ref } from 'vue'
import { saveAs } from 'file-saver'
import { fetchCodexModelsManifest } from '@/api/codex'
import type { CodexModelManifestState } from './types'

export interface UseCodexModelManifestOptions {
  baseUrl: () => string
  apiKey: () => string
  canLoad: () => boolean
}

/**
 * Fetches, holds and resets the downloadable Codex model catalog manifest
 * (config.toml `model_catalog_json` target) used by the Codex instruction tabs.
 */
export function useCodexModelManifest(options: UseCodexModelManifestOptions) {
  const codexModelManifestState = ref<CodexModelManifestState>('idle')
  const codexModelManifestContent = ref('')
  const codexModelManifestModelCount = ref(0)
  let codexModelManifestController: AbortController | null = null
  let codexModelManifestRequestID = 0

  function resetCodexModelManifest() {
    codexModelManifestController?.abort()
    codexModelManifestController = null
    codexModelManifestRequestID += 1
    codexModelManifestState.value = 'idle'
    codexModelManifestContent.value = ''
    codexModelManifestModelCount.value = 0
  }

  async function loadCodexModelManifest() {
    if (!options.canLoad() || !options.apiKey()) return

    codexModelManifestController?.abort()
    const controller = new AbortController()
    const requestID = ++codexModelManifestRequestID
    codexModelManifestController = controller
    codexModelManifestState.value = 'loading'

    try {
      const result = await fetchCodexModelsManifest(options.baseUrl(), options.apiKey(), controller.signal)
      if (requestID !== codexModelManifestRequestID) return
      codexModelManifestContent.value = result.content
      codexModelManifestModelCount.value = result.modelCount
      codexModelManifestState.value = 'ready'
    } catch (error) {
      const errorName = error && typeof error === 'object' && 'name' in error
        ? String((error as { name?: unknown }).name || '')
        : ''
      if (requestID !== codexModelManifestRequestID || errorName === 'AbortError') return
      codexModelManifestState.value = 'error'
    } finally {
      if (requestID === codexModelManifestRequestID) {
        codexModelManifestController = null
      }
    }
  }

  function downloadCodexModelManifest() {
    if (!codexModelManifestContent.value) return
    saveAs(
      new Blob([codexModelManifestContent.value], { type: 'application/json;charset=utf-8' }),
      'codex-models.json'
    )
  }

  return {
    codexModelManifestState,
    codexModelManifestContent,
    codexModelManifestModelCount,
    resetCodexModelManifest,
    loadCodexModelManifest,
    downloadCodexModelManifest
  }
}
