import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { VoiceCall, decodeVoicePCM } from '../voiceCall'

const ticket = vi.hoisted(() => vi.fn())
vi.mock('../voiceApi', () => ({
  issueVoiceTicket: ticket,
  voiceWebSocketURL: () => 'ws://localhost/api/v1/creation/audio/realtime',
}))

class FakeSocket {
  static OPEN = 1
  static CLOSING = 2
  static instances: FakeSocket[] = []
  readyState = 0
  bufferedAmount = 0
  send = vi.fn()
  onopen: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null
  onerror: (() => void) | null = null
  onclose: ((event: { code: number }) => void) | null = null
  constructor(public url: string, public protocols: string[]) { FakeSocket.instances.push(this) }
  open() { this.readyState = 1; this.onopen?.() }
  receive(event: unknown) { this.onmessage?.({ data: JSON.stringify(event) }) }
  close = vi.fn(() => { this.readyState = 3; this.onclose?.({ code: 1000 }) })
}

class FakeWorklet {
  static instances: FakeWorklet[] = []
  port = { onmessage: null as ((event: { data: { pcm: ArrayBuffer; level: number } }) => void) | null }
  connect = vi.fn()
  disconnect = vi.fn()
  constructor() { FakeWorklet.instances.push(this) }
}

class FakeContext {
  static instances: FakeContext[] = []
  sampleRate = 24000
  currentTime = 0
  state = 'running'
  destination = {}
  audioWorklet = { addModule: vi.fn().mockResolvedValue(undefined) }
  source = { connect: vi.fn(), disconnect: vi.fn() }
  gain = { connect: vi.fn(), disconnect: vi.fn(), gain: { value: 1 } }
  players: Array<ReturnType<FakeContext['createBufferSource']>> = []
  resume = vi.fn().mockResolvedValue(undefined)
  close = vi.fn(async () => { this.state = 'closed' })
  constructor() { FakeContext.instances.push(this) }
  createMediaStreamSource() { return this.source }
  createGain() { return this.gain }
  createBuffer(_channels: number, length: number, rate: number) { return { duration: length / rate, copyToChannel: vi.fn() } }
  createBufferSource(): { buffer: unknown; connect: ReturnType<typeof vi.fn>; disconnect: ReturnType<typeof vi.fn>; start: ReturnType<typeof vi.fn>; stop: ReturnType<typeof vi.fn>; onended: (() => void) | null } {
    const player = { buffer: null, connect: vi.fn(), disconnect: vi.fn(), start: vi.fn(), stop: vi.fn(), onended: null }
    this.players.push(player)
    return player
  }
}

