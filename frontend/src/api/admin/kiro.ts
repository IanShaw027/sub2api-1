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

export interface KiroTokenInfo {
  access_token?: string
  refresh_token?: string
  token_type?: string
  expires_at?: string
  auth_method?: string
  client_id?: string
  client_secret?: string
  region?: string
  auth_region?: string
  api_region?: string
  profile_arn?: string
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

export async function generateAuthUrl(
  payload: KiroAuthUrlRequest
): Promise<KiroAuthUrlResponse> {
  const { data } = await apiClient.post<KiroAuthUrlResponse>('/admin/kiro/oauth/auth-url', payload)
  return data
}

export async function exchangeCallback(
  payload: KiroExchangeCallbackRequest
): Promise<KiroTokenInfo> {
  const { data } = await apiClient.post<KiroTokenInfo>('/admin/kiro/oauth/exchange-callback', payload)
  return data
}

export default {
  generateAuthUrl,
  exchangeCallback
}
