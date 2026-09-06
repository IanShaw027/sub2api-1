import { issueVoiceTicket, voiceWebSocketURL, type SpeechVoice } from './voiceApi'
import captureWorkletURL from './voiceCapture.worklet.js?url'

export type VoiceCallStatus = 'requesting' | 'connecting' | 'connected' | 'ended'
export interface VoiceCallOptions {
  groupId: number
  voice: SpeechVoice
  onStatus: (status: VoiceCallStatus) => void
  onLevel: (level: number) => void
  onTranscript: (role: 'user' | 'assistant', text: string) => void
  onError: (error: Error) => void
}

export function decodeVoicePCM(base64: string): Float32Array {
  const binary = atob(base64)
  if (!binary.length || binary.length % 2) throw new Error('Invalid voice audio frame')
  const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0))
  const view = new DataView(bytes.buffer)
  return Float32Array.from({ length: bytes.length / 2 }, (_, index) => view.getInt16(index * 2, true) / 32768)
}

export class VoiceCall {
  private readonly abort = new AbortController()
  private context: AudioContext | null = null
  private stream: MediaStream | null = null
  private capture: AudioWorkletNode | null = null
  private source: MediaStreamAudioSourceNode | null = null
  private silence: GainNode | null = null
  private socket: WebSocket | null = null
  private playback = new Set<AudioBufferSourceNode>()
  private nextAudioAt = 0
  private stopped = false
  private ready = false
  private muted = false
  private timer: ReturnType<typeof setTimeout> | undefined
  private rejectStart: ((reason: Error) => void) | null = null
  private awaitingResponse = false

  constructor(private readonly options: VoiceCallOptions) {}

