import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { CreationImageJob } from '../types'

const { listImages, getImage, importExistingImage, fetchOriginal } = vi.hoisted(() => ({
  listImages: vi.fn(), getImage: vi.fn(), importExistingImage: vi.fn(), fetchOriginal: vi.fn(),
}))
vi.mock('../api', () => ({ listImages, mapCreationImageJob: (item: unknown) => item }))
vi.mock('@/api/client', () => ({ apiClient: { get: getImage } }))
vi.mock('../stores/mediaWorkspace', () => ({ useMediaWorkspace: () => ({ importExistingImage }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'en' } }) }))

import LegacyImageHistory from '../components/LegacyImageHistory.vue'

const item: CreationImageJob = {
  id: 18, user_id: 7, group_id: 3, status: 'completed', model: 'gpt-image-1',
  prompt: 'A red bicycle', created_at: '2026-08-01T12:00:00Z', updated_at: '2026-08-01T12:01:00Z',
  media_url: 'https://storage.test/old.png?signature=old',
}
const fresh = { ...item, media_url: 'https://storage.test/old.png?signature=renewed' }
const blob = new Blob(['original bytes'], { type: 'image/png' })

function mountHistory(open = true) {
  return mount(LegacyImageHistory, {
    props: { open },
    global: { stubs: {
      UiModal: { props: ['open', 'title', 'subtitle'], template: '<section v-if="open"><h2>{{ title }}</h2><p>{{ subtitle }}</p><slot /><slot name="footer" /></section>' },
      Icon: true,
    } },
  })
}

beforeEach(() => {
  vi.resetAllMocks()
  localStorage.setItem('auth_user', '{"id":7}')
  listImages.mockResolvedValue({ items: [item], total: 25, page: 1, page_size: 24 })
  getImage.mockResolvedValue({ data: fresh })
  importExistingImage.mockResolvedValue({ id: 'local-18' })
  fetchOriginal.mockResolvedValue({ ok: true, blob: async () => blob })
  vi.stubGlobal('fetch', fetchOriginal)
})
afterEach(() => { vi.unstubAllGlobals(); localStorage.clear() })

describe('LegacyImageHistory', () => {
  it('only reads private history after opening and never automatically imports', async () => {
    const wrapper = mountHistory(false)
    await flushPromises()
    expect(listImages).not.toHaveBeenCalled()
    await wrapper.setProps({ open: true })
    await flushPromises()
    expect(listImages).toHaveBeenCalledWith({ page: 1, page_size: 24 })
    expect(wrapper.text()).toContain('Legacy Private Cloud History')
    expect(wrapper.text()).toContain('Not published to the public inspiration wall')
    expect(importExistingImage).not.toHaveBeenCalled()
    expect(fetchOriginal).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('searches the current page and reads additional pages on demand', async () => {
    const wrapper = mountHistory()
    await flushPromises()
    await wrapper.get('input[type="search"]').setValue('missing')
    expect(wrapper.text()).toContain('No matching records on this page')
    await wrapper.get('input[type="search"]').setValue('BICYCLE')
    expect(wrapper.find('[data-legacy-id="18"]').exists()).toBe(true)
    await wrapper.get('button[aria-label="Next page"]').trigger('click')
    await flushPromises()
    expect(listImages).toHaveBeenLastCalledWith({ page: 2, page_size: 24 })
    wrapper.unmount()
  })

  it('imports original bytes and metadata locally without a generation or publishing request', async () => {
    const wrapper = mountHistory()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('Import locally'))!.trigger('click')
    await flushPromises()
    expect(getImage).toHaveBeenCalledWith('/creation/images/18', expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(fetchOriginal).toHaveBeenCalledWith(fresh.media_url, expect.objectContaining({ credentials: 'omit', referrerPolicy: 'no-referrer' }))
    expect(importExistingImage).toHaveBeenCalledWith({
      blob, prompt: item.prompt, model: item.model, groupId: item.group_id,
      createdAt: item.created_at, legacyId: item.id,
    })
    expect(wrapper.emitted('import')).toEqual([[18]])
    expect(wrapper.text()).toContain('Imported locally')
    wrapper.unmount()
  })

  it('keeps cloud records and preview accessible when CORS blocks byte downloads', async () => {
    fetchOriginal.mockRejectedValue(new TypeError('Failed to fetch'))
    const wrapper = mountHistory()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('Import locally'))!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('cross-origin downloads')
    expect(importExistingImage).not.toHaveBeenCalled()
    expect(wrapper.emitted('import')).toBeUndefined()
    await wrapper.get('button[aria-label="Preview original"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('.legacy-preview').attributes('src')).toBe(fresh.media_url)
    expect(wrapper.get('a[target="_blank"]').attributes('rel')).toBe('noopener noreferrer')
    expect(fetchOriginal).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('reports local persistence failure instead of claiming a successful migration', async () => {
    importExistingImage.mockRejectedValue(new Error('QuotaExceededError'))
    const wrapper = mountHistory()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('Import locally'))!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('cloud record was not deleted')
    expect(wrapper.emitted('import')).toBeUndefined()
    expect(wrapper.text()).not.toContain('Imported locally')
    wrapper.unmount()
  })

  it('rejects unsafe renewed media URLs without fetching them', async () => {
    getImage.mockResolvedValue({ data: { ...fresh, media_url: 'javascript:alert(1)' } })
    const wrapper = mountHistory()
    await flushPromises()
    await wrapper.get('button[aria-label="Download original"]').trigger('click')
    await flushPromises()
    expect(fetchOriginal).not.toHaveBeenCalled()
    expect(importExistingImage).not.toHaveBeenCalled()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('does not import a previous user image after the signed-in identity changes', async () => {
    fetchOriginal.mockImplementation(async () => {
      localStorage.setItem('auth_user', '{"id":8}')
      return { ok: true, blob: async () => blob }
    })
    const wrapper = mountHistory()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('Import locally'))!.trigger('click')
    await flushPromises()
    expect(importExistingImage).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('signed-in user changed')
    wrapper.unmount()
  })

  it('aborts an in-flight download when the history closes', async () => {
    fetchOriginal.mockImplementation((_url, init) => new Promise((_resolve, reject) => {
      init.signal.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError')))
    }))
    const wrapper = mountHistory()
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text().includes('Import locally'))!.trigger('click')
    await flushPromises()
    await wrapper.setProps({ open: false })
    await flushPromises()
    expect(fetchOriginal.mock.calls[0][1].signal.aborted).toBe(true)
    expect(importExistingImage).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
