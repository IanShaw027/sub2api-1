/**
 * Admin Affiliate API endpoints
 * Handles affiliate rebate summaries for administrators.
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type { AffiliateInvitee } from '@/types'

export interface AdminAffiliateSummary {
  user_id: number
  email: string
  username: string
  aff_code: string
  inviter_id: number | null
  aff_count: number
  aff_quota: number
  aff_history_quota: number
  rebated_invitee_count: number
  period_invited_count: number
  period_rebate_amount: number
  created_at: string
  updated_at: string
}

export interface AdminAffiliateListFilters {
  search?: string
  start_date?: string
  end_date?: string
}

export async function listInvitees(userId: number): Promise<AffiliateInvitee[]> {
  const { data } = await apiClient.get<{ items: AffiliateInvitee[] }>(`/admin/affiliates/${userId}/invitees`)
  return data.items || []
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: AdminAffiliateListFilters,
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<AdminAffiliateSummary>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminAffiliateSummary>>('/admin/affiliates', {
    params: {
      page,
      page_size: pageSize,
      search: filters?.search,
      start_date: filters?.start_date,
      end_date: filters?.end_date,
    },
    signal: options?.signal,
  })
  return data
}

export const affiliateAPI = {
  list,
  listInvitees,
}

export default affiliateAPI
