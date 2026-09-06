import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { apiClient } from '@/api/client'
import { authenticatedFetch } from '@/api/authenticatedFetch'
import { mediaAPI } from '../mediaApi'
import { useMediaWorkspace } from '../stores/mediaWorkspace'
import { localMedia, type MediaRequest, type MediaSettings } from '../localMedia'
import MediaParameters from '../components/MediaParameters.vue'

vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn() }, buildApiUrl: (path: string) => `/api/v1${path}` }))
vi.mock('@/api/authenticatedFetch', () => ({ authenticatedFetch: vi.fn() }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 1 }, isAuthenticated: true }) }))
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getAvailable: vi.fn(async () => [
  { id: 55, name: 'Gemini images', platform: 'gemini', allow_image_generation: true },
  { id: 20, name: 'Grok video', platform: 'grok', allow_image_generation: true },
]) } }))
vi.mock('../localMedia', async importOriginal => ({
  ...await importOriginal<typeof import('../localMedia')>(),
  localMedia: { listTasks: vi.fn(async () => []), saveTask: vi.fn(async () => {}), deleteTasks: vi.fn(), loadDraft: vi.fn(), saveDraft: vi.fn() },
}))

const OriginalURL = URL
let store: ReturnType<typeof useMediaWorkspace>
const input = (settings: MediaSettings = {}): MediaRequest => ({ mode: 'image', groupId: 55, model: 'gemini-3.1-flash-image', prompt: 'A clean poster', settings })
const source = () => new File(['source-image'], 'source.png', { type: 'image/png' })
beforeEach(() => {
  vi.clearAllMocks()
  localStorage.setItem('auth_token', 'session-jwt')
  vi.stubGlobal('URL', class extends OriginalURL {
    static createObjectURL() { return 'blob:generated' }
    static revokeObjectURL() {}
  })
  vi.mocked(apiClient.get).mockResolvedValue({ data: { data: [
    { id: 'gemini-3.1-flash-image' }, { id: 'gemini-3-pro-image' }, { id: 'gemini-3-flash' }, { id: 'grok-imagine-video' },
  ] } })
  vi.mocked(authenticatedFetch).mockResolvedValue({ ok: true, json: async () => ({ task_id: 'gemini-task', status: 'completed', result: { data: [{ b64_json: 'aW1hZ2U=', mime_type: 'image/png' }] } }) } as Response)
  setActivePinia(createPinia())
  store = useMediaWorkspace()
})
afterEach(() => { store.$dispose(); vi.unstubAllGlobals() })

describe('Gemini image workflow', () => {
  it('loads actual Gemini groups and image models without offering text or video models', async () => {
    await store.init()
    expect(store.groups.map(group => group.id)).toContain(55)
    expect(store.imageModels).toEqual(['gemini-3.1-flash-image', 'gemini-3-pro-image'])
    expect(store.videoModels).toEqual([])
  })

  it.each(['1K', '2K', '4K'])('sends explicit %s tier to the local route, never OpenAI dimensions or quality', async resolution => {
    await mediaAPI.submit(input({ resolution, ratio: '16:9', size: '3840x2160', quality: 'high' }), new AbortController().signal)
    const [url, init] = vi.mocked(authenticatedFetch).mock.calls[0]!
    expect(url).toBe('/api/v1/creation/local/images/generations/async?group_id=55')
    expect(init?.headers).toMatchObject({ Authorization: 'Bearer session-jwt', 'X-Group-Id': '55' })
    expect(JSON.parse(init?.body as string)).toEqual({ model: input().model, prompt: input().prompt, n: 1, image_size: resolution, aspect_ratio: '16:9' })
  })

  it('makes 1K explicit even before opening the parameter panel', async () => {
    await mediaAPI.submit(input(), new AbortController().signal)
    expect(JSON.parse(vi.mocked(authenticatedFetch).mock.calls[0]![1]?.body as string)).toMatchObject({ image_size: '1K' })
  })

  it('submits all 14 reference files as multipart and saves only local result bytes', async () => {
    const files = Array.from({ length: 14 }, source)
    const task = await store.submit({ ...input({ resolution: '4K', ratio: '3:4', quality: 'high', size: 'auto' }), files })
    await flushPromises()
    const [url, init] = vi.mocked(authenticatedFetch).mock.calls[0]!
    expect(url).toContain('/images/edits/async?group_id=55')
    const form = init?.body as FormData
    expect(form.getAll('image')).toHaveLength(14)
    expect(form.get('image_size')).toBe('4K')
    expect(form.get('aspect_ratio')).toBe('3:4')
    expect(form.has('quality')).toBe(false)
    expect(form.has('size')).toBe(false)
    expect(init?.headers).not.toHaveProperty('Content-Type')
    expect(task.settings).toEqual({ resolution: '4K', ratio: '3:4', aspectRatio: '3:4', n: 1 })
    expect(store.tasks[0]).toMatchObject({ status: 'completed', mediaUrl: 'blob:generated', blob: expect.any(Blob) })
    expect(localMedia.saveTask).toHaveBeenCalled()
    expect(vi.mocked(authenticatedFetch).mock.calls.every(([path]) => !String(path).includes('/publications'))).toBe(true)
  })

  it('rejects oversized reference counts and unsupported parameters before submitting a paid task', async () => {
    await expect(store.submit({ ...input(), files: Array.from({ length: 15 }, source) })).rejects.toThrow('14')
    await expect(store.submit(input({ resolution: '8K' }))).rejects.toThrow('supported Gemini')
    await expect(store.submit(input({ ratio: '21:9' }))).rejects.toThrow('supported Gemini')
    expect(authenticatedFetch).not.toHaveBeenCalled()
    expect(localMedia.saveTask).not.toHaveBeenCalled()
  })

  it('offers supported ratio and tier controls, without an OpenAI quality selector', async () => {
    const wrapper = mount(MediaParameters, {
      props: { mode: 'image', model: 'gemini-3-pro-image', settings: { ratio: 'auto', resolution: '1K', quality: 'high', size: '1024x1024' } },
      global: { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: {} } })] },
    })
    try {
      await wrapper.get('.media-param-trigger').trigger('click')
      expect(wrapper.findAll('.media-ratios button').map(button => button.text())).toContain('16:9')
      expect(wrapper.findAll('.media-param-segments button').map(button => button.text())).toEqual(['1K', '2K', '4K'])
      await wrapper.findAll('.media-ratios button').find(button => button.text() === '16:9')!.trigger('click')
      await wrapper.setProps({ settings: wrapper.emitted('update:settings')!.at(-1)![0] as MediaSettings })
      await wrapper.findAll('.media-param-segments button').find(button => button.text() === '4K')!.trigger('click')
      expect(wrapper.emitted('update:settings')!.at(-1)![0]).toMatchObject({ resolution: '4K', ratio: '16:9', aspectRatio: '16:9', size: undefined, quality: undefined })
    } finally { wrapper.unmount() }
  })
})
