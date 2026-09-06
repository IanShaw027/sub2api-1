import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import PublishMediaDialog from '../components/PublishMediaDialog.vue'
import InspirationWall from '../components/InspirationWall.vue'
import type { Publication } from '../publicationApi'

const api = vi.hoisted(() => ({ publish: vi.fn(), lookup: vi.fn(), list: vi.fn(), withdraw: vi.fn() }))
vi.mock('../publicationApi', async (importOriginal) => ({
  ...await importOriginal<typeof import('../publicationApi')>(),
  publishMedia: api.publish, getPublicationRequest: api.lookup, listPublications: api.list, withdrawPublication: api.withdraw,
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 7 } }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ locale: { value: 'en' }, t: (key: string) => key }),
}))

const OriginalURL = URL
const revoke = vi.fn()
const publication: Publication = {
  id: 1, owner_user_id: 7, title: 'Forest', prompt: 'A forest', model: 'model-1', kind: 'image',
  status: 'published',
  mime: 'image/png', size: 10, media_url: '/api/v1/creation/gallery/1/media', created_at: '2026-09-06T00:00:00Z',
}
const work = () => ({ id: 'local-1', title: 'Forest', kind: 'image' as const, prompt: 'A forest', model: 'model-1', blob: new Blob(['image'], { type: 'image/png' }) })
const stubs = {
  ReferenceInspiration: true,
  UiModal: { props: ['open'], template: '<div v-if="open"><slot /><slot name="footer" /></div>' },
  ConfirmDialog: { props: ['show'], emits: ['confirm', 'cancel'], template: '<div v-if="show"><button data-confirm @click="$emit(\'confirm\')">Confirm</button><slot /></div>' },
}
let wrapper: VueWrapper | undefined
async function openCommunity() {
  const tab = wrapper!.findAll('[role="radio"]').find(button => button.text() === 'Community works')!
  await tab.trigger('click')
  await flushPromises()
}
beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  vi.stubGlobal('URL', class extends OriginalURL {
    static createObjectURL() { return 'blob:preview' }
    static revokeObjectURL(url: string) { revoke(url) }
  })
  api.publish.mockReset().mockResolvedValue(publication)
  api.lookup.mockReset().mockRejectedValue({ status: 404 })
  api.list.mockReset().mockResolvedValue({ items: [publication], page: 1, pages: 1, total: 1, page_size: 24 })
  api.withdraw.mockReset().mockResolvedValue(undefined)
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.unstubAllGlobals(); vi.useRealTimers() })

describe('explicit media publishing', () => {
  it('does not upload on opening or before explicit public consent', async () => {
    wrapper = mount(PublishMediaDialog, { props: { show: true, work: work() }, global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('Anyone with the public media link')
    expect(api.publish).not.toHaveBeenCalled()
    await wrapper.get('form').trigger('submit')
    expect(api.publish).not.toHaveBeenCalled()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.publish).toHaveBeenCalledOnce()
    expect(api.publish.mock.calls[0][0].blob).toBeInstanceOf(Blob)
    expect(wrapper.emitted('published')?.[0]).toEqual([publication])
    expect(wrapper.text()).toContain('Published publicly')
    wrapper.unmount()
    wrapper = undefined
    expect(revoke).toHaveBeenCalledWith('blob:preview')
  })

  it('keeps the same confirmed payload and request ID across failure and reopen', async () => {
    api.publish.mockRejectedValueOnce(new Error('Network failed'))
    wrapper = mount(PublishMediaDialog, { props: { show: true, work: work() }, global: { stubs } })
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const firstDraft = api.publish.mock.calls[0][1]
    expect(wrapper.get('.publish-field input').attributes('disabled')).toBeDefined()
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(api.lookup).toHaveBeenLastCalledWith(firstDraft.request_id)
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.publish.mock.calls[1][1]).toEqual(firstDraft)
  })

  it('recovers a committed publication after the upload response is lost', async () => {
    api.publish.mockRejectedValueOnce(new Error('Connection lost'))
    api.lookup.mockResolvedValueOnce(publication)
    wrapper = mount(PublishMediaDialog, { props: { show: true, work: work() }, global: { stubs } })
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.emitted('published')?.[0]).toEqual([publication])
    expect(wrapper.find('form').exists()).toBe(false)
  })

  it('keeps a pending upload retryable with the same request ID without reporting success', async () => {
    api.publish.mockRejectedValueOnce(new Error('Upload failed'))
    api.lookup.mockResolvedValueOnce({ ...publication, status: 'pending', media_url: '' })
    wrapper = mount(PublishMediaDialog, { props: { show: true, work: work() }, global: { stubs } })
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.emitted('published')).toBeUndefined()
    expect(wrapper.find('.publish-result').exists()).toBe(false)
    expect(wrapper.text()).toContain('Publication is incomplete and is not publicly visible.')
    expect(wrapper.text()).toContain('Complete public publication')
    const pendingDraft = api.publish.mock.calls[0][1]
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.publish.mock.calls[1][1]).toEqual(pendingDraft)
    expect(wrapper.emitted('published')?.[0]).toEqual([publication])
  })

  it('restores pending status without auto-upload and allows explicit cancellation', async () => {
    localStorage.setItem('creation:publication:7:local-1', JSON.stringify({ request_id: 'pending-1', title: 'Forest', prompt: 'A forest', model: 'model-1' }))
    api.lookup.mockResolvedValueOnce({ ...publication, status: 'pending', media_url: '' })
    wrapper = mount(PublishMediaDialog, { props: { show: true, work: work() }, global: { stubs } })
    await flushPromises()
    expect(api.publish).not.toHaveBeenCalled()
    expect(wrapper.emitted('published')).toBeUndefined()
    expect(wrapper.find('.publish-result').exists()).toBe(false)
    const cleanup = wrapper.findAll('button').find(button => button.text() === 'Cancel pending publication')!
    await cleanup.trigger('click')
    expect(api.withdraw).not.toHaveBeenCalled()
    await wrapper.get('[data-confirm]').trigger('click')
    await flushPromises()
    expect(api.withdraw).toHaveBeenCalledWith(1)
    expect(localStorage.getItem('creation:publication:7:local-1')).toBeNull()
    expect(wrapper.get('.publish-field input').attributes('disabled')).toBeUndefined()
  })

  it('keeps cancellation available when a withdrawn pending upload still needs cleanup', async () => {
    localStorage.setItem('creation:publication:7:local-1', JSON.stringify({ request_id: 'pending-1', title: 'Forest', prompt: 'A forest', model: 'model-1' }))
    api.lookup.mockResolvedValue({ ...publication, status: 'pending', withdrawn_at: '2026-09-06T01:00:00Z', media_url: '' })
    api.withdraw.mockRejectedValueOnce(new Error('Storage unavailable'))
    wrapper = mount(PublishMediaDialog, { props: { show: true, work: work() }, global: { stubs } })
    await flushPromises()
    expect(wrapper.text()).toContain('Publication was cancelled. Server cleanup can be retried.')
    expect(wrapper.find('button[form="publish-media-form"]').exists()).toBe(false)
    const cleanup = wrapper.findAll('button').find(button => button.text() === 'Cancel pending publication')!
    await cleanup.trigger('click')
    await wrapper.get('[data-confirm]').trigger('click')
    await flushPromises()
    expect(localStorage.getItem('creation:publication:7:local-1')).not.toBeNull()
    await wrapper.get('[data-confirm]').trigger('click')
    await flushPromises()
    expect(api.withdraw).toHaveBeenCalledTimes(2)
    expect(localStorage.getItem('creation:publication:7:local-1')).toBeNull()
    expect(api.publish).not.toHaveBeenCalled()
  })

  it('does not infer publication success when the response omits the required status', async () => {
    api.publish.mockResolvedValueOnce({ ...publication, status: undefined })
    wrapper = mount(PublishMediaDialog, { props: { show: true, work: work() }, global: { stubs } })
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.emitted('published')).toBeUndefined()
    expect(wrapper.find('.publish-result').exists()).toBe(false)
    expect(wrapper.text()).toContain('Publication status could not be confirmed')
  })

  it('permits a fresh explicit publication after the earlier publication was withdrawn', async () => {
    localStorage.setItem('creation:publication:7:local-1', JSON.stringify({ request_id: 'old', title: 'Forest', prompt: 'A forest', model: 'model-1' }))
    api.lookup.mockResolvedValueOnce({ ...publication, withdrawn_at: '2026-09-06T01:00:00Z', media_url: '' })
    wrapper = mount(PublishMediaDialog, { props: { show: true, work: work() }, global: { stubs } })
    await flushPromises()
    expect(api.publish).not.toHaveBeenCalled()
    expect(wrapper.get('.publish-field input').attributes('disabled')).toBeUndefined()
    await wrapper.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.publish.mock.calls[0][1].request_id).not.toBe('old')
  })
})

