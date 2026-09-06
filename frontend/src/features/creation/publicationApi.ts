import { apiClient, buildApiUrl } from '@/api/client'

export type PublicationKind = 'image' | 'video'
export interface Publication {
  id: number
  owner_user_id: number
  title: string
  prompt: string
  model: string
  kind: PublicationKind
  status: 'pending' | 'published'
  mime: string
  size: number
  media_url: string
  created_at: string
  withdrawn_at?: string
}
export interface PublicationPage {
  items: Publication[]
  total: number
  page: number
  page_size: number
  pages: number
}
export interface PublishableWork {
  id: string
  kind: PublicationKind
  blob: Blob
  prompt: string
  model?: string
  title?: string
}
export interface PublicationDraft {
  request_id: string
  title: string
  prompt: string
  model: string
}
export const MAX_PUBLICATION_BYTES = 64 * 1024 * 1024
export const PUBLICATION_MIME_TYPES = {
  image: ['image/png', 'image/jpeg', 'image/gif', 'image/webp'],
  video: ['video/mp4', 'video/webm'],
}
function withMediaURL(publication: Publication): Publication {
  return { ...publication, media_url: publication.media_url ? buildApiUrl(publication.media_url) : '' }
}

export async function listPublications(params: { kind?: PublicationKind; search?: string; page?: number; page_size?: number }, signal?: AbortSignal): Promise<PublicationPage> {
  const { data } = await apiClient.get<PublicationPage>('/creation/gallery', { params, signal })
  return { ...data, items: data.items.map(withMediaURL) }
}

export async function publishMedia(work: PublishableWork, draft: PublicationDraft): Promise<Publication> {
  if (!(work.blob instanceof Blob) || work.blob.size === 0 || work.blob.size > MAX_PUBLICATION_BYTES) {
    throw new Error('Invalid publication file size')
  }
  if (!PUBLICATION_MIME_TYPES[work.kind].includes(work.blob.type)) throw new Error('Unsupported publication media type')
  const body = new FormData()
  const extension = work.blob.type.split('/')[1].replace('jpeg', 'jpg')
  body.append('file', work.blob, `publication.${extension}`)
  body.append('kind', work.kind)
  body.append('visibility', 'public')
  body.append('request_id', draft.request_id)
  body.append('title', draft.title)
  body.append('prompt', draft.prompt)
  body.append('model', draft.model)
  const { data } = await apiClient.post<Publication>('/creation/publications', body, {
    timeout: 120000,
    transformRequest: [(data, headers) => {
      // Let the browser set the multipart boundary, overriding the JSON default.
      headers.delete('Content-Type')
      return data
    }],
  })
  return withMediaURL(data)
}

export async function getPublicationRequest(requestId: string): Promise<Publication> {
  const { data } = await apiClient.get<Publication>(`/creation/publications/requests/${encodeURIComponent(requestId)}`)
  return withMediaURL(data)
}

export async function withdrawPublication(id: number): Promise<void> {
  await apiClient.delete(`/creation/publications/${id}`)
}
