/**
 * Admin Platform Capacity Forecast API endpoints
 * Provides a cross-platform capacity/spend forecast timeseries and an on-demand
 * probe (test) of OAuth accounts, used by the "Platform Capacity Forecast" dialog.
 */

import { apiClient } from '../client'

export type CapacityPlatform =
  | 'openai'
  | 'anthropic'
  | 'gemini'
  | 'grok'
  | 'antigravity'

export type CapacityRange = '12h' | '24h' | '48h' | '7d'

export type CapacitySegment = 'past' | 'current' | 'future'

export interface CapacityKPIs {
  current_available_usd: number
  future_forecast_spend_usd: number
  first_shortfall_at: string | null
  suggest_accounts: number
}

export interface CapacityPoint {
  bucket_start: string
  segment: CapacitySegment
  /** null for future buckets (only forecast_spend_usd applies there) */
  spend_usd: number | null
  /** null for past/current buckets that have no forecast value */
  forecast_spend_usd: number | null
  /** null = no persisted record for this bucket (renders as a gap in the line) */
  available_usd: number | null
  /** null for past/current buckets that have no forecast value */
  forecast_available_usd: number | null
  recovered_usd: number
}

export interface CapacityEventAccount {
  id: number
  name: string
}

export type CapacityEventType = 'recover' | 'shortfall'

export interface CapacityEvent {
  at: string
  type: CapacityEventType
  window_kind: string
  account_count: number
  amount_usd: number
  accounts: CapacityEventAccount[]
}

export interface CapacityRecommendation {
  at: string
  gap_usd: number
  suggest_accounts: number
  basis: string
}

export interface CapacityTimeseries {
  generated_at: string
  now: string
  platform: CapacityPlatform
  group_id: number | null
  unit: string
  range: CapacityRange
  kpis: CapacityKPIs
  points: CapacityPoint[]
  events: CapacityEvent[]
  recommendations: CapacityRecommendation[]
}

export interface CapacityTimeseriesParams {
  platform: CapacityPlatform
  group_id?: number
  range?: CapacityRange
  force?: boolean
}

export async function getCapacityTimeseries(
  params: CapacityTimeseriesParams
): Promise<CapacityTimeseries> {
  const query: Record<string, string> = {
    platform: params.platform
  }
  if (typeof params.group_id === 'number') query.group_id = String(params.group_id)
  if (params.range) query.range = params.range
  if (params.force) query.force = 'true'
  const { data } = await apiClient.get<CapacityTimeseries>(
    '/admin/accounts/capacity/timeseries',
    { params: query }
  )
  return data
}

export interface CapacityProbeRequest {
  platform: CapacityPlatform
  group_id?: number
  include_normal: boolean
}

export interface CapacityProbeFailure {
  account_id: number
  name: string
  error: string
}

export interface CapacityProbeResult {
  total: number
  probed: number
  ok: number
  rate_limited: number
  failed: number
  duration_ms: number
  failures: CapacityProbeFailure[]
}

export async function probeCapacity(
  body: CapacityProbeRequest
): Promise<CapacityProbeResult> {
  const { data } = await apiClient.post<CapacityProbeResult>(
    '/admin/accounts/capacity/probe',
    body,
    { timeout: 150000 } // 150s: probing accounts one-by-one against upstream can take a while
  )
  return data
}

const capacityAPI = {
  getCapacityTimeseries,
  probeCapacity
}

export default capacityAPI
