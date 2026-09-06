import { apiClient, buildApiUrl } from '@/api/client'
import { authenticatedFetch } from '@/api/authenticatedFetch'
import type { GatewayModelList } from './types'
import type { MediaCost, MediaMode, MediaRequest, VideoOperation } from './localMedia'
import { GEMINI_IMAGE_ASPECT_RATIOS, isGeminiImageModel } from './mediaModels'

export interface MediaJob {
  id?: string
  task_id?: string
  request_id?: string
  status: string
  result?: { data?: Array<{ b64_json?: string; mime_type?: string; revised_prompt?: string }> }
  error?: unknown
  video?: { url?: string }
  observation_persisted?: boolean
  observation_warning?: string
}

export function unavailableMediaCost(reason = 'unavailable'): MediaCost {
  return { status: 'unavailable', currency: 'USD', amount: null, billing_target: null, reason }
}

export function normalizeMediaCost(raw: MediaCost): MediaCost {
  if (!raw || raw.currency !== 'USD' || !['estimated', 'settled', 'pending', 'unavailable', 'not_billed'].includes(raw.status)) return unavailableMediaCost('invalid_response')
  if (raw.status === 'not_billed') return raw.amount === 0 ? { ...raw, amount: 0, billing_target: null } : unavailableMediaCost('invalid_amount')
  if (raw.status === 'pending' || raw.status === 'unavailable') return { ...raw, amount: null, billing_target: null }
  if (typeof raw.amount !== 'number' || !Number.isFinite(raw.amount) || raw.amount < 0 || !['balance', 'subscription'].includes(raw.billing_target || '')) return unavailableMediaCost('invalid_amount')
  return { ...raw }
}

function errorMessage(value: unknown): string {
  if (typeof value === 'string') return value
  if (value && typeof value === 'object' && 'message' in value && typeof value.message === 'string') return value.message
  return 'Media generation failed.'
}

async function request(groupId: number, path: string, signal: AbortSignal, body?: FormData | Record<string, unknown>): Promise<Response> {
  const token = localStorage.getItem('auth_token')
  if (!token) throw new Error('Sign in before generating media.')
  const headers: Record<string, string> = { Authorization: `Bearer ${token}`, 'X-Group-Id': String(groupId) }
  if (body && !(body instanceof FormData)) headers['Content-Type'] = 'application/json'
  const response = await authenticatedFetch(buildApiUrl(`/creation/local${path}?group_id=${groupId}`), {
    method: body ? 'POST' : 'GET', headers, signal,
    body: body instanceof FormData ? body : body ? JSON.stringify(body) : undefined,
  })
  if (!response.ok) {
    const value = await response.json().catch(() => null)
    throw new Error(errorMessage(value?.error ?? value?.message ?? `Media request failed (${response.status}).`))
  }
  return response
}

export function fileDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error ?? new Error('Could not read source image.'))
    reader.readAsDataURL(blob)
  })
}

export async function inspectSourceVideo(file: Blob, operation: VideoOperation): Promise<number> {
  if (!file.size || file.size > 32 * 1024 * 1024) throw new Error('Select a non-empty MP4 video no larger than 32 MB.')
  const header = new Uint8Array(await file.slice(0, 12).arrayBuffer())
  if (header.length < 12 || String.fromCharCode(...header.slice(4, 8)) !== 'ftyp') throw new Error('The source video must be an MP4 file.')
  const duration = await new Promise<number>((resolve, reject) => {
    const video = document.createElement('video')
    const url = URL.createObjectURL(file)
    const finish = (error?: Error) => {
      clearTimeout(timeout)
      video.onloadedmetadata = null
      video.onerror = null
      const duration = video.duration
      video.removeAttribute('src')
      video.load()
      URL.revokeObjectURL(url)
      if (error) reject(error)
      else resolve(duration)
    }
    const timeout = setTimeout(() => finish(new Error('Could not read source video duration.')), 10000)
    video.preload = 'metadata'
    video.onloadedmetadata = () => finish()
    video.onerror = () => finish(new Error('The source MP4 video could not be decoded.'))
    video.src = url
  })
  if (!Number.isFinite(duration) || duration <= 0) throw new Error('The source video duration is invalid.')
  if (operation === 'edit' && duration > 8.7) throw new Error('Video editing requires a source no longer than 8.7 seconds.')
  if (operation === 'extension' && (duration < 2 || duration > 15)) throw new Error('Video extension requires a source between 2 and 15 seconds.')
  return duration
}

export function imageJobBlob(job: MediaJob): { blob: Blob; revisedPrompt?: string } {
  const item = job.result?.data?.[0]
  if (!item?.b64_json) throw new Error('The completed task did not contain a downloadable image.')
  const binary = atob(item.b64_json)
  const bytes = Uint8Array.from(binary, char => char.charCodeAt(0))
  return { blob: new Blob([bytes], { type: item.mime_type || 'image/png' }), revisedPrompt: item.revised_prompt }
}

async function normalizeVideoBlob(blob: Blob): Promise<Blob> {
  const bytes = new Uint8Array(await blob.slice(0, 32).arrayBuffer())
  const mp4 = bytes.length >= 12 && String.fromCharCode(...bytes.slice(4, 8)) === 'ftyp'
  const webm = bytes[0] === 0x1a && bytes[1] === 0x45 && bytes[2] === 0xdf && bytes[3] === 0xa3
  if (!mp4 && !webm) throw new Error('The returned media is not a supported MP4 or WebM video.')
  return new Blob([blob], { type: mp4 ? 'video/mp4' : 'video/webm' })
}

