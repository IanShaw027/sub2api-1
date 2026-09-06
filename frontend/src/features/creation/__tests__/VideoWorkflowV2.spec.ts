import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { authenticatedFetch } from '@/api/authenticatedFetch'
import { inspectSourceVideo, mediaAPI } from '../mediaApi'
import type { MediaRequest } from '../localMedia'
import MediaParameters from '../components/MediaParameters.vue'

vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn() }, buildApiUrl: (path: string) => `/api/v1${path}` }))
vi.mock('@/api/authenticatedFetch', () => ({ authenticatedFetch: vi.fn() }))

const header = new Uint8Array([0, 0, 0, 20, 102, 116, 121, 112, 105, 115, 111, 109])
function source() {
  const file = new File([header], 'source.mp4', { type: 'video/mp4' })
  Object.defineProperty(file, 'slice', { value: () => ({ arrayBuffer: async () => header.buffer }) })
  return file
}
function input(operation: 'edit' | 'extension'): MediaRequest {
  return { mode: 'video', videoOperation: operation, groupId: 20, model: 'grok-imagine-video', prompt: 'Next scene', files: [source()], settings: { duration: 6, resolution: '720p', aspectRatio: '16:9' } }
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.setItem('auth_token', 'test-jwt')
  vi.mocked(authenticatedFetch).mockResolvedValue({ ok: true, json: async () => ({ request_id: 'accepted', status: 'pending' }) } as Response)
})
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('native local video operations', () => {
  it.each(['edit', 'extension'] as const)('sends %s directly with embedded MP4 and only supported settings', async operation => {
    await mediaAPI.submit(input(operation), new AbortController().signal)
    expect(authenticatedFetch).toHaveBeenCalledTimes(1)
    const [url, init] = vi.mocked(authenticatedFetch).mock.calls[0]!
    expect(url).toBe(`/api/v1/creation/local/videos/${operation === 'edit' ? 'edits' : 'extensions'}?group_id=20`)
    const body = JSON.parse(init?.body as string)
    expect(body).toEqual({ model: 'grok-imagine-video', prompt: 'Next scene', video: { url: expect.stringMatching(/^data:video\/mp4;base64,/) }, ...(operation === 'extension' ? { duration: 6 } : {}) })
    expect(init?.headers).toMatchObject({ Authorization: 'Bearer test-jwt', 'X-Group-Id': '20' })
  })

  it('rejects a missing source or unsupported extension duration without any upload', async () => {
    await expect(mediaAPI.submit({ ...input('edit'), files: [] }, new AbortController().signal)).rejects.toThrow('one source MP4')
    await expect(mediaAPI.submit({ ...input('extension'), settings: { duration: 11 } }, new AbortController().signal)).rejects.toThrow('between 2 and 10')
    expect(authenticatedFetch).not.toHaveBeenCalled()
  })

  it.each([
    ['edit', 8.7, true], ['edit', 8.8, false], ['extension', 2, true],
    ['extension', 15, true], ['extension', 1.9, false], ['extension', 15.1, false],
  ] as const)('validates %s source duration %s and revokes temporary URLs', async (operation, duration, valid) => {
    const video = document.createElement('video')
    Object.defineProperty(video, 'duration', { value: duration })
    Object.defineProperty(video, 'src', { set: () => { queueMicrotask(() => video.dispatchEvent(new Event('loadedmetadata'))) } })
    vi.spyOn(video, 'load').mockImplementation(() => {})
    const create = document.createElement.bind(document)
    vi.spyOn(document, 'createElement').mockImplementation((tag, options) => tag === 'video' ? video : create(tag, options))
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:source'), revokeObjectURL: vi.fn() }))
    if (valid) expect(await inspectSourceVideo(source(), operation)).toBe(duration)
    else await expect(inspectSourceVideo(source(), operation)).rejects.toThrow('source')
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:source')
  })

  it('rejects non-MP4 bytes before creating a media element', async () => {
    const file = new Blob(['not a video'])
    Object.defineProperty(file, 'slice', { value: () => ({ arrayBuffer: async () => new Uint8Array(12).buffer }) })
    await expect(inspectSourceVideo(file, 'edit')).rejects.toThrow('MP4')
  })
})

describe('operation-specific parameters', () => {
  const global = { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: {} } })] }
  it('does not offer resolution or duration controls for video editing', async () => {
    const wrapper = mount(MediaParameters, { props: { mode: 'video', model: 'grok-imagine-video', videoOperation: 'edit', settings: {} }, global })
    await wrapper.get('.media-param-trigger').trigger('click')
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.find('.media-param-segments').exists()).toBe(false)
    wrapper.unmount()
  })
  it('offers every integer extension duration from 2 to 10, without generation settings', async () => {
    const wrapper = mount(MediaParameters, { props: { mode: 'video', model: 'grok-imagine-video', videoOperation: 'extension', settings: { duration: 6 } }, global })
    await wrapper.get('.media-param-trigger').trigger('click')
    const duration = wrapper.get('input[type="number"]')
    expect(duration.attributes()).toMatchObject({ min: '2', max: '10', step: '1' })
    await duration.setValue('7')
    expect(wrapper.emitted('update:settings')?.at(-1)?.[0]).toEqual({ duration: 7 })
    expect(wrapper.find('.media-param-segments').exists()).toBe(false)
    wrapper.unmount()
  })
})
