/**
 * Admin Ops API endpoints
 * Provides stability metrics and error logs for ops dashboard
 */

import { apiClient } from '../client'
import type {
  AlertRule,
  GroupAvailabilityConfig,
  GroupAvailabilityStatus,
  EmailNotificationConfig,
  OpsAlertRuntimeSettings,
  OpsGroupAvailabilityRuntimeSettings
} from '@/views/admin/ops/types'

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
  qps?: number
  tps?: number
  error_4xx_count?: number
  error_5xx_count?: number
  error_timeout_count?: number
  latency_p50?: number
  latency_p95?: number
  latency_p99?: number
  latency_avg?: number
  latency_max?: number
  upstream_latency_avg?: number
  disk_used?: number
  disk_total?: number
  disk_iops?: number
  disk_read_bytes?: number
  disk_write_bytes?: number
  /**
   * Total bytes received in the window (approx RX).
   * Backend may later provide per-second rate or split read/write fields.
   */
  network_in_bytes?: number
  /**
   * Total bytes sent in the window (approx TX).
   * Backend may later provide per-second rate or split read/write fields.
   */
  network_out_bytes?: number
  goroutine_count?: number
  db_conn_active?: number
  db_conn_idle?: number
  db_conn_waiting?: number
  token_consumed?: number
  token_rate?: number
  active_subscriptions?: number
  tags?: Record<string, any>
  success_rate: number
  error_rate: number
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

export interface OpsErrorDetail extends OpsErrorLog {
  // 延迟细化字段
  auth_latency_ms?: number | null
  routing_latency_ms?: number | null
  upstream_latency_ms?: number | null
  response_latency_ms?: number | null
  time_to_first_token_ms?: number | null

  // 请求和响应信息
  request_body?: string
  response_body?: string
  user_agent?: string
}

export type OpsRequestKind = 'success' | 'error'

export interface OpsRequestDetail {
  kind: OpsRequestKind
  created_at: string
  request_id: string

  platform?: string
  model?: string

  duration_ms?: number | null
  status_code?: number | null

  error_id?: number | null
  phase?: OpsPhase
  severity?: OpsSeverity
  message?: string

  user_id?: number | null
  api_key_id?: number | null
  account_id?: number | null
  group_id?: number | null

  stream: boolean
}

export interface OpsRequestDetailsParams {
  start_time?: string
  end_time?: string
  time_range?: string
  kind?: 'success' | 'error' | 'all'
  platform?: OpsPlatform
  platforms?: string
  user_id?: number
  api_key_id?: number
  account_id?: number
  group_id?: number
  model?: string
  request_id?: string
  q?: string
  min_duration_ms?: number
  max_duration_ms?: number
  sort?: 'created_at_desc' | 'duration_desc'
  page?: number
  page_size?: number
}