export const mediaAPI = {
  async quote(input: MediaRequest, signal?: AbortSignal): Promise<MediaCost> {
    const videoGeneration = input.mode === 'video' && (!input.videoOperation || input.videoOperation === 'generation')
    const { data } = await apiClient.get<MediaCost>('/creation/local/pricing', {
      params: {
        group_id: input.groupId, kind: input.mode, model: input.model,
        size: input.mode === 'image' ? isGeminiImageModel(input.model) ? input.settings.resolution || '1K' : input.settings.size : undefined,
        duration: input.mode === 'video' && input.videoOperation !== 'edit' ? input.settings.duration ?? (videoGeneration ? 5 : 6) : undefined,
        resolution: videoGeneration ? input.settings.resolution || '720p' : undefined,
      },
      signal, timeout: 5000,
    })
    return data?.status === 'settled' || data?.status === 'pending' ? unavailableMediaCost('invalid_status') : normalizeMediaCost(data)
  },
  async billing(mode: MediaMode, groupId: number, taskId: string, signal?: AbortSignal): Promise<MediaCost> {
    const { data } = await apiClient.get<MediaCost>('/creation/local/billing', {
      params: { group_id: groupId, kind: mode, task_id: taskId }, signal, timeout: 5000,
    })
    return data?.status === 'estimated' ? unavailableMediaCost('invalid_status') : normalizeMediaCost(data)
  },
  async getModels(groupId: number, signal?: AbortSignal): Promise<GatewayModelList> {
    const { data } = await apiClient.get<GatewayModelList>('/creation/models', { params: { group_id: groupId }, signal })
    return data
  },
  async submit(input: MediaRequest, signal: AbortSignal): Promise<MediaJob> {
    if (input.mode === 'video') {
      const operation = input.videoOperation ?? 'generation'
      if (!['generation', 'edit', 'extension'].includes(operation)) throw new Error('Unsupported video operation.')
      if (operation !== 'generation') {
        const source = input.files?.[0]
        if (!source || input.files?.length !== 1) throw new Error('Select one source MP4 video.')
        const duration = input.settings.duration ?? 6
        if (operation === 'extension' && (!Number.isInteger(duration) || duration < 2 || duration > 10)) throw new Error('Select an extension duration between 2 and 10 seconds.')
        return (await request(input.groupId, `/videos/${operation === 'edit' ? 'edits' : 'extensions'}`, signal, {
          model: input.model, prompt: input.prompt,
          video: { url: await fileDataURL(new Blob([source], { type: 'video/mp4' })) },
          ...(operation === 'extension' ? { duration } : {}),
        })).json()
      }
      const image = input.files?.[0]
      const body = {
        model: input.model, prompt: input.prompt,
        duration: input.settings.duration ?? 5,
        resolution: input.settings.resolution ?? '720p',
        ...(image ? { image: { url: await fileDataURL(image) } } : {}),
      }
      return (await request(input.groupId, '/videos/generations', signal, body)).json()
    }
    const body: Record<string, unknown> = { model: input.model, prompt: input.prompt, n: 1 }
    if (isGeminiImageModel(input.model)) {
      const size = input.settings.resolution || '1K'
      const ratio = input.settings.ratio || input.settings.aspectRatio || 'auto'
      if (!['1K', '2K', '4K'].includes(size) || (ratio !== 'auto' && !GEMINI_IMAGE_ASPECT_RATIOS.includes(ratio))) throw new Error('Select a supported Gemini image size and aspect ratio.')
      if ((input.files?.length || 0) > 14 || input.files?.some(file => !['image/png', 'image/jpeg', 'image/webp'].includes(file.type))) throw new Error('Gemini accepts up to 14 PNG, JPEG or WebP reference images.')
      if ((input.files || []).reduce((sum, file) => sum + file.size, 0) > 19 * 1024 * 1024) throw new Error('Gemini reference images must total no more than 19 MiB.')
      body.image_size = size
      if (ratio !== 'auto') body.aspect_ratio = ratio
    } else {
      if (input.settings.size) body.size = input.settings.size
      if (input.settings.quality) body.quality = input.settings.quality
      if (input.settings.aspectRatio && input.settings.aspectRatio !== 'auto') body.aspect_ratio = input.settings.aspectRatio
    }
    if (input.files?.length) {
      const form = new FormData()
      for (const [key, value] of Object.entries(body)) form.append(key, String(value))
      input.files.forEach(file => form.append('image', file, file.name))
      return (await request(input.groupId, '/images/edits/async', signal, form)).json()
    }
    return (await request(input.groupId, '/images/generations/async', signal, body)).json()
  },
  async poll(mode: MediaRequest['mode'], groupId: number, taskId: string, signal: AbortSignal): Promise<MediaJob> {
    const path = mode === 'image' ? `/images/tasks/${encodeURIComponent(taskId)}` : `/videos/${encodeURIComponent(taskId)}`
    return (await request(groupId, path, signal)).json()
  },
  async videoBlob(groupId: number, taskId: string, signal: AbortSignal): Promise<Blob> {
    const response = await request(groupId, `/videos/${encodeURIComponent(taskId)}/content`, signal)
    const blob = await response.blob()
    if (!blob.size) throw new Error('The completed video is empty.')
    return normalizeVideoBlob(blob)
  },
  errorMessage,
}
