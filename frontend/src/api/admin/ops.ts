/**
 * Admin Ops API endpoints
 * Provides stability metrics and error logs for ops dashboard
 */

import { apiClient } from '../client'

export type OpsSeverity = 'P0' | 'P1' | 'P2' | 'P3'
export type OpsPhase =
  | 'auth'
  | 'concurrency'
  | 'billing'
  | 'scheduling'
  | 'network'
  | 'upstream'
  | 'response'
  | 'internal'
export type OpsPlatform = 'gemini' | 'openai' | 'anthropic' | 'antigravity'

export interface OpsMetrics {
  window_minutes: number
  request_count: number
  success_count: number
  error_count: number
  success_rate: number
  error_rate: number
  p95_latency_ms: number
  p99_latency_ms: number
  http2_errors: number
  active_alerts: number
  cpu_usage_percent?: number
  memory_used_mb?: number
  memory_total_mb?: number
  memory_usage_percent?: number
  heap_alloc_mb?: number
  gc_pause_ms?: number
  concurrency_queue_depth?: number
  updated_at?: string
}

export interface OpsErrorLog {
  id: number
  created_at: string
  phase: OpsPhase
  type: string
  severity: OpsSeverity
  status_code: number
  platform: OpsPlatform
  model: string
  latency_ms: number | null
  request_id: string
  message: string
  user_id?: number | null
  api_key_id?: number | null
  account_id?: number | null
  group_id?: number | null
  client_ip?: string
  request_path?: string
  stream?: boolean
}

export interface OpsErrorListParams {
  start_time?: string
  end_time?: string
  platform?: OpsPlatform
  phase?: OpsPhase
  severity?: OpsSeverity
  q?: string
  limit?: number
}

export interface OpsErrorListResponse {
  items: OpsErrorLog[]
  total?: number
}

export interface OpsMetricsHistoryParams {
  window_minutes?: number
  minutes?: number
  start_time?: string
  end_time?: string
  limit?: number
}

export interface OpsMetricsHistoryResponse {
  items: OpsMetrics[]
}

/**
 * Get latest ops metrics snapshot
 */
export async function getMetrics(): Promise<OpsMetrics> {
  const { data } = await apiClient.get<OpsMetrics>('/admin/ops/metrics')
  return data
}

/**
 * List metrics history for charts
 */
export async function listMetricsHistory(params?: OpsMetricsHistoryParams): Promise<OpsMetricsHistoryResponse> {
  const { data } = await apiClient.get<OpsMetricsHistoryResponse>('/admin/ops/metrics/history', { params })
  return data
}

/**
 * List recent error logs with optional filters
 */
export async function listErrors(params?: OpsErrorListParams): Promise<OpsErrorListResponse> {
  const { data } = await apiClient.get<OpsErrorListResponse>('/admin/ops/error-logs', { params })
  return data
}

export const opsAPI = {
  getMetrics,
  listMetricsHistory,
  listErrors
}

export default opsAPI
