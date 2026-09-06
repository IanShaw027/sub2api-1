import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { reactive } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { useMediaWorkspace } from '../stores/mediaWorkspace'
import { localMedia, MediaStorageError, type MediaDraft, type MediaRequest, type MediaTask } from '../localMedia'
import { inspectSourceVideo, mediaAPI } from '../mediaApi'

vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getAvailable: vi.fn(async () => [
  { id: 10, name: 'Images', platform: 'openai' },
  { id: 20, name: 'Video', platform: 'grok' },
  { id: 30, name: 'Chat', platform: 'anthropic' },
  { id: 40, name: 'Composite', platform: 'composite' },
]) } }))
vi.mock('../localMedia', async importOriginal => ({
  ...await importOriginal<typeof import('../localMedia')>(),
  localMedia: {
    listTasks: vi.fn(), saveTask: vi.fn(), deleteTasks: vi.fn(), loadDraft: vi.fn(), saveDraft: vi.fn(),
  },
}))
vi.mock('../mediaApi', () => ({
  inspectSourceVideo: vi.fn(async () => 5),
  imageJobBlob: vi.fn(() => ({ blob: new Blob(['image'], { type: 'image/png' }), revisedPrompt: 'Revised' })),
  mediaAPI: { getModels: vi.fn(), submit: vi.fn(), poll: vi.fn(), videoBlob: vi.fn(), errorMessage: vi.fn(() => 'Generation failed') },
}))

const auth = reactive({ user: { id: 1 }, isAuthenticated: true })
const records = new Map<string, MediaTask>()
const drafts = new Map<string, MediaDraft>()
let store: ReturnType<typeof useMediaWorkspace>
const input = (): MediaRequest => ({ mode: 'image', prompt: 'A test image', model: 'gpt-image-1', groupId: 10, settings: {} })
const existing = (overrides: Partial<MediaTask> = {}): MediaTask => ({
  id: 'saved', userId: 1, mode: 'image', type: 'generation', model: 'gpt-image-1', prompt: 'Saved prompt',
  groupId: 10, settings: {}, status: 'completed', sourceFiles: [], persisted: true,
  blob: new Blob(['stored'], { type: 'image/png' }), createdAt: '2026-09-01T00:00:00Z', ...overrides,
})

beforeEach(() => {
  vi.clearAllMocks()
  records.clear()
  drafts.clear()
  auth.user = { id: 1 }
  auth.isAuthenticated = true
  vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => `blob:${Math.random()}`), revokeObjectURL: vi.fn() }))
  vi.mocked(localMedia.listTasks).mockImplementation(async owner => [...records.values()].filter(task => task.userId === owner).map(task => ({ ...task })))
  vi.mocked(localMedia.saveTask).mockImplementation(async task => { records.set(`${task.userId}:${task.id}`, { ...task, mediaUrl: undefined, persisted: true }) })
  vi.mocked(localMedia.deleteTasks).mockImplementation(async (owner, ids) => { ids.forEach(id => records.delete(`${owner}:${id}`)) })
  vi.mocked(localMedia.loadDraft).mockImplementation(async (owner, mode) => drafts.get(`${owner}:${mode}`) ?? null)
  vi.mocked(localMedia.saveDraft).mockImplementation(async draft => { drafts.set(`${draft.userId}:${draft.mode}`, draft) })
  vi.mocked(mediaAPI.getModels).mockResolvedValue({ data: [{ id: 'gpt-image-1' }, { id: 'grok-imagine-video' }, { id: 'gpt-chat' }] })
  vi.mocked(mediaAPI.submit).mockResolvedValue({ task_id: 'upstream-1', status: 'completed', result: { data: [{ b64_json: 'aW1hZ2U=' }] } })
  vi.mocked(mediaAPI.poll).mockResolvedValue({ task_id: 'upstream-1', status: 'completed' })
  setActivePinia(createPinia())
  store = useMediaWorkspace()
})

