import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { effectScope, nextTick, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { apiClient } from '@/api/client'
import { mediaAPI, normalizeMediaCost, unavailableMediaCost } from '../mediaApi'
import { localMedia, type MediaCost, type MediaRequest, type MediaTask } from '../localMedia'
import { useMediaWorkspace } from '../stores/mediaWorkspace'
import { useMediaQuote } from '../useMediaQuote'
import MediaBillingLabel from '../components/MediaBillingLabel.vue'

const auth = vi.hoisted(() => ({ current: null as unknown as { user: { id: number }; isAuthenticated: boolean } }))
vi.mock('vue-i18n', async importOriginal => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const { ref } = await import('vue')
  return {
    ...actual,
    useI18n: (options: { messages: { en: Record<string, string> } }) => ({
      locale: ref('en'),
      t: (key: string) => {
        const message = options.messages.en[key]
        if (message === undefined) throw new Error(`Missing English media translation: ${key}`)
        return message
      },
    }),
  }
})
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth.current }))
vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn() }, buildApiUrl: (path: string) => `/api/v1${path}` }))
vi.mock('@/api/authenticatedFetch', () => ({ authenticatedFetch: vi.fn() }))
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getAvailable: vi.fn(async () => [
  { id: 1, name: 'Images', platform: 'openai' }, { id: 2, name: 'Video', platform: 'grok' },
]) } }))
vi.mock('../localMedia', async importOriginal => ({
  ...await importOriginal<typeof import('../localMedia')>(),
  localMedia: { listTasks: vi.fn(async () => []), saveTask: vi.fn(async () => {}), deleteTasks: vi.fn(), loadDraft: vi.fn(), saveDraft: vi.fn() },
}))

const cost = (status: MediaCost['status'] = 'estimated', amount: number | null = 0.04): MediaCost => ({ status, amount, currency: 'USD', billing_target: amount == null ? null : 'balance' })
const input = (overrides: Partial<MediaRequest> = {}): MediaRequest => ({ mode: 'image', groupId: 1, model: 'gpt-image-1', prompt: 'A poster', settings: { size: '1024x1024' }, ...overrides })
const saved = (overrides: Partial<MediaTask> = {}): MediaTask => ({ id: 'local-1', userId: 1, mode: 'image', type: 'generation', model: 'gpt-image-1', prompt: 'A poster', groupId: 1, settings: {}, sourceFiles: [], createdAt: '2026-01-01T00:00:00Z', status: 'completed', providerTaskId: 'remote-1', persisted: true, blob: new Blob(['image'], { type: 'image/png' }), ...overrides })
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(done => { resolve = done })
  return { promise, resolve }
}
let store: ReturnType<typeof useMediaWorkspace>
const OriginalURL = URL
beforeEach(() => {
  vi.useFakeTimers()
  vi.clearAllMocks()
  auth.current = reactive({ user: { id: 1 }, isAuthenticated: true })
  vi.stubGlobal('URL', class extends OriginalURL {
    static createObjectURL() { return 'blob:saved-media' }
    static revokeObjectURL() {}
  })
  vi.mocked(localMedia.listTasks).mockResolvedValue([])
  vi.mocked(localMedia.saveTask).mockResolvedValue(undefined)
  vi.mocked(apiClient.get).mockImplementation(async path => ({ data: path === '/creation/models' ? { data: [{ id: 'gpt-image-1' }, { id: 'grok-imagine-video' }] } : path === '/creation/local/billing' ? cost('settled') : cost() }))
  setActivePinia(createPinia())
  store = useMediaWorkspace()
})
afterEach(() => { store.$dispose(); vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.useRealTimers() })

