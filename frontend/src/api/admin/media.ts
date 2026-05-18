/**
 * Admin Media API endpoints
 */

import { apiClient } from '../client'

export type MediaVisibility = 'public' | 'private'

export interface UploadMediaOptions {
  biz_type?: string
  visibility?: MediaVisibility
}

interface UploadMediaResponse {
  url?: string
  public_url?: string
}

export async function uploadMedia(file: File, options: UploadMediaOptions = {}): Promise<string> {
  const formData = new FormData()
  formData.append('file', file)
  if (options.biz_type) formData.append('biz_type', options.biz_type)
  if (options.visibility) formData.append('visibility', options.visibility)

  const { data } = await apiClient.post<UploadMediaResponse>('/admin/media/upload', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })

  return data.public_url || data.url || ''
}
