import { apiClient } from '../client'

export interface KiroAuthUrlResponse {
  auth_url: string
  session_id: string
  callback_url: string
}

export interface KiroAuthUrlRequest {
  proxy_id?: number
}

export interface KiroExchangeCallbackRequest {
  session_id: string
  callback_url: string
  proxy_id?: number
}

export interface KiroDeviceCompleteRequest {
  session_id: string
  proxy_id?: number
}

export interface KiroRefreshTokenRequest {
  credentials: Record<string, unknown>
  extra?: Record<string, unknown>
  proxy_id?: number
}

export interface KiroTokenInfo {
  access_token?: string
  refresh_token?: string
  token_type?: string
  expires_at?: string
  auth_method?: string
  client_id?: string
  client_secret?: string
  token_endpoint?: string
  region?: string
  auth_region?: string
  api_region?: string
  profile_arn?: string
  profile_id?: string
  email?: string
  name?: string
  user_id?: string
  login_provider?: string
  plan_name?: string
  plan_tier?: string
  usage_reset_at?: string
  status?: string
  status_reason?: string
  issuer_url?: string
  idc_region?: string
  scopes?: string
  login_hint?: string
  subscription_type?: string
  [key: string]: unknown
}

export interface KiroIDCContinuationInfo {
  session_id: string
  status: string
  auth_method: string
  login_option?: string
  start_url?: string
  issuer_url?: string
  idc_region?: string
  scopes?: string[]
  login_hint?: string
  user_code?: string
  verification_uri?: string
  verification_uri_complete?: string
  interval_seconds?: number
  expires_at?: string
  message?: string
}

export interface KiroOAuthProgressResult {
  token_info?: KiroTokenInfo | null
  continuation?: KiroIDCContinuationInfo | null
}

export type KiroExchangeCallbackResponse = KiroTokenInfo | KiroOAuthProgressResult

export function isKiroContinuationResponse(
  value: KiroExchangeCallbackResponse | null | undefined
): value is KiroOAuthProgressResult {
  if (!value || typeof value !== 'object') return false
  const candidate = value as KiroOAuthProgressResult
  return Boolean(candidate.continuation)
}

export async function generateAuthUrl(
  payload: KiroAuthUrlRequest
): Promise<KiroAuthUrlResponse> {
  const { data } = await apiClient.post<KiroAuthUrlResponse>('/admin/kiro/oauth/auth-url', payload)
  return data
}

export async function exchangeCallback(
  payload: KiroExchangeCallbackRequest
): Promise<KiroExchangeCallbackResponse> {
  const { data } = await apiClient.post<KiroExchangeCallbackResponse>(
    '/admin/kiro/oauth/exchange-callback',
    payload
  )
  return data
}

export async function deviceComplete(
  payload: KiroDeviceCompleteRequest
): Promise<KiroOAuthProgressResult> {
  const { data } = await apiClient.post<KiroOAuthProgressResult>(
    '/admin/kiro/oauth/device-complete',
    payload
  )
  return data
}

export async function refreshToken(
  payload: KiroRefreshTokenRequest
): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>('/admin/kiro/oauth/refresh-token', payload)
  return data
}

export default {
  generateAuthUrl,
  exchangeCallback,
  deviceComplete,
  refreshToken
}
