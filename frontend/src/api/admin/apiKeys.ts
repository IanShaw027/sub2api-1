/**
 * Admin API Keys API endpoints
 * Handles API key management for administrators
 */

import { apiClient } from '../client'
import type { ApiKey } from '@/types'

export type AdminApiKeyStatus = 'active' | 'disabled'

export interface UpdateApiKeyGroupResult {
  api_key: ApiKey
  auto_granted_group_access: boolean
  granted_group_id?: number
  granted_group_name?: string
}

export interface UpdateAdminApiKeyPayload {
  groupId?: number | null
  status?: AdminApiKeyStatus
  resetRateLimitUsage?: boolean
}

/**
 * Update an API key's admin-managed fields.
 * @param id - API Key ID
 * @param payload - Optional group binding and/or status
 */
export async function updateApiKey(
  id: number,
  payload: UpdateAdminApiKeyPayload
): Promise<UpdateApiKeyGroupResult> {
  const body: Record<string, unknown> = {}
  if (payload.groupId !== undefined) {
    body.group_id = payload.groupId === null ? 0 : payload.groupId
  }
  if (payload.status !== undefined) {
    body.status = payload.status
  }
  if (payload.resetRateLimitUsage !== undefined) {
    body.reset_rate_limit_usage = payload.resetRateLimitUsage
  }
  const { data } = await apiClient.put<UpdateApiKeyGroupResult>(`/admin/api-keys/${id}`, body)
  return data
}

/**
 * Update an API key's group binding
 * @param id - API Key ID
 * @param groupId - Group ID (0 to unbind, positive to bind, null/undefined to skip)
 * @returns Updated API key with auto-grant info
 */
export async function updateApiKeyGroup(id: number, groupId: number | null): Promise<UpdateApiKeyGroupResult> {
  return updateApiKey(id, { groupId })
}

/**
 * Enable or disable an API key (admin).
 * @param id - API Key ID
 * @param status - `active` to enable, `disabled` to ban/disable
 */
export async function updateApiKeyStatus(
  id: number,
  status: AdminApiKeyStatus
): Promise<UpdateApiKeyGroupResult> {
  return updateApiKey(id, { status })
}

export const apiKeysAPI = {
  updateApiKey,
  updateApiKeyGroup,
  updateApiKeyStatus
}

export default apiKeysAPI
