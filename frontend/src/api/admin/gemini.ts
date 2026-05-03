/**
 * Admin Gemini API endpoints
 * Handles Gemini OAuth flows for administrators
 */

import { apiClient } from '../client'

export interface GeminiAuthUrlResponse {
  auth_url: string
  session_id: string
  state: string
}

export interface GeminiAuthUrlRequest {
  proxy_id?: number
  project_id?: string
  oauth_type?: 'code_assist' | 'google_one'
  tier_id?: string
}

export interface GeminiExchangeCodeRequest {
  session_id: string
  state: string
  code: string
  proxy_id?: number
  oauth_type?: 'code_assist' | 'google_one'
  tier_id?: string
}

export type GeminiTokenInfo = {
  access_token?: string
  refresh_token?: string
  id_token?: string
  token_type?: string
  scope?: string
  expires_in?: number
  expires_at?: number
  project_id?: string
  email?: string
  auth_id?: string
  name?: string
  plan_name?: string
  oauth_type?: string
  tier_id?: string
  extra?: Record<string, unknown>
  [key: string]: unknown
}

export async function generateAuthUrl(
  payload: GeminiAuthUrlRequest
): Promise<GeminiAuthUrlResponse> {
  const { data } = await apiClient.post<GeminiAuthUrlResponse>(
    '/admin/gemini/oauth/auth-url',
    payload
  )
  return data
}

export async function exchangeCode(payload: GeminiExchangeCodeRequest): Promise<GeminiTokenInfo> {
  const { data } = await apiClient.post<GeminiTokenInfo>(
    '/admin/gemini/oauth/exchange-code',
    payload
  )
  return data
}

export default { generateAuthUrl, exchangeCode }