describe('media pricing transport and display', () => {
  it('quotes exact image size, Gemini tier and video parameters without inventing edit dimensions', async () => {
    const requests = [
      input(), input({ model: 'gemini-3-pro-image', settings: { size: '1024x1024', resolution: '4K' } }),
      input({ mode: 'video', groupId: 2, settings: {} }),
      input({ mode: 'video', groupId: 2, videoOperation: 'edit', settings: { duration: 15, resolution: '720p' } }),
      input({ mode: 'video', groupId: 2, videoOperation: 'extension', settings: { duration: 8, resolution: '720p' } }),
    ]
    for (const request of requests) await mediaAPI.quote(request)
    const params = vi.mocked(apiClient.get).mock.calls.map(([, options]) => options?.params)
    expect(params[0]).toMatchObject({ group_id: 1, kind: 'image', size: '1024x1024', duration: undefined })
    expect(params[1]).toMatchObject({ size: '4K' })
    expect(params[2]).toMatchObject({ duration: 5, resolution: '720p' })
    expect(params[3]).toMatchObject({ duration: undefined, resolution: undefined })
    expect(params[4]).toMatchObject({ duration: 8, resolution: undefined })
  })

  it('queries receipts by owned task ID, never by arbitrary billing or request ID', async () => {
    await mediaAPI.billing('video', 2, 'accepted-video')
    expect(apiClient.get).toHaveBeenCalledWith('/creation/local/billing', expect.objectContaining({ params: { group_id: 2, kind: 'video', task_id: 'accepted-video' } }))
  })

  it('does not turn unknown, invalid or pending amounts into zero', () => {
    expect(normalizeMediaCost(cost('unavailable', 0)).amount).toBeNull()
    expect(normalizeMediaCost(cost('pending', 0)).amount).toBeNull()
    expect(normalizeMediaCost(cost('estimated', null)).status).toBe('unavailable')
    expect(normalizeMediaCost(cost('settled', NaN)).amount).toBeNull()
    expect(normalizeMediaCost(cost('not_billed', 1)).status).toBe('unavailable')
    expect(normalizeMediaCost(cost('not_billed', 0))).toMatchObject({ status: 'not_billed', amount: 0, billing_target: null })
  })

  it('rejects statuses belonging to the other endpoint', async () => {
    vi.mocked(apiClient.get).mockResolvedValueOnce({ data: cost('settled') }).mockResolvedValueOnce({ data: cost('estimated') })
    expect((await mediaAPI.quote(input())).status).toBe('unavailable')
    expect((await mediaAPI.billing('image', 1, 'remote-1')).status).toBe('unavailable')
  })

  it('shows actual settlement for unknown pricing and balance or subscription for known amounts', async () => {
    const wrapper = mount(MediaBillingLabel, { props: { cost: unavailableMediaCost() }, global: { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: {} } })] } })
    try {
      expect(wrapper.text()).toBe('Charged on actual usage')
      await wrapper.setProps({ cost: cost() })
      expect(wrapper.text()).toContain('$0.04')
      expect(wrapper.text()).toMatch(/balance/i)
      await wrapper.setProps({ cost: { ...cost('settled'), billing_target: 'subscription' } })
      expect(wrapper.text()).toMatch(/subscription/i)
      await wrapper.setProps({ cost: cost('not_billed', 0) })
      expect(wrapper.text()).toBe('Not billed (simple mode)')
    } finally { wrapper.unmount() }
  })

  it('discards a slow quote after parameters change', async () => {
    const first = deferred<MediaCost>()
    const quoteAPI = vi.spyOn(mediaAPI, 'quote').mockReturnValueOnce(first.promise).mockResolvedValueOnce(cost('estimated', 0.08))
    const request = reactive(input())
    const scope = effectScope()
    const quote = scope.run(() => useMediaQuote(() => request))!
    try {
      await vi.advanceTimersByTimeAsync(300)
      request.settings.size = '1536x1024'
      await nextTick()
      expect(quote.quote.value).toBeNull()
      await vi.advanceTimersByTimeAsync(300)
      expect(quote.quote.value?.amount).toBe(0.08)
      first.resolve(cost('estimated', 0.04))
      await flushPromises()
      expect(quote.quote.value?.amount).toBe(0.08)
      expect(quoteAPI.mock.calls[0]![1]?.aborted).toBe(true)
    } finally { scope.stop() }
  })
})

