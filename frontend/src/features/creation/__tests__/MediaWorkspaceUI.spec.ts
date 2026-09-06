import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import type { MediaTask } from '../localMedia'
import MediaWorkspace from '../components/MediaWorkspace.vue'
import MediaTaskTile from '../components/MediaTaskTile.vue'
import MediaParameters from '../components/MediaParameters.vue'
import { inspectSourceVideo } from '../mediaApi'

const state = vi.hoisted(() => ({
  tasks: [] as MediaTask[], groups: [{ id: 1, name: 'Image group', platform: 'openai' }],
  imageModels: ['gpt-image-2'], videoModels: ['grok-imagine-video'], groupId: 1,
  loading: false, modelsLoading: false, error: null,
  init: vi.fn(), loadDraft: vi.fn(), saveDraft: vi.fn(), loadModels: vi.fn(), submit: vi.fn(),
  retry: vi.fn(), resume: vi.fn(), saveTask: vi.fn(), deleteTasks: vi.fn(), getTaskBlob: vi.fn(), clearError: vi.fn(),
}))
vi.mock('../stores/mediaWorkspace', () => ({ useMediaWorkspace: () => state }))
vi.mock('../mediaApi', () => ({
  inspectSourceVideo: vi.fn(async () => 5),
  mediaAPI: { quote: vi.fn(async () => ({ status: 'unavailable', currency: 'USD', amount: null, billing_target: null })) },
  unavailableMediaCost: () => ({ status: 'unavailable', currency: 'USD', amount: null, billing_target: null }),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 1 }, isAuthenticated: true }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const { ref } = await import('vue')
  return {
    ...actual,
    useI18n: (options?: { messages?: { en?: Record<string, unknown> } }) => ({
      locale: ref('en'),
      t: (key: string) => typeof options?.messages?.en?.[key] === 'string' ? options.messages.en[key] : key,
    }),
  }
})

function i18n() { return createI18n({ legacy: false, locale: 'en', messages: { en: {} } }) }
function task(overrides: Partial<MediaTask> = {}): MediaTask {
  return { id: 'one', userId: 1, mode: 'image', type: 'generation', prompt: 'A red landscape', model: 'gpt-image-2', groupId: 1, settings: { size: '1024x1024' }, status: 'completed', mediaUrl: 'blob:one', createdAt: '2026-09-06T00:00:00Z', completedAt: '2026-09-06T00:00:10Z', sourceFiles: [], persisted: true, ...overrides }
}
const stubs = {
  UiSelect: true, MediaParameters: true, MediaPreview: true, LegacyImageHistory: true, PublishMediaDialog: true,
  UiDrawer: { props: ['open'], template: '<div v-if="open"><slot /></div>' },
  UiModal: { props: ['open'], template: '<div v-if="open"><slot /><slot name="footer" /></div>' },
}