describe('VoiceCall real protocol and lifecycle', () => {
  let call: VoiceCall | undefined
  const track = { enabled: true, stop: vi.fn(), addEventListener: vi.fn() }
  const stream = { getTracks: () => [track], getAudioTracks: () => [track] }
  const getUserMedia = vi.fn()
  const status = vi.fn()
  const errors = vi.fn()
  const transcript = vi.fn()

  beforeEach(() => {
    FakeSocket.instances = []
    FakeWorklet.instances = []
    FakeContext.instances = []
    track.enabled = true
    track.stop.mockClear()
    status.mockClear(); errors.mockClear(); transcript.mockClear()
    ticket.mockReset().mockResolvedValue('a'.repeat(43))
    getUserMedia.mockReset().mockResolvedValue(stream)
    vi.stubGlobal('WebSocket', FakeSocket)
    vi.stubGlobal('AudioContext', FakeContext)
    vi.stubGlobal('AudioWorkletNode', FakeWorklet)
    vi.stubGlobal('navigator', { mediaDevices: { getUserMedia } })
  })
  afterEach(() => { call?.stop(); call = undefined; vi.unstubAllGlobals() })

  function createCall() {
    call = new VoiceCall({ groupId: 3, voice: 'eve', onStatus: status, onError: errors, onLevel: vi.fn(), onTranscript: transcript })
    return call
  }

  it('exchanges an in-memory ticket, configures PCM JSON audio, sends microphone audio and plays responses', async () => {
    const active = createCall()
    const starting = active.start()
    await flushPromises()
    const socket = FakeSocket.instances[0]
    expect(socket.url).not.toContain('?')
    expect(socket.protocols).toEqual(['creation-voice', `ticket.${'a'.repeat(43)}`])
    socket.open()
    const update = JSON.parse(socket.send.mock.calls[0][0])
    expect(update).toMatchObject({ type: 'session.update', session: {
      voice: 'eve', turn_detection: { type: 'server_vad' },
      audio: { input: { format: { type: 'audio/pcm', rate: 24000 }, transport: 'json' } },
    } })
    socket.receive({ type: 'session.updated' })
    await starting
    expect(status).toHaveBeenLastCalledWith('connected')
    const pcm = new Int16Array([0, 16384, -16384])
    FakeWorklet.instances[0].port.onmessage?.({ data: { pcm: pcm.buffer, level: 0.2 } })
    const append = JSON.parse(socket.send.mock.calls[1][0])
    expect(append.type).toBe('input_audio_buffer.append')
    expect(Array.from(decodeVoicePCM(append.audio))).toEqual([0, 0.5, -0.5])
    socket.receive({ type: 'response.created' })
    socket.receive({ type: 'response.output_audio.delta', delta: append.audio })
    const context = FakeContext.instances[0]
    expect(context.players).toHaveLength(1)
    expect(context.players[0].start).toHaveBeenCalledOnce()
    socket.receive({ type: 'conversation.item.input_audio_transcription.completed', transcript: 'Hello' })
    expect(transcript).toHaveBeenCalledWith('user', 'Hello')

    active.setMuted(true)
    expect(track.enabled).toBe(false)
    FakeWorklet.instances[0].port.onmessage?.({ data: { pcm: pcm.buffer, level: 0.2 } })
    expect(socket.send).toHaveBeenCalledTimes(2)
    active.setMuted(false)
    expect(track.enabled).toBe(true)
    socket.receive({ type: 'input_audio_buffer.speech_started' })
    expect(context.players[0].stop).toHaveBeenCalledOnce()
    active.stop()
    expect(track.stop).toHaveBeenCalledOnce()
    expect(context.close).toHaveBeenCalledOnce()
    expect(socket.close).toHaveBeenCalledWith(1000, 'Call ended')
  })

  it('stops tracks returned after the user cancels the permission prompt', async () => {
    let grant!: (value: typeof stream) => void
    getUserMedia.mockReturnValue(new Promise((resolve) => { grant = resolve }))
    const active = createCall()
    const starting = active.start().catch((error) => error)
    await flushPromises()
    active.stop()
    grant(stream)
    expect(await starting).toMatchObject({ name: 'AbortError' })
    expect(track.stop).toHaveBeenCalledOnce()
    expect(ticket).not.toHaveBeenCalled()
    expect(FakeSocket.instances).toHaveLength(0)
  })

  it('surfaces denied microphone permission without opening a paid connection', async () => {
    getUserMedia.mockRejectedValue(new DOMException('Microphone denied', 'NotAllowedError'))
    await expect(createCall().start()).rejects.toMatchObject({ name: 'NotAllowedError' })
    expect(errors).toHaveBeenCalledOnce()
    expect(ticket).not.toHaveBeenCalled()
    expect(FakeContext.instances[0].close).toHaveBeenCalledOnce()
  })

  it('cleans up after an upstream error during session negotiation', async () => {
    const starting = createCall().start().catch((error) => error)
    await flushPromises()
    const socket = FakeSocket.instances[0]
    socket.open()
    socket.receive({ type: 'error', error: { message: 'Voice quota exhausted' } })
    await starting
    expect(errors.mock.calls[0][0].message).toBe('Voice quota exhausted')
    expect(track.stop).toHaveBeenCalledOnce()
    expect(status).toHaveBeenLastCalledWith('ended')
  })

  it('rejects malformed PCM instead of scheduling corrupted playback', () => {
    expect(() => decodeVoicePCM('YQ==')).toThrow('Invalid voice audio frame')
  })
})
