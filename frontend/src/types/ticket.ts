export type TicketCategory = 'consult' | 'refund' | 'concurrency_apply' | 'rate_apply' | 'other'
export type TicketStatus = 'submitted' | 'processing' | 'waiting_user' | 'waiting_admin' | 'resolved' | 'closed' | 'withdrawn'

export interface TicketAttachment {
  media_id: number
  file_name: string
  content_type: string
  size_bytes: number
}

export interface SupportTicket {
  id: number
  ticket_no: string
  user_id: number
  user_name?: string
  user_email?: string
  category: TicketCategory
  title: string
  status: TicketStatus
  current_form_payload: Record<string, unknown>
  current_revision_no: number
  latest_message_at: string
  last_reply_role: string
  unread_by_user: boolean
  unread_by_admin: boolean
  submitted_at?: string
  closed_at?: string
  withdrawn_at?: string
  created_at: string
  updated_at: string
}

export interface SupportTicketMessage {
  id: number
  ticket_id: number
  sender_role: 'user' | 'admin' | 'system'
  sender_user_id?: number
  sender_name_snapshot: string
  message_type: 'message' | 'system'
  content: string
  attachments?: TicketAttachment[]
  created_at: string
}

export interface TicketReplyTemplate {
  id: number
  title: string
  content: string
  sort_order: number
}

export interface TicketRateGroupOption {
  group_id: number
  name: string
  base_rate_multiplier: number
  user_rate_multiplier?: number
  effective_rate: number
}
