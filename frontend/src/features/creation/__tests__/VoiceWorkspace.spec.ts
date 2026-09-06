import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive } from 'vue'
import { createI18n } from 'vue-i18n'
import VoiceWorkspace from '../components/VoiceWorkspace.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import type { VoiceCallOptions } from '../voiceCall'

const mocks = vi.hoisted(() => ({ groups: vi.fn(), speech: vi.fn(), transcribe: vi.fn() }))
const calls = vi.hoisted(() => ({ start: vi.fn(), stop: vi.fn(), mute: vi.fn() }))
const auth = reactive<{ user: { id: number } | null }>({ user: { id: 1 } })
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getAvailable: mocks.groups } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('../voiceCall', () => ({ VoiceCall: class {
  constructor(private readonly options: VoiceCallOptions) {}
  async start() { calls.start(); this.options.onStatus('connected') }
  stop() { calls.stop(); this.options.onStatus('ended') }
  setMuted(muted: boolean) { calls.mute(muted) }
} }))
vi.mock('../voiceApi', async (importOriginal) => ({
  ...await importOriginal<typeof import('../voiceApi')>(),
  generateSpeech: mocks.speech, transcribeAudio: mocks.transcribe,
}))

describe('VoiceWorkspace', () => {
  let wrapper: VueWrapper | undefined
  const createURL = vi.fn(() => 'blob:generated-audio')
  const revokeURL = vi.fn()

  beforeEach(() => {
    auth.user = { id: 1 }
    mocks.groups.mockReset().mockResolvedValue([
      { id: 10, name: 'OpenAI group', platform: 'openai' },
      { id: 20, name: 'Grok group', platform: 'grok' },
    ])
    mocks.speech.mockReset().mockResolvedValue(new Blob(['audio'], { type: 'audio/mpeg' }))
    mocks.transcribe.mockReset().mockResolvedValue('A real transcript')
    calls.start.mockClear(); calls.stop.mockClear(); calls.mute.mockClear()
    createURL.mockClear()
    revokeURL.mockClear()
    vi.stubGlobal('URL', class extends URL {
      static createObjectURL = createURL
      static revokeObjectURL = revokeURL
    })
  })

  afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.unstubAllGlobals() })

  async function open(mode: 'speech' | 'call' = 'speech') {
    wrapper = mount(VoiceWorkspace, {
      global: { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: {} } })] },
    })
    await flushPromises()
    if (mode !== 'call') {
      wrapper.getComponent(SegmentedControl).vm.$emit('update:modelValue', mode)
      await flushPromises()
    }
    return wrapper
  }

  it('uses authorized Grok groups, generates downloadable audio, and releases the blob', async () => {
    const view = await open()
    expect(view.text()).not.toContain('OpenAI group')
    await view.get('textarea').setValue('Hello')
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.speech).toHaveBeenCalledWith(20, { text: 'Hello', voice_id: 'eve', language: 'auto', speed: 1 }, expect.any(AbortSignal))
    expect(view.get('audio').attributes('src')).toBe('blob:generated-audio')
    expect(view.get('a[download]').attributes('download')).toBe('speech.mp3')
    view.unmount()
    wrapper = undefined
    expect(revokeURL).toHaveBeenCalledWith('blob:generated-audio')
  })

  it('does not expose a generation action without a Grok group', async () => {
    mocks.groups.mockResolvedValue([{ id: 10, name: 'OpenAI group', platform: 'openai' }])
    const view = await open()
    expect(view.text()).toContain('No Grok groups available')
    expect(view.find('form').exists()).toBe(false)
    expect(mocks.speech).not.toHaveBeenCalled()
  })

  it('uploads a selected audio file and renders its transcript', async () => {
    const view = await open()
    view.getComponent(SegmentedControl).vm.$emit('update:modelValue', 'transcription')
    await flushPromises()
    const file = new File(['audio'], 'recording.mp3', { type: 'audio/mpeg' })
    const input = view.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [file] })
    await input.trigger('change')
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.transcribe).toHaveBeenCalledWith(20, file, expect.any(AbortSignal))
    expect(view.text()).toContain('A real transcript')
  })

  it('discards an in-flight result after logout', async () => {
    let resolve!: (audio: Blob) => void
    mocks.speech.mockReturnValue(new Promise<Blob>((done) => { resolve = done }))
    const view = await open()
    await view.get('textarea').setValue('Private speech')
    await view.get('form').trigger('submit')
    const signal = mocks.speech.mock.calls[0][2] as AbortSignal
    auth.user = null
    expect(signal.aborted).toBe(true)
    resolve(new Blob(['private'], { type: 'audio/mpeg' }))
    await flushPromises()
    expect(view.find('audio').exists()).toBe(false)
    expect(createURL).not.toHaveBeenCalled()
  })

  it('shows upstream failures without creating a fake playable result', async () => {
    mocks.speech.mockRejectedValue(new Error('Voice upstream unavailable'))
    const view = await open()
    await view.get('textarea').setValue('Hello')
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(view.get('[role="alert"]').text()).toContain('Voice upstream unavailable')
    expect(view.find('audio').exists()).toBe(false)
    expect(view.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('starts a real call controller, mutes it, and hangs up on mode change', async () => {
    const view = await open('call')
    await view.get('button[aria-label="Start call"]').trigger('click')
    await flushPromises()
    expect(calls.start).toHaveBeenCalledOnce()
    expect(view.text()).toContain('Connected')
    await view.get('button[aria-label="Mute"]').trigger('click')
    expect(calls.mute).toHaveBeenCalledWith(true)
    expect(view.text()).toContain('Microphone muted')
    view.getComponent(SegmentedControl).vm.$emit('update:modelValue', 'speech')
    await flushPromises()
    expect(calls.stop).toHaveBeenCalledOnce()
    expect(view.find('button[aria-label="Hang up"]').exists()).toBe(false)
  })

  it('hangs up an active call when the authenticated identity changes', async () => {
    const view = await open('call')
    await view.get('button[aria-label="Start call"]').trigger('click')
    await flushPromises()
    auth.user = null
    expect(calls.stop).toHaveBeenCalledOnce()
  })
})