describe('real-publication inspiration wall', () => {
  it('keeps featured inspiration separate from community publication data', () => {
    wrapper = mount(InspirationWall, { global: { stubs } })
    expect(wrapper.findComponent({ name: 'ReferenceInspiration' }).exists()).toBe(true)
    expect(api.list).not.toHaveBeenCalled()
  })

  it('renders real media and emits the selected kind, prompt and model for creation', async () => {
    wrapper = mount(InspirationWall, { global: { stubs } })
    await openCommunity()
    expect(wrapper.get('.inspiration-media img').attributes('src')).toBe(publication.media_url)
    await wrapper.get('.inspiration-create').trigger('click')
    expect(wrapper.emitted('create')?.[0]).toEqual([{ kind: 'image', prompt: 'A forest', model: 'model-1' }])
  })

  it('uses server video/search filters and resets pagination', async () => {
    vi.useFakeTimers()
    wrapper = mount(InspirationWall, { global: { stubs } })
    await openCommunity()
    const video = wrapper.findAll('[role="radio"]').find(button => button.text() === 'Videos')!
    await video.trigger('click')
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith(expect.objectContaining({ kind: 'video', page: 1 }), expect.any(AbortSignal))
    await wrapper.get('input[type="search"]').setValue('forest')
    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith(expect.objectContaining({ search: 'forest', kind: 'video', page: 1 }), expect.any(AbortSignal))
  })

  it('only offers owner withdrawal and invokes the authenticated deletion after confirmation', async () => {
    api.list.mockResolvedValue({ items: [publication, { ...publication, id: 2, owner_user_id: 8 }], page: 1, pages: 1, total: 2, page_size: 24 })
    wrapper = mount(InspirationWall, { global: { stubs } })
    await openCommunity()
    expect(wrapper.findAll('[aria-label="Withdraw publication"]')).toHaveLength(1)
    await wrapper.get('[aria-label="Withdraw publication"]').trigger('click')
    expect(api.withdraw).not.toHaveBeenCalled()
    await wrapper.get('[data-confirm]').trigger('click')
    await flushPromises()
    expect(api.withdraw).toHaveBeenCalledWith(1)
  })

  it('shows an honest empty state without stock examples', async () => {
    api.list.mockResolvedValue({ items: [], page: 1, pages: 0, total: 0, page_size: 24 })
    wrapper = mount(InspirationWall, { global: { stubs } })
    await openCommunity()
    expect(wrapper.text()).toContain('No public works yet')
    expect(wrapper.findAll('img')).toHaveLength(0)
  })
})
