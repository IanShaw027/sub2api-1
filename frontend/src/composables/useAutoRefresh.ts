import { ref, onBeforeUnmount, unref, watch, type MaybeRef, type Ref } from 'vue'

export interface UseAutoRefreshOptions {
  storageKey: string
  intervals?: readonly number[]
  defaultInterval?: MaybeRef<number>
  onRefresh: () => Promise<void> | void
  /** Skip tick when this returns true (e.g. modal open, document hidden). */
  shouldPause?: () => boolean
}

export function useAutoRefresh(options: UseAutoRefreshOptions) {
  const {
    storageKey,
    intervals = [5, 10, 15, 30] as const,
    defaultInterval,
    onRefresh,
    shouldPause,
  } = options

  const resolveDefaultInterval = () => {
    const resolved = Number(unref(defaultInterval))
    if (Number.isFinite(resolved) && resolved > 0) return resolved
    return intervals[intervals.length - 1]
  }

  const enabled = ref(false)
  const intervalSeconds = ref(resolveDefaultInterval())
  const countdown = ref(0)
  const fetching = ref(false)

  let timerId: number | undefined
  let usingDefaultInterval = true

  function loadFromStorage() {
    try {
      const saved = localStorage.getItem(storageKey)
      if (!saved) return
      const parsed = JSON.parse(saved) as { enabled?: boolean; interval_seconds?: number }
      enabled.value = parsed.enabled === true
      const iv = Number(parsed.interval_seconds)
      if (intervals.includes(iv as any)) {
        intervalSeconds.value = iv
        usingDefaultInterval = iv === resolveDefaultInterval()
      }
    } catch { /* ignore */ }
  }

  function saveToStorage() {
    try {
      localStorage.setItem(storageKey, JSON.stringify({
        enabled: enabled.value,
        interval_seconds: intervalSeconds.value,
      }))
    } catch { /* ignore */ }
  }

  async function tick() {
    if (!enabled.value) return
    if (shouldPause?.()) return
    if (fetching.value) return

    if (countdown.value <= 0) {
      countdown.value = intervalSeconds.value
      fetching.value = true
      try { await onRefresh() } finally { fetching.value = false }
      return
    }
    countdown.value -= 1
  }

  function start() {
    if (timerId !== undefined) return
    timerId = setInterval(tick, 1000) as unknown as number
  }

  function stop() {
    if (timerId !== undefined) {
      clearInterval(timerId)
      timerId = undefined
    }
  }

  function setEnabled(value: boolean) {
    enabled.value = value
    saveToStorage()
    if (value) {
      countdown.value = intervalSeconds.value
      start()
    } else {
      stop()
      countdown.value = 0
    }
  }

  function setInterval_(seconds: number) {
    intervalSeconds.value = seconds
    usingDefaultInterval = seconds === resolveDefaultInterval()
    saveToStorage()
    if (enabled.value) countdown.value = seconds
  }

  function resetCountdown() {
    countdown.value = intervalSeconds.value
  }

  loadFromStorage()

  watch(
    () => resolveDefaultInterval(),
    (seconds) => {
      if (!usingDefaultInterval) return
      intervalSeconds.value = seconds
      saveToStorage()
      if (enabled.value) countdown.value = seconds
    },
  )

  onBeforeUnmount(stop)

  return {
    enabled: enabled as Ref<boolean>,
    intervalSeconds: intervalSeconds as Ref<number>,
    countdown: countdown as Ref<number>,
    fetching: fetching as Ref<boolean>,
    intervals,
    setEnabled,
    setInterval: setInterval_,
    resetCountdown,
    start,
    stop,
  }
}
