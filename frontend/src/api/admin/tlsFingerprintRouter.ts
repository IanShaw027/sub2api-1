import { apiClient } from '../client'

export interface TLSFingerprintRouterRule {
  name: string
  enabled: boolean
  transport?: string
  protocol?: string
  match_type?: string
  pattern?: string
  case_sensitive?: boolean
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

export async function list(): Promise<TLSFingerprintRouter[]> {
  const { data } = await apiClient.get<TLSFingerprintRouter[]>('/admin/tls-fingerprint-routers')
  return data
}

export async function create(payload: CreateRouterRequest): Promise<TLSFingerprintRouter> {
  const { data } = await apiClient.post<TLSFingerprintRouter>('/admin/tls-fingerprint-routers', payload)
  return data
}

export async function update(id: number, payload: UpdateRouterRequest): Promise<TLSFingerprintRouter> {
  const { data } = await apiClient.put<TLSFingerprintRouter>(`/admin/tls-fingerprint-routers/${id}`, payload)
  return data
}

export async function deleteRouter(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/tls-fingerprint-routers/${id}`)
  return data
}

export const tlsFingerprintRouterAPI = { list, create, update, delete: deleteRouter }
export default tlsFingerprintRouterAPI