describe('media submission and actual settlement', () => {
  it('refreshes the quote from an immutable request snapshot before paid submission', async () => {
    await store.init()
    const pending = deferred<MediaCost>()
    const quoteAPI = vi.spyOn(mediaAPI, 'quote').mockReturnValue(pending.promise)
    const submitAPI = vi.spyOn(mediaAPI, 'submit').mockResolvedValue({ task_id: 'new-1', status: 'failed', error: 'provider failed' })
    const request = input()
    const taskPromise = store.submit(request)
    await flushPromises()
    expect(submitAPI).not.toHaveBeenCalled()
    request.settings.size = '1536x1024'
    pending.resolve(cost('estimated', 0.06))
    const task = await taskPromise
    await flushPromises()
    expect(quoteAPI.mock.calls[0]![0].settings.size).toBe('1024x1024')
    expect(submitAPI.mock.calls[0]![0].settings.size).toBe('1024x1024')
    expect(task.estimate?.amount).toBe(0.06)
  })

  it('cancels a delayed quote on account change without submitting a paid task', async () => {
    await store.init()
    const pending = deferred<MediaCost>()
    const quoteAPI = vi.spyOn(mediaAPI, 'quote').mockReturnValue(pending.promise)
    const submitAPI = vi.spyOn(mediaAPI, 'submit')
    const result = store.submit(input()).then(() => null, error => error)
    await flushPromises()
    auth.current.user.id = 2
    pending.resolve(cost())
    expect((await result).name).toBe('AbortError')
    expect(quoteAPI.mock.calls[0]![1]?.aborted).toBe(true)
    expect(submitAPI).not.toHaveBeenCalled()
    expect(store.tasks).toHaveLength(0)
  })

  it('restores receipt queries on reload and persists settled costs without regenerating media', async () => {
    vi.mocked(localMedia.listTasks).mockResolvedValue([saved()])
    const billingAPI = vi.spyOn(mediaAPI, 'billing').mockResolvedValueOnce(cost('pending', null)).mockResolvedValueOnce(cost('settled', 0.023))
    const submitAPI = vi.spyOn(mediaAPI, 'submit')
    await store.init()
    await flushPromises()
    expect(store.tasks[0]?.billing?.status).toBe('pending')
    await vi.advanceTimersByTimeAsync(2000)
    expect(store.tasks[0]).toMatchObject({ status: 'completed', persisted: true, billing: { status: 'settled', amount: 0.023 } })
    expect(localMedia.saveTask).toHaveBeenCalledWith(expect.objectContaining({ billing: expect.objectContaining({ amount: 0.023 }) }))
    expect(submitAPI).not.toHaveBeenCalled()
    await store.refreshBilling('local-1')
    expect(billingAPI).toHaveBeenCalledTimes(2)
  })

  it('bounds pending retries and allows a later explicit refresh', async () => {
    vi.mocked(localMedia.listTasks).mockResolvedValue([saved()])
    const billingAPI = vi.spyOn(mediaAPI, 'billing').mockResolvedValue(cost('pending', null))
    await store.init()
    await vi.advanceTimersByTimeAsync(10000)
    expect(billingAPI).toHaveBeenCalledTimes(3)
    billingAPI.mockResolvedValue(cost('settled', 0.1))
    await store.refreshBilling('local-1')
    expect(store.tasks[0]?.billing?.amount).toBe(0.1)
  })

  it('keeps the saved blob and completion state when receipt lookup or metadata persistence fails', async () => {
    const task = saved()
    vi.mocked(localMedia.listTasks).mockResolvedValue([task])
    vi.spyOn(mediaAPI, 'billing').mockRejectedValue(new Error('billing temporarily unavailable'))
    vi.mocked(localMedia.saveTask).mockRejectedValue(new Error('quota'))
    await store.init()
    await flushPromises()
    expect(store.tasks[0]).toMatchObject({ status: 'completed', persisted: true, blob: task.blob, mediaUrl: 'blob:saved-media', billing: { status: 'unavailable', amount: null } })
    expect(store.error).toBeNull()
  })

  it('saves an accepted video warning and polls the same task exactly once without resubmission', async () => {
    vi.spyOn(mediaAPI, 'quote').mockResolvedValue(unavailableMediaCost())
    const submitAPI = vi.spyOn(mediaAPI, 'submit').mockResolvedValue({ request_id: 'accepted-video', status: 'pending', observation_persisted: false, observation_warning: 'Keep polling; do not submit again.' })
    const pollAPI = vi.spyOn(mediaAPI, 'poll').mockResolvedValue({ request_id: 'accepted-video', status: 'completed' })
    vi.spyOn(mediaAPI, 'videoBlob').mockResolvedValue(new Blob(['video'], { type: 'video/mp4' }))
    const task = await store.submit(input({ mode: 'video', groupId: 2, model: 'grok-imagine-video', settings: {} }))
    await flushPromises()
    expect(localMedia.saveTask).toHaveBeenCalledWith(expect.objectContaining({ providerTaskId: 'accepted-video', observationPersisted: false }))
    expect(task.observationWarning).toContain('do not submit again')
    await vi.advanceTimersByTimeAsync(3000)
    expect(pollAPI).toHaveBeenCalledWith('video', 2, 'accepted-video', expect.any(AbortSignal))
    expect(submitAPI).toHaveBeenCalledTimes(1)
    expect(task.status).toBe('completed')
  })
})
