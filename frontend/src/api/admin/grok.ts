/**
 * Admin Grok/xAI API endpoints
 * Handles xAI OAuth flows for administrators.
 */

import { apiClient } from '../client'
import type { Account, GrokBillingSummary, GrokQuotaWindow, WindowStats } from '@/types'

export type { GrokBillingSummary, GrokQuotaWindow } from '@/types'

export interface GrokAuthUrlResponse {
  auth_url: string
  session_id: string
  state: string
}

export interface GrokAuthUrlRequest {
  proxy_id?: number
  redirect_uri?: string
}

export interface GrokExchangeCodeRequest {
  session_id: string
  state: string
  code: string
  proxy_id?: number
}

export interface GrokTokenInfo {
  access_token?: string
  refresh_token?: string
  token_type?: string
  id_token?: string
  sso_token?: string
  expires_at?: number | string
  expires_in?: number
  scope?: string
  client_id?: string
  email?: string
  sub?: string
  team_id?: string
  subscription_tier?: string
  entitlement_status?: string
  [key: string]: unknown
}

export interface GrokQuotaSnapshot {
  requests?: GrokQuotaWindow | null
  tokens?: GrokQuotaWindow | null
  retry_after_seconds?: number | null
  subscription_tier?: string
  entitlement_status?: string
  status_code?: number
  headers?: Record<string, string>
  headers_observed: boolean
  observation_source?: string
  last_probe_at?: string
  last_headers_seen_at?: string
  updated_at: string
}

export interface GrokQuotaProbeResult {
  source: 'active_probe' | 'billing_probe' | 'hybrid_probe'
  model?: string
  billing?: GrokBillingSummary | null
  snapshot?: GrokQuotaSnapshot | null
  local_usage_24h?: WindowStats | null
  local_usage_7d?: WindowStats | null
  local_usage_monthly?: WindowStats | null
  status_code?: number
  headers_observed: boolean
  reset_supported: boolean
  fetched_at: number
  persisted?: boolean
  probe_error?: string
}

export interface GrokQuotaResetResult {
  supported: boolean
  code: string
  message: string
}

export async function generateAuthUrl(
  payload: GrokAuthUrlRequest
): Promise<GrokAuthUrlResponse> {
  const { data } = await apiClient.post<GrokAuthUrlResponse>(
    '/admin/grok/oauth/auth-url',
    payload
  )
  return data
}

export async function exchangeCode(payload: GrokExchangeCodeRequest): Promise<GrokTokenInfo> {
  const { data } = await apiClient.post<GrokTokenInfo>(
    '/admin/grok/oauth/exchange-code',
    payload
  )
  return data
}

export async function refreshGrokToken(
  refreshToken: string,
  proxyId?: number | null
): Promise<GrokTokenInfo> {
  const payload: Record<string, unknown> = { refresh_token: refreshToken }
  if (proxyId) payload.proxy_id = proxyId

  const { data } = await apiClient.post<GrokTokenInfo>(
    '/admin/grok/oauth/refresh-token',
    payload
  )
  return data
}

export async function validateSSOToken(
  ssoToken: string,
  proxyId?: number | null
): Promise<GrokTokenInfo> {
  const payload: Record<string, unknown> = { sso_token: ssoToken }
  if (proxyId) payload.proxy_id = proxyId

  const { data } = await apiClient.post<GrokTokenInfo>(
    '/admin/grok/oauth/sso-token',
    payload
  )
  return data
}

export async function authorizePassword(
  emailPasswordInput: string,
  proxyId?: number | null
): Promise<GrokTokenInfo> {
  const [emailRaw, ...passwordParts] = emailPasswordInput.split('----')
  const email = emailRaw?.trim() || ''
  const password = passwordParts.join('----')
  const payload: Record<string, unknown> = { email, password }
  if (proxyId) payload.proxy_id = proxyId

  const { data } = await apiClient.post<GrokTokenInfo>(
    '/admin/grok/oauth/password',
    payload
  )
  return data
}

export async function queryQuota(id: number): Promise<GrokQuotaProbeResult> {
  const { data } = await apiClient.get<GrokQuotaProbeResult>(`/admin/grok/accounts/${id}/quota`)
  return data
}

export async function resetQuota(id: number): Promise<GrokQuotaResetResult> {
  const { data } = await apiClient.post<GrokQuotaResetResult>(`/admin/grok/accounts/${id}/reset-quota`)
  return data
}

/**
 * Create a Grok OAuth account by exchanging the OAuth code server-side.
 * Backend requires session_id + code + state; optionally accepts proxy_id,
 * name, concurrency, priority, group_ids.
 * Returns the full Account DTO after create.
 */
export interface GrokCreateFromOAuthRequest {
  session_id: string
  code: string
  state: string
  proxy_id?: number | null
  name?: string
  concurrency?: number
  priority?: number
  group_ids?: number[]
}

export async function createFromOAuth(
  payload: GrokCreateFromOAuthRequest
): Promise<Account> {
  const { data } = await apiClient.post<Account>(
    '/admin/grok/oauth/create-from-oauth',
    payload
  )
  return data
}

/**
 * Refresh credentials for an existing Grok OAuth account.
 * Backend merges new tokens into credentials and returns the updated Account DTO.
 */
export async function refreshAccountToken(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/grok/accounts/${id}/refresh`)
  return data
}

export interface GrokRuntimeSanityCheck {
  value: string
  valid: boolean
  error?: string
  is_default?: boolean
}

export interface GrokRuntimeSanityReport {
  base_url: GrokRuntimeSanityCheck
  oauth_authorize_url: GrokRuntimeSanityCheck
  oauth_token_url: GrokRuntimeSanityCheck
  oauth_redirect_uri: GrokRuntimeSanityCheck
  unsafe_url_overrides: boolean
  public_gateway_scope: string
  proxy_policy: string
}

export async function runtimeSanity(): Promise<GrokRuntimeSanityReport> {
  const { data } = await apiClient.get<GrokRuntimeSanityReport>('/admin/grok/runtime-sanity')
  return data
}

export default {
  generateAuthUrl,
  exchangeCode,
  refreshGrokToken,
  validateSSOToken,
  authorizePassword,
  queryQuota,
  resetQuota,
  createFromOAuth,
  refreshAccountToken,
  runtimeSanity,
}
