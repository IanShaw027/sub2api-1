import { buildApiUrl } from '@/api/client'
import { authenticatedFetch } from '@/api/authenticatedFetch'

export const VOICE_UPLOAD_LIMIT = 25 * 1024 * 1024
export const SPEECH_TEXT_LIMIT = 15000
export const speechVoices = ['eve', 'ara', 'leo', 'rex', 'sal'] as const
export type SpeechVoice = typeof speechVoices[number]
export type SpeechLanguage = 'auto' | 'zh' | 'en'

export interface SpeechRequest {
  text: string
  voice_id: SpeechVoice
  language: SpeechLanguage
  speed: number
}

function headers(groupId: number): Headers {
  const result = new Headers({ 'X-Group-Id': String(groupId) })
  const token = localStorage.getItem('auth_token')
  if (token) result.set('Authorization', `Bearer ${token}`)
  return result
}

async function requestVoice(path: string, groupId: number, body: BodyInit, signal: AbortSignal, json = false): Promise<Response> {
  const owner = localStorage.getItem('auth_user')
  const requestHeaders = headers(groupId)
  if (json) requestHeaders.set('Content-Type', 'application/json')
  const response = await authenticatedFetch(buildApiUrl(`/creation/audio/${path}`), {
    method: 'POST', headers: requestHeaders, body, signal,
  })
  if (signal.aborted || owner !== localStorage.getItem('auth_user')) throw new DOMException('Aborted', 'AbortError')
  if (!response.ok) {
    const payload = await response.json().catch(() => null)
    throw new Error(payload?.error?.message || payload?.message || `Voice request failed (${response.status})`)
  }
  return response
}

export async function generateSpeech(groupId: number, request: SpeechRequest, signal: AbortSignal): Promise<Blob> {
  const response = await requestVoice('speech', groupId, JSON.stringify(request), signal, true)
  const audio = await response.blob()
  if (!audio.size || (!audio.type.startsWith('audio/') && audio.type !== 'application/octet-stream')) {
    throw new Error('Voice gateway returned an invalid audio response')
  }
  return audio.type === 'application/octet-stream' ? new Blob([audio], { type: 'audio/mpeg' }) : audio
}

export async function transcribeAudio(groupId: number, file: File, signal: AbortSignal): Promise<string> {
  if (!file.size || file.size > VOICE_UPLOAD_LIMIT) throw new Error('Audio file must be non-empty and at most 25 MiB')
  const body = new FormData()
  body.append('file', file)
  const response = await requestVoice('transcriptions', groupId, body, signal)
  const result: unknown = await response.json()
  if (!result || typeof result !== 'object' || !('text' in result) || typeof result.text !== 'string') {
    throw new Error('Voice gateway returned an invalid transcript')
  }
  return result.text
}

export async function issueVoiceTicket(groupId: number, signal: AbortSignal): Promise<string> {
  const response = await requestVoice('realtime-ticket', groupId, '{}', signal, true)
  const result: unknown = await response.json()
  if (!result || typeof result !== 'object' || !('ticket' in result) ||
    typeof result.ticket !== 'string' || !/^[A-Za-z0-9_-]{43}$/.test(result.ticket)) {
    throw new Error('Voice gateway returned an invalid connection ticket')
  }
  return result.ticket
}

export function voiceWebSocketURL(): string {
  const url = new URL(buildApiUrl('/creation/audio/realtime'), window.location.origin)
  if (url.origin !== window.location.origin) throw new Error('Voice calls require a same-origin gateway')
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.href
}
