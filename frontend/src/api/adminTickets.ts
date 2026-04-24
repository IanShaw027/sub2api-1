import { apiClient } from './client'
import type { PaginatedResponse, SupportTicket, SupportTicketMessage, TicketCategory, TicketStatus } from '@/types'

export interface TicketReplyTemplate {
  id: string
  title: string
  content: string
}

interface AdminTicketListParams {
  page?: number
  page_size?: number
  status?: TicketStatus | ''
  category?: TicketCategory | ''
  search?: string
  user?: string
  start_date?: string
  end_date?: string
  timezone?: string
}

export async function listAdminTickets(params: AdminTicketListParams = {}) {
  const { data } = await apiClient.get<PaginatedResponse<SupportTicket>>('/admin/tickets', { params })
  return data
}

export async function getAdminTicket(id: number) {
  const { data } = await apiClient.get<SupportTicket>(`/admin/tickets/${id}`)
  return data
}

export async function listAdminTicketMessages(id: number) {
  const { data } = await apiClient.get<SupportTicketMessage[]>(`/admin/tickets/${id}/messages`)
  return data
}

export async function replyAdminTicket(id: number, content: string) {
  const { data } = await apiClient.post<{ message: string }>(`/admin/tickets/${id}/messages`, { content })
  return data
}

export async function updateAdminTicketStatus(id: number, status: TicketStatus) {
  const { data } = await apiClient.post<{ message: string }>(`/admin/tickets/${id}/status`, { status })
  return data
}

export async function listAdminTicketReplyTemplates() {
  const { data } = await apiClient.get<TicketReplyTemplate[]>('/admin/tickets/reply-templates')
  return data
}

export async function replaceAdminTicketReplyTemplates(templates: TicketReplyTemplate[]) {
  const { data } = await apiClient.put<{ message: string }>('/admin/tickets/reply-templates', { templates })
  return data
}

const adminTicketsAPI = {
  listAdminTickets,
  getAdminTicket,
  listAdminTicketMessages,
  replyAdminTicket,
  updateAdminTicketStatus,
  listAdminTicketReplyTemplates,
  replaceAdminTicketReplyTemplates,
}

export default adminTicketsAPI
