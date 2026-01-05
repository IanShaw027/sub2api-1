/**
 * OpsDashboard 组件共享类型定义
 */

import type { OpsSeverity } from '@/api/admin/ops'

export type ChartState = 'loading' | 'empty' | 'ready'

export interface ErrorFilters {
  platforms: string[]
  groupId: number | null
  statusCodes: number[]
  clientIp: string
  severity: OpsSeverity | ''
  searchText: string
}

export interface ErrorLogsPagination {
  page: number
  pageSize: number
}

export type AlertSeverity = 'critical' | 'warning' | 'info'
export type ThresholdMode = 'count' | 'percentage' | 'both'
export type MetricType = 'success_rate' | 'error_rate' | 'p95_latency_ms' | 'p99_latency_ms' | 'cpu_usage_percent' | 'memory_usage_percent' | 'concurrency_queue_depth'
export type Operator = '>' | '>=' | '<' | '<=' | '==' | '!='

export interface AlertRule {
  id?: number
  name: string
  description?: string
  enabled: boolean
  metric_type: MetricType
  operator: Operator
  threshold: number
  window_minutes: number
  sustained_minutes: number
  severity: OpsSeverity
  cooldown_minutes: number
  notify_email: boolean
  created_at?: string
  updated_at?: string
  alert_category?: string
  dimension_filters?: Record<string, any>
  notify_channels?: string[]
  notify_config?: Record<string, any>
  filter_conditions?: Record<string, any>
  aggregation_dimensions?: string[]
}

export interface AlertEvent {
  id: number
  rule_id: number
  severity: OpsSeverity | string
  status: 'firing' | 'resolved' | string
  title?: string
  description?: string
  metric_value?: number
  threshold_value?: number
  fired_at: string
  resolved_at?: string | null
  email_sent: boolean
  created_at: string
}

export interface GroupAvailabilityConfig {
  id?: number
  group_id: number
  enabled: boolean
  min_available_accounts: number
  threshold_mode: ThresholdMode
  min_available_percentage?: number
  severity: AlertSeverity
  notify_email: boolean
  cooldown_minutes: number
  created_at?: string
  updated_at?: string
}

export interface GroupAvailabilityStatus {
  group_id: number
  group_name: string
  platform: string
  total_accounts: number
  available_accounts: number
  disabled_accounts?: number
  error_accounts?: number
  overload_accounts?: number
  monitoring_enabled: boolean
  min_available_accounts: number
  is_healthy: boolean
  alert_status?: string
  last_alert_at?: string | null
  config?: GroupAvailabilityConfig
}

export interface GroupAvailabilityEvent {
  id: number
  config_id: number
  group_id: number
  status: 'firing' | 'resolved' | string
  severity: AlertSeverity | string
  title?: string
  description?: string
  available_accounts: number
  threshold_accounts: number
  total_accounts: number
  email_sent: boolean
  fired_at: string
  resolved_at?: string | null
  created_at: string
  group?: {
    id: number
    name: string
    platform?: string
  }
}

export interface EmailNotificationConfig {
  alert: {
    enabled: boolean
    recipients: string[]
    min_severity: AlertSeverity | ''
    rate_limit_per_hour: number
    batching_window_seconds: number
    include_resolved_alerts: boolean
  }
  report: {
    enabled: boolean
    recipients: string[]
    daily_summary_enabled: boolean
    daily_summary_schedule: string
    weekly_summary_enabled: boolean
    weekly_summary_schedule: string
    error_digest_enabled: boolean
    error_digest_schedule: string
    error_digest_min_count: number
    account_health_enabled: boolean
    account_health_schedule: string
    account_health_error_rate_threshold: number
  }
}

export interface OpsDistributedLockSettings {
  enabled: boolean
  key: string
  ttl_seconds: number
}

export interface OpsAlertRuntimeSettings {
  evaluation_interval_seconds: number
  distributed_lock: OpsDistributedLockSettings
  silencing: {
    enabled: boolean
    global_until_rfc3339: string
    global_reason: string
    entries?: Array<{
      rule_id?: number
      severities?: Array<AlertSeverity | string>
      until_rfc3339: string
      reason: string
    }>
  }
}

export interface OpsGroupAvailabilityRuntimeSettings {
  evaluation_interval_seconds: number
  distributed_lock: OpsDistributedLockSettings
}
