/**
 * Admin TLS Fingerprint Router API endpoints
 * Handles TLS fingerprint router CRUD for administrators
 */

import { apiClient } from '../client'

export type TLSFingerprintRouterMatchType = 'contains' | 'prefix' | 'exact' | 'regex'
export type TLSFingerprintRouterTransport = '' | 'http' | 'websocket'

export interface TLSFingerprintRouterRule {
  name: string
  enabled: boolean
  transport?: TLSFingerprintRouterTransport
  match_type: TLSFingerprintRouterMatchType
  pattern: string
  case_sensitive: boolean
  tls_fingerprint_profile_id: number
  os?: string
  client_type?: string
  upstream_user_agent?: string
  upstream_originator?: string
}

export interface TLSFingerprintRouter {
  id: number
  name: string
  description: string | null
  enabled: boolean
  rules: TLSFingerprintRouterRule[]
  created_at: string
  updated_at: string
}

export interface CreateRouterRequest {
  name: string
  description?: string | null
  enabled?: boolean
  rules?: TLSFingerprintRouterRule[]
}

export interface UpdateRouterRequest {
  name?: string
  description?: string | null
  enabled?: boolean
  rules?: TLSFingerprintRouterRule[]
}

export interface TLSFingerprintRouterMatchResult {
  matched: boolean
  router_id?: number
  router_name?: string
  rule_name?: string
  tls_fingerprint_profile_id?: number
  upstream_user_agent?: string
  upstream_originator?: string
}

export async function list(): Promise<TLSFingerprintRouter[]> {
  const { data } = await apiClient.get<TLSFingerprintRouter[]>('/admin/tls-fingerprint-routers')
  return data
}

export async function getById(id: number): Promise<TLSFingerprintRouter> {
  const { data } = await apiClient.get<TLSFingerprintRouter>(`/admin/tls-fingerprint-routers/${id}`)
  return data
}

export async function create(routerData: CreateRouterRequest): Promise<TLSFingerprintRouter> {
  const { data } = await apiClient.post<TLSFingerprintRouter>('/admin/tls-fingerprint-routers', routerData)
  return data
}

export async function update(id: number, updates: UpdateRouterRequest): Promise<TLSFingerprintRouter> {
  const { data } = await apiClient.put<TLSFingerprintRouter>(`/admin/tls-fingerprint-routers/${id}`, updates)
  return data
}

export async function toggle(id: number, enabled: boolean): Promise<TLSFingerprintRouter> {
  return update(id, { enabled })
}

export async function deleteRouter(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/tls-fingerprint-routers/${id}`)
  return data
}

export const tlsFingerprintRouterAPI = {
  list,
  getById,
  create,
  update,
  toggle,
  delete: deleteRouter
}

export default tlsFingerprintRouterAPI
