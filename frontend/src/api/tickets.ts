import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import type { SupportTicket, SupportTicketMessage, TicketRateGroupOption } from '@/types/ticket'

export const ticketsAPI = {
  create(data: { category: string; title: string; form_payload: Record<string, unknown> }) {
    return apiClient.post<SupportTicket>('/tickets', data)
  },
  list(params?: { page?: number; page_size?: number; status?: string; category?: string; keyword?: string; unread_only?: boolean }) {
    return apiClient.get<BasePaginationResponse<SupportTicket>>('/tickets', { params })
  },
  unreadCount() {
    return apiClient.get<{ count: number }>('/tickets/unread-count')
  },
  rateGroups() {
    return apiClient.get<TicketRateGroupOption[]>('/tickets/rate-groups')
  },
  get(id: number) {
    return apiClient.get<SupportTicket>(`/tickets/${id}`)
  },
  messages(id: number) {
    return apiClient.get<SupportTicketMessage[]>(`/tickets/${id}/messages`)
  },
  withdraw(id: number) {
    return apiClient.post(`/tickets/${id}/withdraw`)
  },
  update(id: number, data: { title: string; form_payload: Record<string, unknown>; expected_revision_no: number }) {
    return apiClient.post(`/tickets/${id}/update`, data)
  },
  resubmit(id: number, data: { title: string; form_payload: Record<string, unknown>; expected_revision_no: number }) {
    return apiClient.post(`/tickets/${id}/resubmit`, data)
  },
  close(id: number) {
    return apiClient.post(`/tickets/${id}/close`)
  },
  reply(id: number, data: { content: string; media_ids?: number[] }) {
    return apiClient.post(`/tickets/${id}/reply`, data)
  },
  downloadGrant(id: number, mediaId: number) {
    return apiClient.post<{ url: string; expires_at: number; ttl_minutes: number }>(`/tickets/${id}/attachments/${mediaId}/download-grant`)
  },
}