afterEach(() => {
  store.$dispose()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('local media workspace store', () => {
  it.each(['edit', 'extension'] as const)('persists and retries video %s with its source and operation', async operation => {
    const file = new File(['source'], 'source.mp4', { type: 'video/mp4' })
    vi.mocked(mediaAPI.videoBlob).mockResolvedValue(new Blob(['video'], { type: 'video/mp4' }))
    const task = await store.submit({ mode: 'video', videoOperation: operation, prompt: 'Continue', model: 'grok-imagine-video', groupId: 20, files: [file], settings: { duration: 6, resolution: '720p', aspectRatio: '16:9' } })
    await flushPromises()
    expect(inspectSourceVideo).toHaveBeenCalledWith(file, operation)
    expect(task).toMatchObject({ videoOperation: operation, sourceDuration: 5, settings: operation === 'edit' ? {} : { duration: 6 } })
    expect(task.settings).not.toHaveProperty('resolution')
    expect(task.sourceFiles[0]).toBe(file)
    const retry = await store.retry(task.id)
    await flushPromises()
    expect(retry.parentId).toBe(task.id)
    expect(retry.videoOperation).toBe(operation)
    expect(mediaAPI.submit).toHaveBeenLastCalledWith(expect.objectContaining({ videoOperation: operation, files: [file] }), expect.any(AbortSignal))
  })

  it('restores video extension polling without submitting generation or reading the source again', async () => {
    const file = new File(['source'], 'source.mp4', { type: 'video/mp4' })
    records.set('1:running', existing({ id: 'running', mode: 'video', videoOperation: 'extension', groupId: 20, settings: { duration: 6 }, sourceFiles: [file], blob: undefined, status: 'generating', providerTaskId: 'accepted-extension' }))
    vi.mocked(mediaAPI.videoBlob).mockResolvedValue(new Blob(['video'], { type: 'video/mp4' }))
    await store.init()
    await flushPromises()
    expect(mediaAPI.submit).not.toHaveBeenCalled()
    expect(inspectSourceVideo).not.toHaveBeenCalled()
    expect(mediaAPI.poll).toHaveBeenCalledWith('video', 20, 'accepted-extension', expect.any(AbortSignal))
    expect(store.tasks[0]).toMatchObject({ status: 'completed', videoOperation: 'extension', sourceFiles: [file] })
  })

  it('keeps legacy image-to-video retries on generation despite legacy edit type', async () => {
    const file = new File(['image'], 'source.png', { type: 'image/png' })
    records.set('1:saved', existing({ mode: 'video', type: 'edit', groupId: 20, sourceFiles: [file] }))
    await store.init()
    const retry = await store.retry('saved')
    expect(retry.videoOperation).toBe('generation')
    expect(inspectSourceVideo).not.toHaveBeenCalled()
  })

  it('rejects invalid extension parameters before persistence or a paid request', async () => {
    await expect(store.submit({ mode: 'video', videoOperation: 'extension', prompt: 'Continue', model: 'grok-imagine-video', groupId: 20, files: [new File(['video'], 'source.mp4')], settings: { duration: 15 } })).rejects.toThrow('between 2 and 10')
    expect(mediaAPI.submit).not.toHaveBeenCalled()
    expect(localMedia.saveTask).not.toHaveBeenCalled()
  })

  it('saves source files before generation, then stores the result without a cloud session', async () => {
    const file = new File(['source'], 'source.png', { type: 'image/png' })
    const task = await store.submit({ ...input(), files: [file] })
    expect(vi.mocked(localMedia.saveTask).mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(mediaAPI.submit).mock.invocationCallOrder[0]!)
    await flushPromises()
    expect(task.status).toBe('completed')
    expect(task.persisted).toBe(true)
    expect(task.mediaUrl).toMatch(/^blob:/)
    expect(records.get(`1:${task.id}`)?.sourceFiles[0]).toBe(file)
    expect(records.get(`1:${task.id}`)?.blob?.size).toBe(5)
    expect(mediaAPI.submit).toHaveBeenCalledWith(expect.objectContaining({ files: [file], groupId: 10 }), expect.any(AbortSignal))
  })

  it('does not start a paid generation when local preflight storage fails', async () => {
    vi.mocked(localMedia.saveTask).mockRejectedValue(new MediaStorageError('quota'))
    await expect(store.submit(input())).rejects.toThrow('Browser storage is full')
    expect(mediaAPI.submit).not.toHaveBeenCalled()
    expect(store.tasks).toHaveLength(0)
    expect(store.error).toContain('not saved')
  })

  it('retains a downloadable result and marks it unsaved when final Blob persistence fails', async () => {
    const save = vi.mocked(localMedia.saveTask).getMockImplementation()!
    vi.mocked(localMedia.saveTask).mockImplementation(async task => {
      if (task.blob) throw new MediaStorageError('quota')
      return save(task)
    })
    const task = await store.submit(input())
    await flushPromises()
    expect(task.status).toBe('completed')
    expect(task.persisted).toBe(false)
    expect((await store.getTaskBlob(task.id)).size).toBe(5)
    expect(store.error).toContain('Browser storage is full')
    vi.mocked(localMedia.saveTask).mockImplementation(save)
    await store.saveTask(task.id)
    expect(task.persisted).toBe(true)
  })

  it('recreates result URLs and resumes remote polling after reload without submitting again', async () => {
    records.set('1:saved', existing())
    records.set('1:running', existing({ id: 'running', blob: undefined, status: 'generating', providerTaskId: 'upstream-1' }))
    await store.init()
    await flushPromises()
    expect(store.tasks.find(task => task.id === 'saved')?.mediaUrl).toMatch(/^blob:/)
    expect(mediaAPI.poll).toHaveBeenCalledWith('image', 10, 'upstream-1', expect.any(AbortSignal))
    expect(mediaAPI.submit).not.toHaveBeenCalled()
  })

  it('marks a reload-interrupted submission as an error instead of automatically charging again', async () => {
    records.set('1:unknown', existing({ id: 'unknown', blob: undefined, status: 'generating' }))
    await store.init()
    expect(store.tasks[0]?.status).toBe('error')
    expect(store.tasks[0]?.error).toContain('new paid request')
    expect(mediaAPI.submit).not.toHaveBeenCalled()
  })

  it('aborts polling and revokes URLs on logout and rejects stale completions', async () => {
    let resolveJob!: (job: { status: string }) => void
    vi.mocked(mediaAPI.poll).mockImplementation(() => new Promise(resolve => { resolveJob = resolve }))
    records.set('1:saved', existing())
    records.set('1:running', existing({ id: 'running', blob: undefined, status: 'generating', providerTaskId: 'pending' }))
    await store.init()
    const signal = vi.mocked(mediaAPI.poll).mock.calls[0]![3]
    auth.isAuthenticated = false
    expect(signal.aborted).toBe(true)
    expect(URL.revokeObjectURL).toHaveBeenCalled()
    expect(store.tasks).toHaveLength(0)
    resolveJob({ status: 'completed' })
    await flushPromises()
    expect(store.tasks).toHaveLength(0)
    expect(records.get('1:running')?.status).toBe('generating')
  })

  it('does not reset tasks on a same-user auth refresh and isolates a different user', async () => {
    records.set('1:saved', existing())
    await store.init()
    auth.user = { id: 1 }
    expect(store.tasks).toHaveLength(1)
    auth.user = { id: 2 }
    expect(store.tasks).toHaveLength(0)
    await store.init()
    await expect(store.getTaskBlob('saved')).rejects.toThrow('active account')
    expect(localMedia.listTasks).toHaveBeenLastCalledWith(2)
  })

  it('keeps independent per-user image and video drafts including source files', async () => {
    const source = new File(['source'], 'source.png', { type: 'image/png' })
    await store.saveDraft('image', { ...input(), files: [source] })
    await store.saveDraft('video', { ...input(), prompt: 'Animate', settings: { duration: 5 } })
    expect((await store.loadDraft('image'))?.files).toEqual([source])
    expect((await store.loadDraft('video'))?.prompt).toBe('Animate')
    auth.user = { id: 2 }
    expect(await store.loadDraft('image')).toBeNull()
  })

  it('preserves source files and ancestry for retry and deletes only the selected local tasks', async () => {
    const source = new File(['source'], 'source.png', { type: 'image/png' })
    records.set('1:saved', existing({ sourceFiles: [source] }))
    await store.init()
    const retry = await store.retry('saved')
    await flushPromises()
    expect(retry.parentId).toBe('saved')
    expect(store.getSourceFiles(retry.id)).toEqual([source])
    await store.deleteTasks([retry.id])
    expect(store.tasks.map(task => task.id)).toEqual(['saved'])
    expect(localMedia.deleteTasks).toHaveBeenCalledWith(1, [retry.id])
  })

  it('imports old images once and applies inspiration without discarding saved draft sources', async () => {
    const blob = new Blob(['legacy'], { type: 'image/png' })
    const first = await store.importExistingImage({ blob, prompt: 'Legacy', model: 'gpt-image-1', groupId: 10, legacyId: 4 })
    const second = await store.importExistingImage({ blob, prompt: 'Legacy', model: 'gpt-image-1', groupId: 10, legacyId: 4 })
    expect(first.id).toBe(second.id)
    expect(mediaAPI.submit).not.toHaveBeenCalled()
    await store.saveDraft('image', { ...input(), parentId: first.id, settings: { quality: 'high' } })
    await store.applyInspiration({ kind: 'image', prompt: 'A new idea' })
    expect(await store.loadDraft('image')).toMatchObject({ prompt: 'A new idea', parentId: first.id, settings: { quality: 'high' } })
  })

  it('keeps a failed local deletion recoverable after stopping an in-flight poll', async () => {
    vi.mocked(mediaAPI.poll).mockImplementation(() => new Promise(() => {}))
    records.set('1:running', existing({ id: 'running', blob: undefined, status: 'generating', providerTaskId: 'pending' }))
    await store.init()
    vi.mocked(localMedia.deleteTasks).mockRejectedValueOnce(new MediaStorageError('write'))
    await expect(store.deleteTasks(['running'])).rejects.toThrow('Could not save')
    expect(store.tasks[0]).toMatchObject({ status: 'error', providerTaskId: 'pending' })
    expect(store.tasks[0]?.error).toContain('Resume querying')
    store.resume('running')
    expect(mediaAPI.poll).toHaveBeenCalledTimes(2)
    expect(mediaAPI.submit).not.toHaveBeenCalled()
  })

  it('creates task IDs when randomUUID is unavailable in an HTTP browser context', async () => {
    vi.stubGlobal('crypto', {})
    const task = await store.submit(input())
    await flushPromises()
    expect(task.id).toMatch(/^\d+-[a-z0-9]+$/)
    expect(task.status).toBe('completed')
  })

  it('derives available media models from actual group API results', async () => {
    await store.init()
    expect(store.groups.map(group => group.id)).toEqual([10, 20, 40])
    expect(store.imageModels).toEqual(['gpt-image-1'])
    expect(store.videoModels).toEqual([])
    await store.loadModels(20)
    expect(store.videoModels).toEqual(['grok-imagine-video'])
    await store.loadModels(40)
    expect(store.imageModels).toEqual(['gpt-image-1'])
    expect(store.videoModels).toEqual(['grok-imagine-video'])
  })
})
