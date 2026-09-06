import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { creationModes, creationMode, creationPath, creationTitleKey } from '../navigation'

const chat = vi.hoisted(() => ({
  init: vi.fn(), selectSession: vi.fn(), createSession: vi.fn(),
  sessionModeFilter: 'image', selectedSession: { id: 1, mode: 'chat' },
  sessions: [{ id: 1, mode: 'chat' }], groupId: 1,
}))
const media = vi.hoisted(() => ({ applyInspiration: vi.fn() }))
vi.mock('../stores/chatWorkspace', () => ({ useChatWorkspace: () => chat }))
vi.mock('../stores/mediaWorkspace', () => ({ useMediaWorkspace: () => media }))
vi.mock('../components/ChatWorkspace.vue', () => ({ __esModule: true, default: { template: '<div data-view="chat" />' } }))
vi.mock('../components/MediaWorkspace.vue', () => ({ __esModule: true, default: { props: ['mode'], template: '<div :data-view="mode" />' } }))
vi.mock('../components/VoiceWorkspace.vue', () => ({ __esModule: true, default: { template: '<div data-view="voice" />' } }))
vi.mock('../components/InspirationWall.vue', () => ({ __esModule: true, default: { emits: ['create'], template: '<button data-view="gallery" @click="$emit(\'create\', { kind: \'video\', prompt: \'A moving landscape\' })">Create</button>' } }))
vi.mock('@/components/common/LocaleSwitcher.vue', () => ({ default: { template: '<span />' } }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div data-app-layout><slot /></div>' } }))
import StudioPage from '../StudioPage.vue'

async function open(path: string) {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/studio', redirect: to => { const { mode, ...query } = to.query; return { path: creationPath(creationMode(mode)), query } } },
    ...creationModes.map(mode => ({ path: creationPath(mode), component: StudioPage, meta: { creationMode: mode, titleKey: creationTitleKey(mode) } })),
    { path: '/:pathMatch(.*)*', component: { template: '<div />' } },
  ] })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(StudioPage, { global: { plugins: [router, createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false, messages: { en: {} } })] } })
  await flushPromises()
  return { router, wrapper }
}

describe('creation workspace shell', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    chat.init.mockResolvedValue(undefined)
    media.applyInspiration.mockResolvedValue(undefined)
    const auth = useAuthStore()
    auth.$patch({ user: { id: 42, username: 'Creator', email: 'creator@example.test', role: 'user', balance: 12 } as NonNullable<typeof auth.user>, token: 'test' })
  })

  it('opens images without creating or loading cloud conversations', async () => {
    const { wrapper } = await open('/studio?mode=image')
    expect(wrapper.find('[data-view="image"]').exists()).toBe(true)
    expect(chat.init).not.toHaveBeenCalled()
    expect(chat.createSession).not.toHaveBeenCalled()
    expect(wrapper.find('.studio-column-controls').exists()).toBe(false)
    expect(wrapper.find('[data-app-layout]').exists()).toBe(true)
    expect(wrapper.find('.creation-header').exists()).toBe(false)
    wrapper.unmount()
  })

  it('initializes chat only after navigating to chat and supports browser back', async () => {
    const { wrapper, router } = await open('/studio?mode=video')
    await router.push('/studio/chat')
    await flushPromises()
    expect(chat.init).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-view="chat"]').exists()).toBe(true)
    router.back()
    await flushPromises()
    expect(wrapper.find('[data-view="video"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('consumes prompt deep links as local drafts, without generation', async () => {
    const { wrapper, router } = await open('/studio?mode=image&prompt=Paint+a+city')
    expect(media.applyInspiration).toHaveBeenCalledWith({ kind: 'image', prompt: 'Paint a city' })
    expect(router.currentRoute.value.query.prompt).toBeUndefined()
    expect(chat.init).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps the gallery open when the local draft cannot be saved', async () => {
    media.applyInspiration.mockRejectedValueOnce(new Error('Local storage is full'))
    const { wrapper, router } = await open('/studio?mode=gallery')
    await wrapper.get('[data-view="gallery"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/studio/gallery')
    expect(wrapper.get('[role="alert"]').text()).toBe('Local storage is full')
    wrapper.unmount()
  })

  it.each(creationModes)('opens the %s menu route directly in the app layout', async mode => {
    const { wrapper, router } = await open(creationPath(mode))
    expect(router.currentRoute.value.meta.titleKey).toBe(creationTitleKey(mode))
    expect(wrapper.find(`[data-view="${mode}"]`).exists()).toBe(true)
    expect(wrapper.find('[data-app-layout]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('hands an inspiration prompt to the canonical video menu route', async () => {
    const { wrapper, router } = await open('/studio/gallery')
    await wrapper.get('[data-view="gallery"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/studio/video')
    expect(media.applyInspiration).toHaveBeenCalledWith({ kind: 'video', prompt: 'A moving landscape' })
    wrapper.unmount()
  })
})
