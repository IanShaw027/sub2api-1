import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { User } from '@/types'
import { useAuthStore } from '@/stores/auth'
import type { CreationMessage, CreationSession } from '../types'

const { listSessions, listSessionMessages, importLegacyConversation, createObjectURL, revokeObjectURL } = vi.hoisted(() => ({
  listSessions: vi.fn(), listSessionMessages: vi.fn(), importLegacyConversation: vi.fn(),
  createObjectURL: vi.fn(), revokeObjectURL: vi.fn(),
}))
vi.mock('../api', () => ({ listSessions, listSessionMessages }))
vi.mock('../stores/chatWorkspace', () => ({ useChatWorkspace: () => ({ importLegacyConversation }) }))
vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const auth = reactive({ user: { id: 7 } })
  return { useAuthStore: () => auth }
})
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'en' } }) }))

import LegacyChatHistory from '../components/LegacyChatHistory.vue'

const session: CreationSession = {
  id: 21, user_id: 7, group_id: 3, title: 'Archived code review', model: 'gpt-5', mode: 'chat', status: 'archived',
  metadata: { attachments: [{ name: 'notes.txt', content: 'original notes' }] },
  created_at: '2026-08-01T12:00:00Z', updated_at: '2026-08-02T12:00:00Z',
}
const messages: CreationMessage[] = [
  { id: 1, session_id: 21, role: 'system', content: 'Preserve system context', created_at: session.created_at },
  {
    id: 2, session_id: 21, role: 'user', model: 'gpt-5',
    content: [{ type: 'text', text: 'Review this' }, { type: 'image_url', image_url: { url: 'https://storage.test/private-reference.png' } }],
    input_tokens: 42, output_tokens: null, created_at: session.created_at,
  },
  { id: 3, session_id: 21, role: 'assistant', model: 'gpt-5', content: { text: 'Result', attachments: [{ id: 'file-1' }] }, input_tokens: 42, output_tokens: 17, created_at: session.updated_at },
]
const NativeURL = URL

function mountHistory(open = true) {
  return mount(LegacyChatHistory, {
    props: { open },
    global: { stubs: {
      UiModal: { props: ['open', 'title', 'subtitle'], template: '<section v-if="open"><h2>{{ title }}</h2><p>{{ subtitle }}</p><slot /><slot name="footer" /></section>' },
    } },
  })
}

async function clickImport(wrapper: ReturnType<typeof mountHistory>) {
  await wrapper.findAll('button').find((button) => button.text().includes('Import locally and continue'))!.trigger('click')
}

function readBlob(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error)
    reader.readAsText(blob)
  })
}

beforeEach(() => {
  vi.resetAllMocks()
  useAuthStore().user = { id: 7 } as User
  localStorage.setItem('auth_user', '{"id":7}')
  listSessions.mockResolvedValue({ items: [session], total: 21, page: 1, page_size: 20 })
  listSessionMessages.mockResolvedValue(messages)
  importLegacyConversation.mockResolvedValue({ id: 'local-chat-21' })
  createObjectURL.mockReturnValue('blob:test-export')
  vi.stubGlobal('URL', class extends NativeURL {
    static createObjectURL = createObjectURL
    static revokeObjectURL = revokeObjectURL
  })
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
})
afterEach(() => {
  if (vi.isFakeTimers()) {
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
  }
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  localStorage.clear()
})