beforeEach(() => {
  vi.clearAllMocks()
  state.tasks = []
  state.loadDraft.mockResolvedValue(null)
  state.init.mockResolvedValue(undefined)
  state.loadModels.mockResolvedValue(undefined)
  state.saveDraft.mockResolvedValue(undefined)
  state.submit.mockResolvedValue(task())
})
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('media workspace', () => {
  it('restores an extension draft with its local file and never silently changes to generation', async () => {
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:source'), revokeObjectURL: vi.fn() }))
    const file = new File(['video'], 'source.mp4', { type: 'video/mp4' })
    state.loadDraft.mockResolvedValue({ prompt: 'Continue this scene', videoOperation: 'extension', model: 'grok-imagine-video', groupId: 1, settings: { duration: 7 }, files: [file], parentId: 'original' })
    const wrapper = mount(MediaWorkspace, { props: { mode: 'video' }, global: { plugins: [i18n()], stubs } })
    try {
      await flushPromises()
      expect(wrapper.get('.media-operation-segments [aria-checked="true"]').text()).toBe('Extend video')
      expect(wrapper.get('input[type="file"]').attributes('accept')).toContain('video/mp4')
      expect(wrapper.find('.media-source video').exists()).toBe(true)
      await wrapper.get('.media-submit').trigger('click')
      await flushPromises()
      expect(state.submit).toHaveBeenCalledWith(expect.objectContaining({ videoOperation: 'extension', files: [file], settings: { duration: 7, n: 1 }, parentId: 'original' }))
    } finally { wrapper.unmount() }
  })

  it('uses a completed local video Blob as an editing source without publishing or generating', async () => {
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:source'), revokeObjectURL: vi.fn() }))
    const blob = new Blob(['video'], { type: 'video/mp4' })
    const original = task({ mode: 'video', blob, model: 'grok-imagine-video', settings: { duration: 5, resolution: '720p' } })
    state.tasks = [original]
    state.getTaskBlob.mockResolvedValue(blob)
    const wrapper = mount(MediaWorkspace, { props: { mode: 'video' }, global: { plugins: [i18n()], stubs } })
    try {
      await flushPromises()
      wrapper.findComponent(MediaTaskTile).vm.$emit('action', 'editVideo', original)
      await flushPromises()
      expect(inspectSourceVideo).toHaveBeenCalledWith(blob, 'edit')
      expect(wrapper.get('.media-operation-segments [aria-checked="true"]').text()).toBe('Edit video')
      expect(state.saveDraft).toHaveBeenCalledWith('video', expect.objectContaining({ videoOperation: 'edit', parentId: 'one', settings: {}, files: [expect.any(File)] }))
      expect(state.submit).not.toHaveBeenCalled()
      await wrapper.get('textarea').setValue('Change the lighting')
      await wrapper.get('.media-submit').trigger('click')
      expect(state.submit).toHaveBeenCalledWith(expect.objectContaining({ videoOperation: 'edit', parentId: 'one' }))
    } finally { wrapper.unmount() }
  })

  it('clears incompatible image sources when switching from generation to video editing', async () => {
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:source'), revokeObjectURL: vi.fn() }))
    state.loadDraft.mockResolvedValue({ prompt: 'A scene', model: 'grok-imagine-video', groupId: 1, files: [new File(['image'], 'source.png', { type: 'image/png' })], settings: { duration: 5, resolution: '720p' } })
    const wrapper = mount(MediaWorkspace, { props: { mode: 'video' }, global: { plugins: [i18n()], stubs } })
    try {
      await flushPromises()
      await wrapper.findAll('.media-operation-segments button')[1]!.trigger('click')
      expect(wrapper.find('.media-source').exists()).toBe(false)
      expect(wrapper.get('.media-submit').attributes('disabled')).toBeDefined()
      expect(state.submit).not.toHaveBeenCalled()
    } finally { wrapper.unmount() }
  })

  it('searches real local tasks and switches grid/list without modifying stored tasks', async () => {
    state.tasks = [task(), task({ id: 'two', prompt: 'A blue building' })]
    const wrapper = mount(MediaWorkspace, { props: { mode: 'image' }, global: { plugins: [i18n()], stubs } })
    try {
      await flushPromises()
      expect(wrapper.findAll('[data-task-id]')).toHaveLength(2)
      await wrapper.get('input[type="search"]').setValue('blue')
      expect(wrapper.findAll('[data-task-id]')).toHaveLength(1)
      await wrapper.get('[aria-label="List view"]').trigger('click')
      expect(wrapper.get('.media-workspace-grid').classes()).toContain('is-list')
      expect(state.tasks).toHaveLength(2)
    } finally { wrapper.unmount() }
  })

  it('submits a local task with displayed model and settings only on explicit generation', async () => {
    const wrapper = mount(MediaWorkspace, { props: { mode: 'video' }, global: { plugins: [i18n()], stubs } })
    try {
      await flushPromises()
      expect(state.submit).not.toHaveBeenCalled()
      await wrapper.get('textarea').setValue('A moving landscape')
      await wrapper.get('.media-submit').trigger('click')
      await flushPromises()
      expect(state.submit).toHaveBeenCalledWith(expect.objectContaining({ mode: 'video', prompt: 'A moving landscape', model: 'grok-imagine-video', groupId: 1, settings: { duration: 5, resolution: '720p', n: 1 } }))
      expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    } finally { wrapper.unmount() }
  })

  it('routes failed accepted tasks to status recovery instead of another paid generation', async () => {
    state.tasks = [task({ status: 'error', providerTaskId: 'accepted-task', error: 'Temporary polling failure' })]
    const wrapper = mount(MediaWorkspace, { props: { mode: 'image' }, global: { plugins: [i18n()], stubs } })
    try {
      await flushPromises()
      const resume = wrapper.findAll('button').find(button => button.text().includes('Resume status check'))
      await resume!.trigger('click')
      expect(state.resume).toHaveBeenCalledWith('one')
      expect(state.retry).not.toHaveBeenCalled()
    } finally { wrapper.unmount() }
  })

  it('confirms batch deletion and deletes only selected local ids', async () => {
    state.tasks = [task(), task({ id: 'two' })]
    const wrapper = mount(MediaWorkspace, { props: { mode: 'image' }, global: { plugins: [i18n()], stubs } })
    try {
      await flushPromises()
      await wrapper.get('.media-workspace-tools [aria-label="Select"]').trigger('click')
      await wrapper.get('[aria-label="Select all"]').trigger('click')
      await wrapper.get('[aria-label="Delete selected"]').trigger('click')
      expect(state.deleteTasks).not.toHaveBeenCalled()
      const remove = wrapper.findAll('button').find(button => button.text() === 'Delete')
      await remove!.trigger('click')
      expect(state.deleteTasks).toHaveBeenCalledWith(['one', 'two'])
    } finally { wrapper.unmount() }
  })
})

describe('media task actions', () => {
  it('offers explicit local save recovery for an unsaved result', async () => {
    const wrapper = mount(MediaTaskTile, { props: { task: task({ persisted: false }), now: Date.now() }, global: { plugins: [i18n()] } })
    await wrapper.get('.media-save-local').trigger('click')
    expect(wrapper.emitted('action')?.[0]?.[0]).toBe('save')
    wrapper.unmount()
  })
})

describe('media parameter contracts', () => {
  it('clears Grok aspect_ratio when switching to GPT and rounds actual dimensions to multiples of 16', async () => {
    const wrapper = mount(MediaParameters, { props: { mode: 'image', model: 'grok-imagine-image', settings: { ratio: '3:2', aspectRatio: '3:2', resolution: '1K' } }, global: { plugins: [i18n()] } })
    await wrapper.setProps({ model: 'gpt-image-2' })
    const update = wrapper.emitted('update:settings')?.at(-1)?.[0] as Record<string, unknown>
    expect(update.aspectRatio).toBeUndefined()
    expect(update.size).toBe('1024x688')
    wrapper.unmount()
  })

  it('does not carry wide DALL-E 3 settings into DALL-E 2', async () => {
    const wrapper = mount(MediaParameters, { props: { mode: 'image', model: 'dall-e-3', settings: { ratio: '7:4', quality: 'hd', size: '1792x1024' } }, global: { plugins: [i18n()] } })
    await wrapper.setProps({ model: 'dall-e-2' })
    expect(wrapper.emitted('update:settings')?.at(-1)?.[0]).toMatchObject({ ratio: '1:1', quality: 'standard', size: '1024x1024' })
    wrapper.unmount()
  })
})