  async start(): Promise<void> {
    try {
      if (!navigator.mediaDevices?.getUserMedia || typeof AudioContext === 'undefined' || typeof AudioWorkletNode === 'undefined') {
        throw new Error('Microphone calls require a secure browser with AudioWorklet support')
      }
      this.options.onStatus('requesting')
      const context = new AudioContext({ sampleRate: 24000 })
      this.context = context
      await context.resume()
      this.assertActive()
      const stream = await navigator.mediaDevices.getUserMedia({ audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true } })
      if (this.stopped) {
        stream.getTracks().forEach((track) => track.stop())
        this.assertActive()
      }
      this.stream = stream
      stream.getAudioTracks().forEach((track) => {
        track.addEventListener('ended', () => this.fail(new Error('Microphone disconnected')), { once: true })
      })
      await context.audioWorklet.addModule(captureWorkletURL)
      this.assertActive()
      this.source = context.createMediaStreamSource(stream)
      this.capture = new AudioWorkletNode(context, 'creation-voice-capture')
      this.silence = context.createGain()
      this.silence.gain.value = 0
      this.source.connect(this.capture)
      this.capture.connect(this.silence)
      this.silence.connect(context.destination)
      this.capture.port.onmessage = ({ data }: MessageEvent<{ pcm: ArrayBuffer; level: number }>) => {
        if (this.stopped || !this.ready || this.muted || this.socket?.readyState !== WebSocket.OPEN) return
        this.options.onLevel(Math.min(1, data.level * 5))
        if (this.socket.bufferedAmount > 256 * 1024) {
          this.fail(new Error('Voice connection is too slow'))
          return
        }
        const audio = btoa(String.fromCharCode(...new Uint8Array(data.pcm)))
        this.socket.send(JSON.stringify({ type: 'input_audio_buffer.append', audio }))
      }
      this.options.onStatus('connecting')
      const ticket = await issueVoiceTicket(this.options.groupId, this.abort.signal)
      this.assertActive()
      const socket = new WebSocket(voiceWebSocketURL(), ['creation-voice', `ticket.${ticket}`])
      this.socket = socket
      await new Promise<void>((resolve, reject) => {
        this.rejectStart = reject
        this.timer = setTimeout(() => this.fail(new Error('Voice connection timed out')), 60000)
        socket.onopen = () => {
          socket.send(JSON.stringify({ type: 'session.update', session: {
            voice: this.options.voice,
            turn_detection: { type: 'server_vad' },
            audio: {
              input: { format: { type: 'audio/pcm', rate: context.sampleRate }, transport: 'json' },
              output: { format: { type: 'audio/pcm', rate: context.sampleRate }, transport: 'json' },
            },
          } }))
        }
        socket.onmessage = ({ data }: MessageEvent) => {
          if (this.stopped) return
          try {
            if (typeof data !== 'string') throw new Error('Unexpected binary voice event')
            const event = JSON.parse(data) as Record<string, unknown>
            if (event.type === 'session.updated' && !this.ready) {
              this.ready = true
              clearTimeout(this.timer)
              this.rejectStart = null
              this.options.onStatus('connected')
              resolve()
            } else {
              this.handleEvent(event)
            }
          } catch (error) {
            this.fail(error instanceof Error ? error : new Error('Invalid voice event'))
          }
        }
        socket.onerror = () => this.fail(new Error('Voice connection failed'))
        socket.onclose = (event) => {
          if (this.stopped) return
          if (!this.ready || (event.code !== 1000 && event.code !== 1001)) {
            this.fail(new Error('Voice connection was interrupted'))
          } else {
            this.stop()
          }
        }
      })
    } catch (error) {
      const aborted = this.stopped || this.abort.signal.aborted
      this.stop()
      if (!aborted) this.options.onError(error instanceof Error ? error : new Error('Voice call failed'))
      throw error
    }
  }

  setMuted(muted: boolean): void {
    this.muted = muted
    this.stream?.getAudioTracks().forEach((track) => { track.enabled = !muted })
    if (muted) this.options.onLevel(0)
  }

  stop(): void {
    if (this.stopped) return
    this.stopped = true
    this.ready = false
    this.abort.abort()
    clearTimeout(this.timer)
    this.rejectStart?.(new DOMException('Aborted', 'AbortError'))
    this.rejectStart = null
    if (this.socket) {
      this.socket.onopen = this.socket.onmessage = this.socket.onerror = this.socket.onclose = null
      if (this.socket.readyState < WebSocket.CLOSING) this.socket.close(1000, 'Call ended')
    }
    this.socket = null
    this.stream?.getTracks().forEach((track) => track.stop())
    this.stream = null
    if (this.capture) this.capture.port.onmessage = null
    this.source?.disconnect()
    this.capture?.disconnect()
    this.silence?.disconnect()
    this.source = this.capture = this.silence = null
    this.clearPlayback()
    if (this.context && this.context.state !== 'closed') void this.context.close().catch(() => undefined)
    this.context = null
    this.options.onLevel(0)
    this.options.onStatus('ended')
  }

  private assertActive(): void {
    if (this.stopped) throw new DOMException('Aborted', 'AbortError')
  }

  private fail(error: Error): void {
    if (this.stopped) return
    this.options.onError(error)
    this.stop()
  }

  private handleEvent(event: Record<string, unknown>): void {
    if (event.type === 'error') {
      const detail = event.error as { message?: string } | undefined
      throw new Error(detail?.message || 'Voice upstream rejected the session')
    }
    if (event.type === 'input_audio_buffer.speech_started') {
      this.awaitingResponse = true
      this.clearPlayback()
    }
    if (event.type === 'response.created') this.awaitingResponse = false
    if ((event.type === 'response.output_audio.delta' || event.type === 'response.audio.delta') &&
      typeof event.delta === 'string' && !this.awaitingResponse) this.playAudio(event.delta)
    if (event.type === 'conversation.item.input_audio_transcription.completed' && typeof event.transcript === 'string') {
      this.options.onTranscript('user', event.transcript)
    }
    if ((event.type === 'response.output_audio_transcript.done' || event.type === 'response.audio_transcript.done') && typeof event.transcript === 'string') {
      this.options.onTranscript('assistant', event.transcript)
    }
  }

  private playAudio(base64: string): void {
    const context = this.context
    if (!context || !this.ready || !base64) return
    const samples = decodeVoicePCM(base64)
    if (this.nextAudioAt - context.currentTime > 20) throw new Error('Voice playback buffer exceeded')
    const buffer = context.createBuffer(1, samples.length, context.sampleRate)
    buffer.copyToChannel(samples, 0)
    const source = context.createBufferSource()
    source.buffer = buffer
    source.connect(context.destination)
    source.onended = () => { this.playback.delete(source); source.disconnect() }
    const startsAt = Math.max(context.currentTime, this.nextAudioAt)
    this.nextAudioAt = startsAt + buffer.duration
    this.playback.add(source)
    source.start(startsAt)
  }

  private clearPlayback(): void {
    this.playback.forEach((source) => { source.stop(); source.disconnect() })
    this.playback.clear()
    this.nextAudioAt = 0
  }
}