describe('LegacyChatHistory explicit import', () => {
  it('does not access cloud history or migrate anything until opened', async () => {
    const wrapper = mountHistory(false)
    await flushPromises()
    expect(listSessions).not.toHaveBeenCalled()
    await wrapper.setProps({ open: true })
    await flushPromises()
    expect(listSessions).toHaveBeenCalledWith({ mode: 'chat', page: 1, page_size: 20 })
    expect(wrapper.text()).toContain('Private history')
    expect(listSessionMessages).not.toHaveBeenCalled()
    expect(importLegacyConversation).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('searches titles and models on the current page and paginates on demand', async () => {
    const wrapper = mountHistory()
    await flushPromises()
    await wrapper.get('input[type="search"]').setValue('GPT-5')
    expect(wrapper.find('[data-legacy-chat-id="21"]').exists()).toBe(true)
    await wrapper.get('input[type="search"]').setValue('missing')
    expect(wrapper.text()).toContain('No matching conversations on this page')
    await wrapper.get('button[aria-label="Next page"]').trigger('click')
    await flushPromises()
    expect(listSessions).toHaveBeenLastCalledWith({ mode: 'chat', page: 2, page_size: 20 })
    wrapper.unmount()
  })

  it('passes the complete session and raw messages to the local store only after explicit import', async () => {
    const wrapper = mountHistory()
    await flushPromises()
    await clickImport(wrapper)
    await flushPromises()
    expect(listSessionMessages).toHaveBeenCalledWith(21)
    expect(importLegacyConversation).toHaveBeenCalledWith({ session, messages, signal: expect.any(AbortSignal) })
    expect(wrapper.emitted('import')).toEqual([['local-chat-21']])
    const imported = wrapper.findAll('button').find((button) => button.text().includes('Imported locally'))!
    expect(imported.attributes('disabled')).toBeDefined()
    expect(createObjectURL).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('exports original roles, structured contents, models, tokens, attachments, and dates as JSON', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout'] })
    const wrapper = mountHistory()
    await flushPromises()
    await wrapper.get('button[aria-label="Download JSON"]').trigger('click')
    await flushPromises()
    const blob = createObjectURL.mock.calls[0][0] as Blob
    expect(blob.type).toBe('application/json')
    const exported = JSON.parse(await readBlob(blob))
    expect(exported).toEqual({
      format: 'sub2api-legacy-chat', version: 1, exported_at: expect.any(String), session, messages,
    })
    expect(HTMLAnchorElement.prototype.click).toHaveBeenCalledOnce()
    expect(importLegacyConversation).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('discards messages returned after close and reopening', async () => {
    let resolveOld!: (value: CreationMessage[]) => void
    listSessionMessages.mockImplementationOnce(() => new Promise((resolve) => { resolveOld = resolve }))
    const wrapper = mountHistory()
    await flushPromises()
    await clickImport(wrapper)
    await wrapper.setProps({ open: false })
    await wrapper.setProps({ open: true })
    await flushPromises()
    resolveOld(messages)
    await flushPromises()
    expect(importLegacyConversation).not.toHaveBeenCalled()
    expect(wrapper.emitted('import')).toBeUndefined()
    wrapper.unmount()
  })

  it('clears visible private history when the authenticated owner changes', async () => {
    const wrapper = mountHistory()
    await flushPromises()
    localStorage.setItem('auth_user', '{"id":8}')
    useAuthStore().user = { id: 8 } as User
    await flushPromises()
    expect(wrapper.find('[data-legacy-chat-id="21"]').exists()).toBe(false)
    expect(wrapper.get('[role="alert"]').text()).toContain('account changed')
    expect(listSessions).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('blocks a user switch while a message fetch is in flight', async () => {
    listSessionMessages.mockImplementation(async () => {
      localStorage.setItem('auth_user', '{"id":8}')
      return messages
    })
    const wrapper = mountHistory()
    await flushPromises()
    await clickImport(wrapper)
    await flushPromises()
    expect(importLegacyConversation).not.toHaveBeenCalled()
    expect(createObjectURL).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('account changed')
    wrapper.unmount()
  })

  it('rejects another owner session and mismatched message session IDs', async () => {
    listSessions.mockResolvedValueOnce({ items: [{ ...session, user_id: 8 }], total: 1, page: 1, page_size: 20 })
    const wrapper = mountHistory()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('does not belong')
    expect(wrapper.find('[data-legacy-chat-id="21"]').exists()).toBe(false)
    await wrapper.get('button[aria-label="Refresh history"]').trigger('click')
    await flushPromises()
    listSessionMessages.mockResolvedValueOnce([{ ...messages[0], session_id: 99 }])
    await clickImport(wrapper)
    await flushPromises()
    expect(importLegacyConversation).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('do not match')
    wrapper.unmount()
  })

  it('does not claim success when IndexedDB import fails', async () => {
    importLegacyConversation.mockRejectedValue(new Error('QuotaExceededError'))
    const wrapper = mountHistory()
    await flushPromises()
    await clickImport(wrapper)
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('original cloud conversation is unchanged')
    expect(wrapper.emitted('import')).toBeUndefined()
    expect(wrapper.text()).not.toContain('Imported locally')
    wrapper.unmount()
  })

  it('suppresses import completion events after the dialog closes', async () => {
    let finish!: (value: { id: string }) => void
    importLegacyConversation.mockImplementationOnce(() => new Promise((resolve) => { finish = resolve }))
    const wrapper = mountHistory()
    await flushPromises()
    await clickImport(wrapper)
    await flushPromises()
    await wrapper.setProps({ open: false })
    expect(importLegacyConversation.mock.calls[0][0].signal.aborted).toBe(true)
    finish({ id: 'local-chat-21' })
    await flushPromises()
    expect(wrapper.emitted('import')).toBeUndefined()
    wrapper.unmount()
  })
})
