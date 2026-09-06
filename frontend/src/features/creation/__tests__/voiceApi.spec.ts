import { beforeEach, describe, expect, it, vi } from 'vitest'
import { generateSpeech, transcribeAudio, issueVoiceTicket, voiceWebSocketURL } from '../voiceApi'

const fetchVoice = vi.hoisted(() => vi.fn())
vi.mock('@/api/authenticatedFetch', () => ({ authenticatedFetch: fetchVoice }))
vi.mock('@/api/client', () => ({ buildApiUrl: (path: string) => `/api/v1${path}` }))

describe('voice API JWT bridge', () => {
  beforeEach(() => {
    fetchVoice.mockReset()
    localStorage.clear()
    localStorage.setItem('auth_token', 'user-jwt')
    localStorage.setItem('auth_user', '{"id":1}')
  })

  it('sends native TTS fields and preserves raw audio instead of unwrapping JSON', async () => {
    const audio = new Blob(['audio'], { type: 'audio/mpeg' })
    fetchVoice.mockResolvedValue({ ok: true, blob: async () => audio })
    const signal = new AbortController().signal
    const body = { text: 'hello', voice_id: 'eve' as const, language: 'auto' as const, speed: 1 }
    expect(await generateSpeech(12, body, signal)).toBe(audio)
    const [url, init] = fetchVoice.mock.calls[0]
    expect(url).toBe('/api/v1/creation/audio/speech')
    expect(init.headers.get('Authorization')).toBe('Bearer user-jwt')
    expect(init.headers.get('X-Group-Id')).toBe('12')
    expect(JSON.parse(init.body)).toEqual(body)
    expect(init.signal).toBe(signal)
  })

  it('uploads the audio as multipart without hardcoding its boundary', async () => {
    fetchVoice.mockResolvedValue({ ok: true, json: async () => ({ text: 'A transcript', duration: 5 }) })
    const file = new File(['audio'], 'voice.mp3', { type: 'audio/mpeg' })
    expect(await transcribeAudio(12, file, new AbortController().signal)).toBe('A transcript')
    const [url, init] = fetchVoice.mock.calls[0]
    expect(url).toBe('/api/v1/creation/audio/transcriptions')
    expect(init.body.get('file')).toBe(file)
    expect(init.headers.has('Content-Type')).toBe(false)
  })

  it('rejects error responses and non-audio success payloads', async () => {
    const request = { text: 'hello', voice_id: 'eve' as const, language: 'auto' as const, speed: 1 }
    fetchVoice.mockResolvedValueOnce({ ok: false, status: 403, json: async () => ({ error: { message: 'quota exceeded' } }) })
    await expect(generateSpeech(12, request, new AbortController().signal)).rejects.toThrow('quota exceeded')
    fetchVoice.mockResolvedValueOnce({ ok: true, blob: async () => new Blob(['{}'], { type: 'application/json' }) })
    await expect(generateSpeech(12, request, new AbortController().signal)).rejects.toThrow('invalid audio')
  })

  it('rejects a response after the signed-in identity changes', async () => {
    fetchVoice.mockImplementation(async () => {
      localStorage.setItem('auth_user', '{"id":2}')
      return { ok: true, json: async () => ({ text: 'Private transcript' }) }
    })
    await expect(transcribeAudio(12, new File(['audio'], 'voice.mp3'), new AbortController().signal)).rejects.toMatchObject({ name: 'AbortError' })
  })

  it('rejects missing transcript data and empty file input', async () => {
    fetchVoice.mockResolvedValue({ ok: true, json: async () => ({ duration: 5 }) })
    await expect(transcribeAudio(12, new File(['audio'], 'voice.mp3'), new AbortController().signal)).rejects.toThrow('invalid transcript')
    fetchVoice.mockClear()
    await expect(transcribeAudio(12, new File([], 'voice.mp3'), new AbortController().signal)).rejects.toThrow('non-empty')
    expect(fetchVoice).not.toHaveBeenCalled()
  })

  it('exchanges JWT for a short-lived ticket without putting credentials in the WebSocket URL', async () => {
    fetchVoice.mockResolvedValue({ ok: true, json: async () => ({ ticket: 'a'.repeat(43), expires_at: 123 }) })
    expect(await issueVoiceTicket(12, new AbortController().signal)).toBe('a'.repeat(43))
    expect(fetchVoice.mock.calls[0][0]).toBe('/api/v1/creation/audio/realtime-ticket')
    expect(fetchVoice.mock.calls[0][1].headers.get('Authorization')).toBe('Bearer user-jwt')
    const url = new URL(voiceWebSocketURL())
    expect(url.pathname).toBe('/api/v1/creation/audio/realtime')
    expect(url.search).toBe('')
    expect(url.username).toBe('')
    expect(localStorage.getItem('voice_ticket')).toBeNull()
  })
})
