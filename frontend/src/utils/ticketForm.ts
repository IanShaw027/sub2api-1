import type { TicketCategory } from '@/types/ticket'

export function emptyTicketForm(currentConcurrency = ''): Record<string, string> {
  return {
    question: '',
    order_no: '',
    refund_amount: '',
    reason: '',
    evidence: '',
    current_concurrency: currentConcurrency,
    target_concurrency: '',
    usage_scenario: '',
    peak_window: '',
    target_rate: '',
    details: '',
  }
}

export function ticketFormFromPayload(
  payload: Record<string, unknown> | null | undefined,
  fallbackConcurrency = ''
): { form: Record<string, string>; selectedGroupIds: number[] } {
  const src = payload || {}
  const form = emptyTicketForm(fallbackConcurrency)
  for (const key of Object.keys(form)) {
    const value = src[key]
    if (value != null) form[key] = String(value)
  }
  if (!form.refund_amount && src.expected_amount != null) {
    form.refund_amount = String(src.expected_amount)
  }
  const rawIds = src.group_ids
  const selectedGroupIds = Array.isArray(rawIds)
    ? rawIds.map((id) => Number(id)).filter((id) => Number.isInteger(id) && id > 0)
    : []
  return { form, selectedGroupIds }
}

export function ticketPayloadFromForm(
  category: TicketCategory,
  form: Record<string, string>,
  selectedGroupIds: number[]
): Record<string, unknown> {
  if (category === 'consult') return { question: form.question }
  if (category === 'refund') {
    return {
      order_no: form.order_no,
      refund_amount: form.refund_amount,
      reason: form.reason,
      evidence: form.evidence,
    }
  }
  if (category === 'concurrency_apply') {
    const payload: Record<string, unknown> = {
      current_concurrency: form.current_concurrency,
      target_concurrency: form.target_concurrency,
      usage_scenario: form.usage_scenario,
    }
    if (form.peak_window?.trim()) payload.peak_window = form.peak_window
    return payload
  }
  if (category === 'rate_apply') {
    return {
      group_ids: selectedGroupIds,
      target_rate: form.target_rate,
      usage_scenario: form.usage_scenario,
    }
  }
  return { details: form.details }
}

export const TICKET_UNREAD_CHANGED_EVENT = 'ticket-unread-changed'

export function notifyTicketUnreadChanged() {
  window.dispatchEvent(new CustomEvent(TICKET_UNREAD_CHANGED_EVENT))
}
