import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AxiosHeaders } from 'axios'
import { getPublicationRequest, listPublications, MAX_PUBLICATION_BYTES, publishMedia, withdrawPublication } from '../publicationApi'

const client = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), delete: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client, buildApiUrl: (path: string) => path }))
const publication = { id: 1, status: 'published', media_url: '/api/v1/creation/gallery/1/media' }
const draft = { request_id: 'request-1', title: 'A work', prompt: 'A forest', model: 'image-model' }
beforeEach(() => { vi.clearAllMocks(); client.post.mockResolvedValue({ data: publication }) })

describe('publication API contract', () => {
  it('uploads the real Blob and explicit public visibility with an idempotency key', async () => {
    const blob = new Blob(['image-bytes'], { type: 'image/png' })
    await publishMedia({ id: 'local-1', kind: 'image', blob, prompt: 'local prompt' }, draft)
    const [path, body, config] = client.post.mock.calls[0]
    expect(path).toBe('/creation/publications')
    expect(body).toBeInstanceOf(FormData)
    expect(body.get('file')).toBeInstanceOf(Blob)
    expect(body.get('file').size).toBe(blob.size)
    expect(body.get('visibility')).toBe('public')
    expect(body.get('request_id')).toBe('request-1')
    expect(body.get('prompt')).toBe('A forest')
    expect(body.has('media_url')).toBe(false)
    const headers = new AxiosHeaders({ 'Content-Type': 'application/json' })
    expect(config.transformRequest[0](body, headers)).toBe(body)
    expect(headers.has('Content-Type')).toBe(false)
  })

  it('rejects unsupported media and oversized files without sending requests', async () => {
    await expect(publishMedia({ id: 'svg', kind: 'image', blob: new Blob(['<svg />'], { type: 'image/svg+xml' }), prompt: '' }, draft)).rejects.toThrow('Unsupported')
    const blob = new Blob(['video'], { type: 'video/mp4' })
    Object.defineProperty(blob, 'size', { value: MAX_PUBLICATION_BYTES + 1 })
    await expect(publishMedia({ id: 'large', kind: 'video', blob, prompt: '' }, draft)).rejects.toThrow('size')
    expect(client.post).not.toHaveBeenCalled()
  })

  it('sends real gallery filters, request lookups and owner withdrawal paths', async () => {
    client.get.mockResolvedValueOnce({ data: { items: [publication], page: 2, pages: 3, total: 50, page_size: 24 } })
    await listPublications({ kind: 'video', search: 'forest', page: 2, page_size: 24 })
    expect(client.get).toHaveBeenCalledWith('/creation/gallery', expect.objectContaining({ params: { kind: 'video', search: 'forest', page: 2, page_size: 24 } }))
    client.get.mockResolvedValueOnce({ data: publication })
    await getPublicationRequest('request/1')
    expect(client.get).toHaveBeenLastCalledWith('/creation/publications/requests/request%2F1')
    await withdrawPublication(1)
    expect(client.delete).toHaveBeenCalledWith('/creation/publications/1')
  })

  it('preserves pending request status and an absent public media URL', async () => {
    client.get.mockResolvedValueOnce({ data: { ...publication, status: 'pending', media_url: '' } })
    expect(await getPublicationRequest('pending-request')).toMatchObject({ status: 'pending', media_url: '' })
  })
})
