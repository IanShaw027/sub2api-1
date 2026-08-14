import { apiClient } from '../client'
import type { CapacityTimeseries } from './capacity'

export interface OAuthPlanCount {
  plan_type: string
  total: number
  schedulable: number
  errors: number
  rate_limited: number
}

export interface OAuthRateLimitBuckets {
  total: number
  up_to_10m: number
  from_10m_to_30m: number
  from_30m_to_1h: number
  from_1h_to_3h: number
  from_3h_to_5h: number
  from_5h_to_1d: number
  from_1d_to_3d: number
  over_3d: number
}

export interface OAuthCapacityWindow {
  window: string
  used_percent: number | null
  remaining_percent: number | null
  reset_at: string | null
  burn_rate: number
  exhausts_at?: string | null
  measured_accounts: number
  alert?: string
}

export interface OAuthCapacityScope {
  group_id?: number | null
  group_name: string
  accounts: OAuthPlanCount
  plan_counts: OAuthPlanCount[]
  rate_limits: OAuthRateLimitBuckets
  windows: OAuthCapacityWindow[]
  alert?: string
  suggest_accounts: number
  first_shortfall_at?: string | null
}

export interface OAuthCapacityOverview {
  generated_at: string
  total: OAuthCapacityScope
  groups: OAuthCapacityScope[]
}

export async function getOverview(groupId?: number): Promise<OAuthCapacityOverview> {
  const { data } = await apiClient.get<OAuthCapacityOverview>('/admin/accounts/openai-oauth-capacity', {
    params: groupId ? { group_id: groupId } : undefined
  })
  return data
}

export async function getTimeseries(params?: {
  group_id?: number
  range?: string
  force?: boolean
}): Promise<CapacityTimeseries> {
  const { data } = await apiClient.get<CapacityTimeseries>('/admin/accounts/openai-oauth-capacity/timeseries', {
    params
  })
  return data
}

export const oauthCapacityAPI = { getOverview, getTimeseries }
export default oauthCapacityAPI
