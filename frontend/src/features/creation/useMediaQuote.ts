import { onScopeDispose, ref, watch } from 'vue'
import { mediaAPI, unavailableMediaCost } from './mediaApi'
import type { MediaCost, MediaRequest } from './localMedia'

export function useMediaQuote(input: () => MediaRequest | null) {
  const quote = ref<MediaCost | null>(null)
  const loading = ref(false)
  let generation = 0
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  async function refresh() {
    const value = input()
    if (!value) return
    const current = ++generation
    controller?.abort()
    controller = new AbortController()
    loading.value = true
    let result: MediaCost
    try { result = await mediaAPI.quote({ ...value, settings: { ...value.settings } }, controller.signal) }
    catch { result = unavailableMediaCost() }
    if (current === generation) { quote.value = result; loading.value = false }
  }
  watch(input, () => {
    generation++
    controller?.abort()
    clearTimeout(timer)
    quote.value = null
    loading.value = false
    if (input()) timer = setTimeout(() => { void refresh() }, 300)
  }, { deep: true, immediate: true })
  onScopeDispose(() => { generation++; controller?.abort(); clearTimeout(timer) })
  return { quote, loading, refresh }
}
