import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import type { SupportTicket } from '@/types'
import TicketDetailPane from '../TicketDetailPane.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => value,
}))

vi.mock('@/utils/tickets', () => ({
  getTicketStatusBadgeClass: () => 'status-badge',
}))

const ticketFactory = (overrides: Partial<SupportTicket> = {}): SupportTicket => ({
  id: 1,
  ticket_no: 'TK-1',
  user_id: 101,
  user_name: 'Alice',
  user_email: 'alice@example.com',
  user_avatar_url: 'https://cdn.example.com/alice.png',
  category: 'consult',
  title: 'Need help',
  status: 'submitted',
  current_form_payload: {},
  current_revision_no: 1,
  latest_message_at: '2026-07-08T00:00:00Z',
  last_reply_role: 'user',
  unread_by_user: false,
  unread_by_admin: true,
  created_at: '2026-07-08T00:00:00Z',
  updated_at: '2026-07-08T00:00:00Z',
  ...overrides,
})

describe('TicketDetailPane', () => {
  it('renders trusted user avatars with privacy-preserving attributes', () => {
    const wrapper = mount(TicketDetailPane, {
      props: {
        ticket: ticketFactory(),
        showUserMeta: true,
      },
      global: {
        stubs: {
          TicketCategoryForm: true,
          InfoItem: true,
        },
      },
    })

    const avatar = wrapper.get('img[alt="Alice"]')
    expect(avatar.attributes('src')).toBe('https://cdn.example.com/alice.png')
    expect(avatar.attributes('referrerpolicy')).toBe('no-referrer')
    expect(avatar.attributes('loading')).toBe('lazy')
  })

  it('does not render unsafe user avatar URLs', () => {
    const wrapper = mount(TicketDetailPane, {
      props: {
        ticket: ticketFactory({ user_avatar_url: 'javascript:alert(1)' }),
        showUserMeta: true,
      },
      global: {
        stubs: {
          TicketCategoryForm: true,
          InfoItem: true,
        },
      },
    })

    expect(wrapper.find('img[alt="Alice"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('A')
  })
})
