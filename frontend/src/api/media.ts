import { apiClient } from './client'

export interface MediaAsset {
  id: number
  owner_user_id: number
  biz_type: 'invoice' | 'ticket' | 'avatar' | 'image_task' | string
  biz_id: string
  storage_key: string
  sha256: string
  mime: string
  filename: string
  size: number
  visibility: 'public' | 'private' | string
  status: string
  storage_profile_id: string
  public_base_url?: string
  access_url?: string
  created_at: string
}

export interface MediaDownloadGrant {
  url: string
  expires_at: number
  sig: string
  ttl_minutes: number
}

export function mediaPublicUrl(id: number): string {
  return `/api/v1/media/public/${id}`
}

function postFormData<T>(url: string, body: FormData) {
  return apiClient.post<T>(url, body, {
    transformRequest: [
      (data, headers) => {
        if (data instanceof FormData) {
          delete headers['Content-Type']
        }
        return data
      },
    ],
    timeout: 120000,
  })
}

export const mediaAPI = {
  upload(file: File, fields: { biz_type: string; biz_id?: string; visibility?: string; filename?: string }) {
    const body = new FormData()
    body.append('file', file)
    body.append('biz_type', fields.biz_type)
    if (fields.biz_id) body.append('biz_id', fields.biz_id)
    if (fields.visibility) body.append('visibility', fields.visibility)
    if (fields.filename) body.append('filename', fields.filename)
    return postFormData<MediaAsset>('/media/upload', body)
  },

  presignDownload(id: number, ttlMinutes?: number) {
    return apiClient.post<MediaDownloadGrant>(`/media/${id}/presign-download`, {
      ttl_minutes: ttlMinutes,
    })
  },
}

export const adminMediaAPI = {
  upload(file: File, fields: { owner_user_id: number; biz_type: string; biz_id?: string; visibility?: string; filename?: string }) {
    const body = new FormData()
    body.append('file', file)
    body.append('owner_user_id', String(fields.owner_user_id))
    body.append('biz_type', fields.biz_type)
    if (fields.biz_id) body.append('biz_id', fields.biz_id)
    if (fields.visibility) body.append('visibility', fields.visibility)
    if (fields.filename) body.append('filename', fields.filename)
    return postFormData<MediaAsset>('/admin/media/upload', body)
  },

  presignDownload(id: number, ttlMinutes?: number) {
    return apiClient.post<MediaDownloadGrant>(`/admin/media/${id}/presign-download`, {
      ttl_minutes: ttlMinutes,
    })
  },
}
