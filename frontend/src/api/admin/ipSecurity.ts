import { apiClient } from '../client'

export interface IPSecurityConfig {
  enabled: boolean
  window_minutes: number
  account_threshold: number
  learning_until: string
}

export interface IPSecurityBan {
  id: number
  ip_address: string
  status: 'active' | 'whitelisted' | 'released' | string
  reason: string
  account_threshold: number
  window_minutes: number
  detected_account_count: number
  first_seen_at: string
  last_seen_at: string
  created_at: string
  released_at?: string
}

export interface IPSecurityActivityDetail {
  ip_address: string
  peer_ip: string
  forwarded_for: string
  user_id: number
  user_email: string
  user_username: string
  source: 'web' | 'apikey' | string
  api_key_id: number
  method: string
  path: string
  request_id: string
  request_count: number
  first_seen_at: string
  last_seen_at: string
}

export const ipSecurityAPI = {
  getConfig: async () => (await apiClient.get<IPSecurityConfig>('/admin/ip-security/config')).data,
  listBans: async (params?: { status?: string; page?: number; page_size?: number }) =>
    (await apiClient.get<{ items: IPSecurityBan[]; page: number; page_size: number; total: number; pages: number }>('/admin/ip-security/bans', { params })).data,
  getBan: async (id: number, params?: { page?: number; page_size?: number }) =>
    (await apiClient.get<{ ban: IPSecurityBan; activities: IPSecurityActivityDetail[]; page: number; page_size: number; total: number; pages: number }>(`/admin/ip-security/bans/${id}`, { params })).data,
  releaseBan: async (id: number) =>
    (await apiClient.post<{ message: string }>(`/admin/ip-security/bans/${id}/release`)).data,
  removeWhitelist: async (id: number) =>
    (await apiClient.post<{ message: string }>(`/admin/ip-security/bans/${id}/remove-whitelist`)).data,
}

export default ipSecurityAPI
