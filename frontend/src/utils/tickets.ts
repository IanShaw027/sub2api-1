import type { TicketCategory, TicketStatus } from '@/types/ticket'
import { ticketFormFromPayload, ticketPayloadFromForm } from '@/utils/ticketForm'

export const ticketCategoryOptions: Array<{ value: TicketCategory; labelKey: string }> = [
  { value: 'consult', labelKey: 'tickets.categories.consult' },
  { value: 'refund', labelKey: 'tickets.categories.refund' },
  { value: 'concurrency_apply', labelKey: 'tickets.categories.concurrency_apply' },
  { value: 'rate_apply', labelKey: 'tickets.categories.rate_apply' },
  { value: 'other', labelKey: 'tickets.categories.other' },
]

export const ticketStatusOptions: Array<{ value: TicketStatus; labelKey: string }> = [
  { value: 'submitted', labelKey: 'tickets.statuses.submitted' },
  { value: 'processing', labelKey: 'tickets.statuses.processing' },
  { value: 'waiting_user', labelKey: 'tickets.statuses.waiting_user' },
  { value: 'waiting_admin', labelKey: 'tickets.statuses.waiting_admin' },
  { value: 'resolved', labelKey: 'tickets.statuses.resolved' },
  { value: 'closed', labelKey: 'tickets.statuses.closed' },
  { value: 'withdrawn', labelKey: 'tickets.statuses.withdrawn' },
]

export function getTicketStatusBadgeClass(status: TicketStatus): string {
  switch (status) {
    case 'submitted':
      return 'badge-tone-accent'
    case 'processing':
      return 'badge-tone-warning'
    case 'waiting_user':
      return 'bg-accent-500/15 text-accent-700'
    case 'waiting_admin':
      return 'bg-accent-500/15 text-accent-700'
    case 'resolved':
      return 'badge-tone-success'
    case 'closed':
      return 'badge-tone-muted'
    case 'withdrawn':
      return 'bg-danger-500/15 text-danger-text'
    default:
      return 'badge-tone-muted'
  }
}

const requiredFieldsByCategory: Record<TicketCategory, string[]> = {
  consult: ['question'],
  refund: ['order_no', 'reason'],
  concurrency_apply: ['current_concurrency', 'target_concurrency', 'usage_scenario'],
  rate_apply: ['target_rate', 'usage_scenario'],
  other: ['details'],
}

function isFiniteNumberString(value: unknown, { allowZero = false }: { allowZero?: boolean } = {}): boolean {
  const normalized = String(value ?? '').trim()
  if (!normalized) return false
  const parsed = Number(normalized)
  if (!Number.isFinite(parsed)) return false
  return allowZero ? parsed >= 0 : parsed > 0
}

export function sanitizeTicketPayload(category: TicketCategory, payload: Record<string, unknown>): Record<string, unknown> {
  const { form, selectedGroupIds } = ticketFormFromPayload(payload)
  const groupIds = Array.isArray(payload.group_ids)
    ? payload.group_ids.map((id) => Number(id)).filter((id) => Number.isInteger(id) && id > 0)
    : selectedGroupIds
  return ticketPayloadFromForm(category, form, groupIds)
}

export function validateTicketPayload(category: TicketCategory, title: string, payload: Record<string, unknown>) {
  if (!title.trim()) {
    return 'tickets.validation.titleRequired'
  }
  const sanitized = sanitizeTicketPayload(category, payload)
  if (category === 'concurrency_apply') {
    if (!isFiniteNumberString(sanitized.current_concurrency, { allowZero: true })) {
      return 'tickets.validation.formIncomplete'
    }
    if (!isFiniteNumberString(sanitized.target_concurrency)) {
      return 'tickets.validation.formIncomplete'
    }
  }
  if (category === 'rate_apply') {
    const groupIDs = Array.isArray(sanitized.group_ids)
      ? sanitized.group_ids.map((item) => Number(item)).filter((item) => Number.isFinite(item) && item > 0)
      : []
    if (groupIDs.length === 0) {
      return 'tickets.validation.formIncomplete'
    }
    if (!isFiniteNumberString(sanitized.target_rate)) {
      return 'tickets.validation.formIncomplete'
    }
  }
  const requiredFields = requiredFieldsByCategory[category] || []
  for (const field of requiredFields) {
    const value = String(sanitized[field] ?? '').trim()
    if (!value) {
      return 'tickets.validation.formIncomplete'
    }
  }
  return ''
}

export interface TicketUploadResult {
  id: number
  filename?: string
  original_file_name?: string
  mime?: string
  mime_type?: string
  size?: number
  size_bytes?: number
}
