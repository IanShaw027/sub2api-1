import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { list, markRead } = vi.hoisted(() => ({
  list: vi.fn(),
  markRead: vi.fn(),
}))

vi.mock('@/api', () => ({
  announcementsAPI: {
    list,
    markRead,
  },
}))

import { useAnnouncementStore } from '@/stores/announcements'
import type { UserAnnouncement } from '@/types'

function announcement(id: number, read = false): UserAnnouncement {
  return {
    id,
    title: `Announcement ${id}`,
    content: `Content ${id}`,
    notify_mode: 'silent',
    read_at: read ? `2026-05-0${(id % 9) + 1}T00:00:00Z` : undefined,
    created_at: `2026-05-0${(id % 9) + 1}T00:00:00Z`,
    updated_at: `2026-05-0${(id % 9) + 1}T00:00:00Z`,
  }
}

describe('useAnnouncementStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    list.mockReset()
    markRead.mockReset()
  })

  it('keeps older hidden unread announcements counted after marking visible unread items as read', async () => {
    const store = useAnnouncementStore()
    const unreadAnnouncements = Array.from({ length: 25 }, (_, index) => announcement(index + 1, false))

    list.mockResolvedValueOnce(unreadAnnouncements)
    markRead.mockResolvedValue({ message: 'ok' })

    await store.fetchAnnouncements(true)

    expect(store.announcements).toHaveLength(20)
    expect(store.unreadTotal).toBe(25)

    await store.markAllAsRead()

    expect(store.announcements.every((item) => !!item.read_at)).toBe(true)
    expect(store.unreadTotal).toBe(5)
    expect(markRead).toHaveBeenCalledTimes(20)
  })
})
