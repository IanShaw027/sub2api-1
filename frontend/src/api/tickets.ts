import { apiClient } from './client'
import type { PaginatedResponse, SupportTicket, SupportTicketMessage, TicketCategory, TicketStatus } from '@/types'

interface TicketListParams {
  page?: number
  page_size?: number
  status?: TicketStatus | ''
  category?: TicketCategory | ''
  search?: string
  start_date?: string
  end_date?: string
  timezone?: string
}

interface TicketUpsertPayload {
  title: string
  category?: TicketCategory
  form_payload: Record<string, unknown>
  expected_revision_no: number
}

interface TicketCreatePayload {
  title: string
  category: TicketCategory
  form_payload: Record<string, unknown>
}

export async function listTickets(params: TicketListParams = {}) {
  const { data } = await apiClient.get<PaginatedResponse<SupportTicket>>('/tickets', { params })
  return data
}

export async function createTicket(payload: TicketCreatePayload) {
  const { data } = await apiClient.post<SupportTicket>('/tickets', payload)
  return data
}

export async function getTicket(id: number) {
  const { data } = await apiClient.get<SupportTicket>(`/tickets/${id}`)
  return data
}

export async function listTicketMessages(id: number) {
  const { data } = await apiClient.get<SupportTicketMessage[]>(`/tickets/${id}/messages`)
  return data
}

export async function updateTicket(id: number, payload: TicketUpsertPayload) {
  const { data } = await apiClient.patch<{ message: string }>(`/tickets/${id}`, payload)
  return data
}

export async function submitTicket(id: number, payload: TicketUpsertPayload) {
  const { data } = await apiClient.post<{ message: string }>(`/tickets/${id}/submit`, payload)
  return data
}

export async function withdrawTicket(id: number) {
  const { data } = await apiClient.post<{ message: string }>(`/tickets/${id}/withdraw`)
  return data
}

export async function closeTicket(id: number) {
  const { data } = await apiClient.post<{ message: string }>(`/tickets/${id}/close`)
  return data
}

export async function replyTicket(id: number, content: string, attachments?: { media_id: number }[]) {
  const { data } = await apiClient.post<{ message: string }>(`/tickets/${id}/messages`, {
    content,
    ...(attachments?.length ? { attachments } : {})
  })
  return data
}

export async function uploadTicketMedia(file: File, ticketId: number | string) {
  const formData = new FormData()
  formData.append('file', file)
  formData.append('biz_type', 'ticket')
  formData.append('biz_id', String(ticketId))
  formData.append('visibility', 'private')
  const { data } = await apiClient.post('/media/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
  return data
}

const ticketsAPI = {
  listTickets,
  createTicket,
  getTicket,
  listTicketMessages,
  updateTicket,
  submitTicket,
  withdrawTicket,
  closeTicket,
  replyTicket,
  uploadTicketMedia,
}

export default ticketsAPI
