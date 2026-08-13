import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'
import type { SupportTicket, SupportTicketMessage, TicketReplyTemplate } from '@/types/ticket'

export const adminTicketsAPI = {
  list(params?: { page?: number; page_size?: number; status?: string; category?: string; keyword?: string; unread_only?: boolean; start_date?: string; end_date?: string }) {
    return apiClient.get<BasePaginationResponse<SupportTicket>>('/admin/tickets', { params })
  },
  unreadCount() {
    return apiClient.get<{ count: number }>('/admin/tickets/unread-count')
  },
  get(id: number) {
    return apiClient.get<SupportTicket>(`/admin/tickets/${id}`)
  },
  messages(id: number) {
    return apiClient.get<SupportTicketMessage[]>(`/admin/tickets/${id}/messages`)
  },
  reply(id: number, data: { content: string; media_ids?: number[] }) {
    return apiClient.post(`/admin/tickets/${id}/reply`, data)
  },
  updateStatus(id: number, status: string) {
    return apiClient.post(`/admin/tickets/${id}/status`, { status })
  },
  listTemplates() {
    return apiClient.get<TicketReplyTemplate[]>('/admin/tickets/reply-templates')
  },
  createTemplate(data: { title: string; content: string; sort_order?: number }) {
    return apiClient.post<TicketReplyTemplate>('/admin/tickets/reply-templates', data)
  },
  updateTemplate(id: number, data: { title: string; content: string; sort_order?: number }) {
    return apiClient.put<TicketReplyTemplate>(`/admin/tickets/reply-templates/${id}`, data)
  },
  deleteTemplate(id: number) {
    return apiClient.delete(`/admin/tickets/reply-templates/${id}`)
  },
  downloadGrant(id: number, mediaId: number) {
    return apiClient.post<{ url: string }>(`/admin/tickets/${id}/attachments/${mediaId}/download-grant`)
  },
}
