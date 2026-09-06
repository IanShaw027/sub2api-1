import { beforeEach, describe, expect, it, vi } from 'vitest'
import { authenticatedFetch } from '@/api/authenticatedFetch'
import { imageJobBlob, mediaAPI } from '../mediaApi'
import type { MediaRequest } from '../localMedia'

vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn() }, buildApiUrl: (path: string) => `/api/v1${path}` }))
vi.mock('@/api/authenticatedFetch', () => ({ authenticatedFetch: vi.fn() }))

const input = (): MediaRequest => ({ mode: 'image', prompt: 'Test', model: 'gpt-image-1', groupId: 2, settings: { quality: 'high', size: '1024x1024' } })

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.setItem('auth_token', 'test-jwt')
  vi.mocked(authenticatedFetch).mockResolvedValue({ ok: true, json: async () => ({ task_id: 'test', status: 'processing' }) } as Response)
})

describe('local media API transport', () => {
  it('uses only the JWT local-generation route, without a session ID', async () => {
    const signal = new AbortController().signal
    await mediaAPI.submit(input(), signal)
    const [url, init] = vi.mocked(authenticatedFetch).mock.calls[0]!
    expect(url).toBe('/api/v1/creation/local/images/generations/async?group_id=2')
    expect(init?.headers).toMatchObject({ Authorization: 'Bearer test-jwt', 'X-Group-Id': '2' })
    expect(init?.headers).not.toHaveProperty('X-Session-Id')
    expect(init?.signal).toBe(signal)
    expect(JSON.parse(init?.body as string)).toEqual({ model: 'gpt-image-1', prompt: 'Test', n: 1, quality: 'high', size: '1024x1024' })
  })

  it('uploads source image files as multipart without forcing a JSON boundary', async () => {
    const file = new File(['image'], 'source.png', { type: 'image/png' })
    await mediaAPI.submit({ ...input(), files: [file] }, new AbortController().signal)
    const [url, init] = vi.mocked(authenticatedFetch).mock.calls[0]!
    expect(url).toContain('/creation/local/images/edits/async?')
    expect(init?.headers).not.toHaveProperty('Content-Type')
    const body = init?.body as FormData
    expect(body.get('prompt')).toBe('Test')
    expect((body.getAll('image')[0] as File).name).toBe('source.png')
  })

  it('does not send the UI ratio as an unsupported OpenAI aspect_ratio parameter', async () => {
    await mediaAPI.submit({ ...input(), settings: { ratio: '3:2', size: '1536x1024' } }, new AbortController().signal)
    expect(JSON.parse(vi.mocked(authenticatedFetch).mock.calls[0]![1]?.body as string)).toEqual({ model: 'gpt-image-1', prompt: 'Test', n: 1, size: '1536x1024' })
  })

  it('uses a data URL for image-to-video and reads video bytes through the authenticated content route', async () => {
    const file = new File(['image'], 'source.png', { type: 'image/png' })
    await mediaAPI.submit({ ...input(), mode: 'video', settings: { duration: 10, resolution: '720p' }, files: [file] }, new AbortController().signal)
    const [url, init] = vi.mocked(authenticatedFetch).mock.calls[0]!
    expect(url).toContain('/creation/local/videos/generations?')
    expect(JSON.parse(init?.body as string)).toMatchObject({ duration: 10, resolution: '720p', image: { url: 'data:image/png;base64,aW1hZ2U=' } })
    const blob = new Blob([new Uint8Array([0, 0, 0, 20, 102, 116, 121, 112, 105, 115, 111, 109])], { type: 'application/octet-stream' })
    Object.defineProperty(blob, 'slice', { value: () => ({ arrayBuffer: async () => new Uint8Array([0, 0, 0, 20, 102, 116, 121, 112, 105, 115, 111, 109]).buffer }) })
    vi.mocked(authenticatedFetch).mockResolvedValueOnce({ ok: true, blob: async () => blob } as Response)
    expect(await mediaAPI.videoBlob(2, 'remote/id', new AbortController().signal)).toMatchObject({ size: 12, type: 'video/mp4' })
    expect(authenticatedFetch).toHaveBeenLastCalledWith('/api/v1/creation/local/videos/remote%2Fid/content?group_id=2', expect.objectContaining({ headers: expect.objectContaining({ Authorization: 'Bearer test-jwt' }) }))
  })

  it('decodes returned image bytes and refuses an external-only result instead of leaking the JWT', () => {
    expect(imageJobBlob({ status: 'completed', result: { data: [{ b64_json: 'aW1hZ2U=', mime_type: 'image/webp' }] } }).blob).toMatchObject({ size: 5, type: 'image/webp' })
    expect(() => imageJobBlob({ status: 'completed', result: { data: [] } })).toThrow('downloadable image')
    expect(authenticatedFetch).not.toHaveBeenCalled()
  })

  it('surfaces server authorization errors rather than treating them as successful jobs', async () => {
    vi.mocked(authenticatedFetch).mockResolvedValueOnce({ ok: false, status: 403, json: async () => ({ error: { message: 'Group unavailable' } }) } as Response)
    await expect(mediaAPI.submit(input(), new AbortController().signal)).rejects.toThrow('Group unavailable')
  })
})