export interface OpsRequestDetailsResponse {
  items: OpsRequestDetail[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface OpsErrorListParams {
  start_time?: string
  end_time?: string
  platform?: OpsPlatform
  /**
   * Comma-separated platforms (e.g. "openai,anthropic"); preferred over `platform` when selecting multiple.
   * Backend supports both `platform` (single) and `platforms` (multi) for compatibility.
   */
  platforms?: string
  phase?: OpsPhase
  severity?: OpsSeverity
  q?: string
  /**
   * Comma-separated integers (e.g. "500,502,503")
   */
  status_codes?: string
  /**
   * Client IP filter (exact match, per backend behavior)
   */
  client_ip?: string
  /**
   * Max 500 (legacy endpoint uses a hard cap); use paginated /admin/ops/errors for larger result sets.
   */
  limit?: number
  page?: number
  page_size?: number
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

type OpsErrorLogsRawResponse = {
  // New endpoint shape: { errors, total, page, page_size }
  errors?: OpsErrorLog[]
  // Legacy endpoint shape: { items, total }
  items?: OpsErrorLog[]
  total?: number
  page?: number
  page_size?: number
}

/**
 * List error logs with server-side filters.
 * Normalizes both legacy and paginated endpoint response shapes into `{ items, total }`.
 */
export async function listErrorLogs(params?: OpsErrorListParams): Promise<OpsErrorListResponse> {
  const { data } = await apiClient.get<OpsErrorLogsRawResponse>('/admin/ops/errors', { params })
  const items = Array.isArray(data.items) ? data.items : Array.isArray(data.errors) ? data.errors : []
  return { items, total: data.total }
}

/**
 * Backwards-compatible alias for `listErrorLogs`.
 */
export async function listErrors(params?: OpsErrorListParams): Promise<OpsErrorListResponse> {
  return listErrorLogs(params)
}

/**
 * Get detailed error log by ID
 */
export async function getErrorDetail(id: number): Promise<OpsErrorDetail> {
  const { data } = await apiClient.get<OpsErrorDetail>(`/admin/ops/errors/${id}`)
  return data
}

export interface RetryErrorResponse {
  can_retry: boolean
  request_id: string
  platform: string
  model: string
  request_body: string
  message: string
}

export async function retryErrorRequest(id: number): Promise<RetryErrorResponse> {
  const { data } = await apiClient.post<RetryErrorResponse>(`/admin/ops/errors/${id}/retry`)
  return data
}

export async function listRequestDetails(params?: OpsRequestDetailsParams): Promise<OpsRequestDetailsResponse> {
  const { data } = await apiClient.get<OpsRequestDetailsResponse>('/admin/ops/requests', { params })
  return data
}

export interface OpsDashboardOverview {
  timestamp: string
  health_score: number
  sla: {
    current: number
    threshold: number
    status: string
    trend: string
    change_24h: number
  }
  qps: {
    current: number
    peak_1h: number
    avg_1h: number
    change_vs_yesterday: number
  }
  tps: {
    current: number
    peak_1h: number
    avg_1h: number
  }
  latency: {
    p50: number
    p95: number
    p99: number
    avg: number
    max: number
    threshold_p99: number
    status: string
  }
  errors: {
    total_count: number
    error_rate: number
    '4xx_count': number
    '5xx_count': number
    timeout_count: number
    top_error?: {
      code: string
      message: string
      count: number
    }
  }
  resources: {
    cpu_usage: number
    memory_usage: number
    disk_usage: number
    goroutines: number
    db_connections: {
      active: number
      idle: number
      waiting: number
      max: number
    }
  }
  system_status: {
    redis: string
    database: string
    background_jobs: string
  }
}

export interface ProviderHealthData {
  name: string
  request_count: number
  success_rate: number
  error_rate: number
  latency_avg: number
  latency_p99: number
  status: string
  errors_by_type: {
    '4xx': number
    '5xx': number
    timeout: number
  }
}

export interface ProviderHealthResponse {
  providers: ProviderHealthData[]
  summary: {
    total_requests: number
    avg_success_rate: number
    best_provider: string
    worst_provider: string
  }
}

export interface LatencyHistogramResponse {
  buckets: {
    range: string
    count: number
    percentage: number
  }[]
  total_requests: number
  slow_request_threshold: number
}

export interface ErrorDistributionResponse {
  items: {
    code: string
    message: string
    count: number
    percentage: number
  }[]
}

/**
 * Get realtime ops dashboard overview
 */
export async function getDashboardOverview(timeRange = '1h'): Promise<OpsDashboardOverview> {
  const { data } = await apiClient.get<OpsDashboardOverview>('/admin/ops/dashboard/overview', {
    params: { time_range: timeRange }
  })
  return data
}

/**
 * Get provider health comparison
 */
export async function getProviderHealth(timeRange = '1h'): Promise<ProviderHealthResponse> {
  const { data } = await apiClient.get<ProviderHealthResponse>('/admin/ops/dashboard/providers', {
    params: { time_range: timeRange }
  })
  return data
}

/**
 * Get latency histogram
 */
export async function getLatencyHistogram(timeRange = '1h'): Promise<LatencyHistogramResponse> {
  const { data } = await apiClient.get<LatencyHistogramResponse>('/admin/ops/dashboard/latency-histogram', {
    params: { time_range: timeRange }
  })
  return data
}

/**
 * Get error distribution
 */
export async function getErrorDistribution(timeRange = '1h'): Promise<ErrorDistributionResponse> {
  const { data } = await apiClient.get<ErrorDistributionResponse>('/admin/ops/dashboard/errors/distribution', {
    params: { time_range: timeRange }
  })
  return data
}

/**
 * Subscribe to realtime QPS updates via WebSocket
 */
export interface SubscribeQPSOptions {
  token?: string | null
  onOpen?: () => void
  onClose?: (event: CloseEvent) => void
  onError?: (event: Event) => void
  wsBaseUrl?: string
}

export function subscribeQPS(onMessage: (data: any) => void, options: SubscribeQPSOptions = {}): () => void {
  let ws: WebSocket | null = null
  let reconnectAttempts = 0
  const maxReconnectAttempts = 5
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let shouldReconnect = true

  const connect = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsBaseUrl = options.wsBaseUrl || import.meta.env.VITE_WS_BASE_URL || window.location.host
    const wsURL = new URL(`${protocol}//${wsBaseUrl}/api/v1/admin/ops/ws/qps`)

    const token = options.token ?? localStorage.getItem('auth_token')
    if (token) wsURL.searchParams.set('token', token)

    ws = new WebSocket(wsURL.toString())

    ws.onopen = () => {
      reconnectAttempts = 0
      options.onOpen?.()
    }

    ws.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data)
        onMessage(data)
      } catch (err) {
        console.warn('[OpsWS] Failed to parse message:', err)
      }
    }

