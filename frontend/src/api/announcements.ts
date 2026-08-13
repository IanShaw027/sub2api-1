/**
 * User Announcements API endpoints
 */

import { apiClient } from './client'
import type { AnnouncementReadStatusFilter, UserAnnouncement } from '@/types'

export async function list(readStatus: AnnouncementReadStatusFilter = 'all'): Promise<UserAnnouncement[]> {
  const { data } = await apiClient.get<UserAnnouncement[]>('/announcements', {
    params: readStatus === 'all' ? {} : { read_status: readStatus }
  })
  return data
}

export async function markRead(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/announcements/${id}/read`)
  return data
}

const announcementsAPI = {
  list,
  markRead
}

export default announcementsAPI