    ws.onerror = (error) => {
      console.error('[OpsWS] Connection error:', error)
      options.onError?.(error)
    }

    ws.onclose = (event) => {
      options.onClose?.(event)
      if (shouldReconnect && reconnectAttempts < maxReconnectAttempts) {
        const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000)
        reconnectTimer = setTimeout(() => {
          reconnectAttempts++
          connect()
        }, delay)
      }
    }
  }

  connect()

  return () => {
    shouldReconnect = false
    if (reconnectTimer) clearTimeout(reconnectTimer)
    if (ws) ws.close()
  }
}

// Alert Rules API
export async function listAlertRules(): Promise<AlertRule[]> {
  const { data } = await apiClient.get<AlertRule[]>('/admin/ops/alert-rules')
  return data
}

export async function createAlertRule(rule: AlertRule): Promise<AlertRule> {
  const { data } = await apiClient.post<AlertRule>('/admin/ops/alert-rules', rule)
  return data
}

export async function updateAlertRule(id: number, rule: Partial<AlertRule>): Promise<AlertRule> {
  const { data } = await apiClient.put<AlertRule>(`/admin/ops/alert-rules/${id}`, rule)
  return data
}

export async function deleteAlertRule(id: number): Promise<void> {
  await apiClient.delete(`/admin/ops/alert-rules/${id}`)
}

// Group Availability API
export interface ListGroupAvailabilityStatusParams {
  search?: string
  monitoring?: 'enabled' | 'disabled' | 'all'
  alert?: 'ok' | 'firing' | 'all'
  page?: number
  page_size?: number
}

export interface ListGroupAvailabilityStatusResponse {
  items: GroupAvailabilityStatus[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export async function listGroupAvailabilityStatus(params?: ListGroupAvailabilityStatusParams): Promise<ListGroupAvailabilityStatusResponse> {
  const { data } = await apiClient.get<ListGroupAvailabilityStatusResponse>('/admin/ops/group-availability/status', { params })
  return data
}

export async function updateGroupAvailabilityConfig(groupId: number, config: Partial<GroupAvailabilityConfig>): Promise<GroupAvailabilityConfig> {
  const { data } = await apiClient.put<GroupAvailabilityConfig>(`/admin/ops/group-availability/configs/${groupId}`, config)
  return data
}

// Email Notification API
export async function getEmailNotificationConfig(): Promise<EmailNotificationConfig> {
  const { data } = await apiClient.get<EmailNotificationConfig>('/admin/ops/email-notification/config')
  return data
}

export async function updateEmailNotificationConfig(config: EmailNotificationConfig): Promise<EmailNotificationConfig> {
  const { data } = await apiClient.put<EmailNotificationConfig>('/admin/ops/email-notification/config', config)
  return data
}

// Runtime settings API (DB-backed)
export async function getAlertRuntimeSettings(): Promise<OpsAlertRuntimeSettings> {
  const { data } = await apiClient.get<OpsAlertRuntimeSettings>('/admin/ops/runtime/alert')
  return data
}

export async function updateAlertRuntimeSettings(config: OpsAlertRuntimeSettings): Promise<OpsAlertRuntimeSettings> {
  const { data } = await apiClient.put<OpsAlertRuntimeSettings>('/admin/ops/runtime/alert', config)
  return data
}

export async function getGroupAvailabilityRuntimeSettings(): Promise<OpsGroupAvailabilityRuntimeSettings> {
  const { data } = await apiClient.get<OpsGroupAvailabilityRuntimeSettings>('/admin/ops/runtime/group-availability')
  return data
}

export async function updateGroupAvailabilityRuntimeSettings(config: OpsGroupAvailabilityRuntimeSettings): Promise<OpsGroupAvailabilityRuntimeSettings> {
  const { data } = await apiClient.put<OpsGroupAvailabilityRuntimeSettings>('/admin/ops/runtime/group-availability', config)
  return data
}

export const opsAPI = {
  getMetrics,
  listMetricsHistory,
  listErrorLogs,
  listErrors,
  getErrorDetail,
  retryErrorRequest,
  listRequestDetails,
  getDashboardOverview,
  getProviderHealth,
  getLatencyHistogram,
  getErrorDistribution,
  subscribeQPS,
  listAlertRules,
  createAlertRule,
  updateAlertRule,
  deleteAlertRule,
  listGroupAvailabilityStatus,
  updateGroupAvailabilityConfig,
  getEmailNotificationConfig,
  updateEmailNotificationConfig,
  getAlertRuntimeSettings,
  updateAlertRuntimeSettings,
  getGroupAvailabilityRuntimeSettings,
  updateGroupAvailabilityRuntimeSettings
}

export default opsAPI
